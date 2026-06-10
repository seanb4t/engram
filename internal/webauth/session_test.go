// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"testing"
	"time"
)

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
	in := Session{Sub: "user-123", Access: "at", Refresh: "rt", Expiry: time.Unix(1000, 0).UTC()}
	sealed, err := c.Seal(in)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	out, err := c.Unseal(sealed)
	if err != nil {
		t.Fatalf("Unseal: %v", err)
	}
	if out.Sub != in.Sub || out.Access != in.Access || out.Refresh != in.Refresh || !out.Expiry.Equal(in.Expiry) {
		t.Fatalf("round-trip mismatch: %+v vs %+v", out, in)
	}
}

func TestUnsealRejectsTamper(t *testing.T) {
	c, _ := NewSessionCodec(testKey())
	sealed, _ := c.Seal(Session{Sub: "u"})
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
	sealed, _ := c1.Seal(Session{Sub: "u"})
	if _, err := c2.Unseal(sealed); err == nil {
		t.Fatal("Unseal accepted cookie sealed with a different key")
	}
}

func TestNewSessionCodecRejectsBadKey(t *testing.T) {
	if _, err := NewSessionCodec([]byte("short")); err == nil {
		t.Fatal("accepted a non-32-byte key")
	}
}
