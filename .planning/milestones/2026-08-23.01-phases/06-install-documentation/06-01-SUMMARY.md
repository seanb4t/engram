---
phase: 06-install-documentation
plan: 01
subsystem: docs
status: complete
verification_status: pending-root-build-and-route-review
tags: [installation, homebrew, setup, mcp, documentation]
requires:
  - phase: 05-slash-command-delegation
    provides: Validated four-mode setup inputs and generated command reference
provides:
  - Canonical Install and Agent Setup guides with explicit unreleased availability
  - Quickstart, CLI, and plugin routes to canonical onboarding
affects: [06-02, install-documentation-verification]
tech-stack:
  added: []
  patterns: [Frontmatter-first Starlight guides, root-relative guide links]
key-files:
  created:
    - docs-site/src/content/docs/guides/install.md
    - docs-site/src/content/docs/guides/agent-setup.md
  modified:
    - docs-site/src/content/docs/guides/quickstart.md
    - docs-site/src/content/docs/guides/cli.md
    - docs-site/src/content/docs/guides/plugin.md
key-decisions:
  - Keep the dated v0.15.1 setup-unavailable notice and provide an intentional source-build route.
  - Link the generated native command reference instead of duplicating its argv table.
  - Leave build, rendered-route review, commits, and release acceptance with root as instructed.
requirements-completed: []
requirements-addressed: [REQ-docs-install-path, REQ-docs-setup-documented]
duration: 5min
completed: 2026-09-12
actuals:
  tokens: 5874
  tasks: 2
  commits: 0
---

# Phase 6 Plan 1: Installation and Agent Setup Guides Summary

**Homebrew and archive acquisition now lead to source-qualified MCP setup, with complete auth/runtime coverage and reconciled Quickstart, CLI, and plugin entry points.**

## Accomplishments

- Added Install with Homebrew first, the four v0.15.1 archive targets, selected-file checksum verification, extraction, a user-chosen PATH directory, JSON version checks, and executable-scoped macOS quarantine guidance.
- Added an intentional unreleased-source build route with a temporary output path and recorded checkout revision. Install, Agent Setup, and Plugin explicitly warn that v0.15.1 lacks setup; no minimum release is invented.
- Added Agent Setup coverage for endpoint preservation, binary detection, preview/review/apply, all four auth modes, the support matrix, non-secret client ID, inherited Claude secret environment, native bearer references, and generic-only token-file provenance.
- Explained separate registration/skills results, skill destinations and Codex AGENTS.md fallback, JSON use, all five outcomes, partial failure, repeat-run convergence, subsequent OAuth login, and the generic manual route for unsupported clients.
- Preserved Quickstart's Docker/configuration/first-memory path, Connect endpoint and credential precedence, standalone plugin installation, Claude-only fallback, and plugin hooks. Removed the obsolete plugin-only assertion and duplicated native argv table.

## Task Commit Handoff

Root owns all index operations and commits. This executor did not stage or commit.

1. **Task 1:** `docs(06-01): add installation and setup preview guides` — Install, initial Agent Setup, and Quickstart. Root commit observed: `33563d0b`.
2. **Root formatting follow-up:** `docs(06-01): fix Quickstart code block formatting` — observed as `83bbf6aa`; preserved.
3. **Task 2:** Agent Setup, CLI, and Plugin. Root commit observed: `301211a9`, `docs(06-01): document auth modes and canonical setup entry points`.
4. **Summary:** `.planning/phases/06-install-documentation/06-01-SUMMARY.md` — ready for root metadata commit.

The actuals token estimate is ceiling(23,493 changed-guide diff characters / 4), measured against starting revision `bef7323864a7349aac217db8c2e51fcd180327a9`. It includes root's observed Quickstart formatting fix and excludes this summary. `commits: 0` counts commits performed by this executor; root commits are listed separately above. The measured implementation/check interval was approximately 16:15–16:20 UTC, excluding initial context reads.

## Verification Results

| Check | Result |
| --- | --- |
| `go run ./cmd/engram setup --help` | Passed, exit 0. Compared URL/runtime/auth/client-ID/token-file and output prose against real help. No registration invoked. |
| `task lint:setup` | Passed, exit 0; existing read-only generated-reference check. |
| `git diff --check` | Passed. |
| Plan's targeted `rumdl check` commands | Exit 0, but repository exclusions filtered out all guide files; this is not evidence of lint coverage. |
| `rumdl check --no-exclude` on Install, Agent Setup, Plugin | Passed: no issues in three files. |
| Forced lint on Task 1 files | Initial Quickstart had three pre-existing MD031/MD040 findings; root's subsequent formatting commit was observed. |
| Forced lint on Task 2 files | Agent Setup and Plugin clean; CLI has two pre-existing findings, MD055/MD058 at line 126, from multiline table rows. Confirmed those rows in starting revision. Left unchanged by executor. |
| Final `rumdl check --no-exclude` on all five guides | Passed after root's Task 2 commit: no issues in five files. Root's edits resolved the CLI table findings. |
| `pnpm --dir docs-site build` | Pending with root, explicitly assigned outside this executor's network sandbox. |
| Rendered five-route and navigation review | Pending with root; not performed or claimed here. |

Source review covered the declared plan/context/research/patterns, Phase 5 D-17–D-20, Quickstart/CLI/Plugin, setup help and source, runtime plans and generic behavior, generated setup command, archive/cask hooks, and docs package/navigation/workflow. Source links and frontmatter were reviewed, but that does not establish rendered navigation. No live registration, install, release action, credential inspection, dependency preparation, package/lockfile/code edits, or memory writes were performed.

## Deviations from Plan

- Per explicit execution instructions, root handles commits, dependency preparation, docs builds, and rendered-route checks. Continued after the tested Task 1 handoff rather than waiting for the tracer build. The build portion of the tracer remains pending, not implicitly passed.
- Forced Markdown lint with `--no-exclude` after the planned command revealed it checked no guide files. Reported pre-existing findings separately; root independently fixed Quickstart formatting.
- Restricted writes to the five guides and this summary. No STATE, ROADMAP, REQUIREMENTS, VALIDATION, WINDOWS, or other planning files were changed by this executor.
- `status: complete` records the assigned documentation execution and handoff. It does not claim all plan verification passed or either requirement is accepted; `requirements-completed` remains empty pending root checks.

## Remaining Acceptance Work

Root must run the existing docs build with repository-selected pnpm 11.20.0 and the unchanged frozen lockfile, inspect all five rendered guide routes, and follow incoming Quickstart/CLI/Plugin links and the generated-reference link. The pre-existing CLI table lint findings were resolved in root's Task 2 commit; final forced lint passed all five guides.

Release acceptance belongs to 06-02: independently refresh release/tap identity and release-run evidence that upload was not skipped; establish a published binary containing the final setup command before replacing the unreleased notice; and record actual Homebrew installs on macOS/Linux × amd64/arm64 with version, resolved executable, completion results, and native/emulated context. Published archives, local builds, help checks, or cask source alone do not satisfy those observations. This executor did not begin 06-02.

## Self-Check: PASSED

All five guide files and this summary exist. The three observed root commits are present in local history. The summary commit remains root-owned. No TODO/FIXME/coming-soon stubs were found in the five guides; example endpoints and credential-provenance markers are intentional reader inputs, not incomplete implementation.

## Orchestrator verification completion

Root prepared dependencies with the project-selected pnpm and `install --frozen-lockfile` (no package/lockfile diff), built the final five guides successfully (21 routes), and inspected their rendered routes and 285 internal links with no missing target or anchor. Logs: `/tmp/engram-06-docs-deps.log`, `/tmp/engram-06-docs-build.log`, `/tmp/engram-06-rendered-link-check.txt`. Forced `rumdl check --no-exclude` passes all five pages; root fixed three pre-existing Quickstart fence findings and two broken CLI table rows. `task lint:setup` and read-only setup help pass. No code or dependency declaration changed, so no broad Go suite was rerun for this prose-only wave.

Content review confirms the acquisition/source-build availability warning, separate server and client paths, preview/review/apply order, full auth matrix and credential requirements, generic output limits and standalone plugin path. This is local documentation validation, not release or installation evidence. Pending root build/route checks described above are now resolved.
