---
verdict: approve-with-notes
phase: implementation
reviewed_at: 2026-11-14T20:50:51+00:00
head: 1df8c7f56ecdb3b8179eae1fae1d13c408619a8d
lenses:
  spec-lint: not-applicable
  approach: ok
  fit: ok
  test-coverage: ok
  test-honesty: ok
  invariants: ok
---
# Review

## Findings

| id | lens | severity | claim | evidence | action |
|---|---|---|---|---|---|
| app-1 | approach | should-fix | D2 moves the signal context to the top of runServe, but at head it is created after config.Load, telemetry.Setup and the MCP path checks, so a signal in that window gets Go's default handling, not exit code 0. | design.md > D2. Bind first, then start up behind the gate — "The signal context moves to the top of `runServe`." | Move signal.NotifyContext above config.Load in runServe, or record in D2 that it sits directly before the bind. |
| app-2 | approach | should-fix | D3 first names store.StartupConnectParams with no parameter; at head it takes maxDelay. | design.md > D3. Retry in the serve provisioning path only — "built by `store.StartupConnectParams()`" | Write store.StartupConnectParams(maxDelay) in that D3 bullet, as the later bullet already does. |
| app-3 | approach | should-fix | D2 gives newListeners the signature newListeners(cfg); at head it also takes the health loop and the shutdown hook. | design.md > D2. Bind first, then start up behind the gate — "`newListeners(cfg)` no longer takes the mux or the MCP handler" | Name newListeners(cfg, health, onShutdown) in D2 and say the primary gate takes the health loop at bind time. |
| app-4 | approach | should-fix | The engram.positiveSeconds helper accepts kind int as well and coerces with int64, where D5 names kindIs of int64 or float64 and a float64 (int $v) comparison; design.md records no deviation. | design.md > D5. Config and chart — "not a whole number (`float64 $v` differs from `float64 (int $v)`)" | Update D5 to the int, int64 and float64 kinds and the int64 coercion the helper uses. |
| app-5 | approach | should-fix | Task 1.1 names validatePositiveDuration with an example parameter; at head it takes value and envName only, as D5 says. | tasks.md > 1. Startup timeout configuration key — "a `validatePositiveDuration(value, envName, example string) error` that calls it" | Change task 1.1 to validatePositiveDuration(value, envName string) error. |
| app-6 | approach | should-fix | Task 2.2 names store.StartupConnectParams with no parameter; at head it takes maxDelay. | tasks.md > 2. Connect-and-create retry while Qdrant is unavailable — "Add `store.StartupConnectParams()` (design D3) and `TestStartupConnectParams`." | Write store.StartupConnectParams(maxDelay) in task 2.2. |
| fit-1 | fit | should-fix | The serve retry's attempt closure re-implements the create-and-wrap body of singleCreate, the one-attempt create step the change adds in the same file, so the 'create collection (budget %s)' text that D3 promises to keep identical lives in two places. | internal/server/tools.go:313 — "if err := st.CreateCollectionIfAbsent(actx, dim); err != nil {" | Make the serve attempt call singleCreate(policy.attemptBudget())(actx, st, dim); its inner timeout is a no-op under the attempt context, and the error text then has one source. |
| fit-2 | fit | should-fix | The new retry tests hand-write equalInts, which is the standard library's slices.Equal; the same package's tests already use slices.Equal throughout. | internal/server/startup_test.go:95 — "func equalInts(a, b []int) bool {" | Delete equalInts and call slices.Equal at its call sites in startup_test.go. |
| fit-3 | fit | should-fix | The change adds two hand-rolled e2e poll loops with the same shape (exit check on done, condition, deadline fatal, 100 ms sleep): serverProc.waitReady in the harness and waitForLog in startup_test.go. | internal/e2e/harness_test.go:347 — "deadline := time.Now().Add(within)" | Add one serverProc poll helper (within, a description, a condition func) to harness_test.go that fails on process exit and on the deadline, and build waitReady's /readyz loop and waitForLog on it. |
| ctl-not-run | test-coverage | should-fix | covered row 'Live during provisioning' has no verification run | review > not-run — "cmd/engram/startupgate_test.go: TestStartupGate/primary gate before and after provisioning (GET /livez 200)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Live while Qdrant is down after start' has no verification run | review > not-run — "cmd/engram/startupgate_test.go: TestStartupGate (livez 200 with failed ping)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Not ready during provisioning' has no verification run | review > not-run — "cmd/engram/startupgate_test.go: TestStartupGate; internal/e2e/startup_test.go: TestServeWaitsForQdrant" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Not ready before the first background check' has no verification run | review > not-run — "cmd/engram/startupgate_test.go: TestReadinessInitialPing/not ready while the initial ping is open" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Ready after provisioning' has no verification run | review > not-run — "cmd/engram/startupgate_test.go: TestReadinessInitialPing/ready after the initial ping; TestPropertyReadiness" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Not ready after a failed health check' has no verification run | review > not-run — "cmd/engram/startupgate_test.go: TestStartupGate (failed ping 503); TestPropertyReadiness" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Ready again after Qdrant recovers' has no verification run | review > not-run — "cmd/engram/startupgate_test.go: TestPropertyReadiness (tick-fail then tick-ok)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Many probes, no Qdrant traffic' has no verification run | review > not-run — "cmd/engram/startupgate_test.go: TestPropertyReadiness (pings == ticks+1)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Default render' has no verification run | review > not-run — "Taskfile.yaml: chart:validate (probe loop on /livez,/readyz port http)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Console path is refused' has no verification run | review > not-run — "cmd/engram/mcproute_test.go: TestMCPOnlyMux (existing)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Login path is refused' has no verification run | review > not-run — "cmd/engram/mcproute_test.go: TestMCPOnlyMux (existing)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Connect route is refused' has no verification run | review > not-run — "cmd/engram/mcproute_test.go: TestMCPOnlyMux (existing)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Bare root is refused' has no verification run | review > not-run — "cmd/engram/mcproute_test.go: TestMCPOnlyMux (existing)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Root MCP path has no catch-all on the second listener' has no verification run | review > not-run — "cmd/engram/mcproute_test.go: TestMCPOnlyMux (existing)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Console path during provisioning' has no verification run | review > not-run — "cmd/engram/startupgate_test.go: TestStartupGate/MCP-only gate before provisioning" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Attempt runs out of its own budget' has no verification run | review > not-run — "internal/server/startup_test.go: TestServeProvisionRetry/an attempt cut off by its own budget is retried" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Budget runs out' has no verification run | review > not-run — "internal/e2e/startup_test.go: TestServeStartupTimeout; TestServeProvisionRetry/a blocked attempt..." | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'CLI verb against an unreachable Qdrant' has no verification run | review > not-run — "internal/server/startup_test.go: TestStoreFromEnvSingleAttempt; cmd/engram/operror_test.go: grpc codes.Unavailable" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Three failures then success' has no verification run | review > not-run — "internal/server/startup_test.go: TestServeProvisionRetry/unavailable then success logs each failure" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Exhaustion' has no verification run | review > not-run — "internal/server/startup_test.go: TestPropertyServeProvisionRetryLog (exhausted outcome)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Unset' has no verification run | review > not-run — "internal/config/config_test.go: defaults test (StartupTimeout 60s)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Invalid startup timeout' has no verification run | review > not-run — "internal/config/validate_test.go: TestValidateFieldRules; internal/config/duration_test.go" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Request during provisioning' has no verification run | review > not-run — "internal/e2e/startup_test.go: TestServeWaitsForQdrant; cmd/engram/startupgate_test.go: TestStartupGate" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Cross-origin request during provisioning' has no verification run | review > not-run — "cmd/engram/startupgate_test.go: TestStartupGate; TestPropertyStartupGate" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Request after provisioning' has no verification run | review > not-run — "internal/e2e/startup_test.go: TestServeWaitsForQdrant (checkDocumentedTools)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Quantization rejected by Qdrant' has no verification run | review > not-run — "internal/server/provision_test.go: TestProvisionStoreBudgets/a rejected quantization update; cmd/engram/startupgate_test" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Signal during an attempt' has no verification run | review > not-run — "internal/server/startup_test.go: TestServeProvisionRetry/a signal during an attempt; cmd/engram/startupgate_test.go: Tes" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Signal during schema convergence' has no verification run | review > not-run — "internal/server/provision_test.go: TestProvisionStoreBudgets/a signal during an index build" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Signal during the backoff wait' has no verification run | review > not-run — "internal/server/startup_test.go: TestServeProvisionRetry/a signal during the backoff wait" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Slow index build within the schema budget' has no verification run | review > not-run — "internal/server/provision_test.go: TestProvisionStoreBudgets/a slow index build" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Schema budget exceeded' has no verification run | review > not-run — "internal/server/provision_test.go: TestProvisionStoreBudgets/exceeding the schema budget" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Invalid schema timeout' has no verification run | review > not-run — "internal/config/duration_test.go and validate_test.go: TestValidateFieldRules schema-timeout cases" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Startup timeout shorter than one attempt' has no verification run | review > not-run — "internal/e2e/startup_test.go: TestServeStartupTimeout/silent listener" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Raised schema timeout' has no verification run | review > not-run — "Taskfile.yaml: chart:validate (SCHEMA_TIMEOUT 600s)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Raised startup timeout' has no verification run | review > not-run — "Taskfile.yaml: chart:validate (STARTUP_TIMEOUT 180s)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Default render' has no verification run | review > not-run — "Taskfile.yaml: chart:validate (QUANTIZATION int8, STARTUP_TIMEOUT 60s)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Invalid startup timeout value' has no verification run | review > not-run — "Taskfile.yaml: chart:validate (BAD loop names memory.qdrant.startupTimeoutSeconds)" | run the covering test and record it in verification-runs.txt |
| tes-1 | test-coverage | should-fix | Non-transient error scenario asserts the error is unchanged and one attempt, not the exit code the THEN step names. | internal/server/startup_test.go:169 — "if !errors.Is(err, createErr) \|\| err.Error() != createErr.Error() {" | Add a case to cmd/engram/operror_test.go or an e2e run asserting the exit code for a PermissionDenied create error through the serve path. |
| hon-1 | test-honesty | should-fix | TestPropertyStartupGate computes its 403 expectation with the same cross-origin helper the gate wraps, so the property cannot catch a wrong refusal rule. | cmd/engram/startupgate_test.go:129 — "refused := newCrossOriginProtection().Check(r) != nil" | Derive refusal from an independent oracle (Sec-Fetch-Site cross-site/same-site with a POST, or a mismatched Origin) or keep only the literal cases in TestStartupGate. |
| inv-1 | invariants | note | The invariant that no handler runs before provisioning holds for every request path, an unbounded domain, but TestPropertyStartupGate draws the path from a fixed set of nine values, so paths such as /livez/, //readyz or /readyzx are never generated. | specs/collection-provisioning/spec.md > Requirement: Server serves requests only after provisioning completes — "Until all three complete, every request that passes the cross-origin check, except `GET /livez` and `GET /readyz`, SHALL" | In TestPropertyStartupGate (pgregory.net/rapid, as in cmd/engram/startupgate_test.go), draw the path with rapid.OneOf over the sampled set and rapid.StringMatching(`/[a-z./]{0,16}`), and assert a counting target sees no call, so near-miss paths of the two health routes are generated. |

## Coverage matrix

| scenario | covered by | status | verified |
|---|---|---|---|
| Live during provisioning | cmd/engram/startupgate_test.go: TestStartupGate/primary gate before and after provisioning (GET /livez 200) | covered | not-run |
| Live while Qdrant is down after start | cmd/engram/startupgate_test.go: TestStartupGate (livez 200 with failed ping) | covered | not-run |
| Not ready during provisioning | cmd/engram/startupgate_test.go: TestStartupGate; internal/e2e/startup_test.go: TestServeWaitsForQdrant | covered | not-run |
| Not ready before the first background check | cmd/engram/startupgate_test.go: TestReadinessInitialPing/not ready while the initial ping is open | covered | not-run |
| Ready after provisioning | cmd/engram/startupgate_test.go: TestReadinessInitialPing/ready after the initial ping; TestPropertyReadiness | covered | not-run |
| Not ready after a failed health check | cmd/engram/startupgate_test.go: TestStartupGate (failed ping 503); TestPropertyReadiness | covered | not-run |
| Ready again after Qdrant recovers | cmd/engram/startupgate_test.go: TestPropertyReadiness (tick-fail then tick-ok) | covered | not-run |
| Many probes, no Qdrant traffic | cmd/engram/startupgate_test.go: TestPropertyReadiness (pings == ticks+1) | covered | not-run |
| Default render | Taskfile.yaml: chart:validate (probe loop on /livez,/readyz port http) | covered | not-run |
| Console path is refused | cmd/engram/mcproute_test.go: TestMCPOnlyMux (existing) | covered | not-run |
| Login path is refused | cmd/engram/mcproute_test.go: TestMCPOnlyMux (existing) | covered | not-run |
| Connect route is refused | cmd/engram/mcproute_test.go: TestMCPOnlyMux (existing) | covered | not-run |
| Bare root is refused | cmd/engram/mcproute_test.go: TestMCPOnlyMux (existing) | covered | not-run |
| Root MCP path has no catch-all on the second listener | cmd/engram/mcproute_test.go: TestMCPOnlyMux (existing) | covered | not-run |
| Console path during provisioning | cmd/engram/startupgate_test.go: TestStartupGate/MCP-only gate before provisioning | covered | not-run |
| Qdrant comes up within the budget | internal/e2e/startup_test.go: TestServeWaitsForQdrant | covered | pass |
| Attempt runs out of its own budget | internal/server/startup_test.go: TestServeProvisionRetry/an attempt cut off by its own budget is retried | covered | not-run |
| Budget runs out | internal/e2e/startup_test.go: TestServeStartupTimeout; TestServeProvisionRetry/a blocked attempt... | covered | not-run |
| Error that is not transient | internal/server/startup_test.go: TestServeProvisionRetry/an error that is not transient fails at once | partial | not-run |
| CLI verb against an unreachable Qdrant | internal/server/startup_test.go: TestStoreFromEnvSingleAttempt; cmd/engram/operror_test.go: grpc codes.Unavailable | covered | not-run |
| Three failures then success | internal/server/startup_test.go: TestServeProvisionRetry/unavailable then success logs each failure | covered | not-run |
| Exhaustion | internal/server/startup_test.go: TestPropertyServeProvisionRetryLog (exhausted outcome) | covered | not-run |
| Unset | internal/config/config_test.go: defaults test (StartupTimeout 60s) | covered | not-run |
| Smallest positive value | internal/config/duration_test.go: TestValidateStartupTimeoutAccepted | covered | pass |
| Invalid startup timeout | internal/config/validate_test.go: TestValidateFieldRules; internal/config/duration_test.go | covered | not-run |
| Port accepts connections during provisioning | internal/e2e/startup_test.go: TestServeWaitsForQdrant | covered | pass |
| Request during provisioning | internal/e2e/startup_test.go: TestServeWaitsForQdrant; cmd/engram/startupgate_test.go: TestStartupGate | covered | not-run |
| Cross-origin request during provisioning | cmd/engram/startupgate_test.go: TestStartupGate; TestPropertyStartupGate | covered | not-run |
| Request after provisioning | internal/e2e/startup_test.go: TestServeWaitsForQdrant (checkDocumentedTools) | covered | not-run |
| Quantization rejected by Qdrant | internal/server/provision_test.go: TestProvisionStoreBudgets/a rejected quantization update; cmd/engram/startupgate_test.go: TestStartServeProvisioningError | covered | not-run |
| Signal during an attempt | internal/server/startup_test.go: TestServeProvisionRetry/a signal during an attempt; cmd/engram/startupgate_test.go: TestStartServeSignal | covered | not-run |
| Signal during schema convergence | internal/server/provision_test.go: TestProvisionStoreBudgets/a signal during an index build | covered | not-run |
| Signal during the backoff wait | internal/server/startup_test.go: TestServeProvisionRetry/a signal during the backoff wait | covered | not-run |
| Slow index build within the schema budget | internal/server/provision_test.go: TestProvisionStoreBudgets/a slow index build | covered | not-run |
| Schema budget exceeded | internal/server/provision_test.go: TestProvisionStoreBudgets/exceeding the schema budget | covered | not-run |
| Invalid schema timeout | internal/config/duration_test.go and validate_test.go: TestValidateFieldRules schema-timeout cases | covered | not-run |
| Startup timeout shorter than one attempt | internal/e2e/startup_test.go: TestServeStartupTimeout/silent listener | covered | not-run |
| Raised schema timeout | Taskfile.yaml: chart:validate (SCHEMA_TIMEOUT 600s) | covered | not-run |
| Raised startup timeout | Taskfile.yaml: chart:validate (STARTUP_TIMEOUT 180s) | covered | not-run |
| Default render | Taskfile.yaml: chart:validate (QUANTIZATION int8, STARTUP_TIMEOUT 60s) | covered | not-run |
| Invalid startup timeout value | Taskfile.yaml: chart:validate (BAD loop names memory.qdrant.startupTimeoutSeconds) | covered | not-run |

### Not run

- Live during provisioning: no verification run names this row
- Live while Qdrant is down after start: no verification run names this row
- Not ready during provisioning: no verification run names this row
- Not ready before the first background check: no verification run names this row
- Ready after provisioning: no verification run names this row
- Not ready after a failed health check: no verification run names this row
- Ready again after Qdrant recovers: no verification run names this row
- Many probes, no Qdrant traffic: no verification run names this row
- Default render: no verification run names this row
- Console path is refused: no verification run names this row
- Login path is refused: no verification run names this row
- Connect route is refused: no verification run names this row
- Bare root is refused: no verification run names this row
- Root MCP path has no catch-all on the second listener: no verification run names this row
- Console path during provisioning: no verification run names this row
- Attempt runs out of its own budget: no verification run names this row
- Budget runs out: no verification run names this row
- Error that is not transient: asserts one attempt and unchanged error, not exit code
- CLI verb against an unreachable Qdrant: no verification run names this row
- Three failures then success: no verification run names this row
- Exhaustion: no verification run names this row
- Unset: no verification run names this row
- Invalid startup timeout: no verification run names this row
- Request during provisioning: no verification run names this row
- Cross-origin request during provisioning: no verification run names this row
- Request after provisioning: no verification run names this row
- Quantization rejected by Qdrant: no verification run names this row
- Signal during an attempt: no verification run names this row
- Signal during schema convergence: no verification run names this row
- Signal during the backoff wait: no verification run names this row
- Slow index build within the schema budget: no verification run names this row
- Schema budget exceeded: no verification run names this row
- Invalid schema timeout: no verification run names this row
- Startup timeout shorter than one attempt: no verification run names this row
- Raised schema timeout: no verification run names this row
- Raised startup timeout: no verification run names this row
- Default render: no verification run names this row
- Invalid startup timeout value: no verification run names this row

## Verification runs

- exit 0: `go test ./cmd/engram -count=1 -run '^(TestPropertyReadiness|TestReadinessInitialPing|TestStartupGate|TestPropertyStartupGate|TestStartServeSignal|TestStartServeProvisioningError|TestMCPOnlyMux|TestOperationErrorExitCodes)$'`
- exit 0: `go test ./internal/config -count=1 -run '^(TestQdrantProvisioningDefaultsAndEnv|TestValidateFieldRules|TestValidateStartupTimeoutAccepted|TestValidatePositiveDurationText|TestPropertyValidatePositiveDuration)$'`
- exit 0: `go test ./internal/server -count=1 -run '^(TestProvisionStoreBudgets|TestPropertyServeProvisionRetryLog|TestServeProvisionRetry|TestStoreFromEnvSingleAttempt)$'`
- exit 0: `go test ./internal/e2e -count=1 -run '^(TestServeStartupTimeout|TestServeWaitsForQdrant|TestServeSignalDuringStartup)$'`
- exit 0: `task chart:validate`

## Dropped

None.

## Lens notes

- approach: Every requirement of the three spec deltas has a decision that produces it, and the code at head follows D1 to D6 apart from the findings above.
- approach: Cited library behavior holds at the pinned versions: backoff v7.0.3 skips Notify on the final attempt and refuses a wait past MaxElapsedTime; qdrant go-client v1.20.1 defaults to a pool of 3, a 1 minute version check with no context, and only warns on its failure; grpc v1.85.0 DefaultConfig has MaxDelay 120s, Jitter 0.2, and minConnectTimeout 20s.
- approach: The rejected alternative of the Qdrant client's RetryConfig holds: RetryConfig bounds retries by MaxRetries and has no per-attempt hook.
- approach: The healthLoop also logs at info level on recovery. D4 does not name it, but no decision forbids it, so it is not reported.
- approach: The Context section of design.md still names isUnavailableTransportErr in operror.go. That section describes the code before the change, so it is not reported as stale.
- fit: The prompt names no path for repository library-preference rows; only the generic rows of the rulebook were applied.
- fit: The retry reuses cenkalti/backoff through the existing newRetryBackOff helper, and the transient test moved to store.IsUnavailable, which the CLI classifier now also calls; both fit the closest existing pattern.
- fit: ParsePositiveDuration and validatePositiveDuration mirror the existing ParsePositiveIntCap and validatePositiveCap pair; the remaining inline positive-duration checks in validate.go and the runtime parsers in internal/server are tracked by #1019 (task 1.4), so they are not reported.
- fit: The chart probes follow the httpGet shape the bundled Qdrant template already uses, and engram.positiveSeconds follows the fail-on-render pattern of engram.mcpListenPort.
- fit: Test files were read for their helpers and fixtures, not for each test body; judging test bodies belongs to the coverage and honesty lenses.
- invariants: I1: ENGRAM_QDRANT_STARTUP_TIMEOUT is always a positive Go duration \| enforced at internal/config/validate.go ParsePositiveDuration via validatePositiveDuration in Config.Validate, and reparsed with the same parser in internal/server/tools.go buildDepsFromEnv \| boundary test internal/config/duration_test.go TestValidateStartupTimeoutAccepted (1ns accepted), TestValidatePositiveDurationText (0 and abc), validate_test.go TestValidateFieldRules (0, -1s, abc), TestPropertyValidatePositiveDuration (every int64)
- invariants: I2: ENGRAM_QDRANT_SCHEMA_TIMEOUT is always a positive Go duration \| enforced at the same validatePositiveDuration and ParsePositiveDuration in ensureStoreFromConfig \| boundary test TestValidateFieldRules (0s, -1m, 120) and TestValidatePositiveDurationText
- invariants: I3: memory.qdrant.startupTimeoutSeconds and schemaTimeoutSeconds always render as a whole number of seconds in 1..9223372036 \| enforced at charts/engram/templates/_helpers.tpl engram.positiveSeconds \| boundary test Taskfile.yaml chart:validate (1, 9223372036, 0, -1, 0.5, 1.9, true, 9223372037). Probe under the handoff dir: 1e20, null and a quoted "60" fail naming the value, 1.0 renders 1s. time.ParseDuration accepts 9223372036s and rejects 9223372037s, so the chart range and the Go range agree at the upper edge.
- invariants: I4: No connect-and-create attempt runs past the startup timeout, and each attempt is bounded by 15 s \| enforced at internal/server/startup.go:97 context.WithTimeout(parent, policy.attemptBudget()) with parent created before client construction in tools.go:305 \| boundary test TestServeProvisionRetry 'a blocked attempt at the end of the budget is a spent budget' (50 ms) and 'each attempt ends at its own budget while the startup budget is live' (20 ms), TestServeCreateErrorText pins the 15s text, e2e TestServeStartupTimeout (2s against a silent listener)
- invariants: I5: The construction-time version check never waits longer than min(startup timeout, 15 s) \| enforced at internal/server/startup.go:63 versionCheckTimeout, passed as qdrant.Config.VersionCheckTimeout (go-client v1.20.1 config.go:89 uses it when > 0) \| boundary test TestVersionCheckTimeout (1ns, 14999ms, 15s, 15001ms, 1m: both sides of the equality)
- invariants: I6: Only Unavailable, a net.Error, or a gRPC DeadlineExceeded status is retried, and only while the startup budget is live \| enforced at internal/server/startup.go:76 isTransientStartupErr and :105 \| boundary test TestServeProvisionRetry PermissionDenied (one attempt) and DeadlineExceeded (second attempt runs), TestPropertyServeProvisionRetryLog
- invariants: I7: The retry result is decided from the contexts: a done signal context gives its error, a spent budget gives an Unavailable status naming ENGRAM_QDRANT_STARTUP_TIMEOUT, anything else gives the create error unchanged \| enforced at internal/server/startup.go:116-129 (backoff v7.0.3 retry.go:141,162,185 return RetryError with LastErr and Cause) \| boundary test TestServeProvisionRetry elapsed-time branch with a parentLive check, signal during attempt and during backoff wait, TestPropertyServeProvisionRetryLog
- invariants: I8: Every failed attempt, the last one included, is logged at warn with its number \| enforced at internal/server/startup.go:103 inside the operation closure \| boundary test TestServeProvisionRetry (1..3) and TestPropertyServeProvisionRetryLog (0..6 transient plus each terminal outcome)
- invariants: I9: Verbs other than serve make exactly one create attempt \| enforced at tools.go:298 policy.retry() false for the zero startupPolicy \| boundary test TestStoreFromEnvSingleAttempt (closed port, no attempt log)
- invariants: I10: No MCP, Connect or console handler runs before provisioning, and every admitted request except GET /livez and GET /readyz gets 503 with Retry-After \| enforced at cmd/engram/startupgate.go:48 (nil target answers 503) \| boundary test TestStartupGate, TestPropertyStartupGate over a fixed path set (see inv-1)
- invariants: I11: A request the cross-origin check refuses gets 403 in every state \| enforced at serve.go newHTTPServer, CrossOriginProtection wraps the gate \| boundary test TestStartupGate cross-site POST before and after setTarget, TestPropertyStartupGate
- invariants: I12: GET /readyz is 200 exactly when the gate target is set and the last finished ping succeeded; the ping result starts false \| enforced at startupgate.go:40 and the zero atomic.Bool \| boundary test TestPropertyReadiness, TestReadinessInitialPing (held initial ping gives 503, then 200)
- invariants: I13: A /readyz request never sends a Qdrant request \| enforced structurally, the gate reads two atomics only \| boundary test TestPropertyReadiness ping count equals ticks plus one whatever the number of /readyz calls
- invariants: I14: Every gate target is set before the health loop starts \| enforced at startupgate.go:100 markProvisioned \| boundary test TestMarkProvisioned (MCP-only gate included)
- invariants: I15: A signal during startup gives exit code 0; any other startup error is returned unchanged \| enforced at serve.go:630 startupResult and serve.go:616 startServe \| boundary test TestStartupResult (both sides), TestStartServeSignal (listener closed), e2e TestServeSignalDuringStartup
- invariants: I16: Schema convergence is bounded by the schema budget and the signal context, never by the startup timeout \| enforced at tools.go:322 and :347 (sctx derived from the signal context) and :350 \| boundary test TestServeSchemaNotCutByStartupTimeout (2s startup, 3s stall, 10s schema), TestProvisionStoreBudgets signal subtest
- invariants: I17: ENGRAM_MCP_PATH never names /livez or /readyz \| enforced at cmd/engram/mcproute.go reservedMCPRoots in resolveMCPPath \| boundary test TestResolveMCPPath rows for /livez and /readyz
- invariants: Chart tightening, outside my categories: a quoted value such as schemaTimeoutSeconds: "600" rendered 600s at base 33dd9c76 through the old int coercion and now fails the render. The message reads got 600, which looks like a number. guides/upgrade.md does not name the change. Reproduced with helm template on both charts under the handoff probe dir.
- invariants: TestPropertyServeBackOffBound draws only the number of NextBackOff calls. The jitter comes from the backoff library's own random source, not from rapid, so a failure would not shrink. With 64 draws the 7.5 s bound is still checked many times past the 5 s cap.
- test-coverage: Two scenarios are titled Default render (server-health and collection-provisioning); both have a row. The chart rows are proved by the chart:validate task, which the controller must run; no Go test covers them.
- test-coverage: The MCP-listener rows rest on existing TestMCPOnlyMux, which was not read in detail.
- test-honesty: TestPropertyServeBackOffBound compares against serveRetryMaxInterval*3/2, a production constant. The 7.5s bound is not pinned independently.
- test-honesty: No new disabled or skipped tests found in the changed test files. The skips found were already there or are the Qdrant-missing skip.
- test-honesty gap: Not read: config, store, chart and harness tests; Taskfile chart:validate gates.

## Plan review (2026-11-14)

Verdict approve-with-notes at head 33dd9c763325381e94d2bb9d09875dd9cd713307.

### Findings

| id | lens | severity | claim | evidence | action |
|---|---|---|---|---|---|
| app-1 | approach | should-fix | Task 1.4 says Config.Validate has three other inline positive-duration checks. At head it has five: ENGRAM_EMBED_MAX_TIMEOUT, ENGRAM_SUMMARY_MAX_TIMEOUT, ENGRAM_DECISIONS_MAX_TIMEOUT, ENGRAM_SEARCH_RERANK_TIMEOUT and ENGRAM_SEARCH_UNDERSTANDING_TIMEOUT. The follow-up issue the task asks for would name too few keys. | tasks.md > 1. Startup timeout configuration key — "Open a GitHub issue to move the three other inline positive-duration checks in `Config.Validate` to the new helpers. Nam" | In task 1.4, change three to five and name the five keys, or name the three keys you mean and give the reason the other two stay out. |
| ctl-lens | fit | should-fix | lens fit reported partial, gaps: 1 | review > lens-status — "partial" | rerun the lens or accept the gap in the review |
| fit-1 | fit | should-fix | The new validatePositiveDuration helper sits beside five other inline positive-duration checks in Config.Validate, but task 1.4 tracks only three, so two copies stay untracked. | tasks.md > 1. Startup timeout configuration key — "Open a GitHub issue to move the three other inline positive-duration checks in `Config.Validate` to the new helpers." | In task 1.4, name all five keys (ENGRAM_EMBED_MAX_TIMEOUT, ENGRAM_SUMMARY_MAX_TIMEOUT, ENGRAM_DECISIONS_MAX_TIMEOUT, ENGRAM_SEARCH_RERANK_TIMEOUT, ENGRAM_SEARCH_UNDERSTANDING_TIMEOUT), not three. |
| fit-2 | fit | should-fix | D5 makes validation and runtime share config.ParsePositiveDuration for the Qdrant budgets only. Five runtime parsers in internal/server repeat the same parse-and-check, and no task or issue covers them. | design.md > D5. Config and chart — "`internal/server` parses the schema and startup budgets with `config.ParsePositiveDuration`, so validation and runtime s" | Widen the follow-up issue of task 1.4 to also move embedMaxTimeout, summaryMaxTimeout, decisionsMaxTimeout and the rerank and understanding timeout parsers in internal/server to config.ParsePositiveDuration, and name those functions in it. |
| inv-1 | invariants | should-fix | The proposal says reconnect behavior after startup does not change. D3 still gives serve's only Qdrant client grpc.WithConnectParams with a 5 s MaxDelay, and that client serves every later request, so post-startup reconnects change from the 120 s default. | proposal.md > What Changes — "Reconnect behavior for requests after startup does not change." | Either restate the out-of-scope line to say serve's client keeps the 5 s reconnect cap for its whole life (and name it in deploy.md), or build the startup attempts on a client separate from the serving client. |
| inv-2 | invariants | should-fix | The existing mcp-listener-isolation requirement says the second listener answers every non-MCP, non-well-known path with 404. The D1 gate on that listener answers 503 to every path until provisioning completes, and no delta in the change modifies that requirement. | openspec/specs/mcp-listener-isolation/spec.md:34 — "SHALL receive 404 with no redirect, no console content, no login flow and no Connect response. The" | Add a MODIFIED delta for 'The second listener refuses every other path' that limits the 404 rule to after provisioning, and add a TestStartupGate case for a /ui/ GET on the MCP-only gate before its target is set. |
| inv-3 | invariants | should-fix | The positiveSeconds rule in D5 accepts a boolean. A probe render of `v: true` passes the float64-versus-int check and renders 1s, which breaks the spec's whole-number rule. The D6 chart row tests numbers only. | design.md > D5. Config and chart — "for example when `float64 $v` differs from `float64 (int $v)`" | Have the helper also require kindIs int64/float64 (or reject kindIs bool), and add a `true` case for both timeouts to the task 5.3 chart:validate assertions. |
| inv-4 | invariants | should-fix | Any whole number of seconds above 9223372036 passes the planned chart helper (a probe rendered 9223372036854775807s), but time.ParseDuration rejects it as an invalid duration. The chart then renders an env value that the binary refuses at startup, and no test passes the maximum. | design.md > D5. Config and chart — "It fails the render on a value that is not a positive whole number" | Bound the helper at 9223372036 seconds (the largest whole-second Go duration) and add chart:validate cases for 9223372036 (renders) and 9223372037 (fails) for both timeouts. |
| inv-5 | invariants | should-fix | The spec's 7.5 s promise rests on the serve retry calling newRetryBackOff with a 5 s maximum interval. Every retry test injects its own BackOff, and TestStartupConnectParams pins only gRPC's MaxDelay, so changing the production maximum interval breaks no test. | specs/collection-provisioning/spec.md > Scenario: Qdrant comes up within the budget — "Qdrant starts accepting them while at least 7.5 s of `ENGRAM_QDRANT_STARTUP_TIMEOUT` remain" | Name the serve policy's BackOff constructor and add a test that draws NextBackOff many times from it and asserts every value is at most 7.5 s. Use a rapid property or a fixed loop. |
| inv-6 | invariants | should-fix | Nothing tests the scenario 'Not ready before the first background check' at its edge. No test sets the target, holds the initial ping open, and asserts /readyz 503. The planned scenario case covers only the state after the initial ping succeeds. | specs/server-health/spec.md > Scenario: Not ready before the first background check — "provisioning completed and no background health check finished yet" | Add a case where the fake ping blocks on a channel, the gate target is set, and /readyz returns 503 until the ping is released. Then it returns 200. |
| spe-1 | spec-lint | should-fix | THEN step 'starts another attempt' names no observable; the retry scenario has no readable attempt count, log line or exit code. | specs/collection-provisioning/spec.md > Scenario: Attempt runs out of its own budget — "starts another attempt instead of failing" | Name the observable, for example a warn log entry with the next attempt number, or the attempt count reaching 2 in the test. |
| tes-1 | test-coverage | should-fix | The BREAKING removal of 'Server listens only after provisioning completes' has no scenario for the removal or the migration to /readyz. | specs/collection-provisioning/spec.md > Requirement: Server listens only after provisioning completes — "A check that relied on the port being closed during provisioning must use `GET /readyz` instead." | Add a scenario under the REMOVED requirement or the gate requirement: a TCP check passes during provisioning while GET /readyz returns 503. Name the e2e test TestServeWaitsForQdrant in internal/e2e as its proof, run by task test:e2e. |
| inv-7 | invariants | note | The retry's result decision (D3 rules 1 to 3) is a function over any sequence of attempt errors, backoff causes and context states. The planned log property draws terminal outcomes but asserts only the log numbering, not the returned error class. | design.md > D6. Tests — "The warn entries are numbered 1 to the number of failed attempts." | Extend TestPropertyServeProvisionRetryLog (pgregory.net/rapid, as in the planned internal/server property) to assert per outcome: a signal gives context.Canceled, exhaustion gives an Unavailable status naming ENGRAM_QDRANT_STARTUP_TIMEOUT, and a permanent error satisfies errors.Is with the create error. |

### Coverage matrix

| scenario | covered by | status | verified |
|---|---|---|---|
| Live during provisioning | 3.2 | covered |  |
| Live while Qdrant is down after start | 3.2 | covered |  |
| Not ready during provisioning | 3.8 | covered |  |
| Not ready before the first background check | 4.2 | covered |  |
| Ready after provisioning | 4.2 | covered |  |
| Not ready after a failed health check | 4.2 | covered |  |
| Ready again after Qdrant recovers | 4.2 | covered |  |
| Many probes, no Qdrant traffic | 4.2 | covered |  |
| Default render | 5.3 | covered |  |
| Qdrant comes up within the budget | 2.7, 3.8 | covered |  |
| Attempt runs out of its own budget | 2.7 | covered |  |
| Budget runs out | 2.7, 3.8 | covered |  |
| Error that is not transient | 2.7 | covered |  |
| CLI verb against an unreachable Qdrant | 2.9 | covered |  |
| Three failures then success | 2.7, 2.8 | covered |  |
| Exhaustion | 2.7, 2.8 | covered |  |
| Unset | 1.2 | covered |  |
| Smallest positive value | 1.2 | covered |  |
| Invalid startup timeout | 1.2, 1.3 | covered |  |
| Port accepts connections during provisioning | 3.8 | covered |  |
| Request during provisioning | 3.2, 3.8 | covered |  |
| Cross-origin request during provisioning | 3.2, 3.3 | covered |  |
| Request after provisioning | 3.2, 3.8 | covered |  |
| Quantization rejected by Qdrant | 3.6 | covered |  |
| Signal during an attempt | 3.6, 3.8 | covered |  |
| Signal during schema convergence | 2.7, 3.6 | covered |  |
| Signal during the backoff wait | 2.7, 3.6 | covered |  |
| Slow index build within the schema budget | 2.4 | covered |  |
| Schema budget exceeded | 2.4 | covered |  |
| Invalid schema timeout | 1.1 | covered |  |
| Startup timeout shorter than one attempt | 2.7, 3.8 | covered |  |
| Raised schema timeout | 5.3 | covered |  |
| Raised startup timeout | 5.3 | covered |  |
| Invalid startup timeout value | 5.3 | covered |  |

### Not run

Plan phase: no verification runs.
