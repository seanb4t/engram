// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc/oidctest"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

const failClosedTestKeyID = "fc-test-key"

// newFailClosedTestFixture starts a minimal OIDC discovery+JWKS server
// (mirrors internal/webauth/oidc_exchange_test.go's newFakeOIDCServer) and
// returns a real *oidc.IDTokenVerifier plus a signer for arbitrary claims, so
// these tests exercise the actual TokenVerifier/ClaimIdentity code path
// against a genuinely signature-verified token rather than a hand-rolled
// stub.
func newFailClosedTestFixture(t *testing.T) (v idVerifier, sign func(claims map[string]any) string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	discoveryAndKeys := &oidctest.Server{
		PublicKeys: []oidctest.PublicKey{{PublicKey: key.Public(), KeyID: failClosedTestKeyID, Algorithm: oidc.RS256}},
	}
	srv := httptest.NewServer(discoveryAndKeys)
	t.Cleanup(srv.Close)
	discoveryAndKeys.SetIssuer(srv.URL)

	provider, err := oidc.NewProvider(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("oidc discovery: %v", err)
	}
	idv := provider.Verifier(&oidc.Config{SkipClientIDCheck: true})

	sign = func(claims map[string]any) string {
		full := map[string]any{
			"iss": srv.URL,
			"sub": "test-subject",
			"exp": time.Now().Add(time.Hour).Unix(),
		}
		for k, val := range claims {
			full[k] = val
		}
		raw, err := json.Marshal(full)
		if err != nil {
			t.Fatalf("marshal claims: %v", err)
		}
		return oidctest.SignIDToken(key, failClosedTestKeyID, oidc.RS256, string(raw))
	}
	return idv, sign
}

// TestFailClosedRejectsEmptyOwner is the D-10 FIRST proof: a failClosed
// (service-lane) Verifier whose configured owner claims (client_id, azp) are
// both absent from an otherwise-valid token is rejected AT THE VERIFIER
// BOUNDARY — errors.Is(err, mcpauth.ErrInvalidToken) and a nil TokenInfo —
// never a TokenInfo carrying an empty owner (SC2/D-08/D-10).
func TestFailClosedRejectsEmptyOwner(t *testing.T) {
	idv, sign := newFailClosedTestFixture(t)
	v := &Verifier{idv: idv, ownerClaims: []string{"client_id", "azp"}, failClosed: true}

	token := sign(map[string]any{}) // no email, client_id, or azp
	info, err := v.TokenVerifier()(context.Background(), token, nil)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Errorf("err = %v, want errors.Is(err, mcpauth.ErrInvalidToken)", err)
	}
	if info != nil {
		t.Errorf("info = %+v, want nil (reject at the verifier boundary, not a TokenInfo with owner==\"\")", info)
	}
}

// TestFailClosedResolvesClientCredentialsOwner proves a client-credentials-
// shaped claims map (client_id present, no email) resolves through the
// existing ClaimIdentity to a non-empty namespaced owner using the service
// owner-claim order (D-05), and that the same map flows through a failClosed
// Verifier to a TokenInfo carrying that owner.
func TestFailClosedResolvesClientCredentialsOwner(t *testing.T) {
	claims := map[string]any{"client_id": "svc-foo"}
	ownerClaims := []string{"client_id", "azp"}

	owner, _, _, err := ClaimIdentity(claims, ownerClaims)
	if err != nil {
		t.Fatalf("ClaimIdentity: %v", err)
	}
	want := namespacedOwner("client_id", "svc-foo")
	if owner != want {
		t.Fatalf("ClaimIdentity owner = %q, want %q", owner, want)
	}

	idv, sign := newFailClosedTestFixture(t)
	v := &Verifier{idv: idv, ownerClaims: ownerClaims, failClosed: true}

	token := sign(claims)
	info, err := v.TokenVerifier()(context.Background(), token, nil)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	got, _ := info.Extra[OwnerClaimExtraKey].(string)
	if got != want {
		t.Errorf("Extra[owner_claim] = %q, want %q", got, want)
	}
}

// TestFailClosedDoesNotAffectHumanLane is the D-08 behavior-preservation
// check: a Verifier with failClosed=false (the human/no-issuer lane) that
// resolves an empty owner still returns a TokenInfo (nil error) with an empty
// Extra[owner_claim] — the existing fail-open-to-anonymous path is unchanged
// by the service-lane reject.
func TestFailClosedDoesNotAffectHumanLane(t *testing.T) {
	idv, sign := newFailClosedTestFixture(t)
	v := &Verifier{idv: idv, ownerClaims: []string{"client_id", "azp"}, failClosed: false}

	token := sign(map[string]any{}) // no email, client_id, or azp
	info, err := v.TokenVerifier()(context.Background(), token, nil)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if info == nil {
		t.Fatal("want a non-nil TokenInfo (fail-open lane)")
	}
	if got, _ := info.Extra[OwnerClaimExtraKey].(string); got != "" {
		t.Errorf("Extra[owner_claim] = %q, want empty (fail-open behavior unchanged)", got)
	}
}
