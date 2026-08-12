// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"testing"

	"github.com/seanb4t/engram/internal/store"
)

// TestRemapOwnerFlagValidation covers the rows buildRemapSource itself still
// owns after D-07: migrateRemapOwnerCmd's declared cobra flag groups
// (MarkFlagsMutuallyExclusive + MarkFlagsOneRequired over
// from/from-missing/from-anon) now guarantee exactly one of the three is
// supplied before this function ever runs, so the old "no source"/"two
// sources" selected-count rows describe a rule cobra owns — they moved to
// end-to-end coverage in flaggroup_test.go's TestFlagGroupMigrateSourceExactlyOne.
// What remains here is what buildRemapSource itself still validates: the
// residual empty-string --from case cobra's flag groups cannot express (a supplied
// flag with an unusable value), and store.ValidateOwnerRemap's own
// conditions (empty --to, identical --from/--to).
func TestRemapOwnerFlagValidation(t *testing.T) {
	cases := []struct {
		name          string
		from          string
		missing, anon bool
		to            string
		wantErr       bool
	}{
		{"missing ok", "", true, false, "x", false},
		{"anon ok", "", false, true, "x", false},
		{"from ok", "old", false, false, "x", false},
		{"empty to", "", true, false, "", true},
		{"ambiguous empty from", "", false, false, "x", true},
		{"from==to", "x", false, false, "x", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src, err := buildRemapSource(c.from, c.missing, c.anon, c.to)
			if (err != nil) != c.wantErr {
				t.Errorf("buildRemapSource(%+v) err=%v, wantErr=%v", c, err, c.wantErr)
			}
			// On success, pin which concrete store.OwnerRemapSource
			// buildRemapSource wired the flags to — comparable structs, so a
			// plain == against the exported constructors both confirms the
			// variant AND its wrapped value (e.g. RemapFrom("old")), unlike a
			// type-name-only check.
			if err == nil {
				var want store.OwnerRemapSource
				switch {
				case c.missing:
					want = store.RemapMissing()
				case c.anon:
					want = store.RemapAnon()
				default:
					want = store.RemapFrom(c.from)
				}
				if src != want {
					t.Errorf("buildRemapSource(%+v) = %v, want %v", c, src, want)
				}
			}
		})
	}
}

// TestMigrateSetOwnerRejectsInvalidOutput proves `migrate-set-owner`
// validates --output through the shared operatorOutputFormat before
// dialing any store.
func TestMigrateSetOwnerRejectsInvalidOutput(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, migrateSetOwnerCmd)
	_, _, err := runClient(t, "migrate-set-owner", "--owner", "x", "--output", "yaml")
	if err == nil {
		t.Fatal("expected an error for --output yaml, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

// TestMigrateRemapOwnerRejectsInvalidOutput proves `migrate-remap-owner`
// validates --output through the shared operatorOutputFormat before
// dialing any store.
func TestMigrateRemapOwnerRejectsInvalidOutput(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, migrateRemapOwnerCmd)
	_, _, err := runClient(t, "migrate-remap-owner", "--from-missing", "--to", "x", "--output", "yaml")
	if err == nil {
		t.Fatal("expected an error for --output yaml, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

// TestMigrateRemapDryRunJSONDistinguishesPreviewFromApplied proves
// migrate-remap-owner's json document carries the dry-run vs applied
// distinction as an explicit boolean plus SEPARATE count fields, never
// inferred from the text sentence's "[dry-run]" prose prefix (T-03-10's
// mitigation): dry-run yields dry_run=true, would_remap=n, remapped=0;
// applied yields dry_run=false, would_remap=0, remapped=n.
func TestMigrateRemapDryRunJSONDistinguishesPreviewFromApplied(t *testing.T) {
	dry := migrateRemapDoc(5, "new-owner", true)
	if !dry.DryRun || dry.WouldRemap != 5 || dry.Remapped != 0 {
		t.Errorf("migrateRemapDoc(5, _, true) = %+v, want DryRun=true WouldRemap=5 Remapped=0", dry)
	}
	applied := migrateRemapDoc(5, "new-owner", false)
	if applied.DryRun || applied.WouldRemap != 0 || applied.Remapped != 5 {
		t.Errorf("migrateRemapDoc(5, _, false) = %+v, want DryRun=false WouldRemap=0 Remapped=5", applied)
	}

	b, err := json.Marshal(dry)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if m["dry_run"] != true {
		t.Errorf(`m["dry_run"] = %v, want true`, m["dry_run"])
	}
	if m["remapped"] != float64(0) {
		t.Errorf(`m["remapped"] = %v, want 0 (a preview must not report as applied)`, m["remapped"])
	}
}

// TestMigrateSetOwnerSummaryUnchanged pins migrate-set-owner's pure text
// formatter against its pre-backfill literal sentence — migrate-set-owner is
// not classified destructive and is unaffected by this plan's --apply flip.
func TestMigrateSetOwnerSummaryUnchanged(t *testing.T) {
	got := migrateSetOwnerSummary("alice", 3)
	want := "stamped owner=alice onto 3 owner-less record(s)"
	if got != want {
		t.Errorf("migrateSetOwnerSummary(\"alice\", 3) = %q, want %q", got, want)
	}
}

// TestMigrateRemapSummary pins migrate-remap-owner's pure text formatter
// against its NEW (03-03-PLAN.md D-02/D-04) preview/applied sentences — the
// preview wording changed from the pre-plan "[dry-run] would remap ..." to
// name --apply explicitly, since --dry-run no longer exists on this command.
func TestMigrateRemapSummary(t *testing.T) {
	if got, want := migrateRemapSummary(3, "alice", true), "preview: 3 record(s) eligible to remap to owner=alice; re-run with --apply to remap"; got != want {
		t.Errorf("migrateRemapSummary(3, \"alice\", true) = %q, want %q", got, want)
	}
	if got, want := migrateRemapSummary(3, "alice", false), "remapped 3 record(s) to owner=alice"; got != want {
		t.Errorf("migrateRemapSummary(3, \"alice\", false) = %q, want %q", got, want)
	}
}
