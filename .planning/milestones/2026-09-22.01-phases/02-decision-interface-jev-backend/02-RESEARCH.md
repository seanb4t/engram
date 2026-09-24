# Phase 2: Decision Interface & Jev Backend - Research

**Researched:** 2026-09-23
**Domain:** Go HTTP client library design (provider-neutral typed-decision interface), OpenRouter Decisions API transport, engram config/Helm/OTel conventions
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Config & enablement
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
  validation fails with the standard field+hint envelope. Rationale: `ENGRAM_OPENAI_BASE_URL` is
  the LiteLLM `/v1` root, and the spike findings forbid assuming the chat gateway serves
  Decisions. The path `/alpha/decisions` is appended to the base (OpenRouter:
  `https://openrouter.ai/api`; LiteLLM pass-through: `https://llm.fzymgc.house/openrouter`).
  This interprets DEC-03's "falling back to the shared OpenRouter values" as key-only.
  **Research correction: see "Architecture Patterns — Pattern 2" below — `Config.Validate()`
  does not actually use a `field=/hint=` envelope anywhere; that grammar is MCP/Connect-only. The
  planner should follow `Config.Validate()`'s real, plain `fmt.Errorf("ENGRAM_VAR %q: ...")`
  style instead.**
- **D-04:** Surfaces: **Helm chart values** (off by default) and the **docs-site config
  reference** are updated in this phase; `engram setup` is unchanged (it configures clients, not
  the server).

#### SDK evaluation (DEC-05)
- **D-05:** Adoption bar for OpenRouter's Go SDK: **(a) maintained upstream with a current,
  non-deprecated module path** (recent releases/commits), and **(b) full Decisions surface** —
  choice/score/noul questions, usage/cost fields, typed answers without `map[string]any` escape
  hatches. **Research finding: both (a) and (b) verified true — see Summary/Standard Stack
  below.**
- **D-06:** If the SDK passes D-05 but **cannot meet DEC-04 (bounded time/bytes/drain,
  status-classified errors) or DEC-06 (OTLP spans)**, the plan stops at a **decision checkpoint**
  and asks the user (adopt-and-wrap vs reject). If it fails D-05, reject and hand-write on
  `net/http` following `internal/embed` / `internal/summarize`. — **Reversibility:** costly — a
  new module dependency (prior milestones held "zero new Go dependencies"; adopting is a
  deliberate, recorded exception). **Research finding: a concrete DEC-04 risk exists (LiteLLM's
  string `code` vs. the SDK's typed `int64` field) — this checkpoint is live, not hypothetical.
  See Common Pitfalls #1 and Open Question 1.**
- **D-07:** The evaluation is recorded as a **phase doc committed before any client code**, plus
  a **durable engram decision record** (spine) so later work doesn't re-evaluate.

#### Interface shape
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

#### Failure & telemetry
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
- **Research adds:** whether `deciderFromConfig` is wired into `buildDepsFromEnv`'s `deps` struct
  this phase or left as a standalone constructor (see Architecture Patterns Pattern 3); whether
  a response-byte-cap gets its own `ENGRAM_DECISIONS_MAX_RESPONSE_BYTES` registry row or follows
  the embed/summarize precedent of an internal-only constant (see Common Pitfalls #4).

### Deferred Ideas (OUT OF SCOPE)
- Metrics instruments (calls/latency/tokens/cost counters) — considered for D-13, deferred.
- Client-side token estimation — considered for D-09, deferred (Phase 4 owns truncation).
- Any caller of the interface (Phase 3 curation verdicts, Phase 4 reranker), the chat-LLM
  emulator backend (DEC-F2), write-time hints (DEC-F1).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|---------------------|
| DEC-01 | Operator can enable a typed-decision provider through `ENGRAM_` config; off by default, no decision call made and behavior byte-identical when off. | `internal/server/tools.go`'s `summarizerFromConfig`/`StoreAndSummarizerFromEnv` gated-construction precedent (Architecture Patterns Pattern 3); registry row pattern with empty `Default` (Code Examples). |
| DEC-02 | Provider-neutral Go interface, System One contract (state + batched Choice/Score/Noul in; typed probabilities/confidence out), so a second backend can be added without changing callers. | OpenRouter SDK's `components.Questions`/`components.Answers` discriminated unions confirm the target shape is representable with fully typed Go structs (Standard Stack, Summary). |
| DEC-03 | Jev backend calls `{base}/alpha/decisions` with its own base-URL/key/model/timeout, each falling back to shared OpenRouter values per D-03, works against OpenRouter direct and LiteLLM pass-through. | Spike-verified request shape and dual-transport reachability (`decision-transport.md`); `ChatAPIKey`/`ChatBaseURL` `cmp.Or`-at-wiring-seam precedent (Architecture Patterns Pattern 3, Open Question 2). |
| DEC-04 | Decision calls bounded (timeout, bytes, drain); failures classified by HTTP status into named errors (OpenRouter vs. LiteLLM dialects differ); a decision failure never fails the surrounding read/sweep. | `internal/httpdrain` reuse pattern; SDK's per-status typed errors PLUS the LiteLLM `int64`-vs-string `code` risk (Common Pitfalls #1, Open Question 1) — this is the crux of the D-06 checkpoint. |
| DEC-05 | OpenRouter Go SDK evaluated (maintained upstream, current module path) and adopted/rejected with rationale recorded, before any hand-written client. | Full SDK evaluation performed this session: release cadence, Go-version compat, dependency footprint, typed-answer coverage, custom-HTTP-client support (Summary, Standard Stack, Package Legitimacy Audit). |
| DEC-06 | Decision calls emit OTLP spans carrying latency, question count, input tokens, cost. | `internal/embed/embed_test.go`'s `tracetest.SpanRecorder` pattern, directly reusable (Code Examples); response-decode-ownership caveat if the SDK is adopted (Common Pitfalls #2). |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

Directives from `./CLAUDE.md` applicable to this phase's implementation, for the planner to
verify compliance against:

- **SPDX headers:** every new `internal/decide/**/*.go` file needs the Apache-2.0 SPDX header
  (`task license:check`/`task license:add`) — these are ordinary `.go` files, not one of the
  frontmatter-exempt paths (`.planning/**`, `skill/**/SKILL.md`, docs-site).
- **Conventional Commits:** required, PR-title-validated in CI.
- **Task runner:** `task` = lint + test; run before considering any plan's tasks complete.
- **Config loading:** `internal/config` (koanf), env-first `ENGRAM_` prefix with `--flag`
  overrides — `ENGRAM_DECISIONS_*` has no `--flag` per the existing provider-tuning-value
  precedent (`embed.drain_bytes`/`summarize.drain_bytes` have none either); no viper.
- **Not used here:** viper, cocogitto — do not introduce either.
- **Migrations:** N/A this phase — no payload/schema evolution, no new `internal/migrate` step.
- **Memory contract:** the D-07 durable engram decision record (SDK adopt/reject) should be
  captured via `store_memory`/`store_rule` per the `curating-memory` skill's routing gate, not a
  markdown TODO or a `bd remember` call (Beads is retired in this repo).
- **CLAUDE.md Layout table:** currently lists `internal/server/`, `internal/store/`,
  `internal/embed/`, `internal/auth/`, `internal/config/`, `charts/engram/`, `proto/`, `gen/` —
  it does NOT yet list `internal/decide/`. The planner should consider whether adding a Layout
  row for `internal/decide/` (mirroring the existing `internal/embed/` row) belongs in this
  phase's scope or a documentation follow-up; not adding it is not a defect, but a stale/missing
  Layout entry is worth flagging either way.

## Summary

Phase 2 adds a new, currently-uncalled `internal/decide` package plus `internal/decide/jev`
backend, following the exact client-construction/config/OTel shape `internal/embed` and
`internal/summarize` already establish in this codebase — there is no green-field design
choice here beyond the System One interface itself. `internal/decide`, `internal/decide/jev`,
`ENGRAM_DECISIONS_*`, and every wire shape (request/response/error dialects) are new: nothing
exists in the tree today (`rg` confirms zero hits for `decide`, `Decisions`,
`ENGRAM_DECISIONS`).

The load-bearing finding is the **DEC-05 SDK evaluation**. I fetched the OpenRouter Go SDK
(`github.com/OpenRouterTeam/go-sdk`) directly from Context7 and GitHub raw source (not
training data): it shipped 5 releases in the 24 hours before this research (2026-09-21/22),
requires Go 1.25.10 (engram is on 1.26.7 — compatible), is Apache-2.0, and adds **exactly one**
genuinely new transitive dependency (`github.com/spyzhov/ajson`) — `testify` and
`go.yaml.in/yaml/v3` are already in engram's `go.sum`. Its `Alpha.Decisions.Create` method
covers Choice/Score/Noul questions and answers as fully typed Go structs (no
`map[string]any` escape hatch for known types — only a documented `GetUnknownRaw()` fallback
for genuinely unrecognized discriminator values), and it accepts a caller-supplied HTTP client
via `WithClient()` (a minimal `Do(req) (*http.Response, error)` interface, satisfied by any
`*http.Client`, so `otelhttp.NewTransport` composes normally). **This clears D-05(a) and
D-05(b) cleanly — recommend ADOPT.**

The catch, which the plan MUST record as the D-06 checkpoint: the SDK's per-status error types
(`sdkerrors.BadRequestResponseError`, `TooManyRequestsResponseError`, etc. — twelve of them for
`Decisions.Create` specifically, plus a generic `sdkerrors.APIError` fallback with a `StatusCode
int` field) decode their `code` field as **`int64`**, matching OpenRouter's own numeric-code
error dialect. The spike's own verified finding is that the **LiteLLM pass-through returns
`code` as a JSON *string*** (`"401"`, not `401`). A string value cannot unmarshal into an
`int64` field. I could not run a live call in this research session to observe the concrete
failure mode (decode error surfacing as an opaque wrapped error vs. silent fallback to the
generic `APIError`), so this is a **flagged, untested integration risk**, not a disqualifying
defect — but it is exactly the DEC-04 "status-classified errors, LiteLLM dialect included"
requirement the D-06 checkpoint exists to catch. The Wave 0 task for whichever backend gets
built MUST include an httptest fixture replaying the spike's captured LiteLLM error body against
the SDK before any other code is written, and record the observed decode behavior as the D-07
phase doc.

A second, non-blocking integration note for the same checkpoint: the SDK owns the full response
decode internally (callers get back a typed `*components.DecisionsResponse`, not a raw
`http.Response`), so byte-bounding a response (D-04's response-too-large bound) cannot use
`internal/embed`/`internal/summarize`'s current pattern (read into a bound, `httpdrain.Drain`
the rest) — it requires a custom `http.RoundTripper` that wraps `resp.Body` in an
`io.LimitReader` (and drains/closes on overflow) *before* handing the response back to the SDK's
own decode path. This is straightforward to write but is new code the current embed/summarize
lineage doesn't need, and belongs in the D-06 checkpoint write-up alongside the LiteLLM
finding.

**Primary recommendation:** Mirror `internal/embed`/`internal/summarize`'s New+Option+registry
pattern exactly for `internal/decide/jev`; adopt the OpenRouter Go SDK per D-05 findings above,
but gate that adoption on a Wave-0 spike task that replays the LiteLLM error-body fixture through
the SDK and records the D-06 checkpoint outcome (adopt-and-wrap vs. reject-and-hand-roll) before
any production `jev.go` is written.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `ENGRAM_DECISIONS_*` config parsing/validation | API / Backend (`internal/config`) | — | Same tier as every other `ENGRAM_` var; koanf registry is the single source of truth, read once at startup. |
| Decider interface + typed request/response (`internal/decide`) | API / Backend | — | A pure Go contract package, no I/O; consumed by future server-tier callers (Phase 3/4), never by a client or browser tier. |
| Jev HTTP transport (`internal/decide/jev`) | API / Backend | External Service (OpenRouter/LiteLLM) | Outbound HTTP client exactly like `internal/embed`/`internal/summarize` — server-tier code calling an external service, never client-tier. |
| Bounded read/drain (byte + time) | API / Backend | — | `internal/httpdrain`, reused verbatim; enforced at the HTTP client boundary inside `jev.go`, not in the interface layer. |
| OTLP span emission | API / Backend | Observability (OTel Collector) | `internal/telemetry`'s tracer/meter setup already runs server-side; the `decide` span is emitted from `jev.Decide`, exported via the existing OTel pipeline. |
| Helm value exposure (secret ref + settings) | Deployment / Chart | — | `charts/engram/values.yaml` + `_helpers.tpl`, mirrors `memory.summarize`/`memory.openai` — a deployment-topology concern, not application code. |
| Server/CLI wiring point for constructing a decider | API / Backend (`internal/server/tools.go: buildDepsFromEnv`) | — | Same seam `embedderFromConfig`/`summarizerFromConfig` already use; this phase adds the constructor, a future phase adds the caller. |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|---------------|
| `github.com/OpenRouterTeam/go-sdk` | `v0.8.19` (latest at research time, 2026-09-22) [CITED: pkg.go.dev/github.com/OpenRouterTeam/go-sdk?tab=versions] | Typed Go client for OpenRouter's `/api/alpha/decisions` endpoint — `Alpha.Decisions.Create` | Official SDK for the exact API this phase talks to; DEC-05 mandates evaluating it before hand-rolling. See "Package Legitimacy Audit" and "SDK Evaluation" below for the full D-05/D-06 record. `[ASSUMED]`-package-name caveat: discovered via WebSearch/Context7, not previously present in engram's `go.sum` — tag the concrete `go get` version pin as `[ASSUMED]` until re-verified at implementation time (SDK ships multiple releases per day). |
| `go.opentelemetry.io/otel` + `otelhttp` | already a dependency | Span creation (`tracer.Start`), `WithHTTPTransport`-style transport wrapping | Zero new module — `internal/embed`/`internal/summarize` already import it this exact way `[VERIFIED: internal/embed/embed.go:21-25, internal/summarize/summarize.go:22-25]`. |
| `internal/httpdrain` | in-repo | Bounded (bytes + time) response-body drain, reused verbatim | The one shared drain mechanism both existing provider clients call; D-02 in its own doc comment states this explicitly `[VERIFIED: internal/httpdrain/httpdrain.go:1-14]`. |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/spyzhov/ajson` | `v0.8.0` (SDK's own pin) [CITED: raw.githubusercontent.com/OpenRouterTeam/go-sdk/main/go.mod] | Dynamic JSON AST manipulation used internally by the SDK's discriminated-union (Questions/Answers) codegen | Transitive only — not called directly by engram code. The **only genuinely new** dependency this phase introduces if the SDK is adopted; `testify` and `go.yaml.in/yaml/v3` (the SDK's other two deps) are already indirect deps of engram today `[VERIFIED: go.mod:44,135,163]`. |
| `github.com/OpenRouterTeam/go-sdk/models/sdkerrors` | bundled with the SDK | Per-HTTP-status typed error structs for `Decisions.Create` (12 named types + generic `APIError`) | Only if D-06 resolves ADOPT. See Common Pitfalls for the LiteLLM-dialect decode risk before relying on these types for status classification. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| OpenRouter Go SDK | Hand-rolled `net/http` client (`internal/embed`/`internal/summarize` pattern) | Full control over byte-bounding and error-body parsing (both lanes uniformly), zero new dependency, but re-implements request/response marshaling the SDK already gets right, and diverges from D-05's own instruction to evaluate the SDK first. D-06 is the formal fork point — record the decision, don't silently pick one. |
| `go-sdk`'s typed per-status errors | Bypass typed errors entirely; read `resp.StatusCode` + raw body via the wrapping RoundTripper (same shape as embed/summarize's own error path) even when using the SDK for request/response marshaling | Sidesteps the `int64`-vs-string `code` decode risk entirely by classifying on the transport-level status code before the SDK gets a chance to decode the error body into a typed struct. Worth considering as the "adopt-and-wrap" shape the D-06 checkpoint should weigh. |

**Installation (only if D-06 resolves ADOPT):**
```bash
go get github.com/OpenRouterTeam/go-sdk@latest
```

**Version verification performed:** `pkg.go.dev/github.com/OpenRouterTeam/go-sdk?tab=versions`
showed `v0.8.12`–`v0.8.19` all published 2026-09-21/22 (the 24h window before this research),
each a Speakeasy-generated "spec change" release — extremely active, current, non-deprecated.
`go.mod` (fetched live) pins `go 1.25.10`; engram's own `go.mod` is `go 1.26.7`
`[VERIFIED: go.mod:3]` — compatible. Re-run `go list -m -versions github.com/OpenRouterTeam/go-sdk`
at implementation time since this module ships multiple releases per day.

## Package Legitimacy Audit

> This phase's only candidate external package is a Go module. `gsd-tools query
> package-legitimacy check` only supports `--ecosystem npm|pypi|crates` — Go is unsupported, so
> the automated seam could not be run. Legitimacy was instead verified manually against
> authoritative sources:

| Package | Registry | Age | Activity | Source Repo | Verdict | Disposition |
|---------|----------|-----|----------|--------------|---------|-------------|
| `github.com/OpenRouterTeam/go-sdk` | Go module proxy (proxy.golang.org) | SDK repo itself is not brand-new (established, versioned since well before v0.8.x); confirmed via `pkg.go.dev` version history | 5 releases in 24h before research (2026-09-21/22) [CITED: pkg.go.dev] | `github.com/OpenRouterTeam/go-sdk` — matches the org name OpenRouter's own docs link to (`openrouter.ai/docs/client-sdks/go`) [CITED: openrouter.ai/docs/client-sdks/go/sdks/decisions] | Manually verified OK (tool unsupported for Go) | Approved for the D-05/D-06 evaluation; still `[ASSUMED]` on the exact pinned version until `go get` is run live at implementation time |
| `github.com/spyzhov/ajson` | Go module proxy | Transitive only, pulled in by the SDK | Not independently audited — inherited dependency, never imported directly by engram code | n/a (transitive) | Not separately verified (inherited, non-direct) | No action — carried automatically if the SDK is adopted |

**Packages removed due to SLOP verdict:** none.
**Packages flagged as suspicious [SUS]:** none — but the pinned SDK version itself should be
re-confirmed live at implementation time (`go list -m -versions`) given the release cadence
observed, and the planner should add a `checkpoint:human-verify` before `go get` regardless,
consistent with the D-06 decision-checkpoint requirement already in CONTEXT.md.

## Architecture Patterns

### System Architecture Diagram

```
                    ┌─────────────────────────────────────────┐
                    │  internal/server (future caller, Ph 3/4) │
                    │  buildDepsFromEnv() — NOT wired to call  │
                    │  Decide() this phase; only constructs    │
                    └───────────────┬───────────────────────────┘
                                    │ (construction only, D-01 gate)
                                    ▼
                    ┌─────────────────────────────────────────┐
                    │  internal/decide                         │
                    │  Decider interface, Request/Response,    │
                    │  Question (Choice/Score/Noul), named     │
                    │  errors — pure Go, no I/O                │
                    └───────────────┬───────────────────────────┘
                                    │ implements
                                    ▼
                    ┌─────────────────────────────────────────┐
                    │  internal/decide/jev                     │
                    │  1. cheap structural validation (D-09,   │
                    │     no network call)                     │
                    │  2. build request (OpenRouter SDK or     │
                    │     hand-rolled net/http, per D-06)      │
                    │  3. tracer.Start("decide") span           │
                    │  4. HTTP call, bounded by timeout/        │
                    │     max_timeout/drain (httpdrain)         │
                    │  5. one retry on 429/5xx (jittered)       │
                    │  6. classify error by HTTP status class   │
                    │     -> named error (ErrDecisionAuth, ...) │
                    │  7. span.SetAttributes(usage, cost, ...)  │
                    └───────────────┬───────────────────────────┘
                                    │ HTTPS POST {base}/alpha/decisions
                                    ▼
        ┌──────────────────────────┴──────────────────────────┐
        ▼                                                       ▼
┌────────────────────┐                            ┌─────────────────────────┐
│ OpenRouter direct   │                            │ LiteLLM pass-through     │
│ openrouter.ai/api   │                            │ llm.fzymgc.house/       │
│ error: {code:int}   │                            │ openrouter               │
└──────────┬──────────┘                            │ error: {code:"string"}   │
           │                                        └──────────┬───────────────┘
           └───────────────────┬────────────────────────────────┘
                                ▼
                    ┌─────────────────────────────────────────┐
                    │  Jev (TypeSafe System One model)         │
                    │  typed answers + usage{tokens,cost}      │
                    └─────────────────────────────────────────┘
```

### Recommended Project Structure
```
internal/
├── decide/
│   ├── decide.go        # Decider interface, State, Question (Choice/Score/Noul), Answer, Usage, named errors
│   ├── decide_test.go    # interface-level contract tests (e.g. DecideMany ordering, partial-failure semantics)
│   └── jev/
│       ├── jev.go         # New()+Option, Decide/DecideMany, HTTP transport, span, error classification
│       ├── jev_test.go     # httptest fixtures: both error dialects, timeout, 429/5xx retry, response-too-large, span assertions
│       └── testdata/       # (optional) captured request/response fixtures mirroring the spike's verified shapes
```

### Pattern 1: Provider Client New()+Option (mirror `internal/embed`/`internal/summarize`)

**What:** A `Client` struct with unexported fields, a `New(baseURL, apiKey, model string, opts
...Option) *Client` constructor, and `With*` functional options for timeout/drain/max-timeout/
transport. Drain-bound defaults are set in the struct literal *before* the options loop runs
(so `WithDrainBytes(0)` is honored, not silently overwritten); the max-timeout ceiling is
resolved *after* the loop (so option order between `WithTimeout`/`WithMaxTimeout` doesn't
matter) `[VERIFIED: internal/embed/embed.go:214-257]`.

**When to use:** Exactly this shape for `jev.New(baseURL, apiKey, model string, opts
...jev.Option) *jev.Client`, with `WithConcurrency` added for D-10's worker-pool bound.

**Example (verbatim pattern to mirror, not code to copy blindly — field names will differ):**
```go
// Source: internal/embed/embed.go:214-257 (verified read this session)
func New(baseURL, apiKey, model string, opts ...Option) *Client {
	c := &Client{
		baseURL: baseURL, apiKey: apiKey, model: model,
		http:         &http.Client{Timeout: defaultEmbedTimeout},
		drainBytes:   defaultDrainBytes,   // set BEFORE options loop (D-05/D-06)
		drainTimeout: defaultDrainTimeout, // set BEFORE options loop (D-05/D-06)
	}
	for _, o := range opts {
		o(c)
	}
	if c.maxResponseBytes <= 0 {
		c.maxResponseBytes = defaultMaxResponseBytes // set AFTER loop
	}
	if c.maxTimeout <= 0 {
		c.maxTimeout = defaultMaxTimeout // set AFTER loop
	}
	if c.http.Timeout <= 0 {
		c.http.Timeout = c.maxTimeout // non-positive Timeout resolves to ceiling, never unbounded
	}
	return c
}
```

### Pattern 2: config registry row + Validate() gating (mirror `summarize.*`'s Model-gated block)

**What:** Every `ENGRAM_DECISIONS_*` var gets exactly one row in
`internal/config/registry.go`'s `registry` slice (`Key`, `Env`, no `Legacy` — brand-new, no
`Flag` — provider-tuning values are never typed at a prompt, per the existing
`embed.drain_bytes`/`summarize.drain_bytes` precedent `[VERIFIED: internal/config/registry.go:41-58,80-90]`).
Validation for the drain/timeout-ceiling knobs is unconditional and always-enforced (mirrors
`embed.max_timeout`); validation for `BaseURL`/`APIKey`/`Model` should be **gated on
`Provider != ""`**, exactly like `Summarize.Model`-gated block wraps
`summarize.max_chars`/`summarize.timeout`/the three drain knobs
`[VERIFIED: internal/config/validate.go:238-281]`.

**IMPORTANT correction to CONTEXT.md's D-03 phrasing:** D-03 says "config validation fails
with the standard field+hint envelope." I read `internal/config/validate.go` in full this
session: **`Config.Validate()` does NOT use the `field=<name> hint=<code>` envelope anywhere.**
That grammar (`internal/server/argerror.go`'s `HintCode` constants) is exclusively the MCP
tool-call / Connect RPC argument-rejection contract, documented end-to-end in
`docs-site/src/content/docs/reference/errors.md`, which states explicitly it covers "every
argument-validation rejection... on both the MCP tool-call lane and the Connect RPC lane"
`[VERIFIED: docs-site/src/content/docs/reference/errors.md:6-11]` — a fundamentally different
audience (an RPC caller/agent) than `Config.Validate()`'s audience (an operator reading
stderr at process startup). `Config.Validate()`'s actual, consistent style is plain
`fmt.Errorf("ENGRAM_VAR_NAME %q: <reason>", value, ...)`, aggregated via `errors.Join`
`[VERIFIED: internal/config/validate.go:29-49,138-149]`. The planner should have the new
`ENGRAM_DECISIONS_BASE_URL`-required-when-`provider=jev` check follow **this** existing style
(plain `errors.New`/`fmt.Errorf` naming the var), not attempt to import `internal/server`'s
`HintCode` type into `internal/config` (which would also be a package-layering violation —
`internal/config` has no dependency on `internal/server` today).

**Example (the actual style to write, adapted from the existing OpenAI.BaseURL check):**
```go
// Source: internal/config/validate.go:138-149, :238 (verified read this session) — pattern to
// adapt, not literal code to copy (Decisions.Provider gate is new).
if c.Decisions.Provider != "" && c.Decisions.Provider != "jev" {
	errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_PROVIDER %q: must be one of \"\" or \"jev\"", c.Decisions.Provider))
}
if c.Decisions.Provider == "jev" {
	switch u, err := url.Parse(c.Decisions.BaseURL); {
	case c.Decisions.BaseURL == "":
		errs = append(errs, errors.New("ENGRAM_DECISIONS_BASE_URL is empty: required when ENGRAM_DECISIONS_PROVIDER=jev (does not fall back to ENGRAM_OPENAI_BASE_URL)"))
	case err != nil:
		errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_BASE_URL %q: must be a valid URL: %w", c.Decisions.BaseURL, err))
	case u.Scheme != "http" && u.Scheme != "https":
		errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_BASE_URL %q: scheme must be http or https", c.Decisions.BaseURL))
	case u.Host == "":
		errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_BASE_URL %q: missing host", c.Decisions.BaseURL))
	}
}
```

**Test pattern to mirror:** `internal/config/providerbounds_test.go`'s table-driven
`providerBoundFields` + `TestProviderBoundRegistryEntries`/`TestValidateProviderBounds` shape —
one shared table drives both the registry-shape assertions (exactly one row, correct
Legacy/Flag/Default) and the Validate() zero/negative/unparseable cases
`[VERIFIED: internal/config/providerbounds_test.go:1-171, full file read this session]`.

### Pattern 3: Wiring seam — `*FromConfig(cfg) (*Client, error)` in `internal/server/tools.go`

**What:** `embedderFromConfig(cfg *config.Config) (*embed.Client, error)` and
`summarizerFromConfig(cfg *config.Config) *summarize.Client` are the two existing functions
`buildDepsFromEnv` (line 301) calls to build provider clients from an already-loaded config,
each wiring `otelhttp.NewTransport(http.DefaultTransport)` as the HTTP transport
`[VERIFIED: internal/server/tools.go:602-672]`. `summarizerFromConfig` returns a client
unconditionally but is only ever called when `cfg.Summarize.Model != ""`
(`StoreAndSummarizerFromEnv` checks first) — this is the direct precedent for D-01's
"provider unset → no decision client constructed" requirement.

**Open decision for the planner (CONTEXT.md flags this as Claude's discretion but does not
resolve it):** whether `deciderFromConfig(cfg) (decide.Decider, error)` gets wired into the
`deps` struct inside `buildDepsFromEnv` now (an unused-this-phase field, mirroring how
`summarize.Client` was presumably added to `deps` before it had every caller wired), or left as
a standalone exported constructor that Phase 3/4 wires in later. Wiring it into `deps` now is
lower-risk (it exercises the "construct nothing when provider=='' " path through the same
integration test surface `buildDepsFromEnv`'s existing tests already cover) but touches a
`deps` struct with an established shape (`tools.go` is large — 2600+ lines observed this
session, confirmed via wc-equivalent `Read` on the wiring region only, not full read). Either
choice satisfies DEC-01 and D-01 as written; record whichever is chosen as a plan decision.

### Anti-Patterns to Avoid

- **Routing the Jev call through the existing `embed`/`summarize` client or `ENGRAM_OPENAI_*`
  chat base URL:** the spike found this 400s — Jev is a decisions model, not a chat-completions
  model, and the LiteLLM route is a `/openrouter/alpha/decisions` pass-through, not under `/v1`
  `[VERIFIED: .claude/skills/spike-findings-engram/references/decision-transport.md — "What to
  Avoid" section, read this session]`.
- **Parsing the error `message` string for control flow:** both dialects' `message` field is a
  stringified upstream body; classify on HTTP status only, per D-12 and the spike's own explicit
  warning.
- **Exact-equality assertions on probabilities in tests:** the spike measured ±0.03 variance on
  identical requests at a 0.9 probability — CONTEXT.md's own "Specific Ideas" section restates
  this. Test fixtures must assert ranges/tolerances, never exact floats.
- **Treating `~typesafe/jev-latest` as equivalent to the pinned `typesafe/jev-1.13` default:**
  the spike explicitly warns the floating alias moves thresholds.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|--------------|-----|
| Bounded response-body drain | A second byte/time-bounded reader | `internal/httpdrain.Drain` | Already the one shared mechanism both existing provider clients call; a second copy is exactly the drift D-02's doc comment warns against `[VERIFIED: internal/httpdrain/httpdrain.go:1-14]`. |
| Request/response JSON marshaling for Choice/Score/Noul questions | Hand-written structs + `encoding/json` | OpenRouter Go SDK's `components.Questions`/`components.Answers` discriminated unions, IF D-06 resolves ADOPT | Typed, generated from the live OpenAPI spec, updated same-day as the API — a hand-rolled struct set would need re-verification against the spike's captured shapes and would drift as the alpha API evolves. |
| Span/metric plumbing | A bespoke tracer setup inside `internal/decide/jev` | `otel.Tracer("github.com/seanb4t/engram/internal/decide/jev")` + the existing `internal/telemetry` slog-bridge, exactly as `internal/embed`/`internal/summarize` do | Zero new OTel wiring needed — the global `TracerProvider`/`MeterProvider` are already configured in `internal/telemetry`; a new tracer only needs `otel.Tracer(pkgPath)` at package scope `[VERIFIED: internal/embed/embed.go:28, internal/summarize/summarize.go:31]`. |
| One-retry-on-429/5xx jittered backoff | A bespoke retry loop | Either the SDK's built-in `WithRetryConfig` (if adopted — note its default strategy would need overriding to match D-11's "one retry" exactly, since the SDK's own README example shows a multi-attempt backoff config) or a small ~15-line manual retry wrapping the single HTTP call (if hand-rolled) | D-11 wants exactly one retry, not a generic backoff policy — verify whichever path is chosen actually caps at one retry, don't assume the SDK's default matches. |

**Key insight:** every piece of infrastructure this phase needs (bounded drain, OTel, config
registry, Helm secret-ref pattern) already has exactly one blessed implementation in this
codebase, used identically by two prior provider integrations. The only genuinely novel
decision is D-05/D-06 (SDK adopt vs. reject) and the System One interface shape itself
(D-08/D-09/D-10, already locked in CONTEXT.md).

## Common Pitfalls

### Pitfall 1: LiteLLM's string `code` field breaking the SDK's typed error decode

**What goes wrong:** If the SDK is adopted, a Decisions call routed through the LiteLLM
pass-through that returns a 401/400/429/etc. may fail to decode into the SDK's typed
`sdkerrors.*ResponseError` structs, because those structs' `Error_.Code` field is `int64`
`[CITED: raw.githubusercontent.com/OpenRouterTeam/go-sdk/main/models/components/badrequestresponseerrordata.go]`
while the spike verified LiteLLM emits `code` as a JSON string
`[VERIFIED: .claude/skills/spike-findings-engram/references/decision-transport.md — "Do not rely
on the error body shape" bullet]`.
**Why it happens:** The SDK's error types are generated from OpenRouter's own OpenAPI spec,
which only documents OpenRouter's own numeric-`code` dialect — LiteLLM's pass-through wraps the
upstream response in its own envelope shape, which the spec never saw.
**How to avoid:** Before writing `jev.go`'s error-classification code, run an httptest fixture
replaying the spike's captured LiteLLM error body through `Alpha.Decisions.Create` and observe
what actually comes back (a decode error? the generic `APIError` with the raw body still intact
in `.Body`? something else?). Record the observation as this phase's D-07 evaluation doc. If the
decode silently degrades to `APIError.StatusCode` + raw `.Body` string, classification can still
work off `StatusCode` alone and this is a non-issue; if it produces an opaque, unclassifiable
error, that is grounds to reject the SDK per D-06 or to bypass its typed-error decode path
entirely (classify off the wrapping `http.RoundTripper`'s observed status code before the SDK
gets a chance to decode).
**Warning signs:** A `jev_test.go` case using the LiteLLM-dialect fixture produces a test
failure or an error that doesn't map to any of the seven `ErrDecision*` named errors D-12
requires.

### Pitfall 2: Byte-bounding the response when the SDK owns the decode

**What goes wrong:** `internal/embed`/`internal/summarize`'s pattern (`io.LimitReader` +
`json.NewDecoder` in the caller, then `httpdrain.Drain` the remainder) assumes the caller reads
`resp.Body` directly. The OpenRouter SDK's `Alpha.Decisions.Create` returns an already-decoded
`*components.DecisionsResponse` — the caller never sees `resp.Body`.
**Why it happens:** The SDK is a full request/response marshaling layer, not a thin transport;
`WithClient()` only lets you swap the `Do(req) (*http.Response, error)` implementation, not
intercept the decode step.
**How to avoid:** Enforce the response-too-large bound with a custom `http.RoundTripper`
wrapping `resp.Body` in an `io.LimitReader` (and closing/draining on overflow) *inside* the
`http.Client` passed to `WithClient()` — this is the one place engram code still touches the raw
bytes before the SDK decodes them. Write this as its own small, testable type
(`limitedBodyTransport` or similar) rather than threading a byte cap through the SDK's own
options (it has none for this).
**Warning signs:** A `jev_test.go` case with an oversized httptest response body succeeds
(decodes fully) instead of erroring — proves the bound isn't actually wired.

### Pitfall 3: `~typesafe/jev-latest` vs the pinned `typesafe/jev-1.13` default

**What goes wrong:** Using the floating alias in tests or defaults produces flaky
probability/confidence assertions as TypeSafe updates the underlying model.
**Why it happens:** The spike explicitly measured and warned about this.
**How to avoid:** `ENGRAM_DECISIONS_MODEL` default is `typesafe/jev-1.13` (D-02, already
locked) — never substitute the floating alias in code, tests, or Helm defaults.
**Warning signs:** A CI test that was green starts flaking on probability-threshold assertions
with no code change.

### Pitfall 4: No existing precedent for a response-byte-cap operator knob

**What goes wrong:** Assuming `ENGRAM_DECISIONS_MAX_RESPONSE_BYTES` (floated with a `?` in the
phase brief) should be added to the config registry alongside the drain/timeout knobs, mirroring
`ENGRAM_EMBED_DRAIN_BYTES`.
**Why it happens:** It looks structurally similar to the other bound knobs.
**How to avoid:** I checked both `internal/embed` and `internal/summarize` — **neither exposes a
success-path max-response-bytes value as an `ENGRAM_` var.** `internal/embed` derives its bound
internally from `ENGRAM_EMBED_DIM` via a formula (`embedderFromConfig`,
`internal/server/tools.go:628-647` `[VERIFIED]`); `internal/summarize` hardcodes `1<<20` inline
`[VERIFIED: internal/summarize/summarize.go:306-308]`. There is no existing operator-facing
knob for this bound anywhere in the registry. Recommend following that precedent: a fixed
internal constant for the decisions success-path decode bound (sized generously — Jev's 32k-
token context ceiling plus a batch of up to 50 questions is still a small JSON payload, well
under embed's dimension-derived bounds), not a new registry row — unless the planner has a
specific reason to diverge, in which case it should be called out as a deliberate departure from
precedent, not an oversight.
**Warning signs:** A config test asserting a registry row for `decisions.max_response_bytes`
with no precedent elsewhere in the file to justify the divergence.

## Code Examples

### Config registry rows (adapt to actual chosen field names)

```go
// Source: pattern from internal/config/registry.go:26-160 (verified read this session,
// combined with CONTEXT.md D-01/D-02/D-08/D-10's locked field list). Illustrative — exact
// Key/koanf names are Claude's discretion per CONTEXT.md.
{Key: "decisions.provider", Env: "ENGRAM_DECISIONS_PROVIDER", Default: ""},
{Key: "decisions.base_url", Env: "ENGRAM_DECISIONS_BASE_URL"},
{Key: "decisions.api_key", Env: "ENGRAM_DECISIONS_API_KEY"}, // falls back to ENGRAM_OPENAI_API_KEY at the wiring seam (cmp.Or), like ChatAPIKey (D-03)
{Key: "decisions.model", Env: "ENGRAM_DECISIONS_MODEL", Default: "typesafe/jev-1.13"},
{Key: "decisions.timeout", Env: "ENGRAM_DECISIONS_TIMEOUT", Default: "30s"},
{Key: "decisions.max_timeout", Env: "ENGRAM_DECISIONS_MAX_TIMEOUT", Default: "10m"},
{Key: "decisions.drain_bytes", Env: "ENGRAM_DECISIONS_DRAIN_BYTES", Default: "262144"},
{Key: "decisions.drain_timeout", Env: "ENGRAM_DECISIONS_DRAIN_TIMEOUT", Default: "2s"},
{Key: "decisions.concurrency", Env: "ENGRAM_DECISIONS_CONCURRENCY", Default: "4"}, // D-10's measured c=4
```

### OTel span assertion test pattern (already used in this repo)

```go
// Source: internal/embed/embed_test.go:392-427 (verified read this session, full function).
// Mirror this exactly for a TestJevDecideEmitsSpan, asserting the D-13 attribute set
// (engram.decide.provider/model/model_snapshot/questions/input_tokens/output_tokens/
// cost_usd/status) instead of embed's dims/model attributes.
sr := tracetest.NewSpanRecorder()
prev := otel.GetTracerProvider()
otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
t.Cleanup(func() { otel.SetTracerProvider(prev) })
// ... make the call against an httptest server ...
spans := sr.Ended()
attrs := map[string]string{}
for _, kv := range spans[0].Attributes() {
	attrs[string(kv.Key)] = kv.Value.String()
}
```

### Helm values + `_helpers.tpl` pattern (secret-ref for the API key, plain value otherwise)

```yaml
# Source: pattern from charts/engram/values.yaml:100-116 (memory.summarize block, verified
# read this session). Decisions block would sit alongside memory.summarize, off by default
# (decisions.provider empty omits the var entirely, matching summarize.model's "empty disables").
decisions:
  provider: "" # ENGRAM_DECISIONS_PROVIDER; "" (default, off) | "jev"
  baseURL: "" # ENGRAM_DECISIONS_BASE_URL; required when provider=jev, does NOT fall back
  apiKeySecret:
    name: "" # Secret holding the decisions API key; empty inherits memory.openai.apiKeySecret
    key: ""
  model: "typesafe/jev-1.13" # ENGRAM_DECISIONS_MODEL (pinned; do not use ~typesafe/jev-latest)
  timeout: 30s
  concurrency: 4
```

```gotemplate
{{- /* Source: charts/engram/templates/_helpers.tpl:27-46 pattern (verified read this session).
       Empty provider omits every decisions.* var -> server constructs nothing (D-01). */}}
{{- with .Values.memory.decisions.provider }}
- { name: ENGRAM_DECISIONS_PROVIDER, value: "{{ . }}" }
{{- with $.Values.memory.decisions.baseURL }}
- { name: ENGRAM_DECISIONS_BASE_URL, value: "{{ . }}" }
{{- end }}
{{- if $.Values.memory.decisions.apiKeySecret.name }}
- name: ENGRAM_DECISIONS_API_KEY
  valueFrom:
    secretKeyRef:
      name: "{{ $.Values.memory.decisions.apiKeySecret.name }}"
      key: "{{ $.Values.memory.decisions.apiKeySecret.key }}"
{{- end }}
{{- end }}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| n/a — greenfield capability | OpenRouter's `/api/alpha/decisions` (Decisions API, "alpha" status) + typed Go SDK support (`Alpha.Decisions.Create`) | SDK support for this endpoint appears to have landed very recently — the fetched changelog language references "SystemOne feature with decision-making capabilities" added around v0.8.8, with the surrounding versions (v0.8.12–v0.8.19) all landing 2026-09-21/22, i.e. essentially concurrent with this research | The API and its SDK coverage are both alpha/fast-moving. Re-verify the SDK's Decisions surface at implementation time — do not assume this research's snapshot is still accurate if implementation lands more than a few days later. |

**Deprecated/outdated:** none applicable — this is a net-new capability in engram.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|-----------------|
| A1 | The exact `go get github.com/OpenRouterTeam/go-sdk@<version>` pin recommended (`v0.8.19` at research time) will still be the correct/current version at implementation time | Standard Stack | Low — re-running `go list -m -versions` at implementation time trivially catches drift; the module's release cadence (5/day) makes this near-certain to need re-verification, not a surprise. |
| A2 | The SDK's typed error decode either (a) fails cleanly and falls back to `sdkerrors.APIError` with `StatusCode`/`.Body` intact, or (b) fails in some other, less classification-friendly way, when handed the LiteLLM string-`code` dialect | Common Pitfalls #1, Summary | Medium — if (b), the D-06 checkpoint may need to reject the SDK outright rather than adopt-and-wrap, changing the phase's shape materially. This is exactly why D-07 requires the evaluation be a Wave-0 spike task before production code, not this research pass (no live credentials/network access to OpenRouter or the LiteLLM gateway in this research session). |
| A3 | No existing config-docs gate (`configure.md`, `guides/upgrade.md`) enforces that every new registry key appears in a specific doc section — verified there is no automated test binding `configure.md` to the registry (unlike the CLI's `TestUpgradeGuideNamesEveryChangedCommand`), and that brand-new, off-by-default additive vars (the `ENGRAM_EMBED_DRAIN_*` precedent) did not require an `upgrade.md` entry | Common Pitfalls, Standard Stack | Low — worst case the planner adds an unnecessary upgrade.md task; the actual risk is *omitting* the manual `configure.md` update, which this research explicitly calls out as required by D-04 regardless of gate enforcement. |
| A4 | `deciderFromConfig` should be wired into `buildDepsFromEnv`'s `deps` struct now (vs. left as a standalone unwired constructor) | Architecture Patterns Pattern 3 | Low-Medium — this is explicitly flagged in CONTEXT.md as Claude's discretion; either choice satisfies DEC-01, but picking wrong could mean rework in Phase 3 when the first real caller (`spine-review consolidate`) needs to reach the constructed client through `deps`. |

**If this table is empty:** N/A — see rows above.

## Open Questions

1. **Does the SDK's typed error decode survive the LiteLLM string-`code` dialect at all?**
   - What we know: the struct field is `int64`; the spike verified LiteLLM sends a JSON string.
   - What's unclear: whether this produces a decode error the SDK surfaces to the caller
     (and how — wrapped how, discoverable how), or whether the SDK's response-parsing layer is
     itself resilient (e.g. it may fall back to the generic `APIError` on any decode failure of
     a more specific typed error, since `APIError`'s fields are all untyped strings/ints read
     more defensively) — I could not find source confirming which without either a live call or
     reading the SDK's internal response-dispatch code, which was out of scope for this pass.
   - Recommendation: this IS the D-07 Wave-0 spike task. Do not guess; build the httptest
     fixture from the spike's captured LiteLLM error body and observe directly, before any
     production `jev.go` code.

2. **Should `ENGRAM_DECISIONS_API_KEY`'s fallback to `ENGRAM_OPENAI_API_KEY` (D-03) happen in
   `internal/config` (a computed field) or at the `deciderFromConfig` wiring seam (a `cmp.Or`
   call, mirroring `summarizerFromConfig`'s `chatAPIKey := cmp.Or(cfg.OpenAI.ChatAPIKey,
   cfg.OpenAI.APIKey)` pattern)?**
   - What we know: `internal/server/tools.go:657-658` resolves `ChatBaseURL`/`ChatAPIKey`
     fallback at the wiring seam, not inside `internal/config` `[VERIFIED]`.
   - What's unclear: whether Decisions should follow that exact precedent or do the resolution
     earlier.
   - Recommendation: follow the `ChatAPIKey` precedent exactly (`cmp.Or` at the wiring seam) —
     it is the closest existing analog (an optional secondary key falling back to the primary
     one) and keeps `internal/config` as pure assembly, consistent with `Config.Validate`'s own
     doc comment ("Validation lives outside config.Load on purpose... a programming error, never
     operator input") `[VERIFIED: internal/config/validate.go:16-28]`.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|--------------|-----------|---------|----------|
| Network access to `openrouter.ai` / `llm.fzymgc.house` | Live Decisions API calls (Wave-0 SDK evaluation spike, integration tests) | Not verified in this research session — no outbound calls were made against either endpoint; all transport findings above are httptest-fixture-based or sourced from the spike's prior live verification, not re-confirmed live here | — | Wave-0 task must confirm live reachability directly (the spike already did this once, 2026-09-22 — see `decision-transport.md`'s Latency table — but that was a separate session and credentials/grants should be re-confirmed, not assumed still valid) |
| `github.com/OpenRouterTeam/go-sdk` on the Go module proxy | D-05/D-06 adoption path | Confirmed reachable and current via `pkg.go.dev`/Context7 in this session | `v0.8.19` at research time | Hand-rolled `net/http` client (D-06's reject branch) if the module proves unreachable or unsuitable at implementation time |

**Missing dependencies with no fallback:** none — every dependency in this phase has an explicit fallback path already locked into D-06.

**Missing dependencies with fallback:** live OpenRouter/LiteLLM reachability (see above) — the
Wave-0 spike task is the fallback verification step regardless of which path (SDK vs.
hand-rolled) is chosen.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go's standard `testing` package + `net/http/httptest` + `go.opentelemetry.io/otel/sdk/trace/tracetest` — no third-party assertion library in production test code (the OpenRouter SDK pulls in `testify` transitively but engram's own tests don't use it `[VERIFIED: rg for testify usage found only go.mod's indirect entry, no import in engram's own _test.go files during this session's greps]`) |
| Config file | none — `go test` defaults, driven by `Taskfile.yaml` |
| Quick run command | `go test ./internal/decide/... -count=1` |
| Full suite command | `task` (lint + `go test ./...`) `[VERIFIED: Taskfile.yaml:50,65 — "go test ./..." rows read this session]` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|---------------------|--------------|
| DEC-01 | Provider unset -> zero decision-related outbound calls, byte-identical behavior | unit | `go test ./internal/server/... -run TestBuildDepsFromEnv -count=1` (extend existing wiring test) | ❌ Wave 0 — new subtest on the existing wiring test, or a new `deciderwiring_test.go` mirroring `embed_wiring_test.go` |
| DEC-02 | One Go interface, batched Choice/Score/Noul in, typed probabilities/confidence out, backend-agnostic | unit | `go test ./internal/decide/... -run TestDecider -count=1` | ❌ Wave 0 |
| DEC-03 | Jev backend reaches `{base}/alpha/decisions` with its own base-URL/key/model/timeout, works against both OpenRouter shape and LiteLLM shape | unit (httptest) | `go test ./internal/decide/jev/... -run TestJevRequestShape -count=1` | ❌ Wave 0 |
| DEC-04 | Bounded calls; status-classified named errors for both error dialects; a decision failure never fails the surrounding operation | unit (httptest, both dialects + timeout + oversized response) | `go test ./internal/decide/jev/... -run TestJevErrorClassification -count=1` | ❌ Wave 0 |
| DEC-05 | SDK evaluated, adopt/reject decision recorded before any client code | manual/doc | N/A — a phase doc (D-07) + durable engram record, not a `go test` target | ❌ Wave 0 — the evaluation spike itself is the Wave-0 deliverable |
| DEC-06 | Every decision call emits an OTLP span with latency/question-count/tokens/cost | unit (`tracetest.SpanRecorder`) | `go test ./internal/decide/jev/... -run TestJevDecideEmitsSpan -count=1` | ❌ Wave 0 — mirrors `internal/embed/embed_test.go:392-427`'s existing pattern exactly |

### Sampling Rate

- **Per task commit:** `go test ./internal/decide/... -count=1`
- **Per wave merge:** `task` (full lint + test suite)
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/decide/decide_test.go` — interface/type contract tests (DEC-02)
- [ ] `internal/decide/jev/jev_test.go` — httptest fixtures for both error dialects, timeout,
      429/5xx retry, response-too-large, span attributes (DEC-03, DEC-04, DEC-06)
- [ ] `internal/decide/jev/spike_evaluation_test.go` (or equivalent) — the D-07 SDK-evaluation
      spike: replay the spike's captured LiteLLM error-body fixture through the OpenRouter SDK's
      `Alpha.Decisions.Create` and record the observed decode behavior. This is the single
      highest-priority Wave-0 task — it resolves the D-06 checkpoint and everything else in the
      phase depends on its outcome.
- [ ] Framework install: none — `go test`/`httptest`/`tracetest` are all already available; if
      D-06 resolves ADOPT, `go get github.com/OpenRouterTeam/go-sdk` is the one new install.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|----------------|---------|---------------------|
| V2 Authentication | No | This phase adds an outbound client credential (`ENGRAM_DECISIONS_API_KEY`), not an inbound auth surface. |
| V3 Session Management | No | N/A — no session state introduced. |
| V4 Access Control | No | N/A — no new authz-relevant resource introduced; the decider has no caller yet this phase. |
| V5 Input Validation | Yes | D-09's cheap structural validation (>255 choices, empty criteria, duplicate question names, unknown question type) — client-side, no network call, named errors. Follows the same "reject at the boundary, never coerce" posture `Config.Validate` and `internal/server/argerror.go` already establish elsewhere in this codebase. |
| V6 Cryptography | No | No new cryptographic material — `ENGRAM_DECISIONS_API_KEY` is a bearer credential handled exactly like `ENGRAM_OPENAI_API_KEY`/`ENGRAM_OPENAI_CHAT_API_KEY` (Kubernetes `Secret` ref in Helm, never a plain value in `values.yaml`, per the existing `apiKeySecret`/`chatApiKeySecret` pattern `[VERIFIED: charts/engram/values.yaml:76-78,111-113]`). |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|------------------------|
| Credential leakage via error-body logging | Information Disclosure | `maxErrorBodyBytes` bound (4096, matching `internal/embed`/`internal/summarize`'s existing constant) before any error body reaches a log line or returned error — the provider's error text is bounded and never includes the request body/API key. |
| Unbounded response read from a misbehaving/malicious gateway (LiteLLM pass-through is operator-controlled infra, but still an external network hop) | Denial of Service | `httpdrain.Drain` (byte + time bound) after every response, on both success and error paths — same pattern as embed/summarize; if the SDK is adopted, this becomes the wrapping `http.RoundTripper`'s job (Common Pitfalls #2) since the SDK owns the decode. |
| A decision-call failure cascading into a caller's own read/sweep failure | Denial of Service (self-inflicted) | D-12's explicit contract: "A decision failure never fails the surrounding read or sweep — that is the caller contract, documented on the interface." This is a design invariant, not a runtime check — the interface's doc comment must state it, and future callers (Phase 3/4) are responsible for honoring it (advisory-only, never blocking). |
| State (record content) sent to a third-party model as part of the `state` field | Information Disclosure (to a third party) | Already an accepted design tradeoff per the milestone's own scope — Jev's data policy is "no training on inputs... retention standard, ZDR unconfirmed" `[CITED: .claude/skills/spike-findings-engram/references/decision-transport.md — Constraints table]`. Not a new risk this phase introduces (the interface carries state; callers choose what state to send — Phase 3/4's concern, not this phase's transport layer) but worth flagging for the planner's awareness since ZDR is explicitly "unconfirmed," not confirmed-off. |

## Sources

### Primary (HIGH confidence)
- Context7 `/openrouterteam/go-sdk` — `Alpha.Decisions.Create` request/response shape, `Answers`
  discriminated union, `Usage` struct, custom `HTTPClient`/`WithClient()`, `WithRetryConfig`,
  error-handling `errors.As` pattern.
- `raw.githubusercontent.com/OpenRouterTeam/go-sdk/main/{go.mod, models/sdkerrors/apierror.go,
  models/sdkerrors/badrequestresponseerror.go, models/sdkerrors/toomanyrequestsresponseerror.go,
  models/components/decisionschoiceanswer.go, models/components/badrequestresponseerrordata.go}`
  — direct source-file fetches (not training data), used for the `int64`-vs-string `code`
  finding and dependency-footprint claim.
- `pkg.go.dev/github.com/OpenRouterTeam/go-sdk?tab=versions` — release cadence/dates.
- In-repo, read this session in full: `internal/embed/embed.go`, `internal/summarize/summarize.go`,
  `internal/httpdrain/httpdrain.go`, `internal/config/registry.go`, `internal/config/validate.go`,
  `internal/config/providerbounds_test.go`, `internal/server/tools.go` (lines 600-700, 2640-2710),
  `charts/engram/values.yaml`, `charts/engram/templates/_helpers.tpl` (lines 1-100),
  `docs-site/src/content/docs/reference/errors.md` (in full), `internal/embed/embed_test.go`
  (lines 385-440), `internal/config/config.go` (lines 1-95),
  `.claude/skills/spike-findings-engram/SKILL.md` and `references/decision-transport.md` (in
  full), `.planning/phases/02-decision-interface-jev-backend/02-CONTEXT.md`,
  `.planning/REQUIREMENTS.md`, `.planning/STATE.md` (lines 1-310), `go.mod`, `Taskfile.yaml`
  (lines 40-90), `.planning/config.json`.

### Secondary (MEDIUM confidence)
- `openrouter.ai/docs/client-sdks/go/sdks/decisions/README` (fetched via GitHub docs mirror,
  `github.com/OpenRouterTeam/go-sdk/blob/main/docs/sdks/decisions/README.mdx`) — the per-operation
  Errors table for `Decisions.Create` (12 named status-code error types + `APIError`).

### Tertiary (LOW confidence)
- WebSearch summary of "OpenRouter Go SDK decisions API" — used only to locate the correct
  Context7 library ID and GitHub repo, cross-checked against the primary sources above before
  any claim was written into this document.

## Metadata

**Confidence breakdown:**
- Standard stack (SDK adoption evidence): HIGH — verified via live Context7 fetch + direct
  GitHub raw-source reads of the actual struct definitions, not training data.
- Architecture (client/config/Helm/wiring patterns): HIGH — every pattern cited is a verbatim
  read of existing, shipped engram code this session, with line numbers.
- Pitfalls (LiteLLM dialect risk, byte-bounding under the SDK): MEDIUM — the underlying facts
  (typed `int64` field, LiteLLM's string dialect) are each independently verified, but their
  *interaction* (what actually happens on decode) was not observed live in this session — flagged
  explicitly as Open Question 1 / Assumption A2, and routed to a mandatory Wave-0 spike rather
  than asserted as fact.

**Research date:** 2026-09-23
**Valid until:** ~7 days for the SDK-version-pin claims specifically (release cadence observed
at 5/day — re-verify `go list -m -versions` at implementation time regardless of how soon that
is); ~30 days for the in-repo architecture patterns (stable unless another phase touches
`internal/embed`/`internal/summarize`/`internal/config`/the Helm chart first).
