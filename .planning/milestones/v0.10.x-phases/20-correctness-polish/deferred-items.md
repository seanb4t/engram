# Deferred Items — Phase 20 Plan 01

- `task` (default lint+test) halts at `lint:markdown` (`rumdl check .`) with
  1476 pre-existing issues across `.planning/**` docs (phases 14/15,
  todos/done, research/SUMMARY.md, seeds/*). This is the systemic
  `.rumdl.toml` `.planning`-exclude gap already tracked in STATE.md
  ("Systemic `.rumdl.toml` `.planning` exclude → Phase 21"). None of these
  findings are in files touched by this plan — out of scope per the
  executor's scope-boundary rule. Ran `lint:go`/`test:go` directly instead
  to verify this plan's changes.
