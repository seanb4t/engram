# Proposal

## Why

The plugin's session-start hook cannot recall anything itself. It works out the memory scopes and prints instructions that tell the agent to call `list_rules` and `list_memory`, so every session spends two or more MCP calls on the bootstrap, and an agent that skips the instructions starts with no memory. Separately, the repository has two scope derivations that disagree: the hook's `skill/engram/hooks/lib/scope.py` and Go's `repoIdentityFromCWD` / `normalizeGitRemote` in `cmd/engram/spine_review_verify.go` produce different scopes for a remote with a port, and for a clone with no `origin` remote run from a subdirectory (engram gotcha `sea5nexbtw`). This change comes from #820, filed after an explore session on 2026-10-17.

## What Changes

- A new `engram prime` command writes a session primer for an agent: the scope it used, the rules index for that scope, a short recall digest of the most recent records in the spine and the workspace overlay, and a pointer to `engram catalog`.
- The scope is optional:
  - With `--scope`, or the `ENGRAM_SCOPE` environment variable, prime uses exactly that scope (the flag wins over the variable).
  - Otherwise it derives the scope from local git, with no network call and no `gh`: the spine is `repo:<host>/<org>/<repo>` from the `origin` remote, and in a linked worktree the overlay is `repo:<id>:ws:<workspace>`.
  - The primer always starts with the scope it used and where that came from: `--scope`, `ENGRAM_SCOPE`, the git `origin` remote, or the repository-directory-name fallback, which it marks as probably not a scope on the server.
  - Outside a git repository, with no override, prime states that no scope was derived, tells the user to pass `--scope`, prints no recall section, and exits 0. It never fails a session start.
- Prime writes a primer meant to be read as agent context by default, whether or not stdout is a terminal. This is a deliberate exception to the client commands' rule of JSON when stdout is not a terminal. A structured form stays available through `--output json`.
- One Go scope derivation replaces the two. It follows `scope.py`'s rules: it drops a numeric port, uses the main repository root's name when there is no `origin` remote, and derives the worktree overlay. `engram prime` and `engram spine-review verify` both use it.
  - **Bug fix:** `spine-review verify` now identifies the repository correctly for a remote with a port (for example `ssh://git@host:22/org/repo`) and when it runs from a subdirectory of a clone with no `origin` remote.
- The session-start hook runs `engram prime` when the `engram` binary is installed and has a credential, and puts its output in the session context. When the binary is missing, has no credential, fails or does not answer within the hook's timeout, the hook falls back to today's instruction text. `scope.py` stays only for that fallback, and its tests and the Go tests check the same table of cases.

## Capabilities

### New Capabilities

- `scope-derivation`: how engram derives the spine and overlay memory scopes from a working directory, shared by the CLI and the plugin hook.
- `session-primer`: what `engram prime` emits, how it chooses its scope, and how the session-start hook uses it and falls back.

### Modified Capabilities

None. No existing spec covers scopes, the CLI or the hook.

## Impact

- **Wire contract:** one new setting, `ENGRAM_SCOPE`, read only by `engram prime`. This is a compatibility decision. It is additive, and no existing command reads it. The scope values that the hook derives do not change, because the Go derivation adopts the hook's rules. `spine-review verify` can now derive a different repository identity than before in the two buggy cases above; that is the fix. No proto field, MCP input schema, error hint code or stored payload key changes.
- **Depends on:** change `cli-login` (#818), because prime calls the server and needs a credential. A server with no issuer configured works without login.
- **Code:** a new `internal/scope` package, `cmd/engram` (the `prime` command, and `spine_review_verify.go` moving to the new package), `internal/config` (the `ENGRAM_SCOPE` row), `internal/surfaces` (a blast-radius row for `prime`), the `help.golden` and `catalog.golden` fixtures, and the plugin's `skill/engram/hooks/session-start-memory-recall` with its Python tests.
- **Server:** deliberately unchanged. Prime uses the existing `ListRules` and `ListMemories` RPCs.
- **MCP tool schemas:** deliberately unchanged.
- **Connect proto:** deliberately unchanged.
- **CLI:** a new `prime` command; a corrected repository identity in `spine-review verify`.
- **Console (`ui/`):** deliberately unchanged.
- **Helm chart:** deliberately unchanged.
- **docs-site:** `guides/cli.md` (an `engram prime` section) and the plugin and session-start pages that describe the bootstrap.
- **engram skill:** the session-start hook changes as described. The skills' text is unchanged unless design.md finds a passage that describes the bootstrap.
- **CLAUDE.md memory contract:** deliberately unchanged. Rules still surface at session start as a one-line-per-rule index.
- **Dependencies:** none new.
- **Out of scope:** session-start hooks for runtimes other than Claude Code (Codex, opencode); using the derived scope as a default `--scope` for `engram search` and `engram list`; and a recall digest larger than the hook's current bounded digest.
