# API Coverage — OpenRouter Decisions API (Jev / System One)

> Full coverage by default. Opt-outs are explicit, reasoned decisions.

External surface: `POST {base}/alpha/decisions` on OpenRouter (`https://openrouter.ai/api`) and
through a LiteLLM pass-through (`https://<gateway>/openrouter`), request/response shapes as
verified live in spikes 001/002 (`.claude/skills/spike-findings-engram/references/decision-transport.md`)
and as declared by the OpenRouter Go SDK's generated Decisions components (static read of
`github.com/OpenRouterTeam/go-sdk` v0.8.19 at planning time; plan 02-01 re-reconciles this
matrix against the version current at execution and records the result in
`02-SDK-EVALUATION.md`).

| capability | decision | reason |
|---|---|---|
| decisions.create (POST {base}/alpha/decisions) | INTEGRATE | |
| transport.openrouter-direct (base https://openrouter.ai/api) | INTEGRATE | |
| transport.litellm-passthrough (base https://gateway/openrouter) | INTEGRATE | |
| request.model (pinned model id, default typesafe/jev-1.13) | INTEGRATE | |
| request.state (object of named fields) | INTEGRATE | |
| request.state non-object forms (string, array) | OPT-OUT | not needed: the System One contract (DEC-02) models state as named fields, and a string or array state is expressible as one named field |
| request.questions (batched named questions per call) | INTEGRATE | |
| question.noul (criteria true/false) | INTEGRATE | |
| question.choice (criteria option to description, at most 255 options) | INTEGRATE | |
| question.score (ordered criteria array) | INTEGRATE | |
| question.instructions and criteria as text | INTEGRATE | |
| question.instructions and criteria as structured (object or array) union members | OPT-OUT | not needed yet: every planned consumer (Phase 3 verdicts, Phase 4 rerank) phrases instructions and criteria as text; structured forms would be an additive field on the same Question type |
| request.provider (OpenRouter provider-routing preferences) | OPT-OUT | not needed: Jev has a single upstream provider (TypeSafe), so routing preferences select nothing |
| request.session_id (OpenRouter observability grouping) | OPT-OUT | not needed yet: engram correlates each call through its own decide span (D-13) and the response id |
| request.trace (OpenRouter Broadcast trace config) | OPT-OUT | not needed yet: engram's own OTLP decide span is the observability record (D-13); forwarding trace ids to OpenRouter is an additive follow-up |
| request.user (optional end-user identifier) | OPT-OUT | not needed yet: no phase consumer attributes a decision call to an end-user identity; additive if a future caller needs one |
| answer.noul (probability) | INTEGRATE | |
| answer.choice (choice, per-option probabilities, confidence) | INTEGRATE | |
| answer.score (score, per-level probabilities, confidence, legend) | INTEGRATE | |
| response.usage (input_tokens, output_tokens, cost) | INTEGRATE | |
| response.model (dated model snapshot) | INTEGRATE | |
| response.id (generation id) | INTEGRATE | |
| response.provider | INTEGRATE | |
| header.X-Generation-Id | OPT-OUT | not needed: the same generation id arrives in the response body id field, integrated above |
| error.400 bad request (unknown question type, more than 255 choices) | INTEGRATE | |
| error.400 max_tokens_exceeded (state plus questions over 32k tokens) | INTEGRATE | |
| error.401 and 403 (bad or missing key, LiteLLM per-key route grant) | INTEGRATE | |
| error.402 payment required | INTEGRATE | |
| error.404, 413 and other 4xx | INTEGRATE | |
| error.429 rate limited | INTEGRATE | |
| error.5xx (500, 502, 503, 524, 529) | INTEGRATE | |
| error-dialect.openrouter (error.code numeric) | INTEGRATE | |
| error-dialect.litellm (error.code string) | INTEGRATE | |
| systemone.create (POST /api/v1/systemone) | OPT-OUT | not needed: same contract as decisions.create; engram uses /alpha/decisions (D-03), the only route the LiteLLM pass-through grants |
| chat-completions path with a Jev model | OPT-OUT | explicitly out of scope: the provider rejects it with 400 ("is a decisions model"), per spike 001 |

## Where each INTEGRATE row is implemented

| Rows | Plan |
|---|---|
| decisions.create, transport.openrouter-direct, request.model, request.state, request.questions, question.noul, answer.noul, response.usage, response.model, response.id, response.provider | 02-03 (tracer) |
| question.choice, question.score, answer.choice, answer.score, transport.litellm-passthrough | 02-04 (types, validation), 02-08 (wire mapping, base-URL shapes) |
| every error.* and error-dialect.* row | 02-04 (named errors), 02-07 (classification) |

Evaluation evidence and the adopt/reject record for the SDK live in `02-SDK-EVALUATION.md`.
