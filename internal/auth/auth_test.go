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

func TestClaimIdentity(t *testing.T) {
	owner, email, user, err := ClaimIdentity(map[string]any{
		"email": "u1@example.com", "email_verified": true, "preferred_username": "u1",
	}, []string{"email"})
	if err != nil || owner != "u1@example.com" || email != "u1@example.com" || user != "u1" {
		t.Fatalf("verified email: owner=%q email=%q user=%q err=%v", owner, email, user, err)
	}
	for name, raw := range map[string]map[string]any{
		"explicit false": {"email": "u@e.com", "email_verified": false},
		"absent":         {"email": "u@e.com"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, _, err := ClaimIdentity(raw, []string{"email"}); err == nil {
				t.Error("expected rejection when email_verified is not true")
			}
		})
	}
	owner, _, _, err = ClaimIdentity(map[string]any{"preferred_username": "alice"}, []string{"preferred_username"})
	if err != nil || owner != namespacedOwner("preferred_username", "alice") {
		t.Fatalf("custom claim: owner=%q err=%v, want %q", owner, err, namespacedOwner("preferred_username", "alice"))
	}
	// email KEY entirely absent under a single-item ["email"] list: absence is
	// eligible to fall through (same as any other claim); with no further
	// claims to try, the list is exhausted and this resolves to owner "" with
	// a nil error (fail-closed preserved — the caller treats an empty owner
	// as fatal), NOT a ClaimIdentity-level rejection.
	if owner, _, _, err = ClaimIdentity(map[string]any{"preferred_username": "x"}, []string{"email"}); err != nil || owner != "" {
		t.Errorf("email absent, exhausted list: owner=%q err=%v, want \"\",nil", owner, err)
	}
	owner, _, _, err = ClaimIdentity(map[string]any{}, []string{"some_claim"})
	if err != nil || owner != "" {
		t.Fatalf("missing non-email claim: owner=%q err=%v, want \"\",nil", owner, err)
	}
}

// TestClaimIdentityD05UnverifiedEmailNeverFallsThrough pins D-05: a present,
// non-empty, but unverified email must reject outright and must never fall
// through to a later claim in the ordered list.
func TestClaimIdentityD05UnverifiedEmailNeverFallsThrough(t *testing.T) {
	owner, _, _, err := ClaimIdentity(map[string]any{
		"email": "u@e.com", "email_verified": false, "sub": "svc-1",
	}, []string{"email", "sub"})
	if err == nil {
		t.Fatal("expected rejection for unverified email, got nil error")
	}
	if owner != "" {
		t.Errorf("owner = %q, want \"\" (must not fall through to encoded sub owner)", owner)
	}
}

// TestNamespacedOwnerInjectivity pins D-06: the length-prefixed encoding must
// be provably injective, in particular over the two review-flagged colliding
// pairs that WOULD collide under the ambiguous "claim:value" form.
func TestNamespacedOwnerInjectivity(t *testing.T) {
	a := namespacedOwner("sub", "x:y")
	b := namespacedOwner("sub:x", "y")
	if a == b {
		t.Fatalf("collision: encode(sub,%q) == encode(sub:x,%q) == %q", "x:y", "y", a)
	}

	// A second adversarial pair with embedded colons in the value only.
	c := namespacedOwner("sub", "a:b:c")
	d := namespacedOwner("sub", "a:b")
	if c == d {
		t.Fatalf("collision: encode(sub,%q) == encode(sub,%q) == %q", "a:b:c", "a:b", c)
	}

	owner, _, _, err := ClaimIdentity(map[string]any{"sub": "x:y"}, []string{"sub"})
	if err != nil {
		t.Fatalf("ClaimIdentity: %v", err)
	}
	if owner != a {
		t.Errorf("ClaimIdentity owner = %q, want %q", owner, a)
	}
}

// TestNamespacedOwnerUnicodeInjectivity pins the byte-vs-rune behavior of the
// length-prefixed encoder against a future rune-based refactor (round-8 LOW,
// Codex): Go's len() counts bytes, so the length prefix is a byte count, and
// injectivity must hold for multi-byte (Unicode) values too.
func TestNamespacedOwnerUnicodeInjectivity(t *testing.T) {
	multiByte := namespacedOwner("sub", "café") // "café", 5 bytes ("caf" + 2-byte é)
	if got, want := len("café"), 5; got != want {
		t.Fatalf("sanity: len(caf\\u00e9) = %d, want %d bytes", got, want)
	}
	crafted := namespacedOwner("sub", "caf\xc3")
	if multiByte == crafted {
		t.Fatalf("collision between multi-byte value and crafted near-collision: %q", multiByte)
	}
	if !strings.HasPrefix(multiByte, "3:sub:5:") {
		t.Errorf("owner = %q, want byte-length prefix 5 for caf\\u00e9 (3-byte c/a/f + 2-byte é)", multiByte)
	}
}

// TestClaimIdentityReservedNamespaceEmailGuard pins T-17-08: an email value
// that matches the reserved namespace grammar ^[0-9]+: must be rejected, not
// written bare (it could otherwise impersonate a namespaced service owner).
func TestClaimIdentityReservedNamespaceEmailGuard(t *testing.T) {
	owner, _, _, err := ClaimIdentity(map[string]any{
		"email": "3:sub:1:x", "email_verified": true,
	}, []string{"email"})
	if err == nil {
		t.Fatal("expected rejection for email in reserved namespace, got nil error")
	}
	if owner != "" {
		t.Errorf("owner = %q, want \"\"", owner)
	}
}

// TestClaimIdentityEmailSubPresenceTable pins round-3 HIGH-1: email presence
// is checked separately from string conversion under an ordered [email, sub]
// list, so a malformed present email rejects instead of silently falling
// through to a different authz bucket (the encoded sub owner).
func TestClaimIdentityEmailSubPresenceTable(t *testing.T) {
	subOwner := namespacedOwner("sub", "svc-1")
	tests := []struct {
		name      string
		raw       map[string]any
		wantErr   bool
		wantOwner string
	}{
		{"non-string email (number) rejects", map[string]any{"email": 42, "sub": "svc-1"}, true, ""},
		{"non-string email (object) rejects", map[string]any{"email": map[string]any{"x": 1}, "sub": "svc-1"}, true, ""},
		{"null email (present, nil) rejects", map[string]any{"email": nil, "sub": "svc-1"}, true, ""},
		{"unverified non-empty email rejects", map[string]any{"email": "u@e.com", "email_verified": false, "sub": "svc-1"}, true, ""},
		{"absent email falls through to sub", map[string]any{"sub": "svc-1"}, false, subOwner},
		{"empty string email falls through to sub", map[string]any{"email": "", "sub": "svc-1"}, false, subOwner},
		{"empty string email with email_verified:false still falls through (round-5 LOW widening)", map[string]any{"email": "", "email_verified": false, "sub": "svc-1"}, false, subOwner},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, _, _, err := ClaimIdentity(tt.raw, []string{"email", "sub"})
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected rejection, got nil error")
				}
				if owner != "" {
					t.Errorf("owner = %q, want \"\" on rejection (must not fall through to encoded sub owner)", owner)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", owner, tt.wantOwner)
			}
		})
	}
}

// TestClaimIdentitySubClientIDPresenceTable pins round-4 HIGH-1: the
// malformed-claim reject generalizes to every ordered claim, not just email —
// a present-but-non-string sub must reject rather than falling through to a
// different authz bucket (the encoded client_id owner).
func TestClaimIdentitySubClientIDPresenceTable(t *testing.T) {
	clientOwner := namespacedOwner("client_id", "app42")
	tests := []struct {
		name      string
		raw       map[string]any
		wantErr   bool
		wantOwner string
	}{
		{"non-string sub (number) rejects", map[string]any{"sub": 7, "client_id": "app42"}, true, ""},
		{"non-string sub (object) rejects", map[string]any{"sub": map[string]any{"x": 1}, "client_id": "app42"}, true, ""},
		{"null sub (present, nil) rejects", map[string]any{"sub": nil, "client_id": "app42"}, true, ""},
		{"absent sub falls through to client_id", map[string]any{"client_id": "app42"}, false, clientOwner},
		{"empty string sub falls through to client_id", map[string]any{"sub": "", "client_id": "app42"}, false, clientOwner},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, _, _, err := ClaimIdentity(tt.raw, []string{"sub", "client_id"})
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected rejection, got nil error")
				}
				if owner != "" {
					t.Errorf("owner = %q, want \"\" on rejection (must not fall through to encoded client_id owner)", owner)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", owner, tt.wantOwner)
			}
		})
	}
}

// TestClaimIdentitySingleClaimEmailNonStringRejects pins round-8 MED (grok):
// under a SINGLE-claim ["email"] list (no fall-through target), a
// present-but-non-string email with email_verified:true must reject — a
// deliberate behavior change from the legacy owner="" + nil-error result.
func TestClaimIdentitySingleClaimEmailNonStringRejects(t *testing.T) {
	tests := []struct {
		name string
		raw  map[string]any
	}{
		{"number", map[string]any{"email": 1, "email_verified": true}},
		{"object", map[string]any{"email": map[string]any{"x": 1}, "email_verified": true}},
		{"array", map[string]any{"email": []any{"a"}, "email_verified": true}},
		{"null", map[string]any{"email": nil, "email_verified": true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, _, _, err := ClaimIdentity(tt.raw, []string{"email"})
			if err == nil {
				t.Fatal("expected rejection (new behavior), got nil error")
			}
			if owner != "" {
				t.Errorf("owner = %q, want \"\"", owner)
			}
		})
	}
}

func TestTokenVerifierStampsOwnerClaimKey(t *testing.T) {
	v := &Verifier{idv: fakeIDV{tok: &oidc.IDToken{Subject: "user-1"}}, ownerClaims: nil}
	info, err := v.TokenVerifier()(context.Background(), "tok", nil)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if _, ok := info.Extra["owner_claim"]; !ok {
		t.Error("Extra must carry an owner_claim key")
	}
	if info.Extra["sub"] != "user-1" {
		t.Errorf("sub = %v, want user-1 (preserved)", info.Extra["sub"])
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
