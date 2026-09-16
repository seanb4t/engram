// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"
)

// codexRuntime implements Runtime for Codex, authoring the live-verified
// `codex mcp add <NAME> --url <URL>` invocation surface
// (.planning/research/SUMMARY.md § "Post-Synthesis Live Verification") —
// structurally identical to claude-code's own runtime implementation, no
// special-casing.
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
				Args:        []string{"codex", "mcp", "add", "engram", "--url", opts.URL, "--bearer-token-env-var", codexBearerForm},
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

// codexBearerForm is the ONE bearer-token environment-variable name
// codex's Plan ever authors (its own --bearer-token-env-var arm above
// references this same constant, so the two cannot drift) and the value
// Observe compares an observed bearer_token_env_var against to classify
// AuthBearer vs. AuthForeign (drift.go).
const codexBearerForm = "ENGRAM_TOKEN"

// codexWholeEntryNote is REQ-drift-preserved-outcome's fixed, composed-
// once sentence naming codex's whole-entry semantics (03-RESEARCH.md
// Pitfall 10 / 04-RESEARCH.md Pitfall 5): `codex mcp add` has no partial-
// merge form — it either overwrites the whole entry or leaves it
// untouched — so a preserved row's Reason must state that risk plainly,
// not merely name the differing facet.
const codexWholeEntryNote = "codex mcp add replaces the whole entry: --apply would overwrite it or leave it untouched, never merge into it"

// codexRegistrationTransport mirrors the "transport" object of
// `codex mcp get <name> --json`'s live-verified shape (04-RESEARCH.md
// Code Examples, codex-cli 0.153.4). BearerTokenEnvVar is a pointer so a
// JSON null (no bearer configured) is distinguishable from an empty
// string; HTTPHeaders/EnvHTTPHeaders's map[string]string typing is
// CONFIRMED OBSERVED (04-05-PLAN.md Task 2,
// .planning/phases/04-drift-detection-read-only/04-OBSERVATIONS.md
// §"Codex — literal header (hand-edited)", codex-cli 0.154.0,
// 2026-09-15): a hand-edited literal header value under `http_headers`
// round-tripped through `codex mcp get --json` as an object of strings,
// coexisting with a populated `bearer_token_env_var` on the same entry —
// confirming 04-RESEARCH.md's original field-naming-based guess for this
// exact shape. HTTPHeadersHelper/EnabledTools/
// DisabledTools/StartupTimeoutSec/ToolTimeoutSec on the sibling doc struct
// are json.RawMessage so their null-ness can be tested (isNullRaw)
// without needing to know their real shape, which D-11's totality parse
// does not require.
type codexRegistrationTransport struct {
	Type              string            `json:"type"`
	URL               string            `json:"url"`
	BearerTokenEnvVar *string           `json:"bearer_token_env_var"`
	HTTPHeaders       map[string]string `json:"http_headers"`
	EnvHTTPHeaders    map[string]string `json:"env_http_headers"`
	HTTPHeadersHelper json.RawMessage   `json:"http_headers_helper"`
}

// codexRegistrationDoc mirrors the top-level object of
// `codex mcp get <name> --json`'s live-verified shape (04-RESEARCH.md
// Code Examples). Enabled is a pointer so JSON false is distinguishable
// from an absent key; every other field engram never sets is
// json.RawMessage purely to test null-ness (isNullRaw), never to parse
// its content — a value there is, by definition, something D-11's
// totality parse must report as unrecognized, not decode.
type codexRegistrationDoc struct {
	Name              string                     `json:"name"`
	Enabled           *bool                      `json:"enabled"`
	DisabledReason    json.RawMessage            `json:"disabled_reason"`
	Transport         codexRegistrationTransport `json:"transport"`
	EnabledTools      json.RawMessage            `json:"enabled_tools"`
	DisabledTools     json.RawMessage            `json:"disabled_tools"`
	StartupTimeoutSec json.RawMessage            `json:"startup_timeout_sec"`
	ToolTimeoutSec    json.RawMessage            `json:"tool_timeout_sec"`
}

// codexKnownTopKeys and codexKnownTransportKeys list every JSON key
// codexRegistrationDoc/codexRegistrationTransport's own tags declare —
// the key-set diff in Observe below consults these SAME lists (never a
// second, independently hand-typed list) to name an unrecognized
// top-level or transport-nested key (D-11).
var codexKnownTopKeys = []string{
	"name", "enabled", "disabled_reason", "transport",
	"enabled_tools", "disabled_tools", "startup_timeout_sec", "tool_timeout_sec",
}

var codexKnownTransportKeys = []string{
	"type", "url", "bearer_token_env_var", "http_headers", "env_http_headers", "http_headers_helper",
}

// isNullRaw reports whether raw is JSON's absence or its null literal —
// the ONLY test D-11's field rules apply to codexRegistrationDoc's
// json.RawMessage fields, never a parse of their actual content.
func isNullRaw(raw json.RawMessage) bool {
	return len(raw) == 0 || string(raw) == "null"
}

// unrecognizedLabelBound is the byte bound Observe applies to every
// unrecognized label before it is quoted for rendering — the same
// discipline apply.go's boundCapture applies to a full third-party
// capture, scaled down for a short field-name label.
const unrecognizedLabelBound = 40

// boundLabel truncates s to at most unrecognizedLabelBound bytes,
// scanning back to the last valid rune boundary so the result is always
// valid UTF-8 — mirroring apply.go's boundCapture, without its
// truncation marker (a label is a field name or key, not prose a
// human needs told was cut).
func boundLabel(s string) string {
	if len(s) <= unrecognizedLabelBound {
		return s
	}
	limit := unrecognizedLabelBound
	for limit > 0 && !utf8.RuneStart(s[limit]) {
		limit--
	}
	return s[:limit]
}

// Observe implements DriftRuntime for codex: a totality parse of
// `codex mcp get <name> --json`'s combined stdout+stderr (04-CONTEXT.md
// D-11), gated by json.Decoder.DisallowUnknownFields on
// codexRegistrationDoc — the natural stdlib fit confirmed against
// 04-RESEARCH.md's Code Examples. AUTHORED HERE: codex's own JSON shape
// is parsed only in this file, never through a shared cross-runtime
// parser (04-RESEARCH.md Pitfall 3).
//
// ok is false when probeOutput cannot be framed as a registration at
// all: empty/whitespace-only body, a JSON syntax error, an unexpected-EOF
// body, or a decoded document whose Name is not "engram" (D-09). Any
// OTHER shape defect — an unrecognized top-level or transport-nested key,
// a non-null field this runtime never sets, a transport type other than
// "streamable_http", enabled other than true, or a strict-decode error
// the tolerant pass and the key-set diff both missed — is reported on
// Observation.Unrecognized instead (D-11): cautious, never blind.
//
// opts is consulted only to build the planned side of the header
// comparison (joinHeaders) — codex declines every custom header up front
// in Plan (D-09/D-10 of 02-CONTEXT.md), so opts.Headers is always empty
// on every reachable call; this method still builds the planned side
// uniformly, never special-cased on that emptiness.
func (codexRuntime) Observe(probeOutput string, opts Options) (Observation, bool) {
	if strings.TrimSpace(probeOutput) == "" {
		return Observation{}, false
	}

	var doc codexRegistrationDoc
	var unrecognized []string
	// typeErrField records which field (if any) a *json.UnmarshalTypeError
	// already named below, so the field-rules block (e) — which
	// independently re-derives the same finding from that field's now-
	// zero Go value (Go's Unmarshal leaves a type-mismatched field at its
	// zero value and decodes the rest) — never reports it a second time
	// (WR-02 of 04-REVIEW.md).
	var typeErrField string
	if err := json.Unmarshal([]byte(probeOutput), &doc); err != nil {
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		switch {
		case errors.As(err, &syntaxErr):
			return Observation{}, false
		case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
			return Observation{}, false
		case errors.As(err, &typeErr):
			if typeErr.Field != "" {
				unrecognized = append(unrecognized, typeErr.Field)
				typeErrField = typeErr.Field
			}
		default:
			return Observation{}, false
		}
	}
	if doc.Name != "engram" {
		return Observation{}, false
	}

	// Key-set diff (c): every top-level and transport-nested key absent
	// from the known lists is unrecognized, sorted for a deterministic
	// rendering order.
	var rawTop map[string]json.RawMessage
	if err := json.Unmarshal([]byte(probeOutput), &rawTop); err == nil {
		var keyDiff []string
		for k := range rawTop {
			if !slices.Contains(codexKnownTopKeys, k) {
				keyDiff = append(keyDiff, k)
			}
		}
		if transportRaw, ok := rawTop["transport"]; ok {
			var rawTransport map[string]json.RawMessage
			if err := json.Unmarshal(transportRaw, &rawTransport); err == nil {
				for k := range rawTransport {
					if !slices.Contains(codexKnownTransportKeys, k) {
						keyDiff = append(keyDiff, "transport."+k)
					}
				}
			}
		}
		slices.Sort(keyDiff)
		unrecognized = append(unrecognized, keyDiff...)
	}

	// Totality gate (d): a strict, DisallowUnknownFields decode is the
	// structural safety net for whatever the tolerant pass and the
	// key-set diff above both missed (e.g. a level neither inspects). It
	// contributes the fixed token "unrecognized field" ONLY when nothing
	// else already found something — never a second, redundant entry.
	dec := json.NewDecoder(strings.NewReader(probeOutput))
	dec.DisallowUnknownFields()
	var strictDoc codexRegistrationDoc
	if err := dec.Decode(&strictDoc); err != nil && len(unrecognized) == 0 {
		unrecognized = append(unrecognized, "unrecognized field")
	}

	// Field rules (e): every non-null field engram never sets, plus
	// enabled != true and a transport type other than streamable_http,
	// is unrecognized content (D-11).
	if typeErrField != "enabled" && (doc.Enabled == nil || !*doc.Enabled) {
		unrecognized = append(unrecognized, "enabled")
	}
	if !isNullRaw(doc.DisabledReason) {
		unrecognized = append(unrecognized, "disabled_reason")
	}
	if typeErrField != "transport.type" && doc.Transport.Type != "streamable_http" {
		unrecognized = append(unrecognized, "transport.type")
	}
	if !isNullRaw(doc.Transport.HTTPHeadersHelper) {
		unrecognized = append(unrecognized, "transport.http_headers_helper")
	}
	if !isNullRaw(doc.EnabledTools) {
		unrecognized = append(unrecognized, "enabled_tools")
	}
	if !isNullRaw(doc.DisabledTools) {
		unrecognized = append(unrecognized, "disabled_tools")
	}
	if !isNullRaw(doc.StartupTimeoutSec) {
		unrecognized = append(unrecognized, "startup_timeout_sec")
	}
	if !isNullRaw(doc.ToolTimeoutSec) {
		unrecognized = append(unrecognized, "tool_timeout_sec")
	}

	// Auth (f): the observed value is compared for equality ONLY, inside
	// this call frame — it is never assigned to Observation or any field
	// that survives past this point (D-02).
	auth := AuthNone
	if v := doc.Transport.BearerTokenEnvVar; v != nil {
		if *v == codexBearerForm {
			auth = AuthBearer
		} else {
			auth = AuthForeign
		}
	}

	// Headers (g): codex never authors an extra header (D-09/D-10 of
	// 02-CONTEXT.md gates opts.Headers to always-empty on every
	// reachable call), so planned is built uniformly and is empty in
	// practice — never special-cased on that emptiness.
	var observedHeaders []rawHeader
	for name, value := range doc.Transport.HTTPHeaders {
		observedHeaders = append(observedHeaders, rawHeader{Name: name, Value: value})
	}
	for name, value := range doc.Transport.EnvHTTPHeaders {
		observedHeaders = append(observedHeaders, rawHeader{Name: name, Value: value})
	}
	var planned []plannedHeader
	for _, h := range sortedHeaders(opts.Headers) {
		planned = append(planned, plannedHeader{Name: h.Name, Value: h.EnvVar})
	}
	headers := joinHeaders(observedHeaders, planned)

	// Unrecognized entries (h): bounded to 40 bytes at a rune boundary,
	// then rendered through the SAME paste-safety quoter every other
	// observed name/label crosses this boundary through.
	for i, u := range unrecognized {
		unrecognized[i] = quoteWord(boundLabel(u))
	}

	return Observation{
		URL:            doc.Transport.URL,
		Auth:           auth,
		BearerForm:     codexBearerForm,
		Headers:        headers,
		Unrecognized:   unrecognized,
		WholeEntryNote: codexWholeEntryNote,
	}, true
}
