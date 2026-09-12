// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import (
	"errors"
	"fmt"
	"strings"
)

// ErrAuthModeUnsupported is returned by a Runtime's Plan when the
// requested auth mode has no authorable invocation on that runtime's own
// `mcp add`-equivalent CLI surface (e.g. opencode + oauth-client, Task 3
// of this phase).
var ErrAuthModeUnsupported = errors.New("setup: auth mode is not supported by this runtime")

// Options carries the resolved, caller-supplied values a Runtime's Plan
// needs to author its invocation: the MCP endpoint URL (passed through
// byte-for-byte, D-02 — never appended to, stripped, or normalized), the
// selected auth mode, the non-secret opaque OAuth client ID, and the path to a bearer token file. TokenFile's
// PATH is the whole payload at this layer: Plan must never open, stat, or
// read it (D-16) — only its path is previewed, as
// "Bearer <from /path/to/token>".
type Options struct {
	URL       string
	Auth      string
	ClientID  string
	TokenFile string
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
	// authorable form on this runtime.
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
