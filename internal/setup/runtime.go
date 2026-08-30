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

// ErrApplyNotImplemented is returned by every mutating apply path this
// phase: actually executing the authored invocation against the real
// machine is Phase 3's work (D-09) — Phase 2 ships only Detect and Plan.
var ErrApplyNotImplemented = errors.New("setup: --apply is not implemented yet (Phase 3)")

// Options carries the resolved, caller-supplied values a Runtime's Plan
// needs to author its invocation: the MCP endpoint URL (passed through
// byte-for-byte, D-02 — never appended to, stripped, or normalized), the
// selected auth mode, and the path to a bearer token file. TokenFile's
// PATH is the whole payload at this layer: Plan must never open, stat, or
// read it (D-16) — only its path is previewed, as
// "Bearer <from /path/to/token>".
type Options struct {
	URL       string
	Auth      string
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
var Runtimes = []Runtime{ClaudeCode, Codex, OpenCode}

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
// runtime, in registry order (D-10: a bare `engram setup` targets every
// detected runtime). A name matching no registered runtime's Name() is a
// usage error naming the offending value and every valid name (D-11) — a
// VALID name whose runtime is simply absent from the machine is NOT an
// error here; that distinction belongs to Detect(), reported as
// OutcomeNotPresent by the caller.
func Select(names []string) ([]Runtime, error) {
	if len(names) == 0 {
		return Runtimes, nil
	}
	byName := make(map[string]Runtime, len(Runtimes))
	for _, rt := range Runtimes {
		byName[rt.Name()] = rt
	}
	out := make([]Runtime, 0, len(names))
	for _, name := range names {
		rt, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("unknown runtime %q: valid runtimes are %s", name, strings.Join(Names(), ", "))
		}
		out = append(out, rt)
	}
	return out, nil
}
