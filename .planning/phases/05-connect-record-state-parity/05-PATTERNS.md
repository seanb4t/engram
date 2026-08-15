# Phase 5: Connect Record-State Parity - Pattern Map

**Mapped:** 2026-08-15
**Files analyzed:** 6 (2 modified, 2 new test files, 2 modified-comment-only)
**Analogs found:** 6 / 6 (all in-repo; every excerpt below re-read live, not from RESEARCH.md memory)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `proto/engram/v1/engram.proto` (`message Memory`, +fields 23-30) | schema/config | request-response | itself — fields 1-22 already in the same message | exact (additive continuation) |
| `internal/server/connectapi.go` (`memoryToProto`) | service (mapper) | transform | `memoryToProto`'s own existing `LastAccessedAt` nil-guard | exact |
| `internal/store/store.go` (`SummaryEgressAt` comment repair) | model | — | same file, adjacent doc comments | exact |
| `internal/server/connectapi_parity_test.go` (NEW — exhaustive parity/population/negative-fixture test) | test | transform | `internal/server/surfaces_test.go` (`jsonschemaExposedFields`) for the detector; `internal/server/connectcsrf_lane_test.go` (`TestCSRFFailedBearerNeverFallsThroughToExemption`) for the permanent-negative-fixture shape | role-match (detector) / partial (negative-fixture idiom, different domain) |
| `internal/server/connectapi_boundary_second_test.go` (NEW — D-09 boundary-second read-lane-identity test) | test | request-response | `internal/server/schemaversion_wire_test.go` (`TestSchemaVersionOnGetMemoryWire`, `dialRawQdrantClient`) | exact |
| `cmd/engram/client_common.go` renderJSON — no code change, only a NEW test asserting its `schema_version: 0` output | test | request-response | `renderJSON` itself (`client_common.go:369-386`) | exact (test targets existing function unchanged) |

## Pattern Assignments

### `proto/engram/v1/engram.proto` (schema, request-response)

**Analog:** the existing `message Memory` block itself, `proto/engram/v1/engram.proto:13-42`.

**Verbatim current state (fields 1-22, confirmed live):**
```protobuf
// A single memory record (mirrors internal/store.Memory's readable fields).
message Memory {
  string id = 1;
  string content = 2;
  string scope = 3;
  string repo = 4;
  string workspace = 5;
  string worktree = 6;
  string base_dir = 7;
  string source = 8;
  string category = 9;
  repeated string tags = 10;
  string actor = 11;
  string owner = 12;
  string visibility = 13;
  google.protobuf.Timestamp created_at = 14;
  string summary = 15;
  string summary_source = 16;
  // Qdrant similarity score on search results (higher = closer); 0 on
  // list/get results, which are not ranked.
  float score = 17;
  string short_id = 18;
  // Monotonic count of strong-signal touches (get-by-id + update).
  uint64 access_count = 19;
  // Recency of the last strong-signal touch.
  google.protobuf.Timestamp last_accessed_at = 20;
  // Discovery-only fields; empty on plain memories (never set by
  // store_memory/schedule_memory, only by store_discovery).
  string kind = 21;
  repeated Citation citations = 22;
}
```

> **D-14 correction (2026-08-15).** Three statements in this document were written against D-01
> and D-02, which Sean REVERSED on 2026-08-15. Each is corrected inline below and flagged
> `D-14 CORRECTION`. Under D-14, fields 23, 28 and 29 are `optional string`, `optional uint32`,
> `optional string` — generating `*string`, `*uint32`, `*string`. Where this document and a PLAN
> disagree, the PLAN wins.

**Field-comment style to copy for the new 23-30 block:** a doc comment sits directly above
a field only when the field's semantics are non-obvious from its name alone (see `score`,
`access_count`, `last_accessed_at`, `kind`/`citations` above — each states *when* the field is
populated/zero and by which lane). Plain self-explanatory fields (`id`, `content`, `scope`,
etc.) get no comment. Apply this selectively to fields 23-30: `superseded_by`/`supersedes`
need a one-liner on soft-hide-from-recall vs `get_memory` (mirrors `store.go` comment intent,
not copied verbatim — proto comment should be terse); the read-side-rounding-is-a-no-op note
(D-09) belongs on `not_before`/`not_after`. Do not import store.go's paragraph-length comments
wholesale — proto comments in this file are 1-3 lines.

> **D-14 CORRECTION.** This paragraph originally said `schema_version` "needs the 'zero is v0 is
> absent' statement (D-01)". D-01 is REVERSED. `schema_version` is `optional uint32` and its
> comment must state explicit presence plus that the server ALWAYS sets it — including to zero for
> a v0 record — and that an unset value on the wire is a server bug. Likewise `superseded_by` is
> `optional string`: UNSET means not superseded, not empty-string. See 05-01-PLAN.md's task 1.

**Field-number continuation:** next field number is 23 (current max is 22, `citations`). No
existing field/number in 1-22 is `deprecated = true` (that pattern exists elsewhere in this
same file at `ListMemoriesResponse.approximate = 3 [deprecated = true];`, `engram.proto:81`
per RESEARCH.md — confirmed as a live example of the constraint, not something to copy, just
to avoid re-triggering by touching an unrelated field).

---

### `internal/server/connectapi.go` — `memoryToProto` (service/mapper, transform)

**Analog:** the function's own existing body — this is an in-place extension, not a
new-file-from-analog case.

**Full current source, verbatim (`internal/server/connectapi.go:49-71`):**
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

**Extension pattern:** add three more `var x *timestamppb.Timestamp; if m.Y != nil { x =
timestamppb.New(*m.Y) }` blocks above the `return`, one each for `NotBefore`, `NotAfter`,
`ArchivedAt` (all `*time.Time` on `store.Memory`) — copy the `lastAccessed` block's shape
exactly, including the "leave unset rather than emitting a year-1 Timestamp" comment style
adapted per field. `SummaryEgressAt` is a **non-pointer** `time.Time`; its guard is `if
!m.SummaryEgressAt.IsZero()` instead of a nil check — same emit-only-if-set outcome, different
zero-test idiom.

> **D-14 CORRECTION — this is the largest change in this document.** The original text here
> described D-02's plain-string collapse for `SupersededBy` and the composite-literal-to-named-local
> restructure it forced. **D-02 is REVERSED and that restructure must NOT be written.** Under D-14
> all four non-Timestamp assignments stay INSIDE the composite literal and `memoryToProto` keeps its
> single-expression return:
> - `SupersededBy: m.SupersededBy` — a direct `*string`-to-`*string` copy. Nil source stays an unset
>   proto field; do not dereference, nil-guard, or substitute `""`.
> - `Supersedes: m.Supersedes` — direct, unchanged.
> - `SchemaVersion: proto.Uint32(uint32(m.SchemaVersion))` — UNCONDITIONAL. `migrate.Version` is a
>   named `int`, so the conversion is required and `unconvert` will not flag it.
> - `SummaryModel: proto.String(m.SummaryModel)` — UNCONDITIONAL.
>
> The word UNCONDITIONAL on the last two is load-bearing and is the one real hazard D-14
> introduces: protojson OMITS an unset `optional` field, and `EmitDefaultValues` does not override
> that (verified against protobuf-go v1.36.11). An `if` around either assignment silently drops the
> key from every rendered JSON document for a zero-valued record. Do NOT copy the neighbouring
> Timestamp nil-guards' shape onto them. This adds `"google.golang.org/protobuf/proto"` to the
> file's imports — the repo has no `proto.String`/`proto.Uint32` call site yet.

---

### `internal/store/store.go` — `SummaryEgressAt` comment repair (model, doc-only)

**Analog:** the same file's own adjacent comment style — no external analog needed, this is a
one-comment factual correction.

**Current text to repair (`internal/store/store.go:272-276`, exact lines):**
```go
	// SummaryEgressAt is the k1oe.2 durable audit stamp: when this record's
	// content was egressed to the summarizer model (auto path only). Store-only;
	// not on the Connect wire. Zero if never egressed or the summary was
	// client-authored/cleared.
	SummaryEgressAt time.Time `json:"summary_egress_at"`
```
D-04 requires striking "Store-only; not on the Connect wire." (both clauses are false after
this phase: it already carries a plain json tag reaching the MCP wire, and after this phase it
also reaches the Connect wire). Replace with a statement of what is actually true — e.g. that
the field is a plain, always-visible json tag reaching both the MCP and (as of this phase)
Connect wires, zero meaning never-egressed.

---

### `internal/server/connectapi_parity_test.go` (NEW test — exhaustive detector, D-05/D-06/D-07)

**Primary analog for the detector walker — `internal/server/surfaces_test.go:29-50`, quoted in full:**
```go
// jsonschemaExposedFields returns the json tag name (before the comma) for
// every VISIBLE field of t that carries a json tag — the REAL, reflected
// field-name set this struct exposes on the wire, never a hand-maintained
// list. reflect.VisibleFields (not a shallow t.NumField() walk) is required
// so an anonymously-embedded struct's promoted fields (e.g. scheduleArgs'
// embedded storeArgs.Category) are seen exactly as jsonschema.For[T]'s own
// schema generation sees them.
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
Mirror this shape exactly for D-05's `store.Memory` walker: `reflect.VisibleFields(reflect.TypeOf(store.Memory{}))`,
same `json:"-"`-skip logic, same comma-split. `store.Memory` is a flat struct (no embeds
today, confirmed by reading `store.go:186-354` in full) so `VisibleFields` degenerates to a
plain field walk here, but use it anyway to match the established idiom and stay correct if a
future field arrives via embedding.

**Rename-map extension (the one alias D-05 requires) — verified live:**
- `internal/store/store.go:197`: `Worktree  string   \`json:"worktree_path"\`
- `proto/engram/v1/engram.proto:19`: `string worktree = 6;`
This is the single name divergence across all 30 json-visible `store.Memory` fields (confirmed
by cross-referencing every json tag against every proto field name this session). Model the
alias map on the shape RESEARCH.md proposes:
```go
var storeToProtoFieldAlias = map[string]string{
	"worktree_path": "worktree",
}
```
The detector must fail loudly on any *other* unaliased name mismatch — no fuzzy/substring
matching (see Anti-Patterns below).

**The two `json:"-"` precedents the inclusion rule is built on — verified live (`internal/store/store.go:291`, `:314`):**
```go
	// ... (full deliberate/load-bearing rationale precedes each, see store.go:279-290 and :294-313)
	EmbedderIdentity string `json:"-"`
	// ...
	IdempotencyFingerprint string `json:"-"`
```
These two comments already state "deliberate and load-bearing" — D-05 promotes that convention
from comment-only to test-enforced. No other `json:"-"` exists on `store.Memory` today.

**Permanent-negative-fixture idiom (D-07) — closest analog is `internal/server/connectcsrf_lane_test.go:120-152`,
`TestCSRFFailedBearerNeverFallsThroughToExemption`**, a "permanent negative, end-to-end" test
that routes a deliberately-bad input through the SAME real resolver/interceptor chain the
happy-path tests use, and asserts a specific rejection code — not a lookalike stub path. This
is a **partial match only**: it proves a security gate can reject, not that a reflection-based
field-mapping detector can reject. No existing test in this repo exercises a detector *function*
(as opposed to an end-to-end request path) against both a positive and a negative fixture through
one shared call site. **This absence is planning-critical**: D-07's structural requirement (the
real path and the negative-fixture path must route through one shared detector function) has no
direct precedent to copy in this codebase — the planner should treat the CSRF test's discipline
("same real resolver, not a lookalike") as the transferable principle, but the concrete Go
shape (a `func detectUnmappedFields(...) []string` or similar, called both on
`store.Memory` and on a test-only struct) must be authored fresh. The nearest *structural*
kin found by search (`internal/store/revert_test.go:307-312`, `internal/store/migrate_test.go:850-856`)
are anti-vacuity assertions on *counts* (proving a preflight scanned the whole range, not a
batch), not on a detector's positive/negative range — also only a partial match, cited here so
the planner does not go looking for a closer one that does not exist.

**Timestamp decode-back idiom for D-08 (inline, not a named function):** no direct file-level
analog exists for "decode a `*timestamppb.Timestamp` back to `time.Time` inline inside a test
comparison" as a repo-wide idiom beyond `formatWindowBound`'s own encode-direction use of
`ts.AsTime()` (`internal/server/protoconv.go:166`, quoted below) — reuse `.AsTime()` the same
way, just in the opposite (proto→Go) direction, directly in the assertion body per D-08 (no
production `protoToMemory` function).

**Anti-pattern warning (from RESEARCH.md, confirmed against live `connectapi.go:115-129` region)
worth restating for the plan: test `memoryToProto` directly, never through `shapeProtoMemories`**
— the latter intentionally clears `Content`/`Citations`/`Kind` and rewrites `Summary` on
`full=false`, which would masquerade as parity failures if the population fixture is driven
through a handler instead of the mapper function.

---

### `internal/server/connectapi_boundary_second_test.go` (NEW test — D-09 boundary-second, request-response)

**Analog:** `internal/server/schemaversion_wire_test.go` — full file read; two directly reusable pieces.

**`dialRawQdrantClient`, verbatim (`internal/server/schemaversion_wire_test.go:150-170`, exact as read):**
```go
// dialRawQdrantClient dials a raw *qdrant.Client against the same
// testQdrantAddr this package's TestMain resolved, for the handful of
// call sites (like TestSchemaVersionOnGetMemoryWire's legacy-seed subtest)
// that must bypass store.Store's payload() codec entirely to construct the
// absent-schema_version-key shape a pre-adoption record actually has.
// Mirrors testDepsWithStore's own dial exactly (this package has no
// exported seam onto *store.Store's unexported client field).
func dialRawQdrantClient(t *testing.T) *qdrant.Client {
	t.Helper()
	if testQdrantAddr == "" {
		failOrSkipNoQdrant(t)
	}
	host, portStr, err := net.SplitHostPort(testQdrantAddr)
	if err != nil {
		t.Fatalf("invalid Qdrant address %q: %v", testQdrantAddr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		t.Fatalf("invalid Qdrant port %q (from %q): %v", portStr, testQdrantAddr, err)
	}
	c, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: port})
	if err != nil {
		t.Fatalf("raw qdrant client: %v", err)
	}
	return c
}
```
Available if the boundary-second test needs to hand-shape a sub-second-bound record bypassing
the normal `payload()` codec; more likely the test drives the bound through the real Connect
`ScheduleMemory` write RPC (per D-09's stated design: "submits a sub-second bound through the
Connect write lane"), in which case `dialRawQdrantClient` is not needed and the test instead
exercises `scheduleMemoryRequestToArgs`/`windowBoundFloor`/`windowBoundCeil` end to end — see
next excerpt.

**The write-path rounding this test drives (never reimplements) — verbatim
(`internal/server/protoconv.go:150-176`):**
```go
// windowBoundFloor / windowBoundCeil format a *timestamppb.Timestamp as a
// scheduling-window bound string for parseWindow (tools.go:452), rounding the
// bound OUTWARD to a whole second BEFORE formatting (round-8 MED, Codex):
// not_before rounds DOWN (never advances the reveal time), not_after rounds
// UP (never truncates an expiry into the past). This keeps the store's
// second-granular `.Unix()` flooring on encode/decode (store.go:320/:323/
// :406/:410) a no-op on the value protoconv hands it — a sub-second
// `not_after` is WIDENED to the containing whole-second window instead of
// silently collapsing to immediate-expiry. time.RFC3339Nano is used (the
// plain-second RFC3339 layout truncates fractional seconds) so the rounded
// whole-second value round-trips exactly. A nil timestamp maps to "" (no
// window bound).
func windowBoundFloor(ts *timestamppb.Timestamp) string {
	return formatWindowBound(ts, false)
}

func windowBoundCeil(ts *timestamppb.Timestamp) string {
	return formatWindowBound(ts, true)
}

func formatWindowBound(ts *timestamppb.Timestamp, roundUp bool) string {
	if ts == nil {
		return ""
	}
	t := ts.AsTime()
	bound := t.Truncate(time.Second)
	if roundUp && bound.Before(t) {
		bound = bound.Add(time.Second)
	}
	return bound.Format(time.RFC3339Nano)
}
```
D-09 is explicit: **no new rounding code is added.** The test's only job is to submit a
sub-second `NotBefore`/`NotAfter` through the real Connect write path (which funnels through
this exact code, unmodified) and then assert the value that comes back is bit-identical on
BOTH read lanes — MCP (`get_memory`/`full=true` recall, which reads `store.Memory` json-tag
verbatim) and Connect (`memoryToProto`, this phase's new field wiring). No new production
function.

**Structural test-shape analog (mirror-in-opposite-direction, MCP-vs-Connect wire assertion):**
`TestSchemaVersionOnRecallWire`/`TestSchemaVersionOnGetMemoryWire`
(`internal/server/schemaversion_wire_test.go:22-` onward) is the closest existing example of a
test that seeds one record and asserts the SAME value surfaces correctly on two different wire
paths — copy that file's overall shape (seed via real write path or raw client, then assert on
both `shapeRecall`/MCP marshal and the Connect proto marshal) rather than inventing a new test
harness pattern.

---

### `cmd/engram/client_common.go` — `renderJSON` (D-03 test target, no source change)

**Analog:** the function itself, unmodified — `client_common.go:369-386`, verbatim:
```go
// renderJSON marshals m as a single JSON document (D-08 — one object per
// invocation, not NDJSON) and writes it plus a trailing newline.
//
// Two option choices are load-bearing. EmitDefaultValues is what makes an
// empty result render as "memories":[] rather than omitting the key or
// emitting null (D-12). UseProtoNames keeps the field names identical to
// the .proto declaration and therefore to the short_id / created_at /
// summary_source vocabulary the MCP tool surface and CLAUDE.md's memory
// contract already use — deriving field names from the message rather than
// a hand-written Go struct is what makes D-08's "mirror the response field
// names" structurally true instead of a convention someone can drift from.
func renderJSON(w io.Writer, m proto.Message) error {
	b, err := protojson.MarshalOptions{
		UseProtoNames:     true,
		EmitDefaultValues: true,
		Multiline:         false,
	}.Marshal(m)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}
```
**Pitfall, re-verified live against protobuf-go v1.36.11 rather than reasoned about:** the four new
`google.protobuf.Timestamp` fields (`not_before`, `not_after`, `archived_at`, `summary_egress_at`)
are message-typed, not scalar — `EmitDefaultValues` does NOT force their emission when unset
(observed: `{"plain_str":""}`), and only `EmitUnpopulated`, which this project does not set, renders
them as `null` (observed: `{"plain_str":"", "not_before":null}`). D-03's test must NOT assert any
Timestamp key is present-but-null for an unscheduled/unarchived/never-egressed record — that would
test behavior `renderJSON`'s actual options do not provide. **This paragraph is unchanged by D-14.**

> **D-14 CORRECTION.** The original text here said `EmitDefaultValues: true` "means:
> `schema_version` (uint32 scalar) renders `0` when zero — this is the assertion D-03's test
> makes." That is now FALSE in both halves. Under D-14 `schema_version` is `optional uint32`, and
> observed against protobuf-go v1.36.11:
> - Unset + `EmitDefaultValues:true` → the key is **ABSENT**. The flag forces zero values for
>   IMPLICIT-presence fields only and does not rescue an unset explicit-presence field.
>   `EmitUnpopulated:true` does not rescue it either.
> - Assigned to zero → renders `"schema_version":0` — and does so with `EmitDefaultValues:false`
>   as well, so this field's visibility no longer depends on that option at all.
>
> Consequence for the test: the fixture must ASSIGN `proto.Uint32(0)` (the state `memoryToProto`
> produces for a v0 record under D-14 §3), and it must be paired with a permanent negative fixture
> leaving the field nil and asserting the key is absent — otherwise the presence assertion is a
> tautology over whatever the stub author chose. `renderJSON` is still pinned for `UseProtoNames`
> (the key's spelling) and for `EmitDefaultValues` on the implicit-presence fields around it. See
> 05-03-PLAN.md's `<d14_amendment>` and task 2.

---

## Shared Patterns

### Nil/zero Timestamp-to-proto discipline
**Source:** `internal/server/connectapi.go:50-55` (the `lastAccessed` guard inside `memoryToProto`)
**Apply to:** `NotBefore`, `NotAfter`, `ArchivedAt` (all `*time.Time` — nil guard) and
`SummaryEgressAt` (non-pointer `time.Time` — `IsZero()` guard) in the extended `memoryToProto`.

### json:"-" as the sole store-only exclusion marker
**Source:** `internal/store/store.go:291` (`EmbedderIdentity`), `:314` (`IdempotencyFingerprint`)
**Apply to:** the D-05 detector's inclusion rule — any `store.Memory` field without `json:"-"`
must have a proto counterpart (direct name match or the one `worktree_path`→`worktree` alias).

### json-tag-derived exhaustiveness walker
**Source:** `internal/server/surfaces_test.go:29-50` (`jsonschemaExposedFields`)
**Apply to:** the new detector's `store.Memory` field-name-set enumeration — copy the
`reflect.VisibleFields` + tag-split + skip-`-`/skip-empty shape verbatim, retargeted at
`store.Memory`.

### Raw-Qdrant bypass helper for hand-shaped legacy/pre-migration records
**Source:** `internal/server/schemaversion_wire_test.go:150-170` (`dialRawQdrantClient`)
**Apply to:** available to the boundary-second test if it needs to seed a record shape the
normal write path cannot produce; likely unnecessary if the test drives the real
`ScheduleMemory` Connect RPC end to end as D-09 describes.

## No Analog Found

| File/Concern | Role | Data Flow | Reason |
|---|---|---|---|
| D-07's shared-detector-function positive/negative-fixture idiom | test | transform | No existing test in this repo exercises a *reflection-based field-mapping detector function* against both a real positive input and a permanent negative fixture through one shared call site. The closest kin (`connectcsrf_lane_test.go`'s end-to-end permanent-negative test, `revert_test.go`/`migrate_test.go`'s anti-vacuity count assertions) are all partial matches in a different domain (auth gate / count preflight, not a struct-field detector). **This gap is planning-critical per the phase brief's explicit ask**: the plan must author this shape fresh, using the CSRF test's *discipline* (route the negative case through the exact same function/path as the positive case, never a lookalike) as the only available guidance, not a copyable code block. |

## Metadata

**Analog search scope:** `proto/engram/v1/`, `internal/server/` (all `*.go` and `*_test.go`),
`internal/store/store.go`, `cmd/engram/client_common.go`; targeted repo-wide `rg` for
"negative fixture" / "anti-vacuity" / "permanent negative" / "detector...reject" idioms.
**Files scanned:** 9 read directly this session (engram.proto, protoconv.go, connectapi.go,
surfaces_test.go, schemaversion_wire_test.go, store.go, client_common.go, revert_test.go,
migrate_test.go, connectcsrf_lane_test.go) plus repo-wide grep passes.
**Pattern extraction date:** 2026-08-15
