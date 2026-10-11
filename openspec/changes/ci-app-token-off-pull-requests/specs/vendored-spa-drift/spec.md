# Spec Delta

## Purpose

Keeps the committed console build under `internal/webauth/static/`, which the binary embeds, in step with the `ui/` source, and repairs a Renovate branch whose dependency bump left it stale.

## ADDED Requirements

### Requirement: Drift gate

For a pull request, the `ui vendored-asset drift` check SHALL rebuild the console from `ui/` and compare the result with the committed `internal/webauth/static/` tree. It SHALL pass when they match and fail when they differ.

#### Scenario: The committed build matches

- **WHEN** a pull request's rebuilt console matches the committed tree
- **THEN** the `ui vendored-asset drift` check passes

#### Scenario: The committed build is stale

- **WHEN** a pull request's rebuilt console differs from the committed tree
- **THEN** the `ui vendored-asset drift` check fails with the error `vendored SPA is stale — run 'task ui:build' and commit`

#### Scenario: A drifted Renovate commit that the self-heal will repair

- **WHEN** a same-repository Renovate pull request's rebuilt console differs from the committed tree
- **THEN** the check fails on that commit too, so a commit whose committed tree is stale never carries a passing drift check

### Requirement: Renovate drift self-heal

When the drift check fails for a pull request whose head branch starts with `renovate/`, lives in this repository, and was last pushed by `fzymgc-renovate[bot]`, CI SHALL push the rebuilt tree onto that branch as one commit, `fix(ui): regenerate vendored spa (renovate drift self-heal)`, authored by the CI bot App.

#### Scenario: A drifted Renovate branch is healed

- **WHEN** the drift check fails on a same-repository `renovate/` branch that `fzymgc-renovate[bot]` pushed
- **THEN** the branch gains the self-heal commit, and the new head commit gets a fresh run of every required check

#### Scenario: A drifted branch that is not Renovate's

- **WHEN** the drift check fails on any other branch, including a `jarvis/` branch or a fork
- **THEN** no commit is pushed, no credential is minted, and the check stays failed until the author commits a rebuild

#### Scenario: The branch moved on before the heal

- **WHEN** the branch head is no longer the commit that the failed check built
- **THEN** no commit is pushed, and the run for the newer head decides again

#### Scenario: The heal pushes a commit

- **WHEN** the self-heal commit lands on the branch
- **THEN** its own run of the drift check does not heal again, because `fzymgc-renovate[bot]` did not push it
