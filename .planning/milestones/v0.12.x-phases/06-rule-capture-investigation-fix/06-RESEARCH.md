<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright 2026 Sean Brandt -->

# Phase 6: Rule Capture — Investigation & Fix - Research

**Researched:** 2026-08-01
**Domain:** Claude Code skill/hook prose engineering (agent-behavior shaping), Go tool-validation surface (read-only confirmation)
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01 (the instruction to propose already exists and is buried inside its own prohibition):**
  `skill/engram/skills/curating-memory/SKILL.md:51-53` reads *"Store a rule **only on explicit user
  instruction**: if you believe something should be a rule, propose it to the user and let them
  bless it — never promote one unilaterally."* The bolded topic clause is a restriction and the
  closing clause is another restriction; the permission to propose sits unbolded between them. The
  net reading is "don't," not "notice and offer."
- **D-02 (the permission is conditioned on a state nothing in the skill produces):** *"if you
  believe something should be a rule"* has no trigger behind it. This is the root cause and it is
  a friction cause, not a mechanical one.
- **D-03 (the symptom is reproducible from the store, not only from session transcripts):** three
  records filed as `category: gotcha` are phrased as normative constraints and were never proposed
  as rules — `r3bjakymtz`, `z4mgz3a4ab`, `478rhhmhb0`.
- **D-04 (suggesting is not promoting, so the invariant is untouched):** the user-blessed gate
  governs who **decides**, not who **notices**. Any intervention that stores a rule without
  explicit user instruction is out of scope — not a trade-off to be balanced.
- **D-05 (two independent triggers, either one fires a proposal):** repeat-hit on a footgun
  (search-before-store already surfaces it), and normative phrasing at capture time (MUST/NEVER/
  ALWAYS about to be stored).
- **D-06 (the proposal is made inline, at the moment the trigger fires):** not batched to a
  session-end sweep.
- **D-07 (a declined proposal must not re-fire indefinitely):** repeated declining is not the
  user's only defense — the design must state what happens on decline.
- **D-08 (the phase ships a rule-curation discipline, not only a capture trigger):** duplicates,
  contradictions, and rot.
- **D-09 (correction is delete-then-re-store, because rules cannot be superseded):**
  `SKILL.md:208` — `set_visibility` is rejected for rules.
- **D-10 (rule deletion is user-blessed, symmetrically with creation):** the agent proposes,
  the user decides.
- **D-11 (contradiction and dedup checks cost a fetch, and the design must price that):** session
  start loads the rules index only; full text requires `get_memory` per rule.
- **D-12 (a one-time sweep proposes the already-stored gotchas that read as rules):** surface the
  existing normatively-phrased gotchas so each can be blessed or declined; doubles as the
  criterion-3 demonstration.

### Claude's Discretion

- Whether the trigger and the curation discipline live in `curating-memory/SKILL.md`, a sibling
  skill, or a split across both.
- The decline-memory mechanism for D-07.
- The exact wording of the normative-phrasing detector (which keywords, how much context).
- Whether the D-12 sweep is a documented procedure an agent follows or a one-off operator command.

### Deferred Ideas (OUT OF SCOPE)

- **Any automatic rule promotion.** Permanently out of scope, not deferred — D-04.
- **Telemetry on proposal accept/decline rates.** Needs a data path this phase does not have.
- **Rule scoping beyond `rule:repo:*` / `rule:project:*`.** Not in scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-rule-capture-investigation | Determine why an agent never proposes a rule; deliverable is a written root cause distinguishing mechanical vs. friction cause. | Already satisfied by 06-CONTEXT.md D-01/D-02 (friction cause, evidenced by D-03). This research does not re-derive it — see Summary. |
| REQ-rule-capture-intervention | Apply the fix without changing who decides — suggesting is not promoting. | Architectural Responsibility Map, Discretion #1 (where the trigger/protocol live), Discretion #2 (D-07 decline mechanism), Pitfalls 1–2, Code Examples (`deleteMemory` has no rule guard, confirming the correction path stays open). |
| REQ-rule-curation-hygiene | A curation discipline covering duplicate, contradictory, and rotted rules — pricing the `get_memory`-per-rule fetch cost and the delete-then-re-store correction shape. | Don't Hand-Roll (`ruleThreshold` advisory), Pitfalls 3–4 (advisory insufficiency, index-vs-content cost), Discretion #3 (D-12 backfill shape), Code Examples (curation-smell advisory, no-supersede line). |

</phase_requirements>

## Summary

Root cause was settled in discuss (06-CONTEXT.md D-01/D-02), so this research does not
re-investigate it. Its job was to make the *intervention* plannable: read every surface named in
06-CONTEXT.md's Integration Points exhaustively, confirm the mechanical facts the design leans on
(`delete_memory` really is unblocked for rules; `list_rules` already has a curation-smell signal;
no existing skill implements a decline-memory pattern to copy), and price the open discretion
questions with evidence instead of guesswork.

The whole phase is prose engineering over two files that already exist and already say almost the
right thing: `curating-memory/SKILL.md:51-53` and `docs-site/reference/tools.md:353-355` both
contain the identical buried-permission bug ("propose it to the user instead" with no trigger
behind it) — the fix pattern is one, not two independent rewrites. `promoting-memory/SKILL.md`
turned out **not** to be a notice-then-propose analog — it's an unconditional per-record decision
loop with no "should I even ask" gate. No sibling skill implements the "notice a condition → offer
→ act only on consent" shape D-05/D-06 need; the planner is authoring a genuinely new pattern in
this codebase, not adapting one. `rules.go` already carries a mechanical curation-smell advisory
(`ruleThreshold = 50`) that is the right template for a *cheap, no-fetch* volume signal, though its
threshold is far above the phase's actual working scale (3 rules today). `delete_memory` (unlike
`set_visibility` and `supersede_memory`) has **no** rule-category guard — D-09/D-10's premise that
delete is the correction path for rules is confirmed, not assumed.

**Primary recommendation:** land the trigger, proposal protocol, and curation discipline as new
subsections inside `curating-memory/SKILL.md`'s existing `## Rules` section (not a new sibling
skill) — rewrite `:51-53`'s single sentence into a short trigger table plus explicit proposal
protocol, matching the file's own established `##`/table/discipline-numbered-list house style —
and mirror the corrected instruction into `docs-site/reference/tools.md`'s `store_rule` prose
(same bug, same fix, second surface). Use a **plain engram memory** (not new machinery) to record a
declined proposal for D-07. Make the D-12 backfill an agent-followed documented procedure, not a Go
CLI command — the judgment call ("is this gotcha phrased normatively?") is exactly the kind of
semantic read a deterministic sweep cannot make, unlike every existing one-time-command precedent
(`migrate-remap-owner`, `backfill-short-ids`, `prune-expired`, `summarize-missing`), which are all
mechanical field comparisons.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Recognition trigger (repeat-hit / normative phrasing) | Agent (skill prose) | — | No server-side signal exists or is being added; the agent reads its own conversation/tool-call context to notice the trigger. This is pure prompt-engineering, not code. |
| Inline proposal protocol | Agent (skill prose) | — | The proposal is a conversational act — the agent asks, the user answers. Nothing to build server-side. |
| Decline-memory (D-07) | Agent (skill prose) + existing `store_memory`/`search_memory` tools | Database (Qdrant, unchanged) | Reuses the already-shipped memory-write path; no new tool, no new field. The "state across sessions" the agent needs already exists as an ordinary memory record. |
| Curation discipline (dup/contradiction/rot) | Agent (skill prose) | Server (existing `list_rules` curation-smell advisory) | The advisory is a cheap, already-shipped volume signal computed server-side on every `list_rules` call; the agent decides when to pay for a full-text check. No new server code is required for the phase's success criteria. |
| Rule deletion (correction path, D-09/D-10) | Server (`internal/server/tools.go` `deleteMemory`) | Agent (proposes, never calls unasked) | `deleteMemory` is generic and already unblocked for rules (confirmed this session, no `errRuleImmutable` guard on that path) — no code change needed here, only the agent-facing instruction that the delete must be user-blessed. |
| D-12 backfill sweep | Agent (documented procedure, run once) | — | Judging "is this gotcha phrased normatively enough to be a rule candidate" is semantic, not a field comparison — unlike every existing operator CLI sweep (`migrate-remap-owner`, `backfill-short-ids`, `prune-expired`, `summarize-missing`), which all resolve on structural/timestamp predicates a Go loop can execute unaided. |

## Package Legitimacy Audit

Not applicable — this phase installs no packages (zero new Go dependencies; no new npm/pip/cargo
surface). Skipped per the milestone-wide constraint already declared in `REQUIREMENTS.md`'s "New
Go dependencies" out-of-scope row.

## Architecture Patterns

### System Architecture Diagram

```
Agent session start
        │
        ▼
session-start-memory-recall hook (Python, SessionStart)
        │  emits additionalContext: rules-index instruction
        ▼
Agent calls mcp__engram__list_rules(scopes=[rule:repo:..., rule:project:...])
        │  (compact index only — no full text, no per-rule fetch)
        ▼
Agent renders Rules index section, holds it in context for the session
        │
        ▼
── during the session, a fact-worth-capturing moment occurs ──
        │
        ├─ Trigger A: search_memory (search-before-store step) surfaces
        │  an existing gotcha matching the current problem  ─┐
        │                                                      │
        ├─ Trigger B: a fact about to be stored is phrased    ├──▶ Proposal
        │  MUST / NEVER / ALWAYS at capture time              │    protocol
        │                                                      │    (inline,
        └─ (curation) rule-count crosses list_rules'          ─┘    at the
           50-rule curation-smell advisory, OR the agent            moment the
           notices two rules read as duplicate/contradictory        trigger
           while reading the index                                  fires)
                                                                     │
                                                                     ▼
                                                        User accepts / declines
                                                          │            │
                                                     accepts        declines
                                                          │            │
                                                          ▼            ▼
                                              agent calls        agent stores a
                                              store_rule /       plain engram
                                              delete_memory      memory recording
                                              (user-blessed,     the decline
                                              explicit consent)  (D-07 — suppresses
                                                                  re-firing next
                                                                  session's search-
                                                                  before-store hit)
```

### Recommended Project Structure

No new files are structurally required. The phase's surfaces are:

```
skill/engram/skills/curating-memory/SKILL.md   # primary edit: ## Rules section expanded
docs-site/src/content/docs/reference/tools.md  # secondary edit: store_rule prose, same bug
skill/engram/hooks/session-start-memory-recall # unchanged unless D-11's session-scoped hint is added
```

A new sibling skill is available as a fallback (`skill/engram/skills/curating-rules/SKILL.md`) if
the planner determines the trigger+protocol+discipline additions would push `curating-memory` past
a reasonable single-file size, but the research below recommends against it (see Discretion #1).

### Pattern: House style of `curating-memory/SKILL.md` (read in full, 277 lines)

**Structure, top to bottom:**
1. YAML frontmatter — `name` + one dense `description` paragraph that is itself the trigger
   surface Claude Code uses to load the skill (not the body text).
2. `# Curating Memory` — one-sentence framing ("explicit and zero-junk").
3. `## Routing: is this an engram memory at all?` — a bulleted decision list, each bullet
   **bold-lead** phrase → arrow → destination, ending in an explicit "ask, never silently pick
   one" escape hatch for the ambiguous case.
4. `## Junk taxonomy` — two-line **STORE**/**DO NOT STORE** bolded contrast, no elaboration.
5. `## Rules (user-blessed ground truth)` — the target section, `:47-56`, quoted verbatim below.
6. `## Tagging`, `## Cross-spine recall` (with a `### When not to use cross-spine` subsection —
   this is the closest existing precedent for a "when NOT to" subsection pattern), `## Reading a
   rejection` (a retry-pattern bulleted list keyed on `hint=` values).
7. `## Discipline` — numbered list (1-4), each item **bold lead phrase.** then prose, item 2
   carries an embedded decision table (Situation | Tool | Why).
8. `## Scheduling`, `## Supersession` (bulleted "Rules that will bite you if ignored" — a second
   precedent for a "guardrail list" pattern), `## Citations`, `## Summaries`, `## Tools and auth`.

**Imperative phrasing conventions observed:** Directives are stated as bare imperatives ("Search
before store.", "Set `source` honestly."), never hedged ("you should probably..."). Constraints
that carry real teeth are **bolded inline**, not just headed. Every constraint that has an
exception states the exception in the same sentence, not a footnote. Tables are used for anything
with more than two dimensions (Situation/Tool/Why; hint code/retry pattern); bulleted lists for
linear guardrail sequences.

**Rules section, quoted verbatim (`:47-56`):**

```
## Rules (user-blessed ground truth)

A **rule** is normative ground truth for the repo/project — a MUST-follow
constraint, always shared, stored via `store_rule` in a `rule:repo:*` /
`rule:project:*` scope. Store a rule **only on explicit user instruction**: if
you believe something should be a rule, propose it to the user and let them
bless it — never promote one unilaterally. A rule's `summary` is a single line
(the session-start index entry). This complements — it does not replace — the
decision / preference / convention / gotcha routing above; a rule is the
narrower, user-blessed, normative case.
```

**No-supersede line, quoted verbatim (`:208`):**

```
- **Rules can't be superseded.** `store_rule` records are normative ground truth;
  delete the rule instead (same restriction as `set_visibility`).
```

This confirms D-09's premise directly from the skill's own text — the planner does not need to
re-derive it.

**`## Discipline` section, quoted verbatim (`:127-159`) — the closest structural template for a
new numbered-discipline addition, since D-08's curation hygiene is functionally a discipline
list:**

```
## Discipline

1. **Search before store.** Call `mcp__engram__search_memory` across both
   the spine and (if present) the workspace overlay first. `search_memory` is
   backed by a semantic/vector engine, so query it with a natural-language
   description of the fact (not keyword fragments) — it surfaces conceptually
   related records even when they share no exact wording. If a near-duplicate
   exists, update it instead of adding a new record.
2. **Supersede on contradiction — within a tier.** When new info *conflicts with*
   an existing memory, call `supersede_memory` — ...
   [full decision table omitted here; see file]
3. **Tier selection.** Default to the **spine** ...
4. **Provenance.** Set `source` honestly (`user-said` vs `agent-inferred`). Do
   not set `actor` — it is assigned server-side from the validated OAuth token.
```

### Pattern: Sibling skills — none implement a notice→propose→consent protocol

- **`promoting-memory/SKILL.md`** (36 lines) — the most likely candidate, checked carefully. It is
  an unconditional 4-step workflow triggered by an explicit user command ("promote memories",
  "merge workspace memories", or finishing/merging a branch) — **not** a recognition trigger over
  ambient conversation. Step 3 is per-record ("For each overlay memory, decide with the user:
  Promote / Keep / Drop") but the decision loop only starts after the user has already invoked the
  skill; there is no "the agent notices an overlay memory looks promotable and interrupts to ask"
  behavior. **It is not the analog.** Its useful precedent is narrower: the three-way disposition
  table shape (Promote/Keep/Drop, each with a one-line rationale) is a reusable template for D-08's
  curation-review shape (Keep/Merge-duplicate/Flag-contradiction/Retire-rotted), and its closing
  line — "Keep the spine zero-junk: promote only facts that are genuinely durable and repo-wide,
  applying the same junk taxonomy as `curating-memory`" — is the established idiom for one skill
  deferring to another's taxonomy rather than restating it, which the new trigger/protocol
  addition should do symmetrically (defer to `curating-memory`'s own Rules section, not restate
  it, if the discretion call lands on a sibling skill).
- **`discovering/SKILL.md`** (64 lines) — search-before-store precedent, `kind: map vs fact`
  branching, mandatory-citations discipline. No propose-then-consent shape; discoveries are
  captured unilaterally by design (no user-blessed gate — discoveries are agent-earned
  understanding, not normative ground truth, so the two are not analogous by design).
- **`migrating-from-beads/SKILL.md`** (62 lines) — closest in *spirit* to a decision loop ("For
  each beads memory, decide with the user: Migrate / Drop") but again gated behind an explicit,
  named user invocation ("migrate beads memories to engram"), not an ambient recognition trigger.
  Its five-step workflow (confirm both stores exist → enumerate → per-item decide → complete the
  move only after the write succeeds → report a summary) is a good template for **sequencing** a
  D-12 backfill sweep, since D-12 is also a bounded, run-once, per-item review loop.

**Conclusion:** D-05/D-06's "notice mid-session, propose inline" shape has no existing analog to
copy verbatim. The planner is authoring new prose, informed by the *tabular disposition* idiom from
`promoting-memory` and the *five-step bounded sweep* idiom from `migrating-from-beads`, not
adapting either wholesale.

### Anti-Patterns to Avoid

- **Restating the junk taxonomy or routing table instead of cross-referencing it.** Every sibling
  skill defers to `curating-memory`'s taxonomy by name rather than repeating it (see
  `promoting-memory`'s and `migrating-from-beads`'s closing lines above). A new trigger/discipline
  addition should do the same for anything not specific to rules.
- **A trigger phrased as a soft suggestion.** The file's existing imperative style ("Search before
  store.", never "you might want to search first") is load-bearing — D-01/D-02's root cause is
  literally that the current permission clause reads as optional. A reworded trigger that is
  itself hedgeable reproduces the bug it's fixing.
- **A decline mechanism that requires new server machinery.** D-07 is answerable entirely with the
  existing `store_memory`/`search_memory` tools (see Discretion #2 below); anything that proposes a
  new field, a new category, or a new tool violates the zero-new-Go-dependency /
  minimal-surface-change posture this milestone has held for five straight phases.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cross-session "don't re-nag" state | A new hook-local marker file (`posttooluse-memory-capture-nudge`'s pattern), a new tool argument, or a new rule field | An ordinary `store_memory` record (category `gotcha` or a new low-ceremony convention, tagged e.g. `declined-rule-proposal`) that `search_memory`'s existing search-before-store step naturally surfaces next time the same footgun is hit | The marker-file pattern is process-scoped and per-session by design (`tempfile.gettempdir()` keyed on `session_id`) — it cannot answer "was this already declined two sessions ago." A memory record is already durable, already searched before every store, and needs zero new machinery. |
| Curation cost gating (D-11) | A new server endpoint that returns rule contradiction pairs, or an agent habit of calling `get_memory` on every rule at session start | The already-shipped `list_rules` curation-smell advisory (`ruleThreshold = 50`, `rules.go:165-168,216-217`) as the volume trigger, plus semantic `search_memory`/reading-the-index-by-eye for the small-N case | Building a contradiction-detector is out of scope for this phase (no ML/embedding-diff infrastructure exists for rule-vs-rule comparison) and unnecessary at N=3 — the cheap signal already exists in code, just not yet surfaced or referenced by the skill prose. |
| D-12 backfill sweep automation | A new `engram backfill-rule-candidates` CLI command mirroring `migrate-remap-owner`/`prune-expired` | A documented agent procedure the user runs interactively once | Every existing one-time sweep command resolves on a **structural** predicate a deterministic Go loop can execute (owner field equality, timestamp comparison, missing short_id). "Is this gotcha phrased normatively enough to read as a rule" is a semantic judgment; hand-rolling it in Go would mean either a crude keyword regex (MUST/NEVER/ALWAYS — cheap, but already what D-05's inline trigger does at write time, so a backfill CLI would just be a batch mode of the same heuristic) or embedding a call to an LLM from a CLI command, which is new infrastructure this phase doesn't need. |

**Key insight:** every mechanism this phase needs already exists on the server (rule storage,
delete, the curation-smell advisory) or in the memory contract (search-before-store, tags). The
entire implementation surface is *when and how the agent decides to invoke tools that already
exist* — which is why it is a prose-engineering phase, not a Go-correctness phase, exactly as
ROADMAP.md's Phase 6 note states.

## Common Pitfalls

### Pitfall 1: Fixing the wording without fixing the burial
**What goes wrong:** rewording `:51-53`'s sentence to be more forceful ("MUST propose") without
restructuring it out of its enclosing prohibition clause reproduces D-01's exact defect in new
words — a permission clause sandwiched between two restrictions still reads as a restriction on
skim.
**Why it happens:** the natural edit is "strengthen the verb," not "restructure the sentence,"
because the content (propose, don't promote) is already correct.
**How to avoid:** give the trigger and the proposal protocol their **own** subsection or their own
sentence boundary, separate from the "never promote unilaterally" prohibition — structurally, not
just typographically, separate the permission from the restriction, mirroring how `## Discipline`
gives each numbered rule its own bold-lead clause.
**Warning signs:** if the new text still reads, on a single skim pass, primarily as a list of things
NOT to do, the burial has been reproduced.

### Pitfall 2: Making the trigger fire on every MUST/NEVER/ALWAYS utterance
**What goes wrong:** D-05's second trigger ("normative phrasing at capture time") is scoped to
facts *about to be stored as memories* — it is not "the agent used the word MUST in conversation."
An overly broad keyword match interrupts constantly (e.g. quoting someone else's MUST-phrased
requirement, or restating a REQ-* line from REQUIREMENTS.md) and becomes exactly the nag D-07
exists to prevent, except now for a false-positive class rather than a repeat-decline class.
**Why it happens:** MUST/NEVER/ALWAYS is a simple, greppable signal, tempting to apply unscoped.
**How to avoid:** scope the trigger check to the moment a `store_memory`/`schedule_memory` call is
about to be made with `category: gotcha` (or a normatively-phrased `convention`) — i.e., hook the
question onto the existing capture decision, not free-floating text.
**Warning signs:** the trigger fires on quoted text, on REQUIREMENTS.md excerpts, or on the agent's
own planning prose rather than on content being written to the store.

### Pitfall 3: Treating `list_rules`' curation-smell advisory as sufficient for D-08
**What goes wrong:** the 50-rule threshold is a volume smell only — it says nothing about
duplication or contradiction, and at the phase's actual current scale (3 rules) it will never fire.
Shipping D-08 as "rely on the existing advisory" leaves duplicate/contradiction detection entirely
unaddressed, failing the roadmap's criterion 5.
**Why it happens:** the advisory already exists in code and is easy to point to as "already
handled."
**How to avoid:** treat the advisory as one input (a cheap "the set has grown, worth a look" nudge)
alongside an explicit read-the-index-and-compare-by-eye discipline for dup/contradiction, priced
per D-11 (see below) — not a substitute for it.
**Warning signs:** the plan's curation section cites only `ruleThreshold` and has no separate
mechanism for catching two contradictory 3-rule-set entries, which the threshold cannot detect at
any count below 51.

### Pitfall 4: Pricing D-11's cost wrong by assuming the index carries enough for contradiction detection
**What goes wrong:** `list_rules`' compact `ruleView` carries only `short_id`, `id`, `summary`
(<=256 bytes), `tags`, `scope`, `created_at` — **not** `content`. A summary-only comparison can
catch obvious duplicates ("two summaries about git worktree hooks") but cannot reliably catch
*contradiction*, since two rules can have similar summaries but opposite content, or dissimilar
summaries and genuinely conflicting content. Assuming the index alone is enough to judge
contradiction understates the fetch cost D-11 asks to be priced honestly.
**Why it happens:** the index is what's already in context every session, so it's the path of
least resistance.
**How to avoid:** state explicitly in the discipline that a *real* contradiction check requires
`get_memory` per candidate pair (full content), and gate that cost behind a trigger (the volume
advisory, or the agent noticing two summaries look related while reading the index) rather than
running it unconditionally every session.
**Warning signs:** the plan claims contradiction detection "from the session-start index" with no
mention of a `get_memory` fetch anywhere in the discipline.

## Code Examples

### The curation-smell advisory (already shipped, `internal/server/rules.go:165-168,216-217,227-229`)
```go
// ruleThreshold is the soft rule-count ceiling per scope above which listRules
// returns a curation-smell advisory (textResult only; the {rules} payload is
// unaffected). A rule set is definitionally small.
const ruleThreshold = 50

// ... inside listRules, per scope:
if len(ms) > ruleThreshold {
    over = append(over, fmt.Sprintf("%d rules in %s", len(ms), sc))
}
// ... after the scope loop:
if len(over) > 0 {
    advisory = "curation smell — " + strings.Join(over, "; ") + " — consider consolidating"
}
```
This is a real, tested (`internal/server/rules_test.go:550-587`,
`TestListRulesHandlerCurationAdvisory`), already-shipped mechanism the curation discipline can cite
by name rather than re-derive.

### `delete_memory` has no rule-category guard (confirmed this session, `internal/server/tools.go:1587-1604`)
```go
// deleteMemory deletes one record by id or short id. Same no-leak re-wrap as
// getMemory: the Delete gate's not-found echoes only the caller's input.
func (d *deps) deleteMemory(ctx context.Context, c caller, a idArgs) error {
    if err := requireID(a.ID); err != nil {
        return err
    }
    pid, err := d.st.ResolvePointID(ctx, a.ID)
    if err != nil {
        return err
    }
    if err := d.st.Delete(ctx, pid, c.Subj); err != nil {
        if errors.Is(err, store.ErrNotFound) {
            return fmt.Errorf("%w: %s", store.ErrNotFound, a.ID)
        }
        return err
    }
    return nil
}
```
Contrast with `setVisibility` (`tools.go:1623-1665`) and `supersedeMemory` (`tools.go:~1667-1725`),
both of which read the record's `Category` and reject with `errRuleImmutable` when it is `"rule"`.
`deleteMemory` has no equivalent read-and-reject — it is a bare `ResolvePointID` → `Delete`. **This
confirms D-09/D-10's premise directly**: delete is genuinely available as the rule correction path,
unlike visibility-change and supersession, which are structurally blocked.

### Existing house-style "when NOT to" subsection precedent (`curating-memory/SKILL.md:88-103`)
```markdown
### When not to use cross-spine

Cross-spine is an opt-in widening, and the failure mode of an opt-in widening
is setting it on every call. Don't. The default is scope-confined, and it
should stay that way for ordinary work: ...
```
Reusable template for a "when NOT to propose" caution the trigger addition may want, symmetric with
D-07's anti-nag concern.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `store_rule` reachable only via explicit user command ("add a rule that...") | Same tool, but the phase adds an agent-initiated *proposal* path that still terminates in explicit user consent | This phase (not yet shipped) | The user-blessed gate (D-04) is unchanged; only the *initiative* to ask changes hands from user to agent. |
| No documented decline-persistence mechanism | A plain `store_memory` record recording the decline (recommended, not yet a locked decision — see Discretion #2) | This phase | Reuses existing infra; no schema change. |

**Deprecated/outdated:** none — this phase adds to a stable contract, it does not deprecate
anything. `docs-site/reference/tools.md:353-355`'s `store_rule` prose carries the identical
buried-permission defect as the SKILL.md and should be corrected in the same commit/plan, not left
stale (it would otherwise contradict the corrected skill prose the moment this phase ships).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Rule `7smp8vy9hr` establishes a milestone-completion memory-curation checkpoint (cited in 06-CONTEXT.md's Established Patterns, not independently re-read this session — no engram MCP tool access in this research session, and the rule's content lives only in the live Qdrant store, not in any repo file) | Discretion #3 / D-08 hooking-point discussion | If the rule's actual text does not describe a milestone-completion cadence as CONTEXT.md characterizes it, the "hook into an existing cadence rather than inventing a new one" recommendation loses its anchor. The planner should `get_memory(7smp8vy9hr)` directly before committing to this as the curation cadence. |
| A2 | The current live rule count in `rule:repo:github.com/seanb4t/engram` is 3 (`7smp8vy9hr`, `rvmts69cz1`, `0v4249kc9d`) | D-11 cost pricing, Package/scope sizing throughout | Sourced from 06-CONTEXT.md (which itself states this was established during discuss, presumably via a live `list_rules` call at that time). Not independently re-queried this session (no engram MCP tool access). If the count has since changed (e.g., a rule added mid-milestone), the "cheap at 3, not at 30" framing in D-11 still holds directionally but the exact current number should be re-confirmed with `list_rules` at plan or execution time. |

**Both assumptions are sourced from 06-CONTEXT.md, itself a product of the discuss session — they
are not fabricated, but this research session could not independently re-verify them against the
live engram store** (the tool list available to this research agent did not include
`mcp__engram__*`). The planner or executor, which will have live MCP access, should treat A1/A2 as
CITED-from-discuss rather than VERIFIED-this-session, and re-confirm A2 trivially via `list_rules`
before writing any cost claim into shipped documentation.

## Open Questions

None blocking. The three Claude's-Discretion items from 06-CONTEXT.md are answered with a
recommendation below (not decisions — still open for the planner/user to confirm), per the
`<open_design_questions_to_inform>` scope of this research.

### Discretion #1 — Where does the trigger/proposal/curation content live?

**What we know:** `curating-memory/SKILL.md` is 277 lines, is loaded on every capture (its
`description` frontmatter is the trigger surface Claude Code scans on essentially every durable-fact
moment), and already owns the Rules section this phase extends. No sibling skill implements the
notice→propose→consent shape, so nothing to graft onto elsewhere. A new sibling skill
(`curating-rules`) would need its **own** `description` frontmatter trigger, which duplicates the
"about to store/notice a rule-shaped fact" surface `curating-memory`'s description already covers
in part ("whenever you are about to record a durable fact") — a second skill risks the file failing
to load at exactly the moment (mid-capture-decision) the trigger needs to fire, unless its
description is written with equal precision.

**Recommendation:** extend `curating-memory/SKILL.md`'s existing `## Rules` section in place —
restructure `:51-53` into the fixed permission-not-buried sentence, then add two new subsections
directly beneath it: `### Proposing a rule` (D-05/D-06's trigger + inline protocol) and
`### Rule hygiene` (D-08's curation discipline, cross-referencing `list_rules`' curation-smell
advisory and pricing the D-11 fetch cost). This keeps the file's existing structural
pattern — `## Rules` already sits between `## Junk taxonomy` and `## Tagging`, and its current 10
lines can absorb two more focused subsections without materially changing the file's shape (the
file already has multi-level subsections under `## Cross-spine recall`). Mirror the corrected
permission sentence into `docs-site/reference/tools.md`'s `store_rule` block (`:353-355`) in the
same plan/commit, since it carries the identical bug. Only fall back to a sibling skill if the
`## Rules` section's word count, after drafting, would push `curating-memory/SKILL.md` uncomfortably
past its current size for a file loaded on every capture — a size judgment the planner should make
after drafting the actual prose, not before.

### Discretion #2 — D-07's decline-persistence mechanism

**What we know:** the only cross-session state an agent in this system has is the engram store
itself (session-scoped hook markers, like `posttooluse-memory-capture-nudge`'s `tempfile`-keyed
file, are explicitly per-session and vanish). `store_memory`'s search-before-store step already
runs before every capture and is semantic (natural-language match, not exact-string), which is
exactly the mechanism D-05's repeat-hit trigger already leans on. No new tool, field, or category is
needed — D-04's zero-new-Go-dependency and minimal-surface posture both favor reuse.

**Recommendation:** on decline, the agent stores an ordinary memory (category `gotcha` is
defensible — it *is* a still-true footgun fact, just not rule-worthy yet in the user's judgment — or
a new lightweight convention such as "declined rule proposals," tagged e.g. `rule-declined`) whose
content states what was proposed and that the user declined rule-status for it (not that the
underlying fact is untrue). The next time `search_memory`'s search-before-store step or a repeat-hit
trigger surfaces the original gotcha, it will also surface this decline record adjacent to it (same
semantic neighborhood), and the proposal protocol should check for a decline record on the same
concern before re-proposing. This is not a hard suppression (nothing blocks the agent from
proposing again if the user's tolerance has changed), but it converts "propose blindly every time"
into "propose only if no adjacent decline record surfaces" — satisfying D-07's "not the user's only
defense" requirement without new infrastructure. State this mechanism explicitly in the new `###
Proposing a rule` subsection so it is not left implicit.

### Discretion #3 — Is D-12's backfill a documented agent procedure or a one-off operator command?

**What we know:** every existing one-time-reconciliation command in this repo
(`migrate-remap-owner`, `backfill-short-ids`, `prune-expired`, `summarize-missing`) resolves on a
**structural** predicate — owner-field equality, a missing `short_id`, an expiry timestamp, an
empty summary field — that a deterministic Go loop over Qdrant records can evaluate without
judgment. D-12's task is different in kind: "does this gotcha's content read as a normative
constraint" is a semantic classification, the same kind of judgment D-05's inline trigger asks the
agent to make at write time. A CLI command doing this would need either a crude keyword heuristic
(MUST/NEVER/ALWAYS substring match — cheap, but then it's just a batch-mode instance of the D-05
trigger's own logic, which argues for reusing the trigger's phrasing rather than building a second
implementation of it) or an LLM call from Go (new infrastructure, out of scope, and a category
of complexity none of this milestone's other operator commands need).

**Recommendation:** make D-12 a documented agent procedure — a short numbered sequence in the new
`### Rule hygiene` subsection (or immediately after it) instructing the agent, when explicitly
invoked by the user for the one-time sweep, to `list_memory`/`search_memory` the spine for
`category: gotcha` records, apply the same normative-phrasing test the D-05 trigger uses, and run
the same inline proposal protocol per candidate — reusing D-05/D-06's machinery rather than
inventing a parallel one. Sequence it like `migrating-from-beads`'s five-step shape (confirm scope →
enumerate → per-item propose/decide → report a summary), since that skill is the closest existing
precedent for a bounded, run-once, per-item review loop even though it isn't the trigger analog.
This also satisfies the roadmap's framing of D-12 doubling as "the criterion-3 demonstration" more
directly than a silent CLI sweep would — a CLI command produces no visible per-item proposal for the
user to accept/decline, which is the actual observable behavior criterion 3 asks to be
demonstrated.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | pytest (`skill/engram/hooks/tests`, subprocess + real git fixtures) for hook text; **no framework exists for `SKILL.md` prose** — skills are loaded by Claude Code directly, not executed |
| Config file | `skill/engram/hooks/tests/` (no pytest.ini found; invoked directly via `uv run`) |
| Quick run command | `uv run --with pytest pytest skill/engram/hooks/tests -q` (Taskfile `test:hooks`, `Taskfile.yaml:42-44`) |
| Full suite command | `task` (repo-wide lint + test, includes `test:hooks` as one of its constituent tasks) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-rule-capture-investigation | Written root cause exists | manual-only (documentation review) | N/A — the deliverable is prose (this file + 06-CONTEXT.md), not executable behavior | N/A |
| REQ-rule-capture-intervention | `curating-memory/SKILL.md`'s Rules section states the trigger + proposal protocol without a buried-permission defect | manual-only (prose review against D-01/D-02's stated defect) | N/A — no automated way to assert "this sentence does not read as a prohibition" without a prose-strength gate that would itself be evadable by rewording (explicitly warned against in this agent's brief) | N/A |
| REQ-rule-capture-intervention | `docs-site/reference/tools.md`'s `store_rule` prose is corrected to match | manual-only | N/A | N/A |
| REQ-rule-capture-intervention | The user-blessed gate remains intact — no code path stores/edits/deletes a rule without an explicit tool call the user consented to | structural (existing Go tests already pin this) | `rg -n "errRuleImmutable" internal/server/tools.go` confirms `setVisibility`/`supersedeMemory` still reject rules; **no new test needed** since this phase makes no Go code change to `store_rule`/`deleteMemory`/`setVisibility` | ✅ (`internal/server/rules_test.go`, `internal/server/tools_test.go` already cover this) |
| REQ-rule-curation-hygiene | The curation-smell advisory fires above 50 rules per scope | automated, already shipped | `go test ./internal/server/... -run TestListRulesHandlerCurationAdvisory -v` | ✅ `internal/server/rules_test.go:550-587` |
| REQ-rule-curation-hygiene | D-07's decline mechanism is documented (a memory record, not new machinery) | manual-only (prose review) | N/A | N/A |

**On evadable prose gates:** the brief explicitly warns against inventing automation that only
asserts a string exists while testing nothing. An `rg` assertion like `grep -c "propose it to the
user" SKILL.md == 1` would pass trivially on a reworded-but-still-buried sentence, or fail on a
correctly-fixed sentence that happens to drop that exact phrase — it tests wording, not the actual
defect (a permission clause structurally subordinate to two restrictions). No pattern-based gate can
distinguish "the permission reads as primary" from "the permission reads as secondary" — that is a
comprehension judgment, honestly manual-only. The one exception: if the new subsection headers
(`### Proposing a rule`, `### Rule hygiene`) are given fixed names, a structural `rg -n "^### Proposing a rule$"
skill/engram/skills/curating-memory/SKILL.md` gate *can* verify the subsections exist at all — a
weak but non-zero signal, worth adding as a cheap regression check even though it cannot verify the
prose inside is correct.

### Sampling Rate

- **Per task commit:** `uv run --with pytest pytest skill/engram/hooks/tests -q` if any hook file is
  touched (only relevant if the planner adds a session-scoped hint to
  `session-start-memory-recall`, which is not required by any locked decision); otherwise skip —
  no Go code changes are anticipated for this phase's core deliverable.
- **Per wave merge:** `task` (repo-wide lint + test) — cheap and already the project's standard
  phase-close gate; run it even for a prose-only phase, since it also re-runs `task license:check`
  and `task fmt` against any new/touched Markdown, and catches an accidental Go edit early.
- **Phase gate:** manual read-through of the corrected `## Rules` section against D-01/D-02's
  stated defect (does the permission clause now read as primary, not buried) before
  `/gsd-verify-work`; this is the phase's actual acceptance test and cannot be automated per the
  reasoning above.

### Wave 0 Gaps

- None for Go — no new Go code is anticipated (both `deleteMemory` and the curation-smell advisory
  already exist and are already tested).
- **If** the planner adds a fixed-header structural gate (`### Proposing a rule` / `### Rule
  hygiene`), it does not yet exist and would need writing as a new `test_*` function inside
  `skill/engram/hooks/tests/` **or** — more honestly, since it is not testing hook *behavior* at
  all — as a standalone lightweight script/CI check outside the hooks pytest suite (the hooks suite
  is scoped to executable hook text via subprocess; a static grep over `SKILL.md` doesn't belong
  there structurally, though nothing prevents adding it if the planner judges the convenience worth
  the scope mismatch).
- No fixtures needed either way — a pure `rg`/`grep` structural check over a static file needs no
  test harness.

## Security Domain

This phase touches no authentication, authorization, cryptography, or new input-validation surface.
`security_enforcement` is absent from `.planning/config.json` (enabled by default), so this section
is included per the protocol, but its content is necessarily thin — there is genuinely little ASVS
surface here.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | No new auth surface; all tool calls this phase relies on (`store_rule`, `delete_memory`, `store_memory`, `list_rules`) already exist and are already gated by the existing OIDC bearer chain. |
| V3 Session Management | no | No session-state change; the decline-memory recommendation (Discretion #2) reuses the existing durable-memory write path, which carries no session semantics of its own. |
| V4 Access Control | yes (unchanged, confirmed) | The user-blessed gate (D-04/D-10) is an **application-level** consent gate, not an authz control — `internal/authz`'s Cedar PDP and the owner/scope isolation model are completely untouched by this phase. Confirmed this session: `deleteMemory` and `store_rule`'s ownership/scope checks are unmodified by any recommendation here. |
| V5 Input Validation | no (unchanged) | `validateStoreRule`/`validateRuleSummary` (`rules.go:62-98`) are unmodified — no new argument, no new field, no new call shape for any tool this phase touches. |
| V6 Cryptography | no | Not applicable. |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| An agent silently "helping" by promoting a rule without asking, because a poorly-worded trigger reads as sufficient permission | Elevation of Privilege (of agent initiative, not a technical privilege) | This is precisely D-04's invariant and exactly what D-01/D-02 diagnose as currently broken (the trigger doesn't fire, so today's failure mode is under-action, not over-action) — the phase's whole design goal is to add initiative to *ask*, never to *act*, and every recommendation in this document (proposal protocol, decline-memory, backfill-as-agent-procedure) preserves that the actual `store_rule`/`delete_memory` call happens only after an explicit user "yes." No mitigation needed beyond what D-04 already specifies; noted here because a careless prose edit that drops the explicit-consent framing while "improving flow" is the realistic failure mode for a prose-only phase, not a code-level vulnerability. |
| A decline-memory record (Discretion #2) being mistaken for a rule or for the underlying fact's negation | Tampering (of interpretation, not data) | State the decline record's content precisely — "user declined to promote X to a rule" is a fact about a *conversation*, not a claim that X is false or unimportant. Getting this wrong in the shipped prose could cause a future agent to treat "declined" as "the gotcha is resolved," which is a correctness bug, not a security one, but worth flagging since it is the one new content-shape this phase introduces. |

## Sources

### Primary (HIGH confidence)
- `Read` of `skill/engram/skills/curating-memory/SKILL.md` (277 lines, full file, this session)
- `Read` of `skill/engram/skills/{promoting-memory,discovering,migrating-from-beads}/SKILL.md`
  (full files, this session)
- `Read` of `skill/engram/hooks/session-start-memory-recall` and
  `posttooluse-memory-capture-nudge` (full files, this session)
- `Read` of `skill/engram/hooks/tests/test_session_start_memory_recall.py` and
  `test_posttooluse_memory_capture_nudge.py` (full files, this session)
- `Read` of `internal/server/rules.go` (full file, 231 lines, this session)
- `Read` of `internal/server/tools.go:1580-1670` (`deleteMemory`, `deleteAll`, `setVisibility`,
  this session)
- `Read` of `docs-site/src/content/docs/reference/tools.md:340-409` (`store_rule`/`list_rules`
  docs, this session)
- `Bash` greps confirming: `errRuleImmutable` definition and every call site
  (`internal/server/identity.go:147-153`, `tools.go:1513,1656,1725`), the curation-smell test
  (`internal/server/rules_test.go:550-587`), no "declin" precedent anywhere in
  `skill/*/SKILL.md`, no SKILL.md test coverage in `Taskfile.yaml` (only
  `skill/engram/hooks/tests` is wired), no `docs-site` rule-hygiene page exists yet, all this
  session

### Secondary (MEDIUM confidence)
- `.planning/phases/06-rule-capture-investigation-fix/06-CONTEXT.md` — the settled root cause
  (D-01/D-02), the locked decisions (D-01 through D-12), and the rule-count/rule-id citations
  (A2), read this session but originally established during the discuss session, not
  independently re-verified against live Qdrant here.

### Tertiary (LOW confidence)
- None — this phase's surfaces were small enough to read exhaustively rather than sample, so no
  claim in this document rests on an unverified web search or training-data guess.

## Metadata

**Confidence breakdown:**
- Standard stack: N/A — no new libraries/frameworks in scope
- Architecture (skill/hook structure, rules.go behavior): HIGH — every claim traced to a `Read` or
  `Bash` grep performed this session against the live repo tree
- Pitfalls: HIGH for pitfalls 1/2 (directly derived from D-01/D-02's own stated mechanism);
  MEDIUM for pitfalls 3/4 (reasoning from the confirmed `ruleView` shape, not from a prior
  incident in this repo — there is no history of a rule-hygiene failure to point to, since the
  rule set has never grown large enough to exercise this problem)
- Assumptions (A1/A2): MEDIUM — sourced from 06-CONTEXT.md, not independently re-queried against
  live Qdrant this session (no `mcp__engram__*` tool access in this research session's tool list)

**Research date:** 2026-08-01
**Valid until:** No fixed expiry — the surfaces researched (skill prose, two Go functions) change
only when this phase or a future one edits them; re-read `06-RESEARCH.md`'s quoted line ranges
against the live files at plan time if more than a few days elapse, since exact line numbers will
drift with any intervening commit.
