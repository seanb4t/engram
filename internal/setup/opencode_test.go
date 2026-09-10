// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestOpenCodePlan is the exhaustive table over opencode's supported auth
// modes (oauth, none, bearer) plus the one unsupported mode
// (oauth-client): each supported cell asserts the EXACT Args slice and
// the EXACT Probe slice; the unsupported cell asserts
// errors.Is(err, ErrAuthModeUnsupported).
func TestOpenCodePlan(t *testing.T) {
	const url = "https://engram.example.com/mcp"
	wantProbe := []string{"opencode", "mcp", "list"}

	cases := []struct {
		auth string
		args []string
	}{
		{
			auth: "oauth",
			args: []string{"opencode", "mcp", "add", "engram", "--url", url},
		},
		{
			auth: "none",
			args: []string{"opencode", "mcp", "add", "engram", "--url", url},
		},
		{
			auth: "bearer",
			args: []string{"opencode", "mcp", "add", "engram", "--url", url,
				"--header", "Authorization=Bearer {env:ENGRAM_TOKEN}"},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.auth, func(t *testing.T) {
			plan, err := OpenCode.Plan(OSEnvironment, Options{URL: url, Auth: c.auth})
			if err != nil {
				t.Fatalf("Plan(%q): %v", c.auth, err)
			}
			if len(plan.Actions) != 1 {
				t.Fatalf("Plan(%q): len(Actions) = %d, want 1", c.auth, len(plan.Actions))
			}
			if plan.Actions[0].Tolerant {
				t.Errorf("Plan(%q): Actions[0].Tolerant = true, want false", c.auth)
			}
			if !reflect.DeepEqual(plan.Actions[0].Args, c.args) {
				t.Errorf("Plan(%q): Args = %v, want %v", c.auth, plan.Actions[0].Args, c.args)
			}
			if !reflect.DeepEqual(plan.Probe, wantProbe) {
				t.Errorf("Plan(%q): Probe = %v, want %v", c.auth, plan.Probe, wantProbe)
			}
		})
	}

	t.Run("oauth-client", func(t *testing.T) {
		_, err := OpenCode.Plan(OSEnvironment, Options{URL: url, Auth: "oauth-client"})
		if !errors.Is(err, ErrAuthModeUnsupported) {
			t.Fatalf("Plan(%q) err = %v, want errors.Is(err, ErrAuthModeUnsupported)", "oauth-client", err)
		}
		if !strings.Contains(err.Error(), "opencode") || !strings.Contains(err.Error(), "oauth-client") {
			t.Errorf("Plan(%q) err = %q, want it to name both the runtime and the mode", "oauth-client", err)
		}
	})
}

// TestOpenCodeSkillTarget covers the four XDG_CONFIG_HOME shapes
// 04-03-PLAN.md Task 2 names: an absolute value (used verbatim), an
// unset variable, an empty value, and a relative value (the latter two
// both fall back to the home-based default) — asserting the resulting
// Skills.Dir for each, and asserting it is absolute in all four
// (threat T-04-08: a destination must never resolve relative to the
// process's own working directory).
func TestOpenCodeSkillTarget(t *testing.T) {
	const url = "https://engram.example.com/mcp"
	const home = "/home/fake"

	cases := []struct {
		name    string
		set     bool
		value   string
		wantDir string
	}{
		{
			name:    "absolute",
			set:     true,
			value:   "/xdg/config",
			wantDir: filepath.Join("/xdg/config", "opencode", "skills"),
		},
		{
			name:    "unset",
			set:     false,
			wantDir: filepath.Join(home, ".config", "opencode", "skills"),
		},
		{
			name:    "empty",
			set:     true,
			value:   "",
			wantDir: filepath.Join(home, ".config", "opencode", "skills"),
		},
		{
			name:    "relative",
			set:     true,
			value:   "relative/config",
			wantDir: filepath.Join(home, ".config", "opencode", "skills"),
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			env := Environment{
				LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
				Getenv: func(key string) string {
					if key == "XDG_CONFIG_HOME" && c.set {
						return c.value
					}
					return ""
				},
				HomeDir: func() (string, error) { return home, nil },
			}
			plan, err := OpenCode.Plan(env, Options{URL: url, Auth: "oauth"})
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if plan.Skills.Dir != c.wantDir {
				t.Errorf("Skills.Dir = %q, want %q", plan.Skills.Dir, c.wantDir)
			}
			if !filepath.IsAbs(plan.Skills.Dir) {
				t.Errorf("Skills.Dir = %q, want an absolute path", plan.Skills.Dir)
			}
			if plan.Skills.Format != SkillFormatNative {
				t.Errorf("Skills.Format = %q, want %q", plan.Skills.Format, SkillFormatNative)
			}
		})
	}
}

// TestOpenCodePlanFailsWhenHomeUnresolvable asserts a fake whose
// home-directory function errors (with no XDG_CONFIG_HOME override) makes
// Plan() return an error naming the runtime.
func TestOpenCodePlanFailsWhenHomeUnresolvable(t *testing.T) {
	env := Environment{
		LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		Getenv:   func(string) string { return "" },
		HomeDir:  func() (string, error) { return "", errors.New("boom") },
	}
	_, err := OpenCode.Plan(env, Options{URL: "https://engram.example.com/mcp", Auth: "oauth"})
	if err == nil {
		t.Fatal("Plan: want an error when the home directory is unresolvable, got nil")
	}
	if !strings.Contains(err.Error(), "opencode") {
		t.Errorf("Plan err = %q, want it to name the runtime", err.Error())
	}
}

// TestOpenCodeBearerHeaderSyntax is the explicit regression test for the
// confirmed live bug (03-RESEARCH.md Pitfall 2): opencode's own --help
// requires the header argv element in KEY=VALUE form, not the
// HTTP-header-string "Key: Value" form the shipped code authored. Both
// halves of the assertion matter — the negative half is what makes this a
// regression test rather than a restatement of the positive case, per
// Pitfall 2's own warning that a test only checking the rendered command
// contains the URL would pass on the broken code.
func TestOpenCodeBearerHeaderSyntax(t *testing.T) {
	plan, err := OpenCode.Plan(OSEnvironment, Options{URL: "https://x", Auth: "bearer"})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	args := plan.Actions[0].Args

	var headerElement string
	found := false
	for i, a := range args {
		if a == "--header" && i+1 < len(args) {
			headerElement = args[i+1]
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Args = %v, want a --header element followed by the header pair", args)
	}

	if !strings.Contains(headerElement, "Authorization=Bearer ") {
		t.Errorf("header element = %q, want it to contain %q", headerElement, "Authorization=Bearer ")
	}
	if strings.Contains(headerElement, "Authorization: Bearer ") {
		t.Errorf("header element = %q, want it to NOT contain the colon-space HTTP-header-string form %q", headerElement, "Authorization: Bearer ")
	}
}

// TestOpenCodeBearerHeaderCarriesNoSecret proves the bearer header element
// names ENGRAM_TOKEN through opencode's own {env:...} substitution token
// and never carries a resolved credential value or Options.TokenFile's
// path (D-05, D-06).
func TestOpenCodeBearerHeaderCarriesNoSecret(t *testing.T) {
	const secretValue = "SUPER-SECRET-VALUE-MUST-NEVER-APPEAR-9f3e2a"
	const tokenFile = "/home/u/.engram/token"

	env := Environment{
		LookPath: func(string) (string, error) { return "/usr/local/bin/x", nil },
		Getenv: func(key string) string {
			if key == "ENGRAM_TOKEN" {
				return secretValue
			}
			return ""
		},
		HomeDir: func() (string, error) { return "/home/fake", nil },
	}

	plan, err := OpenCode.Plan(env, Options{URL: "https://x", Auth: "bearer", TokenFile: tokenFile})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	var headerElement string
	for i, a := range plan.Actions[0].Args {
		if a == "--header" && i+1 < len(plan.Actions[0].Args) {
			headerElement = plan.Actions[0].Args[i+1]
			break
		}
	}
	if headerElement == "" {
		t.Fatalf("Args = %v, want a --header element", plan.Actions[0].Args)
	}
	if strings.Contains(headerElement, secretValue) {
		t.Errorf("header element %q contains the resolved credential value", headerElement)
	}
	if strings.Contains(headerElement, tokenFile) {
		t.Errorf("header element %q contains Options.TokenFile's path", headerElement)
	}
	if !strings.Contains(headerElement, "ENGRAM_TOKEN") {
		t.Errorf("header element %q does not name ENGRAM_TOKEN", headerElement)
	}
}

// TestApplyOpenCodeConvergence pins opencode's D-08 convergence behavior
// under its uniquely polluted probe (03-RESEARCH.md Pitfall 3): `opencode
// mcp list` has no --json flag, lists EVERY registered server, and dials
// the network for each of them on every call, so a read1-vs-read2
// byte-compare can be polluted by an UNRELATED server's transient
// connection-status flip, not just a genuine change to engram's own
// registration. All three subtests drive the shared executor through the
// scripted Run fake — none invokes a real opencode binary (rule
// m45p2b4bp7).
func TestApplyOpenCodeConvergence(t *testing.T) {
	opts := Options{URL: "https://engram.example.com/mcp", Auth: "oauth"}

	t.Run("identical-probe-captures-already-correct", func(t *testing.T) {
		const listOutput = "┌─────────┬─────────┐\n│ engram  │ ✓ connected │\n└─────────┴─────────┘\n"
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: listOutput}}, // probe #1
			scriptedResult{Result: RunResult{ExitCode: 0}},        // write: opencode mcp add
			scriptedResult{Result: RunResult{Stdout: listOutput}}, // probe #2: byte-identical
		), "opencode")

		res := Apply(context.Background(), env, OpenCode, opts)
		if res.Outcome != OutcomeAlreadyCorrect {
			t.Fatalf("Outcome = %q, want %q when both mcp list captures are byte-identical", res.Outcome, OutcomeAlreadyCorrect)
		}
	})

	// unrelated-server-status-flip-still-wrote is D-08's own invariant
	// applied to opencode's worst case: opencode's mcp list output is
	// polluted by every OTHER registered server's live connection status,
	// not just engram's row. A read1-vs-read2 capture that differs only in
	// a region representing an unrelated server's status glyph flipping
	// (e.g. a second server going from connected to a transient failure
	// between the two reads) must still classify as OutcomeWrote, never
	// OutcomeAlreadyCorrect — over-reporting change is the SAFE direction
	// D-08 mandates, not a defect to "fix" by parsing the table to isolate
	// engram's own row (03-RESEARCH.md Pitfall 3's own warning).
	t.Run("unrelated-server-status-flip-still-wrote", func(t *testing.T) {
		probe1 := "┌─────────┬──────────────┐\n│ engram  │ ✓ connected  │\n│ other   │ ✓ connected  │\n└─────────┴──────────────┘\n"
		probe2 := "┌─────────┬──────────────┐\n│ engram  │ ✓ connected  │\n│ other   │ ✗ failed     │\n└─────────┴──────────────┘\n"
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Result: RunResult{Stdout: probe1}}, // probe #1
			scriptedResult{Result: RunResult{ExitCode: 0}},    // write: opencode mcp add
			scriptedResult{Result: RunResult{Stdout: probe2}}, // probe #2: only "other"'s row differs
		), "opencode")

		res := Apply(context.Background(), env, OpenCode, opts)
		if res.Outcome != OutcomeWrote {
			t.Fatalf("Outcome = %q, want %q — an unrelated server's status flip must degrade to wrote, never already-correct (D-08)", res.Outcome, OutcomeWrote)
		}
	})

	t.Run("probe-seam-error-not-already-correct", func(t *testing.T) {
		var calls []runCall
		env := fakeEnvWithRun(scriptedRun(&calls,
			scriptedResult{Err: errors.New("exec: start failure")}, // probe #1 seam error
		), "opencode")

		res := Apply(context.Background(), env, OpenCode, opts)
		if res.Outcome == OutcomeAlreadyCorrect {
			t.Fatalf("Outcome = %q, want anything but %q when the probe itself cannot run", res.Outcome, OutcomeAlreadyCorrect)
		}
	})
}
