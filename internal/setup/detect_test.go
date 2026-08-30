// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
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
// entry expects.
func TestDetectEveryRegisteredRuntime(t *testing.T) {
	wantBinary := map[string]string{
		"claude-code": "claude",
		"codex":       "codex",
		"opencode":    "opencode",
	}
	for _, rt := range Runtimes {
		rt := rt
		t.Run(rt.Name(), func(t *testing.T) {
			binary, ok := wantBinary[rt.Name()]
			if !ok {
				t.Fatalf("TestDetectEveryRegisteredRuntime: no expected binary name recorded for runtime %q — add one to wantBinary", rt.Name())
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
// present=true for codex and present=false for the other two registered
// runtimes.
func TestDetectOnlyCodexPresent(t *testing.T) {
	env := fakeEnv("codex")
	for _, rt := range Runtimes {
		want := rt.Name() == "codex"
		if got := rt.Detect(env); got != want {
			t.Errorf("%s.Detect(env) = %v, want %v (only codex is on PATH)", rt.Name(), got, want)
		}
	}
}
