// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
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
