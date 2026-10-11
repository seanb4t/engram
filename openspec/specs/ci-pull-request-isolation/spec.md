# ci-pull-request-isolation Specification

## Purpose

Keeps credentials and write access away from branch code in the repository's CI, so a pull request from any branch, including one that an automated agent opened, cannot reach a secret.

## Requirements

### Requirement: Pull request jobs get no secret

A workflow job that a `pull_request` or `pull_request_target` event can trigger SHALL NOT name any secret other than `GITHUB_TOKEN`, and SHALL NOT pass the workflow's secrets on to anything it calls.

#### Scenario: A pull request runs CI

- **WHEN** a pull request from any same-repository branch triggers a workflow
- **THEN** none of the jobs that run hold a secret other than `GITHUB_TOKEN`

#### Scenario: The life agent's repository check

- **WHEN** `mise run github:check seanb4t/engram` runs in `seanb4t/life-agent`
- **THEN** it reports no workflow problem, and at most notes

### Requirement: Pull request jobs get no write permission

A workflow job that a `pull_request` or `pull_request_target` event can trigger SHALL run with a read-only `GITHUB_TOKEN`, whether by the repository default or by its own `permissions`.

#### Scenario: A pull request job asks for write access

- **WHEN** a pull request triggers a workflow
- **THEN** no job that runs has a `GITHUB_TOKEN` with any write permission

### Requirement: Privileged follow-up runs no branch code

Work that a pull request triggers and that needs a secret or a write permission SHALL run in a workflow that a `workflow_run` event triggers, from the default branch's definition. It SHALL treat what the pull request's run produced as data and SHALL NOT execute it, build it, or install from it.

#### Scenario: A follow-up consumes a pull request's output

- **WHEN** a privileged follow-up workflow reads an artifact that a pull request's run uploaded
- **THEN** it only copies or commits the artifact's files, and it runs no script, package install, or build from the pull request's branch

#### Scenario: A follow-up is gated on its trigger

- **WHEN** a privileged follow-up workflow starts for a run whose branch, actor, or repository is not the one it serves
- **THEN** it mints no credential and changes nothing
