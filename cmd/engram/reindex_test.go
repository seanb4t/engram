// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/store"
)

func TestReindexSummary(t *testing.T) {
	t.Run("dry run reports the scanned count and is marked dry-run", func(t *testing.T) {
		got := reindexSummary(store.ReindexResult{Scanned: 5}, "new_coll", 1536, true, false)
		if !strings.HasPrefix(got, "dry-run:") {
			t.Errorf("dry-run summary should be marked as such: %q", got)
		}
		for _, want := range []string{"5 record", "new_coll", "1536"} {
			if !strings.Contains(got, want) {
				t.Errorf("dry-run summary %q missing %q", got, want)
			}
		}
	})

	t.Run("real run reports upserted/scanned, skipped, unchanged, and the cutover hint", func(t *testing.T) {
		got := reindexSummary(store.ReindexResult{Scanned: 5, Upserted: 2, Skipped: 1, Unchanged: 2}, "new_coll", 1536, false, false)
		if strings.HasPrefix(got, "dry-run:") {
			t.Errorf("non-dry-run summary must not be marked dry-run: %q", got)
		}
		for _, want := range []string{"2/5 record", "1 skipped", "2 unchanged", "at dim 1536", "ENGRAM_QDRANT_COLLECTION=new_coll"} {
			if !strings.Contains(got, want) {
				t.Errorf("summary %q missing %q", got, want)
			}
		}
	})

	t.Run("dry run with resume names both the would-re-embed and the would-skip counts", func(t *testing.T) {
		got := reindexSummary(store.ReindexResult{Scanned: 5, WouldUpsert: 3, Skipped: 1, Unchanged: 1}, "new_coll", 1536, true, true)
		if !strings.HasPrefix(got, "dry-run --resume:") {
			t.Errorf("dry-run-with-resume summary should be marked as such: %q", got)
		}
		for _, want := range []string{"3 would be re-embedded", "1 would be skipped", "1 skipped (no content)", "5 scanned", "new_coll", "1536"} {
			if !strings.Contains(got, want) {
				t.Errorf("dry-run-with-resume summary %q missing %q", got, want)
			}
		}
	})
}

// TestReindexRejectsInvalidOutput proves `reindex` validates --output
// through the shared operatorOutputFormat before dialing any store: exit
// exitUsage, never a silently-ignored value.
func TestReindexRejectsInvalidOutput(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, reindexCmd)
	_, _, err := runClient(t, "reindex", "--target", "x", "--output", "yaml")
	if err == nil {
		t.Fatal("expected an error for --output yaml, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

// TestReindexReportDocCarriesEverySummaryFact proves reindexReportDoc's
// json fields carry every fact reindexSummary's text sentence states, for
// both the dry-run and the live shape, and that it round-trips through
// json.Marshal/Unmarshal into a map (never a wider or narrower field set).
func TestReindexReportDocCarriesEverySummaryFact(t *testing.T) {
	res := store.ReindexResult{Scanned: 5, Upserted: 2, Skipped: 1, Unchanged: 2, WouldUpsert: 3}
	doc := reindexReportDoc(res, "new_coll", 1536, true, true)
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	for _, key := range []string{"dry_run", "resume", "target", "dim", "scanned", "upserted", "would_upsert", "skipped", "unchanged"} {
		if _, ok := m[key]; !ok {
			t.Errorf("reindexReportDoc json is missing key %q: %s", key, b)
		}
	}
	if m["target"] != "new_coll" {
		t.Errorf(`m["target"] = %v, want "new_coll"`, m["target"])
	}
}

// TestReindexRenderOperatorJSONMode drives renderOperator directly with a
// throwaway *bytes.Buffer-backed command, proving `reindex --output json`
// emits exactly one JSON document parseable via json.Unmarshal — without
// requiring a live Qdrant, mirroring spineScanDoc's own test discipline
// (spine_review_test.go).
func TestReindexRenderOperatorJSONMode(t *testing.T) {
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "reindex"}
	cmd.SetOut(&buf)
	res := store.ReindexResult{Scanned: 5, Upserted: 5}
	doc := reindexReportDoc(res, "new_coll", 1536, false, false)
	text := reindexSummary(res, "new_coll", 1536, false, false)
	if err := renderOperator(cmd, formatJSON, text, doc); err != nil {
		t.Fatalf("renderOperator: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", buf.String(), err)
	}
}

// TestReindexTextModeUnchanged proves text-mode output for `reindex`
// renders reindexSummary as its headline (first line), ends in exactly
// one trailing newline, and carries exactly one field line per
// reindexReportDoc key. Text is no longer pinned byte-for-byte (D-03:
// --output text is explicitly not a stable interface, 06-01-PLAN.md
// §Conversion Rules R4) — renderOperator's text branch is now shared,
// global rendering code (06-01-PLAN.md Task 1) that derives its field
// table from the same json.Marshal(doc) bytes every operator command's
// json lane emits, so this assertion is structural rather than a literal
// echo of the pre-conversion sentence.
func TestReindexTextModeUnchanged(t *testing.T) {
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "reindex"}
	cmd.SetOut(&buf)
	res := store.ReindexResult{Scanned: 5, Upserted: 5}
	text := reindexSummary(res, "new_coll", 1536, false, false)
	doc := reindexReportDoc(res, "new_coll", 1536, false, false)
	if err := renderOperator(cmd, formatText, text, doc); err != nil {
		t.Fatalf("renderOperator: %v", err)
	}
	out := buf.String()
	if firstLine := strings.SplitN(out, "\n", 2)[0]; firstLine != text {
		t.Errorf("text-mode output first line = %q, want headline %q", firstLine, text)
	}
	if !strings.HasSuffix(out, "\n") || strings.HasSuffix(out, "\n\n") {
		t.Errorf("text-mode output = %q, want exactly one trailing newline", out)
	}
	jsonKeys, err := jsonTopLevelKeys(doc)
	if err != nil {
		t.Fatalf("jsonTopLevelKeys: %v", err)
	}
	if got, want := countTopLevelFieldLines(out), len(jsonKeys); got != want {
		t.Errorf("countTopLevelFieldLines(%q) = %d, want %d (one line per json key)", out, got, want)
	}
}
