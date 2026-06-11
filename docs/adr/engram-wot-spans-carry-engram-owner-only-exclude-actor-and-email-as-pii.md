<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-wot; do not edit manually; use `/adr update engram-wot` -->

# Spans carry engram.owner only; exclude actor and email as PII

**Date:** 2026-06-11
**Status:** Accepted
**Decision:** engram-wot
**Deciders:** Sean Brandt

## Context

When adding caller identity as a span attribute across the store/embed/auth/tool spans, the design must choose which identity fields to surface in the trace backend. The server tracks two identity fields: owner (opaque OIDC sub, the authz key) and actor (human-readable email or preferred_username). Trace backends (e.g. Tempo, Jaeger) store spans durably and may be readable by operators who should not see PII.

## Decision

Span attributes across every instrumented seam carry engram.owner (the opaque OIDC sub) and never actor. Actor (email/username) remains only on structured log lines, correlated to spans via trace_id/span_id.

## Rationale

- Email and preferred_username are PII; trace backends must not become a PII store (data-minimization).\n- The opaque sub is sufficient for cross-request correlation and per-owner latency analysis without exposing human-readable identity.\n- Actor already appears on structured access-log lines; logs are the correct channel for human-readable identity, and the slog *Context-variant rule keeps them correlated to the trace.

## Alternatives Considered

- owner + actor (sub + email/username): human-readable identity on spans speeds debugging, but embeds PII in traces, attaching GDPR/CCPA obligations to trace retention. Rejected.\n- No caller identity on spans: zero PII risk, but cannot correlate a slow/erroring span to an owner, defeating per-user latency analysis. Rejected.

## Consequences

Positive: trace backends stay PII-free; consistent with the authz model where sub is the canonical identity. Negative: debugging a specific user's trace requires a sub->actor lookup via logs; rotated subs cannot be re-attributed in historical traces. Neutral: actor stays available on trace-correlated log lines.
