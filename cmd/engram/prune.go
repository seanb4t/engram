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
	pruneOlderThan time.Duration
	pruneTimeout   time.Duration
)

// pruneExpiredCmd deletes memories whose not_after has lapsed by at least
// --older-than. Operator-run reclamation; recall already hides expired records
// at read time, so this only reclaims storage. Collection-wide, no per-caller authz.
var pruneExpiredCmd = &cobra.Command{
	Use:   "prune-expired",
	Short: "Delete memories whose validity window (not_after) has lapsed",
	RunE: func(cmd *cobra.Command, _ []string) error {
		st, err := server.StoreFromEnv()
		if err != nil {
			return classifyOperatorErrConstruction(err)
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if pruneTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, pruneTimeout)
			defer cancel()
		}
		before := pruneCutoff(time.Now().UTC(), pruneOlderThan)
		n, err := st.PruneExpired(ctx, before)
		if err != nil {
			return classifyOperatorErr(err)
		}
		cmd.Printf("pruned ~%d expired record(s) (not_after < %s; best-effort count)\n", n, before.Format(time.RFC3339))
		return nil
	},
}

// pruneCutoff is the not_after threshold prune-expired deletes below: now minus
// the --older-than grace period. A record whose not_after lapsed more recently
// than the grace window is therefore spared (older-than=0 → cutoff is now).
func pruneCutoff(now time.Time, olderThan time.Duration) time.Time {
	return now.Add(-olderThan)
}

func init() {
	pruneExpiredCmd.Flags().DurationVar(&pruneOlderThan, "older-than", 0,
		"grace period: only prune records whose not_after lapsed at least this long ago (0 = any past not_after)")
	pruneExpiredCmd.Flags().DurationVar(&pruneTimeout, "timeout", 5*time.Minute,
		"max wall-clock for the sweep (0 disables); also cancellable via Ctrl-C")
	rootCmd.AddCommand(pruneExpiredCmd)
}
