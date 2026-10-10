# Tasks

## 1. Jev client path option

- [ ] 1.1 Add `WithPath(p string)` to `internal/decide/jev` (an empty `p` keeps the default), build `c.endpoint` after the options loop in `New`, and update the package, `decisionsPath` and `Decide` doc comments from "fixed suffix" to "default suffix, overridable". Verify: `go build ./...`.
- [ ] 1.2 Add `jev_test.go` cases: the default request path is `/alpha/decisions`; `WithPath("/v1/systemone")` against a base with a trailing slash posts to `/v1/systemone`; `WithPath("")` keeps the default; and the same request sent on both paths carries the same method, headers and body and decodes the same response. Verify: `go test ./internal/decide/jev/ -run 'Path' -count=1`.

## 2. Config key and validation

- [ ] 2.1 Add the `decisions.path` / `ENGRAM_DECISIONS_PATH` registry row (default `/alpha/decisions`, no flag, no legacy name) and `DecisionsConfig.Path`. Add the row to `decisionsFields` in `decisions_config_test.go`, and update the eleven-key wording there and in `decisions_docs_test.go`. Verify: `go test ./internal/config/ -run 'TestDecisionsRegistryEntries' -count=1`.
- [ ] 2.2 In `Config.Validate`'s `Provider == "jev"` block, reject a path that does not start with `/` or that contains `?` or `#`, with an error naming `ENGRAM_DECISIONS_PATH`. Add table cases: a missing leading slash, a full URL, a query string, a fragment, a valid `/v1/systemone`, and a provider-off case with a bad path that reports nothing. Verify: `go test ./internal/config/ -count=1`.

## 3. Wiring

- [ ] 3.1 Pass `jev.WithPath(cfg.Decisions.Path)` in `deciderFromConfig`, `searchDeciderFromConfig` and `understandDeciderFromConfig`. Pass it in `TestJevLive` too, and add the path to that test's log line. Verify: `go build ./...`.
- [ ] 3.2 Add a `decider_test.go` case: with `cfg.Decisions.Path = "/v1/systemone"`, a request from each of the three constructors reaches an `httptest` server at `/v1/systemone`; with the path empty, the request reaches `/api/alpha/decisions` as before. Verify: `go test ./internal/server/ -run 'Decider.*Path' -count=1`.

## 4. Helm chart and docs

- [ ] 4.1 Add `memory.decisions.path: ""` to `charts/engram/values.yaml`, rewrite the `baseURL` comment (OpenRouter, LiteLLM pass-through, and the LiteLLM native route with its path), and render `ENGRAM_DECISIONS_PATH` in `_helpers.tpl` with a `with` guard inside the provider guard. Add a `chart:validate` check that the default render emits no `ENGRAM_DECISIONS_PATH` and that `--set memory.decisions.provider=jev,memory.decisions.baseURL=https://x,memory.decisions.path=/v1/systemone` emits it. Verify: `task chart:validate`.
- [ ] 4.2 Update docs-site `guides/configure.md` (Typed decisions): the Base URL paragraph covers the path, with the LiteLLM native-route example (base `https://litellm.example.com`, path `/v1/systemone`, the model must be a model name configured in LiteLLM, and the key needs that model in its allow-list instead of the pass-through route grant), plus a new `ENGRAM_DECISIONS_PATH` table row. Add the `memory.decisions.path` row to `guides/deploy.md`. Verify: `go test ./internal/config/ -run 'Docs' -count=1` and `task lint:markdown`.

## 5. Integration

- [ ] 5.1 Live measurement before the deployment switches routes: run `task eval:decisions` with `ENGRAM_DECISIONS_BASE_URL=https://llm.fzymgc.house`, `ENGRAM_DECISIONS_PATH=/v1/systemone`, the LiteLLM model name and a key allowed to use it. Record whether noul, choice and score answers, `id`, `provider` and `usage.cost` come back. If no such key is available in this session, record that in the PR and leave the deployment on the pass-through route.
- [ ] 5.2 Quality gate: `task` (lint and test) and `task chart:validate` both pass.

## Workflow follow-up

- Open the PR naming #793.
- After the release, the fzymgc-house deployment switch is a separate selfhosted-cluster PR: set the three values, and move the engram key's grant from the pass-through route to the model allow-list.
- Archive the change with `/opsx:archive` after the PR merges.
