---
phase: 3
slug: cross-spine-memory-recall
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-01
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib) + `testcontainers-go/modules/qdrant` for real-Qdrant integration |
| **Config file** | none — `internal/store/store_test.go:TestMain` (50-74) provisions Qdrant programmatically (`ENGRAM_QDRANT_TEST_ADDR` override, else an ephemeral `qdrant/qdrant:v1.18.2` container) |
| **Quick run command** | `go test ./internal/store/... -run TestCrossSpine -v` |
| **Full suite command** | `task` (lint + test) |
| **Estimated runtime** | ~60s quick (container reuse via `TestMain`), ~5 min full |

---

## Sampling Rate

- **After every task commit:** `go test ./internal/<pkg>/... -run <TestName> -v` **plus `go vet ./...`**
- **After every plan wave:** `task` (full lint + test suite)
- **Before `/gsd-verify-work`:** Full suite green; `task proto:lint` and `task proto:gen` leaving a zero diff
- **Max feedback latency:** ~60 seconds

> **`go vet ./...`, not `go build ./...`, is the compile gate.** `go build` does not compile `_test.go`
> files, so a struct or signature change that breaks a test fixture passes `build` and fails only at
> `go test`. This phase changes `searchArgs`, `listArgs`, `coreSearchRequest`, and `coreListRequest` —
> all of which are constructed in test fixtures. Engram gotcha `3q4cx33cta`.

---

## Per-Task Verification Map

Task IDs are provisional until PLAN.md files exist; the plan-checker reconciles them.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 3-01-01 | 01 | 1 | REQ-cross-spine-authz-verified | T-03-01 | A cross-spine-shaped filter (`Must` with only `ownerOrSharedCondition`) never returns another owner's private record, over overlapping scope names, against real Qdrant | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -run TestCrossSpineAuthzIsolation -v` | ✅ `internal/store/store_test.go` | ✅ green |
| 3-01-02 | 01 | 1 | REQ-cross-spine-authz-verified | T-03-01 | RED-by-mutation transcript: the test fails when the authz clause is dropped or defeated, and is restored | manual-recorded | see Manual-Only Verifications; transcript recorded in `03-RED-TRANSCRIPT.md` | ✅ `03-RED-TRANSCRIPT.md` | ✅ green |
| 3-01-03 | 01 | 1 | REQ-cross-spine-authz-verified | — | `03-AUTHZ-GATE.md` amended with the D-06 `listFilter` reading | doc | `rg -n 'listFilter' .planning/phases/03-cross-spine-memory-recall/03-AUTHZ-GATE.md` (5 hits) | ✅ `03-AUTHZ-GATE.md` | ✅ green |
| 3-02-01 | 02 | 2 | REQ-cross-spine-search | T-03-01 | `ownerScopeFilter` and `listFilter` emit the scope clause conditionally; the authz clause stays unconditional | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -run 'TestSearchCrossSpine|TestListCrossSpine' -v` | ✅ `internal/store/store_test.go` | ✅ green |
| 3-02-02 | 02 | 2 | REQ-cross-spine-search | — | `Store.List`'s exact `Count` under cross-spine sums across all readable scopes | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -run TestListCrossSpineTotal -v` | ✅ `internal/store/store_test.go` | ✅ green |
| 3-03-01 | 03 | 3 | REQ-cross-spine-search | T-03-02 | `effectiveSearchScope` rejects an empty scope when `cross_spine` is false — the SOLE guard under D-07 | unit | `go test ./internal/server/... -run TestEffectiveSearchScope -v` | ✅ `internal/server/tools_test.go` | ✅ green |
| 3-03-02 | 03 | 3 | REQ-cross-spine-search | T-03-01 | Handler-level two-owner isolation through `d.searchMemory` / `d.listMemory` (D-17 defense-in-depth) | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... -run TestSearchMemoryCrossSpineIsolation -v` | ✅ `internal/server/tools_test.go` | ✅ green |
| 3-03-03 | 03 | 3 | REQ-cross-spine-search | T-03-03 | Connect does NOT infer cross-spine from an empty scope; the explicit proto field is required (D-04) | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... -run TestConnectCrossSpineNotInferred -v` | ✅ `internal/server/connectapi_crossspine_test.go` | ✅ green |
| 3-03-04 | 03 | 3 | REQ-cross-spine-search | — | MCP↔Connect parity on the same cross-spine query | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... -run TestSearchMemoriesConnectCrossSpine -v` | ✅ `internal/server/connectapi_crossspine_test.go` | ✅ green |
| 3-04-01 | 04 | 4 | REQ-cross-spine-result-provenance | — | Every cross-spine result carries its originating scope on both the compact and full views | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... -run TestCrossSpineResultScope -v` | ✅ `internal/server/tools_test.go` | ✅ green |
| 3-04-02 | 04 | 4 | REQ-cross-spine-result-provenance | T-03-04 | `searched_scopes` / `scopes_truncated` populated on cross-spine calls and **omitted entirely** on non-cross-spine calls | integration | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... -run TestSearchedScopesReporting -v` | ✅ `internal/server/tools_test.go` | ✅ green |
| 3-04-03 | 04 | 4 | REQ-cross-spine-search | — | Generated code has zero drift after `task proto:gen` | build | `task proto:lint && task proto:gen && git diff --exit-code gen/` | ✅ | ✅ green |
| 3-05-01 | 05 | 5 | REQ-cross-spine-search | — | Agent-facing guidance ships with the verb (engram convention `yaj7dqz9qq`) | doc | `rg -n 'cross_spine' skill/engram/skills/curating-memory/SKILL.md CLAUDE.md docs-site/src/content/docs/reference/tools.md` (3 / 1 / 10 hits) | ✅ | ✅ green |
| AUDIT-01 | — | audit | REQ-cross-spine-authz-verified | T-03-01 | The store-side authz gate cannot be skipped away: with no Qdrant reachable **and** `ENGRAM_REQUIRE_QDRANT` set, `internal/store` FAILS instead of skipping — so row 3-01-01 cannot report vacuously green in CI | unit | `go test ./internal/store/... -run 'TestRequireQdrant|TestDialTestClientSkipsWhenNotRequired|TestDialTestClientFailsWhenRequiredAndUnavailable' -v` | ✅ `internal/store/store_test.go` | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `internal/store/store_test.go` — `TestCrossSpineAuthzIsolation` (the primary non-vacuous authz gate; lands and passes BEFORE any filter edit, per D-18)
- [x] `internal/store/store_test.go` — `TestSearchCrossSpine`, `TestListCrossSpine`, `TestListCrossSpineTotal` (the wiring proofs; land after the D-05 filter edit)
- [x] `internal/server/tools_test.go` — `TestEffectiveSearchScope`, `TestSearchMemoryCrossSpineIsolation`, `TestCrossSpineResultScope`, `TestSearchedScopesReporting`
- [x] `internal/server/connectapi_crossspine_test.go` — `TestConnectCrossSpineNotInferred`, `TestSearchMemoriesConnectCrossSpine`
- [x] `internal/store/store_test.go` — **added by this audit** (AUDIT-01): package-local `requireQdrant()`, wired into `TestMain` and `dialTestClient`, plus `TestRequireQdrant`, `TestDialTestClientSkipsWhenNotRequired`, `TestDialTestClientFailsWhenRequiredAndUnavailable`

*No framework install required — `testcontainers-go/modules/qdrant` is already a test dependency and
`TestMain` already provisions Qdrant for the whole `internal/store` package. This phase's
zero-new-dependency constraint holds: `go.mod` / `go.sum` must show a zero diff.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| RED-by-mutation transcript for `TestCrossSpineAuthzIsolation` | REQ-cross-spine-authz-verified | A test cannot assert its own falsifiability. D-15 requires the RED be *observed* by mutating the authz clause — a permanent automated version would have to ship the mutation | 1. Confirm `go test ./internal/store/... -run TestCrossSpineAuthzIsolation -v` is GREEN. 2. Temporarily replace the test's `Must: []*qdrant.Condition{s.ownerOrSharedCondition(...)}` with `Must: []*qdrant.Condition{}` (an empty `Must` matches every record). 3. Re-run — it MUST fail with owner B's private record present. 4. Restore. 5. Re-run — GREEN. Paste the full RUN/FAIL/PASS transcript into the plan's verification notes. **Toggling `cross_spine` is NOT an acceptable substitute** — that yields the vacuous green this whole gate exists to prevent |

---

## Known Precision Note (not a gap)

`Store.ListScopes` applies the authz predicate **alone** — no recall-window, superseded-soft-hide, tag,
or category conditions. So `searched_scopes` can name a scope that contributed zero hits to a given
query (e.g. every record in it is superseded). This is **correct for criterion 5's stated purpose**:
the field reports the *span the query covered*, which is exactly what distinguishes "found nothing
here" from "searched everywhere I can see and found nothing". It is not a hit-distribution report.
The docs task (3-05-01) must word it that way; a test asserting `searched_scopes` equals "scopes with
results" would be asserting the wrong contract.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 60s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-08-02 — retroactive audit, 1 gap found and resolved

---

## Validation Audit 2026-08-02

| Metric | Count |
|--------|-------|
| Gaps found | 1 |
| Resolved | 1 |
| Escalated | 0 |

All 13 contracted rows resolved to satisfied gates: 10 Go tests green (4 in `internal/store` against
real Qdrant, 6 in `internal/server`), the RED-by-mutation transcript recorded in `03-RED-TRANSCRIPT.md`,
`listFilter` present in `03-AUTHZ-GATE.md` (5 hits), `proto:gen` zero drift, and `cross_spine`
documented across the skill / CLAUDE.md / tools reference (3 / 1 / 10 hits).

**GAP (resolved) — the store-side authz gate was vacuous-capable.** `internal/server` honors
`ENGRAM_REQUIRE_QDRANT` (fails instead of skipping when no Qdrant is reachable), and CI's `test` job
sets it — but `internal/store` ignored the flag entirely: `dialTestClient` called `t.Skip`
unconditionally whenever `testQdrantAddr` was empty. Since CI runs `go test ./...`, a store-side
container failure would have silently skipped every store integration test — including row 3-01-01,
`TestCrossSpineAuthzIsolation`, this phase's **primary** non-vacuous cross-spine isolation proof —
while the package still reported `ok`.

This was not hypothetical. The two packages provision Qdrant independently, and they demonstrably
diverged during this audit: `internal/store` provisioned successfully while `internal/server`'s
attempt failed and skipped, in the same working tree, minutes apart. The usual assumption that a
container failure would redden CI via `internal/server` is coupling, not a guarantee.

Closed by adding a package-local `requireQdrant()` to `internal/store/store_test.go` — mirroring
`internal/server/tools_test.go`'s parser exactly, including its refusal to coerce an invalid value to
`false` — wired into both `TestMain` and `dialTestClient`. Flag unset preserves today's skip behavior
so local development without Docker is unaffected.

Verification of the fix, all three directions, run independently of the generated tests:

- **Fails closed:** `ENGRAM_REQUIRE_QDRANT=treu go test ./internal/store/... -run TestCrossSpineAuthzIsolation`
  → `fatal: ENGRAM_REQUIRE_QDRANT: invalid value "treu"` + `FAIL`, proving the gate is reachable from
  `TestMain` and refuses to coerce a malformed value into the permissive branch.
- **Flag unset + Qdrant available:** `--- PASS: TestCrossSpineAuthzIsolation` (unchanged behavior).
- **Flag set + Qdrant available:** `--- PASS: TestCrossSpineAuthzIsolation`.
- The no-Qdrant-with-flag-set path is proven by `TestDialTestClientFailsWhenRequiredAndUnavailable`,
  which re-execs a helper subprocess and asserts it exits non-zero with the fail-closed message and
  emits no `--- SKIP:`. A direct attempt to prove this by pointing `DOCKER_HOST` at a dead socket was
  inconclusive — Docker Desktop overrides that variable and the container booted regardless — which is
  itself the reason the subprocess approach was necessary.

Related engram memory: `478rhhmhb0` documented this asymmetry as a gotcha; this audit converts the
gotcha into an enforced gate.
