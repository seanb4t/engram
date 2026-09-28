---
phase: 03-curation-rpcs-mcp-tools
verified: 2026-09-28T17:30:00Z
status: passed
score: 4/4 must-haves verified
covered_files: [".planning/REQUIREMENTS.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-01-PLAN.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-01-SUMMARY.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-02-PLAN.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-02-SUMMARY.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-03-PLAN.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-03-SUMMARY.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-04-PLAN.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-04-SUMMARY.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-05-PLAN.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-05-SUMMARY.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-06-PLAN.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-06-SUMMARY.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-07-PLAN.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-07-SUMMARY.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-REVIEW.md", ".planning/phases/03-curation-rpcs-mcp-tools/deferred-items.md", "docs-site/src/content/docs/reference/tools.md", "internal/e2e/boot_test.go", "internal/server/archive.go", "internal/server/archive_test.go", "internal/server/connectapi.go", "internal/server/connectapi_parity_test.go", "internal/server/connectapi_write_parity_test.go", "internal/server/connectcsrf.go", "internal/server/connectcsrf_lane_test.go", "internal/server/connectcsrf_test.go", "internal/server/connectdescriptor_test.go", "internal/server/fakestore_test.go", "internal/server/listrules_test.go", "internal/server/protoconv.go", "internal/server/registertools_test.go", "internal/server/related.go", "internal/server/related_test.go", "internal/server/rules.go", "internal/server/scheduled_test.go", "internal/server/store_iface.go", "internal/server/supersedepreview.go", "internal/server/supersedepreview_test.go", "internal/server/tags.go", "internal/server/tags_test.go", "internal/server/tools.go", "internal/store/relatedmemories.go", "internal/store/store.go", "internal/surfaces/toolclass.go", "internal/surfaces/toolclass_test.go", "proto/engram/v1/engram.proto"]
covered_digest: "v2:sha256:7e6c65b39b6541665bc4b425b09543245b56e81ec31c714791c834109c7b3c19"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: 4/4
  gaps_closed: []
  gaps_remaining: []
  regressions: []
---

# Phase 3: Curation RPCs & MCP Tools Verification Report

**Phase Goal:** Every new curation capability is reachable as a Connect RPC (and, where decided, an
MCP tool), correctly authz-scoped and CSRF-protected, before any UI touches it.

**Verified:** 2026-09-28
**Status:** passed
**Re-verification:** Yes — the prior VERIFICATION.md (2026-09-27) was stale: later milestone
phases (4, 5, and especially 6) edited shared files this phase's must-haves depend on
(`proto/engram/v1/engram.proto`, `internal/server/connectapi.go`, `internal/server/tools.go`, and
the CSRF read-exempt test list), most recently Phase 6 adding a new read RPC (`UnderstandQuery`,
last touched 2026-09-28T11:09). This report re-derives all four roadmap truths against current HEAD
(branch `feat/2026-09-25.01`) rather than trusting the prior report or any SUMMARY.md claim.

## Goal Achievement

### Observable Truths (Roadmap Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A positive test proves a request to each new mutating RPC without the double-submit CSRF token is rejected `permission_denied`, written before the handler | ✓ VERIFIED | `csrfWriteProcedures` (`internal/server/connectcsrf.go:36-46`) at current HEAD still contains exactly the original 6 write RPCs plus the 3 Phase-3 ones (`ArchiveMemory`, `RestoreMemory`, `SupersedeMemory`) — no more, no fewer; Phase 6's new `UnderstandQuery` RPC is a read and is correctly absent. Re-ran independently at HEAD: `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestCSRFCurationWritesRequireDoubleSubmit$' -v` → `--- PASS` (0.06s) with all 4 subtests (`ArchiveMemory`, `RestoreMemory`, `SupersedeMemory`, `SupersedeMemory_validate_only`) green. |
| 2 | Each new RPC delegates to the same core function its MCP tool (where one exists) calls, with the MCP-parity decision recorded explicitly | ✓ VERIFIED | Re-checked at current line numbers (shifted by Phase 6's `UnderstandQuery` insertion): `internal/server/connectapi.go` handlers for `ArchiveMemory` (:545-550), `RestoreMemory` (:557-562), `SupersedeMemory` (:576-581), `ListScheduled` (:595-600), `ListRules` (:623-629), `RelatedMemories` (:651-656), `ListTags` (:667-672) all still delegate to `a.d.*` core methods. `internal/server/tools.go`'s MCP closures (lines 3022-3204) still call the identical core methods (`d.listScheduled`, `d.supersede`, `d.archiveMemory`, `d.restoreMemory`, `d.listRules`→`listRuleRecords`, `d.relatedMemories`, `d.listTags`). Re-ran independently: `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestWriteParity$' -v` and `-run '^TestReadParity$' -v` → both `--- PASS` with every RPC subtest green (`ArchiveMemory`, `RestoreMemory`, `SupersedeMemory`, `ListScheduled`, `ListRules`, `RelatedMemories`, `ListTags`). |
| 3 | An invalid `SupersedeMemory` target set is rejected with every offending target named | ✓ VERIFIED | `internal/server/supersedepreview_test.go`'s `TestSupersedeMemoryConnectNamesEveryOffender` unchanged in substance. Re-ran independently at HEAD: `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestSupersedeMemoryConnectNamesEveryOffender$' -v` → `--- PASS` (1.42s), all 3 subtests (`addressability`, `rule`, `already_superseded`) green. |
| 4 | All proto changes additive (`buf breaking` green), gen trees regenerated/committed, vendored SPA passes `ui-drift`, each RPC carries blast-radius annotations and self-describe catalog entry | ✓ VERIFIED | Re-ran every check fresh at HEAD (after Phase 6's own proto/gen changes for `UnderstandQuery`): `go tool buf breaking --against '.git#branch=main'` → clean, exit 0. `task surfaces:gen && git diff --exit-code -- proto/ docs-site/ skill/ gen/ ui/src/lib/gen/ cmd/engram/testdata/` → zero diff (generated trees current and committed, including Phase 6's own additions). `ui-drift` reproduced independently into a scratch directory (not overwriting the committed tree): `cd ui && pnpm install --frozen-lockfile && pnpm build`, then `diff -rq <scratch-copy-of-ui/build> internal/webauth/static` → zero diff (vendored SPA byte-identical, current through Phase 4/5/6's own UI work). Blast-radius: `internal/surfaces/toolclass.go` still carries `Operation` rows for `archive_memory`, `restore_memory`, `related_memories`, `list_tags`; `TestOperationsCoverEveryTool` → `--- PASS`. Self-describe: `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` → `--- PASS` (still asserts `IDEMPOTENCY_UNKNOWN` on every method, including the 7 Phase-3 RPCs and Phase 6's `UnderstandQuery`). |

**Score:** 4/4 truths verified (0 present, behavior-unverified)

### Regression Check — Later-Phase Edits to Shared Files

The dispatch for this re-verification named four files edited by Phases 4-6 after the prior
verification ran. Each was re-checked directly against Phase 3's must-haves:

| File | Later-phase edit | Phase 3 regression check | Result |
|------|-------------------|---------------------------|--------|
| `proto/engram/v1/engram.proto` | Phase 6 appended `UnderstandQueryRequest`/`Response` + `rpc UnderstandQuery` (additive, after `ListTags`) | `buf breaking` still clean; Phase 3's 7 RPC declarations byte-unchanged in shape | ✓ No regression |
| `internal/server/connectapi.go` | Phase 6 added the `UnderstandQuery` handler (new method, Connect-only by design per its own doc comment) | Phase 3's 7 handlers (`ArchiveMemory` through `ListTags`) still present, still delegate to the same `a.d.*` cores, line numbers shifted only by the new method's insertion | ✓ No regression |
| `internal/server/connectcsrf_lane_test.go` / `connectcsrf_test.go` | Phase 6 added an `UnderstandQuery` case to `TestReadRPCsCSRFExempt` | `TestReadRPCsCSRFExempt` now has 10 read-RPC subtests including all 4 Phase-3 reads (`ListScheduled`, `ListRules`, `RelatedMemories`, `ListTags`) plus Phase 6's `UnderstandQuery`; `TestCSRFCurationWritesRequireDoubleSubmit`'s 3 Phase-3 write RPCs untouched | ✓ No regression — Phase 6's new read RPC was correctly exempted, never added to the write allowlist |
| `internal/server/tools.go` | Phase 6 added no new MCP tool (per its own comment, `UnderstandQuery` is "Connect-only by design") | Phase 3's 7 MCP closures (`archive_memory` through `list_tags`) unchanged, still call the same `d.*` cores | ✓ No regression |

No regression found. Phase 6's new read RPC was added correctly (exempted from CSRF, no MCP-tool
gap since it's a documented Connect-only capability outside Phase 3's RPC-01..06 scope) without
disturbing any Phase 3 must-have.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `proto/engram/v1/engram.proto` | 7 new RPCs appended after `ScheduleMemory` | ✓ VERIFIED | `rg` confirms `rpc ArchiveMemory`, `RestoreMemory`, `SupersedeMemory`, `ListScheduled`, `ListRules`, `RelatedMemories`, `ListTags` all still present; `buf breaking` clean at HEAD. |
| `internal/server/archive.go` | Shared batch core for archive/restore | ✓ VERIFIED | `archiveBatch`, `archiveMemory`, `restoreMemory` present and called from both `connectapi.go` and `tools.go`. |
| `internal/server/supersedepreview.go` | Shared `deps.supersede` dispatch (real vs validate_only) | ✓ VERIFIED | Present; called from both lanes. |
| `internal/server/rules.go`, `related.go`, `tags.go`, `scheduled` core in `tools.go` | Shared cores for ListRules/RelatedMemories/ListTags/ListScheduled | ✓ VERIFIED | All present; each Connect handler and MCP tool delegate to the same core function. |
| `internal/server/connectcsrf.go` | 3 new write Procedures in `csrfWriteProcedures`, new read Procedures (incl. Phase 6's `UnderstandQuery`) exempt | ✓ VERIFIED | `EngramServiceArchiveMemoryProcedure`, `RestoreMemoryProcedure`, `SupersedeMemoryProcedure` all `true`; all read RPCs absent from the write map, present in `TestReadRPCsCSRFExempt`. |
| `internal/surfaces/toolclass.go` | Blast-radius rows for 4 new MCP tools | ✓ VERIFIED | Rows for `archive_memory`, `restore_memory`, `related_memories`, `list_tags` present; `TestOperationsCoverEveryTool` passes. |
| `internal/webauth/static` (vendored SPA) | Rebuilt against regenerated client, zero drift | ✓ VERIFIED | Independent `ui-drift` reproduction into a scratch dir: zero diff against committed tree. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/server/connectapi.go` (ArchiveMemory/RestoreMemory handlers) | `internal/server/archive.go` | `a.d.archiveMemory(...)` / `a.d.restoreMemory(...)` | ✓ WIRED | Confirmed at `connectapi.go:550,562` (HEAD). |
| `internal/server/connectapi.go` (SupersedeMemory handler) | `internal/server/supersedepreview.go` | `a.d.supersede(...)` | ✓ WIRED | Confirmed at `connectapi.go:581` (HEAD). |
| `internal/server/connectapi.go` (ListScheduled/ListRules/RelatedMemories/ListTags handlers) | shared cores | `a.d.listScheduled`, `a.d.listRuleRecords`, `a.d.relatedMemories`, `a.d.listTags` | ✓ WIRED | Confirmed at `connectapi.go:600,629,656,672` (HEAD). |
| `internal/server/tools.go` (all 7 MCP closures) | same shared cores | `d.archiveMemory`, `d.restoreMemory`, `d.supersede`, `d.listScheduled`, `d.listRules`→`listRuleRecords`, `d.relatedMemories`, `d.listTags` | ✓ WIRED | Confirmed via grep across `tools.go:3022-3204` (HEAD; shifted from prior report's line numbers due to Phase 6's `UnderstandQuery` insertion, but unchanged in substance). |
| `internal/server/connectcsrf.go` (`csrfWriteProcedures`) | `gen/go/.../engram.connect.go` | generated Procedure constants | ✓ WIRED | Constants resolve; CSRF test passes against the live interceptor chain at HEAD. |

### Behavioral Spot-Checks

All commands re-run fresh in this verification, at current HEAD (commit `e8a1bc35`, branch
`feat/2026-09-25.01`), not copied from the prior report:

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| CSRF double-submit gate on the 3 mutating RPCs | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestCSRFCurationWritesRequireDoubleSubmit$' -v` | `--- PASS` (0.06s), 4 subtests green | ✓ PASS |
| Read RPCs (incl. Phase 6's new `UnderstandQuery`) remain CSRF-exempt | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestReadRPCsCSRFExempt$' -v` | `--- PASS` (0.65s), 10 subtests green incl. `ListScheduled`, `ListRules`, `RelatedMemories`, `ListTags`, `UnderstandQuery` | ✓ PASS |
| Every-offender naming (SupersedeMemory) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestSupersedeMemoryConnectNamesEveryOffender$' -v` | `--- PASS` (1.42s), 3 subtests green | ✓ PASS |
| MCP↔Connect write parity | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestWriteParity$' -v` | `--- PASS` (0.02s), `ArchiveMemory`/`RestoreMemory`/`SupersedeMemory` subtests green | ✓ PASS |
| MCP↔Connect read parity | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestReadParity$' -v` | `--- PASS` (0.00s), `ListScheduled`/`ListRules`/`RelatedMemories`/`ListTags` subtests green | ✓ PASS |
| Blast-radius coverage + tool registry enumerable | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ ./internal/surfaces/ -run '^(TestOperationsCoverEveryTool|TestRegisterToolsEnumerable)$' -v` | both `--- PASS` | ✓ PASS |
| Self-describe descriptor (`IDEMPOTENCY_UNKNOWN` on every method incl. new RPCs) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs$' -v` | `--- PASS` | ✓ PASS |
| `buf breaking` — proto changes additive (incl. Phase 6's own proto edits) | `go tool buf breaking --against '.git#branch=main'` | clean, exit 0 | ✓ PASS |
| Generated trees current (incl. Phase 4/5/6's own gen changes) | `task surfaces:gen && git diff --exit-code -- proto/ docs-site/ skill/ gen/ ui/src/lib/gen/ cmd/engram/testdata/` | zero diff | ✓ PASS |
| `ui-drift` gate (vendored SPA current through Phase 4/5/6 UI work) | `cd ui && pnpm install --frozen-lockfile && pnpm build`; diff scratch copy of `build/` vs `internal/webauth/static/` | zero diff | ✓ PASS |
| Full package suite for the phase (server, surfaces, store, keylinks) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... ./internal/surfaces/... ./internal/store/... ./internal/keylinks/... -count=1` | all `ok`, exit 0 (server 29.8s, surfaces 0.7s, store 190.3s, storetest 5.9s, keylinks 0.2s) | ✓ PASS |
| `go build ./...` | `go build ./...` | clean, exit 0 | ✓ PASS |
| No debt markers in phase core files | `rg -n "TBD\|FIXME\|XXX"` over the 15 new/core phase files (archive.go/_test, supersedepreview.go/_test, scheduled_test.go, rules.go, listrules_test.go, related.go/_test, tags.go/_test, engram.proto, connectapi.go, connectcsrf.go, tools.go) | no matches | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|-----------------|--------------|--------|----------|
| RPC-01 | 03-02, 03-07 | `SupersedeMemory` on Connect, `supersedes`+`idempotency_key`, shared core, every-offender rejection | ✓ SATISFIED | `SupersedeMemory` RPC present at HEAD; `TestSupersedeMemoryConnectNamesEveryOffender` passes. |
| RPC-02 | 03-01, 03-07 | `ArchiveMemory`/`RestoreMemory` on Connect + MCP tools, stamp/clear `archived_at`, reversible | ✓ SATISFIED | RPCs + tools present at HEAD; `TestWriteParity/ArchiveMemory` and `/RestoreMemory` subtests pass. |
| RPC-03 | 03-03, 03-04, 03-07 | `ListRules`/`ListScheduled` on Connect with MCP-equivalent contracts, widenings recorded | ✓ SATISFIED | Both RPCs present; `TestReadParity/ListScheduled` and `/ListRules` subtests pass, incl. cross_spine/cursor and all-scopes widening checks. |
| RPC-04 | 03-05, 03-06, 03-07 | `RelatedMemories`/`ListTags` on Connect + MCP tools, wrapping STORE-02/STORE-03 | ✓ SATISFIED | Both RPCs + tools present; `TestReadParity/RelatedMemories` and `/ListTags` subtests pass. |
| RPC-05 | all 7 plans | Every mutating Procedure CSRF-gated + `engramWrite`-routed, positive CSRF test per RPC, parity-test row per RPC | ✓ SATISFIED | Verified directly above; still true after Phase 6's own read-RPC addition. |
| RPC-06 | all 7 plans | Additive proto, gen trees regenerated/committed, `ui-drift` clean, blast-radius + self-describe per RPC | ✓ SATISFIED | Verified directly above; re-run fresh against HEAD's larger gen tree (post Phase 4/5/6). |

**Orphaned requirements:** none — `.planning/REQUIREMENTS.md`'s Traceability table maps only
RPC-01..06 to Phase 3, and all six appear in at least one plan's `requirements:` frontmatter
(confirmed by direct read of all 7 plan frontmatter blocks at HEAD).

**Tracking-defect note (not a phase gap, unchanged from prior report):**
`.planning/REQUIREMENTS.md`'s checkboxes for RPC-01..06 remain `[ ]` and the Traceability table's
`Status` column reads `Mapped` rather than `Complete`. This is a documented, milestone-wide,
pre-existing tool defect (`gsd-tools requirements mark-complete` only transitions a row from
`Pending`/`Gaps Found` to `Complete`; this milestone's table was authored with every row already at
`Mapped`) — see `.planning/phases/03-curation-rpcs-mcp-tools/deferred-items.md`. Requirement
satisfaction above is judged from code and passing tests at current HEAD, not from checkbox state.

### Anti-Patterns Found

None new. No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers found via `rg` in the phase's
core/new files at HEAD. The same two info-level findings from the phase's own code review
(`03-REVIEW.md`) were re-checked directly against current line numbers and are unchanged — both
cosmetic, non-blocking, and not touched by any later-phase edit:

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/server/registertools_test.go` | 75 | Stale doc-comment count ("17-tool set" vs. the actual registered tool count) | ℹ️ Info | Prose-only; the test's actual assertion is unaffected. |
| `internal/server/tools.go` | 3122 | `supersede_memory` MCP closure checks `out.Validated` before checking `err` | ℹ️ Info | Pre-existing pattern shared with the unmodified `store_memory` closure; cosmetic result text on an error path only. |

### Human Verification Required

None. This phase ships no UI (D-28); every truth above was re-verified by an automated test run
independently in this session or a direct codebase check at current HEAD, not by re-reading
SUMMARY.md or the prior VERIFICATION.md's claims. No item requires manual/mechanical UAT.

### Gaps Summary

No gaps. Re-verification confirms all four roadmap success criteria still hold at current HEAD
(commit `e8a1bc35`, branch `feat/2026-09-25.01`) despite three later milestone phases (4, 5, 6)
editing shared files this phase's must-haves depend on. Specifically: Phase 6's addition of the
`UnderstandQuery` RPC (a new read-only Connect-only RPC, outside Phase 3's RPC-01..06 scope) did not
disturb the CSRF write allowlist, the MCP-parity wiring for any of the 7 Phase-3 RPCs, the
every-offender SupersedeMemory rejection, or the buf-breaking/gen-tree/ui-drift/blast-radius/
self-describe chain — each was re-run fresh and independently confirmed clean. The full test suite
for every package this phase touches (`internal/server`, `internal/surfaces`, `internal/store`,
`internal/keylinks`) was re-run in full (not filtered) and passed with exit 0. The only outstanding
item — `REQUIREMENTS.md` checkboxes not flipping — is the same documented, pre-existing,
milestone-wide tool defect noted in the prior report, unrelated to this phase's implementation.

---

_Verified: 2026-09-28_
_Verifier: Claude (gsd-verifier)_
