// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func testHandler(t *testing.T) *Handler {
	t.Helper()
	codec, err := NewSessionCodec(testKey())
	if err != nil {
		t.Fatal(err)
	}
	a := &Authenticator{
		clientID:    "id",
		redirectURL: "https://x/auth/callback",
		endpoint:    oauth2.Endpoint{AuthURL: "https://issuer/auth", TokenURL: "https://issuer/token"},
	}
	return NewHandler(a, codec, true /* secureCookies */)
}

func TestLoginRedirectsWithChallengeAndState(t *testing.T) {
	h := testHandler(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	h.Login(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d want 302", rec.Code)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	q := loc.Query()
	if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("missing PKCE challenge: %v", q)
	}
	if q.Get("state") == "" {
		t.Fatal("missing state")
	}
	// The flow cookie must be set so callback can recover state+verifier.
	if !strings.Contains(rec.Header().Get("Set-Cookie"), flowCookieName+"=") {
		t.Fatalf("flow cookie not set: %q", rec.Header().Get("Set-Cookie"))
	}
}
