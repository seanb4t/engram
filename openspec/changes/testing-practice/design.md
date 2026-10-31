# Design

## Context

See proposal.md (Why) for the motivation and specs/test-practice/spec.md for the tooling contract.

The facts that shape the approach:

- `go test ./...` runs every package in one CI job, `test`, beside a shared Qdrant service. On the
  2026-10-31 `main` run, the slowest packages were `internal/store` (117 s), `internal/e2e` (46 s
  over two binaries), `internal/surfaces` (45 s) and `internal/server` (29 s).
- Store tests get an isolated collection through `newTestStore` (`internal/store/store_test.go`)
  on the shared Qdrant that `storetest` dials.
- Lane parity is tested today with fixed tables: `TestReadParity`
  (`internal/server/connectapi_parity_test.go`) and `TestWriteParity`
  (`internal/server/connectapi_write_parity_test.go`).
- The hint vocabulary is the `HintCode` constants in `internal/server/argerror.go`, and
  `renderHintEnvelope` writes the `field=<name> hint=<code>` envelope.
- The migration registry (`internal/migrate`) already has `CheckAdditive`, `Validate` and
  `StepsFrom`. The default authorization policy loads through `authz.MustDefault`.
- `ui/vite.config.ts` defines two Vitest projects. A `node` project runs `src/**/*.test.ts`, and a
  `browser` project runs `src/**/*.browser.test.ts` under Playwright.

## Goals / Non-Goals

**Goals:**

- One practice that every later change follows without a new decision.
- Invariant tests that find real defects in code we own, not demonstrations.
- A per-PR cost that the design states and the PR measures.

**Non-Goals:**

- Mutation testing, which #924 builds on top of this change.
- The CI waste fixes, including the 63 s "assert shared-Qdrant participation per package" step.
  The separate `ci:` pull requests for #923 decide those.
- A tool that checks the mapping from spec scenarios to tests. Such a tool is a gate whose subject
  is the suite, and rule `3p0zsqrhmb` forbids it.

## Decisions

### D1. Scenario tests are the BDD form, without Gherkin

A scenario test implements one OpenSpec `#### Scenario:`. In Go, the scenario title is the
`t.Run` subtest name, verbatim. In `ui/`, it is the `it(...)` title. Its body follows the scenario's
WHEN and THEN lines, in their order. A reviewer finds the test for a scenario by its title, with `rg`.

Rejected: godog with `.feature` files. Each scenario then exists twice, once in the spec and once in
Gherkin. The step-definition layer is also more code to maintain. Rejected: a comment that cites the
scenario. House rules keep plan and spec references out of code comments, and a title is a
reference that `go test -run` can select.

### D2. rapid for Go invariants, fast-check for UI invariants

`pgregory.net/rapid` generates inputs and shrinks a failing case to a minimal one. It has a
model-based state machine, `T.Repeat`, with an invariant checker under the `""` key. It reads
`-rapid.checks` and `-rapid.steps`, or the `RAPID_CHECKS` and `RAPID_STEPS` environment variables.
A failure prints the seed, and `-rapid.seed` replays it. Its last push was 2026-09-04. `fast-check`
is the maintained TypeScript equivalent (last push 2026-10-31).

Rejected: `testing/quick`. It is frozen and does not shrink. Rejected: gopter. It is less active
than rapid and has no state-machine API that matches `T.Repeat`.

### D3. A model-based state machine for the store

Each run opens a fresh collection through `newTestStore` and holds a small in-memory model: a map of
records with owner, visibility, `superseded_by`, `archived_at` and the validity window. Two
principals, an owner and another authenticated caller, act on the store. The actions are store,
schedule, supersede, archive, restore, delete, set visibility and list (each cursor page). After
every action the invariant checker compares the store with the model:

- Isolation: neither principal reads the other's private records or changes any record it does not
  own.
- Supersession: each record has at most one successor, each chain has one live head, no chain has a
  cycle, and a superseded record is missing from recall but returned by id.
- Derived states: each record's state words match the model, in canonical order, with expired
  suppressing scheduled. Archive and restore never write `superseded_by` or `not_after`.
- Pagination: walking every cursor page returns the model's visible set with no duplicates and no
  gaps.
- `recall_gate_hidden`: the counts match the records the model hides.

Search is not in the action set, because the model cannot predict vector ranking. A search
invariant is stated as membership only: every search result is in the caller's visible set.

The test uses an injected clock, so the window states change without wall-clock waits.

Rejected: a model-only test of a fake store. It tests the fake, not the code that runs in
production.

### D4. Lane parity and the error envelope as properties

A rapid property draws an argument set for one verb from generators that cover each field's valid
values and its invalid values: missing, too long, too many, wrong format, out of order and mutually
exclusive. It sends the same arguments to the MCP handler and to the Connect handler, each on its own
store. It asserts four things:

- Both lanes accept, or both reject.
- On a rejection, both name the same `field` and the same `hint`.
- The `hint` is one of the `HintCode` constants.
- The message has the `field=<name> hint=<code>` envelope.

The pilot covers the store, schedule, supersede, update and list verbs. The CLI lane gets one
property in `cmd/engram`: every `HintCode` maps to one exit code and to stderr text that names the
same `field` and `hint`.

The property extends `TestReadParity` and `TestWriteParity` and does not replace them. Their fixed
rows document the known cases.

### D5. Bounds on every write path

A rapid property draws content, summary and tag sets with sizes around each limit: below it, at it,
one byte or one entry over it, and far over it. It runs them through each write verb on both lanes:
store, schedule, supersede, update, store rule and store discovery. A write within every bound
succeeds. A write over a bound fails with that field and `too_long` or `too_many`, and the store
holds no record from it. The limits come from the test's configuration, never from literals.

### D6. Idempotency replay

A rapid property draws a store or supersede call and an `idempotency_key`, then sends it two to five
times. Every reply carries the first reply's record id, and the store holds one record. A second
call with the same key and different content fails with the documented conflict, and the store is
unchanged. The property runs in the server package against the real store.

### D7. Migration properties

A rapid property draws a payload that a version-0 record can hold. It asserts three things:

- Every registered step passes `CheckAdditive` on that payload.
- Applying the full sweep a second time changes nothing.
- `schema_version` never decreases.

These are pure tests over maps and need no Qdrant.

### D8. Authorization against a model

A rapid property draws three values:

- a principal: anonymous, the owner, or another authenticated caller
- a record: owner present or empty, private or shared
- an action: read or write

It asserts that the
decision from `authz.MustDefault` equals a small model of the contract:

- The owner can read and write.
- Another authenticated caller can read a shared record and nothing else.
- An anonymous caller reaches only the ownerless bucket.

This tests our policy corpus, not Cedar.

### D9. Native fuzzing for parsers and sanitizers

The Go `testing.F` fuzz targets cover:

- `decodeCursor`: any input either fails with an error or decodes to a cursor that encodes back to
  the same token.
- `sanitizeViewValue`: the output holds no C0 or C1 control character. This target fails on `main`
  today, so the pilot fixes #831 in the same task.
- `scope.NormalizeRemote`: normalizing a second time changes nothing.
- `shortid.Canonical`: the output uses only the Crockford alphabet, and canonicalizing a second time
  changes nothing.
- The RFC3339 window parsing for `created_after`, `created_before`, `not_before` and `not_after`.

`go test ./...` runs each seed corpus. The deep run fuzzes each target for a fixed time. Go fuzzes
one target per invocation, so `task test:deep` lists the targets with `go test -list '^Fuzz'` and
runs them in turn. A failing input lands under the package's `testdata/fuzz/`. A contributor
commits it with the fix, and from then on it runs in every pull request.

### D10. One scheduled workflow for the deep run

`.github/workflows/deep-test.yaml` runs on a weekly cron and on `workflow_dispatch`, with
`permissions: contents: read`. It has two jobs:

- `fuzz`: runs the fuzz stage of `task test:deep`. If it fails, it uploads the new files under
  `testdata/fuzz/` with `actions/upload-artifact`.
- `invariants`: runs the invariant stage of `task test:deep` beside a Qdrant service container. It
  uses the image that `ci.yaml` pins.

Issue #924 adds its mutation jobs to this workflow later.

Rejected: running the deep stages on every push to `main`. The weekly cadence finds the same
defects, and the CPU it saves is the point of #923.

### D11. Budgets

| Stage | Per PR (`go test ./...`) | Deep run |
| --- | --- | --- |
| Pure properties (D4 CLI, D7, D8, round-trips) | rapid default, 100 checks | `RAPID_CHECKS=10000` |
| Server properties (D4, D5, D6) | Measured. At most 20 s added to the `test` job. | `RAPID_CHECKS=1000` |
| Store state machine (D3) | Measured. At most 30 s added to the `test` job. | `RAPID_CHECKS=1000`, `RAPID_STEPS=100` |
| Fuzz targets (D9) | Seed corpus only | 60 s per target |

The per-PR budgets are measurements to take before shipping. The default check count can go over a
budget. In that case the CI test step sets a lower `RAPID_CHECKS` for that package. The task records
each measured value here.

### D12. The test plan lives in the contributor docs

`docs-site/src/content/docs/contributing/testing.md` holds the test plan: test kinds per layer, and
what runs per PR, on `main`, weekly and on demand. It also holds the gap review, one table per area
with the test kind that closes each gap. Each gap that this change does not close becomes a GitHub
issue, and the page links it. The page names commands, not counts or versions.

Rejected: `docs/`. It holds ADRs and agent material, and contributors read the docs site.

### D13. The OpenSpec configuration carries the practice

`openspec/config.yaml` changes in four places:

- `context`: the testing constraint names the three test kinds. It keeps both bans: no tests of
  third-party behaviour, and no hand-rolled machinery that tests the suite.
- `rules.specs`: an invariant is stated as a requirement that holds after any sequence of named
  operations, and it names each derived state and the non-owner case where recall is involved.
- `rules.design`: the design names the test kind for each requirement and, for a stateful
  invariant, the model that the test compares against.
- `rules.tasks`: each scenario gets a scenario-test task, each invariant gets a property or
  state-machine task, and each new parser or sanitizer gets a fuzz target.

## Risks / Trade-offs

- [Under Qdrant load (#646), the store and server properties slow the `test` job or flake.] → Each
  test uses its own collection on the shared CI Qdrant. D11 caps the per-PR cost by measurement. A
  failure prints its seed, so `-rapid.seed` replays it exactly.
- [A model in D3 or D8 drifts from the real rules.] → A model holds only the rules that the memory
  contract states. A disagreement between a model and the code is a finding to resolve against the
  contract, never by changing the model to match the code.
- [The D4 generators miss an invalid class, so parity holds only for the classes they draw.] → The
  generators draw from the `HintCode` list, so a new hint code needs a generator before its test
  can cover it. The test plan names this step.
- [Fuzzing and the properties find old defects outside this change.] → That is intended. Each
  finding becomes an issue or a fix, with its input or seed committed.

## Migration Plan

No migration. Everything this change adds is test code, dev dependencies, one Taskfile target, one
scheduled workflow, one docs page and the OpenSpec configuration. The #831 fix changes only the
operator text view. To roll back, delete the workflow and the target. The tests stay valid on their
own.

## Open Questions

- Which verbs join the D4 and D5 properties after the pilot verbs. The gap review proposes the
  list, and it can grow without changing this design.
