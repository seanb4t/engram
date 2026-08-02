<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright 2026 Sean Brandt -->

---
phase: 07-cli-cross-spine-wiring
plan: 03
subsystem: cli
tags: [cross-spine, cli, docs, self-describe-catalog, discoverability]
status: complete

requires:
  - phase: 07-cli-cross-spine-wiring (plans 01-02)
    provides: validateScopeCrossSpine, renderCoverageFooter, --cross-spine on searchCmd and listCmd
provides:
  - TestCatalogCarriesCrossSpineGuidance (D-07 guidance-content gate)
  - docs-site/guides/cli.md recall-scope-selection section and coverage-footer documentation
  - docs-site/guides/upgrade.md v0.12.0 item 6
  - CLAUDE.md cross_spine sentence naming the CLI as a reachable surface
affects: [docs-site, cmd/engram]

actuals:
  tokens: 2543
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "guidance-content pinning: assert catalog Usage strings are literally equal to the live cobra Flags().Lookup(name).Usage, not merely non-empty (extends the TestCatalogEnumeratesEveryFlag idiom from presence to content)"

key-files:
  created: []
  modified:
    - cmd/engram/catalog_test.go
    - docs-site/src/content/docs/guides/cli.md
    - docs-site/src/content/docs/guides/upgrade.md
    - CLAUDE.md

key-decisions:
  - "No catalog implementation task, as the plan specified: research verified collectFlags derives from the live pflag.FlagSet with zero hardcoded flag list, so --cross-spine and its guidance were already catalog-visible before this plan touched anything. Task 1 only added a test pinning the guidance content."
  - "cli.md's three-verbs table search and list example rows were fixed to carry --scope (they previously showed an invocation that exits 2 both before and after this phase) — this was a pre-existing documentation defect the plan's action text explicitly called out as in-scope to fix, not a new deviation."

requirements-completed: [D-00, D-07, D-08, D-09]

coverage:
  - id: D1
    description: "Catalog carries --cross-spine on both search and list with the same guidance string --help shows, asserted by a test rather than assumed (D-07)"
    requirement: "D-07"
    verification:
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestCatalogCarriesCrossSpineGuidance"
        status: pass
    human_judgment: false
  - id: D2
    description: "docs-site CLI reference states the scope-or-cross-spine rule, the mutual exclusion, the exit code, and the coverage footer; the list example no longer shows an invocation that exits 2 (D-08)"
    requirement: "D-08"
    verification:
      - kind: other
        ref: "task lint (rumdl) + task license:check, both exit 0; rg -c 'cross-spine' cli.md = 4; rg -c 'searched_scopes' cli.md = 3"
        status: pass
    human_judgment: true
    rationale: "Documentation legibility ('a reader who has never run the command can determine the rule') is a prose-quality judgment a grep count cannot fully certify."
  - id: D3
    description: "Upgrade guide carries a sixth v0.12.0 item for CLI cross-spine reach, with the lead count widened to match (D-08)"
    requirement: "D-08"
    verification:
      - kind: other
        ref: "rg -c '^### 6\\.' upgrade.md = 1; rg -c 'six changes' upgrade.md = 1"
        status: pass
    human_judgment: false
  - id: D4
    description: "CLAUDE.md's Memory contract cross_spine sentence names the CLI as a reachable surface, with no flag syntax (D-09)"
    requirement: "D-09"
    verification:
      - kind: other
        ref: "rg -c 'cross_spine' CLAUDE.md = 1 (sentence names the CLI); rg -c 'cross-spine' CLAUDE.md = 0"
        status: pass
    human_judgment: false

duration: ~20min
completed: 2026-08-02
---

# Phase 7 Plan 03: CLI Cross-Spine Wiring (discoverability close) Summary

**Closed D-00's correct-by-reading bar for the phase: pinned the self-describe catalog's cross-spine
guidance with a new test (zero catalog production code, as D-07 predicted), documented the
scope-or-cross-spine rule and coverage footer in the docs-site CLI reference and upgrade guide, and
recorded CLI reachability in CLAUDE.md without teaching flag syntax — then ran the full phase-close
gate set green on the final tree.**

## Performance

- **Duration:** ~20 min
- **Tasks:** 3/3 completed
- **Files modified:** 4 (`cmd/engram/catalog_test.go`, `docs-site/src/content/docs/guides/cli.md`,
  `docs-site/src/content/docs/guides/upgrade.md`, `CLAUDE.md`)

## Accomplishments

- Verified, with observed evidence rather than assumption, that D-07's open question resolves in the
  favorable direction: the self-describe catalog absorbed `--cross-spine` on both `search` and `list`
  with zero catalog code across all three plans in this phase (`git diff --stat` for this plan names
  exactly `cmd/engram/catalog_test.go`).
- Added `TestCatalogCarriesCrossSpineGuidance`, which asserts the catalog's `--cross-spine`/`--scope`
  usage strings are **literally equal** to the live `cobra` command's `Flags().Lookup(name).Usage` —
  making "an agent's discovery path is not strictly worse than a human's" a checked property, not a
  hope.
- Fixed a pre-existing documentation defect in `docs-site/guides/cli.md`: the three-verbs table's
  `search` and `list` example rows previously omitted `--scope`, so copying them verbatim produced an
  invocation that exits 2 both before and after this phase. Both rows now carry `--scope`.
- Added a "Recall scope selection" section to `cli.md` stating the mutual-exclusion rule, the
  client-side pre-flight rejection (exit 2, before any network call), and why the CLI is deliberately
  stricter than the server on the scope+cross-spine combination.
- Extended `cli.md`'s output-contract section with the exact coverage-footer lines (`searched_scopes:
  <n>` / `searched_scopes: <n>  scopes_truncated: true`), sourced verbatim from 07-02-SUMMARY.md's
  recorded output.
- Widened `upgrade.md`'s `## v0.12.0` section from five items to six, adding `### 6.` documenting that
  the CLI now reaches cross-spine recall, following the existing items' "what changed / who is
  affected / what action is needed" voice and the v0.7.10 precedent of stating the change is purely
  additive.
- Amended CLAUDE.md's Memory contract `cross_spine` sentence to name the CLI as a second surface that
  reaches the capability, without adding flag spelling, an example invocation, or usage instructions
  (D-09's "include it, just not how to use it" constraint, verified by `rg -c 'cross-spine' CLAUDE.md`
  returning 0).
- Ran the full phase-close gate set on the final tree: `task`, `go vet ./...`, `task license:check`,
  `task chart:validate`, `task proto:lint`, `task proto:gen` zero-drift, `task ui:build` zero-drift,
  the zero-new-dependency gate, and the `internal/` containment gate — all green, all recorded below.

## Task Commits

1. **Task 1: Verify the catalog absorbed the flag, and pin the guidance it carries** — `d3f47669`
   (test)
2. **Task 2: Document the rule in the CLI reference and the upgrade guide** — `6a32bb01` (docs)
3. **Task 3: Record CLI reachability in CLAUDE.md, then run the phase-close gates** — `d6f00970`
   (docs)

No separate plan-metadata commit was made prior to this SUMMARY; the final metadata commit (this
file plus STATE.md/ROADMAP.md, per the orchestrator's bookkeeping) is deferred to the orchestrator per
this session's explicit instruction that STATE.md/ROADMAP.md are hand-maintained and out of scope for
this executor run.

## Files Created/Modified

- `cmd/engram/catalog_test.go` — added `TestCatalogCarriesCrossSpineGuidance` (94 lines), the D-07
  guidance-content gate.
- `docs-site/src/content/docs/guides/cli.md` — fixed the `search`/`list` example rows, added "Recall
  scope selection", extended the output contract with the coverage footer.
- `docs-site/src/content/docs/guides/upgrade.md` — widened the `## v0.12.0` lead sentence from five to
  six changes, added `### 6.` for CLI cross-spine reach.
- `CLAUDE.md` — amended the `cross_spine` Memory-contract sentence to name the CLI.

## Decisions Made

- No catalog implementation task was needed, exactly as the plan's `<no_catalog_implementation>`
  block predicted: `collectFlags` (`cmd/engram/catalog.go:109-128`) derives from the live
  `pflag.FlagSet` with no hardcoded flag list, so `--cross-spine` was catalog-visible the moment plans
  07-01/07-02 added it. Task 1 is verification-plus-a-guidance-assertion, not implementation — the
  `git diff --stat` for this task naming exactly one file is the verdict itself.
- The `cli.md` three-verbs-table fix (adding `--scope` to the `search`/`list` example rows) was
  explicitly authorized by the plan's action text as an inherited pre-existing defect this phase must
  not leave behind, not a new deviation requiring separate justification.

## Deviations from Plan

None — plan executed exactly as written. The one item worth flagging is not a deviation in outcome:
the plan's action text for Task 2 said "Update the frontmatter `description` (:3) if it enumerates
capabilities in a way this addition falsifies." The existing frontmatter description does not
enumerate exhaustively and is not falsified by adding cross-spine recall, so it was left unchanged —
this was a conditional instruction whose condition did not trigger, not a skipped step.

## Auth Gates

None encountered.

## Known Stubs

None. This plan changed no executable behavior — Task 1 added a test, Tasks 2-3 edited Markdown.

## Threat Flags

None. Both STRIDE entries the plan's `<threat_model>` disposed as `mitigate` (T-07-07 information
disclosure via the footer, T-07-08 repudiation via mischaracterizing the client guard as
authorization) are satisfied exactly as specified: `cli.md` states the footer reports a count, never
scope names, and frames the guard as a client-side pre-flight rejection with an exit code, never as
access control.

## D-00 Manual Evidence — verbatim `--help` excerpts

Both flags name each other by literal `--flag` spelling, on both commands, confirmed by direct
inspection (mechanical half already pinned by `TestScopeCrossSpineFlagsNameEachOther` from plans
07-01/07-02; this is the legibility judgment call 07-VALIDATION.md marks manual-only):

```text
$ engram search --help
...
      --cross-spine             span every scope you can read; mutually exclusive with --scope
...
      --scope string            limit recall to one scope; omit and pass --cross-spine to span every scope you can read; mutually exclusive with --cross-spine
...
```

```text
$ engram list --help
...
      --cross-spine             span every scope you can read; mutually exclusive with --scope
...
      --scope string            limit recall to one scope; omit and pass --cross-spine to span every scope you can read; mutually exclusive with --cross-spine
...
```

## D-07 Verdict — observed catalog entries

`go run ./cmd/engram | jq` filtered to the two flags under assertion, both commands:

```json
{
  "commands": [
    {
      "name": "list",
      "flags": [
        { "name": "cross-spine", "type": "bool", "default": "false",
          "usage": "span every scope you can read; mutually exclusive with --scope" },
        { "name": "scope", "type": "string", "default": "",
          "usage": "limit recall to one scope; omit and pass --cross-spine to span every scope you can read; mutually exclusive with --cross-spine" }
      ]
    },
    {
      "name": "search",
      "flags": [
        { "name": "cross-spine", "type": "bool", "default": "false",
          "usage": "span every scope you can read; mutually exclusive with --scope" },
        { "name": "scope", "type": "string", "default": "",
          "usage": "limit recall to one scope; omit and pass --cross-spine to span every scope you can read; mutually exclusive with --cross-spine" }
      ]
    }
  ]
}
```

D-07 is answered: the catalog carries `--cross-spine` on both commands with the exact same guidance
string `--help` shows, by construction (both derive from the same `pflag.Flag.Usage` field) and now
by pinned test.

## Phase-Close Gate Results

All commands below were run against the final tree (commit `d6f00970`).

| Gate | Command | Result |
|------|---------|--------|
| Task 1 named tests | `go test ./cmd/engram/... -run 'TestCatalogCarriesCrossSpineGuidance\|TestCatalogEnumeratesEveryFlag\|TestCatalogExitCodesMatchMapper\|TestCatalogDocumentsFlagParseExitCode' -v \| rg '^--- (PASS\|FAIL\|SKIP)'` | 4x `--- PASS`, 0 FAIL/SKIP |
| Task 1 new-test grep | `rg -c '^func TestCatalogCarriesCrossSpineGuidance' cmd/engram/catalog_test.go` | `1` |
| Task 1 diff containment | `git diff --stat` (Task 1 alone) | exactly `cmd/engram/catalog_test.go` |
| Task 1 catalog presence (jq) | `go run ./cmd/engram \| jq -e '... cross-spine ... length == 2'` | exit 0 |
| Task 1 catalog usage non-empty (jq) | `go run ./cmd/engram \| jq -e '... length == 4'` | exit 0 |
| Task 1 package isolation | `go test ./cmd/engram/...` | `ok` |
| Task 2 lint | `task lint` (includes rumdl over docs-site Markdown) | all green |
| Task 2 license | `task license:check` | 0 invalid, 241 valid |
| Task 2 cross-spine mentions | `rg -c 'cross-spine' docs-site/.../cli.md` | `4` (>= 4 required) |
| Task 2 searched_scopes mentions | `rg -c 'searched_scopes' docs-site/.../cli.md` | `3` (>= 1 required) |
| Task 2 upgrade item count | `rg -c '^### 6\.' docs-site/.../upgrade.md` | `1` |
| Task 2 upgrade lead count | `rg -c 'six changes' docs-site/.../upgrade.md` | `1` |
| Task 2 list-example fix | `rg -n 'engram list --server' docs-site/.../cli.md` | one occurrence, carries `--scope <scope>` |
| Task 2 diff containment | `git diff --stat` (Task 2 alone) | exactly the two docs-site files |
| Task 3 CLAUDE.md fact | `rg -c 'cross_spine' CLAUDE.md` | `1`, sentence names the CLI |
| Task 3 CLAUDE.md no flag syntax | `rg -c 'cross-spine' CLAUDE.md` | `0` |
| Task 3 catalog gates re-run | `go test ./cmd/engram/... -run 'TestCatalogEnumeratesEveryFlag\|TestCatalogExitCodesMatchMapper\|TestCatalogDocumentsFlagParseExitCode' -v \| rg -c '^--- PASS'` | `3` |
| `task` (lint + full suite) | | all green |
| `go vet ./...` | | exit 0 |
| `task license:check` | | 0 invalid |
| `task chart:validate` | | `chart:validate: OK`, 1 chart linted, 0 failed |
| `task proto:lint` | | passed (idempotency-level ban also passed) |
| `task proto:gen` zero-drift | `task proto:gen && git diff --exit-code -- gen/` | clean |
| `task ui:build` zero-drift | `task ui:build && git diff --exit-code -- web/` | clean (no `web/` diff produced) |
| Zero new deps | `git diff --exit-code b4544d47 -- go.mod go.sum` | clean (`DEPS_CLEAN`) |
| `internal/` containment | `git diff --name-only b4544d47 -- internal/` | exactly `internal/server/tools.go` |
| `engram search --help` sibling naming | manual read | `--scope` names `--cross-spine` and vice versa |
| `engram list --help` sibling naming | manual read | `--scope` names `--cross-spine` and vice versa |

## Commits

- `d3f47669` — `test(07-03): pin the self-describe catalog's cross-spine guidance`
- `6a32bb01` — `docs(07-03): document CLI cross-spine recall and the coverage footer`
- `d6f00970` — `docs(07-03): record that cross_spine is reachable from the CLI`

## Self-Check: PASSED

- `cmd/engram/catalog_test.go` FOUND, contains `TestCatalogCarriesCrossSpineGuidance`
- `docs-site/src/content/docs/guides/cli.md` FOUND, contains "Recall scope selection" and
  `searched_scopes`
- `docs-site/src/content/docs/guides/upgrade.md` FOUND, contains `### 6.`
- `CLAUDE.md` FOUND, `cross_spine` sentence names the CLI, contains no `cross-spine` flag spelling
- `git log --oneline --all | grep d3f47669` FOUND
- `git log --oneline --all | grep 6a32bb01` FOUND
- `git log --oneline --all | grep d6f00970` FOUND

## Next Phase Readiness

Phase 7 (CLI Cross-Spine Wiring) is complete across all three plans. The shipped surface: one shared
`validateScopeCrossSpine` guard, `renderCoverageFooter`, `--cross-spine` on both `engram search` and
`engram list` with bidirectional D-00 help text, the sole authorized `internal/` edit
(`EffectiveSearchScope`), and — as of this plan — a catalog guidance test, docs-site coverage, and a
CLAUDE.md reachability note that close D-00's acceptance bar. No blockers for phase close. The
surface-wide interface audit remains correctly deferred to backlog Phase 999.2, per 07-CONTEXT.md.

---
*Phase: 07-cli-cross-spine-wiring*
*Completed: 2026-08-02*
