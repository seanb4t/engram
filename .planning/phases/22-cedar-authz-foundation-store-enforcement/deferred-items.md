# Deferred Items — Phase 22

Out-of-scope discoveries logged during execution (not fixed — pre-existing, unrelated to this
phase's changes).

- **`.github/workflows/ci.yaml` yamlfmt drift.** `task lint` (`lint:yaml` -> `yamlfmt -lint .`)
  fails on pre-existing formatting drift in `.github/workflows/ci.yaml`, unrelated to any file
  this phase touches. Confirmed `.licenserc.yaml` (the only YAML file this phase edits) passes
  `yamlfmt -lint` cleanly in isolation. Out of scope per the executor's scope boundary (only
  auto-fix issues directly caused by the current task's changes).
