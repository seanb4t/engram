// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package curationeval

import (
	"context"

	"github.com/seanb4t/engram/internal/decide"
	"github.com/seanb4t/engram/internal/verdict"
)

// prediction is one labeled pair's gold label alongside the Verdict the
// decider produced for it (D-06). No record text lives on this type —
// formatReport (plan 03-07 Task 2) receives predictions only, never pairs.
type prediction struct {
	gold string
	v    verdict.Verdict
}

// evaluate sends every pair through the SAME shared verdict contract
// spine-review consolidate uses (D-06): one verdict.NewRequest per pair
// (recordB is already the later note, matching consolidate's
// newer-is-record_b rule), one dec.DecideMany call over the whole batch,
// then verdict.FromResult per result, index aligned. The returned slice has
// exactly one prediction per pair, in the same order, with gold equal to
// the pair's intended label. evaluate performs no I/O of its own beyond the
// single DecideMany call — no text leaves this function except inside the
// requests it builds.
func evaluate(ctx context.Context, dec decide.Decider, pairs []labeledPair, threshold float64, stateChars int) []prediction {
	reqs := make([]decide.Request, len(pairs))
	for i, p := range pairs {
		reqs[i] = verdict.NewRequest(
			verdict.State("", p.recordA, stateChars),
			verdict.State("", p.recordB, stateChars),
		)
	}

	results := dec.DecideMany(ctx, reqs)

	preds := make([]prediction, len(pairs))
	for i, p := range pairs {
		preds[i] = prediction{gold: p.label, v: verdict.FromResult(results[i], threshold)}
	}
	return preds
}
