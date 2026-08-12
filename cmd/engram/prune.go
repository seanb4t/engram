// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/server"
)

var (
	pruneOlderThan time.Duration
	pruneTimeout   time.Duration
	pruneOutput    string
	pruneApply     bool
)

// pruneExpiredCmd reclaims memories whose validity window (not_after) has
// lapsed by at least --older-than. Classified Destructive in
// internal/surfaces/toolclass.go, so it is registered through
// registerDestructive (destructive.go): a bare invocation previews the
// eligible count and performs NO delete; --apply performs it. The command
// does not assign its own RunE — registerDestructive owns that, which is
// what TestDestructiveCommandsRouteThroughGate asserts. Collection-wide, no
// per-caller authz.
var pruneExpiredCmd = &cobra.Command{
	Use:   "prune-expired",
	Short: "Delete memories whose validity window (not_after) has lapsed",
}

// pruneCutoff is the not_after threshold prune-expired deletes below: now
// minus the --older-than grace period. A record whose not_after lapsed more
// recently than the grace window is therefore spared (older-than=0 →
// cutoff is now).
func pruneCutoff(now time.Time, olderThan time.Duration) time.Time {
	return now.Add(-olderThan)
}

// pruneWithTimeout wraps ctx with pruneTimeout when positive (0 disables the
// deadline, unchanged from before this plan) — factored out so the preview
// and apply closures below apply the identical bound.
func pruneWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if pruneTimeout > 0 {
		return context.WithTimeout(ctx, pruneTimeout)
	}
	return ctx, func() {}
}

// pruneCutoffNow computes today's cutoff from the cliNow seam (destructive.go),
// truncated to the second: not_after is compared at second granularity
// (float64(before.Unix()) in expiredFilter), so quantising here is what
// keeps a preview run's rendered/JSON "before" byte-identical across two
// cliNow() reads that differ only by a fraction of a second, without
// depending on both reads landing on the exact same instant.
func pruneCutoffNow() time.Time {
	return pruneCutoff(cliNow().Truncate(time.Second), pruneOlderThan)
}

// prunePreview is registerDestructive's preview closure: it counts eligible
// records via st.CountExpired — the SAME expiredFilter construction
// st.PruneExpired's applied path reads (internal/store/spine.go) — and
// performs no delete.
func prunePreview(ctx context.Context, cmd *cobra.Command) error {
	format, err := operatorOutputFormat(cmd, pruneOutput)
	if err != nil {
		return err
	}
	st, err := server.StoreFromEnv()
	if err != nil {
		return classifyOperatorErrConstruction(err)
	}
	ctx, cancel := pruneWithTimeout(ctx)
	defer cancel()
	before := pruneCutoffNow()
	n, err := st.CountExpired(ctx, before)
	if err != nil {
		return classifyOperatorErr(err)
	}
	return renderOperator(cmd, format, prunePreviewSummary(n, before), prunePreviewDoc(n, before))
}

// pruneApplyRun is registerDestructive's apply closure: the deletion
// behavior this command had unconditionally before this plan, now gated
// behind --apply.
func pruneApplyRun(ctx context.Context, cmd *cobra.Command) error {
	format, err := operatorOutputFormat(cmd, pruneOutput)
	if err != nil {
		return err
	}
	st, err := server.StoreFromEnv()
	if err != nil {
		return classifyOperatorErrConstruction(err)
	}
	ctx, cancel := pruneWithTimeout(ctx)
	defer cancel()
	before := pruneCutoffNow()
	n, err := st.PruneExpired(ctx, before)
	if err != nil {
		return classifyOperatorErr(err)
	}
	return renderOperator(cmd, format, pruneSummary(n, before), pruneReportDoc(n, before))
}

// prunePreviewSummary renders the operator-facing one-line PREVIEW result: no
// delete has occurred, and the sentence names --apply as the flag that
// performs one. Kept pure (no I/O).
func prunePreviewSummary(eligible uint64, before time.Time) string {
	return fmt.Sprintf("preview: %d expired record(s) eligible for deletion (not_after < %s; best-effort count); re-run with --apply to delete",
		eligible, before.Format(time.RFC3339))
}

// pruneSummary renders the operator-facing one-line APPLIED result of a
// prune-expired sweep. Kept pure (no I/O), mirroring reindexSummary's
// discipline (reindex.go), and returns the pre-D-13 sentence unchanged,
// character for character — renderOperator appends the trailing newline on
// write, so this deliberately carries none.
func pruneSummary(deleted uint64, before time.Time) string {
	return fmt.Sprintf("pruned ~%d expired record(s) (not_after < %s; best-effort count)",
		deleted, before.Format(time.RFC3339))
}

// pruneOutputDoc is the JSON-mode shape of a prune-expired sweep. Preview is
// an explicit boolean, never inferred from prose; Eligible and Deleted are
// SEPARATE fields — never one field whose meaning depends on which mode
// ran — so a json consumer cannot mistake a preview count for a completed
// deletion. BestEffort carries the same "this count is not exact" caveat
// the text sentence states in prose, as an explicit field.
type pruneOutputDoc struct {
	Preview    bool      `json:"preview"`
	Eligible   uint64    `json:"eligible"`
	Deleted    uint64    `json:"deleted"`
	Before     time.Time `json:"before"`
	BestEffort bool      `json:"best_effort"`
}

// prunePreviewDoc converts (eligible, before) into pruneOutputDoc's preview
// shape.
func prunePreviewDoc(eligible uint64, before time.Time) pruneOutputDoc {
	return pruneOutputDoc{Preview: true, Eligible: eligible, Before: before, BestEffort: true}
}

// pruneReportDoc converts (deleted, before) into pruneOutputDoc's applied
// shape.
func pruneReportDoc(deleted uint64, before time.Time) pruneOutputDoc {
	return pruneOutputDoc{Preview: false, Deleted: deleted, Before: before, BestEffort: true}
}

func init() {
	addOperatorOutputFlag(pruneExpiredCmd, &pruneOutput)
	pruneExpiredCmd.Flags().DurationVar(&pruneOlderThan, "older-than", 0,
		"grace period: only prune records whose not_after lapsed at least this long ago (0 = any past not_after)")
	pruneExpiredCmd.Flags().DurationVar(&pruneTimeout, "timeout", 5*time.Minute,
		"max wall-clock for the sweep (0 disables); also cancellable via Ctrl-C")
	registerDestructive(pruneExpiredCmd, &pruneApply, prunePreview, pruneApplyRun)
	rootCmd.AddCommand(pruneExpiredCmd)
}
