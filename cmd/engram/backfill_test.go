// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
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

// timeoutRemovalClaimPattern matches a CLAIM that --timeout was removed,
// within a single sentence CLAUSE (bounded by "." or ";"), never a bare
// co-occurrence of the two words (REVIEWS.md C5-L1, C6-H1): the guide's own
// permitted wording -- "`--timeout` is preserved; `--dry-run` is removed"
// -- puts a semicolon between the two, so this pattern does not fire on a
// compliant document at any sentence length, unlike a character-count
// window.
var timeoutRemovalClaimPattern = regexp.MustCompile(`--timeout[^.;]*\b(removed|no longer accepted|dropped)\b`)

// backfillShortIDsDryRunCombo matches the SPECIFIC stale instruction this
// gate exists to catch -- `backfill-short-ids` combined with `--dry-run`,
// in either order, within one line -- rather than a bare `--dry-run`
// search. A whole-guide bare `--dry-run` search is REJECTED here
// (REVIEWS.md-class defect, same shape as the whole-file --timeout grep
// C5-L1 rejects): summarize-missing and reindex each carry their OWN,
// still-valid `--dry-run` flag with legitimate, unrelated examples
// elsewhere in this guide (confirmed present before this plan touched the
// file), so a bare "--dry-run" search would fail on those unrelated,
// compliant mentions and could never go green -- the exact "gate that
// cannot pass on a compliant implementation" defect class this phase's own
// prohibitions repeatedly reject. The property this gate actually proves
// is narrower and correct: no HISTORICAL section still instructs THIS
// command with a flag it no longer accepts.
var backfillShortIDsDryRunCombo = regexp.MustCompile(`backfill-short-ids\b[^\n]{0,40}--dry-run|--dry-run\b[^\n]{0,40}backfill-short-ids`)

// TestUpgradeGuideReconcilesBackfill is the D-12 bidirectional doc<->code
// gate (memory x6v6qxqd6f: never a len>0/presence proxy). Four assertions,
// each independently provable RED by reverting its own half:
//
//  1. doc side: the "## Unreleased" section names the --dry-run removal
//     and the --apply/preview-by-default requirement for backfill-short-ids.
//  2. code side: backfillShortIDsCmd has no "dry-run" flag, carries
//     "timeout" (PRESERVED -- H8), and Deprecated is EXACTLY
//     "use: engram migrate".
//  3. C4-L3: the STALE "backfill-short-ids --dry-run" combination appears
//     ONLY inside "## Unreleased" (where it documents the removal) --
//     never in a historical section still instructing it.
//  4. C5-L1/C6-H1: "## Unreleased" never CLAIMS --timeout was removed
//     (it is preserved), matched as a clause-scoped removal claim, not a
//     bare word co-occurrence or a character-count window -- both of
//     which would trip on this plan's own permitted wording.
//
// All three doc-side assertions (1, 3, 4) are simultaneously satisfiable:
// (1) requires "--dry-run" INSIDE "## Unreleased"; (3) forbids the
// backfill-short-ids/--dry-run COMBINATION outside it (not a bare
// "--dry-run" search, which pre-existing unrelated commands' own valid
// --dry-run mentions would fail); (4) forbids a --timeout REMOVAL CLAIM
// inside it. No pair contradicts: --dry-run's removal is documented, the
// STALE instruction is gone, and --timeout is truthfully never claimed
// removed.
func TestUpgradeGuideReconcilesBackfill(t *testing.T) {
	data, err := os.ReadFile(upgradeGuideRelPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("upgrade guide not present at %s (trimmed checkout?) -- skipping rather than passing silently", upgradeGuideRelPath)
		}
		t.Fatalf("read %s: %v", upgradeGuideRelPath, err)
	}
	doc := string(data)
	section := extractUnreleasedSection(doc)
	if strings.TrimSpace(section) == "" {
		t.Fatalf("no non-empty %q section found in %s -- either the heading is missing or the section is empty", "## Unreleased", upgradeGuideRelPath)
	}

	t.Run("doc side: Unreleased names the --dry-run removal and preview-by-default", func(t *testing.T) {
		// A bare "section contains X" check per token is satisfiable by
		// THREE INDEPENDENT, SCATTERED mentions elsewhere in this long
		// section (e.g. item #2's operator-command list names
		// "backfill-short-ids", item #10 mentions "--dry-run" for a
		// DIFFERENT command, item #9 mentions "--apply" for a THIRD
		// command) without ever describing backfill-short-ids's OWN
		// removal -- confirmed empirically: this exact false-pass was
		// observed during this test's own RED-first authoring, reverted
		// once caught. The real property requires all three tokens to
		// appear TOGETHER in one coherent paragraph.
		found := false
		for _, block := range strings.Split(section, "\n\n") {
			if strings.Contains(block, "backfill-short-ids") &&
				strings.Contains(block, "--dry-run") &&
				strings.Contains(block, "--apply") {
				found = true
				break
			}
		}
		if !found {
			t.Error(`no single paragraph inside "## Unreleased" jointly names "backfill-short-ids", "--dry-run", and "--apply" -- scattered independent mentions do not document THIS command's removal`)
		}
	})

	t.Run("code side: no --dry-run, has --timeout (H8), Deprecated points at migrate", func(t *testing.T) {
		if backfillShortIDsCmd.Flags().Lookup("dry-run") != nil {
			t.Error(`backfillShortIDsCmd carries a "dry-run" flag, want none (D-09)`)
		}
		if backfillShortIDsCmd.Flags().Lookup("timeout") == nil {
			t.Error(`backfillShortIDsCmd has no "timeout" flag, want it PRESERVED (REVIEWS.md H8)`)
		}
		const wantDeprecated = "use: engram migrate"
		if backfillShortIDsCmd.Deprecated != wantDeprecated {
			t.Errorf("backfillShortIDsCmd.Deprecated = %q, want %q", backfillShortIDsCmd.Deprecated, wantDeprecated)
		}
	})

	t.Run("C4-L3: the stale backfill-short-ids/--dry-run combination survives ONLY inside Unreleased", func(t *testing.T) {
		remainder := docWithoutUnreleasedSection(doc)
		if loc := backfillShortIDsDryRunCombo.FindStringIndex(remainder); loc != nil {
			t.Errorf("a historical section (outside %q) still instructs backfill-short-ids with --dry-run, a combination the binary no longer accepts: %q",
				"## Unreleased", remainder[max(0, loc[0]-20):min(len(remainder), loc[1]+20)])
		}
	})

	t.Run("C5-L1/C6-H1: Unreleased never claims --timeout was removed", func(t *testing.T) {
		for _, line := range strings.Split(section, "\n") {
			if timeoutRemovalClaimPattern.MatchString(line) {
				t.Errorf("line claims --timeout was removed (it is PRESERVED -- H8): %q", line)
			}
		}
	})
}

// docWithoutUnreleasedSection returns doc with its "## Unreleased" section's
// BODY (everything between the heading and the next "## " heading, or EOF)
// removed -- reusing extractUnreleasedSection's own boundary logic
// (docsync_test.go) rather than a substring-Replace on the extracted
// section text, which would be ambiguous if the section's own text ever
// recurred elsewhere in the document.
func docWithoutUnreleasedSection(doc string) string {
	loc := unreleasedHeadingPattern.FindStringIndex(doc)
	if loc == nil {
		return doc
	}
	rest := doc[loc[1]:]
	if next := nextH2HeadingPattern.FindStringIndex(rest); next != nil {
		return doc[:loc[1]] + rest[next[0]:]
	}
	return doc[:loc[1]]
}
