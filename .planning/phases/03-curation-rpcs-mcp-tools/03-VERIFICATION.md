---
phase: 03-curation-rpcs-mcp-tools
verified: 2026-09-27T00:00:00Z
status: passed
score: 4/4 must-haves verified
covered_files: [".planning/REQUIREMENTS.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-01-PLAN.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-01-SUMMARY.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-02-PLAN.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-02-SUMMARY.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-03-PLAN.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-03-SUMMARY.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-04-PLAN.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-04-SUMMARY.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-05-PLAN.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-05-SUMMARY.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-06-PLAN.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-06-SUMMARY.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-07-PLAN.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-07-SUMMARY.md", ".planning/phases/03-curation-rpcs-mcp-tools/03-REVIEW.md", ".planning/phases/03-curation-rpcs-mcp-tools/deferred-items.md", "docs-site/src/content/docs/reference/tools.md", "internal/e2e/boot_test.go", "internal/server/archive.go", "internal/server/archive_test.go", "internal/server/connectapi.go", "internal/server/connectapi_parity_test.go", "internal/server/connectapi_write_parity_test.go", "internal/server/connectcsrf.go", "internal/server/connectcsrf_lane_test.go", "internal/server/connectcsrf_test.go", "internal/server/connectdescriptor_test.go", "internal/server/fakestore_test.go", "internal/server/listrules_test.go", "internal/server/protoconv.go", "internal/server/registertools_test.go", "internal/server/related.go", "internal/server/related_test.go", "internal/server/rules.go", "internal/server/scheduled_test.go", "internal/server/store_iface.go", "internal/server/supersedepreview.go", "internal/server/supersedepreview_test.go", "internal/server/tags.go", "internal/server/tags_test.go", "internal/server/tools.go", "internal/store/relatedmemories.go", "internal/store/store.go", "internal/surfaces/toolclass.go", "internal/surfaces/toolclass_test.go", "proto/engram/v1/engram.proto"]
covered_digest: "v1:sha256:ce99dfa9a051e026a31c5523f34cad050689b90d213a17b5d00f874500fd9967"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 3: Curation RPCs & MCP Tools Verification Report

**Phase Goal:** Every new curation capability is reachable as a Connect RPC (and, where decided, an
MCP tool), correctly authz-scoped and CSRF-protected, before any UI touches it.

**Verified:** 2026-09-27
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (Roadmap Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A positive test proves a request to each new mutating RPC without the double-submit CSRF token is rejected `permission_denied`, written before the handler | ✓ VERIFIED | `TestCSRFCurationWritesRequireDoubleSubmit` (`internal/server/connectcsrf_lane_test.go`) has explicit subtests `ArchiveMemory`, `RestoreMemory`, `SupersedeMemory`, `SupersedeMemory_validate_only` — each asserts no-cookie/no-header, cookie-only, and empty-payload-no-token all return `connect.CodePermissionDenied` before validation runs. Ran independently: `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestCSRFCurationWritesRequireDoubleSubmit$' -v` → `--- PASS` (0.01s). All three mutating RPCs (`ArchiveMemory`, `RestoreMemory`, `SupersedeMemory`) are the only new entries added to `csrfWriteProcedures` (`internal/server/connectcsrf.go:43-45`); the four new read RPCs (`ListScheduled`, `ListRules`, `RelatedMemories`, `ListTags`) are correctly absent from it and instead appear in `TestReadRPCsCSRFExempt`. D-27 non-vacuity evidence (each SUMMARY documents removing the Procedure from the allowlist and observing red) corroborates the test is load-bearing, not vacuous. |
| 2 | Each new RPC delegates to the same core function its MCP tool (where one exists) calls, with the MCP-parity decision recorded explicitly | ✓ VERIFIED | `rg` over `internal/server/connectapi.go` shows every handler delegates to a `a.d.*` core method (`archiveMemory`, `restoreMemory`, `supersede`, `listScheduled`, `listRuleRecords`, `relatedMemories`, `listTags`); `internal/server/tools.go`'s MCP closures call the matching `d.*` core (`archiveMemory`, `restoreMemory`, `supersede`, `listScheduled`, `listRules` — which itself calls `listRuleRecords` — `relatedMemories`, `listTags`). All seven RPCs have an MCP tool counterpart (`archive_memory`, `restore_memory`, `supersede_memory`, `list_scheduled`, `list_rules`, `related_memories`, `list_tags`) — no Connect-only asymmetry exists in this phase, so there is no undocumented gap. `TestWriteParity`/`TestReadParity` (both ran green independently) assert cross-lane behavioral parity for every RPC. |
| 3 | An invalid `SupersedeMemory` target set is rejected with every offending target named | ✓ VERIFIED | `TestSupersedeMemoryConnectNamesEveryOffender` (`internal/server/supersedepreview_test.go`) seeds addressability, rule, and already-superseded failure classes and asserts every offending target is named in the rejection, identically across Connect/MCP and real/validate_only. Ran independently: `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestSupersedeMemoryConnectNamesEveryOffender$' -v` → `--- PASS` (0.05s). |
| 4 | All proto changes additive (`buf breaking` green), gen trees regenerated/committed, vendored SPA passes `ui-drift`, each RPC carries blast-radius annotations and self-describe catalog entry | ✓ VERIFIED | `go tool buf breaking --against '.git#branch=main'` → clean (no output, exit 0). `task surfaces:gen && git diff --exit-code -- proto/ docs-site/ skill/ gen/ ui/src/lib/gen/ cmd/engram/testdata/` → zero diff (generated trees current and committed). Reproduced the CI `ui-drift` job locally: `cd ui && pnpm install --frozen-lockfile && pnpm build`, then `rm -rf internal/webauth/static && mkdir -p internal/webauth/static && cp -R ui/build/. internal/webauth/static/`, then `git diff --exit-code internal/webauth/static/` → zero diff (byte-identical to the committed vendored SPA). Blast-radius annotations: `internal/surfaces/toolclass.go` carries `Operation` rows for `archive_memory`, `restore_memory`, `related_memories`, `list_tags` (grep confirmed); `TestOperationsCoverEveryTool` passes. Self-describe catalog: `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` (asserts `IDEMPOTENCY_UNKNOWN` on every method, including the 7 new ones) ran independently → `--- PASS`. |

**Score:** 4/4 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `proto/engram/v1/engram.proto` | 7 new RPCs appended after `ScheduleMemory` | ✓ VERIFIED | `rg` confirms `rpc ArchiveMemory`, `RestoreMemory`, `SupersedeMemory`, `ListScheduled`, `ListRules`, `RelatedMemories`, `ListTags` all present, additive (buf breaking clean). |
| `internal/server/archive.go` | Shared batch core for archive/restore | ✓ VERIFIED | `archiveBatch`, `archiveMemory`, `restoreMemory` present and called from both `connectapi.go` and `tools.go`. |
| `internal/server/supersedepreview.go` | Shared `deps.supersede` dispatch (real vs validate_only) | ✓ VERIFIED | Present; called from both lanes; `validateSupersede` reuses the real call's staged preflight functions (no parallel copy). |
| `internal/server/rules.go`, `related.go`, `tags.go`, `scheduled` core in `tools.go` | Shared cores for ListRules/RelatedMemories/ListTags/ListScheduled | ✓ VERIFIED | All present; each Connect handler and MCP tool delegate to the same core function (grep-confirmed above). |
| `internal/server/connectcsrf.go` | 3 new write Procedures in `csrfWriteProcedures`, 4 new read Procedures exempt | ✓ VERIFIED | `EngramServiceArchiveMemoryProcedure`, `RestoreMemoryProcedure`, `SupersedeMemoryProcedure` all `true`; the 4 read RPCs absent from the write map, present in `TestReadRPCsCSRFExempt`. |
| `internal/surfaces/toolclass.go` | Blast-radius rows for 4 new MCP tools | ✓ VERIFIED | Rows for `archive_memory`, `restore_memory`, `related_memories`, `list_tags` present; `TestOperationsCoverEveryTool` passes. |
| `internal/webauth/static` (vendored SPA) | Rebuilt against regenerated client, zero drift | ✓ VERIFIED | Local `ui-drift` reproduction: zero diff after rebuild. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/server/connectapi.go` (ArchiveMemory/RestoreMemory handlers) | `internal/server/archive.go` | `a.d.archiveMemory(...)` / `a.d.restoreMemory(...)` | ✓ WIRED | Confirmed at `connectapi.go:550,562`. |
| `internal/server/connectapi.go` (SupersedeMemory handler) | `internal/server/supersedepreview.go` | `a.d.supersede(...)` | ✓ WIRED | Confirmed at `connectapi.go:581`. |
| `internal/server/connectapi.go` (ListScheduled/ListRules/RelatedMemories/ListTags handlers) | shared cores | `a.d.listScheduled`, `a.d.listRuleRecords`, `a.d.relatedMemories`, `a.d.listTags` | ✓ WIRED | Confirmed at `connectapi.go:600,629,656,672`. |
| `internal/server/tools.go` (all 7 MCP closures) | same shared cores | `d.archiveMemory`, `d.restoreMemory`, `d.supersede`, `d.listScheduled`, `d.listRules`→`listRuleRecords`, `d.relatedMemories`, `d.listTags` | ✓ WIRED | Confirmed via grep across `tools.go:2994-3176`. |
| `internal/server/connectcsrf.go` (`csrfWriteProcedures`) | `gen/go/.../engram.connect.go` | generated Procedure constants | ✓ WIRED | Constants resolve; CSRF test passes against the live interceptor chain. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| CSRF double-submit gate on the 3 mutating RPCs | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestCSRFCurationWritesRequireDoubleSubmit$' -v` | `--- PASS` (0.01s) | ✓ PASS |
| Owner-gate through both lanes (Archive/Restore) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestArchiveMemoryOwnerGate$' -v` | `--- PASS` (0.71s) | ✓ PASS |
| Every-offender naming (SupersedeMemory) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestSupersedeMemoryConnectNamesEveryOffender$' -v` | `--- PASS` (0.05s) | ✓ PASS |
| MCP↔Connect write/read parity | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ ./internal/surfaces/ -run '^(TestOperationsCoverEveryTool|TestRegisterToolsEnumerable|TestWriteParity|TestReadParity)$' -v` | all `--- PASS` | ✓ PASS |
| Self-describe descriptor (`IDEMPOTENCY_UNKNOWN` on every method incl. new RPCs) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs$' -v` | `--- PASS` | ✓ PASS |
| `buf breaking` — proto changes additive | `go tool buf breaking --against '.git#branch=main'` | clean, exit 0 | ✓ PASS |
| Generated trees current | `task surfaces:gen && git diff --exit-code -- proto/ docs-site/ skill/ gen/ ui/src/lib/gen/ cmd/engram/testdata/` | zero diff | ✓ PASS |
| `ui-drift` gate (vendored SPA current) | `cd ui && pnpm build`; `cp -R build/. ../internal/webauth/static/`; `git diff --exit-code internal/webauth/static/` | zero diff | ✓ PASS |
| Full package suite for the phase (server, surfaces, store, keylinks) | `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... ./internal/surfaces/... ./internal/store/... ./internal/keylinks/... -count=1` | all `ok`, exit 0 | ✓ PASS |
| No debt markers in phase core files | `rg -n "TBD|FIXME|XXX"` over the 11 new/core phase files | no matches | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|-----------------|--------------|--------|----------|
| RPC-01 | 03-02, 03-07 | `SupersedeMemory` on Connect, `supersedes`+`idempotency_key`, shared core, every-offender rejection | ✓ SATISFIED | `SupersedeMemory` RPC present; `TestSupersedeMemoryConnectNamesEveryOffender` passes; `03-02-SUMMARY.md` coverage D1-D5 all `status: pass`. |
| RPC-02 | 03-01, 03-07 | `ArchiveMemory`/`RestoreMemory` on Connect + MCP tools, stamp/clear `archived_at`, reversible | ✓ SATISFIED | RPCs + tools present; `TestArchiveMemoryConnectRoundTrip`, `TestArchiveMemoryOwnerGate`, `TestArchiveMemoryBatchOutcomes` pass; `03-01-SUMMARY.md` coverage D1-D5 all `status: pass`. |
| RPC-03 | 03-03, 03-04, 03-07 | `ListRules`/`ListScheduled` on Connect with MCP-equivalent contracts, widenings recorded | ✓ SATISFIED | Both RPCs present; all-scopes widening and cross_spine/cursor widening explicitly recorded in 03-01's Task 1 decision gate and shipped per 03-03/03-04 SUMMARYs; `TestListScheduledConnectCrossSpinePages`, `TestListRulesConnectAllScopes` pass. |
| RPC-04 | 03-05, 03-06, 03-07 | `RelatedMemories`/`ListTags` on Connect + MCP tools, wrapping STORE-02/STORE-03 | ✓ SATISFIED | Both RPCs + tools present; `TestRelatedMemoriesConnectRoundTrip`, `TestListTagsRoundTripBothLanes` pass. |
| RPC-05 | all 7 plans | Every mutating Procedure CSRF-gated + `engramWrite`-routed, positive CSRF test per RPC, parity-test row per RPC | ✓ SATISFIED | Verified directly above (CSRF allowlist, `TestCSRFCurationWritesRequireDoubleSubmit`, `TestWriteParity`/`TestReadParity` rows for all 7 RPCs). |
| RPC-06 | all 7 plans | Additive proto, gen trees regenerated/committed, `ui-drift` clean, blast-radius + self-describe per RPC | ✓ SATISFIED | Verified directly above (`buf breaking`, `task surfaces:gen` zero-diff, local `ui-drift` reproduction zero-diff, `TestOperationsCoverEveryTool`, descriptor test). |

**Orphaned requirements:** none — `.planning/REQUIREMENTS.md`'s Traceability table maps only RPC-01..06 to Phase 3, and all six appear in at least one plan's `requirements:` frontmatter.

**Tracking-defect note (not a phase gap):** `.planning/REQUIREMENTS.md`'s checkboxes for RPC-01..06 remain `[ ]` and the Traceability table's `Status` column reads `Mapped` rather than `Complete`. This is a documented, milestone-wide, pre-existing tool defect (`gsd-tools requirements mark-complete` only transitions a row from `Pending`/`Gaps Found` to `Complete`; this milestone's table was authored with every row already at `Mapped`, a different vocabulary state the tool's state machine never reaches) — see `.planning/phases/03-curation-rpcs-mcp-tools/deferred-items.md` and `WINDOWS.md` entry #19, and the identical prior report for `STORE-01` (a completed, shipped Phase-1 requirement) proving this is not specific to this phase's execution. Requirement satisfaction above is judged from code, passing tests, and each plan's `requirements-completed:`/`coverage:` SUMMARY frontmatter, not from the checkbox state.

### Anti-Patterns Found

None in the phase's core/new files (`archive.go`, `archive_test.go`, `supersedepreview.go`, `supersedepreview_test.go`, `scheduled_test.go`, `rules.go`, `listrules_test.go`, `related.go`, `related_test.go`, `tags.go`, `tags_test.go`, `proto/engram/v1/engram.proto`): no `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers found via `rg`.

Two info-level findings from the phase's own code review (`03-REVIEW.md`, 0 critical/0 warning/2 info, status clean), both confirmed still present and both cosmetic/non-blocking:

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/server/registertools_test.go` | 75 | Stale doc-comment count ("17-tool set" vs. the actual 19-entry `want` map) | ℹ️ Info | Prose-only; the test's actual assertion is unaffected. |
| `internal/server/tools.go` | 3093-3097 | `supersede_memory` MCP closure checks `out.Validated` before checking `err`, producing a misleading (but harmless, given the SDK's error-precedence convention every other handler in this file relies on) success message if a `validate_only` preflight rejects | ℹ️ Info | Confirmed present at read time; pre-existing pattern shared with the unmodified `store_memory` closure; does not affect correctness of the returned error, only cosmetic result text on an error path. |

### Human Verification Required

None. This phase ships no UI (D-28, confirmed in `03-07-SUMMARY.md`); every truth above was verified by an automated test or a direct codebase check run independently by this verification, not by re-reading SUMMARY.md claims. No item requires manual/mechanical UAT.

### Gaps Summary

No gaps. All four roadmap success criteria are directly verified against the codebase (not merely asserted by SUMMARY.md): the D-15 CSRF-first test exists and passes for all three mutating RPCs; every RPC delegates to the same core its MCP tool calls; the SupersedeMemory every-offender rejection test exists and passes; `buf breaking` is clean, all generated trees are regenerated and committed with zero diff, and the vendored SPA was independently rebuilt and shown byte-identical to the committed `internal/webauth/static` (the `ui-drift` gate). The full test suite for every package this phase touches (`internal/server`, `internal/surfaces`, `internal/store`, `internal/keylinks`) was re-run independently and passed. The only outstanding item — `REQUIREMENTS.md` checkboxes not flipping — is a documented, pre-existing, milestone-wide tool defect unrelated to this phase's implementation, not a gap in the phase's delivered capability.

---

_Verified: 2026-09-27_
_Verifier: Claude (gsd-verifier)_
