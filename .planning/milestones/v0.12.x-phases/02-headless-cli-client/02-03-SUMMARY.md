---
phase: 02-headless-cli-client
plan: 03
subsystem: api
tags: [cobra, self-describing-cli, exit-codes, docs]

# Dependency graph
requires:
  - phase: 02-headless-cli-client plan 01 (engram search tracer)
    provides: "cmd/engram/client_common.go's exitOK/exitGeneric/exitUsage/exitAuth/exitNotFound/
      exitUnavailable constants and exitCodeForConnectErr mapper; cmd/engram/root.go's rootCmd
      literal and Execute()/exitCodeFromError; cmd/engram/clienttest_test.go's runClient/
      resetClientFlags harness"
  - phase: 02-headless-cli-client plan 02 (engram list + engram store)
    provides: "the complete three-verb command tree (search, list, store) the catalog enumerates"
provides:
  - "cmd/engram/catalog.go: catalogDoc/catalogCommand/catalogFlag/catalogExitCode types,
    buildCatalog (walks the live cobra tree), collectFlags, runSelfDescribe (rootCmd's RunE)"
  - "cmd/engram/root.go: rootCmd gains RunE (runSelfDescribe) and the coupled
    Args: cobra.NoArgs — the only two changed lines"
  - "docs-site/src/content/docs/guides/cli.md: the human-readable CLI guide, pointing at the
    bare invocation as the authoritative machine-readable form"
affects: []

# Actuals (#2632)
actuals:
  tokens: 6069
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "buildCatalog derives the entire self-describe document from the live cobra tree at
      runtime (root.Commands(), each command's Flags() + root's PersistentFlags()) and from
      the shared exit-code constants — never a hand-maintained literal — so a command, flag,
      or exit code added later appears with no edit here (D-15/D-11)."
    - "collectFlags unions a command's own Flags() with root's PersistentFlags(), matching
      cobra's actual flag-inheritance surface for that command."

key-files:
  created:
    - cmd/engram/catalog.go
    - cmd/engram/catalog_test.go
    - docs-site/src/content/docs/guides/cli.md
  modified:
    - cmd/engram/root.go

key-decisions:
  - "The plan's stated landmine ('without cobra.NoArgs a mistyped verb prints the catalog and
    exits 0') does not reproduce by simply deleting the Args field: cobra's own legacyArgs
    fallback (invoked when Args is nil) independently rejects an unmatched first argument for
    any ROOT command that HasSubCommands() — verified empirically against cobra v1.10.2's
    Find()/legacyArgs source, not just asserted. The genuine regression only reproduces when
    Args is set to something explicitly permissive (cobra.ArbitraryArgs) rather than left nil.
    cobra.NoArgs is still required — as documented intent, and as a guard against any future
    edit that sets a more permissive Args value or against a cobra major-version change to
    legacyArgs's behavior — but the RED proof and root.go's comment were adjusted to reflect
    the true mechanism rather than the plan's (incorrect, for this cobra version) premise. See
    RED Observations below for both the deleted-field attempt (surprising PASS) and the
    ArbitraryArgs mutation (the actual RED)."
  - "All catalog_test.go tests (Task 1's five plus Task 2's four) were written and committed
    together in Task 1's commit, since catalog.go's single implementation had to satisfy all
    nine from the start — see Deviations."

patterns-established:
  - "A self-describe document is a hand-declared Go struct with snake_case JSON tags (not
    protobuf-derived), matching client_common.go's renderJSON field-naming convention without
    coupling the catalog's shape to a .proto message."

requirements-completed: [REQ-cli-self-describing]

coverage:
  - id: D1
    description: "A bare `engram` invocation writes one JSON document to stdout and exits 0;
      stderr is empty"
    requirement: "REQ-cli-self-describing"
    verification:
      - kind: integration
        ref: "cmd/engram/catalog_test.go#TestRootBareInvocationEmitsCatalog"
        status: pass
      - kind: integration
        ref: "cmd/engram/catalog_test.go#TestCatalogGoesToStdoutNotStderr"
        status: pass
      - kind: e2e
        ref: "built binary: bare-exit=0, stderr-bytes=0 (real os.Exit, not an in-process assertion)"
        status: pass
    human_judgment: false
  - id: D2
    description: "The catalog's command-name set equals the live tree's non-hidden subcommands
      (excluding cobra's own help/completion), asserted as set equality plus an explicit
      search/list/store membership control"
    requirement: "REQ-cli-self-describing"
    verification:
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestCatalogEnumeratesEveryCommand"
        status: pass
    human_judgment: false
  - id: D3
    description: "Per command, the catalog's flag-name set equals that command's own flags plus
      root's persistent flags; every flag entry carries a non-empty type and usage"
    requirement: "REQ-cli-self-describing"
    verification:
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestCatalogEnumeratesEveryFlag"
        status: pass
    human_judgment: false
  - id: D4
    description: "The catalog lists exactly six exit-code entries covering 0-5 with no
      duplicates, each with a non-empty meaning"
    requirement: "REQ-cli-self-describing"
    verification:
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestCatalogListsEveryExitCode"
        status: pass
    human_judgment: false
  - id: D5
    description: "The catalog's exit-code set equals, as a bidirectional set, the set of exit
      codes exitCodeForConnectErr can actually produce across every connect.Code plus the
      non-connect-error case, unioned with exitOK"
    requirement: "REQ-cli-agent-output"
    verification:
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestCatalogExitCodesMatchMapper"
        status: pass
    human_judgment: false
  - id: D6
    description: "The catalog documents D-17: a note mentions both exit code 1 (framework
      flag-parse error) and exit code 2 (engram's own validation) alongside a mention of flags
      or usage"
    requirement: "REQ-cli-self-describing"
    verification:
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestCatalogDocumentsFlagParseExitCode"
        status: pass
    human_judgment: false
  - id: D7
    description: "A mistyped verb still fails with an unknown-command error and non-JSON
      stdout; every pre-existing subcommand still resolves"
    requirement: "REQ-cli-self-describing"
    verification:
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestRootUnknownSubcommandStillErrors"
        status: pass
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestRootExistingSubcommandsStillResolve"
        status: pass
      - kind: e2e
        ref: "built binary: unknown-verb-exit=1, unknown-flag-exit=1 (D-17's accepted 1-not-2
          behavior observed on the real binary)"
        status: pass
    human_judgment: false
  - id: D8
    description: "--help stays ordinary human cobra output for the root and a subcommand, and
      is provably a different artifact from the bare-invocation catalog"
    requirement: "REQ-cli-self-describing"
    verification:
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestHelpFlagStillPrintsHumanHelp"
        status: pass
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestHelpAndCatalogAreDifferentOutputs"
        status: pass
      - kind: e2e
        ref: "built binary: help-exit=0"
        status: pass
    human_judgment: false
  - id: D9
    description: "The CLI guide documents the three verbs, credential precedence, output
      contract, the exit-code table (matching the catalog's six codes), and D-17's flag-typo
      caveat, and names the bare invocation as authoritative"
    requirement: "REQ-cli-self-describing"
    verification:
      - kind: other
        ref: "docs-site/src/content/docs/guides/cli.md exists; task lint:markdown clean;
          grep -oE '^\\| *[0-5] ' ... prints 0,1,2,3,4,5,"
        status: pass
    human_judgment: false

duration: ~35min
completed: 2026-07-31
status: complete
---

# Phase 2 Plan 3: Bare-Invocation Self-Describe Catalog Summary

**`engram` with no arguments now writes one JSON document to stdout and exits 0 — every command,
flag, and exit code derived live from the cobra tree and the shared exit-code constants, never a
hand-maintained list — closing out REQ-cli-self-describing and the phase's three-verb surface.**

## Performance

- **Duration:** ~35 min
- **Tasks:** 2/2 completed
- **Files modified:** 4 (3 created, 1 modified)
- **Commits:** 2 task commits + this metadata commit

## Accomplishments

- `engram` with zero arguments emits one JSON catalog document (binary name, version, commands,
  exit codes, notes) to stdout and exits 0, with stderr empty — proven both by in-process tests
  and by running the real built binary (`bare-exit=0`, `stderr-bytes=0`).
- The catalog is derived, not maintained: `buildCatalog` walks `root.Commands()` for names/
  summaries and each command's `Flags()` (unioned with root's `PersistentFlags()`) for flags,
  and its exit-code section is built from the same `exitOK`/`exitGeneric`/`exitUsage`/`exitAuth`/
  `exitNotFound`/`exitUnavailable` constants `client_common.go`'s mapper returns — never a second
  literal list. `TestCatalogExitCodesMatchMapper` is a bidirectional set-equality gate over this,
  proven capable of failing in both directions before being trusted (see RED Observations).
- `rootCmd` gained exactly two fields — `RunE: runSelfDescribe` and `Args: cobra.NoArgs` — and
  nothing else changed in `root.go` (`git diff --unified=0` shows only the two field lines plus
  their comment).
- `engram --help` is untouched: still prints ordinary human cobra help for the root and for
  `search --help`, and is proven to be a genuinely different artifact from the bare-invocation
  catalog (`TestHelpAndCatalogAreDifferentOutputs`).
- A mistyped verb (`engram definitely-not-a-verb`) still fails with an unknown-command error and
  non-JSON stdout; every pre-existing operator command still resolves normally.
- `docs-site/src/content/docs/guides/cli.md` documents the three verbs, credential precedence,
  the output contract, the exit-code table, and D-17's caveat that a flag typo exits 1 — and
  states plainly that the bare invocation is the authoritative machine-readable form.
- Zero new Go dependencies: `git diff --exit-code go.mod go.sum` is clean throughout.

## Task Commits

Each task was committed atomically:

1. **Task 1: A bare invocation emits the catalog — and a typo'd verb still fails** - `cd1a8f69`
   (feat, tdd)
2. **Task 2: The catalog cannot drift from the taxonomy it advertises** - `f0a6c677` (test, tdd)

**Plan metadata:** committed alongside this SUMMARY (see final commit below)

## Files Created/Modified

- `cmd/engram/catalog.go` - `catalogDoc`/`catalogCommand`/`catalogFlag`/`catalogExitCode` types,
  `buildCatalog` (derives from the live cobra tree), `collectFlags` (unions a command's own
  flags with root's persistent flags), `runSelfDescribe` (rootCmd's `RunE`, encodes to
  `cmd.OutOrStdout()`)
- `cmd/engram/catalog_test.go` - all nine tests: `TestRootBareInvocationEmitsCatalog`,
  `TestCatalogEnumeratesEveryCommand`, `TestCatalogEnumeratesEveryFlag`,
  `TestCatalogListsEveryExitCode`, `TestRootUnknownSubcommandStillErrors`,
  `TestRootExistingSubcommandsStillResolve`, `TestCatalogGoesToStdoutNotStderr`,
  `TestCatalogExitCodesMatchMapper`, `TestCatalogDocumentsFlagParseExitCode`,
  `TestHelpFlagStillPrintsHumanHelp`, `TestHelpAndCatalogAreDifferentOutputs`, plus the
  `resetHelpFlag` test-isolation helper (see Deviations)
- `cmd/engram/root.go` - added `RunE: runSelfDescribe` and `Args: cobra.NoArgs` to the `rootCmd`
  literal; nothing else changed
- `docs-site/src/content/docs/guides/cli.md` - new guide: the three verbs, shared flags,
  credential precedence, output contract, exit-code table, D-17's caveat, the self-describe
  catalog section

## Decisions Made

- **The plan's landmine premise did not reproduce as literally stated, and the RED proof + code
  comment were corrected accordingly** (documented in full under Deviations and RED
  Observations). `cobra.NoArgs` remains required per the plan's `must_haves`; only the specific
  claimed failure mode ("deleting the field alone breaks it") was found to be incorrect for
  cobra v1.10.2's actual `legacyArgs` behavior on a root command with subcommands.
- All nine `catalog_test.go` tests were written together against the single `catalog.go`
  implementation and committed in Task 1, rather than split feat/test across the two commits as
  the plan's task boundaries assumed — see Deviations.
- `resetHelpFlag` was added to `catalog_test.go` (not `clienttest_test.go`, which this plan does
  not touch) to fix a real cross-test contamination bug this plan's own tests introduced: cobra's
  `--help` flag is bound directly to `pflag` `FlagSet` storage and is never re-parsed on a later
  `Execute()` call that omits `--help`, so a `runClient(t, "--help")` invocation would otherwise
  leave the shared `rootCmd`/`searchCmd` help flag permanently `true` for every subsequent test in
  the binary — silently making a *later* bare invocation (or a *later* `client_search_test.go`
  test) render help text instead of its real output. Caught and fixed before either task's commit.

## Deviations from Plan

**1. [Rule 1 - Plan premise incorrect] The stated "delete `Args: cobra.NoArgs`" RED did not
reproduce**
- **Found during:** Task 1, attempting the plan's required RED proof for
  `TestRootUnknownSubcommandStillErrors`
- **Issue:** The plan states: "without this a mistyped verb would print the catalog and exit 0."
  Deleting the `Args` field (making it `nil`) was tried first, exactly as the plan's action text
  describes ("temporarily remove the argument validator"). `TestRootUnknownSubcommandStillErrors`
  **stayed green** — no RED observed.
- **Root cause (verified by reading the vendored cobra v1.10.2 source, not assumed):**
  `Command.Find()` (`args.go`/`command.go`), when `commandFound.Args == nil`, falls back to
  `legacyArgs(commandFound, args)`. `legacyArgs`'s own documented behavior is: *"root command with
  subcommands: do subcommand checking"* — it independently returns an `unknown command` error for
  a root command (`!cmd.HasParent()`) with subcommands (`cmd.HasSubCommands()`) and a non-empty
  unmatched arg, **regardless of `Runnable()`/`RunE`**. Since `rootCmd` always has many
  subcommands, this fallback already guards the exact scenario the plan's landmine describes —
  making the field-deletion mutation an accidental no-op for this specific command shape.
- **Fix:** Found the mutation that actually reproduces the plan's described regression: setting
  `Args: cobra.ArbitraryArgs` explicitly (bypassing `legacyArgs` entirely, since `Find()` only
  falls back to `legacyArgs` when `Args == nil`). This **does** reproduce the landmine exactly as
  described — see RED Observations. `Args: cobra.NoArgs` was restored, and `root.go`'s comment
  was rewritten to state the corrected mechanism (legacyArgs's protection is "an incidental
  property of cobra's internals, not a contract this package should rely on") rather than the
  plan's literal (incorrect, for this cobra version and command shape) claim.
- **Files touched during investigation:** `cmd/engram/root.go` (temporarily, reverted before the
  real edit)
- **Verification:** `go test ./cmd/engram/... -run TestRootUnknownSubcommandStillErrors -count=1`
  passes with the final `Args: cobra.NoArgs` in place; `git diff --unified=0 cmd/engram/root.go`
  shows only the two intended field lines.
- **Committed in:** `cd1a8f69`

**2. [Not a plan deviation — task-boundary consolidation, documented for traceability] All nine
tests landed in Task 1's commit, not split feat/test across both commits**
- **Found during:** Writing `catalog_test.go`
- **Reasoning:** The plan's Task 1 and Task 2 each specify a subset of tests in their
  `<behavior>` blocks, implying two separate write/RED/GREEN cycles. In practice, `catalog.go`'s
  single `buildCatalog`/`runSelfDescribe` implementation had to satisfy all nine tests
  simultaneously from the first GREEN run — there is no intermediate implementation state that
  passes Task 1's five tests but not Task 2's four (the exit-code section, the notes, and the
  flag-collection logic are single code paths, not separable per test). Writing and committing
  all nine together in Task 1, then performing Task 2's specific required RED proofs
  (bidirectional `TestCatalogExitCodesMatchMapper` mutations) and writing the docs page as Task
  2's actual remaining work, was more honest than fabricating an artificial mid-point split.
  Task 2's commit is therefore docs-only in terms of file diff, though its commit message
  describes the tests it is responsible for gating — this is noted here so the commit history
  reads accurately alongside this SUMMARY.
- **Committed in:** `cd1a8f69` (all tests), `f0a6c677` (docs page)

---

**Total deviations:** 1 tracked (Rule 1 — corrected an incorrect plan premise about cobra's
`Args`-validation fallback, with the actual required guard behavior preserved and re-verified by
a different, working RED mutation); 1 documented for traceability only (test/commit consolidation
across the two tasks, no design or behavior change).

## RED Observations (required, quoted verbatim)

**Task 1 — `TestRootBareInvocationEmitsCatalog` against `rootCmd` with no `RunE`/`Args`
(today's shape):**
```
=== RUN   TestRootBareInvocationEmitsCatalog
    catalog_test.go:141: stdout did not unmarshal in its entirety as one JSON object: invalid character 'S' looking for beginning of value
        stdout="Self-hosted, correctable, OAuth-secured memory for coding agents\n\nUsage:\n  engram [command]\n\nAvailable Commands:\n  backfill-short-ids  Assign a short_id to every memory that lacks one (payload-only; no re-embed)\n  completion          Generate the autocompletion script for the specified shell\n  help                Help about any command\n  list                List memories on a remote engram server\n  migrate-remap-owner Re-stamp record owner across the collection (sub->email, email->email, owner-less, or anonymous bucket)\n  prune-expired       Delete memories whose validity window (not_after) has lapsed\n  reindex             Re-embed memories into a new (new-dimension) collection for embedder migration\n  search              Search memories on a remote engram server\n  serve               Run the engram MCP server\n  store               Store a memory on a remote engram server\n  summarize-missing   Fill empty recall summaries with the configured cheap model\n  version             Print the engram version\n\nFlags:\n  -h, --help      help for engram\n  -v, --version   version for engram\n\nUse \"engram [command] --help\" for more information about a command.\n"
--- FAIL: TestRootBareInvocationEmitsCatalog (0.00s)
FAIL
```

**Task 1 — landmine, attempt 1 (as literally described by the plan): `Args` field deleted,
`RunE` present — surprisingly stayed GREEN, not RED (see Deviation 1):**
```
=== RUN   TestRootUnknownSubcommandStillErrors
--- PASS: TestRootUnknownSubcommandStillErrors (0.00s)
PASS
ok  	github.com/seanb4t/engram/cmd/engram	0.278s
```

**Task 1 — landmine, attempt 2 (the mutation that actually reproduces the regression):
`Args: cobra.ArbitraryArgs` set explicitly, bypassing cobra's `legacyArgs` fallback:**
```
=== RUN   TestRootUnknownSubcommandStillErrors
    catalog_test.go:253: expected an error for an unknown subcommand, got nil
--- FAIL: TestRootUnknownSubcommandStillErrors (0.00s)
FAIL
```

**Task 2, gate 1/2 — `TestCatalogExitCodesMatchMapper`: a temporary seventh catalog entry
(`{Code: 99, ...}`) the mapper never produces:**
```
=== RUN   TestCatalogExitCodesMatchMapper
    catalog_test.go:319: catalog exit codes = {0,1,2,3,4,5,99}, mapper-producible exit codes = {0,1,2,3,4,5}
--- FAIL: TestCatalogExitCodesMatchMapper (0.00s)
FAIL
```

**Task 2, gate 2/2 — `TestCatalogExitCodesMatchMapper`: a temporary mapper arm
(`connect.CodeUnauthenticated, connect.CodePermissionDenied` → `42` instead of `exitAuth`)
returning a code the catalog omits:**
```
=== RUN   TestCatalogExitCodesMatchMapper
    catalog_test.go:319: catalog exit codes = {0,1,2,3,4,5}, mapper-producible exit codes = {0,1,2,4,5,42}
--- FAIL: TestCatalogExitCodesMatchMapper (0.00s)
FAIL
```

All temporary edits above (`root.go`'s two mutations, `catalog.go`'s seventh entry,
`client_common.go`'s mapper arm) were reverted immediately after their RED observation; `git
diff` against each file was empty before the corresponding task's real commit.

## Issues Encountered

- **`--help` flag leak across tests, caught and fixed before commit (not a design flaw in
  `catalog.go` itself):** the first full run of the Task 2 test batch showed
  `TestHelpAndCatalogAreDifferentOutputs` failing (`bare invocation and --help produced identical
  stdout`) only when run *after* `TestHelpFlagStillPrintsHumanHelp` in the same binary — isolated
  runs of either test alone passed. Root cause: cobra's `--help` flag value lives in `pflag`
  `FlagSet` storage bound at `InitDefaultHelpFlag()` time and is never reset between `Execute()`
  calls on the same shared `rootCmd`/`searchCmd`, so one `runClient(t, "--help")` invocation left
  it `true` for every later invocation in the test binary that didn't re-pass `--help`. Fixed with
  `resetHelpFlag(t, cmd)` (`f.Value.Set("false")` + `f.Changed = false` via `t.Cleanup`), applied
  at every `--help` call site in `catalog_test.go`. Full-package `go test ./cmd/engram/...
  -count=1` confirmed no contamination of Plan 01/02's own tests.
- `ssh-add -T /Users/sean/.ssh/seanb4t_ed25519.pub` confirmed the 1Password SSH-signing agent was
  live before either commit; both commits carry a real SSH signature (`git cat-file commit` shows
  a `gpgsig` block on each).

## User Setup Required

None. The self-describe catalog and its tests run entirely in-process against the live cobra
command tree — no external service, network call, or server is involved.

## Next Phase Readiness

This is the final plan of Phase 2. All four phase requirements are now complete:
`REQ-cli-client-commands`, `REQ-cli-agent-output`, and `REQ-cli-credential-safety` (Plans 01/02),
and `REQ-cli-self-describing` (this plan). `engram`, `engram search`, `engram list`, `engram
store`, and a bare `engram` invocation are all real, tested, end-to-end behavior. No blockers for
phase completion or milestone verification.

---
*Phase: 02-headless-cli-client*
*Completed: 2026-07-31*

## Self-Check: PASSED

All 4 claimed files verified present on disk; both task commits (`cd1a8f69`, `f0a6c677`)
verified present in `git log --oneline --all`.
