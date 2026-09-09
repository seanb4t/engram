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

// claudeCodeRemoveAction is the tolerant clear-the-slot action every auth
// mode's Plan() authors first (03-RESEARCH.md Pitfall 1, this phase's
// Task 1 checkpoint decision). `claude mcp add` has no --force/--overwrite
// flag and refuses with exit 1 ("already exists") on an existing name at
// EVERY scope, live-reproduced at both --scope project and --scope user —
// unlike codex and opencode, whose `mcp add` silently overwrites. Without
// this step, OutcomeAlreadyCorrect is unreachable for claude-code: every
// second `--apply` would report failed for an already-correct
// registration, violating REQ-setup-idempotent.
//
// Tolerant is true: `claude mcp remove engram --scope user` on an absent
// name exits 1 with "No MCP server named ... in user scope" — a message
// this action's tolerance is NEVER conditioned on (D-11's
// report-rather-than-diagnose philosophy and classifyOperatorErr's
// typed-cause-never-message-text discipline both forbid string-matching a
// third-party error). Tolerance here is blanket: any nonzero exit from
// this action is expected and never fails the row; the exit code and
// stderr are still recorded on Result.Notes via the shared executor
// (apply.go) so a genuinely broken `remove` stays visible even though it
// never fails the row.
//
// Description states the destructive-window consequence explicitly: if
// the following (fatal) add action fails or is interrupted after this
// action succeeds, the operator is left with NO claude-code registration
// where they previously had a working one, and engram cannot restore it —
// it never read the prior entry, and reading a runtime's config file is
// forbidden by this phase's central constraint. The only recovery is
// re-running --apply. This sentence is surfaced to the operator via the
// shared executor's Notes accumulation (apply.go), which records every
// tolerant action's Description regardless of its own exit code — a
// general executor behavior, not a claude-code special case.
var claudeCodeRemoveAction = Action{
	Args:     []string{"claude", "mcp", "remove", "engram", "--scope", "user"},
	Tolerant: true,
	Description: "clear any prior registration (tolerant of \"not found\"); " +
		"if the following registration action fails or is interrupted, no " +
		"claude-code registration remains — re-run --apply to recover",
}

// Plan authors claude-code's two-action write sequence for opts.Auth —
// the checkpoint-approved correction to a locked-decision premise this
// phase's own research falsified (03-RESEARCH.md Pitfall 1): every mode
// returns exactly two Actions, a tolerant claudeCodeRemoveAction first,
// then a fatal `claude mcp add` reproducing
// skill/engram/commands/engram-setup.md's table verbatim except where D-05
// changes the bearer form. opts.URL is used byte-for-byte, never appended
// to or stripped (D-02). "none" reuses the same invocation as "oauth":
// Claude Code's `claude mcp add` has no separate no-auth form, and a
// local/no-auth server simply never returns the 401 that would trigger
// the OAuth flow.
//
// Bearer mode (D-05, D-06): the --header value names ENGRAM_TOKEN as a
// shell-style ${...} variable reference, never bearerProvenance's
// path-provenance placeholder and never a credential value.
// 03-RESEARCH.md verified this end-to-end against claude 2.1.265: a live
// HTTP server registered with the exact header syntax
// "Authorization: Bearer ${ENGRAM_TOKEN}" received the RESOLVED
// environment value on connect, while `claude mcp get` and the on-disk
// config both echo back the literal, unexpanded "${ENGRAM_TOKEN}" text —
// so neither engram's write path nor its probe path ever observes the
// secret. bearerProvenance itself is not deleted; the generic pseudo-
// runtime (a later plan) still uses it.
//
// Every Plan carries Probe: []string{"claude", "mcp", "get", "engram"} —
// authored in this same call (D-09). This probe DIALS the registered URL
// for a user-scope entry (live-observed, 03-RESEARCH.md Pitfall 3), so the
// convergence read is sensitive to the engram server's own transient
// reachability. That degrades in the safe direction only: D-08's
// ambiguity-resolves-to-wrote invariant means an unstable probe can cause
// a false OutcomeWrote, never a false OutcomeAlreadyCorrect.
//
// This Plan runs through the SAME shared executor (apply.go) every other
// runtime uses — Action.Tolerant is read from each action's own authored
// field, never from its position in Plan.Actions or from Plan.Runtime's
// name, so no per-runtime execution code exists anywhere outside this
// file.
func (claudeCodeRuntime) Plan(_ Environment, opts Options) (Plan, error) {
	switch opts.Auth {
	case "oauth", "none":
		return Plan{
			Runtime: "claude-code",
			Actions: []Action{
				claudeCodeRemoveAction,
				{
					Args:        []string{"claude", "mcp", "add", "--transport", "http", "engram", opts.URL, "--scope", "user"},
					Description: "register engram as a user-scope MCP server",
				},
			},
			Probe: []string{"claude", "mcp", "get", "engram"},
		}, nil
	case "oauth-client":
		return Plan{
			Runtime: "claude-code",
			Actions: []Action{
				claudeCodeRemoveAction,
				{
					Args: []string{"claude", "mcp", "add", "--transport", "http", "engram", opts.URL,
						"--scope", "user", "--client-id", "<id>", "--client-secret", "--callback-port", "8765"},
					// --client-secret deliberately takes no inline value: Claude
					// Code prompts for it with masked input, which is why no
					// secret can reach argv on this path.
					Description: "register engram as a user-scope MCP server (pre-registered OAuth client)",
				},
			},
			Probe: []string{"claude", "mcp", "get", "engram"},
		}, nil
	case "bearer":
		return Plan{
			Runtime: "claude-code",
			Actions: []Action{
				claudeCodeRemoveAction,
				{
					Args: []string{"claude", "mcp", "add", "--transport", "http", "engram", opts.URL,
						"--scope", "user", "--header", "Authorization: Bearer ${ENGRAM_TOKEN}"},
					Description: "register engram as a user-scope MCP server (bearer token via ENGRAM_TOKEN)",
				},
			},
			Probe: []string{"claude", "mcp", "get", "engram"},
		}, nil
	default:
		return Plan{}, fmt.Errorf("claude-code: auth mode %q: %w", opts.Auth, ErrAuthModeUnsupported)
	}
}
