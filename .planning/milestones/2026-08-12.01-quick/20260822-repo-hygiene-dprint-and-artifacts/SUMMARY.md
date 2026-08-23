---
quick_id: 260822-im2
slug: repo-hygiene-dprint-and-artifacts
date: 2026-08-22
status: complete
commits:
  - faece501 chore(fmt) — dprint exclude + format drift
  - 2937d265 chore — gitignore .mcp.json
  - cef262d1 docs(phase-06) — track review iteration artifacts
---

# Summary: Repo hygiene — dprint drift + untracked artifacts

All three tasks complete. Working tree clean.

## What changed

**dprint** — added `internal/webauth/static` to `dprint.json` excludes and
formatted the three authored files (`ui/tsconfig.json`,
`docs-site/package.json`, `.claude/settings.json`). `task fmt:check` went
201 → 0.

**Artifacts** — committed the four phase-06 `iter2`/`iter3` review files,
matching phase 04's tracked precedent.

**`.mcp.json`** — gitignored. Verified with `git check-ignore -v`
(`.gitignore:68`).

## Evidence

The exclude was proved non-vacuous and not over-broad rather than assumed:
`dprint check --list-different` no longer names the vendored file while still
naming the other three (3 files, not 0). A gate that silently matched nothing
would have looked identical to success.

Exit status was read from a direct invocation, not through a pipe —
`task fmt:check | tail; echo $?` reports tail's status and prints 0 no matter
what the task did. That trap produced a false green earlier in this same
session's audit.

## Deviations from plan

None.

## Isolation note

`workflow.use_worktrees` was `true` at task start. Per memory `8vmgdqf83p`,
harness worktree isolation in this repo denies every external binary to an
isolated executor (`git`, `rg`, `ls` fail identically to complex commands),
which cost 243K subagent tokens to diagnose once already. Set to `false` for
this task, ran sequentially on the branch — the sanctioned degrade — and
restored to `true` afterward. `.planning/config.json` shows no diff.
