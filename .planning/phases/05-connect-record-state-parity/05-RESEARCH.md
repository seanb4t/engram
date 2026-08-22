# Phase 5: Connect Record-State Parity - Research

**Researched:** 2026-08-15
**Domain:** Go protobuf wire mapping (protobuf-go / protojson / buf), Go reflection-based exhaustiveness testing
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** `schema_version` is a plain `uint32` (field 28) — no `optional`, no explicit
  presence. Zero *is* v0 *is* absent. `uint32` over `uint64` is deliberate (protojson renders
  `uint64` as a JSON string, `uint32` as a number); `int32` rejected (admits negatives
  `migrate.Version` never produces). One-way: field number/type is a published wire contract.
- **D-02:** `superseded_by` is a plain `string` (field 23) — nil maps to `""`, `""` decodes to
  nil. `optional string` rejected. One-way.
- **D-03:** Phase 2's D-10 guarantee ("an operator asking what version is this record always
  gets one") is anchored by a test on the CLI's `renderJSON` output
  (`cmd/engram/client_common.go:381`, `EmitDefaultValues: true`) — a v0 record must render
  `"schema_version": 0`. protovalidate ruled out for this purpose: it is request-side only
  (`internal/server/connectvalidate.go`); `Memory` is a response type with no `buf.validate`
  annotations, and `(buf.validate.field).required` is defined in terms of presence, which the
  presence-less `uint32` D-01 selects cannot express. Reversible.
- **D-04:** The pass carries **eight fields, numbers 23-30**, not six. `store.Memory`'s
  json-visible set is 30 fields against proto `Memory`'s 22 (confirmed this session — see
  Phase Requirements below). `summary_model` and `summary_egress_at` are added alongside the
  roadmap's named six because they are already MCP-wire-visible (`full=true` recall returns
  `store.Memory` verbatim) and excluding them would codify a lane asymmetry rather than remove
  it. This REVERSES durable record `zyaa3m2fvd` under its own "reconsider both together"
  invitation — `REQ-connect-parity-roundtrip-proof` makes classification of every field
  mandatory, so an exclusion now costs a permanent exemption in a mechanism whose value is
  having none.

  Field number assignment (permanent):

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
  field **unset**, never emit a year-1 stamp — the discipline already applied to
  `LastAccessedAt` (`internal/server/connectapi.go:49-54`, confirmed this session). Correction
  required: `store.Memory.SummaryEgressAt`'s comment (`internal/store/store.go:272-276`) claims
  "Store-only; not on the Connect wire" — the second clause becomes false this phase; the first
  clause is already false today (plain `json:"summary_egress_at"` tag, reaches MCP wire).
  Roadmap edit required at plan time (SC1 six→eight fields, `/gsd-phase edit`, never a hand
  edit — rule `8dfdhfs5nn`). One-way.
- **D-05:** The exhaustive test's inclusion rule is **derived from `json:"-"`**, the sole
  exclusion mechanism — reusing the precedent at `EmbedderIdentity`/`IdempotencyFingerprint`
  (confirmed this session, `internal/store/store.go:291`, `:314`; both comments call the tag
  "deliberate and load-bearing"). One documented exception, a rename map not an exemption map:
  `Worktree` is `json:"worktree_path"` but proto field name is `worktree` (confirmed this
  session, `store.go:197` vs `engram.proto:19`) — the single name divergence across 30 fields.
  Must be a single explicit alias entry; the test must fail loudly on an *unaliased* mismatch,
  never fall back to fuzzy name matching. A `wire:"connect"`/`wire:"store-only"` tag vocabulary
  and a default-deny map in `internal/server` were both rejected (second tag system encoding
  what json already encodes; hand-maintained ledger remote from the fields). Reversible.
- **D-06:** The population fixture is built by **reflection auto-fill**: walk `store.Memory`,
  assign every field a type-appropriate, distinctive non-zero value, then assert no mapped
  proto field is zero after `memoryToProto`. A hand-built maximal `store.Memory{...}` literal
  is rejected — a new field defaults to zero in it and the assertion passes vacuously
  (`m56eqp97fq`). Reversible.
- **D-07:** The detector's ability to fire is proven by a **permanent negative fixture** — a
  test-only struct with a deliberately unmapped field, asserted REJECTED by the *same* detector
  function the real test calls (`k000pn14qp`: an anti-vacuity guard must prove the FILTER can
  match, never merely that the producer emitted rows). Structural requirement: the real path
  and the fixture path route through ONE shared detector function. Reversible.
- **D-08:** **No `protoToMemory` inverse is added.** The test decodes each proto value back to
  its Go form inline (`ts.AsTime()`, string, `uint32`) and compares against the source
  `store.Memory` field by field via reflection — a genuine decode, just not a named production
  function. Rejected: nothing in production consumes proto→`store.Memory` today. Reversible.
- **D-09:** **No read-path rounding code is added.** `formatWindowBound`
  (`internal/server/protoconv.go:166-176`, confirmed this session) rounds outward on the WRITE
  path only, because the store then floors to whole seconds on encode/decode (`.Unix()`,
  `store.go:320/:323/:406/:410`) — values coming OUT of the store are already whole-second, so
  a read-path rounding call is a constant gate (`k000pn14qp`). What the phase builds instead: a
  boundary-second test submitting a sub-second bound through the Connect write lane, asserting
  the outward-widened value comes back IDENTICAL on both read lanes (MCP `get_memory`/
  `full=true`, and Connect). Roadmap edit required at plan time (SC3 rewritten to the property
  that actually holds, `/gsd-phase edit`). Reversible.

### Claude's Discretion

- Package placement of the exhaustive test (`internal/server` vs. a package importing both
  `internal/store` and `gen/go/engram/v1` without a cycle) — **RESOLVED this session, see
  Architectural Responsibility Map / Pattern 1 below: `internal/server` is confirmed safe, no
  guessing required.**
- The exact reflection helper shape for D-06's auto-fill and D-08's per-field comparison
  (single walker vs. two), and whether they share a field-descriptor pass.
- Whether `supersedes` needs any ordering assertion beyond field equality (store documents it
  as "ordered as the store received them").

### Deferred Ideas (OUT OF SCOPE)

- Console/CLI rendering of the eight new fields — Phase 7 (`REQ-console-record-state`,
  `REQ-cli-record-state`). This phase stops at the wire plus D-03's single `renderJSON`
  assertion.
- `schema_version` on the compact `recallView` — Phase 2's D-11 deliberately left the
  hand-built compact shape untouched; adding it later is additive. Not this phase.
- Response-side protovalidate — ruled out here (D-03); adopting it later needs its own
  decision about what a validation failure on egress should do.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-connect-record-state-parity | proto's `Memory` carries the record's full supersession/scheduling/archival/version state, added in one additive pass and wired through `memoryToProto` | Field-number table (D-04) confirmed against live `engram.proto:13-42` (22 existing fields) and `store.go:186-354` (32 struct fields, 2 `json:"-"`, 30 json-visible) — see Summary below. `memoryToProto` at `connectapi.go:48-70` is the single funnel confirmed for every read RPC (see Architecture Patterns). |
| REQ-connect-parity-roundtrip-proof | Proof is an exhaustive field-mapping round-trip test, not `buf breaking` + compiling code | `buf breaking` FILE-mode semantics researched (see Common Pitfalls, Pitfall 1) — it operates purely on the `.proto` schema and has zero visibility into `memoryToProto`'s Go body, so it structurally cannot catch this gap. The `jsonschemaExposedFields` precedent at `internal/server/surfaces_test.go:36-50` is a directly reusable json-tag-walking idiom for D-05's detector (see Pattern 2). |
</phase_requirements>

## Summary

This phase is a mechanical, well-scoped proto/Go wiring exercise wrapped in a testing-design
problem. The wire schema part is simple and already fully decided by CONTEXT.md — eight
additive fields, numbers 23-30, mapped from `store.Memory` following the exact nil/zero
discipline `memoryToProto` already uses for `LastAccessedAt`. The hard part, and the actual
point of the phase, is building an exhaustiveness test that cannot silently degrade into the
same "green CI, missing mapping" trap that has recurred three times (v0.8.x, v0.11.x,
v0.13.x). Both the population fixture (D-06) and the detector itself (D-07) need
anti-vacuity design, because a naive version of either can pass while proving nothing.

Verified this session by reading the live files (not from memory): `store.Memory` has exactly
32 struct fields, of which 2 carry `json:"-"` (`EmbedderIdentity` at `store.go:291`,
`IdempotencyFingerprint` at `store.go:314`), leaving 30 json-visible fields. The current proto
`Memory` message has exactly 22 fields (`engram.proto:14-41`). 30 − 22 = 8, matching D-04's
table exactly, field-for-field. The single name divergence D-05 calls out is also confirmed:
`Worktree` carries `json:"worktree_path"` (`store.go:197`) but the existing proto field (added
in an earlier phase, already shipped) is named `worktree` (`engram.proto:19`).

The package-placement question CONTEXT.md left to discretion is resolved by directly reading
imports rather than guessing: `internal/server` already imports both `internal/store`
(`connectapi.go:22`, `schemaversion_wire_test.go:19`) and `gen/go/engram/v1`
(`connectapi.go:19-20`) today, and `internal/store` imports neither `internal/server` nor
`gen/go/engram/v1` (confirmed via `rg` over `store.go`'s import block and the full
`gen/go/engram/v1` package). There is no cycle risk; `internal/server` is not merely the
presumptive home, it is confirmed safe.

A directly reusable precedent for the D-05 detector already exists in this codebase:
`internal/server/surfaces_test.go:36-50`'s `jsonschemaExposedFields` walks a struct via
`reflect.VisibleFields`, reads each field's `json` tag, skips `""` and `"-"`, and returns the
wire-visible name set. This is structurally identical to what D-05 needs for `store.Memory` —
the planner should treat it as the idiom to mirror, not invent fresh.

**Primary recommendation:** Build the eight-field additive proto pass exactly as D-04's table
specifies, wire `memoryToProto` following the `LastAccessedAt` nil-timestamp template, then
build the exhaustive test in `internal/server` as a single reflection-based detector function
(mirroring `jsonschemaExposedFields`) exercised three ways: (1) against the real
`store.Memory`→proto field-name mapping via the `json:"-"`-derived inclusion rule plus the one
`worktree_path`→`worktree` alias, (2) against a reflection-auto-filled `store.Memory` fixture
whose every mapped proto field must be non-zero after `memoryToProto`, and (3) against a
permanent test-only struct with one deliberately-unmapped field, which the same detector must
reject on every run.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Wire schema definition (proto `Memory` message) | API / Backend (schema layer) | — | `proto/engram/v1/engram.proto` is the source of truth for the Connect wire contract; codegen propagates it to Go and TS clients. |
| Store→wire field mapping (`memoryToProto`) | API / Backend | — | `internal/server/connectapi.go` is the single hand-written translation point between the persistence-layer struct (`internal/store.Memory`) and the wire message; this phase's entire surface lives here. |
| Exhaustiveness/parity test | API / Backend (test infra) | — | Must live in a package importing both `internal/store` and `gen/go/engram/v1` without a cycle — confirmed `internal/server` satisfies this (see Summary). Not a Database/Storage concern: it tests a Go mapping function, not persistence. |
| Read-path bound identity (boundary-second test) | API / Backend | Database / Storage (read encoding) | `formatWindowBound` on the write path and the store's `.Unix()` floor on encode/decode (Database tier) jointly make read-side rounding unnecessary; the boundary-second test spans both MCP and Connect read lanes, both API-tier. |

## Package Legitimacy Audit

Not applicable — this phase adds no new external Go/JS dependencies. All work is on existing
proto/Go/generated-code surfaces (`proto/`, `gen/`, `internal/server`, `internal/store`) using
already-vendored `google.golang.org/protobuf v1.36.11`, `connectrpc.com/connect v1.20.0`, and
`buf.build/go/protovalidate v1.2.0` (versions confirmed via `go.mod`, this session).

## Architecture Patterns

### System Architecture Diagram

```
proto/engram/v1/engram.proto (Memory message, fields 23-30 added)
        │
        │  task proto:gen (buf generate, pinned remote plugins)
        ▼
gen/go/engram/v1/*.pb.go  ─────────────────┐  gen/ts/...  ui/src/lib/gen/...
  (committed, CI drift-checked)            │  (committed, CI drift-checked;
        │                                  │   task ui:build required check
        │                                  │   outside phase gate)
        ▼                                  │
internal/server/connectapi.go              │
  memoryToProto(store.Memory) *engramv1.Memory   <── THE single write point
        │            ▲                                  for the Connect read lane
        │            │
        │            │  internal/store.Memory (32 fields, 30 json-visible,
        │            │  2 json:"-" store-only audit stamps)
        │            │
        ├─ GetMemory ──────┐
        ├─ shapeProtoMemories (ListMemories, SearchMemories)
        │      │            │  all funnel through memoryToProto —
        │      └────────────┘  the eight new fields appear on every
        │                      read RPC at once
        ▼
   Connect client (console / gRPC-Web / any Connect caller)

Parallel MCP lane (json tags, not memoryToProto):
internal/store.Memory ──(encoding/json)──► shapeRecall / get_memory
  (new fields land here "for free" — this is the asymmetry that
   caused the v0.8.x/v0.11.x/v0.13.x recurrence: MCP lane always
   current, Connect lane silently stale until this phase's test
   makes drift impossible to ship unnoticed)

Exhaustiveness test (internal/server, new):
  reflect.VisibleFields(store.Memory) ──derive json-visible set──► detector
                                                                       │
  real memoryToProto mapping ───────────────────────────────────────►│ MUST cover every
                                                                       │ json-visible field
  reflection-auto-filled fixture ──memoryToProto──► decode inline ───►│ MUST be non-zero
                                                                       │ after round-trip
  permanent negative fixture (unmapped field) ─────────────────────► │ MUST be REJECTED
                                                                       │ by the SAME detector
```

### Recommended Project Structure

No new packages or directories. Changes land in:

```
proto/engram/v1/engram.proto          # Memory message +8 fields (23-30)
gen/go/engram/v1/                     # regenerated (task proto:gen)
gen/ts/                               # regenerated (task proto:gen)
ui/src/lib/gen/                       # regenerated (task proto:gen / task ui:build)
internal/server/connectapi.go         # memoryToProto wiring
internal/server/connectapi_parity_test.go   # NEW — exhaustive field-mapping test (suggested name)
internal/server/connectapi_boundary_second_test.go  # NEW — D-09's boundary-second test (suggested name)
internal/store/store.go               # SummaryEgressAt comment repair (D-04)
```

### Pattern 1: Confirmed-safe test package placement

**What:** `internal/server` already imports both `internal/store` and `gen/go/engram/v1`;
`internal/store` imports neither. No import-cycle risk exists for placing the exhaustive test
in `internal/server`.

**Verified this session (not guessed):**
```go
// internal/server/connectapi.go:19-22
engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
"github.com/seanb4t/engram/internal/auth"
"github.com/seanb4t/engram/internal/store"
```
```go
// internal/store/store.go:19-22 — no import of internal/server or gen/go anywhere in the package
"github.com/seanb4t/engram/internal/authz"
"github.com/seanb4t/engram/internal/migrate"
"github.com/seanb4t/engram/internal/shortid"
"github.com/seanb4t/engram/internal/telemetry"
```
A repo-wide `rg` for `seanb4t/engram/internal` inside `gen/go/engram/v1/*.go` returned zero
matches — generated code imports nothing internal, only `google.golang.org/protobuf` and
stdlib.

**When to use:** Place the new exhaustive test file directly in `internal/server` (same package
as `memoryToProto`, `schemaversion_wire_test.go`, `connectapi_write_parity_test.go`,
`surfaces_test.go`) — it needs no new package and inherits the existing test helper ecosystem
(`spyStore`, `newSpyDeps`, etc.) if any of those prove useful for fixture construction.

### Pattern 2: json-tag-derived exhaustiveness (reuse, don't reinvent)

**What:** `internal/server/surfaces_test.go` already contains a struct-walking helper that is
structurally identical to what D-05 needs.

**Source (verbatim, `internal/server/surfaces_test.go:36-50`):**
```go
// jsonschemaExposedFields returns the json tag name (before the comma) for
// every VISIBLE field of t that carries a json tag — the REAL, reflected
// field-name set this struct exposes on the wire, never a hand-maintained
// list. reflect.VisibleFields (not a shallow t.NumField() walk) is required
// so an anonymously-embedded struct's promoted fields ... are seen exactly
// as jsonschema.For[T]'s own schema generation sees them.
func jsonschemaExposedFields(t reflect.Type) []string {
	var out []string
	for _, f := range reflect.VisibleFields(t) {
		jsonTag := f.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		name := strings.SplitN(jsonTag, ",", 2)[0]
		if name == "" {
			continue
		}
		out = append(out, name)
	}
	return out
}
```

**When to use:** As the direct template for D-05's inclusion-rule walker over `store.Memory`.
`store.Memory` has no embedded/promoted fields today (confirmed by reading `store.go:186-354`
in full — a flat struct), so `reflect.VisibleFields` degenerates to a flat `t.NumField()` walk
in practice here, but using `VisibleFields` anyway keeps the helper correct if a future field
is added via embedding, and matches the established idiom.

**Rename-map extension needed:** D-05 requires ONE alias entry (`worktree_path` → `worktree`)
that the detector must consult explicitly, and it must FAIL LOUDLY (not silently fuzzy-match)
on any other name mismatch. A minimal shape:
```go
// storeToProtoFieldAlias documents the SOLE name divergence between
// store.Memory's json tags and engramv1.Memory's proto field names.
// Any store.Memory json-visible field name not found verbatim among
// engramv1.Memory's proto field names, and not present as a key here,
// is a parity failure — this map is a rename record, not an exemption list.
var storeToProtoFieldAlias = map[string]string{
	"worktree_path": "worktree",
}
```

### Pattern 3: Nil/zero Timestamp discipline (extend, don't invent)

**What:** `memoryToProto` already has one working example of the "nil pointer → unset proto
Timestamp, never a year-1 stamp" discipline that all four new Timestamp fields must follow.

**Source (verbatim, `internal/server/connectapi.go:48-70`):**
```go
func memoryToProto(m store.Memory) *engramv1.Memory {
	// LastAccessedAt is nil for never-accessed records; leave the proto field
	// unset rather than emitting a year-1 (0001-01-01) Timestamp.
	var lastAccessed *timestamppb.Timestamp
	if m.LastAccessedAt != nil {
		lastAccessed = timestamppb.New(*m.LastAccessedAt)
	}
	return &engramv1.Memory{
		Id: m.ID, Content: m.Content, Scope: m.Scope,
		Repo: m.Repo, Workspace: m.Workspace, Worktree: m.Worktree, BaseDir: m.BaseDir,
		Source: m.Source, Category: m.Category, Tags: m.Tags,
		Actor: m.Actor, Owner: m.Owner, Visibility: m.Visibility,
		CreatedAt:      timestamppb.New(m.CreatedAt),
		Summary:        m.Summary,
		SummarySource:  string(m.SummarySource),
		Score:          m.Score,
		ShortId:        m.ShortID,
		AccessCount:    m.AccessCount,
		LastAccessedAt: lastAccessed,
		Kind:           m.Kind,
		Citations:      citationsToProto(m.Citations),
	}
}
```

**When to use:** `NotBefore *time.Time`, `NotAfter *time.Time`, and `ArchivedAt *time.Time` are
all `*time.Time` on the store struct — apply the exact same nil-guard-then-`timestamppb.New`
pattern as `lastAccessed`. `SummaryEgressAt time.Time` is a **non-pointer**; its zero value
(`time.Time{}`) is the "never egressed" case per D-04's note, so the guard is `if
!m.SummaryEgressAt.IsZero()` rather than a nil check. `SupersededBy *string` follows D-02: `if
m.SupersededBy != nil { out.SupersededBy = *m.SupersededBy }` (else `""`, the string zero
value, needs no explicit assignment). `Supersedes []string` and `SchemaVersion migrate.Version`
map directly (`Supersedes: m.Supersedes`, `SchemaVersion: uint32(m.SchemaVersion)`).
`SummaryModel string` maps directly (`SummaryModel: m.SummaryModel`).

### Anti-Patterns to Avoid

- **Testing the shaper instead of `memoryToProto` directly:** `shapeProtoMemories`
  (`connectapi.go:115-129`, confirmed this session) deliberately CLEARS `Content`, `Citations`,
  and `Kind`, and rewrites `Summary`, when `full=false`. If the exhaustive population test
  drives its assertions through `ListMemories`/`SearchMemories` (which call
  `shapeProtoMemories`) instead of `memoryToProto` directly, the intentional field-clearing
  will masquerade as a parity failure (or worse, a false-negative — a genuinely-missing mapping
  could get overwritten to look absent by the shaper for unrelated reasons). Test
  `memoryToProto` in isolation.
- **A hand-built maximal fixture literal (explicitly rejected by D-06):** any
  `store.Memory{ID: "x", Content: "y", ...}` literal with every field spelled out by hand
  defaults a newly-added field to its Go zero value, and the "populated, not merely present"
  assertion then passes vacuously — this is the exact `m56eqp97fq` failure mode named in
  CONTEXT.md.
- **A detector with no adversarial run (the D-07 gap):** a detector function that has only ever
  been exercised against real, fully-mapped structs has never actually been PROVEN capable of
  rejecting anything. Per `k000pn14qp`, a permanent negative fixture must run through the same
  function on every CI invocation, not once at authoring time.
- **Fuzzy/substring name matching in the parity detector:** D-05 explicitly forbids this. A
  fuzzy matcher could accidentally pair an unrelated field by partial string overlap and hide a
  real gap. Exact-match plus the single explicit alias map only.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Struct-tag-derived field enumeration | A hand-maintained list of `store.Memory` field names | `reflect.VisibleFields` + `json` tag read (Pattern 2) | Already proven correct in this repo at `surfaces_test.go`; a hand list is exactly the "ledger to rubber-stamp" shape D-05 rejects. |
| Proto→Go inverse mapping function | A `protoToMemory` production function | Inline per-field decode inside the test only (D-08) | Nothing in production needs the inverse; adding one creates a second mapping call site that can silently drift from `memoryToProto`. |
| Response-side field validation | protovalidate CEL rules on `Memory` | The exhaustive Go test | `Memory` is a response type; protovalidate here is request-side only (`connectvalidate.go`) and validating egress would turn a schema-drift bug into a `CodeInternal` at runtime instead of a caught test failure (D-03). |
| Read-path timestamp rounding | A second `formatWindowBound`-style call on the read path | Nothing — rely on the store's `.Unix()` floor already making it a no-op (D-09) | Would be a constant gate (branch that can never change an outcome) plus a second call site that can drift from the write path's. |

**Key insight:** every "don't hand-roll" item in this phase is a specific rejection CONTEXT.md
already recorded with a named reason (D-03, D-08, D-09) — the risk here is not inventing a new
custom solution, it is *reintroducing* one of these three during planning under a different
name (e.g., calling the inline decode helper a "mapper" and promoting it to a package function
would silently violate D-08).

## Common Pitfalls

### Pitfall 1: Mistaking `buf breaking` for evidence the mapping exists

**What goes wrong:** `task proto:gen` runs clean, `buf breaking` (FILE mode, `buf.yaml`
confirmed this session: `breaking: use: [FILE]`) reports no breaking changes, `go build`
succeeds — and the team ships a phase where two, four, or eight of the new fields are never
actually populated by `memoryToProto`.

**Why it happens:** `buf breaking` operates purely on the `.proto` schema files. Per the
official docs (confirmed via context7, `bufbuild/buf`), FILE is "the default and strictest
policy, catches anything breaking wire or source compatibility at the file level" — it compares
one proto schema against another. It has zero visibility into `internal/server/connectapi.go`,
the Go file where the actual store→wire mapping lives. A field can be fully declared on the
wire, generate correctly, and simply never be assigned in `memoryToProto` — `buf breaking`
cannot see that gap because it never inspects Go source. This is precisely why the gap has
recurred three times (v0.8.x, v0.11.x, v0.13.x): each time, someone treated a green `buf
breaking` + a green `go build` as sufficient proof.

**How to avoid:** The exhaustive field-mapping test (SC2/D-05/D-06/D-07) is the only mechanism
that inspects the actual Go mapping logic. `buf breaking` remains valuable for what it DOES
check (field-number reuse, type changes) but must never be cited as evidence for parity.

**Warning signs:** A plan or VALIDATION.md step that lists "`buf breaking` passes" as evidence
for `REQ-connect-parity-roundtrip-proof` — this is the exact failure the phase goal names by
name ("not a green `buf breaking` run mistaken for evidence a fourth time").

### Pitfall 2: Reusing a field number `deprecated = true` still occupies

**What goes wrong:** A future or accidental reuse of one of fields 23-30 (or any prior number)
trips `buf breaking` in FILE mode even though the change looks purely additive.

**Why it happens:** Per durable record `s780vae1vr` (cited in CONTEXT.md canonical refs): a
`deprecated = true` field still OCCUPIES its number. `ListMemoriesResponse.approximate` at
field 3 is exactly this case in the live proto (`engram.proto:81`,
`bool approximate = 3 [deprecated = true];`) — confirmed this session. Field numbers 23-30 are
fresh and unused today (proto tops out at field 22 in `Memory`), so this phase itself is clean,
but the planner should be aware the pattern exists elsewhere in this same file as a live
example of the constraint.

**How to avoid:** Assign exactly the eight numbers D-04's table specifies (23-30) and nothing
else; do not renumber or "clean up" any existing field while touching this file.

**Warning signs:** Any diff to `engram.proto` touching a field number outside 23-30, or removing
a `deprecated = true` annotation.

### Pitfall 3: `EmitDefaultValues` does not make nil Timestamp fields visible

**What goes wrong:** Assuming the CLI's `renderJSON` (`EmitDefaultValues: true`,
`client_common.go:381-391`, confirmed this session) will render `"not_before": null` or similar
for an unset scheduling bound, and writing a test that expects a Timestamp key to always be
present.

**Why it happens:** Per protobuf-go's own docs (confirmed via context7,
`/protocolbuffers/protobuf-go`): `EmitDefaultValues` includes "non-optional scalar fields and
empty repeated or map fields" — it does NOT force emission of an unset singular *message* field
like `google.protobuf.Timestamp`. Only `EmitUnpopulated` (a different, stronger option this
project does not use in `renderJSON`) also emits message-typed fields (as `null`). So under this
project's actual `renderJSON` settings: `schema_version` (uint32) renders `0` when zero,
`summary_model` (string) renders `""` when empty, `supersedes` (repeated string) renders `[]`
when empty — but `not_before`/`not_after`/`archived_at`/`summary_egress_at` (all
`google.protobuf.Timestamp`) stay ABSENT from the JSON entirely when unset, never `null`.

**How to avoid:** D-03's `renderJSON` test should assert `"schema_version": 0` is present for a
v0 record (this is what D-03 actually specifies and it is correct). Do NOT extend that
assertion pattern to expect visible keys for the four Timestamp fields when unset — that
would be testing for behavior `EmitDefaultValues` does not provide and this project has not
opted into (`EmitUnpopulated` is not set anywhere in `renderJSON`).

**Warning signs:** A test asserting `"not_before"` (or similar) is present-but-null in
`renderJSON` output for an unscheduled record.

### Pitfall 4: Testing through `shapeProtoMemories` instead of `memoryToProto`

See "Anti-Patterns to Avoid" above — restated here as a pitfall because it is easy to reach for
the handler-level test helpers already present in `connectapi_write_parity_test.go` (spy
deps, `newSpyDeps`) which exercise full handlers, when the exhaustive test needs to call
`memoryToProto` directly to avoid the `full=false` field-clearing behavior contaminating the
population assertion.

## Code Examples

Verified patterns from this repo (all read directly this session, not from training memory):

### Existing nil/zero Timestamp discipline to extend
```go
// Source: internal/server/connectapi.go:48-70 (memoryToProto, existing)
var lastAccessed *timestamppb.Timestamp
if m.LastAccessedAt != nil {
	lastAccessed = timestamppb.New(*m.LastAccessedAt)
}
```

### Existing json-tag exhaustiveness walker to mirror for D-05
```go
// Source: internal/server/surfaces_test.go:36-50 (jsonschemaExposedFields, existing)
func jsonschemaExposedFields(t reflect.Type) []string {
	var out []string
	for _, f := range reflect.VisibleFields(t) {
		jsonTag := f.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		name := strings.SplitN(jsonTag, ",", 2)[0]
		if name == "" {
			continue
		}
		out = append(out, name)
	}
	return out
}
```

### protojson MarshalOptions semantics (protobuf-go docs, confirmed via context7)
```go
// Source: github.com/protocolbuffers/protobuf-go/blob/master/_autodocs/types.md
type MarshalOptions struct {
	Multiline         bool
	Indent            string
	AllowPartial      bool
	UseProtoNames     bool
	UseEnumNumbers    bool
	EmitUnpopulated   bool // also emits unset MESSAGE fields (as null)
	EmitDefaultValues bool // emits zero SCALAR fields + empty repeated/map; message fields untouched
	Resolver          interface{ /* ... */ }
}
```
This project's `renderJSON` (`client_common.go:381-391`) sets `EmitDefaultValues: true` only —
see Pitfall 3 for the consequence on the four new Timestamp fields.

### buf.yaml breaking-change configuration (confirmed live in repo)
```yaml
# Source: buf.yaml (this repo, read this session)
version: v2
modules:
  - path: proto
deps:
  - buf.build/bufbuild/protovalidate
lint:
  use:
    - STANDARD
breaking:
  use:
    - FILE
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Connect wire mirrors `store.Memory` incidentally (whatever fields someone remembered to add) | Connect wire mirrors `store.Memory` by enforced invariant (`json:"-"` as sole exclusion, exhaustive test) | This phase | Closes the gap class that recurred at v0.8.x, v0.11.x, v0.13.x; a future `store.Memory` field addition without a `memoryToProto` mapping now fails a test instead of shipping silently. |
| `summary_model`/`summary_egress_at` deliberately excluded from Connect (durable record `zyaa3m2fvd`, 2026-06-28) | Both included (D-04) | This phase, reversing `zyaa3m2fvd` under its own stated reconsider-both-together clause | Removes the MCP/Connect lane asymmetry for these two fields entirely; both are already MCP-visible so nothing new is exposed. |

**Deprecated/outdated:** None specific to this phase — no library version bumps, no upstream
API changes. `google.golang.org/protobuf v1.36.11` and `connectrpc.com/connect v1.20.0` are
already the versions in `go.mod` (confirmed this session) and remain current.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `internal/server/connectapi_parity_test.go` and `internal/server/connectapi_boundary_second_test.go` are reasonable/idiomatic file names for the two new test files | Recommended Project Structure | Low — cosmetic; planner/executor can choose different names freely, this is a suggestion not a constraint. |

No other claims in this research are tagged `[ASSUMED]`: every factual claim about the wire
schema, the store struct, the mapping function, the test precedents, `buf.yaml`, `go.mod`
versions, and protojson semantics was verified this session either by reading the live file
(with line anchors) or via context7 official documentation (protobuf-go, buf.build).

## Open Questions

1. **Exact struct-tag walker sharing between D-06 (auto-fill) and D-08 (comparison)**
   - What we know: CONTEXT.md leaves "single walker vs. two" to discretion; both need to visit
     every `store.Memory` field via reflection.
   - What's unclear: whether a single generic walker function (field name → set-a-value /
     compare-a-value, parameterized by an operation) is cleaner than two separate functions.
   - Recommendation: the planner should let the phase's task breakdown decide this at
     implementation time — it is purely an internal test-code organization choice with no
     external consequence, exactly as CONTEXT.md's discretion note frames it.

2. **`supersedes` ordering assertion**
   - What we know: `store.Memory.Supersedes` is documented as "ordered as the store received
     them" (`store.go:218-225`, confirmed this session).
   - What's unclear: whether the exhaustive test needs anything beyond `reflect.DeepEqual`-style
     slice comparison (which is already order-sensitive) to satisfy this.
   - Recommendation: plain ordered slice comparison already covers it — `reflect.DeepEqual` (or
     `slices.Equal`) on `[]string` is order-sensitive by default, so no extra assertion is
     needed unless the planner wants an explicit test case proving order survives the round
     trip (cheap to add, not required by any decision).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `buf` CLI | `task proto:gen` / `task proto:lint` / breaking-change check | ✓ | 1.72.0 | — |
| `task` (go-task) | All phase build/test/lint commands | ✓ | 3.52.0 | — |
| `go` | Build, test, `go vet` | ✓ | go1.26.5 darwin/arm64 | — |

**Missing dependencies with no fallback:** none.

**Missing dependencies with fallback:** none.

Note: `task ui:build` (vendored SPA regeneration after a proto edit dirties `ui/src/lib/gen/`)
is a required CI check outside the phase gate lifecycle per STATE.md's "CI gates outside the
phase lifecycle" note — the plan should still run it locally even though it is not part of the
Nyquist validation loop below.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (`go test`) |
| Config file | none — plain `go test ./...` via Taskfile |
| Quick run command | `go test ./internal/server/... -run TestConnectRecordStateParity -v` (name TBD at plan time — see Wave 0 Gaps) |
| Full suite command | `task` (lint + test, per CLAUDE.md's Task runner convention) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-connect-record-state-parity | Every `memoryToProto`-mapped proto field is populated from a real `store.Memory` (SC1, SC2) | unit | `go test ./internal/server/... -run TestConnectRecordStateParity -v` (name TBD) | ❌ Wave 0 |
| REQ-connect-parity-roundtrip-proof | Exhaustive detector proves it can both PASS on a fully-mapped struct and FAIL on the permanent negative fixture (SC2, D-07) | unit | `go test ./internal/server/... -run TestConnectRecordStateParity -v` (same file, sub-test, name TBD) | ❌ Wave 0 |
| REQ-connect-record-state-parity (SC3 slice) | Sub-second `not_before`/`not_after` submitted via Connect comes back identical on MCP and Connect read lanes | unit/integration | `go test ./internal/server/... -run TestBoundarySecond -v` (name TBD) | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** the specific new test's `-run` command above.
- **Per wave merge:** `go test ./internal/server/... ./internal/store/...`
- **Phase gate:** `task` (full lint + test suite) green before `/gsd-verify-work`, plus a local
  `task ui:build` run since this phase dirties `ui/src/lib/gen/` (see Environment Availability
  note — not covered by the phase gate itself).

### Wave 0 Gaps

- [ ] New test file(s) in `internal/server` for the exhaustive parity/population/negative-fixture
      test (D-05/D-06/D-07) — does not exist yet, this phase creates it.
- [ ] New test file or sub-test for the boundary-second read-lane-identity assertion (D-09) —
      does not exist yet; may reuse `dialRawQdrantClient` (`schemaversion_wire_test.go:175`,
      confirmed this session as an existing raw-Qdrant seed helper) if a hand-shaped sub-second
      record needs to bypass `payload()`'s codec.
- [ ] No new framework install needed — `testing` stdlib only, already in use throughout
      `internal/server`.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | no | This phase adds no new auth surface; existing bearer-token middleware (`internal/auth`) is untouched. |
| V3 Session Management | no | Not touched. |
| V4 Access Control | no | This phase adds no new authz-relevant field to any recall/authz filter — D-04's explicit note that `schema_version` (and by extension, none of the eight new fields) may ever be read by a recall/authz filter carries forward from Phase 2's D-13/D-16. |
| V5 Input Validation | no | The eight new proto fields are all response-side (`Memory` is returned, never accepted as request input in this phase); no new client-writable surface is introduced. `StoreMemoryRequest`/`ScheduleMemoryRequest` are unchanged. |
| V6 Cryptography | no | Not touched. |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| Information disclosure via a store-only diagnostic field accidentally reaching the wire | Information Disclosure | The `json:"-"` convention (`EmbedderIdentity`, `IdempotencyFingerprint`) already gates this at the Go struct level; D-05's exhaustive test makes the boundary enforced-by-construction rather than convention-only for every field this phase touches. `summary_model`/`summary_egress_at` were deliberately re-evaluated (not silently exposed) via the D-04 reversal of `zyaa3m2fvd` — both were already MCP-visible, so no NEW disclosure occurs. |
| Field-number confusion enabling wire-format ambiguity across binary versions | Tampering | `buf breaking` in FILE mode (already configured, `buf.yaml`) plus the permanent field-number table in D-04 — reusing a `deprecated = true` field's number is the one way an "additive" change trips this (Pitfall 2), and this phase's numbers (23-30) are confirmed unused today. |

This phase is a read-side wire-representation change with no new authentication, authorization,
or input-validation surface — the security-relevant work already done (owner-scoped recall
filters, `json:"-"` store-only fields) is reused, not modified.

## Sources

### Primary (HIGH confidence)

- Live repo files read this session with line anchors: `proto/engram/v1/engram.proto:13-42`,
  `internal/store/store.go:180-370`, `internal/server/connectapi.go:1-130`,
  `internal/server/protoconv.go:100-176`, `internal/server/surfaces_test.go:1-71`,
  `internal/server/schemaversion_wire_test.go:1-80`,
  `internal/server/connectapi_write_parity_test.go:1-60,150-239`,
  `cmd/engram/client_common.go:360-391`, `buf.yaml`, `buf.gen.yaml`, `go.mod`, `Taskfile.yaml`
  (task names only), `internal/migrate/migrate.go:20,54`, `.planning/phases/05-.../05-CONTEXT.md`,
  `.planning/REQUIREMENTS.md`, `.planning/STATE.md`.
- `/protocolbuffers/protobuf-go` via context7 — `protojson.MarshalOptions` field semantics
  (`EmitDefaultValues` vs. `EmitUnpopulated`), well-known-type JSON mapping.
- `/websites/buf_build` via context7 — `breaking: use:` categories (FILE is default/strictest,
  catches file-level wire/source breaks), `buf.yaml` v2 configuration shape.

### Secondary (MEDIUM confidence)

None — every claim in this document traces to a primary source read or fetched this session.

### Tertiary (LOW confidence)

None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; existing `google.golang.org/protobuf`,
  `connectrpc.com/connect`, `buf` versions confirmed live in `go.mod`/CLI.
- Architecture: HIGH — package placement and import-cycle safety confirmed by reading actual
  import statements, not inferred; every code pattern cited was read verbatim this session.
- Pitfalls: HIGH — `buf breaking` scope and `EmitDefaultValues` semantics confirmed via official
  protobuf-go/buf documentation through context7, cross-checked against this repo's actual
  `buf.yaml` and `renderJSON` configuration.

**Research date:** 2026-08-15
**Valid until:** 30 days (stable Go/protobuf toolchain; no fast-moving external dependency in
this phase's scope) — but this research is also tightly coupled to the CURRENT state of
`store.Memory` and `engram.proto` (field counts, line numbers) and should be treated as stale
immediately if either file changes materially before planning completes.
