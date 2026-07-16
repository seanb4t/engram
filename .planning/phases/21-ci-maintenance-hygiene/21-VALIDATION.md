---
phase: 21
slug: ci-maintenance-hygiene
status: approved
nyquist_compliant: true
wave_0_complete: false
created: 2026-07-15
approved: 2026-07-15
---

# Phase 21 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `21-RESEARCH.md` § Validation Architecture. Task IDs below are the
> real positional IDs from the three PLAN.md files.
>
> **Approved 2026-07-15** — `gsd-plan-checker` returned VERIFICATION PASSED on
> iteration 1 with Dimension 8 (Nyquist) PASS: no watch-mode flags, no E2E suites,
> no unfilled `MISSING` Wave-0 references. `wave_0_complete` stays `false` until
> execution actually lands the Wave-0 items.

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
| 21-01-01 | 01 | 1 | REQ-lint-planning-exclude | — | N/A (no security surface — lint config only) | smoke (config) | `task lint:markdown` | ✅ N/A — the exit code IS the test | ⬜ pending |
| 21-01-02 | 01 | 1 | REQ-lint-planning-exclude (D-12) | — | N/A (docs correction) | grep assertion | verify greps over `ROADMAP.md` **and** `REQUIREMENTS.md` | ✅ N/A | ⬜ pending |
| 21-02-01 | 02 | 1 | REQ-p11-review-residuals (D-04 / WR-03) | — | `Wait()` unreachable from production code — misuse structurally impossible, not doc-guarded | build-level (structural) | `go build ./... && go test ./internal/server/... -run 'TestSummaryQueue|TestUsageQueue' -v` + `gofmt -l internal/server/` | ✅ `summaryqueue_test.go`, `usagequeue_test.go` already exercise `Wait()` | ⬜ pending |
| 21-02-02 | 02 | 1 | REQ-p11-review-residuals (D-05 / IN-01) | — | Enqueue happens **exactly once**, and **only after** a confirmed-successful Upsert — a **failed** Upsert must produce **no** enqueue | unit (**characterization — `tdd="true"`, must pass BEFORE the refactor**) | `go test ./internal/server/... -run 'Test.*(StoreMemory|ScheduleMemory)' -v` + `gofmt -l internal/server/` | ✅ resolved — see Wave 0 | ⬜ pending |
| 21-02-03 | 02 | 1 | REQ-p11-review-residuals (D-06 / IN-02) | — | Ambient env can never start an unshut-down queue (no leaked workers) | unit | `go test ./internal/server/... -run TestBuildDepsFromEnvLoadsConfigOnce -v` + `gofmt -l internal/server/` | ✅ `internal/server/tools_test.go:1624` | ⬜ pending |
| 21-03-01 | 03 | 1 | REQ-ci-renovate-spa-drift | T-21-03-01, T-21-03-02 | Three-signal guard rejects fork + non-Renovate PRs **before** the App-token mint step; token minted just-in-time, scoped `Contents: Read & write`, auto-revoked in the action's `post` step | static (lint) — **proves syntax ONLY, not behavior** | `task lint:actions` (actionlint) | ✅ existing gate | ⬜ pending |
| 21-03-02 | 03 | 1 | REQ-ci-renovate-spa-drift | — | Live-observation obligation is carried past the phase boundary | tracking issue | `gh issue view <n>` | ✅ N/A | ⬜ pending |
| 21-03-03 | 03 | 1 | REQ-ci-renovate-spa-drift | T-21-03-02 | GitHub App provisioned with `Contents: Read & write`; Client ID + private key present as repo vars/secrets | `checkpoint:human-verify` (`gate="blocking-human"`) | **Human-only** — cannot be performed or verified by an agent | ❌ App does not exist yet | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] **RESOLVED at planning time — IN-01 regression coverage enumerated.** `TestStoreMemoryEnqueuesOnSuccess`
      (`tools_test.go:718`) and `TestDiscoveryAndRuleNeverEnqueue` (`:785`) exist, but coverage is
      **positive-path only**: there is no `scheduleMemory` enqueue test, and **nothing asserts that a
      failed `Upsert` produces no enqueue** — the exact invariant D-05 must preserve. Root cause:
      `spyStore.Upsert` **always returns nil**, so the failure path is structurally unreachable by the
      existing suite. Task 21-02-02 adds a characterization test (`tdd="true"`) using a
      `memStore`-embedding wrapper (Qdrant-free, mirrors `newSpyDeps()`), which **must pass before**
      the refactor.
- [ ] **Provision the GitHub App for #301** (Client ID + private key as repo vars/secrets) — human-only,
      blocks 21-03-03 and the live self-heal path. Tracked as a `checkpoint:human-verify`
      (`gate="blocking-human"`) task in Plan 03. See Manual-Only below and `21-RESEARCH.md` Open
      Question 2. **This is the only Wave-0 item that cannot close at execution time without a human.**

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

- [x] All tasks have `<automated>` verify or a Wave 0 / manual-only dependency
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (IN-01 test names **resolved at planning time**; App provisioning carried as a `checkpoint:human-verify`)
- [x] No watch-mode flags
- [x] No E2E suites introduced
- [x] Feedback latency < 120s
- [x] `gofmt -l internal/server/` appears in all three of Plan 02's task verify blocks (CI precheck is stricter than golangci — Phase-20 incident)
- [x] Plans 02/03 use only granular subtargets (no bare `task`) — honors Plan 01's soft ordering dependency
- [x] REQ-ci-renovate-spa-drift's self-heal path is **manual-only** and cannot be marked green by any automated gate
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-07-15 — `gsd-plan-checker` VERIFICATION PASSED (iteration 1, no blockers; Dimension 8 PASS).

**Deliberately left unchecked:** `wave_0_complete: false` — flips to `true` only when execution lands the Wave-0 items. The App-provisioning item cannot close without a human.
