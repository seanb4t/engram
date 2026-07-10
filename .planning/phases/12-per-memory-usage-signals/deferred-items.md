# Deferred Items — Phase 12

Out-of-scope discoveries logged, not fixed, per executor scope-boundary rules.

## 12-01: pre-existing markdown lint failures

`task` (lint+test) fails at the `lint:markdown` step with 331 pre-existing rumdl
issues across 37 `.planning/` files — none touched by 12-01's tasks (all in
`internal/store/`). The failures predate this plan: they live in phase-12
research/pattern/plan docs (`12-RESEARCH.md`, `12-PATTERNS.md`, `12-0[2-6]-PLAN.md`)
and earlier-phase artifacts (`11-REVIEW.md`, `10-VERIFICATION.md`,
`embedder-provider-routing-resilience.md` seed, `STATE.md`). `rumdl fmt` would
auto-fix 316/331 of them.

`task lint:go` (0 issues) and `task test` (all Go + Python suites green) both
pass independently — the Go code this plan touches is clean. Not fixed here:
out of scope per the scope-boundary rule (only auto-fix issues directly caused
by the current task's changes).
