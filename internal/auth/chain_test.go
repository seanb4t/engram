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

	chain := ChainVerifier(human.verify, nil, static.verify)
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

	chain := ChainVerifier(human.verify, service.verify, static.verify)
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
	chain := ChainVerifier(nil, nil, nil)
	_, err := chain(context.Background(), "", nil)
	if !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

// TestChainVerifier_HumanTriedBeforeService proves the D-02 order: when the
// human verifier succeeds, the client-credentials verifier is never
// consulted.
func TestChainVerifier_HumanTriedBeforeService(t *testing.T) {
	human := okVerifier("human-owner")
	service := failVerifier(errStubShouldNeverBeCalled)

	chain := ChainVerifier(human.verify, service.verify, nil)
	info, err := chain(context.Background(), jwtShapedToken, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.UserID != "human-owner" {
		t.Errorf("UserID = %q, want human-owner", info.UserID)
	}
	if service.calls != 0 {
		t.Errorf("service verifier must not be consulted when human succeeds, got %d calls", service.calls)
	}
}

// TestChainVerifier_ServiceTriedAfterHumanFails proves the D-02 fallback:
// only on the human verifier's failure is the service result returned.
func TestChainVerifier_ServiceTriedAfterHumanFails(t *testing.T) {
	human := failVerifier(errors.New("human rejects"))
	service := okVerifier("service-owner")

	chain := ChainVerifier(human.verify, service.verify, nil)
	info, err := chain(context.Background(), jwtShapedToken, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.UserID != "service-owner" {
		t.Errorf("UserID = %q, want service-owner", info.UserID)
	}
	if human.calls != 1 || service.calls != 1 {
		t.Errorf("expected both verifiers consulted in D-02 order: human=%d service=%d", human.calls, service.calls)
	}
}

// TestChainVerifier_NilServiceOnJWTBranchDenies proves the D-03 nil-guard: a
// JWT-shaped bearer whose human verifier fails and whose service verifier is
// nil (unconfigured) denies rather than panicking.
func TestChainVerifier_NilServiceOnJWTBranchDenies(t *testing.T) {
	human := failVerifier(errors.New("human rejects"))
	chain := ChainVerifier(human.verify, nil, nil)
	_, err := chain(context.Background(), jwtShapedToken, nil)
	if !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

// TestChainVerifier_NilStaticOnOpaqueBranchDenies proves the D-03 nil-guard
// on the static branch: an opaque bearer with no static verifier configured
// denies rather than panicking.
func TestChainVerifier_NilStaticOnOpaqueBranchDenies(t *testing.T) {
	chain := ChainVerifier(nil, nil, nil)
	_, err := chain(context.Background(), opaqueToken, nil)
	if !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

// TestChainVerifier_HumanOnlyConfigMatchesPreChainBehavior proves the
// no-issuer/human-only deployment (nil service, nil static) is byte-for-byte
// the pre-chain human-only behavior: JWT bearers verify via the human
// verifier alone, opaque bearers deny.
func TestChainVerifier_HumanOnlyConfigMatchesPreChainBehavior(t *testing.T) {
	human := okVerifier("human-owner")
	chain := ChainVerifier(human.verify, nil, nil)

	info, err := chain(context.Background(), jwtShapedToken, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.UserID != "human-owner" {
		t.Errorf("UserID = %q, want human-owner", info.UserID)
	}

	if _, err := chain(context.Background(), opaqueToken, nil); !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Fatalf("expected opaque bearer to deny with no static lane configured, got %v", err)
	}
}

// TestChainVerifier_Deterministic proves the chain is stateless: verifying
// the same token twice yields the same outcome.
func TestChainVerifier_Deterministic(t *testing.T) {
	human := okVerifier("human-owner")
	chain := ChainVerifier(human.verify, nil, nil)

	info1, err1 := chain(context.Background(), jwtShapedToken, nil)
	info2, err2 := chain(context.Background(), jwtShapedToken, nil)
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}
	if info1.UserID != info2.UserID {
		t.Errorf("non-deterministic result: %q vs %q", info1.UserID, info2.UserID)
	}
	if human.calls != 2 {
		t.Errorf("expected 2 calls (one per verification), got %d", human.calls)
	}
}
