# Architecture Research

**Domain:** Record schema versioning + migration mechanism for a Qdrant-backed memory MCP server (Go)
**Milestone:** `2026-08-12.01` Record State & Schema Evolution
**Researched:** 2026-08-12
**Confidence:** HIGH (every claim below is read directly off the live `internal/store`, `internal/surfaces`, `internal/webauth`, `cmd/engram`, and `proto/engram/v1` trees at HEAD of `feat/v0.13`, not inferred from a generic pattern)

## Standard Architecture

### System Overview (as it exists today, annotated with where this milestone lands)

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Wire surfaces                                                            │
│  ┌───────────────┐  ┌────────────────────────┐  ┌─────────────────────┐  │
│  │ MCP tools      │  │ Connect (proto/gen)     │  │ operator console    │  │
│  │ (tools.go)     │  │ engram.proto → gen/go   │  │ (SvelteKit SPA)     │  │
│  │                │  │ [MODIFY: +6 Memory     │  │ [MODIFY: render     │  │
│  │                │  │  fields, one-way door]  │  │  new record state]  │  │
│  └───────┬────────┘  └──────────┬──────────────┘  └──────────┬──────────┘  │
├──────────┼──────────────────────┼─────────────────────────────┼──────────┤
│          ▼                      ▼                             │          │
│  ┌───────────────────────────────────────────┐                │          │
│  │ internal/server: deps.* business logic      │  ← BOTH MCP    │          │
│  │ (shared by MCP handlers AND Connect write    │    and Connect │          │
│  │  handlers — Connect handlers are thin proto  │    call this,  │          │
│  │  adapters, never touch store.* directly)     │    never       │          │
│  │  connectapi.go: memoryToProto [MODIFY: +6]   │    store.*     │          │
│  └──────────────────────┬────────────────────────┘  directly    │          │
├─────────────────────────┼──────────────────────────────────────┼──────────┤
│                          ▼                                                │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │ internal/store — the ONLY authorization chokepoint (Cedar PDP        │  │
│  │ compiled into Qdrant filters). Owns Memory struct, payload()/        │  │
│  │ fromPayload() codec, and every bulk sweep (Reindex, BackfillShortIDs,│  │
│  │ RemapOwner, ScanSpine — all Subject-less, all scrollAllPoints-based).│  │
│  │ [MODIFY: +SchemaVersion field, payload()/fromPayload() codec,        │  │
│  │  NEW Store.Migrate sweep method — same shape as the four above]      │  │
│  │        ▲                                                             │  │
│  │        │ consults (pure, I/O-free)                                   │  │
│  │  ┌─────┴──────────────────────┐                                      │  │
│  │  │ internal/migrate  [NEW]     │  ← leaf pkg, same shape as           │  │
│  │  │ ordered step registry,      │    internal/surfaces / openaiurl:    │  │
│  │  │ declared-once + sealed,     │    stdlib-only, zero Qdrant/Subject/ │  │
│  │  │ ValidateSteps() invariant   │    authz dependency, imported BY     │  │
│  │  └──────────────────────────────┘    internal/store, never the reverse│  │
│  └────────────────────────────────────────────────────────────────────┘  │
├────────────────────────────────────────────────────────────────────────────┤
│                          ▼ (single Qdrant collection, DEC-2bv)             │
│                    Qdrant (payload keys only — never a new collection)     │
├────────────────────────────────────────────────────────────────────────────┤
│  cmd/engram — cobra CLI, Subject-less operator tier + Subject-bearing      │
│  client tier, thin adapters over generated Connect stubs / server.        │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │ operator tier (no per-caller authz — bulk sweeps only):              │  │
│  │  reindex | migrate-remap-owner | prune-expired | summarize-missing | │  │
│  │  backfill-short-ids | spine-review  [existing six]                   │  │
│  │  + NEW: `engram migrate` (schema-version sweep, subsumes             │  │
│  │    backfill-short-ids) — a SEVENTH Subject-less operator instance,   │  │
│  │    NOT a nested subcommand of the existing migrate-*/prune-* files   │  │
│  │  renderOperator [MODIFY: typed refactor, #481 — lands FIRST so json/ │  │
│  │    text widening for the 6 new fields is structurally impossible]    │  │
│  └────────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | This milestone |
|-----------|-----------------|-----------------|
| `internal/store` (`store.go`, `spine.go`) | Owns `Memory`, the `payload()`/`fromPayload()` codec, EVERY Qdrant filter (authz + recall gates), and every bulk sweep | MODIFY: add `SchemaVersion` field + codec wiring; add `Store.Migrate` (or similarly named) sweep method mirroring `BackfillShortIDs`/`Reindex`'s scroll-batch-resume shape |
| `internal/migrate` | Pure, dependency-free ordered registry of schema-version transform steps | NEW — leaf package, no Qdrant/Subject/authz import, mirrors `internal/surfaces`'s declared-once + sealed-marker + `ValidateX()` shape |
| `internal/surfaces` | Declares each CROSS-SURFACE PROSE RULE once (a sentence repeated verbatim across cobra usage / jsonschema / MCP description / proto comment / docs / skill), conformance-gated | MODIFY (additive row only): register `RuleSweepScopeOrAllScopesRequired` for the `--scope`/`--all-scopes` guard shared by `summarize-missing`, `spine-review scan`, and (if `engram migrate` gains the same guard) the new command. **Its declare-once machinery does NOT extend to migration steps themselves** — see Question 3 below for why |
| `internal/server` (`connectapi.go`, `tools.go`) | `deps.*` shared business logic; `memoryToProto` is the single conversion chokepoint from `store.Memory` to the wire `Memory` message | MODIFY: `memoryToProto` gains 6 additive field mappings |
| `proto/engram/v1/engram.proto` + `gen/` | Wire contract; `buf breaking` CI-gated | MODIFY: `Memory` message gains 6 fields at numbers 23–28 (currently ends at `citations = 22`) — **one-way door** |
| `cmd/engram` | Thin cobra adapters (`RunE` calling exactly one `st.<Op>` or `deps.*` method); the Subject-less operator tier and the Subject-bearing client tier | NEW file for `engram migrate` (cannot reuse `migrate.go` — already houses `migrate-set-owner`/`migrate-remap-owner`); MODIFY `renderOperator` (typed refactor, #481) |
| operator console (SvelteKit) | Renders memory state for a human operator | MODIFY: surface archived/superseded/scheduled/schema-version state — today it "cannot render the v0.13.x archive tier at all" |

## Findings by Question

### 1. WHERE does `schema_version` belong?

The codebase already has **two established precedents that pull in opposite directions**, and the deciding factor is a single question: *does an external actor (console operator, CLI user, agent) need to see or reason about this value?*

**Precedent A — payload-only, wire-invisible (`json:"-"`).** `EmbedderIdentity` and `IdempotencyFingerprint` are both server-set audit stamps, explicitly documented as "must NEVER cross any JSON wire" because `store.Memory` is returned verbatim on full-response MCP paths. They exist purely for internal reconciliation (reindex-boundary audits, replay-conflict detection) and no surface ever needs to display them.

**Precedent B — plain field, wire-visible (`json:"…,omitempty"`).** `Supersedes`/`SupersededBy`/`ArchivedAt` are explicitly commented "the caller must be able to observe this link on `full=true` recall and `get_memory`" — they are observable record STATE, not an internal audit trail.

`schema_version` is Precedent B, not Precedent A, and the milestone's own stated target features settle this without ambiguity: "Console + CLI surfacing" lists showing "migration state" as an explicit goal, and "Connect record-state parity" explicitly adds `schema_version` alongside `superseded_by`/`supersedes`/`not_before`/`not_after`/`archived_at` as one of the six new wire fields. **Recommendation: `SchemaVersion int` (or a small unsigned type) with a plain `json:"schema_version,omitempty"` tag on `store.Memory`, decoded via `fromPayload` and re-emitted by `payload()` like any other visible field — never `json:"-"`.**

What decides it, restated generally for future fields: if the field only ever participates in a server-internal reconciliation decision (reindex identity, idempotency replay), it is payload-only per Precedent A. If a human or agent client is meant to read it back, it is a plain field per Precedent B — the wire-invisibility of `EmbedderIdentity`/`IdempotencyFingerprint` is a deliberate leak-prevention measure, not the default shape for every payload key.

**The closer analogue than either in-store precedent is `internal/webauth`'s `sessionPayloadVersion`** (PROJECT.md names it explicitly as the pattern to mirror), and it settles the mechanics:

- `const sessionPayloadVersion = 1`, and `Seal` **always overwrites** `s.V = sessionPayloadVersion` before marshalling — the caller can never set it. Mirror this in `payload()`: the write path unconditionally stamps the CURRENT schema-version constant, never trusting an inbound value (there is no client-writable `schema_version` argument on any MCP tool or proto `Store*Request` message — same "server-set only" contract as `ArchivedAt`/`AccessCount`).
- A cookie whose `V` is unset decodes to the JSON zero value (0) and is treated as legacy. Mirror this for `schema_version`: **absent (zero value) means v0** — the exact sentence PROJECT.md uses ("Absent means v0, so adoption needs no backfill"). Every record written before this milestone simply has no `schema_version` key; `fromPayload` reading a missing key already means "int zero value" by Go's own zero-value semantics, so no code change beyond adding the field is needed to make old records read as v0.
- `Resolver.Resolve` REJECTS a version mismatch outright (fails the whole request). **Do not mirror that half of the pattern for `schema_version`.** A session cookie has no independent existence outside the request that presents it — rejecting it is safe. A memory record is a persistent asset the store must keep serving; making it invisible or erroring because its `schema_version` is old would be a correctness regression, not a security control. See Question 4.

### 2. Where does the migration mechanism live?

Split into two questions the milestone conflates but the codebase separates cleanly: **where do the transform STEPS live** (pure logic), and **where does the SWEEP that applies them live** (Qdrant I/O + Subject-less operator dispatch)?

**The sweep belongs in `internal/store`, full stop.** Every existing bulk mutation (`BackfillShortIDs`, `MigrateSetOwner`/`RemapOwner`, `Reindex`, `ScanSpine`) is a `*Store` method for a structural reason, not convention: they are the only code with access to `s.client` (the raw `*qdrant.Client`), the raw payload map shape `payload()`/`fromPayload()` codec, and the Subject-less bulk primitives (`scrollAllPoints`-style scan, `SetPayload` batch writes, `deletePayloadKeys`/`setPayloadKeys` function-var seams). `cmd/engram` has none of that — every existing operator command file (`backfill.go`, `migrate.go`, `reindex.go`) is a ~60–90 line cobra shell whose `RunE` does exactly: parse flags → construct store → call **one** `st.<Verb>(ctx, opts)` → render. A new `Store.Migrate`-shaped method follows that same discipline; `cmd/engram` must stay a thin adapter, per `CLAUDE.md`'s own routing table ("`cmd/engram/` — entrypoint only").

**`cmd/engram` is Subject-less by design for this whole tier**, and that constraint is automatically satisfied here: the sweep never takes a `store.Subject`, never calls `ownerOrSharedCondition`/`decideBucket`/`decideRecord`, and iterates every record in the collection regardless of owner — exactly the same "sixth Subject-less operator-tier command" shape `spine.go`'s `ScanSpine` doc comment states for `spine-review` (a phrase this milestone's `engram migrate` extends to a seventh instance). Authorization stays entirely absent from this path, which is correct: schema migration is a collection-wide structural operation, not a per-caller read/write, and `internal/store` still owns the ONE authz chokepoint (Cedar PDP) — this sweep simply never reaches it, matching `Reindex`/`BackfillShortIDs`/`RemapOwner`'s existing precedent of bypassing it entirely rather than routing through it and hoping every bucket allows itself.

**The transform STEPS (the actual per-record payload logic) are the part worth a new package, and `internal/migrate` is the right shape — but it must be a pure, dependency-free leaf, not a second store-like package.** Two structural reasons:

1. `internal/store`'s `payload()`/`fromPayload()` already operate on `map[string]any` / `map[string]*qdrant.Value` — the natural representation for a step registry to transform is that same raw map, not the typed `store.Memory` struct (a step upgrading, say, a scalar `supersedes` payload key to a list should not have to round-trip through the full typed struct to do it — `Reindex`'s own raw-map write at `store.go` 13-03 is precedent for operating on the raw payload directly).
2. If step *logic* lived inside `store.go` itself, `store.go` (already ~3,350 lines) grows without bound as steps accumulate, and the step registry becomes untestable without a live/fake Qdrant. A pure `internal/migrate` package — zero import of `internal/store`, `qdrant`, or `authz`, exactly mirroring `internal/surfaces` (stdlib-only, `go vet`-clean, unit-testable with no I/O at all) and `internal/openaiurl` ("hoisted into a new stdlib-only leaf package…shared…without a backwards dependency edge") — lets each step be tested as a pure function over `map[string]any`, and lets `internal/store` **import** `internal/migrate` (never the reverse, so no cycle risk) to drive the sweep.

**Do not create a THIRD package for this.** A "migration mechanism" spread across `internal/store` (sweep), `internal/migrate` (steps), and `cmd/engram` (CLI) is the same three-tier shape every other operator command already uses (`internal/store` does the work, `cmd/engram` dispatches, and where pure sharable logic exists it lives in its own leaf package) — it is not a new architectural pattern, it is this milestone's specific instance of an existing one.

**Concrete file-naming trap:** `cmd/engram/migrate.go` already exists and owns `migrate-set-owner`/`migrate-remap-owner`. The new `engram migrate` command needs its own file (e.g. `cmd/engram/migrate_schema.go`) — there is no command-name collision (top-level `migrate` vs. hyphenated `migrate-remap-owner`/`migrate-set-owner` are distinct cobra commands), but the Go filename must not silently overwrite or get confused with the existing one.

### 3. Can migration steps reuse `internal/surfaces`'s declare-once + conformance-gate pattern?

**No, not the literal mechanism — but yes, the underlying structural idiom, and that distinction matters enough to spell out.**

`internal/surfaces`'s actual job is: **the same short SENTENCE must appear, byte-identical, on N independent presentation surfaces** (cobra `Usage`, an MCP arg struct's jsonschema tag, a tool `Description`, a proto field comment, a docs-site page, a skill markdown file) — a text-synchronization problem. Its conformance gate literally greps each surface for an anchored region (`engram:rule:start <ID>` / `engram:rule:end <ID>`) and byte-compares against the registry's `Sentence`. A migration step is not prose repeated across surfaces; it is **one executable transform, applied once, to stored data** — there is no second "surface" restating it. The anchored-region/byte-comparison machinery has nothing to attach to.

What DOES transfer, and should be deliberately reused, is the **shape** `internal/surfaces/rules.go` establishes for "a registry that must not be forgeable or silently incomplete":

- **The unexported, package-private marker.** `ConditionalRule.declared bool` is set ONLY inside `rules.go`'s own literal; Go's cross-package unexported-field rule makes an off-registry `ConditionalRule{...}` literal structurally incapable of setting it. Apply the identical trick to `internal/migrate`: a `Step` struct with an unexported `sealed bool` field, set only inside `internal/migrate`'s own step-literal — so nothing outside that file can splice a step into the applied sequence, and `Store.Migrate` can assert `step.sealed` (or simply only ever range over `migrate.Steps()`, which only that file can populate) as a belt-and-suspenders check mirroring `destructiveByClassification`'s "membership is DERIVED from the table, never re-declared."
- **A single `ValidateX()` invariant-checker over the whole registry, run once (test or generator), not scattered per-call.** `ValidateRules`/`validateRuleSet` checks non-empty fields, no duplicate IDs, no substring collisions. `internal/migrate` needs its own `ValidateSteps()` checking the invariants that actually matter for a migration chain: contiguous version numbering (`step[i].To == step[i+1].From`, no gaps, no overlaps), no duplicate `(From,To)` pairs, and — the one migration-specific property `internal/surfaces` has no analogue for — **idempotency provable by construction**: require each step's `Apply` to stamp the resulting `schema_version` as its own final act, so a payload already at `>= step.To` is never re-entered by a correctly-driven sweep (the sweep's own gather filter, `schema_version < currentVersion`, is what actually enforces idempotency at the call-site level — mirroring `missingShortIDFilter()`'s "the filter IS the idempotency guarantee" pattern already used by `BackfillShortIDs`).
- **Discoverability as data, not prose.** `internal/surfaces.Rules()` returns a defensive copy for anything that needs to enumerate rules (`buildCatalog`, the conformance gate). `internal/migrate.Steps()` (and a `StepsFrom(v int) []Step` helper) gives `Store.Migrate` and any future `engram migrate --list-steps`/JSON-output introspection the same "read the registry, don't hand-maintain a second list" property — this is what makes step ordering and coverage machine-checkable rather than doc-comment-asserted.

So: reuse the *registry discipline* (sealed/unexported provenance marker + one `ValidateX` invariant test + a copy-returning accessor), reject the *literal conformance-gate-over-anchored-text* machinery, because migration steps have no second surface to compare against.

### 4. Read path for a record with an old `schema_version`

The existing precedent is exact and directly names one option: `supersedesFromPayload` (Phase 03.1) reads the `supersedes` payload key and branches on its Qdrant `Value` kind — a `ListValue` decodes as a list; a bare `StringValue` (the pre-03.1 scalar shape) decodes as a one-element list. **This is a pure, in-memory, read-time tolerant decode with no write-back** — the record on disk is never touched by a read; every future read pays the same branch again. It is the "existing Phase 03.1 precedent" the milestone context names verbatim.

The three options map to genuinely different consequences, and the deciding line is **whether the field being evolved participates in a Qdrant filter/index condition**:

- **`fromPayload`/`payload()` (the codec) never runs inside any authz or recall filter.** `ownerOrSharedCondition`, `activeWindowConditions`, the `IsEmpty("superseded_by")`/`IsEmpty("archived_at")` soft-hide gates, `categoryMatchCondition`, `tagMatchConditions` — every one of these builds a `qdrant.Condition` that Qdrant evaluates against the **stored** payload, server-side, before any Go code ever calls `fromPayload` on a result row. Decode-time tolerance therefore has **zero effect** on which records a query returns — it only affects how an already-selected record's Go struct gets populated.
- **Consequence:** a schema evolution that only changes how a field is *represented* (scalar→list, a renamed sub-struct, a default backfilled at decode time) is 100% safely handled by tolerant decode alone, exactly like `supersedesFromPayload` — no sweep is structurally required, ever, for that class of change. This is the cheap, low-risk, always-available option, and it is what to reach for by default.
- **Consequence for correctness (the important one):** a schema evolution that changes a field **which a filter condition matches against by literal key/value** — `owner`, `visibility`, `scope`, `superseded_by`, `archived_at`, `not_before`, `not_after`, or any future indexed key — **cannot** be fixed by decode-time tolerance, because the filter runs before decode and reads the raw on-disk value. An old-shaped record would be silently excluded (or silently included) by a filter that assumes the new shape, and no amount of `fromPayload` cleverness fixes that: the bug is server-side, pre-decode. **This is the one case that structurally requires the explicit sweep** (write the new shape back via `SetPayload`, the same mechanism `BackfillShortIDs`/`RemapOwner`/`ScanSpine` already use) before any filter can rely on the new shape being universally present.
- **`schema_version` itself is the trivial case of the safe category**: nothing in today's codebase filters on `schema_version`, so its own introduction needs no sweep at all — every record simply reads as v0 (absent key), and `payload()` stamps the current version on every future write, converging naturally over time exactly like `sessionPayloadVersion` converges as cookies get re-sealed. **The sweep (`engram migrate`) exists for the *other* fields this milestone's future steps might evolve, and for records that are read-mostly and never re-written** (a stale memory nobody calls `update_memory` on will never pick up a newer schema through the write path alone) — that gap is precisely what an explicit, operator-triggered sweep closes.

**Recommendation: hybrid, and it is not a 50/50 choice — it is "tolerant decode always, sweep only when a filter/index depends on the new shape."** Concretely:
1. Every migration step SHOULD implement tolerant decode in `fromPayload` first (cheap, always correct, no operator action required, matches the 03.1 precedent).
2. A step additionally needs the `engram migrate` sweep's write-back **only if** the field it touches is read by a `qdrant.Condition` anywhere in `store.go`/`spine.go` — audit each new field against `ownerScopeFilter`, `activeWindowConditions`, the two `IsEmpty` soft-hide gates, and any future payload index (`ensureIndexes`'s list) before deciding a step needs the sweep at all.
3. Recall correctness is preserved by construction under this rule: the recall gate was never conditioned on `schema_version`, so no record silently drops out of `search_memory`/`list_memory` because it's schema-old — the only failure mode this rule is designed to prevent is a record that is *visible but mis-filtered* on a field the migration itself is trying to change, which is exactly the case the sweep exists for.

### 5. Data-flow changes for the six new proto fields, and one-way doors

**Five of the six already exist as `store.Memory` Go fields today** (`SupersededBy *string`, `Supersedes []string`, `NotBefore *time.Time`, `NotAfter *time.Time`, `ArchivedAt *time.Time`) — they are simply not yet mirrored onto the Connect wire. `schema_version` is the one genuinely new field. This means the data-flow change is narrow and additive at every hop:

```
store.Memory (5 fields already present, +SchemaVersion new)
      │
      ▼  memoryToProto()  — internal/server/connectapi.go, the SOLE conversion site
      │  (mirrors the existing LastAccessedAt nil-guard: emit unset Timestamp,
      │   never a zero/epoch value, for NotBefore/NotAfter/ArchivedAt;
      │   SupersededBy: deref-or-empty; Supersedes: pass-through []string;
      │   SchemaVersion: direct int→proto-int cast)
      ▼
engramv1.Memory{ …existing 22 fields…, SupersededBy, Supersedes, NotBefore,
                 NotAfter, ArchivedAt, SchemaVersion }   [fields 23–28]
      │
      ▼  gen/go (connect-go stubs) + gen/ts (protobuf-es types)
      ▼
Connect wire  →  operator console (SvelteKit) renders record state
              →  `engram search|list|store` CLI (through renderOperator, #481)
```

**One-way doors to flag explicitly for phase sequencing:**

- **Proto field numbers are permanent once shipped** (`buf breaking` is CI-gated on this repo — the milestone context and PROJECT.md both name this constraint). Fields 23–28 for the six new `Memory` fields is the only available range (the message currently ends at `citations = 22`); once cut, that numbering cannot be renumbered, reordered, or reused even if a field is later deprecated (the existing `approximate = 3 [deprecated = true]` on `ListMemoriesResponse` is precedent for how a mistake here gets stuck forever rather than removed).
- **`schema_version`'s Go *and* wire type must be locked before the proto cut**, not after — this is exactly why PROJECT.md orders "Connect record-state parity…after the schema work because proto field numbers are a permanent one-way commitment." Changing `SchemaVersion` from, say, `int` to `string` after the proto field is shipped would itself be a second breaking change on a field that only just stopped being breakable. Decide the store-side representation (Question 1) fully, including the "server always stamps, never client-writable" contract, before writing the `.proto` line.
- **The typed operator renderer (#481) is a genuine prerequisite for surfacing these fields on the CLI/operator side**, not merely nice-to-have-first: PROJECT.md states it explicitly ("ahead of six new fields flowing through `renderOperator`"). Landing it after the new fields exist means every operator command that renders a `Memory`-shaped result inherits the untyped json/text widening risk for six more fields before the fix exists to prevent it — sequencing it first makes that risk structurally unreachable rather than retrofitted.
- **Deprecating `backfill-short-ids` in favor of `engram migrate`** should follow the exact precedent `migrate-set-owner` already set (`Deprecated = "use: migrate-remap-owner --from-missing --to <owner>"` — a soft, discoverable alias, never a hard removal) rather than deleting the command outright; this is a recommendation grounded in the codebase's own established practice, not a requirement stated in PROJECT.md.

## Suggested Build Order

Ordering below is derived from the dependency chain above (a proto cut needs the Go type locked; console/CLI surfacing needs the proto fields on the wire; the typed renderer needs to exist before six new fields flow through it) plus PROJECT.md's own explicit sequencing statements (gate/CI fixes first; proto after schema; renderer ahead of the six fields).

1. **Gate & CI integrity (#479, #497)** — fix the `pattern:` `\\`-escaping bug that made v0.13.x Phases 1–2's key-link gates no-ops, and the Qdrant testcontainer flakiness. Must land first: every later phase in this milestone authors NEW `internal/surfaces` key-links (at minimum `RuleSweepScopeOrAllScopesRequired`) and needs a build that can actually go red.
2. **Record schema versioning foundation** — `store.Memory.SchemaVersion`, the current-version constant, `payload()`/`fromPayload()` wiring (auto-stamp on write, absent-reads-as-v0 on read), plus the new `internal/migrate` leaf package (pure step registry + `ValidateSteps()`, scaffolded even before any real step is registered). No Qdrant sweep, no proto, no CLI yet — this phase is pure/foundational and independently testable.
3. **Migration mechanism (`engram migrate`)** — `Store.Migrate` sweep method (Subject-less, scroll-batch-resume, driven by `internal/migrate.StepsFrom`), new `cmd/engram/migrate_schema.go` (or similar distinct filename) wiring the versioned command through `registerDestructive` (preview-by-default, matching the v0.13.x destructive tier), explicitly scoped OFF `migrate-remap-owner`/`summarize-missing`/`reindex` per PROJECT.md's stated fence, and subsuming (deprecating, not deleting) `backfill-short-ids`.
4. **Connect record-state parity (#482)** — cut the six additive proto fields (23–28) now that `schema_version`'s type is locked; wire `memoryToProto`; confirm `buf breaking` stays clean in CI. Strictly after step 2/3, per the one-way-door reasoning above.
5. **Typed operator renderer (#481)** — the `renderOperator` structural refactor. Can run in parallel with step 4 (orthogonal machinery, no shared files), but MUST complete before step 6.
6. **Console + CLI surfacing** — operator console renders archived/superseded/scheduled/schema-version state; CLI surfaces the same. Depends on step 4 (fields must exist on the wire) and step 5 (typed renderer must exist to receive them safely).
7. **Registry + docs tail** — register `RuleSweepScopeOrAllScopesRequired`; update `reference/memory-record.md`, `reference/tools.md`; revise `CLAUDE.md`'s "Not used here: database migrations" line (this milestone supersedes it). Trailing cleanup; the rule registration could move earlier (it has no forward dependency on the other phases) but documenting a mechanism before it exists invites drift, so keep it last.

## Anti-Patterns to Avoid

### Anti-Pattern 1: Treating `internal/migrate` as a second `internal/store`

**What people might do:** give `internal/migrate` a Qdrant client, a `Subject` parameter, or authz awareness, on the theory that "it's a migration package, it should own the whole operation."
**Why it's wrong:** it would create the exact import-cycle and layering violation `internal/surfaces`'s own doc comment explicitly designs against ("a stdlib-only leaf package with zero dependency on any other repo package, by construction… internal/server imports this package too — never the other way around"). `internal/store` must stay the ONLY place that touches Qdrant and the authz PDP (the locked invariant this milestone must not quietly violate); a second package with its own Qdrant access duplicates that chokepoint.
**Do this instead:** `internal/migrate` stays pure (`map[string]any` in, `map[string]any` out, or equivalent), imported BY `internal/store`'s sweep method, which alone talks to Qdrant.

### Anti-Pattern 2: A new Qdrant collection for versioned records, or a version-keyed collection name

**What people might do:** stand up a `memories_v2` collection to hold migrated records, cutting over like `reindex` does for a dimension change.
**Why it's wrong:** directly violates `DEC-2bv` ("ONE Qdrant collection for every memory kind — new features add payload keys, NEVER new collections"), a LOCKED invariant this milestone's context explicitly restates. `reindex`'s target-collection pattern exists ONLY because Qdrant vector dimension is physically immutable per-collection — schema version has no such constraint; it is a plain payload key like every other evolution this store has shipped (discovery's `kind`, citations, supersession links, archival).
**Do this instead:** `schema_version` is a payload key on the existing collection, exactly like `EmbedderIdentity` or `SupersededBy` before it.

### Anti-Pattern 3: Gating recall (`Search`/`List`) on `schema_version`

**What people might do:** add a `qdrant.NewRange("schema_version", …)` or similar condition to `Search`/`List` so "only current-schema records" are recalled, treating an old schema version like a soft-hide state (mirroring `IsEmpty("superseded_by")`/`IsEmpty("archived_at")`).
**Why it's wrong:** schema version is a REPRESENTATION concern, not a lifecycle-state concern (unlike supersession/archival, which are genuine record states a caller reasons about). Gating recall on it would silently make un-migrated records invisible — a correctness regression, and one this milestone's own "Absent means v0, so adoption needs no backfill" framing explicitly rules out (adoption must not require an immediate sweep just to keep existing records recallable).
**Do this instead:** recall stays unconditioned on `schema_version`; correctness for filter-relevant fields is handled per Question 4's hybrid rule (sweep only the specific fields a filter depends on), never by a blanket version gate.

## Integration Points

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| `internal/migrate` → `internal/store` | `internal/store` imports `internal/migrate` (one direction only) | `internal/migrate` has NO knowledge of Qdrant, Subject, or authz — it operates on raw payload maps only, mirroring `internal/surfaces`'s zero-dependency leaf-package shape |
| `internal/store.Store.Migrate` → Qdrant | `scrollAllPoints`-style scan + batched `SetPayload` writes, Subject-less | Same shape as `BackfillShortIDs`/`RemapOwner`/`Reindex`; gather filter is `schema_version < currentVersion` (mirrors `missingShortIDFilter()`) |
| `internal/server.memoryToProto` ↔ `proto/engram/v1/engram.proto` | Additive struct-literal field mapping, one conversion chokepoint | Six new fields, numbers 23–28, `buf breaking`-gated in CI |
| `cmd/engram` new `migrate` command ↔ `internal/store` | Thin `RunE` → `st.Migrate(ctx, opts)`, routed through `registerDestructive` | Preview-by-default per the v0.13.x destructive tier; Subject-less like every other operator command |
| `cmd/engram` operator commands → `renderOperator` | Typed struct → JSON/text | #481 refactor must land before commands render the six new fields |
| Operator console (SvelteKit) ↔ Connect API | Reads `engramv1.Memory`'s new fields | Cannot render archived/superseded/scheduled/schema state until Connect record-state parity (step 4) ships |

## Sources

- `internal/store/store.go` (read in full for `Memory`, `payload()`/`fromPayload()`, `Search`/`SearchDiscovery` filter composition, `BackfillShortIDs`, `MigrateSetOwner`, `Reindex`, `supersedesFromPayload`) — HIGH confidence, primary source
- `internal/store/spine.go` (`ScanSpine`, the "sixth Subject-less operator-tier command" doc comment) — HIGH confidence, primary source
- `internal/surfaces/rules.go`, `internal/surfaces/toolclass.go`, `internal/surfaces/surfaces.go` — HIGH confidence, primary source
- `internal/webauth/session.go`, `resolver.go`, `reseal.go` (`sessionPayloadVersion` pattern) — HIGH confidence, primary source
- `cmd/engram/backfill.go`, `migrate.go`, `reindex.go`, `destructive.go` — HIGH confidence, primary source
- `proto/engram/v1/engram.proto` — HIGH confidence, primary source
- `internal/server/connectapi.go` (`memoryToProto`) — HIGH confidence, primary source
- `.planning/PROJECT.md` (milestone target features, locked invariants `DEC-2bv`/`DEC-xa6`) — HIGH confidence, primary source
- `.planning/intel/decisions.md` (`DEC-2bv`, `DEC-xa6` full text) — HIGH confidence, primary source
- `CLAUDE.md` (repo layout/conventions table) — HIGH confidence, primary source

---
*Architecture research for: engram record schema versioning & migration mechanism*
*Researched: 2026-08-12*
