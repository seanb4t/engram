---
title: Testing
description: The kinds of test engram uses, how test names select a stage, what runs on each trigger, and how to replay a failure.
---

This page is the test plan for engram. It names the test kinds, the names that select each stage, and what runs on each trigger. The last section is the gap review: one table per area, with the test kind that closes each gap.

## Test kinds

engram uses three kinds of test.

- A scenario test implements one OpenSpec `#### Scenario:` about behaviour that engram owns. The scenario title is the Go `t.Run` subtest name or the Vitest `it` title, word for word. To find the test for a scenario, search for its title with `rg`.
- An invariant test checks a rule that holds for every input or after any sequence of operations. Go invariant tests use [rapid](https://github.com/flyingmutant/rapid). The console uses [fast-check](https://fast-check.dev/). A failure shrinks to a small counterexample and prints the seed that replays it.
- A fuzz target is a Go `testing.F` target for a parser or a sanitizer. Each committed input under the package's `testdata/fuzz/` runs as a regular test.

A scenario about tooling or configuration gets no test. Examples are the permissions of a workflow, or Go running a committed fuzz input. The task for such a scenario names a command or a review step, and the pull request records the result.

Two kinds of test are not used. engram does not test the behaviour of a dependency, and it has no machinery that tests the test suite itself.

## Names select the stage

The test name prefix tells you what the test needs and which stage runs it.

| Prefix | What it is | Needs Qdrant |
| --- | --- | --- |
| `TestProperty` | A pure property over generated input | No |
| `TestModel` | A property or a rapid state machine against the real store | Yes |
| `Fuzz` | A Go fuzz target | No |

Every `TestModel` test uses its own collection on the shared CI Qdrant. Issue [#646](https://github.com/seanb4t/engram/issues/646) tracks the Qdrant testcontainer dying under heavy host load. A `TestModel` test compares the real store or server with a small in-memory model of the contract. A disagreement between the model and the code is a finding. Resolve it against the memory contract in `CLAUDE.md`. Never change the model only to match the code.

Each new hint code needs a generator before the lane tests can draw it. `TestModelLaneParity` and `TestModelWriteBounds` in `internal/server` hold the generators.

## What runs when

| Trigger | What runs |
| --- | --- |
| Every pull request and every push to `main` | `go test ./...` with each rapid test at its default count and each fuzz target on its committed inputs. `pnpm test` in `ui/` with each fast-check property at its default count. |
| Weekly, and on demand | The `deep-test` workflow. It runs `task test:deep:fuzz`, which fuzzes every target in turn, and `task test:deep:invariants`, which runs the `TestProperty` and `TestModel` tests at high counts. |
| On demand, on your machine | `task test:deep` runs both deep stages. It needs Docker or `ENGRAM_QDRANT_TEST_ADDR` for Qdrant. |

To see the counts and the fuzz time that the deep stages use, run `task --summary test:deep:fuzz test:deep:invariants`.

To start the deep run on GitHub, run this command:

```bash
gh workflow run deep-test.yaml
```

## Replay a failure

A failing rapid test prints a line like this:

```text
To reproduce, specify -run="TestModelStore" -rapid.failfile="testdata/rapid/TestModelStore/TestModelStore-20261031205616-45038.fail" (or -rapid.seed=7038422364071888785)
```

To replay it, run the same test with that seed:

```bash
go test ./internal/store -run '^TestModelStore$' -rapid.seed=7038422364071888785 -v
```

rapid also writes a failure file under `testdata/rapid/`. Git ignores that directory, so do not commit it. The seed replays the failure.

A failing fuzz run writes its input to the package's `testdata/fuzz/<target>/` directory and prints the command that replays it:

```bash
go test ./cmd/engram -run 'FuzzSanitizeViewValue/7a7d28678c0b0b92'
```

To keep the input, commit the file beside the fix. From then on it runs in every pull request. When the weekly `deep-test` run finds a failure, the `fuzz-failures` artifact of that run holds the new input files at their repository paths.

## Gap review

The gap review lists, for each area, behaviour that no test covers and the test kind that closes it. A row with an issue link is open work. A row that names a test is closed, and the test is named.

### internal/store

| Gap | Test kind | Status |
| --- | --- | --- |
| Isolation, supersession, derived states and pagination hold after any sequence of writes | model test | Closed by `TestModelStore` |
| A list cursor survives a decode and a round trip for any input | fuzz target | Closed by `FuzzDecodeCursor` and `TestPropertyCursorRoundTrip` |
| Every search result stays in the caller's visible set after any sequence of writes | model test | ISSUE |
| Update and prune-expired keep the state words and the pagination correct after any sequence | model test | ISSUE |
| Related memories and tag counts stay in the caller's visible set after share, archive and supersede | model test | ISSUE |

### MCP lane

| Gap | Test kind | Status |
| --- | --- | --- |
| Write size bounds give the same envelope on both lanes | model test | Closed by `TestModelWriteBounds` |
| A replayed store or supersede returns the same record and writes nothing | model test | Closed by `TestModelIdempotency` |
| `recall_gate_hidden` on `search_memory` matches the recall gate | model test | ISSUE |
| A replayed `schedule_memory` with the same `idempotency_key` writes nothing | model test | ISSUE |
| `delete_all` never removes a record that another caller owns | model test | ISSUE |

### Connect lane

| Gap | Test kind | Status |
| --- | --- | --- |
| MCP and Connect give the same outcome, field and hint for store, schedule, supersede, update and list | model test | Closed by `TestModelLaneParity` |
| A Connect window bound fails with `hint=format` or parses to an equal instant | fuzz target | Closed by `FuzzParseConnectWindowBound` |
| The wire negative matrix covers archive, restore, supersede, the schedule window order and nested citations | scenario test | [#357](https://github.com/seanb4t/engram/issues/357) |
| Lane parity runs through the validation and auth interceptors that real Connect callers pass | model test | ISSUE |
| Generated lane parity covers archive, restore, delete, set visibility and search | model test | ISSUE |

### CLI

| Gap | Test kind | Status |
| --- | --- | --- |
| Operator text output holds no control character for any input | fuzz target | Closed by `FuzzSanitizeViewValue` |
| State words and hint exit codes hold for generated records and hints | property test | Closed by `TestPropertyDerivedState` and `TestPropertyHintExitCode` |
| A stored record goes through `memoryToProto` to the rendered CLI document | scenario test | [#499](https://github.com/seanb4t/engram/issues/499) |
| The terminal console keeps a valid selection and overlay state for any key sequence | model test | ISSUE |

### internal/migrate

| Gap | Test kind | Status |
| --- | --- | --- |
| Every registered step is additive for any version-0 payload | property test | Closed by `TestPropertyStepAdditive` |
| The sweep reaches the current version once and changes nothing on a second run | model test | Closed by `TestModelMigrateSweep` |
| Applying the registry to its own output changes nothing, and never changes the input map | property test | ISSUE |
| `Validate` and `StepsFrom` accept and select only well-formed chains, for generated step lists | property test | ISSUE |

### Auth and authz

| Gap | Test kind | Status |
| --- | --- | --- |
| `DecideRecord` matches the isolation model for every principal, record and action | property test | Closed by `TestPropertyAuthz` |
| `DecideBucket` matches the isolation model | property test | ISSUE |
| The namespaced owner is injective for generated issuer and subject pairs | property test | ISSUE |
| Session unsealing and bearer extraction never panic and accept only well-formed values | fuzz target | ISSUE |
| A CSRF token verifies only for the owner it was minted for | property test | ISSUE |

### internal/e2e

| Gap | Test kind | Status |
| --- | --- | --- |
| `engram migrate` status, apply and status run against the full stack | e2e test | [#652](https://github.com/seanb4t/engram/issues/652) |
| Console write flows log in through a token-issuing OIDC provider | e2e test | [#366](https://github.com/seanb4t/engram/issues/366) |
| The console restores a draft from the re-auth resume envelope after the session expires mid-write | e2e test | ISSUE |

### ui/

| Gap | Test kind | Status |
| --- | --- | --- |
| `classifyInput` returns the right outcome for generated input | property test | Closed by `classify.property.test.ts` |
| The search, related, rules and scheduled URL parameters survive an encode and a parse | property test | ISSUE |
| A resume destination is never an off-origin or unknown route | property test | ISSUE |
| The chain dialog and supersede footer browser tests pass on every run | browser test | [#851](https://github.com/seanb4t/engram/issues/851) |
| On a feature branch, an embedded console older than the `ui/` sources fails the check | scenario test | [#723](https://github.com/seanb4t/engram/issues/723) |

### Skill hooks

| Gap | Test kind | Status |
| --- | --- | --- |
| Go `NormalizeRemote` gives `host/owner/repo` for generated remotes | property test | Closed by `TestPropertyNormalizeRemote` |
| The Python scope derivation matches Go for generated remotes and worktree names | property test | ISSUE |
| Two hooks for one session show the capture nudge once | scenario test | ISSUE |
| The session-start hook works with a built `engram prime` against a live server | e2e test | ISSUE |

### Helm chart

| Gap | Test kind | Status |
| --- | --- | --- |
| `expose.mode` ingress and httproute render, and a missing host, missing parentRefs or unknown mode fails | chart test | ISSUE |
| The OIDC and console values render their `ENGRAM_` variables, with the client secret as a secret reference | chart test | ISSUE |
| Every `ENGRAM_` variable that the chart renders is a key the binary knows | chart test | ISSUE |
| The Qdrant S3 snapshot values render | chart test | ISSUE |
