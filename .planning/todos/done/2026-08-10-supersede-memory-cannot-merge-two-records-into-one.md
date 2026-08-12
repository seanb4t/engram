---
created: 2026-08-10T19:25:00Z
title: supersede_memory cannot merge two records into one without a delete
area: api
severity: major
files:
  - internal/store/store.go:2029-2042
  - internal/store/store.go:1755
  - proto/engram/v1
---

## Problem

Surfaced during v0.13.x Phase 4 discussion (semantic spine curation), where the skill must propose
what to do about a `same-fact` near-duplicate pair. There is no way to reduce two live records to
one while preserving history.

Two facts about the current API combine into the gap:

1. **`Store.Supersede` always creates a record.** Step 3 is an unconditional
   `s.Upsert(ctx, newMem, vec)` (`internal/store/store.go:2029-2032`), and only then does step 4
   back-stamp `superseded_by` onto the target via `SetPayload`. There is no code path that links an
   *existing* record as the successor of another existing record.

2. **`Update` refuses to let a caller touch the link.** It re-reads the record under lock and
   restores `Supersedes`, `SupersededBy`, and `ArchivedAt` from the fresh copy
   (`internal/store/store.go:1755`), so `superseded_by` stays strictly server-set. Correct as a
   forgery defense — but it means `update_memory` cannot be used to point B at an existing A.

`supersede_memory` also takes a single `supersedes` target, not a list.

Every attempt to merge a pair therefore leaves the store in a worse state than intended:

| Attempt | Result |
|---|---|
| supersede A with the merged record | M live + B live, unlinked — the duplicate survives |
| supersede A, then supersede B | M live + M₂ live with **identical content** — two merged records, not one |
| update the survivor, then supersede the loser | survivor live + M live, duplicate content |
| supersede A, delete B | one live record; history kept only for A's lineage |

Only the last row produces the intended outcome, and it costs a `delete_memory` — the one verb the
supersession design exists to avoid, since it destroys history rather than preserving it.

Consequences:

- The `same-fact` half of Phase 4's curation skill has no clean verb to propose. The phase resolved
  this by scoping `delete_memory` narrowly to true exact duplicates, on the reading that a duplicate
  "should never have existed" and so qualifies as junk under `curating-memory`'s own three-way table.
  That is defensible, but it is a workaround for a missing capability, not the natural expression.
- Any future consolidation tooling (a `spine-review consolidate --apply`, a batch merge) inherits the
  same constraint.
- The `overlapping` verdict — two records that share ground but each carry something the other does
  not — has no expression at all beyond manual authoring, because the union case needs exactly the
  merge shape that is missing.

## Solution

Two candidate shapes, either of which closes it. Both are additive; neither breaks an existing caller.

1. **`supersedes` accepts multiple targets.** `supersede_memory(content, supersedes: [A, B])` stores
   one new record and back-stamps every target, rejecting any that is already superseded (the
   existing single-live-head rule, applied per target). Needs the per-target lock to be taken over the
   whole set, and a decision on partial failure — all-or-nothing is likely right given the existing
   `ErrAlreadySuperseded` check-then-act discipline.

2. **A link-existing verb.** Something like `link_superseded(target, successor)` that stamps
   `superseded_by` on `target` pointing at an existing record, with the same owner-only write gate and
   already-superseded rejection. Smaller surface, but adds a second way to create the link, which the
   forgery-defense reasoning behind `store.go:1755` deliberately avoided.

Option 1 is probably the better fit — it keeps a single link-creating path and reads as a natural
generalization of the existing verb.

Note that a chain constraint applies either way: a target already carrying a non-empty `superseded_by`
must still be rejected, so a merge cannot resurrect a non-head record.

Also worth deciding: whether `idempotency_key` should become supported on this verb. It is currently
unsupported, and a retried multi-target supersede would create a second correcting record — the same
hazard the single-target form already documents, but with more targets to leave half-stamped.
