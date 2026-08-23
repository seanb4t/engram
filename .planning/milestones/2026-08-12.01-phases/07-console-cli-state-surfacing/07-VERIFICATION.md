---
phase: 07-console-cli-state-surfacing
verified: 2026-08-21T03:15:36Z
status: passed
score: 3/3 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 7: Console & CLI State Surfacing Verification Report

**Phase Goal:** A record's full state — archived, superseded, scheduled, and its schema
version — plus the collection's pending-migration state, is reachable and legible from the
operator console and the CLI.
**Verified:** 2026-08-21T03:15:36Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | REQ-console-record-state: console shows archived/superseded/scheduled state, previously unrenderable | ✓ VERIFIED | `MemoryRow.svelte:6,52-53` computes `memoryStateWords`/`isPastState` from the same `ui/src/lib/memorystate.ts` derivation; `MemoryDetail.svelte:12,48` renders a State section gated on `stateWords`, with `supersededBy`/`supersedes` as activatable links (:150-153). `ScopesSidebar.svelte` carries the three include toggles. All wiring confirmed live (imports + call sites), not orphaned. |
| 2 | REQ-cli-record-state: `search`/`list`/`get` surface the same state so CLI and console agree | ✓ VERIFIED | `cmd/engram/memory_state.go` is the CLI's single derivation, structurally identical to the console's (see Claim 5 below — canonical order, expired/scheduled precedence, and boundary comparisons match exactly). `renderMemoryTable` (`client_common.go:471-478`) emits an unconditional STATE column via `memoryStateCell`. `engram get` (`client_get.go`) renders via `renderOperatorView`+`getHeadline`, using the same `memoryStateWords`. |
| 3 | REQ-migration-state-visible: pending-migration state visible through the same surfaces, not only via `migrate` | ✓ VERIFIED | New Connect RPC `MigrateStatus` (`proto/engram/v1/engram.proto:357`, handler `internal/server/connectapi.go:199-221`) returns the whole-collection histogram via the shared `MigrateStatusResult.Pending()` helper also used by the startup warning (`tools.go:500`). Console: `MigrationBanner.svelte` renders it on every route, silent at zero/loading/error (browser-tested). CLI: `engram migration-status` verb (`client_migration_status.go`) plus a footer on `search`/`list` (`client_common.go:331-337`, wired at `client_search.go:87-91`, `client_list.go:99-103`). |

**Score:** 3/3 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/store/store.go` | 3 orthogonal recall-gate opt-ins on Search+List | ✓ VERIFIED | `IncludeArchived`/`IncludeSuperseded`/`IncludeScheduled` on both `SearchOptions` (:1075-1077) and `ListOptions` (:1281-1283), each mapping 1:1 onto its own `Must` condition (:1112-1130, :1383-1400) |
| `cmd/engram/memory_state.go` | shared CLI state-word derivation | ✓ VERIFIED | `memoryStateWords`/`memoryStateCell`, canonical order, expired-precedence, Lte/Gt boundary discipline |
| `ui/src/lib/memorystate.ts` | shared console state-word derivation | ✓ VERIFIED | Matches CLI derivation exactly (order, precedence, boundaries) |
| `cmd/engram/operator_view.go` | headline routed through `sanitizeViewValue` | ✓ VERIFIED | `renderOperatorView` line 267 — real call, proven RED-on-revert (see Claim 4) |
| `cmd/engram/client_get.go` | `engram get <id>` client-tier verb | ✓ VERIFIED | Real cobra command, `addClientFlags`, renders via `renderOperatorView`/`renderJSON` with identical protojson options |
| `cmd/engram/client_migration_status.go` | `engram migration-status` client-tier verb | ✓ VERIFIED | Real command, own toolclass row, tested (`client_migration_status_test.go`) |
| `internal/server/connectapi.go` | `MigrateStatus` RPC handler | ✓ VERIFIED | Any-authenticated-caller, whole-collection, shared `Pending()` helper, classified errors via `connectError` |
| `ui/src/lib/components/MigrationBanner.svelte` | silent-at-zero/loading/error banner, behind/ahead distinct | ✓ VERIFIED | No error-logging `$effect`; delegates to centralized `handleQueryError`; two independently-gated strips |
| `ui/src/lib/components/ScopesSidebar.svelte` | 3 include toggles | ✓ VERIFIED | Checkboxes wired to `includeArchived`/`includeSuperseded`/`includeScheduled` props, round-tripped through `queries.ts`/`+page.svelte` |
| `ui/src/lib/components/MemoryDetail.svelte` | State section, `schema` chip | ✓ VERIFIED | State section gated on `stateWords`; `superseded_by`/`supersedes` render as full-UUID links |
| `ui/src/lib/components/MemoryRow.svelte` | state marker + dim treatment | ✓ VERIFIED | `memoryStateWords`/`isPastState` imported and used |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `internal/server/tools.go` (MCP `search_memory`/`list_memory` closures) | `coreSearchRequest{}`/`coreListRequest{}` | literal construction | ✓ WIRED, and deliberately NOT wired to include-flags | Closures at :2407-2410 and :2454-2465 never set `IncludeArchived`/`IncludeSuperseded`/`IncludeScheduled` — MCP `searchArgs`/`listArgs` (:670-694) carry no such jsonschema fields at all |
| `cmd/engram/client_search.go` / `client_list.go` | `migrationFooterCounts` → `renderMigrationFooter` | direct call | ✓ WIRED | :87-91, :99-103, footer bounded by `min(resolvedTimeout, 2s)` at exactly one production site (`migrationFooterContext`, `client_common.go:359`) |
| `ui/src/routes/+layout.svelte` | `handleQueryError` (`ui/src/lib/errors.ts`) | `QueryCache({ onError: handleQueryError })` | ✓ WIRED | Direct function reference, not an inline closure — confirmed at `+layout.svelte:20-21` |
| `ui/src/lib/components/ScopesSidebar.svelte` | `ui/src/routes/observe/+page.svelte` → `ui/src/lib/queries.ts` | prop → URL parse/serialize → query cache key | ✓ WIRED | `parseObserveParams`/`observeSearch` and `listMemoriesKey` all include the three flags (`queries.ts:15,27-29,39,49,51`) |
| `internal/store/store.go Search/List` | `internal/server/connectapi.go` `SearchMemories`/`ListMemories` | field threading | ✓ WIRED (07-01/07-03 SUMMARY + direct read) | Connect request fields feed `coreSearchRequest`/`coreListRequest` feed `SearchOptions`/`ListOptions` |

### Specific Claims Verification (per task brief)

| # | Claim | Verdict | Evidence |
|---|-------|---------|----------|
| 1 | Recall-gate relaxation is opt-in, 1:1, not folded into `include_hidden` | ✓ VERIFIED | `store.go` reads confirm 3 independent `if !opts.IncludeX` guards at both Search (:1112,1119,1128) and List (:1383,1390,1398) sites, each appending exactly one condition |
| 2 | MCP is unchanged | ✓ VERIFIED | `searchArgs`/`listArgs` structs (`tools.go:670-694`) have no include_* fields; MCP closures leave `coreSearchRequest{}`/`coreListRequest{}` include fields at zero value |
| 3 | Authz stays orthogonal — cross-owner test with include flags set | ✓ VERIFIED | `TestSearchAndListAuthorizationOrthogonalToState` (`store_test.go:7285`) — ran it directly, PASS. Private records stay excluded for a non-owning caller even with all 3 flags true; shared archived/superseded records become visible |
| 4 | #505 headline sanitization is structurally fixed, with a regression test that fails if removed | ✓ VERIFIED | `renderOperatorView` line 267 is a real `sanitizeViewValue(headline)` call. `TestRenderOperatorViewSanitizesHeadline` (`operator_view_test.go:567`) passes at HEAD. I reverted the call locally (`fmt.Fprintln(w, headline)`) and re-ran the test — it went RED (2 subtests failed on raw ESC/DEL bytes), then restored the file via `git diff` clean-check |
| 5 | State vocabulary agrees across CLI and console (same 4 words, same canonical order) | ✓ VERIFIED, no divergence | Both `cmd/engram/memory_state.go` and `ui/src/lib/memorystate.ts` use identical order `[archived, superseded, expired, scheduled]`, identical expired-suppresses-scheduled precedence, and identical boundary comparisons (`not_before` Lte-inclusive-no-word / `not_after` Gt-exclusive-expired) |
| 6 | Banner silent at zero/loading/error, behind/ahead distinct, both test-covered | ✓ VERIFIED | `MigrationBanner.browser.test.ts` has 7 tests covering exactly these cases; ran them directly — all 7 PASS. Behind/ahead render as two independently-gated, differently-worded, differently-styled strips in fixed order |
| 7 | No vacuous gates — call/declaration-shaped evidence spot-checked | ✓ VERIFIED | Spot-checked the #505 gate by actually breaking and re-running it (see #4); spot-checked the ListScheduled 2-of-4 exclusion the same way — `TestArchiveRecallGateListScheduled` explicitly re-tests with all three include bools set true and still asserts the archived record stays excluded, which is precisely the "completes the 2-of-4 by accident" regression this repo has documented as a recurring defect class |

### Requirements Coverage

| Requirement | Source Plan(s) | Status | Evidence |
|-------------|----------------|--------|----------|
| REQ-cli-record-state | 07-01, 07-03, 07-05, 07-06 | ✓ SATISFIED | STATE column, `engram get`, migration-status verb + footer, all real |
| REQ-console-record-state | 07-02, 07-03, 07-04 | ✓ SATISFIED | State badges, State section, include toggles, all real and wired |
| REQ-migration-state-visible | 07-06, 07-07 | ✓ SATISFIED | Connect RPC, CLI verb + footer, console banner, all real and wired |

No orphaned requirements found in REQUIREMENTS.md for this phase.

### Anti-Patterns Found

Scanned every file named in the 07-01..07-07 SUMMARY key-files sections (66 unique paths) for
`TBD`/`FIXME`/`XXX`, `TODO`/`HACK`/`PLACEHOLDER`, and "not yet implemented"/"coming soon"
phrasing. **None found.** No debt markers, no blockers.

### Verification Commands Run

| Command | Result |
|---------|--------|
| `go build ./...` | clean, no output |
| `go test ./...` | all packages `ok` (internal/store 86.6s, full suite green) |
| `cd ui && npm run test` | 32 files, 263 tests, all passed |
| `task license:check` | 1596 files checked, 0 invalid |
| `git diff --exit-code gen/ cmd/engram/testdata/` | exit 0, no drift |
| `go test ./internal/store/ -run TestSearchAndListAuthorizationOrthogonalToState -v` | PASS |
| `go test ./cmd/engram/ -run TestRenderOperatorViewSanitizesHeadline -v` (then reverted the fix and re-ran) | PASS at HEAD; **RED** when `sanitizeViewValue` call removed, confirming the test is load-bearing |
| `go test ./internal/store/ -run TestSchemaVersion...` | PASS (recall-gate reachability re-derivation) |
| `go test ./internal/store/ -run "TestArchiveRecallGate...|TestListIncludeSupersededAndScheduled|TestSearchIncludeSupersededAndScheduled"` | PASS |
| `cd ui && npx vitest run src/lib/memorystate.test.ts` | 15/15 PASS |
| `cd ui && npx vitest run src/lib/components/MigrationBanner.browser.test.ts` | 7/7 PASS |

### Human Verification Required

None. All must-haves and specific claims were verifiable directly against source and by
running targeted tests; no visual, real-time, or external-service-dependent behavior was left
unresolved. The two items requiring a live server + Qdrant (07-04's `/observe?...&inc=archived`
round-trip and 07-07's visual banner check) were already deferred to phase UAT by the executors
and are documented in `deferred-items.md` — not reported here as gaps, per the task's
`known_not_failures` list.

### Gaps Summary

None. All three requirements (REQ-console-record-state, REQ-cli-record-state,
REQ-migration-state-visible) are satisfied by real, wired, tested code — not stubs or
comment-only markers. The recall-gate relaxation is correctly scoped to Connect+CLI only, with
MCP structurally unable to set the new flags. Authorization isolation was proven, not merely
asserted, by directly running the cross-owner test. The #505 headline-sanitization fix was
proven load-bearing by deliberately breaking and restoring it. The CLI and console state
vocabularies were read side by side and found identical in order, precedence, and boundary
semantics. The migration banner's three silence conditions and its behind/ahead distinction are
all covered by passing browser tests. No debt markers, no vacuous gates, no codegen drift, no
license violations, all builds and test suites green.

**Overall verdict: PASS.**

---

_Verified: 2026-08-21T03:15:36Z_
_Verifier: Claude (gsd-verifier)_
