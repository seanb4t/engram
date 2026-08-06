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
	pruneOlderThan time.Duration
	pruneTimeout   time.Duration
	pruneOutput    string
)

// pruneExpiredCmd deletes memories whose not_after has lapsed by at least
// --older-than. Operator-run reclamation; recall already hides expired records
// at read time, so this only reclaims storage. Collection-wide, no per-caller authz.
var pruneExpiredCmd = &cobra.Command{
	Use:   "prune-expired",
	Short: "Delete memories whose validity window (not_after) has lapsed",
	RunE: func(cmd *cobra.Command, _ []string) error {
		format, err := operatorOutputFormat(cmd, pruneOutput)
		if err != nil {
			return err
		}
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
		return renderOperator(cmd, format, pruneSummary(n, before), pruneReportDoc(n, before))
	},
}

// pruneCutoff is the not_after threshold prune-expired deletes below: now minus
// the --older-than grace period. A record whose not_after lapsed more recently
// than the grace window is therefore spared (older-than=0 → cutoff is now).
func pruneCutoff(now time.Time, olderThan time.Duration) time.Time {
	return now.Add(-olderThan)
}

// pruneSummary renders the operator-facing one-line result of a prune-expired
// sweep. Kept pure (no I/O), mirroring reindexSummary's discipline
// (reindex.go), and returns the pre-D-13 sentence unchanged, character for
// character — renderOperator appends the trailing newline on write, so this
// deliberately carries none.
func pruneSummary(deleted uint64, before time.Time) string {
	return fmt.Sprintf("pruned ~%d expired record(s) (not_after < %s; best-effort count)",
		deleted, before.Format(time.RFC3339))
}

// pruneOutputDoc is the JSON-mode shape of a prune-expired sweep. BestEffort
// carries the same "this count is not exact" caveat the text sentence
// states in prose, as an explicit boolean field — a json consumer must not
// be told something more certain than the human reader is (this plan's
// transparency prohibition).
type pruneOutputDoc struct {
	Deleted    uint64    `json:"deleted"`
	Before     time.Time `json:"before"`
	BestEffort bool      `json:"best_effort"`
}

// pruneReportDoc converts (deleted, before) into pruneOutputDoc. Pure.
func pruneReportDoc(deleted uint64, before time.Time) pruneOutputDoc {
	return pruneOutputDoc{Deleted: deleted, Before: before, BestEffort: true}
}

func init() {
	addOperatorOutputFlag(pruneExpiredCmd, &pruneOutput)
	pruneExpiredCmd.Flags().DurationVar(&pruneOlderThan, "older-than", 0,
		"grace period: only prune records whose not_after lapsed at least this long ago (0 = any past not_after)")
	pruneExpiredCmd.Flags().DurationVar(&pruneTimeout, "timeout", 5*time.Minute,
		"max wall-clock for the sweep (0 disables); also cancellable via Ctrl-C")
	rootCmd.AddCommand(pruneExpiredCmd)
}
