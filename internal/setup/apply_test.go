// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"context"
	"errors"
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

	t.Run("probe-zero-exit-reports-registered", func(t *testing.T) {
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: "engram: https://engram.example.com/mcp (HTTP)"}},
		), "codex")

		res := Preview(context.Background(), env, Codex, opts)
		if res.Outcome != OutcomeWouldWrite {
			t.Fatalf("Outcome = %q, want %q", res.Outcome, OutcomeWouldWrite)
		}
		if res.Registered == "" {
			t.Error("Registered is empty, want the bounded probe output (D-10)")
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
// sequence means each Apply call drives 4 scripted Run results (probe,
// remove, add, probe), not codex's 3.
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
		const probeOutput = "engram: https://engram.example.com/mcp (HTTP)"
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: probeOutput}}, // probe #1: already registered
			scriptedResult{Result: RunResult{ExitCode: 0}},         // tolerant remove: clears the slot
			scriptedResult{Result: RunResult{ExitCode: 0}},         // fatal add: re-registers identically
			scriptedResult{Result: RunResult{Stdout: probeOutput}}, // probe #2: byte-identical
		), "claude")

		res := Apply(context.Background(), env, ClaudeCode, opts)
		if res.Outcome != OutcomeAlreadyCorrect {
			t.Fatalf("second Apply outcome = %q, want %q — this is the exact defect 03-RESEARCH.md Pitfall 1 names: claude-code must be able to reach already-correct", res.Outcome, OutcomeAlreadyCorrect)
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
