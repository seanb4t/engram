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

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"

	"github.com/seanb4t/engram/internal/auth"
	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/telemetry"
)

var (
	listenAddr           string
	oidcIssuer           string
	oidcAudience         string
	oidcResourceMetadata string
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

	srv := mcp.NewServer(&mcp.Implementation{Name: "engram", Version: version}, nil)
	// Build MCP tools + mount the Connect API on the mux from one deps. nil
	// resolver = no cookie/OIDC lane configured => Connect API not mounted
	// (Task 10 wires a real resolver once the web-auth lane lands).
	if err := server.Register(srv, mux, tm, nil); err != nil {
		slog.Error("server registration failed", "err", err)
		return err
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
