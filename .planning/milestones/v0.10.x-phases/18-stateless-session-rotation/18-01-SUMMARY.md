---
phase: 18-stateless-session-rotation
plan: 01
subsystem: auth
tags: [go, aes-gcm, cookie, session, connect-rpc, csrf]

# Dependency graph
requires:
  - phase: 16-csrf-interceptor
    provides: CSRFSigner.Token(owner) stability across expiry (Owner-only HMAC), CSRFCookieName wire contract
provides:
  - "Handler.Reseal(respHeader http.Header, r *http.Request) — best-effort, void-return sliding-expiry re-seal primitive"
  - "headerOnlyWriter shim — reuses setCookie/setReadableCookie from a plain http.Header"
  - "resealThreshold (sessionTTL/2) and resealSkew (60s) named constants"
  - "TestResolveHardExpiryHasNoSkewTolerance — pins resolver.go's hard-expiry check as byte-for-byte strict"
affects: [18-02-connect-reseal-interceptor, 18-03-serve-wiring, 18-adr-plan]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "headerOnlyWriter: minimal http.ResponseWriter shim over a plain http.Header, mirroring the existing dummy-*http.Request read-direction trick (resolver.go), to reuse cookie-attribute helpers from a write-only header context"
    - "Threshold-only clock-skew budget kept structurally separate (different named constant, different function) from a hard fail-closed expiry check, to prevent skew leaking into the strict path"

key-files:
  created:
    - internal/webauth/reseal.go
    - internal/webauth/reseal_test.go
  modified:
    - internal/webauth/resolver_test.go

key-decisions:
  - "New expiry is always absolute nowUTC().Add(sessionTTL), never oldExpiry+delta (D-06) — verified by a 50-goroutine -race concurrency test asserting every produced expiry equals exactly the pinned nowUTC()+sessionTTL"
  - "resealSkew (60s) is scoped exclusively to the Reseal threshold comparison; resolver.go's hard-expiry check remains untouched (git diff empty) and is pinned by a new negative-space test targeting a 1ns-expired session (D-07, SC4)"
  - "Reseal reuses h.setCookie/h.setReadableCookie unchanged via headerOnlyWriter rather than duplicating cookie-attribute logic or refactoring those methods"

patterns-established:
  - "Best-effort, void-return refresh methods that silently no-op on any internal failure (never turn a handler success into an error) — D-04's contract, implemented as a structural signature (no error return type at all), not just a convention"

requirements-completed: [REQ-session-rotation]

coverage:
  - id: D1
    description: "Handler.Reseal re-seals the session cookie with a fresh absolute expiry only once remaining lifetime drops below resealThreshold+resealSkew"
    requirement: "REQ-session-rotation"
    verification:
      - kind: unit
        ref: "internal/webauth/reseal_test.go#TestResealNoopBeforeThreshold"
        status: pass
      - kind: unit
        ref: "internal/webauth/reseal_test.go#TestResealPastThresholdRefreshesSessionCookie"
        status: pass
    human_judgment: false
  - id: D2
    description: "Reseal refreshes the engram_csrf cookie's Max-Age with the same HMAC(k_csrf, Owner) value in the same call"
    requirement: "REQ-session-rotation"
    verification:
      - kind: unit
        ref: "internal/webauth/reseal_test.go#TestResealPastThresholdRefreshesCSRFCookie"
        status: pass
    human_judgment: false
  - id: D3
    description: "N concurrent near-expiry re-seals through the pinned nowUTC seam all emit forward-monotonic, absolute expiries"
    requirement: "REQ-session-rotation"
    verification:
      - kind: unit
        ref: "internal/webauth/reseal_test.go#TestResealForwardMonotonicUnderConcurrency (go test -race)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Resolver.Resolve rejects a session expired by 1ns — hard expiry has zero skew tolerance, resolver.go unchanged"
    requirement: "REQ-session-rotation"
    verification:
      - kind: unit
        ref: "internal/webauth/resolver_test.go#TestResolveHardExpiryHasNoSkewTolerance"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-07-13
status: complete
---

# Phase 18 Plan 01: Stateless Session Re-seal Primitive Summary

**Handler.Reseal — a best-effort, void-return method that re-seals the AES-GCM session cookie with a fresh absolute expiry past a ½-TTL+skew threshold and refreshes the paired CSRF cookie's Max-Age, proven forward-monotonic under a 50-goroutine `-race` concurrency test, with a pinning test guaranteeing the resolver's hard-expiry check keeps zero skew tolerance.**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-07-13
- **Tasks:** 2
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments
- `internal/webauth/reseal.go`: `Handler.Reseal` — unseal → threshold(+skew) check → re-seal session cookie (absolute `nowUTC().Add(sessionTTL)`) → refresh `engram_csrf` cookie Max-Age, all silent-no-op on any failure (D-04).
- `headerOnlyWriter` shim lets `Reseal` call the existing `setCookie`/`setReadableCookie` unchanged from a plain `http.Header`, with zero duplicated cookie-attribute logic.
- Named constants `resealThreshold` (`sessionTTL/2` = 6h) and `resealSkew` (60s), scoped exclusively to the re-seal threshold comparison — no new `ENGRAM_` var, no server-side state.
- `internal/webauth/reseal_test.go`: threshold no-op, past-threshold dual-cookie re-seal (session + CSRF), and a 50-goroutine `-race` forward-monotonic concurrency test.
- `internal/webauth/resolver_test.go`: `TestResolveHardExpiryHasNoSkewTolerance` — a 1ns-expired session is still rejected, pinning `resolver.go:49-51` as byte-for-byte unchanged (verified via empty `git diff`).

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement Handler.Reseal + headerOnlyWriter shim + threshold constants (with unit + concurrency tests)** - `b904f439` (feat)
2. **Task 2: SC4 guard — pin Resolver.Resolve hard-expiry as byte-for-byte strict (no skew)** - `8f989140` (test)

## Files Created/Modified
- `internal/webauth/reseal.go` - `Handler.Reseal`, `headerOnlyWriter`, `resealThreshold`, `resealSkew`
- `internal/webauth/reseal_test.go` - threshold no-op, dual-cookie re-seal, and concurrency tests
- `internal/webauth/resolver_test.go` - `TestResolveHardExpiryHasNoSkewTolerance` (SC4 guard)

## Decisions Made
- Followed the plan's TDD flow: wrote `reseal_test.go` first, confirmed a compile-failure RED (no `Reseal` method existed yet), then implemented `reseal.go` to GREEN.
- Used `net/http.Response{Header: hdr}.Cookies()` in tests to parse `Set-Cookie` values (stdlib-supported response-cookie parsing) rather than hand-rolling a parser.
- No deviations from the plan's exact constant names, signatures, or comparison direction — implemented exactly as specified in `18-01-PLAN.md` and cross-checked against `18-RESEARCH.md` §3.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None. `gofmt -w` reformatted `reseal.go`'s method-alignment spacing automatically after initial write (cosmetic, no functional change).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `Handler.Reseal` and its threshold/skew constants are ready for Plan 18-02 (the Connect reseal interceptor) to wire in via the `resealFunc`-shaped DI documented in `18-RESEARCH.md` §1/§2.
- `resolver.go` is verified unchanged (empty diff) — Plan 18-02/18-03 must continue to respect the hard-expiry-strict / threshold-skew-tolerant split (D-07).
- No blockers. `go test ./internal/webauth/... -count=1`, `-run Reseal -race`, and `-run HardExpiry` are all green; `golangci-lint run ./internal/webauth/...` reports 0 issues; `task license:check` passes.

---
*Phase: 18-stateless-session-rotation*
*Completed: 2026-07-13*

## Self-Check: PASSED
