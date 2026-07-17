---
quick_id: 260717-g1r
status: complete
issue: 301
branch: fix/301-renovate-ui-vendor-shell
commit: 1462da20
created: 2026-07-17
---

# Quick Task 260717-g1r: Fix #301 — Summary

## Outcome

Fixed the in-repo root cause of #301 (`spawn cd ENOENT`) in `.github/renovate.json`. Committed
`1462da20` on `fix/301-renovate-ui-vendor-shell` (off `origin/main`). **Not merged** — gated on a
cross-repo allowlist update (below).

## Triage

#301 was retitled/updated 2026-07-17. The external homelab blockers (allowlist + node/pnpm) are
now cleared, exposing an in-repo command-shape bug: Renovate runs `postUpgradeTasks` shell-free
(shlex-split, spawns `argv[0]`), so the `cd ui && …` pipeline fails `spawn cd ENOENT` on every
`ui/` bump (#383/#384/…), surfacing as a Repository Problem on the Dependency Dashboard (#155).
Not an observation-gated item any more — a real, reproduced bug.

## What changed

`.github/renovate.json` — the `ui/package.json` `postUpgradeTasks` rule:
- **command** → wrapped in `bash -c '…'` (was a bare `cd ui && …` pipeline). `bash -c '…'` gives
  Renovate one spawn-able binary (`bash`, present at `/usr/bin/bash`) that runs the whole `&&`
  chain in a shell — `cd` works, and the build→destroy interlock (`pnpm build && … rm -rf`)
  short-circuits on a failed build. `task ui:build` was rejected as an option: `go-task` is absent
  from the runner and not installable via containerbase.
- **`installTools: {"pnpm": {}}`** added — installs pnpm at the repo's `packageManager` major
  (runner ships an older pnpm) for byte-reproducible output. Schema requires `{}` here; a version
  string is rejected — version resolves from `packageManager: pnpm@11.11.0`.
- **description** refreshed (mechanism + the cross-repo coupling warning).

Verified: JSON parses; `installTools` is a valid nested `postUpgradeTasks` key and `pnpm:{}`
matches renovate-schema; `executionMode: branch` unchanged.

## ⚠ BLOCKING — do this BEFORE merging (user, cross-repo)

The self-hosted `fzymgc-house/selfhosted-cluster` `allowedCommands` allowlist pins the command by
**anchored exact match**. Update it to the NEW string and land it **cluster-first**, or the `ui/`
bump PRs get rejected as "not in allowedCommands" in the window between.

Exact new command string to allowlist (`^…$`):

    bash -c 'cd ui && pnpm install --frozen-lockfile && pnpm build && rm -rf ../internal/webauth/static && mkdir -p ../internal/webauth/static && cp -R build/. ../internal/webauth/static/'

## Verify on the runner (recommended)

- `installTools: {"pnpm": {}}` resolves to pnpm 11.x from `packageManager` (runner default is 10.x).

## Follow-up

- After the cluster-first allowlist update + this merges: a live `ui/` bump should self-heal green
  → closes the #369 observation, then #301.
- #301 labeled `confirmed-bug` during triage.
