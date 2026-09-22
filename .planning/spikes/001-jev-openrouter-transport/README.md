---
spike: 001
idea: jev-typed-decisions
name: jev-openrouter-transport
type: standard
validates: "Given the OpenRouter key, when a Decisions request (Choice + Noul against one state) is POSTed to /api/alpha/decisions, then typed answers with per-option probabilities return fast enough for engram's surfaces"
verdict: VALIDATED
related: []
tags: [jev, typesafe, openrouter, decisions-api, latency, transport]
---

# Spike 001: Jev on OpenRouter — transport, latency, limits

## What This Validates

Given engram's existing OpenRouter credentials (`ENGRAM_OPENAI_BASE_URL=https://openrouter.ai/api`,
`ENGRAM_OPENAI_API_KEY`), when typed decision requests are POSTed to `/api/alpha/decisions`,
then typed answers with per-option probabilities come back, with known latency, limits, and
error shapes.

## Research

- OpenRouter Jev hub + tutorial (openrouter.ai/docs/guides/community/jev,
  …/jev-tutorial), fetched 2026-09-22.
- Request: `{model, state: {...}, questions: {name: {type: noul|choice|score, instructions,
  criteria}}}`. `criteria` is an object (noul: `true`/`false`; choice: option → description)
  or an ordered array (score).
- Response: `answers.<name>` = `{type, noul}` | `{type, choice, probabilities, confidence}` |
  `{type, score, probabilities, confidence, legend}`, plus `usage {input_tokens,
  output_tokens, cost}`, `model` (dated snapshot), `id`, `provider`.
- **OpenRouter ships a Go SDK for the Decisions API**
  (openrouter.ai/docs/client-sdks/go/sdks/decisions) — the real build should evaluate it
  before hand-rolling a client (rule `xvqj44e5mk`). This spike uses plain `net/http` only to
  see raw bytes.

## How to Run

```sh
cd .planning/spikes/001-jev-openrouter-transport
go run main.go   # needs ENGRAM_OPENAI_BASE_URL + ENGRAM_OPENAI_API_KEY; writes results.json
```

## What to Expect

stdout lines for each probe (happy, latency, batch50, card255/256, badkey, badtype,
oversize, chatpath, determinism) and `results.json` with every event (timestamp, status,
ms, bytes, error bodies).

## Observability

`results.json` — event log per request (ISO timestamp, probe tag, HTTP status, wall ms,
response bytes, body for non-200s and sampled 200s). The account `user_id` echoed in one
error body is redacted.

## Investigation Trail

1. Happy path first: a near-duplicate pair of real spine records (storetest import-cycle
   gotcha, reworded) → `duplicate` 0.92, `same_subject` 0.92, 377 ms, $0.000021.
2. Latency needed a distribution, not one call → 30 sequential calls alternating
   duplicate / unrelated pairs. No difference between pair kinds; one 1.1 s outlier.
3. Rerank shape: can one request carry a whole candidate set? 50 Noul questions over
   one state (query + 50 candidates) → answered in one 316 ms call. Target candidate
   scored 0.90, the 49 distractors 0.02–0.04. **Caveat:** the distractors were one
   identical off-topic record — this proves the batching shape and latency, not ranking
   quality. Spike 004 measures quality on the #261 fixture.
4. Cardinality claim (≤255) checked at the boundary: 255 options → 200 (1.26 s, picked the
   right option); 256 → 400 `Too many choices. Must have at most 255 choices.`
5. Error shapes, since a Go client must classify them:
   - bad key → 401 `{"error":{"message":"Missing Authentication header","code":401}}`
   - unknown question type → 400 with a zod-style `invalid_union` detail (discriminator
     `type`, options noul/choice/score)
   - state > 32k tokens → 400 `{"detail":{"error_type":"max_tokens_exceeded"}}` wrapped in
     OpenRouter's `{"error":{message,code}}`
   - `/v1/chat/completions` with the Jev slug → 400, message names the Decisions endpoint.
     Confirms the prior research: engram's chat client cannot reach Jev.
6. Determinism: same request ×5 → P(duplicate) 0.88–0.94. Stable but not bit-identical;
   thresholds need margin.

## Results

**VALIDATED** — Jev is reachable today with engram's existing OpenRouter key and base URL;
only the path differs (`/alpha/decisions` instead of `/v1/...`).

| Measure | Value |
|---|---|
| Latency, 2-question request (n=30) | p50 267 ms · p90 492 ms · p95 498 ms · max 1127 ms |
| Latency, 50-question batch (1 call) | 316 ms |
| Latency, 255-option choice | 1.26 s |
| Cost, 2-question pair verdict | ~$0.00002 (497 input tokens) |
| Cost, 50-candidate rerank batch | ~$0.00017 (4028 input tokens) |
| Choice cardinality | ≤ 255 (256 → 400) |
| Context | 32k tokens (over → 400 `max_tokens_exceeded`) |
| Run-to-run spread | ±0.03 on a 0.9 probability |

Implications:

- **Spine curation:** trivially viable — cost is negligible, latency is irrelevant offline.
- **Rerank:** one call per search is the right shape (batch all candidates as Noul
  questions). ~300 ms p50, ~500 ms p90 added to every `search_memory` is significant; the
  reranker would need to be opt-in and measured against its quality gain (spike 004).
- **Write hints:** ~270 ms p50 per store if synchronous; would belong off the write path
  (like async summaries) or behind an explicit opt-in.
- **Client:** errors are OpenRouter-wrapped `{"error":{message,code}}` with the upstream
  detail stringified inside `message` — classify on HTTP status, not message text.
- Latencies were measured from a US home connection; TypeSafe states its service runs on
  the US West Coast.
