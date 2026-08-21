# Deferred Items — Phase 07

Out-of-scope discoveries logged during plan execution (SCOPE BOUNDARY rule): pre-existing
issues unrelated to the current task's changes, not auto-fixed.

## 07-01

- **`task lint` pre-existing staticcheck finding (SA1019), unrelated to this plan.**
  `internal/server/connectapi.go:268` (`Approximate: false,` inside the `ListMemoriesResponse{...}`
  literal in `ListMemories`) triggers `staticcheck SA1019` because
  `ListMemoriesResponse.Approximate` carries `[deprecated = true]` in the proto (added prior to
  this phase, unrelated to the `include_archived`/`include_superseded`/`include_scheduled` fields
  this plan adds). Confirmed present at the plan's base commit
  (`ed614e89039eb4f4105b843ec16e1387f2421209`) via `git show <base>:internal/server/connectapi.go`
  — this line and its content predate 07-01 entirely. Out of scope per the executor's SCOPE
  BOUNDARY rule (do not auto-fix pre-existing issues in unrelated code). `task lint:go` fails on
  this single finding; `task test` (the Go suite) is unaffected and fully green.

- **`task test` pre-existing failure: `TestNoEscapedPatternsRepoWide` (`internal/keylinks`),
  unrelated to this plan.** The gate scans every phase `*-PLAN.md`'s `key_links`/`acceptance_criteria`
  regex patterns for over-escaped shapes and flags 19 pre-existing findings across
  `07-01` through `07-07-PLAN.md` (e.g. `IncludeArchived:\\s+req\\.Msg\\.IncludeArchived`,
  `memoryStateCell\\(`). These patterns were authored at plan time, before this executor ran
  (`07-01-PLAN.md`'s last edit is `d1954db6`, an ancestor of this plan's base commit
  `ed614e89039eb4f4105b843ec16e1387f2421209`, confirmed via
  `git merge-base --is-ancestor`). Not this plan's code, and `.planning/**` is a tool-owned
  generated/authored artifact tree this executor must not hand-edit (see the repo's
  `planning-artifacts` convention). Out of scope per SCOPE BOUNDARY — logged, not fixed.
  `go test ./...` fails only on this one package; every other package (including
  `internal/store`, `internal/server`, `cmd/engram`) is green.
