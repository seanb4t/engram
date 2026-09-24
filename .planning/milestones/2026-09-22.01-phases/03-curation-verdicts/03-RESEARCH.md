# Phase 3: Curation Verdicts - Research

**Researched:** 2026-09-23
**Domain:** Go CLI (cobra operator command) wiring an advisory typed-decision pass onto an
existing read-only sweep; text/JSON dual-rendering; config registry; gated eval harness.
**Confidence:** HIGH

## Summary

This phase adds an advisory relation-verdict pass to `engram spine-review consolidate`, on top
of two already-shipped Phase 2 primitives (`internal/decide`'s provider-neutral `Decider`
interface and the `jev` backend) and one already-shipped Phase-earlier primitive
(`internal/store`'s bounded-read mechanism, `boundedread.go`). Nothing about the decision
transport needs building — `decide.Choice`/`decide.Noul`, `Decider.DecideMany` (ordered
results, bounded concurrency, one-failure-never-fails-the-batch), and `decide.Status(err)`
(the named failure class D-10 asks for) already exist and are directly reusable. The real work
in this phase is three-fold: (1) a **new store-layer bounded read** to fetch per-pair state,
since `store.DuplicatePair` deliberately carries no content or summary today; (2) a **new
operator-tier construction seam** (`server.StoreAndDeciderFromEnv`-shaped, mirroring the
existing `StoreAndSummarizerFromEnv`) so `cmd/engram` — which cannot call unexported
`internal/server.deciderFromConfig` directly — can build a `Decider` from env the way every
other operator command builds its store; (3) a **text-rendering gap**: the operator view's
`viewRow` renderer has no path for a JSON-object-valued field inside an array row, and the
guard test that proves this (`TestOperatorViewFixturesHaveNoUnsanitizedNesting`) explicitly
says the day a report adds one, it must fail loudly — this phase's nested `verdict` object on
each candidate pair is exactly that day, and its comment (`WR-02`, `06-REVIEW.md`) names the
security property (`sanitizeViewValue`'s control-character stripping) that must not be skipped
when a fix lands.

CUR-04's scope guard is the smallest, most mechanical piece of this phase: `consolidate` is
currently the one live sweep-style command explicitly classified `nonEnforcingSweepLeaves` in
`cmd/engram/sweep_scope_test.go`, with its exemption reasoning *written into*
`internal/surfaces/rules.go`'s `RuleSweepScopeOrAllScopesRequired` doc comment, citing
`store/spine.go`'s "well-defined empty result" contract by line number. Fixing #508 means
flipping that classification, adding the one-line `requireSweepScope` call already used by
three sibling commands, and updating **both** the doc-comment prose and `docs-site/guides/cli.md`'s
own paragraph that currently asserts the opposite behavior in writing.

**Primary recommendation:** Build the verdict pass as three additive layers — a new
`internal/store` bounded batch-fetch for pair state, a new `server.StoreAndDeciderFromEnv`
wiring seam mirroring the summarizer precedent, and `cmd/engram`-local request-building /
verdict-mapping code, following `spine_review_consolidate.go`'s existing pure-headline /
JSON-doc-struct split — while treating the text-rendering nested-object gap and the
`sweep_scope_test.go` reclassification as first-class, load-bearing tasks, not incidental
plumbing.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Per-pair state fetch (summary + head-of-content, bounded) | Database/Storage (`internal/store`) | — | Must reuse the byte-budget bounded-read mechanism (`boundedread.go`); store already owns every other read view (full/summary/scan/citations/nearDuplicateIdentity) and must never be bypassed by an ad hoc `Get` loop |
| Decider construction from env | API/Backend (`internal/server`, exported wiring seam) | — | Mirrors `StoreAndSummarizerFromEnv`; `deciderFromConfig` is already correct and unexported — only a new exported wrapper is needed, not new decision logic |
| Decision transport (request shape, concurrency, error classes) | API/Backend (`internal/decide`, `internal/decide/jev`) | — | Fully shipped in Phase 2; this phase is a pure consumer |
| Verdict request-building / result-to-JSON mapping | CLI (`cmd/engram`) | — | `consolidatePairDoc`/`consolidateDoc` are `cmd/engram` types; the verdict shape is part of the operator JSON contract owned there, same as every other consolidate field |
| Text rendering of the nested verdict object | CLI (`cmd/engram/operator_view.go`) | — | `viewRow`'s scalar-only renderer is the one piece of shared CLI infrastructure this phase must extend; every other operator command's text view is unaffected |
| Scope-or-all-scopes guard | CLI (`cmd/engram/sweep_scope.go`) + registry (`internal/surfaces/rules.go`) | — | Existing shared helper and rule; this phase only reclassifies one command |
| Config registry knobs (threshold, truncation) | API/Backend (`internal/config`) | — | Same package/pattern as the 9 existing `ENGRAM_DECISIONS_*` keys |
| Eval harness (CUR-03) | CLI/test-only (`internal/<new package>`, gated `go test`) | — | Mirrors `internal/retrievaleval`'s package-local, unregistered koanf gate; never touches production config |

## User Constraints

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Two corpora, one harness: a **committed synthetic pair set (~60 pairs,
  spine-shaped, all 5 classes incl. `updates`)** run by a gated eval, plus the ability to point
  the same harness at a **local, gitignored real-spine pair file** for private runs that report
  aggregates only. No verbatim spine content is ever committed.
- **D-02:** Labeling uses **author-with-label + blind check**: one subagent authors pairs toward
  a target class; a second, blind subagent labels each pair without seeing the intended class;
  only pairs where both agree are kept. The procedure is documented in the fixture comment
  (mirrors Phase 1's D-01 independence discipline).
- **D-03:** **Report-first**: the eval reports accuracy and multi-class Brier score **by
  confidence bucket** (CUR-03). The only hard gate: verdicts at p ≥ the needs-review threshold
  must have **accuracy ≥ 0.9** on the committed set (validates the threshold).
- **D-04:** Verdicts run **by default when a decisions provider is configured**
  (`ENGRAM_DECISIONS_PROVIDER` set), with **`--no-verdicts`** to suppress. With no provider,
  consolidate is byte-identical to today (no flag needed, no outbound calls). — **Reversibility:**
  reversible — a CLI default, flippable later.
- **D-05:** JSON shape: each pair gains a **nested `verdict` object** —
  `verdict: {relation, probabilities{duplicate, contradicts, updates, related, unrelated},
  same_subject, needs_review, model}`; on failure `verdict: {error: "<named class>"}`. The
  object is **absent entirely** when verdicts did not run (additive-only contract for the
  operator JSON). **No separate top-`probability` field** — the chosen relation's probability
  is `probabilities[relation]` (single source of truth; the full map keeps close calls
  visible).
- **D-06:** Relation option set is **5 options: `duplicate` / `contradicts` / `updates` /
  `related` / `unrelated`** (criteria from the spike blueprint, plus `updates` = "B is a newer
  state or a more complete version of the same fact"), plus a `same_subject` Noul question —
  one Decisions request per pair (batch both questions on the pair's state).
- **D-07:** The text view renders the verdict, its probability, same-subject probability, and a
  `needs review` marker (rendered view of the JSON contract).
- **D-08:** Needs-review threshold: registered config key
  **`ENGRAM_DECISIONS_VERDICT_THRESHOLD`** (default 0.9) with a **`--verdict-threshold`** flag
  on consolidate overriding per run. A verdict whose top probability is below the threshold is
  `needs_review: true`.
- **D-09:** State per record = **summary (when present) + head of content**, N ≈ 1500 chars per
  side (configurable via a registered knob), keeping the deciding claim more often than blind
  truncation.
- **D-10:** A failed pair decision → `verdict: {error: "<named class>"}` (JSON) and
  `verdict unavailable (<class>)` (text); the report **completes with exit 0** plus a stderr
  summary line with counts; an all-pairs auth failure still exits 0 but warns loudly. Advisory
  results never fail the sweep (DEC-04 contract).
- **D-11:** **No cap** on pairs sent per run — every candidate pair gets a verdict (uses Phase
  2's `DecideMany` with `ENGRAM_DECISIONS_CONCURRENCY` bounding in-flight calls).
- **D-12:** Reuse the existing sweep-scope rule (`requireSweepScope` / `sweepScopeRule` /
  `surfaces.RuleSweepScopeOrAllScopesRequired`) so consolidate with neither `--scope` nor
  `--all-scopes` returns the same rule error the other sweep leaves return, and publish the
  rule sentence on consolidate's usage now that it is enforced (#508).

### Claude's Discretion

- Exact Go field names beyond the JSON keys above; the truncation knob name; how the eval
  harness selects the local pair file (env var or flag); synthetic pair domains.

### Deferred Ideas (OUT OF SCOPE)

- Per-run verdict cap (`--max-verdicts`) — considered, declined for now (D-11).
- The recall reranker (Phase 4), write-time hints (DEC-F1), changes to the curating-spine
  skill's consent contract.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CUR-01 | Operator running `consolidate` with decisions enabled sees, per candidate pair, a relation verdict, its probability, and a same-subject probability in `--output json` and text | `decide.Choice`/`decide.Noul` request shape, `Decider.DecideMany`, and the text-rendering nested-object gap (`viewRow`/`TestOperatorViewFixturesHaveNoUnsanitizedNesting`) are all documented below with exact call sites |
| CUR-02 | Verdicts below a configurable confidence threshold (default 0.9) are marked needs-review; consolidate never mutates a record | Threshold config pattern (mirrors 9 existing `ENGRAM_DECISIONS_*` registry rows); `NearDuplicates`/`consolidateDoc` already issue zero write RPCs — the verdict pass must be additive-only over the same read-only contract |
| CUR-03 | Relation question set measured on a labeled pair eval, no verbatim spine content committed, reporting accuracy by confidence bucket and Brier score | `internal/retrievaleval`'s gated, unregistered-koanf test harness precedent (`gate.go`, `doc.go`) and `internal/decide/jev/live_test.go`'s live-decider construction precedent are both documented below |
| CUR-04 | `consolidate` with neither `--scope` nor `--all-scopes` gets the scope-or-all-scopes rule error (#508) | `requireSweepScope`/`sweepScopeRule`, `sweep_scope_test.go`'s classification maps, and the `RuleSweepScopeOrAllScopesRequired` doc comment naming consolidate's current exemption are all documented below with the exact edits needed |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- `spine-review` is a Subject-less operator-tier command family; this phase adds behavior to an
  existing leaf, never a new authz path, never composed from `Search`/`List`.
- Conventional Commits required; PR titles CI-validated. `main` protected — branch + PR only.
- `task` = lint (`golangci-lint`, `yamlfmt`, `actionlint`, `rumdl`) + test (`go test ./...`).
  Both must be clean.
- Apache-2.0 SPDX header on every in-scope `.go`/`.md` file (scope owned by `.licenserc.yaml` —
  never hand-edited). `.planning/**` files with YAML frontmatter are excluded — do not add a
  header above frontmatter.
- Payload migrations are schema-version-driven via `internal/migrate`; this phase adds no new
  stored payload key to `store.Memory` (verdicts are never stored — advisory only, per the
  Out-of-Scope table in REQUIREMENTS.md), so no migration is implicated.
- Memory contract: `search_memory`/`list_memory`/etc. are unaffected — this phase touches only
  the operator-tier `spine-review consolidate` CLI surface, not the MCP/Connect tool contracts.
- `internal/store` must **never** import `internal/embed` or `internal/server` — a documented,
  enforced architectural boundary (`internal/store/store.go:1321-1324`, verbatim: *"SearchReranked
  takes plain inputs (query text, query vector, k) and does NOT import internal/embed or
  internal/server (round-2 finding 7)"*). By the same boundary, `internal/store` must never
  import `internal/decide` — confirmed no such import exists today
  `[VERIFIED: internal/store/*.go, internal/decide/*.go, internal/decide/jev/*.go — grep for
  cross-imports returned zero hits this session]`.

## Standard Stack

No new external dependency is needed for this phase. Every transport primitive (HTTP client,
error classification, concurrency-bounded batch execution) already ships from Phase 2's
`internal/decide` / `internal/decide/jev`. The eval harness needs only Go's standard library
plus the already-vendored `koanf` packages already used by `internal/retrievaleval`'s gate.

### Core (already shipped, reused as-is)

| Package | Version | Purpose | Why Standard (this repo) |
|---------|---------|---------|---------------------------|
| `internal/decide` | in-tree (Phase 2) | `Decider` interface, `Request`/`Response`/`Answer`, `DecideMany`, named error classes | The provider-neutral contract this whole phase is a consumer of — `[VERIFIED: internal/decide/decide.go, internal/decide/many.go, internal/decide/errors.go — read this session]` |
| `internal/decide/jev` | in-tree (Phase 2) | Jev backend over OpenRouter's Decisions API | Constructed via `server.deciderFromConfig`; hand-written on `net/http`, SDK evaluated and rejected (`02-SDK-EVALUATION.md`) — `[VERIFIED: internal/decide/jev/jev.go:10-13 — read this session]` |
| `internal/store` (boundedread.go, spine.go) | in-tree | Bounded-read mechanism, `NearDuplicates`/`DuplicatePair` | The one place per-record read ceilings are derived; a new view must follow this file's pattern — `[VERIFIED: internal/store/boundedread.go — read this session]` |
| `github.com/knadh/koanf/v2` + `providers/confmap`, `providers/env/v2` | already in `go.mod` (used by `internal/retrievaleval/gate.go`, `internal/decide/jev/live_test.go`) | Test-local eval gate, independent of the production registry | `[VERIFIED: internal/retrievaleval/gate.go, internal/decide/jev/live_test.go — read this session]` |

### Supporting (new, small, in-tree additions this phase writes)

| Addition | Where | Purpose | When to Use |
|----------|-------|---------|-------------|
| A new bounded `readView` (e.g. `verdictStateView`) + per-record ceiling constant | `internal/store/boundedread.go` | Include `content`, `summary`, `scope`, `short_id`; exclude `tags`, `citations` — sized like `summaryRecordCeiling`/`fullRecordCeiling` but for exactly the fields D-09 needs | Fetching per-pair state for the verdict pass |
| A batched by-id fetch method (e.g. `Store.RecordStates(ctx, ids []string) (map[string]RecordState, error)`) | `internal/store` (new or `spine.go`) | `client.Get` already accepts multiple `*qdrant.PointId` in one `GetPoints` RPC — chunk the id list to `perRPCLimit(view.maxRecordBytes)` the same way `scrollAllPoints`/`NearDuplicates` chunk theirs | Resolving both sides of every candidate pair's state in as few RPCs as possible, deduplicating ids that appear in more than one pair |
| `server.StoreAndDeciderFromEnv` (or equivalently named) | `internal/server` (new exported function, likely `decider.go` or `tools.go`) | Wraps `loadAndValidate` + `ensureStoreFromConfig` + `deciderFromConfig(cfg)` in one config load, mirroring `StoreAndSummarizerFromEnv` exactly | `cmd/engram/spine_review_consolidate.go`'s `RunE` — `deciderFromConfig` is unexported and cannot be called from `package main` today `[VERIFIED: internal/server/decider.go:33 — func deciderFromConfig(cfg *config.Config) (decide.Decider, error) — lower-case, unexported]` |
| `internal/config.ParseProbability` (or similarly named) | `internal/config/validate.go` | Parses `ENGRAM_DECISIONS_VERDICT_THRESHOLD`/`--verdict-threshold` as a float in `[0, 1]`; no existing parser does this — `ParsePositiveIntCap`/`ParseNonNegativeIntCap` are both `int`-only `[VERIFIED: internal/config/validate.go:413-446 — read this session, both parsers use strconv.Atoi]` | Validating the new threshold config/flag value |
| A new package-local eval gate (name TBD, e.g. `ENGRAM_CURATION_EVAL`) | new `internal/<pkg>/gate.go` | Same shape as `internal/retrievaleval/gate.go` — default-false koanf layer, never registered in `internal/config` | Gating CUR-03's eval package so `go test ./...` pays zero live-Decisions cost by default |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| A new `internal/store` batch-fetch method | Loop calling `Store.Get` once per id | `Get` is a single-id `client.Get` RPC with no byte-budget sizing at all — looping it for every pair (up to 2× pair count) skips the bounded-read mechanism this milestone exists to enforce everywhere else; `client.Get` already supports a multi-id `Ids` slice in one RPC, so batching is strictly better and matches the existing pattern |
| `server.StoreAndDeciderFromEnv` wrapper | Export `deciderFromConfig` directly and call `server.LoadAndValidate` + `server.EnsureStoreFromConfig` separately from `cmd/engram` | Every existing operator command uses exactly ONE combined `StoreAndXFromEnv`-shaped call (`StoreAndSummarizerFromEnv`, `StoreAndEmbedderFromEnvNoEnsure`, `StoreFromEnv`) so config loads exactly once; splitting the calls in `cmd/engram` would be the first command to diverge from that convention with no benefit |
| A per-verdict-field text renderer in `viewRow` | Serialize the nested `verdict` object as a second top-level JSON field on `consolidateReportDoc` (a parallel `verdicts` array, not nested inside each candidate) | D-05 explicitly locks the nested-object shape (`each pair gains a nested verdict object`) — flattening at the JSON layer is not available; the fix must live in the text-rendering layer, not the JSON contract |

**Installation:** none — no new `go.mod` entries.

**Version verification:** N/A — no new packages to verify against a registry this phase.

## Package Legitimacy Audit

No external packages are added by this phase. Every dependency used (`internal/decide`,
`internal/decide/jev`, `github.com/knadh/koanf/*`) is already present in `go.mod` and was
verified/adopted in prior phases (`02-SDK-EVALUATION.md` for the Jev client; `koanf` is already
a direct dependency used by `internal/config` and `internal/retrievaleval`). The Package
Legitimacy Gate protocol is not applicable — skipped with reason: no new packages installed.

**Packages removed due to [SLOP] verdict:** none.
**Packages flagged as suspicious [SUS]:** none.

## Architecture Patterns

### System Architecture Diagram

```
operator invokes:
  engram spine-review consolidate --scope <s> [--verdict-threshold P] [--no-verdicts]
        │
        ▼
  requireSweepScope(scope, allScopes)  ──(CUR-04, reused unchanged)──▶ exitUsage on neither flag
        │ ok
        ▼
  server.StoreAndDeciderFromEnv()  (NEW, mirrors StoreAndSummarizerFromEnv)
        │  ├─ loadAndValidate()            (ONE config load)
        │  ├─ ensureStoreFromConfig(cfg)    → *store.Store
        │  └─ deciderFromConfig(cfg)        → decide.Decider | nil  (D-04: nil ⇒ no verdicts)
        ▼
  st.NearDuplicates(ctx, opts)  (UNCHANGED — read-only, no content/summary in DuplicatePair)
        │
        ▼  pairs []store.DuplicatePair
  if decider != nil && !noVerdicts:
        │
        ▼
  st.RecordStates(ctx, dedupedIDs)  (NEW bounded batch fetch — internal/store)
        │  summary + head-of-content(N≈1500 chars) per id, budgeted readView
        ▼  map[id]RecordState
  build one decide.Request per pair (state = {record_a, record_b}; questions = relation, same_subject)
        │
        ▼
  decider.DecideMany(ctx, reqs)   (Phase 2, ordered results, ENGRAM_DECISIONS_CONCURRENCY-bounded)
        │
        ▼  []decide.Result  (index-aligned with pairs)
  map each Result → verdictDoc{relation, probabilities, same_subject, needs_review, model} | {error: decide.Status(err)}
        │  (D-10: a failed pair NEVER fails the sweep — exit 0, stderr summary line)
        ▼
  consolidatePairDoc.Verdict *verdictDoc  (nil/omitted when verdicts did not run — D-05 additive contract)
        │
        ▼
  renderOperator(cmd, format, headline, consolidateDoc)
        │  json: encoding/json over the doc struct (unchanged mechanism)
        │  text: viewFields → viewRow  (MUST be extended — see Pitfall 1 below — to render
        │        a nested object field without bypassing sanitizeViewValue)
        ▼
  operator sees ranked candidate pairs, each optionally carrying an advisory verdict —
  NEVER a mutation (NearDuplicates and every doc-building function issue zero write RPCs)
```

### Recommended Project Structure

No new top-level package is strictly required for CUR-01/02/04 — the additions fit the existing
file layout:

```
internal/store/
├── boundedread.go     # + verdictStateView (or similarly named) + its ceiling const
├── spine.go           # + RecordStates (or similarly named) batched-by-id fetch
internal/config/
├── config.go          # + DecisionsConfig.VerdictThreshold, .VerdictTruncateChars (string fields)
├── registry.go         # + 2 new rows under the existing decisions.* block
├── validate.go         # + ParseProbability (or similarly named) + Config.Validate wiring
internal/server/
├── decider.go          # + StoreAndDeciderFromEnv (exported wrapper), + verdictThreshold/
│                          verdictTruncateChars resolver helpers mirroring decisionsTimeout etc.
cmd/engram/
├── spine_review_consolidate.go   # + --verdict-threshold, --no-verdicts flags; RunE wiring;
│                                    consolidatePairDoc.Verdict field; verdict request/mapping
├── operator_view.go              # + nested-object rendering path for the verdict field
├── sweep_scope.go / sweep_scope_test.go  # move "spine-review consolidate" enforcing↔non-enforcing
internal/surfaces/
├── rules.go            # update RuleSweepScopeOrAllScopesRequired's doc comment (consolidate
│                          is no longer exempt; the "no field set can select exactly the three
│                          enforcing leaves" narrowing logic must be re-derived for FOUR leaves)
docs-site/src/content/docs/guides/
├── cli.md               # rewrite consolidate's "supplying neither sweeps ... never an
│                           accidental whole-spine sweep" paragraph (now wrong) and the
│                           "never labels a pair a duplicate" paragraph (needs an advisory
│                           carve-out)
├── configure.md          # + 2 new ENGRAM_DECISIONS_* rows in the Typed decisions table;
│                           update the "features that actually ask it questions ... will name
│                           this setting when they do" sentence to name consolidate
```

CUR-03's eval harness (new package, e.g. `internal/curationeval` or similar — naming is
Claude's discretion) mirrors `internal/retrievaleval`'s file split:

```
internal/curationeval/       # name is discretionary — pick something that reads naturally
                              # against internal/retrievaleval's own naming
├── doc.go                   # package doc explaining the gate, mirrors retrievaleval/doc.go
├── gate.go                  # package-local koanf gate, NEVER registered (D-15 precedent)
├── fixtures.go              # ~60 committed synthetic pairs, all 5 classes, blind-authoring
│                             # procedure documented in the file comment (D-02)
├── localfile.go             # loads a gitignored real-spine pair file when pointed at one (D-01)
├── metrics.go                # accuracy-by-confidence-bucket + multi-class Brier score (new —
│                             # no existing Brier-score code in this repo)
├── eval_test.go              # the gated go test entry point; task eval:curation Taskfile target
```

### Pattern 1: Store-owned bounded batch fetch (mirrors `NearDuplicates`' own two-primitive design)

**What:** A new `readView` (Company: `internal/store/boundedread.go`) plus a new `Store` method
that fetches multiple records' state (summary + content, for later client-side truncation) in
as few Qdrant RPCs as possible, chunked to the view's byte-derived per-RPC limit.

**When to use:** Any time a CLI/server caller needs record content or summary for MORE THAN ONE
id at once, outside of `scrollAllPoints`' whole-collection sweep. This is genuinely new: no
existing `Store` method does a bounded, by-id, multi-record fetch — `Get` is single-id and
unbudgeted (it fetches full payload with no `readView` at all), and every sweep goes through
`scrollAllPoints` (whole-collection enumeration), not a specific id list.

**Example (structure to follow, based on verified precedent — not verbatim shippable code):**

```go
// internal/store/boundedread.go — new view, mirroring summaryView's shape:
// verdictStateRecordCeiling accounts for content + summary + short_id + scope
// (no tags, no citations) — see fullRecordCeiling/summaryRecordCeiling for the
// derivation pattern (D-04, boundedread.go).
func verdictStateRecordCeiling(c RecordCaps) int {
	return c.ContentBytes + summaryTerm(c) + uncappedFieldsAllowance
}

func (s *Store) verdictStateView() readView {
	return readView{
		selector:       qdrant.NewWithPayloadInclude("content", "summary", "short_id", "scope"),
		maxRecordBytes: verdictStateRecordCeiling(s.RecordCaps()),
	}
}

// internal/store/spine.go — new batched-by-id fetch, chunked like NearDuplicates'
// own chunkIDs(ids, nearDuplicateBatchSize) pattern, but via client.Get's
// multi-id Ids slice instead of QueryBatch:
func (s *Store) RecordStates(ctx context.Context, ids []string) (map[string]RecordState, error) {
	view := s.verdictStateView()
	limit := perRPCLimit(view.maxRecordBytes) // reuse boundedread.go's existing helper
	out := make(map[string]RecordState, len(ids))
	for _, chunk := range chunkIDs(ids, limit) { // reuse spine.go's existing chunkIDs
		pids := make([]*qdrant.PointId, len(chunk))
		for i, id := range chunk {
			pids[i] = qdrant.NewID(id)
		}
		pts, err := s.client.Get(ctx, &qdrant.GetPoints{
			CollectionName: s.collection, Ids: pids, WithPayload: view.selector,
		})
		if err != nil {
			return nil, err
		}
		for _, p := range pts {
			id := p.Id.GetUuid()
			out[id] = RecordState{
				Summary: p.Payload["summary"].GetStringValue(),
				Content: p.Payload["content"].GetStringValue(),
			}
		}
	}
	return out, nil
}
```
*(Sketch based on verified `chunkIDs`, `perRPCLimit`, `readView`, and `client.Get`'s existing
call shape at `internal/store/store.go:2018-2020`; the exact struct/method names are Claude's
discretion per CONTEXT.md.)*

### Pattern 2: `StoreAndXFromEnv` wiring seam (exact precedent to copy)

**What:** `internal/server` exports exactly one combined constructor per operator command that
needs more than a bare store — `StoreAndSummarizerFromEnv` is the direct precedent for this
phase's `StoreAndDeciderFromEnv`.

**When to use:** `cmd/engram/spine_review_consolidate.go`'s `RunE`, replacing the current
`spineConsolidateStoreFromEnv` package var (which only builds a store) with one that also
resolves the decider — **or** add a second package var for the decider alongside it. Either way,
`deciderFromConfig`'s existing `(nil, nil)` contract on empty `Provider` (D-04, unconditionally
verified this session) must be preserved: `StoreAndDeciderFromEnv` must succeed with a `nil`
Decider when no provider is configured, never error.

**Example:**
```go
// Source: internal/server/tools.go:698-711 (StoreAndSummarizerFromEnv) — VERIFIED this
// session; the phase's new function should follow this exact shape, substituting
// deciderFromConfig for summarizerFromConfig, and NEVER erroring on cfg.Decisions.Provider == "".
func StoreAndSummarizerFromEnv() (*store.Store, *summarize.Client, string, int, error) {
	cfg, err := loadAndValidate()
	if err != nil {
		return nil, nil, "", 0, err
	}
	if cfg.Summarize.Model == "" {
		return nil, nil, "", 0, fmt.Errorf("ENGRAM_SUMMARY_MODEL is empty: auto-summary is disabled")
	}
	st, err := ensureStoreFromConfig(cfg)
	if err != nil {
		return nil, nil, "", 0, err
	}
	return st, summarizerFromConfig(cfg), cfg.Summarize.Model, summaryMaxChars(cfg), nil
}
```
The new function must NOT copy the `cfg.Summarize.Model == ""` early-error branch — that is
`summarize-missing`'s own "feature is required, error if unconfigured" contract, which
contradicts D-04's "no provider ⇒ nil decider, no error" requirement for consolidate.

### Pattern 3: `decide.Status(err)` for D-10's named failure class

**What:** Phase 2 already built the exact function D-10 asks for: a stable, lowercase, named
class string derived from any decision error via `errors.Is` matching, never message-text
parsing.

**Example:**
```go
// Source: internal/decide/errors.go:150-177 — VERIFIED this session.
// Status returns the fixed, lowercase word ... "auth"/"bad_request"/"context_too_large"/
// "rate_limited"/"unavailable"/"timeout"/"response_too_large"/"malformed_response"/
// "invalid_request"/"canceled"/"error".
func Status(err error) string { /* ... */ }
```
Use `decide.Status(result.Err)` directly as the value of `verdict.error` in D-10's failure
shape — no new error-classification code is needed for this requirement.

### Anti-Patterns to Avoid

- **Fetching per-pair state with a `Get`-per-id loop:** bypasses the bounded-read mechanism
  entirely (`Get` has no `readView`, no byte ceiling) — exactly the failure mode
  `boundedread.go`'s whole design exists to prevent. Use a new batched view instead (Pattern 1).
- **Calling `internal/server.deciderFromConfig` from `cmd/engram` via an exported alias without
  a combined config load:** every existing operator command loads config exactly once
  (`loadAndValidate`, called from inside a single `StoreAndXFromEnv`); a second, independent
  `config.Load` call in `cmd/engram` would diverge from that convention for no benefit and risks
  the two loads disagreeing under a future config-reload feature.
- **Serializing the verdict as a flat set of `verdict_relation`/`verdict_probability_*` fields
  on `consolidatePairDoc` instead of a nested object:** contradicts D-05's locked JSON shape.
  The nested-object rendering problem (Pitfall 1) must be solved in the text-rendering layer,
  not worked around by flattening the JSON contract.
- **Having `internal/store` import `internal/decide`:** even though `RecordState` conceptually
  feeds a `decide.Request`, the store package must stay decide-agnostic — it already avoids
  importing `internal/embed`/`internal/server` for the identical reason
  (`internal/store/store.go:1321-1324`). Build `decide.Request` values in `cmd/engram` (or a new
  small package under `internal/decide` that `cmd/engram` imports alongside `internal/store`),
  never inside `internal/store`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Typed decision transport (HTTP, retries, error classification) | A second HTTP client for Jev | `internal/decide/jev.Client` via `deciderFromConfig`/`StoreAndDeciderFromEnv` | Already built, tested, and DEC-05-evaluated against the OpenRouter SDK (rejected) in Phase 2 |
| Bounded concurrency over N pair requests | A hand-rolled worker pool / semaphore | `decide.Decider.DecideMany` (wraps `decide.DecideMany`, `ENGRAM_DECISIONS_CONCURRENCY`-bounded, ordered results, panic-safe) | Exactly D-11's requirement — no cap on pair count, bounded in-flight concurrency — already implemented and tested (`internal/decide/many_test.go`) |
| Failure-class naming for a decision error | String-matching on `err.Error()` | `decide.Status(err)` | Already the named, stable, `errors.Is`-driven class D-10 asks for |
| Scope-or-all-scopes CLI validation | A second `usageErrorf` string literal duplicating `sweep_scope.go`'s wording | `requireSweepScope` + `sweepScopeRule()` | CUR-04 explicitly requires bit-identical behavior to the three sibling sweep leaves; a second hand-rolled check would drift from the registered `Sentence` and fail `TestSweepLeavesRejectMissingScopeIdentically`-style tests |
| Per-record byte ceiling arithmetic | A new ad hoc size constant | `internal/store/boundedread.go`'s `readView`/`perRPCLimit`/`RecordCaps` machinery | This milestone's whole design point — a record whose actual bytes exceed a view's ceiling is caught by `scrollAllPoints`'/`client.Get`'s own proto.Size fallback, never silently truncated |

**Key insight:** every piece of "hard" infrastructure this phase would otherwise need
(HTTP transport, retry/backoff, error classification, concurrency bounding, byte-budgeted reads)
already exists in this codebase from Phase 2 (decisions) or the 2026-09-18.01 milestone
(bounded reads). The actual net-new code is small and glue-shaped: one store view + one fetch
method, one config wiring seam, one CLI flag/RunE wiring, one text-renderer extension, and one
scope-guard reclassification.

## Common Pitfalls

### Pitfall 1: The nested `verdict` object will trip an existing, deliberately-loaded landmine test

**What goes wrong:** `--output text` for consolidate renders each candidate pair through
`viewRow` (`cmd/engram/operator_view.go`), which walks a JSON object's keys and calls
`viewScalar(val)` for each one. `viewScalar`'s kind switch recognizes only a JSON string
(sanitized via `sanitizeViewValue`) or `null`; every other shape — including a nested JSON
object — falls through to `return string(raw)`, rendered **verbatim and UNSANITIZED**. D-05's
verdict shape is a nested object (`verdict: {relation, probabilities{...}, same_subject,
needs_review, model}`) inside each `consolidatePairDoc` array element, which is precisely the
shape this code path cannot safely render.

This is not a hypothetical: a guard test already exists and is designed to fail the day this
happens. Quoting it verbatim
`[VERIFIED: cmd/engram/operator_output_test.go:383-402, read this session]`:

> `// TestOperatorViewFixturesHaveNoUnsanitizedNesting is the WR-02`
> `// (06-REVIEW.md) regression guard: ... No operator report struct produces such a shape`
> `// today ..., so this test passes today and is designed to fail LOUDLY — not silently`
> `// reintroduce the T-06-03 gap — the day a future report field crosses that boundary.`

And the underlying helper `[VERIFIED: cmd/engram/operator_output_test.go:458-472, read this
session]`:
> `// assertRowHasNoContainerFields fails if any key inside a rendered row ... is itself a JSON`
> `// array or object`
> `t.Errorf("%s: field %q row key %q is a nested %c — sanitizeViewValue's guarantee does not`
> `reach this shape (WR-02, 06-REVIEW.md); either give viewScalar an exhaustive kind switch or`
> `keep row fields scalar", ...)`

**Why it happens:** the operator-view renderer was deliberately kept flat because, until this
phase, no operator report struct needed a nested object inside an array row.

**How to avoid:** treat this as a required, first-class task, not a side effect. Two viable
approaches (pick one; this is implementation-detail, not locked by CONTEXT.md):
1. Extend `viewScalar`/`viewRow` with an explicit, sanitizing rendering path for a one-level-deep
   nested object (e.g. render `verdict.relation=duplicate verdict.same_subject=0.91
   verdict.needs_review=false` as additional `key=value` tokens on the same row line, running
   every string value through `sanitizeViewValue`) — this is the "give viewScalar an exhaustive
   kind switch" option the test comment names.
2. Add a **row-level custom renderer** used only for consolidate's candidate rows (bypassing
   `viewRow`'s generic walk for this one field), still routing every string through
   `sanitizeViewValue`.
Either way, `TestOperatorViewFixturesHaveNoUnsanitizedNesting`'s fixture set and/or its
"no shape does this today" framing must be updated deliberately (a fixture with a populated
`verdict` field must be added to `spineViewFixtures()`'s consolidate case, and the test's own
doc comment should be updated to say WHY this shape is now allowed and HOW it stays sanitized),
not silently bypassed by, e.g., excluding the new fixture from the walked set.

**Warning signs:** `go test ./cmd/engram/...` fails on `TestOperatorViewFixturesHaveNoUnsanitizedNesting`
the moment a fixture with a non-empty `Verdict` field is added to the consolidate case in
`spineViewFixtures()`. Treat that failure as confirmation the design is engaging correctly, not
as a bug to work around by omitting the fixture.

### Pitfall 2: `store.DuplicatePair` carries no content or summary — a naive verdict pass will not compile against it

**What goes wrong:** it is tempting to assume `NearDuplicates`' existing pairs already carry
enough to build a Decisions request. They deliberately do not.
`[VERIFIED: internal/store/spine.go:472-485, read this session]`:

> `// DuplicatePair is one ranked near-duplicate candidate: two record`
> `// identities, their short ids and scopes, and Score ... Content and summary`
> `// are deliberately absent from this struct — a report row can never leak`
> `// stored substance (T-03-28's mitigation).`

**Why it happens:** `NearDuplicates` was built (milestone 2026-09-18.01) specifically so the
structural ranking sweep never fetches content — its own doc comment says the per-id query
"fetches nothing beyond id and score... record content is never fetched anywhere in this
method" (`internal/store/spine.go:556-560`).

**How to avoid:** build a SEPARATE, NEW fetch (Pattern 1 above) for the verdict pass only,
gated behind "a decider is configured and `--no-verdicts` was not passed" — so the structural
ranking path (`NearDuplicates` itself, and every other consolidate invocation with no decisions
provider configured) stays byte-identical and still never fetches content, satisfying D-04.

### Pitfall 3: `TestDecisionsVarsDocumented` hardcodes a count of 9 — adding 2 new `ENGRAM_DECISIONS_*` vars WILL break it until updated

**What goes wrong:** the docs-completeness gate for the Decisions config block asserts an exact
count. `[VERIFIED: internal/config/decisions_docs_test.go:74-81, read this session]`:

> `envs := decisionsRegistryEnvNames()`
> `if len(envs) != 9 {`
> `    t.Fatalf("decisionsRegistryEnvNames() returned %d names, want 9 (positive control -- an`
> `    empty or short derivation must not pass vacuously): %v", len(envs), envs)`
> `}`

Adding `ENGRAM_DECISIONS_VERDICT_THRESHOLD` and a truncation-chars knob raises this to 11.

**Why it happens:** the test's own "positive control" is a hardcoded literal, by design (so an
empty derivation can't pass vacuously) — but that means every registry addition to the
`decisions.*` block must update it in the same change.

**How to avoid:** when adding the two new registry rows, in the SAME commit: (1) bump the `!= 9`
literal to `!= 11`; (2) add the two new `| \`ENGRAM_DECISIONS_VERDICT_THRESHOLD\` | ... |` rows
to the "red control" synthetic table inside the same test file (the deliberately-incomplete
fixture used to prove the gate itself can fail); (3) add matching rows to
`docs-site/src/content/docs/guides/configure.md`'s `## Typed decisions (Jev)` table.

### Pitfall 4: `RuleSweepScopeOrAllScopesRequired`'s doc comment names consolidate's CURRENT exemption by line number — it will be stale, not just incomplete, once CUR-04 ships

**What goes wrong:** the rule's registration comment doesn't just list which commands are
exempt — it explains WHY, citing the exact store-layer behavior that will change.
`[VERIFIED: internal/surfaces/rules.go:178-198, read this session]`:

> `// SurfaceFields diverges from Fields to`
> `// []string{"scope", "all-scopes", "dry-run"}. Five commands' own flag sets`
> `// expose BOTH scope and all-scopes: spine-review scan, spine-review verify,`
> `// summarize-missing (all three enforce this rule today), plus`
> `// spine-review consolidate and spine-review purge (neither enforces it --`
> `// consolidate's NearDuplicates treats Scope:"" AllScopes:false as a`
> `// well-defined empty result, internal/store/spine.go:384-387; purge applies`
> `// a scope filter only when !AllScopes && Scope != "", internal/store/`
> `// spine.go:991, so a class-only purge naming neither flag deliberately`
> `// spans every scope, D-10). No field set can select exactly the three`
> `// enforcing leaves by Fields alone...`

Once consolidate enforces the rule, this becomes FOUR enforcing leaves, not three, and the
"no field set can select exactly the three enforcing leaves... {scope, all-scopes, output,
timeout} — summarize-missing's *entire* set minus dry-run/older-than/limit — which is a strict
subset of both consolidate's and purge's flag sets" reasoning needs re-verification: does
consolidate's flag set (now enforcing) still get correctly excluded from — or correctly
included in — the derived `SurfaceFields` narrowing for docs-prose purposes? The `dry-run`
narrowing was chosen specifically because consolidate did NOT enforce the rule; that constraint
is gone.

**Why it happens:** the comment intentionally over-documents its own reasoning (per this
project's `4aksmneehh`/correct-by-reading convention) — which means it is also the first thing
that goes stale.

**How to avoid:** in the same change that reclassifies consolidate: (1) move
`"spine-review consolidate": true` from `nonEnforcingSweepLeaves` to `enforcingSweepLeaves` in
`cmd/engram/sweep_scope_test.go`; (2) call `requireSweepScope(spineConsolidateScope,
spineConsolidateAllScopes)` as the first statement of consolidate's `RunE`, mirroring
`spine_review_scan.go`/`summarize.go`/`spine_review_verify.go` exactly; (3) add
`sweepScopeRule().Sentence` to consolidate's `--all-scopes` flag `Usage` string (currently it
does not contain it — confirmed via `TestSweepLeavesUsageStatesRegisteredRule`'s existing
`nonEnforcingSweepLeaves[key] && contains` failure branch, which would fire in reverse once
reclassified); (4) rewrite `RuleSweepScopeOrAllScopesRequired`'s doc comment to reflect
consolidate's new status and re-verify (empirically, against the live tree, the same way
`08-01-PLAN.md fact 5` did) whether the `SurfaceFields` narrowing still resolves correctly with
four enforcing leaves instead of three; (5) purge remains the sole exemption — its own
reasoning (`internal/store/spine.go:991`) is untouched by this phase.

### Pitfall 5: The docs-site CLI guide currently asserts, in writing, the exact behavior #508 is fixing

**What goes wrong:** `docs-site/src/content/docs/guides/cli.md`'s consolidate section states
`[VERIFIED: docs-site/src/content/docs/guides/cli.md:246-260, read this session]`:

> `` `--scope` and `--all-scopes` are ``
> `mutually exclusive; supplying neither sweeps a well-defined empty result`
> `(no record has a literally-empty scope), never an accidental whole-spine`
> `sweep.`
>
> `This command **never merges, never mutates, and never labels a pair a`
> `"duplicate."**`

Both sentences need editing: the first is now factually wrong (neither flag ⇒ a usage error, not
an empty result) — this is the "docs-site reference and surfaces registry entries that must
change" the phase description asked to be established. The second needs a qualifying clause: the
STRUCTURAL sweep still never labels a pair a duplicate on its own, but an advisory `verdict`
object CAN now report `relation: "duplicate"` — the invariant that must survive is "never
merges, never mutates," not "never says the word duplicate anywhere in the output."

**How to avoid:** rewrite both paragraphs in the same change that ships CUR-01/CUR-04, and add
(or extend) a docs-completeness gate analogous to `TestDecisionsVarsDocumented` if one does not
already cover this file for consolidate-specific prose (none was found this session — this may
be a new, planner-authored gate, consistent with the project's `4aksmneehh` correct-by-reading
convention rather than a prose-only fix).

### Pitfall 6: This phase's own `verdict` vocabulary collides with the `curating-spine` skill's pre-existing, DIFFERENT "verdict" vocabulary

**What goes wrong:** the `curating-spine` skill (which this phase's CONTEXT.md explicitly says
must NOT change this phase) already has its own three-way "Identity verdicts" the AGENT assigns
by reading two records manually: `same-fact` / `overlapping` / `distinct`
`[VERIFIED: skill/engram/skills/curating-spine/SKILL.md:84-92, read this session]`. This phase
introduces an ENGRAM-EMITTED JSON field literally named `verdict` with a DIFFERENT five-value
vocabulary (`duplicate`/`contradicts`/`updates`/`related`/`unrelated`). The skill file also
states plainly, about `consolidate`'s current output: `` `score` is raw cosine similarity,
reported as-is — never bucketed, never a verdict `` (line 73) — that sentence becomes stale
prose (not wrong, but incomplete) the moment consolidate's JSON gains an actual `verdict` key,
even though the skill's own consent/judgment workflow is explicitly out of scope for this
phase.

**Why it happens:** two independent "verdict" concepts — one agent-assigned (skill), one
Jev-assigned (this phase) — now coexist in the same command's surface area.

**How to avoid:** this phase should NOT edit the skill's judgment workflow (CONTEXT.md is
explicit), but SHOULD avoid making the collision worse — e.g., do not name any new Go
identifier or JSON key in a way that could be mistaken for the skill's three-value vocabulary,
and flag the stale "never bucketed, never a verdict" sentence as a known follow-up (out of
phase scope, but worth a one-line note in the phase's own SUMMARY/handoff so a future
skill-update phase does not rediscover this from scratch).

## Code Examples

### Reusing `decide.Choice`/`decide.Noul` for D-06's request shape

```go
// Source: internal/decide/decide.go:72-101 — VERIFIED this session.
func Noul(instructions, whenTrue, whenFalse string) Question { /* ... */ }
func Choice(instructions string, options map[string]string) Question { /* ... */ }

// D-06's shape, built directly from these constructors (criteria text from
// the spike blueprint, .claude/skills/spike-findings-engram/references/
// curation-verdicts.md:16-21, plus the "updates" option D-06 adds):
req := decide.Request{
	State: decide.State{"record_a": stateA, "record_b": stateB},
	Questions: map[string]decide.Question{
		"relation": decide.Choice("How does record_b relate to record_a?", map[string]string{
			"duplicate":   "Both state the same fact; keeping both is redundant (one may be more complete).",
			"contradicts": "They make incompatible claims about the same subject; one corrects or reverses the other.",
			"updates":     "B is a newer state or a more complete version of the same fact.",
			"related":     "Same subject area, but different and compatible facts; both are worth keeping.",
			"unrelated":   "Different subjects.",
		}),
		"same_subject": decide.Noul("Are both records about the same specific subject?",
			"both records are about the same specific subject",
			"the records are about different subjects"),
	},
}
```

### Deriving `needs_review` from the configured threshold (D-08)

```go
// probabilities[relation] is the D-05-mandated single source of truth for
// the chosen relation's probability (no separate top-level probability
// field). Compare with a tolerance — never exact equality — per the spike
// blueprint's own measured run-to-run variance.
// Source: .claude/skills/spike-findings-engram/references/decision-transport.md:74
// ("Do not use exact-equality thresholds: identical requests vary ±0.03 on a
// 0.9 probability.")
answer := result.Response.Answers["relation"]
p := answer.Probabilities[answer.Choice]
needsReview := p < threshold // threshold from ENGRAM_DECISIONS_VERDICT_THRESHOLD / --verdict-threshold
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `consolidate` with neither `--scope` nor `--all-scopes` silently reports zero candidates | Rejects with the registered `RuleSweepScopeOrAllScopesRequired` sentence, `exitUsage` | This phase (CUR-04, #508) | Matches the other three sweep leaves; docs and rule-registry comments must be updated in lockstep (Pitfalls 4, 5) |
| `consolidate`'s JSON/text output never mentions "duplicate" anywhere | An advisory `verdict.relation` field CAN legitimately be `"duplicate"` | This phase (CUR-01) | `TestConsolidateNeverLabelsPairAsDuplicateOrCluster` (existing test, no-verdict call path) must keep passing unchanged; its scope is implicitly narrowed to "the no-verdict case," which should be reflected in an updated doc comment even though the test itself needs no logic change |

**Deprecated/outdated:** none — this is additive-only over a shipped Phase 2/earlier-milestone
foundation.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | New Go identifiers (`StoreAndDeciderFromEnv`, `RecordStates`, `verdictStateView`, `ParseProbability`, package name for the eval harness) are RESEARCH-proposed names for illustration, not locked | Standard Stack, Architecture Patterns | None if the planner treats them as illustrative — CONTEXT.md's Claude's Discretion explicitly covers "exact Go field names beyond the JSON keys" and "the truncation knob name" |
| A2 | `client.Get`'s `GetPoints.Ids` accepts multiple `*qdrant.PointId` values in one RPC (used to justify Pattern 1's batched fetch) | Architecture Patterns, Pattern 1 | Based on the existing single-id call shape at `internal/store/store.go:2018-2020` plus general Qdrant Go client knowledge of `GetPoints`; not independently re-verified against a live Qdrant instance this session. If wrong, fall back to one `client.Get` RPC per id chunked only by count, still far better than one per record with no view at all — low risk either way since the *view/ceiling* pattern (the actually load-bearing part) is unaffected |
| A3 | No existing docs-completeness gate covers `docs-site/guides/cli.md`'s consolidate prose (Pitfall 5's "may be a new, planner-authored gate") | Common Pitfalls, Pitfall 5 | Searched `internal/config`, `cmd/engram` for a test referencing `guides/cli.md` and found none this session; if one exists elsewhere it should surface as a straightforward find during planning, not a blocker |

**If this table is empty:** not applicable — see rows above; none of them change a locked
CONTEXT.md decision or a compliance/security requirement, so none require a user checkpoint
beyond ordinary plan review.

## Open Questions

1. **Where does `decide.Request` construction from `RecordState` live?**
   - What we know: `internal/store` must not import `internal/decide` (architectural boundary,
     verified). `cmd/engram` already imports both `internal/store` and would need
     `internal/decide`.
   - What's unclear: whether the request-building + verdict-mapping logic should live directly
     in `cmd/engram/spine_review_consolidate.go` (simplest, matches the file's existing
     self-contained `consolidateDoc`/`consolidateSummary` pattern) or in a small new
     `internal/decide`-adjacent package for independent unit-testability without cobra.
   - Recommendation: start in `cmd/engram` (matches the existing file's own pattern of
     colocating pure, testable functions like `consolidateSummary`/`consolidateDoc` next to the
     `RunE` that calls them); extract to a package only if the resulting file becomes unwieldy.
     This is implementation-detail Claude's Discretion, not a CONTEXT.md-locked choice.

2. **Does the `SurfaceFields` docs-prose narrowing for `RuleSweepScopeOrAllScopesRequired` still
   resolve correctly once consolidate becomes a fourth enforcer?**
   - What we know: the current narrowing (`scope`, `all-scopes`, `dry-run`) was chosen because
     it uniquely selects `summarize-missing` among the CURRENT three enforcers, deliberately
     excluding consolidate/purge (both non-enforcing). The comment says this was verified
     empirically against the live tree.
   - What's unclear: whether that empirical fact changes once consolidate enforces the rule
     (its own flag set does not gain `dry-run`, so the narrowing likely still resolves the same
     way — but this must be re-verified against the live tree, not assumed).
   - Recommendation: re-run the same empirical verification `08-01-PLAN.md fact 5` describes
     against the post-change command tree as part of implementing Pitfall 4's fix, rather than
     assuming the narrowing is unaffected.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|--------------|-----------|---------|----------|
| Go toolchain | Building/testing this phase | ✓ `[VERIFIED: go version go1.27.1 darwin/arm64, go.mod requires go 1.26.7 — this session]` | 1.27.1 (≥ 1.26.7 required) | — |
| Live Qdrant | `internal/store` integration tests, `task test` | Not probed this session (existing testcontainer-based test suite already handles this; unaffected by this phase's design work) | — | testcontainers, already wired |
| Live Decisions endpoint (`ENGRAM_DECISIONS_BASE_URL` + key) | CUR-03's real-pair eval mode (D-01's gitignored local file path), `task eval:decisions` smoke test | Not required for CUR-01/02/04 or the committed-synthetic-set half of CUR-03; only the optional real-spine eval mode needs it | — | Gated off by default (D-01, `internal/decide/jev/live_test.go`'s `ENGRAM_DECISIONS_LIVE` precedent) — CI never pays this cost |

**Missing dependencies with no fallback:** none — every capability this phase needs is either
already in-tree or gated off by default when unavailable.

**Missing dependencies with fallback:** the live Decisions endpoint for CUR-03's real-pair mode
— falls back to "committed synthetic set only," which is the mandatory, always-runnable half of
D-01's two-corpora design.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go's standard `testing` package, run via `task` (Taskfile.yaml) |
| Config file | none — `go test` needs no config file; the eval harness's own gate is package-local (mirrors `internal/retrievaleval/gate.go`, never registered in `internal/config`) |
| Quick run command | `go test ./cmd/engram/... ./internal/store/... ./internal/config/... ./internal/surfaces/...` |
| Full suite command | `task` (lint + `go test ./...`) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|---------------------|--------------|
| CUR-01 | Verdict object present in JSON when decider configured; absent when not | unit | `go test ./cmd/engram/ -run TestSpineReviewConsolidate -v` (extend existing test file) | ✅ existing file, ❌ new test cases (Wave 0) |
| CUR-01 | Text view renders verdict/probability/same-subject/needs-review, sanitized | unit | `go test ./cmd/engram/ -run TestOperatorViewFixturesHaveNoUnsanitizedNesting -v` | ✅ existing test, ❌ new fixture (Wave 0) |
| CUR-02 | Verdict below threshold marked `needs_review: true`; no write RPC is ever issued | unit | `go test ./cmd/engram/ -run TestConsolidate -v`; reuse `TestNearDuplicatesDoesNotMutate`-style before/after assertion pattern from `internal/store` if the new fetch method needs its own mutation-proof test | ✅ pattern exists (`internal/store`), ❌ new test for the verdict pass specifically (Wave 0) |
| CUR-03 | Relation question set measured on labeled pairs, accuracy by confidence bucket + Brier score, ≥0.9 accuracy at p≥threshold | integration (gated, live Decisions) | `ENGRAM_<GATE>=1 go test ./internal/<neweval package>/ -run TestCuration... -v` (new `task eval:curation`-style Taskfile target, mirroring `eval:decisions`/`eval:retrieval`) | ❌ entire package is Wave 0 |
| CUR-04 | `consolidate` with neither flag rejects with the registered rule sentence, `exitUsage` | unit | `go test ./cmd/engram/ -run TestSweepLeavesRejectMissingScopeIdentically -v` (existing table-driven test — extending `enforcingSweepLeaves` auto-extends coverage) | ✅ existing test file, ❌ new table entry (Wave 0 — one-line map edit) |
| Regenerated CLI help/catalog after new flags | `--verdict-threshold`/`--no-verdicts` flags and consolidate's changed `--all-scopes` Usage text reflected | golden | `go test ./cmd/engram -update` then `go test ./cmd/engram/ -run TestGolden -v` | ✅ existing mechanism, ❌ regenerated golden files (Wave 0/close-out) |
| Docs completeness | `configure.md` documents the 2 new `ENGRAM_DECISIONS_*` vars | unit | `go test ./internal/config/ -run TestDecisionsVarsDocumented -v` | ✅ existing test, ❌ literal-count bump + docs rows (Wave 0, Pitfall 3) |

### Sampling Rate

- **Per task commit:** the narrowest relevant package(s) from the Quick run command above.
- **Per wave merge:** `task test` (full `go test ./...`).
- **Phase gate:** `task` (lint + full suite) green, plus `go test ./cmd/engram -update` diff
  reviewed (not blindly committed) before `/gsd-verify-work`.

### Wave 0 Gaps

- [ ] `internal/store/boundedread.go` + `internal/store/spine.go` — new bounded view + batched
      fetch method, with its own mutation-proof and byte-ceiling tests (mirrors
      `TestNearDuplicatesDoesNotMutate`, `TestNearDuplicatesIsDeterministic`)
- [ ] `internal/server/decider.go` (or `tools.go`) — `StoreAndDeciderFromEnv` + threshold/
      truncation resolver helpers, with tests mirroring `decisionsTimeout`/`decisionsConcurrency`'s
      existing coverage shape
- [ ] `internal/config/validate.go` — new `ParseProbability`-shaped parser with its own positive/
      negative-boundary test table (mirrors `TestParsePositiveIntCap`-style coverage)
- [ ] A new `internal/<eval package>/` — entire package is new (gate, fixtures, blind-authoring
      doc, local-file loader, accuracy/Brier metrics, gated test entry point)
- [ ] `docs-site/src/content/docs/guides/configure.md` — 2 new table rows
- [ ] `docs-site/src/content/docs/guides/cli.md` — rewritten consolidate section (Pitfall 5)
- [ ] A new Taskfile target (`eval:curation` or similarly named) mirroring `eval:decisions`/
      `eval:retrieval`

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|----------------|---------|-------------------|
| V2 Authentication | no | This phase adds no new auth surface — consolidate is an existing operator-tier command with no change to its authn/authz posture |
| V3 Session Management | no | N/A — CLI, no session concept |
| V4 Access Control | no | `NearDuplicates`/consolidate remain Subject-less by design (unchanged); the new `RecordStates` fetch must follow the SAME Subject-less, no-owner-filter contract `NearDuplicates` already documents — this is a continuity requirement, not a new access-control surface |
| V5 Input Validation | yes | The new `--verdict-threshold` flag / `ENGRAM_DECISIONS_VERDICT_THRESHOLD` must be validated (range `[0,1]`) via `Config.Validate` + a new `ParsePositiveIntCap`-style parser (`ParseProbability`), following this codebase's existing "validated range == enforced range, one shared parser" convention (`internal/config/validate.go:400-411`'s WR-01-closing rationale) |
| V6 Cryptography | no | No new crypto surface; API key handling is unchanged (reuses `cfg.Decisions.APIKey`/`cfg.OpenAI.APIKey` fallback, already shipped and never logged — `internal/server/decider.go:150-168`) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|-----------------------|
| Stored record content reaching an operator's terminal unsanitized via a nested JSON object bypassing `sanitizeViewValue` (Pitfall 1) | Tampering (a crafted record content string could forge terminal control sequences / extra report lines — the exact T-06-03 threat this codebase already names) | Extend `viewRow`/`viewScalar` with an explicit, sanitizing path for the nested `verdict` object rather than letting it fall through to the raw, unsanitized `string(raw)` branch; prove it with a fixture that exercises `TestOperatorViewFixturesHaveNoUnsanitizedNesting` |
| Record content/summary leaving the deployment to a third-party decision provider (Jev/TypeSafe via OpenRouter) | Information Disclosure | Already disclosed and accepted at the config layer (`configure.md`'s "What leaves your deployment" paragraph, `ENGRAM_DECISIONS_PROVIDER` opt-in, D-04's off-by-default contract) — this phase is the FIRST feature to actually send record content, so `configure.md`'s "the features that actually ask it questions ship separately and will name this setting when they do" sentence must be updated to name consolidate, closing that disclosure gap |
| A verdict, however confident, causing a mutation | Elevation of Privilege / Tampering | Structural: `NearDuplicates` and every doc-building function issue zero write RPCs; the verdict pass must be built the same way — reading, computing, and rendering only, never calling `Supersede`/`Archive`/any write method. D-10's "advisory results never fail the sweep" and the milestone's "no automatic action on a verdict" Out-of-Scope row are the enforced invariants; no code path in this phase should call a mutating `Store` method at all |

## Sources

### Primary (HIGH confidence — read directly this session)

- `internal/decide/decide.go`, `internal/decide/many.go`, `internal/decide/errors.go` — Decider
  contract, DecideMany semantics, named error classes
- `internal/decide/jev/jev.go`, `internal/decide/jev/live_test.go` — Jev backend, SDK-rejection
  rationale, live-test gate precedent
- `internal/server/decider.go`, `internal/server/tools.go` (lines 195-270, 655-711) —
  `deciderFromConfig`, `StoreAndSummarizerFromEnv` precedent, config-load wiring
- `internal/store/spine.go` (lines 400-706), `internal/store/boundedread.go` (whole file) —
  `NearDuplicates`, `DuplicatePair`, the bounded-read mechanism
- `internal/store/store.go` (lines 1315-1325, 2007-2032) — `Get`, `SearchReranked`'s
  store-must-not-import-embed/server boundary comment
- `cmd/engram/spine_review_consolidate.go`, `cmd/engram/spine_review_consolidate_test.go`,
  `cmd/engram/operator_output.go`, `cmd/engram/operator_view.go`,
  `cmd/engram/operator_output_test.go` (lines 163-472), `cmd/engram/sweep_scope.go`,
  `cmd/engram/sweep_scope_test.go` — consolidate's existing implementation, the operator
  text/JSON rendering mechanism, the sweep-scope rule reuse pattern
- `internal/surfaces/rules.go` (lines 150-326), `internal/surfacesgen/main.go` (lines 95-137) —
  the `RuleSweepScopeOrAllScopesRequired` registration, its doc-comment-embedded exemption
  reasoning, and its docs-prose anchor
- `internal/config/config.go` (lines 210-240), `internal/config/registry.go` (lines 85-121),
  `internal/config/validate.go` (lines 286-349, 400-446), `internal/config/decisions_docs_test.go`
  (whole file) — `DecisionsConfig`, registry rows, `Config.Validate`, existing int-cap parsers,
  the docs-completeness gate and its hardcoded count
- `internal/retrievaleval/gate.go`, `internal/retrievaleval/doc.go`,
  `internal/retrievaleval/paraphrase_fixture.go` (lines 1-25) — the gated, unregistered-koanf
  test-harness precedent and blind-authoring documentation convention
- `skill/engram/skills/curating-spine/SKILL.md` (lines 65-118) — the pre-existing, DIFFERENT
  "verdict" vocabulary this phase's terminology collides with
- `docs-site/src/content/docs/guides/cli.md` (lines 130-280), `docs-site/src/content/docs/guides/configure.md`
  (lines 152-215) — prose that must change
- `Taskfile.yaml` (lines 77-88) — existing `eval:summary`/`eval:retrieval`/`eval:decisions`
  target precedents
- `.planning/phases/03-curation-verdicts/03-CONTEXT.md`, `.planning/REQUIREMENTS.md`,
  `.planning/STATE.md`, `.claude/skills/spike-findings-engram/references/curation-verdicts.md`,
  `.claude/skills/spike-findings-engram/references/decision-transport.md` — required reading,
  all read this session

### Secondary (MEDIUM confidence)

- None separately cited — every claim above was verified directly against source in this
  session rather than relayed from a secondary summary.

### Tertiary (LOW confidence)

- A2 in the Assumptions Log (multi-id `client.Get` batching) — based on the existing single-id
  call shape plus general Qdrant Go-client API knowledge, not independently re-verified against
  live Qdrant docs or a running instance this session.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every primitive this phase needs was read directly from source this
  session; nothing is new/unverified.
- Architecture: HIGH — the store-boundary constraint, the `StoreAndXFromEnv` precedent, and the
  text-rendering gap are all grounded in verbatim-quoted source, not inference.
- Pitfalls: HIGH — all six pitfalls are backed by direct source reads (test files, doc comments,
  docs-site prose) rather than speculation about what "might" break.

**Research date:** 2026-09-23
**Valid until:** 2026-10-23 (30 days — this is a fast-moving in-repo milestone; re-verify
`internal/decide`/`internal/store` line references if Phase 3 planning is delayed past other
phases landing first)
