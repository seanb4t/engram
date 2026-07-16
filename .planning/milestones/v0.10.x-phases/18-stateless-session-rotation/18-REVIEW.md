---
phase: 18-stateless-session-rotation
reviewed: 2026-07-13T19:04:17Z
depth: deep
files_reviewed: 16
files_reviewed_list:
  - internal/webauth/reseal.go
  - internal/webauth/reseal_test.go
  - internal/webauth/resolver_test.go
  - internal/server/connectreseal.go
  - internal/server/connectreseal_test.go
  - internal/server/connectapi.go
  - internal/server/tools.go
  - internal/server/connectapi_test.go
  - internal/server/connectapi_negative_test.go
  - internal/server/connectapi_cookie_test.go
  - internal/server/connectcsrf_test.go
  - internal/server/tools_test.go
  - cmd/engram/serve.go
  - docs/adr/engram-slr8-stateless-sliding-session-reseal.md
  - docs/adr/README.md
findings:
  critical: 0
  warning: 2
  info: 2
  total: 4
status: issues_found
---

# Phase 18: Code Review Report

**Reviewed:** 2026-07-13T19:04:17Z
**Depth:** deep (cross-file: import graph + call chains)
**Files Reviewed:** 16
**Status:** issues_found

## Summary

Phase 18 adds a stateless sliding-expiry session re-seal: `webauth.Handler.Reseal`
re-parses the AES-GCM `{owner,expiry}` cookie and, once remaining lifetime drops
below `resealThreshold(6h)+resealSkew(60s)`, re-seals with a fresh **absolute**
`nowUTC().Add(sessionTTL)` expiry plus a Max-Age-refreshed `engram_csrf` cookie.
`newConnectResealInterceptor` wires it innermost (after validate), best-effort, for
both read and write RPCs.

The core mechanism is **correct for the shipped interceptor chain** and holds the
design contract on every point I traced:

- `headerOnlyWriter` correctly emits `Set-Cookie`: `http.SetCookie` calls
  `w.Header().Add(...)`, and the shim's `Header()` returns the response header map
  (a Go map, mutated in place). Verified by `findSetCookie` in `reseal_test.go`.
- Absolute `now+TTL` (never `old+delta`) is applied at `reseal.go:61`; no shared
  mutable state, so SC3 race-safety holds by construction.
- Interceptor short-circuits correctly: `resp==nil || err!=nil || reseal==nil` all
  skip; the subject interceptor (`connectauth.go:22-24`) returns `(nil, err)` on an
  unauthenticated caller, so `resp!=nil` genuinely implies an authorized request.
- Absent cookie (`reseal.go:47-50`) and tampered/wrong-key cookie
  (`Unseal` fail, `reseal.go:51-54`) are silent no-ops — no new session minted.
- Cookie attribute parity is exact: `Reseal` reuses `setCookie`/`setReadableCookie`,
  the same methods `Callback` uses at login (`handlers.go:85-116,181-186`).
- SC4 holds: `resealSkew` is applied only to the threshold compare (`reseal.go:56`),
  never to the resolver hard-expiry check (`resolver.go:49`), and
  `TestResolveHardExpiryHasNoSkewTolerance` pins that at 1ns.
- No info leak: `Reseal` logs nothing; the resolver's version-reject log
  (`resolver.go:60`) is operator-side only and the client string stays generic.
- Kill-switch is correctly `ENGRAM_UI_COOKIE_KEY` (ADR + `serve.go:146`); no new
  `ENGRAM_` var introduced.

The two warnings are both **defense-in-depth / test-coverage gaps**, not
production-reachable failures in the current wiring — but they matter because this
is a security feature whose correctness rests entirely on interceptor ordering, and
the codebase elsewhere (D-05 in `connectcsrf.go`) deliberately hardens against that
same fragility.

## Warnings

### WR-01: `Reseal` omits the resolver's post-`Unseal` guards (version, hard-expiry, empty-owner), diverging from the codebase's own defense-in-depth standard

**File:** `internal/webauth/reseal.go:51-62`

**Issue:** After `Unseal`, `Reseal` re-seals based solely on the elapsed threshold.
It does **not** replicate the three guards `Resolver.Resolve` applies to the same
payload (`resolver.go:49-66`):

1. **Version.** `SessionCodec.Unseal` (`session.go:85-95`) does not check
   `sess.V`. A legacy/`v==0` cookie that decrypts with the current key unseals
   successfully, so `Reseal` would re-`Seal` it — and `Seal` auto-stamps
   `V=sessionPayloadVersion` (`session.go:74`), **laundering a legacy cookie into a
   valid current-version session** and defeating the T-17-14 rollout-invalidation
   seam that `resolver.go:59-63` and `TestResolverRejectsLegacyVersionCookie` exist
   to enforce. The task contract states "version-mismatched cookie — Reseal must NOT
   mint a NEW session"; today only the resolver upholds that.

2. **Hard expiry.** `remaining := sess.Expiry.Sub(nowUTC())` goes **negative** for an
   already-expired session, and negative is `< resealThreshold+resealSkew`, so
   `Reseal` would resurrect a dead cookie with a fresh full-TTL expiry. There is no
   lower bound (`remaining <= 0`) guard.

3. **Empty owner.** No `sess.Owner == ""` check before re-issuing the CSRF token
   `HMAC(k_csrf, "")`.

Currently all three are **gated upstream** by the subject interceptor: `Reseal` is
innermost, so a cookie the resolver rejects (bad version / expired / empty owner)
errors at `connectauth.go:22`, `next()` returns `(nil, err)`, and the reseal
interceptor skips (`connectreseal.go:40`). So this is **not production-reachable in
the shipped chain** — hence Warning, not Blocker. But it becomes a live bug under
(a) any future interceptor-reorder, (b) any direct/first-party call to the exported
`Handler.Reseal`, or (c) the narrow TOCTOU window where a session valid at the
resolver check (`resolver.go:49`) expires before the innermost reseal check.
`connectcsrf.go:52-71` explicitly adds exactly this kind of redundant re-check
"against a future interceptor-ordering regression"; `Reseal` is the one seam that
does not, and no test asserts its behavior on a legacy or expired cookie.

**Fix:** mirror the resolver's guards so `Reseal` fails closed on the same inputs:

```go
sess, err := h.codec.Unseal(c.Value)
if err != nil {
    return
}
// Defense-in-depth: never re-seal a payload the resolver would reject.
// Reseal must not resurrect an expired session, launder a legacy-version
// cookie into the current version, or re-issue a CSRF token for an empty owner.
if sess.V != sessionPayloadVersion || sess.Owner == "" {
    return
}
remaining := sess.Expiry.Sub(nowUTC())
if remaining <= 0 || remaining >= resealThreshold+resealSkew {
    return // expired (hard, zero-skew) or not yet due
}
```

### WR-02: No integration test proves the re-seal `Set-Cookie` survives connect-go's unary response path to the wire

**File:** `internal/server/connectreseal_test.go` (whole file); `internal/server/connectapi_cookie_test.go:58`

**Issue:** The whole feature depends on connect-go actually flushing
`resp.Header()` (mutated at `connectreseal.go:50`) as real HTTP `Set-Cookie`
response headers for a unary Connect call. Nothing tests that end-to-end:

- `connectreseal_test.go` drives the interceptor with a **spy** `resealFunc` and a
  hand-built `connect.NewResponse` — it never boots an HTTP server, so it proves the
  interceptor calls reseal but not that the header reaches the wire.
- `reseal_test.go` exercises the real `Reseal` against a **bare `http.Header{}`** —
  it proves `headerOnlyWriter` populates a map, not that connect-go serializes that
  map onto the response.
- `connectapi_cookie_test.go`, the only test that boots the real chain over
  `httptest.NewServer`, passes `nil` for reseal (`:58`).

So the single most load-bearing integration seam — "does a unary Connect response
emit the `Set-Cookie` headers an interceptor set on `resp.Header()`?" — is unproven.
If connect-go dropped or relocated those headers, every test would still pass while
the feature silently no-ops (sessions would never actually slide).

**Fix:** add one integration test that mounts the real chain over
`httptest.NewServer` with a **real** codec-backed `webauth.Handler.Reseal` (seed a
near-expiry cookie, past the 6h threshold), issue a `ListMemories`, and assert the
raw `http.Response.Header["Set-Cookie"]` contains a refreshed `engram_session` (and
`engram_csrf`) with the expected absolute expiry. This also closes the D-03
read-path claim with a live wire assertion rather than the structural
"never-inspects-Spec()" argument in `TestNewConnectResealInterceptor_FiresOnSuccess`.

## Info

### IN-01: Concurrency test is near-tautological — proves independence, not race-safety of any shared write

**File:** `internal/webauth/reseal_test.go:117-161`

**Issue:** `TestResealForwardMonotonicUnderConcurrency` pins `nowUTC` to a fixed
instant and gives each goroutine its **own** `http.Header{}`. With `now` frozen,
every goroutine deterministically produces the identical `fixedNow.Add(sessionTTL)`,
so `e.Equal(want)` is trivially true and the "forward-monotonic" assertion cannot
fail. Because there is no shared mutable state written concurrently, `-race` has
nothing to flag — the SC3 property is true by construction, but this test does not
demonstrate it (a genuine race would require concurrent writers to one shared
header/expiry, which the design correctly never has). The test is a fine smoke test
but should not be read as evidence that a shared-state race was ruled out.

**Fix:** either drop the concurrency framing (rename to reflect it proves
idempotent independence), or, to make `-race` meaningful, have the goroutines share
a single response `http.Header` and assert the resulting `Set-Cookie` set is
internally consistent — acknowledging that with pinned `now` all values are equal by
design.

### IN-02: "at most ~once per 6h" churn bound is optimistic under burst concurrency

**File:** `internal/webauth/reseal.go:11-14`

**Issue:** The comment claims re-seal is bounded to "~once per 6h of continuous
activity." That holds serially, but a burst of N concurrent authenticated requests
all still carrying the same pre-reseal (near-expiry) cookie each emit a re-seal
`Set-Cookie` on their own response, because none sees the refreshed cookie until the
browser applies the first one. The extra `Set-Cookie` headers are harmless
(idempotent — every one is the same absolute `now+TTL`), but the bound is
per-cookie-generation, not strictly per-6h-wall-clock.

**Fix:** soften the comment to "at most once per cookie generation (~once per 6h of
serial activity)" so the invariant matches observable behavior.

## Cross-AI Corroboration (`--codex --opencode`)

This phase was additionally reviewed by two external AIs on the same source diff.

- **Codex (codex-cli 0.144.1) — CONFIRMS WR-01, rates it higher.** Independently
  found the same two guard gaps at `reseal.go:55`:
  - **HIGH** — "Missing hard-expiry guard re-seals dead sessions; a request
    authenticated just before expiry that completes afterward receives a fresh
    12-hour cookie." This is a concrete TOCTOU reachability path for WR-01 point 2
    (the resolver checks expiry at request *start*; a session that lapses before the
    innermost reseal check gets resurrected with a full fresh TTL — a real, if
    narrow, breach of "hard expiry stays strict", SC4).
  - **MEDIUM** — "Reseal omits payload-version and empty-owner validation; a
    directly supplied decryptable legacy/version-mismatched cookie is upgraded to a
    valid current-version session" (= WR-01 points 1 & 3).
  - No BLOCKER/LOW findings. Codex confirmed no other production-reachable bug.
- **opencode (1.17.15, grok-4.5) — no usable result.** Exited 0 but only its preamble
  reached stdout (the known grok stdout-flush failure under heavy tool use; see engram
  memory `xgehkt9pad`). Its stderr trace shows it *did* read the real net/http cookie
  code, but no findings summary was captured. Not counted.

**Convergence verdict:** two independent reviewers (Claude deep + Codex) flag the
**same WR-01 defense-in-depth gap**, with Codex elevating the missing hard-expiry
lower-bound to HIGH on a concrete mid-flight-expiry reachability path. Recommendation:
**land the WR-01 fix (the three resolver-mirroring guards, including `remaining <= 0`)
before `/gsd-secure-phase 18`** — it hardens the exact "hard expiry stays strict"
property (SC4) at the one seam that currently trusts interceptor ordering, matching the
`connectcsrf.go` D-05 precedent. WR-02 (add a real-server `Set-Cookie`-on-the-wire
test) is a strong follow-up. IN-01/IN-02 are optional polish.

---

_Reviewed: 2026-07-13T19:04:17Z_
_Reviewer: Claude (gsd-code-reviewer), deep — corroborated by Codex 0.144.1 (opencode/grok flush-failed)_
_Depth: deep + cross-AI (`--codex --opencode`)_
