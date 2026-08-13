---
phase: 01-gate-ci-integrity
plan: 04
subsystem: infra
tags: [ci, github-actions, qdrant, testcontainers, go-testing]

# Dependency graph
requires: []
provides:
  - "One shared Qdrant `services:` container for the CI `test` job, replacing four independent per-package testcontainers"
  - "internal/store and internal/server route their previously-colliding `mem_eval_test` collection name through a per-package `testCollection()` prefix helper"
  - "CI one-container assertion step and an `if: failure()` diagnostics step (container state, exit code, bounded logs, dmesg OOM evidence)"
  - "testQdrantContainerBooted + TestSharedQdrantAddressHonored in all four Qdrant-backed packages, proving the shared-address path was actually taken rather than inferring it from logs"
affects: [ci, internal/store, internal/server, internal/e2e, internal/retrievaleval]

# Actuals (#2632)
actuals:
  tokens: 4695
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "GitHub Actions job-level `services:` container with explicit `--health-cmd` (this image ships no HEALTHCHECK of its own — confirmed via `docker inspect`) instead of a hand-rolled boot+poll step"
    - "Per-package `testCollectionPrefix` + `testCollection(name)` helper to namespace shared-instance-only collection collisions, without touching every hardcoded name in the package"
    - "Package-scoped `testQdrantContainerBooted` boolean as the load-bearing assertion for 'took the shared path', since address equality alone can't distinguish a coincidental match from a genuinely shared instance"

key-files:
  created: []
  modified:
    - .github/workflows/ci.yaml
    - internal/store/store_test.go
    - internal/server/tools_test.go
    - internal/e2e/harness_test.go
    - internal/retrievaleval/retrieval_eval_test.go

key-decisions:
  - "internal/retrievaleval's TestSharedQdrantAddressHonored gates on ENGRAM_RETRIEVAL_EVAL first, mirroring every other test in that package, rather than treating ENGRAM_QDRANT_TEST_ADDR alone as sufficient — TestMain never touches Qdrant unless the opt-in eval flag is set, so the CI test job (which doesn't set it) sees this one SKIP, not FAIL or PASS. This is the package's pre-existing opt-in-eval gate, not a gap in the shared-address wiring."
  - "Only the two genuinely-colliding call sites (internal/store's testStore, internal/server's testDepsWithStore) were routed through testCollection() — reindex_test.go's src/tgt pairs and other hardcoded names were left untouched per the plan's explicit scope narrowing (plan 01-05 owns the uniform sweep)."

patterns-established:
  - "CI diagnostics-on-failure steps scope their dump to the specific pinned image/container, never an unscoped docker ps -a or printenv, and tolerate their own sub-command failures so the diagnostics step itself never masks the real failure."

requirements-completed: [REQ-ci-qdrant-container-stability]

coverage:
  - id: D1
    description: "CI test job boots one shared Qdrant container via services:, with an explicit health check gating job start"
    requirement: "REQ-ci-qdrant-container-stability"
    verification:
      - kind: other
        ref: "actionlint .github/workflows/ci.yaml && yamlfmt -lint .github/workflows/ci.yaml"
        status: pass
      - kind: integration
        ref: "local rehearsal: docker run qdrant + go test ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/... -count=1 with ENGRAM_QDRANT_TEST_ADDR set"
        status: pass
    human_judgment: true
    rationale: "The real GHA services: container topology (health-check gating, port mapping under the actual runner) cannot be exercised identically outside a live GitHub Actions run; local rehearsal proves the Go-side wiring but the phase gate requires reading an actual CI run per D-20 (no green-streak-as-evidence)."
  - id: D2
    description: "internal/store and internal/server no longer address the same Qdrant collection name (store_/server_ prefix via testCollection helper)"
    requirement: "REQ-ci-qdrant-container-stability"
    verification:
      - kind: unit
        ref: "go build ./... && go vet ./... && gofmt -l . (both call sites route through testCollection(); rg confirms no bare New(..., \"mem_eval_test\") remains)"
        status: pass
    human_judgment: false
  - id: D3
    description: "CI asserts exactly one running Qdrant container (mechanism, not a green-streak) and captures state/exit code/logs/OOM evidence on failure"
    requirement: "REQ-ci-qdrant-container-stability"
    verification:
      - kind: other
        ref: "local rehearsal: assertion passes with one container (count=1, exit 0) and fails with a forced second container (count=2, exit 1); diagnostics step rehearsed against a SIGKILLed container (status=exited exitCode=137)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Each of internal/store, internal/server, internal/e2e, internal/retrievaleval proves, in its own test, that it resolved to the shared address and booted no container of its own"
    requirement: "REQ-ci-qdrant-container-stability"
    verification:
      - kind: unit
        ref: "TestSharedQdrantAddressHonored in all four packages: 4 PASS / 0 SKIP / 0 FAIL with ENGRAM_QDRANT_TEST_ADDR + ENGRAM_RETRIEVAL_EVAL=1 set; 4 SKIP / 0 PASS / 0 FAIL with it unset; red-proof: forcing internal/store's testcontainer branch with the env var still exported produces --- FAIL naming both assertions"
        status: pass
    human_judgment: false

duration: ~22min
completed: 2026-08-13
status: complete
---

# Phase 01 Plan 04: CI Qdrant Container Stability Summary

**One shared `services:` Qdrant replaces four concurrent per-package testcontainers in the CI `test` job, with a health-gated boot, a one-container assertion, container-death diagnostics, and per-package proof that the shared address was actually honored.**

## Performance

- **Duration:** ~22 min
- **Tasks:** 3
- **Files modified:** 5 (`.github/workflows/ci.yaml`, `internal/store/store_test.go`, `internal/server/tools_test.go`, `internal/e2e/harness_test.go`, `internal/retrievaleval/retrieval_eval_test.go`)

## Accomplishments

- CI `test` job now declares a job-level `services: qdrant:` container (image byte-identical to `internal/store/store_test.go`'s `qdrantImageTag`) with an explicit `--health-cmd` against `/readyz`, and sets `ENGRAM_QDRANT_TEST_ADDR=localhost:6334` alongside the existing `ENGRAM_REQUIRE_QDRANT: "1"`. Confirmed via `docker inspect qdrant/qdrant:v1.18.2 --format '{{.Config.Healthcheck}}'` -> `<nil>` that the image ships **no** `HEALTHCHECK` of its own (RESEARCH.md Assumption A1 resolved: the explicit health check is required, not redundant).
- Resolved the one real cross-package collection-name collision: `internal/store` and `internal/server` both hardcoded `"mem_eval_test"`. Each package now has a `testCollectionPrefix` const (`store_` / `server_`) and a `testCollection(name string) string` helper; the two colliding call sites (`testStore` in store, `testDepsWithStore` in server) route through it. Every other hardcoded collection name in those packages (including `reindex_test.go`'s `src`/`tgt` pairs) was deliberately left untouched — out of this plan's scope per its own instructions (plan 01-05 owns the uniform sweep).
- Added a one-container CI assertion step (before `go test`, no error-suppressing fallback in the count) and an `if: failure()` diagnostics step (container state/exit code, bounded log tail, `dmesg` OOM evidence, scoped to the pinned image only).
- Added `testQdrantContainerBooted` (set true only inside each package's testcontainer branch, immediately after the container's endpoint resolves) and `TestSharedQdrantAddressHonored` to all four Qdrant-backed packages, making "took the shared path" a checkable per-package claim rather than an inference from CI logs.

## Task Commits

1. **Task 1: End-to-end CI env -> shared Qdrant -> prefixed collection (tracer)** - `882a6bda` (feat)
2. **Task 2: Assert one container, and capture why it died** - `ae106e72` (feat)
3. **Task 3: Prove all four packages took the shared-address path** - `d13e1024` (test)

## Files Created/Modified

- `.github/workflows/ci.yaml` - `services: qdrant:` block, `ENGRAM_QDRANT_TEST_ADDR`, one-container assertion step, `if: failure()` diagnostics step
- `internal/store/store_test.go` - `testCollectionPrefix`/`testCollection`, `testStore` routed through it, `testQdrantContainerBooted`, `TestSharedQdrantAddressHonored`
- `internal/server/tools_test.go` - same shape, `server_` prefix, `testDepsWithStore` routed through it
- `internal/e2e/harness_test.go` - `testQdrantContainerBooted`, `TestSharedQdrantAddressHonored` (no collection-name change: `internal/e2e` already generates unique `"e2e_" + port` names)
- `internal/retrievaleval/retrieval_eval_test.go` - `testQdrantContainerBooted`, `TestSharedQdrantAddressHonored` gated on `ENGRAM_RETRIEVAL_EVAL` first (no collection-name change: already generates unique `"retrievaleval_" + uuid` names)

## Decisions Made

- **`internal/retrievaleval`'s shared-address test gates on `ENGRAM_RETRIEVAL_EVAL` first.** Every existing test in this package (`TestRetrievalEval`, `TestRetrievalEval_AsymmetryDiffer`) skips unless `ENGRAM_RETRIEVAL_EVAL=1` is set, because `TestMain` itself never touches Qdrant unless that opt-in flag is `"1"` (its very first statement). `TestSharedQdrantAddressHonored` follows the same convention rather than asserting on `ENGRAM_QDRANT_TEST_ADDR` alone. Consequence, verified directly: running the four-package rehearsal with only `ENGRAM_QDRANT_TEST_ADDR` set (matching the plan's literal `<verify>` command, and matching real CI, which never sets `ENGRAM_RETRIEVAL_EVAL`) produces 3 PASS + 1 SKIP (`internal/retrievaleval`), not 4 PASS. Setting `ENGRAM_RETRIEVAL_EVAL=1` in addition proves the shared-address path genuinely works for that package too: 4 PASS, 0 SKIP, 0 FAIL. Both runs are recorded verbatim below. This is the package's own pre-existing opt-in-eval gate operating exactly as designed — not a defect in the shared-instance wiring — and changing `TestMain`'s precedence to bypass it would have been unscoped, since real CI's `test` job never runs the retrieval eval either.
- **Only the two collision sites were prefixed, not every hardcoded name in `internal/store`/`internal/server`.** The plan's Task 1 explicitly scopes this to a "proven path" rather than a 60-literal diff; the uniform sweep is deferred to plan 01-05.

## Deviations from Plan

None — plan executed as written, with one clarification recorded above (the `ENGRAM_RETRIEVAL_EVAL` interaction with Task 3's `TestSharedQdrantAddressHonored`, which the plan's illustrative `<verify>` command did not call out explicitly but which follows directly from `internal/retrievaleval`'s pre-existing, unmodified test-gating convention).

### Acceptance-criteria note (not a deviation, a wording nuance)

Task 1's acceptance criterion `rg -c '"mem_eval_test"' internal/store/store_test.go internal/server/tools_test.go finds no bare occurrence at the two collision sites` is satisfied in *intent* (both call sites now read `testCollection("mem_eval_test")`, never a raw `New(..., "mem_eval_test")`) but the literal `rg -c` command as written still counts 1 match per file, since the string literal itself is necessarily still present as `testCollection`'s argument. Verified instead with `rg -n 'New\(.*"mem_eval_test"'` (bare-call-site pattern), which correctly returns zero matches in both files.

## Verbatim Evidence

**A1 (RESEARCH.md) — does the Qdrant image ship its own HEALTHCHECK:**
```
$ docker inspect qdrant/qdrant:v1.18.2 --format '{{.Config.Healthcheck}}'
<nil>
```
Confirms the explicit `--health-cmd` in the `services:` block is required, not redundant.

**Task 1 local rehearsal (store + server against one shared Qdrant):**
```
$ ENGRAM_QDRANT_TEST_ADDR=localhost:6334 ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... ./internal/server/... -count=1
ok  	github.com/seanb4t/engram/internal/store	21.392s
ok  	github.com/seanb4t/engram/internal/server	10.841s
EXIT: 0   (0 --- FAIL lines, 0 --- SKIP lines)
```
D-18 fallback (env var unset, testcontainer still boots and passes):
```
$ go test ./internal/store/... -count=1 -v
...
ok  	github.com/seanb4t/engram/internal/store	12.662s
```

**Task 2 one-container assertion, both directions:**
```
# one container running
ENGRAM_QDRANT_TEST_ADDR resolved to: localhost:6334
running Qdrant containers (ancestor qdrant/qdrant:v1.18.2): 1
PASS-DIRECTION EXIT: 0

# forced second container running
ENGRAM_QDRANT_TEST_ADDR resolved to: localhost:6334
running Qdrant containers (ancestor qdrant/qdrant:v1.18.2): 2
::error::expected exactly one Qdrant container, found 2
CONTAINER ID   IMAGE                   ...  NAMES
8586f8dadfd1   qdrant/qdrant:v1.18.2   ...  engram-ci-assert-b
5795e58ddf83   qdrant/qdrant:v1.18.2   ...  engram-ci-assert-a
FAIL-DIRECTION EXIT: 1
```

**Task 2 diagnostics step, against a SIGKILLed container:**
```
--- qdrant container state ---
id=484434e0... status=exited exitCode=137 oomKilled=false error=
--- qdrant container logs (last 200 lines) ---
[Qdrant v1.18.2 startup banner and logs]
--- kernel OOM evidence ---
no OOM evidence in dmesg (or dmesg unavailable)
EXIT: 0
```
(exitCode=137 = 128+SIGKILL, consistent with the forced kill; `dmesg` unavailable on this macOS dev host is tolerated without failing the step, exactly as it would tolerate a genuinely OOM-silent kernel.)

**Task 3 fail-first proof (internal/store, testcontainer branch forced with `ENGRAM_QDRANT_TEST_ADDR` still exported):**
```
=== RUN   TestSharedQdrantAddressHonored
    store_test.go:227: testQdrantAddr = "127.0.0.1:55175", want "localhost:6334" (shared CI Qdrant address not honored)
    store_test.go:230: testQdrantContainerBooted = true, want false: ENGRAM_QDRANT_TEST_ADDR was set but this package booted its own testcontainer anyway
--- FAIL: TestSharedQdrantAddressHonored (0.00s)
FAIL	github.com/seanb4t/engram/internal/store	1.207s
```
(reverted immediately after; `go build ./...` clean afterward)

**Task 3 four-package rehearsal, address set, without `ENGRAM_RETRIEVAL_EVAL` (matches real CI exactly):**
```
$ ENGRAM_QDRANT_TEST_ADDR=localhost:6334 ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/... -run TestSharedQdrantAddressHonored -v -count=1
--- PASS: TestSharedQdrantAddressHonored   (internal/store)
--- PASS: TestSharedQdrantAddressHonored   (internal/server)
--- PASS: TestSharedQdrantAddressHonored   (internal/e2e)
--- SKIP: TestSharedQdrantAddressHonored   (internal/retrievaleval — ENGRAM_RETRIEVAL_EVAL not set)
EXIT: 0
```

**Task 3 four-package rehearsal, address set, with `ENGRAM_RETRIEVAL_EVAL=1` (proves retrievaleval's own shared-address path too):**
```
$ ENGRAM_RETRIEVAL_EVAL=1 ENGRAM_QDRANT_TEST_ADDR=localhost:6334 ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/... -run TestSharedQdrantAddressHonored -v -count=1
--- PASS x4, --- SKIP x0, --- FAIL x0
EXIT: 0
```

**Task 3 rehearsal, address unset (all four skip, no cascading failure):**
```
$ go test ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/... -run TestSharedQdrantAddressHonored -v -count=1
--- SKIP x4, --- PASS x0, --- FAIL x0
EXIT: 0
```

**Final full-suite rehearsal (all four packages, real integration tests, one shared Qdrant):**
```
$ ENGRAM_QDRANT_TEST_ADDR=localhost:6334 ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/... -count=1
ok  	github.com/seanb4t/engram/internal/store	23.362s
ok  	github.com/seanb4t/engram/internal/server	13.605s
ok  	github.com/seanb4t/engram/internal/e2e	4.113s
ok  	github.com/seanb4t/engram/internal/retrievaleval	0.220s
EXIT: 0   (0 FAIL, 0 SKIP printed — retrievaleval's own eval tests are silently skipped without -v, which is expected/unrelated to this fix)
```

**Overall verification block:**
- `actionlint .github/workflows/ci.yaml` — clean
- `yamlfmt -lint .github/workflows/ci.yaml` — clean
- `task lint` — all checks passed (golangci-lint, actionlint, rumdl, yamlfmt, ruff)
- `gofmt -l .` — prints nothing
- `go vet ./...` — clean
- `git diff --stat go.mod go.sum` — empty (no dependency changes)

## Issues Encountered

None beyond the `ENGRAM_RETRIEVAL_EVAL` interaction documented above under Decisions Made.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- CI's `test` job now has the CI-side wiring for one shared Qdrant, the one-container assertion, the diagnostics-on-failure step, and per-package shared-address proof tests — all verified locally against a real Qdrant.
- **Real GHA verification is still required before this claim is fully closed** (D-20 explicitly rejects a green local rehearsal as sufficient — a real GitHub Actions run must be read for the mechanism assertions). This happens naturally when this plan's commits reach a PR against `main`; the phase's own `VERIFICATION.md`/UAT step should confirm the actual CI run shows the one-container assertion passing and no cascading `connection refused` failures.
- `internal/store/reindex_test.go`'s `src`/`tgt`-family hardcoded names and every other hardcoded collection name in `internal/store`/`internal/server` remain un-prefixed by design — plan 01-05 owns that uniform sweep.

---
*Phase: 01-gate-ci-integrity*
*Completed: 2026-08-13*
