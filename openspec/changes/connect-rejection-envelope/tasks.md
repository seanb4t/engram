# Tasks

A Go test task verifies a new test with `go test -v` and the `--- PASS: <name>` line for each named
test, and with no `--- SKIP` line for it. A selector that matches nothing then cannot pass. Every run
of a `TestModel` test sets `ENGRAM_REQUIRE_QDRANT=1`.

## 1. Proto constraints

- [ ] 1.1 Set `IGNORE_IF_ZERO_VALUE` on `category` in `StoreMemoryRequest` and
  `ScheduleMemoryRequest`, and on `scope` in `StoreDiscoveryRequest`. Add `discovery` to the
  `ScheduleMemoryRequest` allowlist. Split the schedule CEL rule into
  `schedule-window-at-least-one-bound` and `window-not-before-before-not-after`. Verify:
  `task proto:lint` passes, and `go tool buf breaking --against '.git#branch=main'` reports nothing.
- [ ] 1.2 Regenerate with `task proto:gen`, then `task ui:build`. Verify: `git diff --stat` shows
  changes only in `gen/`, `ui/src/lib/gen/` and the vendored bundle, and `go build ./...` passes.

## 2. Translator

- [ ] 2.1 Add the rule table and the translator in `internal/server/connectvalidate.go`, and return
  through `connectError`. Apply D1, D2 and D3. Map a field path such as `citations[0].kind` to
  `citations[i].kind`. Verify: `go build ./...` passes.
- [ ] 2.2 Add scenario tests for `A field constraint failure carries the envelope`, one subtest per
  scenario title, over the mounted chain with `newSpyDeps`. Verify: `go test -v ./internal/server
  -run 'TestConnectRejectionEnvelope'` prints a `--- PASS` line for each scenario.
- [ ] 2.3 Add scenario tests for `An empty value reaches the core`, `A cross-field constraint failure
  carries the envelope`, `One rejection names one failure` and `A rejection never echoes the value`.
  Verify: the same command prints a `--- PASS` line for each scenario title.
- [ ] 2.4 Add scenario tests for `A constraint with no mapped hint stays a client error`, with a fake
  validator that returns a `ValidationError` with an unknown rule id. Verify: a `--- PASS` line for
  both scenarios.
- [ ] 2.5 Add a scenario test for each lane of `The category allowlist differs between lanes`. Verify:
  a `--- PASS` line for both scenarios.
- [ ] 2.6 Run `TestWriteRPCNegativeMatrix` and the tests in `connectvalidate_test.go` as they stand, and update any
  cell that pins the old code for an ordering or a bound. The authorization cell stays unchanged, and
  it covers `An unauthenticated caller with an invalid request`. Verify: `go test -v ./internal/server
  -run 'TestWriteRPCNegativeMatrix|Validate'` prints `--- PASS: TestWriteRPCNegativeMatrix`, and the
  diff changes no authorization cell.
- [ ] 2.7 Update `docs-site/src/content/docs/reference/errors.md`: state that a declared constraint
  failure carries the envelope, and that the category allowlist is Connect-only. Update the interceptor
  comments in `engram.proto`. Search with `rg -i 'protovalidate|buf.validate' docs-site/src` and
  correct any other page that describes the old error. Verify: `task lint:markdown` passes.

## 3. Derived lane parity

- [ ] 3.1 Add the wire client to `lanes` as in D6, and widen `envelopeRE` to admit `citations[i].kind`.
  Verify: `go test -v ./internal/server -run '^TestModelLaneParity$'` prints
  `--- PASS: TestModelLaneParity`.
- [ ] 3.2 Add the derivation from D5: walk the request messages, build the break and meet values for
  each of the eight rule kinds, and read MCP fields from the tool schemas. Fail on a message with no
  fixture, a rule that cannot be broken, or zero cases. Verify: the subtest log lists a case for each
  field constraint.
- [ ] 3.3 Add the subtests `A lane that names a different hint`, `A field with no MCP counterpart`,
  `A value that meets a constraint` and `A value that breaks a constraint`. Verify: a `--- PASS` line
  for each subtest, with `ENGRAM_REQUIRE_QDRANT=1`.
- [ ] 3.4 Add the surface rule cases. Each rule whose fields exist on a request message has a breaking
  request, and its expected hint comes from the rule. Cover `A surface rule that applies to a lane` and
  `A surface rule for the CLI alone`. Verify: the log lists the five rules that apply and skips the four
  CLI-only rules.
- [ ] 3.5 Record the tooling scenarios once in the PR, on a throwaway edit that is not committed. Add
  `max_bytes` to a request field. Change one translator hint. Add an unsupported rule, then remove a
  fixture. Verify: the PR body records that the first adds cases with no test edit, and that the other
  three fail and name the rule or message. This covers `A constraint added to a request message`, `A
  constraint kind the test cannot break` and `A request message with no fixture`.
- [ ] 3.6 Update `docs-site/src/content/docs/contributing/testing.md`. Replace the sentence that says
  a new hint needs a generator, and name what a new rule kind needs instead. Verify:
  `task lint:markdown` passes.
- [ ] 3.7 Measure the `Test (model tests)` step on the PR. Verify: the PR body records the step time
  against the 50 s budget.

## 4. Integration

- [ ] 4.1 Run the console tests, because the envelope now reaches `connect-error.ts` on these paths.
  Verify: `cd ui && pnpm test` passes.
- [ ] 4.2 Run the quality gate. Verify: `task` passes, and `task license:check` passes. `charts/` does
  not change, so `task chart:validate` is not needed.

## Workflow follow-up

- The PR closes #942 and refers to #923.
- Archive the change after the PR merges.
- File an issue for the `engram schedule --category handoff` difference if the operator wants it closed.
