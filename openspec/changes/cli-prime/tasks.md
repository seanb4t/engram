# Tasks

## 1. Shared scope derivation in Go

- [x] 1.1 Create `internal/scope/testdata/cases.json` with the `remotes` list (HTTPS, HTTPS with port 443, `ssh://` with port 22, `ssh://` without a port, scp-style, an already-normalised `github.com/o/r`, and scp-style `git@host:1234/repo.git` read as a port) and the `layouts` list (primary checkout, subdirectory, linked worktree, no origin from a subdirectory, no origin in a linked worktree, not a repository), each with its expected identity, spine and overlay from `specs/scope-derivation/spec.md`. Verify: `python3 -m json.tool internal/scope/testdata/cases.json` parses it.
- [x] 1.2 Add `internal/scope` (design D1): remote normalisation with `scope.py`'s rules, the three git probes under a context deadline with a relative `--git-common-dir` resolved against the working directory, the no-origin fallback, the worktree overlay, and "no scope" when git fails or the directory is outside a repository. Verify: `go build ./...`.
- [x] 1.3 Add Go tests that read `cases.json`: a table test over `remotes`, and a test that builds each `layouts` case as a real git repository (with `git worktree add` for the linked cases) in `t.TempDir()` and checks the derived scopes; plus a case with `PATH` set so `git` cannot run. Verify: `go test ./internal/scope/ -count=1`.

## 2. Hook derivation against the shared table

- [x] 2.1 Add tests to `skill/engram/hooks/tests/test_scope.py` that read `internal/scope/testdata/cases.json` through a path relative to the repository root and check `_normalize_remote` over `remotes` and `derive_scopes` over `layouts` built as real git repositories in `tmp_path`. Keep the existing fake-`_run` tests. Verify: `task test:python`.
- [x] 2.2 Lower the `scope.py` probe timeout from 5 s to 1.5 s (design D9), with no change to any derived value. Verify: `task test:python` and `task lint:python`.

## 3. spine-review verify on the shared derivation

- [x] 3.1 Replace `repoIdentityFromCWD` and `normalizeGitRemote` in `cmd/engram/spine_review_verify.go` with `internal/scope`, keeping `sameRepoAsCWD` and the current-directory working root. Outside a repository the identity is empty. Move `TestNormalizeGitRemote`'s cases into `cases.json` if any are missing there, and delete the test. Verify: `go test ./cmd/engram/ -run 'Verify|SameRepo' -count=1`.
- [x] 3.2 Add `spine_review_verify_test.go` cases for the spec's three verify scenarios: an `ssh://` remote with a port is the same repository, a subdirectory of a clone with no origin is the same repository, and outside a repository every citation is `unverifiable`. Verify: `go test ./cmd/engram/ -run 'Verify' -count=1`.

## 4. Prime settings

- [x] 4.1 Add the `prime.scope` (`ENGRAM_SCOPE`, `--scope`) and `prime.project` (`ENGRAM_PROJECT`, no flag) registry rows with the comments design D6 calls for, and a validator for the scope rules (blank when the flag was passed, `rule:` or `discovery:` prefix) whose message names the flag or the variable. Verify: `go test ./internal/config/ -count=1` with new table cases for each rejection and for flag-over-env precedence.

## 5. The prime command

- [x] 5.1 Add `cmd/engram/prime.go`: the command, the client flags from `clientFromFlags` with prime's own `--output` help text (default `text`), scope resolution (flag, env, derived) with its source, rule scopes (`rule:repo:<repo-id>` from the `repo` segment, plus `rule:project:<ENGRAM_PROJECT>`), and the outside-a-repository primer that returns before any client is built. Verify: `go build ./...` and `go run ./cmd/engram prime --help` shows `--scope` naming `ENGRAM_SCOPE` and `--output` naming `text` as the default.
- [x] 5.2 Add the three concurrent RPCs under one deadline, the overlay de-duplication, the 10-record limit and its footer, the text renderer with the `Memory spine scope:` / `Memory workspace scope:` labels and separate source lines (design D5), and the `--output json` document (design D4). Render only after every call succeeds. Verify: `go test ./cmd/engram/ -run 'Prime' -count=1` with cases against `startStubServer` (add a `ListRules` method to `stubEngramService`) for: primer order, header sources, overrides and precedence, the four rejections, rules index present and omitted, full spine footer, empty overlay omitted, outside a repository with no request reaching the stub, piped output stays text, JSON on request, an unknown `--output`, a request that sets no `include_*` flag and `full=false`, and the auth (3), unavailable (5), deadline (6) and missing-server (2) failures with empty stdout.
- [x] 5.3 Add an `internal/e2e` test that runs the built binary's `engram prime` against a real server and store: archived, superseded, expired and scheduled records stay out of the digest, another owner's `shared` record appears and a `private` one does not. Verify: `task test:e2e`.
- [x] 5.4 Add a test that prime completes with stdin attached to a pipe that never closes. Verify: `go test ./cmd/engram/ -run 'PrimeStdin' -count=1`.
- [x] 5.5 Add the `prime` row to `internal/surfaces/toolclass.go` (read-only, idempotent, not destructive, not open-world), then run `task surfaces:gen` to regenerate the anchored regions and the `help.golden` and `catalog.golden` fixtures. Verify: `git diff` on the goldens shows only the new `prime` entry and its flags, and `go test ./cmd/engram/ -run 'Golden|Catalog' -count=1` passes.
- [x] 5.6 Point the primer's catalog line at the catalog command on `main` when this task runs: `engram catalog` if `cli-tui` (#819) has merged, otherwise bare `engram`. Verify: the `Prime` test asserts the line matches the command that runs the catalog in the live cobra tree.
- [ ] 5.7 Add an `engram prime` section to docs-site `guides/cli.md`: what it emits, the scope sources and precedence, `ENGRAM_SCOPE` and `ENGRAM_PROJECT`, text by default and `--output json`, the outside-a-repository behaviour, and the exit codes. Verify: `task lint:markdown`, and the documented commands run as written against a local `engram serve`.

## 6. Session-start hook on the primer

- [x] 6.1 Change `skill/engram/hooks/session-start-memory-recall` to the flow in design D9: derive with `scope.py`, stay silent outside a repository, run `engram prime --timeout 3s` from the session's `cwd` with stdin from `/dev/null` and a 4 s subprocess timeout when `engram` is on `PATH`, and inject the primer plus the primer-path paragraph and today's precedence and capture paragraphs. Every other outcome injects today's text unchanged. Update the docstring. Verify: `task lint:python`.
- [x] 6.2 Add `test_session_start_memory_recall.py` cases with a stub `engram` script placed first on `PATH`: prime succeeds (context holds the primer and the `Memory spine scope:` line and no `list_memory` instruction), `engram` missing, exit 3, exit 1 for an unknown command, empty output, and a stub that sleeps past the timeout (the hook falls back and finishes under the 10 s limit). Keep `test_non_repo_is_silent` passing unchanged. Verify: `task test:python`.
- [x] 6.3 Update the SessionStart section of docs-site `guides/plugin.md`: the primer path, its fallback, that the hook finds the server through the default server that `engram login` records, or through `ENGRAM_SERVER_URL` in the environment that Claude Code runs hooks in, and that the CLI and MCP logins must be the same user. Verify: `task lint:markdown`.
- [x] 6.4 Check the skills that read the scope lines (`curating-memory`, `promoting-memory`, `migrating-from-beads`) against both hook paths. Their text should need no change, because the labels are unchanged. If any passage describes the bootstrap as model-mediated only, update it under `skill/engram/skills/` and run `task skills:vendor`. Verify: `go test ./internal/skills/ -count=1`.

## 7. Integration

- [ ] 7.1 End-to-end check in this repository with a locally built binary on `PATH` and `ENGRAM_SERVER_URL` set: pipe a SessionStart JSON payload with this checkout's `cwd` into the hook and confirm the injected context holds the real rules index and digest. Then repeat with no credential and confirm the fallback text. Record both outputs in the PR.
- [ ] 7.2 Quality gate: `task` (lint and test) passes.

## Workflow follow-up

- Open the PR naming #820, after `cli-login` (#818) has merged.
