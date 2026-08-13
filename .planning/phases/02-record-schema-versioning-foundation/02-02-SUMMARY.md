---
phase: 02-record-schema-versioning-foundation
plan: 02
subsystem: database
tags: [qdrant, go, go-ast, schema-versioning, conformance-gate]

requires:
  - phase: 02-record-schema-versioning-foundation
    provides: "02-01's monotonic schema_version stamp at the payload() seam — the seam this plan's gate proves is the only door"
provides:
  - "Write-boundary AST conformance gate (TestEveryPointWriteRoutesThroughPayload) anchored on the transmitted Upsert call site, not construction"
  - "Partial-write classification gate (TestPartialWritePathsAreClassifiedNonStamping) proving D-02's non-stamping requirement structurally across all 10 SetPayload/DeletePayload sites"
  - "Cross-package qdrant.Client holder guard (TestQdrantClientIsHeldOnlyByStorePackage)"
  - "Behavioral per-write-method stamping proof (TestEveryFullWriteMethodStampsSchemaVersion) against real Qdrant"
  - "Three reviewer-reproducible prove-RED patches under red-evidence/"
affects: [02-03-plan-tests, 02-04-plan-tests, phase-03-migration-foundation]

actuals:
  tokens: 15416
  tasks: 3
  commits: 4

tech-stack:
  added: []
  patterns:
    - "Write-boundary AST gate anchored on the TRANSMITTED call site (callee method name), not on composite-literal construction — over-approximating match, receiver recorded as classification metadata never a filter"
    - "Two-tier gate: a boundary scan for completeness (which functions transmit) paired with a narrower conformance predicate for the one legitimate site (does it route through the codec)"
    - "Set-equality classification in both directions (unclassified derived site AND stale classification entry both fail), never a subset-only or count>0 check"
    - "Prove-RED-then-revert via exact inverse git patch, never git checkout --, with the injected hunk committed as a reviewer-reproducible artifact"

key-files:
  created:
    - internal/store/schemaversion_stamp_gate_test.go
    - internal/store/schemaversion_stamp_test.go
    - internal/store/testdata/schemaversionstamp/good_pkg.go.txt
    - internal/store/testdata/schemaversionstamp/bad_pkg.go.txt
    - internal/store/testdata/schemaversionstamp/limits_pkg.go.txt
    - .planning/phases/02-record-schema-versioning-foundation/red-evidence/02-02-red-1-bypass.patch
    - .planning/phases/02-record-schema-versioning-foundation/red-evidence/02-02-red-2-stale-classification.patch
    - .planning/phases/02-record-schema-versioning-foundation/red-evidence/02-02-red-3-cross-package-client.patch
  modified:
    - .planning/phases/02-record-schema-versioning-foundation/02-01-PLAN.md

key-decisions:
  - "The gate is anchored on the transmitted write boundary (Upsert call sites, method-name-only match) rather than qdrant.PointStruct construction, per the plan's redesign — this is what lets bad_pkg.go.txt's helper-built-request bypass (no PointStruct literal in the transmitting function at all) be caught."
  - "fullWriteClassification carries exactly four entries in three dispositions (Store.Upsert direct-conforming; Store.Update/Store.Supersede conforming delegations; Store.Reindex a named raw-copy exception) — verified against the scanner's own matching rule, not the narrower client\\.Upsert grep the plan's earlier draft used."
  - "partialWriteClassification's ten entries are named at the level the scanner actually reports (Store.defaultSetPayloadKeys / Store.defaultDeletePayloadKeys as seam-level entries), not at the level D-02's prose describes the callers — matching what the derivation reports rather than a hand-written expectation."
  - "The cross-package composition-root check (TestQdrantClientIsHeldOnlyByStorePackage) scopes 'no write calls' to the qdrant.Client-bound identifier itself, not a blind method-name scan over the whole file — see Deviations below."

patterns-established:
  - "qdrantCallSite / scanQdrantCalls / scanPackageDirForCalls / receiverText are built with a caller-supplied method-name set specifically so plan 02-03's emission-site scan can reuse them without a second scanner ever existing."

requirements-completed: [REQ-schema-version-stamped]

coverage:
  - id: D1
    description: "Every direct-selector-call Upsert transmission in internal/store's package directory is derived, classified by set equality against a four-entry three-disposition table with per-entry receiver assertion, and the direct-conforming site is separately proven to route through payload()."
    requirement: "REQ-schema-version-stamped"
    verification:
      - kind: unit
        ref: "internal/store/schemaversion_stamp_gate_test.go#TestEveryPointWriteRoutesThroughPayload"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every SetPayload/DeletePayload/OverwritePayload call site (10 total, across store.go, spine.go, summarize.go) is derived and classified non-stamping with a D-02 justification."
    requirement: "REQ-schema-version-stamped"
    verification:
      - kind: unit
        ref: "internal/store/schemaversion_stamp_gate_test.go#TestPartialWritePathsAreClassifiedNonStamping"
        status: pass
    human_judgment: false
  - id: D3
    description: "internal/store is the only non-test package directory holding/constructing a *qdrant.Client apart from one allowlisted composition root (internal/server/tools.go), which itself never issues a write on the client it holds."
    requirement: "REQ-schema-version-stamped"
    verification:
      - kind: unit
        ref: "internal/store/schemaversion_stamp_gate_test.go#TestQdrantClientIsHeldOnlyByStorePackage"
        status: pass
    human_judgment: false
  - id: D4
    description: "Every full-write mechanism (Upsert fresh, Upsert scheduled, Update, Update-preserves-newer, Supersede) is observed stamping schema_version against real Qdrant; the partial-write path (SetVisibility) is observed NOT stamping a v0 record."
    requirement: "REQ-schema-version-stamped"
    verification:
      - kind: integration
        ref: "internal/store/schemaversion_stamp_test.go#TestEveryFullWriteMethodStampsSchemaVersion"
        status: pass
    human_judgment: false
  - id: D5
    description: "The gate was proven RED in three directions against real source (a helper-built bypass, a removed classification entry, a cross-package client field) and reverted by exact inverse patch, each committed as a reviewer-reproducible artifact."
    verification: []
    human_judgment: true
    rationale: "Reproducing a prove-RED cycle requires running git apply / go test / git apply -R by hand — this is a process property, not something a single automated check status can represent; the SUMMARY's Prove-RED evidence section gives the exact commands."

duration: ~19min
completed: 2026-08-13
status: complete
---

# Phase 02 Plan 02: Write-Boundary Schema-Version Conformance Gate Summary

**AST conformance gate anchored on the transmitted `Upsert` call site (not construction) proving `payload()` is the only door for full-record Qdrant writes, paired with a partial-write classification gate, a cross-package `*qdrant.Client` holder guard, and a six-row behavioral stamping proof — all against real Qdrant, with prove-RED evidence in three directions committed as reviewer-reproducible patches.**

## Performance

- **Duration:** ~19 min
- **Started:** 2026-08-13T18:47:00-04:00 (approx)
- **Completed:** 2026-08-13T19:05:48-04:00
- **Tasks:** 3
- **Files modified:** 8 (created) + 1 (modified: 02-01-PLAN.md's artifact table)

## Accomplishments

- `scanQdrantCalls`/`scanPackageDirForCalls`: reusable `go/ast`-only scanners matched on selector method name alone, deliberately ignoring the receiver expression — the redesign that catches a bypass whose request came from a helper, clone, parameter, or generated builder even when it constructs no new `qdrant.PointStruct` literal.
- `payloadDerivesFromCodec`: a separate conformance predicate applied only to the one call site the boundary scan classifies direct-conforming, checking that its `Payload` value is `qdrant.NewValueMap(payload(...))` — never conflated with the completeness derivation.
- Three fixtures transmitting real `Upsert` calls: `good_pkg.go.txt` (two direct writes + one delegation self-call), `bad_pkg.go.txt` (both bypass shapes, including the H1 helper-built-request bypass whose transmitting function contains no `PointStruct` literal at all), `limits_pkg.go.txt` (a method-value and method-expression write, pinned as an asserted-EMPTY derived set — the documented blind spot).
- Applied the scanner to `internal/store`'s own directory: a four-entry three-disposition full-write classification (`Store.Upsert` direct-conforming, `Store.Update`/`Store.Supersede` conforming delegations, `Store.Reindex` a named raw-copy exception) and a ten-entry partial-write classification, both asserted by set equality in both directions with per-entry receiver-text checks.
- `TestQdrantClientIsHeldOnlyByStorePackage`: walks the whole module for non-test `.go` files naming or constructing a `*qdrant.Client`, asserting the derived set is exactly `{internal/store/store.go, internal/server/tools.go}` and that the composition-root entry never itself issues a write on the client variable it holds.
- `TestEveryFullWriteMethodStampsSchemaVersion`: six behavioral rows against real Qdrant (fresh upsert, scheduled upsert, update, update-preserves-newer, supersede, partial-write-negative), every expected version expressed via `migrate.CurrentVersion`.
- Prove-RED evidence in three directions, each a committed, `git apply --check`-clean patch under `red-evidence/`, reverted by exact inverse patch (never `git checkout --`).

## Task Commits

1. **Task 1: The write-boundary scanner, the conformance predicate, and fixtures that transmit real requests** — `549cfd2b` (test)
2. **Task 2: Classify the real package's write boundary — full writes, partial writes, cross-package client holders — and prove RED three ways** — `a0181d34` (test)
3. **Task 3: Behavioral proof — every full-write method stamps, and no partial-write method does** — `15059e92` (test)

**Plan metadata:** (this commit)

## Files Created/Modified

- `internal/store/schemaversion_stamp_gate_test.go` — the AST scanner (`scanQdrantCalls`, `scanPackageDirForCalls`, `receiverText`, `enclosingFuncDisplayName`), the conformance predicate (`payloadDerivesFromCodec`), the full-write and partial-write classification tables, `TestEveryPointWriteRoutesThroughPayload` (8 subtests), `TestPartialWritePathsAreClassifiedNonStamping`, the cross-package guard (`scanRepoForQdrantClientRefs`, `qdrantClientLocalNames`, `TestQdrantClientIsHeldOnlyByStorePackage`)
- `internal/store/schemaversion_stamp_test.go` — `TestEveryFullWriteMethodStampsSchemaVersion` (6 behavioral rows)
- `internal/store/testdata/schemaversionstamp/{good_pkg.go.txt,bad_pkg.go.txt,limits_pkg.go.txt}` — the three fixtures
- `.planning/phases/02-record-schema-versioning-foundation/red-evidence/{02-02-red-1-bypass.patch,02-02-red-2-stale-classification.patch,02-02-red-3-cross-package-client.patch}` — the three prove-RED patches
- `.planning/phases/02-record-schema-versioning-foundation/02-01-PLAN.md` — added the two artifact-table entries this revision's implementation surfaced beyond the plan's own list (`payloadDerivesFromCodec`, `internal/store/schemaversion_stamp_test.go`), so the drift/source-grounding exclusion list stays accurate

## Decisions Made

- Full-write classification kept at exactly four entries in three dispositions, verified against the scanner's own matching rule (`rg -n '\.Upsert\(' internal/store --glob '*.go' --glob '!*_test.go'`) rather than the narrower `client\.Upsert` grep an earlier plan draft used — matches the plan's own "gate redesign" resolution.
- Partial-write classification's ten entries named at the level the scan actually reports (`Store.defaultSetPayloadKeys`/`Store.defaultDeletePayloadKeys` as seam-level entries alongside the direct callers), matching the derivation rather than D-02's prose description of the callers.
- `payloadDerivesFromCodec` locates the `Payload:` key-value element by searching any composite literal in the enclosing function for that key, rather than requiring the literal's own type name to resolve to `qdrant.PointStruct` — `Store.Upsert`'s actual write is an elided-type literal inside a `[]*qdrant.PointStruct{{...}}` slice literal, a third shape beyond the plan's two named forms (`&qdrant.PointStruct{...}` and the bare form). This is consistent with the gate's own no-type-identity limitation (stated in its doc comment) and was verified safe: the codebase has exactly one `Payload:`-keyed composite literal per relevant enclosing function.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Cross-package composition-root check scoped to the client-bound identifier, not a blind method-name scan**
- **Found during:** Task 2, while implementing `TestQdrantClientIsHeldOnlyByStorePackage`'s "the composition root must never itself transmit a write" assertion.
- **Issue:** The plan's action text says to assert `internal/server/tools.go` "contains no call site whose callee method name is in the full-write or partial-write method sets." A literal implementation (scanning the whole file with `scanQdrantCalls`'s bare method-name match) discovers real, entirely legitimate `d.st.Upsert(...)` calls at `tools.go:1147` and `tools.go:1307` — calls to `*store.Store`'s own already-gated `Upsert` method (the tool handlers' normal write path), not to the `*qdrant.Client` (`qc`) the file briefly holds and hands to `store.New`. A literal implementation would make this test permanently RED against unmodified, correct source — exactly the "false assurance" class of gate defect this whole phase exists to eliminate (durable record `x6v6qxqd6f`).
- **Fix:** Added `qdrantClientLocalNames`, which derives the identifier(s) in the file bound directly to a `*qdrant.Client` value returned by `qdrant.NewClient(...)` (here, `qc`), then filters `scanQdrantCalls`'s over-approximating match to only the sites whose recorded receiver text equals one of those names. This preserves the over-approximating boundary-scan philosophy for the RIGHT question ("did the qdrant.Client variable itself transmit a write") while not double-counting internal/store's own already-gated write path through `*store.Store`.
- **Files modified:** `internal/store/schemaversion_stamp_gate_test.go`
- **Verification:** `TestQdrantClientIsHeldOnlyByStorePackage` passes against real `tools.go` (with its legitimate `d.st.Upsert` calls present) and was proven to still fail loudly when a genuine cross-package client-holder is introduced (prove-RED direction 3, below — a different file, since `tools.go` is itself the allowlisted holder).
- **Committed in:** `a0181d34` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 — bug, a literal implementation would have shipped a permanently-false-positive gate)
**Impact on plan:** Necessary for the gate to be correct against real source; no scope creep — the fix is scoped entirely to the one assertion the plan's prose under-specified relative to the codebase's actual shape.

## Prove-RED Evidence

All three cycles used the required exact-inverse-patch procedure (never `git checkout --`), verified with `git diff --exit-code` scoped to the touched file after each revert.

### Direction 1 — a helper-built bypass added to real `internal/store/store.go`

Injected `proveRedDirection1Bypass`/`proveRedDirection1BypassBuildRequest` right after `Store.Upsert`: a function that assembles its request via a separate helper and calls `s.client.Upsert` directly, bypassing `payload()` — mirroring `bad_pkg.go.txt`'s H1 shape but against real, non-fixture source.

**Observed failure:**
```
schemaversion_stamp_gate_test.go:633: derived write site Store.proveRedDirection1Bypass (store.go:836) has no classification entry — every Upsert call site in internal/store's package directory must be classified
--- FAIL: TestEveryPointWriteRoutesThroughPayload/real_package (0.01s)
```

Reverted by `git apply -R`; `git diff --exit-code -- internal/store/store.go` succeeded; gate re-ran green.

**Reproduce:**
```
git apply .planning/phases/02-record-schema-versioning-foundation/red-evidence/02-02-red-1-bypass.patch
go test -v -run 'TestEveryPointWriteRoutesThroughPayload$' ./internal/store/...   # expect FAIL, naming Store.proveRedDirection1Bypass
git apply -R .planning/phases/02-record-schema-versioning-foundation/red-evidence/02-02-red-1-bypass.patch
```

### Direction 2 — the `Store.Reindex` classification entry removed

Removed the `Store.Reindex` entry from `fullWriteClassification` in `schemaversion_stamp_gate_test.go`, leaving its derived site unclassified.

**Observed failure:**
```
schemaversion_stamp_gate_test.go:633: derived write site Store.Reindex (store.go:3294) has no classification entry — every Upsert call site in internal/store's package directory must be classified
--- FAIL: TestEveryPointWriteRoutesThroughPayload/real_package (0.01s)
```

Reverted by `git apply -R` against a diff isolated against the Task 2 staged baseline (the entry addition and its removal are two independent hunks in git's index/working-tree model, so the captured patch contains only the removal); `git diff --exit-code` against that baseline succeeded; gate re-ran green.

**Reproduce:**
```
git apply .planning/phases/02-record-schema-versioning-foundation/red-evidence/02-02-red-2-stale-classification.patch
go test -v -run 'TestEveryPointWriteRoutesThroughPayload$' ./internal/store/...   # expect FAIL, naming Store.Reindex
git apply -R .planning/phases/02-record-schema-versioning-foundation/red-evidence/02-02-red-2-stale-classification.patch
```

### Direction 3 — a `*qdrant.Client` field added outside `internal/store`

First attempted on `internal/server/tools.go` — but that file is itself the allowlisted composition root, so no RED was observed (a genuine, useful confirmation that the allowlist is doing its job, not a defect). Reverted that attempt and instead added `proveRedDirection3Client *qdrant.Client` to the `usageQueue` struct in `internal/server/usagequeue.go`, a file with no prior `*qdrant.Client` reference.

**Observed failure:**
```
schemaversion_stamp_gate_test.go:944: file internal/server/usagequeue.go holds/constructs a *qdrant.Client but is not in the allowlist — a write path outside internal/store may now exist
--- FAIL: TestQdrantClientIsHeldOnlyByStorePackage (0.03s)
```

Reverted by `git apply -R`; `git diff --exit-code -- internal/server/usagequeue.go` succeeded; guard re-ran green.

**Reproduce:**
```
git apply .planning/phases/02-record-schema-versioning-foundation/red-evidence/02-02-red-3-cross-package-client.patch
go test -v -run 'TestQdrantClientIsHeldOnlyByStorePackage$' ./internal/store/...   # expect FAIL, naming internal/server/usagequeue.go
git apply -R .planning/phases/02-record-schema-versioning-foundation/red-evidence/02-02-red-3-cross-package-client.patch
```

## Issues Encountered

None beyond the deviation documented above.

## User Setup Required

None — no external service configuration required. All tests ran against a real Qdrant provisioned automatically by `TestMain` (Docker was available; `ENGRAM_QDRANT_TEST_ADDR` was not required).

## Next Phase Readiness

- `scanQdrantCalls`, `scanPackageDirForCalls`, and `receiverText` are built with a caller-supplied method-name set specifically so plan 02-03's recall-gate emission-site scan can reuse them without a second scanner ever existing.
- The write boundary is now structurally proven at the strength stated in `<success_criteria>`: every direct selector-call `Upsert` transmission in `internal/store`'s package directory, not operation identity — the five named escapes (method value, method expression, cross-package writer, differently-named wrapper, unenumerated verb) remain exactly as disclaimed in the gate's own doc comment.
- Plan 02-03 (recall-gate proof) and plan 02-04 (forward/backward compat) can build on this proven foundation.

---
*Phase: 02-record-schema-versioning-foundation*
*Plan: 02*
*Completed: 2026-08-13*

## Self-Check: PASSED

All created files verified present: `internal/store/schemaversion_stamp_gate_test.go`,
`internal/store/schemaversion_stamp_test.go`,
`internal/store/testdata/schemaversionstamp/{good_pkg.go.txt,bad_pkg.go.txt,limits_pkg.go.txt}`,
`.planning/phases/02-record-schema-versioning-foundation/red-evidence/{02-02-red-1-bypass.patch,02-02-red-2-stale-classification.patch,02-02-red-3-cross-package-client.patch}`.
All 3 task commit hashes verified present in `git log`: `549cfd2b`, `a0181d34`, `15059e92`.
