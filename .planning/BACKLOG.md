# engram — Backlog (inherited from beads)

**Migrated:** 2026-07-08 — engram moved off the beads issue tracker.

> **GitHub Issues is the source of truth for status.** This file is a one-time pointer
> index so GSD planning can see the inherited backlog and promote items into milestones
> via `/gsd-review-backlog`. Do **not** track status here — it will drift. When an item
> is promoted into a phase/milestone, check it off below and work it through its GitHub issue.
>
> All 20 active items carry the `from-beads` label on GitHub. The full historical export
> (1,037 issues incl. 1,017 closed + the migrated `bd remember` memories) is archived at
> `.planning/archive/beads-export-engram-2026-07-08.jsonl`.

## Active work — 20 items (GitHub Issues #301–#320)

### P2 — medium

- [ ] [#301](https://github.com/seanb4t/engram/issues/301) Renovate vendored-SPA postUpgradeTasks rule is inert — ui-drift keeps reddening main — `engram-38c6` _(was in-progress)_
- [ ] [#305](https://github.com/seanb4t/engram/issues/305) embed: native input_type/task_type passthrough for cloud embedders (Google/Cohere/Voyage/Jina) + document-side prefix for E5/nomic — `engram-wd89.1` _(was in-progress)_ · `feature`
- [ ] [#302](https://github.com/seanb4t/engram/issues/302) embed.Client.embed(): evaluate collapsing the two-path body-build (struct-marshal vs map-merge) into a single map-based path — `engram-43dp`
- [ ] [#303](https://github.com/seanb4t/engram/issues/303) storeDiscoveryArgs.ID jsonschema tag omits short_id despite skill doc claiming support — `engram-c0yl.12.1`
- [ ] [#304](https://github.com/seanb4t/engram/issues/304) embed package: export a shared reserved-param-key list so config.ParseEmbedParams can't silently desync from embedReq's wire contract — `engram-qom1`

### P3 — low

- [ ] [#306](https://github.com/seanb4t/engram/issues/306) PR #62 minor code-quality cleanups (suggestions) — `engram-17j`
- [ ] [#307](https://github.com/seanb4t/engram/issues/307) Decide SearchDiscoveries proto fidelity (kind/citations/summary dropped) — `engram-1hb`
- [ ] [#308](https://github.com/seanb4t/engram/issues/308) MintShortID: bounded attempt cap + exhaustion error — `engram-8em7`
- [ ] [#309](https://github.com/seanb4t/engram/issues/309) Qualify bare 'item N' spec references in short_id tests — `engram-b4a7`
- [ ] [#310](https://github.com/seanb4t/engram/issues/310) backfill CLI pure-function extraction (pattern parity) — `engram-b8ig`
- [ ] [#311](https://github.com/seanb4t/engram/issues/311) Pre-existing svelte-check errors in vendored shadcn primitives (input-group, sidebar) — `engram-btu5` · `bug`
- [ ] [#312](https://github.com/seanb4t/engram/issues/312) Consider wrapNotFoundWithOriginal helper for resolve+re-wrap sites — `engram-et64`
- [ ] [#313](https://github.com/seanb4t/engram/issues/313) Named returns for (id, shortID) on storeMemory/scheduleMemory/storeDiscovery — `engram-ghxx`
- [ ] [#314](https://github.com/seanb4t/engram/issues/314) Fix pre-existing local lint failures on main: rumdl (plans/*.md) + yamlfmt (ui/pnpm-lock.yaml) — `engram-h5xv` · `bug`
- [ ] [#315](https://github.com/seanb4t/engram/issues/315) storeDiscovery replace: avoid redundant second Get after OwnedOrAbsent — `engram-h7xe`
- [ ] [#316](https://github.com/seanb4t/engram/issues/316) Handler-level ErrAmbiguousShortID surfacing tests — `engram-q95d`
- [ ] [#317](https://github.com/seanb4t/engram/issues/317) Design: per-memory usage signals (hit/update/usage counts) — `engram-qx0d` · `feature`
- [ ] [#318](https://github.com/seanb4t/engram/issues/318) Backfill paging test at >1000 records per design matrix item 25 — `engram-wym4`
- [ ] [#319](https://github.com/seanb4t/engram/issues/319) backfill-short-ids CLI: extract+test RunE summary formatting — `engram-yb9f`

### P4 — backlog

- [ ] [#320](https://github.com/seanb4t/engram/issues/320) Async-on-write summary queue (auto-fill new records without an operator sweep) — `engram-4ivr` · `feature`

