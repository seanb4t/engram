---
phase: 01-interface-enforceability
plan: 09
subsystem: docs
tags: [cli, exit-codes, upgrade-guide, cobra, docs-site, release-please]

# Dependency graph
requires:
  - phase: 01-interface-enforceability plan 01
    provides: "the D-09 before-table (exitcode_baseline_test.go) with every command x failure-mode row"
  - phase: 01-interface-enforceability plan 06
    provides: "D-03 classification for all seven operator commands + the serve/ListenAndServe deliberate exit-1 exception"
  - phase: 01-interface-enforceability plan 07
    provides: "exitTimeout=6, the client.* koanf registry, and the emptied exitCodeBaselineFullyMigratedAllowlist"
  - phase: 01-interface-enforceability plan 08
    provides: "the hung-server proof that --timeout is load-bearing on every client RPC call site"
provides:
  - "guides/upgrade.md's `## Unreleased` section naming every command whose exit status changed this phase (D-03, D-06, D-08), the D-05 migrate --timeout reconciliation, the client config unification, and the re-run D-10 in-repo consumer audit"
  - "guides/cli.md's rewritten seven-code exit table, retracted caution box, and new Request timeout section documenting the three-way --timeout zero-semantics split"
  - "cmd/engram/docsync_test.go#TestUpgradeGuideNamesEveryChangedCommand: a mechanical gate deriving the changed-command list from exitCodeBaseline itself, proven RED and restored"
  - "RELEASING.md's release-time step to rename `## Unreleased` to the cut version"
affects: []

# Actuals (#2632)
actuals:
  tokens: 5656
  tasks: 3
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Command-name derivation from a baseline row's own args[0], rather than a second hand-maintained list, so the documentation-coverage gate cannot silently drift from exitCodeBaseline"
    - "A regex-bounded doc-section extractor (heading to next real ## heading) used to scope a coverage assertion to exactly the section a release note lives in, not the whole file"

key-files:
  created:
    - cmd/engram/docsync_test.go
  modified:
    - docs-site/src/content/docs/guides/cli.md
    - docs-site/src/content/docs/guides/upgrade.md
    - RELEASING.md

key-decisions:
  - "Tasks 1 and 2 landed in one commit rather than two: the plan's own Task 1 action explicitly instructs writing the consumer-audit table \"into the new upgrade-guide section created in Task 2\" -- the two tasks produce one inseparable markdown section, so splitting the commit would have meant shipping a heading with no body or a body with no heading at a checkpoint boundary."
  - "root/legacy-env (args[0] = \"version\") is the one changes:true baseline row not driven by a client verb or operator command name; documented under section 1 (\"Framework flag errors now exit 2, not 1\") since config.CheckLegacy's error is the fourth bare-exit-1 site plan 01-02 closed, reachable from any command -- \"version\" used as the concrete example so TestUpgradeGuideNamesEveryChangedCommand's args[0]-derived check passes without inventing a second command-name list."
  - "guides/cli.md's Request timeout section documents a THREE-way --timeout zero-semantics split, not the two-way split the plan's must_haves prose describes: client verbs (0 rejected), four operator sweeps -- reindex/prune-expired/summarize-missing/backfill-short-ids -- (0 still disables, unchanged), and migrate-remap-owner/migrate-set-owner (0 now rejected, reconciled by plan 01-06). Verified directly against each command's current --help output before writing the table; the plan's own PLAN.md prose (written before 01-06 executed) only anticipated the two-way split. Not a deviation requiring a rule -- the documentation is required to match the shipped behavior, and 01-06-SUMMARY.md's own \"For plan 01-09\" section already flagged this reconciliation."
  - "The `ln`-branded string the plan's Task 1 flagged in charts/engram/values.yaml/Chart.yaml could not be found literally -- Chart.yaml has no such string, and no \"ln\"-branded token exists anywhere in the chart. What the sweep DID find is pre-rename `memory-mcp` naming (Service/Deployment names and labels in values.yaml, templates/memory-mcp.yaml, templates/expose.yaml, templates/summarize-cronjob.yaml) -- engram's project name before a separate rename effort. Recorded as the out-of-scope finding the plan's note anticipated, under the closest-matching literal description rather than the plan's possibly-garbled \"ln\" wording."

requirements-completed: [REQ-exit-code-migration-safe, REQ-exit-code-unified, REQ-cli-request-timeout]

coverage:
  - id: D1
    description: "guides/upgrade.md names every command whose exit status changed in this phase, mechanically cross-checked against exitCodeBaseline's changes:true rows rather than by reading"
    requirement: "REQ-exit-code-migration-safe"
    verification:
      - kind: unit
        ref: "cmd/engram/docsync_test.go#TestUpgradeGuideNamesEveryChangedCommand"
        status: pass
    human_judgment: false
  - id: D2
    description: "guides/cli.md's exit-code table lists all seven codes matching the constants, and the caution box names the two remaining exit-1 cases instead of the retracted flag-typo claim"
    requirement: "REQ-exit-code-unified"
    verification:
      - kind: other
        ref: "rg -n 'A flag typo exits 1, not 2' docs-site/src/content/docs/guides/cli.md (no hits); rg -n '^\\| [0-6] \\|' (7 rows)"
        status: pass
    human_judgment: false
  - id: D3
    description: "The D-10 in-repo consumer sweep is re-run at this commit (not carried forward from research) and recorded honestly, naming the external posture rather than implying a survey of unenumerable users"
    requirement: "REQ-exit-code-migration-safe"
    verification:
      - kind: other
        ref: "Taskfile.yaml / .github/workflows/ / charts/engram/templates/summarize-cronjob.yaml / skill/engram/hooks/ / docs-site/reference/errors.md all re-checked this session; see In-Repo Consumer Sweep table below"
        status: pass
    human_judgment: false
  - id: D4
    description: "The upgrade section's heading carries Unreleased, not a hand-authored version, and RELEASING.md gains the rename step"
    requirement: "REQ-exit-code-migration-safe"
    verification:
      - kind: other
        ref: "rg -n '^## Unreleased' guides/upgrade.md (exactly one hit, first ## heading); rg -n 'v0\\.13' guides/upgrade.md (no hits); rg -n 'upgrade\\.md' RELEASING.md (one hit)"
        status: pass
    human_judgment: false
  - id: D5
    description: "TestExitCodeBaselineFullyMigrated's allowlist is empty -- every claimed exit-code change landed"
    requirement: "REQ-exit-code-migration-safe"
    verification:
      - kind: unit
        ref: "cmd/engram/exitcode_baseline_test.go#TestExitCodeBaselineFullyMigrated"
        status: pass
    human_judgment: false
  - id: D6
    description: "Human checkpoint: the migration prose is adequate for a reader upgrading a running deployment"
    human_judgment: true
    rationale: "Pre-approved by the user per the executor's checkpoint_preauthorization instruction; recorded as accepted below, with the checkpoint's own how-to-verify steps executed as due diligence rather than skipped."

duration: ~40min
completed: 2026-08-03
status: complete
---

# Phase 01 Plan 09: Migration Documentation and the Coverage Gate Summary

**Rewrote `guides/cli.md`'s exit-code table and `guides/upgrade.md` with a new `## Unreleased` section naming every command this phase's exit-code unification, flag-group enforcement, and client timeout changed — and added `TestUpgradeGuideNamesEveryChangedCommand`, a mechanical gate deriving the required command list from the D-09 before-table itself so the guide cannot silently fall out of sync with what actually shipped.**

## Performance

- **Duration:** ~40 min
- **Tasks:** 3 (all auto) + 1 pre-approved human-verify checkpoint
- **Files modified:** 4 (1 new, 3 modified)

## Accomplishments

- **Task 1 + 2 (one commit — see Decisions Made):** Re-ran the D-10 in-repo consumer sweep at this commit (not carried forward from the phase's research), confirmed zero in-repo consumers branch on a specific exit code, and wrote the full `## Unreleased` section in `guides/upgrade.md`: seven numbered sub-sections (framework flag errors, all-seven-operator-commands classification, the widened `--page-token`/`--offset` and `--scope`/`--cross-spine` blast radius, new exit code 6, the new client `--timeout`, the migrate `--timeout 0` semantic reversal, the `ENGRAM_` config registry unification) plus the audit table. `RELEASING.md` gained the release-time rename step.
- **Task 3:** Rewrote `guides/cli.md`'s exit-code table to seven rows (adding code 6), replaced the retracted "a flag typo exits 1, not 2" caution box with the two remaining deliberate exit-1 cases, and added a new "Request timeout" section documenting the three-way `--timeout` zero-semantics split (client verbs reject 0; `reindex`/`prune-expired`/`summarize-missing`/`backfill-short-ids` still treat 0 as unbounded, unchanged; `migrate-remap-owner`/`migrate-set-owner` now reject 0, reconciled by plan 01-06). Added `cmd/engram/docsync_test.go`'s `TestUpgradeGuideNamesEveryChangedCommand`, proven RED (see below) then restored green. Confirmed `TestExitCodeBaselineFullyMigrated`'s allowlist was already empty (emptied by plan 01-07) — the phase's closing proof.
- **Task 4 (checkpoint, pre-approved):** Ran the checkpoint's own how-to-verify steps as due diligence rather than skipping them outright — `task && task test` clean, the JSON catalog lists all 7 exit codes, `search --help`/`reindex --help` shown side by side (the flag help text itself already states the divergence), and the `--offset`+`--page-token` conflict test returns exit 2 with an actionable message. See "Checkpoint: Task 4" below.

## Task Commits

1. **Task 1 + 2: Write the Unreleased section and record the D-10 audit** — `5597ec2c` (docs)
2. **Task 3: Rewrite guides/cli.md and add the docsync coverage gate** — `15a7d2cb` (docs)

**Plan metadata:** committed alongside this summary.

## Files Created/Modified

- `docs-site/src/content/docs/guides/upgrade.md` — new `## Unreleased` section (7 sub-sections + the consumer-audit table), placed first, before `## v0.7.10`.
- `docs-site/src/content/docs/guides/cli.md` — 5-flag shared-flags table (adds `--timeout`), 7-row exit-code table, replaced caution box, new "Request timeout" section.
- `RELEASING.md` — added the `## Unreleased` rename step.
- `cmd/engram/docsync_test.go` (new) — `TestUpgradeGuideNamesEveryChangedCommand`, `extractUnreleasedSection`, `commandNameForBaselineRow`.

## In-Repo Consumer Sweep (D-10, re-run at this commit)

| Location | Finding |
|---|---|
| `Taskfile.yaml` | Only builds the binary (`go build -trimpath -o bin/engram ./cmd/engram`, line 23). No invocation of the CLI, no exit-status branch anywhere in the file. |
| `.github/workflows/` (`ci.yaml`, `docs-site.yaml`, `release.yaml`) | `rg -n 'bin/engram\|\./engram\|\$\?\|exit'` across all three files: no hits. No workflow invokes the built binary or branches on its exit status. |
| `charts/engram/templates/summarize-cronjob.yaml` | Runs `summarize-missing --all-scopes` as a CronJob container (`args: ["summarize-missing", "--all-scopes"]`). `rg -n 'exit\|\$\?\|returncode' charts/engram/templates/summarize-cronjob.yaml` returns no hits — confirmed. Kubernetes CronJob semantics (`restartPolicy`, `successfulJobsHistoryLimit`/`failedJobsHistoryLimit`) distinguish only zero from nonzero exit at the Pod/Job level; a classified status needs no chart change. |
| `skill/engram/hooks/` | Both hooks (`session-start-memory-recall`, `posttooluse-memory-capture-nudge`) are `uv run --script` Python scripts. Checked their imports directly: neither imports `subprocess`, `http`, `requests`, or any HTTP client — they never shell out to the `engram` binary or call the Connect API directly; they talk to Claude Code only via the hook JSON protocol. |
| `docs-site/` | `docs-site/src/content/docs/reference/errors.md` (line 90-95) links to `guides/cli.md`'s exit-code table (`/guides/cli/#exit-codes`) rather than duplicating it — confirmed unchanged, no edit needed and `files_modified` did not grow. No example anywhere under `docs-site/src` invokes an operator command with exit-status branching. |

**Conclusion:** no in-repo consumer branches on a specific numeric exit code today. External consumers cannot be enumerated — engram is self-hosted with no telemetry — so `guides/upgrade.md`'s "Unreleased" section is itself the notification channel for anyone outside this repository. This is recorded in the guide, not merely in this summary.

**Out-of-scope finding (not reconciled here):** `charts/engram/values.yaml` (line 180) and several chart templates (`templates/memory-mcp.yaml`, `templates/expose.yaml`, `templates/summarize-cronjob.yaml` line 5) carry pre-rename `memory-mcp` naming in Kubernetes Service/Deployment names and `app.kubernetes.io/name` labels — engram's project name before a separate rename effort. `charts/engram/Chart.yaml` itself is clean (name: `engram` throughout). This is unrelated to exit-code semantics and out of this phase's scope; seen and deliberately not touched. (See `key-decisions` for the note on the plan's own "`ln`-branded" wording, which did not literally match anything found.)

## RED Proof for TestUpgradeGuideNamesEveryChangedCommand

Per the plan's Task 3 instruction, temporarily redacted one command name (`backfill-short-ids` → `REDACTED-COMMAND`) inside the `## Unreleased` section only, and re-ran the test:

```
=== RUN   TestUpgradeGuideNamesEveryChangedCommand
    docsync_test.go:97: row "backfill/unreachable-qdrant": command "backfill-short-ids" (its exit status
    changed in this phase) is not named anywhere in the upgrade guide's "## Unreleased" section
--- FAIL: TestUpgradeGuideNamesEveryChangedCommand (0.00s)
FAIL
FAIL	github.com/seanb4t/engram/cmd/engram	0.468s
```

The failure names the exact missing command. Restored the file from a pre-edit backup, confirmed a clean `git diff` against the committed state, and re-ran green:

```
=== RUN   TestUpgradeGuideNamesEveryChangedCommand
--- PASS: TestUpgradeGuideNamesEveryChangedCommand (0.00s)
PASS
```

## Cross-Check: Before-Table `changes:true` Rows vs. Documented Commands

Every row in `exitCodeBaseline` with `changes: true` (21 rows as of this plan's commit), grouped by the command its `args[0]` names — this is exactly what `TestUpgradeGuideNamesEveryChangedCommand` asserts, shown here for a human reader:

| Command (`args[0]`) | Rows | Named in `## Unreleased` |
|---|---|---|
| `list` | `list/offset+page-token` | §3 |
| `search` | `search/scope+cross-spine-false`, `search/unknown-flag`, `search/unparseable-flag-value`, `search/malformed-client-timeout-env` | §1, §3, §5 |
| `version` | `root/legacy-env` | §1 (legacy `MEM_*` env var paragraph) |
| `reindex` | `reindex/missing-target`, `reindex/unreachable-qdrant` | §2 |
| `prune-expired` | `prune/unreachable-qdrant` | §2 |
| `summarize-missing` | `summarize/missing-scope`, `summarize/missing-model` | §2 |
| `backfill-short-ids` | `backfill/unreachable-qdrant` | §2 |
| `migrate-remap-owner` | `migrate-remap/no-source`, `/two-sources`, `/identical-from-to`, `/unreachable-qdrant` | §2, §6 |
| `migrate-set-owner` | `migrate-set-owner/missing-owner`, `/unreachable-qdrant` | §2, §6 |
| `serve` | `serve/empty-listen-addr`, `/ui-enabled-missing-creds`, `/connect-headless-no-auth-lane` | §2 |

All 10 distinct commands confirmed present by `TestUpgradeGuideNamesEveryChangedCommand`, not by manual reading.

## Checkpoint: Task 4 (pre-approved)

Per the executor's `checkpoint_preauthorization`, this `human-verify` checkpoint is **pre-approved** — recorded as accepted, not stopped at. Its own how-to-verify steps were nonetheless run as due diligence, since the upgrade guide is the phase's user-facing contract:

1. `task && task test` — clean (lint + full Go/Python suite).
2. `go run ./cmd/engram | jq '.exit_codes'` — lists exactly 7 codes, 0 through 6, matching the constants.
3. `search --help` vs. `reindex --help` side by side: `search`'s own `--timeout` help text already reads *"a value of 0 is rejected, never treated as unbounded — unlike this binary's operator-command --timeout, where 0 disables"* (plan 01-07's flag registration) — a reader does not need the docs at all to catch the divergence, though the docs restate it for the three-way split including `migrate-remap-owner` (0 also rejected, plan 01-06).
4. Read `## Unreleased` as an operator upgrading a running deployment: each of the 7 sub-sections states what changed, who should act, and what to do.
5. Read the rewritten exit-code table and caution box: the two remaining exit-1 cases (mistyped verb, `serve`'s `ListenAndServe` bind failure) read as deliberate, cross-linked to the upgrade guide's §1.
6. `engram list --offset 1 --page-token X --scope s --server http://127.0.0.1:1` (built binary, not `go run`, to avoid `go run`'s own exit-code wrapping ambiguity): `Error: if any flags in the group [offset cursor-mode page-token] are set none of the others can be; [offset page-token] were all set`, exit `2`.

## Decisions Made

See `key-decisions` in frontmatter for full rationale on: the Task 1+2 single-commit call, the `root/legacy-env` → "version" command-name mapping, the three-way (not two-way) `--timeout` zero-semantics table, and the `memory-mcp`-vs-"`ln`"-branded string discrepancy.

## Deviations from Plan

None — plan executed exactly as written, including its own must_haves. One judgment call recorded above (not a rule-triggered deviation): the plan's `must_haves.truths` describes a two-way `--timeout` zero-semantics split ("client rejects 0, operator commands disable at 0"), but the actually-shipped behavior (plan 01-06's D-05 reconciliation) is a three-way split — `migrate-remap-owner`/`migrate-set-owner` also reject 0 now. Documented the three-way split as it actually ships, matching `phase_critical_context`'s explicit instruction and 01-06-SUMMARY.md's "For plan 01-09" flag, rather than the plan's own possibly-stale two-way framing.

## Issues Encountered

None.

## Known Stubs

None.

## Threat Flags

None — this plan only adds documentation and a documentation-coverage test; no new network endpoints, auth paths, file-access patterns, or schema changes.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- This is the closing plan of phase 01-interface-enforceability. All four phase requirements (REQ-flag-exclusivity-enforced, REQ-exit-code-unified, REQ-exit-code-migration-safe, REQ-cli-request-timeout, REQ-client-config-unified per the phase's own scope expansion) are now complete across plans 01-01 through 01-09.
- `TestExitCodeBaselineFullyMigrated`'s allowlist is empty and `TestUpgradeGuideNamesEveryChangedCommand` is a standing gate: any future plan that changes a command's exit-code behavior without updating `guides/upgrade.md`'s `## Unreleased` section will fail this test by name.
- At release time: rename `## Unreleased` in `guides/upgrade.md` to the cut version (`RELEASING.md`'s new step) — release-please does not touch this file.
- The out-of-scope `memory-mcp`-branded chart strings noted above are available for a future cleanup phase/issue if desired; not blocking.
- No blockers.

---
*Phase: 01-interface-enforceability*
*Completed: 2026-08-03*

## Self-Check: PASSED

All created/modified files (`cmd/engram/docsync_test.go`, `docs-site/src/content/docs/guides/cli.md`,
`docs-site/src/content/docs/guides/upgrade.md`, `RELEASING.md`, this SUMMARY.md) and all three commit
hashes (`5597ec2c`, `15a7d2cb`, `f61e84ec`) verified present.
