// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

// This file holds ONLY this plan's (06-03) group fixtures: reindex,
// summarize-missing, migrate-set-owner and migrate-remap-owner. Plan 06-07
// merges every group's fixture function (this one, prune's, and the other
// conversion plans') into a single operatorViewFixtures and gates the
// merged key set against operatorCommands() in BOTH directions — adding a
// command without adding its fixture here is exactly what that gate exists
// to catch.

import (
	"fmt"
	"testing"

	"github.com/seanb4t/engram/internal/store"
)

// flatViewFixtures returns one sample document per sentence variant for
// each of this plan's four flat operator reports, keyed by commandKey as
// operatorCommands() produces it. Every sample is built by calling the
// report's own real converter (or, for migrate-set-owner, the same inline
// struct literal shape its call site uses) — never a hand-rolled map — and
// the input values are reused from the phase's now-retired parity gate's
// fixed values (06-CONTEXT.md D-09) for these four commands so the
// fixtures stay comparable to what that retired test once exercised.
func flatViewFixtures() map[string][]any {
	reindexRes := store.ReindexResult{Scanned: 57, Upserted: 23, Skipped: 11, Unchanged: 19}
	summarizeRes := store.SummarizeResult{Scanned: 41, Filled: 17, Skipped: 20, Failed: 4}

	return map[string][]any{
		"reindex": {
			reindexReportDoc(reindexRes, "target-coll", 1536, true, false),  // dry-run
			reindexReportDoc(reindexRes, "target-coll", 1536, true, true),   // dry-run --resume
			reindexReportDoc(reindexRes, "target-coll", 1536, false, false), // cutover
		},
		"summarize-missing": {
			summarizeReportDoc(summarizeRes, true),  // dry-run
			summarizeReportDoc(summarizeRes, false), // applied
		},
		"migrate-set-owner": {
			migrateSetOwnerReportDoc{Owner: "bob", Stamped: 7},
		},
		"migrate-remap-owner": {
			migrateRemapDoc(13, "alice", true),  // preview
			migrateRemapDoc(13, "alice", false), // applied
		},
	}
}

// TestFlatViewIdentity runs the shared identity gate (assertViewIdentity,
// defined in operator_view_test.go by plan 06-01) over every fixture in
// flatViewFixtures: the same non-vacuous correspondence-plus-line-count
// check that guards prune-expired, now covering this plan's four reports.
func TestFlatViewIdentity(t *testing.T) {
	for name, docs := range flatViewFixtures() {
		for i, doc := range docs {
			t.Run(fmt.Sprintf("%s/%d", name, i), func(t *testing.T) {
				assertViewIdentity(t, name, doc)
			})
		}
	}
}
