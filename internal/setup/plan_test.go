// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// planHasArgElement reports whether any Action in plan carries an Args
// element equal to want, byte-for-byte. Searching every action's Args
// slice — rather than a substring of Actions[0]'s rendered display — is
// the load-bearing change Task 3 makes: index zero stops being the
// registration action the moment a runtime authors a tolerant
// clear-the-slot step first, and a substring assertion on a rendered
// (quoted) string cannot catch a malformed individual argument the way a
// direct Args-element comparison can (03-RESEARCH.md Pitfall 2's warning).
func planHasArgElement(plan Plan, want string) bool {
	for _, action := range plan.Actions {
		for _, arg := range action.Args {
			if arg == want {
				return true
			}
		}
	}
	return false
}

// planCarriesURL reports whether url appears byte-for-byte either in some
// Action's Args elements or in the plan's Config text — the structural
// check that covers both a native runtime's argv-authored Plan and the
// generic pseudo-runtime's zero-action, Config-only Plan (03-04). Written
// this way rather than skipping a zero-action plan by name: a runtime
// with neither an Args match nor a Config match authored the URL nowhere
// at all, which is exactly the failure this check exists to catch.
func planCarriesURL(plan Plan, url string) bool {
	if planHasArgElement(plan, url) {
		return true
	}
	return strings.Contains(plan.Config, url)
}

// TestPlanPassesURLVerbatimGatewayRoute proves that for every entry in
// Runtimes, Plan() with auth == "oauth" and a gateway-route URL (a path
// suffix beyond the bare host) authors an Args element equal to that URL
// byte-for-byte — never appended to or stripped (D-02).
func TestPlanPassesURLVerbatimGatewayRoute(t *testing.T) {
	const url = "https://gw.example.com/mcp/engram"
	for _, rt := range Runtimes {
		rt := rt
		t.Run(rt.Name(), func(t *testing.T) {
			plan, err := rt.Plan(OSEnvironment, Options{URL: url, Auth: "oauth"})
			if err != nil {
				t.Fatalf("%s.Plan: %v", rt.Name(), err)
			}
			if !planCarriesURL(plan, url) {
				t.Errorf("%s.Plan: neither an action's Args nor Config contains an element equal to %q byte-for-byte", rt.Name(), url)
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
			if !planCarriesURL(plan, url) {
				t.Errorf("%s.Plan: neither an action's Args nor Config contains an element equal to %q byte-for-byte", rt.Name(), url)
			}
		})
	}
}

// TestNoSecretInArgs proves that for every registered runtime and every
// auth mode, the ACTUAL CREDENTIAL VALUE never reaches Action.Args — the
// argv-side control for REQ-register-auth-modes' "a secret is never
// placed on a command line where the shell or process table would
// capture it." The sentinel here stands in for a RESOLVED secret's bytes,
// exported through the fake Environment.Getenv as ENGRAM_TOKEN would be
// resolved by a runtime at its own connect time — never by Plan() itself,
// which performs zero file reads (leafpurity_test.go) and never touches
// Getenv for the bearer credential. A --token-file PATH is a distinct
// concept: it is not a secret (TestPlanBearerNeverReadsTokenFile already
// pins that Plan() never opens it) and may legitimately appear in a
// claude-code/opencode bearer row's own provenance segment ("Bearer <from
// PATH>", Phase 2 D-16, narrowed by D-06 in a later wave) — this test
// does not assert against that sanctioned, already-pinned rendering.
//
// A mode returning ErrAuthModeUnsupported is a PASS for that pair, not a
// skip-and-forget: the error is asserted to wrap ErrAuthModeUnsupported
// so an accidental future removal of the guard is caught.
func TestNoSecretInArgs(t *testing.T) {
	const secretValue = "SUPER-SECRET-VALUE-MUST-NEVER-APPEAR-9f3e2a"
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

	for _, rt := range Runtimes {
		for _, auth := range []string{"oauth", "oauth-client", "bearer", "none"} {
			rt, auth := rt, auth
			t.Run(rt.Name()+":"+auth, func(t *testing.T) {
				plan, err := rt.Plan(env, Options{URL: "https://x", Auth: auth, TokenFile: "/home/u/.engram/token"})
				if err != nil {
					if errors.Is(err, ErrAuthModeUnsupported) {
						return
					}
					t.Fatalf("Plan: %v (want either success or errors.Is(err, ErrAuthModeUnsupported))", err)
				}
				for _, action := range plan.Actions {
					for _, arg := range action.Args {
						if strings.Contains(arg, secretValue) {
							t.Errorf("%s:%s: Args element %q contains the resolved credential value", rt.Name(), auth, arg)
						}
					}
				}
				// The generic pseudo-runtime (03-04) carries its whole
				// deliverable in Config rather than Args — the resolved
				// credential value must never reach there either.
				if strings.Contains(plan.Config, secretValue) {
					t.Errorf("%s:%s: Config %q contains the resolved credential value", rt.Name(), auth, plan.Config)
				}
			})
		}
	}
}

// TestOAuthAndNoneAuthorIdenticalArgs pins D-01's "oauth and none are
// deliberately the same invocation" intent: for every registered runtime,
// "oauth" and "none" must author deeply-equal Args across every action.
// No runtime's `mcp add` has a separate no-auth form; this test makes
// that a pinned fact rather than a coincidence a future edit could break
// silently.
func TestOAuthAndNoneAuthorIdenticalArgs(t *testing.T) {
	for _, rt := range Runtimes {
		rt := rt
		t.Run(rt.Name(), func(t *testing.T) {
			oauthPlan, err := rt.Plan(OSEnvironment, Options{URL: "https://x", Auth: "oauth"})
			if err != nil {
				t.Fatalf("oauth Plan: %v", err)
			}
			nonePlan, err := rt.Plan(OSEnvironment, Options{URL: "https://x", Auth: "none"})
			if err != nil {
				t.Fatalf("none Plan: %v", err)
			}
			if len(oauthPlan.Actions) != len(nonePlan.Actions) {
				t.Fatalf("action count differs: oauth=%d none=%d", len(oauthPlan.Actions), len(nonePlan.Actions))
			}
			for i := range oauthPlan.Actions {
				if !reflect.DeepEqual(oauthPlan.Actions[i].Args, nonePlan.Actions[i].Args) {
					t.Errorf("action %d: oauth Args = %v, none Args = %v, want identical", i, oauthPlan.Actions[i].Args, nonePlan.Actions[i].Args)
				}
			}
		})
	}
}

// TestSkillTargetDestinationsAreAbsolute is a registry-driven structural
// guard (WR-03), the same shape as codex_test.go's
// TestEveryRuntimeAuthorsAnExplicitSkillFormat: for every entry in
// Runtimes (never a hand-written list, so a fifth runtime is covered
// automatically), Plan()'s authored SkillTarget.Dir must be absolute for
// every non-SkillFormatNone target, and SkillTarget.IndexFile must be
// absolute for every SkillFormatAgentsMD target. Today every runtime
// happens to derive its destination from env.HomeDir() correctly, but
// nothing previously caught a relative Dir/IndexFile — an authoring bug in
// a future runtime, or a refactor that drops the env.HomeDir() call —
// before it silently wrote skill files (or spliced an AGENTS.md-shaped
// file) relative to engram's own current working directory (threat
// T-04-08, 04-CONTEXT.md).
func TestSkillTargetDestinationsAreAbsolute(t *testing.T) {
	env := fakeEnv()
	opts := Options{URL: "https://engram.example.com/mcp", Auth: "oauth"}

	for _, rt := range Runtimes {
		rt := rt
		t.Run(rt.Name(), func(t *testing.T) {
			plan, err := rt.Plan(env, opts)
			if err != nil {
				t.Fatalf("%s: Plan(oauth): %v", rt.Name(), err)
			}
			if plan.Skills.Format != SkillFormatNone && !filepath.IsAbs(plan.Skills.Dir) {
				t.Errorf("%s: Skills.Dir = %q, want an absolute path (Format=%q)", rt.Name(), plan.Skills.Dir, plan.Skills.Format)
			}
			if plan.Skills.Format == SkillFormatAgentsMD && !filepath.IsAbs(plan.Skills.IndexFile) {
				t.Errorf("%s: Skills.IndexFile = %q, want an absolute path (Format=%q)", rt.Name(), plan.Skills.IndexFile, plan.Skills.Format)
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
// entry in Runtimes EXCEPT one declaring itself optInOnlyRuntime (D-14),
// in registry order.
func TestSelectEmptyReturnsEveryRuntime(t *testing.T) {
	got, err := Select(nil)
	if err != nil {
		t.Fatalf("Select(nil): %v", err)
	}
	var want []Runtime
	for _, rt := range Runtimes {
		if oi, ok := rt.(optInOnlyRuntime); ok && oi.OptInOnly() {
			continue
		}
		want = append(want, rt)
	}
	if len(got) != len(want) {
		t.Fatalf("Select(nil) returned %d runtimes, want %d", len(got), len(want))
	}
	for i, rt := range want {
		if got[i].Name() != rt.Name() {
			t.Errorf("Select(nil)[%d].Name() = %q, want %q (registry order)", i, got[i].Name(), rt.Name())
		}
	}
}

// TestSelectDefaultSetExcludesOptInRuntimes proves Select(nil)'s result
// excludes every registry entry that declares itself optInOnlyRuntime —
// derived from the predicate itself, never from a hardcoded expected
// list, so registering a SECOND opt-in runtime later cannot make this
// test vacuous: it would still have to be excluded from Select(nil), and
// this test would still catch it if it were not.
func TestSelectDefaultSetExcludesOptInRuntimes(t *testing.T) {
	got, err := Select(nil)
	if err != nil {
		t.Fatalf("Select(nil): %v", err)
	}
	selected := make(map[string]bool, len(got))
	for _, rt := range got {
		selected[rt.Name()] = true
	}
	var sawOptIn bool
	for _, rt := range Runtimes {
		oi, ok := rt.(optInOnlyRuntime)
		if !ok || !oi.OptInOnly() {
			continue
		}
		sawOptIn = true
		if selected[rt.Name()] {
			t.Errorf("Select(nil) includes %q, which declares OptInOnly() == true", rt.Name())
		}
	}
	if !sawOptIn {
		t.Fatal("no registered runtime declares itself optInOnlyRuntime — this test has nothing to exercise; is generic still registered and does it still implement OptInOnly?")
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

// TestPlanAuthModes is the exhaustive 4x4 runtime-by-auth-mode table
// (Task 3, extended in 03-04 to generic's row): every cell either returns
// a Plan with a non-empty Action.Command OR a non-empty Config (the
// generic pseudo-runtime's entire deliverable, D-16 — it never authors an
// Action), or an error satisfying errors.Is(err, ErrAuthModeUnsupported).
// Two cells (opencode x oauth-client, generic x oauth-client) are the
// error case — the table is exhaustive so a future runtime cannot skip a
// mode silently.
func TestPlanAuthModes(t *testing.T) {
	const url = "https://engram.example.com/mcp"
	const tokenFile = "/home/u/.engram/token"

	unsupported := map[string]bool{
		"opencode:oauth-client": true,
		"generic:oauth-client":  true,
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
				if plan.Display() == "" && plan.Config == "" {
					t.Fatalf("%s: Plan returned no non-empty Action.Command and no Config", key)
				}
			})
		}
	}
	if unsupportedSeen != len(unsupported) {
		t.Errorf("exercised %d unsupported cell(s), want exactly %d (%v) — the table did not run one of them", unsupportedSeen, len(unsupported), unsupported)
	}
}

// TestPlanBearerRedactsCredentialByProvenance proves that every NATIVE
// registered runtime's bearer form (one that authors at least one
// Action) renders neither a credential value nor Options.TokenFile's
// path: codex, claude-code (D-05/D-06, 03-02), and opencode (D-05/D-06,
// 03-03) each name ENGRAM_TOKEN through their own runtime-native
// substitution mechanism instead — codex via its --bearer-token-env-var
// flag (no placeholder at all), claude-code via a ${ENGRAM_TOKEN}
// shell-style variable reference, opencode via its {env:...} substitution
// token. bearerProvenance's path-provenance placeholder form is no longer
// authored by any NATIVE entry in Runtimes.
//
// A zero-action runtime (generic, 03-04) is exempted from that same
// assertion structurally, by the len(plan.Actions) == 0 property rather
// than by name: it has no CLI of its own that could resolve a
// substitution token at connect time on an arbitrary third-party client,
// so its Plan() deliberately falls back to bearerProvenance's literal
// path-provenance placeholder — the ONE remaining production caller of
// that function (generic.go) — rather than a ${...} reference it cannot
// promise will ever expand. Its own narrower assertion below still proves
// the resolved CREDENTIAL VALUE never appears; the path legitimately does
// (TestGenericConfigCarriesNoSecret, generic_test.go, pins the credential
// half specifically for generic).
func TestPlanBearerRedactsCredentialByProvenance(t *testing.T) {
	const tokenFile = "/home/u/.engram/token"

	for _, rt := range Runtimes {
		rt := rt
		t.Run(rt.Name(), func(t *testing.T) {
			plan, err := rt.Plan(OSEnvironment, Options{URL: "https://x", Auth: "bearer", TokenFile: tokenFile})
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if len(plan.Actions) == 0 {
				if !strings.Contains(plan.Config, tokenFile) {
					t.Errorf("%s bearer config = %q, want it to contain the token-file path via bearerProvenance's placeholder (no CLI of its own to name a substitution token)", rt.Name(), plan.Config)
				}
				return
			}
			cmd := plan.Display()
			if strings.Contains(cmd, tokenFile) {
				t.Errorf("%s bearer command = %q, want it to NOT contain the token file path — it names ENGRAM_TOKEN, never a path", rt.Name(), cmd)
			}
			if !strings.Contains(cmd, "ENGRAM_TOKEN") {
				t.Errorf("%s bearer command = %q, want it to name ENGRAM_TOKEN", rt.Name(), cmd)
			}
		})
	}
}

// TestPlanBearerNeverReadsTokenFile is the negative assertion Task 3 of
// 03-01-PLAN.md requires: given a TokenFile whose contents would be a
// secret, Plan() never reads the file. Both claude-code (D-05/D-06,
// 03-02) and opencode (D-05/D-06, 03-03) name ENGRAM_TOKEN via their own
// substitution token and never emit the path at all, so both assertions
// are simply that Plan() still succeeds against a nonexistent path and
// never echoes it.
func TestPlanBearerNeverReadsTokenFile(t *testing.T) {
	const nonexistent = "/definitely/does/not/exist/token"

	t.Run("claude-code", func(t *testing.T) {
		plan, err := ClaudeCode.Plan(OSEnvironment, Options{URL: "https://x", Auth: "bearer", TokenFile: nonexistent})
		if err != nil {
			t.Fatalf("Plan: %v (a nonexistent token file must not cause Plan to fail — it never reads the file)", err)
		}
		if strings.Contains(plan.Display(), nonexistent) {
			t.Errorf("claude-code bearer command = %q, want it to NOT contain the token-file path even though the file does not exist", plan.Display())
		}
	})

	t.Run("opencode", func(t *testing.T) {
		plan, err := OpenCode.Plan(OSEnvironment, Options{URL: "https://x", Auth: "bearer", TokenFile: nonexistent})
		if err != nil {
			t.Fatalf("Plan: %v (a nonexistent token file must not cause Plan to fail — it never reads the file)", err)
		}
		if strings.Contains(plan.Display(), nonexistent) {
			t.Errorf("opencode bearer command = %q, want it to NOT contain the token-file path even though the file does not exist", plan.Display())
		}
	})
}

// TestPlanClaudeCodeBearerIgnoresTokenFile proves that D-05/D-06's
// env-var-reference conversion made Options.TokenFile irrelevant to
// claude-code's bearer output: Plan() with an empty TokenFile and Plan()
// with a populated TokenFile author byte-identical Display() output, both
// naming ENGRAM_TOKEN via the ${...} reference form and never a path —
// this replaces the pre-conversion
// TestPlanBearerEmptyTokenFileNamesEnvVar, which pinned the now-retired
// bearerProvenance-based "Bearer <from ENGRAM_TOKEN>" fallback form.
func TestPlanClaudeCodeBearerIgnoresTokenFile(t *testing.T) {
	withFile, err := ClaudeCode.Plan(OSEnvironment, Options{URL: "https://x", Auth: "bearer", TokenFile: "/home/u/.engram/token"})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	withoutFile, err := ClaudeCode.Plan(OSEnvironment, Options{URL: "https://x", Auth: "bearer", TokenFile: ""})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if withFile.Display() != withoutFile.Display() {
		t.Errorf("claude-code bearer Display() differs by TokenFile: with=%q without=%q, want identical (D-05/D-06: TokenFile is irrelevant once bearer is an env-var reference)", withFile.Display(), withoutFile.Display())
	}
	const want = "Authorization: Bearer ${ENGRAM_TOKEN}"
	if !strings.Contains(withoutFile.Display(), want) {
		t.Errorf("claude-code bearer command = %q, want it to contain %q", withoutFile.Display(), want)
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
	cmd := plan.Display()
	for _, want := range []string{"--client-id", "--client-secret", "--callback-port 8765"} {
		if !strings.Contains(cmd, want) {
			t.Errorf("claude-code oauth-client command = %q, want it to contain %q", cmd, want)
		}
	}
}
