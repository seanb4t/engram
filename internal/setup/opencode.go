// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import "fmt"

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
// KEY=VALUE` surface (.planning/research/SUMMARY.md § "Post-Synthesis
// Live Verification"). opts.URL is used byte-for-byte, never appended to
// or stripped (D-02). "oauth-client" returns ErrAuthModeUnsupported
// permanently — no client-id flag exists on this surface, and Plan never
// guesses one; the caller renders this as an explicit outcome=failed row
// naming both the runtime and the mode (REQ-register-auth-modes' "states
// plainly which are unsupported for that runtime" clause, honoured at
// this planning layer). "none" reuses the same invocation as "oauth":
// opencode's `mcp add` has no separate no-auth form.
//
// Mechanically converted to Args (D-01) as part of Phase 3 Task 1's
// Action.Command field deletion — every argv element here is
// content-identical to the string Phase 2 authored, just split into a
// slice. The header value's colon-form ("Authorization: Bearer ...") is
// live-confirmed WRONG for opencode (03-RESEARCH.md Pitfall 2 — opencode
// requires KEY=VALUE) — fixing that, and adding a Probe (D-09), are
// Wave 3's work, out of scope for this task's mechanical conversion. A
// Plan.Probe-less runtime degrades safely under the shared executor
// (apply.go): it can report OutcomeWrote but never OutcomeAlreadyCorrect
// (D-08's own ambiguity-resolves-to-wrote invariant).
func (openCodeRuntime) Plan(_ Environment, opts Options) (Plan, error) {
	switch opts.Auth {
	case "oauth", "none":
		return Plan{
			Runtime: "opencode",
			Actions: []Action{{
				Args:        []string{"opencode", "mcp", "add", "engram", "--url", opts.URL},
				Description: "register engram as an MCP server",
			}},
		}, nil
	case "bearer":
		return Plan{
			Runtime: "opencode",
			Actions: []Action{{
				Args: []string{"opencode", "mcp", "add", "engram", "--url", opts.URL,
					"--header", fmt.Sprintf("Authorization: Bearer %s", bearerProvenance(opts.TokenFile))},
				Description: "register engram as an MCP server (bearer token)",
			}},
		}, nil
	default:
		return Plan{}, fmt.Errorf("opencode: auth mode %q: %w", opts.Auth, ErrAuthModeUnsupported)
	}
}
