// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"fmt"
)

// claudeCodeRuntime implements Runtime for Claude Code, authoring the
// exact `claude mcp add` invocation the shipped
// skill/engram/commands/engram-setup.md prose path already issues — never
// by hand-editing settings files.
type claudeCodeRuntime struct{}

// ClaudeCode is the Runtime registered for Claude Code.
var ClaudeCode Runtime = claudeCodeRuntime{}

func (claudeCodeRuntime) Name() string { return "claude-code" }

// Detect consults env.LookPath("claude") exclusively (D-12) — never a
// config-directory stat, so a leftover ~/.claude/ from an uninstalled
// binary cannot read as installed. A runtime installed outside PATH reads
// as absent; that is an accepted false negative, the safe direction.
func (claudeCodeRuntime) Detect(env Environment) bool {
	_, err := env.LookPath("claude")
	return err == nil
}

// Plan authors the exact `claude mcp add` invocation for opts.Auth,
// reproducing skill/engram/commands/engram-setup.md's table verbatim
// (D-09) — these four invocation strings are AUTHORED HERE and nowhere
// else. opts.URL is used byte-for-byte, never appended to or stripped
// (D-02). The bearer mode's credential is never opts.TokenFile's
// contents, only its provenance (D-16, bearerProvenance) — Plan never
// opens, stats, or reads the file. "none" reuses the same invocation as
// "oauth": Claude Code's `claude mcp add` has no separate no-auth form,
// and a local/no-auth server simply never returns the 401 that would
// trigger the OAuth flow.
func (claudeCodeRuntime) Plan(_ Environment, opts Options) (Plan, error) {
	switch opts.Auth {
	case "oauth", "none":
		return Plan{
			Runtime: "claude-code",
			Actions: []Action{{
				Command:     fmt.Sprintf("claude mcp add --transport http engram %s --scope user", opts.URL),
				Description: "register engram as a user-scope MCP server",
			}},
		}, nil
	case "oauth-client":
		return Plan{
			Runtime: "claude-code",
			Actions: []Action{{
				Command: fmt.Sprintf(
					"claude mcp add --transport http engram %s --scope user --client-id <id> --client-secret --callback-port 8765",
					opts.URL),
				Description: "register engram as a user-scope MCP server (pre-registered OAuth client)",
			}},
		}, nil
	case "bearer":
		return Plan{
			Runtime: "claude-code",
			Actions: []Action{{
				Command: fmt.Sprintf(
					`claude mcp add --transport http engram %s --scope user --header "Authorization: Bearer %s"`,
					opts.URL, bearerProvenance(opts.TokenFile)),
				Description: "register engram as a user-scope MCP server (bearer token)",
			}},
		}, nil
	default:
		return Plan{}, fmt.Errorf("claude-code: auth mode %q: %w", opts.Auth, ErrAuthModeUnsupported)
	}
}
