---
phase: 01-version-homebrew-distribution
verified: 2026-08-25T00:50:00Z
status: passed
score: 5/5 must-haves verified (ROADMAP success criteria); 0 gaps; 0 blocking anti-patterns; 3 tracked deferrals
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: 5/5
  gaps_closed:
    - "`.goreleaser.yaml` comment leaking `generate_completions_from_executable` (prior WARNING anti-pattern) — fixed in 49e64913, occurrence count independently re-confirmed at 0"
  gaps_remaining: []
  regressions: []
deferred:
  - truth: "SC4 — cross-repo credential proven by an explicit check before any real release depends on it"
    addressed_in: "GitHub issue #514, checklist A (post-merge, blocking on next release-please merge)"
    evidence: "workflow_dispatch is only exposed for workflow definitions present on the default branch; the probe (.github/workflows/verify-tap-credential.yaml) cannot be dispatched from this feature branch. Issue #514 remains OPEN, first line still reads 'DO NOT MERGE A RELEASE-PLEASE PR UNTIL CHECKLIST A BELOW IS RECORDED ON THIS ISSUE.' Confirmed via `gh issue view 514 --json state` -> OPEN."
  - truth: "REQ-homebrew-cask-published — a tagged release actually publishes Casks/engram.rb to the tap"
    addressed_in: "Phase 6 (ROADMAP moved this requirement out of Phase 1 by developer decision, commit b87071f6)"
    evidence: "REQUIREMENTS.md traceability table: 'REQ-homebrew-cask-published | Phase 6 | Pending'. Issue #514 checklist B carries the runnable observation as a Phase 6 handoff."
  - truth: "End-to-end `go install .../cmd/engram@vX.Y.Z` resolves to the bare tag version"
    addressed_in: "Issue #514, checklist C"
    evidence: "Every git tag in this repo predates cmd/engram/buildversion.go, so no go install of a real tag can reach the new resolver in-phase; TestVersionFromModuleVersion proves the parser half only."
---

# Phase 1: Version & Homebrew Distribution Verification Report (RE-VERIFICATION)

**Phase Goal:** A user can install engram with `brew install` on macOS or Linux (amd64 and arm64), and
the pipeline that publishes it is proven to work end to end rather than merely configured.
`engram version --output json` lands in this same phase because it is the cask's install-time
correctness gate.

**Verified:** 2026-08-25
**Status:** passed
**Re-verification:** Yes — after credential design changed (commits 49e64913, f67622eb, dcafc5c1) post-prior-PASS

## Why re-verification, and what was re-derived

Three commits landed after the prior VERIFICATION.md (dated 2026-08-24, status `passed`, score
5/5). Per the dispatch instructions, the credential mechanism — the exact thing SC4's "proven to
work end to end rather than merely configured" clause is about — was re-derived from the live
code rather than inherited from the prior verdict, and from `01-SECURITY.md`'s prior finding
(SECURED, threats_open: 0) rather than trusted at face value. `01-SECURITY.md` was itself read
critically: it already flagged the two rows the redesign touched (T-01-08, T-01-12) and shows its
work re-deriving them against `actions/create-github-app-token`'s compiled source — that citation
was not independently re-run in this pass, since verifying a pinned third-party action's compiled
JS is outside this agent's scope and the finding is internally consistent with what the workflow
files declare.

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `engram version --output json` prints a machine-readable payload; human-readable `engram version` unchanged | ✓ VERIFIED | Untouched by the three delta commits. Re-ran: `go test ./cmd/engram -run 'TestVersion...' -count=1 -v` — `TestVersionJSONLane`, `TestVersionTextLane`, `TestVersionExplicitTextLane`, `TestVersionTextEqualsJSON`, `TestVersionOutputFlagDefault` all `--- PASS`. |
| 2 | Repo configured to publish a cask to `seanb4t/homebrew-tap` via `homebrew_casks:`, amd64+arm64 on macOS+Linux, validated locally without publishing | ✓ VERIFIED | `.goreleaser.yaml`: `homebrew_casks:` present, `brews:` absent; `builds.goos: [linux, darwin]`, `builds.goarch: [amd64, arm64]` (re-read at lines 24-29). `task release:check` exits with only the pre-existing, unrelated `dockers`/`docker_manifests` deprecation warning — 0 errors. |
| 3 | A binary failing the version-json assertion makes `brew install` fail loudly; quarantine strip runs before the gate | ✓ VERIFIED (by construction, D-11/D-15, unchanged) | Untouched by the three delta commits except the comment reword (item below). `xattr` strip precedes the `system_command binary ... version --output json` assertion; `raise` on mismatch present. Re-confirmed by direct read of the current hook body. |
| 4 | Cross-repo credential to `seanb4t/homebrew-tap` proven by an explicit check before any real release depends on it | ✓ VERIFIED / DEFERRED (see `deferred:` frontmatter) — **re-derived under the new credential design** | The credential mechanism materially changed and was re-verified against the live files, not inherited: `release.yaml`'s release-App mint (`id: app-token`) now carries no `repositories:`/`owner:` input — defaults to this-repo-only. A second, dedicated `id: tap-token` mint requests `owner: seanb4t` + `repositories: homebrew-tap`, exposed to GoReleaser as `HOMEBREW_TAP_TOKEN` (`release.yaml:191`, `steps.tap-token.outputs.token`). `.goreleaser.yaml:105` sets `homebrew_casks[].repository.token` to a guarded read of that env var. `verify-tap-credential.yaml` now mints the same TAP App (`app-id: secrets.TAP_PUBLISHER_APP`, `owner: seanb4t`, `repositories: homebrew-tap`) rather than the release App — probing the release App would now assert the wrong thing, and the workflow correctly does not. `rg 'engram,homebrew-tap' .github/ .goreleaser.yaml` returns zero matches — no single credential spans both repositories any more. Secrets `TAP_PUBLISHER_APP`/`TAP_PUBLISHER_APP_PRIVATE_KEY` confirmed present via `gh secret list` (created 2026-08-25T00:33-00:34Z). The probe's first dispatch still cannot happen from a feature branch (GitHub platform constraint, unchanged); issue #514 is confirmed OPEN, its first line still hard-blocks the next release-please merge on checklist A, and — critically — a 2026-08-25 comment thread on the issue documents that the setup steps were rewritten for the new two-App design and that the maintainer reported the 4 prerequisite steps (create tap App, install on tap only, add secrets, revoke tap-please App's tap access) done; the issue body's own "Background" section is stale prose (still describes the old combined-scope design) but the comment thread supersedes it in the same way `01-03-SUMMARY.md`'s "Superseded" section supersedes its own body — the deferral is sound and traceable through both. |
| 5 | A rehearsed failure between tag creation and cask publication recovers via the existing `workflow_dispatch` re-ship path, no hand-edit to the tap | ✓ VERIFIED (by construction, D-11/D-15, unchanged) | `SKIP_HOMEBREW_UPLOAD` guard independently re-confirmed against current line numbers (shifted by the delta commits): `actions/checkout` (line 98) < "Resolve Homebrew upload guard" step (line 152) < `goreleaser-action` (line 182). `=false`/`=true` branches each appear exactly once. `.goreleaser.yaml`'s `skip_upload` still reads `index .Env "SKIP_HOMEBREW_UPLOAD"` with a `false` fallback. |

**Score:** 5/5 ROADMAP success criteria verified (3 and 5 by an explicit, reviewed, documented construction-not-rehearsal decision, unchanged by this delta; 4 re-derived under a materially different credential design and still correctly built, with its first live proof deliberately and traceably deferred post-merge).

### Deferred Items

See `deferred:` in frontmatter — all three remain tracked (GitHub issue #514, confirmed OPEN, and ROADMAP's Phase 6) and none represents undocumented scope loss. None of the three deferrals changed in nature across this delta; #514's setup steps were updated in-place via its comment thread to match the new two-App design.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/engram/version.go` | `--output json\|text`, `versionDoc`, `runVersion` | ✓ VERIFIED (unchanged) | Untouched by delta commits; tests re-pass. |
| `cmd/engram/buildversion.go` | `lastRelease`, `nextPatch`, `deriveDevVersion`, `versionFromModuleVersion`, `resolvedVersion` | ✓ VERIFIED (unchanged) | `lastRelease = "0.14.0" // x-release-please-version` confirmed present; tests re-pass. |
| `.goreleaser.yaml` | `homebrew_casks:` block, ordered post-install hook, `skip_upload`, tap token | ✓ VERIFIED (delta-affected, re-checked) | `repository.token` now reads `HOMEBREW_TAP_TOKEN` (line 105); `generate_completions_from_executable` occurrence count independently re-run = 0 (was 1 at prior verification — now fixed by 49e64913); reworded comment still explains the rescue-to-warning rationale without naming the identifier. |
| `.github/workflows/release.yaml` | Explicit token scope, upload guard step, dual App mints | ✓ VERIFIED (delta-affected, re-checked) | Two `create-github-app-token` steps confirmed: `id: app-token` (no `repositories`/`owner`, this-repo-only) and `id: tap-token` (`owner: seanb4t`, `repositories: homebrew-tap`). Guard-step ordering re-confirmed against new line numbers. |
| `.github/workflows/verify-tap-credential.yaml` | Standalone, read-only, `workflow_dispatch`-only probe, now targeting the TAP App | ✓ VERIFIED (delta-affected, re-checked) | Full file re-read; mints `secrets.TAP_PUBLISHER_APP`/`TAP_PUBLISHER_APP_PRIVATE_KEY` with `owner: seanb4t` + `repositories: homebrew-tap`; asserts `.permissions.push == "true"`, `exit 1` otherwise; token via `env: GH_TOKEN`, never CLI arg. |
| `release-please-config.json` | 4th `extra-files` entry (generic, `cmd/engram/buildversion.go`) | ✓ VERIFIED (unchanged) | Untouched by delta commits. |
| Repository secrets | `TAP_PUBLISHER_APP`, `TAP_PUBLISHER_APP_PRIVATE_KEY` | ✓ VERIFIED (new, delta-affected) | `gh secret list --repo seanb4t/engram` shows both, created 2026-08-25T00:33:46Z and 00:34:14Z respectively, alongside the pre-existing `RELEASE_APP`/`RELEASE_APP_PRIVATE_KEY`. |
| `01-03-SUMMARY.md` | Superseded section overriding stale single-App claims | ✓ VERIFIED | Read in full; the "Superseded" section leads the file, states plainly that everything below describing a single scoped App is historical, and accurately describes the shipped two-App design. No contradiction found between the superseding text and the current code. |
| `01-SECURITY.md` | Threat register re-derived for the two rows the redesign touched | ✓ VERIFIED | T-01-08 (Elevation of Privilege, high) and T-01-12 (Repudiation, low) both explicitly rewritten and CLOSED against the new code; verdict SECURED, threats_open: 0. Cross-checked its "Also confirmed" note against the live `.goreleaser.yaml` — correct (0 occurrences). |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `.goreleaser.yaml` post-install hook | `cmd/engram/version.go` | shells `engram version --output json`, compares to declared version | ✓ WIRED (unchanged) | Confirmed by direct read; unaffected by delta commits except the comment reword. |
| `release.yaml` `id: tap-token` step | `.goreleaser.yaml` `homebrew_casks[].repository.token` | `HOMEBREW_TAP_TOKEN` env var, set at `release.yaml:191` from `steps.tap-token.outputs.token`, read via `.goreleaser.yaml:105`'s guarded `index .Env` template | ✓ WIRED (new, delta-affected) | Both ends independently re-confirmed by direct read — this is the material new link introduced by the credential split, and it closes correctly. |
| `release.yaml` `id: app-token` step | release-please + tag/GHRelease writes | `steps.app-token.outputs.token` passed to `release-please-action` | ✓ WIRED (unchanged in mechanism, narrower in scope) | Token now defaults to this-repo-only scope; still correctly feeds `release-please-action`'s `token:` input. |
| `.github/workflows/verify-tap-credential.yaml` | `seanb4t/homebrew-tap` | `gh api repos/seanb4t/homebrew-tap --jq .permissions.push`, read-only, using the TAP App's own minted token | ✓ WIRED (code, re-pointed to the correct App); not yet dispatched | Probe now mints the tap-publisher App rather than the release App — verified this matches the App that will actually hold `homebrew_casks[].repository.token` at publish time, so the probe proves the credential that matters. Dispatch still deferred post-merge (issue #514). |
| `.github/workflows/release.yaml` | `.goreleaser.yaml` | `SKIP_HOMEBREW_UPLOAD` via `$GITHUB_ENV` → `index .Env` | ✓ WIRED (unchanged) | Both sides independently re-confirmed against current line numbers. |
| `cmd/engram/version.go` / `cmd/engram/serve.go` | `cmd/engram/buildversion.go` | `resolvedVersion()` | ✓ WIRED (unchanged) | Untouched by delta commits. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Named phase-01 Go tests all pass individually | `go test ./cmd/engram -run 'TestVersion...\|TestNextPatch\|TestDeriveDevVersion\|TestVersionFromModuleVersion\|TestLastReleaseMatchesManifest\|TestExitCodeBaseline' -count=1 -v` | All `--- PASS`, `ok` | ✓ PASS |
| `go build ./...` | `go build ./...` | exits 0, no output | ✓ PASS |
| GoReleaser config schema validation | `task release:check` | exits 0 (only pre-existing, unrelated `dockers`/`docker_manifests` deprecation warning) | ✓ PASS |
| Actions/YAML lint | `task lint:actions`, `task lint:yaml` | both `task: ok` | ✓ PASS |
| `generate_completions_from_executable` occurrence count in `.goreleaser.yaml` | `rg -n -o -F 'generate_completions_from_executable' .goreleaser.yaml \| wc -l` | `0` | ✓ PASS (was `1` at prior verification — regression closed by 49e64913) |
| No credential spans both repositories | `rg 'engram,homebrew-tap' .github/ .goreleaser.yaml` | no matches | ✓ PASS |
| Two distinct App-token mints in `release.yaml` | direct read | `id: app-token` (no repo/owner input) and `id: tap-token` (`owner: seanb4t`, `repositories: homebrew-tap`) both present | ✓ PASS |
| `SKIP_HOMEBREW_UPLOAD`/upload-guard ordering, re-derived at new line numbers | `grep -n` against current `release.yaml` | checkout=98 < guard=152 < goreleaser-action=182; `=false` count=1, `=true` count=1 | ✓ PASS |
| Tap-publisher secrets exist | `gh secret list --repo seanb4t/engram` | `TAP_PUBLISHER_APP`, `TAP_PUBLISHER_APP_PRIVATE_KEY` both present, created 2026-08-25 | ✓ PASS |
| Issue #514 still open and still blocking | `gh issue view 514 --json state` | `OPEN`; body first line unchanged (merge-block instruction) | ✓ PASS |
| Debt-marker scan on all files this phase (incl. delta commits) modified | `grep -n -i -E 'TBD\|FIXME\|XXX\|TODO\|HACK\|PLACEHOLDER\|not yet implemented\|coming soon'` across `.goreleaser.yaml`, `release.yaml`, `verify-tap-credential.yaml`, `version.go`, `buildversion.go`, `serve.go`, `root.go`, `release-please-config.json` | no matches | ✓ PASS |
| Full workspace gate (`task`), run once, this pass | `task lint:actions && task lint:yaml && command go build ./... && command go test ./cmd/engram -run ...` (targeted, not full `go test ./...` — already known green except tracked #513 per dispatch instructions) | all pass | ✓ PASS |
| Pre-existing known failure re-confirmed unchanged | `go test ./internal/store -run TestRedEvidencePatchesAreLive -count=1 -v` | fails identically: `redEvidenceDirs is empty while 1 active-milestone phase director(ies) exist` | ✓ CONFIRMED unchanged, tracked #513, not a regression from this phase |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|-----------------|-------------|--------|----------|
| REQ-version-json | 01-01, 01-02 | Machine-readable `engram version`, existing text output unchanged | ✓ SATISFIED | `[x]` Complete in REQUIREMENTS.md; unaffected by delta, re-verified via all 5 version tests. |
| REQ-cask-install-gate | 01-01 | `brew install` fails loudly on version mismatch; quarantine strip first; no rescuing helper delegation | ✓ SATISFIED (prior WARNING now closed) | `[x]` Complete in REQUIREMENTS.md. The prior verification's non-blocking anti-pattern (the forbidden identifier leaking into a `.goreleaser.yaml` comment) is now fixed: independently re-run occurrence count is 0, and `01-SECURITY.md`'s "Also confirmed" section corroborates. |
| REQ-cask-credential-verified | 01-03 | Cross-repo credential proven by explicit check before any real release depends on it | ○ CORRECTLY BUILT UNDER THE NEW DESIGN, DISPATCH DEFERRED | `[ ]` Pending in REQUIREMENTS.md — deliberately. The check now targets the correct, narrower TAP App and was re-derived, not inherited. Deferral remains sound and traceable (issue #514, OPEN, updated via comments for the new design). |
| REQ-cask-reship-recovery | 01-03 | Recoverable without hand-editing the tap, via the existing `workflow_dispatch` path, rehearsed once | ✓ SATISFIED (by construction, D-11/D-15, unaffected by credential split) | `[ ]` Pending in REQUIREMENTS.md by intentional design (closes alongside REQ-cask-credential-verified per issue #514 checklist A); guard logic re-verified at current line numbers. |
| REQ-homebrew-cask-published | (moved to Phase 6, commit `b87071f6`) | A tagged release publishes the cask | N/A — not this phase | Unaffected by delta. `Phase 6 \| Pending`. |

**Orphan check:** `grep -E "Phase 1" REQUIREMENTS.md` returns exactly the 4 requirement IDs declared across the three plans' `requirements:` frontmatter. No orphaned requirement.

### Anti-Patterns Found

None blocking. The single prior WARNING (`generate_completions_from_executable` leaking into a `.goreleaser.yaml` comment) is closed — commit `49e64913` reworded the comment and independent re-count confirms 0 occurrences, while the reworded prose still conveys why the Homebrew helper is rejected as the install gate (rescue-to-warning behavior described without naming the identifier).

One informational note, not a blocker: GitHub issue #514's *body* "Background" section still describes the superseded single-App design (stale prose), while its *comment thread* (2026-08-25) correctly documents the new two-App design, the rewritten prerequisite checklist, and the maintainer's confirmation that prerequisites were completed. This mirrors `01-03-SUMMARY.md`'s own "Superseded" pattern and does not create ambiguity about what checklist A actually verifies today (the tap-publisher App's push access), but a follow-up edit to the issue body itself (not just a comment) would remove the staleness for a future reader who does not scroll to the comments. Non-blocking; does not affect phase goal achievement.

### Human Verification Required

None. All items that cannot be closed within this phase (credential-probe dispatch, cask publication, end-to-end `go install`) are explicitly deferred to a tracked, OPEN GitHub issue (#514) and/or a later ROADMAP phase (Phase 6), per the phase's own documented, reviewed design.

### Gaps Summary

No blocking gaps, and no regressions introduced by the three delta commits. The credential
redesign (commit `f67622eb`) — the material change this re-verification exists to scrutinize — was
independently re-derived against the live workflow and GoReleaser files rather than accepted from
`01-03-SUMMARY.md`'s Superseded section or `01-SECURITY.md`'s prior audit: the release App's mint
now defaults to this-repo-only scope with no `repositories:`/`owner:` input, a separate
tap-publisher App is minted with `owner: seanb4t` + `repositories: homebrew-tap` and exposed as
`HOMEBREW_TAP_TOKEN`, `.goreleaser.yaml` consumes that token for the cask push only, and the
credential probe was correctly re-pointed at the tap App rather than left probing the (now
scope-reduced) release App. No credential string spanning both repositories remains anywhere in
`.github/` or `.goreleaser.yaml`. The two secrets the new design requires
(`TAP_PUBLISHER_APP`/`TAP_PUBLISHER_APP_PRIVATE_KEY`) exist in the repository. The prior WARNING
anti-pattern is closed. The phase's `go build`, targeted `go test`, `task release:check`,
`task lint:actions`, and `task lint:yaml` all pass; the one failing test
(`TestRedEvidencePatchesAreLive`) is re-confirmed identical to the phase's known, tracked,
pre-existing state (#513) and is not caused by this phase.

The phase goal — "a user can install engram with `brew install`... and the pipeline that publishes
it is proven to work end to end rather than merely configured" — remains correctly and honestly
scoped: the *pipeline mechanics* (build matrix, cask config, install-time gate, re-ship guard,
credential separation) are proven by construction and independently re-verified in this pass; the
*end-to-end proof* (first probe dispatch, first cask publication, first `go install` of a
buildversion-carrying tag) is deliberately deferred to post-merge observation on a tracked, OPEN
issue, exactly as the phase's own D-11/D-15 decisions and SC2/SC4 wording anticipate. This is not a
regression from the prior PASS; if anything the credential boundary is now stronger (two
single-purpose Apps instead of one dual-purpose one), and that strengthening was independently
confirmed rather than taken on faith.

---
*Verified: 2026-08-25*
*Verifier: Claude (gsd-verifier)*
