// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"fmt"
	"path/filepath"
)

// openCodeRuntime implements Runtime for opencode, authoring the
// live-verified `opencode mcp add [name] --url <URL>` invocation surface
// (.planning/research/SUMMARY.md § "Post-Synthesis Live Verification") —
// structurally identical to claudeCodeRuntime/codexRuntime, no
// special-casing.
type openCodeRuntime struct{}

// OpenCode is the Runtime registered for opencode.
var OpenCode Runtime = openCodeRuntime{}

func (openCodeRuntime) Name() string { return "opencode" }

// Detect consults env.LookPath("opencode") exclusively (D-12) — never a
// config directory.
func (openCodeRuntime) Detect(env Environment) bool {
	_, err := env.LookPath("opencode")
	return err == nil
}

// Plan authors the exact `opencode mcp add` invocation for opts.Auth,
// from the live-verified `opencode mcp add [name] --url <URL> --header
// KEY=VALUE` surface (03-RESEARCH.md § "Post-Synthesis Live
// Verification", re-confirmed live at opencode 1.18.20 this phase).
// opts.URL is used byte-for-byte, never appended to or stripped (D-02).
// "oauth-client" returns ErrAuthModeUnsupported permanently — no
// client-id flag exists on this surface, and Plan never guesses one; the
// caller renders this as an explicit outcome=failed row naming both the
// runtime and the mode (REQ-register-auth-modes' "states plainly which
// are unsupported for that runtime" clause, honoured at this planning
// layer). "none" reuses the same invocation as "oauth": opencode's
// `mcp add` has no separate no-auth form.
//
// The bearer form joins the header name to its value with an equals
// character ("Authorization=Bearer {env:ENGRAM_TOKEN}") — opencode's own
// `--header` flag requires this KEY=VALUE shape (its `--help`: "HTTP
// header for a remote MCP server (KEY=VALUE)"), NOT the colon-space
// HTTP-header-string form ("Authorization: Bearer ...") this file shipped
// with in Phase 2. That colon-space form was LIVE-REPRODUCED to fail
// outright against opencode 1.18.20 with an immediate nonzero exit
// ("Invalid HTTP header: ... Expected KEY=VALUE") — 03-RESEARCH.md
// Pitfall 2. This fix rides on the same edit that converts this file to
// Args (D-01): it closes a defect in code this phase already rewrites,
// not new scope. The header value names ENGRAM_TOKEN through opencode's
// own brace-delimited, `env:`-prefixed substitution token — live-verified
// this research pass to write and round-trip correctly — replacing
// bearerProvenance(opts.TokenFile) for this runtime (D-06); it carries a
// variable NAME, never a credential value or opts.TokenFile's path. This
// is one argv element with no shell involved, so no quoting concern
// arises at this layer — quoting only matters for Command()'s derived
// display.
//
// Every returned Plan carries Probe = {"opencode", "mcp", "list"} (D-09):
// there is no narrower verb. opencode's full command list is
// add/list/auth/logout/debug — no `get`, no `remove` — and `mcp list` has
// no `--json` flag. Two real limitations follow from this, both recorded
// here rather than rediscovered later:
//
//  1. Probe asymmetry: `mcp list` is the ONLY read verb available. It
//     renders a human-formatted table with box-drawing and status glyphs,
//     lists EVERY registered MCP server (not just engram's), and dials
//     the network for each of them on every invocation (live-observed at
//     roughly 1.2-1.7 seconds per call). The consequence for the shared
//     executor's D-08 byte-compare is that an unrelated server's
//     transient connection-status flip between read #1 and read #2 makes
//     the two captures differ. This degrades in the SAFE direction only —
//     D-08's invariant resolves ambiguity to OutcomeWrote and never to
//     OutcomeAlreadyCorrect — so `already-correct` is expected to be the
//     rare case rather than the common one for opencode specifically. The
//     correct response to this is never to parse or filter `mcp list`'s
//     output to isolate engram's own row; that would buy a dependency on
//     a third-party output format this design already rejected.
//  2. `{env:VAR}` substitution reliability: opencode's own issue tracker
//     (anomalyco/opencode#5299, filed against v1.0.137, open with an
//     associated PR #12390 at last check, not confirmed fixed as of the
//     probed 1.18.20) documents `{env:VAR_NAME}` substitution
//     inconsistently failing for specific MCP servers. engram cannot
//     detect this: it never observes the resolved header value, by
//     design (D-05). The operator-visible failure signature is a
//     registration engram reports as `wrote`/succeeded whose actual MCP
//     connection then fails auth — indistinguishable, from engram's
//     side, from a wrong token. Do not write a test asserting opencode's
//     substitution behavior — repo rule m45p2b4bp7 forbids red-gating
//     third-party behavior engram does not own.
//
// Every auth mode also authors the SAME SkillTarget (Phase 4, D-05,
// D-10): skills install to opencode's own documented global skills
// directory, opencodeConfigRoot(env) joined with "opencode" and "skills"
// — SkillFormatNative, since opencode has a native skill format and this
// phase's routing decision (04-03-SUMMARY.md) does not touch opencode. A
// HomeDir failure surfaced through opencodeConfigRoot is reported as a
// failed row naming this runtime, exactly like any other Plan() error.
func (openCodeRuntime) Plan(env Environment, opts Options) (Plan, error) {
	const probeVerb = "opencode"
	probe := []string{probeVerb, "mcp", "list"}

	configRoot, err := opencodeConfigRoot(env)
	if err != nil {
		return Plan{}, fmt.Errorf("opencode: resolve home directory: %w", err)
	}
	skillTarget := SkillTarget{
		Format: SkillFormatNative,
		Dir:    filepath.Join(configRoot, "opencode", "skills"),
	}

	switch opts.Auth {
	case "oauth", "none":
		return Plan{
			Runtime: "opencode",
			Actions: []Action{{
				Args:        []string{"opencode", "mcp", "add", "engram", "--url", opts.URL},
				Description: "register engram as an MCP server",
			}},
			Probe:  probe,
			Skills: skillTarget,
		}, nil
	case "bearer":
		return Plan{
			Runtime: "opencode",
			Actions: []Action{{
				Args: []string{"opencode", "mcp", "add", "engram", "--url", opts.URL,
					"--header", "Authorization=Bearer {env:ENGRAM_TOKEN}"},
				Description: "register engram as an MCP server (bearer token)",
			}},
			Probe:  probe,
			Skills: skillTarget,
		}, nil
	default:
		return Plan{}, fmt.Errorf("opencode: auth mode %q: %w", opts.Auth, ErrAuthModeUnsupported)
	}
}

// opencodeConfigRoot resolves the operator's own declared configuration
// root: XDG_CONFIG_HOME (04-03-PLAN.md's Task 2 behavior) when that
// variable holds a non-empty, absolute path — the exact value opencode
// itself honors for its own config root (04-RESEARCH.md, opencode.ai's
// documented global-skills path) — falling back to the home directory
// joined with ".config" otherwise. An empty or relative XDG_CONFIG_HOME
// is treated exactly like an unset one, never joined as-is: a relative
// destination would let engram write relative to whatever directory the
// `engram setup --apply` process happens to be running from, which D-10's
// user-scope-only decision exists to prevent (threat T-04-08). This is
// the ONLY environment variable this package's Plan implementations
// consult, and it is consulted only here, in opencode's own file.
func opencodeConfigRoot(env Environment) (string, error) {
	if v := env.Getenv("XDG_CONFIG_HOME"); v != "" && filepath.IsAbs(v) {
		return v, nil
	}
	home, err := env.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}
