# Phase 3: Migration Foundation (Registry, Invariants & Sweep) - Research

**Researched:** 2026-08-13
**Domain:** Go type-system-enforced invariants (sealed interfaces) + Qdrant sweep/reconciliation mechanics
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Invariant Enforcement**
- **D-01:** Split enforcement by what each half can actually guarantee. Declaration
  *presence* is compile-time unrepresentable; additive-only *behavior* is proven by
  test. SC2/SC3 say "fails to build **or** fails a test" — take the stronger option
  wherever it exists, rather than applying one mechanism uniformly.
- **D-02:** `Step` is constructed only through `NewStep(from, to Version, addsKeys
  []string, rev Reversibility, apply ApplyFunc)`. No exported struct-literal path —
  positional required parameters mean "nobody thought about reversibility" is not
  representable (SC3), enforced by the compiler.
- **D-03:** `Reversibility` is a sealed interface (`interface{ isReversibility() }`)
  with exactly two constructors: `Reversible(inverse ApplyFunc)` and
  `Irreversible(reason string)`. `Irreversible` **panics on an empty reason**, failing
  at package init — before any test runs.

**Additive-Only Proof**
- **D-04:** Additive-only behavior is proven by a key-set diff over fixture payloads,
  asserted in BOTH directions: (1) `before ⊆ after`; (2) `after − before ==
  declared addsKeys` (set equality, not superset). The second is load-bearing.
- **D-05:** The diff test MUST assert a non-zero fixture count.
- **D-06:** The diff test runs against test-only fixture steps, not the production
  registry (empty this phase). Fixtures cover: conforming additive, irreversible with
  reason, key-removing, and declared-vs-actual-divergent — the last two must FAIL.

**Sweep Mechanics**
- **D-07:** The sweep re-derives its backlog on every pass and loops to zero: scroll
  for records below target, apply a chunk, re-derive, repeat. No persisted cursor.
- **D-08:** Convergence is proven (SC5): because Phase 2's write path stamps current
  version before the sweep runs, records written mid-sweep arrive already-current and
  are never re-processed. This is why no collection lock is needed.
- **D-09:** Partial-failure recovery reconciles by re-derivation (a fresh scroll/
  count), never by trusting the write call's own success/failure signal (SC4).
  `Store.Supersede` already established this pattern (`store.go:129-136`, `:2218-2293`).

**Fault Injection**
- **D-10:** The forced mid-sequence partial `SetPayload` failure (SC4) is injected via
  a gRPC unary interceptor that fails the Nth `SetPayload`, reusing the seam plan
  02-03 built for the recall-gate proof. Rejected: a test-only hook field on `Store`,
  and killing/pausing the container.

**Future Extension**
- **D-11:** The per-version decoder door (SC2) is left open via an optional separate
  interface checked by type assertion — `type Decoder interface { DecodeAt(v Version,
  raw map[string]any) (Record, error) }`, reached by `if d, ok := step.(Decoder); ok`.
  Rejected: threading a nil-able decoder parameter through `NewStep` now.

### Claude's Discretion
- Chunk size for the sweep, and whether it is a constant or derived from Qdrant's
  `PAYLOAD_OP_BATCH_SIZE`.
- The concrete shape of `Validate` (SC1's single invariant over ordering +
  idempotency) and whether ordering is checked by contiguity of `from`/`to` pairs or
  by topological sort.
- Fixture payload construction details, and whether fixtures live in `_test.go` files
  or `testdata/`.
- Error-envelope wording, subject to the repo's `field=<name> hint=<code>` convention.

### Deferred Ideas (OUT OF SCOPE)
- **Restricted-writer enforcement of additive-only** (a `PayloadAdder` with no
  `Delete`/`Set`) — not chosen this phase; worth revisiting if a step is ever authored
  outside this repo.
- **AST scan of step bodies** for `DeletePayload`/`OverwritePayload` calls — rejected
  as a second layer; 02-02 already spent a full review cycle narrowing this class of
  escape hatch.
- **Registering `backfill-short-ids`** — Phase 4's job. Production registry ships
  empty; `migrate.CurrentVersion` stays `0`.
- The `engram migrate` CLI, `registerDestructive`, `--apply`, `--output`, `migrate
  status` histogram — all Phase 4.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-migration-step-registry | Stdlib-only `internal/migrate` leaf package holds the ordered step registry, zero Qdrant/authz dependency, imported by `internal/store`; single `Validate` invariant over ordering + idempotency. | Architecture Patterns § Leaf Package / Registry Shape; Code Examples § `Validate`. |
| REQ-migration-additive-only-gated | A step may only ADD payload keys; enforced by a step-registration invariant that fails build or test, not review; interface shaped for a future per-version decoder. | D-01/D-04/D-06 (locked); Code Examples § Additive-only diff test; Common Pitfalls § Set-equality not superset. |
| REQ-migration-step-reversibility | Every step declares reversibility; reversible supplies inverse, irreversible names why; declaration is mandatory, not representable to skip. | Architecture Patterns § Sealed Interface; Common Pitfalls § nil escape hatch; Code Examples § `Reversibility`. |
| REQ-migrate-partial-failure-resume | Sweep survives Qdrant's batch `SetPayload` non-atomicity (qdrant/qdrant#9371), proven against real pinned Qdrant with forced mid-sequence failure, then converging resume. | Common Pitfalls § Batch non-atomicity; Architecture Patterns § Fault-injection interceptor; Code Examples § Interceptor skeleton. |
| REQ-migrate-converges-without-lock | Sweep needs no collection lock because write path stamps current version before sweep runs — new writes never enter backlog. | Common Pitfalls § Absent-key backlog trap (the filter that makes this true); Architecture Patterns § Sweep loop; Code Examples § Backlog filter, mid-sweep write test. |
</phase_requirements>

## Summary

This phase is almost entirely a **Go type-system exercise** plus **one Qdrant filter-correctness
trap** — there is no new external dependency, no new library to select, and no framework
decision to make. `internal/migrate` stays stdlib-only (confirmed: `migrate.go` currently has
zero imports [VERIFIED: internal/migrate/migrate.go:11]); the only non-stdlib package touched
is `github.com/qdrant/go-client v1.18.3`, already a pinned dependency
[VERIFIED: go.mod:19], used only from `internal/store`, never from `internal/migrate` itself.

The two hard technical questions this research resolves:

1. **Sealed-interface enforcement has a real gap the plan must close explicitly.** A
   nil-marker-method interface (`interface{ isReversibility() }`) blocks a *third-party
   type* from satisfying `Reversibility`, but it does **not** block a caller from
   passing the literal `nil` as the `rev` argument to `NewStep` — `nil` is always
   assignable to an interface-typed parameter in Go, compiler included. `NewStep` MUST
   nil-check `rev` and panic (mirroring `Irreversible`'s own panic-on-empty-reason
   idiom) or the "reversibility declaration is mandatory" claim (SC3) has a silent
   hole. This is confirmed against this repo's own established idiom:
   `TestRemapFromPanicsOnEmptyValue` [VERIFIED: internal/store/store_test.go:2791-2798]
   already tests exactly this shape (`RemapFrom("")` panics at construction) for a
   different construction-time invariant — the same `defer recover()` pattern is the
   one to reuse for both `Irreversible("")` and `NewStep(rev=nil)`.

2. **The backlog-derivation filter is the single highest-risk line in this phase, and
   it is provably wrong if written the "obvious" way.** Confirmed directly against
   Qdrant's own filter-evaluation source (via the Python reference client's
   `local/payload_filters.py`, which mirrors server semantics for exactly this
   purpose): a `range` (or `match`) `FieldCondition` on a key that is **absent** from
   the payload evaluates to `false`, unconditionally — `if condition.range is not
   None: if values is None: return False` [CITED:
   github.com/qdrant/qdrant-client/blob/master/qdrant_client/local/payload_filters.py].
   A naive `schema_version < target` Range filter therefore **silently excludes every
   legacy record with no `schema_version` key at all** — precisely the record class
   this sweep exists to migrate. The correct filter is a `Should` (OR) of the Range
   condition and `NewIsEmpty(schema_version)`, and Qdrant's own filter-combination
   semantics confirm a bare `Should` clause with no `Must` still acts as a hard
   OR-restriction, not a soft hint [CITED:
   github.com/qdrant/qdrant/blob/master/lib/segment/src/payload_storage/query_checker.rs].

**Primary recommendation:** build the sealed-interface registry exactly as D-01–D-03/D-11
specify (no design decision left open there), add an explicit `rev == nil` panic inside
`NewStep`, and derive the sweep's backlog filter as `{Should: [Range(schema_version, {Lt:
target}), IsEmpty(schema_version)]}` — proven by a fixture asserting both a below-target
record AND an absent-key record are included, and a current-version record is excluded.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Step registry, `Validate` invariant, sealed `Reversibility` type | Database / Storage (leaf) | — | `internal/migrate` is a pure, dependency-free leaf package (mirrors `internal/openaiurl`) — no Qdrant, no authz, no I/O of any kind. It is a type/data layer, not a service layer. |
| `Store.Migrate` sweep (scroll, chunk, `SetPayload`, re-derive) | Database / Storage | — | Lives in `internal/store` because it needs the Qdrant client; `internal/migrate` must not import Qdrant (REQ-migration-step-registry). |
| Fault-injection gRPC interceptor (test-only) | Database / Storage (test infra) | — | Wraps the real `*qdrant.Client` dial options; production code path is byte-identical, per D-10. |
| Additive-only / reversibility enforcement | Database / Storage (leaf, compile-time) + Database / Storage (test, runtime) | — | Split per D-01: declaration presence is a Go-compiler-tier guarantee inside `internal/migrate`; additive-only behavior is a test-tier guarantee, also inside `internal/migrate`'s own test file (D-06, fixture steps, not production registry). |
| CLI (`engram migrate`, `registerDestructive`) | API / Backend (operator CLI) | — | Explicitly out of scope — Phase 4. Not researched here. |

## Standard Stack

### Core

No new library is introduced this phase. Everything needed already exists in the module:

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib only | go 1.26.3 [VERIFIED: go.mod:3] | `internal/migrate`'s entire implementation (types, sealed interface, registry slice, `Validate`) | REQ-migration-step-registry mandates zero Qdrant/authz dependency; mirrors the existing `internal/openaiurl` leaf-package precedent [VERIFIED: internal/openaiurl/openaiurl.go:1-12, `import "strings"` is its only import]. |
| `github.com/qdrant/go-client` | v1.18.3 [VERIFIED: go.mod:19] | `Store.Migrate`'s scroll/`SetPayload` calls, backlog filter construction | Already the pinned client for every other `internal/store` sweep method (`PruneExpired`, `BackfillShortIDs`, `RemapOwner`, `Reindex`). No version bump needed — `NewRange`, `NewIsEmpty`, `ScrollAndOffset`, `SetPayload` all already exist in this pinned version [VERIFIED: /Users/sean/go/pkg/mod/github.com/qdrant/go-client@v1.18.3/qdrant/conditions.go:183-268]. |

### Supporting

None. No test framework beyond stdlib `testing` + `google.golang.org/grpc` (already a transitive dependency via `go-client`, already used for the exact same interceptor purpose in `schemaversion_recallgate_test.go`).

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| gRPC unary interceptor fault injection (D-10) | Test-only hook field on `Store` (the existing `s.setPayloadKeys` pattern already used by `Store.Supersede`'s tests — see `TestSupersedeMultiProductionDefaultsDoNotPanic`, `internal/store/store_test.go:4698-4705`) | Explicitly rejected by D-10: adds a seam to production code and partly tests the hook rather than the sweep. Note for the planner: this hook pattern **already exists in this codebase** for `Supersede` — do not accidentally copy it for `Migrate`; the interceptor is the deliberately different choice here. |
| Sealed interface (`interface{ isReversibility() }`) | A `bool` field + separate `InverseFunc ApplyFunc` field on `Step` directly | Rejected implicitly by D-02/D-03: a bool+optional-func pair has a representable-but-wrong state (`Reversible: true, InverseFunc: nil`) that a sealed interface's two constructors cannot produce. |

**Installation:** None — no new packages to install this phase.

**Version verification:** `go.mod` pins `github.com/qdrant/go-client v1.18.3` [VERIFIED: go.mod:19] and the module cache confirms this exact version is fetched (`/Users/sean/go/pkg/mod/github.com/qdrant/go-client@v1.18.3` present alongside `@v1.18.2` and `@v1.14.0`). Server-side, tests pin `qdrant/qdrant:v1.18.2` [VERIFIED: internal/store/store_test.go:35, `const qdrantImageTag = "qdrant/qdrant:v1.18.2"`; identically in internal/retrievaleval/retrieval_eval_test.go:33, internal/e2e/harness_test.go:139, internal/server/tools_test.go:223] — matching the CONTEXT.md-stated pinned server image exactly. (The Helm chart's deployed image is a newer `v1.19.0` [VERIFIED: charts/engram/values.yaml:246] — irrelevant to this phase's test-time proof, which runs against the testcontainer-pinned `v1.18.2`.)

## Package Legitimacy Audit

**Not applicable — this phase introduces zero new external packages.** `internal/migrate`
remains stdlib-only per REQ-migration-step-registry; `Store.Migrate` uses only the
already-pinned, already-vetted `github.com/qdrant/go-client`. No `npm view` / `pip index` /
`cargo search` step is needed, and no package legitimacy verdicts apply.

## Architecture Patterns

### System Architecture Diagram

```
                    ┌─────────────────────────────────────────┐
                    │  internal/migrate  (leaf, stdlib-only)   │
                    │                                          │
                    │  Version (named int)                     │
                    │  CurrentVersion = 0   (Phase 2, unchanged)│
                    │                                          │
                    │  Reversibility (sealed interface)         │
                    │    isReversibility() -- unexported        │
                    │  Reversible(inverse ApplyFunc)  ─┐        │
                    │  Irreversible(reason string)  ───┤ only   │
                    │                                   two     │
                    │                                  ctors    │
                    │                                          │
                    │  ApplyFunc func(payload map[string]any)  │
                    │            (map[string]any, error)        │
                    │                                          │
                    │  Step struct { from,to Version;           │
                    │    addsKeys []string; rev Reversibility;  │
                    │    apply ApplyFunc }                      │
                    │  NewStep(from,to,addsKeys,rev,apply)      │
                    │    -- ONLY exported constructor           │
                    │                                          │
                    │  Registry []Step  (empty this phase)      │
                    │  Validate(Registry) error                 │
                    │    -- ordering + idempotency invariant     │
                    └──────────────────┬───────────────────────┘
                                        │ import (one-way only)
                                        ▼
                    ┌─────────────────────────────────────────┐
                    │  internal/store                          │
                    │                                          │
                    │  Store.Migrate(ctx, target migrate.Version)│
                    │    for { // D-07: no persisted cursor     │
                    │      backlog := scroll(backlogFilter)     │  ◄── derived FRESH
                    │      if len(backlog)==0 { break }         │      every pass
                    │      chunk := backlog[:min(chunkSize,..)] │
                    │      err := client.SetPayload(chunk-ids)  │  ◄── fault-injected
                    │      // no branch on err beyond logging —  │      here in tests
                    │      // next pass re-derives regardless    │      (D-10)
                    │    }                                      │
                    │                                          │
                    │  backlogFilter(target) *qdrant.Filter {   │
                    │    Should: [                              │
                    │      Range(schema_version, {Lt: target}), │  ◄── the ONE line
                    │      IsEmpty(schema_version),             │      that must not
                    │    ]                                      │      regress to
                    │  }                                        │      Range-only
                    └──────────────────┬───────────────────────┘
                                        │ real gRPC (wrapped by
                                        │ test-only fault interceptor)
                                        ▼
                    ┌─────────────────────────────────────────┐
                    │  Qdrant (pinned qdrant/qdrant:v1.18.2)   │
                    │  schema_version: INTEGER PAYLOAD INDEX   │
                    │  (created in Phase 2, ensureIndexes)     │
                    └───────────────────────────────────────────┘
```

A reader tracing "how does a legacy record with no schema_version key get migrated": enters
via `Store.Migrate`'s loop → `backlogFilter` matches it via the `IsEmpty` arm (NOT the `Range`
arm, which cannot see it) → included in the scrolled chunk → `SetPayload` writes the new
key(s) → next pass's fresh scroll no longer matches it (now has a `schema_version` value ≥
target, satisfying neither `Range(Lt: target)` nor `IsEmpty`).

### Recommended Project Structure

```
internal/migrate/
├── migrate.go          # EXISTS (Phase 2): Version, CurrentVersion — this phase EXTENDS it
├── migrate_test.go      # EXISTS (Phase 2): TestCurrentVersionValue — extend, don't replace
├── step.go              # NEW: Step, NewStep, ApplyFunc, Reversibility + its two ctors,
│                         #      the unexported isReversibility() marker + two unexported
│                         #      implementing structs
├── step_test.go          # NEW: NewStep(rev=nil) panics; Irreversible("") panics; construction
│                         #      round-trip; the two escape-hatch checks (see Pitfalls)
├── registry.go           # NEW: Registry []Step (empty var, per D-not-yet-having-steps),
│                         #      Validate(Registry) error
├── registry_test.go       # NEW: Validate against ordering/idempotency fixtures
└── additive_test.go       # NEW (D-06): fixture-only additive-only key-set diff test —
                          #      test-only Steps constructed here, never touching Registry

internal/store/
├── migrate.go            # NEW: Store.Migrate, backlogFilter, chunking
└── migrate_test.go        # NEW: partial-failure-resume (D-09/D-10), lock-free convergence
                          #      (D-08/SC5) against real pinned Qdrant
```

### Pattern 1: Sealed Interface with Exactly Two Constructors

**What:** An interface with a single unexported marker method can only be satisfied by a
type defined in the same package (Go's own visibility rule for interface methods) — *unless*
the package also exports a struct embedding the marker method, which re-opens the door via
embedding. Do not export any base struct that carries `isReversibility()`.

**When to use:** Exactly D-03's case — a small, closed set of legal values that must be
impossible to construct incorrectly from outside the package.

**Example:**
```go
// Source: pattern confirmed against https://rodusek.com/posts/2024/12/14/go-tip-creating-closed-interfaces/
// (unexported-method visibility rule) — applied to this phase's exact shape.
package migrate

// Reversibility is sealed: only Reversible and Irreversible construct it.
// No exported type in this package embeds the marker method, so external
// packages cannot re-open the seal via embedding (the one documented escape
// hatch for this pattern).
type Reversibility interface {
	isReversibility()
}

type reversibleStep struct{ inverse ApplyFunc }

func (reversibleStep) isReversibility() {}

// Reversible declares a step whose inverse is applyInverse.
func Reversible(inverse ApplyFunc) Reversibility {
	if inverse == nil {
		panic("migrate: Reversible requires a non-nil inverse ApplyFunc")
	}
	return reversibleStep{inverse: inverse}
}

type irreversibleStep struct{ reason string }

func (irreversibleStep) isReversibility() {}

// Irreversible declares a step with no inverse, and PANICS if reason is
// empty — an irreversible step that does not name why is a programming
// error, not a runtime condition, and must fail at package init (D-03).
func Irreversible(reason string) Reversibility {
	if reason == "" {
		panic("migrate: Irreversible requires a non-empty reason")
	}
	return irreversibleStep{reason: reason}
}
```

**Escape hatches, addressed explicitly (per research priority 1):**
1. **Embedding.** Only matters if this package exports a struct carrying the marker
   method. Neither `reversibleStep` nor `irreversibleStep` above is exported — closes
   this hatch completely.
2. **Nil interface value.** NOT closed by the interface being sealed — `nil` is always
   assignable to any interface-typed parameter regardless of sealing. `NewStep` MUST
   nil-check `rev` itself (see Pitfall 1 below). This is the one place D-02's "positional
   required parameters" claim needs a runtime backstop, since "positional" only prevents
   *omission*, not `nil`.
3. **Reflection.** `reflect.New` cannot construct `reversibleStep`/`irreversibleStep`
   with a *working* `isReversibility()` call from outside `internal/migrate`, because
   calling an unexported method via reflection from another package still fails at
   compile time for any statically-typed call, and `reflect.Value.Method` on an
   unexported method returns a zero Value that panics on `.Call()`. Not a practical
   escape hatch here.
4. **Same-package construction.** Since the registry and the constructors live in the
   same package (`internal/migrate`), this "escape hatch" is really just "the trusted
   package can always construct its own sealed types correctly, by definition" — not a
   bypass, since the registry lives here anyway. Nothing to close.

### Pattern 2: `NewStep` — Positional-Required Constructor, No Struct-Literal Path

**What:** `Step`'s fields are all unexported; `NewStep` is the only exported constructor.

**Example:**
```go
package migrate

// ApplyFunc mutates a record's raw payload map in place (or returns a new
// one) — the additive-only diff test asserts key-set behavior against
// exactly this signature (D-04).
type ApplyFunc func(payload map[string]any) (map[string]any, error)

type Step struct {
	from, to Version
	addsKeys []string
	rev      Reversibility
	apply    ApplyFunc
}

// NewStep is the ONLY way to construct a Step — no exported struct-literal
// path exists, so a caller cannot omit addsKeys or rev (D-02). rev is
// explicitly nil-checked here: Go's assignability rules let nil satisfy any
// interface-typed parameter regardless of sealing (see Pattern 1's escape
// hatch #2), so the sealed interface alone does NOT make "reversibility
// unspecified" unrepresentable — this check does.
func NewStep(from, to Version, addsKeys []string, rev Reversibility, apply ApplyFunc) Step {
	if rev == nil {
		panic("migrate: NewStep requires a non-nil Reversibility (use Reversible(...) or Irreversible(...))")
	}
	if apply == nil {
		panic("migrate: NewStep requires a non-nil ApplyFunc")
	}
	return Step{from: from, to: to, addsKeys: addsKeys, rev: rev, apply: apply}
}
```

### Pattern 3: Optional Decoder Extension (D-11), Type-Asserted at Point of Use

**What:** Future per-version decoding attaches without touching `Step`/`NewStep`.

**Example:**
```go
// Source: standard Go optional-interface idiom (same shape as io.ReaderFrom,
// http.Flusher, etc.) — applied per D-11's explicit rejection of a nil-able
// parameter threaded through NewStep now.
type Decoder interface {
	DecodeAt(v Version, raw map[string]any) (Record, error)
}

// At the point of use, a future caller checks:
//   if d, ok := someValue.(Decoder); ok { ... }
// Step is NOT required to implement Decoder — this only matters once a
// concrete need exists (Phase 4+), and no existing Step's signature changes
// when it is added.
```

### Pattern 4: Sweep Loop — Fresh Re-Derivation, No Persisted Cursor

**What:** D-07's shape, matching the four existing `internal/store` sweep methods'
scroll-and-page idiom, but WITHOUT their persisted-offset continuation — this sweep
restarts its scroll from the beginning of the backlog filter on every pass, specifically
BECAUSE convergence is proven by "the backlog shrinks to zero across passes," not by
"we finished one exhaustive scroll."

**When to use:** Exactly this sweep. Contrast with `BackfillShortIDs`
[VERIFIED: internal/store/store.go:2741-2797] and `Reindex`
[VERIFIED: internal/store/store.go:3133-3209ish], both of which use a SINGLE
`ScrollAndOffset` pass with an `offset` cursor threaded through (no outer re-derivation
loop) — those are appropriate there because their filtered set only ever shrinks within
one pass (already-visited points drop out of `missingShortIDFilter`) and they do not need
to survive a partial-failure-then-resume story at the SetPayload layer the way this sweep
does. This sweep's outer `for { backlog := scroll(...); if empty break }` loop is the
one genuinely new shape in this codebase's sweep-method family.

**Example:**
```go
// Source: pattern derived from D-07/D-08/D-09 (CONTEXT.md, locked) plus this
// codebase's existing BackfillShortIDs scroll-and-page idiom
// (internal/store/store.go:2764-2796) for the inner-loop chunking mechanics.
func (s *Store) Migrate(ctx context.Context, target migrate.Version, chunkSize int) (migrated uint64, err error) {
	for {
		var offset *qdrant.PointId
		var chunk []*qdrant.RetrievedPoint
		// One bounded scroll page per pass is enough IF chunkSize >= page size;
		// for a large backlog, page internally here exactly like BackfillShortIDs
		// does, but do NOT persist offset ACROSS passes (D-07) — only within one
		// pass's own paging, if the backlog exceeds one page.
		pts, _, serr := s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
			CollectionName: s.collection,
			Filter:         backlogFilter(target),
			Limit:          qdrant.PtrOf(uint32(chunkSize)),
			Offset:         offset,
			WithPayload:    qdrant.NewWithPayload(true), // steps need the payload to apply()
		})
		if serr != nil {
			return migrated, serr
		}
		if len(pts) == 0 {
			return migrated, nil // backlog is empty: converged
		}
		// Apply registered steps per point (elided: the actual step-application
		// loop over migrate.Registry, from->to chaining), then SetPayload.
		// A SetPayload error here is deliberately NOT branched on beyond
		// recording/logging (D-09): the NEXT pass's fresh scroll re-derives
		// whatever didn't land, because backlogFilter matches it again.
		if _, perr := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
			CollectionName: s.collection, Wait: qdrant.PtrOf(true),
			Payload:        qdrant.NewValueMap(map[string]any{ /* additive keys */ }),
			PointsSelector: qdrant.NewPointsSelectorIDs(idsOf(pts)),
		}); perr != nil {
			// intentionally not returned — see D-09 above
			_ = perr
		} else {
			migrated += uint64(len(pts))
		}
	}
}
```

### Anti-Patterns to Avoid

- **`Range`-only backlog filter:** `&qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewRange(schemaVersionKey, &qdrant.Range{Lt: qdrant.PtrOf(float64(target))})}}` — silently excludes every absent-key legacy record (see Pitfall 3). This is the single most important anti-pattern in this phase.
- **Trusting `SetPayload`'s error to describe partial state:** branching sweep logic on "did this specific chunk fully land" based on the returned `error` value contradicts D-09 and the confirmed upstream behavior (qdrant/qdrant#9371) — always let re-derivation be the source of truth.
- **A test-only hook field on `Store` for fault injection** (the pattern `Store.Supersede`'s tests already use via `s.setPayloadKeys`) — explicitly rejected for `Migrate` by D-10. Reuse the gRPC interceptor seam instead.
- **Threading a nil-able `Decoder` parameter through `NewStep` now** — explicitly rejected by D-11; every existing call site would carry a meaningless nil before any customer needs it.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| "Does this record need migrating" | A custom in-process cache of "already-migrated" ids | The Qdrant `schema_version` integer payload index + `backlogFilter`'s live scroll | The index already exists [VERIFIED: 02-01-SUMMARY.md line "schema_version Qdrant integer payload index added to ensureIndexes"] and re-deriving from it is what makes lock-free convergence provable (D-08) — a cache reintroduces exactly the staleness problem the sweep design exists to avoid. |
| "Simulate a partial write failure" | Killing/pausing the Qdrant testcontainer mid-write | The gRPC unary interceptor (D-10) | Container kill/pause is explicitly rejected: non-deterministic, cannot target a specific mid-sequence point. |
| "Detect an additive-only violation" | An AST scanner of step bodies for `DeletePayload`/`OverwritePayload` calls | The key-set diff test (D-04) | Explicitly rejected as a second layer for this phase (Deferred Ideas) — 02-02 already spent a full review cycle narrowing the escape hatches (indirection, helpers, method values) that make an AST scan partial and false-confident. |
| "Enforce reversibility declaration" | A linter rule or code-review checklist item | The sealed-interface + positional-constructor combo (D-01/D-02/D-03) | Review-catch is exactly the failure mode SC3 exists to eliminate — "nobody thought about reversibility" must be a compile error, not a missed review comment. |

**Key insight:** every "don't hand-roll" item in this phase resolves to "the locked decisions
already picked the non-hand-rolled option" — there is no live design choice left where a
custom mechanism is even tempting, except the backlog filter shape, which risks a *subtly
wrong* hand-rolled filter (Range-only) rather than a hand-rolled mechanism outright.

## Common Pitfalls

### Pitfall 1: `NewStep(rev=nil)` compiles and silently produces an "unrepresentable" step
**What goes wrong:** A caller writes `NewStep(0, 1, []string{"k"}, nil, applyFn)`. Go's
assignability rules let `nil` satisfy any interface parameter — the sealed interface does
NOT block this, only third-party *types* implementing it.
**Why it happens:** "Sealed interface" and "non-nil interface value" are two different
guarantees; only the first is what sealing buys you.
**How to avoid:** `NewStep` must explicitly `if rev == nil { panic(...) }`. This is not
mentioned in D-02/D-03's text and is a genuine gap the planner must close — flag it as a
required line, not an optional defensive check.
**Warning signs:** A test asserting "you can't build a Step without declaring
reversibility" that only exercises the *positional-argument-omission* case (which the Go
compiler already rejects trivially) rather than the *explicit-nil* case (which it does
not) is not actually proving SC3.

### Pitfall 2: `Irreversible("")` panicking "at init" only holds if it's actually called at init
**What goes wrong:** D-03 says `Irreversible` "panics on an empty reason, so an
irreversible step that does not name why fails at package init — before any test runs."
This is only true if the call site is a package-level `var` declaration or an `init()`
function. If a badly-declared step is instead constructed inside a function body called
later (e.g., inside a `Register()` call invoked from `main()` or a test), the panic fires
at THAT call time, not at package load — a materially weaker guarantee.
**Why it happens:** Go's init-time-panic behavior is a property of *where* the panicking
call appears in the source, not of the function itself.
**How to avoid:** If the production registry (Phase 4+) is built via `var Registry =
[]Step{...}` at package scope, a bad `Irreversible("")` call anywhere in that literal
panics before ANY test in ANY package that transitively imports `internal/migrate` can
run — this is the strongest version of the guarantee and is almost certainly what D-03
intends. Document this explicitly in the plan so Phase 4 doesn't accidentally weaken it
by registering steps inside a function.
**Warning signs:** A test that calls `Irreversible("")` directly inside a `TestXxx` body
and asserts panic via `recover()` proves the function panics on bad input — it does NOT
prove the "before any test runs" / package-init claim. This repo already has the
right idiom for the *panics-on-bad-input* half: `TestRemapFromPanicsOnEmptyValue`
[VERIFIED: internal/store/store_test.go:2791-2798]. Proving the *init-order* half (if
ever needed) requires a subprocess test (`os/exec` re-invoking the test binary or a
small `package main`) — no such precedent exists yet in this repo; note it as a discretion
item if the plan decides the init-order claim itself needs a dedicated proof rather than
relying on Go's documented init-order semantics.

### Pitfall 3: The backlog filter silently excludes absent-key records (THE highest-risk pitfall)
**What goes wrong:** `{Must: [Range(schema_version, {Lt: target})]}` never matches a
record with no `schema_version` key at all — Qdrant's own filter-evaluation logic returns
`false` for a range/match condition whenever the field's value set is empty (key absent),
rather than treating absence as "less than everything." A legacy pre-Phase-2 record (the
overwhelming majority of records at adoption, per STATE.md's isolation-cardinality note)
is therefore invisible to the sweep, and `Store.Migrate` converges to zero on its very
first pass while having migrated nothing.
**Why it happens:** Confirmed directly against Qdrant's filter-check logic: `values =
value_by_key(payload, condition.key); if condition.range is not None: if values is
None: return False` [CITED:
github.com/qdrant/qdrant-client/blob/master/qdrant_client/local/payload_filters.py] — the
same pattern applies server-side (`check_field_condition` returns `field_condition
.check_empty()` when `field_values.is_empty()`, and a bare `range`/`match` condition's
`check_empty()` is `false` unless `is_empty`/`is_null` is explicitly what was asked for)
[CITED: github.com/qdrant/qdrant/blob/master/lib/segment/src/payload_storage/query_checker.rs].
**How to avoid:** `backlogFilter(target) = &qdrant.Filter{Should: []*qdrant.Condition{
qdrant.NewRange(schemaVersionKey, &qdrant.Range{Lt: qdrant.PtrOf(float64(target))}),
qdrant.NewIsEmpty(schemaVersionKey), }}`. A bare `Should` with no `Must` at the top level
of a `Filter` is a hard OR-restriction, not a soft ranking hint — confirmed:
`should` requires `conditions.iter().any(check)` and is ANDed with the (here, absent →
vacuously-true) `must`/`must_not` clauses [CITED:
github.com/qdrant/qdrant/blob/master/lib/segment/src/payload_storage/query_checker.rs].
**Warning signs:** A test fixture set that only exercises "record with schema_version=0,
target=1" (below-target) and never constructs a record with the key omitted entirely
would pass a Range-only filter and hide this defect completely — the fixture MUST include
a record written with no `schema_version` key at all (i.e., bypass `payload()`'s stamp,
the way pre-Phase-2 legacy data would look) as a positive-match case.

### Pitfall 4: `qdrant.NewValueMap` panics on the `migrate.Version` named type
**What goes wrong:** Writing `p[schemaVersionKey] = someVersion` (where `someVersion` is
`migrate.Version`, not `int`) into a map passed to `qdrant.NewValueMap` panics with
`"invalid type: migrate.Version"` — `NewValue`'s type switch matches only exact concrete
types (`int`, `int32`, `int64`, ...), and a named type over `int` falls to `default`.
**Why it happens:** Already hit and fixed once in Phase 2 [VERIFIED:
.planning/phases/02-record-schema-versioning-foundation/02-01-SUMMARY.md, Deviation #1].
Any step's `ApplyFunc` or the sweep's own `SetPayload` call that writes a `Version`-typed
value must cast: `int(v)`.
**How to avoid:** Cast explicitly at every Qdrant boundary, exactly as `payload()` already
does: `p[schemaVersionKey] = int(max(migrate.CurrentVersion, m.SchemaVersion))`
[VERIFIED: 02-01-SUMMARY.md key-decisions, "int() cast required before writing
migrate.Version into the payload map"].
**Warning signs:** A panic message containing `"invalid type: migrate.Version"` at the
first `SetPayload` call carrying a raised version — this is the exact signature Phase 2
already saw.

### Pitfall 5: Qdrant's batch `SetPayload` chunking makes "the call errored" ambiguous
**What goes wrong:** The pinned server (`qdrant/qdrant:v1.18.2`) chunks a multi-ID
`SetPayload` internally by `PAYLOAD_OP_BATCH_SIZE` (this repo already documents the value
as 32 for its own purposes: `qdrantPayloadOpBatchSize = 32` [VERIFIED:
internal/store/store.go:127-140]) and mutates every point it finds BEFORE raising an error
for one it doesn't — so an error return means "possibly partial," never reliably "nothing
landed" or "everything landed."
**Why it happens:** Confirmed upstream (qdrant/qdrant#9371, cited directly in this repo's
own `store.go:2218-2223` doc comment describing the identical hazard for `Supersede`).
**How to avoid:** Never branch sweep control flow on the `SetPayload` error's presence
beyond logging/telemetry — rely entirely on the next pass's fresh scroll (D-07/D-09).
This is a repeat of `Store.Supersede`'s already-solved problem, not a new one — but the
new sweep does NOT need `Supersede`'s `reconcileSupersedeFailure`-style explicit re-read
step, because the sweep's OUTER LOOP already re-derives unconditionally every pass; a
dedicated reconciliation function would be redundant machinery here.
**Warning signs:** A test asserting "on partial failure, N records were fixed up by an
explicit reconciliation call" when what's actually needed is "on partial failure, the NEXT
pass converges" — conflating `Supersede`'s per-call reconciliation pattern with this
sweep's per-pass reconciliation pattern would add unneeded complexity.

### Pitfall 6: `go test -run X` matching nothing is a false green
**What goes wrong:** `go test -run TestSomethingMisspelled ./internal/migrate/...` exits 0
with `ok ... [no tests to run]` if the name doesn't match anything.
**Why it happens:** Documented, recurring project trap [VERIFIED:
.planning/STATE.md, "Validation commands can false-green" blocker entry, durable record
`bsbsvn4hbc`].
**How to avoid:** Always run with `-v` and grep for actual `RUN`/`PASS` pairs naming the
exact test, or cross-check against `go test -list '.*' ./internal/migrate/...` before
trusting a `-run` command in VALIDATION.md.
**Warning signs:** A VALIDATION.md row whose command has never been re-run against
`go test -list` since the plan was written.

## Code Examples

### `Validate` — ordering + idempotency invariant (SC1)
```go
// Source: derived from REQ-migration-step-registry's "single Validate
// invariant over ordering + idempotency" and CONTEXT.md's Claude's-Discretion
// note leaving the concrete check-shape open. Contiguity-of-from/to is the
// simpler of the two options CONTEXT.md flags (contiguity vs. topological
// sort) and is sufficient for a REGISTRY THAT IS EXPECTED TO BE A LINEAR
// CHAIN (v0->v1->v2->...), which every other locked decision in this phase
// assumes (D-08's convergence argument, the "current version" framing).
func Validate(steps []Step) error {
	if len(steps) == 0 {
		return nil // empty registry (this phase) is valid by construction
	}
	seen := map[Version]bool{}
	for i, s := range steps {
		if i > 0 && s.from != steps[i-1].to {
			return fmt.Errorf("migrate: step %d: from=%d does not chain from previous step's to=%d", i, s.from, steps[i-1].to)
		}
		if s.from == s.to {
			return fmt.Errorf("migrate: step %d: from==to==%d (a no-op step is not a version transition)", i, s.from)
		}
		if seen[s.from] {
			return fmt.Errorf("migrate: step %d: from=%d already registered by an earlier step (idempotency: each version transitions exactly once)", i, s.from)
		}
		seen[s.from] = true
	}
	return nil
}
```

### Additive-only key-set diff test (D-04/D-05/D-06)
```go
// Source: D-04's exact two-direction assertion, applied against test-only
// fixture Steps (D-06) — never the production Registry, which is empty this
// phase and would make the scan vacuous.
func TestAdditiveOnlyKeySetDiff(t *testing.T) {
	fixtures := []struct {
		name    string
		before  map[string]any
		declared []string
		apply   ApplyFunc
		wantOK  bool
	}{
		{
			name: "conforming additive step",
			before: map[string]any{"content": "x"},
			declared: []string{"schema_version"},
			apply: func(p map[string]any) (map[string]any, error) {
				p["schema_version"] = 1
				return p, nil
			},
			wantOK: true,
		},
		{
			name: "removes a key",
			before: map[string]any{"content": "x", "legacy_field": "y"},
			declared: []string{},
			apply: func(p map[string]any) (map[string]any, error) {
				delete(p, "legacy_field")
				return p, nil
			},
			wantOK: false, // before ⊄ after — must FAIL
		},
		{
			name: "actual adds diverge from declared",
			before: map[string]any{"content": "x"},
			declared: []string{"schema_version"}, // declares ONE key
			apply: func(p map[string]any) (map[string]any, error) {
				p["schema_version"] = 1
				p["undeclared_extra"] = true // adds TWO
				return p, nil
			},
			wantOK: false, // after-before != declared — must FAIL (set equality, not superset)
		},
	}
	if len(fixtures) == 0 {
		t.Fatal("zero fixtures — D-05 requires a non-zero fixture count assertion")
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			before := keysOf(f.before)
			after, err := f.apply(cloneMap(f.before))
			if err != nil {
				t.Fatalf("apply: %v", err)
			}
			afterKeys := keysOf(after)
			removed := setDiff(before, afterKeys)
			added := setDiff(afterKeys, before)
			ok := len(removed) == 0 && setEqual(added, f.declared)
			if ok != f.wantOK {
				t.Errorf("%s: additive-only check = %v, want %v (removed=%v added=%v declared=%v)", f.name, ok, f.wantOK, removed, added, f.declared)
			}
		})
	}
}
```

### Fault-injection interceptor — failing the Nth `SetPayload` (D-10, SC4)
```go
// Source: pattern extended from the existing capturing interceptor in
// internal/store/schemaversion_recallgate_test.go (dialCapturingTestClient /
// recallCaptureInterceptor, lines 878-936) — same grpc.WithUnaryInterceptor
// seam, repurposed from "capture and let through" to "let N-1 through, fail
// the Nth, let the rest through unmodified."
func dialFaultInjectingTestClient(t *testing.T, failNthSetPayload int) *qdrant.Client {
	t.Helper()
	var mu sync.Mutex
	count := 0
	interceptor := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if _, ok := req.(*qdrant.SetPayloadPoints); ok {
			mu.Lock()
			count++
			n := count
			mu.Unlock()
			if n == failNthSetPayload {
				// Never reaches the real server: this chunk's ids stay
				// unmigrated, indistinguishable at the wire from a real
				// mid-sequence failure (D-10's stated property).
				return status.Error(codes.Unavailable, "injected: forced Nth SetPayload failure")
			}
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
	// ... dial exactly like dialCapturingTestClient, with this interceptor
	// instead of recallCaptureInterceptor.
	return dialWithInterceptor(t, interceptor)
}
```

### Backlog filter — absent-key AND below-target, proven (Pitfall 3)
```go
// Source: this file's own Pitfall 3 finding, cited against Qdrant's
// filter-evaluation semantics.
func backlogFilter(target migrate.Version) *qdrant.Filter {
	return &qdrant.Filter{
		Should: []*qdrant.Condition{
			qdrant.NewRange(schemaVersionKey, &qdrant.Range{Lt: qdrant.PtrOf(float64(target))}),
			qdrant.NewIsEmpty(schemaVersionKey),
		},
	}
}

// TestBacklogFilterMatchesAbsentAndBelowTarget — the proof this exists to
// carry. Three records: (1) no schema_version key at all (bypasses
// payload()'s stamp entirely — the true legacy shape), (2)
// schema_version=0 with target=1, (3) schema_version=1 with target=1. Only
// (1) and (2) must appear in the scroll result.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Each schema evolution ships as its own one-shot operator command (the pre-Phase-2 state, explicitly named in the folded todo) | A registered, ordered migration-step mechanism with structural invariants | This milestone (Phases 2-4) | This phase (3) is the mechanism's foundation; the CLAUDE.md "Not used here: database migrations" line is stale and tracked for correction under REQ-claude-md-migrations-convention (Phase 8) — do not treat that line as current truth while planning this phase. |

**Deprecated/outdated:** None specific to this phase's tooling — no library version bump, no
API deprecation encountered.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The production `Registry` (Phase 4+) will be built as a package-level `var` declaration (not inside a function), which is what makes D-03's "panics at package init, before any test runs" claim true in its strongest form. | Pitfall 2 | If Phase 4 instead registers steps via a function call, a bad `Irreversible("")` only panics when that function runs, not universally at import time — weaker than D-03's stated guarantee. Low risk to THIS phase (registry is empty), but worth flagging forward to Phase 4's plan. |
| A2 | `Validate`'s ordering check is implemented via from/to contiguity (a linear chain), not topological sort — chosen because every other locked decision in this phase (D-08's convergence argument, "current version" framing, `migrate.CurrentVersion` as a single scalar) assumes a linear v0→v1→v2→... chain, not a DAG. | Code Examples § `Validate` | If a future phase actually needs branching/merging version graphs, this `Validate` shape would need to change — CONTEXT.md explicitly leaves this as Claude's discretion, so it is not a locked decision being second-guessed, just a reasoned default. |
| A3 | The sweep's inner-chunk `SetPayload` targets a batch of point IDs directly (`PointsSelectorIDs`), not a filter-selector batch — mirroring `BackfillShortIDs`'s per-point and `RemapOwner`'s filter-selector patterns respectively; the exact choice affects how "the Nth SetPayload" in D-10's fault injection lines up with "one chunk." | Architecture Patterns § Pattern 4, Code Examples § fault injection | If the plan instead does one `SetPayload` per POINT (like `BackfillShortIDs`) rather than one per CHUNK, "the Nth SetPayload" in the interceptor targets a single point's write, not a whole chunk — still valid for D-10's purpose, but changes what "partial" means in the test's assertions (partial-within-chunk vs. one-point-of-many-in-a-pass). This is a chunk-size/batching decision CONTEXT.md leaves to Claude's discretion — not wrong, but the plan must state which shape it picked and keep the fault-injection test's expectations consistent with it. |

## Open Questions

1. **Does `Store.Migrate` need its own `chunkSize` parameter, or a package constant like `reindexBatch`?**
   - What we know: CONTEXT.md leaves chunk size to Claude's discretion, and explicitly asks
     "whether it is a constant or derived from Qdrant's `PAYLOAD_OP_BATCH_SIZE`."
   - What's unclear: Whether tying chunk size to `qdrantPayloadOpBatchSize` (32) makes the
     partial-failure test easier to reason about (a chunk that IS exactly one server-side
     batch) or whether a smaller test-only chunk size (injected via the discretionary
     parameter) makes SC4's fault injection cleaner to assert against.
   - Recommendation: Make chunk size a parameter (not a hardcoded constant) so the
     partial-failure test can use a small chunk size (e.g. 2-3) for fast, deterministic,
     easy-to-reason-about assertions, while production code can default to something larger
     (e.g. `reindexBatch`-sized, 256, or `qdrantPayloadOpBatchSize`-sized, 32) via a default
     parameter value — mirroring `ReindexOptions.Batch`'s "0 → sane default" idiom
     [VERIFIED: internal/store/store.go:3019, `Batch uint32 // scroll page size (0 → a sane default)`].

2. **Should the sweep-level partial-failure test assert on `Store.Migrate`'s own return
   value, or purely on Qdrant's post-hoc state (via a fresh `Store.List`/scroll)?**
   - What we know: D-09 says reconciliation happens by re-derivation, never by trusting
     the write call's error signal — this strongly implies the TEST should also verify
     by re-derivation (scroll for backlog == 0 after a resume), not by asserting a
     specific error value bubbled out of the first (failed) `Migrate` call.
   - What's unclear: Whether `Store.Migrate` should return an error at all when a chunk's
     `SetPayload` fails mid-sweep, or silently continue and report only via return-count
     (`migrated uint64`) plus telemetry.
   - Recommendation: `Store.Migrate` should NOT return an error for a per-chunk
     `SetPayload` failure (matching the anti-pattern warning above) — it should keep
     looping until the backlog is naturally exhausted OR `ctx` is cancelled. The test then
     calls `Migrate` once (observing a nonzero `migrated` count less than the full backlog,
     because the injected failure ate one chunk), then calls `Migrate` again (the "resume")
     and asserts the SECOND call's fresh backlog scroll returns zero before it even starts
     writing.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Everything in this phase | ✓ | 1.26.3 [VERIFIED: go.mod:3] | — |
| `github.com/qdrant/go-client` | `Store.Migrate`, fault-injection interceptor | ✓ | v1.18.3, module-cached [VERIFIED: go.mod:19, module cache listing] | — |
| Qdrant server (testcontainer) | All integration tests (`internal/store/*_test.go`) | ✓ (Docker-based testcontainers; CI stability already addressed by Phase 1's REQ-ci-qdrant-container-stability) | Pinned `qdrant/qdrant:v1.18.2` [VERIFIED: internal/store/store_test.go:35] | `ENGRAM_QDRANT_TEST_ADDR` env var to point at an already-running instance instead of spinning up a testcontainer [VERIFIED: internal/store/store_test.go dialTestClient's `testQdrantAddr` check]. |
| `google.golang.org/grpc` | The fault-injection interceptor's `grpc.UnaryClientInterceptor` type | ✓ (transitive dependency of go-client, already imported by schemaversion_recallgate_test.go) | — | — |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** None beyond the standard testcontainer/env-var
fallback already established repo-wide.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing`, no third-party test framework [VERIFIED: Taskfile.yaml test targets all invoke `go test`] |
| Config file | none — `Taskfile.yaml`'s `test:go` task (`go test ./...`) is the canonical entry point [VERIFIED: Taskfile.yaml:38-40] |
| Quick run command | `go test ./internal/migrate/... -v` (pure unit, no Qdrant needed — package stays leaf/stdlib-only) |
| Full suite command | `go test ./internal/migrate/... ./internal/store/... -count=1 -v` (the `internal/store` half needs Qdrant — testcontainer or `ENGRAM_QDRANT_TEST_ADDR`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-migration-step-registry | `Validate` catches ordering/idempotency violations over a registry (empty registry is valid; a broken fixture chain is not) | unit | `go test ./internal/migrate/... -run TestValidate -v` | ❌ Wave 0 |
| REQ-migration-additive-only-gated | Key-set diff over fixture Steps proves conforming/violating steps are classified correctly, including the declared-vs-actual-divergence case | unit | `go test ./internal/migrate/... -run TestAdditiveOnlyKeySetDiff -v` | ❌ Wave 0 |
| REQ-migration-step-reversibility | `NewStep(rev=nil)` panics; `Irreversible("")` panics; `Reversible`/`Irreversible` round-trip correctly | unit | `go test ./internal/migrate/... -run 'TestNewStep|TestIrreversible|TestReversible' -v` | ❌ Wave 0 |
| REQ-migrate-partial-failure-resume | Forced Nth-`SetPayload` failure against real pinned Qdrant, then a resume call converges backlog to zero | integration | `go test ./internal/store/... -run TestMigratePartialFailureResume -v` | ❌ Wave 0 |
| REQ-migrate-converges-without-lock | A record written mid-sweep (already stamped current) is never matched by `backlogFilter` / never re-processed | integration | `go test ./internal/store/... -run TestMigrateConvergesWithoutLock -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/migrate/... -v` (fast, no Qdrant dependency for the leaf package's own tests)
- **Per wave merge:** `go test ./internal/migrate/... ./internal/store/... -count=1 -v`
- **Phase gate:** Full suite green (`task test` / `go test ./...`) before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/migrate/step.go` + `step_test.go` — `Step`, `NewStep`, `Reversibility`, both constructors, both nil/empty panics — covers REQ-migration-step-reversibility
- [ ] `internal/migrate/registry.go` + `registry_test.go` — `Registry`, `Validate` — covers REQ-migration-step-registry
- [ ] `internal/migrate/additive_test.go` — fixture-only diff test — covers REQ-migration-additive-only-gated
- [ ] `internal/store/migrate.go` + `migrate_test.go` — `Store.Migrate`, `backlogFilter`, fault-injection interceptor extension — covers REQ-migrate-partial-failure-resume, REQ-migrate-converges-without-lock
- [ ] Framework install: none — stdlib `testing` only

## Security Domain

`security_enforcement` is absent from `.planning/config.json` — treated as enabled per this
skill's default rule, but this phase's actual attack surface is minimal: it adds no new
network-facing endpoint, no new auth path, and no new user-controlled input parser. The
registry's callers are all trusted, same-binary Go code (steps are registered by developers,
not by end users or operators at runtime).

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | No | No new auth surface — `Store.Migrate` is an internal method; its CLI exposure (`registerDestructive`, auth-gated like every other operator command) is Phase 4's concern, not this phase's. |
| V3 Session Management | No | N/A |
| V4 Access Control | No (this phase) | `Store.Migrate` itself performs no owner/scope filtering — it is a collection-wide operator sweep, matching `PruneExpired`/`Reindex`/`BackfillShortIDs`'s existing operator-tier D-16 exclusion from the recall/authz boundary [VERIFIED: internal/store/schemaversion_recallgate_test.go:515-558, `operatorMigrationEmitters` classification]. Access control to the CLI verb itself is Phase 4's `registerDestructive` wiring. |
| V5 Input Validation | Yes | `NewStep`'s constructor-level validation (`rev` non-nil, `apply` non-nil) and `Validate`'s registry-level checks (ordering, idempotency) ARE the input-validation control for this phase — enforced at the Go type/construction layer, not a separate validation library (none needed for this domain). |
| V6 Cryptography | No | Nothing cryptographic in this phase. |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|-----------------------|
| A migration step silently narrows recall (e.g., a badly-written step accidentally sets `schema_version` INTO a filter condition somewhere) | Tampering / Information Disclosure | Already covered by Phase 2's `TestSchemaVersionNeverGatesRecall` gate [VERIFIED: internal/store/schemaversion_recallgate_test.go] — this phase does not touch any of the six recall entry points, so the existing gate continues to hold. No new mitigation needed IF `Store.Migrate` stays entirely outside `recallTransmitters`' reachable set (verify this holds once written — `TestRecallEmissionSetIsCompleteAndClassified`'s reachability derivation will flag it automatically if `Store.Migrate` is ever called from a recall entry point, which it should never be). |
| A step declared reversible whose inverse is silently wrong (data corruption on revert) | Tampering | Out of scope for THIS phase — `engram migrate` revert execution is Phase 4 (REQ-migrate-revert). This phase only enforces that an inverse EXISTS when declared reversible, not that it is semantically correct — no automated check can prove semantic correctness of an arbitrary `ApplyFunc` inverse. |

## Sources

### Primary (HIGH confidence)
- `internal/migrate/migrate.go`, `internal/migrate/migrate_test.go` — read directly, this session.
- `internal/store/store.go` (lines 100-150, 2200-3210 spanning `Supersede` doc comments, `PruneExpired`, `BackfillShortIDs`, `RemapOwner`, `Reindex`) — read directly, this session.
- `internal/store/store_test.go` (lines 170-200 `dialTestClient`; 2791-2798 `TestRemapFromPanicsOnEmptyValue`; 3484-3620 concurrency tests; 4640-4720 `setPayloadKeys` hook pattern) — read directly, this session.
- `internal/store/schemaversion_recallgate_test.go` (full file) — the gRPC interceptor seam D-10 extends, read directly, this session.
- `internal/openaiurl/openaiurl.go` — the leaf-package precedent, read directly, this session.
- `go.mod` — read directly, this session (Go 1.26.3, `qdrant/go-client v1.18.3`).
- `/Users/sean/go/pkg/mod/github.com/qdrant/go-client@v1.18.3/qdrant/conditions.go`, `qdrant_common.pb.go`, `points.pb.go` — read directly from the local module cache, this session (`NewRange`, `NewIsEmpty`, `Range` struct fields, `SetPayloadPoints` struct).
- `.planning/phases/02-record-schema-versioning-foundation/02-01-SUMMARY.md` — read directly, this session (the `qdrant.NewValueMap` named-type panic, the monotonic stamp seam).
- `.planning/phases/03-migration-foundation-registry-invariants-sweep/03-CONTEXT.md`, `.planning/REQUIREMENTS.md`, `.planning/STATE.md` — read directly, this session.

### Secondary (MEDIUM confidence)
- Qdrant filter-evaluation semantics (absent-key range/match behavior; `Should`-only-filter OR semantics), sourced via Context7 from `qdrant/qdrant`'s own Rust source (`lib/segment/src/payload_storage/query_checker.rs`, `lib/segment/src/index/query_optimization/optimized_filter.rs`) — [CITED: github.com/qdrant/qdrant/blob/master/lib/segment/src/payload_storage/query_checker.rs].
- Python reference client's local filter re-implementation, fetched directly — [CITED: github.com/qdrant/qdrant-client/blob/master/qdrant_client/local/payload_filters.py] — corroborates the server-side finding above from a second, independently-written source.
- Go sealed-interface pattern and its embedding escape hatch — [CITED: https://rodusek.com/posts/2024/12/14/go-tip-creating-closed-interfaces/].

### Tertiary (LOW confidence)
- None used as load-bearing evidence in this document — every non-trivial claim above is either read directly from this repo's own source this session, or cross-checked against Qdrant's own upstream source via Context7/direct fetch.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new library; every version claim verified against `go.mod` and the local module cache this session.
- Architecture (sealed interface, sweep loop, backlog filter): HIGH — the sealed-interface pattern and its one real gap (nil escape hatch) are confirmed against a cited external source AND this repo's own existing `TestRemapFromPanicsOnEmptyValue` precedent; the backlog-filter finding is confirmed against two independent Qdrant source citations (Rust server logic + Python reference-client re-implementation).
- Pitfalls: HIGH — five of six pitfalls are either already-encountered-and-documented in this repo (Version-cast panic, batch non-atomicity, `-run` false-green) or directly derived from this session's Qdrant source reading (absent-key filter trap); one (init-order panic timing) is a reasoned inference from documented Go semantics, flagged as Assumption A1 rather than overclaimed as VERIFIED.

**Research date:** 2026-08-13
**Valid until:** 30 days (stable domain: no external library, pinned Qdrant versions unlikely to move mid-milestone; re-verify the qdrant/go-client pin if `go.mod` changes before this phase executes).
