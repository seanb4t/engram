# Design

## Context

See proposal.md (Why) for the motivation. The current state:

- **The hook.** `skill/engram/hooks/session-start-memory-recall` is a `uv run --script` Python hook with a 10-second limit (`hooks/hooks.json`). It reads the session's `cwd` from the hook's stdin JSON, derives scopes with `hooks/lib/scope.py`, and prints `additionalContext`. That context holds the `Memory spine scope:` and `Memory workspace scope:` lines, the "REQUIRED FIRST ACTION" recall instruction, store-precedence and capture guidance, and a `list_rules` instruction for `rule:repo:<id>` plus `rule:project:<ENGRAM_PROJECT>`. Outside a repository it prints nothing. Its docstring says recall is model-mediated because a hook cannot hold the OAuth token.
- **`scope.py`.** It runs three git probes (`remote get-url origin`, `rev-parse --git-common-dir`, `rev-parse --show-toplevel`), each with a 5-second timeout. It normalises the remote, falls back to the parent directory name of the shared `.git` when there is no `origin`, and derives the overlay from the worktree's top-level directory name. `git rev-parse --git-common-dir` prints a path relative to the working directory (`../../.git` from `cmd/engram`), which `scope.py` resolves against `cwd`.
- **The Go derivation.** `cmd/engram/spine_review_verify.go` has `repoIdentityFromCWD` (origin remote, else `filepath.Base(cwd)`) and `normalizeGitRemote` (it turns the first colon into `/`, so a port becomes a path segment). `sameRepoAsCWD` compares the colon-segment after `repo` with that identity. The verify working root is the current directory, not the repository top level.
- **Skills read the scope lines by label.** `curating-memory`, `promoting-memory` and `migrating-from-beads` tell the agent to use the `Memory spine scope` and `Memory workspace scope` lines from session start.
- **The client lane.** `clientFromFlags` (`client_common.go`) loads `client.*` registry rows, resolves the credential (`ENGRAM_TOKEN`, then `--token-file`), and picks JSON when stdout is not a terminal. `cli-login` (#818) adds the login cache as a third credential source in the same function. Connect `ListMemories` returns records newest first (`Direction_Desc` on `created_at`) and applies the recall gate unless an `include_*` flag is set. `ListRules` takes explicit rule scopes and returns up to 1000 rules per scope.

## Goals / Non-Goals

**Goals:**

- One derivation, written once in Go, that reproduces `scope.py` exactly for every case the hook handles today.
- A primer that works from a hook: no prompt, no stdin, a hard deadline, and a failure that leaves stdout empty.
- A hook that is never worse than today: every failure lands on the current instruction text inside the hook's limit.

**Non-Goals:**

- Changing any scope value the hook derives today. Known odd cases in `scope.py` stay as they are (see Risks).
- Moving the `spine-review verify` working root to the repository top level.
- Designing `engram login` or the credential cache. That is `cli-login` (#818).

## Decisions

### D1: One Go package, `internal/scope`, ported from `scope.py`

`internal/scope` exposes the repository identity and the spine and overlay scopes for a directory. It runs the same three git probes with a context deadline, normalises the remote with `scope.py`'s rules (scheme, user info in the host segment only, numeric port dropped, scp colon replaced, `.git` and slashes trimmed), and reports "no scope" when git fails or the directory is not in a repository. `engram prime` and `engram spine-review verify` both call it. `normalizeGitRemote` and `repoIdentityFromCWD` are removed.

`spine-review verify` keeps its fail-safe outside a repository: an empty identity matches no `repo:` segment, so every citation stays `unverifiable`, as with today's directory-name guess.

Alternatives considered:

- **Keep two derivations and make them agree through a shared test table.** Rejected as the end state. Two implementations in the same language would still drift in code review. The shared table (D3) is kept only for the one pair that must stay in two languages.
- **Drop `scope.py` and have the hook call Go only.** Rejected. The hook must still work when the `engram` binary is missing, which is the common case for a plugin user who has not installed the CLI.
- **Have Go shell out to `scope.py`.** Rejected. The binary would depend on a Python runtime and the plugin directory.

### D2: Prime derives its own scope; the hook does not pass `--scope`

The hook runs `engram prime` with the session's `cwd` as the working directory and lets prime derive the scope. The hook still runs `scope.py` first, for two jobs only: deciding whether the session is in a repository at all (outside one, the hook stays silent and never runs prime), and supplying the scopes for the fallback text.

Alternatives considered:

- **The hook passes `--scope <spine>` from `scope.py`.** Rejected. The header would name `--scope` as the source, which is false, and `--scope` means "exactly this scope", so the overlay would be lost. Keeping the overlay would need a second flag only the hook uses.
- **The hook skips `scope.py` and reads prime's JSON to learn whether a scope exists.** Rejected. The hook would then have to render the text primer itself in Python, which duplicates prime's rendering.

### D3: A shared case table for the Go and Python tests

`internal/scope/testdata/cases.json` holds two lists:

- `remotes`: pairs of remote URL and expected repository identity (the HTTPS, port, SSH, scp-style and already-normalised cases from the spec).
- `layouts`: a layout kind (primary checkout, subdirectory, linked worktree, no origin from a subdirectory, no origin in a linked worktree, not a repository), an optional remote, and the expected spine and overlay.

The Go tests in `internal/scope` read the file directly. `skill/engram/hooks/tests/test_scope.py` reads it through a path relative to the repository root. Each suite builds real git repositories in temporary directories for the `layouts` cases. Both suites test their own derivation's behaviour against the same inputs. No test checks the other suite or that the two ran the same cases, so this is not a test of a test or a gate of a gate (rule `3p0zsqrhmb`). Existing `test_scope.py` cases that use the fake `_run` stay as they are.

Alternative considered: a cross-language conformance gate that runs both derivations and compares their output. Rejected under the same rule.

### D4: Text by default, whether or not stdout is a terminal

Prime's main consumer pipes its stdout into agent context, so the client lane's JSON-when-piped rule (D-05) would give the hook the wrong format. Prime registers its own `--output` flag whose help text says the default is `text` and that `json` is available. It does not reuse the `addClientFlags` usage text, which says "detect from stdout". That keeps the help correct by reading (engram principle `4aksmneehh`).

`--output json` writes one document:

```json
{
  "spine": {"scope": "repo:github.com/org/repo", "source": "git_origin_remote"},
  "overlay": {"scope": "repo:github.com/org/repo:ws:feat", "source": "linked_worktree"},
  "rule_scopes": ["rule:repo:github.com/org/repo"],
  "rules": [{"short_id": "...", "summary": "...", "tags": ["..."], "scope": "..."}],
  "memories": {"spine": [...], "overlay": [...]},
  "catalog_command": "engram catalog"
}
```

`spine` and `overlay` are `null` when there is none. `source` is one of `flag`, `env`, `git_origin_remote`, `repo_directory_name` and `linked_worktree`. Memory entries use the existing compact JSON shape that `engram list` writes (the same `renderJSON` path), so no new record shape is invented.

Alternative considered: JSON when stdout is not a terminal, as on every other client command, with the hook passing `--output text`. Rejected. Anyone who pipes prime into a file or an agent by hand would get JSON by surprise.

### D5: Primer layout

```text
# engram session primer

Memory spine scope: repo:github.com/org/repo
Spine scope source: git origin remote
Memory workspace scope: repo:github.com/org/repo:ws:feat
Workspace scope source: linked git worktree
Rule scopes: rule:repo:github.com/org/repo, rule:project:selfhosted-cluster

## Rules

- n6m4as49mr — MUST pass an explicit pathspec ... [git, workflow]

## Repo-wide memories

- tjfn1dqjdh — jj retired 2026-10-17 ...
- (10 shown; older memories are reachable with search_memory or a higher list_memory limit)

## This workspace's memories

- ...

Machine-readable command catalog: engram catalog
```

The `Memory spine scope:` and `Memory workspace scope:` lines keep today's labels and carry the scope value alone, because three skills read them by label. The source goes on its own line. The fallback source reads "repository directory name (no origin remote; probably not a scope on the server)".

Prime's text is runtime-neutral: it names the MCP tools by their bare names (`search_memory`, `get_memory`), never with a Claude-specific `mcp__engram__` prefix. The Claude Code hook adds its own guidance around it (D9).

The pointer names `engram catalog`, which `cli-tui` (#819) introduces. If this change is implemented before `cli-tui` merges, the pointer names bare `engram` and is updated when the two meet.

### D6: Settings

| Key | Env | Flag | Default | Validation | Helm |
|---|---|---|---|---|---|
| `prime.scope` | `ENGRAM_SCOPE` | `--scope` | empty (derive) | non-blank when the flag is passed; no `rule:` or `discovery:` prefix | none |
| `prime.project` | `ENGRAM_PROJECT` | none | empty (no project rules) | trimmed; empty means unset | none |

- Neither key has a Helm value: both are client-side and the chart deploys only the server, as with `ENGRAM_SERVER_URL`.
- `ENGRAM_PROJECT` already exists as a hook-only variable with the same meaning. Registering it in Go is additive and keeps the hook's project-rule behaviour on the primer path. This is a compatibility decision: no existing Go command reads it, and the hook's reading is unchanged.
- An explicitly empty `--scope ""` is detected with the flag's `Changed` state, because the registry reads an empty value as "unset". Rejecting it follows the rule that an explicitly passed argument is never silently dropped.
- These are CLI-local usage errors. They exit 2 with a message that names the flag or variable, in the CLI's existing usage-error form. Only server rejections carry the `field=` / `hint=` envelope, which prime passes through unchanged.

### D7: Server, credential and deadline

Prime uses `clientFromFlags`, so it gets `--server` / `ENGRAM_SERVER_URL`, `--insecure`, `--timeout` and the credential order with no second resolver. When `cli-login` lands, the login cache arrives through the same function with no change here. Prime never starts a login and never reads stdin. A token refresh inside the credential chain counts against prime's deadline.

`--timeout` (default `30s`) is one deadline for the whole command, git probes and RPCs together, not per request. Outside a repository with no override, prime returns before it builds a client, so it needs no server and no credential there.

### D8: Calls and rendering

Prime makes up to three RPCs at once under the deadline: `ListRules` with the rule scopes (compact), `ListMemories` for the spine (`limit` 10, compact, recall gate on), and `ListMemories` for the overlay when one exists. It renders only after all of them succeed, so a failure leaves stdout empty. Overlay records whose id is in the spine result are dropped. A group with exactly 10 records gets the "older memories" note, matching the hook's rule.

Errors go through the existing connect-error classifier, so the exit codes are the client taxonomy (2, 3, 5, 6, 10).

### D9: The hook's flow and time budget

```text
read stdin JSON -> cwd
scope.py derive (probe timeout 1.5 s each, at most 3 probes)
  no repo         -> print nothing, exit 0                 (unchanged)
  repo, no engram -> fallback text                          (today's text)
  repo, engram    -> run `engram prime --timeout 3s`, cwd=cwd,
                     stdin=/dev/null, subprocess timeout 4 s
                       exit 0 + output -> primer + hook guidance
                       anything else   -> fallback text
```

The worst case is about 4.5 s of probes plus 4 s for prime, inside the 10 s limit with room for `uv` start-up. Lowering the probe timeout from 5 s to 1.5 s is a small change to the fallback path: today's worst case (three 5 s probes) already exceeds the hook limit, and a local git probe normally takes milliseconds.

On the primer path the hook injects prime's output, then a short paragraph saying that the rules and recent memories above were already recalled, that the rules are MUST-follow, and that a rule's full text comes from `get_memory(<short_id>)`, then today's store-precedence and capture paragraphs. It drops the "REQUIRED FIRST ACTION" paragraph and its 401 guidance. The docstring changes to describe both paths.

### D10: Blast radius

`prime` gets an `internal/surfaces` row: read-only, not destructive, idempotent, not open-world, like `list` and `migration-status`. The catalog golden gains its entry through `task surfaces:gen`.

## Risks / Trade-offs

- [The hook's environment has no `ENGRAM_SERVER_URL`] → prime exits 2 and the hook always falls back, so the primer never shows. Mitigation: `cli-login` (#818) records the last login's server as the default server, and `clientFromFlags` reads it after `--server` and `ENGRAM_SERVER_URL`, so a logged-in machine needs no variable. `guides/plugin.md` still documents `ENGRAM_SERVER_URL` for a machine that uses `ENGRAM_TOKEN` or `--token-file` instead of a login.
- [The CLI credential and the MCP credential belong to different users] → the primer shows one owner's memories and the agent writes as another. Mitigation: both use the same IdP realm. `guides/plugin.md` notes that the two logins must be the same user.
- [A globally exported `ENGRAM_SCOPE` applies to every repository] → the header names `ENGRAM_SCOPE` as the source, so the cause is visible in every primer.
- [Every session start now makes up to three network calls] → bounded by the 3 s deadline, and every failure falls back to today's text.
- [An scp-style remote whose path starts with digits (`git@host:1234/repo.git`) is read as a port] → kept for parity with the hook. It is pinned as a case in the shared table so a later fix changes both sides at once.
- [Remoteless submodules and `file://` remotes give odd identities in `scope.py`] → unchanged and out of scope. They are not in the shared table, so nothing pins them.
- [The plugin is newer than the installed binary] → an `engram` without `prime` exits non-zero and the hook falls back.

## Migration Plan

Nothing to migrate: no stored payload or server behaviour changes. A user who has not installed the binary, or who has no credential, keeps today's hook behaviour. Rollback is a revert. A broken prime in the field degrades the hook to its fallback, not to an empty session.

## Open Questions

- The exact wording of the hook's primer-path paragraph (D9) can be settled during implementation. It changes no spec, approach or task.
