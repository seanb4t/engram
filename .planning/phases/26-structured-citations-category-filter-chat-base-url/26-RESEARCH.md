# Phase 26: Structured Citations, Category Filter & Chat Base URL - Research

**Researched:** 2026-07-25
**Domain:** Internal Go seam extension (Qdrant payload/filter shape, MCP+Connect tool surface, OpenAI-compatible HTTP client config) — no new external dependencies, no new libraries.
**Confidence:** HIGH (every claim below was verified by direct inspection of the current `main` source tree, not from training-data recall; this phase introduces zero new third-party packages, so there is no registry/package-legitimacy surface to audit)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Track A — Citations payload shape**
- **D-01**: Split the `payload()` gate at `store.go:502` — `kind` stays discovery-gated; `citations` becomes gated on `len(m.Citations) > 0` for **any** category. No `citations` key written when empty (byte-identical payload to today for non-citation records).
- **D-02 (highest-risk decision)**: The citations write MUST go through `payload()` (the shared whole-payload marshaller), **never** a bespoke targeted `SetPayload`. `fromPayload` already decodes `citations` ungated; `Store.Update` is a whole-payload Upsert. Relaxing `payload()` makes `Update`/`Reindex`/every round-trip preserve citations for free; writing citations outside `payload()` risks the cross-path lost-write class (engram memory `86q25vq6jf`, `m43h2yt97m`).
- **D-03**: No new struct, no new `Memory` field — `Citations []Citation` (store.go:198) and `Citation{Kind,Ref,Locator,Pin,Excerpt}` (store.go:279) already exist; reuse verbatim.

**Track A — Tool surface & validation**
- **D-04**: `Citations []citationArg` declared once on `storeArgs` (tools.go:424) so `store_memory`/`schedule_memory`/`supersede_memory` all inherit it via Go field embedding (the Phase-24 D-13 `idempotency_key` precedent).
- **D-05**: Extract a shared `validateCitations(cites []citationArg, minCount int) error` from `validateStoreDiscovery`'s loop (tools.go:638-656): `kind ∈ {file,commit,url,repo}`, `ref` non-empty, `len(citations) <= maxDiscoveryCitations` (50), `len(excerpt) <= maxCitationExcerptBytes` (16 KiB). Discovery calls with `minCount=1` (unchanged); memory path calls with `minCount=0` (optional).
- **D-06**: Memory citations are stored, never interpreted — not embedded, never affect ranking, never gate recall, not aged/verified. Same posture as discovery citations minus the `>=1` requirement.

**Track A — Recall shaping**
- **D-07**: Citations ride `full=true`/`get_memory` only, omitted from the default compact `search_memory`/`list_memory` view (keeps the session-start spine bootstrap small). `search_discovery` is unchanged (already always returns citations).

**Track B — Category filter**
- **D-08**: Plural `categories []string` (not singular `category`) on `searchArgs`/`listArgs` — zero-impedance match with the already-plural `ListOptions.Categories`/`coreListRequest.Categories`/proto `ListMemoriesRequest.categories` (field 4). **Categories compose as OR** (`Should` sub-filter), unlike `tags` (AND) — the jsonschema description MUST say so explicitly since the adjacent `tags` field says the opposite. A single-element array covers the "singular" reading SC2's text implies; planner may add a singular alias if the verifier reads SC2 literally, but plural is primary.
- **D-09**: Extract a `store.SearchOptions{Tags, Categories, CreatedAfter, CreatedBefore}` struct rather than adding a 9th positional `categories []string` param to `Store.Search`/`SearchReranked` (which would place two adjacent `[]string` params next to each other — a silent-transposition hazard the compiler can't catch). Mirrors the already-shipped `ListOptions` convention exactly. **Escape hatch:** a positional param is acceptable if the refactor threatens the phase, but then `tags`/`categories` MUST NOT be adjacent in the signature.
- **D-10**: Connect `SearchMemories` gains `repeated string categories = 8` (additive proto field) — the goal line says "MCP↔Connect category-filter parity" in **both** directions, not just list-side. `task proto:gen` + committed `gen/` tree in the same commit; buf remote plugins are pinned (engram memory `3tejqw6q3j`) so regen is byte-identical.
- **D-11**: No server-side allowlist on the filter value — an unknown `category` value simply matches nothing (not an error). The write-domain allowlist (`decision|preference|convention|gotcha`, proto `buf.validate` `in:` constraint) must **NOT** be copied onto this read-lane filter field: `discovery` and `rule` are real stored categories and legitimate filter targets.

**Track C — Chat base URL**
- **D-12**: Add `ChatBaseURL string \`koanf:"chat_base_url"\`` to `OpenAIConfig` + registry row `{Key: "openai.chat_base_url", Env: "ENGRAM_OPENAI_CHAT_BASE_URL"}` (no default), adjacent to `openai.embeddings_url`. Resolve with `cmp.Or(cfg.OpenAI.ChatBaseURL, cfg.OpenAI.BaseURL)` **only** at the `summarize.New(...)` call site (tools.go:369); the embedder call site (tools.go:357) is untouched.
- **D-13 (feature-breaking without this)**: Port the shape-aware `/v1` join to the chat lane. `summarize.go:155` currently does a naive `c.baseURL + "/v1/chat/completions"` concat. Every hosted chat base URL ends in `/v1` or `/v1beta/openai`; without the fix, setting the new var to a real provider produces a double `/v1` 404 on first use. Port the exact three-way switch: `…/v1beta/openai` → `+ "/chat/completions"`; `…/v1` → `+ "/chat/completions"`; otherwise `+ "/v1/chat/completions"`. Behavior-preserving for the default `http://localhost:4000` (byte-identical output).
- **D-14**: Hoist the join heuristic to one shared, suffix-parameterized helper (e.g. `Join(baseURL, "embeddings")` / `Join(baseURL, "chat/completions")`) rather than duplicating the switch; refactor `internal/embed` to call it too. **Package placement is planner's discretion.** Do NOT make `internal/summarize` import `internal/embed` (backwards dependency edge).
- **D-15**: Validate `chat_base_url` only when set — mirror the exact `ENGRAM_OPENAI_EMBEDDINGS_URL` idiom (`validate.go:87-101`): skip when empty, else require parseable URL + http/https scheme + non-empty host. Unlike `ENGRAM_OPENAI_BASE_URL`, empty is valid here (means "inherit") — do NOT copy the empty-string-is-an-error branch.

**Cross-cutting**
- **D-16**: SC4 asserted by test, not inspection — a `categories` filter that would match another owner's private record still returns nothing (authz stays outer `Must`); a citation-carrying `shared` record readable by a second actor is still not writable by them.
- **D-17**: Three independent plans, parallelizable (share only `tools.go` arg-struct lines). If one serialization point is wanted, Track B goes first (largest, due to D-09's refactor + D-10's regen).

### Claude's Discretion
- Exact Go names: `SearchOptions` field names, the shared citation-validator signature, the URL-join helper's package/function name, constant renames if `maxDiscoveryCitations` is generalized.
- Whether `SearchOptions` also absorbs `k` — **recommend no**: `SearchReranked` deliberately rejects `k==0` as a caller-default-discipline guard (store.go:889); burying it in a struct weakens that.
- Whether D-07's compact-view citation omission is implemented via clearing the field in the result shaper vs. a dedicated summary-shape helper. **Research finding below: on the Connect lane this is NOT automatic — see Common Pitfall 3.**
- Test-file organization; whether Connect `SearchMemories` categories wiring gets its own parity test or folds into the existing suite.
- Whether `citationArg`/`maxDiscoveryCitations` are renamed now that they serve two categories, or left as-is with a doc comment.

### Deferred Ideas (OUT OF SCOPE)
- Editing citations via `update_memory` (replace-vs-merge semantics undecided; own future design).
- `ENGRAM_OPENAI_CHAT_API_KEY` (per-lane key) — not blocking; local embedders ignore `Authorization`, so the shared key already covers the target scenario.
- Per-lane chat timeout / model options (`ENGRAM_OPENAI_CHAT_TIMEOUT`, chat-side `*_PARAMS`).
- Citations on `store_rule` (rules are normative, not evidence-backed).
- Citation aging/staleness verification for memory citations.
- Idempotency + citations on the Connect *write* lane (REQUIREMENTS.md Deferred — unrelated to D-10, which is a read-lane filter field).
- Category filter on `list_scheduled`/`search_discovery`.
- Pre-existing unpinned CI tools (`actionlint@latest`, `task@latest`) — tracked separately.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-memory-citations | A curated `memory`-category record may optionally carry structured provenance using the existing discovery `Citation` shape verbatim; `payload()` write gate relaxed from discovery-only to any category; `kind` stays discovery-specific; citations optional, never auto-populated. | Confirmed exact gate location (`store.go:502-512`), confirmed `fromPayload` already decodes ungated (`store.go:613-630`), confirmed every downstream write path (`Update`, `UpdatePayload`, `Supersede`, `Reindex`, `BackfillShortIDs`, `RemapOwner`, `IncrementAccess`) preserves citations by construction — see Common Pitfalls 1-2 and Code Examples. Found the ONE gap: `storeArgs.toMemory()` (tools.go:673-695) does not currently map any citations field, so D-04's `storeArgs.Citations` addition MUST be paired with a `toMemory` edit or the wire field is silently dropped before Upsert. |
| REQ-category-filter | `search_memory`/`list_memory` accept an optional `category` filter over the MCP surface, hard Qdrant pre-filter before vector ranking, parity with Connect's `ListMemories`. | Confirmed `listFilter` already builds the OR-`Should`-inside-`Must` category filter (`store.go:1002-1029`) and it is live-Qdrant-tested (`TestListCategoryAndVisibilityFilter`, store_test.go:1631). Confirmed zero list-side store work remains. Enumerated every `Store.Search`/`SearchReranked` caller for the D-09 refactor blast radius — see Common Pitfall 4. Confirmed the Connect write-lane category allowlist (`buf.validate` `in:` constraint, engram.proto:112) must NOT be copied onto the new read-lane filter field (D-11). |
| REQ-chat-base-url | Chat/summarize client targets a base URL distinct from the embedder's; falls back to `ENGRAM_OPENAI_BASE_URL` when unset; resolved only in the summarizer path. | Confirmed the naive concat defect at `summarize.go:155` and the exact shape-aware precedent at `embed.go:125-135` (`joinEmbeddingsURL`). Confirmed the exact registry/validate mirror points (`registry.go:46`, `validate.go:87-97`). Confirmed neither the Helm chart nor `ENGRAM_OPENAI_EMBEDDINGS_URL` (its own direct precedent) is currently wired into `charts/engram` — see Common Pitfall 6. |
</phase_requirements>

## Summary

This phase has no new-technology risk: it extends three already-shipped seams (the discovery
`Citation` payload codec, the `ListOptions.Categories` OR-filter, and the embedder's shape-aware
base-URL join) into places they don't yet reach. All three tracks were fully traceable in the
existing code before writing a single line — every file/line reference in CONTEXT.md's `<code_context>`
was independently re-verified against `main` during this research pass and confirmed accurate.

The single highest-leverage finding this research adds beyond CONTEXT.md: **citations are already
safe on every documented write path except one** — `storeArgs.toMemory()` (the function that turns
wire args into a `store.Memory` for `store_memory`/`schedule_memory`/`supersede_memory`) currently
has no citations mapping at all. `Update`, `UpdatePayload`, `Supersede`, `Reindex`,
`BackfillShortIDs`, `RemapOwner`, and `IncrementAccess` were all read line-by-line and every one
either (a) round-trips an already-`fromPayload`-decoded `Memory` struct through `payload()` again
(citations survive automatically once D-01/D-02 land), or (b) writes via a **targeted** `SetPayload`
touching only its own owned keys (citations untouched because never referenced). The only place a
citations value can be silently lost is if `toMemory` is not updated alongside `storeArgs` — this
is the D-04 implementation's real risk, not a hidden lost-write path elsewhere in the store.

The second finding: **D-07 (compact-view citation omission) is already free on the MCP lane but is
a real gap on the Connect lane.** MCP's `recallView` (summary.go:40-59) is a hand-written
allow-list struct with no `Citations` field, so `shapeRecall`'s non-`full` branch already omits
citations with zero new code. But Connect's `shapeProtoMemories` (connectapi.go:89-101) only
clears `Content` and substitutes `Summary` when `!full` — it does **not** clear `pb.Citations` (or
`pb.Kind`). Once memory-category records can carry up to 50 citations with 16 KiB excerpts each,
a Connect `ListMemories`/`SearchMemories` call with `full=false` would leak the full citation
payload into what is supposed to be the token-cheap default view. This is pre-existing behavior
(discovery records already have this gap today, it's just currently low-impact because discoveries
rarely surface via `ListMemories`), but Track A's payload-gate relaxation makes it a much more
consequential omission once ordinary curated memories adopt citations. The planner should treat
closing this as in-scope for D-07, not a pre-existing bug to defer.

Third: `Store.Search`/`SearchReranked` have roughly two dozen call sites once tests are counted
(2 production, ~23 test). D-09's `SearchOptions` struct refactor is compiler-verified but touches
every one of them — sized honestly in Common Pitfall 4 below.

Fourth: the Helm chart never wired `ENGRAM_OPENAI_EMBEDDINGS_URL` (Phase 13's own direct precedent
for this exact "operator override" pattern) into `charts/engram/values.yaml`/`_helpers.tpl` — so
"update charts/engram" per CONTEXT.md's integration-points list is not actually mirroring an
existing wired-in precedent; it would be establishing new coverage the sibling var never got. The
planner has a real choice here, not just a mechanical copy — see Common Pitfall 6.

**Primary recommendation:** Implement the three tracks exactly as CONTEXT.md's D-01–D-17 specify;
this research changes nothing about that design. The two required *additions* to the plan are (1)
map `storeArgs.Citations` into `store.Memory.Citations` inside `toMemory` (D-04's actual
implementation surface), and (2) clear `Citations`/`Kind` in `shapeProtoMemories` when `!full` on
the Connect lane (D-07's actual full scope).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Citation payload storage/decode | Database / Storage (Qdrant payload codec, `internal/store`) | — | Citations are a payload field on the existing single Memory collection (DEC-2bv); no new collection, no new index. |
| Citation validation (kind/size/count) | API / Backend (`internal/server/tools.go`) | — | Resource-exhaustion guards belong at the request-boundary handler, same tier as the existing discovery validator. |
| Citation wire shape (MCP + Connect) | API / Backend | — | `citationArg`/proto `Citation` are transport-layer shapes; both already exist and are reused verbatim. |
| Category filter (Qdrant pre-filter) | Database / Storage (`internal/store` filter builders) | — | `listFilter`/the new `Search` filter compose the authz+category Qdrant `Filter`; this is exclusively a store-layer concern, same tier as the existing tag/visibility/window filters. |
| Category filter arg surface (MCP + Connect) | API / Backend | — | `searchArgs`/`listArgs`/proto field additions are transport-neutral core-request plumbing, not a storage concern. |
| Chat base-URL resolution | API / Backend (`internal/server/tools.go` construction site) + supporting library (`internal/summarize`) | — | Config resolution (`cmp.Or`) happens once at server-wiring time (API/Backend tier); the URL-join heuristic itself lives in a small supporting library package, not a network/CDN concern. |
| Chat-endpoint HTTP call | API / Backend (outbound to an external OpenAI-compatible gateway) | — | `internal/summarize.Client` makes an outbound HTTP call from the server process; there is no browser/CDN tier in this system. |

engram has no browser/CDN tier for this phase's scope (it is a headless MCP+Connect server); the
five-tier table above collapses to two real tiers here — API/Backend and Database/Storage — which
matches every other v0.11.x phase's shape.

## Standard Stack

No new libraries. This phase extends existing seams using only stdlib (`cmp`, `net/url`,
`strings`) plus already-vendored dependencies.

### Core (unchanged, verified current)
| Library | Version (go.mod) | Purpose | Why unchanged |
|---------|---------|---------|--------------|
| `github.com/qdrant/go-client` | v1.18.3 | Qdrant filter/payload primitives (`qdrant.NewFilterAsCondition`, `qdrant.NewMatch`, `qdrant.NewValueMap`) | Same client already used for `activeWindowConditions`/`listFilter`'s existing OR-inside-AND pattern (store.go:781-789, 1007-1013) — no version bump needed, the pattern is already proven. |
| `connectrpc.com/connect` | v1.20.0 | Connect RPC framework | Additive proto field only; no client/server library change. |
| buf remote plugins (`buf.build/protocolbuffers/go:v1.36.11`, `buf.build/connectrpc/go:v1.20.0`, `buf.build/bufbuild/es:v2.12.1`) | pinned in `buf.gen.yaml` | Codegen for the new `categories` field | Pinned per engram memory `3tejqw6q3j` — regeneration is byte-identical apart from the new field; do not bump versions in this phase. |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| stdlib `cmp` | go1.26.3 (go.mod) | `cmp.Or(cfg.OpenAI.ChatBaseURL, cfg.OpenAI.BaseURL)` fallback resolution (D-12) | `cmp.Or` has been available since Go 1.22; this repo is on 1.26.3, well past that floor — no version gate needed. Not yet used elsewhere in this repo (first use), but it is a standard-library function, not a new dependency. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `SearchOptions` struct (D-09) | 9th positional param on `Store.Search`/`SearchReranked` | Rejected per D-09: two adjacent `[]string` params (`tags, categories`) is a silent-transposition hazard across ~3 production call sites the compiler cannot catch. Struct is the repo's own established pattern (`ListOptions`). |
| Shared `Join` helper package (D-14) | Duplicate the 3-way switch in `internal/summarize` | Rejected: a provider-shape heuristic duplicated across two packages is exactly the kind of logic that silently drifts when a 4th provider shape appears. |

**Installation:** none — no `go get`/`npm install` required for this phase.

**Version verification:** `go.mod` was read directly (not training-recall) — `qdrant/go-client v1.18.3`, `connectrpc.com/connect v1.20.0`, `go 1.26.3`. `buf.gen.yaml` pins were read directly — `protocolbuffers/go:v1.36.11`, `connectrpc/go:v1.20.0`, `bufbuild/es:v2.12.1`. `go tool buf --version` on this machine resolves to 1.72.0 (the CLI, distinct from the pinned remote plugin versions above).

## Package Legitimacy Audit

**N/A for this phase — no new external packages are installed.** All three tracks use only Go
stdlib (`cmp`, `net/url`, `strings`, `encoding/json`) and already-vendored dependencies
(`qdrant/go-client`, `connectrpc.com/connect`, the pinned buf remote plugins). Nothing to run
`package-legitimacy check` against.

## Architecture Patterns

### System Architecture Diagram

```
                         MCP tool call                    Connect RPC call
                    (store_memory / search_memory /       (SearchMemories /
                     list_memory / supersede_memory)        ListMemories)
                              |                                   |
                              v                                   v
                    +-------------------+               +-------------------+
                    |  tools.go args    |               | connectapi.go req |
                    |  (storeArgs w/    |               | (proto Search/List |
                    |  Citations field, |               |  Request w/        |
                    |  searchArgs/      |               |  categories field) |
                    |  listArgs w/      |               |                    |
                    |  Categories)      |               |                    |
                    +--------+----------+               +---------+---------+
                             |                                     |
                             v                                     v
                    +--------------------------------------------------------+
                    |         transport-neutral core (deps.*)                |
                    |  coreSearchRequest / coreListRequest (+Categories)      |
                    |  storeArgs.toMemory() (+Citations mapping — NEW)        |
                    +--------------------------+-------------------------------+
                                                |
                                                v
                    +--------------------------------------------------------+
                    |                 internal/store (Qdrant)                |
                    |                                                        |
                    |  payload()/fromPayload() codec:                       |
                    |    citations gated on len()>0 for ANY category (D-01)  |
                    |                                                        |
                    |  listFilter() / new Search filter:                    |
                    |    outer Must[ scope, ownerOrSharedCondition ]         |
                    |      -> AND categoryMatchCondition (Should-wrapped OR) |
                    |      -> AND tagMatchConditions (AND)                   |
                    |      -> AND activeWindowConditions                    |
                    |      -> AND NewIsEmpty("superseded_by")                |
                    +--------------------------+-------------------------------+
                                                |
                                                v
                                         Qdrant collection
                                       (single Memory collection,
                                        DEC-2bv — no new index)

  Separate, unrelated flow (Track C):

  cmd/engram serve --------> tools.go summarizerFromConfig()
                                  cmp.Or(ChatBaseURL, BaseURL)
                                       |
                                       v
                              internal/summarize.Client
                                  Join(baseURL, "chat/completions")
                                  (shared heuristic w/ internal/embed)
                                       |
                                       v
                          external OpenAI-compatible /chat/completions
```

A reader can trace SC1 (citations) top-to-bottom on the left path, SC2 (category filter) via the
`categoryMatchCondition` box in the middle, and SC3 (chat base URL) via the fully separate bottom
flow — the three tracks genuinely never intersect except at `tools.go`'s arg-struct declarations,
matching D-17.

### Recommended Project Structure

No new packages/directories for Tracks A and B (all changes land in existing files:
`internal/store/store.go`, `internal/server/tools.go`, `internal/server/connectapi.go`,
`proto/engram/v1/engram.proto`, `gen/`). Track C's D-14 helper is the only candidate for a new
file:

```
internal/
├── store/store.go        # D-01 payload() gate split; D-09 SearchOptions + filter thread
├── server/
│   ├── tools.go           # D-04 storeArgs.Citations + toMemory mapping; D-05 validateCitations;
│   │                      #   D-08 searchArgs/listArgs.Categories; D-12 cmp.Or resolution
│   └── connectapi.go      # D-10 categories wiring on SearchMemories; D-07 shapeProtoMemories fix
├── summarize/summarize.go # D-13 shape-aware endpoint join (via the D-14 shared helper)
├── embed/embed.go         # D-14 refactored to call the shared helper (behavior unchanged)
├── config/
│   ├── config.go          # D-12 OpenAIConfig.ChatBaseURL field
│   ├── registry.go        # D-12 registry row
│   └── validate.go        # D-15 validate-only-when-set block
└── <new small package or internal/config>/  # D-14 shared Join(baseURL, suffix) helper — planner's
                                              # discretion on placement
proto/engram/v1/engram.proto  # D-10 SearchMemoriesRequest.categories = 8
gen/                            # regenerated in the same commit (task proto:gen)
```

### Pattern 1: Payload-key gate split (D-01), verified against the live code

**What:** `payload()` currently writes `kind` and `citations` together, gated on
`m.Category == "discovery"`. Split into two independent conditionals.

**When to use:** Exactly this phase's Track A.

**Verbatim current code** (store.go:502-512, read directly from `main`):
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

**Target shape (D-01):**
```go
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
No change is needed to `fromPayload` (store.go:613-630) — it already decodes the `citations` list
key with no category check.

### Pattern 2: OR-within-AND Qdrant filter — already proven, reuse verbatim (D-09/D-11)

**What:** `listFilter` already does exactly what the new `Search`-path category filter needs.

**Verified current code** (store.go:1002-1029):
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
`qdrant.NewFilterAsCondition(&qdrant.Filter{Should: [...]})` appended to the outer `must` slice
produces exactly "OR across the listed categories, ANDed with everything else in `must`" — this is
the *identical* nesting pattern `activeWindowConditions` (store.go:776-790) already uses for the
`not_before`/`not_after` gate, and it is live-Qdrant-tested by `TestListCategoryAndVisibilityFilter`
(store_test.go:1631-1669), which explicitly asserts the OR semantics across two categories against
a real Qdrant testcontainer. **This directly answers "Where research adds the most value" item 1:**
the composition is proven correct in production code today, not a novel construction the planner
needs to verify against `qdrant-go-client` v1.18.3's docs — it's the same client version, same
`NewFilterAsCondition` helper, same nesting shape already shipped and tested.

A `categoryMatchCondition(categories []string) *qdrant.Condition` helper (returning the single
`NewFilterAsCondition` condition, or `nil` for empty input) extracted from this block and shared by
`listFilter` and the new `Search`-path filter builder is the natural refactor — mirrors
`tagMatchConditions`'s existing shape (store.go:760-769) except returning one condition instead of
N.

### Pattern 3: Shape-aware base-URL join — the exact heuristic to port (D-13)

**Verified current code** (embed.go:125-135, the pattern D-13/D-14 generalize):
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
**Verified current defect** (summarize.go:155):
```go
req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(reqBody))
```
Generalize to a suffix-parameterized helper (D-14):
```go
// e.g. in a new small package, or internal/config (already imported transitively by
// internal/embed via ReservedParamKeys — see embed.go:144's existing cross-package comment)
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
Then `embed.go` calls `Join(baseURL, "embeddings")` and `summarize.go` calls
`Join(baseURL, "chat/completions")`. **Caution:** `internal/embed` already has an established
reason to avoid importing certain things to prevent an import cycle (embed.go:137-144: `embed`
aliases `config.ReservedEmbedParamKeys` rather than defining its own, specifically because
`internal/embed` already imports `internal/telemetry`, which imports `internal/config` — a direct
`internal/config -> internal/embed` edge would cycle). Placing the `Join` helper in
`internal/config` is therefore safe for `embed` (no new edge — config already flows into embed
transitively) but adds a **new** `internal/summarize -> internal/config` edge; verify this does not
cycle before committing to that placement (a quick `go list -deps` check is sufficient — this
research did not find any existing `internal/config -> internal/summarize` edge, so it should be
safe, but the planner should verify at implementation time rather than assume).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| OR-semantics category filter | A new Qdrant condition-building approach | `qdrant.NewFilterAsCondition(&qdrant.Filter{Should: [...]})` appended to the outer `Must` — exactly `listFilter`'s existing pattern | Already proven correct and live-tested; a parallel invention risks a subtly different (and untested) nesting. |
| Chat-endpoint URL construction | A second bespoke provider-shape heuristic in `internal/summarize` | The hoisted `Join` helper (D-14), shared with `internal/embed` | Two independently-maintained provider-shape switches is exactly the kind of logic that silently diverges when a 4th provider shape shows up (Gemini's `/v1beta/openai` was itself a "surprise" shape added after the original heuristic shipped). |
| Citation validation (kind enum, size caps, count cap) | A second validator for the memory-citation path | `validateCitations(cites, minCount)` extracted from `validateStoreDiscovery`'s existing loop | Resource-exhaustion guards are the *same* guards regardless of which category the citations attach to — duplicating them is how they drift (e.g. one path's excerpt cap silently changes while the other doesn't). |

**Key insight:** every "don't hand-roll" item in this phase is "don't hand-roll a *second* copy of
something this repo already built correctly once." There is no external-library candidate to reach
for here — the correct answer in every case is internal code reuse.

## Common Pitfalls

### Pitfall 1: Citations silently dropped because `toMemory()` isn't updated (the real D-04 risk)

**What goes wrong:** `store_memory`/`schedule_memory`/`supersede_memory` all funnel through
`storeArgs.toMemory(owner, actor, createdAt) store.Memory` (tools.go:673-695) to build the
`store.Memory` that gets embedded and Upserted. This function currently maps `Content, Scope,
Repo, Workspace, Worktree, BaseDir, Source, Category, Tags, Summary, SummarySource, Actor, Owner,
CreatedAt` — **it does not touch a Citations field, because storeArgs doesn't have one yet.** If
the planner adds `Citations []citationArg` to `storeArgs` (D-04) but forgets to also add the
mapping loop inside `toMemory` (the same 3-line loop `storeDiscovery` already uses at
tools.go:907-910), the wire schema will advertise `citations`, `json.Unmarshal` will happily
populate `storeArgs.Citations`, validation will pass — and the citations will vanish silently
before `Upsert` ever sees them, because `toMemory`'s returned `Memory{}` literal simply never sets
the field (its zero value, `nil`).

**Why it happens:** `toMemory` is a hand-written allow-list mapper (same shape/risk class as
`recallView` in Pitfall 3 below) — adding a wire field to `storeArgs` does not automatically
populate the corresponding `store.Memory` field.

**How to avoid:** The plan's Track A tasks MUST include an explicit edit to `toMemory` (not just
`storeArgs`), and the test suite MUST include a regression test that stores a `memory`-category
record with citations via `store_memory`, then `get_memory`s it back and asserts citations are
present (this is the same test D-02's "single most important thing" note already calls for, but
this pitfall makes explicit which *function* it is protecting against, not just the storage codec).

**Warning signs:** A `store_memory` call with `citations` in the request that "succeeds" (no
error) but `get_memory` returns an empty citations array — that's this exact bug, not a Qdrant
issue.

### Pitfall 2: Update/Reindex/Supersede DO already preserve citations — verify, don't re-derive from scratch

**What goes wrong (if NOT understood):** A cautious implementer might assume every write path
needs an explicit "preserve citations" patch, and spend time adding unnecessary defensive code (or
worse, adding a bespoke `SetPayload` "citations preservation" call that itself becomes a second
write path — the exact anti-pattern D-02 forbids).

**What is actually true (verified by reading every listed function):**
- `Store.Update` (store.go:1578-1641): re-fetches `Supersedes`/`SupersededBy` under a lock, mutates
  `cur.Content/Visibility/Tags/Summary/AccessCount/LastAccessedAt`, then calls
  `s.Upsert(ctx, cur, vec)`. `cur` came from `FetchForUpdate` → `Get` → `fromPayload` (ungated
  citations decode) and `Update`'s body never touches `cur.Citations` — it survives the whole-payload
  re-Upsert **automatically**, once D-01/D-02 land, with zero additional code.
- `Store.UpdatePayload` (store.go:1700-1780): a **targeted** `SetPayload` writing only
  `visibility/summary/summary_source/access_count/last_accessed_at` — never reads or writes a
  `citations` key at all, so it can't drop what it never touches.
- `Store.Supersede` (store.go:1838-1885): the new record is a normal `Upsert(newMem, vec)` (citations
  survive if `toMemory`/mapping upstream populated `newMem.Citations` — see Pitfall 1); the target
  back-stamp is a **targeted** single-key `SetPayload{"superseded_by": ...}` — doesn't touch the
  target's own citations.
- `Store.IncrementAccess` (store.go:1896+): targeted `SetPayload` on `access_count`/`last_accessed_at`
  only.
- `Store.BackfillShortIDs` (store.go:2142-2198) and `Store.RemapOwner` (store.go:2351-2401): both
  targeted `SetPayload` on `short_id` / `owner` only.
- `Store.Reindex` (store.go:2517+): scrolls with `WithPayload: true`, decodes each point via
  `fromPayload` (line 2611, ungated), then re-embeds and re-Upserts through `payload()` again —
  citations round-trip automatically post-D-01.

**How to avoid:** Do not add any new preservation logic to these seven functions. The plan's
verification step should be a **test**, not a code change, for all of them except the one real gap
(Pitfall 1).

**Warning signs:** A plan task that proposes touching `UpdatePayload`, `IncrementAccess`,
`BackfillShortIDs`, or `RemapOwner` to "preserve citations" is very likely unnecessary scope
creep — verify against the actual function body (all four use targeted `SetPayload`) before adding
such a task.

### Pitfall 3: D-07's compact-view omission is free on MCP, NOT free on Connect

**What goes wrong:** Assuming "citations ride full=true only" is already satisfied everywhere
because it's satisfied on the MCP lane.

**Why it happens (verified):** MCP's compact shaper is `toRecallView` (summary.go:95-105), which
builds a **hand-written allow-list struct** `recallView` (summary.go:40-59) that has no `Citations`
field — so `shapeRecall`'s `!full` branch (summary.go:83-93) already omits citations with zero new
code, by construction. But Connect's compact shaper, `shapeProtoMemories` (connectapi.go:89-101),
works differently: it takes the **full** `memoryToProto(m)` (which already sets
`Citations: citationsToProto(m.Citations)` at connectapi.go:66) and then only clears two fields
when `!full`:
```go
if !full {
    summary, _ := summaryOrTruncation(m, maxChars)
    pb.Content = ""
    pb.Summary = summary
}
```
`pb.Citations` (and `pb.Kind`) are never cleared. This is pre-existing behavior (a discovery record
surfaced via `ListMemories` with a category filter already leaks its citations into the compact
view today), but Track A converts it from a low-impact discovery-only quirk into a real problem:
once ordinary `decision`/`preference`/`convention`/`gotcha` records can carry up to 50 citations
with 16 KiB excerpts each, a Connect `ListMemories`/`SearchMemories` call with `full=false` (the
default) would return that full citation payload anyway — defeating the entire point of the
compact view (token-cheap default recall) for exactly the use case (curated memories) this REQ is
designed to serve.

**How to avoid:** Add `pb.Citations = nil` (and consider `pb.Kind = ""`, matching the existing
discovery-only field comment at engram.proto:38-39 that will need updating regardless) inside
`shapeProtoMemories`'s `if !full` branch. This is a small, mechanical addition but it is *in scope*
for D-07 to be true on both transports, and CONTEXT.md's D-07 text (written before this file-level
comparison) only describes the MCP-side mechanism.

**Warning signs:** A live-Connect test that stores a memory with citations, calls
`ListMemories`/`SearchMemories` with `full=false`, and asserts the citations are absent from the
response — if this test is missing from the plan, this gap will ship unnoticed (the MCP-side
equivalent test would pass trivially and mask the Connect-side gap).

### Pitfall 4: `SearchOptions` refactor blast radius — every call site, enumerated

**What goes wrong:** Underestimating how many places `Store.Search`/`Store.SearchReranked`'s
signature change touches, leading to an incomplete or under-time-boxed task.

**Enumerated production call sites (2):**
- `internal/store/store.go:892` — `SearchReranked` calling its own `Search` internally.
- `internal/server/tools.go:1072` — `deps.searchMemory` calling `SearchReranked` (the ONE call
  site both MCP's `search_memory` and Connect's `SearchMemories` funnel through, per the
  transport-neutral core design — so Connect does NOT have a separate direct call site to update).

**Enumerated test call sites (grep-verified, ~23 total):**
- `internal/store/store_test.go` — 15 direct `s.Search(...)` calls across isolation, tag-filter,
  count, and adjacency tests (lines 443, 462, 471, 544, 589, 623, 1202, 1262, 1269, 1433, 1880,
  2578, 2623, 3068, 3963, 3995, 4030, 4073 — several tests call it more than once).
- `internal/store/service_principal_isolation_test.go` — 4 calls (lines 64, 105, 137, 179).
- `internal/store/rerank_test.go` — 1 `SearchReranked` call (line 42).
- `internal/store/instrument_test.go` — 2 calls (lines 93, 170).
- `internal/retrievaleval/retrieval_eval_test.go` — 1 `SearchReranked` call (line 123) + 1 `Search`
  call (line 184, the "ceiling" computation). `internal/retrievaleval/fixtures.go` references
  `SearchReranked` only in a doc comment, not a call — not a real call site.

**Easy-to-miss caller:** `internal/retrievaleval` is a *separate package* from `internal/store` and
`internal/server` — it is easy to search only `internal/store`/`internal/server` and miss this
package's 2 call sites entirely, since the retrieval-eval harness is not part of the normal
request-handling code path and is easy to forget exists.

**How to avoid:** A signature change to `Search`/`SearchReranked` will fail to compile at every one
of the ~25 sites above until updated — Go's compiler makes this exhaustive-by-construction (this is
exactly why D-09 calls the refactor "fully compiler-verified"), but the planner should still budget
for touching ~25 call sites' argument lists, not just the 2 production ones, when sizing the task.

**Warning signs:** none at runtime — this fails at `go build`/`go vet` time, which is the safety
net D-09 relies on. The risk is entirely in *task sizing*, not correctness.

### Pitfall 5: The write-lane category allowlist must not leak onto the new filter field (D-11)

**What goes wrong:** Copying `StoreMemoryRequest.category`'s `buf.validate` constraint
(`(buf.validate.field).string = {in: ["decision", "preference", "convention", "gotcha"]}`,
verified at engram.proto:112) onto the new `SearchMemoriesRequest.categories` field "for
consistency."

**Why it's wrong:** Verified: `discovery` and `rule` are real, distinct stored categories (DEC-2bv,
Phase 3's rule-kind ADR) — a caller filtering `list_memory`/Connect `ListMemories` by
`category=rule` inside a `rule:*` scope, or `category=discovery`, is a legitimate query. The
*legitimate filter domain* is strictly larger than the *legitimate write domain*. `listFilter`
already accepts any category value with no allowlist (store.go:1007-1013) — this is existing,
correct behavior the new `Search`-path filter and the new proto field must match, not diverge from.

**How to avoid:** Add `repeated string categories = 8;` to `SearchMemoriesRequest` with **no**
`buf.validate` annotation — matching `ListMemoriesRequest.categories` (engram.proto:59), which
also has none.

**Warning signs:** A `buf lint`/`buf breaking` pass that's clean but a manual test showing
`category=discovery` filtering returns zero results when discovery records actually exist in scope
— that would indicate an allowlist was accidentally added.

### Pitfall 6: The Helm chart precedent this phase is asked to mirror doesn't actually exist yet

**What goes wrong:** Assuming "update charts/engram values.yaml for the new env var" is a
mechanical copy of an existing pattern, because `ENGRAM_OPENAI_EMBEDDINGS_URL` (D-12's own stated
structural precedent) is already wired into the chart.

**What is actually true (verified):** `ENGRAM_OPENAI_EMBEDDINGS_URL` is **not** present anywhere in
`charts/engram/values.yaml` or `charts/engram/templates/_helpers.tpl` — grepped both files
directly, zero matches. Only `ENGRAM_OPENAI_BASE_URL` (via `.Values.memory.openai.baseURL`) and the
`summarize.*` block (`ENGRAM_SUMMARY_MODEL`/`MAX_CHARS`/`MAX_TOKENS`/`TIMEOUT`) are wired. So the
"D-11 operator override" pattern D-12 explicitly cites as its structural precedent was itself never
extended to the Helm chart when it shipped in Phase 13.

**How to avoid:** This is a real decision point, not a mechanical copy: the planner should either
(a) add `chatBaseURL` under `.Values.memory.summarize` (the natural home, since `chat_base_url` is
config for the summarizer, not the embedder) and wire an `ENGRAM_OPENAI_CHAT_BASE_URL` env row in
`_helpers.tpl`, accepting that this establishes chart coverage its own precedent lacks, or
(b) explicitly note in the plan that chart wiring is deliberately deferred to match the
`embeddings_url` precedent's actual (uncharted) status, and only add the docs-site row. Either is
defensible; silently assuming (a) is "just mirroring what's already there" is the trap — there is
nothing there to mirror.

**Warning signs:** A plan task description that says "mirror the `ENGRAM_OPENAI_EMBEDDINGS_URL`
chart wiring" without first confirming that wiring exists — it doesn't.

## Code Examples

### Extracting `validateCitations` (D-05) from the existing discovery validator

Verified current code being extracted (tools.go:638-656, inside `validateStoreDiscovery`):
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
Target shared shape:
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
Note: the current `validateStoreDiscovery` loop does NOT check `c.Ref == ""` explicitly (it relies
on... nothing — `Ref` is currently unvalidated for emptiness in the shipped code). D-05's text
explicitly calls for a `ref non-empty` check that does not exist in today's discovery validator —
this is a genuine (small) behavior addition being folded into the extraction, not a pure refactor.
Flag this for the plan: extracting `validateCitations` slightly tightens discovery validation too
(a citation with empty `ref` that would pass today will be rejected after this change) — verify
this is acceptable (it should be; an empty `ref` is meaningless for any of the four citation kinds)
and consider a one-line note in the plan rather than a silent side effect.

### The `citationArg -> store.Citation` mapping loop (reuse verbatim for `toMemory`)

Already-shipped in `storeDiscovery` (tools.go:907-910) — the exact loop `toMemory` needs (Pitfall 1):
```go
cites := make([]store.Citation, len(a.Citations))
for i, cit := range a.Citations {
    cites[i] = store.Citation{Kind: cit.Kind, Ref: cit.Ref, Locator: cit.Locator, Pin: cit.Pin, Excerpt: cit.Excerpt}
}
```

## State of the Art

Not applicable — this is a closed, internal-only extension of code that shipped within the last
few days (Phases 22-25, this same milestone). There is no external "current best practice" that has
moved since; the only "prior approach vs. current approach" delta is internal to this repo's own
prior phases (e.g. Phase 13's `joinEmbeddingsURL` is the "current approach" this phase ports to a
second lane — there is no further external evolution to track).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Placing the D-14 `Join` helper in `internal/config` will not create an import cycle with `internal/summarize`. | Architecture Patterns, Pattern 3 | Low — this research found no existing `internal/config -> internal/summarize` edge, but did not run `go list -deps` to exhaustively confirm; if wrong, `go build` fails immediately and loudly (self-correcting), so the practical risk is a few minutes of planner rework, not a silent defect. |
| A2 | Adding `ref` non-empty validation to the extracted `validateCitations` (currently absent from the shipped `validateStoreDiscovery`) is an acceptable, desirable tightening rather than an out-of-scope behavior change. | Code Examples, `validateCitations` extraction | Low — an empty `ref` citation is meaningless under any of the four `kind` values (file/commit/url/repo all require *some* locator string); rejecting it is very unlikely to break a legitimate existing caller, but it IS a behavior change to `store_discovery`'s validation the plan should call out explicitly rather than land silently inside an "extraction." |

**If this table is empty:** N/A — two low-risk items above, both self-correcting or low-impact.

## Open Questions

1. **Does SC2's singular "`category` filter" wording require a singular alias arg, or is plural-only sufficient?**
   - What we know: CONTEXT.md D-08 already flags this deviation and defers the call to the
     verifier's literal reading of SC2. `ListOptions.Categories`/`coreListRequest.Categories`/proto
     `categories` are all already plural with no singular alias anywhere in the existing codebase.
   - What's unclear: whether `/gsd-verify-work`'s reading of SC2 will accept a plural `categories`
     arg accepting a single element as satisfying "an optional `category` filter."
   - Recommendation: implement plural-only per D-08's primary recommendation; if verification balks,
     adding a singular `category string` alias that folds into the same `Categories []string` slice
     is a small, low-risk follow-up, not a redesign.

2. **Should the Helm chart gain `ENGRAM_OPENAI_CHAT_BASE_URL` wiring, given its own precedent
   (`ENGRAM_OPENAI_EMBEDDINGS_URL`) was never charted?**
   - What we know: verified via direct grep that neither `values.yaml` nor `_helpers.tpl` mention
     `ENGRAM_OPENAI_EMBEDDINGS_URL` today.
   - What's unclear: whether the omission for `embeddings_url` was a deliberate choice (advanced/
     rare operator override, not worth a chart value) or an oversight later phases should also not
     repeat.
   - Recommendation: treat this as the planner's call, documented in the plan rather than assumed;
     Pitfall 6 above lays out both options. Given `chat_base_url`'s stated purpose ("a local
     embedder + a hosted chat model," a not-uncommon deployment shape per the REQ text itself), charting
     it seems more likely to be worth the small addition than `embeddings_url` was, but this is a
     judgment call, not a fact this research can settle.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|--------------|-----------|---------|----------|
| Docker (for `testcontainers-go/modules/qdrant`) | Live-Qdrant store-layer tests (category filter OR-semantics, citations round-trip, SC4 authz tests) | ✓ | Docker daemon responds to `docker info` | `ENGRAM_QDRANT_TEST_ADDR` env var points tests at an already-running Qdrant instead, per `store_test.go:47-91`'s existing `TestMain` fallback logic — no new fallback needed, this repo already has one. |
| `go tool buf` (vendored via `go.mod`/`go tool`) | `task proto:gen`/`task proto:lint` for the D-10 additive proto field | ✓ | CLI resolves to 1.72.0 locally; remote plugins pinned separately in `buf.gen.yaml` (protocolbuffers/go v1.36.11, connectrpc/go v1.20.0, bufbuild/es v2.12.1) | None needed — already present and working (`go tool` manages it via `go.mod`'s tool directive, not a separate install). |
| `go1.26.3`+ toolchain (for `cmp.Or`) | D-12's `cmp.Or(...)` fallback resolution | ✓ | Local `go version` reports go1.26.5; `go.mod` declares `go 1.26.3` | None needed — `cmp.Or` has shipped since Go 1.22, well below this repo's floor. |

**Missing dependencies with no fallback:** none.

**Missing dependencies with fallback:** none — every dependency this phase touches is already
present and working on the development machine.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (no external test framework) |
| Config file | none — `go test` invoked directly via Taskfile targets |
| Quick run command | `go test ./internal/store/... ./internal/server/... ./internal/summarize/... ./internal/embed/... ./internal/config/...` (targeted packages; Docker-backed Qdrant tests auto-skip if neither Docker nor `ENGRAM_QDRANT_TEST_ADDR` is available, per `store_test.go` `TestMain`) |
| Full suite command | `task` (runs `task lint` + `task test`, i.e. `go test ./...` + the Python skill-hook suite) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-memory-citations (SC1) | Store a `memory`-category record with citations; `get_memory` returns them verbatim; a record with none has no `citations` payload key. | unit (live Qdrant via testcontainers) | `go test ./internal/store/... -run TestPayloadCitations -v` (new test) | ❌ Wave 0/task — new test, pattern mirrors `TestListCategoryAndVisibilityFilter` |
| REQ-memory-citations (D-02 regression) | `store_memory` w/ citations, then `update_memory` (content-changed and shared/summary-only paths), then re-`get_memory` — citations survive both `Update` and `UpdatePayload`. | unit (live Qdrant) | `go test ./internal/server/... -run TestUpdateMemoryPreservesCitations -v` (new test) | ❌ Wave 0/task |
| REQ-memory-citations (Pitfall 3 — Connect compact view) | Store a memory w/ citations; Connect `ListMemories`/`SearchMemories` with `full=false` omits citations from the response. | unit (live Qdrant + Connect handler) | `go test ./internal/server/... -run TestConnectCompactViewOmitsCitations -v` (new test) | ❌ Wave 0/task |
| REQ-category-filter (SC2, MCP) | `search_memory`/`list_memory` with a `categories` filter return only matching records; OR semantics across multiple values. | unit (live Qdrant) | `go test ./internal/store/... -run TestListCategoryAndVisibilityFilter -v` (existing, extend for Search path) + new `TestSearchCategoryFilter` | ✅ (list) / ❌ (search — new) |
| REQ-category-filter (SC2, MCP↔Connect parity) | Same `categories` filter on MCP `search_memory`/`list_memory` and Connect `SearchMemories`/`ListMemories` returns the identical result set. | integration | `go test ./internal/server/... -run TestMCPConnectCategoryFilterParity -v` (new, or folds into existing parity suite — planner's discretion per CONTEXT.md) | ❌ Wave 0/task |
| REQ-category-filter (SC2, pre-ranking) | A category-filtered-out record cannot appear in search results even at rank 1 (filter applied before vector ranking, not after). | unit (live Qdrant) | `go test ./internal/store/... -run TestSearchCategoryFilterPreRanking -v` (new) | ❌ Wave 0/task |
| REQ-chat-base-url (SC3) | `ENGRAM_OPENAI_CHAT_BASE_URL` set → summarizer hits that host; embedder still hits `ENGRAM_OPENAI_BASE_URL`; unset → both hit the shared URL. | unit (no network — assert on constructed client/request, not a live external call) | `go test ./internal/server/... -run TestSummarizerFromConfigChatBaseURL -v` (new) | ❌ Wave 0/task |
| REQ-chat-base-url (D-13 table test) | The 3-way `/v1`-shape join heuristic produces the correct chat-completions endpoint for LiteLLM-bare, `.../v1`, and `.../v1beta/openai` shapes. | unit (pure function, table-driven) | `go test ./internal/summarize/... -run TestJoin -v` (new, mirrors existing `TestJoinEmbeddingsURL` in `internal/embed`) | ❌ Wave 0/task |
| Cross-cutting (SC4) | A `categories` filter cannot widen visibility across owners; a citation-carrying `shared` record readable by a second actor is still not writable by them. | unit (live Qdrant) | `go test ./internal/store/... -run TestCategoryFilterDoesNotWidenVisibility -v` + `TestCitationsDoNotGrantWriteAccess -v` (new) | ❌ Wave 0/task |

### Sampling Rate
- **Per task commit:** targeted package `go test ./internal/<changed-package>/... -v` for the
  package(s) touched by that task.
- **Per wave merge:** `go test ./internal/store/... ./internal/server/... ./internal/summarize/... ./internal/embed/... ./internal/config/...` (every package this phase touches).
- **Phase gate:** `task` (full lint + test suite, including `task proto:lint`'s idempotency-ban
  grep and the CI `buf` job's lint/breaking/drift checks locally via `go tool buf lint` +
  `go tool buf breaking --against 'https://github.com/seanb4t/engram.git#branch=main'` +
  `go tool buf generate && git diff --exit-code -- gen/`) green before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] No existing test file directly exercises citations on a `memory`-category record (all
      existing citation tests are discovery-scoped) — every citations test above is new.
- [ ] No existing test exercises `Store.Search`'s category filter (only `Store.List`'s is tested
      today, at `TestListCategoryAndVisibilityFilter`) — the D-09 `SearchOptions` refactor needs its
      own coverage, not just a signature change.
- [ ] No existing test exercises the Connect `SearchMemories`/`ListMemories` compact-view field
      clearing beyond `Content`/`Summary` — the new `Citations`-clearing behavior (Pitfall 3) needs
      a dedicated test, since the existing `shapeProtoMemories` tests (if any) likely only assert
      on `Content`/`Summary`, not `Citations`.
- [ ] No test file exists yet for `internal/summarize`'s URL-join behavior (unlike `internal/embed`,
      which already has `TestJoinEmbeddingsURL`) — this is a genuinely new test file, not an
      extension of an existing one.
- Framework install: none — `go test` and the existing Qdrant-testcontainer harness fully cover
  this phase's test needs; no new framework/tool install required.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Unchanged this phase — no new auth mechanism, no touch to `internal/auth`. |
| V3 Session Management | no | Unchanged — no session/token handling in any of the three tracks. |
| V4 Access Control | yes | The existing Cedar-backed `ownerOrSharedCondition` outer-`Must` invariant (DEC-cgb/DEC-cdr1) — both the category filter and citations compose onto it, never widen or bypass it (D-16). No new Cedar action, no new PDP call site is introduced. |
| V5 Input Validation | yes | `validateCitations` (D-05) enforces `kind` enum membership, non-empty `ref`, and size/count caps (`maxCitationExcerptBytes`=16 KiB, `maxDiscoveryCitations`=50) — a resource-exhaustion guard already proven for discovery citations, extended verbatim to memory citations. `ENGRAM_OPENAI_CHAT_BASE_URL` (D-15) gets the same URL-shape validation (scheme + host required) as its `EMBEDDINGS_URL` sibling. |
| V6 Cryptography | no | Unchanged — no new secret, key, or crypto primitive. |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Cross-path lost-write (a whole-payload `Upsert` silently erasing a key another path set via targeted `SetPayload`) | Tampering (data integrity) | D-02's mandate to route citations exclusively through `payload()`/`fromPayload`, never a bespoke `SetPayload`; this is the exact class of bug engram memory `86q25vq6jf`/`m43h2yt97m` already documents from Phase 24/25 — the mitigation is architectural (single write codec), not a runtime check. |
| Authz bypass via an over-broad filter (a category or citation feature accidentally becoming a second, weaker access-control path) | Elevation of Privilege | D-16's explicit regression test: the category filter and citations compose strictly *inside* the existing outer `Must` (`ownerOrSharedCondition`) — verified directly in `listFilter`'s code shape (the category `Should`-condition is appended to `must`, never replacing or reordering the authz condition ahead of it). |
| Resource exhaustion via oversized/unbounded citation payloads | Denial of Service | `maxCitationExcerptBytes` (16 KiB) and `maxDiscoveryCitations` (50) caps, already shipped for discovery and reused verbatim for memory citations via the shared `validateCitations` (D-05). |
| Information disclosure via an unintentionally verbose "compact" recall view (Pitfall 3) | Information Disclosure | Explicit `pb.Citations = nil` (and `pb.Kind = ""`) clearing in `shapeProtoMemories`'s `!full` branch, matching the token-budget intent D-07 already states for the MCP lane. |

## Sources

### Primary (HIGH confidence — direct codebase inspection, this session)
- `internal/store/store.go` (payload/fromPayload codec, listFilter, activeWindowConditions, tagMatchConditions, Search/SearchReranked, Update/UpdatePayload/Supersede/IncrementAccess/BackfillShortIDs/RemapOwner/Reindex) — read in full across multiple passes this session.
- `internal/server/tools.go` (storeArgs/searchArgs/listArgs/citationArg/storeDiscoveryArgs, validateStoreDiscovery, toMemory, checkIdempotentReplay, storeMemory/scheduleMemory/storeDiscovery/listMemory/searchMemory handlers, coreListRequest/coreSearchRequest, MCP tool registrations, shapeRecall usage) — read in full across multiple passes this session.
- `internal/server/connectapi.go` (memoryToProto, citationsToProto, shapeProtoMemories, ListMemories/SearchMemories handlers) — read in full this session.
- `internal/server/summary.go` (shapeRecall, toRecallView, recallView struct) — read in full this session.
- `internal/summarize/summarize.go` (Client, New, Summarize, the naive endpoint concat) — read in full this session.
- `internal/embed/embed.go` (New, joinEmbeddingsURL, ReservedParamKeys cross-package comment) — read this session.
- `internal/config/{config.go,registry.go,validate.go}` (OpenAIConfig, the openai.* registry rows, the EmbeddingsURL validate-only-when-set block) — read this session.
- `proto/engram/v1/engram.proto` (Memory message, ListMemoriesRequest/SearchMemoriesRequest, StoreMemoryRequest's category buf.validate constraint, StoreDiscoveryRequest's citations constraint) — read this session.
- `buf.gen.yaml` (pinned remote plugin versions) and `.github/workflows/ci.yaml` (the `buf` job's lint/breaking/drift steps) — read this session.
- `charts/engram/values.yaml` and `charts/engram/templates/_helpers.tpl` — read/grepped this session (confirmed `ENGRAM_OPENAI_EMBEDDINGS_URL` absence).
- `docs-site/src/content/docs/reference/memory-record.md` — read in full this session (identified the exact doc section needing an update for D-01/D-03).
- `go.mod` (`qdrant/go-client v1.18.3`, `connectrpc.com/connect v1.20.0`, `go 1.26.3`) — read this session.
- `internal/store/store_test.go` (TestListCategoryAndVisibilityFilter, TestMain's Docker/testcontainers fallback), `internal/store/service_principal_isolation_test.go`, `internal/store/rerank_test.go`, `internal/store/instrument_test.go`, `internal/retrievaleval/retrieval_eval_test.go`, `internal/retrievaleval/fixtures.go` — grepped/read for the D-09 call-site enumeration.
- `Taskfile.yaml` (`test`/`test:go`/`proto:lint`/`proto:gen` targets) — read this session.
- Local environment probes this session: `docker info` (available), `go version` (go1.26.5), `go tool buf --version` (1.72.0).

### Secondary (MEDIUM confidence)
- `.planning/phases/26-.../26-CONTEXT.md`, `.planning/REQUIREMENTS.md`, `.planning/STATE.md`, `.planning/ROADMAP.md` — the upstream planning artifacts this research validates against (all read in full this session; treated as locked scope, not re-litigated).

### Tertiary (LOW confidence)
None — this phase required no external web search or training-data recall; every claim above traces to a direct read of the current repository state.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; every version claim read directly from `go.mod`/`buf.gen.yaml`.
- Architecture: HIGH — every pattern shown is copy-verified from already-shipped, already-tested code in this exact repository.
- Pitfalls: HIGH — all six pitfalls were discovered by reading the actual function bodies of every write path named in the phase's own `<additional_context>` checklist, not inferred.

**Research date:** 2026-07-25
**Valid until:** 30 days (stable, internal-only Go codebase; no fast-moving external dependency in scope this phase) — but effectively valid until the next commit touches any of the ~10 files enumerated in Sources, since every claim is line-anchored to the current `main` tree.
