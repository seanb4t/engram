// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// TestPruneCutoffAppliesGracePeriod pins the prune-expired --older-than arithmetic
// (hr2.9): the cutoff is now minus the grace period, so records whose not_after
// lapsed more recently than the grace window are spared. older-than=0 prunes
// anything already past not_after.
func TestPruneCutoffAppliesGracePeriod(t *testing.T) {
	now := time.Date(2031, 1, 2, 15, 0, 0, 0, time.UTC)

	if got := pruneCutoff(now, 0); !got.Equal(now) {
		t.Errorf("pruneCutoff(now, 0) = %v, want %v (no grace → cutoff is now)", got, now)
	}

	want := now.Add(-24 * time.Hour)
	if got := pruneCutoff(now, 24*time.Hour); !got.Equal(want) {
		t.Errorf("pruneCutoff(now, 24h) = %v, want %v (now minus grace)", got, want)
	}
}

// TestPruneRejectsInvalidOutput proves `prune-expired` validates --output
// through the shared operatorOutputFormat before dialing any store.
func TestPruneRejectsInvalidOutput(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, pruneExpiredCmd)
	_, _, err := runClient(t, "prune-expired", "--output", "yaml")
	if err == nil {
		t.Fatal("expected an error for --output yaml, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

// TestPruneSummaryUnchanged pins pruneSummary's pure formatter against the
// pre-backfill literal sentence, byte for byte (minus the trailing newline
// renderOperator now appends on write instead of the format string
// itself).
func TestPruneSummaryUnchanged(t *testing.T) {
	before := time.Date(2031, 1, 2, 15, 0, 0, 0, time.UTC)
	got := pruneSummary(3, before)
	want := "pruned ~3 expired record(s) (not_after < " + before.Format(time.RFC3339) + "; best-effort count)"
	if got != want {
		t.Errorf("pruneSummary(3, before) = %q, want %q", got, want)
	}
}

// TestPruneOutputJSONHasBestEffortMarker proves `prune-expired --output
// json`'s document parses via json.Unmarshal into a map[string]any that
// carries the best-effort marker key — the count's caveat as an explicit
// field, not prose a json consumer cannot rely on.
func TestPruneOutputJSONHasBestEffortMarker(t *testing.T) {
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "prune-expired"}
	cmd.SetOut(&buf)
	before := time.Date(2031, 1, 2, 15, 0, 0, 0, time.UTC)
	doc := pruneReportDoc(3, before)
	if err := renderOperator(cmd, formatJSON, pruneSummary(3, before), doc); err != nil {
		t.Fatalf("renderOperator: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", buf.String(), err)
	}
	bestEffort, ok := m["best_effort"]
	if !ok {
		t.Fatalf("prune-expired json doc missing best_effort key: %s", buf.String())
	}
	if bestEffort != true {
		t.Errorf(`m["best_effort"] = %v, want true`, bestEffort)
	}
	if m["deleted"] != float64(3) {
		t.Errorf(`m["deleted"] = %v, want 3`, m["deleted"])
	}
}

// TestPruneTextModeUnchanged proves text-mode output is byte-identical to
// the pre-backfill sentence.
func TestPruneTextModeUnchanged(t *testing.T) {
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "prune-expired"}
	cmd.SetOut(&buf)
	before := time.Date(2031, 1, 2, 15, 0, 0, 0, time.UTC)
	text := pruneSummary(3, before)
	if err := renderOperator(cmd, formatText, text, pruneReportDoc(3, before)); err != nil {
		t.Fatalf("renderOperator: %v", err)
	}
	if got, want := buf.String(), text+"\n"; got != want {
		t.Errorf("text-mode output = %q, want %q", got, want)
	}
}
