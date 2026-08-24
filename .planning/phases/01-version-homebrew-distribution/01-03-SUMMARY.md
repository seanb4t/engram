---
phase: 01-version-homebrew-distribution
plan: 03
subsystem: ci-release
tags: [github-actions, goreleaser, homebrew-cask, github-app-token, workflow-dispatch]

# Dependency graph
requires:
  - phase: 01-01
    provides: the `homebrew_casks:` entry in `.goreleaser.yaml` this plan adds `skip_upload` to
provides:
  - "Newest-tag `skip_upload` guard: `SKIP_HOMEBREW_UPLOAD` computed before GoReleaser runs, templated into the cask's `skip_upload` field via a guarded optional-env idiom"
  - "Explicit App-token scope: `repositories: engram,homebrew-tap` on both the release job's and the probe's `create-github-app-token` mints"
  - "Standalone, read-only, `workflow_dispatch`-only `verify-tap-credential.yaml` probe asserting push access to `seanb4t/homebrew-tap`"
  - "release-please App installation grant on `seanb4t/homebrew-tap`, observed in the GitHub UI (Task 3)"
  - "Tracked GitHub issue gating the next release-please PR merge on the probe's first post-merge dispatch (Task 4)"
affects: ["Phase 6 (cask publication observation, REQ-homebrew-cask-published)"]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
actuals:
  tokens: 14000
  tasks: 4
  commits: 5

tech-stack:
  added: []
  patterns:
    - "Compute-before-template guard: a value GoReleaser's Go templates cannot derive (`git tag -l`) is computed in a workflow step and passed through `$GITHUB_ENV`, never inlined into the template itself"
    - "Guarded optional-env template idiom (`{{ if index .Env \"X\" }}...{{ else }}...{{ end }}`) so local `task release:snapshot` renders keep working when CI-only env vars are unset"
    - "GitHub App token scope as a closed allowlist (`repositories: a,b`), not an incremental grant — stated explicitly in a comment so a later reader does not 'tighten' it and break a sibling step in the same job"
    - "Read-only `workflow_dispatch`-only probe workflow, kept separate from the release workflow whose `workflow_dispatch` trigger carries release side effects"

key-files:
  created:
    - .github/workflows/verify-tap-credential.yaml
  modified:
    - .github/workflows/release.yaml
    - .goreleaser.yaml

key-decisions:
  - "Credential probe lives in its own workflow file, not folded into release.yaml — release.yaml's workflow_dispatch requires a tag input and treats any supplied tag as a re-ship instruction, so a probe dispatched there would also re-ship artifacts"
  - "App-token mint names BOTH repositories (engram,homebrew-tap), not just the tap — the allowlist REPLACES the unset default rather than adding to it, and scoping to the tap alone would strip engram from the token used by release-please earlier in the same job"
  - "skip_upload uses the guarded index .Env idiom, not skip_upload: auto — auto skips prerelease-tagged tags, a different rule from the newest-tag rule this phase implements"
  - "Probe dispatch is a post-merge observation (Task 4's tracked issue), not a pre-merge gate — GitHub only exposes workflow_dispatch for workflow definitions on the default branch, so a brand-new workflow cannot be dispatched from a feature branch (repairs cycle-1 HIGH-3)"
  - "No CI status check enforces the merge-block instruction — both cycle-2 reviewers rejected that, since it would assert GitHub App-installation state and a third-party repository's behavior (project rule m45p2b4bp7, D-11); enforcement is a maintainer reading the issue's first line"

requirements-completed: [REQ-cask-credential-verified, REQ-cask-reship-recovery]

coverage:
  - id: D1
    description: "SKIP_HOMEBREW_UPLOAD computed before GoReleaser, false exactly when the shipped tag is the newest v* tag, true otherwise; templated into the cask's skip_upload via a guarded optional-env idiom"
    requirement: "REQ-cask-reship-recovery"
    verification:
      - kind: other
        ref: "task lint:actions && task lint:yaml && task release:check, plus 6 rg occurrence-count acceptance gates (ordering, both boundary branches, index .Env guard, skip_upload:auto rejection)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Release App token scoped to exactly engram and homebrew-tap on both create-github-app-token mints; standalone read-only workflow_dispatch-only probe workflow exists and parses under actionlint"
    requirement: "REQ-cask-credential-verified"
    verification:
      - kind: other
        ref: "task lint:actions && task lint:yaml, plus 12 rg occurrence-count acceptance gates (allowlist scope, no owner: input, trigger shape, no-inputs, token-via-env-not-cli, read-only write-verb exclusion, contents:read/no contents:write)"
        status: pass
    human_judgment: false
  - id: D3
    description: "release-please App granted access to seanb4t/homebrew-tap in the GitHub UI; tap's pre-dispatch baseline HEAD SHA captured and unchanged after the grant"
    requirement: "REQ-cask-credential-verified"
    verification:
      - kind: manual_procedural
        ref: "gh api repos/seanb4t/homebrew-tap/commits/main --jq '.sha' before and after the grant, both returning 969aef42d3d8f0d8290d0ad67b4013251ae955f9"
        status: pass
    human_judgment: true
    rationale: "The App installation's Repository access and Contents permission are GitHub account-level UI state with no workflow-reachable API to confirm; the observation was made by a human reading the Configure screen and reported back (see 'Observed installation state' below), and full functional confirmation is deferred to the verify-tap-credential probe (Task 4, post-merge)."
  - id: D4
    description: "GitHub issue filed: first line blocks any release-please PR merge until checklist A (credential probe dispatch) is recorded; also carries the cask-publication handoff to Phase 6 and 01-02's end-to-end go install check"
    requirement: "REQ-cask-credential-verified"
    verification:
      - kind: other
        ref: "gh issue view <N> --json body plus rg gates on the issue body (merge-block head -1, checklist markers, no .github/ changes)"
        status: pass
    human_judgment: false

duration: ~55min
completed: 2026-08-24
status: complete
---

# Phase 01 Plan 03: Cross-repo credential and re-ship guard Summary

**Newest-tag `skip_upload` guard plus a scoped, read-only `workflow_dispatch` credential probe make the Homebrew cask publishing pipeline correct by construction — a `workflow_dispatch` backfill of an older tag can no longer regress the tap, and the cross-repo App token's push access is checkable without a real release.**

## Performance

- **Duration:** ~55 min (across a human-action checkpoint pause)
- **Tasks:** 4
- **Files modified:** 3 (2 modified, 1 created)

## Accomplishments

- `release.yaml` computes `SKIP_HOMEBREW_UPLOAD` from a `git tag -l 'v*' --sort=-v:refname` newest-tag comparison, in a new "Resolve Homebrew upload guard" step, before the `goreleaser-action` step runs; the cask's `skip_upload` field reads it through a guarded optional-env template.
- Both `create-github-app-token` mints (release job and probe) now request `repositories: engram,homebrew-tap` explicitly, documented as a closed allowlist that cannot be tightened to the tap alone without breaking release-please's writes to `engram`.
- `.github/workflows/verify-tap-credential.yaml` — a new, standalone, `workflow_dispatch`-only, input-free, read-only workflow that asserts `gh api repos/seanb4t/homebrew-tap --jq '.permissions.push'` is `true`.
- The release-please GitHub App ("fzy Release-Please", installation id `138490520`) was granted access to `seanb4t/homebrew-tap` in the GitHub UI; the tap's pre-grant baseline HEAD SHA was captured and confirmed unchanged after the grant.
- A tracked GitHub issue carries all three post-merge observations this phase cannot honestly make from a feature branch, with the credential probe dispatch as a hard, first-line merge block.

## Task Commits

Each task was committed atomically:

1. **Task 1: Re-ship guard — newest-tag comparison templated into `skip_upload`** - `b96bb92f` (feat)
2. **Task 2: Cross-repo credential — explicit token scope plus a standalone read-only probe** - `91b23e88` (feat)
3. **Task 3, part A (provisional, pre-checkpoint): capture tap baseline SHA, create provisional SUMMARY** - `c5546f0b` (docs)
4. **Task 3, part B (finalized, post-checkpoint): record observed installation state** - `a0b06096` (docs)
5. **Task 4: file the post-merge tracking issue** - GitHub issue [#514](https://github.com/seanb4t/engram/issues/514) (no source file modified); this file's citation of it is the commit below

_Task 3 is a `checkpoint:human-action` — its provisional-SUMMARY commit landed before the pause; its finalization (this file) and Task 4's issue-citation update land in the commits listed under "Files Created/Modified" below._

## Files Created/Modified

- `.github/workflows/release.yaml` — adds the "Resolve Homebrew upload guard" step (computing and exporting `SKIP_HOMEBREW_UPLOAD`) and the `repositories: engram,homebrew-tap` input on the existing App-token mint
- `.goreleaser.yaml` — adds the guarded `skip_upload` template field to the `homebrew_casks` entry
- `.github/workflows/verify-tap-credential.yaml` — new standalone read-only credential probe
- `.planning/phases/01-version-homebrew-distribution/01-03-SUMMARY.md` — this file

## Decisions Made

See `key-decisions` in frontmatter. The most consequential: the probe's first dispatch is necessarily a post-merge observation (GitHub only exposes `workflow_dispatch` for workflows registered on the default branch), so `REQ-cask-credential-verified`'s "proven before any real release depends on it" is satisfied by Task 4's tracked issue ordering the dispatch before the next release-please merge — enforced by a maintainer reading a blocking instruction, not by a CI gate (per this repo's hard rule against testing third-party/App-installation behavior, `m45p2b4bp7`, and D-11).

## Deviations from Plan

None — plan executed exactly as written, including the checkpoint sequencing cycle-1 review corrected (HIGH-3: probe dispatch moved to Task 4's post-merge checklist rather than a pre-merge gate).

## Observed installation state (Task 3, human-reported)

Captured from a screenshot of `github.com/settings/installations/138490520`, reported by the user after completing the GitHub UI grant:

- **App:** "fzy Release-Please" ("Release Please App to allow specific permissions and rule eval bypass"), installation id `138490520`.
- **Permissions (as rendered by GitHub's summary line):** "Read access to metadata"; "Read and write access to code, issues, and pull requests." GitHub renders the Contents permission as "code" in this summary line — **Contents is Read and write**, matching this plan's requirement.
- **Repository access:** "Only select repositories", "Selected 5 repositories." Visible in the list: `seanb4t/engram`, `seanb4t/fovea`, `seanb4t/codegraph-go`, `seanb4t/homebrew-tap` (a 5th row was below the screenshot's scroll fold and was not captured). Both repositories this plan cares about — `seanb4t/engram` and `seanb4t/homebrew-tap` — are present.

**Caveat, recorded honestly:** the screenshot showed the panel's Save/Cancel buttons present, so the image alone did not confirm the selection was persisted rather than merely staged; the user was separately asked to confirm Save. This SUMMARY does **not** assert the grant is verified live — only that the access list was observed in the UI to include `homebrew-tap`. Live verification of push access remains the job of the `verify-tap-credential` probe, which Task 4's tracked issue gates the next release-please merge on.

**No-write confirmation:** `seanb4t/homebrew-tap`'s HEAD SHA on `main` was re-read after the grant and is byte-identical to the pre-grant baseline: `969aef42d3d8f0d8290d0ad67b4013251ae955f9`. The grant itself performed no write to the tap.

**Why API confirmation was not attempted:** `gh api /user/installations` returns 403 ("You must authenticate with an access token authorized to a GitHub App") for a personal-access-token-authenticated `gh` session — a PAT cannot enumerate or confirm GitHub App installations. This is precisely why `verify-tap-credential.yaml`'s own token-scoped probe (using the App's own minted token, not a PAT) is the only mechanism that can confirm push access, and why Task 4's issue makes its dispatch a hard precondition rather than an optional nicety.

## Issues Encountered

None beyond the expected checkpoint pause for the GitHub App installation grant, which has no CLI/API equivalent (documented in the plan's `<objective>` and `user_setup`).

## User Setup Required

**Completed as part of this plan's Task 3** (not deferred): the release-please App's installation was granted access to `seanb4t/homebrew-tap`, observed in the GitHub UI as described above. No further manual dashboard configuration is required by this plan.

## Open items

- **[BLOCKING, tracked] Credential probe not yet dispatched.** `workflow_dispatch` is only exposed for a workflow definition present on the default branch, so `.github/workflows/verify-tap-credential.yaml` cannot be dispatched from this feature branch. **Tracking issue: [#514](https://github.com/seanb4t/engram/issues/514) — "Phase 01 post-merge: verify tap credential (blocks release-please merge)"** — its first line blocks any release-please PR merge until checklist A (the probe dispatch, from `main`) has been run and its result — including a comparison of the tap's HEAD SHA against the `969aef42d3d8f0d8290d0ad67b4013251ae955f9` baseline recorded above — is recorded on that issue.
- **[Handoff to Phase 6, non-blocking] Cask publication observation.** `REQ-homebrew-cask-published` (ROADMAP Phase 6 criterion 3, moved from Phase 1 by developer decision 2026-08-23, commit `b87071f6`) is carried in issue [#514](https://github.com/seanb4t/engram/issues/514)'s checklist B, explicitly labelled as Phase 6's. Not a Phase 1 debt.
- **[Tracked, non-blocking] End-to-end `go install` version check (01-02, M-D).** Every tag today predates `cmd/engram/buildversion.go`; the check is carried in issue [#514](https://github.com/seanb4t/engram/issues/514)'s checklist C, runnable against the first tag that contains that file.
- **Not touched by this plan: `.planning/REQUIREMENTS.md`.** `REQ-cask-credential-verified` is closed in-branch by configuration plus the App-installation grant, but per the plan's own "Requirement scope" section its REQUIREMENTS.md checkbox is marked complete only once the probe's first dispatch (checklist A) is recorded on issue #514 — deferred to whoever executes that checklist post-merge, not this plan.

## Next Phase Readiness

- Phase 1's cask-publishing pipeline is correct by construction: a `workflow_dispatch` backfill of an older tag cannot regress the tap (guard verified via 6 fixture-proven acceptance gates), and the cross-repo credential is explicitly scoped and checkable via a standalone probe.
- Blocker for the next release-please merge: the tracking issue's checklist A (credential probe dispatch from `main`) must be recorded first — this is intentional, tracked, and not a Phase 1 gap.
- No blockers for proceeding to phase verification; the plan's `<verification>` section explicitly lists which items are not verified in-phase by design (probe dispatch, cask publication, end-to-end `go install`) and states they must not be failed here as missing.

---
*Phase: 01-version-homebrew-distribution*
*Completed: 2026-08-24*
