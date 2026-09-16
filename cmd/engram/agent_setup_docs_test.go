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

// alreadyCorrectRowPattern matches the single markdown table row beginning
// with the backticked cell "`already-correct`", capturing the whole line so
// callers can inspect its content for the corrected pre-write guarantee
// (05-CONTEXT.md D-01) -- bracket classes only, no backslash escape, the
// same discipline preservedRowPattern/wouldWriteRowPattern already follow.
var alreadyCorrectRowPattern = regexp.MustCompile("(?m)^[|][ ]*`already-correct`[ ]*[|].*[|][ ]*$")

// agentSetupGuideStaleClaimAnchors are the two sentences 05-CONTEXT.md D-01
// made false: the `already-correct` row's stale second sentence claiming a
// write may still have run, and the repeat-safety paragraph's "but it may
// perform writes again" clause. Both must occur zero times in the guide
// once the apply-time preserve gate is documented correctly -- the
// migrateGuideStaleClaimAnchors idiom (migrate_docs_test.go), gated on
// ZERO, never a conversion count.
var agentSetupGuideStaleClaimAnchors = []string{
	"does not guarantee that no write",
	"but it may perform writes again",
}

// agentSetupGuideDriftViolations checks doc for the thirteen ways the
// drift detection and apply-time preserve gate documentation (the
// `preserved` row, the `would-write` row, the `already-correct` row, the
// opencode-not-compared statement, the registered/facets/drift JSON field
// sentence, the two stale apply-write claims, the OAuth re-login
// consequence, and the plugin-first delivery statement) can be wrong or
// silently deleted. It returns one error per violation so a caller can
// report every problem in a single run rather than stopping at the first.
// Both TestAgentSetupGuideDocumentsDrift (against the live guide) and
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
		// Leg 10: --apply runs no registration command against a preserved
		// registration and leaves it untouched (05-CONTEXT.md D-01) -- the
		// corrected replacement for the row's former "either overwrites or
		// leaves it untouched" claim.
		if !strings.Contains(preservedRow, "no registration command") || !strings.Contains(preservedRow, "untouched") {
			errs = append(errs, fmt.Errorf("%s: `preserved` row does not state that --apply runs no registration command and leaves the entry untouched", agentSetupGuideRelPath))
		}
		// Leg 11: the row names the manual remediation for both runtimes
		// (05-CONTEXT.md D-05) -- Claude Code's own `mcp remove` command and
		// Codex's `[mcp_servers.engram]` config.toml table.
		if !strings.Contains(preservedRow, "claude mcp remove engram --scope user") || !strings.Contains(preservedRow, "[mcp_servers.engram]") {
			errs = append(errs, fmt.Errorf("%s: `preserved` row does not name the manual remediation for claude-code (`claude mcp remove engram --scope user`) and codex (`[mcp_servers.engram]`)", agentSetupGuideRelPath))
		}
	}

	// Leg 8: zero remaining occurrences of either stale apply-write claim
	// anchor (05-CONTEXT.md D-01) -- gated on ZERO, never a conversion
	// count, the migrateGuideStaleClaimAnchors idiom.
	for _, anchor := range agentSetupGuideStaleClaimAnchors {
		if n := strings.Count(doc, anchor); n != 0 {
			errs = append(errs, fmt.Errorf("%s: stale claim anchor %q still present (%d occurrence(s)) -- the apply-time preserve gate makes this claim false", agentSetupGuideRelPath, anchor, n))
		}
	}

	// Leg 9: the `already-correct` row must exist and state that --apply
	// makes the same comparison before writing and runs no registration
	// command (05-CONTEXT.md D-01) -- the row now describes a PRE-write
	// guarantee, not merely an observed-state claim.
	alreadyCorrectRow := alreadyCorrectRowPattern.FindString(doc)
	if alreadyCorrectRow == "" || !strings.Contains(alreadyCorrectRow, "before writing") || !strings.Contains(alreadyCorrectRow, "no registration command") {
		errs = append(errs, fmt.Errorf("%s: `already-correct` row missing or does not state that --apply compares before writing and runs no registration command", agentSetupGuideRelPath))
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

	// Leg 12: the OAuth re-login consequence (05-CONTEXT.md D-03/D-04) is
	// stated on one line naming `OAuth`, `log in again`, and the `notes`
	// field it rides on.
	oauthStated := false
	for _, line := range strings.Split(doc, newline) {
		if strings.Contains(line, "OAuth") && strings.Contains(line, "log in again") && strings.Contains(line, "notes") {
			oauthStated = true
			break
		}
	}
	if !oauthStated {
		errs = append(errs, fmt.Errorf("%s: no line names `OAuth`, `log in again`, and `notes` together", agentSetupGuideRelPath))
	}

	// Leg 13: plugin-first delivery (05-CONTEXT.md D-07) names both
	// engram's own marketplace and plugin identifiers on one line.
	pluginFirstStated := false
	for _, line := range strings.Split(doc, newline) {
		if strings.Contains(line, "seanb4t/engram") && strings.Contains(line, "engram@engram") {
			pluginFirstStated = true
			break
		}
	}
	if !pluginFirstStated {
		errs = append(errs, fmt.Errorf("%s: no line names both `seanb4t/engram` and `engram@engram`", agentSetupGuideRelPath))
	}

	return errs
}

// TestAgentSetupGuideDocumentsDrift is the closing documentation gate for
// plan 04-03, extended by plan 05-02 for the apply-time preserve gate: it
// asserts the live guide documents the `preserved` outcome (with the
// Codex whole-entry, never-merge, never-shown, no-registration-command,
// and manual-remediation clauses), the `already-correct` row's corrected
// pre-write guarantee, the `would-write` row's facet naming, the
// opencode-not-compared statement, the registered/facets/drift JSON field
// names, the OAuth re-login consequence, and plugin-first delivery -- and
// that the two claims the apply-time preserve gate made false are gone.
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
// fixture from each of the thirteen ways this plan's documentation could
// go wrong, entirely over in-memory strings so no file is ever mutated.
// The clean case is as load-bearing as the violation cases -- a gate that
// fires on everything discriminates nothing.
func TestAgentSetupGuideDriftGateFiresOnInjectedViolation(t *testing.T) {
	// preservedRowLine is the CORRECTED row text (05-02, D-01/D-05): it
	// keeps the pre-existing whole-entry/never-merge/never-shown clauses
	// AND gains the no-registration-command/untouched claim plus both
	// runtimes' manual remediation.
	preservedRowLine := "| `preserved` | The existing registration carries something setup did not author and cannot reproduce, so setup leaves it untouched; header values read from a runtime are never shown. Claude Code and Codex both replace the whole entry on write (no partial merge), so `--apply` leaves a `preserved` registration untouched: it runs no registration command, not even Claude Code's `mcp remove`, and will never merge into it. To replace it yourself, clear it with the runtime's own tool (`claude mcp remove engram --scope user`; for Codex, delete the `[mcp_servers.engram]` table from `config.toml`), then run setup again. |"
	wouldWriteRowLine := "| `would-write` | Proposed changes are shown. `facets` names what differs and `drift` details each difference. |"
	// alreadyCorrectRowLine (05-02, D-01) is the corrected pre-write
	// guarantee -- the replacement for the stale "does not guarantee that
	// no write commands ran" sentence.
	alreadyCorrectRowLine := "| `already-correct` | The registration read through the runtime's own CLI matches the requested URL, auth mode, and header names and references — a real comparison, not a guess. For Claude Code and Codex, `--apply` makes the same comparison before writing and runs no registration command when the row is already-correct; opencode is not compared, so its `--apply` writes and then reads back. |"
	opencodeLine := "opencode's registration is not compared: its `mcp list` prints a table setup does not parse."
	jsonFieldsLine := "Each present runtime's JSON row carries `registered`, `facets`, and `drift` fields."
	// oauthLine (05-02, D-03/D-04) states the OAuth re-login consequence.
	oauthLine := "On Claude Code, a `would-write` row rewritten via OAuth re-registration states in its `notes` field that you will need to log in again after `--apply`."
	// pluginFirstLine (05-02, D-07) states plugin-first delivery.
	pluginFirstLine := "A Claude Code or Codex whose plugin CLI works receives skills through engram's own marketplace and plugin only (`seanb4t/engram`, `engram@engram`)."

	cleanLines := []string{preservedRowLine, wouldWriteRowLine, alreadyCorrectRowLine, opencodeLine, jsonFieldsLine, oauthLine, pluginFirstLine}
	cleanFixture := strings.Join(cleanLines, newline) + newline

	// withoutLine returns cleanLines with removed omitted, preserving the
	// relative order of the rest.
	withoutLine := func(omitted string) string {
		var kept []string
		for _, l := range cleanLines {
			if l != omitted {
				kept = append(kept, l)
			}
		}
		return strings.Join(kept, newline) + newline
	}

	// withReplacedLine returns cleanLines with orig replaced by replacement.
	withReplacedLine := func(orig, replacement string) string {
		var out []string
		for _, l := range cleanLines {
			if l == orig {
				out = append(out, replacement)
			} else {
				out = append(out, l)
			}
		}
		return strings.Join(out, newline) + newline
	}

	cases := []struct {
		name            string
		fixture         string
		expectViolation bool
	}{
		{"clean", cleanFixture, false},
		{"row_deleted", withoutLine(preservedRowLine), true},
		{"whole_entry_sentence_missing", withReplacedLine(preservedRowLine, strings.Replace(preservedRowLine, "whole entry", "the entry", 1)), true},
		{"never_merge_missing", withReplacedLine(preservedRowLine, strings.Replace(preservedRowLine, "never merge", "not merge", 1)), true},
		{"never_shown_missing", withReplacedLine(preservedRowLine, strings.Replace(preservedRowLine, "never shown", "redacted", 1)), true},
		{"would_write_omits_facets", withReplacedLine(wouldWriteRowLine, "| `would-write` | Proposed changes are shown. |"), true},
		// opencode_claimed_compared strips EVERY "not compared" occurrence,
		// not just the dedicated opencodeLine: the already-correct row
		// (leg 9) also states "opencode is not compared" as part of its own
		// pre-write guarantee, so omitting only opencodeLine leaves leg 6
		// accidentally still satisfied by that second, independent mention
		// (the same isolation fix 05-03's plugin_docs_test.go needed for
		// its own agent_setup_link_missing case).
		{"opencode_claimed_compared", strings.ReplaceAll(cleanFixture, "not compared", "compared successfully"), true},
		{"json_fields_missing", withoutLine(jsonFieldsLine), true},
		{"stale_guarantee_sentence_present", cleanFixture + "This row does not guarantee that no write commands ran." + newline, true},
		{"stale_writes_again_sentence_present", cleanFixture + "Repeating setup converges, but it may perform writes again." + newline, true},
		{"already_correct_row_deleted", withoutLine(alreadyCorrectRowLine), true},
		{"already_correct_omits_before_writing", withReplacedLine(alreadyCorrectRowLine, strings.Replace(alreadyCorrectRowLine, "before writing", "before applying", 1)), true},
		{"preserved_omits_untouched", withReplacedLine(preservedRowLine, strings.ReplaceAll(preservedRowLine, "untouched", "left alone")), true},
		{"preserved_omits_claude_remediation", withReplacedLine(preservedRowLine, strings.Replace(preservedRowLine, "claude mcp remove engram --scope user", "the claude-code CLI", 1)), true},
		{"preserved_omits_codex_table", withReplacedLine(preservedRowLine, strings.Replace(preservedRowLine, "[mcp_servers.engram]", "its config table", 1)), true},
		{"oauth_note_missing", withoutLine(oauthLine), true},
		{"plugin_first_missing", withoutLine(pluginFirstLine), true},
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
