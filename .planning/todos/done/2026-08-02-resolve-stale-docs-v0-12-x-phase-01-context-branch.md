---
created: 2026-08-03T00:40:41.278Z
title: Resolve stale docs/v0.12.x-phase-01-context branch
area: planning
severity: minor
files:
  - .planning/milestones/v0.12.x-phases/01-shared-auth-chain-connect-bearer-identity/01-CONTEXT.md
  - .planning/milestones/v0.12.x-phases/01-shared-auth-chain-connect-bearer-identity/01-RESEARCH.md
---

## Problem

A local-only branch `docs/v0.12.x-phase-01-context` survived the v0.12.x milestone close. It
carries **3 commits that are not in `main`**:

```
7e762662 docs(01): research v0.12.x phase 1 auth chain and bearer identity
018e91a4 docs(state): record v0.12.x phase 1 context session
71ccc42c docs(01): capture v0.12.x phase 1 context
```

These predate the work moving onto the `phase-01-shared-auth-chain-connect-bearer-identity`
branch that became PR #464. It has no remote counterpart — it was never pushed — so it is
invisible to anyone else and will silently rot in this clone.

The equivalent *content* is in `main`, archived at
`.planning/milestones/v0.12.x-phases/01-shared-auth-chain-connect-bearer-identity/`
(`01-CONTEXT.md`, `01-RESEARCH.md`, and the rest of the phase directory). But that has **not been
verified line-for-line** against these three commits — the branch tip diffs against `main` at 258
files, which is just accumulated drift from ~250 subsequent commits and tells us nothing about
whether anything unique is stranded here.

Low stakes: worst case a superseded early draft of a phase-1 planning doc is lost, and the phase
itself shipped and verified. But deleting unverified is exactly the move that makes a future
"where did that research note go?" unanswerable, so it was left in place at close.

## Solution

Verify, then delete. Do not delete first.

1. For each of the three commits, diff its planning-file content against the archived versions:
   ```bash
   git show 7e762662 --stat
   git diff 7e762662 main -- .planning/milestones/v0.12.x-phases/01-*/
   ```
   Note that paths moved during the milestone close (`.planning/phases/01-*` →
   `.planning/milestones/v0.12.x-phases/01-*`), so a naive path-matched diff will show everything
   as missing. Compare by content, not by path.
2. If nothing unique: `git branch -D docs/v0.12.x-phase-01-context`.
3. If something unique and worth keeping: it belongs in the archived phase directory on a small
   PR, not resurrected as a branch.

While in there, check for other stranded local branches from earlier milestones — this one only
surfaced because it appeared next to `main` in `git branch -a` during the post-merge cleanup, and
nothing routinely audits for them.
