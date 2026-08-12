---
phase: 5
slug: validation-debt-reconciliation
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-11
validated: 2026-08-12
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

**Why this file exists, and what it deliberately is not.** Research was skipped for this phase, so
nothing in the normal pipeline would have created a validation record for it — leaving Phase 5 as
the only phase in this milestone with none, which is precisely the defect Phase 5 exists to close
(`STATE.md:155`: *"do not let Phase 5 reproduce the bug it exists to close"*). It was therefore
authored at plan time. It is a **record, not a mechanism**: it ships no CI gate, no `task` target
and no committed script, per D-09. Every row below is a command a human runs once and reads.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | **None for the planning-record edits** — the phase changes markdown, one Go comment and one docs table cell. `go test -list` is used as an *oracle* (does this test name exist?), never as a suite this phase adds to. `go vet` / `gofmt` cover the one Go file. |
| **Config file** | none |
| **Quick run command** | `go vet ./internal/retrievaleval/... && gofmt -l internal/retrievaleval/` |
| **Full suite command** | `task` (lint + full repo suite, per `Taskfile.yaml`) |
| **Estimated runtime** | ~90 seconds for `task`; the `go test -list '.*' ./...` oracle build dominates at ~2-4 minutes on a cold cache, seconds after |

---

## Sampling Rate

- **After every task commit:** the structural `rg` assertion for that task's row, plus
  `go vet ./internal/retrievaleval/...` when the Go file was touched.
- **After every plan wave:** `task` (full lint + repo suite) — confirms no planning edit tripped
  `rumdl`, and that the comment edit did not become a code change.
- **Before `/gsd-verify-work`:** `task` green **and** every row below re-run and read, not merely
  exited. The whole phase is about records that report a state nobody re-checked.
- **Max feedback latency:** 90 seconds (excluding the one-time oracle build).

---

## Per-Task Verification Map

Every row is a real, runnable assertion, and every negative assertion is **paired with a positive
one** proving the matcher can match the target at all. That pairing is not decoration: an
unpaired negative grep is exactly how this milestone's forbidden-tool row (`04-VALIDATION.md`
correction 2) passed clean on a file that demonstrably violated it, and it is how markdown table
escaping turns an alternation into an impossible literal that reports zero failures.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 05-01-01 | 01 | 1 | REQ-nyquist-reconciled | T-05-01 | The fictional test name is gone AND its replacement is present — the negative is meaningless without the paired positive | structural (paired) | `V=.planning/phases/03.1-*/03.1-VALIDATION.md; rg -q 'TestSupersedeConcurrentKeyedDisjointTargetsCannotBothLand' $V && rg -q 'TestPayloadRoundTripsIdempotencyFingerprint' $V && ! rg -q 'TestSupersedeIdempotency' $V` | ✅ | ✅ green |
| 05-01-01 | 01 | 1 | REQ-nyquist-reconciled | T-05-01 | Every `-run` element in 01/02/03.1 resolves to ≥1 real test at HEAD — resolution, not exit status | structural (oracle) | `L=$(mktemp); go test -list '.*' ./... > "$L"; for p in TestExitCodeBaseline TestFlagGroup TestExitCode TestTimeout TestClientConfig TestValidateRules TestRegisterToolsEnumerable TestNormalizeFieldRoundTrip TestEveryRuleResolvesToNonEmptySurfaceSet TestNoUnregisteredConditionalRejection TestValidateOperations TestOperationsCoverEveryTool TestToolAnnotationsBothDirections TestCatalogBlastRadiusMatchesToolClasses TestCatalogExitCodesMatchMapper TestClientFilesImportBoundary TestHelpGolden TestCatalogGolden TestSupersedeMulti TestSupersedeMemory TestSupersedeMultiTOCTOU TestSupersedeMemoryIdempotency TestSupersedeConcurrentKeyedDisjointTargetsCannotBothLand TestPayloadRoundTripsIdempotencyFingerprint; do rg -q "^$p" "$L" \|\| echo "UNRESOLVED $p"; done; rm -f "$L"` prints nothing | ✅ | ✅ green |
| 05-01-02 | 01 | 1 | REQ-nyquist-reconciled | T-05-01 | Frontmatter states what resolution found; no verification-map ROW is left claiming nothing (correction 1 — the legend line is not a row) | structural | `for V in .planning/phases/01-*/01-VALIDATION.md .planning/phases/02-*/02-VALIDATION.md .planning/phases/03.1-*/03.1-VALIDATION.md; do rg -q '^status: validated' $V && rg -q '^nyquist_compliant: true' $V && rg -q 'Tests Matched' $V && test "$(rg -c '⬜ pending' $V)" = "$(rg -c '^\*Status: ⬜ pending' $V)" \|\| echo "INCOMPLETE $V"; done` prints nothing | ✅ | ✅ green |
| 05-01-04 | 01 | 1 | REQ-nyquist-reconciled | T-05-02 | The one genuinely unproven requirement in this milestone stays visibly unproven — asserted **positively**, so a matcher failure cannot fake a pass | structural (positive) | `V=.planning/phases/04-*/04-VALIDATION.md; rg -q 'REQ-consent-adversarial-proof.*⬜ pending' $V && rg -q '^nyquist_compliant: false' $V && rg -q '^status: validated' $V` | ✅ | ✅ green |
| 05-01-04 | 01 | 1 | REQ-nyquist-reconciled | T-05-02 | 04's re-anchored no-new-server-code row is a claim that stays true — both ends pinned, test files excluded | structural (diff) | `test -z "$(git diff --stat 72a32c58..b992929b -- internal/ cmd/ proto/ gen/ ':(exclude)*_test.go')"` | N/A (CI-level) | ✅ green |
| 05-01-* | 01 | 1 | REQ-nyquist-reconciled | T-05-03 | Archived milestone artifacts are untouched — including v0.12.x Phase 7's unresolvable command, which keeps its self-documented drift | structural (diff) | `test -z "$(git diff --name-only c5171386..HEAD -- .planning/milestones/)"` | N/A (CI-level) | ✅ green |
| 05-01-*, 05-02-* | 01, 02 | 1 | REQ-nyquist-reconciled | T-05-01 | No durable mechanism shipped — no gate, no `task` target, no Go code that reads planning artifacts | structural (paired) | `test -z "$(git diff --name-only c5171386..HEAD -- Taskfile.yaml .github/)" && ! rg -q '(ReadFile\|ReadDir\|Open\|Glob\|Walk\w*)\([^)]*\.planning' --glob '*.go' . && rg -q '(ReadFile\|ReadDir\|Open\|Glob\|Walk\w*)\([^)]*testdata' --glob '*.go' .` | N/A (CI-level) | ✅ green |
| 05-02-01 | 02 | 1 | REQ-citation-fixture-355 | T-05-06 | Citations name symbols that survive movement, not line numbers that do not | structural (paired) | `T=internal/retrievaleval/retrieval_eval_test.go; rg -q 'deps\.searchMemory' $T && rg -q 'store\.EmbedText' $T && ! rg -q 'tools\.go:[0-9]' $T && go vet ./internal/retrievaleval/... && test -z "$(gofmt -l internal/retrievaleval/)"` | ✅ | ✅ green |
| 05-02-01 | 02 | 1 | REQ-citation-fixture-355 | T-05-07 | The cross-ref resolves to the page that carries the referenced row, and the link is inside the OpenRouter row rather than merely somewhere on the page | structural (scoped) | `E=docs-site/src/content/docs/guides/embedding-instructions.md; rg -q 'OpenRouter.*\(/guides/embedding-models/\)' $E && ! rg -q 'see its row above' $E` | ✅ | ✅ green |
| 05-02-02 | 02 | 1 | REQ-nyquist-reconciled | T-05-05 | The disproven premise and the retired `verify` claim are gone, and ROADMAP gained no structure | structural (paired) | `! rg -q 'six at .status: draft.' .planning/REQUIREMENTS.md .planning/ROADMAP.md && ! rg -q 'calibrat' .planning/REQUIREMENTS.md .planning/ROADMAP.md && rg -q 'REQ-nyquist-reconciled' .planning/REQUIREMENTS.md && test "$(rg -c '^### Phase ' .planning/ROADMAP.md)" = "$(git show c5171386:.planning/ROADMAP.md \| rg -c '^### Phase ')"` | ✅ | ✅ green |
| — | 01, 02 | 1 | both | — | The repo gate is green: no planning edit trips `rumdl`, no comment edit trips `gofmt`, no test regressed | integration | `task` | N/A (CI-level) | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**Markdown table escaping — read this before running anything above.** Every `\|` inside the
commands in this table is *markdown table escaping*, not shell or regex syntax. Unescape each to a
single `|` before running. Read literally, `\|\|` is not a shell `||` and `A\|B` is not an
alternation — it is one impossible literal that matches nothing and reports clean. This exact trap
produced roughly ten phantom failures during this phase's own audit before it was caught
(`05-CONTEXT.md` `<code_context>`, trap 1).

**`go test -list` does not enumerate subtests.** Only the top-level element of a `TestX/sub`
pattern is verifiable by the oracle row above. Every element listed there is top-level, and no row
in this phase asserts anything about a subtest. Stated rather than assumed, per trap 3.

**One correction applied 2026-08-12.**

1. **Row `05-01-02`'s pending clause was a FALSE RED and has been narrowed.** As authored, the row
   asserted `! rg -q '⬜ pending' $V` over `01`/`02`/`03.1`-VALIDATION.md. That command can never
   pass on a *correctly reconciled* file: the legend line directly beneath each verification map
   (`*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*`) defines the marker vocabulary and
   therefore necessarily contains the string. Run as written on 2026-08-12 it reported
   `INCOMPLETE` for all three files, every one of which is in fact fully reconciled — the
   boilerplate-vs-rows trap this file's own Test Infrastructure note warns about, landing in this
   file's own map. The clause now asserts that **every** `⬜ pending` occurrence in the file *is*
   the legend line — `test "$(rg -c '⬜ pending' $V)" = "$(rg -c '^\*Status: ⬜ pending' $V)"` — so
   any occurrence that is not the legend fails it. This equality form was chosen over the more
   obvious row anchor `'^\|.*⬜ pending'` deliberately: a leading-pipe regex has to survive this
   table's own `\|` escaping convention to reach the shell intact, and a matcher that is one
   unescaping mistake away from matching nothing is the very failure mode trap 1 describes. The
   equality form contains no pipe at all, so there is nothing to unescape.

   The narrowing was proved to discriminate rather than merely to pass: it returns GREEN on `01`,
   `02` and `03.1` (1 occurrence, 1 legend), and RED on `04-VALIDATION.md` (3 occurrences, 1
   legend), which genuinely carries two pending rows — `REQ-consent-never-perform` and
   `REQ-consent-adversarial-proof`. An assertion that could not go red on the one file in this
   milestone that *should* fail it would have been worth nothing.

   This is the mirror of the defect this phase was built to close. A stale `-run` pattern matching
   nothing exits 0 and reports a permanent false GREEN; an over-broad negative grep matches
   boilerplate and reports a permanent false RED. Both are assertions nobody executed against a
   known-good input before shipping them. Recorded rather than silently fixed, per the same
   discipline `04-VALIDATION.md`'s numbered corrections follow.

**Resolution counts are recorded, but the asserted bar is ≥1.** Per D-08, each reconciled row in
01/02/03.1 records the number of tests its pattern resolved to as evidence, and the *assertion* is
non-zero, never an exact count — pinning exact counts red-builds on an ordinary sibling-test
addition. Counts observed at plan time on 2026-08-11 against 1,023 names at HEAD: 01 =
4/4/7/4/3; 03.1 = 15/29/2/3 plus 1/1 for the two repointed store-side names; 02's fourteen
elements each resolved to between 1 and 4.

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements. No test file, fixture or framework install
is needed — and deliberately so: D-09 forbids adding a Go test that reads `.planning/**`, so the
seven existing doc-binding tests in this repo stay seven. `go test -list`, `go vet`, `gofmt`, `rg`
and `git diff` are all already present.

- [x] `go test -list` available — the resolution oracle (part of the Go toolchain)
- [x] `rg` available — every structural assertion above
- [x] `task` available — the repo gate (`Taskfile.yaml`)
- [x] Commits `72a32c58`, `b992929b` and `c5171386` reachable — the pinned diff ranges

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| A reconciled record reads as *honest* to a human, not merely as passing | REQ-nyquist-reconciled | The property under test is whether a future reader can tell that a command was repointed and why. A regex can confirm a `## Validation Audit` heading exists; it cannot judge whether the note says enough to be useful. This is the same judgment `04-VALIDATION.md` made when it wrote its corrections down instead of silently patching them | Read each `## Validation Audit 2026-08-11` section cold. It must name what was resolved, how (resolution against `go test -list`, not exit status), what was found to have drifted, and what was deliberately left alone. A note that only says "reconciled" fails |
| The phase shipped no durable mechanism | REQ-nyquist-reconciled | The structural row above catches a new `task` target, a workflow file or Go code reading planning artifacts, but cannot judge whether some other artifact is a gate in disguise | Review `git diff --name-only c5171386..HEAD` in full. Any new file that a future run would *execute* rather than *read* is a D-09 violation, whatever its extension |
| The corrected requirement text is a true specific, not a vague generality | REQ-nyquist-reconciled | "Did the correction preserve checkability?" is a reading judgment; the negative greps only prove the false claims are gone | Read the replacement `REQ-nyquist-reconciled` and `REQ-citation-fixture-355` bodies. Each must still assert something a later reader could falsify |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 90s
- [x] **Every assertion above was re-run and read, not merely exited.** Each row here is a `rg`,
      `git diff` or `go vet` assertion whose exit status *is* the verdict — unlike a `go test -run`
      pattern, none of them can match nothing and still exit 0. That is why this phase's own record
      uses resolution and paired assertions instead of `-run` filters. Re-run in full 2026-08-12:
      10 of 11 rows green as written; row `05-01-02` returned a false red and was narrowed (see
      correction 1).
- [x] Every negative assertion above was confirmed to have a passing positive pair — row `05-01-01`
      (both replacement names present before asserting the retired one absent), row `05-01-*`/`05-02-*`
      (3 Go files reading `testdata` prove the `.planning`-reading matcher can match at all), and
      row `05-02-01` (`deps.searchMemory` and `store.EmbedText` both present before asserting no
      `tools.go:<n>` anchor remains).
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-08-12

---

## Validation Audit 2026-08-12

Every row of the Per-Task Verification Map was executed against the tree at `3e1f1764` and read,
not merely exited. Markdown `\|` escaping was unescaped to real shell/regex operators before
running, per this file's own trap-1 note.

| Metric | Count |
|--------|-------|
| Rows total | 11 |
| Green as written | 10 |
| Gaps found | 1 |
| Resolved | 1 (correction 1 — false-red clause narrowed and proved to discriminate) |
| Escalated | 0 |
| Manual-only verifications performed | 3 of 3, all pass |

**Resolution oracle, re-measured 2026-08-12.** `go test -list '.*' ./...` reports **1,047** names at
HEAD (1,023 at plan time on 2026-08-11 — the delta is ordinary test growth across the milestone).
All 24 top-level pattern elements resolve; zero unresolved. Every count matches the figures the
reconciled records claim, independently re-derived rather than copied from them: `TestSupersedeMulti`
15, `TestSupersedeMemory` 29, `TestSupersedeMultiTOCTOU` 2, `TestSupersedeMemoryIdempotency` 3,
`TestSupersedeConcurrentKeyedDisjointTargetsCannotBothLand` 1,
`TestPayloadRoundTripsIdempotencyFingerprint` 1; `01`'s five elements 4/4/7/4/3; `02`'s fourteen
elements each between 1 and 4.

**Manual-only results.** (1) The `## Validation Audit 2026-08-11` notes in `01`, `02` and `03.1` each
name what was resolved, *how* (fresh `go test -list` at HEAD, explicitly not exit status), what was
found to have drifted and was repointed, and what was deliberately left alone (`TBD` Task ID cells,
and each file's own cautionary `go test -run X ./pkg/...` prose example) — they pass the honest-read
bar. (2) Only two non-`.planning/` files changed across the whole phase
(`internal/retrievaleval/retrieval_eval_test.go`, comments only, and one `embedding-instructions.md`
table cell); no new artifact exists that a future run would *execute* rather than read, so D-09
holds. (3) Both corrected requirement bodies remain falsifiable specifics — `REQ-nyquist-reconciled`
asserts a checkable 89-of-90 at `906a5cf6`, and `REQ-citation-fixture-355` asserts a mechanism
(`EnumerateCitations` reads stored `Citation.Excerpt` from Qdrant) that a reader can go and check.

**Observation, not a defect.** `REQ-citation-fixture-355`'s body cites
`internal/store/spine.go:296-360` — a line-range citation inside the requirement about line-number
citations drifting. Verified accurate today: `EnumerateCitations` is at `spine.go:327`, within the
range. It is planning prose rather than a source-code anchor of the kind #355 governs, so it is out
of that requirement's own scope; recorded here so a future reader who notices the irony finds it
already accounted for rather than re-opening it.

**Scope note.** No test file was generated by this audit, and none should have been. This phase
added no executable behaviour — its whole code delta is comments — and D-09 explicitly forbids
shipping a Go test that reads `.planning/**`. The `nyquist_compliant: true` verdict rests on every
requirement having automated verification through the `rg` / `git diff` / `go vet` / `go test -list`
assertions above; no requirement is manual-only, and the three manual rows are supplementary
judgment checks on requirements that also carry automated coverage.
