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
	"syscall"
	"time"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"

	"github.com/seanb4t/engram/internal/auth"
	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/telemetry"
	"github.com/seanb4t/engram/internal/webauth"
)

var (
	listenAddr           string
	oidcIssuer           string
	oidcAudience         string
	oidcResourceMetadata string

	uiEnabled      string
	uiClientID     string
	uiClientSecret string
	uiRedirectURL  string
	uiCookieKey    string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the engram MCP server",
	RunE: func(_ *cobra.Command, _ []string) error {
		return runServe()
	},
}

// Flag defaults come from the MEM_* env vars (set by the Helm chart), so env
// drives config and flags override — matching holomush's cobra+env approach
// (no viper).
func init() {
	f := serveCmd.Flags()
	f.StringVar(&listenAddr, "listen-addr", server.EnvOr("MEM_LISTEN_ADDR", ":8080"),
		"listen address")
	f.StringVar(&oidcIssuer, "oidc-issuer", server.EnvOr("MEM_OIDC_ISSUER", ""),
		"OIDC issuer URL; setting it enables bearer-token enforcement")
	f.StringVar(&oidcAudience, "oidc-audience", server.EnvOr("MEM_OIDC_AUDIENCE", ""),
		"expected OIDC audience (optional)")
	f.StringVar(&oidcResourceMetadata, "oidc-resource-metadata", server.EnvOr("MEM_OIDC_RESOURCE_METADATA", ""),
		"WWW-Authenticate resource metadata URL (optional)")
	f.StringVar(&uiEnabled, "ui-enabled", server.EnvOr("MEM_UI_ENABLED", ""),
		"enable the web UI + login lane (empty=imply from creds; 'false'=hard off)")
	f.StringVar(&uiClientID, "oidc-client-id", server.EnvOr("MEM_OIDC_CLIENT_ID", ""),
		"OIDC confidential-client ID for the web login")
	f.StringVar(&uiClientSecret, "oidc-client-secret", server.EnvOr("MEM_OIDC_CLIENT_SECRET", ""),
		"OIDC client secret for the web login")
	f.StringVar(&uiRedirectURL, "ui-redirect-url", server.EnvOr("MEM_UI_REDIRECT_URL", ""),
		"OIDC auth-code callback URL")
	f.StringVar(&uiCookieKey, "ui-cookie-key", server.EnvOr("MEM_UI_COOKIE_KEY", ""),
		"32-byte AES-GCM key sealing the session cookie")
}

func runServe() error {
	cfg := telemetry.ConfigFromEnv("engram", version)
	logger, shutdown, err := telemetry.Setup(context.Background(), cfg)
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

	mux := http.NewServeMux()

	uiCfg, err := resolveUIConfig(func(k string) string {
		switch k {
		case "MEM_UI_ENABLED":
			return uiEnabled
		case "MEM_OIDC_CLIENT_ID":
			return uiClientID
		case "MEM_OIDC_CLIENT_SECRET":
			return uiClientSecret
		case "MEM_UI_REDIRECT_URL":
			return uiRedirectURL
		case "MEM_UI_COOKIE_KEY":
			return uiCookieKey
		default:
			return ""
		}
	})
	if err != nil {
		slog.Error("web UI config invalid", "err", err)
		return err
	}

	var connectResolve func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error)
	var webHandler *webauth.Handler
	if uiCfg.Enabled {
		if oidcIssuer == "" {
			return fmt.Errorf("web UI enabled but no --oidc-issuer / MEM_OIDC_ISSUER: the login lane needs an issuer")
		}
		key, err := decodeCookieKey(uiCfg.CookieKey)
		if err != nil {
			return err
		}
		codec, err := webauth.NewSessionCodec(key)
		if err != nil {
			return fmt.Errorf("session cookie key: %w", err)
		}
		oidcCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		authr, err := webauth.NewAuthenticator(oidcCtx, oidcIssuer, uiCfg.ClientID, uiCfg.ClientSecret, uiCfg.RedirectURL)
		cancel()
		if err != nil {
			return fmt.Errorf("web UI OIDC discovery: %w", err)
		}
		webHandler = webauth.NewHandler(authr, codec, true)
		connectResolve = webauth.NewResolver(codec).Resolve
		slog.Info("web UI auth lane enabled", "issuer", oidcIssuer, "redirect", uiCfg.RedirectURL)
	} else {
		slog.Info("web UI disabled (headless); Connect API not mounted")
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: "engram", Version: version}, nil)
	if err := server.Register(srv, mux, tm, connectResolve); err != nil {
		slog.Error("server registration failed", "err", err)
		return err
	}

	if uiCfg.Enabled {
		mux.HandleFunc("GET /auth/login", webHandler.Login)
		mux.HandleFunc("GET /auth/callback", webHandler.Callback)
		mux.HandleFunc("POST /auth/logout", webHandler.Logout)
		// Static SPA is the fallback for non-API, non-auth routes. Registered
		// last and only when enabled; the MCP handler still owns "/" below for
		// the streamable transport, so static is mounted under "/ui/".
		mux.Handle("/ui/", http.StripPrefix("/ui/", webauth.StaticHandler()))
	}

	var handler http.Handler = mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv }, nil)
	handler, err = withAuth(handler)
	if err != nil {
		slog.Error("oidc verifier init failed", "err", err, "issuer", oidcIssuer)
		return err
	}
	handler = accessLog(tm.RecordAuthFailure, nil)(handler)
	handler = otelhttp.NewHandler(handler, "mcp")
	mux.Handle("/", handler) // MCP streamable transport stays the root catch-all

	// ReadHeaderTimeout guards against slowloris; IdleTimeout reclaims idle
	// keep-alive connections. ReadTimeout and WriteTimeout are intentionally
	// unset (0): the streamable-HTTP / SSE transport holds connections open
	// indefinitely and the go-sdk never clears write deadlines, so either
	// timeout would sever long-lived SSE streams.
	httpSrv := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("engram listening", "version", version, "addr", listenAddr)
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
		return httpSrv.Shutdown(shutdownCtx)
	}
}

// withAuth wraps the MCP handler with OIDC bearer-token validation when an
// issuer is configured. LiteLLM forwards the user's Authentik token untouched
// (delegate_auth_to_upstream); engram verifies it and exposes the caller
// identity to tool handlers for attribution. No issuer → validation disabled
// (all requests accepted), logged loudly so it is never silently open.
// Returns an error on verifier-init failure so the deferred telemetry shutdown
// in runServe runs before the process exits (no buffered OTLP records are lost).
func withAuth(handler http.Handler) (http.Handler, error) {
	if oidcIssuer == "" {
		slog.Warn("OIDC validation DISABLED (no --oidc-issuer / MEM_OIDC_ISSUER); all requests accepted")
		return handler, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	verifier, err := auth.New(ctx, oidcIssuer, oidcAudience)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("oidc verifier init: %w", err)
	}

	slog.Info("OIDC bearer-token validation enabled", "issuer", oidcIssuer)
	return mcpauth.RequireBearerToken(verifier.TokenVerifier(), &mcpauth.RequireBearerTokenOptions{
		ResourceMetadataURL: oidcResourceMetadata,
	})(handler), nil
}
