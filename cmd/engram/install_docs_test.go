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
// the first.
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

	// Leg 2: the guide must name both install steps that write files, the
	// `engram completion` runs and the `engram man` run (.goreleaser.yaml's
	// install_steps), so it cannot describe only one of them.
	if !strings.Contains(doc, "`engram completion") || !strings.Contains(doc, "`engram man`") {
		errs = append(errs, fmt.Errorf("%s: the install steps must name both `engram completion` and `engram man` -- the cask-contents list is missing or incomplete", installGuideRelPath))
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

	// Leg 4: the D-06 availability notice must be present. Version-agnostic
	// on purpose (matches "Available since v" rather than a pinned version
	// string). This leg read "Unreleased as of v" until the v0.17.0
	// post-release observation (05-RELEASE-0.17.0.md) flipped it -- the
	// flip is a deliberate gate edit, exactly as 05-POST-RELEASE.md required.
	if !strings.Contains(doc, "Available since v") {
		errs = append(errs, fmt.Errorf("%s: no occurrence of `Available since v` -- the D-06 availability notice is missing", installGuideRelPath))
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
