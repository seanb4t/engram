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

import "fmt"

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

// Action is one authored, ready-to-issue invocation a Plan carries.
// Command is the exact, fully-formed command line engram would run (or
// preview) — credential material appears only by provenance (D-16:
// "Bearer <from /path/to/token>"), never by value. Description is a short
// human-facing label for the action.
type Action struct {
	Command     string
	Description string
}

// Plan is one runtime's set of authored actions for a given set of
// Options. Runtime is the runtime's own Name(). Phase 2 ships exactly one
// Action per Plan (the single `<tool> mcp add` invocation); Actions is a
// slice so a later phase (e.g. skills distribution) can grow it without a
// type change.
type Plan struct {
	Runtime string
	Actions []Action
}

// Result is one runtime's reported outcome — the shape cmd/engram/setup.go
// renders into its report doc. Present mirrors Detect()'s answer; Outcome
// is the classified result; Command is the exact invocation from the
// runtime's Plan (empty when Outcome is OutcomeNotPresent); Reason carries
// a human-readable explanation when Outcome is OutcomeFailed (e.g. an
// unsupported auth mode naming both the runtime and the mode).
type Result struct {
	Runtime string
	Present bool
	Outcome Outcome
	Command string
	Reason  string
}

// bearerProvenance renders the literal, non-secret provenance form of a
// bearer credential (D-16): "<from PATH>" when tokenFile is set, or
// "<from ENGRAM_TOKEN>" when it is empty — the env-var fallback
// resolveToken already implements binary-wide. Every Runtime's Plan that
// authors a `--header "Authorization: Bearer <credential>"` form (Task 3)
// substitutes <credential> with this string, and MUST NOT open, stat, or
// read tokenFile: the path is the whole payload at this layer, so a
// nonexistent path still previews successfully. A fixed "Bearer ***" mask
// was rejected — on a machine with several token files, WHICH credential
// would be used is precisely the detail worth previewing.
func bearerProvenance(tokenFile string) string {
	if tokenFile == "" {
		return "<from ENGRAM_TOKEN>"
	}
	return fmt.Sprintf("<from %s>", tokenFile)
}
