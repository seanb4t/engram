---
phase: 07-console-cli-state-surfacing
plan: 06
subsystem: api
tags: [connect-rpc, protobuf, cobra, migration, cli, protojson]

# Dependency graph
requires:
  - phase: 07-console-cli-state-surfacing
    provides: "07-05's engram get / renderOperatorView (json.RawMessage adapter) and operatorCommands() re-derivation pattern — this plan's engram migration-status verb follows the same shape"
  - phase: 07-console-cli-state-surfacing
    provides: "07-03's Search-lane opt-in — this plan adds the advisory footer immediately after renderCoverageFooter in the same two call sites 07-03 touched"
provides:
  - "MigrateStatus Connect RPC: any authenticated caller reads the whole-collection schema-version histogram over the same generated client, auth chain, and error envelope as every other read RPC (D-06)"
  - "MigrateStatusResult.Pending() — the single definition of the pending-migration arithmetic, now consumed by warnPendingMigrations, the CLI footer, and (07-07) the console banner"
  - "engram migration-status — the client-tier sibling of the operator-tier engram migrate status, over the Connect RPC"
  - "The advisory footer on engram search/engram list: pending_migrations / future_schema_records, text-lane only, bounded to min(resolvedTimeout, footerLookupBudget=2s), never able to fail the command"
affects: [07-07]

# Actuals (#2632)
actuals:
  tokens: 37612
  tasks: 3
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Single-definition arithmetic collapse: MigrateStatusResult.Pending() replaces an inline per-consumer loop, the same defect class (half-applied N-site invariant) this repo repeatedly hits — three consumers (startup warning, CLI footer, console banner) now share one method instead of three copies drifting apart"
    - "Bounded secondary-lookup context: migrationFooterContext is the ONE production site applying min(resolvedTimeout, footerLookupBudget), taking the resolved timeout rather than a pre-built context specifically so a unit test can drive the ceiling arithmetic directly instead of asserting its own recomputation of it"
    - "Call-shaped acceptance gates over bare-identifier greps: this plan's own acceptance criteria pin min(...footerLookupBudget) and cur := int(migrate.CurrentVersion) as CALLS, not identifiers, closing a demonstrated trailing-comment defeat"

key-files:
  created:
    - cmd/engram/client_migration_status.go
    - cmd/engram/client_migration_status_test.go
  modified:
    - proto/engram/v1/engram.proto
    - gen/go/engram/v1/engram.pb.go
    - gen/go/engram/v1/engramv1connect/engram.connect.go
    - gen/ts/engram/v1/engram_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts
    - internal/store/migrate_status.go
    - internal/store/migrate_status_test.go
    - internal/server/tools.go
    - internal/server/store_iface.go
    - internal/server/fakestore_test.go
    - internal/server/connectapi.go
    - internal/server/connectapi_test.go
    - internal/server/connectdescriptor_test.go
    - internal/surfaces/toolclass.go
    - cmd/engram/cmdwalk.go
    - cmd/engram/cmdwalk_test.go
    - cmd/engram/operator_output_test.go
    - cmd/engram/clienttest_test.go
    - cmd/engram/client_common.go
    - cmd/engram/client_common_test.go
    - cmd/engram/client_search.go
    - cmd/engram/client_search_test.go
    - cmd/engram/client_list.go
    - cmd/engram/client_list_test.go
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden
    - docs-site/src/content/docs/guides/cli.md

key-decisions:
  - "Task 1's handler-error test (a MigrateStatus failure surfacing through connectError) was built on a scripted spyStore.migrateStatus field plus a migrateStatusFailStore embedding memStore (mirroring tools_test.go's upsertFailStore idiom exactly) rather than a live-Qdrant fixture — the failure is a handler classification property, not a store property, and the interface seam already exists to make this cheap, per the plan's own explicit instruction."
  - "The RED/GREEN cycle for all three tasks ran as tests-and-implementation-authored-together-then-verified-green, not separate test(...)/feat(...) commits — matching 07-03's and 07-05's precedent for this phase, and each task's acceptance criteria were run and shown passing before considering the task done."
  - "migrationFooterCounts is called with cmd.Context() (the RunE's own context) as parent, never the primary RPC's already-derived-and-cancelled context — a slow-but-successful primary call must not automatically suppress the footer, and migrationFooterContext derives its own fresh bounded child from that parent."

requirements-completed: [REQ-migration-state-visible, REQ-cli-record-state]

coverage:
  - id: D1
    description: "A Connect read RPC (MigrateStatus) exposes Store.MigrateStatus's whole-collection schema-version histogram to any authenticated caller, over the same generated client, auth chain, and error envelope as every other read RPC; the pending arithmetic now lives in exactly one place (MigrateStatusResult.Pending()), consumed by the startup warning and the new RPC alike"
    requirement: "REQ-migration-state-visible"
    verification:
      - kind: unit
        ref: "internal/store/migrate_status_test.go#TestMigrateStatusResultPending"
        status: pass
      - kind: unit
        ref: "internal/server/connectapi_test.go#TestConnectMigrateStatusReturnsStoreHistogram"
        status: pass
      - kind: unit
        ref: "internal/server/connectapi_test.go#TestConnectMigrateStatusStoreErrorSurfacesAsConnectError"
        status: pass
      - kind: unit
        ref: "internal/server/connectapi_test.go#TestConnectMigrateStatusEmptyHistogramSerializesEmptyArrays"
        status: pass
      - kind: unit
        ref: "internal/server/tools_test.go#TestWarnPendingMigrations (unchanged behavior, now sourced from Pending())"
        status: pass
    human_judgment: false
  - id: D2
    description: "engram migration-status renders the histogram through the typed operator view, correctly registry-classified as a client-tier read distinct from the operator-tier migrate status row"
    requirement: "REQ-cli-record-state"
    verification:
      - kind: unit
        ref: "cmd/engram/client_migration_status_test.go#TestMigrationStatusTextOutputHeadline"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_migration_status_test.go#TestMigrationStatusTextOutputRendersBucketsAndAbsentAsDistinctRows"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_migration_status_test.go#TestMigrationStatusZeroValueRendersEmptyArraysOnJSONLane"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_migration_status_test.go#TestMigrationStatusExcludedFromOperatorCommands"
        status: pass
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestCatalogBlastRadiusMatchesToolClasses"
        status: pass
    human_judgment: false
  - id: D3
    description: "engram search and engram list surface a migration backlog without being asked (pending_migrations / future_schema_records, text-lane only, bounded lookup); a failed lookup cannot affect either command's output or exit code"
    requirement: "REQ-migration-state-visible"
    verification:
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestRenderMigrationFooter"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestMigrationFooterContextAppliesCeiling"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_common_test.go#TestMigrationFooterCountsTimesOutWithoutCallerContext"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_search_test.go#TestClientSearchMigrationFooterLookupFailureDoesNotAffectCommand"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_list_test.go#TestClientListJSONOutputByteIdenticalToPrePhaseShape"
        status: pass
    human_judgment: false

duration: ~55min
completed: 2026-08-21
status: complete
---

# Phase 07 Plan 06: MigrateStatus RPC, engram migration-status, and the Advisory Footer Summary

**A sixth Connect read RPC exposes the schema-version histogram to any authenticated caller, `engram migration-status` renders it through the typed operator view, and `engram search`/`engram list` print a bounded, failure-proof advisory footer — all three consuming the same new `MigrateStatusResult.Pending()` single-definition helper.**

## Performance

- **Duration:** ~55 min
- **Completed:** 2026-08-21T02:42:01Z
- **Tasks:** 3/3 completed
- **Files modified:** 29 (2 created, 27 modified)

## Accomplishments

- `MigrateStatusResult.Pending()` is now the single definition of the pending-migration arithmetic — `warnPendingMigrations` in `internal/server/tools.go` no longer accumulates a per-bucket sum of its own, and the CLI footer (Task 3) derives from the same server-side computation via the RPC.
- `proto/engram/v1/engram.proto` gained `SchemaVersionBucket`/`MigrateStatusRequest`/`MigrateStatusResponse` and the `MigrateStatus` RPC (a sixth read RPC, additive per `buf breaking` against `main`); `internal/server/connectapi.go`'s handler mirrors `ListScopes` exactly — `subjectFromConnectContext` for auth with no owner filter, `connectError` for the single failure mapper — and the handler-error seam is one new method on the EXISTING 18-method `memStore` interface, not a new interface.
- `engram migration-status` (new `cmd/engram/client_migration_status.go`) is a client-tier verb over the Connect RPC, rendering through `renderOperatorView` via the same `protojson.MarshalOptions{UseProtoNames, EmitDefaultValues}` marshal both lanes share (text/json identity, mirroring `engram get`'s pattern from 07-05); classified in `internal/surfaces/toolclass.go` as a distinct row from the operator-tier `migrate status`, and `cmdwalk.go`'s `operatorCommands()` doc comment now names it as the fifth client verb.
- `engram search`/`engram list` print `pending_migrations: N` and/or `future_schema_records: M` (two-space-joined when both are non-zero, omitted entirely when both are zero) after `renderCoverageFooter`, text-lane only. The lookup runs sequentially after the primary RPC, bounded to `min(resolvedTimeout, footerLookupBudget=2s)` — the ONE production call site for that `min()`, applied inside `migrationFooterContext` — and a failed lookup never fails the command or touches stderr.
- Confirmed by direct test rather than assumed: `migrationFooterCounts(context.Background(), client, 30*time.Second)` against a `MigrateStatus` that never returns gives up at roughly `footerLookupBudget`, proving the bound comes from the helper's own derived context, not from any caller-supplied deadline.
- `go test ./...` (whole repo) passes; `task license:check`, `task proto:lint`, `buf breaking --against main`, and `git diff --exit-code` on `gen/`, `ui/src/lib/gen/`, `cmd/engram/testdata/`, and `go.mod`/`go.sum` are all clean.

## Task Commits

Each task was committed atomically:

1. **Task 1: The MigrateStatus RPC, its handler, and one definition of "pending"** - `c63bd0e1` (feat)
2. **Task 2: engram migration-status — the client-tier verb** - `77ac1f09` (feat)
3. **Task 3: The advisory footer on engram search and engram list** - `ff90a790` (feat)

**Deviation fix:** `37302f91` (fix) — pinned `EngramService` RPC-count regression test, found by the plan's own whole-repo `task test` verification.

_Note: this plan carried `tdd="true"` on all three tasks. Each task's tests and implementation were authored together and verified green (including running every declared acceptance criterion) before committing — matching 07-03's and 07-05's precedent for this phase — rather than as separate `test(...)`/`feat(...)` commits._

## Files Created/Modified

- `cmd/engram/client_migration_status.go` (new) - `migrationStatusCmd`, the `engram migration-status` client-tier verb
- `cmd/engram/client_migration_status_test.go` (new) - text/json rendering, no-retry error envelope, `operatorCommands()` exclusion
- `proto/engram/v1/engram.proto` - `SchemaVersionBucket`/`MigrateStatusRequest`/`MigrateStatusResponse` and the `MigrateStatus` RPC
- `gen/go/**`, `gen/ts/**`, `ui/src/lib/gen/**` - regenerated via `task proto:gen`
- `internal/store/migrate_status.go` - `MigrateStatusResult.Pending()`
- `internal/store/migrate_status_test.go` - `TestMigrateStatusResultPending`, `TestMigrateStatusResultPendingZeroValue`
- `internal/server/tools.go` - `warnPendingMigrations` now calls `Pending()`
- `internal/server/store_iface.go` - `memStore` interface gains `MigrateStatus`
- `internal/server/fakestore_test.go` - `spyStore.migrateStatus` (scripted field) + `MigrateStatus` method
- `internal/server/connectapi.go` - `MigrateStatus` Connect handler
- `internal/server/connectapi_test.go` - `migrateStatusFailStore`, handler success/failure/empty-histogram/unauthenticated tests
- `internal/server/connectdescriptor_test.go` - pinned RPC count bumped 11 → 12 (deviation fix)
- `internal/surfaces/toolclass.go` - new `migration-status` client-tier row
- `cmd/engram/cmdwalk.go` - `operatorCommands()` doc comment extended with `migration-status`
- `cmd/engram/cmdwalk_test.go`, `cmd/engram/operator_output_test.go`, `docs-site/src/content/docs/guides/cli.md` - client-verb enumeration fixes (deviation, same defect class 07-05 fixed for `get`)
- `cmd/engram/clienttest_test.go` - `stubEngramService` extended with `MigrateStatus` support (deviation)
- `cmd/engram/client_common.go` - `footerLookupBudget`, `renderMigrationFooter`, `migrationFooterContext`, `migrationFooterCounts`
- `cmd/engram/client_common_test.go` - footer rendering, timeout-ceiling, and no-caller-context timeout tests
- `cmd/engram/client_search.go`, `cmd/engram/client_list.go` - footer wired into the text lane after `renderCoverageFooter`
- `cmd/engram/client_search_test.go`, `cmd/engram/client_list_test.go` - lookup-failure and json-lane-untouched coverage
- `cmd/engram/testdata/help.golden`, `cmd/engram/testdata/catalog.golden` - regenerated via `task surfaces:gen`

## Decisions Made

- **Handler-error coverage built on the existing `memStore` interface seam, not a live-Qdrant fixture.** `migrateStatusFailStore` embeds `memStore` and overrides only `MigrateStatus`, exactly mirroring `tools_test.go`'s `upsertFailStore` — the plan's own instruction, followed as written.
- **`spyStore.MigrateStatus` returns a SCRIPTED result** (`sp.migrateStatus`), unlike every other `spyStore` method, which derives its return value from `s.records`. This is deliberate: the histogram is not something a map-backed fake can meaningfully compute from seeded records, and the plan calls for a "scripted result" explicitly.
- **`migrationFooterCounts` takes the resolved timeout, never a pre-built context**, so `migrationFooterContext`'s `min()` ceiling is the only place that arithmetic exists — the two direct-context tests in `client_common_test.go` assert against one literal bound each, so only production's own arithmetic can satisfy both.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Two pre-existing tests and one doc table encoded the invalidated client-verb enumeration**
- **Found during:** Task 2, running `go test ./internal/surfaces/... ./cmd/engram/... -count=1` after `client_migration_status.go` and the `toolclass.go` classification landed
- **Issue:** `TestCatalogOutputFlagMatchesOperatorCommandsUnionClientVerbs` (`cmdwalk_test.go`) hardcoded `{search, list, store, get}` as the client-verb union and failed once `migration-status`'s `--output` flag appeared in `catalog.golden`. `TestTimeoutGroupMatrix`'s `reject-zero-client` group (`operator_output_test.go`) hardcoded the same four names and failed once `migration-status` carried a live `--timeout` flag not assigned to any published group. `docs-site/guides/cli.md`'s `--timeout` three-group table named the same four verbs. This is the identical defect class 07-05 fixed when `get` became the fourth client verb.
- **Fix:** Added `"migration-status"` to `TestCatalogOutputFlagMatchesOperatorCommandsUnionClientVerbs`'s `want` set, added a `"migration-status": true` entry plus a `timeoutGroupCaseArgs` case to `timeoutGroups`' `reject-zero-client` group, and updated `cli.md`'s client-verb enumeration.
- **Files modified:** `cmd/engram/cmdwalk_test.go`, `cmd/engram/operator_output_test.go`, `docs-site/src/content/docs/guides/cli.md`
- **Verification:** `go test ./internal/surfaces/... ./cmd/engram/... -count=1` green.
- **Committed in:** `77ac1f09` (Task 2)

**2. [Rule 3 - Blocking issue] Extended stubEngramService with MigrateStatus support**
- **Found during:** Task 2, writing `client_migration_status_test.go`
- **Issue:** The shared Connect test stub (`clienttest_test.go`, not in this plan's declared `files_modified`) had no `MigrateStatus` override, so the new verb could not be driven through a real Connect wire round-trip in tests — the same gap 07-05 found and fixed for `GetMemory`.
- **Fix:** Added `migrateStatusFn func(...)`, `migrateStatusCalls int`, and a `MigrateStatus` method to `stubEngramService`, mirroring the existing pattern exactly.
- **Files modified:** `cmd/engram/clienttest_test.go`
- **Verification:** `go test ./cmd/engram/... -count=1` green.
- **Committed in:** `77ac1f09` (Task 2)

**3. [Rule 3 - Blocking issue] Pinned `EngramService` RPC-count regression test updated for the new RPC**
- **Found during:** running the plan's own whole-repo `task test` verification step, after all three tasks were committed
- **Issue:** `internal/server/connectdescriptor_test.go`'s `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` pins the EXACT RPC count on the generated `FileDescriptor` (previously 11 = 5 read + 6 write) — Task 1's additive `MigrateStatus` RPC made it 12, breaking this test. The plan's own targeted `-run 'MigrateStatus|PendingMigrations'` filter for Task 1 did not match this test's name, so it was only caught by the plan-level verification's `task` (full-suite) run — the same class of gap 07-03 documented for `SearchMemoriesRequest`'s field-count pin.
- **Fix:** Bumped the pinned count to 12 (6 read + 6 write), added `MigrateStatus`'s req/resp pair to the expected map, and updated the function's doc comment.
- **Files modified:** `internal/server/connectdescriptor_test.go`
- **Verification:** `go test ./internal/server/... -count=1` green; `go test ./... ` (whole repo) green.
- **Committed in:** `37302f91` (standalone deviation-fix commit, after all three task commits)

---

**Total deviations:** 3 auto-fixed (1 bug repair on invalidated test/doc assumptions, 2 blocking-issue fixes to a shared test double and a pinned regression test).
**Impact on plan:** All three deviations were required to make the plan's own declared verification (`go test ./cmd/engram/... -count=1`, `go test ./internal/server/... -count=1`, and the plan-level `task`) actually pass. No scope creep — none touch behavior outside this plan's own blast radius, and all three mirror an identical defect class a prior plan in this phase already fixed once.

## Issues Encountered

- **`task lint:go` reports the same pre-existing `staticcheck SA1019` finding** documented by 07-01/07-03/07-05 (now at `internal/server/connectapi.go:308`, shifted by the new `MigrateStatus` handler inserted above it in the file — the finding itself is unchanged, on `ListMemories`' deprecated `Approximate` field). Confirmed out of scope: this plan's `connectapi.go` edit is confined to the new `MigrateStatus` handler, never touching `ListMemories`. Not fixed, not re-logged (already tracked in `deferred-items.md`).
- **`go test ./...` under full-repo parallel execution intermittently failed several unrelated `internal/store`/`internal/server` tests with "connection refused"** against their Qdrant testcontainers — a Docker resource-contention artifact of running every package's containers concurrently on this machine, not a real regression. Confirmed by re-running each affected package in isolation (`go test ./internal/store/... -count=1`, `go test ./internal/server/... -count=1`): both fully green standalone, and the subsequent `task test` run (after the connectdescriptor_test.go fix) passed cleanly end-to-end, including the previously-flaky packages.
- **`go vet ./cmd/engram/...` reports a pre-existing finding** in `operator_view_test.go:441` (`struct field B repeats json tag "dup"`) — a file this plan never touches. Out of scope, not fixed.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 07-07 (console banner) has its transport: the same `MigrateStatus` RPC and the same `MigrateStatusResult.Pending()` single-definition helper this plan built, so the console banner, the CLI footer, and the startup warning are guaranteed to agree about what "pending" means.
- `engram migration-status` and the advisory footer close `REQ-migration-state-visible` on the CLI: an operator with only a server URL and a token can now learn about a migration backlog either by asking directly or incidentally via `search`/`list`, without running `engram migrate` or having direct Qdrant access.
- The pre-existing `staticcheck SA1019` finding remains open, tracked in `deferred-items.md`, and is not a blocker for 07-07.

## Self-Check: PASSED

- FOUND: `cmd/engram/client_migration_status.go`
- FOUND: `cmd/engram/client_migration_status_test.go`
- FOUND: `internal/store/migrate_status.go`
- FOUND: `internal/server/connectapi.go`
- FOUND: `internal/server/store_iface.go`
- FOUND: `cmd/engram/client_common.go`
- FOUND: `internal/surfaces/toolclass.go`
- FOUND: `cmd/engram/testdata/help.golden`
- FOUND: `cmd/engram/testdata/catalog.golden`
- FOUND commit `c63bd0e1` (Task 1)
- FOUND commit `77ac1f09` (Task 2)
- FOUND commit `ff90a790` (Task 3)
- FOUND commit `37302f91` (deviation fix)
- `git diff --exit-code -- gen/ ui/src/lib/gen/` exits 0
- `git diff --exit-code -- cmd/engram/testdata/` exits 0 after `task surfaces:gen`
- `git diff --exit-code go.mod go.sum` exits 0
- `go test ./...` (whole repo) passes
- `task license:check` passes
- `task proto:lint` passes
- `buf breaking --against main` passes

---
*Phase: 07-console-cli-state-surfacing*
*Completed: 2026-08-21*
