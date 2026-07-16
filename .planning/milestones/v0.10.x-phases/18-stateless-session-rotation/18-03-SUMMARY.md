---
phase: 18-stateless-session-rotation
plan: 03
subsystem: auth
tags: [connect-go, interceptor, session-cookie, csrf, go]

# Dependency graph
requires:
  - phase: 18-stateless-session-rotation (plan 01)
    provides: webauth.Handler.Reseal — the best-effort, void-return cookie re-seal primitive (session + engram_csrf, D-02/D-05/D-06/D-07/D-08)
provides:
  - newConnectResealInterceptor — an innermost, best-effort Connect unary interceptor that fires the injected resealFunc on every successful read AND write response
  - resealFunc type — the DI seam server package uses to accept webauth.Handler.Reseal without an import cycle
  - mountConnect/Register/serve.go wiring: webHandler.Reseal flows from serve.go through Register into mountConnect and is appended last in the interceptor chain
affects: [18-04, gsd-secure-phase (mandatory security review for this phase)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "newConnectXInterceptor factory family extended with an inverse-of-CSRF shape: no procedure allowlist, runs innermost/after-next instead of before-next"
    - "dummy *http.Request{Header: req.Header()} cookie-read trick reused a third time (resolver.go, connectcsrf.go, now connectreseal.go)"

key-files:
  created:
    - internal/server/connectreseal.go
    - internal/server/connectreseal_test.go
  modified:
    - internal/server/connectapi.go
    - internal/server/tools.go
    - cmd/engram/serve.go
    - internal/server/connectcsrf_test.go
    - internal/server/connectapi_test.go
    - internal/server/connectapi_negative_test.go
    - internal/server/connectapi_cookie_test.go
    - internal/server/tools_test.go

key-decisions:
  - "newConnectResealInterceptor is appended LAST in mountConnect's WithInterceptors list (innermost, after validate) so it only ever sees a fully-authorized, valid, successful response (D-04)."
  - "The interceptor is structurally incapable of procedure-gating: it never inspects req.Spec() at all (unlike CSRF's csrfWriteProcedures allowlist), which is how D-03 (fires on read AND write) is enforced by construction rather than by an inverse allowlist."
  - "resealFunc is a plain func(http.Header, *http.Request) type declared in internal/server (not imported from webauth) to avoid a server->webauth->server cycle; webHandler.Reseal is assignable to it structurally."

patterns-established:
  - "Best-effort post-handler interceptor: resp, err := next(...); if err != nil || resp == nil || sideEffect == nil { return resp, err unchanged }; otherwise mutate resp in place and return the same resp, nil."

requirements-completed: [REQ-session-rotation]

coverage:
  - id: D1
    description: "newConnectResealInterceptor fires the injected reseal func exactly once on a successful response for both a read-shaped and a write-shaped request (no procedure allowlist, D-03), and passes it resp.Header() plus a dummy *http.Request carrying the original request cookies."
    requirement: "REQ-session-rotation"
    verification:
      - kind: unit
        ref: "internal/server/connectreseal_test.go#TestNewConnectResealInterceptor_FiresOnSuccess"
        status: pass
    human_judgment: false
  - id: D2
    description: "The interceptor never re-seals when next() returns an error or a literal nil response, and returns next's (resp, err) unchanged in both cases (D-04)."
    requirement: "REQ-session-rotation"
    verification:
      - kind: unit
        ref: "internal/server/connectreseal_test.go#TestNewConnectResealInterceptor_SkipsOnError"
        status: pass
      - kind: unit
        ref: "internal/server/connectreseal_test.go#TestNewConnectResealInterceptor_SkipsOnNilResponse"
        status: pass
    human_judgment: false
  - id: D3
    description: "newConnectResealInterceptor(nil) is a safe permanent no-op passthrough."
    requirement: "REQ-session-rotation"
    verification:
      - kind: unit
        ref: "internal/server/connectreseal_test.go#TestNewConnectResealInterceptor_NilResealIsPassthrough"
        status: pass
    human_judgment: false
  - id: D4
    description: "webHandler.Reseal is threaded from serve.go through Register -> mountConnect and appended innermost in the real WithInterceptors chain; every existing test call site updated and the full internal/server suite stays green."
    requirement: "REQ-session-rotation"
    verification:
      - kind: unit
        ref: "go build ./... && go vet ./... && go test ./internal/server/... -count=1"
        status: pass
    human_judgment: false

# Metrics
duration: ~20min
completed: 2026-07-13
status: complete
---

# Phase 18 Plan 03: Connect Reseal Interceptor + DI Wiring Summary

**New innermost, best-effort `newConnectResealInterceptor` re-seals the session and CSRF cookies on every successful Connect response (read or write), fed `webauth.Handler.Reseal` via a `resealFunc` DI seam threaded from `serve.go` through `Register` into `mountConnect`.**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-07-13
- **Tasks:** 2/2
- **Files modified:** 10 (2 created, 8 modified)

## Accomplishments
- `internal/server/connectreseal.go`: `resealFunc` type + `newConnectResealInterceptor`, a best-effort interceptor that mutates `resp.Header()` in place via the injected reseal func on any successful response, and is a structural no-op on error, nil response, or nil reseal func.
- `internal/server/connectreseal_test.go`: interceptor-contract tests proving fire-on-success (read AND write, no allowlist by construction), request-cookie passthrough via the dummy-`*http.Request` trick, skip-on-error, skip-on-nil-response, and nil-reseal passthrough — no Qdrant, no httptest server.
- Wired `reseal resealFunc` as the new final parameter of `mountConnect` (appended last/innermost in `connect.WithInterceptors`) and of `Register`; `serve.go` assigns `connectReseal = webHandler.Reseal` inside the `uiCfg.Enabled` block and passes it into `server.Register`.
- Updated all 10 existing `mountConnect`/`Register` test call sites (5 files) to pass `nil` for the new arg, mirroring the established `csrfVerify` nil convention.

## Task Commits

Each task was committed atomically:

1. **Task 1: newConnectResealInterceptor + resealFunc type** - `e6a225d3` (feat)
2. **Task 2: Thread reseal through mountConnect → Register → serve.go** - `4b553caf` (feat)

## Files Created/Modified
- `internal/server/connectreseal.go` - `resealFunc` type + `newConnectResealInterceptor` factory (best-effort, innermost, no procedure allowlist)
- `internal/server/connectreseal_test.go` - interceptor-contract tests (fire-on-success read+write, cookie passthrough, skip-on-error/nil, nil-passthrough)
- `internal/server/connectapi.go` - `mountConnect` gains `reseal resealFunc` param; `newConnectResealInterceptor(reseal)` appended last in `WithInterceptors`; ordering comment extended
- `internal/server/tools.go` - `Register` gains trailing `reseal resealFunc` param, threaded into `d.mountConnect`
- `cmd/engram/serve.go` - `connectReseal` declared beside `connectCSRFVerify`, assigned `webHandler.Reseal` inside `uiCfg.Enabled`, passed to `server.Register`
- `internal/server/connectcsrf_test.go`, `connectapi_test.go`, `connectapi_negative_test.go`, `connectapi_cookie_test.go`, `tools_test.go` - all `mountConnect`/`Register` call sites pass `nil` for the new arg

## Decisions Made
- Confirmed `newConnectResealInterceptor` never calls `req.Spec()` at all (unlike CSRF's `csrfWriteProcedures[req.Spec().Procedure]` gate), so D-03 (fires on read AND write) holds by construction — tests exercise this with a read-shaped and a write-shaped request message rather than trying to set `connect.Request.Spec().Procedure` directly (that field has no exported setter on `*connect.Request[T]`, so the plan's literal "regardless of req.Spec().Procedure" framing is verified structurally instead of by setting the field).
- `webHandler.Reseal` (method value, type `func(http.Header, *http.Request)`) is assignable to the local `resealFunc` type by Go's structural func typing — confirmed by a successful `go build ./...`, closing RESEARCH Open Question 1.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] errorlint finding in Task 1's own test file, fixed while wiring Task 2**
- **Found during:** Task 2, running `task lint` before final verification
- **Issue:** `TestNewConnectResealInterceptor_SkipsOnError` compared `gotErr != wantErr` alongside `errors.Is`, which `golangci-lint`'s `errorlint` linter flags as unsafe for wrapped errors.
- **Fix:** Dropped the redundant `!= ` comparison; `errors.Is(gotErr, wantErr)` alone is both correct and lint-clean.
- **Files modified:** `internal/server/connectreseal_test.go`
- **Verification:** `task lint:go` → `0 issues`; `go test ./internal/server/... -count=1` still green.
- **Committed in:** `4b553caf` (Task 2 commit, since it surfaced during Task 2's lint gate)

---

**Total deviations:** 1 auto-fixed (1 bug/lint)
**Impact on plan:** Trivial lint-only fix to a Task 1 test file; no behavior change. No scope creep.

## Issues Encountered
- `connect.Request[T].Spec()` returns `Spec` by value with no exported setter, so `req.Spec().Procedure = proc` (as sketched in the plan's task description) does not compile. Resolved by driving two differently-typed request messages (`ListMemoriesRequest`, `StoreMemoryRequest`) instead, which still proves the no-allowlist property since the interceptor never reads `Spec()` regardless of request shape.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The reseal interceptor is fully wired end-to-end in the real Connect chain (verified by `go build ./...` and the full `internal/server` test suite, both green) and ready for a live-server / concurrency (SC3 forward-monotonic) regression test in a later plan of this phase.
- `task` (lint + test) and `task license:check` are clean for all files this plan touched.
- Mandatory `/gsd-secure-phase` review for this phase still applies per the phase's ROADMAP flag — this plan only wires the transport-side interceptor; it does not itself constitute the security sign-off.

## Self-Check: PASSED

All created/modified files verified present on disk; both task commits (`e6a225d3`, `4b553caf`) verified in git log.

---
*Phase: 18-stateless-session-rotation*
*Completed: 2026-07-13*
