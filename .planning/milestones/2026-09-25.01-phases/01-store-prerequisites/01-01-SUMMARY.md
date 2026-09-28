---
phase: 01-store-prerequisites
plan: 01
subsystem: auth
tags: [cedar, authz, qdrant, tdd, store]

# Dependency graph
requires: []
provides:
  - "authz.ActionArchive — the sixth Cedar authorization verb, gating both archive and restore"
  - "Store.ArchiveAs(ctx, id, subj) — owner-gated archive: lock, getWritable(ActionArchive), shared archiveResolved core"
  - "Store.RestoreAs(ctx, id, subj) — owner-gated restore: lock, getWritable(ActionArchive), shared restoreResolved core"
  - "archiveResolved / restoreResolved — extracted shared cores reused by the subject-less Archive/Restore and the new gated wrappers"
  - "internal/store/archive_authz_test.go — the phase's first test (TestArchiveAsOwnerGate) plus five gated-path edge-case tests"
  - "widened Cedar policy corpus (allActions, TestPolicyCorpus_SharedReadOnly) and reference schema.json covering the archive action"
affects:
  - "Phase 3 — ArchiveMemory/RestoreMemory RPCs and MCP tools will call ArchiveAs/RestoreAs directly with nothing left to authorize (DEC-cgb)"
  - "Phase 1 plan 01-02 — RelatedMemories/ListTags share this plan's internal/store package and getWritable/authz conventions"

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
actuals:
  tokens: 7608
  tasks: 2
  commits: 5

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Gated write wrapper pattern: <Verb>As(ctx, id, subj) locks first, runs getWritable(ctx, id, subj, action) inside the lock, then calls a shared unexported core (archiveResolved/restoreResolved) that the subject-less operator-tier verb also calls — one Qdrant Get per gated call, no duplicated mutation logic, no second authz path."
    - "Not-found indistinguishability (DEC-xa6): getWritable's denial and Get's not-found both collapse to the identical ErrNotFound wrapping text, so a caller cannot distinguish 'exists but not owned' from 'does not exist'."

key-files:
  created:
    - internal/store/archive_authz_test.go
  modified:
    - internal/authz/authz.go
    - internal/authz/schema.json
    - internal/authz/policy_corpus_test.go
    - internal/store/spine.go
    - internal/store/schemaversion_stamp_gate_test.go

key-decisions:
  - "D-01: one Cedar action, ActionArchive, gates both ArchiveAs and RestoreAs — no reuse of ActionWrite, no separate restore action, no .cedar policy file changes (own_records already permits every action for the owner; shared_read permits only read)."
  - "D-02: ArchiveAs/RestoreAs take s.locker.Lock first, then getWritable(..., ActionArchive) inside the lock, then the shared core — matching the CR-04 lock discipline Archive/Restore/Update already use. No nil-means-operator sentinel: a nil Subject fails closed via principalParams."
  - "D-03: the gated path does NOT special-case category=='rule' — the owner may archive/restore a rule; rule-immutability rejections for other verbs live in internal/server, not here."
  - "D-04/DEC-xa6: a readable-but-not-owned (shared) record and a nonexistent id return the identical ErrNotFound + ArchiveOutcomeNotFound."
  - "D-05: TestArchiveAsOwnerGate is the phase's first test, executed as a tracer task (RED committed before ArchiveAs existed, then GREEN)."

requirements-completed: [STORE-01]

coverage:
  - id: D1
    description: "authz.ActionArchive is a distinct sixth action gating both archive and restore directions (D-01)"
    requirement: STORE-01
    verification:
      - kind: unit
        ref: "internal/authz/policy_corpus_test.go#TestPolicyCorpus_SharedReadOnly"
        status: pass
      - kind: unit
        ref: "internal/authz/policy_corpus_test.go#TestPolicyCorpus_OwnRecordAllow"
        status: pass
      - kind: unit
        ref: "internal/authz/policy_corpus_test.go#TestPolicyCorpus_EmptyOwnerDenyAll"
        status: pass
    human_judgment: false
  - id: D2
    description: "Store.ArchiveAs enforces the owner-only gate; the phase's tracer/first test (SC1, D-05)"
    requirement: STORE-01
    verification:
      - kind: unit
        ref: "internal/store/archive_authz_test.go#TestArchiveAsOwnerGate"
        status: pass
    human_judgment: false
  - id: D3
    description: "Store.RestoreAs mirrors ArchiveAs on the same action (D-02); archive/restore round trip preserves content and tags"
    requirement: STORE-01
    verification:
      - kind: unit
        ref: "internal/store/archive_authz_test.go#TestRestoreAsOwnerGate"
        status: pass
    human_judgment: false
  - id: D4
    description: "The owner can archive/restore a rule-category record; a non-owner cannot (D-03)"
    requirement: STORE-01
    verification:
      - kind: unit
        ref: "internal/store/archive_authz_test.go#TestArchiveAsRestoreAsOwnedRule"
        status: pass
    human_judgment: false
  - id: D5
    description: "Nil Subject fails closed and mutates nothing; a nonexistent id and a readable-but-not-owned id are indistinguishable (D-02, D-04)"
    requirement: STORE-01
    verification:
      - kind: unit
        ref: "internal/store/archive_authz_test.go#TestArchiveAsFailsClosed"
        status: pass
    human_judgment: false
  - id: D6
    description: "Anonymous() can archive/restore an ownerless record (own_records matches the anonymous bucket)"
    requirement: STORE-01
    verification:
      - kind: unit
        ref: "internal/store/archive_authz_test.go#TestArchiveAsAnonymousBucket"
        status: pass
    human_judgment: false
  - id: D7
    description: "Repeat ArchiveAs/RestoreAs on an already-settled record is a safe no-op (ArchiveOutcomeAlready, nil error)"
    requirement: STORE-01
    verification:
      - kind: unit
        ref: "internal/store/archive_authz_test.go#TestArchiveAsIdempotent"
        status: pass
    human_judgment: false
  - id: D8
    description: "SC2: the subject-less Archive/Restore and the CLI's spine-review archive/restore path are byte-identical and unregressed"
    requirement: STORE-01
    verification:
      - kind: other
        ref: "git diff --quiet 16f44eda HEAD -- cmd/engram/spine_review_archive.go internal/authz/policies"
        status: pass
      - kind: unit
        ref: "internal/store package: TestArchiveIdempotent, TestRestoreNoOpWhenNeverArchived, TestArchiveUnknownID, TestRestoreUnknownID, TestArchiveSurvivesWholePayloadUpdate, TestArchiveSurvivesConcurrentUpdate, TestRestoreSurvivesConcurrentUpdate, TestArchiveRecallGateSearchAndList, TestArchiveRecallGateIncludeArchived, TestArchiveRecallGateSearchDiscovery, TestArchiveRecallGateListScheduled (11 pre-existing tests)"
        status: pass
      - kind: unit
        ref: "cmd/engram package: TestArchiveReportCorrelatesRequestedToken and 9 sibling archive/report tests"
        status: pass
    human_judgment: false
  - id: D9
    description: "getWritable and the shared core run under one s.locker.Lock acquisition — the lock precedes the gate (CR-04)"
    requirement: STORE-01
    verification:
      - kind: other
        ref: "acceptance criteria: awk-scoped structural check that ArchiveAs/RestoreAs bodies call s.locker.Lock before s.getWritable"
        status: pass
    human_judgment: true
    rationale: "The lock-before-gate ORDER is proven structurally (source-scoped awk check, run every commit); the CONCURRENCY claim itself (a concurrent gated archive and restore on one id serialize into exactly one final state) is flagged in the plan's must_haves as verification: backstop — no dedicated interleaving test was authored this plan, unlike TestArchiveSurvivesConcurrentUpdate's barrier-controlled proof for the subject-less path. Flagging for the phase verifier rather than silently claiming full proof."
duration: 51min
completed: 2026-09-25
status: complete
---

# Phase 1 Plan 1: Gated Archive/Restore Authz Summary

**Owner-gated `Store.ArchiveAs`/`RestoreAs` sharing one Cedar action and one archived_at core with the existing subject-less `Archive`/`Restore`, proven by a tracer-first test before any other Phase 1 code existed.**

## Performance

- **Duration:** 51 min
- **Started:** 2026-09-25T21:45:00Z (approximate)
- **Completed:** 2026-09-26T02:36:22Z
- **Tasks:** 2
- **Files modified:** 6 (1 created, 5 modified)

## Accomplishments

- `authz.ActionArchive` — the sixth engram authorization verb, added to the const block and the reference `schema.json`, requiring zero `.cedar` policy edits because `own_records.cedar` already permits every action for the owner and `shared_read.cedar` permits only `read`.
- `Store.ArchiveAs`/`Store.RestoreAs` — owner-gated wrappers that lock the target, run `getWritable(ctx, id, subj, authz.ActionArchive)` inside that lock, then hand the resolved `Memory` to a shared core (`archiveResolved`/`restoreResolved`) extracted from the pre-existing subject-less `Archive`/`Restore`. One Qdrant `Get` per gated call, no duplicated mutation logic.
- `internal/store/archive_authz_test.go` — `TestArchiveAsOwnerGate` landed RED as the phase's literal first test (tracer task, D-05) before `ArchiveAs` existed, then went GREEN. Five more tests pin `RestoreAs`, the rule-category case (D-03), fail-closed nil-Subject and not-found-indistinguishability behavior (D-02/D-04), the anonymous bucket, and idempotency.
- Widened `internal/authz/policy_corpus_test.go`'s `allActions` and `TestPolicyCorpus_SharedReadOnly`'s deny list so the Cedar policy layer itself denies `archive` on a shared, non-owned record — not just the store's `getWritable` gate.
- `Store.Archive`/`Store.Restore` (the CLI's `spine-review archive`/`restore` path) are untouched in signature and behavior; `cmd/engram/spine_review_archive.go` is byte-identical to commit `16f44eda` (SC2), and all eleven pre-existing subject-less Archive/Restore store tests plus the cmd/engram archive tests pass unmodified.

## Task Commits

Each task was committed atomically (RED then GREEN, per TDD):

1. **Task 1: Gated archive end to end — tracer** (RED) - `a314b3a1` (test) — `TestArchiveAsOwnerGate` added, fails to compile (`ArchiveAs` undefined)
2. **Task 1: Gated archive end to end — tracer** (GREEN) - `c469f152` (feat) — `authz.ActionArchive`, `archiveResolved`, `Store.ArchiveAs`
3. **Task 2: RestoreAs and edge cases** (RED) - `dfdf7479` (test) — five more gated-path tests added, fail to compile (`RestoreAs` undefined)
4. **Task 2: RestoreAs and edge cases** (GREEN) - `4b37bc70` (feat) — `restoreResolved`, `Store.RestoreAs`, widened policy corpus, `schema.json` entry
5. **Deviation fix** - `74b9816e` (fix) — retargeted a pre-existing gate test's classification entry (see Deviations below)

**Plan metadata:** (this commit, pending) — SUMMARY + STATE/ROADMAP

_TDD tasks produced RED→GREEN commit pairs; no REFACTOR commit was needed for either task._

## Files Created/Modified

- `internal/store/archive_authz_test.go` - `TestArchiveAsOwnerGate` (tracer/first test) plus `TestRestoreAsOwnerGate`, `TestArchiveAsRestoreAsOwnedRule`, `TestArchiveAsFailsClosed`, `TestArchiveAsAnonymousBucket`, `TestArchiveAsIdempotent`
- `internal/authz/authz.go` - added `ActionArchive Action = "archive"`, updated the const block and `Action` type doc comments
- `internal/store/spine.go` - extracted `archiveResolved`/`restoreResolved`; added `Store.ArchiveAs`/`Store.RestoreAs`; imported `internal/authz`
- `internal/authz/policy_corpus_test.go` - widened `allActions` and `TestPolicyCorpus_SharedReadOnly`'s deny list with `ActionArchive`
- `internal/authz/schema.json` - added a reference-only `"archive"` action entry
- `internal/store/schemaversion_stamp_gate_test.go` - retargeted the `partialWriteClassification` entry from `Store.Archive` to `Store.archiveResolved` (deviation, see below)

## Decisions Made

- D-01..D-05 implemented exactly as CONTEXT.md locked them (see frontmatter `key-decisions`). No `.cedar` policy file changed.
- The `archiveResolved`/`restoreResolved` extraction mirrors `FetchForUpdate`'s existing doc-commented lock contract: the caller MUST hold `s.locker.Lock(ctx, id)` and MUST pass the `Memory` as read under that lock, so the already-settled check and the write sit in one lock window (CR-04) with no second `Get`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Retargeted a stale write-classification gate entry broken by the archiveResolved extraction**
- **Found during:** Task 2's full-suite verification (`go test ./internal/store/ ./internal/authz/ ./cmd/engram/`)
- **Issue:** `internal/store/schemaversion_stamp_gate_test.go`'s `TestPartialWritePathsAreClassifiedNonStamping` is an AST-derived set-equality gate over every `SetPayload`/`DeletePayload`/`OverwritePayload` call site in `internal/store`, keyed by enclosing function name. Task 1's extraction of `Archive`'s `SetPayload` call into the new shared `archiveResolved` core moved that call site out of `Store.Archive` and into `Store.archiveResolved` — the classification entry still named `Store.Archive`, so the gate failed with both a stale entry and an unclassified new site.
- **Fix:** Renamed the classification entry to `Store.archiveResolved` and updated its justification to note both `Archive` and `ArchiveAs` now call it under their own lock. `Restore`/`restoreResolved` needed no change — their `DeletePayload` call still goes through the pre-existing `deletePayloadKeys` seam (`Store.defaultDeletePayloadKeys`), not a direct `s.client` call inside the enclosing function.
- **Files modified:** `internal/store/schemaversion_stamp_gate_test.go`
- **Verification:** `go test ./internal/store/ -run '^TestPartialWritePathsAreClassifiedNonStamping$'` passes; full `go test ./internal/store/ ./internal/authz/ ./cmd/engram/ -count=1` passes (292.9s).
- **Committed in:** `74b9816e`

---

**Total deviations:** 1 auto-fixed (1 bug — Rule 1).
**Impact on plan:** Necessary to keep an existing cross-cutting correctness gate green after the planned `archiveResolved` extraction; no scope creep, no behavior change to any write path.

## Issues Encountered

None beyond the deviation above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `ArchiveAs`/`RestoreAs` are ready for Phase 3's `ArchiveMemory`/`RestoreMemory` RPCs to call directly with nothing left to authorize (DEC-cgb).
- Plan 01-02 (`RelatedMemories`, `ListTags`) can proceed independently in the same package; no shared file conflicts with this plan's `files_modified`.
- `go test ./internal/keylinks/` is expected to now pass (previously failed pre-SUMMARY with "scanned 0 plan files" — this SUMMARY.md's existence is what the gate scans for).

---

*Phase: 01-store-prerequisites*
*Completed: 2026-09-25*
