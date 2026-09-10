// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"fmt"
	"path/filepath"
)

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
//
// Every auth mode also authors the SAME SkillTarget (Phase 4, D-05, D-10,
// and 04-03-SUMMARY.md's recorded routing decision,
// "codex-native-plus-index"): skills are written to $HOME/.agents/skills
// — what Codex's own official documentation names as the USER scope for
// personal skills (04-RESEARCH.md § "Native format and destination per
// runtime", citing learn.chatgpt.com/docs/build-skills), and
// independently one of opencode's own documented global discovery paths
// (opencode.ai/docs/skills/), so this single write covers both. A live
// research machine also showed a populated, working-looking
// $CODEX_HOME/skills (04-RESEARCH.md's Codex discrepancy write-up); that
// path was DELIBERATELY NOT CHOSEN, and engram never writes two skills
// destinations for one runtime — a future contributor must not "fix"
// this by adding the second path.
//
// Under the recorded routing, Codex ALSO carries an index: the skills
// index block is spliced into $HOME/.codex/AGENTS.md — precisely the
// file 04-CONTEXT.md's D-16 symlink rationale was written about — as a
// hedge for RESEARCH assumption A1 (if Codex's own skills loader does
// not read $HOME/.agents/skills, the index still teaches the agent the
// skills exist and names their absolute paths). SkillFormatAgentsMD is
// what makes both halves happen from one Target:
// internal/skills.Install's FormatAgentsMD case writes skill files to Dir
// exactly as FormatNative does, then independently splices the index
// into IndexFile (internal/skills/install.go). A HomeDir failure is
// reported as a failed row naming this runtime, exactly like any other
// Plan() error.
func (codexRuntime) Plan(env Environment, opts Options) (Plan, error) {
	home, err := env.HomeDir()
	if err != nil {
		return Plan{}, fmt.Errorf("codex: resolve home directory: %w", err)
	}
	skillTarget := SkillTarget{
		Format:    SkillFormatAgentsMD,
		Dir:       filepath.Join(home, ".agents", "skills"),
		IndexFile: filepath.Join(home, ".codex", "AGENTS.md"),
	}

	probe := []string{"codex", "mcp", "get", "engram", "--json"}
	switch opts.Auth {
	case "oauth", "none":
		return Plan{
			Runtime: "codex",
			Actions: []Action{{
				Args:        []string{"codex", "mcp", "add", "engram", "--url", opts.URL},
				Description: "register engram as an MCP server",
			}},
			Probe:  probe,
			Skills: skillTarget,
		}, nil
	case "oauth-client":
		return Plan{
			Runtime: "codex",
			Actions: []Action{{
				Args:        []string{"codex", "mcp", "add", "engram", "--url", opts.URL, "--oauth-client-id", "<id>"},
				Description: "register engram as an MCP server (pre-registered OAuth client)",
			}},
			Probe:  probe,
			Skills: skillTarget,
		}, nil
	case "bearer":
		return Plan{
			Runtime: "codex",
			Actions: []Action{{
				Args:        []string{"codex", "mcp", "add", "engram", "--url", opts.URL, "--bearer-token-env-var", "ENGRAM_TOKEN"},
				Description: "register engram as an MCP server (bearer token via ENGRAM_TOKEN)",
			}},
			Probe:  probe,
			Skills: skillTarget,
		}, nil
	default:
		return Plan{}, fmt.Errorf("codex: auth mode %q: %w", opts.Auth, ErrAuthModeUnsupported)
	}
}
