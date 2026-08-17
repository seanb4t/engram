// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

// This file holds ONLY this plan's group fixtures (06-04-PLAN.md: the
// migrate family — migrate, migrate status, migrate revert, and the
// backfill-short-ids alias). Plan 06-07 merges every group's fixture
// function and gates the merged key set against operatorCommands() in
// both directions (06-01-PLAN.md §Conversion Rules R3); this file does not
// attempt that enumeration gate on its own.

import (
	"fmt"
	"testing"

	"github.com/seanb4t/engram/internal/migrate"
	"github.com/seanb4t/engram/internal/store"
)

// migrateViewFixtures returns the migrate-family fixtures the identity gate
// runs against, keyed by commandKey exactly as operatorCommands() produces
// it. Every sample is built by calling the report's real converter, reusing
// the fixed input values from the phase's now-retired parity gate
// (06-CONTEXT.md D-09) where one exists so the fixtures stay comparable to
// what that retired test once exercised — never that test's own
// declared-fact string list or structure.
func migrateViewFixtures() map[string][]any {
	return map[string][]any{
		// Preview (dry-run, mirrors TestMigrateFamilyReportFields' "bare
		// migrate" fixture: WouldMigrate=7, Migrated=0) and applied (the
		// retired parity gate's "migrate" row's exact fixed values, chosen
		// there specifically because it carries non-empty Spared/Appeared
		// so the nested-row path is exercised).
		"migrate": {
			migrateReportDoc(store.MigrateResult{}, migrate.CurrentVersion, true, 7),
			migrateReportDoc(store.MigrateResult{
				Migrated: 23, Failed: 2, Passes: 1, Backlog: 5,
				Spared: []string{"a"}, Appeared: []string{"b", "c"},
			}, migrate.CurrentVersion, false, 26),
		},
		// backfill-short-ids has no report of its own (D-11): it is a thin
		// delegating alias whose preview and apply closures
		// (backfillPreview/backfillApplyRun, cmd/engram/backfill.go:36,:40)
		// call the SAME migrateSweepPreviewRun/migrateSweepApplyRun as
		// `engram migrate`, so this key is deliberately fixtured from
		// migrate's own converter, migrateReportDoc, rather than a
		// backfill-specific one. Its PREVIEW variant is enumerated here
		// specifically because a hand-written variant list missed it once
		// (06-CONTEXT.md <specifics>) — the applied variant reuses the
		// retired parity gate's "backfill-short-ids" row's exact fixed
		// values.
		"backfill-short-ids": {
			migrateReportDoc(store.MigrateResult{}, migrate.CurrentVersion, true, 29),
			migrateReportDoc(store.MigrateResult{Migrated: 29, Backlog: 0}, migrate.CurrentVersion, false, 29),
		},
		// A populated-Future histogram (the retired parity gate's "migrate
		// status" row's exact fixed values) and a zero-valued result, so
		// both the "[]"-never-"null" shape and the has-future shape render.
		"migrate status": {
			statusReportDoc(store.MigrateStatusResult{
				Buckets: []store.VersionBucket{{Version: 1, Count: 40}}, Absent: 3,
				Future: []store.VersionBucket{{Version: 2, Count: 1}}, FutureTotal: 1, Total: 44,
			}),
			statusReportDoc(store.MigrateStatusResult{}),
		},
		// Four shapes: bare preflight refusal (TestMigrateFamilyRevertRefusals'
		// irreversible-range case: an all-zero res), refusal with partial
		// progress (TestMigrateFamilyRevertApplyRefusalReportsPartialProgress's
		// CR-06 fixture: writes already landed before a mid-loop refusal),
		// reversible preview (the retired parity gate's "migrate revert"
		// row's exact fixed values), and applied
		// (TestMigrateFamilyRevertReversible's --apply fixture).
		"migrate revert": {
			revertReportDoc(store.RevertPlan{
				To: 0, Candidates: 1, Reversible: false,
				Irreversible: []store.IrreversibleStepRef{{From: 0, To: 1, Reason: "no declared inverse"}},
			}, false, store.RevertResult{}),
			func() any {
				refusedPlan := store.RevertPlan{
					To: 0, Candidates: 1, Reversible: false,
					Irreversible: []store.IrreversibleStepRef{{From: 0, To: 1, Reason: "no declared inverse"}},
				}
				progressed := store.RevertResult{Reverted: 256, Failed: 2, Passes: 1, Backlog: 44, Plan: refusedPlan}
				return revertReportDoc(refusedPlan, false, progressed)
			}(),
			revertReportDoc(store.RevertPlan{To: 0, Candidates: 12, Reversible: true}, false, store.RevertResult{}),
			func() any {
				plan := store.RevertPlan{To: 0, Candidates: 5, Reversible: true}
				res := store.RevertResult{Reverted: 5, Backlog: 0, Plan: plan}
				return revertReportDoc(res.Plan, true, res)
			}(),
		},
	}
}

// TestMigrateViewIdentity runs the shared identity gate over every
// migrate-family fixture: the same gate that guards prune-expired
// (06-01-PLAN.md), applied to all four commands and all ten of their
// document variants.
func TestMigrateViewIdentity(t *testing.T) {
	for name, docs := range migrateViewFixtures() {
		for i, doc := range docs {
			t.Run(fmt.Sprintf("%s/%d", name, i), func(t *testing.T) {
				assertViewIdentity(t, name, doc)
			})
		}
	}
}
