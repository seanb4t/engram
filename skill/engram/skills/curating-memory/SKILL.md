---
name: curating-memory
description: Use when storing or updating durable project memory via the engram MCP tools — enforces the engram-vs-beads routing gate (engram is preferred over `bd remember`/`bd memories` for durable facts), durable-only capture, search-before-store, supersede-on-contradiction, and the two-tier spine/overlay scope. Trigger when the user states a durable decision/preference/convention/gotcha, when the user explicitly asks to remember something (including a time-bound reminder, due date, or "not before"/expiry — even if it looks task-shaped), whenever you are about to record a durable fact and the repo also has a beads memory store (prefer engram; do not write it to `bd remember`), on the session-start recall and capture nudges, whenever a durable fact you are about to store contradicts or corrects one already in the store (supersede it, do not overwrite it), and before any mcp__engram__store_memory / schedule_memory / supersede_memory / update_memory / delete_memory / store_rule / list_rules call, when a fact you are about to store is phrased as a MUST / NEVER / ALWAYS constraint on future behavior (propose a rule, never promote one), and when a footgun the store already records is hit again.
---

# Curating Memory

The memory store is **explicit and zero-junk**: only deliberately chosen durable
facts live in it, and it stays correct over time. Apply this discipline before
every memory write.

## Routing: is this an engram memory at all?

Decide *where* the fact belongs **before** applying the taxonomy below:

- **A task to *do*** — a work item or action ("drain hl-kt1r", "fix the flaky
  test", "ship the release") → that is a **bead**, not a memory. Route it to the
  issue tracker; engram does not track work.
- **A durable fact about the repo/project** — a decision, preference,
  convention, or gotcha → engram memory. Continue below.
- **A durable fact in a repo that *also* has beads memory** (`bd remember` /
  `bd memories`) → **still engram.** engram is the preferred durable-memory store;
  it wins over beads memory. Write the fact to engram, search engram first, and
  do **not** split durable facts across both stores or mirror them into
  `bd remember`. Beads memory is a read-only fallback for *recall* only when
  engram is unavailable (401/403). Note the asymmetry that makes this easy to get
  wrong: `bd prime` *auto-injects* its memories at session start, so memory can
  *look* covered before you have read engram at all — it is not. Pull engram
  explicitly, and capture into engram the moment a fact is established rather than
  storing in beads and "lifting" later.
- **An explicit ask to remember with a time bound** — "remember X by/until/after
  `<when>`", a due date, a deferred reveal → engram, via `schedule_memory`. The
  explicit *remember* plus the time window is the durability signal; do **not**
  drop it as transient. See Scheduling.
- **Unclear** — task-shaped *and* an explicit timed "remember" (e.g. "remember
  to drain hl-kt1r tonight or tomorrow"), or the scope is ambiguous → **ask /
  offer the choice** (bead vs scheduled memory). Never silently pick one.

## Junk taxonomy

**STORE (durable):** decisions, preferences, conventions, gotchas, and
project-specific facts that outlive the session.

**DO NOT STORE:** transient state, current activity/progress, secrets or API
keys, timestamps, one-off tool output, or anything trivially re-derivable.

## Rules (user-blessed ground truth)

A **rule** is normative ground truth for the repo/project — a MUST-follow
constraint, always shared, stored via `store_rule` in a `rule:repo:*` /
`rule:project:*` scope. Store a rule **only on explicit user instruction** —
never promote one unilaterally. A rule's `summary` is a single line (the
session-start index entry). This complements — it does not replace — the
decision / preference / convention / gotcha routing above; a rule is the
narrower, user-blessed, normative case.

### Proposing a rule

Offer a rule the moment you notice a candidate. Proposing is not promoting —
the user still decides.

Either of two triggers is sufficient:

- **Repeat-hit on a footgun.** The `search_memory` call you already make
  before every store surfaces an existing record describing the same problem
  you have just hit again. The store knew, and knowing did not prevent the
  hit — that gap is what a rule closes. This trigger costs nothing new: the
  search-before-store step already runs and already returns the record.
- **Normative phrasing at capture time.** The `content` of the record you are
  about to write via `store_memory` or `schedule_memory` is phrased as a
  constraint on future behavior — MUST, MUST NOT, NEVER, ALWAYS, or a bare
  imperative with no exception. It is already a constraint; the only open
  question is whether it should be normative ground truth rather than a
  `gotcha`. Scope this to the record's own content, at the moment you are
  about to write it — not to conversation, to text you are quoting, or to a
  requirement you are restating. A trigger that fires on every MUST-shaped
  sentence in a session is the nag that gets the whole mechanism turned off.

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

A proposal that fires too often is the failure mode this whole mechanism
lives or dies by. Do not propose when: the fact is scoped to one file or one
session, not normative; the candidate was already declined; you have already
made one proposal this turn; or you cannot state the constraint as a single
index line, which means you do not yet understand it well enough to propose
it.

This adds a narrower test on top of the routing and junk-taxonomy sections
above; it does not replace them.

### Rule hygiene

A rotted rule is worse than a rotted memory: rules are MUST-follow, so a wrong
one does not merely sit there being incorrect — it misdirects every later
session that reads the index and acts on it.

**When to run these checks.** Not every session — that turns hygiene into a
tax nobody pays. Three moments are enough: when you are about to bless a new
rule (the one point where comparing it against the existing set is directly
useful); when `list_rules` returns its curation-smell advisory; and at
milestone completion, alongside the memory-curation pass rule `7smp8vy9hr`
already establishes for that moment — its procedure extracts reusable facts
embedded in per-phase lifecycle records first, writes one authoritative
milestone summary, and only then deletes the collapsed per-phase records,
never touching reusable codebase facts. Rule hygiene rides that same pass
rather than inventing a separate cadence: while curating memories at
milestone completion, also run the duplicate/contradiction/rot checks below
against the rule set.

**Price the index honestly (D-11).** Session start loads only the compact
`ruleView` — `short_id`, `summary`, `tags`, `scope`, `created_at` — and it
carries no `content`. That split changes what each check can do from the
index alone:

- **Duplicates.** Catchable from summaries alone — a duplicate rule usually
  has a near-duplicate index line. Free; do it while reading the index you
  already have.
- **Contradictions.** Not catchable from summaries. Two rules can carry
  near-identical summaries and opposite content, or unrelated summaries and
  directly conflicting content. A real check needs full text.
- **Rot.** The constraint a rule names no longer exists — the path, command,
  tool, or workflow it constrains is gone or has changed. `created_at` is the
  cheap aging signal; the check itself runs against the current tree, not
  against the index.

Price the full-text read exactly: it is **one `list_rules` call with
`full: true`**, which returns the whole set in a single response — not a
`get_memory` per rule. The cost scales with total rule bytes, not with a
round trip per rule. At the three rules this repo has today it is free; at
fifty the response is large enough to be worth gating. Gate it behind the
moments named above and nothing else — an unconditional full-text read every
session is the habit this pricing exists to prevent.

`list_rules` also returns a curation-smell advisory when a scope holds more
than 50 rules (`ruleThreshold`, pinned by
`TestListRulesHandlerCurationAdvisory`). It is a **volume signal only** — it
says nothing about duplication or contradiction, and it cannot fire below 51
rules in a scope, so it will never fire at this repo's current scale. Treat
it as one input, never as the discipline.

**Correcting a rule.** Rules do not correct like memories —
`supersede_memory` is rejected for them (see Supersession, below). Which tool
applies depends on what actually changed:

| Situation | Tool | Why |
|-----------|------|-----|
| Same constraint, better wording — a clearer index line, an added caveat, a tag fix | `update_memory` | in-place refinement; the `short_id` and index entry survive. The only two guards that apply to a rule: the summary must stay a valid single-line index entry, and `shared: false` is rejected. |
| A full rewrite of the rule text against the same constraint | `store_rule` with `id` set to the existing UUID or `short_id` | an ownership-checked in-place replace that carries the existing `short_id` forward, so anything citing that handle still resolves |
| The constraint is wrong, reversed, or rotted — the rule should stop existing | propose `delete_memory`, then `store_rule` fresh if a corrected rule should stand in its place | rules cannot be superseded, so there is no correcting-record path; this is the only delete-then-re-store case |
| Make a rule private | none — `set_visibility` and `update_memory` with `shared: false` both reject it | rules are always shared; delete instead |

**Deleting a rule is user-blessed, symmetrically with creating one (D-10).**
Nothing in the server stops you — `delete_memory` has no rule guard, unlike
`set_visibility` and `supersede_memory`, which reject rules outright. Propose
the removal, name the rule's `short_id` and the specific reason it should go,
and call `delete_memory` only after the user has explicitly blessed it — this
instruction is the only gate there is, so never perform it on your own
judgment.

**No history survives a rule deletion.** `supersede_memory` is rejected for
rules, so unlike a corrected memory there is no "what we used to believe and
when it changed" trail. If the old text matters, quote it in the proposal
before deleting — after the delete it is gone.

**Disposition vocabulary.** Keep (no issue). Merge (a duplicate — refine the
survivor via `update_memory`, propose deleting the other). Flag (a
contradiction — surface both `short_id`s and let the user resolve; never pick
a winner yourself). Retire (rot — propose deletion, quoting the old text
first). Every disposition that removes anything terminates in a user
decision.

### One-time rule backfill sweep

Rules only started being proposed when the trigger above shipped, so the
store already holds facts that read as normative and were filed as ordinary
memories before it existed. This sweep surfaces them once so each can be
blessed or declined. It runs **only when the user asks for it**, once per
repo — the inline trigger above covers everything after it, and re-running
this on a cadence is the nag the decline record exists to prevent.

1. **Confirm the scopes.** The spine (`Memory spine scope` from session
   start) is where the candidates live; the `rule:repo:*` scope is where a
   blessed rule goes. If engram returns 401 or 403 the server is
   unauthenticated — tell the user to authenticate via `/mcp` and stop. Store
   nothing and delete nothing on an auth failure.
2. **Enumerate the candidates.** Call `list_memory` on the spine with
   `categories` set to `gotcha` and `full` set to `true` — the compact view
   returns summaries, and the normative-phrasing test needs the record's own
   content. Page with `cursor` until `next_cursor` comes back empty. **Skip
   any record tagged `rule-declined`**: those are records of a past decline,
   not candidates, and re-proposing them is exactly what that tag exists to
   prevent. Consider `convention` records too if the user asks for a wider
   pass, but default to `gotcha`.
3. **Apply the test.** Use the same normative-phrasing test `### Proposing a
   rule` defines — do not write a second heuristic here and do not loosen
   it, or the sweep and the inline trigger will drift apart.
4. **Decide per candidate**, running the proposal protocol from `### Proposing
   a rule` unchanged:
   - **Bless.** The user says yes. Call `store_rule` and cite the new
     `short_id`. Then ask a *separate* question about the source record:
     delete it as now-redundant, or keep it for context the one-line rule
     does not carry. That deletion is its own user decision — never fold it
     into the rule proposal, and never perform it because the bless implied
     it.
   - **Decline.** The user says no. Store the decline record exactly as
     `### Proposing a rule` specifies (`category: decision`, tag
     `rule-declined`). Leave the source record untouched — a decline is
     about rule status, not about the fact.
5. **Report a summary.** Each candidate's `short_id` and its outcome —
   blessed with the new rule's `short_id`, declined with the decline
   record's `short_id`, or skipped and why — plus which source records were
   deleted. Cite `short_id`s throughout.

The sweep proposes, it never promotes: a sweep of twenty candidates is twenty
proposals and twenty user answers, not one approval applied twenty times. If
that is too many, stop and ask the user how they want to narrow it — do not
batch the consent.

## Tagging

`tags` are now a recall dimension, not just display metadata: `search_memory`
and `list_memory` accept an optional `tags` filter that narrows results to
records carrying **all** listed tags (AND), and `update_memory` can correct a
record's tag set after the fact. So tag deliberately — a tag is only useful for
recall if it is applied **consistently** across the records that share it.

`search_memory`, `list_memory`, and `list_scheduled` also accept optional
`created_after` / `created_before` (RFC3339, half-open `[after, before)`) to
window recall by creation time. `list_memory` additionally accepts a `cursor`
arg (opaque, from `next_cursor`) for deterministic pagination and returns
`{ "memories": [...], "next_cursor": "<token>" }` — an empty `next_cursor`
signals the last page.

Stay semantic-first, though: tags **complement** vector recall, they don't
replace it. Reach for a tag when you want a precise, boolean axis the content
text can't express (e.g. a project/component label, a `decision` vs `gotcha`
distinction already covered by `category`, a transient-vs-durable marker) — not
to rebuild a folksonomy the embedder already handles. Prefer a handful of stable,
low-cardinality tags over an ever-growing taxonomy; when in doubt, rely on
content + semantic search and leave the record untagged.

## Cross-spine recall

`search_memory` and `list_memory` accept `cross_spine` (bool) to span every
scope you can read; it ignores any `scope` you also supply. Otherwise,
<!-- engram:rule:start scope-required-unless-cross-spine -->scope is required unless cross_spine is true<!-- engram:rule:end scope-required-unless-cross-spine -->.
The response reports `searched_scopes` and `scopes_truncated`, which name the
scopes searched under your authorization — not the scopes that had results.

### When not to use cross-spine

Cross-spine is an opt-in widening, and the failure mode of an opt-in widening
is setting it on every call. Don't. The default is scope-confined, and it
should stay that way for ordinary work: a session-start bootstrap, a recall
about the repo you're in, or anything where the two-tier spine/overlay scope
you already know is the right scope. Reach for `cross_spine` only when the
thing you're looking for might live somewhere you're not — a decision made in
another repo, a convention that spans projects, a memory whose scope you
genuinely don't know.

Two costs come with it. A broader result set dilutes ranking, so a
cross-spine search can rank a distant match above a local one. And a
cross-spine call adds a second bounded scan of your readable set to
enumerate the scopes it searched. Neither is free — reach for `cross_spine`
because you need it, not by default.

## Reading a rejection

A rejected call names the field that failed and a machine-stable hint code in one
envelope: `field=<name> hint=<code>: <human text>`. Branch on `field` and `hint`, not
on the wording after the colon — the wording is not a contract and changed in this
very release. Full vocabulary: [`reference/errors.md`](/reference/errors/); do not
duplicate the table here.

The two or three retry patterns that come up in practice:

- **`hint=too_long` on `summary`** — shorten the summary and resend just that field's
  worth of change; do not reconstruct or resend the whole record.
- **`hint=required`** — the field was absent, not malformed. Add it; nothing else about
  the call was wrong.
- **`hint=mutually_exclusive`** — the error names *two* fields under `field=`. The pair is
  wrong together, not either one alone — don't guess which one to drop without reading both
  names.
- **`hint=ordering`** — read `field=`, which names *one or two* fields. Two means they are
  misordered relative to each other. One means it is misordered relative to a fixed
  reference, not to another argument you sent (`not_after` must be in the future) — so no
  change to a second field will fix it.

## Discipline

1. **Search before store.** Call `mcp__engram__search_memory` across both
   the spine and (if present) the workspace overlay first. `search_memory` is
   backed by a semantic/vector engine, so query it with a natural-language
   description of the fact (not keyword fragments) — it surfaces conceptually
   related records even when they share no exact wording. If a near-duplicate
   exists, update it instead of adding a new record.
2. **Supersede on contradiction — within a tier.** When new info *conflicts with*
   an existing memory, call `supersede_memory` — it stores the correcting record
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
3. **Tier selection.** Default to the **spine** (`Memory spine scope` from
   session start) — most durable facts are repo-wide and should follow the user
   into every workspace. Store to the **overlay** (`Memory workspace scope`)
   only when a fact is genuinely local to this line of work and would be wrong
   or premature elsewhere (e.g. an in-flight decision that contradicts main
   until merged). Promotion of overlay facts to the spine when work merges is
   the `promoting-memory` skill.
4. **Provenance.** Set `source` honestly (`user-said` vs `agent-inferred`). Do
   not set `actor` — it is assigned server-side from the validated OAuth token.

## Scheduling (temporal validity windows)

Use `schedule_memory` instead of `store_memory` when a durable fact should not be
recalled *yet*, or should stop being recalled *after* a point in time. It takes
the same fields as `store_memory` plus a validity window: `not_before` (RFC3339;
hidden from recall until then, a deferred reveal) and/or `not_after` (RFC3339;
dropped from recall at then, an expiry) —
<!-- engram:rule:start schedule-window-at-least-one-bound -->schedule_memory requires not_before and/or not_after (use store_memory for unscheduled records)<!-- engram:rule:end schedule-window-at-least-one-bound -->,
and when both are set,
<!-- engram:rule:start window-not-before-before-not-after -->not_before must be strictly before not_after<!-- engram:rule:end window-not-before-before-not-after -->.
Search-before-store
still applies. The junk-taxonomy "transient" exclusion targets *incidental* state
the user never asked to keep — an explicit request to remember something
by/until/after a time is itself durable (the ask is the signal), so schedule it
rather than discarding it. Scheduling controls *when* a durable fact is active,
not *whether* it is durable.
<!-- engram:rule:start discovery-not-schedulable -->discovery is not schedulable; use store_discovery<!-- engram:rule:end discovery-not-schedulable -->.

A windowed record inside its active window surfaces normally through
`search_memory` / `list_memory`. Outside that window the recall tools hide it,
and `list_scheduled` lists only those hidden records (`state`: `scheduled`
default | `expired` | `all`) — never the active ones, so an in-window record
absent from `list_scheduled` is reached through ordinary recall, not missing.
Recall is gated, but fetch-by-id (`get_memory`) is not — it accepts either the
full id or the short_id. Operators reclaim lapsed records with the `engram
prune-expired --apply` CLI (add `--older-than DUR` for a grace period):
<!-- engram:rule:start destructive-requires-apply -->a destructive operator command previews by default and mutates only when apply is set<!-- engram:rule:end destructive-requires-apply -->
a bare invocation previews the eligible count and deletes nothing.

Operators can permanently delete purge-eligible records with `engram spine-review purge --apply`
(also preview-by-default). This is the deletion contract an agent should understand even though it
never calls this CLI verb directly: a candidate must satisfy an extract-before-delete gate (a
server-set `superseded_by` link to a later record, or an authoritative milestone-summary record
covering the batch) before it can be removed, and its free-form filter path (category/tags/older-than
with no structural class selected) additionally requires:
<!-- engram:rule:start purge-filter-requires-scope -->the free-form filter path requires an explicit --scope or --all-scopes: category or tags always engage it, and older-than engages it when no class is selected<!-- engram:rule:end purge-filter-requires-scope -->.
`discovery` and `rule` category records are never purge-eligible under any class or filter.

## Supersession (correcting without losing history)

`supersede_memory` is the correction verb. It takes the full `store_memory` field
set (content, scope, tags, category, …) for the **new, correcting** record, plus
`supersedes`: the id — full UUID or `short_id` — of the record it replaces. In one
call it stores the new record and stamps `superseded_by` onto the old one.

What that buys you:

- The superseded record **stops surfacing** in `search_memory` / `list_memory` /
  `search_discovery` / `list_scheduled` — recall stays clean and agents act on the
  current truth.
- It remains **fully fetchable by id** via `get_memory`, and the new record carries
  a `supersedes` link back to it — so "what did we believe before, and what
  replaced it" is always answerable. Nothing is deleted or overwritten.

Rules that will bite you if ignored:

- **Supersede the live head, not a link mid-chain.** Superseding an
  already-superseded record is rejected (`already superseded`). A chain keeps one
  live head: correcting C→B→A is fine, but you must always target the current
  head. If you get that rejection, `search_memory` for the current record and
  supersede *that*.
- **You must own the target.** Supersession routes through the ownership *write*
  gate — a `shared` record you can read is **not** one you can supersede. A target
  you don't own is indistinguishable from one that doesn't exist (both 404).
- **Rules can't be superseded.** `store_rule` records are normative ground truth;
  delete the rule instead (same restriction as `set_visibility`).
- **Never automatic.** Do not supersede on a similarity hunch or as a write-through
  side effect. Supersession is an explicit correction of a *specific* record you
  identified — if you're unsure which record is wrong, search first.
- `idempotency_key` is **not** supported here — a retried supersede creates a
  second correcting record rather than replaying the first. Retry only after
  confirming the first call didn't land.

## Citations (structured provenance)

A memory may optionally carry structured **citations** — the same shape
`store_discovery` uses — an array of source anchors, each with `kind`
(`file`, `commit`, `url`, or `repo`), `ref` (required, non-empty), and
optional `locator`, `pin`, and `excerpt`. Available on `store_memory`,
`schedule_memory`, and `supersede_memory` (`update_memory` does not accept
citations yet).

**When to attach one.** A claim whose value depends on being checkable
against a specific source: a convention traced to the file that establishes
it, a gotcha traced to the commit that introduced it, a decision traced to
the ADR or issue that records it. The citation is what lets a future reader
(agent or human) verify the claim instead of taking it on faith.

**When NOT to.** A preference, an opinion, or anything where the anchor
would be decorative rather than checkable. Citations are **optional
provenance, never a routine field to fill in** — attaching them reflexively
"because the capability exists" erodes the zero-junk invariant this skill
exists to protect, the same way auto-extraction would. Most memories should
carry zero citations; that is the well-formed default, not a gap to close.

**Constraints.** At most 50 citations per record. At most 16 KiB per
excerpt. `ref` is required and non-empty. Citations are **never inferred
from content** — only what you explicitly supply is stored, even when the
memory text is dense with file paths, URLs, or commit SHAs that *look*
citable.

**Reading them back.** Citations do not affect ranking or recall order.
They are **omitted from the default compact recall view** (`search_memory`/
`list_memory`), so if you need them, call `get_memory` (always returns
them) or pass `full=true` to `search_memory`/`list_memory`.

## Summaries

Pass `summary` on `store_memory` or `update_memory` when you have explicit
context on the caveat-bearing essence of the fact — `summary_source=client`.
Recall returns compact summaries by default to keep the spine small; before
acting on caveats or edge cases visible only in the summary, call `get_memory`
(id fetch, always returns full content) or pass `full=true` on `search_memory`/`list_memory`.
When `summary_source=auto` (offline-generated), the summary is lossy — rely
on it only as orientation. Changing a memory's content while a caller-authored
summary (`summary_source=client`) is present requires you to address it:
re-send it (unchanged), update it (revised summary), or clear it (send empty
`summary`) — otherwise the update is rejected.

A `summary` is bounded — 512 bytes by default (`ENGRAM_MEMORY_MAX_SUMMARY_BYTES`)
— so a paragraph-shaped summary that used to be silently accepted is now rejected
with `field=summary hint=too_long`; keep it a short caveat-bearing digest, not a
second copy of the content.

## Tools and auth

All tools are on the `engram` server: `mcp__engram__store_memory`,
`…__schedule_memory`, `…__search_memory`, `…__supersede_memory`,
`…__update_memory`, `…__delete_memory`, `…__list_memory`, `…__list_scheduled`,
`…__get_memory`. If a
call returns 401/403 the server is not authenticated —
tell the user to authenticate via `/mcp` (engram → Authenticate), and
restate the durable fact so they can re-store it after authenticating; never
drop it silently.
