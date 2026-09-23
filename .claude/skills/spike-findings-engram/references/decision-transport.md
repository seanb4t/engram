# Decision transport (Jev / System One client)

## Requirements

- Provider-neutral interface in System One vocabulary: shared `state` plus a batch of typed
  questions (Choice / Score / Noul) in, per-option probabilities plus confidence out. Jev is
  one backend; a chat-LLM emulator (a Go port of the MIT `system-one-adapter-python`
  contract) is another.
- Off by default; advisory only — verdicts are surfaced, never acted on (reranking allowed).
- Transport goes through `/api/alpha/decisions`; engram's chat client cannot reach Jev.
- The decision client gets its own base-URL setting; it must not assume the chat/embeddings
  gateway serves Decisions.

## How to Build It

1. **Check for an SDK first.** OpenRouter publishes a Go SDK for the Decisions API
   (openrouter.ai/docs/client-sdks/go/sdks/decisions). Rule `xvqj44e5mk` says evaluate it
   (maintained upstream? module path current?) before hand-rolling. The spikes used plain
   `net/http` only to see raw bytes.
2. **Config** (follow `internal/config/registry.go` conventions: env-first, `ENGRAM_` prefix):
   - a dedicated base URL (e.g. `ENGRAM_DECISIONS_BASE_URL`) falling back to
     `ENGRAM_OPENAI_BASE_URL`; the path appended is `/alpha/decisions`
     (base `https://openrouter.ai/api` → `https://openrouter.ai/api/alpha/decisions`);
   - a dedicated API key falling back to `ENGRAM_OPENAI_API_KEY`;
   - model, default `typesafe/jev-1.13` (pin; `~typesafe/jev-latest` moves thresholds);
   - timeout + bounded response read/drain, reusing the `httpdrain` / max-response-bytes
     pattern from `internal/embed` and `internal/summarize`.
3. **Request shape** (verified live):

   ```json
   {
     "model": "typesafe/jev-1.13",
     "state": {"record_a": "...", "record_b": "..."},
     "questions": {
       "relation": {"type": "choice", "instructions": "How does record_b relate to record_a?",
                    "criteria": {"duplicate": "...", "contradicts": "...", "related": "...", "unrelated": "..."}},
       "same_subject": {"type": "noul", "instructions": "...",
                        "criteria": {"true": "...", "false": "..."}}
     }
   }
   ```

   Score questions take `criteria` as an ordered array.
4. **Response shape:**
   `answers.<name>` = `{type:"noul", noul}` | `{type:"choice", choice, confidence, probabilities{opt:p}}`
   | `{type:"score", score, confidence, probabilities{"0":p,...}, legend}`; plus
   `usage{input_tokens, output_tokens, cost}`, `model` (dated snapshot, e.g.
   `typesafe/jev-1.13-20260917`), `id`, `provider`.
5. **Batch everything about one state into one request** — questions are answered in
   parallel and cannot see each other. 50 Noul questions cost one ~316 ms call.
6. **Classify errors by HTTP status.** OpenRouter wraps upstream errors as
   `{"error":{"message":"<stringified upstream detail>","code":N}}`:
   - 401 bad/missing key
   - 400 unknown question `type` (zod `invalid_union` detail)
   - 400 `Too many choices. Must have at most 255 choices.`
   - 400 `{"detail":{"error_type":"max_tokens_exceeded"}}` (state > 32k tokens)
   - 400 on `/v1/chat/completions` with the Jev slug ("is a decisions model")

## What to Avoid

- Do not route Jev through `openrouter/*` chat model aliases or the chat client — it 400s.
- Do not assume the chat/embeddings base URL also serves Decisions. On the LiteLLM gateway
  the Decisions route is a **pass-through** at `/openrouter/alpha/decisions` (live since
  2026-09-22), not under `/v1`. Set the decision base URL to
  `https://llm.fzymgc.house/openrouter`; the same `/alpha/decisions` suffix then reaches
  Jev, just as it does from `https://openrouter.ai/api`. Access is per key: a key needs
  `allowed_passthrough_routes: ["/openrouter/alpha/decisions"]` in its metadata. The
  engram, fovea and octopus keys have it; others get 403. The durable grant is in
  selfhosted-cluster PR #2227.
- Do not rely on the error body shape: LiteLLM returns
  `{"error":{message,type,param,code:"401"}}` with a string code, while OpenRouter returns
  `{"error":{message,code:401}}` with a numeric code.
- Do not parse the error `message` text for control flow; it is a stringified upstream body.
- Do not use exact-equality thresholds: identical requests vary ±0.03 on a 0.9 probability.

## Constraints

| Constraint | Value |
|---|---|
| Latency, small request | p50 267 ms · p90 492 ms · p95 498 ms · max 1.1 s (n=30) |
| Latency, larger inputs at concurrency 4 | p50 447 ms · p90 1.6 s · max 3.5 s |
| Choice cardinality | ≤ 255 |
| Context | 32k tokens (state + questions) |
| Cost | input-only; ~$0.00002 per 500-token pair verdict |
| Data policy | no training on inputs (privacy policy); retention standard, ZDR unconfirmed |
| Location | TypeSafe service on US West Coast |

## Origin

Synthesized from spikes: 001, 002
Source files available in: sources/001-jev-openrouter-transport/, sources/002-jev-gateway-passthrough/
