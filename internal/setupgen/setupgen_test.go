// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setupgen

import (
	"context"
	"errors"
	"os"
	"os/exec"
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
		wantError     bool
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
			if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
				t.Fatal(err)
			}
			err := Check(path)
			if (err != nil) != tc.wantError {
				t.Fatalf("Check error = %v, wantError %v", err, tc.wantError)
			}
			if err != nil && (!strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "task surfaces:gen")) {
				t.Fatalf("error lacks target or remedy: %v", err)
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != before {
				t.Fatalf("check changed file: %v", err)
			}
		})
	}
}

// mutatedPlan changes one actual registration argv token in one auth mode.
// Copy both slices so the injected author cannot change the baseline Plan.
func mutatedPlan(env setup.Environment, opts setup.Options) (setup.Plan, error) {
	plan, err := setup.ClaudeCode.Plan(env, opts)
	if err != nil || opts.Auth != "oauth-client" {
		return plan, err
	}
	plan.Actions = append([]setup.Action(nil), plan.Actions...)
	for i, action := range plan.Actions {
		if len(action.Args) < 3 || action.Args[0] != "claude" || action.Args[1] != "mcp" || action.Args[2] != "add" {
			continue
		}
		plan.Actions[i].Args = append([]string(nil), action.Args...)
		for j, arg := range action.Args {
			if arg == opts.ClientID {
				plan.Actions[i].Args[j] += "-mutation"
				return plan, nil
			}
		}
	}
	return setup.Plan{}, errors.New("mutation fixture found no client-ID registration token")
}

func TestPlanMutationChangesRegion(t *testing.T) {
	baseline, err := Render(setup.ClaudeCode.Plan)
	if err != nil {
		t.Fatal(err)
	}
	mutant, err := Render(mutatedPlan)
	if err != nil {
		t.Fatal(err)
	}
	before, after := strings.Split(baseline, "\n"), strings.Split(mutant, "\n")
	if len(before) != len(after) {
		t.Fatal("mutation changed table shape")
	}
	changed := 0
	for i := range before {
		if before[i] == after[i] {
			continue
		}
		changed++
		if !strings.HasPrefix(after[i], "| `oauth-client` |") {
			t.Fatalf("unrelated row changed: %s", after[i])
		}
		var opts setup.Options
		for _, c := range Cases() {
			if c.Options.Auth == "oauth-client" {
				opts = c.Options
			}
		}
		plan, err := mutatedPlan(setup.Environment{HomeDir: func() (string, error) { return "/synthetic/home", nil }}, opts)
		if err != nil {
			t.Fatal(err)
		}
		for _, action := range plan.Actions {
			if len(action.Args) >= 3 && action.Args[2] == "add" {
				want := "| `oauth-client` | " + commandCell(action.Command()) + " |"
				if after[i] != want {
					t.Fatalf("changed row = %q, want full mutated command %q", after[i], want)
				}
			}
		}
	}
	if changed != 1 {
		t.Fatalf("changed %d rows, want exactly one", changed)
	}
	again, err := Render(setup.ClaudeCode.Plan)
	if err != nil || again != baseline {
		t.Fatalf("mutant contaminated real Plan: %v", err)
	}
}

func fixtureGit(t *testing.T, dir string, wantDiff bool, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=Setup Fixture", "-c", "user.email=setup-fixture@example.invalid", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=" + filepath.Join(dir, "empty-hooks")}, args...)...)
	cmd.Dir = dir
	// Exclude inherited repository/config overrides and user Git configuration.
	for _, variable := range os.Environ() {
		if !strings.HasPrefix(variable, "GIT_") {
			cmd.Env = append(cmd.Env, variable)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := cmd.CombinedOutput()
	if wantDiff {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			t.Fatalf("git %v must detect diff (exit 1): %v\n%s", args, err, out)
		}
		t.Logf("git %v detected drift (exit 1)", args)
	} else if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestDriftChecks(t *testing.T) {
	for _, sourceMutation := range []bool{true, false} {
		name := "source-mutation"
		if !sourceMutation {
			name = "checked-in-artifact-drift"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "command.md")
			prefix := "---\ndescription: scratch fixture\n---\nAuthored prefix.\n<!-- engram:rule:start setup-commands -->\n"
			suffix := "\n<!-- engram:rule:end setup-commands -->\nAuthored suffix.\n"
			put := func(content string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			read := func() string {
				t.Helper()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.HasPrefix(string(data), prefix) || !strings.HasSuffix(string(data), suffix) {
					t.Fatal("authored outside-region bytes changed")
				}
				return string(data)
			}
			put(prefix + "stale" + suffix)
			if err := Write(path); err != nil {
				t.Fatal(err)
			}
			baseline := read()
			if err := Check(path); err != nil {
				t.Fatal(err)
			}
			fixtureGit(t, dir, false, "init", "-q")
			fixtureGit(t, dir, false, "add", "command.md")
			fixtureGit(t, dir, false, "commit", "-qm", "baseline")
			planFn := PlanFunc(mutatedPlan)
			if !sourceMutation {
				// Commit stale artifact bytes so the CI lane must detect the
				// repair against a checked-in artifact, not an already-dirty tree.
				put(prefix + "independent artifact corruption\n" + suffix)
				fixtureGit(t, dir, false, "add", "command.md")
				fixtureGit(t, dir, false, "commit", "-qm", "corrupt artifact")
				planFn = setup.ClaudeCode.Plan
			}
			before := read()
			if err := check(path, planFn); err == nil {
				t.Fatal("read-only lane accepted drift")
			}
			if read() != before {
				t.Fatal("failed check repaired its evidence")
			}
			fixtureGit(t, dir, false, "diff", "--exit-code", "--", "command.md")
			if err := write(path, planFn); err != nil {
				t.Fatal(err)
			}
			generated := read()
			if generated == before {
				t.Fatal("writer ignored changed source or stale artifact")
			}
			fixtureGit(t, dir, true, "diff", "--exit-code", "--", "command.md")
			if err := check(path, planFn); err != nil {
				t.Fatal(err)
			}
			if err := write(path, planFn); err != nil {
				t.Fatal(err)
			}
			if read() != generated {
				t.Fatal("repeat generation changed bytes")
			}
			if err := Write(path); err != nil {
				t.Fatal(err)
			}
			if read() != baseline {
				t.Fatal("authoritative restoration differs from baseline")
			}
			if !sourceMutation {
				fixtureGit(t, dir, true, "diff", "--exit-code", "--", "command.md")
				fixtureGit(t, dir, false, "add", "command.md")
				fixtureGit(t, dir, false, "commit", "-qm", "restore authoritative artifact")
			}
			fixtureGit(t, dir, false, "diff", "--exit-code", "--", "command.md")
			if err := Check(path); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("invalid-anchors-never-repaired", func(t *testing.T) {
		for _, content := range []string{
			"missing\n",
			"<!-- engram:rule:start setup-commands -->\nunterminated\n",
			"<!-- engram:rule:end setup-commands -->\n<!-- engram:rule:start setup-commands -->\n",
			"<!-- engram:rule:start setup-commands -->\nstale\n<!-- engram:rule:end setup-commands -->\n<!-- engram:rule:start setup-commands -->\nsecond stale\n<!-- engram:rule:end setup-commands -->\n",
		} {
			path := filepath.Join(t.TempDir(), "command.md")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := Check(path); err == nil {
				t.Fatal("check accepted invalid anchors")
			}
			if err := Write(path); err == nil {
				t.Fatal("writer accepted invalid anchors")
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != content {
				t.Fatalf("invalid anchor evidence changed: %v", err)
			}
		}
	})
}
