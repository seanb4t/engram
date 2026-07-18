---
phase: 23-service-auth-chain-tenancy-isolation
verified: 2026-07-18T03:04:41Z
status: passed
score: 12/12 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 23: Service Auth Chain & Tenancy Isolation Verification Report

**Phase Goal:** Headless service principals — OIDC client-credentials or operator-provisioned
static tokens — authenticate through a pluggable, config-selectable verifier chain and are
isolated to their own owner bucket by default, never the anonymous bucket and never colliding with
a human owner. The milestone's #1 risk (a service principal silently resolving to `owner==""`) is
proven fail-closed as the first test of this phase.
**Verified:** 2026-07-18T03:04:41Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC2 (#1 risk): a service principal that resolves to no owner claim is REJECTED at the verifier boundary, never a fail-open TokenInfo | VERIFIED | `internal/auth/auth.go:254-261` — `if v.failClosed && ownerVal == "" { return nil, errors.Join(mcpauth.ErrInvalidToken, ...) }`. Proven as the FIRST test of the phase: `TestFailClosedRejectsEmptyOwner` is line 71, the first `func Test` in `internal/auth/service_owner_failclosed_test.go`. `go test ./internal/auth/...` passes. |
| 2 | Human/no-issuer lane fail-open behavior is unchanged by the service lane's fail-closed reject | VERIFIED | `failClosed` defaults `false` (zero value) on `New()`'s `Verifier`; only `NewService()` sets it `true`. `TestFailClosedDoesNotAffectHumanLane` (auth_test) and `TestWithAuth_NoLaneConfigured_ReturnsHandlerUnchanged` / `TestWithAuth_HumanOnlyConfig_RejectsUnauthenticated` (serve_test.go) pass. |
| 3 | SC1 (chain): `withAuth` composes human/service/static verifiers via `auth.ChainVerifier`, each built only when its own config is present | VERIFIED | `cmd/engram/serve.go:296-341` — three independent `if` gates (`oidc.Issuer != ""`, `svcAuth.OIDCIssuer != ""`, `svcAuth.StaticTokens != ""`) each build one lane; `chain := auth.ChainVerifier(humanVerifier, serviceVerifier, staticVerifier)` feeds the single `mcpauth.RequireBearerToken` call — the one changed call site. |
| 4 | JWT-shaped bearers route only to the OIDC branch; opaque bearers route only to the static comparator — never both | VERIFIED | `internal/auth/chain.go` `discriminate`/`looksLikeJWT` (exactly-two-dots) routes before any verifier runs; `chain_test.go` covers D-04 routing, nil-guard deny, human-only preservation, determinism. `go test ./internal/auth/...` passes. |
| 5 | No 3rd `store.Subject` variant introduced — sealed 2-variant sum stays byte-for-byte unwidened (DEC-12c) | VERIFIED | `internal/store/subject.go` — exactly `anonymous{}` and `authenticated{sub string}`; unexported, sealed via `isSubject()`. Phase 23 adds zero lines to this file. |
| 6 | SC3 (static token): `crypto/subtle.ConstantTimeCompare` over the full token, distinct owner per token, never logged, and it actually authenticates end-to-end from `ENGRAM_SERVICE_AUTH_STATIC_TOKENS` config | VERIFIED (post-CR-01-fix) | `internal/auth/static_token.go:63-71` compares every candidate with `subtle.ConstantTimeCompare`, no early return; error is a constant literal, never interpolates token/candidate. `TestWithAuth_StaticTokenLane_AuthenticatesConfiguredToken` (`cmd/engram/serve_test.go:144`) drives a real `"owner-x=secret-token-value"` config string through `config.ParseServiceStaticTokens` → `withAuth` → live `mcpauth.RequireBearerToken` verify. Ran directly: **PASS** (both subtests). |
| 7 | CR-01 fix holds: `StaticTokenVerifier` natively accepts the token-keyed map `ParseServiceStaticTokens` produces (no orientation mismatch, no raw-token-into-owner leak) | VERIFIED | `internal/auth/static_token.go:37-46` — struct field comment `tokens map[string]string // token -> ownerID`; loop `for candidateToken, ownerID := range v.tokens` compares presented token against the map KEY. `cmd/engram/serve.go:326-333` passes `config.ParseServiceStaticTokens`'s token-keyed map straight through, matching orientation. `staticTokenExpirationHorizon` (100y) added so `mcpauth.RequireBearerToken`'s zero-`Expiration` hard-reject (the second defect WR-01 surfaced) doesn't also break the lane. |
| 8 | SC4 (isolation): a service principal can't read another owner's private records, doesn't collide with anonymous/human owners, recalls empty (not error) when it owns nothing, order-independent | VERIFIED | `internal/store/service_principal_isolation_test.go` `TestServicePrincipalIsolation` — adjacency check, private-record isolation via `Search`/`List`, empty-input case, reordered-insertion case. Ran against real Qdrant (testcontainers): **PASS**. Zero new production store code (`git diff --stat` on the commit shows only the test file, 149 insertions). |
| 9 | SC5 (shared cross-tenant): the decision is WRITTEN and TESTED as a positive, intended behavior | VERIFIED | `docs/adr/engram-svct-service-tenant-global-shared-read.md` (Accepted, 2026-07-17) records global shared-read as intended v0.11.x behavior. `TestSharedCrossTenantReadIntended` asserts tenant B's principal CAN read tenant A's shared record (`len(hits) != 1 \|\| hits[0].ID != m.ID` fatal on failure) — a positive/must-read assertion, not a restriction. Ran against real Qdrant: **PASS**. |
| 10 | `namespacedOwner` reused for the static-token/service-OIDC lanes — no second encoding scheme (DEC-g37x) | VERIFIED | `internal/auth/static_token.go:79` and `internal/auth/auth.go:207` both call the single `namespacedOwner(claim, value)` defined once in `auth.go`. |
| 11 | Config surface: four additive `service_auth.*` koanf rows, `ServiceAuthConfig` wired into `Config`, service owner-claims never default to `email` (D-05) | VERIFIED | `internal/config/registry.go:58-61` (`oidc_issuer`, `oidc_audience`, `owner_claims` default `"client_id,azp"`, `static_tokens`); `internal/config/config.go:30` `ServiceAuth ServiceAuthConfig \`koanf:"service_auth"\``. `go test ./internal/config/...` passes. |
| 12 | Config validation self-gates on empty service_auth fields, shape-checks when set, fatal on malformed static-tokens, and fails fast on a two-dot static token (WR-02 fix) | VERIFIED | `internal/config/validate.go:99-143` — URL shape-check for `oidc_issuer`, `ParseOwnerClaims` reuse, fatal `ParseServiceStaticTokens` error, and the added two-dot rejection loop naming `ENGRAM_SERVICE_AUTH_STATIC_TOKENS` and `looksLikeJWT`. `TestServiceAuthValidate_StaticTokenTwoDotsRejected` exists per REVIEW-FIX. |

**Score:** 12/12 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/auth/auth.go` | fail-closed service lane | VERIFIED | `NewService`/`failClosed` field, wired and tested |
| `internal/auth/service_owner_failclosed_test.go` | FIRST test proving SC2 | VERIFIED | `TestFailClosedRejectsEmptyOwner` is the first func, real signature-verified OIDC fixture |
| `internal/auth/static_token.go` | constant-time static-token lane | VERIFIED | token-keyed map (post-CR-01 fix), 100y expiration horizon (post-WR-01 fix) |
| `internal/auth/static_token_test.go` | no-leak + rotation proof | VERIFIED | `TestStaticTokenNoLeak`, rotation test updated to token-keyed orientation |
| `internal/auth/chain.go` | `chainVerifier` combinator | VERIFIED | structural JWT/opaque routing, deny-by-default, D-02 ordering |
| `internal/auth/chain_test.go` | routing/order/nil-guard/determinism coverage | VERIFIED | present, `go test` passes |
| `internal/config/registry.go`, `config.go`, `validate.go`, `service_auth_test.go` | `service_auth.*` config surface | VERIFIED | 4 additive rows, struct wiring, validation incl. WR-02 fix |
| `internal/store/service_principal_isolation_test.go` | SC4/SC5 proofs | VERIFIED | 2 tests, both pass against real Qdrant, zero new store production code |
| `cmd/engram/serve.go` | `withAuth` chain wiring | VERIFIED | single changed call site, 3 conditional lane builds |
| `internal/server/connectapi_service_auth_parity_test.go` | lane-independent resolution parity | VERIFIED | `TestServiceAuthChainParity` (3 subtests) + `TestServiceAuthChainParity_EmptyOwnerFailsClosedPostComposition`, both pass against real Qdrant |
| `cmd/engram/serve_test.go` | E2E static-token wiring test (WR-01 fix) | VERIFIED | `TestWithAuth_StaticTokenLane_AuthenticatesConfiguredToken` passes |
| `docs/adr/engram-svct-service-tenant-global-shared-read.md` | SC5 decision record | VERIFIED | Accepted, records global shared-read intent, cites the permanent regression test |
| `docs-site/.../reference/auth.md`, `.../guides/configure.md` | operator docs for `ENGRAM_SERVICE_AUTH_*` | VERIFIED | all 4 env vars documented with defaults/behavior |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `withAuth` | `auth.ChainVerifier` | 3 conditionally-built verifiers | WIRED | `cmd/engram/serve.go:337` |
| `auth.ChainVerifier` | `mcpauth.RequireBearerToken` | single verifier param | WIRED | `cmd/engram/serve.go:338` |
| `config.ParseServiceStaticTokens` | `auth.NewStaticTokenVerifier` | token-keyed map (post-CR-01) | WIRED | `cmd/engram/serve.go:326-333`, orientation confirmed matching in `static_token.go` |
| `TokenVerifier` empty-owner reject | `errors.Join(mcpauth.ErrInvalidToken, ...)` | `v.failClosed` branch | WIRED | `internal/auth/auth.go:254-261` |
| `store.Search`/`List` | Phase-22 `DecideBucket` filter | `Authenticated(serviceOwner)` | WIRED | proven by `TestServicePrincipalIsolation`/`TestSharedCrossTenantReadIntended` against real Qdrant |

### Behavioral Spot-Checks / Test Execution

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go build ./...` | full build | exit 0 | PASS |
| `internal/auth`, `internal/config`, `cmd/engram` unit suites | `go test -count=1 ./internal/auth/... ./internal/config/... ./cmd/engram/...` | all ok | PASS |
| CR-01/WR-01 regression test | `go test -run TestWithAuth_StaticTokenLane_AuthenticatesConfiguredToken -v ./cmd/engram/...` | PASS (2 subtests) | PASS |
| SC4/SC5 store proofs (real Qdrant via testcontainers) | `go test -run 'TestServicePrincipalIsolation\|TestSharedCrossTenantReadIntended' -v ./internal/store/...` | PASS (2 tests) | PASS |
| Chain parity + post-composition fail-closed (real Qdrant) | `go test -run TestServiceAuthChainParity -v ./internal/server/...` | PASS (2 tests, 4 subtests) | PASS |
| Debt markers in phase files (`TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER`) | `rg` across the 14 phase-modified/added files | no matches | PASS |
| Zero new production store code (DEC-cgb) | `git diff --stat` on `test(23-05)` commit | only `service_principal_isolation_test.go`, 149 insertions | PASS |
| Sealed `Subject` sum unwidened (DEC-12c) | `internal/store/subject.go` inspection | exactly 2 variants, untouched by phase | PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|-----------------|-------------|--------|----------|
| REQ-service-auth-chain | 23-01, 23-02, 23-03, 23-04, 23-06 | pluggable verifier chain, defined order, shared `TokenInfo`/`Subject` contract | SATISFIED | `chain.go`, `withAuth` wiring, parity test |
| REQ-static-token-auth | 23-02, 23-04 | operator-provisioned static tokens, constant-time compare, 1 token→1 owner | SATISFIED | `static_token.go` (post-CR-01/WR-01 fix), config parser |
| REQ-service-owner-failclosed | 23-01 | unresolvable owner claim is rejected, never anonymous bucket | SATISFIED | `auth.go` failClosed branch, first-test proof |
| REQ-service-principal-isolation | 23-05, 23-06 | isolated to own bucket, Cedar/PDP seam forward-compatible | SATISFIED | isolation test suite, ADR documents forward-compat deferral |

No orphaned requirements — all 4 REQ IDs mapped to phase 23 in REQUIREMENTS.md appear in at least one plan's `requirements:` frontmatter.

### Anti-Patterns Found

None. No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers in any of the 14 files this phase modified or added. No stub returns, no empty handlers, no hardcoded-empty data flows found in the reviewed production files (`auth.go`, `chain.go`, `static_token.go`, `serve.go`, config files).

### Code Review Follow-Through

The phase's own code review (`23-REVIEW.md`) found one CRITICAL defect (CR-01: static-token map orientation mismatch — the feature was deployed non-functional and could leak the raw secret token into the persisted `owner` field) and two WARNINGs (WR-01: no E2E test covering the broken seam; WR-02: JWT-shape/static-token collision has no fail-fast guard). `23-REVIEW-FIX.md` records all three fixed (commits `43f49c49`, `56f227ad`, `c3855a08`), and this verification independently confirmed:
- The token-keyed map orientation in `static_token.go` matches `config.ParseServiceStaticTokens`'s output — read directly from source, not from the SUMMARY's claim.
- `TestWithAuth_StaticTokenLane_AuthenticatesConfiguredToken` exists and passes when run directly (not just cited).
- The WR-01 second defect (zero `Expiration` hard-rejected by `mcpauth.RequireBearerToken`) is also fixed (`staticTokenExpirationHorizon`).
- The WR-02 two-dot validation guard exists in `validate.go` and is named correctly in its error text.

A follow-on info-level item (IN-01, `TokenInfo.Expiration` omission) was superseded by the WR-01 fix's concrete expiration horizon and its doc comment — confirmed present.

### Human Verification Required

None. All must-haves are either directly observable in source or proven by an executed, passing test (including two store-layer tests run against a real Qdrant instance via testcontainers, not mocked).

### Gaps Summary

None. All 5 success criteria (SC1-SC5), all 4 requirement IDs, and the milestone's #1 risk (fail-closed proof as the FIRST test) are verified against the actual codebase, with the CR-01 critical wiring bug confirmed fixed and independently re-verified (not merely trusted from SUMMARY/REVIEW-FIX narrative).

---

_Verified: 2026-07-18T03:04:41Z_
_Verifier: Claude (gsd-verifier)_
