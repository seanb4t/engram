# Phase 6: Cross-Spine Partial Results - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-20
**Phase:** 6-cross-spine-partial-results
**Areas discussed:** The partial-result contract, CLI surface, Proto field numbering + naming, Scope of the fix

Research (`ARCHITECTURE.md` lines 113–124) had already settled the broad shape — a new boolean, three distinguishable states, additive on the wire — so discussion focused on what it left open.

---

## The partial-result contract

### What does the caller get when coverage fails after hits exist?

| Option | Description | Selected |
|--------|-------------|----------|
| Success + hits + coverage-unknown flag | RPC succeeds; hits returned, `searched_scopes` absent, flag true | ✓ |
| Success, but only when hits exist | Fail the call when there are zero hits AND coverage failed, since that is indistinguishable from a plain empty result | |
| Always fail — keep today's behavior | Reject the premise; a coverage claim that cannot be made fails the call | |

**User's choice:** Success + hits + coverage-unknown flag.
**Notes:** One path, not a hits/no-hits split — the rejected middle option would have meant two contracts for one condition. Recorded as D-01 and D-03.

### Where does the underlying `ListScopes` error go?

Raised because returning partial data while silently dropping a real backend error is its own trap.

| Option | Description | Selected |
|--------|-------------|----------|
| Log server-side, flag on the wire | Boolean is the whole wire contract; the cause is logged | ✓ |
| Also surface a scrubbed detail string | Richer for clients, but a second error channel outside the `field=/hint=` envelope, with leak risk | |
| Drop it entirely | Simplest; a real backend failure becomes invisible | |

**User's choice:** Log it server-side, flag on the wire. Recorded as D-02.

---

## CLI surface

Context established first: `renderCoverageFooter` (`cmd/engram/client_common.go:329`) is already the ONE shared renderer for both `engram list` and `engram search`, and its own doc comment states that key names are the proto field names verbatim.

| Option | Description | Selected |
|--------|-------------|----------|
| Extend the footer, same vocabulary | A third form, `scopes_unknown: true`, no count | ✓ |
| Footer plus a stderr warning | Harder to miss interactively; splits one signal across two streams | |
| Non-zero exit code too | Strongest for scripts; misreports a call that genuinely succeeded | |

**User's choice:** Extend the footer, same vocabulary. Recorded as D-05.

---

## Proto field numbering + naming

Framed as permanent: a deprecated field still occupies its number, so the choice cannot be walked back.

| Option | Description | Selected |
|--------|-------------|----------|
| `scopes_unknown` | Research's suggestion; sibling of `scopes_truncated`, same prefix and shape | ✓ |
| `coverage_unknown` | Matches the ROADMAP's own "coverage unknown" phrasing word-for-word; breaks the `scopes_` grouping | |
| `scopes_unavailable` | Emphasises the enumeration failed rather than that the answer is unknowable | |

**User's choice:** `scopes_unknown` — `ListMemoriesResponse` field 7, `SearchMemoriesResponse` field 4. Recorded as D-04.

---

## Scope of the fix

| Option | Description | Selected |
|--------|-------------|----------|
| In the helper; List + Search only | `searchedScopes` returns the signal instead of an error; exactly REQ-cross-spine-partial's four surfaces | ✓ |
| In the helper, and extend to search_discovery | Closes a real asymmetry, but is new capability rather than the #456 fix | |
| Restructure each call site individually | Four copies of one decision — the duplication the single-mapper discipline avoids | |

**User's choice:** In the helper; List + Search only. Recorded as D-06, with `search_discovery`/`list_scheduled` coverage noted as deferred ideas rather than dropped.

---

## Claude's Discretion

- The exact signature `searchedScopes` grows, provided the four call sites cannot silently ignore the signal.
- Log level, message and structured fields for D-02.
- Placement of the MCP `scopes_unknown` key relative to `recallResultMap`'s cross-spine branch.
- Test design (the `ListScopes` failure must be injectable on all four surfaces plus the renderer).
- Phase 6 red-evidence patches.

## Deferred Ideas

- Cross-spine coverage reporting for `search_discovery` (supports `cross_spine`, reports no coverage today).
- The same for `list_scheduled`.
