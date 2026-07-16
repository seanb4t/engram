---
phase: 13-embedder-reliability-foundation
plan: 03
subsystem: store
tags: [go, embed-identity, qdrant, reindex, resume, provenance]

# Dependency graph
requires:
  - phase: 13-02
    provides: "config.EmbedderIdentity(cfg), store.embedderIdentityKey, and Memory.EmbedderIdentity's payload()/fromPayload() codec — the exact seam this plan stamps onto reindex's divergent raw-map write."
provides:
  - "ReindexOptions.Identity — the embedder-config-identity to stamp on every reindexed record, via a guarded additive raw-map write (not a Memory/payload() round-trip) that preserves the verbatim-payload absent-owner-key invariant."
  - "Identity-aware resume: reindexTargetContents/the resume skip predicate now consult both content AND the target's stamped embedder_identity, so a content-match-but-unstamped/stale target is re-embedded and restamped instead of silently skipped."
  - "StoreAndEmbedderFromEnvNoEnsure returns the computed embedder identity as a 5th value from its single config load, threaded into ReindexOptions.Identity by cmd/engram/reindex.go."
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Divergent-mechanism additive stamp: Reindex writes embedder_identity as a single guarded raw-map key (qdrant.NewValueString) immediately before the verbatim Upsert — never via Memory/payload() — the one intentional exception to the payload-preserved-VERBATIM invariant, kept auditable via a doc-comment note."
    - "Identity-aware resume skip: a content-match skip predicate must also check any correctness invariant the write path independently enforces (here: embedder_identity), or the resume fast-path silently regresses that invariant on re-runs."

key-files:
  modified:
    - internal/store/store.go
    - internal/store/reindex_test.go
    - internal/server/tools.go
    - internal/server/tools_test.go
    - cmd/engram/reindex.go
    - internal/retrievaleval/retrieval_eval_test.go

key-decisions:
  - "reindexTargetContents renamed its return shape from map[string]string (id->content) to map[string]reindexTarget (id->{content, identity}) rather than adding a parallel lookup — one Get call, one map, matching the plan's stated O(pages) cost."
  - "The stamp write happens INSIDE the per-point loop only on the embed+upsert path (after embed, before Upsert) — a dry run and a skipped-as-Unchanged point never touch the payload map, so DryRun correctly stamps nothing (correct by construction, no special-case needed)."
  - "TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce (not a new sibling test) extended to assert the identity is non-empty, v1:-prefixed, and equal to config.EmbedderIdentity(cfg) loaded via the unwrapped `orig` loader — keeping the single-load and identity-computed assertions co-located, matching 13-02's TestBuildDepsFromEnvLoadsConfigOnce precedent."

patterns-established:
  - "A resume/idempotent-skip fast path must be audited against every side-effect the full write path performs, not just the fields the skip predicate was originally written to compare — a superset-of-current-invariants review question for future skip-predicate additions."

requirements-completed: [REQ-embed-config-identity]

coverage:
  - id: D1
    description: "ReindexOptions gains an Identity field; Reindex writes p.Payload[embedderIdentityKey] = qdrant.NewValueString(opts.Identity) only when opts.Identity != \"\", immediately before the verbatim Upsert — no Memory/payload() construction introduced at this site."
    requirement: "REQ-embed-config-identity"
    verification:
      - kind: unit
        ref: "internal/store/reindex_test.go#TestReindexStampsEmbedderIdentity"
        status: pass
    human_judgment: false
  - id: D2
    description: "The additive stamp preserves the verbatim-payload owner-key invariant: a source point lacking an owner key still lacks it in the target after the stamp is written; a source point's owner value survives untouched."
    requirement: "REQ-embed-config-identity"
    verification:
      - kind: unit
        ref: "internal/store/reindex_test.go#TestReindexStampsEmbedderIdentity"
        status: pass
    human_judgment: false
  - id: D3
    description: "StoreAndEmbedderFromEnvNoEnsure returns the computed embedder identity from its single config load; all three callers (cmd/engram/reindex.go, internal/retrievaleval/retrieval_eval_test.go, internal/server/tools_test.go) compile against the new 5-value signature; reindex.go threads it into ReindexOptions.Identity."
    requirement: "REQ-embed-config-identity"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce"
        status: pass
      - kind: build
        ref: "go build ./..."
        status: pass
    human_judgment: false
  - id: D4
    description: "The returned identity is behavior-tested (not just arity): non-empty, v1:-prefixed, and equal to config.EmbedderIdentity(cfg) for the default test config, while the single-config-load count stays exactly 1."
    requirement: "REQ-embed-config-identity"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce"
        status: pass
    human_judgment: false
  - id: D5
    description: "Resume is identity-aware: under Resume:true with a non-empty Identity, a target point whose content matches the source but whose embedder_identity is absent or mismatched is re-embedded and restamped (counted Upserted, not Unchanged); a target already carrying the matching identity is still skipped (counted Unchanged)."
    requirement: "REQ-embed-config-identity"
    verification:
      - kind: unit
        ref: "internal/store/reindex_test.go#TestReindexResumeRestampsStaleIdentity"
        status: pass
    human_judgment: false

# Metrics
duration: 20min
completed: 2026-07-11
status: complete
---

# Phase 13 Plan 03: Reindex Embedder-Identity Stamp (5th Write Site) Summary

**`engram reindex` now stamps the embedder-config-identity onto every rewritten record via a guarded additive raw-map write that preserves the verbatim-payload owner-key invariant, with a resume skip predicate made identity-aware so a content-match-but-unstamped target is restamped, not silently skipped — completing SC3 across all 5 document-embed write sites.**

## Performance

- **Duration:** ~20 min
- **Tasks:** 3 (each committed atomically)
- **Files modified:** 6

## Accomplishments

- `ReindexOptions.Identity string` — new field, doc-commented as the stamp to write (empty = no stamp). `Store.Reindex`'s per-point loop writes `p.Payload[embedderIdentityKey] = qdrant.NewValueString(opts.Identity)` on the raw payload map, guarded by `opts.Identity != ""`, immediately before the verbatim `Upsert` — never via `Memory`/`payload()`, which would synthesize an explicit `owner==""` and relocate a pre-isolation record into the anonymous bucket. The `Reindex` doc comment now documents this as the one intentional additive exception to "payload preserved VERBATIM".
- **Identity-aware resume** (review round-1 MEDIUM, incorporated at plan-time): `reindexTargetContents` now returns `map[string]reindexTarget{content, identity}` instead of `map[string]string`. The skip predicate treats a point as `Unchanged` only when content matches AND (`opts.Identity == ""` OR the target's stored identity already equals `opts.Identity`); a content match with an absent/stale identity falls through to embed+upsert, which restamps it. `DryRun` never enters this path, so a dry run correctly stamps nothing — no special-case needed.
- `StoreAndEmbedderFromEnvNoEnsure` returns a 5th value: the identity computed via `config.EmbedderIdentity(cfg)` from the single already-loaded cfg (no second config load — the engram-635 invariant holds). All 3 callers updated: `cmd/engram/reindex.go` threads it into `ReindexOptions.Identity`; `internal/retrievaleval/retrieval_eval_test.go` and `internal/server/tools_test.go` updated to the new arity.
- **Behavior-tested identity** (review round-2 MEDIUM, incorporated at plan-time): `TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce` extended to assert the returned identity is non-empty, `v1:`-prefixed, and equal to `config.EmbedderIdentity` computed from an independently-loaded expected config (via the unwrapped `orig` loader, so the expected-value load does not inflate the single-load counter) — proving the helper computes a real identity, not just that the new arity compiles.
- Two new integration tests in `internal/store/reindex_test.go` (per the review LOW: reindex scaffolding lives here, not `store_test.go`): `TestReindexStampsEmbedderIdentity` (stamp applied to both a full and a raw source point; owner-key invariant preserved; `Identity:""` never adds the key) and `TestReindexResumeRestampsStaleIdentity` (a target pre-populated with matching content but an absent or mismatched identity is restamped under `Resume:true`, not skipped; a subsequent run with the matching identity is correctly skipped).

## Task Commits

1. **Task 1: ReindexOptions.Identity + guarded raw-map stamp + identity-aware resume skip** - `a5f6dd93` (feat)
2. **Task 2: Expose the identity from StoreAndEmbedderFromEnvNoEnsure + update all 3 callers** - `a07d88b6` (feat)
3. **Task 3: Reindex-stamps-identity integration test (incl. resume-restamp)** - `c2867dc1` (test)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `internal/store/store.go` - `ReindexOptions.Identity`, guarded raw-map stamp in `Reindex`'s per-point loop, `reindexTarget` struct + identity-aware `reindexTargetContents`, amended doc comments
- `internal/store/reindex_test.go` - `TestReindexStampsEmbedderIdentity`, `TestReindexResumeRestampsStaleIdentity`, `s2Upsert` helper
- `internal/server/tools.go` - `StoreAndEmbedderFromEnvNoEnsure` returns the computed identity (5 values, single config load)
- `internal/server/tools_test.go` - both `StoreAndEmbedderFromEnvNoEnsure` callers updated to new arity; `TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce` extended with the identity behavior assertion
- `cmd/engram/reindex.go` - destructures the identity return, passes `Identity: identity` into `ReindexOptions`
- `internal/retrievaleval/retrieval_eval_test.go` - caller updated to new arity (`_` for the unused identity)

## Decisions Made

- `reindexTargetContents`'s return shape changed from `map[string]string` to `map[string]reindexTarget{content, identity}` rather than a second parallel lookup call — keeps the resume cost at one `Get` per page (O(pages), not O(points)), matching the existing doc comment's stated cost model.
- The stamp write lives strictly inside the embed+upsert branch of the per-point loop (after `embed`, before `Upsert`), so `DryRun` (which never enters that branch) and an `Unchanged`-skipped point are correct by construction without an explicit guard.
- Kept `TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce` as the single extended test (not a new sibling) per the plan's stated preference — mirrors 13-02's `TestBuildDepsFromEnvLoadsConfigOnce` precedent of co-locating the single-load and identity-computed assertions.

## Deviations from Plan

None — plan executed exactly as written across all 3 tasks. All `must_haves.truths` and `must_haves.prohibitions` from the plan frontmatter are implemented and covered by tests.

One test-authoring wrinkle worth recording (not a deviation from the plan's `must_haves`, but a correction during Task 3 authoring): the initial `Identity:""` sub-case assertion incorrectly expected the `embedder_identity` key to be entirely ABSENT from a reindexed record seeded via the normal `Store.Upsert`/`payload()` path. `payload()` (from 13-02) writes `p[embedderIdentityKey] = m.EmbedderIdentity` UNCONDITIONALLY (the established "unconditional write, conditional read" `AccessCount`-precedent pattern), so such a source point already carries the key with an empty-string value before reindex ever runs. The test was corrected to assert the key stays absent only for the RAW source point (which bypasses `payload()` entirely) and stays unchanged (empty string, not newly added) for the full point — both true statements about `Identity:""` never ADDING the key, which is what the plan's `must_haves.truths` actually requires.

## Issues Encountered

None beyond the test-authoring correction above (caught by the test itself failing on first run, fixed before commit).

## User Setup Required

None — no external service configuration required. `--target`/`--source`/`--resume` reindex CLI flags are unchanged; the identity stamp is computed automatically from the already-configured `ENGRAM_EMBED_*` values, same as the other 4 write sites from 13-02.

## Next Phase Readiness

- SC3 (every newly stored record — now including reindexed records and identity-stale resume targets — carries the embedder-config-identity) is complete across all 5 document-embed write sites. Phase 13's three isolated reliability fixes (timeout, base-URL join, embedder-config-identity) are all shipped.
- `task lint:go` and `task test` both green. `task lint:markdown` fails only on the pre-existing systemic `.planning/` rumdl issue (documented in STATE.md, tracked for Phase 21 `.rumdl.toml` exclude) — not a regression from this plan.
- No further plumbing needed at this layer; a future reindex-boundary audit CLI (deferred in 13-CONTEXT.md) can now read `embedder_identity` off every record regardless of which of the 5 write paths produced it.

---

## Self-Check: PASSED

All created/modified artifact files exist on disk and all task commit hashes
(`a5f6dd93`, `a07d88b6`, `c2867dc1`) are present in git log.

---

*Phase: 13-embedder-reliability-foundation*
*Completed: 2026-07-11*
