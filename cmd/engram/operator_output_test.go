// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// The phase's prior json/text parity gate and its hand-built per-command
// row table were RETIRED by 06-CONTEXT.md D-09 (plan 06-07): retirement is
// obsolescence, not supersession by a stronger assertion, because under
// D-01 both --output text and --output json now derive from ONE
// serialization (the report's hand-declared struct) and there is no
// text/json divergence left for a parity gate to detect.
//
// Two things about the retired gate are recorded here rather than lost
// (durable record b3wd4wwwda): its declared per-row string list was
// hand-listed — precisely the "test over hand-built rows" ROADMAP Success
// Criterion 1 rejects — and it was one-directional, asserting every
// declared text value appeared in the json document but never that the
// json document failed to widen past the text.
//
// Its one genuinely good property — gating its row set against
// operatorCommands() in BOTH directions — is carried forward onto the
// merged view-fixture map by the coverage test below (Task 2 of this
// plan), which is its inheritor.

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
		"spine-review scan":   spineScanDoc(store.SpineScanResult{}, ""),
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
