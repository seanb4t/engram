# Phase 1: Interface Enforceability - Research

**Researched:** 2026-08-03
**Domain:** Go CLI interface contract (cobra flag-group validation, process exit-code taxonomy,
context-deadline plumbing, koanf configuration registry)
**Confidence:** HIGH — every claim below is either read from the pinned source (this repo's
`cmd/engram/`, `internal/config/`, `internal/store/`, `internal/server/`) or from the exact pinned
dependency version (`cobra@v1.10.2`, `connectrpc.com/connect@v1.20.0`) read directly out of the
local module cache this session.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Exit-code taxonomy**

- **D-01:** `exitAuth = 3` survives the unification. The success criterion's "0/2/4/5" names the
  codes the unification moves things *into*, not an exhaustive taxonomy.
  — Reversibility: one-way (published contract).

- **D-02:** `exitGeneric = 1` is kept but redefined as an unreachable-by-design internal-error
  backstop. Every classified path becomes typed (framework flag errors → 2, operator command
  failures → 2/3/4/5), so nothing routes to 1 deliberately.
  — Reversibility: costly.

- **D-03:** The six operator commands (`serve`, `reindex`, `prune-expired`, `migrate-remap-owner`,
  `summarize-missing`, `backfill-short-ids`) get full classification in the same 2/3/4/5
  vocabulary as client verbs: bad flag value → 2, backend unreachable → 5, auth failure → 3,
  missing target → 4. Store/Qdrant errors need explicit classification (not Connect errors, so
  the D-10 mapper does not reach them).
  — Reversibility: one-way.

**Client configuration (scope expansion)**

- **D-04:** All client flags/settings route through koanf, landing in Phase 1. `--timeout`/
  `ENGRAM_TIMEOUT` is the first field declared the new way rather than a fourth hand-rolled
  resolver alongside `resolveServerURL`, `--token-file`, `--output`, `--insecure`. Constraint
  (memory `s780vae1vr`): every new required-with-a-default `internal/config` registry field must
  be added to every full `Config{}` literal in that package's tests, or previously-green tests
  fail on the empty-string zero value.
  — Reversibility: costly.

- **D-05:** Default timeout is 30s; a value of 0 is rejected as a usage error (exit 2), not
  treated as unbounded. This forces a conscious reconciliation with `migrate-remap-owner`'s
  existing `--timeout` where 0 currently *does* mean unbounded (`migrate.go:65,116`) — the binary
  must not ship two `--timeout` flags with opposite semantics.
  — Reversibility: reversible.

- **D-06:** A timeout reports a new dedicated exit code 6, distinguishing "server never answered
  in time" from "couldn't connect at all" (5). Consequences: new constant, new `catalog.go` entry
  (gated by `TestCatalogExitCodesMatchMapper`), a `guides/upgrade.md` line, consumer audit
  covering the new code too.
  — Reversibility: one-way.

**Flag-group enforcement**

- **D-07:** All three exclusivity claim sites convert to cobra's declarative API:
  - `client_list.go:98-106` paging trio (`--offset`/`--cursor-mode`/`--page-token`) — enforced
    nowhere → `MarkFlagsMutuallyExclusive`.
  - `client_common.go:236` (`--scope`/`--cross-spine`) — shared guard across `search`/`list`, so
    conversion means declaring the group on each command.
  - `migrate.go:73-86` (`--from`/`--from-missing`/`--from-anon`) — needs exactly-one, pair
    `MarkFlagsMutuallyExclusive` with `MarkFlagsOneRequired`. `buildRemapSource` keeps its
    `store.ValidateOwnerRemap` call and stays pure/unit-testable, losing only the `selected != 1`
    counting.
  — Reversibility: reversible.

- **D-08:** `--page-token` together with `--offset` becomes an error, not a silent ignore.
  Rejecting before any dial (exit 2) makes the paging model correct-by-reading.
  — Reversibility: one-way.

**Migration proof**

- **D-09:** The before-table lands as its own plan, committed before any behavior change: a
  table-driven test enumerating every command × failure mode with its **current** exit code,
  landing green against unchanged code. Assert before/after codes are distinct where claimed to
  change and identical where claimed not to (memory `nczgrtfec2`) — a loose "codes are as
  expected" assertion passes while classification silently collapses.
  — Reversibility: reversible, but cannot be recovered once behavior changes, so must be
  respected at plan-sequencing time.

- **D-10:** "An audit of known consumers" is closed by an in-repo sweep plus a documented
  statement of external posture: `Taskfile.yaml`, `.github/workflows/`, `charts/engram/`,
  `skill/engram/`, `docs-site/` examples — fix anything that branches on exit status, and record
  in `guides/upgrade.md` that external consumers are addressed by the guide itself (engram is
  self-hosted, no telemetry).
  — Reversibility: reversible.

### Claude's Discretion

- How cobra's flag-group validation errors are intercepted and typed to exit 2 (e.g.
  `SetFlagErrorFunc` on root vs central classification in `Execute()`). The outcome is fixed by
  D-02/D-03; the mechanism is the planner's call. **See Pattern 1 below — this research
  identifies and recommends a specific mechanism verified against cobra 1.10.2's actual source.**
- The precise rewording of the D-17 note in `catalog.go:92-98`.
- The shape of the client koanf config struct introduced by D-04.
- Whether the `client_common.go:236` shared guard is retained as a defense-in-depth backstop
  after cobra takes over the declaration, or removed outright.

### Deferred Ideas (OUT OF SCOPE)

None from scope creep — every expansion raised during discussion (the koanf client-config
unification) was deliberately placed *into* this phase rather than deferred (D-04).

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-flag-exclusivity-enforced | Every documented mutually-exclusive flag combination rejected before any network call, via cobra's declarative flag-group API | Pattern 1 (mechanism), Pattern 2 (all 3 sites enumerated with exact flag lists) |
| REQ-exit-code-unified | Every command — client and operator — resolves errors through one 0/2/3/4/5(/6) taxonomy, including cobra's own flag-group validation | Pattern 1 (interception), Pitfall 1-4 (every bare-exit-1 site found, including 2 NOT named in CONTEXT.md canonical_refs), Code Example 3 (operator classification model) |
| REQ-exit-code-migration-safe | Table-driven regression test pinning current behavior, consumer audit, `guides/upgrade.md` entry | Pitfall 5 (why `assertExitCode` cannot be reused as-is for the before-table), D-10 Consumer Sweep section (complete, verified negative result) |
| REQ-cli-request-timeout | Every CLI RPC path applies a finite, operator-configurable deadline; timeout reports a documented exit code | Pattern 4 (exact wiring point), Pattern 5 (D-06's exit-6 split inside `exitCodeForConnectErr`), Pitfall 6 (`--timeout` name collision with operator commands) |
| REQ-client-config-unified | Every client flag/setting resolves through the `internal/config` koanf registry | Pattern 3 (registry shape + precedent), Pitfall 7 (test-literal tax scoping — how to avoid a 33-call-site blast radius) |

</phase_requirements>

## Summary

This phase is entirely in-repo, code-only, zero-new-dependency work: cobra `v1.10.2` and koanf
`v2.3.5`/`env/v2 v2.0.0`/`confmap v1.0.0` are already vendored and imported
`[VERIFIED: go.mod:8,15-17,20]`. The two hardest technical questions CONTEXT.md defers to planner
discretion both have concrete, verifiable answers from reading cobra 1.10.2's actual `execute()`
control flow and this repo's existing `connectError`/`exitCodeForConnectErr` classification
patterns.

**The flag-group interception mechanism.** cobra 1.10.2 calls `c.ValidateFlagGroups()` directly
inline inside `Command.execute()`, unconditionally, immediately **before** `RunE` and immediately
**after** `PersistentPreRunE`/`PreRunE`
`[VERIFIED: cobra@v1.10.2/command.go:999-1012 — see Pattern 1 for the exact quoted block]`. Cobra
offers **no** dedicated hook for this call the way `SetFlagErrorFunc` hooks `ParseFlags` errors —
`ValidateFlagGroups()`'s three internal validators (`validateRequiredFlagGroups`,
`validateOneRequiredFlagGroups`, `validateExclusiveFlagGroups`) each return a bare `fmt.Errorf`
with no type or sentinel `[VERIFIED: cobra@v1.10.2/flag_groups.go:161,183,204]`. But because
`PersistentPreRunE` runs *before* cobra's own `ValidateFlagGroups()` call, and because this repo's
`rootCmd.PersistentPreRunE` (root.go:45-47) already runs for every subcommand today (no subcommand
in this repo defines its own `PersistentPreRunE`, and `EnableTraverseRunHooks` defaults to `false`
`[VERIFIED: cobra@v1.10.2/cobra.go:49]`, so cobra calls the *first* `PersistentPreRunE` found
walking child→parent, which is root's), the single change of calling `cmd.ValidateFlagGroups()`
manually inside `rootCmd.PersistentPreRunE`, wrapping any non-nil result in `*cliError{code:
exitUsage}`, and returning it short-circuits `execute()` at line 1000 before cobra's own redundant
internal call at line 1010 is ever reached. This is a single, root-level change that types **all
three** D-07 flag-group sites to exit 2 with no per-command duplication. `SetFlagErrorFunc` is
still needed, but for a *different* error class: `ParseFlags` failures (bad flag syntax, unknown
flag), which currently also fall through to exit 1 per the very D-17 catalog note this phase
retracts.

**The exit-6 timeout classification.** `exitCodeForConnectErr` already has a documented,
version-pinned citation that a client-side `context.WithTimeout` deadline surfaces through
connect-go as `connect.CodeDeadlineExceeded`
`[VERIFIED: cmd/engram/client_common.go:286-291, corroborated against connectrpc.com/connect
v1.20.0 pinned in go.mod:8]`. Today that code is folded into `exitUnavailable` alongside
`CodeUnavailable`/`CodeCanceled` (client_common.go:300). D-06 is a clean, surgical change: split
`CodeDeadlineExceeded` into its own `case` returning a new `exitTimeout = 6`, leaving
`CodeUnavailable`/`CodeCanceled` at `exitUnavailable = 5`. `TestCatalogExitCodesMatchMapper`
mechanically re-derives its expected set from the mapper's own producible codes across
`connect.Code(1..16)` `[VERIFIED: cmd/engram/catalog_test.go:304-322]`, so this test needs no
edit — but a **different**, more naive test does: `TestCatalogListsEveryExitCode` hard-codes
`len(doc.ExitCodes) != 6` and `ec.Code < 0 || ec.Code > 5`
`[VERIFIED: cmd/engram/catalog_test.go:218-242, quoted verbatim]` — this WILL fail red once exit 6
is added and must be updated to 7/0-6 as part of the same commit, not discovered later as a
surprise CI failure.

**The bare-exit-1 surface is larger than CONTEXT.md's canonical_refs names.** Reading
`internal/config/legacy.go` found a fourth bare-`fmt.Errorf` site:
`config.CheckLegacy`, called from `rootCmd.PersistentPreRunE` for **every** command (client or
operator) `[VERIFIED: internal/config/legacy.go:54-55, cmd/engram/root.go:45-47]` — a
half-migrated-env-var failure currently exits 1 today, the same "usage-shaped failure falls
through to undocumented 1" bug class SC2 exists to close. Reading `cobra@v1.10.2/command.go:1123-
1134` also found that a mistyped subcommand name (cobra's `Find()` failing) returns its bare error
*before* `execute()` is ever called, bypassing `PersistentPreRunE` entirely — this is a distinct,
earlier interception point that the root-`PersistentPreRunE` mechanism above does **not** reach,
and CONTEXT.md does not decide whether it is in scope (see Open Questions).

**Standard-stack scope is empty.** No new package is introduced by this phase; cobra's
`MarkFlagsMutuallyExclusive`/`MarkFlagsOneRequired`/`SetFlagErrorFunc` and koanf's existing
`registry.go`/`Load()`/`Validate()` machinery are both already present and load-bearing elsewhere
in this codebase. The Package Legitimacy Audit is consequently empty — see that section for the
explicit statement.

**Primary recommendation:** Centralize flag-group-error classification in `rootCmd`'s existing
`PersistentPreRunE` (extending it, not adding a parallel mechanism), pair it with
`rootCmd.SetFlagErrorFunc` for `ParseFlags` errors, add a `cliErrorForStoreErr`-style sentinel
classifier modeled directly on `internal/server/connecterror.go`'s existing `connectError`
pattern for the six operator commands, split `exitCodeForConnectErr`'s `CodeDeadlineExceeded` arm
into the new exit 6, and keep client-only koanf fields (`--server`, `--token-file`, `--output`,
`--insecure`, `--timeout`) validated in a **separate** function from `Config.Validate()` to avoid
inflating the D-04 test-literal tax onto all ~33 existing full-`Config{}`-literal call sites in
`internal/config`'s test files (see Pitfall 7).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Flag-group exclusivity enforcement | CLI / cobra command tree (`cmd/engram/`) | — | Pure client-side input shape validation; must reject before any network dial per SC1 |
| Exit-code classification | CLI / process boundary (`root.go` `Execute()`/`exitCodeFromError`) | Store layer (sentinel errors) | The exit code is the CLI's own process contract; store/Connect errors are *inputs* the CLI classifier consumes, never the other way round |
| Client RPC timeout | CLI / Connect client construction (`clientFromFlags`, `newHTTPClient`) | API/Backend (Connect server, unaffected) | The deadline is a client-side promise about how long *this invocation* will wait; the server's own request handling is untouched by this phase |
| Client configuration resolution | CLI / `internal/config` koanf registry | — | Already the single source of truth for server-side `ENGRAM_` vars; D-04 extends the same registry to client fields rather than inventing a second resolution mechanism |
| Operator-command error classification | CLI / Store (`internal/store` sentinel errors, Qdrant gRPC codes) | — | Operator commands call `internal/store` directly (no Connect hop), so classification must read store/gRPC error shapes, not `connectError`'s Connect-code vocabulary |

## Standard Stack

### Core

No new libraries. This phase's entire surface is already-vendored:

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/spf13/cobra` | v1.10.2 `[VERIFIED: go.mod:20]` | Flag-group declarative API (`MarkFlagsMutuallyExclusive`, `MarkFlagsOneRequired`, `SetFlagErrorFunc`) | Already the CLI framework for every command in this binary; the milestone's standing constraint is near-zero new dependencies `[CITED: .planning/REQUIREMENTS.md:14-16]` |
| `github.com/knadh/koanf/v2` + `providers/env/v2` + `providers/confmap` | v2.3.5 / v2.0.0 / v1.0.0 `[VERIFIED: go.mod:15-17]` | Config registry D-04 extends to client fields | Already `internal/config`'s single source of truth for server-side `ENGRAM_*` vars |
| `connectrpc.com/connect` | v1.20.0 `[VERIFIED: go.mod:8]` | Context-deadline → `CodeDeadlineExceeded` propagation the D-06 exit-6 split relies on | Already the wire protocol for every client verb |

### Supporting

None new.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Extending `rootCmd.PersistentPreRunE` to call `ValidateFlagGroups()` early | A per-command `PreRunE` calling the same helper on each of the 3 D-07 command sites | Functionally equivalent, but adds 3+ near-duplicate `PreRunE` assignments and risks a future command forgetting to wire it — the root hook is structurally impossible to skip |
| Splitting `exitCodeForConnectErr`'s `CodeDeadlineExceeded` case for D-06 | A separate `errors.Is(err, context.DeadlineExceeded)` check ahead of `wrapRPCError` | Redundant: the existing code comment (client_common.go:286-291) already establishes that a client-side deadline surfaces as `connect.CodeDeadlineExceeded` through connect-go's client stack, so a second raw-context check would never fire and would be dead code |
| A single `Config.Validate()` covering client fields too | Keep client-field validation in a separate function (recommended, see Pitfall 7) | The combined approach inflates the D-04 test-literal tax onto every existing test that calls `Validate()` with a hand-built `Config{}` literal; `Validate()`'s own doc comment already documents this exclusion pattern for OIDC/UI fields |

**Installation:** None required — no `go get`/`npm install` step for this phase.

**Version verification:** All three dependency versions above were read directly from
`go.mod` this session, not recalled from training data.

## Package Legitimacy Audit

**Not applicable — this phase installs no new external packages.** `cobra` and `koanf` are both
already-vendored, already-imported dependencies confirmed present at the exact pinned versions
above `[VERIFIED: go.mod]`. Per the milestone's standing constraint, "New Go dependencies for any
capability in this milestone" is explicitly out of scope
`[CITED: .planning/REQUIREMENTS.md — Out of Scope table, "New Go dependencies for any capability
in this milestone"]`.

## Architecture Patterns

### System Architecture Diagram

```
                         engram <verb> [flags...]
                                  │
                                  ▼
                    ┌──────────────────────────┐
                    │  cobra.Command.execute()  │   cobra@v1.10.2/command.go:905
                    └──────────────────────────┘
                                  │
                  ┌───────────────┴───────────────┐
                  │ 1. c.ParseFlags(a)             │  line 919
                  │    on error → c.FlagErrorFunc() │  ← SetFlagErrorFunc hook (Pattern 2)
                  └───────────────┬───────────────┘
                                  ▼
                  ┌───────────────────────────────┐
                  │ 2. rootCmd.PersistentPreRunE   │  line 985-988 (root's hook wins;
                  │    - config.CheckLegacy(...)   │   EnableTraverseRunHooks=false)
                  │    - cmd.ValidateFlagGroups()  │  ← NEW: manual early call (Pattern 1)
                  │      both wrapped in *cliError │
                  └───────────────┬───────────────┘
                                  ▼
                  ┌───────────────────────────────┐
                  │ 3. c.PreRunE (none defined      │  line 999-1005
                  │    today in this repo)          │
                  └───────────────┬───────────────┘
                                  ▼
                  ┌───────────────────────────────┐
                  │ 4. c.ValidateFlagGroups()       │  line 1010 — cobra's OWN call;
                  │    (redundant re-check, no-op   │   short-circuited by step 2 returning
                  │    if step 2 already typed it)  │   non-nil above
                  └───────────────┬───────────────┘
                                  ▼
                  ┌───────────────────────────────┐
                  │ 5. c.RunE(cmd, args)            │  line 1014-1017
                  │    CLIENT VERBS:                │
                  │      validateScopeCrossSpine →  │  client_common.go:220-242 (usage guard,
                  │      usageErrorf                │   D-07 converts the enforcement layer,
                  │      clientFromFlags → dial      │   this call may become a backstop)
                  │      ctx w/ D-05 timeout → RPC   │  ← NEW: context.WithTimeout wiring (Pattern 4)
                  │      wrapRPCError(err) →         │
                  │      exitCodeForConnectErr        │  ← D-06 splits CodeDeadlineExceeded here
                  │    OPERATOR COMMANDS:            │
                  │      server.StoreFromEnv() /      │
                  │      StoreAndEmbedderFromEnvNoEnsure /
                  │      StoreAndSummarizerFromEnv    │  config load+validate errors (usage) OR
                  │      → st.<Operation>(ctx, ...)   │  EnsureCollection/Qdrant errors (unavailable)
                  │      today: plain error returned  │  ← NEW: classify via store sentinel /
                  │                                    │    gRPC-code mapper (Pattern 3, D-03)
                  └───────────────┬───────────────┘
                                  ▼
                  ┌───────────────────────────────┐
                  │ Execute() (root.go:59-64)       │
                  │   os.Exit(exitCodeFromError(err))│  ← the ONE place a process exit status
                  └───────────────────────────────┘     is decided; errors.As walk, default 1
```

### Recommended Project Structure

No new files or directories are required — every change lands in existing `cmd/engram/*.go` and
`internal/config/*.go` files:

```
cmd/engram/
├── root.go            # extend PersistentPreRunE (flag-group interception),
│                       # add SetFlagErrorFunc, extend exitCodeFromError if a
│                       # dedicated flag-group/legacy-env sentinel is introduced
├── client_common.go    # exitCodeForConnectErr gains the exitTimeout=6 split;
│                       # exit-code constants gain exitTimeout; timeout resolution
│                       # + context.WithTimeout wiring
├── client_list.go       # MarkFlagsMutuallyExclusive("offset","cursor-mode","page-token")
├── client_search.go     # MarkFlagsMutuallyExclusive("scope","cross-spine")
├── migrate.go            # MarkFlagsMutuallyExclusive + MarkFlagsOneRequired for the
│                        # tri-state source flags; buildRemapSource loses selected-count math
├── reindex.go, prune.go, summarize.go, backfill.go, serve.go
│                        # each RunE's returned errors reclassified per D-03 (see Pattern 3)
├── catalog.go            # new exitTimeout entry in doc.ExitCodes; D-17 note reworded
└── *_test.go              # D-09's before-table (new test file recommended, e.g.
                           # exitcode_baseline_test.go) + updates to TestCatalogListsEveryExitCode
                           # and TestCatalogDocumentsFlagParseExitCode (Pitfall 3, 5)

internal/config/
├── registry.go           # new client.* fields (server, token_file, output, insecure, timeout)
├── config.go             # new ClientConfig struct
├── validate.go            # client fields validated SEPARATELY (see Pitfall 7), not folded
│                          # into Config.Validate()
└── *_test.go              # only touched if client validation is added to the SAME test file
                           # that already builds full Config{} literals — recommend a new file
```

### Pattern 1: Centralized flag-group-error interception via `PersistentPreRunE`

**What:** `rootCmd.PersistentPreRunE` already runs for every command in this binary today
(`config.CheckLegacy`). Extend it to also call `cmd.ValidateFlagGroups()` and wrap any error.

**When to use:** This is the recommended mechanism for D-07's flag-exclusivity errors reaching
exit 2 (REQ-flag-exclusivity-enforced, REQ-exit-code-unified). It is the *only* mechanism that
requires a single edit rather than N per-command edits, because cobra's own internal
`ValidateFlagGroups()` call (line 1010) has no dedicated hook the way `ParseFlags` errors do.

**Verified control-flow evidence (cobra@v1.10.2/command.go:955-1017):**
```go
c.preRun()
defer c.postRun()
argWoFlags := c.Flags().Args()
// ...
for _, p := range parents {
    if p.PersistentPreRunE != nil {
        if err := p.PersistentPreRunE(c, argWoFlags); err != nil {
            return err                          // <-- short-circuits BEFORE line 1010
        }
        if !EnableTraverseRunHooks {
            break
        }
    }
    // ...
}
if c.PreRunE != nil {
    if err := c.PreRunE(c, argWoFlags); err != nil {
        return err
    }
}
if err := c.ValidateRequiredFlags(); err != nil {
    return err
}
if err := c.ValidateFlagGroups(); err != nil {     // <-- cobra's own unconditional call
    return err
}
if c.RunE != nil {
    if err := c.RunE(c, argWoFlags); err != nil {
        return err
    }
}
```
`[VERIFIED: cobra@v1.10.2/command.go:972-1017]`

**Verified: `p.PersistentPreRunE(c, argWoFlags)` is called with `c`, the leaf/running command,
not `p`** — so a hook defined on `rootCmd` receives the actual subcommand (e.g. `searchCmd`) as
its `cmd` parameter, meaning `cmd.ValidateFlagGroups()` called from inside `rootCmd`'s hook
validates the *running* command's flag groups, not root's own (root declares none)
`[VERIFIED: cobra@v1.10.2/command.go:986, confirmed by the callback signature `func(cmd *cobra.Command, args []string) error` receiving the same `c` bound throughout `execute()`]`.

**Recommended shape:**
```go
// root.go — extends the existing hook, does not add a parallel mechanism.
PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
    if err := config.CheckLegacy(os.Environ()); err != nil {
        return usageErrorf("%s", err)   // D-02: also closes this repo's 4th bare-exit-1 site
    }
    if err := cmd.ValidateFlagGroups(); err != nil {
        return usageErrorf("%s", err)   // D-07/D-08: types all 3 flag-group sites to exit 2
    }
    return nil
},
```
Cobra's own redundant call to `c.ValidateFlagGroups()` at line 1010 still executes afterward when
this hook returns nil, but it re-validates the *same* unchanged flag state and can only ever
agree (nil) — it is a no-op, not a double-classification risk.

**Why not `SetFlagErrorFunc` for this:** `SetFlagErrorFunc` is invoked only from
`c.FlagErrorFunc()(c, err)` at line 921, which is reached only when `c.ParseFlags(a)` itself
errors (line 919-920) — malformed flag syntax, unknown flag name, bad value for the flag's Go
type. `ValidateFlagGroups()` runs at a completely separate call site (line 1010), 90 lines later
in `execute()`, with no `FlagErrorFunc`-style indirection — `SetFlagErrorFunc` structurally cannot
see this error. Both mechanisms are needed; they cover disjoint error classes.

### Pattern 2: `SetFlagErrorFunc` for `ParseFlags` errors (the D-17 note's other half)

**What:** cobra's `SetFlagErrorFunc(f func(*Command, error) error)` — settable on root and
inherited by every subcommand via `FlagErrorFunc()`'s parent walk when the subcommand sets none
of its own `[VERIFIED: cobra@v1.10.2/command.go:326-330,544-558]`.

**When to use:** For malformed flag syntax (`--limit=notanumber`, an unrecognized `--bogus` flag)
— the OTHER half of the D-17 catalog note ("A flag-parsing error raised by the command framework
itself... exits 1, not 2"). SC2's "no path falls through to a bare, undocumented exit 1" reads as
covering this too, since D-02 says "nothing routes to 1 deliberately" once this phase ships.

**Example:**
```go
// Source: cobra@v1.10.2/command.go:326-330 (API), applied at root.go init
rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
    return usageErrorf("%s", err)
})
```

### Pattern 3: Operator-command error classification, modeled on `connectError`

**What:** `internal/server/connecterror.go`'s existing `connectError` function is a single,
already-battle-tested sentinel-matching switch from a `deps.*` business error to a Connect code
`[VERIFIED: internal/server/connecterror.go:55-100, quoted in full below]`. D-03 needs the exact
same *shape* of function — a single switch matching store sentinels — but targeting CLI exit
codes instead of Connect codes, since the six operator commands call `internal/store` directly
(no Connect hop) and so never pass through `connectError`/`exitCodeForConnectErr` at all.

```go
// Source: internal/server/connecterror.go:55-100 (read this session, verbatim)
func connectError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	var ae *argError
	switch {
	case errors.As(err, &ae):
		return connect.NewError(ae.ConnectCode(), err)
	case errors.Is(err, store.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, store.ErrInvalidArgument):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, errRuleImmutable):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, errStaleSummary):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, store.ErrAmbiguousShortID):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, store.ErrIdempotencyConflict):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, store.ErrAlreadySuperseded):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, context.Canceled):
		return connect.NewError(connect.CodeCanceled, err)
	case errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeDeadlineExceeded, err)
	default:
		slog.ErrorContext(ctx, "connect handler: unexpected error", "error", err)
		return connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
}
```

**Store sentinel vocabulary available for the CLI-side equivalent**
`[VERIFIED: internal/store/store.go:66,73,78,84,92,99, quoted verbatim]`:
```go
var ErrNotFound = errors.New("not found")                                    // line 66
var ErrInvalidArgument = errors.New("invalid argument")                       // line 73
var ErrAmbiguousShortID = errors.New("ambiguous short id")                    // line 78
var ErrShortIDExhausted = errors.New("short id mint exhausted")               // line 84
var ErrIdempotencyConflict = errors.New("idempotency key reused with different content") // line 92
var ErrAlreadySuperseded = errors.New("target is already superseded")         // line 99
```

**Recommended CLI-side mapping** (exit taxonomy per D-01/D-02/D-03/D-06):

| Store/config condition | Exit code | Rationale |
|---|---|---|
| `config.Load`/`Config.Validate` error (bad `ENGRAM_QDRANT_ADDR`, missing `ENGRAM_SUMMARY_MODEL` for `summarize-missing`, missing `--target`/`--owner`/`--scope`, `buildRemapSource`'s `selected != 1` and `ValidateOwnerRemap`'s bare errors) | `exitUsage` (2) | Caller-supplied config/flag is wrong; D-03's "bad flag value → 2" |
| `store.ErrNotFound` | `exitNotFound` (4) | D-03's "missing target → 4" |
| `store.ErrInvalidArgument`, `store.ErrAmbiguousShortID` | `exitUsage` (2) | Mirrors `connectError`'s own classification of these as caller-input problems |
| gRPC `codes.Unavailable` from `EnsureCollection`/any Qdrant call not wrapped in a store sentinel (raw dial failure) | `exitUnavailable` (5) | D-03's "backend unreachable → 5"; no sentinel exists today for this — see Pitfall 4 |
| `context.DeadlineExceeded` from an operator command's own `--timeout` firing | `exitUnavailable` (5) **not** the new `exitTimeout` (6) | D-06's exit 6 is scoped to the NEW client-verb single-RPC deadline (REQ-cli-request-timeout); operator `--timeout` is a pre-existing, differently-scoped whole-sweep wall-clock budget — see Pitfall 6 for why conflating these is a trap |
| auth failure (none of the six operator commands currently authenticate against the server — they call `internal/store` directly, bypassing the OIDC/Connect auth chain entirely) | N/A today | `exitAuth` (3) has no reachable operator-command path unless one is later added; do not force a mapping that has no live trigger |

**`ValidateOwnerRemap`'s three bare errors, quoted verbatim, all belong in the `exitUsage`
bucket** `[VERIFIED: internal/store/store.go:2425-2436]`:
```go
func ValidateOwnerRemap(src OwnerRemapSource, to string) error {
	if src == nil {
		return errors.New("owner remap source is required")        // line 2427
	}
	if to == "" {
		return errors.New("to must be non-empty")                   // line 2430
	}
	if f, ok := src.(remapFrom); ok && f.from == to {
		return fmt.Errorf("from and to are identical (%q)", to)      // line 2433
	}
	return nil
}
```

### Pattern 4: Timeout wiring point for every client RPC path

**What:** All three client verbs pass `cmd.Context()` directly to their single Connect RPC call
with zero wrapping, and `newHTTPClient` sets no `http.Client.Timeout`
`[VERIFIED: cmd/engram/client_list.go:49, client_search.go:51, client_store.go:58 — all three
call sites read this session and quoted; cmd/engram/client_common.go:113-119, no Timeout field
set]`.

**Recommended wiring:** derive a `context.WithTimeout(cmd.Context(), timeout)` once, alongside
`clientFromFlags`'s existing client construction, and thread the derived `ctx` (not
`cmd.Context()`) into each of the three RPC calls:
```go
// client_list.go:49 — BEFORE
resp, err := client.ListMemories(cmd.Context(), connect.NewRequest(&engramv1.ListMemoriesRequest{...}))

// AFTER — same pattern applies at client_search.go:51 and client_store.go:58
ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
defer cancel()
resp, err := client.ListMemories(ctx, connect.NewRequest(&engramv1.ListMemoriesRequest{...}))
```
This is a mechanical three-call-site change once `timeout` is resolved from the koanf client
config (D-04/D-05).

### Pattern 5: D-06's exit-6 split inside `exitCodeForConnectErr`

**What:** The existing mapper, quoted verbatim
`[VERIFIED: cmd/engram/client_common.go:292-305]`:
```go
func exitCodeForConnectErr(err error) int {
	switch connect.CodeOf(err) {
	case connect.CodeUnauthenticated, connect.CodePermissionDenied:
		return exitAuth
	case connect.CodeNotFound:
		return exitNotFound
	case connect.CodeInvalidArgument, connect.CodeFailedPrecondition, connect.CodeOutOfRange:
		return exitUsage
	case connect.CodeUnavailable, connect.CodeDeadlineExceeded, connect.CodeCanceled:
		return exitUnavailable
	default:
		return exitGeneric
	}
}
```
D-06's change is confined to one line: move `connect.CodeDeadlineExceeded` into its own case
returning the new `exitTimeout = 6`, leaving `CodeUnavailable`/`CodeCanceled` at `exitUnavailable`.
`TestCatalogExitCodesMatchMapper` self-derives its expected set from this function across every
`connect.Code(1..16)` `[VERIFIED: cmd/engram/catalog_test.go:304-322]`, so it needs no edit — the
catalog's `doc.ExitCodes` literal in `buildCatalog` (catalog.go:83-90) DOES need the new entry
added by hand, and `TestCatalogListsEveryExitCode`'s hard-coded `6`/`0-5` bounds DO need editing
(see Pitfall 3).

### Anti-Patterns to Avoid

- **String-matching cobra's `ValidateFlagGroups()` error text to detect "this came from flag-group
  validation."** All three internal validators produce fixed-format strings ("if any flags in the
  group...", "at least one of the flags in the group...")
  `[VERIFIED: cobra@v1.10.2/flag_groups.go:161,183,204]` that COULD be pattern-matched, but this
  is fragile against a future cobra upgrade changing wording, and Pattern 1 above makes it
  entirely unnecessary — do not reach for this fallback.
- **Adding client-only koanf fields' validation directly into `Config.Validate()`.** See Pitfall 7
  — this needlessly widens the D-04 test-literal tax to every existing server-side test.
- **Reusing `migrate.go`'s existing `--timeout` semantics (0=unbounded) for the new D-05 client
  `--timeout`.** These are two different concepts under one flag name across different commands —
  see Pitfall 6.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Mutually-exclusive / exactly-one-of flag enforcement | A fourth hand-rolled guard alongside `client_common.go:236` and `migrate.go:85` | `cmd.MarkFlagsMutuallyExclusive(...)` / `cmd.MarkFlagsOneRequired(...)` | This is exactly SC1's stated goal — "using cobra's declarative flag-group API rather than a fourth hand-rolled guard" |
| Per-command flag-group-error-to-exit-2 wiring | A `PreRunE` on each of the 3+ affected commands | The single `rootCmd.PersistentPreRunE` extension (Pattern 1) | One central choke point cannot be forgotten by a future command; N per-command hooks can |
| Client config resolvers (`resolveServerURL`, `--token-file`, `--output`, `--insecure`, new `--timeout`) | A `resolveTimeout` helper mirroring `resolveServerURL`'s flag→env→default pattern | `internal/config`'s existing koanf registry (D-04) | Explicitly rejected by the user during `/gsd-discuss-phase` — "expand the scope and run all client flags/settings through koanf" — CONTEXT.md `<specifics>` |

**Key insight:** Every "don't hand-roll" item in this phase is really the same insight applied
three times: this repo already has one general-purpose mechanism for each of (a) flag-shape
enforcement, (b) exit-code classification, and (c) configuration resolution — the entanglement
CONTEXT.md's `<domain>` section names is specifically about not letting a *second* mechanism for
any of these three creep in alongside the first while fixing #453/#467/#452 together.

## Common Pitfalls

### Pitfall 1: `config.CheckLegacy` is a fourth undiscovered bare-exit-1 site

**What goes wrong:** `internal/config.CheckLegacy`, called from `rootCmd.PersistentPreRunE` for
every command, returns a plain `fmt.Errorf` with no `ExitCode()` method
`[VERIFIED: internal/config/legacy.go:39-56, specifically line 54-55]`:
```go
return fmt.Errorf("retired environment variables are set and no longer read:\n%s\nRename them (see the v0.x migration notes) and retry",
    strings.Join(hits, "\n"))
```
This is not named in CONTEXT.md's `canonical_refs` — it was found by reading the file this
session.
**Why it happens:** It predates the `cliError` convention and was never revisited when
`client_common.go`'s exit-code taxonomy was introduced.
**How to avoid:** Wrap it the same way as `ValidateFlagGroups()`'s error in the extended
`PersistentPreRunE` (Pattern 1) — it is a usage error (a misconfigured environment), exit 2.
**Warning signs:** A regression test asserting on `engram <any-command>` with a `MEM_*` legacy var
set still observing exit 1 after this phase ships.

### Pitfall 2: A mistyped subcommand name bypasses `PersistentPreRunE` entirely

**What goes wrong:** `engram bogus-verb` fails inside cobra's `Find()` at
`ExecuteC()` — a call site that runs **before** `cmd.execute()` is ever invoked, so
`rootCmd.PersistentPreRunE` (and therefore Pattern 1's fix) never runs for this case
`[VERIFIED: cobra@v1.10.2/command.go:1120-1134]`:
```go
if c.TraverseChildren {
    cmd, flags, err = c.Traverse(args)
} else {
    cmd, flags, err = c.Find(args)
}
if err != nil {
    if cmd != nil {
        c = cmd
    }
    if !c.SilenceErrors {
        c.PrintErrln(c.ErrPrefix(), err.Error())
        c.PrintErrf("Run '%v --help' for usage.\n", c.CommandPath())
    }
    return c, err                                  // <-- returned straight to Execute(), still exit 1
}
```
`TestRootUnknownSubcommandStillErrors` `[VERIFIED: cmd/engram/catalog_test.go:249-261]` asserts
only that an error occurs and mentions "unknown command" — it does **not** assert on exit code
today, confirming this row is currently untested for its exit-code value (and therefore currently
falls through to the exitGeneric=1 default with nothing pinning it).
**Why it happens:** `Find()` failure is structurally outside `execute()`'s hook chain — a
different cobra code path than either `ParseFlags` or `ValidateFlagGroups`.
**How to avoid:** This is a genuine open question for the planner — see Open Questions. It cannot
be fixed by Pattern 1 or Pattern 2; fixing it (if in scope) requires either a custom `Find()`
wrapper or accepting it as a third bare-exit-1 site the D-02 backstop deliberately still covers.
**Warning signs:** A D-09 before-table row for "unknown command" that the planner assumes Pattern
1 will fix, when it structurally cannot.

### Pitfall 3: `TestCatalogListsEveryExitCode` hard-codes the count/range and WILL break on D-06

**What goes wrong:** Quoted in full `[VERIFIED: cmd/engram/catalog_test.go:218-242]`:
```go
func TestCatalogListsEveryExitCode(t *testing.T) {
	doc := decodeCatalog(t)
	if len(doc.ExitCodes) != 6 {
		t.Fatalf("len(exit_codes) = %d, want 6", len(doc.ExitCodes))
	}
	// ...
	for _, ec := range doc.ExitCodes {
		if ec.Code < 0 || ec.Code > 5 {
			t.Errorf("exit code %d is outside the 0-5 range", ec.Code)
		}
		// ...
	}
	for i := 0; i <= 5; i++ {
		if !seen[i] {
			t.Errorf("exit code %d missing from the catalog", i)
		}
	}
}
```
This is a **different** test from `TestCatalogExitCodesMatchMapper` (which self-derives and needs
no edit — see Pattern 5). This one hard-codes `6` and `0-5` as literals and will fail red the
moment `exitTimeout = 6` is added to `catalog.go`'s `doc.ExitCodes`, unless updated to `7` and
`0-6` in the same commit.
**Why it happens:** Written when the taxonomy was believed fixed at 6 codes; D-06 changes that
premise.
**How to avoid:** Update this test's literals as part of the same plan/commit that adds
`exitTimeout`, not as an afterthought discovered by a failing `task test`.
**Warning signs:** `task test` red on `TestCatalogListsEveryExitCode` after adding the exit-6
constant but before updating this test.

### Pitfall 4: `TestCatalogDocumentsFlagParseExitCode` pins the OLD split and must be replaced, not just reworded

**What goes wrong:** Quoted in full `[VERIFIED: cmd/engram/catalog_test.go:330-345]`:
```go
func TestCatalogDocumentsFlagParseExitCode(t *testing.T) {
	doc := decodeCatalog(t)
	found := false
	for _, n := range doc.Notes {
		if strings.Contains(n, "1") && strings.Contains(n, "2") &&
			(strings.Contains(n, "flag") || strings.Contains(n, "usage")) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no note documents both exit code 1 and exit code 2 alongside a mention of "+
			"flags or usage; notes=%v", doc.Notes)
	}
}
```
This test's entire *purpose* — pinning the D-17 promise that a flag-parse error exits 1, not 2 —
is retracted by D-02/D-03. A reworded note that happens to still contain both digits "1" and "2"
near the word "flag" could pass this test by accident while asserting the *opposite* of what the
old test intended (e.g., "flag errors used to exit 1, now they exit 2" would coincidentally still
match). This needs a **new**, intention-correct assertion — e.g., "no note claims a flag-shaped
failure exits 1" — not a cosmetic edit to the old one.
**Why it happens:** The test's substring-matching approach was deliberately "tolerant of exact
wording" (see its own doc comment) to survive *rewording* of the D-17 note, but was never designed
to survive the note's *retraction*.
**How to avoid:** Treat this as a test to retire and replace, not to patch.
**Warning signs:** The new note text happens to satisfy the old assertion by coincidence, giving
false confidence that D-17's retraction was verified.

### Pitfall 5: `assertExitCode` cannot be reused as-is for D-09's before-table

**What goes wrong:** The existing shared helper `[VERIFIED: cmd/engram/client_search_test.go:425-437]`:
```go
func assertExitCode(t *testing.T, err error, want int) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a non-nil error")
	}
	var ec interface{ ExitCode() int }
	if !errors.As(err, &ec) {
		t.Fatalf("error %v does not carry ExitCode()", err)   // <-- FAILS on a bare error
	}
	if got := ec.ExitCode(); got != want {
		t.Errorf("ExitCode() = %d, want %d", got, want)
	}
}
```
D-09's before-table needs rows for the currently-*untyped* sites (flag-group errors,
`config.CheckLegacy`, `migrate.go`'s `buildRemapSource`) whose current errors do NOT carry
`ExitCode()` — `assertExitCode` would `t.Fatalf` on exactly these rows instead of reporting "falls
through to 1", defeating the whole point of pinning the CURRENT behavior.
**Why it happens:** `assertExitCode` was written to test *already-typed* `cliError` paths, not to
observe the untyped fallback.
**How to avoid:** Build the before-table on `exitCodeFromError(err)` directly (root.go:75-84 —
already exported for `errors.As`-based testing per `TestExitCodeFromError`
`[VERIFIED: cmd/engram/root_test.go:26-41]`), combined with the existing `runClient(t, args...)`
test harness `[VERIFIED: cmd/engram/clienttest_test.go:142-155]` which returns cobra's raw `err`
from `rootCmd.Execute()` — `exitCodeFromError` handles both the typed and untyped case correctly
and is the exact function `Execute()` itself calls, making it the right ground truth for a
before/after pin.
**Warning signs:** The before-table plan reuses `assertExitCode` and immediately `t.Fatal`s on
every currently-untyped row, forcing an unplanned rewrite mid-plan.

### Pitfall 6: Two `--timeout` flags, same name, opposite zero-semantics, different commands

**What goes wrong:** The NEW client `--timeout` (D-04/D-05: single-RPC deadline, default 30s, `0`
rejected as usage error) shares a flag *name* with the PRE-EXISTING operator `--timeout` flags on
all 6 operator commands (whole-sweep wall-clock budget, `0` means unbounded)
`[VERIFIED: cmd/engram/migrate.go:138-139,147; reindex.go:119-120; prune.go:61-62; summarize.go:89;
backfill.go:56 — all read this session]`. Since each is a separate cobra subcommand's own
`FlagSet` (not a `PersistentFlags()` on a shared parent), there is **no Go-level naming
collision** — cobra permits this cleanly. The risk is purely a documentation/UX confusion: `engram
search --timeout 0` becomes a hard usage error under D-05, while `engram reindex --timeout 0`
continues to mean "no deadline" — same flag name, opposite meaning, depending on which command.
**Why it happens:** The operator `--timeout` flags are pre-existing (shipped before this phase)
and scoped to a fundamentally different concept (a multi-minute batch sweep's wall clock) than
REQ-cli-request-timeout's single-RPC deadline; D-04's scope (client-only flags routed through
koanf) does not reach the operator commands at all — they call `server.StoreFromEnv()` directly,
never `clientFromFlags`.
**How to avoid:** Do not rename the operator commands' existing `--timeout` flags (that would
itself be an unplanned, undecided breaking change outside D-05's stated concern). Instead,
explicitly document the divergence in both the client `--timeout` flag's `Usage` string (state
"0 is rejected" inline, since the operator commands' help text says "0 disables") and in
`guides/upgrade.md`'s D-05/D-06 entry, so a reader comparing `engram search --help` against
`engram reindex --help` is not left to infer the difference.
**Warning signs:** A user reports "`--timeout 0` used to work, now it errors" without realizing
they ran a *client verb*, not an operator command — or vice versa, a bug report that `--timeout 0`
"still hangs forever" on an operator command when that is its documented, unchanged behavior.

### Pitfall 7: The D-04 test-literal tax has a real, avoidable blast radius

**What goes wrong:** `internal/config`'s test files construct 33 full/partial `Config{}` literals
`[VERIFIED: rg count over internal/config/*_test.go — 33 occurrences of `Config{`]`. Per memory
`s780vae1vr`'s constraint (also independently present in STATE.md's carry-forward gotchas), any
NEW registry field with a non-empty `Default` (e.g. the new client `--timeout` field needing
`Default: "30s"`) that is validated *unconditionally* inside `Config.Validate()` will silently
break every hand-built `Config{}` literal in `validate_test.go` that doesn't set the new field —
Go's keyed-struct-literal zero-value ("" for string) reaches `Validate()`, not the registry's
declared default (`Load()` is what applies registry defaults; a hand-built literal bypasses
`Load()` entirely), so a previously-green `Validate()` test now accumulates an unexpected new
error.
**Why it happens:** `Config.Validate()`'s own doc comment already documents this design tension
for OTHER fields: "Optional fields..., fields validated elsewhere (OIDC/UI creds via
resolveUIConfig), and the serve-only listen address are intentionally NOT checked here"
`[VERIFIED: internal/config/validate.go:20-25]` — client fields are exactly this kind of
elsewhere-validated field (consumed by `clientFromFlags`/client verbs, never by
`server.StoreFromEnv`/`buildDepsFromEnv`'s `Config.Validate()` call chain).
**How to avoid:** Validate the new client fields (including "`--timeout` must be a positive Go
duration, 0 rejected") in a **separate** function (e.g. a client-side `validateClientConfig`
living in `cmd/engram/` or a new `internal/config/client_validate.go`), following the exact
precedent `Config.Validate()`'s own doc comment already sets for OIDC/UI — this confines any new
test-literal edits to a brand-new test file with zero pre-existing literals, rather than touching
all 33 existing ones.
**Warning signs:** `go test ./internal/config/... -v` showing new, unrelated failures in
`TestValidateRejectsBadSummaryMaxCharsWhenEnabled`-style tests after adding the client timeout
field — a sign the field was wired into the shared `Validate()` rather than kept separate.

## Code Examples

### D-07's exactly-one-of pairing for `migrate-remap-owner`

```go
// Source: this repo's existing buildRemapSource (migrate.go:73-100), read this session.
// D-07 replaces the "selected != 1" counting with MarkFlagsMutuallyExclusive +
// MarkFlagsOneRequired declared at command-construction time (init()):
migrateRemapOwnerCmd.MarkFlagsMutuallyExclusive("from", "from-missing", "from-anon")
migrateRemapOwnerCmd.MarkFlagsOneRequired("from", "from-missing", "from-anon")
// buildRemapSource keeps its store.ValidateOwnerRemap(src, to) call and stays pure/testable;
// it loses only the `selected := 0; ...; if selected != 1` block (lines 74-86), since cobra
// now guarantees exactly one of the three is set before RunE is ever reached.
```

### D-07's paging trio — a single group closes D-08 too

```go
// Source: this repo's client_list.go:93-106, read this session.
// One MarkFlagsMutuallyExclusive call covering all three flags means ANY two of
// {offset, cursor-mode, page-token} passed together is rejected — this is the SAME
// mechanism that closes D-08 ("--page-token together with --offset becomes an error"),
// not a separate guard:
listCmd.Flags().Uint64Var(&listOffset, "offset", 0, "...")
listCmd.Flags().BoolVar(&listCursorMode, "cursor-mode", false, "...")
listCmd.Flags().StringVar(&listPageToken, "page-token", "", "...")
listCmd.MarkFlagsMutuallyExclusive("offset", "cursor-mode", "page-token")
```

### D-07's `--scope`/`--cross-spine` — declared per-command, guard's fate is discretionary

```go
// Source: client_common.go:220-242 (validateScopeCrossSpine) and client_search.go:82-85 /
// client_list.go:93-96, read this session. D-07 requires declaring the group on EACH
// command (searchCmd and listCmd both), since MarkFlagsMutuallyExclusive is a per-Command
// method, not shared the way validateScopeCrossSpine's Go function currently is:
searchCmd.MarkFlagsMutuallyExclusive("scope", "cross-spine")
listCmd.MarkFlagsMutuallyExclusive("scope", "cross-spine")
// validateScopeCrossSpine's SECOND branch (scope required unless cross-spine) has no cobra
// flag-group equivalent — MarkFlagsOneRequired only expresses "at least one of N", it cannot
// express "scope is required UNLESS cross-spine is true" as a single declarative rule, since
// that is not a symmetric one-of relationship (crossSpine=false, scope=""  is the ONLY
// rejected row; crossSpine=true is legal even with scope=""). validateScopeCrossSpine's own
// "!crossSpine && scope == ''" branch (client_common.go:238-240) must stay as hand-written
// code regardless of the mutual-exclusivity conversion — cobra's declarative API does not
// cover this asymmetric rule. This is exactly what CONTEXT.md's discretion item ("whether the
// client_common.go:236 shared guard is retained as a defense-in-depth backstop... or removed
// outright") is about: only HALF of validateScopeCrossSpine (the symmetric exclusivity half)
// is replaceable by cobra; the other half is not.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Hand-rolled mutual-exclusivity guards (`client_common.go:236`, `migrate.go:73-86`) or no enforcement at all (`client_list.go`'s paging trio) | cobra's declarative `MarkFlagsMutuallyExclusive`/`MarkFlagsOneRequired`, available since cobra v1.5.0 and unchanged in shape through v1.10.2 `[VERIFIED: cobra@v1.10.2/flag_groups.go]` | Already available in the pinned cobra version — no upgrade needed | This phase is adopting an existing, already-vendored capability, not migrating to a new one |
| Per-setting `os.Getenv`-based client resolvers (`resolveServerURL`) | `internal/config`'s koanf registry, already the server-side pattern | Already the pattern for every `ENGRAM_*` server var; D-04 extends it to client vars | No new pattern invented — this closes the one place client config diverged from the rest of the binary |

**Deprecated/outdated:** None — this phase is closing a gap between two already-current patterns
(cobra flag groups, koanf config), not migrating away from something obsolete.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `p.PersistentPreRunE(c, argWoFlags)` receives the leaf command `c` (not `p`) as its first argument, so extending `rootCmd.PersistentPreRunE` to call `cmd.ValidateFlagGroups()` validates the *running* subcommand's flag groups, not root's own | Pattern 1 | If wrong, Pattern 1's central mechanism would validate the wrong command's flags and silently no-op for every subcommand invocation — however this was read directly from cobra's source (`command.go:986`, the call site passes `c` not `p`), so this is `[VERIFIED]`, not assumed; listed here only because it is the single most load-bearing inference in this research and merits explicit flagging for the planner's own re-verification during implementation |
| A2 | No live trigger exists today for `exitAuth` (3) from any of the six operator commands, since none of them authenticate against the server (they call `internal/store` directly) | Pattern 3 | If a future auth-gated operator path is added, the classification table's "N/A today" row would need revisiting — low risk, since it is a documented absence, not a forced mapping |
| A3 | `defaultTraverseRunHooks = false` is the value in effect for this binary (i.e., nothing in this codebase sets `cobra.EnableTraverseRunHooks = true` at init time) | Pattern 1, Summary | `[VERIFIED: cobra@v1.10.2/cobra.go:49]` for the *default*; not independently re-grepped across this repo's own `init()` functions for an override — if this repo does set it true somewhere, root's `PersistentPreRunE` would still run (traversal order only affects whether OTHER ancestors' hooks also run), so Pattern 1 remains correct either way, but the "root is the *only* hook that fires" framing in the Summary would need softening |

## Open Questions

1. **Is a mistyped subcommand name (`engram bogus-verb`) in scope for D-02/D-03's reclassification?**
   - What we know: it currently falls through to exit 1 via cobra's `Find()` failure path, which
     structurally bypasses `PersistentPreRunE` (and therefore Pattern 1's fix) entirely — see
     Pitfall 2. `TestRootUnknownSubcommandStillErrors` does not currently pin its exit code.
   - What's unclear: SC2's language ("no path falls through to a bare, undocumented exit 1") reads
     broadly enough to arguably include this, but CONTEXT.md's `<domain>` section frames the phase
     boundary as "flag conflicts, configuration, timeouts, and errors" — a mistyped verb is a
     distinct failure class from a flag conflict, and D-03's per-command classification language
     is scoped to the SIX NAMED operator commands plus the flag-group sites, not cobra's
     command-resolution layer.
   - Recommendation: treat as explicitly out of scope for this phase unless the planner decides
     otherwise — but if left out of scope, add one line to `catalog.go`'s notes or
     `guides/cli.md` naming this as a third documented exit-1 case (alongside the D-02 backstop),
     so it does not read as an oversight later. If in scope, fixing it needs a custom wrapper
     around `Find()`/`ExecuteC()`, not Pattern 1 or Pattern 2.

2. **Does `serve`'s `httpSrv.ListenAndServe()` failure (e.g. port already bound) fit anywhere in
   the 2/3/4/5/6 taxonomy, or does it stay on the exitGeneric=1 backstop?**
   - What we know: `serve`'s own pre-flight errors (empty listen addr, invalid OIDC/UI config,
     malformed cookie key) are cleanly `exitUsage` (2) — they are config-validation failures.
     `ListenAndServe()` itself failing for a reason like "address already in use" is a *local OS
     resource* failure, not cleanly "backend unreachable" (5, which in this taxonomy has meant
     "the REMOTE server/Qdrant is unreachable" everywhere else) nor any of the other four codes.
   - What's unclear: whether D-03's "full classification" for `serve` is meant to reach this deep
     (every conceivable startup failure) or whether `serve`'s scope is "the config/auth-guard
     errors that already return early," leaving genuinely unexpected OS-level failures on the
     D-02 backstop by design.
   - Recommendation: classify `serve`'s pre-flight config/auth-guard errors as `exitUsage` (2)
     confidently (D-03's "bad flag value → 2" clearly covers these); leave `ListenAndServe()`'s
     own OS-level failure on the `exitGeneric` (1) backstop unless the planner has a specific
     reason to invent a mapping — this is consistent with D-02's framing of exit 1 as "a genuinely
     unclassified Go error... degrades loudly rather than being misfiled."

3. **What is the exact release version this phase's `guides/upgrade.md` section should be titled?**
   - What we know: the current shipped version is `0.12.0`
     `[VERIFIED: .release-please-manifest.json:2, charts/engram/Chart.yaml version/appVersion]`,
     and STATE.md records this milestone's release-please treatment as `bump-minor-pre-major`
     `[CITED: .planning/STATE.md — "Release-please treatment: bump-minor-pre-major, not a major"]`.
   - What's unclear: release-please computes the actual next version automatically from commit
     messages at merge time; the plan should not hard-code "v0.13.0" as a heading if that value
     could drift before release.
   - Recommendation: follow `upgrade.md`'s existing section-heading convention (`## vX.Y.Z — Title`)
     but treat the exact version number as filled in at release time, or use a placeholder the
     release process reconciles — do not hand-author a version number that might not match what
     release-please actually cuts.

## Environment Availability

Skipped — this phase adds no new external tool, service, or runtime dependency. The Go toolchain,
`cobra`, and `koanf` are already present and pinned in `go.mod` (verified above). The six operator
commands' existing dependency on a reachable Qdrant instance and (for `summarize-missing`) an
OpenAI-compatible chat endpoint is pre-existing infrastructure this phase does not add to or
change — it only changes how errors *from* those dependencies are classified into exit codes.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package (`go test`) `[VERIFIED: Taskfile.yaml:38-40]` |
| Config file | none — no `.testconfig`; Go's built-in test runner |
| Quick run command | `go test ./cmd/engram/... -run '<TestName>' -v` (or `./internal/config/...` /
  `./internal/store/...` for those packages) |
| Full suite command | `task test` (runs `go test ./...` plus the Python skill-hook suite) `[VERIFIED: Taskfile.yaml:35-44]` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-flag-exclusivity-enforced | `client_list.go` paging trio, `--scope`/`--cross-spine` on search+list, migrate's tri-state source all reject before dial | unit | `go test ./cmd/engram/... -run TestFlagGroup -v` (new) | ❌ Wave 0 — no flag-group-specific test file exists today; closest precedent is `client_common_test.go`'s `TestValidateScopeCrossSpineParity` |
| REQ-exit-code-unified | Every command's error resolves to 0/2/3/4/5/6, no bare 1 for a classified path | unit | `go test ./cmd/engram/... -run TestExitCode -v` (new, plus updates to `TestCatalogListsEveryExitCode`/`TestCatalogExitCodesMatchMapper`) | ❌ Wave 0 for the new operator-classification tests — ✅ existing `TestExitCodeFromError` (root_test.go:29), `TestCatalogExitCodesMatchMapper` (catalog_test.go:304) as scaffolding |
| REQ-exit-code-migration-safe | Before-table pins current exit code per command × failure mode, before any behavior change | unit (table-driven) | `go test ./cmd/engram/... -run TestExitCodeBaseline -v` (new file, e.g. `exitcode_baseline_test.go`) | ❌ Wave 0 — must be authored as its OWN plan/commit landing green against unchanged code (D-09), built on `exitCodeFromError` + `runClient`, NOT `assertExitCode` (Pitfall 5) |
| REQ-cli-request-timeout | A hung/half-open server returns within `--timeout`, exits `exitTimeout`(6) | unit + integration | `go test ./cmd/engram/... -run TestTimeout -v` (new); a true "hung server" integration case likely needs an `httptest.Server` that never writes a response, or a TCP listener that accepts and never responds | ❌ Wave 0 |
| REQ-client-config-unified | `--server`/`--token-file`/`--output`/`--insecure`/`--timeout` all resolve via koanf, no `os.Getenv` resolver remains in `cmd/engram/` | unit | `go test ./internal/config/... -run TestClientConfig -v` (new) plus a `rg -n "os.Getenv" cmd/engram/*.go` negative-check (not itself a Go test, but worth a CI-visible assertion) | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** the relevant package's quick-run command above.
- **Per wave merge:** `task test` (full suite).
- **Phase gate:** full suite green before `/gsd-verify-work`, plus `task lint` (this repo's
  `golangci-lint`/`yamlfmt`/`actionlint`/`rumdl` gate, unaffected by this phase's Go-only changes
  but still part of `task`'s default target `[VERIFIED: Taskfile.yaml:14-18]`).

### Wave 0 Gaps

- [ ] `cmd/engram/exitcode_baseline_test.go` (or similar) — D-09's before-table, its own
      plan/commit, landing green against UNCHANGED code (must be authored and merged before any
      classification change lands)
- [ ] A flag-group-specific test file exercising all three D-07 sites plus D-08's page-token/offset
      case
- [ ] A `TestOperatorCommandExitCodes`-style table covering the 6 operator commands' classified
      error paths (config error → 2, not-found → 4, unavailable → 5)
- [ ] A timeout integration test harness (hung/never-responding server) for REQ-cli-request-timeout
- [ ] Updates (not new files) to `TestCatalogListsEveryExitCode` (Pitfall 3) and
      `TestCatalogDocumentsFlagParseExitCode` (Pitfall 4) — both existing, both currently green,
      both must change as part of this phase's own commits

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | This phase does not touch the OIDC bearer-token verifier or any auth lane; `exitAuth`(3) is preserved unchanged (D-01), not re-implemented |
| V3 Session Management | no | No session/cookie code touched |
| V4 Access Control | no | No authz/owner-scoping code touched; operator commands' existing Subject-less-tier posture is unchanged by this phase |
| V5 Input Validation | yes | Flag-group exclusivity IS input validation — cobra's declarative API (`MarkFlagsMutuallyExclusive`/`MarkFlagsOneRequired`) is the standard control, replacing ad-hoc/absent hand-rolled guards; the `--timeout` value itself also needs input validation (reject 0, reject negative) per D-05 |
| V6 Cryptography | no | No crypto code touched; `--insecure`'s existing `InsecureSkipVerify` gate (client_common.go:113-119) is unchanged by this phase, only its *resolution mechanism* moves to koanf |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| A hung/malicious/half-open server holds a client connection open indefinitely (resource exhaustion on the calling side — an agent's own process or CI runner blocked forever) | Denial of Service | REQ-cli-request-timeout's `context.WithTimeout` deadline (D-05/D-06) — this phase's own deliverable |
| A caller-supplied flag combination that the interface documents as invalid but does not enforce is silently accepted and forwarded, producing a query different from what the caller believes they asked for | Tampering (of intent, not data) — the D-08 "`--page-token` ignores `--offset`" case | Reject before dial (exit 2), never silently drop a caller-supplied value — D-08's explicit rationale |
| An exit-code contract change breaks an unreviewed external consumer's automation (e.g. a script that branches `if [ $? -eq 1 ]`) without warning | Repudiation-adjacent / availability of the *consumer's* automation, not this binary | D-09's before-table + D-10's consumer sweep + `guides/upgrade.md` entry — this phase's own deliverable; verified D-10 sweep below found zero in-repo consumers branching on a *specific* exit-code value (only zero/nonzero via Kubernetes CronJob semantics), narrowing the real blast radius to documentation |

## D-10 Consumer Sweep (verified, complete)

Every location named in CONTEXT.md's D-10 was read this session:

| Location | Finding |
|----------|---------|
| `Taskfile.yaml` | Only reference to the binary is the build step itself (`go build -trimpath -o bin/engram ./cmd/engram`, line 23); no task invokes a client verb or operator command and branches on its exit status `[VERIFIED: full grep of "bin/engram" and "./engram" across Taskfile.yaml — one hit, the build line]` |
| `.github/workflows/*.yaml` (`ci.yaml`, `docs-site.yaml`, `release.yaml`) | No invocation of the built `engram` binary found in any workflow file `[VERIFIED: grep across all three files for the binary path and every client/operator verb name — zero hits]` |
| `charts/engram/templates/summarize-cronjob.yaml` | Invokes `["summarize-missing", "--all-scopes"]` as a CronJob container's `args` `[VERIFIED: full file read, line 23]`. Kubernetes CronJob failure semantics only distinguish zero vs nonzero exit (governs `restartPolicy`/`failedJobsHistoryLimit`); it does not branch on which specific nonzero code is returned. Changing `summarize-missing`'s classified exit code (from today's unclassified 1 to a D-03-assigned 2/4/5) requires **no chart change** — this consumer is exit-code-value-agnostic by construction. |
| `charts/engram/values.yaml`, `Chart.yaml`, `templates/_helpers.tpl` | Contain unrelated `ln`-branded strings (a separate, apparently in-progress rename artifact — `name: ln`, `repository: ghcr.io/seanb4t/ln`) `[VERIFIED: read this session]`. Not exit-code-related; noted only because it surfaced during the sweep — **out of scope for this phase**, flagged here so it is not mistaken for something this phase must reconcile. |
| `skill/engram/hooks/` (`session-start-memory-recall`, `posttooluse-memory-capture-nudge`) | Both hooks talk to the MCP server directly (HTTP/MCP protocol) and never shell out to the `engram` CLI binary; `session-start-memory-recall` always exits 0 itself regardless of the memory server's own auth-failure state `[VERIFIED: grep for "subprocess"/"engram "/"exit"/"returncode" across the hooks directory — no CLI-binary subprocess invocation found]`. Zero exit-code coupling. |
| `docs-site/src/content/docs/guides/cli.md` | The canonical, human-readable exit-code table (lines 90-99) plus the D-17 "flag typo exits 1, not 2" caution box (lines 101-110) — this IS the documentation surface D-03/D-06 must rewrite; quoted and read in full this session. |
| `docs-site/src/content/docs/guides/upgrade.md` | Existing precedent for how a breaking-change section is structured (six numbered sub-sections under `## v0.12.0`, each with "Who should act" framing) `[VERIFIED: full file read]` — this phase's D-03/D-06/D-08 entries should follow the same structure under a new version heading (see Open Question 3 for why the exact version number is left to release time). |
| `docs-site/src/content/docs/reference/errors.md` | Cross-references `cli.md`'s exit-code table by link (`[CLI guide's exit-code table](/guides/cli/#exit-codes)`, lines 90-93,122) rather than duplicating the table — no independent edit needed here beyond what `cli.md` already drives, confirmed by reading both files. |

**Conclusion:** the in-repo sweep found **zero** consumers that branch on a *specific* exit-code
value — only `docs-site/` documents the taxonomy in prose, and only Kubernetes' generic
zero/nonzero CronJob semantics touch it operationally. D-10's "audit of known consumers" is
therefore primarily a **documentation** deliverable (`guides/cli.md` + `guides/upgrade.md`) rather
than a code-fixing sweep across `Taskfile.yaml`/`.github/workflows/`/`charts/`/`skill/` — none of
those needed a behavioral change as a *result* of this audit, only confirmation that none of them
needed one.

## Sources

### Primary (HIGH confidence — read directly from the pinned dependency source or this repo's own source this session)

- `cobra@v1.10.2` module cache (`/Users/sean/go/pkg/mod/github.com/spf13/cobra@v1.10.2/`) —
  `command.go` (execute/ExecuteC control flow, `SetFlagErrorFunc`/`FlagErrorFunc`,
  `EnableTraverseRunHooks` default), `flag_groups.go` (`MarkFlagsMutuallyExclusive`/
  `MarkFlagsOneRequired`/`ValidateFlagGroups` and their three internal error-format functions),
  `cobra.go` (`defaultTraverseRunHooks`) — read in full this session against the exact version
  pinned in `go.mod:20`.
- This repo's own source, read this session: `cmd/engram/{client_common,root,catalog,client_list,
  client_search,migrate,reindex,prune,summarize,backfill,serve,main}.go` and their `_test.go`
  siblings; `internal/config/{config,registry,validate,legacy}.go`; `internal/store/store.go`
  (sentinel errors, `ValidateOwnerRemap`); `internal/server/{connecterror,tools}.go`
  (`connectError`, `StoreFromEnv`/`StoreAndEmbedderFromEnvNoEnsure`/`StoreAndSummarizerFromEnv`);
  `docs-site/src/content/docs/guides/{cli,upgrade}.md`;
  `docs-site/src/content/docs/reference/errors.md`; `charts/engram/templates/
  summarize-cronjob.yaml`; `charts/engram/{values.yaml,Chart.yaml}`; `Taskfile.yaml`;
  `skill/engram/hooks/session-start-memory-recall`; `go.mod`; `.release-please-manifest.json`.

### Secondary (MEDIUM confidence)

None used — every claim in this document is either read from the pinned source this session or
carried forward verbatim from CONTEXT.md/STATE.md/REQUIREMENTS.md as an explicit, tagged
`[CITED: ...]` quote.

### Tertiary (LOW confidence)

None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; every version claim read from `go.mod`
- Architecture (flag-group interception mechanism, exit-6 split, operator classification model):
  HIGH — every mechanism claim is read directly from the pinned cobra source or this repo's own
  source this session, not recalled from training data
- Pitfalls: HIGH for the 7 documented above (all read from source with exact line numbers); the
  two Open Questions are explicitly flagged as genuinely undecided rather than guessed at

**Research date:** 2026-08-03
**Valid until:** Stable for the life of this phase's implementation — re-verify only if
`go.mod`'s `cobra`/`connectrpc.com/connect`/`koanf` pins change before this phase ships (30-day
estimate for a fast-moving Go dependency set is conservative here since none of the three has a
pending major-version bump on this project's roadmap).
