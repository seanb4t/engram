// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// cliGuideRelPath is the path to the CLI guide, relative to this package's
// own directory (cmd/engram). Two levels up reaches the repo root
// (cmd/engram -> cmd -> repo root) -- `go test` always runs with the
// package directory as its working directory, so a plain relative read
// resolves correctly with no repo-root lookup needed. Mirrors
// migrateGuideRelPath's idiom (migrate_docs_test.go).
const cliGuideRelPath = "../../docs-site/src/content/docs/guides/cli.md"

// consolidateSectionHeading is the exact markdown heading
// extractConsolidateSection scans for.
const consolidateSectionHeading = "### `spine-review consolidate`"

// nextHeadingPattern matches the start of the next "### " heading —
// extractConsolidateSection's section boundary.
var nextHeadingPattern = regexp.MustCompile(`(?m)^### `)

// extractConsolidateSection returns the consolidate section of doc: from
// consolidateSectionHeading up to (not including) the next "### " heading,
// or "" when the heading itself is not found. A pure function so
// TestConsolidateGuideStatesVerdictContract (against the live guide) and
// TestConsolidateGuideStatesVerdictContractGateFiresOnInjectedViolation
// (against in-memory fixtures) both exercise the SAME extraction, mirroring
// migrateGuidePendingRowViolations' single-implementation discipline
// (migrate_docs_test.go).
func extractConsolidateSection(doc string) string {
	start := strings.Index(doc, consolidateSectionHeading)
	if start == -1 {
		return ""
	}
	rest := doc[start+len(consolidateSectionHeading):]
	if loc := nextHeadingPattern.FindStringIndex(rest); loc != nil {
		return doc[start : start+len(consolidateSectionHeading)+loc[0]]
	}
	return doc[start:]
}

// consolidateGuideRequiredAnchors are the terms
// TestConsolidateGuideStatesVerdictContract requires the consolidate
// section to state (03-06-PLAN.md Task 3): the two verdict flags, the
// three registered env vars, the JSON/text vocabulary, the failure
// taxonomy's state_unavailable class, the advisory framing, the
// never-merges framing, the two scope flags, and all five D-05 relation
// names.
var consolidateGuideRequiredAnchors = []string{
	"--no-verdicts",
	"--verdict-threshold",
	"ENGRAM_DECISIONS_PROVIDER",
	"ENGRAM_DECISIONS_VERDICT_THRESHOLD",
	"ENGRAM_DECISIONS_VERDICT_STATE_CHARS",
	"needs_review",
	"same_subject",
	"verdict unavailable",
	"state_unavailable",
	"advisory",
	"never merges",
	"--scope",
	"--all-scopes",
	"duplicate",
	"contradicts",
	"updates",
	"related",
	"unrelated",
}

// consolidateGuideRetiredPhrase is the empty-result claim plan 03-03
// retired from this same section (03-03-PLAN.md's own negative-grep
// verify) — this gate reuses the EXACT phrase that plan already proved
// absent, so a future edit cannot silently reintroduce it under this
// section without also being caught here.
const consolidateGuideRetiredPhrase = "well-defined empty result"

// missingConsolidateGuideAnchors returns, in consolidateGuideRequiredAnchors
// order, every anchor NOT present in section, plus consolidateGuideRetiredPhrase
// itself when that retired phrase HAS crept back into section. One
// implementation, called by both the live-guide test and its injected-
// violation positive control below.
func missingConsolidateGuideAnchors(section string) []string {
	var missing []string
	for _, anchor := range consolidateGuideRequiredAnchors {
		if !strings.Contains(section, anchor) {
			missing = append(missing, anchor)
		}
	}
	if strings.Contains(section, consolidateGuideRetiredPhrase) {
		missing = append(missing, consolidateGuideRetiredPhrase)
	}
	return missing
}

// TestConsolidateGuideStatesVerdictContract is the docs-completeness gate
// for the advisory verdict contract (D-04..D-11, plan 03-06 Task 3): every
// anchor in consolidateGuideRequiredAnchors must appear inside the LIVE
// consolidate section of the CLI guide, and the retired empty-result
// sentence (03-03-PLAN.md) must not have crept back in. Guarded against a
// green-but-untested pass exactly like TestMigrateGuidePendingRowIsAccurate
// (migrate_docs_test.go): a trimmed checkout skips explicitly rather than
// silently passing having read nothing.
func TestConsolidateGuideStatesVerdictContract(t *testing.T) {
	data, err := os.ReadFile(cliGuideRelPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("CLI guide not present at %s (trimmed checkout?) -- skipping rather than passing silently", cliGuideRelPath)
		}
		t.Fatalf("read %s: %v", cliGuideRelPath, err)
	}
	if len(data) == 0 {
		t.Fatalf("%s is empty", cliGuideRelPath)
	}

	section := extractConsolidateSection(string(data))
	if section == "" {
		t.Fatalf("%s: no %q section found", cliGuideRelPath, consolidateSectionHeading)
	}

	for _, missing := range missingConsolidateGuideAnchors(section) {
		t.Errorf("%s: consolidate section is missing required anchor %q", cliGuideRelPath, missing)
	}
}

// TestConsolidateGuideStatesVerdictContractGateFiresOnInjectedViolation is
// the committed positive control: a clean synthetic section (every anchor
// present) must report no violations; the same section missing one anchor,
// or carrying the retired phrase, must each be reported — proving
// missingConsolidateGuideAnchors can actually fail rather than passing
// vacuously. "--no-verdicts" is the anchor dropped for the missing-anchor
// case specifically because it is not a substring of any OTHER anchor in
// consolidateGuideRequiredAnchors (unlike "related", which is itself a
// substring of "unrelated" and so cannot be dropped in isolation this way).
func TestConsolidateGuideStatesVerdictContractGateFiresOnInjectedViolation(t *testing.T) {
	clean := consolidateSectionHeading + "\n\n" + strings.Join(consolidateGuideRequiredAnchors, " ") + "\n"
	if missing := missingConsolidateGuideAnchors(clean); len(missing) != 0 {
		t.Fatalf("clean fixture (every anchor present) reported missing anchors: %v", missing)
	}

	t.Run("missing one anchor", func(t *testing.T) {
		const dropped = "--no-verdicts"
		var kept []string
		for _, a := range consolidateGuideRequiredAnchors {
			if a != dropped {
				kept = append(kept, a)
			}
		}
		section := consolidateSectionHeading + "\n\n" + strings.Join(kept, " ") + "\n"
		missing := missingConsolidateGuideAnchors(section)
		if !slices.Contains(missing, dropped) {
			t.Errorf("section missing %q reported missing = %v, want it to include %q", dropped, missing, dropped)
		}
	})

	t.Run("retired phrase present", func(t *testing.T) {
		section := clean + consolidateGuideRetiredPhrase
		missing := missingConsolidateGuideAnchors(section)
		if !slices.Contains(missing, consolidateGuideRetiredPhrase) {
			t.Errorf("section carrying the retired phrase reported missing = %v, want it to include %q", missing, consolidateGuideRetiredPhrase)
		}
	})
}
