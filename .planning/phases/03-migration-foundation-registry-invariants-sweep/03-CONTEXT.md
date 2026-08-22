# Phase 3: Migration Foundation (Registry, Invariants & Sweep) - Context

**Gathered:** 2026-08-13
**Status:** Ready for planning

<domain>
## Phase Boundary

The migration-step **registry** and its **structural invariants**, plus the **sweep**
(`Store.Migrate`) that drives it.

In scope: growing `internal/migrate` (created in Phase 2) into an ordered step registry
with a `Validate` invariant; making additive-only compliance and reversibility
declaration structurally unskippable; and a sweep that survives Qdrant's real batch
`SetPayload` non-atomicity and converges with no collection lock.

Out of scope — belongs to Phase 4: the `engram migrate` CLI (`registerDestructive`,
`--apply`, `--output`, `migrate status` histogram), and registering
`backfill-short-ids` as the v0→v1 first customer. The production registry ships
**empty** this phase and `migrate.CurrentVersion` stays **0**.

</domain>

<decisions>
## Implementation Decisions

### Invariant Enforcement

- **D-01:** Split enforcement by what each half can actually guarantee. Declaration
  *presence* is compile-time unrepresentable; additive-only *behavior* is proven by
  test. The two success criteria (SC2/SC3) say "fails to build **or** fails a test" —
  this takes the stronger option wherever the stronger option exists, rather than
  applying one mechanism uniformly. — **Reversibility:** costly — the constructor
  signature is the package's public surface; widening or relaxing it later touches
  every registration site and every test fixture.

- **D-02:** `Step` is constructed only through `NewStep(from, to Version, addsKeys
  []string, rev Reversibility, apply ApplyFunc)`. There is **no exported struct-literal
  path** — a caller cannot omit `addsKeys` or `rev` because they are positional
  required parameters. "Nobody thought about reversibility" is not a representable
  state (SC3), enforced by the compiler rather than by review or by a test that must
  first be run.

- **D-03:** `Reversibility` is a **sealed interface** (`interface{ isReversibility() }`)
  with exactly two constructors: `Reversible(inverse ApplyFunc)` and
  `Irreversible(reason string)`. Sealing is what prevents a third-party zero value or
  an empty struct from satisfying it. `Irreversible` **panics on an empty reason**, so
  an irreversible step that does not name why fails at package init — before any test
  runs. — **Reversibility:** costly — the sealed interface is the mechanism; unsealing
  it later would silently re-admit the unrepresentable state this phase exists to
  eliminate.

### Additive-Only Proof

- **D-04:** Additive-only behavior is proven by a **key-set diff** over fixture
  payloads, asserted in BOTH directions:
  1. `before ⊆ after` — nothing removed or renamed. Failure names the vanished keys.
  2. `after − before == declared addsKeys` — **set equality**, not a superset check.
     This catches a step that adds an **undeclared** key, not merely one that removes.

  The second assertion is the load-bearing half: a subset/superset check would pass a
  step whose declaration has drifted from its behavior, which is the failure mode the
  declaration exists to prevent.

- **D-05:** The diff test MUST assert a **non-zero fixture count**. A scan or table
  that exercises zero fixtures is vacuously green — this is the exact defect that
  shipped in this milestone's Phase 01 and had to be proven by injection (memory
  `x6v6qxqd6f`). The guard is mandatory here, not optional.

- **D-06:** The diff test runs against **test-only fixture steps**, deliberately not
  against the production registry — which is empty this phase and would therefore make
  the test scan nothing and pass for the wrong reason. Fixtures must cover, at minimum:
  a conforming additive step, an irreversible step with a stated reason, a step that
  removes a key, and a step whose actual adds diverge from its declared `addsKeys`.
  The last two must be shown to FAIL (prove-RED), not merely be absent.

### Sweep Mechanics

- **D-07:** The sweep **re-derives its backlog on every pass** and loops to zero:
  scroll for records below the target version, apply a chunk, re-derive, repeat.
  There is **no persisted cursor**. Resume is therefore just "run it again" — there is
  no stored offset that can go stale, and nothing to reconcile on restart.

- **D-08:** Convergence is proven, not asserted (SC5). Because Phase 2's write path
  stamps the current version *before* the sweep runs, records written mid-sweep arrive
  already-current and never enter the backlog. The test writes new records while the
  sweep is in flight and confirms they are never re-processed. This is why no
  collection lock is needed — the stamp-then-sweep ordering is a stated hard dependency
  on Phase 2, not an incidental property.

- **D-09:** Partial-failure recovery reconciles by **re-derivation** (a fresh
  scroll/count), never by trusting the write call's own success/failure signal (SC4).
  Qdrant chunks a multi-ID `SetPayload` by `PAYLOAD_OP_BATCH_SIZE` and a later chunk
  can error after an earlier chunk has fully committed (qdrant/qdrant#9371), so the
  error value does not describe what landed. `Store.Supersede` already established
  this reconciliation pattern in this codebase — see `internal/store/store.go:129-136`
  and `:2218-2293`.

### Fault Injection

- **D-10:** The forced mid-sequence partial `SetPayload` failure (SC4) is injected via
  a **gRPC unary interceptor** that fails the Nth `SetPayload`, reusing the seam plan
  02-03 already built for the recall-gate proof. The production code path stays
  byte-identical to production, and the induced failure is indistinguishable from a
  real one at the wire. Explicitly rejected: a test-only hook field on `Store` (adds a
  seam to production code and partly tests the hook rather than the sweep) and
  killing/pausing the container (non-deterministic, cannot target a specific
  mid-sequence point, which is precisely what SC4 requires).

### Future Extension

- **D-11:** The per-version decoder door (SC2) is left open via an **optional separate
  interface** checked by type assertion at the point of use — the standard Go extension
  idiom. A future `Decoder` interface can be added in its own phase and existing steps
  need no change, ever. Explicitly rejected: threading a nil-able decoder parameter
  through `NewStep` now (every call site would carry a nil that means nothing, widening
  the constructor before a customer exists).

### Claude's Discretion

- Chunk size for the sweep, and whether it is a constant or derived from Qdrant's
  `PAYLOAD_OP_BATCH_SIZE`.
- The concrete shape of `Validate` (SC1's single invariant over ordering + idempotency)
  and whether ordering is checked by contiguity of `from`/`to` pairs or by topological
  sort.
- Fixture payload construction details, and whether fixtures live in `_test.go` files
  or `testdata/`.
- Error-envelope wording, subject to the repo's `field=<name> hint=<code>` convention.

### Folded Todos

- **Research a versioned payload-migration mechanism**
  (`.planning/todos/pending/2026-08-10-research-versioned-payload-migration-mechanism.md`,
  area `database`, match score 0.6) — this phase IS its resolution. The original
  problem ("no stored schema/payload version; each evolution ships as its own one-shot
  operator command") is answered by Phase 2's `schema_version` plus this phase's
  registry and sweep.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements and scope
- `.planning/REQUIREMENTS.md` § "Migration Mechanism" — the eleven `REQ-migrate*` /
  `REQ-migration*` rows. Phase 3 owns `REQ-migration-step-registry`,
  `REQ-migration-additive-only-gated`, `REQ-migration-step-reversibility`,
  `REQ-migrate-partial-failure-resume`, `REQ-migrate-converges-without-lock`. The
  remaining rows (`REQ-migrate-command`, `REQ-migrate-status-histogram`,
  `REQ-migrate-preview-apply-parity`, `REQ-backfill-shortids-first-step`,
  `REQ-migrate-revert`, `REQ-migrate-never-automatic`) are Phase 4's.
- `.planning/ROADMAP.md` § "Phase 3" — the five success criteria this phase is
  verified against.

### Prior phase output this phase builds on
- `internal/migrate/migrate.go` — the leaf package created in Phase 2. Its
  `CurrentVersion` doc comment states the three reasons the constant is 0 and that
  raising it is "a Phase 3/4 action taken together with registering the step that
  defines the new version — never a standalone bump." Read before touching the
  constant.
- `.planning/phases/02-record-schema-versioning-foundation/02-CONTEXT.md` — Phase 2's
  locked decisions, including the monotonic stamp and the partial-writes-never-stamp
  rule that D-08's convergence argument depends on.
- `.planning/phases/02-record-schema-versioning-foundation/02-03-PLAN.md` — the gRPC
  unary interceptor seam D-10 reuses.

### Codebase precedent
- `internal/store/store.go:129-136` and `:2218-2293` — the existing documentation of
  Qdrant's `PAYLOAD_OP_BATCH_SIZE` chunking and the re-read-the-full-set
  reconciliation pattern, earned during the `Supersede` work. D-09 follows it.
- `internal/store/store.go` — four existing sweep-shaped methods to pattern-match:
  `PruneExpired` (:2594), `BackfillShortIDs` (:2741), `RemapOwner` (:2950),
  `Reindex` (:3133).
- `internal/openaiurl/openaiurl.go` — the stdlib-only leaf-package precedent SC1
  names (single `import "strings"`).

### Upstream
- qdrant/qdrant#9371 — batch `SetPayload` non-atomicity, confirmed upstream. The
  premise of SC4 and D-09.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **gRPC unary interceptor seam** (plan 02-03): already used to capture transmitted
  `*qdrant.Filter` values. D-10 extends the same seam to fail the Nth `SetPayload`.
- **Re-derivation reconciliation** (`Store.Supersede`): the "re-read the FULL requested
  set rather than trusting the error" mechanism, already covering both within-chunk and
  cross-chunk partial application.
- **`internal/migrate`**: the leaf package, the `Version` named type, and
  `CurrentVersion` all exist from Phase 2. This phase grows it; it does not create it.

### Established Patterns
- **Leaf-package convention**: `internal/surfaces` / `internal/openaiurl` are
  stdlib-only leaves imported by `internal/store`. SC1 requires `internal/migrate`
  hold the same shape — zero Qdrant, zero authz.
- **Unrepresentable-over-tested**: a recurring project stance (memory `v0m20y039j`).
  D-01/D-02/D-03 apply it to declaration presence.
- **Set-equality over count assertions**: memory `x6v6qxqd6f` records that
  `len(findings) > 0` passes while catching one of several shapes. D-04 and D-05 are
  written against that lesson.
- **Named types at the Qdrant boundary need an explicit base-type cast**: memory
  `tdt50852ww` — `qdrant.NewValueMap` panics (does not silently drop) on a named type.
  Relevant if any step writes a `Version`-typed value into a payload.

### Integration Points
- `internal/store` imports `internal/migrate` (never the reverse). `Store.Migrate` is
  the sweep entry point and lives in `internal/store`, because it needs the Qdrant
  client the leaf package must not have.
- Phase 4 attaches the CLI to `Store.Migrate` and registers the first real step, so
  the registry's exported surface is a Phase 4 dependency — design it as an API, not
  as an internal detail.

</code_context>

<specifics>
## Specific Ideas

- `NewStep(from, to Version, addsKeys []string, rev Reversibility, apply ApplyFunc)` —
  positional required params, no exported struct literal.
- `Reversibility` sealed via an unexported marker method; `Reversible(inverse
  ApplyFunc)` and `Irreversible(reason string)` as the only constructors;
  `Irreversible` panics on an empty reason.
- Sweep loop shape:
  ```go
  for {
      backlog := scrollWhere(schema_version < target)  // fresh each pass
      if len(backlog) == 0 { break }
      applyChunk(backlog)   // partial failure OK — next pass re-derives
  }
  ```
- Optional decoder attaches later as `type Decoder interface { DecodeAt(v Version, raw
  map[string]any) (Record, error) }`, reached by `if d, ok := step.(Decoder); ok`.

</specifics>

<deferred>
## Deferred Ideas

- **Restricted-writer enforcement of additive-only** — handing each step a
  `PayloadAdder` whose API has no `Delete`/`Set`, making removal unrepresentable rather
  than merely detected. Considered and not chosen for this phase (D-01 keeps behavior
  proof at the test layer). Worth revisiting if a future step is ever authored outside
  this repo, where a test-layer guarantee is weaker.
- **AST scan of step bodies** for `DeletePayload`/`OverwritePayload` calls — rejected
  as a second layer here; 02-02 spent a full review cycle narrowing the escape hatches
  (indirection, helpers, method values) that make such a scan partial.
- **Registering `backfill-short-ids`** — Phase 4, per the recorded milestone decision
  (`e8k7mxb1v6`). Pulling it forward would contradict Phase 2's stated reasoning for
  `CurrentVersion = 0`.

</deferred>

---

*Phase: 3-Migration Foundation (Registry, Invariants & Sweep)*
*Context gathered: 2026-08-13*
