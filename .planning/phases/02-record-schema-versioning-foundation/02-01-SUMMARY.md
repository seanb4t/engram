---
phase: 02-record-schema-versioning-foundation
plan: 01
subsystem: database
tags: [qdrant, go, schema-versioning, migration-foundation]

requires:
  - phase: 01-gate-ci-integrity
    provides: key-link escaping normalization and the shared-Qdrant CI gate this plan's integration tests run against
provides:
  - internal/migrate leaf package (Version type, CurrentVersion=0 constant)
  - store.Memory.SchemaVersion field, stamped monotonically at the payload() seam and decoded absent-safely in fromPayload()
  - Store.Update's in-lock refresh extended to SchemaVersion
  - schema_version Qdrant payload index
  - corrected Reindex doc-comment contract (verbatim payload copy, never advances version)
  - guides/upgrade.md operator note on the stamp and the rollback hazard
affects: [02-02-plan-tests, 02-03-plan-tests, 02-04-plan-tests, phase-03-migration-foundation, phase-04-migration-cli, phase-05-connect-record-state-parity]

actuals:
  tokens: 6975
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Leaf package for a shared version type (internal/migrate mirrors internal/openaiurl: stdlib-only, zero imports, one-way dependency)"
    - "Monotonic stamp at a single payload() seam: max(currentConstant, decodedValue), never an unconditional overwrite"
    - "Absent-payload-key-reads-as-zero-value house style, reused verbatim for SchemaVersion"

key-files:
  created:
    - internal/migrate/migrate.go
    - internal/migrate/migrate_test.go
    - internal/store/schemaversion_test.go
  modified:
    - internal/store/store.go
    - internal/store/store_test.go
    - docs-site/src/content/docs/guides/upgrade.md

key-decisions:
  - "migrate.CurrentVersion = 0 in this phase (not 1): Phase 2 creates no migration registry and registers no steps, and payload() cannot honour a v1 claim since it omits short_id when empty — stamping 1 would be a false-currency claim. Raising the constant is a Phase 3/4 joint action with registering the step that defines the new version."
  - "Narrowed no-downgrade contract, not a broader CAS: a normal full write stamps at least current, and Store.Update's in-lock refresh (now also copying fresh.SchemaVersion) preserves a decoded newer record's version. Store.Upsert replacement-by-id is explicitly OUTSIDE the guarantee, stated on the field's doc comment rather than papered over."
  - "Reindex's per-point write does NOT call payload() and never advances a record's schema_version (correcting CONTEXT.md D-07's assumption that it 'inherits' the monotonic stamp) — it copies the source payload map byte-for-byte, so converging a version is the future migration sweep's job, not Reindex's."
  - "schema_version gets a Qdrant integer payload index now (D-12), serving the future Phase 3 sweep and Phase 4's migrate status histogram exclusively; plan 02-03 is the gate that proves it never leaks into a recall filter."

patterns-established:
  - "Monotonic version stamp: p[key] = int(max(currentConstant, decodedField)) at the single unconditional payload() write site, with fromPayload()'s guarded `if v, ok := p[key]; ok` decode leaving the Go zero value as the absent-key state."

requirements-completed: [REQ-schema-version-stamped, REQ-schema-version-wire-visible]

coverage:
  - id: D1
    description: "A record written through Store.Upsert carries schema_version = migrate.CurrentVersion through Store.List/Store.Get and the JSON wire; a record written with no schema_version key decodes to v0 with no backfill; the monotonic stamp, idempotence, and exact struct tag are pinned by pure-function tests."
    requirement: "REQ-schema-version-stamped"
    verification:
      - kind: integration
        ref: "internal/store/schemaversion_test.go#TestSchemaVersionEndToEnd"
        status: pass
      - kind: unit
        ref: "internal/store/store_test.go#TestPayloadRoundTripsSchemaVersion"
        status: pass
      - kind: unit
        ref: "internal/migrate/migrate_test.go#TestCurrentVersionValue"
        status: pass
    human_judgment: false
  - id: D2
    description: "schema_version is wire-visible (plain json tag, no omitempty) on full=true recall and get_memory; the Qdrant payload index exists (proven from live collection info); Store.Update's in-lock refresh picks up a concurrently-raised version instead of downgrading it; Reindex's contract is documented as it actually behaves; the full internal/server suite was swept for golden-file blast radius (green, no exact-JSON golden asserts a full memory body)."
    requirement: "REQ-schema-version-wire-visible"
    verification:
      - kind: integration
        ref: "internal/store/schemaversion_test.go#TestEnsureCollectionIndexesSchemaVersion"
        status: pass
      - kind: integration
        ref: "internal/store/schemaversion_test.go#TestUpdateRefreshesSchemaVersionUnderLock"
        status: pass
      - kind: integration
        ref: "internal/store/reindex_test.go#TestReindexRoundtrip"
        status: pass
      - kind: unit
        ref: "go test ./internal/server/... (full suite)"
        status: pass
    human_judgment: false

duration: 30min
completed: 2026-08-13
status: complete
---

# Phase 02 Plan 01: Record Schema Versioning Foundation — Tracer Slice Summary

**New `internal/migrate` leaf package plus a monotonically-stamped, wire-visible `schema_version` field on `store.Memory`, proven end to end against real Qdrant.**

## Performance

- **Duration:** ~30 min
- **Started:** 2026-08-13T18:07:00-04:00 (approx, base commit `ccee2b81`)
- **Completed:** 2026-08-13T18:33:09-04:00
- **Tasks:** 3
- **Files modified:** 6 (3 created, 3 modified)

## Accomplishments

- New stdlib-only `internal/migrate` package: `Version` named type and `CurrentVersion = 0` constant, with the full three-point rationale for why 0 (not 1) recorded on the constant's doc comment.
- `store.Memory.SchemaVersion` (`json:"schema_version"`, no `omitempty`) stamped monotonically at the single `payload()` seam (`max(migrate.CurrentVersion, m.SchemaVersion)`) and decoded absent-safely in `fromPayload()` — a legacy record with no key reads `Version(0)`, no backfill.
- `Store.Update`'s in-lock refresh extended to copy `fresh.SchemaVersion` alongside `Supersedes`/`SupersededBy`/`ArchivedAt`, closing the review-flagged gap where a stale caller snapshot could downgrade a concurrently-raised version.
- `schema_version` Qdrant integer payload index added to `ensureIndexes`, proven from live collection info — serving the future migration sweep exclusively (plan 02-03 is the gate holding that line).
- Corrected the `Store.Reindex` doc comment: its per-point write copies the source payload map verbatim and never calls `payload()`, so it preserves `schema_version` byte-for-byte and never advances it.
- End-to-end proof against real Qdrant (`TestSchemaVersionEndToEnd`), a deterministic in-lock-refresh race proof (`TestUpdateRefreshesSchemaVersionUnderLock`), and a full `internal/server` golden-file sweep — all green.
- `guides/upgrade.md` gained "12. Records now carry a `schema_version` stamp": no backfill required, the rollback lossy-rewrite hazard with the (forthcoming) migration sweep as recovery, and that `engram reindex` does not advance a version.

## Task Commits

1. **Task 1: End-to-end "a stored record carries its schema version" — one path only** — `5ef68517` (feat)
2. **Task 2: Pure-function proofs — the constant's value and the monotonic round trip** — `2e041392` (test)
3. **Task 3: Operator surface — the payload index, the in-lock refresh proof, the corrected Reindex contract, and the upgrade-guide note** — `ee476e2c` (feat)

**Plan metadata:** (this commit)

## Files Created/Modified

- `internal/migrate/migrate.go` — new leaf package: `Version` type, `CurrentVersion = 0` constant with cross-phase rationale
- `internal/migrate/migrate_test.go` — `TestCurrentVersionValue` + compile-time `var _ Version = CurrentVersion` assertion
- `internal/store/store.go` — `Memory.SchemaVersion` field, `schemaVersionKey` constant, the `payload()` stamp, the `fromPayload()` decode, `Store.Update`'s in-lock refresh extension, the `ensureIndexes` payload index entry, and the corrected `Reindex` doc comment
- `internal/store/store_test.go` — `TestPayloadRoundTripsSchemaVersion` (above/equal/zero table, idempotence, legacy decode, struct-tag assertion)
- `internal/store/schemaversion_test.go` — `TestSchemaVersionEndToEnd`, `TestEnsureCollectionIndexesSchemaVersion`, `TestUpdateRefreshesSchemaVersionUnderLock`
- `docs-site/src/content/docs/guides/upgrade.md` — new "12. Records now carry a `schema_version` stamp" subsection under `## Unreleased`

## Decisions Made

- `migrate.CurrentVersion = 0` — resolves CONTEXT.md's Open Question for the Planner (plan text already carried the decision and rationale verbatim; executed as written).
- Narrowed the no-downgrade contract on `SchemaVersion`'s doc comment rather than adding a compare-and-swap to `Store.Upsert`: normal full writes and `Store.Update` are covered; raw `Upsert` replacement-by-id is explicitly excluded and stated as such.
- Corrected the Reindex contract in code comments per the plan's RESEARCH-flagged finding: Reindex's per-point write never calls `payload()`, so it does not advance `schema_version` (superseding CONTEXT.md D-07's inherited-stamp assumption).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `int()` cast required before writing `migrate.Version` into the payload map**
- **Found during:** Task 1
- **Issue:** The plan's pattern guidance (mirroring `AccessCount`'s `p["access_count"] = m.AccessCount` treatment) suggested writing `m.SchemaVersion`-derived value directly into `map[string]any`, letting `qdrant.NewValueMap` convert it. `qdrant.NewValueMap`'s `NewValue` type switch matches only exact concrete types (`int`, `int32`, `int64`, `uint`, `uint32`, `uint64`, ...); `migrate.Version` (a named type over `int`) does not match `case int:` and falls to the `default` arm, which panics with `"invalid type: migrate.Version"`.
- **Fix:** Cast explicitly: `p[schemaVersionKey] = int(max(migrate.CurrentVersion, m.SchemaVersion))`, with an inline comment explaining why the cast is load-bearing, not cosmetic.
- **Files modified:** `internal/store/store.go`
- **Verification:** `TestSchemaVersionEndToEnd` and `TestPayloadRoundTripsSchemaVersion` both exercise `payload()` end to end and pass; without the cast, both would panic on the first `payload()` call carrying a non-zero-need write.
- **Committed in:** `5ef68517` (Task 1 commit)

**2. [Rule 1 - Test design] `TestUpdateRefreshesSchemaVersionUnderLock` implemented as a sequential deterministic injection, not a literal `updateAfterReadHook`-based injection**
- **Found during:** Task 3
- **Issue:** The plan's action text describes setting `updateAfterReadHook` to perform the raw version-raising `SetPayload`, asserting the final stored version equals the raised value. Reading `Store.Update`'s actual code shows the hook fires strictly AFTER the in-lock re-read and strictly BEFORE the final `Upsert` — so any value the hook injects into Qdrant is unconditionally overwritten moments later by that same `Update` call's own `Upsert` (which writes `payload(cur)` computed from `cur.SchemaVersion` as captured BEFORE the hook ran). Following the plan literally would make the test assert `raised` while the code would actually store `cur`'s pre-injection value — the test would either fail or (worse) pass only by accident.
- **Fix:** Implemented the test with the raw `SetPayload` injection performed sequentially between `FetchForUpdate` (which snapshots `cur` at the pre-raise version) and the call to `Store.Update` — landing unambiguously in the window Task 1's in-lock refresh fix exists to cover (a version raised after the caller's external snapshot but before `Update`'s own internal re-read). This is still fully deterministic (no goroutines), still restores no lingering state (never sets `updateAfterReadHook` at all in this test, so nothing needs restoring), and correctly distinguishes pre-fix from post-fix behavior: without Task 1's `fresh.SchemaVersion` copy, this test would fail (final version would incorrectly be downgraded back to the stale snapshot's value). Documented the reasoning and the scope this leaves unproven (the narrower post-re-read window) directly in the test's doc comment.
- **Files modified:** `internal/store/schemaversion_test.go`
- **Verification:** `TestUpdateRefreshesSchemaVersionUnderLock` passes; manually traced that removing Task 1's `fresh.SchemaVersion` copy from `Store.Update` would make this test fail (confirming it is fix-differentiating).
- **Committed in:** `ee476e2c` (Task 3 commit)

**3. [Rule 1 - Scope hygiene] Reverted unrelated `task fmt` drift**
- **Found during:** Task 1
- **Issue:** `task fmt` reformatted `.claude/settings.json`, `docs-site/package.json`, `internal/webauth/static/_app/version.json`, and `ui/tsconfig.json` — pre-existing formatting drift unrelated to this plan's files.
- **Fix:** `git checkout --` those four files before committing, keeping the commit scoped to this plan's actual changes.
- **Files modified:** none (reverted, not committed)
- **Verification:** `git status --short` confirmed only this plan's files remained staged.
- **Committed in:** n/a (reverted before commit)

**4. [Rule 1 - Verify-command wording] Avoided a duplicate literal match against the plan's own acceptance-criteria grep**
- **Found during:** Task 1
- **Issue:** Task 1's acceptance criteria requires exactly one occurrence of the literal string `max(migrate.CurrentVersion` in `store.go`. The first draft of `SchemaVersion`'s field doc comment restated that exact expression in prose, producing two matches.
- **Fix:** Reworded the doc comment to describe the rule ("the greater of the current schema constant and the record's own decoded value") without repeating the literal expression.
- **Files modified:** `internal/store/store.go`
- **Verification:** `rg -n 'max\(migrate\.CurrentVersion' internal/store/store.go` reports exactly one line (the code, not the comment).
- **Committed in:** `5ef68517` (Task 1 commit)

---

**Total deviations:** 4 auto-fixed (2 Rule 1 bugs/correctness, 1 scope-hygiene revert, 1 acceptance-criteria wording fix)
**Impact on plan:** All four were necessary for the code to compile/run correctly or for the commit to stay scoped — no architectural changes, no scope creep.

## Out-of-Scope Discovery (logged, not fixed)

`go test ./...` (full-module sweep, Task 3 verification) surfaced a PRE-EXISTING failure in `internal/keylinks`'s `TestNoEscapedPatternsRepoWide`, flagging backslash-escaped `.` in `key_links.pattern` frontmatter fields across `.planning/phases/02-record-schema-versioning-foundation/{02-01,02-02,02-03,02-04}-PLAN.md`. These PLAN.md files predate this execution session (last touched by `18f6237b`, a planning-phase commit) and are not files this plan modifies or owns — `02-02`/`02-03`/`02-04` belong to sibling plans in this phase's later waves. Logged to `.planning/phases/02-record-schema-versioning-foundation/deferred-items.md` per the scope-boundary rule; not fixed here.

## Issues Encountered

None beyond the deviations documented above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- The tracer slice is proven end to end: `internal/migrate.Version`/`CurrentVersion`, `store.Memory.SchemaVersion`, the monotonic stamp, the absent-safe decode, the Qdrant payload index, and the wire-shape all exist and are tested against real Qdrant.
- Plans 02-02 (structural stamp-routing gate), 02-03 (recall-gate proof), 02-04 (forward/backward compat), and 02-05 (Connect wire test, if scheduled this phase) can now build their gates over this proven foundation rather than a hypothetical one.
- `internal/migrate` is ready for Phase 3 to grow the migration-step registry into the same package, per D-04.
- One pre-existing, out-of-scope repo-wide test failure (`TestNoEscapedPatternsRepoWide` against sibling PLAN.md files) is logged in `deferred-items.md` for a future plan to address — it does not block this plan's own verification, which targeted `./internal/migrate/...`, `./internal/store/...`, and `./internal/server/...` per the plan's own `<verification>` block.

---
*Phase: 02-record-schema-versioning-foundation*
*Plan: 01*
*Completed: 2026-08-13*
