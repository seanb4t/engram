# Phase 21: CI / Maintenance Hygiene - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-15
**Phase:** 21-ci-maintenance-hygiene
**Areas discussed:** Renovate self-heal mechanism, WR-03 `Wait()` misuse guard, IN-01 scope reconciliation, rumdl exclude form, Landing strategy
**Mode:** `--auto` — all gray areas auto-selected, recommended option chosen per question with no user prompt. Single pass.

---

## Renovate self-heal mechanism (#301)

| Option | Description | Selected |
|--------|-------------|----------|
| (b) Self-healing CI fallback | `ui-drift` job commits+pushes the regenerated SPA on Renovate branches instead of failing. In-repo, testable, verifiable by CI. | ✓ |
| (a) Fix homelab Renovate | Set `allowedPostUpgradeCommands` + node/pnpm via `binarySource:install` on the self-hosted bot. Addresses the true root cause. | |
| (c) Document only | Keep the manual "`task ui:build` + commit after any ui/ bump" operator burden. | |

**Choice:** (b) — the self-healing CI fallback.
**Notes:** Option (a) is the actual root cause but lives outside this repo; engram's CI can neither implement nor verify it, so it cannot be this phase's deliverable — captured as a deferred idea instead. The two are complementary: if the bot is ever fixed it commits first and CI then finds no drift. Scouting showed the `ui-drift` job already performs the full rebuild (`ci.yaml:172-175`) and merely fails on the diff at `:176`, so the delta is a guard + commit/push step, not a new job.

### Sub-question: keep the inert `postUpgradeTasks` rule?

| Option | Description | Selected |
|--------|-------------|----------|
| Keep it | Harmless while inert; wins first if the homelab bot is ever fixed. | ✓ |
| Delete it | Remove dead config that has never fired. | |

**Choice:** Keep it (D-02).
**Notes:** No conflict between the two paths — belt-and-suspenders.

### Sub-question: permission scoping

| Option | Description | Selected |
|--------|-------------|----------|
| Job-level `contents: write` on `ui-drift` only | Repo-wide default stays `contents: read`; mirrors the existing `commit-lint` job-level permissions block. | ✓ |
| Repo-wide `contents: write` | Simpler, but grants write to every job. | |

**Choice:** Job-level escalation, same-repo + Renovate-actor guard, `main` pushes never self-heal (D-03).
**Notes:** Flagged RESEARCH-CRITICAL and deliberately left unresolved for the researcher: `GITHUB_TOKEN` pushes do not trigger new workflow runs, so combined with `platformAutomerge: true` a self-heal commit risks leaving the head SHA with no checks and wedging auto-merge on "Expected" — trading a red build for a stuck PR. Also unresolved: how Renovate reacts to a third-party commit on its own branch under `config:best-practices` (`rebaseWhen`). CONTEXT instructs the planner to stop and surface rather than ship a wedging fix.

---

## WR-03 — `Wait()` misuse guard (#335)

| Option | Description | Selected |
|--------|-------------|----------|
| Make misuse impossible | Move `Wait()` out of production reach (test-only file / build-tag helper) — the review's own stronger suggestion. | ✓ |
| Doc-comment contract | Document on `Wait()` that it must not be called concurrently with live `tryEnqueue` traffic. Cheapest; latent trap persists. | |

**Choice:** Make misuse structurally impossible (D-04).
**Notes:** Evidence closed this rather than preference: all 10 `Wait()` callers in the repo are `_test.go` files (5 `summaryqueue_test.go`, 4 `usagequeue_test.go`, 1 `connectapi_test.go:875`) — there is no production caller to preserve, so the stronger fix costs nothing. A comment asks future callers to behave; removing the path guarantees it.

### Sub-question: which queue(s)?

| Option | Description | Selected |
|--------|-------------|----------|
| Both `summaryQueue` and `usageQueue` | `usagequeue.go` is a verbatim mirror of the same `inFlight`/`Wait()` kernel. | ✓ |
| `summaryQueue` only | Matches the literal text of issue #335. | |

**Choice:** Both (D-00c).
**Notes:** `usageQueue` did not exist when WR-03 was written during Phase 11, which is why the issue names only `summaryQueue`. Fixing one and leaving its clone carrying the identical hazard would be a half-fix.

---

## IN-01 — scope reconciliation (#335)

| Option | Description | Selected |
|--------|-------------|----------|
| Trust 11-REVIEW.md + issue #335 | IN-01 = `storeMemory`/`scheduleMemory` duplicate the Upsert→`tryEnqueue` block; extract a shared helper. Correct the ROADMAP. | ✓ |
| Trust ROADMAP SC2 | IN-01 = "duplicate depth-gauge registration". | |

**Choice:** The review + issue are authoritative; the ROADMAP is mislabeled and gets corrected this phase (D-00a, D-12).
**Notes:** Verified in code rather than assumed — the depth gauge is registered exactly once (`tools.go:255`), so ROADMAP SC2 describes a defect that does not exist. Mirrors the Phase-20 #303 precedent where the researcher corrected an issue's stated scope before implementing. Left unfixed, the ROADMAP would guarantee a verifier mismatch since it is the acceptance list. Helper scope limited to `storeMemory` + `scheduleMemory`; `storeDiscovery`/`storeRule` deliberately do not enqueue (`tools.go:706`).

---

## rumdl exclude form (REQ-lint-planning-exclude)

| Option | Description | Selected |
|--------|-------------|----------|
| Plain `.planning` | Matches the existing convention of every neighbor (`.git`, `.beads`, `.claude`, `dist`, `docs-site`). | ✓ |
| Glob `.planning/**` | The form ROADMAP SC3's prose implies. | |

**Choice:** Plain `.planning` with a why-comment in the neighboring style (D-09).
**Notes:** Measured the blast radius rather than trusting the roadmap figure: `rumdl check .` reports 1505 issues across 155/354 files, **all** under `.planning/` and **zero** outside. ROADMAP SC3's "331-failure" number is stale (more planning docs have landed since) and is corrected this phase. Because nothing outside `.planning/` fails today, SC3's "shipped Markdown still linted" half is already true and a clean `rumdl check .` proves both halves at once — so no extra guard or test was added.

---

## Landing strategy

| Option | Description | Selected |
|--------|-------------|----------|
| 3 plans, one per requirement | File-disjoint; each closes its own issue with an atomic commit. rumdl lands first (unblocks `task` default). | ✓ |
| 2 plans (fold rumdl into another) | Fewer plans; rumdl is a one-liner. | |
| 1 plan | Single hygiene sweep. | |

**Choice:** 3 plans — A (rumdl), B (Phase-11 residuals), C (Renovate self-heal) (D-11).
**Notes:** Reuses the Phase-20 D-10 by-subsystem split. The three are fully file-disjoint and parallelizable; Plan A is a soft ordering dependency because it unblocks `task` default (blocked since Phase 20, memory `kwp5wq89bq`) which every other plan's quality gate runs. Plan C carries all the phase's uncertainty and is gated on the RESEARCH-CRITICAL items.

---

## Claude's Discretion

All three items are mechanical hygiene with no user-facing decision. Within the captured decisions, implementation detail is Claude's call:

- The test-only `Wait()` mechanism and file naming (D-04).
- The `persistAndEnqueue` helper's exact signature and name (D-05).
- The `.rumdl.toml` comment wording (D-09).
- The workflow guard expression and step phrasing (D-03).

**Explicit non-discretionary carve-out:** if #301's token/auto-merge question has no clean answer, stop and surface it rather than shipping a self-heal that wedges Renovate PRs on "Expected" checks. A wedged PR is worse than the red build it replaces.

## Deferred Ideas

- **Fixing the homelab Renovate instance** (#301 option a) — the true root cause, but external to this repo and unverifiable by engram's CI. Belongs in homelab infra work.
- **Adding `edited` to `ci.yaml`'s `pull_request` trigger types** (memory `d8rjr4zqva`) — would let a `/gsd-ship` PR-title fix re-trigger commit-lint without the close/reopen dance. CI-hygiene-shaped and tempting to fold in here, but not covered by any of Phase 21's three requirements. Worth its own issue.
- **The from-beads refactor cluster** (#306, #309, #310, #312, #313, #315, #316, #318, #319) — marked opportunistic in `REQUIREMENTS.md:74`. Phase 21 touches `internal/server/tools.go`; the planner may fold in an overlapping cluster issue, otherwise leave it.
