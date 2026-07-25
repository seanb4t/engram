# Phase 26: Structured Citations, Category Filter & Chat Base URL - Pattern Map

**Mapped:** 2026-07-25
**Files analyzed:** 11 (3 code-modified packages, proto+gen, config x3, charts, docs/skill)
**Analogs found:** 11 / 11 (every touched file has an in-repo sibling to copy from; no "no analog" bucket this phase)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/store/store.go` (`payload()` gate split, D-01/D-02) | model (Qdrant payload codec) | transform | same file, `p["summary_model"]`/`p["short_id"]` optional-key idiom (:493-499) | exact |
| `internal/store/store.go` (`categoryMatchCondition` helper, D-09) | utility (filter-condition builder) | transform | same file, `tagMatchConditions` (:760-769) | exact (AND→OR variant) |
| `internal/store/store.go` (`SearchOptions` struct + `Search`/`SearchReranked` reshape, D-09) | model/service (query options) | CRUD | same file, `ListOptions` (:950-976) + `listFilter` (:1002-1029) | exact |
| `internal/server/tools.go` (`storeArgs.Citations` + `toMemory` mapping, D-04) | controller (MCP tool arg struct + mapper) | request-response | same file, `storeArgs.IdempotencyKey` field-embedding precedent (:437) + `storeDiscovery`'s `citationArg→store.Citation` loop (:907-910, verified) | exact |
| `internal/server/tools.go` (`validateCitations` extraction, D-05) | utility (validator) | transform | same file, `validateStoreDiscovery`'s inline loop (:638-656, verified) + `validCitationKind` (:660, verified) | exact |
| `internal/server/tools.go` (`searchArgs.Categories`/`listArgs.Categories`, D-08) | controller (MCP tool arg struct) | request-response | same file, `listArgs.Tags`/`searchArgs.Tags` (:526, :565, verified) — copy field shape, invert AND→OR in jsonschema wording | exact |
| `internal/server/tools.go` (`coreSearchRequest.Categories`, D-09) | model (transport-neutral core request) | transform | same file, `coreListRequest.Categories` (:973, verified — **already exists**) | exact |
| `internal/server/tools.go` (`cmp.Or` chat-base-URL resolution, D-12) | config/wiring | request-response | same file, `summarize.New(...)` call site (:369, verified) beside the untouched `embed.New` (:357) | exact |
| `internal/server/connectapi.go` (`req.Msg.Categories` wiring + `shapeProtoMemories` fix, D-07/D-10) | controller (Connect RPC handler) | request-response | same file, `shapeProtoMemories` (:89-101, verified) + existing `ListMemories` categories wiring (~:155, per RESEARCH) | exact |
| `internal/summarize/summarize.go` (endpoint join via shared helper, D-13) | service (HTTP client) | request-response | `internal/embed/embed.go` `joinEmbeddingsURL` (:125-135, verified) | exact (the direct source of the port) |
| `internal/embed/embed.go` (refactor to call hoisted helper, D-14) | service (HTTP client) | request-response | same file, itself pre-refactor — behavior-preserving | exact |
| `internal/config/config.go` (`OpenAIConfig.ChatBaseURL`, D-12) | config | transform | same file, `OpenAIConfig.EmbeddingsURL` (:107-113, verified) | exact |
| `internal/config/registry.go` (registry row, D-12) | config | transform | same file, `{Key: "openai.embeddings_url", Env: "ENGRAM_OPENAI_EMBEDDINGS_URL"}` (:46, verified) | exact |
| `internal/config/validate.go` (validate-only-when-set, D-15) | config | transform | same file, `EmbeddingsURL` block (:85-97, verified) | exact |
| `proto/engram/v1/engram.proto` (`SearchMemoriesRequest.categories = 8`, D-10) | model (wire schema) | request-response | same file, `ListMemoriesRequest.categories` (field 4, no `buf.validate`, per RESEARCH :59) | exact |
| `charts/engram/values.yaml` + `_helpers.tpl` (chat-base-URL env wiring) | config | request-response | same files, `memory.summarize.*` block (values.yaml:84+, `_helpers.tpl`:29-40, verified) — **NOT** `ENGRAM_OPENAI_EMBEDDINGS_URL` (confirmed absent from chart, Pitfall 6) | role-match (nearest wired analog is the summarize block, not the stated-but-unwired precedent) |
| `skill/engram/curating-memory` + docs-site tool/config pages | docs | n/a | Phase 25's own addition to `curating-memory` (supersede_memory guidance) as the shape for "new capability ships with skill guidance" | role-match |

## Pattern Assignments

### `internal/store/store.go` — `payload()` gate split (D-01)

**Analog:** same function, adjacent optional-key idiom

**Current code being split** (verified, store.go ~:502-512):
```go
if m.Category == "discovery" {
    p["kind"] = m.Kind
    cites := make([]any, len(m.Citations))
    for i, c := range m.Citations {
        cites[i] = map[string]any{
            "kind": c.Kind, "ref": c.Ref, "locator": c.Locator,
            "pin": c.Pin, "excerpt": c.Excerpt,
        }
    }
    p["citations"] = cites
}
```

**Target shape (D-01)** — copy the surrounding write-only-when-set idiom used two lines above for `summary_model`/`short_id`:
```go
// (verified precedent immediately above the block, store.go ~:493-499)
if m.SummaryModel != "" {
    p["summary_model"] = m.SummaryModel
}
...
if m.ShortID != "" {
    p["short_id"] = m.ShortID
}

// split target:
if m.Category == "discovery" {
    p["kind"] = m.Kind
}
if len(m.Citations) > 0 {
    cites := make([]any, len(m.Citations))
    for i, c := range m.Citations {
        cites[i] = map[string]any{
            "kind": c.Kind, "ref": c.Ref, "locator": c.Locator,
            "pin": c.Pin, "excerpt": c.Excerpt,
        }
    }
    p["citations"] = cites
}
```

**No change needed:** `fromPayload` (verified, store.go ~:613-630) already decodes the `citations` list key with no category gate — this is why D-02 insists the write MUST go through `payload()` and nowhere else (round-trip symmetry already exists on the read side).

**Load-bearing constraint (D-02):** do NOT add a targeted `SetPayload({"citations": ...})` anywhere. Every other write path (`Update`, `UpdatePayload`, `Supersede`, `IncrementAccess`, `BackfillShortIDs`, `RemapOwner`, `Reindex`) either round-trips a `fromPayload`-decoded `Memory` back through `payload()` (citations survive for free once this split lands) or uses a targeted single-key `SetPayload` that never references `citations` (citations untouched because never read). Verified by direct inspection per RESEARCH.md Pitfall 2 — do not add "preservation" code to any of those seven functions.

---

### `internal/store/store.go` — `categoryMatchCondition` helper + `SearchOptions` (D-09)

**Analog 1 — the AND-condition-builder shape to mirror (invert to OR):** `tagMatchConditions` (verified, store.go :760-769):
```go
func tagMatchConditions(tags []string) []*qdrant.Condition {
    conds := make([]*qdrant.Condition, 0, len(tags))
    for _, t := range tags {
        if t == "" {
            continue
        }
        conds = append(conds, qdrant.NewMatch("tags", t))
    }
    return conds
}
```

**Analog 2 — the OR-within-AND nesting already proven** in `listFilter` (verified, store.go :1002 region — category block per RESEARCH.md):
```go
func (s *Store) listFilter(scope string, subj Subject, opts ListOptions) *qdrant.Filter {
    must := []*qdrant.Condition{
        qdrant.NewMatch("scope", scope),
        s.ownerOrSharedCondition(subj),
    }
    if len(opts.Categories) > 0 {
        should := make([]*qdrant.Condition, 0, len(opts.Categories))
        for _, c := range opts.Categories {
            should = append(should, qdrant.NewMatch("category", c))
        }
        must = append(must, qdrant.NewFilterAsCondition(&qdrant.Filter{Should: should}))
    }
    ...
    return &qdrant.Filter{Must: must}
}
```
Extract this block's shape into `categoryMatchCondition(categories []string) *qdrant.Condition` (returns the single `NewFilterAsCondition`, or `nil` for empty input) and call it from both `listFilter` and the new `Search`-path filter builder — this is the exact D-11 "no allowlist, OR semantics" behavior, already Qdrant-tested via `TestListCategoryAndVisibilityFilter` (store_test.go, per RESEARCH).

**Analog 3 — the struct-of-filters precedent `SearchOptions` copies:** `ListOptions` (verified, store.go :950-976):
```go
type ListOptions struct {
    ...
    Categories []string // empty = all
    ...
}
```
`SearchOptions{Tags, Categories, CreatedAfter, CreatedBefore}` should mirror this field set (minus `Limit`/`CursorMode`, which don't apply to search). **Do not fold `k` into the struct** — `SearchReranked` deliberately rejects `k==0` as a caller-default-discipline guard (store.go :889-ish), and burying it in a struct would weaken that check (Claude's Discretion note in CONTEXT.md).

**Current signatures being reshaped** (verified):
```go
func (s *Store) Search(ctx context.Context, scope string, subj Subject, vec []float32, k uint64, tags []string, after, before time.Time) (out []Memory, err error)          // store.go:817
func (s *Store) SearchReranked(ctx context.Context, scope string, subj Subject, query string, vec []float32, k uint64, tags []string, after, before time.Time) ([]Memory, error) // store.go:888
```
Reshape to `Search(ctx, scope, subj, vec, k, opts SearchOptions)` / `SearchReranked(ctx, scope, subj, query, vec, k, opts SearchOptions)`. **Blast radius (compiler-verified, budget for it):** 2 production call sites (`SearchReranked`'s internal call to `Search`, store.go :892; `deps.searchMemory`, tools.go :1072) + ~23 test call sites across `store_test.go` (15), `service_principal_isolation_test.go` (4), `rerank_test.go` (1), `instrument_test.go` (2), `internal/retrievaleval/retrieval_eval_test.go` (2 — easy to miss, separate package).

---

### `internal/server/tools.go` — `storeArgs.Citations` + `toMemory` mapping (D-04)

**Analog 1 — the field-embedding precedent this reuses verbatim:** `storeArgs.IdempotencyKey` (verified, tools.go :437-438 doc comment, field at :437):
```go
type storeArgs struct {
    Content   string   `json:"content" jsonschema:"..."`
    ...
    // IdempotencyKey is promoted onto scheduleArgs via Go field embedding
    // (both store_memory and schedule_memory gain it from this single
    // declaration, D-13) — do NOT declare it separately on scheduleArgs.
    IdempotencyKey string `json:"idempotency_key,omitempty" jsonschema:"..."`
}
```
Add `Citations []citationArg` the same way — one declaration on `storeArgs`, inherited by `scheduleArgs` (embeds `storeArgs`, :443) and by `supersede_memory`'s arg struct (which per D-04 also takes the `storeArgs` field set).

**Analog 2 — `citationArg` struct, reuse verbatim** (verified, tools.go — appears just after `scopeArgs`):
```go
type citationArg struct {
    Kind    string `json:"kind" jsonschema:"file|commit|url|repo"`
    Ref     string `json:"ref" jsonschema:"path, repo URL, or doc URL"`
    Locator string `json:"locator,omitempty" jsonschema:"e.g. 200-240 line range"`
    Pin     string `json:"pin,omitempty" jsonschema:"commit SHA, content-hash, @rev, or fetched-at"`
    Excerpt string `json:"excerpt,omitempty" jsonschema:"cached substance (<= ~50 lines)"`
}
```

**Analog 3 — the mapping loop to lift into `toMemory`** (verified shipped in `storeDiscovery`, tools.go :907-910):
```go
cites := make([]store.Citation, len(a.Citations))
for i, cit := range a.Citations {
    cites[i] = store.Citation{Kind: cit.Kind, Ref: cit.Ref, Locator: cit.Locator, Pin: cit.Pin, Excerpt: cit.Excerpt}
}
```

**The actual gap to close — `toMemory` today (verified, tools.go :673-694):**
```go
func (a storeArgs) toMemory(owner, actor string, createdAt time.Time) store.Memory {
    src := store.SummarySourceNone
    if a.Summary != "" {
        src = store.SummarySourceClient
    }
    return store.Memory{
        ID:            uuid.NewString(),
        Content:       a.Content,
        Scope:         a.Scope,
        Repo:          a.Repo,
        Workspace:     a.Workspace,
        Worktree:      a.Worktree,
        BaseDir:       a.BaseDir,
        Source:        a.Source,
        Category:      a.Category,
        Tags:          a.Tags,
        Summary:       a.Summary,
        SummarySource: src,
        Actor:         actor,
        Owner:         owner,
        CreatedAt:     createdAt,
    }
}
```
**This is the field that must gain a `Citations:` line** (mapping `a.Citations` via the loop above into `[]store.Citation`) — it currently has NO citations field and is a hand-written allow-list mapper, so adding `Citations` to `storeArgs` without also editing this function silently drops citations before `Upsert` ever sees them (RESEARCH.md Pitfall 1 — the single highest-value finding of this phase's research). **Non-negotiable regression test:** store a `memory`-category record with citations via `store_memory`, `get_memory` it back, assert citations present.

---

### `internal/server/tools.go` — `validateCitations` extraction (D-05)

**Analog — the loop being extracted verbatim** (verified inside `validateStoreDiscovery`, tools.go :638-656 region):
```go
if len(a.Citations) == 0 {
    return fmt.Errorf("at least one citation is required")
}
if len(a.Citations) > maxDiscoveryCitations {
    return fmt.Errorf("too many citations: %d (max %d)", len(a.Citations), maxDiscoveryCitations)
}
for i, c := range a.Citations {
    if !validCitationKind(c.Kind) {
        return fmt.Errorf("citation %d: kind must be one of file|commit|url|repo, got %q", i, c.Kind)
    }
    if len(c.Excerpt) > maxCitationExcerptBytes {
        return fmt.Errorf("citation %d: excerpt too large: %d bytes (max %d)", i, len(c.Excerpt), maxCitationExcerptBytes)
    }
}
```
**`validCitationKind`, reuse verbatim** (verified, tools.go, just above `toMemory`):
```go
func validCitationKind(k string) bool {
    switch k {
    case "file", "commit", "url", "repo":
        return true
    }
    return false
}
```
**Target shared signature** (per RESEARCH.md Code Examples — copy this shape):
```go
func validateCitations(cites []citationArg, minCount int) error {
    if len(cites) < minCount {
        return fmt.Errorf("at least %d citation(s) required", minCount)
    }
    if len(cites) > maxDiscoveryCitations {
        return fmt.Errorf("too many citations: %d (max %d)", len(cites), maxDiscoveryCitations)
    }
    for i, c := range cites {
        if !validCitationKind(c.Kind) {
            return fmt.Errorf("citation %d: kind must be one of file|commit|url|repo, got %q", i, c.Kind)
        }
        if c.Ref == "" {
            return fmt.Errorf("citation %d: ref is required", i)
        }
        if len(c.Excerpt) > maxCitationExcerptBytes {
            return fmt.Errorf("citation %d: excerpt too large: %d bytes (max %d)", i, len(c.Excerpt), maxCitationExcerptBytes)
        }
    }
    return nil
}
```
**Note (flag in the plan, not silent):** the current shipped loop does NOT check `c.Ref == ""` — this is a small, deliberate behavior tightening being folded into the extraction (D-05 text explicitly calls for it). `validateStoreDiscovery` calls with `minCount=1` (unchanged), the memory path calls with `minCount=0`.

---

### `internal/server/tools.go` — `searchArgs.Categories` / `listArgs.Categories` (D-08)

**Analog — the adjacent `Tags` field to copy the shape of, inverting AND→OR wording** (verified, tools.go :522-539):
```go
type searchArgs struct {
    Query         string   `json:"query"`
    Scope         string   `json:"scope"`
    K             uint64   `json:"k,omitempty"`
    Tags          []string `json:"tags,omitempty" jsonschema:"optional; restrict to records carrying ALL listed tags"`
    Full          bool     `json:"full,omitempty" jsonschema:"..."`
    CreatedAfter  string   `json:"created_after,omitempty" jsonschema:"..."`
    CreatedBefore string   `json:"created_before,omitempty" jsonschema:"..."`
}

type listArgs struct {
    Scope         string   `json:"scope" jsonschema:"..."`
    Limit         uint64   `json:"limit,omitempty" jsonschema:"..."`
    Tags          []string `json:"tags,omitempty" jsonschema:"optional; restrict to records carrying ALL listed tags"`
    ...
}
```
Add `Categories []string \`json:"categories,omitempty" jsonschema:"optional; restrict to records in ANY of the listed categories"\`` to both — **the wording MUST say ANY/OR explicitly**, because the adjacent `tags` field's wording says ALL/AND and an agent will otherwise assume symmetry (D-08 explicit warning).

**`coreListRequest.Categories` already exists — zero new plumbing needed there** (verified, tools.go :973):
```go
type coreListRequest struct {
    Scope         string
    Limit         uint64
    Offset        uint64
    Categories    []string
    Visibility    string
    Tags          []string
    CreatedAfter  time.Time
    CreatedBefore time.Time
    Cursor        string
    CursorMode    bool
}
```
**`coreSearchRequest` does NOT have it yet — this is the real gap** (verified, tools.go :1000-1006):
```go
type coreSearchRequest struct {
    Scope         string
    Query         string
    K             uint64
    Tags          []string
    CreatedAfter  time.Time
    CreatedBefore time.Time
}
```
Add `Categories []string` here, following the exact field-ordering convention `coreListRequest` uses (Categories placed near Tags).

---

### `internal/server/tools.go` — chat-base-URL resolution (D-12)

**Analog — the two adjacent construction call sites** (verified, tools.go :357 and :369):
```go
return embed.New(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey, cfg.Embed.Model, ...)      // :357 — untouched
return summarize.New(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey, cfg.Summarize.Model, summaryMaxChars(cfg), ...) // :369 — change this one
```
Change only the `summarize.New` call: `summarize.New(cmp.Or(cfg.OpenAI.ChatBaseURL, cfg.OpenAI.BaseURL), cfg.OpenAI.APIKey, ...)`. Add `"cmp"` to imports (first use of `cmp.Or` in this repo per RESEARCH.md Standard Stack — confirmed Go 1.26.3 floor is well past the 1.22 `cmp.Or` requirement).

---

### `internal/server/connectapi.go` — `shapeProtoMemories` D-07 fix + `Categories` wiring (D-10)

**Current code — the gap** (verified, connectapi.go :89-101):
```go
func shapeProtoMemories(ms []store.Memory, full bool, maxChars int) []*engramv1.Memory {
    out := make([]*engramv1.Memory, len(ms))
    for i, m := range ms {
        pb := memoryToProto(m)
        if !full {
            summary, _ := summaryOrTruncation(m, maxChars)
            pb.Content = ""
            pb.Summary = summary
        }
        out[i] = pb
    }
    return out
}
```
**Target (D-07 full scope — add `pb.Citations = nil`, consider `pb.Kind = ""`):**
```go
if !full {
    summary, _ := summaryOrTruncation(m, maxChars)
    pb.Content = ""
    pb.Summary = summary
    pb.Citations = nil
    pb.Kind = ""
}
```
This closes the Connect-lane leak RESEARCH.md Pitfall 3 identifies: MCP's `recallView` (summary.go, hand-written allow-list struct with no `Citations` field) already omits citations from the compact view for free; Connect's `shapeProtoMemories` does not, because it starts from the full `memoryToProto` and only clears two of four full-view-only fields. **Required test:** live-Connect test storing a memory with citations, calling `ListMemories`/`SearchMemories` with `full=false`, asserting citations absent.

**`Categories` wiring (D-10):** mirror the existing `ListMemories` categories wiring (per RESEARCH.md, connectapi.go ~:155) onto `SearchMemories` — thread `req.Msg.Categories` into `coreSearchRequest.Categories` the same way `ListMemories` threads it into `coreListRequest.Categories` today.

---

### `internal/summarize/summarize.go` + `internal/embed/embed.go` — shared URL-join helper (D-13/D-14)

**Analog — the exact heuristic to port, verbatim** (verified, embed.go :125-135):
```go
func joinEmbeddingsURL(baseURL string) string {
    trimmed := strings.TrimRight(baseURL, "/")
    switch {
    case strings.HasSuffix(trimmed, "/v1beta/openai"):
        return trimmed + "/embeddings"
    case strings.HasSuffix(trimmed, "/v1"):
        return trimmed + "/embeddings"
    default:
        return trimmed + "/v1/embeddings"
    }
}
```
**Current defect being fixed** (verified, summarize.go :155 — the naive concat, twice-verified in this file at both the original scan line and the re-read):
```go
req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(reqBody))
```
**Target hoisted helper** (suffix-parameterized, per RESEARCH.md Pattern 3 — package placement is planner's discretion; verify no import cycle via `go list -deps` before committing to `internal/config` per RESEARCH.md's A1 caution):
```go
func Join(baseURL, suffix string) string {
    trimmed := strings.TrimRight(baseURL, "/")
    switch {
    case strings.HasSuffix(trimmed, "/v1beta/openai"), strings.HasSuffix(trimmed, "/v1"):
        return trimmed + "/" + suffix
    default:
        return trimmed + "/v1/" + suffix
    }
}
```
Then `embed.go` calls `Join(baseURL, "embeddings")` (replacing `joinEmbeddingsURL`, behavior-preserving) and `summarize.go` calls `Join(baseURL, "chat/completions")` at line 155 in place of the naive concat. **Do not** make `internal/summarize` import `internal/embed` (backwards dependency edge per D-14).

---

### `internal/config/{config.go,registry.go,validate.go}` — `chat_base_url` (D-12/D-15)

**Analog 1 — `config.go`, `OpenAIConfig.EmbeddingsURL`** (verified, config.go :107-113):
```go
type OpenAIConfig struct {
    BaseURL string `koanf:"base_url"`
    APIKey  string `koanf:"api_key"`
    // EmbeddingsURL is the ENGRAM_OPENAI_EMBEDDINGS_URL full-URL override that
    // bypasses joinEmbeddingsURL's shape-aware heuristic and is used verbatim as
    // the embeddings endpoint (D-11). Empty (the default) keeps the heuristic.
    EmbeddingsURL string `koanf:"embeddings_url"`
}
```
Add `ChatBaseURL string \`koanf:"chat_base_url"\`` with a doc comment mirroring `EmbeddingsURL`'s, adapted for "no default; falls back to `BaseURL` when empty, resolved only at the summarizer construction site."

**Analog 2 — `registry.go`, the row** (verified, registry.go :46):
```go
{Key: "openai.embeddings_url", Env: "ENGRAM_OPENAI_EMBEDDINGS_URL"},
```
Add `{Key: "openai.chat_base_url", Env: "ENGRAM_OPENAI_CHAT_BASE_URL"}` adjacent to it.

**Analog 3 — `validate.go`, the validate-only-when-set block** (verified, validate.go ~:85-97):
```go
// EmbeddingsURL is the D-11 operator override: self-gated no-op when empty
// (the default — the join heuristic applies), validated the same way as
// ENGRAM_OPENAI_BASE_URL when set.
if c.OpenAI.EmbeddingsURL != "" {
    switch u, err := url.Parse(c.OpenAI.EmbeddingsURL); {
    case err != nil:
        errs = append(errs, fmt.Errorf("ENGRAM_OPENAI_EMBEDDINGS_URL %q: must be a valid URL: %w", c.OpenAI.EmbeddingsURL, err))
    case u.Scheme != "http" && u.Scheme != "https":
        errs = append(errs, fmt.Errorf("ENGRAM_OPENAI_EMBEDDINGS_URL %q: scheme must be http or https", c.OpenAI.EmbeddingsURL))
    case u.Host == "":
        errs = append(errs, fmt.Errorf("ENGRAM_OPENAI_EMBEDDINGS_URL %q: missing host", c.OpenAI.EmbeddingsURL))
    }
}
```
Copy verbatim with `s/EmbeddingsURL/ChatBaseURL/` and `s/ENGRAM_OPENAI_EMBEDDINGS_URL/ENGRAM_OPENAI_CHAT_BASE_URL/`. **Do NOT** copy the `ENGRAM_OPENAI_BASE_URL` empty-string-is-an-error branch (validate.go ~:80-83) — unlike `BASE_URL`, empty `chat_base_url` is valid (means "inherit").

---

### `proto/engram/v1/engram.proto` — `SearchMemoriesRequest.categories = 8` (D-10)

**Analog — `ListMemoriesRequest.categories`, no `buf.validate` annotation** (per RESEARCH.md, field 4, engram.proto :59):
```proto
repeated string categories = 4;
```
Add `repeated string categories = 8;` to `SearchMemoriesRequest` with the **same absence** of any `buf.validate` constraint. **Do NOT** copy the write-lane `StoreMemoryRequest.category` allowlist (`(buf.validate.field).string = {in: ["decision", "preference", "convention", "gotcha"]}`, engram.proto :112 per RESEARCH.md) onto this field — `discovery`/`rule` are legitimate filter values even though they're not legitimate write values (D-11, Pitfall 5). Run `task proto:gen` and commit the regenerated `gen/` tree in the same commit (buf remote plugins are pinned — regen is byte-identical apart from the new field).

---

### `charts/engram/{values.yaml,_helpers.tpl}` — chat-base-URL chart wiring

**Verified: `ENGRAM_OPENAI_EMBEDDINGS_URL` is absent from both files** — grepped directly, zero matches. The only wired `openai.*` var is `ENGRAM_OPENAI_BASE_URL` (`_helpers.tpl` :5) and `ENGRAM_OPENAI_API_KEY` (:21). This means D-12's own stated structural precedent for chart wiring doesn't actually exist (RESEARCH.md Pitfall 6) — there is nothing to mechanically mirror for the embeddings-URL pattern.

**Analog to use instead — the `memory.summarize.*` block**, since `chat_base_url` is summarizer config, not embedder config (verified, values.yaml ~:84+ comment block; `_helpers.tpl` :29-40):
```yaml
# values.yaml — summarize block (comment excerpt)
# Auto-summary for curated memories. Empty model = disabled ...
```
```gotemplate
{{- with .Values.memory.summarize.model }}
...
{{- with .Values.memory.summarize.maxChars }}
...
{{- with .Values.memory.summarize.maxTokens }}
...
{{- with .Values.memory.summarize.timeout }}
...
```
Add `chatBaseURL` under `.Values.memory.summarize` in `values.yaml` and a matching `{{- with .Values.memory.summarize.chatBaseURL }}` env row (`ENGRAM_OPENAI_CHAT_BASE_URL`) in `_helpers.tpl`, following this exact `{{- with }}`-per-optional-field idiom. This establishes chart coverage the `embeddings_url` sibling never got — a deliberate choice, not a silent gap (call it out in the plan per Pitfall 6's guidance).

---

### `skill/engram/curating-memory` + docs-site — agent-facing guidance

**Analog:** Phase 25's addition of `supersede_memory` guidance to `curating-memory` (engram memory `hkb8bwknpb`'s lesson, cited directly in CONTEXT.md/RESEARCH.md: "a tool with no skill/doc guidance is an incomplete feature"). Read the current `curating-memory` SKILL.md's structure (per-tool guidance sections) and add a citations section in the same style/format, plus a category-filter mention on the `search_memory`/`list_memory` docs-site tool pages and a `chat_base_url` row on the docs-site config page (mirroring however `ENGRAM_OPENAI_EMBEDDINGS_URL` is documented there today, since that's a docs-only precedent even where it's chart-unwired).

## Shared Patterns

### Optional payload key — write only when non-empty
**Source:** `internal/store/store.go` (`summary_model`, `short_id` idiom, ~:493-499)
**Apply to:** the citations gate split in `payload()` (D-01)
```go
if m.SummaryModel != "" {
    p["summary_model"] = m.SummaryModel
}
```

### One declaration on `storeArgs`, three write tools inherit via Go field embedding
**Source:** `internal/server/tools.go` `IdempotencyKey` (Phase 24 D-13 precedent, :437-438)
**Apply to:** `Citations []citationArg` on `storeArgs` (D-04) — `store_memory`, `schedule_memory`, `supersede_memory` all gain it for free; never re-declare on the embedding structs.

### Transport-neutral `core*Request` structs decouple MCP from Connect
**Source:** `internal/server/tools.go` `coreListRequest`/`coreSearchRequest` (:970, :1000)
**Apply to:** every new filter field (Categories) belongs on the `core*Request` struct, never bolted directly onto a lane-specific handler. `coreListRequest.Categories` already exists (zero new work); `coreSearchRequest.Categories` is the one gap.

### OR-within-outer-AND Qdrant filter composition
**Source:** `internal/store/store.go` `listFilter`'s existing category block (~:1007-1013) and `activeWindowConditions` (~:776-790) — the same nesting shape
**Apply to:** the new `Search`-path category filter and its shared `categoryMatchCondition` helper. Authz (`ownerOrSharedCondition`) stays the outer `Must`; every new filter composes strictly inside it — never reorder (SC4).

### Full-URL override + shape-aware fallback join for OpenAI-compatible endpoints
**Source:** `internal/embed/embed.go` `joinEmbeddingsURL` (:125-135) + the `EmbeddingsURL` config/registry/validate trio
**Apply to:** the chat lane end-to-end — `internal/config` field+registry+validate, the hoisted `Join` helper, and `internal/summarize`'s endpoint construction (D-12/D-13/D-14/D-15). This is the single most load-bearing analog in the phase: four different files each copy one facet of the same shipped pattern.

### "A new capability ships with skill/docs guidance in the same PR"
**Source:** Phase 25 (engram memory `hkb8bwknpb`)
**Apply to:** citations (curating-memory skill + memory-record docs page), category filter (search_memory/list_memory tool docs), chat_base_url (config docs page, and a chart-values comment if chart wiring is added).

## No Analog Found

None — every file this phase touches has a direct, verified in-repo analog (either the sibling function being generalized, e.g. `joinEmbeddingsURL`/`EmbeddingsURL`, or the adjacent struct field being copied, e.g. `Tags`/`IdempotencyKey`). The one partial-match case (charts) is documented above with an explicit note that the stated precedent (`ENGRAM_OPENAI_EMBEDDINGS_URL`) does not exist in the chart, and the nearest actually-wired analog (`memory.summarize.*`) is used instead.

## Metadata

**Analog search scope:** `internal/store/store.go`, `internal/server/tools.go`, `internal/server/connectapi.go`, `internal/summarize/summarize.go`, `internal/embed/embed.go`, `internal/config/{config.go,registry.go,validate.go}`, `proto/engram/v1/engram.proto`, `charts/engram/{values.yaml,_helpers.tpl}`.
**Files scanned:** 11 direct reads/greps against the live `main` tree (all line numbers in this document were verified by direct read during pattern mapping, not taken solely from CONTEXT.md/RESEARCH.md claims — two discrepancies were checked and confirmed accurate: `coreListRequest.Categories` already exists; `ENGRAM_OPENAI_EMBEDDINGS_URL` is confirmed absent from both chart files).
**Pattern extraction date:** 2026-07-25
