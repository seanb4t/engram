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

// TestCodexDeclinesHeaders proves codex's Plan() declines ANY header
// before its auth-mode switch runs, in every mode, with an
// ErrHeaderUnsupported-wrapped error naming the header(s), the capability
// gap, and the remedy (D-09, D-10) — never ErrAuthModeUnsupported, so
// Phase 4 and docs can tell the two gaps apart. env.HomeDir is replaced
// by a func that fails the test if called at all, proving the guard runs
// BEFORE home resolution.
func TestCodexDeclinesHeaders(t *testing.T) {
	const url = "https://engram.example.com/mcp"
	env := fakeEnv()
	env.HomeDir = func() (string, error) {
		t.Errorf("HomeDir called; the header guard must run before home resolution")
		return "/home/fake", nil
	}

	const wantReason = "codex: custom header(s) x-gateway-api-key: codex mcp add exposes only --bearer-token-env-var (no custom header flag); drop --header or exclude codex via --runtime: setup: custom header is not supported by this runtime"

	for _, auth := range []string{"oauth", "oauth-client", "bearer", "none"} {
		auth := auth
		t.Run(auth, func(t *testing.T) {
			opts := Options{URL: url, Auth: auth, ClientID: "test-client",
				Headers: []HeaderSpec{{Name: "x-gateway-api-key", EnvVar: "GATEWAY_KEY"}}}
			plan, err := Codex.Plan(env, opts)
			if !errors.Is(err, ErrHeaderUnsupported) {
				t.Fatalf("Plan(auth=%q) err = %v, want errors.Is(err, ErrHeaderUnsupported)", auth, err)
			}
			if errors.Is(err, ErrAuthModeUnsupported) {
				t.Errorf("Plan(auth=%q) err = %v, must NOT satisfy errors.Is(err, ErrAuthModeUnsupported) (D-10: the two gaps stay distinguishable)", auth, err)
			}
			if !reflect.DeepEqual(plan, Plan{}) {
				t.Errorf("Plan(auth=%q) = %#v, want the zero Plan", auth, plan)
			}
			if err.Error() != wantReason {
				t.Errorf("Plan(auth=%q) err.Error() = %q, want %q", auth, err.Error(), wantReason)
			}
			if strings.Contains(err.Error(), "GATEWAY_KEY") {
				t.Errorf("Plan(auth=%q) err.Error() = %q, must never name the env var — only the header NAME", auth, err.Error())
			}
		})
	}

	t.Run("two-headers-sorted", func(t *testing.T) {
		opts := Options{URL: url, Auth: "bearer",
			Headers: []HeaderSpec{
				{Name: "x-gateway-api-key", EnvVar: "GATEWAY_KEY"},
				{Name: "CF-Access-Client-Id", EnvVar: "CF_ID"},
			}}
		_, err := Codex.Plan(env, opts)
		if !errors.Is(err, ErrHeaderUnsupported) {
			t.Fatalf("err = %v, want errors.Is(err, ErrHeaderUnsupported)", err)
		}
		const wantPrefix = "codex: custom header(s) CF-Access-Client-Id, x-gateway-api-key:"
		if !strings.HasPrefix(err.Error(), wantPrefix) {
			t.Errorf("err.Error() = %q, want it to start with %q (D-08 sorted, comma-space joined)", err.Error(), wantPrefix)
		}
	})

	// Zero-header control: the guard must not fire on empty, and codex's
	// Plan stays byte-identical to HEAD.
	t.Run("zero-header-control", func(t *testing.T) {
		plan, err := Codex.Plan(fakeEnv(), Options{URL: url, Auth: "bearer"})
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		want := []Action{{
			Args:        []string{"codex", "mcp", "add", "engram", "--url", url, "--bearer-token-env-var", "ENGRAM_TOKEN"},
			Description: "register engram as an MCP server (bearer token via ENGRAM_TOKEN)",
		}}
		if !reflect.DeepEqual(plan.Actions, want) {
			t.Errorf("Actions = %#v, want %#v", plan.Actions, want)
		}
	})
}

// TestObserveCodexRegistration drives codexRuntime.Observe directly on
// scripted probe-output strings — no subprocess, no Environment. Task 1
// authors the scaffold and its first subtest; Task 3 fills the full
// three-state table.
func TestObserveCodexRegistration(t *testing.T) {
	dr, ok := Codex.(DriftRuntime)
	if !ok {
		t.Fatal("Codex does not implement DriftRuntime")
	}

	t.Run("preserved-unrecognized-field", func(t *testing.T) {
		stdout := strings.Replace(codexGetEngramBearer,
			`"enabled_tools"`,
			`"oauth_client_id":"SENTINEL-LITERAL-9f3e2a-DO-NOT-LEAK","enabled_tools"`, 1)
		opts := Options{URL: "https://engram.example.com/mcp", Auth: "bearer"}

		obs, ok := dr.Observe(stdout, opts)
		if !ok {
			t.Fatal("Observe: ok = false, want true")
		}
		if len(obs.Unrecognized) != 1 || obs.Unrecognized[0] != "oauth_client_id" {
			t.Errorf("Unrecognized = %q, want [\"oauth_client_id\"]", obs.Unrecognized)
		}
		if obs.Auth != AuthBearer {
			t.Errorf("Auth = %q, want %q", obs.Auth, AuthBearer)
		}
		if obs.URL != "https://engram.example.com/mcp" {
			t.Errorf("URL = %q, want %q", obs.URL, "https://engram.example.com/mcp")
		}
		if len(obs.Headers) != 0 {
			t.Errorf("Headers = %+v, want none", obs.Headers)
		}
		if obs.BearerForm != "ENGRAM_TOKEN" {
			t.Errorf("BearerForm = %q, want %q", obs.BearerForm, "ENGRAM_TOKEN")
		}
		if obs.WholeEntryNote != codexWholeEntryNote {
			t.Errorf("WholeEntryNote = %q, want %q", obs.WholeEntryNote, codexWholeEntryNote)
		}
	})
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
