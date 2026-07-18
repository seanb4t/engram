// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

// jwtShapedToken/opaqueToken are canonical fixtures for the D-04 structural
// discriminator: exactly two dots routes to the OIDC branch, anything else
// routes to the static branch.
const (
	jwtShapedToken = "aaa.bbb.ccc"
	opaqueToken    = "opaque-static-token-value"
)

// countingVerifier is a stub mcpauth.TokenVerifier that records how many
// times it was invoked, so routing-isolation tests can assert a lane's
// verifier was never consulted.
type countingVerifier struct {
	calls int
	info  *mcpauth.TokenInfo
	err   error
}

func (c *countingVerifier) verify(_ context.Context, _ string, _ *http.Request) (*mcpauth.TokenInfo, error) {
	c.calls++
	return c.info, c.err
}

func okVerifier(owner string) *countingVerifier {
	return &countingVerifier{info: &mcpauth.TokenInfo{UserID: owner, Extra: map[string]any{OwnerClaimExtraKey: owner}}}
}

func failVerifier(cause error) *countingVerifier {
	return &countingVerifier{err: errors.Join(mcpauth.ErrInvalidToken, cause)}
}

var errStubShouldNeverBeCalled = errors.New("stub verifier invoked out of its lane")

func TestDiscriminator_LooksLikeJWT(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{"three segments", "a.b.c", true},
		{"opaque no dots", "abcdef", false},
		{"one dot", "a.b", false},
		{"four segments (three dots)", "a.b.c.d", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := looksLikeJWT(tt.token); got != tt.want {
				t.Errorf("looksLikeJWT(%q) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}

func TestChainVerifier_RoutesJWTToOIDCBranchOnly(t *testing.T) {
	human := okVerifier("human-owner")
	static := failVerifier(errStubShouldNeverBeCalled)

	chain := chainVerifier(human.verify, nil, static.verify)
	info, err := chain(context.Background(), jwtShapedToken, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.UserID != "human-owner" {
		t.Errorf("UserID = %q, want human-owner", info.UserID)
	}
	if human.calls != 1 {
		t.Errorf("human verifier calls = %d, want 1", human.calls)
	}
	if static.calls != 0 {
		t.Errorf("static verifier calls = %d, want 0 (must never run for a JWT-shaped bearer)", static.calls)
	}
}

func TestChainVerifier_RoutesOpaqueToStaticBranchOnly(t *testing.T) {
	human := failVerifier(errStubShouldNeverBeCalled)
	service := failVerifier(errStubShouldNeverBeCalled)
	static := okVerifier("static-owner")

	chain := chainVerifier(human.verify, service.verify, static.verify)
	info, err := chain(context.Background(), opaqueToken, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.UserID != "static-owner" {
		t.Errorf("UserID = %q, want static-owner", info.UserID)
	}
	if human.calls != 0 || service.calls != 0 {
		t.Errorf("OIDC stubs must never run for an opaque bearer: human=%d service=%d", human.calls, service.calls)
	}
	if static.calls != 1 {
		t.Errorf("static verifier calls = %d, want 1", static.calls)
	}
}

func TestChainVerifier_UnrecognizedShapeDeniesByDefault(t *testing.T) {
	chain := chainVerifier(nil, nil, nil)
	_, err := chain(context.Background(), "", nil)
	if !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}
