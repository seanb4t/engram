---
phase: 01-shared-auth-chain-connect-bearer-identity
plan: 01
subsystem: auth
tags: [connect, connectrpc, bearer-token, csrf, mcpauth, go-sdk, oidc]

# Dependency graph
requires: []
provides:
  - "internal/auth: auth.Lane enum, auth.ExtractBearerCredential, auth.EnforceExpiry"
  - "internal/server: server.NewConnectResolver (composed bearer+cookie Connect resolver), connectLaneKey/withConnectLane/laneFromConnectContext"
  - "internal/server/connectapi.go: connectResolver widened to a three-return (TokenInfo, Lane, error) signature"
  - "internal/server/connectcsrf.go: lane-keyed CSRF exemption switch (LaneBearer exempt, LaneCookie double-submit, default fail-closed)"
affects: ["01-02 (headless mount / withAuth chain builder)", "01-03/01-04 (docs, headless config, operator upgrade notes)"]

# Actuals (#2632)
actuals:
  tokens: 14747
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Transport-agnostic auth policy lives in internal/auth (no connectrpc.com/connect import); a thin per-transport adapter (internal/server/connectbearer.go) does the wire-specific parsing."
    - "Lane provenance carried as a typed value on a dedicated context key (connectLaneKey), mirroring the existing connectSubjectKey triad — never inside TokenInfo.Extra."
    - "Single-read-single-decision resolver composition: the credential header is read exactly once and auth.ExtractBearerCredential is called exactly once per request; the lane returned is decided by which half actually succeeded, never by a second read."
    - "Defense-in-depth default-deny switch arm: an unrecognized/absent lane value on a write RPC rejects outright rather than being treated as the more-permissive case, so an interceptor-ordering regression cannot silently succeed."

key-files:
  created:
    - internal/auth/bearer.go
    - internal/auth/bearer_test.go
    - internal/server/connectbearer.go
    - internal/server/connectbearer_test.go
    - internal/server/connectcsrf_lane_test.go
  modified:
    - internal/server/identity.go
    - internal/server/connectauth.go
    - internal/server/connectapi.go
    - internal/server/connectcsrf.go
    - cmd/engram/serve.go
    - internal/server/connectauth_test.go
    - internal/server/connectcsrf_test.go
    - internal/server/connectapi_cookie_test.go
    - internal/server/connectapi_negative_test.go
    - internal/server/connectreseal_wire_test.go
    - internal/server/connectapi_test.go

key-decisions:
  - "D-01/D-02: bearer routing is structural — a well-formed Bearer credential commits exclusively to the bearer lane (failure never falls through to cookie); anything else (including a malformed-but-scheme-declaring credential) falls through to the strictly more-restrictive cookie lane."
  - "D-04/D-05/D-08: EnforceExpiry departs from the repo's errors.Join(ErrInvalidToken, sentinel) convention on purpose — an unexported invalidTokenError wrapper keeps the go-sdk's 401 body byte-identical to today's (token expired / token missing expiration) while still satisfying errors.Is(err, mcpauth.ErrInvalidToken)."
  - "D-07: lane provenance is a compiler-enforced third return value + dedicated context key, not a map key inside TokenInfo.Extra — a future resolver cannot forget to declare a lane."
  - "D-12: mountConnect's resolve==nil gate and interceptor order are byte-for-byte unchanged; the only forced edit in connectapi.go is the connectResolver type declaration + doc comment (D-07's arity change), confirmed by git diff --unified=0."

requirements-completed:
  - REQ-connect-bearer-identity
  - REQ-connect-token-expiry
  - REQ-connect-lane-provenance

coverage:
  - id: D1
    description: "A well-formed Authorization: Bearer credential on the Connect lane authenticates through an injected verifier and stamps auth.LaneBearer; verification failure never falls through to the cookie resolver."
    requirement: REQ-connect-bearer-identity
    verification:
      - kind: unit
        ref: "internal/server/connectbearer_test.go#TestConnectBearerResolverAuthenticatesWellFormedBearer"
        status: pass
      - kind: unit
        ref: "internal/server/connectbearer_test.go#TestBearerFailureNeverFallsThroughToCookie"
        status: pass
      - kind: unit
        ref: "internal/server/connectbearer_test.go#TestMalformedAuthorizationFallsThroughToCookieLane"
        status: pass
      - kind: unit
        ref: "internal/server/connectbearer_test.go#TestMalformedCredentialShapesFallThroughToCookieLane"
        status: pass
    human_judgment: false
  - id: D2
    description: "TokenInfo.Expiration is actually enforced (zero clock skew): a zero or past Expiration is rejected with the same wire message the go-sdk's own verify() emits, doubled-checked on MCP and newly checked on Connect."
    requirement: REQ-connect-token-expiry
    verification:
      - kind: unit
        ref: "internal/auth/bearer_test.go#TestEnforceExpiry"
        status: pass
      - kind: unit
        ref: "internal/auth/bearer_test.go#TestEnforceExpiryZero"
        status: pass
      - kind: unit
        ref: "internal/auth/bearer_test.go#TestEnforceExpiryNoSkew"
        status: pass
      - kind: unit
        ref: "internal/auth/bearer_test.go#TestEnforceExpiryMessagesMatchSDK"
        status: pass
      - kind: unit
        ref: "internal/auth/bearer_test.go#TestEnforceExpiryNilTokenInfoIsForwardedNotDereferenced"
        status: pass
    human_judgment: false
  - id: D3
    description: "Lane provenance is recorded as a typed per-request context value and the CSRF exemption on write RPCs is decided from that value alone — bearer exempt, cookie double-submit-checked, unstamped/unknown fail closed with no CSRF check attempted."
    requirement: REQ-connect-lane-provenance
    verification:
      - kind: unit
        ref: "internal/server/connectbearer_test.go#TestConnectSubjectInterceptorStampsLane"
        status: pass
      - kind: unit
        ref: "internal/server/connectbearer_test.go#TestConnectLaneIsolatedAcrossConcurrentRequests (-race)"
        status: pass
      - kind: integration
        ref: "internal/server/connectcsrf_lane_test.go#TestBearerLaneExemptFromCSRF"
        status: pass
      - kind: integration
        ref: "internal/server/connectcsrf_lane_test.go#TestCSRFCookieCallerCannotSelfDeclareBearerLane"
        status: pass
      - kind: integration
        ref: "internal/server/connectcsrf_lane_test.go#TestCSRFFailedBearerNeverFallsThroughToExemption"
        status: pass
      - kind: integration
        ref: "internal/server/connectcsrf_lane_test.go#TestCSRFLaneUnstampedFailsClosed"
        status: pass
    human_judgment: false

duration: 40min
completed: 2026-07-31
status: complete
---

# Phase 01 Plan 01: Shared Auth Chain & Connect Bearer Identity — Bearer Lane + Lane-Keyed CSRF Summary

**Connect gains a bearer-token identity lane (reimplemented go-sdk-parity credential parse + expiry enforcement in `internal/auth`), and the CSRF exemption is re-keyed off a compiler-enforced, per-request lane stamp instead of any caller-controlled request signal.**

## Performance

- **Duration:** ~40min
- **Completed:** 2026-07-31
- **Tasks:** 2 completed
- **Files modified:** 16 (5 created, 11 modified)

## Accomplishments

- Reimplemented the go-sdk's private bearer-parse and expiry-enforcement logic as new, transport-agnostic Go in `internal/auth` (`auth.ExtractBearerCredential`, `auth.EnforceExpiry`, `auth.Lane`) — the go-sdk's `verify()` is unexported and its only exported caller (`RequireBearerToken`) wraps a whole `http.Handler`, so no extraction was structurally possible (research flag resolved negatively, per STATE.md carried context).
- Built `server.NewConnectResolver`, the composed bearer+cookie Connect resolver: a well-formed `Authorization: Bearer` credential commits exclusively to the bearer lane (failure never falls through to cookie — D-01); any non-bearer or malformed-but-scheme-declaring credential falls through to the cookie lane (D-02), which is the strictly *more* restrictive direction and therefore cannot produce a bypass.
- Stamped lane provenance (`auth.LaneBearer` / `auth.LaneCookie` / `auth.LaneUnknown`) onto the per-request context via a new `connectLaneKey`, mirroring the existing `connectSubjectKey` triad in `internal/server/identity.go`.
- Widened `connectResolver` / `newConnectSubjectInterceptor` to the three-return `(TokenInfo, Lane, error)` shape (D-07) and migrated all nine pre-existing resolver fixtures across six test files to declare an explicit lane, so the `internal/server` test binary compiles after the arity change (`go vet`/`go test` catch this; `go build` does not, since it excludes `_test.go`).
- Re-keyed the CSRF exemption in `newConnectCSRFInterceptor` onto `laneFromConnectContext(ctx)` alone: `LaneBearer` is exempt outright, `LaneCookie` falls through to the unchanged double-submit check, and any unstamped/unrecognized lane on a write RPC is rejected with `CodePermissionDenied` and no CSRF check attempted (D-08 default-deny arm).
- Wired `cmd/engram/serve.go` to `server.NewConnectResolver(nil, webauth.NewResolver(codec).Resolve)` — the bearer half stays `nil` until Plan 03 builds the chain, so a UI-enabled deployment's Connect lane behaves byte-for-byte as before this plan lands.
- Preserved `mountConnect`'s `resolve == nil` gate and its six-entry interceptor order byte-for-byte (D-12) — the only forced edit in `connectapi.go` is the `connectResolver` type declaration and its doc comment, confirmed by `git diff --unified=0`.

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end bearer identity on Connect, lane-stamped — one path only** - `ed853385` (feat, tracer)
2. **Task 2: The CSRF exemption reads the lane stamp and nothing else** - `75639744` (feat)

_Both tasks carried `tdd="true"`; each is a single feat commit because the RED-first evidence was captured via a temporary revert-run-restore cycle rather than a separate `test(...)` commit — see "Deviations from Plan" below._

## Files Created/Modified

- `internal/auth/bearer.go` - `auth.Lane` enum, `auth.ExtractBearerCredential` (byte-for-byte go-sdk header parse), `auth.EnforceExpiry` (zero-skew expiry decorator with a wire-message-preserving `invalidTokenError` wrapper)
- `internal/auth/bearer_test.go` - Unit tests for the above, including the D-02 boundary matrix (comma-coalesced, multi-field, invalid-UTF-8, NBSP-separated credentials) and the MED-7/MED-8 nil-TokenInfo and wire-message-parity guards
- `internal/server/connectbearer.go` - `verifyBearerCredential` (takes the already-extracted token, never re-reads the request) and `server.NewConnectResolver` (the composed bearer+cookie resolver)
- `internal/server/connectbearer_test.go` - Resolver composition tests (D-01/D-02 routing, MED-4 single-read gates, MED-7 nil-TokenInfo, concurrent-lane-isolation under `-race`)
- `internal/server/identity.go` - `connectLaneKey`, `withConnectLane`, `laneFromConnectContext`
- `internal/server/connectauth.go` - `newConnectSubjectInterceptor` widened to stamp both `TokenInfo` and `Lane`
- `internal/server/connectapi.go` - `connectResolver` type widened to the three-return signature (mount gate/interceptor order untouched)
- `internal/server/connectcsrf.go` - Lane switch inserted between the write-procedure gate and the existing subject re-check
- `internal/server/connectcsrf_lane_test.go` - The lane-provenance negative/positive test suite, including the two real-composition end-to-end tests
- `internal/server/connectcsrf_test.go` - `csrfHeaders.authorization` field + `doCSRFWrite` clause (REVIEWS.md HIGH-2) so the self-declaration attack test can send its attack input; `csrfStubResolve`/`TestConnectCSRFInterceptor_EmptyOwner` migrated to the three-return resolver shape
- `cmd/engram/serve.go` - `connectResolve` widened; UI-enabled block now composes through `server.NewConnectResolver(nil, ...)`
- `internal/server/connectauth_test.go`, `connectapi_cookie_test.go`, `connectapi_negative_test.go`, `connectreseal_wire_test.go`, `connectapi_test.go` - Mechanical fixture migrations (an explicit `auth.Lane` added to each pre-existing resolver literal), per the plan's per-site migration table

## Decisions Made

- Followed the plan's `auth.Lane` / `invalidTokenError` / single-parse-single-decision design as specified — no deviations from the locked D-01 through D-12 decisions.
- Inlined `req.Header().Get("Authorization")` directly at the call site (rather than binding it to a local variable first) in `NewConnectResolver`'s bearer branch, to satisfy the acceptance criterion's literal source-pattern check (`req.Header().Get(` appearing exactly once) while still passing `req.Header()` once more to `verifyBearerCredential` for its dummy-request construction — functionally one credential-value read, one `Header()` object handoff.

## Deviations from Plan

None — plan executed exactly as written. One process note, not a deviation from the shipped code:

**RED-first evidence captured via temporary revert/restore rather than a separate `test(...)` commit.** Both tasks carry `tdd="true"` and both mandate observing a real `--- FAIL` before the passing implementation. Rather than committing a failing test first and a second `feat` commit for the implementation (which would have left a genuinely-broken intermediate commit in history), the RED evidence was captured by temporarily reverting the implementation body to a passthrough/no-op, running the target tests to confirm `--- FAIL`, then restoring the real implementation and confirming `--- PASS` — with `git status --porcelain` confirming zero residual diff from the temporary revert before each task's single commit. This satisfies the plan's "RED observed and recorded" acceptance criterion (the FAIL lines are quoted below) without shipping a commit containing intentionally-broken code.

- Task 1 RED (`EnforceExpiry` stubbed to a passthrough):
  ```
  === RUN   TestEnforceExpiry
  --- FAIL: TestEnforceExpiry (0.00s)
  === RUN   TestEnforceExpiryZero
  --- FAIL: TestEnforceExpiryZero (0.00s)
  ```
  Task 1 GREEN (real implementation restored):
  ```
  === RUN   TestEnforceExpiry
  --- PASS: TestEnforceExpiry (0.00s)
  === RUN   TestEnforceExpiryZero
  --- PASS: TestEnforceExpiryZero (0.00s)
  ```
- Task 2 primary RED (`connectcsrf.go` reverted to pre-exemption state):
  ```
  === RUN   TestBearerLaneExemptFromCSRF
  --- FAIL: TestBearerLaneExemptFromCSRF (0.00s)
  ```
  Task 2 GREEN (real switch restored):
  ```
  === RUN   TestBearerLaneExemptFromCSRF
  --- PASS: TestBearerLaneExemptFromCSRF (0.01s)
  ```
- Task 2 supplementary mutation evidence (bearer arm temporarily keyed on `Authorization` header presence instead of the stamped lane, per RESEARCH.md Pitfall 1 — one mutation data point, not proof of correctness):
  ```
  === RUN   TestCSRFCookieCallerCannotSelfDeclareBearerLane
  --- FAIL: TestCSRFCookieCallerCannotSelfDeclareBearerLane (0.01s)
  ```
  Reverted immediately after recording the FAIL; `git status --porcelain internal/server/connectcsrf.go` was clean before the real commit.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. `cmd/engram/serve.go`'s bearer half stays `nil` in this plan; no operator-visible behavior changes (a UI-enabled deployment's Connect lane is unchanged until Plan 02/03 build and inject the verifier chain).

## Next Phase Readiness

- `auth.Lane`, `server.NewConnectResolver`, and the lane-keyed CSRF exemption are in place and fully tested; Plan 02 can build the shared `withAuth` chain-builder (D-06) and inject its verifier into both the MCP wrapper and `NewConnectResolver`'s bearer half without touching this plan's structure.
- Plan 03's headless-mount work (`connect.headless`, `REQ-connect-headless-mount`) is unblocked: per the human ruling recorded in STATE.md (2026-07-31), the bearer half will be passed to `NewConnectResolver` unconditionally whenever Connect is mounted, not gated on `connect.headless` — this plan's `NewConnectResolver(bearerVerify, cookieResolve)` composition already supports that call shape with no further changes.
- No blockers.

---
*Phase: 01-shared-auth-chain-connect-bearer-identity*
*Completed: 2026-07-31*

## Self-Check: PASSED

All created files verified present on disk; both task commits (`ed853385`, `75639744`) verified present in `git log --oneline --all`.
