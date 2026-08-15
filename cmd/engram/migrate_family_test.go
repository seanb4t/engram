// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/migrate"
	"github.com/seanb4t/engram/internal/store"
)

// fakeMigrateFamilyStore is a recording, injectable fake satisfying
// migrateFamilyStore (REVIEWS.md M7) -- no live Qdrant dial. Each method's
// func field is nil-safe (returns a zero value when unset), mirroring
// spineConsolidateFakeStore's precedent (spine_review_consolidate_test.go).
// migrateCalls records every store.MigrateOptions this fake's Migrate saw,
// in call order -- the evidence TestMigrateFamilyPreviewAndApply's H5
// double-call assertion reads. previewRevertCalls/revertCalls/callOrder are
// the same evidence for the revert family: callOrder records "preview"/
// "revert" tokens in the exact sequence the CLI invoked them, proving
// PreviewRevert always runs before any Revert call.
type fakeMigrateFamilyStore struct {
	migrateFn       func(ctx context.Context, opts store.MigrateOptions) (store.MigrateResult, error)
	migrateStatusFn func(ctx context.Context) (store.MigrateStatusResult, error)
	revertFn        func(ctx context.Context, to migrate.Version) (store.RevertResult, error)
	previewRevertFn func(ctx context.Context, to migrate.Version) (store.RevertPlan, error)

	migrateCalls       []store.MigrateOptions
	previewRevertCalls []migrate.Version
	revertCalls        []migrate.Version
	callOrder          []string
}

func (f *fakeMigrateFamilyStore) Migrate(ctx context.Context, opts store.MigrateOptions) (store.MigrateResult, error) {
	f.migrateCalls = append(f.migrateCalls, opts)
	if f.migrateFn == nil {
		return store.MigrateResult{}, nil
	}
	return f.migrateFn(ctx, opts)
}

func (f *fakeMigrateFamilyStore) MigrateStatus(ctx context.Context) (store.MigrateStatusResult, error) {
	if f.migrateStatusFn == nil {
		return store.MigrateStatusResult{}, nil
	}
	return f.migrateStatusFn(ctx)
}

func (f *fakeMigrateFamilyStore) Revert(ctx context.Context, to migrate.Version) (store.RevertResult, error) {
	f.revertCalls = append(f.revertCalls, to)
	f.callOrder = append(f.callOrder, "revert")
	if f.revertFn == nil {
		return store.RevertResult{}, nil
	}
	return f.revertFn(ctx, to)
}

func (f *fakeMigrateFamilyStore) PreviewRevert(ctx context.Context, to migrate.Version) (store.RevertPlan, error) {
	f.previewRevertCalls = append(f.previewRevertCalls, to)
	f.callOrder = append(f.callOrder, "preview")
	if f.previewRevertFn == nil {
		return store.RevertPlan{}, nil
	}
	return f.previewRevertFn(ctx, to)
}

// withFakeMigrateFamilyStore substitutes migrateFamilyStoreFromEnv with one
// that returns fake, restoring the real constructor via t.Cleanup --
// mirrors withFakeConsolidateStore (spine_review_consolidate_test.go:41-46).
func withFakeMigrateFamilyStore(t *testing.T, fake migrateFamilyStore) {
	t.Helper()
	orig := migrateFamilyStoreFromEnv
	migrateFamilyStoreFromEnv = func() (migrateFamilyStore, error) { return fake, nil }
	t.Cleanup(func() { migrateFamilyStoreFromEnv = orig })
}

// TestMigrateFamilyPreviewAndApply is the CLI-level proof that migrate
// previews by default (no writes) and --apply performs the in-apply-closure
// re-preview (REVIEWS.md H5): the fake store's Migrate is called TWICE
// during a single --apply invocation, first with DryRun:true, then with a
// non-nil Manifest. migrate status renders a distribution, and both leaves
// render through renderOperator's two modes.
func TestMigrateFamilyPreviewAndApply(t *testing.T) {
	t.Run("bare migrate previews only", func(t *testing.T) {
		fake := &fakeMigrateFamilyStore{
			migrateFn: func(_ context.Context, opts store.MigrateOptions) (store.MigrateResult, error) {
				if !opts.DryRun {
					t.Fatalf("preview call: opts.DryRun = false, want true")
				}
				return store.MigrateResult{PreviewManifest: map[string]migrate.Version{"a": 0, "b": 0}}, nil
			},
		}
		withFakeMigrateFamilyStore(t, fake)
		resetClientFlags(t)
		resetCommandFlagState(t, migrateCmd)

		stdout, _, err := runClient(t, "migrate", "--output", "json")
		if err != nil {
			t.Fatalf("migrate: %v", err)
		}
		var doc migrateOutputDoc
		if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal: %v (stdout=%q)", uErr, stdout)
		}
		if !doc.DryRun || doc.WouldMigrate != 2 || doc.Migrated != 0 {
			t.Errorf("doc = %+v, want DryRun=true WouldMigrate=2 Migrated=0", doc)
		}
		if len(fake.migrateCalls) != 1 {
			t.Fatalf("Migrate called %d time(s), want 1 (preview only, no write)", len(fake.migrateCalls))
		}
	})

	t.Run("migrate --apply calls Migrate twice: DryRun then Manifest", func(t *testing.T) {
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
		resetCommandFlagState(t, migrateCmd)

		stdout, _, err := runClient(t, "migrate", "--apply", "--output", "json")
		if err != nil {
			t.Fatalf("migrate --apply: %v", err)
		}
		var doc migrateOutputDoc
		if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal: %v (stdout=%q)", uErr, stdout)
		}
		if doc.DryRun {
			t.Errorf("doc.DryRun = true, want false")
		}
		if doc.Migrated != 1 {
			t.Errorf("doc.Migrated = %d, want 1", doc.Migrated)
		}
		if len(fake.migrateCalls) != 2 {
			t.Fatalf("Migrate called %d time(s) inside one --apply invocation, want 2 (H5 in-apply-closure re-preview)", len(fake.migrateCalls))
		}
		if !fake.migrateCalls[0].DryRun {
			t.Errorf("first call: DryRun = false, want true")
		}
		if fake.migrateCalls[1].DryRun {
			t.Errorf("second call: DryRun = true, want false")
		}
		if fake.migrateCalls[1].Manifest == nil {
			t.Errorf("second call: Manifest is nil, want non-nil")
		}
	})

	t.Run("migrate status renders a distribution", func(t *testing.T) {
		fake := &fakeMigrateFamilyStore{
			migrateStatusFn: func(context.Context) (store.MigrateStatusResult, error) {
				return store.MigrateStatusResult{
					Buckets: []store.VersionBucket{{Version: 1, Count: 10}}, Absent: 2,
					Future: []store.VersionBucket{{Version: 2, Count: 1}}, FutureTotal: 1, Total: 13,
				}, nil
			},
		}
		withFakeMigrateFamilyStore(t, fake)
		resetClientFlags(t)
		resetCommandFlagState(t, migrateStatusCmd)

		stdout, _, err := runClient(t, "migrate", "status", "--output", "json")
		if err != nil {
			t.Fatalf("migrate status: %v", err)
		}
		var res store.MigrateStatusResult
		if uErr := json.Unmarshal([]byte(stdout), &res); uErr != nil {
			t.Fatalf("json.Unmarshal: %v (stdout=%q)", uErr, stdout)
		}
		if res.Total != 13 || res.Absent != 2 || res.FutureTotal != 1 {
			t.Errorf("res = %+v, want Total=13 Absent=2 FutureTotal=1", res)
		}
	})

	t.Run("text mode renders through renderOperator too", func(t *testing.T) {
		fake := &fakeMigrateFamilyStore{
			migrateFn: func(_ context.Context, _ store.MigrateOptions) (store.MigrateResult, error) {
				return store.MigrateResult{PreviewManifest: map[string]migrate.Version{}}, nil
			},
		}
		withFakeMigrateFamilyStore(t, fake)
		resetClientFlags(t)
		resetCommandFlagState(t, migrateCmd)

		stdout, _, err := runClient(t, "migrate", "--output", "text")
		if err != nil {
			t.Fatalf("migrate --output text: %v", err)
		}
		if !strings.Contains(stdout, "preview") {
			t.Errorf("stdout = %q, want it to mention the preview", stdout)
		}
	})
}

// TestMigrateFamilyApplyIntersection is the CLI-boundary preview/apply
// parity proof (REVIEWS.md H6/SC3): the manifest handed to the SECOND
// Migrate call must equal, by identity, the exact manifest the FIRST
// (DryRun) call returned -- never a re-derived or partial set -- and the
// rendered doc reports Migrated/Spared/Appeared as an identity-set
// intersection, never a count comparison.
func TestMigrateFamilyApplyIntersection(t *testing.T) {
	previewManifest := map[string]migrate.Version{"still-eligible": 0, "no-longer-eligible": 0}
	fake := &fakeMigrateFamilyStore{
		migrateFn: func(_ context.Context, opts store.MigrateOptions) (store.MigrateResult, error) {
			if opts.DryRun {
				return store.MigrateResult{PreviewManifest: previewManifest}, nil
			}
			if !reflect.DeepEqual(opts.Manifest, previewManifest) {
				t.Fatalf("apply call: Manifest = %v, want the exact manifest the preview call returned (%v) -- H6/SC3 identity-set intersection", opts.Manifest, previewManifest)
			}
			return store.MigrateResult{
				Migrated: 1,
				Spared:   []string{"no-longer-eligible"},
				Appeared: []string{"newly-eligible"},
			}, nil
		},
	}
	withFakeMigrateFamilyStore(t, fake)
	resetClientFlags(t)
	resetCommandFlagState(t, migrateCmd)

	stdout, _, err := runClient(t, "migrate", "--apply", "--output", "json")
	if err != nil {
		t.Fatalf("migrate --apply: %v", err)
	}
	var doc migrateOutputDoc
	if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
		t.Fatalf("json.Unmarshal: %v (stdout=%q)", uErr, stdout)
	}
	if doc.Migrated != 1 {
		t.Errorf("doc.Migrated = %d, want 1", doc.Migrated)
	}
	if doc.Spared != 1 {
		t.Errorf("doc.Spared = %d, want 1", doc.Spared)
	}
	if doc.Appeared != 1 {
		t.Errorf("doc.Appeared = %d, want 1", doc.Appeared)
	}
	if doc.WouldMigrate != uint64(len(previewManifest)) {
		t.Errorf("doc.WouldMigrate = %d, want %d (the manifest length the apply acted from)", doc.WouldMigrate, len(previewManifest))
	}
}

// TestMigrateFamilyTimeoutWiring is the three-case behavioural proof the
// --timeout flag is actually READ, not merely registered (REVIEWS.md H8 +
// N3): a hard-coded 5-minute helper passes the default case but fails the
// 1s case, which is the write-the-cheapest-passing-violator-first check
// this test is built to catch. Covers both migrate and migrate status
// (REVIEWS.md C5-M6).
func TestMigrateFamilyTimeoutWiring(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		cmd      *cobra.Command
		wantNear time.Duration // if > 0, deadline must be <= this
		wantFar  time.Duration // if > 0, deadline must be > this
		wantNone bool          // if true, ctx.Deadline() must report ok=false
	}{
		{name: "migrate --timeout 1s", args: []string{"migrate", "--timeout", "1s", "--output", "json"}, cmd: migrateCmd, wantNear: 2 * time.Second},
		{name: "migrate default timeout", args: []string{"migrate", "--output", "json"}, cmd: migrateCmd, wantFar: 4 * time.Minute},
		{name: "migrate --timeout 0", args: []string{"migrate", "--timeout", "0", "--output", "json"}, cmd: migrateCmd, wantNone: true},
		{name: "migrate status --timeout 1s", args: []string{"migrate", "status", "--timeout", "1s", "--output", "json"}, cmd: migrateStatusCmd, wantNear: 2 * time.Second},
		{name: "migrate status default timeout", args: []string{"migrate", "status", "--output", "json"}, cmd: migrateStatusCmd, wantFar: 4 * time.Minute},
		{name: "migrate status --timeout 0", args: []string{"migrate", "status", "--timeout", "0", "--output", "json"}, cmd: migrateStatusCmd, wantNone: true},
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
				migrateStatusFn: func(ctx context.Context) (store.MigrateStatusResult, error) {
					gotDeadline, gotOK = ctx.Deadline()
					return store.MigrateStatusResult{}, nil
				},
			}
			withFakeMigrateFamilyStore(t, fake)
			resetClientFlags(t)
			resetCommandFlagState(t, tc.cmd)

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

// TestMigrateFamilyReportFields pins WHICH field carries the projection
// count (REVIEWS.md N4): would_migrate is len(PreviewManifest), never
// inferred, on both the bare preview and the --apply path, where it equals
// the manifest length the apply actually acted from.
func TestMigrateFamilyReportFields(t *testing.T) {
	sevenEntryManifest := make(map[string]migrate.Version, 7)
	for i := range 7 {
		sevenEntryManifest[fmt.Sprintf("id-%d", i)] = 0
	}

	t.Run("bare migrate: would_migrate=7 migrated=0", func(t *testing.T) {
		fake := &fakeMigrateFamilyStore{
			migrateFn: func(_ context.Context, opts store.MigrateOptions) (store.MigrateResult, error) {
				if !opts.DryRun {
					t.Fatal("expected a DryRun call")
				}
				return store.MigrateResult{PreviewManifest: sevenEntryManifest}, nil
			},
		}
		withFakeMigrateFamilyStore(t, fake)
		resetClientFlags(t)
		resetCommandFlagState(t, migrateCmd)

		stdout, _, err := runClient(t, "migrate", "--output", "json")
		if err != nil {
			t.Fatalf("migrate: %v", err)
		}
		var doc migrateOutputDoc
		if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal: %v", uErr)
		}
		if doc.WouldMigrate != 7 {
			t.Errorf("doc.WouldMigrate = %d, want 7", doc.WouldMigrate)
		}
		if doc.Migrated != 0 {
			t.Errorf("doc.Migrated = %d, want 0", doc.Migrated)
		}
	})

	t.Run("migrate --apply: would_migrate equals the manifest length the apply acted from", func(t *testing.T) {
		fake := &fakeMigrateFamilyStore{
			migrateFn: func(_ context.Context, opts store.MigrateOptions) (store.MigrateResult, error) {
				if opts.DryRun {
					return store.MigrateResult{PreviewManifest: sevenEntryManifest}, nil
				}
				return store.MigrateResult{Migrated: uint64(len(opts.Manifest))}, nil
			},
		}
		withFakeMigrateFamilyStore(t, fake)
		resetClientFlags(t)
		resetCommandFlagState(t, migrateCmd)

		stdout, _, err := runClient(t, "migrate", "--apply", "--output", "json")
		if err != nil {
			t.Fatalf("migrate --apply: %v", err)
		}
		var doc migrateOutputDoc
		if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal: %v", uErr)
		}
		if doc.WouldMigrate != 7 {
			t.Errorf("doc.WouldMigrate = %d, want 7", doc.WouldMigrate)
		}
		if doc.Migrated != 7 {
			t.Errorf("doc.Migrated = %d, want 7", doc.Migrated)
		}
	})
}

// TestMigrateFamilyStatusReportDocNeverMarshalsNull is the named direct
// gate for REVIEWS.md C5-L8 (this exact name, INV-3 — TestOperatorOutputEmpty
// gains no "migrate status" entry in this phase, C6-L4): marshalling
// statusReportDoc over a ZERO-valued store.MigrateStatusResult must never
// emit "buckets":null or "future":null. A doc built by plain struct-literal
// assignment from a zero result (skipping the nil-to-empty-slice
// normalization) would emit both and go RED here.
func TestMigrateFamilyStatusReportDocNeverMarshalsNull(t *testing.T) {
	doc := statusReportDoc(store.MigrateStatusResult{})
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(b), `"buckets":null`) {
		t.Errorf("marshalled doc contains \"buckets\":null: %s", b)
	}
	if strings.Contains(string(b), `"future":null`) {
		t.Errorf("marshalled doc contains \"future\":null: %s", b)
	}
}

// TestMigrateFamilyRevertRefusals proves both refusal classes (D-13/D-14):
// an irreversible-step range and an unsupported-version range each render
// their own field=/hint= envelope, Revert is called ZERO times on EITHER
// invocation shape, and the exit-code pair is asserted both directions
// (REVIEWS.md C5-H4): bare exits 0 (the preview correctly reported the
// refusal), --apply exits exitUsage (2). The refusal-text identity
// assertion (C5-M4) proves the CLI's rendered refusal is the EXACT
// store.RevertRefusalError(plan).Error() string, never a re-typed
// transcription.
func TestMigrateFamilyRevertRefusals(t *testing.T) {
	cases := []struct {
		name string
		plan store.RevertPlan
	}{
		{
			name: "irreversible range",
			plan: store.RevertPlan{
				To: 0, Candidates: 1, Reversible: false,
				Irreversible: []store.IrreversibleStepRef{{From: 0, To: 1, Reason: "no declared inverse"}},
			},
		},
		{
			name: "unsupported-version range",
			plan: store.RevertPlan{
				To: 0, Candidates: 3, Reversible: false,
				Unsupported: []store.UnsupportedVersionRef{{Version: 42, Count: 3}},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			wantRefusal := store.RevertRefusalError(tc.plan).Error()

			t.Run("bare invocation exits 0, renders the refusal, never calls Revert", func(t *testing.T) {
				fake := &fakeMigrateFamilyStore{
					previewRevertFn: func(context.Context, migrate.Version) (store.RevertPlan, error) { return tc.plan, nil },
				}
				withFakeMigrateFamilyStore(t, fake)
				resetClientFlags(t)
				resetCommandFlagState(t, migrateRevertCmd)

				stdout, _, err := runClient(t, "migrate", "revert", "--to", "0", "--output", "json")
				if got := exitCodeFromError(err); got != exitOK {
					t.Errorf("exitCodeFromError = %d, want %d (exitOK); err=%v", got, exitOK, err)
				}
				var doc revertOutputDoc
				if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
					t.Fatalf("json.Unmarshal: %v (stdout=%q)", uErr, stdout)
				}
				if doc.Refusal != wantRefusal {
					t.Errorf("doc.Refusal = %q, want the EXACT store.RevertRefusalError(plan).Error() string %q", doc.Refusal, wantRefusal)
				}
				if len(fake.revertCalls) != 0 {
					t.Errorf("Revert called %d time(s), want 0", len(fake.revertCalls))
				}
			})

			t.Run("--apply exits exitUsage, renders the refusal, never calls Revert", func(t *testing.T) {
				fake := &fakeMigrateFamilyStore{
					previewRevertFn: func(context.Context, migrate.Version) (store.RevertPlan, error) { return tc.plan, nil },
				}
				withFakeMigrateFamilyStore(t, fake)
				resetClientFlags(t)
				resetCommandFlagState(t, migrateRevertCmd)

				stdout, _, err := runClient(t, "migrate", "revert", "--to", "0", "--apply", "--output", "json")
				if got := exitCodeFromError(err); got != exitUsage {
					t.Errorf("exitCodeFromError = %d, want %d (exitUsage); err=%v", got, exitUsage, err)
				}
				var doc revertOutputDoc
				if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
					t.Fatalf("json.Unmarshal: %v (stdout=%q)", uErr, stdout)
				}
				if doc.Refusal != wantRefusal {
					t.Errorf("doc.Refusal = %q, want the EXACT store.RevertRefusalError(plan).Error() string %q", doc.Refusal, wantRefusal)
				}
				if len(fake.revertCalls) != 0 {
					t.Errorf("Revert called %d time(s), want 0 -- a refused --apply must touch zero records", len(fake.revertCalls))
				}
			})
		})
	}
}

// TestMigrateFamilyRevertApplySecondPreflightRefusal proves the CR-01 gap
// (REVIEWS.md deep-pass): when call A's own PreviewRevert reports
// Reversible: true but Store.Revert's SEPARATE internal preflight (call B)
// refuses -- the exact race window CR-01 and WR-03 both concern -- Revert
// returns *store.RevertRefusedError, and revertApplyRun must recognize it
// via errors.As, render the SAME refusal document contract every other
// refusal path renders, and exit exitUsage (2), never falling through to
// classifyOperatorErr's generic exit-1 passthrough.
func TestMigrateFamilyRevertApplySecondPreflightRefusal(t *testing.T) {
	previewPlan := store.RevertPlan{To: 0, Candidates: 5, Reversible: true}
	refusedPlan := store.RevertPlan{
		To: 0, Candidates: 6, Reversible: false,
		Irreversible: []store.IrreversibleStepRef{{From: 0, To: 1, Reason: "no declared inverse"}},
	}
	wantRefusal := store.RevertRefusalError(refusedPlan).Error()

	fake := &fakeMigrateFamilyStore{
		previewRevertFn: func(context.Context, migrate.Version) (store.RevertPlan, error) { return previewPlan, nil },
		revertFn: func(context.Context, migrate.Version) (store.RevertResult, error) {
			return store.RevertResult{Plan: refusedPlan}, &store.RevertRefusedError{Plan: refusedPlan}
		},
	}
	withFakeMigrateFamilyStore(t, fake)
	resetClientFlags(t)
	resetCommandFlagState(t, migrateRevertCmd)

	stdout, _, err := runClient(t, "migrate", "revert", "--to", "0", "--apply", "--output", "json")
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError = %d, want %d (exitUsage); err=%v", got, exitUsage, err)
	}
	var doc revertOutputDoc
	if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
		t.Fatalf("json.Unmarshal: %v (stdout=%q)", uErr, stdout)
	}
	if doc.Refusal == "" {
		t.Errorf("doc.Refusal is empty, want the rendered refusal document -- CR-01: a second-preflight refusal must render, not fall through as a generic error")
	}
	if doc.Refusal != wantRefusal {
		t.Errorf("doc.Refusal = %q, want the refusedPlan's refusal text %q (call B's fresher plan, not call A's stale one)", doc.Refusal, wantRefusal)
	}
	if doc.Candidates != refusedPlan.Candidates {
		t.Errorf("doc.Candidates = %d, want %d -- must render from refused.Plan (call B), not the stale call-A preview plan (Candidates=%d)",
			doc.Candidates, refusedPlan.Candidates, previewPlan.Candidates)
	}
}

// TestMigrateFamilyRevertReversible proves the REVERSIBLE path (M8): a bare
// invocation renders the reverse-plan preview and calls Revert zero times;
// --apply calls the SAME exported PreviewRevert method again, THEN Revert,
// exactly once each, in that order, and renders the converged result.
func TestMigrateFamilyRevertReversible(t *testing.T) {
	plan := store.RevertPlan{To: 0, Candidates: 5, Reversible: true}

	t.Run("bare invocation previews, calls Revert zero times", func(t *testing.T) {
		fake := &fakeMigrateFamilyStore{
			previewRevertFn: func(context.Context, migrate.Version) (store.RevertPlan, error) { return plan, nil },
		}
		withFakeMigrateFamilyStore(t, fake)
		resetClientFlags(t)
		resetCommandFlagState(t, migrateRevertCmd)

		stdout, _, err := runClient(t, "migrate", "revert", "--to", "0", "--output", "json")
		if err != nil {
			t.Fatalf("bare revert: %v", err)
		}
		var doc revertOutputDoc
		if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal: %v (stdout=%q)", uErr, stdout)
		}
		if doc.Applied || !doc.Reversible || doc.Candidates != 5 {
			t.Errorf("doc = %+v, want Applied=false Reversible=true Candidates=5", doc)
		}
		if len(fake.revertCalls) != 0 {
			t.Errorf("Revert called %d time(s), want 0 (preview only)", len(fake.revertCalls))
		}
	})

	t.Run("--apply calls PreviewRevert then Revert exactly once each, in order", func(t *testing.T) {
		fake := &fakeMigrateFamilyStore{
			previewRevertFn: func(context.Context, migrate.Version) (store.RevertPlan, error) { return plan, nil },
			revertFn: func(context.Context, migrate.Version) (store.RevertResult, error) {
				// Plan mirrors the real Store.Revert contract (revert.go:329,
				// "res.Plan = plan" runs unconditionally before the
				// reversible check) -- revertApplyRun now renders from
				// res.Plan, not the stale outer preview plan (WR-03).
				return store.RevertResult{Reverted: 5, Backlog: 0, Plan: plan}, nil
			},
		}
		withFakeMigrateFamilyStore(t, fake)
		resetClientFlags(t)
		resetCommandFlagState(t, migrateRevertCmd)

		stdout, _, err := runClient(t, "migrate", "revert", "--to", "0", "--apply", "--output", "json")
		if err != nil {
			t.Fatalf("migrate revert --apply: %v", err)
		}
		var doc revertOutputDoc
		if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal: %v (stdout=%q)", uErr, stdout)
		}
		if !doc.Applied || doc.Reverted != 5 {
			t.Errorf("doc = %+v, want Applied=true Reverted=5", doc)
		}
		if len(fake.previewRevertCalls) != 1 {
			t.Errorf("PreviewRevert called %d time(s), want 1", len(fake.previewRevertCalls))
		}
		if len(fake.revertCalls) != 1 {
			t.Errorf("Revert called %d time(s), want 1", len(fake.revertCalls))
		}
		wantOrder := []string{"preview", "revert"}
		if !reflect.DeepEqual(fake.callOrder, wantOrder) {
			t.Errorf("call order = %v, want %v", fake.callOrder, wantOrder)
		}
	})
}

// TestMigrateFamilyRevertToValidation proves --to's usage-error surface
// (D-16, spine_review_purge.go:65 precedent): a non-integer value is
// rejected by cobra's own flag parsing (a framework flag error, exit 2), a
// negative value and a missing --to are each rejected by
// migrateRevertValidateTo, also exit 2 -- never an unregistered panic, and
// never a store dial.
func TestMigrateFamilyRevertToValidation(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "missing --to", args: []string{"migrate", "revert"}},
		{name: "negative --to", args: []string{"migrate", "revert", "--to", "-1"}},
		{name: "non-integer --to", args: []string{"migrate", "revert", "--to", "abc"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeMigrateFamilyStore{}
			withFakeMigrateFamilyStore(t, fake)
			resetClientFlags(t)
			resetCommandFlagState(t, migrateRevertCmd)

			_, _, err := runClient(t, tc.args...)
			if got := exitCodeFromError(err); got != exitUsage {
				t.Errorf("exitCodeFromError = %d, want %d (exitUsage); err=%v", got, exitUsage, err)
			}
			if len(fake.previewRevertCalls) != 0 {
				t.Errorf("PreviewRevert called %d time(s), want 0 -- --to validation must precede any dial", len(fake.previewRevertCalls))
			}
		})
	}
}

// TestMigrateFamilyRevertTimeoutWiring is the three-case behavioural proof
// for migrate revert's own --timeout (REVIEWS.md H8 + N3), mirroring
// TestMigrateFamilyTimeoutWiring's cases for migrate/migrate status.
func TestMigrateFamilyRevertTimeoutWiring(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantNear time.Duration
		wantFar  time.Duration
		wantNone bool
	}{
		{name: "--timeout 1s", args: []string{"migrate", "revert", "--to", "0", "--timeout", "1s", "--output", "json"}, wantNear: 2 * time.Second},
		{name: "default timeout", args: []string{"migrate", "revert", "--to", "0", "--output", "json"}, wantFar: 4 * time.Minute},
		{name: "--timeout 0", args: []string{"migrate", "revert", "--to", "0", "--timeout", "0", "--output", "json"}, wantNone: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var gotDeadline time.Time
			var gotOK bool
			fake := &fakeMigrateFamilyStore{
				previewRevertFn: func(ctx context.Context, _ migrate.Version) (store.RevertPlan, error) {
					gotDeadline, gotOK = ctx.Deadline()
					return store.RevertPlan{To: 0, Reversible: true}, nil
				},
			}
			withFakeMigrateFamilyStore(t, fake)
			resetClientFlags(t)
			resetCommandFlagState(t, migrateRevertCmd)

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
					t.Errorf("time.Until(deadline) = %v, want > %v (the ~5m default is applied)", d, tc.wantFar)
				}
			}
		})
	}
}
