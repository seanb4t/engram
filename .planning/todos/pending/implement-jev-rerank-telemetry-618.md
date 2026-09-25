---
title: Implement Jev reranker telemetry — span attrs + opt-in audit log (#618)
date: 2026-09-25
priority: medium
tags: [jev, rerank, telemetry, otel, search_memory, gh-618]
---

# Implement Jev reranker telemetry (#618)

Design: `.planning/notes/jev-rerank-telemetry-design.md`. Issue #618 states
the problem; this todo is the agreed solution.

## Deliverables

1. **Tier 1 span attributes** (`engram.rerank.*`) stamped on the ambient span
   from `store.applyRankHook`/`RankWithHook`; `fallback_class` stamped by
   `relevance.Hook`. Present for MCP (`tool/search_memory`,
   `tool/search_discovery`) and Connect search RPCs; absent when the ranker
   is off.
2. **Tier 2 audit log** behind `ENGRAM_SEARCH_RERANK_AUDIT` (koanf, default
   false): one structured line per reranked search with query, owner, k,
   outcome, and per-candidate id/lexical_rank/jev_rank/cosine/relevance over
   the full pool. Loud startup log when enabled. Never content or summaries.
3. docs-site: telemetry reference gains the attribute table; configure guide
   gains the audit flag with its privacy caveat.

## Acceptance criteria

- [ ] With the ranker on and a hook that reorders, the recorded search span
      carries `outcome=applied`, correct `top1_changed`/`moved`/`promoted`
      (measured within the caller's k), and `relevance_max`
- [ ] With a failing hook, span carries `outcome=fallback` + `fallback_class`
      matching the existing WARN line's `class`
- [ ] Ranker off → no `engram.rerank.*` attributes at all
- [ ] Audit flag off → no audit line; on → one line per reranked search, ids
      only, no `content`/`summary` keys anywhere in the record
- [ ] Neither surface ever carries the API key, candidate content, or (Tier 1)
      the query
- [ ] `task` (lint + test) green; `gofmt -l .` repo-wide clean
