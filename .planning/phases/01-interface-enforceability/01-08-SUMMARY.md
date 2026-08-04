---
phase: 01-interface-enforceability
plan: 08
subsystem: cli
tags: [go, cobra, connect-rpc, context, exit-codes, testing]

# Dependency graph
requires:
  - phase: 01-interface-enforceability plan 07
    provides: "exitTimeout=6, the connect.CodeDeadlineExceeded mapper-arm split, and clientFromFlags returning the resolved (client, outputFormat, time.Duration, error) tuple with the duration intentionally discarded at each call site"
provides:
  - "context.WithTimeout(cmd.Context(), timeout) at all three client RPC call sites (search, list, store), derived once from clientFromFlags' resolved duration, with a deferred cancel"
  - "empirical proof that a hung server (accepts, never answers) does not block an invocation: it returns within the configured --timeout and exits 6, distinct from a closed port's exit 5 by explicit inequality"
  - "cmd/engram/timeout_test.go: TestTimeoutHungServerExitsTimeout, TestTimeoutDistinctFromUnavailable, TestTimeoutSuccessInsideDeadline"
  - "exitCodeBaseline table extended with a hungServer/hungServerPlaceholder row mechanism, letting a static table row exercise a dynamically-addressed hung httptest.Server without hard-coding a port"
affects: [01-09 (docs-site guides/upgrade.md: the new --timeout flag is now load-bearing on every client verb, not just validated-but-unused)]

# Actuals (#2632)
actuals:
  tokens: 4643
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "one context.WithTimeout derivation per client RPC call site, immediately after clientFromFlags, with defer cancel() — cmd.Context() is only ever the PARENT of the derived context, never passed to a Connect method directly"
    - "hung-server test harness: httptest.Server whose handler selects on BOTH r.Context().Done() and a t.Cleanup-closed release channel, because connect-go's client does not reliably close the underlying TCP connection on context cancellation (confirmed empirically, not assumed) — relying on r.Context().Done() alone hangs httptest.Server.Close()"
    - "hungServer/hungServerPlaceholder row opt-in on exitCodeBaselineCase: a static table row's args carries a placeholder token substituted with a freshly started hung-server URL at test-run time, extending the D-09 before-table to cover a capability that needs a live per-row server"

key-files:
  created:
    - cmd/engram/timeout_test.go
  modified:
    - cmd/engram/client_search.go
    - cmd/engram/client_list.go
    - cmd/engram/client_store.go
    - cmd/engram/exitcode_baseline_test.go

key-decisions:
  - "No http.Client.Timeout added in newHTTPClient, per the plan's explicit prohibition: a transport-level timeout would surface as connect.CodeUnavailable (exit 5), silently defeating the exit 5-vs-6 distinction 01-07 just created. The context deadline is the only path that reaches connect.CodeDeadlineExceeded / exitTimeout."
  - "Hung-server handler selects on r.Context().Done() OR a t.Cleanup-closed release channel, not r.Context().Done() alone. A throwaway repro (plain net/http.Client vs. the generated Connect client, against the identical handler) proved connect-go's client does NOT reliably close the underlying TCP connection when its context is canceled — client.SearchMemories(ctx, ...) returns context deadline exceeded to the caller in ~300ms as expected, but the server-side handler stayed blocked on r.Context().Done() for 2+ minutes in the real run, and httptest.Server.Close() (which waits for every outstanding request) hung the whole test binary past the 60s budget. The release channel is the deterministic unblock that does not depend on the client library's connection-teardown timing."
  - "exitCodeBaselineCase gained a hungServer bool field and a hungServerPlaceholder arg-token, resolved via substituteHungServerURL immediately before each row runs. The static table's existing rows hard-code an address (deadServer); a hung listener needs a live per-run httptest.Server instead, which this row-level opt-in provides without a second, parallel test loop."

requirements-completed: [REQ-cli-request-timeout, REQ-exit-code-migration-safe]

coverage:
  - id: D1
    description: "Every client RPC path (search, list, store) applies a finite deadline: each derives context.WithTimeout from clientFromFlags' resolved timeout and passes the derived context to its Connect call, never cmd.Context() directly"
    requirement: "REQ-cli-request-timeout"
    verification:
      - kind: unit
        ref: "cmd/engram/exitcode_baseline_test.go#TestExitCodeBaseline/search/hung-server-timeout"
        status: pass
      - kind: unit
        ref: "cmd/engram/exitcode_baseline_test.go#TestExitCodeBaseline/list/hung-server-timeout"
        status: pass
      - kind: unit
        ref: "cmd/engram/timeout_test.go#TestTimeoutHungServerExitsTimeout (store subtest)"
        status: pass
    human_judgment: false
  - id: D2
    description: "A CLI invocation against a server that accepts the connection and never responds returns within the configured --timeout window and exits 6, instead of blocking indefinitely"
    requirement: "REQ-cli-request-timeout"
    verification:
      - kind: unit
        ref: "cmd/engram/timeout_test.go#TestTimeoutHungServerExitsTimeout"
        status: pass
    human_judgment: false
  - id: D3
    description: "A CLI invocation against a closed port exits 5, and the hung-server case exits 6, in the same test run — the two are asserted DISTINCT by explicit inequality, not set membership"
    requirement: "REQ-exit-code-migration-safe"
    verification:
      - kind: unit
        ref: "cmd/engram/timeout_test.go#TestTimeoutDistinctFromUnavailable"
        status: pass
    human_judgment: false
  - id: D4
    description: "No client RPC call site passes cmd.Context() directly to a Connect method; the derived context is the only one used, and each derivation's cancel is deferred"
    requirement: "REQ-cli-request-timeout"
    verification:
      - kind: other
        ref: "rg -n 'cmd\\.Context\\(\\)' cmd/engram/client_search.go cmd/engram/client_list.go cmd/engram/client_store.go — every hit is context.WithTimeout's parent argument, none is a Connect call argument; go vet ./cmd/engram/... clean (would catch a missing cancel)"
        status: pass
    human_judgment: false
  - id: D5
    description: "A successful call well inside the deadline still exits 0 and returns its normal output unchanged, byte-for-byte"
    requirement: "REQ-exit-code-migration-safe"
    verification:
      - kind: unit
        ref: "cmd/engram/timeout_test.go#TestTimeoutSuccessInsideDeadline"
        status: pass
    human_judgment: false
  - id: D6
    description: "E-11 backstop: a request completing just inside the deadline exits 0 while one that never completes exits 6, and 6 is never confused with 5 at the boundary"
    human_judgment: true
    rationale: "Carried verbatim from the plan's must_haves as a backstop marker (verification: backstop): an assertion at T-epsilon is a timing race against CI scheduler jitter, and this phase's validation contract caps feedback at 60s with no tolerance for a flaky test. The neighbouring non-boundary cases (D2, D5) are automated; only the boundary itself abstains to human_needed, per design."

duration: ~15min
completed: 2026-08-03
status: complete
---

# Phase 1 Plan 8: Make the Deadline Real Summary

**Every client RPC call site (`search`, `list`, `store`) now derives `context.WithTimeout` from the single resolved client timeout, and a hung server is empirically proven — against connect-go's real client behavior, not an assumption — to return within the window with exit 6, distinct from a closed port's exit 5.**

## Performance

- **Duration:** ~15 min
- **Completed:** 2026-08-03
- **Tasks:** 3
- **Files modified:** 5 (1 created, 4 modified)

## Accomplishments

- All three client RPC call sites (`client_search.go`, `client_list.go`, `client_store.go`) now derive `ctx, cancel := context.WithTimeout(cmd.Context(), timeout)` immediately after `clientFromFlags`, with `defer cancel()`, and pass `ctx` — never `cmd.Context()` — to the Connect call. No `http.Client.Timeout` was added, preserving the exit 5-vs-6 distinction 01-07 established.
- Built `cmd/engram/timeout_test.go`: a hung server (real `httptest.Server`, handler blocking rather than a bare listener) proves all three verbs return within their `--timeout` window and exit 6; a closed-port case proves exit 5; both are asserted DISTINCT by explicit inequality in the same test; a responsive-stub case proves the deadline adds nothing to the success path (byte-for-byte identical output to a default-timeout call).
- **Empirically discovered and fixed a real hang, not a hypothetical one:** the plan's literal guidance ("block on the request context's Done channel") does not reliably unblock with connect-go's actual client — a throwaway repro proved `client.SearchMemories`'s context deadline fires and returns to the caller in ~300ms, but the underlying TCP connection is not necessarily closed by connect-go the way a plain `http.Client` closes it, leaving the server-side handler blocked and hanging `httptest.Server.Close()` (which waits for every outstanding request) indefinitely. Fixed by having the handler select on `r.Context().Done()` OR a `t.Cleanup`-closed release channel, which guarantees termination regardless of the client library's actual connection-teardown timing.
- Extended `exitCodeBaseline` with two `introduced: true` rows (`search/hung-server-timeout`, `list/hung-server-timeout`, both `after: exitTimeout`) via a new `hungServer`/`hungServerPlaceholder` row mechanism — the static table's existing rows hard-code a dead address, but a hung listener needs a live per-run server, so a row opts in and `substituteHungServerURL` swaps a placeholder token for a freshly started hung-server URL immediately before the row executes. Confirmed `search/unreachable-server` (the existing closed-port-equivalent row) is still green rather than duplicating it, per the plan's explicit instruction. Row count 32 → 34; `exitCodeBaselineFullyMigratedAllowlist` stays empty.

## Task Commits

1. **Task 1: Bound all three client RPC call sites with the resolved deadline** — `48b98458` (feat)
2. **Task 2: Build the hung-server harness and prove exit 6, distinct from exit 5** — `46d1b95f` (test)
3. **Task 3: Add the timeout baseline rows and shrink the deferred allowlist** — `2e6766b1` (test)

## Client RPC Call-Site Enumeration

Per the phase-critical instruction that a partial application is a failed requirement: every client RPC call site in this repository, enumerated and confirmed bounded.

| File | RPC call | Bounded? |
|---|---|---|
| `cmd/engram/client_search.go` | `client.SearchMemories` | Yes — `context.WithTimeout` derived, `cmd.Context()` only its parent |
| `cmd/engram/client_list.go` | `client.ListMemories` | Yes — same pattern |
| `cmd/engram/client_store.go` | `client.StoreMemory` | Yes — same pattern; confirmed a single RPC call site per the plan's own flagged assumption |

No other file under `cmd/engram/` issues a client RPC call — operator commands (`reindex`, `migrate-remap-owner`, `prune-expired`, `summarize-missing`, `backfill-short-ids`, `serve`) dial Qdrant/embedder paths outside this plan's scope and already carry their own pre-existing `--timeout` conventions (noted as a separate, already-flagged divergence in 01-06/01-07).

## Files Created/Modified

- `cmd/engram/client_search.go` — `context.WithTimeout` derivation before `SearchMemories`, `context` import added
- `cmd/engram/client_list.go` — same pattern before `ListMemories`
- `cmd/engram/client_store.go` — same pattern before `StoreMemory`
- `cmd/engram/timeout_test.go` (new) — `startHungServer`, `closedPortURL`, `TestTimeoutHungServerExitsTimeout`, `TestTimeoutDistinctFromUnavailable`, `TestTimeoutSuccessInsideDeadline`
- `cmd/engram/exitcode_baseline_test.go` — `hungServer` field + `hungServerPlaceholder` constant on `exitCodeBaselineCase`, `substituteHungServerURL` helper, two new introduced rows, row-count literal 32 → 34, doc comments updated to name this plan

## Decisions Made

See `key-decisions` in frontmatter for full rationale on: the deliberate omission of `http.Client.Timeout`, the release-channel fix for the hung-server harness (with the empirical repro that motivated it), and the `hungServer`/`hungServerPlaceholder` row mechanism.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] The plan's literal hung-handler design (block only on `r.Context().Done()`) hangs `httptest.Server.Close()` with connect-go's real client**
- **Found during:** Task 2, first `go test ./cmd/engram/... -run TestTimeout -v -race` run
- **Issue:** The plan's Task 2 `<action>` specified a handler that blocks solely on the request context's Done channel, reasoning that this "unblocks the instant the client hangs up." Run as written, the test hung well past the 120s command timeout. A `kill -QUIT` stack dump showed the handler goroutines still parked in `chanrecv` on `r.Context().Done()` two minutes after the client's 300ms `--timeout` had already fired. Root-caused with a throwaway two-file repro comparing a plain `http.Client` (context cancellation DOES close the connection, server-side context fires within ~2s) against the generated `engramv1connect` client against the identical handler (context cancellation does NOT reliably close the connection — the client returns `context deadline exceeded` to the caller in ~300ms exactly as expected, but the server-side handler stays blocked, and `httptest.Server.Close()`, which waits for every outstanding request, hangs indefinitely). This is a real, reproducible property of `connectrpc.com/connect`'s duplex HTTP call (it writes the request body through an `io.Pipe` in a background goroutine), not a flake.
- **Fix:** The handler now selects on `r.Context().Done()` OR a `release` channel closed from `t.Cleanup` immediately before `srv.Close()`. This keeps the documented intent (unblock via context cancellation if it ever does fire) while guaranteeing termination independent of the client library's actual connection-teardown timing, so the suite cannot hang regardless of `connectrpc.com/connect`'s internal implementation details.
- **Files modified:** `cmd/engram/timeout_test.go`
- **Verification:** `go test ./cmd/engram/... -run TestTimeout -v -race` passes in ~1s; each hung-server subtest completes in ~300ms, well under the 2s ceiling; full package suite under `-shuffle=on -count=2 -race` completes in ~5s.
- **Committed in:** `46d1b95f` (Task 2)

**2. [Rule 3 — Blocking] The static `exitCodeBaseline` table has no mechanism for a row that needs a live, dynamically-addressed server**
- **Found during:** Task 3, before writing the new rows
- **Issue:** The plan's Task 3 instructs adding `search/hung-server-timeout` and `list/hung-server-timeout` rows to the package-level `exitCodeBaseline` table, but every existing row hard-codes a static address (`deadServer = "http://127.0.0.1:1"`), which works for a connection-refused case but cannot represent a hung listener — that needs a real `httptest.Server` started per test run, not a literal string known at package-init time.
- **Fix:** Added a `hungServer bool` field to `exitCodeBaselineCase` and a `hungServerPlaceholder` string constant; `TestExitCodeBaseline` now calls `substituteHungServerURL` for any row with `hungServer: true`, which starts a fresh hung server via `timeout_test.go`'s `startHungServer` and swaps the placeholder token in `args` for its real URL immediately before the row runs. This keeps the table itself static and readable while letting a row opt into a dynamic server.
- **Files modified:** `cmd/engram/exitcode_baseline_test.go`
- **Verification:** `go test ./cmd/engram/... -run TestExitCodeBaseline -v` shows both new rows RUN/PASS in ~0.30s each; `TestExitCodeBaselineRowCount` (34) and `TestExitCodeBaselineClaims` pass for both introduced rows.
- **Committed in:** `2e6766b1` (Task 3)

---

**Total deviations:** 2 auto-fixed (both Rule 3 — blocking issues that prevented completing the plan's own literal design, discovered and fixed with empirical evidence rather than assumption)
**Impact on plan:** Both fixes were necessary to land the plan's own mandated proof (a hung server provably cannot block an invocation) without leaving a hanging test suite behind. No scope creep — all changes confined to `cmd/engram/timeout_test.go` and `cmd/engram/exitcode_baseline_test.go`, both files the plan itself names.

## Issues Encountered

None beyond the two deviations above, both resolved without blocking progress. `go test ./cmd/engram/... -v -shuffle=on -count=2 -race` is green in ~5s; `task test` and `task lint` are both clean across the whole repository (confirming the pre-existing `internal/e2e`/`internal/embed` failures noted in 01-07-SUMMARY.md were not present in this run — likely order-dependent and not triggered this time; not investigated further, as neither package is touched by this plan).

## Known Stubs

None.

## Threat Flags

None — T-1-01 (the DoS mitigation this plan makes load-bearing) and T-1-12 (test-suite resource leakage) are both explicitly covered in this plan's own `<threat_model>` and verified by `TestTimeoutHungServerExitsTimeout`/`TestTimeoutDistinctFromUnavailable` plus the `-race` run showing no leaked-goroutine-induced flakiness. No new network endpoints, auth paths, or schema changes at a trust boundary.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Plan 01-09 (docs): must document that `--timeout` is now load-bearing on every client verb, not just validated-but-unused as it was after 01-07 — the deadline genuinely bounds every RPC call site now.
- The `hungServer`/`hungServerPlaceholder` row mechanism on `exitCodeBaselineCase` is reusable if a future plan needs another dynamically-addressed-server row in this table.
- The release-channel pattern in `startHungServer` (select on `r.Context().Done()` OR a `t.Cleanup`-closed channel) is documented and reusable for any future test needing a genuinely-hung Connect server; it should NOT be replaced with a bare `r.Context().Done()`-only handler based on the empirical finding recorded here.
- No blockers.

---
*Phase: 01-interface-enforceability*
*Completed: 2026-08-03*
