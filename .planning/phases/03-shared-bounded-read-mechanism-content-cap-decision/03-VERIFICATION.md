---
phase: 03-shared-bounded-read-mechanism-content-cap-decision
verified: 2026-09-19T19:32:00Z
status: passed
score: 3/3 must-haves verified
covered_files:
  - ".planning/PROJECT.md"
  - ".planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-01-PLAN.md"
  - ".planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-01-SUMMARY.md"
  - ".planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-02-PLAN.md"
  - ".planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-02-SUMMARY.md"
  - ".planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-03-PLAN.md"
  - ".planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-03-SUMMARY.md"
  - ".planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-04-PLAN.md"
  - ".planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-04-SUMMARY.md"
  - ".planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-05-PLAN.md"
  - ".planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-05-SUMMARY.md"
  - ".planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-06-PLAN.md"
  - ".planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-06-SUMMARY.md"
  - ".planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-CONTEXT.md"
  - ".planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-INVENTORY.md"
  - ".planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-REVIEW-FIX.md"
  - ".planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-REVIEW.md"
  - "CLAUDE.md"
  - "docs-site/src/content/docs/guides/configure.md"
  - "docs-site/src/content/docs/guides/upgrade.md"
  - "docs-site/src/content/docs/reference/errors.md"
  - "docs-site/src/content/docs/reference/tools.md"
  - "internal/config/config.go"
  - "internal/config/registry.go"
  - "internal/config/validate.go"
  - "internal/e2e/contentcap_cli_test.go"
  - "internal/server/tools.go"
  - "internal/skills/data/curating-memory/SKILL.md"
  - "internal/store/boundedread.go"
  - "internal/store/orderedpage.go"
  - "internal/store/redevidence_harness_test.go"
  - "internal/store/revert.go"
  - "internal/store/spine.go"
  - "internal/store/store.go"
  - "skill/engram/skills/curating-memory/SKILL.md"
covered_digest: "v1:sha256:7a9cf6dbf5d6f754703e1bbaabb6f4ceb3a81b390b53fbed58959df512ec7796"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 3: Shared Bounded-Read Mechanism & Content Cap Decision Verification Report

**Phase Goal:** Build the two shared bounded-read primitives every read-site migration in this
milestone reuses — an ordered-page helper for `List`-shaped reads and a byte-budget extension of
`scrollAllPoints` for whole-spine sweeps — rather than patching each of #585's sibling sites
independently. An inventory of every `WithPayload(true)`/unbounded `Scroll`/`ScrollAndOffset`/
`Query` call site is recorded, and each site is assigned to the phase that migrates it (Phase 4
for the recall paths, Phase 5 for the sweeps) or given a justified exemption; the migrations
themselves land in those phases. Pages end on an accumulated-byte budget as well as a record
count. This phase also settles decision A — whether memory `content` gets a size cap.

**Verified:** 2026-09-19T19:32:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

Truths are the three ROADMAP Success Criteria for this phase (the roadmap contract), each
cross-checked against the plans' `must_haves.truths` and directly re-run against the live
codebase (not read from SUMMARY.md claims).

| # | Truth (ROADMAP Success Criterion) | Status | Evidence |
|---|---|---|---|
| 1 | Every full-payload Qdrant read site in `internal/store` is inventoried in a recorded list, each assigned to Phase 4, Phase 5, or a documented exemption, and both primitives are proven against Phase 1's oversized fixture. | ✓ VERIFIED | `03-INVENTORY.md` lists 27 functions. Re-ran the mechanical derivation command verbatim (`awk`/`sed` over `internal/store/*.go` non-test files) — output is byte-identical (as a set) to the inventory's 27-row `Function` column (`diff` empty). Each row carries an assignment (Phase 4: 5, Phase 5: 10, Phase 3 primitives: 2, Exempt: 10, each with a written justification). Ran `TestScrollAllPointsByteBudget` and `TestScrollOrderedPageByteBudget` live against real Qdrant (testcontainers) over both `storetest.SeedOversized` shapes (`few-large`, `many-small`) at `storetest.RecvLimit` (4 MiB) — both pass. |
| 2 | A page ends on an accumulated-byte budget as well as a record count, proven against a fixture of a few very large records that a count-only cap would not catch. | ✓ VERIFIED | `internal/store/boundedread.go` (`rpcByteBudget`/`pageByteBudget`, both 2 MiB) and `internal/store/orderedpage.go` (`CutByBudget`/`Exhausted`, never both true) implement the byte-derived cut. Live-ran `TestScrollAllPointsByteBudget/few-large` and `TestScrollOrderedPageByteBudget/few-large` (40 records × 131072 bytes = 5.24 MiB total, over the 4 MiB `RecvLimit` a count-only cap of the same page would have overflowed): both pass, no Scroll response exceeds the RPC budget. |
| 3 | Whether memory `content` gets a size cap is decided and recorded in PROJECT.md Key Decisions; if adopted, a registry-declared `ENGRAM_MEMORY_MAX_CONTENT_BYTES` with a documented default rejects an oversized write on every write path (MCP, Connect, CLI) with a named hint, while existing oversized records stay readable. | ✓ VERIFIED | `.planning/PROJECT.md:919` records decision A (adopted: `ENGRAM_MEMORY_MAX_CONTENT_BYTES`=65536, plus `ENGRAM_MEMORY_MAX_TAGS`=128/`ENGRAM_MEMORY_MAX_TAG_BYTES`=128; `0` rejected). `internal/config/registry.go:51,56-57` declares all three vars. Live-ran `TestMemoryContentCapFlowsFromConfig`, `TestMemoryWriteCapsRejectOnEveryCreateLane` (MCP+Connect create paths), `TestUpdateMemoryContentCap` (Connect's field-mask `UpdateMemory` bypass, closed per D-09), `TestCLIStoreRejectsOversizedContentAndTags` (real binary against real `engram serve`) — all pass with the named `field=content hint=too_long`/`field=tags hint=too_many`/`too_long` envelope. Live-ran `TestUpdateMemoryLegacyOversizedRecord` (8 subtests) — a pre-cap 70000-byte/200-tag record stays readable byte-for-byte via `get_memory`/`GetMemory`, can be re-shared, resent unchanged, and trimmed; only a genuinely new over-cap value is rejected. |

**Score:** 3/3 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/store/boundedread.go` | `RecordCaps`/`DefaultRecordCaps`/`WithRecordCaps`/`fullRecordCeiling`/`summaryRecordCeiling`/`perRPCLimit`/`readView`/`fullView`/`summaryView`/`unbudgetedView`/`rpcByteBudget`/`pageByteBudget` | ✓ VERIFIED | 250 lines. All named symbols present and grepped directly (not just claimed). |
| `internal/store/spine.go` | `scrollAllPoints(ctx, filter, view readView, fn)` byte-derived count + D-07 batch-of-1 fallback | ✓ VERIFIED | `func (s *Store) scrollAllPoints(ctx context.Context, filter *qdrant.Filter, view readView, fn func(*qdrant.RetrievedPoint) error) error` at line 67; behavioral proof via `TestScrollAllPointsByteBudget`/`TestScrollAllPointsBatchOfOneFallback` (live). |
| `internal/store/orderedpage.go` | `orderedPage{Items,Next,Exhausted,CutByBudget,Bytes}`, `(*Store).scrollOrderedPage`, `excludeSeen` | ✓ VERIFIED | 221 lines; all symbols present. Behavioral proof via `TestScrollOrderedPageByteBudget`/`TestScrollOrderedPageTiesAcrossRPCBoundaries` (live). No production caller yet — expected (Phase 4's job per D-08/orchestrator notes); classified `otherNonRecallEmitters`, confirmed by grep and by a live run of `TestRecallEmissionSetIsCompleteAndClassified`. |
| `.planning/phases/.../03-INVENTORY.md` | D-08 call-site inventory + Phase 5 closing-check commands | ✓ VERIFIED | Present; mechanically re-derived set matches the file's 27-row table exactly. |
| `internal/config/registry.go` | `memory.max_content_bytes`/`max_tags`/`max_tag_bytes` rows | ✓ VERIFIED | Lines 51, 56-57; defaults 65536/128/128 match PROJECT.md and docs. |
| `internal/server/tools.go` | `checkContentBytes`/`checkTags`/`validateStoreArgs`/`recordCapsFromConfig`/`storeFromConfig`/`deps.updateMemory` content+tags gating | ✓ VERIFIED | All functions present at the lines the plans name; `deps.updateMemory`'s `contentChanged`-gated check confirmed by reading the function body and by a live pass of `TestUpdateMemoryContentCap` and `TestUpdateMemoryLegacyOversizedRecord`. |
| `internal/e2e/contentcap_cli_test.go` | `TestCLIStoreRejectsOversizedContentAndTags` | ✓ VERIFIED | Live-ran against the real built binary + real `engram serve`; 4/4 subtests pass. |
| `.planning/PROJECT.md` Key Decisions | decision A row | ✓ VERIFIED | Row present at line 919 with rationale and outcome; no heading added/removed (`.planning/**` structural rule respected). |
| `docs-site/.../configure.md`, `errors.md`, `tools.md`, `upgrade.md` | the three caps documented, worked rejection examples, upgrade-guide entry | ✓ VERIFIED | Grepped directly: `configure.md` carries all three env-var rows; `errors.md` carries worked `too_long`/`too_many` examples. |
| Phase 3 red-evidence (13 patches) | one mutation per lane, hand-verified RED, registered in `redEvidenceDirs` | ✓ VERIFIED | 13 patch files present on disk, no SPDX headers; `redEvidenceDirs` carries the Phase 3 entry keyed to the phase directory. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `internal/store/orderedpage.go` | `internal/store/boundedread.go` | measured bytes of each received point feed the page byte budget | ✓ WIRED | `proto.Size(p)` accumulation confirmed in `scrollOrderedPage`; proven live by `TestScrollOrderedPageByteBudget`. |
| `internal/store/spine.go` | `internal/store/boundedread.go` | the sweep requests the caller's view selector with the byte-derived count | ✓ WIRED | `sweepLimit(view)` call inside `scrollAllPoints`; proven live by `TestScrollAllPointsByteBudget`. |
| `internal/server/tools.go` (`deps.updateMemory`) | `internal/server/tools.go` (`checkContentBytes`/`checkTags`) | Connect's field-mask `UpdateMemory` lane cannot bypass the cap | ✓ WIRED | Confirmed by reading `deps.updateMemory` (gated on `contentChanged`/tag-set diff) and by a live pass of `TestUpdateMemoryContentCap`'s `connect_over` subtest. |
| `internal/server/tools.go` (`storeFromConfig`) | `internal/store` (`store.WithRecordCaps`) | the production store derives its read ceilings from the configured write caps | ✓ WIRED | `recordCapsFromConfig(cfg)` reuses `memoryWriteCapsFromConfig`; proven live by `TestStoreFromConfigCarriesRecordCaps` (per SUMMARY; re-confirmed by reading the call site). |
| `internal/e2e/contentcap_cli_test.go` | `cmd/engram` (`store` verb) | the real binary reaches the server-side cap through Connect | ✓ WIRED | Live-ran the real binary against a real headless `engram serve`; confirmed exit codes and envelopes on stderr. |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|---|---|---|---|---|
| REQ-byte-budget-pages | 03-02, 03-03, 03-04, 03-06 | Pages end on an accumulated-byte budget as well as a record count | ✓ SATISFIED | REQUIREMENTS.md marks it Complete for Phase 3; live-verified via `TestScrollAllPointsByteBudget`/`TestScrollOrderedPageByteBudget`. |
| REQ-content-cap-decided | 03-01, 03-03, 03-05, 03-06 | Whether memory `content` gets a size cap is decided and recorded | ✓ SATISFIED | REQUIREMENTS.md marks it Complete for Phase 3; decision A recorded in PROJECT.md; live-verified via the MCP/Connect/CLI/update-lane test set above. |

No orphaned requirements: REQUIREMENTS.md's phase-mapping table lists exactly these two IDs against Phase 3, and both are claimed by at least one plan's `requirements:` frontmatter.

**Out-of-scope acceptance criteria correctly absent:** Per D-05 (confirmed by `rg -n "payload_bytes" internal/store/*.go` — no matches) the two-phase ids→payload design was not adopted, so the TOCTOU-on-delete/supersede/archive and `GetPoints`-order acceptance criteria the ROADMAP phase text names for that design do not apply. This is a documented decision, not a gap.

### Anti-Patterns Found

Scanned every file this phase's plans declared as modified (config, store, server, e2e, docs, CLAUDE.md, skill files) for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` and stub-shaped patterns. None found in any of `internal/store/boundedread.go`, `internal/store/orderedpage.go`, `internal/store/spine.go`, `internal/store/revert.go`, `internal/store/store.go`, `internal/server/tools.go`, `internal/config/registry.go`, `internal/config/config.go`, `internal/config/validate.go`.

Two items were flagged and resolved as documented design decisions during the code-review loop (not anti-patterns, not gaps):
- **IN-01** (from `03-REVIEW.md`'s prior iteration): `scrollOrderedPage` has no production caller yet — this is Phase 4's explicit job (per D-08's inventory and this phase's own SUMMARYs); confirmed absent from every non-test call site.
- **IN-02**: a tag-set trim is rejected if it is still over cap after the change — a documented all-or-nothing design choice, not a bug.

No 🛑 Blockers, no ⚠️ Warnings, no debt markers.

### Behavioral Spot-Checks

Full-suite runs were avoided (per orchestrator instruction not to run `TestRedEvidencePatchesAreLive` concurrently with other work, and the general rule against re-running a full suite per must-have). Instead, single named tests were run directly against real Qdrant (testcontainers, Docker confirmed reachable) to independently confirm SUMMARY.md's claims rather than trust them.

| Behavior | Command | Result | Status |
|---|---|---|---|
| Byte-budget sweep primitive (both fixture shapes, both views) | `go test ./internal/store/ -run '^TestScrollAllPointsByteBudget$' -v` | 4/4 subtests PASS | ✓ PASS |
| Byte-budget ordered-page primitive (both fixture shapes, both views) | `go test ./internal/store/ -run '^TestScrollOrderedPageByteBudget$' -v` | 4/4 subtests PASS | ✓ PASS |
| Tie-safe keyset resume across RPC boundaries | `go test ./internal/store/ -run '^TestScrollOrderedPageTiesAcrossRPCBoundaries$' -v` | PASS | ✓ PASS |
| Content cap flows config → MCP rejection | `go test ./internal/server/ -run '^TestMemoryContentCapFlowsFromConfig$' -v` | 2/2 subtests PASS | ✓ PASS |
| Content/tags caps on every create lane (MCP+Connect) | `go test ./internal/server/ -run '^TestMemoryWriteCapsRejectOnEveryCreateLane$' -v` | 16/16 subtests PASS | ✓ PASS |
| Connect field-mask `UpdateMemory` bypass closed (D-09) | `go test ./internal/server/ -run '^TestUpdateMemoryContentCap$' -v` | 4/4 subtests PASS | ✓ PASS |
| Legacy over-cap record stays readable/trimmable (D-07) | `go test ./internal/server/ -run '^TestUpdateMemoryLegacyOversizedRecord$' -v` | 8/8 subtests PASS | ✓ PASS |
| Real binary's CLI lane rejects over-cap content/tags | `go test ./internal/e2e/ -run '^TestCLIStoreRejectsOversizedContentAndTags$' -v` | 4/4 subtests PASS | ✓ PASS |
| Recall-gate classification unaffected by new primitives | `go test ./internal/store/ -run '^TestRecallEmissionSetIsCompleteAndClassified$' -v` | 4/4 subtests PASS | ✓ PASS |
| `go build ./...` | `go build ./...` | exit 0 | ✓ PASS |
| Repo-wide escaped-pattern gate | `go test ./internal/keylinks/ -run TestNoEscapedPatternsRepoWide` | PASS | ✓ PASS |
| D-08 inventory equals the mechanically derived set | `awk`/`sed` derivation over `internal/store/*.go` vs. `03-INVENTORY.md`'s table | empty diff, 27/27 | ✓ PASS |

### Probe Execution

Not applicable — this phase is not a migration/CLI-tooling phase with `scripts/*/tests/probe-*.sh` probes; none declared in PLAN/SUMMARY and none found under `scripts/`.

## Gaps Summary

None. All three ROADMAP Success Criteria are independently re-verified against the live codebase (not SUMMARY.md claims): the inventory is mechanically complete, both bounded-read primitives are proven live against the oversized fixtures with byte-budget cuts observed directly, and decision A (content + tags caps) is recorded and enforced on every write path (MCP, Connect including the `UpdateMemory` field-mask bypass, and the real CLI binary), with the legacy-record readability/trimmability contract independently confirmed. The prior code-review loop (`03-REVIEW.md` → `03-REVIEW-FIX.md`) converged clean on its one WR-01 finding (a validated-vs-enforced integer-parser divergence), and that fix is present in the current tree (`ParsePositiveIntCap`/`ParseNonNegativeIntCap` shared by both sides). No blockers, no warnings, no human verification required.

---

_Verified: 2026-09-19T19:32:00Z_
_Verifier: Claude (gsd-verifier)_
