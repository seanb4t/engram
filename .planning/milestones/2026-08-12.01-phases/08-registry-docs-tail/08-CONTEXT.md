# Phase 8: Registry & Docs Tail - Context

**Gathered:** 2026-08-21
**Status:** Ready for planning

<domain>
## Phase Boundary

The shared sweep `--scope`-or-`--all-scopes` guard becomes a **registered,
conformance-gated `surfaces.ConditionalRule`** instead of a hand-rolled `usageErrorf`, and the
project's normative documentation — docs-site reference pages, a new operator guide, and
CLAUDE.md — describes what milestone `2026-08-12.01` **actually shipped** instead of what it
superseded.

This is the milestone's tail phase (8 of 8). It ships no new runtime capability: every surface it
documents already exists and is already tested. Its two jobs are closing a *registry* gap (#480,
open since v0.13.x Phase 3) and closing a *documentation* gap that Phase 7 explicitly deferred here.

**In scope:**

- Registering `RuleSweepScopeOrAllScopesRequired` in `internal/surfaces/rules.go` and composing it
  onto **all three** sweep leaves that currently hand-roll it, plus the anchored prose regions
  `ApplicableSurfaces` resolves it to, wired through `internal/surfacesgen`.
- `reference/memory-record.md` and `reference/tools.md` documenting the full record-state
  vocabulary this milestone introduced — `schema_version` plus `archived_at`, `superseded_by`, and
  the derived `expired`/`scheduled` words.
- A new operator-facing `docs-site/src/content/docs/guides/migrate.md` covering the
  schema-version-driven migration mechanism end to end.
- A full CLAUDE.md audit against this milestone: the `Not used here` line, the stale
  `cmd/engram/` operator-command inventory, and the Memory contract section's record-state
  vocabulary.

**Out of scope:**

- Any change to the guard's *runtime behavior*. Registration replaces how the constraint is
  **stated and gated**, not when it fires. The three existing rejection tests
  (`spine_review_test.go:54`, `spine_review_verify_test.go:530`, and `summarize`'s equivalent)
  must keep passing on unchanged inputs.
- `client_common.go:286`'s duplication of the already-registered
  `RuleScopeRequiredUnlessCrossSpine` — see Deferred Ideas. It is a genuine second instance of the
  same defect class, but it is a *different rule* on the client read lane Phase 7 just shipped.
- Documenting the wider one-off operator-command family (`migrate-remap-owner`, `prune-expired`,
  `summarize-missing`, `backfill-short-ids`, `reindex`) in the new guide. SC3's own wording draws
  this boundary: the mechanism this milestone shipped is schema-version-driven migrations
  *deliberately not* those.
- Any new runtime surface, RPC, flag, or MCP tool-schema change.

</domain>

<decisions>
## Implementation Decisions

### Rule Registration Scope

- **D-01:** The registered rule covers **all three** sweep sites, not the two the ROADMAP names.

  Scouting found the guard hand-rolled with a **byte-identical** string at three call sites, not
  two: `cmd/engram/summarize.go:39`, `cmd/engram/spine_review_scan.go:44`, and
  `cmd/engram/spine_review_verify.go:623`. `REQ-sweep-scope-rule-registered` and issue #480 both
  name only the first two — `spine-review verify` postdates the issue and was never added to its
  list. Registering two of three would leave an identical unregistered string outside the
  conformance gate, which is precisely the drift #480 exists to close, one site smaller. #480's
  own charter language ("and any future sweep-style leaf that adopts the same pattern") already
  anticipates this.

  **Planner note:** treat "three sites" as a *finding to re-verify*, not a fixed count. Re-run the
  search at plan time — a fourth sweep leaf may have landed. The acceptance gate should assert
  **zero** remaining hand-rolled occurrences of the string, not "three converted", so that a
  newly-added fourth site fails the gate instead of slipping past a hardcoded number.
  — **Reversibility:** costly — the rule's `Sentence` becomes published verbatim into anchored
  regions across docs-site, the skill, and the CLI usage strings; unwinding means regenerating
  every anchor and restoring three hand-rolled strings.

### Canonical Sentence Wording

- **D-02:** The rule's `Sentence` is **flag-shaped**, following the `RulePurgeFilterRequiresScope`
  precedent rather than the wire-neutral majority of the registry.

  The registry holds two conventions. Wire-bearing rules state fields neutrally
  (`"scope is required unless cross_spine is true"`) because the same sentence must serve an MCP
  jsonschema tag and a Connect field alike. CLI-only rules use flag-shaped text — the closest
  cousin, `RulePurgeFilterRequiresScope`, reads `"the free-form filter path requires an explicit
  --scope or --all-scopes: ..."`. This rule is CLI-only: **no MCP tool exposes an `all_scopes`
  field at all**, so there is no wire-neutral audience to serve, and a neutral sentence would name
  fields that exist on no schema.

  Consequently `TagForm` is left **empty**, for the same stated reason
  `RuleDestructiveRequiresApply`, `RuleVerifyFailOnValues`, and `RulePurgeFilterRequiresScope`
  leave it empty: there is no MCP arg struct carrying this field set, so there is no jsonschema tag
  to compress the statement into. Leaving it empty makes any jsonschema-tag surface check compare
  against `Sentence` itself.

  The `Sentence` must be **ASCII-only** — `validateRuleSet` enforces this, because five surfaces
  with different markdown/proto pipelines compare the text by plain byte equality.
  — **Reversibility:** costly — same anchor-regeneration cost as D-01; the sentence is the anchored
  payload.

### Documentation Depth

- **D-03:** The reference pages document the **whole milestone state vocabulary**, not just
  `schema_version`.

  `REQ-docs-record-state` names `schema_version`, but the same staleness that motivates it applies
  to the rest of what Phases 5–7 shipped. `reference/memory-record.md` already carries
  `### Supersession` and `### Archiving` sections, so the structure exists to extend. The pages
  gain `schema_version` **plus** the derived `expired`/`scheduled` words and their **asymmetric
  boundary rule**, which is the part most likely to be got wrong by a reader:

  - `not_before == now` → **ACTIVE** (inclusive `Lte` gate; no `scheduled` word)
  - `not_after == now` → **ALREADY EXPIRED** (exclusive `Gt` gate; emits `expired`)
  - `expired` suppresses `scheduled`; the two are mutually exclusive by a write-time invariant
  - canonical order: `archived, superseded, expired, scheduled` (descending finality)

  **This prose must be derived from the store's gate, never re-reasoned from existing prose.**
  `internal/store/store.go`'s `activeWindowConditions` (~1013/1017) is the authority. Two
  independent implementations already agree with it (`cmd/engram/memory_state.go`,
  `ui/src/lib/memorystate.ts`); a docs page is a **third surface** and must derive from the same
  source, or it becomes an off-by-one trap.
  — **Reversibility:** reversible — prose only.

- **D-04:** The migration guide is a **new standalone `guides/migrate.md`, schema-version only**.

  It mirrors the existing `docs-site/src/content/docs/guides/reindex.md`, which is the established
  shape for an operator procedure (`The migration flow` → `Flags` → `Output` →
  recovery/edge cases → `See also`). Scope is strictly the schema-version-driven mechanism
  (`engram migrate`, `engram migration-status`, the version stamp and its histogram) — SC3
  explicitly excludes `migrate-remap-owner`/`summarize-missing`/`reindex` from what this milestone
  claims, so the guide must not quietly re-absorb them.

  Rejected: grafting it into `guides/upgrade.md`, which already has a `schema_version` section at
  lines 314–340. That section is *release-note* context and correctly version-specific; an
  evergreen operator procedure buried inside a version-specific upgrade doc is not findable by the
  operator who needs it two releases later. `upgrade.md` should instead **link** to the new guide.
  — **Reversibility:** reversible — a new docs page; deleting it costs a redirect at most.

### CLAUDE.md Audit

- **D-05:** CLAUDE.md gets a **full audit against this milestone**, not just the one line SC3 names.

  Three distinct staleness sites, same defect class (normative doc contradicts shipped code):

  1. **Line 70** — `Not used here: database migrations, viper, cocogitto.` Revised to state what
     this milestone ships and its deliberate scope: schema-version-driven migrations only,
     *deliberately not* `migrate-remap-owner`/`summarize-missing`/`reindex`. (This is the literal
     SC3 requirement and the exact contradiction the folded todo flagged.)
  2. **The `cmd/engram/` layout row** — currently lists five operator commands (`reindex`,
     `migrate-remap-owner`, `prune-expired`, `summarize-missing`, `backfill-short-ids`) and omits
     `migrate`, `migration-status`, `get`, and `spine-review` — all shipped in this milestone. A
     routing doc that misnames the command surface actively misdirects agents.
  3. **The Memory contract section** — re-checked for the record-state vocabulary Phases 5–7
     introduced (`schema_version`, and the `archived`/`superseded`/`expired`/`scheduled` words).

  **Constraint:** CLAUDE.md is normative routing, not a changelog. Additions state the *current*
  contract; they do not narrate what changed or when. Keep the existing table/bullet idiom and
  density — this is an edit to a routing doc, not an expansion of it.
  — **Reversibility:** reversible — documentation only.

### Claude's Discretion

- **The anchor surface set is derived, not chosen.** Issue #480 predicted three anchor targets
  (`guides/cli.md`, `skill/engram/skills/curating-memory/SKILL.md`, `reference/tools.md`).
  Empirically that matches `purge-filter-requires-scope` exactly, while `destructive-requires-apply`
  anchors on only two. The set therefore falls out of `ApplicableSurfaces(rule, exposed)` from the
  rule's `Fields`/`SurfaceFields` — the planner computes it and lets `internal/surfacesgen` write
  it. Do **not** hand-pick the target list to match #480's prediction; if the derived set differs,
  the derived set is correct and the divergence is worth a line in the summary.
- Whether `SurfaceFields` needs to diverge from `Fields` for this rule. `Fields` is expected to be
  the flag set (`scope`, `all-scopes`); `SurfaceFields` is only needed if that set over-resolves
  onto surfaces that expose the fields but never raise the rejection. Determine empirically.
- Plan decomposition and wave ordering. The rule-registration track and the docs track are
  independent and parallelizable, with one ordering constraint: docs prose that quotes the rule's
  canonical sentence must follow registration, since `surfacesgen` writes those regions.
- Exact section placement and heading text within each docs page.

### Folded Todos

- **`.planning/todos/pending/2026-08-10-research-versioned-payload-migration-mechanism.md`**
  (area: `database`, severity: minor, `resolves_phase: 3`, match score 0.90).

  **Original problem:** no stored schema/payload version existed anywhere in `internal/store/` or
  `cmd/engram/`; every payload evolution had shipped as its own one-shot operator command
  (`backfill-short-ids`, `migrate-set-owner`, `migrate-remap-owner`, `summarize-missing`), each
  with a near-identical scroll → `SetPayload` → `--dry-run` → `--timeout` shape. The todo asked
  whether these could be wrapped into a single versioned `engram migrate`, and named CLAUDE.md's
  `"Not used here: database migrations"` line as the stance to revisit.

  **How it fits:** this todo is the **origin of this milestone**. Phases 2–4 answered its research
  question by shipping the mechanism (`schema_version`, the migration registry, `engram migrate`).
  Phase 8 closes the loop it explicitly named — the CLAUDE.md contradiction — under D-05.1. The
  todo's `files:` list (`cmd/engram/migrate.go`, `cmd/engram/summarize.go`, `CLAUDE.md`) overlaps
  this phase's edit surface directly. It is retired by this phase's work rather than outliving the
  milestone that answered it.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Rule registry mechanism

- `internal/surfaces/rules.go` — the `ConditionalRule` struct (field-by-field doc comments at
  lines 21–94), the `rules` registry literal (~180–265), and the 8 existing rules. **Read the
  `Fields` vs `SurfaceFields` doc comment and the `declared` provenance comment before adding a
  rule** — `declared: true` is settable only inside this file's literal, by design.
- `internal/surfaces/normalize.go` §`ApplicableSurfaces` (~82) and `SurfaceApplicabilityFields`
  (~102) — how a rule's field set resolves to the surfaces it composes onto.
- `internal/surfaces/anchor.go` — the `engram:rule:start <ID>` / `engram:rule:end <ID>` region
  format.
- `internal/surfaces/conformance_test.go` — the gate that fails when a bound surface drifts from
  the canonical sentence.
- `internal/surfaces/rules_test.go` §`validateRuleSet` cases (~119–175) — the constraints a new
  rule must satisfy: non-empty ID/Sentence/Fields/Hint, **ASCII-only Sentence**, no duplicate or
  substring-colliding IDs.
- `internal/surfacesgen/main.go` — the generator that rewrites anchored regions.

### The closest precedent (read before writing the new rule)

- `internal/surfaces/rules.go` §`RulePurgeFilterRequiresScope` (~162, literal ~258–268) — the
  CLI-only, `conditional_required`, flag-shaped-Sentence, empty-`TagForm` rule this one should
  mirror. Its anchored regions live at `skill/engram/skills/curating-memory/SKILL.md:388`,
  `docs-site/src/content/docs/reference/tools.md:128`, and
  `docs-site/src/content/docs/guides/cli.md:309`.

### Call sites to convert (D-01)

- `cmd/engram/summarize.go:39` — `usageErrorf("--scope <scope> or --all-scopes is required")`
- `cmd/engram/spine_review_scan.go:44` — same string
- `cmd/engram/spine_review_verify.go:623` — same string (**not named by REQ or #480**)
- `cmd/engram/flaggroup_test.go:510` — an existing `--scope`/`--all-scopes` flag-group assertion
  that constrains the Usage strings; check it still holds after conversion.
- Rejection tests that must keep passing: `cmd/engram/spine_review_test.go:54`,
  `cmd/engram/spine_review_verify_test.go:530`.

### Record-state authority (D-03)

- `internal/store/store.go` §`activeWindowConditions` (~1013/1017) — **the authority** for the
  `expired`/`scheduled` boundary semantics. Derive docs prose from this, not from prose.
- `internal/store/store.go` — the four hard recall-gate sites (~1091/1097, 1191/1195, 1328/1333,
  1563/1568): `IsEmpty("superseded_by")` AND `IsEmpty("archived_at")`, deliberately never folded.
- `cmd/engram/memory_state.go` and `ui/src/lib/memorystate.ts` — the two existing independent
  derivations of the state words; both correct as of Phase 7. The docs are a third surface.

### Docs surfaces to edit

- `docs-site/src/content/docs/reference/memory-record.md` — has `## Field reference`,
  `### Supersession` (37), `### Archiving` (55); extend for `schema_version` + window state.
- `docs-site/src/content/docs/reference/tools.md` — anchored-region host (existing rule anchor at
  128).
- `docs-site/src/content/docs/guides/reindex.md` — **the shape template** for the new guide:
  `## The migration flow` (16) → `## Flags` (54) → `## Output` (69) →
  `## Resuming an interrupted run` (104) → `## Repairing a pre-patch resume` (139) →
  `## See also` (180).
- `docs-site/src/content/docs/guides/upgrade.md:314–340` — the existing `schema_version` release
  note; should link to the new guide rather than absorb it.
- `docs-site/src/content/docs/guides/cli.md` — anchored-region host (existing rule anchor at 309).
- `skill/engram/skills/curating-memory/SKILL.md` — anchored-region host (existing rule anchor at
  388).
- `CLAUDE.md:70` and the `cmd/engram/` layout row — D-05.

### Milestone scope

- `.planning/ROADMAP.md` §`### Phase 8: Registry & Docs Tail` (~458) — goal, dependencies
  (2026-08-12.01 Phases 4 and 7), and the three success criteria.
- `.planning/REQUIREMENTS.md:58–60` — `REQ-sweep-scope-rule-registered`, `REQ-docs-record-state`,
  `REQ-claude-md-migrations-convention`.
- `.planning/phases/07-console-cli-state-surfacing/07-CONTEXT.md` §`<domain>` "Out of scope" — the
  two explicit deferrals into this phase.
- GitHub issue **#480** — "register the sweep scope-or-all-scopes conditional rule", including its
  own record of the false "already registered" claim that motivated it. Close it with this phase.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **The rule registry is a pure additive surface.** Adding a rule is a struct literal appended to
  `rules` in `internal/surfaces/rules.go` plus an exported `Rule…` ID const. `ruleByID`,
  `Rules()`, `RuleByID`, and `validateRuleSet` all derive from the slice — nothing else needs
  touching to register.
- **`RulePurgeFilterRequiresScope` is a near-exact template.** Same hint (`conditional_required`),
  same CLI-only nature, same empty `TagForm`, same three-anchor footprint. The new rule is
  substantially a copy with a different sentence and field set.
- **`internal/surfacesgen` already regenerates every anchored region.** The docs/skill edits for
  the canonical sentence are generated, not hand-written — hand-editing inside an anchor is what
  the conformance gate exists to catch.
- **`guides/reindex.md` is a complete, shipped operator-guide template** for `guides/migrate.md`.

### Established Patterns

- **`declared: true` is compiler-enforced provenance.** The field is unexported, so no other
  package can construct a rule that `IsDeclared()` accepts, no matter how faithfully it copies the
  other fields. `internal/server.conditionalErrf` checks it before honoring a rule.
- **`Sentence` is ASCII-only and compared by plain byte equality across five surfaces** with
  different markdown/proto pipelines. A multi-byte character is a normalization hazard;
  `validateRuleSet` rejects it.
- **Declaration order in the `rules` slice is the emission order** `surfacesgen` uses, so
  regeneration is stable across runs. Append rather than insert.
- **Rule IDs must not be substrings of one another** — `rules_test.go` asserts this, because the
  ID doubles as the anchor marker and a substring collision would corrupt region matching.

### Integration Points

- `internal/server.conditionalErrf` — the single construction site converting `rule.Hint` (plain
  string) to `internal/server.HintCode`. The leaf `surfaces` package holds a plain string
  specifically to avoid an import cycle.
- Cobra `Usage` strings on the three sweep commands' `--scope` / `--all-scopes` flag registrations
  (`summarize.go:116`, `spine_review_scan.go:159`, `spine_review_verify.go:666`) — all three
  currently read `"sweep every scope (required if --scope is omitted); mutually exclusive with
  --scope"`. `flaggroup_test.go:510` asserts on this shape.
- `docs-site` builds from `docs-site/src/content/docs/**`; a new guide needs whatever sidebar/nav
  registration that Astro/Starlight setup requires — check how `reindex.md` is registered.

### Known hazards carried into this phase

- **`.planning/**` files must not receive an SPDX header** — GSD requires `---` frontmatter on line
  1, and a header above it makes a passed VERIFICATION.md read as `missing`. `.licenserc.yaml`
  owns scope; if `task license:check` is green, no header is needed. Note the new **docs-site**
  page is also excluded from SPDX by `.licenserc.yaml`.
- **Acceptance gates must be call/declaration-shaped, never bare identifiers.** A trailing comment
  (`// bounded by fooBudget`) defeats a line-anchored comment filter and satisfies a bare-identifier
  grep while the behavior is deleted. This phase is unusually exposed: it is a *documentation*
  phase, so nearly every artifact is prose that a naive `rg <word>` gate would match trivially.
  Prefer set-equality and zero-occurrence assertions over presence greps.
- **Watch for cross-plan pre-satisfaction.** If the docs plans and the registry plan land in
  different waves, a docs plan that writes the canonical sentence can pre-satisfy the registry
  plan's gate. Gate the registry work on the *generated* anchor state, not on the sentence text
  appearing somewhere in the tree.
- **Run `go test ./internal/keylinks/...` at the end of plan-phase and again as an orchestrator
  pre-flight before wave 1.** Key_links escaping has broken three consecutive phases; the guard is
  RED at plan time but nothing runs it until execution, and executors correctly decline to fix
  planning artifacts. Whitespace patterns use `[ ]+` — `[[:space:]]` is RE2-only and outside the
  JS subset.
- **No `ui/` changes are expected in this phase.** If any plan does touch `ui/`, it must run
  `task ui:build` and commit `internal/webauth/static` — CI catches the drift only at PR time.

</code_context>

<specifics>
## Specific Ideas

- The new guide should follow `guides/reindex.md`'s section rhythm closely enough that an operator
  who has read one can navigate the other without re-orienting.
- `guides/upgrade.md`'s existing `schema_version` section stays where it is and gains a link to
  the new guide — release notes point at the evergreen procedure, not the reverse.
- The acceptance gate for D-01 should assert **zero** remaining hand-rolled occurrences of the
  guard string across `cmd/engram/`, rather than asserting three conversions. A count-based gate
  passes if a fourth site is added after planning; a zero-based gate fails, which is the correct
  behavior.

</specifics>

<deferred>
## Deferred Ideas

- **Fold `cmd/engram/client_common.go:286` onto the already-registered
  `RuleScopeRequiredUnlessCrossSpine`.** That site hand-rolls
  `"--scope is required unless --cross-spine is set"` in prose while a registered rule with the
  identical constraint (`"scope is required unless cross_spine is true"`) already exists. It is the
  same defect class as #480 but a **different rule**, on the client read lane Phase 7 just shipped.
  Worth its own issue — the interesting part is that it is a *wire-bearing* rule whose CLI mirror
  drifted, which is a distinct failure mode from an unregistered CLI-only guard.
- **`cmd/engram/client_store.go:48`** (`"--scope is required"`) — a third, simpler unregistered
  scope guard. Assess alongside the above; may or may not warrant a rule.
- **Porting `search`/`list` off `renderMemoryTable` onto Phase 6's view mechanism** — carried
  forward from Phase 7 D-09, still open, still not this phase.
- **Exposing the Phase 7 opt-in recall flags on the MCP tool schemas** — Phase 7 kept agent recall
  zero-junk by default (D-03); the store options already land for both lanes, so this is a pure
  tool-schema addition whenever it is wanted.

</deferred>

---

*Phase: 8-Registry & Docs Tail*
*Context gathered: 2026-08-21*
