# Phase 13: Embedder Reliability Foundation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-10
**Phase:** 13-embedder-reliability-foundation
**Areas discussed:** Identity hash composition, Identity representation & placement, Timeout bounds & queue coupling, Base-URL join strategy

---

## Identity hash composition

| Option | Description | Selected |
|--------|-------------|----------|
| model + dim + doc-side | Hash model + dim + document_instruction + document_params; exclude query-side, base_url, api_key, timeout | ✓ |
| model + dim + ALL params | Also fold in query_instruction/query_params (over-conservative → false positives) | |
| model + dim only | Simplest; misses document-side drift that changes stored vectors (false negatives) | |

**User's choice:** model + dim + doc-side
**Notes:** The stored vector is a document embed; only fields that change the stored vector define the embedding-space identity. Query-side config never touches stored vectors.

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, version-prefix | `v1:` prefix so the hashing scheme can evolve without ambiguous space-drift | ✓ |
| No, unversioned | Simpler; later scheme change needs a coordinated migration | |

**User's choice:** Yes, version-prefix

---

## Identity representation & placement

| Option | Description | Selected |
|--------|-------------|----------|
| Version-prefixed short hash | `v1:` + first 16 hex of SHA-256 over canonical serialization | ✓ |
| Readable canonical descriptor | Self-describing string; variable length, normalization becomes contract | |
| Hybrid: hash key + descriptor | Both; best debuggability, two fields to keep in sync | |

**User's choice:** Version-prefixed short hash

| Option | Description | Selected |
|--------|-------------|----------|
| Pure helper over cfg | `embedderIdentity(cfg)` in config/store layer (holds config + dim); single source, table-testable | ✓ |
| On embed.Client + dim appended | Splits identity across client (no dim) and store | |

**User's choice:** Pure helper over cfg

| Option | Description | Selected |
|--------|-------------|----------|
| All doc-embed writes, payload-only | Stamp store/update/discovery/rule/reindex; NOT on recallView, NOT on proto wire | ✓ |
| All doc-embed writes, also on get_memory | Same + recallView allowlist edit (leaks internals, no consumer) | |

**User's choice:** All doc-embed writes, payload-only
**Notes:** No proto bump this phase. Future reindex-audit CLI reads the stamp directly from Qdrant.

---

## Timeout bounds & queue coupling

| Option | Description | Selected |
|--------|-------------|----------|
| 0 = no timeout, like summary | Mirror ENGRAM_SUMMARY_TIMEOUT; `0s` disables (infinite); default 30s | ✓ |
| Enforce a positive floor | Reject 0; safer but diverges from summarize.timeout convention | |

**User's choice:** 0 = no timeout, like summary

| Option | Description | Selected |
|--------|-------------|----------|
| Assert-only invariant | Embed/queue independent; regression-test that maxElapsed tracks ENGRAM_SUMMARY_TIMEOUT; researcher confirms no hidden coupling; no summaryqueue code change | ✓ |
| Investigate for real coupling | Trace embed→queue coupling; could add a summaryqueue change | |

**User's choice:** Assert-only invariant
**Notes:** Code reading confirmed summaryqueue already derives maxElapsed from ENGRAM_SUMMARY_TIMEOUT (Phase 11); the only stale 30s literal is embed.go:77.

---

## Base-URL join strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Smart heuristic + table test | Normalize; append /embeddings if already at `/v1`/`/v1beta/openai` root, else /v1/embeddings | |
| Heuristic + explicit override escape hatch | Same heuristic PLUS an operator override env for unanticipated shapes (e.g. Azure) | ✓ |
| Explicit path config only | Operator sets full path; predictable but breaks existing configs | |

**User's choice:** Heuristic + explicit override escape hatch
**Notes:** Override wins when set, heuristic applies when empty. Azure-style deployment URLs handled via the override, not the heuristic.

---

## Claude's Discretion

- Exact `embed.Client` timeout wiring (New signature vs WithTimeout option).
- Exact override env var name + full-URL-vs-path form + its validation.
- Canonical serialization feeding the SHA-256 (deterministic, documented; v1: prefix covers future change).
- Package placement of `embedderIdentity` and `joinEmbeddingsURL` helpers.
- Optional OTEL span attribute exposing the identity hash.

## Deferred Ideas

- Reindex-boundary AUDIT CLI (the consumer of the identity stamp) — this phase only stamps.
- Surfacing the identity on get_memory / Connect wire — payload-only for now.
- Azure OpenAI-style deployment URLs — handled by the D-11 override escape hatch, out of scope for the heuristic.
