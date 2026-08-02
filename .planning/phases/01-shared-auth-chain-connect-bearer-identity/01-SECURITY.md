---
phase: 1
slug: shared-auth-chain-connect-bearer-identity
status: complete
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-02
---

# v0.12.x Phase 1 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Retroactive audit — the phase shipped before `/gsd-secure-phase` was run for this milestone.

This is the milestone's most security-critical phase: it opens the ConnectRPC lane to bearer
credentials and makes the CSRF exemption depend on lane provenance.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Client → Connect lane | A caller presents either an `Authorization: Bearer` credential or a session cookie | Bearer token or session cookie; CSRF double-submit token on writes |
| Connect resolver → verifier chain | One composed `mcpauth.TokenVerifier` built once, injected into both mount sites | Token → verified identity (`TokenInfo`) |
| Interceptor → handler | Lane provenance is stamped server-side on a dedicated context key | `auth.Lane` (never client-supplied) |
| Process boot → mount decision | `connect.headless` and the configured chain decide whether Connect mounts at all | Config booleans |

---

## Threat Register

Authored at plan time across `01-01`–`01-04-PLAN.md` — **30 STRIDE rows** (10 + 5 + 9 + 6),
including four per-plan supply-chain rows. Verified retroactively by `gsd-security-auditor` on
2026-08-02; all 30 were checked, none skipped.

| Threat ID | Category | Severity | Disposition | Mitigation / Evidence | Status |
|-----------|----------|----------|-------------|------------------------|--------|
| T-01-01 | Spoofing / Tampering | critical | mitigate | CSRF exemption keyed **exclusively** on `laneFromConnectContext(ctx)` (`connectcsrf.go:79-86`). Negative source gates confirmed live on non-comment lines: zero `Authorization` reads, zero `Peer()`/`HTTPMethod`/`Content-Type` reads — no caller-controlled signal reaches the decision. Write-gate (`:62`) precedes the lane read (`:79`). `TestBearerLaneExemptFromCSRF`, `TestCSRFLaneUnstampedFailsClosed` PASS | closed |
| T-01-02 | Elevation of Privilege | critical | mitigate | A well-formed bearer credential commits structurally to the bearer lane; the cookie half is provably never invoked on bearer failure (`connectbearer.go:88-105`). `TestBearerFailureNeverFallsThroughToCookie` (zero call counter) PASS | closed |
| T-01-03 | Tampering / EoP | high | mitigate | `EnforceExpiry` (`internal/auth/bearer.go:141-167`) rejects zero and past `Expiration` with zero clock skew. `TestEnforceExpiry`, `TestEnforceExpiryZero`, `TestEnforceExpiryNoSkew` PASS | closed |
| T-01-04 | Elevation of Privilege | high | mitigate | Default arm of the lane switch returns `CodePermissionDenied` with **no CSRF check attempted** (`connectcsrf.go:84-86`), so an interceptor-ordering fault surfaces instead of silently succeeding. `TestCSRFLaneUnstampedFailsClosed` supplies valid CSRF material and is still rejected | closed |
| T-01-05 | Information Disclosure | medium | mitigate | Fixed literal rejection strings only; the single `err.Error()` occurrence in `connectcsrf.go` is in a comment, not code. No credential-bearing `slog` call in `bearer.go`/`connectbearer.go`. `TestEnforceExpiryMessagesMatchSDK` PASS | closed |
| T-01-06 | Repudiation | low | accept | See AR-01-01 | closed |
| T-01-07 | Denial of Service | medium | mitigate | A contract-violating `(nil, nil)` verifier return is forwarded unchanged before any field access (`bearer.go:147-157`), with an independent nil guard at `connectbearer.go:36-40`. `TestEnforceExpiryNilTokenInfoIsForwardedNotDereferenced`, `TestConnectBearerRejectsNilTokenInfo` PASS, no panic | closed |
| T-01-08 | Spoofing | high | mitigate | The credential header is read **once** and parsed **once** per request, and the extracted token — not the request — is handed to the verifier, so no TOCTOU seam exists between the lane decision and the verification it authorizes (`connectbearer.go:89`) | closed |
| T-01-09 | Elevation of Privilege | medium | accept | See AR-01-02 | closed |
| T-01-SC | Tampering (supply chain) | high | mitigate | `git diff --stat main -- go.mod go.sum` empty | closed |
| T-02-01 | Spoofing | medium | mitigate | Reseal skip disjunction gains `laneFromConnectContext(ctx) != auth.LaneCookie` (`connectreseal.go:52`), so a bearer request never re-seals a cookie it did not authenticate with. `TestResealGatesOnCookieLane` (spy count 0) PASS | closed |
| T-02-02 | Spoofing / Repudiation | medium | mitigate | `TestBearerLaneParity`, `TestBearerLaneParityActorFallback` compare resolved actor/owner directly across lanes — PASS | closed |
| T-02-03 | Elevation of Privilege | high | mitigate | One verifier value; both lanes reject expired and zero-`Expiration` tokens. `TestBearerLaneParityRejectsExpiredOnBothLanes`, `…RejectsZeroExpirationOnBothLanes` PASS | closed |
| T-02-04 | Information Disclosure | low | accept | See AR-01-03 | closed |
| T-02-SC | Tampering (supply chain) | high | mitigate | `go.mod`/`go.sum` diff empty | closed |
| T-03-01 | Elevation of Privilege | high | mitigate | `connectHeadlessGuard` (`serve.go:326-330`) is called at `:157` — strictly before `connectResolverFor` (`:210`) and `server.Register` (`:213`), so headless-without-a-lane cannot reach a mount. `TestHeadlessRefusesStartWithoutAuthLane` PASS | closed |
| T-03-02 | Information Disclosure | high | mitigate | The mount condition names only `cookieResolve == nil && !headless` (`serve.go:362-367`); a configured chain is never itself an activation signal, and `mountConnect`'s `resolve == nil` gate is unchanged (`connectapi.go:447-449`). `TestConnectResolverForDefaultOff` (non-nil chain, still nil) PASS | closed |
| T-03-03 | Tampering | high | mitigate | `buildAuthChain` (`serve.go:386`) is the sole construction site; `withAuth` (`:456`) admits no config, making a second chain a compile-time impossibility rather than a tested invariant. `TestAuthChainSharedBetweenLanes` PASS | closed |
| T-03-04 | Spoofing | medium | mitigate | `strconv.ParseBool` at load (`validate.go:215-216`) and at point of use (`serve.go:151-156`). `TestConnectHeadlessRejectsNonBoolean` PASS | closed |
| T-03-05 | Information Disclosure | low | mitigate | Boot log lines report boolean outcome and env-var name only, never a credential (`serve.go:154,158,201,203`) | closed |
| T-03-06 | Denial of Service | low | accept | See AR-01-04 | closed |
| T-03-07 | Elevation of Privilege | medium | accept | See AR-01-05 | closed |
| T-03-08 | Elevation of Privilege | medium | mitigate | Same lane-keyed exemption as T-01-01, now reachable on UI-enabled deployments with a chain. `TestCSRFCookieCallerCannotSelfDeclareBearerLane`, `TestCSRFFailedBearerNeverFallsThroughToExemption` (driving the **real** composed resolver, not a pre-stamped stub) PASS | closed |
| T-03-SC | Tampering (supply chain) | high | mitigate | `go.mod`/`go.sum` diff empty | closed |
| T-04-01 | Tampering | medium | mitigate | `configure.md:165` documents `ENGRAM_CONNECT_HEADLESS` / `--connect-headless` matching `registry.go:77` byte-for-byte | closed |
| T-04-02 | Information Disclosure | low | mitigate | No secret-shaped literal in the docs (search for `eyJ`/`sk-`/`Bearer <token>` returns 0 matches) | closed |
| T-04-03 | Repudiation | low | mitigate | Stale `Source:` attributions corrected to `buildAuthChain` (`configure.md:144,159`); zero remaining `Source:.*withAuth` | closed |
| T-04-04 | Elevation of Privilege | high | mitigate | Independently re-rendered: default `helm template` emits 0 `ENGRAM_CONNECT_HEADLESS` rows; `--set …=true` renders `"true"`; `--set …=false` renders `"false"` — the guard is `ne (toString) ""`, not `with`, so an explicit `false` is not silently dropped | closed |
| T-04-05 | Tampering | medium | mitigate | `task chart:validate` → OK; the `engram.containerEnv` checksum was re-pinned and re-verified, not disabled | closed |
| T-04-SC | Tampering (supply chain) | high | accept | See AR-01-06 | closed |

*Severity: critical > high > medium > low — only open threats at or above `workflow.security_block_on` (high) count toward `threats_open`*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-01-01 | T-01-06 | Lane provenance is not logged or persisted this phase, so a post-hoc audit cannot attribute a request to a lane. Verified: no `slog` call in `connectauth.go`/`connectcsrf.go`/`connectreseal.go`/`identity.go` references the lane, and `auth.Lane` is never referenced in `internal/store` | Phase 1 plan | 2026-07-31 |
| AR-01-02 | T-01-09 | A value declaring the bearer scheme with a malformed credential falls through to the **cookie** lane rather than failing closed. Accepted because the fallthrough direction is toward the *more restrictive* lane — such a caller gets the full CSRF check — and D-02 names this rule explicitly. Pinned by `TestMalformedCredentialShapesFallThroughToCookieLane` (5 subtests) so the boundary cannot drift silently | Phase 1 plan (D-02) | 2026-07-31 |
| AR-01-03 | T-02-04 | Test-only stub credentials are literals in `_test.go` files (`connectapi_service_auth_parity_test.go:44`) and never reach a production binary | Phase 1 plan | 2026-07-31 |
| AR-01-04 | T-03-06 | The boot-time verifier construction keeps its existing 15-second `context.WithTimeout` (`serve.go:390`); an unreachable IdP still fails boot the same way, only earlier in the order | Phase 1 plan | 2026-07-31 |
| AR-01-05 | T-03-07 | **Human-blessed behavior change.** Wherever Connect is mounted, a configured chain is its bearer half *unconditionally* — so a UI-enabled deployment that never sets `connect.headless` now accepts bearer credentials on Connect that it previously ignored. No new route is exposed (serve.go already mounted Connect whenever the UI was on); only which credential families an already-reachable surface accepts, for principals who already hold equivalent MCP access through the same verifier. Verified in code: `serve.go:366` passes `chain` unconditionally when mounted; `TestConnectResolverForUIEnabledIncludesBearerHalf` PASS | Sean (standing T-03-07 ruling, REVIEWS.md HIGH-3) | 2026-07-31 |
| AR-01-06 | T-04-SC | Plan 01-04 touches no dependency manifest; `git diff --stat main -- go.mod go.sum` empty | Phase 1 plan | 2026-07-31 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-02 | 30 | 30 | 0 | gsd-security-auditor (retroactive, ASVS L1 + L2 spot-depth, block_on high) |

**Verdict: SECURED.** Register origin: `register_authored_at_plan_time: true`.

**Verification depth exceeded the configured level where it mattered.** ASVS L1 asks only that the
mitigating pattern be present in the cited file. For the CSRF / lane-provenance core
(T-01-01, T-01-04, T-01-08, T-03-02, T-03-08) the auditor additionally performed L2-equivalent
boundary checks: source-level *negative* greps confirming no caller-controlled signal reaches the
decision, and interceptor/call-ordering confirmed by line number. Every cited test was **re-run
live** rather than quoted from the SUMMARY, with `ENGRAM_REQUIRE_QDRANT=1` where applicable and the
lane-isolation test additionally under `-race`. `go build ./... && go vet ./...` clean.

**Register count correction.** The orchestrator's dispatch brief stated 26 rows; the actual count is
**30**. The delta is exactly the four per-plan supply-chain rows (`T-01-SC`, `T-02-SC`, `T-03-SC`,
`T-04-SC`), which an unanchored row-count pattern misses. All 30 were verified. Recorded because the
undercount originated in the orchestrator's own pre-count, not in the plans.

No unregistered threats: no `## Threat Flags` section exists in any of the four SUMMARYs.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
