// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"context"
	"testing"
	"time"
)

// TestSealAutoInjectsVersion proves Seal always stamps the current
// sessionPayloadVersion regardless of what the caller supplies (round-3
// LOW-9): a Session with V left unset round-trips through Seal/Unseal to the
// current version, and Resolve accepts it and yields the owner.
func TestSealAutoInjectsVersion(t *testing.T) {
	c, err := NewSessionCodec(testKey())
	if err != nil {
		t.Fatalf("NewSessionCodec: %v", err)
	}
	in := Session{Owner: "user-123", Expiry: nowUTC().Add(time.Hour)} // V left unset
	sealed, err := c.Seal(in)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	out, err := c.Unseal(sealed)
	if err != nil {
		t.Fatalf("Unseal: %v", err)
	}
	if out.V != sessionPayloadVersion {
		t.Fatalf("Unseal V = %d, want %d (Seal must auto-inject the current version)", out.V, sessionPayloadVersion)
	}
	ti, err := NewResolver(c).Resolve(context.Background(), resolverReq(t, sealed))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if ti.Extra["owner_claim"] != "user-123" {
		t.Fatalf("owner_claim = %v, want user-123", ti.Extra["owner_claim"])
	}
}

func testKey() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i)
	}
	return k
}

func TestSessionRoundTrip(t *testing.T) {
	c, err := NewSessionCodec(testKey())
	if err != nil {
		t.Fatalf("NewSessionCodec: %v", err)
	}
	in := Session{Owner: "user-123", Expiry: time.Unix(1000, 0).UTC()}
	sealed, err := c.Seal(in)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	out, err := c.Unseal(sealed)
	if err != nil {
		t.Fatalf("Unseal: %v", err)
	}
	if out.Owner != in.Owner || !out.Expiry.Equal(in.Expiry) {
		t.Fatalf("round-trip mismatch: %+v vs %+v", out, in)
	}
}

func TestUnsealRejectsTamper(t *testing.T) {
	c, _ := NewSessionCodec(testKey())
	sealed, _ := c.Seal(Session{Owner: "u"})
	b := []byte(sealed)
	b[len(b)-1] ^= 0xff // flip a byte of the ciphertext/tag
	if _, err := c.Unseal(string(b)); err == nil {
		t.Fatal("Unseal accepted tampered cookie")
	}
}

func TestUnsealRejectsWrongKey(t *testing.T) {
	c1, _ := NewSessionCodec(testKey())
	other := testKey()
	other[0] ^= 0xff
	c2, _ := NewSessionCodec(other)
	sealed, _ := c1.Seal(Session{Owner: "u"})
	if _, err := c2.Unseal(sealed); err == nil {
		t.Fatal("Unseal accepted cookie sealed with a different key")
	}
}

func TestNewSessionCodecRejectsBadKey(t *testing.T) {
	if _, err := NewSessionCodec([]byte("short")); err == nil {
		t.Fatal("accepted a non-32-byte key")
	}
}

// TestOldSubKeyedCookieRejected verifies that a session cookie sealed with the
// old "sub" JSON key (the pre-owner-claim cookie format) is rejected by the resolver — callers
// are forced to re-login on upgrade (expected behaviour; documented in release
// notes). The sealed cookie decodes to Session{Owner: ""} which the resolver
// rejects with "session has empty owner".
func TestOldSubKeyedCookieRejected(t *testing.T) {
	codec := mustCodec(t)
	// Manually seal a raw JSON payload that uses the old "sub" key.
	oldPayload := []byte(`{"sub":"legacy-user","exp":"2099-01-01T00:00:00Z"}`)
	sealed, err := codec.sealBytes(oldPayload)
	if err != nil {
		t.Fatalf("sealBytes: %v", err)
	}
	// Unseal: JSON unmarshals into Session{Owner: ""} because "sub" doesn't map.
	sess, err := codec.Unseal(sealed)
	if err != nil {
		t.Fatalf("Unseal: %v", err)
	}
	if sess.Owner != "" {
		t.Fatalf("expected empty Owner from old sub-keyed cookie, got %q", sess.Owner)
	}
	// End-to-end: the resolver must reject the old-format cookie (forced re-login).
	if _, err := NewResolver(codec).Resolve(context.Background(), resolverReq(t, sealed)); err == nil {
		t.Fatal("expected Resolve to reject an old sub-keyed cookie (empty owner)")
	}
}
