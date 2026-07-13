// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc/oidctest"
)

const testKeyID = "test-key"

// fakeOIDCOpts controls the failure modes a fake token endpoint can inject,
// so tests can exercise exchange()'s error paths without a real IdP.
type fakeOIDCOpts struct {
	tokenEndpointFails bool // token endpoint returns 400 instead of a token
	omitIDToken        bool // token response has no id_token field
}

// newFakeOIDCServer starts a minimal OIDC provider backed by a fresh RSA key.
// Discovery and JWKS are served by go-oidc's own oidctest.Server (the
// upstream library's test-support package for exactly this purpose); only the
// token endpoint is hand-written, since oidctest has no code-exchange support
// to fake. Every /token call returns the same signed ID token built from
// claims (merged over iss/aud/exp/sub defaults), which is all a single
// exchange() call under test needs.
func newFakeOIDCServer(t *testing.T, clientID string, claims map[string]any, opts fakeOIDCOpts) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	discoveryAndKeys := &oidctest.Server{
		PublicKeys: []oidctest.PublicKey{{PublicKey: key.Public(), KeyID: testKeyID, Algorithm: oidc.RS256}},
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	issuer := srv.URL
	discoveryAndKeys.SetIssuer(issuer)

	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		if opts.tokenEndpointFails {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}

		full := map[string]any{
			"iss": issuer,
			"aud": clientID,
			"exp": time.Now().Add(time.Hour).Unix(),
			"sub": "test-subject",
		}
		maps.Copy(full, claims)
		rawClaims, err := json.Marshal(full)
		if err != nil {
			t.Fatalf("marshal claims: %v", err)
		}

		resp := map[string]any{
			"access_token": "test-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		}
		if !opts.omitIDToken {
			resp["id_token"] = oidctest.SignIDToken(key, testKeyID, oidc.RS256, string(rawClaims))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.Handle("/", discoveryAndKeys)

	return issuer
}

// TestAuthenticatorExchange covers exchange()'s owner-claim extraction and
// empty-owner guard end to end (discovery -> code exchange -> ID-token
// verify -> ClaimIdentity) — the flow handlers_test.go's Callback-path tests
// intentionally stub around by injecting a pre-built Authenticator.
func TestAuthenticatorExchange(t *testing.T) {
	const clientID = "test-client"
	// namespacedOwner mirrors auth.namespacedOwner's unexported length-prefixed
	// encoding so this cross-package test can compute the expected owner for a
	// non-email winning claim without exporting the encoder.
	namespacedOwner := func(claim, value string) string {
		return fmt.Sprintf("%d:%s:%d:%s", len(claim), claim, len(value), value)
	}
	cases := []struct {
		name          string
		ownerClaims   []string
		claims        map[string]any
		opts          fakeOIDCOpts
		wantOwner     string
		wantErrSubstr string
	}{
		{
			name:        "default email claim, verified",
			ownerClaims: []string{"email"},
			claims:      map[string]any{"email": "user@example.com", "email_verified": true},
			wantOwner:   "user@example.com",
		},
		{
			name:          "email not verified is rejected",
			ownerClaims:   []string{"email"},
			claims:        map[string]any{"email": "user@example.com", "email_verified": false},
			wantErrSubstr: "email not verified",
		},
		{
			name:        "non-email claim skips email_verified enforcement",
			ownerClaims: []string{"preferred_username"},
			claims:      map[string]any{"preferred_username": "alice", "email": "alice@example.com", "email_verified": false},
			wantOwner:   namespacedOwner("preferred_username", "alice"),
		},
		{
			name:          "empty owner-claim value is rejected (empty-owner guard)",
			ownerClaims:   []string{"preferred_username"},
			claims:        map[string]any{"email": "user@example.com", "email_verified": true},
			wantErrSubstr: `missing owner claim(s) [preferred_username]`,
		},
		{
			name:          "token endpoint failure is wrapped",
			ownerClaims:   []string{"email"},
			claims:        map[string]any{"email": "user@example.com", "email_verified": true},
			opts:          fakeOIDCOpts{tokenEndpointFails: true},
			wantErrSubstr: "code exchange",
		},
		{
			name:          "missing id_token in token response",
			ownerClaims:   []string{"email"},
			claims:        map[string]any{"email": "user@example.com", "email_verified": true},
			opts:          fakeOIDCOpts{omitIDToken: true},
			wantErrSubstr: "missing id_token",
		},
		{
			name:        "ordered fallback: email absent, sub present resolves to encoded sub owner",
			ownerClaims: []string{"email", "sub"},
			claims:      map[string]any{"sub": "svc-1"},
			wantOwner:   namespacedOwner("sub", "svc-1"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issuer := newFakeOIDCServer(t, clientID, tc.claims, tc.opts)
			ctx := context.Background()
			a, err := NewAuthenticator(ctx, issuer, clientID, "secret", "https://engram.example/auth/callback", tc.ownerClaims)
			if err != nil {
				t.Fatalf("NewAuthenticator: %v", err)
			}

			tok, owner, err := a.exchange(ctx, "test-code", "test-verifier")
			if tc.wantErrSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrSubstr) {
					t.Fatalf("exchange() err = %v, want substring %q", err, tc.wantErrSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("exchange(): %v", err)
			}
			if owner != tc.wantOwner {
				t.Errorf("owner = %q, want %q", owner, tc.wantOwner)
			}
			// exchange() also returns the raw *oauth2.Token (reserved for the
			// future write phase — see Authenticator.oauthConfig's
			// offline_access comment); assert it round-trips the fake token
			// endpoint's response so that contract stays covered too.
			if tok.AccessToken != "test-access-token" {
				t.Errorf("AccessToken = %q, want %q", tok.AccessToken, "test-access-token")
			}
		})
	}
}
