// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"encoding/json"
	"fmt"
)

// genericRuntime implements Runtime for an MCP client engram has no
// native support for: it authors a portable, pasteable server
// configuration document instead of a CLI invocation, structurally
// identical in shape to claudeCodeRuntime/codexRuntime/openCodeRuntime —
// no special-casing outside this file.
type genericRuntime struct{}

// Generic is the Runtime registered for the "generic" pseudo-runtime
// (03-04): an opt-in, no-binary target that prints a portable MCP server
// configuration an operator can paste into a client engram does not
// natively support.
var Generic Runtime = genericRuntime{}

func (genericRuntime) Name() string { return "generic" }

// OptInOnly implements optInOnlyRuntime (runtime.go): Select(nil) never
// includes generic in a bare `engram setup`'s default selection. This is
// how D-14's stated PURPOSE — a bare invocation must not claim presence
// for, or inflate the denominator with, a runtime that is not a real
// thing installed on the machine — is achieved. D-14's own literal
// phrasing ("its Detect() reports not-present unless it was explicitly
// named") could NOT be implemented as written: Runtime.Detect(env
// Environment) bool carries no signal about whether the runtime was
// named on --runtime, and adding one would either change that method's
// signature for every registered runtime or require a by-name check
// outside this file — both forbidden by this package's no-special-casing
// constraint (runtime.go). The default-set predicate below achieves the
// same operator-visible outcome by construction instead.
func (genericRuntime) OptInOnly() bool { return true }

// Detect always reports true — deliberately, and unconditionally. This is
// the stretch of Detect's ordinary meaning D-14 names, and it lives here,
// in generic's own file, rather than being special-cased anywhere else.
// A pseudo-runtime has no binary of its own: "is it present on this
// machine" has no machine-dependent answer to give, because generic does
// not correspond to any installed software this method could probe for.
// The portable config it produces can always be produced, on any
// machine, regardless of what is or is not on PATH. Detect's answer alone
// would therefore make generic look "present" on every machine if it
// were ever reachable through Select(nil)'s default set — OptInOnly
// above is what actually prevents that, not this method.
func (genericRuntime) Detect(Environment) bool { return true }

// genericMCPServer is the one entry generic's config document carries,
// keyed "engram" in genericConfigDoc.MCPServers. Type is always "http";
// Headers is nil (omitted from the marshaled document) for every mode
// except bearer.
type genericMCPServer struct {
	Type    string            `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// genericConfigDoc is the top-level shape json.Marshal produces for
// Plan.Config: a single "mcpServers" key whose value is an object keyed
// by server name. This is the "mcpServers"-keyed form captured directly
// from a real .mcp.json written by `claude mcp add` — the de facto
// portable convention this milestone's research confirmed Cursor also
// expects (03-RESEARCH.md § "Architecture Patterns" Pattern 3).
type genericConfigDoc struct {
	MCPServers map[string]genericMCPServer `json:"mcpServers"`
}

// Plan authors generic's one deliverable: a minified, single-line
// portable configuration document, never a CLI invocation. Every
// returned Plan carries zero Actions and a nil Probe (D-16, prohibited
// from ever authoring either) — the config text IS the whole output,
// carried on Plan.Config. The shared executor (apply.go) treats a
// zero-Action Plan as a general "no write to perform" case, never
// special-cased on this runtime's name: it copies Config onto
// Result.Config, skips env.LookPath and env.Run entirely, and classifies
// OutcomeWouldWrite in BOTH the preview and the --apply lane, because
// there is no state to converge and no second read to compare against
// (D-16). OutcomeAlreadyCorrect is defined in plan.go as a claim about
// OBSERVED existing state; generic observes nothing, so it can never
// legitimately produce that value, and would-write already counts as a
// non-failed attempt under Classify (exit.go), so nothing about the
// exhaustive exit-code combination table moves.
//
// opts.URL is placed into the "url" value byte-for-byte, never appended
// to or stripped (D-02). "none" reuses the same document as "oauth":
// there is no separate no-auth shape to author. "oauth-client" has no
// authorable form on a portable JSON document — a pre-registered OAuth
// client needs an interactive credential prompt no static config file can
// express — so it returns an error wrapping ErrAuthModeUnsupported,
// naming both the runtime and the mode, exactly like every other
// runtime's unsupported cell.
//
// Bearer mode's header value follows one of two forms, chosen on whether
// the caller supplied a --token-file: with none, it names ENGRAM_TOKEN
// via a shell-style "${ENGRAM_TOKEN}" reference — matching the
// convention claude-code/opencode/codex each independently established
// for themselves in D-05/D-06 — because that is the one substitution
// syntax the shell itself is guaranteed to expand regardless of which
// unsupported client the operator ultimately pastes this into. With a
// --token-file supplied, generic instead falls back to
// bearerProvenance's literal, non-secret path-provenance placeholder
// ("<from PATH>"): generic has no CLI of its own that could resolve a
// substitution token at connect time on an arbitrary third-party client,
// so it cannot promise the operator that a "${...}" reference will ever
// be expanded there — showing the file's own path is the honest
// alternative. generic is therefore the ONLY remaining caller of
// bearerProvenance (D-06); every native runtime moved off it in this
// phase's earlier waves (03-02, 03-03). Neither form ever carries a
// credential VALUE — only a variable reference or a path.
//
// Every returned Plan also authors Skills with the explicit
// SkillFormatNone value (Phase 4, D-11): generic carries the curation
// skills in its DELIVERABLE, exactly as it already carries the portable
// server configuration above, never on the filesystem — it has no
// machine of its own and therefore no destination to derive, so Dir and
// IndexFile stay their empty-string zero value rather than being
// authored from env.HomeDir(). SkillFormatNone is a REAL, explicit value
// here, never an unset field — the same explicit-value-never-absence
// discipline plan.go's SkillFormat doc comment states for the type as a
// whole. Generic's aggregated outcome stays would-write in both the
// preview and the --apply lane for the same reason it already does for
// registration (see this method's own top-level doc comment): there is
// no state to converge and no second read to compare against, so the
// already-correct value can never legitimately be produced. D-11 is what
// makes this the one class of operator — an MCP client engram has no
// native support for — that still gets the curation guidance from
// somewhere: `engram setup --runtime generic --output json` is the
// documented manual path, and a dead end for the guidance would still be
// a dead end for exactly that operator.
func (genericRuntime) Plan(_ Environment, opts Options) (Plan, error) {
	switch opts.Auth {
	case "oauth", "none", "bearer":
		server := genericMCPServer{Type: "http", URL: opts.URL}
		if opts.Auth == "bearer" {
			headerValue := "Bearer ${ENGRAM_TOKEN}"
			if opts.TokenFile != "" {
				headerValue = "Bearer " + bearerProvenance(opts.TokenFile)
			}
			server.Headers = map[string]string{"Authorization": headerValue}
		}
		doc := genericConfigDoc{MCPServers: map[string]genericMCPServer{"engram": server}}
		b, err := json.Marshal(doc)
		if err != nil {
			// doc is a small, fixed, fully-controlled shape (two known
			// string fields and an optional single-entry map of strings);
			// json.Marshal cannot fail on it in practice. Surfaced as an
			// OutcomeFailed row rather than a panic, on the same footing
			// as every other runtime's Plan error path.
			return Plan{}, fmt.Errorf("generic: marshal portable config: %w", err)
		}
		return Plan{Runtime: "generic", Config: string(b), Skills: SkillTarget{Format: SkillFormatNone}}, nil
	default:
		return Plan{}, fmt.Errorf("generic: auth mode %q: %w", opts.Auth, ErrAuthModeUnsupported)
	}
}
