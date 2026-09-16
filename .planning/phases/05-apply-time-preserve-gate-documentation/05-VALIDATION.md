---
phase: "5"
slug: "apply-time-preserve-gate-documentation"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-16"
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (no third-party test framework anywhere in this repo) |
| **Config file** | none — `go test` is invoked directly, per `Taskfile.yaml`'s `test:go` task |
| **Quick run command** | `go test ./internal/setup/... ./cmd/engram/... -count=1` |
| **Full suite command** | `task test` (lint + `go test ./...`) |
| **Estimated runtime** | ~60 seconds (quick) / ~180 seconds (full; `internal/store` needs Docker) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/setup/... ./cmd/engram/... -count=1`
- **After every plan wave:** Run `task test`
- **Before `/gsd-verify-work`:** Full suite green; `05-POST-RELEASE.md` exists and names every qualifying-release check (D-06)
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 05-01-01 | 01 | 1 | REQ-apply-preserve-gate | T-05-01 | `--apply` on a preserved Claude Code registration (observed `x-litellm-api-key`) issues zero `mcp remove`/`mcp add` calls, exits 0, plugin/skills facets still populate, literal never leaks | unit (process boundary, `cmd/engram`) + package (`internal/setup`) | `go test ./cmd/engram -run '^TestSetupApplyPreservedRuntimeSkipsRegistrationWrite$' -count=1 -v && go test ./internal/setup/ ./cmd/engram/ -count=1` | ❌ W0 (new test; `drift_test.go` claude-code Reason pins retargeted) | ⬜ pending |
| 05-01-02 | 01 | 1 | REQ-apply-preserve-gate | T-05-01 / T-05-03 / T-05-04 | already-correct is a one-call no-op on both parsed runtimes; would-write writes then re-observes redaction-safe (post-write literal absent from `json.Marshal(Result)`); ambiguity still writes | unit (`internal/setup`) | `go test ./internal/setup/ -run '^(TestApplyPreservedIssuesZeroWrites\|TestApplyPreservedNeverRunsClaudeCodeRemove\|TestApplyAlreadyCorrectIssuesZeroWrites\|TestApplyWroteRegisteredIsRedacted\|TestApplyConvergesCodex\|TestApplyConvergesClaudeCode\|TestThirdPartyCaptureIsQuotedForDisplay)$' -count=1 -v` | ❌ W0 (four new tests; two existing retargeted to D-01) | ⬜ pending |
| 05-01-03 | 01 | 1 | REQ-apply-rewrite-consequence | T-05-02 | OAuth re-login note on Notes for an `AuthNone` would-write Claude Code row in both lanes; never on bearer/foreign/preserved/already-correct/ambiguous/codex; shape only, never state | unit (`internal/setup`) + plan gate | `go test ./internal/setup/ -run '^(TestOAuthReLoginConsequence\|TestObserveClaudeCodeRegistration)$' -count=1 -v && task && go test ./internal/keylinks/ -count=1 && go test ./internal/setup/ -count=1 -shuffle=on && go run ./internal/surfacesgen --check-setup` | ❌ W0 (new test in `claudecode_test.go`) | ⬜ pending |
| 05-02-01 | 02 | 2 | REQ-apply-preserve-gate, REQ-apply-rewrite-consequence | T-05-03 / T-05-05 / T-05-09 | `--help` states the gate, remediation, re-login; `help.golden` pinned; `catalog.golden` untouched; the observed literal never crosses the process boundary under `--apply` in either lane | unit (`cmd/engram`) | `go test ./cmd/engram -run '^(TestSetupHelpStatesApplyGate\|TestHelpGolden\|TestCatalogGolden\|TestSetupJSONNeverLeaksProbeLiteral)$' -count=1 -v && test -z "$(git status --porcelain -- cmd/engram/testdata/catalog.golden)"` | ❌ W0 (new help test; `claude-code-observed-shape` gains an `apply` mode) | ⬜ pending |
| 05-02-02 | 02 | 2 | REQ-docs-setup-v2 | T-05-08 / T-05-09 | `agent-setup.md` states the apply gate, corrected `already-correct`, preserved remediation (both runtimes), OAuth re-login, plugin-first, unreleased notice; the two stale sentences occur zero times | docs gate (`cmd/engram`) + site build | `go test ./cmd/engram -run '^(TestAgentSetupGuideDocumentsDrift\|TestAgentSetupGuideDriftGateFiresOnInjectedViolation)$' -count=1 -v && rumdl check docs-site/src/content/docs/guides/agent-setup.md && pnpm --dir docs-site install --frozen-lockfile && pnpm --dir docs-site build` | ✅ exists — extended with legs 8-13 + nine positive-control cases | ⬜ pending |
| 05-03-01 | 03 | 1 | REQ-docs-setup-v2 | T-05-11 / T-05-12 | `install.md` names the cask contents (completions + man pages, `man engram-setup`), the plugin-first pointer, and the unreleased notice; hidden `engram man` verb not documented | docs gate (`cmd/engram`) | `go test ./cmd/engram -run '^(TestInstallGuideDocumentsSetupV2\|TestInstallGuideGateFiresOnInjectedViolation)$' -count=1 -v && rumdl check docs-site/src/content/docs/guides/install.md` | ❌ W0 (new file `cmd/engram/install_docs_test.go`) | ⬜ pending |
| 05-03-02 | 03 | 1 | REQ-docs-setup-v2 | T-05-11 / T-05-13 | `plugin.md` states plugin-first via `engram setup --apply`, cross-links `preserved` to the setup guide beside its fallback `mcp remove` block, unreleased notice | docs gate (`cmd/engram`) + plan gate | `go test ./cmd/engram -run '^(TestPluginGuideDocumentsPluginFirst\|TestPluginGuideGateFiresOnInjectedViolation)$' -count=1 -v && pnpm --dir docs-site install --frozen-lockfile && pnpm --dir docs-site build && task && go test ./internal/keylinks/ -count=1` | ❌ W0 (new file `cmd/engram/plugin_docs_test.go`) | ⬜ pending |
| 05-04-01 | 04 | 3 | REQ-docs-setup-v2 | T-05-15 / T-05-16 / T-05-17 | `05-POST-RELEASE.md` in the 06 precedent's exact shape (`phase`/`status: pending`/`tracker`; four headings), a live OPEN tracking issue, REQ-docs-setup-v2 still `[ ]`, no `05-RELEASE-*.md` | artifact check (shell) | `f=.planning/phases/05-apply-time-preserve-gate-documentation/05-POST-RELEASE.md; rg -q -e '^status: pending$' "$f" && url=$(rg -o -e 'https://github[.]com/seanb4t/engram/issues/[0-9]+' "$f" \| head -1) && gh issue view "$url" --json state --jq .state \| rg -q OPEN && rg -q -F -e '- [ ] **REQ-docs-setup-v2**' .planning/REQUIREMENTS.md` | ❌ W0 (new planning artifact — not a Go test) | ⬜ pending |
| 05-04-02 | 04 | 3 | REQ-docs-setup-v2 (plus phase-wide REQ-apply-*) | T-05-05 / T-05-18 | phase gate green; generated surfaces clean; zero replace-registration flag occurrences; docs requirement visibly open | phase gate (shell) | `task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1 && go test ./internal/setup/ -count=1 -shuffle=on && go test ./cmd/engram/ -count=1 && go run ./internal/surfacesgen --check-setup && pnpm --dir docs-site install --frozen-lockfile && pnpm --dir docs-site build` | ✅ (repo gates exist; `TestRedEvidencePatchesAreLive` turns green once the orchestrator registers the phase's patches) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*(Seeded by plan-phase; gsd-planner fills one row per task from the plans it authors. The `\|` inside `-run` filters is the table-cell escape for a literal `|` — run the commands as written in each PLAN.md's `<automated>` block, not from this table.)*

**Post-release (Manual-Only, D-06):** `REQ-docs-setup-v2` stays `[ ]` and this phase's VERIFICATION.md carries `post_release_status: pending` (+ `post_release_tracker`) until a human records `05-RELEASE-<ver>.md` per `05-POST-RELEASE.md`. Phase 5 verification PASSES in that state.

---

## Wave 0 Requirements

- [ ] `internal/setup/apply_test.go` — new tests through the existing `fakeEnvWithRun`/`scriptedRun` harness: `--apply` on a `preserved`-classifying fixture issues zero registration-write `Run` calls (SC1); Claude Code's `mcp remove` argv never appears among recorded calls (SC2); `already-correct` is the same pre-write zero-write case (D-01); `would-write` still runs `plan.Actions` → `wrote` and a second `Apply` converges; `Registered` after a write is rebuilt via `Observe`/`renderObservation`, never raw `displayCapture` (D-02)
- [ ] `internal/setup/apply_test.go` — EXISTING `TestApplyConvergesCodex`/`TestApplyConvergesClaudeCode` retargeted to D-01 semantics (`already-correct` is now a pre-write claim for drift runtimes)
- [ ] `internal/setup/claudecode_test.go` — a `would-write` Claude Code fixture whose observed auth shape is oauth/oauth-client carries the OAuth re-login note in BOTH `Preview` and `Apply` `Notes` (D-03/D-04); a bearer-shaped fixture does NOT
- [ ] `cmd/engram/setup_test.go` — `--apply` at the process boundary: a `preserved` row still populates plugin/skills facets, zero registration writes, exit 0
- [ ] `cmd/engram/install_docs_test.go` — new `agent_setup_docs_test.go`-shaped gate for `install.md` (man pages `man engram-setup`, cask contents, plugin-first pointer)
- [ ] `cmd/engram/agent_setup_docs_test.go` — extended legs: apply gate stated; corrected `already-correct` semantics (the "does not guarantee no write ran" sentence must be GONE); OAuth re-login consequence; manual remediation
- [ ] `cmd/engram/plugin_docs_test.go` (or equivalent) — gate for `plugin.md`'s cross-link to `agent-setup.md` and plugin-first description
- [ ] `.planning/phases/05-apply-time-preserve-gate-documentation/05-POST-RELEASE.md` — NOT a Go test; the D-06 human-handoff deliverable, shaped like `06-POST-RELEASE.md`

*RED-evidence approach: tests referencing not-yet-existing symbols or asserting the new zero-write behavior against the pre-Phase-5 `mutate` branch fail (compile failure or assertion failure — both valid RED in Go). Run each new test individually with `-run '^Name$' -v` before its GREEN commit (gotcha `bsbsvn4hbc`).*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Post-release live observation: `brew install` of the qualifying release; `engram setup` plugin-first + `--header` + `preserved` + the apply gate on a real machine; man pages present in the cask | REQ-docs-setup-v2 | Rule `m45p2b4bp7` — no test invokes a real CLI or touches `$HOME`; the D-10 pattern records the observation after a release, never from code alone | Follow `05-POST-RELEASE.md` after the next release; record `05-RELEASE-<ver>.md` (shape of `06-RELEASE-0.16.0.md`); flip `post_release_status` in `05-VERIFICATION.md` and check off REQ-docs-setup-v2 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
