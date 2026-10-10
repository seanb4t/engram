# Design

## Context

See proposal.md (Why) for the motivation. The current state:

- `internal/decide/jev` builds its endpoint once in `New`: `strings.TrimRight(baseURL, "/") + decisionsPath`, where `decisionsPath` is the constant `/alpha/decisions`. The package comment and the constant's comment say the suffix is fixed on purpose and is never shape-aware like `internal/openaiurl.Join`.
- `internal/server/decider.go` builds three Jev clients from `config.Config`: `deciderFromConfig` (sweeps), `searchDeciderFromConfig` (search re-rank, no retry) and `understandDeciderFromConfig` (query understanding, no retry). `spine-review consolidate` reaches `deciderFromConfig` through the same config.
- `internal/config` holds eleven `ENGRAM_DECISIONS_*` registry rows. `Config.Validate` checks the provider enum always, and checks every other decisions field only when the provider is `jev`.
- The LiteLLM native route was checked by reading LiteLLM's source at tag `v1.105.0-rc.3` (`litellm/proxy/decisions_endpoints/endpoints.py`, `litellm/llms/base_llm/decisions/transformation.py`, `litellm/decisions/main.py`). `/v1/systemone` accepts the System One body. It routes `model` through the proxy's model list, posts to OpenRouter's `/alpha/decisions`, and rebuilds a System One response. The top-level extra keys (`id`, `provider`) and the extra usage keys (`cost`) pass through. Noul, choice and score answers come back with `confidence`, `probabilities` and `legend`. This is a reading of the code, not a live call. Task 5.1 is the live measurement to take before the deployment switches routes.

## Goals / Non-Goals

**Goals:**

- One endpoint rule for every Jev caller, with a default that leaves the URL of every current deployment unchanged.
- Fail at startup on a path that cannot be right, not at the first decision call.

**Non-Goals:**

- Supporting the OpenAI-format `/v1/decisions` route. It takes a different request body (`input` plus a list of questions) and returns answers as a list, so it would need a second wire codec.
- Detecting the gateway from the base URL.
- Any change to the request body, the response decoding, the retries, the timeouts or the error classes.

## Decisions

### D1: A separate path setting, `ENGRAM_DECISIONS_PATH`

The endpoint is `TrimRight(base, "/") + path`, with the default path `/alpha/decisions`. The fzymgc-house deployment sets base `https://llm.fzymgc.house` and path `/v1/systemone`.

Alternatives considered:

- **`ENGRAM_DECISIONS_API` enum (`openrouter` | `litellm`).** Rejected. It is a closed set, so each new gateway shape needs a code change and a release. A path is the only thing that varies between the two routes, and a path covers both.
- **`ENGRAM_DECISIONS_URL`, a full endpoint that overrides the base URL.** Rejected. It needs a rule for which one wins when both are set, a second URL validation, and documentation for two ways to say the same thing.
- **Shape-aware base URL** (use the base URL as-is when it ends in `/systemone` or `/alpha/decisions`). Rejected. It is a heuristic, and the `jev` package comment rules out shape-aware joins for this lane on purpose.

The user chose D1 on 2026-10-10 (engram memory `y1bs6h75vy`).

### D2: The Jev client takes the path as an option, `WithPath`

`jev.New` keeps its signature. A `WithPath(p string)` option sets the path. An empty `p` is ignored and the `/alpha/decisions` default stays, which matches the existing `WithMaxTimeout` style ("a non-positive value is ignored"). `New` builds `c.endpoint` after the options run. The `decisionsPath` constant stays as the default, and its comment changes from "fixed suffix" to "default suffix".

Alternative considered: a fourth positional argument to `New`. Rejected: it would change about ten call sites and tests for a value that almost every caller leaves at its default.

### D3: Config and validation

- A new registry row `{Key: "decisions.path", Env: "ENGRAM_DECISIONS_PATH", Default: "/alpha/decisions"}`, a new `DecisionsConfig.Path` string, no CLI flag, and no fallback to any other variable. It is a provider-tuning value like the other ten.
- `Config.Validate`, inside the existing `Provider == "jev"` block, rejects a path that does not start with `/` or that contains `?` or `#`. The error names `ENGRAM_DECISIONS_PATH` and quotes the value, in the same form as the base URL errors next to it. A path does not hold a secret, so quoting it is safe.
- Not validated: a trailing slash, `..` segments, and percent-encoding. These are the operator's choice, and the gateway answers a wrong path with a 404, which the client already classifies.
- An explicitly empty value reads back as the default (the registry's existing rule), so the validator never sees an empty path through `config.Load`. A `config.Config` built in code with an empty `Path` still works, because `WithPath("")` keeps the default (D2).

### D4: Wiring

`deciderFromConfig`, `searchDeciderFromConfig` and `understandDeciderFromConfig` each pass `jev.WithPath(cfg.Decisions.Path)`. The opt-in live smoke test (`TestJevLive`, `task eval:decisions`) passes it too, so an operator can run that test against the native route.

### D5: Startup logs stay host-only

The startup log lines (`logUnderstandingEnabled` and its siblings) log `endpoint_host` only, never the path, userinfo or query. This change leaves them as they are. The path is not sensitive, but the host is the part that matters for the disclosure ("query text is sent to <host>"), and the path is in the configuration the operator wrote.

### D6: Helm chart

A new `memory.decisions.path` value (default `""`). The `_helpers.tpl` decisions block renders `ENGRAM_DECISIONS_PATH` with the `with` guard that `baseURL`, `model` and `timeout` use, inside the existing `provider` guard. An empty value renders nothing, so the binary default applies. The `values.yaml` comment on `baseURL` stops saying that engram always appends `/alpha/decisions`, and shows the native-route form next to the pass-through form.

## Risks / Trade-offs

- [The native route's response differs from OpenRouter's in a field that engram decodes] → The source reading in Context says it does not. Task 5.1 runs the live smoke test against the native route before the deployment switches. The pass-through route keeps working, so the deployment can stay on it until that test passes.
- [An operator sets the path but leaves `ENGRAM_DECISIONS_MODEL` at the OpenRouter slug `typesafe/jev-1.13`] → LiteLLM rejects a model that is not in its model list with an error that the client already reports as a bad request. The configure guide states that the native route needs a model name configured in LiteLLM.
- [LiteLLM requires `confidence` on choice and score answers] → If the upstream ever leaves it out, LiteLLM fails the request and the client reports a provider error, not a wrong answer. A decision failure never fails the operation that asked for it.

## Migration Plan

None is needed. The default keeps the URL of every current deployment. To use the native route, an operator sets three values: the base URL (proxy root), the path (`/v1/systemone`) and the model (the LiteLLM model name). The key also needs the model in its allow-list instead of the pass-through route grant. To roll back, unset the path and restore the old base URL and model.

## Open Questions

- How LiteLLM prices a call on the native route, and whether `usage.cost` then reports LiteLLM's figure or OpenRouter's. This changes only what the existing telemetry shows, not the design.
