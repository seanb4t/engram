// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setupgen

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanb4t/engram/internal/setup"
	"github.com/seanb4t/engram/internal/surfaces"
)

func TestRenderRealPlans(t *testing.T) {
	body, err := Render(setup.ClaudeCode.Plan)
	if err != nil {
		t.Fatal(err)
	}
	cases := Cases()
	if len(cases) != 4 {
		t.Fatalf("got %d cases, want four", len(cases))
	}
	for i, mode := range []string{"oauth", "oauth-client", "bearer", "none"} {
		t.Run(mode, func(t *testing.T) {
			c := cases[i]
			if c.Options.Auth != mode {
				t.Fatalf("case %d = %q, want %q", i, c.Options.Auth, mode)
			}
			plan, err := setup.ClaudeCode.Plan(setup.Environment{HomeDir: func() (string, error) { return "/fake", nil }}, c.Options)
			if err != nil {
				t.Fatal(err)
			}
			for _, command := range []string{plan.Actions[1].Command(), (setup.Action{Args: c.DelegationArgs}).Command()} {
				if !strings.Contains(body, "| `"+mode+"` | `"+command+"` |") {
					t.Errorf("missing exact Plan/preview row for %q", command)
				}
			}
		})
	}
	if !strings.Contains(body, "'Authorization: Bearer ${ENGRAM_TOKEN}'") {
		t.Fatal("bearer environment reference lost its literal shell quoting")
	}
	cases[0].Options.URL = "changed"
	cases[0].DelegationArgs[0] = "changed"
	if fresh := Cases()[0]; fresh.Options.URL == "changed" || fresh.DelegationArgs[0] == "changed" {
		t.Fatal("Cases shares mutable data")
	}
}

func TestRenderSelectsActionAndQuotes(t *testing.T) {
	args := []string{"claude", "mcp", "add", "engram", "https://example.com/a b", "--client-id", "it's $literal"}
	body, err := Render(func(setup.Environment, setup.Options) (setup.Plan, error) {
		return setup.Plan{Actions: []setup.Action{
			{Args: args},
			{Args: []string{"unrelated", "mcp", "add"}},
			{Args: []string{"claude", "mcp", "remove", "engram"}},
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := (setup.Action{Args: args}).Command(); strings.Count(body, "`"+want+"`") != 4 {
		t.Fatalf("renderer did not preserve source quoting: %s", body)
	}
	if got := commandCell("echo '`a|b`'"); got != "`` echo '`a\\|b`' ``" {
		t.Fatalf("Markdown command cell = %q", got)
	}
}

func TestRenderRejectsInvalidPlans(t *testing.T) {
	for _, mode := range []string{"error", "missing", "ambiguous", "Getenv", "LookPath", "Run"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			body, err := Render(func(env setup.Environment, opts setup.Options) (setup.Plan, error) {
				calls++
				plan, err := setup.ClaudeCode.Plan(env, opts)
				if err != nil || calls == 1 { // Failure after a valid row must still return no partial body.
					return plan, err
				}
				switch mode {
				case "error":
					return setup.Plan{}, errors.New("broken plan")
				case "missing":
					plan.Actions = []setup.Action{{Args: []string{"claude"}}, {Args: []string{"other", "mcp", "add"}}}
				case "ambiguous":
					plan.Actions = append(plan.Actions, plan.Actions[1])
				case "Getenv":
					_ = env.Getenv("DO_NOT_READ")
				case "LookPath":
					_, _ = env.LookPath("claude")
				case "Run":
					_, _ = env.Run(context.Background(), "claude", nil)
				}
				return plan, nil
			})
			if err == nil || body != "" {
				t.Fatalf("Render returned body=%q err=%v", body, err)
			}
		})
	}
	if body, err := Render(nil); err == nil || body != "" {
		t.Fatalf("nil Plan returned %q, %v", body, err)
	}
}

func TestWriteRejectsInvalidAnchors(t *testing.T) {
	for _, content := range []string{
		"no anchors\n",
		"<!-- engram:rule:start setup-commands -->\nunterminated\n",
		"<!-- engram:rule:end setup-commands -->\n<!-- engram:rule:start setup-commands -->\n",
	} {
		path := filepath.Join(t.TempDir(), "command.md")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := Write(path); err == nil {
			t.Fatalf("accepted invalid anchors: %s", content)
		}
		after, err := os.ReadFile(path)
		if err != nil || string(after) != content {
			t.Fatalf("failed write changed fixture: %q, %v", after, err)
		}
	}
}

func TestWriteNoneTracer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "command.md")
	prefix := "---\ndescription: fixture\n---\n\nAuthored before.\n<!-- engram:rule:start setup-commands -->\n"
	suffix := "\n<!-- engram:rule:end setup-commands -->\nAuthored after.\n"
	if err := os.WriteFile(path, []byte(prefix+"stale"+suffix), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Write(path); err != nil {
		t.Fatal(err)
	}
	body, found, err := surfaces.ReadRegion(path, RegionID)
	if err != nil || !found {
		t.Fatalf("read region: found=%v err=%v", found, err)
	}
	plan, err := setup.ClaudeCode.Plan(setup.Environment{HomeDir: func() (string, error) { return "/synthetic/home", nil }}, setup.Options{URL: "https://engram.example.com/mcp", Auth: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "| `none` | `"+plan.Actions[1].Command()+"` |") {
		t.Fatalf("none registration missing from generated region: %s", body)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != prefix+body+suffix {
		t.Fatal("write changed authored bytes outside region")
	}
	if err := Write(path); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("repeat generation changed bytes")
	}
}

func TestCheckReadOnly(t *testing.T) {
	body, err := Render(setup.ClaudeCode.Plan)
	if err != nil {
		t.Fatal(err)
	}
	start := "<!-- engram:rule:start setup-commands -->\n"
	end := "\n<!-- engram:rule:end setup-commands -->\n"
	valid := start + body + end
	for _, tc := range []struct {
		name, content string
		wantError bool
	}{
		{"exact", valid, false},
		{"one-byte-drift", start + "X" + body[1:] + end, true},
		{"trailing-newline-drift", start + strings.TrimSuffix(body, "\n") + end, true},
		{"crlf-drift", strings.ReplaceAll(valid, "\n", "\r\n"), true},
		{"missing", "no anchors\n", true},
		{"unterminated", start + body, true},
		{"reversed", end + body + start, true},
		{"nested", start + valid + end, true},
		{"duplicate-stale", valid + start + "stale" + end, true},
		{"duplicate-inline", strings.TrimSpace(start) + valid, true},
		{"malformed", strings.Replace(valid, "setup-commands -->", "setup-commands ->", 1), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "command.md")
			before := "Authored prefix.\n" + tc.content + "Authored suffix.\n"
			if err := os.WriteFile(path, []byte(before), 0o600); err != nil { t.Fatal(err) }
			err := Check(path)
			if (err != nil) != tc.wantError { t.Fatalf("Check error = %v, wantError %v", err, tc.wantError) }
			if err != nil && (!strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "task surfaces:gen")) { t.Fatalf("error lacks target or remedy: %v", err) }
			after, err := os.ReadFile(path)
			if err != nil || string(after) != before { t.Fatalf("check changed file: %v", err) }
		})
	}
}
