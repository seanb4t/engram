# Phase 4: Jev Reranker & Per-Hit Relevance Signal - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-24
**Phase:** 04-jev-reranker-per-hit-relevance-signal
**Areas discussed:** Enable & ship gate, Composition with lexical, Relevance signal surface, Budget & latency

---

## Enable & ship gate

| Option | Selected |
|--------|----------|
| Ranker enum `ENGRAM_SEARCH_RANKER` | ✓ |
| Rerank bool | |

| Option | Selected |
|--------|----------|
| Beat lexical, keep #261 | |
| Beat vector only | |
| No bar (opt-in) | ✓ |

## Composition with lexical

| Option | Selected |
|--------|----------|
| Re-sort lexical order (stable) | ✓ |
| Replace lexical | |

| Option | Selected |
|--------|----------|
| Whole CandidateK pool | ✓ |
| Top N after lexical | |

## Relevance signal surface

| Option | Selected |
|--------|----------|
| `relevance` float, omitempty | ✓ |
| Nested `relevance` object | |

| Option | Selected |
|--------|----------|
| Per-hit only | ✓ |
| Add response flag | |

| Option | Selected |
|--------|----------|
| search_memory incl. cross_spine | |
| Plus search_discovery | ✓ |

## Budget & latency

| Option | Selected |
|--------|----------|
| Reuse verdict.State + total cap | ✓ |
| Summaries only | |

| Option | Selected |
|--------|----------|
| Dedicated rerank timeout (~2s, no retry) | ✓ |
| Reuse decisions timeout | |

## Claude's Discretion

- Budget constants, Noul wording, search_discovery threading, CLI formatting, span attributes.

## Deferred Ideas

- Response-level no_relevant_results flag; eval ship bar.
