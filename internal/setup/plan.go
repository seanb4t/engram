// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package setup detects which supported agent runtimes are present on the
// machine and authors the exact command each would need to register
// engram as an MCP server — before anything is written. It is the backing
// package for `engram setup` (cmd/engram/setup.go): the Runtime interface,
// the injectable Environment seam, and the package-level Runtimes registry
// carrying real Detect() and real Plan() for every supported runtime.
//
// The claude mcp add / codex mcp add / opencode mcp add invocation strings
// each Runtime's Plan authors are AUTHORED HERE and nowhere else (D-09,
// 02-CONTEXT.md): a later phase that executes them (Apply, Phase 3) or
// generates prose from them (Phase 5) must consume these strings, never
// re-derive them — re-deriving them would create the two-encodings drift
// this milestone exists to prevent.
package setup

import (
	"fmt"
	"strings"
)

// Outcome classifies the result of planning (or, once Apply lands,
// actually performing) one runtime's registration. It is a five-value
// enum, string-backed for readable JSON/text rendering. OutcomeNotPresent
// is a REAL, explicit value — never modeled as an absence or the zero
// value (D-07): a runtime that is simply not installed is an expected
// outcome, never a failure. A zero-valued Outcome ("") is a programming
// error, never meaningful data — every code path that produces a Result
// must set one of the five constants below.
type Outcome string

const (
	// OutcomeNotPresent means the runtime's own binary could not be
	// resolved on PATH (Detect returned false). Not a failure (D-07): a
	// machine with only some runtimes installed still exits 0.
	OutcomeNotPresent Outcome = "not-present"
	// OutcomeWouldWrite means the runtime is present and Plan produced an
	// invocation, but --apply was not requested: a preview result.
	OutcomeWouldWrite Outcome = "would-write"
	// OutcomeAlreadyCorrect means the runtime is present and its existing
	// registration already matches what Plan would produce. Reserved for
	// Phase 3's Apply, which alone can observe existing state; Phase 2
	// never produces this value.
	OutcomeAlreadyCorrect Outcome = "already-correct"
	// OutcomeWrote means --apply actually performed the registration.
	// Reserved for Phase 3's Apply; Phase 2's Apply is stubbed and never
	// produces this value.
	OutcomeWrote Outcome = "wrote"
	// OutcomeFailed means an attempt to plan or apply the registration
	// failed — e.g. Plan returned ErrAuthModeUnsupported for the
	// requested (runtime, auth mode) pair.
	OutcomeFailed Outcome = "failed"
)

// SkillFormat classifies a SkillTarget's write shape. It mirrors
// internal/skills.Format's three explicit values — a "no filesystem
// destination" case, a "native" case, and an "agents-md" case — as a
// DELIBERATELY SEPARATE type, never a shared alias: the leaf-purity gate
// forbids either package importing the other (D-05), so this package
// cannot reference internal/skills.Format directly, and cmd/engram owns
// the one explicit mapping between the two (setupSkillsTarget). Every
// value is a REAL, explicit value, never modeled as an absence or the
// zero value — the same discipline Outcome's own doc comment states
// above.
type SkillFormat string

const (
	// SkillFormatNone means this runtime has no filesystem destination
	// for skills at all (D-11's "no skills" case).
	SkillFormatNone SkillFormat = "none"
	// SkillFormatNative means Dir is the runtime's own native skill
	// directory.
	SkillFormatNative SkillFormat = "native"
	// SkillFormatAgentsMD means the runtime falls back to a delimited
	// block in an AGENTS.md-shaped file at IndexFile (D-13/D-15/D-16).
	SkillFormatAgentsMD SkillFormat = "agents-md"
)

// SkillTarget is one runtime's authored skills-install destination:
// Format selects the write shape, Dir is the absolute destination
// directory, and IndexFile is the absolute path of the file an
// agents-md-shaped splice targets (meaningful only for
// SkillFormatAgentsMD). AUTHORED HERE — in each runtime's own Plan(),
// never a runtime-agnostic default — exactly like Probe above.
type SkillTarget struct {
	Format    SkillFormat
	Dir       string
	IndexFile string
}

// Action is one authored, ready-to-issue invocation a Plan carries. Args
// is the authored source of truth (D-01): the exact argv Apply() execs,
// Args[0] the runtime's own bare binary name (never a resolved path — see
// Result.Binary). Credential material appears only by provenance (D-16:
// "Bearer <from /path/to/token>") or by variable reference (D-05), never
// by value.
//
// Tolerant, when true, means a nonzero exit from THIS action is expected
// and must never fail the row — e.g. a "clear any prior registration" step
// that is tolerant of a "not found" exit. Tolerant is authored explicitly
// per action in Plan(), never inferred from an action's position:
// Plan.Actions is documented as growable by a later phase (skills
// distribution), so a position-derived tolerance rule would silently make
// a later, unrelated action tolerant too. This is a deliberate, reasoned
// choice of 03-RESEARCH.md Pitfall 1's option 2 over its recommended
// option 1 (Assumptions Log A4 requires this be picked explicitly, not
// left to fall out of an unexamined default).
//
// Description is a short human-facing label for the action.
type Action struct {
	Args        []string
	Tolerant    bool
	Description string
}

// Command renders Args through the D-02 minimal POSIX quoter (quoteArgs,
// quote.go): a pure function of Args, deliberately a METHOD rather than a
// settable field, so no code path can author a display string by hand
// that diverges from what Apply() actually execs (D-01). A single-action
// Plan's Command() renders byte-identical to Phase 2's hand-authored
// string for every ordinary URL; only a value carrying a shell
// metacharacter or a space renders single-quoted.
func (a Action) Command() string {
	return quoteArgs(a.Args)
}

// Plan is one runtime's set of authored actions for a given set of
// Options. Runtime is the runtime's own Name(). Actions is a slice so a
// later phase (e.g. skills distribution) can grow it without a type
// change; Phase 3 Task 1 uses more than one Action for a runtime whose
// "make state match Plan()" write cannot be expressed as a single
// idempotent call (03-RESEARCH.md Pattern 1).
//
// Probe is the per-runtime read-only verb used to observe existing
// registration state (D-08, D-09): authored in the SAME Plan() call that
// authors the write Actions, in the runtime's own file — never a
// runtime-agnostic default, and never authored anywhere outside a
// runtime's own file (the package doc comment's AUTHORED-HERE invariant
// extends to this field). Probe[0] is the bare binary name, resolved
// exactly like an Action's Args[0] at exec time (Apply.go closes over one
// resolved path per runtime and reuses it for Probe and every Action). A
// nil or empty Probe means this runtime has no probe wired yet: the
// shared executor degrades to never claiming OutcomeAlreadyCorrect for it
// (D-08's own invariant — ambiguity resolves to wrote, never to
// already-correct).
type Plan struct {
	Runtime string
	Actions []Action
	Probe   []string
	// Config is the generic pseudo-runtime's minified, single-line
	// portable MCP-server-configuration JSON document (03-04) — the
	// whole deliverable for a runtime that authors zero Actions and no
	// Probe. Authored alongside every other field in Plan(), in the
	// runtime's own file (the package doc comment's AUTHORED-HERE
	// invariant extends to this field too); the shared executor
	// (apply.go) copies it onto Result.Config unchanged, never
	// re-deriving or re-serializing it. Empty for every runtime that
	// authors at least one Action.
	Config string
	// Skills is this runtime's authored skills-install destination
	// (Phase 4, D-05), authored alongside every other field in Plan(),
	// in the runtime's own file. A runtime that has not yet been wired
	// (this phase's earlier waves) leaves this at its zero value —
	// cmd/engram's setupSkillsTarget treats that as "no skills facet for
	// this runtime" rather than a fourth SkillFormat value.
	Skills SkillTarget
}

// Display renders every Action's Command() joined by "; " — a
// single-action Plan renders byte-identical to Phase 2's single string; a
// multi-action Plan renders POSIX sequential-execution semantics, which is
// what tolerant-then-fatal action ordering actually means to a human
// reading the preview.
func (p Plan) Display() string {
	cmds := make([]string, len(p.Actions))
	for i, a := range p.Actions {
		cmds[i] = a.Command()
	}
	return strings.Join(cmds, "; ")
}

// Result is one runtime's reported outcome — the shape cmd/engram/setup.go
// renders into its report doc. Present mirrors Detect()'s answer; Outcome
// is the classified result; Command is the exact invocation from the
// runtime's Plan (empty when Outcome is OutcomeNotPresent), populated from
// Plan.Display().
//
// Reason carries a human-readable explanation when Outcome is
// OutcomeFailed. D-11's failure legibility is built from a FIXED, ORDERED
// composition (apply.go's describeFailure/describeSeamError): the
// runtime's Name(), the failing action's rendered argv display
// (Action.Command()), the numeric exit code (when one exists — a seam
// error that never produced an exit status carries the seam error text
// instead), and the bounded captured stderr. The exit code is what keeps
// an EMPTY-stderr failure legible: it is always present, so a runtime that
// fails silently still yields a non-empty, actionable Reason. Nothing that
// builds Reason ever branches on stderr's CONTENT to decide an Outcome
// (the typed-cause-never-message-text discipline cmd/engram/operror.go's
// classifyOperatorErr already follows) — stderr is carried here as DATA,
// never string-matched.
//
// Binary (D-04) is the LookPath-resolved absolute path Apply() actually
// executed — recorded even though Args[0] (and therefore Command) stays
// the bare runtime name, so a PATH-spoofing incident leaves a trace in the
// report. Registered (D-10) is the bounded, informational capture of
// Plan.Probe's output; TokenFile (D-07) is the "token_file=ignored"-style
// marker for a native runtime that received --token-file; Config (D-15) is
// the generic pseudo-runtime's minified portable JSON.
//
// Notes (03-RESEARCH.md Open Question 1) carries a one-line record per
// TOLERANT action in Plan.Actions — whether that action's own exit was
// zero or a tolerated nonzero — joined by "; " when more than one such
// record applies. A tolerated nonzero exit's entry names the action's own
// Description (when authored), its Command(), and its exit code, so a
// genuinely broken tolerant step stays visible in --output json even
// though it never fails the row. A tolerant action that succeeded still
// contributes its bare Description when non-empty: a tolerant action's
// Description can record a consequence that only matters if a LATER,
// non-tolerant action in the same Plan then fails (claudecode.go's
// claudeCodeRemoveAction, Phase 3 Task 2) — surfacing it here, in the
// shared executor, keeps that consequence visible on the failed row's
// Notes without this package (apply.go) ever knowing anything about
// claude-code by name.
//
// Every one of Binary/Registered/TokenFile/Config/Notes is a plain
// string — never json.RawMessage, a map, or a slice — so it can never
// bypass sanitizeViewValue's scalar-only sanitizing branch
// (cmd/engram/operator_view.go).
//
// Skills is the single NON-RENDERED field on this struct — its json tag
// EXCLUDES it from marshaling. It exists so cmd/engram composes its
// skills facet from the SAME Plan() call the shared executor already made
// (execute, apply.go) rather than planning a second time, which now
// matters because Plan() reads the home directory (D-10). Excluding it
// from marshaling is what keeps a struct-valued field from ever reaching
// the row renderer — cmd/engram maps it onto its own scalar row fields
// instead (setupSkillsTarget).
type Result struct {
	Runtime    string
	Present    bool
	Outcome    Outcome
	Command    string
	Reason     string
	Binary     string      `json:"binary,omitempty"`
	Registered string      `json:"registered,omitempty"`
	TokenFile  string      `json:"token_file,omitempty"`
	Config     string      `json:"config,omitempty"`
	Notes      string      `json:"notes,omitempty"`
	Skills     SkillTarget `json:"-"`
}

// bearerProvenance renders the literal, non-secret provenance form of a
// bearer credential (D-16): "<from PATH>" when tokenFile is set, or
// "<from ENGRAM_TOKEN>" when it is empty — the env-var fallback
// resolveToken already implements binary-wide. Originally every
// registered Runtime's bearer form substituted <credential> with this
// string; as of Phase 3 (claude-code, 03-02; opencode, 03-03) every
// NATIVE runtime instead names ENGRAM_TOKEN through its own
// runtime-native substitution mechanism (D-05/D-06). This function's one
// remaining production caller is the generic pseudo-runtime (generic.go,
// 03-04): an opt-in, no-CLI-of-its-own portable-config target that cannot
// promise a "${...}" reference will ever be expanded by whatever
// third-party client the operator ultimately pastes its config into, so
// it falls back to this literal placeholder when a --token-file is
// supplied. MUST NOT open, stat, or read tokenFile: the path is the whole
// payload at this layer, so a nonexistent path still previews
// successfully. A fixed "Bearer ***" mask was rejected — on a machine
// with several token files, WHICH credential would be used is precisely
// the detail worth previewing.
func bearerProvenance(tokenFile string) string {
	if tokenFile == "" {
		return "<from ENGRAM_TOKEN>"
	}
	return fmt.Sprintf("<from %s>", tokenFile)
}
