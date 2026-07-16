---
phase: 18-stateless-session-rotation
fixed_at: 2026-07-13T15:20:00Z
review_path: .planning/phases/18-stateless-session-rotation/18-REVIEW.md
iteration: 1
findings_in_scope: 2
fixed: 2
skipped: 0
status: all_fixed
---

# Phase 18: Code Review Fix Report

**Fixed at:** 2026-07-13T15:20:00Z
**Source review:** .planning/phases/18-stateless-session-rotation/18-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 2 (WR-01, WR-02 — Critical + Warning scope; IN-01/IN-02 deferred, out of scope)
- Fixed: 2
- Skipped: 0

## Fixed Issues

### WR-01: `Reseal` omits the resolver's post-`Unseal` guards (version, hard-expiry, empty-owner)

**Files modified:** `internal/webauth/reseal.go`, `internal/webauth/reseal_test.go`
**Commit:** `5cb78734`
**Applied fix:** Added the three resolver-mirroring guards to `Handler.Reseal` immediately
after `Unseal` succeeds, before the threshold comparison: (1) `sess.V != sessionPayloadVersion`
rejects a legacy-version cookie (never laundered into a current-version session via `Seal`'s
auto-stamp), (2) `sess.Owner == ""` rejects an empty-owner payload (never re-issues a CSRF
token bound to `HMAC(k_csrf, "")`), and (3) `remaining <= 0` rejects an already-expired
session (never resurrected with a fresh full-TTL expiry — the Codex-HIGH mid-flight-expiry
TOCTOU path). A doc comment explains this is defense-in-depth mirroring `Resolver.Resolve`,
matching the `connectcsrf.go` D-05 precedent. `resealSkew` remains scoped only to the
threshold compare, never the hard-expiry check.

Added three regression tests to `reseal_test.go`, each proving `Reseal` is a no-op (no
`Set-Cookie` emitted) for an input otherwise well within the reseal threshold window, so
only the new guard — not the threshold — causes the skip:
- `TestResealNoopOnLegacyVersionCookie` — a `V==0` payload forged via the raw `sealBytes`
  bypass (same technique as `TestResolverRejectsLegacyVersionCookie`), since `Seal` always
  auto-stamps the current version and cannot itself produce a `V==0` cookie.
- `TestResealNoopOnExpiredCookie` — `Expiry = fixedNow.Add(-1*time.Minute)`, `remaining < 0`.
- `TestResealNoopOnEmptyOwnerCookie` — `Owner: ""`, near-expiry.

Verified: `go build ./internal/webauth/...`, `go vet`, and
`go test ./internal/webauth/... -count=1 -race` all green; `git diff internal/webauth/resolver.go`
is empty (resolver.go untouched, SC4 preserved).

### WR-02: No integration test proves the re-seal `Set-Cookie` survives connect-go's unary response path to the wire

**Files modified:** `internal/server/connectreseal_wire_test.go` (new file)
**Commit:** `659e63cb`
**Applied fix:** Added `TestConnectResealSetCookieReachesWire`, which mounts the real
interceptor chain (subject → reseal, innermost) over `httptest.NewServer` — mirroring
`connectapi_cookie_test.go`'s `TestConnectCookieLaneIsolation` pattern — wired with a
**real**, non-nil, non-spy `resealFunc`. Since `internal/server` does not import
`internal/webauth` (the two packages deliberately don't import each other — see
`CSRFCookieName`'s comment in `connectcsrf.go`), the test defines a small independent
AES-256-GCM `testSessionCodec` mirroring `webauth.SessionCodec`'s payload shape
(`{owner, exp, v}`) and algorithm, and a `resealFunc` closure performing the identical
unseal → guard → threshold-check → reseal → `Set-Cookie` flow `Handler.Reseal` performs.

The test seeds a near-expiry `engram_session` cookie (remaining ~1h, past the 6h
threshold) on a `ListMemories` request, issues the call through the real chain, and
asserts the raw wire response (`resp.Header().Values("Set-Cookie")` via the Connect
client) carries a refreshed `engram_session` (decrypted and checked for owner + advanced
expiry) and `engram_csrf` cookie. This closes the previously-unproven seam: whether
connect-go actually flushes an interceptor's `resp.Header()` writes onto a unary response.

Verified: `go build ./internal/server/...`, `go vet`, and
`go test ./internal/server/... -run TestConnectResealSetCookieReachesWire -count=1 -v`
pass (real Qdrant via testcontainers); full `go test ./internal/webauth/... ./internal/server/... -count=1`
green; `task lint:go` 0 issues; `task license:check` clean (SPDX header present on the
new file).

## Deferred (out of scope for this fix pass)

- **IN-01** (`internal/webauth/reseal_test.go:117-161`) — `TestResealForwardMonotonicUnderConcurrency`
  is near-tautological with a pinned clock; Info-severity, not fixed here.
- **IN-02** (`internal/webauth/reseal.go:11-14`) — the "~once per 6h" churn-bound comment is
  optimistic under burst concurrency; Info-severity, not fixed here.

---

_Fixed: 2026-07-13T15:20:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
