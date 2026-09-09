// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
)

// fakeEnv builds an Environment whose LookPath resolves only the binaries
// named present (returning a fake absolute path) and returns
// exec.ErrNotFound for anything else — the injectable-fake harness every
// Detect test in this file drives, never a real PATH mutation (repo rule
// m45p2b4bp7: never test or red-gate third-party CLI behavior).
func fakeEnv(present ...string) Environment {
	set := make(map[string]bool, len(present))
	for _, name := range present {
		set[name] = true
	}
	return Environment{
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

// runCall records one (path, args) invocation a scripted Run fake
// received — the shared prerequisite every Apply/Preview test in this
// package depends on to prove Apply() drives the exact resolved path and
// argv it claims to, never a real binary (rule m45p2b4bp7).
type runCall struct {
	Path string
	Args []string
}

// scriptedResult is one entry in a scriptedRun's response script.
type scriptedResult struct {
	Result RunResult
	Err    error
}

// scriptedRun returns an Environment.Run fake that replays results in
// order, one response per call, recording every (path, args) it receives
// into *calls. Calling it more times than len(results) is a test-authoring
// bug and panics immediately — a missing script entry must fail loudly,
// never silently replay the last scripted response and mask a wrong call
// count.
func scriptedRun(calls *[]runCall, results ...scriptedResult) func(context.Context, string, []string) (RunResult, error) {
	i := 0
	return func(_ context.Context, path string, args []string) (RunResult, error) {
		*calls = append(*calls, runCall{Path: path, Args: args})
		if i >= len(results) {
			panic(fmt.Sprintf("scriptedRun: called %d times, only %d result(s) scripted", i+1, len(results)))
		}
		r := results[i]
		i++
		return r.Result, r.Err
	}
}

// fakeEnvWithRun is fakeEnv(present...) with its Run seam replaced by run
// — the sibling constructor every Apply/Preview test needs, since fakeEnv
// alone carries no Run seam at all.
func fakeEnvWithRun(run func(context.Context, string, []string) (RunResult, error), present ...string) Environment {
	env := fakeEnv(present...)
	env.Run = run
	return env
}

// TestDetectClaudeCodePresent: a fake whose LookPath resolves "claude"
// reports Detect() present for claude-code.
func TestDetectClaudeCodePresent(t *testing.T) {
	env := fakeEnv("claude")
	if !ClaudeCode.Detect(env) {
		t.Error("ClaudeCode.Detect(env) = false, want true when LookPath resolves \"claude\"")
	}
}

// TestDetectClaudeCodeAbsent: a fake whose LookPath returns
// exec.ErrNotFound for "claude" reports NOT present.
func TestDetectClaudeCodeAbsent(t *testing.T) {
	env := fakeEnv() // nothing on PATH
	if ClaudeCode.Detect(env) {
		t.Error("ClaudeCode.Detect(env) = true, want false when LookPath fails")
	}
}

// TestDetectIgnoresConfigDirectory is the false-positive
// REQ-setup-detects-runtimes names: a fake whose LookPath fails for
// "claude" but whose HomeDir resolves to a directory containing a
// .claude/ config dir must still report NOT present (D-12) — Detect
// consults exec.LookPath exclusively and never stats a config directory
// at all.
func TestDetectIgnoresConfigDirectory(t *testing.T) {
	env := Environment{
		LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		Getenv:   func(string) string { return "" },
		HomeDir:  func() (string, error) { return "/home/has-config-dir", nil },
	}
	if ClaudeCode.Detect(env) {
		t.Error("ClaudeCode.Detect(env) = true, want false: a leftover config directory must never read as installed (D-12)")
	}
}

// TestDetectIsDeterministic: Detect() called twice against the same fake
// returns the same answer both times.
func TestDetectIsDeterministic(t *testing.T) {
	env := fakeEnv("claude")
	first := ClaudeCode.Detect(env)
	second := ClaudeCode.Detect(env)
	if first != second {
		t.Errorf("ClaudeCode.Detect(env) = %v then %v, want the same answer both times", first, second)
	}
}

// TestDetectEveryRegisteredRuntime is a table over Runtimes itself
// (Task 2), not over the three names — a fourth runtime added to the
// registry without wiring its own binary into this table fails
// immediately, since the fake only resolves the SPECIFIC binary each
// entry expects. An empty wantBinary entry means "no binary — this
// runtime's Detect() is deliberately unconditional" (generic, 03-04
// D-14): that runtime's assertion is a single unconditional-true check
// against an empty PATH rather than the present/absent pair every other
// entry gets, since it has no binary for fakeEnv to ever resolve or
// withhold.
func TestDetectEveryRegisteredRuntime(t *testing.T) {
	wantBinary := map[string]string{
		"claude-code": "claude",
		"codex":       "codex",
		"opencode":    "opencode",
		"generic":     "",
	}
	for _, rt := range Runtimes {
		rt := rt
		t.Run(rt.Name(), func(t *testing.T) {
			binary, ok := wantBinary[rt.Name()]
			if !ok {
				t.Fatalf("TestDetectEveryRegisteredRuntime: no expected binary name recorded for runtime %q — add one to wantBinary", rt.Name())
			}
			if binary == "" {
				if !rt.Detect(fakeEnv()) {
					t.Errorf("%s.Detect(fakeEnv()) = false, want true (unconditional Detect — no binary required)", rt.Name())
				}
				return
			}
			if !rt.Detect(fakeEnv(binary)) {
				t.Errorf("%s.Detect(fakeEnv(%q)) = false, want true", rt.Name(), binary)
			}
			if rt.Detect(fakeEnv()) {
				t.Errorf("%s.Detect(fakeEnv()) = true, want false (nothing on PATH)", rt.Name())
			}
		})
	}
}

// TestDetectOnlyCodexPresent: a fake resolving only "codex" yields
// present=true for codex and present=false for every other registered
// runtime whose Detect actually consults PATH. A runtime whose Detect is
// unconditional (generic, 03-04 D-14) is exempted structurally — by
// evidence (calling Detect against an EMPTY env and observing it still
// reports true), never by name — rather than skipped.
func TestDetectOnlyCodexPresent(t *testing.T) {
	env := fakeEnv("codex")
	for _, rt := range Runtimes {
		alwaysDetected := rt.Detect(fakeEnv())
		want := rt.Name() == "codex" || alwaysDetected
		if got := rt.Detect(env); got != want {
			t.Errorf("%s.Detect(env) = %v, want %v (only codex is on PATH)", rt.Name(), got, want)
		}
	}
}
