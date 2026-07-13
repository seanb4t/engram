---
phase: 17
slug: wired-write-handlers-full-crud-schedule
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-07-12
---

# Phase 17 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Validation Architecture lives in `17-RESEARCH.md`; the planner lifts per-task
> checks from it into each PLAN.md's `must_haves`.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` |
| **Config file** | none — `Taskfile.yaml` drives `task test`; golangci via `.golangci.yaml` |
| **Quick run command** | `go test ./internal/server/... ./internal/auth/... ./internal/webauth/...` |
| **Full suite command** | `task test` (and `task lint:go` — run explicitly per phase-15 executor blind-spot gotcha) |
| **Estimated runtime** | ~30–90 seconds (Qdrant-backed store tests skip without a live Qdrant; fake-store parity tests run in-process) |

---

## Sampling Rate

- **After every task commit:** Run the quick command scoped to the touched package.
- **After every plan wave:** Run `task test` then `task lint:go`.
- **Before `/gsd-verify-work`:** Full suite + `task lint:go` must be green.
- **Max feedback latency:** 90 seconds.

---

## Per-Task Verification Map

> Reconstructed 2026-07-13 from committed 17-01..17-06 SUMMARY.md coverage
> blocks and confirmed by re-running every cited test fresh (not trusting the
> SUMMARY claims alone). All rows green on `task test` + `task lint:go`.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|--------|
| 17-01-D1 | 01 | 1 | REQ-connect-write-authz-parity | D-04/D-05/D-06 | Ordered ClaimIdentity, fail-closed email_verified gate, injective non-email owner encoding, presence-vs-type generalized to every ordered claim | unit | `go test ./internal/auth/... -run TestClaimIdentity` | ✅ green |
| 17-01-D2 | 01 | 1 | REQ-connect-write-authz-parity | — | Versioned session cookie payload invalidates every pre-upgrade / legacy sub-keyed cookie on Resolve | unit | `go test ./internal/webauth/... -run TestResolverRejectsLegacyVersionCookie\|TestOldSubKeyedCookieRejected` | ✅ green |
| 17-01-D3 | 01 | 1 | REQ-connect-write-authz-parity | — | config.ParseOwnerClaims comma-list parsing separated from defaulting; malformed list rejected | unit | `go test ./internal/config/...` | ✅ green |
| 17-02-D1/D2 | 02 | 2 | REQ-connect-write-authz-parity | D-08 | store.UpdatePayload: targeted, vector-preserving payload-only update; two-op non-atomicity documented + injected-failure covered | unit/integration | `go test ./internal/store/... -run TestUpdatePayload` | ✅ green |
| 17-02-D3 | 02 | 2 | REQ-connect-write-authz-parity | — | memStore narrow interface carve; deps.st retyped; compile-time `var _ memStore = (*store.Store)(nil)` assertion | build | `go build ./...` | ✅ green |
| 17-03-D1..D4 | 03 | 2 | REQ-connect-write-authz-parity | D-09 | protoconv.go exact proto<->args/response mapping for all six write RPCs; outward-rounded scheduling-window bounds (floor/ceil) surviving store's second-granular flooring | unit | `go test ./internal/server/... -run TestProtoconv` | ✅ green |
| 17-04-D1 | 04 | 3 | REQ-connect-write-authz-parity | — | Single production connectError(ctx, err) mapper: typed sentinels -> precise Connect codes; unknown -> CodeInternal, non-leaking | unit | `go test ./internal/server/... -run TestConnectError` | ✅ green |
| 17-04-D2 | 04 | 3 | REQ-connect-write-authz-parity | — | Scripted-spy memStore fake (spyStore) records method+owner+args, full interface | unit | `go test ./internal/server/... -run TestSpyStore` | ✅ green |
| 17-04-D3 | 04 | 3 | REQ-connect-write-authz-parity | — | Six Connect write RPCs wired as thin deps.* adapters; negative matrix + CSRF token matrix | integration | `go test ./internal/server/... -run TestWriteRPCNegativeMatrix\|TestConnectCSRFTokenMatrix` | ✅ green |
| 17-05-D1/D2 | 05 | 4 | REQ-connect-write-authz-parity | D-10 | Per-RPC MCP<->Connect spy delegation parity for all six write RPCs + source/AST proof each handler names its deps.* method | unit | `go test ./internal/server/... -run TestWriteParity` | ✅ green |
| 17-05-D3 | 05 | 4 | REQ-connect-write-authz-parity | D-11 / DEC-xa6 | Split short_id/direct-UUID cross-owner leak tables for by-id write RPCs (no resolved-UUID leak on short_id path) | integration | `go test ./internal/server/... -run TestCrossOwnerRewrap` | ✅ green |
| 17-05-D4 | 05 | 4 | REQ-connect-write-authz-parity | — | Fail-closed ENGRAM_REQUIRE_QDRANT gate: malformed value rejects (not coerce-false); CI job sets it | unit + CI config | `go test ./internal/server/... -run TestRequireQdrant`; `.github/workflows/ci.yaml` | ✅ green |
| 17-05-D5 | 05 | 4 | REQ-connect-write-authz-parity | SC5/D-12 | NO_SIDE_EFFECTS idempotency-level ban re-asserted (no `idempotency_level` in proto) | other + lint gate | `rg idempotency_level proto/engram/v1/*.proto` (no match); `task proto:lint` | ✅ green |
| 17-06-D1..D4 | 06 | 2 | REQ-connect-write-authz-parity | D-07 | Typed core read contract (coreListRequest/coreListResult/coreSearchRequest) as superset of both transport lanes; offset/cursor mode mutual exclusion; MCP cursor-mode preserved | unit | `go test ./internal/server/... -run TestListMemorySuperset\|TestListMemoryReturnsNextCursorField\|TestRerankParityMCPAndConnect` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**Verification method:** every command above was re-run fresh in this audit
(2026-07-13), independent of the SUMMARY.md claims and the prior
17-VERIFICATION.md citations. `task test` (full suite, includes Qdrant
testcontainers-backed store tests) and `task lint:go` both ran clean.

---

## Wave 0 Requirements

- [x] Fake `store` seam (narrow interface) so MCP/Connect parity tests can substitute a non-Qdrant store — landed 17-02 (`internal/server/store_iface.go`, `memStore` interface) + 17-04 (`internal/server/fakestore_test.go`, `spyStore`). Confirmed via `go build ./...` compile assertion and `TestSpyStoreRecordsMethodAndSubject`.
- [x] Shared parity-scenario table fixture (à la `TestRerankParityMCPAndConnect`) driving both lanes — landed 17-05 (`internal/server/connectapi_write_parity_test.go#TestWriteParity`), confirmed passing with all six write-RPC rows plus the source/AST delegation sub-test.

Both Wave-0 prerequisites are satisfied and independently re-verified in this audit.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|--------------------|
| Browser network-tab existence-leak check (DEC-xa6) | REQ-connect-write-authz-parity | Confirms no resolved-UUID leak reaches a real browser | Covered by automated `TestCrossOwnerRewrap` per by-id RPC; manual spot-check optional, not required for compliance |

---

## Validation Sign-Off

- [x] All tasks have automated verify (no Wave-0-only dependencies remain)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all previously-MISSING references (both prereqs landed and confirmed)
- [x] No watch-mode flags
- [x] Feedback latency < 90s (`task test` full suite ran in ~2–3s excluding Docker container startup)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** confirmed 2026-07-13 (adversarial audit — see trail below)

---

## Validation Audit 2026-07-13

Reconstructed the real per-task -> test coverage map from the six committed
PLAN/SUMMARY pairs (this VALIDATION.md was a plan-time scaffold with only one
placeholder row). Read every plan's `must_haves` and every summary's
`coverage` block, then re-ran every cited test fresh — not trusting the
SUMMARY.md/17-VERIFICATION.md citations as proof, only as leads.

All cited tests exist and pass:
`TestClaimIdentity*` (7 sub-tests/tables), `TestNamespacedOwner*Injectivity`,
`TestUpdatePayload*` (Qdrant-testcontainer-backed), `TestProtoconv*` (14
tests), `TestConnectError` (12 cases), `TestSpyStore*`,
`TestWriteRPCNegativeMatrix`, `TestConnectCSRFTokenMatrix`, `TestWriteParity`
(6 RPC rows + AST delegation sub-test), `TestCrossOwnerRewrap` (3 RPCs x 2
input shapes), `TestRequireQdrant` (6 cases), `TestListMemorySuperset*`,
`TestListMemoryReturnsNextCursorField`, `TestRerankParityMCPAndConnect`,
`TestResolverRejectsLegacyVersionCookie`, `TestOldSubKeyedCookieRejected`.
Also confirmed the NO_SIDE_EFFECTS ban via direct grep (no
`idempotency_level` in any proto file) and a full `task test` + `task
lint:go` clean run.

No implementation files were modified. No new test files were required —
every requirement behavior in scope already had real, passing, non-trivial
coverage.

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 (none needed — all 15 task-level behaviors already COVERED) |
| Escalated | 0 |
