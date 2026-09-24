// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package curationeval

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/seanb4t/engram/internal/verdict"
)

// fullProbs builds a five-class Probabilities in verdict.Relations() order.
func fullProbs(dup, con, upd, rel, unr float64) verdict.Probabilities {
	return verdict.Probabilities{Duplicate: dup, Contradicts: con, Updates: upd, Related: rel, Unrelated: unr}
}

func TestBucketOf(t *testing.T) {
	t.Parallel()

	cases := []struct {
		p    float64
		want string
	}{
		{0, bucketLow},
		{0.6999, bucketLow},
		{0.70, bucketMid},
		{0.8999, bucketMid},
		{0.90, bucketHigh},
		{1.0, bucketHigh},
	}
	for _, tc := range cases {
		if got := bucketOf(tc.p); got != tc.want {
			t.Errorf("bucketOf(%v) = %q, want %q", tc.p, got, tc.want)
		}
	}
}

func TestAccuracyByBucket(t *testing.T) {
	t.Parallel()

	t.Run("empty_input_gives_three_ordered_na_rows", func(t *testing.T) {
		t.Parallel()
		got := accuracyByBucket(nil)
		want := []bucketStat{
			{name: bucketLow, n: 0, correct: 0, accuracy: "n/a"},
			{name: bucketMid, n: 0, correct: 0, accuracy: "n/a"},
			{name: bucketHigh, n: 0, correct: 0, accuracy: "n/a"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("accuracyByBucket(nil) = %+v, want %+v", got, want)
		}
	})

	t.Run("mixed_input_counts_correct_per_bucket_excluding_failed", func(t *testing.T) {
		t.Parallel()
		preds := []prediction{
			{gold: "duplicate", v: verdict.Verdict{Relation: "duplicate", Probabilities: fullProbs(0.5, 0.2, 0.1, 0.1, 0.1)}},
			{gold: "duplicate", v: verdict.Verdict{Relation: "related", Probabilities: fullProbs(0.1, 0.1, 0.1, 0.75, 0.05)}},
			{gold: "unrelated", v: verdict.Verdict{Relation: "unrelated", Probabilities: fullProbs(0.01, 0.01, 0.01, 0.02, 0.95)}},
			{gold: "contradicts", v: verdict.Verdict{Relation: "updates", Probabilities: fullProbs(0.01, 0.02, 0.95, 0.01, 0.01)}},
			{gold: "related", v: verdict.Verdict{ErrorClass: "timeout"}},
		}
		got := accuracyByBucket(preds)
		want := []bucketStat{
			{name: bucketLow, n: 1, correct: 1, accuracy: "1.000"},
			{name: bucketMid, n: 1, correct: 0, accuracy: "0.000"},
			{name: bucketHigh, n: 2, correct: 1, accuracy: "0.500"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("accuracyByBucket(mixed) = %+v, want %+v", got, want)
		}
	})
}

func TestBrier(t *testing.T) {
	t.Parallel()

	t.Run("perfect_one_hot_scores_0", func(t *testing.T) {
		t.Parallel()
		preds := []prediction{
			{gold: "duplicate", v: verdict.Verdict{Relation: "duplicate", Probabilities: fullProbs(1, 0, 0, 0, 0)}},
		}
		score, n := brier(preds)
		if n != 1 {
			t.Fatalf("n = %d, want 1", n)
		}
		if diff := math.Abs(score - 0); diff > 1e-12 {
			t.Errorf("score = %v, want ~0 (diff %v)", score, diff)
		}
	})

	t.Run("all_five_at_0.2_scores_0.8", func(t *testing.T) {
		t.Parallel()
		preds := []prediction{
			{gold: "related", v: verdict.Verdict{Relation: "related", Probabilities: fullProbs(0.2, 0.2, 0.2, 0.2, 0.2)}},
		}
		score, n := brier(preds)
		if n != 1 {
			t.Fatalf("n = %d, want 1", n)
		}
		if diff := math.Abs(score - 0.8); diff > 1e-12 {
			t.Errorf("score = %v, want ~0.8 (diff %v)", score, diff)
		}
	})

	t.Run("all_mass_on_wrong_class_scores_2", func(t *testing.T) {
		t.Parallel()
		preds := []prediction{
			{gold: "duplicate", v: verdict.Verdict{Relation: "unrelated", Probabilities: fullProbs(0, 0, 0, 0, 1)}},
		}
		score, n := brier(preds)
		if n != 1 {
			t.Fatalf("n = %d, want 1", n)
		}
		if diff := math.Abs(score - 2); diff > 1e-12 {
			t.Errorf("score = %v, want ~2 (diff %v)", score, diff)
		}
	})

	t.Run("failed_verdicts_excluded", func(t *testing.T) {
		t.Parallel()
		preds := []prediction{
			{gold: "duplicate", v: verdict.Verdict{Relation: "duplicate", Probabilities: fullProbs(1, 0, 0, 0, 0)}},
			{gold: "related", v: verdict.Verdict{ErrorClass: "timeout"}},
		}
		score, n := brier(preds)
		if n != 1 {
			t.Fatalf("n = %d, want 1 (failed excluded)", n)
		}
		if diff := math.Abs(score - 0); diff > 1e-12 {
			t.Errorf("score = %v, want ~0", score)
		}
	})

	t.Run("zero_scored_predictions_report_na", func(t *testing.T) {
		t.Parallel()
		preds := []prediction{
			{gold: "related", v: verdict.Verdict{ErrorClass: "timeout"}},
		}
		score, n := brier(preds)
		if n != 0 {
			t.Fatalf("n = %d, want 0", n)
		}
		if got := formatMetric(score, n); got != "n/a" {
			t.Errorf("formatMetric(brier(...)) = %q, want %q", got, "n/a")
		}
	})
}

func TestThresholdGate(t *testing.T) {
	t.Parallel()

	mk := func(gold, relation string, prob float64) prediction {
		var probs verdict.Probabilities
		switch relation {
		case verdict.Duplicate:
			probs.Duplicate = prob
		case verdict.Contradicts:
			probs.Contradicts = prob
		case verdict.Updates:
			probs.Updates = prob
		case verdict.Related:
			probs.Related = prob
		case verdict.Unrelated:
			probs.Unrelated = prob
		}
		return prediction{gold: gold, v: verdict.Verdict{Relation: relation, Probabilities: probs}}
	}

	t.Run("zero_at_or_above_threshold_is_vacuous", func(t *testing.T) {
		t.Parallel()
		preds := []prediction{mk("duplicate", "duplicate", 0.5)}
		n, _, result := thresholdGate(preds, 0.9)
		if n != 0 || result != "VACUOUS" {
			t.Errorf("thresholdGate = (n=%d, result=%q), want (0, VACUOUS)", n, result)
		}
	})

	t.Run("9_of_10_passes", func(t *testing.T) {
		t.Parallel()
		preds := make([]prediction, 0, 10)
		for range 9 {
			preds = append(preds, mk("duplicate", "duplicate", 0.95))
		}
		preds = append(preds, mk("duplicate", "related", 0.95))
		n, correct, result := thresholdGate(preds, 0.9)
		if n != 10 || correct != 9 || result != "PASS" {
			t.Errorf("thresholdGate = (n=%d, correct=%d, result=%q), want (10, 9, PASS)", n, correct, result)
		}
	})

	t.Run("8_of_10_fails", func(t *testing.T) {
		t.Parallel()
		preds := make([]prediction, 0, 10)
		for range 8 {
			preds = append(preds, mk("duplicate", "duplicate", 0.95))
		}
		for range 2 {
			preds = append(preds, mk("duplicate", "related", 0.95))
		}
		n, correct, result := thresholdGate(preds, 0.9)
		if n != 10 || correct != 8 || result != "FAIL" {
			t.Errorf("thresholdGate = (n=%d, correct=%d, result=%q), want (10, 8, FAIL)", n, correct, result)
		}
	})

	t.Run("27_of_30_passes", func(t *testing.T) {
		t.Parallel()
		preds := make([]prediction, 0, 30)
		for range 27 {
			preds = append(preds, mk("duplicate", "duplicate", 0.95))
		}
		for range 3 {
			preds = append(preds, mk("duplicate", "related", 0.95))
		}
		n, correct, result := thresholdGate(preds, 0.9)
		if n != 30 || correct != 27 || result != "PASS" {
			t.Errorf("thresholdGate = (n=%d, correct=%d, result=%q), want (30, 27, PASS)", n, correct, result)
		}
	})

	t.Run("probability_exactly_equal_to_threshold_is_counted", func(t *testing.T) {
		t.Parallel()
		preds := []prediction{mk("duplicate", "duplicate", 0.9)}
		n, correct, _ := thresholdGate(preds, 0.9)
		if n != 1 || correct != 1 {
			t.Errorf("thresholdGate = (n=%d, correct=%d), want (1, 1) — prob == threshold must count", n, correct)
		}
	})

	t.Run("failed_verdicts_excluded", func(t *testing.T) {
		t.Parallel()
		preds := []prediction{
			mk("duplicate", "duplicate", 0.95),
			{gold: "related", v: verdict.Verdict{ErrorClass: "timeout"}},
		}
		n, correct, result := thresholdGate(preds, 0.9)
		if n != 1 || correct != 1 || result != "PASS" {
			t.Errorf("thresholdGate = (n=%d, correct=%d, result=%q), want (1, 1, PASS)", n, correct, result)
		}
	})
}

// TestFormatReportAggregatesOnly proves the report is aggregate-only (no
// pair text), deterministic, and carries every required line per D-03.
func TestFormatReportAggregatesOnly(t *testing.T) {
	t.Parallel()

	preds := []prediction{
		{gold: "duplicate", v: verdict.Verdict{
			Relation: "duplicate", Probabilities: fullProbs(0.95, 0.01, 0.01, 0.02, 0.01), Model: "typesafe/jev-1.13-a",
		}},
		{gold: "contradicts", v: verdict.Verdict{
			Relation: "updates", Probabilities: fullProbs(0.01, 0.3, 0.6, 0.05, 0.04), Model: "typesafe/jev-1.13-b", NeedsReview: true,
		}},
		{gold: "related", v: verdict.Verdict{ErrorClass: "timeout"}},
	}

	lines1 := formatReport("committed", preds, 0.9)
	lines2 := formatReport("committed", preds, 0.9)
	if !reflect.DeepEqual(lines1, lines2) {
		t.Fatalf("formatReport is not deterministic:\n%v\nvs\n%v", lines1, lines2)
	}

	for i, line := range lines1 {
		if !strings.HasPrefix(line, "CURATION-EVAL | ") {
			t.Errorf("line %d = %q, missing the CURATION-EVAL prefix", i, line)
		}
	}

	joined := strings.Join(lines1, "\n")
	for _, forbidden := range []string{syntheticPairs[0].recordA, syntheticPairs[0].recordB} {
		if strings.Contains(joined, forbidden) {
			t.Errorf("formatReport output leaked pair text: %q", forbidden)
		}
	}

	mustContain := []string{
		"corpus=committed",
		"pairs=3",
		"scored=2",
		"unavailable=1",
		"timeout=1",
		"threshold=0.900",
		"bucket=low",
		"bucket=mid",
		"bucket=high",
		"brier=",
		"uniform=0.800",
		"gate threshold=0.900",
		"result=",
		"confusion gold=duplicate",
		"confusion gold=contradicts",
		"confusion gold=updates",
		"confusion gold=related",
		"confusion gold=unrelated",
		"needs_review=1",
	}
	for _, want := range mustContain {
		if !strings.Contains(joined, want) {
			t.Errorf("formatReport output missing %q:\n%s", want, joined)
		}
	}

	var confusionLines []string
	for _, line := range lines1 {
		if strings.Contains(line, "confusion gold=") {
			confusionLines = append(confusionLines, line)
		}
	}
	if len(confusionLines) != len(verdict.Relations()) {
		t.Fatalf("confusion lines = %d, want %d", len(confusionLines), len(verdict.Relations()))
	}
	for i, rel := range verdict.Relations() {
		want := "confusion gold=" + rel
		if !strings.Contains(confusionLines[i], want) {
			t.Errorf("confusion line %d = %q, want to contain %q (verdict.Relations() order)", i, confusionLines[i], want)
		}
	}
}
