# Phase 2: Interface Discoverability - Context

**Gathered:** 2026-08-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Every server-side conditional requirement, CLI flag, and MCP tool argument is correct-by-reading —
a caller learns the rule from the interface itself, never by triggering the rejection first.

Phase 1 made the CLI's *rejections* uniform and migration-safe. This phase makes the *rules behind
those rejections* legible before the call, on every surface that advertises the argument.

**Roadmapped requirements:** REQ-conditional-rules-stated, REQ-surface-conformance-gate,
REQ-mcp-tool-annotations, REQ-help-output-pinned.

**⚠ Scope expansions accepted during discussion — the roadmap does NOT yet reflect these:**

1. **D-05 widens the conformance gate from two surfaces to six.** REQ-surface-conformance-gate
   names the cobra tree and the MCP arg-struct tags. The user chose all six surfaces that restate
   these rules today, adding the MCP tool `Description` prose, the proto field comments,
   `docs-site/`, and `skill/engram/`.
2. **D-11 extends blast-radius classification to the CLI.** REQ-mcp-tool-annotations covers MCP
   tools only; `engram catalog` now carries the same classification for CLI commands.
3. **D-10 adds `openWorldHint`**, a fourth annotation REQ-mcp-tool-annotations does not name.

`.planning/ROADMAP.md` and `.planning/REQUIREMENTS.md` must be updated through `/gsd-phase` —
**never** by hand (rule `8dfdhfs5nn`, extended to body granularity by memory `apfg4fe199`).
Planning must not proceed on the assumption that the roadmap already reflects these. This is the
same situation Phase 1 hit with D-04.

</domain>

<decisions>
## Implementation Decisions

### Rule inventory and completeness

- **D-01:** Completeness is anchored on the **hint code as a structural marker**, not on a
  hand-curated list. `HintConditionalRequired` (and its cross-field siblings) widens to every
  conditional rejection, and the gate sweeps marked call sites, failing when a site's field has no
  registry entry. The alternative — a curated registry — proves only that *listed* rules agree,
  which is silent on precisely the failure this requirement exists to prevent: a new conditional
  rule that never reaches either surface. Cost: ~6 existing rejections currently filed under
  `HintRequired`/other codes need reclassification.
  — **Reversibility:** costly — undo means re-auditing every rejection site the reclassification
  touched, and the hint codes are part of the published `field=<name> hint=<code>` envelope
  documented in `reference/errors.md`.

- **D-02:** A rejection is in scope for the gate when it **names ≥2 fields** (`argErrFieldsf`)
  **OR** carries one of the four cross-field hint codes (`conditional_required`,
  `mutually_exclusive`, `not_applicable`, `ordering`). Two independent markers, so a rule filed
  under the wrong hint code is still caught by its field arity. `tools.go:546`
  (`not_after must be in the future`) is **explicitly excluded** — single-field and
  clock-dependent, with no second field to cross-reference. That exclusion must be deliberate and
  documented, not incidental.
  — **Reversibility:** reversible — a predicate in the gate.

- **D-03:** Each rule is a **declared Go value** (id, canonical text, fields, hint) constructed
  once. The `argErrf` detail message and the cobra `Usage` string both **compose that const** —
  legal, because only struct tags are literal-only in Go. This cuts the reflection-compared
  surfaces from two to one: the jsonschema tag is the only place the canonical text must be
  re-typed and checked rather than referenced.
  — **Reversibility:** costly — undo touches every rule site and both composing surfaces.

- **D-04:** The invariant "every conditional rejection references a declared rule" is enforced by a
  **dedicated constructor**, `conditionalErrf(rule, …)`, that accepts only a declared rule value —
  so the compiler enforces the common path. The sweep shrinks to one narrow backstop: nobody used
  the generic `argErrf`/`argErrFieldsf` with a cross-field hint code. Per memory `nczgrtfec2`, this
  is the structural argument rather than a behavioral parity assertion.
  — **Reversibility:** costly — the constructor's signature is the enforcement; reverting it
  returns the invariant to test-only status.

### Surface scope and the conformance gate

- **D-05:** The gate binds **all six surfaces** that state these rules today:
  1. cobra `Usage` (`client_list.go:95-108`, `client_search.go:83-92`) — also feeds `--help` and
     `engram catalog` from one source via `buildCatalog`
  2. MCP jsonschema struct tags (`tools.go:598,609,718`)
  3. MCP tool `Description` prose (`tools.go:1803,1851,1976`)
  4. proto field comments (`proto/engram/v1/*.proto:65-70`)
  5. `docs-site/` (`reference/tools.md`, `guides/cli.md`)
  6. `skill/engram/` (`curating-memory`, `discovering` SKILL.md)

  Rationale for going past the requirement's two: an MCP agent sees surface 3 far more reliably
  than surface 2 — many clients render only the description — so binding the tag while leaving the
  prose beside it free to drift would miss the most-read statement of the rule.
  — **Reversibility:** reversible per surface — dropping a surface from the gate is a local change,
  though the anchors left behind in prose files would need removing.

- **D-06:** Prose surfaces carry **anchored regions whose contents are generated** from the rule
  registry, and CI fails on drift — the same contract the committed `gen/` tree already lives
  under. Authors reword freely outside the anchors; the rule sentence has exactly one source. The
  rejected alternative (verbatim substring anywhere in the file) makes ordinary copy-editing a red
  build with no indication of where the blessed text lives.
  — **Reversibility:** costly — the anchors are committed into four prose trees including the
  published docs site and the released skill.

- **D-07:** **One `task surfaces:gen` target regenerates every pinned interface artifact** — rule
  regions, `--help` goldens (D-12), and the catalog JSON (D-13) — and **one CI job** re-runs it and
  fails on a dirty tree. This is the shape `task proto:gen` + the `buf` drift job already has, so
  contributors learn one pattern rather than three, and one command follows any interface change.
  — **Reversibility:** reversible — a Taskfile target and a CI job.

- **D-08:** A rule's applicable surfaces are **derived from the fields the rule names**, never
  declared: a surface is checked if and only if it exposes those fields. Applicability cannot then
  be shortened to silence a failing gate — the only way to drop a surface is to remove the field
  from it, which is the change that *should* drop it. Requires a name normalizer
  (`--cross-spine` ↔ `cross_spine`), and that normalizer is load-bearing: if it mismaps, a rule
  silently resolves to zero surfaces and passes. It needs its own test asserting every rule
  resolves to a **non-empty** surface set.
  — **Reversibility:** reversible.

### MCP tool annotations

- **D-09:** The interpretive stance is **conservative — a hint is true only if it holds under every
  valid invocation**. This resolves all 15 tools without case-by-case debate:
  `store_memory`/`schedule_memory` are `idempotentHint: false` (the idempotency key is optional, so
  an unkeyed repeat duplicates), `update_memory` is `destructiveHint: true` (replaces content in
  place), `supersede_memory` is `destructiveHint: false` (additive by design under all inputs —
  history preserved, target soft-hidden not deleted). An agent reading a hint gets a guarantee, not
  an aspiration. Note the go-sdk carries the MCP spec's own caveat that these are hints clients
  should not treat as authorization.
  — **Reversibility:** reversible per tool, but the values are advertised to every connected agent.

- **D-10:** Annotations live in a **central table keyed by tool name, gated in both directions** —
  every registered tool has an entry, every entry names a registered tool. Follows `catalog.go`'s
  precedent (built from one source, never a second literal, gated by
  `TestCatalogExitCodesMatchMapper`) and makes the whole blast-radius taxonomy reviewable on one
  screen, which is the point of the requirement. `openWorldHint` is **set explicitly to `false`**
  on every tool rather than omitted: engram is a closed memory domain, and an omitted hint reads as
  "nobody decided", which is the ambiguity this phase exists to remove.
  — **Reversibility:** reversible.

- **D-11:** `engram catalog` gains **per-command blast-radius derived from the same table** — one
  taxonomy, both lanes. v0.12.x built the headless CLI lane precisely so agents could drive engram
  without MCP, and those agents currently have no way to classify a command before running it.
  Landing this now means Phase 3's `spine-review purge` is born classified rather than retrofitted.
  — **Reversibility:** one-way — `engram catalog` is a published contract; adding a field is
  additive, but agents will begin branching on it.

### Help output pinning

- **D-12:** A **single golden file, generated by walking the live cobra tree** in deterministic
  order. Completeness is automatic — a new command appears without anyone remembering to add a file
  — and review is one diff instead of fifteen. Same derive-don't-declare property as D-08.
  Confirmed during discussion that `config.FlagDefault` reads a static registry map rather than the
  environment, so Phase 1's koanf migration did **not** make help output env-sensitive; goldens are
  deterministic.
  — **Reversibility:** reversible.

- **D-13:** The bare `engram` **catalog JSON is pinned alongside `--help`**, under the same
  `task surfaces:gen` target. After D-11 the catalog carries classification an agent acts on;
  pinning the rendered JSON catches drift `--help` alone would miss (exit codes, blast radius, JSON
  field names).
  — **Reversibility:** reversible.

- **D-14:** REQ-help-output-pinned's word **"unreviewed" is interpreted as: CI fails whenever the
  committed golden does not match the live tree, so any help-text change forces a regeneration
  commit whose diff shows the exact before/after wording in review.** CODEOWNERS gating was
  considered and rejected as theater on a single-maintainer repo. **This interpretation must be
  recorded in the phase's own docs so the verifier scores against it** — Phase 1's verification
  reopened on precisely this class of wording gap.
  — **Reversibility:** reversible.

### Claude's Discretion

- The exact rule IDs and canonical sentence wording for each declared rule.
- Which specific existing rejections get reclassified under D-01, and in what commit order.
- The concrete normalizer mapping table for D-08, and whether it lives beside the registry or the
  gate.
- Golden and generated-region file locations (`cmd/engram/testdata/` is the obvious home).
- Whether `task surfaces:gen` must chain `task proto:gen` — regenerating a proto comment dirties
  the committed `gen/` tree, so the two drift jobs can fail in a confusing order. Raised during
  discussion, left to the planner.
- Anchor comment syntax for each prose file type.

### Folded Todos

Both pending todos were folded into this phase by explicit user selection.

- **Rotate the Cloudflare API token for docs-site deploy** (`tooling`, severity major,
  `.github/workflows/docs-site.yaml:69-71`). The `deploy` job has failed on every `main` push since
  2026-08-02 with `Invalid access token [code: 9109]` on the bare `/accounts` call — the token is
  invalid or revoked, not merely under-scoped, so re-scoping will not help. **Fits this phase
  because `docs-site` is one of D-05's six bound surfaces**: generating rule regions into
  `reference/tools.md` and `guides/cli.md` is pointless while the site cannot publish, and
  `reference/errors.md` (shipped in v0.12.0, referenced by `CLAUDE.md` and the `curating-memory`
  skill as the lookup for `hint=` codes) is still not live. **Requires Cloudflare dashboard and
  repo-secret access — this is a user action, not a coding step.** Steps: issue a token with
  Workers Scripts: Edit on the account owning the `engram-docs` worker, update the
  `CLOUDFLARE_API_TOKEN` repo secret, `gh run rerun 30774235923 --failed`, then verify
  `reference/errors.md` is actually live rather than merely that the job passed.

- **Resolve stale `docs/v0.12.x-phase-01-context` branch** (`planning`, severity minor). A
  local-only branch carrying 3 commits absent from `main` (`7e762662`, `018e91a4`, `71ccc42c`),
  never pushed. Equivalent content is archived at
  `.planning/milestones/v0.12.x-phases/01-shared-auth-chain-connect-bearer-identity/` but has not
  been verified line-for-line. **Verify by content, then delete — never delete first**; paths moved
  during the milestone close, so a path-matched diff shows everything as missing. While in there,
  audit for other stranded local branches from earlier milestones.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Rule inventory and the error envelope
- `internal/server/argerror.go` §27-36 — the ten `HintCode` constants; the four cross-field codes
  D-02 selects on (`conditional_required`, `mutually_exclusive`, `not_applicable`, `ordering`) and
  the `argErrf` / `argErrFieldsf` constructors D-04 replaces for conditional rejections.
- `internal/server/tools.go` §1356, §1379 — the two existing `HintConditionalRequired` sites
  (`effectiveSearchScope` and its sibling), the worked example in the requirement.
- `internal/server/tools.go` §531, §546, §551 — the `not_applicable` and two `ordering` sites;
  §551 is cross-field (in scope), §546 is single-field/clock-dependent (D-02's explicit exclusion).
- `internal/server/connectapi.go` §184-185 — the Connect lane's `cursor_mode`/`offset`
  `HintMutuallyExclusive` rejection.
- `docs-site/src/content/docs/reference/errors.md` — the published `field=<name> hint=<code>`
  envelope and all ten hint codes; D-01's reclassification changes what this documents.

### The six bound surfaces (D-05)
- `cmd/engram/client_list.go` §94-108 and `cmd/engram/client_search.go` §83-92 — the cobra `Usage`
  strings that state the scope and paging rules today.
- `cmd/engram/client_common.go` §248-277 — the surviving `--scope`/`--cross-spine` guard after
  Phase 1's D-07, including the comment explaining why `MarkFlagsOneRequired` cannot express
  "A is required unless B".
- `internal/server/tools.go` §598, §609, §718 — the three `jsonschema:"required unless cross_spine"`
  tags; the only surface that cannot reference a Go const (struct tags are literal-only).
- `internal/server/tools.go` §1783-2016 — the 15 `mcp.AddTool` registrations and their
  `Description` prose; the insertion point for D-09/D-10's annotations.
- `proto/engram/v1/*.proto` §65-70 — the proto field comments, the most thorough existing statement
  of the `cross_spine` rule.
- `docs-site/src/content/docs/reference/tools.md`, `docs-site/src/content/docs/guides/cli.md` —
  the docs-site surfaces.
- `skill/engram/skills/curating-memory/SKILL.md`, `skill/engram/skills/discovering/SKILL.md` — the
  skill surfaces.

### Catalog, goldens, and codegen precedent
- `cmd/engram/catalog.go` §14-101 — `catalogDoc`/`buildCatalog`, which derives the self-describe
  document from the live cobra tree "never from a hand-maintained literal" (D-15); the extension
  point for D-11's blast radius and the source for D-13's pinned JSON.
- `internal/config/registry.go` §147-149 — `FlagDefault`, confirmed to read a static map, which is
  what makes D-12's goldens deterministic.
- `Taskfile.yaml` — `proto:lint` / `proto:gen`, the generate-and-commit pattern D-07 mirrors.
- `.github/workflows/` — the `buf` drift job, the CI shape D-07 mirrors.

### Conformance-test precedents in this repo
- `internal/server/argattribution_test.go` — asserts on field identifiers and hint codes, **never**
  on message wording, with full-set equality rather than membership; the closest existing analog to
  the conformance gate, and its coverage-floor note (`>=21 rows`) documents why a floor alone is
  weak.
- `internal/server/connectdescriptor_test.go` — semantic reflection over generated descriptors
  rather than a golden wire snapshot; the precedent for reflecting over a schema instead of
  diffing rendered output.
- `internal/server/tools_test.go` §39-113 — the `AddTool` schema-generation harness; the path that
  once panicked at startup, and the place annotations will need coverage.

### Phase and milestone context
- `.planning/phases/01-interface-enforceability/01-CONTEXT.md` — Phase 1's D-01..D-10; D-08 there
  ("`--page-token` with `--offset` becomes an error, not a silent ignore") explicitly names this
  phase as where the paging model gets codified as correct-by-reading.
- `.planning/REQUIREMENTS.md` §60-74 — the four Interface Discoverability requirements verbatim.
- `.planning/ROADMAP.md` §265-294 — the phase goal, the "parallelizable with Phase 1" dependency
  note, and the four success criteria.
- `docs-site/src/content/docs/guides/upgrade.md` — Phase 1's migration-notes target; relevant if
  D-01's reclassification changes any published hint code.
- `.github/workflows/docs-site.yaml` §69-71 — the failing deploy job from the folded Cloudflare
  todo.

### Related memory records
- `nczgrtfec2` — prove "constructed exactly once" with a structural/compile-time argument, not a
  behavioral parity test. Directly motivates D-04.
- `apfg4fe199` — rule `8dfdhfs5nn` applies at body granularity; never invent structure in
  `.planning/` artifacts. Governs how the D-05/D-10/D-11 scope expansions reach the roadmap.
- `p1vqxqhxrm` — `go clean -testcache && task test` before any phase-completion or pre-PR gate on a
  cross-package contract change. This phase adds cross-package gates, so it applies directly.
- `k66tenzbhy`, `akf6xesf64` — cobra/pflag and exit-code test-harness traps in `cmd/engram`;
  relevant to any test that builds a cobra tree, including D-12's golden walker.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `buildCatalog` (`catalog.go:53`) already walks the live cobra tree and derives a document from it
  — D-12's golden walker and D-11's blast-radius field both extend an existing traversal rather
  than adding a second one.
- `argErrf` / `argErrFieldsf` (`argerror.go`) — the existing rejection constructors; D-04's
  `conditionalErrf` wraps rather than replaces them.
- `TestCatalogExitCodesMatchMapper` — the both-directions gate precedent D-10 copies for the
  annotation table.
- `assertEnvelope` and the field-identifier/hint-code assertion style in `argattribution_test.go` —
  the conformance gate should assert on identifiers and codes, not wording, for the same reason.
- The go-sdk's `mcp.ToolAnnotations` (`go-sdk@v1.6.1/mcp/protocol.go:1357`) — already available on
  `mcp.Tool.Annotations`; no dependency change needed for REQ-mcp-tool-annotations.

### Established Patterns
- **Derived, never hand-maintained.** `buildCatalog` derives from the cobra tree; `catalog.go`'s
  exit codes derive from the constants. D-08 and D-12 continue this by deriving surface
  applicability and golden coverage rather than declaring them.
- **Generate, commit, check for drift.** The `gen/` tree is buf-generated, committed, and
  CI-checked. D-06/D-07 apply the same contract to prose rule regions.
- **Assert on identifiers, never on message wording.** `argattribution_test.go` states this
  explicitly. The conformance gate inverts it — it *must* compare text — which is exactly why D-03
  makes that text a single declared const rather than a literal in each surface.
- **Struct tags are literal-only.** The one hard language constraint shaping this phase: the
  jsonschema tag cannot reference the canonical const, so one surface is unavoidably
  reflection-compared.

### Integration Points
- `internal/server/argerror.go` — where D-03's rule values and D-04's constructor are declared.
- `internal/server/tools.go` `Register()` §1783-2016 — where D-09/D-10's annotations attach to the
  15 tool registrations.
- `cmd/engram/catalog.go` `buildCatalog` — where D-11's blast radius and D-13's pinned JSON enter.
- `Taskfile.yaml` + `.github/workflows/` — where D-07's `surfaces:gen` target and drift job land.
- The four prose trees (`proto/`, `docs-site/`, `skill/engram/`, plus cobra usage strings) — where
  D-06's anchored regions are written.

</code_context>

<specifics>
## Specific Ideas

- The user chose the **widest** surface option (six, over a recommended four) and the **strongest**
  enforcement option at every step: structural marker over curated registry (D-01), dedicated
  constructor over test-only sweep (D-04), generated regions over checked regions (D-06), derived
  applicability over declared (D-08). The consistent preference is *make the invariant
  unrepresentable rather than merely tested*. Planning should resolve ambiguity in that direction.
- The `gen/` tree's generate-commit-drift-check contract was cited during discussion as the model
  D-07 should copy. Treat it as the reference implementation, not merely an analogy.
- Phase 1's D-08 already declared that the paging model becomes correct-by-reading "which is
  exactly what Phase 2 codifies" — the paging trio is therefore an expected member of this phase's
  rule registry, not an optional extra.
- The ROADMAP notes this phase's standard "should exist before Phase 3's `spine-review` help text
  is finalized". D-11 strengthens that ordering: `spine-review purge` should be born with a
  blast-radius classification rather than retrofitted.

</specifics>

<deferred>
## Deferred Ideas

- **A scheduled canary or paging alarm for the docs-site deploy.** Raised in the Cloudflare todo:
  the token has no expiry alarm and the deploy job is `skipping` on pull requests by construction,
  so it only ever fails post-merge and can sit red for days. Fixing the token is folded into this
  phase; building the canary is CI/observability work that belongs in its own phase.
- **A routine audit for stranded local branches.** Raised in the stale-branch todo — nothing
  currently audits for local-only branches left behind by milestone closes. Resolving this one
  branch is folded in; the recurring audit is not.

</deferred>

---

*Phase: 2-Interface Discoverability*
*Context gathered: 2026-08-04*
