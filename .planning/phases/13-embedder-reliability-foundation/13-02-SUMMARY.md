---
phase: 13-embedder-reliability-foundation
plan: 02
subsystem: store
tags: [go, embed-identity, qdrant, payload-codec, sha256, provenance]

# Dependency graph
requires:
  - phase: 13-01
    provides: "ENGRAM_EMBED_TIMEOUT / joinEmbeddingsURL embed-client hardening (embed.New construction seam, koanf embed config trio) — this plan builds the identity stamp on top of the already-hardened embedder config path."
provides:
  - "config.EmbedderIdentity(cfg) — pure, deterministic, v1:-prefixed 16-hex-char SHA-256 stamp over model+dim+document_instruction+document_params, with empty-params canonicalization ('' and '{}' hash identically)"
  - "store.Memory.EmbedderIdentity — payload-only field (json:\"-\"), persisted exclusively through payload()/fromPayload() under embedderIdentityKey"
  - "deps.embedderIdentity — computed once in buildDepsFromEnv, stamped on all 5 non-reindex document-embed write sites (store_memory, schedule_memory, update_memory re-embed, store_discovery, store_rule)"
  - "D-06 negative-space regression tests locking payload-only invariant at all 3 verbatim full-response wire paths + toRecallView"
affects: [13-03]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Payload-only audit field: json:\"-\" struct tag + manual payload()/fromPayload() codec is the enforcement mechanism for D-06, not handler-level filtering — struct tag makes the leak structurally impossible rather than merely untested"
    - "Canonicalize-before-hash: semantically-equivalent empty inputs (nil vs empty map, from \"\" vs \"{}\") normalized to one form before marshal/hash to prevent false provenance drift"
    - "Compute-once-at-construction: identity computed a single time in buildDepsFromEnv (mirrors embed.Client.embeddingsURL from 13-01), never recomputed per-request"

key-files:
  created:
    - internal/config/identity.go
    - internal/config/identity_test.go
  modified:
    - internal/store/store.go
    - internal/store/store_test.go
    - internal/server/tools.go
    - internal/server/tools_test.go
    - internal/server/rules.go
    - internal/server/rules_test.go
    - internal/server/summary_test.go

key-decisions:
  - "Memory.EmbedderIdentity tagged json:\"-\" (not a normal json tag) per the plan's incorporated round-1 review blocker: store.Memory is returned verbatim on 3 full-response MCP wire paths (shapeRecall full, get_memory, listRules full), so any normal tag would leak the audit field."
  - "document_params canonicalization: len(params)==0 (covers both ParseEmbedParams's nil-for-\"\" and the empty-map-for-\"{}\" cases) is normalized to map[string]any{} before marshal, so both empty spellings hash identically — per round-2 review MEDIUM finding."
  - "TestBuildDepsFromEnvLoadsConfigOnce extended (not a new sibling test) to also assert the returned deps.embedderIdentity is non-empty and v1:-prefixed, keeping the single-config-load assertion and the production-identity assertion in one place."

patterns-established:
  - "Server-set-only audit fields on store.Memory that must never cross the JSON wire use json:\"-\" plus a dedicated payload key const, proven by negative marshal assertions at every site that serializes store.Memory verbatim (not just the recallView allow-list)."

requirements-completed: [REQ-embed-config-identity]

coverage:
  - id: D1
    description: "config.EmbedderIdentity(cfg) is a pure, deterministic, v1:-prefixed 16-hex-char SHA-256 helper over model+dim+document_instruction+document_params only; query-side fields, base_url, api_key, and timeout never change the hash. document_params canonicalizes key-order differences and normalizes both '' and '{}' to one empty form (no null vs {} drift)."
    requirement: "REQ-embed-config-identity"
    verification:
      - kind: unit
        ref: "internal/config/identity_test.go#TestEmbedderIdentityIsDeterministic"
        status: pass
      - kind: unit
        ref: "internal/config/identity_test.go#TestEmbedderIdentityFieldExclusion"
        status: pass
      - kind: unit
        ref: "internal/config/identity_test.go#TestEmbedderIdentityCanonicalization"
        status: pass
    human_judgment: false
  - id: D2
    description: "Memory.EmbedderIdentity round-trips through payload()/fromPayload() under the shared embedderIdentityKey despite its json:\"-\" tag (the manual codec is the only persistence path); a legacy payload missing the key decodes to \"\" with no backfill."
    requirement: "REQ-embed-config-identity"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestPayloadRoundTripsEmbedderIdentity"
        status: pass
    human_judgment: false
  - id: D3
    description: "Each of the 5 non-reindex document-embed write sites (store_memory, schedule_memory, store_discovery, update_memory re-embed, store_rule) stamps the computed embedder identity, proven per-site by re-reading the persisted record via the store's Get and asserting the identity round-tripped; update_memory's case proves the re-embed path RE-stamps, not just the initial write."
    requirement: "REQ-embed-config-identity"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestStoreMemoryStampsEmbedderIdentityHandler"
        status: pass
      - kind: unit
        ref: "internal/server/tools_test.go#TestScheduleMemoryStampsEmbedderIdentityHandler"
        status: pass
      - kind: unit
        ref: "internal/server/tools_test.go#TestStoreDiscoveryStampsEmbedderIdentityHandler"
        status: pass
      - kind: unit
        ref: "internal/server/tools_test.go#TestUpdateMemoryReStampsEmbedderIdentityHandler"
        status: pass
      - kind: unit
        ref: "internal/server/rules_test.go#TestStoreRuleStampsEmbedderIdentityHandler"
        status: pass
    human_judgment: false
  - id: D4
    description: "D-06 payload-only invariant holds: the embedder identity NEVER appears on any full-response MCP wire path — proven by negative JSON assertions at all three sites that serialize store.Memory verbatim (shapeRecall full, get_memory, listRules full) plus the compact toRecallView allow-list."
    requirement: "REQ-embed-config-identity"
    verification:
      - kind: unit
        ref: "internal/server/summary_test.go#TestEmbedderIdentityNeverOnRecallWire"
        status: pass
      - kind: unit
        ref: "internal/server/tools_test.go#TestGetMemoryNeverSurfacesEmbedderIdentity"
        status: pass
      - kind: unit
        ref: "internal/server/rules_test.go#TestListRulesFullNeverSurfacesEmbedderIdentity"
        status: pass
    human_judgment: false
  - id: D5
    description: "The production builder (buildDepsFromEnv) actually computes a non-empty v1:-prefixed identity from the real config — not just that handlers persist a hand-set sentinel — while preserving the single-config-load invariant."
    requirement: "REQ-embed-config-identity"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestBuildDepsFromEnvLoadsConfigOnce"
        status: pass
    human_judgment: false

# Metrics
duration: 15min
completed: 2026-07-11
status: complete
---

# Phase 13 Plan 02: Embedder-Config-Identity Stamp (4 clean sites + rule) Summary

**config.EmbedderIdentity(cfg) mints a v1:-prefixed SHA-256 stamp over the document-side embed config, persisted payload-only (json:"-") through store.Memory and stamped on store_memory/schedule_memory/update_memory/store_discovery/store_rule — with D-06 negative tests locking it off all three verbatim full-response MCP wire paths.**

## Performance

- **Duration:** ~15 min
- **Tasks:** 4 (each committed atomically)
- **Files modified:** 9 (2 created, 7 modified)

## Accomplishments

- `config.EmbedderIdentity(cfg *config.Config) (string, error)` (new `internal/config/identity.go`) — a pure, deterministic helper hashing `model ∥ dim ∥ document_instruction ∥ canonical(document_params)` (unit-separator-joined, SHA-256, `v1:` + first 16 hex chars). Query-side fields, `base_url`, `api_key`, and `timeout` are excluded by construction (D-01/D-02/D-03/D-04). `document_params` is canonicalized via `ParseEmbedParams` + re-marshal (key-order-independent) with `len(params)==0` normalized to `map[string]any{}` so `""` and `"{}"` mint the identical identity (round-2 review MEDIUM — no false `null` vs `{}` provenance drift).
- `store.Memory.EmbedderIdentity string` tagged **`json:"-"`** (not a normal tag) plus a shared `embedderIdentityKey = "embedder_identity"` const, persisted exclusively through the manual `payload()`/`fromPayload()` codec (mirrors the `AccessCount` precedent: unconditional write, conditional read, legacy-missing reads `""` with no backfill — D-05). The `json:"-"` tag is the round-1 review's HIGH-blocker fix: `store.Memory` is returned verbatim on three MCP full-response paths, so a normal tag would leak the field.
- `deps.embedderIdentity` computed **once** in `buildDepsFromEnv` via `config.EmbedderIdentity(cfg)` and stamped before the store write at all 5 non-reindex sites: `storeMemory`, `scheduleMemory`, `storeDiscovery` (`internal/server/tools.go`), `updateMemory`'s re-embed branch (re-stamps on every re-embed, since `Store.Update` re-`Upsert`s `cur`), and `storeRule` (`internal/server/rules.go`).
- D-06 negative-space regression tests at all **three** verbatim full-response wire sites — `shapeRecall(full=true)`, `get_memory`, `listRules(full=true)` — plus the compact `toRecallView` allow-list, each proving a sentinel identity never appears in the marshaled JSON. Positive per-site persistence tests at all 5 write sites prove a missed stamp assignment would now fail a test (a bare compile-and-pass gap the review flagged as MEDIUM).
- `TestBuildDepsFromEnvLoadsConfigOnce` extended to assert the production builder's returned `deps.embedderIdentity` is non-empty and `v1:`-prefixed — proving the builder *computes* a real identity, not just that handler tests can persist a hand-set sentinel (round-2 review MEDIUM).

## Task Commits

1. **Task 1: Pure config.EmbedderIdentity(cfg) helper** - `abcca957` (feat)
2. **Task 2: Memory.EmbedderIdentity field + payload round-trip** - `570cba46` (feat)
3. **Task 3: Wire deps.embedderIdentity, stamp 5 write sites, guard D-06, prove builder computes identity** - `41f51a32` (feat)
4. **Task 4: Positive per-site persistence tests** - `a7d58989` (test)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `internal/config/identity.go` - `EmbedderIdentity(cfg)` pure helper, canonical serialization + empty-params normalization
- `internal/config/identity_test.go` - determinism, field-exclusion table, canonicalization group (incl. `""` == `"{}"` case)
- `internal/store/store.go` - `Memory.EmbedderIdentity` field (`json:"-"`), `embedderIdentityKey` const, payload writer/reader
- `internal/store/store_test.go` - `TestPayloadRoundTripsEmbedderIdentity`
- `internal/server/tools.go` - `deps.embedderIdentity` field, `buildDepsFromEnv` computes it once, stamps at `storeMemory`/`scheduleMemory`/`storeDiscovery`/`updateMemory`
- `internal/server/tools_test.go` - D-06 negative test at `get_memory`; positive persistence tests for `storeMemory`/`scheduleMemory`/`storeDiscovery`/`updateMemory`; `TestBuildDepsFromEnvLoadsConfigOnce` extended
- `internal/server/rules.go` - `storeRule` stamps `d.embedderIdentity`
- `internal/server/rules_test.go` - D-06 negative test at `listRules(full=true)`; positive persistence test for `storeRule`
- `internal/server/summary_test.go` - D-06 negative test at `shapeRecall(full=true)` + `toRecallView`

## Decisions Made

- `Memory.EmbedderIdentity` uses `json:"-"` rather than `json:"embedder_identity,omitempty"` — the plan's incorporated round-1 review blocker fix, since `store.Memory` serializes verbatim on 3 full-response MCP paths.
- Empty-params canonicalization (`len(params)==0` → `map[string]any{}`) applied unconditionally in `EmbedderIdentity`, not left to callers — keeps the invariant enforced at the single source of truth.
- `TestBuildDepsFromEnvLoadsConfigOnce` extended in place (per plan's stated preference) rather than adding a sibling `TestBuildDepsFromEnvComputesIdentity`, keeping the single-load and identity-computed assertions co-located.

## Deviations from Plan

None — plan executed exactly as written across all 4 tasks. All `must_haves.truths` and `must_haves.prohibitions` from the plan frontmatter are implemented and covered by tests.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required. The identity stamp is computed entirely from already-configured `ENGRAM_EMBED_*` values; no new operator-facing env var is introduced by this plan.

## Next Phase Readiness

- `config.EmbedderIdentity`, `store.embedderIdentityKey`, and `Memory.EmbedderIdentity`'s payload codec are the exact seam 13-03 depends on for `Store.Reindex`'s divergent raw-map stamp (`ReindexOptions.Identity` + `StoreAndEmbedderFromEnvNoEnsure` signature change) — no further plumbing needed at this layer.
- `task lint:go` and `task test` both green. `task lint:markdown` fails only on the pre-existing systemic `.planning/` rumdl issue (documented in STATE.md, tracked for Phase 21 `.rumdl.toml` exclude) — not a regression from this plan.

---

## Self-Check: PASSED

All created/modified artifact files exist on disk and all task commit hashes
(`abcca957`, `570cba46`, `41f51a32`, `a7d58989`) are present in git log.

---

*Phase: 13-embedder-reliability-foundation*
*Completed: 2026-07-11*
