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

## Milestone: v0.11.x — Capture & Service Identity

**Shipped:** 2026-07-26
**Phases:** 5 (22–26) | **Plans:** 19 | **Tasks:** 46
**Merged via:** 4 PRs — #396 (phases 22+23), #404 (24), #429 (25), #432 (26)

### What Was Built

- **Cedar authz foundation** — `internal/authz` (cedar-go v1.8.0) with a 4-policy embedded corpus;
  `DecideBucket` on bulk recall (O(buckets), never per-record), `DecideRecord` on id-addressed
  gates. Byte-for-byte behavior-preserving; ADR `engram-cdr1` refines LOCKED `DEC-cgb`.
- **Service identity** — `auth.ChainVerifier` (OIDC user → client-credentials → static token) at the
  single `withAuth` site, with a fail-closed gate rejecting an authenticated principal that resolves
  to an empty owner.
- **Idempotent capture** — optional `idempotency_key`, deterministic UUIDv5 point ID over injective
  `(owner, scope, key)`, payload-only fingerprint compared before the embedder call, reject-not-upsert.
- **Supersession with history** — `supersede_memory` back-stamps `superseded_by` via single-key
  `SetPayload`; soft-hidden from recall, still fetchable by id.
- **Citations, category filter, chat base URL** — optional structured provenance on any category, a
  `categories` OR pre-filter at MCP↔Connect parity, and `ENGRAM_OPENAI_CHAT_BASE_URL` with a shared
  shape-aware URL join.

### What Worked

- **Ordering the milestone by trust dependency, not by size.** Research put the authz foundation
  first because every later phase's isolation rests on it. That paid off directly: phases 24–26 each
  added filters and payload keys, and the audit confirmed none of them perturbed the authz
  outer-`Must` invariant.
- **Proving the #1 risk as the first test.** The service-principal-resolves-to-`owner==""` risk was
  named at roadmap time and closed by `TestFailClosedRejectsEmptyOwner` before any other Phase 23
  work. Naming the top risk up front and making it the first executable check is repeatable.
- **Independent review caught what green tests could not.** Three phases had a review find a real
  defect: Phase 23's static-token map-orientation inversion (the lane was deployed non-functional and
  could leak the raw token as owner), Phase 25's cross-path lost write, Phase 26's fingerprint
  omission. All three compiled, vetted, linted, and passed the full suite.
- **Additive-only payload evolution.** Three milestones' worth of new payload keys
  (`idempotency_fingerprint`, `superseded_by`, `citations`) landed on one collection with no
  migration, because each was written through the existing `payload()`/`fromPayload()` codec.

### What Was Inefficient

- **A wrong assumption written into a passing test is invisible.** Phase 26's planner flagged
  "citations are excluded from the idempotency fingerprint" as an assumption; the executor
  implemented it faithfully *and wrote a subtest asserting it*. "Do the tests pass?" could never have
  caught it — only a reviewer reasoning from the contract did. The saving grace was that the
  assumption was flagged rather than buried, which made it cheap to overturn.
- **CI gates that no phase workflow runs.** Phase 26 was fully verified, UAT'd, security-reviewed and
  code-reviewed, then went red on `helm chart` (a checksum drift pin) and `ui vendored-asset drift`
  the moment the PR opened. Neither `task chart:validate` nor `task ui:build` is part of
  verify/secure/UAT.
- **Rebuilding generated artifacts against the wrong tree.** The vendored-SPA fix had to be rebuilt
  *after* merging `main`, because CI builds the merge ref — a branch-only rebuild produced different
  content-hash chunk names and stayed red.
- **Stale roadmap bookkeeping.** Progress-table plan counts drifted from reality on 4 of 5 phases
  (one read `0/1` while marked Complete) and were only caught at audit time.

### Patterns Established

- **PDP decides the predicate; the store enforces it.** Bucket-level decisions compiled into the
  Qdrant filter — the workable shape when the policy engine has no partial evaluation.
- **Options struct before the second same-typed parameter.** `store.SearchOptions` replaced a
  positional tail specifically because two adjacent `[]string` params transpose silently.
- **Write-domain allowlists must not be copied to the read domain.** `discovery`/`rule` are legitimate
  *filter* values though not legitimate *write* values (D-11).
- **Targeted `SetPayload` for out-of-band keys; whole-payload `Upsert` only under a lock.**
  `store.TargetLocker` serializes `Update` and `Supersede` per target.
- **Explicit field lists need explicit maintenance.** `contentFingerprint` is not reflection-based, so
  any new client-authored `storeArgs` field must be added to it in the same change — now recorded in
  a doc comment on the function.

### Key Lessons

- Flag assumptions rather than burying them. Phase 26's overturned assumption was cheap to reverse
  *because* it was written down as an assumption; the same belief buried in prose would have shipped.
- A tool without agent-facing guidance is an incomplete feature — Phase 25 shipped the
  `curating-memory` skill update in the same PR as `supersede_memory`, and Phase 26 did the same for
  citations.
- Run `task chart:validate` and `task ui:build` locally before shipping any phase whose diff touches
  `charts/` or generated TS. The phase gates do not cover them.
- When a generated artifact drifts in CI, check whether CI builds the merge ref before rebuilding.

### Cost Observations

- Model mix: opus for planning/orchestration, sonnet for research/execution/verification/review.
- Per-phase lifecycle ran fully (discuss→plan→execute→verify→secure→review→ship) with per-phase PRs,
  except phases 22+23 which shipped together in #396.
- Two phases (24, 26) closed without reconciling `VALIDATION.md`, leaving Nyquist coverage at 3/5.
- Every commit this milestone used the per-commit `git -c commit.gpgsign=false` override — the
  1Password SSH signing agent failed throughout; persistent config was never modified.

---

## Cross-Milestone Trends

Populated as milestones accumulate.

| Trend | v0.9.x | v0.10.x | v0.11.x | Notes |
|-------|--------|---------|---------|-------|
| Already-shipped surprises | 1 (Phase 10) | 0 | 0 | v0.9.x also had Phase 8 in the baseline — baseline-verify before planning |
| Worktree isolation | degraded (#683) | degraded (#683) | degraded (#683) | Stacked unmerged branch each time; cleared post-merge |
| Reusable kernels extracted | 2 (CR-01 shutdown, `*time.Time`) | App-token self-push, `set -e` sub-swallow, post-merge-defer | PDP-decides/store-enforces, options-struct-before-2nd-same-type, targeted-SetPayload, explicit-field-list upkeep | Applied within-milestone and captured for reuse |
| Requirements satisfied | 6/6 | 19/20 (1 post-merge-deferred) | 11/11 | 3-source cross-referenced |
| Audit verdict | PASSED | tech_debt (0 blockers) | PASSED (0 blockers) | v0.11.x also 6/6 integration seams, 2/2 E2E flows |
| Merge shape | 1 PR (all phases) | per-phase PRs | per-phase PRs (22+23 combined) | 4 PRs for 5 phases |
| Defects caught by review, not tests | — | — | 3 (phases 23, 25, 26) | All compiled, vetted, linted, and passed the suite |
| Nyquist coverage | — | 9/9 | 3/5 (24, 26 left `draft`) | v0.11.x regression — reconcile VALIDATION.md at phase close |

---

*Retrospective started at v0.9.x close (2026-07-10).*
