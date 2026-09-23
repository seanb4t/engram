# Phase 1: Eval Foundation & Lexical Reranker Fix - Research

**Researched:** 2026-09-22
**Domain:** Go retrieval-quality evaluation harness, koanf config registry, vector-search reranking
**Confidence:** HIGH (all core claims verified by reading the actual source files this session; the
only genuinely new design surface — the pluggable ranker list and the blind-subagent procedure — is
flagged LOW/MEDIUM and left as an explicit recommendation, not an assumed fact)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Queries are written by a **blind subagent** that sees only short topic descriptions of
  each target (e.g. "the record about pre-commit linting"), never the record text. A separate pass
  maps each query to its target key. The fixture comment records this procedure so the case is
  reproducible and its independence is auditable.
- **D-02:** The corpus is a **new multi-domain synthetic set of 80–120 spine-shaped records**
  (several domains, e.g. tooling, auth, deploy, data model, config, testing), with deliberate
  sticky topical neighbours for each target. Content is synthetic public-style text — no verbatim
  spine content, no secrets (public repo). A corpus this size exceeds `candidateK`'s floor of 32,
  so the reranker sees a real candidate subset, unlike spike 004 where every candidate set was the
  whole corpus.
- **D-03:** About **20 single-answer queries** (one intended record each, fits the existing
  `wantKey` shape) **plus a few no-answer queries** (no correct record in the corpus).
- **D-04:** The #261 case (`gh261Case`) stays unchanged as the permanent near-verbatim regression
  guard.
- **D-05:** **Pre-committed decision rule** (decide before seeing numbers): among variants that
  (a) keep #261's target at rank 1 for both #261 queries and (b) have paraphrase MRR ≥ vector-only
  MRR, ship the one with the best paraphrase MRR. A tuned variant (blend or gate) wins over
  vector-only only if it beats vector-only paraphrase MRR by a clear margin (≥ 0.05); otherwise
  the simpler variant wins. Ties go to the simplest.
- **D-06:** Variants measured: **vector-only (no lexical)**, **cosine blend**
  (score = cosine + α·normalized overlap, rather than reordering purely on overlap),
  **overlap-threshold gate** (promote only on very high, near-verbatim overlap; otherwise keep
  vector order), and **shipped lexical** (baseline).
- **D-07:** α and the gate threshold are tuned over a **small fixed grid (3–4 values each)**, with
  the ≥ 0.05 MRR margin in D-05 as the guard against overfitting to the same eval set.
- **D-08:** If vector-only wins: **keep `store.SearchReranked` as the single shared seam** (MCP
  `deps.searchMemory` and Connect `SearchMemories` both call it) with a no-op rank step, and
  delete the lexical code. Phase 4's Jev reranker plugs into this seam. — **Reversibility:**
  reversible — lexical code is recoverable from git; the seam is unchanged.
- **D-09:** Whichever variant wins, the chosen ranking and the measured table are recorded (in
  the eval log output and the phase summary) so #605 can be closed with evidence.
- **D-10:** Hard `t.Errorf` bars: #261 target **at rank 1** (tightened from "within default k")
  for both #261 queries, and **shipped paraphrase MRR ≥ vector-only paraphrase MRR**. Per-variant
  recall@k and MRR are logged as a table (`t.Logf`), not gated. No absolute thresholds (brittle
  across embedders).
- **D-11:** The eval iterates a **pluggable, named list of rankers** (vector-only, lexical
  variants, …) so Phase 4 appends Jev without refactoring. Phase 1 also adds a **Jev stub entry**
  that reports "Jev: disabled" in the table — no Jev client, config, or network call in Phase 1.
- **D-12:** No-answer queries are **excluded from recall@k/MRR** and only logged (top hit and its
  score) — kept in the fixture for Phase 4's per-hit relevance signal (RANK-04). No gate.
- **D-13:** Differ gate (#353): pass only when **cosine distance > 1e-3** between the query- and
  document-side vectors; a NaN or Inf component in either vector is a hard `t.Fatal`. Soften the
  PASS log to "vectors differ materially" (no claim about the cause). Keep the existing dimension
  contract check.
- **D-14:** Symmetric-config skip (#354): decide from the **resolved koanf config** — the same
  config the embedder is built from — checking its four embed instruction/params fields. This
  likely needs a loader that returns the resolved config alongside the embedder (or reuses
  `loadAndValidate`).
- **D-15:** **Register `ENGRAM_RETRIEVAL_EVAL` in `internal/config`'s field registry** (the single
  source of truth for `ENGRAM_` vars) so any config source enables it. `TestMain` and each test's
  gate read the resolved value, and `TestMain` still short-circuits before any Docker/testcontainer
  startup when it is off. — **Reversibility:** costly — a registered key is documented operator
  surface (config reference/docs gates), so removing it later touches docs and the registry.

### Claude's Discretion

- Exact domains and record count within 80–120; exact α / threshold grid values; the number of
  no-answer queries ("a few").
- How the blind-subagent procedure is run during execution (prompt shape, how topic descriptions
  are produced), as long as D-01's independence holds and is documented.
- Whether per-variant ranking lives in the eval package or as small exported helpers in
  `internal/store`, provided the shipped path is measured through `SearchReranked` itself.

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope. (No-answer queries are captured now but used for
the relevance signal only in Phase 4.)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| EVAL-01 | The embedding differ gate compares with a cosine epsilon, not `reflect.DeepEqual` on `[]float32` (#353) | "Differ gate (EVAL-01)" pattern below gives the exact replacement code and the cosine-distance helper it needs; "Don't Hand-Roll" notes no existing cosine helper exists in production Go code today |
| EVAL-02 | The retrieval-eval skip guard reads the resolved koanf config, not raw `os.Getenv` (#354) | "Config registry (EVAL-02 / D-15)" section gives the registry-entry shape, the `Config` struct field, the `Load`-vs-`Validate` distinction, and every gate a new field trips |
| RANK-01 | The retrieval eval includes an independently written paraphrase case alongside #261, reporting recall@k and MRR for vector-only, lexical, and (when enabled) Jev ordering (#605) | "Fixture shape change", "Pluggable ranker design", and "Blind-subagent query authorship" sections |
| RANK-02 | The default ranking does not regress the paraphrase case versus vector-only order and keeps #261's target at rank 1 (#605) | "Pluggable ranker design" and "SearchReranked seam and callers" sections give the exact decision-rule wiring and the D-08 delete-path blast radius |
</phase_requirements>

## Summary

This phase touches one Go package (`internal/retrievaleval`), one shared library function
(`internal/store/rerank.go`'s `RerankHits`/`SearchReranked`), and one config file
(`internal/config/registry.go`). All three are small, already well-tested, and the exact functions
to change were read this session — there is no framework or library to select; this is pure
in-repo engineering against a system whose seams are already correctly abstracted (one search path,
one config registry, one metric-helper file).

The single largest risk is **not** algorithmic (the metrics/ranking math is simple, and spike 004
already validated the shape) — it is **fixture-shape and config-registry mechanics**. Three
concrete traps exist and are documented in detail below: (1) `retrievalCase.wantKey` is
case-scoped today, but the new paraphrase case needs one target **per query**, which is a real
struct change, not a data addition; (2) `internal/config/registry.go`'s own file comment and
`legacy_test.go`'s `TestCheckLegacyIgnoresTestOnlyVar` establish an explicit, tested convention
that test-only vars are **deliberately excluded** from the registry — D-15 reverses that
convention for this one key, and the planner must both honor D-15 (it is locked) and account for
its cost (a new, permanent, documented operator-facing config key with no runtime effect); (3)
`StoreAndEmbedderFromEnvNoEnsure` currently discards the `*config.Config` it built — exposing it
to the eval is a narrow, well-precedented signature change (widen the return tuple; do not add a
second `config.Load` call, which would break the existing single-load invariant test).

**Primary recommendation:** Widen `server.StoreAndEmbedderFromEnvNoEnsure`'s return tuple to
include `*config.Config` (zero extra config loads); add a new `Eval.RetrievalEval` bool-typed
registry field consumed only by `TestMain`/tests via that same resolved config; promote
`retrievalQuery.wantKey` to replace `retrievalCase.wantKey`; write the two new ranking variants as
exported pure functions beside `RerankHits` in `internal/store/rerank.go`, sharing `Memory.Score`
(already Qdrant's raw cosine similarity) for the blend variant instead of recomputing cosine
locally.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Ranking / reranking logic | API / Backend (`internal/store`) | — | `SearchReranked` is the one shared seam both MCP and Connect call; ranking must never live in a handler |
| Retrieval-quality measurement | API / Backend (`internal/retrievaleval`, test-only) | — | A `_test.go`-gated package, not shipped; measures the backend's own seam end-to-end against real Qdrant |
| Config resolution (env → typed struct) | API / Backend (`internal/config`) | — | Single koanf registry; no other tier touches `ENGRAM_*` resolution |
| Embedding (query/document vectors) | API / Backend (`internal/embed` via `internal/server.StoreAndEmbedderFromEnvNoEnsure`) | — | Both the differ gate and the eval must go through the exact prod embedder builder, never a bespoke shortcut |
| Vector storage / candidate fetch | Database / Storage (Qdrant via `internal/store.Store.Search`) | — | `candidateK` over-fetch and authz-scoped filtering happen here; ranking never widens what Search already returned |

No browser, frontend-server, or CDN tier is in scope for this phase — it is entirely backend/test
infrastructure.

## Config registry (EVAL-02 / D-15)

### The registry, and what it does NOT already contain

`internal/config/registry.go` is the single source of truth for every `ENGRAM_*` var the **server
binary** reads. Its own header comment is explicit and load-bearing:

> "Command-local vars (migrate/reindex targets, the test-only Qdrant addr) are NOT here — they are
> read directly by their command, but their legacy names are registered for the guard in
> legacy.go." [VERIFIED: internal/config/registry.go:21-24]

`internal/store/storetest/storetest.go` confirms the pattern in practice: `ENGRAM_QDRANT_TEST_ADDR`
and `ENGRAM_REQUIRE_QDRANT` are read via plain `os.Getenv`/`strconv.ParseBool`, never through
`config.Load`:

> "It reads exactly two environment variables — ENGRAM_QDRANT_TEST_ADDR and ENGRAM_REQUIRE_QDRANT
> — and never names the production Qdrant address variable, even conceptually" [VERIFIED:
> internal/store/storetest/storetest.go:16-19]

And `internal/config/legacy.go` codifies this as a **tested** invariant — `legacy_test.go`'s
`TestCheckLegacyIgnoresTestOnlyVar` exists specifically to prove the startup guard never trips on
a test-only var:

```go
// legacy.go:18-22
// command-local vars read by a real command (reindex.go / migrate.go).
// Test-only vars (e.g. MEM_QDRANT_TEST_ADDR, read solely by integration
// tests) are deliberately NOT guarded here: the runtime binary never
// reads them, so flagging them would false-trip a CI/dev env that
// exports the legacy name to point tests at Qdrant.
```
[VERIFIED: internal/config/legacy.go:18-22]

**This means there is NO existing precedent for a test-only key living in the registry** — the
established pattern is the opposite. D-15 is a locked decision that deliberately breaks this
pattern (its own "Reversibility: costly" note acknowledges this). The planner should implement
D-15 as written, but the plan should record explicitly *why* this key is an exception (so a future
reviewer does not "fix" it back to `os.Getenv` citing the very convention above) — e.g. a doc
comment on the new registry row citing D-15 and this research file.

### Concrete registry addition

Add a new `EvalConfig` config section (no existing section fits; `ENGRAM_RETRIEVAL_EVAL` gates
the **test package**, not any server subsystem):

```go
// config.go — new field on Config
Eval EvalConfig `koanf:"eval"`

// EvalConfig gates test-only eval packages. Not consumed by the server binary
// (server.go) at all — read only by internal/retrievaleval's TestMain and its
// individual test gates, via the SAME resolved config the eval's embedder is
// built from (D-15). Registered here (an intentional, documented exception to
// registry.go's "test-only vars are NOT here" convention — see 01-RESEARCH.md)
// so ENGRAM_RETRIEVAL_EVAL set via any config source (env today; a future
// config layer tomorrow) enables the eval identically.
type EvalConfig struct {
	RetrievalEval string `koanf:"retrieval_eval"`
}
```

```go
// registry.go — new row
{Key: "eval.retrieval_eval", Env: "ENGRAM_RETRIEVAL_EVAL", Default: "false"},
```

Parse it the same way every other boolean registry field is parsed in this codebase
(`usage.signals`, `connect.headless`, `ui.enabled` all follow this exact idiom — `strconv.ParseBool`
with a documented fallback):

```go
// internal/server/tools.go already has this shape for cfg.Usage.Signals:
signals, err := strconv.ParseBool(cfg.Usage.Signals)
if err != nil || !signals { return nil }
```
[VERIFIED: internal/server/tools.go:384-385 — `strconv.ParseBool(cfg.Usage.Signals)`]

`strconv.ParseBool` already accepts `"1"` as true, so switching from `os.Getenv(...) != "1"` to a
`ParseBool`-based resolved-config read is a **behavior-compatible generalization** (accepts
`"1"/"t"/"T"/"true"/"True"/"TRUE"`, not just `"1"`), not a behavior change that needs a compat
shim.

### `Config.Validate()` is NOT a concern here

`Config.Validate()` (internal/config/validate.go) only validates a fixed, explicit list of
data-plane fields (Qdrant addr/collection, Embed model/dim, memory caps, summarize-when-enabled,
embed timeouts). It does not iterate the registry or reflect over `Config`, so a new
`Eval.RetrievalEval` field requires **no `Validate()` change** and cannot make `Validate()` start
failing for unrelated reasons. [VERIFIED: internal/config/validate.go:1-60, read this session]

### Pitfall: do not gate `TestMain` behind a `Validate()`-calling loader

`loadAndValidate()` (`internal/server/tools.go:196-205`) calls `config.Load` **and**
`cfg.Validate()`. `Validate()` checks unrelated fields (`ENGRAM_QDRANT_ADDR` host:port shape,
`ENGRAM_EMBED_DIM` parseable, etc.) that have nothing to do with whether the eval should run.

If `TestMain` (which runs unconditionally for every `go test ./internal/retrievaleval/...`
invocation, including the ordinary CI `go test ./...` job where the eval is almost always **off**)
calls a `Validate()`-including loader just to read one boolean, then a malformed *unrelated*
`ENGRAM_*` value in the ambient environment (e.g. a developer's shell exporting a bad
`ENGRAM_EMBED_DRAIN_BYTES` while debugging something else) would make the *entire required `test`
job* fail in this package — even though the eval itself is off and untouched. This is worse than
today's behavior (a bare `os.Getenv` check can never fail).

**Recommendation:** the gate check must use `config.Load(nil)` directly (never errors in practice —
it is pure koanf assembly, no field-level validation) and must NOT call `.Validate()`. Only once
the eval is confirmed enabled should the full `loadAndValidate`-equivalent path run (which is
exactly what `StoreAndEmbedderFromEnvNoEnsure` already does internally).

### Widening `StoreAndEmbedderFromEnvNoEnsure` (D-14)

`StoreAndEmbedderFromEnvNoEnsure` (`internal/server/tools.go:269-287`) already loads config exactly
once via `loadAndValidate()` and discards the `*config.Config` after using it:

```go
func StoreAndEmbedderFromEnvNoEnsure() (*store.Store, uint64, *embed.Client, string, error) {
	cfg, err := loadAndValidate()
	...
	return st, dim, em, identity, nil   // cfg is discarded here
}
```
[VERIFIED: internal/server/tools.go:269-287]

The cleanest fix for D-14 (per CONTEXT.md's own framing: "a loader that returns the resolved
config alongside the embedder") is to widen this function's return tuple to also return `*cfg`,
**not** add a second `config.Load` call. This is a small, bounded change:

- Callers found this session, all of which must be updated to the new arity:
  `internal/retrievaleval/retrieval_eval_test.go` (2 call sites: `TestRetrievalEval` line 80,
  `TestRetrievalEval_AsymmetryDiffer` line 261), `cmd/engram/reindex.go:55`,
  `internal/server/tools_test.go` (2 tests:
  `TestStoreAndEmbedderFromEnvNoEnsureValidatesConfig` line 5154,
  `TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce` line 5235). [VERIFIED: `rg -n
  "StoreAndEmbedderFromEnvNoEnsure" --type go`, run this session — 4 non-doc-comment call sites]
- `TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce` pins a **single-load invariant** via a
  `configLoad` seam-swap counter (`loads != 1`). Widening the return tuple without adding a second
  `config.Load` call preserves this invariant trivially — the returned `*config.Config` is the
  exact same value `loadAndValidate()` already produced. [VERIFIED:
  internal/server/tools_test.go:5223-5245, read this session]

The symmetric-config skip in `TestRetrievalEval_AsymmetryDiffer` should then read
`cfg.Embed.QueryInstruction`, `cfg.Embed.DocumentInstruction`, `cfg.Embed.QueryParams`,
`cfg.Embed.DocumentParams` from this SAME returned `cfg` — not a second, independent
`config.Load(nil)` call — so D-14's "the same config the embedder is built from" requirement holds
by construction, not by two calls happening to agree.

## Differ gate (EVAL-01 / D-13)

### Current code (the bug)

```go
// retrieval_eval_test.go:288-290 (current)
if reflect.DeepEqual(queryVec, documentVec) {
	t.Fatalf("asymmetry differ FAIL: ...")
}
```
[VERIFIED: internal/retrievaleval/retrieval_eval_test.go:288-290, read this session]

`reflect.DeepEqual` on `[]float32` only fails the gate (correctly reports "differ") when the two
slices are bit-identical. A hosted embedding API that is not bit-deterministic across calls (GPU
float non-associativity, batching, load-balanced backends) can produce two vectors that differ by
one ULP — `DeepEqual` still says "differ" (test PASSES) — but the intended failure mode (an
operator wired `ENGRAM_EMBED_QUERY_PARAMS`/`task_type` instead of
`ENGRAM_EMBED_QUERY_INSTRUCTION`, so query and document embeds are IDENTICAL in substance but
happen to jitter by float noise) would ALSO produce two non-bit-identical vectors, so
`reflect.DeepEqual` never actually catches that case either — it can only catch true bit-for-bit
identity, which is strictly narrower than "materially the same." [CITED: GitHub #353 issue body,
which is the source of this exact analysis]

### Don't Hand-Roll: no cosine helper exists yet

**No cosine similarity/distance function exists anywhere in this repo's production Go code today.**
`rg -n "cosine" --type go -i` (excluding `_test.go`) returns only Qdrant server-side distance
config (`qdrant.Distance_Cosine`, `internal/store/store.go:624`) and doc-comment prose — no local
`[]float32`-to-`[]float32` cosine function. [VERIFIED: `rg -n "cosine" --type go -i` run this
session against the full non-test tree]

Spike 004 (`.planning/spikes/004-jev-rerank-eval/main.go:75-83`) already wrote and used exactly
this function — reuse it verbatim rather than re-deriving:

```go
// .planning/spikes/004-jev-rerank-eval/main.go:75-83 — reusable as-is
func cosine(a, b []float32) float64 {
	var d, na, nb float64
	for i := range a {
		d += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	return d / (math.Sqrt(na) * math.Sqrt(nb))
}
```
[VERIFIED: .planning/spikes/004-jev-rerank-eval/main.go:75-83, read this session]

D-13 needs **cosine distance** (`1 - cosine similarity`), not similarity, since the gate is
"distance > 1e-3" (materially different, i.e. far apart) — invert the spike's `cosine()` output at
the call site (`distance := 1 - cosine(queryVec, documentVec)`) rather than writing a second
function.

This is a ~10-line pure function with zero external dependencies — a case where hand-rolling is
correct (no vector-math library is justified for one dot product), not a "don't hand-roll"
violation.

### NaN/Inf handling

Neither `math.IsNaN` nor `math.IsInf` is used anywhere in this repo today [VERIFIED: `rg -n
"math\.IsNaN|math\.IsInf" --type go` run this session, zero hits]. `math.IsNaN`/`math.IsInf` are
float64-only in the standard library; each `float32` component needs an explicit
`float64(x)` conversion before the check. D-13 requires this as a **hard `t.Fatal`** (not
`t.Errorf`) on any NaN/Inf component in either vector, checked BEFORE the distance computation
(a NaN in the dot product would silently propagate into a NaN distance, which then fails the
`> 1e-3` comparison in an unhelpful way).

### D-13 shape to implement

```go
distance := cosineDistance(queryVec, documentVec) // 1 - cosine(a, b)
if distance <= 1e-3 {
	t.Fatalf("asymmetry differ FAIL: cosine distance %.6f <= 1e-3 threshold — ...", distance)
}
t.Logf("asymmetry differ PASS: vectors differ materially (cosine distance=%.6f)", distance)
```

The dimension-contract check (`len(queryVec) == len(documentVec) == dim`) that already exists
(review B4, retrieval_eval_test.go:275-282) stays unchanged — D-13 says "Keep the existing
dimension contract check."

## Fixture shape change (RANK-01 / D-03 / D-12)

### The current shape cannot express "one target per query"

```go
// fixtures.go — current shape
type retrievalQuery struct {
	name string
	text string
}
type retrievalCase struct {
	name        string
	seedRecords []seedRecord
	queries     []retrievalQuery
	wantKey     string   // ONE target for the WHOLE case
}
```
[VERIFIED: internal/retrievaleval/fixtures.go:17-30, read this session, quoted verbatim]

`gh261Case` fits this shape because both its queries (`query-a`, `query-b`) target the SAME
record (`recordTKey`). The new paraphrase case does not fit: D-03 wants ~20 single-answer queries,
each intended to retrieve a **different** record out of the 80–120-record corpus (one query per
target, per spike 004's own query design — "16 paraphrase queries, one per record"
[CITED: .planning/spikes/004-jev-rerank-eval/README.md, "Added 16 paraphrase queries, one per
record"]), plus a few no-answer queries with NO correct target at all (D-12).

### Recommended shape

Move `wantKey` from `retrievalCase` to `retrievalQuery`, with an empty string meaning "no-answer
query" (D-12):

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
	// wantKey removed from this level — each query now carries its own target.
}
```

`gh261Case`'s two queries each become `{name: "query-a", text: "...", wantKey: recordTKey}` /
`{name: "query-b", text: "...", wantKey: recordTKey}` — a mechanical, behavior-preserving edit
(D-04: "gh261Case stays unchanged" refers to its *behavior/content*, not its Go struct literal
shape, which must change along with the type it instantiates).

### `TestRetrievalEval`'s loop must move the `wantKey` lookup inside the per-query loop

Today, `wantID` is resolved ONCE per case, before the query loop, from `tc.wantKey`
(`retrieval_eval_test.go:120-123`). With `wantKey` moved to the query level, this lookup must move
inside the `for _, q := range tc.queries` loop, and:

- When `q.wantKey == ""` (no-answer query): skip the hard rank-1 `t.Errorf` bar and the
  recall@k/MRR aggregation entirely; only `t.Logf` the top hit's ID and score (D-12 — "logged, not
  gated").
- When `q.wantKey != ""`: resolve `idByKey[q.wantKey]` (fail the test with `t.Fatalf` if the key is
  not among seeded records — mirrors the existing fixture-bug guard at line 120-123) and run the
  existing recall@k/MRR/rank-1 logic against it.

### D-10's rank tightening

The current hard bar only checks `rank == 0` (i.e., "did NOT surface within default k") —
literally "the target is anywhere in the top-k, at any position, counts as PASS":

```go
// retrieval_eval_test.go:186-190 (current)
if rank == 0 {
	t.Errorf("... Record T did NOT surface within default k=%d (hard rank bar FAILED ...")
} else {
	t.Logf("... rank=%d/%d (hard rank bar: PASS)", ...)
}
```
[VERIFIED: internal/retrievaleval/retrieval_eval_test.go:186-190, quoted verbatim]

D-10 requires tightening this to `rank == 1` specifically for the #261 case's two queries (not the
generic recall@k pass/fail used for the rest of the dataset) — "#261 target at rank 1 (tightened
from 'within default k')". The generic per-query recall@k/MRR computation for the paraphrase case
stays a `t.Logf` table (per D-10's "no absolute thresholds"); only two specific assertions are
hard `t.Errorf`s: (a) #261's two queries' target at rank 1, and (b) shipped-variant paraphrase MRR
≥ vector-only paraphrase MRR (a cross-variant comparison, not a per-query rank check — see
"Pluggable ranker design" below for where this comparison is computed).

## Pluggable ranker design (RANK-01 / RANK-02 / D-06 / D-08 / D-11)

### The core constraint: only ONE ranking ships through `SearchReranked`

`store.SearchReranked` (`internal/store/store.go:1304-1338`) is the single seam both
`deps.searchMemory` (MCP, `internal/server/tools.go:1871-1898`) and `engramAPI.SearchMemories`
(Connect, `internal/server/connectapi.go:329-372`) call — confirmed by reading both call sites this
session, and by the `memStore` interface (`internal/server/store_iface.go:43`) declaring exactly
one `SearchReranked` method that `*store.Store` satisfies. There is **no per-surface ranking
divergence possible today** — whatever `SearchReranked` does is what both MCP and Connect ship.

This means the eval's "pluggable list of rankers" (D-11) cannot all be wired through
`SearchReranked` simultaneously — only the ONE variant that ends up shipping can be. The other
candidate variants (D-06: vector-only, cosine blend, overlap-threshold gate, shipped lexical) must
be measured by the eval calling a **lower-level** seam directly and applying each candidate
ranking function locally, in-process, over the SAME candidate pool `SearchReranked` would have
fetched.

CONTEXT.md's own discretion note confirms this reading: "Whether per-variant ranking lives in the
eval package or as small exported helpers in `internal/store`, **provided the shipped path is
measured through `SearchReranked` itself**."

### Recommended design

1. **Export `candidateK`.** It is currently unexported (`internal/store/rerank.go:16`,
   `func candidateK(k uint64) uint64`), used only inside `store.SearchReranked`. Exporting it (e.g.
   `store.CandidateK`) lets the eval fetch the exact same over-fetched raw candidate pool
   `SearchReranked` would fetch, via `st.Search(ctx, scope, subj, vec, store.CandidateK(k),
   store.SearchOptions{Full: true})`, so every non-shipped variant ranks over an
   apples-to-apples candidate set. `candidateK` is already unit-tested
   (`internal/store/rerank_test.go:12-33`, `TestCandidateK`) and pure, so exporting it is a
   zero-risk rename, not new logic.

2. **Write the new ranking variants as exported pure functions beside `RerankHits` in
   `internal/store/rerank.go`**, not inside `internal/retrievaleval` — this keeps all ranking math
   in one file next to the function it will eventually replace, matches `RerankHits`'s existing
   "pure function of (query, hits, k)" contract (`internal/store/rerank.go:58-64`: "no I/O, no
   embedder, no server/handler concepts leak in ... trivially unit-testable"), and lets
   `internal/store/rerank_test.go` cover them with the SAME hermetic, no-Qdrant unit-test style
   already used for `RerankHits` (`TestRerankHitsPromotesLexicalOverlap`,
   `TestRerankHitsDeterministic`, `TestRerankHitsIgnoresAccessCount`).

   ```go
   // CosineBlendRerank promotes candidates by score = raw cosine + alpha*normalizedOverlap.
   // Memory.Score is ALREADY Qdrant's raw cosine similarity for this query (see the
   // search_memory tool doc string, tools.go:2764) — reuse it; do not recompute cosine
   // locally, there is no query/document vector pair available at this layer to recompute
   // it from (hits carry no vector, only the pre-computed Score).
   func CosineBlendRerank(query string, hits []Memory, k int, alpha float64) []Memory { ... }

   // OverlapGateRerank promotes a candidate ONLY when its normalized lexical overlap with
   // query exceeds threshold (near-verbatim territory); otherwise every hit keeps its
   // incoming (vector) order. threshold is a fraction of query terms matched, 0..1.
   func OverlapGateRerank(query string, hits []Memory, k int, threshold float64) []Memory { ... }
   ```

   Both can share `tokenize`/`lexicalOverlap` (already unexported helpers in the same file,
   `internal/store/rerank.go:27-56`) for the overlap term — no need to re-derive tokenization.

3. **Vector-only needs no new ranking function at all** — it is simply `st.Search(ctx, scope,
   subj, vec, k, opts)` (no over-fetch, no rerank), which the eval ALREADY calls today for its
   "ceiling" harness-correctness check (`retrieval_eval_test.go:199`, `st.Search(ctx, scope, subj,
   vec, ceilingK, ...)`) — same call shape, different `k`.

4. **The eval's per-query loop, for each named variant**, does one of two things:
   - **"shipped"**: call `st.SearchReranked(...)` (the real seam — this is what "measured through
     `SearchReranked` itself" means).
   - every other named variant ("vector-only", "cosine-blend-α=0.X", "overlap-gate-θ=0.Y",
     "lexical (shipped, for comparison even if not chosen)"): call `st.Search(...)` once at
     `CandidateK(k)` (or plain `k` for vector-only) and apply the variant's pure ranking function
     locally.
   - **"jev"**: a stub entry that appends a fixed `"Jev: disabled"` log line to the table and
     participates in no ranking or gating (D-11) — no network call, no config, no client.

5. **The α/threshold grid (D-07)** is a small fixed set (3–4 values each) tried as additional named
   variants (e.g. `cosine-blend-α=0.1`, `-α=0.2`, `-α=0.3`) — each is just another entry in the
   named-ranker list with a different parameter baked into its closure, not a separate code path.

6. **The D-05 decision rule and the D-10 hard "shipped paraphrase MRR ≥ vector-only paraphrase
   MRR" gate are cross-variant comparisons**, computed AFTER all variants have been run for the
   paraphrase case: aggregate recall@k/MRR per variant (reusing spike 004's
   `metrics{r1, r3, mrr int}` accumulator shape — `.planning/spikes/004-jev-rerank-eval/main.go:154-172`,
   directly transferable), then apply D-05's rule in code (not by hand) so the "pre-committed
   decision rule" is mechanically enforced rather than eyeballed, and log which variant won and
   why (D-09).

### `SearchReranked` and its callers (blast radius if D-08 fires — vector-only wins)

If the pre-committed decision rule (D-05) selects vector-only, `SearchReranked` keeps its exact
signature and both callers are untouched — only its BODY changes to skip the `RerankHits` call:

```go
// store.go:1322-1338 (current) — the only body that changes under D-08
func (s *Store) SearchReranked(ctx context.Context, scope string, subj Subject, query string, vec []float32, k uint64, opts SearchOptions) ([]Memory, error) {
	if k == 0 { return nil, fmt.Errorf(...) }
	opts.Full = true
	hits, err := s.Search(ctx, scope, subj, vec, candidateK(k), opts)
	if err != nil { return nil, err }
	return RerankHits(query, hits, int(k)), nil   // <- becomes a no-op truncate-to-k under D-08
}
```
[VERIFIED: internal/store/store.go:1322-1338, read this session, quoted verbatim]

Files that would need deleting/editing under D-08 (confirmed this session via `rg -n
"SearchReranked" --type go` and direct reads):

- `internal/store/rerank.go` — delete `RerankHits`/`tokenize`/`lexicalOverlap` (or keep them as
  dead-but-tested code if a later phase might want them; D-08 says "delete the lexical code").
  `candidateK` MUST stay (still governs the over-fetch bound for whatever ships, and is reused by
  the eval per the design above).
- `internal/store/rerank_test.go` — 5 tests exercise `RerankHits` directly
  (`TestRerankHitsPromotesLexicalOverlap`, `TestRerankHitsDeterministic`,
  `TestRerankHitsTruncatesToK`, `TestRerankHitsIgnoresAccessCount`, plus `TestCandidateK` which
  stays) — the 4 `RerankHits`-specific tests are deleted with it;
  `TestSearchRerankedRejectsZeroK` stays (tests `SearchReranked`'s own guard, not `RerankHits`).
- `internal/store/store.go:1304-1321`'s doc comment on `SearchReranked` references "the pure
  `RerankHits` lexical-overlap reorder" by name and needs rewriting regardless of which variant
  ships (it currently over-specifies the lexical mechanism as if it were permanent).
- Tests that assert membership/delegation behavior only (not the specific ranking) are UNAFFECTED:
  `TestSearchRerankedMatchesSearchMembership` (store_test.go:6988, asserts result-set membership
  parity with `Search`, not ranking), the `schemaversion_recallgate_test.go` delegation-shape
  assertions ("SearchReranked delegates without self-comparison"), and
  `TestRerankParityMCPAndConnect` (connectapi_test.go:350, asserts MCP/Connect call the SAME
  seam, not what that seam does internally) — none of these care which ranking function
  `SearchReranked` calls, only that there IS one shared seam. [VERIFIED: read
  internal/store/store_test.go:6982-6990 and grepped the other two test names this session]

If a tuned variant wins instead (cosine blend or overlap gate), the same block is replaced with a
call to whichever exported function from `internal/store/rerank.go` won, with its winning
parameter baked in as a named constant — `RerankHits`/`tokenize`/`lexicalOverlap` may still be
deleted (the shipped lexical baseline is retired either way per D-05/D-06's framing: it only ships
again if it is the least-regressing option, which spike 004's numbers make unlikely: shipped
lexical is the WORST-performing baseline on paraphrases in spike 004, R@1 0.50 vs vector-only's
0.88 [CITED: .planning/spikes/004-jev-rerank-eval/README.md, results table]).

## Blind-subagent query authorship (RANK-01 / D-01)

No prior GSD phase in this repo has used a "blind subagent" pattern before — searched
`.planning/` this session for "blind" and found no prior reproducible procedure to copy.
[VERIFIED: `rg -n "blind" .planning/` run this session — only this phase's own CONTEXT.md/
DISCUSSION-LOG.md reference the term; no prior implementation to reuse] This is new process design,
not an established pattern — treat the shape below as a recommendation (LOW confidence on exact
mechanics), not a verified precedent.

**Recommended shape**, satisfying D-01's independence requirement and its "reproducible and
auditable" documentation requirement:

1. **The executor** (who has full corpus/target visibility, since it authors the corpus per D-02)
   generates a short, generic topic label per target record — e.g. for a record about pre-commit
   lint config, a label like "the record about pre-commit linting" (CONTEXT.md's own example) —
   deliberately generic enough that it does not leak the record's exact wording (which would let
   a "blind" query-writer produce a near-verbatim query, defeating the paraphrase test's purpose).
2. **A fresh subagent invocation** (Claude Code's Task-tool general-purpose subagent, dispatched
   with a prompt containing ONLY the list of topic labels — never the seed-record content, never
   the corpus) is asked to write one natural-language query per label, as if trying to recall that
   topic from memory, without seeing how the underlying record is worded.
3. **A separate, non-blind pass** (the executor again) maps each returned query back to its
   originating record key by label, and assembles the `retrievalQuery{name, text, wantKey}`
   fixture entries.
4. **The fixture file comment** (in `fixtures.go`, beside the new case) must record: the exact
   label-generation approach, that the query-writing subagent received labels only, and the date/
   commit this was done, so the independence claim is auditable by a future reader without needing
   to trust prose alone — this satisfies CONTEXT.md's "fixture comment records this procedure"
   requirement literally.

This procedure runs once, during phase EXECUTION (not phase planning) — the plan should treat it
as its own task/step producing a concrete fixture-data artifact (the new case's Go literal),
checked into `fixtures.go` alongside `gh261Case`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cosine distance between two `[]float32` vectors | A new from-scratch derivation | Spike 004's already-written `cosine()` function (12 lines, dot-product + norms), inverted to distance at the call site | Already written, already used against the real prod embedder in spike 004; re-deriving risks a sign/normalization bug the spike already avoided |
| recall@k / MRR aggregation across many queries | Ad hoc per-case accumulator variables | Spike 004's `metrics{r1, r3, mrr int}` struct with an `.add(rank)` method (`.planning/spikes/004-jev-rerank-eval/main.go:154-172`) generalized to also track recall@k (already close to `recallAtK`/`reciprocalRank`'s existing per-query shape) | Same aggregation is needed per-variant-per-set (shipped/vector-only/blend/gate × gh261/paraphrase); a shared accumulator avoids N hand-copies of the same summing logic |
| Reading `ENGRAM_RETRIEVAL_EVAL` / the four embed instruction fields | A second, ad hoc `os.Getenv`-based resolver "just for tests" | The SAME `*config.Config` `server.StoreAndEmbedderFromEnvNoEnsure` already builds (widened to return it) | D-14's entire point is that a second, independent resolution path is what caused #354 — building a second one to fix it would reintroduce the exact bug class |

**Key insight:** every piece of math this phase needs (cosine distance, recall@k, MRR) was already
written and run against the real production embedder in spike 004 two days before this phase was
scoped. The engineering risk in this phase is almost entirely in the **plumbing** (config
resolution identity, fixture struct shape, which seam owns which ranking), not in any algorithm
that needs research.

## Common Pitfalls

### Pitfall 1: Registering `ENGRAM_RETRIEVAL_EVAL` without documenting why it breaks convention

**What goes wrong:** A future contributor (or an automated lint) sees a test-only key in
`registry.go` and "fixes" it back to `os.Getenv`, silently reintroducing #354's original bug,
citing `registry.go`'s own header comment and `TestCheckLegacyIgnoresTestOnlyVar` as precedent.
**Why it happens:** D-15 is a deliberate, locked exception to an established, tested convention,
but the convention's own test/comment don't know about the exception.
**How to avoid:** the plan's task for the registry addition must include a doc comment on the new
`EvalConfig`/registry row explicitly naming D-15 and this research file as the rationale, and
should NOT add a `legacyMap` entry (there is no legacy `MEM_RETRIEVAL_EVAL` to guard — this is a
brand-new var, so `legacy.go` needs no change at all).
**Warning signs:** a lint/review pass flags the new registry row as "test-only var in the
registry" without context.

### Pitfall 2: Calling `Validate()` from `TestMain`'s gate check

**What goes wrong:** an unrelated malformed `ENGRAM_*` env var in a developer's or CI's ambient
shell makes the ENTIRE `internal/retrievaleval` package's `go test` invocation fail (not skip),
even when `ENGRAM_RETRIEVAL_EVAL` is unset — regressing the "zero additional cost when the gate is
off" invariant `doc.go` and `TestMain`'s own doc comment currently guarantee.
**Why it happens:** `loadAndValidate()` bundles `Load` + `Validate` as one call; it is the
natural, "just reuse the existing helper" choice, but it is the WRONG helper for a mere gate check.
**How to avoid:** the gate check (in `TestMain` and each test's own defense-in-depth check) must
call `config.Load(nil)` directly, never `loadAndValidate`/`StoreAndEmbedderFromEnvNoEnsure`, until
AFTER the gate confirms the eval is enabled.
**Warning signs:** a passing local dev shell (with some unrelated malformed `ENGRAM_*` var) starts
failing `go test ./...` with a config-validation error attributed to `internal/retrievaleval`, a
package that should be a no-op when the eval is off.

### Pitfall 3: Treating `wantKey`'s move to query-level as a data-only change

**What goes wrong:** the plan adds a `wantKey` field to `retrievalQuery` but leaves
`retrievalCase.wantKey` in place "for backward compat," producing two sources of truth (which one
does `TestRetrievalEval` read?) and silently breaking `gh261Case` (whose case-level `wantKey` would
be read by old code, ignoring the new per-query field, or vice versa, depending on which one the
edit touched).
**Why it happens:** the existing code path resolves `wantID` ONCE before the query loop
(`retrieval_eval_test.go:120-123`); moving the lookup inside the loop is a control-flow change,
easy to miss when focused on "just add a field."
**How to avoid:** remove `retrievalCase.wantKey` entirely as part of the same change that adds
`retrievalQuery.wantKey`, so the compiler forces every construction site (including `gh261Case`) to
be updated — this is exactly the kind of change a "delete the old field" refactor should make, not
a keep-both-for-safety addition.
**Warning signs:** `gh261Case`'s literal still has a `wantKey:` line at the case level after the
edit — a compile error if the field is truly removed, so this pitfall is actually self-defending
IF the old field is deleted rather than merely deprecated.

### Pitfall 4: Recomputing cosine similarity for the blend variant instead of reusing `Memory.Score`

**What goes wrong:** the cosine-blend ranker recomputes cosine similarity from raw vectors, but
`Memory` (the type `Search`/`SearchReranked` return) carries no vector field — only `Score
float32` (`internal/store/store.go:284`, already documented as "the raw Qdrant cosine similarity
for this query" in the `search_memory` tool's own description string,
`internal/server/tools.go:2764`). A ranker written against raw vectors either needs to plumb
vectors through `Memory` (a real, unnecessary API change) or silently duplicates a value Qdrant
already computed server-side.
**Why it happens:** spike 004's throwaway script computed cosine itself because it deliberately
bypassed Qdrant entirely (comment: "The Qdrant search is replaced by an in-memory cosine ranking
over 16 records, because every candidate set here is the whole corpus" — not true in Phase 1,
where the 80–120-record corpus exceeds `candidateK`'s floor).
**How to avoid:** the cosine-blend variant's score formula should be `hit.Score + alpha *
normalizedOverlap` directly — `hit.Score` IS the cosine term already.
**Warning signs:** a new `Memory.Vector []float32` field or a second embed call inside the ranking
function — either is a sign the design reached for the wrong data source.

## Code Examples

### Reusable: cosine distance (adapted from spike 004, EVAL-01)

```go
// Source: .planning/spikes/004-jev-rerank-eval/main.go:75-83 (verified this session), inverted to
// distance for D-13's "cosine distance > 1e-3" gate.
func cosineDistance(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	return 1 - dot/(math.Sqrt(na)*math.Sqrt(nb))
}
```

### Reusable: metrics accumulator (adapted from spike 004, RANK-01)

```go
// Source: .planning/spikes/004-jev-rerank-eval/main.go:154-172 (verified this session).
// recallAtK/reciprocalRank already exist per-query in fixtures.go; this generalizes their
// aggregation across a whole named-variant × query-set combination.
type variantMetrics struct {
	recallHits, mrrSum float64
	n                  int
}

func (m *variantMetrics) add(hit bool, rr float64) {
	m.n++
	if hit {
		m.recallHits++
	}
	m.mrrSum += rr
}

func (m variantMetrics) String() string {
	return fmt.Sprintf("recall@k=%.2f MRR=%.3f (n=%d)", m.recallHits/float64(m.n), m.mrrSum/float64(m.n), m.n)
}
```

### Existing: the shared ranking seam both surfaces call (unchanged shape, body is what varies)

```go
// Source: internal/store/store.go:1322-1338, verified this session.
func (s *Store) SearchReranked(ctx context.Context, scope string, subj Subject, query string, vec []float32, k uint64, opts SearchOptions) ([]Memory, error) {
	if k == 0 {
		return nil, fmt.Errorf("%w: SearchReranked requires k > 0 (caller must apply its default before calling)", ErrInvalidArgument)
	}
	opts.Full = true
	hits, err := s.Search(ctx, scope, subj, vec, candidateK(k), opts)
	if err != nil {
		return nil, err
	}
	return RerankHits(query, hits, int(k)), nil // <- the line that changes per the D-05 decision
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `reflect.DeepEqual` bit-identity differ check | Cosine-distance-epsilon differ check | This phase (EVAL-01, #353) | Gate now catches "materially the same, jittered by float noise" as well as true symmetry, without false-PASSing on API nondeterminism |
| `os.Getenv` skip guards | Resolved koanf `*config.Config` skip guards | This phase (EVAL-02, #354) | Guard can no longer diverge from the config the embedder under test was actually built from |
| Case-level `wantKey` (one target per case) | Query-level `wantKey` (one target per query, or none) | This phase (RANK-01) | Enables many-query, many-target, and no-answer cases in one fixture entry |
| Lexical-overlap-only `RerankHits` as the sole rerank strategy | A named, pluggable ranker list decided by measured MRR | This phase (RANK-02, #605) | The shipped ranking is chosen by a pre-committed, data-driven rule instead of being permanently pinned to the #261-tuned heuristic |

**Deprecated/outdated:** `store.RerankHits`/`internal/store/rerank.go`'s lexical-overlap logic is a
strong deletion candidate per spike 004's numbers (R@1 0.50 vs vector-only's 0.88 on paraphrases)
— not deprecated yet (the decision rule must run for real before shipping), but the research
strongly suggests D-05's pre-committed rule will select vector-only or a tuned blend/gate over the
shipped lexical baseline.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The recommended `EvalConfig`/`eval.retrieval_eval` koanf key path and struct naming (vs. e.g. putting the field directly on `Config` with no sub-struct) is Claude's discretion, not dictated by CONTEXT.md — presented as a recommendation, not a verified requirement | Config registry (EVAL-02 / D-15) | Low — any reasonable koanf-nested-struct shape satisfies D-15's actual requirement (resolved-config read); the planner may pick a different field/section name with no correctness cost |
| A2 | The blind-subagent procedure's exact mechanics (Task-tool dispatch, prompt shape) are a recommendation, not a verified GSD pattern — no prior phase in this repo implemented this before | Blind-subagent query authorship | Medium — if the recommended mechanism turns out to be impractical inside a `gsd-execute-phase` task, the planner needs a fallback that still satisfies D-01's independence requirement (e.g. a human, rather than a subagent, could serve as the "blind" party) |
| A3 | Deleting `store.RerankHits`/`rerank.go`'s lexical logic is presented as the LIKELY outcome of the D-05 decision rule (based on spike 004's numbers on a different, smaller corpus), not a certainty — the actual Phase 1 corpus (80–120 records, independently-written queries) may produce different numbers | Pluggable ranker design / State of the Art | Medium — if the real eval numbers differ from spike 004's, a tuned blend or gate variant (not vector-only) could win instead; the plan should build for all four D-06 variants, not assume the outcome |

## Open Questions

1. **Exact α / overlap-threshold grid values (D-07 discretion)**
   - What we know: "a small fixed grid (3–4 values each)," tuned against the ≥0.05 MRR margin
     guard.
   - What's unclear: no specific numeric grid is prescribed anywhere in CONTEXT.md or the spikes.
   - Recommendation: the planner/executor should pick a grid AFTER the corpus and paraphrase
     queries exist (e.g. α ∈ {0.1, 0.2, 0.3}, overlap-gate threshold ∈ {0.6, 0.75, 0.9} as a
     starting point informed by spike 004's observation that the shipped reranker over-promotes on
     shared tool words), and record the chosen grid + rationale in the eval's fixture comment.

2. **Whether the paraphrase corpus's synthetic domains should mirror engram's OWN real spine
   domains (tooling, auth, deploy, ...) or be genuinely unrelated to engram**
   - What we know: D-02 requires "multi-domain synthetic set... several domains, e.g. tooling,
     auth, deploy, data model, config, testing" and "no verbatim spine content, no secrets."
   - What's unclear: whether domains should intentionally echo engram's actual spine content
     categories (to be realistic for THIS project's recall patterns) or be deliberately generic/
     project-agnostic.
   - Recommendation: lean toward engram-flavored-but-synthetic (matching D-02's own example domain
     list, which reads like engram's actual subsystems), since the goal is realistic recall
     conditions for THIS codebase's future reranking decisions (Phase 4), not a generic benchmark.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Qdrant (Docker testcontainer or `ENGRAM_QDRANT_TEST_ADDR`) | `task eval:retrieval`, `TestRetrievalEval` | Not probed this session (research is static-analysis only; no live run attempted) | `qdrant/qdrant:v1.19.1` pinned in `storetest.QdrantImage` [VERIFIED: internal/store/storetest/storetest.go:49] | `storetest.Run` already degrades gracefully to skip when neither is available — no fallback needed for THIS phase's CI cost, since the whole package is gated off by default |
| Embedding gateway (`ENGRAM_OPENAI_BASE_URL` + model) | `task eval:retrieval` (live differ gate + real embeds for the corpus) | Not probed this session — this is an operator-supplied live dependency, not a repo-local tool | N/A | None — the eval is explicitly "needs a live Qdrant + gateway" per its own Taskfile `desc:` line; this is expected and by design (an opt-in eval, not part of required CI) |
| Go 1.26.7 toolchain | All code changes in this phase | Present (go.mod pins it) | 1.26.7 [VERIFIED: go.mod:3, read this session] | — |

**Missing dependencies with no fallback:** none that block writing/reviewing the code — the live
Qdrant + gateway dependency is a pre-existing, intentional property of this eval (opt-in,
`ENGRAM_RETRIEVAL_EVAL`-gated), not something this phase needs to newly provision.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package (`go test`), no third-party test framework |
| Config file | none — `Taskfile.yaml` defines `task eval:retrieval` / `task test:go` as the entrypoints |
| Quick run command | `go test ./internal/config/... ./internal/store/...` (the pure, hermetic unit tests — no Qdrant/gateway needed) |
| Full suite command | `ENGRAM_RETRIEVAL_EVAL=1 go test ./internal/retrievaleval/ -run TestRetrievalEval -v` (needs live Qdrant + gateway) plus `go test ./...` for the rest of the required CI job |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|--------------------|--------------|
| EVAL-01 | Differ gate uses cosine epsilon, not `DeepEqual`; NaN/Inf hard-fails | unit (pure function, hermetic) | `go test ./internal/retrievaleval/ -run TestCosineDistance -v` | ❌ Wave 0 — new `cosineDistance` helper + its own unit test (NaN/Inf cases, known-orthogonal/identical/near-identical vector cases) does not exist yet |
| EVAL-01 | `TestRetrievalEval_AsymmetryDiffer` end-to-end asserts the new gate against a real embedder | integration (gated, needs live gateway) | `ENGRAM_RETRIEVAL_EVAL=1 go test ./internal/retrievaleval/ -run TestRetrievalEval_AsymmetryDiffer -v` | ✅ exists, needs editing, not creating |
| EVAL-02 | Resolved-config skip guard reads the SAME config the embedder was built from | unit (hermetic, `t.Setenv` + assert skip/no-skip) | `go test ./internal/retrievaleval/ -run TestRetrievalEval_AsymmetryDiffer -v` (with `t.Setenv` cases for symmetric/asymmetric config) | ❌ Wave 0 — the existing symmetric-config skip has no dedicated unit test today (it is inline in `TestRetrievalEval_AsymmetryDiffer`, not independently tested), and the new resolved-config-based version should get one |
| EVAL-02 | New `Eval.RetrievalEval` registry field parses/defaults correctly | unit | `go test ./internal/config/... -run TestLoad` (extend `config_test.go` with a case) | ❌ Wave 0 — add a `TestEvalRetrievalEvalDefaultAndEnv`-shaped case mirroring `TestOwnerClaimDefaultAndOverride` (config_test.go:177) |
| RANK-01 | New paraphrase case exists, seeded, and its no-answer queries are logged-not-gated | integration (gated) | `ENGRAM_RETRIEVAL_EVAL=1 go test ./internal/retrievaleval/ -run TestRetrievalEval -v` | ✅ existing test, extended with new fixture data |
| RANK-01 | Named-ranker-list variants (vector-only, blend, gate) produce a recall@k/MRR table | unit (hermetic — the ranking functions themselves) + integration (the eval loop that calls them) | `go test ./internal/store/... -run TestCosineBlendRerank -v` / `-run TestOverlapGateRerank -v` for the pure functions; the eval loop itself only runs live | ❌ Wave 0 — both the ranking functions and their unit tests are new |
| RANK-02 | #261 target at rank 1 (tightened bar); shipped paraphrase MRR ≥ vector-only paraphrase MRR | integration (gated) hard `t.Errorf` | `ENGRAM_RETRIEVAL_EVAL=1 go test ./internal/retrievaleval/ -run TestRetrievalEval -v` | ✅ existing test, tightened assertions |

### Sampling Rate

- **Per task commit:** `go test ./internal/config/... ./internal/store/...` (hermetic subset —
  covers the new config field, `cosineDistance`, and any new ranker functions without touching
  Qdrant/network).
- **Per wave merge:** `go test ./...` (the required CI shape — proves the eval package still
  compiles and skips cleanly when the gate is off) plus, if a live Qdrant + gateway is available
  in the dev/CI environment, `task eval:retrieval` itself.
- **Phase gate:** `task eval:retrieval` full green (live) is the actual acceptance evidence for
  RANK-01/RANK-02 per D-09 ("the chosen ranking and the measured table are recorded") — this
  cannot be faked by the hermetic subset alone, since the whole point is measuring real embedder
  behavior.

### Wave 0 Gaps

- [ ] `internal/retrievaleval/*_test.go` (or a new `differ_test.go`) — unit tests for the new
  `cosineDistance` helper: known-identical vectors (distance ≈ 0), known-orthogonal vectors
  (distance ≈ 1), a NaN component (must `t.Fatal`, not silently compare), an Inf component (same).
- [ ] `internal/store/rerank_test.go` — unit tests for the new `CosineBlendRerank` and
  `OverlapGateRerank` exported functions, mirroring the existing `TestRerankHitsPromotesLexicalOverlap`/
  `TestRerankHitsDeterministic`/`TestRerankHitsIgnoresAccessCount` hermetic style (no Qdrant needed
  — pure functions over hand-built `[]Memory` literals).
- [ ] `internal/config/config_test.go` — a case for the new `Eval.RetrievalEval` field's
  default/env-override behavior, mirroring `TestOwnerClaimDefaultAndOverride`.
- [ ] Framework install: none — `go test` is already the framework; no new dependency needed.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | No | This phase touches no auth surface; `subj := store.Authenticated("retrieval-eval@engram.dev")` is an existing, unchanged test fixture identity |
| V3 Session Management | No | N/A — test/eval code only |
| V4 Access Control | No (indirectly touched, unchanged) | `SearchReranked` still delegates to `Search`, which still applies owner/scope authz filtering BEFORE any ranking runs (`internal/store/store.go` — ranking never widens visibility); this phase does not change that ordering, only what happens to an already-authorized result set |
| V5 Input Validation | Marginally yes | The new `Eval.RetrievalEval`/`cfg.Embed.*` reads are config values, not user input — `strconv.ParseBool` on an operator-set env var is the existing, standard idiom (`usage.signals`, `connect.headless`) and needs no new validation beyond what `Config.Validate()` already does (and this new field deliberately needs none, since it is a test-only toggle with no downstream trust boundary) |
| V6 Cryptography | No | No crypto in scope; cosine distance is arithmetic, not cryptographic |

### Known Threat Patterns for this stack

No new threat surface is introduced by this phase — it is test-infrastructure and a pure
in-process ranking function change behind an existing, already-authz-scoped seam. The one
loosely-related pattern worth naming explicitly for the plan-checker:

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| A ranking change accidentally reordering results to surface a record the caller is NOT authorized to see | Information Disclosure | Not possible by construction: `SearchReranked` reranks the result of an already owner/scope-filtered `Search` call (`opts` and `subj` are passed through unchanged to the underlying `Search`); no ranker variant proposed in this research adds a second, unfiltered fetch — verified this session by reading `SearchReranked`'s body, which calls `s.Search` (the authz-filtered path) exactly once before any ranking runs |

## Sources

### Primary (HIGH confidence — read this session)

- `internal/retrievaleval/retrieval_eval_test.go` — full file read, `TestRetrievalEval`,
  `TestRetrievalEval_AsymmetryDiffer`, `TestMain`, `newTestcontainerStore`, `TestSharedQdrantAddressHonored`
- `internal/retrievaleval/fixtures.go` — full file read, `seedRecord`, `retrievalQuery`,
  `retrievalCase`, `gh261Case`, `differProbe`, `recallAtK`, `reciprocalRank`
- `internal/retrievaleval/doc.go` — full file read
- `internal/store/rerank.go` — full file read, `candidateK`, `tokenize`, `lexicalOverlap`, `RerankHits`
- `internal/store/rerank_test.go` — full file read, all 6 existing tests
- `internal/store/store.go` — `SearchOptions` (lines 1126-1166), `Search` (1168-1220), `SearchReranked`
  (1304-1338) read directly
- `internal/store/store_iface.go` — full file read, `memStore` interface
- `internal/server/tools.go` — `loadAndValidate` (196-205), `storeFromConfig` (207-231),
  `StoreAndEmbedderFromEnvNoEnsure` (260-287), `buildDepsFromEnv` (289-327), `buildUsageQueue`
  (374-397, the `strconv.ParseBool` idiom), `searchMemory` (1856-1898) all read directly
- `internal/server/tools_test.go` — `TestStoreAndEmbedderFromEnvNoEnsureValidatesConfig` (5149-5162),
  `TestBuildDepsFromEnvLoadsConfigOnce` (5164-5213),
  `TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce` (5215-5269) all read directly
- `internal/server/connectapi.go` — `SearchMemories` (324-373) read directly
- `internal/config/registry.go` — full file read
- `internal/config/config.go` — `Config` struct (23-38), `EmbedConfig` (62-108), `Load` (349-411)
  read directly
- `internal/config/validate.go` — first 60 lines + function list read directly
- `internal/config/legacy.go` — full file read
- `internal/config/legacy_test.go` — full file read
- `internal/config/identity.go` — full file read (`EmbedderIdentity`, confirming no coupling risk)
- `internal/store/storetest/storetest.go` — full file read
- `Taskfile.yaml` — `eval:retrieval`, `test`, `test:go` targets read directly
- `.planning/spikes/004-jev-rerank-eval/main.go` — full file read
- `.planning/spikes/004-jev-rerank-eval/probe_edges.go` — partial read (corpus + `scores` function)
- `.planning/spikes/004-jev-rerank-eval/README.md` — full file read
- `.claude/skills/spike-findings-engram/SKILL.md` — full file read
- `.claude/skills/spike-findings-engram/references/recall-rerank.md` — full file read
- `.planning/phases/01-eval-foundation-lexical-reranker-fix/01-CONTEXT.md` — full file read
- `.planning/REQUIREMENTS.md` — full file read
- `.planning/STATE.md` (partial — first 312 lines; project-decision history, gotchas)
- `go.mod` — Go version confirmed (1.26.7)
- GitHub issue #605 (`gh issue view 605`), #353 (`gh issue view 353`), #354 (`gh issue view 354`)
  — fetched and read this session for verbatim acceptance criteria and root-cause analysis
- codegraph `explore` output for `internal/retrievaleval` — cross-checked against direct file
  reads (used for blast-radius/caller discovery only, all claims re-verified via `Read`)

### Secondary (MEDIUM confidence)

- None distinct from Primary — every claim in this document traces to a file read or an issue
  fetch performed this session; no WebSearch was used (this is a pure in-repo Go engineering task
  with no external library/framework decision to research).

### Tertiary (LOW confidence)

- The blind-subagent procedure's exact mechanics (Task-tool dispatch shape) — no prior
  implementation exists in this repo to verify against; flagged explicitly in the Assumptions Log
  (A2) and the dedicated section above.

## Metadata

**Confidence breakdown:**
- Standard stack: N/A — no new library/framework; all code is in-repo Go against already-vendored
  dependencies (stdlib `math`/`strconv`/`reflect`, existing `qdrant`/`koanf` already in go.mod)
- Architecture (config registry, ranking seam, fixture shape): HIGH — every claim backed by a
  direct `Read` of the actual source this session, with line numbers and verbatim quotes
- Pitfalls: HIGH — each pitfall traces to a specific, cited existing test/convention this session
  confirmed (e.g. `TestCheckLegacyIgnoresTestOnlyVar`, the single-load invariant test)
- Blind-subagent procedure: LOW — genuinely new process design, no precedent found

**Research date:** 2026-09-22
**Valid until:** No external time-sensitivity (pure in-repo code against a stable Go toolchain and
already-committed dependencies) — treat as valid until the referenced source files change,
detectable at plan time via `git diff` against the line numbers cited above.
