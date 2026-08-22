# Phase 2: Record Schema Versioning Foundation - Pattern Map

**Mapped:** 2026-08-13
**Files analyzed:** 6 (1 new package + 5 files/loci modified in `internal/store`)
**Analogs found:** 6 / 6

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/migrate/migrate.go` (new) | model/config (named type + constant) | transform (no I/O) | `internal/openaiurl/openaiurl.go` | exact |
| `internal/migrate/migrate_test.go` (new) | test | transform | `internal/openaiurl/openaiurl_test.go` | exact |
| `internal/store/store.go` — `Memory` struct field addition (`:185-314`) | model | CRUD (field on aggregate) | `EmbedderIdentity`/`AccessCount` fields in same struct | exact (same file, sibling fields) |
| `internal/store/store.go` — `payload()` stamp (`:545`) | service (codec, encode) | transform | the existing `access_count`/`embedder_identity` unconditional-key idiom in the same function | exact |
| `internal/store/store.go` — `fromPayload()` decode (`:617`) | service (codec, decode) | transform | the existing `if v, ok := p["access_count"]; ok { ... }` idiom, same function | exact |
| `internal/store/store.go` — `ensureIndexes()` (`:514`) | config/migration (index setup) | batch (idempotent setup) | existing `short_id`/`scope` index entries in the same `idxs` slice | exact |
| new gate test file (e.g. `internal/store/schemaversion_gate_test.go`) | test (structural + negative) | transform (AST scan / filter-object walk) | `internal/store/collectionprefix_conformance_test.go` | role-match (AST scan idiom identical; target shape differs — see finding 3 below) |
| `internal/store/store_test.go` — `TestPayloadRoundTripsSchemaVersion` (new func in existing file) | test | transform | `TestPayloadRoundTripsShortID` / `TestPayloadRoundTripsEmbedderIdentity` (`store_test.go:2924-2975`) | exact |

## Pattern Assignments

### `internal/migrate/migrate.go` (new leaf package)

**Analog:** `internal/openaiurl/openaiurl.go` (33 lines, stdlib-only, single exported `Join` function) — RESEARCH.md's own designated analog, confirmed by direct read.

**File header / doc-comment style** (`internal/openaiurl/openaiurl.go:1-10`):
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package openaiurl builds OpenAI-compatible endpoint URLs from an operator
// base URL and a request-path suffix (e.g. "embeddings", "chat/completions").
// It is the single shared implementation of the shape-aware provider-endpoint
// join used by both internal/embed and internal/summarize (D-14) — a stdlib-
// only leaf package deliberately so either lane can import it with zero cycle
// risk.
package openaiurl

import "strings"
```
Apply directly: `package migrate` doc comment should name (a) what it holds — the version type + current-version constant — (b) that it is intentionally stdlib-only, (c) the dependency-direction fact from D-04 (`internal/store` imports `internal/migrate`, never the reverse), and (d) that Phase 3 grows the step registry into this same package. No imports needed for `type Version int` + a constant — even leaner than `openaiurl`'s single `"strings"` import.

**Named-type + constant idiom:** `openaiurl` has no named type to mirror (it exports only a function), so the closer in-repo precedent for "named int-backed type + doc comment on its own semantics" is `store.SummarySource` (a `type SummarySource string` with exported constants `SummarySourceClient`/`SummarySourceAuto`/`SummarySourceNone` — same file, `internal/store/store.go`, referenced at `store_test.go:2907` as `SummarySourceAuto`). Follow that shape for `type Version int` + `const CurrentVersion Version = <value>`, with a doc comment on `Version` stating the zero-value-is-absence contract (D-09) directly on the type, not only in `store.Memory`'s field comment.

**Package layout to match** (single small file + single test file, no subpackages):
```
internal/migrate/
├── migrate.go
└── migrate_test.go
```

---

### `internal/migrate/migrate_test.go` (new)

**Analog:** `internal/openaiurl/openaiurl_test.go` (74 lines, plain `testing`, table-driven, no external assertion library).

**Structure to mirror** (`openaiurl_test.go:1-13`):
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package openaiurl

import "testing"

// TestJoin is the invariant test for D-14: ...
func TestJoin(t *testing.T) {
    shapes := []struct{ ... }{ ... }
    for _, shape := range shapes {
        t.Run(shape.name, func(t *testing.T) { ... })
    }
}
```
For `migrate_test.go`, the equivalent invariant to pin is: `CurrentVersion` is a `Version` (type-checks against `Version` at compile time — no runtime assertion needed for that half) and its concrete numeric value is the one the plan records against the open question (Version=0 or 1). A minimal test asserting `CurrentVersion == Version(<n>)` with a comment naming the cross-phase decision is sufficient; this package has no branching logic to table-test the way `Join` does.

---

### `internal/store/store.go` — `Memory` struct field (new `SchemaVersion` field)

**Analog:** the `AccessCount`/`EmbedderIdentity` fields already in the same struct (`store.go:242-247`, `:280-290`).

**Absent-safe field doc-comment pattern to copy** (`store.go:242-247`):
```go
// AccessCount is the monotonic total of strong-signal touches (get-by-id +
// update; never search/list result-set membership, D-02). Server-set only —
// no client-writable tool argument sets it. A legacy record missing the
// payload key reads 0, no backfill required (D-03). MUST NOT be read by the
// reranker or any recall gate (D-08).
AccessCount uint64 `json:"access_count"`
```
`SchemaVersion` should carry the same three assertions in its own doc comment: (1) server-set/monotonic, (2) legacy-absent reads as zero, no backfill, (3) MUST NOT be read by any recall gate — directly citing D-05/D-09/D-13 the way `AccessCount`'s comment cites D-02/D-03/D-08.

**Divergence to note explicitly in the new field's comment:** unlike `EmbedderIdentity`/`IdempotencyFingerprint` (`store.go:284-289`, `:296-302`), which use `json:"-"` with a comment explaining why they're hidden from the wire, `SchemaVersion` uses a **plain** `json:"schema_version"` tag with **no** `omitempty` (D-10) — the new field's comment should point at those two fields' comments and say "diverges from this precedent, deliberately" so a future reader does not "fix" it into `json:"-"` by pattern-matching the two adjacent payload-only fields.

**Key constant idiom to copy** (`store.go:316-325`):
```go
// embedderIdentityKey is the shared Qdrant payload key for
// Memory.EmbedderIdentity, written by payload() and read by fromPayload().
// Reused verbatim by Store.Reindex's divergent raw-map write (13-03) — defined
// once here so the two sites cannot drift.
const embedderIdentityKey = "embedder_identity"
```
If a shared key constant is wanted for `"schema_version"` (optional — most other fields just inline the string literal), follow this exact comment shape. Not strictly required: most payload keys (`content`, `scope`, `access_count`, etc.) are inlined as string literals in both `payload()` and `fromPayload()` with no named constant — only the two payload-only fields get constants because Reindex's raw-map write (`store.go:3207-3212`) needs to reference the same string twice across files. `schema_version` has no such second call site in this phase, so inlining `"schema_version"` directly (matching `access_count`'s treatment) is the lower-friction choice and the more common convention in this file.

---

### `internal/store/store.go` — `payload()` stamping seam (D-01/D-05)

**Analog:** the function's own existing unconditional-key lines, `access_count`/`embedder_identity`/`idempotency_fingerprint` (`store.go:584-586`):
```go
p["access_count"] = m.AccessCount
p[embedderIdentityKey] = m.EmbedderIdentity
p[idempotencyFingerprintKey] = m.IdempotencyFingerprint
```
These three are the shape to copy for D-05's monotonic stamp — an **unconditional** assignment (no `if` guard, unlike the `omitempty`-style optional fields such as `not_before`/`archived_at`/`short_id` at `:565-570`, `:581-583`, `:598-600`). The new line reads:
```go
p["schema_version"] = int64(max(migrate.CurrentVersion, m.SchemaVersion))
```
(exact value type depends on how `fromPayload` reads it back — see below; `qdrant.Value` integer fields are read via `GetIntegerValue() int64`, so writing an `int64`-convertible value, mirroring `m.AccessCount uint64` written directly as `p["access_count"] = m.AccessCount`, is the established idiom — no explicit int64 cast is even present for `access_count`, so match that: write `m.SchemaVersion`-derived value directly, letting `qdrant.NewValueMap` handle the conversion, exactly as the `AccessCount` line does).

**Where in the function to place it:** immediately adjacent to the `access_count`/`embedder_identity` block (`:584-586`), since D-01/D-09 group this field with the other "always-present, server-set, absent-safe" fields — not with the `if m.X != nil` conditional block above it.

---

### `internal/store/store.go` — `fromPayload()` decode seam (D-08/D-09)

**Analog:** the existing `access_count` decode, the closest sibling with identical "absent reads as zero value" semantics (`store.go:678-680`):
```go
if v, ok := p["access_count"]; ok {
    m.AccessCount = uint64(v.GetIntegerValue())
}
```
Copy verbatim shape for `schema_version`:
```go
if v, ok := p["schema_version"]; ok {
    m.SchemaVersion = migrate.Version(v.GetIntegerValue())
}
```
No `else` branch, no explicit zero-assignment — `m.SchemaVersion` is already `Version(0)` (Go zero value) when the key is absent, exactly matching how `m.AccessCount` stays `0` and `m.EmbedderIdentity` stays `""` for legacy records (confirmed pattern at `store_test.go:2949-2956`'s `TestPayloadRoundTripsEmbedderIdentity` legacy case: a payload map with only `"content"` set, asserting the missing field decodes to its zero value with no panic).

**Tolerant-decode precedent (only if D-08's raw-injection test needs it):** `supersedesFromPayload` (`store.go:3270`, tested at `store_test.go:4837` `TestSupersedesFromPayloadTolerantDecode`) reads a scalar as a one-element list — not directly applicable to `schema_version` (a plain integer field, no list/scalar ambiguity), but is the shape to reach for if a malformed value needs tolerance rather than a hard type assertion.

---

### `internal/store/store.go` — `ensureIndexes()` (D-12)

**Analog:** the existing `idxs` slice, specifically the `short_id` keyword-index entry (`store.go:520-526`):
```go
idxs := []idx{
    {"owner", qdrant.FieldType_FieldTypeKeyword,
        qdrant.NewPayloadIndexParamsKeyword(&qdrant.KeywordIndexParams{IsTenant: qdrant.PtrOf(true)})},
    {"scope", qdrant.FieldType_FieldTypeKeyword, nil},
    {"created_at", qdrant.FieldType_FieldTypeDatetime, nil},
    {"short_id", qdrant.FieldType_FieldTypeKeyword, nil},
}
```
Add a fifth entry `{"schema_version", qdrant.FieldType_FieldTypeInteger, nil}` (integer field type, not keyword — `schema_version` is stored as an integer, matching how the value is written/read via `GetIntegerValue()`). The function is already idempotent by construction (the `AlreadyExists` tolerance at `:536-538` applies uniformly to every entry in the slice) — no additional code needed beyond the one slice entry.

---

### New structural gate test file (D-03: "every full write routes through `payload()`")

**Analog:** `internal/store/collectionprefix_conformance_test.go` (full file read, 469 lines) — the established go/ast + go/parser + go/token source-level scan idiom in this exact package.

**Reusable shape (call-site identity, not argument shape):**
```go
// Source: collectionprefix_conformance_test.go:140-150 — isStoreConstructorCall.
// The pattern to mirror: match a specific call expression by identity
// (function name + qualifying package), not by inspecting its arguments.
func isStoreConstructorCall(call *ast.CallExpr, allowUnqualified bool) bool {
    switch fn := call.Fun.(type) {
    case *ast.Ident:
        return allowUnqualified && fn.Name == "New"
    case *ast.SelectorExpr:
        pkg, ok := fn.X.(*ast.Ident)
        return ok && pkg.Name == "store" && fn.Sel.Name == "New"
    default:
        return false
    }
}
```
For D-03, the AST target is different (per RESEARCH.md's corrected finding): scan every `*qdrant.PointStruct` composite literal in `internal/store/*.go` (non-test) whose `Payload` field value is **not** the expression `qdrant.NewValueMap(payload(...))`, and assert the allowlist of exceptions is exactly `{Reindex's per-point write at store.go:3213 (already carries a doc-comment naming the `embedder_identity` exception at :3207-3210 — extend that comment to name the whole write, per RESEARCH.md's Pattern 1)}`. Follow `collectionprefix_conformance_test.go`'s **good/bad fixture** structure (`testdata/collectionprefix/good_pkg_test.go.txt` / `bad_pkg_test.go.txt`, read via `scanConstructions` in the test) for D-15's prove-red-then-revert requirement — i.e. this gate should have its own `testdata/<name>/good_*.txt` and `bad_*.txt` fixtures asserting **set equality**, not `len(...) > 0` (mirroring `TestEveryStoreConstructionRoutesThroughSeam`'s "bad fixture yields a finding for BOTH bypass shapes" subtest at `:264-292`, which explicitly asserts the finding set, not just non-zero count).

**Zero-applicability guard to copy** (`collectionprefix_conformance_test.go:294-300`):
```go
t.Run("zero-applicability guard: nonexistent package directory fails loudly", func(t *testing.T) {
    fset := token.NewFileSet()
    _, _, err := scanPackageDir(fset, "testdata/does-not-exist-zzz", false)
    if err == nil {
        t.Fatal("scanPackageDir(nonexistent dir) = nil error, want a failure — a gate that silently scans nothing must not report clean")
    }
})
```
Same discipline applies to D-14's filter-builder enumeration: assert `filesScanned/buildersWalked != 0`, never trust a bare pass.

---

### D-13/D-14 negative recall-gate test: filter-object walker

**Finding (per the mapping context's explicit ask #3):** No existing test or helper in `internal/store` walks a constructed `*qdrant.Filter` object's conditions. Verified: `rg -n "range.*Must\b|range.*Should\b|func.*[Ww]alk.*[Ff]ilter" internal/store/*.go` returns nothing (RESEARCH.md's own grep, re-confirmed). Every existing filter-related test in this package asserts on **query results** (does `Search` return record X), never on the shape of the `*qdrant.Filter` object itself. **There is no reusable walker to extend — D-13's walker is new code, built from scratch.**

**Closest structural analog (idiom, not object shape):** `collectionprefix_conformance_test.go`'s "derive the set, assert equality" discipline (Pattern 2 in RESEARCH.md) is the idiom to reuse — same package, same "don't hand-list, derive and assert completeness" design — but the traversal target is different: source-level `go/ast` walking (collectionprefix) vs. runtime `*qdrant.Filter` struct graph walking (schema-version gate). The filter object itself (`qdrant.Filter{Must, Should, MustNot []*qdrant.Condition}`, and `qdrant.Condition`'s `FieldCondition`/`IsEmptyCondition`/`Filter` (nested) oneof variants) is a plain proto-generated struct — no reflection needed, a straightforward recursive function.

**Concrete filter-construction shapes the walker must recognize** (from `store.go`, read this session):
```go
// store.go:885-891 — ownerScopeFilter: top-level *qdrant.Filter{Must: [...]}
func (s *Store) ownerScopeFilter(ctx context.Context, scope string, subj Subject) *qdrant.Filter {
    must := make([]*qdrant.Condition, 0, 2)
    if scope != "" {
        must = append(must, qdrant.NewMatch("scope", scope))
    }
    must = append(must, s.ownerOrSharedCondition(ctx, subj))
    return &qdrant.Filter{Must: must}
}

// store.go:924-934 — categoryMatchCondition: a NESTED *qdrant.Filter wrapped
// as a *qdrant.Condition via NewFilterAsCondition (the walker MUST recurse
// into this, not just scan top-level Must/Should).
func categoryMatchCondition(categories []string) *qdrant.Condition {
    should := make([]*qdrant.Condition, 0, len(categories))
    for _, c := range categories {
        if c == "" { continue }
        should = append(should, qdrant.NewMatch("category", c))
    }
    if len(should) == 0 { return nil }
    return qdrant.NewFilterAsCondition(&qdrant.Filter{Should: should})
}

// store.go:1023-1040 — Search: filter built by MUTATING f.Must in place
// after ownerScopeFilter returns (append calls, not a fresh struct literal).
f := s.ownerScopeFilter(ctx, scope, subj)
f.Must = append(f.Must, activeWindowConditions(s.now())...)
f.Must = append(f.Must, qdrant.NewIsEmpty("superseded_by"))
f.Must = append(f.Must, qdrant.NewIsEmpty("archived_at"))
f.Must = append(f.Must, tagMatchConditions(opts.Tags)...)
if c := categoryMatchCondition(opts.Categories); c != nil {
    f.Must = append(f.Must, c)
}
```
The walker must recurse through `Condition.GetFilter()` (the nested-filter oneof case `categoryMatchCondition` produces) in addition to `Must`/`Should`/`MustNot` at the top level, or it will silently miss any field name buried inside a `Should`-composed OR group — exactly the shape of miss D-14 exists to prevent.

**The five recall entry points to enumerate (verified file:line, from RESEARCH.md's direct read), each needing its own filter obtained and walked:**

| Entry point | Line | Builds its own filter via |
|---|---|---|
| `Search` | `store.go:1001` | `ownerScopeFilter` + inline appends (`:1023-1040`) |
| `SearchReranked` | `store.go:1081` | delegates to `Search` — no independent filter, do not double-count |
| `SearchDiscovery` | `store.go:1099` | inline `must` slice, does **not** call `ownerScopeFilter` (`:1118-1132`) |
| `List` | `store.go:1232` | `listFilter` (`:1200`) + inline appends (`:1260-1271`) |
| `ListScheduled` | `store.go:1468` | inline filter, does **not** call `listFilter`/`ownerScopeFilter` (`:1490-1507`) |

D-14's derive-and-assert-complete requirement means this table should be produced programmatically (e.g., an AST scan for exported/unexported funcs in `store.go` returning `*qdrant.Filter` used by these five, or a hand-enumerated list cross-checked against such a scan) rather than trusted as a static hand-written list — per RESEARCH.md Pitfall 2, `SearchDiscovery` and `ListScheduled` are the two most likely to be silently skipped by a naive "walk the shared helpers" implementation.

## Shared Patterns

### Absent-payload-key-reads-as-zero (house style)
**Source:** `internal/store/store.go` — `AccessCount` field comment (`:242-247`) + its `payload()`/`fromPayload()` treatment (`:584`, `:678-680`); also `EmbedderIdentity` (`:280-290`, `:585`, `:681-683`).
**Apply to:** the `SchemaVersion` field, its `payload()` write, and its `fromPayload()` read — no new mechanism needed, D-09 is this same idiom.

### Unconditional (non-`omitempty`) payload write vs. conditional
**Source:** `payload()` (`store.go:550-564` unconditional block vs. `:565-570`/`:581-583`/`:598-600` conditional `if m.X != nil` / `if m.X != ""` blocks).
**Apply to:** `schema_version` belongs in the **unconditional** block (always written, like `access_count`), never the conditional block (unlike `short_id`/`archived_at`, which are legitimately absent for some records at the payload-key level even though the field always has *some* Go zero value).

### AST-based source-level conformance gate
**Source:** `internal/store/collectionprefix_conformance_test.go` (whole file), specifically its `scanConstructions`/`isStoreConstructorCall`/set-equality-assertion pattern (`:140-206`, `:264-292`).
**Apply to:** D-03's "every full write routes through `payload()`" gate — same package, same `go/ast`+`go/parser`+`go/token` toolchain, same "identity of the call site, not shape of its arguments" anchor, same good/bad-fixture-with-set-equality assertion style.

### Payload round-trip pure-function tests (no Qdrant/testcontainer needed)
**Source:** `store_test.go:2924-2975` — `TestPayloadRoundTripsShortID`, `TestPayloadRoundTripsEmbedderIdentity`, `TestPayloadRoundTripsIdempotencyFingerprint`.
**Apply to:** `TestPayloadRoundTripsSchemaVersion` (new, same file) — call `payload(m)` / `fromPayload(id, qdrant.NewValueMap(...))` directly, no `TestMain`/testcontainer dependency, covering (a) non-zero round-trip, (b) legacy-absent decodes to `Version(0)`, (c) D-05's monotonic max rule (the one assertion unique to this field, with no existing sibling to copy — must be authored fresh).

## No Analog Found

| File/Locus | Role | Data Flow | Reason |
|---|---|---|---|
| D-13 filter-object walker (runtime `*qdrant.Filter` tree walk) | test helper | transform (object-graph traversal) | Confirmed via grep (RESEARCH.md + this session): no existing helper of this shape exists anywhere in the repo. Nearest idiom is `collectionprefix_conformance_test.go`'s "derive-the-set, assert completeness" discipline, but its traversal target (source AST) differs from this walker's target (runtime proto struct graph). Must be built from scratch as a new, small, unexported recursive function in the new gate test file — a `func walkFilter(f *qdrant.Filter, keys map[string]bool)` (or similar) recursing through `Must`/`Should`/`MustNot` and, per condition, `GetField().GetKey()` / `GetIsEmpty().GetKey()` / `GetFilter()` (nested, recurse). |
| D-08 forward/backward compat raw-injection test | test (integration, real Qdrant) | request-response (raw SetPayload + full recall paths) | No existing single test exercises "raw `SetPayload` with an unknown key + version, then Search/List/get_memory all still return it." `TestSupersedesFromPayloadTolerantDecode` (`store_test.go:4837`) is the closest partial precedent (tests tolerant *decode* only, not the full round-trip-plus-recall-plus-get_memory assertion chain D-08 needs) — build fresh, reusing that test's raw `map[string]*qdrant.Value`/`client.SetPayload` construction style as a starting shape, not a shape to extend. |

## Metadata

**Analog search scope:** `internal/store/*.go` (non-test and test), `internal/openaiurl/`, `internal/migrate` (confirmed does not yet exist), `internal/server/summary.go` (read for D-11 confirmation, no pattern extraction needed — zero code change there this phase).
**Files scanned:** `internal/openaiurl/openaiurl.go`, `internal/openaiurl/openaiurl_test.go`, `internal/store/store.go` (lines 185-330, 490-729, 880-1040), `internal/store/store_test.go` (lines 2900-2980), `internal/store/collectionprefix_conformance_test.go` (full file, 469 lines).
**Pattern extraction date:** 2026-08-13
