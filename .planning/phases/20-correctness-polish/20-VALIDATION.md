---
phase: 20
slug: correctness-polish
status: approved
nyquist_compliant: true
wave_0_complete: false
created: 2026-07-15
---

# Phase 20 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: `20-RESEARCH.md` §Validation Architecture. Per-task IDs are assigned by the planner;
> this draft seeds the map by requirement.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib) + testcontainers-Qdrant for live-store tests; `helm lint` / `helm template` for chart validation |
| **Config file** | none — `go test ./...` via Taskfile (`task test`); chart validation via a new `chart:lint`-adjacent script (no new tooling dependency) |
| **Quick run command** | `go test ./internal/store/... ./internal/embed/... ./internal/config/... ./internal/server/... -run <Test>` |
| **Full suite command** | `task` (lint + test) |
| **Estimated runtime** | ~60–120s for scoped unit tests; full `task` longer (includes lint + testcontainers) |

---

## Sampling Rate

- **After every task commit:** Run the scoped `go test ./internal/<pkg>/... -run <Test>` for the package that task touched
- **After every plan wave:** Run `task` (full lint + test) plus `helm lint charts/engram` and `helm template charts/engram` (both default-disabled and `--set memory.summarize.cronjob.enabled=true`)
- **Before `/gsd-verify-work`:** Full suite must be green, plus a `git diff` / rendered-diff confirming the Deployment env is byte-identical pre/post the `_helpers.tpl` factor-out
- **Max feedback latency:** ~120 seconds for scoped tests

---

## Per-Task Verification Map

> Task IDs are positional (`20-<plan>-<task>`) per the finalized PLAN.md files. Wave-0 test files are
> authored during execution; `wave_0_complete` flips true once they exist.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 20-01-02 | 20-01 | 1 | REQ-discovery-proto-fidelity | — | New `kind`/`citations` ride existing authz-gated `SearchDiscoveries` (no new read surface) | unit | `go test ./internal/server/... -run TestMemoryToProto -v` (assert `Kind`/`Citations` round-trip) | ❌ W0 | ⬜ pending |
| 20-01-01 | 20-01 | 1 | REQ-discovery-proto-fidelity | — | proto/gen drift stays clean | ci-gate | `go tool buf lint && go tool buf breaking --against '.git#branch=main' && go tool buf generate && git diff --exit-code -- gen/` | ✅ existing `buf` CI job | ⬜ pending |
| 20-01-03 | 20-01 | 1 | REQ-discovery-shortid-schema | — | `storeDiscoveryArgs.ID` jsonschema contains `short_id` (verification-only — already correct since `92a6f610`) | unit | `go test ./internal/server/... -run TestStoreDiscoveryArgsSchema -v` (new — pins the literal tag string) | ❌ W0 | ⬜ pending |
| 20-02-02 | 20-02 | 1 | REQ-embed-param-key-sharing | — | `config.ParseEmbedParams` rejects reserved keys sourced from `embed.ReservedParamKeys` (single shared list) | unit | `go test ./internal/config/... -run TestEmbedParams -v` (extend to reference shared symbol) | ✅ extend | ⬜ pending |
| 20-02-01 | 20-02 | 1 | REQ-embed-body-build-collapse | — | single-path `embed()` decodes to correct body for empty AND non-empty params | unit | `go test ./internal/embed/... -run TestEmbedParamsMergedIntoBody -v` (existing, decode-based) | ✅ existing | ⬜ pending |
| 20-03-01 | 20-03 | 1 | REQ-shortid-mint-cap | — | `MintShortID` returns `ErrShortIDExhausted` after 16 real collision checks; `seen`-map dups do not consume budget | unit | `go test ./internal/store/... -run TestMintShortIDExhaust -v` (new; mirrors `TestMintShortIDRetriesOnCollision` store_test.go:2741) | ❌ W0 | ⬜ pending |
| 20-04-03 | 20-04 | 1 | REQ-summarize-cronjob | — | `helm template` renders a valid `batch/v1` CronJob when enabled; Deployment env unchanged (no-op diff) | integration/render | `task chart:validate` — assert no `kind: CronJob` when disabled, `kind: CronJob` + `schedule`/`concurrencyPolicy` when enabled, and `engram.containerEnv` block drift pin (rendered via 20-04-01/02) | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/server/connectapi_test.go` (or the current `memoryToProto` test file) — add a case asserting `Kind`/`Citations` round-trip through `memoryToProto` / `memoriesToProto`
- [ ] `internal/store/store_test.go` — add `TestMintShortIDExhaustsAfterCap` (force every candidate to collide; assert exactly 16 real `Count` calls then `ErrShortIDExhausted`; a pre-populated `seen` map must NOT consume the budget)
- [ ] `internal/server/tools_test.go` — add an assertion pinning the `storeDiscoveryArgs.ID` jsonschema string (currently unasserted — nothing catches a regression that silently drops the `short_id` wording, which is how #303 was originally filed)
- [ ] Chart validation (no existing harness): a **lightweight** Taskfile target or bash script (grep/diff over `helm template`), **NOT** a full `helm-unittest` plugin dependency — proportional to one chart

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Deployment env is byte-identical after the `_helpers.tpl` factor-out | REQ-summarize-cronjob | Best confirmed by a human-read `git diff` of the rendered Deployment at the phase gate (the scripted diff automates it, but a final eyeball is the sign-off) | `diff <(git stash && helm template charts/engram --show-only templates/memory-mcp.yaml; git stash pop) <(helm template charts/engram --show-only templates/memory-mcp.yaml)` → expect empty |

*All other phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies (verified by gsd-plan-checker)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (4 new/extended test files above)
- [x] No watch-mode flags
- [x] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter
- [ ] `wave_0_complete: true` — flips during execution once the 4 Wave-0 test files exist

**Approval:** approved 2026-07-15 (plans verified by gsd-plan-checker; Wave-0 test files authored at execution)
