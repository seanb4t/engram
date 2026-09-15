// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// agentSetupGuideRelPath is the path to the agent-setup guide, relative to
// this package's own directory (cmd/engram). Two levels up reaches the repo
// root (cmd/engram -> cmd -> repo root) -- `go test` always runs with the
// package directory as its working directory, so a plain relative read
// resolves correctly with no repo-root lookup needed. Mirrors
// migrateGuideRelPath's idiom in migrate_docs_test.go.
const agentSetupGuideRelPath = "../../docs-site/src/content/docs/guides/agent-setup.md"

// newline is a raw literal newline character, spelled without a backslash
// escape sequence -- this file follows the same bracket-class-only, no-
// backslash discipline as its two regex patterns below.
var newline = `
`

// preservedRowPattern matches the single markdown table row beginning with
// the backticked cell "`preserved`", capturing the whole line so callers
// can inspect its content for the required Codex whole-entry, never-merge,
// and never-shown clauses.
var preservedRowPattern = regexp.MustCompile("(?m)^[|][ ]*`preserved`[ ]*[|].*[|][ ]*$")

// wouldWriteRowPattern matches the single markdown table row beginning with
// the backticked cell "`would-write`", capturing the whole line so callers
// can inspect its content for the required facets/drift facet-naming
// sentence (REQ-drift-facet-naming).
var wouldWriteRowPattern = regexp.MustCompile("(?m)^[|][ ]*`would-write`[ ]*[|].*[|][ ]*$")

// agentSetupGuideDriftViolations checks doc for the seven ways the drift
// detection documentation (the `preserved` row, the `would-write` row, the
// opencode-not-compared statement, and the registered/facets/drift JSON
// field sentence) can be wrong or silently deleted. It returns one error
// per violation so a caller can report every problem in a single run
// rather than stopping at the first. Both
// TestAgentSetupGuideDocumentsDrift (against the live guide) and
// TestAgentSetupGuideDriftGateFiresOnInjectedViolation (against in-memory
// fixtures) call this same function, so the positive control exercises the
// real gate rather than a parallel reimplementation.
func agentSetupGuideDriftViolations(doc string) []error {
	var errs []error

	preservedRow := preservedRowPattern.FindString(doc)
	if preservedRow == "" {
		// Leg 1: the `preserved` row must exist as a row of the results
		// table -- it must never be silently deleted to dodge the other
		// legs below.
		errs = append(errs, fmt.Errorf("%s: no `preserved` table row found -- the row documenting the preserved outcome must be present", agentSetupGuideRelPath))
	} else {
		// Leg 2: Codex's whole-entry replace-on-write semantics
		// (REQ-drift-preserved-outcome).
		if !strings.Contains(preservedRow, "whole entry") {
			errs = append(errs, fmt.Errorf("%s: `preserved` row does not state the Codex whole-entry sentence", agentSetupGuideRelPath))
		}
		// Leg 3: the never-merge consequence of the whole-entry semantics.
		if !strings.Contains(preservedRow, "never merge") {
			errs = append(errs, fmt.Errorf("%s: `preserved` row does not state that a later `--apply` will never merge into the entry", agentSetupGuideRelPath))
		}
		// Leg 4: header values read from a runtime are never shown
		// (privacy prohibition, D-02).
		if !strings.Contains(preservedRow, "never shown") {
			errs = append(errs, fmt.Errorf("%s: `preserved` row does not state that observed header values are never shown", agentSetupGuideRelPath))
		}
	}

	// Leg 5: the `would-write` row must exist and name `facets`
	// (REQ-drift-facet-naming).
	wouldWriteRow := wouldWriteRowPattern.FindString(doc)
	if wouldWriteRow == "" {
		errs = append(errs, fmt.Errorf("%s: no `would-write` table row found", agentSetupGuideRelPath))
	} else if !strings.Contains(wouldWriteRow, "facets") {
		errs = append(errs, fmt.Errorf("%s: `would-write` row does not name `facets`", agentSetupGuideRelPath))
	}

	// Leg 6: opencode is explicitly documented as not compared (D-10) --
	// require both tokens on the SAME line so the statement cannot be
	// satisfied by two unrelated sentences that merely both mention
	// opencode and "not compared" elsewhere in the document.
	opencodeStated := false
	for _, line := range strings.Split(doc, newline) {
		if strings.Contains(line, "opencode") && strings.Contains(line, "not compared") {
			opencodeStated = true
			break
		}
	}
	if !opencodeStated {
		errs = append(errs, fmt.Errorf("%s: no line states that opencode's registration is not compared", agentSetupGuideRelPath))
	}

	// Leg 7: the Scripts-and-JSON section names all three JSON fields
	// (`registered`, `facets`, `drift`) on one line (REQ-drift-facet-naming,
	// SC4).
	jsonFieldsStated := false
	for _, line := range strings.Split(doc, newline) {
		if strings.Contains(line, "registered") && strings.Contains(line, "facets") && strings.Contains(line, "drift") {
			jsonFieldsStated = true
			break
		}
	}
	if !jsonFieldsStated {
		errs = append(errs, fmt.Errorf("%s: no line names all of `registered`, `facets`, and `drift`", agentSetupGuideRelPath))
	}

	return errs
}

// TestAgentSetupGuideDocumentsDrift is the closing documentation gate for
// plan 04-03: it asserts the live guide documents the `preserved` outcome
// (with the Codex whole-entry, never-merge, and never-shown clauses), the
// `would-write` row's facet naming, the opencode-not-compared statement,
// and the registered/facets/drift JSON field names.
//
// Guarded against a green-but-untested pass: if the guide is absent (a
// trimmed checkout), this test skips explicitly rather than silently
// passing having read nothing; it also fails outright if the file content
// is empty.
func TestAgentSetupGuideDocumentsDrift(t *testing.T) {
	data, err := os.ReadFile(agentSetupGuideRelPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("agent-setup guide not present at %s (trimmed checkout?) -- skipping rather than passing silently", agentSetupGuideRelPath)
		}
		t.Fatalf("read %s: %v", agentSetupGuideRelPath, err)
	}
	if len(data) == 0 {
		t.Fatalf("%s is empty", agentSetupGuideRelPath)
	}

	for _, err := range agentSetupGuideDriftViolations(string(data)) {
		t.Error(err)
	}
}

// TestAgentSetupGuideDriftGateFiresOnInjectedViolation is the positive
// control: it proves agentSetupGuideDriftViolations discriminates a clean
// fixture from each of the seven ways this plan's documentation could go
// wrong, entirely over in-memory strings so no file is ever mutated. The
// The clean case is as load-bearing as the violation cases -- a gate that
// fires on everything discriminates nothing.
func TestAgentSetupGuideDriftGateFiresOnInjectedViolation(t *testing.T) {
	preservedRowLine := "| `preserved` | The existing registration carries something setup did not author and cannot reproduce, so setup leaves it untouched; header values read from a runtime are never shown. Claude Code and Codex both replace the whole entry on write (no partial merge): a later `--apply` either overwrites the entry or leaves it untouched and will never merge into it. |"
	wouldWriteRowLine := "| `would-write` | Proposed changes are shown. `facets` names what differs and `drift` details each difference. |"
	opencodeLine := "opencode's registration is not compared: its `mcp list` prints a table setup does not parse."
	jsonFieldsLine := "Each present runtime's JSON row carries `registered`, `facets`, and `drift` fields."
	cleanFixture := strings.Join([]string{preservedRowLine, wouldWriteRowLine, opencodeLine, jsonFieldsLine}, newline) + newline

	cases := []struct {
		name            string
		fixture         string
		expectViolation bool
	}{
		{"clean", cleanFixture, false},
		{"row_deleted", strings.Join([]string{wouldWriteRowLine, opencodeLine, jsonFieldsLine}, newline) + newline, true},
		{"whole_entry_sentence_missing", strings.Join([]string{strings.Replace(preservedRowLine, "whole entry", "the entry", 1), wouldWriteRowLine, opencodeLine, jsonFieldsLine}, newline) + newline, true},
		{"never_merge_missing", strings.Join([]string{strings.Replace(preservedRowLine, "never merge", "not merge", 1), wouldWriteRowLine, opencodeLine, jsonFieldsLine}, newline) + newline, true},
		{"never_shown_missing", strings.Join([]string{strings.Replace(preservedRowLine, "never shown", "redacted", 1), wouldWriteRowLine, opencodeLine, jsonFieldsLine}, newline) + newline, true},
		{"would_write_omits_facets", strings.Join([]string{preservedRowLine, "| `would-write` | Proposed changes are shown. |", opencodeLine, jsonFieldsLine}, newline) + newline, true},
		{"opencode_claimed_compared", strings.Join([]string{preservedRowLine, wouldWriteRowLine, jsonFieldsLine}, newline) + newline, true},
		{"json_fields_missing", strings.Join([]string{preservedRowLine, wouldWriteRowLine, opencodeLine}, newline) + newline, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			violations := agentSetupGuideDriftViolations(c.fixture)
			if got := len(violations) > 0; got != c.expectViolation {
				t.Errorf("fixture %q: violations=%v (count %d), want expectViolation=%v", c.name, violations, len(violations), c.expectViolation)
			}
		})
	}
}
