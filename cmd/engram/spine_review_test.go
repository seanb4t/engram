// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"testing"
)

// TestSpineReviewScanRejectsInvalidOutput proves `spine-review scan`
// validates --output through operatorOutputFormat (which calls
// config.ValidateOutputFormat, the SAME exported validator the client tier
// uses) from its very first leaf: an illegal value exits exitUsage through
// Phase 1's taxonomy, never a silently-ignored value or a bare framework
// error.
func TestSpineReviewScanRejectsInvalidOutput(t *testing.T) {
	resetClientFlags(t)
	_, _, err := runClient(t, "spine-review", "scan", "--all-scopes", "--output", "yaml")
	if err == nil {
		t.Fatal("expected an error for --output yaml, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

// TestSpineReviewScanRequiresScopeOrAllScopes pins the bare usageErrorf
// guard (mirroring summarize.go's exact wording) that rejects a scan
// invocation naming neither --scope nor --all-scopes.
func TestSpineReviewScanRequiresScopeOrAllScopes(t *testing.T) {
	resetClientFlags(t)
	_, _, err := runClient(t, "spine-review", "scan")
	if err == nil {
		t.Fatal("expected an error when neither --scope nor --all-scopes is supplied, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

// TestOperatorOutputFormatResolvesNonTTYForCustomWriter proves
// operatorOutputFormat derives TTY state from cmd.OutOrStdout(), not the
// process's real os.Stdout: a command whose output writer is a
// *bytes.Buffer must resolve to JSON (non-TTY) regardless of whatever the
// test binary's own real stdout happens to be.
func TestOperatorOutputFormatResolvesNonTTYForCustomWriter(t *testing.T) {
	var buf bytes.Buffer
	cmd := spineReviewScanCmd
	origOut := cmd.OutOrStdout()
	cmd.SetOut(&buf)
	t.Cleanup(func() { cmd.SetOut(origOut) })

	got, err := operatorOutputFormat(cmd, "")
	if err != nil {
		t.Fatalf("operatorOutputFormat(cmd, \"\") returned error: %v", err)
	}
	if got != formatJSON {
		t.Errorf("operatorOutputFormat resolved %v, want formatJSON (non-TTY) for a *bytes.Buffer writer", got)
	}
}
