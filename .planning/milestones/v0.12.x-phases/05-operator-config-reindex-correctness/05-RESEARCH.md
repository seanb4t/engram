<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright 2026 Sean Brandt -->

# Phase 5: Operator Config & Reindex Correctness - Research

**Researched:** 2026-08-01
**Domain:** Go config wiring (koanf) + Qdrant-backed reindex correctness + Helm chart env plumbing + docs-site
**Confidence:** HIGH — every claim below was verified by reading the live tree this session (no web research was needed; this phase has no third-party API or library surface, only in-repo patterns already established by the sibling `chat_base_url` feature).

## Summary

This phase is two independent, already-diagnosed defects plus a documentation-only repair path. Both defects have a **near-identical shipped precedent already in the tree** — the `ENGRAM_OPENAI_CHAT_BASE_URL` split (D-12 from the base-URL work) for #350, and the existing `opts.Identity` resume-guard conjunct (Phase 13 SC3) for #345 — so this is exact-pattern-mirroring work, not novel design. All 16 CONTEXT.md decisions were checked against the live code this session; **15 hold exactly as written and 1 (D-04) rests on a location premise that does not match the chart's actual convention** (see Finding 1 below — this is the one thing the planner must resolve before writing tasks).

For #345, the fix is smaller than it looks: `reindexTargetContents`'s `Get` call already requests the full payload (`qdrant.NewWithPayload(true)`, which means "all fields," not a field-selector allow-list), so **tags are already being fetched from the target — they are simply not being read out of the response into `reindexTarget`**. The orchestrator's suspected "silent no-op" risk site (a field-subset selector silently omitting `tags`) does not exist; this finding meaningfully de-risks D-07. The correct decode path is `fromPayload`'s existing `"tags"` → `GetListValue()` → `GetValues()` loop (`store.go:562-568`) — reusing that function for the target read (rather than hand-rolling a second decode) is the literal fulfillment of D-08's "both sides decode tags through the same path."

For D-13, the "no new command" premise checks out structurally: `Reindex`'s per-point loop embeds and upserts vector + payload in **one** `Upsert` call built from **one** source read (`store.go:2741-2751`), so a target point's vector and payload can never disagree — a patched `--resume` re-run against a still-live source genuinely heals every record the buggy predicate skipped, because the source is re-scrolled fresh (current content + current tags) on every invocation, not replayed from a snapshot.

**Primary recommendation:** Implement #350 exactly as D-01–D-06 specify (mirror `chat_base_url`'s registry/struct/`cmp.Or`/no-validation shape field-for-field), but resolve the Helm value's location before writing Task 1 — put `chatApiKeySecret` under `memory.summarize`, not `memory.openai`, to match where `chatBaseURL` already lives. Implement #345 by extending `reindexTarget` with a `tags []string` field, decoding it via `fromPayload` (not a hand-rolled second decoder) in `reindexTargetContents`, and adding an order-independent tag-set-equality check (sorted-copy comparison, treating nil and empty as equal) as a third conjunct in the skip predicate at `store.go:2716-2727`. Both fixes stay within existing packages and add zero new Go dependencies — confirmed no new imports are required (both features build on stdlib `cmp`/`sort`/`net/url`, already imported).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Chat/summarize API key resolution | API / Backend (`internal/server/tools.go` construction site) | Config / Storage (`internal/config` registry + struct) | Credential resolution happens once, at the HTTP-client-construction boundary — never in the config loader itself (established `chat_base_url` precedent, D-02) |
| Chat/summarize API key operator surface | Config / Storage (env var + koanf registry) | Deploy (Helm `values.yaml` → env) | The env var is the source of truth; Helm is a thin renderer over it, not an independent surface |
| Reindex resume tag-awareness | Database / Storage (`internal/store.Store.Reindex`, Qdrant read/compare) | — | Purely a storage-layer correctness fix; no API surface changes shape |
| Reindex stale-record repair | Database / Storage (documented re-run of existing `Reindex`) | CLI (`cmd/engram/reindex.go` — `--dry-run --resume` sizing) | No new command; the existing CLI verb becomes the repair tool once the predicate is patched |
| Repair discoverability | Docs (docs-site `guides/reindex.md`, `guides/upgrade.md`) | — | Purely operator-facing; no code owns "discoverability" |

## User Constraints (from CONTEXT.md)

<user_constraints>
### Locked Decisions

- **D-01 (the config surface is an exact mirror of ChatBaseURL):** add
  `openai.chat_api_key` / `ENGRAM_OPENAI_CHAT_API_KEY` to `internal/config/registry.go` with **no**
  `Default` and **no** `Legacy` alias, and a `ChatAPIKey string` field on `OpenAIConfig`. Registry
  and struct shape copied from the `chat_base_url` entry directly above it.

- **D-02 (the inherit fallback is resolved at the construction site, never at config load):**
  `summarizerFromConfig` (`internal/server/tools.go:419-428`) gains
  `cmp.Or(cfg.OpenAI.ChatAPIKey, cfg.OpenAI.APIKey)` alongside the existing `chatBaseURL` line.
  This is D-12 from the base-URL work restated — the config field always faithfully reflects what
  the operator set, and exactly one site knows about the fallback.

- **D-03 (behavior with the key unset is byte-identical to today):** `cmp.Or` on an empty
  `ChatAPIKey` yields `cfg.OpenAI.APIKey`, which is the literal argument passed today. This is
  the requirement's "byte-identical when unset" clause and is provable by argument equality, not
  by asserting equal outbound behavior.

- **D-04 (the Helm value ships in this phase, not as a follow-up):** add
  `memory.openai.chatApiKeySecret` with its own `secretKeyRef` in `charts/engram/values.yaml` +
  `_helpers.tpl`, omitted meaning the env var is absent meaning inherit. Same reasoning that
  pulled `connect.headless` into v0.12.x Phase 1 — `charts/engram` has no generic extra-env
  escape hatch, so an env-only feature is unreachable for chart users.
  **⚠ Research finding: the literal path in this decision does not match the chart's live
  convention — see Finding 1 in Common Pitfalls / Runtime State Inventory below.**

- **D-05 (no startup validation of the key):** unlike a base URL, an API key has no verifiable
  shape, and empty is meaningful (inherit). A wrong key must fail at the provider, not at boot.
  No `loadAndValidate` change.

- **D-06 (configure.md carries a statement this phase makes false, and fixing it is in scope):**
  `docs-site/src/content/docs/guides/configure.md` currently asserts there is no separate key for
  the chat base URL. That sentence must be corrected, not merely supplemented, alongside the new
  table row and a `guides/upgrade.md` v0.12.0 entry.

- **D-07 (the target lookup is extended, not just the equality check):** `reindexTarget`
  (`store.go:2770`) gains a `tags []string`, and `reindexTargetContents` (`store.go:2782`) reads
  the target payload's `tags`. Comparing the source's tags against an always-nil field would
  either preserve the bug or defeat resume entirely — the requirement names this explicitly.

- **D-08 (both sides decode tags through the same path):** the target-side read must produce the
  same Go shape the source side already produces for `m.Tags`. An encoding asymmetry between the
  two reads surfaces as permanent false re-embeds — resume never converges — which is strictly
  worse than the bug being fixed.

- **D-09 (tag comparison is order-independent, and the residual is documented at the predicate):**
  the requirement mandates order-independence. `EmbedText` (`store.go:277-282`) joins tags in slice
  order, so a tags-permuted-only record embeds to different text yet is deliberately skipped. That
  residual is stated in a comment at the predicate rather than chased. Normalizing tag order at
  write time is the root fix and is explicitly out of scope — it touches every write path and
  every stored record.

- **D-10 (nil and empty tag slices compare equal):** a record with no tags stores no `tags` payload
  key, so the target read yields `nil` while the source may decode to an empty slice. Strict
  equality there would re-embed every untagged record on every resume, forever.

- **D-11 (the stale comment at the skip predicate is corrected, not left in place):** the comment
  at `store.go:2718-2720` asserts equal content implies equal tags "from the same source payload".
  That premise is false — it compares source content against target content, and the target's tags
  are a snapshot from whenever it was last written. It must be rewritten to describe what the code
  actually does.

- **D-12 (the paired positive control is mandatory and RED comes from mutating the predicate):**
  one test, three cases, against real Qdrant via testcontainers — content-same and
  tags-differ re-embeds; content-and-tags-same skips; same-elements-different-order skips. Without
  the skip case a fix that silently stops skipping anything goes green while re-embedding the whole
  collection on every resume. RED is observed by mutating the predicate, never by toggling a flag.

- **D-13 (no new command — the patched resume is itself the repair path):** vector and payload are
  written by the same upsert from the same source read, so they never disagree. A record the buggy
  predicate skipped therefore holds both the stale vector and the stale tag snapshot, and the
  patched predicate detects it on a plain re-run of `engram reindex --resume`. A dedicated
  one-time command would be a wrapper around `Reindex` taking the same arguments. This is a
  deliberate, reasoned departure from REQ-reindex-stale-repair's literal wording, which points at
  the `backfill-short-ids` / `migrate-remap-owner` precedent — those commands exist because no
  shipped path healed those records; here one does.

- **D-14 (--dry-run must honor --resume so the repair can be sized before it is run):** today
  `DryRun` short-circuits before the resume lookup (`store.go:2691-2700`), so `--dry-run --resume`
  reports every scanned record as "would be re-embedded" and tells the operator nothing about the
  blast radius. Wiring the resume lookup into the dry-run arm is what makes "just re-run it" a real
  operator tool. The dry-run guarantee that nothing is written and no target is created is
  unchanged.

- **D-15 (the source-collection-gone case is a stated limit, never a best-effort guess):** the
  correct tags live only in the source collection. If it was deleted after cutover they are
  unrecoverable. Re-embedding from the live payload would produce a vector consistent with the
  *stale* tags — silently wrong while looking healed. Document the limit; implement nothing.

- **D-16 (the repair is documented in the existing reindex guide, not a new page):**
  `docs-site/src/content/docs/guides/reindex.md` gains a "Repairing a pre-patch resume" section;
  `guides/upgrade.md`'s v0.12.0 entry names the defect and the re-run.

### Claude's Discretion

- Exact naming of any new comparison helper, and whether it lives in `internal/store` beside
  `EmbedText` or unexported next to the predicate.
- Plan/wave decomposition — #350 and #345 touch disjoint files and could be one plan or two.
- Whether the `--dry-run --resume` counts are surfaced through the existing `ReindexResult` fields
  or a new one, provided `reindexSummary`'s dry-run wording stays honest about what was counted.

### Deferred Ideas (OUT OF SCOPE)

- **Normalizing tag order at write time.** The root fix for D-09's residual. Touches every write
  path and every stored record; belongs in its own phase with a migration story.
- **A dedicated `repair-*` reconciliation command.** Reconsider only if planning research
  falsifies D-13's premise that a patched `--resume` re-run heals the full affected set.
  **Research did NOT falsify this premise — see the Package Legitimacy Audit section note and
  the "D-13 verified" discussion under Common Pitfalls.**
- **A separate credential for any third provider lane.** Only the chat/summarize split is in
  scope; the embedder keeps `ENGRAM_OPENAI_API_KEY`.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-per-lane-api-key | Operator can point chat/summarize client at a different provider credential than the embedder; empty means inherit; byte-identical when unset. Closes #350. | Confirmed single construction site (`summarizerFromConfig`, `tools.go:419-428`); confirmed `chat_base_url` registry/struct/validate.go pattern to mirror exactly; confirmed Helm `_helpers.tpl` checksum gate that must be updated; confirmed `configure.md` prose that becomes false and must be corrected (D-06). |
| REQ-reindex-resume-tags | `--resume` re-embeds a record whose tags changed while content did not; tag comparison order-independent. | Confirmed source-side tag decode (`fromPayload`, `store.go:562-568`); confirmed target Get already fetches full payload (`qdrant.NewWithPayload(true)` = all fields, not a selector) so tags are already retrievable; confirmed no existing helper decodes tags a second way — `fromPayload` is the one true decoder to reuse; confirmed `EmbedText` joins tags in slice order (`store.go:277-282`), grounding D-09's documented residual; confirmed `payload()` always writes a `tags` key (possibly empty list) so nil-vs-empty must be normalized (D-10). |
| REQ-reindex-stale-repair | Operator can heal records an earlier unpatched `--resume` run skipped, via a documented path. | Confirmed vector+payload are written atomically in one `Upsert` call from one source read (`store.go:2741-2751`), so D-13's "patched resume heals everything" premise holds; confirmed `DryRun` branch short-circuits before the resume lookup today (`store.go:2690-2694`), grounding D-14; confirmed `reindex.md`'s existing heading structure for where to place the new section; confirmed `upgrade.md` already has a `## v0.12.0` heading with numbered `### N.` subsections — the new entries are `### 4.`/`### 5.`, not a new top-level heading. |
</phase_requirements>

## Standard Stack

No new libraries. This phase is entirely in-repo Go (stdlib `cmp`, `sort`, `net/url` — all already imported in the touched files) plus Helm/YAML plus Markdown docs. **Zero new Go dependencies** — confirmed by inspecting every touched file's existing imports; no new `import` statements are required for either fix.

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `cmp` (stdlib) | go1.26.3 (per `go.mod`) | `cmp.Or` fallback resolution, already used at `tools.go:423` for `ChatBaseURL` | Established in-repo pattern (D-12 from the base-URL phase); D-02 explicitly asks for the identical idiom |
| `github.com/qdrant/go-client/qdrant` | already a direct dependency | `GetListValue()`/`GetValues()` payload decode, already used in `fromPayload` | Existing client already used throughout `internal/store`; no version change needed |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sort` (stdlib) | go1.26.3 | Order-independent tag-set comparison (D-09) | Sort a copy of both tag slices before comparing, or build a `map[string]struct{}` — either is stdlib-only |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Sorting a copy for tag-set equality | `slices.Sort` + `slices.Equal` (Go 1.21+ stdlib `slices` package) | Either works at go1.26.3; `slices.Equal` after `slices.Sort`-ing copies is marginally more idiomatic than a hand-rolled loop and is still stdlib — **recommend `slices.Sort`/`slices.Equal` over a hand-rolled map**, since a map-based set-equality check silently collapses duplicate tags (`["a","a","b"]` vs `["a","b"]` would incorrectly compare equal), which a sorted-slice comparison does not. |
| A dedicated tag-diff helper package | Inline in `store.go` near `EmbedText`/the predicate | Claude's Discretion per CONTEXT.md — no new package boundary is needed for a ~10-line comparison |

**Installation:** none — no `go get` required, `go.mod`/`go.sum` must show zero diff (milestone-wide constraint, confirmed in `.planning/REQUIREMENTS.md`'s Out of Scope table: *"New Go dependencies... A new dependency in this milestone needs its own justification"*).

**Version verification:** N/A — no package versions to verify; this phase touches only stdlib and the already-vendored `qdrant` client.

## Package Legitimacy Audit

Not applicable — this phase introduces zero new packages (Go, npm, or otherwise). Skipping the gate per the "Required whenever this phase installs external packages" condition, which does not apply here.

## Architecture Patterns

### System Architecture Diagram

```
#350 — per-lane credential                    #345/SC3 — reindex resume tag-awareness
────────────────────────────                  ──────────────────────────────────────

ENGRAM_OPENAI_CHAT_API_KEY (env)               `engram reindex --target T --resume [--dry-run]`
        │                                              │
        ▼                                              ▼
internal/config.registry (koanf)              cmd/engram/reindex.go (reindexCmd.RunE)
        │  (Key/Env/Legacy/Default/Flag)              │
        ▼                                              ▼
config.OpenAIConfig.ChatAPIKey                store.Store.Reindex(ctx, opts, embed)
        │                                              │
        ▼                                       ┌──────┴──────┐
internal/server/tools.go                        │             │
  summarizerFromConfig(cfg)                  DryRun=true   DryRun=false
    chatAPIKey := cmp.Or(                        │             │
      cfg.OpenAI.ChatAPIKey,                     │      opts.Resume?
      cfg.OpenAI.APIKey)                         │        │      │
        │                                        │       yes     no
        ▼                                        │        │      │
summarize.New(chatBaseURL,                       │        ▼      ▼
  chatAPIKey, model, ...)                        │  reindexTargetContents()
        │                                        │  (Get target ids,
        ▼                                        │   WithPayload(true) = ALL fields
  HTTP POST {chatBaseURL}/chat/completions        │   → decode content+identity+TAGS
  Authorization: Bearer {chatAPIKey}              │   via fromPayload-equivalent path)
                                                   │        │
                                                   ▼        ▼
                                            (today: count  per-point skip predicate:
                                             scanned only,  content match AND
                                             no resume      tags match (order-
                                             lookup —       independent, nil==empty)
                                             D-14 gap)      AND identity match
                                                                  │
                                                          ┌───────┴───────┐
                                                         yes             no
                                                          │               │
                                                          ▼               ▼
                                                    Unchanged++    embed(EmbedText(
                                                    (skip)          content, tags))
                                                                    → Upsert{vector,
                                                                       payload} atomically
                                                                    → Upserted++
```

### Recommended Project Structure

No new files or directories. Both fixes land in existing files:

```
internal/config/
├── registry.go       # +1 field entry (D-01)
└── config.go          # +1 struct field on OpenAIConfig (D-01)

internal/server/
└── tools.go            # +1 line in summarizerFromConfig (D-02)

internal/store/
├── store.go             # reindexTarget +Tags field, reindexTargetContents tag decode
│                        # (D-07/D-08), skip predicate +tag conjunct (D-09/D-10/D-11),
│                        # DryRun branch wired to resume lookup (D-14)
└── reindex_test.go      # new test(s): paired positive control (D-12), dry-run+resume sizing

cmd/engram/
└── reindex.go            # reindexSummary wording only if dry-run+resume counts change meaning

charts/engram/
├── values.yaml            # +1 secretKeyRef stanza (D-04 — location TBD, see Finding 1)
└── templates/_helpers.tpl # +1 conditional env block (D-04)

docs-site/src/content/docs/guides/
├── configure.md            # correct the false "shared key" sentence (D-06); +table row
├── reindex.md               # +"Repairing a pre-patch resume" section (D-16)
└── upgrade.md                # +2 numbered subsections under existing `## v0.12.0` (D-06/D-16)
```

### Pattern 1: Construction-site fallback resolution (D-02's shape)

**What:** Never resolve an "inherit shared value" fallback at config-load time; resolve it once, at the point where the value is consumed, via `cmp.Or`.
**When to use:** Any config field with an "empty means inherit sibling field" semantic.
**Example (existing, to mirror exactly):**
```go
// Source: internal/server/tools.go:419-428 (live tree, read this session)
func summarizerFromConfig(cfg *config.Config) *summarize.Client {
	// cmp.Or resolved once here (D-12): ChatBaseURL wins when set, otherwise
	// the shared BaseURL. The embedder below is deliberately left untouched —
	// it always uses BaseURL regardless of ChatBaseURL.
	chatBaseURL := cmp.Or(cfg.OpenAI.ChatBaseURL, cfg.OpenAI.BaseURL)
	return summarize.New(chatBaseURL, cfg.OpenAI.APIKey, cfg.Summarize.Model, summaryMaxChars(cfg),
		summarize.WithHTTPTransport(otelhttp.NewTransport(http.DefaultTransport)),
		summarize.WithMaxTokens(summaryMaxTokens(cfg)),
		summarize.WithTimeout(summaryTimeout(cfg)))
}
```
D-02's edit is a single additional `cmp.Or` line plus swapping `cfg.OpenAI.APIKey` for the resolved local variable as the second `summarize.New` argument — **`summarize.New` has exactly one call site in production code** (confirmed via `rg -n 'summarize\.New\('` — the only production match is inside `summarizerFromConfig` itself; `summarizerFromConfig` itself has two call sites, `tools.go:262` and `tools.go:447`, both of which call the function, not `summarize.New` directly, so both automatically pick up the fix with zero additional edits).

### Pattern 2: Registry-driven config field (D-01's shape)

**What:** A brand-new env var with no legacy alias and no default is a one-line `field{}` literal.
**Example (existing sibling, to copy verbatim and adapt):**
```go
// Source: internal/config/registry.go:51 (live tree, read this session)
{Key: "openai.chat_base_url", Env: "ENGRAM_OPENAI_CHAT_BASE_URL"},
```
New entry (same shape, one line below it):
```go
{Key: "openai.chat_api_key", Env: "ENGRAM_OPENAI_CHAT_API_KEY"},
```
And the struct field (mirroring `ChatBaseURL`'s doc-comment style, `internal/config/config.go:132-137`):
```go
// ChatAPIKey is the ENGRAM_OPENAI_CHAT_API_KEY credential for the
// chat/summarize lane. Empty (the default; no registry default is set) means
// inherit APIKey — the fallback is resolved once, at the summarizer
// construction site (cmp.Or in summarizerFromConfig), not here (D-02/D-03).
ChatAPIKey string `koanf:"chat_api_key"`
```

### Pattern 3: Resume skip predicate as a growing conjunction (D-09's framing)

**What:** The resume skip test is already a 2-conjunct-plus-guard shape (content match AND identity guard); tags become a third conjunct, not a redesign.
**Example (existing predicate to extend, read this session):**
```go
// Source: internal/store/store.go:2716-2727 (live tree)
if ti, ok := targetInfo[p.Id.GetUuid()]; ok && ti.content == content &&
    (opts.Identity == "" || ti.identity == opts.Identity) {
    // Target already holds this id with identical content — equal
    // content (and, from the same source payload, equal tags) re-embeds
    // to an equal vector, so skip the embed+upsert. But only when no
    // Identity is being enforced, or the target already carries the
    // matching stamp: a content match with an absent/stale identity
    // falls through and gets re-embedded+restamped below, so resume
    // never leaves a record untraceable to its embedder config.
    res.Unchanged++
    continue
}
```
The comment's parenthetical `(and, from the same source payload, equal tags)` is the exact stale claim D-11 requires rewriting — it was true when this comment was written (before tags were compared at all, the assumption was implicit and undocumented) but is false as a statement about the code's actual guarantee, since `ti` comes from the **target**, not "the same source payload." The rewritten comment should state plainly: content equality does NOT imply tag equality, because `ti.content`/`ti.tags` are a snapshot of whatever the target held at its last write, while `content`/`m.Tags` are the source's current values.

### Pattern 4: `fromPayload` as the single source-of-truth decoder (D-08's requirement)

**What:** `fromPayload` already knows how to decode `"tags"` out of a raw Qdrant payload map into `[]string`. `reindexTargetContents` should call it (or extract just the tags-decode fragment into a tiny shared helper) rather than writing a second, subtly-different decode loop.
**Source (verified, live tree, `store.go:562-568`):**
```go
if v, ok := p["tags"]; ok {
    if lv := v.GetListValue(); lv != nil {
        for _, item := range lv.GetValues() {
            m.Tags = append(m.Tags, item.GetStringValue())
        }
    }
}
```
**Recommended integration:** in `reindexTargetContents` (`store.go:2782-2807`), either (a) call `fromPayload(p.Id.GetUuid(), p.Payload).Tags` and take just the `.Tags` field (simplest, guarantees byte-identical decode, small extra cost of decoding fields you discard), or (b) extract the tags-decode fragment above into a small unexported `tagsFromPayload(p map[string]*qdrant.Value) []string` helper called from both `fromPayload` and `reindexTargetContents` (avoids the discard cost, keeps the "one true decoder" invariant explicit rather than incidental). Either satisfies D-08; option (b) is slightly more defensible under a "why does this work" code review question since the shared-decoder claim becomes structural (one function, two callers) rather than an implicit consequence of calling the bigger function.

### Anti-Patterns to Avoid

- **Hand-rolling a second `GetListValue()`/`GetValues()` loop in `reindexTargetContents`:** this is precisely the encoding-asymmetry hazard D-08 warns about — a second decode loop that differs in any way (e.g., not calling `.GetValues()` the same way, or mishandling a `nil` `ListValue`) produces false re-embeds forever, which is worse than today's bug (today's bug at least converges once every record's content changes once; a decode asymmetry never converges).
- **Adding a field-subset `WithPayload` selector "to be efficient":** don't. `reindexTargetContents`'s `Get` call already uses `qdrant.NewWithPayload(true)` (= all fields), which is what makes tags already-fetchable with zero query-shape change. Narrowing it to a field list would be a **regression**, not an optimization — verified this session that `NewWithPayload(true)` is an alias for `NewWithPayloadEnable()`, not a field selector (`go doc github.com/qdrant/go-client/qdrant.NewWithPayload`).
- **Map-based tag-set equality (`map[string]struct{}`) for D-09's order-independence:** silently treats `["a","a","b"]` and `["a","b"]` as equal (duplicate collapse). Use `slices.Sort` on copies + `slices.Equal` instead, which preserves multiplicity.
- **Putting the new Helm value at `memory.openai.chatApiKeySecret`** without first checking where `chatBaseURL` actually lives in the chart — see Finding 1 below. `memory.openai.*` is the **embedder's** namespace in this chart; the chat/summarize lane's own overrides already live under `memory.summarize.*` (`chatBaseURL` is there today, not under `memory.openai`).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|--------------|-----|
| Order-independent slice equality | A custom counting/hashing comparator | `slices.Sort` (on copies) + `slices.Equal`, stdlib | Already correct, already imported patterns elsewhere in Go stdlib usage across the repo; avoids the duplicate-collapse bug of a naive set |
| Tag payload decoding | A second `GetListValue`/`GetValues` loop | Reuse/extract `fromPayload`'s existing decode (`store.go:562-568`) | D-08's explicit requirement; the existing decoder is the only tested, correct one |
| A brand-new one-time reconciliation command for stale-tag repair | `engram repair-stale-tags` | Re-running `engram reindex --resume` (patched) | D-13, verified: vector+payload are written atomically from the same source read, so nothing a dedicated command could do differently exists to build |

**Key insight:** both defects in this phase have their "correct" fix already partially built and shipped in this exact codebase (the `chat_base_url` split for #350; the `opts.Identity` resume conjunct for #345) — the highest-value action for the planner is literal pattern-matching against those, not designing anything new.

## Common Pitfalls

### Pitfall 1 (Finding 1 — the one falsified/unclear premise): D-04's Helm value location doesn't match the chart's own namespace convention

**What goes wrong:** CONTEXT.md's D-04 says to add `memory.openai.chatApiKeySecret`. But reading `charts/engram/values.yaml` and `_helpers.tpl` this session shows the chart does **not** organize values by config-key namespace (`openai.*`) — it organizes them by **lane**. `memory.openai.*` holds only the **embedder's** base URL, key, and CA-bundle settings (`values.yaml:70-85`). The chat/summarize lane's *own* override, `chatBaseURL` (which maps to the very same `openai.chat_base_url` config key this phase's key sibling maps to), already lives at `memory.summarize.chatBaseURL` (`values.yaml:101`, rendered by `_helpers.tpl:34-36` into `ENGRAM_OPENAI_CHAT_BASE_URL`). Config-key namespace (`openai.*`) and Helm-value namespace (`memory.openai.*` vs `memory.summarize.*`) have **already diverged on purpose** for this exact sibling field.

**Why it happens:** D-04 was likely drafted by analogy to the *embedder's* `apiKeySecret` (`memory.openai.apiKeySecret`) without cross-checking where its sibling `chatBaseURL` actually landed in the same chart.

**How to avoid:** Put the new value at `memory.summarize.chatApiKeySecret` (mirroring `memory.summarize.chatBaseURL`'s location), rendered by a `secretKeyRef`-shaped conditional block (mirroring `memory.openai.apiKeySecret`'s *shape*, `_helpers.tpl:20-26`) but placed in `_helpers.tpl` immediately after the `chatBaseURL` `with` block (`_helpers.tpl:34-36`), not after the embedder's `apiKeySecret` block. This keeps "everything about the chat lane" grouped together in both `values.yaml` and the render output, matching the existing `chatBaseURL` precedent exactly. **This is a planning decision the phase plan should make explicitly (with a one-line rationale), not silently follow CONTEXT.md's literal path** — CONTEXT.md's own Decisions section frames D-04 as "mirror the existing pattern," and the pattern it should mirror is `chatBaseURL`'s location, not `apiKeySecret`'s.

**Warning signs:** if the plan's Task 1 diff touches `memory.openai.*` in `values.yaml`, re-check this finding before executing.

### Pitfall 2: The `chart:validate` checksum gate will break on ANY edit to `_helpers.tpl`'s `engram.containerEnv` block — this is expected, not a bug to route around

**What goes wrong:** `Taskfile.yaml`'s `chart:validate` target pins a SHA-256 checksum (`EXPECTED_CHECKSUM`) of the entire `engram.containerEnv` named-template block in `_helpers.tpl` (lines 158-176). Adding the new `chatApiKeySecret` conditional will change that checksum and **fail `task chart:validate`/`task chart:lint`/`task`** until the checksum constant is updated.

**Why it happens:** This is a deliberate drift-guard (documented in the Taskfile comment: *"Any edit to this block must be a deliberate, verified change... this checksum guards against silent drift"*), already re-pinned twice before in this exact milestone (2026-07-26 for the `chat_base_url` row, 2026-07-31 for `connect.headless`).

**How to avoid:** The plan's Task must include: (1) make the `_helpers.tpl` edit, (2) run `awk '/define "engram.containerEnv"/{f=1} f{print} f && /^\{\{- end -\}\}$/{exit}' charts/engram/templates/_helpers.tpl | shasum -a 256 | awk '{print $1}'` to compute the new checksum, (3) update `EXPECTED_CHECKSUM` in `Taskfile.yaml`'s `chart:validate` target, (4) manually re-verify the default (no `chatApiKeySecret` set) render is byte-identical to the pre-phase render (the Taskfile comment's own stated verification bar), and (5) add a one-line dated comment to the Taskfile matching the existing two precedents' style ("re-pinned YYYY-MM-DD at plan NN-NN for the with-guarded ENGRAM_OPENAI_CHAT_API_KEY row").

**Warning signs:** `task chart:validate` failing with `"engram.containerEnv block in _helpers.tpl has drifted"` after an otherwise-correct Helm edit — this is the gate doing its job, not a false failure.

### Pitfall 3: D-13's "patched resume heals everything" is TRUE for records touched by `Reindex`, but only as long as the source is still live (already covered by D-15)

**What goes wrong:** It's tempting to read D-13 as "any stale-tag record anywhere heals on re-run." It only heals records reachable via the **current** source collection — confirmed this session: `Reindex` always re-scrolls `source` fresh on every invocation (`store.go:2680-2689`), so a patched `--resume` re-run compares the source's **current** content+tags (which reflect any `update_memory` tag edits made after the original buggy run) against the target's stale snapshot, and re-embeds+overwrites when they differ. This is exactly what makes D-13 true, but it silently stops being true the moment the source collection is gone (D-15 already documents this as a stated limit — no code change needed, just don't let the docs imply otherwise).

**Why it happens:** Easy to conflate "the target is self-healing" (false — the target has no memory of what it should look like) with "the source is authoritative and re-read every time" (true — the actual mechanism).

**How to avoid:** When writing the `guides/reindex.md` "Repairing a pre-patch resume" section (D-16), state the mechanism explicitly (source is re-read fresh, not the target self-correcting) so an operator who has since deleted the source collection doesn't run `--resume` expecting a heal and get silent skips.

**Warning signs:** none at the code level (D-15 says implement nothing) — this is purely a docs-precision pitfall.

### Pitfall 4: `payload()` always writes a `tags` key (even when `Tags` is `nil`/empty) — decode-side nil-vs-empty handling is the only place D-10 needs to hold

**What goes wrong:** Verified this session (`store.go:459-463`): `payload()` does `tags := make([]any, len(m.Tags)); ...; p["tags"] = tags` unconditionally — so every record written through the normal `Upsert` path carries a `"tags"` key with at least an empty list, never an absent key. An absent `"tags"` key can still occur on **raw** (non-`Memory`-round-tripped) points such as the `seedSource` test helper's `rawID` point (`reindex_test.go:124-140`, no `"tags"` key in its raw `qdrant.NewValueMap`) or genuinely legacy pre-tags records. `fromPayload`'s decode (`store.go:562-568`) yields `nil` for `m.Tags` in both the "key absent" and "key present with an empty list" cases (the `append` inside the loop is simply never reached). So the source and target read paths already naturally converge nil-vs-empty to the same Go zero value — **D-10 is largely already satisfied by construction**, as long as the new target-side decode reuses the same `fromPayload`-style logic (Pattern 4 above) rather than a hand-rolled loop that might, e.g., initialize `tags := []string{}` before the loop (which would make empty-but-present differ from absent as `[]string{}` vs `nil`).

**Why it happens:** A naive re-implementation of the tags decode (rather than reusing `fromPayload`'s exact shape) is the likeliest way to accidentally reintroduce a nil-vs-empty distinction.

**How to avoid:** Use `slices.Equal` (which already treats a `nil` slice and an empty non-nil slice as equal — confirmed Go stdlib behavior, `slices.Equal(nil, []string{})` is `true`) on the **sorted copies**, and reuse the exact `fromPayload` decode shape (Pattern 4) so both sides can only ever produce `nil` or a populated slice, never an empty-but-non-nil slice, making the nil-vs-empty question moot in practice as well as by `slices.Equal`'s own semantics (defense in depth).

**Warning signs:** a test asserting `TestReindexResumeSkipsUnchanged`-style behavior for an **untagged** record starts failing (would indicate a nil/empty asymmetry was introduced).

## Runtime State Inventory

Not applicable — this is not a rename/refactor/migration phase. No env var, config key, or identifier is being renamed; `openai.chat_api_key` / `ENGRAM_OPENAI_CHAT_API_KEY` and the reindex predicate change are net-new/corrective, not renames of anything with existing runtime state (no stored data, live-service config, OS-registered state, secrets, or build artifacts carry a name being changed). The reindex fix changes *behavior* of an existing operator command against existing Qdrant data, which is covered under Pitfall 3/D-15 above (a data-repair concern, not a rename concern) rather than this inventory.

## Code Examples

### Order-independent tag-set comparison (new code for D-09/D-10)

```go
// Not yet in the tree — recommended shape, stdlib-only (slices package,
// available since Go 1.21; go.mod pins go 1.26.3).
import "slices"

// tagsEqual reports whether a and b contain the same tags, ignoring order and
// treating nil and empty as equal (slices.Equal already does both — verified:
// slices.Equal(nil, []string{}) == true). Does NOT collapse duplicates (a
// map-based set comparison would); a genuine duplicate-count difference is
// treated as a real difference, matching content-equality's own byte-exact
// semantics.
func tagsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sa, sb := slices.Clone(a), slices.Clone(b)
	slices.Sort(sa)
	slices.Sort(sb)
	return slices.Equal(sa, sb)
}
```

### Extending the skip predicate (D-07/D-09/D-11)

```go
// Source: internal/store/store.go:2716-2727, extended per D-07/D-09/D-11
// (illustrative — exact target-struct field name/decode call is the planner's
// to finalize per Pattern 4 above).
if ti, ok := targetInfo[p.Id.GetUuid()]; ok && ti.content == content &&
    tagsEqual(ti.tags, m.Tags) &&
    (opts.Identity == "" || ti.identity == opts.Identity) {
    // Target already holds this id with identical content AND tags (compared
    // order-independently — a tags-permuted-only record is NOT re-embedded,
    // a documented residual: EmbedText joins tags in slice order, so its
    // embedded text differs, but resume intentionally treats a pure
    // reordering as unchanged; see EmbedText at store.go:277-282).
    // ti's content/tags are a snapshot of the TARGET's last write, not "the
    // same source payload" — content and tags are independently mutable
    // (e.g. via update_memory), so this is a two-part equality check, not
    // one implying the other.
    res.Unchanged++
    continue
}
```

### `--dry-run --resume` sizing (D-14 — illustrative shape only, exact field naming is Claude's Discretion)

```go
// Source: internal/store/store.go:2690-2705, current shape (live tree, read
// this session) — the DryRun branch never looks at opts.Resume today:
if opts.DryRun {
	res.Scanned += uint64(len(pts))
} else {
	var targetInfo map[string]reindexTarget
	if opts.Resume {
		targetInfo, err = s.reindexTargetContents(ctx, opts.Target, pts)
		// ...
	}
	// ... per-point embed/upsert loop, using targetInfo for the skip check
}
```
To honor D-14 without writing anything: run the SAME `reindexTargetContents` lookup and the SAME skip predicate inside the `DryRun` branch, incrementing `res.Unchanged`/a new "would-upsert" counter instead of calling `embed`/`Upsert`. This requires `opts.Target` to already exist for a dry run to inspect it (today `ensureCollection` is skipped entirely under `DryRun`, `store.go:2670-2674`) — **the target must already exist from a prior real run** for `--dry-run --resume` to report anything meaningful; a `--dry-run --resume` against a target that does not exist yet should report the same "would re-embed everything" result as today (⁣`reindexTargetContents` on a nonexistent collection will error — the plan needs to decide whether that's a hard error or a "target does not exist yet, nothing to resume" soft-report; recommend checking `CollectionExists` first, mirroring the existing source-existence check at `store.go:2662-2668`, and treating a missing target under `DryRun+Resume` as "0 unchanged, all scanned would be upserted" rather than an error, since a first-ever dry run naturally has no target to resume against).

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Resume skip predicate: content + identity only | Resume skip predicate: content + tags + identity | This phase (#345) | Tags-only edits are correctly re-embedded instead of silently skipped |
| Chat/summarize lane shares the embedder's API key unconditionally | Chat/summarize lane can carry its own key, falling back to the shared key when unset | This phase (#350) | Operators splitting embedder/chat gateways can use distinct credentials, closing a gap the already-shipped `chat_base_url` split left open |
| `--dry-run --resume` reports the full scanned count regardless of resume state | `--dry-run --resume` reports a meaningful stale/would-re-embed count | This phase (D-14) | The dry-run-as-preflight pattern becomes usable for sizing a repair before running it |

**Deprecated/outdated:** none — no APIs or patterns are being removed in this phase; the base-URL split's `chat_base_url` remains as-is, and the phase only adds a sibling field.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The recommended `_helpers.tpl` placement — immediately after the `chatBaseURL` `with` block, grouped under a "chat lane" comment — is the best rendering location, though CONTEXT.md's D-04 literally named `memory.openai.chatApiKeySecret`. This is a recommendation from reading the chart's existing convention, not something an authoritative Helm style doc confirms (no such doc exists for this chart). | Common Pitfalls, Pitfall 1 | If the planner disagrees and keeps `memory.openai.chatApiKeySecret`, the phase still functions correctly (Helm value naming is not load-bearing for correctness) — the only cost is a chart that no longer groups "chat lane" values together, a discoverability/consistency regression, not a functional one. Low risk either way; worth a plan-level explicit decision either direction. |
| A2 | `slices.Equal(nil, []string{})` returns `true` in Go's stdlib `slices` package. | Code Examples | This is standard, well-known Go stdlib `slices` package behavior (element-wise comparison of two slices of equal length, `0`, is vacuously true regardless of nilness) and was not independently executed against the live `go.mod`-pinned Go version this session, though it is definitional — not version-dependent — behavior of `slices.Equal`. Extremely low risk; the planner may still choose to verify with a one-line `go run` before locking the approach if maximal certainty is wanted. |

**If this table is empty:** N/A — two low-risk assumptions logged above; both are reasoning/recommendation calls, not unverified factual claims about the codebase (every codebase claim in this document was read from the live tree this session).

## Open Questions

1. **Should `--dry-run --resume`'s counters reuse `ReindexResult.Unchanged`/`Scanned` or add a new field?**
   - What we know: `ReindexResult` today has exactly `Scanned`/`Upserted`/`Skipped`/`Unchanged` (`store.go:2569-2574`); a dry run currently only ever populates `Scanned`. `reindexSummary` (`cmd/engram/reindex.go:90-98`) has a dedicated `dryRun` branch that only prints `Scanned`.
   - What's unclear: whether reusing `Unchanged`/computing `Upserted` under `DryRun` (without ever writing) is confusing given those field names' current "only meaningful under a real run" connotation, versus adding e.g. `WouldUpsert`/`WouldSkip` fields that are ONLY populated under `DryRun`.
   - Recommendation: this is explicitly Claude's Discretion per CONTEXT.md. Reusing `Upserted`/`Unchanged` (computed via the same predicate, just never acted on) is the smaller diff and keeps one counter meaning across dry/real runs ("this many records would be/were re-embedded"); the planner should pick one and state the reasoning in the plan, since `reindexSummary`'s dry-run wording must stay honest either way (D-14's own release condition).

2. **Exact target-struct extension: add a field to `reindexTarget`, or extract a shared decode helper first?**
   - What we know: `reindexTarget` is a 2-field unexported struct (`content string`, `identity string`, `store.go:2770-2773`) used only within `reindexTargetContents`/the predicate. D-07 explicitly says "gains a `tags []string`."
   - What's unclear: nothing structurally — D-07 is unambiguous about the struct extension. The only open call is Pattern 4's (a) vs (b) choice (call `fromPayload` wholesale vs. extract a shared tags-only decode helper), which is Claude's Discretion ("exact naming of any new comparison helper... whether it lives in internal/store beside EmbedText or unexported next to the predicate").
   - Recommendation: extract the shared decode fragment (option b) — it makes D-08's "same path" claim structurally checkable in review (one function, two call sites) rather than relying on someone noticing `fromPayload` happens to also be called here.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All code changes | ✓ | go1.26.5 (module pins `go 1.26.3`, compatible) | — |
| Qdrant (via testcontainers or `ENGRAM_QDRANT_TEST_ADDR`) | D-12's paired-control test, all `internal/store` reindex tests | ✓ (pattern) | `dialTestClient` (`store_test.go:88-104`) skips gracefully with `t.Skip` if neither is configured — verified this session; not independently confirmed a live Qdrant is running in this environment right now, but the test harness already handles absence gracefully, so this is a non-blocking runtime concern, not a planning one | — (test skips rather than fails when unavailable, per existing harness design) |
| Helm | `task chart:validate`/`chart:lint` for D-04's checksum re-pin | not independently probed this session — assume available per `Taskfile.yaml`'s existing `helm lint`/`helm template` invocations, which are load-bearing for every other phase in this milestone (e.g., the `connect.headless` and `chat_base_url` chart work already shipped) | — | — |

**Missing dependencies with no fallback:** none identified — this phase has no dependency that both (a) is missing and (b) has no fallback.

**Missing dependencies with fallback:** Qdrant, if unavailable, causes the new/existing `internal/store` tests to skip rather than fail — acceptable for a dev/CI environment where the store test suite already depends on it project-wide (not a new constraint this phase introduces).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing`, real Qdrant via testcontainers or `ENGRAM_QDRANT_TEST_ADDR` (`internal/store/store_test.go:88-104`) |
| Config file | none — no `pytest.ini`/`jest.config`-equivalent; Go test discovery is directory-based |
| Quick run command | `go test ./internal/store/... -run TestReindex -v` (scoped to reindex tests) |
| Full suite command | `task` (lint + full repo test suite, per `CLAUDE.md`'s "Task runner" convention) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|--------------------|-------------|
| REQ-per-lane-api-key | Chat lane uses its own key when set | unit (httptest server asserting `Authorization` header) | `go test ./internal/server/... -run TestSummarizerFromConfigChatAPIKey -v` | ❌ Wave 0 — new test, but the exact template exists: `TestSummarizerFromConfigChatBaseURL` (`internal/server/embed_wiring_test.go:92-157`) is the byte-for-byte pattern to copy, asserting on `r.Header.Get("Authorization")` instead of `r.URL.Path`/which server received the request |
| REQ-per-lane-api-key | Behavior unset is byte-identical | unit | same test, second subtest (mirroring `TestSummarizerFromConfigChatBaseURL`'s two-subtest shape: `"chat base URL set..."` / `"chat base URL empty falls back..."`) | ❌ Wave 0 — same file/test as above |
| REQ-reindex-resume-tags | Content-same/tags-differ re-embeds; content-and-tags-same skips; same-elements-different-order skips (D-12's three cases) | integration (real Qdrant, testcontainers) | `go test ./internal/store/... -run TestReindexResumeTags -v` | ❌ Wave 0 — new test, but `TestReindexResumeSkipsUnchanged` (`internal/store/reindex_test.go:362-417`) is the exact seed/mutate/re-run/`Batch:1` template to extend or sibling |
| REQ-reindex-stale-repair | `--dry-run --resume` reports a meaningful count without writing | integration | `go test ./internal/store/... -run TestReindexDryRunResume -v` | ❌ Wave 0 — new test; `TestReindexDryRunWritesNothing` (`reindex_test.go:419-446`) is the "asserts nothing written/no target created" half to extend with a resume+meaningful-count assertion |

### Sampling Rate
- **Per task commit:** scoped `go test ./internal/server/... -run TestSummarizerFromConfig` / `go test ./internal/store/... -run TestReindex`
- **Per wave merge:** `task` (full lint + suite)
- **Phase gate:** full suite green before `/gsd-verify-work`, plus `task chart:validate` (D-04's checksum gate), `go vet ./...`, `git diff --exit-code go.mod go.sum` (zero-new-dependency proof)

### Wave 0 Gaps
- [ ] `internal/server/embed_wiring_test.go` — add `TestSummarizerFromConfigChatAPIKey` (two subtests, mirroring `TestSummarizerFromConfigChatBaseURL`'s exact shape) — covers REQ-per-lane-api-key
- [ ] `internal/store/reindex_test.go` — add the D-12 paired-positive-control test (three cases: content-same/tags-differ, content-and-tags-same, same-elements-reordered) — covers REQ-reindex-resume-tags
- [ ] `internal/store/reindex_test.go` — extend or add a test asserting `--dry-run --resume` reports a nonzero would-upsert/would-skip split without creating the target or writing — covers REQ-reindex-stale-repair
- [ ] No new test framework or fixture infrastructure is needed — `dialTestClient`/`seedSource`/`scrollPoints`/`payloadKeysEqual` (all in `internal/store/*_test.go`) already cover every fixture need this phase has, including a tags-carrying seed point (`seedSource`'s `full` Memory already sets `Tags: []string{"x", "y"}`, `reindex_test.go:115`)

## Security Domain

`security_enforcement` is not set to `false` in `.planning/config.json` (absent under `workflow`; the present keys are `nyquist_validation: true` and `ai_integration_phase: true`), so this section is included per the default-enabled instruction, scoped to what actually applies.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | no | This phase does not touch caller authentication (OIDC/bearer) — it configures an **outbound** credential the server sends to a third-party LLM provider, not an inbound auth check |
| V3 Session Management | no | Not touched |
| V4 Access Control | no | Not touched — no authz/authn/isolation logic changes; reindex already operates outside the owner-isolation model (it copies raw payloads verbatim, `store.go:2585-2601`, unchanged this phase) |
| V5 Input Validation | partial | D-05 explicitly declines startup validation for the new API key (correct — an API key has no verifiable shape, matching how `ENGRAM_OPENAI_API_KEY` itself is already unvalidated at startup, confirmed no `validate.go` branch exists for it either) |
| V6 Cryptography (secrets handling) | yes | The new `ChatAPIKey` field must never be logged. Confirmed this session: no existing code path logs `cfg.OpenAI.APIKey`/`ChatBaseURL` (`rg` across `internal/server`/`internal/config`/`cmd/engram` found zero logging call sites touching these fields) — the new field should be held to the same "never appears in a log line" bar by construction (it flows only into an HTTP `Authorization` header via the `summarize` client, never through `slog`) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|-----------------------|
| API key leaking via Helm-rendered manifest (plaintext in a `ConfigMap`/inline env value instead of a `Secret` ref) | Information Disclosure | D-04 already specifies a `secretKeyRef` shape (mirroring the existing `apiKeySecret` pattern, `_helpers.tpl:20-26`), not an inline value — this is the correct mitigation and is already the locked decision, not something to newly design |
| Credential sent to the wrong endpoint if `ChatBaseURL`/`ChatAPIKey` are independently misconfigured (key for gateway A sent to gateway B) | Information Disclosure (credential exposure to an unintended third party) | Out of scope for this phase — this is the exact risk `configure.md`'s existing "shared-key" callout (lines 64-73) currently warns about for the *shared*-key case; once split, the risk becomes "operator points `ChatBaseURL` at gateway B but forgets to set `ChatAPIKey`, so gateway A's key silently reaches gateway B" — this is a **behavior-unchanged** risk (identical to today's shared-key-by-default behavior when only `ChatBaseURL` is set and `ChatAPIKey` is left empty), not a new one introduced by this phase, but `configure.md`'s corrected prose (D-06) should make this residual risk explicit rather than silently dropping the warning that used to explain it |
| Stale/incorrect vector silently served for a mutated record (the #345 defect itself) | Tampering (data integrity, not an attacker-driven threat but a correctness/trust threat to recall accuracy) | This IS the defect this phase fixes — not a residual risk, the requirement itself |

## Sources

### Primary (HIGH confidence — all read from the live repository this session)
- `internal/server/tools.go:230-270, 400-450` — `summarizerFromConfig`, `embedderFromConfig`, all call sites
- `internal/config/config.go:110-160` — `OpenAIConfig`, `SummarizeConfig` struct definitions
- `internal/config/registry.go:1-80` — the full field registry, `openai.*` block
- `internal/config/validate.go:90-125` — `ChatBaseURL` validation branch and its explicit "do not copy this onto an inherit-by-default field" comment
- `internal/store/store.go:277-282, 459-522, 524-640, 2569-2807` — `EmbedText`, `payload()`, `fromPayload`, `Reindex`, `reindexTarget`, `reindexTargetContents`
- `internal/store/reindex_test.go:1-160, 355-750` — `seedSource`, `scrollPoints`, `TestReindexResumeSkipsUnchanged`, `TestReindexDryRunWritesNothing`, `TestReindexResumeRestampsStaleIdentity`
- `internal/store/store_test.go:88-104` — `dialTestClient`
- `internal/server/embed_wiring_test.go:1-158` — `TestSummarizerFromConfigChatBaseURL` (the exact test template for the new D-01–D-03 test)
- `charts/engram/values.yaml`, `charts/engram/templates/_helpers.tpl` — full files read
- `Taskfile.yaml:126-176` — `chart:lint`/`chart:validate`, including the containerEnv checksum gate
- `docs-site/src/content/docs/guides/configure.md` (full), `guides/reindex.md` (full), `guides/upgrade.md:108-158` — the exact prose to correct/extend
- `go doc github.com/qdrant/go-client/qdrant.NewWithPayload` — confirmed `true` = all-fields alias, not a selector (run this session)
- `.planning/REQUIREMENTS.md`, `.planning/STATE.md`, `.planning/config.json` — project requirements, decision history, workflow config

### Secondary (MEDIUM confidence)
- none — no external/web sources were needed for this phase; every claim traces to a file read this session

### Tertiary (LOW confidence)
- A2 in the Assumptions Log (`slices.Equal(nil, []string{})` behavior) — definitional stdlib behavior, not independently executed this session

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies, all patterns already shipped in this exact repo
- Architecture: HIGH — every file/line cited was read this session, not recalled from training data
- Pitfalls: HIGH for Pitfalls 2-4 (directly observed in code/Taskfile); HIGH for Pitfall 1 as a **finding** (the location mismatch is directly observable) though the *recommended resolution* (Finding 1's suggested placement) is a judgment call, not itself independently verified against a style guide (none exists for this chart)

**Research date:** 2026-08-01
**Valid until:** 30 days (stable in-repo patterns, no third-party API surface to go stale)
