// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"connectrpc.com/connect"
)

func resolverReq(t *testing.T, cookie string) connect.AnyRequest {
	t.Helper()
	req := connect.NewRequest(&struct{}{})
	if cookie != "" {
		req.Header().Set("Cookie", sessionCookieName+"="+cookie)
	}
	return req
}

func TestResolverValidCookieYieldsOwnerClaim(t *testing.T) {
	codec, _ := NewSessionCodec(testKey())
	r := NewResolver(codec)
	sealed, _ := codec.Seal(Session{Owner: "user-9", Expiry: nowUTC().Add(time.Hour)})
	ti, err := r.Resolve(context.Background(), resolverReq(t, sealed))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if ti == nil || ti.Extra["owner_claim"] != "user-9" {
		t.Fatalf("got %+v want owner_claim=user-9", ti)
	}
}

func TestResolverRejectsMissingCookie(t *testing.T) {
	r := NewResolver(mustCodec(t))
	if _, err := r.Resolve(context.Background(), resolverReq(t, "")); err == nil {
		t.Fatal("expected error for missing cookie")
	}
}

func TestResolverRejectsExpiredSession(t *testing.T) {
	codec := mustCodec(t)
	r := NewResolver(codec)
	sealed, _ := codec.Seal(Session{Owner: "u", Expiry: nowUTC().Add(-time.Minute)})
	if _, err := r.Resolve(context.Background(), resolverReq(t, sealed)); err == nil {
		t.Fatal("expected error for expired session")
	}
}

func TestResolverRejectsTamperedCookie(t *testing.T) {
	codec := mustCodec(t)
	r := NewResolver(codec)
	sealed, _ := codec.Seal(Session{Owner: "u", Expiry: nowUTC().Add(time.Hour)})
	b := []byte(sealed)
	b[len(b)-1] ^= 0xff // flip a byte of the ciphertext/tag (matches TestUnsealRejectsTamper)
	bad := string(b)
	if _, err := r.Resolve(context.Background(), resolverReq(t, bad)); err == nil {
		t.Fatal("expected error for tampered cookie")
	}
}

func TestResolverRejectsZeroExpiry(t *testing.T) {
	codec := mustCodec(t)
	r := NewResolver(codec)
	sealed, _ := codec.Seal(Session{Owner: "u"})
	if _, err := r.Resolve(context.Background(), resolverReq(t, sealed)); err == nil {
		t.Fatal("expected error for zero-expiry session")
	}
}

func TestResolverRejectsEmptyOwner(t *testing.T) {
	codec := mustCodec(t)
	r := NewResolver(codec)
	sealed, _ := codec.Seal(Session{Owner: "", Expiry: nowUTC().Add(time.Hour)})
	if _, err := r.Resolve(context.Background(), resolverReq(t, sealed)); err == nil {
		t.Fatal("expected error for empty-owner session")
	}
}

// TestResolverRejectsLegacyVersionCookie proves the rollout invalidation seam
// (T-17-14, round-2 finding 3): a cookie forged by BYPASSING Seal (sealing a
// version-0/absent payload directly via the raw sealBytes path) is rejected by
// Resolve, so no legacy bare-owner session can be forwarded into the new
// namespaced owner space during the encoding rollout. The client-facing error
// is the SAME generic string Resolve already uses for other invalid-session
// conditions and must not disclose the payload version (round-8 LOW, Codex).
func TestResolverRejectsLegacyVersionCookie(t *testing.T) {
	codec := mustCodec(t)
	r := NewResolver(codec)
	legacy, err := json.Marshal(map[string]any{
		"owner": "legacy-owner",
		"exp":   nowUTC().Add(time.Hour),
		// "v" intentionally omitted: decodes to the JSON zero value (0).
	})
	if err != nil {
		t.Fatalf("marshal legacy payload: %v", err)
	}
	sealed, err := codec.sealBytes(legacy)
	if err != nil {
		t.Fatalf("sealBytes: %v", err)
	}
	_, err = r.Resolve(context.Background(), resolverReq(t, sealed))
	if err == nil {
		t.Fatal("expected rejection of a legacy (unversioned) session cookie")
	}
	if err.Error() != "invalid session cookie" {
		t.Fatalf("error = %q, want the generic \"invalid session cookie\" (must not disclose the payload version)", err.Error())
	}
}

// TestResolveHardExpiryHasNoSkewTolerance (D-07, SC4): a session expired by
// exactly 1 nanosecond — well inside any plausible resealSkew (60s, see
// reseal.go) — must still be rejected. This pins resolver.go's hard-expiry
// check (sess.Expiry.IsZero() || nowUTC().After(sess.Expiry)) as
// byte-for-byte strict: resealSkew is scoped to Reseal's threshold
// comparison only and must NEVER be reused here (Pitfall 4). A sub-skew
// expiry catches a future "helpful" reuse of resealSkew inside Resolve that
// a 1-minute-expired case (TestResolverRejectsExpiredSession) would not.
func TestResolveHardExpiryHasNoSkewTolerance(t *testing.T) {
	codec := mustCodec(t)
	r := NewResolver(codec)
	sealed, err := codec.Seal(Session{Owner: "u", Expiry: nowUTC().Add(-1 * time.Nanosecond)})
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if _, err := r.Resolve(context.Background(), resolverReq(t, sealed)); err == nil {
		t.Fatal("Resolve accepted a session expired by 1ns — hard expiry must have ZERO skew tolerance (D-07)")
	}
}

func mustCodec(t *testing.T) *SessionCodec {
	t.Helper()
	c, err := NewSessionCodec(testKey())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNewResolverPanicsOnNilCodec(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewResolver(nil) did not panic; a nil codec must fail at construction, not first request")
		}
	}()
	NewResolver(nil)
}
