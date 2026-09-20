# Phase 5: Operator Sweeps & CI Backstop - Context

**Gathered:** 2026-09-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Migrate every remaining Phase 5 row of Phase 3's call-site inventory onto the shared
byte-budget mechanism, completing REQ-bounded-read-mechanism: the five `scrollAllPoints`
callers still passing `unbudgetedView` (`ScanSpine`, `EnumerateCitations`, `NearDuplicates`'
id enumeration, `derivePurgeEligible`, `previewRevertWithSteps`) and the four sweeps that
still run their OWN `ScrollAndOffset` loops (`Store.Migrate` ×3 loops, `Store.SummarizeMissing`,
`Store.revertWithSteps`, `Store.Reindex` + its `reindexTargetContents` `Get`). Then, and only
then, raise the production client's `MaxCallRecvMsgSize` as documented defense-in-depth, and
close #497. Covers REQ-bounded-read-mechanism (its closing half), REQ-sweeps-bounded,
REQ-recv-limit-backstop, REQ-ci-store-green.

Out of scope: cross-spine partial results (Phase 6) and provider response draining (Phase 7).

</domain>

<decisions>
## Implementation Decisions

### Carried forward

- **D-00 [informational] (user preference `1w3h5sy56m`, rule `xvqj44e5mk`):** choose by idiom and
  long-term maintenance, never by effort; prefer the established mechanism over a hand-rolled one.
- Phase 3: `scrollAllPoints`'s byte-budget extension, `readView`/`sweepLimit`/`perRPCLimit`,
  `rpcByteBudget`, the D-07 batch-of-1 fallback → named `ErrResponseTooLarge`, and the D-08
  inventory with its closing-check commands.
- Phase 4: `Store.List`/`ListScheduled`/`Search`/`SearchDiscovery` already migrated;
  `MaxRecallLimit`; the `out_of_range` rejection; `response_too_large`. **The backstop must land
  after Phase 4's regression tests already pass without it** — they do (full `task` green three
  times at the Phase 4 close), so no test in this milestone depends on the raised ceiling.
- Rule `m45p2b4bp7`: never write a test or gate that asserts third-party behavior we do not own.

### Own-loop sweeps (REQ-sweeps-bounded, REQ-bounded-read-mechanism)

- **D-01:** All four own-loop sweeps **migrate onto `scrollAllPoints`** — their hand-rolled
  `ScrollAndOffset` cursor loops are deleted, and `unbudgetedView` is deleted once its last caller
  is gone (inventory closing check (b) must print `0`). This is the literal reading of
  REQ-bounded-read-mechanism ("every site goes through a shared primitive or carries its recorded
  exemption") and the alternative — budgeting each loop in place — is the per-site patching
  Pitfall 1 exists to prevent. The stateful bits (`Passes`, the PA-3 non-shrinking-backlog guard,
  `PreviewManifest`, manifest-limited apply, partial-failure resume) become closure state over the
  callback; early termination is expressible because a callback error propagates out of
  `scrollAllPoints` (verified against `spine.go:88-94`). — **Reversibility:** costly — it rewrites
  the read loop of `engram migrate`, the most safety-critical operator command; undoing means
  restoring four deleted loops.
- **D-02 (user, explicit):** **No new characterization tests.** The in-place suites ARE the
  characterization — refactor, run them, and any break must be explainable as an expected change
  rather than a regression. Verified coverage before deciding: 83 tests across
  `migrate_test.go` (14), `revert_test.go` (9), `summarize_test.go` (8), `reindex_test.go` (17),
  `spine_test.go` (35), with named tests for every at-risk semantic —
  `TestMigrateFullBacklogProjection`, `TestMigrateManifestIntersection`,
  `TestMigrateManifestSparedDeletedRecord`, `TestMigrateManifestBacklogAppeared`,
  `TestMigrateDryRunAndManifestMutuallyExclusive`, `TestMigrateRevertPartialFailureReconciliation`,
  `TestMigrateRevertFixtureInjectionConverges`. Writing a second set against code that already has
  one is make-work.
- **D-03:** Each migrated sweep gets its own real-Qdrant regression over a scope whose
  256-record pages would have exceeded the receive limit (REQ-sweeps-bounded names all five
  commands). These assert OUR request shape stays bounded — never that gRPC enforces a ceiling.

### Sweep payload projection

- **D-04:** **Per-sweep projection.** Each migrated sweep declares the fields it actually reads
  rather than sharing one full-payload `sweepView`: `spine-review verify` needs `citations`,
  `scan`/`purge` need materially less, `NearDuplicates` already projects `short_id`+`scope`.
  Extends Phase 3's D-04 payload-selector pattern. A shared full view would cap every whole-spine
  sweep at ~2 records per RPC (the ~947 KiB full-record ceiling), which is the wrong trade for
  commands that iterate the entire collection. A sweep that omits a field it reads is a bug its
  own existing suite must catch (same discipline as D-02).

### Receive-limit backstop (REQ-recv-limit-backstop)

- **D-05 (user-chosen):** `MaxCallRecvMsgSize` = **64 MiB**, set in exactly ONE place
  (`store.NewQdrantClient`), documented as defense-in-depth only — never the fix. Recorded at
  decision time: 64 MiB masks a genuinely unbounded read longer than a tighter ceiling would;
  accepted deliberately.
- **D-06 (user, explicit; rule `m45p2b4bp7`):** Test **only that the setting is passed through** —
  that `NewQdrantClient` puts the configured `MaxCallRecvMsgSize` into its dial options, and that
  `storetest.Dial` passes `storetest.RecvLimit`. Do **NOT** write a test asserting gRPC actually
  enforces a ceiling, or that a response of N bytes fails: that is third-party buffer handling we
  do not own. `storetest.Dial` builds on `NewQdrantClient` and appends its own limit LAST
  (last-wins), so test clients keep the 4 MiB limit under test; that ordering stays documented in
  `dialOptions`' doc comment and is not given a behavioral test against gRPC.

### #497 / CI (REQ-ci-store-green)

- **D-07 (user-chosen):** **Close #497 in-phase on #498's existing evidence.** #497 is a CI
  *stability* issue — Qdrant `connection refused` when four testcontainers contended for a 2-vCPU
  runner — not an overflow question. #498 fixed the root cause (one shared `services:` Qdrant for
  the whole job, plus the explicit `--health-cmd` the image lacks) and has held 60+ runs. This
  phase's fixtures are bounded by the same budgets as Phases 1/3/4's. Record that reasoning and
  close the issue; no post-merge observation gate, and no new test policing our own fixture sizes
  (the fixtures already self-assert that they exceed the limit).

### Claude's Discretion

- How each sweep's stateful bits are expressed as closure state, and the sentinel-error shape for
  early termination.
- Each sweep's exact payload projection (within D-04) and the resulting `readView` construction.
- Whether `unbudgetedView` is deleted in the same task as the last caller's migration or in the
  closing task — as long as inventory closing check (b) prints `0` before the phase ends.
- Whether `NearDuplicates`' `QueryBatch` stays exempt (it requests no payload) — the inventory
  already records it as exempt-in-effect; confirm or restate with a written justification.
- Test design per sweep (both `storetest.SeedOversized` shapes at `storetest.RecvLimit`).
- Phase 5 red-evidence patches (register in `redEvidenceDirs` after the last plan).
- Where the `#497 closed` reasoning is recorded (SUMMARY, the issue itself, or both).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements, roadmap, prior phases
- `.planning/REQUIREMENTS.md` — REQ-bounded-read-mechanism, REQ-sweeps-bounded,
  REQ-recv-limit-backstop, REQ-ci-store-green
- `.planning/ROADMAP.md` — Phase 5 goal and success criteria 1–4
- `.planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-INVENTORY.md` —
  **the checklist for this phase**: the ten Phase 5 rows, and the closing-check commands (a) the
  derivation set, (b) `rg -o 's[.]scrollAllPoints[(].*unbudgetedView[(]' internal/store --glob '!*_test.go' | wc -l`
  must print `0`, (c) every Phase 4/5 row routes through a primitive or moves to Exempt,
  (d) recall-gate classifications match
- `.planning/phases/03-.../03-CONTEXT.md` (D-02/D-06/D-07 budgets and fallback),
  `03-02-SUMMARY.md` (the `scrollAllPoints` byte-budget extension)
- `.planning/phases/04-list-listscheduled-search-bounded-reads/04-CONTEXT.md`,
  `04-VERIFICATION.md` (passed), `04-SECURITY.md` — the regression tests that must keep passing
  WITHOUT the backstop
- `.planning/research/PITFALLS.md` — Pitfall 1 (per-site patches), Pitfall 2 (count-only caps)

### Code — the sites to migrate
- `internal/store/spine.go:67-101` — `scrollAllPoints` (the target primitive; note the callback's
  error propagates, which is how early termination is expressed)
- Five `unbudgetedView` callers: `spine.go:278` (`ScanSpine`), `:385` (`EnumerateCitations`),
  `:591` (`NearDuplicates` id enumeration, already projecting `short_id`+`scope`), `:1052`
  (`derivePurgeEligible`), `revert.go:278` (`previewRevertWithSteps`)
- Four own-loop sweeps: `migrate.go:314`, `:377`, `:531` (+ `migrateBatch = 256` at `:22`);
  `summarize.go:145`; `revert.go:431`; `store.go:3449` (`Reindex`, + `reindexTargetContents`' `Get`)
- `internal/store/boundedread.go:130-170` (`rpcByteBudget`, ceilings, `perRPCLimit`), `:250-270`
  (`keysView`, `unbudgetedView` and its deletion note)
- `internal/store/store.go:529-531` — the doc comment reserving the backstop for this phase
- `internal/store/storetest/storetest.go:265-286` — `dialOptions`, the append-LAST no-widen note
- `internal/store/schemaversion_recallgate_test.go` — `operatorMigrationEmitters` rows for
  `Store.Migrate`, `Store.SummarizeMissing`, `Store.revertWithSteps`, `Store.Reindex`,
  `Store.scrollAllPoints`; update classifications in the same change that moves a site
- `internal/store/qdrant_client_convergence_test.go` — `TestQdrantClientConstructedOnlyByNewQdrantClient`;
  `schemaversion_stamp_gate_test.go:749-780` — `qdrantClientHolderAllowlist`

### Existing tests that ARE the characterization (D-02)
- `internal/store/migrate_test.go` (14), `revert_test.go` (9), `summarize_test.go` (8),
  `reindex_test.go` (17), `spine_test.go` (35)

### CI / issues
- `.github/workflows/ci.yaml:25-60` — the shared `services:` Qdrant, the explicit health check,
  and `TestQdrantImageMatchesCIService`
- GitHub #497 (CI stability, closed here on #498's evidence), #498 (the shared-container fix),
  #585 (the sibling-site report this milestone answers)

### Rules & memories
- Rules `m45p2b4bp7` (never gate third-party behavior — governs D-06 and D-03),
  `xvqj44e5mk` (idiomatic over hand-rolled — governs D-01), `8dfdhfs5nn`, `2rjnv8sc9a`,
  `n6m4as49mr` (explicit `git commit` pathspec)
- Memories `1w3h5sy56m` (frame by idiom and maintenance, never effort), `y02a9ft3gy`
  (`storetest` importers must be `package store_test`), `xb8y5pk6eh` (read-path facts),
  `7r10s08k9q` (the 4 MiB cap this milestone exists for), `rgcp7yb5fh` (keep shared planning
  ledgers out of `covered_files`; land fixes before the verifier), playbook `f7zdc18tn3`,
  `2tb2ew756h` (the progress-table corruption to audit after every `roadmap`/`state` call)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `scrollAllPoints` + `readView`/`sweepLimit`/`perRPCLimit` + the D-07 batch-of-1 fallback: the
  whole mechanism already exists and is proven on both oversized fixture shapes — this phase is
  wiring, not invention.
- `qdrant.NewWithPayloadInclude`/`NewWithPayloadExclude`: the projection primitive D-04 uses.
- `storetest.Dial`/`SeedOversized`/`RecvLimit`: the fixture harness, unchanged since Phase 1.
- The five sweeps' existing test suites: the characterization D-02 relies on.

### Established Patterns
- Name-keyed AST gates (`recallTransmitters`, `operatorMigrationEmitters`) updated in the SAME
  change that moves a call site.
- A deleted constructor's absence is proven by a zero-count `rg -o … | wc -l` gate, not by reading.
- Red evidence: one patch per RED direction, registered in `redEvidenceDirs` by the last plan.
- `engram migrate` never mutates without `--apply`; preview/apply parity is a standing invariant.

### Integration Points
- `internal/store` only for the migrations and the backstop; `cmd/engram` is untouched by D-01
  (the sweeps' CLI surfaces do not change); `.github/workflows/ci.yaml` is untouched by D-07.

</code_context>

<specifics>
## Specific Ideas

- Inventory closing check (b) printing `0` is the phase's own completion signal for D-01.
- 64 MiB, one place, documented as a backstop — and tested only for pass-through.
- "#497 is a CI-stability issue, not an overflow question" — the sentence that settles D-07.

</specifics>

<deferred>
## Deferred Ideas

- Tightening the backstop below 64 MiB if it ever masks a real regression (revisit with evidence).
- Capping the still-uncapped payload fields — GitHub #589 (from Phase 3 D-11).
- Per-sweep projection for any site this phase leaves exempt, should one acquire a payload later.

</deferred>

---

*Phase: 05-operator-sweeps-ci-backstop*
*Context gathered: 2026-09-20*
