---
phase: 20-correctness-polish
verified: 2026-07-16T00:20:00Z
status: passed
score: 6/6 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 20: Correctness & Polish Verification Report

**Phase Goal:** A cluster of independent correctness gaps identified during v0.9.x code review are closed, each removing a specific silent-failure or drift risk.
**Verified:** 2026-07-16T00:20:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth (ROADMAP Success Criterion) | Status | Evidence |
|---|---|---|---|
| 1 | `SearchDiscoveries` returns `kind`, `citations`, `summary` over the Connect wire (#307) | VERIFIED | `proto/engram/v1/engram.proto:40-41` adds `string kind = 21;` / `repeated Citation citations = 22;` strictly after `last_accessed_at = 20`. `gen/go/engram/v1/engram.pb.go:111-112` exposes `Memory.Kind`/`Memory.Citations`. `internal/server/connectapi.go:35` defines `citationsToProto` (read path — confirmed absent from `protoconv.go`), and `memoryToProto` (L65-66) sets `Kind: m.Kind, Citations: citationsToProto(m.Citations)`. `summary` mapping untouched (D-03). Tests `TestMemoryToProtoMapsKindAndCitations` and `TestMemoryToProtoZeroValueKindAndCitations` pass (`go test ./internal/server/...`). `go tool buf generate && git diff --exit-code -- gen/` clean (zero drift). |
| 2 | `MintShortID` gives up with an explicit exhaustion error after a bounded retry cap (#308) | VERIFIED | `internal/store/store.go:62` `ErrShortIDExhausted = errors.New(...)`; `store.go:67` `const maxMintAttempts = 16`; bounded `for attempts := 0; attempts < maxMintAttempts; attempts++` loop (L1790-1821) performs real `Count()` checks and returns `fmt.Errorf("%w: after %d attempts", ErrShortIDExhausted, maxMintAttempts)` on exhaustion. `seen`-map dup hits execute `attempts--` before `continue` (L1798), exempting them from the budget per D-05. `TestMintShortIDExhaustsAfterCap` and `TestMintShortIDSeenMapDoesNotConsumeBudget` pass, plus the pre-existing `TestMintShortIDRetriesOnCollision` is unaffected — all confirmed via a live run (`go test ./internal/store/... -run TestMintShortID -v`, testcontainer-backed Qdrant). |
| 3 | `config.ParseEmbedParams` and the embed wire contract share one reserved-param-key list (#304) | VERIFIED | `internal/config/embedparams.go:24` exports canonical `ReservedEmbedParamKeys = []string{"model","input"}`; `ParseEmbedParams`'s reject loop (L40-44) ranges over it. `internal/embed/embed.go:144` `var ReservedParamKeys = config.ReservedEmbedParamKeys` aliases the canonical list. The executor inverted the plan's stated direction (config imports embed) to avoid a real cycle (`internal/embed` → `internal/telemetry` → `internal/config` → planned `internal/embed` edge); this is a correct, documented, in-spirit deviation — `go build ./...` and `go vet ./...` are clean, confirming no cycle exists. `TestParseEmbedParams` (config) and `TestEmbedderFromConfigPassesParamsAndInstructions` (server) pass. |
| 4 | `embed.Client.embed()` builds its body via a single map-based path (#302) | VERIFIED | `internal/embed/embed.go:215-241`: `embed()` now allocates one `map[string]any`, merges operator params, sets `model`/`input` authoritatively last, and calls `json.Marshal(m)` once. `rg -n 'type embedReq' internal/embed/embed.go` returns nothing — the struct-marshal branch and `embedReq` type are gone. `go test ./internal/embed/...` (`TestEmbedParamsMergedIntoBody` et al.) passes. |
| 5 | `storeDiscoveryArgs.ID`'s jsonschema advertises `short_id` (#303) | VERIFIED | `internal/server/tools.go:550` (unchanged, pre-existing since commit 92a6f610/PR #288) `jsonschema:"...supply the full UUID or short_id to replace in place"`. New regression test `TestStoreDiscoveryArgsIDSchemaAdvertisesShortID` (`internal/server/tools_test.go:2221-2230`) reflects on the `ID` field's tag and asserts `strings.Contains(tag, "short_id")` — passes. `git diff internal/server/tools.go` on this phase's commits is empty, confirming this was correctly treated as verification-only (no production change), exactly as the plan specified. |
| 6 | Helm chart ships `engram summarize-missing` as a `batch/v1` CronJob reusing the Deployment's image/env via a shared `_helpers.tpl` block (#269) | VERIFIED | `charts/engram/templates/_helpers.tpl:1` defines `engram.containerEnv`; `charts/engram/templates/memory-mcp.yaml:30` consumes it via `include "engram.containerEnv" . | nindent 12` (Deployment). `charts/engram/templates/summarize-cronjob.yaml` is gated behind `.Values.memory.summarize.cronjob.enabled` (D-07, default false — confirmed via `values.yaml:106`), declares `apiVersion: batch/v1` / `kind: CronJob`, includes the same `engram.containerEnv` at nindent 16, sets `concurrencyPolicy`/`restartPolicy`/history limits/`schedule` from values (D-08 defaults: Forbid/OnFailure/3/1/daily `0 3 * * *`), and runs `args: ["summarize-missing", "--all-scopes"]`. `task chart:validate` (new Taskfile target) passes live: default render emits no CronJob, `--set ...enabled=true` render emits one with `Forbid` + the daily schedule, `helm lint charts/engram` is clean, and the `engram.containerEnv` sha256 drift pin matches. |

**Score:** 6/6 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `proto/engram/v1/engram.proto` | additive `Memory.kind=21`/`.citations=22` | VERIFIED | Confirmed lines 40-41, no other Memory-message lines touched |
| `internal/server/connectapi.go` | `citationsToProto` helper + extended `memoryToProto` | VERIFIED | L35 helper, L65-66 wiring |
| `internal/server/tools_test.go` | `TestStoreDiscoveryArgsIDSchemaAdvertisesShortID` | VERIFIED | Present, passes |
| `gen/go`, `gen/ts`, `ui/src/lib/gen`, `internal/webauth/static` | regenerated, drift-clean | VERIFIED | `git diff --exit-code -- gen/` clean; working tree clean except unrelated `.planning/config.json` |
| `internal/store/store.go` | `ErrShortIDExhausted` + bounded `MintShortID` | VERIFIED | L58-67, L1774-1821 |
| `internal/store/store_test.go` | `TestMintShortIDExhaustsAfterCap` + seen-map variant | VERIFIED | Present, both pass |
| `internal/embed/embed.go` | `ReservedParamKeys` + single-path `embed()` | VERIFIED | L144, L215-241; `embedReq` type removed |
| `internal/config/embedparams.go` | shares reserved-key list | VERIFIED | L24 canonical, L40 consumed |
| `charts/engram/templates/_helpers.tpl` | `engram.containerEnv` named template | VERIFIED | Present |
| `charts/engram/templates/summarize-cronjob.yaml` | opt-in `batch/v1` CronJob | VERIFIED | Present, gated, correct defaults |
| `charts/engram/values.yaml` | `memory.summarize.cronjob` block | VERIFIED | L105-111 |
| `Taskfile.yaml` | `chart:validate` target | VERIFIED | Present, passes live |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `store.Memory.Kind/.Citations` | `engramv1.Memory.Kind/.Citations` | `memoryToProto`/`citationsToProto` | WIRED | Confirmed by passing round-trip test `TestMemoryToProtoMapsKindAndCitations` |
| `proto:gen` | `ui:build` embedded SPA | `gen/ts` → `ui/src/lib/gen` | WIRED | `git diff --exit-code -- gen/` clean; no uncommitted webauth/static drift |
| `embed.ReservedParamKeys` | `config.ParseEmbedParams` reject loop | shared canonical slice (`config` owns it, `embed` aliases) | WIRED | `go build ./...` clean (no import cycle); both packages consume the identical slice value |
| `engram.containerEnv` named template | Deployment + CronJob | `include` at nindent 12 / nindent 16 | WIRED | Both consumers confirmed via `rg`; `task chart:validate` proves render correctness live |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| MintShortID exhaustion + seen-map budget | `go test ./internal/store/... -run TestMintShortID -v` | 4/4 pass (Unique, RetriesOnCollision, ExhaustsAfterCap, SeenMapDoesNotConsumeBudget) | PASS |
| memoryToProto kind/citations round-trip | `go test ./internal/server/... -run 'TestMemoryToProto|TestEngramServiceDescriptor|TestStoreDiscoveryArgsIDSchemaAdvertisesShortID' -v` | all pass | PASS |
| embed()/ParseEmbedParams shared-list behavior | `go test ./internal/embed/... ./internal/config/... -v` | all pass | PASS |
| proto/gen drift | `go tool buf generate && git diff --exit-code -- gen/` | clean, exit 0 | PASS |
| Helm chart correctness | `task chart:validate && helm lint charts/engram` | both exit 0 | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|---|---|---|---|---|
| REQ-discovery-proto-fidelity | 20-01 | `SearchDiscoveries` carries kind/citations/summary | SATISFIED | Truth #1 above |
| REQ-discovery-shortid-schema | 20-01 | `storeDiscoveryArgs.ID` jsonschema advertises short_id | SATISFIED | Truth #5 above |
| REQ-embed-param-key-sharing | 20-02 | shared reserved-key list | SATISFIED | Truth #3 above |
| REQ-embed-body-build-collapse | 20-02 | single-path `embed()` body build | SATISFIED | Truth #4 above |
| REQ-shortid-mint-cap | 20-03 | bounded `MintShortID` + exhaustion error | SATISFIED | Truth #2 above |
| REQ-summarize-cronjob | 20-04 | Helm CronJob for summarize-missing | SATISFIED | Truth #6 above |

All 6 requirement IDs named in the phase are accounted for across the 4 plans; REQUIREMENTS.md marks all 6 `[x]`. No orphaned requirements found for Phase 20.

### Anti-Patterns Found

None in the files this phase modified — `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` scan across all 16 touched files (proto, connectapi.go/_test.go, tools_test.go, connectdescriptor_test.go, embed.go/_test.go, embedparams.go/_test.go, store.go/_test.go, `_helpers.tpl`, `summarize-cronjob.yaml`, `memory-mcp.yaml`, `values.yaml`, `Taskfile.yaml`) returned zero matches.

### Notable Discrepancy (non-blocking — flagged for orchestrator)

SUMMARY.md narratives for plans 02/03/04 assert their tracked GitHub issues are "closed" (e.g. 20-03: "#308 closed"; 20-02: "#304 and #302 closed"; 20-04: "Closes #269 and the last plan in Phase 20 ... all four plans ... are now complete"). Live `gh issue view` shows **#308, #304, #302, #269 are still OPEN** (only #307 and #303 — both from Plan 01 — are actually closed, and #303's plan explicitly required and verified closure). None of Plan 02/03/04's task `acceptance_criteria` required a `gh issue close` step (unlike Plan 01 Task 3), so this does not fail any must-have artifact/truth in those plans' own contracts, and it does not affect the phase's code-correctness goal (all 6 ROADMAP success criteria are independently code-verified above). It is flagged here because it is exactly the kind of SUMMARY-vs-reality gap this verification exists to catch — the four issues should be closed (citing the relevant commits) before this branch ships/merges.

### Human Verification Required

None. All six success criteria are code-observable and were verified directly against source, generated artifacts, and live test/tool runs (no visual, UX, or external-service-dependent behavior in this phase's scope).

### Gaps Summary

No gaps. All 6 ROADMAP success criteria for Phase 20 are verified true in the codebase, all 6 requirement IDs are accounted for, `go build`/`go vet`/`golangci-lint`/`go test ./...` (targeted, live-run) are clean, `buf` generate produces zero drift, and `helm lint` + `task chart:validate` pass live. The only finding is the non-blocking GitHub issue-closure discrepancy noted above.

---

_Verified: 2026-07-16T00:20:00Z_
_Verifier: Claude (gsd-verifier)_
