# Phase 01 Plan 03: Cross-repo credential and re-ship guard — provisional

**Provisional file created by Task 3, before its blocking checkpoint pauses (L-E). Task 4
finalizes this file after the checkpoint clears — do not create a second file.**

## Task 3 — Grant the release-please App access to seanb4t/homebrew-tap

**Status: PENDING human action.** The credential is NOT yet confirmed. Everything
automatable is done (Tasks 1-2, committed): the token mints on both
`.github/workflows/release.yaml` and `.github/workflows/verify-tap-credential.yaml` request
`repositories: engram,homebrew-tap`, and the read-only probe workflow exists in the tree. The
remaining step — adding `seanb4t/homebrew-tap` to the release-please App's installation
repository list — is GitHub account-level UI state with no workflow-reachable API equivalent,
so it requires a human to act in the GitHub UI.

### Baseline HEAD SHA (captured automatically before any dispatch)

- **Repository:** `seanb4t/homebrew-tap`
- **Default branch:** `main`
- **HEAD commit SHA:** `969aef42d3d8f0d8290d0ad67b4013251ae955f9`
- **Captured:** 2026-08-23 (via `gh api repos/seanb4t/homebrew-tap --jq '.default_branch'` then
  `gh api repos/seanb4t/homebrew-tap/commits/main --jq '.sha'`)
- **No write occurred while capturing this baseline** — both calls above are `gh api` GETs.

This SHA is the baseline Task 4's post-merge no-write check will compare against. A
commit *message* would not be a unique baseline; this exact SHA is.

### Repository access list (observed) — AWAITING human confirmation

*Not yet recorded.* A human must open GitHub -> Settings -> Applications -> Installed
GitHub Apps -> the release-please App (behind `secrets.RELEASE_APP`) -> Configure, add
`seanb4t/homebrew-tap` under Repository access while keeping `seanb4t/engram` selected, and
report back what the Repository access list shows.

### Contents permission (observed) — AWAITING human confirmation

*Not yet recorded.* The human reads (does not set) the Repository permissions -> Contents
value on the same Configure screen and reports it here. Contents: Read and write is declared
by the App and accepted at installation; if it reads anything else, that is a different,
larger change (the App's own permission set needs updating and re-accepting) — stop and
report rather than improvising a workaround.

## Open items

- **[BLOCKING] Task 3 — grant not yet observed.** Awaiting the human's report of the App's
  Repository access list and Contents permission value. Once reported, this section and the
  two "AWAITING" subsections above are updated with the observed values, and the checkpoint's
  `resume-signal` ("granted") is honored.
- **[BLOCKING] Task 4 — credential probe not yet dispatched.** `workflow_dispatch` is only
  exposed for a workflow definition present on the default branch, so
  `.github/workflows/verify-tap-credential.yaml` cannot be dispatched from this feature
  branch. Task 4 (not yet run) files a tracked GitHub issue whose first line blocks any
  release-please PR merge until the probe has been dispatched from `main` and its result
  recorded on that issue.
