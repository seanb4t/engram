<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-dwi; do not edit manually; use `/adr update engram-dwi` -->

# Export telemetry via OTLP only; omit a Prometheus scrape endpoint

**Date:** 2026-06-07
**Status:** Accepted
**Decision:** engram-dwi
**Deciders:** Sean Brandt

## Context

engram targets a Grafana LGTM / OpenTelemetry Collector backend on Kubernetes. Metrics can be exposed via a Prometheus /metrics scrape (pull) or pushed via OTLP. The choice constrains the deployment topology and the dependency set.

## Decision

Export metrics, traces, and logs exclusively over OTLP gRPC to a collector. No Prometheus /metrics endpoint is added. Telemetry is enabled only when OTEL_EXPORTER_OTLP_ENDPOINT is set.

## Rationale

- Matches the project's Grafana LGTM/collector backend target.\n- One transport carries all three signal types, shrinking the configuration surface.\n- OTel SDK is already transitively present; no Prometheus client_go dependency.\n- Helm chart defaults to telemetry-off (no endpoint), keeping the chart generic.

## Alternatives Considered

**Prometheus /metrics scrape** (rejected): conventional for Go servers and works without a collector, but covers metrics only (no traces/logs over the same channel), needs a separate scrape job and a net/http route, and adds the prometheus/client_go dependency.

## Consequences

Positive: metrics/traces/logs share one gRPC channel and one collector config; no /metrics route to maintain; generic Helm default. Negative: a collector (Grafana Alloy or otel-collector) must be deployed; no scrape fallback if the collector is down. Neutral: a Prometheus endpoint stays out of scope (YAGNI) and can be added later without architectural conflict.
