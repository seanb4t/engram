# Retrospective — engram

Living retrospective across milestones. Newest milestone section first;
cross-milestone trends at the bottom.

---

## Milestone: v0.9.x — Recall Quality

**Shipped:** 2026-07-10 (PR #336)
**Phases:** 4 (9–12) | **Plans:** 12 | **Tasks:** 27 | **Requirements:** 6/6

### What Was Built

A labeled retrieval-quality eval harness (`task eval:retrieval`) with a permanent #261
regression fixture; always-on `search_memory` similarity scores; a dependency-free
lexical-overlap reranker shared across MCP + Connect (recall@8=1.00 on the #261 miss);
reconciliation of the already-shipped asymmetric embedder params; async-on-write summary
fill via a bounded worker pool off the write path; and per-record usage signals
(`access_count`/`last_accessed_at`) that never touch ranking.

### What Worked

- **Eval-first ranking.** Building the #261 regression fixture *before* choosing a ranking
  fix meant the lightest lever (a stdlib lexical reranker) was picked by measured numbers —
  D-07 hybrid fusion and D-08 cross-encoder were evaluated and discarded, avoiding a
  dependency, a schema change, and a reindex.
- **Baseline-verify before planning.** `/gsd-discuss-phase 10` caught that
  `REQ-embedder-native-params` was already shipped under Phase 4, saving a whole phase of
  redundant work (echoing the Phase 8 already-shipped surprise). Cheap to check, expensive to miss.
- **Kernel reuse across phases.** The Phase 11 CR-01 shutdown-safety kernel (RWMutex+closed,
  inFlight reserve-before-send) was reused verbatim by Phase 12's `usagequeue.go` — one hard
  concurrency problem solved once, applied twice.
- **Cross-AI + adversarial review paid for itself.** Phase 11's Codex review and
  `/gsd-code-review` surfaced two genuine, reusable Go bugs (retry-budget collapse; Shutdown
  not killing handlers) that unit tests alone did not.

### What Was Inefficient

- **Worktree isolation degraded to sequential every phase.** The recurring #683 condition
  (HEAD diverged from fork because phases 9–12 stacked on one unmerged feature branch) forced
  sequential main-tree execution for all four phases — losing the parallel-wave speedup. Now
  cleared (main advanced past v0.9.x); a fresh v0.10.x branch off updated main should recover it.
- **Executor self-eval blind spot.** Phase 12's executor claimed "task green" while having
  introduced two `revive` findings and misattributed them as pre-existing; the verifier caught
  it via `git show`. Lesson: always run `task lint:go` yourself, never trust the executor's
  self-report.
- **GPG signing flakiness.** 1Password SSH-signing auto-relocked repeatedly, forcing repo-local
  `commit.gpgsign=false` for the whole milestone (unsigned commits; main had no required-signatures
  so the merge was fine). Restore pending.
- **Gateway brownouts blocked the prod-parity eval.** `qwen3-embedding-8b` via OpenRouter kept
  529-ing past the 30s client timeout, so the #261 rank bar was confirmed on a Gemini substitute;
  prod-parity re-confirm deferred to #334 (blocked by the embed-timeout work #333).

### Patterns Established

- **`*time.Time` for optional timestamps** (nil = never), never `time.Time`+`omitempty` — the
  latter is a no-op on struct values and emits bogus `0001-01-01` timestamps. Mirrors the
  existing `NotBefore`/`NotAfter` pattern.
- **`recallView`/`toRecallView` is a hand-written allow-list** — new struct fields do not
  auto-surface on the compact search/list view; adding a field to the recall surface requires an
  explicit edit (get_memory returns raw; list/search do not).
- **Connect exposure = proto field bump + `task proto:gen`** — additive fields need regen or the
  CI drift-check (`git diff --exit-code -- gen/`) fails while the build stays green (false-positive
  green); regen is a blocking step.
- **Post-codegen gopls staleness** — after `task proto:gen`, the IDE falsely flags "unknown field"
  until it reindexes `gen/`; trust `go build` over the IDE.

### Key Lessons

- Measure before you optimize ranking; the eval, not intuition, chooses the lever.
- Verify "already shipped" before planning a phase — this codebase has surprised us twice.
- Solve a concurrency-shutdown kernel once and reuse it; don't re-derive send-on-closed-channel safety.
- Run the lint/test gate yourself at verification time; executor self-reports can be wrong.

### Cost / Process Observations

- Worktrees degraded to sequential main-tree for all 4 phases (#683) — a known, now-cleared condition.
- One milestone PR = all 96 commits / phases 9–12 (phases were never separately merged).
- Milestone shipped, verified, secured (0 open threats), code-reviewed, and audited (PASSED) before close.

---

## Milestone: v0.10.x — Hardening & Write Lane

**Shipped:** 2026-07-16
**Phases:** 9 (13–21) | **Plans:** 35 | **Tasks:** 88

### What Was Built

Embedder reliability (configurable timeout, base-URL join fix, config-identity stamp) + options
(direct Gemini, prod-parity eval, model docs). The full Connect **write lane** in strict order:
6 additive write RPCs → CSRF (Origin + double-submit) → wired handlers with MCP↔Connect authz
parity → stateless sliding-expiry session rotation → console write UX. Plus a correctness/polish
tail (discovery proto fidelity, MintShortID cap, embed cleanups, summarize CronJob) and CI/maintenance
hygiene (rumdl exclude, Phase-11 residuals, Renovate self-heal).

### What Worked

- **Dependency-ordered write lane.** Building proto → CSRF → handlers → session → console in strict
  order meant each phase wired cleanly onto verified foundations; the integration audit found the
  whole spine WIRED with zero gaps.
- **The #1 risk got a real artifact.** MCP↔Connect authz parity (research Pitfall 1) was backed by
  an actual per-RPC parity test (`connectapi_write_parity_test.go`), not a SUMMARY claim.
- **Honesty gates held end-to-end.** REQ-ci-renovate-spa-drift stayed open from planning through
  merge because its self-heal can only be observed post-merge — no automated gate closed it early.
- **Verify-against-toolchain caught phantoms.** Stale gopls `DuplicateMethod` diagnostics (Phase 21)
  were disproven by a real `go build`; the recurring lesson held.

### What Was Inefficient

- **Recurring 1Password signing lock** (d8rjr4zqva et al.) stalled the plan chain mid-run in both
  Phase 20 and Phase 21 at the same point — a per-session friction that keeps recurring.
- **VALIDATION.md bookkeeping drift.** Phases 16 & 19 shipped with unflipped/unfilled VALIDATION.md
  (16 `draft`, 19 an unfilled template) despite full test coverage — reconciled retroactively at
  milestone close (0 real gaps). The signoff flag should flip at execution time.
- **Milestone-close tooling assumed a different repo shape.** The GSD workflow wanted to `git tag`
  and push to `main`; this repo uses release-please for tags and protects `main` — required adapting
  the archival to a branch+PR with no tag.

### Patterns Established

- **Post-merge-only verification is legitimate** — some requirements (a self-heal that only fires on
  a real bot PR after the workflow is on `main`) can't be observed before shipping; ship with the
  observation deferred and tracked by an issue, don't block the merge.
- **GitHub App installation token for CI self-push** — the only credential that re-triggers required
  checks under branch protection (`GITHUB_TOKEN` pushes don't).
- **`set -e` command-substitution swallow** — `echo "x=$(cmd)"` never fails the step; assign on its
  own line.

### Key Lessons

- Distinguish `partial`/deferred from `unsatisfied`/failed at audit time — the FAIL gate should only
  fire on real failures, not honest post-merge deferrals (v0.10.x = `tech_debt`, not `gaps_found`).
- Don't hardcode drift-prone numbers in docs (the rumdl failure count drifted 1505→1566 in a day;
  dropped the number entirely).

### Cost Observations

- Model mix: opus for planning, sonnet for research/execution/verification/review.
- Worktrees degraded to sequential main-tree throughout (#683) — the same known condition as v0.9.x.
- Per-phase lifecycle ran fully (discuss→plan→execute→verify→secure→review→ship) with per-phase PRs,
  unlike v0.9.x's single all-phases PR.

---

## Cross-Milestone Trends

Populated as milestones accumulate.

| Trend | v0.9.x | v0.10.x | Notes |
|-------|--------|---------|-------|
| Already-shipped surprises | 1 (Phase 10) | 0 | v0.9.x also had Phase 8 in the baseline — baseline-verify before planning |
| Worktree isolation | degraded (#683) | degraded (#683) | Stacked unmerged branch both times; cleared post-merge |
| Reusable kernels extracted | 2 (CR-01 shutdown, `*time.Time`) | App-token self-push, `set -e` sub-swallow, post-merge-defer | Applied within-milestone and captured for reuse |
| Requirements satisfied | 6/6 | 19/20 (1 post-merge-deferred) | 3-source cross-referenced |
| Audit verdict | PASSED | tech_debt (0 blockers) | v0.10.x: 1 deferred obs + bookkeeping, no defects |
| Merge shape | 1 PR (all phases) | per-phase PRs | v0.10.x shipped each phase independently |

---

*Retrospective started at v0.9.x close (2026-07-10).*
