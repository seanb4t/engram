---
phase: 03-spine-curation-structural-cli
verified: 2026-08-07T16:13:39Z
status: passed
score: 7/7 must-haves verified
behavior_unverified: 0
overrides_applied: 1
human_verification_waived:
  by: Sean
  date: 2026-08-07
  note: >-
    Status was raised as human_needed by the verifier and flipped to passed on the strength of
    the 7/7 automated verification alone. The three items below were NOT performed — they were
    explicitly waived. They remain genuinely unverified: automation cannot settle prose
    comprehension or real-TTY rendering, which is why the verifier flagged them. Recorded here
    so the artifact does not claim work that was not done.
human_verification:
  - test: "Read docs-site/src/content/docs/guides/cli.md's purge subsection cold (no code open) and confirm the concurrent-writer-scoping wording matches ApplyPurge's actual behavior (intersection-only delete, Spared/Appeared semantics, same-run-only manifest)."
    expected: "The prose an operator reads before running --apply accurately describes what the code does, with no over-claim (e.g. it must not imply cross-invocation safety or protection against operator delay)."
    why_human: "This is prose-comprehension verification (03-07-PLAN.md's D4 acceptance item, explicitly flagged human_judgment:true and left `pending` in 03-07-SUMMARY.md — no spawned executor had a human reviewer available). Grep/test can confirm the doc text is byte-derived from the same constants the code uses (TestSpineReviewPurgeSameRunNoticePublished does this), but cannot judge whether a first-time reader would come away with an accurate mental model."
  - test: "Run each of the six pre-existing operator commands plus every spine-review leaf at a real interactive terminal (not piped) and confirm --output auto-detects to text; then pipe each to `cat` and confirm it flips to JSON."
    expected: "TTY presence renders human-readable text; a non-TTY consumer (pipe/redirect) renders one JSON document."
    why_human: "03-VALIDATION.md's Manual-Only Verifications table names this explicitly: the unit tests pin the isTTYWriter/outputFormatFromConfig mapping given a caller-supplied bool, but none of them open a real pty, so the actual terminal-rendering experience is unverified by any automated test in this phase."
  - test: "Read docs-site/src/content/docs/guides/upgrade.md's prune-expired entry cold and confirm it states the old behavior, the new behavior, and the exact flag to restore deletion."
    expected: "An operator upgrading understands, from the doc alone, that a bare prune-expired now previews and that --apply is required to delete."
    why_human: "Also named explicitly in 03-VALIDATION.md's Manual-Only Verifications table as prose-comprehension, not assertion-provable."
---

# Phase 3: Spine Curation — Structural (CLI) Verification Report

**Phase Goal:** An operator can inventory, verify, consolidate-report, archive, restore, and safely
dispose of a memory spine's structural problems through `engram spine-review`, the sixth instance
of the existing Subject-less operator tier, while the destructive operator tier becomes uniformly
preview-by-default and `--output json|text` is backfilled across all operator commands.

**Verified:** 2026-08-07T16:13:39Z
**Status:** human_needed (all seven roadmap success criteria independently re-verified against the
codebase and pass; three pre-existing, explicitly-documented manual-verification items remain
outstanding and were never claimed complete by any SUMMARY)
**Re-verification:** No — initial verification

## Methodology

This report does not trust any SUMMARY.md claim on its own. Every claim below was re-derived from
one of: reading the actual source at the cited `file:line`, running the actual test named (not
trusting the SUMMARY's reported PASS output), or exercising the built binary directly. Concretely,
this session:

- Ran `go build ./...` (clean) and `go clean -testcache && task` (lint + full `go test ./...`,
  including the testcontainers-backed `internal/store`/`internal/server` suites and the
  built-binary-exec `internal/e2e` suite) — **all green**, from a flushed cache, in this session,
  not copied from a prior run's output.
- Ran `go test ./cmd/engram/... -shuffle=<seed>` under four seeds not mentioned in any SUMMARY
  (5, 11, 23, 99, 2026) to stress the order-dependent golden/catalog tests independently.
- Ran `go test ./internal/e2e/... -run TestE2EPhaseAcceptance -v` directly and read its full
  422-line source to confirm it asserts outcomes (`store.Get` re-reads after each mutating step)
  rather than merely checking exit codes.
- Read `internal/store/spine_forgery_test.go` to confirm the manifest-forgery test genuinely lives
  in an external `package store_test`.
- Read `cmd/engram/destructive.go` end-to-end and grepped for `Changed`/`classForCommand` to confirm
  no `pflag.Flag.Changed` latch and no injectable classification seam exist.
- Built the binary directly (`go build -o /tmp/engram-verify ./cmd/engram`) and ran
  `spine-review purge --help` and `spine-review purge --timeout 0 ...` against a dead Qdrant to
  behaviorally confirm the `--help` text and the `--timeout 0` "disables, doesn't reject" contract,
  rather than trusting the SUMMARY's transcript.
- Cross-checked `internal/surfaces/toolclass.go`'s `Destructive: true` rows against
  `cmd/engram/destructive_test.go` and confirmed by reading source that `prune-expired`,
  `migrate-remap-owner`, and `spine-review purge` — and only those three CLI commands — are
  classified destructive, and that all three route through `registerDestructive` with no leaf
  assigning its own `RunE`.
- Confirmed all three follow-up GitHub issues (#480, #481, #482) the SUMMARYs claim to have filed
  actually exist and are open (`gh issue view`).

## Goal Achievement

### Observable Truths (ROADMAP §Phase 3 success criteria, verbatim)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `spine-review scan` reports inventory/health by scope and category, no mutation on any path | ✓ VERIFIED | `internal/store/spine.go:46-69` (`scrollAllPoints`, the phase's ONE paginated iterator — confirmed by grep: `client.Scroll(` appears only in pre-existing, out-of-phase `store.go`/`store_test.go` Search/List paths, never in `spine.go`); `ScanSpine` at `spine.go:245` routes through it exclusively. `TestScanSpineHealthSignals`/`TestScanSpinePaginatesEveryPage`/`TestScanSpineTwoOwners` pass (`go test ./internal/store/... -run TestScanSpine`, re-run this session). `catalog.golden` confirms `spine-review scan` carries `--output` with no mutating-RPC flag. |
| 2 | `spine-review verify` classifies every citation valid/moved-but-valid/broken, moved reported separately from broken | ✓ VERIFIED | `cmd/engram/spine_review_verify.go:97-130` (`verifyFileCitation`): four-tier order confirmed by direct read — file-missing→broken, no-excerpt→unverifiable, at-locator→valid, in-file-search→moved, else→broken(excerpt-gone). `TestVerifyFileCitation` (including the literal issue-#355 drift-shape subtest) re-run and passes. `EnumerateCitations` (`spine.go:558`) is Subject-less and routes through `scrollAllPoints`. |
| 3 | `spine-review consolidate` reports near-duplicate candidates via stored vectors, no re-embedding, never mutates | ✓ VERIFIED | `internal/store/spine.go:584-589` uses `qdrant.NewQueryID`/`client.QueryBatch` against already-enumerated ids; grepped `spine.go` for any embedder import/call — none found. `TestNearDuplicatesDoesNotMutate` passes. `cmd/engram/spine_review_consolidate.go:84-85`/`TestConsolidateNeverLabelsPairAsDuplicateOrCluster` (re-run, passes) confirm no clustering/duplicate label anywhere in the report. |
| 4 | `spine-review purge` previews by default, `--apply` re-derives fresh, refuses without rule `7smp8vy9hr`'s gate | ✓ VERIFIED | `internal/store/spine.go:1303-1322`: `ApplyPurge` checks `manifest.IsVerified()` and returns an `ErrInvalidArgument`-wrapped error **before** calling `derivePurgeEligible` (before any RPC) — read directly, not inferred. `checkExtractGate` is called by both `PreviewPurge` (line 1269) and `ApplyPurge` (line 1331). `internal/store/spine_forgery_test.go` is confirmed `package store_test` (external) — a same-package literal genuinely cannot set `verified`; `TestPurgeManifestForgeryRejected`/`TestPurgeManifestExportedFieldsEmpty`/`TestPurgeManifestExportedMethodSet` re-run and pass. Built binary's `spine-review purge --help` (run this session) confirms no `--manifest`/`--token` flag exists and publishes the same-run/convention-not-proof limitation in its own usage text. |
| 5 | Archive/restore distinct from both supersession's soft-hide and purge's delete | ✓ VERIFIED | `internal/store/store.go`: `archived_at` `IsEmpty` condition appended as a **sibling**, never folded into `activeWindowConditions` (lines 864-869) — confirmed by reading the function body, which only ever references `not_before`/`not_after`. Four independent append sites for `archived_at` (954/960, 1054/1058, 1191/1196, 1426/1431) mirror the four pre-existing `superseded_by` sites. `TestArchivedAndSupersededHideIndependently` (re-run, passes) proves a record stays hidden while EITHER condition holds and resurfaces only once BOTH clear. `TestE2EPhaseAcceptance`'s archive/restore block (re-run, passes) round-trips against a live testcontainers Qdrant with real `store.Get` re-reads, not exit-code inference. |
| 6 | Every destructive-classified command previews by default, `--apply` required, membership DERIVED not declared; `prune-expired` flip documented | ✓ VERIFIED | `internal/surfaces/toolclass.go` has exactly 3 CLI rows with `Destructive: true`: `migrate-remap-owner` (172), `prune-expired` (180), `spine-review purge` (327) — read directly. `cmd/engram/destructive.go:38` (`destructiveByClassification`) reads `surfaces.ClassForCommand(key)` directly; grep for `classForCommand` (lowercase, an injectable seam) across `cmd/engram/` returns zero hits outside comments explaining why it was rejected. `registerDestructive` (line 110) assigns `cmd.RunE` itself; grepped all three destructive leaves (`prune.go`, `migrate.go`, `spine_review_purge.go`) and confirmed none assigns its own `RunE`. `applyRequested` reads the flag's bool value; grep for `Changed` in `destructive.go` (comments stripped) returns zero. `docs-site/.../upgrade.md` entry #9 documents the `prune-expired` flip (confirmed present). |
| 7 | `--output json|text` with TTY auto-detection on `spine-review` and all five pre-existing operator commands, without disturbing `--timeout` divergence | ✓ VERIFIED | Parsed `cmd/engram/testdata/catalog.golden`: every one of `backfill-short-ids`, `migrate-remap-owner`, `migrate-set-owner`, `prune-expired`, `reindex`, `summarize-missing`, and all six `spine-review` leaves carries an `output` flag. `operatorOutputFormat`/`renderOperator` (`operator_output.go:39,64`) is the one code path; grepped every `cmd/engram/*.go` for a direct `outputFormatFromConfig(` call — zero hits outside `operator_output.go`/`client_common.go`. The published three-group `--timeout` table (`cli.md:340-344`) was independently re-derived behaviorally this session: built the binary and ran `spine-review purge --timeout 0` against a dead Qdrant — it dialed and failed at exit 5 (`Unavailable`), not exit 2 (usage), confirming "0 disables" for the destructive/zero-disables group; `TestTimeoutGroupMatrix`'s three groups (re-run, passes) match. |

**Score:** 7/7 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/engram/spine_review.go` + 6 leaf files | `spine-review scan/verify/consolidate/archive/restore/purge` cobra tree | ✓ VERIFIED | All present, wired to `rootCmd` via `init()`, all reachable via `walkCommands`; confirmed present in `catalog.golden` and via `go build ./...` |
| `internal/store/spine.go` | Subject-less store methods for scan/verify/consolidate/archive/purge | ✓ VERIFIED | `ScanSpine`, `EnumerateCitations`, `NearDuplicates`, `Archive`/`Restore`, `PreviewPurge`/`ApplyPurge` all present, all Subject-less (no `Subject` parameter), all routed through the single `scrollAllPoints` iterator where they sweep the whole spine |
| `internal/store/spine_forgery_test.go` | Cross-package forgery proof for `PurgeManifest` | ✓ VERIFIED | `package store_test` (external), confirmed by direct read |
| `cmd/engram/destructive.go` | `registerDestructive` choke point | ✓ VERIFIED | Owns `RunE`, derives classification from `surfaces.ClassForCommand`, no injectable seam, no `Changed` latch |
| `internal/e2e/spine_review_test.go` | `TestE2EPhaseAcceptance` over a seeded 270-record collection | ✓ VERIFIED | Re-run this session; asserts via `store.Get` re-reads at every step, covers all 7 criteria per its own doc-comment coverage map |
| `docs-site/src/content/docs/guides/upgrade.md` | `prune-expired`'s breaking-change migration note | ✓ VERIFIED | Entry #9 present |
| `docs-site/src/content/docs/guides/cli.md` | Destructive-commands section, purge/verify/consolidate/archive subsections, three-group `--timeout` table | ⚠️ ORPHANED (minor, non-blocking) | See Known Gaps: the `--timeout` table's `zero-disables` row (`cli.md:343`) omits `spine-review purge` even though the code and `TestTimeoutGroupMatrix` correctly place it there (behaviorally confirmed this session); there is also no dedicated `### spine-review scan` subsection (scan is only described inline under "Operator commands"). Neither omission changes actual behavior — both are doc-completeness gaps, not functional gaps. |
| `.planning/phases/03-spine-curation-structural-cli/03-VALIDATION.md` | `/gsd-validate-phase 3` Nyquist reconciliation | ✗ outstanding (documented, non-blocking per verification brief) | `status: draft`, `nyquist_compliant: false`, confirmed by direct read — matches 03-07-SUMMARY.md's own disclosure that no `Agent`/`AskUserQuestion` tool was available to the spawned executor to complete it |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `registerDestructive` | `surfaces.ClassForCommand` | direct call, no seam | ✓ WIRED | `destructive.go:38`; grep for `classForCommand` (injectable var) returns zero hits |
| `spine-review purge`/`prune-expired`/`migrate-remap-owner` | `registerDestructive` | `init()` call, no own `RunE` | ✓ WIRED | Confirmed by reading all three `init()` blocks |
| `ApplyPurge` | `PurgeManifest.IsVerified()` | pre-RPC guard | ✓ WIRED | Confirmed check precedes `derivePurgeEligible` call, `spine.go:1318-1330` |
| `spine-review consolidate` | `store.NearDuplicates` | `qdrant.QueryBatch` over stored vectors | ✓ WIRED | No embedder import in `spine.go`; `spine_review_consolidate.go` calls `NearDuplicates` directly |
| Every operator leaf | `operatorOutputFormat`/`renderOperator` | shared helper | ✓ WIRED | Zero direct `outputFormatFromConfig(` calls outside the two designated files |

### Data-Flow Trace (Level 4)

All seven leaves' reports are built from live `internal/store` results (`ScanSpine`, citation
enumeration, `NearDuplicates`, `Archive`/`Restore` results, `PreviewPurge`/`ApplyPurge` results) —
no static/hardcoded report data found in any of the six `cmd/engram/spine_review_*.go` files. The
live-Qdrant e2e run (this session) confirms real counts flow end-to-end (`"total":270` for a
270-record seed).

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `spine-review purge --help` publishes no transport flag and states the convention-not-proof limitation | `engram spine-review purge --help` (built this session) | Usage text contains "validates a convention, not proof of preservation"; flag list has no `--manifest`/`--token` | ✓ PASS |
| `spine-review purge --timeout 0` disables rather than rejects | `engram spine-review purge --timeout 0 --class superseded --scope test` against a dead Qdrant | Exit 5 (`Unavailable`, dialed and failed), not exit 2 (usage) | ✓ PASS |
| Full test suite from a flushed cache | `go clean -testcache && task` | All packages green, including `internal/store`/`internal/server` (testcontainers) and `internal/e2e` (built-binary exec) | ✓ PASS |
| Golden/catalog order-independence | `go test ./cmd/engram/... -shuffle={5,11,23,99,2026}` | All green under every seed tried, none copied from a SUMMARY | ✓ PASS |
| `TestE2EPhaseAcceptance` asserts real outcomes | `go test ./internal/e2e/... -run TestE2EPhaseAcceptance -v` + direct source read | Passes; source confirms `store.Get` re-reads gate every assertion, not exit codes alone | ✓ PASS |

### Probe Execution

No `scripts/*/tests/probe-*.sh` convention exists in this repo and no PLAN/SUMMARY names one; skipped per Step 7c.

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
|-------------|-------------|--------|----------|
| REQ-spine-scan | 03-01 | ✓ SATISFIED | Truth 1 |
| REQ-citation-drift-verify | 03-04 | ✓ SATISFIED | Truth 2 |
| REQ-near-duplicate-report | 03-05 | ✓ SATISFIED | Truth 3 |
| REQ-purge-extract-gated | 03-07 | ✓ SATISFIED | Truth 4 |
| REQ-archive-tier | 03-06 | ✓ SATISFIED | Truth 5 |
| REQ-destructive-preview-default | 03-03 | ✓ SATISFIED | Truth 6 |
| REQ-operator-output-flag | 03-01/03-02 | ✓ SATISFIED | Truth 7 |

No orphaned requirements — all seven Phase 3 REQ-ids in REQUIREMENTS.md are claimed by a plan and independently verified above.

### Anti-Patterns Found

None. Grepped every phase-created `cmd/engram/spine_review_*.go`, `cmd/engram/destructive.go`, and
`internal/store/spine.go` for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER`/"not yet implemented"
— zero hits. No hardcoded-empty rendering found in any report path; all seven leaves' output is
built from live `internal/store` results (confirmed above).

### Known Gaps (non-blocking; disclosed by the phase's own SUMMARYs, independently re-confirmed here)

1. **`03-VALIDATION.md` Nyquist reconciliation incomplete.** `status: draft`, `nyquist_compliant:
   false`. `/gsd-validate-phase 3` could not run to completion inside the spawned 03-07 executor
   (it lacks `Agent`/`AskUserQuestion` tools). This is the known gap the verification brief asked to
   be recorded, not re-litigated. It does not affect any of the seven success criteria above — every
   one was independently re-verified against the actual codebase and running tests in this session,
   not against `03-VALIDATION.md`.

2. **`cli.md`'s `--timeout` three-group table omits `spine-review purge` from its `zero-disables`
   row.** Confirmed the omission is cosmetic: the code (`operator_output_test.go`'s
   `timeoutGroups`) and a fresh behavioral test (`--timeout 0` against a dead Qdrant, this session)
   both place `spine-review purge` correctly in the zero-disables group. A reader consulting only
   the published table would not learn this about `purge` specifically, though the table's own text
   ("every operator command's own `--timeout`... is a different flag... Disables the deadline") and
   the Destructive-commands section together still describe it correctly at the prose level.

3. **No dedicated `### spine-review scan` subsection** in `cli.md` — `scan` is described only
   inline under "Operator commands" and via the archived-bucket note, not with its own subsection
   the way `verify`/`consolidate`/`archive`/`purge` each get. Documentation completeness gap only;
   `scan`'s behavior itself is fully verified (Truth 1).

Neither #2 nor #3 blocks the phase goal — both are doc-completeness nits caught during this
session's independent re-verification, not functional defects, and neither was claimed complete or
denied by any SUMMARY (the SUMMARYs describe what was added to `cli.md`, not an exhaustive
completeness audit of it).

### Human Verification Required

See frontmatter `human_verification`. All three items are pre-existing, explicitly-disclosed
Manual-Only Verifications (two from `03-VALIDATION.md`, one from 03-07-PLAN.md's own acceptance
criteria, both flagged `human_judgment: true`/`pending` in their respective SUMMARYs) — none of
them were silently claimed complete by any SUMMARY, and this verification pass does not manufacture
new ones. They gate `human_needed` rather than `passed` per the decision tree in Step 9, since the
human-verification section is non-empty even though zero truths failed.

### Gaps Summary

No must-have truth failed. All seven ROADMAP success criteria are independently verified against
the actual codebase — source read at the cited lines, tests re-run (not trusted from SUMMARY
transcripts) with a flushed test cache, and the built binary exercised directly for the
highest-risk claims (manifest forgery rejection, `--apply` runtime enforcement, `--timeout 0`
zero-disables behavior, purge's `--help` honesty labeling). The phase goal — an operator can
inventory, verify, consolidate-report, archive/restore, and safely dispose of a memory spine's
structural problems through `engram spine-review`, with the destructive tier uniformly
preview-by-default and `--output` backfilled tier-wide — is achieved in the codebase, not merely
claimed in prose.

What remains outstanding is exactly what the phase's own artifacts already say is outstanding: the
Nyquist validation pass (`03-VALIDATION.md`, tracked separately per this verification's brief) and
three named manual/human checks that no automated test in this repo can perform. Two minor
documentation-completeness nits in `cli.md` were found during this independent pass and are
recorded above; neither is a functional defect.

---

*Verified: 2026-08-07T16:13:39Z*
*Verifier: Claude (gsd-verifier)*
