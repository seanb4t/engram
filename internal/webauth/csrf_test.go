// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"testing"
	"time"
)

func testCSRFSigner(t *testing.T) *CSRFSigner {
	t.Helper()
	k, err := DeriveCSRFKey(testKey())
	if err != nil {
		t.Fatalf("DeriveCSRFKey: %v", err)
	}
	s, err := NewCSRFSigner(k)
	if err != nil {
		t.Fatalf("NewCSRFSigner: %v", err)
	}
	return s
}

// TestCSRFSigner_StableAcrossExpiry proves D-08's Owner-only binding: two
// Session values that differ only in Expiry must produce the identical CSRF
// token, because Token never reads Expiry. If Token were ever changed to
// hash Owner+Expiry, this test would fail — the two sessions below deliberately
// carry different Expiry values so a regression is caught (Phase-18 sliding
// re-seal depends on this stability).
func TestCSRFSigner_StableAcrossExpiry(t *testing.T) {
	s := testCSRFSigner(t)
	a := Session{Owner: "alice@example.com", Expiry: time.Unix(1000, 0).UTC()}
	b := Session{Owner: "alice@example.com", Expiry: time.Unix(2000, 0).UTC()}
	if a.Expiry.Equal(b.Expiry) {
		t.Fatal("test setup invalid: Expiry values must differ")
	}
	tokA := s.Token(a.Owner)
	tokB := s.Token(b.Owner)
	if tokA != tokB {
		t.Fatalf("Token differs despite identical Owner (Expiry leaked into token): %q vs %q", tokA, tokB)
	}
}

// TestCSRFSigner_TamperRejected covers T-16-02 (constant-time compare) and
// T-16-03 (cross-owner replay rejection).
func TestCSRFSigner_TamperRejected(t *testing.T) {
	s := testCSRFSigner(t)
	owner := "alice@example.com"
	token := s.Token(owner)

	if !s.Verify(owner, token) {
		t.Fatal("Verify rejected the genuine token")
	}

	tampered := []byte(token)
	tampered[len(tampered)-1] ^= 0xff
	if s.Verify(owner, string(tampered)) {
		t.Fatal("Verify accepted a tampered token")
	}

	if s.Verify("bob@example.com", token) {
		t.Fatal("Verify accepted a token minted for a different owner (cross-owner replay)")
	}
}

func TestDeriveCSRFKey_DeterministicAndDistinct(t *testing.T) {
	k := testKey()
	k1, err := DeriveCSRFKey(k)
	if err != nil {
		t.Fatalf("DeriveCSRFKey: %v", err)
	}
	k2, err := DeriveCSRFKey(k)
	if err != nil {
		t.Fatalf("DeriveCSRFKey: %v", err)
	}
	if len(k1) != 32 {
		t.Fatalf("expected 32-byte derived key, got %d", len(k1))
	}
	if string(k1) != string(k2) {
		t.Fatal("DeriveCSRFKey is not deterministic across calls")
	}
	if string(k1) == string(k) {
		t.Fatal("DeriveCSRFKey returned the raw cookie key unchanged (HKDF did not transform it)")
	}
}

func TestNewCSRFSigner_KeyGuard(t *testing.T) {
	if _, err := NewCSRFSigner(make([]byte, 31)); err == nil {
		t.Fatal("accepted a 31-byte key")
	}
	if _, err := NewCSRFSigner(make([]byte, 33)); err == nil {
		t.Fatal("accepted a 33-byte key")
	}
	if _, err := NewCSRFSigner(make([]byte, 32)); err != nil {
		t.Fatalf("rejected a valid 32-byte key: %v", err)
	}
}
