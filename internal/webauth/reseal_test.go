// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// findSetCookie parses hdr's Set-Cookie values (via a throwaway http.Response,
// the stdlib-supported way to parse response-side cookies) and returns the
// cookie named name, failing the test if absent.
func findSetCookie(t *testing.T, hdr http.Header, name string) *http.Cookie {
	t.Helper()
	resp := &http.Response{Header: hdr}
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no Set-Cookie for %q in %v", name, hdr.Values("Set-Cookie"))
	return nil
}

// TestResealNoopBeforeThreshold proves D-05: a session with remaining
// lifetime well above resealThreshold+resealSkew produces no Set-Cookie at
// all — Reseal is a no-op until the ½-TTL threshold is crossed.
func TestResealNoopBeforeThreshold(t *testing.T) {
	h := testHandler(t)
	sealed, err := h.codec.Seal(Session{Owner: "user-1", Expiry: nowUTC().Add(11 * time.Hour)})
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sealed})

	hdr := http.Header{}
	h.Reseal(hdr, req)

	if got := hdr.Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("Reseal below threshold emitted Set-Cookie headers: %v", got)
	}
}

// TestResealPastThresholdRefreshesSessionCookie proves SC1/D-06: once
// remaining lifetime drops below the threshold, Reseal re-seals the session
// cookie with a fresh ABSOLUTE nowUTC().Add(sessionTTL) expiry — never
// oldExpiry+delta.
func TestResealPastThresholdRefreshesSessionCookie(t *testing.T) {
	h := testHandler(t)
	fixedNow := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	orig := nowUTC
	nowUTC = func() time.Time { return fixedNow }
	defer func() { nowUTC = orig }()

	sealed, err := h.codec.Seal(Session{Owner: "user-1", Expiry: fixedNow.Add(1 * time.Hour)})
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sealed})

	hdr := http.Header{}
	h.Reseal(hdr, req)

	sessCookie := findSetCookie(t, hdr, sessionCookieName)
	sess, err := h.codec.Unseal(sessCookie.Value)
	if err != nil {
		t.Fatalf("Unseal resealed session cookie: %v", err)
	}
	want := fixedNow.Add(sessionTTL)
	if !sess.Expiry.Equal(want) {
		t.Fatalf("resealed Expiry = %v, want exactly %v (absolute nowUTC().Add(sessionTTL))", sess.Expiry, want)
	}
}

// TestResealPastThresholdRefreshesCSRFCookie proves D-08: the same
// past-threshold call also re-issues the engram_csrf cookie with an
// unchanged Owner-bound HMAC value (only its Max-Age refreshes).
func TestResealPastThresholdRefreshesCSRFCookie(t *testing.T) {
	h := testHandler(t)
	fixedNow := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	orig := nowUTC
	nowUTC = func() time.Time { return fixedNow }
	defer func() { nowUTC = orig }()

	owner := "user-1"
	sealed, err := h.codec.Seal(Session{Owner: owner, Expiry: fixedNow.Add(1 * time.Hour)})
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sealed})

	hdr := http.Header{}
	h.Reseal(hdr, req)

	got := hdr.Values("Set-Cookie")
	if len(got) != 2 {
		t.Fatalf("Set-Cookie count = %d, want exactly 2 (session + csrf): %v", len(got), got)
	}

	csrfCookie := findSetCookie(t, hdr, CSRFCookieName)
	want := h.signer.Token(owner)
	if csrfCookie.Value != want {
		t.Fatalf("csrf cookie value = %q, want unchanged token %q (D-08: value stable, only Max-Age refreshes)", csrfCookie.Value, want)
	}
}

// TestResealForwardMonotonicUnderConcurrency proves SC3/D-06: N goroutines
// re-sealing the SAME near-expiry cookie through a pinned nowUTC seam all
// produce a forward-monotonic (never-shortened, always-absolute) expiry.
func TestResealForwardMonotonicUnderConcurrency(t *testing.T) {
	h := testHandler(t)
	fixedNow := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	orig := nowUTC
	nowUTC = func() time.Time { return fixedNow }
	defer func() { nowUTC = orig }()

	nearExpiry := fixedNow.Add(1 * time.Hour) // well under resealThreshold (6h)
	sealed, err := h.codec.Seal(Session{Owner: "user-1", Expiry: nearExpiry})
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sealed})

	const n = 50
	var wg sync.WaitGroup
	expiries := make([]time.Time, n)
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			hdr := http.Header{}
			h.Reseal(hdr, req)
			c := findSetCookie(t, hdr, sessionCookieName)
			sess, err := h.codec.Unseal(c.Value)
			if err != nil {
				t.Errorf("goroutine %d Unseal: %v", i, err)
				return
			}
			expiries[i] = sess.Expiry
		}(i)
	}
	wg.Wait()

	want := fixedNow.Add(sessionTTL)
	for i, e := range expiries {
		if e.Before(nearExpiry) {
			t.Errorf("goroutine %d produced expiry %v BEFORE pre-reseal expiry %v (D-06 violation)", i, e, nearExpiry)
		}
		if !e.Equal(want) {
			t.Errorf("goroutine %d expiry = %v, want exactly nowUTC()+sessionTTL = %v (absolute, never a delta)", i, e, want)
		}
	}
}
