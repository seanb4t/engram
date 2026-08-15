// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/migrate"
	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/store"
)

// migrateFamilyStore is the minimal store surface the migrate command
// family's RunE bodies depend on (REVIEWS.md M7 — mirrors
// spineConsolidateStore's injection pattern): satisfied by *store.Store,
// and by a recording fake in migrate_family_test.go, so the flag-to-options
// mapping and the H5 in-apply-closure re-preview are provable without
// dialing a live Qdrant. PreviewRevert is included even though no RunE in
// this file calls it until Task 3 — REVIEWS.md M8, the exported preflight
// accessor 04-02 produced (internal/store/revert.go), never an unexported
// internal/store helper called from the CLI.
type migrateFamilyStore interface {
	Migrate(ctx context.Context, opts store.MigrateOptions) (store.MigrateResult, error)
	MigrateStatus(ctx context.Context) (store.MigrateStatusResult, error)
	Revert(ctx context.Context, to migrate.Version) (store.RevertResult, error)
	PreviewRevert(ctx context.Context, to migrate.Version) (store.RevertPlan, error)
}

// migrateFamilyStoreFromEnv constructs the store every migrate-family RunE
// dials through. A package-level var — mirroring spineConsolidateStoreFromEnv's
// injection pattern (spine_review_consolidate.go) — so migrate_family_test.go
// can substitute a recording fake without dialing a live Qdrant. Returns a
// nil interface (not a non-nil interface wrapping a nil *store.Store) on
// error.
var migrateFamilyStoreFromEnv = func() (migrateFamilyStore, error) {
	st, err := server.StoreFromEnv()
	if err != nil {
		return nil, err
	}
	return st, nil
}

// migrateWithTimeout mirrors spinePurgeWithTimeout (spine_review_purge.go:107)
// and pruneWithTimeout (prune.go:44): 0 disables the deadline. It TAKES d as
// a parameter, rather than reading a single package-level var, because
// THREE leaves share it — migrate, migrate revert, and (04-04) the
// backfill-short-ids alias — each passing its own flag-backed duration
// (REVIEWS.md H8 + N3). A zero-arg migrateWithTimeout(ctx) that ignored the
// caller's flag would be a compile error against this signature, and the
// three-case behavioural test in migrate_family_test.go pins that the
// duration is actually READ, not merely registered.
func migrateWithTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d > 0 {
		return context.WithTimeout(ctx, d)
	}
	return ctx, func() {}
}

var (
	migrateSweepOutput  string
	migrateSweepTimeout time.Duration
	migrateApply        bool

	migrateStatusOutput  string
	migrateStatusTimeout time.Duration
)

// migrateOutputDoc is the shared migrate-sweep report envelope (D-11),
// mirroring migrateRemapReportDoc/pruneOutputDoc's explicit-bool-plus-
// separate-count-fields discipline: DryRun is an explicit boolean, and
// WouldMigrate/Migrated are SEPARATE fields — never one field whose meaning
// depends on which mode ran. Spared/Appeared are COUNTS
// (uint64(len(res.Spared))/uint64(len(res.Appeared))), never id lists — the
// simplest shape that cannot ever marshal a bare "null" for any field here,
// matching pruneReportDoc/migrateRemapDoc's scalar-only precedent.
type migrateOutputDoc struct {
	Target       int    `json:"target"`
	DryRun       bool   `json:"dry_run"`
	WouldMigrate uint64 `json:"would_migrate"`
	Migrated     uint64 `json:"migrated"`
	Failed       uint64 `json:"failed"`
	Passes       uint64 `json:"passes"`
	Backlog      uint64 `json:"backlog"`
	Spared       uint64 `json:"spared"`
	Appeared     uint64 `json:"appeared"`
}

// migrateReportDoc converts (res, target, dryRun, wouldMigrate) into
// migrateOutputDoc. wouldMigrate is ALWAYS supplied by the caller, never
// derived from res.PreviewManifest inside this function (REVIEWS.md N4):
// store.MigrateResult carries no WouldMigrate field by design (04-01's
// prohibition), and an applied-mode res never carries a populated
// PreviewManifest at all — only the CLI run funcs know which manifest (the
// DryRun preview's, or the apply closure's own fresh re-preview) the
// invocation actually acted from, so the projection count is derived once,
// at the call site, from that manifest's length.
func migrateReportDoc(res store.MigrateResult, target migrate.Version, dryRun bool, wouldMigrate uint64) migrateOutputDoc {
	return migrateOutputDoc{
		Target:       int(target),
		DryRun:       dryRun,
		WouldMigrate: wouldMigrate,
		Migrated:     res.Migrated,
		Failed:       res.Failed,
		Passes:       res.Passes,
		Backlog:      res.Backlog,
		Spared:       uint64(len(res.Spared)),
		Appeared:     uint64(len(res.Appeared)),
	}
}

// migrateSummary renders the operator-facing one-line result of a migrate
// sweep, for both the preview and applied shapes — pure (no I/O), mirroring
// pruneSummary/migrateRemapSummary's discipline (prune.go:122, migrate.go:204).
func migrateSummary(res store.MigrateResult, target migrate.Version, dryRun bool, wouldMigrate uint64) string {
	if dryRun {
		return fmt.Sprintf("preview: %d record(s) would migrate to v%d; re-run with --apply to migrate",
			wouldMigrate, int(target))
	}
	return fmt.Sprintf("migrated %d of %d previewed record(s) to v%d (backlog now %d; %d failed, %d spared since preview, %d appeared since preview — not migrated; re-run to include)",
		res.Migrated, wouldMigrate, int(target), res.Backlog, res.Failed, len(res.Spared), len(res.Appeared))
}

// migrateSweepPreviewRun is the shared v0->v1 sweep PREVIEW implementation
// (D-11/cycle-3 #7): migrateCmd's own preview closure and (04-04)
// backfillShortIDsCmd's alias both call this exact function, one-line
// adapters over it, so "thin identical alias" is a structural fact rather
// than a claim. No writes on this path: Store.Migrate's DryRun mode performs
// a full-backlog projection only.
func migrateSweepPreviewRun(ctx context.Context, cmd *cobra.Command, outputFlag string, timeout time.Duration) error {
	format, err := operatorOutputFormat(cmd, outputFlag)
	if err != nil {
		return err
	}
	st, err := migrateFamilyStoreFromEnv()
	if err != nil {
		return classifyOperatorErrConstruction(err)
	}
	ctx, cancel := migrateWithTimeout(ctx, timeout)
	defer cancel()

	res, err := st.Migrate(ctx, store.MigrateOptions{DryRun: true})
	if err != nil {
		return classifyOperatorErr(err)
	}
	wouldMigrate := uint64(len(res.PreviewManifest))
	return renderOperator(cmd, format,
		migrateSummary(res, migrate.CurrentVersion, true, wouldMigrate),
		migrateReportDoc(res, migrate.CurrentVersion, true, wouldMigrate))
}

// migrateSweepApplyRun is the shared v0->v1 sweep APPLY implementation
// (REVIEWS.md H5 — the purge in-apply-closure re-preview pattern,
// spine_review_purge.go:339-377, spinePurgeApplyRun): it calls
// Migrate(DryRun:true) FIRST for a FRESH manifest WITHIN this invocation,
// then calls Migrate(Manifest:freshManifest) with it — never a
// package-level var bridging separate invocations. Only the manifest ∩
// fresh-re-derivation intersection is migrated (H6/SC3): a record previewed
// but no longer eligible is Spared, a record eligible now but never
// previewed is Appeared (not migrated).
func migrateSweepApplyRun(ctx context.Context, cmd *cobra.Command, outputFlag string, timeout time.Duration) error {
	format, err := operatorOutputFormat(cmd, outputFlag)
	if err != nil {
		return err
	}
	st, err := migrateFamilyStoreFromEnv()
	if err != nil {
		return classifyOperatorErrConstruction(err)
	}
	ctx, cancel := migrateWithTimeout(ctx, timeout)
	defer cancel()

	previewRes, err := st.Migrate(ctx, store.MigrateOptions{DryRun: true})
	if err != nil {
		return classifyOperatorErr(err)
	}
	previewManifest := previewRes.PreviewManifest

	res, err := st.Migrate(ctx, store.MigrateOptions{Manifest: previewManifest})
	if err != nil {
		return classifyOperatorErr(err)
	}
	wouldMigrate := uint64(len(previewManifest))
	return renderOperator(cmd, format,
		migrateSummary(res, migrate.CurrentVersion, false, wouldMigrate),
		migrateReportDoc(res, migrate.CurrentVersion, false, wouldMigrate))
}

// migrateSweepPreview is registerDestructive's preview closure for
// migrateCmd: a one-line adapter over the shared migrateSweepPreviewRun,
// passing this leaf's own --output/--timeout vars.
func migrateSweepPreview(ctx context.Context, cmd *cobra.Command) error {
	return migrateSweepPreviewRun(ctx, cmd, migrateSweepOutput, migrateSweepTimeout)
}

// migrateSweepApplyClosure is registerDestructive's apply closure for
// migrateCmd.
func migrateSweepApplyClosure(ctx context.Context, cmd *cobra.Command) error {
	return migrateSweepApplyRun(ctx, cmd, migrateSweepOutput, migrateSweepTimeout)
}

// migrateCmd is Phase 4's forward sweep: additive-only by construction
// (migrate.CheckAdditive refuses the whole call if a registered step's
// declared key set does not match what it actually adds), yet it still
// previews by default and mutates only under --apply, the SAME preview/apply
// contract every destructive operator command gets — the D-16 generalization
// this phase exists to ship. Routed through registerDestructive
// (destructive.go), whose ADMISSION gate is `!class.ReadOnly` (Task 1), not
// `class.Destructive` — this command's own toolclass row below is
// Destructive:false.
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Preview or apply the pending forward schema-version sweep",
	Long: "Advance every below-target record through the registered migrate.Registry step chain " +
		"(today: v0->v1, minting a short_id for any record that lacks one). A bare invocation " +
		"previews the full backlog and writes nothing; --apply re-derives eligibility within its " +
		"own run (a fresh preview inside the apply closure) and migrates only the intersection of " +
		"what it just showed and what is still eligible at that moment — a record that became " +
		"ineligible since preview is spared, and a record that became newly eligible is reported " +
		"appeared (never migrated; re-run to include it).",
}

// migrateStatusCmd reports Store.MigrateStatus's server-side version-
// distribution histogram. Read-only by construction (D-07/D-08) — never
// routed through registerDestructive — yet it STILL carries --timeout and
// signal.NotifyContext like every other Qdrant-dialing operator command,
// including the read-only ones (REVIEWS.md C5-M6: spine-review scan and
// spine-review verify are both read-only and install both).
var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report a server-side schema-version distribution histogram",
	RunE: func(cmd *cobra.Command, _ []string) error {
		format, err := operatorOutputFormat(cmd, migrateStatusOutput)
		if err != nil {
			return err
		}
		st, err := migrateFamilyStoreFromEnv()
		if err != nil {
			return classifyOperatorErrConstruction(err)
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		ctx, cancel := migrateWithTimeout(ctx, migrateStatusTimeout)
		defer cancel()
		res, err := st.MigrateStatus(ctx)
		if err != nil {
			return classifyOperatorErr(err)
		}
		return renderOperator(cmd, format, statusSummary(res), statusReportDoc(res))
	},
}

// statusReportDoc normalizes res into the JSON-mode shape migrate status
// renders: store.MigrateStatusResult's own json tags (buckets/absent/
// future/future_total/total) are already the exact contract this surface
// needs, so no separate CLI-side type exists — this function's only job is
// the REVIEWS.md C5-L8 normalization: Buckets/Future must marshal as `[]`,
// never `null`, and a zero-valued MigrateStatusResult leaves both nil.
func statusReportDoc(res store.MigrateStatusResult) store.MigrateStatusResult {
	if res.Buckets == nil {
		res.Buckets = []store.VersionBucket{}
	}
	if res.Future == nil {
		res.Future = []store.VersionBucket{}
	}
	return res
}

// statusSummary renders the operator-facing text report of a migrate status
// histogram — pure (no I/O). It names the distinct FUTURE versions when any
// exist (REVIEWS.md M4), never collapsing them into FutureTotal alone, so an
// operator can distinguish one-version-ahead drift from a wildly newer
// binary.
func statusSummary(res store.MigrateStatusResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "migrate status: %d record(s) total, %d absent (never migrated)", res.Total, res.Absent)
	for _, bucket := range res.Buckets {
		fmt.Fprintf(&b, ", %d at v%d", bucket.Count, bucket.Version)
	}
	if len(res.Future) > 0 {
		fmt.Fprintf(&b, "; %d record(s) at a version newer than this binary's current target (v%d):", res.FutureTotal, int(migrate.CurrentVersion))
		for _, bucket := range res.Future {
			fmt.Fprintf(&b, " v%d=%d", bucket.Version, bucket.Count)
		}
	}
	return b.String()
}

func init() {
	addOperatorOutputFlag(migrateCmd, &migrateSweepOutput)
	migrateCmd.Flags().DurationVar(&migrateSweepTimeout, "timeout", 5*time.Minute,
		"max wall-clock (0 disables); also cancellable via Ctrl-C")

	addOperatorOutputFlag(migrateStatusCmd, &migrateStatusOutput)
	migrateStatusCmd.Flags().DurationVar(&migrateStatusTimeout, "timeout", 5*time.Minute,
		"max wall-clock (0 disables); also cancellable via Ctrl-C")

	// AddCommand FIRST, mirroring spine_review_purge.go's precedent:
	// registerDestructive's classification lookup keys on commandKey(cmd),
	// which walks cmd.CommandPath() through its parent.
	migrateCmd.AddCommand(migrateStatusCmd)
	rootCmd.AddCommand(migrateCmd)
	registerDestructive(migrateCmd, &migrateApply, migrateSweepPreview, migrateSweepApplyClosure)
}
