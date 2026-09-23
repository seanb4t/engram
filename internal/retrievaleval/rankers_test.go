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

// TestEvalRankersRoster (T1 form) pins the initial three-entry roster order,
// the disabled Jev stub's shape, and that vector-only/lexical delegate to
// store.VectorOrder/lexicalRerank exactly.
func TestEvalRankersRoster(t *testing.T) {
	t.Parallel()
	roster := evalRankers()

	wantNames := []string{"vector-only", "lexical", "jev"}
	gotNames := make([]string, len(roster))
	for i, r := range roster {
		gotNames[i] = r.name
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("roster names = %v, want %v", gotNames, wantNames)
	}

	var jev namedRanker
	for _, r := range roster {
		if r.name == "jev" {
			jev = r
		}
	}
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
	for _, r := range roster {
		switch r.name {
		case "vector-only":
			got := idsOf(r.rank(query, sample, 3))
			want := idsOf(store.VectorOrder(sample, 3))
			if !reflect.DeepEqual(got, want) {
				t.Errorf("vector-only output = %v, want store.VectorOrder %v", got, want)
			}
		case "lexical":
			got := idsOf(r.rank(query, sample, 3))
			want := idsOf(lexicalRerank(query, sample, 3))
			if !reflect.DeepEqual(got, want) {
				t.Errorf("lexical output = %v, want lexicalRerank %v", got, want)
			}
		}
	}
}
