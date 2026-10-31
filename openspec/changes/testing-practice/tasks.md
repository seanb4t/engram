# Tasks

## 1. Dependencies

- [ ] 1.1 Add `pgregory.net/rapid` as a test dependency in `go.mod`. Verify: `go mod tidy` leaves
  no diff, and `go list -m pgregory.net/rapid` names the module.
- [ ] 1.2 Add `fast-check` to the `ui/package.json` dev dependencies. Verify:
  `cd ui && pnpm install --frozen-lockfile` succeeds in CI, and `task ui:build` leaves
  `internal/webauth/static/` unchanged.

## 2. Fuzz targets

- [ ] 2.1 Add `FuzzSanitizeViewValue` in `cmd/engram`, with the property that the output holds no
  C0 or C1 control character. Verify: the target fails on the current code with a C1 input.
- [ ] 2.2 Fix #831 so that `sanitizeViewValue` strips C1 control characters. Commit the failing
  input under `cmd/engram/testdata/fuzz/FuzzSanitizeViewValue/`. Verify:
  `go test ./cmd/engram -run FuzzSanitizeViewValue` passes, and the PR closes #831.
- [ ] 2.3 Add `FuzzDecodeCursor` in `internal/store`: any input either fails with an error or
  decodes to a cursor that encodes back to the same token. Verify:
  `go test ./internal/store -run '^$' -fuzz FuzzDecodeCursor -fuzztime 60s` finds no failure, or
  each failure it finds is fixed with its input committed.
- [ ] 2.4 Add `FuzzNormalizeRemote` in `internal/scope` (normalizing twice equals normalizing once)
  and `FuzzCanonical` in `internal/shortid` (the output uses only the Crockford alphabet, and
  canonicalizing twice equals canonicalizing once). Verify: each passes a 60 s `-fuzz` run, or each
  failure is fixed with its input committed.
- [ ] 2.5 Add a fuzz target for the RFC3339 window parsing that the recall verbs share. Its
  property: an input either fails with the `field` and `hint` that `reference/errors.md` lists, or
  parses to a time that formats back to an equal instant. Verify: it passes a 60 s `-fuzz` run, or
  each failure is fixed with its input committed.

## 3. Pure invariant tests

- [ ] 3.1 Add rapid property tests for the cursor round-trip and the `short_id` round-trip.
  Verify: `go test ./internal/store ./internal/shortid -run 'Property'` passes, and a deliberate
  break in `encodeCursor` makes it fail with a shrunk counterexample. Revert the break.
- [ ] 3.2 Add a rapid property test for the derived-state words: for any window and any archive
  or supersession state, the words follow canonical order and expired suppresses scheduled.
  Verify: the test passes under `go test ./...`.
- [ ] 3.3 Add the D7 migration properties in `internal/migrate`: every registered step passes
  `CheckAdditive`, a second sweep changes nothing, and `schema_version` never decreases. Verify:
  `go test ./internal/migrate -run Property` passes, and a step that drops a key makes it fail.
  Revert the break.
- [ ] 3.4 Add the D8 authorization property in `internal/authz`, against the model of owner,
  visibility and principal. Verify: `go test ./internal/authz -run Property` passes, and a policy
  edit that lets a non-owner write makes it fail. Revert the edit.
- [ ] 3.5 Add the D4 CLI property in `cmd/engram`: every `HintCode` maps to one exit code and to
  stderr text that names the same `field` and `hint`. Verify: `go test ./cmd/engram -run Property`
  passes.
- [ ] 3.6 Add `fast-check` property tests for the UI id classifier: every 36-character UUID
  classifies as an id, every canonical `short_id` classifies as a short id, and other text
  classifies as text. Verify: `cd ui && pnpm test` passes, and the test titles name the property.

## 4. Store state-machine invariant test

- [ ] 4.1 Add the in-memory model from design D3 and a `T.Repeat` state machine with the D3 action
  set and two principals, against a fresh collection from `newTestStore`. Use an injected clock.
  Verify: `go test ./internal/store -run StateMachine` passes against the shared Qdrant.
- [ ] 4.2 Add the D3 invariant assertions for isolation, supersession, derived states, pagination
  and `recall_gate_hidden`. Verify: break each rule in the model only, and the matching assertion
  fails. Revert the break.
- [ ] 4.3 Measure the per-PR cost on CI at the default `RAPID_CHECKS` value. Record it in design.md
  D11. If it goes over 30 s, set a lower `RAPID_CHECKS` for the store package in the CI test step.
  Verify: the PR's `test` job time grows by at most 30 s over the `main` baseline for this stage.
- [ ] 4.4 File an issue for each disagreement between the model and the store that the contract
  does not settle. Verify: each issue is linked from the PR.

## 5. Server invariant tests

- [ ] 5.1 Add the D4 lane-parity property in `internal/server` for the store, schedule, supersede,
  update and list verbs, with generators drawn from the `HintCode` list. Verify:
  `go test ./internal/server -run 'Property.*Parity'` passes, and a change that makes one lane
  return a different `hint` for one field makes it fail. Revert the change.
- [ ] 5.2 Add the D5 bounds property for store, schedule, supersede, update, store rule and store
  discovery on both lanes, with limits read from the test configuration. Verify:
  `go test ./internal/server -run 'Property.*Bounds'` passes, and the shrunk counterexample for a
  deliberately off-by-one limit names that field. Revert the break.
- [ ] 5.3 Add the D6 idempotency property for store and supersede. Verify:
  `go test ./internal/server -run 'Property.*Idempotency'` passes, and every replay returns the
  first record id while the store holds one record.
- [ ] 5.4 Measure the per-PR cost of 5.1 to 5.3 on CI. Record it in design.md D11. If it goes over
  20 s, set a lower `RAPID_CHECKS` for the server package in the CI test step. Verify: the PR's
  `test` job time grows by at most 20 s over the `main` baseline for this stage.

## 6. Deep run

- [ ] 6.1 Add `task test:deep`. It runs every Go fuzz target for 60 s in turn (targets listed with
  `go test -list '^Fuzz'`). Then it runs the rapid tests with the D11 deep-run values. Verify:
  `task test:deep` runs each target and exits 0 on a clean tree.
- [ ] 6.2 Add `.github/workflows/deep-test.yaml` with a weekly cron and `workflow_dispatch`,
  `permissions: contents: read`, and the two D10 jobs. The invariant job uses the Qdrant image that
  `ci.yaml` pins. Verify: `actionlint` passes, and the file names no secret other than
  `GITHUB_TOKEN`.
- [ ] 6.3 Add an upload step to the fuzz job. It runs only on failure, and it uploads the new
  `testdata/fuzz/` files as an artifact. Verify: a
  manual dispatch from the branch completes, and both jobs pass.
- [ ] 6.4 Verify the failure rules. In a scratch branch, make one fuzz target and one invariant test
  fail, and dispatch the workflow. Verify: the run fails, the failing input is an artifact, and the
  invariant job log names the replay seed. Delete the scratch branch.
- [ ] 6.5 Make sure that the deep stages stay off the per-PR path. Verify:
  `rg -n 'test:deep|fuzztime|RAPID_STEPS' .github/workflows/ci.yaml` finds nothing, and the PR's
  required status contexts are unchanged.

## 7. Practice and test plan

- [ ] 7.1 Update `openspec/config.yaml` as design D13 describes, in `context`, `rules.specs`,
  `rules.design` and `rules.tasks`. Verify: `openspec instructions specs --change testing-practice
  --json` shows the new rules, and `task lint` passes.
- [ ] 7.2 Write `docs-site/src/content/docs/contributing/testing.md` with the test plan from design
  D12: test kinds per layer, and the per-PR, `main`, weekly and on-demand runs. Include how to
  replay a failing seed and how to commit a fuzz input. Verify: `task lint:markdown` passes and the
  docs-site build lists the page under Contributing.
- [ ] 7.3 Run the gap review for each area: `internal/store`, the MCP lane, the Connect lane, the
  CLI, `internal/migrate`, auth and authz, `internal/e2e`, `ui/`, the skill hooks and the Helm
  chart. Add one table per area to the testing page, with the test kind that closes each gap.
  Verify: every area has a table, and each row names a test kind.
- [ ] 7.4 File a GitHub issue for each gap that this change does not close. Fold in the open test
  issues #851, #646, #652, #366, #499, #357 and #723 where they fit. Link each issue from its row.
  Verify: every row that this change does not close has an issue link.
- [ ] 7.5 Leave the engram skill and the CLAUDE.md memory contract unchanged, because no agent
  capability changes. Verify: `git diff main -- skill/ CLAUDE.md` is empty.

## 8. Quality gate

- [ ] 8.1 Run `task` (lint and test). Verify: it exits 0.
- [ ] 8.2 Run `task ui:build`. Verify: `git diff --exit-code internal/webauth/static/` passes.

## Workflow follow-up

- Comment on #923 with the deep-run run link and the issues filed in 4.4 and 7.4.
- Point #924 at the merged deep-run workflow.
- Archive the change after the PR merges.
