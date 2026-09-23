#!/usr/bin/env bash
# Spike 002: does the LiteLLM gateway expose any route to OpenRouter's Decisions API?
# Unauthenticated POSTs: LiteLLM answers an unknown route 404 before auth, a known
# route 401 — so no key is needed to map the surface.
set -euo pipefail
GW=${GW:-https://llm.fzymgc.house}
for p in /v1/models /alpha/decisions /api/alpha/decisions /v1/alpha/decisions \
         /openrouter/alpha/decisions /openrouter/api/alpha/decisions \
         /openrouter/api/v1/systemone /v1/systemone; do
  code=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$GW$p" \
    -H 'Content-Type: application/json' -d '{}' --max-time 10)
  printf '%-36s %s\n' "$p" "$code"
done
