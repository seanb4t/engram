---
phase: 03-curation-rpcs-mcp-tools
plan: 05
subsystem: api
tags: [connectrpc, mcp-tools, proto, buf, related-memories, oneof-evidence]

requires:
  - phase: 03-curation-rpcs-mcp-tools
    provides: "plan 03-01's option-a wire contract confirmation, item 5: enum EdgeType {UNSPECIFIED, SUPERSESSION, CITATION, TAG, VECTOR}; enum SupersessionDirection {UNSPECIFIED, SUCCESSOR, PREDECESSOR}; oneof evidence {VectorEvidence, TagEvidence, CitationEvidence, SupersessionEvidence}; no buf.validate on new request messages"
provides:
  - "RelatedMemories Connect RPC, a read Procedure (CSRF-exempt), delegating to the same deps.relatedMemories core the related_memories MCP tool calls (RPC-04, RPC-05)"
  - "Store.RelatedMemories gains a full bool (D-13): compact by default (relatedShape/summaryShape), full opt-in via recallView/isSummaryView — the same projection contract List/Search already established"
  - "A type-safe oneof wire shape for the four edge types (D-12), generated Go/TS types Phase 5's graph work and any future console related-memories view can consume directly"
  - "related_memories MCP tool, framed on-demand (D-02) — dedup-before-store / correction-target discovery, never at session start, never an automatic search follow-up"
affects: [03-06-curation-rpcs-mcp-tools, 03-07-curation-rpcs-mcp-tools]

actuals:
  tokens: 52438
  tasks: 2
  commits: 3
  plan_head_before: 5c3602539603e9101b31e90cbeaed7e96cc7ad88

tech-stack:
  added: []
  patterns:
    - "relatedShape(m, full): the phase's now-third full/compact selector alongside recallView/isSummaryView (List, Search) and shapeProtoMemories (Connect) — full unchanged, else summaryShape; used at every former summaryShape call site in internal/store/relatedmemories.go so the store's own compact/full contract has exactly one selector."
    - "relatedResultToProto composes shapeProtoMemories for the anchor and every entry's memory, exactly like every other read RPC — the Connect lane's compact/full projection is never a separate code path."
    - "relatedEdgeToProto/edgeTypeToProto/supersessionDirectionToProto: a switch on the store's flat RelatedEdgeType setting exactly one proto oneof wrapper per case — the canonical D-12 enforcement point, mirrored by relatedEdgeToProto's test-only inverse in the parity test."

key-files:
  created:
    - internal/server/related.go
    - internal/server/related_test.go
  modified:
    - internal/store/relatedmemories.go
    - internal/store/relatedmemories_test.go
    - internal/store/concurrent_gates_test.go
    - internal/store/schemaversion_recallgate_test.go
    - internal/server/store_iface.go
    - internal/server/fakestore_test.go
    - proto/engram/v1/engram.proto
    - gen/go/engram/v1/engram.pb.go
    - gen/go/engram/v1/engramv1connect/engram.connect.go
    - gen/ts/engram/v1/engram_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts
    - internal/server/connectapi.go
    - internal/server/protoconv.go
    - internal/server/tools.go
    - internal/surfaces/toolclass.go
    - internal/surfaces/toolclass_test.go
    - internal/server/registertools_test.go
    - internal/e2e/boot_test.go
    - internal/server/connectapi_parity_test.go
    - internal/server/connectcsrf_test.go
    - docs-site/src/content/docs/reference/tools.md
    - .planning/phases/01-store-prerequisites/01-03-PLAN.md
    - .planning/phases/01-store-prerequisites/01-04-PLAN.md

key-decisions:
  - "Task 1's tracer feedback gate: HUMAN_VERIFY_MODE default (end-of-phase), interactive run, the tracer's <verify> carried only <automated> blocks — re-ran the automated verify, it passed, proceeded straight to Task 2 with no checkpoint (per the #3299 row-3 carve-out)."
  - "Both TestRelatedMemoriesConnectRoundTrip (Task 1) and TestRelatedMemoriesNeverShowsPrivate/TestRelatedMemoriesCompactVsFullMCP (Task 2) pad their owned fixture with 2 plain untagged records: relatedTagEdges' ubiquity guard (2*d > n) would otherwise filter out a rarity-weighted tag shared by exactly 2 records whenever the shared test collection happens to be freshly created (n as low as the fixture's own 3 records, since 2*2=4 > 3). Padding to n>=4 (2*2<=4) makes the tag-edge assertion deterministic regardless of what else is visible in the shared internal/server test collection."
  - "Task 2's spy-based TestReadParity/RelatedMemories row round-trips relatedResultMap through encoding/json into typed pieces (idOnly, and store.RelatedEdge itself via re-marshal) rather than a hand-rolled map[string]any walk, and compares the Connect side by mapping each proto RelatedEdge back onto store.RelatedEdge with a test-only inverse of relatedEdgeToProto — both sides land on literally the same Go struct, so reflect.DeepEqual proves exact per-field parity (type, score, shared_tags/tag_weight, shared_citations, direction/depth) without a second hand-written comparison shape."
  - "Fixed two Phase-1 key_links patterns (01-03-PLAN.md, 01-04-PLAN.md) that this plan's own store change legitimately broke — see Deviations."

patterns-established:
  - "relatedShape is the RelatedMemories-specific instance of the phase's now-repeated 'one full/compact selector per surface' idiom (recallView for List/Search, shapeProtoMemories for the Connect wire, relatedShape for the store's own related-memories fetch) — a future related-memories-adjacent read should reach for the same shape rather than inventing a fourth."

requirements-completed: [RPC-04, RPC-05, RPC-06]

coverage:
  - id: D1
    description: "RelatedMemories lands on Connect: additive proto (EdgeType/SupersessionDirection enums, four evidence messages, RelatedEdge with a oneof keyed by type, RelatedMemory/RelatedMemoriesRequest/RelatedMemoriesResponse), Store.RelatedMemories gains a full bool (D-13), and the Connect handler delegates to the shared deps.relatedMemories core"
    requirement: RPC-04
    verification:
      - kind: integration
        ref: "internal/server/related_test.go#TestRelatedMemoriesConnectRoundTrip"
        status: pass
      - kind: unit
        ref: "internal/store/relatedmemories_test.go"
        status: pass
    human_judgment: false
  - id: D2
    description: "related_memories MCP tool registered over the same core, framed on-demand (D-02: curation/dedup only, never at session start, never an automatic search follow-up); blast-radius row and tool inventories (toolclass_test.go, registertools_test.go, boot_test.go) bumped 17->18 by construction; docs-site tool-blast-radius table regenerated"
    requirement: RPC-04
    verification:
      - kind: unit
        ref: "internal/surfaces/toolclass_test.go#TestOperationsCoverEveryTool"
        status: pass
      - kind: integration
        ref: "internal/server/registertools_test.go#TestRegisterToolsEnumerable"
        status: pass
      - kind: integration
        ref: "internal/server/related_test.go#TestRelatedMemoriesCompactVsFullMCP"
        status: pass
    human_judgment: false
  - id: D3
    description: "Another actor's private record never appears in the caller's neighbourhood on either lane, compact or full, even when it shares the anchor's tag and citation; the same actor's SHARED record (positive control) does appear with matching edges; an unreadable anchor reads not_found on both lanes echoing only the caller's own supplied input (D-21)"
    requirement: RPC-04
    verification:
      - kind: integration
        ref: "internal/server/related_test.go#TestRelatedMemoriesNeverShowsPrivate"
        status: pass
    human_judgment: false
  - id: D4
    description: "The flat MCP JSON and the Connect oneof carry identical evidence values entry by entry (type, score, shared_tags/tag_weight, shared_citations, direction/depth), plus identical rejection envelopes for an empty id and k=1001, with k=1000 succeeding on both lanes (D-20)"
    requirement: RPC-06
    verification:
      - kind: integration
        ref: "internal/server/connectapi_parity_test.go#TestReadParity/RelatedMemories"
        status: pass
    human_judgment: false
  - id: D5
    description: "RelatedMemories is a read Procedure: absent from csrfWriteProcedures and reachable without a CSRF token on both the allowlist gate and the live interceptor chain (RPC-05)"
    requirement: RPC-05
    verification:
      - kind: unit
        ref: "internal/server/connectcsrf_test.go#TestCSRFWriteProcedureAllowlist"
        status: pass
      - kind: integration
        ref: "internal/server/connectcsrf_test.go#TestReadRPCsCSRFExempt/RelatedMemories"
        status: pass
    human_judgment: false

duration: 49min
completed: 2026-09-27
status: complete
---

# Phase 3 Plan 5: Curation RPCs & MCP Tools — RelatedMemories Summary

**RelatedMemories lands on Connect and as an on-demand `related_memories` MCP tool over one shared core, with a type-safe oneof wire shape for supersession/citation/tag/vector evidence and a full/compact projection knob, proven for isolation and flat-vs-oneof parity on both lanes.**

## Performance

- **Duration:** 49 min (approx)
- **Started:** 2026-09-27T09:03:41Z (approx, per STATE.md's prior session timestamp)
- **Completed:** 2026-09-27T09:52:10Z
- **Tasks:** 2 (1 tracer, 1 auto/tdd)
- **Files modified:** 21 modified, 2 created (plus 2 Phase-1 PLAN.md key_links pattern fixes)

## Accomplishments

- `Store.RelatedMemories` gains a `full bool` (D-13): `relatedShape` (compact by default, unchanged when full) replaces every former `summaryShape` call site — the anchor, both supersession-chain directions, and `assembleRelated`'s merged entries — and the payload fetch now goes through `s.recallView(full)`/`isSummaryView(view)` instead of a hardcoded `s.summaryView()`, matching List/Search's existing projection contract.
- Additive proto (`proto/engram/v1/engram.proto`): `EdgeType`/`SupersessionDirection` enums, `WeightedTag`/`CitationRef`/`VectorEvidence`/`TagEvidence`/`CitationEvidence`/`SupersessionEvidence` messages, `RelatedEdge` with a `oneof evidence` keyed by `type` (D-12), `RelatedMemory`/`RelatedMemoriesRequest`/`RelatedMemoriesResponse`, and the `RelatedMemories` RPC — `buf lint`/`buf breaking` clean, `gen/go`, `gen/ts`, and `ui/src/lib/gen` regenerated.
- New `internal/server/related.go`: the shared `deps.relatedMemories` core (no post-filter, Pitfall 4/DEC-cgb — the Subject IS the enforcement point) and `relatedResultMap` for the MCP tool's flat JSON shaping; `protoconv.go` gains `edgeTypeToProto`/`supersessionDirectionToProto`/`relatedEdgeToProto` (exactly one oneof case per edge type) and `relatedResultToProto` (composed through `shapeProtoMemories`, matching every other read RPC).
- `TestRelatedMemoriesConnectRoundTrip` (real Qdrant): a supersession edge (X supersedes P, `PREDECESSOR`/depth 1), a citation edge and a tag edge (Y shares both with X), every edge's oneof case matching its type, compact-vs-full, exact repeatability (`proto.Equal`), and an unknown short_id reading `CodeNotFound` with the caller's own input.
- `related_memories` MCP tool registered on-demand (D-02: curation/dedup framing, "never at session start"); blast-radius row and the three tool-inventory tests (`toolclass_test.go`, `registertools_test.go`, `boot_test.go`) bumped 17→18; `docs-site/reference/tools.md`'s generated tool-blast-radius table regenerated.
- `TestRelatedMemoriesNeverShowsPrivate` proves D-21 on both lanes, compact and full: another actor's PRIVATE record sharing the anchor's tag and citation never appears; the same actor's SHARED record (positive control) does, with matching tag/citation edges; a not_found on the private anchor echoes only the caller's own input.
- `TestRelatedMemoriesCompactVsFullMCP` proves D-13 through the MCP tool: compact carries no `content` key anywhere and always a `summary` key; full's anchor content is real; every edge on every entry is a flat object whose keys are drawn only from `{type, score, shared_tags, tag_weight, shared_citations, direction, depth}`.
- `TestReadParity/RelatedMemories` (scripted spy, one entry per edge type plus a dual-edge entry) proves the flat MCP shape and the Connect oneof carry identical values entry by entry, plus identical rejection envelopes for an empty id and `k=1001`, with `k=1000` succeeding on both lanes (D-20).
- `RelatedMemories` joins the read-Procedure CSRF-exemption allowlist and `TestReadRPCsCSRFExempt`'s case table (RPC-05).

## Task Commits

Each task was committed atomically:

1. **Task 1: RelatedMemories end to end on Connect — oneof proto, store full knob, shared core, mapping, handler (tracer)** — `1fa1e745` (feat)
2. **Task 2: related_memories MCP tool, blast-radius row, private-record isolation on both lanes, flat-vs-oneof parity, CSRF exemption (auto/tdd)** — `696f9d18` (feat)

**Plan metadata:** pending (this commit)

_A third, non-task commit (`22aec07f`, fix) corrects two Phase-1 `key_links` patterns this plan's own store change broke — see Deviations._

## Files Created/Modified

- `internal/server/related.go` — `relatedArgs`, `deps.relatedMemories`, `relatedMemoryView`, `relatedResultMap`
- `internal/server/related_test.go` — `TestRelatedMemoriesConnectRoundTrip`, `TestRelatedMemoriesNeverShowsPrivate`, `TestRelatedMemoriesCompactVsFullMCP`, plus shared test helpers (`findRelatedMemory`, `findEdgeByType`, `decodeRelated`, isolation assertions)
- `internal/store/relatedmemories.go` — `full bool` threaded through `RelatedMemories`/`relatedSupersessionChain`/`assembleRelated`; new `relatedShape`
- `internal/store/relatedmemories_test.go`, `internal/store/concurrent_gates_test.go`, `internal/store/schemaversion_recallgate_test.go` — every `RelatedMemories` call site gains the new `full` argument (`false`)
- `internal/server/store_iface.go` / `fakestore_test.go` — `memStore.RelatedMemories`; `spyStore`'s scripted `related` field and fake
- `proto/engram/v1/engram.proto` — `EdgeType`, `SupersessionDirection`, four evidence messages, `RelatedEdge`/`RelatedMemory`/`RelatedMemoriesRequest`/`RelatedMemoriesResponse`, one RPC (additive; regenerated `gen/go`, `gen/ts`, `ui/src/lib/gen`)
- `internal/server/connectapi.go` / `protoconv.go` — the `RelatedMemories` handler and its proto conversion helpers
- `internal/server/tools.go` — the `related_memories` tool registration (on-demand Description, error-first closure)
- `internal/surfaces/toolclass.go` / `toolclass_test.go`, `internal/server/registertools_test.go`, `internal/e2e/boot_test.go` — one new `Operation` row, inventories bumped 17→18
- `internal/server/connectapi_parity_test.go` — `relatedParityFixture`, the proto→store inverse helpers, `TestReadParity`'s `RelatedMemories` subtest
- `internal/server/connectcsrf_test.go` — `RelatedMemories` added to the read-Procedure allowlist and `TestReadRPCsCSRFExempt`'s case table
- `docs-site/src/content/docs/reference/tools.md` — regenerated tool-blast-radius table (via `task surfaces:gen`)
- `.planning/phases/01-store-prerequisites/01-03-PLAN.md`, `01-04-PLAN.md` — two stale `key_links` `pattern` values corrected (see Deviations)

## Decisions Made

See `key-decisions` in the frontmatter above (the tracer feedback gate's automated re-run and no-checkpoint continuation, the padding-record fixtures that make the tag-edge rarity math deterministic, the typed-struct parity comparison approach, and the two Phase-1 `key_links` pattern fixes).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed two Phase-1 `key_links` patterns broken by this plan's own store change**
- **Found during:** the mandatory full `go test ./...` run before writing this SUMMARY
- **Issue:** `internal/keylinks`'s `TestActiveMilestoneKeyLinksSatisfiable` failed with two `unsatisfiable` patterns: `01-03-PLAN.md:61`'s `s[.]fetchPayloadsByID[(]ctx, f, s[.]summaryView[(][)], ` and `01-04-PLAN.md:58`'s `s[.]assembleRelated[(]ctx, f, anchor[.]ID, chain, citations, tags, vector[)]`. Both patterns pinned the pre-Task-1 call shapes in `internal/store/relatedmemories.go`; Task 1's `full` knob legitimately changed both call sites (`fetchPayloadsByID`'s third argument is now the `view := s.recallView(full)` variable, not a literal `s.summaryView()` call; `assembleRelated` gained a `full` parameter before its variadic `gated` lists).
- **Fix:** Updated both pattern values to the current call shape (`s[.]fetchPayloadsByID[(]ctx, f, view, ` and `s[.]assembleRelated[(]ctx, f, anchor[.]ID, full, chain, citations, tags, vector[)]`); the `from`/`to`/`via` fields are unchanged — the semantic link these entries assert did not change, only the literal text the pattern must match.
- **Files modified:** `.planning/phases/01-store-prerequisites/01-03-PLAN.md`, `.planning/phases/01-store-prerequisites/01-04-PLAN.md`
- **Verification:** `go test ./internal/keylinks/ -count=1` passes; re-ran the full `go test ./...` afterward, all packages green.
- **Committed in:** `22aec07f` (fix, a standalone non-task commit — the finding surfaced only at the plan-level full-suite gate, after both tasks' own commits)

---

**Total deviations:** 1 auto-fixed (1 bug — a downstream key_links gate broken by this plan's own legitimate signature change).
**Impact on plan:** No scope creep — the fix corrects exactly the two patterns this plan's own code change invalidated, in files this plan does not otherwise touch, per the plan-frontmatter `key_links` maintenance instruction given to this executor.

## Non-Vacuity Evidence (D-27)

Each new/behavior-proving test was confirmed to go red against a temporary, uncommitted mutation of the guarded code, then the mutation was reverted (no harness, patch file, or meta-test committed):

1. **Task 1 — the full/compact selector.** Temporarily changed `relatedShape` to `if false && full { return m }` (never taking the full branch). `TestRelatedMemoriesConnectRoundTrip` went red on both full-content assertions: `full anchor content is empty, want non-empty` and `full Y row content is empty, want non-empty`. Reverted; the test passes green.
2. **Task 1 — the oneof case matching its type (D-12).** Temporarily swapped the tag and citation branches in `relatedEdgeToProto` (tag's branch built a `CitationEvidence`, citation's branch built a `TagEvidence`). `TestRelatedMemoriesConnectRoundTrip` went red on the exact-equality shared_citations/shared_tags assertions AND on `assertEveryEdgeEvidenceMatchesType`'s general invariant (both a wrong-evidence-case error and an exact-value mismatch on the same edges). Reverted; the test passes green.
3. **Task 2 — the Subject threading (Pitfall 4/DEC-cgb).** Temporarily replaced `c.Subj` with `store.Anonymous()` in `deps.relatedMemories`'s call to `d.st.RelatedMemories`. `TestRelatedMemoriesNeverShowsPrivate` went red immediately: the Connect compact call on A's own anchor X failed with `not_found` (an anonymous subject cannot even read A's private anchor), proving the caller's own Subject — not a hardcoded or missing one — is what makes the neighbourhood read work at all. Reverted; the test passes green.
4. **Task 2 — depth precision across the wire (D-20/RPC-06).** Temporarily hardcoded `Depth: 0` in `relatedEdgeToProto`'s supersession branch (ignoring `e.Depth`). `TestReadParity/RelatedMemories/success` went red: the MCP-decoded edge (`Depth: 2`) and the Connect-side edge (`Depth: 0`, converted back to the same shape) mismatched via `reflect.DeepEqual`, printing both sides. Reverted; `TestReadParity` passes green.

## TDD Gate Compliance (Task 2)

Task 2 (`type="auto" tdd="true"`) followed the plan's explicit single-commit instruction (one `feat(server): ...` commit with a pathspec that includes the new/extended test files alongside the production files), matching this phase's established convention (03-01/03-02/03-03/03-04-SUMMARY.md): red evidence via temporary uncommitted mutation (D-27, above) rather than a committed RED-phase commit.

- **RED:** `TestRelatedMemoriesNeverShowsPrivate` and `TestRelatedMemoriesCompactVsFullMCP` were written first and passed on their FIRST run against Task 1's already-implemented production code — expected, not vacuous: Task 1 (one task earlier, same plan) already implemented the full isolation-through-Subject-threading and compact/full shaping these tests exercise. `TestReadParity/RelatedMemories` and the CSRF-exemption case likewise passed on first run over Task 1's code. The two D-27 mutations above (items 3-4) prove `TestRelatedMemoriesNeverShowsPrivate` and `TestReadParity` are load-bearing rather than passing vacuously.
- **GREEN:** All tests pass with production code unchanged from its Task-1 state, except the one lint fix below.
- **Evidence of load-bearing tests:** the four D-27 mutations (Task 1's two, Task 2's two) prove the hardest-to-get-right invariants — full/compact projection, oneof-matches-type, Subject-is-the-enforcement-point, and exact-value wire precision — are all actually enforced by these tests.

### Small in-flight fix during Task 2

`golangci-lint run ./internal/server/... ./internal/surfaces/... ./internal/store/...` flagged `revive: unused-parameter` on `spyStore.RelatedMemories`'s scripted `k`/`full` parameters (the fake ignores both by design, mirroring `MigrateStatus`'s scripted-not-derived precedent). Renamed both to `_`; no behavior change, no test impact. Not logged as a numbered deviation — a same-commit lint-clean fix on a file this plan already modifies, not a bug in shipped behavior.

## Issues Encountered

- **Real-Qdrant tag-edge rarity determinism.** `relatedTagEdges`' ubiquity guard (`2*d > n`) filters out a tag carried by more than half the caller's cross-spine-visible record set. A fixture with exactly 2 tag-carrying records (anchor + one neighbour) and no other owned records would fail this guard whenever the shared `internal/server` test collection happens to be freshly created (2*2=4 > 3 visible records) — non-deterministic depending on test run order and prior test cleanup. Resolved by padding each affected fixture (`TestRelatedMemoriesConnectRoundTrip`, `TestRelatedMemoriesNeverShowsPrivate`, `TestRelatedMemoriesCompactVsFullMCP`) with 2 plain untagged owned records, guaranteeing `n>=4` from the fixture alone regardless of external test-suite state. Confirmed working against a genuinely fresh testcontainer (not just a warm, already-populated one).
- **Stale cross-phase `key_links` patterns.** See Deviations above — not a defect in this plan's own work, but a downstream consequence of it, caught and fixed by the mandatory full-suite run before this SUMMARY.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Plans 03-06/03-07 can proceed against the confirmed option-a wire contract; `RelatedMemories`' shape (item 5 of 03-01's bundle) is now shipped alongside `ArchiveMemory`/`RestoreMemory` (03-01), `SupersedeMemory` (03-02), `ListScheduled` (03-03), `ListRules` (03-04).
- `RPC-05`/`RPC-06` are shared with sibling plans; `requirements-completed` above lists them per this plan's own frontmatter, and the shared-ID gate governs when they actually check off in `REQUIREMENTS.md`.
- No blockers.

## Self-Check: PASSED
