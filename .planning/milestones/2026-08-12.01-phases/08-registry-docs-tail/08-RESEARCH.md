# Phase 8: Registry & Docs Tail - Research

**Researched:** 2026-08-21
**Domain:** internal Go conformance-gate registry (`internal/surfaces`) + docs-site/skill/CLAUDE.md documentation
**Confidence:** HIGH — every mechanical claim below is grounded in a file:line read this session; no
new external package, no new runtime surface, no unverified library claim.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01 — Rule Registration Scope.** The registered rule covers **all three** sweep sites, not the
  two the ROADMAP names: `cmd/engram/summarize.go:39`, `cmd/engram/spine_review_scan.go:44`, and
  `cmd/engram/spine_review_verify.go:623` all carry the byte-identical string
  `"--scope <scope> or --all-scopes is required"`. `REQ-sweep-scope-rule-registered` and issue #480
  both name only the first two. Treat "three sites" as a *finding to re-verify at plan time*, not a
  fixed count — the acceptance gate must assert **zero** remaining hand-rolled occurrences of the
  string, never "three converted". Reversibility: costly (anchors regenerate across docs-site, the
  skill, and CLI usage strings).
- **D-02 — Canonical Sentence Wording.** The rule's `Sentence` is **flag-shaped**, following the
  `RulePurgeFilterRequiresScope` precedent (`"the free-form filter path requires an explicit --scope
  or --all-scopes: ..."`), not the wire-neutral majority convention. This rule is CLI-only — no MCP
  tool exposes an `all_scopes` field at all — so there is no wire-neutral audience to serve.
  `TagForm` stays **empty**, same reasoning as `RuleDestructiveRequiresApply`/`RuleVerifyFailOnValues`/
  `RulePurgeFilterRequiresScope`: no MCP arg struct carries this field set. `Sentence` must be
  **ASCII-only** (`validateRuleSet` enforces this by plain byte comparison across five surfaces).
- **D-03 — Documentation Depth.** The reference pages document the **whole milestone state
  vocabulary**, not just `schema_version`: `schema_version` **plus** the derived `expired`/
  `scheduled` words and their asymmetric boundary rule:
  - `not_before == now` → **ACTIVE** (inclusive `Lte` gate; no `scheduled` word)
  - `not_after == now` → **ALREADY EXPIRED** (exclusive `Gt` gate; emits `expired`)
  - `expired` suppresses `scheduled`; mutually exclusive by a write-time invariant
  - canonical order: `archived, superseded, expired, scheduled` (descending finality)
  This prose must be **derived from the store's gate** (`internal/store/store.go`'s
  `activeWindowConditions`), never re-reasoned from existing prose — two independent
  implementations (`cmd/engram/memory_state.go`, `ui/src/lib/memorystate.ts`) already agree with it;
  a docs page is a third surface.
- **D-04 — Migration Guide Shape.** A new standalone `docs-site/src/content/docs/guides/migrate.md`,
  mirroring `guides/reindex.md`'s shape (`The migration flow` → `Flags` → `Output` →
  recovery/edge cases → `See also`). Scope is strictly the schema-version-driven mechanism
  (`engram migrate`, `engram migration-status`, the version stamp and its histogram) — explicitly
  excludes `migrate-remap-owner`/`summarize-missing`/`reindex`. Rejected: grafting into
  `guides/upgrade.md` (release-note context, not an evergreen operator procedure); `upgrade.md`
  should **link** to the new guide instead.
- **D-05 — CLAUDE.md Audit.** A **full audit** against this milestone, not just the SC3 line:
  1. Line 70 `Not used here: database migrations, viper, cocogitto.` — revise to state what this
     milestone ships and its deliberate scope (schema-version-driven migrations only, *deliberately
     not* `migrate-remap-owner`/`summarize-missing`/`reindex`).
  2. The `cmd/engram/` layout row — lists five operator commands and omits `migrate`,
     `migration-status`, `get`, and `spine-review`, all shipped this milestone.
  3. The Memory contract section — re-checked for `schema_version` and the
     `archived`/`superseded`/`expired`/`scheduled` vocabulary.
  Constraint: CLAUDE.md is normative routing, not a changelog — additions state the *current*
  contract, never narrate what changed or when.

### Claude's Discretion

- The anchor surface set is **derived, not chosen** — falls out of `ApplicableSurfaces(rule,
  exposed)` from the rule's `Fields`/`SurfaceFields`. Do not hand-pick the target list to match
  issue #480's prediction (`guides/cli.md`, `curating-memory/SKILL.md`, `reference/tools.md`); if
  the derived set differs, the derived set is correct and the divergence is worth a line in the
  summary.
- Whether `SurfaceFields` needs to diverge from `Fields` — determine empirically (see
  `## ApplicableSurfaces Resolution` below; this research found a genuine over-resolution case).
- Plan decomposition and wave ordering — registration and docs tracks are parallelizable with one
  constraint: docs prose quoting the canonical sentence must follow registration, since
  `surfacesgen` writes those regions from the registry.
- Exact section placement and heading text within each docs page.

### Deferred Ideas (OUT OF SCOPE)

- Folding `cmd/engram/client_common.go:286`'s `"--scope is required unless --cross-spine is set"`
  onto the already-registered `RuleScopeRequiredUnlessCrossSpine` — same defect class as #480 but a
  **different rule**, on the client read lane. Worth its own issue.
- `cmd/engram/client_store.go:48` (`"--scope is required"`) — a third, simpler unregistered scope
  guard; assess alongside the above, may or may not warrant a rule.
- Porting `search`/`list` off `renderMemoryTable` onto Phase 6's view mechanism.
- Exposing the Phase 7 opt-in recall flags on the MCP tool schemas.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-sweep-scope-rule-registered | The `--scope`-or-`--all-scopes` requirement shared by `summarize-missing` and `spine-review scan` is a registered `surfaces.ConditionalRule` rather than a hand-rolled `usageErrorf`, with its canonical sentence anchored on every surface its fields resolve to (#480). | `## Rule Registry Mechanics`, `## The Generator Pipeline`, `## The Conformance Gate`, `## ApplicableSurfaces Resolution`, `## The Three Conversion Sites` |
| REQ-docs-record-state | `reference/memory-record.md` and `reference/tools.md` document the full record state including `schema_version`, and the migration mechanism has an operator-facing guide. | `## Record-State Authority`, `## Docs-Site Mechanics`, `## The Migration Guide's Subject Matter` |
| REQ-claude-md-migrations-convention | CLAUDE.md's Conventions line "Not used here: database migrations" is revised to describe what this milestone actually ships, so the normative doc does not contradict the code. | `## CLAUDE.md Audit Findings` |
</phase_requirements>

## Summary

This is a mechanics-verification phase, not a technology-selection phase — every decision was
already locked in CONTEXT.md. The research below verifies the registry/generator/gate machinery the
plan must drive correctly, and surfaces two findings CONTEXT.md's own text anticipated but did not
pin: (1) the new rule's `Fields` set genuinely **over-resolves** onto `skill/engram/skills/
curating-memory/SKILL.md` for a reason that has nothing to do with the sweep commands — a raw
substring artifact of the pre-existing `purge-filter-requires-scope` anchor's rendered text — and
the planner must treat this as the correct, if surprising, derived surface set rather than a bug to
route around; and (2) `reference/memory-record.md`'s "Archiving" section (lines 73-79) makes a now
**false** claim that the Connect `Memory` message does not carry `superseded_by`/`supersedes`/
`not_before`/`not_after` — REQ-connect-record-state-parity (#482) shipped all of these onto
`proto/engram/v1/engram.proto`'s `Memory` message (fields 23-28) earlier in this milestone, so this
paragraph is now stale documentation of the exact kind D-03 exists to fix, beyond what CONTEXT.md's
text called out by name.

**Primary recommendation:** Register `RuleSweepScopeOrAllScopesRequired` as a struct literal appended
to `internal/surfaces/rules.go`'s `rules` slice (mirroring `RulePurgeFilterRequiresScope` field-for-
field), hand-author empty `<!-- engram:rule:start/end -->` anchor pairs in the docs-site/skill target
files the `ApplicableSurfaces` computation below predicts, add a `ruleTargets` entry in
`internal/surfacesgen/main.go`, then run `task surfaces:gen` (never `go run ./internal/surfacesgen`
alone — it also chains `proto:gen` and regenerates the `--help`/catalog goldens under
`cmd/engram/testdata/`) to fill the anchors and the three converted CLI call sites' `Usage` strings.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Sweep scope-or-all-scopes constraint | CLI (`cmd/engram/`) | Registry leaf (`internal/surfaces`) | The rejection fires in three `cobra.Command.RunE` bodies; the registry leaf exists solely so those three sites (and their prose mirrors) compose one declared value instead of re-typing it |
| Anchored-region generation | Generator (`internal/surfacesgen`) | Docs-site / Skill (prose) | The generator is the only writer of anchor bodies; docs-site/skill files are read-only targets that must pre-exist with empty anchor pairs before generation |
| Conformance gate | Go test (`internal/surfaces` package) | CI (`surfaces` job, `.github/workflows/ci.yaml`) | The gate is a `go test` assertion; CI re-runs generation and diffs against the committed tree so a forgotten `task surfaces:gen` fails the PR, not silently merges |
| Record-state vocabulary | Store (`internal/store/store.go`) | Docs-site (reference pages) + CLI (`memory_state.go`) + UI (`memorystate.ts`) | The store's `activeWindowConditions` is sole authority for the boundary semantics; every other surface (docs included) must derive from it, never restate independently |
| Migration mechanism | CLI (`cmd/engram/migrate_family.go`) | Docs-site (new guide) | `engram migrate`/`migrate status`/`migrate revert` already exist and are tested; the guide is pure documentation of existing, unchanged behavior |
| CLAUDE.md accuracy | Docs (routing doc) | — | No code tier — CLAUDE.md is prose describing the `cmd/engram/` and memory-contract surfaces, which this phase does not change |

## Rule Registry Mechanics

`internal/surfaces/rules.go` is a stdlib-only leaf package (`internal/surfaces/rules.go:9-13`) that
declares `ConditionalRule` values once, composed by every surface that states the constraint. Key
mechanics verified this session:

- **Registration is purely additive.** Appending a struct literal to the `rules` slice
  (`internal/surfaces/rules.go:180-266`) is the entire registration step — `ruleByID`, `Rules()`,
  `RuleByID`, and `validateRuleSet` all derive from the slice
  (`internal/surfaces/rules.go:268-286`). Nothing else needs touching to register.
- **`declared: true` is compiler-enforced provenance**, not a runtime check
  (`internal/surfaces/rules.go:97-108`). The field is unexported; no package outside
  `internal/surfaces` can set it, so an off-registry `surfaces.ConditionalRule{...}` literal built
  elsewhere always carries `declared: false` and fails `conditionalErrf`'s `IsDeclared()` check
  (`internal/server/conditionalerr.go:36-44`).
- **`ValidateRules()`/`validateRuleSet`** (`internal/surfaces/rules.go:288-347`) rejects: empty ID,
  empty Sentence, empty Fields, an empty-string `SurfaceFields` entry, a duplicate ID, a non-ASCII
  byte in a Sentence, or **one rule's Sentence being a substring of another's**
  (`internal/surfaces/rules.go:337-345` — `strings.Contains(a.Sentence, b.Sentence)`). **Correction
  to a common misreading:** this substring check runs on **Sentence text**, not on rule **ID**
  strings — there is no code-enforced ID-substring-collision check in this package (verified via
  `rg -n "substring" internal/surfaces/*.go`, three hits, all Sentence-scoped). `Hint` is **not**
  validated for non-emptiness despite some doc-comment prose implying otherwise — confirmed by
  reading the full body of `validateRuleSet`.
- **The closest precedent, `RulePurgeFilterRequiresScope`** (`internal/surfaces/rules.go:150-162`,
  literal `internal/surfaces/rules.go:257-265`): `Hint: "conditional_required"`, `TagForm` empty,
  `Fields: []string{"scope", "category", "tags", "older-than"}`, no `SurfaceFields` override. Its
  three anchored regions live at `skill/engram/skills/curating-memory/SKILL.md:388`,
  `docs-site/src/content/docs/reference/tools.md:128`, and
  `docs-site/src/content/docs/guides/cli.md:309` (all confirmed by direct read this session).

**Candidate registration** (naming inferred from Success Criterion 1's exact identifier
`RuleSweepScopeOrAllScopesRequired`; the ID string itself is not pinned by CONTEXT.md — the
`Pascal→kebab` mapping every other rule follows, e.g. `RulePurgeFilterRequiresScope` →
`"purge-filter-requires-scope"`, predicts `"sweep-scope-or-all-scopes-required"`, but confirm no
Sentence-substring collision with `RulePurgeFilterRequiresScope`'s sentence at `validateRuleSet`
time before finalizing wording — both discuss `--scope`/`--all-scopes` and a careless phrasing could
trip the substring check):

```go
// internal/surfaces/rules.go — append to the rules slice
{
    ID:       RuleSweepScopeOrAllScopesRequired,
    Sentence: "an explicit --scope or --all-scopes is required", // ILLUSTRATIVE — exact wording is the planner's/executor's call under D-02; verify no Sentence-substring collision with RulePurgeFilterRequiresScope
    Fields:   []string{"scope", "all-scopes"},
    Hint:     "conditional_required", // inferred from the closest precedent (RulePurgeFilterRequiresScope); not pinned by CONTEXT.md
    declared: true,
},
```

## The Generator Pipeline

`internal/surfacesgen/main.go` is the **only** generator (`task surfaces:gen` → `go run
./internal/surfacesgen`, `Taskfile.yaml:245-249`). Its content source is exclusively
`surfaces.Rules()` — no env var, no flag, no file content beyond the anchored targets it rewrites
(`internal/surfacesgen/main.go:6-9`). Mechanics that answer the orientation's central question:

- **The generator does NOT discover anchors.** It is driven entirely by a **hand-maintained Go map**,
  `ruleTargets map[string][]target` (`internal/surfacesgen/main.go:44-133`), mapping each declared
  rule ID to every file path carrying its anchor pair, plus a `surfaceKind` (`kindMarkdown` or
  `kindProto`) controlling how the Sentence is rendered (bare text vs. `// `-prefixed for `.proto`).
- **`WriteRegion` (`internal/surfaces/anchor.go:164-206`) errors if the anchor pair does not already
  exist** in the target file — "start anchor for rule %q not found" / "end anchor for rule %q not
  found". This directly answers the orientation's central question: **the anchor region is
  hand-created ONCE, then machine-maintained.** A human (executor) must first write
  `<!-- engram:rule:start <ID> --><!-- engram:rule:end <ID> -->` (with the body between them left
  empty or with placeholder text) into every target file, THEN add a `ruleTargets` entry, THEN run
  `task surfaces:gen` to have `WriteRegion` fill the body with the rendered Sentence.
- **Author's required steps, precisely, for this phase:**
  1. Append the rule struct literal to `rules.go`'s `rules` slice (with an exported `Rule…` const).
  2. Hand-write an empty (or placeholder) `engram:rule:start <ID>` / `engram:rule:end <ID>` anchor
     pair into each target file `ApplicableSurfaces` predicts (see below) — HTML-comment flavor for
     markdown, `//`-line-comment flavor for `.proto` (not applicable here — see below).
  3. Add a `ruleTargets[RuleSweepScopeOrAllScopesRequired]` entry in
     `internal/surfacesgen/main.go` naming those same files with `kindMarkdown`.
  4. Run `task surfaces:gen` (not the bare `go run ./internal/surfacesgen` invocation) — this task
     ALSO chains `task: proto:gen` (buf generate + vendored TS client re-copy) and regenerates the
     pinned `--help`/catalog goldens via `go test ./cmd/engram -run 'TestHelpGolden|TestCatalogGolden'
     -update -count=1` (`Taskfile.yaml:245-263`). Both side effects apply to this phase even though
     the new rule has no proto anchor: the `--help` text for the three converted commands' `--scope`/
     `--all-scopes` flags changes if the executor composes `rule.Sentence` into their `Usage`
     strings, which dirties `cmd/engram/testdata/help.golden` and `catalog.golden`.
  5. `WriteRegion` rewrites **atomically** (temp file + `os.Rename`,
     `internal/surfaces/anchor.go:210-231`) and preserves the anchor's own leading whitespace for
     multi-line bodies — not relevant here since the Sentence is single-line, but the render is a
     bare sentence with no added punctuation (`internal/surfacesgen/main.go:145-150`), so any
     surrounding punctuation (trailing period, etc.) in the docs prose must sit OUTSIDE the anchor
     pair on the same line — the exact pattern the existing `purge-filter-requires-scope` anchors
     use (e.g. `skill/engram/skills/curating-memory/SKILL.md:388` ends with `...selected<!--
     engram:rule:end purge-filter-requires-scope -->.` — note the period sits after the closing
     anchor comment, not inside the generated body).

## The Conformance Gate

`internal/surfaces/conformance_test.go` (`TestSurfaceConformanceProseFiles`, driven by `runGate`,
`internal/surfaces/conformance_test.go:180-217`) is the gate. Per rule, per surface:

- **`SurfaceDocsSite` / `SurfaceSkill` (prose)** — `checkProseSurface`
  (`internal/surfaces/conformance_test.go:113-133`): for every path under the surface, if an anchor
  pair exists its body must be an **exact** match (`body != rule.Sentence`) for the canonical
  Sentence; **at least one** path under the surface must carry an anchor at all, or the whole surface
  fails with `"no anchored region found in any file under this surface"`. A path with NO anchor is
  not itself a violation — only a surface with ZERO anchors anywhere under it is.
- **`SurfaceProtoComment`** — `checkProtoSurface` (`internal/surfaces/conformance_test.go:151-170`):
  scans **every** proto field comment (across every message) whose **bare, normalized field name**
  matches one of `rule.Fields`, and requires that comment to **contain** (not equal) `rule.Sentence`.
  This is the documented trap: matching is by bare field name across the whole file, not scoped to a
  specific message. **Verified this session: no collision exists for this new rule.** `scope`
  appears as a field name in seven proto messages (`Memory`, `ScopeCount`, `ListMemoriesRequest`,
  `SearchMemoriesRequest`, `SearchDiscoveriesRequest`, `StoreMemoryRequest`,
  `ScheduleMemoryRequest` — `proto/engram/v1/engram.proto` lines 16/61/72/129/178/221/325), but only
  two of those `scope` fields carry a **leading comment** at all (lines 71-72 and 127-129, both
  already anchored for `RuleScopeRequiredUnlessCrossSpine`); the other five have no leading comment
  and so are invisible to `checkProtoSurface`'s comment scan. `all_scopes` does not appear anywhere
  in `proto/engram/v1/engram.proto` (`rg -ni "all[-_]scopes?" proto/engram/v1/engram.proto` → zero
  hits) — no MCP/Connect surface exposes this field at all, matching D-02's own stated reasoning.
  **Net effect: `SurfaceProtoComment` will NOT resolve as applicable for this rule** (see
  `ApplicableSurfaces Resolution` below), so `checkProtoSurface` never runs against it — the trap is
  real in general but does not bite here.
- **`SurfaceCobraUsage` / `SurfaceJSONSchemaTag` / `SurfaceMCPDescription`** are explicitly
  **not checked by this package** (`internal/surfaces/conformance_test.go:212-216`) — they are
  covered by `internal/server/surfaces_test.go` and `cmd/engram/surfaces_test.go` respectively, to
  avoid an import cycle. Since this rule is CLI-only, only the `cmd/engram/surfaces_test.go` side is
  relevant; the plan should check whether that file asserts anything about `--scope`/`--all-scopes`
  Usage-string content, though `TestEveryScopeAllScopesPairHasAFlagGroup`
  (`cmd/engram/flaggroup_test.go:490-524`, see below) is the one confirmed test that touches this
  exact flag pair's Usage text.
- **`assertRuleAppliesSomewhere`** (`internal/surfaces/conformance_test.go:90-97`) independently
  fails the gate if a rule resolves to **zero** applicable surfaces across ALL SIX — this is the
  test the orientation flagged as worth checking; it means an under-specified `Fields` set (e.g. a
  typo) fails loudly rather than silently passing nowhere.

## ApplicableSurfaces Resolution

`ApplicableSurfaces(rule, exposed)` (`internal/surfaces/normalize.go:72-88`) resolves a rule onto a
surface only if **every** field in `SurfaceApplicabilityFields(rule)` (= `SurfaceFields` if set,
else `Fields`) is present, after `NormalizeField` (strip leading `--`, `-`→`_`, lowercase,
`internal/surfaces/normalize.go:52-61`), in that surface's exposed field set. For prose surfaces,
`buildProseExposed` (`internal/surfaces/conformance_test.go:63-73`) derives "exposed" by **raw
substring scan of the live file content** (`exposedFileFields`,
`internal/surfaces/conformance_test.go:38-56`) — not a structured parse. This has a real consequence
for this rule, verified directly against the live tree (state as of this research session, before
any Phase 8 edits):

**Predicted resolution for `Fields: ["scope", "all-scopes"]`:**

| Surface | Files under it | `scope` present? | `all_scopes`/`all-scopes` present? | Resolves applicable? |
|---|---|---|---|---|
| `SurfaceDocsSite` | `docs-site/reference/tools.md`, `docs-site/guides/cli.md` | yes (both, pervasive) | yes — `cli.md:161-249` genuinely documents all three sweep commands' `--scope`/`--all-scopes`; `tools.md:512-520` documents `summarize-missing`'s CLI usage block with the identical `(--scope <scope> \| --all-scopes)` shape | **YES — genuine, substantive match** |
| `SurfaceSkill` | `skill/curating-memory/SKILL.md`, `skill/discovering/SKILL.md` | yes (curating-memory, pervasive) | yes, but **only inside `curating-memory/SKILL.md:388`'s existing `purge-filter-requires-scope` anchor** — no other, independent mention of `--all-scopes` in either skill file | **YES — but this is an over-resolution artifact, verified genuine** |
| `SurfaceProtoComment` | `proto/engram/v1/engram.proto` | yes (pervasive) | **no** — zero occurrences anywhere in the file | **NO — correctly not applicable** |
| `SurfaceCobraUsage` / `SurfaceJSONSchemaTag` / `SurfaceMCPDescription` | (not evaluated by this package's gate) | — | — | out of scope for `internal/surfaces`' own test; handled by the CLI-side conversion directly |

**The `SurfaceSkill` over-resolution is real and confirmed, not a false alarm.** Verified this
session: `rg -ni "spine-review\|summarize-missing" skill/engram/skills/curating-memory/SKILL.md
skill/engram/skills/discovering/SKILL.md` returns exactly **one** hit — a passing mention of
`engram spine-review purge --apply` at `curating-memory/SKILL.md:382`, in the context of purge's
own contract, with zero mentions of `scan`, `verify`, or `summarize-missing` anywhere in either
skill file. The word `--all-scopes` in `curating-memory/SKILL.md` occurs **exactly once**, inside
the *rendered text of the unrelated `purge-filter-requires-scope` anchor* at line 388. Since
`exposedFileFields` is a raw content scan with no awareness of which rule "owns" a given occurrence,
this pre-existing text is enough to make `ApplicableSurfaces` claim `SurfaceSkill` is applicable —
even though neither skill file currently discusses the sweep commands this new rule actually
constrains.

Per CONTEXT.md's own instruction ("if the derived set differs, the derived set is correct"), **this
is not a bug to route around** — it means the conformance gate WILL require an anchored region for
the new rule to exist somewhere under `SurfaceSkill` (satisfied by either skill file; `checkProseSurface`
only requires `found > 0` across the surface's paths, not every path). The planner's genuine choice
is where to place a **substantive** sentence, not whether to place one: `curating-memory/SKILL.md:382`
already discusses `spine-review purge --apply`'s deletion contract in the same paragraph family — a
natural, non-degenerate extension is to broaden that mention to also name `spine-review scan`/
`verify`/`summarize-missing`'s own scope requirement, giving the anchor a genuine home rather than a
mechanically-forced one. `discovering/SKILL.md` has no comparable natural anchor point (it never
discusses operator sweep commands at all) and is not a good target.

**`SurfaceFields` does not resolve this over-resolution** — the field set is identical either way
(`scope`, `all-scopes`); the issue is that raw substring matching cannot distinguish "this file
discusses the rule's actual constraint" from "this file happens to contain both words for an
unrelated reason." `SurfaceFields` is designed to disambiguate a field SHARED across differently-
behaving struct types (the `RuleDiscoveryNotSchedulable` precedent,
`internal/surfaces/rules.go:203-217`), not to filter out an incidental prose collision — there is no
mechanism in this package for the latter. This is worth a one-line note in the plan's summary, per
CONTEXT.md's own discretion guidance.

**Predicted final anchor set: 3 files** — `docs-site/src/content/docs/reference/tools.md`,
`docs-site/src/content/docs/guides/cli.md`, `skill/engram/skills/curating-memory/SKILL.md` —
matching issue #480's own prediction and the `purge-filter-requires-scope` precedent's 3-file
footprint exactly, though for a subtly different reason on the skill-file leg than #480 likely
assumed. No proto anchor (unlike `RuleScopeRequiredUnlessCrossSpine`, which has one) — matching the
`RuleDestructiveRequiresApply`/`RuleVerifyFailOnValues` precedent of "no proto anchor when the field
genuinely isn't a proto field" (`internal/surfacesgen/main.go:100-118`).

## The Three Conversion Sites

All three sites currently hand-roll the identical string via a bare `usageErrorf` call
(`cmd/engram/summarize.go:39`, `cmd/engram/spine_review_scan.go:44`,
`cmd/engram/spine_review_verify.go:623` — `usageErrorf("--scope <scope> or --all-scopes is
required")`). Confirmed via `rg -n "scope <scope> or --all-scopes is required"` across the whole
repo (excluding an irrelevant archived planning doc under `docs/superpowers/plans/`): **exactly 3
live occurrences**, matching D-01's finding precisely. `spine_review_scan.go`'s own `RunE` even
carries a comment naming this exact follow-up: *"NOT because this check satisfies an already-
registered surfaces.ConditionalRule: summarize.go's own check is a bare usageErrorf, and the
registry ... holds no rule for it. See this plan's SUMMARY ... for the follow-up that would register
one."* (`cmd/engram/spine_review_scan.go:36-42`) — i.e. this phase is the literal completion of a
TODO an earlier phase left in the source.

**The exact CLI-side conversion template** — already proven in production at
`cmd/engram/spine_review_purge.go:79-91` for `RulePurgeFilterRequiresScope` (this is the CLI
equivalent of `internal/server.conditionalErrf`; `cmd/engram/*.go` production files are import-
boundary-denylisted from `internal/server`, per `internal/surfaces/rules.go:9-13`'s doc comment, so
this hand-written pattern — not `conditionalErrf` — is what the three sweep sites must use):

```go
// cmd/engram/spine_review_purge.go:79-91 — the proven template
func requirePurgeFilterScope(opts store.PurgeOptions) error {
    if opts.Scope != "" || opts.AllScopes {
        return nil
    }
    if !store.PurgeFilterPathActive(opts) {
        return nil
    }
    rule, ok := surfaces.RuleByID(surfaces.RulePurgeFilterRequiresScope)
    if !ok {
        panic("spine-review purge: surfaces.RulePurgeFilterRequiresScope is not registered in internal/surfaces/rules.go")
    }
    return usageErrorf("%s", rule.Sentence)
}
```

Applied to the three sites, each `if <scope> == "" && !<allScopes> { return usageErrorf("--scope
<scope> or --all-scopes is required") }` block becomes a `surfaces.RuleByID` lookup + panic-on-
missing + `usageErrorf("%s", rule.Sentence)`. **Import note:** `spine_review_verify.go` already
imports `internal/surfaces` (for `RuleVerifyFailOnValues`, confirmed at
`cmd/engram/spine_review_verify.go:22` and its own conversion at `spine_review_verify.go:669-673`);
`summarize.go` and `spine_review_scan.go` do **not** yet import it and will need the import added.

**Usage-string composition (optional but precedented):** `spine_review_purge.go:426-441`
additionally composes `scopeRule.Sentence` directly into three of its OTHER flags' `Usage` strings
(`--category`, `--tags`, `--older-than`), reached via a second `surfaces.RuleByID` lookup in `init()`
with the identical panic guard. The three sweep sites' `--scope`/`--all-scopes` flags currently carry
plain, rule-independent Usage text (`"sweep every scope (required if --scope is omitted); mutually
exclusive with --scope"` — identical wording at `summarize.go:116`, `spine_review_scan.go:159`,
`spine_review_verify.go:666`). Composing the new rule's Sentence into these Usage strings is
consistent with precedent but not required by D-01/D-02's own text — the acceptance gate only
concerns the `RunE`-body rejection, not `--help` wording. If the executor DOES change these Usage
strings, remember step 4 of the generator pipeline above: `task surfaces:gen` regenerates
`cmd/engram/testdata/help.golden`/`catalog.golden` to match.

**`cmd/engram/flaggroup_test.go:510`** (`TestEveryScopeAllScopesPairHasAFlagGroup`,
`cmd/engram/flaggroup_test.go:490-524`) asserts two things per command declaring both `--scope` and
`--all-scopes`: (1) a real `cobra` flag group covers the pair via `MarkFlagsMutuallyExclusive`, and
(2) **at least one** of the two flags' `Usage` strings contains the substring `"mutually exclusive"`.
It does **not** assert anything about the "required" half of the sentence. Since the existing
`MarkFlagsMutuallyExclusive` calls and the `"; mutually exclusive with --scope"` Usage substring are
untouched by this phase's conversion, **this test requires no change** regardless of whether the
Usage strings gain the new rule's Sentence. The test's own trailing comment records a **hardcoded
lower bound of 5** commands checked (`scan`, `verify`, `purge`, `consolidate`, `summarize-missing`)
as "a sanity check on the walk itself... not an enumeration the test relies on" — confirms no other
site needs conversion for this test to keep passing.

**Rejection tests that must keep passing (verified content, not just existence):**
- `cmd/engram/spine_review_test.go:54` — `TestSpineReviewScanRequiresScopeOrAllScopes`: asserts only
  `err != nil` and `exitCodeFromError(err) == exitUsage`. No string-content assertion — safe against
  any Sentence wording change.
- `cmd/engram/spine_review_verify_test.go:530` — `TestSpineReviewVerifyRequiresScopeOrAllScopes`:
  identical shape, same safety.
- **`cmd/engram/summarize_test.go` has NO equivalent "neither --scope nor --all-scopes supplied"
  test** — verified by reading the whole file (`cmd/engram/summarize_test.go:1-70`). It has
  `TestSummarizeMissingScopeAndAllScopesRejected` (tests the *mutual-exclusivity* case, both
  supplied together) but nothing pinning the *required* rejection path this phase's conversion
  touches. This is a genuine Wave 0 gap for the Validation Architecture section below — worth adding
  a fourth test mirroring the scan/verify pair's shape, since CONTEXT.md's own canonical_refs list
  names only two rejection tests to protect and this research found the third site (`summarize.go`)
  has no test pinning that behavior at all today.

## Docs-Site Mechanics

- **Sidebar autogenerates from directory** — confirmed at `docs-site/astro.config.mjs:28-35`:
  `{ label: 'Guides', items: [{ autogenerate: { directory: 'guides' } }] }` (Starlight's
  `autogenerate` form, nested under `items`, not a sibling of `label` — the config file's own
  comment flags this as a common mistake). A new `guides/migrate.md` file needs **no separate
  sidebar registration** — it appears automatically once created with valid frontmatter.
- **Required frontmatter** — confirmed via `guides/reindex.md:1-3`: a `---`-delimited block with
  `title:` and `description:` keys, nothing else required.
- **Docs lint** — `rumdl check .` (`Taskfile.yaml:89-91`, part of `task lint`'s `lint:markdown` dep)
  runs across the whole repo including `docs-site/**` (no exclusion found in a scan of
  `Taskfile.yaml`/`.licenserc.yaml` for a docs-site-specific rumdl scope). `dprint fmt`/`dprint check`
  (`Taskfile.yaml:104-117`) are the formatting counterpart.
- **Docs-site build gate** — `.github/workflows/docs-site.yaml` runs `pnpm build` (Astro/Starlight
  build) on any PR touching `docs-site/**`; a malformed new page (bad frontmatter, broken internal
  link syntax) fails this job. It does not appear to run a dedicated link-checker beyond what Astro's
  own build performs.
- **`.licenserc.yaml`** excludes `docs-site/**` from the SPDX-header requirement (confirmed at
  `.licenserc.yaml:44`) — the new `guides/migrate.md` needs no license header, consistent with
  CONTEXT.md's own hazard note.

## The Migration Guide's Subject Matter

Verified directly against `cmd/engram/migrate_family.go` and `cmd/engram/client_migration_status.go`
(the mechanism the new guide documents "end to end" per D-04):

- **`engram migrate`** (`cmd/engram/migrate_family.go:248-260`) — "Advance every below-target record
  through the registered `migrate.Registry` step chain (today: v0→v1, minting a `short_id` for any
  record that lacks one)." Routed through `registerDestructive` (`migrate_family.go:640`) — same
  preview-by-default/`--apply`-to-mutate idiom as every other destructive operator command, NOT the
  `--dry-run` idiom `reindex`/`summarize-missing` use. `--apply` re-derives eligibility within its
  own run (a fresh preview inside the apply closure) and migrates only the intersection of what it
  showed and what is still eligible — a record that became ineligible since preview is spared, one
  that became newly eligible is reported "appeared" but not migrated (re-run to include it). Output
  shape: `migrateOutputDoc` (`migrate_family.go:86-96`) — `target`, `dry_run`, `would_migrate`,
  `migrated`, `failed`, `passes`, `backlog`, `spared`, `appeared`, all scalar counts (never id lists).
- **`engram migrate status`** (`migrate_family.go:268-286`) — read-only, reports
  `Store.MigrateStatus`'s server-side version-distribution histogram. Output shape:
  `migrateStatusReportDoc` (`migrate_family.go:296-311`) — `buckets`, `absent`, `future`,
  `future_total`, `total`, `current_version`.
- **`engram migrate revert --to <v>`** (`migrate_family.go:605-618`) — reverse-walks records above
  `--to` back down, "refusing any irreversible range whole" rather than partially reverting. Also
  routed through `registerDestructive` (`migrate_family.go:641`). Output shape: `revertOutputDoc`
  (`migrate_family.go:365-374`) — `to`, `applied`, `reversible`, `candidates`, `reverted`, `failed`,
  `passes`, `backlog`, `refusal` (populated with the exact `store.RevertRefusalError(plan).Error()`
  text on an irreversible refusal, empty otherwise). **In scope for the new guide** — confirmed via
  `.planning/REQUIREMENTS.md:38`, `REQ-migrate-revert` is `[x]` Complete, Phase 4, and the "Out of
  Scope" table (`REQUIREMENTS.md:75`) explicitly distinguishes "reverting an irreversible step"
  (out of scope, refused by design) from "reverting *reversible* steps IS in scope."
- **`engram migration-status`** (`cmd/engram/client_migration_status.go:18-66`) — the client-tier
  (Connect RPC) sibling of `migrate status`, reaching the same histogram over the wire for an
  operator with only a server URL and token, no direct Qdrant access. Takes no arguments
  (`cobra.NoArgs`).
- **Template to mirror** — `docs-site/src/content/docs/guides/reindex.md` is confirmed structured as
  `## The migration flow` (numbered steps) → `## Flags` (table) → `## Output` → `## Resuming an
  interrupted run` → `## Repairing a pre-patch resume` → `## See also` (final section, links out).
  D-04 says the new guide should mirror this rhythm; the exact section list is Claude's discretion.
- **`guides/upgrade.md:314-340`** currently carries a `### 12. Records now carry a schema_version
  stamp` release note that says, verbatim, *"there is no `engram migrate` command to run yet, so do
  not look for one"* — this sentence is now **false** (the command shipped in Phase 4 of this same
  milestone) and must be corrected to link to the new guide rather than continue asserting the
  command doesn't exist, per D-04's own instruction that `upgrade.md` should link, not absorb.

## Record-State Authority

`internal/store/store.go`'s `activeWindowConditions` (`internal/store/store.go:1006-1019`) is the
authority, quoted verbatim:

```go
// activeWindowConditions gates recall to records whose validity window is open
// at now: (not_before absent OR <= now) AND (not_after absent OR > now). Stored
// window keys are epoch-second integers; the Range bound is *float64 (Qdrant's
// Range field type). Records with no window match via NewIsEmpty — unchanged
// behavior for every pre-feature record. not_after is exclusive (expires AT it).
func activeWindowConditions(now time.Time) []*qdrant.Condition {
    sec := float64(now.Unix())
    return []*qdrant.Condition{
        qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
            qdrant.NewRange("not_before", &qdrant.Range{Lte: qdrant.PtrOf(sec)}),
            qdrant.NewIsEmpty("not_before"),
        }}),
        qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
            qdrant.NewRange("not_after", &qdrant.Range{Gt: qdrant.PtrOf(sec)}),
            qdrant.NewIsEmpty("not_after"),
        }}),
    }
}
```

This confirms D-03's boundary rule exactly: `not_before` uses inclusive `Lte` (a record with
`not_before == now` is already active — no `scheduled` word), `not_after` uses exclusive `Gt` (a
record with `not_after == now` is already expired). The four hard recall-gate sites pairing
`IsEmpty("superseded_by")`/`IsEmpty("archived_at")` (CONTEXT.md's approximate line numbers ~1091/1097
etc. drift slightly against the live tree; verified exact locations this session: `store.go:1120`
+`1129`, `store.go:1224`+`1228`, `store.go:1391`+`1399`, `store.go:1630`+`1635` — a fifth, non-gate
mention appears in a comment at `store.go:2434`) are confirmed present and, per `store.go:1070-1071`'s
own comment, "Deliberate 2-of-4 scope... appears at four sites in this file... deliberately never
folded."

`cmd/engram/memory_state.go:1-64` is the second independent derivation, and its own doc comment
already states the identical boundary rule in near-CONTEXT.md wording — quoted verbatim:

```go
// Boundary comparisons mirror internal/store's activeWindowConditions
// exactly: not_before uses an inclusive Lte bound (not_before == now yields
// NO "scheduled" word), not_after uses an exclusive Gt bound (not_after ==
// now yields "expired").
```

and its canonical emission order is `archived, superseded, expired, scheduled` (verified by reading
`memoryStateWords`'s body: `archived_at` check first, `superseded_by` second, `expired` third,
`scheduled` last, with `expired` explicitly suppressing the `scheduled` check via an `if !expired`
guard). `ui/src/lib/memorystate.ts` was not read directly this session (TypeScript; out of this
research's Go-centric tool reach) — CONTEXT.md asserts it is "correct as of Phase 7" and the planner
should treat that as `[CITED: CONTEXT.md]` rather than independently re-verified here; if the docs
prose needs a third cross-check, read that file directly at plan or execute time rather than
inferring its content from the Go derivation.

**Docs staleness beyond what CONTEXT.md named:** `docs-site/src/content/docs/reference/memory-
record.md`'s `## Field reference` table (lines 10-35) has **no row at all** for `not_before`,
`not_after`, or `schema_version` — confirmed by reading the full table. The only place these fields
are discussed is the `### Archiving` section's prose (lines 55-84), which contains this now-**false**
claim (quoted verbatim, `memory-record.md:75-79`):

> "...but is **not present on the Connect lane**: `proto/engram/v1/engram.proto`'s `Memory` message
> does not carry `superseded_by`, `supersedes`, `not_before`, or `not_after` either, so this is
> consistent with the shipped Connect contract rather than a new gap. See [GitHub issue #482]
> (https://github.com/seanb4t/engram/issues/482) for the tracked follow-up to add all five fields to
> the Connect lane together."

This is now false: `proto/engram/v1/engram.proto`'s `Memory` message (verified this session,
`proto/engram/v1/engram.proto:13-53`) carries `superseded_by` (field 23), `supersedes` (24),
`not_before` (25), `not_after` (26), `archived_at` (27), and `schema_version` (28) — all shipped by
`REQ-connect-record-state-parity` (#482), which `.planning/REQUIREMENTS.md:43` marks `[x]` complete
in this same milestone. D-03's own text says the docs task extends beyond `schema_version` to "the
rest of what Phases 5-7 shipped" — this paragraph is exactly that kind of staleness and belongs in
the plan's scope for `reference/memory-record.md`, not just the missing `schema_version` row.

## CLAUDE.md Audit Findings

All three D-05 sites confirmed verbatim against the live file:

1. **Line 70** (`CLAUDE.md:70`): `- **Not used here:** database migrations, viper, cocogitto.` —
   confirmed present, unrevised.
2. **The `cmd/engram/` layout row** (`CLAUDE.md:15`): `` | `cmd/engram/` | cobra CLI: `root`,
   `serve`, `version` + operator commands (`reindex` embedder migration — see docs-site
   `guides/reindex`; `migrate-remap-owner`; `prune-expired`; `summarize-missing`;
   `backfill-short-ids`) (entrypoint only) | `` — confirmed missing `migrate` (the whole
   `migrate`/`migrate status`/`migrate revert` family), `migration-status`, `get`, and
   `spine-review` (the whole `scan`/`verify`/`consolidate`/`purge`/`archive`/`restore` family), all
   of which exist in the live `cmd/engram/` tree per the file reads above.
3. **The Memory contract section** — confirmed via `rg -n "schema_version|archived_at" CLAUDE.md`:
   **zero hits for either term anywhere in CLAUDE.md.** The section already documents
   `supersede_memory`/`superseded_by`/`supersedes` and the `schedule_memory`/`list_scheduled`
   `not_before`/`not_after`/expired/scheduled vocabulary in some depth, but never mentions
   `schema_version` or `archived_at`/`spine-review archive` at all — a genuine, complete gap, not a
   staleness-of-existing-text issue for this specific pair.

## Environment Availability

No external tool/service dependency beyond what the repo's existing `task`/`go`/`pnpm` toolchain
already provides — this phase adds no new package, no new runtime dependency, and no new build
target beyond the existing `surfaces:gen`/`proto:gen`/docs-site `pnpm build` chains, all confirmed
present and working in this session's reads.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `go` toolchain | rule registration, generator, tests | ✓ (repo builds/tests today) | per `go.mod` | — |
| `task` (Taskfile) | `task surfaces:gen`, `task lint`, `task test` | ✓ (used throughout this research) | — | — |
| `go tool buf` | `proto:gen` chain inside `surfaces:gen` | ✓ (existing CI job passes) | — | — |
| `pnpm` / Astro / Starlight | docs-site build (`docs-site.yaml` CI) | ✓ (existing CI job passes) | — | — |
| `rumdl` | `task lint:markdown` | assumed ✓ (existing CI `license`/`lint` jobs pass) — not independently invoked this session | — | — |

## Package Legitimacy Audit

**Not applicable.** This phase installs no new external package in any ecosystem — it registers an
in-repo Go struct literal, edits Go source at three existing call sites, and writes/edits markdown
documentation. No `npm install`/`pip install`/`cargo add` of any kind is in scope.

## Common Pitfalls

### Pitfall 1: Running `go run ./internal/surfacesgen` instead of `task surfaces:gen`
**What goes wrong:** The anchors get the correct Sentence text, but `proto/`'s generated code and the
`--help`/catalog goldens under `cmd/engram/testdata/` are NOT regenerated, so CI's `surfaces` job
(`.github/workflows/ci.yaml:272-298`) fails on `git diff --exit-code` over the full path set
(`proto/ docs-site/ skill/ gen/ ui/src/lib/gen/ cmd/engram/testdata/`).
**Why it happens:** The bare binary invocation is what the generator's own `main.go` doc comment
describes; `task surfaces:gen`'s additional chained steps live only in `Taskfile.yaml`, one layer up.
**How to avoid:** Always run `task surfaces:gen`, never the bare `go run` form, whenever a rule or a
Usage string changes.
**Warning signs:** `git status` shows `cmd/engram/testdata/*.golden` or `gen/` files unchanged after
a rule edit that should have touched CLI Usage text.

### Pitfall 2: Anchoring on `skill/discovering/SKILL.md` because it "looks" like a scope-adjacent file
**What goes wrong:** `discovering/SKILL.md` never mentions `spine-review`, `summarize-missing`, or
`--all-scopes` at all (confirmed this session); an anchor placed there has no supporting prose
context and reads as an orphaned sentence.
**Why it happens:** It is the OTHER file under `SurfaceSkill`, and a plan that mechanically lists
"every file under the resolved surface" without reading each one could pick it by default.
**How to avoid:** `checkProseSurface` only requires ONE file under the surface to carry the anchor.
Place it in `curating-memory/SKILL.md`, extending the existing `spine-review purge` mention at line
382 to cover the sibling sweep commands, not in `discovering/SKILL.md`.
**Warning signs:** A generated anchor with no surrounding sentence that references it in context.

### Pitfall 3: Treating the acceptance gate as "three sites converted" instead of "zero occurrences remain"
**What goes wrong:** A gate written as `assert 3 usageErrorf sites converted` passes even if a
fourth hand-rolled site is introduced later (e.g. a future sweep command copy-pastes the string
again), silently reintroducing the exact drift #480 exists to close.
**Why it happens:** Three is the count this research (and CONTEXT.md) found; a plan gate is often
written against the count found at plan time rather than against the invariant.
**How to avoid:** Gate on `rg -n "scope <scope> or --all-scopes is required" cmd/engram/ | wc -l`
(or equivalent) returning **zero**, per D-01's explicit instruction and the specifics section's own
worked example.
**Warning signs:** A gate assertion that hardcodes the number 3 anywhere in its condition.

### Pitfall 4: Rewording the new rule's Sentence to overlap `RulePurgeFilterRequiresScope`'s wording
**What goes wrong:** `validateRuleSet` rejects the WHOLE registry if any rule's Sentence is a
substring of another's (`internal/surfaces/rules.go:337-345`). Since both this new rule and
`RulePurgeFilterRequiresScope` are flag-shaped sentences about `--scope`/`--all-scopes`, a wording
choice like "an explicit --scope or --all-scopes" that happens to appear verbatim inside the other
rule's longer sentence (or vice versa) fails the build.
**Why it happens:** Both rules genuinely share vocabulary by design (D-02 says to follow the purge
rule's style).
**How to avoid:** Run `go test ./internal/surfaces -run TestValidateRules` (or equivalent) as soon as
the literal is drafted, before writing any anchor — this test would already exist per the doc
comment referencing `rules_test.go` cases at lines ~119-175.
**Warning signs:** A build/test failure citing "Sentence contains rule ... Sentence as a substring."

## Code Examples

### Rule struct literal (mirrors `RulePurgeFilterRequiresScope` exactly)
```go
// Source: internal/surfaces/rules.go:257-265 (adapted)
{
    ID:       RuleSweepScopeOrAllScopesRequired,
    Sentence: "<flag-shaped sentence per D-02 — exact wording is plan/executor discretion>",
    Fields:   []string{"scope", "all-scopes"},
    Hint:     "conditional_required",
    declared: true,
},
```

### CLI call-site conversion (mirrors `requirePurgeFilterScope`, `cmd/engram/spine_review_purge.go:79-91`)
```go
// Source: cmd/engram/spine_review_purge.go:79-91 (adapted pattern)
if <cmdScope> == "" && !<cmdAllScopes> {
    rule, ok := surfaces.RuleByID(surfaces.RuleSweepScopeOrAllScopesRequired)
    if !ok {
        panic("<command>: surfaces.RuleSweepScopeOrAllScopesRequired is not registered in internal/surfaces/rules.go")
    }
    return usageErrorf("%s", rule.Sentence)
}
```

### `ruleTargets` generator entry (mirrors `RulePurgeFilterRequiresScope`'s entry, `internal/surfacesgen/main.go:127-135`)
```go
// Source: internal/surfacesgen/main.go:127-135 (adapted)
surfaces.RuleSweepScopeOrAllScopesRequired: {
    {path: "docs-site/src/content/docs/guides/cli.md", kind: kindMarkdown},
    {path: "docs-site/src/content/docs/reference/tools.md", kind: kindMarkdown},
    {path: "skill/engram/skills/curating-memory/SKILL.md", kind: kindMarkdown},
},
```

## State of the Art

Not applicable in the usual "library/framework changed" sense — this is a fully in-repo, custom
registry with no external upstream to track. The one relevant "old approach → current approach" is
internal to this codebase:

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Bare `usageErrorf("literal string")` scope guard, hand-typed at each of 2 (then discovered: 3) call sites | Registered `surfaces.ConditionalRule`, composed via `surfaces.RuleByID` + `usageErrorf("%s", rule.Sentence)` | This phase | A future 4th sweep site copy-pasting the guard fails the "zero occurrences" acceptance gate instead of silently drifting; docs/skill prose stays byte-identical to the enforced rejection by construction |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `RuleSweepScopeOrAllScopesRequired`'s ID string is `"sweep-scope-or-all-scopes-required"` (Pascal→kebab convention inferred from all 8 existing rules) | Rule Registry Mechanics | Low — any valid, unique, ASCII kebab string works; only matters for anchor-marker text consistency with sibling rules, not for correctness |
| A2 | `Hint: "conditional_required"` is the right hint code for this rule | Rule Registry Mechanics | Low-medium — affects the `field=`/`hint=` error envelope's remediation code, not the rejection behavior itself; if wrong, a follow-up one-line literal edit fixes it, no anchor regeneration needed (Hint is not part of any anchored Sentence text) |
| A3 | `ui/src/lib/memorystate.ts` independently agrees with `activeWindowConditions`' boundary semantics, "correct as of Phase 7" | Record-State Authority | Medium — this claim is carried from CONTEXT.md, not independently re-read this session (TypeScript file, outside this Go-focused research pass); if the docs prose is derived from this unverified claim and the TS file has since drifted, the new "third surface" prose would encode the same error. Read the file directly before finalizing the docs prose. |
| A4 | `task lint:markdown`'s `rumdl check .` has no docs-site-specific exclusion/config that would change linting behavior for the new `guides/migrate.md` page | Docs-Site Mechanics | Low — no `.rumdlrc` was located during this session's scan of the repo root and `docs-site/`; if one exists elsewhere, `task lint` would still catch any violation before merge |

**Assumption A3 needs user/executor confirmation before the migration guide's `## Output` /
`## Flags` sections and the reference-page prose are finalized** — everything else in this table is
low-risk and self-correcting within the phase's own gates.

## Open Questions

1. **Exact canonical Sentence wording for the new rule.**
   - What we know: flag-shaped, ASCII-only, must not substring-collide with
     `RulePurgeFilterRequiresScope`'s sentence, `Fields: ["scope", "all-scopes"]`.
   - What's unclear: the literal text. CONTEXT.md deliberately leaves this to D-02's stylistic
     constraint rather than pinning it.
   - Recommendation: draft 1-2 candidate sentences at plan time, run `go test ./internal/surfaces
     -run TestValidateRules` against each before committing to one in the plan's task list.

2. **Whether `curating-memory/SKILL.md:382`'s existing purge-only sentence should be broadened
   in-place, or whether a new adjacent sentence should be added.**
   - What we know: the anchor must land somewhere under `SurfaceSkill`;
     `curating-memory/SKILL.md:382` is the only existing sentence in either skill file that
     discusses any of the three sweep commands' family (`spine-review purge`).
   - What's unclear: whether extending that exact sentence (turning a purge-specific mention into a
     sweep-family mention) reads better than adding a new, separate sentence naming
     `scan`/`verify`/`summarize-missing` explicitly.
   - Recommendation: this is copy-editing discretion, not a mechanical question — leave to the
     executor, but flag in the plan that the anchor's surrounding sentence needs to make sense in
     context, since the anchor itself renders only the bare canonical Sentence with no surrounding
     words supplied by the generator.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib) |
| Config file | none — `go test` driven via `Taskfile.yaml` |
| Quick run command | `go test -short ./internal/surfaces/... ./cmd/engram/...` |
| Full suite command | `task test` (→ `go test ./...` + `uv run --with pytest pytest skill/engram/hooks/tests -q`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-sweep-scope-rule-registered | Rule registered, `declared: true`, passes `validateRuleSet` | unit | `go test ./internal/surfaces -run TestValidateRules -v` | ✅ (`internal/surfaces/rules_test.go`) |
| REQ-sweep-scope-rule-registered | Conformance gate passes for the new rule across resolved surfaces | integration (reads live docs/skill/proto tree) | `go test ./internal/surfaces -run TestSurfaceConformanceProseFiles -v` | ✅ (`internal/surfaces/conformance_test.go`) |
| REQ-sweep-scope-rule-registered | All three sweep sites reject with `exitUsage` on neither `--scope` nor `--all-scopes` | unit | `go test ./cmd/engram -run 'RequiresScopeOrAllScopes' -v` | ✅ for scan/verify (`spine_review_test.go:54`, `spine_review_verify_test.go:530`); ❌ Wave 0 gap for summarize-missing — no existing test |
| REQ-sweep-scope-rule-registered | Zero remaining hand-rolled `usageErrorf` occurrences of the guard string | acceptance (shell, not `go test`) | `rg -c "scope <scope> or --all-scopes is required" cmd/engram/ \| wc -l` → expect `0` | N/A — new gate this phase must add to the plan's verification loop |
| REQ-sweep-scope-rule-registered | `--help`/catalog goldens match live cobra tree after Usage-string changes (if any) | golden | `go test ./cmd/engram -run 'TestHelpGolden\|TestCatalogGolden' -v` (regenerate via `task surfaces:gen` first if failing) | ✅ (`cmd/engram/testdata/help.golden`, `catalog.golden`) |
| REQ-docs-record-state | Reference pages document `schema_version` + window-state vocabulary derived from the store gate | manual (docs review) | N/A — prose correctness is not machine-checkable beyond `rumdl`/build gates | N/A |
| REQ-docs-record-state | New `guides/migrate.md` builds cleanly and appears in the sidebar | build | `cd docs-site && pnpm build` | N/A — page does not exist yet, Wave 0 |
| REQ-claude-md-migrations-convention | CLAUDE.md's three D-05 sites are internally consistent with the live `cmd/engram/` tree and Memory contract | manual (docs review) | N/A | N/A |
| generated-artifact drift (all REQs) | Anchors, proto, goldens, and vendored client match the registry after any rule/Usage change | CI parity | `go run ./internal/surfacesgen && go tool buf generate && git diff --exit-code -- proto/ docs-site/ skill/ gen/ ui/src/lib/gen/ cmd/engram/testdata/` (mirrors `.github/workflows/ci.yaml:291-298`) | ✅ (CI job `surfaces`) |

### Sampling Rate
- **Per task commit:** `go test -short ./internal/surfaces/... ./cmd/engram/...` plus the zero-
  occurrence `rg` gate above.
- **Per wave merge:** `task test` (full Go + python suite) and the `surfaces` job's local
  reproduction (generator + `buf generate` + `git diff --exit-code`).
- **Phase gate:** Full suite green, `task lint` green (`rumdl`/`golangci-lint`/`dprint`/`actionlint`),
  `go test ./internal/keylinks/...` green (per CONTEXT.md's known-hazard note — this guard has broken
  three consecutive phases and is RED at plan time), and `docs-site`'s `pnpm build` green before
  `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] A rejection test for `summarize-missing` mirroring `TestSpineReviewScanRequiresScopeOrAllScopes`/
      `TestSpineReviewVerifyRequiresScopeOrAllScopes` — confirmed missing from
      `cmd/engram/summarize_test.go` this session; the third conversion site currently has zero test
      coverage pinning its "neither flag supplied" rejection.
- [ ] `docs-site/src/content/docs/guides/migrate.md` — does not exist yet; this phase creates it.
- [ ] The zero-occurrence acceptance-gate shell assertion itself — not a `go test`, needs to be
      written into the plan's own verification steps (see Common Pitfalls #3).

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | unchanged — this phase touches no auth path |
| V3 Session Management | no | unchanged |
| V4 Access Control | no | unchanged — the sweep commands' scope/all-scopes gate is an operator-input validation concern, not an authorization boundary (operator tier is Subject-less by design, per `cli.md`'s own `spine-review scan` documentation read this session) |
| V5 Input Validation | yes — unchanged behavior, new expression | The rejection behavior (reject when neither `--scope` nor `--all-scopes` is supplied) is **already enforced today** at all three sites; this phase changes only how the rejection's text/registration is composed, not when it fires (CONTEXT.md's "Out of scope" explicitly excludes runtime-behavior changes) |
| V6 Cryptography | no | unchanged |

### Known Threat Patterns for this stack

No new threat pattern is introduced. This phase performs a pure refactor of an existing input-
validation rejection's construction path (from an inline string literal to a registry-composed
value) plus documentation edits. The three existing rejection tests
(`spine_review_test.go:54`, `spine_review_verify_test.go:530`, and the new summarize-missing test
this research recommends adding) are the correct proof that no behavior regressed — no new STRIDE
category is opened by string-provenance changes to an already-enforced CLI usage error.

## Sources

### Primary (HIGH confidence — direct file reads this session)
- `internal/surfaces/rules.go` — `ConditionalRule` struct, the `rules` registry literal, all 8
  existing rules, `ValidateRules`/`validateRuleSet`.
- `internal/surfaces/normalize.go` — `ApplicableSurfaces`, `SurfaceApplicabilityFields`,
  `NormalizeField`.
- `internal/surfaces/anchor.go` — `ReadRegion`/`WriteRegion`/`scanAnchors`, anchor literal formats.
- `internal/surfaces/conformance_test.go` — `runGate`, `checkProseSurface`, `checkProtoSurface`,
  `assertRuleAppliesSomewhere`.
- `internal/surfacesgen/main.go` — `ruleTargets`, `render`, `run`.
- `cmd/engram/spine_review_purge.go` — the proven CLI-side conversion template
  (`requirePurgeFilterScope`, the `init()` Usage-string composition).
- `cmd/engram/summarize.go`, `cmd/engram/spine_review_scan.go`, `cmd/engram/spine_review_verify.go`
  — the three conversion sites, their `RunE` bodies, flag registration, and existing comments.
- `cmd/engram/flaggroup_test.go` — `TestEveryScopeAllScopesPairHasAFlagGroup`.
- `cmd/engram/spine_review_test.go`, `cmd/engram/spine_review_verify_test.go`,
  `cmd/engram/summarize_test.go` — existing rejection test coverage (and the gap in the third file).
- `internal/store/store.go` — `activeWindowConditions`, the four `IsEmpty` recall-gate sites.
- `cmd/engram/memory_state.go` — the CLI-side state-word derivation.
- `proto/engram/v1/engram.proto` — the `Memory` message field list, `scope`/`all_scopes` occurrence
  scan.
- `docs-site/src/content/docs/reference/memory-record.md` — `## Field reference` table, `###
  Archiving` section (including the now-stale Connect-lane claim).
- `docs-site/src/content/docs/reference/tools.md`, `docs-site/src/content/docs/guides/cli.md` —
  existing sweep-command documentation and anchor placements.
- `docs-site/src/content/docs/guides/reindex.md` — the migration-guide shape template.
- `docs-site/src/content/docs/guides/upgrade.md` — the stale `schema_version` release note.
- `skill/engram/skills/curating-memory/SKILL.md`, `skill/engram/skills/discovering/SKILL.md` —
  scope/all-scopes/spine-review mention scan.
- `docs-site/astro.config.mjs` — sidebar autogeneration config.
- `cmd/engram/migrate_family.go`, `cmd/engram/client_migration_status.go` — migration mechanism
  mechanics.
- `Taskfile.yaml` — `surfaces:gen`, `test`, `lint` targets.
- `.github/workflows/ci.yaml` — `buf`, `surfaces`, `ui-drift`, `license` job definitions.
- `.github/workflows/docs-site.yaml` — docs-site build/deploy pipeline.
- `.licenserc.yaml` — SPDX-header scope exclusions.
- `CLAUDE.md` — the three D-05 audit sites, verbatim.
- `.planning/REQUIREMENTS.md` — REQ-sweep-scope-rule-registered/REQ-docs-record-state/
  REQ-claude-md-migrations-convention, REQ-connect-record-state-parity, REQ-migrate-revert.
- `.planning/phases/08-registry-docs-tail/08-CONTEXT.md` — all locked decisions, discretion areas,
  canonical refs.

### Secondary (MEDIUM confidence)
- None — this research used no web search or third-party documentation; the entire domain is
  in-repo.

### Tertiary (LOW confidence)
- `ui/src/lib/memorystate.ts`'s claimed agreement with `activeWindowConditions` — carried from
  CONTEXT.md's assertion, not independently re-read this session (see Assumption A3).

## Metadata

**Confidence breakdown:**
- Rule registry / generator / conformance-gate mechanics: HIGH — every claim grounded in a direct
  file:line read and, where relevant, a live `rg` scan of the actual repo state.
- ApplicableSurfaces resolution prediction: HIGH for the reasoning and underlying scans; the
  resolved surface SET is a confident prediction from the documented algorithm, not itself a code
  execution — the planner should still let `internal/surfacesgen`/the conformance test be the final
  arbiter once anchors are placed, per CONTEXT.md's own instruction.
- Docs/CLAUDE.md staleness findings: HIGH — every specific claim (missing fields, stale Connect-lane
  paragraph, missing `cmd/engram/` rows, missing schema_version/archived_at mentions) is a direct
  quote-and-cite from a file read this session, not an inference.
- Migration-guide subject matter: HIGH for mechanics (flags, output shapes, exit paths) — the
  guide's actual prose/organization is unavoidably a writing task, not a research one.

**Research date:** 2026-08-21
**Valid until:** This phase should execute promptly against this research — it is a snapshot of a
fast-moving milestone tail (Phases 2-7 landed the exact vocabulary this phase documents in the days
immediately prior). If plan-phase or execute-phase happens more than a few days after this research,
re-run the `rg` scans for the three conversion sites and the `SurfaceSkill`/`SurfaceDocsSite`
exposed-field checks before trusting the predicted anchor set — a same-milestone commit could shift
the underlying prose.
