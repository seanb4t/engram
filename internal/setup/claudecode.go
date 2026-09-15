// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
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
// claudeCodeHeaderArgs renders one "--header" / "NAME: ${ENVVAR}" pair
// per entry of sortedHeaders(hs) — claude-code's own colon-space
// HTTP-header-string dialect, live-verified in 02-RESEARCH.md against
// `claude mcp add --help` 2.1.270 (`-H, --header <header...>`). Returns
// nil for no headers, so appending its result to an existing Args slice
// is a no-op and every no-header Args slice stays byte-identical to HEAD
// (D-01, D-04, D-08). This is the ONE place claude-code's header dialect
// is authored — no other runtime's file shares it (the opencode
// colon-space regression is exactly the anti-pattern this separation
// avoids).
func claudeCodeHeaderArgs(hs []HeaderSpec) []string {
	sorted := sortedHeaders(hs)
	if len(sorted) == 0 {
		return nil
	}
	args := make([]string, 0, len(sorted)*2)
	for _, h := range sorted {
		args = append(args, "--header", h.Name+": ${"+h.EnvVar+"}")
	}
	return args
}

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
//
// Extra headers (D-01, D-04, D-08): opts.Headers is valid with every auth
// mode and renders as bare "${ENVVAR}" references in claude-code's own
// syntax, never a scheme — any Bearer/raw-key shape lives in the
// variable's value, which engram never sees. Each sorted extra header is
// appended, via claudeCodeHeaderArgs, to the SAME `claude mcp add`
// action's Args after every shipped argument (and, in the bearer arm,
// after the auth-mode header itself) — never a second Action.
//
// Every auth mode also authors the SAME SkillTarget (Phase 4, D-05,
// D-10): skills install at user scope only, with the destination derived
// from env.HomeDir() — never a literal beginning with a tilde and never
// an environment-variable read. A HomeDir failure is reported as a failed
// row naming this runtime, exactly like any other Plan() error.
func (claudeCodeRuntime) Plan(env Environment, opts Options) (Plan, error) {
	home, err := env.HomeDir()
	if err != nil {
		return Plan{}, fmt.Errorf("claude-code: resolve home directory: %w", err)
	}
	skillTarget := SkillTarget{
		Format: SkillFormatNative,
		Dir:    filepath.Join(home, ".claude", "skills"),
	}

	switch opts.Auth {
	case "oauth", "none":
		return Plan{
			Runtime: "claude-code",
			Actions: []Action{
				claudeCodeRemoveAction,
				{
					Args: append([]string{"claude", "mcp", "add", "--transport", "http", "engram", opts.URL, "--scope", "user"},
						claudeCodeHeaderArgs(opts.Headers)...),
					Description: "register engram as a user-scope MCP server",
				},
			},
			Probe:  []string{"claude", "mcp", "get", "engram"},
			Skills: skillTarget,
		}, nil
	case "oauth-client":
		return Plan{
			Runtime: "claude-code",
			Actions: []Action{
				claudeCodeRemoveAction,
				{
					Args: append([]string{"claude", "mcp", "add", "--transport", "http", "engram", opts.URL,
						"--scope", "user", "--client-id", opts.ClientID, "--client-secret", "--callback-port", "8765"},
						claudeCodeHeaderArgs(opts.Headers)...),
					// --client-secret deliberately takes no inline value: Claude
					// Code can prompt interactively, but engram provides no stdin.
					// Scripted registration requires MCP_CLIENT_SECRET in the
					// inherited environment; no secret reaches argv here.
					Description: "register engram as a user-scope MCP server (pre-registered OAuth client)",
				},
			},
			Probe:  []string{"claude", "mcp", "get", "engram"},
			Skills: skillTarget,
		}, nil
	case "bearer":
		return Plan{
			Runtime: "claude-code",
			Actions: []Action{
				claudeCodeRemoveAction,
				{
					Args: append([]string{"claude", "mcp", "add", "--transport", "http", "engram", opts.URL,
						"--scope", "user", "--header", "Authorization: Bearer ${ENGRAM_TOKEN}"},
						claudeCodeHeaderArgs(opts.Headers)...),
					Description: "register engram as a user-scope MCP server (bearer token via ENGRAM_TOKEN)",
				},
			},
			Probe:  []string{"claude", "mcp", "get", "engram"},
			Skills: skillTarget,
		}, nil
	default:
		return Plan{}, fmt.Errorf("claude-code: auth mode %q: %w", opts.Auth, ErrAuthModeUnsupported)
	}
}

// claudePluginListProbe is claude-code's D-10 capability-and-state probe:
// one JSON read that answers both "does this CLI have a working plugin
// subsystem" and "is engram installed, and at what version" (plugin.go's
// executePlugin). claudePluginMarketplaceProbe is the D-11 marketplace
// probe: whether a marketplace named "engram" is already registered, and
// under what source.
var claudePluginListProbe = []string{"claude", "plugin", "list", "--json"}
var claudePluginMarketplaceProbe = []string{"claude", "plugin", "marketplace", "list"}

// claudePluginMarketplaceAddAction is the ONLY marketplace source this
// package ever authors for claude-code (D-04): engram's own GitHub
// repository, default branch, unpinned, at user scope (D-06). Authored
// only when PluginActions observes no marketplace already named "engram"
// (D-05) — a marketplace that already exists under that name, whatever it
// points at, is used as-is and never re-pointed.
var claudePluginMarketplaceAddAction = Action{
	Args:        []string{"claude", "plugin", "marketplace", "add", "seanb4t/engram", "--scope", "user"},
	Description: "add engram's own plugin marketplace (GitHub owner/repo form, default branch, unpinned, user scope)",
}

// claudePluginInstallAction installs the engram plugin (skills, hooks,
// /engram-setup) from engram's own marketplace. -y is REQUIRED for a
// non-interactive/non-TTY invocation (03-RESEARCH.md Pitfall 1, live
// `claude plugin install --help`) — omitting it hangs or fails every
// scripted --apply. --scope user is D-06's explicit pin, matching `mcp
// add --scope user`; no engram-side scope flag exists for this action.
var claudePluginInstallAction = Action{
	Args:        []string{"claude", "plugin", "install", "engram@engram", "--scope", "user", "--json", "-y"},
	Description: "install the engram plugin (skills, hooks, /engram-setup) from engram's own marketplace",
}

// claudePluginUpdateAction updates an outdated engram plugin to the
// marketplace's current version. Same -y and user-scope discipline as
// claudePluginInstallAction above.
var claudePluginUpdateAction = Action{
	Args:        []string{"claude", "plugin", "update", "engram@engram", "--scope", "user", "--json", "-y"},
	Description: "update the engram plugin to the marketplace's current version",
}

// PluginProbes implements PluginRuntime.
func (claudeCodeRuntime) PluginProbes() (list, marketplace []string) {
	return claudePluginListProbe, claudePluginMarketplaceProbe
}

// claudePluginListEntry is the tolerant, minimal shape of one entry in
// `claude plugin list --json`'s array — extra fields such as
// "mcpServers" (present on some entries, absent on others,
// 03-RESEARCH.md Code Examples) are ignored.
type claudePluginListEntry struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// ParsePluginList implements PluginRuntime. A non-JSON-array stdout
// (empty, an object, or garbage) returns the unmarshal error — D-10's
// no-working-plugin-CLI signal. A parsed array with no engram@engram
// entry returns installed == false, matched by id, never by position (a
// decoy entry may sort first).
func (claudeCodeRuntime) ParsePluginList(stdout string) (version string, installed bool, err error) {
	var entries []claudePluginListEntry
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		return "", false, err
	}
	for _, entry := range entries {
		if entry.ID == "engram@engram" {
			return entry.Version, true, nil
		}
	}
	return "", false, nil
}

// ParseMarketplaceList implements PluginRuntime: a COARSE exact-name
// match over `claude plugin marketplace list`'s human-formatted output —
// two adjacent lines matched by position, never a table model (D-11,
// 03-RESEARCH.md Pitfall 5). A line naming the marketplace "engram" is
// found by stripping the leading marker glyph; the source is the first
// following non-empty line, with its own "Source:" prefix removed.
func (claudeCodeRuntime) ParseMarketplaceList(stdout string) (present bool, source string) {
	lines := strings.Split(stdout, "\n")
	for i, line := range lines {
		name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "❯"))
		if name != "engram" {
			continue
		}
		for _, follow := range lines[i+1:] {
			trimmed := strings.TrimSpace(follow)
			if trimmed == "" {
				continue
			}
			trimmed = strings.TrimPrefix(trimmed, "Source:")
			return true, strings.TrimSpace(trimmed)
		}
		return true, ""
	}
	return false, ""
}

// PluginActions implements PluginRuntime: PluginAbsent authors a
// marketplace-add (only when the marketplace probe found none) then an
// install; PluginOutdated authors an update; PluginCurrent and
// PluginUnavailable author nothing (D-01, D-04, D-05, D-06, D-10, D-11).
func (claudeCodeRuntime) PluginActions(state PluginState, marketplacePresent bool) []Action {
	switch state {
	case PluginAbsent:
		var actions []Action
		if !marketplacePresent {
			actions = append(actions, claudePluginMarketplaceAddAction)
		}
		return append(actions, claudePluginInstallAction)
	case PluginOutdated:
		return []Action{claudePluginUpdateAction}
	default:
		return nil
	}
}
