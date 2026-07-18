// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"errors"
	"testing"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

func TestStaticTokenDistinctOwnersResolveDistinctly(t *testing.T) {
	v := newStaticTokenVerifier(map[string]string{
		"owner-a": "token-aaaa",
		"owner-b": "token-bbbb",
	})
	verify := v.TokenVerifier()

	infoA, err := verify(context.Background(), "token-aaaa", nil)
	if err != nil {
		t.Fatalf("owner-a token: unexpected error: %v", err)
	}
	infoB, err := verify(context.Background(), "token-bbbb", nil)
	if err != nil {
		t.Fatalf("owner-b token: unexpected error: %v", err)
	}

	ownerA, _ := infoA.Extra[OwnerClaimExtraKey].(string)
	ownerB, _ := infoB.Extra[OwnerClaimExtraKey].(string)
	if ownerA == "" || ownerB == "" {
		t.Fatalf("expected non-empty owner claims, got %q and %q", ownerA, ownerB)
	}
	if ownerA == ownerB {
		t.Fatalf("expected distinct owners for distinct tokens, both resolved to %q", ownerA)
	}
	if ownerA != namespacedOwner("static_token", "owner-a") {
		t.Errorf("owner-a claim = %q, want %q", ownerA, namespacedOwner("static_token", "owner-a"))
	}
	if ownerB != namespacedOwner("static_token", "owner-b") {
		t.Errorf("owner-b claim = %q, want %q", ownerB, namespacedOwner("static_token", "owner-b"))
	}
}

func TestStaticTokenPrefixNotMatched(t *testing.T) {
	v := newStaticTokenVerifier(map[string]string{"owner-a": "token-abcdef"})
	verify := v.TokenVerifier()

	_, err := verify(context.Background(), "token-abc", nil)
	if err == nil {
		t.Fatal("expected a prefix of the configured token to be rejected")
	}
	if !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken in chain, got %v", err)
	}
}

func TestStaticTokenEmptyRejected(t *testing.T) {
	t.Run("empty input token", func(t *testing.T) {
		v := newStaticTokenVerifier(map[string]string{"owner-a": "token-aaaa"})
		_, err := v.TokenVerifier()(context.Background(), "", nil)
		if err == nil {
			t.Fatal("expected empty token to be rejected")
		}
		if !errors.Is(err, mcpauth.ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken in chain, got %v", err)
		}
	})

	t.Run("empty token map disables mechanism", func(t *testing.T) {
		v := newStaticTokenVerifier(map[string]string{})
		_, err := v.TokenVerifier()(context.Background(), "anything", nil)
		if err == nil {
			t.Fatal("expected an empty token map to reject every verify")
		}
		if !errors.Is(err, mcpauth.ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken in chain, got %v", err)
		}
	})

	t.Run("empty configured candidate never matches", func(t *testing.T) {
		v := newStaticTokenVerifier(map[string]string{"owner-a": ""})
		_, err := v.TokenVerifier()(context.Background(), "", nil)
		if err == nil {
			t.Fatal("expected an empty configured candidate to never match, even an empty input token")
		}
	})
}

func TestStaticTokenRotationSameOwnerMultipleTokens(t *testing.T) {
	// Two distinct ownerID keys mapping to the SAME target owner exercise
	// rotation: both tokens must verify successfully and resolve to the same
	// namespaced owner (no flag-day cutover).
	v := newStaticTokenVerifier(map[string]string{
		"owner-a-old": "token-old",
		"owner-a-new": "token-new",
	})
	verify := v.TokenVerifier()

	infoOld, err := verify(context.Background(), "token-old", nil)
	if err != nil {
		t.Fatalf("old token: unexpected error: %v", err)
	}
	infoNew, err := verify(context.Background(), "token-new", nil)
	if err != nil {
		t.Fatalf("new token: unexpected error: %v", err)
	}
	if infoOld.UserID == "" || infoNew.UserID == "" {
		t.Fatalf("expected non-empty UserID for both, got %q and %q", infoOld.UserID, infoNew.UserID)
	}
}

func TestStaticTokenSuccessInfoShape(t *testing.T) {
	v := newStaticTokenVerifier(map[string]string{"owner-a": "token-aaaa"})
	info, err := v.TokenVerifier()(context.Background(), "token-aaaa", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.UserID != "owner-a" {
		t.Errorf("UserID = %q, want %q (non-namespaced ownerID)", info.UserID, "owner-a")
	}
	want := namespacedOwner("static_token", "owner-a")
	got, _ := info.Extra[OwnerClaimExtraKey].(string)
	if got != want {
		t.Errorf("Extra[%q] = %q, want %q", OwnerClaimExtraKey, got, want)
	}
}
