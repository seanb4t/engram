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

	"github.com/coreos/go-oidc/v3/oidc"
)

func TestIdentityPrefersHumanReadable(t *testing.T) {
	tests := []struct {
		name                     string
		subject, email, username string
		want                     string
	}{
		{"email wins", "sub-123", "a@b.com", "alice", "a@b.com"},
		{"username when no email", "sub-123", "", "alice", "alice"},
		{"subject as last resort", "sub-123", "", "", "sub-123"},
		{"email beats username", "sub-123", "a@b.com", "alice", "a@b.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := identity(tt.subject, tt.email, tt.username); got != tt.want {
				t.Errorf("identity(%q,%q,%q) = %q, want %q",
					tt.subject, tt.email, tt.username, got, tt.want)
			}
		})
	}
}

// stubIDV is an idVerifier that always fails, so the rejection path can be
// exercised without a real OIDC provider.
type stubIDV struct{ err error }

func (s stubIDV) Verify(_ context.Context, _ string) (*oidc.IDToken, error) {
	return nil, s.err
}

func TestTokenVerifierLogsRejectionReason(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	v := &Verifier{idv: stubIDV{err: errors.New("bad token")}}
	_, err := v.TokenVerifier()(context.Background(), "garbage", nil)
	if err == nil {
		t.Fatal("expected rejection")
	}
	if !strings.Contains(buf.String(), "token rejected") {
		t.Errorf("expected a 'token rejected' log line, got %q", buf.String())
	}
}
