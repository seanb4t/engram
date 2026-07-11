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

## Cross-Milestone Trends

Populated as milestones accumulate.

| Trend | v0.9.x | Notes |
|-------|--------|-------|
| Already-shipped surprises | 1 (Phase 10) | Also Phase 8 in the baseline — baseline-verify before planning |
| Worktree isolation | degraded (#683) | Stacked unmerged branch; cleared post-merge |
| Reusable kernels extracted | 2 (CR-01 shutdown, `*time.Time`) | Applied across Phases 11–12 |
| Requirements satisfied | 6/6 | 3-source cross-referenced; audit PASSED |

---

*Retrospective started at v0.9.x close (2026-07-10).*
