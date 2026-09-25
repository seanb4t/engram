// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// reportPool builds n hits with ids id-0..id-{n-1}, descending Score, and a
// sentinel in every content-bearing field so the audit test can prove none
// of it leaks.
func reportPool(n int, sentinel string) []Memory {
	hits := make([]Memory, n)
	for i := range hits {
		hits[i] = Memory{
			ID:      "id-" + string(rune('0'+i)),
			Score:   float32(n-i) / float32(n),
			Content: sentinel, Summary: sentinel, Tags: []string{sentinel}, Scope: sentinel,
		}
	}
	return hits
}

// mapHook returns a RankHook scoring each id from rel verbatim.
func mapHook(rel map[string]float64) RankHook {
	return func(context.Context, string, []Memory) (map[string]float64, error) { return rel, nil }
}

// stampedSpan runs fn inside a fresh recorded span and returns that span.
func stampedSpan(t *testing.T, fn func(ctx context.Context)) sdktrace.ReadOnlySpan {
	t.Helper()
	sr := withSpanRecorder(t)
	before := len(sr.Ended())
	ctx, span := tracer.Start(context.Background(), "test.ambient")
	fn(ctx)
	span.End()
	got := spanByName(sr.Ended()[before:], "test.ambient")
	if got == nil {
		t.Fatal("ambient span was not recorded")
	}
	return got
}

func spanFrom(ctx context.Context) trace.Span { return trace.SpanFromContext(ctx) }

func rerankAttrs(sp sdktrace.ReadOnlySpan) map[string]attribute.Value {
	out := map[string]attribute.Value{}
	for _, kv := range sp.Attributes() {
		if strings.HasPrefix(string(kv.Key), "engram.rerank.") {
			out[string(kv.Key)] = kv.Value
		}
	}
	return out
}

// TestRankReportStampApplied pins the displacement arithmetic within the
// caller's window: pool of 6 in lexical order id-0..id-5, k=3, a hook that
// promotes id-4 to the top and swaps nothing else the caller can see.
func TestRankReportStampApplied(t *testing.T) {
	hits := reportPool(6, "s")
	rel := map[string]float64{"id-4": 0.9, "id-0": 0.5, "id-1": 0.4, "id-2": 0.3, "id-3": 0.2, "id-5": 0.1}

	var out []Memory
	sp := stampedSpan(t, func(ctx context.Context) {
		var rep rankReport
		out, rep = rankWithReport(ctx, "q", hits, 3, mapHook(rel))
		rep.stamp(spanFrom(ctx))
	})
	if got := []string{out[0].ID, out[1].ID, out[2].ID}; got[0] != "id-4" || got[1] != "id-0" || got[2] != "id-1" {
		t.Fatalf("ranked top-3 = %v, want [id-4 id-0 id-1]", got)
	}

	a := rerankAttrs(sp)
	want := map[string]attribute.Value{
		AttrRerankOutcome:      attribute.StringValue(RerankOutcomeApplied),
		AttrRerankTop1Changed:  attribute.BoolValue(true),
		AttrRerankMoved:        attribute.IntValue(3), // id-4/id-0/id-1 vs id-0/id-1/id-2: every window slot differs
		AttrRerankPromoted:     attribute.IntValue(1), // only id-4 came from beyond k
		AttrRerankRelevanceMax: attribute.Float64Value(0.9),
	}
	for k, v := range want {
		if got, ok := a[k]; !ok || got != v {
			t.Errorf("%s = %v (present=%v), want %v", k, got.String(), ok, v.String())
		}
	}
	if _, ok := a[AttrRerankFallbackClass]; ok {
		t.Errorf("%s stamped on an applied rerank", AttrRerankFallbackClass)
	}
}

// TestRankReportStampIdenticalOrder: scores that keep the lexical order
// stamp applied with zero displacement — the "Jev agreed" case must be
// distinguishable from fallback.
func TestRankReportStampIdenticalOrder(t *testing.T) {
	hits := reportPool(4, "s")
	rel := map[string]float64{"id-0": 0.8, "id-1": 0.6, "id-2": 0.4, "id-3": 0.2}
	sp := stampedSpan(t, func(ctx context.Context) {
		_, rep := rankWithReport(ctx, "q", hits, 2, mapHook(rel))
		rep.stamp(spanFrom(ctx))
	})
	a := rerankAttrs(sp)
	if a[AttrRerankOutcome] != attribute.StringValue(RerankOutcomeApplied) {
		t.Fatalf("outcome = %v, want applied", a[AttrRerankOutcome].String())
	}
	if a[AttrRerankTop1Changed] != attribute.BoolValue(false) || a[AttrRerankMoved] != attribute.IntValue(0) || a[AttrRerankPromoted] != attribute.IntValue(0) {
		t.Errorf("displacement = top1 %v moved %v promoted %v, want false/0/0", a[AttrRerankTop1Changed].String(), a[AttrRerankMoved].String(), a[AttrRerankPromoted].String())
	}
	if a[AttrRerankRelevanceMax] != attribute.Float64Value(0.8) {
		t.Errorf("relevance_max = %v, want 0.8", a[AttrRerankRelevanceMax].String())
	}
}

// TestRankReportStampFallbacks pins the three store-visible fallback shapes
// and the skipped/off shapes: a hook error stamps fallback with NO class
// (the hook stamps its own), a nil map stamps no_scores, a partial map
// stamps rejected_scores, an empty pool stamps skipped, and a nil hook
// stamps nothing at all.
func TestRankReportStampFallbacks(t *testing.T) {
	hits := reportPool(3, "s")
	errHook := func(context.Context, string, []Memory) (map[string]float64, error) { return nil, errors.New("boom") }
	nilHook := func(context.Context, string, []Memory) (map[string]float64, error) { return nil, nil }

	cases := []struct {
		name      string
		hits      []Memory
		hook      RankHook
		wantOut   string
		wantClass string
	}{
		{"hook error", hits, errHook, RerankOutcomeFallback, ""},
		{"nil map", hits, nilHook, RerankOutcomeFallback, "no_scores"},
		{"partial map", hits, mapHook(map[string]float64{"id-0": 0.5}), RerankOutcomeFallback, "rejected_scores"},
		{"empty pool", nil, errHook, RerankOutcomeSkipped, ""},
		{"ranker off", hits, nil, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sp := stampedSpan(t, func(ctx context.Context) {
				_, rep := rankWithReport(ctx, "q", tc.hits, 2, tc.hook)
				rep.stamp(spanFrom(ctx))
			})
			a := rerankAttrs(sp)
			if tc.wantOut == "" {
				if len(a) != 0 {
					t.Fatalf("ranker off stamped %d engram.rerank.* attributes, want none: %v", len(a), a)
				}
				return
			}
			if a[AttrRerankOutcome] != attribute.StringValue(tc.wantOut) {
				t.Errorf("outcome = %v, want %s", a[AttrRerankOutcome].String(), tc.wantOut)
			}
			got, ok := a[AttrRerankFallbackClass]
			if tc.wantClass == "" && ok {
				t.Errorf("fallback_class = %v, want absent", got.String())
			}
			if tc.wantClass != "" && got != attribute.StringValue(tc.wantClass) {
				t.Errorf("fallback_class = %v, want %s", got.String(), tc.wantClass)
			}
			for _, k := range []string{AttrRerankTop1Changed, AttrRerankMoved, AttrRerankPromoted, AttrRerankRelevanceMax} {
				if _, ok := a[k]; ok {
					t.Errorf("%s stamped on a non-applied rerank", k)
				}
			}
		})
	}
}

// TestRankReportAuditIsIdsOnly: the audit line carries the query, owner,
// k, pool, outcome and a candidates JSON array of id/before_rank/after_rank/
// cosine/relevance over the WHOLE pool — and not one byte of record
// content, summary, tags or scope, even though every hit carries a
// sentinel in all four.
func TestRankReportAuditIsIdsOnly(t *testing.T) {
	const sentinel = "SENTINEL-AUDIT-9c1e"
	hits := reportPool(4, sentinel)
	rel := map[string]float64{"id-3": 0.9, "id-0": 0.5, "id-1": 0.4, "id-2": 0.3}

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	_, rep := rankWithReport(context.Background(), "the query text", hits, 2, mapHook(rel))
	rep.audit(context.Background(), "search_memory", "the query text", "owner@example")

	out := strings.TrimSpace(buf.String())
	if strings.Contains(out, sentinel) {
		t.Fatalf("audit line leaked record content: %s", out)
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(out), &rec); err != nil {
		t.Fatalf("audit line is not one JSON record: %v: %q", err, out)
	}
	for k, want := range map[string]any{"msg": "search rerank audit", "surface": "search_memory", "query": "the query text", "owner": "owner@example", "k": 2.0, "pool": 4.0, "outcome": "applied"} {
		if rec[k] != want {
			t.Errorf("%s = %v, want %v", k, rec[k], want)
		}
	}
	var cands []auditCandidate
	if err := json.Unmarshal([]byte(rec["candidates"].(string)), &cands); err != nil {
		t.Fatalf("candidates is not a JSON array: %v", err)
	}
	if len(cands) != 4 {
		t.Fatalf("candidates has %d entries, want the whole pool (4)", len(cands))
	}
	// id-3 was lexical #4, now #1 with relevance 0.9; id-0 was #1, now #2.
	if c := cands[0]; c.ID != "id-3" || c.BeforeRank != 4 || c.AfterRank != 1 || c.Relevance == nil || *c.Relevance != 0.9 || c.Cosine == 0 {
		t.Errorf("candidates[0] = %+v, want id-3 before 4 after 1 relevance 0.9 with cosine", c)
	}
	if c := cands[1]; c.ID != "id-0" || c.BeforeRank != 1 || c.AfterRank != 2 {
		t.Errorf("candidates[1] = %+v, want id-0 before 1 after 2", c)
	}
}

// TestRankReportAuditSilentWhenRankerOff: an empty outcome (nil hook) emits
// nothing — the audit is a property of a rerank, never of a plain search.
func TestRankReportAuditSilentWhenRankerOff(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	_, rep := rankWithReport(context.Background(), "q", reportPool(2, "s"), 2, nil)
	rep.audit(context.Background(), "search_memory", "q", "owner")
	if buf.Len() != 0 {
		t.Fatalf("audit emitted with the ranker off: %q", buf.String())
	}
}
