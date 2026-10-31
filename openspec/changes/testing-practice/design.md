# Design

## Context

See proposal.md (Why) for the motivation and specs/test-practice/spec.md for the tooling contract.

The facts that shape the approach:

- `go test ./...` runs every package in one CI step, in the `test` job, beside a shared Qdrant
  service (`.github/workflows/ci.yaml`). On the 2026-10-31 `main` run, the slowest packages were
  `internal/store` (117 s), `internal/e2e` (46 s over two binaries), `internal/surfaces` (45 s) and
  `internal/server` (29 s).
- Store tests get an isolated collection through `newTestStore` (`internal/store/store_test.go`)
  on the shared Qdrant that `storetest` dials. The store takes an injected clock through
  `store.WithClock`, and the server handlers take one through the `deps.now` field.
- `listRecallHidden` (`internal/server/hiddencount.go`) computes `recall_gate_hidden` in the
  server, not the store. It fetches an ungated page under the request's inclusion flags, and it
  reads `time.Now()` directly instead of the `deps` clock.
- Lane parity is tested today with fixed tables: `TestReadParity`
  (`internal/server/connectapi_parity_test.go`) and `TestWriteParity`
  (`internal/server/connectapi_write_parity_test.go`). The lanes differ on purpose for update: the
  MCP handler runs `validateUpdateArgs`, which requires `content` (`internal/server/tools.go`), and
  Connect `UpdateMemory` calls `deps.updateMemory` directly and allows a partial update.
- The hint vocabulary is the `HintCode` constants in `internal/server/argerror.go`, and
  `renderHintEnvelope` writes the `field=<name> hint=<code>` envelope.
- Write limits come from three sources. Memory content, summary and tag limits come from
  configuration, and a summary limit of 0 turns that check off. Rules use the fixed constants
  `maxRuleContentBytes` and `maxRuleSummaryBytes` (`internal/server/rules.go`). Discoveries use the
  fixed `maxDiscoveryContentBytes`.
- Idempotency derives a deterministic point id from owner, scope and key (`idempotencyPointID`,
  `internal/server/idempotency.go`), and `Store.Upsert` replaces a point with the same id in place.
  So the record id and the record count cannot show whether a replay wrote again.
- The migration sweep is `Store.Migrate` (`internal/store/migrate.go`): chain selection, Qdrant
  reads and writes, and `schema_version` stamping. `internal/migrate` holds the steps and
  `CheckAdditive`.
- `Store.Delete` deletes a record without changing any supersession link that names it.
- `decodeCursor` accepts any base64url JSON that unmarshals into the cursor struct, so `e30`
  (`{}`) decodes, and `encodeCursor` writes canonical JSON. `shortid.Canonical` lowercases any
  input, so its output is in the Crockford alphabet only for a valid handle. `scope.NormalizeRemote`
  strips one trailing `.git`, so it is not idempotent, and its output becomes persisted scope names.
- `classifyInput` (`ui/src/lib/search/classify.ts`) returns one of five kinds: `empty`, `id`,
  `short_id`, `text` and `operators`.
- `ui/vite.config.ts` defines two Vitest projects. A `node` project runs `src/**/*.test.ts`, and a
  `browser` project runs `src/**/*.browser.test.ts` under Playwright. CI runs both through
  `pnpm test` in the `ui tests` job.

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
- A deep run for the UI properties. They are pure and cheap, so they run per PR at fast-check's
  default run count, in `pnpm test`.

## Decisions

### D1. Scenario tests are the BDD form, without Gherkin

A scenario test implements one OpenSpec `#### Scenario:` about application behaviour that we own.
In Go, the scenario title is the `t.Run` subtest name, verbatim. In `ui/`, it is the `it(...)`
title. Its body takes the scenario's WHEN lines, then its THEN lines. A reviewer finds the
test for a scenario by its title, with `rg`.

A scenario about tooling or configuration gets no scenario test. Examples are a workflow's
permissions, or Go running a committed fuzz input. Its task names a command or a review step, and
the PR records the result. A test of such a scenario tests the toolchain or the suite, which rules
`m45p2b4bp7` and `3p0zsqrhmb` forbid. Every scenario in this change's own spec is of this kind.

Rejected: godog with `.feature` files. Each scenario then exists twice, once in the spec and once in
Gherkin. The step-definition layer is also more code to maintain. Rejected: a comment that cites the
scenario. House rules keep plan and spec references out of code comments, and a title is a
reference that `go test -run` can select.

### D2. rapid for Go invariants, fast-check for UI invariants

`pgregory.net/rapid` generates inputs and shrinks a failing case to a minimal one. It has a
model-based state machine, `T.Repeat`, with an invariant checker under the `""` key. It reads
`-rapid.checks` and `-rapid.steps`, or the `RAPID_CHECKS` and `RAPID_STEPS` environment variables,
once per test binary. A failure prints the seed, and `-rapid.seed` replays it. Its last push was
2026-09-04. `fast-check` is the maintained TypeScript equivalent (last push 2026-10-31).

Test names select the stages:

- `TestProperty...` is a pure property. It needs no Qdrant.
- `TestModel...` is a property or a state machine against the real store. It needs Qdrant.
- `Fuzz...` is a fuzz target.

Rejected: `testing/quick`. It is frozen and does not shrink. Rejected: gopter. It is less active
than rapid and has no state-machine API that matches `T.Repeat`.

### D3. A model-based state machine for the store

`TestModelStore` opens a fresh collection through `newTestStore` with an injected clock, and holds
a small in-memory model: a map of records with owner, visibility, `superseded_by`, `archived_at`,
the validity window and a deleted mark. Two principals, an owner and another authenticated caller,
act on the store. The actions are store, schedule, supersede, archive, restore, delete, set
visibility, advance the clock, and list (each cursor page). After every action the invariant
checker compares the store with the model:

- Isolation: neither principal reads the other's private records or changes any record it does not
  own.
- Supersession: no record has more than one successor, and no chain has a cycle. A superseded
  record that still exists is missing from recall and returned by id. A delete leaves links to the
  deleted record as they were, because the store does not repair them. The model records this and
  makes no claim about a deleted record.
- Derived states: each existing record's state words match the model, in canonical order, with
  expired suppressing scheduled. Archive and restore never write `superseded_by` or `not_after`.
- Pagination: walking every cursor page returns the model's visible set with no duplicates and no
  gaps.

Search is not in the action set, because the model cannot predict vector ranking. A search
invariant is stated as membership only: every search result is in the caller's visible set.

Rejected: a model-only test of a fake store. It tests the fake, not the code that runs in
production.

### D4. Server properties: lane parity, the envelope and the hidden counts

`TestModelLaneParity` draws an argument set for one verb from generators. The generators cover each
field's valid values and its invalid values: missing, too long, too many, wrong format, out of order
and mutually exclusive. The property sends the same arguments to the MCP handler and to the Connect
handler, each on its own store, and asserts four things:

- Both lanes accept, or both reject.
- On a rejection, both name the same `field` and the same `hint`.
- The `hint` is one of the `HintCode` constants.
- The message has the `field=<name> hint=<code>` envelope.

The pilot covers the store, schedule, supersede, update and list verbs. The generators draw only
inputs inside the shared contract of both lanes. The update verb's intended difference gets its own
assertions: MCP rejects an update without `content` with `field=content hint=required`, and Connect
applies the same partial update. Both lanes' wire contracts stay as they are.

The CLI lane gets `TestPropertyHintExitCode` in `cmd/engram`: every `HintCode` maps to one exit code
and to stderr text that names the same `field` and `hint`.

`TestModelRecallHidden` checks `recall_gate_hidden` at the server, where it is computed. It drives
`deps.listMemory` with generated records and inclusion flags under a controlled clock. It compares
the counts with what the model hides in the same ungated page under the same flags. That page is
the window that the contract defines. This needs one production change: `listRecallHidden` reads the
`deps` clock instead of `time.Now()`. Production behaviour stays the same, because that clock
defaults to `time.Now`.

The properties extend `TestReadParity` and `TestWriteParity` and do not replace them. Their fixed
rows document the known cases.

### D5. Bounds on every write path

`TestModelWriteBounds` draws otherwise-valid writes with content, summary and tag sizes around each
verb's effective limit: below it, at it, one byte or one entry over it, and far over it. It runs
them through each write verb on both lanes: store, schedule, supersede, update, store rule and store
discovery. The effective limit is the configured value for memory content, summary and tags. It is
the production constant for rule content, rule summary and discovery content, which the
same-package test reads by name. With the memory summary limit set to 0, the property asserts that
no summary size is rejected.

A write within every bound succeeds. A write over a bound fails with that field and `too_long` or
`too_many`, and the store holds no record from it.

### D6. Idempotency replay

`TestModelIdempotency` sets up any predecessor records first. Then it draws a store or supersede
call with a non-empty `idempotency_key` and sends it two to five times. It asserts five things:

- Every reply carries the same record handle.
- The store gains exactly one record beyond the setup.
- Each predecessor of a supersede still exists, with its `superseded_by` set to that record.
- The stored payload after the last replay equals the payload after the first call, byte for byte.
  This shows that no replay wrote again, which the id and the count cannot show.
- A later call with the same key and different content fails with the documented conflict, and
  the store is unchanged.

### D7. Migration properties

`TestPropertyStepAdditive` in `internal/migrate` draws a payload that a version-0 record can hold,
and asserts that every registered step passes `CheckAdditive` on it.

`TestModelMigrateSweep` in `internal/store` seeds a fresh collection with generated version-0
records and runs `Store.Migrate` with apply twice. The first run moves every record to
`migrate.CurrentVersion`. The second run changes no stored payload. `schema_version` never
decreases between the runs.

### D8. Authorization against a model

`TestPropertyAuthz` draws three values:

- a principal: anonymous, the owner, or another authenticated caller
- a record: owner present or empty, private or shared
- an action: read or write

It asserts that the decision from `authz.MustDefault` equals a small model of the contract:

- The owner can read and write.
- Another authenticated caller can read a shared record and nothing else.
- An anonymous caller reaches only the ownerless bucket.

This tests our policy corpus, not Cedar.

### D9. Fuzz targets and the pure round-trips

The Go `testing.F` fuzz targets cover:

- `FuzzDecodeCursor`: any input decodes without a panic. When it decodes, encoding the cursor and
  decoding it again gives an equal cursor. Token equality is asserted only for tokens that
  `encodeCursor` wrote, in `TestPropertyCursorRoundTrip`.
- `FuzzSanitizeViewValue`: the output holds no C0 or C1 control character. This target fails on
  `main` today, so the pilot fixes #831 in the same task.
- `FuzzCanonical`: any input canonicalizes without a panic, and the output holds no surrounding
  whitespace, no uppercase letter and none of `i`, `l` or `o`.
- The RFC3339 window parsing for `created_after`, `created_before`, `not_before` and `not_after`.

Two rapid properties cover the documented transformations, not idempotence:

- `TestPropertyCanonicalHandle` generates valid handles and their confusable and mixed-case
  spellings. Canonical gives the stored handle, in the Crockford alphabet, and a second call
  changes nothing.
- `TestPropertyNormalizeRemote` generates well-formed remotes from host, owner and repository parts,
  in https, ssh, scp and port forms, with and without `.git`. It asserts that each one normalizes to
  `host/owner/repo`. It does not assert idempotence, because a changed rule changes persisted scope
  names.

`go test ./...` runs each seed corpus. The deep run fuzzes each target for a fixed time. Go fuzzes
one target per invocation, so `task test:deep` lists the targets with `go test -list '^Fuzz'` and
runs them in turn. A failing input lands under the package's `testdata/fuzz/`. A contributor
commits it with the fix, and from then on it runs in every pull request.

### D10. One scheduled workflow for the deep run

`.github/workflows/deep-test.yaml` runs on a weekly cron and on `workflow_dispatch`, with
`permissions: contents: read`. It has two jobs:

- `fuzz`: runs the fuzz stage of `task test:deep`. On failure, it uploads the new files under
  `testdata/fuzz/` with `actions/upload-artifact`.
- `invariants`: runs the invariant stage of `task test:deep` beside a Qdrant service container. It
  uses the image that `ci.yaml` pins.

Issue #924 adds its mutation jobs to this workflow later.

Rejected: running the deep stages on every push to `main`. The weekly cadence finds the same
defects, and the CPU it saves is the point of #923.

### D11. Budgets and invocations

rapid reads one check count per test binary. So each stage with its own count runs in its own
`go test` invocation, and the D2 name prefixes select it. No test runs twice.

| Stage | Per PR | Deep run |
| --- | --- | --- |
| `TestProperty...` (Go) | In `go test ./...`, rapid default, 100 checks | `RAPID_CHECKS=10000 go test -run '^TestProperty' ./...` |
| `TestModel...` (Go) | In `go test ./...`. Measured. At most 50 s added to the `test` job. | `RAPID_CHECKS=1000 RAPID_STEPS=100 go test -run '^TestModel' ./internal/store/... ./internal/server/...` |
| `Fuzz...` (Go) | Seed corpus, in `go test ./...` | 60 s per target |
| UI properties | In `pnpm test`, fast-check default | None |

The per-PR `TestModel` budget is a measurement to take before shipping. If the default count goes
over 50 s, the CI test step splits in two. The first step runs `go test -skip '^TestModel' ./...`.
The second runs `RAPID_CHECKS=<n> go test -run '^TestModel'` on the two packages, with `<n>` set to
fit the budget. The task records the measured values here.

### D12. The test plan lives in the contributor docs

`docs-site/src/content/docs/contributing/testing.md` holds the test plan: test kinds per layer, the
D2 name prefixes, and what runs per PR, on `main`, weekly and on demand. It also holds the gap
review, one table per area with the test kind that closes each gap. Each gap that this change does
not close becomes a GitHub issue, and the page links it. The page names commands, not counts or
versions.

Rejected: `docs/`. It holds ADRs and agent material, and contributors read the docs site.

### D13. The OpenSpec configuration carries the practice

`openspec/config.yaml` changes in four places:

- `context`: the testing constraint names the three test kinds and the D2 name prefixes. It keeps
  both bans: no tests of third-party behaviour, and no hand-rolled machinery that tests the suite.
- `rules.specs`: an invariant is stated as a requirement that holds after any sequence of named
  operations, and it names each derived state and the non-owner case where recall is involved.
- `rules.design`: the design names the test kind for each requirement and, for a stateful
  invariant, the model that the test compares against.
- `rules.tasks`: each scenario about owned application behaviour gets a scenario-test task. Each
  tooling or configuration scenario gets a task that names the command or review step and records
  the result. Each invariant gets a property or state-machine task, and each new parser or
  sanitizer gets a fuzz target. A task that runs a new test verifies it with `-v` and the
  `--- PASS: <name>` line, so a selector that matches nothing cannot pass.

## Risks / Trade-offs

- [Under Qdrant load (#646), the `TestModel` tests slow the `test` job or flake.] → Each test uses
  its own collection on the shared CI Qdrant. D11 caps the per-PR cost by measurement. A failure
  prints its seed, so `-rapid.seed` replays it exactly.
- [A model in D3, D4 or D8 drifts from the real rules.] → A model holds only the rules that the
  memory contract states. A disagreement between a model and the code is a finding to resolve
  against the contract, never by changing the model to match the code.
- [The D4 generators miss an invalid class, so parity holds only for the classes they draw.] → The
  generators draw from the `HintCode` list, so a new hint code needs a generator before its test
  can cover it. The test plan names this step.
- [A test that needs Qdrant skips without it and looks green.] → Every task that runs a `TestModel`
  test sets `ENGRAM_REQUIRE_QDRANT=1`, so a missing Qdrant fails the run.
- [Fuzzing and the properties find old defects outside this change.] → That is intended. Each
  finding becomes an issue or a fix, with its input or seed committed.

## Migration Plan

No migration. This change adds test code, dev dependencies, one Taskfile target, one scheduled
workflow, one docs page and the OpenSpec configuration. It makes two production edits: the #831
sanitizer fix, and the `listRecallHidden` clock source, which keeps today's behaviour. To roll back,
delete the workflow and the target. The tests stay valid on their own.

## Open Questions

- Which verbs join the D4 and D5 properties after the pilot verbs. The gap review proposes the
  list, and it can grow without changing this design.
