---
phase: 01-shared-auth-chain-connect-bearer-identity
reviewed: 2026-07-31T00:00:00Z
depth: standard
files_reviewed: 33
files_reviewed_list:
  - Taskfile.yaml
  - charts/engram/templates/_helpers.tpl
  - charts/engram/values.yaml
  - cmd/engram/serve.go
  - cmd/engram/serve_test.go
  - docs-site/src/content/docs/guides/configure.md
  - internal/auth/bearer.go
  - internal/auth/bearer_test.go
  - internal/config/config.go
  - internal/config/config_test.go
  - internal/config/connect_test.go
  - internal/config/registry.go
  - internal/config/service_auth_test.go
  - internal/config/validate.go
  - internal/config/validate_test.go
  - internal/server/connectapi.go
  - internal/server/connectapi_bearer_parity_test.go
  - internal/server/connectapi_cookie_test.go
  - internal/server/connectapi_negative_test.go
  - internal/server/connectapi_service_auth_parity_test.go
  - internal/server/connectapi_test.go
  - internal/server/connectauth.go
  - internal/server/connectauth_test.go
  - internal/server/connectbearer.go
  - internal/server/connectbearer_test.go
  - internal/server/connectcsrf.go
  - internal/server/connectcsrf_lane_test.go
  - internal/server/connectcsrf_test.go
  - internal/server/connectmount_test.go
  - internal/server/connectreseal.go
  - internal/server/connectreseal_test.go
  - internal/server/connectreseal_wire_test.go
  - internal/server/connectbearer.go
  - internal/server/identity.go
findings:
  critical: 0
  warning: 1
  info: 1
  total: 2
status: issues_found
---

# Phase 01: Code Review Report

**Reviewed:** 2026-07-31T00:00:00Z
**Depth:** standard
**Files Reviewed:** 33
**Status:** issues_found

## Summary

This phase wires bearer-token authentication into the ConnectRPC lane
(`auth.Lane`, `auth.EnforceExpiry`, `server.NewConnectResolver`, the
lane-keyed CSRF exemption and reseal gate, and the single-construction
verifier chain in `cmd/engram/serve.go`). I read every file in scope and
adversarially attacked the six load-bearing security properties called out
for this phase:

1. **Lane provenance is server-set, never request-derived.** Confirmed:
   `NewConnectResolver`'s closure decides the lane exclusively by which half
   answered, stamps it once via `withConnectLane`, and both the CSRF
   interceptor and the reseal interceptor read it back through
   `laneFromConnectContext` — never re-parsing `Authorization` or `Cookie`.
   `TestCSRFCookieCallerCannotSelfDeclareBearerLane` and
   `TestCSRFFailedBearerNeverFallsThroughToExemption`
   (`connectcsrf_lane_test.go`) exercise this end-to-end through the *real*
   composed resolver, not a stub, and both pass for the right reason.
2. **Fail-closed on unknown/absent lane.** `newConnectCSRFInterceptor`'s
   `switch` has an explicit `default` that rejects with
   `CodePermissionDenied` for any lane other than the two named constants;
   `newConnectResealInterceptor` similarly only fires on the exact
   `auth.LaneCookie` value (`!=` comparison, not a negated allowlist).
   `TestCSRFLaneUnstampedFailsClosed` / `TestResealSkippedForUnknownLane`
   cover this.
3. **Bearer failure never falls through to cookie.** Confirmed in
   `NewConnectResolver`: a well-formed bearer credential commits
   exclusively to `verifyBearerCredential`; its error returns immediately
   with `LaneUnknown`, and `cookieResolve` is provably never invoked
   (`TestBearerFailureNeverFallsThroughToCookie` and the
   `cookieCalls`/`sawAuthHeader` counters in the lane tests are real
   assertions, not vacuous ones).
4. **Expiry enforcement.** `auth.EnforceExpiry` rejects a zero
   `Expiration` and rejects `Expiration.Before(time.Now())` with no grace
   window — this exactly matches the go-sdk's own hard-expiry check
   (`auth/auth.go`, confirmed by reading the vendored source: both use
   `Expiration.Before(time.Now())`), so this is intentional parity, not a
   boundary bug. The nil-`TokenInfo` forward path is covered and does not
   panic (`TestEnforceExpiryNilTokenInfoIsForwardedNotDereferenced`).
5. **Error-text wire contract.** `invalidTokenError` wraps the sentinel
   without `errors.Join`, so `Error()` stays exactly `"token expired"` /
   `"token missing expiration"`, and `Unwrap() []error` still satisfies
   `errors.Is(err, mcpauth.ErrInvalidToken)`. Verified byte-for-byte on the
   MCP wire via `TestBearerLaneParityRejectionBodiesMatch`. See WR-01 below
   for a gap in the equivalent Connect-side assertion.
6. **`mountConnect`'s `resolve == nil` gate.** Untouched: a single equality
   check, no `OR`/`AND` loosening, and `connectResolverFor` keeps the mount
   decision and the bearer-inclusion decision as two independently-tested
   booleans (`TestConnectResolverForDefaultOff`,
   `TestMountConnectDefaultOffWithoutUIOrHeadlessFlag`).

I found no BLOCKER-level defect against these properties: every negative
path I could construct (self-declared bearer lane on a cookie-authenticated
write, unstamped lane on a write RPC, a well-formed-but-rejected bearer
falling back to a valid cookie, headless mount with no configured auth
lane) is independently rejected in the implementation and is backed by a
non-vacuous test that fails when the guard is removed (`TestBearerLaneExemptFromCSRF`'s
docstring records the actual pre-fix red result). `internal/config`'s
`connect.headless` wiring (registry → `Load` → `Validate` →
`connectHeadlessGuard` → `connectResolverFor`) is consistent end-to-end and
matches `docs-site/.../configure.md`'s description of the same behavior.

## Warnings

### WR-01: No test asserts the Connect-lane wire body text for an expired/zero-expiration bearer token

**File:** `internal/server/connectapi_bearer_parity_test.go:189-242` (missing coverage; compare to the MCP-side assertion at `connectapi_bearer_parity_test.go:251-294`)
**Issue:** Property #5 (this phase's own stated rationale for `invalidTokenError`) is that the exact rejection string reaching the wire must never regress into a doubled `errors.Join` message. `TestBearerLaneParityRejectionBodiesMatch` proves this **only for the MCP transport** — it drives `mcpauth.RequireBearerToken` directly and asserts `body == "token expired"` / `body == "token missing expiration"` on the raw HTTP response. The Connect-lane counterparts
(`TestBearerLaneParityRejectsExpiredOnBothLanes`,
`TestBearerLaneParityRejectsZeroExpirationOnBothLanes`) only assert
`connect.CodeOf(connErr) == connect.CodeUnauthenticated` — they never
inspect `connErr.(*connect.Error).Message()`. `connect.NewError(code, err).Message()`
is `err.Error()` under the hood (connectrpc.com/connect@v1.19-1.20's
`error.go`), so today's Connect body almost certainly still matches, but a
future regression on the Connect-only path (e.g. someone wraps the
resolver error with `fmt.Errorf("...: %w: %w", ...)` or `errors.Join`
inside `newConnectSubjectInterceptor`, `NewConnectResolver`, or
`verifyBearerCredential`) would silently corrupt the Connect wire body
while every existing test — including this one — kept passing. This is
exactly the "test that cannot actually fail for the property it claims to
guard" pattern the review brief calls out, on the lane this phase adds.
**Fix:** Add an assertion in `TestBearerLaneParityRejectionBodiesMatch` (or
a sibling test) that inspects the `*connect.Error` returned by
`bearerParityStoreMemory` for the expired/zero-expiration cases and checks
`connErr.(*connect.Error).Message() == "token expired"` /
`"token missing expiration"`, mirroring the exact-equality (not
`Contains`) discipline already used on the MCP side:
```go
var cerr *connect.Error
if errors.As(connErr, &cerr) {
    if cerr.Message() != "token expired" {
        t.Errorf("Connect wire message = %q, want exactly %q", cerr.Message(), "token expired")
    }
}
```

## Info

### IN-01: `wantNamespacedOwner` test oracle is duplicated verbatim across two packages

**File:** `cmd/engram/serve_test.go:31-36` and `internal/server/connectapi_service_auth_parity_test.go:19-29`
**Issue:** Both files independently reimplement the same
`fmt.Sprintf("%d:%s:%d:%s", ...)` oracle for `auth`'s namespaced-owner
encoding, each with its own doc comment restating the same contract. This
doesn't affect either test's reliability today, but the encoding is a
security-relevant format (it disambiguates a static-token owner from a
human email owner in the authz key), and having two independently-written
copies means a future encoding change has two silent places to miss rather
than one shared helper to update.
**Fix:** If a shared test-only helper package is acceptable for this
project's layering, hoist `wantNamespacedOwner` there; otherwise leave a
cross-reference comment in each copy pointing at the other so a change to
one prompts a check of the sibling.

---

_Reviewed: 2026-07-31T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
