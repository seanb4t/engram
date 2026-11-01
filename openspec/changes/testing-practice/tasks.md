# Tasks

A Go test task verifies a new test with `go test -v` and the `--- PASS: <name>` line for each named
test, and with no `--- SKIP` line for it. A selector that matches nothing then cannot pass. Every run
of a `TestModel` test sets `ENGRAM_REQUIRE_QDRANT=1`.

## 1. Dependencies

- [x] 1.1 Add `pgregory.net/rapid` as a test dependency in `go.mod`. Verify: `go mod tidy` leaves
  no diff, and `go list -m pgregory.net/rapid` names the module.
- [x] 1.2 Add `fast-check` to the `ui/package.json` dev dependencies. Verify:
  `jq -r '.devDependencies["fast-check"]' ui/package.json` prints a version, `cd ui && pnpm install
  --frozen-lockfile` succeeds in CI, and `task ui:build` leaves `internal/webauth/static/`
  unchanged.

## 2. Fuzz targets

- [x] 2.1 Add `FuzzSanitizeViewValue` in `cmd/engram`, with the property that the output holds no
  C0 or C1 control character. Verify: the target fails on the current code with a C1 input.
- [x] 2.2 Fix #831 so that `sanitizeViewValue` strips C1 control characters. Commit the failing
  input under `cmd/engram/testdata/fuzz/FuzzSanitizeViewValue/`. Verify:
  `go test -v ./cmd/engram -run FuzzSanitizeViewValue` prints `--- PASS: FuzzSanitizeViewValue`,
  and the PR closes #831.
- [x] 2.3 Add `FuzzDecodeCursor` in `internal/store`: any input decodes without a panic, and a
  decoded cursor encodes and decodes again to an equal cursor. Verify:
  `go test ./internal/store -run '^$' -fuzz FuzzDecodeCursor -fuzztime 60s` finds no failure, or
  each failure it finds is fixed with its input committed.
- [x] 2.4 Add `FuzzCanonical` in `internal/shortid`: any input canonicalizes without a panic, and
  the output holds no surrounding whitespace, no rune that lowercasing changes, and none of `i`, `l` or `o`.
  Verify: it passes a 60 s `-fuzz` run, or each failure is fixed with its input committed.
- [x] 2.5 Add a fuzz target for the RFC3339 window parsing that the recall verbs share. Its
  property: an input either fails with the `field` and `hint` that `reference/errors.md` lists, or
  parses to a time that formats back to an equal instant. Verify: it passes a 60 s `-fuzz` run, or
  each failure is fixed with its input committed.

## 3. Pure invariant tests

- [x] 3.1 Add `TestPropertyCursorRoundTrip` in `internal/store`: every token that `encodeCursor`
  writes decodes and encodes back to the same token. Add `TestPropertyCanonicalHandle` in
  `internal/shortid` for generated valid handles and their confusable and mixed-case spellings.
  Verify: both print `--- PASS` under `-v`, and a deliberate break in `encodeCursor` fails the
  first with a shrunk counterexample. Revert the break.
- [x] 3.2 Add `TestPropertyNormalizeRemote` in `internal/scope`: generated https, ssh, scp and port
  remotes, with and without `.git`, normalize to `host/owner/repo`. It asserts no idempotence.
  Verify: it prints `--- PASS` under `-v`, and `scope.go` is unchanged.
- [x] 3.3 Add `TestPropertyDerivedState`: for any window and any archive or supersession state, the
  words follow canonical order and expired suppresses scheduled. Verify: it prints `--- PASS` under
  `-v`.
- [x] 3.4 Add `TestPropertyStepAdditive` in `internal/migrate`: every registered step passes
  `CheckAdditive` on generated version-0 payloads. Verify: it prints `--- PASS` under `-v`, and a
  step that drops a key makes it fail. Revert the break.
- [x] 3.5 Add `TestPropertyAuthz` in `internal/authz`, against the D8 model of owner, visibility and
  principal. Verify: it prints `--- PASS` under `-v`, and a policy edit that lets a non-owner write
  makes it fail. Revert the edit.
- [x] 3.6 Add `TestPropertyHintExitCode` in `cmd/engram`: every `HintCode` maps to one exit code and
  to stderr text that names the same `field` and `hint`. Verify: it prints `--- PASS` under `-v`.
- [x] 3.7 Add `fast-check` properties for `classifyInput` that cover all five outcomes:
  - blank input is `empty`
  - a UUID is `id`
  - a canonical `short_id` is `short_id`
  - operator-only input such as `tag:lint` is `operators`
  - other input is `text`, with whitespace trimmed

  Verify:
  `cd ui && pnpm test` passes, and each property's title names its outcome.

## 4. Store invariant tests

- [x] 4.1 Add the D3 model and `TestModelStore`, a `T.Repeat` state machine with the D3 action set
  and two principals, against a fresh collection from `newTestStore` with `store.WithClock`. Verify:
  `ENGRAM_REQUIRE_QDRANT=1 go test -v ./internal/store -run TestModelStore` prints
  `--- PASS: TestModelStore`.
- [x] 4.2 Add the D3 invariant assertions for isolation, supersession (no fork, no cycle, deleted
  records tracked), derived states and pagination. Verify: break each rule in the model only, and
  the matching assertion fails. Revert the break.
- [x] 4.3 Add `TestModelMigrateSweep`: seed generated version-0 records and run `Store.Migrate` with
  apply twice. The first run reaches `migrate.CurrentVersion`, and the second changes no stored
  payload. Verify: it prints `--- PASS` under `-v`, and a sweep that stamps a version below the
  target makes it fail. Revert the break. A step that rewrites a key cannot make it fail, because
  the sweep writes only added keys and never selects a current record again.
- [x] 4.4 File an issue for each disagreement between the model and the store that the contract
  does not settle. Verify: each issue is linked from the PR.

## 5. Server invariant tests

- [x] 5.1 Change `listRecallHidden` to read the `deps` clock instead of `time.Now()`. Verify: the
  existing `internal/server` tests pass unchanged, and `rg -n 'time\.Now' internal/server/hiddencount.go`
  finds nothing.
- [x] 5.2 Add `TestModelRecallHidden` from design D4: under a controlled clock, the counts from
  `deps.listMemory` match the model for the same ungated page and flags. Verify: it prints
  `--- PASS` under `-v`.
- [x] 5.3 Add `TestModelLaneParity` for the store, schedule, supersede, update and list verbs, with
  generators inside the lanes' shared contract and drawn from the `HintCode` list. Add the explicit
  update assertions: MCP rejects a missing `content` with `field=content hint=required`, and
  Connect applies the same partial update. Verify: it prints `--- PASS` under `-v`, and a change
  that makes one lane return a different `hint` for one field makes it fail. Revert the change.
- [x] 5.4 Add `TestModelWriteBounds` for store, schedule, supersede, update, store rule and store
  discovery on both lanes. Memory limits come from the test configuration, rule and discovery
  limits from the production constants, and a 0 summary limit asserts no summary rejection.
  Verify: it prints `--- PASS` under `-v`, and the shrunk counterexample for a deliberately
  off-by-one limit names that field. Revert the break.
- [x] 5.5 Add `TestModelIdempotency` for store and supersede, with the five D6 assertions, including
  the byte-equal stored payload after the last replay. Verify: it prints `--- PASS` under `-v`, and
  a change that rewrites the payload on replay makes it fail. Revert the change.
- [ ] 5.6 Measure the per-PR cost of every `TestModel` test on CI at the default `RAPID_CHECKS`.
  Record it in design.md D11. If it goes over 50 s, split the CI test step as D11 describes, with
  disjoint `-skip '^TestModel'` and `-run '^TestModel'` selections. Verify: the PR's `test` job
  time grows by at most 50 s over the `main` baseline, and each `TestModel` test appears once in
  the job log.

## 6. Deep run

- [ ] 6.1 Add `task test:deep`. It runs every Go fuzz target for 60 s in turn (targets listed with
  `go test -list '^Fuzz'`), then runs the two D11 deep invocations for `TestProperty` and
  `TestModel`. Verify: `task test:deep` runs each target and exits 0 on a clean tree.
- [x] 6.2 Add `.github/workflows/deep-test.yaml` with a weekly cron and `workflow_dispatch`,
  `permissions: contents: read`, and the two D10 jobs. The invariant job uses the Qdrant image that
  `ci.yaml` pins. Verify: `actionlint` passes, and the file names no secret other than
  `GITHUB_TOKEN`.
- [ ] 6.3 Add an upload step to the fuzz job. It runs only on failure, and it uploads the new
  `testdata/fuzz/` files as an artifact. Verify: a manual dispatch from the branch completes, and
  both jobs pass.
- [ ] 6.4 Verify the failure rules. In a scratch branch, make one fuzz target and one invariant test
  fail, and dispatch the workflow. Verify: the run fails, the failing input is an artifact, and the
  invariant job log names the replay seed. Delete the scratch branch.
- [x] 6.5 Make sure that the deep stages stay off the per-PR path. Review `task --dry` and
  `task --dry test`, and every workflow that `rg -l 'pull_request' .github/workflows` lists,
  including the commands each one invokes. Verify: none of them runs `test:deep`, a `-fuzz` or
  `-fuzztime` flag, or the deep-run `RAPID_CHECKS` or `RAPID_STEPS` values. The PR's
  required status contexts are unchanged. The PR records this review.

## 7. Practice and test plan

- [x] 7.1 Update `openspec/config.yaml` as design D13 describes, in `context`, `rules.specs`,
  `rules.design` and `rules.tasks`. Verify: `openspec instructions specs --change testing-practice
  --json` shows the new rules, and `task lint` passes.
- [x] 7.2 Write `docs-site/src/content/docs/contributing/testing.md` with the test plan from design
  D12: test kinds per layer, the D2 name prefixes, and the per-PR, `main`, weekly and on-demand
  runs. Include how to replay a failing seed and how to commit a fuzz input. Verify:
  `task lint:markdown` passes and the docs-site build lists the page under Contributing.
- [x] 7.3 Run the gap review for each area: `internal/store`, the MCP lane, the Connect lane, the
  CLI, `internal/migrate`, auth and authz, `internal/e2e`, `ui/`, the skill hooks and the Helm
  chart. Add one table per area to the testing page, with the test kind that closes each gap.
  Verify: every area has a table, and each row names a test kind.
- [ ] 7.4 File a GitHub issue for each gap that this change does not close. Fold in the open test
  issues #851, #646, #652, #366, #499, #357 and #723 where they fit. Link each issue from its row.
  Verify: every row that this change does not close has an issue link.
- [x] 7.5 Leave the engram skill and the CLAUDE.md memory contract unchanged, because no agent
  capability changes. Verify: `git diff main -- skill/ CLAUDE.md` is empty.

## 8. Quality gate

- [ ] 8.1 Run `task` (lint and test). Verify: it exits 0.
- [ ] 8.2 Run `task ui:build`. Verify: `git diff --exit-code internal/webauth/static/` passes.

## Workflow follow-up

- Comment on #923 with the deep-run run link and the issues filed in 4.4 and 7.4.
- Point #924 at the merged deep-run workflow.
- Archive the change after the PR merges.
