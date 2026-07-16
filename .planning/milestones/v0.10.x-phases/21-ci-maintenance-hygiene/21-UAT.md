---
status: partial
phase: 21-ci-maintenance-hygiene
source: [21-VERIFICATION.md]
started: 2026-07-16
updated: 2026-07-16
---

## Current Test

[testing complete for this session — 1 item deferred, phase stays pending]

## Tests

### 1. Live Renovate self-heal end-to-end (REQ-ci-renovate-spa-drift / #301)
expected: |
  Next Renovate `ui/` bump PR self-heals: drift detected → App-token mint (guard passes)
  → SPA regenerated and pushed to the head branch → required checks rerun green on the new
  SHA → auto-merge completes. Fork PRs and non-Renovate PRs never reach the mint step.
  Verify no `main` merge-commit lands on the Renovate branch (merge-ref safety).
result: skipped
reason: "skip, can't verify at this time — no live Renovate ui/ bump PR exists yet to observe. Prerequisite-gated on a real-world event, not a testing choice. Tracked by #369; re-run /gsd-verify-work 21 once such a PR appears."
why_human: |
  GitHub Actions expression context and App-token push/re-trigger semantics only evaluate
  inside a real workflow run against a real Renovate PR. This is REQ-ci-renovate-spa-drift's
  own <requirement_completion_honesty> design: the requirement stays open until this live
  observation happens. Tracked by GitHub issue #369.

## Summary

total: 1
passed: 0
issues: 0
pending: 0
skipped: 1
blocked: 0

## Gaps

[none — the deferral is a prerequisite gate awaiting a live Renovate PR, not a code defect]
