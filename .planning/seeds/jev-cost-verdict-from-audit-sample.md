---
title: Jev cost/complexity verdict from a graded audit sample
trigger_condition: >-
  After #618 ships and ENGRAM_SEARCH_RERANK_AUDIT has been on in production
  for roughly a week (or ~200 reranked searches, whichever first).
planted_date: 2026-09-25
tags: [jev, rerank, cost-justification, llm-as-judge, retrieval-eval, gh-618]
---

# Jev cost/complexity verdict from a graded audit sample

## Idea

Pull the captured audit lines from ClickStack, reconstruct each search
(query + lexical order + Jev order, content via `get_memory`), and grade a
sample: "which order better answers the query?" — human, or a stronger
model as judge (not Jev itself; that would be circular). Combine with Tier 1
aggregates (reorder rate, top1-change rate, no-answer rate) and the `decide`
span's cost/latency to decide **keep / off / tune**, and record the verdict
next to `04-EVAL-JEV.md` so the opt-in has a production-traffic number, not
only a fixture number.

## Open questions

- Judge choice and blinding (present orders A/B unlabeled).
- Whether "no answer" queries (all relevance ≤ ~0.03) should be scored
  separately — the eval suggested that signal is the most valuable part.
- Turn the audit flag off afterwards, or keep sampling at low rate.
