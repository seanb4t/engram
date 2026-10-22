// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"sync"
	"syscall"
	"time"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"

	"github.com/seanb4t/engram/internal/auth"
	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/telemetry"
	"github.com/seanb4t/engram/internal/webauth"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the engram MCP server",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runServe(cmd)
	},
}

// Flag defaults come from the ENGRAM_* env vars (set by the Helm chart) via
// internal/config, so env drives config and explicitly-set flags override —
// cobra+koanf, no viper.
func init() {
	f := serveCmd.Flags()
	f.String("listen-addr", config.FlagDefault("listen-addr"), "listen address")
	f.String("mcp-listen-addr", config.FlagDefault("mcp-listen-addr"),
		"second listen address serving only the MCP transport and discovery (optional)")
	f.String("oidc-issuer", config.FlagDefault("oidc-issuer"),
		"OIDC issuer URL; setting it enables bearer-token enforcement")
	f.String("oidc-audience", config.FlagDefault("oidc-audience"), "expected OIDC audience (optional)")
	f.String("oidc-resource-metadata", config.FlagDefault("oidc-resource-metadata"),
		"WWW-Authenticate resource metadata URL (optional)")
	f.String("ui-enabled", config.FlagDefault("ui-enabled"),
		"enable the web UI + login lane (empty=imply from creds; 'false'=hard off)")
	f.String("connect-headless", config.FlagDefault("connect-headless"),
		"mount the ConnectRPC lane on a headless deployment (default off; requires at least one configured auth lane)")
	f.String("ui-issuer", config.FlagDefault("ui-issuer"),
		"OIDC issuer for the web-UI login lane (empty=default to --oidc-issuer)")
	f.String("oidc-client-id", config.FlagDefault("oidc-client-id"), "OIDC confidential-client ID for the web login")
	f.String("oidc-client-secret", config.FlagDefault("oidc-client-secret"), "OIDC client secret for the web login")
	f.String("ui-redirect-url", config.FlagDefault("ui-redirect-url"), "OIDC auth-code callback URL")
	f.String("ui-cookie-key", config.FlagDefault("ui-cookie-key"), "32-byte AES-GCM key sealing the session cookie")
	f.String("mcp-path", config.FlagDefault("mcp-path"),
		"path for the MCP transport (empty=/mcp; '/'=legacy root catch-all)")
	f.String("owner-claim", config.FlagDefault("owner-claim"),
		"OIDC claim whose value becomes the record owner / authz key (default email)")
}

func runServe(cmd *cobra.Command) error {
	cfg, err := config.Load(cmd.Flags())
	if err != nil {
		// D-03 bucket 1 (pre-flight config/auth-guard validation): a
		// malformed flag/env value the process cannot use.
		return usageErrorf("load config: %w", err)
	}
	// Serve-local guard: an empty listen address makes http.Server bind ":http"
	// (port 80) silently. ENGRAM_LISTEN_ADDR defaults to :8080; only an explicit
	// --listen-addr "" can empty it. Data-plane fields are validated in the store
	// path (buildDepsFromEnv -> loadAndValidate -> Config.Validate). Bucket 1.
	if cfg.Server.ListenAddr == "" {
		return usageErrorf("ENGRAM_LISTEN_ADDR (or --listen-addr) is empty: a listen address is required")
	}
	if err := checkMCPListenAddr(cfg.Server.ListenAddr, cfg.Server.MCPListenAddr); err != nil {
		return err
	}

	// resolvedVersion (buildversion.go) is the single source for every
	// version surface this file emits: this OpenTelemetry attribute, the
	// MCP handshake below, and the startup log line — one engram serve
	// process never reports three different version strings.
	telCfg := telemetry.ConfigFromEnv("engram", resolvedVersion())
	logger, shutdown, err := telemetry.Setup(context.Background(), telCfg)
	if err != nil {
		// Bucket 1. telemetry.Setup never returns a non-nil error today --
		// it degrades to stdout logging instead of failing startup
		// (internal/telemetry/telemetry.go's own doc comment) -- but the
		// classification is typed here for if that contract ever changes.
		return usageErrorf("telemetry setup: %w", err)
	}
	slog.SetDefault(logger)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(ctx)
	}()

	// One ToolMetrics instance shared by tool instrumentation (Register) and
	// the auth-failure recorder (accessLog), so all counters/histograms share
	// the same instrument objects and export together.
	tm := telemetry.NewToolMetrics(otel.Meter("github.com/seanb4t/engram"))
	// SummaryQueueMetrics builds ONLY the static instruments (counters +
	// fill-latency histogram) here — no queue exists yet at this point in
	// startup. The queue-depth observable gauge is registered later, inside
	// buildDepsFromEnv, via telemetry.RegisterSummaryQueueDepth(meter,
	// q.depth), once a live queue exists to sample (D-09, Codex finding #1).
	sqm := telemetry.NewSummaryQueueMetrics(otel.Meter("github.com/seanb4t/engram"))
	// UsageQueueMetrics mirrors sqm: only the static enqueue/drop/fail
	// instruments are built here (no queue exists yet); the queue itself is
	// constructed later inside buildDepsFromEnv (buildUsageQueue) once
	// ENGRAM_USAGE_SIGNALS is resolved.
	uqm := telemetry.NewUsageQueueMetrics(otel.Meter("github.com/seanb4t/engram"))
	// Install the store/embed/auth latency instruments as telemetry package
	// state so the layer Record* helpers emit (no-op until this runs).
	telemetry.InitLayerMetrics(otel.Meter("github.com/seanb4t/engram"))

	// The MCP path and the protected-resource settings are serve-only, so they
	// are shape-checked here in runServe rather than in Config.Validate (which
	// is scoped to fields every command's store/embedder path consumes), and
	// before any backend is dialled. Bucket 1: a malformed or colliding
	// ENGRAM_MCP_PATH, a malformed ENGRAM_MCP_RESOURCE_URL or
	// ENGRAM_MCP_RESOURCE_ORIGINS, or both of the last two set.
	resolvedMCPPath, err := resolveMCPPath(cfg.Server.MCPPath)
	if err != nil {
		slog.Error("invalid MCP path", "err", err)
		return usageErrorf("%w", err)
	}
	resource, err := newProtectedResource(protectedResourceConfig{
		ResourceURL: cfg.Server.MCPResourceURL,
		Origins:     cfg.Server.MCPResourceOrigins,
		MetadataURL: cfg.OIDC.ResourceMetadata,
		MCPPath:     resolvedMCPPath,
		OnRefused:   tm.RecordResourceMetadataRefused,
	})
	if err != nil {
		slog.Error("invalid MCP resource config", "err", err)
		return usageErrorf("%w", err)
	}

	mux := http.NewServeMux()

	uiCfg, err := resolveUIConfig(uiRaw{
		Enabled:      cfg.UI.Enabled,
		UIIssuer:     cfg.UI.Issuer,
		OIDCIssuer:   cfg.OIDC.Issuer,
		ClientID:     cfg.OIDC.ClientID,
		ClientSecret: cfg.OIDC.ClientSecret,
		RedirectURL:  cfg.UI.RedirectURL,
		CookieKey:    cfg.UI.CookieKey,
	})
	if err != nil {
		slog.Error("web UI config invalid", "err", err)
		// Bucket 1. resolveUIConfig (uiconfig.go) is outside this plan's
		// file scope, so its bare error is classified here at the call
		// site rather than at its source, mirroring migrate.go's
		// buildRemapSource "wrap, don't reword" discipline.
		return usageErrorf("%w", err)
	}
	// Parse the comma-list once (parsing is separate from defaulting: the
	// registry already supplies "email" when unset). A malformed list
	// (duplicate/interior-empty/bad claim name) fails startup immediately
	// rather than surfacing as a runtime auth failure.
	ownerClaims, err := config.ParseOwnerClaims(cfg.OIDC.OwnerClaim)
	if err != nil {
		slog.Error("owner-claim config invalid", "err", err)
		// Bucket 1: a malformed --owner-claim comma-list.
		return usageErrorf("%w", err)
	}
	if err := ownerClaimGuard(cfg.OIDC.Issuer, uiCfg.Enabled, ownerClaims); err != nil {
		slog.Error("owner-claim config invalid", "err", err)
		// ownerClaimGuard already returns a usageErrorf-typed error below.
		return err
	}

	// D-06: build the verifier chain exactly ONCE here. The same chain value
	// reaches the MCP wrapper (withAuth, below) and the Connect bearer half
	// (connectResolverFor), so the two mount sites cannot drift — there is one
	// instance, so there is nothing to diverge.
	chain, err := buildAuthChain(cfg.OIDC, cfg.ServiceAuth, ownerClaims)
	if err != nil {
		// buildAuthChain already returns a usageErrorf-typed error below
		// (bucket 1: OIDC discovery misconfiguration and service-auth
		// config, D-03).
		slog.Error("oidc verifier init failed", "err", err, "issuer", cfg.OIDC.Issuer)
		return err
	}
	headless, err := strconv.ParseBool(cfg.Connect.Headless)
	if err != nil {
		// Bucket 1: a malformed --connect-headless boolean.
		err = usageErrorf("ENGRAM_CONNECT_HEADLESS (or --connect-headless) %q: must be a boolean: %w", cfg.Connect.Headless, err)
		slog.Error("connect-headless config invalid", "err", err)
		return err
	}
	if err := connectHeadlessGuard(headless, chain); err != nil {
		// connectHeadlessGuard already returns a usageErrorf-typed error
		// below (bucket 1: headless without a configured auth lane).
		slog.Error("connect-headless config invalid", "err", err)
		return err
	}
	if err := mcpListenerGuard(cfg.Server.MCPListenAddr, chain); err != nil {
		// Bucket 1: the MCP-only listener without a configured auth lane.
		slog.Error("MCP listener config invalid", "err", err)
		return err
	}

	var cookieResolve func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error)
	var connectCSRFVerify func(owner, token string) bool
	var connectReseal func(http.Header, *http.Request)
	var webHandler *webauth.Handler
	switch {
	case uiCfg.Enabled:
		// resolveUIConfig guarantees uiCfg.Issuer is non-empty here (it defaults
		// to ENGRAM_OIDC_ISSUER and fails fast when neither issuer is set).
		// Bucket 1 (cookie-key / CSRF construction, malformed configured
		// values) for every error below through the OIDC discovery call.
		key, err := decodeCookieKey(uiCfg.CookieKey)
		if err != nil {
			return usageErrorf("%w", err)
		}
		codec, err := webauth.NewSessionCodec(key)
		if err != nil {
			return usageErrorf("session cookie key: %w", err)
		}
		kcsrf, err := webauth.DeriveCSRFKey(key)
		if err != nil {
			return usageErrorf("derive csrf key: %w", err)
		}
		csrfSigner, err := webauth.NewCSRFSigner(kcsrf)
		if err != nil {
			return usageErrorf("csrf signer: %w", err)
		}
		oidcCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		authr, err := webauth.NewAuthenticator(oidcCtx, uiCfg.Issuer, uiCfg.ClientID, uiCfg.ClientSecret, uiCfg.RedirectURL, ownerClaims)
		cancel()
		if err != nil {
			return usageErrorf("web UI OIDC discovery: %w", err)
		}
		webHandler = webauth.NewHandler(authr, codec, true, csrfSigner)
		// internal/webauth is not modified (D-07); webauth.NewResolver(codec).Resolve
		// is the cookie half passed through unwrapped to connectResolverFor below,
		// which composes it with chain (the bearer half) once both are known.
		cookieResolve = webauth.NewResolver(codec).Resolve
		connectCSRFVerify = csrfSigner.Verify
		connectReseal = webHandler.Reseal
		slog.Info("web UI auth lane enabled", "issuer", uiCfg.Issuer, "redirect", uiCfg.RedirectURL)
	case headless:
		slog.Info("web UI disabled; Connect API mounted headless (ENGRAM_CONNECT_HEADLESS)")
	default:
		slog.Info("web UI disabled and ENGRAM_CONNECT_HEADLESS unset; Connect API not mounted")
	}

	// D-12: mounting (cookieResolve != nil || headless) and bearer-inclusion
	// (chain, unconditionally, whenever mounted) are decided together here —
	// after both halves are known — and nowhere else. mountConnect's own
	// resolve==nil gate is untouched.
	connectResolve := connectResolverFor(chain, headless, cookieResolve)

	srv := mcp.NewServer(&mcp.Implementation{Name: "engram", Version: resolvedVersion()}, nil)
	drain, err := server.Register(srv, mux, tm, sqm, uqm, connectResolve, connectCSRFVerify, connectReseal)
	if err != nil {
		slog.Error("server registration failed", "err", err)
		// D-03 bucket 2 (backend construction). server.Register's dominant
		// failure mode is buildDepsFromEnv -> ensureStoreFromConfig -- the
		// identical config-versus-unreachable-Qdrant split
		// classifyOperatorErrConstruction already resolves for the four
		// sweep commands (cmd/engram/operror.go): EnsureCollection is the
		// only network-touching call in that construction path, so
		// anything else reaching the classifier's default is config-shaped
		// by elimination. Reused here rather than re-deciding at this
		// fifth call site.
		return classifyOperatorErrConstruction(err)
	}

	if uiCfg.Enabled {
		mux.HandleFunc("GET /auth/login", webHandler.Login)
		mux.HandleFunc("GET /auth/callback", webHandler.Callback)
		mux.HandleFunc("POST /auth/logout", webHandler.Logout)
		// The console SPA is served under "/ui/"; mountMCPRoutes below redirects a
		// browser GET of the bare root here.
		mux.Handle("/ui/", http.StripPrefix("/ui/", webauth.StaticHandler()))
	}

	// streams ends every MCP event stream when a server shuts down, so
	// Shutdown waits only for requests that finish on their own.
	streams, endStreams := context.WithCancel(context.Background())
	defer endStreams()
	// The 401 challenge names ENGRAM_OIDC_RESOURCE_METADATA when set, else the
	// document on ENGRAM_MCP_RESOURCE_URL's origin when that is set, else the
	// path-suffix document on each request's own derived origin, or none when
	// ENGRAM_MCP_RESOURCE_ORIGINS refuses that origin.
	handler := mcpTransport(streams, srv, chain, resource, tm)
	// The RFC 9728 protected-resource metadata document is mounted bare on the
	// mux, not through withAuth/accessLog/otelhttp, like the /auth and /ui
	// mounts: RFC 9728 requires unauthenticated public reachability. The
	// whole-mux CrossOriginProtection wrapper still covers it and passes
	// safe-method GET untouched.
	prmHandler := protectedResourceHandler(resource, cfg.OIDC.Issuer)
	mountWellKnownRoutes(mux, prmHandler, resolvedMCPPath)
	slog.Info("protected-resource metadata mounted", resource.mode()...)
	if cfg.Server.MCPListenAddr != "" && resource.unbounded() {
		slog.Warn("ENGRAM_MCP_LISTEN_ADDR is set with neither ENGRAM_MCP_RESOURCE_ORIGINS nor ENGRAM_MCP_RESOURCE_URL: the protected-resource documents and the 401 challenge name whatever origin a caller's Host or X-Forwarded-Host gives")
	}
	// Mount the MCP transport at its configured path and give "/" to the console
	// landing / 404 handler (mcpPath="/" restores the legacy root catch-all).
	mountMCPRoutes(mux, handler, uiCfg.Enabled, resolvedMCPPath)
	slog.Info("MCP transport mounted", "path", resolvedMCPPath, "ui_enabled", uiCfg.Enabled)

	listeners := newListeners(cfg.Server, mux, handler, prmHandler, resolvedMCPPath, endStreams)

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// A bind failure (e.g. "address already in use") here exits 1, the
	// documented exception in guides/cli.md and guides/upgrade.md, alongside
	// root.go's mistyped-verb path. A serve or shutdown failure from serveAll
	// below is returned the same way and also exits 1. Return each as an
	// untyped error: never wrap it in usageErrorf, classifyOperatorErr or any
	// other error that carries an exit code.
	bound, err := listenAll(listeners)
	if err != nil {
		slog.Error("listen failed", "err", err)
		return err
	}
	listening := []any{"version", resolvedVersion(), "addr", bound[0].ln.Addr().String()}
	if len(bound) > 1 {
		listening = append(listening, "mcp_addr", bound[1].ln.Addr().String())
	}
	slog.Info("engram listening", listening...)
	return serveAll(sigCtx, bound, shutdownGrace, drain)
}

// shutdownGrace bounds how long serveAll waits for the servers to shut down
// and the queues to drain.
const shutdownGrace = 15 * time.Second

// mcpTransport is the MCP streamable-HTTP handler for srv behind the bearer
// gate (withAuth, its challenge from resource), the access log and tracing.
// A GET, the long-lived event stream, also ends when streams is done.
// runServe mounts this one value on every listener.
func mcpTransport(streams context.Context, srv *mcp.Server, chain mcpauth.TokenVerifier, resource *protectedResource, tm *telemetry.ToolMetrics) http.Handler {
	var handler http.Handler = mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv }, nil)
	handler = endGETWith(streams, handler)
	handler = withAuth(handler, chain, resource)
	handler = accessLog(tm.RecordAuthFailure, nil)(handler)
	return otelhttp.NewHandler(handler, "mcp")
}

// endGETWith cancels the context of each GET request to next when ctx is
// done. A GET to the MCP transport is the event stream, which stays open
// until its context ends and would otherwise hold http.Server.Shutdown until
// its deadline. Every other method keeps its own request context and
// finishes normally.
func endGETWith(ctx context.Context, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		reqCtx, cancel := context.WithCancel(r.Context())
		defer cancel()
		defer context.AfterFunc(ctx, cancel)()
		next.ServeHTTP(w, r.WithContext(reqCtx))
	})
}

// newListeners returns the primary listener on mux and, when
// cfg.MCPListenAddr is set, the MCP-only listener with mcpHandler and
// prmHandler (newMCPOnlyMux). Each server calls onShutdown when it begins to
// shut down.
func newListeners(cfg config.ServerConfig, mux *http.ServeMux, mcpHandler, prmHandler http.Handler, mcpPath string, onShutdown func()) []listener {
	listeners := []listener{{setting: "ENGRAM_LISTEN_ADDR", srv: newHTTPServer(cfg.ListenAddr, mux)}}
	if cfg.MCPListenAddr != "" {
		listeners = append(listeners, listener{
			setting: "ENGRAM_MCP_LISTEN_ADDR",
			srv:     newHTTPServer(cfg.MCPListenAddr, newMCPOnlyMux(mcpHandler, prmHandler, mcpPath)),
		})
	}
	for _, l := range listeners {
		l.srv.RegisterOnShutdown(onShutdown)
	}
	return listeners
}

// newHTTPServer is the http.Server for one engram listener.
// ReadHeaderTimeout guards against slowloris; IdleTimeout reclaims idle
// keep-alive connections. ReadTimeout and WriteTimeout are intentionally
// unset (0): the streamable-HTTP / SSE transport holds connections open
// indefinitely and the go-sdk never clears write deadlines, so either
// timeout would sever long-lived SSE streams.
func newHTTPServer(addr string, mux *http.ServeMux) *http.Server {
	return &http.Server{
		Addr: addr,
		// CrossOriginProtection wraps the whole mux of the listener, so
		// every route on it is covered in one place. Safe-method GET and
		// no-Origin/no-Sec-Fetch-Site traffic (the MCP transport) pass
		// Check() untouched; only cross-origin unsafe-method requests are
		// rejected, before any handler (Connect included) parses them.
		Handler:           newCrossOriginProtection().Handler(mux),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

// listener is one engram serve listener: the setting that names its
// address, and its server.
type listener struct {
	setting string
	srv     *http.Server
}

// boundListener is a listener with the socket listenAll bound for it.
type boundListener struct {
	listener
	ln net.Listener
}

// listenAll binds the address of each listener, in order. When one bind
// fails it closes the sockets already bound and returns that error, named
// with the listener's setting.
func listenAll(listeners []listener) ([]boundListener, error) {
	bound := make([]boundListener, 0, len(listeners))
	for _, l := range listeners {
		ln, err := net.Listen("tcp", l.srv.Addr)
		if err != nil {
			for _, b := range bound {
				_ = b.ln.Close()
			}
			return nil, fmt.Errorf("bind %s: %w", l.setting, err)
		}
		bound = append(bound, boundListener{listener: l, ln: ln})
	}
	return bound, nil
}

// serveAll serves each bound listener until sigCtx is done or one of them
// fails to serve. Either way it then shuts every server down in parallel
// within grace, closes the open connections of any server whose Shutdown
// runs out of grace, runs drain within the same deadline, and waits for
// every serve call to return. It returns every serve failure and every
// shutdown error, joined and named with the listener's setting, or nil
// after a clean stop.
func serveAll(sigCtx context.Context, bound []boundListener, grace time.Duration, drain func(context.Context)) error {
	serveErr := make(chan error, len(bound))
	var serving sync.WaitGroup
	for _, b := range bound {
		serving.Go(func() {
			if err := b.srv.Serve(b.ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serveErr <- fmt.Errorf("serve %s: %w", b.setting, err)
			}
		})
	}

	var errs []error
	select {
	case err := <-serveErr:
		slog.Error("listener failed; shutting down", "err", err)
		errs = append(errs, err)
	case <-sigCtx.Done():
		slog.Info("shutdown signal received; draining")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), grace)
	defer cancel()
	shutdownErrs := make([]error, len(bound))
	var shuttingDown sync.WaitGroup
	for i, b := range bound {
		shuttingDown.Go(func() {
			err := b.srv.Shutdown(shutdownCtx)
			switch {
			case err == nil:
				return
			case errors.Is(err, context.DeadlineExceeded):
				slog.Warn("listener did not shut down in time; closing its open connections", "listener", b.setting, "addr", b.srv.Addr, "err", err)
				_ = b.srv.Close()
			default:
				slog.Warn("listener socket did not close cleanly", "listener", b.setting, "addr", b.srv.Addr, "err", err)
			}
			shutdownErrs[i] = fmt.Errorf("shut down %s: %w", b.setting, err)
		})
	}
	shuttingDown.Wait()
	// Drain the summary and usage queues strictly after every Shutdown
	// returns, never in parallel, so nearly all handlers have finished
	// before either enqueue channel is closed. Necessary but not
	// sufficient: Shutdown can return at its deadline with a handler still
	// running, so both queues also guard the close against a late enqueue.
	slog.Info("draining summary and usage queues")
	drain(shutdownCtx)
	serving.Wait()
	close(serveErr)
	for err := range serveErr {
		errs = append(errs, err)
	}
	return errors.Join(append(errs, shutdownErrs...)...)
}

// checkMCPListenAddr refuses an ENGRAM_MCP_LISTEN_ADDR that is not
// host:port with a port, and one on the same non-zero TCP port as
// ENGRAM_LISTEN_ADDR where the two hosts overlap (listenHostsOverlap). An
// unparsable ENGRAM_LISTEN_ADDR is left to its own bind.
func checkMCPListenAddr(listenAddr, mcpListenAddr string) error {
	if mcpListenAddr == "" {
		return nil
	}
	mcpHost, mcpPortName, err := net.SplitHostPort(mcpListenAddr)
	if err != nil {
		return usageErrorf("ENGRAM_MCP_LISTEN_ADDR (or --mcp-listen-addr) %q: must be host:port, e.g. \":8081\": %w", mcpListenAddr, err)
	}
	if mcpPortName == "" {
		return usageErrorf("ENGRAM_MCP_LISTEN_ADDR (or --mcp-listen-addr) %q: must name a port, e.g. \":8081\"", mcpListenAddr)
	}
	mcpPort, err := net.LookupPort("tcp", mcpPortName)
	if err != nil {
		return usageErrorf("ENGRAM_MCP_LISTEN_ADDR (or --mcp-listen-addr) %q: invalid port: %w", mcpListenAddr, err)
	}
	host, portName, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return nil
	}
	port, err := net.LookupPort("tcp", portName)
	if err != nil || port == 0 || port != mcpPort {
		return nil
	}
	if listenHostsOverlap(host, mcpHost) {
		return usageErrorf("ENGRAM_MCP_LISTEN_ADDR (or --mcp-listen-addr) %q and ENGRAM_LISTEN_ADDR (or --listen-addr) %q both listen on port %d: the MCP-only listener needs its own port", mcpListenAddr, listenAddr, port)
	}
	return nil
}

// listenHostsOverlap reports whether two listen hosts on the same port can
// collide: the same host, an empty host or :: (every interface of both IP
// families), or 0.0.0.0 against an IPv4 address or a name, which can
// resolve to one.
func listenHostsOverlap(a, b string) bool {
	if a == b || a == "" || b == "" {
		return true
	}
	return unspecifiedCovers(a, b) || unspecifiedCovers(b, a)
}

// unspecifiedCovers reports whether listening on the unspecified address u
// covers host h: :: covers every host, and 0.0.0.0 covers an IPv4 address
// or a name.
func unspecifiedCovers(u, h string) bool {
	ua, err := netip.ParseAddr(u)
	if err != nil || !ua.IsUnspecified() {
		return false
	}
	if ua.Is6() {
		return true
	}
	ha, err := netip.ParseAddr(h)
	return err != nil || ha.Unmap().Is4()
}

// ownerClaimGuard fails fast if any auth lane would run with an empty
// owner-claim list. ENGRAM_OWNER_CLAIM defaults to "email" and an empty env
// var preserves that default (config.Load), so the only way to reach an empty
// list here is an explicit --owner-claim="" — but if it happens, every
// authenticated request/login fails with "missing owner claim" (
// SubjectFromTokenInfo / Authenticator.exchange), which is better caught at
// startup than discovered as a fleet-wide outage. It also warns when "email"
// is absent from the list: only a winning "email" claim gets the
// email_verified enforcement in auth.ClaimIdentity, so a different claim's
// uniqueness and verification become the operator's IdP-side responsibility.
func ownerClaimGuard(bearerIssuer string, uiEnabled bool, ownerClaims []string) error {
	if bearerIssuer == "" && !uiEnabled {
		return nil // no auth lane active; owner-claim is inert
	}
	if len(ownerClaims) == 0 {
		// D-03 bucket 1: an empty --owner-claim while an auth lane is
		// active is a configuration-validation failure.
		return usageErrorf("ENGRAM_OWNER_CLAIM (or --owner-claim) is empty while an OIDC lane is enabled: every authenticated request would fail with a missing-owner-claim error")
	}
	if !slices.Contains(ownerClaims, "email") {
		slog.Warn("owner-claim list does not include \"email\"; the email_verified enforcement only applies to a winning \"email\" claim — ensure your IdP guarantees the configured claim(s) are unique and stable",
			"owner_claims", ownerClaims)
	}
	return nil
}

// connectHeadlessGuard fails startup fast when connect.headless is set but
// no auth lane is configured (D-11): mounting Connect headless with a nil
// chain would expose every write RPC unauthenticated into the anonymous
// empty-owner bucket, on a surface that did not exist on this deployment
// before the upgrade — the same opt-in-only posture PROJECT.md requires.
// This constrains ONLY the new flag: withAuth's existing no-lane behavior
// (validation disabled, logged loudly) and the MCP lane's anonymous bucket
// are deliberately untouched, so no existing deployment can fail to boot as
// a result of this phase.
func connectHeadlessGuard(headless bool, chain mcpauth.TokenVerifier) error {
	if !headless || chain != nil {
		return nil
	}
	// D-03 bucket 1: headless requested with no auth lane configured is a
	// configuration-validation failure, not a runtime condition.
	return usageErrorf("ENGRAM_CONNECT_HEADLESS (or --connect-headless) is set while no auth lane is configured: mounting would expose every write RPC unauthenticated into the anonymous empty-owner bucket; configure ENGRAM_OIDC_ISSUER and/or ENGRAM_SERVICE_AUTH_* (client-credentials issuer/audience or static tokens) to add an auth lane")
}

// mcpListenerGuard fails startup fast when ENGRAM_MCP_LISTEN_ADDR is set
// but no auth lane is configured: the MCP-only listener exists to face a
// public ingress, and with a nil chain withAuth accepts every request into
// the anonymous empty-owner bucket. The primary listener keeps withAuth's
// no-lane behavior (validation disabled, logged loudly).
func mcpListenerGuard(mcpListenAddr string, chain mcpauth.TokenVerifier) error {
	if mcpListenAddr == "" || chain != nil {
		return nil
	}
	return usageErrorf("ENGRAM_MCP_LISTEN_ADDR (or --mcp-listen-addr) is set while no auth lane is configured: the MCP-only listener would serve every memory tool unauthenticated into the anonymous empty-owner bucket; configure ENGRAM_OIDC_ISSUER and/or ENGRAM_SERVICE_AUTH_* (client-credentials issuer/audience or static tokens) to add an auth lane")
}

// connectResolverFor decides whether to mount Connect at all, and — only
// when it does — composes the bearer and cookie halves. Mounting and
// bearer-inclusion are two SEPARATE decisions (REVIEWS.md HIGH-3); do not
// let one flag answer both:
//
//   - Mount decision: nil when cookieResolve == nil && !headless. The two
//     independent activation booleans are "the UI is on" (a non-nil
//     cookieResolve) and "the operator explicitly asked for the lane"
//     (headless). A configured chain is NEVER, on its own, an activation
//     signal — this is what keeps a deployment with an auth lane but no UI
//     and no headless flag gaining nothing on upgrade (D-12, SC5).
//   - Bearer decision: whenever the lane IS mounted, chain is passed through
//     as the bearer half UNCONDITIONALLY — never gated on headless. A
//     UI-enabled deployment that never sets connect.headless still gets
//     chain as its bearer half, because D-06 says one chain value reaches
//     both mount sites and REQ-connect-bearer-identity says a token
//     accepted on MCP is accepted on Connect everywhere Connect exists.
//
// Checked against the locked decisions: D-10 and D-12 govern mounting only
// (D-10's "defaults off independently of every ui.* and service_auth.*
// key" and D-12's "UI enabled OR connect.headless set" are both statements
// about when the lane is mounted); neither says the bearer half is
// conditional on headless, and D-06 says the opposite. There is no
// conflict to escalate.
//
// If headless is true, connectHeadlessGuard guarantees chain is non-nil
// before this is ever called in runServe; server.NewConnectResolver's own
// "both nil -> nil" rule is the defense-in-depth backstop if that guarantee
// is ever broken.
func connectResolverFor(chain mcpauth.TokenVerifier, headless bool, cookieResolve func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error)) func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error) {
	if cookieResolve == nil && !headless {
		return nil
	}
	return server.NewConnectResolver(chain, cookieResolve)
}

// buildAuthChain constructs up to three independently-enabled verifier lanes
// (D-01): the human/no-issuer lane (auth.New, iff oidc.Issuer is set), the
// client-credentials service lane (auth.NewService, iff svcAuth.OIDCIssuer
// is set), and the static-token lane (auth.NewStaticTokenVerifier, iff
// svcAuth.StaticTokens is non-empty). Each lane is built ONLY when its own
// config is present (D-03) — a human-only deployment (no service_auth.*
// config) constructs a chain containing only the human verifier, byte-for-
// byte the pre-chain behavior.
//
// This is the SOLE site where a lane verifier is constructed (D-06): the
// caller (runServe) builds the chain here exactly once and hands the same
// value to both the MCP wrapper (withAuth) and the Connect bearer half
// (connectResolverFor), so the two mount sites cannot drift. The final
// composition step is delegated to composeAuthChain, a pure function of the
// three verifiers with no config, so a test can prove the expiry wrapping
// (REVIEWS.md MED-9) without buildAuthChain's config-only signature getting
// in the way.
func buildAuthChain(oidc config.OIDCConfig, svcAuth config.ServiceAuthConfig, ownerClaims []string) (mcpauth.TokenVerifier, error) {
	var humanVerifier, serviceVerifier, staticVerifier mcpauth.TokenVerifier

	if oidc.Issuer != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		verifier, err := auth.New(ctx, oidc.Issuer, oidc.Audience, ownerClaims)
		cancel()
		if err != nil {
			// D-03 bucket 1: OIDC discovery misconfiguration.
			return nil, usageErrorf("oidc verifier init: %w", err)
		}
		humanVerifier = verifier.TokenVerifier()
		slog.Info("OIDC bearer-token validation enabled", "issuer", oidc.Issuer)
	}

	if svcAuth.OIDCIssuer != "" {
		svcOwnerClaims, err := config.ParseOwnerClaims(svcAuth.OwnerClaims)
		if err != nil {
			// D-03 bucket 1.
			return nil, usageErrorf("service-auth owner-claim config invalid: %w", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		verifier, err := auth.NewService(ctx, svcAuth.OIDCIssuer, svcAuth.OIDCAudience, svcOwnerClaims)
		cancel()
		if err != nil {
			// D-03 bucket 1: OIDC discovery misconfiguration.
			return nil, usageErrorf("service-auth oidc verifier init: %w", err)
		}
		serviceVerifier = verifier.TokenVerifier()
		slog.Info("service OIDC client-credentials validation enabled", "issuer", svcAuth.OIDCIssuer, "owner_claims", svcOwnerClaims)
	}

	if svcAuth.StaticTokens != "" {
		tokens, err := config.ParseServiceStaticTokens(svcAuth.StaticTokens)
		if err != nil {
			// D-03 bucket 1.
			return nil, usageErrorf("service-auth static-tokens config invalid: %w", err)
		}
		staticVerifier = auth.NewStaticTokenVerifier(tokens).TokenVerifier()
		slog.Info("service static-token validation enabled", "token_count", len(tokens))
	}

	return composeAuthChain(humanVerifier, serviceVerifier, staticVerifier), nil
}

// composeAuthChain is the pure composition step buildAuthChain delegates to
// (REVIEWS.md MED-9): it takes three already-built verifiers and no config,
// so a test can prove the expiry wrapping below with a stub verifier
// returning a past-Expiration TokenInfo — something buildAuthChain's
// config-only signature cannot do, since it always constructs real OIDC/
// static verifiers.
//
// Returns nil only when all three are nil — the caller decides what "no
// auth lane" means for its transport (withAuth logs and disables
// validation; connectHeadlessGuard treats it as fatal when headless is
// set). Otherwise wraps the composed auth.ChainVerifier in
// auth.EnforceExpiry (D-04), so expiry is a property of the verifier that
// every present and future lane inherits — not a property of one call
// site. composeAuthChain constructs nothing itself; D-06's "sole
// construction site" guarantee remains buildAuthChain alone.
func composeAuthChain(human, service, static mcpauth.TokenVerifier) mcpauth.TokenVerifier {
	if human == nil && service == nil && static == nil {
		return nil
	}
	return auth.EnforceExpiry(auth.ChainVerifier(human, service, static))
}

// withAuth wraps handler with bearer-token validation using the already-
// built chain (D-06): a nil chain means no auth lane configured, so the
// handler is returned unchanged after logging loudly; otherwise it is
// wrapped with mcpauth.RequireBearerToken. withAuth receives a verifier and
// takes no config, so there is no code path by which the MCP lane can
// construct a second chain — the compiler is the D-06 assertion that the
// two mount sites cannot drift.
//
// The middleware is built for each request, with the resource_metadata
// value resource gives that request. A 401 it writes without a
// WWW-Authenticate header gains a bare "Bearer" challenge, which RFC 6750
// requires: the SDK writes the header only when it has a parameter to put
// in it. Every such 401 carries Cache-Control: no-store, and a Vary on the
// forwarded headers when its value depends on them. handler writes to the
// original ResponseWriter, so its own responses are never touched.
func withAuth(handler http.Handler, chain mcpauth.TokenVerifier, resource *protectedResource) http.Handler {
	if chain == nil {
		slog.Warn("bearer-token validation DISABLED (no --oidc-issuer / ENGRAM_OIDC_ISSUER, no ENGRAM_SERVICE_AUTH_* config); all requests accepted")
		return handler
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gate := mcpauth.RequireBearerToken(chain, &mcpauth.RequireBearerTokenOptions{
			ResourceMetadataURL: resource.challengeURL(r),
		})
		inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { handler.ServeHTTP(w, r) })
		gate(inner).ServeHTTP(&challengeWriter{ResponseWriter: w, varies: resource.challengeVaries()}, r)
	})
}

// challengeWriter is the ResponseWriter of the bearer middleware. On a 401
// it adds "WWW-Authenticate: Bearer" when the header is missing, sets
// Cache-Control: no-store, and adds a Vary on the forwarded headers when
// varies is set.
type challengeWriter struct {
	http.ResponseWriter
	varies bool
}

func (w *challengeWriter) WriteHeader(code int) {
	if code == http.StatusUnauthorized {
		h := w.Header()
		if h.Get("WWW-Authenticate") == "" {
			h.Set("WWW-Authenticate", "Bearer")
		}
		h.Set("Cache-Control", "no-store")
		if w.varies {
			h.Add("Vary", "X-Forwarded-Host, X-Forwarded-Proto")
		}
	}
	w.ResponseWriter.WriteHeader(code)
}
