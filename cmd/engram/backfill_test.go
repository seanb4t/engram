// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/seanb4t/engram/internal/migrate"
	"github.com/seanb4t/engram/internal/store"
)

// lastStdoutLine strips cobra's own "Command %q is deprecated, %s" notice
// (command.go's execute(), printed via c.Printf -> c.OutOrStderr()) from a
// runClient stdout capture. In production this notice lands on os.Stderr
// (rootCmd never calls SetOut, so OutOrStderr's walk-to-parent falls back to
// the real os.Stderr default) -- it only shares runClient's captured stdout
// buffer here because the test harness explicitly wires rootCmd.SetOut to
// the SAME buffer runClient labels "stdout", which is a test-harness
// artifact, not a production stdout/stderr mixing defect. The alias's own
// JSON document is always the LAST line.
func lastStdoutLine(stdout string) string {
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	return lines[len(lines)-1]
}

// TestBackfillCmdFlagSet proves the alias contract's flag surface (D-09/H8):
// no --dry-run (removed outright), but --timeout is PRESERVED.
func TestBackfillCmdFlagSet(t *testing.T) {
	if backfillShortIDsCmd.Flags().Lookup("dry-run") != nil {
		t.Error("backfill-short-ids carries a --dry-run flag, want it removed (D-09)")
	}
	if backfillShortIDsCmd.Flags().Lookup("timeout") == nil {
		t.Error("backfill-short-ids has no --timeout flag, want it PRESERVED (REVIEWS.md H8)")
	}
	if backfillShortIDsCmd.Flags().Lookup("apply") == nil {
		t.Error("backfill-short-ids has no --apply flag, want one via registerDestructive")
	}
}

// TestBackfillRejectsInvalidOutput proves `backfill-short-ids` validates
// --output through the shared operatorOutputFormat before dialing any
// store — the format check is the first statement in the shared
// migrateSweepPreviewRun/migrateSweepApplyRun this alias delegates to.
func TestBackfillRejectsInvalidOutput(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, backfillShortIDsCmd)
	_, _, err := runClient(t, "backfill-short-ids", "--output", "yaml")
	if err == nil {
		t.Fatal("expected an error for --output yaml, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

// TestBackfillPreviewsByDefaultAndSharesMigrateEnvelope proves the D-09/D-11
// behavior reversal: a bare invocation previews (DryRun, no writes) using
// the SHARED migrate report envelope (migrateOutputDoc), never the deleted
// backfill-specific one.
func TestBackfillPreviewsByDefaultAndSharesMigrateEnvelope(t *testing.T) {
	fake := &fakeMigrateFamilyStore{
		migrateFn: func(_ context.Context, opts store.MigrateOptions) (store.MigrateResult, error) {
			if !opts.DryRun {
				t.Fatalf("bare invocation: opts.DryRun = false, want true")
			}
			return store.MigrateResult{PreviewManifest: map[string]migrate.Version{"a": 0, "b": 0}}, nil
		},
	}
	withFakeMigrateFamilyStore(t, fake)
	resetClientFlags(t)
	resetCommandFlagState(t, backfillShortIDsCmd)

	stdout, _, err := runClient(t, "backfill-short-ids", "--output", "json")
	if err != nil {
		t.Fatalf("backfill-short-ids: %v", err)
	}
	var doc migrateOutputDoc
	if uErr := json.Unmarshal([]byte(lastStdoutLine(stdout)), &doc); uErr != nil {
		t.Fatalf("json.Unmarshal: %v (stdout=%q)", uErr, stdout)
	}
	if !doc.DryRun || doc.WouldMigrate != 2 || doc.Migrated != 0 {
		t.Errorf("doc = %+v, want DryRun=true WouldMigrate=2 Migrated=0", doc)
	}
	if len(fake.migrateCalls) != 1 {
		t.Fatalf("Migrate called %d time(s), want 1 (preview only, no write)", len(fake.migrateCalls))
	}
}

// TestBackfillApplyPerformsSharedEnvelope proves --apply executes with the
// shared envelope (dry_run:false).
func TestBackfillApplyPerformsSharedEnvelope(t *testing.T) {
	fake := &fakeMigrateFamilyStore{
		migrateFn: func(_ context.Context, opts store.MigrateOptions) (store.MigrateResult, error) {
			if opts.DryRun {
				return store.MigrateResult{PreviewManifest: map[string]migrate.Version{"a": 0}}, nil
			}
			if opts.Manifest == nil {
				t.Fatalf("apply call: opts.Manifest is nil, want non-nil")
			}
			return store.MigrateResult{Migrated: uint64(len(opts.Manifest))}, nil
		},
	}
	withFakeMigrateFamilyStore(t, fake)
	resetClientFlags(t)
	resetCommandFlagState(t, backfillShortIDsCmd)

	stdout, _, err := runClient(t, "backfill-short-ids", "--apply", "--output", "json")
	if err != nil {
		t.Fatalf("backfill-short-ids --apply: %v", err)
	}
	var doc migrateOutputDoc
	if uErr := json.Unmarshal([]byte(lastStdoutLine(stdout)), &doc); uErr != nil {
		t.Fatalf("json.Unmarshal: %v (stdout=%q)", uErr, stdout)
	}
	if doc.DryRun {
		t.Errorf("doc.DryRun = true, want false")
	}
	if doc.Migrated != 1 {
		t.Errorf("doc.Migrated = %d, want 1", doc.Migrated)
	}
}

// TestBackfillApplyPathParityWithMigrateApply is the load-bearing new
// assertion (REVIEWS.md cycle-3 #7): backfill-short-ids's --apply and
// `engram migrate --apply` must record the IDENTICAL two-call Migrate
// sequence -- first DryRun:true with a nil Manifest, then a non-nil
// Manifest equal to the first call's returned PreviewManifest and
// DryRun:false. This is the write-the-cheapest-passing-violator-first
// check: an alias that called Migrate once with {DryRun:false} would record
// a one-call sequence and fail this test, whereas an "the alias produced
// some output" assertion would not.
func TestBackfillApplyPathParityWithMigrateApply(t *testing.T) {
	previewManifest := map[string]migrate.Version{"a": 0, "b": 0}
	newFake := func() *fakeMigrateFamilyStore {
		return &fakeMigrateFamilyStore{
			migrateFn: func(_ context.Context, opts store.MigrateOptions) (store.MigrateResult, error) {
				if opts.DryRun {
					return store.MigrateResult{PreviewManifest: previewManifest}, nil
				}
				return store.MigrateResult{Migrated: uint64(len(opts.Manifest))}, nil
			},
		}
	}

	backfillFake := newFake()
	withFakeMigrateFamilyStore(t, backfillFake)
	resetClientFlags(t)
	resetCommandFlagState(t, backfillShortIDsCmd)
	if _, _, err := runClient(t, "backfill-short-ids", "--apply", "--output", "json"); err != nil {
		t.Fatalf("backfill-short-ids --apply: %v", err)
	}

	migrateFake := newFake()
	withFakeMigrateFamilyStore(t, migrateFake)
	resetClientFlags(t)
	resetCommandFlagState(t, migrateCmd)
	if _, _, err := runClient(t, "migrate", "--apply", "--output", "json"); err != nil {
		t.Fatalf("migrate --apply: %v", err)
	}

	if len(backfillFake.migrateCalls) != 2 {
		t.Fatalf("backfill-short-ids --apply: Migrate called %d time(s), want 2", len(backfillFake.migrateCalls))
	}
	if len(migrateFake.migrateCalls) != 2 {
		t.Fatalf("migrate --apply: Migrate called %d time(s), want 2", len(migrateFake.migrateCalls))
	}
	for i := range 2 {
		bc, mc := backfillFake.migrateCalls[i], migrateFake.migrateCalls[i]
		if bc.DryRun != mc.DryRun {
			t.Errorf("call %d: backfill DryRun=%v, migrate DryRun=%v, want equal", i, bc.DryRun, mc.DryRun)
		}
	}
	if !backfillFake.migrateCalls[0].DryRun || backfillFake.migrateCalls[0].Manifest != nil {
		t.Errorf("backfill call 0 = %+v, want DryRun:true Manifest:nil", backfillFake.migrateCalls[0])
	}
	if backfillFake.migrateCalls[1].DryRun || backfillFake.migrateCalls[1].Manifest == nil {
		t.Errorf("backfill call 1 = %+v, want DryRun:false Manifest:non-nil", backfillFake.migrateCalls[1])
	}
}

// TestBackfillTimeoutWiring is the three-case behavioural proof the
// preserved --timeout flag is actually READ (REVIEWS.md H8/N3): a
// hard-coded 5-minute helper passes the default case but fails the 1s case.
func TestBackfillTimeoutWiring(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantNear time.Duration
		wantFar  time.Duration
		wantNone bool
	}{
		{name: "--timeout 1s", args: []string{"backfill-short-ids", "--timeout", "1s", "--output", "json"}, wantNear: 2 * time.Second},
		{name: "default timeout", args: []string{"backfill-short-ids", "--output", "json"}, wantFar: 4 * time.Minute},
		{name: "--timeout 0", args: []string{"backfill-short-ids", "--timeout", "0", "--output", "json"}, wantNone: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var gotDeadline time.Time
			var gotOK bool
			fake := &fakeMigrateFamilyStore{
				migrateFn: func(ctx context.Context, _ store.MigrateOptions) (store.MigrateResult, error) {
					gotDeadline, gotOK = ctx.Deadline()
					return store.MigrateResult{}, nil
				},
			}
			withFakeMigrateFamilyStore(t, fake)
			resetClientFlags(t)
			resetCommandFlagState(t, backfillShortIDsCmd)

			if _, _, err := runClient(t, tc.args...); err != nil {
				t.Fatalf("%v: %v", tc.args, err)
			}

			switch {
			case tc.wantNone:
				if gotOK {
					t.Errorf("ctx.Deadline() ok = true, want false (--timeout 0 disables the deadline)")
				}
			case tc.wantNear > 0:
				if !gotOK {
					t.Fatal("ctx.Deadline() ok = false, want true")
				}
				if d := time.Until(gotDeadline); d > tc.wantNear {
					t.Errorf("time.Until(deadline) = %v, want <= %v -- a hard-coded default duration cannot pass this case", d, tc.wantNear)
				}
			case tc.wantFar > 0:
				if !gotOK {
					t.Fatal("ctx.Deadline() ok = false, want true")
				}
				if d := time.Until(gotDeadline); d <= tc.wantFar {
					t.Errorf("time.Until(deadline) = %v, want > %v (the ~5m default is applied, not zero deadline)", d, tc.wantFar)
				}
			}
		})
	}
}

// TestBackfillDeprecatedPointsAtMigrate proves the soft-deprecation (D-12):
// the command is never removed, but names its replacement.
func TestBackfillDeprecatedPointsAtMigrate(t *testing.T) {
	if backfillShortIDsCmd.Deprecated == "" {
		t.Fatal("backfillShortIDsCmd.Deprecated is empty, want a non-empty pointer to migrate")
	}
	const want = "use: engram migrate"
	if backfillShortIDsCmd.Deprecated != want {
		t.Errorf("backfillShortIDsCmd.Deprecated = %q, want %q", backfillShortIDsCmd.Deprecated, want)
	}
}

// TestBackfillPreBackfilledRecordDelegates discharges M10 by composition
// (REVIEWS.md C6-H7): rather than a CLI-lane real-Qdrant test (cmd/engram
// has zero container harness, and the fixture -- short_id present,
// schema_version absent -- can only be constructed with a raw client, which
// is why internal/store ships seedLegacyRecord), this test proves the
// SECOND half of the composed proof at the CLI boundary: both a bare and an
// --apply invocation of the alias reach migrateSweepPreviewRun/
// migrateSweepApplyRun with this leaf's own backfillOutput/backfillTimeout,
// and no store.MigrateOptions value originates in backfill.go -- it always
// asks the shared sweep to derive its own options. The FIRST half -- that
// the sweep itself correctly converges this exact record state -- is
// 04-01 Task 3's TestMigrateExistingShortIDPreserves, against real pinned
// Qdrant at the store layer. Composed, the two prove the whole claim: the
// sweep handles the state, and the alias invokes exactly that sweep.
func TestBackfillPreBackfilledRecordDelegates(t *testing.T) {
	t.Run("bare invocation reaches migrateSweepPreviewRun", func(t *testing.T) {
		fake := &fakeMigrateFamilyStore{
			migrateFn: func(_ context.Context, opts store.MigrateOptions) (store.MigrateResult, error) {
				if !opts.DryRun {
					t.Fatalf("bare invocation: opts.DryRun = false, want true (only migrateSweepPreviewRun sets this)")
				}
				// short_id present / schema_version absent converges with
				// zero eligible records once already at current -- an
				// empty manifest here stands in for "already converged."
				return store.MigrateResult{PreviewManifest: map[string]migrate.Version{}}, nil
			},
		}
		withFakeMigrateFamilyStore(t, fake)
		resetClientFlags(t)
		resetCommandFlagState(t, backfillShortIDsCmd)

		if _, _, err := runClient(t, "backfill-short-ids", "--output", "json"); err != nil {
			t.Fatalf("backfill-short-ids: %v", err)
		}
		if len(fake.migrateCalls) != 1 {
			t.Fatalf("Migrate called %d time(s), want 1", len(fake.migrateCalls))
		}
	})

	t.Run("--apply reaches migrateSweepApplyRun with no options constructed in backfill.go", func(t *testing.T) {
		fake := &fakeMigrateFamilyStore{
			migrateFn: func(_ context.Context, opts store.MigrateOptions) (store.MigrateResult, error) {
				if opts.DryRun {
					return store.MigrateResult{PreviewManifest: map[string]migrate.Version{}}, nil
				}
				if opts.Manifest == nil {
					t.Fatalf("apply call: opts.Manifest is nil, want the fresh manifest migrateSweepApplyRun derives")
				}
				return store.MigrateResult{Migrated: 0}, nil
			},
		}
		withFakeMigrateFamilyStore(t, fake)
		resetClientFlags(t)
		resetCommandFlagState(t, backfillShortIDsCmd)

		if _, _, err := runClient(t, "backfill-short-ids", "--apply", "--output", "json"); err != nil {
			t.Fatalf("backfill-short-ids --apply: %v", err)
		}
		if len(fake.migrateCalls) != 2 {
			t.Fatalf("Migrate called %d time(s), want 2 (H5 in-apply-closure re-preview)", len(fake.migrateCalls))
		}
	})
}
