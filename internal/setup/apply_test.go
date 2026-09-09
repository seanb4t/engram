// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"context"
	"testing"
)

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
		const probeOutput = `{"url":"https://engram.example.com/mcp"}`
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: probeOutput}}, // probe #1: already registered
			scriptedResult{Result: RunResult{ExitCode: 0}},         // write: codex mcp add (still runs — D-08 always writes)
			scriptedResult{Result: RunResult{Stdout: probeOutput}}, // probe #2: byte-identical
		), "codex")

		res := Apply(context.Background(), env, Codex, opts)
		if res.Outcome != OutcomeAlreadyCorrect {
			t.Fatalf("second Apply outcome = %q, want %q (distinctly from %q)", res.Outcome, OutcomeAlreadyCorrect, OutcomeWrote)
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
