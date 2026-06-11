<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-7qd; do not edit manually; use `/adr update engram-7qd` -->

# Reuse OTel-standard env vars for sampler and export interval; add no MEM_* equivalents

**Date:** 2026-06-11
**Status:** Accepted
**Decision:** engram-7qd
**Deciders:** Sean Brandt

## Context

engram's config convention is env-first MEM_* vars with cobra flag overrides. The OTel SDK already reads OTEL_TRACES_SAMPLER, OTEL_TRACES_SAMPLER_ARG, and OTEL_METRIC_EXPORT_INTERVAL natively (sdktrace.NewTracerProvider and metric.NewPeriodicReader honor them when no WithSampler/WithInterval option is passed, which is engram's case). The chart did not previously expose these knobs. The design must choose whether to surface them as MEM_* vars or template the OTel-standard vars directly.

## Decision

Sampler and metric export interval are configured exclusively through the OTel-standard env vars. The Helm chart templates them via observability.traces.sampler, observability.traces.samplerArg, and observability.metrics.exportInterval, with no MEM_* counterparts and no Go code change.

## Rationale

- Deepwiki grounding confirmed the SDK reads these vars without explicit code; MEM_* wrappers would duplicate SDK behavior with no benefit and risk precedence bugs if both namespaces were set.\n- Telemetry tuning (OTEL_*) is already a distinct concern from engram domain config (MEM_*); sampler and interval are SDK concerns.\n- Avoids minting a new MEM_ knob for every future OTel-standard var.

## Alternatives Considered

- Invent MEM_TRACES_SAMPLER / MEM_METRIC_EXPORT_INTERVAL: consistent with the MEM_* namespace, but requires code to parse and forward to the SDK, duplicating native behavior and risking double-set precedence bugs. Rejected.

## Consequences

Positive: no code change; any OTel-standard sampler works immediately; operators can override with standard OTEL_ vars outside the chart. Negative: the config convention is split (MEM_* domain vs OTEL_* telemetry) and contributors must know which namespace owns which knob. Neutral: the chart observability block is the canonical operator interface, with direct OTEL_ override as an escape hatch.
