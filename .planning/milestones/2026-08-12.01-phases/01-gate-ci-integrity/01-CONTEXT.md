# Phase 1: Gate & CI Integrity - Context

**Gathered:** 2026-08-13
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase makes the gates the rest of milestone 2026-08-12.01 will lean on capable of
actually going red. Nothing user-facing ships. Two independent trust defects are closed:

1. **Key-link `pattern:` gates are no-ops.** `gsd-tools verify key-links` compiles a plan's
   `pattern:` field with `new RegExp` (`verify.cjs:1049-1117`) against a string
   `parseMustHavesBlock` never unescapes. A YAML-style `\\.` therefore reaches the regex
   engine as "a backslash, then any char" — valid regex, silently unmatchable. 38 such
   patterns exist repo-wide; 25 of them are the v0.13.x Phase 1–2 gates that passed
   verification while checking nothing (#479).
2. **The CI Qdrant testcontainer dies mid-run.** Four packages each boot their own
   `qdrant/qdrant:v1.18.2`, and `go test ./...` runs packages concurrently, so a 2-vCPU
   `ubuntu-latest` runner can be hosting four Qdrant instances at once. `internal/store`
   dies while siblings pass; the resulting `connection refused` cascade buries whatever
   really failed (#497).

**In scope:** normalizing this repo's key-link patterns, a recurring guard against
reintroduction, a one-time reassessment of the v0.13.x Phase 1–2 gates, and a CI change that
removes the resource pressure plus captures container death as evidence.

**Out of scope:** fixing `parseMustHavesBlock` in gsd-core; repinning v0.13.x gates found
unpinned; any schema, migration, or record-state work (Phases 2–8).

</domain>

<decisions>
## Implementation Decisions

### Key-Link Fix Locus

- **D-01:** The fix is **repo-local only**. This repo's patterns are normalized and guarded;
  gsd-core's `parseMustHavesBlock` is **not** patched. The unescaping gap is reported
  upstream per rule `8dfdhfs5nn` (a shape the tool lacks is an upstream gap to report,
  never a local invention), but this phase ships nothing into gsd-core and does not gate on
  an upstream fix. Note the live precedent in memory `cvvrwjbsnz`: a prior GSD-core bug was,
  by explicit decision, spine-tracked rather than filed — so "report" here means recording
  the gap, and the filing decision is Sean's, not the executor's.
  — **Reversibility:** reversible — the guard is independent of what upstream does.

- **D-02:** Normalized patterns use the **escape-free character-class form** — `[.]`, `[(]`,
  `[)]` — never `\.` or `\\.`. This is the form v0.13.x Phase 3 already adopted in
  `ca8d337c` and verified against the tool on a scratch fixture. Because no backslash is
  ever needed, the guard's escaping rule can be a flat "no backslash in `pattern:`" rather
  than a fuzzy single-vs-double-backslash discrimination.
  — **Reversibility:** reversible — mechanical rewrite either direction.

- **D-03:** The guard covers **both** silent-no-op shapes, not only the escaping one:
  (a) the `\\` escaping shape, and (b) a pattern that is valid, correctly escaped, and
  **unsatisfiable** — #479's second finding, where Phase 3 pinned `addApplyFlag[(]` on a
  file whose leaf routes through `registerDestructive`, so the symbol never appears in the
  `from` file at all. A pattern that can never match is exactly as much a no-op as one that
  is silently unmatchable.
  — **Reversibility:** reversible.

- **D-04:** The two halves of the guard have **different scopes**, because they have
  different time-sensitivity. The **escaping** check is time-invariant (a pattern is
  well-formed or it isn't, forever) and runs over **every** plan in `.planning/`, archived
  milestones included. The **satisfiability** check depends on the code as it stands right
  now and runs **only against the active milestone** (`.planning/phases/**`). Running
  satisfiability over archived plans at HEAD would go red whenever shipped code is
  refactored — a red that is not a defect, which trains people to ignore the gate and
  recreates the exact failure this phase exists to fix.
  — **Reversibility:** reversible.

### Guard Shape & Fail-First Proof

- **D-05:** The guard lives in a **new stdlib-only leaf package, `internal/keylinks`**
  (name indicative), exercised by a Go test so it runs inside `go test ./...` — the gate
  that actually blocks CI today. This mirrors the `internal/surfaces` / `internal/openaiurl`
  leaf precedent, and `internal/surfaces` already establishes that a Go conformance gate
  parsing markdown from elsewhere in the repo is an accepted pattern here. No new toolchain,
  no new CI job.
  — **Reversibility:** reversible.

- **D-06:** Fail-first is proven by a **committed good/bad fixture pair** in testdata: a
  known-good `key_links` block and a known-corrupted one, with the test asserting GREEN on
  the first and RED on the second. This makes memory `v5q7jdbw43`'s rule — an assertion is
  not shippable until run against both a known-good and a known-bad input — a permanent
  repo artifact that re-runs forever, rather than a one-time observation during execution
  that leaves nothing behind to stop the guard being silently defanged later.
  — **Reversibility:** reversible.

- **D-07:** On failure the guard reports **every offender in one run**: `file:line`, which
  shape failed (escaping vs unsatisfiable), and the corrected character-class form. Not
  fail-fast-on-first — with 38 known instances, first-failure-only turns cleanup into a
  serial grind, and naming the corrected form keeps the surface learnable by reading, which
  is this repo's established habit.
  — **Reversibility:** reversible.

- **D-08:** Patterns are **restricted to the RE2 ∩ JavaScript common subset** — the guard
  rejects backreferences, lookaround, and named groups, not merely malformed patterns. This
  matters because Go's `regexp` is RE2 while the consuming tool (`verify.cjs`) uses
  JavaScript's backtracking engine; they are different languages. Constraining patterns to
  constructs that mean the same thing in both is what makes RE2-side validation valid
  evidence about JS-side behavior. It also keeps key-link patterns simple, which is what a
  key-link should be.
  — **Reversibility:** costly — loosening later is easy, but any pattern written against
  the wider grammar in the meantime would need re-auditing against both engines.

### Reassessment Scope & Archived Plans

- **D-09:** **All 38** offending patterns are normalized repo-wide, not only the 25 in
  v0.13.x Phases 1–2 that `REQ-keylink-past-gates-reassessed` names. Since the escaping
  check runs everywhere (D-04), leaving 13 offenders outside the milestone would force
  either a permanently-red guard or an exclusion list — and an exclusion list is how a gate
  quietly stops gating.
  — **Reversibility:** reversible.

- **D-10:** Archived, shipped `PLAN.md` files are **edited in place, in one commit**, with
  the rationale in the commit message and no inline annotation in the documents. These are
  mechanical `\\.` → `[.]` rewrites that change what the pattern means *to the tool* but not
  what the plan author intended, and git already carries the before/after. Adding 38 pieces
  of explanatory prose to shipped `.planning/**` documents was rejected: it creates parallel
  bookkeeping and edges toward the frontmatter/parsing hazards rule `2rjnv8sc9a` guards
  against.
  — **Reversibility:** reversible — single revertible commit.

- **D-11:** "Genuinely pinned" means **the corrected pattern resolves against its `from`
  file at HEAD**. Match → pinned. No match → recorded unpinned *with the reason* (symbol
  renamed, routed through a wrapper, file moved). The stronger reading — "a real Go test
  must exist that fails on regression" — was explicitly rejected as a phase of its own:
  auditing 25 links for test coverage and writing tests for shipped v0.13.x behavior is not
  what "reassessed" asks for.
  — **Reversibility:** reversible.

- **D-12:** The reassessment is a **one-time sweep**, distinct from the recurring guard even
  though both use the same matching logic. The sweep resolves v0.13.x Phase 1–2 patterns
  against HEAD once and records verdicts; the recurring guard only ever checks satisfiability
  on the active milestone (D-04). Wiring archived plans into the recurring gate would
  reintroduce the refactor-churn problem D-04 exists to prevent.
  — **Reversibility:** reversible.

- **D-13:** Verdicts land as a **table in this phase's own `VERIFICATION.md`** — every
  v0.13.x Phase 1–2 link with its verdict and reason. Not `.planning/WINDOWS.md` (that would
  file permanent open debt against shipped work nobody plans to revisit, converting a closed
  finding into a standing broken window) and not one GitHub issue per unpinned gate (a burst
  of low-value issues for gates on shipped code).
  — **Reversibility:** reversible.

- **D-14:** A gate found unpinned is **recorded only — not repaired in this phase**. The
  requirement says "reassessed", not "repaired". Writing regression tests for shipped
  v0.13.x behavior is unscoped work discovered mid-phase; recording it keeps the decision
  deliberate and later, rather than absorbed silently now.
  — **Reversibility:** reversible.

### Qdrant CI Mitigation

- **D-15:** CI runs **one shared Qdrant** and sets `ENGRAM_QDRANT_TEST_ADDR`, collapsing
  four containers into one and removing the resource pressure at its source. `TestMain`
  already honors this env var as its fast path, so the plumbing exists — the only blocker is
  the collection-name collision. Serialization (`-p 1`) was rejected: 546 of the suite's
  tests live in `internal/store` + `internal/server`, so it lengthens the critical path
  substantially. A larger runner was rejected as treating the symptom — it costs money per
  run and the flake returns the moment a fifth Qdrant-backed package appears.
  — **Reversibility:** costly — undoing means reverting both the CI service definition and
  the collection-name namespacing across four packages.

- **D-16:** Collections are namespaced by a **per-package constant prefix** (e.g.
  `store_mem_eval_test` / `server_mem_eval_test`). `internal/store` and `internal/server`
  both currently hardcode `mem_eval_test` (`store_test.go:249`, `tools_test.go:321`), which
  is harmless today and a collision the moment they share an instance. Per-test unique names
  (`t.Name()` / random suffix) were rejected as a much wider diff across 546 tests that also
  changes cleanup semantics current tests rely on. Note that
  `internal/store/reindex_test.go` creates and drops **pairs** of collections (`src`/`tgt`)
  with their own names — these need the same prefix treatment.
  — **Reversibility:** costly — touches four packages' test setup.

- **D-17:** **All four** Qdrant-backed packages move onto the shared instance —
  `internal/store`, `internal/server`, `internal/e2e`, `internal/retrievaleval`. All already
  read `ENGRAM_QDRANT_TEST_ADDR`, so one CI env var covers them; partial adoption would
  leave resource pressure on the table and create two CI behaviors to reason about.
  — **Reversibility:** reversible.

- **D-18:** The **per-package testcontainer boot path stays** as a fallback. `TestMain`'s
  existing precedence — env var → testcontainer → skip — already does the right thing: CI
  takes the shared fast path, a developer with Docker and no env var still gets a container,
  and `ENGRAM_REQUIRE_QDRANT` keeps CI fail-closed. Removing it would force every developer
  to run Qdrant manually before `go test ./...`, an ergonomics regression in a phase about
  trusting the gates.
  — **Reversibility:** reversible.

- **D-19:** Container exit reason is captured by a **CI post-step on failure** —
  `if: failure()` dumping container state, exit code, last logs, and OOM evidence from
  `dmesg`. This works regardless of which package or mitigation is involved, works when the
  Go process is already gone, and directly answers "was it OOM-killed", the hypothesis
  memory `cmjxxswmm2` leaves open. In-Go capture via testcontainers' `Logs()`/`State()` was
  rejected as primary: when the container dies mid-run the Go side often cannot reach it
  either, so the most important case is the one most likely to yield nothing.
  — **Reversibility:** reversible.

- **D-20:** The fix is proven by **asserting the mechanism, not the absence of the flake**.
  Checkable claims: exactly one Qdrant container in the CI run; all four packages resolving
  to the same address; collection-name prefix sets provably disjoint across packages.
  Repeated-green-runs-as-evidence was rejected — memory `cmjxxswmm2` records ~3 hits in 2
  hours, so N would have to be large to mean anything, and a green streak never proves
  absence. The resource pressure that caused the flake *is* directly observable even though
  the flake's absence is not.
  — **Reversibility:** reversible.

### Claude's Discretion

No area was answered "you decide" — every question resolved to an explicit choice. Open to
planning judgment: the exact package name for the guard leaf (`internal/keylinks` is
indicative), the precise prefix strings in D-16, and the specific shape of the CI service
container definition vs. a plain boot step.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone & requirements
- `.planning/ROADMAP.md` §"Phase 1: Gate & CI Integrity" — goal, three success criteria,
  and the milestone goal this phase's gates protect
- `.planning/REQUIREMENTS.md` §"Gate & CI Integrity" — `REQ-keylink-pattern-matchable`,
  `REQ-keylink-past-gates-reassessed`, `REQ-ci-qdrant-container-stability`
- `.planning/PROJECT.md` — as-built baseline; §"Current State: v0.13.x" records the
  `internal/surfaces` conformance-gate precedent this phase's guard mirrors

### Repo conventions the guard must respect
- `CLAUDE.md` §Conventions — task runner, lint/format gates, SPDX header scope
- `.licenserc.yaml` — owns SPDX scope; `.planning/**` is excluded (rule `2rjnv8sc9a`)
- `.planning/codebase/TESTING.md` — existing test-tier conventions
- `.planning/codebase/CONVENTIONS.md` — Go package and naming conventions

### The defect itself
- GitHub issue #479 — key-link `pattern:` escaping defect; also documents the second
  (unsatisfiable-pattern) failure mode folded in via D-03
- GitHub issue #497 — recurrent Qdrant testcontainer CI flake
- `$HOME/.claude/gsd-core/bin/lib/verify.cjs` §1049-1117 — the `parseMustHavesBlock` →
  `new RegExp` path being worked around. **Read-only reference — this phase does not modify
  gsd-core** (D-01).

### Code touched
- `internal/store/store_test.go` §33-152 — `qdrantImageTag`, `requireQdrant`, `TestMain`
  precedence chain, `terminateQdrant`, `dialTestClient`, `testStore` (fixed collection name
  at :249)
- `internal/server/tools_test.go` §214, §321 — sibling `terminateQdrant`; the colliding
  `mem_eval_test` collection name
- `internal/e2e/harness_test.go` §61-134 — third `TestMain`/container path
- `internal/retrievaleval/retrieval_eval_test.go` §299, §312, §343 — fourth container path;
  already uses a `retrievaleval_` prefix
- `internal/store/reindex_test.go` — creates/drops `src`/`tgt` collection pairs needing the
  same prefix treatment (D-16)
- `.github/workflows/ci.yaml` §22-56 — the `test` job, `ENGRAM_REQUIRE_QDRANT: "1"`, and
  `go test ./...`
- `Taskfile.yaml` §35-73 — `test`, `test:go`, `test:strict` targets

### Precedent to mirror
- `internal/surfaces/` — stdlib-only leaf package whose conformance gate parses markdown
  across the repo; the model for `internal/keylinks` (D-05)
- `.planning/milestones/v0.13.x-phases/03-*` commit `ca8d337c` — the character-class
  normalization already verified against the tool (D-02)

### Durable memory (engram spine — `repo:github.com/seanb4t/engram`)
- `cmjxxswmm2` — the CI flake's diagnosis: environmental, proven by a docs-only PR;
  per-package containers; **diagnostic rule — sort failures by timestamp and read the
  earliest, since a `connection refused` leader means cascade**
- `v5q7jdbw43` — an assertion is not shippable until run against known-good *and*
  known-bad input; the rule D-06 makes permanent
- `zbz7ajvaaa` — the clock-race precedent behind "assert the mechanism, not the absence"
- `cvvrwjbsnz` — precedent that a GSD-core bug may be spine-tracked rather than filed
  upstream (bears on D-01's "report the gap")
- `4g5gbrmv29` — `task` default never runs `gofmt -l .`; run it before pushing, since CI
  gofmt is a separate step (`ci.yaml:47`)
- Rules `8dfdhfs5nn` (never invent structure in tool-parsed planning artifacts) and
  `2rjnv8sc9a` (never add SPDX headers to `.planning/**`) both bind the plan-file edits in
  D-09/D-10

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`TestMain`'s existing precedence chain** (`store_test.go:75-113`): `ENGRAM_QDRANT_TEST_ADDR`
  → testcontainer → skip. The shared-instance path in D-15 needs **no new Go plumbing** — CI
  only has to set the env var. All four packages already implement this chain.
- **`requireQdrant()`** (`store_test.go:56-66`): the fail-closed gate, deliberately returning
  an error rather than coercing a bad parse to `false`. Sharing must not weaken it —
  `ENGRAM_REQUIRE_QDRANT: "1"` stays set in CI.
- **`internal/surfaces`**: existing proof that a stdlib-only leaf package can host a
  conformance gate that reads markdown from across the repo and fail `go test ./...`.
- **pytest job in CI** (`ci.yaml:113-124`): exists and runs, but was **not** chosen as the
  guard's home (D-05).

### Established Patterns
- **Leaf-package convention**: `internal/surfaces`, `internal/openaiurl` — stdlib-only, no
  store/authz dependency, imported by heavier packages never the reverse. `internal/keylinks`
  follows it, as will `internal/migrate` in Phase 3.
- **Fixed collection names in tests**: `mem_eval_test` (store + server — the collision),
  `mem_eval_test_prod_defaults_{1,2}` (store), `retrievaleval_*` (already prefixed),
  `mem_load_once_test` (server). Deterministic names are the existing habit; D-16 keeps that
  and adds a package prefix rather than switching to random names.
- **Name-targeted teardown only**: every `DeleteCollection` in the test tree targets a named
  collection, and there is **no** `ListCollections`-then-delete-all anywhere. This is what
  makes a shared instance safe with only name namespacing — verified, not assumed.
- **`gofmt` sits outside `task`'s default gate** (`Taskfile.yaml:115`, `ci.yaml:47`): local
  green is not CI green. Run `gofmt -l .` before pushing (memory `4g5gbrmv29`).

### Integration Points
- **`.github/workflows/ci.yaml` `test` job** — gains a Qdrant service (or boot step), the
  `ENGRAM_QDRANT_TEST_ADDR` env var, and an `if: failure()` diagnostic post-step (D-19).
- **`go test ./...`** — picks up the new `internal/keylinks` guard automatically; no CI job
  or Taskfile target needs adding (D-05).
- **`.planning/**` plan files** — 38 `pattern:` fields rewritten across `milestones/v0.9.x-`,
  `v0.10.x-`, `v0.12.x-`, and `v0.13.x-phases/` (D-09).
- **This phase's `VERIFICATION.md`** — carries the reassessment verdict table (D-13).

</code_context>

<specifics>
## Specific Ideas

- **The escaping/satisfiability asymmetry is the load-bearing insight** behind D-04: one
  property is time-invariant and one is not, so they get different scopes. A planner that
  collapses them into a single repo-wide check will produce a guard that goes red on
  unrelated refactors of shipped code.
- **RE2 ≠ JavaScript RegExp.** The guard validates in Go; the tool consumes in JS. D-08's
  common-subset restriction is what closes that gap — it is not incidental strictness.
- **Do not chase the flake statistically.** D-20 is deliberate: assert one container, one
  shared address, disjoint collection prefixes. Green streaks are not evidence.
- **When reading a CI failure during this phase**, apply memory `cmjxxswmm2`'s rule — sort
  by timestamp and read the earliest failure. A `connection refused` leader means cascade; a
  real assertion means a real bug, and the cascade will bury it.

</specifics>

<deferred>
## Deferred Ideas

- **Fixing `parseMustHavesBlock` in gsd-core** — the actual root cause. Reported as an
  upstream gap per D-01, not fixed here. Whether it is *filed* upstream or spine-tracked
  (per precedent `cvvrwjbsnz`) is Sean's call, not the executor's.
- **Repinning v0.13.x gates found unpinned** — recorded in this phase (D-14), repaired in
  none. If the sweep surfaces a gate whose loss actually matters, that is its own scoped
  work.
- **Per-test collection isolation** (`t.Name()` / random suffixes) — stronger than D-16's
  per-package prefix and would enable intra-package parallelism, but a much wider diff
  across 546 tests with different cleanup semantics. Revisit only if intra-package
  parallelism is ever wanted.
- **Pinned-commit resolution for archived key-links** — resolving each archived plan's
  `from` file at the commit it shipped, rather than at HEAD. Strictly more correct than
  D-04's scope carve-out, but needs git plumbing per link for gates on completed work.

### Reviewed Todos (not folded)
- **`research-versioned-payload-migration-mechanism`** (`.planning/todos/`, matched at score
  0.6 on keywords "migration/phase/internal") — **not folded.** STATE.md already records it
  as scoped into Phases 2–4 (schema versioning foundation, migration registry/sweep,
  migration CLI). It is Phase 2–4 material; the keyword match is coincidental.

</deferred>

---

*Phase: 1-Gate & CI Integrity*
*Context gathered: 2026-08-13*
