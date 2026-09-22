---
phase: 02-error-classification-resourceexhausted-mapping
plan: 03
subsystem: cli
tags: [cli, exit-codes, connect, docs-gate, argerror, go-parser]

requires:
  - phase: 02-error-classification-resourceexhausted-mapping (plan 02)
    provides: "Connect connectError arm mapping store.ErrResponseTooLarge -> CodeResourceExhausted; MCP mapResponseTooLarge middleware; argerror.go's HintTooLarge (11th HintCode) and renderHintEnvelope/responseTooLargeEnvelope()"
provides:
  - "exitTooLarge = 10, a dedicated case in exitCodeForConnectErr(CodeResourceExhausted) -> exitTooLarge, and the same code applied to the operator tier via classifyOperatorErr's store.ErrResponseTooLarge arm"
  - "the self-describe catalog and its golden advertising exit code 10"
  - "docs-site reference/errors.md's eleven-code hint-code table (too_large row) and its new resource_exhausted -> exit 10 section, mechanically gated against argerror.go by internal/server/hintcodedocs_test.go"
  - "docs-site guides/cli.md exit-code-10 row and guides/upgrade.md entry 14"
affects: ["02-04"]

actuals:
  tokens: 10062
  tasks: 3
  commits: 3
plan_head_before: 2a7236589c112da4258c3ca60627bd262986e98b

tech-stack:
  added: []
  patterns:
    - "A doc gate deriving a published vocabulary from source via go/parser (argerror.go's HintCode const block) rather than a second hand-typed list, mirroring conditionalsweep_test.go's house style; the gate checks BOTH directions (missing/invented codes), the heading's count word, every OTHER count-word phrase on the page via two regex shapes, the rendered envelope example verbatim, and every cross-page anchor link to the heading — closing RESEARCH.md Pitfall 5 (a doc claim with no mechanical enforcement)."
    - "A dynamically-addressed stub-server row in a static exit-code baseline table (tooLargeServer/tooLargeServerPlaceholder/substituteTooLargeServerURL), mirroring the existing hungServer pattern for the identical reason: the static table cannot hard-code a per-run httptest.Server URL."

key-files:
  created:
    - internal/server/hintcodedocs_test.go
  modified:
    - cmd/engram/client_common.go
    - cmd/engram/client_common_test.go
    - cmd/engram/catalog.go
    - cmd/engram/catalog_test.go
    - cmd/engram/testdata/catalog.golden
    - cmd/engram/exitcode_baseline_test.go
    - cmd/engram/operror.go
    - cmd/engram/operror_test.go
    - docs-site/src/content/docs/guides/cli.md
    - docs-site/src/content/docs/reference/errors.md
    - docs-site/src/content/docs/guides/upgrade.md

key-decisions:
  - "exitTooLarge placed as its own case in exitCodeForConnectErr's switch (never folded into the default arm) and in classifyOperatorErr's switch (placed after the ErrShortIDExhausted arm, per the plan's action item) — same published code (10) reachable from both tiers (D-10)."
  - "hintcodedocs_test.go derives the vocabulary via parser.ParseFile(fset, \"argerror.go\", nil, 0) walking every const GenDecl for a HintCode-typed ValueSpec, never a second hand-typed constant list — matches the plan's explicit call-shape requirement and closes the D-05 verification: internal/surfaces declares conditional-rule sentences only, never the hint-code vocabulary itself, so this gate — not surfaces' single-declaration convention — is what keeps errors.md honest."
  - "The response-too-large stub server used by the two new exitcode_baseline_test.go rows reproduces responseTooLargeEnvelope()'s exact rendered text as a literal string rather than importing internal/server (cmd/engram cannot reach that unexported renderer, and rule m45p2b4bp7 forbids asserting a third party's behavior as the oracle here — this is our own code's literal, reproduced once)."

patterns-established:
  - "A markdown doc-gate test that cuts a heading's 'section' at the next top-level heading before scanning for table rows (parseHintCodeTable), so a same-shaped table under a LATER heading is never miscounted — proven by its own inline positive/negative fixture tests, independent of the live doc."

requirements-completed: [REQ-exhausted-cli-docs]

coverage:
  - id: D1
    description: "engram list/search against a resource_exhausted server exit 10 through the real command path (runClient), pinned as two exitCodeBaseline rows (before: exitGeneric, after: exitTooLarge, landed: true) plus TestExitCodeTooLargeDistinct's literal-and-distinctness pin"
    requirement: "REQ-exhausted-cli-docs"
    verification:
      - kind: integration
        ref: "cmd/engram#TestExitCodeBaseline/list/response-too-large"
        status: pass
      - kind: integration
        ref: "cmd/engram#TestExitCodeBaseline/search/response-too-large"
        status: pass
      - kind: unit
        ref: "cmd/engram#TestExitCodeTooLargeDistinct"
        status: pass
    human_judgment: false
  - id: D2
    description: "The self-describe catalog and its committed golden advertise exit code 10, and TestCatalogExitCodesMatchMapper's set-equality derivation stays exact (catalog set == mapper-producible set)"
    requirement: "REQ-exhausted-cli-docs"
    verification:
      - kind: unit
        ref: "cmd/engram#TestCatalogExitCodesMatchMapper"
        status: pass
      - kind: unit
        ref: "cmd/engram#TestCatalogListsEveryExitCode"
        status: pass
      - kind: unit
        ref: "cmd/engram#TestCatalogGolden"
        status: pass
      - kind: other
        ref: "git diff --exit-code HEAD -- cmd/engram/testdata/help.golden"
        status: pass
    human_judgment: false
  - id: D3
    description: "An operator command whose own Qdrant read overflows (store.ErrResponseTooLarge reaching classifyOperatorErr directly) exits 10 too, with the operator classifier's sentinel exit-code set staying exactly {2,4,5,10}"
    requirement: "REQ-exhausted-cli-docs"
    verification:
      - kind: unit
        ref: "cmd/engram#TestClassifyOperatorErr/store.ErrResponseTooLarge"
        status: pass
      - kind: unit
        ref: "cmd/engram#TestClassifyOperatorErrCodesAreDistinct"
        status: pass
    human_judgment: false
  - id: D4
    description: "docs-site guides/cli.md's exit-code table documents 10 for both tiers, and reference/errors.md documents the eleven-code hint vocabulary (too_large row + contrast paragraph), the resource_exhausted -> exit 10 section (rendered envelope verbatim, separate from the three-argument-class table), and the no-number-carried note — mechanically gated against argerror.go by a new go/parser-based test, and guides/upgrade.md records the visible 1->10 change (entry 14) with the anchor rename followed through"
    requirement: "REQ-exhausted-cli-docs"
    verification:
      - kind: unit
        ref: "internal/server#TestErrorsDocHintCodesMatchArgErrorConstants"
        status: pass
      - kind: unit
        ref: "internal/server#TestParseHintCodeTable"
        status: pass
      - kind: unit
        ref: "cmd/engram#TestUpgradeGuideNamesEveryChangedCommand"
        status: pass
      - kind: other
        ref: "pnpm --dir docs-site build"
        status: pass
    human_judgment: false

duration: 55min
completed: 2026-09-19
status: complete
---

# Phase 2 Plan 3: Exit Code 10 End to End, and the Hint-Code Table Gate Against argerror.go Summary

**`connect.CodeResourceExhausted` now maps to a dedicated, documented CLI exit code `10` on both the Connect client tier (`list`/`search`) and the operator tier (any Qdrant-reading sweep), and `docs-site/reference/errors.md`'s hint-code table — now eleven codes including `too_large` — is mechanically bound to `argerror.go`'s source via a new go/parser-based doc gate, closing a claim the page previously made with no enforcement.**

## Performance

- **Duration:** ~55 min
- **Started:** 2026-09-19
- **Completed:** 2026-09-19
- **Tasks:** 3 completed
- **Files modified:** 12 (1 created, 11 modified)

## Accomplishments

- `cmd/engram/client_common.go` — `exitTooLarge = 10` (with a doc comment explaining the D-07 producer and remedy), and a dedicated `case connect.CodeResourceExhausted: return exitTooLarge` in `exitCodeForConnectErr`, split out of the default arm.
- `cmd/engram/client_common_test.go` — `TestExitCodeForConnectErrTable`'s `CodeResourceExhausted` row now expects `exitTooLarge`; `TestExitCodeTimeoutDistinctFromUnavailable`'s full-set assertion widened to `{0,1,2,3,4,5,6,10}`; new `TestExitCodeTooLargeDistinct` pins the literal `10` and its distinctness from `exitGeneric`/`exitUsage`/`exitUnavailable`/`exitTimeout`.
- `cmd/engram/catalog.go` + `catalog_test.go` + `testdata/catalog.golden` — the self-describe catalog advertises the new exit code (one new golden object, `help.golden` byte-unchanged); `wantExitCodes` extended.
- `cmd/engram/exitcode_baseline_test.go` — `tooLargeServer`/`tooLargeServerPlaceholder`/`substituteTooLargeServerURL` (mirroring the existing `hungServer` mechanism) plus two new rows (`list/response-too-large`, `search/response-too-large`), each observed RED (exit 1) before the mapper case existed and GREEN after; `wantRows` 39 → 41.
- `cmd/engram/operror.go` + `operror_test.go` — `classifyOperatorErr` gains a `store.ErrResponseTooLarge` arm returning `exitTooLarge`, applying D-07's code to the operator tier (D-10); `TestClassifyOperatorErrCodesAreDistinct`'s sentinel set now `{2,4,5,10}`.
- `docs-site/src/content/docs/guides/cli.md` — exit-code-10 row (naming both tiers) and an extended intro sentence.
- `docs-site/src/content/docs/reference/errors.md` — new `## Response too large: resource_exhausted and exit 10` section (Task 1); renamed heading `## The eleven hint codes`, new `too_large` table row, a `too_long`-vs-`too_large` contrast paragraph, a "carries no number at all" note in `## What is NOT in an error`, and every stale "ten"/"ten-code" phrase on the page corrected (Task 3).
- `docs-site/src/content/docs/guides/upgrade.md` — anchor renamed to `#the-eleven-hint-codes`; new `### 14.` entry (old exit 1/Connect `internal`/raw MCP text vs. new exit 10/`resource_exhausted`/scrubbed envelope) plus a Do-you-need-to-act row.
- `internal/server/hintcodedocs_test.go` (new) — `hintCodeConstants` (go/parser over `argerror.go`'s const block), `parseHintCodeTable` (heading + section-scoped table-row parser), `numberWords`, `TestParseHintCodeTable` (four inline-fixture subtests), `TestErrorsDocHintCodesMatchArgErrorConstants` (the D-05 gate itself: set equality both directions, heading word, every stale count-word phrase on the page, the rendered envelope verbatim, every cross-page anchor).

## Task Commits

Each task was committed atomically:

1. **Task 1: Exit 10 end to end** — `dfc982a9` (feat)
2. **Task 2: Operator commands exit 10 too** — `704765ad` (feat)
3. **Task 3: errors.md's hint-code table gated against argerror.go** — `f63c5517` (docs)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP/REQUIREMENTS)

## Files Created/Modified

- `cmd/engram/client_common.go` — `exitTooLarge` constant + mapper case.
- `cmd/engram/client_common_test.go` — table/set-test updates + `TestExitCodeTooLargeDistinct`.
- `cmd/engram/catalog.go` / `catalog_test.go` / `testdata/catalog.golden` — advertised exit code + golden.
- `cmd/engram/exitcode_baseline_test.go` — stub-server mechanism + two new baseline rows.
- `cmd/engram/operror.go` / `operror_test.go` — operator-tier classifier arm + tests.
- `docs-site/src/content/docs/guides/cli.md` — exit-code table row + intro sentence.
- `docs-site/src/content/docs/reference/errors.md` — eleven-code table, `too_large` row, new section, no-number note.
- `docs-site/src/content/docs/guides/upgrade.md` — entry 14, anchor rename, Do-you-need-to-act row.
- `internal/server/hintcodedocs_test.go` — the new doc gate.

## Decisions Made

See `key-decisions` in frontmatter — arm placement in both switches, the go/parser-derived vocabulary (closing the D-05 surfaces-verification question with a concrete finding), and the literal-envelope reproduction in the CLI test package rather than an import of `internal/server`.

## Deviations from Plan

### Auto-fixed Issues

None — no bugs, missing critical functionality, or blocking issues were found during implementation.

### Noted plan/acceptance-criterion mismatch (not a defect)

**Task 3's acceptance criterion `rg -o '^[|] .too_large. [|]' docs-site/src/content/docs/reference/errors.md | wc -l` prints `1`** — this literally prints `2`, not `1`. The second match is Task 1's own one-row table in the `## Response too large: resource_exhausted and exit 10` section (`| \`too_large\` | \`resource_exhausted\` ... |`), which Task 1's own `<action>` explicitly specifies with that exact first-cell shape. Task 3's criterion was authored without accounting for that earlier table, since both tables legitimately use the same `| \`too_large\` |` row-opening shape for different purposes (the eleven-code hint vocabulary table vs. the hint/Connect-code/exit-code cross-reference table). The criterion's actual INTENT — exactly one `too_large` row in the eleven-code hint-code table itself — holds: `TestErrorsDocHintCodesMatchArgErrorConstants` (the real, mechanical gate) passes, proving the eleven-code table has exactly the eleven codes `argerror.go` declares, no more and no less. No code or doc content was changed to force the literal `rg` count to `1`, since doing so would mean removing or reshaping one of the two legitimately-different tables Task 1 and Task 3 each independently specified. Recorded here per deviation-documentation discipline rather than silently ignored.

---

**Total deviations:** 0 auto-fixed. **Impact:** None on functionality or on any mechanical gate; one acceptance-criterion literal count does not match due to two independently-specified tables sharing a row-opening shape, noted above for the record.

## RED Evidence (per task)

**Task 1** — with `exitTooLarge = 10` declared (for compilation) but no mapper case yet:

```
exitcode_baseline_test.go:633: row "list/response-too-large": exitCodeFromError(err) = 1, want 10 (err=resource_exhausted: field=response hint=too_large: the result is too large to return in one response; retry with a smaller limit or k, or omit full)
exitcode_baseline_test.go:633: row "search/response-too-large": exitCodeFromError(err) = 1, want 10 (err=resource_exhausted: field=response hint=too_large: the result is too large to return in one response; retry with a smaller limit or k, or omit full)
--- FAIL: TestExitCodeBaseline (0.62s)
    --- FAIL: TestExitCodeBaseline/list/response-too-large (0.00s)
    --- FAIL: TestExitCodeBaseline/search/response-too-large (0.00s)
```
`TestExitCodeForConnectErrTable`, `TestExitCodeTimeoutDistinctFromUnavailable`, and `TestExitCodeTooLargeDistinct` all failed too, as expected, before the mapper case landed.

**Task 2** — with only the test edits present (no `classifyOperatorErr` arm yet):

```
--- FAIL: TestClassifyOperatorErr (0.00s)
    --- FAIL: TestClassifyOperatorErr/store.ErrResponseTooLarge (0.00s)
    operror_test.go:179: sentinel ErrResponseTooLarge: classifyOperatorErr returned an unclassified error: ErrResponseTooLarge: qdrant response exceeded the client's receive limit
--- FAIL: TestClassifyOperatorErrCodesAreDistinct (0.00s)
```

**Task 3** — against the tree Tasks 1-2 left (before any doc edit):

```
hintcodedocs_test.go:250: ../../docs-site/src/content/docs/reference/errors.md is missing hint code(s) argerror.go declares: [too_large]
hintcodedocs_test.go:257: ../../docs-site/src/content/docs/reference/errors.md heading count word = "ten", want "eleven" (argerror.go declares 11 HintCode constants)
hintcodedocs_test.go:273: ../../docs-site/src/content/docs/reference/errors.md: stale count word "ten" found via pattern "(?i)\b([a-z]+)-code\b" -- every count word on the page must read "eleven"  [x3]
hintcodedocs_test.go:273: ../../docs-site/src/content/docs/reference/errors.md: stale count word "ten" found via pattern "(?i)\b([a-z]+) hint codes\b" -- every count word on the page must read "eleven"  [x2]
hintcodedocs_test.go:306: ../../docs-site/src/content/docs/guides/upgrade.md: stale hint-code anchor "/reference/errors/#the-ten-hint-codes", want word "eleven"
--- FAIL: TestErrorsDocHintCodesMatchArgErrorConstants (0.01s)
```
`TestParseHintCodeTable` (the parser's own pure self-test, independent of the live doc) passed immediately — as expected, since it exercises inline fixtures, not `errors.md`.

D-05 surfaces finding (per this task's own scope): `internal/surfaces` declares conditional-rule sentences only (`ConditionalRule.Hint` is a plain string referencing the vocabulary) — its single-declaration convention does not cover the hint-code vocabulary itself, only conditional rule text. `hintcodedocs_test.go` is what closes that gap, deriving the vocabulary from `argerror.go`'s source directly rather than from `internal/surfaces` or a second hand-typed list.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 02-04 (this phase's own red-evidence registration) can register RED patches against this plan's three genuine RED observations above — the missing mapper case (`TestExitCodeBaseline`), the missing operator arm (`TestClassifyOperatorErr`), and the missing/stale docs (`TestErrorsDocHintCodesMatchArgErrorConstants`).
- `cmd/engram` and `internal/server` stay green (`ENGRAM_REQUIRE_QDRANT=1 go test ./cmd/engram/... ./internal/server/... -count=1`); `TestRedEvidencePatchesAreLive` (Phase 1's own registrations) still passes unaffected; `go test ./internal/keylinks/ -count=1` passes.
- `task lint` and `task license:check` are clean repo-wide; `pnpm --dir docs-site install --frozen-lockfile && pnpm --dir docs-site build` succeeds (21 pages); `git diff --exit-code HEAD -- go.mod go.sum` exits 0 — zero new Go dependencies.
- ROADMAP Phase 2 success criterion 4 and `REQ-exhausted-cli-docs` are both satisfied — exit code 10 is documented end to end and the hint-code table is now mechanically bound to its source.
- The one recorded acceptance-criterion mismatch above (a literal `rg` count of `2` vs. the plan's stated `1`) is not a blocker: the mechanical gate it exists to approximate (`TestErrorsDocHintCodesMatchArgErrorConstants`) passes for real.
- No blockers or concerns for plan 02-04.

---
*Phase: 02-error-classification-resourceexhausted-mapping*
*Completed: 2026-09-19*

## Self-Check: PASSED

All 12 files (1 created, 11 modified) verified present on disk with expected content; all three task commits (`dfc982a9`, `704765ad`, `f63c5517`) verified present in `git log --oneline --all`. Every acceptance criterion for all three tasks re-run and confirmed passing except the one literal-count mismatch documented above under Deviations (whose underlying mechanical gate passes). `go vet ./cmd/engram/` and `go vet ./internal/server/...` both clean of new findings (the pre-existing `operator_view_test.go` duplicate-json-tag vet note is unrelated and untouched by this plan); `golangci-lint run ./cmd/engram/...` and `./internal/server/...` both report 0 issues; `gofmt -l` clean on every touched file. The full plan-level `<verification>` block passes: `ENGRAM_REQUIRE_QDRANT=1 go test ./cmd/engram/... ./internal/server/... -count=1` ok; `pnpm --dir docs-site install --frozen-lockfile && pnpm --dir docs-site build` succeeds; `task lint` and `task license:check` both clean; `git diff --exit-code HEAD -- go.mod go.sum` exits 0; `go test ./internal/keylinks/ -count=1` ok; `TestRedEvidencePatchesAreLive` still passes.
