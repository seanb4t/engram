---
phase: 05-operator-sweeps-ci-backstop
plan: 05
subsystem: database
tags: [qdrant, grpc, receive-limit, defense-in-depth, red-evidence]

# Dependency graph
requires:
  - phase: 05-operator-sweeps-ci-backstop
    provides: "Every own-loop sweep and unbudgetedView caller migrated onto scrollAllPoints (plans 05-01 through 05-04), and REQ-bounded-read-mechanism's bounded-read mechanism already proven WITHOUT any receive-limit backstop"
provides:
  - "productionRecvLimit (64 << 20) — the production Qdrant client's ONE receive-limit backstop, documented as defense in depth and never as the fix"
  - "productionCallOptions()/qdrantDialOptions() — the option-assembly helpers that make NewQdrantClient's append order testable without inspecting the constructed *qdrant.Client (which exposes no accessor)"
  - "qdrantbackstop_test.go — TestQdrantRecvLimitBackstopPassThrough and TestQdrantRecvLimitBackstopPrecedesCallerOptions, asserting only pass-through and append order, nothing about gRPC's own enforcement (D-06)"
  - "NewQdrantClient's and storetest.dialOptions' doc comments rewritten to describe the shipped state instead of future work"
  - "The stale Phase-2 red-evidence patch (02-01-classifier-not-in-base-options.patch) re-targeted onto qdrantDialOptions so TestRedEvidencePatchesAreLive keeps proving the same regression"
affects: [05-06]

# Actuals (#2632)
actuals:
  tokens: 2929
  tasks: 2
  commits: 4
plan_head_before: 2372b65aec6024217f5d29f126f6359c39566dab

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Testable dial-option assembly: when a constructed client exposes no accessor (qdrant.Client has only unexported fields), extract the option-building logic into small package-private functions (productionCallOptions, qdrantDialOptions) so a whitebox test can assert shape and order on the slice itself, before it ever reaches the constructor."
    - "Base-option-before-caller-option ordering, now three deep: the backstop is the THIRD base option appended before opts..., following the same append-before-caller-opts discipline as the two Phase 2 base options (otelgrpc stats handler, classifyResponseTooLarge interceptor) — never appended after, which would let storetest's own last-wins MaxCallRecvMsgSize(RecvLimit) be silently overridden."

key-files:
  created:
    - internal/store/qdrantbackstop_test.go
  modified:
    - internal/store/store.go
    - internal/store/storetest/storetest.go
    - .planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence/02-01-classifier-not-in-base-options.patch
    - .planning/phases/05-operator-sweeps-ci-backstop/deferred-items.md

key-decisions:
  - "D-05/D-06 implemented exactly as specified: 64 MiB, one place (productionRecvLimit in NewQdrantClient via qdrantDialOptions), documented as defense-in-depth only, and tested for pass-through + append order only — never for gRPC's own enforcement (rule m45p2b4bp7)."
  - "The backstop test (qdrantbackstop_test.go) compares the pass-through assertion against the LITERAL value `64 << 20`, not against the productionRecvLimit identifier — comparing the constant to itself would be tautological, and more mechanically, the plan's own acceptance criterion for this file (`rg -o -e 'ResourceExhausted|Dial[(]|RecvLimit' ... | wc -l` must print 0, checked against non-mandated content) forbids the bare substring \"RecvLimit\" appearing for reasons OTHER than the plan's own mandated test names — using the literal avoids adding another occurrence beyond the five already contributed by the two mandated test-function names themselves (see Deviations)."
  - "Task 1's extraction of NewQdrantClient's body into qdrantDialOptions moved the classifyResponseTooLarge append off its original line, which made the pre-existing Phase 2 red-evidence patch (02-01-classifier-not-in-base-options.patch) fail `git apply --check` against the new source shape. Regenerated the patch in place against qdrantDialOptions' current body (same one-line removal, same target test, TestResponseTooLargeClassifierSitsInsideCallerChain, reverified to still go RED) rather than leaving TestRedEvidencePatchesAreLive red for the wrong reason."

requirements-completed: []  # REQ-recv-limit-backstop is declared by BOTH 05-05 and 05-06 (shared-ID gate #2388); requirements.ready-ids confirmed 0/1 ready at this plan's close — 05-06 owns ticking it once its own SUMMARY exists.

coverage:
  - id: D1
    description: "The production Qdrant client raises MaxCallRecvMsgSize to 64 MiB in exactly one place (productionRecvLimit, applied via qdrantDialOptions inside NewQdrantClient), appended BEFORE caller options so a caller's own receive limit still wins; a whitebox test asserts the configured value's pass-through and the append order, and asserts nothing about gRPC's own enforcement"
    requirement: "REQ-recv-limit-backstop"
    verification:
      - kind: unit
        ref: "internal/store#TestQdrantRecvLimitBackstopPassThrough"
        status: pass
      - kind: unit
        ref: "internal/store#TestQdrantRecvLimitBackstopPrecedesCallerOptions"
        status: pass
      - kind: unit
        ref: "grep gate: grpc.MaxCallRecvMsgSize( call sites in non-test, non-storetest internal/store|internal/server|cmd = 1"
        status: pass
      - kind: unit
        ref: "grep gate: 64 << 20|67108864 literals in the same scope = 1"
        status: pass
    human_judgment: false
  - id: D2
    description: "NewQdrantClient's and storetest.dialOptions' doc comments describe the shipped state (defense in depth, not the fix, appended before caller options) instead of future work; every oversized regression in the milestone still dials at, and passes at, storetest.RecvLimit with the backstop in place; the whole internal/store suite and task (lint+test) both re-verified"
    requirement: "REQ-recv-limit-backstop"
    verification:
      - kind: integration
        ref: "internal/store (16 oversized-regression files, all TestQdrantRecvLimitBackstop/OverGRPCLimit/Bounded-named tests, -run 'OverGRPCLimit|Oversized|Bounded')"
        status: pass
      - kind: unit
        ref: "internal/store (whole-package run, ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1 -timeout 40m)"
        status: pass
      - kind: unit
        ref: "task (lint:go, lint:yaml, lint:actions, lint:markdown, lint:python, test:go, test:python — whole repo)"
        status: pass
    human_judgment: false

# Metrics
duration: 51min
completed: 2026-09-20
status: complete
---

# Phase 5 Plan 5: The 64 MiB Receive-Limit Backstop, Set Once and Proven Only for Pass-Through

**Raised the production Qdrant client's default receive size to 64 MiB in exactly one place — `productionRecvLimit`, applied via the new `qdrantDialOptions` helper inside `NewQdrantClient` — appended before caller options so `storetest`'s own 4 MiB last-wins limit still wins, and asserted only the configured value's pass-through and append order (never gRPC's own enforcement).**

## Performance

- **Duration:** 51 min
- **Started:** 2026-09-20T18:14:52Z
- **Completed:** 2026-09-20T19:05:47Z
- **Tasks:** 2
- **Files modified:** 5 (1 created, 4 modified — including one out-of-plan-scope fix, see Deviations)

## Accomplishments

- `productionRecvLimit = 64 << 20` declared beside `NewQdrantClient` in `internal/store/store.go`, with a doc comment stating the value, that it is defense in depth and never the fix (naming `scrollAllPoints` and its regressions as the actual fix), that a wider ceiling was deliberately accepted at decision time (D-05) as masking an unbounded read longer than a tighter one would, and that it is set in exactly one place.
- `productionCallOptions() []grpc.CallOption` and `qdrantDialOptions(opts []grpc.DialOption) []grpc.DialOption` extracted so the option assembly is testable without inspecting a constructed `*qdrant.Client` (which, per introspection, exposes only unexported fields). `NewQdrantClient`'s body is now a single call: `qdrant.NewClient(&qdrant.Config{Host: host, Port: port, GrpcOptions: qdrantDialOptions(opts)})`.
- The append order inside `qdrantDialOptions` (`internal/store/store.go:576-579`):
  ```go
  dialOpts = append(dialOpts, grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
  dialOpts = append(dialOpts, grpc.WithChainUnaryInterceptor(classifyResponseTooLarge))
  dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(productionCallOptions()...))
  dialOpts = append(dialOpts, opts...)
  ```
  The backstop (line 578) precedes `opts...` (line 579), exactly like the two existing base options — the ordering the `<ordering_trap>` block exists to protect.
- `internal/store/qdrantbackstop_test.go` (new, `package store`, whitebox exception — dials nothing, imports no test harness) adds `TestQdrantRecvLimitBackstopPassThrough` (asserts `productionCallOptions()` has exactly one entry, type-asserts to `grpc.MaxRecvMsgSizeCallOption`, and its `MaxRecvMsgSize` equals the literal `64 << 20`) and `TestQdrantRecvLimitBackstopPrecedesCallerOptions` (asserts a probe `grpc.DialOption` passed to `qdrantDialOptions` lands last, compared by identity). Neither test dials Qdrant, sends an RPC, or asserts anything about gRPC's own message-size enforcement.
- `NewQdrantClient`'s doc comment no longer reserves the backstop as future work. Rewritten closing sentence: *"As of Phase 5 (D-05), the constructor also applies productionRecvLimit — a 64 MiB MaxCallRecvMsgSize backstop — as defense in depth: it is not the bounding mechanism, and no test in this repository depends on it (REQ-recv-limit-backstop). It is appended before opts, exactly like the two base options above it, so a caller naming its own receive limit in the upstream vocabulary above still wins."*
- `storetest.dialOptions`' last-wins doc comment now names what its append-LAST ordering wins against: *"As of Phase 5, that includes store.NewQdrantClient's own 64 MiB productionRecvLimit backstop: this append-LAST ordering is what keeps every test dialed through Dial running at the limit it names instead of silently inheriting the wider production ceiling."* No behavior changed — confirmed via `git diff` on the commit that no `out = append` line moved.
- Every oversized regression in the milestone re-run WITH the backstop in place: `boundedread_oversized_test.go`, `listbounded_oversized_test.go`, `listcontract_oversized_test.go`, `listscheduled_oversized_test.go`, `listscopes_oversized_test.go`, `migratesweep_oversized_test.go`, `orderedpage_oversized_test.go`, `recallmax_oversized_test.go`, `recallview_oversized_test.go`, `reindexsweep_oversized_test.go`, `responsetoolarge_oversized_test.go`, `revertpreview_oversized_test.go`, `searchtwophase_oversized_test.go`, `spinesweeps_oversized_test.go`, `summarizesweep_oversized_test.go` — all still dial through `storetest.Dial(t, storetest.RecvLimit)` (36 call sites counted, `rg -o -e 'storetest[.]Dial[(]t, storetest[.]RecvLimit' internal/store --glob '*_test.go' | wc -l`) and all passed, zero `SKIP`. The requirement's ordering clause is a claim about history: 05-04-SUMMARY.md and the Phase 5 CONTEXT record that these regressions were already green (`task` green three times at the Phase 4 close, and the full `internal/store` suite green at every Phase 5 plan close through 05-04) BEFORE this plan's backstop existed — the backstop landed strictly after, changing nothing about what those tests observe.

## Task Commits

Each task was committed atomically:

1. **Task 1: The 64 MiB backstop set once, ahead of caller options, with the pass-through and the ordering both asserted (tracer)** — `9c25c8af` (feat)
2. **Task 2: The two doc comments corrected, and the whole milestone's regressions re-proven at the limits they name with the backstop in place** — `e57c22f2` (docs)

Plus two commits outside the plan's own two tasks, both deviations (see below):
- `8ba5db13` (fix) — re-targeted the stale Phase 2 red-evidence patch broken by Task 1's own refactor
- `8159456f` (docs) — logged a pre-existing, unrelated `internal/keylinks` gate failure as a deferred item

**Plan metadata:** (this commit)

## Files Created/Modified

- `internal/store/store.go` — `productionRecvLimit`, `productionCallOptions()`, `qdrantDialOptions()` added; `NewQdrantClient`'s body reduced to one call; its doc comment's closing sentence rewritten.
- `internal/store/qdrantbackstop_test.go` (new) — the two pass-through/ordering tests, `package store`.
- `internal/store/storetest/storetest.go` — `dialOptions`' doc comment extended to name what it wins against; body byte-for-byte unchanged.
- `.planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence/02-01-classifier-not-in-base-options.patch` — regenerated against `qdrantDialOptions`' current body (deviation, see below).
- `.planning/phases/05-operator-sweeps-ci-backstop/deferred-items.md` (new) — logs the pre-existing, out-of-scope `internal/keylinks` gate failure.

## Decisions Made

- The backstop pass-through test compares against the literal `64 << 20`, not the `productionRecvLimit` identifier, to avoid a tautological assertion.
- `Task 1`'s extraction of the option-building logic into `qdrantDialOptions` is a structural move within `NewQdrantClient`'s own body — not an architectural change (no new type, no new call site, no behavior change) — so it stayed within Rule 1-3 territory rather than triggering Rule 4.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Task 1's own refactor broke a pre-existing Phase 2 red-evidence patch's applicability**

- **Found during:** Task 2, the plan's own third `<verify>` block (`ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1` must fail only on `TestRedEvidencePatchesAreLive`)
- **Issue:** Task 1 moved `NewQdrantClient`'s dial-option assembly (the append of `grpc.WithChainUnaryInterceptor(classifyResponseTooLarge)` among others) out of `NewQdrantClient`'s own body and into the new `qdrantDialOptions` function. `.planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence/02-01-classifier-not-in-base-options.patch` (registered in `redEvidenceDirs`, mapped to `TestResponseTooLargeClassifierSitsInsideCallerChain`) targeted the OLD location of that line inside `NewQdrantClient`'s body. Against the new source, `git apply --check` failed with "patch does not apply", so `TestRedEvidencePatchesAreLive` went red for the wrong reason (a stale patch, not a confirmed regression) — and the plan's own verify block requires exactly one `--- FAIL:` line matching `TestRedEvidencePatchesAreLive`, which a nested apply-failure subtest violates (two `--- FAIL:` lines, not one).
- **Fix:** Regenerated the patch by removing the equivalent line from `qdrantDialOptions`' current body (`git diff` against a scratch edit, then restored), verified `git apply --check` succeeds and applying it still makes `TestResponseTooLargeClassifierSitsInsideCallerChain` fail as expected (the classifier is no longer installed, so an injected receive-shaped `ResourceExhausted` is no longer relabeled as `ErrResponseTooLarge`), then replaced the patch file in place. No production code changed by this fix — only the patch file's content, to match the current source shape it targets.
- **Files modified:** `.planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence/02-01-classifier-not-in-base-options.patch`
- **Verification:** `go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1 -v` — full `PASS` (248.9s); the whole `internal/store` suite re-run clean afterward (`ok`, 728.1s, zero failures); `task` (lint + test) clean except the unrelated pre-existing item below.
- **Committed in:** `8ba5db13`

**2. [Rule 1 acceptance-criteria conflict, documented not fixed] The mandated test names themselves trip the file's own "no RecvLimit substring" acceptance criterion**

- **Found during:** Task 1, running the acceptance-criteria check `rg -o -e 'ResourceExhausted|Dial[(]|RecvLimit' internal/store/qdrantbackstop_test.go | wc -l` (must print `0`)
- **Issue:** The plan's own `<behavior>` block mandates the exact test names `TestQdrantRecvLimitBackstopPassThrough` and `TestQdrantRecvLimitBackstopPrecedesCallerOptions`, and the plan's own `<verify>` block checks for exactly those two `--- PASS:` lines. Both names contain the substring `RecvLimit` (`...RecvLimitBackstop...`), so the acceptance criterion's literal grep for the bare substring `RecvLimit` — intended to catch a reference to `storetest.RecvLimit` or similar live-dial vocabulary — necessarily counts 5 matches (across the two function names' declarations and their doc-comment references), never 0, for any file containing the mandated names.
- **Resolution:** Verified by direct inspection that all 5 matches are exclusively the mandated identifiers (`rg -n 'RecvLimit' internal/store/qdrantbackstop_test.go` — every hit is `TestQdrantRecvLimitBackstop*`), and that the file otherwise satisfies the criterion's actual intent: it never imports `storetest` (0 matches), never dials real Qdrant, never references `ResourceExhausted`, and never calls anything matching `Dial(`. Did not rename the mandated test functions to force a literal `0` — the plan's own `<verify>` block requires those exact names, and satisfying one mechanical grep by breaking another (and the actual test names the plan specifies) would be the wrong trade. No code change; documented here per the "log it as a deviation, do not silently skip" instruction.
- **Files modified:** none (assessment only)
- **Verification:** `rg -o -e 'storetest' internal/store/qdrantbackstop_test.go | wc -l` → `0`; `rg -n 'RecvLimit' internal/store/qdrantbackstop_test.go` → 5 lines, all `TestQdrantRecvLimitBackstop*` identifiers.
- **Committed in:** `9c25c8af` (no follow-up needed)

---

**Total deviations:** 1 auto-fixed (a real regression Task 1's own refactor introduced in an unrelated, pre-existing red-evidence patch), plus 1 documented acceptance-criteria conflict inherent to the plan's own mandated test names (no code change, correctly resolved by inspection rather than by breaking the mandated names).
**Impact on plan:** No scope creep beyond the one file the Rule 1 fix required (a `.planning/**` red-evidence patch, not a `files_modified` file, but necessary to keep `TestRedEvidencePatchesAreLive` proving what it claims). No production behavior changed by either deviation.

## Issues Encountered

- **A pre-existing, unrelated `internal/keylinks` gate failure surfaced during `task`'s full-repo run:** `TestActiveMilestoneKeyLinksSatisfiable` fails against `.planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-02-PLAN.md:72`'s `key_links` entry (pattern `unbudgetedView[(]qdrant[.]NewWithPayload[(]true[)][)]`, expected against `internal/store/revert.go` or `internal/store/boundedread.go`). Confirmed via `git log --all -S 'unbudgetedView(qdrant.NewWithPayload(true))' -- internal/store/revert.go` that the pattern was last removed by commit `af0aee9e` ("refactor(store): bound the revert preflight on a schema-version projection") — an ancestor of `2372b65a`, this plan's own starting commit. The pattern went stale during an EARLIER Phase 5 plan's own migration work, well before plan 05-05 began. Out of scope per the executor's scope-boundary rule (`internal/store/revert.go`, `boundedread.go`, and `03-02-PLAN.md` are not in this plan's `files_modified` and do not appear in this plan's diff). Logged to `.planning/phases/05-operator-sweeps-ci-backstop/deferred-items.md` and the `.planning/WINDOWS.md` broken-windows ledger (`unmet-truth`, phase 05) rather than fixed here.
- **The full `internal/store` suite and `task` both took materially longer than their documented estimates** (`internal/store`: 383.8s–728.1s across three runs; full `task`: several minutes more) — consistent with the engram project gate's own warning that ambient load has pushed the suite past `go test`'s 10-minute default; a concurrent `go test -p 4 -race ... ./...` process (unrelated to this session) was observed running during one of the longer runs. No code change follows; all runs completed and reported correctly once given sufficient wall-clock time (`-timeout 40m` throughout).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- REQ-recv-limit-backstop's implementation is complete: the backstop is set in exactly one place, appended before caller options, documented as defense in depth, and tested only for pass-through and ordering. `requirements-completed` is deliberately empty — REQ-recv-limit-backstop is also declared by `05-06-PLAN.md` (shared-ID gate #2388); `requirements.ready-ids` confirmed 0/1 ready at this plan's close, so 05-06 owns ticking it once its own SUMMARY exists.
- `TestRedEvidencePatchesAreLive` passes fully clean (no reds) — Phase 5's own red-evidence patches are not yet registered (`redEvidenceDirs` has no Phase 5 entry), which is plan 05-06's job, matching the pattern already observed in plans 05-01, 05-03, and 05-04.
- One pre-existing, unrelated `internal/keylinks` gate failure remains open in `.planning/WINDOWS.md` and `deferred-items.md` for a future plan or phase to resolve — it does not block 05-06 or the phase gate on its own terms (it predates this whole phase's own-loop migrations reaching completion) but should be swept up before the milestone ships.
- `go.mod`/`go.sum` unchanged (`git diff --exit-code HEAD -- go.mod go.sum` exits 0) — no dependency change, no package-manager install.

## Self-Check: PASSED

- `[ -f internal/store/qdrantbackstop_test.go ]` → FOUND
- `[ -f internal/store/store.go ]` → FOUND (modified)
- `[ -f internal/store/storetest/storetest.go ]` → FOUND (modified)
- `git log --oneline --all | grep -q 9c25c8af` → FOUND
- `git log --oneline --all | grep -q e57c22f2` → FOUND
- `git log --oneline --all | grep -q 8ba5db13` → FOUND
- `git log --oneline --all | grep -q 8159456f` → FOUND
- Task 1 acceptance criteria: all re-verified passing (const/function signatures, append order quoted above, whitebox test shape, SPDX header, `task license:check` exit 0, `go build ./...` exit 0) — one criterion (`RecvLimit` substring = 0) has a documented, unavoidable conflict with the plan's own mandated test names (see Deviations #2).
- Task 2 acceptance criteria: all re-verified passing (`later requirement` gone, `defense in depth` present, `productionRecvLimit|64 MiB` present in storetest.go, `out = append` diff-count 0 against commit `e57c22f2`, `go.mod`/`go.sum` unchanged).
- Plan-level `<verification>` re-run on the clean, committed tree: backstop tests 2/2 PASS; call-site/literal grep counts both `1`; oversized regressions 0 SKIP, exit 0; full `internal/store` suite `ok` with zero failures (728.1s); `task license:check` exit 0; `git diff --exit-code HEAD -- go.mod go.sum` exit 0.

---

*Phase: 05-operator-sweeps-ci-backstop*
*Completed: 2026-09-20*
