// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"errors"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestCodexSkillTarget asserts codex's authored SkillTarget against a
// fake environment with a deterministic home, across every supported
// auth mode, and asserts the value is byte-identical in every branch
// (04-03-PLAN.md Task 2's must_haves: "a runtime's skills destination
// does not vary by auth mode"). Format and IndexFile assert the
// 04-03-SUMMARY.md-recorded routing decision, "codex-native-plus-index":
// SkillFormatAgentsMD, with the index spliced into
// $HOME/.codex/AGENTS.md alongside the native skill files at
// $HOME/.agents/skills.
func TestCodexSkillTarget(t *testing.T) {
	const url = "https://engram.example.com/mcp"
	env := fakeEnv()
	wantDir := filepath.Join("/home/fake", ".agents", "skills")
	wantIndex := filepath.Join("/home/fake", ".codex", "AGENTS.md")
	wantTarget := SkillTarget{Format: SkillFormatAgentsMD, Dir: wantDir, IndexFile: wantIndex}

	modes := []string{"oauth", "none", "oauth-client", "bearer"}
	for _, mode := range modes {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			plan, err := Codex.Plan(env, Options{URL: url, Auth: mode, ClientID: "test-client"})
			if err != nil {
				t.Fatalf("Plan(%q): %v", mode, err)
			}
			if !reflect.DeepEqual(plan.Skills, wantTarget) {
				t.Errorf("Plan(%q).Skills = %+v, want %+v", mode, plan.Skills, wantTarget)
			}
		})
	}
}

// TestCodexSkillTargetIsNotCodexHome asserts the authored directory does
// not contain the segment ".codex" immediately followed by "skills" — the
// one destination (04-RESEARCH.md's Codex discrepancy write-up,
// $CODEX_HOME/skills) that was deliberately excluded from this plan's
// recorded routing decision.
func TestCodexSkillTargetIsNotCodexHome(t *testing.T) {
	plan, err := Codex.Plan(fakeEnv(), Options{URL: "https://engram.example.com/mcp", Auth: "oauth"})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	segments := strings.Split(filepath.ToSlash(plan.Skills.Dir), "/")
	for i := 0; i < len(segments)-1; i++ {
		if segments[i] == ".codex" && segments[i+1] == "skills" {
			t.Fatalf("Skills.Dir = %q contains the excluded .codex/skills segment pair", plan.Skills.Dir)
		}
	}
}

// TestCodexPlanFailsWhenHomeUnresolvable asserts a fake whose
// home-directory function errors makes Plan() return an error naming the
// runtime, rather than silently producing an empty-destination
// SkillTarget (04-03-PLAN.md Task 2 must_haves).
func TestCodexPlanFailsWhenHomeUnresolvable(t *testing.T) {
	env := Environment{
		LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		Getenv:   func(string) string { return "" },
		HomeDir:  func() (string, error) { return "", errors.New("boom") },
	}
	_, err := Codex.Plan(env, Options{URL: "https://engram.example.com/mcp", Auth: "oauth"})
	if err == nil {
		t.Fatal("Plan: want an error when the home directory is unresolvable, got nil")
	}
	if !strings.Contains(err.Error(), "codex") {
		t.Errorf("Plan err = %q, want it to name the runtime", err.Error())
	}
}

// TestEveryRuntimeAuthorsAnExplicitSkillFormat loops the package-level
// Runtimes registry (never a hand-written list) and asserts every
// registered runtime's Plan() returns a SkillTarget.Format that is one of
// the three explicit values — never the Go zero value ("") — the
// structural guard against a fifth runtime being added later with a
// zero-valued, silently-does-nothing target (04-03-PLAN.md Task 2).
//
// A runtime implementing optInOnlyRuntime (currently only the generic
// pseudo-runtime, generic.go) is skipped: it is not wired for skills in
// this plan's scope (deferred to 04-04-generic-and-summary, per this
// plan's own files_modified boundary), and — like Select()'s own
// default-set exclusion (runtime.go) — that exclusion is expressed
// through the existing structural predicate, never by naming "generic"
// in this file, honoring the "never special-case a runtime by name
// outside that runtime's own file" prohibition.
func TestEveryRuntimeAuthorsAnExplicitSkillFormat(t *testing.T) {
	env := fakeEnv()
	opts := Options{URL: "https://engram.example.com/mcp", Auth: "oauth"}

	for _, rt := range Runtimes {
		if oi, ok := rt.(optInOnlyRuntime); ok && oi.OptInOnly() {
			continue
		}
		plan, err := rt.Plan(env, opts)
		if err != nil {
			t.Fatalf("%s: Plan(oauth): %v", rt.Name(), err)
		}
		switch plan.Skills.Format {
		case SkillFormatNone, SkillFormatNative, SkillFormatAgentsMD:
			// one of the three explicit values — fine.
		default:
			t.Errorf("%s: Skills.Format = %q, want one of %q/%q/%q, never the zero value",
				rt.Name(), plan.Skills.Format, SkillFormatNone, SkillFormatNative, SkillFormatAgentsMD)
		}
	}
}

func TestCodexClientID(t *testing.T) {
	const url = "https://engram.example.com/mcp"
	for _, id := range []string{"test-client", "  client 'quoted'; $(echo nope) &  "} {
		t.Run(id, func(t *testing.T) {
			plan, err := Codex.Plan(fakeEnv(), Options{URL: url, Auth: "oauth-client", ClientID: id})
			if err != nil {
				t.Fatal(err)
			}
			want := []Action{{
				Args:        []string{"codex", "mcp", "add", "engram", "--url", url, "--oauth-client-id", id},
				Description: "register engram as an MCP server (pre-registered OAuth client)",
			}}
			if !reflect.DeepEqual(plan.Actions, want) {
				t.Errorf("Actions = %#v, want %#v", plan.Actions, want)
			}
			if !reflect.DeepEqual(plan.Probe, []string{"codex", "mcp", "get", "engram", "--json"}) {
				t.Errorf("Probe = %q", plan.Probe)
			}
			wantSkills := SkillTarget{Format: SkillFormatAgentsMD, Dir: filepath.Join("/home/fake", ".agents", "skills"), IndexFile: filepath.Join("/home/fake", ".codex", "AGENTS.md")}
			if plan.Skills != wantSkills {
				t.Errorf("Skills = %+v, want %+v", plan.Skills, wantSkills)
			}
		})
	}
}
