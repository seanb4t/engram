// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// upgradeGuideRelPath is the path to the upgrade guide, relative to this
// package's own directory (cmd/engram). Two levels up reaches the repo
// root (cmd/engram -> cmd -> repo root) -- `go test` always runs with the
// package directory as its working directory, so a plain relative read
// resolves correctly with no repo-root lookup needed.
const upgradeGuideRelPath = "../../docs-site/src/content/docs/guides/upgrade.md"

// unreleasedHeadingPattern matches the "## Unreleased" heading line itself
// (a real "## " heading, not a "### " sub-heading, and not any other line
// that merely mentions the word "Unreleased").
var unreleasedHeadingPattern = regexp.MustCompile(`(?m)^## Unreleased\b.*$`)

// nextH2HeadingPattern matches the next real "## " heading (excluding
// "### " sub-headings) after the Unreleased section starts, marking where
// that section ends.
var nextH2HeadingPattern = regexp.MustCompile(`(?m)^## [^#]`)

// extractUnreleasedSection returns the body of doc's "## Unreleased"
// section: everything from just after its heading line up to (but not
// including) the next "## " heading, or to the end of the document if
// there is none. Returns "" if no "## Unreleased" heading is found.
func extractUnreleasedSection(doc string) string {
	loc := unreleasedHeadingPattern.FindStringIndex(doc)
	if loc == nil {
		return ""
	}
	rest := doc[loc[1]:]
	if next := nextH2HeadingPattern.FindStringIndex(rest); next != nil {
		return rest[:next[0]]
	}
	return rest
}

// commandNameForBaselineRow returns the engram subcommand name a
// exitCodeBaseline row exercises: args[0] for every row in the table, since
// every row's args always leads with the subcommand under test (a client
// verb, an operator command, or -- for the one root-level legacy-env row --
// the subcommand used to invoke it). Deriving this from the row's own args
// rather than maintaining a second, hand-authored command list is what
// keeps this test unable to silently drift from exitCodeBaseline itself.
func commandNameForBaselineRow(c exitCodeBaselineCase) string {
	if len(c.args) == 0 {
		return ""
	}
	return c.args[0]
}

// TestUpgradeGuideNamesEveryChangedCommand is the D-09/D-03 closing
// documentation gate: for every exitCodeBaseline row claiming changes:
// true, the command name that row drives must appear somewhere in the
// upgrade guide's "## Unreleased" section. This is what makes "the upgrade
// guide names every command whose exit status changed" a mechanically
// enforced claim instead of a reading exercise a reviewer could get wrong.
//
// Guarded against a green-but-untested pass: if the docs file is absent (a
// trimmed checkout), this test skips explicitly rather than silently
// passing having read nothing; it also fails outright if the extracted
// section is empty, since an empty section satisfies no row's presence
// check.
func TestUpgradeGuideNamesEveryChangedCommand(t *testing.T) {
	data, err := os.ReadFile(upgradeGuideRelPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("upgrade guide not present at %s (trimmed checkout?) -- skipping rather than passing silently", upgradeGuideRelPath)
		}
		t.Fatalf("read %s: %v", upgradeGuideRelPath, err)
	}

	section := extractUnreleasedSection(string(data))
	if strings.TrimSpace(section) == "" {
		t.Fatalf("no non-empty %q section found in %s -- either the heading is missing or the section is empty", "## Unreleased", upgradeGuideRelPath)
	}

	seen := make(map[string]bool)
	for _, c := range exitCodeBaseline {
		if !c.changes {
			continue
		}
		cmdName := commandNameForBaselineRow(c)
		if cmdName == "" || seen[cmdName] {
			continue
		}
		seen[cmdName] = true
		if !strings.Contains(section, cmdName) {
			t.Errorf("row %q: command %q (its exit status changed in this phase) is not named anywhere in the upgrade guide's %q section", c.name, cmdName, "## Unreleased")
		}
	}
}
