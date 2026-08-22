# Phase 7: Console & CLI State Surfacing - Context

**Gathered:** 2026-08-20
**Status:** Ready for planning

<domain>
## Phase Boundary

A record's full state — archived, superseded, scheduled, and its schema version — plus the
collection's pending-migration state, is **reachable and legible** from the operator console and
the CLI.

Phase 5 already landed the wire half: all eight fields are on `engramv1.Memory` (23–30), mapped by
`memoryToProto`, and present in the console's vendored TypeScript types. Nothing reads them. Phase 6
landed the operator-tier view mechanism. This phase is the consumption pass — and, because recall is
hard-gated, it is **not** presentation-only.

**In scope:**

- An opt-in relaxation of the recall gate on `Search`/`List` (three orthogonal request flags),
  exposed on the Connect lane and the CLI client tier.
- A new Connect read RPC exposing the migration histogram `Store.MigrateStatus` already computes.
- Console rendering: a State section in the record detail pane, state markers on list rows, include
  toggles in the scopes sidebar, and a migration banner in the app shell.
- CLI: a new `engram get` verb rendering through Phase 6's view mechanism, a STATE column on the
  compact table, a client-tier migration-status verb, and an advisory footer on `search`/`list`.
- Closing the `renderOperatorView` headline sanitization hole (#505), which this phase is the first
  to reach.

**Out of scope:**

- The MCP tool schemas. `list_memory`/`search_memory`/`get_memory` are unchanged — agent recall stays
  zero-junk by default (D-03). The store options land once and both lanes call them, so exposing the
  flags on MCP later is a pure tool-schema addition.
- Porting `search`/`list` off `renderMemoryTable` onto the view mechanism (D-09).
- Any docs-site pass beyond what the new surfaces themselves require — `REQ-docs-record-state` is
  Phase 8.
- The `--scope`-or-`--all-scopes` conditional-rule registration (#480) — Phase 8.

</domain>

<decisions>
## Implementation Decisions

### Reaching Hidden Records

- **D-01:** The phase **relaxes the recall gate behind an explicit opt-in**, rather than rendering
  state only where a record is already reachable.

  This is the fork that sizes the phase. Superseded, archived, and not-yet-active records are
  hard-filtered from `Search` and `List` on every lane by `Must` conditions in
  `internal/store/store.go` (`IsEmpty("superseded_by")` and `IsEmpty("archived_at")` at four sites —
  1091/1097, 1191/1195, 1328/1333, 1563/1568 — plus `activeWindowConditions` at 1013/1017).
  `GetMemory` bypasses all of it. Without an opt-in, an **archived** record is unreachable in
  practice: archiving is orthogonal to supersession, so nothing links to an archived record and no
  live head points at it. Presentation-only and follow-the-link navigation were both considered and
  rejected for that reason — each leaves SC1's "cannot render the v0.13.x archive tier at all"
  substantially unaddressed. The milestone goal's word is *reachable*.
  — **Reversibility:** one-way — the request fields are a published wire contract; removing them
  later is breaking under `buf breaking` FILE mode.

- **D-02:** The opt-in is **three orthogonal plain bools** — `include_archived`,
  `include_superseded`, `include_scheduled` — false meaning today's behavior, on both
  `ListMemoriesRequest` and `SearchMemoriesRequest`.

  Each flag maps **1:1 onto one gate condition**, which is the property that makes correctness
  checkable by reading: a reviewer pairs each flag with its `IsEmpty` and is done. The store keeps
  these three conditions deliberately separate — the comments at `store.go:1093-1097` and
  `:1191-1195` explicitly refuse to fold archived into superseded — and the flag set mirrors that
  rather than re-collapsing it. Composable, so "archived AND superseded" is expressible.

  A `repeated RecordState` enum was rejected as introducing a flag→condition mapping table the 1:1
  form does not need; a single `include_hidden` bool was rejected because it cannot express "just the
  archive tier", which is precisely what SC1 names.
  — **Reversibility:** one-way — six published field numbers across two request messages.

- **D-03:** The flags are exposed on **Connect and the CLI only**. MCP tool schemas are unchanged.

  `Store.Search`/`Store.List` serve both lanes, so the store work is identical either way — this is a
  tool-schema exposure decision, not a store one. Agent recall stays zero-junk by default, which is
  the memory contract's stated design intent, and an agent that needs a superseded record already has
  `get_memory` (ungated by construction). Exposing the flags on MCP later is additive.
  — **Reversibility:** reversible — adding the fields to the MCP tool schemas later is additive.

- **D-04:** **Authorization stays orthogonal to state.** The three flags relax only the state gates;
  the owner/shared authz filter is untouched. A `shared` record that is superseded or archived
  therefore becomes readable by exactly the callers its live predecessor was.

  Coupling hidden-state to ownership would push authz logic into the recall filter — the same fold
  the store's own comments refuse for archived-vs-superseded — and would require stating a new rule
  ("shared but hidden is private") everywhere sharing is documented. Isolation is unchanged by this
  phase; only state visibility moves.
  — **Reversibility:** reversible — narrowing later is a behavior change, not a wire change.

### Migration State Transport

- **D-05:** Pending-migration state reaches the console through a **new Connect read RPC** returning
  the histogram `Store.MigrateStatus` (`internal/store/migrate_status.go:102`) already computes.

  The console consumes it through the same generated client, the same auth chain, and the same error
  envelope as every other call — no second transport with its own auth story and hand-written TS
  type, which is the parallel-surface shape Phase 6 spent itself eliminating one tier up. It also
  gives the CLI client tier a path to the number: `engram migrate status` is **operator-tier**
  (`StoreFromEnv`, direct Qdrant access), so someone running `engram search` against a remote server
  cannot run it at all. That is the actual gap SC3 names.

  Client-side derivation from `schema_version` on listed records was rejected: it measures only the
  current page of the current scope, and it cannot see the absent-key legacy bucket that
  `backlogFilter`'s `IsEmpty` arm exists to catch — a number wrong in a way that looks right.
  — **Reversibility:** one-way — a published RPC and its message types.

- **D-06:** The RPC is readable by **any authenticated caller**, returning the **whole-collection**
  histogram: version buckets plus totals, no owner filter.

  No record content, no scopes, no owners. What it discloses is aggregate collection size across
  owners, which is acceptable for a self-hosted operator console and is the number that actually
  answers the operator's question — "will running `engram migrate` do work?" is a whole-collection
  property. An owner-scoped histogram was rejected because a clean per-owner count can read zero
  while a large legacy backlog exists, so the console would report "nothing pending" when the sweep
  has plenty to do. Anonymous callers (auth disabled) fall in the same single bucket they already do.

  Note for planning: an admin-only gate is **not** a cheap alternative here.
  `internal/authz/entities.go:42-47` intentionally OMITS `roles` from the principal entity — not set
  to empty, omitted — so the `has`-guarded `tenant_isolate` policy stays vacuous and a later
  milestone can populate roles with no breaking schema change. Populating roles is reserved work.
  — **Reversibility:** costly — narrowing a published read RPC's audience later breaks callers.

- **D-07:** In the console, migration state renders as a **banner in `AppShell.svelte`, on every
  route, only when there is something to say** — silent at zero, so the healthy case carries no
  chrome.

  This mirrors the intent of the existing startup `slog.Warn` (`internal/server/tools.go:517-535`):
  a condition an operator should not have to go looking for. The server already distinguishes two
  conditions and the banner must keep them distinct: records **behind** the current version ("pending
  migrations exist; run `engram migrate`") as a normal advisory, and records **ahead** of it
  ("records at a future schema version exist — this binary may be too old") as a stronger warning,
  because it means the running binary cannot fully read its own collection.
  — **Reversibility:** reversible — local to the console.

- **D-08:** The CLI gets **both** a client-tier verb for the full histogram **and** a one-line
  advisory footer on `search`/`list` when a backlog exists.

  The verb makes the number actionable; the footer is what satisfies SC3's "not only by running a
  command directly", since the operator learns about the backlog without asking. The footer follows
  the established shape of `renderCoverageFooter` (`cmd/engram/client_common.go:310`), which already
  prints an advisory line after results for `searched_scopes`/`scopes_truncated`.

  **Accepted cost:** one extra RPC per `search`/`list` call to populate the footer. Piggybacking a
  pending-count field onto `SearchMemoriesResponse`/`ListMemoriesResponse` would make it free at
  runtime and was rejected — it couples an unrelated operator concern into two hot response messages
  permanently, and every future consumer inherits a field it did not ask for.
  — **Reversibility:** reversible — the footer is on the explicitly-unstable text lane (Phase 6
  D-03); the verb is additive.

### CLI Shape and Renderer

- **D-09:** **`engram get <id>` is added** over the existing `GetMemory` RPC, accepting a full UUID or
  a `short_id` exactly as the MCP tool does.

  SC2 names it and it does not exist — the client tier is `search`/`list`/`store`. Naming it was not
  an accident: `GetMemory` is the only ungated read path, so it is the one place a superseded or
  archived record is reachable by id with no flag at all. Small: one file following
  `cmd/engram/client_list.go`'s shape, reusing the shared client flags, timeout, and error envelope.
  — **Reversibility:** reversible — a new verb is additive.

- **D-10:** `engram get` renders **one record through `renderOperatorView`**; `search`/`list` keep
  `renderMemoryTable`.

  A field-per-line view is the right shape for one record and the wrong shape for N; a table is the
  reverse. Porting the whole client tier — Phase 6's deferred idea — would turn N results into N
  stacked field tables and discard the scannable columnar output that makes `engram list` useful,
  along with `truncateSummary`'s entire purpose. SC2's "through the typed renderer" becomes true
  where it means something.

  **The adapter is one line and is the reason this is cheap:** `viewFields(doc any)` calls
  `json.Marshal(doc)`, and `encoding/json` returns a `json.RawMessage` **verbatim**. Passing
  `json.RawMessage(protojsonBytes)` therefore reuses the whole view over a protobuf message while
  preserving protojson's Timestamp and `optional` semantics — which matters directly, because
  Phase 5's D-14 made `superseded_by`/`schema_version`/`summary_model` `optional` and protojson omits
  an unset `optional` field regardless of `EmitDefaultValues`.
  — **Reversibility:** reversible — local to `cmd/engram`.

- **D-11:** The **headline sanitization hole is closed structurally, in this phase.** Route
  `renderOperatorView`'s headline through `sanitizeViewValue` so the property holds by construction.

  Today the field path is fully sanitized (C0 + DEL → single space, threat T-06-03) and the headline
  bypasses it entirely — safe only because all 15 headline producers interpolate CLI flag values and
  counts, never stored record content. That invariant is written down nowhere and enforced by
  nothing (durable record `5dr8amcx1w`, Issue #505, whose own tag reads `phase-07-will-reach-it`).
  This phase is the first to render record-derived content through the view, so it **creates the
  exploit condition** — that makes the fix a precondition, not smuggled scope. It is distinct from
  Phase 6's deferred red-evidence-harness fix, which was unrelated test infrastructure this phase
  merely inherited.

  Note the "safe values" set is narrower than it looks: `scope` and `tags` are user-supplied and
  `owner` comes from the IdP, so a convention-based fix would be fragile even if adopted.
  — **Reversibility:** reversible — a one-line structural change.

- **D-12:** `renderMemoryTable` gains an **always-present STATE column**, blank for live records.

  *(Sean's choice over the recommended conditional-column option, which would have added the column
  only when an include flag was set.)* One stable table shape regardless of flags — easier to
  describe and to consume than a shape that varies with the request. It changes today's default
  output for every existing user, and that is **contractually free**: Phase 6 D-03 declared
  `--output text` explicitly not a stable interface, with `--output json` as the contract. The json
  lane is untouched either way.

  `schema_version` stays **out** of the table as operator noise — it is reachable through
  `engram get` and the json lane.
  — **Reversibility:** reversible — the text lane carries no stability contract.

### State Vocabulary and Placement

- **D-13:** **The wire is the vocabulary.** Both surfaces derive a record's state label from the same
  proto field being set: `archived_at` present → `archived`, `superseded_by` present → `superseded`,
  `not_before` in the future → `scheduled`. No shared string table and no codegen.

  The agreement SC2 claims — "the CLI and the console agree on what a record is" — is that both read
  the same field and neither can invent a state the wire cannot express. **Gate it with a test per
  surface asserting the derivation**, so a fourth state added later cannot land on one surface only.
  A generated shared string table was rejected as a fourth generated tree to maintain alongside
  `gen/go`, `gen/ts`, and `ui/src/lib/gen` for three short words; convention-plus-docs was rejected
  as the "invariant enforced by nothing" shape that produced #505 and this repo's recurring
  half-applied-N-site pattern (`42nsbchqkn`, `3qcpntpavm`).
  — **Reversibility:** reversible.

- **D-14:** `MemoryDetail.svelte` gets a **dedicated State section**, rendered only when the record
  carries non-default state, plus `schema_version` as an **always-present** operator field.

  The section renders what a chip cannot: `superseded_by` as a link to the successor, `supersedes` as
  links back to predecessors, `not_before`/`not_after` as real timestamps, `archived_at` as a date.
  A chip row would tell the operator a record was superseded but not by what — the question they ask
  next. A live record's detail pane is unchanged.

  `schema_version` is always present because Phase 2's D-10 promised that an operator asking what
  version a record is always gets one — and Phase 5's D-14 §3 makes that conditional on the mapper
  assigning it unconditionally, since protojson omits an unset `optional` field. The console is the
  consumer that makes that promise observable.
  — **Reversibility:** reversible.

- **D-15:** `MemoryRow.svelte` uses **an explicit state marker in the meta line, plus a dimmed row
  treatment** for archived and superseded rows.

  *(Sean's choice over the recommended marker-only option.)* The marker is the direct parallel of
  D-12's always-present STATE column — the same word in a labeled slot on both surfaces. The dimming
  separates the two groups at a glance in a mixed list.

  **Two constraints planning must honor.** The marker is what carries the state to a screen reader,
  so the dimming stays strictly decorative on top of it and is never the sole signal. And "archived
  AND superseded" needs a **defined** dimming rule — the marker is composable because the flags are
  orthogonal (D-02), and the dimming must not silently pick one.
  — **Reversibility:** reversible.

- **D-16:** The three include toggles live in **`ScopesSidebar.svelte`, beside the existing categories
  and visibility filters**.

  It is where every other filter already lives, so an operator finds it where they look, and it
  inherits the existing URL round-trip for free — `parseObserveParams`/`observeSearch` in
  `$lib/queries` already encode filter state into the URL, making a filtered view bookmarkable and
  shareable, and `listMemoriesKey` already keys the query cache off the filter params.
  — **Reversibility:** reversible.

### Claude's Discretion

- The exact names of the new RPC and its request/response messages, and the shape of the histogram
  bucket message (`Store.MigrateStatus`'s existing result type is the obvious source).
- The client-tier migration verb's name (`engram status`, `engram migration-status`, or a client-tier
  sibling of `engram migrate status`) — resolve against the existing verb naming and the toolclass
  registry rather than by preference.
- Field numbers for the six new request fields, and whether `cross_spine` composition with the three
  flags needs an explicit guard or is naturally orthogonal.
- Whether `engram get` accepts multiple ids, and whether it needs a `--full` flag or always returns
  full content (`GetMemory` is not recall-gated and the MCP analogue returns full content).
- How the footer's extra RPC is scheduled — sequential, concurrent with the main call, or best-effort
  with the footer omitted on failure. A failed footer lookup must never fail the command.
- Whether the banner polls, refetches on route change, or fetches once per session.
- The exact wording of the two banner conditions and the footer line.
- Whether the STATE column renders one composed value or a fixed-width flag set when a record carries
  two states at once.
- Migration/task ordering across plans, and whether the store work, the proto work, the CLI work, and
  the console work split by tier or by capability.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and requirements

- `.planning/ROADMAP.md` §"Phase 7: Console & CLI State Surfacing" — goal, the three success
  criteria, the dependency on Phases 5 and 6, and the `UI hint: yes` marker.
  **Two SC premises do not match the tree and were confirmed against source during this discussion:**
  SC2 names `engram get`, which does not exist (D-09 adds it); SC2 says "through the typed renderer",
  which Phase 6 explicitly scoped out of the client tier (D-10 adopts it for `get` only). If either
  still misdescribes what will ship at plan time, correct it with `/gsd-phase edit` — **never** a
  hand edit to `ROADMAP.md` (rule `8dfdhfs5nn`). Phase 5 required exactly two such edits (its D-04
  and D-09); this is the established handling.
- `.planning/REQUIREMENTS.md` lines 52-54 — `REQ-console-record-state`, `REQ-cli-record-state`,
  `REQ-migration-state-visible`, the three requirements mapped to this phase.

### Upstream phases this one consumes

- `.planning/phases/05-connect-record-state-parity/05-CONTEXT.md` — **D-14 is load-bearing here.**
  `superseded_by`/`schema_version`/`summary_model` are `optional`; protojson OMITS an unset
  `optional` field and `EmitDefaultValues` does not override that, so `schema_version` is present in
  rendered JSON only because `memoryToProto` assigns it unconditionally. `superseded_by` is the
  deliberate exception — a pointer copy, nil stays unset. D-04 carries the full field table (23–30).
- `.planning/phases/06-typed-operator-renderer/06-CONTEXT.md` — D-03 (text is not a stable interface,
  json is the contract — this is what makes D-12 free), D-04 (headline is hand-written prose above a
  complete table), D-05 (humanized top-level labels, raw inline keys for nested rows), D-08 (no text
  goldens), and the deferred idea that D-10 partially adopts.
- `.planning/phases/04-migration-cli-first-customer/04-CONTEXT.md` — D-08 (`MigrateStatus` computes
  the histogram server-side: facet counts plus an `IsEmpty(schema_version)` exact Count for the
  absent/legacy bucket) and the startup warning as the only automatic surface today.

### The recall gate (D-01/D-02 change this)

- `internal/store/store.go` — the four gate sites: 1091/1097, 1191/1195, 1328/1333, 1563/1568.
  `superseded_by` and `archived_at` are deliberately separate `Must` conditions; the comments
  explicitly warn against folding them. `activeWindowConditions` at 1013/1017 is the scheduling gate.
- `internal/store/schemaversion_recallgate_test.go` — **re-run this derivation, do not assume it
  survives.** It derives which store methods are reachable from `recallEntryPointSeeds` and proves
  `schema_version` never gates recall; the assertion at :300-301 pins the two `IsEmpty` conditions
  directly. A conditional relaxation changes what those entry points can build.
- `internal/store/migratebacklog.go` — `backlogFilter`'s doc comment claims it is "operator-tier only
  and never reachable from any recall entry point", naming that claim as a dependency of Phase 2's
  reachability proof. Verify the claim still holds after D-01.

### Wire and authz

- `proto/engram/v1/engram.proto` — `ListMemoriesRequest` (fields 1-12), `SearchMemoriesRequest`
  (fields 1-9), `Memory` (fields 23-30 from Phase 5), and the 11 existing RPCs at :293-304.
- `internal/server/connectapi.go` — `memoryToProto` and its established nil/zero Timestamp discipline
  (leave unset rather than emit a year-1 stamp, :49-54).
- `internal/authz/entities.go:42-47` — `roles` intentionally omitted from the principal entity, a
  forward-compat reservation. Relevant to D-06: an admin-only gate is reserved work, not a cheap
  alternative.
- `internal/authz/policies/` — `own_records.cedar`, `shared_read.cedar`, `tenant_isolate.cedar`,
  `defense_empty_owner.cedar`. D-04 leaves all four untouched.

### CLI

- `cmd/engram/client_common.go` — `renderJSON` (:380, `UseProtoNames` + `EmitDefaultValues`),
  `renderMemoryTable` (:396, and its existing conditional `withScore` column),
  `renderCoverageFooter` (:310, the footer precedent D-08 follows), `truncateSummary` (:428),
  `clientFromFlags`, and the `cliError`/exit-code plumbing every new verb inherits.
- `cmd/engram/client_list.go` — the shape `engram get` should follow (D-09).
- `cmd/engram/operator_view.go` — `viewFields` (:45, marshals via `json.Marshal`, which is why the
  `json.RawMessage` adapter in D-10 works), `sanitizeViewValue` (:223), `renderOperatorView` (:255,
  the headline path D-11 fixes), `humanizeKey` (:197).
- `cmd/engram/cmdwalk.go` — `operatorCommands()`; derive any command work-list from the live cobra
  tree, never a hand-written list (`3qcpntpavm`).
- `internal/store/migrate_status.go` — `Store.MigrateStatus` (:102) and `MigrateStatusResult` (:57),
  the source D-05's RPC exposes.
- `internal/server/tools.go:487-535` — the existing 10-second-bounded startup check and its two
  distinct `slog.Warn` conditions (pending vs future-version), which D-07's banner mirrors.

### Console

- `ui/src/lib/components/MemoryDetail.svelte` — the meta chip row (`by`/`src`/`vis`), the category
  Badge, and where D-14's State section lands.
- `ui/src/lib/components/MemoryRow.svelte` — the `isRule`/`isShared` derived-flag pattern D-15
  extends, and the meta line carrying category, `ScopeChip`, and relative time.
- `ui/src/lib/components/ScopesSidebar.svelte` — the existing categories and visibility filters
  D-16's toggles join.
- `ui/src/lib/components/AppShell.svelte` — where D-07's banner mounts.
- `ui/src/routes/observe/+page.svelte` — the query wiring: `parseObserveParams`, `observeSearch`,
  `listMemoriesKey`, `PAGE_LIMIT`, and the svelte-query options-function idiom (v6 runes form).
- `ui/src/lib/gen/engram/v1/engram_pb.ts` — the vendored generated types, **already carrying all
  eight Phase 5 fields** (`supersededBy` :153, `notBefore` :163, `archivedAt` :173, `schemaVersion`
  :182, `summaryModel` :190). `ui/src/lib/gen/engram_pb.ts` is a hand-authored re-export barrel, NOT
  generated — never overwrite it.
- `Taskfile.yaml:242-243` — the re-vendor step (`rm -rf` then `cp -R gen/ts/.`); any proto change must
  be re-vendored or the console builds against stale types.

### Project conventions

- `CLAUDE.md` §Conventions — cobra CLI, `internal/config` (koanf), `task` = lint + test, SPDX headers
  on in-scope Go files, `go tool buf` for codegen with `gen/` committed and CI-checked for drift.
- `.planning/codebase/CONVENTIONS.md`, `.planning/codebase/TESTING.md` — established patterns.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **`renderOperatorView` works on protobuf messages with a one-line adapter.** `viewFields(doc any)`
  calls `json.Marshal(doc)`, and `encoding/json` returns a `json.RawMessage` verbatim — so
  `json.RawMessage(protojsonBytes)` reuses the entire Phase 6 view while preserving protojson's
  Timestamp and `optional` semantics. Phase 6 assumed this port would be more expensive than it is.
- **`renderCoverageFooter`** (`client_common.go:310`) is the established shape for an advisory line
  printed after results — D-08's footer follows it rather than inventing a placement.
- **`renderMemoryTable` already varies its columns** via `withScore` (SCORE for search, not for
  list), so a conditional column is a precedented pattern here even though D-12 chose an
  unconditional one.
- **`Store.MigrateStatus` already exists and is already called at startup** — D-05's RPC is a new
  transport over an existing computation, not new counting logic.
- **`parseObserveParams`/`observeSearch`/`listMemoriesKey`** already round-trip filter state through
  the URL and key the query cache off it, so D-16's toggles inherit bookmarkability and cache
  invalidation for free.
- **The console's vendored TS types already carry all eight Phase 5 fields.** The console work is
  purely rendering; no regeneration is needed unless this phase adds proto fields (which D-02 and
  D-05 both do — so a re-vendor IS required).

### Established Patterns

- **The three recall gates are deliberately unfolded.** `store.go`'s own comments state that archived
  and superseded are separate conditions and must never be merged. D-02's flag set mirrors that
  structure exactly rather than re-collapsing it.
- **`optional` presence asymmetry from Phase 5 D-14.** `schema_version` and `summary_model` are
  assigned unconditionally; `superseded_by` is a pointer copy where nil stays unset. Any consumer
  deriving state from field presence must respect that split — `superseded_by` presence is
  meaningful, `schema_version` presence is guaranteed.
- **Derive command and report sets from the live cobra tree**, never a hand-written list
  (`3qcpntpavm`, `q608f2c42w`) — and prove any such gate goes RED by live mutation, both directions.
- **`--output json` is the contract; `--output text` is not** (Phase 6 D-03). Text goldens are
  deliberately absent (D-08); the identity property is what gets pinned.
- **Zero new Go dependencies** — held across three consecutive milestones. Nothing in these decisions
  requires one.

### Integration Points

- `Store.Search` / `Store.List` gain options; both MCP and Connect handlers call them, but only the
  Connect request messages and the CLI expose the flags (D-03).
- `memoryToProto` is unchanged by this phase — Phase 5 already did that mapping.
- The new RPC joins `EngramService`'s read set (currently 5 read + 6 write).
- `engram get` joins the client tier (`search`/`list`/`store`) and inherits `addClientFlags`,
  `clientFromFlags`, the timeout, the exit-code taxonomy, and `wrapRPCError`.
- Any new command needs a toolclass registry row and will be caught by the `--help` golden files
  pinned from the live cobra tree.
- `renderOperatorView`'s headline path is shared with all 15 operator reports — D-11's fix changes
  behavior for every one of them (sanitization is idempotent on their current inputs, so no visible
  change is expected, but the blast radius is real).

</code_context>

<specifics>
## Specific Ideas

- **The state derivation, stated once, for both surfaces (D-13):**

  ```
  archived_at present         → archived
  superseded_by present       → superseded
  not_before in the future    → scheduled
  ```

  Neither surface invents a state the wire cannot express. A test per surface asserts this
  derivation, so a fourth state cannot land on one surface only.

- **The two banner conditions must stay distinct (D-07)** — the server already separates them at
  `internal/server/tools.go:522` and `:534`. "Pending migrations exist; run `engram migrate`" is an
  advisory. "Records at a future schema version exist" means the running binary cannot fully read its
  own collection, and reads as a stronger warning.

- **`superseded_by` renders as a link, not a string (D-14).** The operator's next question after
  "this was superseded" is always "by what" — the detail pane should answer it in one click, and
  `supersedes` should link back the other way.

- **A failed footer lookup must never fail the command (Claude's discretion, but stated).** The
  advisory footer is strictly additive information about a query the operator did not ask about.

</specifics>

<deferred>
## Deferred Ideas

- **The three include flags on the MCP tool schemas** (`list_memory`, `search_memory`). D-03 keeps
  agent recall zero-junk by default. The store options land in this phase, so exposing them on MCP
  later is a pure tool-schema addition plus a docs-site tool-reference update.
- **Porting `search`/`list` off `renderMemoryTable` onto the view mechanism.** D-10 adopts the view
  for `get` only. Phase 6's original deferred idea, still open for the multi-record case — but the
  table's columnar scannability is a real property to trade away deliberately, not incidentally.
- **Piggybacking a pending-migration count onto the hot list/search responses.** Rejected in D-08 as
  a permanent coupling of an unrelated concern into two response messages. Revisit only if the
  footer's extra RPC proves measurably costly.
- **An admin/operator role for the migration-status RPC.** D-06 opens it to any authenticated caller.
  `roles` is a deliberate forward-compat reservation in `internal/authz/entities.go`; if a later
  milestone populates it, narrowing this RPC is one of the first candidates.
- **Hardening the red-evidence harness** so a patch that merely breaks compilation cannot count as
  RED (`internal/store/redevidence_harness_test.go:303-316`; durable record `366pjeht8e`). Inherited
  from Phase 6, deferred there for the same reason: it is test infrastructure this phase does not
  cause. Distinct from #505, which this phase DOES reach and therefore fixes (D-11).

### Reviewed Todos (not folded)

- **Research a versioned payload-migration mechanism**
  (`.planning/todos/pending/2026-08-10-research-versioned-payload-migration-mechanism.md`, score 0.6,
  matched on keywords "research/migration/phase/engram/migrate") — **not folded**, for the third
  consecutive phase. STATE.md records it as already consumed by this milestone's Phases 2-4 (schema
  versioning foundation, migration registry/sweep, migration CLI), all complete. The match is keyword
  noise. Phases 5 and 6 declined it on the same grounds.

</deferred>

---

*Phase: 7-Console & CLI State Surfacing*
*Context gathered: 2026-08-20*
