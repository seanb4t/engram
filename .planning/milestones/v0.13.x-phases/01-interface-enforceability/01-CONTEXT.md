# Phase 1: Interface Enforceability - Context

**Gathered:** 2026-08-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Every `engram` CLI invocation — client verb or operator command alike — resolves flag conflicts,
configuration, timeouts, and errors through one predictable, migration-safe contract.

This phase resolves the load-bearing entanglement between #453 and #467 in a single pass: cobra's
`MarkFlagsMutuallyExclusive` raises a plain `fmt.Errorf` that bypasses `cliError`/`ExitCode()` and
falls through to exit 1, so adopting #453 without #467 would reintroduce, one command over, the
exact undocumented exit-code split #467 exists to close.

**Roadmapped requirements:** REQ-flag-exclusivity-enforced, REQ-exit-code-unified,
REQ-exit-code-migration-safe, REQ-cli-request-timeout.

**⚠ Scope expansion accepted during discussion (D-04):** the user widened this phase to route *all*
client flags/settings through koanf, not only the new timeout. This is broader than the four
requirements above. `.planning/ROADMAP.md` and `.planning/REQUIREMENTS.md` must be updated through
`/gsd-phase` — **never** by hand (rule `8dfdhfs5nn`, extended to body granularity by memory
`apfg4fe199`). Planning should not proceed on the assumption that the roadmap already reflects this.

</domain>

<decisions>
## Implementation Decisions

### Exit-code taxonomy

- **D-01:** `exitAuth = 3` **survives** the unification. The success criterion's "0/2/4/5" names the
  codes the unification moves things *into*, not an exhaustive taxonomy. `exitAuth` is already
  distinct, already pinned by `TestClientSearchExitCodeAuth`, already advertised in `catalog.go`, and
  reachable only through the D-10 mapper. Collapsing it would lose a distinction callers act on
  (re-auth vs fix-your-flags vs retry-later) and would itself be a gratuitous breaking change.
  — **Reversibility:** one-way — the exit-code set is a published contract advertised by `engram
  catalog`; removing a code later breaks any consumer branching on it and needs its own upgrade note.

- **D-02:** `exitGeneric = 1` is **kept but redefined** as an unreachable-by-design internal-error
  backstop. Every classified path becomes typed (framework flag errors → 2, operator command failures
  → 2/3/4/5), so nothing routes to 1 deliberately; 1 survives only as `root.go`'s `errors.As` default
  for a genuinely unclassified Go error, documented in `catalog.go` as exactly that. This satisfies
  "no bare, undocumented exit 1" without pretending an untyped error is impossible — a future untyped
  error degrades loudly rather than being misfiled as a usage error.
  — **Reversibility:** costly — undoing means re-auditing every typed error path added by this phase.

- **D-03:** The six operator commands (`serve`, `reindex`, `prune-expired`, `migrate-remap-owner`,
  `summarize-missing`, `backfill-short-ids`) get **full classification** in the same 2/3/4/5
  vocabulary as client verbs: bad flag value → 2, backend unreachable → 5, auth failure → 3, missing
  target → 4. This is what "one taxonomy" actually means — a caller scripting `reindex` gets the same
  contract as one scripting `engram search`. Each command's error paths must be audited individually;
  `buildRemapSource`-style pure validators make the 2-cases straightforward, while store/Qdrant errors
  need explicit classification (they are not Connect errors, so the D-10 mapper does not reach them).
  — **Reversibility:** one-way — this is the breaking change #467 exists to make; `guides/upgrade.md`
  names every affected command and consumers will have adapted.

### Client configuration (scope expansion)

- **D-04:** **All** client flags/settings route through koanf, and this lands **in Phase 1**.
  `--timeout`/`ENGRAM_TIMEOUT` is the first field declared the new way rather than a fourth
  hand-rolled resolver alongside `resolveServerURL`, `--token-file`, `--output`, and `--insecure`.
  Rationale: strongest coherence with the phase goal (one contract, declared where enforced), and it
  avoids building a `resolveTimeout` helper this phase only to delete it next. Accepted cost: Phase 1
  now carries a breaking exit-code change *and* a client-wide config refactor.
  Constraint from memory `s780vae1vr`: every new required-with-a-default `internal/config` registry
  field must be added to **every** full `Config{}` literal in that package's tests, or previously-green
  tests fail on the empty-string zero value.
  — **Reversibility:** costly — undo touches every client verb's entry path plus `internal/config`
  and its test literals.

- **D-05:** Default timeout is **30s**; a value of **0 is rejected as a usage error** (exit 2), not
  treated as unbounded. REQ-cli-request-timeout demands a *finite* deadline, so no value may mean
  unbounded. This forces a conscious reconciliation with `migrate-remap-owner`'s existing
  `--timeout` where `0` currently *does* mean unbounded (`migrate.go:65,116`) — the binary must not
  ship two `--timeout` flags with opposite semantics.
  — **Reversibility:** reversible — a default value and a validation rule.

- **D-06:** A timeout reports a **new dedicated exit code 6**. This distinguishes "server never
  answered in time" from "couldn't connect at all" (5), which a caller may want to treat differently
  (raise the timeout vs check the server is up). Consequences: a new constant, a new `catalog.go`
  entry (gated by `TestCatalogExitCodesMatchMapper`, which builds the advertised list from the
  constants and never from a second literal), a `guides/upgrade.md` line, and a consumer audit that
  covers the **new** code as well as the changed ones.
  — **Reversibility:** one-way — same published-contract argument as D-01.

### Flag-group enforcement

- **D-07:** **All three** exclusivity claim sites convert to cobra's declarative API, eliminating the
  three-tier enforcement condition documented in memory `5kqrs63zte`:
  - `client_list.go:98-106` paging trio (`--offset`/`--cursor-mode`/`--page-token`) — currently
    enforced *nowhere*, pure help-text fiction → `MarkFlagsMutuallyExclusive`.
  - `client_common.go:236` (`--scope`/`--cross-spine`) — a guard *shared* across `search` and `list`,
    so conversion means declaring the group on each command.
  - `migrate.go:73-86` (`--from`/`--from-missing`/`--from-anon`) — needs **exactly**-one, so
    `MarkFlagsMutuallyExclusive` alone is insufficient (it permits zero); pair it with
    `MarkFlagsOneRequired`. `buildRemapSource` keeps its `store.ValidateOwnerRemap` call and stays
    pure and unit-testable, losing only the `selected != 1` counting.
  — **Reversibility:** reversible — declarative wiring, local to command construction.

- **D-08:** `--page-token` together with `--offset` becomes an **error**, not a silent ignore. The
  help text currently makes two different claims — `--offset` is "mutually exclusive with
  `--cursor-mode`", but `--page-token` "ignores `--offset`". Silently ignoring a flag the caller
  explicitly passed is the same class of defect as an unenforced exclusivity claim: the interface
  accepts input it does not honor. Rejecting before any dial (exit 2) makes the paging model
  correct-by-reading — which is exactly what Phase 2 codifies. This breaks any caller passing both and
  relying on the ignore, so it joins the `guides/upgrade.md` migration notes.
  — **Reversibility:** one-way — a previously-accepted invocation now fails; it is a documented
  breaking change in the upgrade guide.

### Migration proof

- **D-09:** The before-table lands as **its own plan, committed before any behavior change**. It is a
  table-driven test enumerating every command × failure mode with its **current** exit code, landing
  green against unchanged code — so the commit itself is the proof the baseline was *observed*, not
  reconstructed. Later plans flip expected values row by row, and git history shows exactly which rows
  moved. This is the only considered option where "before" is verifiable by a third party after the
  fact. Ordering constraint: a table written *after* the change is not a regression test, it is a
  transcription of whatever the new code happens to do, and would pass just as happily on a broken
  unification. Per memory `nczgrtfec2`, assert the before/after codes are **distinct where claimed to
  change** and **identical where claimed not to** — a loose "codes are as expected" assertion passes
  while classification silently collapses.
  — **Reversibility:** reversible — a test-ordering discipline, but it cannot be recovered once the
  behavior change has landed, so it must be respected at plan-sequencing time.

- **D-10:** "An audit of known consumers" is closed by an **in-repo sweep plus a documented statement
  of external posture**. Sweep every in-repo caller of the binary — `Taskfile.yaml`,
  `.github/workflows/`, `charts/engram/`, `skill/engram/`, `docs-site/` examples — fix anything that
  branches on exit status, and record in `guides/upgrade.md` that external consumers are addressed by
  the guide itself, since engram is self-hosted with no telemetry. This names what *was* checked rather
  than implying a survey of users who cannot be enumerated.
  — **Reversibility:** reversible.

### Claude's Discretion

- How cobra's flag-group validation errors are intercepted and typed to exit 2 (e.g. `SetFlagErrorFunc`
  on root vs central classification in `Execute()`). The *outcome* is fixed by D-02/D-03; the mechanism
  is the planner's call.
- The precise rewording of the D-17 note in `catalog.go:92-98` — it currently tells callers that
  framework flag-parsing errors exit "1, not 2" and that they should "not assume every usage-shaped
  failure exits 2." That published promise is **retracted** by D-02/D-03; how the replacement note is
  phrased is discretionary.
- The shape of the client koanf config struct introduced by D-04.
- Whether the `client_common.go:236` shared guard is retained as a defense-in-depth backstop after
  cobra takes over the declaration, or removed outright.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Exit-code taxonomy and error plumbing
- `cmd/engram/client_common.go` §195-201 — the six `exitOK`/`exitGeneric`/`exitUsage`/`exitAuth`/
  `exitNotFound`/`exitUnavailable` constants; §203-217 `cliError` + `usageErrorf`; §276-303
  `exitCodeForConnectErr`, the single D-10 Connect-code→exit-code mapper.
- `cmd/engram/root.go` §59-79 — `Execute()` and `exitCodeFromError`; the `errors.As` on
  `ExitCode() int` defaulting to 1, with the comment documenting D-09's deliberate fall-through for
  all six operator commands.
- `cmd/engram/catalog.go` §78-101 — `doc.ExitCodes` built from the constants (never a second literal
  list), gated by `TestCatalogExitCodesMatchMapper`; and `doc.Notes`, which carries the **D-17**
  published promise that framework flag errors exit 1 not 2. D-02/D-03 retract that note.
- `docs-site/src/content/docs/reference/errors.md` — the `field=<name> hint=<code>` error envelope
  reference; the exit-code taxonomy documentation must stay consistent with it.

### Flag-exclusivity claim sites
- `cmd/engram/client_list.go` §94-106 — the paging trio and the `--scope`/`--cross-spine` claim; the
  trio is the only claim currently enforced nowhere.
- `cmd/engram/client_common.go` §236 — the hand-rolled `--scope`/`--cross-spine` guard shared by
  `search` and `list`.
- `cmd/engram/migrate.go` §68-101 — `buildRemapSource`, the exactly-one-of tri-state validator,
  deliberately pure and unit-testable; §85 raises a plain `fmt.Errorf` (a bare-exit-1 site).
- `cmd/engram/client_search.go` §83-85 — the `--scope`/`--cross-spine` claim on the search side.

### Timeout and client configuration
- `cmd/engram/client_common.go` §113-115 — `newHTTPClient`, which sets **no** `Timeout`; there is no
  `context.WithTimeout` on any client path, so this is greenfield.
- `cmd/engram/client_common.go` §46-60 — `resolveServerURL`, the existing flag → `os.Getenv` →
  `usageErrorf` client pattern that D-04 replaces with koanf.
- `cmd/engram/migrate.go` §65, §116-120 — `migrate-remap-owner`'s existing `--timeout time.Duration`
  where `0` means unbounded; must be reconciled with D-05.
- `internal/config/config.go`, `internal/config/validate.go` — the koanf registry and its validation
  pattern (see `Embed.Timeout` / `Summarize.Timeout` for the duration-field precedent).

### Migration and requirements
- `docs-site/src/content/docs/guides/upgrade.md` — the migration-notes target for D-03, D-06, D-08.
- `.planning/REQUIREMENTS.md` §29-44 — the four Interface Enforceability requirements verbatim.
- `.planning/ROADMAP.md` §190-219 — the phase goal, dependencies, and four success criteria.

### Related GitHub issues
- #453 (flag exclusivity), #467 (exit-code unification), #452 (CLI request timeout).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cliError` + `usageErrorf` (`client_common.go:203-217`): the existing typed-error carrier. Operator
  commands' D-03 classification should reuse this rather than inventing a parallel mechanism.
- `exitCodeForConnectErr` (`client_common.go:292`): the single D-10 mapper. Reusable for any operator
  path that *does* speak Connect, but note most operator errors come from the store/Qdrant directly
  and are not Connect errors — they need explicit classification, not this mapper.
- `TestCatalogExitCodesMatchMapper`: gates that the advertised taxonomy is built from the constants.
  It is an ally for D-01/D-02/D-06 — retire or add a constant and the catalog cannot silently drift.
- `assertExitCode` (`client_search_test.go:425`): the shared exit-code test helper; the D-09
  before-table can build on it.
- `store.ValidateOwnerRemap` (called from `buildRemapSource`): survives D-07 unchanged.
- `internal/config`'s `Embed.Timeout`/`Summarize.Timeout` duration fields: the precedent for how D-04
  should declare and validate `ENGRAM_TIMEOUT`.

### Established Patterns
- **Exit codes are advertised, not incidental.** `engram catalog` publishes the taxonomy and a test
  enforces it matches the mapper. Any change to the code set is a contract change.
- **Client config is currently ad-hoc.** Each client setting has its own resolver and its own
  `usageErrorf`; `internal/config` is imported by `root.go` (server side) only. D-04 changes this.
- **Pure, unit-testable validators before I/O.** `buildRemapSource` is explicitly documented as pure
  "so both fail fast before opening a Qdrant connection." Preserve this property under D-07.
- **Config registry fields carry a test-literal tax** (`s780vae1vr`): every new
  required-with-a-default field must be added to every full `Config{}` literal in the package's tests.

### Integration Points
- `root.go:Execute()` — where every command's error becomes a process exit status; the single point
  D-02's "unreachable-by-design backstop" is defined.
- Cobra command construction in each `client_*.go` / operator command file — where D-07's flag groups
  are declared.
- `newHTTPClient` / the Connect client construction path — where D-05's deadline is applied.
- `catalog.go:buildCatalog` — where D-06's new code 6 must appear.
- `guides/upgrade.md` — where D-03, D-06, and D-08's breaking changes are named.

</code_context>

<specifics>
## Specific Ideas

- The user explicitly rejected the offered flag → raw-env → default helper pattern for the timeout,
  asking instead to "expand the scope and run all client flags/settings through koanf" — and then
  placed that work *inside* Phase 1 rather than deferring it. Treat D-04 as a deliberate,
  reaffirmed widening, not an accident.
- Prior decision carried forward (memory `cwhfygc4t6`, 2026-08-03): #467 resolves by **unifying** the
  exit-code taxonomy across all six operator-command sites — explicitly *against* the v0.13.x research
  recommendation to merely document the boundary. This makes it a breaking change and makes #453 and
  #467 one phase. Release-please treatment: `bump-minor-pre-major`, not a major.

</specifics>

<deferred>
## Deferred Ideas

- None from scope creep — every expansion raised during discussion (the koanf client-config
  unification) was deliberately placed *into* this phase rather than deferred. See D-04.

### Reviewed Todos (not folded)
- **Rotate the Cloudflare API token for docs-site deploy** (`tooling`, match score 0.6) — not folded.
  Matched on the keywords api/token/every; it is docs-site deploy tooling, unrelated to CLI
  interface enforceability.
- **Resolve stale `docs/v0.12.x-phase-01-context` branch** (`planning`, match score 0.2) — not folded.
  Matched on the word "phase" alone. Worth noting that this phase creates a v0.13.x phase-01 context
  and the old branch name is confusingly similar, but the cleanup is not this phase's work.

</deferred>

---

*Phase: 1-Interface Enforceability*
*Context gathered: 2026-08-03*
