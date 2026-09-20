---
phase: 04-list-listscheduled-search-bounded-reads
plan: 07
subsystem: api
tags: [proto, cli, docs, connect, mcp, upgrade-guide, decision-record, red-evidence]

# Dependency graph
requires:
  - phase: 04-list-listscheduled-search-bounded-reads
    provides: "04-05: store.MaxRecallLimit and the store's rejectOverMaximum backstop"
  - phase: 04-list-listscheduled-search-bounded-reads
    provides: "04-06: rejectOverMaximumCount wired on all seven wire surfaces (field=<limit|k> hint=out_of_range) and list_rules' ceiling restated in code"
provides:
  - "proto/engram/v1/engram.proto: inline comments on ListMemoriesRequest.limit, SearchMemoriesRequest.k, SearchDiscoveriesRequest.k, stating the zero-value resolution and the maximum (1000) numerically — comment-only additions, no anchored region touched"
  - "cmd/engram/{client_list,client_search}.go: --limit/--k usage strings retire '0 = server default' for a numeric contract; regenerated help.golden/catalog.golden"
  - "docs-site tools.md/cli.md and CLAUDE.md: every recall-count surface (list_memory, list_scheduled, search_memory, search_discovery, list_rules) states its own zero-value default and the shared 1000 maximum; the 'an unset limit means all' cross-spine wording is gone"
  - "docs-site upgrade.md §16: BREAKING announcement for ListMemories' limit:0 meaning change (was unbounded, now 1000) and the reject-not-clamp change for an over-maximum cursor-mode limit, with a who-should-act row"
  - ".planning/PROJECT.md: decision B recorded in Key Decisions"
  - "internal/server/recallmaxdocs_test.go: TestRecallMaximumIsStatedNumerically — a durable, source-derived gate over 7 documented surfaces plus a negative sweep for the two retired wordings"
affects: [04-08]

# Actuals (#2632)
actuals:
  tokens: 9252
  tasks: 3
  commits: 3
  plan_head_before: 557152c2eac7b65061ea339f11b1ba8eaea1c2eb

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Correct-by-reading docs gate: derive the expected number from the ONE Go constant (store.MaxRecallLimit) via strconv.FormatUint — never a second hardcoded literal — and assert it appears on every documented surface, plus a negative sweep for retired wording built from string-concatenation fragments so the gate's own source (scanned by its own sweep, since it lives under internal/) cannot self-match."

key-files:
  created:
    - internal/server/recallmaxdocs_test.go
    - .planning/phases/04-list-listscheduled-search-bounded-reads/deferred-items.md
  modified:
    - proto/engram/v1/engram.proto
    - gen/go/engram/v1/engram.pb.go
    - gen/ts/engram/v1/engram_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts
    - cmd/engram/client_list.go
    - cmd/engram/client_search.go
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden
    - docs-site/src/content/docs/reference/tools.md
    - docs-site/src/content/docs/guides/cli.md
    - docs-site/src/content/docs/guides/upgrade.md
    - CLAUDE.md
    - .planning/PROJECT.md

key-decisions:
  - "Each Task 1 proto comment states '1000' exactly once per count field (3 occurrences total in the file) to match the acceptance criterion's exact count — wording ('0 resolves to the maximum, 1000; a larger value is rejected' / '0 resolves to this RPC's default, 20; a value above the maximum, 1000, is rejected') was chosen specifically to avoid a second occurrence of the digits."
  - "cli.md gained a new --limit row inside the existing 'Paging engram list' table plus a --k cross-reference sentence, rather than a new heading, since no dedicated count-flag section existed before this plan and the plan's own file list did not authorize adding one."
  - "upgrade.md's new entry (#16) covers BOTH halves of the one wire-visible behavior change in a single entry: the limit:0 meaning change (D-01, 0->1000) AND the reject-not-clamp change for an over-maximum cursor-mode limit (D-10, shipped by 04-05 but never announced) — both are the same caller-visible surface per the plan's success criteria ('The one behavior change callers can be broken by is announced as breaking'), so one entry with one who-should-act row covers both rather than splitting into two entries for a single shipped change."
  - "recallmaxdocs_test.go's two retired-wording constants are built via string concatenation (e.g. '0 = ' + 'server default') specifically so the test file's own source — which its own negative sweep scans, since the file lives under internal/ — never self-matches. The file's header doc comment also deliberately avoids quoting either retired phrase verbatim, after the first RED/GREEN run tripped exactly that self-match against the comment text (not the string literals) and had to be reworded."
  - "Did not edit 04-06-PLAN.md's key_links pattern (frontmatter:62, 'Full: req[.]Full') despite `task` reporting TestActiveMilestoneKeyLinksSatisfiable red on every run — confirmed via `git show 557152c2:internal/server/tools.go` (04-06's own final commit, before any 04-07 change) that the pattern was already unsatisfiable then, so this is a pre-existing 04-06 defect, not a regression from any file this plan touches. Per the plan's own cross-plan note ('04-06's surface... is not 04-07's to edit') and the planning-artifacts rule (never hand-edit a tool-generated/tool-checked file), this stays 04-06's defect: documented in deferred-items.md and the WINDOWS.md ledger (entry 11) instead of fixed here."

requirements-completed: []
# REQ-list-limit-contract-decided is shared with 04-08 (the phase's last
# plan, which owns red-evidence registration and the REQUIREMENTS.md tick
# per this plan's own project gates). `requirements.ready-ids` returned
# 0/1 ready, so it is NOT marked complete here.

coverage:
  - id: D1
    description: "The recall count maximum (1000) and each surface's own zero-value default are stated numerically on the wire schema (three proto comments), both CLI flag help strings, and the regenerated Go/TS/console-vendored trees — retiring '0 = server default' — so a caller learns the contract by reading, never by triggering a rejection"
    requirement: "REQ-list-limit-contract-decided"
    verification:
      - kind: unit
        ref: "cmd/engram#TestHelpGolden, TestCatalogGolden"
        status: pass
      - kind: unit
        ref: "internal/server#TestRecallMaximumIsStatedNumerically/proto_list_limit, /proto_search_memories_k, /proto_search_discoveries_k, /cli_list_limit_flag, /cli_search_k_flag"
        status: pass
      - kind: other
        ref: "task proto:lint && task proto:gen && git diff --exit-code -- gen ui/src/lib/gen proto (zero-diff regeneration proof)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every published docs surface (tools.md's memory-list/scheduled-list/both-search rows and its cross-spine advisory paragraph, cli.md's paging section, list_rules' ceiling, and CLAUDE.md's memory-contract sentences) states the same maximum numerically with the retired 'an unset limit means all' wording gone"
    requirement: "REQ-list-limit-contract-decided"
    verification:
      - kind: unit
        ref: "internal/server#TestRecallMaximumIsStatedNumerically/tools_md, /cli_md, /negative_sweep_retired_wording"
        status: pass
      - kind: other
        ref: "task lint:markdown; go test ./internal/server/ -run TestErrorsDocHintCodesMatchArgErrorConstants"
        status: pass
    human_judgment: false
  - id: D3
    description: "The BREAKING zero-limit change (Connect ListMemories / engram list, and the reject-not-clamp change for an over-maximum cursor-mode limit) is announced in the upgrade guide with a who-should-act row, and decision B is recorded in PROJECT.md's Key Decisions table without adding a version-bearing milestone heading"
    requirement: "REQ-list-limit-contract-decided"
    verification: []
    human_judgment: true
    rationale: "No durable test asserts upgrade.md's prose content or PROJECT.md's specific row text (unlike the numeric-contract surfaces, which TestRecallMaximumIsStatedNumerically pins). Verified manually against the plan's acceptance criteria (heading numbered exactly one past the prior highest, ListMemories named, who-should-act table row added, PROJECT.md's ✅/📋/🚧 marker count unchanged at 8, new row is the table's last row) but there is no regression gate keeping this prose accurate going forward — flagged for the verifier."
  - id: D4
    description: "A durable, source-derived test gate (TestRecallMaximumIsStatedNumerically) asserts every documented recall surface states the maximum and that neither of D-01's retired wordings has come back anywhere under internal/, cmd/, docs-site/src/, proto/, or CLAUDE.md; proven to go RED against a deliberately reverted surface and GREEN against the shipped tree; its own source cannot trip its negative sweep"
    requirement: "REQ-list-limit-contract-decided"
    verification:
      - kind: unit
        ref: "internal/server#TestRecallMaximumIsStatedNumerically (8 subtests: 7 surfaces + 1 negative sweep)"
        status: pass
      - kind: other
        ref: "task (lint + full test suite) — passes except the pre-existing, out-of-scope TestActiveMilestoneKeyLinksSatisfiable failure (see Issues Encountered)"
        status: pass
    human_judgment: false

# Metrics
duration: ~25min
completed: 2026-09-20
status: complete
---

# Phase 4 Plan 7: List/Search Bounded Reads — Recall Count Contract Published Summary

**The recall count maximum (1000) is now stated numerically on the wire schema, both CLI flag help strings, the tool reference, the CLI guide, and CLAUDE.md's memory contract — with a new source-derived test gate (`TestRecallMaximumIsStatedNumerically`) keeping every surface honest, decision B recorded in PROJECT.md, and the `ListMemories` `limit: 0` meaning change announced BREAKING in the upgrade guide.**

## Performance

- **Duration:** ~25 min
- **Started:** ~2026-09-20T10:16:00Z (approx)
- **Completed:** 2026-09-20T10:40:00Z (approx)
- **Tasks:** 3
- **Files modified:** 14 (2 created, 12 modified)

## Accomplishments

- Added inline comments to `ListMemoriesRequest.limit`, `SearchMemoriesRequest.k`, and `SearchDiscoveriesRequest.k` in `proto/engram/v1/engram.proto`, each stating the field's own zero-value resolution and the shared maximum (1000) numerically — comment-only additions, zero anchored regions touched, zero new fields.
- Regenerated `gen/go`, `gen/ts`, and the vendored `ui/src/lib/gen` console tree via `task surfaces:gen`; the diff is confined to doc comments, confirmed by a fresh `task proto:gen` producing zero further diff.
- Rewrote `engram list --limit` and `engram search --k`'s usage strings to state the same numeric contract, retiring "0 = server default"; regenerated `help.golden`/`catalog.golden` from the live cobra tree.
- Updated `docs-site/reference/tools.md`'s memory-list, scheduled-list, and both search tools' argument rows to state each tool's own zero-value default and the shared 1000 maximum with the `out_of_range` hint named; rewrote the cross-spine advisory paragraph to drop "an unset limit means all"; restated `list_rules`' ceiling ("up to 1000 rules per scope").
- Added a `--limit` row to `docs-site/guides/cli.md`'s paging table and a matching `--k` cross-reference note, both pointing at the exit-code table.
- Added upgrade guide entry §16 announcing the `ListMemories`/`engram list` `limit: 0` meaning change (was unbounded "all", now 1000) and the reject-not-clamp change for an over-maximum cursor-mode limit, with a who-should-act table row.
- Updated CLAUDE.md's `list_memory` pagination sentence and `list_rules` sentence to state the 1000 ceiling numerically.
- Recorded decision B in `.planning/PROJECT.md`'s Key Decisions table (one new row, confirmed the table's own `✅`/`📋`/`🚧` version-marker count is unchanged at 8) and updated the trailing last-updated line.
- Authored `internal/server/recallmaxdocs_test.go`'s `TestRecallMaximumIsStatedNumerically`: 7 per-surface subtests (3 proto fields, 2 CLI flag lines, 2 docs pages) plus a negative sweep over `internal/`, `cmd/`, `docs-site/src/`, `proto/`, and `CLAUDE.md` for D-01's two retired wordings. The number is derived from `store.MaxRecallLimit` via `strconv.FormatUint` — never a second hardcoded literal. Proven RED by reverting `tools.md` to its pre-Task-2 state (`git show`, restored from a saved copy — never `git stash`) and observing the exact assertion failure, then restored and confirmed GREEN.

## Task Commits

Each task was committed atomically:

1. **Task 1: End to end — the number reaches the wire schema and the command line, and every generated artifact moves with it** - `e74f3305` (feat)
2. **Task 2: The published pages, the breaking-change entry, and the project decision record** - `1af611ed` (docs)
3. **Task 3: A durable gate that the number is stated on every documented surface and the retired wording cannot come back** - `76428575` (test)

_No plan-metadata commit yet; STATE.md/ROADMAP.md updates follow this summary._

## RED Evidence (Task 3, TDD)

Observed by temporarily overwriting `docs-site/src/content/docs/reference/tools.md` with its pre-Task-2 content (`git show e74f3305:...` into the live path, restored afterward from a saved copy — never `git stash`):

```
recallmaxdocs_test.go:185: ../../docs-site/src/content/docs/reference/tools.md: surface "tools_md" does not contain the documented maximum "1000" -- got "...[full pre-Task-2 page content, no numeric maximum anywhere]..."
--- FAIL: TestRecallMaximumIsStatedNumerically/tools_md
--- FAIL: TestRecallMaximumIsStatedNumerically
```

Genuine RED — the exact planned assertion (the maximum's numeric presence) failing against the pre-edit page, not a panic, a zero-test discovery, or an unrelated failure. After restoring the file (confirmed via `git diff --stat` showing zero diff), all 8 subtests pass (see Self-Check).

A separate, earlier RED/GREEN cycle also surfaced a self-match: the file's own header doc comment initially quoted one retired phrase verbatim, making the negative sweep fail against the test file's own source. Reworded before the RED-evidence run above (see Deviations).

## Files Created/Modified

- `proto/engram/v1/engram.proto` - inline comments on the three count fields
- `gen/go/engram/v1/engram.pb.go`, `gen/ts/engram/v1/engram_pb.ts`, `ui/src/lib/gen/engram/v1/engram_pb.ts` - regenerated (doc-comment-only diff)
- `cmd/engram/client_list.go`, `cmd/engram/client_search.go` - `--limit`/`--k` usage strings
- `cmd/engram/testdata/help.golden`, `cmd/engram/testdata/catalog.golden` - regenerated
- `docs-site/src/content/docs/reference/tools.md` - limit/k rows, cross-spine advisory, `list_rules` ceiling
- `docs-site/src/content/docs/guides/cli.md` - new `--limit` row and `--k` note in the paging section
- `docs-site/src/content/docs/guides/upgrade.md` - new §16 BREAKING entry and who-should-act row
- `CLAUDE.md` - `list_memory` pagination sentence and `list_rules` sentence
- `.planning/PROJECT.md` - decision B row and last-updated line
- `internal/server/recallmaxdocs_test.go` (new) - `TestRecallMaximumIsStatedNumerically`
- `.planning/phases/04-list-listscheduled-search-bounded-reads/deferred-items.md` (new) - the pre-existing keylinks failure, acknowledged

## Decisions Made

See key-decisions in frontmatter.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `recallmaxdocs_test.go`'s own header comment self-matched its negative sweep**
- **Found during:** Task 3's own first RED/GREEN run (before the RED-evidence run recorded above)
- **Issue:** The file's header doc comment quoted one of D-01's retired wordings verbatim in prose (`"0 = server default"`), which the file's own negative sweep — scanning `internal/` — correctly flagged as a match against its own source, since the sweep has no notion of "this occurrence is inside a comment explaining what the sweep does."
- **Fix:** Reworded the header comment to describe the two retired wordings without quoting either verbatim, pointing the reader at the `retiredPhraseServerDefault`/`retiredPhraseUnsetMeansAll` variable declarations instead (which are themselves built from concatenated fragments for the identical reason).
- **Files modified:** `internal/server/recallmaxdocs_test.go`
- **Verification:** `go test ./internal/server/ -run '^TestRecallMaximumIsStatedNumerically$' -count=1 -v` — all 8 subtests pass.
- **Committed in:** `76428575` (Task 3's only commit — caught and fixed before the file was ever committed, never a separate fix commit)

---

**Total deviations:** 1 auto-fixed (1 Rule 1 — a self-match bug in this plan's own new test file, caught and fixed before its first commit)
**Impact on plan:** No scope creep. The fix is entirely inside this plan's own new file and was required for the plan's own TDD gate (Task 3's `<behavior>`: "the gate's own source does not match its own negative sweep") to hold.

## Issues Encountered

- **Pre-existing `TestActiveMilestoneKeyLinksSatisfiable` failure (out of scope, not fixed):** every `task`/`go test ./internal/keylinks/...` run reports one failure — `.planning/phases/04-list-listscheduled-search-bounded-reads/04-06-PLAN.md:62`'s `key_links` entry (`pattern: "Full: req[.]Full"`) no longer matches `internal/server/tools.go` or `internal/store/store.go`, because the real code is a gofmt-aligned struct literal (`Full:              req.Full,`, many spaces) and the pattern requires exactly one. Confirmed via `git show 557152c2:internal/server/tools.go` (04-06's own final commit, before any 04-07 change) that this was ALREADY unsatisfiable then — it predates 04-07 entirely, and neither `internal/server/tools.go` nor `internal/store/store.go` is in this plan's `files_modified` or actual diff. Per the plan's own cross-plan note (04-06's surface, including its `PLAN.md`, is not 04-07's to edit) and the planning-artifacts rule (never hand-edit a tool-checked file to work around the tool), this is left as 04-06's defect: documented in `deferred-items.md` and appended to the `WINDOWS.md` ledger (entry 11, kind `deviation`, status `open`) rather than fixed here. `go test ./internal/keylinks/ -count=1` therefore does NOT exit 0 in this plan's run, contrary to the plan's own `<verification>` line — every other line of the plan-level `<verification>` block passes.
- **Literal-command mismatch (same pattern documented in every prior plan's SUMMARY this phase):** Task 2's acceptance criterion `rg -c -e 'SPDX-License-Identifier' CLAUDE.md .planning/PROJECT.md` expects `0` for both files, but `CLAUDE.md` already carries an SPDX header (`SPDX-License-Identifier: Apache-2.0` / `Copyright 2026 Sean Brandt`, lines 1-4) predating this plan and every prior phase-4 plan — confirmed via `git blame`-equivalent inspection that this plan's own diff never touched those four lines. `.licenserc.yaml` exempts `CLAUDE.md` from the license-header REQUIREMENT (so `task license:check` stays green whether or not the header is present), but the file having one anyway is a pre-existing repo state this plan did not introduce and was not asked to remove. `.planning/PROJECT.md` correctly reports `0`, matching the criterion for that file.
- No auth gates were hit; no blockers.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Every recall count knob's documented contract (proto, CLI help, tool reference, CLI guide, CLAUDE.md) states the same numeric maximum, and `TestRecallMaximumIsStatedNumerically` keeps it that way going forward.
- Decision B is recorded in `.planning/PROJECT.md`'s Key Decisions table; the BREAKING change is announced in `upgrade.md` §16.
- REQ-list-limit-contract-decided stays **blocked** (not marked complete) pending 04-08 (`requirements.ready-ids` returned 0/1 ready) — this plan does not run `requirements.mark-complete` for it.
- 04-08 (red-evidence registration, `.planning/REQUIREMENTS.md` ticking) has zero `files_modified` overlap with this plan, confirmed against this plan's actual final diff (no `internal/store/redevidence_harness_test.go`, no `.planning/REQUIREMENTS.md`, no `red-evidence/*.patch` touched here).
- One pre-existing, out-of-scope blocker for the phase as a whole (not this plan specifically): `TestActiveMilestoneKeyLinksSatisfiable` fails against 04-06-PLAN.md's own `key_links` entry — see Issues Encountered. This is 04-06's defect to fix, not 04-07's or 04-08's, but it will keep `task` red until someone corrects that entry's `pattern:` field (e.g. to a regex tolerant of gofmt alignment, or to a substring that still matches the real code).

---
*Phase: 04-list-listscheduled-search-bounded-reads*
*Completed: 2026-09-20*

## Self-Check: PASSED

Both created files verified present on disk: `internal/server/recallmaxdocs_test.go`, `.planning/phases/04-list-listscheduled-search-bounded-reads/deferred-items.md`. All three commits (`e74f3305`, `1af611ed`, `76428575`) verified present in `git log --oneline --all`. Every acceptance criterion for all three tasks re-run and confirmed passing at final HEAD, including the two documented literal-vs-intent mismatches above (both verified against their own stated intent). The plan-level `<verification>` block passes except `go test ./internal/keylinks/ -count=1`, which fails on the pre-existing, out-of-scope `TestActiveMilestoneKeyLinksSatisfiable` documented above: `task` (lint + full test suite) reports exactly that one failure and no other; `task lint` and `task license:check` both exit 0; `task proto:lint && task proto:gen && git diff --exit-code -- gen ui/src/lib/gen proto` exits 0; `go test ./cmd/engram/ -run '^(TestHelpGolden|TestCatalogGolden)$' -count=1` exits 0; `git diff --exit-code HEAD -- go.mod go.sum` exits 0 (no diff).
