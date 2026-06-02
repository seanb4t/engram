// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"log"
	"net/http"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/auth"
	"github.com/seanb4t/engram/internal/server"
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
	srv := mcp.NewServer(&mcp.Implementation{Name: "engram", Version: version}, nil)
	server.Register(srv)

	var handler http.Handler = mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv }, nil)
	handler = withAuth(handler)

	log.Printf("engram %s listening on %s", version, listenAddr)
	return http.ListenAndServe(listenAddr, handler)
}

// withAuth wraps the MCP handler with OIDC bearer-token validation when an
// issuer is configured. LiteLLM forwards the user's Authentik token untouched
// (delegate_auth_to_upstream); engram verifies it and exposes the caller
// identity to tool handlers for attribution. No issuer → validation disabled
// (all requests accepted), logged loudly so it is never silently open.
func withAuth(handler http.Handler) http.Handler {
	if oidcIssuer == "" {
		log.Printf("WARNING: OIDC validation DISABLED (no --oidc-issuer / MEM_OIDC_ISSUER) — all requests accepted")
		return handler
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	verifier, err := auth.New(ctx, oidcIssuer, oidcAudience)
	cancel()
	if err != nil {
		log.Fatalf("oidc verifier init: %v", err)
	}

	log.Printf("OIDC bearer-token validation enabled (issuer=%s)", oidcIssuer)
	return mcpauth.RequireBearerToken(verifier.TokenVerifier(), &mcpauth.RequireBearerTokenOptions{
		ResourceMetadataURL: oidcResourceMetadata,
	})(handler)
}
