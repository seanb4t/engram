<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-uxh; do not edit manually; use `/adr update engram-uxh` -->

# Telemetry is never a hard server startup dependency

**Date:** 2026-06-07
**Status:** Accepted
**Decision:** engram-uxh
**Deciders:** Sean Brandt

## Context

engram is self-hosted and deployed generically via Helm; operators may run without a collector. Telemetry setup can fail (bad endpoint, partition, TLS). The memory service is the primary function; observability is secondary. The design must decide whether a telemetry failure aborts the server.

## Decision

When OTEL_EXPORTER_OTLP_ENDPOINT is unset or telemetry setup fails, Setup returns no-op providers and a no-op shutdown and the server starts normally with a warning. A silent-process guard forces stdout back on when MEM_LOG_STDOUT=false yet no OTLP endpoint exists, so logs are never wholly dropped.

## Rationale

- The Helm chart must ship telemetry-off by default; a hard dependency would break the default install.\n- Exporter runtime failures are handled by SDK batch processors (retry/drop) and must never surface into request handling.\n- The primary function (memory) must not depend on the secondary (observability).

## Alternatives Considered

**Telemetry failure aborts startup** (rejected): fails fast and surfaces misconfiguration immediately, but a misconfigured endpoint would take down the memory service entirely, violating the primary/secondary separation.

## Consequences

Positive: the default Helm install (no endpoint) yields a working, non-crash-looping server; operators enable telemetry incrementally. Negative: a misconfigured endpoint silently disables telemetry, with only a warning log as signal. Neutral: genuine startup-fatal errors (store init, verifier init) remain hard failures; only the observability subsystem degrades.
