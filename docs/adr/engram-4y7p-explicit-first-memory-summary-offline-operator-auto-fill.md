<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-4y7p; do not edit manually; use `/adr update engram-4y7p` -->

# Explicit-first memory summary with offline operator auto-fill

**Date:** 2026-06-26
**Status:** Accepted
**Decision:** engram-4y7p
**Deciders:** Sean

## Context

engram records are dense 2-4 KB mini-documents. Full-content recall at session bootstrap already overflows the MCP tool token limit (~70 KB for a single list_memory on a busy spine). A `summary` field must be populated to shrink recall, but the question was WHO writes it and WHEN, under engram's core constraint: explicit, zero-junk, correctable, no auto-extraction.

## Decision

The submitter may author a `summary` at write time (`summary_source=client`). When absent, the operator runs `engram summarize-missing`, which calls a shared, idempotent `FillSummary` per record via the same OpenAI-compatible gateway used for embeddings (a cheaper model, selected by `ENGRAM_SUMMARY_MODEL`). Auto-summary is NEVER on the write path; it is an offline operator sweep mirroring reindex / prune-expired / migrate-set-owner.

## Rationale

- Auto-summary of already-explicit content as an operator action preserves "no auto-extraction" — it is not silent mining at write time.
- The store_memory write path keeps zero summarization latency and no new failure mode; a gateway outage cannot fail a write.
- `FillSummary` is idempotent and vector-preserving (SetPayload, no re-embed), so a future async-on-write queue worker reuses it unchanged — the seam costs nothing in v1.
- Reuses the established offline operator-CLI pattern already in the codebase.

## Alternatives Considered

- **Explicit-first + offline operator auto-fill (chosen):** full submitter control; auto-fill is an explicit operator action; correctable; no write-path cost. Trade-off: records lacking a submitter summary show truncated content until the next sweep.
- **Auto-always at write time, synchronous (rejected):** every record summarized immediately, but adds summarizer latency and a failure mode to the hot write path and reads as auto-extraction at write time.
- **Explicit-only, no auto path (rejected):** simplest, but leaves the legacy backlog unsummarized forever and demands per-record submitter discipline.
- **Lazy-at-first-recall (rejected):** mutates on read (concurrency hazard) and adds cold-start latency to recall, contradicting side-effect-free pull recall.

## Consequences

- Positive: write path unchanged (no latency/failure); legacy backlog summarizable with no schema migration; async-on-write queue is a thin documented future seam over `FillSummary`.
- Negative: summary coverage is operator-cadence-dependent; records without a submitter summary surface truncated content until swept.
- Neutral: auto summaries are lossy by design; `summary_source=auto` is surfaced at recall so agents know to fetch full before acting on specifics.
