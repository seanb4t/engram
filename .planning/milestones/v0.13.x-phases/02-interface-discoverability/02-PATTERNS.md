# Phase 2: Interface Discoverability - Pattern Map

**Mapped:** 2026-08-04
**Files analyzed:** 13 (new/modified, per CONTEXT.md D-01..D-14 and RESEARCH.md's Recommended Project Structure)
**Analogs found:** 10 / 13 (3 are explicitly greenfield — no in-repo precedent; documented under "No Analog Found")

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/server/argerror.go` (+ rule registry, `conditionalErrf`) | model/constants + constructor | transform (error construction) | itself (extend in place) | exact — extending an existing, fully-read file |
| `internal/server/toolannotations.go` (new) | config/registry | CRUD-adjacent (static lookup table) | `cmd/engram/catalog.go` (`buildCatalog`'s "derive from constants, never a second literal" + `TestCatalogExitCodesMatchMapper`'s both-directions gate) | role-match — same registry-table shape, different consumer |
| `internal/toolclass/toolclass.go` (new leaf package) | config/shared model | CRUD-adjacent (static lookup table) | `internal/config/registry.go` (a small leaf package holding a declared `[]field` table + lookup functions, imported by multiple trees) | role-match — closest existing "shared small leaf table" shape in the repo |
| `internal/server/surfaces_test.go` (new, MCP-side half of D-05 gate) | test (conformance) | request-response (reads schema/description text) | `internal/server/argattribution_test.go` (identifier/hint-code equality, never wording) | exact — same assertion discipline, new subject matter |
| `internal/server/toolannotations_test.go` (new, D-10 both-directions gate) | test (conformance) | batch (map equality) | `cmd/engram/catalog_test.go`'s `TestCatalogExitCodesMatchMapper` | exact — literal shape to copy (`reflect.DeepEqual` on two independently-built maps) |
| `cmd/engram/surfaces_test.go` (new, cobra-`Usage`/proto half of D-05 gate) | test (conformance) | file-I/O (subprocess to `go tool buf build`) + request-response | `internal/server/connectdescriptor_test.go` (semantic reflection over a generated descriptor, not a golden wire snapshot) | role-match — same "reflect over the schema, not the rendered bytes" discipline, applied to `descriptorpb.FileDescriptorSet` instead of `protoreflect.FileDescriptor` |
| `cmd/engram/testdata/help.golden` + `TestHelpGolden` (D-12) | test (golden-file) | file-I/O | none in-repo — **greenfield**, see below | no analog |
| `cmd/engram/testdata/catalog.golden` + `TestCatalogGolden` (D-13) | test (golden-file) | file-I/O | `cmd/engram/catalog.go` + `catalog_test.go`'s `decodeCatalog` helper (existing JSON-decode harness to build the golden against) | role-match — reuses the existing catalog-JSON decode path, but the golden-comparison shape itself is new |
| `internal/server/tools.go` (Register() → extracted `registerTools(s, d)` seam) | service/wiring | request-response | `tools.go:1772-1783` itself (the code being refactored) | exact — in-place extraction, not a new pattern |
| Prose anchor regions: `proto/engram/v1/engram.proto` field comments | config/schema comment | transform (generated text region) | none in-repo for the *anchor* mechanism — see below; `//` line-comment convention itself is standard proto | partial — comment syntax is trivial, anchor mechanism is greenfield |
| Prose anchor regions: `docs-site/src/content/docs/reference/tools.md`, `guides/cli.md` | docs/content | transform (generated text region) | none in-repo — **greenfield**, see below | no analog |
| Prose anchor regions: `skill/engram/skills/{curating-memory,discovering}/SKILL.md` | docs/content | transform (generated text region) | none in-repo — **greenfield**, see below | no analog |
| `internal/surfacesgen/` (new generator invoked by `task surfaces:gen`) + `Taskfile.yaml` target + CI job | tooling/CLI generator | batch (regenerate-in-place) | `Taskfile.yaml`'s `proto:gen` target + `.github/workflows/ci.yaml`'s `buf` job's "generated-code drift" step | exact — explicitly named as the reference implementation in CONTEXT.md D-06/D-07 |

## Pattern Assignments

### `internal/server/argerror.go` (extend in place: rule registry + `conditionalErrf`)

**Analog:** itself — full file already read this session (180 lines).

**Existing hint vocabulary** (lines 15-37):
```go
type HintCode string

const (
	HintRequired            HintCode = "required"
	HintConditionalRequired HintCode = "conditional_required"
	HintTooLong             HintCode = "too_long"
	HintTooMany             HintCode = "too_many"
	HintEnum                HintCode = "enum"
	HintFormat              HintCode = "format"
	HintPrefix              HintCode = "prefix"
	HintOrdering            HintCode = "ordering"
	HintMutuallyExclusive   HintCode = "mutually_exclusive"
	HintNotApplicable       HintCode = "not_applicable"
)
```

**Constructors to wrap, not replace** (lines 122-149):
```go
func argErrf(class argClass, hint HintCode, field, format string, a ...any) error {
	return &argError{Fields: []string{field}, Hint: hint, Detail: fmt.Sprintf(format, a...), Class: class}
}

func argErrFieldsf(class argClass, hint HintCode, fields []string, detail string) error {
	return &argError{Fields: fields, Hint: hint, Detail: detail, Class: class}
}
```
D-04's `conditionalErrf(rule, …)` should be a third constructor with this same shape, taking a declared
`rule` value (`ID`, `Fields []string`, `Hint HintCode`, `Sentence string`) and forwarding into
`argErrFieldsf` internally — so `argFieldsOf`/`argHintOf`/`argClassOf` (lines 151-179, unchanged) and
`connectError`'s `errors.As(&argError{})` handling need zero changes; only construction call sites change.

**Existing worked-example rule site to convert into a registry entry** (`internal/server/tools.go:1374-1390`,
verified this session):
```go
func effectiveSearchScope(scope string, crossSpine bool) (string, error) {
	if crossSpine {
		return "", nil
	}
	if scope == "" {
		return "", argErrf(classMalformed, HintConditionalRequired, "scope", "scope is required unless cross_spine is true")
	}
	return scope, nil
}

// EffectiveSearchScope is the exported form of effectiveSearchScope. It
// exists solely so cmd/engram's client-side scope guard can be pinned
// against this rule at compile time, per Phase 7's D-03 amendment.
func EffectiveSearchScope(scope string, crossSpine bool) (string, error) {
	return effectiveSearchScope(scope, crossSpine)
}
```
This `exported-form-for-cross-package-pinning` shape is the precedent for any new rule value that
`cmd/engram` also needs to reference at compile time.

**Doc-comment discipline to preserve on any new rule's `Detail`/`Sentence`** (lines 64-69, quoted verbatim):
> Detail states the constraint and names the bound — it never interpolates the caller's rejected VALUE.

---

### `internal/toolclass/toolclass.go` (new leaf package, D-10/D-11's shared blast-radius table)

**Analog:** `internal/config/registry.go` — a small leaf package holding a declared table + lookup
function(s), already imported by both `internal/server`-adjacent code and `cmd/engram`.

**Pattern to copy** (`internal/config/registry.go:144-149`, quoted verbatim):
```go
// FlagDefault returns the registry default for the field bound to flag name, so
// cobra flag registration shows accurate --help defaults without duplicating
// literals. Returns "" when the flag is unknown or its field has no default.
func FlagDefault(flagName string) string {
	return flagToDefault[flagName]
}
```
`flagToDefault` is built once from a package-level `registry []field` literal — copy this "declared
slice, built-once derived map, exported lookup function" shape exactly for `[]ToolClass{Name, ReadOnly,
Destructive, Idempotent, OpenWorld}`.

**Placement is load-bearing — confirmed this session, not merely cited:**
`cmd/engram/client_common_test.go`'s `TestClientFilesImportBoundary` denylists production
`client_*.go`-prefixed files from importing `internal/server` (comment at line 402-404, verified this
session: *"TestClientFilesImportBoundary's file walk skips any file whose name ends in `_test.go`...
only production client_*.go files are denylisted from internal/server"*). `catalog.go` is **not**
`client_*`-prefixed, so option 2 (server exports the table, `catalog.go` imports `internal/server`
directly) would likely pass that specific test — but RESEARCH.md's Q7 recommendation (a new leaf
package) is still lower-risk and matches this analog's shape. Use `internal/toolclass` as a dependency
of both `internal/server/toolannotations.go` and `cmd/engram/catalog.go`; zero cycle risk.

---

### `internal/server/toolannotations.go` + `internal/server/toolannotations_test.go` (D-09/D-10)

**Analog for the table:** `cmd/engram/catalog.go`'s `buildCatalog` (derive-never-declare) — see below.

**Analog for the both-directions gate — copy this shape verbatim** (`cmd/engram/catalog_test.go:338-356`,
verified this session):
```go
func TestCatalogExitCodesMatchMapper(t *testing.T) {
	doc := decodeCatalog(t)

	catalogCodes := make(map[int]bool)
	for _, ec := range doc.ExitCodes {
		catalogCodes[ec.Code] = true
	}

	mapperCodes := map[int]bool{exitOK: true}
	for i := 1; i <= 16; i++ {
		mapperCodes[exitCodeForConnectErr(connect.NewError(connect.Code(i), errors.New("boom")))] = true
	}
	mapperCodes[exitCodeForConnectErr(errors.New("not a connect error"))] = true

	if !reflect.DeepEqual(catalogCodes, mapperCodes) {
		t.Errorf("catalog exit codes = {%s}, mapper-producible exit codes = {%s}",
			sortedIntKeys(catalogCodes), sortedIntKeys(mapperCodes))
	}
}
```
For D-10, replace `catalogCodes`/`mapperCodes` with `map[string]bool` keyed by tool name: one map built
from `toolClassTable`, one map built from the REAL registered tool set obtained via the
`registerTools(s, &deps{})` + `mcp.NewInMemoryTransports()` + `ListTools` RPC round trip (RESEARCH.md
Q2c/Code Examples — no existing precedent for the in-memory-transport enumeration itself, this is the
one genuinely new mechanical piece; the map-equality assertion around it is the copied part).

**`*bool` vs `bool` field-shape trap (compile-time, not a style choice):** `go-sdk@v1.7.0`'s
`mcp.ToolAnnotations.DestructiveHint`/`OpenWorldHint` are `*bool` (nil = spec default `true`);
`ReadOnlyHint`/`IdempotentHint` are bare `bool` (zero value = `false`). A bare
`&mcp.ToolAnnotations{OpenWorldHint: false}` will not compile. Use a package-level `var falseVal = false`
(or a generic `ptr[T any](v T) *T`) and take its address consistently in the table.

**Registration seam to extract from `Register()`** (`internal/server/tools.go:1772-1783`, verified this
session — the exact insertion point for annotations and the exact reason a test cannot enumerate tools
today):
```go
func Register(s *mcp.Server, mux *http.ServeMux, tm *telemetry.ToolMetrics, sqm *telemetry.SummaryQueueMetrics, uqm *telemetry.UsageQueueMetrics, resolve connectResolver, csrfVerify func(owner, token string) bool, reseal resealFunc) (shutdown func(context.Context), err error) {
	d, err := buildDepsFromEnv(sqm, uqm)
	// ... all 15 mcp.AddTool(s, &mcp.Tool{Name: "...", Description: "..."}, ...) calls inline here
```
Extract `func registerTools(s *mcp.Server, d *deps) error` containing the 15 `mcp.AddTool` calls, called
from `Register` after `buildDepsFromEnv` succeeds. This mirrors `argattribution_test.go`'s existing
pattern of calling `deps` methods directly against a bare `&deps{}` literal without live Qdrant/embedder
config (`argattribution_test.go` full file, verified this session — every case constructs args and calls
a `deps` method directly, never through `Register`).

**One AddTool site, verbatim, as the shape every annotation attaches to** (`tools.go:1996`):
```go
mcp.AddTool(s, &mcp.Tool{Name: "supersede_memory", Description: "Correct a memory you own by superseding it: stores a new record and marks the target superseded_by the new one. The target is soft-hidden from search_memory/list_memory but remains fetchable via get_memory — history is preserved, nothing is deleted or overwritten. Rejects if the target is already superseded (single live head per chain). The target id may be the full UUID or short_id."},
```
Per D-09, this tool's `DestructiveHint` must be `false` (ptr) — additive by design under every valid
input, the conservative-stance worked example named in CONTEXT.md.

---

### `internal/server/surfaces_test.go` + `cmd/engram/surfaces_test.go` (D-05, six-surface conformance gate)

**Analog for assertion discipline — copy verbatim style, not code** (`internal/server/argattribution_test.go:16-23`, quoted):
> Every assertion is on the FIELD IDENTIFIER (argFieldsOf, compared for full-set EQUALITY, never
> membership) and the HINT CODE — never on message wording.

The D-05 gate inverts this ONE way (per RESEARCH.md's Pattern discussion): it must compare rule-sentence
TEXT across surfaces (that's the whole point), but it should still assert PRESENCE of the declared rule's
canonical sentence as a substring, never a hand-typed re-statement of the sentence in the test itself —
the sentence under test is D-03's single declared const, referenced not retyped.

**Analog for schema-reflection style (not wire-snapshot)** — `internal/server/connectdescriptor_test.go`'s
`assertFields` helper (lines 14-60, verified this session): asserts field name/kind/cardinality/message-type
by walking `protoreflect.FileDescriptor` directly, never diffing rendered/serialized bytes. Copy this
"walk the descriptor, assert on structured fields" shape for surface (d) (proto comments), but note the
descriptor type differs: proto *comments* require `descriptorpb.FileDescriptorSet` (obtained by shelling
to `go tool buf build --as-file-descriptor-set`, since `protoc-gen-go`'s embedded descriptor in
`engramv1.File_engram_v1_engram_proto` strips `SourceCodeInfo`), not the already-imported
`protoreflect.FileDescriptor` this existing test uses for field-shape assertions.

**Struct-tag text extraction (surface b) — no existing precedent, illustrative only:**
```go
t := reflect.TypeOf(searchArgs{})
f, _ := t.FieldByName("Scope")
tagText := f.Tag.Get("jsonschema") // "required unless cross_spine"
```
Concrete jsonschema tags to read (`internal/server/tools.go:598,609`, verified this session):
```go
Scope string `json:"scope,omitempty" jsonschema:"required unless cross_spine"`   // searchArgs
Scope string `json:"scope,omitempty" jsonschema:"required unless cross_spine"`   // listArgs
```

**D-02's carve-out — the one site the gate must explicitly exclude:** `tools.go:546`'s `not_after must
be in the future` (`HintOrdering`, single-field, clock-dependent) must not be swept in by a bare
`hint == HintOrdering` check; implement as a named, commented allowlist-of-one keyed by rule ID, not a
heuristic (RESEARCH.md Pitfall 2).

---

### `cmd/engram/testdata/help.golden` + `TestHelpGolden` (D-12)

**No analog found.** No golden/testdata directory or golden-file test pattern exists anywhere in this
repo today — `cmd/engram/testdata/` does not yet exist (RESEARCH.md's Recommended Project Structure lists
it as new). Build from RESEARCH.md's Code Examples/Pitfall 5 guidance directly: walk the live cobra tree
in deterministic order (mirroring `buildCatalog`'s traversal shape from `catalog.go:53-77`, verified this
session — reuse that traversal rather than writing a second one), capture each command's `--help` output
via `cmd.SetOut`/`cmd.Help()`, and compare against a committed golden. Construct the root command with an
explicit test-only `Version: "test"` (not the ldflags-injected value) to avoid coupling the golden to
release tags.

---

### `cmd/engram/testdata/catalog.golden` + `TestCatalogGolden` (D-13)

**Analog:** `cmd/engram/catalog.go` + `catalog_test.go`'s existing `decodeCatalog` helper (referenced at
`catalog_test.go:339`, reused by `TestCatalogExitCodesMatchMapper`) — reuse this decode path to build the
document under test, then serialize and diff against a committed golden rather than inventing a second
JSON decode helper. Same `Version: "test"` fix as `TestHelpGolden` applies here (`catalogDoc.Version` is
set from `root.Version`, itself ldflags-injected `main.version` — `catalog.go:56`, verified this session).

---

### `internal/surfacesgen/` (new generator) + `Taskfile.yaml` `surfaces:gen` target + CI drift job

**Reference implementation — copy this shape byte-for-byte, per CONTEXT.md D-06/D-07's explicit
citation.** `Taskfile.yaml`'s `proto:gen` target:
```yaml
proto:gen:
  desc: Regenerate connect stubs (Go + TS) and re-vendor the console gen client tree
  cmds:
    - go tool buf generate
    - rm -rf ui/src/lib/gen/engram ui/src/lib/gen/buf
    - cp -R gen/ts/. ui/src/lib/gen/
```
`.github/workflows/ci.yaml`'s `buf` job "generated-code drift" step:
```yaml
- name: generated-code drift
  run: |
    go tool buf generate
    git diff --exit-code -- gen/ || (echo "gen/ is stale; run 'task proto:gen'"; exit 1)
```
`task surfaces:gen` should regenerate anchored regions + both goldens in place; the new `surfaces` CI
job (or an added step in the existing `buf` job) re-runs it then `git diff --exit-code` over
`docs-site/`, `skill/engram/`, `proto/`, `cmd/engram/testdata/`. No CI write-back — this mirrors the
existing job exactly (verified this session: no `git push`/commit step in the `buf` job).

## Shared Patterns

### Derive-never-declare
**Source:** `cmd/engram/catalog.go:49-52` doc comment, quoted verbatim:
> `buildCatalog` walks the live cobra tree ... and derives a catalogDoc from it — never from a
> hand-maintained literal — so a command or flag added later appears here with no edit, and cannot
> silently go missing.

**Apply to:** the rule registry (D-03), the D-08 surface-applicability normalizer, D-11's per-command
blast radius, D-12's golden walker — every one of these must compute from a single declared source at
generation/test time, never hand-duplicate.

### Both-directions gate
**Source:** `cmd/engram/catalog_test.go:338-356` (`TestCatalogExitCodesMatchMapper`, full excerpt above).
**Apply to:** `toolannotations_test.go` (D-10: every registered tool has a table entry, every table entry
names a registered tool) and, if useful, the D-08 normalizer's own "every rule resolves to a non-empty
applicable-surface set" test.

### Assert on identifiers/codes, never on message wording (with the one documented exception)
**Source:** `internal/server/argattribution_test.go:16-23`, quoted above.
**Apply to:** every conformance-gate assertion EXCEPT the six-surface text-presence check itself, which
must compare canonical-sentence text by construction (that is the requirement) — but even there, assert
against the single declared const, never a second hand-typed literal in the test.

### Generate-commit-drift-check
**Source:** `Taskfile.yaml`'s `proto:gen` target + `.github/workflows/ci.yaml`'s `buf` job, both quoted
above.
**Apply to:** `task surfaces:gen` (D-06/D-07) and its CI job — the single reference implementation named
explicitly in CONTEXT.md, copy the shape rather than inventing a new one.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `cmd/engram/testdata/help.golden`, `TestHelpGolden` | test (golden-file) | file-I/O | No golden/testdata directory or golden-file test pattern exists anywhere in this repo today — confirmed by this session's file reads; this is a greenfield mechanism, build from RESEARCH.md's Code Examples/Pitfalls rather than an in-repo analog. |
| `docs-site/src/content/docs/reference/tools.md`, `guides/cli.md` anchored regions | docs/content generator target | transform (generated text region) | No Go program in this repo rewrites markdown today. The `gen/` tree's generate-commit-drift-check *CI contract* is the correct analog for the surrounding task/CI shape (see Shared Patterns), but the actual anchor-rewrite mechanism (sentinel-line find/replace inside prose) has zero in-repo precedent — RESEARCH.md recommends plain `bufio.Scanner` line-matching, explicitly rejecting a markdown AST library as disproportionate for a two-sentinel-line region swap. |
| `skill/engram/skills/{curating-memory,discovering}/SKILL.md` anchored regions | docs/content generator target | transform (generated text region) | Same reasoning as the docs-site row — greenfield anchor mechanism, same recommended `bufio.Scanner` approach, same sentinel syntax (`<!-- GSD:ANCHOR:START <rule-id> -->` per RESEARCH.md Q4) for a single generator implementation shared across all four prose files. |

## Metadata

**Analog search scope:** `internal/server/*.go` (argerror.go, tools.go, connectapi.go, connecterror.go,
summary.go, argattribution_test.go, connectdescriptor_test.go, tools_test.go), `cmd/engram/*.go`
(catalog.go, catalog_test.go, client_common.go, client_common_test.go, client_list.go, client_search.go),
`internal/config/registry.go`, `Taskfile.yaml`, `.github/workflows/ci.yaml`.
**Files scanned:** ~15 read directly this session plus the full RESEARCH.md/CONTEXT.md corpus (both
already contain extensive verified excerpts from this session's earlier research pass, reused here
rather than re-read).
**Pattern extraction date:** 2026-08-04
