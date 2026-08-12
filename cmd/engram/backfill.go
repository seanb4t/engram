// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/server"
)

var (
	backfillDryRun  bool
	backfillTimeout time.Duration
	backfillOutput  string
)

var backfillShortIDsCmd = &cobra.Command{
	Use:   "backfill-short-ids",
	Short: "Assign a short_id to every memory that lacks one (payload-only; no re-embed)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		format, err := operatorOutputFormat(cmd, backfillOutput)
		if err != nil {
			return err
		}
		st, err := server.StoreFromEnv()
		if err != nil {
			return classifyOperatorErrConstruction(err)
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if backfillTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, backfillTimeout)
			defer cancel()
		}
		n, err := st.BackfillShortIDs(ctx, backfillDryRun)
		if err != nil {
			if n > 0 {
				cmd.PrintErrf("aborted after backfilling %d record(s)\n", n)
			}
			return classifyOperatorErr(err)
		}
		return renderOperator(cmd, format, backfillSummary(n, backfillDryRun), backfillReportDoc(n, backfillDryRun))
	},
}

// backfillSummary renders the operator-facing one-line result of a
// backfill-short-ids run. Pure (no I/O), returning the pre-D-13 sentence
// unchanged, character for character, for both the dry-run and applied
// shapes.
func backfillSummary(n uint64, dryRun bool) string {
	if dryRun {
		return fmt.Sprintf("[dry-run] would backfill %d record(s)", n)
	}
	return fmt.Sprintf("backfilled %d record(s)", n)
}

// backfillOutputDoc is the JSON-mode shape of a backfill-short-ids run.
type backfillOutputDoc struct {
	DryRun bool   `json:"dry_run"`
	Count  uint64 `json:"count"`
}

// backfillReportDoc converts (n, dryRun) into backfillOutputDoc.
func backfillReportDoc(n uint64, dryRun bool) backfillOutputDoc {
	return backfillOutputDoc{DryRun: dryRun, Count: n}
}

func init() {
	addOperatorOutputFlag(backfillShortIDsCmd, &backfillOutput)
	backfillShortIDsCmd.Flags().BoolVar(&backfillDryRun, "dry-run", false, "count records missing a short_id without writing")
	backfillShortIDsCmd.Flags().DurationVar(&backfillTimeout, "timeout", 5*time.Minute, "max wall-clock (0 disables); also cancellable via Ctrl-C")
	rootCmd.AddCommand(backfillShortIDsCmd)
}
