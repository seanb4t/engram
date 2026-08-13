# Phase 1: Gate & CI Integrity - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-13
**Phase:** 1-Gate & CI Integrity
**Areas discussed:** Key-link fix locus, Guard shape & fail-first proof, Reassessment scope & archived plans, Qdrant CI mitigation

---

## Key-Link Fix Locus

**Q1 — Where does the `\\`-not-unescaped bug actually get fixed?**

| Option | Description | Selected |
|--------|-------------|----------|
| Repo-local only + report gap | Normalize our patterns, add a repo guard, report the `parseMustHavesBlock` gap upstream per rule `8dfdhfs5nn` but ship nothing into gsd-core | ✓ |
| Repo-local + upstream PR to gsd-core | Same, plus fixing the unescaping at its root and opening a PR | |
| Guard only — don't touch existing patterns | Prevent new instances, leave the 38 existing (all archived) alone | |

**Notes:** Memory `cvvrwjbsnz` records a prior decision to spine-track a GSD-core bug rather
than file it, so "report the gap" leaves the filing decision with Sean. The guard works
regardless of upstream, keeping the phase self-contained.

**Q2 — Which escape-free pattern form?**

| Option | Description | Selected |
|--------|-------------|----------|
| Character classes — `[.]`, `[(]`, `[)]` | What v0.13.x Phase 3 already used in `ca8d337c`, verified against the tool | ✓ |
| Single backslash — `\.`, `\(` | Reads more naturally as regex; makes the guard discriminate single vs double backslash | |
| You decide | | |

**Notes:** Needing no backslash at all lets the guard be a flat "no backslash in `pattern:`"
rule rather than a fuzzy discrimination.

**Q3 — Is the second failure mode (valid, escaped, but unsatisfiable) in scope?**

| Option | Description | Selected |
|--------|-------------|----------|
| In scope — guard both shapes | Assert every pattern actually matches its `from` file; subsumes the escaping bug | ✓ |
| Out of scope — escaping shape only | Stick to literal REQ text; smaller and more certain to land | |
| You decide | | |

**Notes:** #479's own "Related" section documents this: Phase 3 pinned `addApplyFlag[(]` on
a file whose leaf routes through `registerDestructive`, so the symbol never appeared there.

**Q4 — Which plans does the satisfiability half run against?**

| Option | Description | Selected |
|--------|-------------|----------|
| Active milestone only | Satisfiability on `.planning/phases/**`; escaping check everywhere | ✓ |
| Everything, with pinned-commit resolution | Resolve archived `from` files at the commit that plan shipped | |
| Everything at HEAD, accept the churn | Simplest rule; guaranteed to go red for non-defects eventually | |

**Notes:** Raised because the strong check is time-sensitive in a way the escaping check is
not — an archived plan pinning a since-renamed symbol would go red without anything being
wrong. Pinned-commit resolution deferred as more correct but needing git plumbing per link.

---

## Guard Shape & Fail-First Proof

**Q1 — Where does the guard live?**

| Option | Description | Selected |
|--------|-------------|----------|
| Go test under `cmd/engram` or `internal/` | Runs in `go test ./...`; `internal/surfaces` precedent for a Go gate parsing markdown | ✓ |
| `task lint:planning` + CI step (shell/rg) | Close to the existing `rg` gate idiom; brittle for nested YAML parsing | |
| Python test in `skill/engram/hooks/tests` | pytest already in CI and parses YAML natively; wrong home for a repo-wide planning gate | |

**Q2 — How is fail-first proven?**

| Option | Description | Selected |
|--------|-------------|----------|
| Committed fixture pair, good + bad | Testdata asserting GREEN on one, RED on the other; discrimination proof lives in the repo | ✓ |
| Live tree only, RED observed during execution | Satisfies the criterion once, leaves nothing behind | |
| Both — fixtures plus a live-tree run | Two mechanisms answering two different questions | |

**Notes:** Makes memory `v5q7jdbw43`'s known-good/known-bad rule a permanent artifact rather
than a one-time observation.

**Q3 — What does the guard report on failure?**

| Option | Description | Selected |
|--------|-------------|----------|
| Every offender in one run + the corrected form | `file:line`, which shape failed, and the character-class rewrite | ✓ |
| Fail fast on the first offender | Simpler; turns 38 instances into a serial grind | |
| You decide | | |

**Q4 — How to handle the RE2-vs-JavaScript engine gap?**

| Option | Description | Selected |
|--------|-------------|----------|
| Restrict patterns to the common subset | Reject backreferences, lookaround, named groups — so RE2 validation is valid evidence about JS | ✓ |
| Compile in RE2, accept the gap, document it | Less work; leaves a silent class of divergent pattern | |
| You decide | | |

**Notes:** Raised because Go's `regexp` is RE2 while `verify.cjs` uses JavaScript's
backtracking engine — a Go guard proving "this compiles and matches" is strictly evidence
about a different regex language than the one that consumes the pattern.

**Q5 — Where under the Go tree?**

| Option | Description | Selected |
|--------|-------------|----------|
| New leaf package, e.g. `internal/keylinks` | Mirrors `internal/surfaces` / `internal/openaiurl`; stdlib-only, isolated | ✓ |
| Bare `_test.go`, no production package | Smallest footprint; logic buried in a test file | |
| You decide | | |

---

## Reassessment Scope & Archived Plans

**Q1 — What gets normalized?**

| Option | Description | Selected |
|--------|-------------|----------|
| All 38, repo-wide | One sweep, clean tree, no exclusion list | ✓ |
| Only v0.13.x Phases 1–2's 25 | Exactly what the REQ names; needs the other 13 excluded somehow | |
| None — record findings, don't edit archived plans | Preserves history exactly; leaves the REQ unmet | |

**Notes:** With the escaping check running everywhere, a partial sweep forces either a
permanently-red guard or an exclusion list — and an exclusion list is how a gate quietly
stops gating.

**Q2 — How to handle editing shipped, archived PLAN.md files?**

| Option | Description | Selected |
|--------|-------------|----------|
| Edit in place, one commit, rationale in the message | Git carries before/after; no parallel bookkeeping | ✓ |
| Edit in place + a note in each touched plan | More honest at a glance; 38 pieces of prose into `.planning/**` | |
| You decide | | |

**Q3 — What counts as "genuinely pinned"?**

| Option | Description | Selected |
|--------|-------------|----------|
| Corrected pattern re-run and matching at HEAD | Match = pinned; no match = recorded unpinned with reason | ✓ |
| A real Go test must exist that fails on regression | Strongest reading; auditing 25 links for coverage is a phase of its own | |
| You decide | | |

**Q4 — Where does an "unpinned" verdict live?**

| Option | Description | Selected |
|--------|-------------|----------|
| A table in this phase's VERIFICATION.md | Self-contained, reviewable in the phase PR | ✓ |
| `.planning/WINDOWS.md` as broken windows | Would file permanent open debt against shipped work nobody plans to revisit | |
| GitHub issues, one per unpinned gate | Durable and actionable; risks a burst of low-value issues | |

**Q5 — Repair unpinned gates in this phase?**

| Option | Description | Selected |
|--------|-------------|----------|
| No — record only | The REQ says "reassessed", not "repaired" | ✓ |
| Yes, if it's cheap | "Cheap" judged mid-execution with no boundary | |
| Record, and file follow-ups for the important ones | Bounded, doesn't drop a real finding | |

---

## Qdrant CI Mitigation

**Q1 — Which mitigation?**

| Option | Description | Selected |
|--------|-------------|----------|
| One shared Qdrant via `ENGRAM_QDRANT_TEST_ADDR` | 4 containers → 1; `TestMain` already honors the env var; needs collection namespacing first | ✓ |
| Serialize the Qdrant-backed packages | No Go changes, no shared-state risk; lengthens the critical path (546 tests) | |
| Buy headroom — larger runner | Smallest diff; treats the symptom, costs per run, returns with a 5th package | |

**Q2 — How is the container's exit reason captured?**

| Option | Description | Selected |
|--------|-------------|----------|
| CI post-step on failure: `docker inspect` + logs + `dmesg` | Works when the Go process is gone; answers the OOM hypothesis directly | ✓ |
| In-Go: capture logs/state in `TestMain` teardown | Runs locally too; likely yields nothing in the case that matters most | |
| Both | Most coverage; two mechanisms to maintain | |

**Q3 — How to namespace collections?**

| Option | Description | Selected |
|--------|-------------|----------|
| Per-package constant prefix | Deterministic, greppable, minimal diff; fixes the actual collision | ✓ |
| Per-test unique names (`t.Name()` / random) | Strongest isolation; wide diff across 546 tests, changes cleanup semantics | |
| You decide | | |

**Q4 — Which packages move onto the shared instance?**

| Option | Description | Selected |
|--------|-------------|----------|
| All four | All already read the env var; partial adoption leaves pressure and two CI behaviors | ✓ |
| Only store + server (the heavy two) | Smaller namespacing diff; leaves 3 containers where 1 would do | |
| You decide | | |

**Q5 — Does the testcontainer fallback stay?**

| Option | Description | Selected |
|--------|-------------|----------|
| Keep it — local dev unchanged | Existing precedence already correct; `ENGRAM_REQUIRE_QDRANT` keeps CI fail-closed | ✓ |
| Remove it — require an address everywhere | One code path; real ergonomics regression for developers | |

**Q6 — How is the fix proven?**

| Option | Description | Selected |
|--------|-------------|----------|
| Assert the mechanism, not the absence | One container, one shared address, disjoint collection prefixes | ✓ |
| Repeated green CI runs as evidence | ~3 hits in 2 hours means N must be large; a streak never proves absence | |
| Both | Mechanism assertions plus repeat runs | |

---

## Claude's Discretion

No question was answered "you decide" — every option resolved explicitly. Planning judgment
remains open on: the guard leaf package's final name (`internal/keylinks` indicative), the
exact prefix strings for collection namespacing, and whether the shared Qdrant is a GitHub
Actions `services:` container or a plain boot step.

## Deferred Ideas

- Fixing `parseMustHavesBlock` in gsd-core (root cause; reported not fixed — filing vs
  spine-tracking is Sean's call per precedent `cvvrwjbsnz`)
- Repinning any v0.13.x gate the sweep finds unpinned
- Per-test collection isolation (`t.Name()` / random suffixes) for intra-package parallelism
- Pinned-commit resolution for archived key-links (`from` file resolved at ship commit)
- `research-versioned-payload-migration-mechanism` todo — reviewed, not folded; already
  scoped to Phases 2–4 per STATE.md
