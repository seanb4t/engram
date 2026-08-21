// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sort"
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

// operatorViewFixtures merges every group's fixture function into one map,
// keyed by commandKey exactly as operatorCommands() produces it: the
// complete view-fixture universe TestOperatorViewFixturesCoverEveryOperatorCommand
// checks against operatorCommands() in both directions, and the set every
// document TestOperatorViewIdentityAcrossEveryOperatorCommand walks.
//
// A key returned by more than one group function is a build-time defect —
// two plans both claimed the same command — and this panics loudly rather
// than silently letting one group's fixtures overwrite another's.
func operatorViewFixtures() map[string][]any {
	groups := []map[string][]any{
		pruneViewFixtures(),
		flatViewFixtures(),
		migrateViewFixtures(),
		archivePurgeViewFixtures(),
		spineViewFixtures(),
	}
	merged := make(map[string][]any)
	for _, group := range groups {
		for key, docs := range group {
			if _, exists := merged[key]; exists {
				panic(fmt.Sprintf("operatorViewFixtures: %q is claimed by more than one fixture group", key))
			}
			merged[key] = docs
		}
	}
	return merged
}

// setDiff returns, sorted, the keys present in want but not got (missing)
// and the keys present in got but not want (extra). Pure and independent
// of any command or fixture — TestSetDiffDetectsDivergence exercises it
// directly, without touching operatorCommands() or operatorViewFixtures(),
// so the enumeration gate's non-vacuity proof cannot be satisfied by the
// same data the gate itself reads.
func setDiff(want, got map[string]bool) (missing, extra []string) {
	for key := range want {
		if !got[key] {
			missing = append(missing, key)
		}
	}
	for key := range got {
		if !want[key] {
			extra = append(extra, key)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return missing, extra
}

// TestOperatorViewFixturesCoverEveryOperatorCommand is the inheritor of
// the retired parity gate's one good property (06-CONTEXT.md D-09): it
// gates the merged view-fixture set against operatorCommands() in BOTH
// directions, computing its expectation from the live cobra tree — never
// a string literal list. Deriving the expectation from the tree, rather
// than transcribing it, is what caught backfill-short-ids's unregistered
// preview variant that a hand-written variant list had missed once
// (06-CONTEXT.md <specifics>). Every fixture entry's document slice must
// also be non-empty, so a key registered with zero documents cannot
// satisfy this gate vacuously.
func TestOperatorViewFixturesCoverEveryOperatorCommand(t *testing.T) {
	want := commandKeySet(operatorCommands())

	fixtures := operatorViewFixtures()
	got := make(map[string]bool, len(fixtures))
	for key := range fixtures {
		got[key] = true
	}

	missing, extra := setDiff(want, got)
	for _, key := range missing {
		t.Errorf("operator command %q has no entry in operatorViewFixtures()", key)
	}
	for _, key := range extra {
		t.Errorf("operatorViewFixtures() has an entry keyed %q, which is not in operatorCommands()", key)
	}

	for key, docs := range fixtures {
		if len(docs) == 0 {
			t.Errorf("operatorViewFixtures()[%q] is empty — a key registered with zero documents cannot satisfy this gate vacuously", key)
		}
	}
}

// TestSetDiffDetectsDivergence is the committed non-vacuity proof for
// TestOperatorViewFixturesCoverEveryOperatorCommand's enumeration gate.
// The red-evidence patch harness is deferred for this phase
// (06-CONTEXT.md <deferred>) and unavailable, so this table-driven test
// over setDiff alone — deliberately independent of operatorCommands() and
// operatorViewFixtures() — is the committed proof the gate can actually
// fail: a key in want only, a key in got only, disjoint sets, identical
// sets, and empty inputs each produce the expected missing/extra pair.
func TestSetDiffDetectsDivergence(t *testing.T) {
	cases := []struct {
		name                   string
		want, got              map[string]bool
		wantMissing, wantExtra []string
	}{
		{
			name:        "key in want only",
			want:        map[string]bool{"a": true},
			got:         map[string]bool{},
			wantMissing: []string{"a"},
		},
		{
			name:      "key in got only",
			want:      map[string]bool{},
			got:       map[string]bool{"a": true},
			wantExtra: []string{"a"},
		},
		{
			name:        "disjoint sets",
			want:        map[string]bool{"a": true},
			got:         map[string]bool{"b": true},
			wantMissing: []string{"a"},
			wantExtra:   []string{"b"},
		},
		{
			name: "identical sets",
			want: map[string]bool{"a": true, "b": true},
			got:  map[string]bool{"a": true, "b": true},
		},
		{
			name: "empty inputs",
			want: map[string]bool{},
			got:  map[string]bool{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			missing, extra := setDiff(tc.want, tc.got)
			if !slices.Equal(missing, tc.wantMissing) {
				t.Errorf("setDiff(%v, %v) missing = %v, want %v", tc.want, tc.got, missing, tc.wantMissing)
			}
			if !slices.Equal(extra, tc.wantExtra) {
				t.Errorf("setDiff(%v, %v) extra = %v, want %v", tc.want, tc.got, extra, tc.wantExtra)
			}
		})
	}
}

// TestOperatorViewIdentityAcrossEveryOperatorCommand runs the shared
// identity gate (assertViewIdentity, operator_view_test.go) over the
// COMPLETE merged fixture set, one subtest per operator command (never per
// document — a command with several document variants is asserted inside
// a single subtest so `go test -v` prints exactly one PASS line per
// command), proving the phase converted the complete set rather than only
// each group in isolation.
func TestOperatorViewIdentityAcrossEveryOperatorCommand(t *testing.T) {
	fixtures := operatorViewFixtures()
	names := make([]string, 0, len(fixtures))
	for name := range fixtures {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			for i, doc := range fixtures[name] {
				assertViewIdentity(t, fmt.Sprintf("%s/%d", name, i), doc)
			}
		})
	}
}

// operatorDocsAreHandDeclaredMarker exists only so
// TestOperatorDocsAreHandDeclared can derive this package's own import
// path via reflection, rather than hardcoding a string literal that would
// silently go stale on a module rename.
type operatorDocsAreHandDeclaredMarker struct{}

// TestOperatorDocsAreHandDeclared is the tier-wide statement of the
// property archiveReportDoc's doc comment asserts per report
// (spine_review_archive.go: "so this exclusion is enforced by the type
// itself"): every operator document is a struct hand-declared in THIS
// package, never an embedded internal/store result type, so record
// content is unreachable by construction (threat T-06-01). Under D-01
// that bound now covers the text lane as well as the json lane, since
// both derive from the same value.
//
// A store.VersionBucket used as an ELEMENT type inside
// migrateStatusReportDoc's Buckets/Future slice fields is permitted and
// does NOT trip this test: it is a two-scalar value type carrying no
// record content, and it is a field's element type rather than an
// embedded struct that would promote unknown fields into the document.
func TestOperatorDocsAreHandDeclared(t *testing.T) {
	thisPkgPath := reflect.TypeOf(operatorDocsAreHandDeclaredMarker{}).PkgPath()
	const storePkgPrefix = "github.com/seanb4t/engram/internal/store"

	fixtures := operatorViewFixtures()
	names := make([]string, 0, len(fixtures))
	for name := range fixtures {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			for i, doc := range fixtures[name] {
				typ := reflect.TypeOf(doc)
				if typ.Kind() == reflect.Pointer {
					typ = typ.Elem()
				}
				if typ.Kind() != reflect.Struct {
					t.Fatalf("%s/%d: doc has kind %s, want a struct", name, i, typ.Kind())
				}
				if got := typ.PkgPath(); got != thisPkgPath {
					t.Errorf("%s/%d: doc type %s is declared in package %q, want %q (every operator document must be hand-declared in this package)", name, i, typ.Name(), got, thisPkgPath)
				}
				for f := 0; f < typ.NumField(); f++ {
					field := typ.Field(f)
					if !field.Anonymous {
						continue
					}
					fieldType := field.Type
					if fieldType.Kind() == reflect.Pointer {
						fieldType = fieldType.Elem()
					}
					if strings.HasPrefix(fieldType.PkgPath(), storePkgPrefix) {
						t.Errorf("%s/%d: doc type %s embeds anonymous field %s (package %q), an internal/store type — record content becomes reachable through field promotion", name, i, typ.Name(), fieldType.Name(), fieldType.PkgPath())
					}
				}
			}
		})
	}
}

// TestOperatorViewFixturesHaveNoUnsanitizedNesting is the WR-02
// (06-REVIEW.md) regression guard: sanitizeViewValue's control-character
// stripping — the mitigation for T-06-03 — only ever reaches a top-level
// scalar string field (viewFields's default case) or a row-level scalar
// field rendered by viewRow's own key=value walk. viewScalar's kind switch
// (operator_view.go) recognizes only a JSON string and JSON null; every
// other shape, including a nested array or object TWO levels deep from the
// doc root (an array-of-arrays element, or a row-level object/array
// field), falls through to `return string(raw)` verbatim and UNSANITIZED.
//
// No operator report struct produces such a shape today (confirmed via `rg
// '\[\]\[\]|map\[string\]' cmd/engram/*.go` over non-test files, and
// re-proven here structurally over the live fixture set rather than by
// grep alone), so this test passes today and is designed to fail LOUDLY —
// not silently reintroduce the T-06-03 gap — the day a future report field
// crosses that boundary. This is deliberately a test-only guard, not a
// production-code change: WR-02's suggested exhaustive viewScalar rewrite
// is out of scope here because no live doc needs it yet, and rewriting
// rendering behavior for a shape nothing produces risks changing
// `--output text` for no live benefit.
func TestOperatorViewFixturesHaveNoUnsanitizedNesting(t *testing.T) {
	fixtures := operatorViewFixtures()
	names := make([]string, 0, len(fixtures))
	for name := range fixtures {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			for i, doc := range fixtures[name] {
				b, err := json.Marshal(doc)
				if err != nil {
					t.Fatalf("%s/%d: json.Marshal: %v", name, i, err)
				}
				assertNoTwoLevelContainerNesting(t, fmt.Sprintf("%s/%d", name, i), b)
			}
		})
	}
}

// assertNoTwoLevelContainerNesting walks a marshaled operator doc's
// top-level object exactly as viewFields does, and fails if any array
// element or row-level field is itself a JSON array or object — the shape
// viewScalar renders verbatim, unsanitized, because its kind switch only
// recognizes JSON string and null.
func assertNoTwoLevelContainerNesting(t *testing.T, label string, docJSON []byte) {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(docJSON, &top); err != nil {
		// A non-object top-level document (errViewNotObject's case) has no
		// field table to walk.
		return
	}
	for key, raw := range top {
		switch valueKind(raw) {
		case '[':
			var elems []json.RawMessage
			if err := json.Unmarshal(raw, &elems); err != nil {
				t.Fatalf("%s: field %q: json.Unmarshal array: %v", label, key, err)
			}
			for i, elem := range elems {
				switch valueKind(elem) {
				case '{':
					assertRowHasNoContainerFields(t, label, key, elem)
				case '[':
					t.Errorf("%s: field %q element %d is a nested array — sanitizeViewValue's guarantee does not reach this shape (WR-02, 06-REVIEW.md); either give viewScalar an exhaustive kind switch or keep this field one level deep", label, key, i)
				}
			}
		case '{':
			assertRowHasNoContainerFields(t, label, key, raw)
		}
	}
}

// assertRowHasNoContainerFields fails if any key inside a rendered row
// (viewRow's key=value walk) is itself a JSON array or object — see
// TestOperatorViewFixturesHaveNoUnsanitizedNesting.
func assertRowHasNoContainerFields(t *testing.T, label, fieldKey string, raw json.RawMessage) {
	t.Helper()
	var row map[string]json.RawMessage
	if err := json.Unmarshal(raw, &row); err != nil {
		t.Fatalf("%s: field %q: json.Unmarshal row: %v", label, fieldKey, err)
	}
	for rowKey, val := range row {
		if kind := valueKind(val); kind == '[' || kind == '{' {
			t.Errorf("%s: field %q row key %q is a nested %c — sanitizeViewValue's guarantee does not reach this shape (WR-02, 06-REVIEW.md); either give viewScalar an exhaustive kind switch or keep row fields scalar", label, fieldKey, rowKey, kind)
		}
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
		commands:     map[string]bool{"search": true, "list": true, "store": true, "get": true},
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
	case "get":
		return []string{"get", "some-id", "--server", deadServer, "--timeout", "0"}, nil
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
