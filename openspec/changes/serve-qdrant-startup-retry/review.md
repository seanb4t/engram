---
verdict: changes-requested
phase: implementation
reviewed_at: 2026-11-14T20:32:05+00:00
head: 0e42d4ccbd0f423859f01a6e7c6eb80ea26d22c1
lenses:
  spec-lint: not-applicable
  approach: ok
  fit: ok
  test-coverage: ok
  test-honesty: partial
  invariants: ok
---
# Review

## Incomplete lenses

- test-honesty: partial, gaps: 1

The verdict covers only what the lenses checked.

## Findings

| id | lens | severity | claim | evidence | action |
|---|---|---|---|---|---|
| fit-5 | fit | blocker | The new assertClosed helper is a third copy of the dial-and-fail closed-listener check that serve_test.go already writes inline twice in the same package. | cmd/engram/startupgate_test.go:172 — "if conn, err := net.Dial("tcp", addr); err == nil {" | Call assertClosed from TestServeBothListeners and TestServeAllServeFailure instead of the inline dial checks. |
| tes-1 | test-coverage | blocker | No test cancels the signal context while schema convergence (index build) runs and asserts a prompt exit 0. | specs/collection-provisioning/spec.md > Requirement: Shutdown signal during startup is a normal stop — "SIGTERM arrives while `engram serve` builds payload indexes" | Add a subtest to TestProvisionStoreBudgets in internal/server/provision_test.go: cancel ctx during a stalled createFieldIndex (stallFirst) and assert provisionStore returns context.Canceled well before ENGRAM_QDRANT_SCHEMA_TIMEOUT; or an e2e case in internal/e2e/startup_test.go. |
| app-1 | approach | should-fix | D2 says startServe holds steps 3 and 4 and returns the gate targets. In the code startServe returns only the drain, and route mounting, the gate targets and the health-loop start run inside the register closure (serveSetup.provision). design.md records no deviation. | design.md > D2. Bind first, then start up behind the gate — "It takes the register-and-provision step as a function parameter and returns the gate targets, so tests drive it without" | Either record in D2 that startServe returns the drain and that the register step (serveSetup.provision) mounts the routes, sets the gate targets and starts the health loop, or move step 4 into startServe and return the targets as D2 says. |
| app-2 | approach | should-fix | D2 names a leading ctx as Register's only signature change. The code also adds a second result, the ping function behind /readyz. design.md records no deviation. | design.md > D2. Bind first, then start up behind the gate — "`Register` gains a leading `ctx context.Context` parameter and passes it to `buildDepsFromEnv`." | Record in D2 or D4 that Register also returns the Store's HealthCheck as the ping function of the health loop. |
| app-3+inv-2 | approach,invariants | should-fix | app-3: D3 rule 3 says a non-transient create error keeps today's text. On the serve path the retry returns the bare CreateCollectionIfAbsent error, without the create collection (budget 15s) prefix that the one-attempt step still adds, so the serve error text changes.; inv-2: Design D3 rule 3 says a non-transient create error keeps today's text, but the serve path no longer wraps it with the create-collection step and budget; only singleCreate (the CLI path) still adds that prefix, and no test pins the serve-path text end to end. | design.md > D3. Retry in the serve provisioning path only — "so the retry returns that wrapper's `LastErr`, the create step's own error. Its text and its exit code stay what they ar" | app-3: Wrap the serve attempt's error with the same create collection (budget ...) text that singleCreate uses, or record in D3 that the serve path's permanent-error text loses that prefix.; inv-2: Wrap the permanent create error in the serve create closure with the same create-collection step text singleCreate uses, and pin the full serve-path error text for a PermissionDenied create in a test through ensureStoreFromConfig. |
| fit-1+inv-7 | fit,invariants | should-fix | fit-1: The 5 s gRPC reconnect cap and the 5 s retry maximum interval are two named constants in two packages for one value, kept equal only by a comment.; inv-7: The 7.5 s reach promise assumes a redial within one retry interval, but gRPC jitters the 5 s MaxDelay by 0.2 (up to 6 s) and the client round-robins three pooled connections, so an attempt after Qdrant comes up can still fail fast on a subchannel that has not redialed. | internal/store/store.go:563 — "const startupReconnectMaxDelay = 5 * time.Second" | fit-1: Give store.StartupConnectParams a maxDelay parameter and pass serveRetryMaxInterval from startupPolicy.dialOptions, then delete startupReconnectMaxDelay.; inv-7: Restate the promise as the backoff bound plus the jittered reconnect delay (7.5 s + 6 s), or lower the reconnect MaxDelay so the jittered maximum fits the shortest retry wait, and put the corrected number in the spec scenario and deploy.md. |
| fit-3 | fit | should-fix | server.Register grows a third positional return, the ping function, which design D2 does not name; the design describes only a leading ctx parameter. | internal/server/tools.go:2890 — "(shutdown func(context.Context), ping func(context.Context) error, err error) {" | Name the ping return of Register in design D2 or D4 with the reason, or return the readiness source through an existing value instead of a new positional result. |
| inv-1+hon-3 | invariants,test-honesty | should-fix | inv-1: The test named for Qdrant coming up on the last attempt the budget allows uses a one-minute budget, a zero backoff and success on call 5, a value deep inside the range; the 7.5 s edge of the spec scenario is never exercised.; hon-3: Subtest 'Qdrant up on the last attempt the budget allows' uses a one-minute budget and zero backoff, so it never reaches the budget edge its name claims; it only asserts err == nil, same as the first subtest. | internal/server/startup_test.go:233 — "t.Run("Qdrant up on the last attempt the budget allows", func(t *testing.T) {" | inv-1: Inject a BackOff with a fixed wait and a budget sized so the success lands on the final attempt that backoff's elapsed-time check still admits, and add the twin case one wait later that must end in the budget error.; hon-3: Make the budget bind (short timeout, success on the final attempt that fits) or remove the duplicate. |
| inv-3 | invariants | should-fix | Signal during schema convergence returns the context error from provisionStore instead of the schema message, but no test cancels the context passed to provisionStore while EnsureSchema runs; TestStartServeSignal uses a fake register step. | internal/server/tools.go:347 — "if ctx.Err() != nil {" | Add a TestProvisionStoreBudgets subtest that stalls createFieldIndex, cancels the provisionStore context mid-stall with a long schema budget, and asserts errors.Is(err, context.Canceled), a return well before the budget, and no schema-provisioning text. |
| inv-4 | invariants | should-fix | The health loop warns on the initial failed ping and on an ok-to-fail change only, but TestPropertyReadiness counts warns in its own checked hook from the test's model, so the slog.Warn branch of run is never observed at either edge. | cmd/engram/startupgate.go:76 — "case err != nil && (first \|\| last):" | Capture slog output in TestPropertyReadiness and count the qdrant health check failed warn records emitted by run, then compare that count with the model, covering an initial failure and two failures in a row. |
| inv-5 | invariants | should-fix | The version-check bound is min(startup timeout, 15 s), and no test checks either side of the 15 s equality; only the e2e 2 s run exercises the lower side indirectly through its exit deadline. | internal/server/startup.go:53 — "return min(p.timeout, collectionCreateBudget)" | Add a table test of startupPolicy.versionCheckTimeout for 1ns, 14.999s, 15s, 15.001s and 60s. |
| inv-6 | invariants | should-fix | Each attempt is capped at the 15 s per-attempt budget while the parent is live, but collectionCreateBudget is a constant and no test sees an attempt end at its own budget; the cut-off test returns a fake DeadlineExceeded at once. | internal/server/startup.go:86 — "actx, cancel := context.WithTimeout(parent, collectionCreateBudget)" | Carry the per-attempt budget in startupPolicy so a test can set it to 20 ms under a 1 s parent, block the attempt on ctx.Done, and assert the attempt ends near 20 ms and a second attempt runs. |
| ctl-not-run | test-coverage | should-fix | covered row 'Live during provisioning' has no verification run | review > not-run — "cmd/engram/startupgate_test.go: TestStartupGate/primary gate before and after provisioning; internal/e2e/startup_test.go" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Not ready during provisioning' has no verification run | review > not-run — "cmd/engram/startupgate_test.go: TestStartupGate; internal/e2e/startup_test.go: TestServeWaitsForQdrant" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Not ready before the first background check' has no verification run | review > not-run — "cmd/engram/startupgate_test.go: TestReadinessInitialPing/not ready while the initial ping is open" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Ready after provisioning' has no verification run | review > not-run — "cmd/engram/startupgate_test.go: TestReadinessInitialPing/ready after the initial ping, with no tick" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Ready again after Qdrant recovers' has no verification run | review > not-run — "cmd/engram/startupgate_test.go: TestPropertyReadiness (tick-fail then tick-ok)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Many probes, no Qdrant traffic' has no verification run | review > not-run — "cmd/engram/startupgate_test.go: TestPropertyReadiness (pings == ticks+1 invariant)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Default render' has no verification run | review > not-run — "Taskfile.yaml: chart:validate (probe awk checks, lines 461-466); probes not asserted via a Go test" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Budget runs out' has no verification run | review > not-run — "internal/e2e/startup_test.go: TestServeStartupTimeout; internal/server/startup_test.go: TestServeProvisionRetry" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Error that is not transient' has no verification run | review > not-run — "internal/server/startup_test.go: TestServeProvisionRetry/an error that is not transient fails at once, unchanged" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Three failures then success' has no verification run | review > not-run — "internal/server/startup_test.go: TestServeProvisionRetry/unavailable then success logs each failure" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Smallest positive value' has no verification run | review > not-run — "internal/config/duration_test.go: 1ns case" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Request during provisioning' has no verification run | review > not-run — "internal/e2e/startup_test.go: TestServeWaitsForQdrant; cmd/engram/startupgate_test.go: TestStartupGate" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Cross-origin request during provisioning' has no verification run | review > not-run — "cmd/engram/startupgate_test.go: TestStartupGate; TestPropertyStartupGate" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Request after provisioning' has no verification run | review > not-run — "internal/e2e/startup_test.go: TestServeWaitsForQdrant (checkDocumentedTools)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Quantization rejected by Qdrant' has no verification run | review > not-run — "internal/server/provision_test.go: TestProvisionStoreBudgets/a rejected quantization update fails provisioning; cmd/engr" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Signal during an attempt' has no verification run | review > not-run — "internal/server/startup_test.go: TestServeProvisionRetry; internal/e2e/startup_test.go: TestServeSignalDuringStartup" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Signal during the backoff wait' has no verification run | review > not-run — "internal/server/startup_test.go: TestServeProvisionRetry/a signal during the backoff wait stops the retry" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Schema budget exceeded' has no verification run | review > not-run — "internal/server/provision_test.go: TestProvisionStoreBudgets/exceeding the schema budget names the step and the variable" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Startup timeout shorter than one attempt' has no verification run | review > not-run — "internal/e2e/startup_test.go: TestServeStartupTimeout/silent listener" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Raised schema timeout' has no verification run | review > not-run — "Taskfile.yaml: chart:validate (line 469)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Raised startup timeout' has no verification run | review > not-run — "Taskfile.yaml: chart:validate (line 469)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Invalid startup timeout value' has no verification run | review > not-run — "Taskfile.yaml: chart:validate (line 475 loop)" | run the covering test and record it in verification-runs.txt |
| ctl-not-run | test-coverage | should-fix | covered row 'Console path during provisioning' has no verification run | review > not-run — "cmd/engram/startupgate_test.go: TestStartupGate/MCP-only gate before provisioning" | run the covering test and record it in verification-runs.txt |
| tes-2 | test-coverage | should-fix | Qdrant-comes-up scenario asserts attempt logs only by wait and not that each failed attempt is a warn-level entry; log level unchecked in e2e. | internal/e2e/startup_test.go:115 — "waitForLog(t, srv, attemptFailedLn, 2)" | In TestServeWaitsForQdrant assert the logged attempt lines carry level=WARN and count matches failed attempts (or rely on the unit test and name it). |
| tes-3 | test-coverage | should-fix | Mid-run Qdrant outage for /livez is asserted only with a fake failed ping, not a stopped Qdrant after provisioning. | cmd/engram/startupgate_test.go:81 — "GET /livez with a failed ping = %d, want 200" | The failed-ping assertion never sets a failing ping on the health loop; make the ping fail before the /livez check. |
| ctl-lens | test-honesty | should-fix | lens test-honesty reported partial, gaps: 1 | review > lens-status — "partial" | rerun the lens or accept the gap in the review |
| hon-1 | test-honesty | should-fix | TestPropertyReadiness warn-count invariant compares the test's own re-implementation of the warn rule (in its checked hook) against its model; the production warn log is never captured. | cmd/engram/startupgate_test.go:243 — "health.checked = func() {" | Capture the slog output of healthLoop (as captureAttempts does) and count its real warn lines; drop the hook-side counter. |
| hon-2 | test-honesty | should-fix | TestPropertyStartupGate asserts the target saw 0 calls, but the target is never attached to the gate, so that check cannot fail; the same holds for the MCP-only subtest of TestStartupGate. | cmd/engram/startupgate_test.go:107 — "target := &countingTarget{}" | Call g.setTarget(target) in a variant so the pre-provisioning check can fail if the gate leaks, or drop the dead assertion. |
| fit-2 | fit | note | ParsePositiveDuration adds a further positive-duration check beside the inline ones in Config.Validate and the runtime parsers; design names it and task 1.4 tracks the collapse, but no issue number is recorded in the artifacts. | internal/config/validate.go:124 — "case d <= 0:" | Record the issue number opened by task 1.4 in tasks.md or design.md D5, so the collapse is traceable. |
| fit-4 | fit | note | TestServeWaitsForQdrant repeats the /readyz poll loop of serverProc.waitReady inline with a 10 s deadline instead of 45 s. | internal/e2e/startup_test.go:135 — "for get(t, srv.baseURL()+"/readyz") != http.StatusOK {" | Give waitReady a deadline parameter and call srv.waitReady(t, 10*time.Second) from TestServeWaitsForQdrant. |

## Coverage matrix

| scenario | covered by | status | verified |
|---|---|---|---|
| Live during provisioning | cmd/engram/startupgate_test.go: TestStartupGate/primary gate before and after provisioning; internal/e2e/startup_test.go: TestServeWaitsForQdrant | covered | not-run |
| Live while Qdrant is down after start | cmd/engram/startupgate_test.go: TestStartupGate (livez after setTarget; ping not actually failed) | partial | not-run |
| Not ready during provisioning | cmd/engram/startupgate_test.go: TestStartupGate; internal/e2e/startup_test.go: TestServeWaitsForQdrant | covered | not-run |
| Not ready before the first background check | cmd/engram/startupgate_test.go: TestReadinessInitialPing/not ready while the initial ping is open | covered | not-run |
| Ready after provisioning | cmd/engram/startupgate_test.go: TestReadinessInitialPing/ready after the initial ping, with no tick | covered | not-run |
| Not ready after a failed health check | cmd/engram/startupgate_test.go: TestPropertyReadiness | covered | pass |
| Ready again after Qdrant recovers | cmd/engram/startupgate_test.go: TestPropertyReadiness (tick-fail then tick-ok) | covered | not-run |
| Many probes, no Qdrant traffic | cmd/engram/startupgate_test.go: TestPropertyReadiness (pings == ticks+1 invariant) | covered | not-run |
| Default render | Taskfile.yaml: chart:validate (probe awk checks, lines 461-466); probes not asserted via a Go test | covered | not-run |
| Qdrant comes up within the budget | internal/e2e/startup_test.go: TestServeWaitsForQdrant | partial | pass |
| Attempt runs out of its own budget | internal/server/startup_test.go: TestServeProvisionRetry/an attempt cut off by its own budget is retried | partial | not-run |
| Budget runs out | internal/e2e/startup_test.go: TestServeStartupTimeout; internal/server/startup_test.go: TestServeProvisionRetry | covered | not-run |
| Error that is not transient | internal/server/startup_test.go: TestServeProvisionRetry/an error that is not transient fails at once, unchanged | covered | not-run |
| CLI verb against an unreachable Qdrant | internal/server/startup_test.go: TestStoreFromEnvSingleAttempt | covered | pass |
| Three failures then success | internal/server/startup_test.go: TestServeProvisionRetry/unavailable then success logs each failure | covered | not-run |
| Exhaustion | internal/server/startup_test.go: TestPropertyServeProvisionRetryLog | covered | pass |
| Unset | internal/config/config_test.go: TestQdrantProvisioningDefaultsAndEnv | covered | pass |
| Smallest positive value | internal/config/duration_test.go: 1ns case | covered | not-run |
| Invalid startup timeout | internal/config/duration_test.go; internal/config/validate_test.go: TestValidateFieldRules | covered | pass |
| Port accepts connections during provisioning | internal/e2e/startup_test.go: TestServeWaitsForQdrant | covered | pass |
| Request during provisioning | internal/e2e/startup_test.go: TestServeWaitsForQdrant; cmd/engram/startupgate_test.go: TestStartupGate | covered | not-run |
| Cross-origin request during provisioning | cmd/engram/startupgate_test.go: TestStartupGate; TestPropertyStartupGate | covered | not-run |
| Request after provisioning | internal/e2e/startup_test.go: TestServeWaitsForQdrant (checkDocumentedTools) | covered | not-run |
| Quantization rejected by Qdrant | internal/server/provision_test.go: TestProvisionStoreBudgets/a rejected quantization update fails provisioning; cmd/engram/startupgate_test.go: TestStartServeProvisioningError | covered | not-run |
| Signal during an attempt | internal/server/startup_test.go: TestServeProvisionRetry; internal/e2e/startup_test.go: TestServeSignalDuringStartup | covered | not-run |
| Signal during schema convergence | none | missing | not-run |
| Signal during the backoff wait | internal/server/startup_test.go: TestServeProvisionRetry/a signal during the backoff wait stops the retry | covered | not-run |
| Slow index build within the schema budget | internal/server/provision_test.go: TestProvisionStoreBudgets | covered | pass |
| Schema budget exceeded | internal/server/provision_test.go: TestProvisionStoreBudgets/exceeding the schema budget names the step and the variable | covered | not-run |
| Invalid schema timeout | internal/config/duration_test.go; validate_test.go: TestValidateFieldRules | covered | pass |
| Startup timeout shorter than one attempt | internal/e2e/startup_test.go: TestServeStartupTimeout/silent listener | covered | not-run |
| Raised schema timeout | Taskfile.yaml: chart:validate (line 469) | covered | not-run |
| Raised startup timeout | Taskfile.yaml: chart:validate (line 469) | covered | not-run |
| Invalid startup timeout value | Taskfile.yaml: chart:validate (line 475 loop) | covered | not-run |
| Console path is refused | internal/e2e/mcp_listener_test.go: TestMCPOnlyListenerWithConsole | covered | pass |
| Login path is refused | internal/e2e/mcp_listener_test.go: TestMCPOnlyListenerWithConsole | covered | pass |
| Connect route is refused | internal/e2e/mcp_listener_test.go: TestMCPOnlyListener | covered | pass |
| Bare root is refused | internal/e2e/mcp_listener_test.go: TestMCPOnlyListenerWithConsole | covered | pass |
| Root MCP path has no catch-all on the second listener | cmd/engram/mcproute_test.go (not verified by name) | partial | not-run |
| Console path during provisioning | cmd/engram/startupgate_test.go: TestStartupGate/MCP-only gate before provisioning | covered | not-run |

### Not run

- Live during provisioning: no verification run names this row
- Live while Qdrant is down after start: no verification run names this row
- Not ready during provisioning: no verification run names this row
- Not ready before the first background check: no verification run names this row
- Ready after provisioning: no verification run names this row
- Ready again after Qdrant recovers: no verification run names this row
- Many probes, no Qdrant traffic: no verification run names this row
- Default render: no verification run names this row
- Attempt runs out of its own budget: asserts retry only, not the warn log entry
- Budget runs out: no verification run names this row
- Error that is not transient: no verification run names this row
- Three failures then success: no verification run names this row
- Smallest positive value: no verification run names this row
- Request during provisioning: no verification run names this row
- Cross-origin request during provisioning: no verification run names this row
- Request after provisioning: no verification run names this row
- Quantization rejected by Qdrant: no verification run names this row
- Signal during an attempt: no verification run names this row
- Signal during schema convergence: no verification run names this row
- Signal during the backoff wait: no verification run names this row
- Schema budget exceeded: no verification run names this row
- Startup timeout shorter than one attempt: no verification run names this row
- Raised schema timeout: no verification run names this row
- Raised startup timeout: no verification run names this row
- Invalid startup timeout value: no verification run names this row
- Root MCP path has no catch-all on the second listener: matched by name only; not read
- Console path during provisioning: no verification run names this row

## Verification runs

- exit 0: `go test ./cmd/engram -count=1 -run '^(TestPropertyReadiness|TestReadinessInitialPing|TestStartServeProvisioningError|TestStartupGate|TestPropertyStartupGate|TestResolveMCPPath)$'`
- exit 0: `go test ./internal/config -count=1 -run '^(TestQdrantProvisioningDefaultsAndEnv|TestValidateFieldRules|TestValidatePositiveDurationText|TestValidateStartupTimeoutAccepted|TestPropertyValidatePositiveDuration)$'`
- exit 0: `go test ./internal/server -count=1 -run '^(TestProvisionStoreBudgets|TestPropertyServeProvisionRetryLog|TestServeProvisionRetry|TestStoreFromEnvSingleAttempt)$'`
- exit 0: `go test ./internal/e2e -count=1 -run '^(TestMCPOnlyListener|TestMCPOnlyListenerWithConsole|TestServeSignalDuringStartup|TestServeStartupTimeout|TestServeWaitsForQdrant)$'`
- exit 0: `task chart:validate`

## Dropped

None.

## Lens notes

- approach: Every requirement of the three spec deltas has a producing decision and code at head: startupGate and healthLoop in cmd/engram/startupgate.go, retryConnectAndCreate in internal/server/startup.go, the ENGRAM_QDRANT_STARTUP_TIMEOUT key and ParsePositiveDuration in internal/config, the positiveSeconds helper and httpGet probes in the chart.
- approach: Library facts cited in D3 hold at the pinned versions: backoff/v7@v7.0.3 refuses a wait longer than the time left, does not call Notify for the final attempt, and returns *RetryError with LastErr and Cause; qdrant/go-client@v1.20.1 has VersionCheckTimeout with a 1 minute default and a pool size of 3.
- approach: The D6 test names were matched by name only: every named test function exists at head.
- approach: app-3 rests on singleCreate's wrapper being today's serve text; the prior text at main was not read, because this lens has no git access.
- approach: healthLoop also logs an info line on recovery, which D4 does not name; it is additive and not reported as a deviation.
- fit: openspec/config.yaml context has one library-preference row: prefer an established OSS or idiomatic solution over a hand-rolled one. The retry uses cenkalti/backoff/v7 through the existing newRetryBackOff, and store.IsUnavailable replaces the CLI classifier's copy rather than adding one; both fit.
- fit: newRetryBackOff is now shared by the summary queue and the serve startup retry but still lives in summaryqueue.go.
- fit: Design D2 says startServe returns the gate targets; at head startServe returns the drain and the register closure (serveSetup.provision) sets the targets through markProvisioned. Left to the controller as a design-to-code drift rather than a fit finding.
- fit: fit-5 is classed third-copy on the reading that the two serve_test.go checks predate the change; if both were added by this change, the kind is change-internal at should-fix.
- fit: The chart helper engram.positiveSeconds and the chart:validate loops are parameterized over both timeouts; no copy found. ENGRAM_QDRANT_STARTUP_TIMEOUT appears in the same file set as ENGRAM_QDRANT_SCHEMA_TIMEOUT.
- invariants: I1: ENGRAM_QDRANT_STARTUP_TIMEOUT and ENGRAM_QDRANT_SCHEMA_TIMEOUT are always > 0 \| enforced at internal/config/validate.go ParsePositiveDuration, shared with runtime parse in internal/server/tools.go \| boundary test TestPropertyValidatePositiveDuration (any int64), TestValidateStartupTimeoutAccepted (1ns), TestValidateFieldRules (0, -1s, abc)
- invariants: I2: No MCP, Connect or console handler runs before every gate target is set \| enforced at cmd/engram/startupgate.go ServeHTTP nil-target branch \| boundary test TestStartupGate, TestPropertyStartupGate
- invariants: I3: A cross-origin-refused request gets 403 in every state \| enforced at newHTTPServer wrapping the gate in newCrossOriginProtection \| boundary test TestStartupGate before and after setTarget, TestPropertyStartupGate
- invariants: I4: /readyz is 200 only when provisioned and last ping ok \| enforced at startupGate.ServeHTTP provisioned() && health.ok \| boundary test TestPropertyReadiness, TestReadinessInitialPing (initial ping held open)
- invariants: I5: /readyz never calls Qdrant; pings == ticks + 1 \| enforced at healthLoop.run \| boundary test TestPropertyReadiness ping-count invariant
- invariants: I6: Every gate target is set before the health loop starts \| enforced at markProvisioned \| boundary test TestMarkProvisioned
- invariants: I7: Each failed attempt logs a numbered warn, the last included \| enforced at retryConnectAndCreate closure \| boundary test TestPropertyServeProvisionRetryLog (0 to 6 transient, four outcomes)
- invariants: I8: Longest serve backoff wait <= 7.5 s \| enforced at newServeBackOff (5 s MaxInterval, factor 0.5) \| boundary test TestPropertyServeBackOffBound
- invariants: I9: No attempt runs past the startup timeout \| enforced at context.WithTimeout(parent, collectionCreateBudget) \| boundary test TestServeProvisionRetry blocked-attempt case, e2e TestServeStartupTimeout 2s
- invariants: I10: Per-attempt budget 15 s \| enforced at startup.go:86 \| boundary test none (inv-6)
- invariants: I11: versionCheckTimeout = min(timeout, 15 s) \| enforced at startup.go:53 \| boundary test none (inv-5)
- invariants: I12: Signal during startup exits 0 \| enforced at startupResult and retryConnectAndCreate sigCtx check \| boundary test TestStartupResult, TestServeProvisionRetry signal cases, e2e TestServeSignalDuringStartup; schema-convergence branch none (inv-3)
- invariants: I13: Non-serve verbs make one create attempt \| enforced at ensureStoreFromConfig !policy.retry() branch with singleCreate \| boundary test TestStoreFromEnvSingleAttempt
- invariants: I14: Chart timeouts are whole seconds in [1, 9223372036] \| enforced at _helpers.tpl engram.positiveSeconds \| boundary test chart:validate (1, 9223372036, 0, -1, 0.5, 1.9, true, 9223372037 via --set-json)
- invariants: I15: ENGRAM_MCP_PATH never names /livez or /readyz \| enforced at reservedMCPRoots in cmd/engram/mcproute.go \| boundary test TestResolveMCPPath exact-root cases; the /livez/x prefix case and a /livezz lookalike are not tested
- invariants: I16: create attempts and post-startup requests are not wait-for-ready \| enforced at parent derived from the unmarked ctx before store.WithWaitForReady in ensureStoreFromConfig \| boundary test TestWaitForReadyInterceptor (marked vs unmarked)
- invariants: Probe: helm template with a values file for startupTimeoutSeconds 9223372036 renders 9223372036s; 9223372037, "60" (string), null and 9223372036.4 fail naming the path; 1e3 renders 1000s and 1.0 renders 1s. --set 0 and --set 1.5 fail. Inputs are under probes/ in the handoff directory.
- invariants: Probe: backoff v7.0.3 ExponentialBackOff never returns Stop, so in production the retry only stops on a permanent error, the parent deadline, ErrMaxElapsedTime or a signal.
- invariants: Read internal/e2e/harness_test.go waitReady: it polls /readyz for 200 under a 45 s deadline and fails fast if the process exits, then checks the MCP-only listener is not 503.
- invariants: Not weakened: newRetryBackOff keeps the summary queue at backoff.DefaultInitialInterval and summaryQueueMaxInterval, the same values as before; store.IsUnavailable has the same body as the removed isUnavailableTransportErr.
- test-coverage: Task 7.1 integration check is unchecked in tasks.md.
- test-coverage: The BREAKING removal has a migration note and the gate/readyz scenarios cover it.
- test-honesty: No new unconditional t.Skip found in the cmd/engram test files grepped; internal/e2e TestServeWaitsForQdrant uses the existing SkipOrFailNoQdrant helper.
- test-coverage gap: Test bodies for e2e mcp_listener and mcproute not read in full; chart:validate shell not read beyond grep.
- test-honesty gap: Not read: TestServeBothListeners, config tests, store tests, chart:validate Taskfile gate, provision_test, harness_test, mcp_listener_test, operror/mcproute tests.

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
