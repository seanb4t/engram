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
