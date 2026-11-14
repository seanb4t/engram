---
verdict: approve-with-notes
phase: plan
reviewed_at: 2026-11-14T19:23:47+00:00
head: 33dd9c763325381e94d2bb9d09875dd9cd713307
lenses:
  spec-lint: ok
  approach: ok
  fit: partial
  test-coverage: ok
  test-honesty: not-applicable
  invariants: ok
---
# Review

## Incomplete lenses

- fit: partial, gaps: 1

The verdict covers only what the lenses checked.

## Findings

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

## Coverage matrix

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

## Verification runs

None.

## Dropped

- spe-2 (spec-lint, confidence 0.6): THEN step 'startup completes without a process restart' names no observable such as a restart count, an exit code or a /readyz status.

## Lens notes

- approach: I checked these citations at head and they hold: the runServe order and the bind after server.Register (cmd/engram/serve.go), newHTTPServer wrapping the mux in newCrossOriginProtection, reservedMCPRoots and resolveMCPPath, the TCP-connect readiness wait in internal/e2e startServer, the 615 s startup-window check and the containerEnv checksum in Taskfile.yaml, the tcpSocket probes and the int coercion of schemaTimeoutSeconds in the chart, collectionCreateBudget, provisionStore, storeFromConfig, ensureStoreFromConfig and its CLI callers, NewQdrantClient as the only qdrant.NewClient caller, isUnavailableTransportErr and classifyOperatorErrConstruction, and newRetryBackOff in summaryqueue.go.
- approach: The dependency behavior the design relies on holds. In backoff v7.0.3, Notify is not called for the final error, the elapsed-time limit refuses a wait longer than the time left, and a permanent error comes back as a *RetryError with LastErr set. The defaults are a 500 ms initial interval and a 0.5 randomization factor. In qdrant go-client v1.20.1, the version check runs during NewGrpcClient with a 1 minute default VersionCheckTimeout and no context, and a failed check only logs a warning. RetryConfig is bounded by MaxRetries. In grpc v1.85.0, WithConnectParams replaces both the backoff config and minConnectTimeout, DefaultConfig.MaxDelay is 120 s and the default minConnectTimeout is 20 s.
- approach: Not a finding, for the controller: D2 says a step 3 or step 4 failure shuts the listeners down 'through the wait-and-shutdown step'. At head, serveAll's wait blocks until a serve error or the signal context. The design does not say how the startup-error path skips that wait, for example by handing it an already-done context. Without that, the scenario 'Quantization rejected by Qdrant' (exit non-zero) would hang until a signal. TestStartServeProvisioningError checks only that the error comes back unchanged.
- approach: Not verified: the 15 s budget of the bearer-chain OIDC discovery in buildAuthChain. I read the 15 s budget of the web UI discovery in serve.go only.
- approach: The halfway envelope was not written at the halfway turn. This final envelope replaces it.
- fit: openspec/config.yaml context has no library-preference rows; only the generic rows and its 'Prefer an established OSS or idiomatic solution' line apply.
- fit: D3 reuses cenkalti/backoff/v7 through newRetryBackOff, as internal/server/summaryqueue.go already does; no bespoke retry loop.
- fit: D3 moves isUnavailableTransportErr (cmd/engram/operror.go:41) to store.IsUnavailable, so the CLI classifier and the retry share one predicate; the gRPC DeadlineExceeded rule stays local to the retry with a stated reason.
- fit: D3 passes the connect parameters through the existing opts ...grpc.DialOption of store.NewQdrantClient (internal/store/store.go:550), keeping one qdrant.NewClient site.
- fit: D4 departs from the span-and-RecordStoreOp pattern of sibling Store methods (internal/store/provision.go:66) and names the reason; no existing Store ping or health method exists to reuse.
- fit: D5's ParsePositiveDuration and validatePositiveDuration mirror the existing ParsePositiveIntCap and validatePositiveCap pair (internal/config/validate.go:520-534).
- fit: D5's chart probes follow the httpGet /livez and /readyz shape the chart already uses for Qdrant (charts/engram/templates/qdrant.yaml:64-68).
- fit: The repository has no existing HTTP health library or swappable-handler helper; go.mod lists none.
- invariants: I1: ENGRAM_QDRANT_STARTUP_TIMEOUT is always a positive Go duration \| enforced at planned validatePositiveDuration in Config.Validate (internal/config/validate.go) \| boundary test TestValidateFieldRules cases 1ns, 0, -1s, abc and TestPropertyValidatePositiveDuration (int64 round trip probed sound, MinInt64 to MaxInt64)
- invariants: I2: runtime and validation parse both Qdrant budgets with one parser \| enforced at planned config.ParsePositiveDuration (today tools.go:282 uses time.ParseDuration) \| boundary test TestValidatePositiveDurationText
- invariants: I3: no connect-and-create attempt runs past the startup timeout \| enforced at planned per-attempt context.WithTimeout(parent, 15s) plus version-check bound min(startup, 15s) \| boundary test TestServeProvisionRetry 50 ms budget and TestServeStartupTimeout (2s, accepting listener)
- invariants: I4: only serve retries; every other verb makes one attempt \| enforced at the zero startupPolicy \| boundary test TestStoreFromEnvSingleAttempt
- invariants: I5: budget exhaustion always exits 5 \| enforced at rule 2 status.Error(codes.Unavailable) plus classifyOperatorErrConstruction (operror.go:184) \| boundary test TestServeProvisionRetry exhaustion cases, including ErrMaxElapsedTime while the parent is live
- invariants: I6: a signal during startup always exits 0 \| enforced at planned startupResult(sigCtx, err) \| boundary test TestStartupResult, TestStartServeSignal, TestServeSignalDuringStartup. OIDC discovery takes no ctx, so a signal during it waits up to its 15 s budget before exit 0
- invariants: I7: no MCP, Connect or console handler runs before every gate target is set \| enforced at the planned gate atomic.Pointer \| boundary test TestStartupGate and TestPropertyStartupGate
- invariants: I8: a cross-origin refused request gets 403 in every state \| enforced at newCrossOriginProtection wrapping the gate (csrf.go:26) \| boundary test TestStartupGate cross-site POST before and after
- invariants: I9: primary /readyz 200 implies every gate target is set \| enforced at planned markProvisioned order \| boundary test TestMarkProvisioned
- invariants: I10: /readyz is 200 only when the target is set and the last finished ping succeeded \| enforced at the atomic.Bool starting false \| boundary test TestPropertyReadiness. The initial-ping edge is missing, see inv-6
- invariants: I11: /readyz never calls Qdrant \| enforced structurally, since the loop owns the only ping \| boundary test TestPropertyReadiness ping count = ticks + 1
- invariants: I12: the longest backoff wait is 7.5 s \| enforced at newRetryBackOff(500ms, 5s) with default randomization 0.5 (backoff v7.0.3 exponential.go). No boundary test, see inv-5
- invariants: I13: chart timeouts render only as positive whole seconds \| enforced at planned engram.positiveSeconds \| boundary test chart:validate 1, 0, -1, 0.5, 1.9. Probes under inv-probe/ show that true and 9223372036854775807 pass, see inv-3 and inv-4. Today's int coercion was probed too: 1.9 renders 1s and 0.5 fails, which confirms D5
- invariants: I14: /livez and /readyz cannot be ENGRAM_MCP_PATH \| enforced at reservedMCPRoots prefix loop (mcproute.go:46) \| boundary test planned resolveMCPPath case
- invariants: Backoff v7 Retry returns ErrExhausted when an injected BackOff returns Stop. D3 rule 3 would then return a transient LastErr as if it were not transient. The production ExponentialBackOff never returns Stop, so only injected test BackOffs can reach this.
- invariants: Backoff v7 Retry always runs the operation once, even under a done context (retry.go:131-133). So with a 1ns startup timeout, serve makes one logged attempt and then exits 5.
- invariants: D4 says the health goroutine stops when the signal context ends. No D6 row tests that stop.
- invariants: Probe inputs were written under the handoff directory, in inv-probe/ (chart/, dur/, vals.yaml).
- spec-lint: All Taskfile targets named in tasks.md exist: task, lint:markdown, test:e2e, chart:validate.
- spec-lint: BREAKING is present in the proposal and in the REMOVED requirement of collection-provisioning, so the BREAKING check passes.
- spec-lint: Every ADDED, MODIFIED and REMOVED requirement has at least one scenario with WHEN or GIVEN and THEN lines.
- spec-lint: Every requirement heading shares two or more distinctive words with a task line; no untasked-requirement finding.
- spec-lint: Every tracked task names a command, test, file or observable; no unverifiable-task finding.
- spec-lint: design.md Open Questions reads None, so no assumed-answer finding.
- test-coverage: Two scenarios are both named 'Default render' and 'Invalid startup timeout' in different capabilities; each is graded once per heading text.
- fit gap: Issue #684, the source of the remaining telemetry work, is not in the repository and this lens has no GitHub access, so forward fit against it is not judged.
