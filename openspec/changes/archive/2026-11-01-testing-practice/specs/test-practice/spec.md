# Spec Delta

## Purpose

Defines the repository's test tooling contract. It covers the scheduled deep run of fuzz and
invariant tests, with its failure rules and permissions. It also covers the fuzz and invariant
coverage that every pull request runs.

## ADDED Requirements

### Requirement: Scheduled deep run

A workflow SHALL run once a week and on manual dispatch from `main`. It SHALL run each Go fuzz
target for a fixed time. It SHALL run the Go invariant tests with more checks and more steps than
the per-PR suite. The UI invariant tests are not part of the deep run.

#### Scenario: The weekly run

- **WHEN** the weekly schedule fires
- **THEN** the workflow runs every Go fuzz target and every Go invariant test at the deep-run
  settings

#### Scenario: A manual run

- **WHEN** an operator dispatches the workflow from `main`
- **THEN** it runs the same stages as the weekly run

### Requirement: Deep-run failures

In the scheduled deep run, a fuzz target that finds a failing input, or a Go invariant test that
fails, SHALL fail the workflow run. The run SHALL upload each failing fuzz input as an artifact. A
failing invariant test SHALL report the seed that replays it.

#### Scenario: A fuzz target finds a failing input

- **WHEN** a fuzz target fails during the deep run
- **THEN** the workflow run fails, and the failing input is available as an artifact

#### Scenario: An invariant test fails

- **WHEN** an invariant test fails during the deep run
- **THEN** the workflow run fails, and the job log names the seed that reproduces the failure

#### Scenario: A clean run

- **WHEN** no fuzz target and no invariant test fails
- **THEN** the workflow run succeeds

### Requirement: Deep-run permissions

The scheduled deep-run workflow SHALL run with a read-only `GITHUB_TOKEN` and SHALL NOT name any
other secret.

#### Scenario: The workflow definition

- **WHEN** the deep-run workflow file is read
- **THEN** it grants only read permissions and references no secret other than `GITHUB_TOKEN`

### Requirement: Deep run stays off the per-PR path

`task`, `task test` and every workflow job that a `pull_request` event triggers SHALL NOT run the
deep-run stages. No required status check SHALL depend on the deep run.

#### Scenario: A pull request

- **WHEN** a pull request runs CI
- **THEN** no job runs a fuzz target past its seed corpus
- **AND** no job runs a Go invariant test at the deep-run settings
- **AND** the required checks are the same as before this change

### Requirement: Local deep run

`task test:deep` SHALL run the stages of the scheduled deep run on a developer machine, with the same
durations and check counts.

#### Scenario: A developer runs the deep stages

- **WHEN** a developer runs `task test:deep`
- **THEN** each Go fuzz target runs for the deep-run time
- **AND** the Go invariant tests run at the deep-run settings

### Requirement: Per-PR fuzz and invariant coverage

`go test ./...` SHALL run each fuzz target's seed corpus, which includes every input committed under
`testdata/fuzz/`. It SHALL also run each Go invariant test at its per-PR settings. `pnpm test` in
`ui/` SHALL run each UI invariant test at the library's default run count.

#### Scenario: A committed failing input

- **WHEN** a contributor commits an input that once failed a fuzz target under that target's
  `testdata/fuzz/` directory
- **THEN** `go test ./...` runs that input in every pull request

#### Scenario: A UI invariant test

- **WHEN** a pull request runs the `ui tests` job
- **THEN** `pnpm test` runs every UI invariant test at the library's default run count
