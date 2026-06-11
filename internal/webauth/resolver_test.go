// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"context"
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

func TestResolverValidCookieYieldsSub(t *testing.T) {
	codec, _ := NewSessionCodec(testKey())
	r := NewResolver(codec)
	sealed, _ := codec.Seal(Session{Sub: "user-9", Expiry: nowUTC().Add(time.Hour)})
	ti, err := r.Resolve(context.Background(), resolverReq(t, sealed))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if ti == nil || ti.Extra["sub"] != "user-9" {
		t.Fatalf("got %+v want sub=user-9", ti)
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
	sealed, _ := codec.Seal(Session{Sub: "u", Expiry: nowUTC().Add(-time.Minute)})
	if _, err := r.Resolve(context.Background(), resolverReq(t, sealed)); err == nil {
		t.Fatal("expected error for expired session")
	}
}

func TestResolverRejectsTamperedCookie(t *testing.T) {
	codec := mustCodec(t)
	r := NewResolver(codec)
	sealed, _ := codec.Seal(Session{Sub: "u", Expiry: nowUTC().Add(time.Hour)})
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
	sealed, _ := codec.Seal(Session{Sub: "u"})
	if _, err := r.Resolve(context.Background(), resolverReq(t, sealed)); err == nil {
		t.Fatal("expected error for zero-expiry session")
	}
}

func TestResolverRejectsEmptySub(t *testing.T) {
	codec := mustCodec(t)
	r := NewResolver(codec)
	sealed, _ := codec.Seal(Session{Sub: "", Expiry: nowUTC().Add(time.Hour)})
	if _, err := r.Resolve(context.Background(), resolverReq(t, sealed)); err == nil {
		t.Fatal("expected error for empty-sub session")
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
