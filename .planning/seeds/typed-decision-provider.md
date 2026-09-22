---
title: Typed-decision provider interface (Jev backend + chat-LLM emulator)
trigger_condition: >-
  When the Jev feasibility spike shows value on at least one surface (spine
  curation, recall ranking, write-time hints), or when spine-curation, rerank,
  or rule-capture (#351) work is scoped.
planted_date: 2026-09-22
tags: [jev, typesafe, system-one, decisions, rerank, spine-review, curation, provider-neutral, advisory]
---

# Typed-decision provider interface

## The idea

Add a provider-neutral "decision" client to engram, alongside
`internal/embed` and `internal/summarize`: shared state plus a batch of typed
questions (Choice / Score / Noul, System One's own vocabulary) in, per-option
probabilities and confidence out. Two backends:

1. **Jev** via OpenRouter's Decisions API (`/api/alpha/decisions`) or
   TypeSafe directly — new `ENGRAM_` registry keys, same env-first /
   bounded-timeout / bounded-drain conventions as the existing clients.
2. **Chat-LLM emulator** — a Go port of the MIT
   `system-one-adapter-python` contract over the existing `openai.chat_*`
   plumbing, so self-hosted deployments need no hosted vendor.

Off by default. Advisory only: results are surfaced, never acted on.

## Consumers (in likely order)

1. `spine-review consolidate` — verdict + confidence per candidate pair.
2. Recall reranker — relevance score per candidate, gated on
   `task eval:retrieval` recall@k / MRR vs the lexical reranker.
3. Store-response hints — likely-junk, category, rule candidate, possible
   contradiction (suggest `supersede_memory`, never do it).

## Why it's a seed, not a phase

Value is unproven: calibration and latency claims are vendor-reported, and
the product is weeks old. The spike decides.

## Links

- Note: `.planning/notes/jev-system-one-decisions.md`
- Research question: `.planning/research/questions.md` (gateway passthrough, ZDR)
