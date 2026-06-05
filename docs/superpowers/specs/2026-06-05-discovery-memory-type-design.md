<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Discovery memory type — citation-backed, aging-aware

**Date:** 2026-06-05
**Status:** Design
**Design bead:** engram-4v1

## Context

engram stores durable memory for coding agents as vectors in a single Qdrant
collection. Today a `Memory` carries `content`, `scope`, repo/workspace/
worktree/base_dir, `source` (`user-said` | `agent-inferred`), `category`
(`decision` | `preference` | `convention` | `gotcha`), `tags`, `actor`
(server-set), and `created_at`. These four categories capture **timeless
intent** — "use cobra not viper", "Sean prefers X". They do not rot, because
they describe what we *want*, not what the code *currently is*; they are
deliberately sparse and zero-junk, and are loaded at session start.

There is a second, structurally different kind of knowledge an agent produces:
**understanding it earned by reading code** — usually unfamiliar territory (a
third-party dependency, or a large part of the repo it had to trace). "The
retry logic lives in `client.go:200-240`, keyed off the context deadline."
"Auth flows token → JWKS verify → actor injection across these three files."
This understanding is expensive to derive and is currently thrown away at the
end of a session, forcing the next agent to re-trace the same ground.

This design adds a **`discovery`** memory type to cache that understanding.

### North star: work-saving, not just recall

The goal is **token / context saving** as much as memory. A discovery is a
*cache of earned understanding*, and its value equals the re-derivation it
prevents. Two consequences are load-bearing for every decision below:

1. **Recall must cost less than re-derivation.** If loading discoveries bloats
   context more than re-reading the code would, the feature is net-negative.
   Therefore discoveries are **not** loaded at session start; they are fetched
   on demand and bounded by semantic top-k.
2. **Capture is only worth it when re-deriving is expensive.** This is the
   zero-junk rule restated in token terms, and it is why a minimal `file:line`
   pointer is often the *wrong* economy: cheap to store, but expensive to use
   if acting on it forces re-reading a 5,000-line source. A discovery is
   allowed to **cache the substance** (the 25 relevant lines + a summary), not
   just a pointer.

### Server is repo-blind

engram is **server (MCP + Qdrant) + plugin (skills / hooks)**. There is no
engram client binary. Whenever this design says "the client checks the repo",
the actual actor is the **coding agent** (Claude Code, etc.), which already has
filesystem and git access through its own tools, driven by an engram skill,
calling MCP tools. The server never touches a repo. This is why staleness is
not server-computed (see "Aging").

### Decisions (from brainstorming, recorded on engram-4v1)

1. **Additive, same collection.** `discovery` is a 5th `category` on the
   existing `Memory` record; new fields are optional payload keys the other
   four categories leave empty. No new collection, no migration — Qdrant
   payloads are schemaless and `fromPayload` skips absent keys.
2. **`kind: map | fact`.** A discrete sub-type. `map` discoveries orient
   (where things live, how flows connect); `fact` discoveries are pinned,
   individually checkable behavioral claims. Recall can ask for maps when
   orienting and facts when verifying.
3. **Graceful decay, not binary stale.** The server stores citation pins +
   capture time and computes **no** freshness verdict. Recall *surfaces* the
   aging signals (age, pinned commit, citation locator) and the agent — which
   has the repo — judges how much to trust the record. Veracity erodes
   naturally rather than flipping a flag.
4. **Separate pool, on-demand recall.** Discoveries live in a `discovery:`
   scope namespace keyed to the repo, are not in the session-start recall, and
   are retrieved by targeted top-k semantic search when entering an area. The
   curated four stay the small, always-loaded set.
5. **Explicit cross-spine search, default off.** Default recall stays scoped
   and cheap. The agent may explicitly request a search that spans scopes / the
   broader discovery pool when warranted. This is also the seam through which
   future cross-project reuse arrives, without committing to it now.
6. **Permissive but explicit capture.** Discoveries (especially maps) are
   allowed to proliferate more than the curated four, but capture is still
   agent-decided and gated by the token-saving threshold. engram's
   "no auto-extraction" tenet holds — nothing scrapes discoveries automatically.
7. **Dedicated tools and a dedicated skill.** `store_memory` / `search_memory`
   stay untouched for the curated four. Discoveries get `store_discovery` /
   `search_discovery` and a new **`discovering`** plugin skill that drives the
   capture workflow.

## Goals / Non-goals

**Goals**

- Persist agent-earned codebase understanding as citation-backed `discovery`
  records that save re-derivation cost.
- Make recall cheap and on-demand: top-k semantic over a separate discovery
  scope, surfacing aging signals so the agent can judge trust.
- Ship the capture workflow as a `discovering` skill and the consume path as
  `search_discovery` (+ explicit cross-spine broadening).

**Non-goals (explicitly deferred)**

- **Cross-project / dependency-keyed reuse** (`discovery:dep:repo@rev` shared
  across projects). The scope key is designed to allow it later; not built now.
- **A `verify` tool / persisted freshness verdict.** Aging is surfaced raw in
  v1; an agent-driven verify-and-refresh round-trip is a later nicety.
- **The memory UI.** A separate spec → plan → build cycle, designed after this
  lands so it can render discovery records (citations, aging) from the start.
- **Auto-extraction of discoveries.** Out of scope by tenet.

## Data model

A discovery reuses the `Memory` record with additive fields. `Content` holds
the *understanding* — the prose claim or map — and is what gets embedded and
searched. The other four categories never set the new fields.

```go
type Memory struct {
    // ...existing fields (ID, Content, Scope, Repo, Workspace, Worktree,
    //    BaseDir, Source, Category, Tags, Actor, CreatedAt) ...

    // Discovery-only (empty/zero for the curated four):
    Kind      string     `json:"kind,omitempty"`      // "map" | "fact"
    Citations []Citation `json:"citations,omitempty"` // >=1 for discoveries
    Summary   string     `json:"summary,omitempty"`   // optional one-liner
}

type Citation struct {
    Kind    string `json:"kind"`              // file | commit | url | repo
    Ref     string `json:"ref"`               // path / repo URL / doc URL
    Locator string `json:"locator,omitempty"` // e.g. "200-240" line range
    Pin     string `json:"pin,omitempty"`     // aging anchor captured at store time
    Excerpt string `json:"excerpt,omitempty"` // cached substance (the 25 lines)
}
```

- `Content` (understanding) is the embedded/searched text. `CreatedAt` doubles
  as **captured-at** — no new timestamp field needed.
- **`Source`** is set server-side to `agent-inferred` for every discovery (they
  are, by definition, agent-derived); `store_discovery` does not take a `source`
  argument. The field stays populated so the existing `Memory` invariant holds.
- **Update path.** `update_memory` works on discovery records *without dropping
  citations*: `updateMemory` does `cur := get(id); cur.Content = new; Upsert(cur)`,
  and once `fromPayload` populates `Kind` / `Citations` / `Summary` (see
  Storage), `cur` round-trips the full record — only `Content` (and its
  embedding) changes. To revise citations/excerpts, call `store_discovery` with
  the same `id` (full replace). This is stated so implementers do not assume the
  scalar-only `update_memory` silently truncates the nested payload.
- **Citation kinds and their pin:**

  | `kind`   | `ref`                     | `locator`     | `pin` captured at store time |
  |----------|---------------------------|---------------|------------------------------|
  | `file`   | repo-relative path        | line range    | **`fact`:** content-hash of the cited region (precise: did *these lines* change). **`map`:** commit SHA (coarse: did anything here move). |
  | `commit` | repo identifier           | —             | commit SHA |
  | `url`    | doc URL                   | anchor/section| fetched-at timestamp |
  | `repo`   | repo URL                  | —             | `@rev` / version (e.g. `@v1.2`) |

- **Excerpt** is the cached substance for that citation — the few lines worth
  keeping so the discovery is usable without re-fetching the source. Optional;
  maps may omit it, high-value facts usually carry it.

### Storage (Qdrant)

Grounded against `/qdrant/go-client` (context7): `qdrant.NewValueMap` converts
nested `map[string]any` → struct values and `[]any` → list values.

**Write path** (`payload(m)` additions, guarded by
`if m.Category == "discovery" { … }` so curated records are byte-for-byte
unchanged): add `kind`, `summary`, and `citations` as a `[]any` whose elements
are `map[string]any{"kind","ref","locator","pin","excerpt"}`. `NewValueMap`
handles the nesting.

**Read path** (`fromPayload` additions — this is new code, not free; the
contract):

```go
if v, ok := p["kind"]; ok { m.Kind = v.GetStringValue() }
if v, ok := p["summary"]; ok { m.Summary = v.GetStringValue() }
if v, ok := p["citations"]; ok {
    for _, item := range v.GetListValue().GetValues() {
        s := item.GetStructValue().GetFields() // map[string]*qdrant.Value
        m.Citations = append(m.Citations, Citation{
            Kind:    s["kind"].GetStringValue(),
            Ref:     s["ref"].GetStringValue(),
            Locator: s["locator"].GetStringValue(),
            Pin:     s["pin"].GetStringValue(),
            Excerpt: s["excerpt"].GetStringValue(),
        })
    }
}
```

The accessor chain (`GetListValue().GetValues()` → `GetStructValue().GetFields()`
→ per-field `GetStringValue()`) is novel relative to the existing flat
`fromPayload`; the store round-trip test (see Testing) is the gate that proves
it. Absent keys read back as zero values, preserving backward compatibility. The
change to `internal/store/store.go` is additive (no field is removed or
retyped).

### New `Store` query method

`cross_spine` and the `kind` filter need a compound filter the current
exact-match `Search` cannot express. Add one method rather than a prefix/wildcard
primitive (none is needed):

```go
// SearchDiscovery runs a top-k vector search constrained to discovery records.
// Empty scope = span all discovery scopes (the cross_spine case);
// empty kind = both map and fact.
func (s *Store) SearchDiscovery(ctx context.Context, scope, kind string,
    vec []float32, k uint64) ([]Memory, error)
```

It builds a Qdrant `Filter{Must: …}` from the existing `NewMatch` primitive:
always `NewMatch("category", "discovery")`; add `NewMatch("scope", scope)` when
`scope != ""`; add `NewMatch("kind", kind)` when `kind != ""`. No prefix
matching, no new namespace field — "span every discovery scope" is simply the
`category`-only filter with the scope condition dropped. The curated
`Search`/`scopeFilter` path is untouched.

### Scope convention

Discoveries use a **distinct scope namespace** so they never appear in the
curated session-start recall:

```text
discovery:repo:github.com/seanb4t/engram
```

The `discovery:` prefix keys the pool to the repo today. The namespace shape
(`discovery:<tier>:<key>`) is intentionally parallel to the curated
`<tier>:<key>` scopes, leaving room for a future `discovery:dep:<repo@rev>`
tier (deferred non-goal) without reworking the model.

## MCP tools

Curated tools are unchanged. Two discovery tools are added in
`internal/server/tools.go`:

- **`store_discovery(content, kind, citations[], scope, tags?, summary?, id?)`**
  — server sets `category="discovery"`, `source="agent-inferred"`, `actor` (from
  token), `created_at`, then embeds `content` and upserts. Validates
  `kind ∈ {map, fact}` and `len(citations) >= 1` (no `source` argument — see
  Data model). `id` is **optional**: absent → server mints a new UUID (create);
  present → upsert-replaces that record in place (the full-replace path for
  revising citations/excerpts, since `update_memory` only touches `Content`).
- **`search_discovery(query, scope?, kind?, k?, cross_spine?)`** — top-k
  semantic search over the discovery pool, implemented via
  `Store.SearchDiscovery(scope, kind, …)`. `kind` (`map` | `fact`, optional)
  adds a `kind` filter condition. `cross_spine` (default **false**) is the
  explicit broadening from Decision 5: when false the search is constrained to
  the given `scope` (compound `category=discovery AND scope=…`); when true the
  scope condition is dropped (`category=discovery` only), spanning every
  discovery scope. Results carry the full record including `citations` (with
  `pin`) and `created_at` — the aging signals the agent renders.

  **Parameter validity:**

  | `cross_spine` | `scope` | behavior |
  |---------------|---------|----------|
  | `false` (default) | present | search within that scope (typical) |
  | `false` | absent | **error** — scope required, refuses an accidental pool-wide scan |
  | `true` | absent | search all discovery scopes |
  | `true` | present | `scope` ignored (cross-spine supersedes it); server warns |

`get_memory` / `update_memory` / `delete_memory` operate by id; on discovery
records `update_memory` revises `Content` only and preserves citations (see Data
model → Update path), while `store_discovery` with the same `id` is the
full-replace path.

### Aging on read

`search_discovery` returns each citation's `pin` and the record's `created_at`.
That is sufficient for the agent to render trust context — e.g. "map of
`pkg/auth`, true as of commit `abc123`, captured 3 months ago; `auth.go` has
since changed" — by recomputing the current state locally with its own tools.
The server stores and surfaces; it never verdicts. (A future `verify` tool
could let the agent write back a `last_verified` per citation; out of scope.)

## The `discovering` skill

A new plugin skill at `skill/engram/skills/discovering/SKILL.md` — the
capture-side analog of `curating-memory`. It drives the agent to systematically
explore a repo/project and record discoveries.

- **Trigger:** "map this repo", "help me understand this codebase", onboarding
  to unfamiliar code, or before substantial work in an unmapped area.
- **Workflow:** explore breadth-first → for each meaningful unit, decide
  `map` (structure/flow) vs `fact` (pinned behavioral claim) → capture citation
  pins (commit SHA / content-hash) and excerpts → `store_discovery` into the
  `discovery:repo:<repo>` scope.
- **Discipline:** **search-before-store** (semantic dedup, mirroring
  `curating-memory`) so re-running the skill updates rather than duplicates;
  capture gated by the **token-saving threshold** (would re-deriving this cost
  meaningful tokens?); more permissive than curated but never auto-extraction;
  always cite; carry excerpts for high-value facts. The skill enforces a
  **soft excerpt cap of ~50 lines** per citation (Resolved decision 2): enough
  to keep the substantive 25-of-5,000 win, bounded so excerpts don't balloon the
  payload; the agent may exceed it only with explicit justification.
- **Authoring note:** `discovering/SKILL.md` must open with YAML frontmatter on
  line 1 and carry **no leading SPDX comment** — `.licenserc.yaml` exempts
  `skill/**/SKILL.md` from header coverage because a leading comment breaks the
  skill loader and rumdl frontmatter detection (see existing `curating-memory` /
  `promoting-memory` skills and the `.licenserc.yaml` exemption block).
- **Consume side:** documented as a convention — the agent issues a targeted
  `search_discovery` at the start of a task in mapped territory (and may pass
  `cross_spine=true` when explicitly broadening). No session-start hook change;
  discoveries stay out of the default recall by design.

## Error handling

- `store_discovery` rejects `kind ∉ {map, fact}` and empty `citations` with a
  tool error (no silent default) — a discovery without a citation defeats its
  defining property.
- Embedding failure surfaces the error to the caller, same as `store_memory`
  today.
- `fromPayload` tolerates absent/old keys (records written before this change
  read back with empty discovery fields) — backward-compatible by construction.
- `search_discovery` validates per the parameter-validity table above:
  `cross_spine=false` with an absent scope is an explicit error (scope required)
  rather than an accidental pool-wide scan; `cross_spine=true` with a `scope`
  logs a warning and ignores the scope.

## Testing

- **store round-trip** (`internal/store`): a `discovery` record with two
  citations (a `file` with line range + content-hash pin + excerpt, and a
  `repo@rev`) upserts and reads back identically, including nested citation
  fields. Mirrors `TestUpsertGetDeleteRoundtrip`.
- **search isolation**: discoveries in `discovery:repo:X` do not surface in a
  curated `repo:X` search and vice versa; `kind` filter returns only maps or
  only facts.
- **tool validation** (`internal/server`): `store_discovery` rejects bad `kind`
  and empty citations; `search_discovery` requires a scope unless
  `cross_spine`.
- **skill**: `discovering/SKILL.md` carries valid frontmatter (no leading SPDX
  comment — per the SKILL.md license-exemption gotcha) and is exercised by the
  existing plugin-config test pattern if applicable.

## Backward compatibility & migration

None required. The Qdrant payload is schemaless and all new fields are
optional; existing records and the curated tools are untouched. The
`discovery:` scope is new namespace, so nothing pre-existing collides.

## Resolved decisions (previously open)

1. **Pin rule** — per-kind split: `fact` file-citations pin a **content-hash**
   of the cited region (precise change detection); `map` file-citations pin a
   **commit SHA** (coarse "something here moved"). Rationale: facts make exact
   claims worth checking exactly; maps are orientation where commit-level
   granularity is enough. (Reversible to a single uniform rule if implementation
   shows the split isn't worth it — flagged for the implementer, not a blocker.)
2. **Excerpt size** — soft cap of **~50 lines per citation**, enforced as
   guidance in the `discovering` skill (not a server-side hard limit), with the
   agent free to exceed on explicit justification. Keeps the substantive-excerpt
   win while bounding payload growth.
