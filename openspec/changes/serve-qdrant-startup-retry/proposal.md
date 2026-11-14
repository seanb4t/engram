# Proposal

## Why

`engram serve` exits on its first failed Qdrant call. If Qdrant starts a few seconds after engram, for example when both roll out together, the pod crash-loops until Qdrant is up. On 2026-09-29, engram exited with code 5 and restarted twice in such a rollout. The chart's `tcpSocket` probes also cannot tell a server whose Qdrant is down from a healthy one. This change comes from #1013, which was split from #684.

## What Changes

- `engram serve` retries the connect-and-create provisioning step while Qdrant is unavailable, with backoff, for up to `ENGRAM_QDRANT_STARTUP_TIMEOUT` (default `60s`). It logs each failed attempt. Any other error still fails startup at once, with today's exit code. When the budget runs out, startup fails with exit code 5, as it does today.
- `engram serve` binds its HTTP listeners before it contacts Qdrant or an OIDC issuer. Until provisioning completes, the listeners answer only `/livez` (200) and `/readyz` (503). Every other request gets 503 with a `Retry-After` header. **BREAKING** for the requirement "Server listens only after provisioning completes": the port now accepts connections during provisioning.
- A SIGTERM or SIGINT during startup stops the retry and exits with code 0, like a normal stop.
- New `GET /livez` route: 200 while the process runs.
- New `GET /readyz` route. It answers 200 only after provisioning completes and while the last background Qdrant health check passes. A probe never triggers a Qdrant call.
- The Helm chart gets `memory.qdrant.startupTimeoutSeconds` (default 60). The startup and liveness probes move to `httpGet /livez`, and the readiness probe moves to `httpGet /readyz`.
- `configure.md` documents the new key. `deploy.md` documents the retry, the routes and the probes.

Out of scope:

- OIDC discovery at startup gets no retry.
- The schema-convergence step (quantization and payload indexes) gets no retry.
- Reconnect behavior for requests after startup does not change, with one exception. The serve client keeps its 5 s gRPC reconnect cap for its whole life, so a reconnect after a Qdrant outage comes sooner than with the 120 s default. `deploy.md` names this.
- The other telemetry work in #684 stays in #684.

## Capabilities

### New Capabilities

- `server-health`: the liveness and readiness routes of `engram serve`, what each reports, and how the Helm chart probes them.

### Modified Capabilities

- `mcp-listener-isolation`: the second listener's 404 rule holds after provisioning. Before it, the startup gate answers 503.
- `collection-provisioning`: the connect-and-create step retries while Qdrant is unavailable, under a new startup budget. The server binds its port before provisioning and answers 503 until provisioning completes. The chart's startup window no longer has to cover provisioning, because liveness answers during it.

## Impact

- Code: `cmd/engram/serve.go` (bind order, the not-ready gate, health routes), `internal/server/tools.go` (retry in the serve provisioning path only), `internal/store` (`IsUnavailable`, `HealthCheck`), `internal/config` (new key, and one shared positive-duration check).
- Tests: the `internal/e2e` server start-up wait polls `/readyz`, and `task chart:validate` drops its startup-window check for the new probe checks.
- MCP tool schemas: deliberately unchanged. Before provisioning completes, an MCP request gets HTTP 503, not a tool result.
- Connect proto: deliberately unchanged. A Connect request during provisioning gets HTTP 503.
- CLI: `engram serve` behavior changes as above. Every other verb is deliberately unchanged. If Qdrant is unreachable, each one still fails at once with exit code 5.
- Console (`ui/`): deliberately unchanged.
- Helm chart: new value `memory.qdrant.startupTimeoutSeconds`, new env `ENGRAM_QDRANT_STARTUP_TIMEOUT`, and all three probes change.
- docs-site: `guides/configure.md`, `guides/deploy.md`, and a new numbered entry under Unreleased in `guides/upgrade.md`.
- engram skill: deliberately unchanged.
- Compatibility: `ENGRAM_QDRANT_STARTUP_TIMEOUT` is a new, additive `ENGRAM_` config key. `/livez` and `/readyz` are new public, unauthenticated routes on the primary listener. The open-port-during-provisioning behavior changes what an external TCP check sees. An operator with a custom `tcpSocket` readiness check must switch to `/readyz`.
