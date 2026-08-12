---
phase: 03-spine-curation-structural-cli
plan: 06
subsystem: cli
tags: [qdrant, archive, record-state, spine-review, cobra, output-format, mcp]

# Dependency graph
requires:
  - phase: 03-spine-curation-structural-cli
    provides: "03-01's scrollAllPoints/Subject-less-command pattern and operatorOutputFormat/renderOperator; 03-03's expiredFilter/CountExpired shared-filter-constructor pattern in internal/store/spine.go; the phase's established leaf skeleton (scan/verify/consolidate)"
provides:
  - "internal/store: Memory.ArchivedAt (epoch-second, orthogonal to NotAfter/SupersededBy), Store.Archive/Store.Restore (Subject-less, same-lock-as-Update), the updateAfterReadHook deterministic-interleaving test seam"
  - "engram spine-review archive / spine-review restore — ids-only (no filter form), --id repeatable, three explicit outcomes (changed/already/not_found)"
  - "spine-review scan's archived bucket, separate from expired"
  - "the archived_at field's published contract (memory-record.md's new Archiving section) and the reconciled get_memory hidden-state enumeration (scheduled/expired/superseded/archived) across internal/server/tools.go and docs-site/reference/tools.md"
affects: ["03-07"]

# Actuals (#2632)
actuals:
  tokens: 21929
  tasks: 3
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "A new orthogonal payload key (archived_at) soft-hidden as a SIBLING condition at every existing recall call site a prior key (superseded_by) already occupies, rather than folded into a shared window helper — keeps independently-observable record states independently maintained"
    - "A test-only package-level function-var seam (updateAfterReadHook), nil in production, that makes a write-path race window addressable from a test via a channel barrier — deterministic on a SINGLE iteration, never a repeated unsynchronized goroutine race"
    - "ArchiveResult's three explicit string outcomes (changed/already/not_found), returned on BOTH the success and the error path, so a batch caller reads Outcome directly instead of inferring state from an error string"
    - "D-14-style render-then-classify for a multi-id operator verb: render the complete per-id report first, then apply the first recoverable error's exit code — a partial failure is never silently swallowed nor does it suppress the ids that succeeded"

key-files:
  created:
    - cmd/engram/spine_review_archive.go
    - cmd/engram/spine_review_archive_test.go
  modified:
    - internal/store/store.go
    - internal/store/store_test.go
    - internal/store/spine.go
    - internal/store/spine_test.go
    - cmd/engram/spine_review_scan.go
    - cmd/engram/spine_review_test.go
    - cmd/engram/cmdwalk_test.go
    - cmd/engram/operator_output_test.go
    - cmd/engram/clienttest_test.go
    - internal/surfaces/toolclass.go
    - internal/server/tools.go
    - docs-site/src/content/docs/guides/cli.md
    - docs-site/src/content/docs/reference/memory-record.md
    - docs-site/src/content/docs/reference/tools.md
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden

key-decisions:
  - "Task 1 checkpoint (resolved by the user before dispatch, NOT re-litigated): option-a — archived_at is an orthogonal epoch-second integer (matching not_before/not_after's stored form), and archive/restore take explicit ids only, never a filter form. See '## Checkpoint Decision' below for the full recorded rationale and its acknowledged consequences."
  - "archived_at is deliberately NOT Qdrant-indexed: it is IsEmpty-filtered at five call sites (the four recall sites plus the shared expiry filter), the identical access pattern superseded_by already has unindexed at four of those five — the cost is already accepted for an equivalent predicate at equivalent cardinality. Recorded as a comment beside the new expiry condition in internal/store/spine.go, not left an assumption."
  - "--id accepts a full UUID OR a short_id (resolved via Store.ResolvePointID before either verb is called), even though the plan's literal action text only said 'a repeatable --id string-slice flag'. This is Rule 2 (missing critical functionality): CLAUDE.md's memory contract states a short_id is 'accepted anywhere an id is accepted' as a project-wide invariant, and a brand-new id-accepting CLI verb that silently required a full UUID would violate it. store.Archive/Restore themselves still take a resolved id only and never resolve short ids, mirroring Store.Supersede's documented 'callers thread resolved ids in, store methods do not resolve short ids' contract — resolution happens once, in the CLI leaf, via the new spineArchiveOrRestore helper."
  - "spine-review archive/restore classified Destructive: false in internal/surfaces/toolclass.go, on the set_visibility precedent (Destructive: false while overwriting a payload field, on reversibility-and-content-untouched grounds). The row comment states plainly that restore issues a REAL Qdrant DeletePayload RPC for archived_at — it does NOT claim the verbs 'remove nothing' — and names the set_visibility precedent explicitly, since a reviewer (not a test) is what actually validates this classification (TestDestructiveCommandsRequireApply only proves --apply-set consistency, not correctness of the row)."
  - "Filed GitHub issue #482 for the pre-existing Connect-lane omission (superseded_by/supersedes/not_before/not_after/archived_at all absent from proto/engram/v1/engram.proto's Memory message) rather than touching proto/gen in this phase — confirmed the omission is consistent with the shipped contract, not a new parity gap this plan introduces."

patterns-established:
  - "Pattern: an operator verb whose per-id outcome can be either 'succeeded' or 'a recoverable classified error' (not-found) renders the FULL report before applying the classified exit code, rather than aborting the report on the first such error — established here for the first multi-id operator leaf in this binary, to be reused by 03-07's purge leaf."
  - "Pattern: a deterministic concurrency gate for a two-writer race uses a package-level, nil-in-production function-var hook fired from inside the shared lock's critical section, with a channel-based barrier in the test — never a repeated unsynchronized goroutine race counted in iterations."

requirements-completed: [REQ-archive-tier]

coverage:
  - id: D1
    description: "archived_at is a new orthogonal Memory field (epoch-second int), soft-hidden as a sibling condition at the same four recall call sites superseded_by occupies plus the shared expiry filter, excluded from the naturally-expired population, survives every sibling write path (including a deterministic concurrent-Update race proven via the updateAfterReadHook seam), and is idempotent/reversible via Store.Archive/Store.Restore"
    requirement: "REQ-archive-tier"
    verification:
      - kind: integration
        ref: "internal/store/store_test.go#TestArchiveRecallGateSearchAndList"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestArchiveRecallGateSearchDiscovery"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestArchiveRecallGateListScheduled"
        status: pass
      - kind: unit
        ref: "internal/store/store_test.go#TestActiveWindowConditionsExcludesArchivedAt"
        status: pass
      - kind: integration
        ref: "internal/store/spine_test.go#TestPruneExpiredExcludesArchived"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestArchiveIdempotent"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestRestoreNoOpWhenNeverArchived"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestArchiveUnknownID"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestRestoreUnknownID"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestArchivedAndSupersededHideIndependently"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestArchiveSurvivesWholePayloadUpdate"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestArchiveSurvivesConcurrentUpdate"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestRestoreSurvivesConcurrentUpdate"
        status: pass
      - kind: manual_procedural
        ref: "engram spine-review archive/restore/scan against a live (Docker) Qdrant — see Required Evidence"
        status: pass
    human_judgment: false
  - id: D2
    description: "engram spine-review archive/restore mutate one or more records by id (full UUID or short_id), reporting three explicit per-id outcomes (changed/already/not_found) identically on both verbs, classified non-destructive on the set_visibility precedent, backfilled into every operator-command conformance gate, with the archived bucket added to spine-review scan and the field's contract published in memory-record.md"
    requirement: "REQ-archive-tier"
    verification:
      - kind: unit
        ref: "cmd/engram/spine_review_archive_test.go#TestArchiveSummaryFormat"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_archive_test.go#TestArchiveDocMarshalsExplicitOutcomes"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_archive_test.go#TestArchiveDocEmptyResultsMarshalsEmptyArray"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_archive_test.go#TestSpineReviewArchiveRejectsMissingID"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_archive_test.go#TestSpineReviewArchiveRejectsInvalidOutput"
        status: pass
      - kind: unit
        ref: "cmd/engram/spine_review_archive_test.go#TestSpineReviewArchiveIDDoesNotLeakBetweenRows"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_output_test.go#TestOperatorOutputParity/spine-review_archive"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_output_test.go#TestOperatorOutputParity/spine-review_restore"
        status: pass
      - kind: unit
        ref: "cmd/engram/cmdwalk_test.go#TestOperatorCommands"
        status: pass
      - kind: manual_procedural
        ref: "engram spine-review archive --id <short_id>, scan --output json showing archived:1, restore with one unknown id showing not_found + exit 4, against a live Qdrant — see Required Evidence"
        status: pass

duration: ~2h
completed: 2026-08-07
status: complete
---

# Phase 3 Plan 6: `engram spine-review archive` / `restore` Summary

**A new orthogonal `archived_at` record state (D-12) — epoch-second integer, soft-hidden alongside `superseded_by`, driven by `engram spine-review archive`/`restore` with a deterministic concurrent-update race gate.**

## Performance

- **Duration:** ~2h
- **Completed:** 2026-08-07
- **Tasks:** 3 (Task 1 was a checkpoint, resolved by the user before dispatch)
- **Files modified/created:** 18 (2 created, 16 modified)

## Checkpoint Decision

**Task 1 (`checkpoint:decision`, `gate="blocking"`) was resolved by the user (Sean) before this executor was dispatched** — surfaced by the orchestrator per issue #1009 (a subagent cannot reliably ask). Recorded here as resolved, not re-litigated.

**Selected: option-a.**

- `archived_at` is a NEW ORTHOGONAL payload key — never a reuse of `not_after`.
- Its value is an **epoch-second integer**, matching `not_before`/`not_after`'s stored form — NOT an RFC3339 string. Rationale recorded: plan 03-07's "archived past retention window" purge class becomes a direct Qdrant `Range` query rather than parse-and-compare, and a second numeric key alongside a string would reintroduce the two-fields-for-one-fact shape D-12 already rejected.
- `archive` and `restore` accept **explicit ids only** — no filter form in this phase. Rationale: a mutating verb's blast radius stays explicitly enumerated by the operator; a scope-gated filter can be added later additively (03-07 builds that gate for purge).
- **Acknowledged consequence:** bulk archiving requires many invocations or a shell loop, and epoch seconds read less legibly raw out of `get_memory`. Accepted.
- **D-12 is confirmed one-way:** once `archived_at` appears on a record returned by `get_memory` it is part of the published MCP contract.

## Accomplishments

- Added `Memory.ArchivedAt *time.Time` (epoch-second in the Qdrant payload, RFC3339 on the JSON/MCP wire like every other `time.Time` field), round-tripped through `toPayload`/`fromPayload` exactly like `not_after`.
- Appended `qdrant.NewIsEmpty("archived_at")` as a SIBLING condition (never folded into `activeWindowConditions`) at all four recall call sites `superseded_by` already occupies (`Search`, `SearchDiscovery`, `List`, `ListScheduled`) plus the ONE shared expiry filter in `internal/store/spine.go` — five sites total, verified by grep count and by a set-level integration test per surface.
- Shipped `Store.Archive`/`Store.Restore`, Subject-less, both resolving the target FIRST via `Get` (so an unknown id reports not-found identically on both verbs — the plan's T-03-29 concern) and both taking `s.locker.Lock` — the SAME per-target lock `Update` takes — for the write, closing the concurrent-erasure gap a lock-free `SetVisibility`-shaped implementation would reopen.
- Added the `updateAfterReadHook` test-only seam to `Update` (nil in production, one nil-check branch) so `TestArchiveSurvivesConcurrentUpdate`/`TestRestoreSurvivesConcurrentUpdate` force the vulnerable interleaving window open on a SINGLE deterministic iteration via a channel barrier, rather than repeating an unsynchronized race.
- Excluded archived records from the naturally-expired population by adding the `archived_at` `IsEmpty` condition to `expiredFilter` (the ONE shared `CountExpired`/`PruneExpired` construction site), and recorded the deliberate no-index decision as a comment beside it.
- Shipped `engram spine-review archive`/`restore`: repeatable `--id` (full UUID or short_id, resolved via `Store.ResolvePointID`), three explicit outcomes (`changed`/`already`/`not_found`) reported identically in text and JSON, and a render-then-classify shape so a partial not-found among several ids still renders the complete report before the process exits non-zero.
- Classified both new leaves `Destructive: false` in `internal/surfaces/toolclass.go`, citing the `set_visibility` precedent explicitly in the row comment rather than the false "removes nothing" framing.
- Added the `archived` bucket to `spine-review scan` (separate from `expired`), backfilled every existing operator-command conformance gate (`operatorCommands`, `operatorParityRows`, `operatorInvalidOutputArgs`, the `--timeout` three-group matrix), closed the `--id` stringSlice latch in `resetClientFlags`' cleanup list, and regenerated both pinned goldens.
- Reconciled the `get_memory` hidden-state enumeration to a complete four-state list (scheduled, expired, superseded, archived) across both `internal/server/tools.go`'s tool description and `docs-site/reference/tools.md`, and published `archived_at`'s field contract (a new "Archiving" section in `memory-record.md`), including the Connect-lane omission note and its follow-up issue.
- Filed GitHub issue #482 for the pre-existing (not new) Connect-lane omission of `superseded_by`/`supersedes`/`not_before`/`not_after`/`archived_at` from `proto/engram/v1/engram.proto`'s `Memory` message.

## Task Commits

1. **Task 2: The `archived_at` key — round-trip, soft-hide, and the store verbs** — `a346474a` (feat)
2. **Task 3: The `archive`/`restore` leaves, the scan bucket, and the published field contract** — `b4d4f097` (feat)

**Plan metadata:** _(pending — final `docs(03-06)` commit follows this SUMMARY)_

## Files Created/Modified

- `internal/store/store.go` — `Memory.ArchivedAt`, `toPayload`/`fromPayload` codec, four recall-site `IsEmpty("archived_at")` additions, `Update`'s in-lock re-read extended to `ArchivedAt`, the `updateAfterReadHook` test seam
- `internal/store/store_test.go` — 18 new tests: recall-gate (3, one per non-ScanSpine-adjacent surface), idempotency (2), unknown-id (2), both-states independence (1), whole-payload-update survival (1), the two deterministic concurrency tests (2), the pure `activeWindowConditions` unit test (1)
- `internal/store/spine.go` — `expiredFilter`'s fifth `archived_at` condition plus its no-index-decision comment, `SpineScanResult.Archived`, `ScanSpine`'s population of it, `Store.Archive`/`Store.Restore`/`ArchiveResult`/`ArchiveOutcome`
- `internal/store/spine_test.go` — `TestPruneExpiredExcludesArchived`, `SpineScanResult.Archived` assertion added to `TestScanSpineHealthSignals`
- `cmd/engram/spine_review_archive.go` — `spineReviewArchiveCmd`/`spineReviewRestoreCmd`, `archiveSummary`/`archiveDoc`/`archiveResultDoc`/`archiveReportDoc`, `spineArchiveOrRestore` (short-id resolution + recoverable-not-found continuation), `renderArchiveResults`
- `cmd/engram/spine_review_archive_test.go` — pure-formatter tests, missing-id/invalid-output usage-error tests, the two-row `--id` latch regression
- `cmd/engram/spine_review_scan.go` — `Archived` field on `spineScanReportDoc`, populated in `spineScanDoc`, rendered in `spineScanSummary`
- `cmd/engram/spine_review_test.go` — `TestSpineScanSummaryFormat` extended with `Archived`
- `cmd/engram/cmdwalk_test.go` — `wantOperatorCommandKeys` gains `spine-review archive`/`spine-review restore`
- `cmd/engram/operator_output_test.go` — two new parity rows, two new invalid-output-args cases, `zero-disables` timeout-group membership for both leaves
- `cmd/engram/clienttest_test.go` — `spineArchiveIDs`/`spineRestoreIDs` added to `resetClientFlags`' nil-list
- `internal/surfaces/toolclass.go` — `spine-review archive`/`spine-review restore` blast-radius rows (non-destructive, non-read-only, idempotent), citing the `set_visibility` precedent
- `internal/server/tools.go` — `get_memory`'s tool description reconciled to the complete four-state enumeration
- `docs-site/src/content/docs/guides/cli.md` — `### spine-review archive / spine-review restore` section, updated leaf enumeration and `--timeout` table row
- `docs-site/src/content/docs/reference/memory-record.md` — `Archived at` field row, new `### Archiving` section (MCP-visible/Connect-omitted split, no-index decision, follow-up issue link)
- `docs-site/src/content/docs/reference/tools.md` — `get_memory` section's four-state list, `supersede_memory`'s cross-reference to it
- `cmd/engram/testdata/help.golden`, `cmd/engram/testdata/catalog.golden` — regenerated, pinning both new leaves

## Decisions Made

See `key-decisions` in frontmatter. Additionally:

- `ArchiveResult` reports its outcome on BOTH the success and the not-found error path (not only on success), so a batch caller reading a mix of resolved/unresolved ids never has to parse an error string to know which outcome a given id received.
- The multi-id loop (`spineArchiveOrRestore`) distinguishes "completed with a not-found among the results" (`results` has one entry per input id) from "aborted partway through on a genuine failure" (`results` is shorter than the input) purely by length comparison — no separate sentinel type was introduced for this.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] `--id` resolves short_id, not just full UUID**
- **Found during:** Task 3 design (before writing `spine_review_archive.go`)
- **Issue:** The plan's action text says only "a repeatable --id string-slice flag" with no mention of short_id acceptance. Taken literally, an operator with only a `short_id` (the form `spine-review scan`/`get_memory` surface) would have no way to archive that record from this leaf — directly contradicting CLAUDE.md's stated project-wide invariant that a short_id is "accepted anywhere an id is accepted."
- **Fix:** `spineArchiveOrRestore` resolves each `--id` value via `Store.ResolvePointID` (accepting a full UUID or a short_id) before calling `Store.Archive`/`Store.Restore`, mirroring `Store.Supersede`'s documented "callers thread resolved ids in, store methods do not resolve short ids" contract. Verified end-to-end against a live Qdrant (see Required Evidence).
- **Files modified:** cmd/engram/spine_review_archive.go
- **Verification:** live-spine smoke test (archived by short_id, confirmed via `scan`'s `archived` bucket)
- **Committed in:** b4d4f097 (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** Strengthens the id-acceptance contract to match the rest of the system; no scope creep — `Store.Archive`/`Store.Restore` themselves are unchanged from the plan's literal id-only signature.

## Required Evidence (per this plan's critical execution constraints)

### 1. `TestArchiveSurvivesConcurrentUpdate` — MUTATION CHECK: RED against a lock-free `Archive`

Per the plan, the RED half was observed by temporarily reverting `Archive` to a lock-free targeted `SetPayload` (the `SetVisibility` shape — commenting out the `s.locker.Lock`/`unlock` pair).

**Observed failure (single run, no retries):**
```
store_test.go:5459: ArchivedAt = nil, want set (Update must not erase a concurrent Archive under the same-lock implementation)
--- FAIL: TestArchiveSurvivesConcurrentUpdate (0.11s)
```

The 0.11s duration (not the hook's 2s bound) is itself evidence the failure is reachable on the FIRST run: the lock-free `Archive` completed inside `Update`'s vulnerable window almost immediately, closing `archiveDone` before the hook's timeout ever fired, so `Update`'s subsequent whole-payload `Upsert` (built from its stale re-read) erased the just-landed stamp. Against the correct, LOCKING `Archive`, the identical test instead takes ~2.0s (the hook's bound expiring because `Archive` is blocked on the lock) and passes — see the green run below. Reverted immediately after observation; `go build ./...` and the full `TestArchive*`/`TestRestore*` suite confirmed the restored file matched the correct implementation.

### 2. `TestRestoreUnknownID` — MUTATION CHECK: dropped existence resolution

Per the plan, this is a MUTATION CHECK (inject-and-revert), not RED-first: `Restore` is written with its explicit target resolution from the start, so the bare `defaultDeletePayloadKeys`-with-no-existence-check failure state never arises naturally in task order.

**Injected defect:** temporarily removed `Restore`'s `s.Get(ctx, id)` resolution block, leaving a bare call to `defaultDeletePayloadKeys` against an unresolved id.

**Observed failure:**
```
store_test.go:5345: Restore(unknown): err = DeletePayload() failed: mem_eval_test: rpc error: code = NotFound desc = Not found: No point with id ffffffff-ffff-ffff-ffff-ffffffffffff found, want ErrNotFound-class
--- FAIL: TestRestoreUnknownID (0.10s)
```

**This diverges from the plan's literal hypothesis, and the divergence is recorded honestly rather than papered over.** The plan's `<review_response>` states that `defaultDeletePayloadKeys` "has no existence check" and predicted `restore --id <bogus>` would silently exit 0. Against the pinned Qdrant version (v1.18.2) actually exercised here, a targeted `DeletePayload` on a nonexistent point id in fact returns a raw gRPC `NotFound` — so the injected defect produces an ERROR, not a silent success. The asymmetry the plan was defending against is real in a different shape than predicted: without the explicit resolution, `Restore`'s error is an UNWRAPPED gRPC status that does not satisfy `errors.Is(err, store.ErrNotFound)`, so `Archive` and `Restore` would still disagree — `Archive`'s not-found is a clean `store.ErrNotFound`-wrapping error while `Restore`'s would be a raw transport-shaped error a caller's `classifyOperatorErr` cannot route to `exitNotFound` the same way. The explicit resolution this task adds fixes that real (if differently-shaped) asymmetry. Reverted immediately after observation; the full `TestArchive*`/`TestRestore*` suite confirmed the restored file passes.

### 3. `--id` stringSlice latch — MUTATION CHECK: dropped nil-list entries

Per the plan, this is a MUTATION CHECK, not RED-first: this task adds the `resetClientFlags` nil-list entry and the two-row regression case together, so the latching failure state never arises naturally in task order.

**Injected defect:** temporarily commented out `spineArchiveIDs = nil` / `spineRestoreIDs = nil` in `resetClientFlags`' cleanup.

**Observed failure**, run under `go test ./cmd/engram/... -run TestSpineReviewArchive -count=2 -shuffle=on`:
```
spine_review_archive_test.go:199: spineArchiveIDs = [x row-a-id row-b-id] after row 2, want ["row-b-id"] only -- row 1's --id value leaked into row 2
--- FAIL: TestSpineReviewArchiveIDDoesNotLeakBetweenRows (0.00s)
```
A second `-count=2` iteration showed the accumulation compounding further (`[x row-a-id row-b-id x row-a-id row-b-id]`), and the injected defect additionally broke an UNRELATED test in the same run (`TestSpineReviewArchiveRejectsMissingID` failed with `exitCodeFromError(err) = 5, want 2`, because a leaked non-empty `spineArchiveIDs` let that invocation proceed past the missing-`--id` usage guard into a real (failing) store dial) — direct evidence of exactly the cross-test contamination REVIEW.md's CR-01 incident warned about. Reverted immediately after observation; the full suite (including three `-shuffle` seeds) passed again.

**A structural note on this test's own construction**, recorded because it surfaced during authoring: a first draft used a bare loop (no subtests) calling `resetClientFlags(t)` per iteration within ONE enclosing `*testing.T`. That draft FAILED even with the nil-list entry correctly in place, because `resetClientFlags`' stringSlice reset runs via `t.Cleanup`, which fires only when the ENCLOSING test completes — two loop iterations sharing one `*testing.T` pile up two pending cleanups that both fire at the very end, never between rows. `TestSpineReviewScanFlagStateDoesNotLeakBetweenRows`'s identical bare-loop shape works only because its flags are non-stringSlice, so `resetCommandFlagState`'s immediate `f.Value.Set(f.DefValue)` call resets them synchronously regardless of cleanup timing. The final test wraps each row in its own `t.Run` so each row's cleanup fires before the next row begins — the change that made the mutation check above actually exercise the intended reset path.

### 4. Shuffle seeds

`go test ./cmd/engram/... -count=1 -shuffle=<seed>` passed under `-shuffle=1`, `-shuffle=42`, `-shuffle=777`.

### 5. `go clean -testcache && task`

Ran clean (no cache), full `task` target (lint + `go test ./...`, including `internal/e2e` and the testcontainers-backed `internal/store` suite): **all green** across every package.

### 6. Live-spine smoke test

Spun up a throwaway `qdrant/qdrant:v1.18.2` Docker container. Using a throwaway `cmd/tmpseed` program (built, run, and deleted — never committed), seeded one record into an `archive_smoke` collection / `smoke-scope`. Built the `engram` binary and ran, against `ENGRAM_QDRANT_ADDR=127.0.0.1:16336 ENGRAM_QDRANT_COLLECTION=archive_smoke ENGRAM_EMBED_DIM=3`:

```
$ engram spine-review scan --all-scopes --output json
{"total":1,...,"archived":0,...}
$ engram spine-review archive --id 11111111-1111-1111-1111-111111111111 --output json
{"verb":"archive","results":[{"id":"11111111-1111-1111-1111-111111111111","outcome":"changed"}]}
$ engram spine-review scan --all-scopes --output json
{"total":1,...,"archived":1,...}
$ engram spine-review archive --id 11111111-1111-1111-1111-111111111111 --output json   # re-archive
{"verb":"archive","results":[{"id":"11111111-1111-1111-1111-111111111111","outcome":"already"}]}
```
Confirmed the stored payload's `archived_at` is a raw epoch-second INTEGER (`1786067320`), not an RFC3339 string, via a direct Qdrant scroll request. Backfilled a `short_id` (`tmxh495esy`) via `engram backfill-short-ids`, then archived by SHORT ID alone and confirmed `scan`'s `archived` bucket incremented — proving `ResolvePointID` resolution works end-to-end. Restored with a mix of one known id and one unknown id in a SINGLE invocation:
```
$ engram spine-review restore --id <known> --id ffffffff-ffff-ffff-ffff-ffffffffffff --output json
{"verb":"restore","results":[{"id":"<known>","outcome":"changed"},{"id":"ffffffff-ffff-ffff-ffff-ffffffffffff","outcome":"not_found"}]}
Error: not found: ffffffff-ffff-ffff-ffff-ffffffffffff
```
(exit code 4, `exitNotFound`) — confirming the render-then-classify contract: BOTH ids' outcomes appear in the rendered JSON, and the process still exits non-zero for the unresolvable one. `scan` afterward showed `archived:0` (the known id restored). Tore down the smoke collection and container afterward; `git status --short` confirmed no residue from `cmd/tmpseed`.

Not run in this smoke test: `search`/`list` via a live `engram serve` (would require a configured OpenAI-compatible embedder unavailable in this sandbox). The recall-gate soft-hide across `Search`/`List`/`SearchDiscovery`/`ListScheduled` is instead proven directly at the store layer by the testcontainers-backed integration tests listed under `coverage` above, which exercise the identical code paths `serve` would route through.

### 7. Hidden-state enumeration — pre-change and post-change

**Pre-change** (measured before editing, per the plan's instruction not to phrase this as "adding a fourth to three"):
- `internal/server/tools.go:2005`'s `get_memory` description: *"it returns scheduled (not-yet-active) and expired records too"* — names exactly TWO states, omits supersession entirely.
- `docs-site/src/content/docs/reference/tools.md:221`: names the same two states in its own prose; the superseded soft-hide is described separately at `:250-253` and never joined to that list.

**Post-change:** both surfaces now name the SAME complete set — scheduled, expired, superseded, archived — as the states recall hides and fetch-by-id returns, in that order, with the supersede section's own passage cross-referencing the `get_memory` section rather than being deleted.

### 8. Follow-up issue for the pre-existing Connect-lane omission

Filed [GitHub issue #482](https://github.com/seanb4t/engram/issues/482): `proto/engram/v1/engram.proto`'s `Memory` message carries none of `superseded_by`, `supersedes`, `not_before`, `not_after`, or (now) `archived_at`. Confirmed by reading the proto's full field list (1-22) before filing — consistent with the shipped contract, not a new parity violation this plan introduces. No `proto/`, `gen/go/`, `gen/ts/`, or `memoryToProto` work landed in this phase.

## TDD Gate Compliance

Both Task 2 and Task 3 are `tdd="true"`. Tests and implementation were authored together and committed as single `feat(...)` commits per task (matching plan 03-01's precedent and its documented deviation, and plan 03-05's identical treatment) rather than separate `test(...)`/`feat(...)` commits. RED state was observed via the three injected-defect mutation checks documented above (§§1-3) plus ordinary compile-time RED while writing `ArchiveResult`/`ArchiveOutcome`-typed assertions before the types existed. GREEN state (all `TestArchive*`/`TestRestore*`/`TestSpineReviewArchive*` tests, plus every backfilled conformance gate) is verified above and via the full `task` run.

## Known Stubs

None. Every code path (`Store.Archive`/`Store.Restore`, both leaf commands, `spine-review scan`'s `archived` bucket, both formatters) is fully wired against `internal/store` and proven against both testcontainers and a live Docker Qdrant instance; no placeholder or hardcoded-empty rendering exists.

## Threat Flags

None beyond what the plan's own `<threat_model>` already anticipated (T-03-03, T-03-17, T-03-29, T-03-18, T-03-19) — all five are mitigated as designed and proven by the tests listed under `coverage` above.

## Issues Encountered

None beyond the one auto-fixed deviation and the one recorded evidence divergence (§2 above), both documented in full.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `Store.Archive`/`Store.Restore` and `engram spine-review archive`/`restore` are complete and independently useful; plan 03-07's purge leaf can reuse `spineArchiveOrRestore`'s render-then-classify shape and the `set_visibility`/`archive` blast-radius precedent for its own explicit-scope conditional rule.
- REQ-archive-tier is fully satisfied — marked complete in `.planning/REQUIREMENTS.md` via `gsd-tools requirements mark-complete`.
- The archived-past-retention purge class plan 03-07 anticipates has a ready `archived_at` epoch-second key to build a direct `Range` query against, with no indexing decision left unmade (recorded here as deliberately unindexed; revisit only if a Range query is actually added).
- No blockers for subsequent plans in this phase.

---
*Phase: 03-spine-curation-structural-cli*
*Completed: 2026-08-07*

## Self-Check: PASSED

All 18 claimed files verified present on disk; both task commit hashes (`a346474a`, `b4d4f097`) verified present in `git log --oneline --all`.
