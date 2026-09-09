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
// (.planning/research/SUMMARY.md § "Post-Synthesis Live Verification",
// re-confirmed live at codex-cli 0.153.4 by 03-RESEARCH.md). opts.URL is
// used byte-for-byte, never appended to or stripped (D-02). The bearer
// mode names ENGRAM_TOKEN via codex's own `--bearer-token-env-var` flag
// rather than embedding a credential or its provenance in the command
// line at all — codex resolves the token from that named environment
// variable at its own invocation time, so there is no secret (and no
// path) for this command to carry. "none" reuses the same invocation as
// "oauth": codex's `mcp add` has no separate no-auth form. Codex is the
// only registered runtime whose `mcp add` genuinely overwrites an
// existing entry silently (03-RESEARCH.md Pitfall 1) — so, unlike
// claude-code, a single non-tolerant Action is the whole write sequence;
// no remove-then-add is needed. Probe is `codex mcp get <name> --json`, a
// pure local config read confirmed to dial no network (03-RESEARCH.md
// Pitfall 3) — the ideal D-09 convergence oracle.
func (codexRuntime) Plan(_ Environment, opts Options) (Plan, error) {
	probe := []string{"codex", "mcp", "get", "engram", "--json"}
	switch opts.Auth {
	case "oauth", "none":
		return Plan{
			Runtime: "codex",
			Actions: []Action{{
				Args:        []string{"codex", "mcp", "add", "engram", "--url", opts.URL},
				Description: "register engram as an MCP server",
			}},
			Probe: probe,
		}, nil
	case "oauth-client":
		return Plan{
			Runtime: "codex",
			Actions: []Action{{
				Args:        []string{"codex", "mcp", "add", "engram", "--url", opts.URL, "--oauth-client-id", "<id>"},
				Description: "register engram as an MCP server (pre-registered OAuth client)",
			}},
			Probe: probe,
		}, nil
	case "bearer":
		return Plan{
			Runtime: "codex",
			Actions: []Action{{
				Args:        []string{"codex", "mcp", "add", "engram", "--url", opts.URL, "--bearer-token-env-var", "ENGRAM_TOKEN"},
				Description: "register engram as an MCP server (bearer token via ENGRAM_TOKEN)",
			}},
			Probe: probe,
		}, nil
	default:
		return Plan{}, fmt.Errorf("codex: auth mode %q: %w", opts.Auth, ErrAuthModeUnsupported)
	}
}
