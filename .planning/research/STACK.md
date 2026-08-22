# Stack Research

**Domain:** Record schema versioning + versioned migration mechanism + typed operator
report rendering, in an already-shipped Go + Qdrant service under a hard zero-new-Go-dependency
constraint.
**Researched:** 2026-08-12
**Confidence:** HIGH (schema-version discriminator, Qdrant payload-versioning affordances,
typed-renderer field-set-by-construction) / MEDIUM (exact shape of the migration-step registry —
no canonical Go library exists for a dependency-store-agnostic, db-less migration ledger; this is
a designed pattern, not an imported one)

## Recommended Stack

**There is nothing to add.** All four capabilities land on stdlib or an already-vendored
module. The "stack decision" for this milestone is which existing seam each capability extends,
not which package to install.

### Core Technologies (existing, reused — not new)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go stdlib `encoding/json` | go 1.26.3 (toolchain) | payload marshal/unmarshal, `schema_version` discriminator, migration-report JSON | `store.Memory`'s `payload()`/`fromPayload()` pair (`internal/store/store.go`) already round-trips every optional field (`archived_at`, `supersedes`, `not_before`, …) through plain struct tags — a `SchemaVersion int` field is one more line in that same pair, not a new mechanism |
| `github.com/qdrant/go-client` | v1.18.3 (pinned) | payload read/write, `SetPayload` merge for migration writes | Already the only Qdrant client in the tree; a schema-version migration is exactly the `SetPayload` single-key-merge shape `Store.Supersede`'s back-stamp and `spine-review archive` already use — no new query surface needed |
| `github.com/spf13/cobra` | v1.10.2 (pinned) | `engram migrate` subcommand, ordered step registry, `--apply` wiring | `registerDestructive` (`cmd/engram/destructive.go`) is the existing structural choke point for exactly this shape of command (preview-by-default, `--apply` flips to write); `migrate-remap-owner` is the direct precedent to imitate, not diverge from |

### Supporting Patterns (no library — described because "what do Go projects that avoid
migration frameworks actually do" has a real, named answer)

| Pattern | Source of the pattern | When to Use |
|---------|------------------------|-------------|
| **Schema Versioning Pattern** (MongoDB-ecosystem term of art) | Document-store schema-evolution literature (MongoDB's own "Building with Patterns: The Schema Versioning Pattern"; also documented generically at bool.dev's "Data Versioning and Schema Evolution Patterns" and jsonic.io's JSON-schema-migration guide) | A `schema_version` field lives directly on the document/payload; absence = version 0 by convention, no backfill required to adopt. This is precisely what `sessionPayloadVersion` already does in `internal/webauth/session.go` and precisely what PROJECT.md specifies for the memory payload |
| **Eager (background-sweep) migration**, not lazy (migrate-on-read) | Same literature; engram already chose eager exclusively — every existing one-shot operator command (`backfill-short-ids`, `migrate-remap-owner`) is a background sweep, never a read-time upgrade | `engram migrate` should be a sweep-per-step command, not a read-path shim. A lazy/migrate-on-read variant would mean touching every recall code path (`internal/store` has 5+ read call sites) to special-case old-version payloads — far larger blast radius than one more operator command, and inconsistent with every migration engram has shipped to date |
| **Idempotent scan-and-repair as the resumability mechanism** — no separate migration ledger/state table | This repo's own prior art: `Store.BackfillShortIDs` (scans for `short_id` absent) and `Store.RemapOwner` (scans for `owner == X`) are both already idempotent, resumable-by-rerun sweeps with **no** external progress table — Qdrant IS the ledger, because the predicate ("is this record still at the old version") is itself the resume condition | Each migration step's `Apply` should be `SELECT WHERE schema_version < N` (a Qdrant scroll+filter, same shape `BackfillShortIDs`/`RemapOwner` already use) rather than introducing a `schema_migrations`-style tracking collection — Qdrant has no relational-migration-log affordance, and building one would be new infrastructure this milestone doesn't need |
| **Ordered step registry via a Go slice/const table, not a DAG or a file-discovery mechanism** | Go idiom for embedded, compile-order-checked sequences — the same shape `internal/surfaces/rules.go`'s rule registry and `internal/surfaces/toolclass.go`'s classification table already use in this codebase | `[]migrationStep{ {Version: 1, Name: "...", Apply: ...}, {Version: 2, ...} }` declared once in `cmd/engram/migrate.go` (or a small `internal/migrate` leaf package if the step logic wants to live next to `store`), with a table-driven test asserting `Version` fields are strictly increasing and start at 1 — mirrors how `golang-migrate`'s numbered-file convention encodes order, but without a filesystem or a second migrations directory to keep in sync with the binary |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `go vet` / `golangci-lint` (already wired via `task lint`) | Catches an unused migration step, an unreachable version gap | No new lint rules needed — a table-driven `TestMigrationStepsOrdered` (mirroring the pattern of `TestSurfaceConformance*`) is a normal Go test, not a new tool |
| `testcontainers-go` (already vendored, `modules/qdrant`) | Prove a migration step against real Qdrant, prove resumability under a forced mid-sweep failure | Same harness `TestDestructiveGatePreventsMutation` and `migrate-remap-owner`'s reconciliation-pass test already use — no new test infra |

## Installation

```bash
# Nothing to install. go.mod is unchanged by this milestone.
# Confirm before merge:
git diff go.mod go.sum   # MUST be empty for this milestone's phases
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|--------------------------|
| Hand-rolled ordered-step registry in Go (`[]migrationStep`) | `github.com/golang-migrate/migrate` or similar SQL-migration framework | Never here — golang-migrate is built around a relational `schema_migrations` table and `.sql`/file-based up/down pairs; Qdrant is not relational and this milestone forbids new deps. If engram ever adds a relational side-store, revisit then, not now |
| Eager background-sweep migration (`engram migrate`, a real operator command) | Lazy migrate-on-read (upgrade a payload the instant it's read, write it back opportunistically) | Only if engram's read paths were few and centralized. They are not (search/list/get span `internal/store` + both MCP and Connect lanes); lazy migration would smear version-handling logic across every read site instead of concentrating it in one sweep command. Revisit only if a future schema change is too expensive to sweep eagerly (e.g., requires re-embedding) |
| `schema_version` absent == v0 (no backfill to adopt) | Explicit `schema_version: 0` stamped on every existing record before shipping the discriminator | PROJECT.md already locks "absent = v0, no backfill to adopt" — matches `sessionPayloadVersion`'s precedent where a JSON zero-value (`V: 0`) on a pre-existing cookie is treated as legacy without a migration step of its own. Only reconsider if `schema_version` needs to be queryable/filterable in bulk before any migration ever runs (a Qdrant `IsEmpty` filter already covers that, same idiom as `archived_at`) |
| Single shared `[]Field`-style report type for the typed renderer | A separate hand-written `Text() string` method per report struct, kept in sync with its JSON tags by convention (today's shape) + `TestOperatorOutputParity` | The current shape is precisely what issue #481 flags as *detectable-by-test*, not *unrepresentable*. Keep it only if the typed-renderer redesign is deliberately deferred again — but PROJECT.md schedules it for this milestone specifically because 6 new proto-mirrored fields are about to flow through `renderOperator` |
| Ordered `[]Field` slice walked by both `Text()` and `MarshalJSON()` | Reflection over a single struct's fields + a custom struct tag (e.g. `text:"label"`) read at render time | Reflection-based tag-walking is a legitimate alternative (see `go-render`, github.com/jimeh/go-render) and would also make json/text agree by construction. Rejected here because: (1) it is a new dependency if pulled from a library, or a hand-rolled `reflect` walker if not — more moving parts than a slice literal; (2) this codebase's existing taste (see `internal/surfaces`'s compile-time "declared marker" pattern) favors explicit, non-reflective structural guarantees over runtime tag inspection; (3) reflection order over struct fields is guaranteed stable by the language spec, but a hand-rolled walker still needs its own tests to prove type-safety for `any`-typed values, buying little over a literal `[]Field{...}` |
| Ordered `[]Field` slice walked by both `Text()` and `MarshalJSON()` | `go:generate`-driven codegen (parse the struct via `go/ast`/`go/types`, emit `Text()` and a field-iterator at build time) | This is the more "fully proved at compile time" option and uses only stdlib compiler-frontend packages (`go/ast`, `go/parser`, `go/types` — no new dependency). Worth it only if the number of report types grows large enough that hand-writing `[]Field{...}` per command becomes the bottleneck; for the ~7 commands + `spine-review` leaves in scope now, a generator is more machinery than the problem needs. Revisit if a future milestone adds many more operator report types |
| A generic `Field[T any](key string, val T) Field` constructor (light use of generics, for call-site type inference only) | A fully generic `OperatorReport[T]` wrapper type (per issue #481's second suggestion) parameterizing the whole report over its JSON payload type | The codebase uses generics sparingly today (`internal/webauth`, `internal/server` connect-error mapping — no report/renderer generics anywhere). A generic constructor function is a small, optional ergonomics win; a generic wrapper type adds a type parameter to every report struct's declaration for no additional compile-time guarantee beyond what the shared `[]Field` walk already provides — the field-set-identity property comes from *one method walking one slice*, not from the type parameter |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|--------------|
| `golang-migrate`, `goose`, `atlas`, or any SQL/relational migration framework | All assume a relational schema-migrations table and a `.sql`/file-based up/down convention; none has a Qdrant driver, and adopting one would violate the zero-new-Go-dependency constraint for a capability the store layer's own idempotent-sweep pattern already covers | The ordered `[]migrationStep` registry pattern above, driven through `registerDestructive` |
| A new `schema_migrations`-equivalent Qdrant collection or a version ledger outside the payload itself | Adds new infrastructure (a second collection to provision, back up, and reason about) for a property the payload's own `schema_version < target` predicate already gives you for free, exactly as `BackfillShortIDs`/`RemapOwner` demonstrate | Scan-and-repair keyed on the payload's own `schema_version` field |
| Lazy (migrate-on-read) schema upgrades | Would require touching every read call site across `internal/store`, both the MCP and Connect lanes, and both `search`/`list`/`get` — a far larger blast radius than a single new operator command, and a departure from every migration engram has shipped so far (all eager sweeps) | Eager `engram migrate` sweep, mirroring `backfill-short-ids`/`migrate-remap-owner` |
| Reflection-based struct-tag walking (hand-rolled or via a small library) for the typed renderer | Either pulls in a new dependency or hand-rolls a `reflect` walker that needs its own correctness tests and diverges from this codebase's existing preference for compile-time-visible structure (`internal/surfaces`'s marker-type pattern) | A literal `[]Field{...}` slice shared by `Text()` and `MarshalJSON()` |
| A fully generic `OperatorReport[T]` wrapper parameterized over the JSON payload type | Adds a type parameter to every report declaration without adding to the actual guarantee (which comes from the shared field-slice walk, not from `T`); inconsistent with how sparingly this codebase already uses generics | A concrete `[]Field` (or a small `Field` constructor with light generics for ergonomics only) |

## Stack Patterns by Variant

**If a migration step needs to re-embed content (touches the vector, not just the payload):**
- Route it through the existing `internal/embed` seam the same way `engram reindex` already does, not through the new `engram migrate` mechanism.
- PROJECT.md is explicit that `reindex` (embedder-config-identity) stays a separate command; a schema-version bump that also requires re-embedding is a `reindex`-shaped problem layered on top of a `migrate`-shaped payload change, not a reason to merge the two commands.

**If a future schema version needs a multi-step upgrade chain (v1→v2→v3 in one run, for a record still at v1):**
- Compose the ordered step slice: iterate `migrationSteps` filtered to `Version > record's current version`, applying in ascending order — the "incremental migration" composition documented in the NoSQL schema-evolution literature (see Sources).
- This composition is a pure function over the registry slice; no new mechanism beyond what the v1-only case needs.

**If the typed renderer needs a field to appear in JSON but NOT in text (or vice versa) for a genuinely asymmetric report:**
- Do not special-case this in the shared `[]Field` type. If a real case appears, it is evidence the "json must never expose more than text" invariant (the thing #481 exists to protect) is being deliberately relaxed for one command — that needs its own decision record, not a silent escape hatch in the renderer.

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|------------------|-------|
| `github.com/qdrant/go-client` v1.18.3 | Qdrant server (pinned in CI per `requireQdrant` gate, v1.18.2) | Already the pinned pairing project-wide (see Phase 17's `requireQdrant` CI gate); no version change needed for payload-field additions — Qdrant's payload is schema-free JSON, so adding `schema_version` requires no server-side migration, index, or version bump at all |
| Go toolchain 1.26.3 | `go/ast`/`go/parser`/`go/types` (if a codegen alternative is ever adopted for the renderer) | All three are stdlib in 1.26.3; no separate install. Not needed for the recommended `[]Field` approach, noted only because it was evaluated as an alternative |

## Sources

- `internal/webauth/session.go` (this repo) — `sessionPayloadVersion` precedent: auto-inject on write (`Seal`), strict check on read (`Resolver.Resolve`), absent/zero-value treated as legacy. HIGH confidence (in-repo, read directly).
- `internal/store/store.go`, `internal/store/spine.go` (this repo) — `archived_at`/`supersedes`/`not_before` payload round-trip pattern; `BackfillShortIDs`/`RemapOwner` idempotent-sweep precedent. HIGH confidence (in-repo, read directly).
- `cmd/engram/destructive.go`, `cmd/engram/migrate.go`, `cmd/engram/backfill.go` (this repo) — `registerDestructive` choke point and the exact preview/apply/output shape a new `engram migrate` command should imitate. HIGH confidence (in-repo, read directly).
- GitHub issue #481 (`operator tier: typed per-result renderer to make json/text widening structurally unrepresentable`) — names both candidate designs (single report type with `Text()` + json tags, or generic `OperatorReport[T]`) and the property `TestOperatorOutputParity` currently only detects rather than prevents. HIGH confidence (in-repo issue, read directly via `gh issue view`).
- GitHub issue #482 (`Connect lane omits superseded_by, supersedes, not_before, not_after, archived_at from Memory`) — confirms the six-field additive proto pass this milestone performs. HIGH confidence (in-repo issue).
- [Qdrant Fundamentals FAQ](https://qdrant.tech/documentation/faq/qdrant-fundamentals/) — confirms Qdrant offers no built-in payload schema versioning; recommends a "user-managed payload counter" for application-level write tracking, i.e. payload versioning is purely an application concern. MEDIUM-HIGH confidence (official docs, fetched directly).
- [Qdrant: Migrate to a New Embedding Model](https://qdrant.tech/documentation/tutorials-operations/embedding-model-migration/) — confirms Qdrant's own migration tooling is scoped to vector/collection migration (re-embedding, collection copy), not payload-shape versioning; reinforces that `reindex` and `migrate` are correctly separate concerns. MEDIUM confidence (official docs, web search summary).
- Data Versioning and Schema Evolution Patterns overview (bool.dev) and JSON Schema Migration Strategy guide (jsonic.io) — corroborate the named "Schema Versioning Pattern" (version field on the document, absent = legacy) and the lazy-vs-eager migration dichotomy used above to justify eager sweeps. MEDIUM confidence (community/vendor blog summaries, not primary academic sources — used only to name an already-well-established pattern this repo already independently follows via `sessionPayloadVersion`).
- go-render (github.com/jimeh/go-render) — surfaced as the closest existing OSS approximation of a reflection/tag-based text+JSON dual renderer, used above only as the rejected alternative's concrete example. LOW-MEDIUM confidence (package existence confirmed via search; not independently vetted for production quality, and not recommended for adoption).

---
*Stack research for: engram v2026-08-12.01 (Record State & Schema Evolution)*
*Researched: 2026-08-12*
