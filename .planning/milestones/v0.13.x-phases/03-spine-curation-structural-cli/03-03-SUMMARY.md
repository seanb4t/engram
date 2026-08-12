---
phase: 03-spine-curation-structural-cli
plan: 03
subsystem: cli
tags: [cobra, destructive-commands, blast-radius, safety-gate, qdrant]

# Dependency graph
requires:
  - phase: 03-spine-curation-structural-cli
    provides: "plan 03-01's cmdwalk.go (walkCommands/commandKey), internal/surfaces blast-radius table; plan 03-02's operatorCommands()/operator_output.go (addOperatorOutputFlag/operatorOutputFormat/renderOperator)"
provides:
  - "cmd/engram/destructive.go: registerDestructive — the structural choke point every destructive command's RunE is installed by (a leaf supplies preview/apply closures and never assigns RunE itself), addApplyFlag/applyRequested, and the cliNow clock seam"
  - "surfaces.RuleDestructiveRequiresApply — the registered conditional rule the --apply flag's Usage string composes, anchored in the CLI guide's new Destructive-commands section and curating-memory's SKILL.md"
  - "prune-expired flipped to preview-by-default: bare invocation previews via the new internal/store.CountExpired/expiredFilter, --apply deletes"
  - "migrate-remap-owner flipped to preview-by-default with --dry-run REMOVED (checkpoint option-a)"
  - "internal/e2e/spine_review_test.go — the first end-to-end coverage any operator command has had (previously serve/search/list only)"
affects: [03-04, 03-05, 03-06, 03-07]

# Actuals (#2632)
actuals:
  tokens: 18833
  tasks: 3
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "registerDestructive as the RunE choke point: destructive-tier membership is derived from surfaces.Operations() at registration time (panics if the routed command isn't classified Destructive), and the installed RunE closure is verified at RUNTIME via runtime.FuncForPC substring match, not merely proven present by flag existence"
    - "applyRequested reads the flag's VALUE (never pflag.Flag.Changed) so --apply=false behaves identically to an omitted flag"
    - "cliNow package var as the injectable clock seam for a preview cutoff, truncated to the second at the call site so it matches the not_after payload's second-granularity comparison and stays byte-identical across a sub-second clock drift"
    - "expiredFilter/CountExpired in internal/store/spine.go: the ONE not_after range constructor PruneExpired's applied path and CountExpired's preview path both call, so the preview count and the applied count cannot silently drift onto two independently-maintained conditions"
    - "No injectable classification seam: destructiveByClassification reads surfaces.ClassForCommand directly — verified by a repo-wide grep for 'classForCommand' (0 hits outside comments)"

key-files:
  created:
    - cmd/engram/destructive.go
    - cmd/engram/destructive_test.go
    - internal/e2e/spine_review_test.go
  modified:
    - cmd/engram/prune.go
    - cmd/engram/prune_test.go
    - cmd/engram/migrate.go
    - cmd/engram/migrate_test.go
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden
    - internal/surfaces/rules.go
    - internal/surfaces/rules_test.go
    - internal/surfaces/normalize_test.go
    - internal/surfacesgen/main.go
    - internal/store/spine.go
    - internal/store/spine_test.go
    - internal/store/store.go
    - docs-site/src/content/docs/guides/cli.md
    - docs-site/src/content/docs/guides/upgrade.md
    - docs-site/src/content/docs/reference/tools.md
    - docs-site/src/content/docs/reference/auth.md
    - skill/engram/skills/curating-memory/SKILL.md
    - CLAUDE.md

key-decisions:
  - "Task 1 checkpoint (resolved by the user before dispatch, not re-litigated): D-02/D-04 locked (--apply replaces --dry-run as the destructive-tier safety contract; prune-expired stops deleting without --apply immediately, no deprecation window); the two-idiom boundary (destructive->--apply, mutating-non-destructive->--dry-run) is an explicit documented decision, written into the CLI guide's new Destructive-commands section; option-a (full derivation) selected for migrate-remap-owner — its --dry-run flag is REMOVED, not aliased or deprecation-shimmed. Option-c (scope the gate to two commands) was already struck during the cross-AI review cycle and was not offered."
  - "mutationMode/modePreview/modeApply (named in the plan's action text as an available convenience type) was NOT added: golangci-lint's 'unused' check flags unused package-level declarations, and no call site in this plan's implementation needed to vary wording by an explicit mode value — each leaf's preview/apply closure already knows its own mode by construction (it IS the preview closure or the apply closure). Adding the type without a use site would have failed 'task lint clean', a hard acceptance criterion for both tasks."
  - "registerDestructive's preview/apply closure signature is func(context.Context, *cobra.Command) error — ctx BEFORE cmd — diverging from the plan's literally-written func(*cobra.Command, context.Context) error. golangci-lint's revive linter (context-as-argument, enabled repo-wide with no per-signature exception) requires context.Context to be a function's first parameter; the plan's own 'task lint clean' criterion takes precedence over the literal parameter order in the action text. The functional contract (two closures taking a command and a context) is unchanged."
  - "internal/surfaces/normalize_test.go's exposedForTest() fixture (NOT in this plan's declared file list) required a small extension: it predates the destructive tier and modeled only search/list/schedule_memory field sets, so the new destructive-requires-apply rule (Fields: [\"apply\"]) resolved to zero surfaces against it, failing the pre-existing TestEveryRuleResolvesToNonEmptySurfaceSet. Added a cobraDestructiveFields list unioned into the fixture's SurfaceCobraUsage — a Rule 3 (auto-fix blocking issue) fix, scoped to the exact gap this plan's own registry addition opened."
  - "docs-site/src/content/docs/reference/auth.md (NOT in this plan's declared file list) carried three live --dry-run mentions/examples for migrate-remap-owner (a preview-then-apply two-command idiom, plus a --dry-run flag callout) that would have told an operator to pass a flag the binary now rejects. Fixed as a direct, in-scope consequence of removing --dry-run (Rule 1: auto-fix a bug the plan's own change introduced), not a scope expansion — CLAUDE.md/tools.md were the only two surfaces the plan explicitly named, but this is the identical failure mode (a live doc advertising a since-removed flag)."
  - "pruneOutputDoc's JSON shape gained an explicit Preview boolean plus separate Eligible/Deleted fields (Eligible=0 in the applied doc, Deleted=0 in the preview doc) — the same 'explicit boolean plus separate count fields, never inferred from prose' pattern plan 03-02 established for migrate-remap-owner's dry-run/applied distinction, applied symmetrically here."

requirements-completed: [REQ-destructive-preview-default]

coverage:
  - id: D1
    description: "registerDestructive is the structural RunE choke point: destructive-tier membership is derived from surfaces.Operations() (panics on a misrouted non-destructive command), and every destructive command's installed RunE is verified at runtime via a runtime.FuncForPC substring match against \"registerDestructive\" — a hand-assigned RunE fails this gate"
    requirement: "REQ-destructive-preview-default"
    verification:
      - kind: unit
        ref: "cmd/engram/destructive_test.go#TestDestructiveCommandsRequireApply"
        status: pass
      - kind: unit
        ref: "cmd/engram/destructive_test.go#TestDestructiveCommandsRouteThroughGate"
        status: pass
      - kind: unit
        ref: "cmd/engram/destructive_test.go#TestDestructiveGatePreventsMutation"
        status: pass
      - kind: unit
        ref: "cmd/engram/destructive_test.go#TestDestructiveCommandsExactFlagSet"
        status: pass
      - kind: unit
        ref: "cmd/engram/destructive_test.go#TestDestructiveModeIgnoresEnvironment"
        status: pass
    human_judgment: false
  - id: D2
    description: "prune-expired previews by default (no delete, exits 0, reports the eligible count via the new CountExpired/expiredFilter pair) and mutates only under --apply; --apply=false behaves exactly like an omitted flag; proven both at package level and end-to-end against the built binary by re-reading the store"
    requirement: "REQ-destructive-preview-default"
    verification:
      - kind: unit
        ref: "cmd/engram/prune_test.go#TestPrunePreviewCutoffQuantisedThroughCliNowSeam"
        status: pass
      - kind: unit
        ref: "internal/store/spine_test.go#TestCountExpiredAndPruneExpiredAgree"
        status: pass
      - kind: e2e
        ref: "internal/e2e/spine_review_test.go#TestE2EPruneExpiredPreviewsBeforeApply"
        status: pass
      - kind: e2e
        ref: "internal/e2e/spine_review_test.go#TestE2EPruneExpiredPreviewZeroEligible"
        status: pass
    human_judgment: false
  - id: D3
    description: "migrate-remap-owner flips to the identical --apply contract per the resolved checkpoint (option-a): --dry-run is removed (not deprecated), a bare invocation previews, --apply performs the remap"
    requirement: "REQ-destructive-preview-default"
    verification:
      - kind: unit
        ref: "cmd/engram/migrate_test.go#TestMigrateRemapSummary"
        status: pass
      - kind: unit
        ref: "cmd/engram/destructive_test.go#TestDestructiveCommandsExactFlagSet"
        status: pass
    human_judgment: false
  - id: D4
    description: "The --apply contract is a registered conditional rule (RuleDestructiveRequiresApply) anchored on every applicable surface (CLI guide, curating-memory SKILL.md), and the old prune-expired usage contract survives in no doc surface named by this plan's acceptance criteria"
    requirement: "REQ-destructive-preview-default"
    verification:
      - kind: unit
        ref: "internal/surfaces/conformance_test.go#TestSurfaceConformanceProseFiles"
        status: pass
      - kind: unit
        ref: "internal/surfaces/rules_test.go#TestRuleByIDDestructiveRequiresApply"
        status: pass
      - kind: manual_procedural
        ref: "rg -o 'prune-expired \\[--older-than' CLAUDE.md docs-site/src/content/docs/reference/tools.md | wc -l -> 0"
        status: pass
    human_judgment: false

duration: ~90min
completed: 2026-08-06
status: complete
---

# Phase 3 Plan 3: Destructive-Tier Preview-By-Default Gate Summary

**`registerDestructive` makes the destructive tier's `--apply` gate a runtime-enforced RunE choke point (not just a derived flag), flips `prune-expired` and `migrate-remap-owner` to preview-by-default, and adds the first end-to-end coverage any operator command has ever had.**

## Performance

- **Duration:** ~90 min
- **Completed:** 2026-08-06
- **Tasks:** 3 (Task 1 was a checkpoint, resolved by the user before dispatch)
- **Files modified:** 22 (3 created, 19 modified)

## Accomplishments

- Built `registerDestructive` (`cmd/engram/destructive.go`): the structural RunE choke point every destructive command routes through. A destructive leaf supplies a preview closure and an apply closure and never assigns `cmd.RunE` itself — there is no code path from the preview branch to the apply closure. `destructiveByClassification` derives membership from `surfaces.Operations()` (panicking on a misrouted non-destructive command), `applyRequested` reads the flag's VALUE (never `pflag.Flag.Changed`, so `--apply=false` behaves exactly like an omitted flag), and `cliNow` is the injectable clock seam a preview cutoff reads.
- Made the guard runtime-verifiable, not merely flag-presence-verifiable: `TestDestructiveCommandsRouteThroughGate` resolves every destructive command's `RunE` via `runtime.FuncForPC` and asserts a substring match against `"registerDestructive"` — a hand-assigned `RunE` fails this test. This went naturally RED (not injected) at the commit that added `destructive.go`, since `prune-expired`/`migrate-remap-owner` still hand-assigned their own `RunE` at that point; converting them in the next commit turned it GREEN (see § Required Evidence below).
- Declared `RuleDestructiveRequiresApply` in the `internal/surfaces` registry and anchored its Sentence in a new "Destructive commands" section of the CLI guide (documenting the two-idiom `--apply`/`--dry-run` boundary explicitly, per the resolved checkpoint) and in curating-memory's `SKILL.md`, both filled by `task surfaces:gen`.
- Flipped `prune-expired` to preview-by-default: a bare invocation now counts eligible records via the new `internal/store.CountExpired` (sharing the ONE `expiredFilter` construction `PruneExpired`'s applied path also reads — the preview count and the applied count can never silently drift) and deletes nothing; `--apply` performs the delete unchanged from before.
- Flipped `migrate-remap-owner` to the identical contract per the resolved checkpoint's option-a: its `--dry-run` flag is REMOVED, not deprecated or aliased — passing it now fails with an unknown-flag usage error.
- Added `internal/e2e/spine_review_test.go`: the first end-to-end coverage any operator command has had (`internal/e2e` previously execed only `serve`/`search`/`list`). It seeds an expired record through `internal/store`, execs the built binary with no `--apply` and with `--apply=false`, and asserts BY RE-READING THE COLLECTION (`store.Get`) that the record survives — not merely that the process exited 0 — then execs `--apply` and asserts the record is gone.
- Documented the flip across all four surfaces the review cycle flagged (`upgrade.md` entries #9/#10, `CLAUDE.md`, `reference/tools.md`), plus a fifth the review cycle didn't name but this plan's own change broke: `reference/auth.md`'s two migration examples and a `--dry-run` callout.

## Task Commits

1. **Task 1: Checkpoint decision (resolved by user before dispatch)** — no commit; recorded below.
2. **Task 2: The derived preview-by-default gate and its registered conditional rule** — `f0d4926f` (feat)
3. **Task 3: Apply the gate — the hard flip, documentation, e2e coverage, fail-first proof** — `efd47d6a` (feat)

**Plan metadata:** _(pending — final `docs(03-03)` commit follows this SUMMARY)_

## Checkpoint Decision (Task 1 — resolved-by-user, not re-litigated)

The checkpoint was answered by the user before this executor was dispatched (subagent nested-checkpoint limitation, issue #1009). Recorded here for the plan's audit trail:

- **D-02 LOCKED:** `--apply` replaces `--dry-run` as the destructive tier's advertised safety contract.
- **D-04 LOCKED:** `prune-expired` stops deleting without `--apply` immediately, no deprecation window.
- **Two-idiom boundary ACKNOWLEDGED and documented:** destructive commands (`Destructive: true`) get opt-in MUTATION via `--apply`; mutating-but-non-destructive commands (`reindex`, `backfill-short-ids`, `summarize-missing`) keep opt-in PREVIEW via `--dry-run`. Written into `docs-site/src/content/docs/guides/cli.md`'s new "Destructive commands" section with the rationale (a forgotten `--apply` is a harmless no-op; a forgotten `--dry-run` merely performs the recoverable thing already asked for).
- **SELECTED: option-a (full derivation).** `migrate-remap-owner` also flips to `--apply`; its `--dry-run` flag is REMOVED — no deprecation shim (option-b was not chosen; option-c was already struck during the cross-AI review cycle and was not on the table).

## Files Created/Modified

- `cmd/engram/destructive.go` — `registerDestructive`, `destructiveByClassification`, `addApplyFlag`, `applyRequested`, `cliNow`
- `cmd/engram/destructive_test.go` — the six destructive-tier gate tests (see § Coverage)
- `cmd/engram/prune.go` — converted to `registerDestructive`; `prunePreview`/`pruneApplyRun` closures; `pruneOutputDoc` gains `Preview`/`Eligible` fields
- `cmd/engram/prune_test.go` — preview-summary, preview-doc, help.golden-section, cliNow-quantisation tests
- `cmd/engram/migrate.go` — `migrate-remap-owner` converted to `registerDestructive`; `--dry-run` flag removed; `migrateRemapRun`/`migrateRemapPreview`/`migrateRemapApply`
- `cmd/engram/migrate_test.go` — `TestMigrateRemapSummary` updated for the new preview wording
- `cmd/engram/testdata/help.golden`, `cmd/engram/testdata/catalog.golden` — regenerated (`--apply` added on both commands, `--dry-run` removed from `migrate-remap-owner`)
- `internal/surfaces/rules.go` — `RuleDestructiveRequiresApply`
- `internal/surfaces/rules_test.go` — `TestRuleByIDDestructiveRequiresApply`
- `internal/surfaces/normalize_test.go` — `exposedForTest()` extended with `cobraDestructiveFields` (see § Deviations)
- `internal/surfacesgen/main.go` — `ruleTargets` entries for the new rule
- `internal/store/spine.go` — `expiredFilter`, `CountExpired`
- `internal/store/spine_test.go` — `TestCountExpiredAndPruneExpiredAgree`
- `internal/store/store.go` — `PruneExpired` now calls `CountExpired`/`expiredFilter` instead of building its own filter
- `internal/e2e/spine_review_test.go` — end-to-end coverage of the flip
- `docs-site/src/content/docs/guides/cli.md` — "Destructive commands" section with the anchored rule sentence
- `docs-site/src/content/docs/guides/upgrade.md` — entries #9 (`prune-expired`) and #10 (`migrate-remap-owner`)
- `docs-site/src/content/docs/reference/tools.md`, `CLAUDE.md` — stale `prune-expired [--older-than DUR]` usage lines fixed
- `docs-site/src/content/docs/reference/auth.md` — two migration examples and a `--dry-run` callout fixed (see § Deviations)
- `skill/engram/skills/curating-memory/SKILL.md` — anchored rule sentence; `prune-expired` mention updated

## Decisions Made

See `key-decisions` in frontmatter.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Extended `internal/surfaces/normalize_test.go`'s `exposedForTest()` fixture**
- **Found during:** Task 2, first full-suite run after declaring `RuleDestructiveRequiresApply`
- **Issue:** `TestEveryRuleResolvesToNonEmptySurfaceSet` failed — the new rule's `Fields: ["apply"]` resolved to zero surfaces against the pre-existing synthetic exposed-fields fixture, which predates the destructive tier and models only search/list/schedule_memory field sets.
- **Fix:** Added a `cobraDestructiveFields` list (`apply`, `older-than`, `from`, `from-anon`, `from-missing`, `to`, `timeout`, `output`) unioned into the fixture's `SurfaceCobraUsage` entry, documented as destructive-tier-specific (not added to the jsonschema/proto lists, since no MCP arg struct or proto message carries an `apply` field).
- **Files modified:** `internal/surfaces/normalize_test.go`
- **Verification:** `go test ./internal/surfaces/... -count=1` green.
- **Committed in:** `f0d4926f` (Task 2 commit)

**2. [Rule 1 - Bug] Fixed `docs-site/src/content/docs/reference/auth.md`'s stale `--dry-run` mentions**
- **Found during:** Task 3, sweeping for every live doc surface still advertising the old `migrate-remap-owner --dry-run` contract
- **Issue:** Three live mentions (two migration-example command pairs and one `--dry-run`-first-then-real-run callout in a `:::caution` block) instructed an operator to pass a flag the binary now rejects with an unknown-flag usage error — a direct breakage caused by this plan's own `--dry-run` removal, on a surface the plan's acceptance criteria did not name (only `CLAUDE.md` and `reference/tools.md` were explicitly listed).
- **Fix:** Updated both example pairs to the preview-then-`--apply` idiom and reworded the caution block to describe the new default-preview behavior.
- **Files modified:** `docs-site/src/content/docs/reference/auth.md`
- **Verification:** `task lint` clean (`rumdl` markdown check); manual read of the rendered section.
- **Committed in:** `efd47d6a` (Task 3 commit)

**3. [Rule 3 - Blocking] Removed the unused `mutationMode`/`modePreview`/`modeApply` type the plan's action text named**
- **Found during:** Task 2, `task lint` after first draft of `destructive.go`
- **Issue:** `golangci-lint`'s `unused` linter flagged the type and both constants as unused — no leaf closure in this plan's implementation needed to branch on an explicit mode value (each closure already knows its own mode by construction).
- **Fix:** Removed the type/consts; `task lint clean` is a hard acceptance criterion for both tasks and takes precedence over an unused convenience the plan described but did not mandate a call site for.
- **Files modified:** `cmd/engram/destructive.go`
- **Verification:** `task lint` clean.
- **Committed in:** `f0d4926f` (Task 2 commit)

**4. [Rule 3 - Blocking] Swapped `registerDestructive`'s closure parameter order to `(ctx, cmd)`**
- **Found during:** Task 2, `task lint` after first draft
- **Issue:** `golangci-lint`'s `revive` (`context-as-argument`) flagged `func(*cobra.Command, context.Context) error` — the plan's literally-written signature — three times; the linter has no per-signature exception mechanism and is enabled repo-wide.
- **Fix:** Reordered to `func(context.Context, *cobra.Command) error` throughout `registerDestructive`, `prune.go`, `migrate.go`, and `destructive_test.go`'s closure literals. The functional contract (two closures taking a command and a context, dispatched by `applyRequested`) is unchanged.
- **Files modified:** `cmd/engram/destructive.go`, `cmd/engram/prune.go`, `cmd/engram/migrate.go`, `cmd/engram/destructive_test.go`
- **Verification:** `task lint` clean; all destructive-tier tests still green after the rename.
- **Committed in:** `f0d4926f` (Task 2 commit)

---

**Total deviations:** 4 auto-fixed (2 blocking/lint, 1 bug, 1 blocking test fixture gap)
**Impact on plan:** All four are either hard lint-gate compliance (mandated by the plan's own "task lint clean" acceptance criterion) or a direct, narrowly-scoped bug fix caused by this plan's own breaking change. No scope creep — no unrelated file was touched, and every fix is traceable to a concrete, observed failure this plan's own diff produced.

## Required Evidence (per this plan's critical execution constraints)

### 1. Natural (not injected) RED observation for `TestDestructiveCommandsRouteThroughGate`

Per constraint #4, `prune.go` genuinely still carried a hand-written `RunE` when this test was authored in Task 2 (Task 3 is what converts it), so the failure was reachable naturally rather than needing an injected mutation. Observed immediately after committing `destructive.go`/`destructive_test.go` (before converting `prune.go`/`migrate.go`):

```
=== RUN   TestDestructiveCommandsRequireApply
    destructive_test.go:92: destructive command "migrate-remap-owner" has no --apply flag
    destructive_test.go:92: destructive command "prune-expired" has no --apply flag
--- FAIL: TestDestructiveCommandsRequireApply (0.00s)
=== RUN   TestDestructiveCommandsRouteThroughGate
=== RUN   TestDestructiveCommandsRouteThroughGate/migrate-remap-owner
    destructive_test.go:143: migrate-remap-owner: RunE = github.com/seanb4t/engram/cmd/engram.init.func7, want a closure installed by registerDestructive (substring match)
=== RUN   TestDestructiveCommandsRouteThroughGate/prune-expired
    destructive_test.go:143: prune-expired: RunE = github.com/seanb4t/engram/cmd/engram.init.func8, want a closure installed by registerDestructive (substring match)
--- FAIL: TestDestructiveCommandsRouteThroughGate (0.00s)
```

After converting `prune.go` and `migrate.go` in Task 3 (both closures now routed through `registerDestructive`), the identical test run is fully green — see § Coverage. Both commits (`f0d4926f`, `efd47d6a`) themselves pass their own full test suite; the RED state above was observed in the working tree between the two commits, never committed.

### 2. MUTATION CHECK — `TestDestructiveGatePreventsMutation`

Not an injected defect (the plan classifies this as the ONE behavioural proof, built correctly from the start): verified the throwaway command's apply-closure call counter is 0 for a bare run, 0 for `--apply=false`, and 1 for `--apply`, using a name derived from `surfaces.Operations()` (`firstDestructiveTopLevelCommandName`) rather than a hardcoded `"prune-expired"` literal — confirmed via `rg -v '^\s*//' cmd/engram/destructive_test.go | rg -o 'prune-expired' | wc -l` → `0`.

### 3. Grep-gate results (all Task 2/3 acceptance-criteria grep checks)

```
rg -v '^\s*//' cmd/engram/destructive_test.go | rg -o 'prune-expired' | wc -l          -> 0
rg -v '^\s*//' cmd/engram/destructive_test.go | rg -o '\.func[0-9]' | wc -l            -> 0
rg -v '^\s*//' cmd/engram/destructive_test.go | rg -o 'strings\.Contains' | wc -l      -> 3
rg -c '^//go:noinline' cmd/engram/destructive.go                                       -> 1
rg -v '^\s*//' cmd/engram/destructive.go | rg -o 'Changed' | wc -l                     -> 0
rg -v '^\s*//' cmd/engram/destructive.go cmd/engram/destructive_test.go | rg -o 'classForCommand' | wc -l -> 0
rg -c 'engram:rule:start destructive-requires-apply' docs-site/.../cli.md              -> 1
rg -c 'engram:rule:start destructive-requires-apply' skill/.../SKILL.md                -> 1
rg -c 'cliNow' cmd/engram/destructive.go                                               -> 2
rg -v '^\s*//' cmd/engram/prune.go | rg -o 'time\.Now\(\)' | wc -l                     -> 0
rg -o 'prune-expired \[--older-than' CLAUDE.md docs-site/.../reference/tools.md        -> 0
rg -v '^\s*//' internal/store/spine.go | rg -o 'func expiredFilter\(' | wc -l          -> 1
rg -v '^\s*//' internal/store/store.go | rg -o 's\.CountExpired\(' | wc -l             -> 1
```

**One documented exception:** `rg -v '^\s*//' internal/store/spine.go internal/store/store.go | rg -o 'NewRange\("not_after"' | wc -l` → **3**, not the plan's stated `1`. Verified before making any change (per the planning-artifacts discipline of checking the tool/codebase before assuming a plan defect): `store.go` carries two OTHER, pre-existing `qdrant.NewRange("not_after", ...)` calls unrelated to `PruneExpired` — `activeWindowConditions` (line ~860, `Gt` comparison, the recall "not yet expired" gate) and `scheduledStateCondition` (line ~1340, `Lte` comparison, the `list_scheduled` expired-state filter). Both predate this plan, serve a structurally different purpose (recall visibility, not deletion), and use different comparison operators than `expiredFilter`'s `Lt` — they cannot correctly share `expiredFilter` without changing their semantics. The plan's own two MORE PRECISE greps (`func expiredFilter\(` and `s\.CountExpired\(`, both `= 1` as required) are what actually prove the "one filter, called twice" invariant; the broader `NewRange("not_after"` grep appears to have been written without accounting for these two pre-existing, unrelated call sites. Not fixed, since "fixing" it would mean refactoring unrelated recall-gating code to share a filter with different semantics — a Rule 4 architectural change with no clear benefit, not attempted.

### 4. Three `-shuffle=<seed>` runs (`cmd/engram` + `internal/surfaces`)

`-shuffle=1`, `-shuffle=7`, `-shuffle=13` all green, run after the full implementation (post-Task-3).

### 5. `go clean -testcache && task`

Ran after Task 3's changes (the point at which the "first `internal/e2e` test execing a changed operator command" justification becomes true, per constraint #8's corrected scope — before this task, `internal/e2e` execed only `serve`/`search`/`list`, so the flush protected nothing about this specific change). All green, including `internal/store` (testcontainers) and `internal/e2e` (testcontainers + built-binary exec).

### 6. Key-links verification

`gsd-tools verify key-links .planning/phases/03-spine-curation-structural-cli/03-03-PLAN.md` → `{"all_verified": true, "verified": 4, "pending": 0, "total": 4}`.

## Known Stubs

None. `prunePreview`/`pruneApplyRun` and `migrateRemapPreview`/`migrateRemapApply` are wired to real `internal/store` calls (`CountExpired`/`PruneExpired`, `RemapOwner`); no placeholder or hardcoded-empty rendering exists anywhere in this plan's diff.

## Threat Flags

None beyond what the plan's own `<threat_model>` already anticipated (T-03-08, T-03-12, T-03-13, T-03-25) — all four are mitigated as designed:

- T-03-08 (classification tampering): membership derived from `surfaces.Operations()`, gated both directions by `TestDestructiveCommandsRequireApply`; `registerDestructive`'s own panic backstop prevents an unclassified command from reaching the gate.
- T-03-12 (over-broad blast radius / `prune-expired` default): inverted — a bare run previews, proven by `TestE2EPruneExpiredPreviewsBeforeApply` re-reading the store, not by exit-code inspection.
- T-03-13 (repudiation / silent behavior change): the upgrade guide entries name old behavior, new behavior, and the exact restoring flag for both commands.
- T-03-25 (a future destructive command bypassing the gate): `registerDestructive` owns the `RunE`; `TestDestructiveCommandsRouteThroughGate` resolves `runtime.FuncForPC` for every table-derived destructive command.

## Issues Encountered

None beyond the four auto-fixes recorded in § Deviations from Plan — no architectural (Rule 4) questions arose, and no checkpoint decision required re-litigation.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `registerDestructive` is established as this phase's shared choke point for the destructive tier — plan 03-07's `spine-review purge` (and any `archive`/`restore` leaves 03-06's checkpoint classifies destructive) route through it directly, adding a row to `cmd/engram/destructive_test.go`'s `destructiveFlagCases` table rather than inventing a parallel mechanism.
- `internal/store/spine.go`'s `expiredFilter`/`CountExpired` pair is the declared single site plan 03-06 extends with an `archived_at` IsEmpty condition — its own acceptance grep already spans both `store.go` and `spine.go` for this reason.
- `internal/e2e/spine_review_test.go` is the file plan 03-07 extends into the phase-wide acceptance run.
- One documented, justified exception to a literal acceptance-criteria grep (§ Required Evidence #3) — not a blocker, and does not affect the functional guarantee the grep was meant to prove (verified by two more precise greps that both pass exactly as specified).
- No blockers for plan 03-04.

## Self-Check: PASSED

All key files (`cmd/engram/destructive.go`, `destructive_test.go`, `prune.go`, `migrate.go`, `internal/store/spine.go`, `internal/e2e/spine_review_test.go`, `docs-site/.../cli.md`, `docs-site/.../upgrade.md`) confirmed present on disk. Both task commit hashes (`f0d4926f`, `efd47d6a`) confirmed present in `git log --oneline --all`.

---
*Phase: 03-spine-curation-structural-cli*
*Completed: 2026-08-06*
