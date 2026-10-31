# Proposal

## Why

Originating issue: #923.

The suite has about 2,050 Go tests in 33 packages and 70 UI test files. Every test checks hand-picked
examples. The code names invariants in 104 places, but no test generates inputs, runs a random
sequence of operations, or fuzzes a parser. So a test proves only the cases its author thought of.
Open bug #831 (sanitizeViewValue keeps C1 control characters) is the kind of defect that a fuzz
target finds and an example test misses. No document says which kind of test each layer gets, so
each change decides alone.

## What Changes

- **Three test kinds, defined for this repository.**
  - A scenario test is the BDD form. It is a Go or UI test that implements one OpenSpec
    `#### Scenario:` and carries the scenario's name. There is no Gherkin layer, because the spec is
    already the scenario text.
  - An invariant test is a property-based test (`pgregory.net/rapid` in Go, `fast-check` in `ui/`).
    For stateful invariants it is a model-based state-machine test: random operation sequences run
    against the real component and a small in-memory model, and the invariant is checked after
    every step.
  - A fuzz target uses Go native fuzzing (`testing.F`), for parsers and sanitizers.
- **Invariant tests across the core.** The pilot covers the invariants that the memory contract
  states:
  - In `internal/store`, a state machine for isolation, supersession, the derived states, cursor
    pagination and `recall_gate_hidden`.
  - Lane parity: MCP, Connect and the CLI return the same outcome, `field` and `hint` for the same
    input.
  - Idempotency: a replayed `idempotency_key` returns the first result and never writes twice.
  - Migration: every registered step is additive, and applying the sweep twice equals applying it
    once.
  - Bounds: the content, summary and tag limits hold on every write path.
  - Authorization: the policy decision matches a small model of owner, visibility and principal.
  - Error envelope: every rejection carries a `field` and a `hint` from the documented vocabulary.
  - Pure round-trips: cursor, `short_id`, scope and RFC3339 windows.
- **Fuzz targets** for the view sanitizers, the cursor decoder, scope and `short_id`
  normalization, and the RFC3339 window parsing. The sanitizer target fails today, so this change
  fixes #831.
- **A weekly deep run.** A scheduled workflow runs each fuzz target for a fixed time and the
  invariant tests with more checks than the per-PR suite. `task test:deep` runs the same stages
  locally.
- **A test plan and a gap review.** A new contributor page, `docs-site` `contributing/testing.md`,
  holds the test plan and lists the gaps for each area. The plan says which test kinds each layer
  gets, and what runs per PR, on `main`, weekly and on demand. Each gap names the test kind that
  closes it.
- **OpenSpec configuration.** `openspec/config.yaml` requires the practice in every later change:
  invariants stated as requirements, one scenario test per scenario, and a task that names each
  test.

## Capabilities

### New Capabilities

- `test-practice`: The repository's test tooling contract. It covers the scheduled deep run of fuzz
  and invariant tests, its failure rules and permissions, and the fuzz and invariant coverage that
  every pull request runs.

### Modified Capabilities

None. The invariant tests cover behaviour that today's specs and the memory contract already define.
They add no requirement to those capabilities. The #831 fix makes `sanitizeViewValue` do what its
contract already says.

## Impact

- Surfaces, each deliberately unchanged unless named: MCP tool schemas, Connect proto, CLI verbs,
  the console's behaviour, the Helm chart, the engram skill and the CLAUDE.md memory contract are
  all unchanged. `docs-site` gains one contributor page. `ui/` gains test code and one dev
  dependency.
- No wire contract changes: no proto field, MCP input schema, error hint code, `ENGRAM_` key or
  stored payload key. The CLI operator text view strips C1 control characters after the #831 fix.
- New dependencies: `pgregory.net/rapid` (test-only) in `go.mod`, and `fast-check` in the
  `ui/package.json` dev dependencies.
- New files: test files in `internal/store`, `internal/server`, `internal/migrate`,
  `internal/authz`, `internal/shortid`, `internal/scope`, `cmd/engram` and `ui/`, and a scheduled
  workflow under `.github/workflows/`. `Taskfile.yaml` gains `task test:deep`.
- Per-PR CI time: the invariant tests and fuzz seed corpora join `go test ./...`. The design sets
  their per-PR budget.

## Out of scope

- Mutation testing. It moves to #924, which builds on the invariant tests and the deep-run workflow
  from this change.
- The CI waste fixes from #923, such as a `concurrency` group and less duplicated setup. They are
  configuration and ship as separate `ci:` pull requests against #923.
- Coverage-percentage gates.
- A Gherkin or godog layer.
- Rewriting existing behaviour tests, except where an invariant test replaces example tests that it
  covers.
- Closing every gap that the review finds. The review files each remaining gap as an issue.
