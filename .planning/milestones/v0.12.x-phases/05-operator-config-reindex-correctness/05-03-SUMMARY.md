---
phase: 05-operator-config-reindex-correctness
plan: 03
subsystem: docs
tags: [docs-site, starlight, markdown, operator-guide]

requires:
  - phase: 05-operator-config-reindex-correctness (05-01)
    provides: "ENGRAM_OPENAI_CHAT_API_KEY / OpenAIConfig.ChatAPIKey, memory.summarize.chatApiKeySecret Helm value"
  - phase: 05-operator-config-reindex-correctness (05-02)
    provides: "the three-conjunct resume skip predicate, ReindexResult.WouldUpsert, reindexSummary's dry-run-with-resume wording"
provides:
  - "configure.md's per-lane chat credential section, corrected of the three false shared-key statements"
  - "reindex.md's Repairing a pre-patch resume section (mechanism, procedure, D-15 limit)"
  - "upgrade.md's v0.12.0 ### 4 and ### 5 entries"
  - "phase-close gate results for the whole of v0.12.x Phase 5"
affects: []

actuals:
  tokens: 3580
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Docs corrections stated as replacements of falsified prose, not deletions — the residual risk a callout explained survives in corrected form even when the constraint it was attached to becomes false"

key-files:
  created: []
  modified:
    - docs-site/src/content/docs/guides/configure.md
    - docs-site/src/content/docs/guides/reindex.md
    - docs-site/src/content/docs/guides/upgrade.md

key-decisions:
  - "D-06 followed as a correction, not a deletion: the shared-key callout's residual credential-exposure warning was rewritten as the opt-out consequence of the inherit-by-default fallback, not removed alongside the now-false shared-key constraint it used to justify."
  - "D-13 followed literally: the repair section names only 'engram reindex --resume' and its --dry-run form. No repair command was invented; the negative gate (! rg -qi 'engram repair') confirms it."
  - "D-15 stated as a plain limit with the mechanism given first (source re-scrolled fresh, target never self-corrects) so the limit reads as a consequence, not a floating caveat — per the plan's key_links requirement."
  - "Also corrected the --resume flag-table row's description ('skip target points that already hold identical content') to name the tags and identity conjuncts, even though the plan's action text only explicitly named --dry-run for that table — the old wording was the same class of stale-prose defect D-06 flags, sitting one row above the text the plan did ask to fix (Rule 1 - bug, scoped to the file already being edited in this task)."

patterns-established: []

requirements-completed: [REQ-per-lane-api-key, REQ-reindex-stale-repair]

coverage:
  - id: D1
    description: "configure.md no longer asserts a shared key across both lanes; documents ENGRAM_OPENAI_CHAT_API_KEY and memory.summarize.chatApiKeySecret; the residual credential-exposure warning survives in opt-out form."
    requirement: "REQ-per-lane-api-key"
    verification:
      - kind: other
        ref: "Task 1 <verify> gates (8 rg/task checks) — all PASS; see Phase-Close Gate Results table"
        status: pass
    human_judgment: false
  - id: D2
    description: "reindex.md documents the three-conjunct resume predicate (content, order-independent tags, embedder identity), the --dry-run --resume repair-sizing output, and the Repairing a pre-patch resume section (what went wrong, mechanism, procedure, D-15 limit)."
    requirement: "REQ-reindex-stale-repair"
    verification:
      - kind: other
        ref: "Task 2 <verify> gates (7 rg/awk/task checks) — all PASS; see Phase-Close Gate Results table"
        status: pass
    human_judgment: false
  - id: D3
    description: "upgrade.md's v0.12.0 section gains ### 4 (chat credential) and ### 5 (resume fix) continuing the existing numbering; the lead paragraph no longer promises exactly three argument-rejection changes."
    requirement: "REQ-per-lane-api-key"
    verification:
      - kind: other
        ref: "Task 3 doc-specific <verify> gates (6 rg/task checks) — all PASS; see Phase-Close Gate Results table"
        status: pass
    human_judgment: false
  - id: D4
    description: "Every phase-close gate, prohibition gate, and edge gate named in this plan's <verification> block is green on the final tree."
    requirement: "REQ-reindex-stale-repair"
    verification:
      - kind: other
        ref: "task; go vet ./...; task license:check; task chart:validate; task proto:lint; task proto:gen (zero drift); task ui:build (zero drift); git diff --exit-code dc98ec0c -- go.mod go.sum; 6 prohibition gates; 4 edge gates (TestReindexResumeTags, TestReindexDryRunWritesNothing, TestSummarizerFromConfigChatAPIKey, go.mod/go.sum diff)"
        status: pass
    human_judgment: false

duration: 7min
completed: 2026-08-01
status: complete
---

# Phase 5 Plan 03: Operator Docs & Phase-Close Gates Summary

**Corrects `configure.md`'s now-false shared-key assertion, adds `reindex.md`'s pre-patch-resume repair section with its stated D-15 limit, extends `upgrade.md`'s v0.12.0 entries to five, and closes the whole phase with every gate green.**

## Performance

- **Duration:** ~7 min (18:31:15 to 18:37:45, from the prior plan's last commit to this plan's last task commit)
- **Started:** 2026-08-01T18:31:15-04:00 (approx, prior commit)
- **Completed:** 2026-08-01T18:37:45-04:00
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments

- `configure.md`: added the `ENGRAM_OPENAI_CHAT_API_KEY` table row directly after `ENGRAM_OPENAI_CHAT_BASE_URL`; retitled and rewrote the shared-key callout to "Each lane can carry its own API key" — removing the three now-false statements (no separate key exists, the key is shared across both lanes, per-lane credentials are unsupported this milestone) while preserving the residual credential-exposure warning in corrected, opt-out form (T-05-03); fixed both stale cross-references (the `ENGRAM_OPENAI_CHAT_BASE_URL` row's pointer and the Auto-summary section's back-reference to Embedder); added the `memory.summarize.chatApiKeySecret` Helm mention.
- `reindex.md`: rewrote `## Resuming an interrupted run` from a single content-equality claim to the actual three-part conjunction (content, order-independent tag set, embedder identity), stating the tag-reorder residual as deliberate; updated the `--dry-run`/`--resume` flag-table rows and the `## Output` section with the actual `dry-run --resume` summary wording quoted from `cmd/engram/reindex.go`; added `## Repairing a pre-patch resume` (what went wrong, the re-scroll-fresh mechanism stated before the limit as its cause, the size-then-run procedure naming only the existing `--resume`/`--dry-run` flags, and the D-15 deleted-source limit stated plainly with no best-effort recovery implied).
- `upgrade.md`: widened the v0.12.0 lead paragraph from "three wire-visible changes" to five changes total; added `### 4` (per-lane chat credential, no action required unless the operator has repointed `ENGRAM_OPENAI_CHAT_BASE_URL`) and `### 5` (the resume tags defect, prescribing `--dry-run --resume` then `--resume`, linking to the reindex guide's repair section) continuing the existing numbering with 1-3 left unmodified.
- Ran the full phase-close gate set (`task`, `go vet ./...`, `task license:check`, `task chart:validate`, `task proto:lint`, `task proto:gen`/`task ui:build` zero-drift, `go.mod`/`go.sum` zero diff vs `dc98ec0c`), every phase-wide prohibition gate, and every phase-wide edge gate — all green, all results recorded below.
- Marked `REQ-reindex-stale-repair` complete in `REQUIREMENTS.md` (checkbox + traceability table) now that D-16's documentation has landed; hand-edited `ROADMAP.md`'s Phase 5 checkbox, the 05-03 plan checkbox, and the progress-table row (3/3, Complete, 2026-08-01) per this plan's explicit warning that `gsd-tools`' `roadmap.update-plan-progress`/`state.advance-plan` are unreliable on this repo's flat ROADMAP/STATE shape — `git diff` on both files confirmed clean, correctly-formatted diffs before committing.

## Phase-Close Gate Results

All gates run after Task 3's edits, on the tree with all three of this plan's commits landed.

| Gate | Command | Result |
|------|---------|--------|
| Lint + full suite | `task` | PASS — golangci-lint, actionlint, rumdl, yamlfmt, ruff (check+format), pytest (33 passed), `go test ./...` all packages ok |
| Vet | `go vet ./...` | PASS (exit 0) |
| License headers | `task license:check` | PASS — 241 valid, 0 invalid |
| Chart drift + render | `task chart:validate` | PASS — default render omits CronJob, `cronjob.enabled=true` render emits it with `Forbid`/daily schedule, `helm lint` clean, `EXPECTED_CHECKSUM` (re-pinned by 05-01) matches |
| Proto lint | `task proto:lint` | PASS — buf lint clean, no `NO_SIDE_EFFECTS` idempotency level |
| Proto codegen drift | `task proto:gen` then `git status --porcelain` | PASS — zero drift (only this plan's in-flight docs edit showed) |
| UI build drift | `task ui:build` then `git status --porcelain` | PASS — zero drift (only this plan's in-flight docs edit showed) |
| Zero new deps | `git diff --exit-code dc98ec0c -- go.mod go.sum` | PASS (exit 0) |

### Phase-wide prohibition gates

| Gate | Result |
|------|--------|
| `ChatAPIKey` never reaches `slog.`/`fmt.Errorf`/`Printf`/`Println` in non-test Go | PASS |
| Embedder still passes `cfg.OpenAI.APIKey` unchanged to `embed.New` | PASS |
| No hunk header in `internal/store/store.go` names `func payload(` or `func EmbedText(` | PASS |
| `internal/authz` zero diff from `dc98ec0c` | PASS |
| `internal/config/validate.go` zero diff from `dc98ec0c` | PASS |
| `cmd/engram/` gains no new file — diff lists only `reindex.go`/`reindex_test.go` | PASS |

### Phase-wide edge gates

| Edge | Gate | Result |
|------|------|--------|
| 1-4 (tags-only re-embed, positive control, reorder skips, nil==empty) | `go test ./internal/store/... -run 'TestReindexResumeTags$' -v -count=1` → `^--- PASS: TestReindexResumeTags \(` | PASS, no `--- SKIP` in output |
| 5 (`--dry-run` writes nothing) | `go test ./internal/store/... -run 'TestReindexDryRun' -v -count=1` → `^--- PASS: TestReindexDryRunWritesNothing \(` | PASS, no `--- SKIP` in output |
| 6 (chat key unset is byte-identical) | `go test ./internal/server/... -run 'TestSummarizerFromConfigChatAPIKey$' -v -count=1` → `^--- PASS: TestSummarizerFromConfigChatAPIKey \(` | PASS, no `--- SKIP` in output |
| 7 (`go.mod`/`go.sum` unchanged from phase base) | `git diff --exit-code dc98ec0c -- go.mod go.sum` | PASS |

## Task Commits

1. **Task 1: Correct configure.md's shared-key assertion and document the new per-lane credential** - `6f260fae` (docs)
2. **Task 2: Document the stale-record repair path and its hard limit in the reindex guide** - `6d2b622d` (docs)
3. **Task 3: Add the two v0.12.0 upgrade entries and run the phase-close gates** - `b922d715` (docs)

**Plan metadata:** (this commit, docs)

## Files Created/Modified

- `docs-site/src/content/docs/guides/configure.md` - `ENGRAM_OPENAI_CHAT_API_KEY` table row, corrected/retitled shared-key callout, fixed cross-references, Helm value mention
- `docs-site/src/content/docs/guides/reindex.md` - three-conjunct resume predicate description, `--dry-run --resume` output wording, `## Repairing a pre-patch resume` section
- `docs-site/src/content/docs/guides/upgrade.md` - widened v0.12.0 lead paragraph, `### 4` and `### 5` subsections
- `.planning/REQUIREMENTS.md` - `REQ-reindex-stale-repair` marked complete (checkbox + traceability table)
- `.planning/ROADMAP.md` - Phase 5 checkbox, 05-03 plan checkbox, progress-table row marked complete
- `.planning/STATE.md` - position, decisions, session updated (this commit)

## Decisions Made

- Followed D-06 as a correction, not a deletion — the callout's real residual risk (an unset chat key sends the embedder's key to whatever gateway the chat base URL points at) survives in opt-out form, gated by its own acceptance criterion rather than only the negative greps removing the false sentences.
- Followed D-13 literally: no repair command was invented. The `--resume`/`--dry-run --resume` pairing is presented as the repair path on its own terms, matching the plan's explicit departure from REQ-reindex-stale-repair's literal `backfill-short-ids`-precedent wording.
- Stated D-15's limit as a direct consequence of the re-scroll-fresh mechanism (source authoritative, target never self-corrects), per the plan's key_links requirement that the mechanism precede the limit rather than the limit floating unexplained.
- Also corrected the `--resume` flag-table row's stale "identical content" description while already editing that file for Task 2 — the same class of falsified-prose defect the task's own acceptance criteria target, one table row above the text explicitly named (Rule 1 - bug fix, no separate task/commit).
- Hand-edited `ROADMAP.md`/`REQUIREMENTS.md`/`STATE.md` rather than invoking `gsd-tools query roadmap.update-plan-progress`/`state.advance-plan`, per this plan's explicit operating note that both handlers have corrupted this repo's flat-ROADMAP/hand-maintained-STATE shape in prior phases; `git diff` confirmed clean formatting on both files before committing.

## Deviations from Plan

None beyond the Rule-1 flag-table row correction documented above (same file, same task, same class of defect the task already required fixing) — the plan's own tasks, verify gates, and acceptance criteria were followed as written, and all three tasks' automated verify gates passed on first attempt.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. This plan is documentation-only plus the phase-close gate run; no new operator-facing configuration was introduced (05-01 and 05-02 already shipped the config surface this plan documents).

## Next Phase Readiness

- **v0.12.x Phase 5 (Operator Config & Reindex Correctness) is COMPLETE.** All three plans (05-01, 05-02, 05-03) landed; all three requirements (`REQ-per-lane-api-key`, `REQ-reindex-resume-tags`, `REQ-reindex-stale-repair`) are marked complete in `REQUIREMENTS.md`.
- Every phase-close, prohibition, and edge gate is green on the final tree (table above); `go.mod`/`go.sum` confirmed zero diff from the phase base commit `dc98ec0c` across the whole phase.
- The branch remains on `phase-01-shared-auth-chain-connect-bearer-identity` (`git.branching_strategy: none`, whole milestone rides one branch) and is behind `origin/main` by the same three commits (#448/#449/#450) noted since Phase 1 — zero file overlap, integrate before the PR as previously noted.
- v0.12.x Phase 6 (Rule Capture — Investigation & Fix) is the only remaining phase in this milestone.

## Self-Check: PASSED

All 3 modified doc files confirmed present on disk with the expected section headings (`rg` checks above); this SUMMARY.md confirmed present on disk; all 3 task commit hashes (`6f260fae`, `6d2b622d`, `b922d715`) confirmed in `git log --oneline -5`.

---
*Phase: 05-operator-config-reindex-correctness*
*Completed: 2026-08-01*
