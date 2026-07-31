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

	var connectResolve func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error)
	var connectCSRFVerify func(owner, token string) bool
	var connectReseal func(http.Header, *http.Request)
	var webHandler *webauth.Handler
	if uiCfg.Enabled {
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
		// The bearer half stays nil here because no verifier chain is built
		// yet at this point in serve.go (D-06's chain construction happens
		// later, inside withAuth); a later plan builds it and passes it in.
		// webauth.NewResolver(codec).Resolve is passed through unwrapped and
		// internal/webauth is not modified (D-07).
		connectResolve = server.NewConnectResolver(nil, webauth.NewResolver(codec).Resolve)
		connectCSRFVerify = csrfSigner.Verify
		connectReseal = webHandler.Reseal
		slog.Info("web UI auth lane enabled", "issuer", uiCfg.Issuer, "redirect", uiCfg.RedirectURL)
	} else {
		slog.Info("web UI disabled (headless); Connect API not mounted")
	}

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
	handler, err = withAuth(handler, cfg.OIDC, cfg.ServiceAuth, ownerClaims)
	if err != nil {
		slog.Error("oidc verifier init failed", "err", err, "issuer", cfg.OIDC.Issuer)
		return err
	}
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

// withAuth wraps the MCP handler with bearer-token validation, composing up to
// three independently-enabled lanes into a single auth.ChainVerifier (D-01):
// the human/no-issuer lane (auth.New, iff oidc.Issuer is set), the
// client-credentials service lane (auth.NewService, iff
// svcAuth.OIDCIssuer is set), and the static-token lane
// (auth.NewStaticTokenVerifier, iff svcAuth.StaticTokens is non-empty). Each
// lane is built ONLY when its own config is present (D-03) — a human-only
// deployment (no service_auth.* config) constructs a chain containing only
// the human verifier, byte-for-byte the pre-chain behavior. This is the ONE
// call site that changes for the service-auth chain (SC1).
// No lane configured → validation disabled (all requests accepted), logged loudly.
func withAuth(handler http.Handler, oidc config.OIDCConfig, svcAuth config.ServiceAuthConfig, ownerClaims []string) (http.Handler, error) {
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

	if humanVerifier == nil && serviceVerifier == nil && staticVerifier == nil {
		slog.Warn("bearer-token validation DISABLED (no --oidc-issuer / ENGRAM_OIDC_ISSUER, no ENGRAM_SERVICE_AUTH_* config); all requests accepted")
		return handler, nil
	}

	chain := auth.ChainVerifier(humanVerifier, serviceVerifier, staticVerifier)
	return mcpauth.RequireBearerToken(chain, &mcpauth.RequireBearerTokenOptions{
		ResourceMetadataURL: oidc.ResourceMetadata,
	})(handler), nil
}
