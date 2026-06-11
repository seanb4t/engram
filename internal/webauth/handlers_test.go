// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"encoding/json"
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

func TestNewHandlerPanicsOnNilDeps(t *testing.T) {
	cases := map[string]struct {
		auth  *Authenticator
		codec *SessionCodec
	}{
		"nil auth":  {nil, mustCodec(t)},
		"nil codec": {&Authenticator{}, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("NewHandler did not panic on a nil dependency; nil must fail at construction, not first request")
				}
			}()
			NewHandler(tc.auth, tc.codec, true)
		})
	}
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

func TestLogoutClearsSessionCookie(t *testing.T) {
	h := testHandler(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	h.Logout(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204", rec.Code)
	}
	sc := rec.Header().Get("Set-Cookie")
	if !strings.Contains(sc, sessionCookieName+"=") || !strings.Contains(sc, "Max-Age=0") {
		t.Fatalf("session cookie not cleared: %q", sc)
	}
}

func TestCallbackRejectsMissingFlowCookie(t *testing.T) {
	h := testHandler(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=x&state=y", nil)
	h.Callback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 (no flow cookie)", rec.Code)
	}
}

func TestCallbackRejectsStateMismatch(t *testing.T) {
	h := testHandler(t)
	// Seal a flow cookie with state "good".
	fs, _ := json.Marshal(flowState{State: "good", Verifier: "v"})
	sealed, _ := h.codec.sealBytes(fs)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=x&state=evil", nil)
	req.AddCookie(&http.Cookie{Name: flowCookieName, Value: sealed})
	h.Callback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 (state mismatch)", rec.Code)
	}
}

func TestCallbackRejectsMissingCode(t *testing.T) {
	h := testHandler(t)
	// Valid flow cookie with matching state but no code param.
	fs, _ := json.Marshal(flowState{State: "good", Verifier: "v"})
	sealed, _ := h.codec.sealBytes(fs)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state=good", nil)
	req.AddCookie(&http.Cookie{Name: flowCookieName, Value: sealed})
	h.Callback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 (missing code)", rec.Code)
	}
}

func TestCallbackRejectsCorruptFlowCookie(t *testing.T) {
	h := testHandler(t)
	// Seal a valid flow cookie, then corrupt a byte so unseal fails closed.
	fs, _ := json.Marshal(flowState{State: "good", Verifier: "v"})
	sealed, _ := h.codec.sealBytes(fs)
	// Flip the last base64url char to a different valid one so the value stays
	// cookie-safe but the ciphertext/tag no longer authenticates (fails closed).
	b := []byte(sealed)
	if b[len(b)-1] == 'A' {
		b[len(b)-1] = 'B'
	} else {
		b[len(b)-1] = 'A'
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=x&state=good", nil)
	req.AddCookie(&http.Cookie{Name: flowCookieName, Value: string(b)})
	h.Callback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 (corrupt flow cookie)", rec.Code)
	}
}
