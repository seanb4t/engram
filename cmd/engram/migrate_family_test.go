// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
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

// TestMigrateFamilyStatusReportDocKeyOrder pins the marshaled key order for
// a populated migrateStatusReportDoc (Task 2's acceptance criterion): the
// five pre-existing keys stay in their pre-existing order, and
// current_version — the R2 gap-closure addition — lands last.
func TestMigrateFamilyStatusReportDocKeyOrder(t *testing.T) {
	doc := statusReportDoc(store.MigrateStatusResult{
		Buckets: []store.VersionBucket{{Version: 1, Count: 40}}, Absent: 3,
		Future: []store.VersionBucket{{Version: 2, Count: 1}}, FutureTotal: 1, Total: 44,
	})
	keys, err := jsonTopLevelKeys(doc)
	if err != nil {
		t.Fatalf("jsonTopLevelKeys: %v", err)
	}
	want := []string{"buckets", "absent", "future", "future_total", "total", "current_version", "pending"}
	for _, e := range orderedKeyDiff(want, keys) {
		t.Error(e)
	}
}

// TestMigrateFamilyStatusReportDocPendingNeverRederived is the 09-01
// discriminating-fixture gate for D-02/D-05: it proves statusReportDoc's
// Pending field comes from the single store.MigrateStatusResult.Pending()
// definition rather than any of the plausible-looking naive rederivations
// a copy-paste could introduce. The fixture is built cur-relative
// (migrate.CurrentVersion, never a literal version number) so it keeps
// discriminating after CurrentVersion advances: Absent 88, one bucket
// below current (cur-1, count 9), one bucket at current (cur, count 40),
// one future bucket (cur+1, count 5), FutureTotal 5, Total 142 — chosen so
// Pending() (97), the naive absent-plus-every-bucket sum (137), that plus
// FutureTotal (142), and Total-minus-current-bucket (102) are four
// pairwise-distinct values.
func TestMigrateFamilyStatusReportDocPendingNeverRederived(t *testing.T) {
	cur := int(migrate.CurrentVersion)
	res := store.MigrateStatusResult{
		Absent: 88,
		Buckets: []store.VersionBucket{
			{Version: cur - 1, Count: 9},
			{Version: cur, Count: 40},
		},
		Future:      []store.VersionBucket{{Version: cur + 1, Count: 5}},
		FutureTotal: 5,
		Total:       142,
	}

	t.Run("equals_store_pending", func(t *testing.T) {
		doc := statusReportDoc(res)
		if doc.Pending != res.Pending() {
			t.Errorf("doc.Pending = %d, want res.Pending() = %d", doc.Pending, res.Pending())
		}
		if doc.Pending != 97 {
			t.Errorf("doc.Pending = %d, want 97", doc.Pending)
		}
	})

	t.Run("fixture_discriminates_every_naive_rederivation", func(t *testing.T) {
		want := res.Pending()
		var absentPlusEveryBucket uint64 = res.Absent
		for _, b := range res.Buckets {
			absentPlusEveryBucket += b.Count
		}
		plusFutureTotal := absentPlusEveryBucket + res.FutureTotal
		var atCurrentCount uint64
		for _, b := range res.Buckets {
			if b.Version == cur {
				atCurrentCount = b.Count
			}
		}
		totalMinusCurrentBucket := res.Total - atCurrentCount

		if absentPlusEveryBucket != 137 {
			t.Fatalf("absentPlusEveryBucket = %d, want 137 (fixture arithmetic changed)", absentPlusEveryBucket)
		}
		if plusFutureTotal != 142 {
			t.Fatalf("plusFutureTotal = %d, want 142 (fixture arithmetic changed)", plusFutureTotal)
		}
		if totalMinusCurrentBucket != 102 {
			t.Fatalf("totalMinusCurrentBucket = %d, want 102 (fixture arithmetic changed)", totalMinusCurrentBucket)
		}

		values := map[string]uint64{
			"want":                    want,
			"absentPlusEveryBucket":   absentPlusEveryBucket,
			"plusFutureTotal":         plusFutureTotal,
			"totalMinusCurrentBucket": totalMinusCurrentBucket,
		}
		seen := map[uint64]string{}
		for name, v := range values {
			if other, ok := seen[v]; ok {
				t.Errorf("%s and %s both equal %d — fixture no longer discriminates naive rederivations", name, other, v)
			}
			seen[v] = name
		}
	})

	t.Run("zero_value_marshals_pending_zero", func(t *testing.T) {
		b, err := json.Marshal(statusReportDoc(store.MigrateStatusResult{}))
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if !strings.Contains(string(b), `"pending":0`) {
			t.Errorf("marshalled doc does not contain \"pending\":0: %s", b)
		}
	})

	t.Run("converter_is_pure", func(t *testing.T) {
		inputCopy := store.MigrateStatusResult{
			Absent: 88,
			Buckets: []store.VersionBucket{
				{Version: cur - 1, Count: 9},
				{Version: cur, Count: 40},
			},
			Future:      []store.VersionBucket{{Version: cur + 1, Count: 5}},
			FutureTotal: 5,
			Total:       142,
		}
		first := statusReportDoc(res)
		second := statusReportDoc(res)
		if !reflect.DeepEqual(first, second) {
			t.Errorf("statusReportDoc is not idempotent: first=%+v second=%+v", first, second)
		}
		if !reflect.DeepEqual(res, inputCopy) {
			t.Errorf("statusReportDoc mutated its input: got=%+v want=%+v", res, inputCopy)
		}
	})

	t.Run("renders_last_in_both_lanes", func(t *testing.T) {
		doc := statusReportDoc(res)
		fields, err := viewFields(doc)
		if err != nil {
			t.Fatalf("viewFields: %v", err)
		}
		if len(fields) == 0 {
			t.Fatal("viewFields returned no fields")
		}
		last := fields[len(fields)-1]
		if last.Key != "pending" {
			t.Errorf("last field Key = %q, want %q", last.Key, "pending")
		}
		if last.Label != "Pending" {
			t.Errorf("last field Label = %q, want %q", last.Label, "Pending")
		}

		var textBuf bytes.Buffer
		if err := renderOperatorView(&textBuf, statusSummary(res), doc); err != nil {
			t.Fatalf("renderOperatorView: %v", err)
		}
		if !strings.Contains(textBuf.String(), "Pending") || !strings.Contains(textBuf.String(), "97") {
			t.Errorf("rendered text view missing Pending/97: %s", textBuf.String())
		}

		var jsonBuf bytes.Buffer
		if err := json.NewEncoder(&jsonBuf).Encode(doc); err != nil {
			t.Fatalf("json.NewEncoder.Encode: %v", err)
		}
		if !strings.Contains(jsonBuf.String(), `"pending":97`) {
			t.Errorf("rendered json missing pending:97: %s", jsonBuf.String())
		}
	})
}

// TestMigrateFamilyStatusSummaryPendingClause is the 09-01 gate for D-03/
// D-04: statusSummary's pending clause is asserted on VALUES and POSITION,
// never on the sentence's exact wording, because the sentence carries no
// stability contract (unlike --output json, D-03). Reuses Task 1's
// cur-relative fixture (Pending() 97).
func TestMigrateFamilyStatusSummaryPendingClause(t *testing.T) {
	cur := int(migrate.CurrentVersion)
	res := store.MigrateStatusResult{
		Absent: 88,
		Buckets: []store.VersionBucket{
			{Version: cur - 1, Count: 9},
			{Version: cur, Count: 40},
		},
		Future:      []store.VersionBucket{{Version: cur + 1, Count: 5}},
		FutureTotal: 5,
		Total:       142,
	}

	t.Run("ordering_tokens_are_unambiguous", func(t *testing.T) {
		headline := statusSummary(res)
		for _, tok := range []string{"40", "97", "5"} {
			if n := strings.Count(headline, tok); n != 1 {
				t.Errorf("strings.Count(headline, %q) = %d, want 1 (migrate.CurrentVersion may have changed — re-pick fixture counts, do not weaken this assertion): %s", tok, n, headline)
			}
		}
	})

	t.Run("states_pending_value_between_buckets_and_future", func(t *testing.T) {
		headline := statusSummary(res)
		iBucket := strings.Index(headline, "40")
		iPending := strings.Index(headline, "97")
		iFuture := strings.Index(headline, "5")
		if iBucket < 0 || iPending < 0 || iFuture < 0 {
			t.Fatalf("expected ordering tokens 40, 97, 5 all present: %s", headline)
		}
		if !(iBucket < iPending && iPending < iFuture) {
			t.Errorf("expected order bucket(40) < pending(97) < future(5) by index, got %d, %d, %d in: %s", iBucket, iPending, iFuture, headline)
		}
	})

	t.Run("emitted_unconditionally_at_zero", func(t *testing.T) {
		headline := strings.ToLower(statusSummary(store.MigrateStatusResult{}))
		if !strings.Contains(headline, "pending") {
			t.Errorf("zero-valued statusSummary missing pending clause: %s", headline)
		}
	})
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

// TestMigrateFamilyRevertApplyRefusalReportsPartialProgress pins deep-pass
// CR-06: a mid-loop refusal (the WR-05 sites inside Store.Revert's write
// loop) can fire AFTER whole batches already reverted, and Store.Revert
// returns those accumulated counters alongside the typed refusal. Rendering
// a zero-value RevertResult there reports `reverted: 0` for a run that
// really did mutate the collection, telling an operator there is nothing to
// reconcile when there is. The refusal envelope itself must be unchanged —
// only the progress counters are added.
func TestMigrateFamilyRevertApplyRefusalReportsPartialProgress(t *testing.T) {
	previewPlan := store.RevertPlan{To: 0, Candidates: 300, Reversible: true}
	// The single-record plan the WR-05 mid-loop sites synthesize.
	refusedPlan := store.RevertPlan{
		To: 0, Candidates: 1, Reversible: false,
		Irreversible: []store.IrreversibleStepRef{{From: 0, To: 1, Reason: "no declared inverse"}},
	}
	// Progress that already landed before the racing record tripped refusal.
	progressed := store.RevertResult{Reverted: 256, Failed: 2, Passes: 1, Backlog: 44, Plan: refusedPlan}

	fake := &fakeMigrateFamilyStore{
		previewRevertFn: func(context.Context, migrate.Version) (store.RevertPlan, error) { return previewPlan, nil },
		revertFn: func(context.Context, migrate.Version) (store.RevertResult, error) {
			return progressed, &store.RevertRefusedError{Plan: refusedPlan}
		},
	}
	withFakeMigrateFamilyStore(t, fake)
	resetClientFlags(t)
	resetCommandFlagState(t, migrateRevertCmd)

	stdout, _, err := runClient(t, "migrate", "revert", "--to", "0", "--apply", "--output", "json")
	if got := exitCodeFromError(err); got != exitUsage {
		t.Fatalf("exitCodeFromError = %d, want %d (exitUsage)", got, exitUsage)
	}
	var doc revertOutputDoc
	if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
		t.Fatalf("json.Unmarshal: %v (stdout=%q)", uErr, stdout)
	}
	if doc.Reverted != progressed.Reverted {
		t.Errorf("doc.Reverted = %d, want %d — CR-06: a refusal that followed real writes must report them, not zero", doc.Reverted, progressed.Reverted)
	}
	if doc.Failed != progressed.Failed {
		t.Errorf("doc.Failed = %d, want %d (CR-06)", doc.Failed, progressed.Failed)
	}
	if doc.Backlog != progressed.Backlog {
		t.Errorf("doc.Backlog = %d, want %d (CR-06)", doc.Backlog, progressed.Backlog)
	}
	// The envelope must be untouched by the CR-06 fix.
	if want := store.RevertRefusalError(refusedPlan).Error(); doc.Refusal != want {
		t.Errorf("doc.Refusal = %q, want %q — CR-06's counters must not alter the refusal envelope", doc.Refusal, want)
	}
	if doc.Applied {
		t.Errorf("doc.Applied = true, want false — the operation WAS refused; counters say how far it got")
	}

	// Text mode carries the same truth: an operator reading the terminal, not
	// the JSON, must still learn that records changed before the refusal.
	resetClientFlags(t)
	resetCommandFlagState(t, migrateRevertCmd)
	textOut, _, tErr := runClient(t, "migrate", "revert", "--to", "0", "--apply", "--output", "text")
	if got := exitCodeFromError(tErr); got != exitUsage {
		t.Fatalf("text mode exitCodeFromError = %d, want %d", got, exitUsage)
	}
	if !strings.Contains(textOut, "already reverted 256 record(s)") {
		t.Errorf("text summary = %q, want it to disclose the 256 records reverted before the refusal (CR-06)", textOut)
	}
}
