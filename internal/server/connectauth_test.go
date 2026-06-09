// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"testing"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

func TestSubjectFromConnectContext(t *testing.T) {
	// injected authenticated sub.
	ctx := withConnectTokenInfo(context.Background(), &mcpauth.TokenInfo{Extra: map[string]any{"sub": "sub-A"}})
	if got, err := subjectFromConnectContext(ctx); err != nil || got.Owner() != "sub-A" {
		t.Errorf("authed: got (%v,%v), want Authenticated(sub-A)", got, err)
	}
	// no key on context (interceptor absent / anon) -> anonymous bucket.
	if got, err := subjectFromConnectContext(context.Background()); err != nil || got.Owner() != "" {
		t.Errorf("absent: got (%v,%v), want Anonymous", got, err)
	}
}
