---
phase: 02-interface-discoverability
plan: 05
subsystem: api
tags: [go, cobra, cli, golden-testing, catalog, codegen]

# Dependency graph
requires:
  - phase: 02-interface-discoverability
    plan: 04
    provides: "internal/surfaces/toolclass.go: Class/Operation, Operations(), ClassForTool/ClassForCommand, ValidateOperations() — the shared blast-radius table this plan's catalog column and both-directions gate read from."
  - phase: 02-interface-discoverability
    plan: 02
    provides: "The extracted registerTools seam and the three package-local surfaces_test.go conformance-gate files this plan's own goldens/tests build alongside, without disturbing."
provides:
  - "cmd/engram/catalog.go: catalogCommand.BlastRadius (catalogBlastRadius nested object — read_only/destructive/idempotent/open_world), derived in buildCatalog from surfaces.ClassForCommand, panicking on an unclassified command rather than emitting a false-safe zero value."
  - "internal/surfaces/toolclass.go: two new CLI-only rows (serve, migrate-set-owner) that plan 02-04's table never covered — found live by this plan's own both-directions gate."
  - "cmd/engram/golden_test.go: the -update-gated golden walker, TestHelpGolden (cmd/engram/testdata/help.golden) and TestCatalogGolden (cmd/engram/testdata/catalog.golden), both deterministic against the ldflags version, a contributor's local env, and cross-test cobra singleton state."
  - "Taskfile.yaml's surfaces:gen and the surfaces CI job now regenerate and diff-check the two new goldens alongside the existing anchored-region regeneration, in one command/one job, never as a side effect of default/test/lint."
  - "docs-site/contributing/architecture.md's 'Pinned interface surfaces' section and docs-site/guides/cli.md's 'Blast radius' subsection — durable, scoreable statements of D-14's 'unreviewed' interpretation and the blast_radius field's shape/meaning."
affects: [02-06]

# Actuals (#2632)
actuals:
  tokens: 14959
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "buildCatalog's derive-never-declare traversal extended (not duplicated) to also derive blast-radius classification from the same shared internal/surfaces table plan 02-04 built for the MCP lane — one taxonomy, both lanes, confirmed by TestCatalogBlastRadiusMatchesToolClasses' both-directions gate."
    - "Golden-generation determinism as an explicit, named concern distinct from golden CONTENT correctness: withGoldenDeterminism normalizes both the ldflags-injected version and the two env-derived flag defaults (reindex --target, migrate-set-owner --owner) for the duration of generation only, restoring both via t.Cleanup."
    - "cobra's lazy -h/--help registration (only added inside a command's own execute() path, never by cmd.Help() alone) is a genuine, previously-undocumented test-order hazard on a shared package-level command tree — closed by explicitly calling InitDefaultHelpFlag() before capturing --help text (matching real interactive behavior) and by stripping any leaked 'help' flag entry before building the catalog golden (matching real bare-invocation behavior, which never executes a subcommand and so never sees it)."
    - "Atomic golden writes (os.CreateTemp in the target directory, then os.Rename) mirror internal/surfaces.WriteRegion's own contract, reimplemented locally in cmd/engram since WriteRegion itself is scoped to anchor-region rewriting of an EXISTING file, not whole-file golden generation."

key-files:
  created:
    - cmd/engram/golden_test.go
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden
    - .planning/phases/02-interface-discoverability/deferred-items.md
  modified:
    - cmd/engram/catalog.go
    - cmd/engram/catalog_test.go
    - internal/surfaces/toolclass.go
    - Taskfile.yaml
    - .github/workflows/ci.yaml
    - docs-site/src/content/docs/contributing/architecture.md
    - docs-site/src/content/docs/guides/cli.md

key-decisions:
  - "Checkpoint resolved per the orchestrator's pre-answered decision: blast_radius ships as a nested object (read_only/destructive/idempotent/open_world), losslessly mirroring the MCP lane's four hints in snake_case, never a collapsed string enum."
  - "buildCatalog PANICS (never emits a zero-valued classification) when a command has no surfaces.ClassForCommand entry — the 'fail loudly' half of the plan's own explicit either/or, chosen over an omitted-field design because a nested, always-present object is simpler for a consumer than one that sometimes carries a null blast_radius, and the both-directions test gate is meant to catch any gap long before this ever executes in production."
  - "Found and fixed live: internal/surfaces/toolclass.go's registry (built in plan 02-04) never classified two real, non-Hidden commands — serve and migrate-set-owner (migrate-set-owner is a DEPRECATED cobra alias, but cobra's Deprecated field only prints a warning; it does not hide a command from root.Commands() or buildCatalog's traversal). TestCatalogBlastRadiusMatchesToolClasses' both-directions gate failed immediately until both were classified: serve as {read_only:false, destructive:false, idempotent:false} (it ensures/creates Qdrant infrastructure and is the surface every mutating call runs through for its whole lifetime; running it twice on the same --listen-addr is a genuine additional effect, a bind conflict, not a no-op repeat); migrate-set-owner as {read_only:false, destructive:false, idempotent:true} (its own doc comment states it only ever fills an EMPTY owner field, the same additive reasoning as summarize-missing/backfill-short-ids, unlike migrate-remap-owner's general --from form which can overwrite a non-empty owner)."
  - "The 'engram' CLICommand row (02-04's sentinel for the bare self-describe invocation, added solely because ValidateOperations rejects both-empty rows) is explicitly excluded from TestCatalogBlastRadiusMatchesToolClasses' name-set comparison — it names the root command itself, which buildCatalog never emits as one of doc.Commands, so counting it would be a false mismatch, not a real one."
  - "Discovered live, NOT anticipated by 02-RESEARCH.md: cobra only lazily registers a command's own -h/--help flag from inside that command's execute() path (InitDefaultHelpFlag, called right before ParseFlags) — cmd.Help() called directly bypasses this. Since rootCmd and every subcommand are package-level singletons shared across the WHOLE cmd/engram test binary, whether '-h, --help' appeared in a captured golden depended on whether some EARLIER, unrelated test had already Execute()'d that specific command — reproduced via go test ./cmd/engram/... -count=1 (full suite, fails) vs -run 'TestHelpGolden|TestCatalogGolden' (isolated, passes). Fixed asymmetrically per each golden's true target: TestHelpGolden explicitly calls InitDefaultHelpFlag() before capturing (matching what a real `engram <cmd> --help` invocation always shows — confirmed against a freshly built binary); TestCatalogGolden strips any 'help' flag entry after calling buildCatalog (matching what a real bare `engram` invocation actually emits, which never executes a subcommand and so never sees it — also confirmed against a freshly built binary)."
  - "os.Getenv sweep of cmd/engram/*.go (required by the plan): three hits total. reindex.go's --target and migrate.go's --owner feed a flag DEFAULT directly from the environment at init() time (the real hazard — reproduced live: ENGRAM_REINDEX_TARGET=my_target changes --help's rendered default) and are normalized for golden generation via withGoldenDeterminism. client_common.go's resolveToken reads ENGRAM_TOKEN at RUNTIME to resolve a credential and never touches a flag registration, Usage string, or any golden-visible surface — not a hazard."
  - "A pre-existing, unrelated fragility was found (not fixed, out of scope): exitcode_baseline_test.go's reindex/missing-target and migrate-set-owner/missing-owner rows silently pass through to a dial attempt instead of a usage error if a contributor's shell happens to carry ENGRAM_REINDEX_TARGET/ENGRAM_MIGRATE_OWNER. Filed as GitHub issue #476 and logged in this phase's deferred-items.md; verified out of scope by confirming go test ./cmd/engram/... -count=1 -shuffle=on passes cleanly without the env vars set."

patterns-established:
  - "Golden-file generation for a cobra-tree-derived artifact must explicitly normalize THREE independent non-determinism sources, not just the build-time ldflags version: (1) any flag whose pflag default reads os.Getenv directly, (2) the process's shared, singleton command tree's own lazily-mutated state (cobra's InitDefaultHelpFlag) if the SAME package's test binary runs unrelated tests that Execute() real commands. A determinism assertion (generate twice in-process, require equal bytes) catches only the FIRST class of bug (unsorted iteration); it does NOT catch cross-test shared-singleton-state flakiness, which only surfaces running the full package suite, not the golden test in isolation — verify goldens with `go test ./pkg/... -count=1` (full package), not only `-run TestGolden`, before trusting either golden's content."

requirements-completed: [REQ-mcp-tool-annotations, REQ-help-output-pinned]

coverage:
  - id: D1
    description: "engram catalog carries a per-command blast-radius classification derived from the same internal/surfaces table that produces the MCP tool annotations."
    requirement: "REQ-mcp-tool-annotations"
    verification:
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestCatalogBlastRadiusMatchesToolClasses"
        status: pass
    human_judgment: false
  - id: D2
    description: "Every command buildCatalog emits carries a classification; the both-directions gate fails when a catalog command has no table entry and when a table entry names a command the catalog does not emit — proven fail-first in both directions this session."
    requirement: "REQ-mcp-tool-annotations"
    verification:
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestCatalogBlastRadiusMatchesToolClasses"
        status: pass
      - kind: manual_procedural
        ref: "Session transcript: temporarily blanked search's CLICommand row (buildCatalog panics, test FAILs); temporarily added a no_such_command_probe row (map-equality mismatch, test FAILs). Both reverted, both returned to PASS — see 'Fail-first proofs' below."
        status: pass
    human_judgment: true
    rationale: "The fail-first demonstration is an observed, one-time session event (mutate, observe RED, revert, observe GREEN), not a repeatable automated assertion beyond what TestCatalogBlastRadiusMatchesToolClasses already pins — a human should confirm the transcript below matches the claim."
  - id: D3
    description: "Every command's --help output is pinned by a golden file generated by walking the live cobra tree, so a new command appears in the golden with no one remembering to add it."
    requirement: "REQ-help-output-pinned"
    verification:
      - kind: unit
        ref: "cmd/engram/golden_test.go#TestHelpGolden"
        status: pass
    human_judgment: false
  - id: D4
    description: "The bare engram catalog JSON is pinned as a second golden under the same regeneration target, catching drift --help alone would miss."
    requirement: "REQ-help-output-pinned"
    verification:
      - kind: unit
        ref: "cmd/engram/golden_test.go#TestCatalogGolden"
        status: pass
    human_judgment: false
  - id: D5
    description: "Both goldens are deterministic: a fixed test-only version string is used instead of the ldflags-injected build variable, and the two env-derived flag defaults (reindex --target, migrate-set-owner --owner) are normalized — proven by regenerating under an injected env-var hazard and re-checking against the committed golden."
    requirement: "REQ-help-output-pinned"
    verification:
      - kind: integration
        ref: "ENGRAM_REINDEX_TARGET=probe ENGRAM_MIGRATE_OWNER=probe@example.com go test ./cmd/engram -run 'TestHelpGolden|TestCatalogGolden' -v -count=1 -> both PASS"
        status: pass
      - kind: other
        ref: "rg -n 'os\\.Getenv' cmd/engram/*.go — three hits, reviewed and recorded in key-decisions"
        status: pass
    human_judgment: false
  - id: D6
    description: "The golden walker visits commands and flags in stable lexicographic order; two consecutive regenerations produce byte-identical files — and are deterministic against the full package test suite's shared cobra-singleton state, not only in isolation."
    requirement: "REQ-help-output-pinned"
    verification:
      - kind: unit
        ref: "cmd/engram/golden_test.go#checkGolden's in-process double-generation determinism assertion (called from both TestHelpGolden and TestCatalogGolden)"
        status: pass
      - kind: integration
        ref: "go clean -testcache && go test ./cmd/engram/... -count=1 -shuffle=on (3 consecutive runs, all green)"
        status: pass
    human_judgment: false
  - id: D7
    description: "The help golden keys each command's block by its full command path, so two commands with byte-identical help text still occupy distinct, individually-diffable golden sections; a command with zero non-hidden flags still emits a golden section; help/completion/Hidden commands are excluded by the same predicate buildCatalog already uses."
    requirement: "REQ-help-output-pinned"
    verification:
      - kind: other
        ref: "rg -c '^## engram ' cmd/engram/testdata/help.golden -> 11 (matches the 11 non-hidden commands); rg -c 'engram help|engram completion' cmd/engram/testdata/help.golden -> 0"
        status: pass
    human_judgment: false
  - id: D8
    description: "REQ-help-output-pinned's word 'unreviewed' is recorded, in a durable repo doc, as: CI fails whenever the committed golden does not match the live tree, so any help-text change forces a regeneration commit whose diff shows the exact before/after wording in review."
    requirement: "REQ-help-output-pinned"
    verification:
      - kind: other
        ref: "docs-site/src/content/docs/contributing/architecture.md, 'Pinned interface surfaces' section, 'What \"unreviewed\" means here' paragraph"
        status: pass
    human_judgment: true
    rationale: "Documentation accuracy — a human should read the section and confirm it correctly states the mechanical CI-fails-on-drift interpretation and the considered-and-rejected CODEOWNERS alternative."
  - id: D9
    description: "task surfaces:gen regenerates the rule regions, the --help golden, and the catalog JSON golden in one command, and one CI job re-runs it and fails on a dirty tree — never as a side effect of task default/test/lint, and the CI job holds no write access."
    requirement: "REQ-help-output-pinned"
    verification:
      - kind: other
        ref: "rg -n 'surfaces:gen' Taskfile.yaml — one target, running the generator + proto:gen + the golden -update test; no match inside default/test/test:go/lint bodies. rg -n 'cmd/engram/testdata' .github/workflows/ci.yaml — matches inside the surfaces job's diff path list, no write permission added."
        status: pass
    human_judgment: false
  - id: D10
    description: "task surfaces:gen writes each generated golden atomically (temp file in the same directory, then rename), so an interrupted or concurrently-run regeneration leaves either the previous or the new content, never a truncated golden."
    requirement: "REQ-help-output-pinned"
    verification: []
    human_judgment: true
    rationale: "Code-level guarantee (cmd/engram/golden_test.go#writeGoldenFileAtomic, mirroring internal/surfaces.WriteRegion's own create-temp-then-rename contract) with no dedicated interruption-simulation test in this plan — a human should read the implementation and confirm the same atomicity discipline internal/surfaces already relies on."

duration: ~40min
completed: 2026-08-05
status: complete
---

# Phase 2 Plan 5: CLI Blast Radius + Pinned --help/Catalog Goldens Summary

**`engram catalog` now carries a per-command `blast_radius` classification derived from the same shared table the MCP tool annotations publish, and every command's `--help` output plus the bare catalog JSON are pinned behind deterministic goldens — closing two determinism hazards (env-derived flag defaults, and cobra's lazy `-h/--help` registration) neither the plan nor its RESEARCH pass anticipated.**

## Performance

- **Duration:** ~40 min
- **Tasks:** 3
- **Files modified:** 11 (5 created, 6 modified — plus this SUMMARY, STATE.md, ROADMAP.md, REQUIREMENTS.md)

## Accomplishments

- **Catalog blast radius, both-directions gated** (`8dd5c637`). `catalogCommand.BlastRadius` is a
  `catalogBlastRadius` nested object (the checkpoint's option-a shape), derived inside `buildCatalog`'s
  existing traversal from `surfaces.ClassForCommand(cmd.Name())` — never a second literal.
  `buildCatalog` panics rather than emitting a zero-valued classification for an unclassified command.
  `TestCatalogBlastRadiusMatchesToolClasses` copies `TestCatalogExitCodesMatchMapper`'s both-directions
  shape and, in doing so, immediately found a real gap: `internal/surfaces/toolclass.go` (built in plan
  02-04) never classified `serve` or `migrate-set-owner` — two live, non-Hidden commands cobra's
  `Deprecated` field does not hide from `root.Commands()`. Both were classified with documented
  reasoning and the gate went green.
- **`--help` and catalog-JSON goldens, walked from the live tree** (`601aed7e`). `cmd/engram/golden_test.go`
  adds the standard `-update`-gated golden idiom, reusing `buildCatalog`'s own skip predicate so the two
  goldens can only ever cover the identical command set. Two genuine, previously-unrecorded determinism
  hazards were found and closed: (1) `reindex --target`/`migrate-set-owner --owner`'s pflag defaults are
  computed directly from `os.Getenv` at package `init()` time, which `t.Setenv` cannot retroactively
  undo — normalized via `withGoldenDeterminism`; (2) cobra only lazily registers a command's own
  `-h, --help` flag from inside that command's `execute()` path, which `cmd.Help()` never triggers on
  its own, so whether it appeared depended on which OTHER tests in the shared `rootCmd` singleton had
  already run first — fixed asymmetrically (forced present for the help golden, matching real
  `--help` behavior; stripped for the catalog golden, matching real bare-invocation behavior). Both
  fixes verified against a freshly built binary, not just test-suite assumptions.
- **The durable doc record** (`8290be89`). `architecture.md` gains a "Pinned interface surfaces" section
  naming all six bound surfaces, the one `task surfaces:gen` regeneration target, the `surfaces` CI job,
  D-14's mechanical "unreviewed" interpretation, and the considered-and-rejected CODEOWNERS alternative.
  `cli.md` gains a "Blast radius" subsection documenting the new field's shape and conservative,
  non-authorization stance.

## Fail-first proofs (Task 1, observed this session)

**(a) Missing CLI row** — temporarily blanked `search_memory`'s `CLICommand` to `""`:
```
--- FAIL: TestCatalogBlastRadiusMatchesToolClasses (0.00s)
panic: catalog: command "search" has no internal/surfaces blast-radius classification — add a row to
internal/surfaces/toolclass.go's operations table [recovered, repanicked]
```
Reverted; `go test` returned to `PASS`.

**(b) Extra row naming a nonexistent command** — temporarily added `{MCPTool: "", CLICommand: "no_such_command_probe", ...}`:
```
--- FAIL: TestCatalogBlastRadiusMatchesToolClasses (0.00s)
    catalog_test.go:398: catalog command names = [... 11 real names ...],
      surfaces.Operations() non-empty CLICommand names = [... 11 real names ... no_such_command_probe]
```
Reverted; `go test` returned to `PASS`.

## Fail-first proof (Task 2, observed this session)

Temporarily appended `" PROBE"` to `client_search.go`'s `--query` Usage string:
```
--- FAIL: TestHelpGolden (0.00s)
    golden_test.go:285: testdata/help.golden has drifted from the live tree — run `task surfaces:gen` ...
--- FAIL: TestCatalogGolden (0.00s)
    golden_test.go:295: testdata/catalog.golden has drifted from the live tree — run `task surfaces:gen` ...
```
Reverted; `git status --short` confirmed a byte-identical tree and both tests returned to `PASS`.

## `os.Getenv` sweep (Task 2, required by the plan)

`rg -n 'os\.Getenv' cmd/engram/*.go` — three hits:

| Site | Feeds | Golden-visible? |
|---|---|---|
| `reindex.go:111` (`--target` default) | pflag flag DEFAULT at `init()` | Yes — normalized via `withGoldenDeterminism` |
| `migrate.go:147` (`--owner` default) | pflag flag DEFAULT at `init()` | Yes — normalized via `withGoldenDeterminism` |
| `client_common.go:68` (`resolveToken`) | a credential resolved at RUNTIME | No — never touches a flag registration, Usage string, or catalog field |

Both golden-visible hazards were reproduced live (`ENGRAM_REINDEX_TARGET=my_target go run ./cmd/engram
reindex --help` changes the rendered default without the fix) and re-verified green under the same
injected env vars after the fix.

## Task Commits

1. **Task 1: Derive per-command blast radius on the catalog from the shared table** — `8dd5c637` (feat)
2. **Task 2: Pin --help and the catalog JSON behind goldens generated from the live tree** — `601aed7e` (test)
3. **Task 3: Record the surfaces:gen contract and D-14's "unreviewed" interpretation in a durable doc** — `8290be89` (docs)

**Plan metadata:** commit pending (this SUMMARY + STATE.md/ROADMAP.md/REQUIREMENTS.md update)

## Files Created/Modified

- `cmd/engram/catalog.go` — `catalogCommand.BlastRadius`, `catalogBlastRadius`, `buildCatalog`'s
  classification lookup + panic-on-miss
- `cmd/engram/catalog_test.go` — `TestCatalogBlastRadiusMatchesToolClasses`, `decodedCatalog.BlastRadius`
- `internal/surfaces/toolclass.go` — two new rows: `serve`, `migrate-set-owner`
- `cmd/engram/golden_test.go` — the golden walker, `withGoldenDeterminism`, `TestHelpGolden`,
  `TestCatalogGolden`, `writeGoldenFileAtomic`
- `cmd/engram/testdata/help.golden`, `cmd/engram/testdata/catalog.golden` — the two pinned goldens
- `Taskfile.yaml` — `surfaces:gen` extended with the golden `-update` invocation
- `.github/workflows/ci.yaml` — the `surfaces` job's drift check extended to `cmd/engram/testdata/`
- `docs-site/src/content/docs/contributing/architecture.md` — "Pinned interface surfaces" section
- `docs-site/src/content/docs/guides/cli.md` — "Blast radius" subsection
- `.planning/phases/02-interface-discoverability/deferred-items.md` — the out-of-scope
  `exitcode_baseline_test.go` fragility (GitHub issue #476)

## Decisions Made

See `key-decisions` in frontmatter for the full list. Highlights: `blast_radius` ships per the
pre-resolved checkpoint (option-a, nested object); `buildCatalog` panics rather than omitting the field
on a classification miss; `serve`/`migrate-set-owner` were found live-unclassified and fixed; cobra's
lazy `-h/--help` registration was found to be a genuine, previously-undocumented cross-test determinism
hazard on the shared `rootCmd` singleton, closed asymmetrically per each golden's true target semantics.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical] `serve` and `migrate-set-owner` had no blast-radius classification**
- **Found during:** Task 1, first run of `TestCatalogBlastRadiusMatchesToolClasses`
- **Issue:** `internal/surfaces/toolclass.go` (plan 02-04) classified 15 MCP tools + 7 CLI-only
  operations, but two live, non-Hidden `root.Commands()` entries — `serve` and `migrate-set-owner` (a
  cobra `Deprecated` alias, which cobra does not hide from the command list) — had no row at all. The
  plan's own acceptance criteria (no catalog command may lack a classification) could not pass without
  this fix.
- **Fix:** Added two rows with documented conservative-stance reasoning (see key-decisions).
- **Files modified:** `internal/surfaces/toolclass.go`
- **Verification:** `TestCatalogBlastRadiusMatchesToolClasses` — `--- PASS`
- **Committed in:** `8dd5c637` (Task 1 commit)

**2. [Rule 1 - Bug] cobra's lazily-registered `-h, --help` flag flapped both goldens by test run/subset**
- **Found during:** Task 2, `go clean -testcache && task` (full package suite) failing immediately after
  goldens generated cleanly in isolation (`-run 'TestHelpGolden|TestCatalogGolden'`)
- **Issue:** cobra only adds a command's own `-h, --help` flag inside that command's `execute()` path
  (`InitDefaultHelpFlag`), never via `cmd.Help()` alone. `rootCmd` and every subcommand are package-level
  singletons shared by the WHOLE `cmd/engram` test binary, so whether a given command's `-h, --help`
  flag had already been lazily registered depended entirely on whether some UNRELATED, earlier test had
  executed that specific command first — a real cross-test, run-order-dependent hazard, not anticipated
  by 02-RESEARCH.md.
- **Fix:** `buildHelpGoldenContent` explicitly calls `cmd.InitDefaultHelpFlag()` before capturing (making
  `-h, --help` always present, matching a real `engram <cmd> --help` invocation, confirmed against a
  freshly built binary); `buildCatalogGoldenContent` strips any `"help"`-named flag entry after calling
  `buildCatalog` (making it always absent, matching a real bare `engram` invocation, also confirmed
  against a freshly built binary, since a bare invocation never executes any subcommand).
- **Files modified:** `cmd/engram/golden_test.go`
- **Verification:** `go clean -testcache && go test ./cmd/engram/... -count=1` and three consecutive
  `-shuffle=on` runs — all green
- **Committed in:** `601aed7e` (Task 2 commit)

**3. [Rule 2 - Missing critical] env-derived flag defaults would have leaked a contributor's local
environment into a committed golden**
- **Found during:** Task 2, the plan's required `os.Getenv` sweep
- **Issue:** `reindex --target` and `migrate-set-owner --owner` set their pflag default directly from
  `os.Getenv(...)` at package `init()` time — before any test's `t.Setenv` can take effect. Reproduced
  live: `ENGRAM_REINDEX_TARGET=my_target go run ./cmd/engram reindex --help` visibly changes the
  rendered default.
- **Fix:** `withGoldenDeterminism` blanks each named flag's `pflag.Flag.DefValue` for the duration of
  golden generation only, restoring it via `t.Cleanup`.
- **Files modified:** `cmd/engram/golden_test.go`
- **Verification:** `ENGRAM_REINDEX_TARGET=probe ENGRAM_MIGRATE_OWNER=probe@example.com go test
  ./cmd/engram -run 'TestHelpGolden|TestCatalogGolden' -v -count=1` — both `--- PASS`
- **Committed in:** `601aed7e` (Task 2 commit)

---

**Total deviations:** 3 auto-fixed (1 missing-critical classification gap in a prior plan's registry,
1 bug in this plan's own golden-generation determinism discovered via full-suite testing, 1
missing-critical determinism normalization required by the plan's own acceptance criteria)
**Impact on plan:** All three were necessary corrections discovered while executing the plan's own
instructions faithfully — no scope creep, no architectural change. Neither of the two golden-generation
hazards (env-derived defaults, lazy `-h/--help`) was flagged by 02-RESEARCH.md; both were found live by
actually stress-testing determinism (env-var injection, full-suite + shuffled runs) rather than trusting
an isolated `-run` pass.

## Issues Encountered

A pre-existing, unrelated test fragility was found (not fixed — out of scope): `exitcode_baseline_test.go`'s
`reindex/missing-target` and `migrate-set-owner/missing-owner` rows silently pass through to a dial
attempt instead of the expected usage error if `ENGRAM_REINDEX_TARGET`/`ENGRAM_MIGRATE_OWNER` happens to
be set in the shell running `go test`. Filed as [GitHub issue #476](https://github.com/seanb4t/engram/issues/476)
and logged in `.planning/phases/02-interface-discoverability/deferred-items.md`. Confirmed out of scope:
`go clean -testcache && go test ./cmd/engram/... -count=1 -shuffle=on` passes cleanly without the env
vars injected.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `engram catalog`'s `blast_radius` field and both `cmd/engram/testdata/*.golden` fixtures are now the
  CLI's stable, machine-readable teaching surface — any future command or flag change is caught by the
  `surfaces` CI job, never silently.
- `internal/surfaces/toolclass.go`'s registry is now complete over every live `root.Commands()` entry
  (15 MCP tools + 9 CLI-only operations, including the two found this session) — a future new command
  should add its row here in the SAME commit that registers it with cobra, or the both-directions gate
  will fail immediately.
- The golden-determinism pattern this plan established (normalize env-derived flag defaults; force or
  strip cobra's lazily-registered `-h/--help` per the golden's true target semantics; verify via full
  package suite + `-shuffle=on`, not only an isolated `-run`) is the sanctioned template for any future
  cobra-tree-derived golden in this repo.
- `.planning/ROADMAP.md`/`REQUIREMENTS.md` still do not reflect D-05/D-10/D-11's scope expansions (per
  02-CONTEXT.md's standing note and rule `8dfdhfs5nn`) — that roadmap edit remains the user's outstanding
  action via `/gsd-phase`, not performed by this plan.

## Self-Check: PASSED

- FOUND: `cmd/engram/golden_test.go`
- FOUND: `cmd/engram/testdata/help.golden`
- FOUND: `cmd/engram/testdata/catalog.golden`
- FOUND: `internal/surfaces/toolclass.go` (serve, migrate-set-owner rows)
- FOUND commit `8dd5c637` in `git log --oneline --all`
- FOUND commit `601aed7e` in `git log --oneline --all`
- FOUND commit `8290be89` in `git log --oneline --all`

---
*Phase: 02-interface-discoverability*
*Completed: 2026-08-05*
