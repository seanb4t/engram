# Design

## Context

See proposal.md (Why) for the motivation. The current state:

- `ci.yaml` runs on `pull_request` and on `push` to `main`, with workflow-level `permissions: contents: read`. The repository default for `GITHUB_TOKEN` is `read`.
- The `ui-drift` job checks out the merge ref, runs `pnpm install --frozen-lockfile && pnpm build` in `ui/`, and copies the build over `internal/webauth/static/`. On drift it mints an App token (`actions/create-github-app-token`, `CI_BOT_APP_*` secrets) when the run is a same-repository `renovate/` pull request with `github.actor == 'fzymgc-renovate[bot]'`, looks up the bot's user id, clones the head branch, copies the build in, and pushes the self-heal commit. Without a token it fails with guidance.
- An App token, not `GITHUB_TOKEN`, pushes the heal, because a `GITHUB_TOKEN` push never triggers `pull_request: synchronize`, and the pushed commit would wait on its required checks forever.
- The life agent's check (`scripts/github_check.py` in `seanb4t/life-agent`) treats a job as triggered by a pull request unless the job-level `if` names other events with `github.event_name ==` and has no `||` and no `!`. It flags such a job when its YAML mentions `secrets` other than `secrets.GITHUB_TOKEN` anywhere, or when it asks for a write permission. It ignores `workflow_run`, and it ignores a `push` trigger limited to the default branch. It reads the workflows of the default branch, not of a pull request.
- Measured 2026-10-11 by running that check's `check_repository` against `seanb4t/engram` on `main`. The only workflow problem is `ci.yaml: the job ui-drift can operate for pull_request and reads a secret`, and the only other line is the Renovate bypass note.
- Measured 2026-10-11: of the 13 Renovate pull requests that touched `ui/package.json` since 2026-10-04, one (#784, a grouped non-major bump) carried a self-heal commit. Renovate's `postUpgradeTasks` rebuild covered the rest. Why #784 still drifted was not investigated. It does not change this design.
- Successful `ci` runs on pull requests take 0 to 4 minutes (the last 15).

## Goals / Non-Goals

**Goals:**

- No secret and no write permission in any job that a pull request triggers, by the rule the life agent's check applies, and also by its intent: branch code never shares a runner with a credential.
- Keep the Renovate self-heal working, with the same commit, identity, and re-trigger behavior.

**Non-Goals:**

- Hardening against a branch that edits `.github/workflows/`. A pull request's run uses the branch's workflow definition. The life agent's App has no `workflows` permission, so GitHub refuses that push. Renovate's own trust is unchanged by this change.
- Changing which Renovate bumps drift, or the `postUpgradeTasks` rebuild.

## Decisions

### D1: Build in the pull request run, push from a `workflow_run` workflow

The pull request run stays unprivileged and produces the rebuilt tree as an artifact. A new workflow, `ui-drift-heal.yaml`, triggered by `workflow_run` on `ci` (`types: [completed]`), runs from the default branch's definition, reads the artifact as data, and pushes it. This is the split that GitHub Security Lab recommends for pull request work that needs a credential: `pull_request` for the untrusted build, `workflow_run` for the privileged step.

Alternatives considered:

- **Step-level `if: github.event_name == 'push'` on the token steps (the issue's proposal).** Rejected. The check reads only the job-level `if` and scans the whole job for secret references, so the job still fails it. A push to `main` also has no pull request branch to heal, so the self-heal becomes dead code.
- **A separate `ui-drift-heal` job in `ci.yaml`, `needs: ui-drift`, with the same artifact handoff.** Rejected. It is still a job that a pull request triggers and that reads a secret, so the check fails, and its definition still comes from the branch.
- **`pull_request_target`.** Rejected. It has no clean way to wait for the pull request run's artifact, and checking out or building branch code under it is the classic way to leak a secret. The check also flags it once a step checks out the head.
- **A `push` trigger on `renovate/**`.** Rejected. The check counts a push trigger that is not limited to the default branch, and the job would build branch code with the token in reach.
- **Delete the self-heal and rely on `postUpgradeTasks`.** Rejected for now. #784 needed the heal two days ago. If the heal stops firing for a long period, deleting it is the simpler follow-up.

### D2: The heal workflow's gate and order

The job-level `if` requires all of: `workflow_run.event == 'pull_request'`, `startsWith(workflow_run.head_branch, 'renovate/')`, `!workflow_run.head_repository.fork` (the same-repository test, in a form yamlfmt keeps on one line), and `workflow_run.actor.login == 'fzymgc-renovate[bot]'`. The self-heal commit's own run has the CI App as its actor, so it never heals again. The life agent's App cannot re-run a workflow, because it has no `actions: write`.

The steps run in this order, so the App token exists only when a push will happen:

1. Look up the artifact `ui-drift-static` on the triggering run with `GITHUB_TOKEN` (`actions: read`). If it is absent, the job ends with success and does nothing else.
2. Clone the head branch at depth 1. If its head is not `workflow_run.head_sha`, end with success: a newer push has its own run.
3. Download the artifact (`actions/download-artifact`, `run-id` and `github-token` inputs) into `$RUNNER_TEMP`.
4. Mint the App token (`permission-contents: write`), look up the bot's user id, and set the bot's git identity, as today.
5. Replace `internal/webauth/static/` in the clone with the artifact's files, commit only that path, and push `HEAD:refs/heads/<branch>` without force. If the branch moved after step 2, GitHub rejects the push as non-fast-forward. That leaves no partial state, and the job goes red so the failure is visible.

Workflow-level `permissions: {}`. The job asks for `actions: read` and `contents: read` only. `concurrency` is keyed on the head branch with `cancel-in-progress: true`, so only the newest heal for a branch runs.

### D3: The artifact is data

The pull request run's definition comes from the branch, so the artifact holds whatever the branch produced. The heal workflow never executes, installs, or builds anything from it or from the clone. It only copies files into `internal/webauth/static/` and commits that path. What lands on the Renovate branch still goes through the pull request's required checks and review before it reaches `main`, the same as today's in-job heal.

### D4: `ui-drift` keeps one result per commit

The job loses the App token, user id, and heal steps, and so names no secret. On drift it uploads the tree with `actions/upload-artifact` (name `ui-drift-static`, `retention-days: 1`) when the head branch starts with `renovate/` and the pull request is from this repository. This is only a pre-filter to avoid storing artifacts that nothing reads. D2's gate is the one that decides. It then fails on any drift with the existing error text, plus a notice on Renovate branches that the heal workflow will push a rebuild.

Alternative considered: pass the drift check when the heal is expected. Rejected. If the heal then fails, a commit with a stale tree carries a passing required check, and automerge can merge it.

### D5: Comments move with the code

The comment block that explains the App token over `GITHUB_TOKEN` (the anti-recursion guard) and the bot identity moves to `ui-drift-heal.yaml`. `ui-drift` gets a short comment that points to it and states why the job holds no secret (#812).

## Risks / Trade-offs

- [`workflow_run` runs only workflow files that are on the default branch, so the heal cannot run on the pull request that adds it] → Before merge: `actionlint`, and the life agent's check run against the branch's local workflow files. After merge: the life agent's real check, and observation of the next Renovate pull request that drifts. Until that observation, a drifted Renovate pull request at worst fails with guidance, which is what happens today when the App token is missing.
- [The heal waits for the whole `ci` run, not only `ui-drift`] → 0 to 4 minutes, measured above. Renovate pull requests do not need faster.
- [The drifted commit now shows a failed required check before the heal] → Intended (D4). Renovate's automerge waits for the healed head. The heal commit keeps the identity in `gitIgnoredAuthors`, so Renovate does not mark the pull request as edited.
- [The `ci` run is cancelled or fails before `ui-drift` uploads] → No artifact, so the heal does nothing. The next push runs again.

## Migration Plan

Merge the change. No secret, App permission, or Renovate setting changes. After merge, Sean runs `mise run github:check seanb4t/engram` in `seanb4t/life-agent`, then adds the repository to the App's installation. To roll back, revert the pull request. The in-job heal comes back, and the check fails again.
