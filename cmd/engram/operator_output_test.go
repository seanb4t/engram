// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/migrate"
	"github.com/seanb4t/engram/internal/store"
)

// TestOperatorOutputFormat is table-driven coverage of operatorOutputFormat
// over a throwaway *cobra.Command (never rootCmd or a live operator
// command): a legal value resolves to the matching outputFormat, an
// illegal value is rejected with exitUsage through the SAME
// config.ValidateOutputFormat the client tier calls (never a second,
// unvalidated switch).
func TestOperatorOutputFormat(t *testing.T) {
	newCmdWithBuffer := func() *cobra.Command {
		cmd := &cobra.Command{Use: "throwaway"}
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		return cmd
	}

	t.Run("json resolves to formatJSON", func(t *testing.T) {
		got, err := operatorOutputFormat(newCmdWithBuffer(), "json")
		if err != nil {
			t.Fatalf("operatorOutputFormat(_, \"json\") returned error: %v", err)
		}
		if got != formatJSON {
			t.Errorf("operatorOutputFormat(_, \"json\") = %v, want formatJSON", got)
		}
	})

	t.Run("text resolves to formatText", func(t *testing.T) {
		got, err := operatorOutputFormat(newCmdWithBuffer(), "text")
		if err != nil {
			t.Fatalf("operatorOutputFormat(_, \"text\") returned error: %v", err)
		}
		if got != formatText {
			t.Errorf("operatorOutputFormat(_, \"text\") = %v, want formatText", got)
		}
	})

	t.Run("empty resolves to formatJSON for a non-TTY writer", func(t *testing.T) {
		got, err := operatorOutputFormat(newCmdWithBuffer(), "")
		if err != nil {
			t.Fatalf("operatorOutputFormat(_, \"\") returned error: %v", err)
		}
		if got != formatJSON {
			t.Errorf("operatorOutputFormat(_, \"\") = %v, want formatJSON (non-TTY *bytes.Buffer)", got)
		}
	})

	t.Run("illegal value exits exitUsage via the shared validator", func(t *testing.T) {
		_, err := operatorOutputFormat(newCmdWithBuffer(), "yaml")
		if err == nil {
			t.Fatal("operatorOutputFormat(_, \"yaml\") = nil error, want a usage error")
		}
		if got := exitCodeFromError(err); got != exitUsage {
			t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
		}
		if !strings.Contains(err.Error(), "--output") {
			t.Errorf("error %q does not name --output", err)
		}
	})
}

// TestRenderOperatorTextAndJSON pins renderOperator's own two-mode
// contract, independent of any specific operator command: text mode
// writes text plus exactly one trailing newline; json mode marshals doc as
// exactly one JSON document plus a trailing newline, parseable via
// json.Unmarshal.
func TestRenderOperatorTextAndJSON(t *testing.T) {
	type doc struct {
		Count int `json:"count"`
	}

	t.Run("text mode", func(t *testing.T) {
		// Structural assertions only (R4/D-08): text is a rendered VIEW of
		// doc now, not a literal echo of the headline argument, so this
		// pins the contract renderOperator still owes — headline first,
		// exactly one trailing newline, one field line per json key — not
		// an exact byte string.
		var buf bytes.Buffer
		cmd := &cobra.Command{Use: "throwaway"}
		cmd.SetOut(&buf)
		if err := renderOperator(cmd, formatText, "hello", doc{Count: 3}); err != nil {
			t.Fatalf("renderOperator: %v", err)
		}
		out := buf.String()
		if firstLine := strings.SplitN(out, "\n", 2)[0]; firstLine != "hello" {
			t.Errorf("renderOperator text mode first line = %q, want %q", firstLine, "hello")
		}
		if !strings.HasSuffix(out, "\n") || strings.HasSuffix(out, "\n\n") {
			t.Errorf("renderOperator text mode wrote %q, want exactly one trailing newline", out)
		}
		if got, want := countTopLevelFieldLines(out), 1; got != want {
			t.Errorf("countTopLevelFieldLines(%q) = %d, want %d (one field: count)", out, got, want)
		}
	})

	t.Run("json mode", func(t *testing.T) {
		var buf bytes.Buffer
		cmd := &cobra.Command{Use: "throwaway"}
		cmd.SetOut(&buf)
		if err := renderOperator(cmd, formatJSON, "hello", doc{Count: 3}); err != nil {
			t.Fatalf("renderOperator: %v", err)
		}
		var got doc
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("json.Unmarshal(%q): %v", buf.String(), err)
		}
		if got.Count != 3 {
			t.Errorf("decoded doc.Count = %d, want 3", got.Count)
		}
		if strings.Count(buf.String(), "\n") != 1 {
			t.Errorf("renderOperator json mode wrote %q, want exactly one trailing newline", buf.String())
		}
	})
}

// operatorParityRow is one (command, result value, text sentence, json
// document, facts) tuple driving TestOperatorOutputParity below. facts is
// the set of values the text sentence states that MUST also appear as a
// scalar value somewhere in the json document — the parity claim, made
// checkable rather than asserted by inspection alone.
type operatorParityRow struct {
	name  string
	text  string
	doc   any
	facts []string
}

// operatorParityRows builds one row per operator command, each computed
// from a single hand-built result value run through BOTH the pure text
// formatter and the pure json-doc builder — never two different result
// values for the same row, which is what "the same fact must appear on
// both sides" actually requires.
func operatorParityRows() []operatorParityRow {
	reindexRes := store.ReindexResult{Scanned: 57, Upserted: 23, Skipped: 11, Unchanged: 19}
	pruneBefore := time.Date(2031, 6, 15, 12, 0, 0, 0, time.UTC)
	summarizeRes := store.SummarizeResult{Scanned: 41, Filled: 17, Skipped: 20, Failed: 4}
	spineRes := store.SpineScanResult{
		Total: 9, Owners: 3, WithSummary: 5, WithoutSummary: 4,
		Superseded: 1, Expired: 1, Scheduled: 0, WithCitations: 2, Citations: 6,
	}
	consolidatePairs := []store.DuplicatePair{
		{A: "id-a", B: "id-b", AShortID: "sa", BShortID: "sb", AScope: "s", BScope: "s", Score: 0.5},
	}
	consolidateMinScore := float32(0.5)

	return []operatorParityRow{
		{
			name:  "reindex",
			text:  reindexSummary(reindexRes, "target-coll", 1536, false, false),
			doc:   reindexReportDoc(reindexRes, "target-coll", 1536, false, false),
			facts: []string{"23", "57", "target-coll", "1536"},
		},
		{
			name:  "prune-expired",
			text:  pruneSummary(31, pruneBefore),
			doc:   pruneReportDoc(31, pruneBefore),
			facts: []string{"31"},
		},
		{
			name:  "summarize-missing",
			text:  summarizeSummary(summarizeRes, false),
			doc:   summarizeReportDoc(summarizeRes, false),
			facts: []string{"41", "17", "20", "4"},
		},
		{
			// 04-04-PLAN.md Task 1: backfill-short-ids is now a thin
			// delegating alias sharing migrate's own report envelope
			// (D-11) -- this row is built from the SAME migrateSummary/
			// migrateReportDoc pair the "migrate" row above uses, never a
			// backfill-specific formatter (those are deleted).
			name:  "backfill-short-ids",
			text:  migrateSummary(store.MigrateResult{Migrated: 29, Backlog: 0}, migrate.CurrentVersion, false, 29),
			doc:   migrateReportDoc(store.MigrateResult{Migrated: 29, Backlog: 0}, migrate.CurrentVersion, false, 29),
			facts: []string{"29"},
		},
		{
			name:  "migrate-remap-owner",
			text:  migrateRemapSummary(13, "alice", false),
			doc:   migrateRemapDoc(13, "alice", false),
			facts: []string{"13", "alice"},
		},
		{
			name:  "migrate-set-owner",
			text:  migrateSetOwnerSummary("bob", 7),
			doc:   migrateSetOwnerReportDoc{Owner: "bob", Stamped: 7},
			facts: []string{"bob", "7"},
		},
		{
			name:  "spine-review scan",
			text:  spineScanSummary(spineRes, "s"),
			doc:   spineScanDoc(spineRes),
			facts: []string{"9", "3", "5", "4", "1", "2", "6"},
		},
		{
			name: "spine-review verify",
			text: verifySummary(verifyReport{
				ValidCount: 2, MovedCount: 1, BrokenCount: 1, UnverifiableCount: 1,
				Moved:        []verifyEntry{{RecordID: "rec-moved", ShortID: "short-moved", Ref: "a.go", Reason: "excerpt found at byte offset 12, not at the cited locator"}},
				Broken:       []verifyEntry{{RecordID: "rec-broken", ShortID: "short-broken", Ref: "b.go", Reason: reasonFileMissing}},
				Unverifiable: []verifyEntry{{RecordID: "rec-unverifiable", ShortID: "short-unverifiable", Ref: "c.go", Reason: "different repo"}},
			}),
			doc: verifyDoc(verifyReport{
				ValidCount: 2, MovedCount: 1, BrokenCount: 1, UnverifiableCount: 1,
				Moved:        []verifyEntry{{RecordID: "rec-moved", ShortID: "short-moved", Ref: "a.go", Reason: "excerpt found at byte offset 12, not at the cited locator"}},
				Broken:       []verifyEntry{{RecordID: "rec-broken", ShortID: "short-broken", Ref: "b.go", Reason: reasonFileMissing}},
				Unverifiable: []verifyEntry{{RecordID: "rec-unverifiable", ShortID: "short-unverifiable", Ref: "c.go", Reason: "different repo"}},
			}),
			facts: []string{"2", "1", "rec-moved", "rec-broken", "rec-unverifiable"},
		},
		{
			name:  "spine-review consolidate",
			text:  consolidateSummary(consolidatePairs, "s", false, &consolidateMinScore, 5, 9, 9),
			doc:   consolidateDoc(consolidatePairs, "s", false, &consolidateMinScore, 5, 9, 9),
			facts: []string{"9", "5", "0.5", "id-a", "id-b"},
		},
		{
			name: "spine-review archive",
			text: archiveSummary([]store.ArchiveResult{
				{ID: "id-changed", Outcome: store.ArchiveOutcomeChanged},
				{ID: "id-already", Outcome: store.ArchiveOutcomeAlready},
			}, "archive"),
			doc: archiveDoc([]store.ArchiveResult{
				{ID: "id-changed", Outcome: store.ArchiveOutcomeChanged},
				{ID: "id-already", Outcome: store.ArchiveOutcomeAlready},
			}, "archive"),
			facts: []string{"id-changed", "id-already", "changed", "already"},
		},
		{
			name: "spine-review restore",
			text: archiveSummary([]store.ArchiveResult{
				{ID: "id-restored", Outcome: store.ArchiveOutcomeChanged},
				{ID: "id-unknown", Outcome: store.ArchiveOutcomeNotFound},
			}, "restore"),
			doc: archiveDoc([]store.ArchiveResult{
				{ID: "id-restored", Outcome: store.ArchiveOutcomeChanged},
				{ID: "id-unknown", Outcome: store.ArchiveOutcomeNotFound},
			}, "restore"),
			facts: []string{"id-restored", "id-unknown", "changed", "not_found"},
		},
		{
			name: "spine-review purge",
			text: purgeAppliedSummary(store.PurgeResult{
				Deleted: []string{"id-deleted"}, Spared: []string{"id-spared"}, Appeared: []string{"id-appeared"},
			}, store.PurgeOptions{Classes: []store.PurgeClass{store.PurgeClassExpired}, Scope: "s"}),
			doc: purgeAppliedDoc([]string{"id-deleted", "id-spared"}, store.PurgeResult{
				Deleted: []string{"id-deleted"}, Spared: []string{"id-spared"}, Appeared: []string{"id-appeared"},
			}, store.PurgeOptions{Classes: []store.PurgeClass{store.PurgeClassExpired}, Scope: "s"}),
			facts: []string{"id-deleted", "id-spared", "id-appeared"},
		},
		{
			// 04-03-PLAN.md Task 2: an APPLIED result (dryRun=false) so the
			// "migrated" fact is meaningful; backlog is stated explicitly
			// in migrateSummary's applied-mode sentence for exactly this
			// parity claim.
			name:  "migrate",
			text:  migrateSummary(store.MigrateResult{Migrated: 23, Failed: 2, Passes: 1, Backlog: 5, Spared: []string{"a"}, Appeared: []string{"b", "c"}}, migrate.CurrentVersion, false, 26),
			doc:   migrateReportDoc(store.MigrateResult{Migrated: 23, Failed: 2, Passes: 1, Backlog: 5, Spared: []string{"a"}, Appeared: []string{"b", "c"}}, migrate.CurrentVersion, false, 26),
			facts: []string{"26", "23", "5"},
		},
		{
			name: "migrate status",
			text: statusSummary(store.MigrateStatusResult{
				Buckets: []store.VersionBucket{{Version: 1, Count: 40}}, Absent: 3,
				Future: []store.VersionBucket{{Version: 2, Count: 1}}, FutureTotal: 1, Total: 44,
			}),
			doc: statusReportDoc(store.MigrateStatusResult{
				Buckets: []store.VersionBucket{{Version: 1, Count: 40}}, Absent: 3,
				Future: []store.VersionBucket{{Version: 2, Count: 1}}, FutureTotal: 1, Total: 44,
			}),
			facts: []string{"44", "3", "40", "1", "2"},
		},
		{
			// A REVERSIBLE preview fixture (Task 3): facts name
			// plan.Candidates and the target version, per this row's plan
			// text.
			name:  "migrate revert",
			text:  revertSummary(store.RevertPlan{To: 0, Candidates: 12, Reversible: true}, false, store.RevertResult{}),
			doc:   revertReportDoc(store.RevertPlan{To: 0, Candidates: 12, Reversible: true}, false, store.RevertResult{}),
			facts: []string{"12", "0"},
		},
	}
}

// jsonScalarValues flattens doc's json.Marshal output into every scalar
// (string/number/bool) value reachable anywhere in the document —
// including inside a slice of structs (spineScanReportDoc's
// ByScopeCategory) — as their JSON-rendered string form, so a fact can be
// looked up by exact-element membership rather than substring containment
// (which would trivially "match" any short numeral inside a longer one).
func jsonScalarValues(doc any) ([]string, error) {
	b, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	var out []string
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			for _, vv := range t {
				walk(vv)
			}
		case []any:
			for _, vv := range t {
				walk(vv)
			}
		case float64:
			out = append(out, strconv.FormatFloat(t, 'f', -1, 64))
		case string:
			out = append(out, t)
		case bool:
			out = append(out, strconv.FormatBool(t))
		}
	}
	walk(raw)
	return out, nil
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// TestOperatorOutputParity is the phase's json/text field-for-fact parity
// gate (T-03-09's mitigation, Codex review #3's "known limitation,
// recorded rather than redesigned" — see this plan's SUMMARY): for every
// operator command, the SAME result value drives both the text sentence
// and the json document, and every declared fact from the text side must
// also appear as a scalar value somewhere in the json side.
//
// The row-set itself is gated both directions against operatorCommands():
// a seventh operator command added later without a corresponding row here
// fails this test, rather than silently narrowing the parity claim.
func TestOperatorOutputParity(t *testing.T) {
	rows := operatorParityRows()

	rowNames := make(map[string]bool, len(rows))
	for _, r := range rows {
		rowNames[r.name] = true
	}
	wantNames := commandKeySet(operatorCommands())
	for name := range wantNames {
		if !rowNames[name] {
			t.Errorf("operatorParityRows() is missing a row for operator command %q", name)
		}
	}
	for name := range rowNames {
		if !wantNames[name] {
			t.Errorf("operatorParityRows() has a row for %q, which is not in operatorCommands()", name)
		}
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			values, err := jsonScalarValues(row.doc)
			if err != nil {
				t.Fatalf("jsonScalarValues: %v", err)
			}
			for _, fact := range row.facts {
				if !strings.Contains(row.text, fact) {
					t.Errorf("row %q: text %q does not contain declared fact %q (row is malformed)", row.name, row.text, fact)
				}
				if !containsString(values, fact) {
					t.Errorf("row %q: json document %v does not carry fact %q that the text sentence %q states", row.name, values, fact, row.text)
				}
			}
		})
	}
}

// TestOperatorOutputEncoding proves a scope/tag value carrying a
// non-ASCII rune AND a double quote survives a json.Marshal then
// json.Unmarshal round trip byte-identically — UTF-8 fidelity through the
// operator tier's one json-doc shape that carries free-form scope/category
// text (spineScanBreakdownDoc).
func TestOperatorOutputEncoding(t *testing.T) {
	const tricky = `légïnûs "quoted" scope`
	doc := spineScanBreakdownDoc{Scope: tricky, Category: "note", Count: 1}

	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var got spineScanBreakdownDoc
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got.Scope != tricky {
		t.Errorf("round-tripped Scope = %q, want %q", got.Scope, tricky)
	}
}

// TestOperatorOutputEmpty proves an empty-result invocation of every
// operator command's json-doc builder marshals to zero-valued counters
// and never emits a bare `null` anywhere in the document — a caller
// scripting against the json lane should never have to special-case a
// zero-record sweep.
func TestOperatorOutputEmpty(t *testing.T) {
	docs := map[string]any{
		"reindex":           reindexReportDoc(store.ReindexResult{}, "t", 0, false, false),
		"prune-expired":     pruneReportDoc(0, time.Time{}),
		"summarize-missing": summarizeReportDoc(store.SummarizeResult{}, false),
		// 04-04-PLAN.md Task 1: the deleted backfillReportDoc's entry
		// replaced with the shared migrate zero-value doc -- this step
		// REPLACES the backfill-short-ids entry only; it does NOT add a
		// "migrate status" entry (statusReportDoc's null-safety is gated
		// directly by 04-03 Task 2, not by this map -- REVIEWS.md C6-L4).
		"backfill-short-ids":  migrateReportDoc(store.MigrateResult{}, migrate.CurrentVersion, false, 0),
		"migrate-remap-owner": migrateRemapDoc(0, "", false),
		"migrate-set-owner":   migrateSetOwnerReportDoc{},
		"spine-review scan":   spineScanDoc(store.SpineScanResult{}),
	}
	for name, doc := range docs {
		t.Run(name, func(t *testing.T) {
			b, err := json.Marshal(doc)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			if strings.Contains(string(b), "null") {
				t.Errorf("%s: empty-result json = %s, must not contain \"null\" anywhere", name, b)
			}
		})
	}
}

// TestOperatorOutputStream proves the json branch writes to
// cmd.OutOrStdout() only, and that a warning the command emits on
// cmd.ErrOrStderr() never leaks into that stream — so a piped consumer
// (`engram <cmd> --output json | jq .`) always reads exactly one
// document with no leading or trailing non-JSON bytes.
func TestOperatorOutputStream(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := &cobra.Command{Use: "throwaway"}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	fmt.Fprintln(cmd.ErrOrStderr(), "warning: something noteworthy")

	doc := struct {
		Count int `json:"count"`
	}{Count: 3}
	if err := renderOperator(cmd, formatJSON, "3 record(s)", doc); err != nil {
		t.Fatalf("renderOperator: %v", err)
	}

	dec := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	var got map[string]any
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("decoding stdout as JSON: %v (stdout=%q)", err, stdout.String())
	}
	if dec.More() {
		t.Error("stdout carries more than one JSON document")
	}
	if strings.Contains(stdout.String(), "warning") {
		t.Errorf("warning text leaked into stdout: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "warning") {
		t.Errorf("warning text did not reach stderr: %q", stderr.String())
	}
}

// timeoutGroup names one of the three published --timeout groups
// (docs-site/src/content/docs/guides/cli.md's three-group table) and the
// commandKey set belonging to it.
type timeoutGroup struct {
	name     string
	commands map[string]bool
	// zeroRejected is true when --timeout 0 is a usage error for every
	// command in this group (the two reject groups); false when 0 means
	// "no deadline" (the zero-disables group).
	zeroRejected bool
}

var timeoutGroups = []timeoutGroup{
	{
		name:         "reject-zero-client",
		commands:     map[string]bool{"search": true, "list": true, "store": true},
		zeroRejected: true,
	},
	{
		name: "zero-disables",
		commands: map[string]bool{
			"reindex": true, "prune-expired": true, "summarize-missing": true,
			"backfill-short-ids": true, "spine-review scan": true, "spine-review verify": true,
			"spine-review consolidate": true, "spine-review archive": true, "spine-review restore": true,
			"spine-review purge": true,
			// 04-03-PLAN.md: migrate/migrate status (Task 2) and migrate
			// revert (Task 3) all join this group.
			"migrate": true, "migrate status": true, "migrate revert": true,
		},
		zeroRejected: false,
	},
	{
		name:         "reject-zero-operator",
		commands:     map[string]bool{"migrate-remap-owner": true, "migrate-set-owner": true},
		zeroRejected: true,
	},
}

// timeoutGroupCaseArgs returns the full argv and any extra env this
// command needs to reach its --timeout check with every OTHER required
// flag already satisfied, plus an unreachable Qdrant so a command that
// proceeds PAST the --timeout check fails with a transport error
// (exitUnavailable), never a coincidental usage error from an unrelated
// missing flag or env var.
func timeoutGroupCaseArgs(t *testing.T, name string) (args []string, env map[string]string) {
	t.Helper()
	env = map[string]string{"ENGRAM_QDRANT_ADDR": deadQdrant}
	switch name {
	case "search":
		return []string{"search", "--server", deadServer, "--scope", "s", "--query", "q", "--timeout", "0"}, nil
	case "list":
		return []string{"list", "--server", deadServer, "--scope", "s", "--timeout", "0"}, nil
	case "store":
		return []string{"store", "--server", deadServer, "--scope", "s", "--content", "c", "--timeout", "0"}, nil
	case "reindex":
		return []string{"reindex", "--target", "t", "--timeout", "0"}, env
	case "prune-expired":
		return []string{"prune-expired", "--timeout", "0"}, env
	case "summarize-missing":
		env["ENGRAM_SUMMARY_MODEL"] = "cheap-model"
		return []string{"summarize-missing", "--all-scopes", "--timeout", "0"}, env
	case "backfill-short-ids":
		return []string{"backfill-short-ids", "--timeout", "0"}, env
	case "spine-review scan":
		return []string{"spine-review", "scan", "--all-scopes", "--timeout", "0"}, env
	case "spine-review verify":
		return []string{"spine-review", "verify", "--all-scopes", "--timeout", "0"}, env
	case "spine-review consolidate":
		return []string{"spine-review", "consolidate", "--all-scopes", "--timeout", "0"}, env
	case "spine-review archive":
		return []string{"spine-review", "archive", "--id", "x", "--timeout", "0"}, env
	case "spine-review restore":
		return []string{"spine-review", "restore", "--id", "x", "--timeout", "0"}, env
	case "spine-review purge":
		return []string{"spine-review", "purge", "--class", "expired", "--timeout", "0"}, env
	case "migrate-remap-owner":
		return []string{"migrate-remap-owner", "--from-missing", "--to", "x", "--timeout", "0"}, env
	case "migrate-set-owner":
		return []string{"migrate-set-owner", "--owner", "x", "--timeout", "0"}, env
	case "migrate":
		return []string{"migrate", "--timeout", "0"}, env
	case "migrate status":
		return []string{"migrate", "status", "--timeout", "0"}, env
	case "migrate revert":
		return []string{"migrate", "revert", "--to", "0", "--timeout", "0"}, env
	default:
		t.Fatalf("timeoutGroupCaseArgs: no row defined for command %q", name)
		return nil, nil
	}
}

// timeoutGroupCommand resolves name (a commandKey, possibly "parent leaf"
// for a nested command) to its live *cobra.Command, for
// resetCommandFlagState pairing.
func timeoutGroupCommand(t *testing.T, name string) *cobra.Command {
	t.Helper()
	for _, cmd := range walkCommands(rootCmd, commandWalkSkip) {
		if commandKey(cmd) == name {
			return cmd
		}
	}
	t.Fatalf("timeoutGroupCommand: %q not found in the live tree", name)
	return nil
}

// operatorInvalidOutputArgs returns the minimal argv (name plus any OTHER
// required flag) that reaches this command's own operatorOutputFormat
// call — never a store dial — when "--output" "yaml" is appended.
func operatorInvalidOutputArgs(t *testing.T, name string) []string {
	t.Helper()
	switch name {
	case "reindex":
		return []string{"reindex", "--target", "t"}
	case "prune-expired":
		return []string{"prune-expired"}
	case "summarize-missing":
		return []string{"summarize-missing", "--all-scopes"}
	case "backfill-short-ids":
		return []string{"backfill-short-ids"}
	case "migrate-remap-owner":
		return []string{"migrate-remap-owner", "--from-missing", "--to", "x"}
	case "migrate-set-owner":
		return []string{"migrate-set-owner", "--owner", "x"}
	case "spine-review scan":
		return []string{"spine-review", "scan", "--all-scopes"}
	case "spine-review verify":
		return []string{"spine-review", "verify", "--all-scopes"}
	case "spine-review consolidate":
		return []string{"spine-review", "consolidate", "--all-scopes"}
	case "spine-review archive":
		return []string{"spine-review", "archive", "--id", "x"}
	case "spine-review restore":
		return []string{"spine-review", "restore", "--id", "x"}
	case "spine-review purge":
		return []string{"spine-review", "purge", "--class", "expired"}
	case "migrate":
		return []string{"migrate"}
	case "migrate status":
		return []string{"migrate", "status"}
	case "migrate revert":
		return []string{"migrate", "revert", "--to", "0"}
	default:
		t.Fatalf("operatorInvalidOutputArgs: no row defined for command %q", name)
		return nil
	}
}

// TestEveryOperatorCommandRejectsInvalidOutput is the behavioural
// complement to the `rg outputFormatFromConfig(` count check: it drives
// EVERY command in operatorCommands() with "--output yaml" and asserts
// exitUsage for each, so a command wired past the shared validator fails
// on behaviour, not only on a source grep.
func TestEveryOperatorCommandRejectsInvalidOutput(t *testing.T) {
	for _, cmd := range operatorCommands() {
		name := commandKey(cmd)
		t.Run(name, func(t *testing.T) {
			resetClientFlags(t)
			resetCommandFlagState(t, timeoutGroupCommand(t, name))
			args := append(operatorInvalidOutputArgs(t, name), "--output", "yaml")
			_, _, err := runClient(t, args...)
			if got := exitCodeFromError(err); got != exitUsage {
				t.Errorf("%s --output yaml: exitCodeFromError = %d, want %d (exitUsage); err=%v", name, got, exitUsage, err)
			}
		})
	}
}

// TestTimeoutGroupMatrix pins the PUBLISHED three-group --timeout matrix
// (docs-site/src/content/docs/guides/cli.md's table) behaviourally, not
// by help-text wording: --timeout 0 exits exitUsage for both reject
// groups and does NOT exit exitUsage for the zero-disables group (it
// proceeds to dial and fails with exitUnavailable against an unreachable
// Qdrant/server instead). A second assertion is a set equality: the union
// of the three groups must equal every --timeout-bearing command in the
// live tree, so a new --timeout-bearing command cannot be added to the
// tier without a deliberate group assignment.
//
// MUTATION CHECK, not RED-first (see this plan's SUMMARY): neither this
// test nor the three groups exist in the tree before this task, so the
// mis-grouping failure state cannot arise naturally in task order. This
// plan authors the groups correctly in the same breath as the test.
func TestTimeoutGroupMatrix(t *testing.T) {
	for _, g := range timeoutGroups {
		g := g
		t.Run(g.name, func(t *testing.T) {
			for name := range g.commands {
				name := name
				t.Run(name, func(t *testing.T) {
					resetClientFlags(t)
					resetCommandFlagState(t, timeoutGroupCommand(t, name))
					args, env := timeoutGroupCaseArgs(t, name)
					for k, v := range env {
						t.Setenv(k, v)
					}
					_, _, err := runClient(t, args...)
					got := exitCodeFromError(err)
					if g.zeroRejected {
						if got != exitUsage {
							t.Errorf("%s --timeout 0: exitCodeFromError = %d, want %d (exitUsage, reject-zero group)", name, got, exitUsage)
						}
					} else if got == exitUsage {
						t.Errorf("%s --timeout 0: exitCodeFromError = %d (exitUsage), want anything else (zero-disables group; err=%v)", name, got, err)
					}
				})
			}
		})
	}

	union := make(map[string]bool)
	for _, g := range timeoutGroups {
		for name := range g.commands {
			union[name] = true
		}
	}

	live := make(map[string]bool)
	for _, cmd := range walkCommands(rootCmd, commandWalkSkip) {
		if cmd.Flags().Lookup("timeout") != nil {
			live[commandKey(cmd)] = true
		}
	}

	for name := range union {
		if !live[name] {
			t.Errorf("timeoutGroups names %q, which does not carry a --timeout flag in the live tree", name)
		}
	}
	for name := range live {
		if !union[name] {
			t.Errorf("live command %q carries --timeout but is not assigned to any of the three published groups", name)
		}
	}
}
