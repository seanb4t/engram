// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"testing"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/seanb4t/engram/internal/store"
)

func TestSubjectFromTokenInfo(t *testing.T) {
	// nil TokenInfo (auth disabled) -> anonymous bucket.
	if got, err := SubjectFromTokenInfo(nil); err != nil || got.Owner() != "" {
		t.Errorf("nil: got (%v, %v), want (Anonymous, nil)", got, err)
	}
	// valid sub -> authenticated.
	ti := &mcpauth.TokenInfo{Extra: map[string]any{"sub": "sub-A"}}
	if got, err := SubjectFromTokenInfo(ti); err != nil || got.Owner() != "sub-A" {
		t.Errorf("sub-A: got (%v, %v), want (Authenticated(sub-A), nil)", got, err)
	}
	// present token, missing/empty sub -> error (fail closed, never anonymous).
	if _, err := SubjectFromTokenInfo(&mcpauth.TokenInfo{Extra: map[string]any{}}); err == nil {
		t.Error("empty sub: expected error, got nil")
	}
	_ = store.Anonymous() // store import anchor
}
