// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
	"github.com/seanb4t/engram/internal/auth"
	"github.com/seanb4t/engram/internal/store/storetest"
)

// slogRecord is one captured log entry: level, message, and every attribute
// (including WithAttrs-carried ones) rendered as key=value strings.
type slogRecord struct {
	level slog.Level
	msg   string
	attrs map[string]string
}

// slogRecorder is a mutex-guarded sink for slogRecorderHandler. Safe for
// concurrent Handle calls (the go-sdk and Connect stack may log from more
// than one goroutine).
type slogRecorder struct {
	mu      sync.Mutex
	records []slogRecord
}

func (r *slogRecorder) add(rec slogRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, rec)
}

// containing returns every captured record whose message OR any attribute
// value contains substr.
func (r *slogRecorder) containing(substr string) []slogRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []slogRecord
	for _, rec := range r.records {
		if strings.Contains(rec.msg, substr) {
			out = append(out, rec)
			continue
		}
		for _, v := range rec.attrs {
			if strings.Contains(v, substr) {
				out = append(out, rec)
				break
			}
		}
	}
	return out
}

// slogRecorderHandler is a slog.Handler that records every entry into a
// shared *slogRecorder rather than writing anywhere — WithAttrs returns a
// handler carrying the accumulated attrs forward (matching slog's own
// contract), WithGroup is a no-op since this test package never needs
// grouped attribute keys.
type slogRecorderHandler struct {
	rec   *slogRecorder
	attrs []slog.Attr
}

func (h *slogRecorderHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *slogRecorderHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := make(map[string]string, len(h.attrs))
	for _, a := range h.attrs {
		attrs[a.Key] = a.Value.String()
	}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.String()
		return true
	})
	h.rec.add(slogRecord{level: r.Level, msg: r.Message, attrs: attrs})
	return nil
}

func (h *slogRecorderHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	merged = append(merged, h.attrs...)
	merged = append(merged, attrs...)
	return &slogRecorderHandler{rec: h.rec, attrs: merged}
}

func (h *slogRecorderHandler) WithGroup(string) slog.Handler { return h }

// captureSlog installs a slogRecorder as slog.Default() for the duration of
// t, restoring the previous default in t.Cleanup — this is the Connect
// lane's end-to-end proof of D-06's "logged raw, exactly once" requirement
// (rule m45p2b4bp7: we assert OUR log line, never grpc-go's own default).
// Tests using this helper must not call t.Parallel() (mutates the process
// global slog default).
func captureSlog(t *testing.T) *slogRecorder {
	t.Helper()
	rec := &slogRecorder{}
	prev := slog.Default()
	slog.SetDefault(slog.New(&slogRecorderHandler{rec: rec}))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return rec
}

// noASCIIDigit reports whether s contains no ASCII digit — used to assert
// the wire envelope never carries a byte ceiling.
func noASCIIDigit(s string) bool {
	return !regexp.MustCompile(`[0-9]`).MatchString(s)
}

// TestConnectListMemoriesResponseTooLarge is the Connect lane's end-to-end
// proof at the named limit (rule m45p2b4bp7): a real 4 MiB-named overflow
// through Store.List, reached over real HTTP via the production
// mountConnect interceptor chain, surfaces to a Connect client as
// resource_exhausted carrying the ONE shared envelope — never internal, and
// never a byte count or upstream grpc/Qdrant text. This is why no
// internal/e2e binary test is added for this scenario: engram serve dials
// Qdrant with no named receive limit until Phase 5's
// REQ-recv-limit-backstop, so a binary-level overflow would rest on
// grpc-go's own default, which this project's own tests must never assert
// against (rule m45p2b4bp7).
func TestConnectListMemoriesResponseTooLarge(t *testing.T) {
	d, st := testDepsWithStore(t)
	fx := storetest.SeedOversized(t, st, storetest.Spec{
		Limit:  storetest.RecvLimit,
		Shape:  storetest.FewLarge,
		Vector: []float32{0.1, 0.2, 0.3},
	})

	resolve := func(_ context.Context, _ connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error) {
		return &mcpauth.TokenInfo{Extra: map[string]any{auth.OwnerClaimExtraKey: fx.Owner}}, auth.LaneBearer, nil
	}
	csrfVerify := func(_, _ string) bool { return true }

	mux := http.NewServeMux()
	if err := d.mountConnect(mux, resolve, csrfVerify, nil); err != nil {
		t.Fatalf("mountConnect: %v", err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)

	rec := captureSlog(t)

	resp, err := client.ListMemories(context.Background(), connect.NewRequest(&engramv1.ListMemoriesRequest{
		Scope: fx.Scope,
		Limit: 0, // all — one full-payload Scroll, exactly the shape that overflows
	}))
	if resp != nil {
		t.Fatalf("ListMemories: got a non-nil response, want nil (the overflow must never look like a success)")
	}
	if err == nil {
		t.Fatal("ListMemories: got nil error, want a resource_exhausted error")
	}
	if code := connect.CodeOf(err); code != connect.CodeResourceExhausted {
		t.Fatalf("ListMemories: code = %v, want CodeResourceExhausted", code)
	}
	var cerr *connect.Error
	if !errors.As(err, &cerr) {
		t.Fatalf("ListMemories: err %v does not unwrap to *connect.Error", err)
	}
	msg := cerr.Message()
	if want := responseTooLargeEnvelope(); msg != want {
		t.Fatalf("ListMemories: message = %q, want %q", msg, want)
	}
	if !strings.HasPrefix(msg, "field=response hint=too_large: ") {
		t.Fatalf("ListMemories: message %q does not start with the field/hint envelope prefix", msg)
	}
	for _, banned := range []string{"grpc", "larger than max", "qdrant", "Scroll"} {
		if strings.Contains(msg, banned) {
			t.Errorf("ListMemories: message %q leaks banned substring %q", msg, banned)
		}
	}
	if !noASCIIDigit(msg) {
		t.Errorf("ListMemories: message %q contains an ASCII digit (a byte ceiling must never reach the wire)", msg)
	}

	larger := rec.containing("larger than max")
	if len(larger) != 1 {
		t.Fatalf("captured log records containing %q: got %d, want exactly 1 (log records: %+v)", "larger than max", len(larger), rec.records)
	}
	if larger[0].level != slog.LevelError {
		t.Errorf("the one raw-error log record has level %v, want ERROR", larger[0].level)
	}
	found := false
	for _, v := range larger[0].attrs {
		if strings.Contains(v, "/qdrant.Points/Scroll") {
			found = true
			break
		}
	}
	if !found && !strings.Contains(larger[0].msg, "/qdrant.Points/Scroll") {
		t.Errorf("the one raw-error log record does not contain the RPC method /qdrant.Points/Scroll: %+v", larger[0])
	}
}
