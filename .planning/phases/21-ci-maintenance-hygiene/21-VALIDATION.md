---
phase: 21
slug: ci-maintenance-hygiene
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-15
---

# Phase 21 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `21-RESEARCH.md` § Validation Architecture. Task IDs are filled in
> after `gsd-planner` writes the PLAN.md files; the plan-checker finalizes this
> doc (`status: approved`, `nyquist_compliant: true`).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (existing suite) + `rumdl` (existing lint gate) — **no new framework** |
| **Config file** | none new — `.rumdl.toml` is edited, not created |
| **Quick run command** | `go test ./internal/server/... -run 'TestSummaryQueue|TestUsageQueue|TestBuildDepsFromEnvLoadsConfigOnce' -v` |
| **Full suite command** | `task` (lint + test) |
| **Estimated runtime** | ~60–120 seconds (`task test:go` pulls a Qdrant testcontainer) |

**Blocking caveat:** `task` (the full gate) is **currently BLOCKED** by the exact rumdl
failure this phase fixes. It only becomes a meaningful gate for Plans B/C **after Plan A
lands**. Until then use the granular subtargets (`task test:go`, `task lint:go`,
`task lint:markdown`).

---

## Sampling Rate

- **After every task commit:** `go test ./internal/server/...` for Go tasks (Plan B);
  `task lint:markdown` for the rumdl task (Plan A); `task lint:actions` (actionlint) for
  workflow tasks (Plan C).
- **After every plan wave:** `task` (full local gate) — meaningful only once Plan A has landed.
- **Before `/gsd-verify-work`:** Full suite must be green.
- **Max feedback latency:** ~120 seconds.

**Mandatory pre-push check (recurrence guard):** CI's `test` job runs a `gofmt -l .` precheck
(`.github/workflows/ci.yaml:49-52`) that is **stricter than golangci-lint**. Phase 20 lost a CI
run to this exact trap (a gofmt-unclean test file passed local golangci and `task test:go`, then
reddened CI). Run `gofmt -l .` (or `task fmt`) before every push on Plan B.

---

## Per-Task Verification Map

> Task IDs are positional and filled in post-planning. Plan letters map to `21-CONTEXT.md` D-11:
> **A** = rumdl exclude · **B** = #335 residuals · **C** = #301 Renovate self-heal.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 21-A-01 | A | 1 | REQ-lint-planning-exclude | — | N/A (no security surface — lint config only) | smoke (config) | `task lint:markdown` | ✅ N/A — the exit code IS the test | ⬜ pending |
| 21-B-01 | B | 1 | REQ-p11-review-residuals (D-04 / WR-03) | — | `Wait()` unreachable from production code — misuse structurally impossible, not doc-guarded | build-level (structural) | `go build ./... && go test ./internal/server/... -run 'TestSummaryQueue|TestUsageQueue' -v` | ✅ `summaryqueue_test.go`, `usagequeue_test.go` already exercise `Wait()` | ⬜ pending |
| 21-B-02 | B | 1 | REQ-p11-review-residuals (D-05 / IN-01) | — | Enqueue still happens **exactly once**, and **only after** a confirmed-successful Upsert | unit | `go test ./internal/server/... -run 'Test.*(StoreMemory|ScheduleMemory)' -v` | ⚠️ exact test names unconfirmed — see Wave 0 | ⬜ pending |
| 21-B-03 | B | 1 | REQ-p11-review-residuals (D-06 / IN-02) | — | Ambient env can never start an unshut-down queue (no leaked workers) | unit | `go test ./internal/server/... -run TestBuildDepsFromEnvLoadsConfigOnce -v` | ✅ `internal/server/tools_test.go:1624` | ⬜ pending |
| 21-C-01 | C | 1 | REQ-ci-renovate-spa-drift | T-21-C-* | Guard rejects fork + non-Renovate PRs **before** the token mint; App token is minted just-in-time, scoped `Contents: Read & write`, auto-revoked in the action's `post` step | static (lint) | `task lint:actions` (actionlint) | ✅ existing gate | ⬜ pending |
| 21-C-02 | C | 1 | REQ-ci-renovate-spa-drift | T-21-C-* | Existing fail-with-guidance path unchanged for non-Renovate branches | manual (CI-integration) | **Not unit-testable** — see Manual-Only | ❌ Wave 0 gap | ⬜ pending |
| 21-C-03 | C | 1 | REQ-ci-renovate-spa-drift | — | GitHub App provisioned with `Contents: Read & write`; secrets present | `checkpoint:human-verify` | **Human-only** — cannot be performed or verified by an agent | ❌ App does not exist yet | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] **Enumerate the IN-01 regression tests before writing Plan B's verify steps.** The exact
      `TestStoreMemory*` / `TestScheduleMemory*` names in `internal/server/tools_test.go` were not
      enumerated during research. Planner/executor must run
      `rg 'func Test.*(StoreMemory|ScheduleMemory)' internal/server/tools_test.go` and pin the real
      names. If no test asserts the enqueue-after-successful-Upsert ordering, **add one** — the D-05
      helper extraction is a behavior-preserving refactor and needs a test that would actually fail
      if the ordering regressed.
- [ ] **Provision the GitHub App for #301** (Client ID + private key as repo secrets/vars) — human-only,
      blocks 21-C-02's live path. See Manual-Only below and `21-RESEARCH.md` Open Question 2.

*Everything else: existing infrastructure covers the phase requirements — no new test framework,
no new fixtures, no new config.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Self-heal commits + pushes the regenerated SPA on a real Renovate PR, and the re-triggered checks let auto-merge complete | REQ-ci-renovate-spa-drift | GitHub Actions expression context (`github.actor`, `github.event.pull_request.head.repo.full_name`) cannot be evaluated outside a real workflow run; and the App token does not exist yet | Observe the **first real Renovate `ui/` bump PR** after this ships: confirm (a) the self-heal step fires, (b) the push re-triggers `pull_request: synchronize`, (c) all 8 required checks rerun on the new SHA, (d) auto-merge completes. **Do not mark REQ-ci-renovate-spa-drift done until this is observed live.** |
| Guard correctly rejects a non-Renovate PR (existing `::error::` fail path intact) | REQ-ci-renovate-spa-drift | Same — needs a real PR context | This phase's own PR is a non-Renovate PR by construction. Confirm `ui-drift` takes the fail-with-guidance branch if drift is introduced, or reports no drift (the common case — this phase makes no SPA changes). |
| GitHub App has `Contents: Read & write` on this repo | REQ-ci-renovate-spa-drift | Requires GitHub org admin access; an agent cannot introspect App installation permissions | Human: `Settings → GitHub Apps → <App> → Permissions`. Either provision a new App or confirm `RELEASE_APP`'s installation already grants `Contents: write` (unconfirmed — `21-RESEARCH.md` Open Question 2). Add Client ID + private key as repo vars/secrets. |
| Auto-merge survives Renovate abandoning the branch after the self-heal push | REQ-ci-renovate-spa-drift | Assumption A1 in `21-RESEARCH.md` — not independently verifiable pre-ship | Observe on the first live Renovate PR. **Failure here is strictly weaker than the wedge** (a human clicks merge; the PR is not stuck), so it does not block shipping. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or a Wave 0 / manual-only dependency
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (IN-01 test names; App provisioning)
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `gofmt -l .` clean before every Plan B push (CI precheck is stricter than golangci)
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
