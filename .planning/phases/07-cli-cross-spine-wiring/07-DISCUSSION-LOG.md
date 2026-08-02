# Phase 7: CLI Cross-Spine Wiring - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-02
**Phase:** 07-cli-cross-spine-wiring
**Areas discussed:** Scope flag ergonomics, `--scope` + `--cross-spine` conflict, Provenance in text
output, Discoverability & docs

---

## Area selection

All four offered gray areas were selected. Two findings surfaced during the pre-discussion scout
reframed the phase before any question was asked:

1. **The gap is a live failure, not just an absence.** `effectiveSearchScope`
   (`internal/server/tools.go:1374-1382`) rejects `scope == ""` unless `cross_spine` is true. Because
   the CLI never sets the field, `engram search --query x` with no `--scope` does not quietly search
   one spine — it errors. The milestone audit framed this as a missing flag; it is also a broken
   default.

2. **JSON provenance is already half-wired.** `renderJSON` sets `EmitDefaultValues: true`
   (`client_common.go:264-268`), so `searched_scopes` / `scopes_truncated` already appear in every
   JSON response. Only the text lane is genuinely blind.

---

## Scope flag ergonomics

### Q1 — How should the CLI treat a missing `--scope`?

| Option | Description | Selected |
|--------|-------------|----------|
| Pure passthrough | 1:1 bool; missing `--scope` stays a server-side error. Honors Phase 3 D-04 and D-08, but the most obvious invocation keeps failing. | |
| Missing `--scope` implies cross-spine | CLI sets `cross_spine=true` when scope is empty. Fixes the default; re-introduces the inference D-04 banned, moved into the client where no test guards it. | |
| Passthrough + better error | 1:1 passthrough, plus a client-side catch that exits 2 naming `--cross-spine` as the fix. Honors D-04 and D-09/D-17. | ✓ |

**User's choice:** Passthrough + better error → **D-01**
**Notes:** Later amended under D-00 — the error's wording became a pointer back to documented
behavior rather than a self-teaching tutorial.

### Q2 — Where should the pre-flight check live?

| Option | Description | Selected |
|--------|-------------|----------|
| Shared helper | One function in `client_common.go` called by both commands, per Phase 2 D-10. | ✓ |
| Inline in each `RunE` | Two-line `if` per command, mirroring the existing empty-`--query` check. Local but duplicated. | |
| You decide | Defer to the planner. | |

**User's choice:** Shared helper → **D-02**

### Q3 — How do we keep the client guard and the server rule from drifting?

| Option | Description | Selected |
|--------|-------------|----------|
| Parity test | Assert both agree across the scope × cross-spine matrix; compiles against both, goes RED when either moves. | ✓ |
| Document as fail-fast only | A comment marking the client check as a UX shortcut, matching the server's own language. Nothing structurally catches drift. | |
| Both | Comment plus test. | |

**User's choice:** Parity test → **D-03**

---

## `--scope` + `--cross-spine` conflict

### Q4 — How should the CLI handle both flags together?

| Option | Description | Selected |
|--------|-------------|----------|
| Reject as usage error | Exit 2 before dialing; precedent is `--offset` / `--cursor-mode` at `client_list.go:86`. | (arrived at) |
| Pass through silently | Let the server apply D-02. Transparent, but the Info log never reaches the caller. | |
| Pass through + warn on stderr | Send both, warn per D-07 and the `--insecure` precedent. | |

**User's choice:** Free-text — a **general principle** rather than any listed option:

> "General principle, the flags, cli help, all of this should _read_ well and be discoverable for an
> agent or human. no surprises, no error and wait to see how it works. The goal is to NOT need to
> teach by example how to use the cli, it should be evident from it's help and the naming of it's
> operations and parameters"

**Notes:** Recorded as **D-00**, governing the whole phase rather than this area. It was flagged back
to the user that D-00 puts pressure on D-01 — an error cannot be the teaching mechanism — and D-01
was amended rather than reversed: help text teaches, the error is a backstop. A follow-up proposed
mutual exclusion as the honest reading of the principle (silent discard is the surprise D-00
forbids), with each flag's help naming the other so the relationship is discoverable from either
entry point.

**Confirmed:** mutual exclusion (**D-04**), and retroactive application of D-00 to D-01's error
wording.

---

## Provenance in text output

### Q5 — What should text mode do?

| Option | Description | Selected |
|--------|-------------|----------|
| Footer line on stdout | Coverage footer after the table, reusing `list`'s existing `total:` footer pattern. | ✓ |
| Footer + the scope list | Name the scopes rather than counting them. More informative; unbounded width, needs its own truncation rule. | |
| Stderr diagnostic | Coverage note to stderr per D-07. Keeps stdout pipe-clean, but disagrees with the JSON lane about whether it is data. | |

**User's choice:** Footer line on stdout → **D-05**

### Q6 — When should the footer appear?

| Option | Description | Selected |
|--------|-------------|----------|
| Only on cross-spine | Matches the server contract exactly; existing output stays byte-identical. | ✓ |
| Always, with a scope-confined form | Uniform, but changes existing output and states something the server never reported. | |
| You decide | Defer to the planner. | |

**User's choice:** Only on cross-spine → **D-06**

---

## Discoverability & docs

### Q7 — Which surfaces must carry the guidance? *(multi-select)*

| Option | Description | Selected |
|--------|-------------|----------|
| Flag help + self-describe catalog | The two surfaces an agent reads at runtime. Under D-00, these are the feature. | ✓ |
| docs-site CLI reference | Published, human-facing, versioned. | ✓ |
| CLAUDE.md memory contract | Currently describes `cross_spine` as MCP-only. | (see Q8) |
| Upgrade note / changelog | Follows the v0.12.0 precedent from `05-03`. | ✓ |

**User's choice:** Flag help + self-describe, docs-site reference, upgrade note → **D-07**, **D-08**

### Q8 — CLAUDE.md was left out; is that intentional?

| Option | Description | Selected |
|--------|-------------|----------|
| Defer to 999.2 | Let the surface-wide audit sweep it under one consistent standard. | |
| Out of scope permanently | CLAUDE.md documents the memory contract, not client surfaces. | |
| Actually include it | One line; leaving the canonical description incomplete is the same class of gap the audit caught. | ✓ (qualified) |

**User's choice:** *"3 include it, just not _how_ to use it"* → **D-09**
**Notes:** CLAUDE.md records that the capability is CLI-reachable. It does not gain flag syntax or
usage examples — the "how" stays in the docs-site reference and the flag help.

---

## Claude's Discretion

- Exact footer wording and how `scopes_truncated` is phrased in text mode.
- Whether the coverage footer and `list`'s existing `total:` footer render as one line or two.
- Where the D-03 parity test physically lives, provided it compiles against both the client guard and
  `effectiveSearchScope`.

## Deferred Ideas

- **Surface-wide CLI + MCP interface audit under D-00** → filed as backlog **Phase 999.2**
  (ROADMAP.md, commit `dfde73ca`) at the user's explicit request during the conflict-area discussion.
- **STATE.md milestone-count drift** — `total_phases: 6`, `percent: 100`, written before the audit
  added Phase 7. Belongs to the milestone lifecycle, not this phase.
- **`.planning/v0.12.x-MILESTONE-AUDIT.md` was untracked** during this discussion despite being the
  document that justified Phase 7.

### Areas offered but not explored

Raised in the final gate and declined in favor of proceeding to planning:

- Whether `engram store` needs anything here (it has no scope-recall path, so likely not).
- Whether the D-03 parity test belongs in `cmd/engram` or `internal/server`.
- Whether Phase 7 should close the STATE.md drift.
