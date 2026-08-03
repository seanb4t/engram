# Architecture Research — v0.13.x "Curation & Self-Evidence"

**Domain:** integrating a structural-curation CLI, a semantic-curation skill, and a
three-surface interface audit into an existing, shipped Go/Qdrant memory server
**Researched:** 2026-08-03
**Confidence:** HIGH (grounded directly in the shipped `cmd/engram/*`, `internal/store/store.go`,
`internal/server/tools.go`, `internal/authz/*`, and `skill/engram/skills/curating-memory/SKILL.md`
source — not external ecosystem research; this milestone is pure integration, not new-technology
adoption)

## Standing invariants this design must not violate

- **Authorization lives only in `internal/store`.** Every actor-facing store method
  (`Search`, `SearchReranked`, `SearchDiscovery`, `List`, `ListScheduled`, `Get`/`GetReadable`,
  `getWritable`, `Update`, `Delete`, `SetVisibility`, `Supersede`) takes a `Subject` and calls
  `decideBucket`/`decideRecord` (`internal/store/store.go:726,753`), which consult the Cedar PDP
  in `internal/authz`. A design that adds a second authorization check anywhere else — a CLI-side
  filter, a skill-side "am I allowed to see this" heuristic — is an architectural violation here,
  not a stylistic choice.
- **MCP/Connect parity.** Connect's six write RPCs are thin `protoconv` adapters delegating to the
  same `deps.*` business-logic methods the MCP tool handlers call (REQ-connect-write-authz-parity,
  v0.10.x Phase 17) — proven by a per-RPC `TestWriteParity`. Any new write path (the skill, a future
  spine-review write mode) that bypasses this shared layer and calls `internal/store` directly from
  a new adapter would break that provable-parity guarantee.
- **Near-zero new Go dependencies**, held across four consecutive milestones. Every design below
  reuses cobra, the existing `jsonschema`-tagged arg-struct pattern, and stdlib — no new codegen
  toolchain, no plan-file serialization library.

## Existing precedent this milestone must extend, not reinvent

`cmd/engram/` already has five **operator commands**, each resolving exactly one structural
predicate against the whole collection, **without a `Subject`**:

| Command | Store method | Predicate | Subject-based authz? |
|---|---|---|---|
| `reindex` | `Store.Reindex` (`store.go:2614`) | re-embed into new dim | No |
| `migrate-remap-owner` | `Store.RemapOwner` (`store.go:2443`) | owner re-stamp | No |
| `prune-expired` | `Store.PruneExpired` (`store.go:2087`) | lapsed window reclaim | No |
| `summarize-missing` | (summary queue sweep) | missing summary | No |
| `backfill-short-ids` | `Store.BackfillShortIDs` (`store.go:2234`) | missing short_id | No |

None of these five methods takes a `Subject`, and none calls `decideBucket`/`decideRecord`. They
scan/mutate the **whole** Qdrant collection via `Count`/`Scroll`/`SetPayload` filters keyed on
payload fields (`owner==""`, `owner==X`, expiry timestamp), run by an operator who already has
direct Qdrant/process access — a deliberate, five-times-repeated second tier that sits **beside**
(not inside) the per-actor authz model, not a gap in it. `engram spine-review` is the **sixth**
instance of this tier, not a new one. This is the answer to "must not invent a second authorization
path": the correct move is to reuse this already-established Subject-less operator tier, not to
compose `Search`/`List` (which would silently authz-filter to one actor's slice — wrong for a
spine-wide sweep) and not to add any new permission check.

Every one of the five existing commands also already implements **dry-run as "same method,
skip-the-write" — never a second code path**:

- `store.RemapOwner(ctx, src, to, dryRun bool)` (`store.go:2443`) computes the same `Count` either
  way and only calls `SetPayload` `if cnt == 0 || dryRun { return }`.
- `store.Reindex(ctx, opts.DryRun, ...)` (`store.go:2614`) scans and classifies every record
  identically; `DryRun` only gates the final `Upsert`. `reindexSummary()` (`reindex.go:93`) is a
  **pure, unit-tested** renderer that produces a different sentence for `dry-run`, `dry-run
  --resume`, and the real cutover, from the **same** `ReindexResult` struct.
- `migrate-remap-owner --dry-run` (`migrate.go:121`) prints `[dry-run] would remap %d` vs
  `remapped %d`, again from the same `n, err := st.RemapOwner(...)` call.

This is engram's existing plan-then-apply architecture — lighter than Terraform's persisted
plan-file model (no serialized artifact, no separate `apply <planfile>` step), and that lightness
is deliberate: it costs nothing (`near-zero new dependencies`) and is proven safe at this data
scale by four operator commands already in production. **Recommendation: extend this pattern,
don't replace it with a Terraform-style plan file.**

## A. `engram spine-review` — structural curation CLI

### Shape: one parent command, cobra subcommands — not five siblings

The milestone names five phases: review, consolidate, verify, purge, archive. Unlike the five
existing operator commands (which are **independent** — nothing about `reindex` depends on
`prune-expired` having run), spine-review's phases have a **real pipeline dependency**: `purge`
must not run before `verify` has confirmed the extract-before-delete ordering invariant on the
records `review`/scan flagged. A flat set of five new top-level sibling commands (matching the
existing five-command convention) would let an operator run `engram spine-purge` before ever
running `engram spine-review`, with nothing in the CLI's own structure preventing it.

Compare the two precedents the question names:

- **`git gc`** bundles several internally-sequenced sub-tasks (repack, prune, rerere-gc) behind
  **one** verb with no exposed subcommands — because the sequencing is entirely internal and the
  user never needs to run one phase without the others.
- **`terraform {plan, apply, destroy}`** exposes the phases as **separate subcommands** precisely
  because a human needs to inspect `plan`'s output before choosing to `apply` it — the boundary
  between propose and perform is a deliberate, user-visible gate, not an implementation detail.

Spine-review is the Terraform shape, not the `git gc` shape: the whole point of a structural
curation tool touching deletes is that a human (or the calling agent, per invariant B below) reads
the proposal before anything is destroyed. Recommend:

```
engram spine-review scan     # read-only; walks the whole collection, classifies findings
                              # (drifted citations, orphaned records, lapsed windows).
                              # Always safe. --output json for programmatic consumption.
engram spine-review verify   # read-only; asserts the extract-before-delete ordering
                              # invariant for whatever `purge` would touch. This is the
                              # command #355 is the live fixture for (see Build Order).
engram spine-review purge    # destructive; requires --apply (default = preview only,
                              # a DELIBERATE inversion of the existing --dry-run-opt-in
                              # convention — see "Dry-run representation" below).
engram spine-review archive  # softer disposal (e.g. tag-and-hide) for borderline findings
                              # scan surfaces but verify/purge shouldn't auto-delete.
```

Grouping under one parent namespace communicates the pipeline relationship cobra's flat top-level
list cannot; each subcommand still reuses the shared propose/perform convention below.

### Where the scan lives

**New `internal/store` bulk-scan method(s)**, Subject-less, following `Reindex`/`PruneExpired`'s
existing shape exactly (a `Scroll`-based whole-collection walk classified against payload fields —
`superseded_by`, `not_after`, `citations[].locator`/`ref`, orphan markers), returning a typed
findings struct the way `ReindexResult`/`RemapOwner`'s count do today. **Do not** build this by
composing `Search`/`List`: those are Subject-gated and would silently scope the sweep to one
actor's records, defeating the tool's purpose and creating a false sense that the spine was fully
reviewed when only a slice was. **Do not** add a new authorization primitive to gate the scan:
the existing five-command precedent establishes that this tier runs with direct operator access,
outside `decideBucket`/`decideRecord` entirely — matching that precedent is what keeps
`internal/store`'s authz chokepoint singular rather than duplicating it.

### Dry-run / propose representation

Keep the existing "same method, `DryRun`/`Apply` flag skips the write" architecture — it already
produces both the proposal and the execution from one code path (`RemapOwner`, `Reindex`). Two
refinements specific to spine-review's higher blast radius:

1. **Invert the default for `purge` only.** The five existing commands default to *write* (opt-in
   `--dry-run` to preview). A tool whose job is cross-owner deletes should default to *preview*
   (opt-in `--apply` to perform) — this is a deliberate divergence from the established
   convention, not an oversight, and should be recorded as such (an ADR or `D-` decision) precisely
   because a future reader diffing spine-review against `reindex`/`migrate-remap-owner` will
   otherwise assume the flag polarity is a bug.
2. **Re-derive eligibility at apply time; never trust a stale externally-passed ID list.**
   Terraform's plan file needs a "stale plan" refresh check because plan and apply are separate
   invocations, possibly far apart in time. Engram's existing `PruneExpired(ctx, before
   time.Time)` sidesteps this entirely by computing-then-deleting inside **one call** — no ID list
   crosses the propose/perform boundary. `purge --apply` should do the same: recompute its own
   fresh Subject-less scan and predicate check immediately before each delete, not act on IDs
   copy-pasted from an earlier `scan` report. Pin this with a golden/snapshot test asserting
   `scan`'s report and `purge`'s dry-run preview are byte-identical for the same underlying data —
   the concrete, testable meaning of "same code path produces both the proposal and the
   execution."

## B. The semantic-judgment skill

### Route: the existing MCP tool surface — never the CLI, never a new Connect path

The skill must reach the same Qdrant collection spine-review scans, but it should do so through
the **already-shipped MCP tools** (`list_memory`/`search_memory` to enumerate candidates,
`get_memory` for full content, `update_memory`/`supersede_memory`/`delete_memory` to apply a
user-blessed disposition) — exactly the mechanism `skill/engram/skills/curating-memory/SKILL.md`'s
existing "One-time rule backfill sweep" (`SKILL.md:203-248`) already uses for the v0.12.x Phase 6
precedent this milestone explicitly cites. Three architectural reasons, not just consistency:

1. **Consent boundary via authz, not via prose.** MCP tool calls run through the calling agent's
   own authenticated actor, which means every candidate the skill can even *see* is already
   filtered by `internal/store`'s Subject-based authz — the skill is structurally incapable of
   proposing a change to a record the calling agent's owner can't read/write. This is the "propose,
   never perform" boundary made real by the architecture, not by an instruction the model might
   ignore.
2. **The CLI is the wrong side of a hard privilege line.** `engram spine-review` is (by design A)
   a Subject-less, cross-owner operator tool. Letting an agent invoke it — even in read-only `scan`
   mode — would hand LLM-driven code visibility into every owner's private records, a strictly
   worse boundary than anything the MCP surface grants today. The skill must never shell out to
   `engram spine-review` in any mode.
3. **Connect adds a transport with zero behavioral difference and real new plumbing.** Because of
   the MCP/Connect parity invariant, a Connect write RPC does exactly what the matching MCP tool
   does. Routing the skill over Connect would require it to bootstrap a second client identity
   (bearer token, TLS config) for no capability gain — a pure regression against "near-zero new
   dependencies" and against the invariant that MCP and Connect are two adapters over one shared
   layer, not two independent capability sets an agent should choose between.

### What's new vs. reused

- **Reused, unmodified:** every MCP tool the skill calls already exists (`list_memory`,
  `search_memory`, `get_memory`, `update_memory`, `supersede_memory`, `delete_memory`). No new
  server-side code.
- **New/extended:** the skill content itself — most naturally an extension of
  `curating-memory`'s existing rule-hygiene section (`SKILL.md:122-249`), generalizing its
  duplicate/contradiction/rot triad from *rules* to *all memories*, plus the new "is this record
  still true" staleness judgment. The per-candidate propose-then-stop consent protocol
  (`SKILL.md:228-248`, "Bless"/"Decline", never batch-applying a sweep on one approval) is the
  precedent to generalize verbatim — do not design a new consent shape for this milestone.

### The extract-before-delete handoff (B → A coupling)

Rule `7smp8vy9hr`'s existing milestone-completion curation pass already performs the semantic
"extract reusable facts, write one authoritative summary, only then delete the collapsed records"
sequence (`SKILL.md:132-139`) — that extraction step is inherently semantic (which facts are
reusable) and belongs in the skill. `engram spine-review verify`'s job is the **structural**
mirror: mechanically confirm, for any record `purge` is about to remove, that a corresponding
extract already landed (a superseding summary record exists, or an equivalent marker). The skill
and the CLI never call into each other — they act on the same Qdrant data at different times,
coordinated by the human's own workflow ("run the curation skill, then run `spine-review
verify`/`purge`"), not by a shared code path. Do not try to make `verify` semantically aware, and
do not try to make the skill perform structural checks it has no privileged view to perform
correctly.

## C. The interface audit — three surfaces, two independent sources

### Correction to the framing: `--help` and the self-describe catalog are already ONE source

`cmd/engram/catalog.go`'s `buildCatalog(root *cobra.Command)` (`catalog.go:52`) walks the **live**
cobra tree — `root.Commands()`, `cmd.Flags().VisitAll` via `collectFlags` (`catalog.go:109`) — and
its own doc comment states the design intent explicitly: derived "never from a hand-maintained
literal — so a command or flag added later appears here with no edit, and cannot silently go
missing (D-15)." The exit-code table inside it is likewise built from the `exitOK`/`exitUsage`/…
constants in `client_common.go:194-201`, not a second literal list — and `catalog.go`'s own comment
names the regression gate: `TestCatalogExitCodesMatchMapper`. **`--help` and the JSON catalog
cannot drift from each other by construction; that problem is already solved.**

The genuine, independent third surface is `internal/server/tools.go`'s Go struct + `jsonschema`
struct-tag declarations (e.g. `Scope string `json:"scope,omitempty" jsonschema:"required unless
cross_spine"``, `tools.go:598,609,718`) — a completely separate type system (proto/MCP arg structs
reflected into JSON Schema by the go-sdk) with no natural common representation shared with a
`pflag.Flag.Usage` string. A CLI flag (`--scope`) and an MCP tool argument (`scope`) describing the
identical server-side rule (`effectiveSearchScope` and its siblings) are two independently-authored
English sentences today, on two different commands/tools that don't correspond 1:1.

### Proportionate mechanism: a conformance test, not a codegen unification

Given only a **small, enumerable** set of conditional-requirement rules exist
(`effectiveSearchScope` and its named siblings), the proportionate fix is the same pattern this
codebase already uses for the exit-code catalog: a **hand-authored table** mapping each rule to
(a) the MCP tool + field name(s) whose `jsonschema` tag must state it, and (b) the CLI command +
flag(s) whose `Usage` string must state it — then a test walks **both** live sources (the CLI side
already exposed via `buildCatalog`/`collectFlags`; the MCP side needs one small new introspection
helper in `internal/server`, mirroring `buildCatalog`'s shape) and asserts the named substring
appears in both. This is a golden/conformance test, not full codegen: it needs no new dependency,
derives no struct tags from flag text or vice versa, and is proportionate to a handful of rules
rather than the full cross product of every flag against every tool argument (most of which
describe unrelated concerns and have no reason to agree).

This becomes the **permanent regression gate** for D-00 (correct-by-reading) — the same role
`TestCatalogExitCodesMatchMapper` plays for the exit-code taxonomy and the spine-review `verify`
step plays for citation drift. Build it once; every future flag/tool-arg addition is checked by it
going forward, rather than re-auditing by hand each milestone.

### #453 is this same family, with a mechanical rather than a documentary fix

`client_list.go:94-106` already **states** three mutual exclusivities in `Usage` text
(`--offset`/`--cursor-mode`/`--page-token`) that nothing validates — a documented-but-unenforced
constraint is exactly the D-00 violation class C exists to catch, just on the "is it true" axis
rather than the "is it discoverable" axis. The fix is mechanical and narrow: adopt cobra's own
`cmd.MarkFlagsMutuallyExclusive(...)`, replacing the undone `client_list.go` declarations, and
noting it as the third variant alongside the two existing hand-rolled guards
(`validateScopeCrossSpine`, `client_common.go:234`; `buildRemapSource`, `migrate.go:73`) — those two
stay hand-rolled (they also enforce cross-field *semantic* rules, e.g. exactly-one-of, that
`MarkFlagsMutuallyExclusive` alone can't express), but no *new* hand-rolled guard should be added
where cobra's builtin already covers the case.

## New vs. modified components

| Component | Status | Notes |
|---|---|---|
| `cmd/engram/spinereview.go` (new file) | **New** | cobra parent command + `scan`/`verify`/`purge`/`archive` subcommands, `--apply` gate on `purge` |
| `internal/store` bulk-scan method(s) (new) | **New** | Subject-less `Store.ScanSpine`-shaped method(s), Scroll-based, parallel to `Reindex`/`PruneExpired` |
| `internal/store/store.go` authz surface | **Unmodified** | `decideBucket`/`decideRecord` and every `Subject`-taking method stay exactly as shipped — spine-review never calls them |
| `skill/engram/skills/curating-memory/SKILL.md` | **Modified** | extend rule-hygiene triad (duplicate/contradiction/rot) to all memories; add "is this still true" judgment; reuse existing consent protocol verbatim |
| MCP tool surface (`internal/server/tools.go`) | **Unmodified** | the skill introduces zero new tools/args |
| `internal/server` MCP introspection helper (new, small) | **New** | mirrors `buildCatalog`'s shape for the C conformance test's MCP-side walk |
| Conformance test (new) | **New** | table + test asserting named conditional-requirement rules appear in both cobra `Usage` strings and MCP `jsonschema` tags |
| `cmd/engram/client_list.go` | **Modified** | replace undone mutual-exclusivity prose with `MarkFlagsMutuallyExclusive` (#453) |
| `cmd/engram/client_common.go` exit-code constants | **Possibly modified** | depends on the #467 decision — see Build Order |

## Anti-patterns to avoid (violations, not trade-offs, in this codebase)

### Anti-pattern 1: composing `Search`/`List` for the spine-review scan

**What people would do:** reuse the existing, well-tested `Store.Search`/`Store.List` to "walk the
spine" for structural findings, since they're already bulk query paths.
**Why it's wrong:** both take a `Subject` and are authz-filtered to one actor's readable records —
a spine-wide structural sweep run this way silently reviews a slice and reports it as complete.
**Instead:** a new Subject-less bulk method, matching the five existing operator-command precedent.

### Anti-pattern 2: giving the semantic-judgment skill CLI or direct-store access

**What people would do:** let the skill shell out to `engram spine-review scan --output json` for
convenience, since the CLI already computes findings.
**Why it's wrong:** it hands agent-driven code a cross-owner, Subject-less view no MCP call would
ever grant that actor — a privilege-boundary violation, not a style choice.
**Instead:** the skill stays entirely on the MCP tool surface, actor-scoped by construction.

### Anti-pattern 3: a second, skill-specific "may I act on this" check

**What people would do:** have the skill call some new authz-aware helper to double-check it's
allowed to touch a record before calling `update_memory`/`delete_memory`.
**Why it's wrong:** `internal/store`'s existing gate already enforces this on every call; a second
check is redundant at best and, if it ever disagrees with the store's decision, a drift bug at
worst — authorization has exactly one enforcement point in this codebase.
**Instead:** let the tool call fail/succeed on the store's own decision; the skill's job is consent
before the call, not a parallel permission model.

## Data flow

```
Operator (human)                    Calling agent session
    │                                     │
    │ engram spine-review scan            │ mcp__engram__list_memory /
    │ (Subject-less Scroll,               │ search_memory / get_memory
    │  cmd/engram → internal/store,       │ (Subject-based, actor-scoped —
    │  bypasses decideBucket entirely)    │  same authz path every other
    │        │                            │  MCP tool uses)
    │        ▼                            │        │
    │  Qdrant (whole collection)  ◄───────┼────────┘
    │        │                            │
    │ engram spine-review verify          │ curating-memory skill: semantic
    │ (checks extract-before-delete       │ judgment, propose per-candidate,
    │  ordering — reads #355-shaped       │ user blesses/declines, then
    │  findings)                          │ update_memory / supersede_memory /
    │        │                            │ delete_memory (Subject-based,
    │ engram spine-review purge --apply   │ same store.go write gates as any
    │ (re-derives eligibility fresh,      │ other agent write)
    │  never trusts a stale ID list)      │
    │        │                            │
    │        ▼                            │
    │  Qdrant (deletes)                    │
    └──────────────┬───────────────────────┘
                   ▼
       Same Qdrant collection, two
       independent access tiers:
       Subject-less operator (A) and
       Subject-based actor (B) — never
       the same code path, never a
       shared authz check.
```

## Build order

Real dependencies, not conceptual tidiness:

1. **#453 (systematic mutual-exclusivity via `MarkFlagsMutuallyExclusive`).** Small, self-contained,
   no dependency on anything else. Do this **before** spine-review so its own subcommand flags
   (e.g., whatever future exclusive-flag pairs `purge`/`archive` grow) are built on the corrected
   mechanism rather than adding a *fourth* hand-rolled guard alongside `validateScopeCrossSpine`
   and `buildRemapSource`.
2. **#467 (one exit-code taxonomy, or a documented boundary).** A genuine blocker for spine-review:
   today, operator commands (`reindex`, `migrate-remap-owner`, `prune-expired`, …) return a plain
   `error` and always exit 1 (D-09's carve-out), while client commands use the `*cliError` 0/2/3/4/5
   taxonomy. `engram spine-review` is architecturally an operator command (per A) and would silently
   become a *third* undocumented case unless #467 is resolved first — even if the resolution is
   "operator commands deliberately keep exit 1, documented as the boundary," that decision must
   exist before spine-review's own error handling is written, not be improvised mid-build.
3. **C's audit machinery** (the conditional-requirement-rule table + the small MCP-side
   introspection helper + the conformance test itself). No code dependency on A or B — build in
   parallel with steps 1–2. Its full fix-sweep across *existing* `cmd/engram/*`/`tools.go` surfaces
   can run concurrently with A's build, since it touches unrelated files; but its **standard**
   (state conditional requirements inline, in both `Usage` text and `jsonschema` tags) should exist
   in written or test form before spine-review's own help text is finalized, so spine-review is
   built correct-by-reading from day one rather than becoming the next thing C has to retrofit.
4. **`engram spine-review` (A).** Built once 1–3 have settled: a Subject-less
   `internal/store` bulk-scan method (parallel to `Reindex`/`PruneExpired`), `scan`/`verify`/`purge`
   /`archive` subcommands, the corrected mutual-exclusivity mechanism, the #467-decided exit-code
   convention, and help text meeting C's standard. `verify` is validated against **#355 as a live
   fixture** — leave #355 unfixed until `verify` can detect it (see step 7).
5. **The semantic-judgment skill (B).** No code dependency on A — it only calls MCP tools that
   already exist, so its SKILL.md content can be authored any time, including in parallel with A.
   Its extract-before-delete handoff, however, is only end-to-end demonstrable once A's `verify`
   subcommand exists to mechanically confirm the ordering invariant the skill's extraction step is
   supposed to satisfy — so full acceptance of that coupling waits on step 4.
6. **#452 (CLI request timeout).** Fully independent — scoped to `newHTTPClient`/the Connect-lane
   client commands (`client_*.go`), a different code path from spine-review's operator-command
   pattern (which already has its own `signal.NotifyContext` + `context.WithTimeout` convention,
   `reindex.go:57`, `migrate.go:43`, to copy directly). No ordering constraint with 1–5; do it
   whenever convenient.
7. **#355 fix.** Deliberately **last** relative to A: PROJECT.md's own framing is that #355 *is* the
   drifted-citation failure class spine-review's `verify` step exists to detect, so it is the live
   acceptance fixture for step 4, not a prerequisite to it.
8. **Nyquist `VALIDATION.md` reconciliation.** Orthogonal documentation/process work across the six
   already-shipped v0.12.x phases with `status: draft` — zero technical dependency on 1–7.
   Parallelizable throughout; the one coupling worth naming is that v0.13.x's *own* new phases
   (spine-review, the skill, the audit) should reconcile their `VALIDATION.md` as each phase closes,
   so this milestone doesn't add three more files to the same backlog it's supposed to be clearing.

## Sources

- `/Volumes/Code/github.com/seanb4t/engram/cmd/engram/catalog.go` — self-describe JSON catalog
  (D-15), built from the live cobra tree; exit-code table sourced from `client_common.go` constants
- `/Volumes/Code/github.com/seanb4t/engram/cmd/engram/root.go` — root command wiring,
  `exitCodeFromError`, D-09 backward-compatible default-exit-1 carve-out
- `/Volumes/Code/github.com/seanb4t/engram/cmd/engram/reindex.go` — existing dry-run/apply
  precedent (`ReindexOptions.DryRun`, `reindexSummary` pure renderer)
- `/Volumes/Code/github.com/seanb4t/engram/cmd/engram/migrate.go` — `migrate-remap-owner`
  dry-run precedent; `buildRemapSource`'s hand-rolled exactly-one-of guard
- `/Volumes/Code/github.com/seanb4t/engram/cmd/engram/client_common.go` — exit-code taxonomy
  constants (D-09/D-17), `validateScopeCrossSpine` hand-rolled guard, `wrapRPCError`/`cliError`
- `/Volumes/Code/github.com/seanb4t/engram/cmd/engram/client_list.go` — undone mutual-exclusivity
  prose (#453's concrete target)
- `/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go` — `decideBucket`/`decideRecord`
  (Subject-based authz chokepoint) vs. `RemapOwner`/`Reindex`/`PruneExpired`/`BackfillShortIDs`
  (Subject-less operator tier) — the precedent spine-review extends
  (see `store.go:726,753,2443,2614,2087,2234`)
- `/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go` — MCP tool arg structs with
  `jsonschema` tags, the third, genuinely-independent interface surface
- `/Volumes/Code/github.com/seanb4t/engram/internal/server/argerror.go` — the `argError` envelope
  and `HintMutuallyExclusive`, the server-side (MCP/Connect) expression of the same "mutually
  exclusive" concept #453 is fixing on the CLI side
- `/Volumes/Code/github.com/seanb4t/engram/skill/engram/skills/curating-memory/SKILL.md` — the
  shipped rule-hygiene triad and "One-time rule backfill sweep" (`:122-249`), the v0.12.x Phase 6
  precedent this milestone's semantic-judgment skill extends
- `/Volumes/Code/github.com/seanb4t/engram/.planning/PROJECT.md` — milestone scope, decision
  history, and the #355/#453/#452/#467 issue framing this research is grounded against

---
*Architecture research for: engram v0.13.x — Curation & Self-Evidence*
*Researched: 2026-08-03*
