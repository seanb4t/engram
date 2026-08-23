# Deferred Items

Out-of-scope discoveries logged during plan execution, per the executor's scope-boundary rule
(only auto-fix issues directly caused by the current task's changes).

## From 05-04 (G-05-9 gap closure)

- **Status:** acknowledged

- **`task fmt:check` (dprint) pre-existing drift, unrelated to this plan's diff.** Running
  `task fmt:check` during 05-04's task-3 verification surfaced 4 already-unformatted files, none
  touched by this plan's commits (confirmed via `git status --short` on each — zero output, i.e.
  no working-tree changes to any of them):
  - `.claude/settings.json`
  - `docs-site/package.json`
  - `internal/webauth/static/_app/version.json`
  - `ui/tsconfig.json`

  These predate this plan's execution and are out of its file scope
  (`internal/e2e/*, go.mod, go.sum, .github/workflows/ci.yaml, Taskfile.yaml`). Not fixed here;
  flagging for a future cleanup pass (`dprint fmt .`).
