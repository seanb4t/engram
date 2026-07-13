// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
)

// This file closes WR-02: it proves the single most load-bearing seam of the
// Phase-18 sliding re-seal feature — that connect-go actually flushes a
// unary interceptor's resp.Header() Set-Cookie writes onto the wire — using a
// REAL (non-nil, non-spy) codec-backed resealFunc, mounted over a real
// httptest.NewServer alongside the rest of the interceptor chain.
//
// internal/server deliberately does not import internal/webauth (see
// CSRFCookieName's comment atop connectcsrf.go), so testSessionCodec below is
// a small, independent, real AES-256-GCM seal/unseal pair — same algorithm
// and payload shape ({owner, exp, v}) as webauth.SessionCodec — used only to
// drive a resealFunc that performs the identical unseal -> guard ->
// threshold-check -> reseal -> Set-Cookie flow Handler.Reseal performs in
// internal/webauth/reseal.go. It is not a spy: it does real encryption and a
// real threshold decision.

var errSealedTooShort = errors.New("sealed value too short")

// testSessionPayload mirrors webauth.Session's wire shape.
type testSessionPayload struct {
	Owner string    `json:"owner"`
	Exp   time.Time `json:"exp"`
	V     int       `json:"v"`
}

const (
	testSessionPayloadVersion = 1
	testResealThreshold       = 6 * time.Hour
	testResealSkew            = 60 * time.Second
	testSessionTTL            = 12 * time.Hour
)

type testSessionCodec struct{ aead cipher.AEAD }

func newTestSessionCodec(t *testing.T) *testSessionCodec {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("cipher.NewGCM: %v", err)
	}
	return &testSessionCodec{aead: aead}
}

func (c *testSessionCodec) sealPayload(p testSessionPayload) (string, error) {
	p.V = testSessionPayloadVersion
	plain, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(c.aead.Seal(nonce, nonce, plain, nil)), nil
}

func (c *testSessionCodec) unsealPayload(v string) (testSessionPayload, error) {
	raw, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return testSessionPayload{}, err
	}
	ns := c.aead.NonceSize()
	if len(raw) < ns {
		return testSessionPayload{}, errSealedTooShort
	}
	plain, err := c.aead.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return testSessionPayload{}, err
	}
	var p testSessionPayload
	if err := json.Unmarshal(plain, &p); err != nil {
		return testSessionPayload{}, err
	}
	return p, nil
}

func mustSealTestPayload(t *testing.T, codec *testSessionCodec, p testSessionPayload) string {
	t.Helper()
	sealed, err := codec.sealPayload(p)
	if err != nil {
		t.Fatalf("sealPayload: %v", err)
	}
	return sealed
}

// newTestResealFunc returns a resealFunc backed by REAL AES-GCM
// seal/unseal — not a spy — performing the same unseal -> guard ->
// threshold -> reseal -> Set-Cookie flow as webauth.Handler.Reseal.
func newTestResealFunc(codec *testSessionCodec) resealFunc {
	return func(h http.Header, r *http.Request) {
		c, err := r.Cookie(sessionCookieNameForTest)
		if err != nil {
			return
		}
		sess, err := codec.unsealPayload(c.Value)
		if err != nil {
			return
		}
		if sess.V != testSessionPayloadVersion || sess.Owner == "" {
			return
		}
		remaining := sess.Exp.Sub(time.Now().UTC())
		if remaining <= 0 || remaining >= testResealThreshold+testResealSkew {
			return
		}
		freshExp := time.Now().UTC().Add(testSessionTTL)
		sealed, err := codec.sealPayload(testSessionPayload{Owner: sess.Owner, Exp: freshExp})
		if err != nil {
			return
		}
		h.Add("Set-Cookie", (&http.Cookie{
			Name:     sessionCookieNameForTest,
			Value:    sealed,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int(testSessionTTL.Seconds()),
		}).String())
		h.Add("Set-Cookie", (&http.Cookie{
			Name:     CSRFCookieName,
			Value:    "test-csrf-token-" + sess.Owner,
			Path:     "/",
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int(testSessionTTL.Seconds()),
		}).String())
	}
}

// TestConnectResealSetCookieReachesWire proves WR-02: mounts the REAL
// interceptor chain (subject -> reseal, innermost) over httptest.NewServer
// with a real codec-backed resealFunc (not nil, not a spy), seeds a
// near-expiry session cookie (past the 6h threshold) on a ListMemories
// request, and asserts the raw wire response actually carries refreshed
// engram_session and engram_csrf Set-Cookie headers with a fresh absolute
// expiry. If connect-go ever dropped or relocated an interceptor's
// resp.Header() writes on a unary response, this test — and only this
// test — would catch it; every other reseal test operates below the wire.
func TestConnectResealSetCookieReachesWire(t *testing.T) {
	d := testDeps(t) // Qdrant-backed; skip-gated like TestConnectCookieLaneIsolation
	ctx := context.Background()

	codec := newTestSessionCodec(t)
	owner := "wire-actor"
	nearExpiry := time.Now().UTC().Add(1 * time.Hour) // < resealThreshold (6h), > 0
	sealedCookie := mustSealTestPayload(t, codec, testSessionPayload{Owner: owner, Exp: nearExpiry})

	resolve := func(_ context.Context, _ connect.AnyRequest) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": owner}}, nil
	}

	mux := http.NewServeMux()
	// ListMemories is a read RPC, never CSRF-gated (SC3) — nil csrfVerify is fine.
	if err := d.mountConnect(mux, resolve, nil, newTestResealFunc(codec)); err != nil {
		t.Fatalf("mountConnect: %v", err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)

	req := connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: "wire-test:project:actor"})
	req.Header().Set("Cookie", sessionCookieNameForTest+"="+sealedCookie)

	resp, err := client.ListMemories(ctx, req)
	if err != nil {
		t.Fatalf("ListMemories: %v", err)
	}

	setCookies := resp.Header().Values("Set-Cookie")
	if len(setCookies) == 0 {
		t.Fatal("no Set-Cookie header on the wire response — the interceptor's resp.Header() write never reached the client")
	}

	fakeResp := &http.Response{Header: resp.Header()}
	var gotSession, gotCSRF *http.Cookie
	for _, c := range fakeResp.Cookies() {
		switch c.Name {
		case sessionCookieNameForTest:
			gotSession = c
		case CSRFCookieName:
			gotCSRF = c
		}
	}
	if gotSession == nil {
		t.Fatalf("no refreshed %s Set-Cookie reached the wire: %v", sessionCookieNameForTest, setCookies)
	}
	if gotCSRF == nil {
		t.Fatalf("no refreshed %s Set-Cookie reached the wire: %v", CSRFCookieName, setCookies)
	}

	sess, err := codec.unsealPayload(gotSession.Value)
	if err != nil {
		t.Fatalf("unseal refreshed session cookie: %v", err)
	}
	if sess.Owner != owner {
		t.Fatalf("refreshed session owner = %q, want %q", sess.Owner, owner)
	}
	if !sess.Exp.After(nearExpiry) {
		t.Fatalf("refreshed session expiry %v did not advance past pre-reseal expiry %v", sess.Exp, nearExpiry)
	}
	if time.Until(sess.Exp) < testResealThreshold {
		t.Fatalf("refreshed session expiry %v is not a fresh ~%s absolute expiry", sess.Exp, testSessionTTL)
	}
}
