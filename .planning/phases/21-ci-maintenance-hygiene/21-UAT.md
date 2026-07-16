---
status: testing
phase: 21-ci-maintenance-hygiene
source: [21-VERIFICATION.md]
started: 2026-07-16
updated: 2026-07-16
---

## Current Test

number: 1
name: Observe the first real Renovate `ui/` bump PR self-heal end-to-end (GitHub issue #369)
expected: |
  On the next Renovate-authored `ui/` dependency-bump PR, the `ui-drift` CI job
  detects vendored-SPA drift, mints the GitHub App installation token (the three-signal
  guard passes: `renovate/` head-ref + `fzymgc-renovate[bot]` actor + same-repo head),
  commits the regenerated `internal/webauth/static/` onto the true PR head branch, and
  that push re-triggers `pull_request: synchronize` so all 8 required checks (including
  `ui vendored-asset drift`) rerun GREEN on the new SHA — allowing auto-merge to complete
  without a human running `task ui:build`. No `main`-into-branch merge occurs (merge-ref
  correction). A non-Renovate PR with drift still fails with the existing
  `::error::vendored SPA is stale` guidance.
awaiting: user response

## Tests

### 1. Live Renovate self-heal end-to-end (REQ-ci-renovate-spa-drift / #301)
expected: |
  Next Renovate `ui/` bump PR self-heals: drift detected → App-token mint (guard passes)
  → SPA regenerated and pushed to the head branch → required checks rerun green on the new
  SHA → auto-merge completes. Fork PRs and non-Renovate PRs never reach the mint step.
  Verify no `main` merge-commit lands on the Renovate branch (merge-ref safety).
result: [pending]
why_human: |
  GitHub Actions expression context and App-token push/re-trigger semantics only evaluate
  inside a real workflow run against a real Renovate PR. `task lint:actions` proves only
  that the YAML parses — not that the guarded push + re-trigger + auto-merge chain works.
  This is REQ-ci-renovate-spa-drift's own <requirement_completion_honesty> design: the
  requirement stays open (`[ ]` in REQUIREMENTS.md) until this live observation happens.
  Tracked by GitHub issue #369.

## Summary

total: 1
passed: 0
issues: 0
pending: 1
skipped: 0
blocked: 0

## Gaps
