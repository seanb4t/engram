// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file holds ONLY this plan's (06-06) group fixtures: spine-review
// scan, spine-review consolidate, and spine-review verify. Plan 06-07
// merges every group's fixture function (this one plus prune-expired's
// pruneViewFixtures and every sibling group's own <group>ViewFixtures)
// and adds the both-directions enumeration gate against operatorCommands()
// (06-01-PLAN.md §Conversion Rules R3; 06-CONTEXT.md D-09).
package main

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/seanb4t/engram/internal/store"
)

// spineViewFixtures returns the fixtures the identity gate runs against
// for this plan's three commands, keyed by commandKey exactly as
// operatorCommands() produces it. Every sample is built by calling the
// report's real converter (spineScanDoc/consolidateDoc/verifyDoc) over
// fixed input values -- reusing the numeric/string literals the phase's
// now-retired parity gate (06-CONTEXT.md D-09) already established for
// these three commands, never that retired test's declared-value list or
// any other part of its structure.
func spineViewFixtures() map[string][]any {
	scanRes := store.SpineScanResult{
		Total: 9, Owners: 3, WithSummary: 5, WithoutSummary: 4,
		Superseded: 1, Expired: 1, Scheduled: 0, Archived: 1, WithCitations: 2, Citations: 6,
		ByScopeCategory: []store.ScopeCategoryCount{{Scope: "s", Category: "note", Count: 3}},
	}

	consolidatePairs := []store.DuplicatePair{
		{A: "id-a", B: "id-b", AShortID: "sa", BShortID: "sb", AScope: "s", BScope: "s", Score: 0.5},
	}
	consolidateMinScore := float32(0.5)

	verifiedAt := time.Date(2031, 6, 15, 12, 0, 0, 0, time.UTC)
	verifyPopulated := verifyReport{
		ScannedAt:  verifiedAt,
		ValidCount: 2, MovedCount: 1, BrokenCount: 1, UnverifiableCount: 1,
		Moved:        []verifyEntry{{RecordID: "rec-moved", ShortID: "short-moved", Ref: "a.go", Reason: "excerpt found at byte offset 12, not at the cited locator"}},
		Broken:       []verifyEntry{{RecordID: "rec-broken", ShortID: "short-broken", Ref: "b.go", Reason: reasonFileMissing}},
		Unverifiable: []verifyEntry{{RecordID: "rec-unverifiable", ShortID: "short-unverifiable", Ref: "c.go", Reason: "different repo"}},
	}

	return map[string][]any{
		"spine-review scan": {
			// A named scope with a non-empty ByScopeCategory breakdown.
			spineScanDoc(scanRes, "s"),
			// A zero-valued result with an empty scope: exercises both the
			// all-scopes encoding and the []-never-null breakdown.
			spineScanDoc(store.SpineScanResult{}, ""),
		},
		"spine-review consolidate": {
			// A non-nil minScore with at least one candidate pair.
			consolidateDoc(consolidatePairs, "s", false, &consolidateMinScore, 5, 9, 9),
			// A nil minScore with zero candidates: exercises the omitempty
			// key-absence path and the empty-array path together.
			consolidateDoc(nil, "", true, nil, 5, 0, 0),
		},
		"spine-review verify": {
			// At least one entry in each of the three non-valid tiers.
			verifyDoc(verifyPopulated),
			// A zero-valued report: three empty entry-row lists.
			verifyDoc(verifyReport{}),
		},
	}
}

// TestSpineViewIdentity runs the shared identity gate over every fixture
// this group declares.
func TestSpineViewIdentity(t *testing.T) {
	fixtures := spineViewFixtures()
	for name, docs := range fixtures {
		for i, doc := range docs {
			t.Run(fmt.Sprintf("%s/%d", name, i), func(t *testing.T) {
				assertViewIdentity(t, name, doc)
			})
		}
	}

	// This group's own additional assertion: for the nil-minScore
	// consolidate fixture, the rendered output must carry exactly one
	// fewer top-level field line than the non-nil fixture -- a COUNT
	// comparison, never a label-text comparison, so this stays invariant
	// to the humanizer per D-06 (06-CONTEXT.md).
	t.Run("spine-review consolidate/min_score omitempty field count", func(t *testing.T) {
		consolidateDocs := fixtures["spine-review consolidate"]
		if len(consolidateDocs) != 2 {
			t.Fatalf("spine-review consolidate fixtures = %d, want 2", len(consolidateDocs))
		}
		withMinScore, withoutMinScore := consolidateDocs[0], consolidateDocs[1]

		var bufWith, bufWithout bytes.Buffer
		if err := renderOperatorView(&bufWith, "headline", withMinScore); err != nil {
			t.Fatalf("renderOperatorView(withMinScore): %v", err)
		}
		if err := renderOperatorView(&bufWithout, "headline", withoutMinScore); err != nil {
			t.Fatalf("renderOperatorView(withoutMinScore): %v", err)
		}
		gotWith := countTopLevelFieldLines(bufWith.String())
		gotWithout := countTopLevelFieldLines(bufWithout.String())
		if gotWithout != gotWith-1 {
			t.Errorf("countTopLevelFieldLines(nil-minScore rendered) = %d, want %d (one fewer top-level field line than the non-nil fixture's %d)", gotWithout, gotWith-1, gotWith)
		}
	})
}
