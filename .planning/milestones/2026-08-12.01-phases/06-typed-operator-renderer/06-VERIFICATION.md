---
phase: 06-typed-operator-renderer
verified: 2026-08-17T00:00:00Z
status: passed
score: 3/3 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 6: Typed Operator Renderer Verification Report

**Phase Goal:** Operator command output cannot let a json document silently carry more state than its text sentence states — enforced by construction, not merely detected by test.
**Verified:** 2026-08-17
**Status:** passed
**Re-verification:** No — initial verification

> **Note on `06-VALIDATION.md`:** per the orchestrator's explicit warning, this file is a stale
> RESEARCH-time artifact written before the 2026-08-16 re-discussion that replaced the
> `[]Field`/template design (superseded old D-01..D-07) with the shipped "one serialization, plus
> a view" design (current D-01..D-09 in `06-CONTEXT.md`). It was not used as evidence. Its Wave-0
> file list (`cmd/engram/fieldset.go`, `FieldSet`/`Field` types) and its mandate to add phase 06 to
> `redEvidenceDirs` both describe work that was correctly never built — the redevidence-harness
> deferral is explicit in `06-CONTEXT.md`'s `<deferred>` section and confirmed untouched below.
> This staleness is a known housekeeping item to resolve at sign-off, not a phase gap.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1 — text and json both derive from one shared ordered field set; identity holds by construction | ✓ VERIFIED | `viewFields` (`cmd/engram/operator_view.go:45`) marshals `doc` to bytes exactly once (`rg -o 'json\.Marshal\(' cmd/engram/ --glob '!*_test.go'` → exactly 1 hit, `operator_view.go`) then walks the MARSHALED BYTES with `json.Decoder.Token`/`.Decode` — not struct reflection — so `omitempty`, `json:"-"`, embedded-struct promotion, and custom `MarshalJSON` are all resolved before the text lane ever sees a value. `renderOperator` (`operator_output.go:83`) has exactly one text-mode branch, delegating to `renderOperatorView`, and exactly one json-mode branch (`json.NewEncoder(...).Encode(doc)`) — no second call site from `doc` to rendered output exists. Confirmed by 19 `renderOperator(` call sites across all 15 operator reports (`rg -o` count), all routed through this one function. |
| 2 | SC1 (non-vacuity) — the identity gate can actually fail, and is decomposed from the humanizer per D-06 | ✓ VERIFIED | `TestOrderedKeyDiffDetectsDivergence` (`operator_view_test.go:170`) is a committed table proving dropped/extra/reordered/renamed-key diffs are each non-empty while identical/empty inputs are empty — run directly, all 6 subtests PASS. `TestSetDiffDetectsDivergence` (`operator_output_test.go:247`) is the equivalent non-vacuity proof for the tree-derived coverage gate, independent of `operatorCommands()`/`operatorViewFixtures()` — PASS. `TestHumanizeKey` is a separate table-driven test pinning label text, decoupled from `TestOperatorViewIdentity*` (which asserts Key correspondence only, never Label) per D-06 — 06-01-SUMMARY.md records a live mutation-probe transcript proving the decomposition; independently corroborated by reading `assertViewIdentity` (`operator_view_test.go:120`), which never references `Label`. |
| 3 | SC1 — bidirectional coverage of all 15 operator commands, derived from the live cobra tree, not hand-listed | ✓ VERIFIED | `TestOperatorViewFixturesCoverEveryOperatorCommand` (`operator_output_test.go:215`) computes `want := commandKeySet(operatorCommands())` from the live tree (`cmdwalk.go:101`, walks `rootCmd`, filters by non-nil `RunE`, absence of the client-tier `server` flag, and a 2-entry named exclusion for `serve`/`version`) and diffs it against the merged fixture map in both directions, erroring on `missing` and `extra`, plus a zero-document guard so a key can't satisfy the gate vacuously. Ran directly: 15/15 commands enumerated and PASS (`backfill-short-ids`, `migrate`, `migrate_revert`, `migrate_status`, `migrate-remap-owner`, `migrate-set-owner`, `prune-expired`, `reindex`, `spine-review_{archive,consolidate,purge,restore,scan,verify}`, `summarize-missing`). |
| 4 | SC2 — every existing operator command's `--output json/text` behavior is unchanged (regression-free) | ✓ VERIFIED | `go build ./...` = 0, `go test ./...` = 0 failures (whole module, one run). `TestOperatorOutputEmpty`, `TestOperatorOutputStream`, `TestRenderOperatorTextAndJSON` (never-emit-bare-null, write-to-cmd's-own-writer, exactly-one-trailing-newline contracts) all pass unchanged. The three intentional additive keys were diffed directly: `migrateStatusReportDoc.CurrentVersion` (json `current_version`) appended at the END of the struct — pre-existing keys/order/tags unchanged. `purgeReportDoc.Rerun` (json `rerun,omitempty`) appended at the END of the struct, empty on the applied-run path (matches pre-conversion behavior where only preview printed a re-run line) — pre-existing keys/order/tags unchanged. `spineScanReportDoc.Scope` (json `scope`) was placed FIRST in the struct, which **does reorder** `Total` and every subsequent pre-existing key one position later in both the struct declaration and the marshaled JSON — see caveat below. No key was renamed or removed in any of the three. |
| 5 | SC3 — adding a field to an operator report touches exactly one field-set declaration and appears correctly in both lanes; live-demonstrated | ✓ VERIFIED | 06-07-SUMMARY.md's "SC3 Probe Transcript" section is a real captured run (`go test ./cmd/engram/ -run TestZZZSC3Probe -v`), not a bare assertion: a throwaway `Probe string` field was added to `pruneOutputDoc` and set in exactly one constructor (`pruneReportDoc`), and the captured output shows `"probe":"sc3-probe-value"` in the json lane and a `Probe  sc3-probe-value` line in the text lane with no second call site touched. Confirmed reverted: `git diff --stat 6a0df053..HEAD -- cmd/engram/prune.go` is empty (no trace of `Probe` remains). |
| 6 | D-09 — the obsolete parity gate is retired, and its one good property (bidirectional cobra-tree coverage) is carried forward | ✓ VERIFIED | `rg -n 'TestOperatorOutputParity|operatorParityRows' cmd/engram/` → no matches (exit 1); the test and its hand-built row table are gone. `TestOperatorViewFixturesCoverEveryOperatorCommand` is its documented inheritor (comment at `operator_output_test.go:135-152` records the retirement rationale and the two facts about the old gate per durable record `b3wd4wwwda`) and is derived from `operatorCommands()`, not hand-listed — confirmed above (truth 3). |
| 7 | Deferred red-evidence harness registration honored (not smuggled in) | ✓ VERIFIED | `grep -n '"06' internal/store/redevidence_harness_test.go` → no matches; `git diff --stat 6a0df053..HEAD -- internal/store/redevidence_harness_test.go` → empty. `06-CONTEXT.md`'s `<deferred>` section explicitly defers this, and the codebase confirms it was not touched. |

**Score:** 3/3 roadmap Success Criteria verified (7 supporting truths all VERIFIED, 0 present-but-behavior-unverified)

**Caveat on truth 4 (not a gap):** `spineScanReportDoc.Scope` was inserted as the *first* struct field rather than appended at the end, which reorders the JSON key position of every pre-existing key in that one document (`total` moves from position 1 to position 2, etc.). This is a real, observable change to marshaled byte order for `spine-review scan --output json` — flagged here per the verification brief's explicit instruction to treat any reordering as a potential wire break. It is documented and reasoned about directly in 06-06-SUMMARY.md ("this changes marshaled key order but adds no key removal and no tag change; key order is not part of any consumer contract `encoding/json` guarantees"), and RFC 8259 §4 states JSON object member order is not semantically significant, so no conformant JSON consumer (key-based lookup) observes a behavior change — only a byte-diff would. No key was removed, renamed, or had its tag changed; SC2's "behavior unchanged" is read as semantic behavior (same keys, same values, same client-parseable shape), consistent with D-01's framing of json as "the contract" under `encoding/json`'s own ordering guarantee (none). This is a judgment call worth a human's awareness, not a blocking regression — recorded here rather than silently passed over.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/engram/operator_view.go` | `renderOperatorView`, `viewFields`, `viewField`, `humanizeKey`, `viewScalar`, `viewRow`, `sanitizeViewValue` | ✓ VERIFIED | All seven present, read in full; matches 06-01-PLAN.md's must_haves.artifacts exactly. |
| `cmd/engram/operator_output.go` | `renderOperator` rewired to delegate to the view for text, `json.Encoder` for json | ✓ VERIFIED | Read in full; single dispatch, no second path. |
| `cmd/engram/operator_view_test.go` + 4 group test files | Identity gate, negative-case tables, per-group fixtures | ✓ VERIFIED | `operator_view_test.go`, `operator_view_flat_test.go`, `operator_view_migrate_test.go`, `operator_view_archive_purge_test.go`, `operator_view_scan_test.go` all present, all tests pass. |
| `cmd/engram/operator_output_test.go` | Merged fixture map, bidirectional coverage gate, hand-declared-doc gate | ✓ VERIFIED | `operatorViewFixtures`, `TestOperatorViewFixturesCoverEveryOperatorCommand`, `TestOperatorViewIdentityAcrossEveryOperatorCommand`, `TestOperatorDocsAreHandDeclared`, `setDiff`/`TestSetDiffDetectsDivergence` all present and passing. |
| `TestOperatorOutputParity` / `operatorParityRows` | Retired per D-09 | ✓ VERIFIED (absence confirmed) | `rg` returns zero matches anywhere in `cmd/engram/`. |
| `cmd/engram/fieldset.go`, `FieldSet`/`Field` types | N/A — superseded design, correctly never built | N/A | Per orchestrator's explicit note; confirmed absent, correctly so. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| 15 operator report constructors (`pruneReportDoc`, `archiveReportDoc`, `purgeReportDoc`, `spineScanReportDoc`, `migrateStatusReportDoc`, etc.) | `renderOperator` | 19 call sites across `prune.go`, `migrate.go`, `migrate_family.go`, `reindex.go`, `summarize.go`, and the six `spine_review_*.go` leaves | WIRED | `rg -o 'renderOperator\('` across non-test `cmd/engram/*.go` = 19 call sites + 1 definition, matching `06-CONTEXT.md`'s documented "15 reports (19 call sites)". |
| `renderOperator` (text mode) | `renderOperatorView` | direct call, `operator_output.go:85` | WIRED | Confirmed by reading the function body. |
| `renderOperatorView` | `viewFields` → `json.Marshal(doc)` | direct call chain | WIRED, single path | Confirmed — one `json.Marshal(` call site total in non-test files. |
| `operatorCommands()` (live cobra tree) | `TestOperatorViewFixturesCoverEveryOperatorCommand` | `commandKeySet(operatorCommands())` compared against `operatorViewFixtures()` | WIRED, both directions | Ran the test directly; 15/15 commands present, zero missing, zero extra. |

### Behavioral Spot-Checks / Test Execution

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full module build | `go build ./...` | exit 0 | ✓ PASS |
| Full module test suite (one run) | `go test ./...` | all packages ok, 0 failures | ✓ PASS |
| Identity gate, negative-case proofs, humanizer, empty/ordering/encoding/control-char edge probes | `go test ./cmd/engram/... -run 'TestOperatorView\|TestOrderedKeyDiff\|TestSetDiff\|TestOperatorDocsAreHandDeclared\|TestOperatorOutputParity\|TestHumanizeKey\|TestFlatViewIdentity\|TestMigrateViewIdentity\|TestSpineViewIdentity\|TestArchivePurgeViewIdentity' -v` | all subtests PASS | ✓ PASS |
| Bidirectional coverage + hand-declared-doc gate + full merged-fixture identity run | `go test ./cmd/engram/... -run 'TestOperatorViewIdentityAcrossEveryOperatorCommand\|TestOperatorViewFixturesCoverEveryOperatorCommand\|TestOperatorDocsAreHandDeclared' -v` | 15/15 commands PASS each gate | ✓ PASS |
| `gofmt -l` on touched view/output files | `gofmt -l cmd/engram/operator_view.go cmd/engram/operator_output.go` | empty output | ✓ PASS |
| SPDX headers present on new files | `rg -n 'SPDX-License-Identifier'` | present, line 1 | ✓ PASS |

### Anti-Patterns Found

None. Scanned every file in `git diff --name-only 6a0df053..HEAD -- cmd/engram docs-site` for `TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER|not yet implemented|coming soon` (case-insensitive) — zero matches across all 26 changed files.

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|--------------|-------------|--------|----------|
| REQ-operator-renderer-typed | 06-01 through 06-07 (all 7) | Operator command output derives text and json from one shared ordered field set; identity holds by construction (#481) | ✓ SATISFIED | See truths 1-6 above. REQUIREMENTS.md line 48 marked `[x]`, line 110 mapping table marked "Complete", consistent both directions — no orphan. |

No other requirement ID maps to Phase 6 in REQUIREMENTS.md; no plan in this phase declares any requirement ID other than REQ-operator-renderer-typed.

### Human Verification Required

None.

### Gaps Summary

No gaps. All three ROADMAP Success Criteria are verified against the actual codebase (not SUMMARY prose): the mechanism is genuinely construction-based (single `json.Marshal` call site, byte-walk not reflection), the identity gate is non-vacuous with committed negative-case proof, coverage is derived from the live cobra tree in both directions across all 15 commands, the obsolete parity gate is confirmed retired, the deferred red-evidence harness registration is confirmed untouched, and SC3 is backed by a genuine captured live-run transcript rather than a bare claim. One byte-order caveat on `spineScanReportDoc.Scope`'s placement is recorded above as a documented, reasoned, non-blocking deviation — not counted as a gap because JSON key order carries no consumer contract and no key was renamed/removed.

---

_Verified: 2026-08-17_
_Verifier: Claude (gsd-verifier)_
