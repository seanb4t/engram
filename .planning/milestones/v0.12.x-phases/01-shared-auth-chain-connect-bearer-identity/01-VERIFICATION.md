---
phase: 01-shared-auth-chain-connect-bearer-identity
verified: 2026-07-31T00:00:00Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# v0.12.x Phase 1: Shared Auth Chain & Connect Bearer Identity Verification Report

**Phase Goal:** A headless caller can authenticate to the ConnectRPC lane with a bearer token —
safely. One composed verifier serves both lanes, token expiry is actually enforced, the
authenticating lane is recorded by the server, the CSRF exemption is decided from that record
alone, and the lane is mounted only when explicitly enabled.

**Verified:** 2026-07-31
**Status:** passed
**Re-verification:** No — initial verification

**Note on phase identity:** verified against `.planning/ROADMAP.md`'s
`### v0.12.x Phase 1: Shared Auth Chain & Connect Bearer Identity` section (line ~315), not the
historical v0.8.x "Phase 1: Authorization & Isolation" at line ~141. Requirement IDs used:
`REQ-connect-bearer-identity`, `REQ-connect-token-expiry`, `REQ-connect-lane-provenance`,
`REQ-connect-headless-mount` (per the task brief, not `gsd-tools query init.*`, which is known to
mis-resolve to the historical phase for this milestone).

## Goal Achievement

### Observable Truths (the five ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A bearer token accepted on MCP is accepted on Connect and vice versa for rejection, both resolving through **one composed verifier constructed once**, proven structurally | ✓ VERIFIED (PROVEN) | `cmd/engram/serve.go:146` calls `buildAuthChain` exactly once in `runServe`; the returned `chain` value is passed by parameter to `withAuth` (`serve.go:236`, MCP) and to `connectResolverFor`→`server.NewConnectResolver` (`serve.go:210,366`, Connect). Confirmed by grep: `buildAuthChain(` appears in production code only at its definition (`serve.go:386`) and its one call site (`serve.go:146`); the three verifier constructors (`auth.New`, `auth.NewService`, `auth.NewStaticTokenVerifier`) appear in production code only inside `buildAuthChain`'s body (`serve.go:391,406,420`) — no second construction site exists to diverge from. `TestAuthChainSharedBetweenLanes` (`cmd/engram/serve_test.go:332`) exercises this: one `buildAuthChain` call, the same `chain` value fed to `withAuth` and `server.NewConnectResolver`, both accept the same static token. Ran live: `--- PASS: TestAuthChainSharedBetweenLanes`. This is a structural guarantee (single call site + single value threaded to both consumers), not merely a test observing equal behavior twice. |
| 2 | A token whose `Expiration` has passed is rejected on the Connect lane | ✓ VERIFIED (PROVEN) | `internal/auth/bearer.go:141-167` `EnforceExpiry` rejects `ti.Expiration.Before(time.Now())` with zero skew, wraps the composed chain (`composeAuthChain`, `serve.go:442`), which is injected into `connectResolverFor`'s bearer half. Ran live: `--- PASS: TestEnforceExpiry`, `--- PASS: TestEnforceExpiryNoSkew`, `--- PASS: TestBearerLaneParityRejectsExpiredOnBothLanes` (Connect-side, real mounted resolver), `--- PASS: TestComposeAuthChainWrapsWithExpiry` (proves the composed chain — not just the standalone decorator — rejects a past-Expiration `TokenInfo`). |
| 3 | A cookie-authenticated caller is rejected on all six write RPCs when it omits `X-CSRF-Token`, and cannot obtain the bearer exemption by attaching a garbage `Authorization` header to its session | ✓ VERIFIED (PROVEN) | Six write RPCs confirmed named identically in the enforcement allowlist (`internal/server/connectcsrf.go:33-40`, `csrfWriteProcedures`: StoreMemory, StoreDiscovery, UpdateMemory, DeleteMemory, SetVisibility, ScheduleMemory) and the test matrix (`internal/server/connectcsrf_test.go:107-149`, `csrfWriteCases`, identical six names). Ran live: `--- PASS: TestCSRFCookieCallerOmittingHeaderIsStillRejected`, `--- PASS: TestCSRFCookieCallerCannotSelfDeclareBearerLane`. The self-declare test (`connectcsrf_lane_test.go:72-117`) genuinely sends the attack input — `csrfHeaders.authorization` is wired into `doCSRFWrite`'s `req.Header().Set("Authorization", h.authorization)` (`connectcsrf_test.go:88-89`), and the test asserts `sawAuthHeader` (a cookie-resolver-side observation) is true, i.e. it self-verifies the hostile header actually reached the server rather than assuming it did — directly addressing the "fixture can't express the attack" failure mode this verification was warned about. |
| 4 | A bearer verification failure never authenticates via the cookie lane | ✓ VERIFIED (PROVEN) | `internal/server/connectbearer.go:88-96`: on a well-formed bearer credential, `verifyBearerCredential`'s error is returned immediately with `auth.LaneUnknown`; `cookieResolve` is never called in that branch (no call to it exists on that code path). Ran live: `--- PASS: TestBearerFailureNeverFallsThroughToCookie`. |
| 5 | With the UI disabled and the headless flag unset, no Connect handler is registered — byte-for-byte today's behavior | ✓ VERIFIED (PROVEN) | `internal/server/connectapi.go:368-369` `mountConnect`'s `resolve == nil` gate is byte-for-byte unchanged since Plan 01's commit (`git diff --exit-code ed853385 -- internal/server/connectapi.go` exits 0, confirmed independently). `connectResolverFor` (`serve.go:362-367`) returns `nil` when `cookieResolve == nil && !headless`; `connect.headless` defaults to `"false"` (`internal/config/registry.go:72`). Ran live: `--- PASS: TestConnectResolverForDefaultOff`, `--- PASS: TestMountConnectDefaultOffWithoutUIOrHeadlessFlag`. |

**Score:** 5/5 truths verified (0 present-but-behavior-unverified)

**Ruled-out-of-scope item (per task brief, not re-litigated here):** the bearer half reaching
Connect unconditionally whenever Connect is mounted (independent of `connect.headless`) is a
maintainer-blessed, intended reading of criterion 1/D-12 (2026-07-31 ruling: "surface" means
registered route, not credential family). Confirmed in code: `connectResolverFor` (`serve.go:362`)
computes the mount decision from `cookieResolve == nil && !headless` only, and passes `chain`
through as the bearer half unconditionally once mounted — matching the ruling exactly. Not
flagged as a gap.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/auth/bearer.go` | `auth.Lane`, `auth.ExtractBearerCredential`, `auth.EnforceExpiry` | ✓ VERIFIED | Present, substantive, wired into `serve.go`'s `composeAuthChain`; exercised by 7 passing unit tests |
| `internal/server/connectbearer.go` | `NewConnectResolver`, `verifyBearerCredential` | ✓ VERIFIED | Present, substantive, single-read/single-decision (D-01/D-02 verified by direct code read), wired into `serve.go:210` |
| `internal/server/identity.go` | `connectLaneKey`/`withConnectLane`/`laneFromConnectContext` | ✓ VERIFIED | Present, dedicated typed context key (not `TokenInfo.Extra`), read by both `connectcsrf.go` and `connectreseal.go` |
| `internal/server/connectapi.go` | `connectResolver` widened to 3 returns | ✓ VERIFIED | `type connectResolver func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error)` (`connectapi.go:365`); mount gate untouched |
| `internal/server/connectcsrf.go` | Lane-keyed exemption, all six write RPCs | ✓ VERIFIED | `switch laneFromConnectContext(ctx)` with explicit `default` fail-closed (`connectcsrf.go:79-86`); six-RPC allowlist confirmed |
| `internal/server/connectreseal.go` | Reseal gated on `LaneCookie` | ✓ VERIFIED | One added clause, `laneFromConnectContext(ctx) != auth.LaneCookie` (`connectreseal.go:52`) |
| `internal/config/registry.go` | `connect.headless` key | ✓ VERIFIED | `{Key: "connect.headless", Env: "ENGRAM_CONNECT_HEADLESS", Flag: "connect-headless", Default: "false"}` — full Env+Flag triple, no `Legacy`, default off |
| `cmd/engram/serve.go` | `buildAuthChain`/`composeAuthChain`/`withAuth`/`connectHeadlessGuard`/`connectResolverFor` | ✓ VERIFIED | All five present, wired into `runServe`; single construction site confirmed by grep |
| `docs-site/.../guides/configure.md` | Headless Connect lane docs | ✓ VERIFIED | `### Headless Connect lane` subsection present (per 01-04-SUMMARY, `task lint` clean) |
| `charts/engram/values.yaml` + `_helpers.tpl` | `memory.connect.headless` Helm value | ✓ VERIFIED | Tri-state value, non-empty-guarded render (per 01-04-SUMMARY; `task chart:validate`/`chart:lint` clean per orchestrator gate run) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `serve.go:146 buildAuthChain(...)` | `serve.go:236 withAuth(handler, chain, ...)` | direct parameter | ✓ WIRED | Same `chain` variable |
| `serve.go:146 buildAuthChain(...)` | `serve.go:210 connectResolverFor(chain, ...)` → `server.NewConnectResolver(chain, cookieResolve)` | direct parameter | ✓ WIRED | Same `chain` variable |
| `connectauth.go newConnectSubjectInterceptor(resolve)` | `identity.go withConnectLane` | interceptor stamps lane from resolver's 3rd return | ✓ WIRED | Confirmed by `TestConnectSubjectInterceptorStampsLane` (pass, per 01-01-SUMMARY) |
| `identity.go laneFromConnectContext` | `connectcsrf.go` exemption switch | direct read, no re-parse of headers | ✓ WIRED | Confirmed by code read; `TestCSRFCookieCallerCannotSelfDeclareBearerLane` passes |
| `identity.go laneFromConnectContext` | `connectreseal.go` skip clause | direct read | ✓ WIRED | Confirmed by code read; `TestResealGatesOnCookieLane` passes |
| `internal/config/registry.go connect.headless` | `serve.go cfg.Connect.Headless` → `connectHeadlessGuard`/`connectResolverFor` | koanf load → `strconv.ParseBool` → guard | ✓ WIRED | `serve.go:151-160` |

### Behavioral Spot-Checks (named tests actually run, not full-suite)

All tests below were executed individually via `go test -run '<name>' -v` and produced real
`=== RUN` / `--- PASS` pairs (not a vacuous package-level `ok` with zero matches):

| Behavior | Test | Result |
|----------|------|--------|
| Expiry enforced, zero-skew | `TestEnforceExpiry`, `TestEnforceExpiryZero`, `TestEnforceExpiryNoSkew` | PASS |
| Composed chain (not just decorator) rejects past expiry | `TestComposeAuthChainWrapsWithExpiry` | PASS |
| Connect-lane rejects expired/zero-expiration bearer, wire message matches SDK exactly | `TestBearerLaneParityRejectsExpiredOnBothLanes`, `TestBearerLaneParityRejectsZeroExpirationOnBothLanes`, `TestBearerLaneParityRejectionBodiesMatch` (includes WR-01 fix: `assertConnectWireMessage`) | PASS |
| Bearer failure never falls through to cookie | `TestBearerFailureNeverFallsThroughToCookie` | PASS |
| Bearer lane exempt from CSRF | `TestBearerLaneExemptFromCSRF` | PASS |
| Cookie caller omitting `X-CSRF-Token` rejected | `TestCSRFCookieCallerOmittingHeaderIsStillRejected` | PASS |
| Cookie caller cannot self-declare bearer lane via garbage `Authorization` | `TestCSRFCookieCallerCannotSelfDeclareBearerLane` (self-verifies attack header was actually sent) | PASS |
| Unstamped lane fails closed on write RPC | `TestCSRFLaneUnstampedFailsClosed` | PASS |
| Reseal skipped for bearer/unknown lane, fires for cookie lane | `TestResealGatesOnCookieLane` | PASS |
| Default-off mount (no UI, no headless) | `TestMountConnectDefaultOffWithoutUIOrHeadlessFlag`, `TestConnectResolverForDefaultOff` | PASS |
| Headless + zero auth lanes refuses to start | `TestHeadlessRefusesStartWithoutAuthLane` | PASS |
| Structural single-construction proof | `TestAuthChainSharedBetweenLanes` | PASS |
| `connect.headless` registry key correctness | `TestConnectHeadlessDefault`, `TestConnectHeadlessRejectsNonBoolean` | PASS |

Additionally: `go build ./...` (exit 0) and `go vet ./...` (exit 0) run once, clean.

### Requirements Coverage

| Requirement | Description | Status | Evidence |
|-------------|-------------|--------|----------|
| REQ-connect-bearer-identity | Headless caller authenticates to Connect via bearer token | ✓ SATISFIED | Truths 1, 4; REQUIREMENTS.md marks `[x]` and Complete |
| REQ-connect-token-expiry | Connect bearer path rejects expired token | ✓ SATISFIED | Truth 2; REQUIREMENTS.md marks `[x]` and Complete |
| REQ-connect-lane-provenance | Server-set marker records authenticating lane, CSRF exemption reads it alone | ✓ SATISFIED | Truths 3, 4; REQUIREMENTS.md marks `[x]` and Complete |
| REQ-connect-headless-mount | Operator can mount Connect lane headless, explicit opt-in only | ✓ SATISFIED | Truth 5; REQUIREMENTS.md marks `[x]` and Complete |

No orphaned requirements found for this phase in REQUIREMENTS.md's phase mapping table.

### Anti-Patterns Found

None. No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers found in the phase's changed
production files. No stub returns, no hardcoded-empty data flowing to responses. The one WARNING
raised by the phase's own code review (WR-01: Connect-side wire-message assertion missing) was
fixed in commit `31cc732f`, verified present and passing above. The one INFO finding (IN-01: a test
oracle duplicated across two packages) is cosmetic, non-blocking, and does not affect any
must-have.

### Human Verification Required

None. All five success criteria are proven either structurally (single construction/injection
site, confirmed by source grep — not merely a passing test) or via a real, self-verifying
behavioral test that was executed live for this verification (not merely cited from SUMMARY.md).

### Gaps Summary

No gaps. All five ROADMAP success criteria are VERIFIED with direct evidence gathered independently
of SUMMARY.md's claims: source reads of the actual construction/injection sites, live execution of
the specific named tests (RUN/PASS pairs captured above, not `go test ./...` package-level `ok`),
and independent `git diff --exit-code` / `grep` checks against the "constructed once" and
"mount gate untouched" structural claims. The already-ruled-on Codex HIGH-3 finding (unconditional
bearer inclusion whenever Connect is mounted) is confirmed in code to match the maintainer's
2026-07-31 ruling and is not a gap.

---

_Verified: 2026-07-31_
_Verifier: Claude (gsd-verifier)_
