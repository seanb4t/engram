---
phase: 03-plugin-first-delivery
verified: 2026-09-15T04:24:00Z
status: passed
score: 7/7 must-haves verified
covered_files:
  - ".planning/phases/03-plugin-first-delivery/03-01-PLAN.md"
  - ".planning/phases/03-plugin-first-delivery/03-01-SUMMARY.md"
  - ".planning/phases/03-plugin-first-delivery/03-02-PLAN.md"
  - ".planning/phases/03-plugin-first-delivery/03-02-SUMMARY.md"
  - ".planning/phases/03-plugin-first-delivery/03-03-PLAN.md"
  - ".planning/phases/03-plugin-first-delivery/03-03-SUMMARY.md"
  - ".planning/phases/03-plugin-first-delivery/03-04-PLAN.md"
  - ".planning/phases/03-plugin-first-delivery/03-04-SUMMARY.md"
  - ".planning/phases/03-plugin-first-delivery/03-REVIEW-FIX.md"
  - ".planning/phases/03-plugin-first-delivery/03-REVIEW.md"
  - "cmd/engram/operator_view_setup_test.go"
  - "cmd/engram/setup.go"
  - "cmd/engram/setup_delegation_test.go"
  - "cmd/engram/setup_test.go"
  - "cmd/engram/testdata/help.golden"
  - "internal/setup/claudecode.go"
  - "internal/setup/codex.go"
  - "internal/setup/plugin.go"
  - "internal/setup/plugin_test.go"
  - "internal/setupgen/manifest_test.go"
  - "internal/setupgen/setupgen.go"
  - "internal/setupgen/setupgen_test.go"
  - "internal/skills/environment.go"
  - "internal/skills/presence.go"
  - "internal/skills/presence_test.go"
  - "internal/store/redevidence_harness_test.go"
  - "internal/surfacesgen/main_test.go"
  - "release-please-config.json"
  - "skill/engram/.codex-plugin/plugin.json"
  - "skill/engram/commands/engram-setup.md"
covered_digest: "v1:sha256:cacf89db76943b00245c4cda2dc7bde37d44dedf976525382d3be8fe32b3e0bb"
overrides_applied: 0
behavior_unverified: 0
---

# Phase 3: Plugin-First Delivery Verification Report

**Phase Goal:** Under `--apply`, a plugin-capable Claude Code or Codex receives engram's skills,
hooks, and `/engram-setup` command through its own plugin system — installing or updating only
engram's own marketplace plugin, never a foreign one — while a runtime without a working `plugin`
CLI, and opencode/`generic` regardless, keep the native skills-copy path, mutually exclusive per
runtime per run so `curating-memory` never appears twice.

**Verified:** 2026-09-15T04:24:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1 — capability detection is one `plugin list --json` probe per plugin-capable runtime, distinct from `Detect()`; a nonzero exit, timeout, or unparseable output is `unavailable` with a reason after exactly one Run call and never a failed outcome | ✓ VERIFIED | `internal/setup/plugin.go`'s `executePlugin` returns after step 3/4 on any probe failure; `TestPluginCapabilityProbeFailureFallsBackToNative` (claude-code + codex, 12 subtests total) pins `Outcome == ""`, `Delivered() == false`, and exactly one recorded call for every failure class (exit 1, timeout, empty stdout, non-JSON, wrong shape). Ran directly: all PASS. |
| 2 | SC2 — absent → marketplace add (only when absent) + install; outdated → update (claude-code) or remove-then-add (codex); current → zero actions and already-correct; preview shows the exact argv | ✓ VERIFIED | `TestPluginPlan` (19 subtests across claude-code/codex) asserts exact `Command` strings and exact recorded call sequences for every state, including `current`'s zero-write-verb idempotency and the marketplace-present/fork/absent variants. Ran directly: all PASS. Live preview against the real `claude`/`codex` binaries on this machine (no `--apply`) reproduced the exact authored argv in `plugin_command`. |
| 3 | SC3 (state half) — state is exactly one of absent/outdated/current from a numeric SemVer-core comparison against the binary version; a newer plugin or dev build is `current` with a note and never churned | ✓ VERIFIED | `TestPluginVersionCompare` (12 subtests) pins the numeric-not-lexical ordering, equal-is-current, newer-is-current-with-note, and dev-binary/derived-dev/v-prefixed/leading-zero/prerelease non-comparable cases. Ran directly: all PASS. |
| 4 | SC3 (mutual-exclusion half) — a plugin-delivered runtime never also receives a native skills-copy install or an `AGENTS.md` index block | ✓ VERIFIED | Single call site `skills.Install(...)` (`cmd/engram/setup.go:170`) is reached only via `setupApplySkillsFacet`, which `setupRuntimeRowFromResult` invokes only in the `else` branch of `if p.Delivered() { setupReportNativeSkills } else { setupApplySkillsFacet }` (setup.go:794-798) — routing by capability, never by install success (doc comment lines 750-757). `TestSetupPluginDeliveredRuntimeAuthorsZeroNativeWrites` (executed directly, PASS) counts zero `WriteFile`/`MkdirAll` calls and asserts unchanged seeded `AGENTS.md` bytes across an `--apply` run with both runtimes `current`. Live preview reproduced `skills=plugin-delivered` for claude-code/codex vs `skills=would-write` for opencode in the same run. |
| 5 | SC4 — a result row shows plugin delivery as its own facet in text and JSON, alongside registration and skills, so a `wrote` registration next to a `failed` plugin install stays visible | ✓ VERIFIED | `TestSetupApplyJSONEmitsPluginFacet` (executed directly, PASS) asserts `registration=wrote`, `plugin=failed`, `plugin_state=absent`, row `outcome=failed`, and process `exitCodeFromError == exitPartial` on the same row. `TestOperatorViewFixturesHaveNoUnsanitizedNesting` (PASS) proves every plugin field is a flat sanitized scalar in both renderers. |
| 6 | SC5 (manifest half) — `skill/engram/.codex-plugin/plugin.json` exists, is release-please-synced like `.claude-plugin/plugin.json`, with a drift gate keeping identity fields equal | ✓ VERIFIED | Manifest file inspected directly: 4 keys, `name`/`version`/`description` byte-equal to `.claude-plugin/plugin.json`. `release-please-config.json` carries one `extra-files` entry for each manifest. `TestPluginManifestIdentityMatches` (read directly and executed, PASS) genuinely compares key sets, byte-equality of the three identity fields, version-core shape, and the extra-files entries — not a narration. |
| 7 | SC5 (setupgen half) — `/engram-setup`'s generated prose reflects the new plugin actions/outcomes in the same change, keeping the `setupgen` CI drift gate green | ✓ VERIFIED | `engram-setup.md` carries the `### Claude Code plugin delivery (--apply)` table with the 3 non-trivial states' exact argv, plus hand-authored plugin-first prose. `go run ./internal/surfacesgen --check-setup` exits 0 (ran directly). `TestRenderPluginTable` (PASS) proves the renderer rejects a wrong action count per state or non-`claude plugin` argv before writing a row. |

**Score:** 7/7 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/setup/plugin.go` | `PluginState`, `PluginRuntime`, `PluginResult`, `PluginPreview`/`PluginApply`/`executePlugin`, stdlib SemVer-core comparator | ✓ VERIFIED | Exists, substantive (448 new lines), exercised by 60+ passing subtests; wired into `cmd/engram/setup.go` via `setup.PluginPreview`/`setup.PluginApply`. |
| `internal/setup/claudecode.go` | claude-code's `PluginRuntime` impl | ✓ VERIFIED | Exact D-04/D-05/D-06/Pitfall-1 argv confirmed by `rg` and live preview output. |
| `internal/setup/codex.go` | codex's `PluginRuntime` impl | ✓ VERIFIED | `installed[]` matched by name+marketplaceName, HTTPS Git URL marketplace add, remove-then-add for outdated (D-02); confirmed by test and live preview. |
| `internal/skills/environment.go` + `presence.go` | `Lstat` seam + read-only `DetectPresence` | ✓ VERIFIED | `Install` untouched (`rg -n Lstat internal/skills/install.go` empty); `DetectPresence` touches only `Lstat`/`ReadFile`; `TestDetectPresence` (12 subtests incl. real-filesystem) PASS. |
| `cmd/engram/setup.go` plugin facet + routing | 7 flat-scalar fields, `setupApplyPluginFacet`, `setupReportNativeSkills`, D-07/D-08/D-09 routing | ✓ VERIFIED | Read directly; routing confirmed at the single `skills.Install` call site; live preview confirms end-to-end behavior. |
| `skill/engram/.codex-plugin/plugin.json` | minimal 4-key manifest | ✓ VERIFIED | Present, git-tracked, byte-equal identity fields, drift-gated. |
| `internal/setupgen/setupgen.go` + `manifest_test.go` | third rendered table + identity drift gate | ✓ VERIFIED | `TestRenderPluginTable`, `TestPluginManifestIdentityMatches` both PASS; renderer validates its own shape. |
| `skill/engram/commands/engram-setup.md` | regenerated plugin-delivery table + prose | ✓ VERIFIED | Table present with exact argv; `surfacesgen --check-setup` exits 0. |

### Key Link Verification

| From | To | Via | Status |
|------|----|----|--------|
| `internal/setup/plugin.go` | `internal/setup/apply.go` | `runSeam`/`execTimeout` reuse, no second bounded-exec helper | ✓ WIRED — `rg -c 'runSeam(ctx, env, binary, '` = 3 in plugin.go |
| `internal/setup/plugin.go` | `internal/setup/runtime.go` | `rt.(PluginRuntime)` type assertion at lane entry | ✓ WIRED |
| `cmd/engram/setup.go` | `internal/setup/plugin.go` | `setup.PluginApply(ctx, env, rt, r.Binary, setupVersion())` | ✓ WIRED — confirmed in source and live preview |
| `cmd/engram/setup.go` | `internal/skills/presence.go` | `skills.DetectPresence(skillsEnv, target, inv)` in `setupReportNativeSkills` | ✓ WIRED |
| `internal/setupgen/setupgen.go` | `internal/setup/claudecode.go` | `setup.ClaudeCode.(setup.PluginRuntime)` sources real argv, never a re-typed literal | ✓ WIRED — confirmed by `TestRenderPluginTable`/`TestRenderRealPlans` |
| `release-please-config.json` | `skill/engram/.codex-plugin/plugin.json` | `extra-files` `$.version` sync entry | ✓ WIRED |

### Behavioral Spot-Checks / Live Preview

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Plugin facet renders live against real `claude`/`codex` binaries, preview-only | `HOME=$(mktemp -d) /tmp/engram-verify setup --url ... --auth oauth --output json` (no `--apply`) | claude-code/codex: `plugin=would-write`, `plugin_state=absent`, exact argv on `plugin_command`, `skills=plugin-delivered`, `skills_native=none`; opencode: `plugin` fields empty, `skills=would-write` | ✓ PASS |
| Full repo gate | `task` | lint (golangci-lint/yamlfmt/actionlint/rumdl/ruff) + `go test ./...` all green, incl. `internal/store` (61s, red-evidence) | ✓ PASS |
| `go.mod`/`go.sum` unchanged since v0.16.1 | `git diff --exit-code v0.16.1 HEAD -- go.mod go.sum` | no output, exit 0 | ✓ PASS |
| `internal/keylinks` | `go test ./internal/keylinks/ -count=1` | PASS | ✓ PASS |
| Shipped-bundle privacy guard | `uv run --with pytest pytest skill/engram/hooks/tests -q` | 33 passed (guard's `rglob` covers the new manifest) | ✓ PASS |
| `setupgen`/`surfacesgen` drift | `go run ./internal/surfacesgen --check-setup` | exit 0 | ✓ PASS |

### Probe Execution (red-evidence)

| Probe | Command | Result | Status |
|-------|---------|--------|--------|
| `TestRedEvidencePatchesAreLive` (5 phase-03 patches: `03-01-codex-remove-then-add`, `03-01-plugin-state-machine`, `03-01-plugin-version-compare`, `03-02-detect-presence`, `03-03-plugin-skips-native`) | `go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1` (no `-short`) | all 14 patches across 3 phases confirmed live-RED against their mapped target tests | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| REQ-plugin-capability-detection | 03-01 | one-probe capability, distinct from binary-on-PATH | ✓ SATISFIED | `TestPluginCapabilityProbeFailureFallsBackToNative` |
| REQ-plugin-install-or-update | 03-01 | marketplace add/install/update/current under `--apply`, exact argv preview | ✓ SATISFIED | `TestPluginPlan` |
| REQ-plugin-three-way-state | 03-01 | absent/outdated/current from SemVer comparison | ✓ SATISFIED | `TestPluginVersionCompare` |
| REQ-plugin-skips-skills-copy | 03-02, 03-03 | mutual exclusion, no delete | ✓ SATISFIED | `TestSetupPluginDeliveredRuntimeAuthorsZeroNativeWrites`, single-call-site source read |
| REQ-plugin-facet-reported | 03-03 | own facet in text+JSON, exit taxonomy | ✓ SATISFIED | `TestSetupApplyJSONEmitsPluginFacet` |
| REQ-codex-plugin-manifest | 03-04 | manifest exists, synced, drift-gated | ✓ SATISFIED | `TestPluginManifestIdentityMatches` |
| REQ-plugin-setupgen-regenerated | 03-04 | generated prose reflects new actions, drift gate green | ✓ SATISFIED | `TestRenderPluginTable`, `surfacesgen --check-setup` |

No orphaned requirements: `REQ-plugin-opencode` is explicitly scoped out of this phase in `REQUIREMENTS.md` (deferred, opencode has no plugin CLI today) and is not among this phase's declared requirement IDs.

### Anti-Patterns Found

Scanned every file this phase created/modified for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER`/
"not yet implemented"/empty-return stubs. Two incidental matches for the substring "placeholder"
(`internal/setup/claudecode.go:108`, `cmd/engram/setup_test.go:423`) are both the phrase
"path-provenance placeholder" documenting an unrelated, pre-existing pattern (not a stub marker,
not touched by this phase's own new code) — no debt marker found. No blockers.

### Human Verification Required

None. Every must-have truth resolved to a behavioral test result or a directly-observed live
preview run; no truth was left on presence-only evidence.

### Known, Explicitly-Deferred Item (non-blocking)

- **IN-01** (`03-REVIEW.md`/`03-REVIEW-FIX.md`): `setupNativePresenceSummary` can render `"none"`
  for a bare symlinked native skills directory with zero matching skill entries, dropping the
  symlink signal in that narrow edge case. The reviewer and orchestrator explicitly classified
  this as not a defect (D-08's own wording conditions the report on skills being present) and
  deferred it rather than fixing it in this phase. Recorded here for visibility; does not affect
  any of the phase's success criteria or must-have truths.

### Gaps Summary

None. All 7 roadmap success-criteria-derived truths verified with passing tests run directly in
this verification pass (not merely cited from SUMMARY.md), all artifacts exist/are substantive/are
wired, all key links confirmed by source inspection, the full `task` gate and the red-evidence
harness (all 5 phase-03 patches, 14 total across 3 phases) are green, `go.mod`/`go.sum` are
byte-unchanged since v0.16.1, and a live preview run against real `claude`/`codex` binaries on this
machine reproduced the exact documented plugin-facet behavior with zero write verbs issued.

---

_Verified: 2026-09-15T04:24:00Z_
_Verifier: Claude (gsd-verifier)_

## Re-fingerprint 2026-09-15 (orchestrator, after Phase 4)
Phase 4 (Drift Detection, Read-Only) additively edited six files in this phase's `covered_files`
(`internal/setup/{claudecode,codex}.go`, `cmd/engram/{setup.go,operator_view_setup_test.go,testdata/help.golden}`,
`internal/store/redevidence_harness_test.go` — a new Phase 4 entry in `redEvidenceDirs`; this
phase's own entry is untouched), which correctly flipped the covered digest and this report to
`stale`. This phase's CONCLUSION is unchanged and was re-proven at Phase 4's HEAD before
re-fingerprinting: `TestPluginVersionCompare`, `TestPluginPlan`,
`TestPluginCapabilityProbeFailureFallsBackToNative`, `TestPluginRuntimeIsOptional` (`internal/setup`);
`TestDetectPresence` (`internal/skills`); `TestSetupPluginDeliveredRuntimeAuthorsZeroNativeWrites`,
`TestSetupApplyJSONEmitsPluginFacet`, `TestSetupPreviewShowsPluginArgv`,
`TestSetupPluginUnavailableFallsBackToNative` (`cmd/engram`) all pass with `-count=1`, and all five
`03-*.patch` red-evidence entries stayed live under `TestRedEvidencePatchesAreLive` (23/23 at
`d5a5a694`). The digest is re-pinned to the current bytes so the staleness signal stays meaningful
for the NEXT unrelated change.
