// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
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
)

var backfillShortIDsCmd = &cobra.Command{
	Use:   "backfill-short-ids",
	Short: "Assign a short_id to every memory that lacks one (payload-only; no re-embed)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		st, err := server.StoreFromEnv()
		if err != nil {
			return err
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
			return err
		}
		if backfillDryRun {
			cmd.Printf("[dry-run] would backfill %d record(s)\n", n)
		} else {
			cmd.Printf("backfilled %d record(s)\n", n)
		}
		return nil
	},
}

func init() {
	backfillShortIDsCmd.Flags().BoolVar(&backfillDryRun, "dry-run", false, "count records missing a short_id without writing")
	backfillShortIDsCmd.Flags().DurationVar(&backfillTimeout, "timeout", 5*time.Minute, "max wall-clock (0 disables); also cancellable via Ctrl-C")
	rootCmd.AddCommand(backfillShortIDsCmd)
}
