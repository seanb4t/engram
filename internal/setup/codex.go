// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
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
// (opencode.ai/docs/skills/), so this single write covers both.
//
// Any opts.Headers entry is declined up front (D-09, D-10, before this
// method resolves home or dispatches on opts.Auth) because `codex mcp
// add` (codex-cli 0.154.0, live-probed read-only in 02-RESEARCH.md)
// exposes only --bearer-token-env-var — a purpose-built,
// Authorization-shaped flag — and no generic header flag; openai/codex#5180
// was closed COMPLETED by adding header keys to Codex's own config file,
// not a CLI flag, so this is Codex's shipped design rather than a
// temporary gap. The decline is per-runtime (claude-code, opencode, and
// generic CAN express any header name), which is why it lives here and
// not at the CLI boundary, and why it is the same failed-row shape
// oauth-client on opencode already produces.
//
// A live research machine also showed a populated, working-looking
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
	if len(opts.Headers) > 0 {
		sorted := sortedHeaders(opts.Headers)
		names := make([]string, len(sorted))
		for i, h := range sorted {
			names[i] = h.Name
		}
		return Plan{}, fmt.Errorf("codex: custom header(s) %s: codex mcp add exposes only --bearer-token-env-var (no custom header flag); drop --header or exclude codex via --runtime: %w",
			strings.Join(names, ", "), ErrHeaderUnsupported)
	}

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
				Args:        []string{"codex", "mcp", "add", "engram", "--url", opts.URL, "--oauth-client-id", opts.ClientID},
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

// codexPluginListProbe is codex's D-10 capability-and-state probe: the
// bare `plugin list --json` call already populates the documented
// "installed" array (03-RESEARCH.md Pitfall 2) — no flag that would ADD
// the not-yet-installed listing is ever authored, since this lane only
// needs the installed-state question answered.
// codexPluginMarketplaceProbe is the D-11 marketplace probe: whether a
// marketplace named "engram" is already registered, and under what root.
var codexPluginListProbe = []string{"codex", "plugin", "list", "--json"}
var codexPluginMarketplaceProbe = []string{"codex", "plugin", "marketplace", "list"}

// codexPluginMarketplaceAddAction is the ONLY marketplace source this
// package ever authors for codex (D-04): engram's own GitHub repository
// as an HTTPS Git URL, default branch, unpinned — no flag pins it to a
// particular revision. Authored only when PluginActions observes no
// marketplace already named "engram" (D-05) — a marketplace that already
// exists under that name, whatever it points at, is used as-is and never
// re-pointed. Codex has no scope concept, so this action carries none
// (D-06).
var codexPluginMarketplaceAddAction = Action{
	Args:        []string{"codex", "plugin", "marketplace", "add", "https://github.com/seanb4t/engram", "--json"},
	Description: "add engram's own plugin marketplace (HTTPS Git URL, default branch, unpinned)",
}

// codexPluginAddAction installs the engram plugin (skills, hooks,
// /engram-setup) from engram's own marketplace. Codex is the one
// registered runtime whose `plugin add` genuinely overwrites an existing
// entry silently (unlike claude-code's `plugin install`, which refuses on
// an existing entry) — so, unlike claude-code, no tolerant clear-the-slot
// step is needed ahead of it.
var codexPluginAddAction = Action{
	Args:        []string{"codex", "plugin", "add", "engram@engram", "--json"},
	Description: "install the engram plugin (skills, hooks, /engram-setup) from engram's own marketplace",
}

// codexPluginRemoveAction is D-02's substitute for a native update verb:
// codex has no `plugin update` subcommand at all, so an outdated plugin
// is updated by removing it and re-adding it. If the following add fails
// after this action succeeds, no codex plugin remains until --apply is
// re-run — the same destructive-window shape claudeCodeRemoveAction
// documents for its own tolerant clear step, restated here as a fatal
// (non-tolerant) ordering instead, since D-02 requires remove to
// STRICTLY precede add rather than merely tolerate a prior absence.
var codexPluginRemoveAction = Action{
	Args:        []string{"codex", "plugin", "remove", "engram@engram", "--json"},
	Description: "remove the outdated engram plugin before re-adding it (codex has no update verb — D-02); if the following add fails, no codex plugin remains — re-run --apply to recover",
}

// PluginProbes implements PluginRuntime.
func (codexRuntime) PluginProbes() (list, marketplace []string) {
	return codexPluginListProbe, codexPluginMarketplaceProbe
}

// codexPluginListEntry is the tolerant, minimal shape of one entry in
// `codex plugin list --json`'s "installed" array.
type codexPluginListEntry struct {
	Name            string `json:"name"`
	MarketplaceName string `json:"marketplaceName"`
	Version         string `json:"version"`
}

// codexPluginListDoc is the documented top-level shape: BOTH "installed"
// and "available" keys are always present (03-RESEARCH.md Code
// Examples). Installed is a pointer so a genuinely missing key (an
// object that is not this documented shape at all, e.g. "{}") is
// distinguishable from a present-but-empty array.
type codexPluginListDoc struct {
	Installed *[]codexPluginListEntry `json:"installed"`
}

// ParsePluginList implements PluginRuntime. A non-JSON-object stdout (an
// array, empty, or garbage) returns the unmarshal error; a JSON object
// missing the documented "installed" key returns a distinct error naming
// it — neither is this CLI's documented JSON shape (D-10's
// no-working-plugin-CLI signal). A parsed document with an empty
// "installed" array returns installed == false, nil error (present but
// empty is absent, not unavailable). Matching requires BOTH name and
// marketplaceName equal "engram" — a same-named plugin from another
// marketplace is not engram's own.
func (codexRuntime) ParsePluginList(stdout string) (version string, installed bool, err error) {
	var doc codexPluginListDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		return "", false, err
	}
	if doc.Installed == nil {
		return "", false, fmt.Errorf(`codex: plugin list --json response has no "installed" key`)
	}
	for _, entry := range *doc.Installed {
		if entry.Name == "engram" && entry.MarketplaceName == "engram" {
			return entry.Version, true, nil
		}
	}
	return "", false, nil
}

// ParseMarketplaceList implements PluginRuntime: a COARSE, position-based
// read of `codex plugin marketplace list`'s two-column table — never a
// table model (D-11, 03-RESEARCH.md Pitfall 5). The first line whose
// first whitespace-separated field is "engram" is present, with source
// set to the remainder of that line (the ROOT column) joined back
// together; the header line never matches, since its first field is
// "MARKETPLACE".
func (codexRuntime) ParseMarketplaceList(stdout string) (present bool, source string) {
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "engram" {
			return true, strings.Join(fields[1:], " ")
		}
	}
	return false, ""
}

// PluginActions implements PluginRuntime: PluginAbsent authors a
// marketplace-add (only when the marketplace probe found none) then an
// add; PluginOutdated authors a remove strictly before an add (D-02,
// codex has no native update verb); PluginCurrent and PluginUnavailable
// author nothing.
func (codexRuntime) PluginActions(state PluginState, marketplacePresent bool) []Action {
	switch state {
	case PluginAbsent:
		var actions []Action
		if !marketplacePresent {
			actions = append(actions, codexPluginMarketplaceAddAction)
		}
		return append(actions, codexPluginAddAction)
	case PluginOutdated:
		return []Action{codexPluginRemoveAction, codexPluginAddAction}
	default:
		return nil
	}
}
