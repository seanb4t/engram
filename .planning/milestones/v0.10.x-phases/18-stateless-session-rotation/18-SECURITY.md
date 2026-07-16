---
phase: 18
slug: stateless-session-rotation
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-07-13
---

# Phase 18 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> State B run: register authored at plan time (18-01-PLAN.md and 18-03-PLAN.md
> carried `<threat_model>` STRIDE blocks; 18-02-PLAN.md carried the ADR's own
> accept-disposition entry for T-18-01), ASVS L1 → L1 grep/read-depth
> verification. Register built by extracting and deduplicating the
> `<threat_model>` blocks from 18-01/18-02/18-03 PLAN.md (8 unique threat IDs
> referenced across the three plans), then verifying each declared mitigation
> against the ACTUAL implemented code and live test execution — not against
> SUMMARY.md claims or plan intent. A `/gsd-code-review --depth=deep --codex
> --opencode --fix` pass ran between implementation and this audit (18-REVIEW.md
> / 18-REVIEW-FIX.md): it found one real defense-in-depth gap (WR-01,
> corroborated HIGH by Codex) directly bearing on T-18-03/SC4's hard-expiry
> guarantee, and it was fixed (commit `5cb78734`) and re-verified here against
> the current source, not accepted on the review report's word.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|----------------|
| browser → Connect server | The AES-GCM sealed `{owner, expiry}` session cookie (untrusted, client-held) crosses here on every Connect request; `Reseal` reads and re-issues it | sealed session cookie value |
| Connect interceptor chain → handler → interceptor chain | The reseal interceptor sits innermost, only touching a response that has already passed subject(401)/CSRF(403)/validate(400) | `connect.AnyResponse.Header()` |
| operator → `ENGRAM_UI_COOKIE_KEY` | The sole kill-switch for a stolen, actively-resealed cookie: rotating this key invalidates every sealed cookie at once | AES-256 key material |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-18-01 | Elevation of Privilege / Repudiation | sliding re-seal extends an actively-used stolen cookie indefinitely | high | accept | `docs/adr/engram-slr8-stateless-sliding-session-reseal.md` — `Status: Accepted`; Consequences section states prominently ("Negative (accepted risk — state prominently)") that a stolen sealed cookie never self-expires while actively re-sealed, and names the real kill-switch `ENGRAM_UI_COOKIE_KEY` (`ui.cookie_key`, `internal/config/registry.go:56`) — the phantom `ENGRAM_SESSION_KEY` from the ROADMAP prose does not appear anywhere in the ADR (grep confirmed, no match) or elsewhere in the phase's touched files. Documented as a detection/response gap, not a preventable one. See Accepted Risks Log AR-18-01 | closed |
| T-18-02 | Tampering | `Handler.Reseal` expiry computation | medium | mitigate | `internal/webauth/reseal.go:70-73` computes the fresh session as `Session{Owner: sess.Owner, Expiry: nowUTC().Add(sessionTTL)}` — always absolute, never `oldExpiry+delta`. `TestResealForwardMonotonicUnderConcurrency` (`reseal_test.go:118-162`) pins `nowUTC`, launches 50 goroutines re-sealing the same near-expiry cookie, and asserts every decoded `Expiry` equals exactly `fixedNow.Add(sessionTTL)` and none is before the pre-reseal expiry. Live-verified: `go test ./internal/webauth/... -run Reseal -race -count=1 -v` — 7/7 PASS, no data race | closed |
| T-18-03 | Elevation of Privilege | `resealSkew` leaking into `resolver.go`'s hard-expiry check | high | mitigate | `resealSkew` (`reseal.go:22`) is referenced exactly once in the whole `internal/webauth` package tree, inside `Reseal`'s threshold comparison (`reseal.go:67`) — `rg -n resealSkew internal/webauth/resolver.go` returns zero matches. `git diff origin/main..HEAD -- internal/webauth/resolver.go` is empty: the hard-expiry check (`resolver.go:49-51`, `if sess.Expiry.IsZero() \|\| nowUTC().After(sess.Expiry)`) is byte-for-byte unchanged from the pre-phase baseline. `TestResolveHardExpiryHasNoSkewTolerance` (`resolver_test.go:122-132`) seals a session expired by exactly 1ns — well inside the 60s `resealSkew` budget — and asserts `Resolve` still rejects it. Additionally closes the WR-01/Codex-HIGH finding: `Reseal` itself now independently guards `remaining <= 0` (`reseal.go:67-68`) before re-sealing, so a session that expires in the narrow TOCTOU window between the resolver's request-start check and the innermost reseal call can no longer be resurrected with a fresh full-TTL cookie — pinned by `TestResealNoopOnExpiredCookie` (`reseal_test.go:209-229`). Live-verified: all four tests PASS | closed |
| T-18-04 | Information Disclosure | `Reseal` error paths | medium | mitigate | `Reseal` (`reseal.go:46-79`) is void-return; every failure path (`r.Cookie` miss, `Unseal` error, version/owner/expiry guard reject, `Seal` error) is a silent `return` with no `slog` call anywhere in the function body (`rg -n 'slog' internal/webauth/reseal.go` — zero matches) — no owner value or session-identity detail is ever logged on a Reseal failure path. The interceptor (`connectreseal.go`) adds no error surface of its own | closed |
| T-18-05 | Tampering | reseal interceptor ordering | medium | mitigate | `newConnectResealInterceptor(reseal)` is the literal last entry in `mountConnect`'s `connect.WithInterceptors(...)` list (`connectapi.go:361-368`), appended after `newConnectValidateInterceptor` — confirmed by direct read, not inference. The interceptor body (`connectreseal.go:38-42`) branches `if err != nil \|\| resp == nil \|\| reseal == nil { return resp, err }` before ever touching `resp.Header()`, so it structurally cannot fire on a rejected/errored upstream call. `TestNewConnectResealInterceptor_SkipsOnError` and `_SkipsOnNilResponse` (`connectreseal_test.go`) both PASS | closed |
| T-18-06 | Denial of Service | `engram_csrf` cookie lapse mid-session | medium | mitigate | `reseal.go:77-78` calls both `h.setCookie(...sessionCookieName...)` and `h.setReadableCookie(...CSRFCookieName...)` in the same past-threshold branch — one `Reseal` call always refreshes both cookies' `Max-Age` together. `TestResealPastThresholdRefreshesCSRFCookie` asserts exactly 2 `Set-Cookie` headers are emitted and the CSRF cookie's value equals the unchanged `h.signer.Token(owner)` (D-08: value stable, only `Max-Age` refreshes). `TestConnectResealSetCookieReachesWire` (`connectreseal_wire_test.go`) additionally proves both cookies reach the real HTTP wire response over a live `httptest.NewServer` chain, not just the in-memory `http.Header{}` used by the unit test | closed |
| T-18-07 | Denial of Service | reseal failure surfacing as a client error | medium | mitigate | The interceptor (`connectreseal.go:53`) always `return resp, nil` on the success path regardless of what `reseal(...)` did internally — there is no error path back from `reseal` into the interceptor at all, because `resealFunc`'s signature (`func(http.Header, *http.Request)`) has no return value. `Reseal` itself (`reseal.go:46`) is likewise void-return. A transient seal failure (bad key, marshal error) is structurally incapable of failing the RPC. `TestNewConnectResealInterceptor_NilResealIsPassthrough` further proves a nil reseal func is a safe no-op, not a panic | closed |
| T-18-SC | Tampering | package-manager installs | low | accept | Zero new Go dependencies across all three plans in this phase — confirmed via `git diff origin/main..HEAD -- go.mod go.sum` (empty diff). All new code (`reseal.go`, `headerOnlyWriter`, `connectreseal.go`, `connectreseal_wire_test.go`'s independent test-only AES-GCM codec) uses only stdlib (`crypto/aes`, `crypto/cipher`, `net/http`) and already-vendored `connectrpc.com/connect`. See Accepted Risks Log AR-18-02 | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `high` count toward `threats_open`*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|--------------|------|
| AR-18-01 | T-18-01 | Sliding-expiry re-seal is a deliberate design choice (D-01 through D-08, `18-CONTEXT.md`) that trades per-session revocability for statelessness (honoring the foundational `engram-u9v` decision). An actively-used stolen cookie is re-sealed with a fresh 12h expiry every time it crosses the 6h remaining-lifetime threshold, so it never expires on its own while the attacker keeps using it. This is accepted because: (1) the alternative (server-side session store / revocation) reverses `DEC-u9v`/`engram-8q3` and needs its own ADR, explicitly out of scope for this milestone; (2) the ONLY kill-switch — operator rotation of `ENGRAM_UI_COOKIE_KEY` — is documented prominently in `docs/adr/engram-slr8-stateless-sliding-session-reseal.md`'s Consequences section, not buried; (3) the blast radius is bounded to the web-UI cookie lane only (MCP's bearer-token transport is unaffected, has no session cookie to slide). Pinned by the ADR's explicit "Negative (accepted risk — state prominently)" framing. | Sean (via `/gsd-secure-phase 18`) | 2026-07-13 |
| AR-18-02 | T-18-SC | Zero new external packages across all three plans in this phase — confirmed via `git diff origin/main..HEAD -- go.mod go.sum` (empty diff; no commit in this phase's history touches `go.mod`/`go.sum`). All new code (the re-seal primitive, the `headerOnlyWriter` shim, the Connect interceptor, and the WR-02 wire-test's independent test-only session codec) uses only Go stdlib crypto/net primitives and already-vendored dependencies. | Sean (via `/gsd-secure-phase 18`) | 2026-07-13 |

*Accepted risks do not resurface in future audit runs.*

---

## Code-Review Convergence Note (pre-existing gap, now closed)

`18-REVIEW.md` (deep + Codex cross-AI) found **WR-01**: the original `Reseal`
trusted interceptor ordering and omitted the resolver's post-`Unseal` guards
(payload version, hard expiry, empty owner). Codex independently rated the
missing hard-expiry lower bound **HIGH** as a concrete mid-flight-expiry TOCTOU
path — a session valid when the subject interceptor resolved it could expire
before the innermost `Reseal` call and be resurrected with a fresh full-TTL
cookie, a real (if narrow) breach of the T-18-03/SC4 "hard expiry stays strict"
property. `18-REVIEW-FIX.md` records the fix (commit `5cb78734`): three guards
mirroring `Resolver.Resolve` were added to `Reseal` — `sess.V !=
sessionPayloadVersion`, `sess.Owner == ""`, and `remaining <= 0` — each pinned
by a dedicated regression test. This audit independently re-verified the fix is
present in the current source (`reseal.go:63-69`) and green under
`go test ./internal/webauth/... -run Reseal -race -count=1` and `go test
./internal/server/... -run Reseal -race -count=1` (the latter including the
WR-02 real-wire integration test, `TestConnectResealSetCookieReachesWire`) —
it is not accepted on the review/fix report's word alone.

---

## Unregistered Flags

None. All three `18-0N-SUMMARY.md` files were checked for a `## Threat Flags`
section (`rg -ni "threat" 18-0*-SUMMARY.md`) — no matches in any of the three.
No new attack surface was flagged by the executor during implementation beyond
the plan-time STRIDE register.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open (blocking) | Open (non-blocking) | Run By |
|------------|----------------|--------|-------------------|------------------------|--------|
| 2026-07-13 | 8 | 8 | 0 | 0 | gsd-secure-phase (L1 grep/read-depth; register authored at plan time across three PLAN.md, deduplicated by threat ID; every `mitigate` disposition verified against live source code on the `phase-18-stateless-session-rotation` branch and live test execution — `go build ./...`, `go vet ./...`, `go test ./internal/webauth/... ./internal/server/... -count=1` all green, plus targeted `-race` runs of `Reseal`/`HardExpiry`/interceptor-contract tests, all green including the real-wire WR-02 integration test; every `accept` disposition recorded in the Accepted Risks Log above; `git diff origin/main..HEAD` confirmed empty for both `internal/webauth/resolver.go` and `go.mod`/`go.sum`) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-13
