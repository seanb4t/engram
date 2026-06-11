<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-6gb; do not edit manually; use `/adr update engram-6gb` -->

# Instrument store/embed/auth with inline spans, not a decorator layer

**Date:** 2026-06-11
**Status:** Accepted
**Decision:** engram-6gb
**Deciders:** Sean Brandt

## Context

engram's three inner packages (internal/store, internal/embed, internal/auth) previously had zero direct OpenTelemetry instrumentation; they were visible only through transport-level otelgrpc/otelhttp auto-instrumentation. Adding domain spans requires choosing between inline spans (each public method calls otel.Tracer directly) and a decorator/wrapper interface layer that intercepts calls without coupling the package to the OTel API.

## Decision

Each public method in store, embed, and auth creates its own span inline via a package-level otel.Tracer (attributes at creation, RecordError+SetStatus on the error path, defer span.End()). Per-operation duration metrics are delegated to thin helper functions in internal/telemetry (RecordStoreOp/RecordEmbed/RecordAuthVerify) so domain packages never import the meter; the dependency is one-way (domain -> telemetry, never the reverse).

## Rationale

- The Go OTel SDK guarantees a no-op tracer/meter when no provider is registered, so inline spans cost nothing when telemetry is disabled (preserves ADR engram-uxh).\n- Samplers only observe attributes set at span creation; inline instrumentation is the only pattern that can set engram.owner/engram.scope before the sampling decision.\n- A decorator over 19 store methods adds more abstraction than the problem warrants, and result-derived attributes (result_count, error detail) cannot be set by an outer wrapper without return-value inspection.\n- Routing metric recording through internal/telemetry keeps histogram instruments at a single registration point.

## Alternatives Considered

- Decorator / wrapper interface layer: isolates telemetry outside domain packages and is easy to swap or disable, but requires new interface types for ~19 store methods, doubles the API surface, adds indirection, and cannot set in-method attributes from outside. Rejected as over-abstracted and non-idiomatic for Go OTel.

## Consequences

Positive: span attributes are available at the correct call site with no return-value inspection; new store methods are instrumented by following the in-file pattern; no new interface types. Negative: store/embed/auth now import go.opentelemetry.io/otel, and reversing the decision touches every instrumented method; instrumentation logic is distributed across three packages. Neutral: the telemetry helper boundary separates the tracing concern (inline) from the metrics concern (centralized).
