<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-f7p; do not edit manually; use `/adr update engram-f7p` -->

# Instrument at three seams: HTTP, MCP method, and downstream clients

**Date:** 2026-06-07
**Status:** Accepted
**Decision:** engram-f7p
**Deciders:** Sean Brandt

## Context

engram's request path has three structurally distinct boundaries where signals arise: the HTTP layer (RequireBearerToken returns 401/403 BEFORE MCP dispatch, so auth failures are invisible deeper down), the MCP method-dispatch layer (where tool and caller identity resolve), and downstream clients (embedder HTTP, Qdrant gRPC). A single wrap at one layer would silently miss whole signal classes.

## Decision

Instrument all three layers: otelhttp.NewHandler plus an access-log/auth-failure middleware at HTTP; AddReceivingMiddleware at the MCP method layer; otelhttp.NewTransport on the embedder client and the otelgrpc stats handler on the Qdrant dial.

## Rationale

- Auth failures are only observable at the HTTP layer because RequireBearerToken short-circuits before MCP dispatch.\n- Tool identity and caller actor/owner are only resolvable at the MCP layer.\n- context.Context propagation makes downstream spans children of the tool span with no signature changes.\n- Three seams is the minimum that captures every structurally distinct signal class.

## Alternatives Considered

**Single wrap at the MCP handler level only** (rejected): one instrumentation point, but auth 401/403s occur before MCP dispatch and are invisible, and HTTP-level fields (remote addr, path, user-agent) are lost.

## Consequences

Positive: one trace ties inbound HTTP -> tool -> embedder -> Qdrant; auth-failure reason captured where the detail exists; log/trace correlation via the otelslog bridge. Negative: embed.New() needs a transport-injection seam (an internal API change). Neutral: Qdrant instrumentation is a dial-site config change via qdrant.Config.GrpcOptions, no fork.
