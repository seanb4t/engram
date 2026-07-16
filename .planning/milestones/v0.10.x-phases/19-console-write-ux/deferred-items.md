# Deferred Items — Phase 19

Out-of-scope discoveries logged, not fixed, per executor scope-boundary rules.

## 19-06: pre-existing markdown lint failures

`task` (lint+test) fails at the `lint:markdown` step with 1342 pre-existing
rumdl issues across 139 `.planning/` files — none touched by 19-06's tasks
(all in `ui/` and `internal/webauth/static/`). The failures predate this
plan: they span phase-14/15/16/19 research/plan/summary docs (`14-REVIEWS.md`,
`19-RESEARCH.md`, `19-PATTERNS.md`, `19-UI-SPEC.md`, `19-0[2-5]-PLAN.md`,
`19-0[2-5]-SUMMARY.md`, etc.). This is `.rumdl.toml`'s known `.planning`
exclude gap, already tracked in `PROJECT.md`'s v0.10.x scope as a deferred
Phase 21 CI-hygiene item ("`.rumdl.toml` `.planning` exclude → Phase 21").

`task lint:go` (0 issues), `task lint:yaml`, `task lint:actions`,
`task lint:python`, and `task test` (all Go + Python suites green) all pass
independently — the code this plan touches is clean. `task ui:build` is
byte-reproducible (`git diff --exit-code -- internal/webauth/static/` clean
immediately after a fresh rebuild), satisfying T-19-65's actual intent (the
shipped binary carries the write UX and CI's dedicated `ui-drift` job, which
does not run `lint:markdown`, will pass). Not fixed here: out of scope per
the scope-boundary rule (only auto-fix issues directly caused by the current
task's changes) and per `.rumdl.toml`'s own exclude list, which is Phase 21's
responsibility, not Phase 19's.
