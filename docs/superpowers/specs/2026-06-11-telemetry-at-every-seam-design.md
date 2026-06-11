<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# engram telemetry & metrics at every seam — instrumentation depth

**Design bead:** engram-sk8
**Date:** 2026-06-11
**Status:** Design (pending design-reviewer)
**Author:** Sean Brandt

## Summary

engram already ships a complete OpenTelemetry foundation: the
`internal/telemetry` package wires trace, metric, and log providers to OTLP/gRPC
exporters with graceful degradation, slog↔trace correlation, per-tool spans and
metrics, HTTP/Connect access logging, and a Helm `observability:` config block
(shipped under `engram-651`, see
`2026-06-07-observability-logging-telemetry-design.md`).

That foundation is **transport-deep, not function-deep**. The three layers below
the tool-handler seam — `internal/store` (19 methods), `internal/embed`
(`Embed`), and `internal/auth` (token verification) — carry **zero** direct
OpenTelemetry instrumentation. They are visible only through the auto-instrumented
gRPC (Qdrant) and HTTP (embedder) transports beneath them, so a single tool call
jumps from the `tool/<name>` span straight to raw RPC spans with no
engram-domain span or metric in between.

This design closes those gaps so telemetry and metrics exist at **every
important function and step**:

1. Inline spans + domain-latency metrics in `store`, `embed`, and `auth`.
2. `engram.owner` (opaque OIDC `sub`) as a span attribute across tool and layer
   spans.
3. A complete, idiomatic OpenTelemetry **resource** (currently missing
   `telemetry.sdk.*`, `process.*`, `host.*`, `os.*`, `container.*`).
4. Helm chart knobs for the OTel-standard sampler and metric-export-interval
   env vars (already honored by the binary, never exposed) and k8s resource
   attributes via the Downward API.

## Goals

- Every public `store`/`embed`/`auth` method emits a span with meaningful
  attributes and error status, nested correctly under the existing tool span and
  above the existing transport span.
- Per-engram-operation latency metrics that transport metrics cannot
  reconstruct (multiple store methods collapse onto the same Qdrant RPC).
- A production-grade resource: service identity + SDK + process + host + OS +
  container + (in k8s) pod identity.
- All telemetry knobs — sampler ratio, metric export interval, resource
  attributes — surfaced in `charts/engram/`.
- Zero change to the graceful-degradation posture: telemetry is never a hard
  dependency (ADR `engram-uxh`).

## Non-goals (YAGNI)

- No Prometheus `/metrics` scrape endpoint — OTLP-push remains the model.
- No request/response **body** logging (summary access logs already exist).
- No new `MEM_*` config surface — sampler/interval use the OTel-standard env
  vars the SDK already reads.
- No refactor of the existing `internal/telemetry` provider internals beyond the
  resource construction and the new metric instruments/helpers.
- No `actor` (email/PII) on spans — it stays on logs.

## Grounding (Rule 7)

Recorded as `bd note`s on `engram-sk8`:

- **probe:** confirmed `internal/store`/`embed`/`auth` carry zero direct otel;
  tool spans/metrics live in `internal/server/instrument.go`.
- **deepwiki `open-telemetry/opentelemetry-go`:** `sdktrace.NewTracerProvider`
  without `WithSampler` already reads `OTEL_TRACES_SAMPLER` +
  `OTEL_TRACES_SAMPLER_ARG`; `metric.NewPeriodicReader` without `WithInterval`
  already reads `OTEL_METRIC_EXPORT_INTERVAL` (default 60s). `providers.go`
  passes neither → **both knobs already work in the binary**; the gap is purely
  chart-templating.
- **deepwiki `open-telemetry/opentelemetry-go` (resource):** `resource.New` is
  opt-in per detector and does **not** include `resource.Default()`. Current
  `providers.go` opts into none of `WithTelemetrySDK`/`WithProcess`/`WithHost`/
  `WithOS`/`WithContainer`, so those attributes are absent today.
  `service.instance.id` auto-detector is experimental (`x.Resource`-gated) →
  keep the manual UUID.
- **context7 `/open-telemetry/opentelemetry-go`:** manual span idiom —
  package-level `otel.Tracer(name)`; `tracer.Start(ctx, name,
  trace.WithAttributes(...))` (attributes at creation, since samplers only see
  creation-time attributes); `span.RecordError(err)` +
  `span.SetStatus(codes.Error, msg)` on failure; `defer span.End()`;
  `Float64Histogram.Record(ctx, ms, metric.WithAttributes(...))`. No-op
  tracer/meter when no provider is set → zero cost when telemetry is disabled.

## Architecture

### Tracing seams (inline spans)

Each package gains one package-level tracer:

```go
var tracer = otel.Tracer("github.com/seanb4t/engram/internal/store") // and …/embed, …/auth
```

Span-per-public-method. Attributes set **at creation** where known, with
result-derived attributes set before `End`. Error path calls `RecordError` +
`SetStatus(codes.Error, …)`.

| Layer | Spans | Creation attributes | Result attributes |
|-------|-------|--------------------|-------------------|
| `store` | `store.Search`, `store.Upsert`, `store.List`, `store.Get`, `store.GetReadable`, `store.Delete`, `store.DeleteAll`, `store.SetVisibility`, `store.Update`, `store.FetchForUpdate`, `store.SearchDiscovery`, `store.ListScopes`, `store.OwnedOrAbsent`, `store.MigrateSetOwner`, `store.EnsureCollection`, `store.CountOwnerless`, `store.CountAnonymousBucket` | `engram.scope`, `engram.k`, `engram.owner` (opaque `sub`) where applicable | `engram.result_count` where applicable |
| `embed` | `embed.Embed` | `engram.embed.model` | `engram.embed.dims` |
| `auth` | `auth.VerifyToken` — created **inside the closure returned by** `(*Verifier).TokenVerifier()`, not on the `TokenVerifier()` method (which only constructs the closure) | — | `engram.auth.outcome` (`ok`/`error`) |

The existing `otelgrpc` (Qdrant) and `otelhttp` (embedder) transport spans become
**children** of these. Example post-change `search_memory` trace:

```text
tool/search_memory                 (internal/server/instrument.go)
├── embed.Embed                    (internal/embed)         ← NEW
│   └── POST …/embeddings          (otelhttp transport)
└── store.Search                   (internal/store)         ← NEW
    └── Qdrant Query               (otelgrpc transport)
```

### Domain metrics

New instruments, registered from the same `otel.Meter` that owns the existing
`engram.tool.*` family, mirroring its naming:

- `engram.store.duration` — Float64 histogram, unit `ms`; attributes
  `operation`, `outcome` (`ok`|`error`).
- `engram.embed.duration` — Float64 histogram, unit `ms`; attribute `outcome`.
- `engram.auth.verify.duration` — Float64 histogram, unit `ms`; attribute
  `outcome`.

**Rationale over transport metrics:** `otelgrpc` already emits
`rpc.client.duration`, but tagged by Qdrant RPC — and multiple store methods map
to the same RPC (`Search` and `SearchDiscovery` both issue `Query`). Domain
histograms give per-engram-operation latency that transport metrics cannot
express.

**Recording seam:** to keep `store`/`embed`/`auth` from importing the meter
directly, spans are created inline in each method, but **duration is recorded
through a thin `internal/telemetry` helper** that each layer calls:

```go
func RecordStoreOp(ctx context.Context, op string, start time.Time, err error)
func RecordEmbed(ctx context.Context, start time.Time, err error)
func RecordAuthVerify(ctx context.Context, start time.Time, err error)
```

These helpers own the histogram instruments (single registration point),
tolerate a nil meter (no-op when telemetry is off), and derive `outcome` from
`err`. The instruments are created alongside `ToolMetrics` in
`internal/telemetry/metrics.go` and reached via package state initialised during
`telemetry.Setup`. A nil/uninitialised meter makes every call a no-op.

**Import-direction constraint:** the dependency is one-way —
`store`/`embed`/`auth` import `go.opentelemetry.io/otel` (for the inline tracer)
and `internal/telemetry` (for the metric helpers); `internal/telemetry` MUST NOT
import `internal/store`/`embed`/`auth` (it has no such import today). The helpers
are layer-agnostic — they take a plain `operation string`, never an
engram-domain type — which keeps the direction acyclic.

### Identity & the slog correctness rule

- Spans carry **`engram.owner`** (the opaque OIDC `sub` via `subj.Owner()`)
  only. `actor` (email/username) stays on logs — it is PII and must not enter
  the trace backend.
- **Hard rule (existing convention, memory `79d2ee6f`):** only the stdout slog
  handler is wrapped in `traceContextHandler`, which stamps `trace_id`/`span_id`
  from the context's span. Therefore **every log call on these new seams MUST
  use the `*Context` slog variants** (`InfoContext`/`WarnContext`/
  `ErrorContext`) or the trace IDs silently drop from stdout. The OTLP bridge
  resolves context itself and must not be double-wrapped.

### Resource attributes (idiomatic best-practice set)

Rebuild the resource in `providers.go` to opt into the full standard detector
set (replacing the current `WithFromEnv` + manual-attributes-only construction):

```go
res, err := resource.New(ctx,
    resource.WithFromEnv(),        // OTEL_RESOURCE_ATTRIBUTES + OTEL_SERVICE_NAME
    resource.WithTelemetrySDK(),   // telemetry.sdk.name|language|version
    resource.WithProcess(),        // process.pid, process.executable.*, process.runtime.*, process.owner, process.command_args
    resource.WithHost(),           // host.name
    resource.WithHostID(),         // host.id
    resource.WithOS(),             // os.type, os.description
    resource.WithContainer(),      // container.id (docker/k8s cgroup)
    resource.WithAttributes(       // schemaless — no schema-URL merge conflict
        semconv.ServiceName(cfg.ServiceName),       // "engram"
        semconv.ServiceVersion(cfg.ServiceVersion), // ldflags main.version
        attribute.String("service.instance.id", uuid.New().String()),
    ),
)
```

Full attribute inventory after the change:

| Group | Attributes | Source |
|-------|-----------|--------|
| Service identity | `service.name`, `service.version`, `service.instance.id` | code (version via ldflags) |
| Service context | `service.namespace`, `deployment.environment.name` | operator via env (no code conflict) |
| SDK | `telemetry.sdk.{name,language,version}` | `WithTelemetrySDK` |
| Process | `process.pid`, `process.executable.{name,path}`, `process.runtime.{name,version,description}`, `process.owner`, `process.command_args` | `WithProcess` |
| Host | `host.name`, `host.id` | `WithHost`/`WithHostID` |
| OS | `os.type`, `os.description` | `WithOS` |
| Container | `container.id` | `WithContainer` |
| K8s | `k8s.pod.name`, `k8s.namespace.name`, `k8s.node.name`, `k8s.pod.uid` | chart Downward API → `OTEL_RESOURCE_ATTRIBUTES` |

Decisions:

1. **Precedence:** keep the current ordering — code-set
   `service.name`/`version`/`instance.id` win over `OTEL_*` env (deterministic
   identity). Operators still add `service.namespace` /
   `deployment.environment.name` / k8s attributes freely because the code never
   sets those (no conflict).
2. **`service.instance.id` stays a manual UUID** — the auto-detector is
   experimental. In k8s, `k8s.pod.uid` (Downward API) supplies the stable pod
   identity; the per-process UUID is acceptable for the instance id.
3. **K8s attributes are a chart concern, not code** — the Downward API injects
   pod metadata into `OTEL_RESOURCE_ATTRIBUTES`, which `WithFromEnv()` already
   consumes. The same binary is therefore correct on a laptop, in Docker, and in
   k8s.

**Schema-URL gotcha:** mixing detectors with a `semconv/vX` import can trip
`resource.New` into `ErrSchemaURLConflict`. `WithAttributes` is schemaless (safe)
and the built-in detectors share the SDK's bundled schema, so no conflict is
expected — but a test asserts `resource.New` returns no error, and if it ever
does the SDK degrades gracefully (logs + continues) per ADR `engram-uxh`.

### Chart knobs

Grounding moved sampler + interval from a code task to a chart task — the binary
already honors the standard env vars. New `values.yaml` surface:

```yaml
observability:
  traces:
    sampler: ""        # OTEL_TRACES_SAMPLER (e.g. parentbased_traceidratio)
    samplerArg: ""     # OTEL_TRACES_SAMPLER_ARG (e.g. "0.1")
  metrics:
    exportInterval: "" # OTEL_METRIC_EXPORT_INTERVAL in ms (e.g. "30000")
```

Templated in `templates/memory-mcp.yaml` with `{{- with }}` guards exactly like
the existing `otlpEndpoint` block. Additionally, k8s resource attributes via the
Downward API:

```yaml
- name: POD_NAME
  valueFrom: { fieldRef: { fieldPath: metadata.name } }
- name: POD_NAMESPACE
  valueFrom: { fieldRef: { fieldPath: metadata.namespace } }
- name: NODE_NAME
  valueFrom: { fieldRef: { fieldPath: spec.nodeName } }
- name: POD_UID
  valueFrom: { fieldRef: { fieldPath: metadata.uid } }
- name: OTEL_RESOURCE_ATTRIBUTES
  value: "k8s.pod.name=$(POD_NAME),k8s.namespace.name=$(POD_NAMESPACE),k8s.node.name=$(NODE_NAME),k8s.pod.uid=$(POD_UID){{ with .Values.observability.resourceAttributes }},{{ . }}{{ end }}"
```

The existing `observability.resourceAttributes` value is appended **inline**
(via `{{ with … }},{{ . }}{{ end }}`) so user-supplied attributes merge with the
auto-injected k8s set.

**Plan constraint — replace, do not duplicate:** `templates/memory-mcp.yaml`
already emits a *separate* `OTEL_RESOURCE_ATTRIBUTES` env entry under
`{{- with .Values.observability.resourceAttributes }}`. The plan MUST **remove**
that existing block and fold its value into the single Downward-API-constructed
entry above. Two `OTEL_RESOURCE_ATTRIBUTES` entries in one container is
last-wins in k8s, which would silently drop either the k8s attributes or the
user-supplied ones — exactly one entry must be emitted.

## Error handling & graceful degradation

Unchanged posture (ADR `engram-uxh`). When telemetry is disabled, the global
tracer is a no-op and the metric helpers see a nil meter, so every `tracer.Start`
and `Record*` call is a zero-cost no-op. No new hard dependency, no new failure
mode. Resource construction errors are surfaced exactly as today (returned from
`buildProviders`, falling back to the stdout logger).

## Testing

- **Span assertions** per layer using `sdk/trace/tracetest.SpanRecorder`:
  assert span name, key attributes (`engram.scope`, `engram.owner`,
  `engram.result_count`, `engram.embed.model/dims`), and `codes.Error` status on
  the failure path. Mirrors the `internal/server/instrument_test.go` capture
  pattern.
- **Metric assertions** using `sdk/metric` with a manual reader
  (`metricdata`): assert each histogram records with the correct
  `operation`/`outcome` attributes.
- **Resource test:** assert `resource.New` with the full detector set returns no
  error and that `telemetry.sdk.*`, `process.*`, `host.name`, `os.type` are
  present.
- **Chart test:** `helm template` assertions that sampler/samplerArg/
  exportInterval env vars render only when set, and that the Downward API
  `OTEL_RESOURCE_ATTRIBUTES` renders with the four `$(POD_*)`/`$(NODE_NAME)`
  references.
- Existing integration tests (ephemeral Qdrant via testcontainers, memory
  `b489bb0b`) already exercise the store paths; spans flow through them with no
  new infrastructure.

## File change map

| File | Change |
|------|--------|
| `internal/store/store.go` | package tracer + inline spans in the public methods listed above; call `telemetry.RecordStoreOp` |
| `internal/embed/embed.go` | package tracer + span in `Embed`; call `telemetry.RecordEmbed` |
| `internal/auth/auth.go` | span around the `TokenVerifier` closure; call `telemetry.RecordAuthVerify` |
| `internal/telemetry/metrics.go` | 3 new histograms + `RecordStoreOp`/`RecordEmbed`/`RecordAuthVerify` helpers |
| `internal/telemetry/providers.go` | rebuild resource with full detector set |
| `internal/server/instrument.go` | add `engram.owner` to the existing tool spans |
| `charts/engram/values.yaml` | `observability.traces.*`, `observability.metrics.exportInterval` |
| `charts/engram/templates/memory-mcp.yaml` | template the new env vars + Downward API resource attributes |
| `internal/store/*_test.go`, `internal/embed/*_test.go`, `internal/auth/*_test.go`, `internal/telemetry/*_test.go` | span + metric + resource assertions |

## Risks & mitigations

- **Span/metric cardinality.** Attributes are bounded: `operation` is a fixed
  method name, `outcome` is `ok`/`error`, `engram.owner` is an opaque `sub`.
  `engram.scope` is the one potentially high-cardinality attribute — it is set on
  **spans** (acceptable; spans are sampled) but **not** as a metric dimension
  (metrics use only `operation`/`outcome`).
- **`process.command_args` leakage.** `WithProcess()` captures **all** of
  `os.Args` onto the resource (and thus every exported span/metric/log).
  engram passes config via env, not args; today's flags (`--oidc-issuer`,
  addresses) are non-secret, so this is acceptable. **Durable guard:** if any
  future flag carries a token, key, or secret path, swap `WithProcess` for
  `WithProcessRuntimeName()` + `WithProcessRuntimeVersion()` (which omit
  `command_args`). The plan should leave a comment to this effect at the
  `WithProcess` call site.
- **Schema-URL conflict.** Covered by the resource test above; graceful
  degradation already absorbs a construction error.
<!-- adr-capture: sha256=cdd5ffae3b3e0740; session=cli; ts=2026-06-11T18:27:19Z; adrs=engram-6gb,engram-wot,engram-7qd,engram-9tj -->
