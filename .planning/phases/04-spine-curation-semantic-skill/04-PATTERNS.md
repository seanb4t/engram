# Phase 4: Spine Curation — Semantic (Skill) - Pattern Map

**Mapped:** 2026-08-11
**Files analyzed:** 2 (1 primary deliverable + 1 test artifact)
**Analogs found:** 2 / 2

This phase ships prose (a `SKILL.md`) and a manual test artifact, not application code. There is no
Go/TS file to create or modify — RESEARCH.md's own commitment is "zero new server-side code beyond
Phase 03.1's already-merged multi-target `supersede_memory`." Pattern extraction below covers the
two artifacts CONTEXT.md/RESEARCH.md actually require.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `skill/engram/skills/curating-spine/SKILL.md` (path per D-01; name provisional) | skill (agent-read prose, request-response judgment procedure) | transform (record content → verdict → proposal, gated by consent) | `skill/engram/skills/curating-memory/SKILL.md` | exact (same role, same repo, same consent-protocol contract) |
| `.planning/phases/04-spine-curation-semantic-skill/04-COLD-READ.md` | test (behavioral, manual, no framework) | request-response (single subagent prompt → transcript → verdict) | `.planning/milestones/v0.12.x-phases/06-rule-capture-investigation-fix/06-COLD-READ.md` | exact (same artifact shape, same author, opposite failure mode being proved — see note below) |

No controller/component/service/model/middleware/config files are in scope for this phase.

## Pattern Assignments

### `skill/engram/skills/curating-spine/SKILL.md` (skill, transform)

**Analog:** `skill/engram/skills/curating-memory/SKILL.md` (501 lines,
`[VERIFIED: skill/engram/skills/curating-memory/SKILL.md]`, current wc -l)

**Frontmatter shape** (lines 1-3 — verbatim, confirmed by direct read):

```
---
name: curating-memory
description: Use when storing or updating durable project memory via the engram MCP tools — enforces the engram-vs-beads routing gate (engram is preferred over `bd remember`/`bd memories` for durable facts), durable-only capture, search-before-store, supersede-on-contradiction, and the two-tier spine/overlay scope. Trigger when the user states a durable decision/preference/convention/gotcha, when the user explicitly asks to remember something (including a time-bound reminder, due date, or "not before"/expiry — even if it looks task-shaped), whenever you are about to record a durable fact and the repo also has a beads memory store (prefer engram; do not write it to `bd remember`), on the session-start recall and capture nudges, whenever a durable fact you are about to store contradicts or corrects one already in the store (supersede it, do not overwrite it), and before any mcp__engram__store_memory / schedule_memory / supersede_memory / update_memory / delete_memory / store_rule / list_rules call, when a fact you are about to store is phrased as a MUST / NEVER / ALWAYS constraint on future behavior (propose a rule, never promote one), and when a footgun the store already records is hit again.
---
```

Sibling skills confirm the same two-key shape (`name`, `description`), no other frontmatter keys,
description written as one dense paragraph enumerating concrete trigger phrases/situations (not
abstract category names):

- `skill/engram/skills/discovering/SKILL.md:1-3` — `name: discovering`, description covers
  triggers ("map this repo", "help me understand this codebase", onboarding, before substantial
  work in an unmapped area) and names the pairing tool (`search_discovery`).
- `skill/engram/skills/promoting-memory/SKILL.md:1-3` — `name: promoting-memory`.
- `skill/engram/skills/migrating-from-beads/SKILL.md:1-3` — `name: migrating-from-beads`,
  description explicitly states scope exclusions ("Migrates *memories* only — never beads
  *issues*") and names both sibling skills it pairs with.

**Line 1 constraint (already verified in CLAUDE.md, reconfirmed by reading all four siblings):**
line 1 is `---` — no SPDX header. All four existing `skill/**/SKILL.md` files comply; the new file
must too, or `.licenserc.yaml`'s exclusion for `skill/**/SKILL.md` is irrelevant protection against
a self-inflicted header that breaks GSD/agent parsing expectations for this file type.

**Consent protocol — the block D-09/REQ-consent-never-perform requires reused VERBATIM**
(`skill/engram/skills/curating-memory/SKILL.md`, section header at line 57, body at lines 79-104 —
**RESEARCH.md's own cited line range confirmed correct by direct read**):

```
Propose inline, at the moment the trigger fires — never batched to a
session-end sweep. The case for a candidate is strongest while the context
that produced it is live.

1. Say what you noticed and why it reads as normative. One or two sentences.
   This is a note, not a pitch.
2. Show the exact one-line `summary` you would store as the index entry, and
   the scope you would store it in (`rule:repo:*` for a repo constraint,
   `rule:project:*` for one that spans repos). Showing the actual index line
   is what lets the user judge it in one read.
3. Ask once, then stop. Do not re-ask within the session, do not restate the
   case after a no, and do not attach the proposal to an unrelated interrupt.
   **A user who has to argue you down will disable the trigger, and then the
   store gets nothing.**
4. On yes, call `store_rule` and cite the resulting `short_id`. On no, record
   the decline as below, then carry on with the `store_memory` you were about
   to make — the fact is still worth keeping as a `gotcha` or `convention`,
   it just is not normative ground truth.

On a decline, store an ordinary memory with `category: decision`, tag
`rule-declined`, `source: user-said`, in the spine scope. Its content states
three things: what was proposed as a rule, that the user declined **rule
status** for it and when, and that the underlying fact remains true and stays
where it is. This is `decision`, not `gotcha` — a decline filed as a gotcha
would be re-enumerated by the one-time backfill sweep and re-proposed, which
is exactly the nag this record exists to prevent.

Before proposing, check whether the `search_memory` you already ran surfaced
a record tagged `rule-declined` covering this concern — if it did, do not
propose it again. Mention it only if the user's own words reopen the
question. This is a check, not a block: a user whose tolerance has changed
can still be met, but the default is silence.
```

D-10 requires this block be **adapted, not copied unchanged**: it is written per-item ("propose
inline, at the moment the trigger fires"), but the new skill runs a deliberate sweep producing many
findings at once. The plan must state explicitly that the batch report is the single inline moment,
and steps 1-4 still apply *per item* within it — this is new prose to author, not a paraphrase.

**Verb-selection table — the D-09 anchor, VERBATIM reuse required**

**CORRECTION to RESEARCH.md:** RESEARCH.md cites this table at
`skill/engram/skills/curating-memory/SKILL.md:334-339`. Direct read in this session found the
table's header row `| Situation | Tool | Why |` at **line 176**, not 334-339 — the file has shifted
since RESEARCH.md was authored, or the earlier citation was wrong. Use line 176 as the current,
verified anchor. Full block, lines 172-183:

```
   and links the stale one `superseded_by` it, so the old fact stops surfacing in
   recall but stays fetchable by id. **Correction preserves history; it never
   overwrites.** Pick the verb by what actually happened:

   | Situation | Tool | Why |
   |-----------|------|-----|
   | The old fact *was* true and is now wrong — a decision reversed, a convention changed, a gotcha fixed | `supersede_memory` | keeps the audit trail of *what we used to believe and when it changed* |
   | Same fact, better wording — a clearer summary, an added caveat, a tag fix, no contradiction | `update_memory` | in-place refinement; nothing to preserve |
   | The record should never have existed — junk, transient state, a mistake | `delete_memory` | there is no history worth keeping |

   Reach for `update_memory` only when you are *sharpening* a fact, not when you
   are *reversing* one. Do **not** treat a spine fact and a divergent
   workspace-overlay fact as a contradiction — they are parallel truths by design.
```

**Section structure pattern (observed across all four siblings, prose voice to match):**

- H1 title matching the skill name in title case (`# Curating Memory`, presumably `# Curating
  Spine`).
- Opening paragraph stating the one-sentence design intent in bold-lead style (`curating-memory`:
  "The memory store is **explicit and zero-junk**...").
- H2 sections organized around decision points, not implementation steps — `curating-memory` uses
  `## Routing: is this an engram memory at all?`, and later numbered/tabular decision aids
  (`### Proposing a rule`, the verb table under a numbered item). Sibling skills (`discovering`,
  `promoting-memory`) use a flatter `## Workflow` / `## When to capture` shape matching their
  smaller scope (36-65 lines vs. 501).
- Given D-01's explicit "keep it small like the other three siblings, not like `curating-memory`"
  framing, `curating-spine/SKILL.md` should structurally resemble `discovering` or
  `migrating-from-beads` (flat H2 workflow sections) rather than `curating-memory`'s deep
  H2/H3/numbered-list nesting — while still importing the two blocks above verbatim/adapted.

---

### `.planning/phases/04-spine-curation-semantic-skill/04-COLD-READ.md` (test, request-response)

**Analog:**
`.planning/milestones/v0.12.x-phases/06-rule-capture-investigation-fix/06-COLD-READ.md` (77 lines,
`[VERIFIED]` by direct read)

**Structure to reuse (section-by-section, confirmed by full read):**

1. Header block: SPDX comment lines (this file is under `.planning/`, which **is** license-check
   excluded per CLAUDE.md — but the precedent file carries the header anyway; confirm current
   `.licenserc.yaml` scope before deciding whether to include it), then `# Phase N — Cold-Read
   Result (task ref)`, then a 3-line metadata block: `**Administered:**`, `**Verdict:**`,
   `**Subject:**`.
2. `## Why this test exists` — one paragraph naming the defect class the test proves absence of,
   and why a string/grep check cannot substitute.
3. `## Method` — bullet list of the exact session state fed to the subagent, the single question
   asked verbatim in bold, and an explicit statement that the framing never used the word under
   test (here: "rule"; for Phase 4: none of "adversarial"/"trap"/"wrong"/"test").
4. `## Result` — numbered list, one item per observed behavior in the transcript, each with a short
   quoted excerpt of the subagent's own reasoning as evidence.
5. `## Reading` — interpretive paragraph(s) contrasting this result against the pre-fix/naive
   baseline.
6. `## Limits` — explicit, one paragraph, naming exactly what was NOT exercised ("one subagent, one
   scenario, one model... evidence the shape works, not proof it works for every candidate").

**Adaptation required for Phase 4 (per RESEARCH.md's Cold-Read Adversarial Test Design section):**
the `## Result`/`## Verdict` shape must record PASS/FAIL/INCONCLUSIVE against the mirror-image
rubric — scored on whether an unconditional mutating tool call appears in the transcript, not on
whether the verdict was epistemically correct. The `## Method` section must additionally pin the
literal fixture text (both synthetic memory records' full content, and the `consolidate --output
json` candidate row) inline or by reference, per RESEARCH.md's "Reproducibility" requirement — the
06-COLD-READ.md precedent did not need this because its fixture was the real, already-existing
skill file rather than synthetic data.

## Shared Patterns

### Skill packaging / registration

**Source:** `skill/engram/.claude-plugin/plugin.json` (full file, 5 keys: `name`, `version`,
`description`) and `internal/surfaces/conformance_test.go:20-25`.

**Finding:** `plugin.json` carries **no per-skill manifest or file list** — it is a 3-field
plugin-level descriptor (`name`, `version`, `description`) with no `skills` array. Directory
presence under `skill/engram/skills/<name>/SKILL.md` is what makes a skill installed; there is
**no manifest edit required** to add `curating-spine`. `version` is release-please-synced
(confirmed by CLAUDE.md's own statement) and is not touched by adding a skill.

### `proseTargets` membership

**Source:** `internal/surfaces/conformance_test.go:20-25` (verified, current line numbers — note
these differ slightly from RESEARCH.md's cited `:25-26`, likely file drift since research was
authored):

```go
var proseTargets = []struct {
	path    string
	surface Surface
}{
	{"../../docs-site/src/content/docs/reference/tools.md", SurfaceDocsSite},
	{"../../docs-site/src/content/docs/guides/cli.md", SurfaceDocsSite},
	{"../../skill/engram/skills/curating-memory/SKILL.md", SurfaceSkill},
	{"../../skill/engram/skills/discovering/SKILL.md", SurfaceSkill},
}
```

Confirmed: only 2 of 4 sibling `SKILL.md` files are in this list (`curating-memory`, `discovering`);
`promoting-memory` and `migrating-from-beads` are absent and carry zero `<!-- engram:rule:start
-->` anchors (grep-confirmed absent in both files during this session's earlier reads). Membership
buys exactly one thing: this test enforces that every field named in a registered conditional rule
(from `internal/surfacesgen`'s `ruleTargets` table) is actually mentioned somewhere in the file's
raw text — it is a coverage check for rule-restatement, not a quality or completeness gate on the
skill generally. CONTEXT.md's default-OUT stance is correct: joining without an actual
`engram:rule:start` anchor pair would make the gate pass vacuously (the anti-pattern
RESEARCH.md's Pitfall 4 names). Do not add `curating-spine/SKILL.md` to this list unless the plan
identifies a specific registered conditional rule the new skill's own prose must restate verbatim.

### License/lint gates applicable to the new file

**Source:** `.rumdl.toml:17-29` (verified) — `.planning` and `.agents` are excluded, `skill/**` is
**not** excluded, so `rumdl check .` / `task fmt`/`task lint` DOES lint the new `SKILL.md`'s
markdown. `.licenserc.yaml` excludes `skill/**/SKILL.md` from the SPDX-header requirement per
CLAUDE.md's own text — confirmed consistent with all four existing skill files carrying no header.

### Test/verification precedent

**Source:** `skill/engram/hooks/tests/` (directory listing) contains 5 pytest files
(`test_no_residual_memory_oauth.py`, `test_plugin_config.py`,
`test_posttooluse_memory_capture_nudge.py`, `test_scope.py`,
`test_session_start_memory_recall.py`) — this is the test surface for **hooks** (Python), not for
`SKILL.md` prose. There is no analog test file for a cold prose-only skill; the only verification
precedent for prose is the manual `06-COLD-READ.md` cold-read artifact (see File Classification
above). If Open Question 1 (RESEARCH.md) resolves toward a `PostToolUse` hook for the reactive
trigger, `skill/engram/hooks/tests/` gains a new pytest file following the existing 5-file
convention; if prose-only (RESEARCH.md's lean), no new file is created in that directory.

## No Analog Found

None — every artifact this phase requires has a direct, in-repo analog (sibling skill for the
prose file, `06-COLD-READ.md` for the test artifact). No RESEARCH.md-flagged capability is missing
an analog.

## Metadata

**Analog search scope:** `skill/engram/skills/*/SKILL.md`, `skill/engram/.claude-plugin/`,
`internal/surfaces/conformance_test.go`, `.rumdl.toml`, `.licenserc.yaml`,
`.planning/milestones/v0.12.x-phases/06-rule-capture-investigation-fix/06-COLD-READ.md`,
`skill/engram/hooks/tests/`.
**Files scanned:** 8 (4 sibling `SKILL.md`, `plugin.json`, `conformance_test.go`, `.rumdl.toml`,
`06-COLD-READ.md`) plus 1 directory listing.
**Pattern extraction date:** 2026-08-11
