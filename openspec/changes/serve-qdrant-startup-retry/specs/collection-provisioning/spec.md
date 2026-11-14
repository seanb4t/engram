# Spec Delta

## ADDED Requirements

### Requirement: Connect-and-create retries while Qdrant is unavailable

`engram serve` SHALL retry the connect-and-create step with backoff while Qdrant is unavailable. Unavailable means a gRPC `Unavailable` status, a failed dial, or an attempt that ran out of its own per-attempt budget. Any other error SHALL fail startup at once. Other engram commands SHALL NOT retry.

#### Scenario: Qdrant comes up within the budget

- **WHEN** `engram serve` starts while Qdrant refuses connections, and Qdrant starts accepting them while at least 7.5 s of `ENGRAM_QDRANT_STARTUP_TIMEOUT` remain
- **THEN** the same process answers `GET /readyz` with HTTP 200, and the log holds one warn-level entry per failed attempt

#### Scenario: Attempt runs out of its own budget

- **WHEN** one connect-and-create attempt does not finish within its per-attempt budget and the startup timeout has time left
- **THEN** the log holds a warn-level entry for that attempt, and the next attempt runs in the same process

#### Scenario: Budget runs out

- **WHEN** Qdrant stays unavailable for longer than `ENGRAM_QDRANT_STARTUP_TIMEOUT`
- **THEN** `engram serve` exits with code 5 and an error naming the connect-and-create step and the timeout

#### Scenario: Error that is not transient

- **WHEN** the connect-and-create step fails with an error that is not unavailable, for example `PermissionDenied`
- **THEN** `engram serve` exits at once with the exit code it uses for that error today, without a second attempt

#### Scenario: CLI verb against an unreachable Qdrant

- **WHEN** an `engram` command other than `serve` opens the store while Qdrant refuses connections
- **THEN** it fails on the first attempt with exit code 5

### Requirement: Each failed connect-and-create attempt is logged

`engram serve` SHALL log every failed connect-and-create attempt at warn level, with the attempt number and the error. This includes the last attempt, whether it ended in exhaustion, in an error that is not transient, or in cancellation.

#### Scenario: Three failures then success

- **WHEN** the connect-and-create step fails three times with `Unavailable` and then succeeds
- **THEN** the log holds three warn-level entries, numbered 1 to 3, each with its error

#### Scenario: Exhaustion

- **WHEN** every attempt fails until the startup timeout runs out
- **THEN** the log holds one warn-level entry per attempt, the last attempt included

### Requirement: Startup timeout setting

engram SHALL accept `ENGRAM_QDRANT_STARTUP_TIMEOUT`, a positive duration defaulting to `60s`, as the total time `engram serve` spends on the connect-and-create step. Configuration validation SHALL reject a value that is zero, negative or not a duration, and name the variable.

#### Scenario: Unset

- **WHEN** `ENGRAM_QDRANT_STARTUP_TIMEOUT` is unset
- **THEN** the effective startup timeout is 60 seconds

#### Scenario: Smallest positive value

- **WHEN** `ENGRAM_QDRANT_STARTUP_TIMEOUT` is `1ns`
- **THEN** configuration validation accepts it

#### Scenario: Invalid startup timeout

- **WHEN** `ENGRAM_QDRANT_STARTUP_TIMEOUT` is zero, negative, or not a duration
- **THEN** configuration validation fails with an error naming `ENGRAM_QDRANT_STARTUP_TIMEOUT`, before contacting Qdrant

### Requirement: Server serves requests only after provisioning completes

`engram serve` SHALL bind its listeners before it contacts Qdrant or an OIDC issuer. Provisioning is collection creation, the quantization reconcile and payload index provisioning. Until all three complete, every request that passes the cross-origin check, except `GET /livez` and `GET /readyz`, SHALL get HTTP 503 and a `Retry-After` header. A request the cross-origin check refuses SHALL get HTTP 403 in every state. No MCP, Connect or console handler SHALL run before provisioning completes.

#### Scenario: Port accepts connections during provisioning

- **WHEN** `engram serve` is still provisioning and a client opens a TCP connection to its primary port
- **THEN** the connection succeeds and `GET /readyz` returns HTTP 503

#### Scenario: Request during provisioning

- **WHEN** `engram serve` is still provisioning payload indexes and a client sends an MCP `tools/list` POST
- **THEN** the response is HTTP 503 with a `Retry-After` header, and no MCP handler runs

#### Scenario: Cross-origin request during provisioning

- **WHEN** `engram serve` is still provisioning and a browser sends a cross-site POST to the MCP path
- **THEN** the response is HTTP 403, and no MCP handler runs

#### Scenario: Request after provisioning

- **WHEN** provisioning completed and a client sends an authenticated MCP `tools/list` request
- **THEN** the response is HTTP 200 and its result lists the memory tools

#### Scenario: Quantization rejected by Qdrant

- **WHEN** Qdrant rejects the quantization update during startup
- **THEN** `engram serve` exits non-zero with an error naming the quantization step, and never runs an MCP, Connect or console handler

### Requirement: Shutdown signal during startup is a normal stop

If `engram serve` receives SIGTERM or SIGINT before provisioning completes, it SHALL stop retrying and close its listeners. It SHALL then exit with code 0, as it does for a signal after start.

#### Scenario: Signal during an attempt

- **WHEN** SIGTERM arrives while a connect-and-create attempt is running
- **THEN** `engram serve` exits with code 0 without another attempt

#### Scenario: Signal during schema convergence

- **WHEN** SIGTERM arrives while `engram serve` builds payload indexes
- **THEN** `engram serve` exits with code 0 without waiting for `ENGRAM_QDRANT_SCHEMA_TIMEOUT`

#### Scenario: Signal during the backoff wait

- **WHEN** SIGTERM arrives while `engram serve` waits between two attempts
- **THEN** `engram serve` exits with code 0 without another attempt

## MODIFIED Requirements

### Requirement: Startup provisioning has separate time budgets

`engram serve` SHALL bound each connect-and-create attempt by 15 seconds, and all attempts together by `ENGRAM_QDRANT_STARTUP_TIMEOUT`. No attempt SHALL run past the end of the startup timeout. It SHALL bound schema convergence (the quantization reconcile and payload index provisioning) separately by `ENGRAM_QDRANT_SCHEMA_TIMEOUT`, a positive duration defaulting to `2m`. Exceeding either budget SHALL fail startup with an error naming the step.

#### Scenario: Slow index build within the schema budget

- **WHEN** payload index provisioning takes longer than 15 seconds but less than `ENGRAM_QDRANT_SCHEMA_TIMEOUT`
- **THEN** startup succeeds

#### Scenario: Schema budget exceeded

- **WHEN** schema convergence does not finish within `ENGRAM_QDRANT_SCHEMA_TIMEOUT`
- **THEN** `engram serve` exits non-zero with an error naming schema provisioning and the timeout

#### Scenario: Invalid schema timeout

- **WHEN** `ENGRAM_QDRANT_SCHEMA_TIMEOUT` is zero, negative, or not a duration
- **THEN** configuration validation fails with an error naming `ENGRAM_QDRANT_SCHEMA_TIMEOUT`

#### Scenario: Startup timeout shorter than one attempt

- **WHEN** `ENGRAM_QDRANT_STARTUP_TIMEOUT` is `2s` and Qdrant accepts the connection but never answers
- **THEN** `engram serve` exits with code 5 before 15 seconds pass

### Requirement: Chart startup window covers the schema budget

The Helm chart SHALL expose `memory.qdrant.quantization`, `memory.qdrant.schemaTimeoutSeconds` and `memory.qdrant.startupTimeoutSeconds`. It SHALL pass them to the server as `ENGRAM_QDRANT_QUANTIZATION`, `ENGRAM_QDRANT_SCHEMA_TIMEOUT` and `ENGRAM_QDRANT_STARTUP_TIMEOUT`. If either timeout is not a positive whole number of seconds, the chart SHALL fail to render.

#### Scenario: Raised schema timeout

- **WHEN** the chart is rendered with `memory.qdrant.schemaTimeoutSeconds: 600`
- **THEN** the server container's environment sets `ENGRAM_QDRANT_SCHEMA_TIMEOUT=600s`

#### Scenario: Raised startup timeout

- **WHEN** the chart is rendered with `memory.qdrant.startupTimeoutSeconds: 180`
- **THEN** the server container's environment sets `ENGRAM_QDRANT_STARTUP_TIMEOUT=180s`

#### Scenario: Default render

- **WHEN** the chart is rendered with default values
- **THEN** the server container's environment sets `ENGRAM_QDRANT_QUANTIZATION=int8` and `ENGRAM_QDRANT_STARTUP_TIMEOUT=60s`

#### Scenario: Invalid startup timeout value

- **WHEN** the chart is rendered with `memory.qdrant.startupTimeoutSeconds: 0`
- **THEN** rendering fails with an error naming `memory.qdrant.startupTimeoutSeconds`

## REMOVED Requirements

### Requirement: Server listens only after provisioning completes

**Reason**: **BREAKING.** The server now binds before provisioning, so `/livez` and `/readyz` can answer while it waits for Qdrant. The guarantee that matters, that no request handler runs before provisioning completes, moves to "Server serves requests only after provisioning completes".

**Migration**: A check that relied on the port being closed during provisioning must use `GET /readyz` instead. The chart's probes and the e2e server start-up wait change in the same release.
