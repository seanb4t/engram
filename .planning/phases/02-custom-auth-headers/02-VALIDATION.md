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
| 2-01-01 | 01 | 1 | REQ-header-name-parameter, REQ-header-value-env-ref-only | T-02-03 | `Options.Headers`/`HeaderSpec`/`ErrHeaderUnsupported`/`sortedHeaders`; claude-code renders sorted `--header 'NAME: ${ENVVAR}'` pairs on its single add action in every mode; `internal/setup` never reads a header var; second sentinel absent from Args/Config for every runtime × mode | unit (tracer + negative-space) | `go test ./internal/setup/ -run '^(TestClaudeCodeHeaders\|TestClaudeCodePlan\|TestNoSecretInArgs)$' -count=1` | ❌ W0 (`TestClaudeCodeHeaders` new; `TestNoSecretInArgs` extended) | ⬜ pending |
| 2-01-02 | 01 | 1 | REQ-header-codex-declined | T-02-04, T-02-07 | Codex `Plan()` guard is its first statement; zero Plan + exact `ErrHeaderUnsupported`-wrapped reason naming header(s)/gap/remedy; never `ErrAuthModeUnsupported`; never TOML or the config-override flag | unit | `go test ./internal/setup/ -run '^(TestCodexDeclinesHeaders\|TestCodexClientID\|TestNoSecretInArgs)$' -count=1` | ❌ W0 (`TestCodexDeclinesHeaders` new) | ⬜ pending |
| 2-01-03 | 01 | 1 | (plan gate) | T-02-SC | Zero new deps; stdlib-only leaf; key-links gate green | gate | `task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1 && go test ./internal/setup/ -count=1 -shuffle=on` | ✅ | ⬜ pending |
| 2-02-01 | 02 | 2 | REQ-header-name-parameter | T-02-09 | opencode renders sorted `--header 'NAME={env:ENVVAR}'` pairs in its own file (no shared formatter); `oauth-client` still declined with the auth sentinel | unit (tracer) | `go test ./internal/setup/ -run '^(TestOpenCodeHeaders\|TestOpenCodePlan\|TestOpenCodeBearerHeaderSyntax\|TestNoSecretInArgs)$' -count=1` | ❌ W0 (`TestOpenCodeHeaders` new) | ⬜ pending |
| 2-02-02 | 02 | 2 | REQ-header-name-parameter, REQ-header-value-env-ref-only, REQ-header-bearer-unchanged | T-02-03, T-02-10 | generic `headers` object carries `${ENVVAR}` extras Authorization-first then sorted via `genericHeaders.MarshalJSON`; three zero-header Config literals byte-identical to HEAD; `TestNoSecretInArgs` positive control (env var NAME present) | unit (byte-identity + negative/positive space) | `go test ./internal/setup/ -run '^(TestGenericHeaders\|TestGenericConfig\|TestGenericConfigCarriesNoSecret\|TestNoSecretInArgs)$' -count=1` | ❌ W0 (`TestGenericHeaders` new) | ⬜ pending |
| 2-02-03 | 02 | 2 | (plan gate) | T-02-SC | Zero new deps; key-links gate green | gate | `task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1 && go test ./internal/setup/ -count=1 -shuffle=on` | ✅ | ⬜ pending |
| 2-03-01 | 03 | 3 | REQ-header-name-parameter, REQ-header-value-env-ref-only, REQ-header-codex-declined | T-02-01, T-02-02, T-02-03, T-02-11, T-02-12 | `--header` flag + `ENGRAM_HEADERS` default (no registry row); four exact usage errors (Authorization collision / malformed NAME / never-a-value ENVVAR / duplicate) with `effects == 0` in both lanes and no right-hand-side echo; codex decline via CLI with `exitSetupFailed`/`exitPartial`; valid with every `--auth` mode; goldens regenerated in the same commit | unit (tracer) | `go test ./cmd/engram -run '^(TestSetupParseHeaders\|TestSetupHeaderEnvDefaultReadsEnv\|TestSetupHeaderRejectsAuthorizationCollision\|TestSetupHeaderRejectsMalformedName\|TestSetupHeaderRejectsMalformedEnvVar\|TestSetupHeaderRejectsDuplicateName\|TestSetupHeaderCodexDeclined\|TestSetupHeaderValidWithEveryAuthMode\|TestHelpGolden\|TestCatalogGolden\|TestSetupGeneratedInvocations)$' -count=1` | ❌ W0 (eight new `TestSetupHeader*`/`TestSetupParseHeaders`) | ⬜ pending |
| 2-03-02 | 03 | 3 | REQ-header-documented, REQ-header-bearer-unchanged | T-02-06 | Flat-scalar `headers` row facet in D-08 order; `--output json` byte-identical across header orderings; help paragraph + fifth example; `Accepted --auth modes` block byte-identical to `ceabd8e9`; fixtures pass the nesting guard | unit (help-golden + fixtures) | `go test ./cmd/engram -run '^(TestSetupHeaderOrderIndependent\|TestSetupHelpNamesEveryRuntimeAndAuthMode\|TestSetupHelpClientIDContract\|TestSetupViewIdentity\|TestOperatorViewFixturesHaveNoUnsanitizedNesting\|TestHelpGolden\|TestCatalogGolden\|TestSetupGeneratedInvocations\|TestSetupReportCoversEveryRuntimeShape)$' -count=1` | ❌ W0 (`TestSetupHeaderOrderIndependent` new; `TestSetupHelpNamesEveryRuntimeAndAuthMode` extended; two fixtures) | ⬜ pending |
| 2-03-03 | 03 | 3 | (plan gate) | T-02-SC | Goldens committed and non-drifting; setupgen region unchanged until 02-04 | gate | `task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1 && go test ./cmd/engram -run 'TestHelpGolden\|TestCatalogGolden' -count=1 && test -z "$(git status --porcelain -- cmd/engram/testdata/)" && go run ./internal/surfacesgen --check-setup` | ✅ | ⬜ pending |
| 2-04-01 | 04 | 4 | REQ-header-documented, REQ-header-bearer-unchanged, REQ-header-codex-declined | T-02-13, T-02-04 | Fifth `bearer+header` `Cases()` entry; four shipped table rows byte-identical; `TestSetupGeneratedInvocations` runs it end-to-end (codex `failed`, `exitPartial` under apply); region regenerated in the same commit | unit (setupgen conformance, tracer) | `go test ./internal/setupgen/ -count=1 && go test ./cmd/engram -run '^TestSetupGeneratedInvocations$' -count=1 && go run ./internal/surfacesgen --check-setup` | ✅ (both tests extended) | ⬜ pending |
| 2-04-02 | 04 | 4 | REQ-header-documented | T-02-14, T-02-04 | `/engram-setup` prose + `agent-setup.md` gateway subsection, `--header` table column, `ENGRAM_HEADERS`, Codex limitation; no literal secret; anchored region untouched | grep + lint (prose read is manual, see below) | `rg -q -F '### Gateway headers' docs-site/src/content/docs/guides/agent-setup.md && rg -q -F 'Gateway header (optional)' skill/engram/commands/engram-setup.md && go run ./internal/surfacesgen --check-setup && task lint` | n/a (docs) | ⬜ pending |
| 2-04-03 | 04 | 4 | (plan gate) | T-02-SC | Every generated surface committed and non-drifting | gate | `task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1 && go run ./internal/surfacesgen --check-setup && go test ./cmd/engram -run 'TestHelpGolden\|TestCatalogGolden' -count=1` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Task IDs reconciled by the planner against `02-01-PLAN.md` … `02-04-PLAN.md` (four plans, four sequential waves, three tasks each; the third task of every plan is the plan gate).*

---

## Wave 0 Requirements

- [ ] `internal/setup/plan_test.go` — extend `TestNoSecretInArgs` over `Options.Headers` with a second sentinel (02-01 Task 1) and the positive env-var-NAME control (02-02 Task 2)
- [ ] `internal/setup/claudecode_test.go` — `TestClaudeCodeHeaders` (02-01 Task 1); `internal/setup/codex_test.go` — `TestCodexDeclinesHeaders` (02-01 Task 2)
- [ ] `internal/setup/opencode_test.go` — `TestOpenCodeHeaders` (02-02 Task 1); `internal/setup/generic_test.go` — `TestGenericHeaders` with the three zero-header byte-identity literals (02-02 Task 2)
- [ ] `cmd/engram/setup_test.go` — `TestSetupParseHeaders`, `TestSetupHeaderEnvDefaultReadsEnv`, `TestSetupHeaderRejectsAuthorizationCollision`, `TestSetupHeaderRejectsMalformedName`, `TestSetupHeaderRejectsMalformedEnvVar`, `TestSetupHeaderRejectsDuplicateName` (modeled on `TestSetupClientID`), `TestSetupHeaderCodexDeclined` (modeled on `TestSetupUnsupportedAuthModeIsFailedRow`), `TestSetupHeaderValidWithEveryAuthMode` (02-03 Task 1); `TestSetupHeaderOrderIndependent` + `TestSetupHelpNamesEveryRuntimeAndAuthMode` extension (02-03 Task 2)
- [ ] `cmd/engram/clienttest_test.go` — `setupHeaders = nil` in `resetClientFlags`' cleanup; `cmd/engram/golden_test.go` — `envDerivedFlagDefaults["setup"]["header"]` (02-03 Task 1)
- [ ] `cmd/engram/operator_view_setup_test.go` — `headerGateway` and `codexHeaderDeclined` fixture rows carrying the header facet as a FLAT scalar (feeds `TestOperatorViewFixturesHaveNoUnsanitizedNesting`) (02-03 Task 2)
- [ ] `cmd/engram/testdata/{help,catalog}.golden` — regenerated via `go test ./cmd/engram -run 'TestHelpGolden|TestCatalogGolden' -update -count=1` in the SAME commit as each `setup.go` change (02-03 Tasks 1 and 2)
- [ ] `internal/setupgen/setupgen_test.go` — `TestRenderRealPlans` over five labeled cases; `cmd/engram/setup_delegation_test.go` — header-aware `TestSetupGeneratedInvocations`; regenerate `skill/engram/commands/engram-setup.md` via `go run ./internal/surfacesgen` (the setupgen step of `task surfaces:gen`) in the same commit (02-04 Task 1)
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
