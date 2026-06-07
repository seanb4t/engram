<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Structured logging + OpenTelemetry observability (logs / metrics / traces)

**Date:** 2026-06-07
**Status:** Design
**Design bead:** engram-ew7

## Context

engram is a self-hosted MCP memory server (Go + Qdrant) deployed to Kubernetes
via Helm/ArgoCD. Its entire operational output today is a handful of stdlib
`log.Printf`/`log.Fatalf` lines:

- `serve.go:58` — startup banner (`engram <v> listening on <addr>`)
- `serve.go:69` — OIDC-disabled warning
- `serve.go:77` — `log.Fatalf` on verifier init failure
- `serve.go:80` — OIDC-enabled banner
- `tools.go:80` — `log.Fatalf` on store init
- `tools.go:99,103` — pre-isolation owner-less warning
- `tools.go:365` — `search_discovery` cross-spine notice

That is the complete observability surface. In production this means:

- **No per-request visibility.** You cannot tell which tool was called, by
  whom, how long it took, or whether it succeeded. The 11 memory/discovery
  tools are entirely opaque once running.
- **No auth-failure visibility.** `mcpauth.RequireBearerToken` rejects invalid
  tokens with a 401 *before* any handler runs; nothing logs that a request was
  rejected or why.
- **No metrics, no traces.** OpenTelemetry appears in `go.mod` only as indirect
  transitive deps (pulled by the Qdrant client / go-sdk). There is no
  exporter, provider, or instrumentation.
- **The most valuable signal is discarded.** The verified caller identity
  (`actor`/`owner`, derived from the OIDC `sub`) is computed on every call for
  authorization but never logged.

This design adds full observability — structured logging, metrics, and traces —
instrumented at every meaningful seam and exported over OTLP, with logs also
available on stdout for `kubectl logs` / ArgoCD.

## Decisions

Settled during brainstorming (see bead engram-ew7 notes):

| Axis | Decision | Rationale |
|------|----------|-----------|
| Scope | Logs **+** metrics **+** traces | Full observability; OTel deps already transitively present. |
| Logging library | stdlib `log/slog` | Zero new logging dep; fits the repo's minimal-dep, env-first, no-viper ethos. Go 1.26. |
| Telemetry transport | **Full OTLP** (metrics + traces + logs over OTLP) | No Prometheus scrape endpoint; matches a Grafana LGTM/collector backend. |
| Log sinks | stdout **and** OTLP, stdout disableable via config | Keep `kubectl logs` / ArgoCD working by default; allow OTLP-only via `MEM_LOG_STDOUT=false`. |
| Delivery | One cohesive spec, phased implementation plan | Single design; plan sequences slog → telemetry bootstrap → seams → helm so PRs land incrementally. |
| Instrumentation seams | HTTP layer, MCP method layer, downstream clients | Auth failures only visible at HTTP; per-tool only at MCP method layer; embedder/qdrant via client instrumentation. |
| Tool-call seam | go-sdk `Server.AddReceivingMiddleware` | One wrapper instruments all 11 tools; no per-handler edits. |

## Architecture

### Three instrumentation seams

Observability concerns do not all surface at the same layer. Conflating them
into a single "wrap the handler" approach would silently miss auth failures,
because `RequireBearerToken` short-circuits before the MCP dispatcher runs.

```text
┌─ HTTP layer (http.Handler) ───────────────────────────────┐
│  otelhttp.NewHandler + logging middleware                  │
│  • AUTH FAILURES (401/403 from RequireBearerToken)         │
│  • http method, path, status, duration, remote addr        │
│  ┌─ MCP method layer (AddReceivingMiddleware) ───────────┐ │
│  │  • tool name (CallToolRequest.Params.Name)            │ │
│  │  • span tool/<name> + per-tool metrics + log line     │ │
│  │  • actor / owner (verified identity)                   │ │
│  │  ┌─ downstream seams (ctx-propagated child spans) ───┐ │ │
│  │  │  embedder: otelhttp.NewTransport on http.Client   │ │ │
│  │  │  qdrant:   otelgrpc stats handler on gRPC dial    │ │ │
│  │  └────────────────────────────────────────────────────┘ │ │
│  └────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────┘
```

Because `context.Context` is already threaded from every tool handler through
the store and embedder calls, a span placed on the context by the MCP
middleware automatically becomes the parent of the downstream embedder/qdrant
spans — no function signatures change.

### Components

#### `internal/telemetry` (new package)

Bootstrap and teardown for the OTel SDK. Single responsibility: construct
providers, set globals, return one shutdown closure.

- `Setup(ctx, cfg) (shutdown func(context.Context) error, err error)`:
  - Builds a `Resource` with `service.name=engram`, `service.version=<ldflags
    version>`, and a generated instance id; merges `OTEL_RESOURCE_ATTRIBUTES`.
  - `TracerProvider` — OTLP gRPC exporter (`otlptracegrpc.New`),
    `trace.WithBatcher`, `trace.WithResource`.
  - `MeterProvider` — OTLP gRPC exporter (`otlpmetricgrpc.New`),
    `metric.WithReader(metric.NewPeriodicReader(exp))`.
  - `LoggerProvider` — OTLP log exporter feeding the `otelslog` bridge.
  - Registers all three via `otel.SetTracerProvider` / `otel.SetMeterProvider`
    / `global.SetLoggerProvider` (the last from
    `go.opentelemetry.io/otel/log/global`), sets a W3C `TextMapPropagator`.
  - Returns a `shutdown` that flushes/closes all three providers in order.
- **Disabled cleanly** when `OTEL_EXPORTER_OTLP_ENDPOINT` is unset: returns
  no-op providers and a no-op shutdown; the process still runs and logs to
  stdout. Telemetry never being a hard startup dependency is deliberate.

#### Logging — `internal/telemetry` logger constructor

- Builds a `*slog.Logger` whose handler is a fan-out:
  - JSON (default) or text handler to stdout, gated by `MEM_LOG_STDOUT`
    (default `true`) and `MEM_LOG_FORMAT` (`json` default | `text`).
  - `otelslog` handler bridging records to the OTel `LoggerProvider` (active
    only when telemetry is enabled).
- Level from `MEM_LOG_LEVEL` (`debug|info|warn|error`, default `info`).
- The logger is installed as the slog default (`slog.SetDefault`) and passed
  explicitly where a component needs it.
- **All existing `log.*` calls are migrated** to structured slog with named
  attributes. `log.Fatalf` startup failures become `slog.Error` + a returned
  error / `os.Exit(1)` from `main`, preserving fail-fast without `log.Fatal`'s
  un-flushed exit.

#### HTTP instrumentation (`cmd/engram/serve.go`)

- Wrap the MCP handler with `otelhttp.NewHandler(handler, "mcp")` for inbound
  HTTP server spans/metrics.
- A thin logging middleware wraps a status-capturing `http.ResponseWriter` to
  emit one structured access log per request (method, path, status, dur_ms,
  remote, user-agent) and to increment `engram.auth.failures{reason}` on
  401/403 responses.
- The verifier (`internal/auth`) additionally logs the *reason* a token failed
  (expired / bad signature / wrong issuer / wrong audience) at the point of
  failure, where the detail exists — the HTTP layer only sees the resulting
  status.

#### MCP tool instrumentation (`internal/server`)

- `srv.AddReceivingMiddleware(instrument)` wraps the `MethodHandler`.
- For `method == "tools/call"`, the `req mcp.Request` (an interface) is
  type-asserted to `*mcp.CallToolRequest` (which is
  `*mcp.ServerRequest[*mcp.CallToolParamsRaw]`); the tool name is then
  `req.Params.Name`. It starts a span `tool/<name>`, records start time, calls
  the inner handler, then:
  - classifies outcome (`ok` | `error`) from the returned error and
    `CallToolResult.IsError`,
  - emits a structured log line (`tool`, `actor`, `owner`, `outcome`,
    `dur_ms`, `error`),
  - records `engram.tool.calls{tool,outcome}` and
    `engram.tool.duration{tool,outcome}`,
  - sets span status/attributes and ends the span.
- Non-tool methods (`initialize`, `tools/list`, …) are traced/logged at debug
  level without per-tool metric cardinality.

#### Downstream client instrumentation

- **Embedder** (`internal/embed`): the client's `http.Client` is currently
  built inside `embed.New()` and stored in an unexported field with no way to
  inject a transport. `embed.New()` must gain a seam — a functional option
  (`WithHTTPTransport` / `WithHTTPClient`) or an accepted `*http.Client` — so
  the caller can set
  `Transport = otelhttp.NewTransport(http.DefaultTransport)`. Each
  `/v1/embeddings` call then produces a client span + HTTP client metrics,
  parented to the tool span. (Plan task: add the constructor seam before
  wiring otelhttp.)
- **Qdrant** (`internal/store` / wherever the gRPC client is dialed): add the
  `otelgrpc` stats handler to the dial options so every Qdrant RPC
  (`Upsert`, `Query`, `CollectionExists`, …) produces a client span + metrics.
  The Qdrant Go client's `qdrant.Config` exposes a `GrpcOptions
  []grpc.DialOption` field (verified present in v1.18.2), so attaching the
  `otelgrpc` stats handler is a config change at the dial site — no fork or
  custom `ClientConn` needed.

#### Graceful shutdown (`cmd/engram/serve.go`)

- Replace bare `http.ListenAndServe` with an `*http.Server` configured with
  `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`.
- Install a `signal.NotifyContext` (SIGINT/SIGTERM) handler that:
  1. calls `server.Shutdown(ctx)` to drain in-flight requests,
  2. calls `telemetry.shutdown(ctx)` to flush batched spans/metrics/logs.
- This is **required**, not cosmetic: OTLP batchers buffer in memory, and the
  Kubernetes rolling restart (visible in ArgoCD) sends SIGTERM. Without a
  flush, the last window of telemetry — including the shutdown itself — is
  lost.

### Metrics

All exported over OTLP. Low, bounded cardinality (tool ∈ 11 known names;
outcome ∈ {ok,error}; reason ∈ a small fixed set).

| Metric | Type | Attributes | Source |
|--------|------|-----------|--------|
| `engram.tool.calls` | counter | `tool`, `outcome` | MCP middleware |
| `engram.tool.duration` | histogram (ms) | `tool`, `outcome` | MCP middleware |
| `engram.auth.failures` | counter | `reason` | HTTP middleware + auth verifier |
| `http.server.request.duration` | histogram | semconv | `otelhttp` |
| `http.client.request.duration` | histogram | semconv | `otelhttp` (embedder) |
| gRPC client RPC metrics | histogram/counter | semconv | `otelgrpc` (qdrant) |

Deferred (YAGNI): total-record-count gauge (needs Qdrant count polling), trace
runtime metrics (`otel runtime` instrumentation).

### Configuration (env-first, per repo convention)

Standard OTEL_* variables are honored natively by the OTLP exporters — no
re-plumbing:

| Variable | Effect |
|----------|--------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Collector endpoint; **unset → telemetry disabled** |
| `OTEL_EXPORTER_OTLP_HEADERS` | Auth headers for the collector |
| `OTEL_RESOURCE_ATTRIBUTES` | Extra resource attributes (e.g. `deployment.environment`) |
| `MEM_LOG_LEVEL` | `debug\|info\|warn\|error` (default `info`) |
| `MEM_LOG_FORMAT` | `json\|text` (default `json`) |
| `MEM_LOG_STDOUT` | `true\|false` (default `true`); `false` → OTLP-only logs |

**Silent-process guard:** `MEM_LOG_STDOUT=false` with no OTLP endpoint would
leave the process with *no* log sink at all. The logger constructor must detect
this combination, force stdout back on, and emit one warning that the requested
OTLP-only mode is degraded because no endpoint is configured — never silently
swallow all output.

The Helm chart (`charts/engram/`) gains values + env wiring for the OTEL_*
variables and the MEM_LOG_* knobs, defaulting telemetry off (no endpoint) so
the chart stays generic.

## Data flow (one `store_memory` call)

```text
client → HTTP (otelhttp span "mcp", access log)
       → RequireBearerToken (401+log+metric on failure)
       → MCP dispatch → AddReceivingMiddleware
            span "tool/store_memory"; capture actor/owner
            → handler → embed (child HTTP client span)
                      → qdrant Upsert (child gRPC span)
            ← classify outcome → log line + tool.calls/tool.duration
       ← HTTP access log (status, dur_ms)
```

A single trace ties the inbound request to the embedder HTTP call and the
Qdrant RPC; the structured log lines carry the same trace/span ids via the
`otelslog` bridge for log↔trace correlation in Grafana.

## Error handling

- **Telemetry setup failure** (e.g. bad endpoint) logs a warning and falls back
  to no-op providers — the server still starts and serves. Observability is
  never a hard dependency of memory service.
- **Exporter runtime failures** are handled by the SDK's batch processors
  (retry/drop); they never propagate into request handling.
- **Startup fatal errors** (store init, verifier init) switch from `log.Fatalf`
  to `slog.Error` + explicit `os.Exit(1)` from `main`, after attempting a
  telemetry flush, so the failure record is not lost.
- The `otelslog` bridge must not deadlock or block request handling if the log
  exporter stalls — rely on the SDK's async batch log processor.

## Testing

- `internal/telemetry`: `Setup` returns no-op providers + nil-safe shutdown
  when the endpoint is unset; provider construction succeeds with a stub
  exporter; shutdown is idempotent.
- Logger constructor: respects `MEM_LOG_STDOUT=false` (no stdout writes),
  `MEM_LOG_FORMAT`, and level filtering; emits expected attributes.
- HTTP middleware: status capture is correct; `engram.auth.failures`
  increments on 401/403; one access log per request.
- MCP middleware: tool name extracted from `CallToolRequest`; outcome
  classification for error return vs `IsError` result vs success; actor/owner
  present on the log line.
- slog migration: the pre-isolation owner-less warning (which already has a
  regression path) is asserted as a structured record with named fields.
- Existing handler/store/auth tests continue to pass unchanged (instrumentation
  is additive and context-propagated).

## Out of scope (YAGNI)

- Prometheus `/metrics` scrape endpoint (full-OTLP decision).
- Per-record / business gauges requiring Qdrant polling.
- Custom sampling strategies — use SDK defaults; revisit only if volume hurts.
- Log redaction framework — memory `content` is already never logged; only
  metadata (tool, actor, owner, scope, outcome) appears in logs.

## Implementation phases (for the plan)

1. **slog foundation** — logger constructor, replace all `log.*`, drop
   `log.Fatal` for flush-safe exit. (No telemetry yet; stdout only.)
2. **Telemetry bootstrap** — `internal/telemetry.Setup`, providers, no-op
   path, `otelslog` bridge, graceful shutdown + `http.Server`.
3. **Seams** — HTTP middleware (access log + auth-failure metric), MCP
   `AddReceivingMiddleware` (tool span/metrics/log), embedder `otelhttp`,
   qdrant `otelgrpc`.
4. **Helm + docs** — chart env wiring, README/CLAUDE.md notes, and `go.mod`
   dependency cleanup (see below).

## Dependencies

Only `go.opentelemetry.io/otel`, `.../otel/metric`, `.../otel/trace`,
`.../otel/sdk` (sdk currently in `go.sum` only), and
`.../contrib/instrumentation/net/http/otelhttp` are in the module graph today,
all as indirect deps. The exporter, log, and bridge packages below are **not in
`go.mod`/`go.sum` at all** — they require `go get` (each pulls new transitive
deps), not merely a "promote indirect → direct" edit:

- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc`
- `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc`
- `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc`
- `go.opentelemetry.io/otel/log` + `go.opentelemetry.io/otel/log/global`
- `go.opentelemetry.io/otel/sdk` + `.../sdk/metric` + `.../sdk/log`
- `go.opentelemetry.io/contrib/bridges/otelslog`
- `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc`

After `go get`, promote the now-used OTel packages to direct requires and run
`go mod tidy`. Phase 2 cannot compile until these are added — sequence the
`go get` as the first task of Phase 2, not Phase 4.
