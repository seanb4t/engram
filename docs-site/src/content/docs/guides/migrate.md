---
title: Migrate (schema-version migrations)
description: Advance or roll back the server-stamped schema_version on every record with engram migrate, migrate status, migrate revert, and migration-status — the schema-version-driven migration mechanism. Not embedder reindexing, not owner remapping.
---

Every record engram writes carries a server-stamped `schema_version`: an
integer recording which schema shape the record's payload conforms to. As
this project evolves the record schema, `engram migrate` advances any
below-target record through the registered chain of steps that gets it
there. This guide covers that mechanism end to end: `engram migrate`,
`engram migrate status`, `engram migrate revert`, and the Connect-lane
`engram migration-status` sibling.

This is **not** about switching embedding models (see
[Reindex](/guides/reindex/)) and it is **not** about re-owning records after
an identity-provider claim change. Neither of those keys off
`schema_version`, and `migrate status` reports nothing about either of them.

## The mechanism

A record written before `schema_version` existed carries no such key at all
— it reads as version `0` by absence, identically to a record explicitly
stamped `0`. No backfill was ever needed for that to be true.

`internal/migrate` holds an ordered registry of steps, each declaring the
version it starts from, the version it produces, the payload keys it adds,
and whether it can be undone. Today the registry holds a single step: v0 to
v1, which mints a `short_id` for any record that lacks one.
`backfill-short-ids` is this step's first customer — its standalone command
is now a thin delegating alias onto the same sweep `engram migrate` runs,
not a separate procedure.

Two invariants are enforced at registration, not by convention: a step must
declare every payload key it adds, and its declaration is checked against
what it actually adds before any write happens; and a step must declare
whether it is reversible, with a nil declaration rejected outright. A step
that is silent about either is not a representable state in this codebase.

## The preview-and-apply contract

**`engram migrate` and `engram migrate revert` preview by default and mutate
only under `--apply`.** A bare invocation reports what the sweep would do
and writes nothing; add `--apply` to perform the mutation. `engram migrate
status` and `engram migration-status` are read-only and carry no `--apply`
flag at all.

This is the same preview/apply contract every command routed through the
shared destructive registration uses — see the [CLI guide's operator-tier
classification](/guides/cli/#destructive-commands) for the full roster and
for why a sibling group of non-destructive operator sweeps instead use an
opt-in `--dry-run` idiom. The two idioms coexist deliberately; this guide
does not re-argue that boundary.

Every `--apply` example below appears after this paragraph, never before it.

## `engram migrate`

```sh
engram migrate
```

Previews the full backlog: every record below the current target version,
across the whole collection. Nothing is created or written.

```sh
engram migrate --apply
```

Re-derives eligibility inside its own run — a fresh preview, then a write
pass — and migrates only the intersection of what the first preview showed
and what is still eligible at that moment. A record that became ineligible
since the preview (for example, a concurrent write already stamped it
current) is reported `spared`, not migrated. A record that became newly
eligible since the preview is reported `appeared`, also not migrated —
picking it up means re-running.

**Re-running is the normal recovery path, not a special procedure.** The
sweep is idempotent and safe to re-run against a freshly re-derived backlog.
**It is not resumable.** There is no persisted cursor: every pass re-derives
its backlog from scratch, with no offset carried from the previous pass.
There is no checkpoint file and no `--resume` flag, and there is nothing to
reconcile after an interruption — you re-run the same command.

### How convergence works

The write path stamps every record at the current target version before the
sweep ever runs, so an ordinary write arrives already-current and never
creates new below-target work. A successful migration write reduces the
backlog by one. Put together, ordinary traffic never works against the
sweep, and every write the sweep itself performs shrinks what remains.

That is not the same as saying the backlog is guaranteed to shrink on every
pass. The sweep carries a non-shrinking-backlog termination guard: if the
backlog does not shrink between two consecutive passes, the sweep stops and
reports rather than looping forever. Its message distinguishes two causes —
writes that failed to land, or a concurrent writer replenishing the backlog
at exactly the rate the sweep is draining it — because the two call for
different responses. Either way, re-running is still the right next step
once the underlying cause (a write failure, or the concurrent writer) is
resolved.

One more wrinkle specific to `--apply`: because it acts on the intersection
of a stale preview and a fresh re-derivation, its own reported `backlog`
truthfully includes any `appeared` records this run intentionally did not
touch. A non-zero `backlog` after an `--apply` run is not by itself a
failure signal — check `appeared` and `failed` before concluding anything
went wrong.

## `engram migrate status`

```sh
engram migrate status
```

Reports a version **distribution** across the collection, never a single
scalar. A mixed-version collection is a legitimate, expected mid-rollout
state — while a sweep is in progress, or before one has ever run — and a
single scalar would misreport it as broken.

The report's fields:

- `buckets` — one `{version, count}` entry per distinct version at or below
  this binary's current target, ascending.
- `absent` — records with no `schema_version` key at all (the legacy,
  pre-this-feature shape). Counted separately from `buckets` because a
  facet query cannot bucket on a missing key.
- `future` — one `{version, count}` entry per distinct version **above**
  this binary's current target, so an operator can tell one-version-ahead
  drift from a wildly newer binary apart, rather than seeing one blended
  total.
- `future_total` — the sum of `future`'s counts, a convenience derived from
  `future`, never the only place that population is reported.
- `total` — a fresh, exact whole-collection count, independent of the other
  fields — never inferred by summing them.
- `current_version` — the version this specific binary's registry produces
  when fully applied.

Seeing non-empty `future` buckets means some records were written by a
newer binary than the one you are running `migrate status` with; that is
informational, not an error, and those records are never touched by this
binary's `migrate --apply`.

## `engram migration-status`

```sh
engram migration-status --server https://engram.example.com --token "$TOKEN"
```

The Connect-lane sibling of `migrate status`: same histogram, reached over
the wire for an operator with a server URL and a bearer token but no direct
Qdrant access. It takes no arguments — the histogram is always a
whole-collection aggregate. This is the same `MigrateStatus` call the
startup warning and the operator console banner use, so all three surfaces
agree on what "pending" means.

## What is and is not automatic

**No migration ever applies automatically** — not on server startup, not on
failure. A failed `--apply` is never auto-reverted either: auto-reverting
would turn one partial write into two.

**What is automatic** is a read-only status probe at startup: the server
calls `MigrateStatus` once, bounded to ten seconds, and logs a warning if
pending work exists — a second, separate warning if future-version records
exist — without ever gating startup on the result. That probe is
structurally forbidden from invoking the sweep itself; nothing about it
writes to the collection.

If you have seen a "pending schema migrations exist" warning in your
server's startup log, that is this probe, not a migration that already ran.
Nothing runs `engram migrate` for you — you decide when, by running it
yourself.

## `engram migrate revert --to <v>`

```sh
engram migrate revert --to 0
```

Previews reverse-walking every record above `--to` back down to it, applying
each traversed step's declared inverse in reverse order. Nothing is written.

```sh
engram migrate revert --to 0 --apply
```

Applies the reverse walk. **The recovery path for an irreversible step is a
collection snapshot — `migrate revert` is not a general undo.** A step
declared irreversible, or a stored version this binary's registry has no
chain for, refuses the operation rather than silently skipping the
unreachable records.

That refusal is reachable in two different shapes, and they have different
consequences:

1. **Preflight refusal (the normal case).** Before anything is touched, a
   whole-range, zero-write preflight runs — twice, in fact: once when the
   command starts, and again inside the store as its own independent check.
   If the range contains any irreversible step or any unsupported stored
   version, the **whole** operation refuses, naming every offending step and
   version. Nothing was written.
2. **Race-discovered refusal.** A concurrent `engram migrate --apply` can
   land a new above-target record in the narrow window between the
   preflight finishing and the write loop's own re-scroll — a window that
   reopens on every pass, since the walk re-derives its backlog each time.
   When that happens, the write loop refuses on that single record, with the
   same typed refusal — but by then, earlier records in this same pass may
   already have been reverted. The report's `reverted` count in this case is
   real, not zero: the operation is not atomic. Re-run `engram migrate
   status` and take a fresh preview to reconcile. The way to avoid this
   window entirely is not to run a forward sweep concurrently with a
   revert.

`--timeout` matters more here than it does for `migrate`: `--apply` budgets
**two** full read-only whole-range scans — the command's own preflight, then
the store's independent second preflight — plus the write-convergence loop
itself, all under one `--timeout`. See the [CLI guide's timeout
budgeting](/guides/cli/#request-timeout) for the full picture across every
operator command.

## Flags

### `engram migrate`

| Flag | Default | Purpose |
|------|---------|---------|
| `--apply` | `false` | Perform the write pass. Absent, the command previews only. |
| `--output` | auto-detected | `json` or `text`; see the [CLI guide](/guides/cli/#operator-commands). |
| `--timeout` | `5m` | Wall-clock bound for the whole run; `0` disables the deadline. `--apply` budgets a fresh preview plus the write pass under this one value — see [Request timeout](/guides/cli/#request-timeout). |

### `engram migrate status`

| Flag | Default | Purpose |
|------|---------|---------|
| `--output` | auto-detected | `json` or `text`. |
| `--timeout` | `5m` | Wall-clock bound; `0` disables the deadline. |

### `engram migrate revert`

| Flag | Default | Purpose |
|------|---------|---------|
| `--to` | *(required)* | Target schema version to revert down to. Must be a non-negative integer at or below this binary's current target. |
| `--apply` | `false` | Perform the reverse write pass. Absent, the command previews only. |
| `--output` | auto-detected | `json` or `text`. |
| `--timeout` | `5m` | Wall-clock bound. `--apply` budgets **two** full-range preflight scans plus the write-convergence loop under this one value. |

## Output

Both mutating commands and `migrate status` render through the same
`--output json`/`text` split every operator command uses: `json` is the
stable contract, `text` is a human-readable rendering of the identical
document and may change wording in any release. Progress and warnings go to
stderr; the one machine-parseable document goes to stdout.

### `engram migrate` / `engram migrate --apply`

| Field | Meaning |
|-------|---------|
| `target` | The schema version this run advanced records toward. |
| `dry_run` | `true` for a preview, `false` for an applied run — an explicit boolean, never inferred from other fields. |
| `would_migrate` | The count of records the preview identified as eligible. Populated on both preview and applied runs, since `--apply` always previews internally first. |
| `migrated` | Records whose write succeeded this run. |
| `failed` | Records whose write returned an error this run. |
| `passes` | Outer re-derivation passes this call performed. |
| `backlog` | A fresh, post-run count of records still below target. On `--apply`, truthfully includes any `appeared` records this run intentionally did not touch. |
| `spared` | Count of previewed records no longer eligible by the time `--apply` ran (already advanced, or gone). |
| `appeared` | Count of records that became eligible after the preview and were therefore **not** migrated this run — re-run to include them. |

### `engram migrate status` / `engram migration-status`

| Field | Meaning |
|-------|---------|
| `buckets` | Per-version counts at or below `current_version`. |
| `absent` | Records with no `schema_version` key. |
| `future` | Per-version counts strictly above `current_version`. |
| `future_total` | Sum of `future`'s counts. |
| `total` | Fresh whole-collection count. |
| `current_version` | This binary's own current target version. |
| `pending` | `absent` plus every bucket **strictly below** `current_version` — buckets sitting *at* `current_version` and every `future` bucket are excluded, because `pending` answers "would running `engram migrate` do work?". Reported by `engram migrate status` (both `text` and `json`), by `engram migration-status`, and by the Connect `MigrateStatusResponse` (field 7); all three read the same server-side `MigrateStatusResult.Pending()`, never a re-derivation. |

The Connect lane also differs in how it renders numbers: protojson encodes
`uint64` fields as JSON **strings**, not numbers, so a script consuming both
lanes needs two different numeric parsers — one for the CLI's `json` output,
one for `migration-status`'s.

### `engram migrate revert` / `engram migrate revert --apply`

| Field | Meaning |
|-------|---------|
| `to` | The target version this run reverted toward. |
| `applied` | `true` only for a successful applied run. `false` for a preview and for a refusal (even a refusal that reverted some records before it fired — see below). |
| `reversible` | Whether the whole range preflighted clean. `false` means every count field below reports what already happened before the refusal, not what the operation as a whole accomplished. |
| `candidates` | Records above `--to` this run's preflight observed. |
| `reverted` | Records successfully reverted. On a race-discovered refusal, this can be non-zero even though `applied` is `false` — records already reverted before the mid-loop refusal fired. |
| `failed` | Records whose reverse write returned an error. |
| `passes` | Outer re-derivation passes performed. |
| `backlog` | A fresh, post-run count of records still above `--to`. |
| `refusal` | Present only when `reversible` is `false`: the exact refusal text, naming every offending irreversible step and unsupported version. Identical wording whether read from `json` or from the `text` rendering's stderr. |

## See also

- [CLI guide](/guides/cli/) — the shared operator-tier contracts this guide
  leans on rather than restates: [destructive command
  classification](/guides/cli/#destructive-commands), [`--output`](/guides/cli/#operator-commands),
  [exit codes](/guides/cli/#exit-codes), and [`--timeout`
  budgeting](/guides/cli/#request-timeout).
- [Record reference](/reference/memory-record/) — the `schema_version` field
  contract on a stored record.
- [Upgrade guide](/guides/upgrade/) — the release note covering this
  mechanism's rollout and the binary-rollback hazard it introduced.
- [Reindex](/guides/reindex/) — a different mechanism entirely: reindexing
  re-embeds into a new collection for a new embedding model and copies
  payloads verbatim, so it never advances a record's `schema_version`.
