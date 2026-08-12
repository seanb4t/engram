# Phase 4: Spine Curation — Semantic (Skill) - Context

**Gathered:** 2026-08-10
**Status:** Ready for planning

<domain>
## Phase Boundary

A new agent-side skill that judges two things about stored memories — **staleness** ("is this
record still true against the tree it describes") and **near-duplicate identity** ("are these the
same fact") — and proposes mutations that always stop at explicit user consent.

The skill is the *semantic* half of a pairing whose *structural* half shipped in Phase 3:
`spine-review consolidate` produces ranked candidate pairs with deliberately **no** cluster verdict
label (D-15), and `spine-review verify` classifies citations into four tiers. This phase supplies
the judgment those two withhold.

Not in scope: producing candidates (Phase 3 owns that), and the multi-target `supersede_memory`
verb (Phase 03.1, inserted during this discussion — see Decisions D-08).

</domain>

<decisions>
## Implementation Decisions

### Skill home & triggering

- **D-01:** The content lives in a **new sibling skill**, not in `curating-memory`. Home:
  `skill/engram/skills/curating-spine/` (name provisional). Rationale: `curating-memory` is already
  486 lines and its description fires on a very hot path — before every `store_memory` /
  `schedule_memory` / `supersede_memory` call — so folding a cold, rare, long procedure into it
  taxes every durable write in the repo. The three existing sibling skills (`discovering`,
  `promoting-memory`, `migrating-from-beads`) are 36–65 lines and scoped exactly this way.
  — **Reversibility:** reversible — a skill file can be merged into another later; no published
  contract depends on which file the prose lives in.

- **D-02:** The skill fires on **explicit invocation AND reactively on recall**. The user chose the
  reactive option over explicit-only and over a milestone-completion hook. This does not violate the
  standing "no auto-extraction, no similarity-triggered supersession" invariant: that invariant
  constrains **writes**, and the reactive path neither creates nor mutates — it notices and routes
  to the consent gate.

- **D-03:** The reactive trigger is bounded to **free evidence only**. It fires ONLY when staleness
  is visible from what is already in context — the recalled record cites a file or commit the agent
  already has open or just read, and plainly contradicts it. No extra reads, no tree-walking to hunt
  for drift. It surfaces a **one-line note**, never a proposal; only deliberate invocation opens the
  full flow. Rationale: a skill that interrupts often is the one that gets turned off — the exact
  failure mode `curating-memory`'s rule-proposal section spends four paragraphs guarding against.

### Evidence sources — the CLI↔skill handoff

- **D-04:** Near-duplicate candidate pairs are **consumed from `engram spine-review consolidate
  --output json`**, never derived by the skill. The skill judges identity; it does not produce
  candidates. Rationale: `search_memory` returns an always-on `score`, so the skill *could* derive
  pairs — but that is one query per record with a re-embed each time, and it is not exhaustive.
  `consolidate` already sweeps the filtered set with `QueryBatch` + `NewQueryID` over
  already-stored vectors, no re-embedding. Accepted cost: an MCP-only agent with no binary cannot
  produce candidates standalone.

- **D-05:** For records with **no citations** — the well-formed default in this repo — the skill
  **extracts checkable refs from the record's own prose** (paths, symbols, commits) and checks those
  against the tree. Rationale: `spine-review verify` can only speak to citation-bearing records, a
  deliberate minority, so citation-only scoping would deliver far less than the goal statement
  implies. Because extraction is a judgment call, findings MUST be reported as **checkable
  evidence** — "this record says X about `internal/store/spine.go:400`; that function no longer
  exists" — never as a bare verdict.

- **D-06:** During a **deliberate sweep** the skill may read the tree, but MUST prefer cheap
  structural search tools when available, degrading gracefully when they are not. User's words:
  *"yes, but encourage use of tools such as codegraph, ast-grep, etc to make searches cheap, if
  they're available."* Precedence ladder: **codegraph** (`explore` / `impact` / `callers`) →
  **ast-grep / `sg`** for structural shapes a text regex cannot express → **`rg`** for text →
  `Read` only the enclosing region. All three are present in the authoring environment
  (`.codegraph/` exists; `sg` and `codegraph` on PATH), but the skill ships to users who may have
  none, so **every rung must be optional**. Note the deliberate asymmetry with D-03: explicit
  invocation is what buys the read budget; the reactive path never gets it.

### Verdicts and verb mapping

- **D-07:** Staleness verdicts **mirror `spine-review verify`'s four tiers** — `valid` / `moved` /
  `broken` / `unverifiable` — same names, same order, extended from citations to prose-extracted
  refs. One vocabulary across CLI and skill so an operator reads both reports the same way; `moved`
  stays separate from `broken` exactly as Phase 3 established. When the skill cannot reach a
  confident answer it reports **`unverifiable` with the reason** (path outside the tree, ambiguous
  rename, different repo) — never a confident wrong verdict, and never silence, because absence of
  a finding must not be indistinguishable from "checked and fine".
  — **Reversibility:** costly — the tier vocabulary would appear in the skill prose, any JSON the
  skill emits, and operator muscle memory shared with `verify`; changing it later means changing
  both surfaces together or accepting a split vocabulary.

- **D-08:** Identity verdicts for a candidate pair are **`same-fact` / `overlapping` / `distinct`**.
  `same-fact` = one record should survive. `overlapping` = they share ground but each carries
  something the other does not (the dangerous middle, and the most common real case). `distinct` =
  high cosine score, different facts — a false positive from ranking, recorded so the pair is not
  re-proposed every sweep.

  **These verdicts no longer drive divergent mechanics.** During discussion the user observed that
  `same-fact` and `overlapping` are the same operation differing only in what text the survivor
  carries — the better of the two, or the union. Phase 03.1's multi-target `supersede_memory`
  expresses both as one call: `supersede_memory(content = <the right statement>, supersedes = [A,
  B])` — one new record, every predecessor linked, history preserved for all of them, **no
  `delete_memory` anywhere in the merge path**. The verdict now tells the user how much authoring
  judgment went into the survivor; it does not select a different verb.

- **D-09:** The skill **proposes the verb, always with the evidence that drove the choice**, using
  `curating-memory`'s existing three-way table (`supersede_memory` when a true fact became wrong,
  `update_memory` when sharpening wording, `delete_memory` for junk that should never have existed)
  rather than inventing a parallel one. Rejected: a fixed verdict→verb mapping — it is wrong often
  enough to matter, since a `broken` ref can mean the fact reversed OR merely that a path changed.
  Rejected: report-only — that pushes back onto the user most of the work this phase exists to do.

### Consent

- **D-10:** Consent is **batch review, per-item confirm**. All findings are presented as one
  reviewable report grouped by verdict, then each mutation is confirmed individually before it runs.
  The user sees the whole picture before deciding anything, and no mutation happens without its own
  yes. This preserves `store_rule`'s "propose, never promote" at the item level while paying the
  interruption cost once rather than once per finding. Rejected: strict one-at-a-time (a sweep over
  hundreds of records becomes hundreds of interruptions — the nag failure mode); rejected: batch
  approve by verdict class (one yes authorizing many mutations is closest to the unilateral
  promotion REQ-consent-never-perform forbids).

### Claude's Discretion

- Whether the new `SKILL.md` joins `internal/surfaces` `proseTargets` (the conditional-rule
  conformance gate at `internal/surfaces/conformance_test.go:25-26`). **Default OUT**; join only if
  the skill actually restates a registered conditional rule, so the gate never passes vacuously —
  the same false-green class this repo has been bitten by. Research/planning settles it.
- The skill's final name (`curating-spine` is provisional) and its description wording, which is
  what actually determines triggering fidelity for both D-02 paths.
- How the `distinct` decision is recorded so a false-positive pair is not re-proposed every sweep.
  Nothing in the current store expresses "these two were judged unrelated" — this may need a tag
  convention, or may be out of scope.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### The consent protocol and its proof method

- `skill/engram/skills/curating-memory/SKILL.md` — the consent protocol being reused verbatim
  (§ "Proposing a rule": propose inline, show the exact index line, ask once then stop, record the
  decline as `category: decision` + tag `rule-declined`), and the three-way verb table D-09 reuses
  (§ "Discipline" item 2).
- `.planning/milestones/v0.12.x-phases/06-rule-capture-investigation-fix/06-COLD-READ.md` — the
  **only** internal precedent for the adversarial cold-read test, and the artifact the ROADMAP's
  research flag says deserves a focused pass rather than mechanical reuse. Its method is the part
  that matters: a `general-purpose` subagent with zero phase context, a realistic session state, one
  open question, and **the framing never used the word "rule"** — a test that names its subject
  pre-loads the answer it is trying to measure.
- `.planning/milestones/v0.12.x-phases/06-rule-capture-investigation-fix/06-CONTEXT.md` — D-01/D-02
  (why a string check cannot detect the defect) and D-14 (why neither executor nor orchestrator can
  administer the test).

### What Phase 3 built that this phase consumes

- `internal/store/spine.go` — `NearDuplicates` (`QueryBatch` + `NewQueryID` over stored vectors,
  deterministic collapsed `(A,B,score)` pairs); the citation four-tier classifier; `scrollAllPoints`.
- `cmd/engram/spine_review_consolidate.go` — the `--output json` shape D-04 consumes, including
  `--min-score` / `--top-k` and the deliberate absence of any cluster verdict label (D-15).
- `.planning/phases/03-spine-curation-structural-cli/03-CONTEXT.md` — D-15 (ranked pairs, no
  clustering) and D-12 (`archived_at` orthogonality), both of which constrain what a proposal may say.
- `.planning/phases/03-spine-curation-structural-cli/COVERAGE.md` — the Qdrant surface decisions,
  including why `SearchMatrixPairs` was rejected (sampled results make absence of a pair
  indistinguishable from non-similarity — the same false-green reasoning behind D-07's rejection of
  silence).

### The API this phase now depends on

- `.planning/ROADMAP.md` § "Phase 03.1: Merge Supersession" — inserted during this discussion.
- `.planning/todos/pending/2026-08-10-supersede-memory-cannot-merge-two-records-into-one.md` — the
  full analysis: why `Store.Supersede` always creates a record
  (`internal/store/store.go:2029-2032`), why `Update` refuses to let a caller set `superseded_by`
  (`store.go:1755`), the four-row table of failed merge attempts, and the two candidate fixes.
- `internal/store/store.go:2000-2042` — `Supersede`'s per-target lock, owner-only write gate, and
  single-live-head rejection, all of which Phase 03.1 must generalize to a set.

### Contract and conventions

- `CLAUDE.md` § "Memory contract (stable)" — supersession semantics, the soft-hide recall gate, and
  the explicit note that supersession is *never automatic*.
- `docs-site/src/content/docs/reference/errors.md` — the `field=<name> hint=<code>` envelope the
  skill must read rather than pattern-matching error prose.
- `internal/surfaces/conformance_test.go:15-30` — `proseTargets`, relevant to the Claude's-Discretion
  item above.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **`curating-memory`'s consent protocol** — a four-step propose/decline flow with a written-down
  decline record shape. REQ-consent-never-perform says reuse it verbatim rather than inventing a
  second consent shape; D-10 layers batching on top without changing the per-item semantics.
- **`spine-review verify`'s four-tier classifier** — the tier names, their order, and the discipline
  that produced them (never a confident wrong verdict; `moved` reported separately from `broken`;
  start-anchored at-locator definition). D-07 extends the vocabulary, not the implementation.
- **`spine-review consolidate --output json`** — the candidate feed for D-04.
- **`search_memory`'s always-on `score`** — available as a fallback signal, deliberately NOT the
  primary candidate source (D-04).

### Established Patterns

- **Correct-by-reading (`4aksmneehh`)** — help text and self-describing output are deliverables with
  acceptance criteria; a validation error is a backstop for someone who did not read, never the
  teaching mechanism. For a skill, this means the prose must make the right action evident, not
  teach by showing a failure.
- **Zero-junk capture** — no auto-extraction, no similarity-triggered supersession. D-02's reactive
  trigger sits deliberately on the *notice* side of that line; D-03 is what keeps it there.
- **Skills are small and single-purpose** — 36–65 lines for the three siblings; `curating-memory` at
  486 is the outlier, and D-01 exists to avoid making it worse.
- **Evidence over verdict** — Phase 3's `verify` never leaks excerpt text but always names what it
  checked. D-05 and D-09 carry that forward: the user must be able to overrule a proposal in one read.

### Integration Points

- **Phase 03.1's multi-target `supersede_memory`** — a hard dependency introduced by this discussion
  (D-08). REQ-semantic-curation-skill was rewritten from "zero new server-side code" to reflect it.
- **`spine-review consolidate` JSON** — the input boundary (D-04).
- **The engram MCP tool surface** — `list_memory` / `search_memory` / `get_memory` /
  `update_memory` / `supersede_memory` / `delete_memory`; the skill adds no server-side code of its
  own beyond Phase 03.1's verb.
- **The plugin manifest** — `skill/engram/.claude-plugin/plugin.json` is version-synced by
  release-please; a new skill directory must be reachable through whatever mechanism installs the
  existing four.

</code_context>

<specifics>
## Specific Ideas

- **Cheap search is a requirement, not a nicety.** The user explicitly asked that the skill
  "encourage use of tools such as codegraph, ast-grep, etc to make searches cheap, if they're
  available." The conditional matters as much as the preference — the skill must work without them.

- **No deletes in the merge path.** The user pushed back twice on delete-bearing designs and drove
  the discussion to the API change that removes the need for one. Phase 03.1 exists because of this;
  a plan that reintroduces `delete_memory` into merging has misread the intent.

- **The API gap was found by following the mechanics, not by asserting them.** `supersede_memory`
  looked like it could merge until `Store.Supersede`'s unconditional `Upsert` was read. Downstream
  agents should verify the same way rather than trusting the summary here.

</specifics>

<deferred>
## Deferred Ideas

- **Proposing citation backfill during a sweep** — when the skill extracts a checkable ref from
  prose, it could also propose attaching it as a structured citation so the next sweep is structural
  rather than inferential. Compounding value, but it adds a second proposal type and risks the
  reflexive-citation habit `curating-memory` explicitly warns against. Not in this phase.

- **`spine-review consolidate --apply`** — a CLI-side merge that would inherit Phase 03.1's verb.
  Out of scope here; the skill proposes, the operator confirms.

- **Recording a `distinct` judgment durably** — see Claude's Discretion. If it needs a store change
  it is a separate phase, not this one.

- **Milestone-completion curation hook** — offered as a trigger option and not chosen. Rule
  `7smp8vy9hr` already establishes a curation pass at milestone completion; wiring this skill into
  that moment remains available later.

</deferred>

---

*Phase: 4-Spine Curation — Semantic (Skill)*
*Context gathered: 2026-08-10*
