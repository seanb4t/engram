---
spike: 002
idea: jev-typed-decisions
name: jev-gateway-passthrough
type: standard
validates: "Given the in-cluster LiteLLM gateway (llm.fzymgc.house), when a Decisions request is sent through it, then it reaches OpenRouter's /api/alpha/decisions with typed probabilities intact"
verdict: VALIDATED
related: [001]
tags: [jev, litellm, gateway, llm-fzymgc-house, pass-through, transport]
---

# Spike 002: Jev through the LiteLLM gateway

## What This Validates

Given the cluster's LLM gateway, when engram sends a Decisions request through it, then the
request reaches Jev and the typed answers come back unchanged, so engram can keep a single
egress point.

## Research

- **Which gateway.** The live gateway is **LiteLLM v1.96.2 at `llm.fzymgc.house`**
  (selfhosted-cluster `docs/reference/services.md`,
  `argocd/app-configs/litellm-chart/values.yaml`). Older memories name `llm-gw.fzymgc.house`
  (the agentgateway era); that hostname is out of date.
- It publishes only the OpenAI surface (`/v1/chat/completions`, `/v1/embeddings`,
  `/v1/models`) plus MCP at `/<server>/mcp`. Its `model_list` routes `openrouter/*` through
  `OPENROUTER_API_KEY` over the **chat** path, which spike 001 showed Jev rejects (400).
- LiteLLM's generic **`general_settings.pass_through_endpoints`** can forward a custom path to
  an arbitrary upstream with server-side headers (docs.litellm.ai/docs/proxy/pass_through,
  via Context7), and can report cost headers (`x-litellm-response-cost`). No such entry
  exists in the cluster's values today.

## How to Run

```sh
.planning/spikes/002-jev-gateway-passthrough/probe.sh   # GW=... to override
```

## What to Expect

One status code per candidate path. `404` from LiteLLM (JSON `{"detail":"Not Found"}`,
returned before authentication) means no route; `401` or `405` means a route exists.

## Investigation Trail

1. Asked for the gateway URL; found it through engram memory plus the cluster repo, and
   corrected the hostname (llm-gw → llm).
2. Read the LiteLLM chart values: no pass-through entries, OpenRouter reached only through
   the chat-shaped `openrouter/*` model_list entries.
3. Probed, unauthenticated, all plausible Decisions/System One paths: every one returns
   LiteLLM's own 404, while `/v1/models` returns 405 (a real route, wrong method). So the
   absence is LiteLLM's route table, not auth or ingress.
4. Checked LiteLLM docs for a sanctioned way to add the route → `pass_through_endpoints`.

## Results

**PARTIAL** — Jev is **not reachable through the gateway today**, but there is a sanctioned
LiteLLM mechanism that would make it reachable without custom code:

```yaml
general_settings:
  pass_through_endpoints:
    - path: "/openrouter/alpha/decisions"
      target: "https://openrouter.ai/api/alpha/decisions"
      headers:
        Authorization: "bearer os.environ/OPENROUTER_API_KEY"
        content-type: application/json
```

Open points before relying on it (none verified here):

- **Per-consumer auth on the pass-through route.** Whether requiring a LiteLLM virtual key
  on a pass-through route (`auth: true`) is available without a LiteLLM Enterprise license
  — **unresolved**; the fetched docs show enterprise gating for related custom-auth
  features but did not settle this one. Without per-key auth the route would spend the
  cluster's OpenRouter key for anyone who can reach it.
- Whether cost tracking and OTLP spans attach to pass-through calls the way they do on
  `/v1` — **unverified**.
- It is a change to the selfhosted-cluster repo (a LiteLLM values PR), not to engram.

**Impact on engram's design:** the decision client must not assume the chat/embeddings base
URL also serves Decisions. It needs its own base-URL key (for example
`ENGRAM_DECISIONS_BASE_URL`, falling back to the shared OpenRouter base URL), so an operator
can point it at OpenRouter directly or at a gateway pass-through path.

## Update — 2026-09-22: gateway route live → VALIDATED

The gateway now serves Jev. The user added a LiteLLM pass-through route and granted it to
three keys:

- **Route:** `POST https://llm.fzymgc.house/openrouter/alpha/decisions`. Callers send their
  existing LiteLLM virtual key and `"model": "typesafe/jev-1.13"`.
- **Per-key access:** each key's metadata carries
  `allowed_passthrough_routes: ["/openrouter/alpha/decisions"]`. The engram, fovea and
  octopus keys are granted and each got a 200 with a decision (the user's live test).
  Keys without the grant, such as `openrouter-passthrough`, get 403. This settles the
  earlier open point: per-key authorization on the pass-through route works on this
  deployment without spending the cluster key for everyone.
- **Durable config:** selfhosted-cluster PR #2227 adds the grant to
  `scripts/seed-litellm-vault.sh` and `docs/operations/litellm.md`, so re-creating a key
  keeps its Jev access.
- **Re-probed here without a key:** `/openrouter/alpha/decisions` → 401
  `{"error":{"message":"Authentication Error, No api key passed in.","type":"auth_error","param":"None","code":"401"}}`;
  a bad key → 401 "Invalid proxy server token"; `/alpha/decisions` → still 404. This shell
  has no engram LiteLLM key, so the 200 path rests on the user's test.

Implications for the client:

- The base URL `https://llm.fzymgc.house/openrouter` plus the same `/alpha/decisions` suffix
  reaches Jev. The same join works for OpenRouter directly (`https://openrouter.ai/api`).
- Error bodies differ by hop. LiteLLM sends `{"error":{message,type,param,code:"401"}}`
  (code is a string); OpenRouter sends `{"error":{message,code:401}}` (code is a number).
  Classify by HTTP status only.
