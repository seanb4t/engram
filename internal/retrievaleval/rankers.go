// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package retrievaleval

import (
	"fmt"
	"strings"

	"github.com/seanb4t/engram/internal/store"
)

// shippedRowName labels the table row measured through store.SearchReranked
// itself, distinct from every named-roster row (which ranks in-process over
// the same candidate pool). It is a stable log marker plans 01-05/01-06
// grep for.
const shippedRowName = "shipped (SearchReranked)"

// jevDisabledReason is the disabled Jev stub's table cell text (D-11): a
// stable log marker plans 01-05/01-06 grep for. Phase 4 enables Jev by
// giving this roster entry a rank function; nothing else about the roster
// or the eval loop changes.
const jevDisabledReason = "Jev: disabled"

// d05TunedMargin is D-05's anti-overfitting guard: a tuned (grid) variant —
// cosine-blend or overlap-gate — must beat vector-only's paraphrase MRR by
// at least this much to win over it when vector-only is itself eligible.
// D-05's text applies the margin only to tuned variants: an untuned one
// (lexical) needs no margin, only a strictly better MRR.
const d05TunedMargin = 0.05

// mrrEpsilon is the floating-point tolerance every D-05 MRR comparison
// uses, so two values a naive float64 comparison would read as "different"
// by rounding noise alone are treated as equal.
const mrrEpsilon = 1e-9

// gateThetaGrid is the fixed D-07 overlap-gate threshold grid, fixed BEFORE
// any live number exists. It spans near-verbatim-only promotion (0.90) —
// the #261 shape, where a restatement matches nearly every query term — to
// moderate promotion (0.60), since a genuine paraphrase matches far fewer
// terms; D-05's 0.05 margin is the guard against overfitting this grid to
// this eval's own corpus.
var gateThetaGrid = []float64{0.90, 0.75, 0.60}

// blendAlphaGrid is the fixed D-07 cosine-blend weight grid, fixed BEFORE
// any live number exists. It spans a barely-nudging blend (0.05) to an
// overlap-dominant one (0.30), reflecting spike 004's small raw-cosine
// gaps between sticky topical neighbours — a small alpha is already
// enough to matter; D-05's 0.05 margin is the guard against overfitting
// this grid to this eval's own corpus.
var blendAlphaGrid = []float64{0.05, 0.10, 0.20, 0.30}

// rankerFunc ranks pool for query, returning at most k hits. It carries the
// same pure (query, hits, k) contract as store.RerankHits/store.VectorOrder
// and this package's comparison rankers: no I/O, no server concepts.
type rankerFunc func(query string, pool []store.Memory, k int) []store.Memory

// namedRanker is one entry in the pluggable named-ranker roster (D-11). A
// disabled entry (Jev today) carries a nil rank and a non-empty
// disabledReason; every other entry carries a non-nil rank and an empty
// disabledReason.
type namedRanker struct {
	name           string
	family         string
	simplicity     int
	tuned          bool
	rank           rankerFunc
	disabledReason string
}

// evalRankers returns the pluggable named-ranker roster (D-11) the eval
// measures every retrieval-case query against, over SearchReranked's own
// candidate pool (store.CandidateK(defaultK)): vector-only, lexical, the
// D-07 overlap-gate grid (highest theta first, so it is also the
// simplest), the D-07 cosine-blend grid (lowest alpha first), and a
// disabled Jev stub last. Phase 4 enables Jev by giving its stub entry a
// rank function — a pure append, never a refactor of this function's shape
// or the eval loop that consumes it.
func evalRankers() []namedRanker {
	rankers := []namedRanker{
		{
			name:       "vector-only",
			family:     "vector-only",
			simplicity: 0,
			rank: func(_ string, pool []store.Memory, k int) []store.Memory {
				return store.VectorOrder(pool, k)
			},
		},
		{
			// D-05's live eval retained lexical reranking as the approved
			// winner (01-RANKING-DECISION.md), so this roster entry points
			// directly at the exported store.RerankHits — there is no
			// eval-local lexical copy to drift from it (see
			// comparison_rankers.go's file doc comment).
			name:       "lexical",
			family:     "lexical",
			simplicity: 10,
			rank:       store.RerankHits,
		},
	}
	for i, theta := range gateThetaGrid {
		rankers = append(rankers, namedRanker{
			name:       fmt.Sprintf("overlap-gate-t%.2f", theta),
			family:     "overlap-gate",
			simplicity: 20 + i,
			tuned:      true,
			rank: func(query string, pool []store.Memory, k int) []store.Memory {
				return overlapGateRerank(query, pool, k, theta)
			},
		})
	}
	for i, alpha := range blendAlphaGrid {
		rankers = append(rankers, namedRanker{
			name:       fmt.Sprintf("cosine-blend-a%.2f", alpha),
			family:     "cosine-blend",
			simplicity: 30 + i,
			tuned:      true,
			rank: func(query string, pool []store.Memory, k int) []store.Memory {
				return cosineBlendRerank(query, pool, k, alpha)
			},
		})
	}
	return append(rankers, namedRanker{
		name:           "jev",
		family:         "jev",
		simplicity:     99,
		disabledReason: jevDisabledReason,
	})
}

// variantMetrics accumulates recall@k, MRR and per-query ranks for one
// (ranker, case-role) pair. add wraps the package's existing
// recallAtK/reciprocalRank helpers rather than reimplementing the id-list
// scan; recall/mrr/allRank1 derive their aggregate from the accumulated
// ranks so an empty value never divides by zero into NaN.
type variantMetrics struct {
	ranks []int // 1-indexed rank per query; 0 means the target was absent.
	n     int
}

// add records one query's ranked-id list against wantID.
func (m *variantMetrics) add(ids []string, wantID string) {
	m.n++
	rank := 0
	if rr := reciprocalRank(ids, wantID); rr > 0 {
		// reciprocalRank returns exactly 1/rank for a positive integer rank,
		// so this round-trips exactly for any realistic candidate-pool size.
		rank = int(1.0/rr + 0.5)
	}
	m.ranks = append(m.ranks, rank)
}

// recall reports the fraction of adds whose target was present anywhere in
// its id list. Returns 0 (never NaN) on an empty value.
func (m variantMetrics) recall() float64 {
	if m.n == 0 {
		return 0
	}
	hits := 0
	for _, r := range m.ranks {
		if r > 0 {
			hits++
		}
	}
	return float64(hits) / float64(m.n)
}

// mrr reports the mean reciprocal rank across all adds (absent targets
// contribute 0). Returns 0 (never NaN) on an empty value.
func (m variantMetrics) mrr() float64 {
	if m.n == 0 {
		return 0
	}
	var sum float64
	for _, r := range m.ranks {
		if r > 0 {
			sum += 1.0 / float64(r)
		}
	}
	return sum / float64(m.n)
}

// allRank1 reports whether every add's target was present AND at rank 1.
// An empty value reports false — there are no ranks to hold the bar.
func (m variantMetrics) allRank1() bool {
	if len(m.ranks) == 0 {
		return false
	}
	for _, r := range m.ranks {
		if r != 1 {
			return false
		}
	}
	return true
}

// rankingDecision is D-05's mechanically-applied decision: which named
// ranker (if any) ships, which rows were eligible, and why. The zero value
// (winner "", eligible nil, reason "") is what formatVariantTable renders
// before decideRanking has run.
type rankingDecision struct {
	winner   string
	eligible []string
	reason   string
}

// variantSummary is one row of the eval's variant comparison table: a named
// roster entry's per-role aggregate metrics (or the shipped row, family
// "shipped"), independent of any particular rankingDecision.
type variantSummary struct {
	name             string
	family           string
	simplicity       int
	tuned            bool
	disabled         bool
	disabledReason   string
	guardRanks       []int
	guardAllRank1    bool
	paraphraseRecall float64
	paraphraseMRR    float64
	paraphraseN      int
}

// buildSummaries builds one variantSummary per roster entry, in roster
// order, followed by the shipped row (family "shipped", never a D-05
// candidate). guardMetrics/paraphraseMetrics are keyed by ranker name;
// shippedGuard/shippedParaphrase are the shipped row's own accumulated
// metrics, measured through store.SearchReranked itself.
func buildSummaries(roster []namedRanker, guardMetrics, paraphraseMetrics map[string]variantMetrics, shippedGuard, shippedParaphrase variantMetrics) []variantSummary {
	rows := make([]variantSummary, 0, len(roster)+1)
	for _, r := range roster {
		rows = append(rows, variantSummary{
			name:             r.name,
			family:           r.family,
			simplicity:       r.simplicity,
			tuned:            r.tuned,
			disabled:         r.rank == nil,
			disabledReason:   r.disabledReason,
			guardRanks:       guardMetrics[r.name].ranks,
			guardAllRank1:    guardMetrics[r.name].allRank1(),
			paraphraseRecall: paraphraseMetrics[r.name].recall(),
			paraphraseMRR:    paraphraseMetrics[r.name].mrr(),
			paraphraseN:      paraphraseMetrics[r.name].n,
		})
	}
	rows = append(rows, variantSummary{
		name:             shippedRowName,
		family:           "shipped",
		guardRanks:       shippedGuard.ranks,
		guardAllRank1:    shippedGuard.allRank1(),
		paraphraseRecall: shippedParaphrase.recall(),
		paraphraseMRR:    shippedParaphrase.mrr(),
		paraphraseN:      shippedParaphrase.n,
	})
	return rows
}

// formatVariantTable renders rows as a Markdown table with a leading
// "| variant |" header — a stable log marker plans 01-05/01-06 grep for —
// so the eval's t.Logf output is both human-readable and greppable. A
// disabled row (Jev today) renders its disabledReason in place of numbers.
// The D-05 eligible column is "yes" for a row named in d.eligible, "no" for
// any other enabled non-shipped row, and "—" for disabled and shipped rows,
// which never compete in D-05. Numbers render with 3 decimals.
func formatVariantTable(rows []variantSummary, d rankingDecision) string {
	eligible := make(map[string]bool, len(d.eligible))
	for _, name := range d.eligible {
		eligible[name] = true
	}

	var b strings.Builder
	// The literal "8" mirrors retrieval_eval_test.go's defaultK (a _test.go
	// constant this non-test file cannot reference without breaking `go
	// build ./...`); both name the production MCP default k.
	b.WriteString("| variant | #261 ranks | #261 all rank 1 | paraphrase recall@8 | paraphrase MRR | D-05 eligible |\n")
	b.WriteString("|---|---|---|---|---|---|\n")
	for _, row := range rows {
		if row.disabled {
			fmt.Fprintf(&b, "| %s | %s | — | — | — | — |\n", row.name, row.disabledReason)
			continue
		}
		elig := "—"
		if row.family != "shipped" {
			if eligible[row.name] {
				elig = "yes"
			} else {
				elig = "no"
			}
		}
		fmt.Fprintf(&b, "| %s | %v | %v | %.3f | %.3f | %s |\n",
			row.name, row.guardRanks, row.guardAllRank1, row.paraphraseRecall, row.paraphraseMRR, elig)
	}
	return b.String()
}

// decideRanking applies D-05's pre-committed decision rule mechanically —
// this is the ONLY thing that picks the shipped ranking; no human or agent
// judgment substitutes for it, and it is committed and unit-tested
// (TestDecideRanking) before any live number exists:
//
//  1. The baseline is the single row with family "vector-only". Its
//     absence is a hard stop: no winner, naming the missing baseline.
//  2. Candidates are every row that is neither disabled nor family
//     "shipped" (disabled and shipped rows never compete in D-05).
//  3. A candidate is eligible when its guardAllRank1 holds AND its
//     paraphraseMRR is at least the baseline's (within mrrEpsilon). This
//     includes the baseline itself, trivially, when its own guardAllRank1
//     holds. d.eligible lists every row that passes this step — what the
//     table shows — independent of step 4's margin filter.
//  4. When the baseline itself is eligible, every TUNED candidate (D-05's
//     margin applies only to the grid — cosine-blend and overlap-gate,
//     never lexical, which is untuned) whose paraphraseMRR falls short of
//     the baseline's by less than d05TunedMargin is dropped from winner
//     consideration — the anti-overfitting guard. No margin applies when
//     the baseline itself is ineligible.
//  5. The winner is the maximum remaining paraphraseMRR; values within
//     mrrEpsilon tie, broken first by the lowest simplicity, then by the
//     lexically smallest name.
//  6. No remaining candidate means no winner, with a reason starting
//     "no eligible variant".
func decideRanking(rows []variantSummary) rankingDecision {
	var baseline *variantSummary
	for i := range rows {
		if rows[i].family == "vector-only" {
			baseline = &rows[i]
			break
		}
	}
	if baseline == nil {
		return rankingDecision{reason: "no vector-only baseline row found — D-05 cannot be applied without it"}
	}

	eligible := make([]variantSummary, 0, len(rows))
	eligibleNames := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.disabled || row.family == "shipped" {
			continue
		}
		if row.guardAllRank1 && row.paraphraseMRR >= baseline.paraphraseMRR-mrrEpsilon {
			eligible = append(eligible, row)
			eligibleNames = append(eligibleNames, row.name)
		}
	}

	baselineEligible := false
	for _, row := range eligible {
		if row.family == "vector-only" {
			baselineEligible = true
			break
		}
	}

	finalists := eligible
	marginDroppedAny := false
	if baselineEligible {
		filtered := make([]variantSummary, 0, len(eligible))
		for _, row := range eligible {
			if row.tuned && row.paraphraseMRR < baseline.paraphraseMRR+d05TunedMargin-mrrEpsilon {
				marginDroppedAny = true
				continue
			}
			filtered = append(filtered, row)
		}
		finalists = filtered
	}

	if len(finalists) == 0 {
		return rankingDecision{eligible: eligibleNames, reason: "no eligible variant: every candidate failed the #261 rank-1 guard or the paraphrase-MRR floor"}
	}

	best := finalists[0]
	for _, row := range finalists[1:] {
		switch {
		case row.paraphraseMRR > best.paraphraseMRR+mrrEpsilon:
			best = row
		case row.paraphraseMRR < best.paraphraseMRR-mrrEpsilon:
			// Strictly worse — never replaces best.
		case row.simplicity < best.simplicity:
			best = row
		case row.simplicity == best.simplicity && row.name < best.name:
			best = row
		}
	}

	reason := "best eligible paraphrase MRR"
	if marginDroppedAny && best.name == baseline.name {
		reason = fmt.Sprintf("vector-only wins: no tuned variant cleared D-05's %.2f margin over vector-only", d05TunedMargin)
	}

	return rankingDecision{winner: best.name, eligible: eligibleNames, reason: reason}
}
