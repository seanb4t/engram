// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

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
// to create.
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
	})
}

// TestSetupPreviewNeverClassifiesAlreadyCorrect scripts a probe returning
// IDENTICAL output on every call and asserts no row's outcome is ever
// "already-correct" under a bare preview (D-10): byte-compare needs a
// WRITE between two reads, and a preview never writes, so no single probe
// read — however convincing — has an honest basis for that classification.
func TestSetupPreviewNeverClassifiesAlreadyCorrect(t *testing.T) {
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
		if row.Outcome == string(setup.OutcomeAlreadyCorrect) {
			t.Errorf("%s row.Outcome = %q, want never %q under a bare preview (D-10)", row.Name, row.Outcome, setup.OutcomeAlreadyCorrect)
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
// and states what --apply does.
func TestSetupHelpNamesEveryRuntimeAndAuthMode(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "help.golden"))
	if err != nil {
		t.Fatalf("read help.golden: %v", err)
	}
	section := helpGoldenSection(t, string(data), "engram setup")
	for _, want := range []string{
		"claude-code", "codex", "opencode",
		"oauth", "oauth-client", "bearer", "none",
		"apply",
	} {
		if !strings.Contains(section, want) {
			t.Errorf("## engram setup section does not contain %q:\n%s", want, section)
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
// (scripted via skillsEnv); codex — not yet wired for skills this wave —
// simply registers successfully with no skills facet at all. Both rows
// must still render in the captured output before the nonzero exit
// (T-02-06).
func TestSetupSkillsFailureReachesPartialExit(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("claude", "codex"))

	// Override the auto-faked skillsEnv (withFakeSetupEnv) with one whose
	// every write fails — claude-code is the only runtime this wave whose
	// Plan() authors a recognized SkillTarget, so this failure reaches
	// exactly one row's skills facet, never codex's (it has none).
	skillsEnv = skills.Environment{
		ReadFile:  func(string) ([]byte, error) { return nil, os.ErrNotExist },
		WriteFile: func(string, []byte, os.FileMode) error { return errors.New("boom: disk full") },
		MkdirAll:  func(string, os.FileMode) error { return nil },
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
