# Phase 1: Test Harness & Fixture Helper - Research

**Researched:** 2026-09-18
**Domain:** Go test-infrastructure consolidation for a Qdrant-backed store (shared client constructor, shared oversized-fixture seeder, AST convergence gates)
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Framing principle (user-stated, applies to every decision below)

- **D-00:** Choose by idiom and long-term maintenance, never by effort. Prefer upstream vocabulary
  over our own wrappers, put code in the package that owns the concept, and reject an option only
  for a real correctness/maintenance reason. Engram memory `1w3h5sy56m`; complements rule
  `xvqj44e5mk`.

#### Shared client constructor

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

#### Named receive limit & fixture sizing

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

#### Harness structure

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

#### Convergence scope & CI

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

### Deferred Ideas (OUT OF SCOPE)

- **Third fixture shape — a single record larger than the limit.** Only meaningful once decision A
  (content size cap, REQ-content-cap-decided) is made → Phase 3.
- **Store byte budget derived from the client's configured limit** (vs a fixed constant under
  4 MiB) → Phase 3 design; D-04 only requires the budget stay under the tests' named 4 MiB.
- **REQ-ci-store-green closing evidence and closing #497** → Phase 5.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-oversized-fixture-helper | A shared real-Qdrant test helper seeds a scope whose full payloads exceed a named receive limit, self-asserting the logical (post-decompression) byte count as `TestListScopesFullPayloadsOverGRPCLimit` (#583) does, in two shapes — many small records and a few large records. Every read-path regression test in this milestone uses it, and each is RED against the pre-fix code. | Pattern 1/2 (Architecture Patterns) give the `storetest` package shape and the base-plus-caller dial-option composition it needs; the Code Examples' 11-site table and the 4-`TestMain` comparison table give the exact call sites and behavioral deltas the helper must absorb; Pitfall 7 gives a measured seeding-cost baseline (0.32s for the existing 40×128KiB fixture) to budget the new many-small shape against; Pitfall 1 establishes that this requirement's "RED against the pre-fix code" clause is enforced live today by `TestRedEvidencePatchesAreLive`, which is currently failing and must be closed by this phase's red-evidence registration. |
| REQ-test-client-parity | Test Qdrant clients are built through one shared constructor applying the same dial options as the production client, with each test naming its receive limit explicitly — so a regression test proves the mechanism keeps responses bounded regardless of any production backstop (`REQ-recv-limit-backstop`), and the independent `qdrant.NewClient` test call sites converge on it. | Pattern 1 verifies (via direct module-cache source reads) that `grpc.WithDefaultCallOptions`/`MaxCallRecvMsgSize` and `qdrant.Config.GrpcOptions` compose additively, which is what lets `store.NewQdrantClient`'s base-plus-caller-options design work; Pitfalls 3–5 give the exact AST-gate mechanics (`scanPackageDirForCalls`'s non-recursive scope, `TestQdrantClientIsHeldOnlyByStorePackage`'s hardcoded tools.go-only write-check, and the new logic D-11's gate needs) that determine what "converge" is mechanically checkable; the Code Examples' 11-site table classifies every site into the `NewQdrantClient`-direct vs. `storetest.Dial` convergence path required to satisfy this requirement. |
</phase_requirements>

## Summary

This phase is pure test-infrastructure consolidation, not new production behavior (D-03). Everything
needed is already visible in the working tree: an existing fixture (`TestListScopesFullPayloadsOverGRPCLimit`,
#583) to generalize, an existing harness (`TestMain`/`requireQdrant`/`dialTestClient`) to extract, and two
existing AST gates (`TestQdrantClientIsHeldOnlyByStorePackage`, the recall-gate scan) whose scanning
mechanics dictate exactly what the new `storetest` package and the new `store.NewQdrantClient` constructor
must look like to stay green — plus one gate this phase must add from scratch (D-11).

The critical live finding: **`TestRedEvidencePatchesAreLive` is failing RIGHT NOW** (verified by running it
against HEAD in this session — see Pitfall/Gate section below). `redEvidenceDirs` is empty while
`.planning/phases/01-test-harness-fixture-helper` already exists, and the harness's own empty-map guard
treats that combination as a defect, not a clean run. This is not a future risk to plan around — it is the
present state of `task test` on this branch, and Phase 1 cannot close without registering its two red-evidence
patches (D-12/discretion item) in `redEvidenceDirs`.

The 11 test `qdrant.NewClient` call sites split cleanly into two groups by package boundary, which is exactly
what D-06/D-07 already anticipate: 6 sites live in `package store` itself (in-package tests, which cannot
import `storetest` without an import cycle) and must call the new `store.NewQdrantClient` directly by its
bare name; the other 5 live in `internal/server`, `internal/e2e`, and `internal/retrievaleval` (separate
packages, no cycle) and converge onto a new `storetest.Dial`-shaped helper that itself calls
`store.NewQdrantClient`. Five of the six in-package sites wrap a custom `grpc.WithUnaryInterceptor` (fault
injection, side-effect counting, recall-filter capture) that `store.NewQdrantClient`'s
`opts ...grpc.DialOption` pass-through must accommodate unchanged.

**Primary recommendation:** Build `store.NewQdrantClient(host string, port int, opts ...grpc.DialOption) (*qdrant.Client, error)`
first (it is the single dependency every other task in this phase waits on), then `storetest` as a thin
package around it, then migrate the 11 call sites and the 4 `TestMain`s, then add the D-11 AST gate, and
close by registering red-evidence in `redEvidenceDirs` — in that order, because the AST gates (existing and
new) are the actual acceptance criteria, not a human read of the diff.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Shared Qdrant client construction (dial options, receive-limit pass-through) | `internal/store` (production package) | — | D-01: store is already "the one holder" in the client-holder gate; dial options bounding Qdrant responses belong beside the code that issues the requests they bound |
| Oversized-fixture seeding + container lifecycle (test-only) | `internal/store/storetest` (new, non-`_test.go` support package) | — | D-06: mirrors `net/http/httptest` and this repo's own `internal/testhttp` precedent — test-support code that is importable but carries no test-framework imports into the production graph |
| Composition-root client construction (production wiring) | `internal/server/tools.go` (`storeFromConfig`) | — | Unchanged this phase (D-03); already allowlisted as a client holder; switches its `qdrant.NewClient` call to `store.NewQdrantClient` |
| Convergence enforcement (AST gates) | `internal/store` (`_test.go` gate files) | — | Existing precedent (`schemaversion_stamp_gate_test.go`) already lives here; D-11's new gate is the same style, same package |

## Standard Stack

No new dependencies — this phase only reorganizes existing, already-vendored APIs.

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/qdrant/go-client` | v1.19.2 [VERIFIED: go.mod:21] | `qdrant.NewClient`, `Config.GrpcOptions` pass-through | Already the sole Qdrant client in this repo |
| `google.golang.org/grpc` | v1.83.2 [VERIFIED: go.mod:46] | `grpc.WithDefaultCallOptions`, `grpc.MaxCallRecvMsgSize`, `grpc.WithUnaryInterceptor` | Already imported; `store.NewQdrantClient`'s `opts ...grpc.DialOption` signature is this package's own vocabulary (D-02) |
| `go.opentelemetry.io/contrib/.../otelgrpc` | v0.71.0 [VERIFIED: go.mod:26] | `otelgrpc.NewClientHandler()` base dial option | Already the production client's only dial option (`tools.go:127`); moves into `store.NewQdrantClient`'s base opts unchanged (D-01) |
| `github.com/testcontainers/testcontainers-go/modules/qdrant` | v0.44.0 [CITED: SUMMARY.md Sources] | Ephemeral Qdrant container for `storetest.Main` | Already the harness's container provider in all 4 existing `TestMain`s |

No `go.mod` entries change. No `npm view`/`pip index`/`cargo search` verification applies — this is a
Go-only phase with zero new third-party packages, so the Package Legitimacy Gate is not applicable.

## Package Legitimacy Audit

**Not applicable.** This phase adds zero new external dependencies to `go.mod` — it only adds one new
in-repo package (`internal/store/storetest`) and one new in-repo exported function (`store.NewQdrantClient`).
No `npm`/`PyPI`/`crates` registry check is meaningful here.

## Architecture Patterns

### System Architecture Diagram

```
                    ┌─────────────────────────────────────────────┐
                    │         internal/store (production)          │
                    │                                               │
                    │  NewQdrantClient(host, port, opts...) ─────┐  │
                    │    base: grpc.WithStatsHandler(otelgrpc)   │  │
                    │    + caller opts appended                  │  │
                    │           │                                 │  │
                    │           ▼                                 │  │
                    │      *qdrant.Client ──► store.New() ──► *Store
                    └───────────┬───────────────────────────────┬──┘
                                │                                │
              (in-package,     │                     (cross-package,
               D-07 cycle)      │                      D-06/D-07 no cycle)
                                │                                │
     ┌──────────────────────────▼───┐                ┌──────────▼─────────────┐
     │ package store (_test.go)     │                │ internal/store/storetest │
     │ dialTestClient,               │                │  (non-_test.go support) │
     │ dialFacetInterceptingTestClient,│               │  Dial(t, limit, opts...)│
     │ dialCountSideEffectTestClient,│                │  OversizedFixture(...)  │
     │ dialMidSweepTestClient,       │               │  Main(m) container      │
     │ dialCapturingTestClient,      │               │  lifecycle              │
     │ dialFaultInjectingTestClient  │               └──────────┬──────────────┘
     │  → calls NewQdrantClient(...)  │                          │
     │    bare name, own interceptor  │              ┌───────────┼───────────────┐
     └────────────────────────────────┘              ▼           ▼               ▼
                                          internal/server  internal/e2e   internal/retrievaleval
                                          (package server,  (package e2e,  (package retrievaleval,
                                           TestMain here)    TestMain here + TestMain here,
                                                              binary build)  ENGRAM_RETRIEVAL_EVAL gate)

Enforcement (both AST gates run repo-wide over the tree above):
  TestQdrantClientIsHeldOnlyByStorePackage  → *qdrant.Client TYPE refs, non-test files only
      (existing; storetest joins qdrantClientHolderAllowlist, D-09)
  D-11 new gate                              → qdrant.NewClient(...) CALL sites, test files included
      (new; every call site above must resolve to the one definition in store.go)
```

### Recommended Project Structure

```
internal/store/
├── store.go                          # existing; gains NewQdrantClient (D-01)
├── store_test.go                     # loses TestMain/requireQdrant/dialTestClient/
│                                      #   image-tag consts to storetest; keeps
│                                      #   TestListScopesFullPayloadsOverGRPCLimit
│                                      #   migrated onto storetest's seeder
├── schemaversion_stamp_gate_test.go   # qdrantClientHolderAllowlist gains storetest (D-09)
├── qdrant_client_convergence_test.go  # NEW: D-11 gate (name is Claude's discretion)
├── store_test_external_test.go        # NEW: package store_test — the moved TestMain
│                                       #   (D-07 cycle-breaker; name is discretion)
└── storetest/
    ├── storetest.go                   # Dial, dial-option helpers (non-_test.go, SPDX header required)
    ├── seed.go                        # oversized-fixture seeder, two shapes (D-05)
    └── main.go                        # Main(m) container lifecycle (D-06)

internal/server/tools_test.go          # 2 call sites converge onto storetest.Dial
internal/server/schemaversion_wire_test.go  # 1 call site converges onto storetest.Dial
internal/e2e/spine_review_test.go      # 1 call site converges onto storetest.Dial
internal/e2e/harness_test.go           # TestMain's Qdrant half converges onto storetest;
                                        #   binary-build half stays local (discretion item)
internal/retrievaleval/retrieval_eval_test.go  # 1 call site + TestMain converge (discretion item)
```

### Pattern 1: Shared client constructor with base + caller dial options

**What:** `store.NewQdrantClient` builds the base dial-option slice production already uses
(`grpc.WithStatsHandler(otelgrpc.NewClientHandler())`) and appends caller-supplied
`opts ...grpc.DialOption` — the same "base, then caller" order `qdrant.Config.GrpcOptions` itself uses
internally.

**When to use:** Every site that today calls `qdrant.NewClient(&qdrant.Config{...})` directly.

**Example (production callsite, after convergence):**
```go
// Source: internal/server/tools.go:123-129 (current shape, to be replaced)
qc, err := qdrant.NewClient(&qdrant.Config{
    Host: host,
    Port: port,
    GrpcOptions: []grpc.DialOption{
        grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
    },
})
```
becomes:
```go
qc, err := store.NewQdrantClient(host, port)
```
and a test needing a custom interceptor AND the shared base options (in-package, D-07):
```go
c, err := NewQdrantClient(host, port,
    grpc.WithUnaryInterceptor(recallCaptureInterceptor(t, capture)))
```

**Verified library fact backing this pattern:** qdrant-go-client v1.19.2's own `NewGrpcClient` builds its
internal dial options first, THEN appends `config.GrpcOptions` — the doc comment states this explicitly:

```go
// Source: qdrant-go-client@v1.19.2 qdrant/grpc_client.go:33-53 [VERIFIED: read this session]
// We append config.GrpcOptions in the end
// so that user's explicit options take precedence
...
dialOptions := make([]grpc.DialOption, 0, len(grpcOptions)+len(config.GrpcOptions))
dialOptions = append(dialOptions, grpcOptions...)
dialOptions = append(dialOptions, config.GrpcOptions...)
conn, err := grpc.NewClient(config.getAddr(), dialOptions...)
```
This confirms `Config.GrpcOptions` **appends to, never replaces**, the client's own defaults — directly
answering the additional-context question. grpc-go's `WithDefaultCallOptions` is itself additive across
multiple calls:
```go
// Source: grpc-go@v1.83.2 dialoptions.go:274-278 [VERIFIED: read this session]
func WithDefaultCallOptions(cos ...CallOption) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.callOptions = append(o.callOptions, cos...)
	})
}
```
So a base `grpc.WithStatsHandler(...)` dial option plus a caller's own
`grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(n))` — or even two separate
`WithDefaultCallOptions` calls from different layers — compose correctly; nothing silently overwrites the
other. [VERIFIED: grpc-go source, this session]

### Pattern 2: `net/http/httptest`-shaped non-`_test.go` support package

**What:** `storetest` is a normal Go package (not suffixed `_test.go`), importable from any package,
holding dial/seed/lifecycle helpers — the same shape as this repo's existing `internal/testhttp`.

**Verified precedent, read this session:**
```go
// Source: internal/testhttp/reuse.go:1-19 [VERIFIED: read this session]
// Package testhttp provides connection-reuse test instrumentation shared by
// internal/embed and internal/summarize. It is a normal (non-_test.go) file
// in an internal package rather than a _test.go helper because Go cannot
// share a _test.go across package boundaries, and both provider clients'
// test packages need the same tracker.
//
// It imports no test framework and exposes only counters and accessors, so
// nothing test-only is pulled into a production import graph even though the
// package is importable from non-test code.
package testhttp
```
`storetest` follows this exactly, with one addition: it DOES import `testing` (for `t testing.TB` params
and `t.Fatalf`/`t.Skip`) since every existing dial helper it replaces already does — `internal/testhttp`
happens not to need `testing` at all, but that is incidental to its narrower job, not a constraint
`storetest` must also satisfy.

### Anti-Patterns to Avoid

- **Reusing `Store.Get`/`GetPoints` inside the seeder for anything but the write path itself:** not
  applicable here (D-08 restricts the seeder to `Store.Upsert`/`Store.DeleteAll` only — no raw client
  writes), but worth stating since PITFALLS.md Pitfall 4 flags exactly this shape as a hazard in later
  phases; Phase 1's own seeder must not set the precedent of a raw-client write path.
- **A test dial helper that sets its OWN `MaxCallRecvMsgSize` different from the production value:**
  PITFALLS.md Pitfall 7 names this directly — it is precisely what D-04 (name the 4 MiB limit explicitly,
  every test) and D-11 (the AST convergence gate) exist to prevent structurally rather than by reviewer
  vigilance.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Bounding a test client's receive size | A bespoke `WithMaxRecvBytes`-style wrapper type | `grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(n))` passed through `...grpc.DialOption` | D-02: upstream vocabulary, zero translation layer, matches `qdrant.Config.GrpcOptions`'s own shape |
| Detecting convergence violations | A `grep`-based CI script matching the string `qdrant.NewClient` | A `go/ast`-based gate in the style of `TestQdrantClientIsHeldOnlyByStorePackage` (D-11) | A textual grep cannot tell a legitimate definition site from a violation, and cannot be selectively scoped to include `_test.go` while excluding the one legitimate definition — an AST walk already exists in this exact file for exactly this purpose |
| Container lifecycle bookkeeping across 4 packages | A shell script or Makefile target that boots Qdrant before `go test` | `storetest.Main(m)` (D-06), the `net/http/httptest` idiom | Every one of the 4 existing `TestMain`s already does this correctly in Go; the only defect is that it is duplicated 4 times, not that the approach is wrong |

**Key insight:** every piece of this phase already has a working, correct implementation somewhere in the
tree today. The entire task is extraction and convergence, not invention — which is exactly why the AST
gates (existing and new) are the load-bearing verification, not "does the new code work" (it is a copy of
code that already works).

## Runtime State Inventory

**Trigger check:** this phase renames/moves test-only Go symbols (`TestMain`, `requireQdrant`,
`dialTestClient`, package-level vars, image-tag constants) and moves a package boundary (`package store`
→ `package store_test` for `TestMain`). It does **not** rename any user-facing string, collection name,
schema key, CLI flag, or environment variable name — `ENGRAM_QDRANT_TEST_ADDR`, `ENGRAM_REQUIRE_QDRANT`,
and every `testCollectionPrefix` stay byte-identical (D-10 explicit contract). Applying the 5-category
inventory for completeness:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — no Qdrant collection name, payload key, or record shape changes in this phase | None |
| Live service config | CI's `services.qdrant.image` (`.github/workflows/ci.yaml:57`) references the same literal tag `qdrant/qdrant:v1.19.1` this phase's code also carries; no CI YAML edit is required by the constructor/dial-option changes themselves, only the STALE COMMENT at `ci.yaml:53-56` that says "Keep byte-identical to internal/store/store_test.go's qdrantImageTag" — that pointer becomes wrong once the constant moves into `storetest` (D-10 explicit action item) | Code edit: update the comment's file pointer only, not the pinned tag value |
| OS-registered state | None | None |
| Secrets/env vars | `ENGRAM_QDRANT_TEST_ADDR`, `ENGRAM_REQUIRE_QDRANT`, `ENGRAM_RETRIEVAL_EVAL` — all read by name via `os.Getenv` at call sites that move packages; the STRING names are unchanged (D-10), only which package's code reads them | Code edit only — verify every relocated `os.Getenv` call still fires (see Pitfall: package-boundary variable visibility below) |
| Build artifacts | None — no `go.mod`/`pyproject.toml`/package manager change | None |

## Common Pitfalls

### Pitfall 1: `TestRedEvidencePatchesAreLive` is ALREADY RED on this branch — verified live, not theoretical

**What goes wrong:** The harness fails today because `.planning/phases/01-test-harness-fixture-helper`
already exists while `redEvidenceDirs` is empty.

**Live evidence (this session):**
```
$ go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -v -count=1
--- FAIL: TestRedEvidencePatchesAreLive (0.01s)
    redevidence_harness_test.go:203: redEvidenceDirs is empty while 1 active-milestone
    phase director(ies) exist (01-test-harness-fixture-helper): a phase that shipped
    red-evidence must register it here, or record explicitly that it has none. An empty
    map during an open milestone is a vacuous gate, not a clean run.
FAIL
```
[VERIFIED: live `go test` run, this session, against HEAD]

**Why it happens:** `TestRedEvidencePatchesAreLive`'s own empty-map guard (`redevidence_harness_test.go:201-209`)
treats "phase directory exists, map empty" as a defect by design — this is the harness working exactly as
documented, not a bug.

**How to avoid:** This phase's LAST plan must add a `red-evidence/` directory under
`.planning/phases/01-test-harness-fixture-helper/` containing at least the two patches CONTEXT.md's
discretion section names (a patch reverting `ListScopes`' payload selector so the migrated #583 test goes
RED; a patch adding a bare `qdrant.NewClient` call in a `_test.go` file so the D-11 gate goes RED), and
register both in `redEvidenceDirs` in the same change. Follow the exact unified-diff shape of prior
milestones' patches, e.g.:
```diff
// Source: .planning/milestones/2026-09-13.01-phases/01-executor-correctness-man-pages/red-evidence/01-01-osrun-ctx-err-first.patch [VERIFIED: read this session]
diff --git a/internal/setup/environment.go b/internal/setup/environment.go
index 48834141..5a50c9bb 100644
--- a/internal/setup/environment.go
+++ b/internal/setup/environment.go
@@ -120,8 +120,6 @@ func osRun(ctx context.Context, path string, args []string) (RunResult, error) {
 	switch {
 	case runErr == nil:
 		return result, nil
-	case ctx.Err() != nil:
-		return RunResult{}, ctx.Err()
 	case errors.As(runErr, &exitErr):
```
One behavioral revert per patch, mapped 1:1 to the target test function name in `redEvidenceDirs["<dir>"]`.

**Warning signs:** Any plan in this phase that treats "the code works" as done without touching
`redEvidenceDirs` — `task test` (no `-short`) will stay red for every subsequent phase until this is fixed,
since this harness gates the whole `internal/store` package, not just this phase's own tests.

### Pitfall 2: The CI shared-Qdrant-participation count (3 PASS + 1 SKIP) is pinned to a TEST NAME, not a package

**What goes wrong:** `.github/workflows/ci.yaml:101-115` runs
`go test -run '^TestSharedQdrantAddressHonored$' -v -count=1 ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/...`
and asserts exactly `3 PASS + 1 SKIP` (`internal/retrievaleval` is expected to SKIP since it gates on
`ENGRAM_RETRIEVAL_EVAL`). `TestSharedQdrantAddressHonored` in `internal/store` currently lives in
`package store` (`store_test.go:241`) and reads the package-level vars `testQdrantAddr` /
`testQdrantContainerBooted` that `TestMain` (same package) sets.

**Why it happens:** D-10 moves `internal/store`'s `TestMain` to a new `package store_test` file. A Go test
binary has exactly one `TestMain`, and it may live in either the in-package or external test package for
the same directory — but `TestSharedQdrantAddressHonored`'s assertion depends on state
(`testQdrantContainerBooted`) that only the package holding `TestMain` can set. If `TestMain` moves to
`package store_test` and `TestSharedQdrantAddressHonored` stays in `package store`, the two are now in
different packages and `package store` cannot see `package store_test`'s state without an exported seam —
which does not exist and D-10 does not ask for one (the exported contract is `ENGRAM_QDRANT_TEST_ADDR` env
var only).

**How to avoid:** Move `TestSharedQdrantAddressHonored` to the SAME new `package store_test` file as
`TestMain`, reading storetest's exported booted-state accessor directly, rather than leaving it behind in
`package store`. Because both files live in the same directory (`internal/store/`) and compile into the
same test binary, `go test ./internal/store/...` still reports exactly one PASS-or-SKIP result for this
test name regardless of which of the two packages in that directory defines it — the pinned CI count
(3 PASS + 1 SKIP) is unaffected as long as the test is not accidentally duplicated across both packages.

**Warning signs:** A plan that moves `TestMain` but does not move `TestSharedQdrantAddressHonored`
alongside it (compile error — the test would reference vars nothing sets), or one that defines the test in
BOTH packages (the CI step's `grep -c '^--- PASS'`-style count would then read 4, not 3, tripping
`ci.yaml:112`'s exact-equality check).

### Pitfall 3: A recursive-scan assumption about the AST gates is wrong — verify before relying on it

**What goes wrong:** It is tempting to assume the existing write-boundary/recall-gate AST scans
(`scanPackageDirForCalls`) walk subdirectories, which would mean `internal/store/storetest/`'s Upsert-based
seeder gets scanned by the SAME gate that polices `internal/store`'s own partial-write classification.

**Live verification, this session:**
```go
// Source: internal/store/schemaversion_stamp_gate_test.go:197-222 [VERIFIED: read this session]
func scanPackageDirForCalls(fset *token.FileSet, dir, suffix, excludeSuffix string, methods map[string]bool) (sites []qdrantCallSite, filesScanned int, err error) {
	entries, err := os.ReadDir(dir)
	...
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
```
`os.ReadDir` is non-recursive and explicitly skips subdirectory entries (`e.IsDir()` short-circuits). This
means `scanPackageDirForCalls(fset, ".", ...)` — as called from `internal/store`'s own gate tests — NEVER
sees `internal/store/storetest/*.go`, regardless of what the seeder does internally.

**How to avoid:** Do not add a defensive workaround inside `storetest`'s seeder to "stay off" the
write-boundary/recall-gate scans — it is already structurally invisible to them by directory scope. The
gate that DOES see `storetest` is the module-wide one (`scanRepoForQdrantClientRefs`, used by
`TestQdrantClientIsHeldOnlyByStorePackage`), which is exactly why D-09 (add `storetest` to
`qdrantClientHolderAllowlist`) is the correct and sufficient response — not an extension of the
recall-gate scan.

**Warning signs:** A plan task that proposes modifying `schemaversion_recallgate_test.go`'s scan scope to
"cover storetest too" — unnecessary and outside this phase's stated scope (that gate's job is
`internal/store`'s own six recall entry points, not test infrastructure).

### Pitfall 4: `TestQdrantClientIsHeldOnlyByStorePackage`'s write-check is hardcoded to ONE file, not generalized to every allowlist entry

**What goes wrong:** The existing gate's "composition root must never itself issue a write" check is
literally scoped to `internal/server/tools.go` only:
```go
// Source: internal/store/schemaversion_stamp_gate_test.go:962-966 [VERIFIED: read this session]
toolsPath := filepath.Join(root, "internal", "server", "tools.go")
toolsSrc, err := os.ReadFile(toolsPath)
```
Adding `storetest` to `qdrantClientHolderAllowlist` (D-09) satisfies the set-equality half of the test
(every holder file is allowlisted, every allowlist entry has a matching holder) — but it does NOT
automatically extend the "never issues a write on its own held client" verification to `storetest`, because
that verification block only ever reads `tools.go`.

**Why it happens:** The test was written when there were exactly two allowlist entries and one of them
(`store.go`) IS the write-boundary gate's own subject, so only the OTHER entry (`tools.go`) needed the extra
check. A third entry changes that assumption without the test code knowing it.

**How to avoid:** This is a genuine design decision for the plan, not a mechanical fact: either (a)
generalize the write-check loop to run over every `qdrantClientHolderAllowlist` entry except `store.go`
itself (a real code change to `TestQdrantClientIsHeldOnlyByStorePackage`), or (b) accept that `storetest`'s
"never issues a raw write outside `Store.Upsert`/`Store.DeleteAll`" property (D-08) is enforced by design
review and the red-evidence patch alone, not by this particular AST check, and document that explicitly.
Flagging this now so the plan makes the choice deliberately rather than assuming the existing test already
covers it (it does not, as verified above).

**Warning signs:** A plan or VERIFICATION.md that claims "the client-holder gate proves storetest never
writes directly" without having generalized the check — that claim is false against the code as it stands.

### Pitfall 5: D-11's new gate needs different AST logic than either existing gate, not a copy of one

**What goes wrong:** D-11 asks for a gate "in the style of `TestQdrantClientIsHeldOnlyByStorePackage`" that
scans `_test.go` files too. But the existing `fileRefsQdrantClient`/`scanRepoForQdrantClientRefs` flags
BOTH type references (`*qdrant.Client` appearing anywhere — a field, parameter, return type) AND
`qdrant.NewClient` calls, together, as one boolean per file. D-11 needs something narrower and different:
only the `qdrant.NewClient(...)` CALL EXPRESSION is forbidden outside its one legitimate definition site;
naming the `*qdrant.Client` TYPE is fine everywhere (every dial helper's signature returns `*qdrant.Client`,
and that must stay legal).

**Why it happens:** Reusing `fileRefsQdrantClient` verbatim for D-11 would either (a) also flag every
existing `func dialXxx(t *testing.T) *qdrant.Client` signature (a false positive, since naming the return
type is not the violation) or (b) require excluding type references and keeping only the call-expression
half, which is a smaller, different function than what exists today.

**How to avoid:** Write a new, narrower AST walker for D-11: for every non-conflicting `.go` file in the
repo (INCLUDING `_test.go`, per D-11 — this is the one respect in which it must NOT reuse
`scanRepoForQdrantClientRefs`'s exclusion), find every `CallExpr` matching `qdrant.NewClient(...)`
(selector `X.Sel.Name == "NewClient"`, `X.X.(*ast.Ident).Name == "qdrant"`), and record its enclosing
function name and file path (the existing `qdrantClientLocalNames`/`scanQdrantCalls` machinery already
has the pieces for "find the enclosing function of a call site" — reuse that half). The ONLY allowed
occurrence, repo-wide, is inside the function body that defines `store.NewQdrantClient` itself
(`internal/store/store.go`). Every other occurrence — including the 6 in-package `store` test dial helpers
which call the unqualified `NewQdrantClient(...)` (not `qdrant.NewClient`) once converged — is a violation.

**Warning signs:** A gate implementation that accidentally flags `store.go`'s own `NewQdrantClient`
definition as a violation (an off-by-one in "is this call site INSIDE vs. the definition of the allowed
function" logic) — write the allowed-site check as "this call's enclosing `*ast.FuncDecl` is named
`NewQdrantClient` AND its receiver-less AND it's in `internal/store/store.go`", not merely "this file is
`store.go`" (a second, illegitimate `qdrant.NewClient` call added anywhere else in `store.go` should still
trip the gate).

### Pitfall 6: `internal/retrievaleval`'s `TestMain` has a materially different shape from the other three

**What goes wrong:** Unlike `internal/store`, `internal/server`, and `internal/e2e`, `retrievaleval`'s
`TestMain` gates on `ENGRAM_RETRIEVAL_EVAL != "1"` and returns immediately via `os.Exit(m.Run())` as its
VERY FIRST statement — before even calling `requireQdrant()`-equivalent logic (it has none: no
`ENGRAM_REQUIRE_QDRANT` fail-closed path exists in this package at all).

**Live comparison, this session:**
```go
// Source: internal/retrievaleval/retrieval_eval_test.go:353-356 [VERIFIED: read this session]
func TestMain(m *testing.M) {
	if os.Getenv("ENGRAM_RETRIEVAL_EVAL") != "1" {
		os.Exit(m.Run())
	}
	if addr := os.Getenv("ENGRAM_QDRANT_TEST_ADDR"); addr != "" {
```
versus `internal/store`/`internal/server`/`internal/e2e`, all three of which call `requireQdrant()` FIRST
and support `ENGRAM_REQUIRE_QDRANT` fail-closed behavior.

**How to avoid:** If `storetest.Main` is designed as "one function every package's `TestMain` delegates
its entire body to," it must either (a) take the fail-closed/opt-in-gate behavior as a parameter so
`retrievaleval` can opt OUT of the `ENGRAM_REQUIRE_QDRANT` check while still opting IN to the
`ENGRAM_QDRANT_TEST_ADDR` fast path and container-boot fallback, or (b) `retrievaleval`'s own `TestMain`
keeps its opt-in gate as a thin wrapper and calls a NARROWER `storetest` helper (address resolution +
container boot only) rather than a monolithic `storetest.Main`. This is exactly the discretion item
CONTEXT.md names ("how internal/e2e and internal/retrievaleval harness specifics adapt to
`storetest.Main`") — flagging the concrete divergence here so the plan does not discover it mid-task.
`internal/e2e`'s `TestMain` has an ADDITIONAL divergence: it builds the `engram` binary via
`exec.Command("go", "build", ...)` before ever touching Qdrant — that half is completely orthogonal to
`storetest` and must stay local to `internal/e2e`'s own `TestMain`.

**Warning signs:** A `storetest.Main(m)` signature that takes no parameters and unconditionally applies
`ENGRAM_REQUIRE_QDRANT` fail-closed behavior — this would silently change `retrievaleval`'s behavior (make
a missing Qdrant FATAL when `ENGRAM_RETRIEVAL_EVAL=1` is set but Docker is unavailable, where today it only
skips) or would require `retrievaleval` to route around `storetest.Main` entirely, defeating D-10's "full
consolidation" goal for that one package.

### Pitfall 7: No batch write path exists — the seeder pays one round trip per record, `Wait: true` each time

**What goes wrong:** `Store.Upsert` issues exactly one `qdrant.UpsertPoints` RPC per call, with
`Wait: qdrant.PtrOf(true)` (blocks until Qdrant confirms indexing), and there is no batch-upsert method on
`*Store`:
```go
// Source: internal/store/store.go:808-816 [VERIFIED: read this session]
_, err = s.client.Upsert(ctx, &qdrant.UpsertPoints{
    CollectionName: s.collection, Wait: qdrant.PtrOf(true),
    Points: []*qdrant.PointStruct{{...}}, // exactly one point
})
```
```
$ grep -n 'func (s \*Store) [A-Z]' internal/store/store.go | grep -i 'upsert\|batch'
795: func (s *Store) Upsert(...)   # the only one
```
D-08 requires the seeder write through this exact public path (no raw-client batch writes) — so the
many-small shape (candidate ~1000 records) pays ~1000 sequential blocking round trips, not one batched
call.

**Live measurement, this session (baseline, not a projection):** the EXISTING few-large fixture
(`TestListScopesFullPayloadsOverGRPCLimit`, 40 records × 128 KiB, sequential `Store.Upsert` calls with
`Wait: true`) completes its whole test body — seed + `ListScopes` + assertions — in **0.32s**
(container boot separately took ~1.2s):
```
$ go test ./internal/store/ -run '^TestListScopesFullPayloadsOverGRPCLimit$' -v -count=1
--- PASS: TestListScopesFullPayloadsOverGRPCLimit (0.32s)
```
[VERIFIED: live `go test` run, this session]

**How to avoid:** [ASSUMED — extrapolated, not independently measured] at ~8ms/record observed for
128 KiB payloads, a many-small shape with roughly 1000 records at ~5 KiB each is plausibly in the same
single-digit-seconds range if per-call round-trip latency dominates over payload-serialization time (likely,
given `Wait: true` blocks on server-side indexing, not on wire transfer of a comparatively small payload).
This is a projection from one data point, not a verified number — the plan should budget for the seeder
being noticeably slower than the existing 40-record fixture and should NOT assume it is free just because
the existing fixture is fast. Consider (Claude's discretion, not a decision) running the many-small shape's
Upsert calls from multiple goroutines inside the seeder to shorten wall time — this does not violate D-12
(`t.Parallel` is about SUBTESTS/tests, not goroutines a single test's own helper spawns internally) and does
not violate D-08 (each goroutine still calls the same public `Store.Upsert`).

**Warning signs:** A plan that treats seeding time as negligible when sizing CI budgets, or one that
reaches for a raw-client batch `Upsert` call "just for speed" — that would violate D-08 directly.

## Code Examples

### The 11 call sites, classified by convergence path

Derived by reading every enclosing function this session (`rg` for `qdrant.NewClient`, then `Read` on each
match's containing function):

| # | Site | Package | Convergence path | What it does with the client |
|---|------|---------|-------------------|-------------------------------|
| 1 | `internal/store/store_test.go:190` (`dialTestClient`) | `store` (in-package) | `NewQdrantClient` direct (D-07) | Feeds `newTestStore`→`store.New`; reindex tests also use it raw to drive two collections directly |
| 2 | `internal/store/migrate_status_test.go:202` (`dialFacetInterceptingTestClient`) | `store` (in-package) | `NewQdrantClient` direct + `grpc.WithUnaryInterceptor` | Facet-count fixture construction via interceptor |
| 3 | `internal/store/revert_test.go:557` (`dialCountSideEffectTestClient`) | `store` (in-package) | `NewQdrantClient` direct + interceptor | Counts/fires side effects on outgoing `*qdrant.CountPoints` |
| 4 | `internal/store/migrate_converge_test.go:446` (`dialMidSweepTestClient`) | `store` (in-package) | `NewQdrantClient` direct + interceptor | Hooks `ScrollPoints`/`SetPayloadPoints` mid-sweep for convergence tests |
| 5 | `internal/store/schemaversion_recallgate_test.go:940` (`dialCapturingTestClient`) | `store` (in-package) | `NewQdrantClient` direct + interceptor | Captures `*qdrant.Filter` from `Query`/`Scroll`/`Count` for the recall-gate proof |
| 6 | `internal/store/migrate_faultinject_test.go:249` (`dialFaultInjectingTestClient`) | `store` (in-package) | `NewQdrantClient` direct + interceptor | Injects post-invoke `SetPayload` failures |
| 7 | `internal/server/tools_test.go:377` (`testDepsWithStore`) | `server` | `storetest.Dial` candidate | Feeds `newTestStore`→`store.New`; client not used again afterward |
| 8 | `internal/server/tools_test.go:6070` (`dialWarnPendingMigrationsTestClient`) | `server` | `storetest.Dial` candidate | Plain dial; caller does raw `c.SetPayload(...)` afterward to inject an arbitrary `schema_version` bypassing the codec |
| 9 | `internal/server/schemaversion_wire_test.go:195` (`dialRawQdrantClient`) | `server` | `storetest.Dial` candidate | Plain dial; bypasses `store.Store`'s payload codec entirely for a legacy-shape fixture |
| 10 | `internal/e2e/spine_review_test.go:73` (`spineReviewQdrantClient`) | `e2e` | `storetest.Dial` candidate | Feeds `newSpineReviewStore`→`store.New`; caller also does raw `c.DeleteCollection(...)` setup/teardown |
| 11 | `internal/retrievaleval/retrieval_eval_test.go:336` (`newTestcontainerStore`) | `retrievaleval` | `storetest.Dial` candidate | Feeds `newTestStore`→`store.New`+`EnsureCollection`; client not used again afterward |

None of sites 7–11 need a custom interceptor — a single `storetest.Dial(t, host, port, opts ...grpc.DialOption) *qdrant.Client`
(or however Claude's discretion shapes the signature) covers all five; their POST-dial raw usage (sites 8, 9,
10) is unaffected by convergence — D-08's write-path restriction applies only to the NEW oversized-fixture
seeder, not to every existing raw-client test usage in the suite.

### The 4 `TestMain`s — verified behavioral differences

All four read this session in full.

| Behavior | `internal/store` | `internal/server` | `internal/e2e` | `internal/retrievaleval` |
|----------|-------------------|--------------------|-----------------|---------------------------|
| First statement | `requireQdrant()` | `requireQdrant()` | `requireQdrant()` | `os.Getenv("ENGRAM_RETRIEVAL_EVAL") != "1"` gate, returns BEFORE any Qdrant/requireQdrant logic [VERIFIED: retrieval_eval_test.go:353-356] |
| `ENGRAM_REQUIRE_QDRANT` fail-closed | Yes | Yes | Yes | **No such check exists** [VERIFIED: no `requireQdrant`-equivalent call in this file's `TestMain`] |
| `ENGRAM_QDRANT_TEST_ADDR` fast path | Yes, `os.Exit(m.Run())` immediately | Yes, same | Yes, same, but with an extra `os.RemoveAll(tmp)` for the built binary's temp dir | Yes, same |
| Container boot timeout | `3*time.Minute` | `3*time.Minute` | `3*time.Minute` | `3*time.Minute` (all four identical) |
| Container terminate timeout | `30*time.Second` | `30*time.Second` | `30*time.Second` | `30*time.Second` (all four identical) |
| Extra non-Qdrant setup | None | None | Builds the `engram` binary via `exec.Command("go","build",...)` into a temp dir BEFORE the Qdrant branch runs | None |
| Image tag source | Named const `qdrantImageTag = "qdrant/qdrant:v1.19.1"` [VERIFIED: store_test.go:35] | Inline literal `"qdrant/qdrant:v1.19.1"` [VERIFIED: tools_test.go:224] — no named constant today | Inline literal `"qdrant/qdrant:v1.19.1"` [VERIFIED: harness_test.go:139] — no named constant today | Named const `qdrantImageTag = "qdrant/qdrant:v1.19.1"` [VERIFIED: retrieval_eval_test.go:33] |
| `testQdrantContainerBooted` bookkeeping | Yes | Yes | Yes | Yes (all four set it identically, right after `GRPCEndpoint` resolves) |

**Go-language confirmation for the "TestMain in an external package + in-package `_test.go` files"
question:** Go permits exactly one `TestMain` per test binary, and a directory's test binary is built from
BOTH the in-package (`package store`) and external (`package store_test`) test files together — this is
standard, documented `go test` behavior (a package's `_test.go` files may declare either `package foo` or
`package foo_test`, and both compile into the one test binary for that directory; `TestMain` may be defined
in either). This repo already uses the external-package pattern for `internal/store/spine_forgery_test.go`
(`package store_test`, cited in CONTEXT.md's canonical refs) sitting alongside in-package `store_test.go` in
the same directory — direct existing proof this coexistence already works in this codebase today.
**Gotcha (see Pitfall 2 above):** package-level `var`s declared in the in-package files (`testQdrantAddr`,
`testQdrantContainerBooted`) are NOT visible to the external package's `TestMain`, and vice versa — they are
genuinely different packages sharing only a directory and a compiled test binary, not a shared variable
namespace. The only cross-package channel is the environment variable
`ENGRAM_QDRANT_TEST_ADDR`, which is why D-10 explicitly calls it out as "the existing contract."

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Each of 4 packages boots its own testcontainer | One shared CI `services:` Qdrant container (#498) | 2026-08-22 [CITED: STATE.md deferred-items log] | Already shipped, already stable (60+ CI runs, per SUMMARY.md) — Phase 1 must not regress it, only keep it green as new fixtures land |
| Each of 11 test call sites dials `qdrant.NewClient` independently | Converges on one constructor + one AST gate | This phase | Eliminates PITFALLS.md Pitfall 7's drift risk (test client silently diverging from production's dial options) |

No deprecated/outdated external API usage found — `grpc.WithDefaultCallOptions`/`MaxCallRecvMsgSize` and
`qdrant.Config.GrpcOptions` are both current, non-deprecated APIs in the pinned versions (confirmed by
reading `dialoptions.go`/`rpc_util.go`/`config.go` directly this session — no `// Deprecated:` marker on
either).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Many-small seeding (~1000 records × ~5 KiB) will complete in roughly single-digit seconds, extrapolated from the measured 40×128KiB fixture's 0.32s | Pitfall 7 | If wildly wrong (e.g., 60s+), the phase's own regression test could threaten the CI job's overall wall-clock budget on top of the memory-pressure concern #497 already tracks; mitigated by the existing `testing.Short()` gate regardless |
| A2 | Running the many-small seeder's Upsert calls concurrently (multiple goroutines) inside the seeder does not violate D-12's "no `t.Parallel`" constraint | Pitfall 7 | If the planner or a reviewer reads D-12 as forbidding ANY concurrency (not just `t.Parallel` on tests/subtests), this optimization would need to be dropped in favor of strictly sequential seeding |
| A3 | `storetest.Main`'s eventual signature can be parameterized (or `retrievaleval` can call a narrower helper) to accommodate `retrievaleval`'s missing `ENGRAM_REQUIRE_QDRANT` check without changing that package's observable behavior | Pitfall 6 | If the eventual API cannot cleanly express this, `retrievaleval`'s convergence becomes partial rather than full, which is explicitly named as Claude's discretion in CONTEXT.md and would need discussion at execute-time |

**If this table is empty:** N/A — see rows above. Every other claim in this document is backed by a direct
`Read`/`Bash` verification performed this session (file/line citations throughout), not training-data
recall.

## Open Questions

1. **Should `TestQdrantClientIsHeldOnlyByStorePackage`'s write-check be generalized to every allowlist
   entry, or left scoped to `tools.go` alone with `storetest`'s write-restriction enforced only by
   design/review?**
   - What we know: the check is hardcoded to `internal/server/tools.go` today (verified by reading the
     source); adding `storetest` to the allowlist does not extend that specific check to it.
   - What's unclear: whether generalizing this check is in-scope for Phase 1 (it is a real, if small,
     modification to an existing gate) or should be explicitly deferred/documented as an accepted gap.
   - Recommendation: raise explicitly at plan time (Pitfall 4 above) rather than assume either answer.

2. **Exact `storetest.Main` API shape for `internal/e2e` and `internal/retrievaleval`'s divergent
   `TestMain`s.**
   - What we know: both packages need SOME of `storetest`'s container-lifecycle logic; neither needs to
     delegate its ENTIRE `TestMain` body without modification (e2e's binary build; retrievaleval's opt-in
     gate ordering and missing fail-closed check).
   - What's unclear: whether `storetest` exposes one monolithic `Main(m)` plus escape hatches, or a set of
     smaller composable helpers (resolve-address, boot-or-fast-path, terminate) that each package's own
     `TestMain` calls in its own order.
   - Recommendation: this is explicitly named as Claude's discretion in CONTEXT.md — treat it as a plan-time
     design decision, not something to resolve here.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker | Testcontainers (all 4 `TestMain`s, `storetest.Main`) | ✓ [VERIFIED: `docker info` succeeded this session] | Docker Desktop, Testcontainers Desktop 1.26.0, API 1.56 [VERIFIED: live testcontainers log output] | `ENGRAM_QDRANT_TEST_ADDR` env var (CI's path) |
| `qdrant/qdrant:v1.19.1` image | Every Qdrant-backed test | ✓ [VERIFIED: `docker images` shows it cached locally, 300MB] | v1.19.1 (pinned, matches `qdrantImageTag`/CI's `services.qdrant.image`) | None needed — already cached |
| Go toolchain | Everything | ✓ [VERIFIED: `go test` ran successfully this session] | go 1.26.7 [VERIFIED: go.mod:3] | — |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** None — everything needed for this phase's verification loop is
present in this environment right now.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `testcontainers-go/modules/qdrant` v0.44.0 |
| Config file | None — behavior is coded directly in each package's `TestMain` (moving to `storetest` this phase) |
| Quick run command | `go test -short ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/...` (skips oversized fixtures + the red-evidence harness) |
| Full suite command | `task test` (== `go test ./...` + python hook tests, no `-short`) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|---------------------|--------------|
| REQ-oversized-fixture-helper | Shared seeder produces both shapes, self-asserts logical byte count | integration (real Qdrant) | `go test ./internal/store/... -run 'TestListScopesFullPayloadsOverGRPCLimit\|TestStoretestOversizedFixture' -count=1 -v` (exact new test names are plan-time discretion) | ❌ Wave 0 — new `storetest` seeder + its own regression test |
| REQ-test-client-parity | Every test Qdrant client constructed via `store.NewQdrantClient`; D-11 gate proves convergence | AST/static + integration | `go test ./internal/store/... -run TestQdrantClientConvergence -count=1 -v` (new D-11 gate test name, discretion) | ❌ Wave 0 — new gate |
| (existing gate, must stay green) | `TestQdrantClientIsHeldOnlyByStorePackage` still passes once `storetest` is allowlisted | AST/static | `go test ./internal/store/... -run TestQdrantClientIsHeldOnlyByStorePackage -count=1 -v` | ✅ existing |
| (existing gate, must stay green) | `TestRedEvidencePatchesAreLive` passes once this phase registers its patches | integration (mutates+reverts tree) | `go test ./internal/store/... -run TestRedEvidencePatchesAreLive -count=1 -v` (no `-short`) | ✅ existing, currently RED on this branch (see Pitfall 1) — must turn green by phase close |
| (CI pinned invariant) | Shared-Qdrant participation stays 3 PASS + 1 SKIP after `TestMain`/`TestSharedQdrantAddressHonored` move | integration | `go test -run '^TestSharedQdrantAddressHonored$' -v -count=1 ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/...` | ✅ existing test names; verify count unchanged after the move (Pitfall 2) |

### Sampling Rate

- **Per task commit:** `go test -short ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/...` (fast; skips oversized fixtures and the red-evidence harness by design)
- **Per wave merge:** `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/... -count=1` (fail-closed, exercises the new seeder/gates)
- **Phase gate:** `task test` full suite green before `/gsd-verify-work`, INCLUDING `TestRedEvidencePatchesAreLive` (no `-short`) — this is the test that is currently red and must be the last thing this phase turns green.

### Wave 0 Gaps

- [ ] `internal/store/storetest/storetest.go` (or similarly named) — `Dial` helper, does not exist yet
- [ ] `internal/store/storetest/seed.go` (or similarly named) — oversized-fixture seeder, two shapes, does not exist yet
- [ ] `internal/store/storetest/main.go` (or similarly named) — `Main(m)` container lifecycle, does not exist yet
- [ ] `store.NewQdrantClient` in `internal/store/store.go` — does not exist yet [VERIFIED: `rg` for `func NewQdrantClient` returns no matches this session]
- [ ] D-11's new AST convergence gate — does not exist yet
- [ ] `.planning/phases/01-test-harness-fixture-helper/red-evidence/` directory + `redEvidenceDirs` registration — does not exist yet, and `TestRedEvidencePatchesAreLive` is RED without it right now

## Security Domain

This phase touches no authn/authz, input validation, or cryptography surface — it is test-infrastructure
consolidation with `D-03: No production behavior change in Phase 1` explicit in CONTEXT.md. The one
security-adjacent property in scope is **write-boundary integrity** (does a new test-support package
introduce an unaudited Qdrant write path) — covered above under Don't Hand-Roll and Pitfalls 3/4, not a
traditional ASVS category.

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | No | N/A — no auth surface touched |
| V3 Session Management | No | N/A |
| V4 Access Control | No — but see write-boundary note above | `qdrantClientHolderAllowlist` + (open question) whether its write-check generalizes to `storetest` |
| V5 Input Validation | No | N/A — no new externally-facing input |
| V6 Cryptography | No | N/A |

### Known Threat Patterns for this stack

Not applicable this phase — no new attack surface. The closest analog is the existing AST-gate discipline
this phase extends, which is a supply-chain/code-integrity control (preventing an unaudited Qdrant write
path from silently appearing) rather than a runtime security control.

## Sources

### Primary (HIGH confidence — read directly this session)
- `internal/store/store_test.go` (full `TestMain`/`requireQdrant`/`dialTestClient`/`testStore`/
  `TestListScopesFullPayloadsOverGRPCLimit` region, lines 1–260 and 1740–1860)
- `internal/store/{migrate_status_test.go, revert_test.go, migrate_converge_test.go,
  schemaversion_recallgate_test.go, migrate_faultinject_test.go}` (all 6 dial-helper functions + enclosing
  interceptors)
- `internal/store/schemaversion_stamp_gate_test.go` (lines 700–998: `qdrantClientHolderAllowlist`,
  `fileRefsQdrantClient`, `scanRepoForQdrantClientRefs`, `qdrantClientLocalNames`,
  `TestQdrantClientIsHeldOnlyByStorePackage`, `scanPackageDirForCalls`)
- `internal/store/schemaversion_recallgate_test.go` (lines 1–100: recall-gate scan scope and rationale)
- `internal/store/redevidence_harness_test.go` (full file — harness mechanics and `redEvidenceDirs`)
- `internal/store/store.go` (lines 1–40: imports; 795–817: `Upsert`; 503: `New`; 1458: `maxListLimit`;
  2626+: `DeleteAll` signature; 477–482: `Option` type)
- `internal/server/tools.go` (lines 95–149: `storeFromConfig`, `ensureStoreFromConfig`)
- `internal/server/tools_test.go` (lines 150–390, 6030–6100: `newTestStore`, `requireQdrant`,
  `failOrSkipNoQdrant`, `TestMain`, `testDepsWithStore`, `dialWarnPendingMigrationsTestClient`)
- `internal/server/schemaversion_wire_test.go` (lines 150–230: `dialRawQdrantClient`)
- `internal/e2e/harness_test.go` (full file: `TestMain`, binary build, container lifecycle)
- `internal/e2e/spine_review_test.go` (lines 1–100: `spineReviewQdrantClient`, `newSpineReviewStore`)
- `internal/retrievaleval/retrieval_eval_test.go` (lines 1–60, 300–419: package vars, `newTestcontainerStore`,
  `TestMain`, `TestSharedQdrantAddressHonored`)
- `internal/testhttp/reuse.go` (full file — non-`_test.go` support-package precedent)
- `.licenserc.yaml` (full file — confirms `internal/store/storetest/**` is NOT excluded, so SPDX headers
  are required there)
- `.github/workflows/ci.yaml` (lines 1–160: `services.qdrant`, the two Qdrant-participation assertion steps,
  the `Test` step's env vars)
- `Taskfile.yaml` (lines 1–90: `test`/`test:go`/`test:strict`/`test:short` task definitions)
- `github.com/qdrant/go-client@v1.19.2` module source, read from `$GOMODCACHE`:
  `qdrant/grpc_client.go` (lines 1–70: `NewGrpcClient`, `GrpcOptions` append-order),
  `qdrant/config.go:42` (`GrpcOptions` field)
- `google.golang.org/grpc@v1.83.2` module source, read from `$GOMODCACHE`:
  `dialoptions.go:260–300` (`WithDefaultCallOptions`), `rpc_util.go:390–430` (`MaxCallRecvMsgSize`),
  `rpc_util.go:795–1039` (exact `ResourceExhausted` error-text call sites)
- Live `go test`/`docker` command output, this session:
  `TestListScopesFullPayloadsOverGRPCLimit` timing (0.32s), `TestRedEvidencePatchesAreLive` current failure,
  `docker images`/`docker info` availability
- `go.mod` (version pins: `go 1.26.7`, `github.com/qdrant/go-client v1.19.2`,
  `google.golang.org/grpc v1.83.2`, `go.opentelemetry.io/contrib/.../otelgrpc v0.71.0`)
- `.planning/milestones/2026-09-13.01-phases/01-executor-correctness-man-pages/red-evidence/*.patch`
  (example red-evidence patch shape from a prior shipped phase)

### Secondary (MEDIUM confidence)
- `.planning/research/SUMMARY.md`, `.planning/research/PITFALLS.md` (milestone-level research, already
  read as required reading for this phase; cited inline above where directly relevant)
- `.planning/STATE.md` (project history/decisions, cited for #497/#498 CI-stability timeline)

### Tertiary (LOW confidence)
- None used — every claim in this document traces to a direct `Read`/`Bash` verification this session or
  to the mandatory upstream research/context documents.

## Project Constraints (from CLAUDE.md)

Extracted directives from the repo's `CLAUDE.md` that bind this phase's plans:

- **License headers:** every in-scope Go file carries the Apache-2.0 SPDX header (`task license:check`
  / `task license:add`). Verified this session against `.licenserc.yaml`: `internal/**` is in scope with
  no carve-out for a `storetest` subdirectory, so every new file under `internal/store/storetest/` needs
  the header — this is NOT one of the frontmatter-conflict exclusions (`.planning/**`, `SKILL.md`, etc.).
- **Migrations are schema-version-driven, additive-only, never automatic.** Not directly exercised by this
  phase (Phase 1 makes no production behavior change per D-03), but the seeder's fixtures must not be
  mistaken for a migration step — they are ordinary `Store.Upsert` writes at current schema version.
- **`task` = lint + test; CI mirrors these gates via official actions.** `task test` (`go test ./...`, no
  `-short`) is this phase's own closing gate — and per the live verification in this document, that gate
  is CURRENTLY RED (`TestRedEvidencePatchesAreLive`) until this phase registers its red-evidence.
- **Commits:** Conventional Commits, PR titles CI-validated; `main` protected (branch + PR only).
- **Not used here:** viper, cocogitto — irrelevant to this phase, no risk of accidental reach.
- **Issue tracking:** GitHub Issues is the tracker; durable project memory goes through the `engram` MCP
  store, never a markdown TODO list — not directly invoked by this research task, but binding on any
  follow-up issue this phase's executor might file (e.g., the Open Questions above, if unresolved at
  plan time).
- **User's project rules cited in the phase brief:** rule `m45p2b4bp7` (test our paths against a limit we
  name, never assert grpc-go's own default — directly satisfied by D-04's named 4 MiB constant and the
  existing `TestListScopesFullPayloadsOverGRPCLimit` comment already stating this discipline); rule
  `xvqj44e5mk` (prefer established/idiomatic over hand-rolled — directly satisfied by Pattern 1's "upstream
  vocabulary, zero wrapper" design and the Don't Hand-Roll table above); rule `2rjnv8sc9a` (no SPDX headers
  in `.planning/**` — this RESEARCH.md itself carries none, consistent with that rule).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies; every API cited was read directly from the pinned module
  cache this session
- Architecture: HIGH — every call site, gate, and `TestMain` cited was read directly this session, not
  inferred from CONTEXT.md's summary alone
- Pitfalls: HIGH — Pitfalls 1 and 7 are backed by live command execution this session, not speculation;
  Pitfalls 2–6 are derived from direct source reads of the exact mechanisms they describe

**Research date:** 2026-09-18
**Valid until:** 30 days (stable, internal-only test-infrastructure work; no external API surface that
churns faster than that)
