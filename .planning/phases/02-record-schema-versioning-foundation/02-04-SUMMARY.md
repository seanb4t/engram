---
phase: 02-record-schema-versioning-foundation
plan: 04
subsystem: database
tags: [qdrant, go, schema-versioning, forward-compatibility, testing]

requires:
  - phase: 02-record-schema-versioning-foundation
    provides: internal/migrate.Version/CurrentVersion, store.Memory.SchemaVersion's monotonic stamp and absent-safe decode (plan 02-01)
provides:
  - "internal/store/schemaversion_compat_test.go: TestSchemaVersionForwardBackwardCompat — raw-payload-injection proof of ROADMAP success criterion 5 (forward AND backward schema_version compatibility) against real Qdrant"
affects: [phase-03-migration-foundation, phase-04-migration-cli, phase-05-connect-record-state-parity]

actuals:
  tokens: 5718
  tasks: 2
  commits: 1

tech-stack:
  added: []
  patterns:
    - "Raw-payload-injection compatibility proof: SetPayload/DeletePayload directly through the Qdrant client, bypassing payload() entirely, so the test evidence is about DECODING an unfamiliar payload rather than the stamping plumbing that produced it."
    - "Two-record-per-row seeding (active + pending-windowed) for any claim spanning Search/List and ListScheduled — the two recall gates select disjoint populations, so a single record cannot prove membership in both."
    - "Shared per-(record, recall-path) applicability matrix, executed-count-checked, reused verbatim across every compatibility row instead of re-derived per row."
    - "Row set size derived from migrate.CurrentVersion (2 while 0, 3 once raised) with a runtime-checked executed-count assertion instead of a t.Skip, so the older-than-binary row self-activates when a later phase raises the constant."

key-files:
  created:
    - internal/store/schemaversion_compat_test.go
  modified: []

key-decisions:
  - "Both plan tasks (newer-than-binary proof; older-than-binary proof + derived row count) were implemented and committed as ONE commit rather than two. Task 2's row-set derivation, coverage-checked-boolean, and shared applicability matrix are structural scaffolding that Task 1's 'newer' row already had to be written against (the plan explicitly says Task 2 must 'reuse the matrix helper rather than re-deriving the expectations per row') — splitting the diff into two commits would require fabricating an artificial intermediate state (e.g., a matrix with no reuse contract yet) that was never actually built or tested standalone. The full file was written, compiled, and verified as one coherent unit before the first commit."
  - "The older-than-binary direction at CurrentVersion=0 is proven by the key-absent legacy record ('absent' row) exactly as the plan specifies — CurrentVersion-1 is not representable at 0, so 'older-explicit' is gated behind `migrate.CurrentVersion > 0` and does not execute yet. The row count and the older-than-coverage claim are both runtime-checked against migrate.CurrentVersion, not hardcoded to 2."
  - "Unknown-key value types for the 'newer' row: bool + string (no numeric literal), sidestepping any ambiguity with the plan's 'no bare integer literal as a version value' constraint."

patterns-established:
  - "compatRow / schemaVersionCompatMatrix: a row-driven compatibility-table pattern (derived row count, shared per-path applicability matrix, executed-count self-checks) available as precedent for any future recall-semantics-across-multiple-record-shapes proof."

requirements-completed: [REQ-schema-version-forward-compatible]

coverage:
  - id: D1
    description: "A binary reads a record whose schema_version is NEWER than its own migrate.CurrentVersion (plus two unrecognized payload keys of different types) without rejecting, hiding, or downgrading it — decoded via Store.Get, recalled per an explicit per-path applicability matrix (Search/SearchReranked/SearchDiscovery present; List/ListScheduled per the record's window), and undowngraded across a subsequent Store.Update, with D-06's unknown-key loss on rewrite asserted rather than assumed."
    requirement: "REQ-schema-version-forward-compatible"
    verification:
      - kind: integration
        ref: "internal/store/schemaversion_compat_test.go#TestSchemaVersionForwardBackwardCompat/newer"
        status: pass
    human_judgment: false
  - id: D2
    description: "The older-than-binary direction is covered honestly at CurrentVersion=0 by the key-absent legacy record (raw-deleted schema_version key, verified absent on the RAW payload, not inferred from a decoded zero), decoding to migrate.Version(0), with the same recall-matrix and post-Update proof. The row set (2 rows at CurrentVersion=0, 3 once raised) is asserted via a runtime-checked executed-row-count, never a t.Skip, so the explicit older-explicit row self-activates when Phase 3/4 raises the constant."
    requirement: "REQ-schema-version-forward-compatible"
    verification:
      - kind: integration
        ref: "internal/store/schemaversion_compat_test.go#TestSchemaVersionForwardBackwardCompat/absent"
        status: pass
      - kind: integration
        ref: "go test -v -run 'TestSchemaVersionForwardBackwardCompat$' ./internal/store/... | rg -c -- '--- PASS: TestSchemaVersionForwardBackwardCompat/' == 2"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-08-13
status: complete
---

# Phase 02 Plan 04: Forward/Backward Schema-Version Compatibility Summary

**Raw-payload-injection proof (`TestSchemaVersionForwardBackwardCompat`) that a binary never rejects, hides, or downgrades a record stamped one version ahead of its own `migrate.CurrentVersion`, mirrored by the genuine key-absent legacy-record proof for the older-than-binary direction — both directions run against real Qdrant with every version value derived from `migrate.CurrentVersion`.**

## Performance

- **Duration:** ~25 min
- **Started:** approx 2026-08-13T18:26:00-04:00 (base commit `a8d75017`)
- **Completed:** 2026-08-13T18:51:10-04:00 (commit `186a0d33`)
- **Tasks:** 2 (committed together — see Decisions Made)
- **Files modified:** 1 (created)

## Accomplishments

- New `internal/store/schemaversion_compat_test.go` holding `TestSchemaVersionForwardBackwardCompat`, routed through `testCollection("schemaversion_compat")` and `newTestStore` — never a raw `New`, never an unprefixed collection name.
- A reusable raw-injection helper trio (`injectRawPayload`/`deleteRawPayloadKeys`/`rawPayload`) that writes/reads payload directly through the Qdrant client's `SetPayload`/`DeletePayload`/`Get`, bypassing `payload()` entirely — the file contains no call to `payload(` to construct an injected record's payload.
- The `newer` row: seeds an `active` and a `windowed` discovery record identically, raw-injects `schema_version = migrate.CurrentVersion + 1` plus two unknown payload keys (`bool` and `string` — deliberately no numeric literal), and asserts decode success, known-field survival, per-path recall membership, and — after a `Store.Update` on the active record — that the version stays elevated while the two future-only keys are gone from the raw stored payload (D-06's accepted loss, checked rather than assumed).
- The `absent` row: raw-deletes the `schema_version` key from both seeded records (the only way to construct the genuine key-absent legacy-record shape, since `payload()` always writes the key), asserting the key is genuinely gone from the RAW payload (not inferred from a decoded zero), decode to `migrate.Version(0)`, and a post-`Update` version of exactly `migrate.CurrentVersion`.
- A shared `schemaVersionCompatMatrix` (10 entries: 2 records × 5 recall paths — `Search`, `SearchReranked`, `SearchDiscovery`, `List`, `ListScheduled`) reused verbatim by every row, with each entry's boolean expectation asserted against actual membership-by-id and a written reason (e.g., the pending record is absent from `Search`/`List` because of the active-window gate, but present in `SearchDiscovery` because that path applies no temporal gate at all) — executed-entry-count checked equal to the enumerated count on every row.
- Row-set derivation: `expectedRowCount` computed independently from `migrate.CurrentVersion` (2 while 0, 3 once raised), the constructed `rows` slice cross-checked against it, and the actually-executed subtest count checked again after the loop — no `t.Skip` anywhere. The older-than-binary coverage claim is a runtime-checked boolean (`olderThanCovered`), not a comment: true today because `absent` ran with its `coversOlderThan` flag; the day the constant is raised, it flips to requiring `older-explicit` to have actually run.
- The previously-flagged probe item is resolved per the plan text (covered by D-08; no reviewer action outstanding); this plan does not re-litigate it.

## Task Commits

Both tasks were implemented and verified together as one coherent test file (see "Decisions Made" for why), then committed atomically:

1. **Task 1 (newer-than-binary proof) + Task 2 (older-than-binary proof + derived row count)** — `186a0d33` (test)

**Plan metadata:** (this commit)

## Files Created/Modified

- `internal/store/schemaversion_compat_test.go` — `TestSchemaVersionForwardBackwardCompat`, `compatRow`, `schemaVersionCompatMatrix`, `runCompatRow`, `assertApplicabilityMatrix`, `assertKnownFieldsIntact`, `injectRawPayload`, `deleteRawPayloadKeys`, `rawPayload`

## Decisions Made

- **Both plan tasks committed as one commit.** Task 2's action text explicitly directs reusing Task 1's applicability-matrix helper "rather than re-deriving the expectations per row," and its row-count derivation wraps around the same `rows` slice Task 1's `newer` row is a member of. Building Task 1 in isolation, committing it, then extending it for Task 2 would have meant either (a) writing a throwaway single-row version of the shared matrix/derivation scaffolding first and then immediately reworking it (net-negative: two diffs where the second largely reverts the first's simplifications), or (b) writing the real shared scaffolding under a "Task 1" commit message that already describes work Task 2 owns. Neither serves the atomic-commit goal better than one commit whose message names both tasks. The full file was written, `go build`/`go vet`/`gofmt`/`task lint`/the exact Task 1 and Task 2 verify commands/the full `internal/store` suite/the full `go test ./...` suite all ran and passed BEFORE the single commit landed.
- Unknown-key value types for the `newer` row: `bool` + `string`, not `int` — sidesteps any ambiguity with the plan's "no bare integer literal is used as a version value anywhere in the file" constraint, since an unrelated `int` payload value could invite a reviewer to ask whether it was meant as a version.
- Deterministic point IDs (`cf000000-0000-0000-0000-<row-tag><kind>`) rather than randomly generated ones, matching the project's existing test-fixture-ID convention (`schemaversion_test.go`'s `c0000000-...`, `store_test.go`'s `e4000000-...`) and making a failing assertion's id immediately traceable to (row, record-kind) by inspection.

## Deviations from Plan

None — plan executed exactly as written, including the two review-driven corrections already folded into the plan text (two-record-per-row seeding; the resolved probe item). The task-granularity commit decision above is a mechanical execution choice, not a deviation from any `<action>` or `<acceptance_criteria>` — every acceptance criterion in both tasks is met.

## Issues Encountered

None. Docker was available locally; `TestMain` provisioned a `qdrant/qdrant:v1.18.2` testcontainer for every run without falling back to `ENGRAM_QDRANT_TEST_ADDR`.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- ROADMAP success criterion 5 (forward/backward `schema_version` compatibility) is proven against real Qdrant in both directions, closing out `REQ-schema-version-forward-compatible`.
- The row-derivation and shared-applicability-matrix pattern in this file is reusable precedent: when Phase 3/4 raises `migrate.CurrentVersion` above 0, `TestSchemaVersionForwardBackwardCompat`'s `older-explicit` row activates automatically (no code change needed here) and the row-count/coverage assertions will start requiring it to have actually run.
- `go test ./...` (full module) is green, including `internal/keylinks` (no escaped-pattern regression introduced by this plan) and `task lint` (golangci-lint, yamlfmt, actionlint, rumdl, ruff) is clean.
- Plan 02-05 (Connect wire test, if scheduled this phase) can build over this proof without re-establishing the recall-semantics-divergence facts this plan pins in comments.

---
*Phase: 02-record-schema-versioning-foundation*
*Plan: 04*
*Completed: 2026-08-13*

## Self-Check: PASSED

File verified present: `internal/store/schemaversion_compat_test.go` (FOUND).
Commit hash verified present in `git log`: `186a0d33` (FOUND).
