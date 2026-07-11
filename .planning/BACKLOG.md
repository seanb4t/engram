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

## Backlog review — 2026-07-11 (`/gsd-review-backlog`)

Reviewed after Phase 13 (Embedder Reliability Foundation) merged to `main` (PR #348).
GitHub Issues + the `v0.10.x` milestone are the source of truth; this is a log snapshot.

- **Closed — delivered by Phase 13 (PR #348):** **#333** (`ENGRAM_EMBED_TIMEOUT` + re-derived
  summary-queue backoff) and **#332** (shape-aware base-URL → `/embeddings` join). #332's
  remaining operator-error edge cases (doubled `/embeddings`, query-string bases) live on in **#346**.
- **Promoted → `v0.10.x`** (12 issues):
  - Phase-13 code-review follow-ups: **#345** (reindex `--resume` skips tag-only changes → stale
    vector), **#346** (base-URL join malformed edge cases), **#347** (embed non-2xx discards
    provider error body).
  - from-beads code-quality leftovers folded into the correctness/CI tail (Phase 20/21): **#306,
    #309, #310, #312, #313, #315, #316, #318, #319**. These were marked `[x] → v0.9.x` in the
    2026-07-09 review below but were never actually completed (still open) — re-promoted here.
- **Deferred → next milestone (`v0.11.x` candidates, left un-milestoned)** — 5 newer feature ideas
  beyond v0.10.x's hardening/write-lane goal: **#340** (idempotency/upsert on `store_memory`),
  **#341** (structured provenance/citations on curated memories), **#342** (supersession links),
  **#343** (headless client transport), **#344** (`cross_spine` on `search_memory`). Surface at the
  next `/gsd-new-milestone`.
- **Untouched maintenance:** #301 (already `v0.10.x` → Phase 21), #311 / #314 (un-milestoned).
- **Removed:** none. (#155 Dependency Dashboard is a Renovate bot artifact — not tracked.)

## Backlog review — 2026-07-09 (`/gsd-review-backlog`)

Reviewed after v0.8.4 shipped (milestone v0.8.x complete). Outcome:

- **Promoted → GitHub milestone [`v0.9.x`](https://github.com/seanb4t/engram/milestone/1)** (21 issues):
  clusters B (embedder/ops), C (short_id polish), D (design/quality), plus two newly
  filed Phase-8 follow-ups — **#322** (Connect write-lane RPCs + CSRF) and **#323**
  (session refresh-token rotation). Formal scoping happens via `/gsd-new-milestone`;
  this milestone is the promotion target the issues are grouped under.
- **Maintenance — fixing now, outside any milestone** (cluster A, reddens main on every PR):
  **#314** (local lint gate: rumdl + yamlfmt), **#311** (svelte-check errors in vendored
  shadcn primitives), **#301** (Renovate vendored-SPA drift — root cause is the _external_
  homelab Renovate instance; in-repo self-healing fallback deferred, not a quick fix).
- **Removed:** none.

Checked items below (`[x]`) are promoted to `v0.9.x`; cluster-A items are annotated
_(maintenance)_ and remain unmilestoned. GitHub Issues + the milestone are the source
of truth for status.

## Active work — 20 items (GitHub Issues #301–#320)

### P2 — medium

- [ ] [#301](https://github.com/seanb4t/engram/issues/301) Renovate vendored-SPA postUpgradeTasks rule is inert — ui-drift keeps reddening main — `engram-38c6` _(maintenance — external root cause, in-repo fallback deferred)_
- [x] [#305](https://github.com/seanb4t/engram/issues/305) embed: native input_type/task_type passthrough for cloud embedders (Google/Cohere/Voyage/Jina) + document-side prefix for E5/nomic — `engram-wd89.1` · `feature` → **v0.9.x**
- [x] [#302](https://github.com/seanb4t/engram/issues/302) embed.Client.embed(): evaluate collapsing the two-path body-build (struct-marshal vs map-merge) into a single map-based path — `engram-43dp` → **v0.9.x**
- [x] [#303](https://github.com/seanb4t/engram/issues/303) storeDiscoveryArgs.ID jsonschema tag omits short_id despite skill doc claiming support — `engram-c0yl.12.1` → **v0.9.x**
- [x] [#304](https://github.com/seanb4t/engram/issues/304) embed package: export a shared reserved-param-key list so config.ParseEmbedParams can't silently desync from embedReq's wire contract — `engram-qom1` → **v0.9.x**

### P3 — low

- [x] [#306](https://github.com/seanb4t/engram/issues/306) PR #62 minor code-quality cleanups (suggestions) — `engram-17j` → **v0.9.x**
- [x] [#307](https://github.com/seanb4t/engram/issues/307) Decide SearchDiscoveries proto fidelity (kind/citations/summary dropped) — `engram-1hb` → **v0.9.x**
- [x] [#308](https://github.com/seanb4t/engram/issues/308) MintShortID: bounded attempt cap + exhaustion error — `engram-8em7` → **v0.9.x**
- [x] [#309](https://github.com/seanb4t/engram/issues/309) Qualify bare 'item N' spec references in short_id tests — `engram-b4a7` → **v0.9.x**
- [x] [#310](https://github.com/seanb4t/engram/issues/310) backfill CLI pure-function extraction (pattern parity) — `engram-b8ig` → **v0.9.x**
- [ ] [#311](https://github.com/seanb4t/engram/issues/311) Pre-existing svelte-check errors in vendored shadcn primitives (input-group, sidebar) — `engram-btu5` · `bug` _(maintenance)_
- [x] [#312](https://github.com/seanb4t/engram/issues/312) Consider wrapNotFoundWithOriginal helper for resolve+re-wrap sites — `engram-et64` → **v0.9.x**
- [x] [#313](https://github.com/seanb4t/engram/issues/313) Named returns for (id, shortID) on storeMemory/scheduleMemory/storeDiscovery — `engram-ghxx` → **v0.9.x**
- [ ] [#314](https://github.com/seanb4t/engram/issues/314) Fix pre-existing local lint failures on main: rumdl (plans/*.md) + yamlfmt (ui/pnpm-lock.yaml) — `engram-h5xv` · `bug` _(maintenance)_
- [x] [#315](https://github.com/seanb4t/engram/issues/315) storeDiscovery replace: avoid redundant second Get after OwnedOrAbsent — `engram-h7xe` → **v0.9.x**
- [x] [#316](https://github.com/seanb4t/engram/issues/316) Handler-level ErrAmbiguousShortID surfacing tests — `engram-q95d` → **v0.9.x**
- [x] [#317](https://github.com/seanb4t/engram/issues/317) Design: per-memory usage signals (hit/update/usage counts) — `engram-qx0d` · `feature` → **v0.9.x**
- [x] [#318](https://github.com/seanb4t/engram/issues/318) Backfill paging test at >1000 records per design matrix item 25 — `engram-wym4` → **v0.9.x**
- [x] [#319](https://github.com/seanb4t/engram/issues/319) backfill-short-ids CLI: extract+test RunE summary formatting — `engram-yb9f` → **v0.9.x**

### P4 — backlog

- [x] [#320](https://github.com/seanb4t/engram/issues/320) Async-on-write summary queue (auto-fill new records without an operator sweep) — `engram-4ivr` · `feature` → **v0.9.x**

## Also promoted (not `from-beads`)

Pre-existing open issues folded into `v0.9.x` in the same review:

- [x] [#261](https://github.com/seanb4t/engram/issues/261) search_memory: near-verbatim queries miss a highly-relevant record while topical neighbors dominate (phrasing-sensitive ranking) → **v0.9.x**
- [x] [#269](https://github.com/seanb4t/engram/issues/269) chart: ship the summarize-missing sweep CronJob in the Helm chart → **v0.9.x**

_(#155 Dependency Dashboard is a Renovate bot artifact — not tracked here.)_
