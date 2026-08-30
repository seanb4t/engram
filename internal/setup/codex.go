// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import "fmt"

// codexRuntime implements Runtime for Codex, authoring the live-verified
// `codex mcp add <NAME> --url <URL>` invocation surface
// (.planning/research/SUMMARY.md § "Post-Synthesis Live Verification") —
// structurally identical to claudeCodeRuntime, no special-casing.
type codexRuntime struct{}

// Codex is the Runtime registered for Codex.
var Codex Runtime = codexRuntime{}

func (codexRuntime) Name() string { return "codex" }

// Detect consults env.LookPath("codex") exclusively (D-12) — never a
// config directory.
func (codexRuntime) Detect(env Environment) bool {
	_, err := env.LookPath("codex")
	return err == nil
}

// Plan authors the exact `codex mcp add` invocation for opts.Auth, from
// the live-verified `codex mcp add <NAME> --url <URL>` surface
// (.planning/research/SUMMARY.md § "Post-Synthesis Live Verification").
// opts.URL is used byte-for-byte, never appended to or stripped (D-02).
// The bearer mode names ENGRAM_TOKEN via codex's own
// `--bearer-token-env-var` flag rather than embedding a credential or its
// provenance in the command line at all — codex resolves the token from
// that named environment variable at its own invocation time, so there is
// no secret (and no path) for this command to carry. "none" reuses the
// same invocation as "oauth": codex's `mcp add` has no separate no-auth
// form.
func (codexRuntime) Plan(_ Environment, opts Options) (Plan, error) {
	switch opts.Auth {
	case "oauth", "none":
		return Plan{
			Runtime: "codex",
			Actions: []Action{{
				Command:     fmt.Sprintf("codex mcp add engram --url %s", opts.URL),
				Description: "register engram as an MCP server",
			}},
		}, nil
	case "oauth-client":
		return Plan{
			Runtime: "codex",
			Actions: []Action{{
				Command:     fmt.Sprintf("codex mcp add engram --url %s --oauth-client-id <id>", opts.URL),
				Description: "register engram as an MCP server (pre-registered OAuth client)",
			}},
		}, nil
	case "bearer":
		return Plan{
			Runtime: "codex",
			Actions: []Action{{
				Command:     fmt.Sprintf("codex mcp add engram --url %s --bearer-token-env-var ENGRAM_TOKEN", opts.URL),
				Description: "register engram as an MCP server (bearer token via ENGRAM_TOKEN)",
			}},
		}, nil
	default:
		return Plan{}, fmt.Errorf("codex: auth mode %q: %w", opts.Auth, ErrAuthModeUnsupported)
	}
}
