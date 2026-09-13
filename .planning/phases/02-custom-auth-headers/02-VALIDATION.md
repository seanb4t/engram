---
phase: "2"
slug: "custom-auth-headers"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-13"
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go stdlib `testing`) + golden files (`cmd/engram/testdata/*.golden`, regenerated with `-update`) |
| **Config file** | none — `go test ./...` via `task test:go`; surfaces regen via `task surfaces:gen` |
| **Quick run command** | `go test ./internal/setup/... ./cmd/engram/... ./internal/setupgen/... -count=1` |
| **Full suite command** | `task` (== `task lint` + `task test`) |
| **Estimated runtime** | ~90 seconds (quick runs < 15s) |

---

## Sampling Rate

- **After every task commit:** Run the specific `-run` command(s) for the file(s) touched by that commit (always `-count=1`; never `-count>1` on `cmd/engram`)
- **After every plan wave:** Run `task test:go`
- **Before `/gsd-verify-work`:** `task` green; regenerated goldens (`help.golden`, `catalog.golden`, `skill/engram/commands/engram-setup.md`) committed in the SAME change as the code that changed them; `git diff --exit-code go.mod go.sum`
- **Max feedback latency:** 90 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 2-01-01 | 01 | 1 | REQ-header-name-parameter | — | Per-runtime rendering only; no shared formatter | unit | `go test ./internal/setup/ -run TestHeader -count=1` | ❌ W0 | ⬜ pending |
| 2-01-02 | 01 | 1 | REQ-header-value-env-ref-only | T-02-01 | Env var NAME only; `internal/setup` never `os.Getenv`s a header var; sentinel value absent from argv/preview/json | unit (negative-space) | `go test ./internal/setup/ -run TestNoSecretInArgs -count=1` | ✅ | ⬜ pending |
| 2-01-03 | 01 | 1 | REQ-header-codex-declined | T-02-02 | Codex row `failed` via `ErrHeaderUnsupported`; zero write actions; never TOML | unit | `go test ./cmd/engram/ -run TestSetupHeaderCodexDeclined -count=1` | ❌ W0 | ⬜ pending |
| 2-02-01 | 02 | 1 | REQ-header-name-parameter, REQ-header-value-env-ref-only | T-02-03 | D-02/D-03 usage errors: Authorization collision, malformed NAME/ENVVAR, duplicates, literal-looking values | unit | `go test ./cmd/engram/ -run 'TestSetupHeaderRejects' -count=1` | ❌ W0 | ⬜ pending |
| 2-02-02 | 02 | 1 | REQ-header-bearer-unchanged | — | Four no-header modes byte-identical to 2026-08-23.01 | unit (golden / byte-identity) | `go test ./cmd/engram/ -run 'TestSetupGeneratedInvocations|TestHelpGolden' -count=1` | ✅ | ⬜ pending |
| 2-02-03 | 02 | 1 | REQ-header-documented | — | Help + regenerated `/engram-setup` prose + `agent-setup.md` show the gateway shape and the Codex limitation | unit (help-golden + setupgen conformance) | `go test ./cmd/engram/ -run TestSetupHelpNamesEveryRuntimeAndAuthMode -count=1 && go test ./internal/setupgen/ -count=1` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Task IDs are provisional until the planner assigns plan/task numbers; the planner MUST reconcile this table against the emitted PLAN.md files.*

---

## Wave 0 Requirements

- [ ] `cmd/engram/setup_test.go` — `TestSetupHeaderCodexDeclined` (modeled on `TestSetupUnsupportedAuthModeIsFailedRow`); `TestSetupHeaderRejectsAuthorizationCollision`, `TestSetupHeaderRejectsMalformedName`, `TestSetupHeaderRejectsMalformedEnvVar`, `TestSetupHeaderRejectsDuplicateName` (modeled on `TestSetupClientID`)
- [ ] `cmd/engram/operator_view_setup_test.go` — fixture row(s) exercising the header facet as a FLAT scalar (feeds `TestOperatorViewFixturesHaveNoUnsanitizedNesting`)
- [ ] `internal/setup/plan_test.go` — extend `TestNoSecretInArgs` to cover `Options.Headers`
- [ ] `internal/setup/{claudecode,opencode,generic,codex}_test.go` — header rendering / decline cases
- [ ] `internal/setupgen/setupgen_test.go` — 5th `Cases()` entry conformance; regenerate `skill/engram/commands/engram-setup.md` via `task surfaces:gen` in the same commit
- [ ] Framework install: none

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `guides/agent-setup.md` gateway example reads correctly on the docs site | REQ-header-documented | Prose quality; no Go gate covers docs-site rendering | Build docs-site locally (`pnpm --dir docs-site build` if the plan adopts it) and read the Authentication section |

*All behavioral requirements have automated verification; docs prose is the only manual item.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
