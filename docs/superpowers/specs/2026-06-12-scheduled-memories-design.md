<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Design: Scheduled / Future Memories (Temporal Validity Window)

- **Bead:** engram-rb2
- **Date:** 2026-06-12
- **Status:** Design (pending design-reviewer)

## Problem

Agents want to capture memories that are not relevant *now* but will be — or
that should stop being relevant after a point in time. Three concrete shapes,
all decided in-scope:

1. **Deferred recall** — store now, hide from recall until a future date
   (`not_before`). The "agenda" case: *"next session, do X."*
2. **Expiry / TTL** — relevant only until a future date (`not_after`), then
   drop out of recall.
3. **Intentions / agenda** — forward-looking items that "wake up" into normal
   recall when their time arrives. This is the deferred-recall case viewed from
   the consumer side; it needs no separate mechanism.

**Explicitly out of scope:** active reminders / push / cron. Engram is a
pull-based MCP server with no scheduler or notification loop, and we are not
adding one.

## Constraints from the existing system

- **Recall is pull-based.** Memories surface only when an agent calls
  `search_memory` / `list_memory`. "Scheduled" therefore means *time-gated
  filtering at recall time*, never a server-initiated push.
- **`store_memory`'s contract forbids timestamps as content** (*"Do NOT store
  transient state, secrets, or timestamps"*) and the design intent is
  "explicit, zero-junk, correctable, no auto-extraction." The temporal window
  must be **structured metadata**, not content, and must not muddy
  `store_memory` itself.
- **`Search` returns Qdrant top-k with no Go post-filter** (`internal/store/store.go`
  `Store.Search`). Any recall gate therefore MUST be a Qdrant filter
  condition — post-filtering would silently shrink results below `k`.
- **`List` builds a Qdrant filter (`listFilter`) then sorts in Go.** Same gate
  applies in the filter.
- **`created_at` is stored as an RFC3339 string** (`payload`, the
  `Memory`→Qdrant payload builder), which Qdrant cannot range-filter. The new
  temporal fields must be stored as a **numeric epoch-second** payload so
  `NewRange` applies. Note the type seam: the payload *value* is an integer
  epoch second, but the Qdrant `Range` filter struct fields (`Gt`/`Gte`/`Lt`/
  `Lte`) are `*float64` — so the gate bound is built as
  `qdrant.PtrOf(float64(now.Unix()))`. Epoch seconds fit in `float64` without
  precision loss; Qdrant compares the integer payload against the float bound
  numerically.
- **Absent-value handling has a precedent.** The `visibility` filter already
  composes "match the value OR the key is absent/empty." The temporal gate
  reuses that pattern via `NewIsEmpty`.

### Grounding trace

- `grounding/probe`: `Search` = Qdrant top-k, no post-filter (gate must be in
  the filter); `List` = `listFilter` + Go sort; authz is a composable
  `Must:[scope, ownerOrShared, ...optional]`; `created_at` is an RFC3339 string.
- `grounding/context7 /qdrant/go-client`: `NewRange(field,{Gte,Lte,Gt,Lt})`
  ranges numeric/date fields; `NewIsEmpty(field)` / `NewIsNull(field)` test for
  absent fields. Server-side gate is feasible and index-free (Qdrant filters
  without a payload index; full-scan acceptable at current scale).

## Data model (`internal/store`)

Add two nullable fields to `Memory`:

```go
// NotBefore gates deferred reveal: the record is hidden from recall until
// now >= NotBefore. nil = always active (no lower gate).
NotBefore *time.Time `json:"not_before,omitempty"`
// NotAfter gates expiry: the record drops out of recall once now >= NotAfter.
// nil = never expires.
NotAfter *time.Time `json:"not_after,omitempty"`
```

- **Pointers**, not zero-value `time.Time`, so "unset" is unambiguous.
- `payload` writes `not_before` / `not_after` as **epoch-second integer** keys
  **only when non-nil** (e.g. `p["not_before"] = m.NotBefore.Unix()`). Omission
  (not an empty value) is what makes `NewIsEmpty` match the no-gate case.
- `fromPayload` reads them back into `*time.Time` when the keys are present
  (`time.Unix(v.GetIntegerValue(), 0).UTC()`); absent keys leave the pointers
  nil.

**Active predicate.** A record is *active* at instant `now` iff:

```
(NotBefore == nil || NotBefore <= now) && (NotAfter == nil || NotAfter > now)
```

`not_after` is treated as **exclusive** (a record expires *at* `not_after`).

## Default-recall gate (transparent, backward-compatible)

`Store.Search` and `Store.List` append two composable `Must` conditions to the
existing authz filter, using a server-supplied `now` (epoch seconds):

```
Must += Should[ NewRange("not_before", {Lte: now}), NewIsEmpty("not_before") ]  // not deferred
Must += Should[ NewRange("not_after",  {Gt:  now}), NewIsEmpty("not_after")  ]  // not expired
```

- Records with **no window** match via `NewIsEmpty` → behavior unchanged
  (full backward compatibility for every existing record, which has neither
  key).
- The gate is an additional outer `Must`, orthogonal to
  `ownerOrSharedCondition`; no filter combination can widen cross-actor reads.
- **`get_memory` / `FetchForUpdate` (by-id) are left ungated.** The rule is:
  **recall is gated, explicit-by-id is not.** This is what lets an owner manage
  a pending or expired memory — `list_scheduled` finds the ID, then
  `get` / `update` / `delete` operate on it ungated.

**Time source — settled (no public signature break).** `Store` gains an
injectable clock: an unexported `now func() time.Time` field defaulting to
`time.Now`, set via a functional option on `New` (e.g.
`New(client, collection, WithClock(fn))`). `Search` and `List` read `s.now()`
internally to build the gate, so **their public signatures are unchanged** and
no caller in `tools.go` or the test harness needs updating for the gate itself.
Tests inject a fixed clock through the option to exercise the
active/scheduled/expired boundaries deterministically. (Chosen over threading a
`now` parameter or a context value: smallest blast radius, fully testable, and
keeps "current time" a store-construction concern rather than a per-call one.)

## New tools (`internal/server`)

### `schedule_memory` (write)

Same shape as `store_memory` plus:

- `not_before` (RFC3339 string, optional)
- `not_after` (RFC3339 string, optional)
- **At least one** of the two is required (otherwise it is just
  `store_memory`).
- `category` accepts the **same four curated values** as `store_memory`
  (`decision | preference | convention | gotcha`). `discovery` is **not**
  schedulable — discoveries are citation-backed maps/facts captured via a
  separate path, not time-gated records.

Validation (fail-closed, returns an MCP error):

- If both set, `not_before < not_after` (an empty/never-active window is rejected).
- `not_after`, if set, must be `> now` (creating an already-expired memory is
  rejected as a likely mistake).
- `not_before` may be in the past (harmless — record is immediately active).

Behavior: embeds `content` and stores the record exactly like `store_memory`,
with `NotBefore` / `NotAfter` populated. Server clock is the time authority;
`actor` / `owner` / `visibility` semantics are identical to `store_memory`.

### `list_scheduled` (read) — single tool

The owner's management view of their windowed memories. One parameter:

- `state` enum:
  - `scheduled` (**default**) — not-yet-active (`now < not_before`)
  - `expired` — past expiry (`now >= not_after`)
  - `all` — the **union** of the two sets above (scheduled **OR** expired);
    active windowed records are still excluded.

*Active* windowed memories are intentionally **not** listed here — they have
already surfaced transparently through normal `search_memory` / `list_memory`
(the agenda "wakes up" automatically). `list_scheduled` exists for the records
that the recall gate is *hiding*, so the owner can review, then `update_memory`
(e.g. reschedule) or `delete_memory` them.

**Store method — settled.** `list_scheduled` does **not** reuse `Store.List`:
after this change `List`/`listFilter` bake in the *active* gate and would
exclude exactly the records `list_scheduled` wants. It calls a new
`Store.ListScheduled(ctx, scope, subj, state, opts) ([]Memory, ...)` that
applies the **inverse** temporal condition while keeping the same outer
`Must:[scope, ownerOrSharedCondition]` authz envelope:

- `scheduled` → `NewRange("not_before", {Gt: now})` (lower gate still closed).
- `expired` → `NewRange("not_after", {Lte: now})` (upper gate already passed).
- `all` → `Should[` the two conditions above `]`.

Owner isolation and scope filtering are identical to `List`; only the temporal
clause is inverted. `now` comes from the same injected `s.now()` clock.

## Sweep command (`cmd/engram`)

`engram prune-expired [--older-than DUR]`:

- Deletes records where `not_after < now − grace` (default grace `0`).
- Opt-in, operator-run — **no always-on scheduler**. Soft-hide already happened
  at the read gate; this only reclaims storage for long-dead records.

**Store method — settled.** The existing `Delete` is by-ID and `DeleteAll` is
per-(scope, subject); neither does a condition-based bulk delete. `prune-expired`
calls a new `Store.PruneExpired(ctx, before time.Time) (deleted uint64, err error)`
that issues a single Qdrant filter-delete (`DeletePoints` with
`Filter{ Must:[ NewRange("not_after", {Lt: float64(before.Unix())}) ] }`). It is
an **operator/admin** operation: collection-wide, **no subject authz** (it runs
from the CLI against the whole collection, not on behalf of a caller), and does
not touch records without a `not_after` key. The CLI computes
`before = s.now() − grace` and reports the deleted count.

## Auth / isolation

The temporal gate is an extra `Must` layered on top of the existing
`ownerOrSharedCondition`. Private and `shared` records gate identically; a
`shared` scheduled memory is invisible to everyone (including other authed
callers) until active, and drops out for everyone once expired. No new authz
surface, no change to the anonymous-bucket or nil-subject fail-closed paths.

## Testing

- **Store unit tests (injected fixed clock):** active / scheduled / expired
  matrix across `Search` and `List`; absent-window records remain visible
  (backward-compat); by-id (`GetReadable` / `FetchForUpdate`) is ungated; the
  gate composes with owner/shared isolation (a scheduled `shared` record stays
  hidden until active). Boundary behavior: a record is active **at** exactly
  `not_before` (`Lte`) and **inactive at** exactly `not_after` (exclusive, the
  gate uses `Gt`). `ListScheduled` returns the correct set per `state`
  (`scheduled` / `expired` / `all` union) and never leaks another owner's
  records. `PruneExpired` deletes only records with `not_after < before` and
  leaves active/scheduled/unwindowed records intact.
- **Server tests:** `schedule_memory` validation (missing window, inverted
  window, past `not_after`, `discovery` category rejected); `list_scheduled`
  maps the `state` arg to `ListScheduled`; active windowed records appear in
  normal recall and are absent from `list_scheduled`.
- **CLI test:** `prune-expired` honors `--older-than` grace and reports the
  deleted count.

## Docs

- Update the CLAUDE.md "Memory contract (stable)" section: add `schedule_memory`
  / `list_scheduled` to the tool list and document the validity-window fields
  and the recall-gated / by-id-ungated rule.
- Update the docs-site reference for the tool surface and the
  `engram prune-expired` command.

## Out of scope (YAGNI)

- Push / notifications / cron / any server-initiated surfacing.
- Recurring or cron-style schedules.
- Auto-extraction of dates from memory content.
- A Qdrant payload index on the temporal fields (filters work without one at
  current scale; add later if profiling shows a need).
- Timezone handling beyond UTC normalization at the boundary.
