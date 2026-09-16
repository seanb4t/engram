// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/seanb4t/engram/internal/setup"
	"github.com/seanb4t/engram/internal/skills"
)

// fakeSetupEnvSucceedingRun is the default Run every fakeSetupEnv-built
// Environment carries: every invocation succeeds with no captured output.
// Apply() is real as of this phase (D-09 retired), so any test exercising
// --apply against a present runtime now drives a real Environment.Run
// call — a nil Run field panics (repo rule m45p2b4bp7 forbids scripting a
// SPECIFIC runtime's real behavior here; "succeeded, no output" is the
// generic default every test that does not care about the exact response
// can rely on).
func fakeSetupEnvSucceedingRun(context.Context, string, []string) (setup.RunResult, error) {
	return setup.RunResult{ExitCode: 0}, nil
}

// fakeSetupEnv builds a setup.Environment resolving only the binaries
// named present — swapped onto the package-level setupEnv seam so a test
// drives detection without touching the real machine's PATH (repo rule
// m45p2b4bp7: never test or red-gate third-party CLI behavior). Its Run
// seam defaults to fakeSetupEnvSucceedingRun; use fakeSetupEnvWithRun to
// script a specific response.
func fakeSetupEnv(present ...string) setup.Environment {
	set := make(map[string]bool, len(present))
	for _, name := range present {
		set[name] = true
	}
	return setup.Environment{
		LookPath: func(file string) (string, error) {
			if set[file] {
				return "/usr/local/bin/" + file, nil
			}
			return "", exec.ErrNotFound
		},
		Getenv:  func(string) string { return "" },
		HomeDir: func() (string, error) { return "/home/fake", nil },
		Run:     fakeSetupEnvSucceedingRun,
	}
}

// fakeSetupEnvWithRun is fakeSetupEnv(present...) with its Run seam
// replaced by run, for a test that needs to script a specific runtime CLI
// failure.
func fakeSetupEnvWithRun(run func(context.Context, string, []string) (setup.RunResult, error), present ...string) setup.Environment {
	env := fakeSetupEnv(present...)
	env.Run = run
	return env
}

// fakeSkillsEnv builds an in-memory skills.Environment backed by a map,
// so no test in this file ever installs skills to a real home directory
// (repo rule m45p2b4bp7) — the skills-package analogue of fakeSetupEnv
// above. MkdirAll is a no-op; the in-memory map has no directory concept
// to create. Lstat always reports absent (os.ErrNotExist): every test
// that drives this fake exercises the NATIVE write path
// (setupApplySkillsFacet), never the plugin-delivered presence report
// (setupReportNativeSkills), so there is nothing for a real Lstat to
// find here.
func fakeSkillsEnv() skills.Environment {
	store := make(map[string][]byte)
	return skills.Environment{
		ReadFile: func(name string) ([]byte, error) {
			b, ok := store[name]
			if !ok {
				return nil, os.ErrNotExist
			}
			return b, nil
		},
		WriteFile: func(name string, data []byte, _ os.FileMode) error {
			cp := make([]byte, len(data))
			copy(cp, data)
			store[name] = cp
			return nil
		},
		MkdirAll: func(string, os.FileMode) error { return nil },
		Lstat:    func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
	}
}

// fakeFileInfo is a minimal os.FileInfo implementation for
// fakeSkillsEnvWithEntries' Lstat responses — only Mode() is ever
// consulted by skills.DetectPresence (presence.go), but the interface
// requires the rest.
type fakeFileInfo struct {
	name string
	mode os.FileMode
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeFileInfo) Sys() any           { return nil }

// fakeSkillsEnvWithEntries builds an in-memory skills.Environment exactly
// like fakeSkillsEnv, backed by store, but with a SCRIPTED Lstat: a path
// present in entries (keyed by its full path, e.g.
// "/home/fake/.agents/skills" or "/home/fake/.agents/skills/curating-memory")
// reports a fakeFileInfo carrying that entry's os.FileMode (os.ModeDir,
// os.ModeSymlink, or 0 for an ordinary file); any other path reports
// os.ErrNotExist — the D-08 presence-report analogue of fakeSkillsEnv
// above, for a test exercising setupReportNativeSkills/DetectPresence
// rather than the native write path.
func fakeSkillsEnvWithEntries(entries map[string]os.FileMode, store map[string][]byte) skills.Environment {
	return skills.Environment{
		ReadFile: func(name string) ([]byte, error) {
			b, ok := store[name]
			if !ok {
				return nil, os.ErrNotExist
			}
			return b, nil
		},
		WriteFile: func(name string, data []byte, _ os.FileMode) error {
			cp := make([]byte, len(data))
			copy(cp, data)
			store[name] = cp
			return nil
		},
		MkdirAll: func(string, os.FileMode) error { return nil },
		Lstat: func(name string) (os.FileInfo, error) {
			mode, ok := entries[name]
			if !ok {
				return nil, os.ErrNotExist
			}
			return fakeFileInfo{name: filepath.Base(name), mode: mode}, nil
		},
	}
}

// withFakeSetupEnv points the package-level setupEnv seam at env for the
// duration of the test, and ALSO points skillsEnv at a fresh
// fakeSkillsEnv() (Phase 4): every existing test in this file drives
// setup.Environment through this one helper, and the skills facet
// (setupApplySkillsFacet, setup.go) now runs unconditionally for any
// present, skills-wired runtime (claude-code, this wave) reached through
// setupPreview/setupApplyRun — without this, an unmodified pre-Phase-4
// test would silently attempt a REAL skills.Install against the caller's
// actual home directory. A test that needs to script a specific skills
// outcome (e.g. a scripted install failure) overrides the package-level
// skillsEnv itself, after calling this helper.
func withFakeSetupEnv(t *testing.T, env setup.Environment) {
	t.Helper()
	orig := setupEnv
	setupEnv = env
	t.Cleanup(func() { setupEnv = orig })

	origSkills := skillsEnv
	skillsEnv = fakeSkillsEnv()
	t.Cleanup(func() { skillsEnv = origSkills })
}

// withFakeSetupVersion points the package-level setupVersion seam at a
// fixed value v for the duration of the test, restoring the original via
// t.Cleanup — mirrors withFakeSetupEnv above. Every plugin test MUST go
// through this helper rather than assigning setupVersion directly: a
// real test binary's resolvedVersion() resolves to "dev" or a derived
// "-dev.0+g<hash>" form, which D-03 deliberately never treats as
// comparable, so an un-seamed plugin test would nondeterministically
// exercise only the non-comparable branch of classifyPluginVersion.
func withFakeSetupVersion(t *testing.T, v string) {
	t.Helper()
	orig := setupVersion
	setupVersion = func() string { return v }
	t.Cleanup(func() { setupVersion = orig })
}

// codexGetEngramBearerJSON is cmd/engram's own copy of
// internal/setup/drift_test.go's codexGetEngramBearer fixture: the two
// packages cannot share test code, so this is the SAME 04-RESEARCH.md
// verbatim `codex mcp get engram --json` document — name "engram", URL
// https://engram.example.com/mcp, bearer var ENGRAM_TOKEN, every other
// field null/true exactly as observed — kept in sync by inspection
// (04-04-PLAN.md Task 1).
const codexGetEngramBearerJSON = `{"name":"engram","enabled":true,"disabled_reason":null,"transport":{"type":"streamable_http","url":"https://engram.example.com/mcp","bearer_token_env_var":"ENGRAM_TOKEN","http_headers":null,"env_http_headers":null,"http_headers_helper":null},"enabled_tools":null,"disabled_tools":null,"startup_timeout_sec":null,"tool_timeout_sec":null}`

// claudeGetProbeLiteralText is cmd/engram's own copy of
// internal/setup/claudecode_test.go's literal-header fixture: the two
// packages cannot share test code, so this is the SAME
// .planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md
// §"Claude Code — literal value" VERBATIM `claude mcp get` capture
// (claude 2.1.273, 2026-09-15), with every occurrence of the record's
// probe entry name "probe-literal-04" rewritten to "engram" (the only
// edit) — kept in sync by inspection (04-05-PLAN.md Task 3).
const claudeGetProbeLiteralText = `engram:
  Scope: User config (available in all your projects)
  Status: ✘ Failed to connect
  Issue: ConnectionRefused: Unable to connect. Is the computer able to access the url?
  Type: http
  URL: http://127.0.0.1:1/mcp
  Headers:
    x-litellm-api-key: sk-DO-NOT-COMMIT-literal-test-abc123

To remove this server, run: claude mcp remove engram -s user
`

// codexGetProbeLiteralJSON is cmd/engram's own copy of
// internal/setup/drift_test.go's codexObservedLiteralHeader fixture: the
// SAME .planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md
// §"Codex — literal header (hand-edited)" VERBATIM
// `codex mcp get probe-literal-04 --json` capture (codex-cli 0.154.0,
// 2026-09-15), with ONLY "name":"probe-literal-04" rewritten to
// "name":"engram" — kept in sync by inspection (04-05-PLAN.md Task 3).
const codexGetProbeLiteralJSON = `{
  "name": "engram",
  "enabled": true,
  "disabled_reason": null,
  "transport": {
    "type": "streamable_http",
    "url": "http://127.0.0.1:1/mcp",
    "bearer_token_env_var": "DUMMY_04",
    "http_headers": {
      "x-litellm-api-key": "sk-DO-NOT-COMMIT-literal-test-abc123"
    },
    "env_http_headers": null,
    "http_headers_helper": null
  },
  "enabled_tools": null,
  "disabled_tools": null,
  "startup_timeout_sec": null,
  "tool_timeout_sec": null
}`

// scriptedSetupRun returns a Run seam that responds to a `mcp get` probe
// (args[0]=="mcp", args[1]=="get" — codex's own probe shape, plan.go's
// Probe with the binary already stripped) with mcpGet, and to every
// other invocation — including the plugin probes — with a bare
// zero-exit RunResult, exactly like every existing preview test's
// default fake: the plugin probes then read as unavailable, so no
// plugin facet folds into a drift-focused test's assertions
// (04-04-PLAN.md Task 1).
func scriptedSetupRun(t *testing.T, mcpGet setup.RunResult) func(context.Context, string, []string) (setup.RunResult, error) {
	t.Helper()
	return func(_ context.Context, _ string, args []string) (setup.RunResult, error) {
		if len(args) >= 2 && args[0] == "mcp" && args[1] == "get" {
			return mcpGet, nil
		}
		return setup.RunResult{ExitCode: 0}, nil
	}
}

// defaultRuntimeCount returns the number of runtimes a BARE `engram
// setup` (no --runtime) targets — setup.Select(nil)'s own result length,
// derived from the live registry rather than hardcoded, so a test
// asserting a bare invocation's row count stays correct regardless of how
// many opt-in-only runtimes (03-04 D-14) the registry carries. This is
// deliberately NOT len(setup.Names()): Names() reports every REGISTERED
// runtime (including an opt-in one like generic), while a bare invocation
// only ever targets the DEFAULT SET Select(nil) resolves to.
func defaultRuntimeCount(t *testing.T) int {
	t.Helper()
	rts, err := setup.Select(nil)
	if err != nil {
		t.Fatalf("setup.Select(nil): %v", err)
	}
	return len(rts)
}

// TestSetupPreviewJSONHasClaudeCodeCommand proves `engram setup --output
// json` against a fake Environment resolving "claude" exits 0 and emits a
// document carrying a runtimes array with a claude-code element whose
// command field is the exact `claude mcp add` invocation.
func TestSetupPreviewJSONHasClaudeCodeCommand(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("claude"))

	stdout, stderr, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v (stdout=%q stderr=%q)", err, stdout, stderr)
	}

	var doc setupReportDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	if len(doc.Runtimes) == 0 {
		t.Fatalf("setup --output json emitted no runtimes: %s", stdout)
	}

	var found bool
	for _, row := range doc.Runtimes {
		if row.Name != "claude-code" {
			continue
		}
		found = true
		if !row.Present {
			t.Error("claude-code row Present = false, want true")
		}
		want := "claude mcp remove engram --scope user; claude mcp add --transport http engram https://engram.example.com/mcp --scope user"
		if row.Command != want {
			t.Errorf("claude-code row Command = %q, want %q", row.Command, want)
		}
	}
	if !found {
		t.Fatalf("setup --output json has no claude-code row: %s", stdout)
	}
}

// TestSetupPreviewJSONDeterministic proves two identical `engram setup
// --output json` invocations against the same fake Environment produce
// byte-identical stdout.
func TestSetupPreviewJSONDeterministic(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("claude"))

	stdout1, _, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--output", "json")
	if err != nil {
		t.Fatalf("runClient (1st): %v", err)
	}

	resetCommandFlagState(t, setupCmd)
	stdout2, _, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--output", "json")
	if err != nil {
		t.Fatalf("runClient (2nd): %v", err)
	}

	if stdout1 != stdout2 {
		t.Errorf("two identical setup --output json runs differ:\n%q\n%q", stdout1, stdout2)
	}
}

// TestSetupRejectsInvalidAuth proves `engram setup --auth basic` exits
// exitUsage (2).
func TestSetupRejectsInvalidAuth(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv())

	_, stderr, err := runClient(t, "setup", "--auth", "basic")
	if err == nil {
		t.Fatal("expected an error for --auth basic, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage); stderr=%q", got, exitUsage, stderr)
	}
}

// TestSetupPreviewExecutesNoRuntimeCLI proves a bare `engram setup` (no
// --apply) with a fake Environment resolves detection exactly once per
// registered runtime via the injected LookPath hook and never through any
// other path — there is no exec.Command call anywhere on the preview code
// path (Apply is stubbed, D-09), so this read-only resolution count is the
// entire "runtime CLI execution" surface this phase has.
func TestSetupPreviewExecutesNoRuntimeCLI(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)

	var lookupCalls int
	env := setup.Environment{
		LookPath: func(string) (string, error) {
			lookupCalls++
			return "", exec.ErrNotFound
		},
		Getenv:  func(string) string { return "" },
		HomeDir: func() (string, error) { return "/home/fake", nil },
	}
	withFakeSetupEnv(t, env)

	_, _, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v", err)
	}
	if want := defaultRuntimeCount(t); lookupCalls != want {
		t.Errorf("LookPath called %d times, want exactly %d (one read-only detection call per registered runtime, no execution)", lookupCalls, want)
	}
}

// TestSetupApplyRunsRealRegistrationSucceeds proves `engram setup --apply`
// against a fake Environment whose Run seam always succeeds performs a
// real registration (no panic, exit 0) for a present runtime — Apply() is
// no longer stubbed (D-09 retired this phase).
func TestSetupApplyRunsRealRegistrationSucceeds(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("claude"))

	_, stderr, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--apply")
	if err != nil {
		t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
	}
}

// TestSetupRejectsUnknownRuntime proves `engram setup --runtime nope`
// exits exitUsage (2) and stderr names all three valid runtimes.
func TestSetupRejectsUnknownRuntime(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv())

	_, stderr, err := runClient(t, "setup", "--runtime", "nope")
	if err == nil {
		t.Fatal("expected an error for --runtime nope, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage); stderr=%q", got, exitUsage, stderr)
	}
	for _, want := range []string{"nope", "claude-code", "codex", "opencode"} {
		if !strings.Contains(err.Error(), want) && !strings.Contains(stderr, want) {
			t.Errorf("engram setup --runtime nope: neither err (%q) nor stderr (%q) names %q", err, stderr, want)
		}
	}
}

// TestSetupRuntimeAbsentIsNotPresentRow proves `engram setup --runtime
// codex --output json` against a fake Environment where codex is ABSENT
// exits 0 and emits exactly one runtime row whose outcome is
// "not-present".
func TestSetupRuntimeAbsentIsNotPresentRow(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("claude")) // codex absent

	stdout, _, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--runtime", "codex", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v", err)
	}
	var doc setupReportDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	if len(doc.Runtimes) != 1 {
		t.Fatalf("setup --runtime codex emitted %d rows, want exactly 1: %s", len(doc.Runtimes), stdout)
	}
	row := doc.Runtimes[0]
	if row.Name != "codex" {
		t.Errorf("row.Name = %q, want %q", row.Name, "codex")
	}
	if row.Present {
		t.Error("row.Present = true, want false (codex is absent from the fake Environment)")
	}
	if row.Outcome != "not-present" {
		t.Errorf("row.Outcome = %q, want %q", row.Outcome, "not-present")
	}
}

// TestSetupAllThreeRuntimesPresentEmitsThreeRows proves a bare `engram
// setup --output json` against a fake Environment where all three
// runtimes are present emits three rows.
func TestSetupAllThreeRuntimesPresentEmitsThreeRows(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("claude", "codex", "opencode"))

	stdout, _, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v", err)
	}
	var doc setupReportDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	if len(doc.Runtimes) != 3 {
		t.Fatalf("setup --output json emitted %d rows, want exactly 3: %s", len(doc.Runtimes), stdout)
	}
}

// TestSetupRuntimeEnvDefaultReadsEnv proves ENGRAM_RUNTIME=codex yields
// ["codex"] from setupRuntimeEnvDefault — exercised directly since pflag
// defaults are bound at init() time, so t.Setenv after the binary has
// already started cannot retroactively change a live flag's default.
func TestSetupRuntimeEnvDefaultReadsEnv(t *testing.T) {
	t.Setenv("ENGRAM_RUNTIME", "codex")
	got := setupRuntimeEnvDefault()
	want := []string{"codex"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("setupRuntimeEnvDefault() = %v, want %v", got, want)
	}
}

// TestSetupBearerTokenFileRedactedInOutput proves `engram setup --auth
// bearer --token-file /tmp/does-not-exist --output json` exits 0, and no
// row's emitted command field ever contains a token-shaped value. Both
// runtimes exercised here now author bearer credentials through their own
// runtime-native substitution token rather than bearerProvenance's
// path-provenance placeholder: claude-code (D-05/D-06, 03-02) via a
// ${ENGRAM_TOKEN} shell-style reference, opencode (D-05/D-06, 03-03) via
// its {env:...} substitution token — neither carries the token-file path
// at all. This is a nonexistent path, so either runtime's Plan() reading
// it and leaking real content is structurally impossible, but the
// per-runtime assertions pin each corrected form itself.
func TestSetupBearerTokenFileRedactedInOutput(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("claude", "opencode"))

	stdout, _, err := runClient(t, "setup",
		"--url", "https://engram.example.com/mcp",
		"--auth", "bearer",
		"--token-file", "/tmp/does-not-exist",
		"--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v", err)
	}

	var doc setupReportDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	if len(doc.Runtimes) == 0 {
		t.Fatalf("setup --auth bearer emitted no runtimes: %s", stdout)
	}
	for _, row := range doc.Runtimes {
		if row.Outcome != "would-write" {
			continue
		}
		if strings.Contains(row.Command, "Bearer eyJ") {
			t.Errorf("%s row.Command = %q, want it to NOT contain a token-shaped value", row.Name, row.Command)
		}
		if strings.Contains(row.Command, "/tmp/does-not-exist") {
			t.Errorf("%s row.Command = %q, want it to NOT contain the token-file path (D-05/D-06: names ENGRAM_TOKEN, never a path)", row.Name, row.Command)
		}
		if !strings.Contains(row.Command, "ENGRAM_TOKEN") {
			t.Errorf("%s row.Command = %q, want it to name ENGRAM_TOKEN via its own runtime-native substitution token", row.Name, row.Command)
		}
	}
}

// TestSetupTokenFileMarkedIgnoredForNativeRuntimes proves `--token-file`
// with every runtime selected marks each NATIVE runtime's row with the
// D-07 token_file=ignored marker — in BOTH the preview and the apply lane
// — while the generic row carries no such marker (D-06: --token-file
// genuinely applies to generic's portable config).
func TestSetupTokenFileMarkedIgnoredForNativeRuntimes(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"preview", []string{"setup", "--url", "https://engram.example.com/mcp", "--auth", "bearer", "--token-file", "/tmp/does-not-exist", "--runtime", "claude-code,codex,opencode,generic", "--output", "json"}},
		{"apply", []string{"setup", "--url", "https://engram.example.com/mcp", "--auth", "bearer", "--token-file", "/tmp/does-not-exist", "--runtime", "claude-code,codex,opencode,generic", "--output", "json", "--apply"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetClientFlags(t)
			resetCommandFlagState(t, setupCmd)
			withFakeSetupEnv(t, fakeSetupEnv("claude", "codex", "opencode"))

			stdout, stderr, _ := runClient(t, tc.args...)

			var doc setupReportDoc
			if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
				t.Fatalf("json.Unmarshal(%q): %v (stderr=%q)", stdout, err, stderr)
			}
			if len(doc.Runtimes) != 4 {
				t.Fatalf("setup emitted %d rows, want exactly 4: %s", len(doc.Runtimes), stdout)
			}
			for _, row := range doc.Runtimes {
				if row.Name == "generic" {
					if row.TokenFile != "" {
						t.Errorf("generic row.TokenFile = %q, want empty — --token-file genuinely applies to the portable config (D-06)", row.TokenFile)
					}
					continue
				}
				if row.TokenFile == "" {
					t.Errorf("%s row.TokenFile is empty, want the D-07 marker — a native runtime's own row must say --token-file did not apply", row.Name)
				}
			}
		})
	}
}

// TestSetupNoTokenFileLeavesNoMarker proves that with NO --token-file
// supplied, no row carries a token_file field at all — an omitempty field
// that always renders is the fails-by-absence shape D-07 exists to
// prevent, in reverse.
func TestSetupNoTokenFileLeavesNoMarker(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("claude", "codex", "opencode"))

	stdout, stderr, err := runClient(t, "setup",
		"--url", "https://engram.example.com/mcp",
		"--runtime", "claude-code,codex,opencode,generic",
		"--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
	}

	var raw struct {
		Runtimes []map[string]any `json:"runtimes"`
	}
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	for _, row := range raw.Runtimes {
		if _, ok := row["token_file"]; ok {
			t.Errorf("row %v carries a token_file field though --token-file was never supplied", row)
		}
	}
}

// TestSetupTokenFilePathNotDuplicatedIntoMarker proves the D-07 marker
// value never contains the supplied --token-file PATH — the path renders
// only where it means something (generic's provenance form), never
// duplicated into a native runtime's "ignored" marker.
func TestSetupTokenFilePathNotDuplicatedIntoMarker(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("claude"))

	const tokenPath = "/tmp/does-not-exist-token-path"
	stdout, stderr, err := runClient(t, "setup",
		"--url", "https://engram.example.com/mcp",
		"--auth", "bearer",
		"--token-file", tokenPath,
		"--runtime", "claude-code",
		"--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
	}

	var doc setupReportDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	if len(doc.Runtimes) != 1 {
		t.Fatalf("setup --runtime claude-code emitted %d rows, want exactly 1: %s", len(doc.Runtimes), stdout)
	}
	row := doc.Runtimes[0]
	if row.TokenFile == "" {
		t.Fatal("claude-code row.TokenFile is empty, want the D-07 marker")
	}
	if strings.Contains(row.TokenFile, tokenPath) {
		t.Errorf("claude-code row.TokenFile = %q, want it to NOT contain the supplied path %q", row.TokenFile, tokenPath)
	}
}

// TestSetupUnsupportedAuthModeIsFailedRow proves `engram setup --auth
// oauth-client --runtime opencode --output json` exits 0 and emits one
// row whose outcome is "failed" and whose reason names both "opencode"
// and "oauth-client".
func TestSetupUnsupportedAuthModeIsFailedRow(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("opencode"))

	stdout, _, err := runClient(t, "setup",
		"--url", "https://engram.example.com/mcp",
		"--auth", "oauth-client",
		"--client-id", "test-client",
		"--runtime", "opencode",
		"--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v", err)
	}

	var doc setupReportDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	if len(doc.Runtimes) != 1 {
		t.Fatalf("setup --runtime opencode emitted %d rows, want exactly 1: %s", len(doc.Runtimes), stdout)
	}
	row := doc.Runtimes[0]
	if row.Outcome != "failed" {
		t.Errorf("row.Outcome = %q, want %q", row.Outcome, "failed")
	}
	for _, want := range []string{"opencode", "oauth-client"} {
		if !strings.Contains(row.Reason, want) {
			t.Errorf("row.Reason = %q, want it to name %q", row.Reason, want)
		}
	}
}

// TestSetupApplyAllAbsentExitsZero proves `engram setup --apply` against a
// fake Environment where every selected runtime is absent exits 0 and
// every row's outcome is not-present (D-07): nothing was attempted, so
// nothing can have failed.
func TestSetupApplyAllAbsentExitsZero(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv())

	stdout, stderr, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--apply", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v (stdout=%q stderr=%q)", err, stdout, stderr)
	}

	var doc setupReportDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	if want := defaultRuntimeCount(t); len(doc.Runtimes) != want {
		t.Fatalf("setup --apply emitted %d rows, want %d: %s", len(doc.Runtimes), want, stdout)
	}
	for _, row := range doc.Runtimes {
		if row.Outcome != string(setup.OutcomeNotPresent) {
			t.Errorf("%s row.Outcome = %q, want %q", row.Name, row.Outcome, "not-present")
		}
	}
}

// TestSetupApplyAtLeastOnePresentExitsSetupFailed proves `engram setup
// --apply` against a fake Environment with at least one runtime present
// exits exitSetupFailed (9), and that the SAME run's captured stdout still
// carries a report row for every selected runtime (T-02-06): a nonzero
// exit must never erase the per-runtime record of what happened.
func TestSetupApplyAtLeastOnePresentExitsSetupFailed(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	failingRun := func(context.Context, string, []string) (setup.RunResult, error) {
		return setup.RunResult{ExitCode: 1, Stderr: "boom: mcp add failed"}, nil
	}
	withFakeSetupEnv(t, fakeSetupEnvWithRun(failingRun, "claude"))

	stdout, stderr, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--apply", "--output", "json")
	if err == nil {
		t.Fatal("expected a non-nil error for --apply with at least one runtime present, got nil")
	}
	if got := exitCodeFromError(err); got != exitSetupFailed {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitSetupFailed); stderr=%q", got, exitSetupFailed, stderr)
	}
	if stdout == "" {
		t.Fatal("stdout is empty; the report must be rendered before the nonzero exit (T-02-06)")
	}
	var doc setupReportDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	if want := defaultRuntimeCount(t); len(doc.Runtimes) != want {
		t.Fatalf("setup --apply emitted %d rows, want %d (one per selected runtime, none collapsed): %s", len(doc.Runtimes), want, stdout)
	}
	var found bool
	for _, row := range doc.Runtimes {
		if row.Name != "claude-code" {
			continue
		}
		found = true
		if row.Outcome != string(setup.OutcomeFailed) {
			t.Errorf("claude-code row.Outcome = %q, want %q", row.Outcome, "failed")
		}
		if row.Reason == "" {
			t.Error("claude-code row.Reason is empty, want it to name why the attempt failed")
		}
	}
	if !found {
		t.Fatalf("setup --apply has no claude-code row: %s", stdout)
	}
}

// TestSetupPreviewExitsZeroRegardlessOfPresence proves a bare `engram
// setup` (no --apply) exits 0 whether every selected runtime is present or
// every one is absent (D-08): a preview never fails on a detection
// outcome, only on a usage/config error.
func TestSetupPreviewExitsZeroRegardlessOfPresence(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  setup.Environment
	}{
		{"all-absent", fakeSetupEnv()},
		{"all-present", fakeSetupEnv("claude", "codex", "opencode")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetClientFlags(t)
			resetCommandFlagState(t, setupCmd)
			withFakeSetupEnv(t, tc.env)

			_, stderr, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--output", "json")
			if err != nil {
				t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
			}
		})
	}
}

// TestSetupPreviewExitsZeroWhenProbeFails is the behavioral replacement
// proof for T-02-08 (03-05-PLAN.md Task 1): now that setup.Preview starts a
// real child process for a present runtime's Plan.Probe (D-10), the
// structural argument 02-SECURITY.md's T-02-08 originally rested on
// ("setupPreview starts no process") no longer holds, and the exit-code
// property must be re-pinned on its own terms. Both subtests assert the
// command returns a nil error AND that the rendered report still carries
// one row per selected runtime (T-02-06: a probe failure must never erase
// the per-runtime record).
func TestSetupPreviewExitsZeroWhenProbeFails(t *testing.T) {
	t.Run("nonzero-probe-exit", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		failingProbe := func(context.Context, string, []string) (setup.RunResult, error) {
			return setup.RunResult{ExitCode: 1, Stderr: "No MCP server named 'engram' found"}, nil
		}
		withFakeSetupEnv(t, fakeSetupEnvWithRun(failingProbe, "claude"))

		stdout, stderr, err := runClient(t, "setup",
			"--url", "https://engram.example.com/mcp", "--runtime", "claude-code", "--output", "json")
		if err != nil {
			t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
		}
		var doc setupReportDoc
		if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
		}
		if len(doc.Runtimes) != 1 {
			t.Fatalf("setup preview emitted %d rows, want exactly 1 (a probe failure must not erase the row): %s", len(doc.Runtimes), stdout)
		}
		if doc.Runtimes[0].Facets != "" {
			t.Errorf("Facets = %q, want empty (D-09: a failed probe is ambiguous, never compared)", doc.Runtimes[0].Facets)
		}
		if doc.Runtimes[0].Registered != "" {
			t.Errorf("Registered = %q, want empty (D-09: a failed probe is ambiguous, never compared)", doc.Runtimes[0].Registered)
		}
	})

	t.Run("probe-seam-error", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		seamErrorRun := func(context.Context, string, []string) (setup.RunResult, error) {
			return setup.RunResult{}, errors.New("exec: start failure")
		}
		withFakeSetupEnv(t, fakeSetupEnvWithRun(seamErrorRun, "claude"))

		stdout, stderr, err := runClient(t, "setup",
			"--url", "https://engram.example.com/mcp", "--runtime", "claude-code", "--output", "json")
		if err != nil {
			t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
		}
		var doc setupReportDoc
		if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
		}
		if len(doc.Runtimes) != 1 {
			t.Fatalf("setup preview emitted %d rows, want exactly 1 (a probe seam error must not erase the row): %s", len(doc.Runtimes), stdout)
		}
		if doc.Runtimes[0].Facets != "" {
			t.Errorf("Facets = %q, want empty (D-09: a probe seam error is ambiguous, never compared)", doc.Runtimes[0].Facets)
		}
		if doc.Runtimes[0].Registered != "" {
			t.Errorf("Registered = %q, want empty (D-09: a probe seam error is ambiguous, never compared)", doc.Runtimes[0].Registered)
		}
	})
}

// TestSetupPreviewNeverClassifiesAlreadyCorrectFromAmbiguousRead used to
// pin "a bare preview never classifies already-correct" outright — Phase
// 4 overturns that for a PARSED claude-code/codex registration
// (TestSetupPreviewJSONCarriesDriftFacets/codex-already-correct covers
// that positive case now). What this retargeted test pins is D-09/D-10:
// an AMBIGUOUS read — one the scanner cannot frame as a registration at
// all — never yields already-correct or preserved, however convincing it
// looks, and opencode is never compared, full stop.
func TestSetupPreviewNeverClassifiesAlreadyCorrectFromAmbiguousRead(t *testing.T) {
	t.Run("ambiguous-read-every-runtime", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		identicalProbe := func(context.Context, string, []string) (setup.RunResult, error) {
			return setup.RunResult{Stdout: "engram: https://engram.example.com/mcp (HTTP)"}, nil
		}
		withFakeSetupEnv(t, fakeSetupEnvWithRun(identicalProbe, "claude", "codex", "opencode"))

		stdout, stderr, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--output", "json")
		if err != nil {
			t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
		}
		var doc setupReportDoc
		if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
		}
		if len(doc.Runtimes) == 0 {
			t.Fatalf("setup preview emitted no runtimes: %s", stdout)
		}
		for _, row := range doc.Runtimes {
			if row.Outcome == string(setup.OutcomeAlreadyCorrect) || row.Outcome == string(setup.OutcomePreserved) {
				t.Errorf("%s row.Outcome = %q, want never %q or %q under an ambiguous read (D-09)", row.Name, row.Outcome, setup.OutcomeAlreadyCorrect, setup.OutcomePreserved)
			}
			if row.Registration != string(setup.OutcomeWouldWrite) {
				t.Errorf("%s row.Registration = %q, want %q", row.Name, row.Registration, setup.OutcomeWouldWrite)
			}
			if row.Facets != "" {
				t.Errorf("%s row.Facets = %q, want empty", row.Name, row.Facets)
			}
			if row.Registered != "" {
				t.Errorf("%s row.Registered = %q, want empty (D-03: an unframeable read renders nothing)", row.Name, row.Registered)
			}
		}
	})

	t.Run("opencode-never-compared", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		// A convincing-looking box-drawing table naming engram and its URL
		// with a connected glyph line — opencode's mcp list shape (D-10) —
		// must still never be compared, whatever it says.
		table := "┌────────┬─────────────────────────────────┬───────────┐\n" +
			"│ name   │ url                             │ status    │\n" +
			"├────────┼─────────────────────────────────┼───────────┤\n" +
			"│ engram │ https://engram.example.com/mcp │ connected │\n" +
			"└────────┴─────────────────────────────────┴───────────┘\n"
		convincingProbe := func(context.Context, string, []string) (setup.RunResult, error) {
			return setup.RunResult{Stdout: table}, nil
		}
		withFakeSetupEnv(t, fakeSetupEnvWithRun(convincingProbe, "opencode"))

		stdout, stderr, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp",
			"--runtime", "opencode", "--output", "json")
		if err != nil {
			t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
		}
		var doc setupReportDoc
		if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
		}
		row := rowByName(t, doc.Runtimes, "opencode")
		if row.Outcome != string(setup.OutcomeWouldWrite) {
			t.Errorf("Outcome = %q, want %q", row.Outcome, setup.OutcomeWouldWrite)
		}
		if row.Facets != "" {
			t.Errorf("Facets = %q, want empty", row.Facets)
		}
		if row.Registered != "" {
			t.Errorf("Registered = %q, want empty", row.Registered)
		}
		wantDrift := "opencode: not compared: runtime authors no registration scanner"
		if row.Drift != wantDrift {
			t.Errorf("Drift = %q, want %q", row.Drift, wantDrift)
		}
	})
}

// TestSetupPreviewJSONCarriesDriftFacets proves facets/drift/preserved
// ride setupRuntimeRow in both output lanes, composed straight from
// internal/setup's Result.Facets/Result.Drift/Result.Reason (04-01): a
// would-write URL diff, a preserved unrecognized-content registration
// (first-class in the raw JSON lane too), an already-correct registration
// (facets/drift both empty and omitted from JSON), and the flat fields
// rendered in the text lane (04-04-PLAN.md Task 1).
func TestSetupPreviewJSONCarriesDriftFacets(t *testing.T) {
	t.Run("codex-would-write-url", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		stdout := strings.Replace(codexGetEngramBearerJSON,
			`"url":"https://engram.example.com/mcp"`, `"url":"https://old.example/mcp"`, 1)
		withFakeSetupEnv(t, fakeSetupEnvWithRun(scriptedSetupRun(t, setup.RunResult{Stdout: stdout}), "codex"))

		out, stderr, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp",
			"--auth", "bearer", "--runtime", "codex", "--output", "json")
		if err != nil {
			t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
		}
		var doc setupReportDoc
		if uErr := json.Unmarshal([]byte(out), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal(%q): %v", out, uErr)
		}
		row := rowByName(t, doc.Runtimes, "codex")
		if row.Outcome != string(setup.OutcomeWouldWrite) {
			t.Errorf("Outcome = %q, want %q", row.Outcome, setup.OutcomeWouldWrite)
		}
		if row.Registration != string(setup.OutcomeWouldWrite) {
			t.Errorf("Registration = %q, want %q", row.Registration, setup.OutcomeWouldWrite)
		}
		if row.Facets != "url" {
			t.Errorf("Facets = %q, want %q", row.Facets, "url")
		}
		wantDrift := "url: observed https://old.example/mcp, would write https://engram.example.com/mcp"
		if row.Drift != wantDrift {
			t.Errorf("Drift = %q, want %q", row.Drift, wantDrift)
		}
		wantRegistered := "url=https://old.example/mcp auth=bearer headers=none"
		if row.Registered != wantRegistered {
			t.Errorf("Registered = %q, want %q", row.Registered, wantRegistered)
		}
		if row.Reason != "" {
			t.Errorf("Reason = %q, want empty", row.Reason)
		}
		assertFoldedOutcome(t, row)
	})

	t.Run("codex-preserved-unrecognized-field", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		stdout := strings.Replace(codexGetEngramBearerJSON,
			`"name":"engram"`, `"name":"engram","oauth_client_id":"x"`, 1)
		withFakeSetupEnv(t, fakeSetupEnvWithRun(scriptedSetupRun(t, setup.RunResult{Stdout: stdout}), "codex"))

		out, stderr, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp",
			"--auth", "bearer", "--runtime", "codex", "--output", "json")
		if err != nil {
			t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
		}
		if !strings.Contains(out, `"outcome":"preserved"`) {
			t.Errorf("raw JSON does not carry outcome:preserved: %s", out)
		}
		var doc setupReportDoc
		if uErr := json.Unmarshal([]byte(out), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal(%q): %v", out, uErr)
		}
		row := rowByName(t, doc.Runtimes, "codex")
		if row.Outcome != string(setup.OutcomePreserved) {
			t.Errorf("Outcome = %q, want %q", row.Outcome, setup.OutcomePreserved)
		}
		if row.Registration != string(setup.OutcomePreserved) {
			t.Errorf("Registration = %q, want %q", row.Registration, setup.OutcomePreserved)
		}
		if row.Facets != "unrecognized-content" {
			t.Errorf("Facets = %q, want %q", row.Facets, "unrecognized-content")
		}
		if row.Drift != "unrecognized-content: oauth_client_id" {
			t.Errorf("Drift = %q, want %q", row.Drift, "unrecognized-content: oauth_client_id")
		}
		wantPrefix := "codex: preserved: unrecognized-content: oauth_client_id; "
		if !strings.HasPrefix(row.Reason, wantPrefix) {
			t.Errorf("Reason = %q, want prefix %q", row.Reason, wantPrefix)
		}
		if !strings.HasSuffix(row.Reason, "never merge into it") {
			t.Errorf("Reason = %q, want suffix %q", row.Reason, "never merge into it")
		}
		if !strings.HasSuffix(row.Registered, "unrecognized=oauth_client_id") {
			t.Errorf("Registered = %q, want suffix %q", row.Registered, "unrecognized=oauth_client_id")
		}
		assertFoldedOutcome(t, row)
	})

	t.Run("codex-already-correct", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		withFakeSetupEnv(t, fakeSetupEnvWithRun(scriptedSetupRun(t, setup.RunResult{Stdout: codexGetEngramBearerJSON}), "codex"))

		out, stderr, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp",
			"--auth", "bearer", "--runtime", "codex", "--output", "json")
		if err != nil {
			t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
		}
		var doc setupReportDoc
		if uErr := json.Unmarshal([]byte(out), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal(%q): %v", out, uErr)
		}
		row := rowByName(t, doc.Runtimes, "codex")
		if row.Registration != string(setup.OutcomeAlreadyCorrect) {
			t.Errorf("Registration = %q, want %q", row.Registration, setup.OutcomeAlreadyCorrect)
		}
		if row.Outcome != string(setup.OutcomeAlreadyCorrect) {
			t.Errorf("Outcome = %q, want %q (already-correct outranks would-write skills)", row.Outcome, setup.OutcomeAlreadyCorrect)
		}
		if row.Facets != "" {
			t.Errorf("Facets = %q, want empty", row.Facets)
		}
		if row.Drift != "" {
			t.Errorf("Drift = %q, want empty", row.Drift)
		}
		wantRegistered := "url=https://engram.example.com/mcp auth=bearer headers=none"
		if row.Registered != wantRegistered {
			t.Errorf("Registered = %q, want %q", row.Registered, wantRegistered)
		}

		var raw struct {
			Runtimes []json.RawMessage `json:"runtimes"`
		}
		if uErr := json.Unmarshal([]byte(out), &raw); uErr != nil {
			t.Fatalf("json.Unmarshal(raw): %v", uErr)
		}
		if len(raw.Runtimes) != 1 {
			t.Fatalf("want exactly 1 runtime row, got %d", len(raw.Runtimes))
		}
		var rowMap map[string]any
		if uErr := json.Unmarshal(raw.Runtimes[0], &rowMap); uErr != nil {
			t.Fatalf("json.Unmarshal(row): %v", uErr)
		}
		if _, ok := rowMap["facets"]; ok {
			t.Errorf("row JSON carries a facets key for an already-correct row: %v", rowMap)
		}
	})

	t.Run("text-lane-renders-flat-fields", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		stdout := strings.Replace(codexGetEngramBearerJSON,
			`"name":"engram"`, `"name":"engram","oauth_client_id":"x"`, 1)
		withFakeSetupEnv(t, fakeSetupEnvWithRun(scriptedSetupRun(t, setup.RunResult{Stdout: stdout}), "codex"))

		out, stderr, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp",
			"--auth", "bearer", "--runtime", "codex", "--output", "text")
		if err != nil {
			t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
		}
		for _, want := range []string{"outcome=preserved", "facets=unrecognized-content", "registration=preserved"} {
			if !strings.Contains(out, want) {
				t.Errorf("text output does not contain %q: %s", want, out)
			}
		}
	})
}

// TestSetupJSONNeverLeaksProbeLiteral is the process-boundary mirror of
// internal/setup's TestRedactionUnconditional (04-01-SUMMARY.md): a
// sentinel ("SENTINEL-VALUE-cmd-7c1d4b-DO-NOT-LEAK") planted where codex
// would carry the bearer env var name must never reach engram setup's
// stdout or stderr, in EITHER output lane. 04-01's redaction-by-
// construction already holds this by construction — this test is GREEN
// on first run, pinning that existing behavior at the CLI process
// boundary rather than only inside internal/setup. Plan 04-05 extends
// this with the observed literal-echo shapes from 04-OBSERVATIONS.md
// (04-04-PLAN.md Task 2).
func TestSetupJSONNeverLeaksProbeLiteral(t *testing.T) {
	const sentinel = "SENTINEL-VALUE-cmd-7c1d4b-DO-NOT-LEAK"

	for _, lane := range []string{"json", "text"} {
		t.Run(lane, func(t *testing.T) {
			resetClientFlags(t)
			resetCommandFlagState(t, setupCmd)
			stdout := strings.Replace(codexGetEngramBearerJSON,
				`"bearer_token_env_var":"ENGRAM_TOKEN"`, `"bearer_token_env_var":"`+sentinel+`"`, 1)
			withFakeSetupEnv(t, fakeSetupEnvWithRun(scriptedSetupRun(t, setup.RunResult{Stdout: stdout}), "codex"))

			out, errOut, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp",
				"--auth", "oauth", "--runtime", "codex", "--output", lane)
			if err != nil {
				t.Fatalf("runClient: %v (stderr=%q)", err, errOut)
			}
			if strings.Contains(out, sentinel) {
				t.Errorf("%s stdout leaks the probe sentinel: %s", lane, out)
			}
			if strings.Contains(errOut, sentinel) {
				t.Errorf("%s stderr leaks the probe sentinel: %s", lane, errOut)
			}

			if lane == "json" {
				var doc setupReportDoc
				if uErr := json.Unmarshal([]byte(out), &doc); uErr != nil {
					t.Fatalf("json.Unmarshal(%q): %v", out, uErr)
				}
				row := rowByName(t, doc.Runtimes, "codex")
				if row.Outcome != string(setup.OutcomePreserved) {
					t.Errorf("Outcome = %q, want %q", row.Outcome, setup.OutcomePreserved)
				}
				wantRegistered := "url=https://engram.example.com/mcp auth=foreign headers=none"
				if row.Registered != wantRegistered {
					t.Errorf("Registered = %q, want %q", row.Registered, wantRegistered)
				}
			}
		})
	}

	// claude-code-observed-shape and codex-observed-shape (04-05-PLAN.md
	// Task 3) are the SC5 fixture proof at the CLI process boundary
	// against the OBSERVED shapes .planning/phases/04-drift-detection-read-only/
	// 04-OBSERVATIONS.md recorded, rather than an assumed one — extending
	// this test beyond the (still-present) synthetic sentinel case above.
	const observedLiteral = "sk-DO-NOT-COMMIT-literal-test-abc123"

	t.Run("claude-code-observed-shape", func(t *testing.T) {
		for _, lane := range []string{"json", "text"} {
			t.Run(lane, func(t *testing.T) {
				resetClientFlags(t)
				resetCommandFlagState(t, setupCmd)
				withFakeSetupEnv(t, fakeSetupEnvWithRun(
					scriptedSetupRun(t, setup.RunResult{Stdout: claudeGetProbeLiteralText, ExitCode: 0}), "claude"))

				out, errOut, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp",
					"--auth", "oauth", "--runtime", "claude-code", "--output", lane)
				if err != nil {
					t.Fatalf("runClient: %v (stderr=%q)", err, errOut)
				}
				if strings.Contains(out, observedLiteral) {
					t.Errorf("%s stdout leaks the observed literal: %s", lane, out)
				}
				if strings.Contains(errOut, observedLiteral) {
					t.Errorf("%s stderr leaks the observed literal: %s", lane, errOut)
				}

				if lane == "json" {
					var doc setupReportDoc
					if uErr := json.Unmarshal([]byte(out), &doc); uErr != nil {
						t.Fatalf("json.Unmarshal(%q): %v", out, uErr)
					}
					row := rowByName(t, doc.Runtimes, "claude-code")
					if row.Outcome != string(setup.OutcomePreserved) {
						t.Errorf("Outcome = %q, want %q", row.Outcome, setup.OutcomePreserved)
					}
					if !strings.Contains(row.Facets, "header-name") {
						t.Errorf("Facets = %q, want it to contain %q", row.Facets, "header-name")
					}
				}
			})
		}
	})

	t.Run("codex-observed-shape", func(t *testing.T) {
		for _, lane := range []string{"json", "text"} {
			t.Run(lane, func(t *testing.T) {
				resetClientFlags(t)
				resetCommandFlagState(t, setupCmd)
				withFakeSetupEnv(t, fakeSetupEnvWithRun(
					scriptedSetupRun(t, setup.RunResult{Stdout: codexGetProbeLiteralJSON}), "codex"))

				out, errOut, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp",
					"--auth", "oauth", "--runtime", "codex", "--output", lane)
				if err != nil {
					t.Fatalf("runClient: %v (stderr=%q)", err, errOut)
				}
				if strings.Contains(out, observedLiteral) {
					t.Errorf("%s stdout leaks the observed literal: %s", lane, out)
				}
				if strings.Contains(errOut, observedLiteral) {
					t.Errorf("%s stderr leaks the observed literal: %s", lane, errOut)
				}

				if lane == "json" {
					var doc setupReportDoc
					if uErr := json.Unmarshal([]byte(out), &doc); uErr != nil {
						t.Fatalf("json.Unmarshal(%q): %v", out, uErr)
					}
					row := rowByName(t, doc.Runtimes, "codex")
					if row.Outcome != string(setup.OutcomePreserved) {
						t.Errorf("Outcome = %q, want %q", row.Outcome, setup.OutcomePreserved)
					}
					if !strings.Contains(row.Facets, "header-name") {
						t.Errorf("Facets = %q, want it to contain %q", row.Facets, "header-name")
					}
				}
			})
		}
	})
}

// TestSetupApplySummaryCountsPreserved pins setupApplySummary's preserved
// bucket (D-04: preserved is reported, never counted as failed).
func TestSetupApplySummaryCountsPreserved(t *testing.T) {
	rows := []setupRuntimeRow{
		{Name: "a", Outcome: string(setup.OutcomeWrote)},
		{Name: "b", Outcome: string(setup.OutcomeAlreadyCorrect)},
		{Name: "c", Outcome: string(setup.OutcomePreserved)},
		{Name: "d", Outcome: string(setup.OutcomeFailed)},
		{Name: "e", Outcome: string(setup.OutcomeNotPresent)},
	}
	got := setupApplySummary(rows)
	want := "apply: 1 wrote, 1 already correct, 1 preserved, 1 failed (of 5 selected runtime(s))"
	if got != want {
		t.Errorf("setupApplySummary(...) = %q, want %q", got, want)
	}
}

// TestSetupPreviewSummaryNamesComparison pins setupPreviewSummary's
// comparison wording (REQ-setup-correct-by-reading).
func TestSetupPreviewSummaryNamesComparison(t *testing.T) {
	got := setupPreviewSummary([]setupRuntimeRow{{Present: true}})
	for _, want := range []string{"compared with what setup would write", "opencode is not compared"} {
		if !strings.Contains(got, want) {
			t.Errorf("setupPreviewSummary(...) = %q, does not contain %q", got, want)
		}
	}
}

// TestSetupApplyJSONEmitsPerRuntimeOutcome proves `engram setup --output
// json --apply` against a mixed fake Environment emits a runtimes array
// with exactly one element per selected runtime, each carrying its own
// outcome field — no aggregate count field ever replaces the per-runtime
// rows.
func TestSetupApplyJSONEmitsPerRuntimeOutcome(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("claude", "codex")) // opencode absent

	stdout, _, _ := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--output", "json", "--apply")

	var raw map[string]any
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	if _, ok := raw["runtimes"]; !ok {
		t.Fatalf("emitted document has no runtimes field: %s", stdout)
	}
	for _, aggregateKey := range []string{"count", "failed_count", "present_count", "summary_count"} {
		if _, ok := raw[aggregateKey]; ok {
			t.Errorf("emitted document has an aggregate field %q; no runtime's outcome may be collapsed into a count", aggregateKey)
		}
	}

	var doc setupReportDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	if want := defaultRuntimeCount(t); len(doc.Runtimes) != want {
		t.Fatalf("setup --output json --apply emitted %d rows, want %d: %s", len(doc.Runtimes), want, stdout)
	}
	for _, row := range doc.Runtimes {
		if row.Outcome == "" {
			t.Errorf("%s row has an empty outcome", row.Name)
		}
	}
}

// TestSetupExitCodes is the direct three-case table over setupExitCode's
// mapping function.
func TestSetupExitCodes(t *testing.T) {
	cases := []struct {
		class setup.ExitClass
		want  int
	}{
		{setup.ExitTotalSuccess, exitOK},
		{setup.ExitPartial, exitPartial},
		{setup.ExitTotalFailure, exitSetupFailed},
	}
	for _, c := range cases {
		if got := setupExitCode(c.class); got != c.want {
			t.Errorf("setupExitCode(%v) = %d, want %d", c.class, got, c.want)
		}
	}
}

// TestSetupHelpNamesEveryRuntimeAndAuthMode is the golden-backed
// assertion that `engram setup --help`'s help.golden section names each
// of claude-code, codex, opencode, oauth, oauth-client, bearer, and none,
// and states what --apply does. 02-03-PLAN.md Task 2 extends this with
// the --header paragraph's own vocabulary (REQ-header-documented).
func TestSetupHelpNamesEveryRuntimeAndAuthMode(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "help.golden"))
	if err != nil {
		t.Fatalf("read help.golden: %v", err)
	}
	section := helpGoldenSection(t, string(data), "engram setup")
	for _, want := range []string{
		"claude-code", "codex", "opencode",
		"oauth", "oauth-client", "bearer", "none",
		"client-id", "non-secret client ID", "Other auth modes reject --client-id",
		"MCP_CLIENT_SECRET", "inherited environment", "no interactive stdin",
		"apply",
		"--header", "NAME=ENVVAR", "ENGRAM_HEADERS", "x-gateway-api-key=GATEWAY_KEY",
		"--bearer-token-env-var", "never a value", "owned by --auth",
	} {
		if !strings.Contains(section, want) {
			t.Errorf("## engram setup section does not contain %q:\n%s", want, section)
		}
	}
}

// TestSetupHelpNamesDriftOutcomes proves setupCmd.Long teaches the Phase
// 4 comparison — the three classifications, the four differing facets,
// what preserved means, that header values read from a runtime are
// never shown, and that an unreadable registration reads would-write —
// and that testdata/help.golden proves the paragraph shipped, not only
// the in-memory Long (04-04-PLAN.md Task 1).
func TestSetupHelpNamesDriftOutcomes(t *testing.T) {
	lower := strings.ToLower(setupCmd.Long)
	for _, want := range []string{
		"preserved", "already-correct", "would-write", "facets", "drift",
		"url", "auth-mode", "header-name", "header-value-ref",
		"never shown", "not compared", "cannot reproduce",
	} {
		if !strings.Contains(lower, want) {
			t.Errorf("setup long description does not mention %q: %s", want, setupCmd.Long)
		}
	}

	data, err := os.ReadFile(filepath.Join("testdata", "help.golden"))
	if err != nil {
		t.Fatalf("read help.golden: %v", err)
	}
	section := helpGoldenSection(t, string(data), "engram setup")
	for _, want := range []string{"preserved", "not compared"} {
		if !strings.Contains(section, want) {
			t.Errorf("help.golden's engram setup section does not contain %q:\n%s", want, section)
		}
	}
}

// TestSetupHelpNamesSkillsInstallation proves setupCmd's long description
// (REQ-setup-correct-by-reading, success criterion 5) states that
// --apply installs the curation skills in addition to registering the
// MCP server, and that the skills are carried inside the binary itself —
// while naming no per-runtime destination path segment, so a destination
// is stated in exactly one place: the runtime's own file. The forbidden
// segments are derived from each registered runtime's own authored
// SkillTarget via Plan() against a fake environment, never hardcoded, so
// this assertion cannot go stale as runtimes are added or their
// destinations change.
func TestSetupHelpNamesSkillsInstallation(t *testing.T) {
	long := setupCmd.Long
	lower := strings.ToLower(long)
	for _, want := range []string{"skill", "binary"} {
		if !strings.Contains(lower, want) {
			t.Errorf("setup long description does not mention %q: %s", want, long)
		}
	}

	const fakeHome = "/home/fake-setup-help-test"
	fakeEnv := setup.Environment{
		LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		Getenv:   func(string) string { return "" },
		HomeDir:  func() (string, error) { return fakeHome, nil },
	}
	for _, rt := range setup.Runtimes {
		plan, err := rt.Plan(fakeEnv, setup.Options{URL: "https://engram.example.com/mcp", Auth: "oauth"})
		if err != nil {
			t.Fatalf("%s.Plan: %v", rt.Name(), err)
		}
		for _, dest := range []string{plan.Skills.Dir, plan.Skills.IndexFile} {
			suffix := strings.TrimPrefix(dest, fakeHome)
			if suffix == "" || suffix == dest {
				// Empty destination (generic's no-destination format) or
				// HomeDir was not this destination's prefix — nothing to
				// check.
				continue
			}
			for _, segment := range strings.Split(suffix, string(filepath.Separator)) {
				if segment == "" {
					continue
				}
				forbidden := string(filepath.Separator) + segment
				if strings.Contains(long, forbidden) {
					t.Errorf("%s: setup long description contains destination path segment %q derived from Plan().Skills (%q): %s",
						rt.Name(), forbidden, dest, long)
				}
			}
		}
	}
}

// TestSetupURLFromEnvReachesCommand proves ENGRAM_URL, with no --url flag,
// reaches the previewed command for every present runtime (CR-01): the
// environment lane setupPlanDoc's config.Load(cmd.Flags()) call wires.
func TestSetupURLFromEnvReachesCommand(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("claude", "codex"))
	t.Setenv("ENGRAM_URL", "https://env-url.example.com/mcp")

	stdout, stderr, err := runClient(t, "setup", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v (stdout=%q stderr=%q)", err, stdout, stderr)
	}

	var doc setupReportDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	var sawClaude, sawCodex bool
	for _, row := range doc.Runtimes {
		switch row.Name {
		case "claude-code":
			sawClaude = true
			if !strings.Contains(row.Command, "https://env-url.example.com/mcp") {
				t.Errorf("claude-code row.Command = %q, want it to contain the ENGRAM_URL value", row.Command)
			}
		case "codex":
			sawCodex = true
			want := "codex mcp add engram --url https://env-url.example.com/mcp"
			if row.Command != want {
				t.Errorf("codex row.Command = %q, want %q", row.Command, want)
			}
		}
	}
	if !sawClaude || !sawCodex {
		t.Fatalf("expected claude-code and codex rows, got: %s", stdout)
	}
}

// TestSetupAuthFromEnvSelectsBearerForm proves ENGRAM_AUTH=bearer, with no
// --auth flag, selects the bearer form of the previewed command.
func TestSetupAuthFromEnvSelectsBearerForm(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("codex"))
	t.Setenv("ENGRAM_URL", "https://env-url.example.com/mcp")
	t.Setenv("ENGRAM_AUTH", "bearer")

	stdout, stderr, err := runClient(t, "setup", "--runtime", "codex", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v (stdout=%q stderr=%q)", err, stdout, stderr)
	}

	var doc setupReportDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	if len(doc.Runtimes) != 1 {
		t.Fatalf("setup --runtime codex emitted %d rows, want exactly 1: %s", len(doc.Runtimes), stdout)
	}
	want := "codex mcp add engram --url https://env-url.example.com/mcp --bearer-token-env-var ENGRAM_TOKEN"
	if doc.Runtimes[0].Command != want {
		t.Errorf("codex row.Command = %q, want %q", doc.Runtimes[0].Command, want)
	}
}

// TestSetupFlagBeatsEnvForURL is the adjacency edge: when both --url/--auth
// and ENGRAM_URL/ENGRAM_AUTH are set, the FLAG wins for each independently.
// Each sub-case runs in its own t.Run scope so its resetClientFlags(t)
// t.Cleanup fires before the next sub-case starts — otherwise the
// StringSliceVar-backed --runtime flag would ACCUMULATE across sub-cases
// (resetCommandFlagState's own doc comment: a stringSlice flag's Changed
// latch is cleared but its value is not, by design; only resetClientFlags's
// deferred cleanup nils it).
func TestSetupFlagBeatsEnvForURL(t *testing.T) {
	t.Run("url", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		withFakeSetupEnv(t, fakeSetupEnv("codex"))
		t.Setenv("ENGRAM_URL", "https://env-url.example.com/mcp")

		stdout, stderr, err := runClient(t, "setup", "--runtime", "codex",
			"--url", "https://flag-url.example.com/mcp", "--output", "json")
		if err != nil {
			t.Fatalf("runClient: %v (stdout=%q stderr=%q)", err, stdout, stderr)
		}
		var doc setupReportDoc
		if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
			t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
		}
		if len(doc.Runtimes) != 1 {
			t.Fatalf("setup --runtime codex emitted %d rows, want exactly 1: %s", len(doc.Runtimes), stdout)
		}
		if strings.Contains(doc.Runtimes[0].Command, "env-url.example.com") {
			t.Errorf("codex row.Command = %q, want it to NOT carry the ENGRAM_URL value when --url is set", doc.Runtimes[0].Command)
		}
		if !strings.Contains(doc.Runtimes[0].Command, "flag-url.example.com") {
			t.Errorf("codex row.Command = %q, want it to carry the --url value", doc.Runtimes[0].Command)
		}
	})

	t.Run("auth", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		withFakeSetupEnv(t, fakeSetupEnv("codex"))
		t.Setenv("ENGRAM_AUTH", "none")

		stdout, stderr, err := runClient(t, "setup", "--runtime", "codex",
			"--url", "https://flag-url.example.com/mcp", "--auth", "bearer", "--output", "json")
		if err != nil {
			t.Fatalf("runClient: %v (stdout=%q stderr=%q)", err, stdout, stderr)
		}
		var doc setupReportDoc
		if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
			t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
		}
		if len(doc.Runtimes) != 1 || !strings.Contains(doc.Runtimes[0].Command, "--bearer-token-env-var") {
			t.Errorf("codex row.Command = %q, want the bearer form (--auth bearer beats ENGRAM_AUTH=none)", doc.Runtimes[0].Command)
		}
	})
}

// TestSetupMissingURLIsUsageError is the empty edge: neither lane supplying
// a URL, ENGRAM_URL="", and an explicit --url "" must all produce exitUsage
// (2), not a malformed would-write command (WR-01).
func TestSetupMissingURLIsUsageError(t *testing.T) {
	t.Run("neither-flag-nor-env", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		withFakeSetupEnv(t, fakeSetupEnv("claude"))

		stdout, stderr, err := runClient(t, "setup", "--output", "json")
		if err == nil {
			t.Fatal("expected an error when neither --url nor ENGRAM_URL supplies a URL, got nil")
		}
		if got := exitCodeFromError(err); got != exitUsage {
			t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage); stderr=%q", got, exitUsage, stderr)
		}
		for _, want := range []string{"--url", "ENGRAM_URL"} {
			if !strings.Contains(err.Error(), want) && !strings.Contains(stderr, want) {
				t.Errorf("neither err (%q) nor stderr (%q) names %q", err, stderr, want)
			}
		}
		if strings.Contains(stdout, "would-write") {
			t.Errorf("stdout carries a would-write outcome despite the missing-URL usage error: %s", stdout)
		}
	})

	t.Run("empty-env", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		withFakeSetupEnv(t, fakeSetupEnv("claude"))
		t.Setenv("ENGRAM_URL", "")

		_, stderr, err := runClient(t, "setup", "--output", "json")
		if err == nil {
			t.Fatal(`expected an error for ENGRAM_URL="", got nil`)
		}
		if got := exitCodeFromError(err); got != exitUsage {
			t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage); stderr=%q", got, exitUsage, stderr)
		}
	})

	t.Run("explicit-empty-flag", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		withFakeSetupEnv(t, fakeSetupEnv("claude"))

		_, stderr, err := runClient(t, "setup", "--url", "", "--output", "json")
		if err == nil {
			t.Fatal(`expected an error for --url "", got nil`)
		}
		if got := exitCodeFromError(err); got != exitUsage {
			t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage); stderr=%q", got, exitUsage, stderr)
		}
	})
}

// TestSetupEnvURLPassedVerbatim is the encoding edge: a URL crossing the
// environment lane must reach the emitted command byte-for-byte — no
// appended /mcp, no stripped trailing slash, no decoded escape, no
// re-encoding (D-02).
func TestSetupEnvURLPassedVerbatim(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("codex"))
	const url = "https://gw.example.com/mcp%2Fengram/"
	t.Setenv("ENGRAM_URL", url)

	stdout, stderr, err := runClient(t, "setup", "--runtime", "codex", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v (stdout=%q stderr=%q)", err, stdout, stderr)
	}
	var doc setupReportDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	if len(doc.Runtimes) != 1 {
		t.Fatalf("setup --runtime codex emitted %d rows, want exactly 1: %s", len(doc.Runtimes), stdout)
	}
	if !strings.Contains(doc.Runtimes[0].Command, url) {
		t.Errorf("codex row.Command = %q, want it to contain %q byte-for-byte", doc.Runtimes[0].Command, url)
	}
}

// TestSetupEnvLanePreviewDeterministic is the idempotency edge: with
// ENGRAM_URL/ENGRAM_AUTH set and no matching flags, two previews on the
// json lane and two on the text lane must each be byte-identical.
func TestSetupEnvLanePreviewDeterministic(t *testing.T) {
	resetClientFlags(t)
	withFakeSetupEnv(t, fakeSetupEnv("claude", "codex"))
	t.Setenv("ENGRAM_URL", "https://env-url.example.com/mcp")
	t.Setenv("ENGRAM_AUTH", "bearer")

	resetCommandFlagState(t, setupCmd)
	jsonOut1, _, err := runClient(t, "setup", "--output", "json")
	if err != nil {
		t.Fatalf("runClient (json 1st): %v", err)
	}
	resetCommandFlagState(t, setupCmd)
	jsonOut2, _, err := runClient(t, "setup", "--output", "json")
	if err != nil {
		t.Fatalf("runClient (json 2nd): %v", err)
	}
	if jsonOut1 != jsonOut2 {
		t.Errorf("two identical setup --output json runs differ:\n%q\n%q", jsonOut1, jsonOut2)
	}

	resetCommandFlagState(t, setupCmd)
	textOut1, _, err := runClient(t, "setup", "--output", "text")
	if err != nil {
		t.Fatalf("runClient (text 1st): %v", err)
	}
	resetCommandFlagState(t, setupCmd)
	textOut2, _, err := runClient(t, "setup", "--output", "text")
	if err != nil {
		t.Fatalf("runClient (text 2nd): %v", err)
	}
	if textOut1 != textOut2 {
		t.Errorf("two identical setup --output text runs differ:\n%q\n%q", textOut1, textOut2)
	}
}

// TestSetupGenericRowCarriesPortableConfig proves `engram setup --runtime
// generic --output json` emits a row whose config field is a JSON STRING
// (never a nested object — the D-15 clause Task 2 deliberately does not
// take, cmd/engram/setup.go's setupRuntimeRow doc comment) whose contents
// themselves unmarshal into the mcpServers document. The two-step decode
// (once for the outer document, once for the string's own contents) is
// exactly what fails if Config is ever promoted to a nested-object type.
func TestSetupGenericRowCarriesPortableConfig(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv())

	stdout, stderr, err := runClient(t, "setup",
		"--url", "https://engram.example.com/mcp",
		"--runtime", "generic",
		"--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
	}

	// First decode: confirm the raw wire shape carries config as a JSON
	// string, not an object or array.
	var raw struct {
		Runtimes []struct {
			Name   string          `json:"name"`
			Config json.RawMessage `json:"config"`
		} `json:"runtimes"`
	}
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	var genericRaw json.RawMessage
	for _, r := range raw.Runtimes {
		if r.Name == "generic" {
			genericRaw = r.Config
		}
	}
	if len(genericRaw) == 0 {
		t.Fatalf("setup --runtime generic emitted no generic row with a config field: %s", stdout)
	}
	if genericRaw[0] != '"' {
		t.Fatalf("generic row config is not encoded as a JSON string (first byte %q): %s", genericRaw[0], genericRaw)
	}

	// Second decode: the string's own CONTENTS must unmarshal into the
	// mcpServers document.
	var doc setupReportDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	var configText string
	for _, r := range doc.Runtimes {
		if r.Name == "generic" {
			configText = r.Config
		}
	}
	if configText == "" {
		t.Fatalf("generic row Config is empty: %s", stdout)
	}
	var inner struct {
		MCPServers map[string]struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(configText), &inner); err != nil {
		t.Fatalf("generic row Config %q does not itself unmarshal into the mcpServers document: %v", configText, err)
	}
	entry, ok := inner.MCPServers["engram"]
	if !ok {
		t.Fatalf("generic row Config %q has no mcpServers.engram entry", configText)
	}
	if entry.URL != "https://engram.example.com/mcp" {
		t.Errorf("mcpServers.engram.url = %q, want the --url value byte-for-byte", entry.URL)
	}
}

// TestSetupBareInvocationOmitsGeneric proves a bare `engram setup` (no
// --runtime) against a fake Environment where all three native runtime
// binaries resolve never emits a "generic" row — the opt-in pseudo-
// runtime (D-14) is excluded from the default set even though every
// OTHER registered runtime is present.
func TestSetupBareInvocationOmitsGeneric(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("claude", "codex", "opencode"))

	stdout, stderr, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
	}

	var doc setupReportDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, err)
	}
	for _, row := range doc.Runtimes {
		if row.Name == "generic" {
			t.Errorf("bare `engram setup` emitted a generic row, want none (D-14): %s", stdout)
		}
	}
}

// TestSetupGenericAndFailingRuntimeExitsPartial proves `engram setup
// --apply --runtime generic,claude-code` with a scripted claude-code
// failure exits exitPartial (8), never exitSetupFailed (9) — D-16's
// stated, defensible consequence of leaving internal/setup/exit.go
// untouched: generic's OutcomeWouldWrite counts as a non-failed attempt
// alongside claude-code's OutcomeFailed, so Classify's own combination
// table (unchanged) yields ExitPartial. Pinning the EXACT constant (not
// merely "nonzero") is what stops a later Classify edit from silently
// reclassifying this combination.
func TestSetupGenericAndFailingRuntimeExitsPartial(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	failingRun := func(context.Context, string, []string) (setup.RunResult, error) {
		return setup.RunResult{ExitCode: 1, Stderr: "boom: mcp add failed"}, nil
	}
	withFakeSetupEnv(t, fakeSetupEnvWithRun(failingRun, "claude"))

	_, stderr, err := runClient(t, "setup",
		"--url", "https://engram.example.com/mcp",
		"--runtime", "generic,claude-code",
		"--apply")
	if err == nil {
		t.Fatal("expected a non-nil error (claude-code fails while generic succeeds), got nil")
	}
	if got := exitCodeFromError(err); got != exitPartial {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitPartial); stderr=%q", got, exitPartial, stderr)
	}
}

// TestSetupPartialExitIsLiveProducible proves exitPartial (8) has a REAL,
// live production path — not merely an allowlist claim
// (catalog_test.go's nonConnectProducedCodes names "setup (partial
// per-runtime failure)" as this code's producer, but until this phase no
// production path could actually reach it). Two NATIVE runtimes are
// selected: claude-code's write succeeds, codex's write fails. This turns
// the allowlist entry into a proven one, which is what that entry's own
// doc comment says it is supposed to mean.
func TestSetupPartialExitIsLiveProducible(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	// Distinguish by the LookPath-resolved BINARY PATH (never a
	// runtime-name check inside the fake — the fake only ever sees
	// path/args, mirroring what the real Environment.Run seam sees):
	// codex's every invocation fails, claude-code's every invocation
	// succeeds.
	mixedRun := func(_ context.Context, path string, _ []string) (setup.RunResult, error) {
		if strings.Contains(path, "codex") {
			return setup.RunResult{ExitCode: 1, Stderr: "boom: mcp add failed"}, nil
		}
		return setup.RunResult{ExitCode: 0}, nil
	}
	withFakeSetupEnv(t, fakeSetupEnvWithRun(mixedRun, "claude", "codex"))

	stdout, stderr, err := runClient(t, "setup",
		"--url", "https://engram.example.com/mcp",
		"--runtime", "claude-code,codex",
		"--output", "json",
		"--apply")
	if err == nil {
		t.Fatal("expected a non-nil error (codex fails while claude-code succeeds), got nil")
	}
	if got := exitCodeFromError(err); got != exitPartial {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitPartial); stdout=%q stderr=%q", got, exitPartial, stdout, stderr)
	}

	var doc setupReportDoc
	if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
	}
	var sawSucceeded, sawFailed bool
	for _, row := range doc.Runtimes {
		switch row.Name {
		case "claude-code":
			sawSucceeded = row.Outcome == string(setup.OutcomeWrote) || row.Outcome == string(setup.OutcomeAlreadyCorrect)
		case "codex":
			sawFailed = row.Outcome == string(setup.OutcomeFailed)
		}
	}
	if !sawSucceeded {
		t.Errorf("claude-code row did not report a successful outcome: %s", stdout)
	}
	if !sawFailed {
		t.Errorf("codex row did not report %q: %s", setup.OutcomeFailed, stdout)
	}
}

// setupRowJSONTag returns setupRuntimeRow's json tag NAME for fieldName
// (the part before any comma), failing the test if the field or its tag
// is missing. Deriving the literal this way — rather than restating the
// tag text a second time in an assertion — means a field rename cannot
// leave an assertion passing vacuously (04-01-PLAN.md Task 2).
func setupRowJSONTag(t *testing.T, fieldName string) string {
	t.Helper()
	field, ok := reflect.TypeOf(setupRuntimeRow{}).FieldByName(fieldName)
	if !ok {
		t.Fatalf("setupRuntimeRow has no field named %q", fieldName)
	}
	name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
	if name == "" {
		t.Fatalf("setupRuntimeRow.%s has no json tag name", fieldName)
	}
	return name
}

// TestSetupPreviewSkillsJSON proves the json output lane carries the FULL
// skill content (D-03): the claude-code row's skills_content field is a
// non-empty string that parses as a JSON object whose key count equals
// the number of files skills.Inventory() discovers — never a literal
// count — and the row also carries a non-empty digest summary and a
// byte count that parses as a positive integer.
func TestSetupPreviewSkillsJSON(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("claude"))

	stdout, stderr, err := runClient(t, "setup",
		"--url", "https://engram.example.com/mcp", "--runtime", "claude-code", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
	}

	var doc setupReportDoc
	if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
	}
	if len(doc.Runtimes) != 1 {
		t.Fatalf("setup preview emitted %d rows, want exactly 1: %s", len(doc.Runtimes), stdout)
	}
	row := doc.Runtimes[0]

	if row.SkillsContent == "" {
		t.Fatalf("claude-code row.SkillsContent is empty, want the full skill content (D-03 json lane): %s", stdout)
	}
	var contentDoc map[string]string
	if uErr := json.Unmarshal([]byte(row.SkillsContent), &contentDoc); uErr != nil {
		t.Fatalf("json.Unmarshal(row.SkillsContent = %q): %v", row.SkillsContent, uErr)
	}

	inv, invErr := skills.Inventory()
	if invErr != nil {
		t.Fatalf("skills.Inventory(): %v", invErr)
	}
	wantKeys := 0
	for _, s := range inv {
		wantKeys += len(s.Files)
	}
	if len(contentDoc) != wantKeys {
		t.Errorf("row.SkillsContent has %d keys, want %d (derived from skills.Inventory(), never a literal)", len(contentDoc), wantKeys)
	}

	if row.SkillsDigest == "" {
		t.Error("claude-code row.SkillsDigest is empty, want a non-empty per-skill digest summary")
	}
	n, convErr := strconv.Atoi(row.SkillsBytes)
	if convErr != nil {
		t.Errorf("claude-code row.SkillsBytes = %q, want it to parse as an integer: %v", row.SkillsBytes, convErr)
	} else if n <= 0 {
		t.Errorf("claude-code row.SkillsBytes = %q, want a positive integer", row.SkillsBytes)
	}
}

// TestSetupTextRowOmitsSkillContent renders the SAME preview through the
// text format and asserts the dense row carries the destination, digest,
// and byte-count fields, but NEVER the content field (D-03) — the text
// lane must stay dense even though the five skills total tens of
// kilobytes.
func TestSetupTextRowOmitsSkillContent(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("claude"))

	stdout, stderr, err := runClient(t, "setup",
		"--url", "https://engram.example.com/mcp", "--runtime", "claude-code", "--output", "text")
	if err != nil {
		t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
	}

	contentKey := setupRowJSONTag(t, "SkillsContent")
	digestKey := setupRowJSONTag(t, "SkillsDigest")
	bytesKey := setupRowJSONTag(t, "SkillsBytes")
	destKey := setupRowJSONTag(t, "SkillsDest")

	for _, want := range []string{digestKey, bytesKey, destKey} {
		if !strings.Contains(stdout, want) {
			t.Errorf("text output does not contain %q: %s", want, stdout)
		}
	}
	if strings.Contains(stdout, contentKey) {
		t.Errorf("text output contains %q, want the dense text row to NEVER carry skill content field (D-03): %s", contentKey, stdout)
	}
	if len(stdout) >= 4096 {
		t.Errorf("text output is %d bytes, want under 4096 for a single-runtime preview", len(stdout))
	}
}

// TestSetupSkillsFailureReachesPartialExit proves that a SKILLS-ONLY
// failure on one runtime, alongside another runtime's success, produces
// the partial exit class — because setupResultsFromRows feeds Classify
// the AGGREGATED outcome (D-06/D-07, REQ-setup-partial-failure-legible).
// claude-code's registration succeeds while its skills install fails
// (scripted via skillsEnv, scoped to claude-code's own destination —
// 04-03-PLAN.md wires codex for skills too, so the fake WriteFile must
// distinguish the two runtimes' destinations rather than failing
// universally); codex's own skills write succeeds, giving a clean
// registration+skills row to pair against claude-code's skills-only
// failure. Both rows must still render in the captured output before the
// nonzero exit (T-02-06).
func TestSetupSkillsFailureReachesPartialExit(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("claude", "codex"))

	// Override the auto-faked skillsEnv (withFakeSetupEnv) with one whose
	// writes fail ONLY under claude-code's own skills destination
	// (.claude/skills) — codex's distinct destination (.agents/skills,
	// plus its .codex/AGENTS.md index) writes through cleanly, so this
	// failure reaches exactly one row's skills facet.
	skillsEnv = skills.Environment{
		ReadFile: func(string) ([]byte, error) { return nil, os.ErrNotExist },
		WriteFile: func(path string, _ []byte, _ os.FileMode) error {
			if strings.Contains(path, string(os.PathSeparator)+".claude"+string(os.PathSeparator)+"skills"+string(os.PathSeparator)) {
				return errors.New("boom: disk full")
			}
			return nil
		},
		MkdirAll: func(string, os.FileMode) error { return nil },
	}

	stdout, stderr, err := runClient(t, "setup",
		"--url", "https://engram.example.com/mcp",
		"--runtime", "claude-code,codex",
		"--output", "json",
		"--apply")
	if err == nil {
		t.Fatal("expected a non-nil error (claude-code's skills install fails), got nil")
	}
	if got := exitCodeFromError(err); got != exitPartial {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitPartial); stdout=%q stderr=%q", got, exitPartial, stdout, stderr)
	}
	if stdout == "" {
		t.Fatal("stdout is empty; the report must be rendered before the nonzero exit (T-02-06)")
	}

	var doc setupReportDoc
	if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
	}
	if len(doc.Runtimes) != 2 {
		t.Fatalf("setup --apply emitted %d rows, want 2 (claude-code and codex, both rendered): %s", len(doc.Runtimes), stdout)
	}

	var sawClaudeFailed, sawCodexRow bool
	for _, row := range doc.Runtimes {
		switch row.Name {
		case "claude-code":
			sawClaudeFailed = row.Outcome == string(setup.OutcomeFailed)
			if row.Skills != string(setup.OutcomeFailed) {
				t.Errorf("claude-code row.Skills = %q, want %q (skills-only failure)", row.Skills, setup.OutcomeFailed)
			}
		case "codex":
			sawCodexRow = row.Outcome != ""
		}
	}
	if !sawClaudeFailed {
		t.Errorf("claude-code row did not report an aggregated outcome of %q: %s", setup.OutcomeFailed, stdout)
	}
	if !sawCodexRow {
		t.Errorf("codex row missing or carries an empty outcome: %s", stdout)
	}
}

// TestSetupIndexReadFailureReachesPartialExit proves that an unreadable
// AGENTS.md index (issue #559, Task 1 of this plan) surfaces at the CLI
// boundary exactly like TestSetupSkillsFailureReachesPartialExit's
// scripted write failure does: a failed codex row naming the index path,
// a non-failed claude-code row, zero index writes, continued native
// skill writes, and the partial exit code — proving the wiring from
// Report.Err through setupApplySkillsFacet, SkillsOutcome,
// AggregateOutcome, and Classify already carries an upstream index-read
// failure end to end (D-06/D-07). Unlike the analog above, this test
// never fails a WRITE — the point is that a successful writer must never
// be REACHED for the index once its read failed.
func TestSetupIndexReadFailureReachesPartialExit(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("claude", "codex"))

	const codexIndex = "/home/fake/.codex/AGENTS.md"
	seed := []byte("# Operator's own AGENTS.md\n\nHand-written guidance.\n")

	store := map[string][]byte{codexIndex: append([]byte(nil), seed...)}
	var writeLog []string

	// Override the auto-faked skillsEnv (withFakeSetupEnv) with one whose
	// READ fails ONLY for codex's index path — every other known key
	// reads its stored bytes, every other unknown key reports
	// os.ErrNotExist (the ordinary "not yet installed" case).
	skillsEnv = skills.Environment{
		ReadFile: func(name string) ([]byte, error) {
			if name == codexIndex {
				return nil, os.ErrPermission
			}
			if b, ok := store[name]; ok {
				return b, nil
			}
			return nil, os.ErrNotExist
		},
		WriteFile: func(name string, data []byte, _ os.FileMode) error {
			writeLog = append(writeLog, name)
			cp := make([]byte, len(data))
			copy(cp, data)
			store[name] = cp
			return nil
		},
		MkdirAll: func(string, os.FileMode) error { return nil },
	}

	stdout, stderr, err := runClient(t, "setup",
		"--url", "https://engram.example.com/mcp",
		"--runtime", "claude-code,codex",
		"--output", "json",
		"--apply")
	if err == nil {
		t.Fatal("expected a non-nil error (codex's index read fails), got nil")
	}
	if got := exitCodeFromError(err); got != exitPartial {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitPartial); stdout=%q stderr=%q", got, exitPartial, stdout, stderr)
	}
	if stdout == "" {
		t.Fatal("stdout is empty; the report must be rendered before the nonzero exit (T-02-06)")
	}

	var doc setupReportDoc
	if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
	}
	if len(doc.Runtimes) != 2 {
		t.Fatalf("setup --apply emitted %d rows, want 2 (claude-code and codex, both rendered): %s", len(doc.Runtimes), stdout)
	}

	for _, row := range doc.Runtimes {
		switch row.Name {
		case "codex":
			if row.Outcome != string(setup.OutcomeFailed) {
				t.Errorf("codex row.Outcome = %q, want %q", row.Outcome, setup.OutcomeFailed)
			}
			if row.Skills != string(setup.OutcomeFailed) {
				t.Errorf("codex row.Skills = %q, want %q", row.Skills, setup.OutcomeFailed)
			}
			if row.Registration == string(setup.OutcomeFailed) {
				t.Errorf("codex row.Registration = %q, want it NOT failed (registration itself succeeds; only the skills facet's index read fails)", row.Registration)
			}
			if row.SkillsIndex != codexIndex {
				t.Errorf("codex row.SkillsIndex = %q, want %q", row.SkillsIndex, codexIndex)
			}
			if !strings.Contains(row.Reason, codexIndex) {
				t.Errorf("codex row.Reason = %q, want it to contain %q", row.Reason, codexIndex)
			}
		case "claude-code":
			if row.Outcome == string(setup.OutcomeFailed) {
				t.Errorf("claude-code row.Outcome = %q, want it NOT failed", row.Outcome)
			}
			if row.Skills == string(setup.OutcomeFailed) {
				t.Errorf("claude-code row.Skills = %q, want it NOT failed", row.Skills)
			}
		}
	}

	var indexWriteCount int
	var sawSkillsDirWrite bool
	for _, p := range writeLog {
		if p == codexIndex {
			indexWriteCount++
		}
		if strings.HasPrefix(p, "/home/fake/.agents/skills/") {
			sawSkillsDirWrite = true
		}
	}
	if indexWriteCount != 0 {
		t.Errorf("writeLog contains %d entries for %q, want 0 (zero index writes on a read failure)", indexWriteCount, codexIndex)
	}
	if !sawSkillsDirWrite {
		t.Errorf("writeLog = %v, want at least one entry under /home/fake/.agents/skills/ (codex's native skill files still install, D-07)", writeLog)
	}
	if !bytes.Equal(store[codexIndex], seed) {
		t.Errorf("store[%q] = %q, want it UNCHANGED — zero bytes written on a read failure", codexIndex, store[codexIndex])
	}
}

// setupE2ERecorder builds an in-memory setup.Environment/skills.Environment
// pair for TestSetupReportCoversEveryRuntimeShape, recording every process
// invocation and every skills filesystem write it receives — so the
// "no real runtime binary is executed" and "no real home directory is
// written" properties (repo rule m45p2b4bp7) can be asserted STRUCTURALLY
// against the recording, rather than merely by construction.
type setupE2ERecorder struct {
	home     string
	resolved map[string]string // bare binary name -> the ONE fake path LookPath returns for it

	runCalls  []string       // every path env.Run was invoked with, in call order
	callCount map[string]int // per-path call counter, so a probe read and a later probe read never coincidentally look identical

	// runResult scripts env.Run's response for one call to path; nil means
	// "always succeed, with output that varies by call count" — the
	// default every subtest but the two failure scenarios below wants,
	// since a probe read that never changes byte-for-byte would make
	// every native runtime's registration facet ambiguous between wrote
	// and already-correct (D-08) rather than deterministically wrote.
	runResult func(path string, callNum int) setup.RunResult

	writes    map[string][]byte // path -> content, backing both ReadFile and WriteFile
	writeLog  []string          // every path skills.Environment.WriteFile was invoked with, in call order
	failWrite func(path string) bool
}

func newSetupE2ERecorder(home string, present ...string) *setupE2ERecorder {
	resolved := make(map[string]string, len(present))
	for _, name := range present {
		resolved[name] = "/fake/bin/e2e-" + name
	}
	return &setupE2ERecorder{
		home:      home,
		resolved:  resolved,
		callCount: make(map[string]int),
		writes:    make(map[string][]byte),
	}
}

func (r *setupE2ERecorder) setupEnv() setup.Environment {
	return setup.Environment{
		LookPath: func(file string) (string, error) {
			if p, ok := r.resolved[file]; ok {
				return p, nil
			}
			return "", exec.ErrNotFound
		},
		Getenv:  func(string) string { return "" },
		HomeDir: func() (string, error) { return r.home, nil },
		Run: func(_ context.Context, path string, _ []string) (setup.RunResult, error) {
			r.runCalls = append(r.runCalls, path)
			r.callCount[path]++
			if r.runResult != nil {
				return r.runResult(path, r.callCount[path]), nil
			}
			return setup.RunResult{ExitCode: 0, Stdout: fmt.Sprintf("call-%d", r.callCount[path])}, nil
		},
	}
}

func (r *setupE2ERecorder) skillsEnv() skills.Environment {
	return skills.Environment{
		ReadFile: func(name string) ([]byte, error) {
			b, ok := r.writes[name]
			if !ok {
				return nil, os.ErrNotExist
			}
			return b, nil
		},
		WriteFile: func(name string, data []byte, _ os.FileMode) error {
			r.writeLog = append(r.writeLog, name)
			if r.failWrite != nil && r.failWrite(name) {
				return errors.New("boom: scripted skills write failure")
			}
			cp := make([]byte, len(data))
			copy(cp, data)
			r.writes[name] = cp
			return nil
		},
		MkdirAll: func(string, os.FileMode) error { return nil },
		Lstat:    func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
	}
}

// install points the package-level setupEnv/skillsEnv seams at r for the
// duration of the calling test, restoring both via t.Cleanup — the
// setupE2ERecorder-backed analogue of withFakeSetupEnv above, used instead
// of it because this test needs the extra call/write recording that
// helper does not provide.
func (r *setupE2ERecorder) install(t *testing.T) {
	t.Helper()
	origSetup, origSkills := setupEnv, skillsEnv
	setupEnv = r.setupEnv()
	skillsEnv = r.skillsEnv()
	t.Cleanup(func() {
		setupEnv = origSetup
		skillsEnv = origSkills
	})
}

// assertNoRealInvocationOrHomeWrite is the structural proof (repo rule
// m45p2b4bp7) that this test never touched a real runtime binary or the
// real filesystem: every recorded Run call used a path r's own fake
// LookPath returned, and every recorded skills write landed under r's own
// fake home directory.
func (r *setupE2ERecorder) assertNoRealInvocationOrHomeWrite(t *testing.T) {
	t.Helper()
	fakePaths := make(map[string]bool, len(r.resolved))
	for _, p := range r.resolved {
		fakePaths[p] = true
	}
	for _, called := range r.runCalls {
		if !fakePaths[called] {
			t.Errorf("env.Run was invoked with %q, which is not a path the fake LookPath ever returned (%v) — this would be a REAL runtime binary", called, r.resolved)
		}
	}
	for _, written := range r.writeLog {
		if !strings.HasPrefix(written, r.home) {
			t.Errorf("skills.Environment.WriteFile was invoked with %q, which is not under the fake home directory %q — this would be a REAL filesystem write", written, r.home)
		}
	}
}

// namesOf returns each Runtime's own Name(), in the given slice's order —
// the derived-from-the-registry equivalent of a hardcoded runtime-name
// literal, used throughout TestSetupReportCoversEveryRuntimeShape so the
// test needs no edit when a fifth runtime is registered.
func namesOf(rts []setup.Runtime) []string {
	names := make([]string, len(rts))
	for i, rt := range rts {
		names[i] = rt.Name()
	}
	return names
}

// setupOutcomeFoldTable is the LITERAL, pinned lookup table
// TestSetupReportCoversEveryRuntimeShape's aggregation assertions consult
// for every (Registration, Skills) facet pair this test's own scenarios
// produce. This is deliberately NOT computed by calling
// setup.AggregateOutcome — a test that calls the function under test to
// derive its own expectation proves nothing (04-04-PLAN.md Task 3
// acceptance criteria). Every entry here is a fact this test's own
// fakes are scripted to produce, pinned by hand.
var setupOutcomeFoldTable = map[[2]string]string{
	{string(setup.OutcomeWouldWrite), string(setup.OutcomeWouldWrite)}:     string(setup.OutcomeWouldWrite),
	{string(setup.OutcomeWrote), string(setup.OutcomeWrote)}:               string(setup.OutcomeWrote),
	{string(setup.OutcomeWrote), string(setup.OutcomeFailed)}:              string(setup.OutcomeFailed),
	{string(setup.OutcomeFailed), string(setup.OutcomeWrote)}:              string(setup.OutcomeFailed),
	{string(setup.OutcomePreserved), string(setup.OutcomeWouldWrite)}:      string(setup.OutcomePreserved),
	{string(setup.OutcomeAlreadyCorrect), string(setup.OutcomeWouldWrite)}: string(setup.OutcomeAlreadyCorrect),
}

// assertFoldedOutcome asserts row.Outcome equals setupOutcomeFoldTable's
// entry for row's own (Registration, Skills) pair — never a call to
// setup.AggregateOutcome.
func assertFoldedOutcome(t *testing.T, row setupRuntimeRow) {
	t.Helper()
	key := [2]string{row.Registration, row.Skills}
	want, ok := setupOutcomeFoldTable[key]
	if !ok {
		t.Fatalf("%s: setupOutcomeFoldTable has no entry for facet pair (Registration=%q, Skills=%q) — extend the literal table", row.Name, row.Registration, row.Skills)
	}
	if row.Outcome != want {
		t.Errorf("%s: row.Outcome = %q, want %q (the literal table's fold of Registration=%q, Skills=%q)", row.Name, row.Outcome, want, row.Registration, row.Skills)
	}
}

// rowByName finds the row named name in rows, failing the test if absent.
func rowByName(t *testing.T, rows []setupRuntimeRow, name string) setupRuntimeRow {
	t.Helper()
	for _, r := range rows {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("no row named %q in %+v", name, rows)
	return setupRuntimeRow{}
}

// TestSetupReportCoversEveryRuntimeShape is the phase's closing gate: one
// table-driven test that drives the REAL command through both lanes with
// every runtime shape present at once, so the composition, the
// aggregation, the row shape, and the exit classification are proven
// together rather than one at a time (04-04-PLAN.md Task 3). Every
// scenario below runs against setupE2ERecorder's fully in-memory
// Environment pair — no real runtime binary is ever executed and no path
// outside this test's own fakes is ever written (repo rule m45p2b4bp7),
// asserted structurally via assertNoRealInvocationOrHomeWrite after every
// subtest that mutates.
func TestSetupReportCoversEveryRuntimeShape(t *testing.T) {
	inv, invErr := skills.Inventory()
	if invErr != nil {
		t.Fatalf("skills.Inventory(): %v", invErr)
	}
	// WR-06: wantDigest/wantBytes call the SAME production functions
	// (setupSkillsDigestSummary, skills.TotalBytes) that
	// setupApplySkillsFacet calls to populate row.SkillsDigest/SkillsBytes.
	// The assertions below therefore prove ONLY that the composition
	// threads one skills.Inventory() call through consistently across
	// runtimes and lanes — legitimate plumbing coverage, but NOT
	// independent verification that setupSkillsDigestSummary or
	// skills.TotalBytes compute the correct value; a bug shared between
	// this setup and the production call site would pass silently. That
	// narrower correctness question is independently covered by
	// internal/skills/inventory_test.go's TestDigestIsStableAndTruncated.
	// A hand-verified literal (setupOutcomeFoldTable's own pattern,
	// deliberately NOT used here) would pin an exact digest/byte-count tied
	// to today's five skills' content, reopening this test for every
	// content edit — the opposite of D-04's "a sixth skill ships with zero
	// code change" property this test is built to preserve.
	wantDigest := setupSkillsDigestSummary(inv)
	wantBytes := strconv.Itoa(skills.TotalBytes(inv))

	nativeRuntimes, selErr := setup.Select(nil)
	if selErr != nil {
		t.Fatalf("setup.Select(nil): %v", selErr)
	}
	nativeNames := namesOf(nativeRuntimes) // claude-code, codex, opencode — generic excluded (D-14)
	allNames := setup.Names()              // claude-code, codex, opencode, generic, in registry order

	t.Run("bare preview omits generic and covers the default set", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		rec := newSetupE2ERecorder("/home/e2e-bare", "claude", "codex", "opencode")
		rec.install(t)

		stdout, stderr, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--output", "json")
		if err != nil {
			t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
		}
		var doc setupReportDoc
		if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
		}
		if len(doc.Runtimes) != len(nativeNames) {
			t.Fatalf("bare preview emitted %d rows, want %d (setup.Select(nil)'s own length): %s", len(doc.Runtimes), len(nativeNames), stdout)
		}
		for _, row := range doc.Runtimes {
			if row.Name == "generic" {
				t.Errorf("bare preview emitted a generic row, want none (D-14): %s", stdout)
			}
			assertFoldedOutcome(t, row)
		}
		rec.assertNoRealInvocationOrHomeWrite(t)
	})

	t.Run("explicit four-runtime preview renders in caller-stated order with the full row shape", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		rec := newSetupE2ERecorder("/home/e2e-four", "claude", "codex", "opencode")
		rec.install(t)

		// A deliberately non-registry order, derived from the registry
		// (reversed) rather than a hand-typed literal — proves the
		// rendering-order property is about the CALLER's order, not
		// registry order.
		reordered := make([]string, len(allNames))
		for i, name := range allNames {
			reordered[len(allNames)-1-i] = name
		}
		runtimeArg := strings.Join(reordered, ",")

		stdoutJSON, stderr, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--runtime", runtimeArg, "--output", "json")
		if err != nil {
			t.Fatalf("runClient (json): %v (stderr=%q)", err, stderr)
		}
		var doc setupReportDoc
		if uErr := json.Unmarshal([]byte(stdoutJSON), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal(%q): %v", stdoutJSON, uErr)
		}
		if len(doc.Runtimes) != len(reordered) {
			t.Fatalf("explicit selection emitted %d rows, want %d: %s", len(doc.Runtimes), len(reordered), stdoutJSON)
		}
		// Rendering-order property: row N's name must equal reordered[N].
		for i, row := range doc.Runtimes {
			if row.Name != reordered[i] {
				t.Errorf("row %d has Name %q, want %q (caller-stated order): %s", i, row.Name, reordered[i], stdoutJSON)
			}
		}

		for _, row := range doc.Runtimes {
			assertFoldedOutcome(t, row)
			if row.SkillsDigest != wantDigest {
				t.Errorf("%s: row.SkillsDigest = %q, want %q (derived from skills.Inventory(), never a literal)", row.Name, row.SkillsDigest, wantDigest)
			}
			if row.SkillsBytes != wantBytes {
				t.Errorf("%s: row.SkillsBytes = %q, want %q (derived from skills.Inventory(), never a literal)", row.Name, row.SkillsBytes, wantBytes)
			}
			if row.SkillsContent == "" {
				t.Errorf("%s: row.SkillsContent is empty in the json lane, want the full skill content for every runtime (D-03)", row.Name)
			}
			if row.Name == "generic" {
				if row.SkillsDest != "" {
					t.Errorf("generic: row.SkillsDest = %q, want empty (no destination, D-11)", row.SkillsDest)
				}
				if row.SkillsIndex != "" {
					t.Errorf("generic: row.SkillsIndex = %q, want empty (no destination, D-11)", row.SkillsIndex)
				}
			} else {
				if row.SkillsDest == "" {
					t.Errorf("%s: row.SkillsDest is empty, want a destination", row.Name)
				}
			}
		}

		resetCommandFlagState(t, setupCmd)
		stdoutText, stderr, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--runtime", runtimeArg, "--output", "text")
		if err != nil {
			t.Fatalf("runClient (text): %v (stderr=%q)", err, stderr)
		}
		contentKey := setupRowJSONTag(t, "SkillsContent")
		if strings.Contains(stdoutText, contentKey) {
			t.Errorf("text lane contains %q, want the dense text row to never carry skill content: %s", contentKey, stdoutText)
		}

		rec.assertNoRealInvocationOrHomeWrite(t)
	})

	t.Run("apply with one skills-only failure reaches the partial exit class", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		rec := newSetupE2ERecorder("/home/e2e-partial", "claude", "codex", "opencode")
		claudeSkillsPrefix := filepath.Join(rec.home, ".claude", "skills")
		rec.failWrite = func(path string) bool { return strings.HasPrefix(path, claudeSkillsPrefix) }
		rec.install(t)

		runtimeArg := strings.Join(allNames, ",")
		stdout, stderr, err := runClient(t, "setup",
			"--url", "https://engram.example.com/mcp",
			"--runtime", runtimeArg,
			"--output", "json",
			"--apply")
		if err == nil {
			t.Fatal("expected a non-nil error (claude-code's skills install fails), got nil")
		}
		if got := exitCodeFromError(err); got != exitPartial {
			t.Errorf("exitCodeFromError(err) = %d, want %d (exitPartial); stderr=%q", got, exitPartial, stderr)
		}
		if stdout == "" {
			t.Fatal("stdout is empty; the report must be rendered before the nonzero exit (T-02-06)")
		}

		var doc setupReportDoc
		if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
		}
		if len(doc.Runtimes) != len(allNames) {
			t.Fatalf("apply emitted %d rows, want %d (every row rendered despite the failure): %s", len(doc.Runtimes), len(allNames), stdout)
		}

		claudeRow := rowByName(t, doc.Runtimes, "claude-code")
		if claudeRow.Outcome != string(setup.OutcomeFailed) {
			t.Errorf("claude-code: row.Outcome = %q, want %q (skills-only failure)", claudeRow.Outcome, setup.OutcomeFailed)
		}
		for _, row := range doc.Runtimes {
			assertFoldedOutcome(t, row)
			if row.Name != "claude-code" && row.Outcome == string(setup.OutcomeFailed) {
				t.Errorf("%s: row.Outcome = %q, want its OWN outcome preserved alongside claude-code's failure, not collapsed to failed", row.Name, row.Outcome)
			}
		}

		rec.assertNoRealInvocationOrHomeWrite(t)
	})

	t.Run("apply where every attempted runtime fails reaches the total-failure exit class", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		rec := newSetupE2ERecorder("/home/e2e-total-failure", "claude", "codex", "opencode")
		rec.runResult = func(string, int) setup.RunResult {
			return setup.RunResult{ExitCode: 1, Stderr: "boom: mcp add failed"}
		}
		rec.install(t)

		runtimeArg := strings.Join(nativeNames, ",") // generic never fails — excluded so this scenario stays total-failure, not partial
		stdout, stderr, err := runClient(t, "setup",
			"--url", "https://engram.example.com/mcp",
			"--runtime", runtimeArg,
			"--output", "json",
			"--apply")
		if err == nil {
			t.Fatal("expected a non-nil error (every attempted runtime fails), got nil")
		}
		if got := exitCodeFromError(err); got != exitSetupFailed {
			t.Errorf("exitCodeFromError(err) = %d, want %d (exitSetupFailed/total-failure); stderr=%q", got, exitSetupFailed, stderr)
		}
		if stdout == "" {
			t.Fatal("stdout is empty; the report must be rendered before the nonzero exit (T-02-06)")
		}

		var doc setupReportDoc
		if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
		}
		if len(doc.Runtimes) != len(nativeNames) {
			t.Fatalf("apply emitted %d rows, want %d: %s", len(doc.Runtimes), len(nativeNames), stdout)
		}
		for _, row := range doc.Runtimes {
			if row.Outcome != string(setup.OutcomeFailed) {
				t.Errorf("%s: row.Outcome = %q, want %q (every attempted runtime fails)", row.Name, row.Outcome, setup.OutcomeFailed)
			}
			assertFoldedOutcome(t, row)
		}

		rec.assertNoRealInvocationOrHomeWrite(t)
	})

	t.Run("a selection where no runtime is present exits successfully with not-present rows", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		rec := newSetupE2ERecorder("/home/e2e-absent") // nothing resolves
		rec.install(t)

		stdout, stderr, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--output", "json")
		if err != nil {
			t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
		}
		var doc setupReportDoc
		if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
		}
		if len(doc.Runtimes) != len(nativeNames) {
			t.Fatalf("bare preview (nothing present) emitted %d rows, want %d: %s", len(doc.Runtimes), len(nativeNames), stdout)
		}
		for _, row := range doc.Runtimes {
			if row.Present {
				t.Errorf("%s: row.Present = true, want false (nothing resolves on this fake PATH)", row.Name)
			}
			if row.Outcome != string(setup.OutcomeNotPresent) {
				t.Errorf("%s: row.Outcome = %q, want %q", row.Name, row.Outcome, setup.OutcomeNotPresent)
			}
			if row.Skills != "" || row.Registration != "" {
				t.Errorf("%s: row carries a skills facet (Skills=%q, Registration=%q), want none for a not-present runtime (D-07)", row.Name, row.Skills, row.Registration)
			}
		}

		rec.assertNoRealInvocationOrHomeWrite(t)
	})
}

// TestSetupClientID exercises only the fake runtime and in-memory skills seams.
func TestSetupClientID(t *testing.T) {
	const url = "https://engram.example.com/mcp"
	for _, tc := range []struct {
		name string
		id   string
	}{
		{"ordinary", "test-client"},
		{"metacharacters", "  test client; $(echo nope) 'quoted' &  "},
	} {
		for _, apply := range []bool{false, true} {
			lane := "preview"
			if apply {
				lane = "fake_apply"
			}
			t.Run(tc.name+"/"+lane, func(t *testing.T) {
				resetClientFlags(t)
				resetCommandFlagState(t, setupCmd)
				var calls [][]string
				env := fakeSetupEnvWithRun(func(_ context.Context, path string, args []string) (setup.RunResult, error) {
					calls = append(calls, append([]string{filepath.Base(path)}, args...))
					return setup.RunResult{}, nil
				}, "claude")
				withFakeSetupEnv(t, env)
				mutations := 0
				write, mkdir := skillsEnv.WriteFile, skillsEnv.MkdirAll
				skillsEnv.WriteFile = func(path string, data []byte, mode os.FileMode) error {
					mutations++
					return write(path, data, mode)
				}
				skillsEnv.MkdirAll = func(path string, mode os.FileMode) error {
					mutations++
					return mkdir(path, mode)
				}
				args := []string{"setup", "--url", url, "--auth", "oauth-client", "--client-id", tc.id, "--runtime", "claude-code", "--output", "json"}
				if apply {
					args = append(args, "--apply")
				}
				stdout, stderr, err := runClient(t, args...)
				if err != nil {
					t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
				}
				var doc setupReportDoc
				if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
					t.Fatal(err)
				}
				wantAdd := []string{"claude", "mcp", "add", "--transport", "http", "engram", url, "--scope", "user", "--client-id", tc.id, "--client-secret", "--callback-port", "8765"}
				wantCommand := "claude mcp remove engram --scope user; " + (setup.Action{Args: wantAdd}).Command()
				if len(doc.Runtimes) != 1 || doc.Runtimes[0].Command != wantCommand {
					t.Fatalf("report = %+v, want one row with command %q", doc.Runtimes, wantCommand)
				}
				adds := 0
				for _, call := range calls {
					if len(call) > 2 && call[2] == "add" {
						adds++
						if !reflect.DeepEqual(call, wantAdd) {
							t.Errorf("add argv = %q, want %q", call, wantAdd)
						}
					} else if !apply && !reflect.DeepEqual(call, []string{"claude", "mcp", "get", "engram"}) &&
						!reflect.DeepEqual(call, []string{"claude", "plugin", "list", "--json"}) {
						// Phase 3: every present claude-code row also runs the D-10
						// plugin capability-and-state probe (read-only, in both
						// preview and apply) — a legitimate additional probe call,
						// never a write.
						t.Errorf("preview ran a non-probe command: %q", call)
					}
				}
				if apply && (adds != 1 || mutations == 0) {
					t.Errorf("apply: adds=%d skills mutations=%d, want one add and installed skills", adds, mutations)
				}
				if !apply && (adds != 0 || mutations != 0) {
					t.Errorf("preview: adds=%d skills mutations=%d, want zero", adds, mutations)
				}
			})
		}
	}

	for _, selection := range []struct {
		name    string
		present []string
		runtime string
	}{
		{"codex", []string{"codex"}, "codex"},
		{"default_mixed", []string{"claude", "codex", "opencode"}, ""},
	} {
		for _, apply := range []bool{false, true} {
			t.Run(selection.name+"/apply="+strconv.FormatBool(apply), func(t *testing.T) {
				resetClientFlags(t)
				resetCommandFlagState(t, setupCmd)
				const id = "  mixed client; 'quoted' $(echo nope) &  "
				var calls [][]string
				withFakeSetupEnv(t, fakeSetupEnvWithRun(func(_ context.Context, path string, args []string) (setup.RunResult, error) {
					calls = append(calls, append([]string{filepath.Base(path)}, args...))
					return setup.RunResult{}, nil
				}, selection.present...))
				args := []string{"setup", "--url", url, "--auth", "oauth-client", "--client-id", id, "--output", "json"}
				if selection.runtime != "" {
					args = append(args, "--runtime", selection.runtime)
				}
				if apply {
					args = append(args, "--apply")
				}
				stdout, stderr, err := runClient(t, args...)
				wantExit := 0
				if apply && selection.runtime == "" {
					wantExit = exitPartial
				}
				if got := exitCodeFromError(err); got != wantExit {
					t.Fatalf("exit=%d, want %d: %v (stderr=%q)", got, wantExit, err, stderr)
				}
				var doc setupReportDoc
				if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
					t.Fatal(err)
				}
				if len(doc.Runtimes) != len(selection.present) {
					t.Fatalf("rows = %+v", doc.Runtimes)
				}
				wantAdds := map[string][]string{
					"claude-code": {"claude", "mcp", "add", "--transport", "http", "engram", url, "--scope", "user", "--client-id", id, "--client-secret", "--callback-port", "8765"},
					"codex":       {"codex", "mcp", "add", "engram", "--url", url, "--oauth-client-id", id},
				}
				for _, row := range doc.Runtimes {
					if row.Name == "opencode" {
						if row.Outcome != "failed" || !strings.Contains(row.Reason, "oauth-client") {
							t.Errorf("unsupported row = %+v", row)
						}
						continue
					}
					wantAdd, ok := wantAdds[row.Name]
					if !ok {
						t.Fatalf("unexpected runtime %q (generic must stay opt-in)", row.Name)
					}
					wantCommand := (setup.Action{Args: wantAdd}).Command()
					if row.Name == "claude-code" {
						wantCommand = "claude mcp remove engram --scope user; " + wantCommand
					}
					if row.Command != wantCommand {
						t.Errorf("command = %q, want %q", row.Command, wantCommand)
					}
					wantOutcome := "would-write"
					if apply {
						wantOutcome = "wrote"
					}
					if row.Outcome != wantOutcome {
						t.Errorf("%s outcome = %q, want %q", row.Name, row.Outcome, wantOutcome)
					}
					adds := 0
					for _, call := range calls {
						if len(call) > 2 && call[0] == wantAdd[0] && call[2] == "add" {
							adds++
							if !reflect.DeepEqual(call, wantAdd) {
								t.Errorf("argv = %q, want %q", call, wantAdd)
							}
						}
					}
					wantCount := 0
					if apply {
						wantCount = 1
					}
					if adds != wantCount {
						t.Errorf("%s add count=%d, want %d", row.Name, adds, wantCount)
					}
				}
				for _, call := range calls {
					if call[0] == "opencode" {
						t.Errorf("unsupported runtime executed %q", call)
					}
				}
			})
		}
	}

	type invalidCase struct {
		name string
		args []string
	}
	invalid := make([]invalidCase, 0, 13)
	invalid = append(invalid, []invalidCase{
		{"missing", []string{"--auth", "oauth-client"}},
		{"empty", []string{"--auth", "oauth-client", "--client-id="}},
		{"whitespace", []string{"--auth", "oauth-client", "--client-id", " \t\n\u2003"}},
	}...)
	for _, auth := range []string{"default", "", "oauth", "bearer", "none"} {
		for _, id := range []string{"", "irrelevant"} {
			args := []string{"--client-id=" + id}
			if auth != "default" {
				args = append(args, "--auth", auth)
			}
			invalid = append(invalid, invalidCase{"irrelevant/" + auth + "/" + id, args})
		}
	}
	for _, tc := range invalid {
		for _, apply := range []bool{false, true} {
			t.Run(tc.name+"/apply="+strconv.FormatBool(apply), func(t *testing.T) {
				resetClientFlags(t)
				resetCommandFlagState(t, setupCmd)
				effects := 0
				env := fakeSetupEnv("claude")
				env.LookPath = func(string) (string, error) { effects++; return "", exec.ErrNotFound }
				env.HomeDir = func() (string, error) { effects++; return "/home/fake", nil }
				env.Run = func(context.Context, string, []string) (setup.RunResult, error) {
					effects++
					return setup.RunResult{}, nil
				}
				withFakeSetupEnv(t, env)
				skillsEnv.WriteFile = func(string, []byte, os.FileMode) error { effects++; return nil }
				skillsEnv.MkdirAll = func(string, os.FileMode) error { effects++; return nil }
				args := append([]string{"setup", "--url", url}, tc.args...)
				if apply {
					args = append(args, "--apply")
				}
				_, stderr, err := runClient(t, args...)
				var coded interface{ ExitCode() int }
				if !errors.As(err, &coded) || coded.ExitCode() != exitUsage {
					t.Errorf("error = %v, want ExitCode() == exitUsage (stderr=%q)", err, stderr)
				}
				if err == nil || !strings.Contains(err.Error(), "--client-id") {
					t.Errorf("error = %v, want it to name --client-id", err)
				}
				if effects != 0 {
					t.Errorf("invalid input caused %d runtime/skills effects, want zero", effects)
				}
			})
		}
	}
}

func TestSetupHelpClientIDContract(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	stdout, stderr, err := runClient(t, "setup", "--help")
	if err != nil {
		t.Fatalf("help: %v (stderr=%q)", err, stderr)
	}
	for _, want := range []string{
		"requires --client-id", "non-secret client ID", "Other auth modes reject --client-id",
		"MCP_CLIENT_SECRET", "inherited environment", "no interactive stdin",
		"--auth oauth-client --client-id example-client", "ENGRAM_TOKEN",
		"applies only to the portable configuration", "token_file=ignored",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help missing %q:\n%s", want, stdout)
		}
	}
	flag := setupCmd.Flags().Lookup("client-id")
	if flag == nil {
		t.Fatal("--client-id flag missing")
	}
	for _, want := range []string{"non-secret", "required for --auth oauth-client", "other auth modes reject"} {
		if !strings.Contains(flag.Usage, want) {
			t.Errorf("client-id flag help = %q, missing %q", flag.Usage, want)
		}
	}
	if flag.DefValue != "" {
		t.Errorf("client-id default = %q, want empty", flag.DefValue)
	}
}

// assertSetupHeaderUsageError fails t unless err is a *cliError carrying
// exitUsage, names "--header", and contains every substring in want. This
// is the direct-call analogue of TestSetupClientID's invalid-branch
// assertions, reused by TestSetupParseHeaders (02-03-PLAN.md Task 1).
func assertSetupHeaderUsageError(t *testing.T, err error, want ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("setupParseHeaders: got nil error, want a usage error")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want exitUsage (%d): %v", got, exitUsage, err)
	}
	if !strings.Contains(err.Error(), "--header") {
		t.Errorf("error = %v, want it to name --header", err)
	}
	for _, w := range want {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("error = %v, want it to contain %q", err, w)
		}
	}
}

// TestSetupParseHeaders exercises setupParseHeaders directly: valid specs
// convert to setup.HeaderSpec values in INPUT order (the CLI does not
// sort — runtimes do, D-08); every invalid shape returns a *cliError
// carrying exitUsage, naming "--header" and the offending NAME, and never
// echoing anything right of a spec's first "=" (a pasted secret lands
// exactly there).
func TestSetupParseHeaders(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			specs []string
			want  []setup.HeaderSpec
		}{
			{"single", []string{"x-gateway-api-key=GATEWAY_KEY"}, []setup.HeaderSpec{{Name: "x-gateway-api-key", EnvVar: "GATEWAY_KEY"}}},
			{"two_input_order", []string{"x-gateway-api-key=GATEWAY_KEY", "CF-Access-Client-Id=CF_ID"},
				[]setup.HeaderSpec{{Name: "x-gateway-api-key", EnvVar: "GATEWAY_KEY"}, {Name: "CF-Access-Client-Id", EnvVar: "CF_ID"}}},
			{"nil", nil, nil},
			{"empty_slice", []string{}, nil},
			{"full_token_class", []string{"x!#$%&'*+.^_`|~-1=OK_9"}, []setup.HeaderSpec{{Name: "x!#$%&'*+.^_`|~-1", EnvVar: "OK_9"}}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				got, err := setupParseHeaders(tc.specs)
				if err != nil {
					t.Fatalf("setupParseHeaders(%q): %v", tc.specs, err)
				}
				if !reflect.DeepEqual(got, tc.want) {
					t.Errorf("setupParseHeaders(%q) = %+v, want %+v", tc.specs, got, tc.want)
				}
			})
		}
	})

	t.Run("authorization_collision", func(t *testing.T) {
		for _, spec := range []string{"Authorization=T", "authorization=T", "AUTHORIZATION=T"} {
			t.Run(spec, func(t *testing.T) {
				_, err := setupParseHeaders([]string{spec})
				assertSetupHeaderUsageError(t, err, "the Authorization header is owned by --auth", "use --auth bearer")
			})
		}
	})

	t.Run("malformed_name", func(t *testing.T) {
		for _, spec := range []string{"", "=GATEWAY_KEY", "x key=GATEWAY_KEY", "x:key=GATEWAY_KEY", "x-clé=GATEWAY_KEY", "sk-live-RHS-SENTINEL-3a9f"} {
			t.Run(spec, func(t *testing.T) {
				_, err := setupParseHeaders([]string{spec})
				assertSetupHeaderUsageError(t, err, "malformed header name", "NAME=ENVVAR")
				if spec == "sk-live-RHS-SENTINEL-3a9f" && strings.Contains(err.Error(), "sk-live-RHS-SENTINEL-3a9f") {
					t.Errorf("error echoed the no-= argument: %v", err)
				}
			})
		}
	})

	t.Run("malformed_envvar", func(t *testing.T) {
		for _, spec := range []string{
			"x-key=", "x-key=sk-live-RHS-SENTINEL-3a9f", "x-key=${GATEWAY_KEY}", "x-key={env:GATEWAY_KEY}",
			"x-key=Bearer abc", "x-key=a:b", "x-key=1BAD", "x-key=MY-KEY",
		} {
			t.Run(spec, func(t *testing.T) {
				_, err := setupParseHeaders([]string{spec})
				assertSetupHeaderUsageError(t, err, "never a value")
				for _, forbidden := range []string{"sk-live-RHS-SENTINEL-3a9f", "${", "{env:", "Bearer"} {
					if strings.Contains(err.Error(), forbidden) {
						t.Errorf("error echoed the right-hand side (%q): %v", forbidden, err)
					}
				}
			})
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		_, err := setupParseHeaders([]string{"x-key=A", "X-KEY=B"})
		assertSetupHeaderUsageError(t, err, "duplicate header name", "X-KEY")
		_, err = setupParseHeaders([]string{"x-key=A", "x-key=A"})
		assertSetupHeaderUsageError(t, err, "duplicate header name")
	})
}

// TestSetupHeaderEnvDefaultReadsEnv proves ENGRAM_HEADERS is split on ","
// into --header's default value, and that an unset/empty var yields nil —
// exercised directly since pflag defaults are bound at init() time, so
// t.Setenv after the binary has already started cannot retroactively
// change a live flag's default (mirrors TestSetupRuntimeEnvDefaultReadsEnv
// above).
func TestSetupHeaderEnvDefaultReadsEnv(t *testing.T) {
	t.Setenv("ENGRAM_HEADERS", "x-gateway-api-key=GATEWAY_KEY,CF-Access-Client-Id=CF_ID")
	got := setupHeaderEnvDefault()
	want := []string{"x-gateway-api-key=GATEWAY_KEY", "CF-Access-Client-Id=CF_ID"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("setupHeaderEnvDefault() = %v, want %v", got, want)
	}

	t.Setenv("ENGRAM_HEADERS", "")
	if got := setupHeaderEnvDefault(); got != nil {
		t.Errorf("setupHeaderEnvDefault() with empty ENGRAM_HEADERS = %v, want nil", got)
	}
}

// setupHeaderInvalidCLI runs `engram setup --url ... <headerArgs>` (and
// its --apply variant) against a fake claude-code-present Environment
// recording every runtime/skills effect, asserting exitUsage, that the
// error names "--header" and every string in want, and that not one
// countable effect occurred (LookPath/HomeDir/Run/WriteFile/MkdirAll) —
// TestSetupClientID's own invalid-input branch shape: D-02/D-03 are
// CLI-boundary usage errors, never a per-runtime capability gap
// (RESEARCH.md Pitfall 5).
func setupHeaderInvalidCLI(t *testing.T, headerArgs []string, want ...string) {
	t.Helper()
	for _, apply := range []bool{false, true} {
		lane := "preview"
		if apply {
			lane = "apply"
		}
		t.Run(lane, func(t *testing.T) {
			resetClientFlags(t)
			resetCommandFlagState(t, setupCmd)
			effects := 0
			env := fakeSetupEnv("claude")
			env.LookPath = func(string) (string, error) { effects++; return "", exec.ErrNotFound }
			env.HomeDir = func() (string, error) { effects++; return "/home/fake", nil }
			env.Run = func(context.Context, string, []string) (setup.RunResult, error) {
				effects++
				return setup.RunResult{}, nil
			}
			withFakeSetupEnv(t, env)
			skillsEnv.WriteFile = func(string, []byte, os.FileMode) error { effects++; return nil }
			skillsEnv.MkdirAll = func(string, os.FileMode) error { effects++; return nil }
			args := append([]string{"setup", "--url", "https://engram.example.com/mcp"}, headerArgs...)
			if apply {
				args = append(args, "--apply")
			}
			_, stderr, err := runClient(t, args...)
			var coded interface{ ExitCode() int }
			if !errors.As(err, &coded) || coded.ExitCode() != exitUsage {
				t.Errorf("error = %v, want ExitCode() == exitUsage (stderr=%q)", err, stderr)
			}
			if err == nil || !strings.Contains(err.Error(), "--header") {
				t.Errorf("error = %v, want it to name --header", err)
			}
			for _, w := range want {
				if err == nil || !strings.Contains(err.Error(), w) {
					t.Errorf("error = %v, want it to contain %q", err, w)
				}
			}
			if effects != 0 {
				t.Errorf("invalid --header caused %d runtime/skills effects, want zero", effects)
			}
		})
	}
}

// TestSetupHeaderRejectsAuthorizationCollision proves --header Authorization=...
// (in any letter case) is a usage error naming --auth bearer, with zero
// runtime/skills effects, in both the preview and --apply lane (D-02).
func TestSetupHeaderRejectsAuthorizationCollision(t *testing.T) {
	for _, spec := range []string{"Authorization=T", "authorization=T", "AUTHORIZATION=T"} {
		t.Run(spec, func(t *testing.T) {
			setupHeaderInvalidCLI(t, []string{"--header", spec}, "the Authorization header is owned by --auth", "use --auth bearer")
		})
	}
}

// TestSetupHeaderRejectsMalformedName proves a --header NAME failing the
// RFC 7230 token grammar — including an explicitly supplied empty
// --header (Changed is true but pflag's readAsCSV("") yields a
// zero-length slice, so it must not silently degrade to "no headers") —
// is a usage error, with zero runtime/skills effects, in both lanes
// (D-03).
func TestSetupHeaderRejectsMalformedName(t *testing.T) {
	for _, spec := range []string{"", "=GATEWAY_KEY", "x key=GATEWAY_KEY", "x:key=GATEWAY_KEY", "x-clé=GATEWAY_KEY", "sk-live-RHS-SENTINEL-3a9f"} {
		t.Run(spec, func(t *testing.T) {
			setupHeaderInvalidCLI(t, []string{"--header", spec}, "malformed header name", "NAME=ENVVAR")
		})
	}
}

// TestSetupHeaderRejectsMalformedEnvVar proves a --header ENVVAR failing
// the POSIX identifier grammar — including an empty ENVVAR and every
// literal-looking right-hand side — is a usage error, with zero
// runtime/skills effects in both lanes, and that the error/stderr never
// echo the offending right-hand side (D-03, REQ-header-value-env-ref-only).
func TestSetupHeaderRejectsMalformedEnvVar(t *testing.T) {
	for _, spec := range []string{
		"x-key=", "x-key=sk-live-RHS-SENTINEL-3a9f", "x-key=${GATEWAY_KEY}", "x-key={env:GATEWAY_KEY}",
		"x-key=Bearer abc", "x-key=a:b", "x-key=1BAD", "x-key=MY-KEY",
	} {
		t.Run(spec, func(t *testing.T) {
			setupHeaderInvalidCLI(t, []string{"--header", spec}, "never a value")

			resetClientFlags(t)
			resetCommandFlagState(t, setupCmd)
			withFakeSetupEnv(t, fakeSetupEnv("claude"))
			_, stderr, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--header", spec)
			if err == nil {
				t.Fatal("want a usage error")
			}
			for _, forbidden := range []string{"sk-live-RHS-SENTINEL-3a9f", "${", "{env:", "Bearer"} {
				if strings.Contains(err.Error(), forbidden) {
					t.Errorf("error echoed the right-hand side (%q): %v", forbidden, err)
				}
				if strings.Contains(stderr, forbidden) {
					t.Errorf("stderr echoed the right-hand side (%q): %q", forbidden, stderr)
				}
			}
		})
	}
}

// TestSetupHeaderRejectsDuplicateName proves two --header entries whose
// NAMEs are equal case-insensitively — across flag repeats or within one
// comma list — are a usage error, with zero runtime/skills effects in
// both lanes (D-03).
func TestSetupHeaderRejectsDuplicateName(t *testing.T) {
	t.Run("flag_repeat", func(t *testing.T) {
		setupHeaderInvalidCLI(t, []string{"--header", "x-key=A", "--header", "X-KEY=B"}, "duplicate header name")
	})
	t.Run("comma_list", func(t *testing.T) {
		setupHeaderInvalidCLI(t, []string{"--header", "x-key=A,X-KEY=B"}, "duplicate header name")
	})
}

// setupCodexHeaderDeclineReason is the reason codex's Plan() authors for
// a declined "x-gateway-api-key" header (internal/setup/codex.go,
// 02-01), quoted here once so this file's own header-related tests never
// restate it by hand. Asserted via strings.Contains, matching
// TestSetupUnsupportedAuthModeIsFailedRow's own precedent: a row whose
// registration fails still runs the skills facet
// (setupApplySkillsFacet/setupRuntimeRowFromResult) against the failed
// Plan()'s zero-value SkillTarget, which itself fails as "unrecognized
// skill format" and appends onto Reason (setupJoinReason) — a pre-existing
// property of the row-rendering pipeline, not something this plan's
// header validation introduces or is responsible for correcting.
const setupCodexHeaderDeclineReason = "codex: custom header(s) x-gateway-api-key: codex mcp add exposes only --bearer-token-env-var (no custom header flag); drop --header or exclude codex via --runtime: setup: custom header is not supported by this runtime"

// TestSetupHeaderCodexDeclined proves --header + codex is a "failed" row
// naming the header and the capability gap end-to-end through the CLI —
// in preview, under --apply with only codex selected (exitSetupFailed),
// and under --apply with claude-code also present (exitPartial, exactly
// like oauth-client on opencode) — with zero "add" invocations ever
// reaching codex (D-09, D-10, REQ-header-codex-declined).
func TestSetupHeaderCodexDeclined(t *testing.T) {
	const url = "https://engram.example.com/mcp"

	t.Run("preview", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		withFakeSetupEnv(t, fakeSetupEnv("codex"))
		stdout, stderr, err := runClient(t, "setup", "--url", url,
			"--header", "x-gateway-api-key=GATEWAY_KEY", "--runtime", "codex", "--output", "json")
		if err != nil {
			t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
		}
		var doc setupReportDoc
		if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
		}
		if len(doc.Runtimes) != 1 {
			t.Fatalf("rows = %+v, want exactly 1", doc.Runtimes)
		}
		row := doc.Runtimes[0]
		if row.Outcome != "failed" {
			t.Errorf("row.Outcome = %q, want %q", row.Outcome, "failed")
		}
		if row.Command != "" {
			t.Errorf("row.Command = %q, want empty", row.Command)
		}
		if !strings.Contains(row.Reason, setupCodexHeaderDeclineReason) {
			t.Errorf("row.Reason = %q, want it to contain %q", row.Reason, setupCodexHeaderDeclineReason)
		}
	})

	t.Run("apply_codex_only", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		var calls [][]string
		withFakeSetupEnv(t, fakeSetupEnvWithRun(func(_ context.Context, path string, args []string) (setup.RunResult, error) {
			calls = append(calls, append([]string{filepath.Base(path)}, args...))
			return setup.RunResult{}, nil
		}, "codex"))
		_, stderr, err := runClient(t, "setup", "--url", url,
			"--header", "x-gateway-api-key=GATEWAY_KEY", "--runtime", "codex", "--apply", "--output", "json")
		if got := exitCodeFromError(err); got != exitSetupFailed {
			t.Fatalf("exit=%d, want exitSetupFailed: %v (stderr=%q)", got, err, stderr)
		}
		for _, call := range calls {
			for _, arg := range call {
				if arg == "add" {
					t.Errorf("codex decline still ran an add invocation: %q", call)
				}
			}
		}
	})

	t.Run("mixed_claude_codex_opencode", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		withFakeSetupEnv(t, fakeSetupEnvWithRun(func(_ context.Context, _ string, _ []string) (setup.RunResult, error) {
			return setup.RunResult{}, nil
		}, "claude", "codex"))
		stdout, stderr, err := runClient(t, "setup", "--url", url,
			"--header", "x-gateway-api-key=GATEWAY_KEY", "--apply", "--output", "json")
		if got := exitCodeFromError(err); got != exitPartial {
			t.Fatalf("exit=%d, want exitPartial: %v (stderr=%q)", got, err, stderr)
		}
		var doc setupReportDoc
		if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
		}
		for _, row := range doc.Runtimes {
			switch row.Name {
			case "claude-code":
				if row.Outcome != "wrote" || !strings.Contains(row.Command, "--header 'x-gateway-api-key: ${GATEWAY_KEY}'") {
					t.Errorf("claude-code row = %+v, want wrote with the header pair", row)
				}
			case "codex":
				if row.Outcome != "failed" || !strings.Contains(row.Reason, setupCodexHeaderDeclineReason) {
					t.Errorf("codex row = %+v, want failed with the decline reason", row)
				}
			case "opencode":
				if row.Outcome != "not-present" {
					t.Errorf("opencode row = %+v, want not-present", row)
				}
			default:
				t.Fatalf("unexpected runtime row: %+v", row)
			}
		}
	})
}

// TestSetupHeaderValidWithEveryAuthMode proves --header is valid with
// EVERY --auth mode (D-01): each renders the same sorted extra --header
// pair after that mode's own auth header, and no code path anywhere in
// the stack reads any environment variable other than XDG_CONFIG_HOME
// (opencode's own config-root read, unrelated to headers).
func TestSetupHeaderValidWithEveryAuthMode(t *testing.T) {
	const url = "https://engram.example.com/mcp"
	for _, auth := range []string{"oauth", "oauth-client", "bearer", "none"} {
		t.Run(auth, func(t *testing.T) {
			resetClientFlags(t)
			resetCommandFlagState(t, setupCmd)
			env := fakeSetupEnv("claude")
			env.Getenv = func(key string) string {
				if key != "XDG_CONFIG_HOME" {
					t.Errorf("unexpected environment read %q", key)
				}
				return ""
			}
			withFakeSetupEnv(t, env)
			args := []string{"setup", "--url", url, "--auth", auth, "--runtime", "claude-code",
				"--header", "x-gateway-api-key=GATEWAY_KEY", "--output", "json"}
			if auth == "oauth-client" {
				args = append(args, "--client-id", "test-client")
			}
			stdout, stderr, err := runClient(t, args...)
			if err != nil {
				t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
			}
			var doc setupReportDoc
			if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
				t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
			}
			if len(doc.Runtimes) != 1 {
				t.Fatalf("rows = %+v, want exactly 1", doc.Runtimes)
			}
			row := doc.Runtimes[0]
			if !strings.HasSuffix(row.Command, "--header 'x-gateway-api-key: ${GATEWAY_KEY}'") {
				t.Errorf("%s: command = %q, want it to end with the header pair", auth, row.Command)
			}
			if auth == "bearer" && !strings.Contains(row.Command, "--header 'Authorization: Bearer ${ENGRAM_TOKEN}' --header 'x-gateway-api-key: ${GATEWAY_KEY}'") {
				t.Errorf("bearer: command = %q, want auth header then extra header", row.Command)
			}
		})
	}

	t.Run("generic", func(t *testing.T) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		env := fakeSetupEnv()
		env.Getenv = func(key string) string {
			if key != "XDG_CONFIG_HOME" {
				t.Errorf("unexpected environment read %q", key)
			}
			return ""
		}
		withFakeSetupEnv(t, env)
		stdout, stderr, err := runClient(t, "setup", "--url", url, "--auth", "bearer",
			"--token-file", "/home/u/.engram/token", "--runtime", "generic",
			"--header", "x-gateway-api-key=GATEWAY_KEY", "--output", "json")
		if err != nil {
			t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
		}
		var doc setupReportDoc
		if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
		}
		if len(doc.Runtimes) != 1 {
			t.Fatalf("rows = %+v, want exactly 1", doc.Runtimes)
		}
		row := doc.Runtimes[0]
		if !strings.Contains(row.Config, `"x-gateway-api-key":"${GATEWAY_KEY}"`) {
			t.Errorf("generic: config = %q, missing extra header", row.Config)
		}
		if !strings.Contains(row.Config, "Bearer \\u003cfrom /home/u/.engram/token\\u003e") {
			t.Errorf("generic: config = %q, missing bearer provenance", row.Config)
		}
	})
}

// TestSetupHeaderOrderIndependent proves D-08: --header order (flag
// repeat vs comma list, and either direction) never changes rendered
// output — three differently-ordered invocations produce byte-identical
// `--output json` documents, and every present row's flat `headers`
// facet is comma-joined and sorted case-insensitively regardless of
// input order (Pitfall 2: the facet is a JSON string, never an
// array/object — proved directly on the raw decoded JSON below).
func TestSetupHeaderOrderIndependent(t *testing.T) {
	const url = "https://engram.example.com/mcp"
	const wantHeaders = "CF-Access-Client-Id=CF_ID,x-gateway-api-key=GATEWAY_KEY"

	invocations := [][]string{
		{"--header", "x-gateway-api-key=GATEWAY_KEY", "--header", "CF-Access-Client-Id=CF_ID"},
		{"--header", "CF-Access-Client-Id=CF_ID", "--header", "x-gateway-api-key=GATEWAY_KEY"},
		{"--header", "CF-Access-Client-Id=CF_ID,x-gateway-api-key=GATEWAY_KEY"},
	}
	stdouts := make([]string, 0, len(invocations))
	for _, hdrArgs := range invocations {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		setupHeaders = nil
		withFakeSetupEnv(t, fakeSetupEnv("claude", "codex", "opencode"))
		args := append([]string{"setup", "--url", url}, hdrArgs...)
		args = append(args, "--output", "json")
		stdout, stderr, err := runClient(t, args...)
		if err != nil {
			t.Fatalf("runClient(%q): %v (stderr=%q)", hdrArgs, err, stderr)
		}
		stdouts = append(stdouts, stdout)
	}
	for i := 1; i < len(stdouts); i++ {
		if stdouts[i] != stdouts[0] {
			t.Errorf("invocation %d differs from invocation 0:\n%q\n%q", i, stdouts[i], stdouts[0])
		}
	}

	var doc setupReportDoc
	if err := json.Unmarshal([]byte(stdouts[0]), &doc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdouts[0], err)
	}
	for _, row := range doc.Runtimes {
		switch row.Name {
		case "claude-code":
			if !strings.Contains(row.Command, "--header 'CF-Access-Client-Id: ${CF_ID}' --header 'x-gateway-api-key: ${GATEWAY_KEY}'") {
				t.Errorf("claude-code command = %q, want the sorted header pair", row.Command)
			}
		case "opencode":
			if !strings.Contains(row.Command, "--header 'CF-Access-Client-Id={env:CF_ID}' --header 'x-gateway-api-key={env:GATEWAY_KEY}'") {
				t.Errorf("opencode command = %q, want the sorted header pair", row.Command)
			}
		}
		// Every PRESENT row carries the facet — including codex's failed
		// row, which reports what was REQUESTED regardless of whether its
		// own Plan() accepted or declined it (D-08).
		if row.Present && row.Headers != wantHeaders {
			t.Errorf("%s: Headers = %q, want %q", row.Name, row.Headers, wantHeaders)
		}
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(stdouts[0]), &raw); err != nil {
		t.Fatalf("json.Unmarshal(raw): %v", err)
	}
	rawRuntimes, _ := raw["runtimes"].([]any)
	if len(rawRuntimes) == 0 {
		t.Fatal("raw runtimes array is empty")
	}
	for _, r := range rawRuntimes {
		row, _ := r.(map[string]any)
		if v, ok := row["headers"]; ok {
			if _, isString := v.(string); !isString {
				t.Errorf("row %+v: headers field type = %T, want string", row, v)
			}
		}
	}

	// Fourth run: only claude-code present, no --header at all — every
	// row (present or not) must omit the facet entirely (omitempty),
	// never render it as "".
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	setupHeaders = nil
	withFakeSetupEnv(t, fakeSetupEnv("claude"))
	stdout, stderr, err := runClient(t, "setup", "--url", url, "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
	}
	var absentDoc setupReportDoc
	if uErr := json.Unmarshal([]byte(stdout), &absentDoc); uErr != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
	}
	for _, row := range absentDoc.Runtimes {
		if row.Headers != "" {
			t.Errorf("%s: Headers = %q, want empty", row.Name, row.Headers)
		}
	}
	if strings.Contains(stdout, `"headers"`) {
		t.Errorf("stdout with no --header carries a headers key: %s", stdout)
	}
}

// --- Phase 3 (Plugin-First Delivery): plugin-facet test infrastructure ---

// claudeListEmptyJSON/claudeListCurrentJSON/claudeMarketplaceAbsentText/
// claudeMarketplacePresentText/codexListEmptyJSON/codexListCurrentJSON/
// codexMarketplacePresentText are scripted `plugin list --json`/`plugin
// marketplace list` responses for the plugin-facet tests below —
// engram@engram at version 0.16.1, matching withFakeSetupVersion's
// binary-side operand in those tests so classifyPluginVersion resolves to
// PluginCurrent.
const (
	claudeListEmptyJSON          = "[]"
	claudeListCurrentJSON        = `[{"id":"engram@engram","version":"0.16.1","scope":"user"}]`
	claudeMarketplaceAbsentText  = "❯ other\n    Source: GitHub (someone/other)\n"
	claudeMarketplacePresentText = "❯ engram\n    Source: GitHub (seanb4t/engram)\n"
	codexListEmptyJSON           = `{"installed":[],"available":[]}`
	codexListCurrentJSON         = `{"installed":[{"pluginId":"engram@engram","name":"engram","marketplaceName":"engram","version":"0.16.1"}],"available":[]}`
	codexMarketplacePresentText  = "MARKETPLACE  ROOT\nengram  /home/fake/.codex/plugins/marketplaces/engram\n"
)

// fakePluginRun builds an env.Run implementation scripting a plugin
// runtime's responses by (bare binary name, joined argv-after-binary) —
// the plugin-lane analogue of fakeSetupEnvWithRun's single (path, args)
// closure. script's outer key is filepath.Base(path) ("claude"/"codex");
// the inner key is strings.Join(args, " "). Any call not matched by
// script — including every registration verb and every plugin WRITE verb
// this test does not care about — delegates to base, unmodified.
func fakePluginRun(base func(context.Context, string, []string) (setup.RunResult, error), script map[string]map[string]setup.RunResult) func(context.Context, string, []string) (setup.RunResult, error) {
	return func(ctx context.Context, path string, args []string) (setup.RunResult, error) {
		if perBinary, ok := script[filepath.Base(path)]; ok {
			if rr, ok := perBinary[strings.Join(args, " ")]; ok {
				return rr, nil
			}
		}
		return base(ctx, path, args)
	}
}

// recording wraps run, additionally appending
// append([]string{filepath.Base(path)}, args...) onto *calls for every
// invocation, in call order — the same idiom
// TestSetupGeneratedInvocations (setup_delegation_test.go) already uses
// inline, factored out here so the plugin tests below can layer it on
// top of fakePluginRun/fakeSetupEnvSucceedingRun.
func recording(calls *[][]string, run func(context.Context, string, []string) (setup.RunResult, error)) func(context.Context, string, []string) (setup.RunResult, error) {
	return func(ctx context.Context, path string, args []string) (setup.RunResult, error) {
		*calls = append(*calls, append([]string{filepath.Base(path)}, args...))
		return run(ctx, path, args)
	}
}

// counterBase is a plugin-test base Run implementation returning a
// DISTINCT stdout ("read N") per DISTINCT (bare binary name, joined args)
// key, incrementing on every call to that same key — every OTHER call
// exits 0 with empty output. This is what makes a runtime's own
// registration probe (`mcp get engram`, run once before and once after
// the write) read as two DIFFERENT captures, so the shared executor
// (apply.go) classifies registration as OutcomeWrote rather than
// OutcomeAlreadyCorrect — every unscripted plugin write verb, in
// contrast, only has its ExitCode consulted, so a fixed "read %d" body
// text is inert there.
func counterBase() func(context.Context, string, []string) (setup.RunResult, error) {
	counts := make(map[string]int)
	return func(_ context.Context, path string, args []string) (setup.RunResult, error) {
		key := filepath.Base(path) + " " + strings.Join(args, " ")
		counts[key]++
		return setup.RunResult{ExitCode: 0, Stdout: fmt.Sprintf("read %d", counts[key])}, nil
	}
}

// assertNoPluginWriteVerb fails the test if any recorded call is a
// plugin WRITE verb (install/update/add/remove, or marketplace add) for
// either claude or codex — the negative-space half of
// TestSetupPluginDeliveredRuntimeAuthorsZeroNativeWrites: a current
// plugin authors zero plugin actions (03-CONTEXT.md "Specific Ideas").
// The read-only list/marketplace-list probes are never flagged.
func assertNoPluginWriteVerb(t *testing.T, calls [][]string) {
	t.Helper()
	for _, call := range calls {
		if len(call) < 3 || call[1] != "plugin" {
			continue
		}
		switch call[2] {
		case "list":
			continue
		case "marketplace":
			if len(call) >= 4 && call[3] == "list" {
				continue
			}
			t.Errorf("recorded a plugin marketplace write verb: %q", call)
		default:
			t.Errorf("recorded a plugin write verb: %q", call)
		}
	}
}

// TestSetupApplyJSONEmitsPluginFacet proves the end-to-end plugin facet
// under --apply: a claude-code row whose plugin install fails stays
// `registration=wrote` beside `plugin=failed` (both facets visible,
// exitPartial), while a codex row whose plugin add succeeds reports
// `plugin=wrote` — and neither row ever falls back to a native skills
// write, because BOTH runtimes are plugin-capable this run (D-07/D-12,
// REQ-plugin-facet-reported).
func TestSetupApplyJSONEmitsPluginFacet(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupVersion(t, "0.16.1")

	script := map[string]map[string]setup.RunResult{
		"claude": {
			"plugin list --json":                                  {ExitCode: 0, Stdout: claudeListEmptyJSON},
			"plugin marketplace list":                             {ExitCode: 0, Stdout: claudeMarketplaceAbsentText},
			"plugin install engram@engram --scope user --json -y": {ExitCode: 1, Stderr: "boom: install refused"},
		},
		"codex": {
			"plugin list --json":      {ExitCode: 0, Stdout: codexListEmptyJSON},
			"plugin marketplace list": {ExitCode: 0, Stdout: codexMarketplacePresentText},
		},
	}
	var calls [][]string
	withFakeSetupEnv(t, fakeSetupEnvWithRun(recording(&calls, fakePluginRun(counterBase(), script)), "claude", "codex"))

	mutations := 0
	write, mkdir := skillsEnv.WriteFile, skillsEnv.MkdirAll
	skillsEnv.WriteFile = func(path string, data []byte, mode os.FileMode) error {
		mutations++
		return write(path, data, mode)
	}
	skillsEnv.MkdirAll = func(path string, mode os.FileMode) error {
		mutations++
		return mkdir(path, mode)
	}

	stdout, stderr, err := runClient(t, "setup",
		"--url", "https://engram.example.com/mcp", "--runtime", "claude-code,codex", "--output", "json", "--apply")
	if err == nil {
		t.Fatal("expected a non-nil error (claude-code's plugin install fails), got nil")
	}
	if got := exitCodeFromError(err); got != exitPartial {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitPartial); stdout=%q stderr=%q", got, exitPartial, stdout, stderr)
	}

	var doc setupReportDoc
	if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
	}
	if len(doc.Runtimes) != 2 {
		t.Fatalf("emitted %d rows, want 2: %s", len(doc.Runtimes), stdout)
	}

	claudeRow := rowByName(t, doc.Runtimes, "claude-code")
	if claudeRow.Outcome != "failed" {
		t.Errorf("claude-code row.Outcome = %q, want %q", claudeRow.Outcome, "failed")
	}
	if claudeRow.Registration != "wrote" {
		t.Errorf("claude-code row.Registration = %q, want %q", claudeRow.Registration, "wrote")
	}
	if claudeRow.Plugin != "failed" {
		t.Errorf("claude-code row.Plugin = %q, want %q", claudeRow.Plugin, "failed")
	}
	if claudeRow.PluginState != "absent" {
		t.Errorf("claude-code row.PluginState = %q, want %q", claudeRow.PluginState, "absent")
	}
	if claudeRow.PluginTarget != "0.16.1" {
		t.Errorf("claude-code row.PluginTarget = %q, want %q", claudeRow.PluginTarget, "0.16.1")
	}
	if claudeRow.PluginInstalled != "" {
		t.Errorf("claude-code row.PluginInstalled = %q, want empty", claudeRow.PluginInstalled)
	}
	wantClaudeCommand := "claude plugin marketplace add seanb4t/engram --scope user; claude plugin install engram@engram --scope user --json -y"
	if claudeRow.PluginCommand != wantClaudeCommand {
		t.Errorf("claude-code row.PluginCommand = %q, want %q", claudeRow.PluginCommand, wantClaudeCommand)
	}
	wantReasonSubstr := "plugin: claude-code: claude plugin install engram@engram --scope user --json -y exited 1: 'boom: install refused'"
	if !strings.Contains(claudeRow.Reason, wantReasonSubstr) {
		t.Errorf("claude-code row.Reason = %q, want it to contain %q", claudeRow.Reason, wantReasonSubstr)
	}
	if claudeRow.Skills != setupSkillsPluginDelivered {
		t.Errorf("claude-code row.Skills = %q, want %q", claudeRow.Skills, setupSkillsPluginDelivered)
	}
	if claudeRow.SkillsNative != "none" {
		t.Errorf("claude-code row.SkillsNative = %q, want %q", claudeRow.SkillsNative, "none")
	}

	codexRow := rowByName(t, doc.Runtimes, "codex")
	if codexRow.Outcome != "wrote" {
		t.Errorf("codex row.Outcome = %q, want %q", codexRow.Outcome, "wrote")
	}
	if codexRow.Registration != "wrote" {
		t.Errorf("codex row.Registration = %q, want %q", codexRow.Registration, "wrote")
	}
	if codexRow.Plugin != "wrote" {
		t.Errorf("codex row.Plugin = %q, want %q", codexRow.Plugin, "wrote")
	}
	if codexRow.PluginState != "absent" {
		t.Errorf("codex row.PluginState = %q, want %q", codexRow.PluginState, "absent")
	}
	wantCodexSource := "/home/fake/.codex/plugins/marketplaces/engram"
	if codexRow.PluginSource != wantCodexSource {
		t.Errorf("codex row.PluginSource = %q, want %q", codexRow.PluginSource, wantCodexSource)
	}
	wantCodexCommand := "codex plugin add engram@engram --json"
	if codexRow.PluginCommand != wantCodexCommand {
		t.Errorf("codex row.PluginCommand = %q, want %q", codexRow.PluginCommand, wantCodexCommand)
	}
	if codexRow.Skills != setupSkillsPluginDelivered {
		t.Errorf("codex row.Skills = %q, want %q", codexRow.Skills, setupSkillsPluginDelivered)
	}

	if mutations != 0 {
		t.Errorf("skills mutations = %d, want 0 (both runtimes delivered; a failed install must never fall back to a native copy)", mutations)
	}

	wantClaudeMarketplaceAdd := []string{"claude", "plugin", "marketplace", "add", "seanb4t/engram", "--scope", "user"}
	wantClaudeInstall := []string{"claude", "plugin", "install", "engram@engram", "--scope", "user", "--json", "-y"}
	wantCodexAdd := []string{"codex", "plugin", "add", "engram@engram", "--json"}
	claudeMarketplaceIdx, claudeInstallIdx, codexAddIdx := -1, -1, -1
	for i, call := range calls {
		switch {
		case reflect.DeepEqual(call, wantClaudeMarketplaceAdd):
			claudeMarketplaceIdx = i
		case reflect.DeepEqual(call, wantClaudeInstall):
			claudeInstallIdx = i
		case reflect.DeepEqual(call, wantCodexAdd):
			codexAddIdx = i
		case len(call) >= 4 && call[0] == "codex" && call[1] == "plugin" && call[2] == "marketplace" && call[3] == "add":
			t.Errorf("codex ran a marketplace add despite an already-present marketplace: %q", call)
		}
	}
	if claudeMarketplaceIdx == -1 || claudeInstallIdx == -1 || claudeMarketplaceIdx >= claudeInstallIdx {
		t.Errorf("recorded calls did not contain %q followed by %q: %q", wantClaudeMarketplaceAdd, wantClaudeInstall, calls)
	}
	if codexAddIdx == -1 {
		t.Errorf("recorded calls did not contain %q: %q", wantCodexAdd, calls)
	}
}

// TestSetupPluginDeliveredRuntimeAuthorsZeroNativeWrites proves the D-07
// routing invariant end to end: when both claude-code's and codex's
// engram plugin are already current, --apply authors ZERO plugin write
// verbs, ZERO native skills writes (skillsEnv.WriteFile/MkdirAll never
// called), and NEVER touches codex's existing AGENTS.md index block —
// while still reporting exactly what already sits at each runtime's
// native destination via SkillsNative (D-08/D-09).
func TestSetupPluginDeliveredRuntimeAuthorsZeroNativeWrites(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupVersion(t, "0.16.1")

	script := map[string]map[string]setup.RunResult{
		"claude": {
			"plugin list --json":      {ExitCode: 0, Stdout: claudeListCurrentJSON},
			"plugin marketplace list": {ExitCode: 0, Stdout: claudeMarketplacePresentText},
		},
		"codex": {
			"plugin list --json":      {ExitCode: 0, Stdout: codexListCurrentJSON},
			"plugin marketplace list": {ExitCode: 0, Stdout: codexMarketplacePresentText},
		},
	}
	var calls [][]string
	// base returns IDENTICAL mcp get output for every call (registration's
	// own two probe reads therefore compare equal -> already-correct).
	withFakeSetupEnv(t, fakeSetupEnvWithRun(recording(&calls, fakePluginRun(fakeSetupEnvSucceedingRun, script)), "claude", "codex"))

	inv, invErr := skills.Inventory()
	if invErr != nil {
		t.Fatalf("skills.Inventory(): %v", invErr)
	}
	const codexAgentsMD = "/home/fake/.codex/AGENTS.md"
	entries := map[string]os.FileMode{
		"/home/fake/.agents/skills": os.ModeDir,
	}
	for _, s := range inv {
		entries["/home/fake/.agents/skills/"+s.Name] = os.ModeSymlink
	}
	seed := []byte("# Mine\n" + skills.BlockStartMarker + "\nold\n" + skills.BlockEndMarker + "\n")
	store := map[string][]byte{codexAgentsMD: append([]byte(nil), seed...)}
	skillsEnv = fakeSkillsEnvWithEntries(entries, store)

	mutations := 0
	write, mkdir := skillsEnv.WriteFile, skillsEnv.MkdirAll
	skillsEnv.WriteFile = func(path string, data []byte, mode os.FileMode) error {
		mutations++
		return write(path, data, mode)
	}
	skillsEnv.MkdirAll = func(path string, mode os.FileMode) error {
		mutations++
		return mkdir(path, mode)
	}

	stdout, stderr, err := runClient(t, "setup",
		"--url", "https://engram.example.com/mcp", "--runtime", "claude-code,codex", "--output", "json", "--apply")
	if err != nil {
		t.Fatalf("runClient: %v (stderr=%q stdout=%q)", err, stderr, stdout)
	}

	var doc setupReportDoc
	if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
	}
	if len(doc.Runtimes) != 2 {
		t.Fatalf("emitted %d rows, want 2: %s", len(doc.Runtimes), stdout)
	}

	for _, name := range []string{"claude-code", "codex"} {
		row := rowByName(t, doc.Runtimes, name)
		if row.Outcome != "already-correct" {
			t.Errorf("%s row.Outcome = %q, want %q", name, row.Outcome, "already-correct")
		}
		if row.Plugin != "already-correct" {
			t.Errorf("%s row.Plugin = %q, want %q", name, row.Plugin, "already-correct")
		}
		if row.PluginState != "current" {
			t.Errorf("%s row.PluginState = %q, want %q", name, row.PluginState, "current")
		}
		if row.PluginInstalled != "0.16.1" {
			t.Errorf("%s row.PluginInstalled = %q, want %q", name, row.PluginInstalled, "0.16.1")
		}
		if row.PluginTarget != "0.16.1" {
			t.Errorf("%s row.PluginTarget = %q, want %q", name, row.PluginTarget, "0.16.1")
		}
		if row.PluginCommand != "" {
			t.Errorf("%s row.PluginCommand = %q, want empty", name, row.PluginCommand)
		}
		if row.PluginNote != "" {
			t.Errorf("%s row.PluginNote = %q, want empty", name, row.PluginNote)
		}
		if row.Skills != setupSkillsPluginDelivered {
			t.Errorf("%s row.Skills = %q, want %q", name, row.Skills, setupSkillsPluginDelivered)
		}
		if row.SkillsDigest != "" || row.SkillsBytes != "" || row.SkillsContent != "" {
			t.Errorf("%s row carries native skill detail (SkillsDigest=%q SkillsBytes=%q SkillsContent=%q), want all empty (nothing was installed natively)",
				name, row.SkillsDigest, row.SkillsBytes, row.SkillsContent)
		}
	}

	claudeRow := rowByName(t, doc.Runtimes, "claude-code")
	if claudeRow.SkillsDest != "/home/fake/.claude/skills" {
		t.Errorf("claude-code row.SkillsDest = %q, want %q", claudeRow.SkillsDest, "/home/fake/.claude/skills")
	}
	if claudeRow.SkillsNative != "none" {
		t.Errorf("claude-code row.SkillsNative = %q, want %q", claudeRow.SkillsNative, "none")
	}

	codexRow := rowByName(t, doc.Runtimes, "codex")
	if codexRow.SkillsDest != "/home/fake/.agents/skills" {
		t.Errorf("codex row.SkillsDest = %q, want %q", codexRow.SkillsDest, "/home/fake/.agents/skills")
	}
	if codexRow.SkillsIndex != codexAgentsMD {
		t.Errorf("codex row.SkillsIndex = %q, want %q", codexRow.SkillsIndex, codexAgentsMD)
	}
	wantCodexNative := fmt.Sprintf("%d skills present at /home/fake/.agents/skills (symlink); index block present at /home/fake/.codex/AGENTS.md — remove manually to avoid duplicates", len(inv))
	if codexRow.SkillsNative != wantCodexNative {
		t.Errorf("codex row.SkillsNative = %q, want %q", codexRow.SkillsNative, wantCodexNative)
	}

	if mutations != 0 {
		t.Errorf("skills mutations = %d, want 0 (both runtimes current and plugin-delivered)", mutations)
	}
	if got := store[codexAgentsMD]; string(got) != string(seed) {
		t.Errorf("store[%q] = %q, want byte-identical to the seed (D-09: no AGENTS.md write)", codexAgentsMD, got)
	}
	assertNoPluginWriteVerb(t, calls)
}

// TestSetupApplyPreservedRuntimeSkipsRegistrationWrite is Phase 5's SC1/SC2
// process-boundary proof (REQ-apply-preserve-gate, D-01, D-05): `--apply`
// against the OBSERVED `x-litellm-api-key` Claude Code shape
// (claudeGetProbeLiteralText, .planning/phases/04-drift-detection-read-only/
// 04-OBSERVATIONS.md §"Claude Code — literal value") classifies `preserved`
// BEFORE any write, so it records exactly one `mcp get` call and never a
// `mcp remove`/`mcp add` — while the plugin lane still runs independently
// of the registration outcome (Phase 3 D-12), in both plugin shapes.
func TestSetupApplyPreservedRuntimeSkipsRegistrationWrite(t *testing.T) {
	withFakeSetupVersion(t, "0.16.1")

	// run drives one --apply invocation against the shared preserved-shape
	// probe, scripting only the plugin-lane responses named by script — any
	// other call (including a would-be mcp remove/add) falls through to
	// scriptedSetupRun's zero-exit default, which is exactly what lets
	// assertNoRegistrationWrite's argv scan catch a regression rather than
	// merely a wrong assertion (RESEARCH.md Pitfall 2).
	run := func(t *testing.T, script map[string]map[string]setup.RunResult) (doc setupReportDoc, calls [][]string, stdout, stderr string) {
		t.Helper()
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		withFakeSetupEnv(t, fakeSetupEnvWithRun(
			recording(&calls, fakePluginRun(
				scriptedSetupRun(t, setup.RunResult{Stdout: claudeGetProbeLiteralText, ExitCode: 0}), script)),
			"claude"))

		var err error
		stdout, stderr, err = runClient(t, "setup",
			"--url", "https://engram.example.com/mcp", "--auth", "oauth",
			"--runtime", "claude-code", "--output", "json", "--apply")
		if err != nil {
			t.Fatalf("runClient: %v (stderr=%q stdout=%q)", err, stderr, stdout)
		}
		if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
		}
		return doc, calls, stdout, stderr
	}

	// assertNoRegistrationWrite is SC2's structural proof: exactly one
	// recorded call is the `mcp get` probe, and no recorded call is any
	// OTHER `mcp` subcommand (`remove`/`add`) — a count-only assertion
	// could pass if some other action replaced the remove call, so both
	// checks are required (RESEARCH.md Pitfall 2).
	assertNoRegistrationWrite := func(t *testing.T, calls [][]string) {
		t.Helper()
		want := []string{"claude", "mcp", "get", "engram"}
		gets := 0
		for _, c := range calls {
			if reflect.DeepEqual(c, want) {
				gets++
			}
			if len(c) >= 3 && c[1] == "mcp" && c[2] != "get" {
				t.Errorf("recorded a registration write call, want none: %q", c)
			}
		}
		if gets != 1 {
			t.Errorf("recorded %d call(s) equal to %q, want exactly 1: %q", gets, want, calls)
		}
	}

	t.Run("plugin-current", func(t *testing.T) {
		script := map[string]map[string]setup.RunResult{
			"claude": {
				"plugin list --json":      {ExitCode: 0, Stdout: claudeListCurrentJSON},
				"plugin marketplace list": {ExitCode: 0, Stdout: claudeMarketplacePresentText},
			},
		}
		doc, calls, stdout, stderr := run(t, script)
		assertNoRegistrationWrite(t, calls)

		row := rowByName(t, doc.Runtimes, "claude-code")
		if row.Registration != "preserved" {
			t.Errorf("Registration = %q, want %q", row.Registration, "preserved")
		}
		if row.Outcome != "preserved" {
			t.Errorf("Outcome = %q, want %q (aggregate: preserved outranks the plugin's already-correct)", row.Outcome, "preserved")
		}
		if row.Plugin != "already-correct" {
			t.Errorf("Plugin = %q, want %q", row.Plugin, "already-correct")
		}
		if row.Skills != setupSkillsPluginDelivered {
			t.Errorf("Skills = %q, want %q", row.Skills, setupSkillsPluginDelivered)
		}
		if !strings.Contains(row.Facets, "header-name") {
			t.Errorf("Facets = %q, want it to contain %q", row.Facets, "header-name")
		}
		if !strings.Contains(row.Reason, "claude mcp remove engram --scope user") {
			t.Errorf("Reason = %q, want it to contain the manual-remediation command", row.Reason)
		}
		if !strings.HasPrefix(row.Reason, "claude-code: preserved: ") {
			t.Errorf("Reason = %q, want prefix %q", row.Reason, "claude-code: preserved: ")
		}
		const literal = "sk-DO-NOT-COMMIT-literal-test-abc123"
		if strings.Contains(stdout, literal) {
			t.Errorf("stdout leaks the observed literal: %s", stdout)
		}
		if strings.Contains(stderr, literal) {
			t.Errorf("stderr leaks the observed literal: %s", stderr)
		}
	})

	t.Run("plugin-absent", func(t *testing.T) {
		script := map[string]map[string]setup.RunResult{
			"claude": {
				"plugin list --json":      {ExitCode: 0, Stdout: claudeListEmptyJSON},
				"plugin marketplace list": {ExitCode: 0, Stdout: claudeMarketplaceAbsentText},
			},
		}
		doc, calls, _, _ := run(t, script)
		assertNoRegistrationWrite(t, calls)

		row := rowByName(t, doc.Runtimes, "claude-code")
		if row.Registration != "preserved" {
			t.Errorf("Registration = %q, want %q", row.Registration, "preserved")
		}
		if row.Plugin != "wrote" {
			t.Errorf("Plugin = %q, want %q", row.Plugin, "wrote")
		}
		if row.PluginState != "absent" {
			t.Errorf("PluginState = %q, want %q", row.PluginState, "absent")
		}
		if row.Outcome != "wrote" {
			t.Errorf("Outcome = %q, want %q (aggregate: the plugin facet wrote)", row.Outcome, "wrote")
		}

		wantMarketplaceAdd := []string{"claude", "plugin", "marketplace", "add"}
		wantInstall := []string{"claude", "plugin", "install"}
		var sawMarketplaceAdd, sawInstall bool
		for _, c := range calls {
			if len(c) >= len(wantMarketplaceAdd) && reflect.DeepEqual(c[:len(wantMarketplaceAdd)], wantMarketplaceAdd) {
				sawMarketplaceAdd = true
			}
			if len(c) >= len(wantInstall) && reflect.DeepEqual(c[:len(wantInstall)], wantInstall) {
				sawInstall = true
			}
		}
		if !sawMarketplaceAdd {
			t.Errorf("recorded calls did not contain a %q call: %q", wantMarketplaceAdd, calls)
		}
		if !sawInstall {
			t.Errorf("recorded calls did not contain a %q call: %q", wantInstall, calls)
		}
	})
}

// TestSetupPreviewShowsPluginArgv proves a bare preview (no --apply)
// shows the exact plugin argv the apply lane would run, and runs no
// write verb of any kind — the plugin lane's own D-11 read-only
// discipline, mirrored in both the json and text output lanes.
func TestSetupPreviewShowsPluginArgv(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupVersion(t, "0.16.1")

	script := map[string]map[string]setup.RunResult{
		"claude": {
			"plugin list --json":      {ExitCode: 0, Stdout: claudeListEmptyJSON},
			"plugin marketplace list": {ExitCode: 0, Stdout: claudeMarketplaceAbsentText},
		},
	}
	var calls [][]string
	withFakeSetupEnv(t, fakeSetupEnvWithRun(recording(&calls, fakePluginRun(fakeSetupEnvSucceedingRun, script)), "claude"))

	mutations := 0
	write, mkdir := skillsEnv.WriteFile, skillsEnv.MkdirAll
	skillsEnv.WriteFile = func(path string, data []byte, mode os.FileMode) error {
		mutations++
		return write(path, data, mode)
	}
	skillsEnv.MkdirAll = func(path string, mode os.FileMode) error {
		mutations++
		return mkdir(path, mode)
	}

	stdout, stderr, err := runClient(t, "setup",
		"--url", "https://engram.example.com/mcp", "--runtime", "claude-code", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
	}
	var doc setupReportDoc
	if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
		t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
	}
	if len(doc.Runtimes) != 1 {
		t.Fatalf("emitted %d rows, want 1: %s", len(doc.Runtimes), stdout)
	}
	row := doc.Runtimes[0]

	if row.Outcome != "would-write" {
		t.Errorf("row.Outcome = %q, want %q", row.Outcome, "would-write")
	}
	if row.Registration != "would-write" {
		t.Errorf("row.Registration = %q, want %q", row.Registration, "would-write")
	}
	if row.Plugin != "would-write" {
		t.Errorf("row.Plugin = %q, want %q", row.Plugin, "would-write")
	}
	if row.PluginState != "absent" {
		t.Errorf("row.PluginState = %q, want %q", row.PluginState, "absent")
	}
	wantPluginCommand := "claude plugin marketplace add seanb4t/engram --scope user; claude plugin install engram@engram --scope user --json -y"
	if row.PluginCommand != wantPluginCommand {
		t.Errorf("row.PluginCommand = %q, want %q", row.PluginCommand, wantPluginCommand)
	}

	realPlan, planErr := setup.ClaudeCode.Plan(setup.Environment{
		HomeDir: func() (string, error) { return "/home/fake", nil },
	}, setup.Options{URL: "https://engram.example.com/mcp", Auth: "oauth"})
	if planErr != nil {
		t.Fatalf("setup.ClaudeCode.Plan: %v", planErr)
	}
	if row.Command != realPlan.Display() {
		t.Errorf("row.Command = %q, want the real registration Plan's Display() %q (untouched by the plugin lane)", row.Command, realPlan.Display())
	}
	if row.Skills != setupSkillsPluginDelivered {
		t.Errorf("row.Skills = %q, want %q", row.Skills, setupSkillsPluginDelivered)
	}

	wantCalls := [][]string{
		{"claude", "mcp", "get", "engram"},
		{"claude", "plugin", "list", "--json"},
		{"claude", "plugin", "marketplace", "list"},
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Errorf("recorded calls = %q, want exactly %q (no write verb)", calls, wantCalls)
	}
	if mutations != 0 {
		t.Errorf("skills mutations = %d, want 0 (a preview never writes)", mutations)
	}

	resetCommandFlagState(t, setupCmd)
	stdoutText, stderr, err := runClient(t, "setup",
		"--url", "https://engram.example.com/mcp", "--runtime", "claude-code", "--output", "text")
	if err != nil {
		t.Fatalf("runClient (text): %v (stderr=%q)", err, stderr)
	}
	for _, want := range []string{"plugin=would-write", "plugin_state=absent", "skills=plugin-delivered", "plugin_command="} {
		if !strings.Contains(stdoutText, want) {
			t.Errorf("text output does not contain %q: %s", want, stdoutText)
		}
	}
}

// TestSetupPluginUnavailableFallsBackToNative proves D-12: when a
// present, otherwise plugin-capable runtime's capability probe fails
// (nonzero exit, or a seam timeout), registration and the native skills
// copy proceed EXACTLY as today — the row is never failed by the probe —
// and the plugin facet reports nothing but PluginState=unavailable plus
// the reason on PluginNote.
func TestSetupPluginUnavailableFallsBackToNative(t *testing.T) {
	run := func(t *testing.T, listResponse setup.RunResult, listErr error, wantNoteSubstr string) {
		resetClientFlags(t)
		resetCommandFlagState(t, setupCmd)
		withFakeSetupVersion(t, "0.16.1")

		base := counterBase()
		var calls [][]string
		listCallCount := 0
		runFn := func(ctx context.Context, path string, args []string) (setup.RunResult, error) {
			if filepath.Base(path) == "claude" && strings.Join(args, " ") == "plugin list --json" {
				listCallCount++
				return listResponse, listErr
			}
			return base(ctx, path, args)
		}
		withFakeSetupEnv(t, fakeSetupEnvWithRun(recording(&calls, runFn), "claude"))

		mutations := 0
		write, mkdir := skillsEnv.WriteFile, skillsEnv.MkdirAll
		skillsEnv.WriteFile = func(path string, data []byte, mode os.FileMode) error {
			mutations++
			return write(path, data, mode)
		}
		skillsEnv.MkdirAll = func(path string, mode os.FileMode) error {
			mutations++
			return mkdir(path, mode)
		}

		stdout, stderr, err := runClient(t, "setup",
			"--url", "https://engram.example.com/mcp", "--runtime", "claude-code", "--output", "json", "--apply")
		if err != nil {
			t.Fatalf("runClient: %v (stderr=%q stdout=%q)", err, stderr, stdout)
		}
		var doc setupReportDoc
		if uErr := json.Unmarshal([]byte(stdout), &doc); uErr != nil {
			t.Fatalf("json.Unmarshal(%q): %v", stdout, uErr)
		}
		if len(doc.Runtimes) != 1 {
			t.Fatalf("emitted %d rows, want 1: %s", len(doc.Runtimes), stdout)
		}
		row := doc.Runtimes[0]
		if row.Outcome != "wrote" {
			t.Errorf("row.Outcome = %q, want %q", row.Outcome, "wrote")
		}
		if row.Registration != "wrote" {
			t.Errorf("row.Registration = %q, want %q", row.Registration, "wrote")
		}
		if row.Plugin != "" {
			t.Errorf("row.Plugin = %q, want empty", row.Plugin)
		}
		if row.PluginState != "unavailable" {
			t.Errorf("row.PluginState = %q, want %q", row.PluginState, "unavailable")
		}
		if !strings.Contains(row.PluginNote, wantNoteSubstr) {
			t.Errorf("row.PluginNote = %q, want it to contain %q", row.PluginNote, wantNoteSubstr)
		}
		if row.PluginCommand != "" {
			t.Errorf("row.PluginCommand = %q, want empty", row.PluginCommand)
		}
		if row.Skills != "wrote" {
			t.Errorf("row.Skills = %q, want %q (the native copy proceeded exactly as today)", row.Skills, "wrote")
		}
		if row.SkillsNative != "" {
			t.Errorf("row.SkillsNative = %q, want empty", row.SkillsNative)
		}
		if mutations == 0 {
			t.Error("skills mutations = 0, want > 0 (the native copy must proceed exactly as today)")
		}
		if listCallCount != 1 {
			t.Errorf("claude plugin list --json called %d times, want exactly 1", listCallCount)
		}
		for _, call := range calls {
			if len(call) >= 3 && call[0] == "claude" && call[1] == "plugin" && call[2] == "marketplace" {
				t.Errorf("recorded a plugin marketplace probe despite the list probe being unavailable: %q", call)
			}
		}
		assertNoPluginWriteVerb(t, calls)
	}

	t.Run("exit-nonzero", func(t *testing.T) {
		run(t, setup.RunResult{ExitCode: 1, Stderr: "unknown command plugin"}, nil,
			"claude-code: claude plugin list --json exited 1: 'unknown command plugin'")
	})
	t.Run("timeout", func(t *testing.T) {
		run(t, setup.RunResult{}, context.DeadlineExceeded, "timed out after 20s")
	})
}

// TestSetupHelpNamesPluginDelivery is the golden-adjacent assertion that
// setupCmd.Long describes plugin-first delivery (Phase 3): a plugin-
// capable runtime is delivered through engram's own marketplace, mutually
// exclusive with the native copy, updated when outdated, and an existing
// native copy or index block is reported rather than removed — and the
// stale "no separate plugin install is required" claim is gone.
func TestSetupHelpNamesPluginDelivery(t *testing.T) {
	lower := strings.ToLower(setupCmd.Long)
	for _, want := range []string{"plugin", "marketplace", "mutually exclusive", "updated when outdated", "never removed"} {
		if !strings.Contains(lower, want) {
			t.Errorf("setup long description does not mention %q: %s", want, setupCmd.Long)
		}
	}
	if strings.Contains(lower, "separate plugin install") {
		t.Errorf("setup long description still claims a separate plugin install is unnecessary: %s", setupCmd.Long)
	}
}
