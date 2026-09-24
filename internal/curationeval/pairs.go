// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package curationeval

import (
	"fmt"
	"strings"

	"github.com/seanb4t/engram/internal/verdict"
)

// This file is CUR-03's committed labeled pair corpus (D-01, D-02): short,
// synthetic notes about fictional software projects, in pairs, each
// carrying an intended relation label from verdict.Relations(). D-02's
// independence discipline: the executor that writes syntheticPairs knows
// every intended label, so it cannot also be the blind labeler. Instead, a
// fresh, tool-less context labels every pair from blindLabelPrompt's text
// alone (id, recordA, recordB — never label, proven by
// TestBlindLabelPromptIsLabelIndependent), and only pairs where the
// author's intended label and the blind label agree are kept.
//
// Procedure as executed (plan 03-04):
//   - Task 1 authored 5 pairs (P01-P05), one per relation, proving the
//     corpus shape and the integrity rules.
//   - Task 2 extended the corpus to 80 pairs (P01-P80), 16 per relation.
//   - Task 3 records the blind-labeling outcome here: pairs blind-labeled,
//     pairs kept (per-relation counts), dropped pair ids, the labeling
//     date, and the prompt's commit SHA. Until Task 3 runs, syntheticPairs
//     holds every authored pair, not yet filtered by agreement.

// labeledPair is one CUR-03 fixture pair: two short notes about the same
// or different fictional projects, recordB written after recordA, and the
// relation label from verdict.Relations() the author intended (and, after
// Task 3, the blind pass agreed with).
type labeledPair struct {
	id, label, recordA, recordB string
}

// syntheticPairs is the committed CUR-03 corpus (D-01). See this file's
// header comment for the authoring/blind-check procedure.
var syntheticPairs = []labeledPair{
	{id: "P01", label: "duplicate", recordA: "The mobile client retries a failed upload three times with exponential backoff before giving up.", recordB: "Uploads from the mobile client get retried three times with backoff, and then the client gives up."},
	{id: "P02", label: "contradicts", recordA: "The billing API's request timeout for calls to the payment processor is five seconds.", recordB: "The billing API's request timeout for calls to the payment processor is thirty seconds, not five; the five-second figure was wrong."},
	{id: "P03", label: "updates", recordA: "The ledger service's payout table has no currency column, so every amount is assumed to be in USD.", recordB: "The ledger service's payout table now has a currency column, added in this quarter's schema migration."},
	{id: "P04", label: "related", recordA: "The ingest worker logs each batch's record count at info level after every successful flush.", recordB: "The ingest worker also emits a counter metric for the number of batches flushed, alongside the log line."},
	{id: "P05", label: "unrelated", recordA: "The catalog service rebuilds its search index nightly from the primary Postgres table.", recordB: "The mobile client resets its push notification badge count to zero once the app is foregrounded."},
}

// blindLabelMarker delimits the orchestrator's own provenance prose (above
// the line) from the text a blind labeler must receive verbatim (below
// it) — the same convention 01-BLIND-QUERY-PROMPT.md established.
const blindLabelMarker = "---8<--- SEND EVERYTHING BELOW THIS LINE VERBATIM AS THE ENTIRE PROMPT ---8<---"

// blindLabelPrompt renders pairs into the D-02 blind labeler's entire
// input: an instruction paragraph, every relation's name and criteria
// (verdict.RelationCriteria(), verbatim, in verdict.Relations() order),
// then one heading and both record texts per pair. It reads only id,
// recordA and recordB from each pair — never label — so its output cannot
// vary with any pair's intended label (proven by
// TestBlindLabelPromptIsLabelIndependent).
func blindLabelPrompt(pairs []labeledPair) string {
	var b strings.Builder

	b.WriteString("Below are pairs of short notes from software teams' shared project memory. You have not seen where they came from. In every pair, record_b was written after record_a. For each pair, choose exactly ONE relation describing how record_b relates to record_a, using only the five relation names below. Do not use any tools and do not read any files. Answer from this message alone. Reply with exactly one line per pair in the form `- Pnn: <relation>`, in the same order, using only the five names, and nothing else.\n\n")

	criteria := verdict.RelationCriteria()
	for _, name := range verdict.Relations() {
		fmt.Fprintf(&b, "- %s: %s\n", name, criteria[name])
	}
	b.WriteString("\n")

	for _, p := range pairs {
		fmt.Fprintf(&b, "### %s\n", p.id)
		fmt.Fprintf(&b, "record_a: %s\n", p.recordA)
		fmt.Fprintf(&b, "record_b: %s\n\n", p.recordB)
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}
