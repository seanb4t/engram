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
// candidate pool (store.CandidateK(defaultK)). This is the T1 form:
// vector-only, lexical, and a disabled Jev stub. Plan 01-04 Task 2 extends
// this with the D-07 overlap-gate/cosine-blend grid; Phase 4 enables Jev by
// giving its stub entry a rank function — both are pure appends, never a
// refactor of this function's shape or the eval loop that consumes it.
func evalRankers() []namedRanker {
	return []namedRanker{
		{
			name:       "vector-only",
			family:     "vector-only",
			simplicity: 0,
			rank: func(_ string, pool []store.Memory, k int) []store.Memory {
				return store.VectorOrder(pool, k)
			},
		},
		{
			name:       "lexical",
			family:     "lexical",
			simplicity: 10,
			rank:       lexicalRerank,
		},
		{
			name:           "jev",
			family:         "jev",
			simplicity:     99,
			disabledReason: jevDisabledReason,
		},
	}
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
// ranker (if any) ships, which rows were eligible, and why. Task 1 always
// passes the zero value (decideRanking does not exist until Task 2); every
// cell it drives in formatVariantTable then reads "no" or "—". winner and
// reason are consumed starting Task 2 (decideRanking's return value and the
// eval's "D-05 decision" log line) — the nolint below is temporary,
// removed in the same commit that wires their first real reader.
type rankingDecision struct {
	winner   string //nolint:unused // consumed by decideRanking + the D-05 log line, plan 01-04 Task 2
	eligible []string
	reason   string //nolint:unused // consumed by decideRanking + the D-05 log line, plan 01-04 Task 2
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
