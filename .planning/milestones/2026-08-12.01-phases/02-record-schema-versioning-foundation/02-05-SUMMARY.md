---
phase: 02-record-schema-versioning-foundation
plan: 05
subsystem: testing
tags: [go, qdrant, schema-versioning, wire-shape, json]

requires:
  - phase: 02-record-schema-versioning-foundation (plan 01)
    provides: store.Memory.SchemaVersion (plain json:"schema_version" tag, no omitempty), migrate.Version/CurrentVersion, monotonic stamp at payload()
provides:
  - internal/server/schemaversion_wire_test.go — TestSchemaVersionOnRecallWire (shapeRecall full=true + compact recallView exclusion + struct-tag pin) and TestSchemaVersionOnGetMemoryWire (get_memory handler path, normal + legacy record)
affects: [phase-03-migration-foundation, phase-04-migration-cli, phase-05-connect-record-state-parity]

actuals:
  tokens: 3312
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Wire-shape proof by decoding into map[string]json.RawMessage and asserting on map-key identity, never by substring-counting the marshalled JSON"
    - "Mirror an existing json:'-' guard (TestEmbedderIdentityNeverOnRecallWire / TestGetMemoryNeverSurfacesEmbedderIdentity) in the opposite direction for a deliberately wire-visible field, documenting both fields' hidden/visible status in the mirroring test's doc comment"
    - "Raw qdrant.Client seed (bypassing store.Store's payload() codec) is the only way to construct an absent-key legacy-record shape in a test"

key-files:
  created:
    - internal/server/schemaversion_wire_test.go
  modified: []

key-decisions:
  - "Task 2's subtests are named with a 'get_memory:' prefix so the plan's literal verify command (`rg -q -- 'get_memory'` against `go test -v` output) matches on the RUN/PASS lines themselves, not just on doc comments — a plain descriptive subtest name would have made the verify command pass only by accident (or not at all)."
  - "TestSchemaVersionOnGetMemoryWire kept as a sibling function (not folded into TestSchemaVersionOnRecallWire) since its harness (testDepsWithStore, real Qdrant, authedContext) is heavy enough to warrant separation — the plan explicitly allowed either shape."
  - "The zero-versioned record in TestSchemaVersionOnRecallWire is constructed by leaving store.Memory.SchemaVersion unset (Go zero value) rather than assigning a literal 0, keeping every version value in the file derived from migrate.CurrentVersion with no bare integer literal standing in for a version."

patterns-established:
  - "dialRawQdrantClient(t) — a test-local raw *qdrant.Client dial mirroring testDepsWithStore's own dial, needed because store.Store's client field is unexported and no seam exists onto it from internal/server; used only to construct payload shapes payload() cannot produce (the absent-schema_version-key legacy record)."

requirements-completed: [REQ-schema-version-wire-visible]

coverage:
  - id: D1
    description: "shapeRecall(full=true) serves schema_version on the wire for both a versioned and a zero-versioned (legacy) record; the compact recallView omits it (D-11); the marshalled JSON is proven to carry exactly one schema_version member by decoding into map[string]json.RawMessage, not by substring counting; store.Memory's SchemaVersion struct tag is pinned to exactly schema_version via reflect.StructTag, alongside proof that EmbedderIdentity and IdempotencyFingerprint still carry their hidden json:\"-\" tag."
    requirement: "REQ-schema-version-wire-visible"
    verification:
      - kind: unit
        ref: "internal/server/schemaversion_wire_test.go#TestSchemaVersionOnRecallWire"
        status: pass
    human_judgment: false
  - id: D2
    description: "get_memory's registered tool handler (deps.getMemory, invoked the same way TestGetMemoryNeverSurfacesEmbedderIdentity does) serves schema_version for a normally-written record (equal to migrate.CurrentVersion) and for a legacy record whose stored payload was seeded directly through the raw Qdrant client with no schema_version key (decodes to zero, is not rejected); both responses are re-asserted free of embedder_identity and idempotency_fingerprint."
    requirement: "REQ-schema-version-wire-visible"
    verification:
      - kind: integration
        ref: "internal/server/schemaversion_wire_test.go#TestSchemaVersionOnGetMemoryWire"
        status: pass
    human_judgment: false

duration: 10min
completed: 2026-08-13
status: complete
---

# Phase 02 Plan 05: Record Schema Versioning Foundation — Recall/get_memory Wire-Shape Proof Summary

**A new `TestSchemaVersionOnRecallWire`/`TestSchemaVersionOnGetMemoryWire` pair in `internal/server` proves `schema_version` is wire-visible on `shapeRecall(full=true)` and `get_memory` — including for zero-versioned legacy records — while the compact `recallView` and the two payload-only audit fields (`embedder_identity`, `idempotency_fingerprint`) stay exactly as hidden as before.**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-08-13T18:39:03-04:00 (approx, base commit `a8d75017`)
- **Completed:** 2026-08-13T18:49:14-04:00
- **Tasks:** 2
- **Files modified:** 1 (created)

## Accomplishments

- `TestSchemaVersionOnRecallWire` (5 subtests): proves `shapeRecall(full=true)` carries `schema_version` for both a versioned and a zero-versioned record (the `omitempty`-would-break-this case); proves `toRecallView`'s compact shape omits it (D-11 scoping, pinned rather than assumed); proves the marshalled JSON carries exactly one `schema_version` member by decoding into `map[string]json.RawMessage` (map-key identity, not substring counting) and decoding that member numerically; and pins `store.Memory.SchemaVersion`'s exact `json` struct tag via `reflect.StructTag`, alongside a re-assertion that `EmbedderIdentity`/`IdempotencyFingerprint` still carry their hidden `json:"-"` tag.
- `TestSchemaVersionOnGetMemoryWire` (2 subtests, against real Qdrant): mirrors `TestGetMemoryNeverSurfacesEmbedderIdentity`'s exact harness and handler call (`d.getMemory(ctx, callerFor(ctx, t), idArgs{ID: id})` — the registered `get_memory` tool handler, not a helper). Proves a normally-written record's `schema_version` equals `migrate.CurrentVersion` on the wire, and that a legacy record (payload seeded directly through a raw `qdrant.Client.Upsert`, bypassing `store.Store`'s `payload()` codec entirely, with the `schema_version` key deliberately absent) is still returned by `get_memory` and decodes to zero rather than disappearing or erroring. Both subtests re-assert absence of `embedder_identity`/`idempotency_fingerprint`.
- Both tests document, in their doc comments, exactly which existing guard they mirror and in which direction — a future reader arriving via either the embedder-identity or the schema-version test can find the other.

## Task Commits

1. **Task 1: The recall wire shape — present on full=true, absent from the compact view, exactly one decoded member** — `bc538c51` (test)
2. **Task 2: The get_memory handler path carries the field for a record fetched by id** — `0b0408be` (test)

**Plan metadata:** (this commit)

## Files Created/Modified

- `internal/server/schemaversion_wire_test.go` — `TestSchemaVersionOnRecallWire` (5 subtests: full-path presence, zero-versioned presence, compact-view absence, exactly-one-member decode, struct-tag pin), `TestSchemaVersionOnGetMemoryWire` (2 subtests: normal record, legacy record with no stored key), plus `dialRawQdrantClient` and `assertNoPayloadOnlyNeighboursOnWire` test helpers.

## Decisions Made

- Named Task 2's subtests with a literal `get_memory:` prefix (e.g. `"get_memory: normally-written record carries the field"`) so the plan's own verify command (`go test -v ... | rg -q -- 'get_memory'`) matches deterministically against the `--- RUN`/`--- PASS` lines in `go test -v` output, rather than depending on the test happening to log that string elsewhere.
- Left `SchemaVersion` unset (Go zero value) rather than writing `SchemaVersion: 0` for the zero-versioned subtest, so every version value in the file traces to `migrate.CurrentVersion` with no bare integer literal standing in for a version (plan acceptance criterion).
- Kept the get_memory proof as a sibling test function (`TestSchemaVersionOnGetMemoryWire`) rather than folding it into `TestSchemaVersionOnRecallWire`, since the plan explicitly permitted either shape and the real-Qdrant harness (`testDepsWithStore`, `authedContext`, raw-client seeding) is heavy enough to warrant separation from the pure-function subtests above it.

## Deviations from Plan

None — plan executed exactly as written. Both tasks' `<action>` and `<acceptance_criteria>` were followed as specified; the only judgment calls made (subtest naming for the verify-command match, sibling-vs-nested function shape) were both explicitly left open by the plan text itself, not deviations from it.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- ROADMAP success criterion 3 is now proven at both wire surfaces this phase scopes: `shapeRecall(full=true)` and `get_memory`. Combined with plan 02-01's tracer-slice proof and plan 02-03's recall-gate proof, `schema_version`'s full wire contract for this phase is tested, not assumed.
- `go test ./internal/keylinks/...` is green — no escaped `\.` regex introduced in this plan's frontmatter `key_links.pattern` fields.
- `task` (lint + test) is green for the full module.
- No new production code was required or added — as the plan's `<objective>` and "Artifacts this phase produces" note stated, `internal/server` needed zero production changes; this plan's entire job was turning the zero-code wire-visibility property into a tested one.

---
*Phase: 02-record-schema-versioning-foundation*
*Plan: 05*
*Completed: 2026-08-13*

## Self-Check: PASSED

File verified present: `internal/server/schemaversion_wire_test.go` (13249 bytes).
Both commit hashes verified present in `git log`: `bc538c51`, `0b0408be`.
