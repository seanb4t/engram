# Phase 2: Record Schema Versioning Foundation - Research

**Researched:** 2026-08-13
**Domain:** Go payload codec / Qdrant filter construction (in-repo, no external libraries)
**Confidence:** HIGH — every claim below was checked by reading the cited file:line in this
session, not inferred from CONTEXT.md's own prose.

## Summary

This phase adds one field (`SchemaVersion migrate.Version`) to `store.Memory`, stamps it inside
the existing `payload()` codec, and proves two negative properties (never gates recall; never
downgrades). Nearly every implementation decision was already locked in `02-CONTEXT.md`. Research
here is not "what stack to use" — it is "does the codebase actually behave the way CONTEXT.md's
decisions assume," because two of CONTEXT.md's decisions (D-01's "Reindex funnels through
`payload()`" and D-07's "no carve-out because Reindex rebuilds from the decoded struct") rest on a
premise that reading the code disproves.

**The single highest-value finding:** `payload()` (`internal/store/store.go:545`) is called from
exactly ONE call site in the whole package — `internal/store/store.go:749`, inside `Store.Upsert`.
`Store.Reindex`'s per-point write (`store.go:3213`) calls `s.client.Upsert` **directly**, bypassing
`Store.Upsert` entirely, and writes `Payload: p.Payload` — the **raw payload map scrolled off the
source collection**, not `qdrant.NewValueMap(payload(m))`. Reindex has never gone through
`payload()`. D-01's claim that "Upsert, Update, Supersede and Reindex all funnel through it" is
true for three of the four names and false for the fourth. This does not break D-07's *conclusion*
("no carve-out needed") — it breaks D-07's *reasoning*, and the plan's D-03 structural conformance
gate must be written to know the difference, or it will red on a write path that was never broken.

**Primary recommendation:** Implement exactly as CONTEXT.md's D-01 through D-16 specify, with one
correction: treat Reindex's raw-map write (`store.go:3213-3223`) as a **named, documented
exception** to the "every full write routes through `payload()`" invariant — mirroring the
existing `embedder_identity` exception at `store.go:3207-3210` — rather than asserting Reindex
"inherits" the monotonic stamp. Reindex preserves `schema_version` (present or absent) by verbatim
copy, which is actually *more* preserving than a `payload()`/`fromPayload()` round-trip and is
consistent with the milestone's additive-only, sweep-converges-later design.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

D-01 through D-16, verbatim in `.planning/phases/02-record-schema-versioning-foundation/02-CONTEXT.md`
(`<decisions>` block). Restated compactly here; do not re-litigate:

- **D-01:** Stamp is written inside `payload()` (`store.go:545`) — the single point a full `Memory`
  becomes a Qdrant payload map.
- **D-02:** Partial-write paths (`setPayloadKeys`/`deletePayloadKeys`) never stamp.
- **D-03:** Criterion 1 proven structurally — every Qdrant point-write in `internal/store` routes
  through `payload()` — anchored on call-site identity, not argument shape.
- **D-04:** Version type + current-version constant live in a new stdlib-only `internal/migrate`
  leaf package created this phase.
- **D-05:** Stamping rule is monotonic: `max(migrate.CurrentVersion, m.SchemaVersion)`.
- **D-06:** The lossy-rewrite-on-rollback hazard (v1 binary editing a v2 record drops v2-only keys
  but keeps the v2 stamp) is a documented, accepted limitation — must be written down for an
  operator, not left in CONTEXT.md.
- **D-07:** `Store.Reindex` takes the same monotonic rule, no carve-out. **Research flag issued
  here — see Findings below: the stated reasoning is wrong, the conclusion happens to still hold.**
- **D-08:** Forward/backward compatibility proven by raw payload injection against real Qdrant, in
  both directions (`CurrentVersion+1` and `CurrentVersion-1`), asserting full recallability,
  `get_memory` return, non-downgraded version, and a subsequent `Update` preserving it.
- **D-09:** Field is `SchemaVersion migrate.Version` where `type Version int`; zero value is v0 is
  absent — no `*int`.
- **D-10:** json tag is `json:"schema_version"` with **no** `omitempty` — the deliberate divergence
  from `EmbedderIdentity`/`IdempotencyFingerprint`'s `json:"-"`.
- **D-11:** This phase exposes the field on `full=true` recall and `get_memory` only;
  `recallView` stays untouched.
- **D-12:** `schema_version` gets a Qdrant payload index now, added to `ensureIndexes`.
- **D-13:** Criterion-4 proof is filter introspection: walk the actual `*qdrant.Filter` object each
  builder returns and assert `schema_version` is absent from the referenced-key set.
- **D-14:** The builder set is derived and asserted complete (set-equality), never hand-listed with
  a bare non-zero count.
- **D-15:** Fail-first proof is inject-a-real-condition-into-a-real-builder, observe RED, revert —
  recorded in this phase's `VERIFICATION.md`.
- **D-16:** Gate scope is recall filters plus the Cedar-derived authz conditions compiled into the
  same filter; the operator tier (spine-review/prune/reindex) is deliberately excluded.

### Claude's Discretion

- Exact name/signature of the `internal/migrate` version symbol (`Version`, `CurrentVersion` are
  indicative, not binding).
- Shape and location of the D-13 filter-walking helper (test-only helper vs. exported introspection
  aid).
- Whether D-06's operator-facing note lands in `guides/upgrade.md`, a doc comment, or both (Phase 8
  owns the docs tail; the decision that it must be written down is D-06's, this phase).

### Open Question for the Planner (unresolved, CONTEXT.md hands this to planning, not research)

**What does `migrate.CurrentVersion` equal in Phase 2?** `0` (Phase 3/4 bump it when
`backfill-short-ids` becomes the registered v0→v1 step) or `1` (asserting the v1 shape already
exists before the step that produces it). This is a cross-phase consistency question the CONTEXT.md
explicitly says is "not a preference question" — resolve against how Phase 3's registry defines
"current," and flag at the plan-checker gate if the two phases would disagree. Research has no
additional evidence to add: `internal/migrate` does not exist yet (confirmed below), so there is no
existing Phase-3-authored code to read for the answer.

### Deferred Ideas (OUT OF SCOPE for this phase)

- Unknown-payload-key passthrough on `Memory` (Phase 3 candidate if the sweep wants it).
- Refusing writes from a binary older than the record's version (rejected permanently — reinstates
  `sessionPayloadVersion`'s hard-reject, which `REQ-schema-version-wire-visible` deliberately
  diverges from).
- `schema_version` on compact `recallView` (Phase 7's state-surfacing question).
- A test-only override of the version constant for D-08 (acceptable secondary mechanism only).
- Dropping the D-12 index later (an operator action, not a code change).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-schema-version-stamped | Every record write stamps `schema_version`; absent reads as v0, no backfill | Confirmed: `payload()` is the sole codec write path (1 call site, `store.go:749`); confirmed the house style for absent-key-reads-zero already exists (`AccessCount`, `EmbedderIdentity`) — D-09's zero-value approach needs no new mechanism |
| REQ-schema-version-never-gates-recall | `schema_version` never appears in a recall/authz filter | Complete, file:line-anchored enumeration of every `*qdrant.Filter`-building function in scope, below |
| REQ-schema-version-wire-visible | Plain json tag, no reject/hide/downgrade | Confirmed `shapeRecall`'s `full=true` branch returns `store.Memory` verbatim (`summary.go:87`) and `getMemory` returns `store.Memory` directly (`tools.go:1721`) — D-11's zero-extra-code claim holds |
| REQ-schema-version-forward-compatible | Newer-than-binary version reads, recalls, and is not downgraded | Confirmed `fromPayload` is a tolerant field-by-field decoder (`if v, ok := p[key]; ok` per field) that silently ignores unrecognized keys — this is exactly the behavior D-08's raw-injection test needs and it already exists for every other field |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| `schema_version` field + monotonic stamping | Database/Storage (`internal/store`) | — | The codec (`payload()`/`fromPayload()`) is the store's own translation layer; no server/API tier involvement this phase |
| Version type + current-version constant | Database/Storage (new `internal/migrate` leaf) | — | Deliberately dependency-direction-correct now: `internal/store` imports `internal/migrate`, never reverse, so Phase 3's registry can grow into the same package without a later move |
| Recall-gate negative proof | Database/Storage (`internal/store`'s Search/List/... filter builders) | — | Recall/authz filters are built exclusively in `internal/store`; nothing in `internal/server` constructs `*qdrant.Filter` |
| Qdrant payload index | Database/Storage (`ensureIndexes`) | — | Index management already lives entirely in `internal/store` |
| Wire visibility (full=true, get_memory) | API/Backend (`internal/server`) | — | Falls out of D-10's plain json tag with zero code change — `internal/server` need not be touched this phase (confirmed below) |

## Standard Stack

No third-party libraries are introduced this phase. `internal/migrate` is explicitly stdlib-only
per D-04 (mirrors `internal/openaiurl`, which imports only `"strings"`). No `npm install` / `pip
install` / `go get` step applies.

### Package Legitimacy Audit

**N/A — this phase installs no external packages.** `internal/migrate` is a new first-party leaf
package with zero third-party dependencies, matching the `internal/surfaces`/`internal/openaiurl`/
`internal/keylinks` precedent (all three import only Go stdlib packages; verified by reading
`internal/openaiurl/openaiurl.go`, whose only import is `"strings"`).

## Architecture Patterns

### System Architecture Diagram

```
                    ┌─────────────────────────────────────────────┐
                    │         internal/store (Database tier)        │
                    │                                                 │
  full write ──────►│  Store.Upsert (store.go:731)                   │
  (Upsert/Update/    │    └─► payload(m) (store.go:545)  ◄── ONLY    │
   Supersede)        │          stamps SchemaVersion =                │
                    │          max(migrate.CurrentVersion,            │
                    │              m.SchemaVersion)   [D-05]           │
                    │          └─► qdrant.NewValueMap(...)             │
                    │               └─► s.client.Upsert ──► Qdrant     │
                    │                                                 │
  partial write ───►│  setPayloadKeys/deletePayloadKeys (:391-404)   │
  (SetVisibility,    │    NEVER call payload() — NEVER stamp [D-02]   │
   IncrementAccess,  │    └─► s.client.SetPayload/DeletePayload       │
   RemapOwner, ...)  │                                                 │
                    │                                                 │
  Reindex ─────────►│  Store.Reindex (store.go:3052)                 │
  (raw copy, NOT     │    scroll SOURCE → fromPayload (decode-only,   │
   through payload)  │    for content/tags) → s.client.Upsert         │
                    │    DIRECTLY with Payload: p.Payload (RAW map)  │
                    │    ── bypasses payload() entirely; NEVER calls │
                    │       payload() anywhere in this function      │
                    │    [D-01's premise wrong here; D-07's outcome  │
                    │     still holds — see Findings]                 │
                    │                                                 │
  recall read ─────►│  Search/SearchReranked/SearchDiscovery/List/   │
  (5 entry points)   │  ListScheduled → build *qdrant.Filter via      │
                    │  ownerScopeFilter/listFilter/ownerOrShared-     │
                    │  Condition/ownerOnlyCondition/activeWindow-     │
                    │  Conditions/scheduledStateCondition — NONE      │
                    │  reference "schema_version" [D-13/D-16 proof]   │
                    └─────────────────────────────────────────────┘
                                        │
                                        ▼
                    ┌─────────────────────────────────────────────┐
                    │      internal/server (API tier, untouched)     │
                    │  shapeRecall(full=true) returns store.Memory   │
                    │  VERBATIM (summary.go:87) → schema_version     │
                    │  appears on the wire automatically, zero code  │
                    │  change [D-11 confirmed]                        │
                    └─────────────────────────────────────────────┘
```

### Recommended Package Layout

```
internal/migrate/          # NEW leaf package this phase, stdlib only
├── migrate.go             # type Version int; const CurrentVersion Version = ? (see open question)
└── migrate_test.go
internal/store/
├── store.go                # Memory struct gains SchemaVersion; payload()/fromPayload() gain the
│                            # stamp/decode; ensureIndexes gains the index
├── store_test.go            # TestPayloadRoundTripsSchemaVersion alongside the three siblings
├── reindex_test.go          # unaffected — payloadKeysEqual continues to pass (see Findings)
└── <new file, e.g. schemaversion_gate_test.go>  # D-03 structural gate + D-13/D-14/D-15 negative
                             # recall-gate proof; mirrors collectionprefix_conformance_test.go's
                             # go/ast-based, source-level scan idiom
```

### Pattern 1: The manual codec is the only door — and Reindex is outside it

**What:** `payload()` (`store.go:545`) is called from exactly one place: `store.go:749`, inside
`Store.Upsert`. Verified by:
```bash
$ rg -n "payload\(m\)|= payload\(|Payload: payload\(|Payload:.*payload\(" internal/store/store.go
749:			Payload: qdrant.NewValueMap(payload(m)),
```
`Update` (`store.go:1872`) and `Supersede` (`store.go:2279`) both call `s.Upsert(...)`, which
funnels through `payload()` transitively. `Reindex`'s per-point write
(`store.go:3213`) calls `s.client.Upsert` directly — the *raw* Qdrant client, not `Store.Upsert` —
and its `Payload` field is `p.Payload` (the point's raw payload as scrolled off the *source*
collection at `store.go:3118-3124`), not `qdrant.NewValueMap(payload(...))`.

**When to use:** D-03's structural gate ("every Qdrant point-write in `internal/store` routes
through `payload()`") must be written with Reindex as a **named exception**, not asserted as an
inheritor. The existing doc comment at `store.go:3207-3210` already calls out "the one intentional
additive exception to the verbatim-payload invariant" for the `embedder_identity` raw-map write
inside Reindex — the D-03 gate should extend that same exception list to cover Reindex's entire
per-point write, not just the embedder-identity line within it.

**Example (existing embedder_identity exception, the pattern to mirror for the gate's allowlist):**
```go
// Source: internal/store/store.go:3207-3212 (read verbatim this session)
if opts.Identity != "" {
    // The one intentional additive exception to the verbatim-payload
    // invariant: a single guarded raw-map key write, never a
    // Memory/payload() round-trip (see the Reindex doc comment).
    p.Payload[embedderIdentityKey] = qdrant.NewValueString(opts.Identity)
}
if _, err = s.client.Upsert(ctx, &qdrant.UpsertPoints{
    CollectionName: opts.Target,
    Wait:           qdrant.PtrOf(true),
    Points: []*qdrant.PointStruct{{
        Id:      p.Id,
        Vectors: qdrant.NewVectors(vec...),
        Payload: p.Payload,
    }},
}); err != nil {
```

### Pattern 2: AST-based source-level conformance gate (the D-03/D-13/D-14 idiom)

**What:** `internal/store/collectionprefix_conformance_test.go` already implements exactly the
"identity, not argument shape" scan D-03 calls for, using `go/ast`, `go/parser`, `go/token`,
`go/types` to walk source text and find call-site expressions matching a pattern (there: a raw
string literal argument at a `New(...)`/`store.New(...)` call site). It lives *inside*
`internal/store`'s own test scope specifically to read sibling packages as plain source text
without creating an import cycle (`internal/store` cannot import `internal/server`, which already
imports it).

**When to use:** D-03's gate ("every full write in `internal/store` routes through `payload()`")
and D-13's filter-walker are both same-package, same-idiom problems: scan `internal/store`'s own
`.go` files, find `*qdrant.PointStruct{...Payload: ...}` composite literals (for D-03) or
`*qdrant.Filter{...}` construction call sites (for D-13/D-14's completeness assertion), and assert
against the derived set — never a hand-typed list with an "at least one" check. This mirrors the
exact defect memory `x6v6qxqd6f` records: Phase 1's own gate scanned zero call sites and still
passed, because it checked `len(...) > 0` rather than set equality against a derived enumeration.

**Example:**
```go
// Source: internal/store/collectionprefix_conformance_test.go:41-53, 68-79 (read verbatim this
// session) — the existing go/ast scan idiom to mirror, not to reuse directly (different target
// shape: composite-literal field value vs. call argument).
import (
    "go/ast"
    "go/parser"
    "go/token"
    "go/types"
    ...
)

type qdrantPackage struct {
    name             string
    dir              string
    allowUnqualified bool
}
```

### Anti-Patterns to Avoid

- **Copying the `superseded_by`/`archived_at` `IsEmpty` idiom for `schema_version`:** their
  cardinality is inverted — absence is the *minority* state for supersession/archival and the
  *majority* state for schema versioning at adoption. `qdrant.NewIsEmpty("schema_version")`
  anywhere in a recall filter would silently exclude every pre-migration record. This is the
  headline risk CONTEXT.md, ROADMAP.md, and REQUIREMENTS.md all separately call out — do not
  reason by analogy from the sibling fields.
- **Trusting D-07's stated reasoning for Reindex ("rebuilds from the decoded struct… genuinely
  current-shaped"):** false — see Pattern 1. Trust the *outcome* (no carve-out needed), not the
  *mechanism* described.
- **Writing the D-03 gate as an argument-shape check** (e.g., "does this call pass a `Memory`
  value?") instead of a call-site-identity check. Memory `x6v6qxqd6f` records that exactly this
  kind of shape-based check was bypassed in Phase 1 by a local variable holding the literal.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Go source-level conformance scanning | A new hand-rolled string-matching/regex scan of `store.go` | `go/ast` + `go/parser`, exactly as `collectionprefix_conformance_test.go` already does in the same package | Regex-based scanning is exactly the class of defect memory `x6v6qxqd6f` warns about — it is bypassed by trivial refactors (e.g., wrapping the call in a local variable). AST inspection sees the actual expression at the call site. |
| Filter object introspection | Reimplementing `*qdrant.Filter` traversal from scratch per test | A single recursive walker over `Must`/`Should`/`MustNot` (proto-generated, plain struct fields — no reflection needed) that collects referenced field keys from `qdrant.Condition`'s `FieldCondition`/`IsEmptyCondition`/`Filter` (nested) variants | D-13 requires exactly one correct implementation reused by every builder's test, not five hand-copied traversals that can silently drift apart |

**Key insight:** This phase's "don't hand-roll" risk is not a third-party-library risk (there are
none) — it is the risk of re-deriving an ad-hoc conformance-scan or filter-walk mechanism when a
working, in-package precedent for both already exists and should be extended, not reinvented.

## Runtime State Inventory

**Trigger check:** this phase is additive (new field, new package), not a rename/refactor/migration
of existing state. The Runtime State Inventory protocol does not apply — no existing string, key,
or identifier is being renamed. Explicitly stated so the planner does not need to separately
determine this: **N/A, not a rename/refactor/migration phase.**

## Criterion-4 Filter-Builder Enumeration (the second highest-value research ask)

Complete, file:line-anchored set of every function in `internal/store` that constructs a
`*qdrant.Filter` or a `*qdrant.Condition` consumed by `Search`, `SearchReranked`,
`SearchDiscovery`, `List`, or `ListScheduled` — derived by reading `store.go` in full this session,
cross-checked against every `IsEmpty(` occurrence in the package.

### The five recall entry points (criterion-4 scope)

| Entry point | Line | Filter construction |
|---|---|---|
| `Search` | `store.go:1001` | `f := s.ownerScopeFilter(ctx, scope, subj)` (:1023), then appends `activeWindowConditions` (:1024), `qdrant.NewIsEmpty("superseded_by")` (:1028), `qdrant.NewIsEmpty("archived_at")` (:1034), `tagMatchConditions` (:1035), `categoryMatchCondition` (:1036), `createdRangeCondition` (:1039) |
| `SearchReranked` | `store.go:1081` | **No independent filter construction** — calls `s.Search(...)` at `:1085` and reorders the already-filtered result. Its coverage is inherited from `Search`, not separate. |
| `SearchDiscovery` | `store.go:1099` | Builds its own inline `must` slice (does NOT call `ownerScopeFilter`): `qdrant.NewMatch("category","discovery")` (:1118), optional scope/kind matches (:1119-1124), `s.ownerOrSharedCondition` (:1125), `qdrant.NewIsEmpty("superseded_by")` (:1128), `qdrant.NewIsEmpty("archived_at")` (:1132) |
| `List` | `store.go:1232` | `f := s.listFilter(ctx, scope, subj, opts)` (:1260), then appends `activeWindowConditions` (:1261), `qdrant.NewIsEmpty("superseded_by")` (:1265), `qdrant.NewIsEmpty("archived_at")` (:1270), `createdRangeCondition` (:1271) |
| `ListScheduled` | `store.go:1468` | Builds its own inline filter (does NOT call `listFilter` or `ownerScopeFilter`): `qdrant.NewMatch("scope", scope)`, `s.ownerOnlyCondition` (:1495), `scheduledStateCondition` (:1496), `qdrant.NewIsEmpty("superseded_by")` (:1500), `qdrant.NewIsEmpty("archived_at")` (:1505), `createdRangeCondition` (:1507) |

### Shared helper functions (called by more than one entry point above)

| Helper | Line | Called by |
|---|---|---|
| `ownerScopeFilter` | `store.go:885` | `Search` only (`:1023`) — **not** `List`/`SearchDiscovery`/`ListScheduled`, which each build their own scope+authz composition inline or via `listFilter` |
| `listFilter` | `store.go:1200` | `List` only (`:1260`) |
| `ownerOrSharedCondition` | `store.go:769` | `ownerScopeFilter` (:890), `SearchDiscovery` (:1125), `listFilter` (:1205) — the authz half of D-16, reached by 3 of the 5 filter-building paths |
| `ownerOnlyCondition` | `store.go:796` | `ListScheduled` only (`:1495`) |
| `decideBucket` | `store.go:819` | Called from inside `ownerOrSharedCondition` (:774-775) and `ownerOnlyCondition` (:801) — the PDP call-site indirection, not a filter builder itself |
| `activeWindowConditions` | `store.go:943` | `Search` (:1024), `List` (:1261) — **not** `SearchDiscovery` or `ListScheduled` (the latter uses the inverse, `scheduledStateCondition`, by design) |
| `tagMatchConditions` | `store.go:902` | `Search` (:1035), `listFilter` (:1209) |
| `categoryMatchCondition` | `store.go:924` | `Search` (:1036), `listFilter` (:1206) |
| `createdRangeCondition` | `store.go:1173` | `Search` (:1039), `List` (:1271), `ListScheduled` (:1507) — **not** `SearchDiscovery` |
| `scheduledStateCondition` | `store.go:1443` | `ListScheduled` only (:1496) |

### A finding that narrows D-16's scope, not widens it: `decideRecord` does not build a filter

CONTEXT.md's D-16 names `decideRecord` (`:846`) alongside `decideBucket`/`ownerOrSharedCondition`/
`ownerOnlyCondition` as compiling "into the same `*qdrant.Filter`." Reading the actual call sites
(`store.go:1676`, `:1704`, `:1739` — inside `GetReadable`, `getWritable`, `OwnedOrAbsent`) shows
`decideRecord` is invoked *after* a point is already fetched by id (`Store.Get`, an ungated raw
point read), as an in-memory decision over the decoded `Memory` struct's fields. It never
constructs or touches a `*qdrant.Filter` or `*qdrant.Condition`. This is consistent with, not
contradictory to, D-16's actual gate scope: `GetReadable`/`getWritable`/`OwnedOrAbsent` back
id-addressed operations (`Get`, `Update`, `Delete`, `Supersede` target validation), which are
**not** in criterion 4's named list (`Search`/`SearchReranked`/`SearchDiscovery`/`List`/
`ListScheduled`) and are explicitly documented elsewhere in the file as "ungated" (e.g.
`store.go:1027`, `:1131`, `:1499`: "`get_memory` (`Store.Get`) stays ungated"). **Consequence for
the plan:** if D-13's derived-builder-set scan is implemented as "every function returning
`*qdrant.Filter`", `decideRecord` will correctly and automatically NOT appear in that set — no
special-casing needed. If the scan is instead implemented as "every function taking an
`authz.Action`", it WOULD incorrectly sweep in `decideRecord` alongside `decideBucket`; anchor the
scan on the `*qdrant.Filter`/`*qdrant.Condition` return type, not on authz-action parameters, to
avoid this false inclusion.

### Reusable test infrastructure

No existing test helper already walks `*qdrant.Filter` conditions (checked: `rg -n "range.*Must\b|
range.*Should\b|func.*[Ww]alk.*[Ff]ilter" internal/store/*.go` returns nothing). D-13's filter
walker is new code this phase. The closest structural precedent for "AST/source-level scan living
inside `internal/store`'s own test scope" is `collectionprefix_conformance_test.go` (Pattern 2
above) — a different traversal target (source AST vs. runtime `*qdrant.Filter` object graph) but
the same "derive the set, don't hand-list it" design already proven in this package.

## D-07 Research Flag — Resolved

**The literal question asked:** does adding a `schema_version` key break `reindexTargetContents`'s
comparison (or `payloadKeysEqual`'s assertion in `reindex_test.go:68`), and does `engram reindex`
need a carve-out?

**Answer: No carve-out needed — CONTEXT.md's "no carve-out" stance holds — but for a different
reason than D-07 states, and the plan should record the corrected reason so a future reader does
not re-derive the wrong mental model from D-07's text.**

1. **`reindexTargetContents` (`store.go:3326-3350`) does not compare whole payloads at all.**
   Reading its body: it fetches target points and returns `map[string]reindexTarget` where
   `reindexTarget` (`store.go:3310-3314`) holds exactly three decoded fields — `content`, `tags`,
   `identity` (`embedder_identity`). It is consumed at `store.go:3169-3171` purely for the
   resume-skip decision (`ti.content == content && tagsEqual(...) && (opts.Identity == "" ||
   ti.identity == opts.Identity)`). `schema_version` participates in none of this — the function
   cannot "see" a payload-key-count mismatch because it never looks at the key set, only three
   named fields.

2. **`payloadKeysEqual` (`reindex_test.go:68-84`) is a test-only helper, invoked once**, at
   `reindex_test.go:245` inside `TestReindexRoundtrip`, comparing `want := srcBefore[id].payload`
   (the SOURCE collection's raw scrolled payload, captured *before* Reindex runs, at
   `reindex_test.go:222`) against `got := scrollPoints(tgt)[id].payload` (the TARGET's raw scrolled
   payload *after* Reindex runs). Because Reindex's actual write (`store.go:3213-3223`) copies
   `p.Payload` — the exact map scrolled from the source at `store.go:3118-3124` — verbatim into the
   target (the sole exception being the guarded `embedder_identity` raw-key overwrite at
   `store.go:3207-3212`, which `TestReindexRoundtrip`'s scenario does not trigger since
   `opts.Identity` is unset in that test), `want` and `got` will contain an identical
   `schema_version` key (same value, or both absent) regardless of what value the field carries.
   **The test cannot fail due to the new key**, because the new key, once `payload()` starts
   writing it (via `Store.Upsert`/`Update`/`Supersede`), is copied byte-for-byte by Reindex's raw
   map write — Reindex never rebuilds the payload from a `Memory` struct, so it cannot omit a field
   the source had.

3. **The genuine consequence, which is NOT what D-07 asserts:** because Reindex's write never calls
   `payload()`, D-05's monotonic stamp (`max(migrate.CurrentVersion, m.SchemaVersion)`) **never
   executes during Reindex**. A v0 source record reindexes to a v0 target record — the
   `schema_version` key stays absent on both sides, copied as-is — not "current-shaped" as D-07's
   text claims. This is *consistent* with the milestone's design (the sweep, not Reindex, is
   responsible for advancing a record's version; Reindex's entire contract, per its own doc
   comments and `seedSource`'s test setup at `reindex_test.go:94-99`, is to preserve **every**
   payload key verbatim, known or unknown — `schema_version` is simply one more key that
   contract already covers). No code change to `Reindex` is required to make this work correctly.

4. **The one place this DOES matter: the D-03 structural gate.** If the plan's "every full write
   routes through `payload()`" gate walks `internal/store`'s Qdrant-writing call sites and expects
   Reindex's per-point Upsert to appear as a `payload()`-routed write, it will be wrong (there is
   no `payload()` call anywhere inside `Reindex`, confirmed by the single-call-site grep above).
   The gate must name Reindex's write as an exception, exactly mirroring the existing
   `embedder_identity` exception comment (`store.go:3207-3210`), rather than trying to route it
   through `payload()` or asserting it "inherits" the stamp.

## Common Pitfalls

### Pitfall 1: Writing D-03's gate as "does this write call `payload()` with `m.SchemaVersion`
already correct" instead of "does this call site route through `payload()` at all"
**What goes wrong:** A shape-based check ("the payload map passed to Upsert has key
`schema_version`") can be satisfied by a hand-copied literal map that happens to include the key
without ever calling `payload()` — reintroducing exactly the class of defect memory `x6v6qxqd6f`
documents for Phase 1.
**Why it happens:** Shape checks are easier to write than identity checks and often pass the
obvious test cases.
**How to avoid:** Anchor on call-site identity (an AST match on "this `*qdrant.PointStruct`'s
`Payload` field value is the expression `qdrant.NewValueMap(payload(...))`"), per Pattern 2 above.
**Warning signs:** A gate that would still pass if `payload()` were deleted and replaced with a
hand-rolled map literal at the one legitimate call site.

### Pitfall 2: Assuming the D-13 negative test only needs to cover `Search`/`List`/`ListScheduled`
because those are the "obvious" recall paths
**What goes wrong:** `SearchDiscovery` builds its filter with an entirely separate code path (an
inline `must` slice, not `ownerScopeFilter`/`listFilter`), so a test that only exercises the shared
helpers would miss a `schema_version` condition injected directly into `SearchDiscovery`'s inline
list.
**Why it happens:** `SearchDiscovery` looks structurally similar to `Search` at a glance but does
not call `ownerScopeFilter`.
**How to avoid:** D-14's "assert the enumeration matches the derived set" already forces coverage
of all five entry points, provided the derivation actually walks the AST/source rather than a
hand-typed list of function names copied from this document.
**Warning signs:** A negative test that references only `ownerScopeFilter`/`listFilter` without a
separate assertion covering `SearchDiscovery`'s inline `must` construction.

### Pitfall 3: Believing Reindex "inherits" D-05's monotonic rule
**What goes wrong:** As shown in the D-07 Research Flag section above, Reindex's write bypasses
`payload()` entirely — there is no monotonic computation happening during Reindex at all. A plan
task that says "verify Reindex applies D-05's max(...) rule" is testing something that structurally
cannot happen given the current code, and will either be a vacuous/misleading assertion or force an
unplanned refactor of Reindex to route through `payload()` (out of this phase's stated scope, which
is additive-only: "the field on `store.Memory`; the stamping rule at the payload codec... the
negative recall-gate proof; the forward/backward compatibility proof" — Reindex refactoring is not
listed).
**Why it happens:** D-07's own stated reasoning ("It is a full rewrite through `payload()`... it
inherits D-05") is factually incorrect, as shown by reading the code.
**How to avoid:** Task the plan around the corrected finding: Reindex preserves `schema_version` by
verbatim raw-map copy, which needs zero new code and needs a corrected doc-comment/test-comment
explaining *why* (not because it "rebuilds from the decoded struct," but because it never rebuilds
anything — it copies the map).
**Warning signs:** A VERIFICATION.md line claiming "Reindex correctly applies the monotonic stamp"
— this cannot be independently true; the correct claim is "Reindex preserves `schema_version`
verbatim, consistent with its existing unknown-key preservation contract."

## Code Examples

### The pure-function round-trip test pattern to mirror (no Qdrant needed)

```go
// Source: internal/store/store_test.go:2924-2935, read verbatim this session — the exact shape
// TestPayloadRoundTripsSchemaVersion should follow. payload()/fromPayload() are pure functions;
// this test needs no testcontainer.
func TestPayloadRoundTripsShortID(t *testing.T) {
	m := Memory{ID: "a0000000-0000-0000-0000-000000000001", ShortID: "j7k2m9p4x0", Content: "c", Scope: "s"}
	got := fromPayload(m.ID, qdrant.NewValueMap(payload(m)))
	if got.ShortID != "j7k2m9p4x0" {
		t.Fatalf("round-trip short_id = %q", got.ShortID)
	}
	// Empty ShortID MUST be omitted (not stamped as an explicit ""), so legacy /
	// reindexed records stay key-absent and the NewIsEmpty backfill filter matches them.
	if _, ok := payload(Memory{ID: "x"})["short_id"]; ok {
		t.Fatal("empty ShortID must be omitted from payload")
	}
}
```
A `TestPayloadRoundTripsSchemaVersion` sibling should assert: (a) a non-zero `SchemaVersion` round
trips through `payload()`/`fromPayload()`; (b) a legacy payload missing the `schema_version` key
decodes to `Version(0)` with no panic (mirroring `TestPayloadRoundTripsEmbedderIdentity`'s legacy
case at `store_test.go:2949-2956`); and — the case unique to this field — (c) D-05's monotonic
rule: `payload(Memory{SchemaVersion: 5})["schema_version"]` stamps `5` (not
`migrate.CurrentVersion`) when `5 > migrate.CurrentVersion`, and stamps `migrate.CurrentVersion`
when the input is `0` or below current.

### The `fromPayload` tolerant-decode idiom D-08 relies on

```go
// Source: internal/store/store.go:617-728, read verbatim this session. Every field decode is
// `if v, ok := p["key"]; ok { ... }` — an unrecognized key in p is simply never read, and a
// present-but-newer-shaped key this binary doesn't have a case for is silently skipped. This
// is the existing behavior D-08's "decode without error, recall fully, get_memory returns it"
// assertions are actually testing — no new tolerant-decode mechanism needs to be built for
// schema_version's own field; SchemaVersion just needs the same `if v, ok := p["schema_version"];
// ok { ... }` shape as every sibling field.
if v, ok := p["access_count"]; ok {
	m.AccessCount = uint64(v.GetIntegerValue())
}
```

### The verbatim-copy Reindex write (the D-07 mechanism, corrected)

```go
// Source: internal/store/store.go:3213-3223, read verbatim this session.
if _, err = s.client.Upsert(ctx, &qdrant.UpsertPoints{
    CollectionName: opts.Target,
    Wait:           qdrant.PtrOf(true),
    Points: []*qdrant.PointStruct{{
        Id:      p.Id,
        Vectors: qdrant.NewVectors(vec...),
        Payload: p.Payload,   // raw source payload map — NOT qdrant.NewValueMap(payload(m))
    }},
}); err != nil {
    return res, fmt.Errorf("reindex: upsert point %s into %q: %w", p.Id.GetUuid(), opts.Target, err)
}
```

## State of the Art

Not applicable in the usual sense (no external library churn to track). The one relevant
"state-of-the-art vs. prior-milestone" comparison is internal: this milestone's schema-version
field deliberately diverges from the internally-established `sessionPayloadVersion` precedent in
`internal/webauth` (`resolver.go:59`, `reseal.go:63`), which hard-rejects on version mismatch.
`REQ-schema-version-wire-visible` names this divergence as deliberate; do not reintroduce a
reject-on-mismatch branch anywhere in this phase's code.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The exact symbol name/signature for `migrate.CurrentVersion`/`migrate.Version` is left to the planner's judgment, per CONTEXT.md's own "Claude's Discretion" note — no verified precedent fixes this name | Recommended Package Layout | Low — CONTEXT.md already marks this as open; any reasonable name choice is correctable before Phase 3/5/7 start referencing it, per D-04's own reversibility note |
| A2 | `migrate.CurrentVersion`'s concrete value (0 vs 1) is unresolved and explicitly deferred to plan-checker cross-phase reconciliation, per CONTEXT.md's Open Question | User Constraints → Open Question | Medium — if Phase 2 and Phase 3 disagree on what "current" means, Phase 3's sweep could either see zero backlog on day one (if Phase 2 ships 1) or a backlog it's not yet equipped to process (if Phase 2 ships 0 and Phase 3's registry assumes otherwise) |

**All other claims in this research were verified by reading the cited file:line in this session**
(store.go, reindex_test.go, summary.go, store_test.go, openaiurl.go, collectionprefix_conformance_test.go)
or by running a command in this session (`rg` grep counts, `go version`, directory listings). None
are `[ASSUMED]`.

## Open Questions

1. **`migrate.CurrentVersion`'s value (0 or 1) — CONTEXT.md's own open question, restated.**
   - What we know: `internal/migrate` does not exist yet (confirmed: `ls internal/migrate` finds
     nothing). Phase 3/4 will register `backfill-short-ids` as the v0→v1 step.
   - What's unclear: whether Phase 2 should ship `CurrentVersion = 0` (deferring the bump to
     Phase 3/4) or `1` (asserting the v1 shape already exists).
   - Recommendation: CONTEXT.md already flags this as a plan-checker-gate item, not a research
     item — research adds no new evidence beyond confirming there is no existing code to read for
     the answer. The planner should pick one and record it as an explicit, named plan decision
     (not silently default to whichever the executor happens to type first), since Phase 3's plan
     will need to match it exactly.

2. **Where does the D-13 filter-walker helper live — `internal/store`'s test scope, or an
   exported introspection aid?**
   - What we know: no existing helper of this shape exists in the repo (checked: no function
     matching a `*qdrant.Filter`-walking signature found via grep). `collectionprefix_conformance_test.go`
     establishes the precedent of source-level scans living in `internal/store`'s own test files.
   - What's unclear: whether the walker needs to be exported (e.g., for reuse by a future
     `internal/server` test) or can stay test-file-local.
   - Recommendation: CONTEXT.md marks this as Claude's Discretion. Given nothing outside
     `internal/store` currently constructs `*qdrant.Filter` values (confirmed: filter construction
     is 100% internal to this package), a test-file-local, unexported helper is sufficient and
     matches the existing precedent's scope discipline (add exported surface only when a second
     consumer actually needs it).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All of this phase's code | ✓ | go1.26.5 darwin/arm64 (go.mod requires 1.26.3) | — |
| Docker (for Qdrant testcontainer) | D-08's raw-injection tests, D-13's real-builder introspection tests, `TestReindexRoundtrip` regression check | ✓ | daemon responds to `docker info` | — |
| Qdrant testcontainer harness | Same as above | ✓ (existing `TestMain`, `internal/store/store_test.go:115`; Phase 1's shared-CI-Qdrant mitigation already in place per `.planning/STATE.md`) | — | — |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** none — everything this phase needs is already present and
already exercised by the existing test suite.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard library `testing` (no testify/assertion DSL — confirmed via `.planning/codebase/TESTING.md` and by reading `store_test.go`) |
| Config file | none — plain `go test`, package-local `TestMain` at `internal/store/store_test.go:115` provisions the shared Qdrant testcontainer |
| Quick run command | `go test ./internal/store/... ./internal/migrate/... -run TestPayloadRoundTripsSchemaVersion` (pure-function tests, no container needed) |
| Full suite command | `task test:go` (`go test ./...`, which includes the Qdrant-backed tests) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-schema-version-stamped | Every full-write path stamps `schema_version` | unit (structural AST) + unit (behavioral round-trip per method) | `go test ./internal/store/... -run TestEveryFullWriteRoutesThroughPayload` (D-03 gate name indicative) | ❌ Wave 0 |
| REQ-schema-version-stamped | `payload()`/`fromPayload()` round-trip, legacy-absent decodes to v0, monotonic max rule | unit (pure function, no Qdrant) | `go test ./internal/store/... -run TestPayloadRoundTripsSchemaVersion` | ❌ Wave 0 |
| REQ-schema-version-never-gates-recall | Negative proof: `schema_version` absent from every filter built by the 5 recall entry points | unit + integration (real Qdrant, fail-first per D-15) | `go test ./internal/store/... -run TestSchemaVersionNeverGatesRecall` (indicative name) | ❌ Wave 0 |
| REQ-schema-version-wire-visible | `full=true`/`get_memory` serialize `schema_version` | integration (via existing `internal/server` handler tests) | `go test ./internal/server/... -run TestGetMemory` / `TestSearchMemoryFull` (existing test names — verify against `go test -list` before citing in VALIDATION.md, per STATE.md's false-green trap) | ✅ likely existing tests to extend, not create |
| REQ-schema-version-forward-compatible | Raw payload injection at `CurrentVersion+1` and `CurrentVersion-1`, both directions fully recallable and never downgraded | integration (real Qdrant, raw `SetPayload`) | `go test ./internal/store/... -run TestSchemaVersionForwardBackwardCompat` (indicative name) | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** the pure-function quick-run command above.
- **Per wave merge:** `task test:go` (full `go test ./...`).
- **Phase gate:** full suite green before `/gsd-verify-work`, per this repo's standing convention
  (`.planning/codebase/TESTING.md`).

### Wave 0 Gaps
- [ ] `internal/migrate/migrate.go` + `migrate_test.go` — the leaf package does not exist yet.
- [ ] A new test file for the D-03 structural gate and D-13/D-14/D-15 negative recall-gate proof
  (e.g. `internal/store/schemaversion_gate_test.go`) — no existing file covers this.
- [ ] `TestPayloadRoundTripsSchemaVersion` in `internal/store/store_test.go` (new test function,
  existing file).
- [ ] D-08's forward/backward raw-injection test (new test function, likely in `store_test.go` or a
  new file — no existing analog to extend; the closest precedent,
  `TestSupersedesFromPayloadTolerantDecode` at `store_test.go:4837`, tests tolerant *decode* only,
  not the full round-trip-plus-recall-plus-get_memory assertion set D-08 requires).
- [ ] Per this repo's `-run` false-green trap (`.planning/STATE.md`, durable record `bsbsvn4hbc`):
  every `-run` pattern in this phase's VALIDATION.md must be re-resolved against a live
  `go test -list` before being recorded as passing, not written once at plan time and trusted.

## Security Domain

`security_enforcement` is not set to `false` in `.planning/config.json` (absent → enabled per the
default rule), so this section is included, scoped honestly to what actually applies.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | This phase touches no auth code path |
| V3 Session Management | No | Not touched |
| V4 Access Control | Yes — negatively | The entire point of D-13/D-14/D-16 is proving `schema_version` is **excluded** from every access-control-relevant filter (`ownerOrSharedCondition`, `ownerOnlyCondition`, the Cedar-compiled conditions) — this phase's security-relevant work is a non-regression proof, not a new access-control mechanism |
| V5 Input Validation | Marginal | `schema_version` is never client-writable (server-set only via the monotonic stamp) — there is no user-input surface to validate this phase; D-08's raw-injection tests write directly via the Qdrant client (test-only, bypassing the MCP/Connect input surface entirely by design) |
| V6 Cryptography | No | Not touched |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Recall-gate cardinality inversion (a new field's absence silently becomes an exclusion filter) | Denial of Service (legitimate records become invisible) / Information Disclosure inverse (data loss from the caller's perspective) | D-13's filter-introspection proof + D-14's set-equality completeness check + D-15's fail-first-in-a-real-package proof — this is the phase's own designed mitigation for its own designed risk, already fully specified in CONTEXT.md |
| Downgrade-on-rewrite (a record's declared version regresses on a later write) | Tampering (data integrity) | D-05's monotonic `max(...)` rule, proven by D-08's newer-than-binary raw injection test |

## Sources

### Primary (HIGH confidence — read verbatim this session)
- `internal/store/store.go` — full read of lines 180-330 (Memory struct), 360-620 (payload/
  fromPayload codec, ensureIndexes), 730-1525 (Upsert, all five recall entry points and their
  filter-building helpers), 1794-1872 (Update), 3040-3352 (Reindex, reindexTargetContents,
  tagsFromPayload, tagsEqual)
- `internal/store/reindex_test.go` — lines 1-260 (payloadKeysEqual, scrollPoints, seedSource,
  TestReindexRoundtrip)
- `internal/store/store_test.go` — lines 2924-2975 (TestPayloadRoundTrips* siblings)
- `internal/server/summary.go` — full file (shapeRecall, toRecallView, recallView)
- `internal/store/collectionprefix_conformance_test.go` — lines 1-90 (AST-scan precedent)
- `internal/openaiurl/openaiurl.go` — full file (leaf-package style precedent)
- `.planning/phases/02-record-schema-versioning-foundation/02-CONTEXT.md`
- `.planning/REQUIREMENTS.md`
- `.planning/STATE.md`
- `.planning/config.json`
- `.planning/codebase/TESTING.md` (partial read, lines 1-60)

### Secondary (MEDIUM confidence)
- None — no web/external documentation was needed for this phase; it is entirely in-repo Go code
  investigation.

### Tertiary (LOW confidence)
- None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no external libraries; stdlib-only leaf package pattern confirmed against
  an existing sibling package's actual source.
- Architecture: HIGH — every filter-builder and codec call site was read directly, not inferred.
- Pitfalls: HIGH — the two headline pitfalls (D-07's mechanism and the IsEmpty cardinality
  inversion) are both grounded in direct code reads plus the durable memory (`x6v6qxqd6f`) CONTEXT.md
  already cites.

**Research date:** 2026-08-13
**Valid until:** Until this phase's code lands (this research is a snapshot of pre-Phase-2 code;
once `internal/migrate` and the `payload()` stamp exist, re-verify against the shipped shape rather
than this document for any later phase's research).
