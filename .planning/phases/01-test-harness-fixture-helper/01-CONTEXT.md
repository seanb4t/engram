# Phase 1: Test Harness & Fixture Helper - Context

**Gathered:** 2026-09-18
**Status:** Ready for planning

<domain>
## Phase Boundary

Deliver the test infrastructure every later phase of milestone 2026-09-18.01 proves its fix with:
(1) one shared, reusable oversized-fixture helper — extracted from
`TestListScopesFullPayloadsOverGRPCLimit`'s (#583) seeding shape — that seeds a real-Qdrant scope
whose full payloads exceed a named receive limit, in two shapes (many small records, a few large
records), self-asserting the logical byte count; and (2) one shared Qdrant client constructor used
by production and by every test, so a regression test proves the *mechanism* bounds responses
rather than a client-side accident. Covers REQ-oversized-fixture-helper and REQ-test-client-parity.
Keeping `internal/store`'s CI job green as fixtures land is in scope; the closing evidence for
REQ-ci-store-green / #497 is Phase 5's. No read path is fixed here.

</domain>

<decisions>
## Implementation Decisions

### Framing principle (user-stated, applies to every decision below)

- **D-00:** Choose by idiom and long-term maintenance, never by effort. Prefer upstream vocabulary
  over our own wrappers, put code in the package that owns the concept, and reject an option only
  for a real correctness/maintenance reason. Engram memory `1w3h5sy56m`; complements rule
  `xvqj44e5mk`.

### Shared client constructor

- **D-01:** Export the constructor from `internal/store` — e.g.
  `store.NewQdrantClient(host string, port int, opts ...grpc.DialOption) (*qdrant.Client, error)`.
  It applies the shared base dial options (today: `grpc.WithStatsHandler(otelgrpc.NewClientHandler())`)
  and appends caller options. `internal/server/tools.go` `storeFromConfig` calls it instead of
  `qdrant.NewClient`. `store` is already "the one holder" in the client-holder gate, so the dial
  options live beside the code whose responses they bound. — **Reversibility:** costly — every
  test and the composition root import it once converged.
- **D-02:** The receive limit is passed in upstream vocabulary —
  `grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(n))` through the `...grpc.DialOption`
  pass-through (same shape as `qdrant.Config.GrpcOptions`). No bespoke `WithMaxRecvBytes` wrapper.
- **D-03:** No production behavior change in Phase 1. Production keeps otelgrpc only; tests gain
  the same handler (no-op without a tracer provider). The `MaxCallRecvMsgSize` backstop lands in
  Phase 5 (REQ-recv-limit-backstop), set in exactly one place, after the regression tests exist.

### Named receive limit & fixture sizing

- **D-04:** Regression tests name **4 MiB (`4 << 20`)** explicitly — the ceiling production
  effectively runs at today and the value #583's test already uses. Expose it as a named constant
  in `storetest`; each test passes it explicitly to both the dial helper and the seeder. Tests keep
  pinning 4 MiB after Phase 5 raises the production backstop, which is what proves the mechanism
  (not the backstop) bounds responses. Consequence for Phase 3: its page byte budget must stay
  under this limit (a simple constant is fine).
- **D-05:** The seeder derives record count/size from the limit plus a margin (target ≥ 1.25× the
  limit) and self-asserts the logical byte total exceeds the limit (the #583 `n*contentBytes`
  check, generalized). Two shapes:
  - **many-small** — sized so ONE page within `maxListLimit` (1000) overflows the limit (≈5 KiB
    records at 4 MiB).
  - **few-large** — the #583 shape (records ≈ limit/32, i.e. 128 KiB at 4 MiB).
  Every individual record stays well under the limit; a single record larger than the limit is a
  Phase 3 content-cap question (see Deferred).

### Harness structure

- **D-06:** New package `internal/store/storetest` (the `net/http/httptest` idiom; repo precedent
  for a non-`_test.go` shared test-support package: `internal/testhttp`). It imports `store` and
  owns: the dial-with-required-limit helper, the oversized seeder, container lifecycle
  (`storetest.Main(m)`-style), `requireQdrant`, and the single Qdrant image tag.
- **D-07:** Because `storetest` imports `store`, `internal/store`'s in-package tests cannot import
  it (import cycle). Oversized regression tests in `internal/store` are therefore black-box
  `package store_test` files — Go's sanctioned cycle-breaker (`go help test`), already used here by
  `internal/store/spine_forgery_test.go`. Use an `export_test.go` only where an internal must be
  exposed. In-package tests keep dialing through `NewQdrantClient` directly.
- **D-08:** The seeder writes through public `Store.Upsert` (the real write path — schema stamping,
  owner, etc.). Raw `*qdrant.Client` writes are rejected: they bypass stamping and would trip the
  client-holder write gate. Cleanup uses public `Store.DeleteAll(ctx, scope, Authenticated(owner))`
  registered via `t.Cleanup` — NOT `DeleteAllRaw`, which is a test-only method in package `store`
  (`store_test.go:1748`) invisible to `storetest`.
- **D-09:** `storetest` holds a `*qdrant.Client`, so it joins `qdrantClientHolderAllowlist`
  (`schemaversion_stamp_gate_test.go:756`) with a justification (test-support dialer, never
  transmits writes itself).

### Convergence scope & CI

- **D-10:** Full consolidation. All 11 test `qdrant.NewClient` call sites converge on the shared
  constructor (via `storetest` or, inside `package store`, `NewQdrantClient` directly), AND the
  duplicated harness collapses into `storetest`: 4 `TestMain`s, 3 `requireQdrant`s, 4 image-tag
  copies. `internal/store`'s `TestMain` moves to a `package store_test` file; in-package tests get
  the address via `ENGRAM_QDRANT_TEST_ADDR` — the existing contract (storetest exports the booted
  container's address through it). Behaviors to preserve verbatim: `ENGRAM_REQUIRE_QDRANT`
  fail-closed parsing (invalid value is an error, never coerced to false), 3-minute bounded
  startup, 30-second bounded terminate, skip-with-message when no Qdrant, per-package collection
  prefixes (prior-milestone D-16, `testCollectionPrefix`), `qdrantTOCTOUVerifiedVersion` staying a
  SEPARATE constant from the image tag, and CI's `services.qdrant.image` staying byte-identical to
  the tag (update the CI comment that points at `store_test.go`'s `qdrantImageTag`).
- **D-11:** Enforce convergence with an AST gate in the style of
  `TestQdrantClientIsHeldOnlyByStorePackage`: `qdrant.NewClient` may be called only inside
  `store.NewQdrantClient`, scanning repo-wide INCLUDING `_test.go` files. The existing composition-
  root check (`qdrantClientLocalNames`, which keys on `name, err := qdrant.NewClient(...)` in
  `tools.go`) must be updated for the new call shape, and the allowlist set-equality re-derived.
- **D-12:** The `testing.Short()` skip lives once, inside the `storetest` seeder (following #583's
  precedent), so every oversized test inherits it. Oversized tests do not call `t.Parallel`, so
  peak payload in Qdrant stays ≈ one fixture per package. CI runs them (`go test ./...`, no
  `-short`).

### Claude's Discretion

- Exact `storetest` API names and signatures (`Dial`, `Main`, seeder name, shapes as an enum vs
  two functions), and the seeder's return shape (seeded IDs / scope / owner for assertions).
- The exact margin (≥ 1.25×) and per-shape record sizes, within D-05.
- How `internal/e2e` and `internal/retrievaleval` harness specifics adapt to `storetest.Main`
  (e.g. e2e's full-server boot) while preserving D-10's behaviors.
- Phase 1 red evidence (required by `TestRedEvidencePatchesAreLive` once this phase directory
  exists): e.g. a patch reverting `ListScopes`' payload selector so the migrated #583 test goes
  RED, and a patch adding a bare `qdrant.NewClient` in a `_test.go` so the D-11 gate goes RED.
  Register in `redEvidenceDirs` after the phase's last plan.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & milestone research
- `.planning/REQUIREMENTS.md` — REQ-oversized-fixture-helper, REQ-test-client-parity (this
  phase); REQ-ci-store-green, REQ-recv-limit-backstop (Phase 5, constrain this phase)
- `.planning/ROADMAP.md` — Phase 1 goal and sequencing
- `.planning/research/SUMMARY.md` — root cause, phase ordering, Pitfall 5 (fixture CI pressure)
- `.planning/research/PITFALLS.md` — fixture-size-vs-compression reasoning, CI pressure
- `.planning/research/ARCHITECTURE.md`, `.planning/research/STACK.md` — client construction,
  `qdrant.Config.GrpcOptions`, AST gates

### Code the phase extracts from / converges
- `internal/store/store_test.go:1793-1840` — `TestListScopesFullPayloadsOverGRPCLimit` (fixture
  shape to extract: 40 × 128 KiB, `n*contentBytes <= 4<<20` self-check, `testing.Short()` skip)
- `internal/store/store_test.go:29-42` — `qdrantImageTag`, `qdrantTOCTOUVerifiedVersion`
- `internal/store/store_test.go:82-230` — `newTestStore` prefix seam, `requireQdrant`, `TestMain`,
  `terminateQdrant`, `dialTestClient`, `TestRequireQdrant`
- `internal/store/store_test.go:1748` — `DeleteAllRaw` (test-only; not reachable from storetest)
- `internal/store/store.go:795` `Upsert`, `:2626` `DeleteAll` — public seeding/cleanup path
- `internal/server/tools.go:110-135` — `storeFromConfig`, the production client construction
- Test `qdrant.NewClient` sites (11): `internal/store/{store_test.go:190, migrate_status_test.go:202,
  revert_test.go:557, migrate_converge_test.go:446, schemaversion_recallgate_test.go:940,
  migrate_faultinject_test.go:249}`, `internal/server/{tools_test.go:377, tools_test.go:6070,
  schemaversion_wire_test.go:195}`, `internal/e2e/spine_review_test.go:73`,
  `internal/retrievaleval/retrieval_eval_test.go:336`
- Duplicated harness: `internal/server/tools_test.go:174,211`, `internal/e2e/harness_test.go:96,108,139`,
  `internal/retrievaleval/retrieval_eval_test.go:33,353,364`

### Gates this phase must keep green / extend
- `internal/store/schemaversion_stamp_gate_test.go:756` — `qdrantClientHolderAllowlist`;
  `:874` `qdrantClientLocalNames`; `TestQdrantClientIsHeldOnlyByStorePackage`
- `internal/store/redevidence_harness_test.go:106` — `redEvidenceDirs` (currently empty)
- `internal/store/schemaversion_recallgate_test.go` — recall-gate AST test (not changed here, but
  later phases add helpers to `recallTransmitters`)

### Precedents
- `internal/store/spine_forgery_test.go` — existing `package store_test` external test file
- `internal/testhttp/reuse.go` — non-`_test.go` shared test-support package precedent
- `.github/workflows/ci.yaml:20-130` — shared `services:` Qdrant (#497/#498),
  `ENGRAM_QDRANT_TEST_ADDR: localhost:6334`, image pin comment

### Rules & memories
- Rule `m45p2b4bp7` — test our paths against a limit we name; never assert grpc-go's default
- Rule `xvqj44e5mk` — prefer established/idiomatic over hand-rolled
- Rule `n6m4as49mr` — explicit pathspec on `git commit`
- Memory `7r10s08k9q` — 4 MiB receive-cap root cause, #583 fix, #585 siblings
- Memory `xb8y5pk6eh` — internal/store read-path facts (no package-wide Scroll gate, count-only bounds)
- Memory `1w3h5sy56m` — idiom-over-effort framing (D-00)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `TestListScopesFullPayloadsOverGRPCLimit`: the fixture shape and byte self-check to generalize.
- `requireQdrant` / `TestMain` / `terminateQdrant` in `store_test.go`: the most complete harness
  copy (fail-closed parsing, bounded start/stop) — the basis for `storetest`.
- `newTestStore` + `testCollection`: per-package collection-prefix seam storetest must honor.
- `internal/testhttp`: shows how a shared test-support package stays free of test-framework
  imports in the production graph.

### Established Patterns
- Name-keyed AST gates enforce store boundaries (`TestQdrantClientIsHeldOnlyByStorePackage`,
  recall-gate test) — D-11 follows this style.
- `testing.Short()` skip for multi-MiB fixtures (#583).
- `ENGRAM_QDRANT_TEST_ADDR` is the shared-instance contract CI already sets.

### Integration Points
- `storeFromConfig` (tools.go) switches to `store.NewQdrantClient`.
- The client-holder allowlist and `qdrantClientLocalNames` composition-root check must change in
  the same change as the constructor.
- `redEvidenceDirs` gains a Phase 1 entry once red-evidence patches exist.

</code_context>

<specifics>
## Specific Ideas

- 4 MiB is named by us (a constant in storetest), never inferred from grpc-go — rule `m45p2b4bp7`.
- The #583 test migrates onto the helper and stays equivalent in detection power (it is the
  natural red-evidence target).

</specifics>

<deferred>
## Deferred Ideas

- **Third fixture shape — a single record larger than the limit.** Only meaningful once decision A
  (content size cap, REQ-content-cap-decided) is made → Phase 3.
- **Store byte budget derived from the client's configured limit** (vs a fixed constant under
  4 MiB) → Phase 3 design; D-04 only requires the budget stay under the tests' named 4 MiB.
- **REQ-ci-store-green closing evidence and closing #497** → Phase 5.

</deferred>

---

*Phase: 01-test-harness-fixture-helper*
*Context gathered: 2026-09-18*
