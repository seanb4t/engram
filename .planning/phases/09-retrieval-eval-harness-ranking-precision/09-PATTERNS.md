# Phase 9: Retrieval Eval Harness & Ranking Precision - Pattern Map

**Mapped:** 2026-07-09
**Files analyzed:** 6 (new/modified, Wave 0 scope per RESEARCH.md; D-07/D-08 files listed but conditional on eval evidence)
**Analogs found:** 6 / 6

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/retrievaleval/retrieval_eval_test.go` (new) | test (integration eval) | request-response (batch of query→search assertions) | `internal/summarize/fidelity_test.go` (env-gated eval) + `internal/store/store_test.go` `TestMain` (Qdrant testcontainer) | exact (structural composite of two proven analogs) |
| `internal/retrievaleval/fixtures.go` (new, or inline) | model/fixture data | batch | `fidelityCases` slice in `internal/summarize/fidelity_test.go` | exact |
| `Taskfile.yaml` (`eval:retrieval` target, modified) | config | batch/CLI | `eval:summary` target, `Taskfile.yaml:52-55` | exact |
| `internal/server/tools.go` (`search_memory` `Description`, modified) | controller (MCP tool registration) | request-response | same file, `search_memory` registration `tools.go:937-941` (documenting in place; no new file) | exact (self-analog) |
| `CLAUDE.md` / `docs-site/.../reference/tools.md` (docs, modified) | docs | n/a | `docs-site/src/content/docs/guides/embedding-instructions.md:126-128` (already documents the score informally) | exact |
| `internal/store/store.go` (`ensureCollection`/`Search`, conditionally modified — D-07 only) | service (store layer) | CRUD / vector query | same file, current dense-only `ensureCollection` (`store.go:132-150`) and `Search` (`store.go:544-573`) | exact (self-analog; extend in place) |
| `cmd/engram/reindex.go` (conditionally modified — D-07 backfill only) | service (operator CLI command) | batch | same file, existing `reindexCmd` (`reindex.go:20-45+`) | exact (self-analog) |
| `internal/config/registry.go` (conditionally modified — D-08 only) | config | n/a | same file, `embed.query_instruction` field (`registry.go:32`) | exact (self-analog) |

No component/UI files — this phase is entirely backend Go.

## Pattern Assignments

### `internal/retrievaleval/retrieval_eval_test.go` (test, integration eval)

**Analogs:** `internal/summarize/fidelity_test.go` (env-gate shape) + `internal/store/store_test.go` (`TestMain`, Qdrant provisioning)

**Env-gate skip pattern** (`internal/summarize/fidelity_test.go:36-46`):
```go
func TestSummaryFidelity(t *testing.T) {
	if os.Getenv("ENGRAM_SUMMARY_EVAL") != "1" {
		t.Skip("set ENGRAM_SUMMARY_EVAL=1 (and the gateway/model env) to run the fidelity eval")
	}
	maxChars, _ := strconv.Atoi(os.Getenv("ENGRAM_SUMMARY_MAX_CHARS"))
	if maxChars <= 0 {
		maxChars = 280
	}
	c := New(os.Getenv("ENGRAM_OPENAI_BASE_URL"), os.Getenv("ENGRAM_OPENAI_API_KEY"), os.Getenv("ENGRAM_SUMMARY_MODEL"), maxChars)
	...
```
Mirror exactly for `TestRetrievalEval`, gated on `ENGRAM_RETRIEVAL_EVAL=1`, reconstructing prod embedder config from `ENGRAM_OPENAI_BASE_URL`/`ENGRAM_OPENAI_API_KEY`/`ENGRAM_EMBED_MODEL`/`ENGRAM_EMBED_DIM`/`ENGRAM_EMBED_QUERY_INSTRUCTION` (D-05a).

**Qdrant testcontainer provisioning** (`internal/store/store_test.go:22-72`):
```go
const qdrantImageTag = "qdrant/qdrant:v1.18.2"

var testQdrantAddr string

func TestMain(m *testing.M) {
	if addr := os.Getenv("ENGRAM_QDRANT_TEST_ADDR"); addr != "" {
		testQdrantAddr = addr
		os.Exit(m.Run())
	}
	startCtx, startCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	container, err := tcqdrant.Run(startCtx, qdrantImageTag)
	if err != nil {
		startCancel()
		fmt.Fprintf(os.Stderr, "qdrant testcontainer unavailable (%v); integration tests will skip — set ENGRAM_QDRANT_TEST_ADDR or start Docker\n", err)
		os.Exit(m.Run())
	}
	testQdrantAddr, err = container.GRPCEndpoint(startCtx)
	startCancel()
	if err != nil {
		terminateQdrant(container)
		fmt.Fprintf(os.Stderr, "qdrant grpc endpoint: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	terminateQdrant(container)
	os.Exit(code)
}
```
`TestMain` is package-scoped — the new `internal/retrievaleval` package needs its own copy (or extraction), not a shared import from `internal/store`.

**End-to-end call shape to exercise** (not `Store.Search` in isolation — go through the handler-equivalent so default-`k` injection is covered; `internal/server/tools.go:704-728`):
```go
func (d *deps) searchMemory(ctx context.Context, a searchArgs) ([]any, error) {
	if a.K == 0 {
		a.K = 8
	}
	...
	vec, err := d.em.EmbedQuery(ctx, a.Query)
	...
	ms, err := d.st.Search(ctx, a.Scope, subj, vec, a.K, a.Tags, after, before)
	...
	return shapeRecall(ms, a.Full, d.summaryMaxChars), nil
}
```
The eval should either import `internal/server` and call an equivalent of `searchMemory`, or replicate: `EmbedQuery` → `Store.Search` → inspect `Memory.Score`, matching what a real MCP client experiences (per RESEARCH.md Open Question 1 recommendation).

**Metrics (no existing precedent — hand-roll, ~10 lines each; RESEARCH.md "Code Examples"):**
```go
func recallAtK(results []store.Memory, wantID string, k int) bool {
	for i, m := range results {
		if i >= k {
			break
		}
		if m.ID == wantID {
			return true
		}
	}
	return false
}

func reciprocalRank(results []store.Memory, wantID string) float64 {
	for i, m := range results {
		if m.ID == wantID {
			return 1.0 / float64(i+1)
		}
	}
	return 0.0
}
```

---

### `internal/retrievaleval/fixtures.go` (fixture data)

**Analog:** `fidelityCases` in `internal/summarize/fidelity_test.go:15-31`
```go
type fidelityCase struct {
	name        string
	content     string
	mustContain []string
}

var fidelityCases = []fidelityCase{
	{
		name:        "decline-suggestion",
		content:     "...",
		mustContain: []string{"DECLINE", "#_top"},
	},
	...
}
```
Mirror shape for `retrievalCase` (per RESEARCH.md "Code Examples"):
```go
type retrievalCase struct {
	name         string
	seedRecords  []seedRecord // Record T + N sticky topical-neighbor distractors
	queries      []string     // Query A, Query B (near-verbatim restatements of Record T)
	wantRecordID string
	wantScoreGap float64      // min separation vs best distractor (D-03)
}
```
Convention: inline Go slice, NOT a `testdata/` directory (no precedent in repo — RESEARCH.md Anti-Patterns).

---

### `Taskfile.yaml` (`eval:retrieval` target)

**Analog** (`Taskfile.yaml:51-54`):
```yaml
  eval:summary:
    desc: Score whether the configured summary model preserves caveats (needs a live gateway+model)
    cmds:
      - ENGRAM_SUMMARY_EVAL=1 go test ./internal/summarize/ -run TestSummaryFidelity -v
```
Copy verbatim shape:
```yaml
  eval:retrieval:
    desc: Score retrieval recall@k/MRR incl. the #261 regression fixture (needs a live Qdrant+gateway)
    cmds:
      - ENGRAM_RETRIEVAL_EVAL=1 go test ./internal/retrievaleval/ -run TestRetrievalEval -v
```

---

### `internal/server/tools.go` (search_memory Description, D-02)

**Analog / site to edit** (`tools.go:937-941`):
```go
mcp.AddTool(s, &mcp.Tool{Name: "search_memory", Description: "Semantic search within a scope. Optionally pass `tags` to restrict to records carrying all listed tags (AND) before ranking. Returns compact summaries by default (id, summary, summary_source, scope, category, tags, created_at); pass `full=true` for full content, or fetch one record in full via get_memory."},
	func(ctx context.Context, _ *mcp.CallToolRequest, a searchArgs) (*mcp.CallToolResult, any, error) {
		hits, err := d.searchMemory(ctx, a)
		return textResult(fmt.Sprintf("%d hits", len(hits))), map[string]any{"memories": hits}, err
	})
```
Add a sentence documenting the always-on `score` field to the `Description` string (prose channel — the only durable one, since the closure is `(*mcp.CallToolResult, any, error)` with `Out=any`, which the go-sdk's `AddTool` explicitly omits from any auto-generated output schema; RESEARCH.md Pitfall 1). Do NOT attempt to change the return type to a concrete struct — that would be inconsistent with all 12 sibling tools.

**Score is already fully plumbed** — no new field, only docs:
- `internal/store/store.go` `Memory.Score` field (comment already present, ~line 136-139): `Score float32 \`json:"score,omitempty"\`` — "Set only on Search results; zero on list/get."
- `internal/store/store.go` `memoriesFromPoints` (~544-587): `m.Score = p.Score`
- `internal/server/summary.go` `recallView.Score` (~line 52): `Score float32 \`json:"score,omitempty"\`` + `toRecallView` (~89-96): `Score: m.Score`
- `internal/server/connectapi.go:41`: `Score: m.Score`

---

### Docs (D-02b)

**Analog:** `docs-site/src/content/docs/guides/embedding-instructions.md:126-128` already informally documents "search_memory results now carry the Qdrant similarity score." Mirror that prose into:
- `CLAUDE.md` "Memory contract (stable)" section (add one clause about the score field to the existing DTO description).
- `docs-site/src/content/docs/reference/tools.md` `search_memory` section (~lines 85-100) — confirmed GAP, currently does not mention the score field.

---

### `internal/store/store.go` (D-07, conditional on eval evidence — hybrid sparse schema)

**Analog / current dense-only baseline** (`store.go:132-150`):
```go
func (s *Store) ensureCollection(ctx context.Context, name string, dim uint64) error {
	exists, err := s.client.CollectionExists(ctx, name)
	if err != nil {
		return err
	}
	if !exists {
		if err := s.client.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: name,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size: dim, Distance: qdrant.Distance_Cosine,
			}),
		}); err != nil {
			return err
		}
	}
	return s.ensureIndexes(ctx, name)
}
```
Current unnamed single dense vector — converting to a named-vector map (`"dense"` + `"sparse"`) is a **breaking schema change** requiring `engram reindex --target <new-collection>`, NOT in-place mutation (RESEARCH.md Pitfall 2). `Search` (`store.go:544-573`, uses `qdrant.NewQuery(vec...)` against the unnamed vector) would need to become a `Prefetch`+`Fusion` query per RESEARCH.md Pattern 3.

`EmbedText` (`store.go:143-151`) already composes `content + tags` for the dense embedder — the same composed text should feed a client-side sparse tokenizer for consistency (Pitfall 3: tokenizer must be identical on document- and query-side, unlike `EmbedQuery`'s asymmetric instruction).

---

### `cmd/engram/reindex.go` (D-07 backfill, conditional)

**Analog** (`reindex.go:20-45`):
```go
var reindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Re-embed memories into a new (new-dimension) collection for embedder migration",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if reindexTarget == "" {
			return fmt.Errorf("--target (new collection name) is required")
		}
		st, dim, em, err := server.StoreAndEmbedderFromEnvNoEnsure()
		...
```
Existing operator ergonomics (`--target`, `--dry-run`, `--resume`) — a hybrid backfill (D-07) should extend this command (or add an equivalent flow) to compute both the dense embedding (existing `Embed`) and a new sparse term vector per record, writing into the new named-vector target collection, then cut over `ENGRAM_QDRANT_COLLECTION` — same operator flow, not a new command.

---

### `internal/config/registry.go` (D-08 reranker config, conditional)

**Analog** (`registry.go:23-32`):
```go
var registry = []field{
	{Key: "server.listen_addr", Env: "ENGRAM_LISTEN_ADDR", Legacy: "MEM_LISTEN_ADDR", Flag: "listen-addr", Default: ":8080"},
	...
	{Key: "embed.query_instruction", Env: "ENGRAM_EMBED_QUERY_INSTRUCTION"},
	{Key: "embed.query_params", Env: "ENGRAM_EMBED_QUERY_PARAMS"},
	...
}
```
New opt-in reranker knobs (`ENGRAM_RERANK_BASE_URL`, `ENGRAM_RERANK_MODEL`, etc.) are single-line additions to this `field` slice — same shape, no `Default` (opt-in, empty = disabled), no `Legacy` (brand-new).

---

## Shared Patterns

### Env-gated live-integration eval
**Source:** `internal/summarize/fidelity_test.go:36-46` (`ENGRAM_SUMMARY_EVAL`)
**Apply to:** `internal/retrievaleval/retrieval_eval_test.go` (`ENGRAM_RETRIEVAL_EVAL`)
```go
if os.Getenv("ENGRAM_RETRIEVAL_EVAL") != "1" {
	t.Skip("set ENGRAM_RETRIEVAL_EVAL=1 (and the gateway/model env) to run the retrieval eval")
}
```
This is the mechanism that satisfies the CI-gating discretion: the eval participates in the already-required `test` job as a no-op skip (protect-main's exact-8-checks constraint), and runs for real only via `task eval:retrieval` locally/nightly.

### Qdrant testcontainer bootstrap
**Source:** `internal/store/store_test.go:22-72`
**Apply to:** any new package needing a live Qdrant (`internal/retrievaleval`), including the `ENGRAM_QDRANT_TEST_ADDR` fast-path override.

### koanf `ENGRAM_` config registry
**Source:** `internal/config/registry.go` (single `field` slice, `Key`/`Env`/`Legacy`/`Flag`/`Default`)
**Apply to:** any new D-08 reranker config surface — never viper, never ad hoc `os.Getenv` outside this registry for durable server config.

### Score plumbing (already shipped, do not re-implement)
**Source:** `internal/store/store.go` `Memory.Score` / `memoriesFromPoints` → `internal/server/summary.go` `recallView.Score` / `toRecallView` → `internal/server/connectapi.go:41`
**Apply to:** REQ-search-similarity-scores is doc-and-test only; do not add new fields or plumbing.

### `AddTool` output-schema constraint
**Source:** `internal/server/tools.go` — all 13 `AddTool` closures uniformly typed `(*mcp.CallToolResult, any, error)`; go-sdk's `AddTool` omits output schema when `Out=any`.
**Apply to:** D-02's "result jsonschema" — satisfy via `Description` prose only, not a struct-tag-driven schema (RESEARCH.md Pitfall 1).

## No Analog Found

None — every file in scope has a direct structural analog already in the codebase (this phase is explicitly designed by CONTEXT.md/RESEARCH.md to mirror existing patterns rather than invent new ones).

## Metadata

**Analog search scope:** `internal/summarize/`, `internal/store/`, `internal/server/`, `internal/config/`, `cmd/engram/`, `Taskfile.yaml`, `docs-site/src/content/docs/`
**Files scanned:** `internal/summarize/fidelity_test.go`, `internal/store/store_test.go`, `internal/store/store.go`, `internal/server/tools.go`, `internal/server/summary.go`, `internal/server/connectapi.go`, `internal/config/registry.go`, `cmd/engram/reindex.go`, `Taskfile.yaml`
**Pattern extraction date:** 2026-07-09
</content>
