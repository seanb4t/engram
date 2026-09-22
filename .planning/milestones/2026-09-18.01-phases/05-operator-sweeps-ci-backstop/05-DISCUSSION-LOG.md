# Phase 5: Operator Sweeps & CI Backstop - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-20
**Phase:** 5-operator-sweeps-ci-backstop
**Areas discussed:** Own-loop sweeps, Backstop size + enforcement, Sweep view projection, #497 CI-green evidence

---

## Own-loop sweeps: migrate or budget in place

Survey that framed the question: the ten Phase 5 inventory rows are not uniform. Five ride `scrollAllPoints` with `unbudgetedView` (a view swap). Four carry their own `ScrollAndOffset` loops with stateful semantics — `migrate` ×3, `summarize-missing`, `revert` apply, `reindex`.

| Option | Description | Selected |
|--------|-------------|----------|
| Migrate onto scrollAllPoints | All four move to the shared primitive; hand-rolled loops and `unbudgetedView` deleted | ✓ |
| Budget them where they stand | Size each `Limit` from the view ceiling in place; lower risk, but "one mechanism" becomes "one sizing rule in five places" | |
| Split by risk | Migrate the mechanical two, budget `migrate`/`revert` in place with a recorded exemption | |

**User's choice:** Migrate onto scrollAllPoints.
**Notes:** Consistent with rule `xvqj44e5mk` and preference `1w3h5sy56m`. Verified feasible first: a callback error propagates out of `scrollAllPoints`, so early termination is expressible; stateful bits become closure state.

### What guards the `engram migrate` rewrite?

| Option | Description | Selected |
|--------|-------------|----------|
| Pin behavior before touching it | Write characterization tests against the current loops first, then migrate | |
| Red-evidence patches only | Rely on the phase's normal red-evidence discipline | |
| Both | Characterization + red evidence | |

**User's response (free text):** *"why are we (re) writing tests for something that already has tests? isn't the idiomatic thing here to make changes and run the _in place_ tests, if we break them they need to break in expected ways?"*

**Resolution:** The question was wrong. No new characterization tests — the in-place suites are the characterization. Verified the coverage actually spans the at-risk semantics before accepting: 83 tests across the five sweep test files, with named tests for backlog projection, manifest intersection/spared/appeared, dry-run/manifest exclusivity, partial-failure reconciliation, and convergence. Recorded as D-02.

---

## Backstop size + where it lives

| Option | Description | Selected |
|--------|-------------|----------|
| 16 MiB | 4× the grpc default; above the page budget and record ceiling, small enough that a runaway still fails named | |
| 64 MiB | Qdrant's commonly-cited ceiling; very little can overflow it | ✓ |
| Derive from the budgets | e.g. 4× `pageByteBudget`, so the two cannot drift | |

**User's choice:** 64 MiB.
**Notes:** The masking risk was raised at decision time (a real unbounded-read regression hides longer at 64 MiB) and accepted deliberately. Recorded in D-05.

### Keeping "never relied on by a regression test" true

Surfaced during the discussion: `storetest.Dial` builds on `store.NewQdrantClient`, so the 64 MiB would flow into every test client — saved only by `storetest` appending its 4 MiB last (last-wins in grpc).

| Option | Description | Selected |
|--------|-------------|----------|
| Assert the effective limit in storetest | A response between 4 MiB and 64 MiB must still fail | |
| Document the ordering, no new test | Rely on the `dialOptions` doc comment | |
| Keep the backstop out of NewQdrantClient | Set it only at the production composition root | |

**User's response (free text):** *"we are not going ot test grpc or golang buffer handling. we _do_ need to test that we pass through the setting."*

**Resolution:** Rule `m45p2b4bp7` — the proposed assertion tests grpc's buffer handling, which we do not own. Test only that the configured value reaches the dial options on both constructors. Recorded as D-06.

---

## Sweep view: full payload or projected

| Option | Description | Selected |
|--------|-------------|----------|
| Per-sweep projection | Each sweep declares the fields it reads; extends Phase 3 D-04 | ✓ |
| One shared full-payload sweepView | Simplest and safest, but caps whole-spine sweeps at ~2 records/RPC | |
| Full view now, project later | Defers the work, leaves sweeps slow meanwhile | |

**User's choice:** Per-sweep projection.

---

## #497 CI-green evidence

| Option | Description | Selected |
|--------|-------------|----------|
| Fixture-size discipline + observe on the PR | Assert fixtures are minimally sized; close #497 after observing CI green post-merge | |
| Close it now on #498's evidence | #498 fixed the root cause and held 60+ runs; record the reasoning and close | ✓ |
| Drop as already-satisfied | Mark satisfied-on-arrival, spend no phase scope | |

**User's response (free text):** *"Why the hell are we testing grpc overflow?"*

**Resolution:** Answered directly rather than folding — the oversized fixtures assert OUR request shape stays bounded, never that grpc raises anything (the distinction `TestListScopesFullPayloadsOverGRPCLimit`'s own doc comment draws). But two things had been conflated, and one was genuine over-engineering on the assistant's part: #497 is a CI *stability* issue (testcontainer contention), not an overflow question at all, and the proposed "assert fixture sizes" gate was bookkeeping about bookkeeping. Re-asked narrowly; user chose to close #497 in-phase on #498's evidence. Recorded as D-07.

---

## Claude's Discretion

- Closure-state shape for each migrated sweep and the sentinel-error form for early termination.
- Each sweep's exact payload projection and `readView` construction.
- Timing of `unbudgetedView`'s deletion (any task, so long as closing check (b) prints `0`).
- Whether `NearDuplicates`' `QueryBatch` stays exempt.
- Per-sweep test design; Phase 5 red-evidence patches; where the #497 reasoning is recorded.

## Deferred Ideas

- Tightening the 64 MiB backstop if it ever masks a real regression.
- Capping the still-uncapped payload fields — GitHub #589.
- Per-sweep projection for any site left exempt that later acquires a payload.
