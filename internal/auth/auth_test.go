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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
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

type fakeIDV struct {
	tok *oidc.IDToken
	err error
}

func (f fakeIDV) Verify(_ context.Context, _ string) (*oidc.IDToken, error) {
	return f.tok, f.err
}

func recorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	prevTP := otel.GetTracerProvider()
	prevTracer := tracer
	otel.SetTracerProvider(tp)
	tracer = tp.Tracer("github.com/seanb4t/engram/internal/auth")
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		tracer = prevTracer
	})
	return sr
}

func TestTokenVerifierSpanSuccess(t *testing.T) {
	sr := recorder(t)
	v := &Verifier{idv: fakeIDV{tok: &oidc.IDToken{Subject: "user-1"}}}
	info, err := v.TokenVerifier()(context.Background(), "tok", nil)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if info.Extra["sub"] != "user-1" {
		t.Errorf("sub = %v, want user-1", info.Extra["sub"])
	}
	sp := sr.Ended()
	if len(sp) != 1 || sp[0].Name() != "auth.VerifyToken" {
		t.Fatalf("want auth.VerifyToken span, got %v", sp)
	}
	if got := attr(sp[0], "engram.auth.outcome"); got != "ok" {
		t.Errorf("outcome = %q, want ok", got)
	}
}

func TestTokenVerifierSpanError(t *testing.T) {
	sr := recorder(t)
	v := &Verifier{idv: fakeIDV{err: errors.New("bad token")}}
	_, err := v.TokenVerifier()(context.Background(), "tok", nil)
	if err == nil {
		t.Fatal("want error")
	}
	sp := sr.Ended()
	if len(sp) != 1 {
		t.Fatalf("want 1 span, got %d", len(sp))
	}
	if sp[0].Status().Code != codes.Error {
		t.Errorf("status = %v, want Error", sp[0].Status().Code)
	}
	if got := attr(sp[0], "engram.auth.outcome"); got != "error" {
		t.Errorf("outcome = %q, want error", got)
	}
}

func attr(s sdktrace.ReadOnlySpan, key string) string {
	for _, kv := range s.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.String()
		}
	}
	return ""
}
