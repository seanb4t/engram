---
phase: 01-test-harness-fixture-helper
plan: 01
subsystem: testing
tags: [qdrant, testcontainers, grpc, storetest, ast-gate]

requires: []
provides:
  - "store.NewQdrantClient(host, port, opts...) — the single Qdrant client constructor for production and every test"
  - "internal/store/storetest package: RecvLimit, QdrantImage, Run/terminate lifecycle, RequireQdrant/SkipOrFailNoQdrant, Addr/EnvAddr/ContainerBooted, AssertSharedAddressHonored, Dial"
  - "storetest.SeedOversized: two-shape (ManySmall/FewLarge) oversized fixture seeder writing only through Store.Upsert/Store.DeleteAll"
  - "three-entry qdrantClientHolderAllowlist with a D-13 generalized never-writes check over every holder except store.go"
  - "storetest registered in qdrantBackedPackages (5 packages, 20 ordered pairs) for the collection-prefix/seam gates"
affects: ["01-02", "01-03", "01-04", "01-05"]

actuals:
  tokens: 15919
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "net/http/httptest-shaped non-_test.go test-support package (storetest), mirroring internal/testhttp"
    - "base-then-caller-options Qdrant dial construction (store.NewQdrantClient)"
    - "named receive limit passed in upstream grpc vocabulary (grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(n))), never a bespoke wrapper"
    - "AST-gate generalization: a per-holder write-check loop over an allowlist, keyed by the constructor call shape rather than a hardcoded file path"

key-files:
  created:
    - internal/store/storetest/storetest.go
    - internal/store/storetest/storetest_test.go
    - internal/store/storetest/seed.go
    - internal/store/storetest/seed_test.go
  modified:
    - internal/store/store.go
    - internal/server/tools.go
    - internal/store/schemaversion_stamp_gate_test.go
    - internal/store/collectionprefix_conformance_test.go

key-decisions:
  - "NewQdrantClient lives in internal/store (D-01): store is already the one qdrant.Client holder the write-boundary gate scans, so the shared dial options live beside the code they bound."
  - "The named receive limit travels as grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(n)) — upstream vocabulary only, appended LAST after any caller options (D-02, D-04)."
  - "storetest is a non-_test.go package (net/http/httptest idiom, D-06) and deliberately imports testing/testcontainers, unlike internal/testhttp."
  - "D-13: generalized TestQdrantClientIsHeldOnlyByStorePackage's never-issues-a-write check from a hardcoded tools.go path to a loop over every qdrantClientHolderAllowlist entry except store.go, gate-enforcing storetest's D-08 write restriction instead of leaving it asserted by review only."

requirements-completed: [REQ-oversized-fixture-helper, REQ-test-client-parity]

coverage:
  - id: D1
    description: "store.NewQdrantClient is the single Qdrant client constructor; production (storeFromConfig) and storetest.Dial both build through it with unchanged production dial options"
    requirement: "REQ-test-client-parity"
    verification:
      - kind: unit
        ref: "internal/store/storetest#TestDialOptions"
        status: pass
      - kind: integration
        ref: "internal/server#TestBuildDepsFromEnvLoadsConfigOnce"
        status: pass
      - kind: integration
        ref: "internal/server#TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce"
        status: pass
    human_judgment: false
  - id: D2
    description: "storetest.Dial requires a caller-named receive limit (never grpc-go's default), appended last after any caller options"
    requirement: "REQ-test-client-parity"
    verification:
      - kind: unit
        ref: "internal/store/storetest#TestRecvLimitIsFourMiB"
        status: pass
      - kind: unit
        ref: "internal/store/storetest#TestDialOptions"
        status: pass
      - kind: integration
        ref: "internal/store/storetest#TestDialRoundTrip"
        status: pass
    human_judgment: false
  - id: D3
    description: "storetest owns the single ENGRAM_REQUIRE_QDRANT parser and container lifecycle, failing closed rather than skipping when Qdrant is required and unavailable"
    requirement: "REQ-test-client-parity"
    verification:
      - kind: unit
        ref: "internal/store/storetest#TestRequireQdrant"
        status: pass
      - kind: unit
        ref: "internal/store/storetest#TestSkipOrFailNoQdrantSkipsWhenNotRequired"
        status: pass
    human_judgment: false
  - id: D4
    description: "SeedOversized seeds ManySmall/FewLarge fixtures through Store.Upsert only, self-asserting the logical total strictly exceeds the named limit"
    requirement: "REQ-oversized-fixture-helper"
    verification:
      - kind: unit
        ref: "internal/store/storetest#TestLayout"
        status: pass
      - kind: unit
        ref: "internal/store/storetest#TestCheckOversized"
        status: pass
      - kind: unit
        ref: "internal/store/storetest#TestValidateSpec"
        status: pass
      - kind: integration
        ref: "internal/store/storetest#TestSeedOversizedShapes"
        status: pass
    human_judgment: false
  - id: D5
    description: "Fixture identity (scope/owner) is freshly uuid-derived per call, and CreatedAt is strictly increasing/second-distinct in write order"
    requirement: "REQ-oversized-fixture-helper"
    verification:
      - kind: unit
        ref: "internal/store/storetest#TestFixtureIdentityIsUnique"
        status: pass
      - kind: unit
        ref: "internal/store/storetest#TestCreatedAtStrictlyIncreasing"
        status: pass
      - kind: integration
        ref: "internal/store/storetest#TestSeedOversizedShapes"
        status: pass
    human_judgment: false
  - id: D6
    description: "The client-holder gate derives three holders (store.go, tools.go, storetest.go) and write-checks every holder except store.go (D-13), proven live by an injected-write RED"
    requirement: "REQ-test-client-parity"
    verification:
      - kind: integration
        ref: "internal/store#TestQdrantClientIsHeldOnlyByStorePackage"
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-09-18
status: complete
---

# Phase 1 Plan 1: End-to-End Shared Client, storetest Harness, and Oversized Fixture Seeder Summary

**`store.NewQdrantClient` is now the single Qdrant dial path for production and every test; `internal/store/storetest` ships a real-Qdrant-verified lifecycle, dial helper, and two-shape oversized fixture seeder, with the client-holder gate generalized to write-check every allowlisted holder.**

## Performance

- **Duration:** ~45 min
- **Completed:** 2026-09-18
- **Tasks:** 3 completed
- **Files:** 8 changed (4 created, 4 modified)

## Accomplishments

- `store.NewQdrantClient(host, port, opts...)` — the one exported constructor: base `grpc.WithStatsHandler(otelgrpc.NewClientHandler())` first, then caller opts appended, matching qdrant-go-client's own base-then-`Config.GrpcOptions` order.
- `internal/store/storetest` — a new non-`_test.go` test-support package (the `net/http/httptest` idiom): `RecvLimit` (4 MiB, `4194304` exactly), `QdrantImage`, `Run`/`terminate` container lifecycle, the single `ENGRAM_REQUIRE_QDRANT` parser (`RequireQdrant`), `SkipOrFailNoQdrant`, `Addr`/`EnvAddr`/`ContainerBooted`, `AssertSharedAddressHonored`, and `Dial` (dials through `store.NewQdrantClient`, receive limit appended LAST after caller options).
- `storetest.SeedOversized` — seeds ManySmall (1000 records) or FewLarge (`limit/32` records) fixtures through `Store.Upsert` only, self-asserting the logical byte total strictly exceeds the named limit (`layout`/`checkOversized`, ceiling division, ≥1.25x margin); cleanup via `Store.DeleteAll` registered before the first write; the phase's single `-short` skip lives in the seeder (D-12).
- `storeFromConfig` (production composition root) now builds its client via `store.NewQdrantClient(host, port)` with zero dial-option behavior change (D-03).
- `TestQdrantClientIsHeldOnlyByStorePackage`'s never-issues-a-write check generalized (D-13) from a `tools.go`-only block to every `qdrantClientHolderAllowlist` entry except `store.go`; `storetest` joined the allowlist and the `qdrantBackedPackages` set (now 5 packages, 20 ordered pairs).

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end shared client: store.NewQdrantClient, storetest lifecycle and Dial, one real-Qdrant round trip** — `5aa5fd59` (refactor)
2. **Task 2: Two-shape oversized seeder: SeedOversized writes many-small or few-large fixtures that self-assert they exceed the named limit** — `c6c306ec` (test)
3. **Task 3: Production composition root on the shared constructor, holder gate recognizes it, and the never-writes check covers every allowlisted holder (D-13)** — `4da5eb96` (refactor)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP)

## Files Created/Modified

- `internal/store/store.go` — adds `NewQdrantClient`, imports `google.golang.org/grpc` and the otelgrpc alias.
- `internal/store/storetest/storetest.go` — lifecycle, parser, dial helper (301 lines).
- `internal/store/storetest/storetest_test.go` — `TestMain`, seam, pure tests + `TestDialRoundTrip` (244 lines).
- `internal/store/storetest/seed.go` — `Shape`, `Spec`, `Fixture`, `layout`, `checkOversized`, `validateSpec`, `fixtureIdentity`, `createdAtFor`, `SeedOversized` (252 lines).
- `internal/store/storetest/seed_test.go` — `TestLayout`, `TestCheckOversized`, `TestValidateSpec`, `TestFixtureIdentityIsUnique`, `TestCreatedAtStrictlyIncreasing`, `TestSeedOversizedShapes` (275 lines).
- `internal/server/tools.go` — `storeFromConfig` switched to `store.NewQdrantClient(host, port)`; `qdrant`/`otelgrpc`/`grpc` imports dropped (all three were used only in the replaced block).
- `internal/store/schemaversion_stamp_gate_test.go` — third allowlist entry (`storetest.go`), `fileRefsQdrantClient`/`qdrantClientLocalNames` recognize `store.NewQdrantClient`, D-13 generalized write-check loop.
- `internal/store/collectionprefix_conformance_test.go` — `storetest` added to `qdrantBackedPackages`; "four packages"-style doc-comment wording replaced with count-free wording per the task's action item.

## Decisions Made

- **Seeding wall time (RESEARCH Assumption A1, now measured, not projected):** FewLarge (40 x 131072 bytes) seeded in ~77–170 ms across runs; ManySmall (1000 x 5243 bytes) seeded in ~2.07–2.4 s across runs — confirms the ~single-digit-seconds-per-fixture budget RESEARCH projected, well within the phase's stated cost tolerance. No goroutine-based speedup was needed.
- `Dial` binds its client via assignment (`c, err := store.NewQdrantClient(...)`) rather than a direct return, specifically so the D-13 write-check has a client-bound identifier in `storetest.go` to verify against — matches the plan's explicit rationale.
- `SeedOversized` resolves `Scope`/`Owner` independently (Template value if non-empty, else a fresh `fixtureIdentity` value for that field alone) rather than an all-or-nothing swap, satisfying "an empty Template Scope or Owner gets a fresh generated value" without discarding a caller-supplied Scope when only Owner is empty (or vice versa).
- Re-worded two `seed.go`/`storetest.go` doc comments (`testing.Short()`, `grpc.MaxCallRecvMsgSize(recvLimit)`) so the plan's exact-count `rg` acceptance greps match the literal call site only, not the prose describing it.

## Deviations from Plan

None — plan executed exactly as written. Two RED observations were explicitly required by the plan and are recorded as normal flow, not deviations:

1. **Pre-existing-gate RED (Task 3, before the gate update):** `go test ./internal/store/ -run '^TestQdrantClientIsHeldOnlyByStorePackage$'` failed with `allowlist entry internal/server/tools.go ... has no matching derived holder — stale allowlist entry` and `found no *qdrant.Client-bound identifier in .../tools.go` immediately after switching `storeFromConfig` onto `store.NewQdrantClient`, before `fileRefsQdrantClient`/`qdrantClientLocalNames` were updated to recognize the new call shape. Resolved by the same task's gate update; the test now passes green.
2. **D-13 injected-write RED (Task 3):** a temporary `_, _ = c.Upsert(context.Background(), &qdrant.UpsertPoints{})` inserted into `storetest.go`'s `Dial`, right after the client binding, produced `--- FAIL: TestQdrantClientIsHeldOnlyByStorePackage` naming `storetest.go`, `Upsert`, and the enclosing function `Dial` — proving the generalized write-check actually fires on a non-`store.go` holder. Captured via `git diff -- internal/store/storetest/storetest.go` into a scratchpad patch and reverted with `git apply -R`; `git status --porcelain -- internal/store/storetest/storetest.go` printed nothing before continuing. (This is the observation plan 01-05 will package as a registered red-evidence patch — not done in this plan.)

**Total deviations:** 0. **Impact:** None — plan executed as written; both REDs above were required observations, not corrections.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `store.NewQdrantClient`, `storetest.Dial`, and `storetest.SeedOversized` are ready for plans 01-02/01-03/01-04 to converge the remaining 10 test `qdrant.NewClient` call sites and the 4 duplicated `TestMain`s onto, and for oversized read-path regression tests to use directly.
- `internal/store` stays green except the single expected `TestRedEvidencePatchesAreLive` red, which plan 01-05's Task 3 closes by registering this phase's red-evidence patches (including the D-13 injected-write shape captured above) in `redEvidenceDirs`.
- No blockers or concerns for the next plan.

---
*Phase: 01-test-harness-fixture-helper*
*Completed: 2026-09-18*

## Self-Check: PASSED

All 8 created/modified files verified present on disk; all 3 task commits (`5aa5fd59`, `c6c306ec`, `4da5eb96`) verified present in `git log --oneline --all`.
