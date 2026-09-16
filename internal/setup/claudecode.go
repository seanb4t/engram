// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
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
						"--scope", "user", "--header", "Authorization: " + claudeCodeBearerForm},
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

// claudeCodeBearerForm is the ONE bearer-token header VALUE claude-code's
// Plan ever authors (its own bearer arm above references this same
// constant via "Authorization: "+claudeCodeBearerForm, so the two cannot
// drift) and the value Observe compares an observed Authorization header
// against to classify AuthBearer vs. AuthForeign (drift.go) — mirroring
// codex.go's codexBearerForm/Observe pairing, AUTHORED HERE: claude-code's
// own header dialect is never shared with codex's file.
const claudeCodeBearerForm = "Bearer ${ENGRAM_TOKEN}"

// claudeCodeWholeEntryNote is REQ-drift-preserved-outcome's fixed,
// composed-once sentence naming claude-code's whole-entry semantics
// (mirroring codexWholeEntryNote): `claude mcp remove` then `claude mcp
// add` has no partial-merge form — the pair either overwrites the whole
// entry or (if remove found nothing and add then fails) leaves the prior
// entry untouched — so a preserved row's Reason must state that risk
// plainly, not merely name the differing facet.
const claudeCodeWholeEntryNote = "claude mcp remove then add replaces the whole entry: --apply would overwrite it or leave it untouched, never merge into it"

// claudeCodeManualRemediation is D-05's fixed, runtime-authored sentence
// naming the exact manual step that clears a preserved claude-code
// registration — the human performs the destructive `claude mcp remove`
// step themselves, with claude-code's own confirmation semantics, never
// engram's. Composed here, never in apply.go/drift.go (AUTHORED-HERE):
// the shared executor only appends this field when it is non-empty.
// After the human runs this command, a re-run of --apply reads
// would-write — nothing remains to preserve.
const claudeCodeManualRemediation = "to replace it yourself, clear it with claude-code's own tool first: claude mcp remove engram --scope user, then run setup again; the row then reads would-write"

// claudeCodeOAuthReLoginNote is D-03/D-04's fixed, runtime-authored
// consequence sentence: a claude-code registration observed with NO
// Authorization header (AuthNone) is treated as OAuth-authenticated BY
// SHAPE — that is how a real oauth or oauth-client registration reads
// back — so a would-write row (claude mcp remove then add) discards that
// login. Deliberately conservative: it fires even if the user never
// completed a login, because a false "you will need to log in again"
// costs nothing and a missed one is a silent logout. Composed here, never
// in apply.go/drift.go (AUTHORED-HERE): the shared executor only
// consults Observation.RewriteConsequence, a generic, content-blind
// field — never a comparison against opts.Auth or "claude-code" by name
// (D-04, Pitfall 4). No stderr side-channel, no TTY pause: --apply is the
// milestone's only consent gate, and this sentence is the single
// rendering path, on Result.Notes, in both preview and apply.
const claudeCodeOAuthReLoginNote = "the existing registration carries no Authorization header, so it is treated as OAuth-authenticated: claude mcp remove then add discards that login, and you will need to log in again after --apply (in Claude Code run /mcp, select engram, and authenticate)"

// unrecognizedLabelBound is claude-code's own copy of codex.go's identical
// bound — AUTHORED HERE rather than shared, since 04-RESEARCH.md Pitfall 3
// forbids a cross-runtime parsing dependency, and this is a two-line
// utility, not a parser.
const claudeCodeUnrecognizedLabelBound = 40

// claudeCodeLineLabel extracts a D-11 unrecognized-content label from one
// line this scanner's known vocabulary does not account for: the text
// before its first colon, trimmed, bounded to
// claudeCodeUnrecognizedLabelBound bytes at a rune boundary, and
// quoteWord'ed for paste-safety — the remainder after the colon (which
// may carry an observed value, e.g. a header line missing its expected
// "Name: value" shape) is NEVER copied. A line with no colon at all
// contributes the fixed token "line" instead of any of its own text.
func claudeCodeLineLabel(line string) string {
	idx := strings.Index(line, ":")
	if idx == -1 {
		return quoteWord("line")
	}
	label := strings.TrimSpace(line[:idx])
	if len(label) > claudeCodeUnrecognizedLabelBound {
		limit := claudeCodeUnrecognizedLabelBound
		for limit > 0 && !utf8.RuneStart(label[limit]) {
			limit--
		}
		label = label[:limit]
	}
	return quoteWord(label)
}

// Observe implements DriftRuntime for claude-code: a TOTAL parse (D-11) of
// `claude mcp get engram`'s combined stdout+stderr, built entirely from
// the verbatim framing .planning/phases/04-drift-detection-read-only/
// 04-OBSERVATIONS.md §"Claude Code — literal value" recorded (claude
// 2.1.273, 2026-09-15) — every line class this method recognizes is
// justified by a line that record shows. AUTHORED HERE: claude-code's own
// fixed-label text is parsed only in this file, sharing no parsing code
// with codex.go's JSON scan (04-RESEARCH.md Pitfall 3).
//
// (1) Framing: the presence of a "URL:" line (matched by trimmed prefix)
// is the ONLY thing that determines ok. Its absence — an empty probe, the
// post-remove "No MCP server named ..." text, or any other garbage —
// returns Observation{}, false uniformly (D-09): a not-found entry, an
// empty capture, and an unparseable capture are indistinguishable at this
// layer, all resolving to "not compared" one level up (apply.go).
//
// (2) Every OTHER non-blank line is classified by trimmed prefix into
// exactly one class: the entry-name line — matched by SHAPE (zero
// indentation, ends with ":"), and ONLY for the very first non-blank line
// encountered, so the scanner is name-agnostic and never checks for the
// literal string "engram"; "Scope:" — chrome; "Status:" and any
// "Issue:"-prefixed continuation line — chrome by prefix, regardless of
// content (04-RESEARCH.md Pitfall 4: connection state varies with network
// reachability and must never affect classification — a failed dial's
// exit code does not block parsing either, since execute() in apply.go
// hands this method probe1.Stdout+probe1.Stderr regardless of exit code);
// "Type:" — a FACET line: its value compared case-insensitively to "http"
// is accounted for, any other value contributes the fixed label "Type" to
// Unrecognized (D-01: claude-code authors no other transport); "URL:" —
// already captured in step 1; "Headers:" — opens the header block: every
// following line indented deeper than "Headers:" own indentation is a
// header line, split at its first ": " into name and raw value (an
// "Authorization" name, matched case-insensitively, is folded into Auth
// below and NEVER added to the raw header list); the trailing hint line
// "To remove this server, run: ..." — chrome by its fixed prefix; every
// other non-blank line — claudeCodeLineLabel(line) appended to
// Unrecognized (D-11: unaccounted content is always reported, never
// silently ignored).
//
// (3) Auth: an observed "Authorization" header's raw value is compared,
// INSIDE this call frame only, against claudeCodeBearerForm — equal ->
// AuthBearer, otherwise -> AuthForeign; no "Authorization" line observed
// at all -> AuthNone. The raw value itself never survives past this
// comparison (D-02/D-03).
//
// (4) Headers: the observed non-Authorization header lines are joined
// (drift.go's joinHeaders) against a planned side built from
// sortedHeaders(opts.Headers) rendered as "${"+EnvVar+"}" — the SAME bare
// reference form claudeCodeHeaderArgs authors for the write path; the two
// must stay in lockstep, referenced by name rather than duplicated.
//
// ok is false ONLY when probeOutput cannot be framed as a registration AT
// ALL (no "URL:" line) — never for unrecognized CONTENT within an
// otherwise-framed registration, which is reported on
// Observation.Unrecognized instead (D-11).
func (claudeCodeRuntime) Observe(probeOutput string, opts Options) (Observation, bool) {
	lines := strings.Split(probeOutput, "\n")

	var url string
	foundURL := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(trimmed, "URL:"); ok {
			url = strings.TrimSpace(rest)
			foundURL = true
			break
		}
	}
	if !foundURL {
		return Observation{}, false
	}

	var unrecognized []string
	var observedHeaders []rawHeader
	auth := AuthNone
	inHeaderBlock := false
	headerIndent := 0
	seenFirstLine := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			inHeaderBlock = false
			continue
		}
		leading := len(line) - len(strings.TrimLeft(line, " "))

		if inHeaderBlock && leading > headerIndent {
			name, value, ok := strings.Cut(trimmed, ": ")
			if !ok {
				unrecognized = append(unrecognized, claudeCodeLineLabel(trimmed))
				continue
			}
			if strings.EqualFold(name, "Authorization") {
				if value == claudeCodeBearerForm {
					auth = AuthBearer
				} else {
					auth = AuthForeign
				}
				continue
			}
			observedHeaders = append(observedHeaders, rawHeader{Name: name, Value: value})
			continue
		}
		inHeaderBlock = false

		first := !seenFirstLine
		seenFirstLine = true

		switch {
		case first && leading == 0 && strings.HasSuffix(trimmed, ":"):
			// Entry-name line — matched by SHAPE, never the literal name.
		case strings.HasPrefix(trimmed, "Scope:"):
			// Chrome (Claude's own discretion, this plan's objective:
			// engram's write is always user scope, so comparing scope
			// would add a facet with no safety gain).
		case strings.HasPrefix(trimmed, "Status:"), strings.HasPrefix(trimmed, "Issue:"):
			// Chrome by prefix, regardless of content (Pitfall 4): a live
			// connection-status line must never gate classification.
		case strings.HasPrefix(trimmed, "Type:"):
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "Type:"))
			if !strings.EqualFold(val, "http") {
				unrecognized = append(unrecognized, quoteWord("Type"))
			}
		case strings.HasPrefix(trimmed, "URL:"):
			// Already captured above.
		case strings.HasPrefix(trimmed, "Headers:"):
			inHeaderBlock = true
			headerIndent = leading
		case strings.HasPrefix(trimmed, "To remove this server, run:"):
			// Trailing hint chrome.
		default:
			unrecognized = append(unrecognized, claudeCodeLineLabel(trimmed))
		}
	}

	var planned []plannedHeader
	for _, h := range sortedHeaders(opts.Headers) {
		planned = append(planned, plannedHeader{Name: h.Name, Value: "${" + h.EnvVar + "}"})
	}
	headers := joinHeaders(observedHeaders, planned)

	// D-03/D-04: the OAuth re-login consequence fires by SHAPE alone —
	// no Authorization/bearer header observed at all is how a real oauth
	// or oauth-client registration reads back. Never gated on opts.Auth,
	// and never on "would-write" alone (that is decided one layer up, by
	// Compare/apply.go) — AuthBearer and AuthForeign are NOT this shape
	// (Pitfall 4).
	consequence := ""
	if auth == AuthNone {
		consequence = claudeCodeOAuthReLoginNote
	}

	return Observation{
		URL:                url,
		Auth:               auth,
		BearerForm:         claudeCodeBearerForm,
		Headers:            headers,
		Unrecognized:       unrecognized,
		WholeEntryNote:     claudeCodeWholeEntryNote,
		ManualRemediation:  claudeCodeManualRemediation,
		RewriteConsequence: consequence,
	}, true
}
