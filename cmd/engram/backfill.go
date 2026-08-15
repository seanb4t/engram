// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"time"

	"github.com/spf13/cobra"
)

var (
	backfillTimeout time.Duration
	backfillOutput  string
	backfillApply   bool
)

// backfillShortIDsCmd is a thin delegating alias (D-10/D-11) onto the SAME
// v0->v1 sweep `engram migrate` runs: its two closures are one-line adapters
// over 04-03's shared migrateSweepPreviewRun/migrateSweepApplyRun, so its
// --apply path performs the identical DryRun->manifest-limited-apply
// intersection as `engram migrate --apply` (REVIEWS.md cycle-3 #7) rather
// than constructing its own store.MigrateOptions. It is soft-deprecated
// toward `engram migrate` (D-12, migrateSetOwnerCmd.Deprecated precedent,
// migrate.go:261) — never hard-removed. Its own --timeout flag is
// PRESERVED (REVIEWS.md H8), including the "0 disables" semantics it
// already shipped, and its value travels into the shared run funcs as an
// argument (they own deadline installation) — see backfillTimeout's own
// doc comment on the init() registration below.
var backfillShortIDsCmd = &cobra.Command{
	Use:   "backfill-short-ids",
	Short: "Assign a short_id to every memory that lacks one (payload-only; no re-embed)",
}

func backfillPreview(ctx context.Context, cmd *cobra.Command) error {
	return migrateSweepPreviewRun(ctx, cmd, backfillOutput, backfillTimeout)
}

func backfillApplyRun(ctx context.Context, cmd *cobra.Command) error {
	return migrateSweepApplyRun(ctx, cmd, backfillOutput, backfillTimeout)
}

func init() {
	addOperatorOutputFlag(backfillShortIDsCmd, &backfillOutput)
	// backfillTimeout is PRESERVED (REVIEWS.md H8), including its shipped
	// default and usage string — it is never used at a call site in this
	// file (REVIEWS.md C6-H4): it is passed AS AN ARGUMENT into
	// migrateSweepPreviewRun/migrateSweepApplyRun above, which install the
	// deadline themselves. This file must not also call migrateWithTimeout
	// directly, which would install a second, nested, redundant deadline.
	backfillShortIDsCmd.Flags().DurationVar(&backfillTimeout, "timeout", 5*time.Minute, "max wall-clock (0 disables); also cancellable via Ctrl-C")
	// AddCommand BEFORE registerDestructive, mirroring migrate_family.go's
	// precedent: registerDestructive's classification lookup keys on
	// commandKey(cmd), which walks cmd.CommandPath() through its parent.
	rootCmd.AddCommand(backfillShortIDsCmd)
	registerDestructive(backfillShortIDsCmd, &backfillApply, backfillPreview, backfillApplyRun)
	backfillShortIDsCmd.Deprecated = "use: engram migrate"
}
