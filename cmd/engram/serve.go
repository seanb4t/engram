// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strconv"
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
		return fmt.Errorf("load config: %w", err)
	}
	// Serve-local guard: an empty listen address makes http.Server bind ":http"
	// (port 80) silently. ENGRAM_LISTEN_ADDR defaults to :8080; only an explicit
	// --listen-addr "" can empty it. Data-plane fields are validated in the store
	// path (buildDepsFromEnv -> loadAndValidate -> Config.Validate).
	if cfg.Server.ListenAddr == "" {
		return fmt.Errorf("ENGRAM_LISTEN_ADDR (or --listen-addr) is empty: a listen address is required")
	}

	telCfg := telemetry.ConfigFromEnv("engram", version)
	logger, shutdown, err := telemetry.Setup(context.Background(), telCfg)
	if err != nil {
		return fmt.Errorf("telemetry setup: %w", err)
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
		return err
	}
	// Parse the comma-list once (parsing is separate from defaulting: the
	// registry already supplies "email" when unset). A malformed list
	// (duplicate/interior-empty/bad claim name) fails startup immediately
	// rather than surfacing as a runtime auth failure.
	ownerClaims, err := config.ParseOwnerClaims(cfg.OIDC.OwnerClaim)
	if err != nil {
		slog.Error("owner-claim config invalid", "err", err)
		return err
	}
	if err := ownerClaimGuard(cfg.OIDC.Issuer, uiCfg.Enabled, ownerClaims); err != nil {
		slog.Error("owner-claim config invalid", "err", err)
		return err
	}

	// D-06: build the verifier chain exactly ONCE here. The same chain value
	// reaches the MCP wrapper (withAuth, below) and the Connect bearer half
	// (connectResolverFor), so the two mount sites cannot drift — there is one
	// instance, so there is nothing to diverge.
	chain, err := buildAuthChain(cfg.OIDC, cfg.ServiceAuth, ownerClaims)
	if err != nil {
		slog.Error("oidc verifier init failed", "err", err, "issuer", cfg.OIDC.Issuer)
		return err
	}
	headless, err := strconv.ParseBool(cfg.Connect.Headless)
	if err != nil {
		err = fmt.Errorf("ENGRAM_CONNECT_HEADLESS (or --connect-headless) %q: must be a boolean: %w", cfg.Connect.Headless, err)
		slog.Error("connect-headless config invalid", "err", err)
		return err
	}
	if err := connectHeadlessGuard(headless, chain); err != nil {
		slog.Error("connect-headless config invalid", "err", err)
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
		key, err := decodeCookieKey(uiCfg.CookieKey)
		if err != nil {
			return err
		}
		codec, err := webauth.NewSessionCodec(key)
		if err != nil {
			return fmt.Errorf("session cookie key: %w", err)
		}
		kcsrf, err := webauth.DeriveCSRFKey(key)
		if err != nil {
			return fmt.Errorf("derive csrf key: %w", err)
		}
		csrfSigner, err := webauth.NewCSRFSigner(kcsrf)
		if err != nil {
			return fmt.Errorf("csrf signer: %w", err)
		}
		oidcCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		authr, err := webauth.NewAuthenticator(oidcCtx, uiCfg.Issuer, uiCfg.ClientID, uiCfg.ClientSecret, uiCfg.RedirectURL, ownerClaims)
		cancel()
		if err != nil {
			return fmt.Errorf("web UI OIDC discovery: %w", err)
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

	srv := mcp.NewServer(&mcp.Implementation{Name: "engram", Version: version}, nil)
	drain, err := server.Register(srv, mux, tm, sqm, uqm, connectResolve, connectCSRFVerify, connectReseal)
	if err != nil {
		slog.Error("server registration failed", "err", err)
		return err
	}

	if uiCfg.Enabled {
		mux.HandleFunc("GET /auth/login", webHandler.Login)
		mux.HandleFunc("GET /auth/callback", webHandler.Callback)
		mux.HandleFunc("POST /auth/logout", webHandler.Logout)
		// The console SPA is served under "/ui/"; mountMCPRoutes below redirects a
		// browser GET of the bare root here.
		mux.Handle("/ui/", http.StripPrefix("/ui/", webauth.StaticHandler()))
	}

	resolvedMCPPath, err := resolveMCPPath(cfg.Server.MCPPath)
	if err != nil {
		slog.Error("invalid MCP path", "err", err)
		return err
	}

	var handler http.Handler = mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv }, nil)
	handler = withAuth(handler, chain, cfg.OIDC.ResourceMetadata)
	handler = accessLog(tm.RecordAuthFailure, nil)(handler)
	handler = otelhttp.NewHandler(handler, "mcp")
	// Mount the MCP transport at its configured path and give "/" to the console
	// landing / 404 handler (mcpPath="/" restores the legacy root catch-all).
	mountMCPRoutes(mux, handler, uiCfg.Enabled, resolvedMCPPath)
	slog.Info("MCP transport mounted", "path", resolvedMCPPath, "ui_enabled", uiCfg.Enabled)

	// ReadHeaderTimeout guards against slowloris; IdleTimeout reclaims idle
	// keep-alive connections. ReadTimeout and WriteTimeout are intentionally
	// unset (0): the streamable-HTTP / SSE transport holds connections open
	// indefinitely and the go-sdk never clears write deadlines, so either
	// timeout would sever long-lived SSE streams.
	httpSrv := &http.Server{
		Addr: cfg.Server.ListenAddr,
		// CrossOriginProtection wraps the fully-assembled mux (D-07 whole-
		// server wrap, verified safe): it covers the Connect handler,
		// /auth/*, /ui/, and the MCP transport in one place. Safe-method GET
		// and no-Origin/no-Sec-Fetch-Site traffic (the MCP transport) pass
		// Check() untouched; only cross-origin unsafe-method requests are
		// rejected, before Connect ever parses them.
		Handler:           newCrossOriginProtection().Handler(mux),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("engram listening", "version", version, "addr", cfg.Server.ListenAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return err
	case <-sigCtx.Done():
		slog.Info("shutdown signal received; draining")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		shutdownErr := httpSrv.Shutdown(shutdownCtx)
		// Drain the summary and usage queues STRICTLY after httpSrv.Shutdown
		// returns — never in parallel — so nearly all handlers have finished
		// before either enqueue channel is closed (D-08, RESEARCH Pitfall 1).
		// Necessary but not sufficient: Shutdown can return via deadline with a
		// handler still in flight, so both queues self-guard the close vs a
		// late enqueue (CR-01).
		slog.Info("draining summary and usage queues")
		drain(shutdownCtx)
		return shutdownErr
	}
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
		return fmt.Errorf("ENGRAM_OWNER_CLAIM (or --owner-claim) is empty while an OIDC lane is enabled: every authenticated request would fail with a missing-owner-claim error")
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
	return fmt.Errorf("ENGRAM_CONNECT_HEADLESS (or --connect-headless) is set while no auth lane is configured: mounting would expose every write RPC unauthenticated into the anonymous empty-owner bucket; configure ENGRAM_OIDC_ISSUER and/or ENGRAM_SERVICE_AUTH_* (client-credentials issuer/audience or static tokens) to add an auth lane")
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
			return nil, fmt.Errorf("oidc verifier init: %w", err)
		}
		humanVerifier = verifier.TokenVerifier()
		slog.Info("OIDC bearer-token validation enabled", "issuer", oidc.Issuer)
	}

	if svcAuth.OIDCIssuer != "" {
		svcOwnerClaims, err := config.ParseOwnerClaims(svcAuth.OwnerClaims)
		if err != nil {
			return nil, fmt.Errorf("service-auth owner-claim config invalid: %w", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		verifier, err := auth.NewService(ctx, svcAuth.OIDCIssuer, svcAuth.OIDCAudience, svcOwnerClaims)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("service-auth oidc verifier init: %w", err)
		}
		serviceVerifier = verifier.TokenVerifier()
		slog.Info("service OIDC client-credentials validation enabled", "issuer", svcAuth.OIDCIssuer, "owner_claims", svcOwnerClaims)
	}

	if svcAuth.StaticTokens != "" {
		tokens, err := config.ParseServiceStaticTokens(svcAuth.StaticTokens)
		if err != nil {
			return nil, fmt.Errorf("service-auth static-tokens config invalid: %w", err)
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
func withAuth(handler http.Handler, chain mcpauth.TokenVerifier, resourceMetadataURL string) http.Handler {
	if chain == nil {
		slog.Warn("bearer-token validation DISABLED (no --oidc-issuer / ENGRAM_OIDC_ISSUER, no ENGRAM_SERVICE_AUTH_* config); all requests accepted")
		return handler
	}
	return mcpauth.RequireBearerToken(chain, &mcpauth.RequireBearerTokenOptions{
		ResourceMetadataURL: resourceMetadataURL,
	})(handler)
}
