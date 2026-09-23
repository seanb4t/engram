// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package retrievaleval

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/seanb4t/engram/internal/store"
)

// TestVariantMetrics pins variantMetrics' accumulation and derived-metric
// behavior on top of the package's existing recallAtK/reciprocalRank
// helpers: a target at rank 1, then rank 3, then absent.
func TestVariantMetrics(t *testing.T) {
	t.Parallel()

	t.Run("rank 1, rank 3, absent", func(t *testing.T) {
		t.Parallel()
		var m variantMetrics
		m.add([]string{"want", "b", "c"}, "want")
		m.add([]string{"a", "b", "want"}, "want")
		m.add([]string{"a", "b", "c"}, "want")

		if got, want := m.ranks, []int{1, 3, 0}; !reflect.DeepEqual(got, want) {
			t.Fatalf("ranks = %v, want %v", got, want)
		}
		if got, want := m.recall(), 2.0/3; math.Abs(got-want) > 1e-9 {
			t.Errorf("recall() = %v, want %v", got, want)
		}
		wantMRR := (1.0 + 1.0/3) / 3
		if got := m.mrr(); math.Abs(got-wantMRR) > 1e-9 {
			t.Errorf("mrr() = %v, want %v", got, wantMRR)
		}
		if m.allRank1() {
			t.Error("allRank1() = true, want false")
		}
	})

	t.Run("two rank-1 adds -> allRank1 true", func(t *testing.T) {
		t.Parallel()
		var m variantMetrics
		m.add([]string{"want", "b"}, "want")
		m.add([]string{"want"}, "want")
		if !m.allRank1() {
			t.Error("allRank1() = false, want true")
		}
	})

	t.Run("empty value never NaN", func(t *testing.T) {
		t.Parallel()
		var m variantMetrics
		if got := m.recall(); got != 0 {
			t.Errorf("recall() = %v, want 0", got)
		}
		if got := m.mrr(); got != 0 {
			t.Errorf("mrr() = %v, want 0", got)
		}
		if m.allRank1() {
			t.Error("allRank1() = true, want false")
		}
	})
}

// TestFormatVariantTable pins the table's structural shape: a leading
// "| variant |" header, one row per summary, a disabled row rendering its
// disabledReason instead of numbers, and 3-decimal number formatting.
func TestFormatVariantTable(t *testing.T) {
	t.Parallel()
	rows := []variantSummary{
		{name: "vector-only", family: "vector-only", guardRanks: []int{1, 1}, guardAllRank1: true, paraphraseRecall: 0.9, paraphraseMRR: 0.812345},
		{name: "jev", family: "jev", disabled: true, disabledReason: jevDisabledReason},
	}
	got := formatVariantTable(rows, rankingDecision{})

	lines := strings.Split(got, "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "| variant |") {
		t.Fatalf("first line %q does not start with \"| variant |\"", lines[0])
	}
	if !strings.Contains(got, jevDisabledReason) {
		t.Errorf("disabled row does not render %q: %s", jevDisabledReason, got)
	}
	if !strings.Contains(got, "0.812") {
		t.Errorf("numbers not rendered with 3 decimals: %s", got)
	}

	nonEmpty := 0
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonEmpty++
		}
	}
	// header + separator + one line per row.
	if want := len(rows) + 2; nonEmpty != want {
		t.Errorf("got %d non-empty lines, want %d (header + separator + %d rows)", nonEmpty, want, len(rows))
	}
}

// TestEvalRankersRoster (T2 form) pins the full ten-entry roster order (the
// D-07 grid inserted between lexical and the disabled Jev stub), that
// tuned is true exactly for the gate/blend rows, that simplicity strictly
// increases across every enabled row, and that every rank function
// delegates to the exact comparison-ranker/store function it wraps.
func TestEvalRankersRoster(t *testing.T) {
	t.Parallel()
	roster := evalRankers()

	wantNames := []string{
		"vector-only", "lexical",
		"overlap-gate-t0.90", "overlap-gate-t0.75", "overlap-gate-t0.60",
		"cosine-blend-a0.05", "cosine-blend-a0.10", "cosine-blend-a0.20", "cosine-blend-a0.30",
		"jev",
	}
	gotNames := make([]string, len(roster))
	byName := make(map[string]namedRanker, len(roster))
	for i, r := range roster {
		gotNames[i] = r.name
		byName[r.name] = r
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("roster names = %v, want %v", gotNames, wantNames)
	}

	for _, name := range wantNames {
		r := byName[name]
		wantTuned := r.family == "overlap-gate" || r.family == "cosine-blend"
		if r.tuned != wantTuned {
			t.Errorf("%s: tuned = %v, want %v", name, r.tuned, wantTuned)
		}
	}

	prevSimplicity := -1
	for _, r := range roster {
		if r.rank == nil {
			continue // disabled (jev): excluded from the strictly-increasing check.
		}
		if r.simplicity <= prevSimplicity {
			t.Errorf("%s: simplicity %d does not strictly increase over the previous enabled row's %d", r.name, r.simplicity, prevSimplicity)
		}
		prevSimplicity = r.simplicity
	}

	jev := byName["jev"]
	if jev.rank != nil {
		t.Error("jev.rank is not nil, want nil (disabled stub)")
	}
	if jev.disabledReason != jevDisabledReason {
		t.Errorf("jev.disabledReason = %q, want %q", jev.disabledReason, jevDisabledReason)
	}

	query := "alpha bravo"
	sample := []store.Memory{
		{ID: "a", Content: "alpha bravo", Score: 0.3},
		{ID: "b", Content: "nothing matching", Score: 0.9},
		{ID: "c", Content: "alpha only", Score: 0.5},
	}
	if got, want := idsOf(byName["vector-only"].rank(query, sample, 3)), idsOf(store.VectorOrder(sample, 3)); !reflect.DeepEqual(got, want) {
		t.Errorf("vector-only output = %v, want store.VectorOrder %v", got, want)
	}
	if got, want := idsOf(byName["lexical"].rank(query, sample, 3)), idsOf(store.RerankHits(query, sample, 3)); !reflect.DeepEqual(got, want) {
		t.Errorf("lexical output = %v, want store.RerankHits %v", got, want)
	}
	if got, want := idsOf(byName["overlap-gate-t0.90"].rank(query, sample, 3)), idsOf(overlapGateRerank(query, sample, 3, 0.90)); !reflect.DeepEqual(got, want) {
		t.Errorf("overlap-gate-t0.90 output = %v, want overlapGateRerank(theta=0.90) %v", got, want)
	}
	if got, want := idsOf(byName["cosine-blend-a0.05"].rank(query, sample, 3)), idsOf(cosineBlendRerank(query, sample, 3, 0.05)); !reflect.DeepEqual(got, want) {
		t.Errorf("cosine-blend-a0.05 output = %v, want cosineBlendRerank(alpha=0.05) %v", got, want)
	}
}

// TestDecideRanking pins D-05's decision rule, hermetically, before any live
// number exists: every clause and every tie rule.
func TestDecideRanking(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		rows             []variantSummary
		wantWinner       string
		wantReasonSubstr string
	}{
		{
			name: "vector-only has the best MRR",
			rows: []variantSummary{
				{name: "vector-only", family: "vector-only", simplicity: 0, guardAllRank1: true, paraphraseMRR: 0.90},
				{name: "lexical", family: "lexical", simplicity: 10, guardAllRank1: true, paraphraseMRR: 0.50},
			},
			wantWinner: "vector-only",
		},
		{
			name: "tuned variant below the 0.05 margin loses to vector-only",
			rows: []variantSummary{
				{name: "vector-only", family: "vector-only", simplicity: 0, guardAllRank1: true, paraphraseMRR: 0.90},
				{name: "cosine-blend-a0.10", family: "cosine-blend", simplicity: 31, tuned: true, guardAllRank1: true, paraphraseMRR: 0.94},
			},
			wantWinner:       "vector-only",
			wantReasonSubstr: "0.05",
		},
		{
			name: "tuned variant clears the margin and wins",
			rows: []variantSummary{
				{name: "vector-only", family: "vector-only", simplicity: 0, guardAllRank1: true, paraphraseMRR: 0.80},
				{name: "cosine-blend-a0.10", family: "cosine-blend", simplicity: 31, tuned: true, guardAllRank1: true, paraphraseMRR: 0.86},
			},
			wantWinner: "cosine-blend-a0.10",
		},
		{
			name: "margin boundary is epsilon-inclusive",
			rows: []variantSummary{
				{name: "vector-only", family: "vector-only", simplicity: 0, guardAllRank1: true, paraphraseMRR: 0.80},
				{name: "cosine-blend-a0.10", family: "cosine-blend", simplicity: 31, tuned: true, guardAllRank1: true, paraphraseMRR: 0.85},
			},
			wantWinner: "cosine-blend-a0.10",
		},
		{
			name: "guard failure makes a high-MRR variant ineligible",
			rows: []variantSummary{
				{name: "vector-only", family: "vector-only", simplicity: 0, guardAllRank1: true, paraphraseMRR: 0.70},
				{name: "lexical", family: "lexical", simplicity: 10, guardAllRank1: false, paraphraseMRR: 0.99},
				{name: "cosine-blend-a0.10", family: "cosine-blend", simplicity: 31, tuned: true, guardAllRank1: true, paraphraseMRR: 0.80},
			},
			wantWinner: "cosine-blend-a0.10",
		},
		{
			name: "MRR below vector-only's makes a gate variant ineligible",
			rows: []variantSummary{
				{name: "vector-only", family: "vector-only", simplicity: 0, guardAllRank1: true, paraphraseMRR: 0.80},
				{name: "overlap-gate-t0.90", family: "overlap-gate", simplicity: 20, tuned: true, guardAllRank1: true, paraphraseMRR: 0.70},
			},
			wantWinner: "vector-only",
		},
		{
			name: "untuned lexical needs no margin",
			rows: []variantSummary{
				{name: "vector-only", family: "vector-only", simplicity: 0, guardAllRank1: true, paraphraseMRR: 0.90},
				{name: "lexical", family: "lexical", simplicity: 10, guardAllRank1: true, paraphraseMRR: 0.91},
			},
			wantWinner: "lexical",
		},
		{
			name: "exact tie with vector-only: vector-only wins as the simplest",
			rows: []variantSummary{
				{name: "vector-only", family: "vector-only", simplicity: 0, guardAllRank1: true, paraphraseMRR: 0.90},
				{name: "overlap-gate-t0.90", family: "overlap-gate", simplicity: 20, tuned: true, guardAllRank1: true, paraphraseMRR: 0.90},
			},
			wantWinner: "vector-only",
		},
		{
			name: "tie inside the gate family goes to the lower simplicity",
			rows: []variantSummary{
				{name: "vector-only", family: "vector-only", simplicity: 0, guardAllRank1: true, paraphraseMRR: 0.70},
				{name: "overlap-gate-t0.90", family: "overlap-gate", simplicity: 20, tuned: true, guardAllRank1: true, paraphraseMRR: 0.80},
				{name: "overlap-gate-t0.75", family: "overlap-gate", simplicity: 21, tuned: true, guardAllRank1: true, paraphraseMRR: 0.80},
			},
			wantWinner: "overlap-gate-t0.90",
		},
		{
			name: "vector-only ineligible: no margin applies",
			rows: []variantSummary{
				{name: "vector-only", family: "vector-only", simplicity: 0, guardAllRank1: false, paraphraseMRR: 0.80},
				{name: "cosine-blend-a0.10", family: "cosine-blend", simplicity: 31, tuned: true, guardAllRank1: true, paraphraseMRR: 0.82},
				{name: "overlap-gate-t0.90", family: "overlap-gate", simplicity: 20, tuned: true, guardAllRank1: true, paraphraseMRR: 0.81},
			},
			wantWinner: "cosine-blend-a0.10",
		},
		{
			name: "nothing eligible",
			rows: []variantSummary{
				{name: "vector-only", family: "vector-only", simplicity: 0, guardAllRank1: false, paraphraseMRR: 0.80},
				{name: "lexical", family: "lexical", simplicity: 10, guardAllRank1: false, paraphraseMRR: 0.90},
			},
			wantWinner:       "",
			wantReasonSubstr: "no eligible variant",
		},
		{
			name: "disabled jev row ignored despite a fake high MRR",
			rows: []variantSummary{
				{name: "vector-only", family: "vector-only", simplicity: 0, guardAllRank1: true, paraphraseMRR: 0.70},
				{name: "jev", family: "jev", simplicity: 99, disabled: true, paraphraseMRR: 0.99},
			},
			wantWinner: "vector-only",
		},
		{
			name: "shipped row ignored",
			rows: []variantSummary{
				{name: "vector-only", family: "vector-only", simplicity: 0, guardAllRank1: true, paraphraseMRR: 0.70},
				{name: shippedRowName, family: "shipped", paraphraseMRR: 0.99},
			},
			wantWinner: "vector-only",
		},
		{
			name: "missing vector-only baseline",
			rows: []variantSummary{
				{name: "lexical", family: "lexical", simplicity: 10, guardAllRank1: true, paraphraseMRR: 0.90},
			},
			wantWinner:       "",
			wantReasonSubstr: "vector-only",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := decideRanking(tc.rows)
			if got.winner != tc.wantWinner {
				t.Errorf("winner = %q, want %q (reason=%q)", got.winner, tc.wantWinner, got.reason)
			}
			if tc.wantReasonSubstr != "" && !strings.Contains(got.reason, tc.wantReasonSubstr) {
				t.Errorf("reason = %q, want substring %q", got.reason, tc.wantReasonSubstr)
			}
		})
	}
}
