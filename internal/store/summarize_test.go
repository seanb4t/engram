// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestUpdateSummaryPersists verifies that Store.Update with a non-nil summary
// writes Summary and SummarySource="client", and that passing summary=&""
// clears Summary, SummarySource, and SummaryModel. Integration: needs Qdrant.
func TestUpdateSummaryPersists(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	id := "b0000000-0000-0000-0000-000000000001"
	m := Memory{
		ID: id, Content: "original content", Scope: "repo:summary-update",
		Category: "convention", Source: "agent-inferred", Owner: "owner-B",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() { _ = s.Delete(ctx, id, Authenticated("owner-B")) })

	// Case 1: non-nil summary persists with SummarySource="client".
	sum := "concise summary"
	if err := s.Update(ctx, m, m.Content, nil, nil, &sum, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("Update with summary: %v", err)
	}
	got, err := s.GetReadable(ctx, id, Authenticated("owner-B"))
	if err != nil {
		t.Fatalf("GetReadable after Update: %v", err)
	}
	if got.Summary != "concise summary" || got.SummarySource != SummarySourceClient {
		t.Fatalf("summary not persisted: Summary=%q SummarySource=%q", got.Summary, got.SummarySource)
	}

	// Case 2: summary=&"" clears Summary, SummarySource, and SummaryModel.
	empty := ""
	if err := s.Update(ctx, got, got.Content, nil, nil, &empty, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("Update clear summary: %v", err)
	}
	cleared, err := s.GetReadable(ctx, id, Authenticated("owner-B"))
	if err != nil {
		t.Fatalf("GetReadable after clear: %v", err)
	}
	if cleared.Summary != "" || cleared.SummarySource != "" || cleared.SummaryModel != "" {
		t.Fatalf("summary not cleared: Summary=%q SummarySource=%q SummaryModel=%q",
			cleared.Summary, cleared.SummarySource, cleared.SummaryModel)
	}
}

// TestUpdateClearsSummaryEgressAt (k1oe.2) asserts Update invalidates the
// auto-egress stamp whenever a client sets or clears the summary — the stamp is
// auto-egress provenance only, so a client-authored/cleared summary must zero
// it. Without this, removing the clear in Update leaves every test green.
// Integration: needs Qdrant.
func TestUpdateClearsSummaryEgressAt(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	id := "b0000000-0000-0000-0000-0000000000f1"
	stamped := time.Now().UTC().Add(-time.Hour)
	m := Memory{
		ID: id, Content: "original content", Scope: "repo:egress-clear",
		Category: "convention", Source: "agent-inferred", Owner: "owner-F",
		Summary: "auto", SummarySource: SummarySourceAuto, SummaryEgressAt: stamped,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() { _ = s.Delete(ctx, id, Authenticated("owner-F")) })

	got, err := s.GetReadable(ctx, id, Authenticated("owner-F"))
	if err != nil {
		t.Fatalf("GetReadable seeded: %v", err)
	}
	if got.SummaryEgressAt.IsZero() {
		t.Fatalf("seeded stamp not persisted: %+v", got)
	}

	// A client-authored summary invalidates the auto-egress stamp.
	clientSum := "client rewrite"
	if err := s.Update(ctx, got, got.Content, nil, nil, &clientSum, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("Update client summary: %v", err)
	}
	after, err := s.GetReadable(ctx, id, Authenticated("owner-F"))
	if err != nil {
		t.Fatalf("GetReadable after client set: %v", err)
	}
	if !after.SummaryEgressAt.IsZero() {
		t.Fatalf("client summary must clear summary_egress_at: %v", after.SummaryEgressAt)
	}

	// Re-stamp, then a summary clear must also zero it.
	if err := s.SetSummary(ctx, id, "auto again", "m"); err != nil {
		t.Fatalf("SetSummary re-stamp: %v", err)
	}
	empty := ""
	if err := s.Update(ctx, after, after.Content, nil, nil, &empty, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("Update clear summary: %v", err)
	}
	cleared, err := s.GetReadable(ctx, id, Authenticated("owner-F"))
	if err != nil {
		t.Fatalf("GetReadable after clear: %v", err)
	}
	if !cleared.SummaryEgressAt.IsZero() {
		t.Fatalf("clearing summary must clear summary_egress_at: %v", cleared.SummaryEgressAt)
	}
}

// TestSummarizeMissingDryRun verifies that DryRun:true counts eligible records
// as Filled (would-fill) without writing — GetReadable must still show an empty
// Summary afterward. Integration: needs Qdrant.
func TestSummarizeMissingDryRun(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "repo:summary-dryrun"
	long := "this is a long body well over the tiny cap used by the test harness here"

	m := Memory{
		ID: "c0000000-0000-0000-0000-000000000001", Content: long, Scope: scope,
		Category: "convention", Source: "agent-inferred", Owner: "owner-C",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Cleanup(func() { _ = s.Delete(ctx, m.ID, Authenticated("owner-C")) })

	fn := func(_ context.Context, _ string) (string, error) { return "WOULD-FILL", nil }
	res, err := s.SummarizeMissing(ctx, SummarizeOptions{
		Scope: scope, MaxChars: 8, Model: "test", DryRun: true,
	}, fn)
	if err != nil {
		t.Fatalf("DryRun SummarizeMissing: %v", err)
	}
	if res.Filled < 1 {
		t.Fatalf("DryRun: expected Filled>=1 (would-fill count), got %+v", res)
	}

	// No write must have occurred: Summary remains empty.
	got, err := s.GetReadable(ctx, m.ID, Authenticated("owner-C"))
	if err != nil {
		t.Fatalf("GetReadable: %v", err)
	}
	if got.Summary != "" {
		t.Errorf("DryRun must not write: Summary=%q, want empty", got.Summary)
	}
}

func TestShouldSummarize(t *testing.T) {
	cases := []struct {
		name     string
		m        Memory
		maxChars int
		want     bool
	}{
		{"long no-summary", Memory{Content: "abcdefghij"}, 4, true},
		{"already summarized", Memory{Content: "abcdefghij", Summary: "x"}, 4, false},
		{"too short", Memory{Content: "abc"}, 4, false},
		{"exactly cap", Memory{Content: "abcd"}, 4, false},
	}
	for _, tc := range cases {
		if got := shouldSummarize(tc.m, tc.maxChars); got != tc.want {
			t.Errorf("%s: shouldSummarize=%v want %v", tc.name, got, tc.want)
		}
	}
}

func TestFillSummarySkipsWhenNotEligible(t *testing.T) {
	// A short record must not call the summarizer or touch Qdrant (nil store is
	// safe precisely because shouldSummarize short-circuits first).
	called := false
	fn := func(_ context.Context, _ string) (string, error) { called = true; return "x", nil }
	var s *Store
	filled, err := s.FillSummary(context.Background(), Memory{Content: "abc"}, fn, "model", 4)
	if err != nil || filled || called {
		t.Fatalf("expected no-op: filled=%v called=%v err=%v", filled, called, err)
	}
}

func TestSummarizeMissingFillsEmptyOnly(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "repo:summary-it"
	long := "this is a long body well over the tiny cap used by the test harness here"

	seed := []Memory{
		{ID: "a0000000-0000-0000-0000-000000000001", Content: long, Scope: scope, Category: "convention", Source: "agent-inferred", Owner: "owner-A", CreatedAt: time.Now().UTC()},
		{ID: "a0000000-0000-0000-0000-000000000002", Content: long, Scope: scope, Category: "convention", Source: "agent-inferred", Owner: "owner-A", Summary: "already", SummarySource: SummarySourceClient, CreatedAt: time.Now().UTC()},
		{ID: "a0000000-0000-0000-0000-000000000003", Content: "short", Scope: scope, Category: "convention", Source: "agent-inferred", Owner: "owner-A", CreatedAt: time.Now().UTC()},
	}
	for _, m := range seed {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}

	fn := func(_ context.Context, _ string) (string, error) { return "AUTO terse", nil }
	res, err := s.SummarizeMissing(ctx, SummarizeOptions{Scope: scope, MaxChars: 8, Model: "summary-cheap"}, fn)
	if err != nil {
		t.Fatalf("SummarizeMissing: %v", err)
	}
	if res.Filled != 1 || res.Skipped != 2 {
		t.Fatalf("tally: filled=%d skipped=%d failed=%d scanned=%d", res.Filled, res.Skipped, res.Failed, res.Scanned)
	}
	got, err := s.GetReadable(ctx, seed[0].ID, Authenticated("owner-A"))
	if err != nil {
		t.Fatalf("get filled: %v", err)
	}
	if got.Summary != "AUTO terse" || got.SummarySource != SummarySourceAuto || got.SummaryModel != "summary-cheap" {
		t.Fatalf("auto summary not persisted: %+v", got)
	}
}

// TestSummarizeMissingStampsEgressAt asserts the k1oe.2 durable audit trail: a
// record whose content is egressed to the summarizer gets a summary_egress_at
// timestamp (store-only payload; not on the Connect wire). A record that was
// skipped (already summarized) is never egressed and gets no stamp.
// Integration: needs Qdrant.
func TestSummarizeMissingStampsEgressAt(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "repo:summary-egress"
	long := "this is a long body well over the tiny cap used by the test harness here"

	filled := Memory{
		ID: "d0000000-0000-0000-0000-000000000001", Content: long, Scope: scope,
		Category: "convention", Source: "agent-inferred", Owner: "owner-D",
		CreatedAt: time.Now().UTC(),
	}
	already := Memory{
		ID: "d0000000-0000-0000-0000-000000000002", Content: long, Scope: scope,
		Category: "convention", Source: "agent-inferred", Owner: "owner-D",
		Summary: "already", SummarySource: SummarySourceClient, CreatedAt: time.Now().UTC(),
	}
	for _, m := range []Memory{filled, already} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}
	t.Cleanup(func() {
		_ = s.Delete(ctx, filled.ID, Authenticated("owner-D"))
		_ = s.Delete(ctx, already.ID, Authenticated("owner-D"))
	})

	fn := func(context.Context, string) (string, error) { return "AUTO", nil }
	if _, err := s.SummarizeMissing(ctx, SummarizeOptions{Scope: scope, MaxChars: 8, Model: "m"}, fn); err != nil {
		t.Fatalf("SummarizeMissing: %v", err)
	}
	got, err := s.GetReadable(ctx, filled.ID, Authenticated("owner-D"))
	if err != nil {
		t.Fatalf("GetReadable filled: %v", err)
	}
	if got.SummaryEgressAt.IsZero() {
		t.Fatalf("filled record missing summary_egress_at stamp: %+v", got)
	}
	got2, err := s.GetReadable(ctx, already.ID, Authenticated("owner-D"))
	if err != nil {
		t.Fatalf("GetReadable already: %v", err)
	}
	if !got2.SummaryEgressAt.IsZero() {
		t.Fatalf("already-summarized record must not be egress-stamped: %v", got2.SummaryEgressAt)
	}
}

// TestSummarizeMissingEmitsEgressAuditLog asserts the k1oe.2 ephemeral audit
// trail: each external-content egress attempt emits one structured slog record
// with id/scope/visibility/owner/content_len/model/outcome (+err on failure).
// Filled and failed paths are both covered. Integration: needs Qdrant.
func TestSummarizeMissingEmitsEgressAuditLog(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "repo:summary-audit"
	long := "this is a long body well over the tiny cap used by the test harness here"

	h := &captureHandler{}
	orig := slog.Default()
	slog.SetDefault(slog.New(h))
	defer slog.SetDefault(orig)

	// Phase 1: successful egress → one audit record, outcome=filled.
	idOK := "e0000000-0000-0000-0000-000000000001"
	mOK := Memory{
		ID: idOK, Content: long, Scope: scope, Category: "convention",
		Source: "agent-inferred", Owner: "owner-E", Visibility: visibilityShared,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, mOK, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed ok: %v", err)
	}
	t.Cleanup(func() { _ = s.Delete(ctx, idOK, Authenticated("owner-E")) })

	fnOK := func(context.Context, string) (string, error) { return "AUTO OK", nil }
	if _, err := s.SummarizeMissing(ctx, SummarizeOptions{Scope: scope, MaxChars: 8, Model: "audit-model"}, fnOK); err != nil {
		t.Fatalf("SummarizeMissing ok: %v", err)
	}
	audits := h.audits()
	if len(audits) != 1 {
		t.Fatalf("phase1: expected 1 audit, got %d %+v", len(audits), audits)
	}
	if audits[0].attrs["outcome"] != "filled" {
		t.Fatalf("phase1 outcome=%q want filled", audits[0].attrs["outcome"])
	}
	if audits[0].attrs["id"] != idOK {
		t.Fatalf("phase1 id=%q want %q", audits[0].attrs["id"], idOK)
	}
	if audits[0].attrs["visibility"] != visibilityShared {
		t.Fatalf("phase1 visibility=%q want %q", audits[0].attrs["visibility"], visibilityShared)
	}
	if audits[0].attrs["model"] != "audit-model" {
		t.Fatalf("phase1 model=%q want audit-model", audits[0].attrs["model"])
	}
	for _, k := range []string{"scope", "owner", "content_len"} {
		if _, ok := audits[0].attrs[k]; !ok {
			t.Fatalf("phase1 audit missing attr %q: %+v", k, audits[0].attrs)
		}
	}

	// Phase 2: failed egress (fresh eligible record; summarize fn errors).
	h.reset()
	idFail := "e0000000-0000-0000-0000-000000000002"
	mFail := Memory{
		ID: idFail, Content: long, Scope: scope, Category: "convention",
		Source: "agent-inferred", Owner: "owner-E", Visibility: visibilityShared,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, mFail, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed fail: %v", err)
	}
	t.Cleanup(func() { _ = s.Delete(ctx, idFail, Authenticated("owner-E")) })

	fnFail := func(context.Context, string) (string, error) { return "", fmt.Errorf("gateway down") }
	if _, err := s.SummarizeMissing(ctx, SummarizeOptions{Scope: scope, MaxChars: 8, Model: "audit-model"}, fnFail); err != nil {
		t.Fatalf("SummarizeMissing fail: %v", err)
	}
	audits = h.audits()
	if len(audits) != 1 {
		t.Fatalf("phase2: expected 1 audit, got %d %+v", len(audits), audits)
	}
	if audits[0].attrs["outcome"] != "failed" {
		t.Fatalf("phase2 outcome=%q want failed", audits[0].attrs["outcome"])
	}
	if !strings.Contains(audits[0].attrs["err"], "gateway down") {
		t.Fatalf("phase2 err attr missing/wrong: %+v", audits[0].attrs)
	}
	// Failed egress must not have written a summary or an egress stamp.
	got, err := s.GetReadable(ctx, idFail, Authenticated("owner-E"))
	if err != nil {
		t.Fatalf("GetReadable fail: %v", err)
	}
	if got.Summary != "" {
		t.Fatalf("failed egress wrote summary: %q", got.Summary)
	}
}

type auditRecord struct {
	level slog.Level
	msg   string
	attrs map[string]string
}

// captureHandler is a slog.Handler that records every record for audit tests.
type captureHandler struct {
	mu   sync.Mutex
	recs []auditRecord
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	m := map[string]string{}
	r.Attrs(func(a slog.Attr) bool { m[a.Key] = a.Value.String(); return true })
	h.mu.Lock()
	h.recs = append(h.recs, auditRecord{level: r.Level, msg: r.Message, attrs: m})
	h.mu.Unlock()
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(string) slog.Handler      { return h }

// audits returns only the egress audit records (filtered by message).
func (h *captureHandler) audits() []auditRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []auditRecord
	for _, r := range h.recs {
		if r.msg == "summarize-missing: egress" {
			out = append(out, r)
		}
	}
	return out
}

func (h *captureHandler) reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.recs = nil
}
