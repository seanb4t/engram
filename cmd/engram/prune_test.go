// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// TestPrunePreviewSummaryNamesApply proves the preview sentence names
// --apply as the flag that performs the deletion, so an operator who
// expected a delete learns the fix from the output itself.
func TestPrunePreviewSummaryNamesApply(t *testing.T) {
	before := time.Date(2031, 1, 2, 15, 0, 0, 0, time.UTC)
	got := prunePreviewSummary(3, before)
	if !strings.Contains(got, "--apply") {
		t.Errorf("prunePreviewSummary(3, before) = %q, want it to name --apply", got)
	}
	if !strings.Contains(got, "3 expired record") {
		t.Errorf("prunePreviewSummary(3, before) = %q, want it to state the eligible count", got)
	}
}

// TestPrunePreviewDocMarksPreviewNotDeleted proves the preview json document
// carries preview=true and eligible=n, and deleted=0 — never a value that
// could be mistaken for a completed deletion.
func TestPrunePreviewDocMarksPreviewNotDeleted(t *testing.T) {
	before := time.Date(2031, 1, 2, 15, 0, 0, 0, time.UTC)
	doc := prunePreviewDoc(3, before)
	if !doc.Preview {
		t.Error("prunePreviewDoc(3, before).Preview = false, want true")
	}
	if doc.Eligible != 3 {
		t.Errorf("prunePreviewDoc(3, before).Eligible = %d, want 3", doc.Eligible)
	}
	if doc.Deleted != 0 {
		t.Errorf("prunePreviewDoc(3, before).Deleted = %d, want 0 (a preview must not report as applied)", doc.Deleted)
	}

	applied := pruneReportDoc(3, before)
	if applied.Preview {
		t.Error("pruneReportDoc(3, before).Preview = true, want false")
	}
	if applied.Eligible != 0 {
		t.Errorf("pruneReportDoc(3, before).Eligible = %d, want 0", applied.Eligible)
	}
	if applied.Deleted != 3 {
		t.Errorf("pruneReportDoc(3, before).Deleted = %d, want 3", applied.Deleted)
	}
}

// TestPruneExpiredHelpGoldenListsApplyFlag proves --apply is documented on
// prune-expired's OWN help.golden section, never inferred from a whole-file
// `rg -c 'apply'` (which would also match the word appearing in an unrelated
// command's usage text — e.g. reindex's or backfill's own --apply-adjacent
// prose, if any existed).
func TestPruneExpiredHelpGoldenListsApplyFlag(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "help.golden"))
	if err != nil {
		t.Fatalf("read help.golden: %v", err)
	}
	section := helpGoldenSection(t, string(data), "engram prune-expired")
	if !strings.Contains(section, "--apply") {
		t.Errorf("## engram prune-expired section does not list --apply:\n%s", section)
	}
}

// helpGoldenSection extracts the body of help.golden's "## <heading>"
// section (up to the next "## " heading or EOF), so a golden assertion can
// be scoped to one command's own flag block rather than the whole file.
func helpGoldenSection(t *testing.T, content, heading string) string {
	t.Helper()
	marker := "## " + heading + "\n"
	idx := strings.Index(content, marker)
	if idx == -1 {
		t.Fatalf("help.golden has no %q section", marker)
	}
	rest := content[idx+len(marker):]
	if next := strings.Index(rest, "\n## "); next != -1 {
		return rest[:next]
	}
	return rest
}

// TestPrunePreviewCutoffQuantisedThroughCliNowSeam is the byte-identical
// re-run proof at the pure-function level: pinning cliNow to a FIXED instant
// makes two preview renderings byte-identical by construction, and
// advancing cliNow by a fraction of a second within the same whole second
// still matches — the cutoff is quantised (Truncate(time.Second)) through
// the seam rather than re-read from the wall clock at full precision.
func TestPrunePreviewCutoffQuantisedThroughCliNowSeam(t *testing.T) {
	origCliNow := cliNow
	origOlderThan := pruneOlderThan
	t.Cleanup(func() { cliNow = origCliNow; pruneOlderThan = origOlderThan })
	pruneOlderThan = 0

	fixed := time.Date(2031, 1, 2, 15, 0, 0, 0, time.UTC)

	cliNow = func() time.Time { return fixed }
	before1 := pruneCutoffNow()
	text1 := prunePreviewSummary(5, before1)

	cliNow = func() time.Time { return fixed }
	before2 := pruneCutoffNow()
	text2 := prunePreviewSummary(5, before2)

	if text1 != text2 {
		t.Fatalf("two preview renders with an IDENTICAL pinned cliNow differ:\n%q\n%q", text1, text2)
	}

	// Advance by a fraction of a second, still within the same whole second.
	cliNow = func() time.Time { return fixed.Add(700 * time.Millisecond) }
	before3 := pruneCutoffNow()
	text3 := prunePreviewSummary(5, before3)
	if text1 != text3 {
		t.Errorf("sub-second cliNow drift changed preview text:\n%q\n%q\nwant them to match — the cutoff must be quantised through the seam", text1, text3)
	}
}
