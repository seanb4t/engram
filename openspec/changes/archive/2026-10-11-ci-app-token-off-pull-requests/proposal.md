# Proposal

## Why

The `ui-drift` job in `ci.yaml` runs on `pull_request`, builds the branch's `ui/` code, and then mints a CI bot App token from `CI_BOT_APP_CLIENT_ID` and `CI_BOT_APP_PRIVATE_KEY` to push the Renovate vendored-SPA self-heal (#812). The life agent's App (`seanb4t-jarvis`) can push `jarvis/` branches and open pull requests here, so its branch code runs in a job that holds a secret in reach. `mise run github:check seanb4t/engram` (in `seanb4t/life-agent`) fails on exactly this job, and the repository cannot join the App's installation until the check passes.

The fix the issue proposes, a step-level `if: github.event_name == 'push'`, does not work. The check reads only a job-level `if` and scans the whole job for secret references, so the job would still fail it. And a push to `main` has no pull request branch to heal, so the self-heal would become dead code. The self-heal still does real work: Renovate's own `postUpgradeTasks` rebuild covers most `ui/` bumps, but the grouped bump #784 (2026-10-09) still needed a self-heal commit.

## What Changes

- No job that a `pull_request` event triggers names a secret other than `GITHUB_TOKEN` or asks for a write permission. The `ui-drift` job loses its App token, bot identity, and push steps.
- On drift, `ui-drift` always fails with guidance. On a same-repository Renovate branch it also uploads the rebuilt `internal/webauth/static/` tree as a short-lived artifact. Before this change, a heal-eligible Renovate run pushed the rebuild and passed. Now the drifted commit shows red until the rebuild commit lands, and that commit gets a fresh run.
- A new workflow, `ui-drift-heal.yaml`, runs on `workflow_run` after `ci` completes. It runs from the default branch's definition and executes no branch code. When the triggering run was a Renovate pull request on a same-repository `renovate/` branch, uploaded the artifact, and the branch head is still the commit that was built, it mints the App token and pushes the artifact's tree as the existing `fix(ui): regenerate vendored spa (renovate drift self-heal)` commit, under the same bot identity.
- The `ui vendored-asset drift` check name, the self-heal commit message, the bot identity, and the Renovate `gitIgnoredAuthors` entry stay the same.

## Capabilities

### New Capabilities

- `ci-pull-request-isolation`: the trust boundary of the repository's CI. A job that a pull request triggers gets no secret beyond `GITHUB_TOKEN` and no write permission, and privileged work driven by a pull request runs only in a workflow that executes no branch code.
- `vendored-spa-drift`: the CI gate that keeps the committed `internal/webauth/static/` tree in step with `ui/`, and the Renovate self-heal that repairs a drifted Renovate branch.

### Modified Capabilities

None. No existing spec covers CI.

## Impact

- **Code:** `.github/workflows/ci.yaml` (the `ui-drift` job) and a new `.github/workflows/ui-drift-heal.yaml`. No Go, TypeScript, or Taskfile change.
- **Secrets:** `CI_BOT_APP_CLIENT_ID` and `CI_BOT_APP_PRIVATE_KEY` stay as they are and move to the new workflow. No App permission changes.
- **Wire contract:** deliberately unchanged. No proto field, MCP input schema, error hint code, `ENGRAM_` config key, or stored payload key changes.
- **MCP tool schemas:** deliberately unchanged.
- **Connect proto:** deliberately unchanged.
- **CLI:** deliberately unchanged.
- **Console (`ui/`):** deliberately unchanged. The vendored tree is rebuilt the same way; only where the push happens moves.
- **Helm chart:** deliberately unchanged.
- **docs-site:** deliberately unchanged. No guide or reference page describes the self-heal; the workflow comments are its documentation.
- **engram skill and CLAUDE.md memory contract:** deliberately unchanged.
- **Renovate config:** deliberately unchanged. `postUpgradeTasks` and `gitIgnoredAuthors` keep working as they do.
- **Out of scope:** the `protect-main` bypass for Renovate (integration 3982691), which the check reports only as a note; any change to the life agent's App, its check script, or its installation; the `release.yaml` and `verify-tap-credential.yaml` App tokens, which run on `push` to `main`, `release`, or `workflow_dispatch` and pass the check today; and moving the self-heal into Renovate's `postUpgradeTasks` alone.
