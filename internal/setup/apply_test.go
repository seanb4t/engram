// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// fakeRuntime implements Runtime with a caller-supplied Plan, so
// TestDriftReportedLegibly can drive the shared executor through exact
// Action/Tolerant/Probe combinations without going through a real
// registered runtime's own Plan() logic.
type fakeRuntime struct {
	name string
	plan Plan
}

func (f fakeRuntime) Name() string                            { return f.name }
func (f fakeRuntime) Detect(Environment) bool                 { return true }
func (f fakeRuntime) Plan(Environment, Options) (Plan, error) { return f.plan, nil }

// TestApplyConvergesCodex drives two consecutive Apply calls against codex
// through the scripted Run fake — the only test shape that can prove
// REQ-setup-idempotent's success criterion 5 (03-RESEARCH.md's own
// "Warning signs" note): a plan or implementation that only exercises the
// FIRST --apply run can look correct and still never reach
// OutcomeAlreadyCorrect on a second run.
func TestApplyConvergesCodex(t *testing.T) {
	opts := Options{URL: "https://engram.example.com/mcp", Auth: "oauth"}

	t.Run("first-run-wrote", func(t *testing.T) {
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stderr: "No MCP server named 'engram' found", ExitCode: 1}}, // probe #1: nothing registered yet
			scriptedResult{Result: RunResult{ExitCode: 0}},                                               // write: codex mcp add
			scriptedResult{Result: RunResult{Stdout: `{"url":"https://engram.example.com/mcp"}`}},        // probe #2: now registered
		), "codex")

		res := Apply(context.Background(), env, Codex, opts)
		if res.Outcome != OutcomeWrote {
			t.Fatalf("first Apply outcome = %q, want %q", res.Outcome, OutcomeWrote)
		}
		if res.Binary != "/usr/local/bin/codex" {
			t.Errorf("res.Binary = %q, want %q", res.Binary, "/usr/local/bin/codex")
		}
		if len(calls) != 3 {
			t.Fatalf("Run called %d times, want exactly 3 (probe, write, probe): %+v", len(calls), calls)
		}
		for i, c := range calls {
			if c.Path != "/usr/local/bin/codex" {
				t.Errorf("calls[%d].Path = %q, want the LookPath-resolved absolute path", i, c.Path)
			}
		}
	})

	t.Run("second-run-already-correct", func(t *testing.T) {
		// D-01: already-correct is now a PRE-write claim from probe1's
		// own classification. The former `{"url":...}` body was NEVER a
		// framable codex document (no "name":"engram"), so this subtest
		// used to reach already-correct only via the ambiguous byte-
		// compare fallback — 4 scripted calls including a real write.
		// The no-bearer variant of codexGetEngramBearer IS framable and
		// matches opts (Auth: "oauth" => plannedBearer=false, observed
		// AuthNone), so classifyProbe now returns already-correct after
		// exactly ONE Run call — no write, no probe #2.
		noBearer := strings.Replace(codexGetEngramBearer,
			`"bearer_token_env_var":"ENGRAM_TOKEN"`, `"bearer_token_env_var":null`, 1)
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: noBearer}}, // probe #1: already registered, matches opts
		), "codex")

		res := Apply(context.Background(), env, Codex, opts)
		if res.Outcome != OutcomeAlreadyCorrect {
			t.Fatalf("second Apply outcome = %q, want %q (distinctly from %q)", res.Outcome, OutcomeAlreadyCorrect, OutcomeWrote)
		}
		if len(calls) != 1 {
			t.Fatalf("Run called %d times, want exactly 1 (D-01: already-correct is a pre-write claim, no write/probe#2): %+v", len(calls), calls)
		}
	})

	t.Run("plan-first-args-stays-bare", func(t *testing.T) {
		plan, err := Codex.Plan(fakeEnv("codex"), opts)
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		if got := plan.Actions[0].Args[0]; got != "codex" {
			t.Errorf("Actions[0].Args[0] = %q, want the bare name %q (D-04: Apply resolves the path, Args[0] never does)", got, "codex")
		}

		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{ExitCode: 1}},
			scriptedResult{Result: RunResult{ExitCode: 0}},
			scriptedResult{Result: RunResult{}},
		), "codex")
		res := Apply(context.Background(), env, Codex, opts)
		if res.Binary != "/usr/local/bin/codex" {
			t.Errorf("res.Binary = %q, want the LookPath-resolved absolute path", res.Binary)
		}
	})
}

// TestApplyNotPresentNeverExecs proves a not-present runtime returns
// OutcomeNotPresent without ever touching env.Run — a nil Run field must
// not even be reachable in this path.
func TestApplyNotPresentNeverExecs(t *testing.T) {
	env := fakeEnv() // codex absent, Run left nil
	res := Apply(context.Background(), env, Codex, Options{URL: "https://x", Auth: "oauth"})
	if res.Outcome != OutcomeNotPresent {
		t.Errorf("Apply outcome = %q, want %q", res.Outcome, OutcomeNotPresent)
	}
	if res.Present {
		t.Error("res.Present = true, want false")
	}
}

// TestDriftReportedLegibly covers every 03-01-PLAN.md Task 2 <behavior>
// bullet: every executor failure path yields a Reason an operator can act
// on, and nothing branches on stderr's CONTENT to decide an Outcome
// (D-11). Assertions check for the presence of the runtime name, the argv
// display, and the exit code as SUBSTRINGS of Reason — never the whole
// Reason string, which would pin engram's own prose unnecessarily.
func TestDriftReportedLegibly(t *testing.T) {
	t.Run("non-tolerant-nonzero-with-stderr", func(t *testing.T) {
		rt := fakeRuntime{name: "faketool", plan: Plan{
			Runtime: "faketool",
			Actions: []Action{{Args: []string{"faketool", "mcp", "add"}, Description: "add"}},
		}}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{ExitCode: 3, Stderr: "boom: something broke"}},
		), "faketool")

		res := Apply(context.Background(), env, rt, Options{})
		if res.Outcome != OutcomeFailed {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeFailed)
		}
		for _, want := range []string{"faketool", "mcp add", "3", "boom: something broke"} {
			if !strings.Contains(res.Reason, want) {
				t.Errorf("Reason = %q, want it to contain %q", res.Reason, want)
			}
		}
	})

	t.Run("non-tolerant-nonzero-empty-stderr", func(t *testing.T) {
		rt := fakeRuntime{name: "faketool", plan: Plan{
			Runtime: "faketool",
			Actions: []Action{{Args: []string{"faketool", "mcp", "add"}, Description: "add"}},
		}}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{ExitCode: 7}}, // no stderr at all
		), "faketool")

		res := Apply(context.Background(), env, rt, Options{})
		if res.Outcome != OutcomeFailed {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeFailed)
		}
		if res.Reason == "" {
			t.Fatal("Reason is empty, want a non-empty Reason even with no captured stderr")
		}
		for _, want := range []string{"faketool", "7"} {
			if !strings.Contains(res.Reason, want) {
				t.Errorf("Reason = %q, want it to contain %q", res.Reason, want)
			}
		}
	})

	t.Run("tolerant-nonzero-does-not-fail-row", func(t *testing.T) {
		rt := fakeRuntime{name: "faketool", plan: Plan{
			Runtime: "faketool",
			Actions: []Action{
				{Args: []string{"faketool", "remove"}, Tolerant: true, Description: "clear prior registration"},
				{Args: []string{"faketool", "add"}, Description: "add"},
			},
		}}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{ExitCode: 1, Stderr: "not found"}}, // tolerated
			scriptedResult{Result: RunResult{ExitCode: 0}},                      // add succeeds
		), "faketool")

		res := Apply(context.Background(), env, rt, Options{})
		if res.Outcome == OutcomeFailed {
			t.Fatalf("Outcome = %q, want anything but %q — a tolerated action must never fail the row", res.Outcome, OutcomeFailed)
		}
		if res.Notes == "" {
			t.Error("Notes is empty, want a record of the tolerated nonzero exit")
		}
		if !strings.Contains(res.Notes, "1") {
			t.Errorf("Notes = %q, want it to record the tolerated exit code", res.Notes)
		}
		if len(calls) != 2 {
			t.Fatalf("Run called %d times, want 2 (the tolerated action, then the next one) — sequence must continue past a tolerated failure", len(calls))
		}
	})

	t.Run("probe-seam-error-under-apply-fails", func(t *testing.T) {
		rt := fakeRuntime{name: "faketool", plan: Plan{
			Runtime: "faketool",
			Actions: []Action{{Args: []string{"faketool", "add"}}},
			Probe:   []string{"faketool", "get"},
		}}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Err: errors.New("exec: start failure")},
		), "faketool")

		res := Apply(context.Background(), env, rt, Options{})
		if res.Outcome != OutcomeFailed {
			t.Fatalf("Apply outcome = %q, want %q", res.Outcome, OutcomeFailed)
		}
	})

	t.Run("probe-seam-error-under-preview-stays-would-write", func(t *testing.T) {
		rt := fakeRuntime{name: "faketool", plan: Plan{
			Runtime: "faketool",
			Actions: []Action{{Args: []string{"faketool", "add"}}},
			Probe:   []string{"faketool", "get"},
		}}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Err: errors.New("exec: start failure")},
		), "faketool")

		res := Preview(context.Background(), env, rt, Options{})
		if res.Outcome != OutcomeWouldWrite {
			t.Fatalf("Preview outcome = %q, want %q (a probe seam error must not change the preview classification)", res.Outcome, OutcomeWouldWrite)
		}
	})

	t.Run("probe-seam-deadline-exceeded-names-timeout", func(t *testing.T) {
		rt := fakeRuntime{name: "faketool", plan: Plan{
			Runtime: "faketool",
			Actions: []Action{{Args: []string{"faketool", "add"}}},
			Probe:   []string{"faketool", "get"},
		}}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Err: context.DeadlineExceeded},
		), "faketool")

		res := Apply(context.Background(), env, rt, Options{})
		if res.Outcome != OutcomeFailed {
			t.Fatalf("Apply outcome = %q, want %q", res.Outcome, OutcomeFailed)
		}
		const wantReason = "faketool: faketool get: timed out after 20s: context deadline exceeded"
		if res.Reason != wantReason {
			t.Fatalf("Reason = %q, want %q (D-11)", res.Reason, wantReason)
		}
		if len(calls) != 1 {
			t.Fatalf("Run called %d times, want exactly 1 (the probe) — a probe seam error must not proceed to the write action", len(calls))
		}
	})

	t.Run("probe-seam-canceled-passes-through-unwrapped", func(t *testing.T) {
		rt := fakeRuntime{name: "faketool", plan: Plan{
			Runtime: "faketool",
			Actions: []Action{{Args: []string{"faketool", "add"}}},
			Probe:   []string{"faketool", "get"},
		}}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Err: context.Canceled},
		), "faketool")

		res := Apply(context.Background(), env, rt, Options{})
		if res.Outcome != OutcomeFailed {
			t.Fatalf("Apply outcome = %q, want %q", res.Outcome, OutcomeFailed)
		}
		if res.Reason != "faketool: faketool get: context canceled" {
			t.Fatalf("Reason = %q, want %q (D-11: only DeadlineExceeded is wrapped)", res.Reason, "faketool: faketool get: context canceled")
		}
		if strings.Contains(res.Reason, "timed out") {
			t.Errorf("Reason = %q, want it to NOT contain %q — a cancellation must never be described as a timeout", res.Reason, "timed out")
		}
	})

	t.Run("captured-output-over-budget-truncated-on-rune-boundary", func(t *testing.T) {
		longStderr := strings.Repeat("€", 2000) // 3-byte rune, 6000 bytes total, indivisible by maxCapturedBytes
		rt := fakeRuntime{name: "faketool", plan: Plan{
			Runtime: "faketool",
			Actions: []Action{{Args: []string{"faketool", "add"}}},
		}}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{ExitCode: 1, Stderr: longStderr}},
		), "faketool")

		res := Apply(context.Background(), env, rt, Options{})
		if !utf8.ValidString(res.Reason) {
			t.Fatalf("Reason is not valid UTF-8 after truncation: %q", res.Reason)
		}
		if !strings.Contains(res.Reason, truncationMarker) {
			t.Errorf("Reason = %q, want it to carry the truncation marker %q", res.Reason, truncationMarker)
		}
		if len(res.Reason) >= len(longStderr) {
			t.Errorf("Reason length %d, want it bounded well below the untruncated stderr length %d", len(res.Reason), len(longStderr))
		}
	})

	t.Run("probe-reads-differ-only-beyond-budget-still-wrote", func(t *testing.T) {
		probe1 := strings.Repeat("x", maxCapturedBytes) + "AAAA"
		probe2 := strings.Repeat("x", maxCapturedBytes) + "BBBB" // identical for the first maxCapturedBytes bytes, differs after
		rt := fakeRuntime{name: "faketool", plan: Plan{
			Runtime: "faketool",
			Actions: []Action{{Args: []string{"faketool", "add"}}},
			Probe:   []string{"faketool", "get"},
		}}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: probe1}},
			scriptedResult{Result: RunResult{ExitCode: 0}},
			scriptedResult{Result: RunResult{Stdout: probe2}},
		), "faketool")

		res := Apply(context.Background(), env, rt, Options{})
		if res.Outcome != OutcomeWrote {
			t.Fatalf("Outcome = %q, want %q — a raw byte-compare must never bound before comparing (D-08)", res.Outcome, OutcomeWrote)
		}
	})
}

// TestPreviewReportsRegisteredState covers 03-05-PLAN.md Task 1's
// <behavior> bullets at the package level: probe-zero, probe-nonzero,
// probe-seam-error, not-present, and zero-action (generic) plans, each
// asserting the resulting Outcome and whether Run was invoked at all.
func TestPreviewReportsRegisteredState(t *testing.T) {
	opts := Options{URL: "https://engram.example.com/mcp", Auth: "oauth"}

	// Phase 4: the old raw-capture assertion this subtest pinned no
	// longer holds — Registered is now REBUILT from the parsed-and-
	// redacted observation (D-03), never the raw probe bytes. A codex
	// fixture whose auth is oauth-shaped (no bearer configured) against
	// opts.Auth == "oauth" converges on every facet, so Outcome is
	// already-correct.
	t.Run("probe-zero-exit-reports-normalized-registration", func(t *testing.T) {
		stdout := strings.Replace(codexGetEngramBearer, `"bearer_token_env_var":"ENGRAM_TOKEN"`, `"bearer_token_env_var":null`, 1)
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: stdout}},
		), "codex")

		res := Preview(context.Background(), env, Codex, opts)
		if res.Outcome != OutcomeAlreadyCorrect {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeAlreadyCorrect)
		}
		wantRegistered := "url=https://engram.example.com/mcp auth=none headers=none"
		if res.Registered != wantRegistered {
			t.Errorf("Registered = %q, want %q (D-03: rebuilt from the parsed-and-redacted observation, never the raw probe capture)", res.Registered, wantRegistered)
		}
		if len(calls) != 1 {
			t.Fatalf("Run called %d times, want exactly 1 (the probe, never the write): %+v", len(calls), calls)
		}
	})

	t.Run("probe-nonzero-exit-still-would-write", func(t *testing.T) {
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{ExitCode: 1, Stderr: "No MCP server named 'engram' found"}},
		), "codex")

		res := Preview(context.Background(), env, Codex, opts)
		if res.Outcome != OutcomeWouldWrite {
			t.Fatalf("Outcome = %q, want %q — a probe's nonzero exit must never change a preview's classification (D-10)", res.Outcome, OutcomeWouldWrite)
		}
		if res.Facets != "" {
			t.Errorf("Facets = %q, want empty", res.Facets)
		}
	})

	t.Run("probe-seam-error-still-would-write", func(t *testing.T) {
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Err: errors.New("exec: start failure")},
		), "codex")

		res := Preview(context.Background(), env, Codex, opts)
		if res.Outcome != OutcomeWouldWrite {
			t.Fatalf("Outcome = %q, want %q — a probe seam error must never change a preview's classification (D-10)", res.Outcome, OutcomeWouldWrite)
		}
		if res.Registered != "" {
			t.Errorf("Registered = %q, want empty when the probe never produced a valid read", res.Registered)
		}
		if res.Facets != "" {
			t.Errorf("Facets = %q, want empty", res.Facets)
		}
	})

	t.Run("not-present-never-execs", func(t *testing.T) {
		env := fakeEnvWithRun(func(context.Context, string, []string) (RunResult, error) {
			t.Fatal("Run must never be called for a not-present runtime")
			return RunResult{}, nil
		}) // codex absent — nothing on this fake's PATH

		res := Preview(context.Background(), env, Codex, opts)
		if res.Outcome != OutcomeNotPresent {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeNotPresent)
		}
	})

	t.Run("zero-action-plan-never-execs", func(t *testing.T) {
		env := fakeEnvWithRun(func(context.Context, string, []string) (RunResult, error) {
			t.Fatal("Run must never be called for a zero-Action Plan (D-16) — generic has no Probe and no write")
			return RunResult{}, nil
		})

		res := Preview(context.Background(), env, Generic, opts)
		if res.Outcome != OutcomeWouldWrite {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeWouldWrite)
		}
		if res.Registered != "" {
			t.Errorf("Registered = %q, want empty — generic observes nothing", res.Registered)
		}
	})
}

// TestApplyConvergesClaudeCode is TestApplyConvergesCodex's claude-code
// sibling — the ONLY test shape 03-RESEARCH.md's Pitfall 1 names as
// catching the defect: a plan that only exercises the FIRST --apply run
// can look correct and still never reach OutcomeAlreadyCorrect on a
// second run. claude-code's two-action tolerant-remove-then-fatal-add
// sequence means the FIRST Apply call, against an ambiguous (unframeable)
// probe1, drives 4 scripted Run results (probe, remove, add, probe), not
// codex's 3 — a SECOND, already-converged run is now a D-01 one-call
// no-op instead (below).
func TestApplyConvergesClaudeCode(t *testing.T) {
	opts := Options{URL: "https://engram.example.com/mcp", Auth: "oauth"}

	t.Run("first-run-wrote", func(t *testing.T) {
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stderr: "No MCP server named 'engram' in user scope", ExitCode: 1}}, // probe #1: nothing registered
			scriptedResult{Result: RunResult{ExitCode: 1, Stderr: "No MCP server named 'engram' in user scope"}}, // tolerant remove: slot already empty
			scriptedResult{Result: RunResult{ExitCode: 0}},                                                       // fatal add: succeeds
			scriptedResult{Result: RunResult{Stdout: "engram: https://engram.example.com/mcp (HTTP)"}},           // probe #2: now registered
		), "claude")

		res := Apply(context.Background(), env, ClaudeCode, opts)
		if res.Outcome != OutcomeWrote {
			t.Fatalf("first Apply outcome = %q, want %q", res.Outcome, OutcomeWrote)
		}
		if len(calls) != 4 {
			t.Fatalf("Run called %d times, want exactly 4 (probe, remove, add, probe): %+v", len(calls), calls)
		}
	})

	t.Run("second-run-already-correct", func(t *testing.T) {
		// D-01: already-correct is now a PRE-write claim from probe1's
		// own classification, using a REAL claude-code fixture (a "URL:"
		// line, matching opts) rather than the former bare
		// "engram: https://... (HTTP)" text, which claude-code's Observe
		// cannot frame at all (no "URL:" line) and previously reached
		// already-correct only via the ambiguous byte-compare fallback.
		probeOutput := claudeGetFixture(nil, claudeStatusConnected)
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: probeOutput}}, // probe #1: already registered, matches opts
		), "claude")

		res := Apply(context.Background(), env, ClaudeCode, opts)
		if res.Outcome != OutcomeAlreadyCorrect {
			t.Fatalf("second Apply outcome = %q, want %q — this is the exact defect 03-RESEARCH.md Pitfall 1 names: claude-code must be able to reach already-correct", res.Outcome, OutcomeAlreadyCorrect)
		}
		if len(calls) != 1 {
			t.Fatalf("Run called %d times, want exactly 1 (D-01: already-correct is a pre-write claim, no remove/add/probe#2): %+v", len(calls), calls)
		}
	})
}

// TestApplyPreservedIssuesZeroWrites is REQ-apply-preserve-gate SC1's
// package-level proof: --apply against a probe1 read that classifies
// preserved issues EXACTLY the probe call and nothing else — for BOTH
// parsed runtimes — with the runtime's own D-05 manual-remediation
// sentence on Reason and the observed literal never surviving to the
// marshaled Result.
func TestApplyPreservedIssuesZeroWrites(t *testing.T) {
	t.Run("claude-code", func(t *testing.T) {
		opts := Options{URL: "https://engram.example.com/mcp", Auth: "oauth"}
		stdout := claudeGetFixture([]string{claudeLiteralHeaderLine}, claudeStatusFailedDial)
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: stdout, ExitCode: 0}},
		), "claude")

		res := Apply(context.Background(), env, ClaudeCode, opts)
		if res.Outcome != OutcomePreserved {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomePreserved)
		}
		if res.Facets != "header-name" {
			t.Errorf("Facets = %q, want %q", res.Facets, "header-name")
		}
		wantDrift := "x-litellm-api-key: observed <redacted>, not authored by setup"
		if res.Drift != wantDrift {
			t.Errorf("Drift = %q, want %q", res.Drift, wantDrift)
		}
		wantRegistered := "url=https://engram.example.com/mcp auth=none headers=x-litellm-api-key=<redacted>"
		if res.Registered != wantRegistered {
			t.Errorf("Registered = %q, want %q", res.Registered, wantRegistered)
		}
		wantReason := "claude-code: preserved: " + wantDrift + "; " + claudeCodeWholeEntryNote + "; " + claudeCodeManualRemediation
		if res.Reason != wantReason {
			t.Errorf("Reason = %q, want %q", res.Reason, wantReason)
		}
		if res.Notes != "" {
			t.Errorf("Notes = %q, want empty", res.Notes)
		}
		if len(calls) != 1 {
			t.Fatalf("Run called %d times, want exactly 1: %+v", len(calls), calls)
		}
		wantArgs := []string{"mcp", "get", "engram"}
		if !reflect.DeepEqual(calls[0].Args, wantArgs) {
			t.Errorf("calls[0].Args = %q, want %q", calls[0].Args, wantArgs)
		}
		b, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if strings.Contains(string(b), "sk-DO-NOT-COMMIT-literal-test-abc123") {
			t.Errorf("json.Marshal(res) = %s, must not contain the observed literal", b)
		}
		if got := Classify([]Result{res}); got != ExitTotalSuccess {
			t.Errorf("Classify([]Result{res}) = %v, want %v (D-04: preserved is a non-failed attempt, exit 0)", got, ExitTotalSuccess)
		}
	})

	t.Run("codex", func(t *testing.T) {
		// codexObservedLiteralHeader is 04-OBSERVATIONS.md's own "Codex —
		// literal header (hand-edited)" shape (drift_test.go).
		opts := Options{URL: "https://engram.example.com/mcp", Auth: "oauth"}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: codexObservedLiteralHeader, ExitCode: 0}},
		), "codex")

		res := Apply(context.Background(), env, Codex, opts)
		if res.Outcome != OutcomePreserved {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomePreserved)
		}
		if len(calls) != 1 {
			t.Fatalf("Run called %d times, want exactly 1: %+v", len(calls), calls)
		}
		if !strings.HasSuffix(res.Reason, codexManualRemediation) {
			t.Errorf("Reason = %q, want it to end with %q", res.Reason, codexManualRemediation)
		}
	})
}

// TestApplyPreservedNeverRunsClaudeCodeRemove is SC2's structural proof
// (04-RESEARCH.md Pitfall 2): a preserved classification never dispatches
// claudeCodeRemoveAction, which is plan.Actions[0] for every claude-code
// auth mode. Two independent checks are BOTH required — a count-only
// assertion could pass if some OTHER action silently replaced the remove
// call: scriptedRun's own panic-on-overrun is the structural proof (one
// scripted result; a second Run call panics the test), and the explicit
// argv scan below names the offending call if the loop is ever entered.
func TestApplyPreservedNeverRunsClaudeCodeRemove(t *testing.T) {
	opts := Options{URL: "https://engram.example.com/mcp", Auth: "oauth"}
	stdout := claudeGetFixture([]string{claudeLiteralHeaderLine}, claudeStatusFailedDial)
	var calls []runCall
	env := fakeEnvWithRun(scriptedRun(&calls,
		scriptedResult{Result: RunResult{Stdout: stdout, ExitCode: 0}},
	), "claude")

	res := Apply(context.Background(), env, ClaudeCode, opts)
	if res.Outcome != OutcomePreserved {
		t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomePreserved)
	}
	for _, c := range calls {
		if len(c.Args) >= 2 && c.Args[0] == "mcp" && (c.Args[1] == "remove" || c.Args[1] == "add") {
			t.Fatalf("recorded a claude-code registration write call, want none: %+v", c)
		}
	}
	if len(calls) != 1 {
		t.Fatalf("Run called %d times, want exactly 1: %+v", len(calls), calls)
	}
}

// TestApplyAlreadyCorrectIssuesZeroWrites proves already-correct is a
// TRUE one-call no-op for both parsed runtimes under --apply (D-01),
// including on a SECOND, freshly-scripted call — the idempotency truth
// REQ-apply-preserve-gate's own success criteria name explicitly.
func TestApplyAlreadyCorrectIssuesZeroWrites(t *testing.T) {
	cases := []struct {
		name string
		rt   Runtime
	}{
		{"claude-code", ClaudeCode},
		{"codex", Codex},
	}
	opts := Options{URL: "https://engram.example.com/mcp", Auth: "oauth"}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fixture := func() string {
				if c.name == "claude-code" {
					return claudeGetFixture(nil, claudeStatusConnected)
				}
				return strings.Replace(codexGetEngramBearer,
					`"bearer_token_env_var":"ENGRAM_TOKEN"`, `"bearer_token_env_var":null`, 1)
			}

			assertOnce := func(t *testing.T) {
				t.Helper()
				var calls []runCall
				env := fakeEnvWithRun(scriptedRun(&calls,
					scriptedResult{Result: RunResult{Stdout: fixture(), ExitCode: 0}},
				), map[string]string{"claude-code": "claude", "codex": "codex"}[c.name])

				res := Apply(context.Background(), env, c.rt, opts)
				if res.Outcome != OutcomeAlreadyCorrect {
					t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeAlreadyCorrect)
				}
				if res.Facets != "" {
					t.Errorf("Facets = %q, want empty", res.Facets)
				}
				if res.Drift != "" {
					t.Errorf("Drift = %q, want empty", res.Drift)
				}
				if res.Reason != "" {
					t.Errorf("Reason = %q, want empty", res.Reason)
				}
				if res.Notes != "" {
					t.Errorf("Notes = %q, want empty", res.Notes)
				}
				wantRegistered := "url=https://engram.example.com/mcp auth=none headers=none"
				if res.Registered != wantRegistered {
					t.Errorf("Registered = %q, want %q", res.Registered, wantRegistered)
				}
				if len(calls) != 1 {
					t.Fatalf("Run called %d times, want exactly 1: %+v", len(calls), calls)
				}
			}

			// The idempotency truth: a SECOND Apply call, freshly
			// scripted with the same converged fixture, is the SAME
			// one-call no-op — not merely "the first call happened to
			// look right".
			assertOnce(t)
			assertOnce(t)
		})
	}
}

// TestApplyWroteRegisteredIsRedacted is D-02's proof: after a real write,
// Registered is rebuilt from the POST-write probe through the SAME
// Observe -> renderObservation path Preview uses — never the raw probe2
// capture — so a literal a runtime's CLI echoes back after registering
// can never reach Registered or the marshaled Result.
func TestApplyWroteRegisteredIsRedacted(t *testing.T) {
	opts := Options{
		URL:  "https://engram.example.com/mcp",
		Auth: "oauth",
		Headers: []HeaderSpec{
			{Name: "x-gateway-api-key", EnvVar: "GATEWAY_KEY"},
		},
	}
	const sentinel = "SENTINEL-POSTWRITE-9a1c-DO-NOT-LEAK"

	t.Run("redacted-on-success", func(t *testing.T) {
		probe1 := strings.Replace(
			claudeGetFixture([]string{"x-gateway-api-key: ${GATEWAY_KEY}"}, claudeStatusFailedDial),
			"URL: https://engram.example.com/mcp", "URL: https://old.example/mcp", 1)
		probe2 := claudeGetFixture([]string{"x-gateway-api-key: " + sentinel}, claudeStatusConnected)

		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: probe1, ExitCode: 0}}, // probe #1: would-write on url
			scriptedResult{Result: RunResult{ExitCode: 0}},                 // tolerant remove
			scriptedResult{Result: RunResult{ExitCode: 0}},                 // fatal add
			scriptedResult{Result: RunResult{Stdout: probe2, ExitCode: 0}}, // probe #2: literal echo
		), "claude")

		res := Apply(context.Background(), env, ClaudeCode, opts)
		if res.Outcome != OutcomeWrote {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeWrote)
		}
		if res.Facets != "url" {
			t.Errorf("Facets = %q, want %q", res.Facets, "url")
		}
		wantDrift := "url: observed https://old.example/mcp, would write https://engram.example.com/mcp"
		if res.Drift != wantDrift {
			t.Errorf("Drift = %q, want %q", res.Drift, wantDrift)
		}
		wantRegistered := "url=https://engram.example.com/mcp auth=none headers=x-gateway-api-key=<redacted>"
		if res.Registered != wantRegistered {
			t.Errorf("Registered = %q, want %q", res.Registered, wantRegistered)
		}
		if len(calls) != 4 {
			t.Fatalf("Run called %d times, want exactly 4: %+v", len(calls), calls)
		}
		if calls[1].Args[0] != "mcp" || calls[1].Args[1] != "remove" {
			t.Errorf("calls[1].Args = %q, want a %q call", calls[1].Args, "mcp remove")
		}
		if calls[2].Args[0] != "mcp" || calls[2].Args[1] != "add" {
			t.Errorf("calls[2].Args = %q, want a %q call", calls[2].Args, "mcp add")
		}
		b, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		for _, field := range []string{string(b), res.Registered, res.Reason, res.Drift, res.Notes} {
			if strings.Contains(field, sentinel) {
				t.Errorf("field %q leaks the post-write sentinel", field)
			}
		}
	})

	t.Run("probe2-unframeable-leaves-registered-empty", func(t *testing.T) {
		probe1 := strings.Replace(
			claudeGetFixture([]string{"x-gateway-api-key: ${GATEWAY_KEY}"}, claudeStatusFailedDial),
			"URL: https://engram.example.com/mcp", "URL: https://old.example/mcp", 1)

		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: probe1, ExitCode: 0}},
			scriptedResult{Result: RunResult{ExitCode: 0}},
			scriptedResult{Result: RunResult{ExitCode: 0}},
			scriptedResult{Result: RunResult{Stdout: "garbage", ExitCode: 1}}, // probe #2: unframeable
		), "claude")

		res := Apply(context.Background(), env, ClaudeCode, opts)
		if res.Outcome != OutcomeWrote {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeWrote)
		}
		if res.Registered != "" {
			t.Errorf("Registered = %q, want empty (never the raw probe2 bytes)", res.Registered)
		}
		if strings.Contains(res.Registered, "garbage") {
			t.Errorf("Registered = %q, must not contain the raw probe2 capture", res.Registered)
		}
	})

	t.Run("probe2-seam-error-still-wrote", func(t *testing.T) {
		probe1 := strings.Replace(
			claudeGetFixture([]string{"x-gateway-api-key: ${GATEWAY_KEY}"}, claudeStatusFailedDial),
			"URL: https://engram.example.com/mcp", "URL: https://old.example/mcp", 1)

		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: probe1, ExitCode: 0}},
			scriptedResult{Result: RunResult{ExitCode: 0}},
			scriptedResult{Result: RunResult{ExitCode: 0}},
			scriptedResult{Err: errors.New("boom")}, // probe #2: seam error
		), "claude")

		res := Apply(context.Background(), env, ClaudeCode, opts)
		if res.Outcome != OutcomeWrote {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeWrote)
		}
		if res.Registered != "" {
			t.Errorf("Registered = %q, want empty", res.Registered)
		}
	})
}

// TestApplyToleratesClearSlotFailure proves that when claude-code's
// tolerant clear-the-slot action exits nonzero (the slot was already
// empty) and the following registration action succeeds, the row is NOT
// failed and Notes records the tolerated action.
func TestApplyToleratesClearSlotFailure(t *testing.T) {
	var calls []runCall
	env := fakeEnvWithRun(scriptedRun(&calls,
		scriptedResult{Result: RunResult{ExitCode: 1, Stderr: "No MCP server named 'engram' in user scope"}}, // probe #1
		scriptedResult{Result: RunResult{ExitCode: 1, Stderr: "No MCP server named 'engram' in user scope"}}, // tolerant remove: fails, tolerated
		scriptedResult{Result: RunResult{ExitCode: 0}},                                                       // fatal add: succeeds
		scriptedResult{Result: RunResult{Stdout: "engram registered"}},                                       // probe #2
	), "claude")

	res := Apply(context.Background(), env, ClaudeCode, Options{URL: "https://x", Auth: "oauth"})
	if res.Outcome == OutcomeFailed {
		t.Fatalf("Outcome = %q, want anything but %q — a tolerated action must never fail the row", res.Outcome, OutcomeFailed)
	}
	if res.Notes == "" {
		t.Error("Notes is empty, want a record of the tolerated clear-the-slot exit")
	}
}

// TestApplyFailsWhenRegistrationActionFails proves that when claude-code's
// tolerant clear-the-slot action SUCCEEDS (an existing registration was
// actually removed) and the following fatal registration action then
// fails, the row IS OutcomeFailed — this is the destructive-window case
// the Task 1 checkpoint accepted: the operator is left with no claude-code
// registration where they previously had one.
func TestApplyFailsWhenRegistrationActionFails(t *testing.T) {
	var calls []runCall
	env := fakeEnvWithRun(scriptedRun(&calls,
		scriptedResult{Result: RunResult{Stdout: "engram: https://engram.example.com/mcp (HTTP)"}}, // probe #1: existing registration
		scriptedResult{Result: RunResult{ExitCode: 0}},                                             // tolerant remove: succeeds, clears the slot
		scriptedResult{Result: RunResult{ExitCode: 1, Stderr: "unknown flag: --transport"}},        // fatal add: fails
	), "claude")

	res := Apply(context.Background(), env, ClaudeCode, Options{URL: "https://x", Auth: "oauth"})
	if res.Outcome != OutcomeFailed {
		t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeFailed)
	}
	if res.Reason == "" {
		t.Error("Reason is empty, want a non-empty Reason naming the failing action")
	}
}

// TestActionToleranceIsAuthoredNotPositional pins the Assumptions Log A4
// decision 03-02-PLAN.md's Task 1 checkpoint made explicitly: an action's
// failure tolerance comes ONLY from its own authored Action.Tolerant
// field, never from its position in Plan.Actions. 03-RESEARCH.md's
// Pattern 1 recommended the opposite — a general "only the LAST action's
// exit determines OutcomeFailed" positional rule — and this phase
// rejected it because plan.go's own doc comment records Plan.Actions as
// growable by a later phase (skills distribution, Phase 4): a positional
// rule would silently make every later, non-final action tolerant the
// moment Actions grows past length 2, which is exactly the kind of
// latent defect that ships green and is discovered only in production.
//
// The synthetic Plan below is built inline, deliberately NOT obtained
// from any registered runtime's own Plan() — this test must prove the
// EXECUTOR's rule, not accidentally validate a runtime author's ordering
// choice. Two orderings:
//
//   - tolerant-last: Actions[0] non-tolerant and FAILING,
//     Actions[len-1] tolerant and succeeding. Under the rejected
//     positional rule ("only the last action's exit fails the row"), the
//     first action's failure would be silently swallowed and the row
//     would NOT be OutcomeFailed — exactly the regression this test
//     exists to catch if anyone ever reintroduces a position-derived
//     tolerance rule. Under the correct authored rule, Actions[0]'s
//     failure is fatal regardless of its position, so the row MUST be
//     OutcomeFailed.
//   - tolerant-first (inverted): Actions[0] tolerant and FAILING,
//     Actions[len-1] non-tolerant and succeeding — the shape
//     claudecode.go's remove-then-add sequence actually uses. The row
//     must NOT be OutcomeFailed, proving tolerance is read from the
//     field on the FIRST action too, not merely "not the last one".
func TestActionToleranceIsAuthoredNotPositional(t *testing.T) {
	t.Run("tolerant-last", func(t *testing.T) {
		rt := fakeRuntime{name: "faketool", plan: Plan{
			Runtime: "faketool",
			Actions: []Action{
				{Args: []string{"faketool", "add"}, Tolerant: false, Description: "first, non-tolerant, FAILS"},
				{Args: []string{"faketool", "verify"}, Tolerant: true, Description: "last, tolerant, succeeds"},
			},
		}}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{ExitCode: 1, Stderr: "boom"}}, // first action fails
			scriptedResult{Result: RunResult{ExitCode: 0}},                 // never reached if the executor is correct
		), "faketool")

		res := Apply(context.Background(), env, rt, Options{})
		if res.Outcome != OutcomeFailed {
			t.Fatalf("Outcome = %q, want %q — a non-tolerant FIRST action's failure must fail the row even though a LATER action is tolerant (a positional \"only the last action counts\" rule would wrongly pass this)", res.Outcome, OutcomeFailed)
		}
		if len(calls) != 1 {
			t.Errorf("Run called %d times, want exactly 1 — the sequence must stop at the first non-tolerant failure, never run a later action to find its tolerance", len(calls))
		}
	})

	t.Run("tolerant-first", func(t *testing.T) {
		rt := fakeRuntime{name: "faketool", plan: Plan{
			Runtime: "faketool",
			Actions: []Action{
				{Args: []string{"faketool", "remove"}, Tolerant: true, Description: "first, tolerant, FAILS (tolerated)"},
				{Args: []string{"faketool", "add"}, Tolerant: false, Description: "last, non-tolerant, succeeds"},
			},
		}}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{ExitCode: 1, Stderr: "not found"}}, // tolerated
			scriptedResult{Result: RunResult{ExitCode: 0}},                      // succeeds
		), "faketool")

		res := Apply(context.Background(), env, rt, Options{})
		if res.Outcome == OutcomeFailed {
			t.Fatalf("Outcome = %q, want anything but %q — a tolerant FIRST action's failure must never fail the row", res.Outcome, OutcomeFailed)
		}
		if len(calls) != 2 {
			t.Fatalf("Run called %d times, want 2 — the sequence must continue past a tolerated failure regardless of position", len(calls))
		}
	})
}

// TestPreviewNeverExecutesWriteAction proves Preview() against a present
// runtime never runs the write Action — only Detect/Plan/LookPath, plus
// (if wired) a read-only probe. Scripting the write action's response as
// a panic-inducing exhaustion of the script proves it was never reached.
func TestPreviewNeverExecutesWriteAction(t *testing.T) {
	var calls []runCall
	env := fakeEnvWithRun(scriptedRun(&calls,
		scriptedResult{Result: RunResult{Stdout: "existing state"}}, // probe #1 only
	), "codex")

	res := Preview(context.Background(), env, Codex, Options{URL: "https://x", Auth: "oauth"})
	if res.Outcome != OutcomeWouldWrite {
		t.Fatalf("Preview outcome = %q, want %q", res.Outcome, OutcomeWouldWrite)
	}
	if len(calls) > 1 {
		t.Fatalf("Run called %d times during Preview, want at most 1 (the probe, never the write): %+v", len(calls), calls)
	}
}

// TestEveryActionArgsValidated pins that execute() validates EVERY action's
// Args, not only Actions[0]'s. The loop that runs the actions slices
// action.Args[1:] for each one, and a zero-length slice panics there
// ("slice bounds out of range [1:0]") — a panic no recover() in the call
// chain catches, so it would crash the whole `--apply` invocation and lose
// every OTHER runtime's row, contradicting the per-runtime isolation the
// executor is built for.
//
// This is not hypothetical scope: Plan.Actions is documented as growable
// (claude-code already authors two actions, and Phase 4's skills
// distribution appends more), and it was exactly that growability that
// justified keeping Action.Tolerant an authored field rather than a
// positional rule. A guard that only ever looked at index 0 was correct
// when Plans held one action and silently stopped being correct when they
// did not.
func TestEveryActionArgsValidated(t *testing.T) {
	// A well-formed first action followed by a malformed second one: the
	// index-0-only guard passes this, then the run loop panics on it.
	rt := fakeRuntime{name: "faketool", plan: Plan{
		Runtime: "faketool",
		Actions: []Action{
			{Args: []string{"faketool", "mcp", "remove"}, Tolerant: true, Description: "clear the slot"},
			{Args: nil, Description: "malformed — authored with no Args"},
		},
	}}
	var calls []runCall
	env := fakeEnvWithRun(scriptedRun(&calls,
		scriptedResult{Result: RunResult{ExitCode: 0}},
	), "faketool")

	// Apply must RETURN a failed row, never panic.
	res := Apply(context.Background(), env, rt, Options{})

	if res.Outcome != OutcomeFailed {
		t.Fatalf("Outcome = %q, want %q — a malformed action must fail its own row", res.Outcome, OutcomeFailed)
	}
	if !strings.Contains(res.Reason, "faketool") {
		t.Errorf("Reason = %q, want it to name the runtime", res.Reason)
	}
	// The index is what makes the report actionable: "some action is
	// malformed" does not tell an author which one to fix.
	if !strings.Contains(res.Reason, "1") {
		t.Errorf("Reason = %q, want it to name the offending action's index (1)", res.Reason)
	}
	// Fail BEFORE running anything: a malformed Plan is an authoring bug,
	// and running its well-formed prefix first would half-apply it. For
	// claude-code's shape that prefix is the tolerant `mcp remove`, so
	// running it would clear a real registration on behalf of a Plan that
	// was never going to complete.
	if len(calls) != 0 {
		t.Errorf("Run called %d times, want 0 — validation must precede execution: %+v", len(calls), calls)
	}
}

// TestThirdPartyCaptureIsQuotedForDisplay closes T-03-04's Tampering half.
//
// Registered, Reason and Notes are the first report fields carrying bytes
// engram did not author — they are whatever the third-party runtime CLI
// wrote to stdout/stderr. Command is quoted (D-02/#523) precisely because
// a human may paste it, and these fields render on adjacent key=value
// lines, so an operator has every reason to read the whole report line as
// paste-safe.
//
// boundCapture alone does not make them so: it closes the flooding facet,
// and cmd/engram's sanitizeViewValue closes the control-character facet
// (it maps only r < 0x20 || r == 0x7f), but every shell metacharacter is
// printable and passes both untouched. That is phase 02's T-02-13 (control
// chars, mitigated) being mistaken for T-02-05 (shell metachars, not
// covered) — the false-closure trap those adjacent rows set.
//
// The control belongs HERE, at the boundary the untrusted bytes cross,
// not at the display layer: sanitizeViewValue is shared by every operator
// view in this CLI, and memory content, tags, scopes and summaries all
// legitimately carry arbitrary printable characters that must NOT be
// quoted.
func TestThirdPartyCaptureIsQuotedForDisplay(t *testing.T) {
	// A capture that is hostile if pasted into a shell.
	const hostile = "engram: connected; rm -rf ~"

	quotedForm := func(t *testing.T, field, value string) {
		t.Helper()
		if value == hostile {
			t.Errorf("%s = %q — the raw third-party capture reached the field verbatim; a paste executes the trailing command", field, value)
			return
		}
		if !strings.HasPrefix(value, "'") || !strings.HasSuffix(value, "'") {
			t.Errorf("%s = %q, want it single-quoted so a paste treats it as one inert word", field, value)
		}
		if !strings.Contains(value, hostile) {
			t.Errorf("%s = %q, want the capture's text preserved inside the quotes — quoting must not destroy the operator's information", field, value)
		}
	}

	t.Run("registered-from-probe-stdout", func(t *testing.T) {
		rt := fakeRuntime{name: "faketool", plan: Plan{
			Runtime: "faketool",
			Actions: []Action{{Args: []string{"faketool", "mcp", "add"}}},
			Probe:   []string{"faketool", "mcp", "get"},
		}}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: hostile}}, // probe #1
			scriptedResult{Result: RunResult{ExitCode: 0}},     // write
			scriptedResult{Result: RunResult{Stdout: hostile}}, // probe #2
		), "faketool")

		res := Apply(context.Background(), env, rt, Options{})
		quotedForm(t, "Registered", res.Registered)

		// T-03-08 regression guard: the convergence byte-compare reads the
		// RAW probe captures, never the display field. Quoting the field
		// must not perturb the classification — identical captures still
		// converge to already-correct.
		if res.Outcome != OutcomeAlreadyCorrect {
			t.Errorf("Outcome = %q, want %q — quoting the display field must not affect the byte-compare, which reads the raw captures", res.Outcome, OutcomeAlreadyCorrect)
		}
	})

	t.Run("reason-from-failing-stderr", func(t *testing.T) {
		rt := fakeRuntime{name: "faketool", plan: Plan{
			Runtime: "faketool",
			Actions: []Action{{Args: []string{"faketool", "mcp", "add"}}},
		}}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{ExitCode: 3, Stderr: hostile}},
		), "faketool")

		res := Apply(context.Background(), env, rt, Options{})
		if res.Outcome != OutcomeFailed {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeFailed)
		}
		if strings.Contains(res.Reason, hostile) && !strings.Contains(res.Reason, "'"+hostile+"'") {
			t.Errorf("Reason = %q — the raw stderr reached it unquoted", res.Reason)
		}
	})

	t.Run("notes-from-tolerated-stderr", func(t *testing.T) {
		rt := fakeRuntime{name: "faketool", plan: Plan{
			Runtime: "faketool",
			Actions: []Action{
				{Args: []string{"faketool", "mcp", "remove"}, Tolerant: true, Description: "clear the slot"},
				{Args: []string{"faketool", "mcp", "add"}},
			},
		}}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{ExitCode: 1, Stderr: hostile}}, // tolerated
			scriptedResult{Result: RunResult{ExitCode: 0}},                  // add
		), "faketool")

		res := Apply(context.Background(), env, rt, Options{})
		if res.Notes == "" {
			t.Fatalf("Notes is empty; want the tolerated action recorded")
		}
		if strings.Contains(res.Notes, hostile) && !strings.Contains(res.Notes, "'"+hostile+"'") {
			t.Errorf("Notes = %q — the raw tolerated stderr reached it unquoted", res.Notes)
		}
	})

	// Positive control: quoting must not become blanket noise. A capture
	// made entirely of safe runes renders bare, exactly as quoteWord's own
	// safe-set table promises — otherwise every ordinary report line would
	// grow quotes and the signal would be lost.
	t.Run("safe-capture-stays-bare", func(t *testing.T) {
		const benign = "connected"
		rt := fakeRuntime{name: "faketool", plan: Plan{
			Runtime: "faketool",
			Actions: []Action{{Args: []string{"faketool", "mcp", "add"}}},
			Probe:   []string{"faketool", "mcp", "get"},
		}}
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: benign}},
			scriptedResult{Result: RunResult{ExitCode: 0}},
			scriptedResult{Result: RunResult{Stdout: benign}},
		), "faketool")

		res := Apply(context.Background(), env, rt, Options{})
		if res.Registered != benign {
			t.Errorf("Registered = %q, want %q rendered bare — a safe capture must not gain quotes", res.Registered, benign)
		}
	})
}

// TestDriftFieldsStayBoundedAgainstOversizedProbeContent covers WR-01
// (04-REVIEW.md): an observed header NAME or URL is untrusted third-party
// content — parsed straight out of codex's `mcp get --json` stdout by
// codexRuntime.Observe, with no length cap of its own (unlike D-11's
// Unrecognized labels, which codex.go's own unrecognizedLabelBound
// already caps at 40 bytes). Result.Drift/Registered/Reason, built from
// the drift-compare path (Compare/renderObservation), must stay bounded
// to maxCapturedBytes the same way the mutate lane's own
// displayCapture(probe2.Stdout+probe2.Stderr) always has — never flood
// the operator's terminal or --output json from a single oversized
// header name or URL.
func TestDriftFieldsStayBoundedAgainstOversizedProbeContent(t *testing.T) {
	hugeName := strings.Repeat("A", 100_000)
	hugeURL := "https://engram.example.com/" + strings.Repeat("x", 100_000)

	stdout := strings.Replace(codexGetEngramBearer, `"url":"https://engram.example.com/mcp"`, `"url":"`+hugeURL+`"`, 1)
	stdout = strings.Replace(stdout, `"http_headers":null`, `"http_headers":{"`+hugeName+`":"v"}`, 1)
	if !strings.Contains(stdout, hugeURL) || !strings.Contains(stdout, hugeName) {
		t.Fatal("fixture setup failed: expected replacements did not land")
	}

	// opts.Auth stays "oauth" (not "bearer") against a fixture whose
	// bearer_token_env_var is codex's own authored form, and opts.URL
	// deliberately does not match hugeURL — both an auth-mode facet and
	// a would-write URL facet fire alongside the unplanned (preserved)
	// header, so this single scripted probe exercises Drift, Registered,
	// AND Reason (OutcomePreserved) all at once.
	opts := Options{URL: "https://engram.example.com/mcp", Auth: "oauth"}
	var calls []runCall
	env := fakeEnvWithRun(scriptedRun(&calls,
		scriptedResult{Result: RunResult{Stdout: stdout}},
	), "codex")

	res := Preview(context.Background(), env, Codex, opts)

	if res.Outcome != OutcomePreserved {
		t.Fatalf("Outcome = %q, want %q (an unplanned observed header is always a preserved cause)", res.Outcome, OutcomePreserved)
	}

	// maxCapturedBytes plus truncationMarker's own length is the hard
	// ceiling boundCapture ever produces; a little slack covers the
	// fixed prose boundCapture's caller prepends (e.g. "codex: preserved: ").
	const maxAllowed = maxCapturedBytes + len(truncationMarker) + 64
	for _, tc := range []struct {
		field string
		got   string
	}{
		{"Drift", res.Drift},
		{"Registered", res.Registered},
		{"Reason", res.Reason},
	} {
		if len(tc.got) > maxAllowed {
			t.Errorf("len(res.%s) = %d, want <= %d (bounded via boundCapture, WR-01)", tc.field, len(tc.got), maxAllowed)
		}
		if strings.Contains(tc.got, hugeName) {
			t.Errorf("res.%s contains the full 100KB oversized header name unbounded", tc.field)
		}
		if strings.Contains(tc.got, hugeURL) {
			t.Errorf("res.%s contains the full 100KB oversized URL unbounded", tc.field)
		}
	}
}
