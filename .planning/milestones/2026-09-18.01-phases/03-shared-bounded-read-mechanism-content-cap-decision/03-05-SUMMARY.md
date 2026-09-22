---
phase: 03-shared-bounded-read-mechanism-content-cap-decision
plan: 05
subsystem: docs
tags: [content-cap, tags-cap, cli, e2e, docs, D-01, D-09, D-10]

requires:
  - phase: 03-shared-bounded-read-mechanism-content-cap-decision
    provides: "plan 03-01's memoryWriteCaps/checkContentBytes/checkTags (the enforcement this plan proves through the real binary and documents)"
provides:
  - "TestCLIStoreRejectsOversizedContentAndTags — the real engram binary's store verb proven against a real engram serve, closing the one lane (CLI) not yet exercised end to end"
  - "docs-site guides/configure.md, reference/errors.md, reference/tools.md, guides/upgrade.md documenting the three caps, their rejection shapes, and the upgrade impact"
  - "decision A recorded in .planning/PROJECT.md Key Decisions (REQ-content-cap-decided closes)"
  - "CLAUDE.md and the curating-memory skill (canonical + vendored) stating the content/tags bounds beside the summary bound"
affects: [03-06]

actuals:
  tokens: 5083
  tasks: 3
  commits: 3
  plan_head_before: 4b8abe3c3dbe8d5aea7909584470b46f3fbca15f

tech-stack:
  added: []
  patterns:
    - "runCLIEnv(t, env, args...) generalizes runCLI's hermetic-env body so a test can point the real binary's subprocess at a real server with a bearer token, without inheriting the developer shell's ENGRAM_* vars"
    - "--output text forced on every CLI invocation in an e2e test that asserts stdout text shape, since exec.Command's non-TTY stdout otherwise defaults to JSON (existing spine_review_test.go precedent, applied here)"

key-files:
  created:
    - internal/e2e/contentcap_cli_test.go
  modified:
    - internal/e2e/cli_exitcode_test.go
    - docs-site/src/content/docs/guides/configure.md
    - docs-site/src/content/docs/reference/errors.md
    - docs-site/src/content/docs/reference/tools.md
    - docs-site/src/content/docs/guides/upgrade.md
    - .planning/PROJECT.md
    - CLAUDE.md
    - skill/engram/skills/curating-memory/SKILL.md
    - internal/skills/data/curating-memory/SKILL.md

key-decisions:
  - "D-01/D-10 CLI proof: the real engram binary's store verb, against a real engram serve (headless Connect, static-token auth), accepts an at-cap 65536-byte content and rejects a 65537-byte content, 129 tags, and one 129-byte tag, each with exit 2 and the named field=/hint= envelope on stderr — the CLI holds no client-side copy of the cap, so this closes the one lane not yet exercised end to end."
  - "Decision A recorded in PROJECT.md Key Decisions: memory content gets an always-enforced write cap (ENGRAM_MEMORY_MAX_CONTENT_BYTES, default 65536) plus the tags caps (ENGRAM_MEMORY_MAX_TAGS/ENGRAM_MEMORY_MAX_TAG_BYTES, 128/128); 0 is rejected — REQ-content-cap-decided closes."
  - "Documentation follows the existing hand-maintained configure.md table (no registry-to-docs generator or drift gate for that page in this repo) and the upgrade guide's numbered ## Unreleased entries, per D-00's framing."

requirements-completed: [REQ-content-cap-decided]

coverage:
  - id: D1
    description: "The real engram binary's store verb, against a real engram serve, accepts an at-cap 65536-byte content and rejects a 65537-byte content, 129 tags, and one 129-byte tag with exit 2 and the named envelope on stderr (D-01, D-10, CLI lane)"
    requirement: "REQ-content-cap-decided"
    verification:
      - kind: e2e
        ref: "internal/e2e/contentcap_cli_test.go#TestCLIStoreRejectsOversizedContentAndTags"
        status: pass
    human_judgment: false
  - id: D2
    description: "guides/configure.md documents the three caps beside the summary bound, states the always-enforced divergence (0 rejected) and that existing over-cap records stay readable/trimmable"
    requirement: "REQ-content-cap-decided"
    verification:
      - kind: other
        ref: "pnpm --dir docs-site build (exit 0) + rg -o counts against configure.md"
        status: pass
    human_judgment: false
  - id: D3
    description: "reference/errors.md carries worked field=content hint=too_long and field=tags hint=too_many examples, still lists exactly eleven hint codes, and the registered 02-03-errors-doc-drops-too-large.patch still applies"
    requirement: "REQ-content-cap-decided"
    verification:
      - kind: integration
        ref: "internal/server TestErrorsDocHintCodesMatchArgErrorConstants"
        status: pass
      - kind: other
        ref: "git apply --check .planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence/02-03-errors-doc-drops-too-large.patch"
        status: pass
    human_judgment: false
  - id: D4
    description: "reference/tools.md states the content and tags bounds on store_memory, schedule_memory and update_memory's field rows"
    requirement: "REQ-content-cap-decided"
    verification:
      - kind: other
        ref: "rg -o counts against reference/tools.md (3 content rows, 3 tags rows)"
        status: pass
    human_judgment: false
  - id: D5
    description: "guides/upgrade.md's ## Unreleased section gains ### 15. naming the behavior break, the hints, and that stored records are never rewritten"
    requirement: "REQ-content-cap-decided"
    verification:
      - kind: integration
        ref: "cmd/engram TestUpgradeGuideNamesEveryChangedCommand"
        status: pass
      - kind: other
        ref: "rg -n '^### 15. ' guides/upgrade.md — sits between ### 14. and ## v0.7.10"
        status: pass
    human_judgment: false
  - id: D6
    description: ".planning/PROJECT.md Key Decisions records decision A with rationale and outcome; no heading added or removed"
    requirement: "REQ-content-cap-decided"
    verification:
      - kind: other
        ref: "git diff -- .planning/PROJECT.md | rg -o '^[+-]#' | wc -l == 0"
        status: pass
    human_judgment: false
  - id: D7
    description: "CLAUDE.md's memory-contract sentence and the curating-memory skill (canonical == vendored) state the content/tags bounds beside the summary bound"
    requirement: "REQ-content-cap-decided"
    verification:
      - kind: integration
        ref: "internal/skills TestSkillsEmbedMatchesVendored"
        status: pass
      - kind: other
        ref: "diff -q skill/engram/skills/curating-memory/SKILL.md internal/skills/data/curating-memory/SKILL.md"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-09-19
status: complete
---

# Phase 3 Plan 5: CLI Proof and Documentation of the Content/Tags Caps Summary

**The real `engram` binary's `store` verb, against a real `engram serve`, proves D-01/D-10's content and tags caps end to end (exit 2, `field=content hint=too_long` / `field=tags hint=too_many|too_long`), and the caps are now documented on `configure.md`, `errors.md`, `tools.md`, `upgrade.md`, recorded as decision A in `PROJECT.md`, and stated in `CLAUDE.md` and the `curating-memory` skill.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-09-19T22:02:00Z
- **Completed:** 2026-09-19T22:22:05Z
- **Tasks:** 3 completed
- **Files modified:** 10 (1 created, 9 modified)

## Accomplishments

- Proved D-01/D-10 on the CLI lane: `TestCLIStoreRejectsOversizedContentAndTags` drives the real built `engram` binary's `store` verb against a real `engram serve` (headless Connect, static-token auth) — an at-cap 65536-byte `--content` exits `0`, a 65537-byte content exits `2` with `field=content hint=too_long`, 129 comma-joined `--tags` exits `2` with `field=tags hint=too_many`, and one 129-byte tag exits `2` with `field=tags hint=too_long`. The CLI holds no second copy of the cap — it reaches the server's configured value through Connect and reports the server's own rejection.
- Generalized `runCLI` into `runCLIEnv(t, env, args...)` so a test can point the CLI subprocess at a real server with a bearer token, without inheriting the developer shell's `ENGRAM_*` vars.
- Documented all three caps (`ENGRAM_MEMORY_MAX_CONTENT_BYTES`, `ENGRAM_MEMORY_MAX_TAGS`, `ENGRAM_MEMORY_MAX_TAG_BYTES`) in `guides/configure.md`'s `## Memory` table, including the always-enforced divergence from `ENGRAM_MEMORY_MAX_SUMMARY_BYTES` and the never-rewritten legacy-record guarantee.
- Added worked `field=content hint=too_long` / `field=tags hint=too_many` examples to `reference/errors.md`'s envelope-grammar section, extended the `too_many` hint row's example to name `tags`, and confirmed the registered `02-03-errors-doc-drops-too-large.patch` still applies unchanged.
- Extended `reference/tools.md`'s `content`/`tags` rows on `store_memory`, `schedule_memory`, and `update_memory` with the byte/count bounds (the `update_memory` rows note "enforced when the content/tag set changes").
- Added `### 15.` to `guides/upgrade.md`'s `## Unreleased` section, naming the behavior break, every affected surface (MCP, Connect, `engram store`, `idempotency_key` retries), and that stored records are never rewritten.
- Recorded decision A in `.planning/PROJECT.md`'s `## Key Decisions` table (rationale, outcome, residual) and extended `CLAUDE.md`'s memory-contract sentence and the `curating-memory` skill (canonical + `task skills:vendor`-refreshed vendored copy) to state the content/tags bounds beside the summary bound.

## Task Commits

Each task was committed atomically:

1. **Task 1: End to end — the real engram binary's store verb is rejected by a real server for over-cap content and tags** - `6dda93a6` (test, tracer)
2. **Task 2: Document the three caps, the rejection shapes, the tool bounds and the upgrade impact on docs-site** - `52f6b104` (docs)
3. **Task 3: Record decision A in PROJECT.md and update the agent-facing contract (CLAUDE.md, curating-memory skill)** - `bb00e18e` (docs)

**Plan metadata:** *(this commit, following this SUMMARY)*

## Tracer Feedback Gate

Task 1 (`type="tracer"`) was re-verified end to end after its own commit before Task 2 began: `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/e2e/ -run '^(TestCLIStoreRejectsOversizedContentAndTags|TestCLIExitCodes)$' -count=1 -v` passed with all 4 named subtests plus `TestCLIExitCodes` green, no `--- SKIP:` lines. Expansion into Task 2/3 proceeded without a checkpoint.

## Files Created/Modified

- `internal/e2e/contentcap_cli_test.go` (new) - `TestCLIStoreRejectsOversizedContentAndTags` (4 subtests: `at-cap`, `content-over-cap`, `too-many-tags`, `tag-too-long`)
- `internal/e2e/cli_exitcode_test.go` - `runCLI` reduced to `return runCLIEnv(t, nil, args...)`; new `runCLIEnv(t, env, args...)` holds the prior body with `cmd.Env = childEnv(env)`
- `docs-site/src/content/docs/guides/configure.md` - three new `## Memory` table rows + always-enforced/never-rewritten paragraph + extended `Source:` line
- `docs-site/src/content/docs/reference/errors.md` - two worked examples in `## The envelope grammar`; `too_many` row's example extended to name `tags`
- `docs-site/src/content/docs/reference/tools.md` - `content`/`tags` rows extended on `store_memory`, `schedule_memory`, `update_memory`
- `docs-site/src/content/docs/guides/upgrade.md` - `### 15. Memory content and tags are now capped: an oversized write is rejected`
- `.planning/PROJECT.md` - one new `## Key Decisions` row (decision A)
- `CLAUDE.md` - memory-contract sentence extended with the content/tags bounds
- `skill/engram/skills/curating-memory/SKILL.md` / `internal/skills/data/curating-memory/SKILL.md` - new paragraph stating the content/tags bounds, vendored copy refreshed via `task skills:vendor`

## Decisions Made

See `key-decisions` in frontmatter (the CLI proof closing the last unexercised lane, decision A recorded in PROJECT.md closing REQ-content-cap-decided, and the documentation-approach framing from D-00/03-CONTEXT.md). All were pre-locked in `03-CONTEXT.md` by the user in discuss-phase on 2026-09-19; no new architectural decision was made in this plan.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The plan's literal CLI invocation defaulted to JSON output, not the text output the acceptance criteria assert**
- **Found during:** Task 1 (first live run of `TestCLIStoreRejectsOversizedContentAndTags`)
- **Issue:** The plan's `<action>` names `store --server <srv.baseURL()> --scope repo:e2e-cap --source user-said --category decision` with no `--output` flag. `engram`'s output-format resolution defaults to JSON when stdout is not a TTY (always true under `exec.Command` in a test), so the `at-cap` subtest's `stdout` was a JSON document, not the expected `stored: <short_id> (<id>)` text — the subtest failed on `stdout = "{...}", want prefix "stored: "` even though the underlying at-cap write itself succeeded correctly.
- **Fix:** Added `--output text` to `commonArgs`, matching the existing `spine_review_test.go` precedent (`runCLIWithEnv(t, pruneEnv(collection), "prune-expired", "--output", "text")`) for forcing deterministic text output in a non-TTY e2e test.
- **Files modified:** internal/e2e/contentcap_cli_test.go
- **Verification:** All 4 subtests pass; `at-cap`'s stdout now starts `stored: `.
- **Committed in:** 6dda93a6 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug, test-only). **Impact:** No production code touched; the fix only corrected the test's own CLI invocation to match how the shipped binary actually resolves output format in a non-interactive context.

## TDD Gate Compliance

Not applicable — this plan's tasks carry no `tdd="true"` frontmatter. Task 1 (`type="tracer"`) still required an observed RED/GREEN cycle per its `<action>`'s "Detection power" instruction (not the full TDD gate):

| Task | RED observed | GREEN restored | Commit scope |
|------|---------------|-----------------|---------------|
| 1 | Yes — temporarily removed the `checkContentBytes(a.Content, caps.contentBytes)` call inside `validateStoreArgs` (`internal/server/tools.go`); re-ran `TestCLIStoreRejectsOversizedContentAndTags`: `--- FAIL: TestCLIStoreRejectsOversizedContentAndTags/content-over-cap` (`contentcap_cli_test.go:49: exit code = 0, want 2`), with `at-cap`/`too-many-tags`/`tag-too-long` still passing (proving the RED was specific to the removed check, not a broken harness) | Yes — restored the call via `Edit`; re-ran the same command: all 4 subtests + `TestCLIExitCodes` pass; `git diff --exit-code -- internal/server/tools.go` confirmed the file byte-for-byte restored | `test(e2e)` — the test file itself is the only thing committed for Task 1; the RED/GREEN cycle exercised existing production code from plan 03-01, never modifying it in this plan's commit |

RED was produced by a scoped `Edit`/re-`Edit` revert-and-restore cycle (never `git stash`), confirmed via the target test's `-v` output, then reverted before Task 1's single commit.

## Issues Encountered

None beyond the deviation above.

## Known Stubs

None. No hardcoded empty values, placeholder text, or unwired data sources were introduced.

## Threat Flags

None. This plan's own `<threat_model>` register (T-03-05-01 through T-03-05-03, T-03-05-SC) covers every new surface this plan introduces (the CLI lane reaching the server-side cap with no client-side duplicate, the e2e static token, operator misconfiguration of a cap, and the `pnpm install --frozen-lockfile` supply-chain concern); no new surface fell outside it.

## User Setup Required

None - no external service configuration required. All three env vars (`ENGRAM_MEMORY_MAX_CONTENT_BYTES`, `ENGRAM_MEMORY_MAX_TAGS`, `ENGRAM_MEMORY_MAX_TAG_BYTES`) were already registered with documented defaults by plan 03-01; this plan only adds documentation and the CLI-lane proof.

## Next Phase Readiness

- REQ-content-cap-decided is now fully closed: the cap is enforced on every memory write path (MCP, Connect including `UpdateMemory`'s field-mask lane, and `engram store`), documented on every reader-facing surface, recorded as decision A in `PROJECT.md`, and stated in the agent-facing contract (`CLAUDE.md`, `curating-memory` skill).
- No red-evidence patches are registered by this plan — per this phase's executor notes, plan 03-06 registers Phase 3's patches after the last plan.
- No blockers.

---
*Phase: 03-shared-bounded-read-mechanism-content-cap-decision*
*Completed: 2026-09-19*

## Self-Check: PASSED

- All 10 key files (1 created + 9 modified) confirmed present on disk.
- All 3 task commit hashes (`6dda93a6`, `52f6b104`, `bb00e18e`) confirmed present in `git log --oneline --all`.
- `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/e2e/ ./internal/server/ ./internal/surfaces/ ./internal/skills/ ./cmd/engram/ -count=1`: `ok` across all five packages.
- `pnpm --dir docs-site install --frozen-lockfile && pnpm --dir docs-site build`: exit 0, 21 pages built.
- `task lint` / `task license:check`: both clean.
- `go test ./internal/keylinks/ -run TestNoEscapedPatternsRepoWide -count=1`: `ok`.
- `git apply --check .planning/phases/02-error-classification-resourceexhausted-mapping/red-evidence/02-03-errors-doc-drops-too-large.patch`: applies clean.
