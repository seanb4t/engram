// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrAuthModeUnsupported is returned by a Runtime's Plan when the
// requested auth mode has no authorable invocation on that runtime's own
// `mcp add`-equivalent CLI surface (e.g. opencode + oauth-client, Task 3
// of this phase).
var ErrAuthModeUnsupported = errors.New("setup: auth mode is not supported by this runtime")

// ErrHeaderUnsupported is returned by a Runtime's Plan when
// opts.Headers is non-empty and that runtime's own `mcp add` CLI has no
// custom-header flag (codex, D-09). It is deliberately distinct from
// ErrAuthModeUnsupported because a custom header is orthogonal to auth
// mode (02-CONTEXT.md's framing correction: an ADDITIONAL header that
// rides alongside whatever --auth produces, never a rename of it) — a
// caller (Phase 4 drift comparison, docs) must be able to tell a header
// gap from an auth-mode gap (D-10).
var ErrHeaderUnsupported = errors.New("setup: custom header is not supported by this runtime")

// HeaderSpec names one additional HTTP header a registration should
// carry alongside whatever Auth produces. Name is an RFC 7230 token
// (validated at the CLI boundary, cmd/engram/setup.go's setupResolve,
// plan 02-03 — this package carries already-validated data). EnvVar is
// the NAME of an environment variable the runtime itself resolves at its
// own connect time — it is NEVER dereferenced anywhere in this package
// (no environment read of a header variable exists here; D-04), so no
// code path in internal/setup can place a header VALUE on argv, in
// Config, in preview text, or in a log.
type HeaderSpec struct {
	Name   string
	EnvVar string
}

// Options carries the resolved, caller-supplied values a Runtime's Plan
// needs to author its invocation: the MCP endpoint URL (passed through
// byte-for-byte, D-02 — never appended to, stripped, or normalized), the
// selected auth mode, the non-secret opaque OAuth client ID, and the path to a bearer token file. TokenFile's
// PATH is the whole payload at this layer: Plan must never open, stat, or
// read it (D-16) — only its path is previewed, as
// "Bearer <from /path/to/token>".
//
// Headers carries zero or more ADDITIONAL headers that ride alongside
// whatever Auth produces — orthogonal to it, never a substitute for it
// (D-01). ENGRAM_TOKEN remains bearer's own fixed variable; each extra
// header names its own EnvVar (D-06).
//
// Headers is expected to arrive already validated by the CLI boundary
// (cmd/engram/setup.go's setupParseHeaders, plan 02-03, 02-RESEARCH.md
// Pitfall 5): no Authorization collision, no case-insensitive duplicate
// names, and every EnvVar a POSIX identifier. This package orders and
// renders the headers it is given deterministically (sortedHeaders below)
// but does NOT re-validate them — a direct caller of this package that
// skips that validation owns the consequences (a colliding Authorization
// header or duplicate names surviving into a rendered invocation). This is
// a deliberate boundary, not an oversight: 02-RESEARCH.md Pitfall 5
// requires header validation to live ONCE, at the CLI boundary, rather
// than duplicated per-runtime inside this package.
type Options struct {
	URL       string
	Auth      string
	ClientID  string
	TokenFile string
	Headers   []HeaderSpec
}

// Runtime is one agent runtime engram knows how to detect and plan a
// registration for. Every implementation is reached only through this
// interface — no code path outside a runtime's own implementation file may
// special-case it by name (the assumption-delta "promote" decision,
// 02-CONTEXT.md: claude-code is structurally indistinguishable from codex
// and opencode).
type Runtime interface {
	// Name returns the runtime's own stable identifier (e.g.
	// "claude-code"), used as both the Plan/Result.Runtime value and a
	// --runtime selector value.
	Name() string
	// Detect reports whether this runtime's own binary is present on the
	// machine, consulting env exclusively (D-12): exec.LookPath on the
	// runtime's binary name, and nothing else — never a config-directory
	// stat, so a leftover directory from an uninstalled runtime cannot
	// report as installed.
	Detect(env Environment) bool
	// Plan authors the exact invocation this runtime would issue for
	// opts, without executing it. Returns an error satisfying
	// errors.Is(err, ErrAuthModeUnsupported) when opts.Auth has no
	// authorable form on this runtime, or errors.Is(err,
	// ErrHeaderUnsupported) when opts.Headers is non-empty and this
	// runtime's own CLI has no custom-header flag (D-10).
	Plan(env Environment, opts Options) (Plan, error)
}

// Runtimes is the package-level registry of every runtime engram knows
// about, in the order Names() and every report list them — the report's
// row order and the order Names() returns, deliberately stable so a
// reader can rely on it. Declared as a literal at package scope —
// mirroring internal/migrate.Registry's discipline
// (internal/migrate/registry.go) — never built inside an init() or behind
// a lazy getter: a runtime hidden behind indirection is a worse failure
// than a compile-time-visible list.
//
// Generic (03-04) is deliberately LAST: it is the one entry that opts
// itself out of the default (no-`--runtime`) selection (see
// optInOnlyRuntime below and Select's doc comment), and appending it
// preserves the three native runtimes' existing relative order rather
// than renumbering them.
var Runtimes = []Runtime{ClaudeCode, Codex, OpenCode, Generic}

// optInOnlyRuntime is an OPTIONAL interface a Runtime may implement to
// declare that it must never be included in Select(nil)'s default (no
// `--runtime`) selection — a structural predicate the runtime states
// about ITSELF, consumed once here, rather than a by-name exclusion
// anywhere in this file (the same structural-predicates-over-enumerations
// shape cmd/engram/cmdwalk.go's operatorCommands() already uses). Only
// the generic pseudo-runtime (generic.go, D-14) implements this today; no
// native runtime does, and none is expected to.
type optInOnlyRuntime interface {
	// OptInOnly reports true when this runtime must be explicitly named
	// via --runtime to ever be selected — Select(nil) skips it entirely.
	OptInOnly() bool
}

// Names returns every registered runtime's Name(), in registry order.
func Names() []string {
	names := make([]string, len(Runtimes))
	for i, rt := range Runtimes {
		names[i] = rt.Name()
	}
	return names
}

// Select resolves names (typically --runtime's value) to their matching
// entries in Runtimes. A nil or empty names returns every registered
// runtime EXCEPT one that declares itself optInOnlyRuntime (D-14), in
// registry order (D-10: a bare `engram setup` targets every detected
// NATIVE runtime — the generic pseudo-runtime never claims presence or
// inflates that report's denominator unless the caller named it
// explicitly). A name matching no registered runtime's Name() is a
// usage error naming the offending value and every valid name (D-11) — a
// VALID name whose runtime is simply absent from the machine is NOT an
// error here; that distinction belongs to Detect(), reported as
// OutcomeNotPresent by the caller.
//
// A repeated name is deduplicated to its FIRST occurrence rather than
// rejected (WR-02): `--runtime a,a` and `--runtime a --runtime a` both
// plainly mean "target a", so producing two identical report rows and
// inflating setupApplySummary's denominator would be a second, unrequested
// behavior change layered on an unambiguous invocation. The dedup preserves
// the caller's stated order — it does not re-sort into registry order. The
// unknown-name check still runs for every element before any dedup
// decision, so a repeated unknown name still errors.
//
// The explicit-names branch below is UNCHANGED by D-14: `--runtime
// generic` still resolves exactly as before — optInOnlyRuntime only ever
// gates the empty-names (default-set) branch.
func Select(names []string) ([]Runtime, error) {
	if len(names) == 0 {
		out := make([]Runtime, 0, len(Runtimes))
		for _, rt := range Runtimes {
			if oi, ok := rt.(optInOnlyRuntime); ok && oi.OptInOnly() {
				continue
			}
			out = append(out, rt)
		}
		return out, nil
	}
	byName := make(map[string]Runtime, len(Runtimes))
	for _, rt := range Runtimes {
		byName[rt.Name()] = rt
	}
	seen := make(map[string]bool, len(names))
	out := make([]Runtime, 0, len(names))
	for _, name := range names {
		rt, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("unknown runtime %q: valid runtimes are %s", name, strings.Join(Names(), ", "))
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, rt)
	}
	return out, nil
}

// sortedHeaders returns hs sorted by strings.ToLower(Name) ascending, with
// ties (names equal under case folding) broken by an exact byte-wise
// strings.Compare(a.Name, b.Name) — a TOTAL order, so the result never
// depends on sort stability. This is defense-in-depth for the WR-01 gap:
// Options.Headers is expected to already be free of case-insensitive
// duplicate names (see Options' own doc comment), but a direct package
// caller that skips CLI-boundary validation could hand this function two
// case-colliding names, and D-08's "deterministic ordering" guarantee must
// hold even then. The result is a CLONE — the caller's slice is never
// re-ordered in place (D-08). It returns nil for a nil or empty hs, so
// append(args, claudeCodeHeaderArgs(sortedHeaders(nil))...) is a no-op and
// every no-header Args slice stays byte-identical.
//
// This function ORDERS but never FORMATS: no runtime dialect string is
// authored here, because each runtime authors its own ("--header
// NAME: ${ENVVAR}" for claude-code, "NAME={env:ENVVAR}" for opencode,
// "NAME": "${ENVVAR}" for generic) — a shared cross-runtime formatter is
// exactly the opencode colon-space regression this package must not
// repeat. The auth-mode header (if any) is authored first by each
// runtime's own case arm; sortedHeaders orders only the EXTRA headers
// that follow it.
func sortedHeaders(hs []HeaderSpec) []HeaderSpec {
	if len(hs) == 0 {
		return nil
	}
	out := slices.Clone(hs)
	slices.SortFunc(out, func(a, b HeaderSpec) int {
		if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out
}
