---
phase: "3"
slug: "plugin-first-delivery"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
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
| 3-01-01 | 01 | 1 | REQ-plugin-capability-detection, REQ-plugin-install-or-update, REQ-plugin-three-way-state | T-03-01, T-03-02 | claude-code lane: ONE `claude plugin list --json` probe = capability + state (D-10); `marketplace list` probed in preview too (D-11); local stdlib SemVer-core comparator (`<` outdated, `==` current, `>` current+note, dev/non-core never compares — D-01/D-03); absent → `marketplace add seanb4t/engram --scope user` (only when absent) + `install engram@engram --scope user --json -y`; outdated → `update … --json -y`; current → zero actions; probe failure → `unavailable` + reason after exactly one Run, never a failed outcome (D-12); only engram's own source (D-04/D-05) | unit (fake Environment) | `go test ./internal/setup/ -run '^TestPlugin' -count=1 -v` | ✅ | ✅ green |
| 3-01-02 | 01 | 1 | REQ-plugin-capability-detection, REQ-plugin-install-or-update, REQ-plugin-three-way-state | T-03-01, T-03-02 | codex lane: `installed[]` parse by name AND marketplaceName; two-column marketplace parse; `marketplace add https://github.com/seanb4t/engram --json` (only when absent) + `add engram@engram --json`; outdated → `remove engram@engram --json` THEN `add` (D-02); opencode/generic issue zero plugin Run calls; empty binary → not attempted | unit (fake Environment) | `go test ./internal/setup/ -run '^TestPlugin' -count=1 -v && go test ./internal/setup/ -run '^TestPluginRuntimeIsOptional$' -count=1` | ✅ | ✅ green |
| 3-02-01 | 02 | 1 | REQ-plugin-skips-skills-copy | T-03-03 | `skills.Environment.Lstat` seam (`os.Lstat` in production) + read-only `DetectPresence`: count of present skills, symlink vs copy, `AGENTS.md` index block via `scanBlock` — Lstat/ReadFile only, never a write or delete (D-08, D-09) | unit (fake Lstat + real TempDir symlink) | `go test ./internal/skills/ -run '^TestDetectPresence$' -count=1 -v && go test ./internal/skills/ -count=1` | ✅ | ✅ green |
| 3-03-01 | 03 | 2 | REQ-plugin-facet-reported, REQ-plugin-skips-skills-copy, REQ-plugin-install-or-update, REQ-plugin-three-way-state | T-03-03, T-03-04 | seven flat-scalar `plugin*` row fields + `skills_native` (text + `--output json`); `registration=wrote` beside `plugin=failed` on one row → `exitPartial` (8); `Delivered()` runtime → `skills.Install` never called, zero WriteFile/MkdirAll, `skills=plugin-delivered`, leftover-native report, `AGENTS.md` untouched (D-07/D-08/D-09); preview shows exact `plugin_command` and runs no write verb; `TestSetupGeneratedInvocations` pins each PluginRuntime's list probe in the exact argv sequence | unit (fake Environment + fake skills env) | `go test ./cmd/engram -run '^TestSetupApplyJSONEmitsPluginFacet$' -count=1 && go test ./cmd/engram -run '^TestSetupPluginDeliveredRuntimeAuthorsZeroNativeWrites$' -count=1 && go test ./cmd/engram -run '^TestSetupPreviewShowsPluginArgv$' -count=1 && go test ./cmd/engram -run '^TestSetupGeneratedInvocations$' -count=1` | ✅ | ✅ green |
| 3-03-02 | 03 | 2 | REQ-plugin-capability-detection, REQ-plugin-facet-reported | T-03-04 | probe failure/timeout → `outcome=wrote`, `plugin` empty, `plugin_state=unavailable`, `plugin_note` carries `exited 1` / `timed out after 20s`, native copy written (D-12); `--help` describes plugin-first delivery with no destination path segment and no stale claim; plugin-facet fixtures pass the flat-scalar identity gate; help/catalog goldens regenerated in the same commit | unit + fixture + golden | `go test ./cmd/engram -run '^TestSetupPluginUnavailableFallsBackToNative$' -count=1 && go test ./cmd/engram -run '^TestSetupHelpNamesPluginDelivery$' -count=1 && go test ./cmd/engram -run '^TestOperatorViewFixturesHaveNoUnsanitizedNesting$' -count=1 && go test ./cmd/engram -run 'TestHelpGolden' -count=1 && go test ./cmd/engram -run 'TestCatalogGolden' -count=1` | ✅ | ✅ green |
| 3-04-01 | 04 | 2 | REQ-codex-plugin-manifest | T-03-05, T-03-13 | `skill/engram/.codex-plugin/plugin.json` exists with exactly `$schema`, `name`, `version`, `description`; identity fields byte-equal to `.claude-plugin/plugin.json`; `release-please-config.json` has exactly one `$.version` sync entry for it; no vendor/private substrings (pytest guard) | unit + pytest | `go test ./internal/setupgen/ -run '^TestPluginManifestIdentityMatches$' -count=1 && uv run --with pytest pytest skill/engram/hooks/tests -q` | ✅ | ✅ green |
| 3-04-02 | 04 | 2 | REQ-plugin-setupgen-regenerated | T-03-14 | `Render(planFn, pluginFn)` appends the `### Claude Code plugin delivery (--apply)` table (absent×2, outdated, current) from claude-code's REAL `PluginActions`; shipped tables byte-identical; region regenerated by `go run ./internal/surfacesgen` in the same commit; `--check-setup` drift gate green; prose explains plugin-first delivery and never-removed | unit (drift gate) | `go test ./internal/setupgen/ -run '^TestRenderPluginTable$' -count=1 && go test ./internal/setupgen/ -count=1 && go run ./internal/surfacesgen --check-setup` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Task IDs reconciled 2026-09-14 against the four emitted PLAN.md files (03-01 … 03-04; each plan's final gate task is not listed — it re-runs `task` + `task license:check` + `git diff --exit-code -- go.mod go.sum` + `go test ./internal/keylinks/ -count=1`).*

---

## Wave 0 Requirements

- [x] `internal/setup/plugin_test.go` — new file (plan 03-01): `TestPluginVersionCompare` (D-01/D-03 boundary table incl. `dev`, derived dev, `v`-prefix, leading zero, prerelease, numeric ordering), `TestPluginPlan` (`claude-code/` + `codex/` subtests × every state, both lanes, exact argv sequence), `TestPluginCapabilityProbeFailureFallsBackToNative` (exit 1, timeout, empty, non-JSON, wrong shape, marketplace probe failure), `TestPluginRuntimeIsOptional`
- [x] `internal/skills/presence_test.go` — new file (plan 03-02): `TestDetectPresence` with a fake `Lstat` closure (`fakeFileInfo`) and a real `t.TempDir()` symlink subtest
- [x] `cmd/engram/setup_test.go` — (plan 03-03) `fakeSkillsEnv` gains `Lstat`; `fakeSkillsEnvWithEntries`, `fakePluginRun`, `withFakeSetupVersion`; `TestSetupApplyJSONEmitsPluginFacet`, `TestSetupPluginDeliveredRuntimeAuthorsZeroNativeWrites`, `TestSetupPreviewShowsPluginArgv`, `TestSetupPluginUnavailableFallsBackToNative`, `TestSetupHelpNamesPluginDelivery`; `cmd/engram/setup_delegation_test.go`'s `TestSetupGeneratedInvocations` expects each `setup.PluginRuntime`'s list probe
- [x] `cmd/engram/operator_view_setup_test.go` — (plan 03-03) plugin-facet fixture rows (flat scalars: delivered-current, failed-beside-wrote, unavailable, delivered-with-leftovers) for `TestOperatorViewFixturesHaveNoUnsanitizedNesting` / `TestSetupViewIdentity` — this sibling file, not `operator_output_test.go` (03-PATTERNS.md)
- [x] `internal/setupgen/manifest_test.go` — new file (plan 03-04): `TestPluginManifestIdentityMatches` (both manifests + release-please entry + vendor-neutral description)
- [x] `internal/setupgen/setupgen_test.go` — (plan 03-04) `TestRenderPluginTable`; existing `Render`/`check`/`write` call sites threaded through the widened signatures
- [x] Framework install: none

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Codex's loader accepts the minimal `.codex-plugin/plugin.json` (no `interface` block) | REQ-codex-plugin-manifest | Needs a real `codex plugin add` write against a real Codex — never run from a test (rule `m45p2b4bp7`, gotcha `ryr82bf2s2`) | Post-release live observation (2026-08-23.01 D-10 pattern): after the next release, `codex plugin marketplace add` + `codex plugin add engram@engram --json` on the maintainer's machine and record the outcome in the phase's deferred/observation note |
| `claude plugin install`/`update` refuse-vs-no-op when already installed | REQ-plugin-install-or-update | Third-party behavior; the D-01 state machine never issues install for `current`, so only the `absent` path depends on it | Post-release live observation alongside the item above |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-09-17

---

## Validation Audit 2026-09-17

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

Retroactive reconciliation by `/gsd-validate-phase 3` at `main` HEAD `fbcf587f` (post-merge of PR #569): every
Per-Task Verification Map command was re-run and is green. The full gate (`task` == lint + test, and
`task license:check`) ran once at this HEAD and passed; every gate row cites that single run rather than
repeating it. Wave 0 files exist and are the tests the rows name.
