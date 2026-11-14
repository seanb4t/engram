# collection-provisioning Specification

## Purpose

Defines how engram creates and reconciles its Qdrant collections: vector parameters, payload indexes, and vector quantization. It also defines which entry points may change which collection settings, and the time budgets that bound startup provisioning.

## Requirements

### Requirement: Collection created when absent

engram SHALL create its memory collection when it does not exist, with the configured embedding dimension and cosine distance. It SHALL leave an existing collection's vector size and distance unchanged.

#### Scenario: First start against an empty Qdrant

- **WHEN** `engram serve` starts and the configured collection does not exist
- **THEN** the collection exists with vector size equal to `ENGRAM_EMBED_DIM` and cosine distance before the server accepts connections

#### Scenario: Existing collection

- **WHEN** `engram serve` starts and the configured collection already exists
- **THEN** its vector size and distance are unchanged

### Requirement: Recall payload indexes ensured on every provisioning

engram SHALL ensure its recall payload indexes on every provisioning of a collection, creating any that are missing and leaving existing ones intact. It SHALL NOT require a data migration to add an index to an existing collection.

#### Scenario: Existing collection missing an index

- **WHEN** an existing collection lacks one of the recall payload indexes (`owner`, `scope`, `created_at`, `short_id`, `schema_version`, `tags`, `visibility`, `superseded_by`, `archived_at`, `not_before`, `not_after`) and the collection is provisioned
- **THEN** the missing index exists afterwards, and the existing indexes and stored records are unchanged

#### Scenario: All indexes already present

- **WHEN** every recall payload index already exists and the collection is provisioned
- **THEN** provisioning succeeds without error

### Requirement: Payload index build time is logged

engram SHALL log each payload index it builds during provisioning at info level, with the field name and the time the build took. An index that already existed SHALL be logged at debug level only, so CLI commands stay quiet on a provisioned collection.

#### Scenario: Provisioning builds missing indexes

- **WHEN** a collection with no payload indexes is provisioned
- **THEN** the log contains one info-level entry per recall payload index naming the field and its duration

#### Scenario: Provisioning an already-indexed collection

- **WHEN** a collection that already carries every recall payload index is provisioned
- **THEN** no index entry is logged above debug level

### Requirement: Quantization mode setting

engram SHALL accept a vector quantization mode through `ENGRAM_QDRANT_QUANTIZATION` with the values `int8`, `off`, and `unmanaged`, defaulting to `int8`. Configuration validation SHALL reject any other value and name the variable.

#### Scenario: Unset

- **WHEN** `ENGRAM_QDRANT_QUANTIZATION` is unset
- **THEN** the effective mode is `int8`

#### Scenario: Invalid value

- **WHEN** `ENGRAM_QDRANT_QUANTIZATION` is set to a value other than `int8`, `off`, or `unmanaged`
- **THEN** the command fails configuration validation with an error naming `ENGRAM_QDRANT_QUANTIZATION`, before contacting Qdrant

### Requirement: int8 mode quantizes the collection

In `int8` mode, `engram serve` SHALL ensure the memory collection carries int8 scalar quantization with quantile 0.99 and the quantized vectors pinned in RAM. This applies to a newly created collection and to an existing one whose quantization is absent or differs, without altering stored vectors or payloads.

#### Scenario: New collection

- **WHEN** `engram serve` starts in `int8` mode and creates the collection
- **THEN** the collection's quantization config is int8 scalar, quantile 0.99, memory pinned

#### Scenario: Existing unquantized collection

- **WHEN** `engram serve` starts in `int8` mode against an existing collection with no quantization
- **THEN** the collection's quantization config becomes int8 scalar, quantile 0.99, memory pinned, and every stored record remains retrievable with unchanged content

#### Scenario: Existing collection with different quantization parameters

- **WHEN** `engram serve` starts in `int8` mode against a collection whose quantization differs from int8 scalar, quantile 0.99, memory pinned
- **THEN** the collection's quantization config is replaced with int8 scalar, quantile 0.99, memory pinned

### Requirement: Quantization reconcile is idempotent

engram SHALL change a collection's quantization config only when it differs from the configured mode's target. When it already matches, provisioning SHALL issue no quantization update.

#### Scenario: Restart with matching config

- **WHEN** `engram serve` restarts in `int8` mode and the collection already carries int8 scalar, quantile 0.99, memory pinned
- **THEN** no quantization update is sent to Qdrant

#### Scenario: Restart with off and no quantization

- **WHEN** `engram serve` restarts in `off` mode and the collection carries no quantization
- **THEN** no quantization update is sent to Qdrant

### Requirement: off mode removes quantization

In `off` mode, `engram serve` SHALL ensure the memory collection carries no vector quantization, removing any that is present without altering stored vectors or payloads.

#### Scenario: Turning quantization off

- **WHEN** `engram serve` starts in `off` mode against a collection that carries quantization
- **THEN** the collection carries no quantization afterwards and every stored record remains retrievable with unchanged content

#### Scenario: New collection in off mode

- **WHEN** `engram serve` starts in `off` mode and creates the collection
- **THEN** the collection carries no quantization

### Requirement: unmanaged mode leaves quantization untouched

In `unmanaged` mode, engram SHALL NOT change a collection's quantization config under any entry point.

#### Scenario: Hand-tuned collection

- **WHEN** an operator has set a quantization config by hand and `engram serve` starts in `unmanaged` mode
- **THEN** the collection's quantization config is unchanged

### Requirement: Only the server reconciles quantization

CLI commands other than `engram serve` SHALL NOT change an existing collection's quantization config, whatever `ENGRAM_QDRANT_QUANTIZATION` holds in their environment. They SHALL still create a missing collection and missing payload indexes.

#### Scenario: Operator CLI with a different environment

- **WHEN** the server runs in `off` mode, the collection carries no quantization, and an operator runs `engram prune-expired` with `ENGRAM_QDRANT_QUANTIZATION` unset
- **THEN** the collection still carries no quantization afterwards

### Requirement: Reindex target created with the configured mode

When `engram reindex` creates its target collection, it SHALL apply the configured quantization mode to that new collection. It SHALL NOT change the quantization config of a target collection that already exists.

#### Scenario: Fresh reindex target

- **WHEN** `engram reindex` runs with `ENGRAM_QDRANT_QUANTIZATION=int8` and the target collection does not exist
- **THEN** the target collection is created with int8 scalar quantization, quantile 0.99, memory pinned

#### Scenario: Resumed reindex into an existing target

- **WHEN** `engram reindex` resumes into a target collection that already exists
- **THEN** the target's quantization config is unchanged

### Requirement: Search scores stay exact under quantization

Quantization SHALL NOT change the meaning of a search result's `score`: it SHALL remain the cosine similarity computed from the original stored vectors, not from the quantized copy.

#### Scenario: Same record scored with and without quantization

- **WHEN** the same query is run against the same records in `int8` mode and in `off` mode, and a record is returned in both
- **THEN** its `score` is the same cosine similarity in both modes, within float32 rounding

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

### Requirement: Connect-and-create retries while Qdrant is unavailable

`engram serve` SHALL retry the connect-and-create step with backoff while Qdrant is unavailable. Unavailable means a gRPC `Unavailable` status, a failed dial, or an attempt that ran out of its own per-attempt budget. Any other error SHALL fail startup at once. Other engram commands SHALL NOT retry.

#### Scenario: Qdrant comes up within the budget

- **WHEN** `engram serve` starts while Qdrant refuses connections, and Qdrant starts accepting them while at least 15 s of `ENGRAM_QDRANT_STARTUP_TIMEOUT` remain
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
