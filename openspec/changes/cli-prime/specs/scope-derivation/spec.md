# Spec Delta

## Purpose

Defines how engram derives the spine and workspace-overlay memory scopes from a working directory, using local git only, so that the CLI and the plugin's session-start hook agree on the scope an agent reads from and writes to.

## ADDED Requirements

### Requirement: Spine scope from the origin remote

engram SHALL derive the spine scope as `repo:<repo-id>`, where `<repo-id>` is the `origin` remote URL with the scheme removed, user info removed, a numeric port removed, the colon of an scp-style `host:path` replaced by `/`, and a trailing `.git` and surrounding slashes removed. Derivation SHALL use local git only and SHALL make no network call.

#### Scenario: HTTPS remote

- **WHEN** the `origin` remote is `https://github.com/org/repo.git`
- **THEN** the spine scope is `repo:github.com/org/repo`

#### Scenario: HTTPS remote with a port

- **WHEN** the `origin` remote is `https://github.com:443/org/repo.git`
- **THEN** the spine scope is `repo:github.com/org/repo`

#### Scenario: SSH URL with a port

- **WHEN** the `origin` remote is `ssh://git@host.example:22/org/repo`
- **THEN** the spine scope is `repo:host.example/org/repo`

#### Scenario: SSH URL without a port

- **WHEN** the `origin` remote is `ssh://git@github.com/org/repo`
- **THEN** the spine scope is `repo:github.com/org/repo`

#### Scenario: scp-style remote

- **WHEN** the `origin` remote is `git@github.com:org/repo.git`
- **THEN** the spine scope is `repo:github.com/org/repo`

#### Scenario: Same spine from every directory of the repository

- **WHEN** the scope is derived from the repository root, from a subdirectory, and from a linked worktree of the same clone
- **THEN** all three derive the same spine scope

### Requirement: Spine scope without an origin remote

When a git repository has no `origin` remote, engram SHALL derive the spine scope as `repo:<name>`, where `<name>` is the directory name of the main repository root (the directory that holds the shared `.git`). The result SHALL NOT depend on which subdirectory or linked worktree the derivation runs from.

#### Scenario: Subdirectory of a clone with no origin remote

- **WHEN** the scope is derived from `/home/u/myrepo/cmd/tool` and `/home/u/myrepo` is a git repository with no `origin` remote
- **THEN** the spine scope is `repo:myrepo`

#### Scenario: Linked worktree of a clone with no origin remote

- **WHEN** the scope is derived from the linked worktree `/home/u/wt/feat` of the repository `/home/u/myrepo`, which has no `origin` remote
- **THEN** the spine scope is `repo:myrepo`

### Requirement: Workspace overlay in a linked worktree

In a linked git worktree, engram SHALL derive a workspace overlay scope `<spine>:ws:<workspace>`, where `<workspace>` is the directory name of the worktree's top level. In the primary checkout it SHALL derive no overlay.

#### Scenario: Linked worktree

- **WHEN** the scope is derived from the linked worktree `/repo/_wt/feat` of a repository whose `origin` remote is `git@github.com:org/repo.git`
- **THEN** the spine scope is `repo:github.com/org/repo` and the overlay scope is `repo:github.com/org/repo:ws:feat`

#### Scenario: Primary checkout

- **WHEN** the scope is derived from the primary checkout of the same repository
- **THEN** the spine scope is `repo:github.com/org/repo` and there is no overlay scope

### Requirement: No scope outside a repository

When the working directory is not inside a git repository, or git cannot be run, engram SHALL derive neither a spine nor an overlay scope. It SHALL NOT substitute the working directory's name or any other guess.

#### Scenario: Not a git repository

- **WHEN** the scope is derived from `/tmp/scratch`, which is not inside a git repository
- **THEN** no spine scope and no overlay scope are derived

#### Scenario: git is not installed

- **WHEN** the scope is derived and no `git` executable can be run
- **THEN** no spine scope and no overlay scope are derived

### Requirement: One derivation for every consumer

The session-start hook's fallback, `engram prime` and `engram spine-review verify` SHALL derive the same repository identity, spine scope and overlay scope for the same working directory.

#### Scenario: Hook and CLI agree

- **WHEN** the hook's fallback and `engram prime` derive scopes from the same directory, for every remote form and repository layout in the scenarios above
- **THEN** both derive the same spine scope and the same overlay scope

#### Scenario: spine-review verify with a remote that has a port

- **WHEN** `engram spine-review verify` runs in a clone whose `origin` remote is `ssh://git@host.example:22/org/repo` and a record in scope `repo:host.example/org/repo` cites a file
- **THEN** the citation is checked against the working tree and is not classified `unverifiable` with reason `different repo`

#### Scenario: spine-review verify from a subdirectory of a clone with no origin remote

- **WHEN** `engram spine-review verify` runs from `/home/u/myrepo/cmd` in a clone with no `origin` remote and a record in scope `repo:myrepo` cites a file
- **THEN** the citation is checked against the working tree and is not classified `unverifiable` with reason `different repo`

#### Scenario: spine-review verify outside a repository

- **WHEN** `engram spine-review verify` runs outside any git repository
- **THEN** every citation is classified `unverifiable`, as before this change
