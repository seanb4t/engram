// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"errors"
	"strings"
	"testing"
)

// TestPlanPassesURLVerbatimGatewayRoute proves that for every entry in
// Runtimes, Plan() with auth == "oauth" and a gateway-route URL (a path
// suffix beyond the bare host) returns an Action.Command containing that
// URL byte-for-byte — never appended to or stripped (D-02).
func TestPlanPassesURLVerbatimGatewayRoute(t *testing.T) {
	const url = "https://gw.example.com/mcp/engram"
	for _, rt := range Runtimes {
		rt := rt
		t.Run(rt.Name(), func(t *testing.T) {
			plan, err := rt.Plan(OSEnvironment, Options{URL: url, Auth: "oauth"})
			if err != nil {
				t.Fatalf("%s.Plan: %v", rt.Name(), err)
			}
			if len(plan.Actions) == 0 {
				t.Fatalf("%s.Plan: no actions", rt.Name())
			}
			if !strings.Contains(plan.Actions[0].Command(), url) {
				t.Errorf("%s.Plan.Actions[0].Command = %q, want it to contain %q byte-for-byte", rt.Name(), plan.Actions[0].Command(), url)
			}
		})
	}
}

// TestPlanPassesURLVerbatimRootMounted is
// TestPlanPassesURLVerbatimGatewayRoute's root-mounted-URL sibling
// (D-02's second documented deployment shape).
func TestPlanPassesURLVerbatimRootMounted(t *testing.T) {
	const url = "https://engram.example.com/"
	for _, rt := range Runtimes {
		rt := rt
		t.Run(rt.Name(), func(t *testing.T) {
			plan, err := rt.Plan(OSEnvironment, Options{URL: url, Auth: "oauth"})
			if err != nil {
				t.Fatalf("%s.Plan: %v", rt.Name(), err)
			}
			if len(plan.Actions) == 0 {
				t.Fatalf("%s.Plan: no actions", rt.Name())
			}
			if !strings.Contains(plan.Actions[0].Command(), url) {
				t.Errorf("%s.Plan.Actions[0].Command = %q, want it to contain %q byte-for-byte", rt.Name(), plan.Actions[0].Command(), url)
			}
		})
	}
}

// TestSelectByName proves Select([]string{"codex"}) returns exactly the
// codex runtime.
func TestSelectByName(t *testing.T) {
	got, err := Select([]string{"codex"})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(got) != 1 || got[0].Name() != "codex" {
		t.Errorf("Select([\"codex\"]) = %v, want exactly the codex runtime", got)
	}
}

// TestSelectUnknownName proves Select([]string{"nope"}) returns a non-nil
// error whose message contains "nope" and all three valid runtime names.
func TestSelectUnknownName(t *testing.T) {
	_, err := Select([]string{"nope"})
	if err == nil {
		t.Fatal("Select([\"nope\"]) = nil error, want an error naming the unknown value")
	}
	for _, want := range []string{"nope", "claude-code", "codex", "opencode"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Select([\"nope\"]) error = %q, want it to contain %q", err, want)
		}
	}
}

// TestSelectEmptyReturnsEveryRuntime proves Select(nil) returns every
// entry in Runtimes, in registry order (D-10).
func TestSelectEmptyReturnsEveryRuntime(t *testing.T) {
	got, err := Select(nil)
	if err != nil {
		t.Fatalf("Select(nil): %v", err)
	}
	if len(got) != len(Runtimes) {
		t.Fatalf("Select(nil) returned %d runtimes, want %d", len(got), len(Runtimes))
	}
	for i, rt := range Runtimes {
		if got[i].Name() != rt.Name() {
			t.Errorf("Select(nil)[%d].Name() = %q, want %q (registry order)", i, got[i].Name(), rt.Name())
		}
	}
}

// TestSelectUnknownNameIsErrAuthModeUnsupportedDistinct is a sanity check
// that Select's unknown-name error is NOT ErrAuthModeUnsupported — a
// usage-shaped error, distinct from the sentinel this package exports.
func TestSelectUnknownNameIsErrAuthModeUnsupportedDistinct(t *testing.T) {
	_, err := Select([]string{"nope"})
	if err == nil {
		t.Fatal("Select([\"nope\"]) = nil, want an error")
	}
	if errors.Is(err, ErrAuthModeUnsupported) {
		t.Error("Select's unknown-name error satisfies errors.Is(err, ErrAuthModeUnsupported), want distinct error classes")
	}
}

// TestSelectDedupesRepeatedNames proves Select gives a repeated --runtime
// name first-occurrence deduplication (WR-02): a repeat is silently
// skipped (never an error), and the caller's stated order is preserved
// rather than re-sorted into registry order.
func TestSelectDedupesRepeatedNames(t *testing.T) {
	t.Run("single-name-repeated", func(t *testing.T) {
		got, err := Select([]string{"claude-code", "claude-code"})
		if err != nil {
			t.Fatalf("Select: %v", err)
		}
		if len(got) != 1 || got[0].Name() != "claude-code" {
			t.Errorf("Select([\"claude-code\", \"claude-code\"]) = %v, want exactly one claude-code runtime", got)
		}
	})

	t.Run("mixed-first-occurrence-order", func(t *testing.T) {
		got, err := Select([]string{"codex", "claude-code", "codex"})
		if err != nil {
			t.Fatalf("Select: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("Select([\"codex\", \"claude-code\", \"codex\"]) returned %d runtimes, want 2", len(got))
		}
		if got[0].Name() != "codex" || got[1].Name() != "claude-code" {
			t.Errorf("Select([\"codex\", \"claude-code\", \"codex\"]) = [%s %s], want [codex claude-code] (first-occurrence order)",
				got[0].Name(), got[1].Name())
		}
	})

	t.Run("unknown-name-repeated-still-errors", func(t *testing.T) {
		_, err := Select([]string{"nope", "nope"})
		if err == nil {
			t.Fatal(`Select(["nope", "nope"]) = nil error, want an error naming "nope"`)
		}
		if !strings.Contains(err.Error(), "nope") {
			t.Errorf("Select([\"nope\", \"nope\"]) error = %q, want it to contain %q", err, "nope")
		}
	})
}

// TestPlanAuthModes is the exhaustive 3x4 runtime-by-auth-mode table
// (Task 3): every cell either returns a Plan with a non-empty
// Action.Command, or an error satisfying errors.Is(err,
// ErrAuthModeUnsupported). Exactly one cell (opencode x oauth-client) is
// the error case — the table is exhaustive so a future runtime cannot
// skip a mode silently.
func TestPlanAuthModes(t *testing.T) {
	const url = "https://engram.example.com/mcp"
	const tokenFile = "/home/u/.engram/token"

	unsupported := map[string]bool{
		"opencode:oauth-client": true,
	}

	var unsupportedSeen int
	for _, rt := range Runtimes {
		for _, auth := range []string{"oauth", "oauth-client", "bearer", "none"} {
			rt, auth := rt, auth
			key := rt.Name() + ":" + auth
			t.Run(key, func(t *testing.T) {
				plan, err := rt.Plan(OSEnvironment, Options{URL: url, Auth: auth, TokenFile: tokenFile})
				if unsupported[key] {
					unsupportedSeen++
					if !errors.Is(err, ErrAuthModeUnsupported) {
						t.Fatalf("%s: Plan err = %v, want errors.Is(err, ErrAuthModeUnsupported)", key, err)
					}
					return
				}
				if err != nil {
					t.Fatalf("%s: Plan: %v", key, err)
				}
				if len(plan.Actions) == 0 || plan.Actions[0].Command() == "" {
					t.Fatalf("%s: Plan returned no non-empty Action.Command", key)
				}
			})
		}
	}
	if unsupportedSeen != len(unsupported) {
		t.Errorf("exercised %d unsupported cell(s), want exactly %d (%v) — the table did not run one of them", unsupportedSeen, len(unsupported), unsupported)
	}
}

// TestPlanBearerRedactsCredentialByProvenance proves that for every
// runtime whose bearer form embeds a credential placeholder (claude-code
// and opencode — codex names ENGRAM_TOKEN via its own
// --bearer-token-env-var flag and carries no credential placeholder at
// all), the authored command contains the literal provenance form
// "Bearer <from /home/u/.engram/token>" and no other credential material.
func TestPlanBearerRedactsCredentialByProvenance(t *testing.T) {
	const tokenFile = "/home/u/.engram/token"
	want := "Bearer <from " + tokenFile + ">"

	for _, rt := range []Runtime{ClaudeCode, OpenCode} {
		rt := rt
		t.Run(rt.Name(), func(t *testing.T) {
			plan, err := rt.Plan(OSEnvironment, Options{URL: "https://x", Auth: "bearer", TokenFile: tokenFile})
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			cmd := plan.Actions[0].Command()
			if !strings.Contains(cmd, want) {
				t.Errorf("%s bearer command = %q, want it to contain %q", rt.Name(), cmd, want)
			}
		})
	}

	// codex's own bearer form: no credential placeholder to redact at all,
	// just a fixed env-var name.
	plan, err := Codex.Plan(OSEnvironment, Options{URL: "https://x", Auth: "bearer", TokenFile: tokenFile})
	if err != nil {
		t.Fatalf("codex Plan: %v", err)
	}
	cmd := plan.Actions[0].Command()
	if strings.Contains(cmd, tokenFile) {
		t.Errorf("codex bearer command = %q, want it to NOT contain the token file path — codex names ENGRAM_TOKEN, never a path", cmd)
	}
	if !strings.Contains(cmd, "ENGRAM_TOKEN") {
		t.Errorf("codex bearer command = %q, want it to name ENGRAM_TOKEN", cmd)
	}
}

// TestPlanBearerNeverReadsTokenFile is the negative assertion Task 3
// requires: given a TokenFile whose contents would be a secret, Plan()
// never reads the file — proven by pointing TokenFile at a path that does
// not exist and confirming Plan() still succeeds and still emits the
// provenance string.
func TestPlanBearerNeverReadsTokenFile(t *testing.T) {
	const nonexistent = "/definitely/does/not/exist/token"
	want := "Bearer <from " + nonexistent + ">"

	for _, rt := range []Runtime{ClaudeCode, OpenCode} {
		rt := rt
		t.Run(rt.Name(), func(t *testing.T) {
			plan, err := rt.Plan(OSEnvironment, Options{URL: "https://x", Auth: "bearer", TokenFile: nonexistent})
			if err != nil {
				t.Fatalf("Plan: %v (a nonexistent token file must not cause Plan to fail — it never reads the file)", err)
			}
			if !strings.Contains(plan.Actions[0].Command(), want) {
				t.Errorf("%s bearer command = %q, want it to contain %q even though the file does not exist", rt.Name(), plan.Actions[0].Command(), want)
			}
		})
	}
}

// TestPlanBearerEmptyTokenFileNamesEnvVar proves that --auth bearer with
// an empty --token-file emits "Bearer <from ENGRAM_TOKEN>" — the env-var
// fallback resolveToken already implements binary-wide — and never a
// literal variable value.
func TestPlanBearerEmptyTokenFileNamesEnvVar(t *testing.T) {
	plan, err := ClaudeCode.Plan(OSEnvironment, Options{URL: "https://x", Auth: "bearer", TokenFile: ""})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	want := "Bearer <from ENGRAM_TOKEN>"
	if !strings.Contains(plan.Actions[0].Command(), want) {
		t.Errorf("claude-code bearer command (empty token-file) = %q, want it to contain %q", plan.Actions[0].Command(), want)
	}
}

// TestPlanClaudeCodeOAuthClientForm proves auth == "oauth-client" on
// claude-code emits the --client-id / --client-secret / --callback-port
// 8765 form from the shipped table.
func TestPlanClaudeCodeOAuthClientForm(t *testing.T) {
	plan, err := ClaudeCode.Plan(OSEnvironment, Options{URL: "https://x", Auth: "oauth-client"})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	cmd := plan.Actions[0].Command()
	for _, want := range []string{"--client-id", "--client-secret", "--callback-port 8765"} {
		if !strings.Contains(cmd, want) {
			t.Errorf("claude-code oauth-client command = %q, want it to contain %q", cmd, want)
		}
	}
}
