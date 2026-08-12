# Phase 3: Spine Curation — Structural (CLI) - Context

**Gathered:** 2026-08-06
**Status:** Ready for planning

<domain>
## Phase Boundary

An operator gets structural custody of a memory spine — inventory it, verify its citations still
point at real code, see near-duplicate candidates, and dispose of records safely (including an
archive tier) — delivered as `engram spine-review`, the sixth instance of the existing Subject-less
operator tier (`reindex` / `migrate-remap-owner` / `prune-expired` / `summarize-missing` /
`backfill-short-ids`).

Never by composing the Subject-gated `Search`/`List`, and never via a new authorization path.

Phase 4's semantic skill makes the *judgment* calls (is this still true, are these the same fact).
This phase supplies only what can be derived structurally, plus the safe disposal machinery those
judgments will eventually drive.

**Roadmapped requirements:** REQ-spine-scan, REQ-citation-drift-verify, REQ-near-duplicate-report,
REQ-purge-extract-gated, REQ-archive-tier.

**⚠ Scope expansions accepted during discussion — the roadmap does NOT yet reflect these:**

1. **D-04 flips `prune-expired`'s default.** No requirement in this phase names `prune-expired`.
   Making the destructive tier uniformly preview-by-default changes an already-shipped operator
   command's behavior — a breaking change with its own migration note.
2. **D-13 backfills `--output` across the whole operator tier.** REQ-spine-scan and its siblings
   concern `spine-review` only; adding `--output json|text` to `reindex`,
   `migrate-remap-owner`, `prune-expired`, `summarize-missing`, and `backfill-short-ids` is
   tier-wide work no requirement asks for.
3. **D-12 adds two verbs the roadmap goal does not enumerate.** The goal text names
   inventory / verify / consolidate-report / dispose; `archive` and `restore` are the
   operator-visible shape REQ-archive-tier's "retained and restorable" implies, but the goal's verb
   list should be updated to match.

`.planning/ROADMAP.md` and `.planning/REQUIREMENTS.md` must be updated through `/gsd-phase` —
**never** by hand (rule `8dfdhfs5nn`, extended to body granularity by memory `apfg4fe199`).
Planning must not proceed on the assumption that the roadmap already reflects these. This is the
third consecutive phase to hit this (Phase 1's D-04, Phase 2's D-05/D-10/D-11).

</domain>

<decisions>
## Implementation Decisions

### Command shape and the safety convention

- **D-01:** `spine-review` is a **nested subcommand tree** — `engram spine-review
  scan|verify|consolidate|purge|archive|restore`. It groups related capabilities under one noun,
  keeps `engram --help` from growing six top-level entries, and gives `--apply` a natural home on
  the one destructive leaf. This is the operator tier's first subcommand tree; the five existing
  commands are flat verbs, and Phase 2's golden walker (D-12) and catalog JSON (D-13) must both
  traverse the new depth correctly.
  — **Reversibility:** costly — the command path is a published CLI contract pinned by Phase 2's
  `--help` goldens and catalog JSON; renaming after release breaks operator scripts and both
  pinned artifacts.

- **D-02:** The **destructive tier inverts to preview-by-default with an explicit `--apply`**,
  rather than the `--dry-run` convention the other operator commands use. The decision turns on
  which way a typo fails: with `--dry-run`, a forgotten flag destroys; with `--apply`, a forgotten
  flag is a no-op. Note the current state this corrects — `prune-expired`, the tier's only
  destructive command today, has **no preview flag at all** (`cmd/engram/prune.go:59-62` carries
  only `--older-than` and `--timeout`). Rejected: supporting both flags tier-wide, because two ways
  to say one thing is the ambiguity Phase 2 spent itself removing.
  — **Reversibility:** one-way — `--apply` becomes the advertised safety contract for every
  destructive operator command; reverting re-arms the dangerous default on tooling operators will
  by then have written scripts against.

- **D-03:** Destructive-tier membership is **derived from Phase 2's D-11 blast-radius table**, never
  declared: a command is preview-by-default **iff** that table classifies it destructive. Same
  derive-don't-declare property as Phase 2's D-08 and D-12 — the boundary cannot be shortened to
  silence a gate, and a future destructive command inherits `--apply` automatically rather than by
  someone remembering. This is the direct payoff of D-11, which existed so `spine-review purge`
  would be *born* classified rather than retrofitted. Requires D-10's central annotation table to
  genuinely be the single source, which D-10 established.
  — **Reversibility:** reversible — a predicate over an existing table.

- **D-04:** `prune-expired` gets a **hard flip plus an upgrade note** — it stops deleting without
  `--apply` immediately, documented in `docs-site/src/content/docs/guides/upgrade.md` alongside
  Phase 1's exit-code migration. A deprecation window was rejected because it would carry the
  dangerous default through the very release this phase exists to make safe. The failure mode of a
  hard flip is benign and loud: a script that assumed deletion silently stops deleting — nothing is
  destroyed.
  — **Reversibility:** one-way — a published CLI behavior change; reverting is a second breaking
  change and a second migration note.

### Citation verification tiers

- **D-05:** `verify` reports a **fourth `unverifiable` tier, with a reason per entry**. Only
  `Kind: file` resolves against a local tree; `commit`, `url`, and `repo` (a version pin like
  `@v1.18.2`) are reported as unverifiable rather than silently skipped. Same rationale as the REQ's
  own moved/broken split: an operator reading a clean report must be able to tell "checked and fine"
  from "never looked at". Rejected: verifying `commit` via git, which buys coverage at the price of
  a git dependency in the CLI.
  — **Reversibility:** reversible — promoting a kind out of `unverifiable` later is additive.

- **D-06:** For a `file` citation, the tiers are **anchored on the `Excerpt` and bounded to the same
  file**. Valid = excerpt still present at `Locator`. Moved-but-valid = excerpt found at a different
  offset in the **same** file. Broken = excerpt absent from that file. Tight, cheap, no tree walk.
  This is exactly #355's shape (edit-above line drift), which is the drift class that would
  otherwise train an operator to ignore the verifier. Rejected: falling back to a whole-tree search
  on miss (short excerpts risk confident wrong matches) and fuzzy matching (a similarity threshold
  with no defensible value either cries wolf or hides real rot).
  — **Reversibility:** reversible — widening the search radius later is additive; the tier names
  do not change.

- **D-07:** `verify` resolves file `Ref`s **only against the repo it is run in**, matched by the
  scope's repo identity (`discovery:repo:<repo>`). Citations belonging to other repos land in
  `unverifiable` with reason "different repo". Zero configuration, and it reuses D-05's tier rather
  than inventing a second skip path. Rejected outright: resolving every `Ref` against CWD
  regardless of scope, which would check another repo's `internal/store/store.go` against yours and
  emit a confident wrong verdict — worse than not checking. A repeatable `--repo-root` mapping flag
  was also rejected for this phase (it would add a conditional-argument rule that must then bind on
  all six of Phase 2's surfaces).
  — **Reversibility:** reversible — a `--repo-root` mapping can be added later without changing
  the tiers.

- **D-08:** The broken tier is **split by cause on the line**: `broken: file missing` vs
  `broken: excerpt gone`. Same tier, but a missing file is usually a rename an operator fixes
  mechanically, while a vanished excerpt means the cached knowledge itself may be stale. The
  information is already available at verify time, so this costs nothing to report.
  — **Reversibility:** reversible.

### Purge gating and eligibility

- **D-09:** The extract-before-delete gate is **two-path — a per-record extraction link when
  present, with the milestone-summary precondition as the batch floor**. A candidate passes
  individually if it carries a link to the standalone record that preserved its content; absent
  links, the whole batch is gated on an authoritative milestone-summary record existing that covers
  the candidate set and postdates the newest candidate. This is derived from rule `7smp8vy9hr`'s own
  step 2 (write one authoritative milestone-summary) rather than invented, so it checks a real
  artifact. `purge` is usable now and gets strictly stronger when Phase 4's skill starts writing
  links. Rejected: operator attestation via a flag, which proves nothing — the
  attestation-as-theater shape Phase 2's D-14 rejected CODEOWNERS for. Pure per-record links were
  rejected alone because nothing writes them today, which would make `purge` inert until Phase 4
  lands and invert the roadmap's ordering.
  — **Reversibility:** costly — the gate is the requirement; weakening it later means re-auditing
  every path that can reach a delete.

- **D-10:** Eligibility is **structural classes plus free-form filters, with the filter path gated
  harder**. Classes are what `purge` can derive without judgment (superseded past a grace window,
  expired `not_after` lapsed, archived past a retention window) and need only D-09's gate. The
  free-form filter path (scope/category/tag/age) additionally requires an explicit scope. Both are
  needed because rule `7smp8vy9hr`'s own use case — collapsing a milestone's per-phase process
  records — is a semantic judgment no structural class expresses. Blast radius is matched to how
  much judgment the operator supplied.
  — **Reversibility:** reversible per class; the filter path's extra gate is a predicate.

- **D-11:** `--apply` deletes the **intersection of the preview set and the fresh re-derivation**.
  Records that became eligible since the preview are reported as "appeared since preview — not
  purged; re-run to include". This satisfies REQ-purge-extract-gated's re-derive-at-apply-time
  requirement (a record that became *ineligible* is spared) while guaranteeing the operator never
  destroys something they did not see. Rejected: aborting on any divergence (on a live spine with
  agents writing, the sets almost always differ, and a gate that fails constantly gets bypassed) and
  proceeding with the fresh set (lets `--apply` delete records never previewed). **This requires
  carrying the preview's result into apply — a manifest or token — which is precisely the
  tombstone/grace-window mechanism `research/SUMMARY.md` flags as unspecified by any single source.
  It is the phase's primary research item.**
  — **Reversibility:** costly — the manifest format is the contract between preview and apply;
  changing it invalidates any persisted preview.

- **D-12:** Archive is a **first-class state with `archive` / `restore` verbs**, an `archived`
  bucket in `scan` output, and an `archived_at` stamp visible via `get_memory`. `list_scheduled` is
  the existing precedent for "retained but not recalled" — surfacing what the recall gate hides is
  an established shape here, not a new idea. Rejected: extending `prune-expired`'s `not_after`
  soft-hide with an archive marker, because REQ-archive-tier demands distinctness from **both**
  other states, and if a payload field is the only difference then a naturally-expired record can be
  misread as archived. **The storage mechanism remains open and is roadmap-flagged for plan-time
  research** — this decision fixes the operator-visible shape and the distinctness constraint, not
  the implementation.
  — **Reversibility:** one-way — a new record state is observable through `get_memory` and the MCP
  surface; agents will begin branching on it.

### Report output contract

- **D-13:** `spine-review` reuses the client tier's **`--output json|text` with TTY
  auto-detection** (`cmd/engram/client_common.go:50-51`, `outputFormatFromConfig`), **and the flag
  is backfilled across the five existing operator commands**. Agents driving the headless CLI lane —
  which v0.12.x built precisely so engram could be driven without MCP — get machine-readable
  reports; a human at a terminal gets text without asking. `consolidate`'s candidate list is the
  output most obviously meant to be handed to something else.
  **Planning caution:** the two tiers deliberately diverge on `--timeout` — the client flag rejects
  `0` while the operator flag treats `0` as "disabled", and `client_common.go:53-56` documents this
  in the flag's own help text. The backfill adopts `--output` only; it must not incidentally unify
  `--timeout`.
  — **Reversibility:** costly — adding a flag to five shipped commands is additive, but it lands in
  Phase 2's pinned `--help` goldens and catalog JSON, so removal is a visible contract change.

- **D-14:** `verify` **exits 0 by default**, with an opt-in flag (`--fail-on broken` or similar)
  turning findings into a nonzero exit. Default keeps "the command worked" separate from "the data
  is healthy" — the distinction Phase 1 spent itself establishing — while still giving Phase 5 a CI
  gate for the #355 fixture without output parsing. The flag introduces a conditional-argument rule
  that **must be registered in Phase 2's rule registry and bind on the applicable surfaces per
  D-05/D-08**.
  — **Reversibility:** reversible.

- **D-15:** `consolidate` reports **ranked pairs with scores** — each row is (record A, record B,
  cosine score), sorted by score, no clustering and no default threshold. This matches how
  `search_memory` already exposes a raw per-result score, so it introduces no new concept. Rejected:
  transitive clustering above a threshold, which chains unrelated records together (the documented
  RAG-dedup failure mode `research/SUMMARY.md` cites) and needs a tuning knob with no defensible
  value. `consolidate` never merges or mutates, so its report should not pre-decide either.
  — **Reversibility:** reversible.

### Claude's Discretion

- The exact verb spellings and flag names within `spine-review` (`--fail-on` vs `--strict`,
  `--min-score` naming, whether `archive`/`restore` take ids or filters).
- Which specific health signals `scan` reports (count by scope/category, summary coverage, citation
  age distribution, superseded/archived counts) — the requirement says "inventory and health
  signals" without enumerating them.
- Whether the preview→apply manifest (D-11) is persisted to disk, held in the payload as a
  tombstone marker, or expressed as an opaque token the operator passes back. Research item.
- How a "milestone-summary record" is identified for D-09's floor — tag convention, category, or a
  dedicated marker.
- The retention window default for archived records under D-10's third structural class.
- Whether the operator-tier `--output` backfill (D-13) lands as its own commit ahead of the
  `spine-review` work, mirroring how Phase 1 landed its before-table first.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### The operator tier this phase joins
- `cmd/engram/prune.go` §26-63 — `prune-expired`: the tier's only destructive command, its
  `--older-than`/`--timeout` flag set with **no preview flag**, and `pruneCutoff`'s grace-window
  shape. D-02 and D-04 change this file's contract.
- `cmd/engram/migrate.go` §111-158 — `migrate-remap-owner`: the tier's `--dry-run` idiom, its
  `[dry-run] would remap %d record(s)` output line, and the mutually-exclusive
  `--from`/`--from-missing`/`--from-anon` flag group.
- `cmd/engram/summarize.go` §34-89 — `summarize-missing`: the closest existing analog to a scoped
  sweep (`--scope`/`--all-scopes`/`--older-than`/`--limit`/`--dry-run`/`--timeout`), including the
  `--scope`-or-`--all-scopes` conditional rule already in Phase 2's registry.
- `cmd/engram/reindex.go` §36-120 — `reindex`: `reindexSummary` is kept pure (no I/O) so dry-run vs
  cutover wording is unit-testable without a live Qdrant. **The precedent to copy for
  `spine-review`'s preview/apply wording.**
- `cmd/engram/backfill.go` — `backfill-short-ids`, the fifth tier member.

### Output format and exit codes
- `cmd/engram/client_common.go` §50-56 — the `--output` flag D-13 adopts, **and** the documented
  client-vs-operator `--timeout` divergence (`0` rejected vs `0` disables) the backfill must not
  disturb.
- `cmd/engram/client_common.go` §190-215 — `outputFormatFromConfig`: the TTY-detection mapping and
  the note that the registry validator already rejected illegal values, so there is no second
  rejection site.
- `cmd/engram/operror.go` — the Phase 1 exit-code taxonomy and `classifyOperatorErr` /
  `classifyOperatorErrConstruction`, which every `spine-review` RunE must route through.
- `cmd/engram/catalog.go` — `buildCatalog`, which derives the self-describe document from the live
  cobra tree; D-01's nesting must traverse correctly here and in Phase 2's golden walker.

### Store surface (Subject-less operator entry points)
- `internal/store/store.go` §286-292 — the `Citation` struct: `Kind` (file|commit|url|repo), `Ref`,
  `Locator`, `Pin` (aging anchor at store time), `Excerpt` (cached substance). D-05..D-08 are
  entirely determined by these five fields.
- `internal/store/store.go` §2079-2133 — `PruneExpired`: the existing collection-wide, authz-free
  delete, and its **best-effort** returned count. The shape `purge` extends.
- `internal/store/store.go` §2135, §2298, §2332, §2443 — `CountOwnerless`, `CountAnonymousBucket`,
  `MigrateSetOwner`, `RemapOwner`: the Subject-less scan/mutate precedents `scan` follows.
- `internal/store/store.go` §846-870 — the recall gate (`not_before`/`not_after` window filter) and
  §935, §1029, §1162, §1392 — the `superseded_by` `IsEmpty` soft-hide. D-12's archived state must be
  observably distinct from both.
- `internal/store/store.go` §1317-1340, §1360 — `ScheduledPending`/`ScheduledExpired` and
  `ListScheduled`: the "surface what the recall gate hides" precedent D-12 cites.
- `internal/server/tools.go` §856-890 — `validateCitations` / `validCitationKind`: the four accepted
  kinds and the excerpt-length bound, the contract `verify` reads against.

### Phase and milestone context
- `.planning/ROADMAP.md` §313-357 — the phase goal, the five success criteria, the dependency notes
  on Phases 1 and 2, and the research flag.
- `.planning/REQUIREMENTS.md` §84-105 — the five requirements verbatim, including
  REQ-archive-tier's explicit "open at definition time" note.
- `.planning/REQUIREMENTS.md` §167 — the accepted posture that if Qdrant snapshot/backup tooling is
  absent, REQ-purge-extract-gated's precondition gate carries the full weight of recovery safety.
- `.planning/research/SUMMARY.md` §334-346 — Gaps to Address. **Note the phase numbers in that file
  are off by one relative to the current roadmap** (it says "Phase 4's plan-phase" for the tombstone
  gap, which is this phase, Phase 3). The tombstone/grace-window gap is D-11's research item.
- `.planning/phases/02-interface-discoverability/02-CONTEXT.md` — Phase 2's D-01..D-14. D-11
  (catalog blast radius) is what D-03 derives from; D-05/D-06/D-08 (six bound surfaces, generated
  regions, derived applicability) govern any conditional rule this phase adds, including D-14's.
- `.planning/phases/01-interface-enforceability/01-CONTEXT.md` — the exit-code taxonomy and the
  before-table-lands-first commit ordering D-13's discretion note echoes.
- `docs-site/src/content/docs/guides/upgrade.md` — D-04's migration-note target, already carrying
  Phase 1's exit-code migration.
- GitHub issue #355 — "Phase 14 docs/comment nits: stale tools.go line-number citations + dangling
  cross-ref". Phase 5's live acceptance fixture for `verify`; D-06's same-file moved tier is chosen
  specifically to classify this drift correctly rather than as broken.

### Governing memory records and rules
- Rule `7smp8vy9hr` — the 4-step extract-before-delete procedure. **Read the full text via
  `get_memory 7smp8vy9hr`, not the summary**: D-09's gate is derived from its step 2 (write one
  authoritative milestone-summary), and its step 4 (never touch reusable codebase facts) bounds what
  `purge` may ever classify eligible.
- Rule `n6m4as49mr` — explicit pathspec on every `git commit` when agents share a working directory.
- Rule `8dfdhfs5nn` / memory `apfg4fe199` — never invent structure in `.planning/` artifacts; the
  three scope expansions above reach the roadmap only via `/gsd-phase`.
- Memory `55zra87def` — an exported struct is not a capability token; use an unexported provenance
  marker so forgery is unrepresentable rather than merely tested. **Directly applicable to D-11's
  preview manifest**: if a manifest is a plain exported struct, a caller can forge one and defeat
  the intersection guarantee.
- Memory `p1vqxqhxrm` — `go clean -testcache && task test` before any phase-completion or pre-PR
  gate on a cross-package contract change. This phase changes `cmd/engram` behavior consumed by
  `internal/e2e`, so it applies directly.
- Memory `jb33frww29` — any `cmd/engram` cobra-tree snapshot is order-dependent (lazy `-h`
  registration, shared command singletons, nil-vs-empty slice marshalling). **D-01 adds the first
  subcommand tree to that snapshot** — stress the goldens and catalog JSON across several
  `-shuffle=<seed>` runs, since a single green run proves nothing.
- Memory `k66tenzbhy`, `akf6xesf64` — cobra/pflag and exit-code harness traps in `cmd/engram`;
  `pflag.Flag.Changed` latches across the whole test binary, which matters for any test asserting
  `--apply` was or was not supplied.
- Memory `dnanmnkqmg` — observation-shaped acceptance criteria must be re-performed during recovery;
  green tests are not completion. D-02/D-04's fail-first proof (that a bare `purge`/`prune-expired`
  really does not delete) is exactly this shape.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `outputFormatFromConfig` (`cmd/engram/client_common.go:200`) — the TTY-detection mapping D-13
  adopts wholesale; takes `isTTY` as a parameter specifically so table tests can force both
  branches without a pty.
- `reindexSummary` (`cmd/engram/reindex.go:93`) — a pure, I/O-free summary formatter so dry-run vs
  cutover wording is unit-testable without a live Qdrant. The model for `spine-review`'s preview
  and report rendering.
- `pruneCutoff` (`cmd/engram/prune.go:57`) — the grace-window helper, already pure and unit-tested;
  D-10's archive-retention window is the same shape.
- `PruneExpired` (`internal/store/store.go:2087`) — the existing collection-wide authz-free delete
  `purge` extends. Note its returned count is explicitly **best-effort**, which shapes what a
  preview can honestly promise.
- `ListScheduled` + `ScheduledState` (`internal/store/store.go:1317-1360`) — the existing
  "surface what the recall gate hides" mechanism; D-12's `archived` bucket follows it.
- `classifyOperatorErr` / `classifyOperatorErrConstruction` (`cmd/engram/operror.go`) — Phase 1's
  routing every operator RunE uses; `spine-review` inherits it rather than adding exit-code logic.
- Phase 2's blast-radius table — D-03 reads it rather than declaring a second list.

### Established Patterns
- **Derived, never hand-maintained.** `buildCatalog` derives from the cobra tree; Phase 2's D-08
  derives surface applicability. D-03 continues this by deriving destructive-tier membership from
  the blast-radius table rather than naming commands.
- **Operator tier is Subject-less and collection-wide.** Every operator store method takes no
  `Subject` and applies no owner filter, unlike `Search`/`List`/`Get`. `scan` must follow this —
  the phase goal forbids composing the Subject-gated read path.
- **Pure formatters, testable without Qdrant.** `reindexSummary` and `pruneCutoff` both isolate
  wording and arithmetic from I/O. The preview/apply divergence report (D-11) should too.
- **Client and operator tiers deliberately differ.** `--timeout` means opposite things in each
  (`0` rejected vs `0` disables), documented in the client flag's own help text. D-13 crosses one
  flag between tiers and must not assume the rest should follow.

### Integration Points
- `cmd/engram/` — a new `spine_review.go` (or a small package) registering the nested tree on
  `rootCmd`, alongside the five existing operator commands.
- `internal/store/` — new Subject-less methods for scan aggregation, citation enumeration,
  vector-neighbor query (reusing stored vectors, no re-embedding), archive/restore, and the gated
  purge.
- `cmd/engram/catalog.go` + Phase 2's golden walker — both must handle D-01's added depth.
- Phase 2's rule registry — D-14's `--fail-on` conditional rule, and any conditional flag rule
  `purge` introduces, register here and bind on the applicable surfaces.
- `docs-site/src/content/docs/guides/upgrade.md` — D-04's migration note.

</code_context>

<specifics>
## Specific Ideas

- The user again chose the **strongest-guarantee** option at nearly every step, continuing the
  pattern Phase 2 recorded: derived over declared (D-03), structural artifact over attestation
  (D-09), intersection over trust (D-11), first-class state over a reused marker (D-12). Planning
  should resolve ambiguity in that direction — *make the invariant unrepresentable rather than
  merely tested*.
- The two exceptions are deliberate and both favor **honest narrowness over coverage**: D-05's
  `unverifiable` tier admits what the verifier did not check rather than guessing, and D-06 keeps
  the moved search inside one file rather than risking confident wrong matches. Do not "improve"
  these into wider searches during planning.
- D-14 is the one place the user chose the *weaker* default (exit 0) — explicitly to protect
  Phase 1's separation of "the command worked" from "the data is healthy". The CI gate exists, but
  behind a flag.
- `reindex`'s pure-formatter discipline was cited as the shape for preview/apply wording. Treat it
  as the reference implementation, not an analogy.
- Phase 2's D-11 was landed specifically so `spine-review purge` would be born classified. D-03
  cashes that in — the blast-radius table is now load-bearing for runtime safety behavior, not just
  documentation.

</specifics>

<deferred>
## Deferred Ideas

- **A repeatable `--repo-root` mapping so one run can verify a multi-repo spine.** Raised while
  deciding D-07. Deferred because it introduces a conditional-argument rule that must then bind on
  Phase 2's six surfaces, and because D-05's `unverifiable` tier already reports other-repo
  citations honestly rather than silently.
- **Verifying `commit` citations via local git history.** Considered under D-05; deferred because
  it adds a git dependency to the CLI for one of four kinds.
- **Transitive clustering for `consolidate`.** Considered under D-15; deferred as a Phase 4
  semantic concern — grouping is a judgment about identity, which is exactly what the skill exists
  to make.
- **Unifying `--timeout` semantics between the client and operator tiers.** Surfaced while scoping
  D-13's backfill. The divergence (`0` rejected vs `0` disables) is currently documented in help
  text rather than resolved; reconciling it is its own decision with its own migration note.

</deferred>

---

*Phase: 3-Spine Curation — Structural (CLI)*
*Context gathered: 2026-08-06*
