---
phase: "3"
slug: "plugin-first-delivery"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-14"
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go stdlib `testing`) + golden files (`cmd/engram/testdata/*.golden`, `-update`) + pytest guard (`skill/engram/hooks/tests`) |
| **Config file** | none — `go test ./...` via `task test:go`; surfaces regen via `task surfaces:gen` / `go run ./internal/surfacesgen --check-setup` |
| **Quick run command** | `go test ./internal/setup/... ./cmd/engram/... ./internal/setupgen/... ./internal/skills/... -count=1` |
| **Full suite command** | `task` (== `task lint` + `task test`) |
| **Estimated runtime** | ~120 seconds (quick runs < 20s) |

---

## Sampling Rate

- **After every task commit:** Run the specific `-run` command(s) for the package(s) touched (always `-count=1`; never `-count>1` on `cmd/engram`)
- **After every plan wave:** Run the quick-run command across all four packages
- **Before `/gsd-verify-work`:** `task` green; regenerated artifacts (`help.golden`, `catalog.golden`, `skill/engram/commands/engram-setup.md`) committed in the SAME change as their cause; `git diff --exit-code go.mod go.sum` (`internal/setup` stays a stdlib-only leaf — `TestSetupPackageIsStdlibOnlyLeaf`); `uv run --with pytest pytest skill/engram/hooks/tests -q` (shipped-bundle privacy guard covers the new `.codex-plugin/plugin.json`)
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 3-01-01 | 01 | 1 | REQ-plugin-three-way-state | T-03-02 | Local stdlib SemVer-core comparator: `<` → outdated, `==` → current, `>` → current+note, non-SemVer (`dev`) → never triggers | unit (table) | `go test ./internal/setup/ -run TestPluginVersionCompare -count=1` | ❌ W0 | ⬜ pending |
| 3-01-02 | 01 | 1 | REQ-plugin-capability-detection, REQ-plugin-install-or-update | T-03-01, T-03-02 | One `plugin list --json` read probe = capability + state; marketplace `list` probed in preview; absent → marketplace add + install(-y)/add; outdated → update(-y) / remove+add; current → zero actions; probe failure → native path + `unavailable: <reason>`, never a failed row; only `engram@engram` from engram's own source | unit (fake Environment) | `go test ./internal/setup/ -run 'TestPluginPlan\|TestPluginCapabilityProbeFailureFallsBackToNative' -count=1` | ❌ W0 | ⬜ pending |
| 3-02-01 | 02 | 2 | REQ-plugin-skips-skills-copy | T-03-03 | Plugin-delivered runtime: `skills.Install` never called, zero native writes, no `AGENTS.md` action; existing native copies / index block reported (symlink vs copy via new `Lstat` seam), never deleted | unit (negative-space) | `go test ./cmd/engram -run TestSetupPluginDeliveredRuntimeAuthorsZeroNativeWrites -count=1 && go test ./internal/skills/ -count=1` | ❌ W0 | ⬜ pending |
| 3-02-02 | 02 | 2 | REQ-plugin-facet-reported | T-03-04 | Plugin facet as flat scalars in text + `--output json`; `failed` plugin beside `wrote` registration → both visible, `exitPartial` (8) | unit + fixture | `go test ./cmd/engram -run 'TestSetupApplyJSONEmitsPluginFacet\|TestOperatorViewFixturesHaveNoUnsanitizedNesting\|TestSetupExitCode' -count=1` | ❌ W0 | ⬜ pending |
| 3-03-01 | 03 | 3 | REQ-codex-plugin-manifest | T-03-05 | `skill/engram/.codex-plugin/plugin.json` exists (minimal schema: `$schema`, `name`, `version`, `description`); identity fields equal `.claude-plugin/plugin.json`; `release-please-config.json` has the `$.version` sync entry; no vendor/private substrings | unit + pytest | `go test ./internal/setupgen/ -run TestPluginManifestIdentityMatches -count=1 && uv run --with pytest pytest skill/engram/hooks/tests -q` | ❌ W0 | ⬜ pending |
| 3-03-02 | 03 | 3 | REQ-plugin-setupgen-regenerated | — | Generated `/engram-setup` prose reflects plugin actions/outcomes; `--check-setup` gate green; help/catalog goldens regenerated in the same commit | unit (drift gate) | `go test ./internal/setupgen/ -count=1 && go run ./internal/surfacesgen --check-setup && go test ./cmd/engram -run 'TestHelpGolden\|TestCatalogGolden' -count=1` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Task IDs are provisional until the planner assigns plan/task numbers; the planner MUST reconcile this table against the emitted PLAN.md files.*

---

## Wave 0 Requirements

- [ ] `internal/setup/plugin_test.go` — new file: `TestPluginVersionCompare` (comparator boundary table incl. `dev`), `TestPluginPlan`, `TestPluginCapabilityProbeFailureFallsBackToNative`
- [ ] `cmd/engram/setup_test.go` — plugin-lane fixtures via `fakeSetupEnvWithRun` scripting `plugin list --json` / `marketplace list` responses; `TestSetupPluginDeliveredRuntimeAuthorsZeroNativeWrites`, `TestSetupApplyJSONEmitsPluginFacet`
- [ ] `cmd/engram/operator_output_test.go` — plugin-facet fixture row(s) (flat scalars) for `TestOperatorViewFixturesHaveNoUnsanitizedNesting`
- [ ] `internal/skills/*_test.go` — fake `Lstat` closure on the skills `Environment` for the D-08 symlink-vs-copy report
- [ ] `internal/setupgen/setupgen_test.go` — `TestPluginManifestIdentityMatches` (both manifests + release-please entry)
- [ ] Framework install: none

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Codex's loader accepts the minimal `.codex-plugin/plugin.json` (no `interface` block) | REQ-codex-plugin-manifest | Needs a real `codex plugin add` write against a real Codex — never run from a test (rule `m45p2b4bp7`, gotcha `ryr82bf2s2`) | Post-release live observation (2026-08-23.01 D-10 pattern): after the next release, `codex plugin marketplace add` + `codex plugin add engram@engram --json` on the maintainer's machine and record the outcome in the phase's deferred/observation note |
| `claude plugin install`/`update` refuse-vs-no-op when already installed | REQ-plugin-install-or-update | Third-party behavior; the D-01 state machine never issues install for `current`, so only the `absent` path depends on it | Post-release live observation alongside the item above |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
