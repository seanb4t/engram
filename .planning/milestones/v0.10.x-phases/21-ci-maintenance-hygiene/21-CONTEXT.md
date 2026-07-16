# Phase 21: CI / Maintenance Hygiene - Context

**Gathered:** 2026-07-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Stop the CI pipeline and the local lint gate from generating false-positive red builds, so
real signal isn't drowned out by noise. Three independent hygiene gaps, each landing on its
own. Purely maintenance — **no new capabilities, no product surface, no runtime behavior
change**. Independent of every other track; can land any time.

1. **#301 (REQ-ci-renovate-spa-drift)** — a Renovate bump to `ui/` no longer reddens `main`:
   an in-repo self-healing fallback replaces the inert `postUpgradeTasks` rule.
2. **#335 (REQ-p11-review-residuals)** — the three deferred Phase-11 async-summary review
   residuals are resolved: WR-03 (`Wait()` misuse hazard), IN-01 (duplicated
   Upsert→`tryEnqueue` block), IN-02 (test hermeticity).
3. **REQ-lint-planning-exclude** (no GH issue) — `.rumdl.toml` excludes `.planning`, so
   `task lint:markdown` (and therefore `task` default) passes, while shipped Markdown
   outside `.planning/` stays linted.

</domain>

<decisions>
## Implementation Decisions

### Scope corrections found during scouting (read these first)

- **D-00a: ROADMAP SC2 mislabels IN-01.** `.planning/ROADMAP.md:467` describes IN-01 as
  "duplicate depth-gauge registration". **That is wrong.** Both authoritative sources — the
  archived review (`.planning/milestones/v0.9.x-phases/11-async-on-write-summaries/11-REVIEW.md:265`)
  and GitHub issue #335 — define IN-01 as *`storeMemory` and `scheduleMemory` duplicate the
  Upsert-then-`tryEnqueue` block verbatim*. The depth gauge is registered exactly once
  (`internal/server/tools.go:255`, via `telemetry.RegisterSummaryQueueDepth`); there is no
  duplicate registration to fix. **The review + issue win.** Implement the real IN-01 and
  correct the ROADMAP SC2 wording in the same phase.
- **D-00b: the rumdl blast radius is 1505 issues, not 331** (ROADMAP SC3's figure is stale —
  more planning docs have landed since it was written). Measured 2026-07-15:
  `rumdl check .` → `Found 1505 issues in 155/354 files`. **All 1505 are under `.planning/`;
  zero are outside it.** SC3's "shipped Markdown outside `.planning/` is still linted" is
  therefore already true today and is provable by `rumdl check .` exiting 0 after the
  exclude lands — no separate guard needed.
- **D-00c: `usageQueue` is a verbatim mirror of `summaryQueue`'s kernel** and carries the
  identical WR-03 hazard (`internal/server/usagequeue.go:37-41` `inFlight sync.WaitGroup`,
  `:194` `Wait()`). WR-03 was written during Phase 11, before `usageQueue` existed, so the
  issue text names only `summaryQueue`. Treat the mirror as in scope (D-04).

### Renovate vendored-SPA drift (#301)

- **D-01:** Take **option (b) from the issue — the self-healing CI fallback.** The `ui-drift`
  job (`.github/workflows/ci.yaml:155-177`) already rebuilds the SPA into
  `internal/webauth/static/` and then *fails* on `git diff --exit-code`. On a Renovate-authored
  branch it should instead **commit and push the regenerated output and pass**. Option (a)
  (fix the homelab Renovate instance: `allowedPostUpgradeCommands` /
  `RENOVATE_ALLOWED_POST_UPGRADE_COMMANDS` + `binarySource:install`) is **external to this
  repo** — it cannot be implemented, tested, or verified by CI here, so it is not this
  phase's deliverable. Fixing the bot later is complementary, not a substitute.
- **D-02:** **Keep the existing `postUpgradeTasks` rule** in `.github/renovate.json` rather
  than deleting it. It is inert but harmless, and if the homelab bot is ever fixed it wins
  first — CI then finds no drift and does nothing. Belt-and-suspenders, no conflict.
- **D-03:** The self-heal path must be **narrowly gated and least-privilege**:
  - Repo-wide `permissions: contents: read` (`ci.yaml:9-10`) **stays as-is**; write is
    granted at the **`ui-drift` job level only**.
  - The commit+push step is guarded to Renovate-authored, **same-repo** PR branches — a fork
    PR must never reach it. The exact guard expression (head-ref prefix `renovate/`, actor,
    and `github.event.pull_request.head.repo.full_name == github.repository`) is the
    researcher's to pin down against current GitHub semantics.
  - Non-Renovate branches keep the **current fail-with-guidance behavior** verbatim
    (`::error::vendored SPA is stale — run 'task ui:build' and commit`). Humans are told; only
    the bot is healed.
  - **`main` pushes must never self-heal** — the job is PR-scoped for this path.

**RESEARCH-CRITICAL for #301** (do not let the planner hand-wave these):

  - **`GITHUB_TOKEN` pushes do not trigger new workflow runs** (GitHub's recursion guard).
    Combined with `platformAutomerge: true` (`renovate.json`), a self-heal commit could leave
    the new head SHA with **no checks at all**, and GitHub auto-merge would stall on
    "Expected" forever — trading a red build for a wedged PR. The researcher MUST resolve how
    the self-heal commit gets validated/merged: the current job exiting 0 after pushing, a
    PAT/app token that does re-trigger, or another mechanism. **This is the make-or-break
    design question of #301, not a detail.**
  - **Renovate's reaction to a third-party commit on its branch** — Renovate may treat the
    branch as user-modified (and stop updating it) or rebase and discard the commit,
    depending on `rebaseWhen` under `config:best-practices`. Confirm the resulting behavior
    and whether a `rebaseWhen` setting is needed.
  - **Branch protection / rulesets** — `main` is protected (`protect-main`); confirm a
    bot push to a *PR branch* is unaffected.

### Phase-11 residuals (#335)

- **D-04: WR-03 — make the misuse structurally impossible, and fix both queues.** Do not
  settle for the doc-comment option. Evidence closes this: **every** `Wait()` caller in the
  repo is a `_test.go` file (5 in `summaryqueue_test.go`, 4 in `usagequeue_test.go`, 1 in
  `connectapi_test.go:875`) — there is no production caller to preserve. So move `Wait()`
  out of production reach (test-only file / build-tag-guarded helper, per the review's own
  "not exposing `Wait()` outside `_test.go`" suggestion) for **both** `summaryQueue` and
  `usageQueue` (D-00c). A comment asks future callers to behave; removing the path
  guarantees it. Exact mechanism (e.g. `export_test.go` convention) is Claude's call.
- **D-05: IN-01 — extract the shared post-write helper** for the duplicated
  MintShortID → Upsert → `tryEnqueue` sequence, shaped roughly as the review suggests:
  `d.persistAndEnqueue(ctx, m, vec) (id, shortID string, err error)`. Called from
  `storeMemory` and `scheduleMemory` **only** — `storeDiscovery`/`storeRule` deliberately do
  **not** enqueue (discoveries own their own summaries, per the D-06 note at
  `internal/server/tools.go:706`); do not fold them in. Preserve the existing
  "enqueue only after a confirmed-successful Upsert" ordering exactly.
- **D-06: IN-02 — tighten test hermeticity** in `TestBuildDepsFromEnvLoadsConfigOnce`
  (`internal/server/tools_test.go`): `t.Setenv("ENGRAM_SUMMARY_MODEL", "")` and
  `t.Setenv("ENGRAM_SUMMARY_ON_WRITE", "")` alongside the existing `t.Setenv` calls, matching
  the pattern in `config_test.go`'s `TestLoadDefaults`, so an ambient env can never start an
  unshut-down queue (2 leaked worker goroutines for the test binary's lifetime).
- **D-07 [informational]:** WR-01 stays closed — a decision to take **no action**, so no plan
  covers it (nothing to implement). The worker pool's `context.Background()` was reviewed and
  **accepted as a non-issue** in #335 (`serve.go` exits immediately after `drainSummaries`;
  each fill is bounded by a per-attempt `context.WithTimeout`). Do not reopen it.
- **D-08: line numbers in #335 and 11-REVIEW.md have drifted** — the review cites
  `summaryqueue.go:123-139,254-263` and `tools.go:562-582,594-621`, but the code has moved
  (`tryEnqueue` now at `summaryqueue.go:148`, `inFlight.Add` at `:168`; the `tryEnqueue`
  call sites now at `tools.go:665` and `:701`). Re-locate by symbol, never by line number.

### rumdl planning exclude (REQ-lint-planning-exclude)

- **D-09:** Add **`.planning`** — the plain directory name — to the existing `exclude` array
  in `.rumdl.toml`, matching the established convention of its neighbors (`.git`, `.beads`,
  `.claude`, `dist`, `docs-site`). **Not** the `.planning/**` glob form that ROADMAP SC3's
  prose implies; the researcher should confirm rumdl's exclude matching semantics if the
  plain form doesn't take. Add a short comment in the same style as the neighboring entries
  explaining *why* (GSD planning artifacts: agent-generated, not shipped prose).
- **D-10:** Acceptance is mechanical and binary: **`task lint:markdown` exits 0**, which
  unblocks `task` default (lint + test) — blocked since Phase 20 (memory `kwp5wq89bq`).
  Because zero issues exist outside `.planning/` (D-00b), a clean `rumdl check .` proves both
  halves of SC3 at once. No new test or guardrail is warranted.

### Landing strategy

- **D-11:** The planner **splits by subsystem into 3 plans**, one per requirement/issue, each
  closing its own GitHub issue with an atomic commit (the Phase-20 D-10 pattern). The three
  are fully file-disjoint and can parallelize:
  - **Plan A — rumdl exclude** (`.rumdl.toml`): REQ-lint-planning-exclude. One-line config
    change. **Land this first** — it unblocks `task` default for every subsequent plan's
    own quality gate.
  - **Plan B — Phase-11 residuals** (`internal/server/summaryqueue.go`, `usagequeue.go`,
    `tools.go`, `tools_test.go`, `summaryqueue_test.go`, `usagequeue_test.go`): #335,
    D-04/D-05/D-06.
  - **Plan C — Renovate self-heal** (`.github/workflows/ci.yaml`, possibly
    `.github/renovate.json`): #301. Highest-uncertainty plan — gated on the
    RESEARCH-CRITICAL items under D-03.
  - Plan A also unblocks Plan B/C's ability to run the full local gate, so it is a soft
    ordering dependency rather than a hard one. Final boundaries follow the planner's
    file-overlap analysis.
- **D-12: correct ROADMAP SC2 + SC3 in this phase** — SC2's IN-01 description (D-00a) and
  SC3's stale "331-failure" figure (D-00b). Fold into whichever plan is convenient; the
  ROADMAP is the acceptance list downstream agents verify against, so leaving it wrong
  guarantees a verifier mismatch.

### Claude's Discretion

- All three items are mechanical hygiene with **no user-facing decision**. Within the
  decisions above, implementation detail is Claude's call: the test-only `Wait()` mechanism
  and file naming (D-04), the `persistAndEnqueue` helper's exact signature and name (D-05),
  the `.rumdl.toml` comment wording (D-09), and the workflow step/guard expression phrasing
  (D-03) — all consistent with existing repo conventions.
- **Exception — #301 is NOT discretionary in one respect:** if the RESEARCH-CRITICAL
  token/auto-merge question (D-03) has no clean answer, **stop and surface it** rather than
  shipping a self-heal that wedges Renovate PRs on "Expected" checks. A wedged PR is worse
  than the red build it replaces.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & roadmap

- `.planning/REQUIREMENTS.md:66-68` — REQ-ci-renovate-spa-drift (#301), REQ-p11-review-residuals
  (#335), REQ-lint-planning-exclude (no issue).
- `.planning/ROADMAP.md:459-471` §"Phase 21: CI / Maintenance Hygiene" — goal + 3 success
  criteria (the acceptance list). **Read with D-00a and D-00b: SC2's IN-01 description and
  SC3's "331-failure" figure are both wrong and are corrected this phase (D-12).**
- GitHub issues: **#301** (Renovate vendored-SPA drift), **#335** (Phase-11 residuals).
  REQ-lint-planning-exclude has no issue — it is roadmap-only.

### #301 — Renovate vendored-SPA drift

- `.github/workflows/ci.yaml:155-177` — the `ui-drift` job. Already rebuilds the SPA
  (`cd ui && pnpm install --frozen-lockfile && pnpm build` → `internal/webauth/static/`);
  `:176` is the `git diff --exit-code` that fails. The self-heal step goes here.
- `.github/workflows/ci.yaml:3-10` — `on:` triggers (`pull_request: {}` + push to `main`) and
  the repo-wide `permissions: contents: read` that must stay read (D-03).
- `.github/renovate.json` — the inert `postUpgradeTasks` rule (kept per D-02), plus
  `platformAutomerge: true` and `extends: ["config:best-practices"]` — both load-bearing for
  the RESEARCH-CRITICAL auto-merge question (D-03).
- `internal/webauth/static/` — the vendored SPA build output under drift check.
- `Taskfile.yaml` — `ui:build` (the manual regen operators are told to run today).

### #335 — Phase-11 residuals

- `.planning/milestones/v0.9.x-phases/11-async-on-write-summaries/11-REVIEW.md:238-289` —
  **the authoritative definition of WR-03 / IN-01 / IN-02**, each with its recommended fix.
  Note the path: phase 11 was archived on milestone completion, so the
  `.planning/phases/11-async-on-write-summaries/` path cited in issue #335 **no longer
  exists**. Line numbers in the review have drifted (D-08).
- `internal/server/summaryqueue.go` — `inFlight sync.WaitGroup` (`:63`), `tryEnqueue` (`:148`,
  `inFlight.Add(1)` at `:168`), `itemDone` (`:259`), `Shutdown` (`:282`), `Wait()` (`:309`),
  `depth()` (`:320`).
- `internal/server/usagequeue.go` — the verbatim mirror (D-00c): `inFlight` (`:41`),
  `tryEnqueue` (`:71`), `Wait()` (`:194`).
- `internal/server/tools.go` — `buildSummaryQueue` (`:227`), the single depth-gauge
  registration (`:255` — *not* duplicated, per D-00a), the two `tryEnqueue` call sites at
  `:665` (`storeMemory`) and `:701` (`scheduleMemory`), and the discoveries-own-their-summaries
  note at `:706` (why `storeDiscovery` is excluded from the D-05 helper).
- `internal/server/tools_test.go` — `TestBuildDepsFromEnvLoadsConfigOnce` (IN-02 target).
- `internal/config/config_test.go` — `TestLoadDefaults`, the env-clearing pattern IN-02 copies.
- `internal/server/summaryqueue_test.go` / `usagequeue_test.go` / `connectapi_test.go:875` —
  all 10 `Wait()` call sites; the evidence base for D-04.
- `internal/telemetry/metrics.go` — `RegisterSummaryQueueDepth`.

### REQ-lint-planning-exclude

- `.rumdl.toml` — the `exclude` array (`.git`, `.worktrees`, `.beads`, `.agents`, `.claude`,
  `.codex`, `dist`, `node_modules`, `vendor`, `CHANGELOG.md`, `docs-site`) that `.planning`
  joins, plus `respect-gitignore = true` (`.planning/` is committed, hence linted today).
- `Taskfile.yaml:74-76` — `lint:markdown` (`rumdl check .`); `:63` — the `default`/`lint`
  task that aggregates it and is blocked today.

### Codebase maps & prior context

- `.planning/codebase/CONVENTIONS.md`, `.planning/codebase/TESTING.md` — repo conventions for
  Go test hygiene and CI gates.
- `.planning/phases/20-correctness-polish/20-CONTEXT.md` §D-10 — the by-subsystem plan-split
  pattern D-11 follows.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **The `ui-drift` job already does the whole rebuild** (`ci.yaml:172-175`) — #301 is not
  "teach CI to build the SPA", it is "let the existing rebuild commit its output on bot
  branches". The delta is a guard + a commit/push step + a job-level permission, not a new job.
- **`itemDone()` / `Shutdown()` shutdown kernel** (`summaryqueue.go:259/282`) — WR-03 touches
  only `Wait()`'s reachability; the RWMutex closed-guard that fixed CR-01 is untouched.
- **`config_test.go`'s `TestLoadDefaults` env-clearing pattern** — IN-02's fix is a
  copy of an already-established in-repo pattern, not a new idea.

### Established Patterns

- **Per-subsystem plans, one issue per atomic commit** (Phase 20 D-10) — D-11 reuses it.
- **`.rumdl.toml` excludes are plain directory names with a why-comment** — `.planning`
  follows `docs-site` and `.beads` exactly.
- **Least-privilege workflow permissions** — repo-wide `contents: read` with per-job
  escalation only where needed; `commit-lint` (`ci.yaml:214`) is the in-repo precedent for a
  job-level `permissions:` block.

### Integration Points

- **#301 is the only item with an external dependency and real unknowns** (GitHub token
  push semantics × Renovate rebase behavior × `platformAutomerge`). #335 and the rumdl
  exclude are self-contained and low-risk.
- **The rumdl exclude unblocks `task` default**, which every other plan's quality gate runs —
  a soft ordering dependency (D-11 Plan A first).
- **No runtime/product surface is touched by any of the three.** No proto change, no wire
  change, no Helm change, no new deps → no console re-vendor, no `gen/` drift, no
  new attack surface.

</code_context>

<specifics>
## Specific Ideas

- **#301** — self-healing CI fallback (issue option **b**), *not* the external homelab fix
  (option a). Keep the inert `postUpgradeTasks` rule anyway.
- **WR-03** — make misuse **structurally impossible** (test-only `Wait()`), not a
  doc-comment; apply to **both** `summaryQueue` and `usageQueue`.
- **IN-01** — the ROADMAP's "duplicate depth-gauge registration" is a **mislabel**; the real
  finding is the `storeMemory`/`scheduleMemory` duplication. Correct the ROADMAP.
- **rumdl** — plain `.planning` exclude; all 1505 issues are inside it, zero outside.

</specifics>

<deferred>
## Deferred Ideas

- **Fixing the homelab Renovate instance** (issue #301 option a: `allowedPostUpgradeCommands`
  / `RENOVATE_ALLOWED_POST_UPGRADE_COMMANDS` + node/pnpm via `binarySource:install`) — the
  real root cause, but it lives outside this repo and cannot be verified by engram's CI.
  Complementary to D-01, not a substitute; belongs in homelab infra work.
- **Adding `edited` to `ci.yaml`'s `pull_request` trigger types** — the optional repo fix from
  memory `d8rjr4zqva` that would let a `/gsd-ship` PR-title correction re-trigger commit-lint
  without a close/reopen dance. Genuinely CI-hygiene-shaped and tempting to fold in here, but
  it is **not** in any of Phase 21's three requirements — out of scope. Worth its own issue.
- **The remaining from-beads refactor cluster** (#306, #309, #310, #312, #313, #315, #316,
  #318, #319) — `.planning/REQUIREMENTS.md:74` marks these opportunistic. Phase 21 touches
  `internal/server/tools.go` (D-05); if any cluster issue lands in the same block, the
  planner may fold it — otherwise leave it.

</deferred>

---

*Phase: 21-ci-maintenance-hygiene*
*Context gathered: 2026-07-15*
