---
title: Jev / System One typed decisions for engram
date: 2026-09-22
context: >-
  Surfaced from /gsd-explore on TypeSafe AI's "System One" model class and
  its first model, Jev (announced 2026-09-15:
  https://typesafe.ai/blog/introducing-system-one-models-and-jev). Question:
  where could calibrated, typed decisions improve engram?
tags: [jev, typesafe, system-one, decisions, rerank, spine-review, curation, openrouter, advisory]
---

# Jev / System One typed decisions for engram

## What Jev is (vendor claims — unverified unless tagged below)

Unstructured or structured `state` in, typed probabilistic decisions out. No
string generation. Claimed 70–500 ms per call, $0.042/MTok input, output
unmetered, calibrated confidence, choice cardinality up to 255. Early access,
hosted-only.

## Decisions reached in exploration

1. **Surfaces — all three are in scope for evaluation:**
   - **Spine curation** — duplicate / contradicts / unrelated verdicts on
     `engram spine-review consolidate` candidate pairs (today judged by an
     agent via the `curating-spine` skill). Offline, latency-insensitive.
   - **Recall ranking** — calibrated relevance scores to augment or replace
     the stdlib lexical reranker (`store.SearchReranked`). Hot path; the
     retrieval-eval harness (`task eval:retrieval`, #261 fixture) is the
     scoreboard.
   - **Write-time hints** — advisory signals on store: likely-junk, suggested
     category, rule candidate (#351), possible contradiction → suggest
     `supersede_memory`.
   - Summaries are **not** a candidate — Jev does not generate strings.
2. **Seam — provider-neutral.** engram defines a typed-decision interface
   (like `internal/embed` / `internal/summarize`); Jev is one backend, a
   chat-LLM emulator another. Off by default; self-host deployments stay whole.
   Prefer adopting System One's own contract (state + typed Choice / Score /
   Noul questions → probabilities) as the interface vocabulary over inventing
   one (rule `xvqj44e5mk`).
3. **Authority — advisory only.** Verdicts and confidence are surfaced
   (consolidate output columns, store-response hints, score fields); agents
   and operators still act. Reordering recall results is not a mutation, so a
   reranker stays inside the contract. This preserves the "explicit,
   zero-junk, never automatic" design intent.
4. **Transport** — the user reports Jev is reachable via OpenRouter and via
   their LLM gateway.

## Research findings (quick pass, 2026-09-22)

Admitted (survived a refute pass, primary source):

- Native API batches many typed questions against one shared state per
  request, evaluated in parallel; primitives Choice, Score, Noul each return
  probabilities/confidence. — docs.typesafe.ai
- OpenRouter serves Jev via a separate Decisions API
  (`POST /api/alpha/decisions`, alias `/api/v1/systemone`), **not**
  `/chat/completions` (which 400s for the slug); slug `typesafe/jev-1.13`
  (alias `~typesafe/jev-latest`); probabilities come back as native typed
  fields. So engram's OpenAI-chat client is not reusable as-is — only its
  conventions (base URL / key split, bounded timeout, bounded drains). —
  openrouter.ai/docs/guides/community/jev
- `system-one-adapter-python` is MIT-licensed and is the inverse of a Jev
  client: it emulates the System One contract on ordinary chat LLMs (native
  JSON-schema structured outputs, or prompted JSON with schema-validated
  retries; probabilities or discrete mode; renormalizes invalid
  distributions). Small enough to port to Go as the emulator backend over the
  existing `openai.chat_*` plumbing. —
  github.com/typesafe-ai/system-one-adapter-python
- TypeSafe's privacy policy states it does not train on customer inputs;
  retention is standard (as long as reasonably necessary, deleted on
  request), not zero-retention by default. — typesafe.ai/legal/privacy-policy

Unresolved (do not treat as fact):

- Enterprise zero-data-retention on request, and no self-host / on-prem
  option — **non-authoritative source** (third-party aggregator).
- Whether the user's LLM gateway passes through the Decisions API (not just
  chat) — **unverifiable** from docs.
- Whether any probability signal survives a chat-completions fallback path —
  **unverifiable**; product surface is about a week old.

## Tensions

- Sending memory content to a hosted proprietary service vs engram's
  self-hosted posture — mitigated by provider-neutrality and off-by-default.
- Latency on the recall path and on every write vs value — the spike measures it.

## Follow-ups

- Spike: Jev Decisions API feasibility (latency, calibration, rerank eval).
- Seed: `typed-decision-provider` (trigger: spike shows value on a surface).
- Research question: gateway passthrough + ZDR (`research/questions.md`).
