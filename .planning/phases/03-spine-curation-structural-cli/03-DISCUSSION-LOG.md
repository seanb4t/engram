# Phase 3: Spine Curation — Structural (CLI) - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-06
**Phase:** 3-Spine Curation — Structural (CLI)
**Areas discussed:** Command shape & safety convention, Citation verification tiers, Purge gate &
archive tier, Report output contract

---

## Command shape & safety convention

### Q1 — Preview-by-default vs the tier's `--dry-run` convention

| Option | Description | Selected |
|--------|-------------|----------|
| Invert only purge (`--apply`) | Honors REQ literally, scopes the change; two safety idioms coexist in one tier | |
| Invert the whole destructive tier | purge AND prune-expired become preview-by-default; one idiom, but a breaking change | ✓ |
| Support both flags everywhere | `--apply` and `--dry-run` both accepted, mutually exclusive; no breakage but restores ambiguity | |
| You decide | Leave to the planner | |

**User's choice:** Invert the whole destructive tier → **D-02**, **D-04**
**Notes:** Scouting surfaced that `prune-expired` — the tier's only destructive command — has no
preview flag at all today, making the existing precedent for destructive work the weakest in the
tier. The deciding argument was which way a typo fails: `--dry-run` defaults to destruction, `--apply`
defaults to a no-op.

### Q2 — Verb shape

| Option | Description | Selected |
|--------|-------------|----------|
| `spine-review <verb>` (nested) | Groups five capabilities under one noun; first subcommand tree in the operator tier | ✓ |
| Flat verbs | `engram spine-scan` etc.; consistent with all five existing operator commands | |
| Nested, but purge is top-level | Read-only verbs nest; destructive one separated by path, not just flag | |

**User's choice:** Nested → **D-01**
**Notes:** Matches the roadmap's own naming. Consequence flagged for planning: Phase 2's golden
walker and catalog JSON must traverse the added depth, and memory `jb33frww29` warns that
cobra-tree snapshots are order-dependent.

### Q3 — What defines "the destructive tier"

| Option | Description | Selected |
|--------|-------------|----------|
| Derive from D-11 blast radius | Preview-by-default iff Phase 2's blast-radius table says destructive; derive-don't-declare | ✓ |
| Explicit list of two | purge + prune-expired named; simple but a future destructive command ships with the wrong default | |
| Irreversibility, not blast radius | Preview iff not undoable from within engram; sharper criterion, but a second taxonomy | |

**User's choice:** Derive from D-11 blast radius → **D-03**
**Notes:** Cashes in Phase 2's D-11, which was landed specifically so `spine-review purge` would be
born classified. Makes the blast-radius table load-bearing for runtime safety, not just docs.

### Q4 — Migration posture for prune-expired's flipped default

| Option | Description | Selected |
|--------|-------------|----------|
| Hard flip + upgrade note | Stops deleting without `--apply` immediately; documented alongside Phase 1's exit-code migration | ✓ |
| Deprecation window | Warns and still deletes for one minor release, then flips | |
| Hard flip + refuse ambiguity | Bare invocation exits nonzero asking which you meant | |

**User's choice:** Hard flip + upgrade note → **D-04**
**Notes:** A deprecation window would carry the dangerous default through the release this phase
exists to make safe. The hard flip's failure mode is benign and loud — a script silently stops
deleting; nothing is destroyed.

---

## Citation verification tiers

### Q1 — Coverage of non-file citation kinds

| Option | Description | Selected |
|--------|-------------|----------|
| Fourth tier: unverifiable | commit/url/repo reported with a reason; honest about what was not checked | ✓ |
| Verify file + commit, tier the rest | More coverage at the cost of a git dependency in the CLI | |
| File only, silently skipped | Simplest; a clean report can't be distinguished from an unexamined one | |

**User's choice:** Fourth tier → **D-05**
**Notes:** Same rationale as the REQ's own moved/broken split — the operator must be able to tell
"checked and fine" from "never looked at".

### Q2 — What separates moved-but-valid from broken

| Option | Description | Selected |
|--------|-------------|----------|
| Excerpt found elsewhere in same file | Tight, cheap, no tree walk; exactly #355's edit-above drift shape | ✓ |
| Also search the tree on file-miss | Catches renames/splits; short excerpts risk confident wrong matches | |
| Fuzzy match, not exact | Most forgiving; a similarity threshold with no defensible value | |

**User's choice:** Same-file excerpt search → **D-06**
**Notes:** Chosen specifically so #355 — Phase 5's live acceptance fixture — classifies as moved
rather than broken.

### Q3 — Which tree `verify` resolves against

| Option | Description | Selected |
|--------|-------------|----------|
| CWD repo; other scopes unverifiable | Zero config; reuses the tier from Q1 rather than a second skip path | ✓ |
| `--repo-root` flag, repeatable | Verifies a multi-repo spine; adds a conditional rule binding six surfaces | |
| CWD only, no scope matching | Simplest code; would emit confident wrong verdicts across repos | |

**User's choice:** CWD repo with scope matching → **D-07**
**Notes:** The `--repo-root` option was recorded as a deferred idea, not rejected outright.

### Q4 — Reporting granularity within the broken tier

| Option | Description | Selected |
|--------|-------------|----------|
| Split broken by cause | `file missing` vs `excerpt gone` on the line; costs nothing, the info is already there | ✓ |
| Single broken tier | Keeps to exactly the REQ's tiers with no sub-taxonomy to document | |
| You decide | Leave granularity to the planner | |

**User's choice:** Split by cause → **D-08**
**Notes:** A missing file is usually a mechanically-fixable rename; a vanished excerpt suggests the
cached knowledge itself is stale.

---

## Purge gate & archive tier

### Q1 — Proving extract-before-delete (rule `7smp8vy9hr`)

| Option | Description | Selected |
|--------|-------------|----------|
| Milestone-summary precondition | Gate derived from the rule's own step 2; real checkable artifact | |
| Per-record extraction link | Strongest, but nothing writes links today — purge inert until Phase 4 | |
| Both: link if present, summary as floor | Usable now, strictly stronger once Phase 4 writes links | ✓ |
| Operator attestation | Simplest; proves nothing — attestation-as-theater | |

**User's choice:** Both paths → **D-09**
**Notes:** The full rule text was fetched during discussion (`get_memory 7smp8vy9hr`) rather than
working from its summary. Its step 2 — "write one authoritative milestone-summary" — is the step a
CLI can actually verify; its step 4 bounds what purge may ever classify eligible.

### Q2 — What makes a record purge-eligible

| Option | Description | Selected |
|--------|-------------|----------|
| Explicit candidate query only | Honest that "spent" is a semantic judgment; all safety rests on gate + preview | |
| Structural eligibility classes | Narrow and derivable; doesn't cover rule `7smp8vy9hr`'s own use case | |
| Classes plus filters, filters gated harder | Covers both; blast radius matched to judgment supplied | ✓ |

**User's choice:** Classes plus gated filters → **D-10**

### Q3 — Preview/apply divergence

| Option | Description | Selected |
|--------|-------------|----------|
| Intersection only | Never destroys what wasn't previewed; needs a preview→apply manifest | ✓ |
| Abort on any divergence | Predictable, but fails constantly on a live spine and gets bypassed | |
| Proceed with the fresh set | Literal reading of the REQ; can delete records never previewed | |

**User's choice:** Intersection only → **D-11**
**Notes:** Identified as the phase's primary research item — the manifest/token mechanism is exactly
the tombstone/grace-window gap `research/SUMMARY.md` flags as unspecified. Memory `55zra87def`
applies: a plain exported manifest struct would be forgeable.

### Q4 — Archive's operator-visible shape

| Option | Description | Selected |
|--------|-------------|----------|
| First-class state + archive/restore verbs | Follows `list_scheduled`'s "retained but not recalled" precedent | ✓ |
| Extend the expiry soft-hide | Less machinery; distinctness would rest on one payload field | |
| Defer shape to plan-time research | Honest about the roadmap flag; hands the planner a UX decision too | |

**User's choice:** First-class state + verbs → **D-12**
**Notes:** Storage mechanism remains open and roadmap-flagged for plan-time research. This decision
fixes only the observable shape and the distinctness constraint.

---

## Report output contract

### Q1 — Output format

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse `--output` on spine-review | Existing flag + TTY detection; spine-review diverges from the five neighbors | |
| Reuse it, and backfill the tier | Fully consistent tier; a real scope expansion past the five requirements | ✓ |
| Text only | Consistent with the tier today; forecloses handing consolidate's output to anything | |

**User's choice:** Reuse and backfill → **D-13**
**Notes:** Recorded as scope expansion #2 requiring a `/gsd-phase` roadmap edit. Planning caution
captured: the two tiers deliberately diverge on `--timeout` (`0` rejected vs `0` disables) and the
backfill must adopt `--output` only.

### Q2 — Exit-code semantics for `verify`

| Option | Description | Selected |
|--------|-------------|----------|
| Nonzero on broken only | CI gate out of the box; conflates "command worked" with "data is healthy" | |
| Always exit 0 | Clean separation; no CI gate without parsing output | |
| Exit 0, opt-in via flag | Both consumers served; one more conditional rule to bind on six surfaces | ✓ |

**User's choice:** Exit 0 with opt-in flag → **D-14**
**Notes:** The only place in this discussion the weaker default was chosen, explicitly to protect
Phase 1's separation of concerns.

### Q3 — `consolidate` report shape

| Option | Description | Selected |
|--------|-------------|----------|
| Ranked pairs with scores | Matches `search_memory`'s existing per-result score; no new concept | ✓ |
| Clusters above a threshold | Reads better for milestone collapse; transitive chaining is a known RAG-dedup failure | |
| Pairs, with an opt-in threshold flag | Judgment-free default plus a narrowing knob | |

**User's choice:** Ranked pairs → **D-15**

---

## Claude's Discretion

- Exact verb spellings and flag names within `spine-review`.
- Which health signals `scan` reports (the requirement says "inventory and health signals" without
  enumerating them).
- Whether D-11's preview→apply manifest is persisted, a payload tombstone, or an opaque token.
- How a "milestone-summary record" is identified for D-09's batch floor.
- The default retention window for archived records under D-10's third class.
- Whether the `--output` backfill lands as its own commit ahead of the `spine-review` work.

## Deferred Ideas

- A repeatable `--repo-root` mapping so one run can verify a multi-repo spine.
- Verifying `commit` citations via local git history.
- Transitive clustering for `consolidate` (a Phase 4 semantic concern).
- Unifying `--timeout` semantics between the client and operator tiers.
