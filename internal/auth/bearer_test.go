// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

func TestEnforceExpiry(t *testing.T) {
	stub := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{Expiration: time.Now().Add(-time.Minute)}, nil
	}
	_, err := EnforceExpiry(stub)(context.Background(), "tok", nil)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
	if !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Errorf("expected errors.Is(err, mcpauth.ErrInvalidToken), got %v", err)
	}
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected errors.Is(err, ErrTokenExpired), got %v", err)
	}
}

func TestEnforceExpiryZero(t *testing.T) {
	stub := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{}, nil // zero-value Expiration
	}
	_, err := EnforceExpiry(stub)(context.Background(), "tok", nil)
	if err == nil {
		t.Fatal("expected error for zero Expiration, got nil")
	}
	if !errors.Is(err, ErrTokenMissingExpiration) {
		t.Errorf("expected errors.Is(err, ErrTokenMissingExpiration), got %v", err)
	}
}

func TestEnforceExpiryValidPasses(t *testing.T) {
	want := &mcpauth.TokenInfo{Expiration: time.Now().Add(time.Hour)}
	stub := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return want, nil
	}
	got, err := EnforceExpiry(stub)(context.Background(), "tok", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("expected the same *TokenInfo pointer to be returned, got a different value")
	}
}

func TestEnforceExpiryPassesThroughVerifierError(t *testing.T) {
	someErr := errors.New("some verifier error")
	stub := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return nil, someErr
	}
	_, err := EnforceExpiry(stub)(context.Background(), "tok", nil)
	if !errors.Is(err, someErr) {
		t.Errorf("expected the verifier's error unwrapped/unmodified, got %v", err)
	}
}

func TestEnforceExpiryNoSkew(t *testing.T) {
	stub := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{Expiration: time.Now().Add(-time.Nanosecond)}, nil
	}
	_, err := EnforceExpiry(stub)(context.Background(), "tok", nil)
	if err == nil {
		t.Fatal("expected rejection for an Expiration one nanosecond in the past (no skew grace)")
	}
}

func TestEnforceExpiryNilTokenInfoIsForwardedNotDereferenced(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("EnforceExpiry panicked on (nil, nil) verifier return: %v", r)
		}
	}()
	stub := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return nil, nil
	}
	ti, err := EnforceExpiry(stub)(context.Background(), "tok", nil)
	if ti != nil || err != nil {
		t.Errorf("expected (nil, nil) forwarded unchanged, got (%v, %v)", ti, err)
	}
}

func TestEnforceExpiryMessagesMatchSDK(t *testing.T) {
	expiredStub := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{Expiration: time.Now().Add(-time.Minute)}, nil
	}
	_, err := EnforceExpiry(expiredStub)(context.Background(), "tok", nil)
	if err == nil || err.Error() != "token expired" {
		t.Errorf(`expected Error() == "token expired", got %v`, err)
	}
	if !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Errorf("expected errors.Is(err, mcpauth.ErrInvalidToken), got %v", err)
	}

	zeroStub := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{}, nil
	}
	_, err = EnforceExpiry(zeroStub)(context.Background(), "tok", nil)
	if err == nil || err.Error() != "token missing expiration" {
		t.Errorf(`expected Error() == "token missing expiration", got %v`, err)
	}
	if !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Errorf("expected errors.Is(err, mcpauth.ErrInvalidToken), got %v", err)
	}
}

func TestExtractBearerCredential(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantToken string
		wantOK    bool
	}{
		{"canonical", "Bearer abc", "abc", true},
		{"lowercase scheme", "bearer abc", "abc", true},
		{"uppercase scheme", "BEARER abc", "abc", true},
		{"mixed case scheme", "BeArEr abc", "abc", true},
		{"tab separator", "Bearer\tabc", "abc", true},
		{"extra whitespace", "  Bearer   abc  ", "abc", true},
		{"nbsp separator", "Bearer abc", "abc", true},
		{"non-bearer scheme", "Basic abc", "", false},
		{"bare token no scheme", "abc", "", false},
		{"bare scheme only", "Bearer", "", false},
		{"scheme with empty credential", "Bearer  ", "", false},
		{"empty header", "", "", false},
		{"multi-field credential", "Bearer a b", "", false},
		{"comma-coalesced duplicates", "Bearer a, Bearer b", "", false},
		{"invalid utf-8 credential is opaque", "Bearer \xff\xfe", "\xff\xfe", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotToken, gotOK := ExtractBearerCredential(tt.header)
			if gotToken != tt.wantToken || gotOK != tt.wantOK {
				t.Errorf("ExtractBearerCredential(%q) = (%q, %v), want (%q, %v)", tt.header, gotToken, gotOK, tt.wantToken, tt.wantOK)
			}
		})
	}
}
