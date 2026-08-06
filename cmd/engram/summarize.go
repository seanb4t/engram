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
	"github.com/seanb4t/engram/internal/store"
)

var (
	summarizeScope     string
	summarizeAllScopes bool
	summarizeOlderThan time.Duration
	summarizeLimit     int
	summarizeDryRun    bool
	summarizeTimeout   time.Duration
	summarizeOutput    string
)

// summarizeMissingCmd fills empty recall summaries via the configured cheap
// model (ENGRAM_SUMMARY_MODEL). Operator-run, off the write path; mirrors
// reindex/prune-expired. Records that already have a summary or whose content is
// shorter than ENGRAM_SUMMARY_MAX_CHARS are skipped.
var summarizeMissingCmd = &cobra.Command{
	Use:   "summarize-missing",
	Short: "Fill empty recall summaries with the configured cheap model",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if summarizeScope == "" && !summarizeAllScopes {
			return usageErrorf("--scope <scope> or --all-scopes is required")
		}
		format, err := operatorOutputFormat(cmd, summarizeOutput)
		if err != nil {
			return err
		}
		st, sm, model, maxChars, err := server.StoreAndSummarizerFromEnv()
		if err != nil {
			return classifyOperatorErrConstruction(err)
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if summarizeTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, summarizeTimeout)
			defer cancel()
		}
		var older time.Time
		if summarizeOlderThan > 0 {
			older = time.Now().UTC().Add(-summarizeOlderThan)
		}
		res, err := st.SummarizeMissing(ctx, store.SummarizeOptions{
			Scope:     summarizeScope,
			AllScopes: summarizeAllScopes,
			OlderThan: older,
			Limit:     summarizeLimit,
			MaxChars:  maxChars,
			Model:     model,
			DryRun:    summarizeDryRun,
		}, sm.Summarize)
		if err != nil {
			return classifyOperatorErr(err)
		}
		return renderOperator(cmd, format, summarizeSummary(res, summarizeDryRun), summarizeReportDoc(res, summarizeDryRun))
	},
}

// summarizeSummary renders the operator-facing one-line result. Pure (no I/O) so
// the dry-run vs live wording is unit-testable without a live gateway.
func summarizeSummary(res store.SummarizeResult, dryRun bool) string {
	if dryRun {
		return fmt.Sprintf("dry-run: %d of %d scanned record(s) would be summarized (%d skipped)",
			res.Filled, res.Scanned, res.Skipped)
	}
	return fmt.Sprintf("summarized %d of %d scanned record(s); %d skipped, %d failed",
		res.Filled, res.Scanned, res.Skipped, res.Failed)
}

// summarizeOutputDoc is the JSON-mode shape of a summarize-missing sweep:
// filled, skipped, scanned and failed counts as SEPARATE fields, plus
// DryRun as an explicit boolean — reusing summarizeSummary's own inputs,
// never recomputed.
type summarizeOutputDoc struct {
	DryRun  bool `json:"dry_run"`
	Scanned int  `json:"scanned"`
	Filled  int  `json:"filled"`
	Skipped int  `json:"skipped"`
	Failed  int  `json:"failed"`
}

// summarizeReportDoc converts (res, dryRun) into summarizeOutputDoc.
func summarizeReportDoc(res store.SummarizeResult, dryRun bool) summarizeOutputDoc {
	return summarizeOutputDoc{
		DryRun:  dryRun,
		Scanned: res.Scanned,
		Filled:  res.Filled,
		Skipped: res.Skipped,
		Failed:  res.Failed,
	}
}

func init() {
	addOperatorOutputFlag(summarizeMissingCmd, &summarizeOutput)
	summarizeMissingCmd.Flags().StringVar(&summarizeScope, "scope", "", "only summarize records in this scope")
	summarizeMissingCmd.Flags().BoolVar(&summarizeAllScopes, "all-scopes", false, "sweep every scope (required if --scope is omitted)")
	summarizeMissingCmd.Flags().DurationVar(&summarizeOlderThan, "older-than", 0, "only records created at least this long ago (0 = any age)")
	summarizeMissingCmd.Flags().IntVar(&summarizeLimit, "limit", 0, "max records to scan (0 = no cap)")
	summarizeMissingCmd.Flags().BoolVar(&summarizeDryRun, "dry-run", false, "count eligible records without writing")
	summarizeMissingCmd.Flags().DurationVar(&summarizeTimeout, "timeout", 30*time.Minute, "max wall-clock for the sweep (0 disables); also cancellable via Ctrl-C")
	rootCmd.AddCommand(summarizeMissingCmd)
}
