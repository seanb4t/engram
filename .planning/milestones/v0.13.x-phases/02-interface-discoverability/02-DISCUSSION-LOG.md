# Phase 2: Interface Discoverability - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-04
**Phase:** 02-interface-discoverability
**Areas discussed:** Rule inventory + completeness, Surface scope, Annotation semantics, Help-golden mechanics

---

## Todo cross-reference

| Option | Description | Selected |
|--------|-------------|----------|
| Neither (recommended) | Both matched on generic keywords only; neither touches the CLI/MCP interface surface | |
| Fold: stale docs branch | Resolve the stale `docs/v0.12.x-phase-01-context` branch in this phase | ✓ |
| Fold: Cloudflare token | Rotate the Cloudflare API token for docs-site deploy in this phase | ✓ |

**User's choice:** Fold both — against the recommendation.
**Notes:** The Cloudflare fold turned out better-justified than the recommendation credited:
`docs-site` is one of the six surfaces the conformance gate ends up binding (D-05), and it has been
failing to deploy since 2026-08-02, so generated rule regions would not reach readers. Requires
dashboard access — a user action, not a coding step.

---

## Rule inventory + completeness

### Q1 — How should the gate know the rule inventory is COMPLETE?

| Option | Description | Selected |
|--------|-------------|----------|
| `HintConditionalRequired` as marker | Widen the hint code to every conditional rejection; gate sweeps call sites and fails on any site lacking a registry entry | ✓ |
| Curated registry only | Hand-written table; proves listed rules agree, silent on unlisted ones | |
| Registry + coverage floor | Registry plus a count assertion, mirroring `argattribution_test.go`'s `>=21 rows` | |

**User's choice:** Structural marker.
**Notes:** Accepted cost of reclassifying ~6 existing rejections currently filed under other hint codes.

### Q2 — Which rejections count as a "conditional rule"?

| Option | Description | Selected |
|--------|-------------|----------|
| Cross-field test, two markers | In scope when the rejection names ≥2 fields OR carries one of the four cross-field hint codes | ✓ |
| All four hint codes, no field test | Sweep by hint code alone; a mis-filed code is invisible | |
| `conditional_required` + `mutually_exclusive` only | Only the two shapes a caller can fix by restructuring the call | |

**User's choice:** Two independent markers.
**Notes:** `not_after must be in the future` is explicitly excluded as single-field and
clock-dependent — an exclusion recorded deliberately rather than left incidental.

### Q3 — How is a rule's canonical text declared and linked?

| Option | Description | Selected |
|--------|-------------|----------|
| Rule value per rule, shared by const | Declared once; `argErrf` detail and cobra `Usage` both compose it; only the jsonschema tag needs reflection | ✓ |
| Key by (field, hint code) | Registry joins on the pair; canonical text still a separate literal from the detail message | |
| Match on the detail message itself | Detail text is canonical; every error reword breaks unrelated surfaces | |

**User's choice:** Declared rule values composed by const.
**Notes:** Turns on the fact that Go struct tags must be compile-time literals — so the tag is the
one surface a shared const cannot reach, and reflection is confined to it.

### Q4 — How is "every conditional rejection references a declared rule" enforced?

| Option | Description | Selected |
|--------|-------------|----------|
| Dedicated constructor + narrow backstop | `conditionalErrf(rule, …)` accepts only declared rules; compiler enforces the common path | ✓ |
| AST sweep in a test | `go/parser` over package source; whole invariant lives in deletable test code | |
| Grep-based gate | `rg` count vs registry size; brittle, and would need the grepping skill's durable-gate checklist | |

**User's choice:** Constructor-enforced.
**Notes:** Aligns with memory `nczgrtfec2` — structural argument over behavioral assertion.

---

## Surface scope

### Q1 — Which surfaces does the conformance gate bind?

| Option | Description | Selected |
|--------|-------------|----------|
| Four in-repo code surfaces (recommended) | cobra Usage + jsonschema tag + tool Description + proto comment | |
| Two, exactly as specified | Ships REQ verbatim; accepts drift in the prose beside the tag | |
| All six, including docs + skill | Adds `docs-site/` and `skill/engram/` markdown | ✓ |
| Three — drop the proto lane | Covers both live callers without the Connect/proto lane | |

**User's choice:** All six — the widest option, past the recommendation.
**Notes:** Scope expansion beyond REQ-surface-conformance-gate's two named surfaces; requires a
`/gsd-phase` roadmap edit.

### Q2 — How are prose surfaces checked without fighting ordinary editing?

| Option | Description | Selected |
|--------|-------------|----------|
| Marked regions, generated + drift-checked | Anchored regions generated from the registry; CI fails on drift, same contract as `gen/` | ✓ |
| Marked regions, checked not generated | Same anchors, no generator; a rule reword is a six-file manual edit | |
| Verbatim substring anywhere in file | Simplest gate; any copy-edit of the rule sentence fails CI | |

**User's choice:** Generated regions.

### Q3 — Where does regeneration live, and how does CI catch drift?

| Option | Description | Selected |
|--------|-------------|----------|
| One task target + one drift job | `task surfaces:gen` regenerates every pinned artifact; one job fails on a dirty tree | ✓ |
| Separate mechanisms per artifact | `task rules:gen` for regions, `go test -update` for goldens | |
| `go generate` directives | Standard Go idiom; sits outside the Taskfile every other codegen path uses | |

**User's choice:** One target, one job — mirroring `task proto:gen` + the `buf` drift job.

### Q4 — How does the gate decide which surfaces a rule must appear on?

| Option | Description | Selected |
|--------|-------------|----------|
| Derive from the fields the rule names | A surface is checked iff it exposes those fields; needs a name normalizer | ✓ |
| Declared, with explicit exclusions | Hand-maintained but auditable via `NotApplicable(surface, reason)` | |
| Plain inclusion list | Simplest; shortening the list silences the gate with no signal | |

**User's choice:** Derived.
**Notes:** Prompted by the paging trio, which exists on the CLI and Connect lanes but not in
`list_memory`'s MCP args. The normalizer becomes load-bearing and needs its own non-empty-surface-set test.

---

## Annotation semantics

### Q1 — What interpretive stance sets the three hints?

| Option | Description | Selected |
|--------|-------------|----------|
| Conservative — must hold for every input | A hint is true only if it holds under every valid invocation | ✓ |
| Designed-intent | Annotate intended use; `store_memory` idempotent "when you pass a key" | |
| Only where unambiguous | Leave the gray zone nil; conflicts with REQ's "every tool declares all three" | |

**User's choice:** Conservative.
**Notes:** Resolves all 15 tools by rule rather than case-by-case — `store_memory`
`idempotentHint: false`, `update_memory` `destructiveHint: true`, `supersede_memory`
`destructiveHint: false`.

### Q2 — Where do annotations live, and what stops a new tool shipping without them?

| Option | Description | Selected |
|--------|-------------|----------|
| Central table, gated both directions | Every registered tool has an entry and vice versa; follows `catalog.go` precedent | ✓ |
| Inline at each `AddTool` site + completeness test | Best locality; classification spread across ~250 lines | |

**User's choice:** Central table.

### Q3 — What happens to `openWorldHint`, the fourth hint REQ doesn't name?

| Option | Description | Selected |
|--------|-------------|----------|
| Set it explicitly to `false` | engram is a closed memory domain; omission reads as "nobody decided" | ✓ |
| Leave it unset | Ships exactly the three named hints | |
| Set true where an embedder call happens | Most literal reading of "external entities" | |

**User's choice:** Explicit `false`.
**Notes:** Scope expansion past REQ-mcp-tool-annotations' three named hints.

### Q4 — Should the CLI catalog carry blast-radius too?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — one taxonomy, both lanes | `engram catalog` gains per-command classification from the same table | ✓ |
| Yes, but a single destructive bit | One boolean rather than the full trio | |
| No — MCP-only | Keeps the diff closer to the four requirements | |

**User's choice:** Full parity.
**Notes:** Scope expansion past REQ-mcp-tool-annotations. Motivated by v0.12.x's headless CLI lane
and by Phase 3's forthcoming `spine-review purge`.

---

## Help-golden mechanics

### Q1 — What shape do the goldens take?

| Option | Description | Selected |
|--------|-------------|----------|
| One file, walked from the live tree | Completeness automatic; one diff to review | ✓ |
| One golden per command | Precisely scoped diffs; a new command silently has no golden | |
| Curated subset | Client verbs only; skips the commands Phase 1 just changed | |

**User's choice:** Single walked golden.
**Notes:** Verified during discussion that `config.FlagDefault` reads a static registry map, not the
environment — so Phase 1's koanf migration did not make help output env-sensitive.

### Q2 — Is the catalog JSON pinned alongside `--help`?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, same target | Catches drift `--help` misses: exit codes, blast radius, JSON field names | ✓ |
| No — `--help` only | Catalog already has build-from-constants tests | |

**User's choice:** Pin both.

### Q3 — What satisfies REQ-help-output-pinned's word "unreviewed"?

| Option | Description | Selected |
|--------|-------------|----------|
| A visible golden diff in the PR | CI fails on staleness; regeneration commit shows before/after wording | ✓ |
| Diff plus an upgrade-guide entry | Extends Phase 1's migration discipline to help text; noisy for typo fixes | |
| Diff plus CODEOWNERS on the golden path | Closest to the literal word; means reviewing your own PR here | |

**User's choice:** Diff-visibility, with the interpretation recorded in phase docs so the verifier
scores against it — Phase 1's verification reopened on exactly this class of wording gap.

---

## Claude's Discretion

- Exact rule IDs and canonical sentence wording per declared rule.
- Which existing rejections get reclassified under D-01, and in what commit order.
- The concrete `--cross-spine` ↔ `cross_spine` normalizer mapping table and where it lives.
- Golden and generated-region file locations.
- Whether `task surfaces:gen` must chain `task proto:gen` — regenerating a proto comment dirties the
  committed `gen/` tree, so the two drift jobs can fail in a confusing order.
- Anchor comment syntax per prose file type.

## Deferred Ideas

- A scheduled canary or paging alarm for the docs-site deploy (the token has no expiry alarm, and
  the job is `skipping` on PRs by construction, so it only fails post-merge).
- A routine audit for stranded local branches left behind by milestone closes.
