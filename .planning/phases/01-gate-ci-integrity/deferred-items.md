# Deferred Items

Out-of-scope discoveries logged during phase execution per the executor's SCOPE BOUNDARY rule
(not fixed — pre-existing, unrelated to the discovering plan's task).

## Plan 01-05

- **`task lint` fails on `rumdl` MD041 for `internal/keylinks/testdata/{bad,good}_key_links.md`.**
  Both fixture files intentionally start with GSD-style YAML frontmatter (`---`) rather than an H1
  heading, since they are keylinks-parser test fixtures shaped like real PLAN.md files. Confirmed
  pre-existing at base commit `408f7db4` (introduced by plan 01-01, `18ce6e14`), unrelated to any
  file plan 01-05 touched. `golangci-lint` itself reports "0 issues." — only the markdown linter
  fails, and only on this pre-existing pair. Not fixed here (out of scope); `task lint`'s go/vet/
  gofmt-relevant portions for this plan's own files are clean.
