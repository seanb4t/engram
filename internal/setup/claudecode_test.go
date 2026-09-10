// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// TestClaudeCodePlan is the table over all four auth modes proving
// claude-code's Plan() authors the checkpoint-approved two-action write
// sequence (03-02-PLAN.md Task 1's decision, correcting 03-CONTEXT.md
// D-08's falsified "mcp add overwrites" premise, 03-RESEARCH.md Pitfall
// 1): a tolerant clear-the-slot `claude mcp remove` first, then a fatal
// `claude mcp add` — for EVERY auth mode, not only bearer, because `claude
// mcp add` refuses on an existing name independent of auth mode.
func TestClaudeCodePlan(t *testing.T) {
	const url = "https://engram.example.com/mcp"
	wantRemove := []string{"claude", "mcp", "remove", "engram", "--scope", "user"}
	wantProbe := []string{"claude", "mcp", "get", "engram"}
	// Phase 4 widened Plan() to consult env.HomeDir() for the SkillTarget
	// it now authors (claudecode.go:114); a fake keeps this test isolated
	// from the real $HOME rather than reaching os.UserHomeDir() as a side
	// effect of asserting the (unrelated) registration Args (WR-01).
	env := Environment{
		LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		Getenv:   func(string) string { return "" },
		HomeDir:  func() (string, error) { return "/home/fake", nil },
	}

	tests := []struct {
		auth    string
		wantAdd []string
	}{
		{
			auth:    "oauth",
			wantAdd: []string{"claude", "mcp", "add", "--transport", "http", "engram", url, "--scope", "user"},
		},
		{
			auth:    "none",
			wantAdd: []string{"claude", "mcp", "add", "--transport", "http", "engram", url, "--scope", "user"},
		},
		{
			auth: "oauth-client",
			wantAdd: []string{"claude", "mcp", "add", "--transport", "http", "engram", url,
				"--scope", "user", "--client-id", "<id>", "--client-secret", "--callback-port", "8765"},
		},
		{
			auth: "bearer",
			wantAdd: []string{"claude", "mcp", "add", "--transport", "http", "engram", url,
				"--scope", "user", "--header", "Authorization: Bearer ${ENGRAM_TOKEN}"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.auth, func(t *testing.T) {
			plan, err := ClaudeCode.Plan(env, Options{URL: url, Auth: tc.auth})
			if err != nil {
				t.Fatalf("Plan(auth=%q): %v", tc.auth, err)
			}
			if len(plan.Actions) != 2 {
				t.Fatalf("Plan(auth=%q): len(Actions) = %d, want 2 (tolerant remove, fatal add)", tc.auth, len(plan.Actions))
			}
			if !plan.Actions[0].Tolerant {
				t.Errorf("Plan(auth=%q): Actions[0].Tolerant = false, want true (the clear-the-slot action)", tc.auth)
			}
			if !reflect.DeepEqual(plan.Actions[0].Args, wantRemove) {
				t.Errorf("Plan(auth=%q): Actions[0].Args = %v, want %v", tc.auth, plan.Actions[0].Args, wantRemove)
			}
			if plan.Actions[1].Tolerant {
				t.Errorf("Plan(auth=%q): Actions[1].Tolerant = true, want false (the registration action is fatal)", tc.auth)
			}
			if !reflect.DeepEqual(plan.Actions[1].Args, tc.wantAdd) {
				t.Errorf("Plan(auth=%q): Actions[1].Args = %v, want %v", tc.auth, plan.Actions[1].Args, tc.wantAdd)
			}
			if !reflect.DeepEqual(plan.Probe, wantProbe) {
				t.Errorf("Plan(auth=%q): Probe = %v, want %v (D-09: authored in the same Plan() call)", tc.auth, plan.Probe, wantProbe)
			}
		})
	}
}

// TestClaudeCodeBearerHeaderIsAnEnvVarReference proves claude-code's
// bearer mode (D-05, D-06) authors the --header value as a shell-style
// ${ENGRAM_TOKEN} variable REFERENCE, verified end-to-end against claude
// 2.1.265 (03-RESEARCH.md Pattern 2) to resolve at connect time while
// `claude mcp get` and the on-disk config both echo back only the literal
// unexpanded text — never a credential value and never Options.TokenFile's
// path.
func TestClaudeCodeBearerHeaderIsAnEnvVarReference(t *testing.T) {
	const sentinelCredential = "SUPER-SECRET-VALUE-MUST-NEVER-APPEAR-9f3e2a"
	const sentinelTokenFile = "/home/u/.engram/token-sentinel"

	// Fake home, for the same WR-01 isolation reason as TestClaudeCodePlan
	// above: Plan() now calls env.HomeDir() to author the SkillTarget this
	// test never asserts against.
	env := Environment{
		LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		Getenv:   func(string) string { return "" },
		HomeDir:  func() (string, error) { return "/home/fake", nil },
	}
	plan, err := ClaudeCode.Plan(env, Options{URL: "https://x", Auth: "bearer", TokenFile: sentinelTokenFile})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Actions) != 2 {
		t.Fatalf("len(Actions) = %d, want 2", len(plan.Actions))
	}
	headerArg := findHeaderArg(t, plan.Actions[1].Args)

	if !strings.Contains(headerArg, "${ENGRAM_TOKEN}") {
		t.Errorf("header arg = %q, want it to contain the ${ENGRAM_TOKEN} variable reference form", headerArg)
	}
	if strings.Contains(headerArg, sentinelCredential) {
		t.Errorf("header arg = %q, contains a sentinel credential value — must never carry a resolved secret", headerArg)
	}
	if strings.Contains(headerArg, sentinelTokenFile) {
		t.Errorf("header arg = %q, contains the sentinel token-file path — D-06 requires an env-var reference, never a path", headerArg)
	}
}

// findHeaderArg locates the argv element immediately following --header
// in args, failing the test if --header is not present.
func findHeaderArg(t *testing.T, args []string) string {
	t.Helper()
	for i, a := range args {
		if a == "--header" && i+1 < len(args) {
			return args[i+1]
		}
	}
	t.Fatalf("no --header flag found in Args %v", args)
	return ""
}
