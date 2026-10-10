# Proposal

## Why

The Jev client always sends a request to `{ENGRAM_DECISIONS_BASE_URL}/alpha/decisions` (#793). LiteLLM v1.105.0-rc.3 now serves the same System One request body natively at `/v1/systemone`, through its normal model pipeline (model allow-list, budgets, retries and spend tracking). No base URL turns `/alpha/decisions` into `/v1/systemone`. So engram can reach Jev on that proxy only through a custom pass-through route that needs its own per-key grant. The fzymgc-house deployment moved to v1.105.0-rc.3 on 2026-10-10.

## What Changes

- A new setting, `ENGRAM_DECISIONS_PATH`, names the path that the Jev client appends to `ENGRAM_DECISIONS_BASE_URL`. The default is `/alpha/decisions`, so every existing deployment sends its requests to the same URL as before.
- An operator who uses LiteLLM's native route sets the base URL to the proxy root (for example `https://llm.fzymgc.house`), the path to `/v1/systemone`, and `ENGRAM_DECISIONS_MODEL` to the name of a model configured in LiteLLM.
- Startup validation rejects a path that does not start with `/` (which also rules out a full URL) or that contains `?` or `#`. Like the other per-provider `ENGRAM_DECISIONS_*` settings, the path is checked only when `ENGRAM_DECISIONS_PROVIDER=jev`.
- The request body, the response decoding, the retry policy, the timeouts and the error classes do not change. A request to the native route is the same request.
- The Helm chart exposes `memory.decisions.path`. An empty value sends no variable, so the binary default applies.

## Capabilities

### New Capabilities

- `decision-provider`: how engram reaches the typed-decision provider. This change specifies only the endpoint: how the request URL is built from the base URL and the path, the default path, and the validation of the path setting.

### Modified Capabilities

None. No existing spec covers typed decisions.

## Impact

- **Wire contract:** one new `ENGRAM_DECISIONS_*` key. This is a compatibility decision. The default keeps the URL that every current deployment sends its requests to, so the change is additive and non-breaking. No proto field, MCP input schema, error hint code or stored payload key changes.
- **Code:** `internal/config` (registry row, `DecisionsConfig.Path`, validation), `internal/decide/jev` (a `WithPath` option replaces the fixed `decisionsPath` suffix), `internal/server/decider.go` (passes the path to all three Jev clients: sweep, search re-rank and query understanding).
- **MCP tool schemas:** deliberately unchanged.
- **Connect proto:** deliberately unchanged.
- **CLI:** no new flag. The CLI verbs that build a Jev client (`spine-review consolidate`) read the same setting through `internal/config`.
- **Console (`ui/`):** deliberately unchanged.
- **Helm chart:** new `memory.decisions.path` value and its env wiring in `_helpers.tpl`.
- **docs-site:** `guides/configure.md` (the Typed decisions section: the new row, and a native-route example next to the pass-through example) and `guides/deploy.md` (the chart values table).
- **engram skill:** deliberately unchanged. It describes the memory tools, and none of them changes.
- **CLAUDE.md memory contract:** deliberately unchanged.
- **Dependencies:** none new.
- **Out of scope:** the OpenAI-format `/v1/decisions` route (it takes a different request body and returns answers as a list); any automatic detection of the gateway's shape from the base URL; pricing on the native route; and changes to the fzymgc-house deployment, which go in a separate selfhosted-cluster PR after release.
