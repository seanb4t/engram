# Phase 5: Connect Record-State Parity - Context

**Gathered:** 2026-08-15
**Status:** Ready for planning

<domain>
## Phase Boundary

The Connect wire's `Memory` message carries a record's full state — the same fields
`store.Memory` already exposes — wired through `memoryToProto`, with an exhaustive
field-mapping test as the proof rather than a green `buf breaking` run.

**In scope:** the additive proto pass on `engramv1.Memory`, the `memoryToProto` wiring, the
exhaustive parity/population test and its RED-proof fixture, and a boundary-second test for
the scheduling bounds on the read path.

**Out of scope:** console and CLI *surfacing* of the new state (2026-08-12.01 Phase 7), the
typed operator renderer refactor (2026-08-12.01 Phase 6), and any docs pass beyond the proto
field comments and the one repaired `store.Memory` comment named in D-04.

</domain>

<decisions>
## Implementation Decisions

### Wire Representation

- **D-01:** `schema_version` is a **plain `uint32`** (field 28) — no `optional`, no explicit
  presence. Zero *is* v0 *is* absent, which is the exact semantics Phase 2's D-09 locked when
  it rejected `*int` in Go for the same reason: absent and zero are *defined here* to be the
  same state. This keeps every downstream consumer (Phase 7's CLI and console) free of a nil
  branch. Accepted consequence: a client marshalling with default protojson options sees a v0
  record as having no `schema_version` key. `uint32` over `uint64` is deliberate — protojson
  renders `uint64` as a JSON *string* (see `cmd/engram/client_list_test.go:50`) and `uint32`
  as a number; `int32` was rejected as admitting negatives `migrate.Version` never produces.
  — **Reversibility:** one-way — the field number and type are a published wire contract; a
  later change to `optional` is a breaking change under `buf breaking` FILE mode.

- **D-02:** `superseded_by` is a **plain `string`** (field 23) — nil maps to `""`, `""`
  decodes to nil. Lossless in practice because `store.Memory.SupersededBy` only ever holds a
  real record id (server-set via `Store.Supersede`'s back-stamp); empty-string is not a value
  the store can produce. `optional string` was rejected: it costs a `*string` in Go and a nil
  branch in every consumer for a state that cannot occur, and it would split presence
  conventions across the eight new fields.
  — **Reversibility:** one-way — same published-wire-contract reasoning as D-01.

- **D-03:** Phase 2's D-10 guarantee — *"an operator asking what version is this record always
  gets one"* — is **anchored by a test**, not left emergent. A test asserts the CLI's
  `renderJSON` output (`cmd/engram/client_common.go:381`, `EmitDefaultValues: true`) for a v0
  record contains `"schema_version": 0`, so the coupling between D-10's promise and one
  renderer's marshal options is gated rather than incidental. The zero-means-v0 contract is
  also stated in the proto field comment.
  **protovalidate was considered and ruled out for this purpose**, on facts: it is a
  *request-side* runtime validator wired as a unary interceptor
  (`internal/server/connectvalidate.go`) — `Memory` is a response type and carries no
  `buf.validate` annotations today; nothing validates responses, and doing so would turn a
  schema drift into a `CodeInternal` rather than a caught bug. Separately,
  `(buf.validate.field).required` is defined in terms of field *presence* and therefore cannot
  be expressed on the presence-less `uint32` D-01 selects.
  — **Reversibility:** reversible.

### Scope of the Additive Pass

- **D-04:** The pass carries **eight fields, numbers 23–30**, not the six the roadmap names.
  The codebase scout found `store.Memory`'s json-visible set is 30 fields against the proto
  `Memory`'s 22 — beyond the named six, `summary_model` and `summary_egress_at` are also
  absent from the Connect lane while being present on the MCP lane (`shapeRecall` returns
  `store.Memory` verbatim on `full=true`). Both are added: they are already caller-visible on
  MCP, so this exposes nothing new, and it removes the lane asymmetry rather than codifying it.
  There is no do-nothing option — the exhaustive test of `REQ-connect-parity-roundtrip-proof`
  cannot be written without classifying them.

  **This REVERSES a prior decision, deliberately.** Durable record `zyaa3m2fvd` (2026-06-28,
  closing `engram-3nof`) held that `summary_model` stays store-only and off the proto/Connect
  wire, reasoning that only `summary`/`summary_source` are client-meaningful, that
  `summary_model` and `summary_egress_at` are store-internal diagnostics, and that adding one
  without the other would be asymmetric. That record closed with an explicit invitation:
  *"Reconsider BOTH together if a consumer ever needs them."* D-04 is that reconsideration,
  taken together as the record required. What changed: `REQ-connect-parity-roundtrip-proof`
  makes the classification of every `store.Memory` field mandatory rather than optional, and
  the consumer is the parity invariant itself — under D-05 an exclusion is no longer free, it
  costs a permanent exemption in a mechanism whose whole value is having none. `zyaa3m2fvd`
  is superseded by this decision, not merely overridden.

  **Field number assignment** (permanent; assigning the roadmap's named six to 23–28 in its
  own stated order keeps SC1's "field numbers 23–28" literally true):

  | # | Field | Proto type | Source |
  |---|-------|-----------|--------|
  | 23 | `superseded_by` | `string` | `SupersededBy *string` (D-02) |
  | 24 | `supersedes` | `repeated string` | `Supersedes []string` |
  | 25 | `not_before` | `google.protobuf.Timestamp` | `NotBefore *time.Time` |
  | 26 | `not_after` | `google.protobuf.Timestamp` | `NotAfter *time.Time` |
  | 27 | `archived_at` | `google.protobuf.Timestamp` | `ArchivedAt *time.Time` |
  | 28 | `schema_version` | `uint32` | `SchemaVersion migrate.Version` (D-01) |
  | 29 | `summary_model` | `string` | `SummaryModel string` |
  | 30 | `summary_egress_at` | `google.protobuf.Timestamp` | `SummaryEgressAt time.Time` |

  Timestamp fields inherit `memoryToProto`'s established nil/zero handling: leave the proto
  field **unset** rather than emitting a year-1 (`0001-01-01`) stamp — the discipline already
  applied to `LastAccessedAt` at `internal/server/connectapi.go:49-54`. `SummaryEgressAt` is a
  non-pointer `time.Time`, so its zero value is the "never egressed" case and maps to unset.

  **Correction this decision requires:** `store.Memory.SummaryEgressAt`'s comment
  (`internal/store/store.go:272-276`) claims *"Store-only; not on the Connect wire."* The
  second clause is true today and false after this phase; the first clause is already false —
  the field carries a plain `json:"summary_egress_at"` tag and reaches the MCP wire. Repair
  the comment to state what is actually true.

  **Roadmap edit required at plan time:** SC1 must widen from six fields / 23–28 to eight /
  23–30. Use `/gsd-phase edit` — never a hand edit to `ROADMAP.md` (rule `8dfdhfs5nn`).
  — **Reversibility:** one-way — eight published field numbers.

### Parity Test Construction

- **D-05:** The exhaustive test's inclusion rule is **derived from `json:"-"`**, not from a
  ledger. Every json-visible `store.Memory` field MUST have a proto counterpart; `json:"-"` is
  the *sole* exclusion mechanism, reusing a convention with two load-bearing precedents
  (`EmbedderIdentity` `internal/store/store.go:291`, `IdempotencyFingerprint` `:314`, both of
  whose comments already call the hidden tag "deliberate and load-bearing"). D-04 is what
  makes this possible: with the two extras added, the MCP-visible and Connect-visible sets
  become identical, so there is no exclusion list left to rubber-stamp. A future store-only
  field is excluded by the idiom the codebase already uses, not by remembering to update a
  table — the "invariant by construction, not by rule" pattern from durable record
  `ye5922wnvf`.

  **One documented exception, and it is a rename map, not an exemption map:** `Worktree` is
  `json:"worktree_path"` but proto `worktree` — the single name divergence across 30 fields.
  It must be a single explicit alias entry, and the test must fail loudly on an *unaliased*
  mismatch rather than falling back to a fuzzy name match.

  A `wire:"connect"`/`wire:"store-only"` struct-tag vocabulary and a default-deny map in
  `internal/server` were both rejected: the first adds a second tag system encoding what the
  json tag already encodes, and the second is a hand-maintained ledger remote from the fields —
  the shape that becomes a rubber stamp the first time someone adds an entry to turn a red
  test green.
  — **Reversibility:** reversible.

- **D-06:** The population fixture is built by **reflection auto-fill**: walk `store.Memory`
  and assign every field a type-appropriate, distinctive non-zero value, then assert no mapped
  proto field is zero after `memoryToProto`. This satisfies SC2's *populated*, not merely
  *present*, requirement, and a newly added field is covered the moment it exists — nobody has
  to remember to extend a fixture. A hand-built maximal `store.Memory{...}` literal was
  rejected precisely because a new field defaults to zero in it and the population assertion
  then passes vacuously: the by-construction-zero failure recorded in `m56eqp97fq`.
  — **Reversibility:** reversible.

- **D-07:** The detector's ability to fire is proven by a **permanent negative fixture** — a
  test-only struct carrying a deliberately unmapped field, asserted to be REJECTED by the
  *same* detector function the real test calls. This proves the filter's range is non-trivial
  on every CI run rather than once at authoring time, which is the general rule established by
  `k000pn14qp`: an anti-vacuity guard must prove the FILTER can match, never merely that the
  producer emitted rows. **Structural requirement:** the real path and the fixture path must
  route through one shared detector — two lookalike code paths would defeat the point.
  — **Reversibility:** reversible.

### Round-Trip and Rounding

- **D-08:** **No `protoToMemory` inverse is added.** The test decodes each proto value back to
  its Go form inline (`ts.AsTime()`, string, `uint32`) and compares against the source
  `store.Memory` field by field via reflection. That is genuinely a decode — it just is not a
  named production function. A real inverse was rejected because nothing in production
  consumes proto→`store.Memory` (the console, the CLI, and every Connect client consume proto
  directly), so it would be a second mapping call site that can drift from `memoryToProto`
  unobserved — the exact drift surface 2026-08-12.01 Phase 6 exists to eliminate on the
  renderer side.
  — **Reversibility:** reversible.

- **D-09:** **No read-path rounding code is added.** `formatWindowBound`
  (`internal/server/protoconv.go:166-176`) rounds outward on the *write* path specifically
  because the store then floors to whole seconds on encode/decode (`.Unix()`,
  `store.go:320/:323/:406/:410`); values coming *out* of the store are therefore already
  whole-second, and a read-path rounding call would be a branch that can never change an
  outcome — the constant-gate shape recorded in `k000pn14qp` — plus a second rounding call
  site that can drift from the write path's.

  What the phase builds instead: a **boundary-second test** that submits a sub-second bound
  through the Connect write lane, and asserts the outward-widened value comes back **identical
  on both read lanes** — MCP (`get_memory`/`full=true` recall) and Connect. SC3's "both lanes"
  is read as MCP and Connect. The proto field comments on `not_before`/`not_after` record that
  read-side rounding is a no-op by construction.

  **Roadmap edit required at plan time:** SC3's premise ("apply the same sub-second
  outward-rounding discipline" on the read path) does not describe what will ship. Correct it
  via `/gsd-phase edit` to state the property that actually holds.
  — **Reversibility:** reversible.

### Claude's Discretion

- Package placement of the exhaustive test (`internal/server` vs a package that can import
  both `internal/store` and `gen/go/engram/v1` without a cycle) — resolve by reading, not by
  guessing. `internal/server` already imports both, so it is the presumptive home.
- The exact reflection helper shape for D-06's auto-fill and D-08's per-field comparison
  (single walker vs two), and whether they share a field-descriptor pass.
- Whether `supersedes` needs any ordering assertion beyond field equality (store documents it
  as "ordered as the store received them").

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Durable memory records this phase reverses or relies on

- `zyaa3m2fvd` — the 2026-06-28 decision keeping `summary_model`/`summary_egress_at` off the
  Connect wire, **reversed by D-04** under the reconsider-both-together clause it stated.
- `s780vae1vr` — a `deprecated = true` proto field still occupies its number; reusing one is the
  single way an otherwise-additive change trips `buf breaking` in FILE mode.
- `fbgyfq6j6j` — `/gsd-discuss-phase` records scope expansions in CONTEXT.md but never
  propagates them to `ROADMAP.md`; a separate `/gsd-phase edit` is required and has been missed
  three consecutive phases. Both roadmap edits named in D-04 and D-09 are subject to this.
- `k000pn14qp` — an anti-vacuity guard must prove the FILTER can match, not merely that the
  producer emitted rows (drives D-07 and D-09).
- `m56eqp97fq` — a fixture that makes the asserted value zero by construction passes vacuously
  (drives D-06).
- `ye5922wnvf` — enforce an invariant by construction, not by written rule (drives D-05).

### Prior-phase decisions this phase is bound by

- `.planning/phases/02-record-schema-versioning-foundation/02-CONTEXT.md` §"Encoding & Wire
  Shape" — D-09 (`SchemaVersion migrate.Version`, `*int` rejected because absent and zero are
  the same state), D-10 (`json:"schema_version"` with no `omitempty`, rated *costly* precisely
  because "Phase 5 freezes this onto a proto field number"), D-11 (Connect exposure is Phase 5
  by design; console/CLI is Phase 7).
- `.planning/REQUIREMENTS.md` lines 43-44 — `REQ-connect-record-state-parity`,
  `REQ-connect-parity-roundtrip-proof`.
- `.planning/ROADMAP.md` §"Phase 5: Connect Record-State Parity" — SC1 and SC3 both require a
  `/gsd-phase edit` correction per D-04 and D-09.

### Wire schema and codegen

- `proto/engram/v1/engram.proto` — `message Memory` (fields 1-22) at line 13; the additive
  pass takes 23-30.
- `buf.yaml` — `breaking: use: [FILE]`; `deps: buf.build/bufbuild/protovalidate`.
- `CLAUDE.md` §Conventions "Protobuf/buf" — `gen/` is committed and CI-checks for drift via the
  `buf` job; regenerate with `task proto:gen`.

### Mapping and conversion code

- `internal/server/connectapi.go:48-78` — `memoryToProto` / `memoriesToProto`; lines 49-54 are
  the established nil-timestamp discipline D-04 inherits.
- `internal/server/protoconv.go:123-176` — `scheduleMemoryRequestToArgs`, `windowBoundFloor`
  /`windowBoundCeil`/`formatWindowBound`; the write-path outward-rounding rationale D-09 builds on.
- `internal/store/store.go:186-370` — the `Memory` struct, its json tags, the two `json:"-"`
  precedents (`:291`, `:314`), `SummaryEgressAt` (`:272-276`, comment to repair per D-04), and
  `SchemaVersion` (`:353`).
- `cmd/engram/client_common.go:369-391` — `renderJSON`'s `EmitDefaultValues`/`UseProtoNames`
  options; the coupling D-03 pins.

### Existing test precedent

- `internal/server/schemaversion_wire_test.go` — `TestSchemaVersionOnRecallWire` and
  `TestSchemaVersionOnGetMemoryWire`; the deliberate mirror-in-the-opposite-direction pattern
  and the raw-Qdrant legacy-seed helper (`dialRawQdrantClient`, `:175`).
- `internal/server/connectapi_write_parity_test.go` — `TestWriteParity` (`:172`), the closest
  existing analogue for a cross-lane parity assertion.
- `internal/server/connectvalidate.go` — the protovalidate interceptor; read before proposing
  any response-side validation (D-03 rules it out).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- `memoryToProto`'s `LastAccessedAt` handling (`connectapi.go:49-54`) is the ready-made
  template for all four new Timestamp fields — including `SummaryEgressAt`, whose non-pointer
  zero value plays the same role as a nil pointer.
- `formatWindowBound` (`protoconv.go:166`) already encodes the outward-rounding semantics; the
  boundary-second test drives it rather than reimplementing it.
- `dialRawQdrantClient` (`schemaversion_wire_test.go:175`) bypasses `payload()`'s codec
  entirely — available if the boundary-second or v0 test needs a hand-shaped record.
- `renderJSON` (`client_common.go:380`) makes the CLI's json output derive field names from the
  message itself, so the eight new fields reach `engram get`/`list` json output automatically
  once they exist on `Memory` — which is what makes D-03's test cheap here.

### Established Patterns

- **Two wire lanes, asymmetric risk.** MCP serializes `store.Memory` through json tags, so a
  new field lands there for free; Connect goes through hand-written `memoryToProto`, so it does
  not. That asymmetry is the mechanical cause of the gap recurring across v0.8.x, v0.11.x, and
  v0.13.x — adding a Go field and regenerating protobuf both stay green while the mapping is
  simply absent.
- **`json:"-"` as the store-only marker** — two precedents, both with comments calling the tag
  deliberate and load-bearing. D-05 promotes this from convention to enforced invariant.
- **`buf breaking` runs in FILE mode**, so field numbers 23-30 are a permanent commitment and a
  `deprecated = true` field still occupies its number (durable record `s780vae1vr`).
- **Anti-vacuity discipline** — this milestone has recorded three distinct vacuous-gate shapes
  (`x6v6qxqd6f`, `k000pn14qp`, `v3nm25sfkx`). D-06 and D-07 exist specifically to keep the
  parity test out of that family.

### Integration Points

- `proto/engram/v1/engram.proto` → `task proto:gen` → committed `gen/go/`, `gen/ts/`, and
  `ui/src/lib/gen/`. A proto edit dirties all three; the `buf` CI job checks for drift, and
  `task ui:build` is a required check no phase gate runs (see STATE.md "CI gates outside the
  phase lifecycle"). Expect to run both locally.
- `memoryToProto` is the single write point for the Connect read lane — `SearchMemories`,
  `ListMemories`, and `GetMemory` all funnel through `memoriesToProto`/`memoryToProto`, so the
  eight fields appear on every read RPC at once.
- Phase 7 consumes this: console and CLI surfacing of archived/superseded/scheduled/version
  state depends on these fields existing on the wire. Phase 6's typed renderer is independent
  but must land before Phase 7.

</code_context>

<specifics>
## Specific Ideas

- The user raised protovalidate and proto options/annotations as a possible enforcement
  mechanism. Ruled out for *emission* on the facts recorded in D-03 — but the underlying
  instinct (declarative annotation over hand-maintained list) is what D-05 adopts, relocated to
  the correct side: the classification must be anchored on the Go struct, because the failure
  mode is a `store.Memory` field with **no** proto counterpart, and a proto annotation cannot
  describe a field that is not in the proto. `store.Memory` is the superset; that is where the
  rule has to live.

- Two roadmap success criteria are factually wrong as written and must be corrected via
  `/gsd-phase edit` at plan time, not silently implemented around:
  - **SC1** — six fields / numbers 23-28 → eight fields / 23-30 (D-04).
  - **SC3** — the read path does not need the write path's rounding discipline; the honest
    property is that store's second-granular encoding makes it a no-op (D-09).

</specifics>

<deferred>
## Deferred Ideas

- **Console/CLI rendering of the eight new fields** — Phase 7 (`REQ-console-record-state`,
  `REQ-cli-record-state`). This phase stops at the wire plus D-03's single `renderJSON`
  assertion.
- **`schema_version` on the compact `recallView`** — Phase 2's D-11 deliberately left the
  hand-built compact shape untouched; adding it later is additive. Not this phase.
- **Response-side protovalidate** — ruled out here (D-03), and adopting it later would need its
  own decision about what a validation failure on egress should do.

### Reviewed Todos (not folded)

- **Research a versioned payload-migration mechanism**
  (`.planning/todos/pending/2026-08-10-research-versioned-payload-migration-mechanism.md`,
  score 0.6, matched on keywords "phase"/"store") — not folded. STATE.md records it as already
  consumed by this milestone's Phases 2-4 (schema versioning foundation, migration
  registry/sweep, migration CLI); the match is keyword noise, not live scope.

</deferred>

---

*Phase: 5-Connect Record-State Parity*
*Context gathered: 2026-08-15*
