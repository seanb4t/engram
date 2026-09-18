---
phase: 05-apply-time-preserve-gate-documentation
verified: 2026-09-16T23:25:16Z
status: passed
score: 4/4 must-haves verified
covered_files:
  - ".planning/phases/05-apply-time-preserve-gate-documentation/05-01-PLAN.md"
  - ".planning/phases/05-apply-time-preserve-gate-documentation/05-01-SUMMARY.md"
  - ".planning/phases/05-apply-time-preserve-gate-documentation/05-02-PLAN.md"
  - ".planning/phases/05-apply-time-preserve-gate-documentation/05-02-SUMMARY.md"
  - ".planning/phases/05-apply-time-preserve-gate-documentation/05-03-PLAN.md"
  - ".planning/phases/05-apply-time-preserve-gate-documentation/05-03-SUMMARY.md"
  - ".planning/phases/05-apply-time-preserve-gate-documentation/05-04-PLAN.md"
  - ".planning/phases/05-apply-time-preserve-gate-documentation/05-04-SUMMARY.md"
  - ".planning/phases/05-apply-time-preserve-gate-documentation/05-CONTEXT.md"
  - ".planning/phases/05-apply-time-preserve-gate-documentation/05-POST-RELEASE.md"
  - ".planning/phases/05-apply-time-preserve-gate-documentation/05-REVIEW-FIX.md"
  - ".planning/phases/05-apply-time-preserve-gate-documentation/05-REVIEW.md"
  - ".planning/phases/05-apply-time-preserve-gate-documentation/05-VALIDATION.md"
  - "cmd/engram/agent_setup_docs_test.go"
  - "cmd/engram/install_docs_test.go"
  - "cmd/engram/plugin_docs_test.go"
  - "cmd/engram/setup.go"
  - "cmd/engram/setup_test.go"
  - "cmd/engram/testdata/help.golden"
  - "docs-site/src/content/docs/guides/agent-setup.md"
  - "docs-site/src/content/docs/guides/install.md"
  - "docs-site/src/content/docs/guides/plugin.md"
  - "internal/setup/apply.go"
  - "internal/setup/apply_test.go"
  - "internal/setup/claudecode.go"
  - "internal/setup/claudecode_test.go"
  - "internal/setup/codex.go"
  - "internal/setup/drift.go"
  - "internal/setup/drift_test.go"
  - "internal/setup/plan.go"
  - "internal/store/redevidence_harness_test.go"
covered_digest: "v1:sha256:74127a6dfc3e28f15ab900e2be004300214c71abfce496b55c41b05c27581547"
behavior_unverified: 0
overrides_applied: 0
post_release_status: complete
post_release_tracker: https://github.com/seanb4t/engram/issues/567
---

# Phase 5: Apply-Time Preserve Gate & Documentation Verification Report

**Phase Goal:** `--apply` consults the same drift classification before writing and performs zero
write actions against a `preserved` registration — including never running Claude Code's
destructive `mcp remove` step — closing the root cause of the 2026-09-10 overwrite incident
(gotcha `ryr82bf2s2`); a reproducible rewrite on Claude Code states upfront that an
OAuth-authenticated registration will need to log in again. `guides/install.md`,
`guides/agent-setup.md`, and `guides/plugin.md` are brought current with everything this
milestone shipped — plugin-first delivery, the header shape, the `preserved` outcome and apply
gate, and man pages — closed out with a post-release live-observation note (the `2026-08-23.01`
D-10 pattern) rather than checked off from code alone.

**Verified:** 2026-09-16T23:25:16Z
**Status:** passed
**Re-verification:** No — initial verification

## Governing decision (D-06)

Per `05-CONTEXT.md` D-06 and `05-POST-RELEASE.md`, this phase's verification **passes** with the
documentation requirement's live-observation half deliberately open. `REQ-docs-setup-v2` is
correctly `[ ]` in `REQUIREMENTS.md` (confirmed by direct read, not by trusting the SUMMARY) — this
is the intended D-06 state, not a gap. `REQ-apply-preserve-gate` and `REQ-apply-rewrite-consequence`
are `[x]` and are the code-complete requirements this phase actually ships. SC4's code-gated half
(three guides brought current, gated by tests with positive controls) is verified below; the
live-observation half is tracked by the OPEN GitHub issue
[#567](https://github.com/seanb4t/engram/issues/567) and `05-POST-RELEASE.md`.

## Goal Achievement

### Observable Truths (Success Criteria)

| # | Truth (ROADMAP SC) | Status | Evidence |
|---|---|---|---|
| 1 | `--apply` against a pre-seeded, unreproducible registration performs zero registration-write actions for that runtime while still applying skills/plugin actions, proven by a fixture test that runs `--apply` itself | ✓ VERIFIED | `internal/setup/apply.go:490-495` — `classifyProbe`/`renderClassification` short-circuit `return res` on `OutcomeAlreadyCorrect`/`OutcomePreserved` strictly BEFORE the `for _, action := range plan.Actions` loop (verified by direct read + `awk` ordering check in 05-01's own acceptance criteria, re-confirmed by reading the compiled function). Process-boundary proof: `cmd/engram/setup_test.go#TestSetupApplyPreservedRuntimeSkipsRegistrationWrite` (`plugin-current` and `plugin-absent` subtests) — both PASS when re-run (`go test ./cmd/engram/... -run '^TestSetupApplyPreservedRuntimeSkipsRegistrationWrite$' -v`); `plugin-absent` explicitly asserts the plugin lane's `marketplace add`/`plugin install` argv WERE recorded while zero `mcp remove`/`mcp add` calls exist. Package-level: `internal/setup/apply_test.go#TestApplyPreservedIssuesZeroWrites` (claude-code + codex) — PASS on re-run. |
| 2 | Claude Code's `mcp remove` step never runs against a `preserved` registration | ✓ VERIFIED | `internal/setup/apply_test.go#TestApplyPreservedNeverRunsClaudeCodeRemove` — PASS on re-run; asserts no recorded argv begins `mcp remove`/`mcp add` with exactly one scripted `Run` result (panic-on-overrun harness). `cmd/engram/setup_test.go`'s `assertNoRegistrationWrite` helper independently re-proves this at the process boundary (exactly one `mcp get`, zero of any other `mcp` subcommand). Ten red-evidence patches under `red-evidence/` (registered in `internal/store/redevidence_harness_test.go`'s `redEvidenceDirs`) were re-run via `TestRedEvidencePatchesAreLive` — all 10 Phase-5 patches (plus all 23 patches from Phases 1-4) confirmed to break their mapped test when applied and revert byte-identical (107s live run, PASS). |
| 3 | When a reproducible difference on Claude Code requires remove-then-add of an existing OAuth-authenticated registration, both preview and apply state that the rewrite will require logging in again before it runs | ✓ VERIFIED | `internal/setup/claudecode.go:354-368` — `claudeCodeOAuthReLoginNote` fires by observed SHAPE alone (`auth == AuthNone`, never state-parsed), set on `Observation.RewriteConsequence`; `internal/setup/apply.go:277-279` surfaces it on `Result.Notes` in preview via `renderClassification`, and the mutate lane seeds the write-loop's `notes` accumulator from the same field (`apply.go:502-505`) so it appears FIRST, ahead of the tolerant-remove record. `internal/setup/claudecode_test.go#TestOAuthReLoginConsequence` (11 subtests, re-run, all PASS) proves the positive case in both lanes and the negative cases (bearer, foreign, preserved, already-correct, ambiguous, codex — codex's `Observe` never sets this field, confirmed by `rg` returning zero hits for `RewriteConsequence` in `codex.go`). |
| 4 | `guides/install.md`, `guides/agent-setup.md`, and `guides/plugin.md` describe the shipped plugin-first delivery, header shape, `preserved` outcome and apply gate, and man pages — checked off only after a post-release live observation, not from code alone | ✓ VERIFIED (code-gated half); live-observation half correctly deferred per D-06 | Direct read of all three guides confirms: `agent-setup.md` states the apply gate, the corrected pre-write `already-correct` guarantee (the stale "does not guarantee that no write" sentence is absent — confirmed by `rg` returning no match), the `preserved` row's dual-runtime remediation, the OAuth re-login note, and plugin-first delivery, under `:::note[Unreleased as of v0.16.1]`; `install.md` states cask contents (completions + `man engram-setup`/`man engram`) and a plugin-first pointer; `plugin.md` states plugin-first install via `engram setup --apply` and a `preserved` remediation cross-link. All three gated by dedicated docs-drift tests (`TestAgentSetupGuideDocumentsDrift`, `TestInstallGuideDocumentsSetupV2`, `TestPluginGuideDocumentsPluginFirst`) plus positive-control gate-firing tests — all re-run, all PASS. `REQUIREMENTS.md` confirmed by direct read to still carry `- [ ] **REQ-docs-setup-v2**`; `05-POST-RELEASE.md` exists with `status: pending` and a live OPEN tracking issue (`gh issue view` not re-run here, but the URL and file shape were directly read and match the `06-POST-RELEASE.md` precedent's frontmatter key set exactly: `phase`/`status`/`tracker`, four headings). |

**Score:** 4/4 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/setup/apply.go` | `classification`/`classifyProbe`/`renderClassification`; mutate-branch pre-action short-circuit; D-02 post-write re-observe | ✓ VERIFIED | Read directly; short-circuit precedes the action loop (`apply.go:490-496` before `apply.go:506`); D-02 tail rebuilds `Registered` via `c.dr.Observe`→`renderObservation`, never raw probe2 bytes (`apply.go:551-573`) |
| `internal/setup/drift.go` | `Observation.ManualRemediation`/`RewriteConsequence` fields | ✓ VERIFIED | `rg` confirms both fields present with AUTHORED-HERE doc comments |
| `internal/setup/claudecode.go` | `claudeCodeManualRemediation`, `claudeCodeOAuthReLoginNote`, `Observe` sets both | ✓ VERIFIED | Read directly; shape-only trigger (`auth == AuthNone`) confirmed |
| `internal/setup/codex.go` | `codexManualRemediation`; never sets `RewriteConsequence` | ✓ VERIFIED | Read directly; `rg -n RewriteConsequence internal/setup/codex.go` returns zero matches |
| `cmd/engram/setup.go` | `setupLongDescription` apply-gate sentences | ✓ VERIFIED | `rg` confirms `before writing`, `log in again`, `no registration command`, `runtime's own tool` all present |
| `cmd/engram/testdata/help.golden` | regenerated section carrying the new sentences | ✓ VERIFIED | `TestHelpGolden`/`TestCatalogGolden` PASS on re-run; `catalog.golden` untouched |
| `docs-site/.../agent-setup.md`, `install.md`, `plugin.md` | current guide content per D-07 | ✓ VERIFIED | Direct read (see truth 4 evidence) |
| `cmd/engram/agent_setup_docs_test.go`, `install_docs_test.go`, `plugin_docs_test.go` | docs-drift gates with positive controls | ✓ VERIFIED | All re-run, all subtests PASS (17, 5, 5 positive-control cases respectively) |
| `.planning/phases/05.../05-POST-RELEASE.md` | D-06 handoff shape | ✓ VERIFIED | Read directly; matches `06-POST-RELEASE.md` precedent's exact frontmatter keys (`phase`/`status`/`tracker`) and four headings |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `internal/setup/apply.go` | `internal/setup/drift.go` | shared `classifyProbe` type-asserts `DriftRuntime` exactly once | ✓ WIRED | `rg -o -F 'rt.(DriftRuntime)' internal/setup/apply.go` → exactly 1 occurrence (inside `classifyProbe`), confirmed |
| `internal/setup/apply.go` | `internal/setup/claudecode.go`/`codex.go` | `.ManualRemediation`/`.RewriteConsequence` field reads | ✓ WIRED | `renderClassification` reads `c.obs.ManualRemediation`/`c.obs.RewriteConsequence`; both runtimes' `Observe` populate them |
| `cmd/engram/setup_test.go` | `internal/setup/apply.go` | SC1/SC2 proven through the real CLI entry point (`runClient(t, "setup", ..., "--apply")`) | ✓ WIRED | Confirmed by reading `TestSetupApplyPreservedRuntimeSkipsRegistrationWrite` — a real `cobra` command invocation, not a package-level call |
| `internal/store/redevidence_harness_test.go` | `red-evidence/*.patch` | ten Phase-5 patches registered and live | ✓ WIRED | `TestRedEvidencePatchesAreLive` re-run live (107s): all 10 Phase-5 patches + 23 prior-phase patches PASS |

### Behavioral Spot-Checks / Test Re-Runs

| Behavior | Command | Result | Status |
|---|---|---|---|
| Preserved gate — zero writes, plugin lane still runs (process boundary) | `go test ./cmd/engram/... -run '^TestSetupApplyPreservedRuntimeSkipsRegistrationWrite$' -v` | 2/2 subtests PASS | ✓ PASS |
| Preserved/already-correct one-call no-op, would-write redaction-safe re-observe | `go test ./internal/setup/... -run '^(TestApplyPreservedIssuesZeroWrites\|TestApplyPreservedNeverRunsClaudeCodeRemove\|TestApplyAlreadyCorrectIssuesZeroWrites\|TestApplyWroteRegisteredIsRedacted\|TestApplyConvergesCodex\|TestApplyConvergesClaudeCode\|TestPreviewNotComparedDriftStaysBounded)$' -v` | all PASS | ✓ PASS |
| OAuth re-login note — shape-only, both lanes, 11 subtests | `go test ./internal/setup/... -run '^TestOAuthReLoginConsequence$' -v` | 11/11 PASS | ✓ PASS |
| `--help`/golden/literal-leak/docs-drift gates | `go test ./cmd/engram/... -run '^(TestSetupHelpStatesApplyGate\|TestHelpGolden\|TestCatalogGolden\|TestSetupJSONNeverLeaksProbeLiteral\|TestAgentSetupGuideDocumentsDrift\|TestAgentSetupGuideDriftGateFiresOnInjectedViolation\|TestInstallGuideDocumentsSetupV2\|TestInstallGuideGateFiresOnInjectedViolation\|TestPluginGuideDocumentsPluginFirst\|TestPluginGuideGateFiresOnInjectedViolation)$' -v` | all PASS (44 subtests total) | ✓ PASS |
| Red-evidence patches (10 Phase-5 + 23 prior) still live | `go test ./internal/store/... -run '^TestRedEvidencePatchesAreLive$' -v` | 33/33 PASS (107s) | ✓ PASS |
| Full build + targeted regression | `go build ./...` and `go test ./cmd/engram/... ./internal/server/... ./internal/setup/... ./internal/setupgen/... ./internal/skills/... ./internal/surfacesgen/... -count=1` | all `ok` | ✓ PASS |
| Lint (touched packages) | `golangci-lint run ./internal/setup/... ./cmd/engram/...` | 0 issues | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|---|---|---|---|---|
| REQ-apply-preserve-gate | 05-01, 05-02 | `--apply` consults classification before writing; zero writes on `preserved` | ✓ SATISFIED (Complete in REQUIREMENTS.md) | `internal/setup/apply.go`, tests above |
| REQ-apply-rewrite-consequence | 05-01, 05-02 | OAuth re-login stated upfront on a Claude Code rewrite | ✓ SATISFIED (Complete in REQUIREMENTS.md) | `internal/setup/claudecode.go`, `TestOAuthReLoginConsequence` |
| REQ-docs-setup-v2 | 05-02, 05-03, 05-04 | Three guides current; post-release observation before checkoff | ✓ SATISFIED — code-gated half (D-06); intentionally `[ ]` pending live observation | `05-POST-RELEASE.md`, issue #567, all three docs-drift gates |

No orphaned requirements: `REQUIREMENTS.md`'s Phase 5 row set (`REQ-apply-preserve-gate`, `REQ-apply-rewrite-consequence`, `REQ-docs-setup-v2`) exactly matches the union of `requirements:` declared across all four PLAN frontmatters.

### Anti-Patterns Found

None. `rg` scan of every file this phase modified for `TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER` returned zero matches. `git status --porcelain` shows only two untracked GSD state files (`.planning/milestone.lock`, `.planning/state.json`) unrelated to this phase's scope.

### Code Review Findings (05-REVIEW.md / 05-REVIEW-FIX.md)

Re-review (iteration 2) status: **clean** (0 critical, 0 warning, 1 info). The one prior Warning
(WR-01 — Preview's "not compared" `Drift` note bypassed `boundCapture`) was fixed in commit
`fd045e52` and independently re-verified above via `TestPreviewNotComparedDriftStaysBounded`
(re-run, PASS). The remaining IN-01 (Info: `strings.Replace` fixture derivations lack a
landed-substitution assertion) is a test-fixture robustness note, not a functional defect, and does
not block phase completion.

### Human Verification Required

None. Every ROADMAP success criterion is either code-verified directly (SC1-SC3, SC4's code-gated
half) or is explicitly and correctly deferred to a tracked post-release human observation by this
phase's own design (D-06) — that deferral is documented in `05-POST-RELEASE.md` and GitHub issue
#567, not an unresolved verification gap.

### Gaps Summary

None. All must-haves verified against the live codebase (not SUMMARY claims): the pre-write
classification gate is real and ordered correctly before the write-action loop, the OAuth
re-login consequence is shape-triggered and correctly scoped to Claude Code only, all three guides
are current and gated, ten Phase-5 red-evidence patches are registered and proven live, and the
one deliberately-open item (`REQ-docs-setup-v2`'s live observation) is open by design per D-06,
not by omission.

---

_Verified: 2026-09-16T23:25:16Z_
_Verifier: Claude (gsd-verifier)_

## Re-fingerprint 2026-09-18 (orchestrator, after the v0.17.0 post-release observation)

`05-RELEASE-0.17.0.md` closed the D-06 handoff: the three guides' `Unreleased as of v0.16.1`
asides became `Available since v0.17.0`, `install_docs_test.go` / `plugin_docs_test.go` leg 4 now
requires `Available since v` (fixture + injected-violation case renamed), `05-POST-RELEASE.md`
reads `status: complete`, and `REQ-docs-setup-v2` is checked off. Every conclusion above was
re-proven before re-fingerprinting: `go test ./cmd/engram -count=1` green (all three guide gates
incl. their injected-violation positive controls), `task lint` and `task license:check` clean,
`pnpm --dir docs-site build` complete. `post_release_status` flipped to `complete` (value only).
