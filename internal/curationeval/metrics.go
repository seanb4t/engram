// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package curationeval

import (
	"fmt"
	"sort"
	"strings"

	"github.com/seanb4t/engram/internal/verdict"
)

// The three D-03 confidence buckets, lower-inclusive: low is [0, 0.70), mid
// is [0.70, 0.90), high is [0.90, 1.00].
const (
	bucketLow  = "low"
	bucketMid  = "mid"
	bucketHigh = "high"
)

// bucketOf classifies a chosen relation's probability into one of the three
// D-03 confidence buckets.
func bucketOf(p float64) string {
	switch {
	case p < 0.70:
		return bucketLow
	case p < 0.90:
		return bucketMid
	default:
		return bucketHigh
	}
}

// bucketStat is one confidence bucket's scored count, correct count and
// formatted accuracy ("n/a" when n is 0).
type bucketStat struct {
	name       string
	n, correct int
	accuracy   string
}

// accuracyByBucket returns one bucketStat per D-03 bucket, always in low,
// mid, high order regardless of input — an empty bucket still reports its
// row with n 0 and accuracy "n/a". Failed verdicts are excluded.
func accuracyByBucket(preds []prediction) []bucketStat {
	stats := map[string]*bucketStat{
		bucketLow:  {name: bucketLow},
		bucketMid:  {name: bucketMid},
		bucketHigh: {name: bucketHigh},
	}
	for _, p := range preds {
		if p.v.Failed() {
			continue
		}
		prob, ok := p.v.Probabilities.Of(p.v.Relation)
		if !ok {
			continue
		}
		s := stats[bucketOf(prob)]
		s.n++
		if p.v.Relation == p.gold {
			s.correct++
		}
	}
	rows := []*bucketStat{stats[bucketLow], stats[bucketMid], stats[bucketHigh]}
	out := make([]bucketStat, len(rows))
	for i, r := range rows {
		r.accuracy = formatMetric(safeRatio(r.correct, r.n), r.n)
		out[i] = *r
	}
	return out
}

// safeRatio returns correct/n, or 0 when n is 0 (formatMetric never reads
// the value in that case, but this avoids a NaN leaking into a struct
// field for callers that inspect it directly).
func safeRatio(correct, n int) float64 {
	if n == 0 {
		return 0
	}
	return float64(correct) / float64(n)
}

// formatMetric formats v as "%.3f", or "n/a" when n is 0 — the single
// rounding point shared by accuracyByBucket's per-bucket accuracy and
// brier's report line; every probability upstream is used verbatim and
// rounded only here, for display.
func formatMetric(v float64, n int) string {
	if n == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.3f", v)
}

// brier returns the multi-class Brier score (D-03): for each scored
// prediction, the sum over verdict.Relations() of the squared error
// between that relation's verbatim probability and the one-hot gold
// indicator, averaged over every scored prediction (n). Failed verdicts
// are excluded. n is 0 (score undefined, callers format via formatMetric)
// when no prediction was scored.
func brier(preds []prediction) (score float64, n int) {
	var sum float64
	for _, p := range preds {
		if p.v.Failed() {
			continue
		}
		n++
		for _, rel := range verdict.Relations() {
			prob, _ := p.v.Probabilities.Of(rel)
			target := 0.0
			if rel == p.gold {
				target = 1.0
			}
			d := prob - target
			sum += d * d
		}
	}
	if n == 0 {
		return 0, 0
	}
	return sum / float64(n), n
}

// thresholdGate is D-03's single hard gate: among scored predictions whose
// chosen relation's probability is at or above threshold (never a strict
// equality exclusion), n is the count and correct the number whose chosen
// relation matched gold. result is VACUOUS when n is 0 (a vacuous truth is
// never a pass), PASS when correct*10 is at least 9*n (integer arithmetic
// only — no float comparison), else FAIL. Failed verdicts are excluded
// before the threshold filter.
func thresholdGate(preds []prediction, threshold float64) (n, correct int, result string) {
	for _, p := range preds {
		if p.v.Failed() {
			continue
		}
		prob, ok := p.v.Probabilities.Of(p.v.Relation)
		if !ok || prob < threshold {
			continue
		}
		n++
		if p.v.Relation == p.gold {
			correct++
		}
	}
	switch {
	case n == 0:
		result = "VACUOUS"
	case correct*10 >= 9*n:
		result = "PASS"
	default:
		result = "FAIL"
	}
	return n, correct, result
}

// confusion returns a five-by-five count matrix keyed gold relation then
// predicted relation, over every verdict.Relations() name — every gold key
// is present even at zero, so callers can render a deterministic full
// matrix. Failed verdicts are excluded.
func confusion(preds []prediction) map[string]map[string]int {
	m := make(map[string]map[string]int, len(verdict.Relations()))
	for _, gold := range verdict.Relations() {
		m[gold] = make(map[string]int, len(verdict.Relations()))
	}
	for _, p := range preds {
		if p.v.Failed() {
			continue
		}
		if _, ok := m[p.gold]; !ok {
			continue
		}
		m[p.gold][p.v.Relation]++
	}
	return m
}

// formatReport renders preds (and only preds — no pair text) into the D-03
// aggregate-only report: corpus/pair/scored/unavailable counts and their
// per-class breakdown, the sorted distinct model snapshots, the resolved
// threshold, the three bucket rows, the Brier score against the uniform
// 0.800 baseline, the single threshold gate row, one confusion row per
// gold class in verdict.Relations() order, and the needs-review count.
// Every line is prefixed "CURATION-EVAL | " so plan 03-08's records carry
// aggregates only. Pure and deterministic: two calls with the same
// arguments produce identical output.
func formatReport(corpus string, preds []prediction, threshold float64) []string {
	const pfx = "CURATION-EVAL | "

	scored, unavailable := 0, 0
	unavailableByClass := map[string]int{}
	modelSet := map[string]bool{}
	needsReview := 0
	for _, p := range preds {
		if p.v.Failed() {
			unavailable++
			unavailableByClass[p.v.ErrorClass]++
			continue
		}
		scored++
		if p.v.Model != "" {
			modelSet[p.v.Model] = true
		}
		if p.v.NeedsReview {
			needsReview++
		}
	}

	models := make([]string, 0, len(modelSet))
	for m := range modelSet {
		models = append(models, m)
	}
	sort.Strings(models)

	classKeys := make([]string, 0, len(unavailableByClass))
	for k := range unavailableByClass {
		classKeys = append(classKeys, k)
	}
	sort.Strings(classKeys)
	unavailParts := make([]string, 0, len(classKeys))
	for _, k := range classKeys {
		unavailParts = append(unavailParts, fmt.Sprintf("%s=%d", k, unavailableByClass[k]))
	}

	lines := make([]string, 0, 8+len(verdict.Relations()))
	lines = append(lines, fmt.Sprintf("%scorpus=%s pairs=%d scored=%d unavailable=%d", pfx, corpus, len(preds), scored, unavailable))
	lines = append(lines, fmt.Sprintf("%sunavailable_by_class %s", pfx, strings.Join(unavailParts, " ")))
	lines = append(lines, fmt.Sprintf("%smodels=%s threshold=%.3f", pfx, strings.Join(models, ","), threshold))

	for _, b := range accuracyByBucket(preds) {
		lines = append(lines, fmt.Sprintf("%sbucket=%s n=%d correct=%d accuracy=%s", pfx, b.name, b.n, b.correct, b.accuracy))
	}

	bscore, bn := brier(preds)
	lines = append(lines, fmt.Sprintf("%sbrier=%s uniform=0.800 n=%d", pfx, formatMetric(bscore, bn), bn))

	gn, gcorrect, gresult := thresholdGate(preds, threshold)
	lines = append(lines, fmt.Sprintf("%sgate threshold=%.3f n=%d correct=%d result=%s", pfx, threshold, gn, gcorrect, gresult))

	cm := confusion(preds)
	for _, gold := range verdict.Relations() {
		parts := make([]string, 0, len(verdict.Relations()))
		for _, pred := range verdict.Relations() {
			parts = append(parts, fmt.Sprintf("%s=%d", pred, cm[gold][pred]))
		}
		lines = append(lines, fmt.Sprintf("%sconfusion gold=%s %s", pfx, gold, strings.Join(parts, " ")))
	}

	lines = append(lines, fmt.Sprintf("%sneeds_review=%d", pfx, needsReview))

	return lines
}
