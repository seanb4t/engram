<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-9tj; do not edit manually; use `/adr update engram-9tj` -->

# Inject k8s resource attributes via chart Downward API, not Go SDK detectors

**Date:** 2026-06-11
**Status:** Accepted
**Decision:** engram-9tj
**Deciders:** Sean Brandt

## Context

A production-grade OTel resource needs k8s attributes (pod name, namespace, node name, pod UID) when running in Kubernetes. The Go OTel SDK offers experimental (x.Resource-gated) k8s detectors, but the engram binary runs on laptops, in Docker, and in k8s. The design must decide where k8s attribute injection belongs: the binary (SDK detectors) or the deployment layer (Helm chart).

## Decision

K8s resource attributes are injected by the Helm chart via the Downward API into OTEL_RESOURCE_ATTRIBUTES, which resource.WithFromEnv() already consumes. The binary's resource.New() adds no k8s detector. Exactly one OTEL_RESOURCE_ATTRIBUTES env entry is emitted (k8s attrs folded with any user-supplied observability.resourceAttributes) to avoid k8s last-wins duplication.

## Rationale

- Keeps the binary deployment-agnostic: the same image is correct on a laptop, in Docker, and in k8s, with the orchestrator supplying its own identity (12-factor).\n- The k8s SDK detector is experimental; the Downward API is stable Kubernetes functionality.\n- OTEL_RESOURCE_ATTRIBUTES is already consumed by WithFromEnv(); no new binary code path.\n- Clear responsibility boundary: the chart knows deployment topology, the binary should not.

## Alternatives Considered

- Go k8s resource detector in binary: self-contained and needs no chart config, but uses an experimental API, adds a k8s API client dependency, requires a pod-metadata API call at startup, and makes the binary k8s-aware. Rejected.

## Consequences

Positive: no binary dependency on the k8s API server; non-k8s deployments are unaffected and need no stub. Negative: running the binary in k8s without the chart (raw manifests) means k8s attrs are absent unless the Downward API env block is replicated manually. Neutral: k8s.pod.uid from the Downward API gives a stable pod identity alongside the per-process UUID service.instance.id.
