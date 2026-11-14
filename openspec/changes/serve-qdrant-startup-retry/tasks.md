# Tasks

## 1. Startup timeout configuration key

- [x] 1.1 In `internal/config/validate.go`, add an exported `ParsePositiveDuration` and a `validatePositiveDuration(value, envName, example string) error` that calls it, next to `validatePositiveCap`. Use them for `ENGRAM_QDRANT_SCHEMA_TIMEOUT`. Add `TestValidatePositiveDurationText`, which pins the full error text for a zero and a malformed value. Make sure that `go test ./internal/config` passes, including the existing schema-timeout cases in `TestValidateFieldRules`.
- [x] 1.2 Add the startup timeout configuration key `qdrant.startup_timeout` (`ENGRAM_QDRANT_STARTUP_TIMEOUT`, default `60s`) to `registry.go` and `config.go`. Validate it with `validatePositiveDuration` in `Config.Validate`. Add cases to `TestValidateFieldRules`, the defaults test and `TestValidatePositiveDurationText`: unset gives `60s`, `1ns` and `1s` pass, and `0`, `-1s` and `abc` fail naming the variable. Make sure that `go test ./internal/config` passes.
- [x] 1.3 Add `TestPropertyValidatePositiveDuration`, a rapid property over any `int64` duration. Make sure that `go test ./internal/config -run TestPropertyValidatePositiveDuration` passes.
- [x] 1.4 Open a GitHub issue to move the other inline positive-duration checks in `Config.Validate`, and the runtime positive-duration parsers in `internal/server`, to the new helpers. List each key in the issue, and name the issue in the PR. Opened as #1019.
- [x] 1.5 Document `ENGRAM_QDRANT_STARTUP_TIMEOUT` in docs-site `guides/configure.md`. In the `ENGRAM_QDRANT_SCHEMA_TIMEOUT` row, remove "The server opens its port only after both finish". Also remove the claim that the chart derives its startup probe from it. Make sure that `task lint:markdown` passes.

## 2. Connect-and-create retry while Qdrant is unavailable

- [x] 2.1 Move the unavailable-error test from `cmd/engram/operror.go` to an exported `store.IsUnavailable`, and point `classifyOperatorErr` at it. Make sure that `go test ./cmd/engram` passes unchanged.
- [x] 2.2 Add a version-probe timeout parameter to `store.NewQdrantClient`, where zero keeps the client default. Add `store.StartupConnectParams()` (design D3) and `TestStartupConnectParams`. Give `storeFromConfig` that timeout and a dial-option list to pass through. `StoreAndEmbedderFromEnvNoEnsure` and the test callers pass the zero value. `NewQdrantClient` stays the only caller of `qdrant.NewClient`. Make sure that `go test ./internal/store` passes, including the client-construction convergence test.
- [x] 2.3 Give `newRetryBackOff` in `internal/server/summaryqueue.go` initial-interval and max-interval arguments, and pass the summary queue's current values. Make sure that `go test ./internal/server -run SummaryQueue` passes unchanged.
- [x] 2.4 Give `provisionStore` a `ctx` and a create-step parameter (design D3). Schema convergence runs under a context derived from `ctx`. Make sure that the existing `TestProvisionStoreBudgets` subtests pass with `go test ./internal/server -run TestProvisionStoreBudgets`.
- [x] 2.5 Give `ensureStoreFromConfig` a `ctx` and a `startupPolicy` (design D3), and parse both Qdrant budgets with `config.ParsePositiveDuration`. `StoreFromEnv` and the CLI constructors pass `context.Background()` and the zero policy. `buildDepsFromEnv` passes the signal context and the serve policy, and `ensureStoreFromConfig` passes the policy's version-probe timeout and connect parameters to `storeFromConfig`. Make sure that `go test ./internal/server` passes.
- [x] 2.6 Implement the serve policy's connect-and-create retry of design D3. The startup parent context starts before the client is built and bounds only construction and the attempts. The client gets the bounded version probe, and `grpc.WithConnectParams` with a 5 s `MaxDelay` in its dial options. Each attempt gets 15 s, capped by the parent deadline. The serve backoff comes from `newServeBackOff()`, and its longest wait is 7.5 s. Add `TestPropertyServeBackOffBound`. The closure logs each failed attempt at warn level with its number. The result follows the three rules of D3, and rule 3 returns `LastErr` from the `*backoff.RetryError`.
- [x] 2.7 Add `TestServeProvisionRetry` with a fake create step that returns gRPC status errors and a captured slog handler. Cover every case in the design D6 rows for retries, the spent budget, the signal during the retry and schema convergence. Make sure that `go test ./internal/server -run TestServeProvisionRetry` passes.
- [x] 2.8 Add `TestPropertyServeProvisionRetryLog`, a rapid property over N transient errors and four terminal outcomes, a signal during attempt k included. It also asserts the error class of each outcome (design D6). Make sure that `go test ./internal/server -run TestPropertyServeProvisionRetryLog` passes.
- [x] 2.9 Add `TestStoreFromEnvSingleAttempt`: `StoreFromEnv` against a closed port makes one attempt. The existing `exitCodeBaseline` rows keep pinning exit code 5. Make sure that `go test ./internal/server -run TestStoreFromEnvSingleAttempt` passes.

## 3. Early bind and the startup gate

- [x] 3.1 Add the startup gate of design D1, inside the `newCrossOriginProtection` wrapper that `newHTTPServer` builds. It serves requests only after provisioning completes. Before that, no MCP, Connect or console handler runs, and every admitted request except `/livez` and `/readyz` gets 503 and `Retry-After`. On the primary listener, it serves the liveness route `/livez` and the readiness route `/readyz`. Add `/livez` and `/readyz` to `reservedMCPRoots`.
- [x] 3.2 Add `TestStartupGate` with the cases in design D6, cross-site POSTs before and after the target is set included. Add a `resolveMCPPath` case that rejects `/livez` and `/readyz`. Make sure that `go test ./cmd/engram -run 'TestStartupGate|TestResolveMCPPath'` passes.
- [x] 3.3 Add `TestPropertyStartupGate`, a rapid property over the method, the path, `Sec-Fetch-Site` and `Origin`. Make sure that `go test ./cmd/engram -run TestPropertyStartupGate` passes.
- [x] 3.4 Add a leading `ctx` parameter to `server.Register` and pass it to `buildDepsFromEnv`, and update the test callers. Make sure that `go test ./internal/server` passes.
- [x] 3.5 Change `newListeners` to build listeners with gates only (design D2). Split `serveAll` into start and wait-and-shutdown steps. Move design D2 steps 3 and 4 into `startServe`, which takes the register step as a parameter. Add `startupResult` and `markProvisioned`, and reorder `runServe` to bind before `startServe`. Add `TestMarkProvisioned`. On a startup error, skip the wait and call the same shutdown function directly, with a no-op drain. Update `TestServeBothListeners` to set both gate targets and use the split steps, with its assertions unchanged. Make sure that `go test ./cmd/engram -run TestServeBothListeners` passes.
- [x] 3.6 Add `TestStartServeSignal`, `TestStartupResult` and `TestStartServeProvisioningError` with the cases in design D6. Make sure that `go test ./cmd/engram -run 'TestStartServeSignal|TestStartupResult|TestStartServeProvisioningError'` passes.
- [x] 3.7 Split `startServer` in `internal/e2e/harness_test.go` into a launch step and a readiness wait. The wait polls `GET /readyz` on the primary listener for 200, then sends a request to the MCP-only listener that must not get the gate's 503. Make sure that `task test:e2e` passes for every existing test.
- [x] 3.8 Add `TestServeWaitsForQdrant`, `TestServeStartupTimeout` and `TestServeSignalDuringStartup` to `internal/e2e` with the cases in design D6. The first and the last use the launch step alone. Make sure that `task test:e2e` passes.

## 4. Readiness from the background health loop

- [x] 4.1 Add `Store.HealthCheck(ctx) error` with no span of its own and no store-op metric (design D4). Add the health loop, driven by a tick channel and a ping function. It sends one initial ping and then one per tick, and its ping result starts false. If a ping fails and the last result was a success, or if the initial ping fails, it logs at warn level.
- [x] 4.2 Add `TestPropertyReadiness`, a rapid state machine with the model and the three invariants of design D6: readiness, the ping count and the warn count. The ping-count invariant shows that readiness never calls Qdrant per request. Add a case with no tick that expects `/readyz` 200 after the initial ping. Add a case that holds the initial ping open and expects 503 until it finishes. Make sure that `go test ./cmd/engram -run TestPropertyReadiness` passes.
- [x] 4.3 After the gate targets are set, start the health loop in `runServe`. It uses a 5 s ticker and a 2 s ping timeout. Make sure that `TestServeWaitsForQdrant` in 3.8 sees `/readyz` turn 200.

## 5. Helm chart

- [x] 5.1 Add an `engram.positiveSeconds` helper to `charts/engram/templates/_helpers.tpl` and use it for `schemaTimeoutSeconds` and the new `memory.qdrant.startupTimeoutSeconds: 60`. Render `ENGRAM_QDRANT_STARTUP_TIMEOUT` as `"<n>s"` in `engram.containerEnv`, and update the `containerEnv` checksum in `chart:validate`. In `values.yaml`, remove the comment that says the startup probe is sized from `schemaTimeoutSeconds`.
- [x] 5.2 Change the probes in `memory-mcp.yaml` as in design D5: startup and liveness probes `httpGet /livez`, the readiness probe `httpGet /readyz`, and a startup probe of 6 periods of 5 s.
- [x] 5.3 In the `chart:validate` task, remove the 615 s startup-window check. Add the probe and timeout assertions of the design D6 chart row, the `1`, `9223372036`, `0`, `-1`, `0.5`, `1.9`, `true` and `9223372037` cases for both timeouts included. The helper rejects a value that is not a whole number, a boolean, and a value above 9223372036. Keep the `schemaTimeoutSeconds: 600` assertion. Make sure that `task chart:validate` passes.

## 6. Deploy and upgrade documentation

- [x] 6.1 Update docs-site `guides/deploy.md`: add `memory.qdrant.startupTimeoutSeconds` to the values table, and remove "also sizes the startup probe" from the `schemaTimeoutSeconds` row. Rewrite the startup-probe paragraph for the startup retry, the two health routes and the new probes. Say that the serve client keeps a 5 s gRPC reconnect cap after startup. Tell operators to probe `/readyz`, not the TCP port. Make sure that `task lint:markdown` passes.
- [x] 6.2 Add a numbered entry under Unreleased in docs-site `guides/upgrade.md` for the early bind, the health routes, the probes and `ENGRAM_QDRANT_STARTUP_TIMEOUT`. Give it a list of the operators who need to act, the ones with TCP readiness checks. In entry 23, change the sentences about the port and the startup probe to point to the new entry. Make sure that `task lint:markdown` passes.

## 7. Integration check

- [x] 7.1 Run `task` (lint and test), `task test:e2e`, `task chart:validate` and `openspec validate serve-qdrant-startup-retry --strict`. Make sure that all four pass.
- [x] 7.2 Commit the behavior change with a `BREAKING CHANGE:` footer that tells operators to switch a TCP readiness check to `GET /readyz`. Make sure that `git log -1` shows the footer.

## Workflow follow-up

- Open the PR with `Closes #1013`, then archive the change after the PR merges.
