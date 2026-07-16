---
phase: 21-ci-maintenance-hygiene
verified: 2026-07-16T16:20:00Z
status: human_needed
score: 3/3 must-haves verified (code/infra complete); 1 live-observation item outstanding
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "Observe the first real Renovate `ui/` bump PR self-heal end-to-end (tracked by GitHub issue #369)."
    expected: "The `ui-drift` job's three-signal guard matches the Renovate PR branch; the App-token mint succeeds; the self-heal step commits+pushes onto the true PR head branch; the push re-triggers `pull_request: synchronize`; all 8 required checks from `protect-main` rerun on the new SHA (none stuck in \"Expected\"); auto-merge completes (or, if Assumption A1 is wrong, a human clicks merge on a green PR — a weaker, acceptable failure mode). On a *non*-Renovate PR with genuine drift, the job must still take the fail-with-guidance branch (`::error::vendored SPA is stale...` + exit 1)."
    why_human: "GitHub Actions expression context (`github.actor`, `github.event.pull_request.head.repo.full_name`) and App-token push/re-trigger semantics only evaluate inside a real workflow run against a real Renovate PR. `task lint:actions` (run and confirmed passing this verification) proves only that the YAML parses — it is not evidence the guarded push, re-trigger, and auto-merge chain actually works. This is REQ-ci-renovate-spa-drift's own `<requirement_completion_honesty>` design: the requirement is deliberately left open (`[ ]` in REQUIREMENTS.md) until this live observation happens, and issue #369 carries the obligation past the phase boundary."
---

# Phase 21: CI / Maintenance Hygiene Verification Report

**Phase Goal:** The CI pipeline and planning tooling stop generating false-positive red builds so
real signal isn't drowned out by noise.
**Verified:** 2026-07-16T16:20:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `task lint:markdown` exits 0 on a clean tree, unblocking `task` default's own gate | ✓ VERIFIED | `task lint:markdown` run live: `Success: No issues found in 120 files`. |
| 2 | Markdown outside `.planning/` is still linted (exclude did not overreach) | ✓ VERIFIED | Scope probe (`rumdl-scope-probe.md` at repo root, deliberate MD022/MD024/MD041 violations) run live: `Found 4 issues in 1 file`, exit 1. Probe file removed after the check; `git status` confirms it is not tracked/left behind. |
| 3 | ROADMAP SC2 no longer mislabels IN-01 as "duplicate depth-gauge registration"; SC3 carries no stale failure count | ✓ VERIFIED | `grep -n 'depth-gauge\|331-failure' .planning/ROADMAP.md .planning/REQUIREMENTS.md` → no match (exit 1). ROADMAP.md:466 SC2 reads "storeMemory\`/\`scheduleMemory\` duplicated Upsert-then-enqueue block"; SC3:468 reads "the systemic planning-doc noise is gone" with no hardcoded count. |
| 4 | WR-03: `Wait()` cannot be called from production code — for BOTH `summaryQueue` and `usageQueue` | ✓ VERIFIED | `rg --glob '!*_test.go' -n 'func \(q \*(summaryQueue\|usageQueue)\) Wait' internal/server/` → no match. `go build ./...` succeeds (exit 0). Both methods live in `internal/server/queue_export_test.go` (verified by reading the file — `package server`, SPDX header present, both methods present). |
| 5 | IN-01: `storeMemory`/`scheduleMemory` no longer duplicate MintShortID→Upsert→enqueue; both delegate to one shared helper; enqueue never happens on a failed Upsert; `storeDiscovery`/`storeRule` still never enqueue | ✓ VERIFIED | `internal/server/tools.go:670` defines exactly one `func (d *deps) persistAndEnqueue(...)`; call sites at `:657` (`storeMemory`) and `:707` (`scheduleMemory`). `storeDiscovery` (`:757-782`) and `storeRule` (`rules.go:95`) contain no `tryEnqueue` call — confirmed via `grep -rn tryEnqueue internal/server/*.go` (only 2 non-test hits: `tools.go:679` inside `persistAndEnqueue`, and `tools.go:1059` for the unrelated `usageQueue`/get-path). `TestPersistAndEnqueueSkipsEnqueueOnUpsertFailure` exists (`tools_test.go:846`) and passes live (both `storeMemory` and `scheduleMemory` subtests PASS). |
| 6 | IN-02: `TestBuildDepsFromEnvLoadsConfigOnce` cannot start a real queue from ambient `ENGRAM_SUMMARY_*` env | ✓ VERIFIED | `tools_test.go:1709-1710` adds `t.Setenv("ENGRAM_SUMMARY_MODEL", "")` / `t.Setenv("ENGRAM_SUMMARY_ON_WRITE", "")`. Test run live (with Qdrant testcontainer) — PASS. |
| 7 | REQ-ci-renovate-spa-drift: self-heal code/infra exists, correctly guarded, degrades safely — but the end-to-end live behavior is unverifiable by this phase by design | ⚠️ See Human Verification | `ci.yaml`'s `ui-drift` job reworked exactly as specified (see Key Link Verification below). `task lint:actions` passes. This truth's own plan (`21-03-PLAN.md` `<requirement_completion_honesty>`) explicitly forbids marking it done off any automated gate — the live-observation item below is the correct, designed outstanding item, not a gap. |

**Score:** 6/6 fully-verifiable truths verified. 1 truth (#7) is intentionally left open per the phase's own honesty gate and routed to human verification — not a failure.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `.rumdl.toml` | `exclude` array contains a plain `.planning` entry with why-comment | ✓ VERIFIED | Line 29: `".planning", # GSD planning artifacts — agent-generated working docs, not shipped prose` — plain form, not a glob, appended last per convention. |
| `.planning/ROADMAP.md` | Phase 21 SC2 + SC3 corrected | ✓ VERIFIED | Confirmed via direct read (lines 459-471) and negative greps. |
| `.planning/REQUIREMENTS.md` | REQ-p11-review-residuals + REQ-lint-planning-exclude wording corrected; checkboxes reflect real status | ✓ VERIFIED | `REQ-ci-renovate-spa-drift` `[ ]` (correctly open), `REQ-p11-review-residuals` `[x]`, `REQ-lint-planning-exclude` `[x]`. |
| `internal/server/queue_export_test.go` | NEW test-only file holding both relocated `Wait()` methods, SPDX header | ✓ VERIFIED | File exists, `package server`, Apache-2.0 SPDX header present, both `Wait()` methods present with doc-comments citing WR-03. |
| `internal/server/tools.go` — `persistAndEnqueue` | New helper; both call sites collapsed to it | ✓ VERIFIED | Exactly one definition (`:670`); two call sites (`:657`, `:707`). |
| `internal/server/tools_test.go` | New enqueue-ordering regression test + two added `t.Setenv` calls | ✓ VERIFIED | `TestPersistAndEnqueueSkipsEnqueueOnUpsertFailure` (`:846`) and the two `t.Setenv("", "")` calls (`:1709-1710`) both present and passing. |
| `.github/workflows/ci.yaml` — reworked `ui-drift` job | Drift-detection step w/ output, guarded App-token mint, guarded self-heal push, preserved fail-with-guidance step | ✓ VERIFIED | All 5 steps present in the shipped job (read live, lines 155-236): `drift` step (always exits 0, sets `drifted` output), `app-token` mint (SHA-pinned `bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3`, triple-guarded, `continue-on-error: true`), `get-user-id`, self-heal commit/push (guarded on `steps.app-token.outcome == 'success'`), fail-with-guidance (guarded on `drifted == 'true' && steps.app-token.outcome != 'success'`, message text unchanged). |
| GitHub issue tracking the live end-to-end observation | Created by Plan 03 Task 2 | ✓ VERIFIED | Issue #369, OPEN, carries the five-item checklist described in the plan; does not carry a `Closes #301` keyword (confirmed by reading the issue body). |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `.rumdl.toml` `exclude` | `task lint:markdown` | `rumdl check .` | ✓ WIRED | Live run exits 0. |
| `storeMemory`/`scheduleMemory` | `persistAndEnqueue` | direct call | ✓ WIRED | Both handlers `return d.persistAndEnqueue(ctx, m, vec)`. |
| `persistAndEnqueue` | `MintShortID → Upsert → tryEnqueue` ordering | sequential calls inside the helper | ✓ WIRED | `tools.go:671` MintShortID, `:674` Upsert with early-return on error, `:679` `tryEnqueue` only reached after a successful Upsert. Regression test pins this. |
| `queue_export_test.go` `Wait()` | 10 in-package `_test.go` call sites | in-package method resolution | ✓ WIRED | `go test ./internal/server/... -run 'TestSummaryQueue\|TestUsageQueue'` style calls not separately re-run here, but the broader targeted test run (below) exercised `Wait()` via `TestPersistAndEnqueueSkipsEnqueueOnUpsertFailure`'s `q.Wait()` call and passed. |
| `ui-drift` drift output + three-signal guard | App-token mint step | `if:` conditional | ✓ WIRED | Guard expression present verbatim: `drifted == 'true' && event_name == 'pull_request' && startsWith(head_ref,'renovate/') && actor == 'fzymgc-renovate[bot]' && head.repo.full_name == github.repository`. |
| App-token mint outcome | self-heal commit/push step | `steps.app-token.outcome == 'success'` guard | ✓ WIRED | Confirmed in the shipped YAML. |
| Self-heal push | PR head branch (not merge ref) | shallow clone of `$HEAD_REF` to `/tmp/ui-drift-heal`, commit+push from there | ✓ WIRED | `git clone --depth 1 --branch "$HEAD_REF" ... /tmp/ui-drift-heal`; commit and `git push origin "HEAD:${HEAD_REF}"` run from the scratch clone, not the job's original `pull_request`-merge-ref checkout. Matches scope correction 2's required mechanism. |
| repo-wide `permissions: contents: read` | `ui-drift` job | absence of job-level `permissions:` block | ✓ WIRED (as a negative) | `ci.yaml:9-10` unchanged; `awk` extraction of the `ui-drift:` job block contains no `permissions:` key. Token write capability comes solely from the App installation + `permission-contents: 'write'` self-scoping on the mint step, matching `release.yaml`'s precedent. |
| `secrets.CI_BOT_APP_CLIENT_ID` / `secrets.CI_BOT_APP_PRIVATE_KEY` | `app-token` mint step `with:` | direct reference | ✓ WIRED | `ci.yaml:203-204` reads `secrets.CI_BOT_APP_CLIENT_ID` / `secrets.CI_BOT_APP_PRIVATE_KEY`; `gh secret list` confirms both exist as repo secrets (added 2026-07-16). |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|--------------|--------|----------|
| REQ-lint-planning-exclude | 21-01 | `.rumdl.toml` excludes `.planning`; `task lint:markdown` passes; shipped Markdown outside `.planning/` stays linted | ✓ SATISFIED | Live `task lint:markdown` exit 0; scope probe still catches violations outside `.planning/`. Checkbox `[x]` in REQUIREMENTS.md — correct. |
| REQ-p11-review-residuals | 21-02 | WR-03, IN-01, IN-02 resolved for both queues | ✓ SATISFIED | `go build ./...` clean of `Wait()`; `persistAndEnqueue` extraction verified; hermetic test verified; all targeted tests pass live. Checkbox `[x]` in REQUIREMENTS.md — correct. |
| REQ-ci-renovate-spa-drift | 21-03 | Self-healing CI fallback for Renovate `ui/` bump drift | ⧗ CODE/INFRA COMPLETE, LIVE OBSERVATION PENDING | `ci.yaml`'s guarded self-heal path shipped and verified structurally (see above); App provisioned (`gh secret list` confirms both credentials exist); `task lint:actions` passes. The requirement is correctly left `[ ]` in REQUIREMENTS.md per the phase's own `<requirement_completion_honesty>` design — closure requires observing issue #369's checklist against a real Renovate PR, which cannot happen inside this verification. |

No orphaned requirements: all three phase requirement IDs (REQ-ci-renovate-spa-drift, REQ-p11-review-residuals, REQ-lint-planning-exclude) are declared in a plan's `requirements` frontmatter and appear in REQUIREMENTS.md under "CI / Maintenance Hygiene".

### Anti-Patterns Found

Scanned all files modified by the three plans (`.rumdl.toml`, `.planning/ROADMAP.md`, `.planning/REQUIREMENTS.md`, `internal/server/{summaryqueue,usagequeue,tools,tools_test,queue_export_test}.go`, `.github/workflows/ci.yaml`) for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` and hardcoded-empty patterns.

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `.rumdl.toml` | 12 | `TODO: re-enable after fixing broken links` | ℹ️ Info | Pre-existing marker on the unrelated MD057 disable rule, not introduced by this phase and not on the `.planning` line this phase added. Not a debt marker on phase-21 work. |

No blocker-tier debt markers found in any file this phase modified. No placeholder/stub patterns (`return null`, empty handlers, hardcoded empty returns) found in the Go files — all new/changed code is substantive and exercised by passing tests.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| rumdl exclude unblocks the markdown gate | `task lint:markdown` | `Success: No issues found in 120 files` | ✓ PASS |
| rumdl exclude does not overreach | scope probe (`rumdl check` on a deliberately-violating repo-root `.md`) | `Found 4 issues in 1 file`, exit 1 | ✓ PASS |
| No stale acceptance-list wording remains | `grep -n 'depth-gauge\|331-failure' .planning/ROADMAP.md .planning/REQUIREMENTS.md` | no match, exit 1 | ✓ PASS |
| Production binary compiles without `Wait()` on either queue | `go build ./...` | exit 0 | ✓ PASS |
| No non-test `Wait()` definition remains | `rg --glob '!*_test.go' -n 'func \(q \*(summaryQueue\|usageQueue)\) Wait' internal/server/` | no match, exit 1 | ✓ PASS |
| `gofmt` clean on the modified package | `gofmt -l internal/server/` | (empty output) | ✓ PASS |
| Targeted regression + hermeticity tests | `go test ./internal/server/... -run 'TestPersistAndEnqueueSkipsEnqueueOnUpsertFailure\|TestBuildDepsFromEnvLoadsConfigOnce\|TestDiscoveryAndRuleNeverEnqueue\|TestStoreMemoryEnqueuesOnSuccess' -v` | all 4 tests + 2 subtests PASS | ✓ PASS |
| `ui-drift` job YAML is syntactically valid | `task lint:actions` | exit 0 | ✓ PASS |
| `.github/renovate.json` untouched | `git diff <merge-base> -- .github/renovate.json` | empty diff | ✓ PASS |
| No `permissions:`/`contents: write` escalation added | `awk` extraction of the `ui-drift:` job block, grep for `permissions:` | no match | ✓ PASS |
| App credentials provisioned | `gh secret list` | `CI_BOT_APP_CLIENT_ID`, `CI_BOT_APP_PRIVATE_KEY` present | ✓ PASS |
| Live-observation issue exists and is correctly scoped | `gh issue view 369` | OPEN, full 5-item checklist, no `Closes #301` keyword | ✓ PASS |
| Live end-to-end self-heal (guard match → push → re-trigger → auto-merge on a real Renovate PR) | N/A — requires a real Renovate PR | not run | ? SKIP (routed to human verification — by design, per `<requirement_completion_honesty>`) |

### Probe Execution

Not applicable — no `scripts/*/tests/probe-*.sh` probes are declared by or relevant to this phase.

## Deferred / Out-of-Scope Items (informational, not gaps)

- **Pre-existing `task lint:yaml` (yamlfmt) failure on `Taskfile.yaml`.** Confirmed via `git diff <merge-base-with-main> -- Taskfile.yaml` → empty diff; the file is untouched by any of this phase's three plans and the failure predates the phase (logged in `deferred-items.md`, matches independent verification here). Not a Phase 21 requirement — none of the three requirements' must-haves reference `task lint:yaml` or full `task` default going green; only `task lint:markdown` (REQ-lint-planning-exclude) and granular subtargets (`task test:go`, `task lint:go`, `task lint:actions`, `gofmt`, `task license:check`) are in scope, and all of those pass. Needs its own issue per the deferred-items note; does not block this phase's goal.
- **Issues #301 and #335 remain OPEN on GitHub.** Expected: this phase's work lives on an unmerged branch (`phase-21-ci-maintenance-hygiene`); `Closes #NNN` keywords in commit bodies only auto-close on merge to the default branch via PR. Not a gap.

## Human Verification Required

### 1. Live Renovate self-heal observation (issue #369)

**Test:** Wait for (or trigger) the first real Renovate `ui/` dependency-bump PR after this phase ships, and observe the `ui-drift` job's behavior on it.

**Expected:**
- The three-signal guard (`renovate/` head-ref prefix + `fzymgc-renovate[bot]` actor + same-repo `head.repo.full_name`) matches the Renovate PR branch and the App-token mint step succeeds.
- The self-heal step commits the regenerated `internal/webauth/static/` onto the true PR head branch (not the merge ref) and pushes successfully.
- The push re-triggers `pull_request: synchronize`, producing a new head SHA with fresh check runs.
- All 8 required checks from the `protect-main` ruleset rerun and report on the new SHA (none left stuck in "Expected").
- Auto-merge completes without human intervention — or, if Assumption A1 is wrong, a human clicks merge on an otherwise-green PR (a weaker, accepted failure mode, not a design break).
- Separately, on a genuinely-drifted *non*-Renovate PR, the job still takes the fail-with-guidance branch (`::error::vendored SPA is stale...`, exit 1) — confirming the guard correctly rejects everything else.

**Why human:** GitHub Actions expression context and the App-token push/re-trigger/auto-merge chain only evaluate inside a real workflow run against a real Renovate PR — there is no way to execute or simulate this from a static repository checkout. This is not an oversight: `21-03-PLAN.md`'s `<requirement_completion_honesty>` section explicitly designed REQ-ci-renovate-spa-drift to stay open (and REQUIREMENTS.md correctly carries it as `[ ]`) until this exact observation happens. Issue #369 is the durable tracker for this obligation.

## Gaps Summary

No gaps. All mechanically-verifiable must-haves across all three plans pass live re-execution of their acceptance checks (not just SUMMARY narration): rumdl exclude + scope probe, ROADMAP/REQUIREMENTS corrections, `Wait()` structural unreachability + build, `persistAndEnqueue` extraction + regression test, `TestBuildDepsFromEnvLoadsConfigOnce` hermeticity, and the `ui-drift` job's guard/permissions/merge-ref/renovate.json/issue-tracking structure. The single outstanding item — live observation of the Renovate self-heal path — is the phase's own deliberate, designed honesty gate, not an implementation gap. The phase goal ("CI pipeline and planning tooling stop generating false-positive red builds") is achieved for 2 of 3 requirements today (rumdl noise gone, Phase-11 residuals resolved) and is code/infra-complete-but-unobserved for the third (Renovate self-heal), exactly matching the phase's own stated completion contract.

---

_Verified: 2026-07-16T16:20:00Z_
_Verifier: Claude (gsd-verifier)_
