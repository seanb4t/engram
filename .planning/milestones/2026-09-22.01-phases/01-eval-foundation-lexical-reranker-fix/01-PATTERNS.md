# Phase 1: Eval Foundation & Lexical Reranker Fix - Pattern Map

**Mapped:** 2026-09-22
**Files analyzed:** 9 (2 new, 7 modified)
**Analogs found:** 8 / 9

> **D-15 note (binding on this map):** RESEARCH.md's own body text (pre-revision) recommends
> registering `ENGRAM_RETRIEVAL_EVAL` in `internal/config`'s registry. **That recommendation is
> superseded.** CONTEXT.md D-15 (user-confirmed 2026-09-22) says do NOT register it — resolve it
> with a test-local koanf load instead, same `ENGRAM_` prefix/precedence, no `Validate()`. Every
> pattern assignment below for the config-gate file follows the revised D-15, not the research
> body's registry-entry excerpt.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/retrievaleval/fixtures.go` | test-fixture (model-like) | CRUD (fixture data + pure metric helpers) | itself (existing file, edited in place) | exact — extend existing shape |
| `internal/retrievaleval/retrieval_eval_test.go` | test (integration, gated) | request-response (search) + event-driven (gate) | itself (existing file, edited in place) | exact — extend existing shape |
| `internal/retrievaleval/differ_test.go` (new, or inline in retrieval_eval_test.go) | test (unit, hermetic) | transform (pure function) | `internal/store/rerank_test.go` | role-match — hermetic pure-function unit tests in a sibling package |
| `internal/store/rerank.go` | service (pure ranking library) | transform | itself (existing file, edited in place — add `CosineBlendRerank`, `OverlapGateRerank`, export `CandidateK`) | exact — extend existing shape |
| `internal/store/rerank_test.go` | test (unit, hermetic) | transform | itself (existing file, edited in place) | exact — extend existing shape |
| `internal/store/store.go` (`SearchReranked` + doc comment) | service (shared seam) | request-response | itself (existing file, edited in place) | exact — body/doc-comment edit only, signature unchanged |
| `internal/server/tools.go` (`StoreAndEmbedderFromEnvNoEnsure`) | service (config/embedder builder) | request-response | itself (existing file, edited in place — widen return tuple) | exact — extend existing shape |
| retrieval-eval config gate (test-local koanf load, D-15 revised) | config (test-local, NOT registry) | request-response (one-shot resolve) | `internal/config/config.go`'s `Load` (env.Provider mechanics) — **NOT** `internal/config/registry.go` (excluded by D-15) | role-match, mechanics-only — no exact precedent for a registry-bypassing koanf load exists in this repo |
| `internal/config/config_test.go` | test (unit, hermetic) | transform | itself (existing file — `TestOwnerClaimDefaultAndOverride`, unrelated to D-15's gate since the gate is NOT a `Config` field) | **only if** Claude's discretion picks a `Config`-struct-adjacent shape for the test-local loader's target type; otherwise no analog needed here |

## Pattern Assignments

### `internal/retrievaleval/fixtures.go` (test-fixture, extend in place)

**Analog:** itself — `gh261Case`/`retrievalCase`/`retrievalQuery` at lines 17-131 (already read in full this session).

**Current shape to change** (lines 17-30):
```go
type retrievalQuery struct {
	name string
	text string
}
type retrievalCase struct {
	name        string
	seedRecords []seedRecord
	queries     []retrievalQuery
	wantKey     string
}
```

**Target shape** (RESEARCH.md "Fixture shape change" — move `wantKey` to query level, empty = no-answer, D-12):
```go
type retrievalQuery struct {
	name    string
	text    string
	wantKey string // "" = no-answer query (D-12): excluded from recall@k/MRR, logged only
}

type retrievalCase struct {
	name        string
	seedRecords []seedRecord
	queries     []retrievalQuery
	// wantKey removed from this level — delete it entirely (Pitfall 3): do NOT
	// keep both fields "for compat" — the compiler must force every literal
	// (gh261Case included) to move onto the new shape.
}
```

**`gh261Case` literal update** (mechanical, behavior-preserving — D-04): each of its two
`queries` entries gains `wantKey: recordTKey`, and the case-level `wantKey: recordTKey` line
(line 81) is deleted.

**New paraphrase case pattern to add:** a second `retrievalCase` entry appended to
`retrievalCases` (line 85, currently `[]retrievalCase{gh261Case}`), following `gh261Case`'s own
shape — `seedRecords` (80-120 synthetic multi-domain records per D-02), `queries` (~20
single-answer + a few no-answer, each `wantKey: ""` for no-answer per D-12). Follow
`gh261Case`'s existing comment convention (a doc comment above the var, citing the relevant
decision IDs) and, per D-01, record the blind-subagent authorship procedure and date directly
in that comment — this is new content, not a pattern copy, but the *comment-citing-decision-IDs*
convention itself is directly copyable from `gh261Case`'s own doc comment (lines 61-67) and
`differProbe`'s (lines 87-108).

**Reusable as-is, no changes:** `recallAtK` (lines 112-119), `reciprocalRank` (lines 123-130) —
both are already generic pure `([]string, string) -> bool/float64` helpers; the new
`variantMetrics` accumulator (see rerank/eval pattern below) should call these, not reimplement
them.

---

### `internal/retrievaleval/retrieval_eval_test.go` (test, integration/gated, extend in place)

**Analog:** itself — `TestRetrievalEval` (lines 64-218), `TestRetrievalEval_AsymmetryDiffer`
(lines 238-293), `TestMain` (lines 322-327).

**Imports pattern** (lines 6-19) — this file's own convention; new code (cosine distance,
NaN/Inf checks) needs `math` added to this exact import block:
```go
import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)
```
`reflect` is dropped once `reflect.DeepEqual` (EVAL-01 fix) is replaced by `cosineDistance`;
`math` is added for the new distance function.

**Differ gate — current bug (EVAL-01 / D-13), lines 284-292:**
```go
if reflect.DeepEqual(queryVec, documentVec) {
	t.Fatalf("asymmetry differ FAIL: query vector == document vector (dim=%d) — the asymmetric instruction-prefix had no effect; the operator likely wired the no-op ENGRAM_EMBED_QUERY_PARAMS/ENGRAM_EMBED_DOCUMENT_PARAMS/task_type mechanism instead of ENGRAM_EMBED_QUERY_INSTRUCTION/ENGRAM_EMBED_DOCUMENT_INSTRUCTION", dim)
}

t.Logf("asymmetry differ PASS: query vector != document vector (dim=%d) — instruction-prefix took effect", dim)
```
**Target shape** (D-13 — cosine distance > 1e-3, NaN/Inf hard `t.Fatal` before the distance
compare, softened PASS log):
```go
for _, v := range [][]float32{queryVec, documentVec} {
	for _, x := range v {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			t.Fatalf("asymmetry differ: NaN/Inf component in embedding vector (dim=%d) — embedder returned a malformed vector, not a comparable one", dim)
		}
	}
}
distance := cosineDistance(queryVec, documentVec)
if distance <= 1e-3 {
	t.Fatalf("asymmetry differ FAIL: cosine distance %.6f <= 1e-3 threshold — query and document vectors are not materially different", distance)
}
t.Logf("asymmetry differ PASS: vectors differ materially (cosine distance=%.6f)", distance)
```
Keep the existing dimension-contract check (lines 278-282) unchanged, exactly as D-13 specifies.

**Skip guard — current bug (EVAL-02 / D-14), lines 239-241 and 248-253** (both `TestRetrievalEval`
and `TestRetrievalEval_AsymmetryDiffer` read raw `os.Getenv`):
```go
if os.Getenv("ENGRAM_RETRIEVAL_EVAL") != "1" {
	t.Skip(...)
}
...
if os.Getenv("ENGRAM_EMBED_QUERY_INSTRUCTION") == "" &&
	os.Getenv("ENGRAM_EMBED_DOCUMENT_INSTRUCTION") == "" &&
	os.Getenv("ENGRAM_EMBED_QUERY_PARAMS") == "" &&
	os.Getenv("ENGRAM_EMBED_DOCUMENT_PARAMS") == "" {
	t.Skip(...)
}
```
**Target:** both gates read the resolved `*config.Config` returned by the widened
`StoreAndEmbedderFromEnvNoEnsure` (see that file's pattern block below) — `cfg.Embed.*` fields,
not `os.Getenv`. The retrieval-eval-enabled gate itself (`ENGRAM_RETRIEVAL_EVAL`) reads the
test-local koanf load (D-15 revised — see the dedicated pattern block below), NOT `cfg` (since
that var is deliberately excluded from the registry `cfg` is built from).

**`TestMain` gate** (lines 322-327) — same `os.Getenv("ENGRAM_RETRIEVAL_EVAL") != "1"` pattern,
same fix target (test-local koanf load), same "before any Docker/testcontainer startup" contract
that MUST be preserved (Pitfall 2 in RESEARCH.md — never call `Validate()` from this path).

**`wantKey` resolution loop-move** (EVAL-01 unrelated, RANK-01 fixture-shape follow-on), lines
120-123 and 176-190 — see RESEARCH.md "TestRetrievalEval's loop must move the wantKey lookup
inside the per-query loop" for the exact control-flow change; this is new logic, not a copy, but
it reuses this file's own existing `recallAtK`/`reciprocalRank`/rank-loop idiom (lines 156-192)
as the template to duplicate per-variant (see Pluggable ranker pattern below).

---

### `internal/retrievaleval/differ_test.go` (new — unit test for `cosineDistance`)

**Analog:** `internal/store/rerank_test.go` — table-driven hermetic pure-function test style
(`TestCandidateK`, lines 12-33).

**Imports pattern:**
```go
package retrievaleval

import (
	"math"
	"testing"
)
```

**Core pattern** (copy `TestCandidateK`'s table-driven shape, lines 12-33 of `rerank_test.go`):
```go
func TestCosineDistance(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		a, b []float32
		want float64
	}{
		{"identical vectors", []float32{1, 0}, []float32{1, 0}, 0},
		{"orthogonal vectors", []float32{1, 0}, []float32{0, 1}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cosineDistance(tc.a, tc.b); math.Abs(got-tc.want) > 1e-9 {
				t.Errorf("cosineDistance(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
```
NaN/Inf cases are exercised through `TestRetrievalEval_AsymmetryDiffer`'s own hard-`t.Fatal`
guard (see above), not a separate table case, since that check lives inline in the test function
per D-13, not inside `cosineDistance` itself (RESEARCH.md: "checked BEFORE the distance
computation").

---

### `internal/store/rerank.go` (service, extend in place)

**Analog:** itself — `candidateK`/`tokenize`/`lexicalOverlap`/`RerankHits` (full file, 100 lines,
already read this session).

**Imports pattern** (lines 6-9) — unchanged, no new import needed for the two new rankers
(pure `[]Memory` transforms, same `sort`/`strings` already imported):
```go
import (
	"sort"
	"strings"
)
```

**Core pattern to copy — pure-function-of-(query, hits, k) contract** (`RerankHits`, lines 58-100):
```go
func RerankHits(query string, hits []Memory, k int) []Memory {
	queryTerms := tokenize(query)
	type scored struct {
		m       Memory
		overlap int
	}
	ranked := make([]scored, len(hits))
	for i, h := range hits {
		ranked[i] = scored{m: h, overlap: lexicalOverlap(queryTerms, h)}
	}
	sort.SliceStable(ranked, func(i, j int) bool { /* ... */ })
	if k <= 0 || k >= len(ranked) {
		k = len(ranked)
	}
	out := make([]Memory, k)
	for i := 0; i < k; i++ {
		out[i] = ranked[i].m
	}
	return out
}
```
`CosineBlendRerank(query string, hits []Memory, k int, alpha float64) []Memory` and
`OverlapGateRerank(query string, hits []Memory, k int, threshold float64) []Memory` (D-06) copy
this exact contract shape — same signature pattern (query/hits/k first, tuning param last), same
`sort.SliceStable` + truncate-to-k tail, same determinism guarantee (tie-break on `Score` then
`ID`, per `RerankHits`'s own comment at lines 66-70). **Reuse `hit.Score` directly for the cosine
term** (Pitfall 4 — `Memory.Score` is already Qdrant's raw cosine similarity; do not recompute
from vectors, `Memory` carries none). Both new rankers reuse the existing unexported
`tokenize`/`lexicalOverlap` helpers (lines 27-56) unchanged — do not re-derive tokenization.

**`candidateK` export** (line 16): rename `candidateK` → `CandidateK` (capitalize only; body,
tests, and doc comment unchanged) so the eval can call it directly per RESEARCH.md's "Pluggable
ranker design" step 1. Update the one call site inside this same file (`SearchReranked` in
`store.go`) to the new name.

**Doc-comment-citing-decision-IDs convention** (already this file's own style, lines 11-15,
58-64, 66-70): follow it for the two new functions — a doc comment naming the GH issue/decision
ID the function exists for and its determinism/purity contract, matching `RerankHits`'s own
comment shape verbatim.

---

### `internal/store/rerank_test.go` (test, extend in place)

**Analog:** itself — `TestCandidateK` (lines 12-33), `TestRerankHitsPromotesLexicalOverlap`
(lines 51-63), `TestRerankHitsDeterministic` (lines 65-80), `TestRerankHitsIgnoresAccessCount`
(lines 104-136).

**Imports pattern** (lines 6-10) — unchanged:
```go
import (
	"context"
	"errors"
	"testing"
)
```

**Core pattern to copy for `TestCosineBlendRerank`/`TestOverlapGateRerank`** — hand-built
`[]Memory` literals, hermetic, no Qdrant (`TestRerankHitsPromotesLexicalOverlap`, lines 51-63):
```go
func TestRerankHitsPromotesLexicalOverlap(t *testing.T) {
	t.Parallel()
	hits := []Memory{
		{ID: "topical-neighbor", Content: "...", Score: 0.91},
		{ID: "high-overlap", Content: "...", Tags: []string{"lint", "task"}, Score: 0.80},
	}
	query := "..."
	got := RerankHits(query, hits, 2)
	if len(got) != 2 || got[0].ID != "high-overlap" {
		t.Fatalf(...)
	}
}
```
Also copy the negative-space `TestRerankHitsIgnoresAccessCount` pattern (lines 104-136) for both
new rankers — same "identical-except-one-field, wildly divergent order" construction proving
`AccessCount` never leaks into ranking, and the determinism pattern (`TestRerankHitsDeterministic`,
lines 65-80) proving repeated calls on identical input tie-break identically. `idsOf` (lines
138-145) is a reusable helper already in this file — call it, don't redefine it.

`TestCandidateK`'s table shape (lines 12-33) is the direct template for `TestCandidateK` itself
staying green after the rename to `CandidateK` — only the function-name references inside the
test change; the table and assertions are otherwise untouched.

---

### `internal/store/store.go` (`SearchReranked`, body/doc-comment edit only)

**Analog:** itself — current body, lines ~1304-1338 (verified this session, quoted below).

**Current shape** (unchanged signature — only the body's last line and the doc comment change
under D-08 if vector-only wins, or the ranker-choice constant if a tuned variant wins):
```go
func (s *Store) SearchReranked(ctx context.Context, scope string, subj Subject, query string, vec []float32, k uint64, opts SearchOptions) ([]Memory, error) {
	if k == 0 {
		return nil, fmt.Errorf("%w: SearchReranked requires k > 0 (caller must apply its default before calling)", ErrInvalidArgument)
	}
	opts.Full = true
	hits, err := s.Search(ctx, scope, subj, vec, candidateK(k), opts)
	if err != nil {
		return nil, err
	}
	return RerankHits(query, hits, int(k)), nil
}
```
Do not change the signature — both MCP (`deps.searchMemory`, `internal/server/tools.go:1871`)
and Connect (`engramAPI.SearchMemories`, `internal/server/connectapi.go:324`) call this exact
shape and must not need edits themselves. Only the final `return` line's ranking call changes
per the D-05 decision-rule outcome (RANK-02), and the doc comment above it (lines ~1304-1321,
which currently over-specifies "the pure RerankHits lexical-overlap reorder" by name) needs
rewriting to describe whichever ranking actually ships, generically enough to survive a future
Jev-seam swap (Phase 4).

---

### `internal/server/tools.go` (`StoreAndEmbedderFromEnvNoEnsure`, widen return tuple — D-14)

**Analog:** itself — current signature/body, lines 260-287 (verified this session, quoted in
RESEARCH.md and re-confirmed here).

**Current shape:**
```go
func StoreAndEmbedderFromEnvNoEnsure() (*store.Store, uint64, *embed.Client, string, error) {
	cfg, err := loadAndValidate()
	if err != nil {
		return nil, 0, nil, "", err
	}
	st, dim, err := storeFromConfig(cfg)
	if err != nil {
		return nil, 0, nil, "", err
	}
	em, err := embedderFromConfig(cfg)
	if err != nil {
		return nil, 0, nil, "", err
	}
	identity, err := config.EmbedderIdentity(cfg)
	if err != nil {
		return nil, 0, nil, "", fmt.Errorf("embedder identity: %w", err)
	}
	return st, dim, em, identity, nil
}
```
**Target:** widen the return tuple to also return `cfg` — `(*store.Store, uint64, *embed.Client,
string, *config.Config, error)` — every error-path `return` gains a `nil` in the new slot; the
success `return` becomes `return st, dim, em, identity, cfg, nil`. **Do not add a second
`config.Load` call** — reuse the exact `cfg` `loadAndValidate()` already produced (this is the
entire point of D-14: the differ gate's skip check must read the SAME config the embedder was
built from, not a second, independently-resolved one).

**Callers that must be updated to the new 6-value arity** (confirmed this session via
`StoreAndEmbedderFromEnvNoEnsure` call-site search):
- `internal/retrievaleval/retrieval_eval_test.go` — `TestRetrievalEval` (line 80),
  `TestRetrievalEval_AsymmetryDiffer` (line 261) — both currently discard `cfg` via `_`; both
  must now capture it and use `cfg.Embed.QueryInstruction`/`.DocumentInstruction`/
  `.QueryParams`/`.DocumentParams` for the symmetric-config skip.
- `cmd/engram/reindex.go` (line ~55) — discard the new `cfg` value with `_` (reindex does not
  need it).
- `internal/server/tools_test.go` — `TestStoreAndEmbedderFromEnvNoEnsureValidatesConfig` and
  `TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce` (both quoted below) — update their
  multi-value assignment lines to the new arity.

**Single-load-invariant test pattern to preserve** (`TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce`,
lines 5215-5269 of `tools_test.go` — read this session, excerpt):
```go
loads := 0
orig := configLoad
configLoad = func(flags *flag.FlagSet) (*config.Config, error) {
	loads++
	return orig(flags)
}
t.Cleanup(func() { configLoad = orig })

st, dim, em, identity, err := StoreAndEmbedderFromEnvNoEnsure()
...
if loads != 1 {
	t.Errorf("StoreAndEmbedderFromEnvNoEnsure loaded config %d times, want exactly 1", loads)
}
```
This test's assignment line (`st, dim, em, identity, err :=`) must be updated to
`st, dim, em, identity, cfg, err :=` (or `_` for `cfg` if unused in that specific test body) —
the `loads != 1` assertion itself is untouched by the widened tuple and remains the exact
regression guard for "do not add a second `config.Load` call."

---

### Retrieval-eval config gate (D-15 revised — test-local koanf load, NOT registry)

**No exact analog exists in this repo** — `internal/config/config.go`'s `Load` (lines 349-411)
is the only `koanf.New(...)` call site in the codebase (`rg -n "koanf.New" --type go` — one hit),
and it routes every env var through `registry.go`'s `envToKey` map, which by design excludes
test-only vars (confirmed by `internal/config/legacy_test.go`'s `TestCheckLegacyIgnoresTestOnlyVar`
and `registry.go`'s own header comment, lines 21-24). A test-local loader for one unregistered
key must NOT call `config.Load` or reuse `envToKey` — it needs its own minimal koanf
provider setup.

**Closest mechanics-only analog — `Load`'s env-provider shape** (`internal/config/config.go`,
lines 349-411, quoted in relevant part):
```go
k := koanf.New(".")
if err := k.Load(env.Provider(".", env.Opt{
	Prefix: Prefix, // "ENGRAM_"
	TransformFunc: func(key, val string) (string, any) {
		if val == "" {
			return "", nil
		}
		if mapped, ok := envToKey[key]; ok {
			return mapped, val
		}
		return "", nil // ignore unknown ENGRAM_* vars — THIS is what excludes our key
	},
}), nil); err != nil {
	return nil, fmt.Errorf("config env: %w", err)
}
```
**What must differ for D-15:** the test-local loader's `TransformFunc` must map
`ENGRAM_RETRIEVAL_EVAL` directly (not through `envToKey`, which never contains it), e.g. a
`koanf.New(".")` + `env.Provider(".", env.Opt{Prefix: config.Prefix, TransformFunc: func(key,
val string) (string, any) { if key == "retrieval_eval" { return "retrieval_eval", val }; return
"", nil }})` — same prefix constant (`config.Prefix`, exported), same env package
(`github.com/knadh/koanf/providers/env/v2`), same delimiter (`"."`), but a private, one-key
transform instead of the shared registry map. **Never call `.Validate()`** on any config this
loader touches (Pitfall 2) — this loader has no `Config` struct to validate against in the first
place, which structurally enforces that constraint.

**Precedent for "test-only var, deliberately not in the registry" as a *convention*** (the
convention this loader must respect, even though its *mechanism* differs) —
`internal/store/storetest/storetest.go`, lines 1-19 (full package doc comment, quoted in
RESEARCH.md) and its plain `os.Getenv`/`strconv.ParseBool` reads of `ENGRAM_QDRANT_TEST_ADDR`/
`ENGRAM_REQUIRE_QDRANT`. D-15 chooses koanf over `storetest`'s plain `os.Getenv` specifically
because "any config source koanf reads" (not just the process env) must enable the gate — but
the *never-touches-the-registry* discipline is identical to `storetest`'s.

**Where this loader lives:** per CONTEXT.md D-15 and RESEARCH.md's supersession note, this is a
`internal/retrievaleval`-local helper (e.g. `retrievalEvalEnabled() bool` in
`retrieval_eval_test.go` or a small new file in that package) — never a `internal/config`
package change. `TestMain` and each test's own gate call this same local helper, matching the
existing "TestMain, mirrored by each test's own defense-in-depth check" pattern already present
in this file (see `TestRetrievalEval`/`TestRetrievalEval_AsymmetryDiffer`/`TestMain`/
`TestSharedQdrantAddressHonored`, all four independently checking the gate today with
`os.Getenv` — the fix replaces that repeated `os.Getenv` call with a repeated call to the new
local helper, same call-site count, same defense-in-depth shape).

---

## Shared Patterns

### Config resolution: single-load invariant
**Source:** `internal/server/tools.go` `loadAndValidate`/`StoreAndEmbedderFromEnvNoEnsure`
(lines 196-205, 260-287); pinned by `internal/server/tools_test.go`
`TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce` (lines 5215-5269).
**Apply to:** the widened `StoreAndEmbedderFromEnvNoEnsure` and both retrieval-eval test files
that call it (`retrieval_eval_test.go`). Never call `config.Load`/`loadAndValidate` a second time
just to get a value already present on an already-loaded `cfg`.

### Test-only var exclusion from the registry
**Source:** `internal/config/registry.go` header comment (lines 21-24); `internal/config/legacy.go`
lines 18-22; `internal/config/legacy_test.go` `TestCheckLegacyIgnoresTestOnlyVar`;
`internal/store/storetest/storetest.go` lines 1-19.
**Apply to:** the new `ENGRAM_RETRIEVAL_EVAL` gate — D-15 revised keeps this var OUT of
`registry.go` (do not add a row there, do not add an `EvalConfig` struct, do not touch
`legacy.go`). The gate is resolved by a package-local koanf load instead (see dedicated block
above).

### Pure-function ranking contract (query, hits, k) -> []Memory
**Source:** `internal/store/rerank.go` `RerankHits` (lines 58-100), doc comment lines 58-64: "no
I/O, no embedder, no server/handler concepts leak in ... trivially unit-testable."
**Apply to:** `CosineBlendRerank`, `OverlapGateRerank` (new, same file) — same signature shape,
same determinism/tie-break guarantee, same hermetic-unit-testability requirement enforced by
`rerank_test.go`'s existing style.

### Shared ranking seam — exactly one path both surfaces call
**Source:** `internal/store/store.go` `SearchReranked` (lines ~1304-1338); callers
`internal/server/tools.go` `deps.searchMemory` (lines 1871-1898) and
`internal/server/connectapi.go` `engramAPI.SearchMemories` (lines 324-373); interface contract
`internal/server/store_iface.go` `memStore.SearchReranked` (line ~35).
**Apply to:** any ranking-variant work — only the variant the D-05 decision rule selects may ever
be wired into `SearchReranked`'s body. Every other named variant (vector-only, blend, gate,
shipped-lexical-for-comparison) is measured by the eval calling `st.Search` at `CandidateK(k)`
directly and applying its pure ranking function locally — never by adding a second code path
inside `SearchReranked` itself.

### Doc-comment convention: cite the decision/issue ID inline
**Source:** pervasive across every file read this session — e.g. `rerank.go` lines 11-15,
`fixtures.go` lines 61-67/87-108, `tools.go` lines 260-268, `store_iface.go` lines 12-22.
**Apply to:** every new function/type/test added this phase — name the GH issue (#353, #354,
#605, #261) or decision ID (D-05, D-13, D-14, D-15) the code exists to satisfy, directly in its
doc comment, matching this codebase's dominant convention.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| Retrieval-eval config gate (test-local koanf load) | config (test-local) | request-response | No prior "koanf load that deliberately bypasses the registry" exists in this repo — `config.Load` is the only `koanf.New` call site and it is registry-bound by design. Use `config.go`'s `Load` for koanf *mechanics* only (provider setup, prefix, delimiter), and `storetest.go`'s package doc comment for the *test-only-var-excluded-from-registry* convention it must still honor. |
| Blind-subagent paraphrase-query authorship procedure (D-01) | N/A (process, not code) | N/A | RESEARCH.md confirms: `rg -n "blind" .planning/` finds no prior GSD phase using this pattern. Treat RESEARCH.md's "Blind-subagent query authorship" recommended shape (executor writes topic labels -> fresh subagent writes queries from labels only -> executor maps back to keys) as the only guidance; there is no existing code to copy from. |

## Metadata

**Analog search scope:** `internal/retrievaleval/`, `internal/store/` (rerank.go, rerank_test.go,
store.go, store_iface.go via `internal/server/`), `internal/config/` (config.go, registry.go,
legacy.go, legacy_test.go, config_test.go), `internal/server/` (tools.go, tools_test.go,
connectapi.go), `internal/store/storetest/`, `.planning/spikes/004-jev-rerank-eval/`.
**Files scanned:** 15 read in full or targeted-section this session (see RESEARCH.md Sources for
the complete list with line numbers; this pattern map re-confirmed line numbers against the live
tree rather than trusting RESEARCH.md's citations verbatim).
**Pattern extraction date:** 2026-09-22
