package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/seanb4t/engram/internal/auth"
	"github.com/seanb4t/engram/internal/server"
)

func main() {
	addr := server.EnvOr("MEM_LISTEN_ADDR", ":8080")
	srv := mcp.NewServer(&mcp.Implementation{Name: "engram", Version: "0.2.0"}, nil)
	server.Register(srv)

	var handler http.Handler = mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv }, nil)
	handler = withAuth(handler)

	log.Printf("engram listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// withAuth wraps the MCP handler with OIDC bearer-token validation when
// MEM_OIDC_ISSUER is set. The LiteLLM gateway forwards the user's Authentik
// token untouched (delegate_auth_to_upstream); we verify it and make the
// caller identity available to tool handlers for attribution. When the issuer
// is unset, validation is disabled (any request is accepted) — logged loudly
// so an unauthenticated deployment is never silent.
func withAuth(handler http.Handler) http.Handler {
	issuer := os.Getenv("MEM_OIDC_ISSUER")
	if issuer == "" {
		log.Printf("WARNING: OIDC validation DISABLED (MEM_OIDC_ISSUER unset) — all requests accepted")
		return handler
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	verifier, err := auth.New(ctx, issuer, os.Getenv("MEM_OIDC_AUDIENCE"))
	if err != nil {
		log.Fatalf("oidc verifier init: %v", err)
	}

	log.Printf("OIDC bearer-token validation enabled (issuer=%s)", issuer)
	return mcpauth.RequireBearerToken(verifier.TokenVerifier(), &mcpauth.RequireBearerTokenOptions{
		ResourceMetadataURL: os.Getenv("MEM_OIDC_RESOURCE_METADATA"),
	})(handler)
}
