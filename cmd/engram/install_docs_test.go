// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// installGuideRelPath is the path to the install guide, relative to this
// package's own directory (cmd/engram). Two levels up reaches the repo
// root (cmd/engram -> cmd -> repo root) -- `go test` always runs with the
// package directory as its working directory, so a plain relative read
// resolves correctly with no repo-root lookup needed. Mirrors
// agentSetupGuideRelPath's idiom in agent_setup_docs_test.go.
const installGuideRelPath = "../../docs-site/src/content/docs/guides/install.md"

// installGuideViolations checks doc for the four ways install.md can fail
// to describe what the Homebrew cask installs and where plugin-first
// delivery is documented (D-07). It returns one error per violation so a
// caller can report every problem in a single run rather than stopping at
// the first. Both TestInstallGuideDocumentsSetupV2 (against the live
// guide) and TestInstallGuideGateFiresOnInjectedViolation (against
// in-memory fixtures) call this same function, so the positive control
// exercises the real gate rather than a parallel reimplementation.
func installGuideViolations(doc string) []error {
	var errs []error

	// Leg 1: the man page for the setup subcommand must be named
	// explicitly -- cobra's GenManTree dash-separates the command path
	// (05-RESEARCH.md, verified against the pinned cobra source), so the
	// installed page is engram-setup.1 and `man engram-setup` is the
	// truthful invocation.
	if !strings.Contains(doc, "man engram-setup") {
		errs = append(errs, fmt.Errorf("%s: no occurrence of `man engram-setup` -- the guide must name the installed setup man page", installGuideRelPath))
	}

	// Leg 2: one line must state that the cask installs BOTH completions
	// and man pages -- the same install hook writes both
	// (.goreleaser.yaml's post_install), so the cask-contents sentence
	// must not describe only completions.
	caskContentsStated := false
	for _, line := range strings.Split(doc, newline) {
		if strings.Contains(line, "completions") && strings.Contains(line, "man page") {
			caskContentsStated = true
			break
		}
	}
	if !caskContentsStated {
		errs = append(errs, fmt.Errorf("%s: no line names both `completions` and `man page` -- the cask-contents sentence is missing or incomplete", installGuideRelPath))
	}

	// Leg 3: one line must point at the plugin-first delivery pointer --
	// `plugin` and the agent-setup guide link on the same line.
	pluginPointerStated := false
	for _, line := range strings.Split(doc, newline) {
		if strings.Contains(line, "plugin") && strings.Contains(line, "/guides/agent-setup/") {
			pluginPointerStated = true
			break
		}
	}
	if !pluginPointerStated {
		errs = append(errs, fmt.Errorf("%s: no line names both `plugin` and `/guides/agent-setup/` -- the plugin-first pointer is missing", installGuideRelPath))
	}

	// Leg 4: the D-06 unreleased notice must be present. Version-agnostic
	// on purpose (matches "Unreleased as of v" rather than a pinned
	// version string) so the post-release flip to an "available in"
	// notice (05-POST-RELEASE.md) is a deliberate gate edit, not an
	// accident this gate would silently miss.
	if !strings.Contains(doc, "Unreleased as of v") {
		errs = append(errs, fmt.Errorf("%s: no occurrence of `Unreleased as of v` -- the D-06 unreleased notice is missing", installGuideRelPath))
	}

	return errs
}

// TestInstallGuideDocumentsSetupV2 is the closing documentation gate for
// plan 05-03: it asserts the live guide names the installed setup man
// page, states what the cask installs (completions and man pages), points
// at plugin-first delivery, and carries the D-06 unreleased notice.
//
// Guarded against a green-but-untested pass: if the guide is absent (a
// trimmed checkout), this test skips explicitly rather than silently
// passing having read nothing; it also fails outright if the file content
// is empty.
func TestInstallGuideDocumentsSetupV2(t *testing.T) {
	data, err := os.ReadFile(installGuideRelPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("install guide not present at %s (trimmed checkout?) -- skipping rather than passing silently", installGuideRelPath)
		}
		t.Fatalf("read %s: %v", installGuideRelPath, err)
	}
	if len(data) == 0 {
		t.Fatalf("%s is empty", installGuideRelPath)
	}

	for _, err := range installGuideViolations(string(data)) {
		t.Error(err)
	}
}

// TestInstallGuideGateFiresOnInjectedViolation is the D-07 positive
// control: it proves installGuideViolations discriminates a clean
// fixture from each of the four ways this plan's documentation could go
// wrong, entirely over in-memory strings so no file is ever mutated. The
// "clean" case is as load-bearing as the violation cases -- a gate that
// fires on everything discriminates nothing.
func TestInstallGuideGateFiresOnInjectedViolation(t *testing.T) {
	manPageLine := "After a cask install, `man engram` and `man engram-setup` describe the CLI and the setup command."
	caskContentsLine := "The cask's install hooks check the installed version, generate bash, zsh, and fish completions, and write one man page per command into Homebrew's `share/man/man1` directory."
	pluginPointerLine := "Setup registers the MCP server and, on a Claude Code or Codex whose plugin CLI works, delivers the curation skills plugin-first through the runtime's plugin system -- see [Agent Setup](/guides/agent-setup/)."
	unreleasedLine := ":::note[Unreleased as of v0.16.1]"
	cleanFixture := strings.Join([]string{unreleasedLine, caskContentsLine, manPageLine, pluginPointerLine}, newline) + newline

	cases := []struct {
		name            string
		fixture         string
		expectViolation bool
	}{
		{"clean", cleanFixture, false},
		{"man_page_missing", strings.Join([]string{unreleasedLine, caskContentsLine, pluginPointerLine}, newline) + newline, true},
		{"cask_contents_missing", strings.Join([]string{unreleasedLine, manPageLine, pluginPointerLine}, newline) + newline, true},
		{"plugin_pointer_missing", strings.Join([]string{unreleasedLine, caskContentsLine, manPageLine}, newline) + newline, true},
		{"unreleased_notice_missing", strings.Join([]string{caskContentsLine, manPageLine, pluginPointerLine}, newline) + newline, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			violations := installGuideViolations(c.fixture)
			if got := len(violations) > 0; got != c.expectViolation {
				t.Errorf("fixture %q: violations=%v (count %d), want expectViolation=%v", c.name, violations, len(violations), c.expectViolation)
			}
		})
	}
}
