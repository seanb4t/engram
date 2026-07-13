// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"testing"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/seanb4t/engram/internal/store"
)

func TestSubjectFromTokenInfo(t *testing.T) {
	// Authenticated: owner_claim present → Authenticated(value).
	ti := &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "u1@example.com"}}
	subj, err := SubjectFromTokenInfo(ti)
	if err != nil {
		t.Fatalf("authenticated: %v", err)
	}
	if subj.Owner() != "u1@example.com" {
		t.Errorf("Owner() = %q, want u1@example.com", subj.Owner())
	}

	// Missing/empty owner_claim → fail closed.
	for name, ex := range map[string]map[string]any{
		"absent": {"sub": "x"},
		"empty":  {"owner_claim": ""},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := SubjectFromTokenInfo(&mcpauth.TokenInfo{Extra: ex}); err == nil {
				t.Error("expected fail-closed error")
			}
		})
	}

	// nil TokenInfo (auth disabled) → anonymous.
	subj, err = SubjectFromTokenInfo(nil)
	if err != nil {
		t.Fatalf("nil: %v", err)
	}
	if subj.Owner() != "" {
		t.Errorf("anonymous Owner() = %q, want \"\"", subj.Owner())
	}
	_ = store.Anonymous() // store import anchor
}

// TestCallerFromTokenInfoActorFallsBackToOwner pins landmine 3: the Connect
// cookie lane's TokenInfo carries only Extra[owner_claim], never UserID — so
// Actor must fall back to the resolved owner rather than staying empty.
func TestCallerFromTokenInfoActorFallsBackToOwner(t *testing.T) {
	c, err := callerFromTokenInfo(&mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "o@e.com"}})
	if err != nil {
		t.Fatalf("callerFromTokenInfo: %v", err)
	}
	if c.Actor == "" {
		t.Error("Actor should fall back to the resolved owner, got empty")
	}
	if c.Actor != "o@e.com" {
		t.Errorf("Actor = %q, want o@e.com (fallback to owner)", c.Actor)
	}
	if c.Subj.Owner() != "o@e.com" {
		t.Errorf("Subj.Owner() = %q, want o@e.com", c.Subj.Owner())
	}
}

// TestCallerFromTokenInfoUserIDWins pins MCP-lane parity: when UserID IS
// present (the MCP bearer lane), it wins over the owner-claim fallback.
func TestCallerFromTokenInfoUserIDWins(t *testing.T) {
	c, err := callerFromTokenInfo(&mcpauth.TokenInfo{
		UserID: "human@e.com",
		Extra:  map[string]any{"owner_claim": "o@e.com"},
	})
	if err != nil {
		t.Fatalf("callerFromTokenInfo: %v", err)
	}
	if c.Actor != "human@e.com" {
		t.Errorf("Actor = %q, want human@e.com (UserID wins)", c.Actor)
	}
}

// TestCallerFromTokenInfoAnonymousAndFailClosed pins the nil-token anonymous
// invariant (owner "", actor "", no error) and that a present token with an
// empty owner_claim still fails closed (mirrors SubjectFromTokenInfo).
func TestCallerFromTokenInfoAnonymousAndFailClosed(t *testing.T) {
	c, err := callerFromTokenInfo(nil)
	if err != nil {
		t.Fatalf("nil TokenInfo: %v", err)
	}
	if c.Subj.Owner() != "" || c.Actor != "" {
		t.Errorf("anonymous caller = %+v, want empty owner and actor", c)
	}

	if _, err := callerFromTokenInfo(&mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": ""}}); err == nil {
		t.Error("empty owner_claim on a present token should fail closed")
	}
}

// TestCallerFromConnectContextFailsClosedWhenInterceptorAbsent mirrors
// subjectFromConnectContext's fail-closed contract: the connectSubjectKey
// absent (interceptor never installed) is a programming error, not a silent
// anonymous grant.
func TestCallerFromConnectContextFailsClosedWhenInterceptorAbsent(t *testing.T) {
	if _, err := callerFromConnectContext(context.Background()); err == nil {
		t.Error("expected a fail-closed error when the interceptor key is absent")
	}
}
