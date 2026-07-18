// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc/oidctest"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	"github.com/seanb4t/engram/internal/auth"
	"github.com/seanb4t/engram/internal/config"
)

// wantNamespacedOwner mirrors auth.go's DOCUMENTED namespacedOwner encoding
// (`<len(claim)>:<claim>:<len(value)>:<value>`), used as an independent test
// oracle rather than reaching into the unexported namespacedOwner helper.
func wantNamespacedOwner(claim, value string) string {
	return fmt.Sprintf("%d:%s:%d:%s", len(claim), claim, len(value), value)
}

func TestOwnerClaimGuard(t *testing.T) {
	cases := []struct {
		name         string
		bearerIssuer string
		uiEnabled    bool
		ownerClaims  []string
		wantErr      bool
	}{
		{"no auth lane, empty claim ok", "", false, nil, false},
		{"bearer lane, default claim ok", "https://issuer", false, []string{"email"}, false},
		{"bearer lane, non-default claim ok (warns, no error)", "https://issuer", false, []string{"preferred_username"}, false},
		{"bearer lane, ordered list including email ok", "https://issuer", false, []string{"email", "sub"}, false},
		{"bearer lane, empty claim rejected", "https://issuer", false, nil, true},
		{"ui lane, empty claim rejected", "", true, nil, true},
		{"ui lane, default claim ok", "", true, []string{"email"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ownerClaimGuard(tc.bearerIssuer, tc.uiEnabled, tc.ownerClaims)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ownerClaimGuard(%q, %v, %v) err=%v, wantErr=%v",
					tc.bearerIssuer, tc.uiEnabled, tc.ownerClaims, err, tc.wantErr)
			}
		})
	}
}

// TestOwnerClaimGuardEmptyFlagRejected proves the round-3 review's finding-9
// acceptance path: an explicit empty CLI flag (--owner-claim="") parses to an
// empty list via config.ParseOwnerClaims, and the guard rejects it when an
// auth lane is active. This is deliberately NOT tested via
// ENGRAM_OWNER_CLAIM="" — the env TransformFunc's empty-value guard makes an
// empty ENV var preserve the registry default ("email"), never an empty list
// (see TestLoadEmptyEnvPreservesDefault in internal/config).
func TestOwnerClaimGuardEmptyFlagRejected(t *testing.T) {
	claims, err := config.ParseOwnerClaims("") // simulates the parsed --owner-claim="" flag value
	if err != nil {
		t.Fatalf("ParseOwnerClaims(\"\"): %v", err)
	}
	if len(claims) != 0 {
		t.Fatalf("ParseOwnerClaims(\"\") = %v, want empty list", claims)
	}
	if err := ownerClaimGuard("https://issuer", false, claims); err == nil {
		t.Error("expected rejection for an explicit empty owner-claim with the bearer lane active")
	}
}

// TestOwnerClaimGuardUnsetDefaultNoWarn proves the companion half of the
// finding-9 acceptance path: an unset ENGRAM_OWNER_CLAIM resolves via the
// registry default to ["email"] and does NOT trigger the missing-"email" warn.
func TestOwnerClaimGuardUnsetDefaultNoWarn(t *testing.T) {
	claims, err := config.ParseOwnerClaims("email") // the registry's Default: "email" when unset
	if err != nil {
		t.Fatalf("ParseOwnerClaims(\"email\"): %v", err)
	}

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	if err := ownerClaimGuard("https://issuer", false, claims); err != nil {
		t.Fatalf("ownerClaimGuard: %v", err)
	}
	if strings.Contains(buf.String(), "owner-claim") {
		t.Errorf("expected no warning for the default [\"email\"] claim list, got %q", buf.String())
	}
}

// TestWithAuth_NoLaneConfigured_ReturnsHandlerUnchanged proves the SC1/D-03
// behavior-preservation guarantee: with no human OIDC issuer AND no
// service_auth.* config present, withAuth returns the handler untouched (the
// pre-chain "validation DISABLED" path) — the inner handler's response is
// observed unmodified, with no bearer-token gate in front of it.
func TestWithAuth_NoLaneConfigured_ReturnsHandlerUnchanged(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	handler, err := withAuth(inner, config.OIDCConfig{}, config.ServiceAuthConfig{}, nil)
	if err != nil {
		t.Fatalf("withAuth: %v", err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("expected the unwrapped handler to run untouched (no auth lane configured), got status %d", rec.Code)
	}
}

// newServeTestOIDCServer starts a minimal OIDC discovery+JWKS server (mirrors
// internal/auth/service_owner_failclosed_test.go's fixture) so withAuth's
// human-lane branch can perform real discovery against a live issuer.
func newServeTestOIDCServer(t *testing.T) (issuer string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	discoveryAndKeys := &oidctest.Server{
		PublicKeys: []oidctest.PublicKey{{PublicKey: key.Public(), KeyID: "serve-test-key", Algorithm: oidc.RS256}},
	}
	srv := httptest.NewServer(discoveryAndKeys)
	t.Cleanup(srv.Close)
	discoveryAndKeys.SetIssuer(srv.URL)
	return srv.URL
}

// TestWithAuth_StaticTokenLane_AuthenticatesConfiguredToken proves the
// config-string -> withAuth -> live verify seam that CR-01 broke: it sets
// svcAuth.StaticTokens to a real "owner=token" config STRING (not a
// hand-constructed map), driving it through
// config.ParseServiceStaticTokens exactly as withAuth does in production. A
// request bearing the configured token must reach the inner handler and
// resolve to the namespaced owner; an unrelated bearer must be rejected 401.
func TestWithAuth_StaticTokenLane_AuthenticatesConfiguredToken(t *testing.T) {
	var gotOwner string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ti := mcpauth.TokenInfoFromContext(r.Context()); ti != nil {
			gotOwner, _ = ti.Extra[auth.OwnerClaimExtraKey].(string)
		}
		w.WriteHeader(http.StatusTeapot)
	})
	svcAuth := config.ServiceAuthConfig{StaticTokens: "owner-x=secret-token-value"}
	handler, err := withAuth(inner, config.OIDCConfig{}, svcAuth, nil)
	if err != nil {
		t.Fatalf("withAuth: %v", err)
	}

	t.Run("configured token authenticates", func(t *testing.T) {
		gotOwner = ""
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer secret-token-value")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusTeapot {
			t.Fatalf("expected the configured token to authenticate and reach the inner handler, got status %d", rec.Code)
		}
		want := wantNamespacedOwner("static_token", "owner-x")
		if gotOwner != want {
			t.Fatalf("resolved owner = %q, want %q", gotOwner, want)
		}
	})

	t.Run("unrelated bearer rejected", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer not-the-configured-token")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected an unrelated bearer to be rejected 401, got %d", rec.Code)
		}
	})
}

// TestWithAuth_HumanOnlyConfig_RejectsUnauthenticated proves the human-only
// lane (issuer set, no service_auth.* config) constructs a chain that gates
// the inner handler behind bearer-token validation — an unauthenticated
// request is rejected 401 rather than reaching the inner handler, in
// contrast to TestWithAuth_NoLaneConfigured_ReturnsHandlerUnchanged above.
func TestWithAuth_HumanOnlyConfig_RejectsUnauthenticated(t *testing.T) {
	issuer := newServeTestOIDCServer(t)
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	handler, err := withAuth(inner, config.OIDCConfig{Issuer: issuer}, config.ServiceAuthConfig{}, []string{"email"})
	if err != nil {
		t.Fatalf("withAuth: %v", err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for an unauthenticated request against the human-only chain, got %d", rec.Code)
	}
}
