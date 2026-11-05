# Tasks

A Go test task verifies a new test with `go test -v` and the `--- PASS: <name>` line for each named
test, and with no `--- SKIP` line for it. A selector that matches nothing then cannot pass. Go prints
a subtest title with underscores, so match the underscore form of each scenario title. Every run of a
`TestModel` test sets `ENGRAM_REQUIRE_QDRANT=1`.

## 1. Proto constraints

- [x] 1.1 Set `IGNORE_IF_ZERO_VALUE` on `category` in `StoreMemoryRequest` and
  `ScheduleMemoryRequest`, and on `scope` in `StoreDiscoveryRequest`. Add `discovery` to the
  `ScheduleMemoryRequest` allowlist. Split the schedule CEL rule into
  `schedule-window-at-least-one-bound` and `window-not-before-before-not-after`. In the same edit,
  correct the `ScheduleMemoryRequest` comment that describes one CEL rule, and the
  `SupersedeMemoryRequest` comment that says no rule applies. Verify: `task proto:lint` passes, and
  `go tool buf breaking --against '.git#branch=main'` reports nothing.
- [x] 1.2 Regenerate with `task proto:gen`, then `task ui:build`. Verify: `git status --short` lists
  changes only under `proto/`, `gen/`, `ui/src/lib/gen/` and `internal/webauth/static/`, and
  `go build ./...` passes.

## 2. Translator

- [x] 2.1 Add the rule table and the translator in `internal/server/connectvalidate.go`, and return
  through `connectError`. Apply D1, D2 and D3. Correct the function comment that says a validation
  failure maps to `CodeInvalidArgument`. Verify: `go build ./...` passes.
- [x] 2.2 Add scenario tests for `A field constraint failure carries the envelope`, one subtest per
  scenario title, over the mounted chain with `newSpyDeps`. Verify: `go test -v ./internal/server
  -run 'TestConnectRejectionEnvelope'` prints a `--- PASS` line for each scenario.
- [x] 2.3 Add scenario tests for `An empty value reaches the core`, `A cross-field constraint failure
  carries the envelope`, `One rejection names one failure` and `A rejection never echoes the value`.
  Verify: the same command prints a `--- PASS` line for each scenario title.
- [x] 2.4 Add scenario tests for `A constraint with no mapped hint stays a client error`, with a fake
  validator that returns a `ValidationError` with an unknown rule id. Verify: a `--- PASS` line for
  both scenarios.
- [x] 2.5 Add a scenario test for each lane of `The category allowlist differs between lanes`. Verify:
  a `--- PASS` line for both scenarios.
- [x] 2.6 Add subtests for `An unauthenticated caller with an invalid request` and `A failed CSRF
  check with an invalid request`. Each sends an invalid payload and asserts the code and that the
  message holds no `field=`. Verify: a `--- PASS` line for both scenarios.
- [x] 2.7 Run `TestWriteRPCNegativeMatrix` and the tests in `connectvalidate_test.go` as they stand.
  Verify: `go test -v ./internal/server -run 'TestWriteRPCNegativeMatrix|Validate'` prints
  `--- PASS: TestWriteRPCNegativeMatrix`, and the change edits no cell of that matrix. If a cell must
  change, stop and record why in the PR.
- [x] 2.8 Update `docs-site/src/content/docs/reference/errors.md`: state that a declared constraint
  failure carries the envelope, add the pseudo-field `request` beside `response`, and state that the
  category allowlist is Connect-only. Add an entry to the Unreleased section of
  `docs-site/src/content/docs/guides/upgrade.md` for the changed text and codes. Search with
  `rg -i 'invalid_argument|CodeInvalidArgument|validation error' docs-site/src` and correct any other
  page that describes the old error. Verify: `task lint:markdown` shows no finding under `docs-site/`.

## 3. Derived lane parity

- [x] 3.1 Add the wire client to `lanes` as in D6, and widen `envelopeRE` to admit `citations[i].kind`.
  Verify: `go test -v ./internal/server -run '^TestModelLaneParity$'` prints
  `--- PASS: TestModelLaneParity`.
- [x] 3.2 Add the derivation from D5: walk the request messages and their nested messages, and read
  MCP fields from the tool schemas. Add the fixtures, including `SupersedeMemoryRequest`, with a
  seeded target on each lane. Build break and meet cases for the six rule kinds. Fail on a message
  with no fixture, a rule that cannot be broken, or zero cases. Check that a non-empty break value is
  absent from the message. Verify: the subtest log lists a case for each rule value on a field that
  both lanes check, and none for `update_mask` or `visibility`.
- [x] 3.3 Add the subtests `A value that meets a constraint`, `A value at a declared bound`, `A value
  that breaks a constraint` and `The category allowlist exception`. Verify: a `--- PASS` line for
  each subtest, with `ENGRAM_REQUIRE_QDRANT=1`.
- [x] 3.4 Add the surface rule cases. Each rule that has all of its `Fields` on a request message has
  a breaking request, and its expected hint comes from the rule. Verify: the log lists a case for
  each of the five rules that apply, and no case for the four CLI-only rules. This covers `A surface
  rule for the CLI alone` and `A field with no MCP counterpart`.
- [x] 3.5 Commit, then record the tooling scenarios once on throwaway edits that are never committed.
  For each proto edit, run `task proto:gen`. Run `TestModelLaneParity` after each edit:
  1. Add `string.min_len: 1` to `StoreMemoryRequest.source`. The test passes with a new case and no
     test edit (`A constraint added to a request message`).
  2. Change the translator's `string.prefix` hint to `format`. The test fails and names the message,
     the field and the rule (`A lane that names a different hint`).
  3. Add a `string.pattern` rule to `StoreMemoryRequest.scope`. The test fails and names the rule
     (`A constraint kind the test cannot break`).
  4. Remove the `StoreDiscoveryRequest` fixture. The test fails and names the message (`A request
     message with no fixture`).
  5. Remove the breaking request of one surface rule. The test fails and names the rule (`A surface
     rule that applies to a lane`).

  Revert with `git restore proto gen ui/src/lib/gen internal/server internal/webauth/static`. Verify: the PR body records each result, and `git status --short`
  lists no tracked file.
- [x] 3.6 Update `docs-site/src/content/docs/contributing/testing.md`. Narrow the sentence on new hint
  codes: a hint that only the core gives, or a configured bound, still needs a generator. A new proto
  rule on a known kind needs no test edit, and a new rule kind needs a break value in
  `TestModelLaneParity`. Verify: `task lint:markdown` shows no finding under `docs-site/`.
- [x] 3.7 Measure the `Test (model tests)` step on the PR. Verify: the PR body records the step time
  against the 50 s budget.

## 4. Integration

- [x] 4.1 Run the console tests against the regenerated clients. Verify: `cd ui && pnpm test` passes.
- [x] 4.2 Run the quality gate. Verify: `task` passes, and `task license:check` passes. Also confirm
  that `task proto:gen` leaves `git status --short` unchanged, because `task` does not run the drift
  check. `charts/` does not change, so `task chart:validate` is not needed.

## Workflow follow-up

- The implementation commit uses `fix(server)!:` (D7). The PR closes #942 and refers to #923.
- Archive the change after the PR merges.
- Decide on an issue for the Connect rejection of `category=handoff`, which makes
  `engram schedule --category handoff` fail while MCP `schedule_memory` accepts it.
