# Phase 1: Gate & CI Integrity - Pattern Map

**Mapped:** 2026-08-13
**Files analyzed:** 8 (new/modified; excludes the 38 mechanical `.planning/**` pattern rewrites,
which are data edits, not code, and have no code analog)
**Analogs found:** 8 / 8

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/keylinks/keylinks.go` | utility (conformance-gate core) | transform (parse + validate) | `internal/surfaces/surfaces.go` + `internal/openaiurl/openaiurl.go` | role-match (leaf stdlib package) |
| `internal/keylinks/keylinks_test.go` | test (repo-wide scan gate) | batch | `internal/surfaces/conformance_test.go` | exact (same shape: walk repo, collect ALL violations, `t.Error` per offender) |
| `internal/keylinks/testdata/good_key_links.md` / `bad_key_links.md` | test fixture | file-I/O | `internal/surfaces/conformance_test.go`'s inline fixture (`TestSurfaceConformanceDeterministic`, `t.TempDir()`+`os.WriteFile`) | role-match (fixture proves fail-first; existing analog builds fixture inline rather than as a committed file, so pattern is adapted not copied verbatim) |
| One-time v0.13.x sweep script/test (D-12; output → this phase's `VERIFICATION.md`) | test (one-off, reuses `internal/keylinks` matcher) | batch | `internal/keylinks/keylinks_test.go` itself (self-referential — no separate analog needed) | role-match |
| `internal/store/store_test.go` (collection-name edit only, D-16) | test (integration setup) | CRUD | itself — edit in place | exact |
| `internal/server/tools_test.go` (collection-name edit only, D-16) | test (integration setup) | CRUD | `internal/store/store_test.go`'s `TestMain`/`requireQdrant`/`terminateQdrant` (already the sibling this file mirrors, per its own comments) | exact |
| `internal/store/reindex_test.go` (collection-name edit only, D-16) | test (integration setup) | CRUD | same file — mechanical prefix rewrite of existing `const src, tgt = "..."` literals | exact |
| `.github/workflows/ci.yaml` `test` job | config (CI workflow) | event-driven | itself — edit in place (`test` job, lines 23-56) | exact |

## Pattern Assignments

### `internal/keylinks/keylinks.go` (utility, transform)

**Analogs:** `internal/surfaces/surfaces.go` (leaf-package shape, no store/authz deps, pure
functions returning data + error) and `internal/openaiurl/openaiurl.go` (minimal stdlib-only leaf,
doc comment convention).

**Package doc comment pattern** (`internal/openaiurl/openaiurl.go` lines 4-10):
```go
// Package openaiurl builds OpenAI-compatible endpoint URLs from an operator
// base URL and a request-path suffix (e.g. "embeddings", "chat/completions").
// It is the single shared implementation of the shape-aware provider-endpoint
// join used by both internal/embed and internal/summarize (D-14) — a stdlib-
// only leaf package deliberately so either lane can import it with zero cycle
// risk.
package openaiurl
```
Apply the same shape to `internal/keylinks`: package doc names what it does, which callers use it
(the guard's own test + the one-time sweep), and that it is a leaf (imports nothing repo-internal).

**Core transform pattern — RESEARCH.md's own composed example is the closest available "analog"
since no prior repo file does regex-dialect validation; it already follows this repo's error-value
(not panic) convention seen in `surfaces.go`'s `ProtoFieldComments`/`BufDescriptorSet`:**
```go
// internal/surfaces/surfaces.go lines 45-53 — the repo convention this
// package must match: wrapped errors via fmt.Errorf("%w", ...), package-
// prefixed error strings ("surfaces: ..."), no panics.
func ProtoFieldComments(fdsPath string) (map[string]string, error) {
	data, err := os.ReadFile(fdsPath)
	if err != nil {
		return nil, fmt.Errorf("surfaces: read %s: %w", fdsPath, err)
	}
	...
}
```
`internal/keylinks` should follow the identical error convention: `fmt.Errorf("keylinks: %w", err)`,
package-prefixed messages, functions returning `(T, error)` — never `log.Fatal`/panic inside the
leaf package (that belongs only in the `_test.go` caller, mirroring `surfaces.go`'s split between
pure logic and `conformance_test.go`'s `t.Fatalf`/`t.Error` calls).

**Deterministic-violation-formatting pattern** (`internal/surfaces/conformance_test.go` lines 96-103):
```go
// violationLine formats one gate divergence deterministically: rule ID,
// surface, the expected canonical sentence, and what was actually found.
func violationLine(ruleID string, surface Surface, expected, found string) string {
	return fmt.Sprintf("rule=%s surface=%s expected=%q found=%q", ruleID, surface, expected, found)
}
```
D-07 requires every offender reported in one run with `file:line`, shape, and corrected form — copy
this exact "one small deterministic formatter, called from a loop that appends rather than
returns-on-first" shape. E.g. `func offenderLine(file string, line int, shape, raw, fix string) string`.

### `internal/keylinks/keylinks_test.go` (test, batch)

**Analog:** `internal/surfaces/conformance_test.go` — this is the single strongest analog in the
repo for "a Go test that walks real repo files and asserts zero violations, reporting every one."

**Collect-all-violations, never fail-fast pattern** (`conformance_test.go` lines 175-230):
```go
func runGate(t *testing.T) []string {
	t.Helper()
	var violations []string
	for _, rule := range Rules() {
		...
		violations = append(violations, checkProseSurface(paths, rule, surface)...)
	}
	return violations
}

func TestSurfaceConformanceProseFiles(t *testing.T) {
	for _, v := range runGate(t) {
		t.Error(v)
	}
}
```
`internal/keylinks_test.go`'s `TestNoOffendingPatterns` (escaping, ALL plans, D-04) and
`TestActiveMilestoneSatisfiable` (satisfiability, active milestone only, D-04) should follow this
exact shape: a `run*(t)` helper returning `[]string`, the actual `Test*` func doing
`for _, v := range run*(t) { t.Error(v) }` — never `t.Fatal` on first offender (D-07).

**Deterministic-output proof pattern** (`conformance_test.go` lines 232-259,
`TestSurfaceConformanceDeterministic`): write a corrupted fixture to `t.TempDir()`, run the check
function twice, assert `reflect.DeepEqual(first, second)`. D-06's fail-first proof (good fixture →
GREEN, bad fixture → RED) is a variant of this same pattern but uses **committed** `testdata/`
files instead of an inline `t.TempDir()` fixture (per RESEARCH.md's recommended project structure) —
adapt the write-then-assert structure, not the temp-file mechanics.

**Zero-applicability / worst-failure-mode proof pattern** (`conformance_test.go` lines 261-278,
`TestZeroApplicableSurfacesFailsGate`): constructs a synthetic "ghost" input designed to trigger the
gate's own most dangerous failure mode (silently passing) and asserts it does NOT pass silently.
Directly applicable to D-03's unsatisfiable-pattern shape: construct a synthetic pattern that
compiles cleanly, is escape-free, but cannot match its `from:` file, and assert the satisfiability
checker flags it — this is the regression test for #479's second finding.

### `internal/keylinks/testdata/{good,bad}_key_links.md` (test fixture, file-I/O)

**Analog pattern** (adapted, not copied verbatim, from `conformance_test.go` lines 242-248):
```go
dir := t.TempDir()
path := filepath.Join(dir, "fixture.md")
corrupted := "before\n<!-- engram:rule:start " + rule.ID + " -->WRONG TEXT<!-- engram:rule:end " + rule.ID + " -->\nafter\n"
if err := os.WriteFile(path, []byte(corrupted), 0o600); err != nil {
	t.Fatalf("write fixture: %v", err)
}
```
D-06 requires these fixtures to be **committed** under `internal/keylinks/testdata/`, not generated
inline at test time — RESEARCH.md's recommended structure already specifies this. The fixture
content should mirror a real `key_links:` frontmatter block shape (see CONTEXT.md D-02/D-03: one
`pattern:` using the character-class form `[.]`/`[(]`/`[)]` for the good fixture, one using `\\.`
for the bad-escaping case, and — per D-03 — a second bad case with a correctly-escaped-but-never-
matching pattern for the unsatisfiable shape).

### `internal/store/store_test.go`, `internal/server/tools_test.go`, `internal/store/reindex_test.go` (test, CRUD — D-16 collection-name edits only)

**Analog:** each other — this is a same-file mechanical edit, not a new pattern.

**Current colliding names to rewrite** (verified via grep this session):
- `internal/store/store_test.go:249` — `s := New(dialTestClient(t), "mem_eval_test")` → prefix e.g.
  `store_mem_eval_test`.
- `internal/server/tools_test.go:321` — `st := store.New(c, "mem_eval_test")` → prefix e.g.
  `server_mem_eval_test`.
- `internal/server/tools_test.go:5286` — `t.Setenv("ENGRAM_QDRANT_COLLECTION", "mem_load_once_test")`
  → same `server_` prefix.
- `internal/store/reindex_test.go` — every `const src, tgt = "reindex_..."` literal (7 pairs found:
  `reindex_src`/`reindex_tgt`, `reindex_srcov_real`/`reindex_srcov_env`/`reindex_srcov_tgt`,
  `reindex_prog_src`/`reindex_prog_tgt`, `reindex_resume_src`/`reindex_resume_tgt`,
  `reindex_resume_tags_src`/`reindex_resume_tags_tgt`, `reindex_dry_src`/`reindex_dry_tgt`,
  `reindex_missing_src`/`reindex_missing_tgt`, `reindex_page_src`/`reindex_page_tgt`,
  `reindex_partial_src`/`reindex_partial_tgt`, `reindex_identity_src`/`reindex_identity_tgt`/
  `reindex_identity_tgt_none`, `reindex_restamp_src`/`reindex_restamp_tgt`,
  `reindex_fail_src`/`reindex_fail_tgt`, `reindex_dryresume_src`/`reindex_dryresume_tgt`,
  `reindex_dryresume_missing_tgt`) — since `internal/store` is one package, RESEARCH.md's
  recommendation is to prefix uniformly with `store_` even though `reindex_` already
  disambiguates within the package (consistency over minimal diff, per RESEARCH's Open Question 2).

**Deliberately NOT touched** (Pitfall 3, verified this session): `internal/e2e/harness_test.go:257`
(`"e2e_" + strconv.FormatInt(...)`) and `internal/retrievaleval/retrieval_eval_test.go:299`
(`"retrievaleval_" + uuid.NewString()`) are already collision-safe — zero Go-side edits, only the
CI env-var wiring applies to them.

**TestMain precedence chain — NO code change needed, already correct** (`internal/store/store_test.go`
lines 68-113, the analog `internal/server/tools_test.go` explicitly mirrors per its own comment at
line 127 "TestMain and [dialTestClient] act only on [requireQdrant]'s result, never parsing the env
var themselves"):
```go
func TestMain(m *testing.M) {
	required, rerr := requireQdrant()
	if rerr != nil { ... os.Exit(1) }
	if addr := os.Getenv("ENGRAM_QDRANT_TEST_ADDR"); addr != "" {
		testQdrantAddr = addr
		os.Exit(m.Run())
	}
	// ...testcontainer fallback...
}
```
D-15/D-18 rely on this **exact existing precedence** (env var → testcontainer → skip) needing zero
Go-side plumbing changes — only CI setting `ENGRAM_QDRANT_TEST_ADDR` is new.

### `.github/workflows/ci.yaml` `test` job (config, event-driven)

**Analog:** itself, lines 23-56 (current job to extend with `services:` + diagnostics post-step).

**Current job shape to extend** (`.github/workflows/ci.yaml` lines 23-40):
```yaml
test:
  name: test
  if: ${{ !startsWith(github.head_ref, 'release-please--') }}
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@...
    - uses: actions/setup-go@...
      with:
        go-version-file: go.mod
    - name: Test
      env:
        ENGRAM_REQUIRE_QDRANT: "1"
      run: go test ./...
```
D-15/D-19 require adding a `services:` block at the job level (not inside `steps:`) and a new
`env:` entry `ENGRAM_QDRANT_TEST_ADDR: localhost:6334` alongside the existing
`ENGRAM_REQUIRE_QDRANT: "1"`, plus a new `if: failure()` step appended after the existing steps.
RESEARCH.md's Code Examples section has the exact `services:` YAML shape to insert (image tag must
match `qdrantImageTag = "qdrant/qdrant:v1.18.2"` at `store_test.go:33`, health-checked against
`/readyz` per this repo's own `charts/engram/templates/qdrant.yaml:67-68` precedent).

**Existing injection-avoidance precedent to mirror for the D-19 diagnostics step**
(`.github/workflows/ci.yaml:252-260`, the `ui-drift` job — read this range before writing the
diagnostics step; not re-quoted here since it was not re-read this session beyond the grep-located
reference in RESEARCH.md's Security Domain section): passes `${{ }}` expression values through
`env:` rather than inline shell interpolation. The D-19 diagnostics step touches only
`docker ps`/`docker logs`/`dmesg` output (no PR-controlled content), so this precedent is
precautionary, not strictly required, but should be followed for consistency.

## Shared Patterns

### Leaf-package convention (applies to `internal/keylinks/keylinks.go`)
**Source:** `internal/surfaces/`, `internal/openaiurl/` — both stdlib-only (or, for `surfaces`,
only `google.golang.org/protobuf` — no repo-internal imports), no `internal/store`/`internal/authz`
dependency, imported by heavier packages, never the reverse.
**Apply to:** `internal/keylinks/keylinks.go` — must import only stdlib (`regexp`, `os`,
`path/filepath`, `strings`, `fmt`) per D-05's "stdlib-only leaf" requirement; zero repo-internal
imports.

### Table-driven stdlib-only testing (applies to `internal/keylinks/keylinks_test.go`)
**Source:** confirmed repo-wide zero `stretchr/testify` imports (RESEARCH.md, verified this
session); `internal/surfaces/conformance_test.go`'s plain `testing` + `t.Run` subtests +
`reflect.DeepEqual` for structural comparison.
**Apply to:** `internal/keylinks/keylinks_test.go` and the sweep test — no `testify` assertions,
plain `if got != want { t.Errorf(...) }`.

### Error-value convention, never panic (applies to `internal/keylinks/keylinks.go`)
**Source:** `internal/surfaces/surfaces.go`'s `ProtoFieldComments`/`BufDescriptorSet` — every
failure returns `(T, error)` with `fmt.Errorf("surfaces: ...: %w", err)`-style wrapped, package-
prefixed errors; the `_test.go` caller is the only place that calls `t.Fatalf`.
**Apply to:** all `internal/keylinks` production code (not its test file).

### TestMain env-var-first precedence chain (applies to no NEW code — governs what NOT to touch)
**Source:** `internal/store/store_test.go` lines 75-113, explicitly mirrored by
`internal/server/tools_test.go` (its own comment names `store_test.go` as the pattern it follows).
**Apply to:** D-15/D-17's shared-instance wiring requires zero changes to this chain in any of the
four packages — confirm this during planning rather than re-deriving the chain per package.

## No Analog Found

None. Every file in scope has a strong same-repo analog; RESEARCH.md's Code Examples section
supplies the one genuinely novel piece (RE2-vs-JS regex dialect validation), which has no repo
precedent but is fully specified there with empirically-verified behavior.

## Metadata

**Analog search scope:** `internal/surfaces/`, `internal/openaiurl/`, `internal/store/`,
`internal/server/`, `internal/e2e/`, `internal/retrievaleval/`, `.github/workflows/ci.yaml`
**Files scanned:** 11 Go files (full or targeted reads) + 1 CI workflow file + 2 CONTEXT/RESEARCH
docs
**Pattern extraction date:** 2026-08-13
