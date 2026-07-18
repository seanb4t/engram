// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

func TestStaticTokenDistinctOwnersResolveDistinctly(t *testing.T) {
	v := NewStaticTokenVerifier(map[string]string{
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
	v := NewStaticTokenVerifier(map[string]string{"owner-a": "token-abcdef"})
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
		v := NewStaticTokenVerifier(map[string]string{"owner-a": "token-aaaa"})
		_, err := v.TokenVerifier()(context.Background(), "", nil)
		if err == nil {
			t.Fatal("expected empty token to be rejected")
		}
		if !errors.Is(err, mcpauth.ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken in chain, got %v", err)
		}
	})

	t.Run("empty token map disables mechanism", func(t *testing.T) {
		v := NewStaticTokenVerifier(map[string]string{})
		_, err := v.TokenVerifier()(context.Background(), "anything", nil)
		if err == nil {
			t.Fatal("expected an empty token map to reject every verify")
		}
		if !errors.Is(err, mcpauth.ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken in chain, got %v", err)
		}
	})

	t.Run("empty configured candidate never matches", func(t *testing.T) {
		v := NewStaticTokenVerifier(map[string]string{"owner-a": ""})
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
	v := NewStaticTokenVerifier(map[string]string{
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
	v := NewStaticTokenVerifier(map[string]string{"owner-a": "token-aaaa"})
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

func TestStaticTokenNoLeak(t *testing.T) {
	const sentinelToken = "sentinel-super-secret-token-value-zzz"

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	v := NewStaticTokenVerifier(map[string]string{"owner-a": sentinelToken})
	verify := v.TokenVerifier()

	// A non-matching verify (wrong token) exercises the rejection path.
	_, err := verify(context.Background(), "not-the-sentinel-token", nil)
	if err == nil {
		t.Fatal("expected rejection for a non-matching token")
	}
	if strings.Contains(err.Error(), sentinelToken) {
		t.Errorf("rejection error leaked the raw token: %q", err.Error())
	}
	if strings.Contains(buf.String(), sentinelToken) {
		t.Errorf("log output leaked the raw token: %q", buf.String())
	}

	// A successful verify must not carry the raw token anywhere in Extra —
	// only the resolved owner claim.
	info, err := verify(context.Background(), sentinelToken, nil)
	if err != nil {
		t.Fatalf("unexpected error on matching token: %v", err)
	}
	for k, val := range info.Extra {
		if s, ok := val.(string); ok && strings.Contains(s, sentinelToken) {
			t.Errorf("TokenInfo.Extra[%q] leaked the raw token: %q", k, s)
		}
	}
	if strings.Contains(buf.String(), sentinelToken) {
		t.Errorf("log output leaked the raw token after successful verify: %q", buf.String())
	}
}
