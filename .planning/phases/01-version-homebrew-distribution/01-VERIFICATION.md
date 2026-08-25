---
phase: 01-version-homebrew-distribution
verified: 2026-08-24T20:10:00Z
status: passed
score: 5/5 must-haves verified (ROADMAP success criteria); 0 gaps; 1 non-blocking anti-pattern; 3 tracked deferrals
behavior_unverified: 0
overrides_applied: 0
deferred:
  - truth: "SC4 — cross-repo credential proven by an explicit check before any real release depends on it"
    addressed_in: "GitHub issue #514, checklist A (post-merge, blocking on next release-please merge)"
    evidence: "workflow_dispatch is only exposed for workflow definitions present on the default branch; the probe (.github/workflows/verify-tap-credential.yaml) cannot be dispatched from this feature branch. Issue #514's first line: 'DO NOT MERGE A RELEASE-PLEASE PR UNTIL CHECKLIST A BELOW IS RECORDED ON THIS ISSUE.' Confirmed OPEN via `gh issue view 514`."
  - truth: "REQ-homebrew-cask-published — a tagged release actually publishes Casks/engram.rb to the tap"
    addressed_in: "Phase 6 (ROADMAP moved this requirement out of Phase 1 by developer decision, commit b87071f6)"
    evidence: "REQUIREMENTS.md traceability table: 'REQ-homebrew-cask-published | Phase 6 | Pending'. Issue #514 checklist B carries the runnable observation as a Phase 6 handoff."
  - truth: "End-to-end `go install .../cmd/engram@vX.Y.Z` resolves to the bare tag version"
    addressed_in: "Issue #514, checklist C"
    evidence: "Every git tag in this repo predates cmd/engram/buildversion.go, so no go install of a real tag can reach the new resolver in-phase; TestVersionFromModuleVersion proves the parser half only, per 01-02-PLAN.md's own explicit narrowing (M-D, cycle-2 review)."
---

# Phase 1: Version & Homebrew Distribution Verification Report

**Phase Goal:** A user can install engram with `brew install` on macOS or Linux (amd64 and arm64), and
the pipeline that publishes it is proven to work end to end rather than merely configured.
`engram version --output json` lands in this same phase because it is the cask's install-time
correctness gate.

**Verified:** 2026-08-24
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `engram version --output json` prints a machine-readable payload; human-readable `engram version` unchanged | ✓ VERIFIED | `go test ./cmd/engram -run 'TestVersion...'` — `TestVersionJSONLane`, `TestVersionTextLane`, `TestVersionExplicitTextLane`, `TestVersionTextEqualsJSON`, `TestVersionOutputFlagDefault` all `--- PASS`. Source read: `cmd/engram/version.go` `RunE` branches on `json` only, text lane is `fmt.Fprintln`, json lane is a one-field `versionDoc{Version}`. |
| 2 | Repo configured to publish a cask to `seanb4t/homebrew-tap` via `homebrew_casks:`, amd64+arm64 on macOS+Linux, validated locally without publishing | ✓ VERIFIED | `.goreleaser.yaml`: `homebrew_casks:` block present (count=1), no `brews:` (count=0); `builds.goos: [linux, darwin]`, `builds.goarch: [amd64, arm64]`. `task release:check` (`goreleaser check`) exits 0. |
| 3 | A binary failing the version-json assertion makes `brew install` fail loudly; quarantine strip runs before the gate | ✓ VERIFIED (by construction, D-11/D-15) | Ordering gate independently re-run against the live file: quarantine `xattr` call at line 137, version-json assertion at line 150 (137 < 150); macOS guard (`if OS.mac?`) at line 136, strictly above the xattr call (136 < 137). `raise` on mismatch present in source. Per D-11/D-15 (recorded, reviewed decisions in `01-CONTEXT.md`), actual Homebrew/Gatekeeper execution is explicitly out of this phase's verification scope — the verifier is instructed not to fail this on rehearsal wording, and no rehearsal was performed. This is a first-party, documented scope boundary, not a hidden gap. |
| 4 | Cross-repo credential to `seanb4t/homebrew-tap` proven by an explicit check before any real release depends on it | ✓ VERIFIED / DEFERRED (see `deferred:` frontmatter) | The check itself is fully built and correctly scoped: both `create-github-app-token` mints request `repositories: engram,homebrew-tap` (confirmed via independent `rg` re-run, count=1 each, non-comment). `.github/workflows/verify-tap-credential.yaml` is a standalone, `workflow_dispatch`-only, read-only probe (`permissions: contents: read`, token via `env: GH_TOKEN`, never CLI arg; asserts `.permissions.push == "true"`, exits 1 otherwise). Its **first dispatch** cannot happen from a feature branch (GitHub only exposes `workflow_dispatch` for workflows on the default branch) — this is a genuine GitHub platform constraint, not an implementation gap. GitHub issue #514 (confirmed OPEN) makes that dispatch a hard, first-line block on the next release-please PR merge, with the tap's baseline HEAD SHA (`969aef42...`) recorded for the no-write comparison. REQUIREMENTS.md correctly leaves `REQ-cask-credential-verified` `[ ]` Pending until that dispatch is recorded — this is deliberate and traceable, not an oversight. |
| 5 | A rehearsed failure between tag creation and cask publication recovers via the existing `workflow_dispatch` re-ship path, no hand-edit to the tap | ✓ VERIFIED (by construction, D-11/D-15) | `SKIP_HOMEBREW_UPLOAD` boundary logic independently re-confirmed: `=false` and `=true` branches each appear exactly once (non-comment) in `release.yaml`; guard step sits between `actions/checkout` (line 87) and `goreleaser-action` (line 171) at line 141. `.goreleaser.yaml`'s `skip_upload` reads `index .Env "SKIP_HOMEBREW_UPLOAD"` with a `false` fallback for local snapshots. No actual backfill was rehearsed against the real tap, consistent with D-15's explicit instruction. |

**Score:** 5/5 ROADMAP success criteria verified (3 and 5 by an explicit, reviewed, documented construction-not-rehearsal decision; 4 verified as correctly built, with its first live proof deliberately and traceably deferred post-merge).

### Deferred Items

See `deferred:` in frontmatter — all three are tracked (GitHub issue #514, and ROADMAP's Phase 6) and none represents undocumented scope loss.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/engram/version.go` | `--output json\|text`, `versionDoc`, `runVersion` | ✓ VERIFIED | Read in full; matches plan exactly, `resolvedVersion()` is the sole value source for both render lanes. |
| `cmd/engram/version_test.go` | 5 behavioral tests, `resetClientFlags(t)` discipline | ✓ VERIFIED | 136 lines; all 5 named tests pass individually and under `-shuffle=on` (via `task test`). |
| `cmd/engram/buildversion.go` | `lastRelease`, `nextPatch`, `deriveDevVersion`, `versionFromModuleVersion`, `resolvedVersion` | ✓ VERIFIED | 166 lines; all functions present, `x-release-please-version` annotation on the same line as the const. |
| `cmd/engram/buildversion_test.go` | Table tests + manifest-drift gate | ✓ VERIFIED | 185 lines; `TestNextPatch`, `TestDeriveDevVersion`, `TestVersionFromModuleVersion`, `TestLastReleaseMatchesManifest` all pass. |
| `.goreleaser.yaml` | `homebrew_casks:` block, ordered post-install hook, `skip_upload` | ✓ VERIFIED | Full file read; `brews:` absent, three-step hook order confirmed, uninstall removes exactly the 3 files the install hook writes. |
| `.github/workflows/release.yaml` | Explicit token scope, upload guard step | ✓ VERIFIED | Full relevant section read; ordering and both boundary branches independently re-confirmed. |
| `.github/workflows/verify-tap-credential.yaml` | Standalone, read-only, `workflow_dispatch`-only probe | ✓ VERIFIED | Full file read; matches plan exactly. |
| `release-please-config.json` | 4th `extra-files` entry (generic, `cmd/engram/buildversion.go`) | ✓ VERIFIED | `extra-files` array has exactly 4 entries; 4th is `{"type":"generic","path":"cmd/engram/buildversion.go"}`, no `jsonpath`. Matches `.release-please-manifest.json`'s `"0.14.0"` via `TestLastReleaseMatchesManifest`. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `.goreleaser.yaml` post-install hook | `cmd/engram/version.go` | shells `engram version --output json`, compares to declared version | ✓ WIRED | Confirmed both textually (line 150) and by the version-json contract's own test coverage. |
| `cmd/engram/version.go` | `internal/config/client_validate.go` | `config.ValidateOutputFormat` | ✓ WIRED | `runVersion` calls it and returns `usageErrorf` on error, landing on `exitUsage` (2), confirmed by `TestExitCodeBaseline/version/output-bogus`. |
| `release-please-config.json` | `cmd/engram/buildversion.go` | generic `extra-files` updater rewrites `lastRelease` | ✓ WIRED | Entry present; drift test (`TestLastReleaseMatchesManifest`) passes against the current manifest. |
| `cmd/engram/version.go` / `cmd/engram/serve.go` | `cmd/engram/buildversion.go` | `resolvedVersion()` | ✓ WIRED | `version.go:76`, `serve.go:83,231,296` all call `resolvedVersion()`; `catalog.go`/`root.go` deliberately do not (D-02, confirmed by direct read). |
| `.github/workflows/release.yaml` | `.goreleaser.yaml` | `SKIP_HOMEBREW_UPLOAD` via `$GITHUB_ENV` → `index .Env` | ✓ WIRED | Both sides independently confirmed. |
| `.github/workflows/verify-tap-credential.yaml` | `seanb4t/homebrew-tap` | `gh api repos/seanb4t/homebrew-tap --jq .permissions.push`, read-only | ✓ WIRED (code); not yet dispatched | See deferred item — dispatch requires the workflow to exist on `main`. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Named phase-01 Go tests all pass individually | `go test ./cmd/engram -run 'TestVersion...\|TestNextPatch\|TestDeriveDevVersion\|TestVersionFromModuleVersion\|TestLastReleaseMatchesManifest\|TestExitCodeBaseline' -count=1 -v` | All `--- PASS`, `ok` | ✓ PASS |
| Full workspace gate (`task` — lint + test), run once | `task` | golangci-lint 0 issues; `go test ./...` green except pre-existing `TestRedEvidencePatchesAreLive` (tracked #513, base-commit failure, unrelated to this phase's files) | ✓ PASS (with known, pre-existing, tracked exception) |
| GoReleaser config schema validation | `task release:check` | exits 0 (only pre-existing, unrelated `dockers`/`docker_manifests` deprecation warning) | ✓ PASS |
| Debt-marker scan on all files this phase modified | `rg -n -i 'TBD\|FIXME\|XXX\|TODO\|HACK\|PLACEHOLDER\|not yet implemented\|coming soon' <10 modified files>` | no matches | ✓ PASS |
| `.goreleaser.yaml` post-install hook ordering (quarantine → version gate → completions) | line-number `rg` gates, independently re-derived (not copied from SUMMARY) | quarantine=137 < version-gate=150 < completions=173; OS-guard=136 < quarantine=137 | ✓ PASS |
| `release.yaml` guard-step ordering | line-number `rg` gates | checkout=87 < guard=141 < goreleaser-action=171 | ✓ PASS |
| `SKIP_HOMEBREW_UPLOAD` both boundary branches present exactly once | `rg` occurrence count, comment-stripped | `=false` count=1, `=true` count=1 | ✓ PASS |
| Diff scope matches declared `files_modified` across all 3 plans | `git diff --stat main...HEAD` | Exactly the 14 files the three plans declare (`cmd/engram/{version,buildversion,serve,root}.go` + 2 test files + 2 goldens + `cmdwalk_test.go`/`exitcode_baseline_test.go`, `.goreleaser.yaml`, both workflow files, `release-please-config.json`) | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|-----------------|-------------|--------|----------|
| REQ-version-json | 01-01, 01-02 | Machine-readable `engram version`, existing text output unchanged | ✓ SATISFIED | Marked `[x]` Complete in REQUIREMENTS.md; independently re-verified via all 5 version tests + all 4 buildversion tests. |
| REQ-cask-install-gate | 01-01 | `brew install` fails loudly on version mismatch; quarantine strip first; no rescuing helper delegation | ✓ SATISFIED (functionally) — see anti-pattern note below | Marked `[x]` Complete in REQUIREMENTS.md. Ordering gates re-confirmed. **Caveat:** the plan's own explicit, zero-tolerance acceptance criterion for the literal string `generate_completions_from_executable` appearing anywhere in `.goreleaser.yaml` — including comments — is violated once (see Anti-Patterns below). Functional behavior is unaffected. |
| REQ-cask-credential-verified | 01-03 | Cross-repo credential proven by explicit check before any real release depends on it | ○ CORRECTLY BUILT, DISPATCH DEFERRED | Marked `[ ]` Pending in REQUIREMENTS.md — deliberately, per 01-03-PLAN.md's own "Requirement scope" section and issue #514. The check exists, is correct, and is tracked as a hard pre-merge block. This deferral is sound and traceable. |
| REQ-cask-reship-recovery | 01-03 | Recoverable without hand-editing the tap, via the existing `workflow_dispatch` path, rehearsed once | ✓ SATISFIED (by construction, D-11/D-15) | Marked `[ ]` Pending in REQUIREMENTS.md, but 01-03-PLAN.md's objective explicitly states this and REQ-cask-credential-verified close by construction, and the checkbox lag is intentional pending #514's checklist A (which also closes credential-verified). No rehearsal against the real tap was performed, matching D-15's explicit instruction to the verifier. |
| REQ-homebrew-cask-published | (moved to Phase 6, commit `b87071f6`) | A tagged release publishes the cask | N/A — not this phase | REQUIREMENTS.md traceability: `Phase 6 | Pending`. No orphan: the move is documented in three places (ROADMAP, REQUIREMENTS.md, all three plans' "Artifacts this phase produces" tables) and issue #514 checklist B carries the runnable Phase 6 handoff. |

**Orphan check:** `grep -E "Phase 1" REQUIREMENTS.md` returns exactly the 4 requirement IDs declared across the three plans' `requirements:` frontmatter (`REQ-version-json`, `REQ-cask-install-gate`, `REQ-cask-credential-verified`, `REQ-cask-reship-recovery`). No orphaned requirement.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `.goreleaser.yaml` | 161 | Literal string `generate_completions_from_executable` appears inside a YAML comment explaining why it was rejected | ⚠️ WARNING | 01-01-PLAN.md Task 1's `<action>` states, twice, in bold: *"Keep that identifier... out of `.goreleaser.yaml` entirely — including out of YAML comments... there is no safe guard for a token this task never wants written at all, so naming either one in a comment turns [the gate] red."* The plan's own stated acceptance criterion — `test "$(rg -o -F 'generate_completions_from_executable' .goreleaser.yaml \| wc -l)" = 0` — **fails when actually run** (count=1, independently re-confirmed). Functional impact is nil: the actual `hooks.post.install` correctly writes completions via `system_command`, never via the rescuing Homebrew helper, and the mention is explanatory prose about the rejected alternative — the same kind of prose Task 2's own `<read_first>` pointed the executor toward. This is a real, provable deviation from an explicit, heavily-emphasized plan discipline (the same comment-anchor defect class that cycles 1–3 of review repeatedly fixed elsewhere in this phase, e.g. HIGH-5, M-A, M-C, M-G), landing once, unnoticed, in the one place the plan explicitly said must have zero tolerance. Not a functional defect and not blocking the phase goal, but worth a follow-up commit to strip the identifier from the comment (rephrase without naming it, matching the discipline the rest of this phase enforced everywhere else). |

**This looks intentional but under-scrutinized, not deliberate.** No override is suggested — the correct fix is trivial (reword the comment to describe the rescue-to-warning behavior without naming the specific Homebrew helper identifier) and should simply be done in a follow-up, not accepted as a permanent deviation.

### Human Verification Required

None. All items that cannot be fully closed within this phase (credential-probe dispatch, cask publication, end-to-end `go install`) are explicitly deferred to a tracked GitHub issue (#514) and/or a later ROADMAP phase (Phase 6), per the phase's own documented, reviewed design — not left as ambiguous or silent gaps.

### Gaps Summary

No blocking gaps. The phase's own artifacts (D-11, D-15, and issue #514) correctly and traceably account for every piece of the "proven to work end to end rather than merely configured" goal that cannot honestly be established from a feature branch: the credential probe's first live dispatch, the first real cask publication (Phase 6), and the end-to-end `go install …@vX.Y.Z` observation. All in-repo, in-branch deliverables — the version-json contract, the dev-build version derivation, the cask's install-time gate, the re-ship guard, and the credential probe's own correctness — are implemented, tested, and independently re-verified against the live codebase rather than taken from SUMMARY.md claims.

One non-blocking anti-pattern was found (see above): a single occurrence of a specifically-forbidden identifier in a `.goreleaser.yaml` comment, violating the plan's own explicit, unusually strict "zero occurrences anywhere, including comments" acceptance criterion. Recommend a small follow-up commit to reword that comment; does not block phase completion.

---
*Verified: 2026-08-24*
*Verifier: Claude (gsd-verifier)*
