// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"errors"
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
