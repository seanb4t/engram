# Phase 2: Decision Interface & Jev Backend - Context

**Gathered:** 2026-09-23
**Status:** Ready for planning

<domain>
## Phase Boundary

engram gains a provider-neutral, advisory-only typed-decision client, inert until an operator
opts in, with Jev (via OpenRouter's Decisions API) as the first backend (DEC-01..DEC-06):
config and enablement, a Go interface in System One vocabulary (shared state + batched
Choice/Score/Noul questions → typed probabilities/confidence), a Jev backend that works against
OpenRouter directly and through the LiteLLM pass-through, bounded calls with status-classified
named errors, an SDK evaluation recorded before any hand-written client, and OTLP spans.

Not in this phase: any caller of the interface (Phase 3 curation verdicts, Phase 4 reranker),
the chat-LLM emulator backend (DEC-F2), write-time hints (DEC-F1).

</domain>

<decisions>
## Implementation Decisions

### Config & enablement
- **D-01:** Enablement is a **provider enum**: `ENGRAM_DECISIONS_PROVIDER` = `""` (default, off)
  | `"jev"`. When unset, no decision client is constructed and no decision call is ever made —
  behavior and outbound calls byte-identical to today (DEC-01). The enum leaves room for the
  emulator backend (DEC-F2) without another toggle. — **Reversibility:** costly — a registered
  operator config key, documented surface.
- **D-02:** Settings use the capability-named prefix **`ENGRAM_DECISIONS_*`**: `_BASE_URL`,
  `_API_KEY`, `_MODEL` (default `typesafe/jev-1.13`, pinned), `_TIMEOUT`, plus drain/bound knobs
  following the existing `ENGRAM_{EMBED,SUMMARY}_{DRAIN_BYTES,DRAIN_TIMEOUT,MAX_TIMEOUT}` pattern,
  and `_CONCURRENCY` (D-08). All registered in `internal/config/registry.go`.
- **D-03:** Fallback semantics: **the API key falls back** to `ENGRAM_OPENAI_API_KEY`; **the base
  URL does not fall back.** With `provider=jev` and no `ENGRAM_DECISIONS_BASE_URL`, config
  validation fails via `Config.Validate()`'s existing style (`fmt.Errorf("ENGRAM_VAR %q: ...")` joined
  with `errors.Join`; the field+hint envelope is the MCP/Connect tool-call grammar only — corrected
  after research). Rationale: `ENGRAM_OPENAI_BASE_URL` is
  the LiteLLM `/v1` root, and the spike findings forbid assuming the chat gateway serves
  Decisions. The path `/alpha/decisions` is appended to the base (OpenRouter:
  `https://openrouter.ai/api`; LiteLLM pass-through: `https://llm.fzymgc.house/openrouter`).
  This interprets DEC-03's "falling back to the shared OpenRouter values" as key-only.
- **D-04:** Surfaces: **Helm chart values** (off by default) and the **docs-site config
  reference** are updated in this phase; `engram setup` is unchanged (it configures clients, not
  the server).

### SDK evaluation (DEC-05)
- **D-05:** Adoption bar for OpenRouter's Go SDK: **(a) maintained upstream with a current,
  non-deprecated module path** (recent releases/commits), and **(b) full Decisions surface** —
  choice/score/noul questions, usage/cost fields, typed answers without `map[string]any` escape
  hatches.
- **D-06:** If the SDK passes D-05 but **cannot meet DEC-04 (bounded time/bytes/drain,
  status-classified errors) or DEC-06 (OTLP spans)**, the plan stops at a **decision checkpoint**
  and asks the user (adopt-and-wrap vs reject). If it fails D-05, reject and hand-write on
  `net/http` following `internal/embed` / `internal/summarize`. — **Reversibility:** costly — a
  new module dependency (prior milestones held "zero new Go dependencies"; adopting is a
  deliberate, recorded exception).
- **D-07:** The evaluation is recorded as a **phase doc committed before any client code**, plus
  a **durable engram decision record** (spine) so later work doesn't re-evaluate.

### Interface shape
- **D-08:** Packages: **`internal/decide`** holds the `Decider` interface and types (`State`,
  `Question` with Choice/Score/Noul variants, `Answer`, `Usage`, named errors); **`internal/decide/jev`**
  holds the Jev backend. A future `internal/decide/emulate` slots in beside it.
- **D-09:** **Cheap structural validation** client-side, returning named errors with no network
  call: >255 choices, empty criteria, duplicate question names, unknown question type. **No token
  counting** — the 32k-context limit is left to callers (Phase 4 truncates) and to the server's
  `max_tokens_exceeded` 400.
- **D-10:** Call shape: **`Decide(ctx, Request{State, Questions map[name]Question}) (Response, error)`**
  (one state, many questions per call — batch everything about one state) **plus a
  `DecideMany` helper** over many states with a bounded worker pool, returning **per-item
  `[]Result{Response, Err}` in input order**; one item's failure never aborts the others.
  Concurrency from `ENGRAM_DECISIONS_CONCURRENCY` (default 4, the spike's measured c=4).

### Failure & telemetry
- **D-11:** **One retry on 429/5xx** (jittered), still inside the overall timeout budget; no
  retry on 4xx other than 429, timeouts, or response-too-large.
- **D-12:** Named errors **by status class**, all `errors.Is`-able: `ErrDecisionAuth` (401/403),
  `ErrDecisionBadRequest` (400, carrying upstream detail), `ErrDecisionContextTooLarge` (400
  `max_tokens_exceeded`, detected structurally where possible), `ErrDecisionRateLimited` (429),
  `ErrDecisionUnavailable` (5xx/transport), `ErrDecisionTimeout`, and the shared
  response-too-large bound. Classification never parses the error `message` text for control flow;
  both OpenRouter (`code` numeric) and LiteLLM (`code` string) bodies are handled. A decision
  failure never fails the surrounding read or sweep — that is the caller contract, documented on
  the interface.
- **D-13:** Telemetry: a **`decide` span** per call with attributes
  `engram.decide.{provider, model, model_snapshot, questions, input_tokens, output_tokens,
  cost_usd, status}` (latency is the span duration) plus a **debug-level slog line**. **No new
  metrics instruments** in this phase.

### Claude's Discretion
- Exact Go type/field names beyond those listed; default values for timeout/drain knobs (follow
  the embed/summarize defaults unless the spike latency data argues otherwise).
- Retry jitter/backoff values within the D-11 constraints.
- How tests fake the Decisions endpoint (httptest server fixtures from the spike's verified
  request/response and error shapes).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope
- `.planning/ROADMAP.md` §"Phase 2: Decision Interface & Jev Backend" — goal and success criteria
- `.planning/REQUIREMENTS.md` — DEC-01..DEC-06, DEC-F2 (deferred)

### Spike blueprint
- `.claude/skills/spike-findings-engram/SKILL.md` — findings index
- `.claude/skills/spike-findings-engram/references/decision-transport.md` — verified request/response shapes, error bodies (OpenRouter vs LiteLLM), latency/cardinality/context constraints, LiteLLM pass-through route and per-key grant
- `.planning/spikes/001-jev-openrouter-transport/`, `.planning/spikes/002-jev-gateway-passthrough/` — spike sources

### Code patterns
- `internal/embed/embed.go`, `internal/summarize/summarize.go` — provider client pattern (New + Options, bounded reads)
- `internal/httpdrain/` — shared byte/time-bounded drain
- `internal/config/registry.go` — ENGRAM_ field registry (single source of truth); drain/timeout key pattern
- `internal/telemetry/` — OTel setup and slog bridge
- `charts/engram/` — Helm values
- docs-site config reference and `reference/errors.md` — field+hint error envelope

### External
- OpenRouter Go SDK docs: openrouter.ai/docs/client-sdks/go/sdks/decisions (evaluate per D-05)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/httpdrain`: bounded drain for response bodies (DEC-04).
- `internal/embed` / `internal/summarize`: client constructor + Option pattern, timeout/max-timeout
  resolution, error-body truncation.
- `otelhttp` and `go.opentelemetry.io/otel` are already dependencies — spans need no new module.

### Established Patterns
- Config: koanf, env-first `ENGRAM_` prefix, registry-driven docs/validation gates; errors use the
  `field=<name> hint=<code>` envelope.
- A zero request timeout resolves to a configurable ceiling, never unbounded (from milestone
  2026-09-18.01).

### Integration Points
- Server/CLI wiring constructs the decider only when `provider != ""` (D-01); Phase 3's
  `spine-review consolidate` and Phase 4's `SearchReranked` → `rankCandidates` seam are the future
  consumers.

</code_context>

<specifics>
## Specific Ideas

- Test fixtures should mirror the spike's verified shapes, including both error-body dialects
  and the `max_tokens_exceeded` detail.
- Probability comparisons must never use exact equality (identical requests vary ±0.03).

</specifics>

<deferred>
## Deferred Ideas

- Metrics instruments (calls/latency/tokens/cost counters) — considered for D-13, deferred.
- Client-side token estimation — considered for D-09, deferred (Phase 4 owns truncation).

</deferred>

---

*Phase: 02-decision-interface-jev-backend*
*Context gathered: 2026-09-23*
