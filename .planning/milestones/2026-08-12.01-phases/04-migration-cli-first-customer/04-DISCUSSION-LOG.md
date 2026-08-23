# Phase 4: Migration CLI & First Customer - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-14
**Phase:** 04-migration-cli-first-customer
**Areas discussed:** Minting under pure ApplyFunc, Command layout & naming, Backfill alias reconciliation, Revert UX surface

---

## Minting under pure ApplyFunc

### Q1: How should the v0→v1 backfill-short-ids step mint collision-safe short_ids within the pure-ApplyFunc architecture?

| Option | Description | Selected |
|--------|-------------|----------|
| Step-declared minter injection | Step declares it needs a minter; the sweep injects a per-Migrate-call mint closure capturing store+ctx+a fresh seen set | ✓ |
| Sweep special-cases short_id | The sweep hard-codes short_id minting knowledge | |
| Pure mint + post-sweep audit | Mint without global collision checks, audit afterward | |

**User's choice:** Step-declared minter injection (Recommended)
**Notes:** Keeps MintShortID's global Count guarantee; keeps ApplyFunc pure for all other steps; follows D-11's optional-capability precedent.

### Q2: What API shape should the minter-aware step take in internal/migrate?

| Option | Description | Selected |
|--------|-------------|----------|
| Second constructor NewMintingStep | `NewMintingStep(from, to, addsKeys, rev, applyMinter)` with `ApplyMinterFunc`; Step carries exactly one apply path, constructor-enforced | ✓ |
| Builder modifier + sentinel apply | Modifier method sets a second apply path; sentinel marks it | |
| Minter via MigrateOptions | Thread the minter through sweep options | |

**User's choice:** Second constructor NewMintingStep (Recommended)
**Notes:** No representable nil-apply state. Sweep branches on which is present; Registry literal stays self-describing.

### Q3: Should the v0→v1 backfill-short-ids step be declared Reversible or Irreversible?

| Option | Description | Selected |
|--------|-------------|----------|
| Reversible — inverse deletes short_id | Inverse removes the minted key | |
| Irreversible — short_ids are citable | Minted short_ids may already be cited by agents; deletion orphans external references | ✓ |

**User's choice:** Irreversible — short_ids are citable (Recommended)
**Notes:** Reason names snapshot recovery. Accepted consequence: REQ-migrate-revert refuses on the only chain link that exists until a later reversible step lands.

### Q4: How should the sweep's injected mint closure check collisions?

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse MintShortID per candidate | `mint = func() { return s.MintShortID(ctx, seen) }`, fresh seen map per Migrate call, built lazily | ✓ |
| Per-pass pre-fetch into seen | One scroll per pass pre-loads all existing short_ids | |

**User's choice:** Reuse MintShortID per candidate (Recommended)
**Notes:** N extra Counts joins PA-14 as accepted large-collection debt, documented, never silently optimized. Pre-fetch deferred (see Deferred Ideas).

---

## Command layout & naming

### Q1: How should the migrate command tree be shaped?

| Option | Description | Selected |
|--------|-------------|----------|
| Subcommands: status + revert | `engram migrate` (sweep), `engram migrate status`, `engram migrate revert --to <v>` | ✓ |
| Flags on one command | `--status` / `--apply` / `--revert` flags on a single `migrate` command | |

**User's choice:** Subcommands: status + revert (Recommended)
**Notes:** One toolclass row per space-joined path; flag sets stay disjoint; spine-review-purge nested precedent. Rejected flags: --status (read-only) and --apply (destructive-tier) would share one row keyed "migrate"; mode dispatch fights registerDestructive's RunE ownership.

### Q2: Where should the new migrate command family live?

| Option | Description | Selected |
|--------|-------------|----------|
| New file for the family | Family gets its own file under cmd/engram/ | ✓ |
| Fold into migrate.go | Add to the existing owner-migration file | |

**User's choice:** New file for the family (Recommended)
**Notes:** File-per-command-family convention; permanent schema-migration infrastructure separate from transient owner-migration commands (one already deprecated).

### Q3: What Destructive class should the `migrate` sweep row carry?

| Option | Description | Selected |
|--------|-------------|----------|
| Additive reasoning: sweep=false | Additive-only ENFORCED (CheckAdditive; per-point SetPayload of added keys only) — same reasoning as the backfill-short-ids row | ✓ |
| Conservative: sweep=true | Classify destructive because it writes | |

**User's choice:** Additive reasoning: sweep=false (Recommended)
**Notes:** `migrate status` ReadOnly:true; `migrate revert` Destructive:true (inverses may remove keys — the anti-additive direction). One operation never carries two classifications once the alias delegates.

### Q4: How should `migrate status` compute the per-version histogram?

| Option | Description | Selected |
|--------|-------------|----------|
| Store method, server-side agg | New store method: facet counts on schema_version + one exact Count with IsEmpty(schema_version) for the absent bucket | ✓ |
| CLI-side scroll aggregation | CLI scrolls the collection and buckets client-side | |

**User's choice:** Store method, server-side agg (Recommended)
**Notes:** Facet can't bucket absent keys (4syx1ggfxk). Per-version exact-Count loop is an equally valid planner fallback. Rejected CLI scroll: O(collection) transfer, duplicates absent-key semantics.

---

## Backfill alias reconciliation

### Q1: What happens to backfill-short-ids' --dry-run flag when it becomes a preview-by-default alias?

| Option | Description | Selected |
|--------|-------------|----------|
| Remove --dry-run outright | Flag deleted; alias routes through registerDestructive (preview default, --apply executes) | ✓ |
| Keep --dry-run, invert default | Flag stays with default flipped | |

**User's choice:** Remove --dry-run outright (Recommended)
**Notes:** Identical UX to migrate-remap-owner ("REMOVED, not deprecated — see the upgrade guide"). The behavioral break (bare `backfill-short-ids` previously applied, now previews) is what the upgrade-guide entry documents.

### Q2: Should the alias delegate to Store.Migrate and remove the old sweep, or keep BackfillShortIDs as its backing?

| Option | Description | Selected |
|--------|-------------|----------|
| Delegate to Migrate, delete old sweep | Alias calls Store.Migrate with defaults; Store.BackfillShortIDs and missingShortIDFilter deleted as dead code | ✓ |
| Alias keeps BackfillShortIDs | Old sweep stays as the alias's backing | |

**User's choice:** Delegate to Migrate, delete old sweep (Recommended)
**Notes:** MintShortID stays (write path + sweep mint closure). One migration code path; fulfills the REQ's "thin delegating alias" and e8k7mxb1v6's demotion.

### Q3: Which JSON report envelope should the deprecated alias emit?

| Option | Description | Selected |
|--------|-------------|----------|
| Shared migrate envelope | Same MigrateResult-shaped document as `engram migrate` + explicit dry_run bool | ✓ |
| Preserve legacy envelope | Keep the backfill-specific report shape | |

**User's choice:** Shared migrate envelope (Recommended)
**Notes:** migrateRemapReportDoc precedent — explicit fields, never prose-inferred. Compat value of the legacy envelope is marginal because behavior is already breaking.

### Q4: What shape should the upgrade-guide gate take?

| Option | Description | Selected |
|--------|-------------|----------|
| Bidirectional doc↔code gate | One test asserts BOTH the upgrade-guide entry AND the command's missing --dry-run flag + Deprecated pointer | ✓ |
| Doc-only gate | Test asserts only the guide entry | |

**User's choice:** Bidirectional doc↔code gate (Recommended)
**Notes:** x6v6qxqd6f vacuous-gate lessons: exact strings, prove-RED by reverting either side, never a len>0 proxy.

---

## Revert UX surface

### Q1: When should `migrate revert --to <v>` evaluate irreversibility — preflight over the whole range, or as it walks?

| Option | Description | Selected |
|--------|-------------|----------|
| Whole-range preflight refusal | Evaluate the full range BEFORE any write; any irreversible step refuses the entire revert with zero records touched | ✓ |
| Revert up to first irreversible | Apply inverses top-down, stop at the first irreversible step | |

**User's choice:** Whole-range preflight refusal (Recommended)
**Notes:** e8k7mxb1v6's exact wording — refuse rather than revert partially. The check is pure (walk the chain, inspect Reversibility). Rejected partial: leaves the collection at a version nobody asked for with inverses already written.

### Q2: What should the irreversibility refusal message contain?

| Option | Description | Selected |
|--------|-------------|----------|
| Step + reason + snapshot path | Names every irreversible step (From/To), each declared reason, and snapshot recovery as the path back | ✓ |
| Minimal step name only | First offending step's identity, nothing else | |

**User's choice:** Step + reason + snapshot path (Recommended)
**Notes:** field=<name> hint=<code> envelope, machine-stable. The declared reason exists precisely to be surfaced here (D-03 panics to make it non-empty).

### Q3: How should the revert sweep's write path be structured?

| Option | Description | Selected |
|--------|-------------|----------|
| Dedicated Store.Revert | Walks the chain in reverse applying declared inverses per record with per-point DeletePayload/SetPayload | ✓ |
| Direction param on Migrate | One sweep method, internal forward/reverse branch | |

**User's choice:** Dedicated Store.Revert (Recommended)
**Notes:** CheckAdditive is additive-specific; a direction param would need an exemption, quietly disabling the enforcement this milestone exists to build. Same re-derive-per-pass loop shape; resume = call Revert again.

### Q4: Should `migrate revert` get the same preview/apply parity as the sweep?

| Option | Description | Selected |
|--------|-------------|----------|
| Full parity via registerDestructive | Preview surfaces the reverse plan (steps to invert, records at each version, preflight result); --apply executes | ✓ |
| No preview, preflight suffices | Execute immediately; preflight is protection enough | |

**User's choice:** Full parity via registerDestructive (Recommended)
**Notes:** `migrate revert` is Destructive:true; a Destructive:true row outside registerDestructive breaks the one-routing-mechanism invariant. Preflight answers "can this range revert at all", not "what will this revert touch".

---

## Claude's Discretion

- Exact filename for the new migrate-family file under `cmd/engram/`.
- The status store method's name and facet-vs-exact-Count-loop implementation (D-08 leaves both open).
- Report document field ordering and text-renderer wording, subject to existing precedents.
- Where the lazy seen-map construction lives inside `Store.Migrate`.
- The startup warning's exact wording and placement alongside `warnOwnerlessRecords`.

## Deferred Ideas

- Per-pass pre-fetch of all existing short_ids into the seen set, then mint purely against it (one payload-light scroll per pass instead of N exact Counts) — large-collection optimization, deferred alongside PA-14; reintroduces a theoretical read-after-write collision window so it is only acceptable under the offline/operator-run deployment shape.
- PA-14 itself (the sweep's O(passes × backlog) exact Count) — documented in the operator guide this phase, fixed never-silently later.

---

*Phase: 04-migration-cli-first-customer*
*Discussion log generated: 2026-08-14*
