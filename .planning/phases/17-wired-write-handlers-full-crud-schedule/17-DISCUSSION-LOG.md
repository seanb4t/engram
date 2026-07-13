# Phase 17: Wired Write Handlers (Full CRUD + Schedule) - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-12
**Phase:** 17-wired-write-handlers-full-crud-schedule
**Areas discussed:** MCP re-home, deps signature shape, service-token/OBO identity, refactor scope, parity test harness, phase-size proceed gate (proto↔args adapter locked to recommendation without discussion)

---

## MCP re-home (user-added area)

| Option | Description | Selected |
|--------|-------------|----------|
| Defer — keep shared deps core | Phase 17 wires both lanes onto shared deps.*; capture MCP-on-Connect as a future ADR | ✓ |
| Adopt now — re-home this phase | Make Connect canonical, rewrite MCP tools as a shim; large scope expansion | |
| Spike it first | Throwaway spike of MCP-tool-over-Connect for one RPC before deciding | |

**User's choice:** Defer — keep shared deps core.
**Notes:** `deps.*` already IS the shared core; re-homing doesn't reduce duplication beyond the deps refactor, forces MCP `(id, short_id)`+advisory ergonomics through the proto contract, and doesn't escape the `(subj, actor)` threading. Out of this phase's REQ boundary.

---

## deps signature shape

| Option | Description | Selected |
|--------|-------------|----------|
| caller struct (uniform) | One `caller{Subj, Actor}` value; no unused-param lint; extensible | ✓ |
| Two explicit params | Literal SC1 `(ctx, subj, actor, a)`; `_`-name unused actor on 3 methods | |
| Surgical (subj all, actor 3) | subj on all six, actor only on the 3 create methods | |

**User's choice:** caller struct — with the explicit constraint that the caller/actor concept must support **service tokens** (authenticated principals with no email) and **OBO JWT/OAuth2 tokens** (acting party ≠ subject acted-for).
**Notes:** Drove the identity model into a sub-discussion (below). `caller` struct chosen partly because it's the additive home for an OBO delegation dimension later.

---

## Service-token / OBO identity (depth from the caller constraint)

### How far should Phase 17 go?

| Option | Description | Selected |
|--------|-------------|----------|
| Design-for, implement later | caller + single callerFromTokenInfo seam; defer service-claim + OBO parsing | |
| Also fix service-token owner | Additionally derive non-empty owner from alt claim when email absent | ✓ |
| Also model OBO delegation | Additionally parse the OBO act-chain now | |

**User's choice:** Also fix service-token owner (expands scope into `internal/auth` + config; OBO stays design-for-only per D-08).

### Service-token owner derivation mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| Fallback claim (default sub) | Keep primary email + new ENGRAM_OWNER_CLAIM_FALLBACK (default sub) | |
| Ordered claim list | Ordered ENGRAM_OWNER_CLAIMS=email,client_id,sub — first non-empty wins | ✓ |
| Explicit service marker | Classify service tokens (azp/token_use) and route to a service claim | |

**User's choice:** Ordered claim list.
**Notes:** Two security invariants locked by rationale (not preference): (D-05) present-but-unverified email must reject, never fall through; (D-06) non-email owners namespaced by claim source, email stays bare. Authz-key change → hard-flag `/gsd-secure-phase`.

---

## Refactor scope

| Option | Description | Selected |
|--------|-------------|----------|
| Writes-only (defer reads) | Wire 6 write handlers; leave Connect reads calling store.* directly | |
| Also rewire reads now | Route Connect read handlers through refactored deps.* too — closes Pitfall 1 fully | ✓ |

**User's choice:** Also rewire reads now.
**Notes:** Larger diff + read-lane retest; fully uniform deps API across both lanes.

---

## Parity test harness

| Option | Description | Selected |
|--------|-------------|----------|
| Fake store seam + shared table | Hermetic fake store; one shared scenario table through both lanes | ✓ |
| Real Qdrant, skip-when-unavailable | testDeps(t) real Qdrant + shared table; authz cells skip when absent | |
| Per-lane separate tests | Independent MCP + Connect tests asserting same codes | |

**User's choice:** Fake store seam + shared table (à la `TestRerankParityMCPAndConnect`).
**Notes:** Requires extracting a narrow `store` interface (`deps.st` is concrete Qdrant today) — planner sequences it first.

---

## Phase-size proceed gate

| Option | Description | Selected |
|--------|-------------|----------|
| Write CONTEXT.md as one phase | Capture everything; let planner wave-split | ✓ |
| Split service-token auth out | Carve service-token/ordered-claim auth into a separate phase | |
| Explore more gray areas | Keep discussing | |

**User's choice:** Write CONTEXT.md as one phase.

## Claude's Discretion

- **Proto↔args adapter** — user did not elect to discuss; locked to recommendation (D-09): dedicated `protoconv` conversion layer + round-trip tests; field_mask→partial-update via Phase-15 allowlist; enum/Citation/Timestamp mapped there.
- caller type naming; ordered-claim config key mechanism (comma list vs plural key); namespace prefix format; fake-store/interface shape; read-rewire wave placement.

## Deferred Ideas

- Re-home MCP tools on the Connect service (future ADR / own milestone).
- OBO / RFC-8693 act-chain parsing + delegation semantics (dedicated auth phase; Phase 17 designs the seam only).
- Session sliding re-seal (Phase 18); console write UX (Phase 19).
