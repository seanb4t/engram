---
phase: 19-console-write-ux
plan: 02
subsystem: ui
tags: [connect-rpc, connect-es, csrf, interceptors, vitest]

# Dependency graph
requires:
  - phase: 19-01-console-write-ux-prerequisites
    provides: re-vendored ui/src/lib/gen/engram_pb.ts with all 6 write RPCs, Citation, and Visibility
provides:
  - attachCsrf Interceptor (ui/src/lib/interceptors/csrf.ts) echoing the engram_csrf cookie as X-CSRF-Token on every write request
  - retryOnce Interceptor (ui/src/lib/interceptors/retryOnce.ts) — a single opportunistic auth-race retry on Unauthenticated/PermissionDenied, terminal on second failure
  - engramWrite client (ui/src/lib/client.ts) on a [retryOnce, attachCsrf] transport; read client `engram` unchanged
affects: [19-03-destructive-actions, 19-04-write-forms, 19-05-resume-envelope, 19-06]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Transport-level interceptor composition for cross-cutting write concerns (CSRF attach, auth-race retry) instead of per-mutationFn logic"
    - "Interceptor order is outer→inner in the array: [retryOnce, attachCsrf] so a retry re-enters attachCsrf and re-reads document.cookie fresh"
    - "createRouterTransport + a call-counting service handler for interceptor unit tests (no live server)"
    - "Node-tier vitest tests (environment: 'node', no DOM) stub globalThis.document = { cookie } per test via Object.defineProperty, since attachCsrf/tests only need a mutable cookie string, not a real DOM"

key-files:
  created:
    - ui/src/lib/interceptors/csrf.ts
    - ui/src/lib/interceptors/csrf.test.ts
    - ui/src/lib/interceptors/retryOnce.ts
    - ui/src/lib/interceptors/retryOnce.test.ts
  modified:
    - ui/src/lib/client.ts
    - ui/src/lib/client.test.ts

key-decisions:
  - "retryOnce's retry set is exactly {Code.Unauthenticated, Code.PermissionDenied} — a client-side interpretation (Pitfall 1), since there is no dedicated server 'needs rotation' signal"
  - "Retry is framed and named as a SINGLE OPPORTUNISTIC AUTH-RACE RETRY (session-cookie freshness race), never 're-seal on retry' or 'rotation' — connectreseal.go:39 skips re-sealing on errored/nil responses, so a failed request cannot itself trigger a re-seal; tests/comments use the 'auth-race retry' name throughout, per round-2/round-4 review reconciliation"
  - "engramWrite is a separate client/transport from the existing plain `engram` read client (not shared interceptors on one client) — cleaner, more auditable, read-path latency/behavior completely unchanged"
  - "Interceptor array literal is [retryOnce, attachCsrf] (retryOnce outer) — verified both by an `rg` order-gate and a composed router-transport test that changes document.cookie mid-flight and asserts the retry's request carries the refreshed value"

patterns-established:
  - "Auth-race retry: catch ConnectError, narrow via ConnectError.from(err), retry next(req) exactly once on a fixed code set, rethrow unchanged on second failure — no backoff, no retry library"
  - "CSRF echo: read document.cookie fresh per request in one Interceptor choke point, never cache, never mint client-side — names must match server wire-contract constants verbatim"

requirements-completed: [REQ-console-write-ux]

coverage:
  - id: D1
    description: "attachCsrf interceptor sets X-CSRF-Token from the engram_csrf cookie on every write request, re-reading it fresh per request (never cached)"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/interceptors/csrf.test.ts#attachCsrf (3 tests: present, absent, re-read-per-request)"
        status: pass
    human_judgment: false
  - id: D2
    description: "retryOnce interceptor performs exactly one opportunistic auth-race retry on Unauthenticated or PermissionDenied, rethrowing the second failure unchanged; no retry for other codes"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/interceptors/retryOnce.test.ts#auth-race retry (4 tests: success-on-retry x2 codes, hard-fail-propagates, non-auth-no-retry)"
        status: pass
    human_judgment: false
  - id: D3
    description: "engramWrite client exported from client.ts on a [retryOnce, attachCsrf] transport, distinct from the unchanged read client engram"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/client.test.ts#engramWrite (distinct-client assertion)"
        status: pass
      - kind: other
        ref: "rg \"interceptors: \\[retryOnce, attachCsrf\\]\" ui/src/lib/client.ts"
        status: pass
    human_judgment: false
  - id: D4
    description: "Composed-interceptor proof: retryOnce (outer) re-enters attachCsrf on retry and reads a cookie value refreshed mid-flight, not the stale first-attempt value"
    requirement: "REQ-console-write-ux"
    verification:
      - kind: unit
        ref: "ui/src/lib/client.test.ts#auth-race retry (composed [retryOnce, attachCsrf])"
        status: pass
    human_judgment: false

# Metrics
duration: 15min
completed: 2026-07-15
status: complete
---

# Phase 19 Plan 02: Write Transport (CSRF + Auth-Race Retry) Summary

**Two composed Connect-ES interceptors — `attachCsrf` (echoes the `engram_csrf` cookie as `X-CSRF-Token`) and `retryOnce` (a single opportunistic auth-race retry on Unauthenticated/PermissionDenied) — plus a dedicated `engramWrite` client on `[retryOnce, attachCsrf]`, unit-tested against `createRouterTransport` including a composed test proving the retry re-reads a refreshed cookie.**

## Performance

- **Duration:** 15 min
- **Started:** 2026-07-15T09:45:00-04:00
- **Completed:** 2026-07-15T09:47:11-04:00
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments

- `attachCsrf` (`ui/src/lib/interceptors/csrf.ts`) parses `document.cookie` fresh per request and sets `X-CSRF-Token` when `engram_csrf` is present — cookie/header names match `internal/webauth/csrf.go`'s exported constants verbatim; the server (`connectcsrf.go`) remains the sole authoritative verifier
- `retryOnce` (`ui/src/lib/interceptors/retryOnce.ts`) retries `next(req)` exactly once on `Code.Unauthenticated` or `Code.PermissionDenied`, rethrowing the second failure unchanged — framed accurately as a **single opportunistic auth-race retry** (session-cookie freshness race), never "retry through re-seal" or "rotation," grounded in `connectreseal.go:39` (reseal skips errored/nil responses) and `resolver.go:49` (no "needs rotation" server state)
- `engramWrite` (`ui/src/lib/client.ts`) is a new write-only client on `createConnectTransport({ baseUrl: '/', interceptors: [retryOnce, attachCsrf] })` — array order is retryOnce-outer so a retry re-enters `attachCsrf` and re-reads the cookie; the existing `engram` read client and `mapAuthError` are untouched
- A composed-interceptor test (Codex round-4 LOW) in `client.test.ts` mutates `document.cookie` inside the handler's first invocation, then fails `PermissionDenied`; asserts the retry's second request carries the refreshed cookie value and the handler was invoked exactly twice — proving the interceptor order actually delivers the "retry re-reads the cookie" guarantee, not just the individual units in isolation

## Task Commits

Each task was committed atomically:

1. **Task 1: attachCsrf interceptor (reads engram_csrf, sets X-CSRF-Token)** - `8c12dc6f` (feat)
2. **Task 2: retryOnce interceptor (single silent retry on auth-class failure)** - `775e2aeb` (feat)
3. **Task 3: engramWrite client on the two-interceptor transport** - `f2b01ae8` (feat)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `ui/src/lib/interceptors/csrf.ts` - `attachCsrf: Interceptor`, echoes `engram_csrf` cookie as `X-CSRF-Token`
- `ui/src/lib/interceptors/csrf.test.ts` - cookie-present / cookie-absent / re-read-per-request cases via `createRouterTransport`
- `ui/src/lib/interceptors/retryOnce.ts` - `retryOnce: Interceptor`, one retry on `{Unauthenticated, PermissionDenied}`, rethrow unchanged otherwise
- `ui/src/lib/interceptors/retryOnce.test.ts` - "auth-race retry" suite: success-on-retry (both codes), hard-fail-propagates, non-auth-code-no-retry, each with exact invocation-count assertions
- `ui/src/lib/client.ts` - adds `writeTransport`/`engramWrite`; `engram`/`mapAuthError` unchanged
- `ui/src/lib/client.test.ts` - adds engramWrite-distinct-from-engram assertion and the composed `[retryOnce, attachCsrf]` refreshed-cookie test

## Decisions Made

- Retry-set decision (Pitfall 1 interpretation): the retry set is exactly `{Code.Unauthenticated, Code.PermissionDenied}`, a client-side judgment call since the server has no dedicated "needs rotation" signal — documented in a source comment in `retryOnce.ts` rather than left implicit.
- Interceptor order rationale: `[retryOnce, attachCsrf]` (retryOnce outer) is the ONLY order under which a retry re-enters `attachCsrf` and re-reads `document.cookie`; the reverse order would freeze the header from the first attempt. This is now proven by a dedicated composed test (`client.test.ts`), not just asserted in a comment.
- Accurate opportunistic-race framing: the retry is documented (in code comments and test names, "auth-race retry") as repairing a session-cookie freshness race only — a failed request cannot trigger a server re-seal (`connectreseal.go:39` skips errored responses) — never as "retry through re-seal" or "rotation recovery." A hard-expired session fails the retry too and is terminal, driving D-09's inline re-auth in the (not-yet-built) form layer.
- `engramWrite` kept as a fully separate client/transport from `engram` (not shared interceptors on one client) per RESEARCH.md's Assumption A3 — cleaner, more auditable, and the read path's behavior/latency is completely unaffected.

## Deviations from Plan

None - plan executed exactly as written. Node-tier `document` stubbing (via `Object.defineProperty(globalThis, 'document', ...)` per test) was implicit in the plan's "Stub `document.cookie` for the node environment" instruction and required no interpretation beyond what was specified.

## Issues Encountered

None. `pnpm test:node -- <path>` (via the `pnpm` script wrapper) does not forward the trailing file-path filter correctly (runs the full node-tier suite instead of just the named file) — a pre-existing `package.json`/pnpm-args interaction, not something this plan's tasks touch. Worked around by invoking `npx vitest run --project node <path>` directly for scoped verification; the full `pnpm test:node` (unfiltered) run and `pnpm check` were both also run clean as the authoritative gates.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `engramWrite` is ready for Plan 04 (write forms) to import for all six write RPCs — CSRF attachment and the auth-race retry are both handled transparently at the transport layer, so no mutation wrapper needs to reimplement either.
- Plan 03 (destructive actions) and Plan 04 (write forms) can build `createMutation` calls directly against `engramWrite` without any further transport work.
- The terminal (post-retry) failure path that Plan 04/05/06 must handle is a `ConnectError` with `code` still `Unauthenticated` or `PermissionDenied` reaching `onError` — the interceptor is fully transparent to `createMutation`, which only ever observes the final outcome.
- No blockers.

---
*Phase: 19-console-write-ux*
*Completed: 2026-07-15*

## Self-Check: PASSED

All created files (ui/src/lib/interceptors/csrf.ts, csrf.test.ts, retryOnce.ts, retryOnce.test.ts, this SUMMARY.md) verified present on disk. All 3 task commits (8c12dc6f, 775e2aeb, f2b01ae8) verified present in git log.
