# Phase 4: Spine Curation — Semantic (Skill) - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-10
**Phase:** 4-Spine Curation — Semantic (Skill)
**Areas discussed:** Skill home & triggering, CLI↔skill handoff, Verdict → mutation mapping, Consent granularity at scale

---

## Skill home & triggering

**Q1 — Where should the semantic curation content live?**

| Option | Description | Selected |
|--------|-------------|----------|
| New sibling skill | Own description, cold trigger; keeps curating-memory's hot path unchanged at 486 lines | ✓ |
| Extend curating-memory | One place for everything; pushes to ~700 lines on a skill loaded before every durable write | |
| New skill + delegation pointer | Heavy content separate, curating-memory points at it; two files to keep consistent | |

**Q2 — What should make the new skill fire?**

| Option | Description | Selected |
|--------|-------------|----------|
| Explicit invocation only | Deliberate ask only; never ambient (recommended) | |
| Explicit + milestone-completion hook | Also at milestone close, where rule `7smp8vy9hr` already establishes a curation pass | |
| Explicit + reactive on recall | Also when a recall surfaces a record that looks stale against the tree | ✓ |

**Notes:** The reactive option was flagged as closest to the forbidden auto-extraction. On
examination the objection did not hold: the standing invariant constrains **writes**, and the
reactive path neither creates nor mutates. The real cost is noise and expense, which Q4 bounded.

**Q3 — Should the new SKILL.md join `internal/surfaces` proseTargets?**

| Option | Description | Selected |
|--------|-------------|----------|
| You decide | Let research settle it from what the skill actually states | ✓ |
| Yes, join proseTargets | Consistent with curating-memory and discovering | |
| No, keep it out | Avoid a gate that passes vacuously | |

**Notes:** Recorded as Claude's discretion — default OUT, join only if the skill restates a
registered conditional rule.

**Q4 — How bounded should the reactive trigger be?**

| Option | Description | Selected |
|--------|-------------|----------|
| Free evidence only | Fire only when staleness is visible from what's already in context; one-line note, never a proposal | ✓ |
| Cheap verification allowed | May spend a bounded read to confirm before surfacing | |
| Notice silently, batch at session end | Accumulate and surface once before finishing | |

---

## CLI↔skill handoff

**Q1 — How does the skill get its near-duplicate candidate pairs?**

| Option | Description | Selected |
|--------|-------------|----------|
| Consume consolidate output | Operator runs `spine-review consolidate --output json`; skill judges, never derives | ✓ |
| Derive from search_memory scores | Fully MCP-only; re-embeds every record and is not exhaustive | |
| Both, CLI preferred | Widest reach; two prose paths, and the fallback's non-exhaustiveness must be stated | |

**Q2 — For records with NO citations, where does staleness evidence come from?**

| Option | Description | Selected |
|--------|-------------|----------|
| Skill extracts refs from prose | Covers the whole spine; extraction is a judgment call, so findings must carry checkable evidence | ✓ |
| Citation-bearing records only | Exact, no guesswork; covers a deliberate minority of the spine | |
| Extract refs, and propose backfilling citations | Compounding value; adds a second proposal type | |

**Q3 — What happens when the skill can't reach a confident answer?**

| Option | Description | Selected |
|--------|-------------|----------|
| Explicit unverifiable tier | Mirrors verify's fourth tier; never a confident wrong verdict | ✓ |
| Stay silent | Quietest; makes absence indistinguishable from "checked and fine" | |
| Ask the user to adjudicate | Most accurate; turns every uncertain record into an interruption | |

**Q4 — Should the skill read the tree during a deliberate sweep?**

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, read the tree freely | The only way the goal is answerable at all | |
| Read only within the record's own scope | Bounds blast radius; a sibling-repo record becomes unverifiable by construction | |
| You decide | Let research settle the read budget | |

**User's choice:** Free text — *"yes, but encourage use of tools such as codegraph, ast-grep, etc
to make searches cheap, if they're available."*

**Notes:** Confirmed all three are present in the authoring environment (`.codegraph/` exists;
`sg` and `codegraph` on PATH), so the preference is grounded rather than aspirational. Captured as
a precedence ladder in which every rung is optional, since the skill ships to users who may have none.

---

## Verdict → mutation mapping

**Q1 — What staleness verdict vocabulary?**

| Option | Description | Selected |
|--------|-------------|----------|
| Mirror verify's four tiers | Same names and order as the shipped citation classifier | ✓ |
| Truth-oriented vocabulary | Names what happened to the fact rather than the reference | |
| Verdict plus confidence | More information per finding; confidence risks substituting for evidence | |

**Q2 — How does a verdict map to an engram verb?**

| Option | Description | Selected |
|--------|-------------|----------|
| Skill proposes the verb, always with evidence | Reuses curating-memory's existing three-way table | ✓ |
| Fixed verdict→verb mapping | Predictable and testable; wrong often enough to matter | |
| Skill reports, user picks the verb | Maximum safety; pushes back most of the work this phase exists to do | |

**Q3 — What identity verdicts for a candidate pair?**

| Option | Description | Selected |
|--------|-------------|----------|
| same-fact / overlapping / distinct | Three tiers; `overlapping` is the dangerous middle and most common real case | ✓ |
| Two tiers: duplicate / not | Simpler; collapses the overlapping case where a wrong merge destroys a caveat | |
| Same four-tier shape as staleness | Symmetric; forces identity judgments into reference-checking tier names | |

**Q4 — Given `supersede_memory` takes a single target, how should a `same-fact` merge be proposed?**

| Option | Description | Selected |
|--------|-------------|----------|
| Supersede the loser with the survivor | One call; survivor gains nothing the loser had | |
| Author a merged record, supersede both | Two calls presented as one proposal | (challenged) |
| Never merge; propose update-then-delete | Delete destroys the history supersession exists to keep | |

**User's choice:** Free text — *"given that retry works, and we don't delete, why would we not go
with 2?"*

**Notes:** The pushback was correct about safety and exposed that the stated cost was wrong. Reading
the code showed the real blocker: `Store.Supersede` **always** creates a record (unconditional
`s.Upsert` at `internal/store/store.go:2029-2032`) and `Update` restores `SupersededBy` from a fresh
read (`store.go:1755`), so no verb links an existing record to an existing record. Two supersede
calls therefore produce **two** merged records with identical content, not one record with two
links. Reducing two live records to one is not expressible without a delete.

**Q4b — Given that, how should `same-fact` pairs be proposed?**

| Option | Description | Selected |
|--------|-------------|----------|
| Supersede one + delete the copy | Narrowly scoped to true same-fact; duplicate reads as junk under curating-memory's table | ✓ (superseded by Q5) |
| Never delete; leave both, link one | Duplicate survives in recall and is re-proposed every sweep | |
| Propose nothing; report only | Honest about the gap; leaves curation manual | |

**Q4c — Should the API gap be filed?**

**User's choice:** File it, *"but use /gsd-capture --todo"*.
Written to `.planning/todos/pending/2026-08-10-supersede-memory-cannot-merge-two-records-into-one.md`
with both candidate fixes (multi-target `supersedes`, or a link-existing verb) and the chain and
`idempotency_key` constraints each inherits.

**Q5 — What should the skill propose for an `overlapping` pair?**

| Option | Description | Selected |
|--------|-------------|----------|
| Update one to absorb, keep both | Needs no verb that doesn't exist; the pair stays a pair | |
| Report only, never propose | Safest; leaves the hardest case entirely manual | |
| Propose a supersede-with-union on one side | Leaves two live records with overlapping content | |

**User's choice:** Free text — *"seems like this is a different case/type of merge, and we can
simplify this with the previous question too? Maybe that api change we just todo'd?"*

**Notes:** Correct, and it restructured the area. `same-fact` and `overlapping` are the same
operation differing only in what text the survivor carries. Multi-target `supersede_memory` expresses
both in one call with no delete, which also retires the Q4b workaround. This made the identity
verdicts descriptive rather than mechanical.

**Q6 — How should the API change enter v0.13.x?**

| Option | Description | Selected |
|--------|-------------|----------|
| Ship skill now, simplify later | Keeps Phase 4 zero-server-code and unblocked; prose gets simplified later | |
| Add the API change to this milestone | One coherent design, no delete, no rework; needs a roadmap + requirements edit | ✓ |
| Author the skill against the future API | Ships prose describing a call that fails today; violates `4aksmneehh` | |

**Q7 — How should the verb enter the roadmap?**

| Option | Description | Selected |
|--------|-------------|----------|
| Insert as a new phase after Phase 3 | Go/proto work separated from a prose deliverable | ✓ |
| Expand Phase 4 to cover both | No renumbering; mixes proto/store/gen with prose in one phase | |
| Insert before Phase 3 | Not viable — Phase 3 already shipped | |

**Notes:** Executed via `gsd-tools phase insert 3 "..."`, which used decimal numbering (**03.1**)
rather than renumbering — so Phase 4 kept its number and directory. The tool wrote a stub
(full description as heading, `[Urgent work - to be planned]` goal); goal, requirements, success
criteria, and research flag were filled into the generated shape. Verified the roadmap still parses
via `getMilestonePhaseFilter`, which now returns all five directories plus `03.1`.

---

## Claude's Discretion

- Whether the new `SKILL.md` joins `internal/surfaces` `proseTargets` — default OUT, join only if
  the skill restates a registered conditional rule.
- The skill's final name (`curating-spine` provisional) and its description wording, which
  determines triggering fidelity for both paths in D-02.
- How a `distinct` judgment is recorded so a false-positive pair is not re-proposed every sweep.

## Deferred Ideas

- Proposing citation backfill during a sweep — compounding value, but adds a second proposal type
  and risks the reflexive-citation habit `curating-memory` warns against.
- `spine-review consolidate --apply` — a CLI-side merge inheriting Phase 03.1's verb.
- Recording a `distinct` judgment durably, if it turns out to need a store change.
- Milestone-completion curation hook — offered as a trigger and not chosen; remains available.
