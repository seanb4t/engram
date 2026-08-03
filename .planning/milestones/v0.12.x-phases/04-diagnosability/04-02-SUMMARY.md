---
phase: 04-diagnosability
plan: 02
subsystem: auth
tags: [go, cedar, authz, slog, structured-logging, diagnostics]

requires:
  - phase: 04-diagnosability
    provides: "D-01/D-02/D-03/D-04 decisions from 04-CONTEXT.md governing the Cedar decision-diagnostics accessor and log line"
provides:
  - "authz.DecisionLog and (Decision).Log() — the narrow D-02 allowlist accessor: satisfied policy IDs, an error count, and the decision; Decision.diag stays unexported (D-03), Log() is the only read path"
  - "authz.Bucket.String() — a readable 'own'/'shared' token instead of a raw int, used at the store's log call sites"
  - "internal/store's first logging statement: one unconditional slog.DebugContext call at each of decideBucket/decideRecord, both arms, D-04"
  - "context.Context threaded through decideBucket/decideRecord and the four filter-builder helpers between them and the ctx-carrying store methods"
affects: []

actuals:
  tokens: 7100
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "authz.Decision.Log() — an unexported-field guarded by a narrow exported accessor, the structural (not call-site-discipline) enforcement of a field allowlist (D-03)"
    - "Chokepoint logging: emit once at the two functions every production Decision consumption funnels through (decideBucket/decideRecord), never at call sites, so coverage is total and cardinality is O(1) per request rather than O(result count)"

key-files:
  created:
    - internal/store/decisionlog_test.go
  modified:
    - internal/authz/authz.go
    - internal/authz/authz_test.go
    - internal/store/store.go
    - internal/store/store_test.go
    - internal/store/bench_test.go

key-decisions:
  - "D-01/D-04 confirmed as researched: decideBucket/decideRecord (store.go) are the two chokepoints every production Decision consumption funnels through — verified by reading every call site (ownerOrSharedCondition x2, ownerOnlyCondition, and the three id-addressed gates) before threading ctx, not assumed from the plan's citation alone."
  - "The DecisionLog allowlist ships exactly as specified: Allow, PolicyIDs ([]string from diag.Reasons), ErrorCount (int, len(diag.Errors)) — no Message, no Position, no raw cedar.Diagnostic."
  - "Bucket.String() added to internal/authz/authz.go (not mapped locally in store.go) since it's a property of the Bucket type itself and the plan explicitly permitted either location."
  - "TestDecisionLogNeverLeaksExpressionTrace's error-carrying case lives in internal/authz/authz_test.go, not internal/store/decisionlog_test.go, per the plan's own fallback clause — see 'Route taken' below."

patterns-established:
  - "Unexported-field-plus-narrow-accessor as the mechanism for a structural (non-call-site-discipline) allowlist across a package boundary — reusable anywhere else in the codebase that surfaces a subset of an internal diagnostic type to a caller."

requirements-completed: [REQ-authz-decision-diagnostics]

coverage:
  - id: D1
    description: "At debug level, an operator debugging an ALLOW sees a log line naming the satisfied Cedar policy IDs, the decision, the action, and the bucket"
    requirement: "REQ-authz-decision-diagnostics"
    verification:
      - kind: unit
        ref: "internal/store/decisionlog_test.go#TestDecideBucketLogsAllowAndDeny/allow"
        status: pass
    human_judgment: false
  - id: D2
    description: "At debug level, an operator debugging a DENY sees the same field set as the allow arm — both arms unconditionally logged, never gated on Allow"
    requirement: "REQ-authz-decision-diagnostics"
    verification:
      - kind: unit
        ref: "internal/store/decisionlog_test.go#TestDecideBucketLogsAllowAndDeny/deny"
        status: pass
      - kind: unit
        ref: "internal/store/decisionlog_test.go#TestDecideRecordLogsBothArms"
        status: pass
    human_judgment: false
  - id: D3
    description: "No full Cedar expression trace and no DiagnosticError.Message text ever reaches a log line at any level, enforced structurally (diag stays unexported, Log() is the only read path)"
    requirement: "REQ-authz-decision-diagnostics"
    verification:
      - kind: unit
        ref: "internal/authz/authz_test.go#TestDecisionLogNeverLeaksExpressionTrace"
        status: pass
      - kind: unit
        ref: "internal/authz/authz_test.go#TestDecisionLogCarriesOnlyAllowlistedFields"
        status: pass
    human_judgment: false
  - id: D4
    description: "internal/authz still holds no logger dependency and emits zero slog calls"
    requirement: "REQ-authz-decision-diagnostics"
    verification:
      - kind: unit
        ref: "gate: rg -v '^\\s*//' -g '!*_test.go' internal/authz/ | rg -c 'log/slog|slog\\.' == 0"
        status: pass
    human_judgment: false
  - id: D5
    description: "Decision logging is O(1) per request (at most two calls for a bulk Search/List, one for an id-addressed op), never O(result count)"
    requirement: "REQ-authz-decision-diagnostics"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestSearchAuthzCallCount (pre-existing, unaffected by this plan's ctx threading — reconfirmed green)"
        status: pass
    human_judgment: false

duration: ~35min
completed: 2026-08-01
status: complete
---

# Phase 4 Plan 02: Cedar Decision Diagnostics Summary

**A debug-level `slog.DebugContext` line now fires unconditionally on both the allow and deny arm at `internal/store`'s two authz chokepoints, carrying only the D-02 allowlist (satisfied policy IDs, an error count, decision/action/bucket) via a narrow `(authz.Decision).Log()` accessor — `Decision.diag` stays unexported, so a future `cedar.Diagnostic` field structurally cannot leak through a well-meaning call site.**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-08-01T14:14:00-04:00 (approx)
- **Completed:** 2026-08-01T14:24:00-04:00
- **Tasks:** 3
- **Files modified:** 6 (1 created, 5 modified)

## Accomplishments

- `internal/authz/authz.go`: `DecisionLog{Allow bool; PolicyIDs []string; ErrorCount int}` and `(Decision).Log() DecisionLog` — builds the slice from `diag.Reasons`, the count from `len(diag.Errors)`, and deliberately never reads `DiagnosticError.Message` (can embed evaluated entity values) or `DiagnosticReason.Position` (policy-source location, not caller data but not useful to an operator). `Decision.diag` stays unexported; `Log()` is the only method that reads it. `Bucket.String()` renders `"own"`/`"shared"` instead of a raw int. `internal/authz` gained **zero** logging imports.
- `internal/store/store.go`: threaded `context.Context` (leading parameter) through `decideBucket`, `decideRecord`, `ownerOrSharedCondition`, `ownerOnlyCondition`, `ownerScopeFilter`, and `listFilter`, updating all 14 call sites in the package. Each of `decideBucket`/`decideRecord` now emits exactly one `slog.DebugContext` call, unconditionally (no `if allow` branch), with fields `allow`, `action`, `policy_ids`, `policy_error_count`, plus `bucket` on the bucket arm only. This is `internal/store`'s first logging statement (`log/slog` newly imported).
- `internal/store/decisionlog_test.go` (new): `TestDecideBucketLogsAllowAndDeny` and `TestDecideRecordLogsBothArms` — table-driven, both arms in one table, driving the real chokepoints directly against a `Store` built with `New(nil, ...)` (no live Qdrant required — `decideBucket`/`decideRecord` never touch `s.client`, and `s.authz` defaults to the real embedded four-policy corpus) and capturing debug-level JSON slog output via the repo's existing `internal/auth/auth_test.go:50-62` idiom. `TestDecisionLogFieldSetIsExact` pins each chokepoint's exact emitted key set as a store-side complement to the authz-side negative gate.
- `internal/authz/authz_test.go`: `TestDecisionLogCarriesOnlyAllowlistedFields` (field-name-set equality, not a bare count) and `TestDecisionLogNeverLeaksExpressionTrace` (the D-02 negative gate across allow/deny/multi-policy/error-carrying `Decision` shapes, asserting absence of a sentinel marker planted in a diagnostic error message).
- `internal/store/bench_test.go`, `internal/store/store_test.go`: the three direct (non-hook) call sites the ctx threading broke — two in `BenchmarkSearchFilter`, one in the Phase 3 cross-spine authz-composition pin (`store_test.go:4458`) — migrated to pass `context.Background()`.

## Route Taken: the error-carrying negative-gate case

The plan's action anticipated this and gave explicit discretion. Read the four embedded Cedar policies (`internal/authz/policies/*.cedar`) and `entities.go`'s entity builders: every attribute access is either `has`-guarded (`tenant_isolate`, `defense_empty_owner`'s `resource has owner`) or a plain `cedar.String` built from a Go `string` with no type mismatch possible. **No input reachable through the public `DecideBucket`/`DecideRecord` API against the shipped corpus can produce a Cedar evaluation error** — confirmed empirically too: every real `DecideBucket`/`DecideRecord` call exercised during this plan's own test-writing returned `ErrorCount == 0`.

Since `Decision.diag` is unexported by design (D-03), and Go does not let one package's `_test.go` file import symbols from another package's `_test.go` file, the "error-carrying" case of `TestDecisionLogNeverLeaksExpressionTrace` **cannot** be constructed from `internal/store`'s test file at all — only `internal/authz`'s own test file has the field access to build a `Decision` literal with a populated, marker-bearing `diag.Errors`. So the full four-case table (allow, deny, multi-policy, error-carrying) lives in `internal/authz/authz_test.go`, asserting on `Log()`'s output directly — the exact boundary D-03 exists to guard. `internal/store/decisionlog_test.go` complements this with `TestDecisionLogFieldSetIsExact`, which pins the actual emitted JSON key set at both chokepoints (structurally impossible to include `Message`, since `DecisionLog` has no such field), and the two arm tests, which prove the real store call sites emit real, non-degenerate policy IDs for allow/deny cases sourced from the actual policy corpus.

This deviates from the `must_haves.artifacts` line listing `TestDecisionLogNeverLeaksExpressionTrace` under `internal/store/decisionlog_test.go` — a planning-time expectation written before the reachability question was answered. The plan's own action text names this exact fallback ("add a test-only constructor in internal/authz's OWN test file and assert the negative gate there instead"), so this is not a deviation from instruction, only from the artifacts table's pre-execution guess. The verify gate (`go test ./internal/store/... ./internal/authz/... -run 'TestDecisionLogNeverLeaksExpressionTrace$' ...`) explicitly runs against both packages and only needs one `--- PASS:` match, which the authz-package placement satisfies.

## RED → GREEN Transcripts

### Arm assertion: `TestDecideRecordLogsBothArms`

Task 1 was already committed, so per the plan's explicit fallback the `decideRecord` emission was temporarily disabled (the `slog.DebugContext` call body replaced with a no-op), the test rerun, the failure recorded, then the file restored from a pre-edit backup and reverified green.

RED (emission disabled):
```
=== RUN   TestDecideRecordLogsBothArms
=== RUN   TestDecideRecordLogsBothArms/allow
    decisionlog_test.go:138: got 0 log lines, want exactly 1: []
=== RUN   TestDecideRecordLogsBothArms/deny
    decisionlog_test.go:138: got 0 log lines, want exactly 1: []
--- FAIL: TestDecideRecordLogsBothArms (0.00s)
    --- FAIL: TestDecideRecordLogsBothArms/allow (0.00s)
    --- FAIL: TestDecideRecordLogsBothArms/deny (0.00s)
FAIL
FAIL	github.com/seanb4t/engram/internal/store	1.204s
```

GREEN (emission restored):
```
=== RUN   TestDecideRecordLogsBothArms
--- PASS: TestDecideRecordLogsBothArms (0.00s)
    --- PASS: TestDecideRecordLogsBothArms/allow (0.00s)
    --- PASS: TestDecideRecordLogsBothArms/deny (0.00s)
PASS
ok  	github.com/seanb4t/engram/internal/store	1.073s
```

### Negative gate: `TestDecisionLogNeverLeaksExpressionTrace`

`Log()` was temporarily modified to leak (appending `e.Message` for each `diag.Errors` entry into the returned `PolicyIDs` slice — simulating exactly the regression class D-03 exists to prevent), then reverted from a pre-edit backup.

RED (leak injected):
```
=== RUN   TestDecisionLogNeverLeaksExpressionTrace
=== RUN   TestDecisionLogNeverLeaksExpressionTrace/allow
=== RUN   TestDecisionLogNeverLeaksExpressionTrace/deny
=== RUN   TestDecisionLogNeverLeaksExpressionTrace/multi-policy
=== RUN   TestDecisionLogNeverLeaksExpressionTrace/error-carrying
    authz_test.go:176: DecisionLog leaked the sentinel marker: {"Allow":false,"PolicyIDs":["tenant-isolate","attribute `SENTINEL-caller-entity-value-9f3c2a` not found on entity"],"ErrorCount":1}
--- FAIL: TestDecisionLogNeverLeaksExpressionTrace (0.00s)
    --- PASS: TestDecisionLogNeverLeaksExpressionTrace/allow (0.00s)
    --- PASS: TestDecisionLogNeverLeaksExpressionTrace/deny (0.00s)
    --- PASS: TestDecisionLogNeverLeaksExpressionTrace/multi-policy (0.00s)
    --- FAIL: TestDecisionLogNeverLeaksExpressionTrace/error-carrying (0.00s)
FAIL
FAIL	github.com/seanb4t/engram/internal/authz	0.161s
```

Note the allow/deny/multi-policy subtests correctly stayed green under the injected leak — they carry no diagnostic errors by construction, so there is nothing for the leaked code path to expose. Only the `error-carrying` subtest, the one case with a populated `diag.Errors`, caught the regression — confirming the table's four-case design is load-bearing, not redundant.

GREEN (`Log()` restored):
```
=== RUN   TestDecisionLogNeverLeaksExpressionTrace
--- PASS: TestDecisionLogNeverLeaksExpressionTrace (0.00s)
    --- PASS: TestDecisionLogNeverLeaksExpressionTrace/allow (0.00s)
    --- PASS: TestDecisionLogNeverLeaksExpressionTrace/deny (0.00s)
    --- PASS: TestDecisionLogNeverLeaksExpressionTrace/multi-policy (0.00s)
    --- PASS: TestDecisionLogNeverLeaksExpressionTrace/error-carrying (0.00s)
PASS
ok  	github.com/seanb4t/engram/internal/authz	0.124s
```

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): the accessor and one chokepoint** — `aa870647` (feat)
2. **Task 2: migrate the three broken test call sites** — `6004895a` (test)
3. **Task 3: prove both arms are logged AND the excluded half is unreachable** — `4e4276e0` (test)

Tasks 1 and 2 are compile-coupled by design (the plan's own critical-constraint framing: ctx threading in Task 1 breaks three direct test call sites that only Task 2 fixes), so both were applied to the working tree before either was verified or committed, then split into two separate atomic commits reflecting the production-vs-test-file boundary — `go vet ./...` and `go test ./internal/store/... ./internal/authz/...` were run and confirmed green only after both were applied, immediately before making the first of the two commits.

## Files Created/Modified

- `internal/authz/authz.go` — `DecisionLog`, `(Decision).Log()`, `Bucket.String()`
- `internal/authz/authz_test.go` — `TestDecisionLogCarriesOnlyAllowlistedFields`, `TestDecisionLogNeverLeaksExpressionTrace`
- `internal/store/store.go` — ctx threading through 6 functions/14 call sites; the two `slog.DebugContext` emission sites; new `log/slog` import
- `internal/store/store_test.go` — one call site (`:4458`) migrated to pass `context.Background()`
- `internal/store/bench_test.go` — two call sites migrated; `context` import added
- `internal/store/decisionlog_test.go` — new: `TestDecideBucketLogsAllowAndDeny`, `TestDecideRecordLogsBothArms`, `TestDecisionLogFieldSetIsExact`, plus the shared `captureDebugLog`/`decodeLogLines` test helpers

## Decisions Made

See `key-decisions` in the frontmatter. All decisions were pre-set by `04-CONTEXT.md` (D-01 through D-04); the only executor-level call was where to place the error-carrying negative-gate case (documented above under "Route Taken").

## Deviations from Plan

### Auto-fixed Issues

None — no Rule 1/2/3 auto-fixes were needed. The plan's own code was implemented as specified; the only departure from the `must_haves.artifacts` table is the negative-gate test placement, which the plan's action text explicitly anticipated and authorized as a named fallback (see "Route Taken" above), not an unplanned fix.

---

**Total deviations:** 0 auto-fixed. One plan-authorized routing decision (negative-gate test placement), not a deviation.
**Impact on plan:** None — full criterion coverage achieved via the plan's own documented fallback path.

## Issues Encountered

**Shared, non-isolated working tree with a concurrently-executing wave-1 sibling plan (04-03).** This plan ran in the same git working directory as another agent executing plan 04-03 (embeddings provider error body/drain, touching `internal/embed/*` and `internal/summarize/summarize_test.go`). Effects, both handled without touching the sibling plan's work:

1. Mid-session, `git status --short` showed `internal/embed/embed_test.go` and `internal/summarize/summarize_test.go` as modified (not files this plan touches) — the sibling agent's in-progress, not-yet-committed work in the same tree.
2. **Caught before it became a commit-boundary error.** Before Task 1's commit, `git add internal/authz/authz.go internal/store/store.go` (explicit files only) was used, then `git status --short` was re-checked and showed the sibling's two files had *also* been staged (by the sibling agent's own `git add`, running concurrently) — `git restore --staged internal/embed/embed_test.go internal/summarize/summarize_test.go` cleaned the index back down to exactly this plan's two files before committing. No destructive operation was used; the sibling's working-tree changes were untouched throughout, and the sibling's own 04-03 commits landed cleanly afterward (`docs(04-03): complete embeddings provider error body/drain plan`, visible in `git log` between this plan's Task 2 and Task 3 commits).
3. `task` (full lint + test) was run only once, after all three of this plan's tasks were committed and the sibling's 04-03 work had also fully landed — by that point the whole tree was green with zero unrelated failures.

## Next Phase Readiness

- `authz.Decision.Log()` and `authz.Bucket.String()` are available for any future diagnostics/observability work that needs the same narrow-accessor allowlist pattern.
- `internal/store`'s six ctx-threaded helper functions are the only production-code surface this plan changed; no other package imports them, so no downstream ripple.
- No blockers. `go.mod`/`go.sum` show zero diff — verified via `git diff --exit-code -- go.mod go.sum`.
- `task` (lint + full test suite, repo-wide) is green as of the final commit.

---
*Phase: 04-diagnosability*
*Completed: 2026-08-01*

## Self-Check: PASSED

All created/modified files confirmed present on disk (`internal/authz/authz.go`, `internal/authz/authz_test.go`,
`internal/store/store.go`, `internal/store/store_test.go`, `internal/store/bench_test.go`,
`internal/store/decisionlog_test.go`, this SUMMARY.md). All three task commits (`aa870647`, `6004895a`,
`4e4276e0`) confirmed present in `git log --oneline --all`.
