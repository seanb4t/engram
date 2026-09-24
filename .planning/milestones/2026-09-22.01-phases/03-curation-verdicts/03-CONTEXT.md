# Phase 3: Curation Verdicts - Context

**Gathered:** 2026-09-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Operators running `engram spine-review consolidate` see advisory Jev relation verdicts per
candidate pair, built on Phase 2's `internal/decide` interface (CUR-01, CUR-02), measured by a
labeled pair eval (CUR-03), and the command stops silently reporting zero candidates when
neither `--scope` nor `--all-scopes` is given (CUR-04, #508).

Verdicts are **advisory only**: no verdict, at any confidence, ever causes consolidate (or any
other path) to mutate a record. Not in this phase: the recall reranker (Phase 4), write-time
hints (DEC-F1), changes to the curating-spine skill's consent contract.

</domain>

<decisions>
## Implementation Decisions

### Pair eval corpus (CUR-03)
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

### Verdict surface & opt-in (CUR-01)
- **D-04:** Verdicts run **by default when a decisions provider is configured**
  (`ENGRAM_DECISIONS_PROVIDER` set), with **`--no-verdicts`** to suppress. With no provider,
  consolidate is byte-identical to today (no flag needed, no outbound calls). — **Reversibility:**
  reversible — a CLI default, flippable later.
- **D-05:** JSON shape: each pair gains a **nested `verdict` object** —
  `verdict: {relation, probabilities{duplicate, contradicts, updates, related, unrelated},
  same_subject, needs_review, model}`; on failure `verdict: {error: "<named class>"}`. The object
  is **absent entirely** when verdicts did not run (additive-only contract for the operator JSON).
  **No separate top-`probability` field** — the chosen relation's probability is
  `probabilities[relation]` (single source of truth; the full map keeps close calls visible).
- **D-06:** Relation option set is **5 options: `duplicate` / `contradicts` / `updates` /
  `related` / `unrelated`** (criteria from the spike blueprint, plus `updates` = "B is a newer
  state or a more complete version of the same fact"), plus a `same_subject` Noul question —
  one Decisions request per pair (batch both questions on the pair's state).
- **D-07:** The text view renders the verdict, its probability, same-subject probability, and a
  `needs review` marker (rendered view of the JSON contract).

### Threshold & truncation (CUR-02)
- **D-08:** Needs-review threshold: registered config key
  **`ENGRAM_DECISIONS_VERDICT_THRESHOLD`** (default 0.9) with a **`--verdict-threshold`** flag on
  consolidate overriding per run. A verdict whose top probability is below the threshold is
  `needs_review: true`.
- **D-09:** State per record = **summary (when present) + head of content**, N ≈ 1500 chars per
  side (configurable via a registered knob), keeping the deciding claim more often than blind
  truncation.

### Failure & cost policy
- **D-10:** A failed pair decision → `verdict: {error: "<named class>"}` (JSON) and
  `verdict unavailable (<class>)` (text); the report **completes with exit 0** plus a stderr
  summary line with counts; an all-pairs auth failure still exits 0 but warns loudly. Advisory
  results never fail the sweep (DEC-04 contract).
- **D-11:** **No cap** on pairs sent per run — every candidate pair gets a verdict (uses Phase 2's
  `DecideMany` with `ENGRAM_DECISIONS_CONCURRENCY` bounding in-flight calls).

### Scope guard (CUR-04)
- **D-12:** Reuse the existing sweep-scope rule (`requireSweepScope` / `sweepScopeRule` /
  `surfaces.RuleSweepScopeOrAllScopesRequired`) so consolidate with neither `--scope` nor
  `--all-scopes` returns the same rule error the other sweep leaves return, and publish the
  rule sentence on consolidate's usage now that it is enforced (#508).

### Claude's Discretion
- Exact Go field names beyond the JSON keys above; the truncation knob name; how the eval
  harness selects the local pair file (env var or flag); synthetic pair domains.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope
- `.planning/ROADMAP.md` §"Phase 3: Curation Verdicts" — goal and success criteria
- `.planning/REQUIREMENTS.md` — CUR-01..CUR-04
- GitHub #508 — consolidate scope guard

### Spike blueprint
- `.claude/skills/spike-findings-engram/references/curation-verdicts.md` — request shape, criteria text, 0.9 threshold evidence, miss classes, `updates` proposal
- `.claude/skills/spike-findings-engram/references/decision-transport.md` — wire facts

### Prior phases
- `.planning/phases/02-decision-interface-jev-backend/02-CONTEXT.md` and summaries — `internal/decide` contract (`Decide`, `DecideMany`, Choice/Noul, named errors), `deciderFromConfig`, `ENGRAM_DECISIONS_*`
- `.planning/phases/01-eval-foundation-lexical-reranker-fix/` — gated-eval + blind-authoring precedent (`internal/retrievaleval`, koanf test-local gate)

### Code
- `cmd/engram/spine_review_consolidate.go` — consolidate command, `consolidateDoc`, text summary
- `internal/store/spine.go` — `NearDuplicates`, `DuplicatePair`, empty-result contract for Scope:"" AllScopes:false
- `cmd/engram/sweep_scope.go` and `spine_review_scan.go` / `spine_review_verify.go` / `summarize.go` — the sweep-scope rule pattern to reuse
- `internal/server/decider.go` — decider construction
- `.claude/skills/` curating-spine (engram plugin) — explicit-consent contract that must stay unchanged

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/decide.DecideMany` — per-item results, bounded concurrency (D-11).
- `deciderFromConfig` — returns nil when provider unset (D-04 byte-identical default).
- Sweep-scope rule helpers already enforced on three sweep leaves (D-12).

### Established Patterns
- Operator JSON (`--output json`) is the contract; text is a rendered view.
- Gated evals resolve their gate via test-local koanf (Phase 1 D-15), never registered.
- Registry-derived docs gate (`TestDecisionsVarsDocumented`) will require documenting the new threshold/truncation keys.

### Integration Points
- consolidate's `RunE` → store `NearDuplicates` → (new) verdict pass over pairs → `consolidateDoc` / summary.

</code_context>

<specifics>
## Specific Ideas

- Pair `same_subject` high + `related` is worth a look (spike guidance) — the text view could flag it; planner's call.
- Probability comparisons never use exact equality (±0.03 run-to-run variance).

</specifics>

<deferred>
## Deferred Ideas

- Per-run verdict cap (`--max-verdicts`) — considered, declined for now (D-11).

</deferred>

---

*Phase: 03-curation-verdicts*
*Context gathered: 2026-09-23*
