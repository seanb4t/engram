// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// pluginGuideRelPath is the path to the plugin guide, relative to this
// package's own directory (cmd/engram). Two levels up reaches the repo
// root (cmd/engram -> cmd -> repo root) -- `go test` always runs with the
// package directory as its working directory, so a plain relative read
// resolves correctly with no repo-root lookup needed. Mirrors
// installGuideRelPath's idiom in install_docs_test.go.
const pluginGuideRelPath = "../../docs-site/src/content/docs/guides/plugin.md"

// pluginGuideViolations checks doc for the four ways plugin.md can fail
// to describe plugin-first installation and the preserved-result
// cross-link (D-07). It returns one error per violation so a caller can
// report every problem in a single run rather than stopping at the
// first. Both TestPluginGuideDocumentsPluginFirst (against the live
// guide) and TestPluginGuideGateFiresOnInjectedViolation (against
// in-memory fixtures) call this same function, so the positive control
// exercises the real gate rather than a parallel reimplementation.
func pluginGuideViolations(doc string) []error {
	var errs []error

	// Leg 1: the guide must cross-link to the agent-setup guide (already
	// present in the shipped guide's delegation paragraph -- this leg
	// pins that it is never silently deleted).
	agentSetupLinkStated := false
	for _, line := range strings.Split(doc, newline) {
		if strings.Contains(line, "/guides/agent-setup/") {
			agentSetupLinkStated = true
			break
		}
	}
	if !agentSetupLinkStated {
		errs = append(errs, fmt.Errorf("%s: no occurrence of `/guides/agent-setup/` -- the cross-link to the agent-setup guide is missing", pluginGuideRelPath))
	}

	// Leg 2: one line must state the plugin-first install statement --
	// `engram setup`, `--apply`, and `plugin` together.
	pluginFirstStated := false
	for _, line := range strings.Split(doc, newline) {
		if strings.Contains(line, "engram setup") && strings.Contains(line, "--apply") && strings.Contains(line, "plugin") {
			pluginFirstStated = true
			break
		}
	}
	if !pluginFirstStated {
		errs = append(errs, fmt.Errorf("%s: no line names `engram setup`, `--apply`, and `plugin` together -- the plugin-first install statement is missing", pluginGuideRelPath))
	}

	// Leg 3: the `preserved` remediation cross-link -- one line naming
	// `preserved` and pointing at the agent-setup guide's results
	// section, so an operator hitting the standalone-fallback block
	// discovers the manual step's rationale.
	preservedCrosslinkStated := false
	for _, line := range strings.Split(doc, newline) {
		if strings.Contains(line, "preserved") && strings.Contains(line, "/guides/agent-setup/") {
			preservedCrosslinkStated = true
			break
		}
	}
	if !preservedCrosslinkStated {
		errs = append(errs, fmt.Errorf("%s: no line names both `preserved` and `/guides/agent-setup/` -- the preserved remediation cross-link is missing", pluginGuideRelPath))
	}

	// Leg 4: the D-06 availability notice must be present. Version-agnostic
	// on purpose (matches "Available since v" rather than a pinned version
	// string). This leg read "Unreleased as of v" until the v0.17.0
	// post-release observation (05-RELEASE-0.17.0.md) flipped it -- the
	// flip is a deliberate gate edit, exactly as 05-POST-RELEASE.md required.
	if !strings.Contains(doc, "Available since v") {
		errs = append(errs, fmt.Errorf("%s: no occurrence of `Available since v` -- the D-06 availability notice is missing", pluginGuideRelPath))
	}

	return errs
}

// TestPluginGuideDocumentsPluginFirst is the closing documentation gate
// for plan 05-03: it asserts the live guide cross-links the agent-setup
// guide, states plugin-first installation via `engram setup --apply`,
// cross-links a `preserved` result to the agent-setup guide's results
// section, and carries the D-06 unreleased notice.
//
// Guarded against a green-but-untested pass: if the guide is absent (a
// trimmed checkout), this test skips explicitly rather than silently
// passing having read nothing; it also fails outright if the file content
// is empty.
func TestPluginGuideDocumentsPluginFirst(t *testing.T) {
	data, err := os.ReadFile(pluginGuideRelPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("plugin guide not present at %s (trimmed checkout?) -- skipping rather than passing silently", pluginGuideRelPath)
		}
		t.Fatalf("read %s: %v", pluginGuideRelPath, err)
	}
	if len(data) == 0 {
		t.Fatalf("%s is empty", pluginGuideRelPath)
	}

	for _, err := range pluginGuideViolations(string(data)) {
		t.Error(err)
	}
}

// TestPluginGuideGateFiresOnInjectedViolation is the D-07 positive
// control: it proves pluginGuideViolations discriminates a clean fixture
// from each of the four ways this plan's documentation could go wrong,
// entirely over in-memory strings so no file is ever mutated. The
// "clean" case is as load-bearing as the violation cases -- a gate that
// fires on everything discriminates nothing.
func TestPluginGuideGateFiresOnInjectedViolation(t *testing.T) {
	agentSetupLinkLine := "See [Agent Setup](/guides/agent-setup/) for runtime support and credential requirements."
	pluginFirstLine := "On a Claude Code whose plugin CLI works, `engram setup --apply` installs this plugin for you."
	preservedCrosslinkLine := "If setup reports the registration as `preserved`, see the [results section](/guides/agent-setup/#read-results-and-repeat-safely) of the setup guide."
	availabilityLine := ":::note[Available since v0.17.0]"
	cleanFixture := strings.Join([]string{agentSetupLinkLine, pluginFirstLine, preservedCrosslinkLine, availabilityLine}, newline) + newline

	cases := []struct {
		name            string
		fixture         string
		expectViolation bool
	}{
		{"clean", cleanFixture, false},
		// preservedCrosslinkLine also names /guides/agent-setup/ (the
		// results-section anchor), so removing agentSetupLinkLine alone
		// would leave leg 1 satisfied by leg 3's own cross-link -- strip
		// every occurrence of the substring to isolate leg 1's check.
		{"agent_setup_link_missing", strings.ReplaceAll(strings.Join([]string{pluginFirstLine, preservedCrosslinkLine, availabilityLine}, newline)+newline, "/guides/agent-setup/", ""), true},
		{"plugin_first_missing", strings.Join([]string{agentSetupLinkLine, preservedCrosslinkLine, availabilityLine}, newline) + newline, true},
		{"preserved_crosslink_missing", strings.Join([]string{agentSetupLinkLine, pluginFirstLine, availabilityLine}, newline) + newline, true},
		{"availability_notice_missing", strings.Join([]string{agentSetupLinkLine, pluginFirstLine, preservedCrosslinkLine}, newline) + newline, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			violations := pluginGuideViolations(c.fixture)
			if got := len(violations) > 0; got != c.expectViolation {
				t.Errorf("fixture %q: violations=%v (count %d), want expectViolation=%v", c.name, violations, len(violations), c.expectViolation)
			}
		})
	}
}
