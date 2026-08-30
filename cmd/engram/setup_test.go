// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanb4t/engram/internal/setup"
)

// fakeSetupEnv builds a setup.Environment resolving only the binaries
// named present — swapped onto the package-level setupEnv seam so a test
// drives detection without touching the real machine's PATH (repo rule
// m45p2b4bp7: never test or red-gate third-party CLI behavior).
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
	}
}

// withFakeSetupEnv points the package-level setupEnv seam at env for the
// duration of the test.
func withFakeSetupEnv(t *testing.T, env setup.Environment) {
	t.Helper()
	orig := setupEnv
	setupEnv = env
	t.Cleanup(func() { setupEnv = orig })
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
		want := "claude mcp add --transport http engram https://engram.example.com/mcp --scope user"
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

	_, _, err := runClient(t, "setup", "--output", "json")
	if err != nil {
		t.Fatalf("runClient: %v", err)
	}
	if want := len(setup.Names()); lookupCalls != want {
		t.Errorf("LookPath called %d times, want exactly %d (one read-only detection call per registered runtime, no execution)", lookupCalls, want)
	}
}

// TestSetupApplyReturnsErrorNotPanic proves `engram setup --apply` returns
// a non-nil error and does not panic — Apply() is stubbed this phase
// (D-09).
func TestSetupApplyReturnsErrorNotPanic(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, setupCmd)
	withFakeSetupEnv(t, fakeSetupEnv("claude"))

	_, _, err := runClient(t, "setup", "--url", "https://engram.example.com/mcp", "--apply")
	if err == nil {
		t.Fatal("engram setup --apply = nil error, want a non-nil error (Apply is stubbed this phase)")
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

	stdout, _, err := runClient(t, "setup", "--runtime", "codex", "--output", "json")
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

	stdout, _, err := runClient(t, "setup", "--output", "json")
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
// bearer --token-file /tmp/does-not-exist --output json` exits 0, and the
// emitted command field contains the provenance form and does not contain
// the string "Bearer eyJ" or any other token-shaped value — this is a
// nonexistent path, so Plan() reading it and leaking real content is
// structurally impossible, but the string-shape assertion pins the
// REDACTED FORM itself.
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
		if !strings.Contains(row.Command, "Bearer <from /tmp/does-not-exist>") {
			t.Errorf("%s row.Command = %q, want it to contain the provenance form %q", row.Name, row.Command, "Bearer <from /tmp/does-not-exist>")
		}
		if strings.Contains(row.Command, "Bearer eyJ") {
			t.Errorf("%s row.Command = %q, want it to NOT contain a token-shaped value", row.Name, row.Command)
		}
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
	if len(doc.Runtimes) != len(setup.Names()) {
		t.Fatalf("setup --apply emitted %d rows, want %d: %s", len(doc.Runtimes), len(setup.Names()), stdout)
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
	withFakeSetupEnv(t, fakeSetupEnv("claude"))

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
	if len(doc.Runtimes) != len(setup.Names()) {
		t.Fatalf("setup --apply emitted %d rows, want %d (one per selected runtime, none collapsed): %s", len(doc.Runtimes), len(setup.Names()), stdout)
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

			_, stderr, err := runClient(t, "setup", "--output", "json")
			if err != nil {
				t.Fatalf("runClient: %v (stderr=%q)", err, stderr)
			}
		})
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
	if len(doc.Runtimes) != len(setup.Names()) {
		t.Fatalf("setup --output json --apply emitted %d rows, want %d: %s", len(doc.Runtimes), len(setup.Names()), stdout)
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
