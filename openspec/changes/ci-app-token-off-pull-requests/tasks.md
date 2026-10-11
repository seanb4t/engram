# Tasks

## 1. Take the credential out of `ui-drift`

- [x] 1.1 In `.github/workflows/ci.yaml`, remove the `app-token`, `get-user-id`, and `self-heal vendored SPA on renovate PR` steps from the `ui-drift` job (design D4). Verify: `rg -n 'secrets\.' .github/workflows/ci.yaml` prints only the `secrets.GITHUB_TOKEN` line of `commit-lint`.
- [x] 1.2 Add an `actions/upload-artifact` step, pinned to the same digest as `docs-site.yaml`, that uploads `internal/webauth/static/` as `ui-drift-static` with `retention-days: 1` when `steps.drift.outputs.drifted == 'true'`, the head branch starts with `renovate/`, and the pull request is not from a fork (`!github.event.pull_request.head.repo.fork`, a form yamlfmt keeps on one line). Verify: `task lint:actions` is clean.
- [x] 1.3 Make the guidance step fail on every drift with the existing `vendored SPA is stale — run 'task ui:build' and commit` error, and add a `::notice::` on a `renovate/` branch saying that `ui-drift-heal` will push a rebuild. Replace the old comment block with a short one that points to `ui-drift-heal.yaml` and #812 (design D5). Verify: `task lint:actions` and `task lint:yaml` are clean, and `rg -n 'magic___' .github/workflows` prints nothing.

## 2. Add the `ui-drift-heal` workflow

- [x] 2.1 Create `.github/workflows/ui-drift-heal.yaml`: `on: workflow_run` for `workflows: [ci]`, `types: [completed]`; workflow-level `permissions: {}`; one job with `permissions: actions: read, contents: read`, `concurrency` keyed on `github.event.workflow_run.head_branch` with `cancel-in-progress: true`, and the job-level `if` from design D2 (event, `renovate/` prefix, not a fork, `fzymgc-renovate[bot]` actor). Verify: `task lint:actions` is clean.
- [x] 2.2 Add design D2 steps 1 to 3: look up `ui-drift-static` on `github.event.workflow_run.id` and end with success when it is absent; clone the head branch at depth 1 into `$RUNNER_TEMP` and end with success when its head is not `github.event.workflow_run.head_sha`; download the artifact with `actions/download-artifact` (pinned to the same digest as `docs-site.yaml`, `run-id` and `github-token` inputs). Pass every event value to `run:` scripts through `env:`, never by inline `${{ }}`. Verify: `task lint:actions` is clean; actionlint runs shellcheck on each `run:` script and flags an untrusted event value such as `workflow_run.head_branch` used inline.
- [x] 2.3 Add design D2 steps 4 and 5: mint the App token with the existing pinned `actions/create-github-app-token` and `permission-contents: 'write'`, gated on the earlier steps having found work; look up the bot user id; replace `internal/webauth/static/` in the clone with the artifact's files; commit only that path as `fix(ui): regenerate vendored spa (renovate drift self-heal)`; push `HEAD:refs/heads/<branch>` without force. Move the anti-recursion and bot-identity comments here from `ci.yaml` (design D5). Verify: `task lint:actions` and `task lint:yaml` are clean, `rg -n 'magic___' .github/workflows` prints nothing, and `rg -n -- '--force|-f ' .github/workflows/ui-drift-heal.yaml` prints nothing.

## 3. Verify the change as a whole

- [x] 3.1 Run the life agent's check on the branch's local workflow files: a one-off script in the session scratchpad (never committed) that imports `scripts/github_check.py` from `seanb4t/life-agent` and calls `check_workflows` on every `.github/workflows/*.yaml` with `default_branch="main"` and `default_write=False`. Verify: it returns an empty list. Before this change, the same script reports `ci.yaml: the job ui-drift can operate for pull_request and reads a secret`.
- [x] 3.2 Check that the specs hold on paper for the paths that cannot run before merge: walk the job-level `if` and step conditions of `ui-drift-heal.yaml` against each scenario in `specs/vendored-spa-drift/spec.md` and `specs/ci-pull-request-isolation/spec.md`, and record the result in the pull request description. Verify: every scenario names the condition that produces its outcome.
- [x] 3.3 Run `openspec validate ci-app-token-off-pull-requests --strict`. Verify: it reports the change as valid.
- [x] 3.4 Run the quality gate: `task` (lint and test). Verify: it exits 0.

## Workflow follow-up

- Open the pull request with `Fixes #812` and the scenario walk from 3.2.
- After merge, Sean runs `mise run github:check seanb4t/engram` in `seanb4t/life-agent`. Expect no workflow problem, and only the Renovate bypass note.
- On the next Renovate pull request whose drift check fails, confirm that `ui-drift-heal` pushes the self-heal commit as `engram-ci-app[bot]` and that the new head's `ui vendored-asset drift` check passes. On a drifted pull request from any other branch, confirm that no heal run gets past its job-level `if`.
- Archive the change after both observations.
