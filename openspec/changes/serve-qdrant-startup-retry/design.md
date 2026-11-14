# Design

## Context

See proposal.md, Why. Today `runServe` in `cmd/engram/serve.go` runs in this order:

1. It loads the configuration with `config.Load`, runs the serve-only shape checks and validates the listen addresses. `Config.Validate` runs later, inside `buildDepsFromEnv`.
2. It builds the auth chain. Each OIDC discovery has its own 15 s budget.
3. It calls `server.Register`. `Register` first runs `buildDepsFromEnv`, which loads config, builds the store through `ensureStoreFromConfig`, and provisions it with `provisionStore`. Provisioning creates the collection (15 s budget) and converges the schema (`ENGRAM_QDRANT_SCHEMA_TIMEOUT`). `buildDepsFromEnv` also builds the embedder, the decision provider and the queues. Only after that does `Register` mount the MCP, Connect and console routes.
4. It mounts the remaining routes and builds the listeners with `newListeners`. `newHTTPServer` wraps each listener's whole mux in `newCrossOriginProtection().Handler(mux)`.
5. `listenAll` binds the sockets, and `serveAll` serves until a signal arrives.

These parts of the repository depend on the closed port:

- The chart's `tcpSocket` probes.
- The `internal/e2e` server start-up wait in `startServer` (`harness_test.go`), which treats a TCP connect as ready.
- The `chart:validate` task, which asserts a startup window of at least 615 s for `schemaTimeoutSeconds: 600` (`Taskfile.yaml`).

Libraries in use: `cenkalti/backoff/v7`, which `internal/server/summaryqueue.go` already uses through `newRetryBackOff`, and `pgregory.net/rapid` for property tests. `cmd/engram/operror.go` holds `isUnavailableTransportErr` (gRPC `Unavailable` or a `net.Error`), and `classifyOperatorErrConstruction` maps it to exit code 5.

## Goals / Non-Goals

Goals:

- Bind first, and gate every request handler on provisioning (see `specs/collection-provisioning`).
- Retry only the connect-and-create step, only in `serve`, inside a hard total budget.
- Readiness that reflects Qdrant reachability, at no per-probe cost (see `specs/server-health`).

Non-Goals:

- Retrying `EnsureSchema`. A schema failure is not a transient connection failure, and it already has its own budget.
- Retrying OIDC discovery. It now runs after the bind, so a slow issuer no longer holds the port closed, but it still fails startup as today.
- Readiness for the embedder, the summarizer or the decision provider. Readiness covers Qdrant only.
- Health routes on the MCP-only listener (`ENGRAM_MCP_LISTEN_ADDR`). The chart probes the primary listener only.

## Decisions

### D1. A startup gate inside each listener's cross-origin wrapper

A gate handler goes between each listener's cross-origin wrapper and its routes. `newHTTPServer` keeps building every server, and its `Handler` becomes `newCrossOriginProtection().Handler(gate)`. Every route, the gate's own routes included, stays covered in one place. So a cross-origin unsafe-method request gets 403 from the wrapper in every state, before the gate runs, as it does today. The requirement covers the requests the cross-origin check admits. No handler runs for any request before provisioning completes. `/livez` and `/readyz` join `reservedMCPRoots`, so `ENGRAM_MCP_PATH` cannot name them. The gate holds an `atomic.Pointer[http.Handler]` target and serves:

- `GET /livez`: always 200 (primary listener only).
- `GET /readyz`: 503 by default. If the target is set and the health loop's last ping succeeded, 200 (primary listener only).
- Anything else: 503 with `Retry-After: 5` until the target is set, then the target.

The target pointer is the only record of "provisioned". The health loop holds only the last ping result (D4), so the two values never disagree. A `markProvisioned(gates, startHealth)` function sets the target of every gate and only then calls `startHealth`. So a primary `/readyz` of 200 implies every listener serves its routes, the MCP-only listener included. Each target is set once and never cleared.

Why a gate and not registering routes on a live mux: `http.ServeMux` allows concurrent registration, but then a route can answer before its dependencies exist. The MCP and console routes also need values that `Register` returns. The gate makes "no handler runs before provisioning" one atomic switch, which is easy to test.

Alternative rejected: keep the bind order and add `/readyz` only after start. The owner chose the early bind, so the probes see the server while it retries.

### D2. Bind first, then start up behind the gate

`serveAll` splits into a start step, which runs `Serve` for each bound listener, and the existing wait-and-shutdown step. The signal context moves to the top of `runServe`. `runServe` then reads:

1. Load the configuration, run the serve-only shape checks and validate the listen addresses (no network). `Config.Validate` stays in `buildDepsFromEnv`, after the bind and before any Qdrant call. So an invalid `ENGRAM_QDRANT_STARTUP_TIMEOUT` fails after the bind but before engram contacts Qdrant, with today's exit code for a validation error.
2. Build the listeners and gates, bind, and start serving the gates.
3. Build the auth chain, run the guards, and call `server.Register(ctx, ...)` with the signal context. This is where provisioning runs.
4. Mount the remaining routes, set every gate's target, and start the health loop.
5. Wait for a signal or a serve failure.
6. Shut down and drain, as today.

The wait-and-shutdown step splits once more: a wait for the signal or a serve error, then a shutdown function. If step 3 or 4 fails, `runServe` skips the wait and calls that shutdown function directly. The drain is a no-op, because `Register` returned none. One function shuts the listeners down in both paths. Then a small `startupResult(sigCtx, err)` function decides the return value. If the signal context is done, it returns nil, so a signal during startup exits with code 0 like a normal stop. In every other case, it returns the startup error unchanged, so its exit code stays what it is today.

`newListeners(cfg)` no longer takes the mux or the MCP handler, because the auth chain that `mcpTransport` needs does not exist at bind time. It returns the listeners with their gates. In step 4, the primary gate's target becomes the shared mux and the MCP-only gate's target becomes `newMCPOnlyMux(...)`. `TestServeBothListeners` changes to bind, set both gate targets and use the split serve steps.

Steps 3 and 4 move into a new `startServe(ctx, ...)` function. It takes the register-and-provision step as a function parameter and returns the gate targets, so tests drive it without a binary. `runServe` calls it with the real `server.Register`.

`Register` gains a leading `ctx context.Context` parameter and passes it to `buildDepsFromEnv`. Its other callers are tests, which pass `context.Background()`.

### D3. Retry in the serve provisioning path only

`ensureStoreFromConfig` gains a `ctx` and a `startupPolicy` value: `ensureStoreFromConfig(ctx, cfg, policy, opts...)`. The zero `startupPolicy` means one create attempt and the Qdrant client's default version check. `StoreFromEnv` and the CLI constructors pass `context.Background()` and the zero value. `buildDepsFromEnv`, which only `serve` reaches, passes the signal context and a policy built from `ENGRAM_QDRANT_STARTUP_TIMEOUT`. The parse, build and provision sequence stays in that one function. The retry belongs to the serve entry point, not to the store. So it does not travel as a `store.Option`, as `WithQuantization` does.

`provisionStore` gains a `ctx` and the create step as parameters: `provisionStore(ctx, st, dim, create, schemaBudget)`. The schema step, its budget and its error text stay in that one function. `ensureStoreFromConfig` passes it the caller's `ctx`, which for serve is the signal context, never the startup parent context below. Schema convergence runs under `context.WithTimeout(ctx, schemaBudget)`. So a signal ends it, and the startup timeout never cuts it short.

With a non-zero policy, the startup clock starts before the Qdrant client exists. `ensureStoreFromConfig` first creates a parent context, `context.WithTimeout(ctx, startupTimeout)`. The parent bounds the client construction and the create attempts, and nothing else:

- The Qdrant Go client checks the server version while it is constructed, with a 1 minute default timeout and no context. With a policy, the client gets `VersionCheckTimeout` set to the smaller of the startup timeout and 15 s. A failed version check only logs a client warning, and it does not fail construction. A signal during the check waits at most that bound, and then D2 turns it into exit code 0.
- With a policy, the client also gets `grpc.WithConnectParams`, built by `store.StartupConnectParams()`. That option replaces the whole gRPC backoff configuration. So the function starts from `backoff.DefaultConfig` and sets only `MaxDelay` to 5 s, the retry's own maximum interval. It also sets `MinConnectTimeout` to 20 s, the gRPC default. gRPC's default reconnect backoff grows to 120 s, and a subchannel in that wait fails each RPC at once without dialing. Then a Qdrant that comes up late in the budget is not reached before the budget ends. The 5 s cap keeps every reconnect within one retry interval.
- The connect parameters travel in the existing `opts ...grpc.DialOption` list of `store.NewQdrantClient`, the upstream vocabulary its doc names for caller options. The version check timeout is a `qdrant.Config` field with no dial-option form, so it gets its own parameter: `store.NewQdrantClient(host, port, versionCheckTimeout, opts...)`, where zero keeps the client default. `NewQdrantClient` stays the only function that calls `qdrant.NewClient`, so the client-construction convergence test keeps its one sanctioned site. `storeFromConfig` gains the timeout and the dial options and passes them through. `ensureStoreFromConfig` passes the policy's values, so the serve path gets them. `StoreAndEmbedderFromEnvNoEnsure` and the test callers pass zero and no options.
- Each attempt runs under `context.WithTimeout(parent, collectionCreateBudget)`. An attempt therefore ends at 15 s or at the end of the startup timeout, whichever comes first. No attempt runs past the total budget.
- The backoff comes from `newRetryBackOff`, given an initial interval and a maximum interval as arguments (500 ms and 5 s here). The summary queue passes its current values. `WithMaxElapsedTime` is also set to the startup timeout. The default randomization factor of 0.5 makes the longest wait 7.5 s, and backoff refuses a wait longer than the time left. So the retry promises to reach a Qdrant that comes up while at least 7.5 s of the startup timeout remain. A Qdrant that comes up later can miss the last attempt and get exit code 5.
- While the parent context is live, two kinds of attempt error are transient. The first is any error that `store.IsUnavailable` reports. That includes a bare `context.DeadlineExceeded`, because it implements `net.Error`. The second is a gRPC status with code `DeadlineExceeded`, which is what a call cut off by its own 15 s budget returns. Every other error is wrapped in `backoff.Permanent`. Once the parent context is done, every attempt error ends the loop.
- The operation closure logs each failed attempt at warn level, with an attempt counter, the elapsed time and the error, before it returns. `WithNotify` is not used, because backoff does not call it for the final attempt.

After `backoff.Retry` returns an error, the retry decides the result from the contexts, not from the error's cause:

1. If the signal context is done, it returns the signal context's error. D2 turns that into exit code 0.
2. Else, if the parent context ran out, or backoff stopped on its elapsed-time limit, the budget ran out. Backoff can stop on that limit while the parent is still live, because it refuses a wait longer than the time left. It returns `status.Error(codes.Unavailable, ...)`, whose message names the connect-and-create step, `ENGRAM_QDRANT_STARTUP_TIMEOUT` and the last attempt's error. `classifyOperatorErrConstruction` maps it to exit code 5 with no change to the classifier.
3. Else, the error was not transient. Backoff returns it inside a `*backoff.RetryError`, so the retry returns that wrapper's `LastErr`, the create step's own error. Its text and its exit code stay what they are today.

The Qdrant client keeps a pool of three gRPC connections and sends each call to the next one in turn. A successful create attempt proves one connection up, and another one can still be in its reconnect wait. A schema call on that connection fails at once with `Unavailable`. So in the serve path, schema convergence runs under `store.WithWaitForReady(ctx)`. The serve client also gets `store.WaitForReadyInterceptor()`, a unary interceptor that adds `grpc.WaitForReady(true)` to each call made under that marked context. A schema call then waits for a ready connection, within the schema budget, and does not fail on a connection that is reconnecting. The create attempts and every request after startup run under unmarked contexts and fail at once, as before. The CLI client has no interceptor, so the mark has no effect there. `TestServeWaitsForQdrant` found this gap: without the mark, startup failed in the schema step right after Qdrant came up.

The transient test moves from `cmd/engram/operror.go` to an exported `store.IsUnavailable`. Both the CLI classifier and the retry call it, so the two cannot disagree about what "unavailable" means. The gRPC `DeadlineExceeded` rule stays local to the retry, so CLI classification does not change.

Alternatives rejected:

- Retrying all of `server.Register`. `Register` returns on a provisioning error before it mounts anything, so a retry is possible. But it also repeats the config load and the construction of the embedder, the decision provider and the queues for every attempt. The narrow retry repeats only the step that waits on Qdrant.
- The retry configuration of the Qdrant Go client. It applies to every RPC for the life of the client. It bounds retries by count, not by elapsed time. It gives no hook for a log line per attempt.
- gRPC `WaitForReady`. It makes one RPC wait for the server until its deadline, so there are no attempts to log, and the per-attempt budget no longer applies.
- `SkipCompatibilityCheck` in the serve path. It also bounds construction, but it drops the version warning that operators get today.
- `ClientConn.ResetConnectBackoff` before each attempt. It reaches into each pooled connection of the client. The connect parameters do the same job once, at construction.

### D4. Readiness from a background health loop

After the gate targets are set, `runServe` starts a goroutine. It sends one initial ping at once and then pings on every tick of a 5 s ticker, each ping with a 2 s timeout. It stores the result in an `atomic.Bool` that starts false. `/readyz` reads that value and the gate target. A failed ping logs at warn level when the last result was a success or when it is the initial ping. A later failure in a row does not log again, so an outage that exists at the first ping is logged once. When the signal context ends, the goroutine stops.

`Store` gains a `HealthCheck(ctx) error` method over its existing client. Unlike its sibling methods, it starts no span of its own and records no store-op metric. A record every 5 s per replica adds no information beyond the otelgrpc client span that each ping already gets. The loop takes the ping function and a tick channel as inputs, so tests drive it with no timers. Once started, the loop has sent one ping more than the number of ticks it received.

The interval and timeout are fixed constants, not configuration keys. Nothing yet needs them tuned, and the readiness probe's own `periodSeconds` already controls how fast Kubernetes reacts.

### D5. Config and chart

| Setting | Env key | Default | Validation | Chart value |
| --- | --- | --- | --- | --- |
| Startup retry budget | `ENGRAM_QDRANT_STARTUP_TIMEOUT` | `60s` | positive Go duration | `memory.qdrant.startupTimeoutSeconds: 60` |

The key goes in `internal/config/registry.go` (key `qdrant.startup_timeout`) and `config.go`. It is validated in `Config.Validate`, as its serve-only sibling `ENGRAM_QDRANT_SCHEMA_TIMEOUT` is today, so a bad value fails every verb the same way. In `validate.go`, a new exported `ParsePositiveDuration(value string) (time.Duration, error)` sits next to the existing cap parser, and `validatePositiveDuration(value, envName string) error` calls it. Its error text names the variable and keeps the `e.g. 30s, 2m` hint. `internal/server` parses the schema and startup budgets with `config.ParsePositiveDuration`, so validation and runtime share one parser. This change uses the helpers for the two Qdrant timeouts only. Moving the other positive-duration keys to them is follow-up work in its own issue, because this change has no requirement that touches them.

In the chart, a `engram.positiveSeconds` helper in `_helpers.tpl` takes the value path and the value. It fails the render on a value that is not a number (`kindIs` of `int64` or `float64`, so `true` fails), not a whole number (`float64 $v` differs from `float64 (int $v)`), not positive, or above 9223372036, the largest whole-second Go duration. Today's check coerces with `int`, so `1.9` renders as `1s` and `0.5` fails with a message that calls it not positive. It replaces the existing `schemaTimeoutSeconds` check and also covers `startupTimeoutSeconds`. The chart has no `values.schema.json` today, and adding one for two values is a larger change than this one needs. `engram.containerEnv` renders `ENGRAM_QDRANT_STARTUP_TIMEOUT` as `"<n>s"`, so the `containerEnv` checksum in `chart:validate` changes with it.

Probes in `charts/engram/templates/memory-mcp.yaml`:

- `startupProbe`: `httpGet /livez`, period 5 s, failure threshold 6. It covers process start up to the bind only. The bind now comes before every network call, and `/livez` answers during provisioning.
- `livenessProbe`: `httpGet /livez`, with today's timing.
- `readinessProbe`: `httpGet /readyz`, with today's timing.

In `chart:validate`, the 615 s startup-window check goes. Assertions for the three `httpGet` probes and the `startupTimeoutSeconds` cases take its place. The existing `schemaTimeoutSeconds: 600` assertion stays.

### D6. Tests

Fakes for the create step return the error shapes that the Qdrant client returns. A cut-off call gives a gRPC status, not a bare context error.

| Requirement or scenario | Test kind | Test and command |
| --- | --- | --- |
| Startup timeout setting | scenario | New cases in `TestValidateFieldRules` and the defaults test. Unset gives `60s`. `1ns` and `1s` are accepted. `0`, `-1s` and `abc` are rejected with the variable named. Validation runs with no store, so nothing dials Qdrant. `go test ./internal/config` |
| Startup timeout setting (all durations) | invariant | `TestPropertyValidatePositiveDuration`, a rapid property. For any `int64` d, `validatePositiveDuration(time.Duration(d).String(), ...)` fails exactly when d is not above zero, and the error names the variable. `go test ./internal/config -run TestPropertyValidatePositiveDuration` |
| Invalid schema timeout | scenario | The existing schema-timeout cases in `TestValidateFieldRules` keep passing. New `TestValidatePositiveDurationText` pins the full error text of both Qdrant timeouts for a zero and a malformed value. `go test ./internal/config` |
| Connect-and-create retries | scenario | `TestServeProvisionRetry` with a fake create step and a captured slog handler. Cases: N `Unavailable` statuses then success, with N numbered warn entries. A gRPC `DeadlineExceeded` status while the parent is live, followed by a second attempt that runs. `PermissionDenied`: one attempt, `errors.Is` finds the create error, and the error text and exit code are today's. `go test ./internal/server -run TestServeProvisionRetry` |
| Budget runs out | scenario | `TestServeProvisionRetry` cases, each giving exit code 5 from `classifyOperatorErrConstruction` and an error that names the connect-and-create step and `ENGRAM_QDRANT_STARTUP_TIMEOUT`. A 50 ms budget with a create step that blocks and then returns a gRPC `DeadlineExceeded` status. A 50 ms budget with a constant 40 ms backoff and a create step that fails at once, where backoff stops on its elapsed-time limit while the parent is still live. |
| Signal during the retry | scenario | `TestServeProvisionRetry` cases that cancel the signal context during an attempt and during a non-zero backoff wait. The attempt count stops at the cancelled attempt, `errors.Is(err, context.Canceled)` holds, and the error is not an `Unavailable` status. |
| Schema convergence not cut by the startup timeout | scenario | A `TestServeProvisionRetry` case through `ensureStoreFromConfig` with a 100 ms startup timeout, a 10 s schema budget and a schema step that takes 300 ms. It succeeds. |
| Each failed attempt logged (all sequences) | invariant | `TestPropertyServeProvisionRetryLog`, a rapid property. It draws N transient errors and a terminal outcome: success, a permanent error, exhaustion, or a signal during attempt k. The warn entries are numbered 1 to the number of failed attempts. Per outcome, a signal gives `context.Canceled`, exhaustion gives an `Unavailable` status naming `ENGRAM_QDRANT_STARTUP_TIMEOUT`, and a permanent error satisfies `errors.Is` with the create error. `go test ./internal/server -run TestPropertyServeProvisionRetryLog` |
| CLI verb against an unreachable Qdrant | scenario | `TestStoreFromEnvSingleAttempt`: `StoreFromEnv` against a closed port makes one attempt. The existing unreachable-Qdrant rows of `exitCodeBaseline` already pin exit code 5 for the `StoreFromEnv` verbs. `go test ./internal/server -run TestStoreFromEnvSingleAttempt` |
| Slow index build, schema budget exceeded, quantization rejected | scenario | The existing `TestProvisionStoreBudgets` subtests keep passing with the new `provisionStore` signature. `go test ./internal/server -run TestProvisionStoreBudgets` |
| Shutdown signal during startup | scenario | `TestStartServeSignal` drives `startServe` with a fake register step and cancels during an attempt, the backoff wait and schema convergence. Each time, `startServe` returns the context error and the listeners are closed. `TestStartupResult` shows that `startupResult` gives nil for a done signal context and the error unchanged otherwise. `TestServeSignalDuringStartup` in `internal/e2e` sends SIGTERM to the built binary while it waits for a refusing Qdrant, and expects exit code 0 well before the startup timeout. `go test ./cmd/engram -run 'TestStartServeSignal\|TestStartupResult'` and `task test:e2e` |
| Quantization rejected by Qdrant (gate) | scenario | `TestStartServeProvisioningError`: a register step that fails naming the quantization step leaves every gate target unset and returns the error unchanged. `go test ./cmd/engram -run TestStartServeProvisioningError` |
| Server serves requests only after provisioning (gate) | scenario | `TestStartupGate`. Before the target is set, an MCP `tools/list` POST, a Connect call and a console GET each get 503 with `Retry-After`, and the target is not called. After, requests reach the target. `/livez` answers 200 in every state, including a failed ping. A cross-site POST gets 403 both before and after the target is set. `TestServeBothListeners` changes as D2 says and keeps its assertions. A `resolveMCPPath` case rejects `/livez` and `/readyz`. `go test ./cmd/engram -run 'TestStartupGate\|TestServeBothListeners\|TestResolveMCPPath'` |
| No handler before provisioning (all requests) | invariant | `TestPropertyStartupGate`, a rapid property. It draws a method, a path, a `Sec-Fetch-Site` value and an `Origin`. Before the target is set, the target sees no call. A request the cross-origin check refuses gets 403. Every other request except `GET /livez` and `GET /readyz` gets 503 with `Retry-After`. `go test ./cmd/engram -run TestPropertyStartupGate` |
| Port accepts connections, requests during and after provisioning, Qdrant up late | scenario | `TestServeWaitsForQdrant` in `internal/e2e`, against the built binary. It starts serve behind a TCP proxy that refuses connections and expects a TCP connect, `/livez` 200, `/readyz` 503 and an MCP `tools/list` 503. After several failed attempts it opens the proxy to the test Qdrant, and it expects `/readyz` 200 within 10 s and an authenticated `tools/list` with HTTP 200 and the tool list. Polls have a deadline. `task test:e2e` |
| Budget runs out (binary) | scenario | `TestServeStartupTimeout` in `internal/e2e` runs with `ENGRAM_QDRANT_STARTUP_TIMEOUT=2s` against a closed port and against a listener that accepts and never answers. Each time it expects exit code 5 within 15 s and a log line naming the connect-and-create step and `ENGRAM_QDRANT_STARTUP_TIMEOUT`. `task test:e2e` |
| Every existing e2e test | scenario | `startServer` splits into a launch step and a readiness wait. The wait polls `GET /readyz` on the primary listener for 200, then sends a request to the MCP-only listener that must not get the gate's 503. `TestServeWaitsForQdrant` and `TestServeSignalDuringStartup` use the launch step alone. `task test:e2e` |
| Longest backoff wait | invariant | `TestPropertyServeBackOffBound`, a rapid property over the serve policy's `newServeBackOff()`. Every `NextBackOff` value it draws is at most 7.5 s. `go test ./internal/server -run TestPropertyServeBackOffBound` |
| Not ready before the first background check (edge) | scenario | A `TestPropertyReadiness` scenario case sets the target while the fake initial ping blocks on a channel. `/readyz` answers 503 until the ping is released, then 200. |
| Second listener during provisioning | scenario | A `TestStartupGate` case: a `/ui/` GET on the MCP-only gate before its target is set gets 503 with `Retry-After` and no console content. |
| Every gate set before readiness | scenario | `TestMarkProvisioned`: the `startHealth` hook, when it runs, finds the target of every gate set, the MCP-only gate included. `go test ./cmd/engram -run TestMarkProvisioned` |
| Reconnect configuration | scenario | `TestStartupConnectParams`: the parameters have a `MaxDelay` of 5 s, and a non-zero `BaseDelay`, `Multiplier` and `MinConnectTimeout`. `go test ./internal/store -run TestStartupConnectParams` |
| Qdrant comes up late in the budget | scenario | The retry takes its `BackOff` as a parameter, so tests inject one. A `TestServeProvisionRetry` case injects a `BackOff` whose waits fit inside the budget, and a create step that starts to succeed on the last attempt that the budget allows. It expects success. No test waits for a real 7.5 s interval. |
| Readiness route, readiness never calls Qdrant | invariant | `TestPropertyReadiness`, a rapid state machine. Actions: start the loop, tick with ping ok, tick with ping failing, and `GET /readyz`. The model: before the initial ping finishes, not ready. After it, ready exactly while the target is set and the last finished ping succeeded. Invariants: `/readyz` matches the model. Once the loop has started, the ping count equals the tick count plus one, whatever the number of `/readyz` calls. The warn count equals the number of ok-to-fail changes, plus one if the initial ping failed. A scenario case starts the loop, sends no tick, and expects `/readyz` 200 after the initial ping succeeds. `go test ./cmd/engram -run TestPropertyReadiness` |
| Chart probes and env | scenario | `task chart:validate`. The default render has three `httpGet` probes and `ENGRAM_QDRANT_STARTUP_TIMEOUT=60s`. For both `startupTimeoutSeconds` and `schemaTimeoutSeconds`: `1` renders `1s` and `9223372036` renders, and `0`, `-1`, `0.5`, `1.9`, `true` and `9223372037` fail naming the value. `startupTimeoutSeconds: 180` renders `180s`, and `schemaTimeoutSeconds: 600` still renders `600s`. |

No test asserts backoff's own timing, gRPC's reconnect behavior or the Qdrant client's version check.

## Risks / Trade-offs

- [An external TCP check now passes during provisioning] → The proposal and the removed requirement mark this as breaking. deploy.md says to probe `/readyz`, and the chart switches in the same release.
- [A request during provisioning gets 503 instead of a refused connection] → `Retry-After` gives clients a retry delay. MCP clients already retry a failed connect.
- [Readiness lags a Qdrant outage by up to 5 s] → This is acceptable for a readiness signal. Requests during the lag fail as they do today.
- [The health loop adds one Qdrant RPC every 5 s per replica] → `HealthCheck` is the cheapest Qdrant RPC, and it records no store-op metric. There is no per-probe cost.
- [A slow OIDC issuer now delays readiness, not the bind] → The pod stays live and not ready while discovery runs. A discovery failure still fails startup with today's error.

## Migration Plan

The chart and the binary ship in the same release, and the chart's probes match the new routes. An operator who runs the binary outside the chart and uses a TCP readiness check must switch to `GET /readyz`. deploy.md says so. The implementing commit carries a `BREAKING CHANGE:` footer, so release-please puts the migration in the release notes. To roll back, deploy the previous chart and image together. An older binary ignores the new env key.

## Open Questions

None.
