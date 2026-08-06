// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import "testing"

func TestBackfillCmdHasDryRunAndTimeoutFlags(t *testing.T) {
	if backfillShortIDsCmd.Flags().Lookup("dry-run") == nil ||
		backfillShortIDsCmd.Flags().Lookup("timeout") == nil {
		t.Fatal("backfill-short-ids missing --dry-run/--timeout flag")
	}
}

// TestBackfillRejectsInvalidOutput proves `backfill-short-ids` validates
// --output through the shared operatorOutputFormat before dialing any
// store — the format check is this command's very first RunE statement,
// since it has no other required flags.
func TestBackfillRejectsInvalidOutput(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, backfillShortIDsCmd)
	_, _, err := runClient(t, "backfill-short-ids", "--output", "yaml")
	if err == nil {
		t.Fatal("expected an error for --output yaml, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

// TestBackfillSummaryUnchanged pins backfillSummary's pure formatter
// against the pre-backfill literal sentences.
func TestBackfillSummaryUnchanged(t *testing.T) {
	if got, want := backfillSummary(3, true), "[dry-run] would backfill 3 record(s)"; got != want {
		t.Errorf("backfillSummary(3, true) = %q, want %q", got, want)
	}
	if got, want := backfillSummary(3, false), "backfilled 3 record(s)"; got != want {
		t.Errorf("backfillSummary(3, false) = %q, want %q", got, want)
	}
}
