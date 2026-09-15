// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// pluginScript maps a joined argv (strings.Join(args, " "), args as the
// fake Environment.Run receives them — WITHOUT argv[0]) to the RunResult
// that key should produce.
type pluginScript map[string]RunResult

// scriptedPluginRun returns an Environment.Run fake that records every
// args slice it receives into *calls, returns the scripted RunResult for
// a known key, and RunResult{ExitCode: 0} for any other key — a write
// verb the test did not script succeeds silently, so the assertion is on
// the recorded call sequence, never on a panic (mirrors detect_test.go's
// scriptedRun idiom, adapted for content-keyed lookup rather than
// call-order replay).
func scriptedPluginRun(script pluginScript, calls *[][]string) func(context.Context, string, []string) (RunResult, error) {
	return func(_ context.Context, _ string, args []string) (RunResult, error) {
		*calls = append(*calls, append([]string(nil), args...))
		if rr, ok := script[strings.Join(args, " ")]; ok {
			return rr, nil
		}
		return RunResult{ExitCode: 0}, nil
	}
}

// scriptedPluginRunErr is scriptedPluginRun's companion for a script that
// also needs to return a seam error (a process that never produced an
// exit status, e.g. a runSeam timeout) for a given key — checked before
// the ordinary result script.
func scriptedPluginRunErr(script pluginScript, errs map[string]error, calls *[][]string) func(context.Context, string, []string) (RunResult, error) {
	return func(_ context.Context, _ string, args []string) (RunResult, error) {
		*calls = append(*calls, append([]string(nil), args...))
		key := strings.Join(args, " ")
		if err, ok := errs[key]; ok {
			return RunResult{}, err
		}
		if rr, ok := script[key]; ok {
			return rr, nil
		}
		return RunResult{ExitCode: 0}, nil
	}
}

// Fixtures: claude-code list/marketplace stdout shapes.
const claudeListEmpty = `[]`

// claudeListStdout builds a `claude plugin list --json` array with an
// UNRELATED entry first (proves matching by id, never by position), then
// the engram@engram entry at version.
func claudeListStdout(version string) string {
	return fmt.Sprintf(`[{"id":"other@somewhere","version":"9.9.9","scope":"user"},{"id":"engram@engram","version":%q,"scope":"user","enabled":true}]`, version)
}

const claudeMarketplacePresent = "❯ engram\n    Source: GitHub (seanb4t/engram)"
const claudeMarketplaceFork = "❯ engram\n    Source: Directory (/Users/dev/engram-fork)"
const claudeMarketplaceAbsent = "❯ other\n    Source: GitHub (someone/other)"

// Fixtures: codex list/marketplace stdout shapes.
const codexListEmpty = `{"installed":[],"available":[]}`
const codexListNoKey = `{}`

// codexListStdout builds a `codex plugin list --json` document with an
// entry whose NAME matches but whose marketplace does not (proving both
// fields are matched), then the real engram@engram entry at version.
func codexListStdout(version string) string {
	return fmt.Sprintf(`{"installed":[{"pluginId":"engram@other","name":"engram","marketplaceName":"other","version":"9.9.9"},{"pluginId":"engram@engram","name":"engram","marketplaceName":"engram","version":%q,"installed":true,"enabled":true}],"available":[]}`, version)
}

const codexMarketplacePresent = "MARKETPLACE  ROOT\nengram  /home/fake/.codex/plugins/marketplaces/engram\n"
const codexMarketplaceAbsent = "MARKETPLACE  ROOT\nother  /home/fake/.codex/plugins/marketplaces/other\n"

// TestPluginVersionCompare pins classifyPluginVersion's whole table
// (D-01, D-03) and the four PluginState constants' string values.
func TestPluginVersionCompare(t *testing.T) {
	cases := []struct {
		name           string
		installed      string
		isInstalled    bool
		binary         string
		wantState      PluginState
		wantNoteExact  string
		wantNotePrefix string
		wantContains   []string
	}{
		{name: "absent", installed: "", isInstalled: false, binary: "0.16.1", wantState: PluginAbsent, wantNoteExact: ""},
		{name: "absent-dev-binary", installed: "", isInstalled: false, binary: "dev", wantState: PluginAbsent, wantNoteExact: ""},
		{name: "equal", installed: "0.16.1", isInstalled: true, binary: "0.16.1", wantState: PluginCurrent, wantNoteExact: ""},
		{name: "less", installed: "0.16.0", isInstalled: true, binary: "0.16.1", wantState: PluginOutdated, wantNoteExact: ""},
		{name: "less-numeric-not-lexical", installed: "0.9.9", isInstalled: true, binary: "0.16.1", wantState: PluginOutdated, wantNoteExact: ""},
		{name: "greater", installed: "0.17.0", isInstalled: true, binary: "0.16.1", wantState: PluginCurrent, wantNoteExact: "plugin 0.17.0 is newer than this binary 0.16.1"},
		{name: "greater-numeric-patch", installed: "0.16.10", isInstalled: true, binary: "0.16.9", wantState: PluginCurrent, wantNoteExact: "plugin 0.16.10 is newer than this binary 0.16.9"},
		{name: "dev-binary-bare", installed: "0.16.1", isInstalled: true, binary: "dev", wantState: PluginCurrent, wantNotePrefix: "dev build", wantContains: []string{"0.16.1"}},
		{name: "dev-binary-derived", installed: "0.16.1", isInstalled: true, binary: "0.16.2-dev.0+gabc123", wantState: PluginCurrent, wantNotePrefix: "dev build"},
		{name: "v-prefixed-binary", installed: "0.16.1", isInstalled: true, binary: "v0.16.1", wantState: PluginCurrent, wantNotePrefix: "dev build"},
		{name: "leading-zero-binary", installed: "0.16.1", isInstalled: true, binary: "0.16.01", wantState: PluginCurrent, wantNotePrefix: "dev build"},
		{name: "installed-prerelease", installed: "0.17.0-rc.1", isInstalled: true, binary: "0.16.1", wantState: PluginCurrent, wantContains: []string{"0.17.0-rc.1", "not a release version"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state, note := classifyPluginVersion(c.installed, c.isInstalled, c.binary)
			if state != c.wantState {
				t.Errorf("classifyPluginVersion(%q, %v, %q) state = %q, want %q", c.installed, c.isInstalled, c.binary, state, c.wantState)
			}
			switch {
			case c.wantNotePrefix != "":
				if !strings.HasPrefix(note, c.wantNotePrefix) {
					t.Errorf("note = %q, want prefix %q", note, c.wantNotePrefix)
				}
				for _, want := range c.wantContains {
					if !strings.Contains(note, want) {
						t.Errorf("note = %q, want contains %q", note, want)
					}
				}
			case len(c.wantContains) > 0:
				for _, want := range c.wantContains {
					if !strings.Contains(note, want) {
						t.Errorf("note = %q, want contains %q", note, want)
					}
				}
			default:
				if note != c.wantNoteExact {
					t.Errorf("note = %q, want %q", note, c.wantNoteExact)
				}
			}
		})
	}

	if PluginAbsent != "absent" || PluginOutdated != "outdated" || PluginCurrent != "current" || PluginUnavailable != "unavailable" {
		t.Errorf("PluginState constants = %q/%q/%q/%q, want absent/outdated/current/unavailable",
			PluginAbsent, PluginOutdated, PluginCurrent, PluginUnavailable)
	}
}

// pluginCase is one TestPluginPlan subtest's full expectation, driving
// both PluginPreview and PluginApply against a fresh scripted fake.
type pluginCase struct {
	name              string
	binaryVersion     string // default "0.16.1"
	listStdout        string
	marketplaceStdout string

	wantState     PluginState
	wantInstalled string
	wantSource    string
	wantCommand   string

	wantNoteExact  string
	wantNotePrefix string

	wantApplyOutcome Outcome
	wantApplyReason  string

	// actionScript/actionErrs override specific action-key results/errors
	// for the apply lane only (used by the apply-*-fails/apply-*-seam-error
	// cases).
	actionScript map[string]RunResult
	actionErrs   map[string]error
	// wantApplyExtraArgs is the sequence of action argv (Args[1:]) the
	// apply lane is expected to run AFTER the two probes, in order —
	// truncated at a failing/erroring action (inclusive).
	wantApplyExtraArgs [][]string
}

func runPluginCase(t *testing.T, rt Runtime, binary string, c pluginCase) {
	t.Helper()
	binaryVersion := c.binaryVersion
	if binaryVersion == "" {
		binaryVersion = "0.16.1"
	}
	pr, ok := rt.(PluginRuntime)
	if !ok {
		t.Fatalf("runtime %s does not implement PluginRuntime", rt.Name())
	}
	list, marketplace := pr.PluginProbes()

	baseScript := pluginScript{
		strings.Join(list[1:], " "):        {ExitCode: 0, Stdout: c.listStdout},
		strings.Join(marketplace[1:], " "): {ExitCode: 0, Stdout: c.marketplaceStdout},
	}

	// ---- preview lane: no write verb, no override scripts consulted ----
	var previewCalls [][]string
	previewEnv := fakeEnvWithRun(scriptedPluginRun(baseScript, &previewCalls), rt.Name())
	previewRes := PluginPreview(context.Background(), previewEnv, rt, binary, binaryVersion)
	assertPluginCommon(t, "preview", previewRes, c, binaryVersion)
	if previewRes.Outcome != OutcomeWouldWrite {
		t.Errorf("preview Outcome = %q, want %q", previewRes.Outcome, OutcomeWouldWrite)
	}
	wantPreviewCalls := [][]string{append([]string(nil), list[1:]...), append([]string(nil), marketplace[1:]...)}
	if !reflect.DeepEqual(previewCalls, wantPreviewCalls) {
		t.Errorf("preview calls = %v, want %v", previewCalls, wantPreviewCalls)
	}

	// ---- apply lane ----
	applyScript := pluginScript{}
	for k, v := range baseScript {
		applyScript[k] = v
	}
	for k, v := range c.actionScript {
		applyScript[k] = v
	}
	var applyCalls [][]string
	applyEnv := fakeEnvWithRun(scriptedPluginRunErr(applyScript, c.actionErrs, &applyCalls), rt.Name())
	applyRes := PluginApply(context.Background(), applyEnv, rt, binary, binaryVersion)
	assertPluginCommon(t, "apply", applyRes, c, binaryVersion)
	if applyRes.Outcome != c.wantApplyOutcome {
		t.Errorf("apply Outcome = %q, want %q", applyRes.Outcome, c.wantApplyOutcome)
	}
	if c.wantApplyReason != "" && applyRes.Reason != c.wantApplyReason {
		t.Errorf("apply Reason = %q, want %q", applyRes.Reason, c.wantApplyReason)
	}
	wantApplyCalls := make([][]string, 0, 2+len(c.wantApplyExtraArgs))
	wantApplyCalls = append(wantApplyCalls, append([]string(nil), list[1:]...), append([]string(nil), marketplace[1:]...))
	wantApplyCalls = append(wantApplyCalls, c.wantApplyExtraArgs...)
	if !reflect.DeepEqual(applyCalls, wantApplyCalls) {
		t.Errorf("apply calls = %v, want %v", applyCalls, wantApplyCalls)
	}
}

func assertPluginCommon(t *testing.T, lane string, res PluginResult, c pluginCase, binaryVersion string) {
	t.Helper()
	if !res.Attempted {
		t.Errorf("%s Attempted = false, want true", lane)
	}
	if res.State != c.wantState {
		t.Errorf("%s State = %q, want %q", lane, res.State, c.wantState)
	}
	if res.Installed != c.wantInstalled {
		t.Errorf("%s Installed = %q, want %q", lane, res.Installed, c.wantInstalled)
	}
	if res.Target != binaryVersion {
		t.Errorf("%s Target = %q, want %q", lane, res.Target, binaryVersion)
	}
	if res.Source != c.wantSource {
		t.Errorf("%s Source = %q, want %q", lane, res.Source, c.wantSource)
	}
	if res.Command != c.wantCommand {
		t.Errorf("%s Command = %q, want %q", lane, res.Command, c.wantCommand)
	}
	if c.wantNotePrefix != "" {
		if !strings.HasPrefix(res.Note, c.wantNotePrefix) {
			t.Errorf("%s Note = %q, want prefix %q", lane, res.Note, c.wantNotePrefix)
		}
	} else if res.Note != c.wantNoteExact {
		t.Errorf("%s Note = %q, want %q", lane, res.Note, c.wantNoteExact)
	}
	if !res.Delivered() {
		t.Errorf("%s Delivered() = false, want true", lane)
	}
}

// TestPluginPlan drives PluginPreview/PluginApply for every registered
// PluginRuntime across every PluginState (D-01..D-12).
func TestPluginPlan(t *testing.T) {
	t.Run("claude-code", func(t *testing.T) {
		rt := ClaudeCode
		binary := "/usr/local/bin/claude"

		cases := []pluginCase{
			{
				name:              "absent-marketplace-absent",
				listStdout:        claudeListEmpty,
				marketplaceStdout: claudeMarketplaceAbsent,
				wantState:         PluginAbsent,
				wantInstalled:     "",
				wantSource:        "",
				wantCommand:       "claude plugin marketplace add seanb4t/engram --scope user; claude plugin install engram@engram --scope user --json -y",
				wantNoteExact:     "",
				wantApplyOutcome:  OutcomeWrote,
				wantApplyExtraArgs: [][]string{
					{"plugin", "marketplace", "add", "seanb4t/engram", "--scope", "user"},
					{"plugin", "install", "engram@engram", "--scope", "user", "--json", "-y"},
				},
			},
			{
				name:              "absent-marketplace-present",
				listStdout:        claudeListEmpty,
				marketplaceStdout: claudeMarketplacePresent,
				wantState:         PluginAbsent,
				wantInstalled:     "",
				wantSource:        "GitHub (seanb4t/engram)",
				wantCommand:       "claude plugin install engram@engram --scope user --json -y",
				wantNoteExact:     "",
				wantApplyOutcome:  OutcomeWrote,
				wantApplyExtraArgs: [][]string{
					{"plugin", "install", "engram@engram", "--scope", "user", "--json", "-y"},
				},
			},
			{
				name:              "absent-marketplace-fork",
				listStdout:        claudeListEmpty,
				marketplaceStdout: claudeMarketplaceFork,
				wantState:         PluginAbsent,
				wantInstalled:     "",
				wantSource:        "Directory (/Users/dev/engram-fork)",
				wantCommand:       "claude plugin install engram@engram --scope user --json -y",
				wantNoteExact:     "",
				wantApplyOutcome:  OutcomeWrote,
				wantApplyExtraArgs: [][]string{
					{"plugin", "install", "engram@engram", "--scope", "user", "--json", "-y"},
				},
			},
			{
				name:              "outdated",
				listStdout:        claudeListStdout("0.16.0"),
				marketplaceStdout: claudeMarketplacePresent,
				wantState:         PluginOutdated,
				wantInstalled:     "0.16.0",
				wantSource:        "GitHub (seanb4t/engram)",
				wantCommand:       "claude plugin update engram@engram --scope user --json -y",
				wantNoteExact:     "",
				wantApplyOutcome:  OutcomeWrote,
				wantApplyExtraArgs: [][]string{
					{"plugin", "update", "engram@engram", "--scope", "user", "--json", "-y"},
				},
			},
			{
				name:              "outdated-numeric",
				listStdout:        claudeListStdout("0.9.9"),
				marketplaceStdout: claudeMarketplacePresent,
				wantState:         PluginOutdated,
				wantInstalled:     "0.9.9",
				wantSource:        "GitHub (seanb4t/engram)",
				wantCommand:       "claude plugin update engram@engram --scope user --json -y",
				wantNoteExact:     "",
				wantApplyOutcome:  OutcomeWrote,
				wantApplyExtraArgs: [][]string{
					{"plugin", "update", "engram@engram", "--scope", "user", "--json", "-y"},
				},
			},
			{
				name:              "current",
				listStdout:        claudeListStdout("0.16.1"),
				marketplaceStdout: claudeMarketplacePresent,
				wantState:         PluginCurrent,
				wantInstalled:     "0.16.1",
				wantSource:        "GitHub (seanb4t/engram)",
				wantCommand:       "",
				wantNoteExact:     "",
				wantApplyOutcome:  OutcomeAlreadyCorrect,
			},
			{
				name:              "current-newer-than-binary",
				listStdout:        claudeListStdout("0.17.0"),
				marketplaceStdout: claudeMarketplacePresent,
				wantState:         PluginCurrent,
				wantInstalled:     "0.17.0",
				wantSource:        "GitHub (seanb4t/engram)",
				wantCommand:       "",
				wantNoteExact:     "plugin 0.17.0 is newer than this binary 0.16.1",
				wantApplyOutcome:  OutcomeAlreadyCorrect,
			},
			{
				name:              "current-dev-binary",
				binaryVersion:     "dev",
				listStdout:        claudeListStdout("0.16.1"),
				marketplaceStdout: claudeMarketplacePresent,
				wantState:         PluginCurrent,
				wantInstalled:     "0.16.1",
				wantSource:        "GitHub (seanb4t/engram)",
				wantCommand:       "",
				wantNotePrefix:    "dev build",
				wantApplyOutcome:  OutcomeAlreadyCorrect,
			},
			{
				name:              "absent-dev-binary",
				binaryVersion:     "0.16.2-dev.0+gabc123",
				listStdout:        claudeListEmpty,
				marketplaceStdout: claudeMarketplacePresent,
				wantState:         PluginAbsent,
				wantInstalled:     "",
				wantSource:        "GitHub (seanb4t/engram)",
				wantCommand:       "claude plugin install engram@engram --scope user --json -y",
				wantNoteExact:     "",
				wantApplyOutcome:  OutcomeWrote,
				wantApplyExtraArgs: [][]string{
					{"plugin", "install", "engram@engram", "--scope", "user", "--json", "-y"},
				},
			},
			{
				name:              "apply-install-fails",
				listStdout:        claudeListEmpty,
				marketplaceStdout: claudeMarketplacePresent,
				wantState:         PluginAbsent,
				wantInstalled:     "",
				wantSource:        "GitHub (seanb4t/engram)",
				wantCommand:       "claude plugin install engram@engram --scope user --json -y",
				wantNoteExact:     "",
				actionScript: map[string]RunResult{
					"plugin install engram@engram --scope user --json -y": {ExitCode: 1, Stderr: "boom: install refused"},
				},
				wantApplyOutcome: OutcomeFailed,
				wantApplyReason:  "claude-code: claude plugin install engram@engram --scope user --json -y exited 1: 'boom: install refused'",
				wantApplyExtraArgs: [][]string{
					{"plugin", "install", "engram@engram", "--scope", "user", "--json", "-y"},
				},
			},
			{
				name:              "apply-marketplace-add-fails",
				listStdout:        claudeListEmpty,
				marketplaceStdout: claudeMarketplaceAbsent,
				wantState:         PluginAbsent,
				wantInstalled:     "",
				wantSource:        "",
				wantCommand:       "claude plugin marketplace add seanb4t/engram --scope user; claude plugin install engram@engram --scope user --json -y",
				wantNoteExact:     "",
				actionScript: map[string]RunResult{
					"plugin marketplace add seanb4t/engram --scope user": {ExitCode: 1, Stderr: "boom"},
				},
				wantApplyOutcome: OutcomeFailed,
				wantApplyReason:  "claude-code: claude plugin marketplace add seanb4t/engram --scope user exited 1: boom",
				wantApplyExtraArgs: [][]string{
					{"plugin", "marketplace", "add", "seanb4t/engram", "--scope", "user"},
				},
			},
			{
				name:              "apply-action-seam-error",
				listStdout:        claudeListEmpty,
				marketplaceStdout: claudeMarketplacePresent,
				wantState:         PluginAbsent,
				wantInstalled:     "",
				wantSource:        "GitHub (seanb4t/engram)",
				wantCommand:       "claude plugin install engram@engram --scope user --json -y",
				wantNoteExact:     "",
				actionErrs: map[string]error{
					"plugin install engram@engram --scope user --json -y": errors.New("exec: start failure"),
				},
				wantApplyOutcome: OutcomeFailed,
				wantApplyReason:  "claude-code: claude plugin install engram@engram --scope user --json -y: exec: start failure",
				wantApplyExtraArgs: [][]string{
					{"plugin", "install", "engram@engram", "--scope", "user", "--json", "-y"},
				},
			},
		}

		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				runPluginCase(t, rt, binary, c)
			})
		}
	})

	t.Run("codex", func(t *testing.T) {
		rt := Codex
		binary := "/usr/local/bin/codex"

		cases := []pluginCase{
			{
				name:              "absent-marketplace-absent",
				listStdout:        codexListEmpty,
				marketplaceStdout: codexMarketplaceAbsent,
				wantState:         PluginAbsent,
				wantInstalled:     "",
				wantSource:        "",
				wantCommand:       "codex plugin marketplace add https://github.com/seanb4t/engram --json; codex plugin add engram@engram --json",
				wantNoteExact:     "",
				wantApplyOutcome:  OutcomeWrote,
				wantApplyExtraArgs: [][]string{
					{"plugin", "marketplace", "add", "https://github.com/seanb4t/engram", "--json"},
					{"plugin", "add", "engram@engram", "--json"},
				},
			},
			{
				name:              "absent-marketplace-present",
				listStdout:        codexListEmpty,
				marketplaceStdout: codexMarketplacePresent,
				wantState:         PluginAbsent,
				wantInstalled:     "",
				wantSource:        "/home/fake/.codex/plugins/marketplaces/engram",
				wantCommand:       "codex plugin add engram@engram --json",
				wantNoteExact:     "",
				wantApplyOutcome:  OutcomeWrote,
				wantApplyExtraArgs: [][]string{
					{"plugin", "add", "engram@engram", "--json"},
				},
			},
			{
				name:              "outdated",
				listStdout:        codexListStdout("0.16.0"),
				marketplaceStdout: codexMarketplacePresent,
				wantState:         PluginOutdated,
				wantInstalled:     "0.16.0",
				wantSource:        "/home/fake/.codex/plugins/marketplaces/engram",
				wantCommand:       "codex plugin remove engram@engram --json; codex plugin add engram@engram --json",
				wantNoteExact:     "",
				wantApplyOutcome:  OutcomeWrote,
				wantApplyExtraArgs: [][]string{
					{"plugin", "remove", "engram@engram", "--json"},
					{"plugin", "add", "engram@engram", "--json"},
				},
			},
			{
				name:              "outdated-remove-fails",
				listStdout:        codexListStdout("0.16.0"),
				marketplaceStdout: codexMarketplacePresent,
				wantState:         PluginOutdated,
				wantInstalled:     "0.16.0",
				wantSource:        "/home/fake/.codex/plugins/marketplaces/engram",
				wantCommand:       "codex plugin remove engram@engram --json; codex plugin add engram@engram --json",
				wantNoteExact:     "",
				actionScript: map[string]RunResult{
					"plugin remove engram@engram --json": {ExitCode: 1, Stderr: "boom"},
				},
				wantApplyOutcome: OutcomeFailed,
				wantApplyReason:  "codex: codex plugin remove engram@engram --json exited 1: boom",
				wantApplyExtraArgs: [][]string{
					{"plugin", "remove", "engram@engram", "--json"},
				},
			},
			{
				// WR-03: codexPluginRemoveAction's own doc comment names
				// this exact destructive window ("if the following add
				// fails after this action succeeds, no codex plugin
				// remains until --apply is re-run") — this pins the
				// observable outcome the lane actually produces: remove
				// (unscripted here, so it succeeds via
				// scriptedPluginRunErr's default ExitCode:0) runs first,
				// then add fails, and the row fails naming "add" (never
				// "remove") in its Reason, with no third action attempted
				// (wantApplyExtraArgs stops at exactly the two actions).
				name:              "outdated-add-fails-after-remove-succeeds",
				listStdout:        codexListStdout("0.16.0"),
				marketplaceStdout: codexMarketplacePresent,
				wantState:         PluginOutdated,
				wantInstalled:     "0.16.0",
				wantSource:        "/home/fake/.codex/plugins/marketplaces/engram",
				wantCommand:       "codex plugin remove engram@engram --json; codex plugin add engram@engram --json",
				wantNoteExact:     "",
				actionScript: map[string]RunResult{
					"plugin add engram@engram --json": {ExitCode: 1, Stderr: "boom: add refused"},
				},
				wantApplyOutcome: OutcomeFailed,
				wantApplyReason:  "codex: codex plugin add engram@engram --json exited 1: 'boom: add refused'",
				wantApplyExtraArgs: [][]string{
					{"plugin", "remove", "engram@engram", "--json"},
					{"plugin", "add", "engram@engram", "--json"},
				},
			},
			{
				name:              "current",
				listStdout:        codexListStdout("0.16.1"),
				marketplaceStdout: codexMarketplacePresent,
				wantState:         PluginCurrent,
				wantInstalled:     "0.16.1",
				wantSource:        "/home/fake/.codex/plugins/marketplaces/engram",
				wantCommand:       "",
				wantNoteExact:     "",
				wantApplyOutcome:  OutcomeAlreadyCorrect,
			},
			{
				name:              "absent-empty-installed",
				listStdout:        codexListEmpty,
				marketplaceStdout: codexMarketplacePresent,
				wantState:         PluginAbsent,
				wantInstalled:     "",
				wantSource:        "/home/fake/.codex/plugins/marketplaces/engram",
				wantCommand:       "codex plugin add engram@engram --json",
				wantNoteExact:     "",
				wantApplyOutcome:  OutcomeWrote,
				wantApplyExtraArgs: [][]string{
					{"plugin", "add", "engram@engram", "--json"},
				},
			},
		}

		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				runPluginCase(t, rt, binary, c)
			})
		}

		// Every codex plugin action/probe uses "plugin" as Args[1] and
		// never carries a flag other than --json (Pitfall 2's
		// available-listing flag and D-06's scope flag are both excluded
		// by construction).
		codexPr, ok := rt.(PluginRuntime)
		if !ok {
			t.Fatalf("Codex does not implement PluginRuntime")
		}
		list, marketplace := codexPr.PluginProbes()
		for _, argv := range [][]string{list, marketplace} {
			if argv[0] != "codex" || argv[1] != "plugin" {
				t.Errorf("codex probe argv = %v, want Args[0]==\"codex\" and Args[1]==\"plugin\"", argv)
			}
		}
		for _, state := range []PluginState{PluginAbsent, PluginOutdated} {
			for _, marketplacePresent := range []bool{true, false} {
				for _, a := range codexPr.PluginActions(state, marketplacePresent) {
					if a.Args[0] != "codex" || a.Args[1] != "plugin" {
						t.Errorf("codex action Args = %v, want Args[0]==\"codex\" and Args[1]==\"plugin\"", a.Args)
					}
					for _, arg := range a.Args {
						if arg != "--json" && strings.HasPrefix(arg, "--") {
							t.Errorf("codex action Args = %v contains disallowed flag %q", a.Args, arg)
						}
					}
				}
			}
		}
	})
}

// TestPluginCapabilityProbeFailureFallsBackToNative asserts D-12's whole
// point: a probe failure of any kind is PluginUnavailable, never a failed
// outcome, and never runs a write verb.
func TestPluginCapabilityProbeFailureFallsBackToNative(t *testing.T) {
	t.Run("claude-code", func(t *testing.T) {
		rt := ClaudeCode
		binary := "/usr/local/bin/claude"
		listKey := "plugin list --json"
		marketplaceKey := "plugin marketplace list"

		runLane := func(t *testing.T, script pluginScript, errs map[string]error, wantCalls [][]string, wantNote string, checkNoteExact bool) {
			t.Helper()
			for _, lane := range []struct {
				name string
				fn   func(context.Context, Environment, Runtime, string, string) PluginResult
			}{
				{"preview", PluginPreview},
				{"apply", PluginApply},
			} {
				var calls [][]string
				env := fakeEnvWithRun(scriptedPluginRunErr(script, errs, &calls), "claude")
				res := lane.fn(context.Background(), env, rt, binary, "0.16.1")
				if !res.Attempted {
					t.Errorf("%s Attempted = false, want true", lane.name)
				}
				if res.State != PluginUnavailable {
					t.Errorf("%s State = %q, want %q", lane.name, res.State, PluginUnavailable)
				}
				if res.Outcome != "" {
					t.Errorf("%s Outcome = %q, want empty (never a failed outcome)", lane.name, res.Outcome)
				}
				if res.Command != "" {
					t.Errorf("%s Command = %q, want empty", lane.name, res.Command)
				}
				if res.Delivered() {
					t.Errorf("%s Delivered() = true, want false", lane.name)
				}
				if checkNoteExact {
					if res.Note != wantNote {
						t.Errorf("%s Note = %q, want %q", lane.name, res.Note, wantNote)
					}
				} else if !strings.Contains(res.Note, wantNote) {
					t.Errorf("%s Note = %q, want contains %q", lane.name, res.Note, wantNote)
				}
				if !reflect.DeepEqual(calls, wantCalls) {
					t.Errorf("%s calls = %v, want %v", lane.name, calls, wantCalls)
				}
			}
		}

		t.Run("list-exit-1", func(t *testing.T) {
			script := pluginScript{listKey: {ExitCode: 1, Stderr: "unknown command plugin"}}
			runLane(t, script, nil, [][]string{{"plugin", "list", "--json"}},
				"claude-code: claude plugin list --json exited 1: 'unknown command plugin'", true)
		})

		t.Run("list-timeout", func(t *testing.T) {
			errs := map[string]error{listKey: context.DeadlineExceeded}
			runLane(t, pluginScript{}, errs, [][]string{{"plugin", "list", "--json"}},
				"timed out after 20s", false)
		})

		t.Run("list-empty-stdout", func(t *testing.T) {
			script := pluginScript{listKey: {ExitCode: 0, Stdout: ""}}
			runLane(t, script, nil, [][]string{{"plugin", "list", "--json"}},
				"claude plugin list --json", false)
		})

		t.Run("list-not-json", func(t *testing.T) {
			script := pluginScript{listKey: {ExitCode: 0, Stdout: "not json"}}
			runLane(t, script, nil, [][]string{{"plugin", "list", "--json"}},
				"claude plugin list --json", false)
		})

		t.Run("list-wrong-shape", func(t *testing.T) {
			script := pluginScript{listKey: {ExitCode: 0, Stdout: "{}"}}
			runLane(t, script, nil, [][]string{{"plugin", "list", "--json"}},
				"claude plugin list --json", false)
		})

		t.Run("marketplace-probe-fails-while-absent", func(t *testing.T) {
			script := pluginScript{
				listKey:        {ExitCode: 0, Stdout: claudeListEmpty},
				marketplaceKey: {ExitCode: 1, Stderr: "boom"},
			}
			runLane(t, script, nil, [][]string{{"plugin", "list", "--json"}, {"plugin", "marketplace", "list"}},
				"claude plugin marketplace list", false)
			runLane(t, script, nil, [][]string{{"plugin", "list", "--json"}, {"plugin", "marketplace", "list"}},
				"exited 1", false)
		})

		t.Run("marketplace-probe-fails-while-current", func(t *testing.T) {
			script := pluginScript{
				listKey:        {ExitCode: 0, Stdout: claudeListStdout("0.16.1")},
				marketplaceKey: {ExitCode: 1, Stderr: "boom"},
			}
			for _, lane := range []struct {
				name string
				fn   func(context.Context, Environment, Runtime, string, string) PluginResult
			}{
				{"preview", PluginPreview},
				{"apply", PluginApply},
			} {
				var calls [][]string
				env := fakeEnvWithRun(scriptedPluginRun(script, &calls), "claude")
				res := lane.fn(context.Background(), env, rt, binary, "0.16.1")
				if res.State != PluginCurrent {
					t.Errorf("%s State = %q, want %q", lane.name, res.State, PluginCurrent)
				}
				if !res.Delivered() {
					t.Errorf("%s Delivered() = false, want true", lane.name)
				}
				if !strings.Contains(res.Note, "claude plugin marketplace list") || !strings.Contains(res.Note, "exited 1") {
					t.Errorf("%s Note = %q, want contains %q and %q", lane.name, res.Note, "claude plugin marketplace list", "exited 1")
				}
				if res.Source != "" {
					t.Errorf("%s Source = %q, want empty", lane.name, res.Source)
				}
				if lane.name == "apply" && res.Outcome != OutcomeAlreadyCorrect {
					t.Errorf("apply Outcome = %q, want %q", res.Outcome, OutcomeAlreadyCorrect)
				}
			}
		})
	})

	t.Run("codex", func(t *testing.T) {
		rt := Codex
		binary := "/usr/local/bin/codex"
		listKey := "plugin list --json"
		marketplaceKey := "plugin marketplace list"

		runLane := func(t *testing.T, script pluginScript, errs map[string]error, wantCalls [][]string, wantNote string, checkNoteExact bool) {
			t.Helper()
			for _, lane := range []struct {
				name string
				fn   func(context.Context, Environment, Runtime, string, string) PluginResult
			}{
				{"preview", PluginPreview},
				{"apply", PluginApply},
			} {
				var calls [][]string
				env := fakeEnvWithRun(scriptedPluginRunErr(script, errs, &calls), "codex")
				res := lane.fn(context.Background(), env, rt, binary, "0.16.1")
				if !res.Attempted {
					t.Errorf("%s Attempted = false, want true", lane.name)
				}
				if res.State != PluginUnavailable {
					t.Errorf("%s State = %q, want %q", lane.name, res.State, PluginUnavailable)
				}
				if res.Outcome != "" {
					t.Errorf("%s Outcome = %q, want empty (never a failed outcome)", lane.name, res.Outcome)
				}
				if res.Delivered() {
					t.Errorf("%s Delivered() = true, want false", lane.name)
				}
				if checkNoteExact {
					if res.Note != wantNote {
						t.Errorf("%s Note = %q, want %q", lane.name, res.Note, wantNote)
					}
				} else if !strings.Contains(res.Note, wantNote) {
					t.Errorf("%s Note = %q, want contains %q", lane.name, res.Note, wantNote)
				}
				if !reflect.DeepEqual(calls, wantCalls) {
					t.Errorf("%s calls = %v, want %v", lane.name, calls, wantCalls)
				}
			}
		}

		t.Run("list-exit-1", func(t *testing.T) {
			script := pluginScript{listKey: {ExitCode: 1, Stderr: "unknown subcommand"}}
			runLane(t, script, nil, [][]string{{"plugin", "list", "--json"}},
				"codex: codex plugin list --json exited 1: 'unknown subcommand'", true)
		})

		t.Run("list-wrong-shape-array", func(t *testing.T) {
			script := pluginScript{listKey: {ExitCode: 0, Stdout: "[]"}}
			runLane(t, script, nil, [][]string{{"plugin", "list", "--json"}},
				"codex plugin list --json", false)
		})

		t.Run("list-no-installed-key", func(t *testing.T) {
			script := pluginScript{listKey: {ExitCode: 0, Stdout: codexListNoKey}}
			runLane(t, script, nil, [][]string{{"plugin", "list", "--json"}},
				"codex plugin list --json", false)
		})

		t.Run("list-empty-stdout", func(t *testing.T) {
			script := pluginScript{listKey: {ExitCode: 0, Stdout: ""}}
			runLane(t, script, nil, [][]string{{"plugin", "list", "--json"}},
				"codex plugin list --json", false)
		})

		t.Run("marketplace-probe-fails-while-absent", func(t *testing.T) {
			script := pluginScript{
				listKey:        {ExitCode: 0, Stdout: codexListEmpty},
				marketplaceKey: {ExitCode: 1, Stderr: "boom"},
			}
			runLane(t, script, nil, [][]string{{"plugin", "list", "--json"}, {"plugin", "marketplace", "list"}},
				"codex plugin marketplace list", false)
		})
	})
}

// TestPluginRuntimeIsOptional asserts PluginRuntime's optional-interface
// idiom: only claude-code and codex implement it, and driving
// PluginPreview/PluginApply against a runtime that does not never issues
// a single Run call.
func TestPluginRuntimeIsOptional(t *testing.T) {
	if _, ok := ClaudeCode.(PluginRuntime); !ok {
		t.Error("ClaudeCode.(PluginRuntime) = false, want true")
	}
	if _, ok := Codex.(PluginRuntime); !ok {
		t.Error("Codex.(PluginRuntime) = false, want true")
	}
	if _, ok := OpenCode.(PluginRuntime); ok {
		t.Error("OpenCode.(PluginRuntime) = true, want false")
	}
	if _, ok := Generic.(PluginRuntime); ok {
		t.Error("Generic.(PluginRuntime) = true, want false")
	}

	for _, rt := range []Runtime{OpenCode, Generic} {
		t.Run(rt.Name(), func(t *testing.T) {
			var calls [][]string
			recordingRun := func(_ context.Context, _ string, args []string) (RunResult, error) {
				calls = append(calls, args)
				return RunResult{ExitCode: 0}, nil
			}
			env := fakeEnvWithRun(recordingRun, "opencode")
			previewRes := PluginPreview(context.Background(), env, rt, "/usr/local/bin/x", "0.16.1")
			if previewRes.Attempted {
				t.Errorf("preview Attempted = true, want false")
			}
			if previewRes.Delivered() {
				t.Errorf("preview Delivered() = true, want false")
			}
			applyRes := PluginApply(context.Background(), env, rt, "/usr/local/bin/x", "0.16.1")
			if applyRes.Attempted {
				t.Errorf("apply Attempted = true, want false")
			}
			if applyRes.Delivered() {
				t.Errorf("apply Delivered() = true, want false")
			}
			if len(calls) != 0 {
				t.Errorf("recorded %d calls, want 0", len(calls))
			}
		})
	}

	t.Run("empty-binary", func(t *testing.T) {
		var calls [][]string
		recordingRun := func(_ context.Context, _ string, args []string) (RunResult, error) {
			calls = append(calls, args)
			return RunResult{ExitCode: 0}, nil
		}
		env := fakeEnvWithRun(recordingRun, "claude")
		res := PluginApply(context.Background(), env, ClaudeCode, "", "0.16.1")
		if res.Attempted {
			t.Errorf("Attempted = true, want false")
		}
		if len(calls) != 0 {
			t.Errorf("recorded %d calls, want 0", len(calls))
		}
	})
}

// TestPluginResultFieldsAreBoundCaptured proves WR-02: the three untrusted
// plugin-CLI strings that land on rendered PluginResult fields — Installed
// (entry.Version out of `plugin list --json`), Source (the observed
// "Source:" line out of `plugin marketplace list`), and
// classifyPluginVersion's "newer than binary" Note — are bounded through
// boundCapture exactly like apply.go bounds Result.Reason/Registered/Notes,
// mirroring TestApply's own
// "captured-output-over-budget-truncated-on-rune-boundary" (apply_test.go).
func TestPluginResultFieldsAreBoundCaptured(t *testing.T) {
	rt := ClaudeCode
	binary := "/usr/local/bin/claude"

	// A pure-ASCII digit run well over maxCapturedBytes. It carries no dots,
	// so pluginVersionCorePattern does not match and parseVersionCore
	// reports !ok at the regex stage (never reaching strconv.ParseUint),
	// landing classifyPluginVersion in its "not a release version" arm —
	// which is exactly the arm whose Note interpolates the untrusted
	// installed string (plugin.go WR-02 fix).
	longVersion := strings.Repeat("9", maxCapturedBytes+2000)
	listStdout := claudeListStdout(longVersion)

	longSource := "GitHub (" + strings.Repeat("z", maxCapturedBytes+2000) + ")"
	marketplaceStdout := "❯ engram\n    Source: " + longSource

	pr, ok := rt.(PluginRuntime)
	if !ok {
		t.Fatalf("claude-code does not implement PluginRuntime")
	}
	list, marketplace := pr.PluginProbes()
	script := pluginScript{
		strings.Join(list[1:], " "):        {ExitCode: 0, Stdout: listStdout},
		strings.Join(marketplace[1:], " "): {ExitCode: 0, Stdout: marketplaceStdout},
	}
	var calls [][]string
	env := fakeEnvWithRun(scriptedPluginRun(script, &calls), "claude")

	res := PluginPreview(context.Background(), env, rt, binary, "0.16.1")

	if !res.Attempted {
		t.Fatalf("Attempted = false, want true")
	}
	if !utf8.ValidString(res.Installed) {
		t.Errorf("Installed is not valid UTF-8 after truncation: %q", res.Installed)
	}
	if !strings.Contains(res.Installed, truncationMarker) {
		t.Errorf("Installed = %q, want it to carry the truncation marker %q", res.Installed, truncationMarker)
	}
	if len(res.Installed) >= len(longVersion) {
		t.Errorf("Installed length %d, want it bounded well below the untruncated version length %d", len(res.Installed), len(longVersion))
	}

	if !utf8.ValidString(res.Source) {
		t.Errorf("Source is not valid UTF-8 after truncation: %q", res.Source)
	}
	if !strings.Contains(res.Source, truncationMarker) {
		t.Errorf("Source = %q, want it to carry the truncation marker %q", res.Source, truncationMarker)
	}
	if len(res.Source) >= len(longSource) {
		t.Errorf("Source length %d, want it bounded well below the untruncated source length %d", len(res.Source), len(longSource))
	}

	if !strings.Contains(res.Note, truncationMarker) {
		t.Errorf("Note = %q, want it to carry the truncation marker %q (classifyPluginVersion's installed interpolation)", res.Note, truncationMarker)
	}
}

// fakeTolerantPluginRuntime is a synthetic Runtime+PluginRuntime authored
// ONLY in this test: no real runtime (claude-code, codex) ever sets
// Action.Tolerant on a plugin action today (see plugin.go's own package
// doc comment), so this is the only way to exercise executePlugin's
// Tolerant handling (WR-01) — proving a FUTURE tolerant plugin action gets
// apply.go's own execute() semantics rather than silently failing the row.
type fakeTolerantPluginRuntime struct{}

func (fakeTolerantPluginRuntime) Name() string                            { return "faketolerant" }
func (fakeTolerantPluginRuntime) Detect(Environment) bool                 { return true }
func (fakeTolerantPluginRuntime) Plan(Environment, Options) (Plan, error) { return Plan{}, nil }

func (fakeTolerantPluginRuntime) PluginProbes() (list, marketplace []string) {
	return []string{"faketolerant", "plugin", "list", "--json"},
		[]string{"faketolerant", "plugin", "marketplace", "list"}
}

func (fakeTolerantPluginRuntime) ParsePluginList(string) (version string, installed bool, err error) {
	return "", false, nil
}

func (fakeTolerantPluginRuntime) ParseMarketplaceList(string) (present bool, source string) {
	return false, ""
}

// PluginActions returns a Tolerant "clear" step (authored to fail in the
// test's script below) followed by a non-tolerant "install" step — the
// same tolerant-clear-then-fatal-write shape 03-RESEARCH.md's Pattern
// 1/Pitfall 1 and apply.go's own doc comment (apply.go:220-221) describe.
func (fakeTolerantPluginRuntime) PluginActions(PluginState, bool) []Action {
	return []Action{
		{Args: []string{"faketolerant", "clear"}, Tolerant: true, Description: "clear any prior state"},
		{Args: []string{"faketolerant", "install"}},
	}
}

// TestExecutePluginHonorsTolerant proves WR-01: a Tolerant action's nonzero
// exit is recorded onto Note and the sequence continues to run the
// following action, mirroring apply.go's execute() (apply.go:335,344) —
// rather than failing the row on the tolerated action's own exit, which is
// what executePlugin did before this fix.
func TestExecutePluginHonorsTolerant(t *testing.T) {
	rt := fakeTolerantPluginRuntime{}
	binary := "/usr/local/bin/faketolerant"

	script := pluginScript{
		"plugin list --json":      {ExitCode: 0},
		"plugin marketplace list": {ExitCode: 0},
		"clear":                   {ExitCode: 1, Stderr: "not found"},
		"install":                 {ExitCode: 0},
	}
	var calls [][]string
	env := fakeEnvWithRun(scriptedPluginRun(script, &calls), "faketolerant")

	res := PluginApply(context.Background(), env, rt, binary, "0.16.1")

	if res.Outcome != OutcomeWrote {
		t.Fatalf("Outcome = %q, want %q — a Tolerant action's nonzero exit must not fail the row", res.Outcome, OutcomeWrote)
	}
	if !strings.Contains(res.Note, "clear") || !strings.Contains(res.Note, "exited 1") {
		t.Errorf("Note = %q, want it to record the tolerated action's nonzero exit", res.Note)
	}
	wantCalls := [][]string{
		{"plugin", "list", "--json"},
		{"plugin", "marketplace", "list"},
		{"clear"},
		{"install"},
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Errorf("calls = %v, want %v — the sequence must continue past the tolerated failure to run the following action", calls, wantCalls)
	}
}
