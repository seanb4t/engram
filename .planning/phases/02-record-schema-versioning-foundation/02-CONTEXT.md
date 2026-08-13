# Phase 2: Record Schema Versioning Foundation - Context

**Gathered:** 2026-08-13
**Status:** Ready for planning

<domain>
## Phase Boundary

Every record gains a `schema_version` discriminator on `store.Memory`, stamped on write,
absent-safe for the records already in the collection, visible on the wire, and structurally
barred from ever narrowing recall.

Nothing migrates in this phase. Phase 3 builds the step registry and the `Store.Migrate` sweep;
Phase 4 builds the `engram migrate` CLI and registers `backfill-short-ids` as the first real
step. Phase 2 exists first — and separately — because Phase 5 freezes `schema_version` onto a
permanent proto field number, and a field whose semantics are still settling must not reach the
wire.

**In scope:** the field on `store.Memory`; the stamping rule at the payload codec; the
`internal/migrate` leaf package holding the version type and current-version constant; the
Qdrant payload index; the negative recall-gate proof; the forward/backward compatibility proof.

**Out of scope:** the step registry, reversibility declarations, and `Store.Migrate` (Phase 3);
`engram migrate` and the `backfill-short-ids` fold-in (Phase 4); proto/Connect exposure
(Phase 5); the typed operator renderer (Phase 6); console/CLI surfacing (Phase 7); docs and the
CLAUDE.md "Not used here: database migrations" revision (Phase 8).

</domain>

<decisions>
## Implementation Decisions

### Stamping Seam

- **D-01:** The stamp is written **inside `payload()`** (`internal/store/store.go:545`) — the
  single point at which a full `Memory` becomes a Qdrant payload map. `Upsert` (`:744`),
  `Update` (`:1872`), `Supersede` (`:2279`) and `Reindex` (`:3213`) all funnel through it, so
  "a full write that forgets to stamp" becomes unrepresentable rather than merely tested-for.
  This mirrors Phase 1 D-05/D-20: enforce structurally, then assert the mechanism.
  `payload()` receives the whole `Memory`, so it can read `m.SchemaVersion` as decoded by
  `fromPayload` — which is what makes D-05's monotonic rule implementable at this seam without
  a second read.
  — **Reversibility:** reversible — one function.

- **D-02:** The **partial-write paths never stamp.** `setPayloadKeys` / `deletePayloadKeys`
  (`store.go:391-404`) rewrite one key and leave the rest of the payload untouched; their
  callers are `UpdatePayload`, `SetVisibility`, `IncrementAccess`, the `Supersede` back-stamp,
  archive/restore, `RemapOwner`, and `BackfillShortIDs`. A partial `SetPayload` that stamped
  the current version would assert "every key of this version is present" while having written
  only `visibility` — a false currency claim that would make Phase 3's sweep skip records that
  still need migrating. **Accepted cost:** a v0 record touched by `SetVisibility` stays v0 until
  the sweep reaches it. That is the honest state, and the sweep is what fixes it.
  — **Reversibility:** reversible.

- **D-03:** Criterion 1's "100% of write paths stamp — not a sample" is proven **structurally**:
  a test establishing that the stamping seam is the only door — i.e. every Qdrant point-write in
  `internal/store` routes through `payload()` — in the source-level conformance-gate idiom
  `internal/surfaces` and Phase 1's `internal/keylinks` already established, plus a behavioral
  round-trip assertion per full-write method. The structural half is the load-bearing one: it
  catches the write path that does not exist yet. Memory `x6v6qxqd6f` is the reason — an
  enumerated-shape assertion passed in Phase 1 while catching only one of two bypass shapes.
  **Planner note:** that same memory records that a Phase 1 AST scan was itself bypassed via a
  local variable holding the literal; anchor this gate on identity (does this call site route
  through `payload()`) rather than on argument shape.
  — **Reversibility:** reversible.

- **D-04:** The version type and current-version constant live in a **new stdlib-only
  `internal/migrate` leaf package, created in this phase**, holding only those symbols. Phase 3
  grows the step registry into the same package. This puts the dependency direction
  (`internal/store` imports `internal/migrate`, never the reverse — `REQ-migration-step-registry`)
  right from day one, and avoids moving a symbol mid-milestone that Phase 5's proto mapping and
  Phase 7's CLI would already reference. Follows the `internal/surfaces` / `internal/openaiurl` /
  `internal/keylinks` leaf precedent (Phase 1 D-05).
  — **Reversibility:** costly — undoing means moving the symbol and updating every importer,
  which by Phase 5 includes the proto mapping layer.

### Downgrade Guard (criteria 1 and 5 resolved together)

- **D-05:** The stamping rule is **monotonic**: `max(migrate.CurrentVersion, m.SchemaVersion)`.
  This is the only rule that satisfies criterion 1 (every full write stamps) and criterion 5
  (a record is never downgraded) simultaneously. A new record decodes as `0` and stamps current;
  a record already at v2 rewritten by a v1 binary stays at v2. An unconditional current-stamp
  was rejected precisely because it silently downgrades on the rollback path criterion 5 exists
  to make safe; "preserve if non-zero, else current" was rejected because a genuine v0 record
  (absent key → 0) would then be current-stamped on edit while its payload was never migrated —
  the same false-currency claim D-02 rejects for partial writes.
  — **Reversibility:** reversible.

- **D-06:** **The lossy-rewrite-on-rollback hazard is a documented, accepted limitation — not a
  discovery for the executor to make.** Under D-05 a v1 binary editing a v2 record keeps the v2
  stamp, but it rebuilt the payload from a v1-shaped struct, so `fromPayload` dropped the
  v2-only keys and `payload()` did not re-emit them: the stamp says v2 and the payload is v1.
  This is acceptable *because* migration steps are additive-only
  (`REQ-migration-additive-only-gated`), so what is lost is always re-derivable, and the
  recovery is re-running the sweep. Two alternatives were considered and rejected: an
  unknown-key passthrough map on `Memory` (strictly correct and makes the stamp truthful, but
  it is a new field and a new codec contract — Phase 3 scope at the earliest), and refusing the
  write outright (safest, but reintroduces exactly the hard-reject-on-version-mismatch behavior
  of `webauth`'s `sessionPayloadVersion` that `REQ-schema-version-wire-visible` says this field
  deliberately diverges from). **This limitation must be written down where an operator will
  find it, not left in this file.**
  — **Reversibility:** reversible — adopting passthrough later is additive.

- **D-07:** `Store.Reindex` (`store.go:3213`) takes the **same monotonic rule, no carve-out**.
  It is a full rewrite through `payload()`, so it inherits D-05 like every other full write, and
  because it rebuilds from the decoded struct its output genuinely is current-shaped.
  — **Reversibility:** reversible.
  — **Research flag:** `reindexTargetContents` (`store.go:3326`) compares source and target
  payloads for equality, and `reindex_test.go`'s `payloadKeysEqual` (`:68`) asserts on that
  comparison. Under D-07 a v0 source record becomes v1 in the target, so the two payload maps
  differ by one key. Whether that breaks the existing equality check is a **code question to be
  answered by reading, not guessed at** — hand it to research.

- **D-08:** Forward/backward compatibility is proven by **raw payload injection, in both
  directions**, against real Qdrant. Write a record via raw `SetPayload` carrying
  `schema_version = CurrentVersion + 1` **plus a payload key the binary has never heard of**,
  then assert: it decodes without error; it is fully recallable through every recall path;
  `get_memory` returns it; the version reads back as `CurrentVersion + 1` (not downgraded); and
  a subsequent `Update` leaves it at `CurrentVersion + 1`. Then the mirror case at
  `CurrentVersion - 1`. A test-only override of the version constant was rejected as primary:
  it exercises the stamping plumbing rather than the decode-unknown-payload behavior that is
  what actually breaks on rollback.
  — **Reversibility:** reversible.

### Encoding & Wire Shape

- **D-09:** The field is **`SchemaVersion migrate.Version`** where `type Version int` — a named
  type from the D-04 leaf package. The zero value *is* v0 *is* absent, so criterion 2's
  "no backfill required" falls out of Go's own semantics with no nil handling anywhere,
  including on the recall paths criterion 4 governs. The named type makes D-05's `max` comparison
  type-checked and stops a bare `int` being passed where a version belongs. `*int` was rejected:
  it distinguishes absent from zero, and absent and zero are *defined here* to be the same state,
  so it buys nothing and adds nil-deref surface.
  — **Reversibility:** costly — the type name is referenced by Phase 3's registry, Phase 5's
  proto mapping and Phase 7's CLI once those land.

- **D-10:** The json tag is **`json:"schema_version"` with NO `omitempty`.** This is the
  deliberate criterion-3 divergence from the twice-established `json:"-"` precedent
  (`EmbedderIdentity` `store.go:290`, `IdempotencyFingerprint` `:313`, both of whose comments
  call the hidden tag "deliberate and load-bearing"). `omitempty` was rejected for a specific
  reason: it hides the field exactly when it reads `0`, so every legacy record would look like
  it has no version rather than v0 — hiding it on precisely the records criterion 2 is about.
  A v0 record serializes as `"schema_version": 0`, which is the honest answer, and an operator
  asking "what version is this record" always gets one.
  — **Reversibility:** costly — Phase 5 freezes this onto a proto field number; changing the
  emit rule afterward changes an observable wire contract.

- **D-11:** In **this phase** the field is exposed on **`full=true` recall and `get_memory`
  only.** That falls out of D-10's plain json tag with zero extra code, because `shapeRecall`
  (`internal/server/summary.go:83`) returns `store.Memory` verbatim when `full`. The compact
  `recallView` (`summary.go:96-105`) is a hand-built struct and stays **untouched** — a version
  number is operator/diagnostic data, not something an agent scanning summaries acts on.
  Connect exposure is Phase 5 by design; console and CLI surfacing is Phase 7.
  — **Reversibility:** reversible — adding it to `recallView` later is additive.

- **D-12:** `schema_version` **gets a Qdrant payload index now**, added to `ensureIndexes`
  (`store.go:514`, currently `owner`/`scope`/`created_at`/`short_id`). Phase 3's sweep and
  Phase 4's `migrate status` histogram both count by version, and `ensureIndexes` is Phase 2
  territory. **Consequence to carry into D-13:** an index that exists makes it *easier* for a
  future filter to reach for the field, so the criterion-4 gate — not inconvenience — is now
  the only thing holding that line. The index serves the operator sweep exclusively and must
  never serve recall.
  — **Reversibility:** costly — index creation against a large existing collection is a real
  operation, and dropping it later is an operator action, not a code change.

### Recall-Gate Proof (criterion 4)

- **D-13:** The load-bearing proof is **filter introspection**: call each recall builder with
  representative `Subject`s and options, recursively walk the returned `*qdrant.Filter`
  (`Must` / `Should` / `MustNot` / nested), collect the **set** of field keys it references, and
  assert `schema_version` is absent from that set. This is direct evidence about the object
  actually sent to Qdrant — not about source text, and not about one record happening to match.
  Behavioral-only was rejected: a filter could reference the field and still pass if the test
  data happens to satisfy it. Source-level-only was rejected on the Phase 1 evidence in memory
  `x6v6qxqd6f` — an AST scan there was bypassed by a local variable holding the literal.
  — **Reversibility:** reversible.

- **D-14:** The builder set is **derived and asserted complete, not hand-listed.** The
  enumerated list may stay for readability, but the test must assert that the enumeration
  matches the set of recall entry points that actually exist in the package, **and** that the
  number of filters actually walked is non-zero and equals the expected count. Memory
  `x6v6qxqd6f` is explicit about why: Phase 1's own gate passed while scanning nothing, and an
  `at least one` assertion caught one of two bypass shapes. Set-equality over enumerated shapes,
  never `len(...) > 0`.
  — **Reversibility:** reversible.

- **D-15:** Fail-first is proven by **injecting a real `schema_version` condition into a real
  recall builder, observing RED, and reverting** — recorded as evidence in this phase's
  `VERIFICATION.md`. Memory `x6v6qxqd6f`'s conclusion was exactly this: *prove red then revert
  clean*, and *inject into a real package, not just a fixture*, because a fixture-only proof
  missed the bypass. A permanent committed negative fixture is welcome as a secondary artifact
  but is not the proof.
  — **Reversibility:** reversible.

- **D-16:** Scope of the gate is **recall filters plus the Cedar-derived authz conditions.**
  `ownerOrSharedCondition` (`:769`), `ownerOnlyCondition` (`:796`), `decideBucket` (`:819`) and
  `decideRecord` (`:846`) compile authz decisions **into the same** `*qdrant.Filter`, so walking
  the composed filter from `ownerScopeFilter` (`:885`) covers both in one pass — no separate
  mechanism. The operator tier (`spine-review`, `prune`, `reindex`) is deliberately **excluded**:
  Phase 3's sweep *must* filter by `schema_version` to find its backlog, so a blanket ban across
  all Qdrant queries would make Phase 3 unimplementable. The recall/authz boundary is the
  correct line, and it is the line criterion 4 actually draws.
  — **Reversibility:** reversible.

### Claude's Discretion

Every question resolved to an explicit choice. Open to planning judgment:

- The exact name and signature of the `internal/migrate` version symbol (`Version`,
  `CurrentVersion` are indicative).
- The shape of the filter-walking helper in D-13 and where it lives (test-only helper vs an
  exported introspection aid).
- Whether D-06's operator-facing note lands in `guides/upgrade.md`, a doc comment, or both —
  though Phase 8 owns the docs tail, the *decision* that it must be written down is D-06's.

### Open Question for the Planner

**What does `migrate.CurrentVersion` equal in Phase 2?** Phase 4 registers `backfill-short-ids`
as the v0→v1 step. So Phase 2 either ships `CurrentVersion = 1` — asserting the v1 payload shape
already exists, before the step that produces it is written — or ships `0` and lets Phase 3/4
bump it. This changes what a new record stamps *today* and therefore what Phase 3's sweep sees
as backlog on day one. Not a preference question; resolve it against how Phase 3's registry
defines "current" and raise it at the plan-checker gate if the two phases disagree.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone & requirements
- `.planning/ROADMAP.md` §"Phase 2: Record Schema Versioning Foundation" — goal and the five
  success criteria, including the inverted-cardinality warning that is this phase's headline risk
- `.planning/ROADMAP.md` §"2026-08-12.01 — Record State & Schema Evolution" (Overview) — why
  schema versioning must land before the Connect proto pass (#482): proto field numbers are a
  permanent one-way commitment
- `.planning/REQUIREMENTS.md` §"Record Schema Versioning" — `REQ-schema-version-stamped`,
  `REQ-schema-version-never-gates-recall`, `REQ-schema-version-wire-visible`,
  `REQ-schema-version-forward-compatible`
- `.planning/REQUIREMENTS.md` §"Migration Mechanism" — `REQ-migration-step-registry` (the
  dependency direction D-04 respects) and `REQ-migration-additive-only-gated` (the invariant
  D-06 leans on)
- `.planning/REQUIREMENTS.md` §"v2 Requirements" — `REQ-version-dispatch-codec` is **deferred
  deliberately**; do not reintroduce a per-version decoder in this phase
- `.planning/phases/01-gate-ci-integrity/01-CONTEXT.md` — Phase 1's D-05 (leaf-package
  convention), D-06 (fail-first via good/bad fixtures), D-20 (assert the mechanism, not the
  absence)

### Code this phase modifies
- `internal/store/store.go:185-320` — the `Memory` struct; note `EmbedderIdentity:290` and
  `IdempotencyFingerprint:313`, whose `json:"-"` comments state exactly why they are hidden.
  D-10 diverges from both, on purpose
- `internal/store/store.go:545` — `payload()`, the D-01 stamping seam
- `internal/store/store.go:617` — `fromPayload()`, the decode side; D-08's unknown-key behavior
  is about this function
- `internal/store/store.go:514` — `ensureIndexes`, gaining the D-12 index
- `internal/store/store.go:744` — `Store.Upsert`, the single Qdrant point-write for full records
- `internal/store/store.go:391-404` — the `setPayloadKeys` / `deletePayloadKeys` function-var
  seams; D-02 says these never stamp
- `internal/store/store.go:3213`, `:3326` — `Reindex`'s Upsert and `reindexTargetContents`;
  D-07's research flag lives here
- `internal/store/reindex_test.go:68` — `payloadKeysEqual`, the assertion D-07's flag may break
- `internal/server/summary.go:83-105` — `shapeRecall` / `toRecallView`; D-11 leaves `recallView`
  untouched

### Recall and authz filter builders (the D-13/D-16 coverage set)
- `internal/store/store.go:885` — `ownerScopeFilter` (composes the authz conditions in)
- `internal/store/store.go:1001` — `Search`
- `internal/store/store.go:1081` — `SearchReranked`
- `internal/store/store.go:1099` — `SearchDiscovery`
- `internal/store/store.go:1200` — `listFilter` (backing `List:1232`)
- `internal/store/store.go:1468` — `ListScheduled`
- `internal/store/store.go:769`, `:796`, `:819`, `:846` — `ownerOrSharedCondition`,
  `ownerOnlyCondition`, `decideBucket`, `decideRecord` — the authz half of D-16

### Precedent to mirror, and precedent to diverge from
- `internal/surfaces/`, `internal/openaiurl/`, `internal/keylinks/` — the stdlib-only leaf
  convention D-04 follows and the conformance-gate idiom D-03 uses
- `internal/webauth/resolver.go:59`, `internal/webauth/reseal.go:63` —
  `sessionPayloadVersion`'s **hard reject on version mismatch**. `REQ-schema-version-wire-visible`
  diverges from this deliberately; D-06 records the rejected "refuse the write" option that
  would have reinstated it

### Repo conventions
- `CLAUDE.md` §Conventions — task runner (`task` = lint + test), Conventional Commits, SPDX
  header scope. Note the "Not used here: database migrations" line is revised in **Phase 8**,
  not here
- `.planning/codebase/CONVENTIONS.md`, `.planning/codebase/TESTING.md` — Go package/naming
  conventions and test-tier conventions

### Durable memory (engram spine — `repo:github.com/seanb4t/engram`)
- `x6v6qxqd6f` — **the most load-bearing memory for this phase.** Phase 1's own gates shipped
  with the vacuous-gate defect they existed to eliminate, and the verifier passed them "no
  gaps". Sources D-03's anchor-on-identity warning, D-14's set-equality rule, and D-15's
  prove-red-then-revert-in-a-real-package requirement
- `e8k7mxb1v6` — the 2026-08-12 milestone-scoping decision: additive-only enforced by a
  registration invariant, forward-compat an explicit requirement, version-dispatch codec
  REJECTED because Qdrant filters run server-side on raw payload before any decode
- `v5q7jdbw43` — an assertion is not shippable until run against known-good *and* known-bad
  input
- `axcg19baz6`, `cmjxxswmm2`, `fze45mygy4` — the Qdrant testcontainer/CI facts any test in this
  phase runs on top of; when reading a CI failure, sort by timestamp and read the earliest
- `4g5gbrmv29` — `task`'s default gate never runs `gofmt -l .`; run it before pushing
- Rules `8dfdhfs5nn` (never invent structure in tool-parsed planning artifacts) and
  `2rjnv8sc9a` (never add SPDX headers to `.planning/**`)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **One manual codec, both directions.** `payload()` (`:545`) and `fromPayload()` (`:617`) are
  the only translation between `Memory` and Qdrant payload. There is no reflection-based or
  generated marshaller to fight — D-01's single stamping point already exists as a function.
- **`recallView` is an explicit hand-built struct** (`internal/server/summary.go:96`). Adding a
  field to `store.Memory` therefore does NOT leak into compact recall; D-11's scoping is the
  default behavior, not something to enforce.
- **Function-var seams for Qdrant writes** (`setPayloadKeys`, `deletePayloadKeys`,
  `deletePoint`, `mintCandidate`, `store.go:367-419`) — the established way this package injects
  test probes. If D-03's structural proof needs to count writes, this is the existing idiom.
- **`ensureIndexes` is idempotent by construction** (`:514`) — it tolerates `AlreadyExists`, so
  D-12's index addition is safe to run against an existing collection.

### Established Patterns
- **Two distinct write mechanisms, and the distinction is the phase's central design fact.**
  Full-record replacement through `payload()` → `Upsert` (`Update:1872`, `Supersede:2279`,
  `Reindex:3213`) versus partial key merges through `setPayloadKeys` / `deletePayloadKeys`
  (`UpdatePayload`, `SetVisibility`, `IncrementAccess`, the supersede back-stamp,
  archive/restore, `RemapOwner`, `BackfillShortIDs`). D-01 and D-02 split on exactly this line.
- **Absent-payload-key-reads-as-zero is already the house style.** `AccessCount` (`:242-245`)
  and `EmbedderIdentity` (`:280-290`) both document "a legacy record missing the payload key
  reads 0/\"\" — no backfill". D-09 is the same pattern, which is why criterion 2 needs no new
  mechanism.
- **The `IsEmpty` recall-gate idiom is the trap, not the model.** `SupersededBy` (`:229`) and
  `ArchivedAt` (`:241`) both use a sibling `IsEmpty` condition to soft-hide records from recall.
  Their cardinality is inverted relative to `schema_version` — absence is the *minority* state
  for them and the *majority* state here — so copying the idiom would exclude every
  pre-migration record from recall. This is the single highest-risk finding in the milestone and
  the entire reason criterion 4 and D-13 exist.
- **Payload round-trip tests are an existing convention** — `TestPayloadRoundTripsShortID`,
  `TestPayloadRoundTripsEmbedderIdentity`, `TestPayloadRoundTripsIdempotencyFingerprint`
  (`store_test.go:2924/2942/2965`). A `TestPayloadRoundTripsSchemaVersion` has an obvious home.
- **Tolerant decode has precedent** — `supersedesFromPayload` (`:3270`) and
  `TestSupersedesFromPayloadTolerantDecode` (`store_test.go:4837`) read a scalar as a
  one-element list. If a malformed `schema_version` value needs a tolerant read, this is the
  shape to follow.

### Integration Points
- **`internal/migrate`** — new leaf package (D-04), imported by `internal/store`. Phase 3 grows
  the registry into it.
- **`internal/store/store.go`** — the `Memory` struct, `payload()`, `fromPayload()`,
  `ensureIndexes()`.
- **`go test ./...`** — picks up the new package and the D-03/D-13 gates automatically; no CI
  job or Taskfile target needs adding, matching Phase 1 D-05.
- **CI Qdrant** — Phase 1 moved all four Qdrant-backed packages onto one shared instance with
  per-package collection prefixes. D-08's raw-injection tests and D-13's introspection tests run
  on that; respect the prefix convention when creating collections.
- **This phase's `VERIFICATION.md`** — carries D-15's prove-red-then-revert evidence.

</code_context>

<specifics>
## Specific Ideas

- **Criteria 1 and 5 are in direct tension, and D-05 is the resolution.** Criterion 1 says every
  write stamps the *current* version; criterion 5 says a record is never *downgraded*. `Update`
  and `Supersede` both re-`Upsert` through `payload()`, so an unconditional current-stamp
  satisfies the first and violates the second. A planner that does not notice this will
  implement one criterion and silently break the other. The monotonic rule is the only shape
  that holds both.
- **The `IsEmpty` idiom is a trap with an inverted sign.** Do not reason by analogy from
  `superseded_by` / `archived_at`. Their absence is rare; `schema_version`'s absence is the
  norm at adoption. This is stated in ROADMAP prose, in `REQ-schema-version-never-gates-recall`,
  and again here because it is the failure this phase is designed to prevent.
- **A green `buf breaking` run is not evidence** — that lesson belongs to Phase 5, but its root
  cause is here: a field that exists on `store.Memory` and compiles is not thereby wired
  anywhere. D-03, D-13 and D-15 all insist on evidence about the *object actually produced*,
  not about the code compiling.
- **Do not add a version-dispatch codec.** `REQ-version-dispatch-codec` is explicitly deferred
  to v2, and memory `e8k7mxb1v6` records why: Qdrant filters run server-side on raw payload
  before any decode, so dispatch cannot help on filtered fields at all, and every supported
  version's decoder would then be maintained indefinitely.
- **D-12 raises the stakes on D-13.** Adding the payload index before anything needs it means
  the only thing keeping `schema_version` out of a recall filter is the gate itself. Build the
  gate accordingly.

</specifics>

<deferred>
## Deferred Ideas

- **Unknown-payload-key passthrough on `Memory`** — decoding unrecognized keys into a
  passthrough map and re-emitting them from `payload()`, so an older binary genuinely
  round-trips a newer record losslessly and D-06's stamp becomes truthful. Strictly more
  correct; rejected for this phase as a new field plus a new codec contract. Revisit in Phase 3
  if the sweep's semantics want it.
- **Refusing writes from a binary older than the record's version** — rejected as reinstating
  `sessionPayloadVersion`'s hard-reject behavior that `REQ-schema-version-wire-visible`
  deliberately diverges from. Recorded so the option is not silently rediscovered.
- **`schema_version` on compact `recallView`** — D-11 scopes it to `full=true` / `get_memory`
  for now. Whether the compact agent-facing summary should carry it is properly Phase 7's
  state-surfacing question.
- **A test-only override of the version constant** — considered for D-08 and rejected as
  primary proof. Still a reasonable secondary mechanism if the planner wants to exercise the
  stamping path independently of the decode path.
- **Dropping the D-12 index if Phase 3/4 turn out not to need it** — noted as an operator
  action, not a code change, should the sweep end up counting by scroll instead.

### Reviewed Todos (not folded)
- **`research-versioned-payload-migration-mechanism`**
  (`.planning/todos/pending/2026-08-10-research-versioned-payload-migration-mechanism.md`,
  `resolves_phase: 3`, matched at score 0.6) — **not folded.** Its ask was explicitly "research,
  not a decision", and that research shipped: it produced `.planning/research/` and this
  milestone, with the resulting decisions recorded in memory `e8k7mxb1v6`. It is background for
  Phase 2 and is *delivered* by Phases 3 and 4 (the registry, the sweep, and `engram migrate`
  replacing the one-command-per-evolution accretion it describes). Nothing in it is Phase 2 work.

</deferred>

---

*Phase: 2-Record Schema Versioning Foundation*
*Context gathered: 2026-08-13*
