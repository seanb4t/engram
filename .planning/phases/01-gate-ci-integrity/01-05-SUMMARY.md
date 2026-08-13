---
phase: 01-gate-ci-integrity
plan: 05
subsystem: testing
tags: [go-testing, qdrant, testcontainers, ci]

# Dependency graph
requires:
  - phase: 01-gate-ci-integrity
    provides: "01-04's shared-Qdrant CI seam (services: container, testCollectionPrefix/testCollection in internal/store and internal/server, testQdrantContainerBooted + TestSharedQdrantAddressHonored in all four packages)"
provides:
  - "newTestStore(t, c, name) — a prefix-enforcing construction seam in all four Qdrant-backed test packages (internal/store, internal/server, internal/e2e, internal/retrievaleval) that t.Fatalf's naming the offending value if a collection name does not carry the package's testCollectionPrefix"
  - "testCollectionPrefix + testCollection extended to internal/e2e (\"e2e_\") and internal/retrievaleval (\"retrievaleval_\"), naming their pre-existing per-port/per-UUID unique-name generators"
  - "internal/store fully swept: every hardcoded collection literal (store_test.go's production-defaults pair, spine_test.go's ~30 call sites via one helper-level prefix application, reindex_test.go's 15 const src/tgt-family declarations) now routes through testCollection() + newTestStore()"
  - "internal/e2e/spine_review_test.go's newSpineReviewStore (a second, previously-unprefixed store.New call site the plan's own read_first list omitted) also routed through the seam"
affects: [internal/store, internal/server, internal/e2e, internal/retrievaleval, ci]

# Actuals (#2632)
actuals:
  tokens: 7369
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "newTestStore(t testing.TB, c *qdrant.Client, name string, opts ...Option) *Store — a runtime construction seam (not a source-level lint) that fails the test naming the offending collection name if it does not carry the package's own testCollectionPrefix; a raw name assigned to a variable first cannot route around it the way a static check could be"
    - "Prefix-once-at-the-helper: a shared per-package test-store constructor (newSpineTestStore, newSpineReviewStore) applies testCollection() a single time at its own top, so its many call sites keep passing bare descriptive names unchanged — the delete-before-create + t.Cleanup deletion pairing then automatically operates on the prefixed name"
    - "When the SAME literal collection name must be shared between a Go-side store construction and a subprocess's env var (internal/e2e's pruneEnv), the caller pre-computes the prefixed name via testCollection() once and threads it to both consumers, rather than having the helper apply the prefix internally (which would silently produce two different names)"

key-files:
  created: []
  modified:
    - internal/store/store_test.go
    - internal/store/spine_test.go
    - internal/store/reindex_test.go
    - internal/server/tools_test.go
    - internal/e2e/harness_test.go
    - internal/e2e/spine_review_test.go
    - internal/retrievaleval/retrieval_eval_test.go

key-decisions:
  - "internal/e2e/spine_review_test.go's newSpineReviewStore was routed through the seam even though it was not in Task 1's <files> list, because it is a second real store.New call site in the e2e package that the plan's own must_haves (\"No test constructs a store over a raw collection name: every construction goes through the package's seam\") require covered — leaving it out would have made that must_have false for the shipped code."
  - "internal/store's newTestStore signature carries a variadic opts ...Option parameter (the other three packages' do not) so newSpineTestStore's WithClock/WithAuthz callers route through the same single seam as every unopted construction, instead of bypassing it or constructing twice."
  - "internal/retrievaleval's newTestcontainerStore signature changed from (addr string, dim uint64) (*store.Store, error) to (t testing.TB, addr string, dim uint64) *store.Store — threading a test handle in per the plan's own guidance (\"where a current construction site has no test handle in scope, thread one in rather than dropping the assertion\") since its one call site is inside a t.Run closure with t already in scope."

patterns-established:
  - "A pre-existing, out-of-scope lint failure discovered mid-plan (rumdl MD041 on internal/keylinks test fixtures, unrelated to any file this plan touched) is logged to deferred-items.md rather than fixed, per the executor's SCOPE BOUNDARY rule."

requirements-completed: [REQ-ci-qdrant-container-stability]

coverage:
  - id: D1
    description: "newTestStore exists in all four Qdrant-backed packages and fails the test at runtime, naming the offending value, when a collection name does not carry the package's testCollectionPrefix"
    requirement: "REQ-ci-qdrant-container-stability"
    verification:
      - kind: unit
        ref: "rg -c '^func newTestStore' internal/store internal/server internal/e2e internal/retrievaleval — 1 per package"
        status: pass
      - kind: integration
        ref: "red-proof: internal/server's testDepsWithStore temporarily passed the unprefixed literal \"mem_eval_test\" to newTestStore; TestStoreMemoryUsesInjectedClock failed naming the offending value, then reverted"
        status: pass
    human_judgment: false
  - id: D2
    description: "internal/e2e and internal/retrievaleval extend testCollectionPrefix/testCollection, naming their existing collision-safe per-port/per-UUID generators without changing the generated suffix"
    requirement: "REQ-ci-qdrant-container-stability"
    verification:
      - kind: integration
        ref: "ENGRAM_QDRANT_TEST_ADDR=localhost:6334 ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/... -count=1 — all PASS against one shared Qdrant"
        status: pass
    human_judgment: false
  - id: D3
    description: "No raw collection name survives in internal/store: store_test.go's production-defaults pair, spine_test.go's ~30 call sites (prefix applied once at the helper), and reindex_test.go's 15 const src/tgt-family declarations all route through testCollection()/newTestStore()"
    requirement: "REQ-ci-qdrant-container-stability"
    verification:
      - kind: unit
        ref: "rg -n 'New(c, \"' internal/store/reindex_test.go && rg -n 'New(dialTestClient' internal/store/store_test.go && rg -c 'const src, tgt' internal/store/reindex_test.go — all empty/zero"
        status: pass
      - kind: integration
        ref: "two consecutive ENGRAM_QDRANT_TEST_ADDR=localhost:6334 ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -count=1 runs against the SAME running Qdrant, both PASS"
        status: pass
    human_judgment: false
  - id: D4
    description: "No enumerate-then-delete-all teardown was introduced; the whole-repo go test ./... -count=1 passes end to end with the shared address set"
    requirement: "REQ-ci-qdrant-container-stability"
    verification:
      - kind: unit
        ref: "rg -n 'ListCollections' internal/ — empty"
        status: pass
      - kind: integration
        ref: "ENGRAM_QDRANT_TEST_ADDR=localhost:6334 ENGRAM_REQUIRE_QDRANT=1 go test ./... -count=1 — all packages ok"
        status: pass
    human_judgment: false

duration: ~45min
completed: 2026-08-13
status: complete
---

# Phase 01 Plan 05: Per-Package Qdrant Collection Namespacing Summary

**Every Qdrant-backed test package now builds its test stores through a runtime `newTestStore` seam that `t.Fatalf`'s naming the offending value if a collection name skips that package's `testCollectionPrefix`, closing the one gap 01-04 deliberately left open (the uniform sweep of `internal/store`'s ~50 remaining hardcoded names) and covering a second unprefixed construction site in `internal/e2e` the plan itself hadn't listed.**

## Performance

- **Duration:** ~45 min
- **Tasks:** 2
- **Files modified:** 7 (`internal/store/{store,spine,reindex}_test.go`, `internal/server/tools_test.go`, `internal/e2e/{harness,spine_review}_test.go`, `internal/retrievaleval/retrieval_eval_test.go`)

## Accomplishments

- Added `newTestStore(t testing.TB, c *qdrant.Client, name string[, opts ...Option]) *Store` to all four Qdrant-backed test packages: it asserts `name` carries the package's own `testCollectionPrefix` and `t.Fatalf`'s naming the offending value otherwise — a runtime assertion that cannot be routed around the way a source-level lint could (by first assigning the raw name to a variable).
- Extended `testCollectionPrefix`/`testCollection` to `internal/e2e` (`"e2e_"`) and `internal/retrievaleval` (`"retrievaleval_"`), naming their pre-existing collision-safe per-port and per-UUID generators rather than replacing them — the generated suffix is byte-identical to before.
- Proved the seam's red direction live: temporarily passed the unprefixed literal `"mem_eval_test"` to `internal/server`'s `newTestStore`; `TestStoreMemoryUsesInjectedClock` failed naming the offending value, then reverted.
- Swept every remaining hardcoded collection name in `internal/store`: the two `TestSupersedeMultiProductionDefaultsDoNotPanic` production-defaults literals, `spine_test.go`'s `newSpineTestStore` (prefix applied once at the helper so its ~30 call sites stay untouched), and `reindex_test.go`'s 15 `const src, tgt = "…", "…"` (and two/three-name variant) declarations, converted to `:=` sourced from `testCollection()`.
- Found and fixed a second, unprefixed `store.New` call site in `internal/e2e/spine_review_test.go` (`newSpineReviewStore`) that the plan's Task 1 `<files>`/`<read_first>` list did not mention — routed it through the same seam so the plan's own must-have ("no test constructs a store over a raw collection name") holds for the whole package, not just `harness_test.go`.
- Verified two consecutive `go test ./internal/store/... -count=1` runs against the SAME already-used shared Qdrant both pass (the idempotency proof), and `go test ./... -count=1` passes end to end with the shared address set.

## Task Commits

1. **Task 1: End-to-end "an unprefixed collection name fails the test" — the seam, proven on one package (tracer)** - `052fdfe6` (feat)
2. **Task 2: Route every remaining hardcoded collection name in internal/store through the seam** - `9ecc64e1` (feat)

## Files Created/Modified

- `internal/store/store_test.go` - `newTestStore` (with variadic `opts ...Option`), `testStore` routed through it, the two production-defaults literals prefixed
- `internal/store/spine_test.go` - `newSpineTestStore` applies `testCollection()` once at the top and routes construction through `newTestStore`; its ~30 call sites unchanged
- `internal/store/reindex_test.go` - `seedSource` and every `New(c, …)` construction routed through `newTestStore`; all 15 `const src, tgt`-family declarations converted to `testCollection()`-sourced `:=`
- `internal/server/tools_test.go` - `newTestStore` added; `testDepsWithStore` routed through it; the `ENGRAM_QDRANT_COLLECTION` setenv literal routed through `testCollection()` directly
- `internal/e2e/harness_test.go` - `testCollectionPrefix`/`testCollection`/`newTestStore` added; `startServer`'s per-port generator now routes through `testCollection()`
- `internal/e2e/spine_review_test.go` - `newSpineReviewStore` routed through `newTestStore`; its three call sites now pass an already-`testCollection()`-prefixed name (shared with the subprocess's `ENGRAM_QDRANT_COLLECTION` env var)
- `internal/retrievaleval/retrieval_eval_test.go` - `testCollectionPrefix`/`testCollection`/`newTestStore` added; `newTestcontainerStore` re-signatured to `(t testing.TB, addr string, dim uint64) *store.Store` and routed through the seam
- `.planning/phases/01-gate-ci-integrity/deferred-items.md` - logged the pre-existing, out-of-scope `internal/keylinks` rumdl MD041 lint failure

## Decisions Made

- **`internal/e2e/spine_review_test.go` was folded into scope** even though Task 1's `<files>` list named only `harness_test.go`. `newSpineReviewStore` is a second real `store.New` call site in the same package, and leaving it unprefixed would have made the plan's own must-have false for the shipped code. Documented as a Rule 2 (auto-add missing critical functionality) deviation below.
- **`internal/store`'s `newTestStore` alone carries a variadic `opts ...Option` parameter.** `newSpineTestStore` needs `WithClock`/`WithAuthz` passthrough for a handful of its ~30 call sites; extending only this package's seam signature (rather than inventing a second construction path) keeps every construction — opted or not — going through the one function.
- **`internal/retrievaleval`'s `newTestcontainerStore` gained a `t testing.TB` parameter and lost its `error` return.** Its sole call site is already inside a `t.Run` closure with `t` in scope, so threading it in (per the plan's own guidance for sites with no test handle) let the function `t.Fatalf` internally like every other seam call site, rather than dropping the runtime assertion for lack of a handle.
- **`internal/e2e`'s `newSpineReviewStore` does NOT apply the prefix internally** (unlike `internal/store`'s `newSpineTestStore`). The same literal collection name is also handed to the built binary's `ENGRAM_QDRANT_COLLECTION` env var via `pruneEnv`; applying the prefix inside the helper would silently produce two different collection names (one for the Go-side store, one for the subprocess) since the caller's own `collection` variable would stay unprefixed. Callers instead call `testCollection()` once when defining `collection` and pass that single value to both consumers.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical Functionality] Routed `internal/e2e/spine_review_test.go`'s `newSpineReviewStore` through the seam**
- **Found during:** Task 1
- **Issue:** The plan's Task 1 `<files>`/`<read_first>` list covered only `internal/e2e/harness_test.go`'s per-port generator, but `internal/e2e/spine_review_test.go` has its own, independent `store.New(c, collection)` call site (`newSpineReviewStore`) used by three tests with hardcoded `"e2e_prune_expired"`/`"e2e_prune_expired_empty"`/`"e2e_phase_acceptance"` names — a second construction path the plan's own must-have ("No test constructs a store over a raw collection name: every construction goes through the package's seam") required covered.
- **Fix:** Routed `newSpineReviewStore`'s construction through `newTestStore`; updated its three call sites to pass an already-`testCollection()`-prefixed name (needed because the same literal is also handed to the subprocess's `ENGRAM_QDRANT_COLLECTION` env var via `pruneEnv`, so the Go-side store and the built-binary subprocess must agree on one literal).
- **Files modified:** `internal/e2e/spine_review_test.go`
- **Verification:** `go test ./internal/e2e/... -count=1` passes against the shared Qdrant (all three affected tests — `TestE2EPruneExpiredPreviewsBeforeApply`, `TestE2EPruneExpiredPreviewZeroEligible`, `TestE2EPhaseAcceptance` — PASS).
- **Committed in:** `052fdfe6` (Task 1 commit)

**2. [Rule 3 - Blocking] Logged an out-of-scope pre-existing lint failure instead of fixing it**
- **Found during:** Task 2 (`task lint` verification)
- **Issue:** `rumdl` MD041 fails on `internal/keylinks/testdata/{bad,good}_key_links.md` — both intentionally start with GSD-style YAML frontmatter rather than an H1 heading, since they are keylinks-parser fixtures shaped like real PLAN.md files. Confirmed pre-existing at base commit `408f7db4` (from plan 01-01, unrelated to any file this plan touched).
- **Fix:** Not fixed (out of scope per the SCOPE BOUNDARY rule); logged to `.planning/phases/01-gate-ci-integrity/deferred-items.md`.
- **Files modified:** `.planning/phases/01-gate-ci-integrity/deferred-items.md` (new)
- **Verification:** `golangci-lint run ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/...` reports `0 issues.` — the only `task lint` failure is this pre-existing, unrelated markdown issue.
- **Committed in:** `9ecc64e1` (Task 2 commit)

---

**Total deviations:** 2 (1 Rule 2 auto-add, 1 Rule 3 scope-boundary log-and-defer)
**Impact on plan:** The Rule 2 fix closes a genuine gap in the plan's own must-have coverage with no scope creep beyond the same seam pattern already being applied elsewhere in the same package. The Rule 3 item is pure logging — zero code impact.

## Verbatim Evidence

**The four prefix values (pairwise distinct, none a leading substring of another):**
```
internal/store/store_test.go:62:            const testCollectionPrefix = "store_"
internal/server/tools_test.go:140:          const testCollectionPrefix = "server_"
internal/e2e/harness_test.go:67:            const testCollectionPrefix = "e2e_"
internal/retrievaleval/retrieval_eval_test.go:60: const testCollectionPrefix = "retrievaleval_"
```

**Red-proof (internal/server, unprefixed literal passed to newTestStore):**
```
$ ENGRAM_QDRANT_TEST_ADDR=localhost:6334 go test ./internal/server/ -run TestStoreMemoryUsesInjectedClock -v
=== RUN   TestStoreMemoryUsesInjectedClock
    tools_test.go:529: collection name "mem_eval_test" does not carry this package's prefix "server_": route it through testCollection()
--- FAIL: TestStoreMemoryUsesInjectedClock (0.02s)
FAIL	github.com/seanb4t/engram/internal/server	0.276s
FAIL
```
(reverted immediately after; `go build ./...` clean afterward)

**Task 1 three-package rehearsal (server + e2e + retrievaleval against one shared Qdrant):**
```
$ ENGRAM_QDRANT_TEST_ADDR=localhost:6334 ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/... -count=1
ok  	github.com/seanb4t/engram/internal/server	15.113s
ok  	github.com/seanb4t/engram/internal/e2e	3.720s
ok  	github.com/seanb4t/engram/internal/retrievaleval	0.272s
EXIT: 0
```

**Task 2 idempotency proof — two consecutive runs against the SAME running Qdrant, both PASS:**
```
$ ENGRAM_QDRANT_TEST_ADDR=localhost:6334 ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -count=1   # run 1
ok  	github.com/seanb4t/engram/internal/store	23.718s

$ ENGRAM_QDRANT_TEST_ADDR=localhost:6334 ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -count=1   # run 2, same container
ok  	github.com/seanb4t/engram/internal/store	24.083s
```

**Collection namespace after both runs — every name carries its package's prefix, no bare pre-sweep name survives:**
```
$ curl -s http://localhost:6333/collections | jq -r '.result.collections[].name' | sort
e2e_61980
e2e_62253
e2e_62379
server_mem_eval_test
server_mem_load_once_test
store_mem_eval_test
store_mem_eval_test_prod_defaults_1
store_mem_eval_test_prod_defaults_2
```

**Full-suite rehearsal, shared address set:**
```
$ ENGRAM_QDRANT_TEST_ADDR=localhost:6334 ENGRAM_REQUIRE_QDRANT=1 go test ./... -count=1
ok  	github.com/seanb4t/engram/cmd/engram	2.336s
ok  	github.com/seanb4t/engram/internal/auth	0.456s
ok  	github.com/seanb4t/engram/internal/authz	0.154s
ok  	github.com/seanb4t/engram/internal/config	0.166s
ok  	github.com/seanb4t/engram/internal/e2e	5.216s
ok  	github.com/seanb4t/engram/internal/embed	0.273s
ok  	github.com/seanb4t/engram/internal/keylinks	0.132s
ok  	github.com/seanb4t/engram/internal/openaiurl	0.136s
ok  	github.com/seanb4t/engram/internal/retrievaleval	0.388s
ok  	github.com/seanb4t/engram/internal/server	15.364s
ok  	github.com/seanb4t/engram/internal/shortid	0.136s
ok  	github.com/seanb4t/engram/internal/store	23.514s
ok  	github.com/seanb4t/engram/internal/summarize	0.194s
ok  	github.com/seanb4t/engram/internal/surfaces	0.687s
ok  	github.com/seanb4t/engram/internal/telemetry	0.452s
ok  	github.com/seanb4t/engram/internal/webauth	0.509s
```

**Overall verification block:**
- `rg -n 'ListCollections' internal/` — empty (no enumerate-then-delete-all teardown introduced)
- `rg -c 'const src, tgt' internal/store/reindex_test.go` — zero matches
- `rg -n 'New(c, "' internal/store/reindex_test.go` and `rg -n 'New(dialTestClient' internal/store/store_test.go` — both empty
- `gofmt -l .` — prints nothing
- `go vet ./...` — clean
- `golangci-lint run ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/...` — `0 issues.`
- `task license:check` — 295 valid, 0 invalid
- `git diff --stat go.mod go.sum` — empty (no dependency changes)

## Issues Encountered

None beyond the two documented deviations above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Every Qdrant collection any test in the four Qdrant-backed packages creates now carries its own package's prefix, enforced at runtime by `newTestStore` rather than by review — the property plan 01-04 left as a convention is now structural.
- `internal/store` has zero remaining raw collection names; the idempotency proof (two consecutive runs against the same already-used shared instance) is live evidence, not an assumption.
- **Real GHA verification of the shared-Qdrant CI wiring itself was already covered by plan 01-04's own SUMMARY** — this plan adds no new CI-side wiring, only test-side namespace enforcement, so no additional CI-run observation is needed beyond what 01-04 already flagged.
- One pre-existing, unrelated `internal/keylinks` rumdl lint failure remains open, tracked in `deferred-items.md` — not introduced or touched by this plan.

---
*Phase: 01-gate-ci-integrity*
*Completed: 2026-08-13*
