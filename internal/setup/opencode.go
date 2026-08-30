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
func (openCodeRuntime) Plan(_ Environment, opts Options) (Plan, error) {
	switch opts.Auth {
	case "oauth", "none":
		return Plan{
			Runtime: "opencode",
			Actions: []Action{{
				Command:     fmt.Sprintf("opencode mcp add engram --url %s", opts.URL),
				Description: "register engram as an MCP server",
			}},
		}, nil
	case "bearer":
		return Plan{
			Runtime: "opencode",
			Actions: []Action{{
				Command: fmt.Sprintf(
					`opencode mcp add engram --url %s --header "Authorization: Bearer %s"`,
					opts.URL, bearerProvenance(opts.TokenFile)),
				Description: "register engram as an MCP server (bearer token)",
			}},
		}, nil
	default:
		return Plan{}, fmt.Errorf("opencode: auth mode %q: %w", opts.Auth, ErrAuthModeUnsupported)
	}
}
