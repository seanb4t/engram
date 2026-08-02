<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright 2026 Sean Brandt -->

# Phase 6: Rule Capture — Investigation & Fix - Context

**Gathered:** 2026-08-01
**Status:** Ready for planning
**Mode:** Smart discuss (autonomous), escalated to interactive — the roadmap's stated premise was
falsified during discussion and Sean restated the problem directly.

<domain>
## Phase Boundary

**The roadmap's framing was wrong and is superseded by this document.** ROADMAP.md asks *"why does
`store_rule` effectively never fire — one rule exists repo-wide against dozens of ordinary
memories"*. Two problems with that premise, both established during discuss:

1. **The count is stale.** Three rules exist in `rule:repo:github.com/seanb4t/engram`
   (`7smp8vy9hr`, `rvmts69cz1`, `0v4249kc9d`), two created after the roadmap was written.

2. **The count was never evidence.** Rules are per-scope, normative, always-shared and
   user-blessed; memories are per-fact and captured continuously. There is no principled target
   ratio between them, so "N rules against M memories" cannot by itself indicate a defect.

**The actual problem, as stated by Sean:** an agent with the skill and MCP installed *never
suggests* a rule. It has to be pushed or asked explicitly. The gap is in **noticing**, not in
storing.

This phase therefore delivers two halves:

- **Capture** — give the agent a recognition trigger and an inline proposal protocol, so rule
  candidates get surfaced without the user having to think of it first.
- **Curation** — a rule-hygiene discipline analogous to `curating-memory`, so that a rule set which
  now actually grows does not accumulate duplicates, contradictions, or rot.

Out of scope: any path that stores, edits, or deletes a rule without explicit user instruction.

</domain>

<decisions>
## Implementation Decisions

### Root cause (established during discuss, not deferred to research)

- **D-01 (the instruction to propose already exists and is buried inside its own prohibition):**
  `skill/engram/skills/curating-memory/SKILL.md:51-53` reads *"Store a rule **only on explicit user
  instruction**: if you believe something should be a rule, propose it to the user and let them
  bless it — never promote one unilaterally."* The bolded topic clause is a restriction and the
  closing clause is another restriction; the permission to propose sits unbolded between them. The
  net reading is "don't," not "notice and offer."

- **D-02 (the permission is conditioned on a state nothing in the skill produces):** *"if you
  believe something should be a rule"* has no trigger behind it. The routing table at `SKILL.md:12`
  routes facts to decision / preference / convention / gotcha; rules are described only as "the
  narrower, user-blessed, normative case" with no test for narrowness. The condition never fires,
  so the agent never proposes. **This is the root cause and it is a friction cause, not a
  mechanical one** — nothing blocks the call; the agent simply never reaches the point of making
  it.

- **D-03 (the symptom is reproducible from the store, not only from session transcripts):** three
  records filed as `category: gotcha` are phrased as normative constraints and were never proposed
  as rules — `r3bjakymtz` ("NEVER run two GSD executor agents concurrently in the SAME working
  directory"), `z4mgz3a4ab` ("ALWAYS `git diff .planning/ROADMAP.md` and hand-correct before
  committing"), `478rhhmhb0` (gate store tests on `--- PASS:`, never exit status). All three are
  repeat-hit footguns. This is the evidence base; the investigation does not need to reconstruct
  invocation attempts from logs.

- **D-04 (suggesting is not promoting, so the invariant is untouched):** the user-blessed gate
  governs who **decides**, not who **notices**. Adding a recognition trigger and a proposal
  protocol leaves the gate exactly where it is. Any intervention that stores a rule without
  explicit user instruction is out of scope — not a trade-off to be balanced.

### Capture — trigger and proposal

- **D-05 (two independent triggers, either one fires a proposal):** decided by Sean.
  - **Repeat-hit on a footgun** — the agent encounters a class of problem the store already
    records. `search_memory`'s search-before-store step already runs and already surfaces the prior
    record, so the signal is available without new machinery.
  - **Normative phrasing at capture time** — a fact about to be stored is phrased as MUST / NEVER /
    ALWAYS. It is already a constraint, so the question "should this be a rule rather than a
    gotcha?" is exactly on point. This trigger catches all three D-03 examples at the moment they
    were written.

- **D-06 (the proposal is made inline, at the moment the trigger fires):** decided by Sean — not
  batched to a session-end sweep. The justification for a candidate is strongest while the context
  that produced it is live, and a batched list asks the user to re-derive why each item is there.

- **D-07 (a declined proposal must not re-fire indefinitely):** an agent that re-proposes the same
  candidate every session converts a helpful trigger into a nag, which is the failure mode most
  likely to get the whole mechanism disabled. The design must state what happens on decline.
  Planning to determine the mechanism; the requirement is that repeated declining is not the user's
  only defense.

### Curation — rule hygiene

- **D-08 (the phase ships a rule-curation discipline, not only a capture trigger):** added by Sean
  during discuss. If the triggers work, rule volume rises, and an unmanaged normative set degrades.
  A rotted rule is worse than a rotted memory because rules are MUST-follow — it actively
  misdirects every future session. The discipline must cover duplicates, contradictions, and rot.

- **D-09 (correction is delete-then-re-store, because rules cannot be superseded):**
  `SKILL.md:208` — rules are normative ground truth and `set_visibility` is rejected for them. The
  supersession machinery memories use is unavailable here, so the curation half cannot be modelled
  on `supersede_memory`.

- **D-09a (delete-then-re-store applies to retirement, not to rewording — amended after planning):**
  D-09's premise holds but its conclusion was too broad. Reading the tree found two in-place paths:
  `store_rule` with `id` set does an ownership-validated replace that carries the existing
  `short_id` forward (`internal/server/rules.go:103-146`), and `update_memory` is permitted on a
  rule (`internal/server/tools.go:1501-1513`). So **retiring or reversing** a rule is
  delete-then-re-store, while **rewording or refining** one uses the in-place path — deleting to
  reword would churn a `short_id` that other records cite in their `related-*` tags. The hygiene
  discipline must distinguish the two cases rather than prescribing delete for both.

- **D-10 (rule deletion is user-blessed, symmetrically with creation):** deleting normative ground
  truth is the same class of act as creating it. The agent proposes "this rule appears rotted" or
  "this contradicts that one" and the user decides. Leaving deletion unilateral while creation is
  gated would put a hole in the invariant on the destructive side.

- **D-11 (contradiction and dedup checks cost a fetch, and the design must price that):** session
  start deliberately loads the rules **index** only — one line per rule, progressive disclosure by
  design. Full text requires `get_memory` per rule. The design must state when paying for that
  check is warranted rather than assuming the full rule set is already in context. At three rules
  it is cheap; at thirty it is not.

### Backfill

- **D-12 (a one-time sweep proposes the already-stored gotchas that read as rules):** decided by
  Sean. Surface the existing normatively-phrased gotchas (the D-03 three and any siblings) so each
  can be blessed or declined. Same gate as everything else — proposed, never promoted. This sweep
  doubles as the criterion-3 demonstration: rule capture firing in a scenario where it previously
  did not.

### Added after planning

- **D-13 (the defect has three sites, not one):** found by the planner. Besides
  `curating-memory/SKILL.md:51-53`, the identical buried-permission wording appears at
  `docs-site/src/content/docs/reference/tools.md:353-355`, and `CLAUDE.md:126-135` carries the
  purest instance — *"`store_rule` is invoked only on explicit user instruction (never promoted
  unilaterally)"*, two prohibitions with no permission at all. All three ship together or the
  corrected skill is contradicted by its own documentation the moment it lands. Follows the
  three-surface precedent set by v0.12.x plan 03-05.

- **D-14 (the cold read is performed by a fresh subagent, not by a blocking human checkpoint):**
  decided by Sean. The only real test of this phase is whether the corrected prose reads as
  permission to someone who did not help write it — which the orchestrator cannot self-administer,
  having produced the context. A subagent spawned with zero phase context, shown only the edited
  section and asked whether it would proactively offer a rule, is a genuine cold read at no
  interrupt cost. A negative result is a real failure of the phase's core deliverable, not advisory.

- **D-15 (a full-text rule check is one call, not N):** the planner corrected D-11's pricing.
  `list_rules` accepts `full`, so a contradiction or dedup pass over the whole set costs a single
  `list_rules(full=true)` rather than a `get_memory` per rule. D-11's underlying point stands — the
  check is not free and the session-start index deliberately does not pay for it — but the cost
  curve is far flatter than stated.

### Claude's Discretion

- Whether the trigger and the curation discipline live in `curating-memory/SKILL.md`, a sibling
  skill, or a split across both.
- The decline-memory mechanism for D-07.
- The exact wording of the normative-phrasing detector (which keywords, how much context).
- Whether the D-12 sweep is a documented procedure an agent follows or a one-off operator command.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets

- `skill/engram/skills/curating-memory/SKILL.md` (277 lines) — the routing table (`:12`), the Rules
  section (`:47-56`), the no-supersede constraint (`:208`). This is the primary surface.
- `skill/engram/hooks/session-start-memory-recall` — renders the rules **index** at session start
  via `list_rules`, plus the recall digest and the capture nudge. The natural place for any
  session-scoped part of the discipline.
- `skill/engram/hooks/tests/` — the hook has a real pytest suite (33 tests green this session), so
  hook-text changes are testable rather than prose-only.
- `internal/server/rules.go` — `validateStoreRule` and `listRules`. Relevant only if the
  investigation turns up a mechanical cause; per D-02 it did not.
- Sibling skills `discovering`, `promoting-memory`, `migrating-from-beads` — precedents for how a
  discipline is written in this repo.

### Established Patterns

- Rules surface as a progressive-disclosure index (one line per rule) with full text fetched on
  demand — the constraint behind D-11.
- `search_memory` is semantic, so a natural-language description of a fact surfaces related records
  for dedup without shared keywords. This is what makes D-05's repeat-hit trigger cheap.
- Rule `7smp8vy9hr` already establishes milestone-completion as a memory-curation checkpoint — a
  precedent the rule-hygiene discipline could hook into rather than inventing a new cadence.

### Integration Points

- `skill/engram/skills/curating-memory/SKILL.md` (trigger, proposal protocol, curation discipline)
- `skill/engram/hooks/session-start-memory-recall` + its pytest suite
- `docs-site/src/content/docs/` — if the discipline is operator-visible
- Possibly a new sibling skill under `skill/engram/skills/`

</code_context>

<specifics>
## Specific Ideas

- Sean's words for the problem: *"why an agent with the skill/mcp installed doesn't suggest rules
  to capture — it has to be pushed/asked explicitly. Seems like finding high friction/repeated
  footguns would be a good trigger for a rule?"*
- Sean's addition on curation: *"we need to add in something similar to curate memory, and ensure
  that we don't duplicate rules, create contradictory ones, or have rules that have rotted."*
- ROADMAP.md's Phase 6 goal sentence and REQUIREMENTS.md's parenthetical both need amending — they
  assert the falsified "one rule exists repo-wide" premise.
- Criterion 3 as written (*"rule capture demonstrably fires in a scenario where it previously did
  not"*) is satisfiable via D-12's sweep.
- The whole milestone rides one branch (`git.branching_strategy: none`). No phase branch.

</specifics>

<deferred>
## Deferred Ideas

- **Any automatic rule promotion.** Permanently out of scope, not deferred — D-04.
- **Telemetry on proposal accept/decline rates.** Would answer "is the trigger calibrated?" but
  needs a data path this phase does not have.
- **Rule scoping beyond `rule:repo:*` / `rule:project:*`.** Not in scope.

</deferred>
