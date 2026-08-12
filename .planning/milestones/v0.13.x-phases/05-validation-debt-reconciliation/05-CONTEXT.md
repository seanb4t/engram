# Phase 5: Validation Debt Reconciliation - Context

**Gathered:** 2026-08-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Repair the small number of validation records that are actually wrong, and fix #355. That is the
whole phase.

**The scope was cut during this discussion.** The ROADMAP frames this phase around clearing
"six inherited Nyquist drafts plus one phase with none." That premise is **factually stale** — the
audit was run live during this discussion (see `<code_context>` for the method and the numbers) and
found the v0.12.x debt already cleared. The remaining defect surface is one misnamed test pattern,
four `status: draft` frontmatter flags on files whose rows resolve fine, and #355's two drifted
comment anchors plus a dangling docs cross-ref.

The user's explicit direction, after seeing those numbers: **fix the real defects, drop the
ceremony.** No CI gate, no durable mechanism, no reconciliation report artifact, no retroactive
rewrite of correct records. This is likely one plan, not a wave structure.

Not in scope: any gate or committed script that audits `.planning/**`; any edit to archived
milestone artifacts; staging fixture memory records to exercise `spine-review verify`.

</domain>

<decisions>
## Implementation Decisions

### What is actually broken (the work list)

- **D-01:** `03.1-VALIDATION.md`'s store-side command for `REQ-merge-idempotency` names
  `TestSupersedeIdempotency`, which **does not exist**. It is a plan-time seed nobody repointed when
  the phase closed. The requirement is **not** a coverage hole — the server half resolves to three
  real tests (`TestSupersedeMemoryIdempotencyReplay`,
  `TestSupersedeMemoryIdempotencyDifferentTargetSetConflicts`,
  `TestSupersedeMemoryIdempotencyReorderedSetReplays`) and the store-side coverage shipped as
  `TestSupersedeConcurrentKeyedDisjointTargetsCannotBothLand` and
  `TestPayloadRoundTripsIdempotencyFingerprint`. Repoint the command; flip the file's four
  `⬜ pending` rows to their real state.

- **D-02:** `01`, `02`, and `03.1` flip `status: draft` → `validated`. Their rows resolve against
  HEAD (01: 5/5 real rows, 02: 9/9, 03.1: 5/6 pending D-01's repoint). `03` is already `validated`.

- **D-03:** `04-VALIDATION.md` is **partially** reconciled: resolve the two structural rows that
  have commands and passed (the paired forbidden-tool grep, and the `72a32c58..HEAD` no-new-
  server-code diff). Leave the `REQ-consent-adversarial-proof` cold-read row `⬜ pending` and
  `nyquist_compliant: false`. That requirement is genuinely open — the cold read hit its run cap at
  `NOT-OBTAINED`, the user accepted the non-result, and `04-VERIFICATION.md` carries an explicit
  override stating it does not assert SC-3 was proven (engram `sgf61qexhh`, broken window id=3).
  Flipping it would manufacture the exact false green this phase exists to prevent.

- **D-04:** #355 is repaired as the plain docs fix it is, following the issue's own prescription:
  cite symbol names without line numbers in `internal/retrievaleval/retrieval_eval_test.go` (IN-01),
  and point the OpenRouter row at `[Embedding model recipes](/guides/embedding-models/)` in
  `docs-site/src/content/docs/guides/embedding-instructions.md:106` (IN-02). Both confirmed still
  drifted: the cited `tools.go:706` default now lives at `tools.go:1617`; `tools.go:508-515` is now
  the `storeArgs` struct.

### Requirement and roadmap text corrections

- **D-05:** `REQ-citation-fixture-355`'s "the repair is used to calibrate `verify`'s false-positive
  rate" clause is **dropped**, along with the matching half of SC-3. The premise was never true:
  #355's anchors are Go source *comments* and a docs cross-ref, while `spine-review verify` reads
  stored `Citation.Excerpt` payloads out of Qdrant via `EnumerateCitations` — it cannot see them.
  Making the sentence literally true would require staging fixture memory records citing those
  anchors, i.e. inventing test data to exercise a path #355 never touched. Rejected as manufactured
  work. — **Reversibility:** reversible — a requirement's wording, no shipped contract depends on it.

- **D-06:** `REQ-nyquist-reconciled`'s "six at `status: draft`, plus the one phase with no file" is
  corrected to what the audit found. This is a **value** fix to existing ROADMAP/REQUIREMENTS prose,
  not new structure (rule `8dfdhfs5nn` permits filling in values in shapes GSD already writes).

### Method — and why it is not a deliverable

- **D-07:** The oracle's reference tree is **per-milestone**: archived milestones resolve against
  their own merge commit (v0.12.x = `906a5cf6`), the active milestone against HEAD. This is not a
  style preference — resolving archived rows against HEAD manufactures phantom drift out of
  deliberate later refactors, which is the over-flagging failure SC-3 warns about for `verify`
  ("does not train an operator to ignore the verifier"). It produced a wrong finding during this
  discussion before being corrected. Only the HEAD half is needed for the descoped work, but the
  rule matters if anyone re-runs the sweep.

- **D-08:** A reconciled row records the number of tests its pattern resolved to; the asserted bar is
  ≥1, not an exact count. Pinning exact counts would red-build on ordinary sibling-test additions —
  the pinned-vs-derived trade-off this repo has consistently settled toward derived.

- **D-09:** **No durable mechanism ships.** No CI gate, no `task` target, no Go test reading
  `.planning/**`. Rationale accepted by the user: CI already runs `go test ./...` on every PR, so a
  `-run` string in a planning markdown proves nothing about the code — auditing it is bookkeeping,
  and gating the audit is bookkeeping about bookkeeping. Accepted consequence: a future milestone
  re-derives the method from scratch.

- **D-10:** **Archived milestone artifacts are immutable.** Nothing under
  `.planning/milestones/**` is edited. Accepted consequence:
  `v0.12.x-phases/07-cli-cross-spine-wiring/07-VALIDATION.md` keeps its unresolvable `ScopeGuard`
  command in the row — mitigated by the fact that the file already documents that exact drift in its
  own prose at line 137.

- **D-11:** **No reconciliation report artifact.** Findings that matter are captured in this
  CONTEXT.md and in the edits themselves. (The user first chose report-only over a durable gate,
  then descoped the report too once the audit turned up almost nothing.)

### Claude's Discretion

- Whether the audit sweep is re-run at plan/execute time to re-confirm the numbers below, or whether
  the counts recorded here are taken as given. Re-running is cheap (~15 min, dominated by
  `go test -list ./...`); the risk of taking them as given is that HEAD moves.
- Exact wording of the D-05/D-06 ROADMAP and REQUIREMENTS edits, and whether they go through
  `/gsd-phase edit` or a direct value edit.
- Whether the four `⬜ pending` rows in `03.1-VALIDATION.md` and the two in `04-VALIDATION.md` are
  marked `✅ green` or given a distinct "reconciled after close" marker.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### The files being edited

- `.planning/phases/03.1-merge-supersession-supersede-memory-accepts-multiple-targets/03.1-VALIDATION.md`
  — D-01's misnamed row is at line 50; the four pending rows are lines 47–50.
- `.planning/phases/01-interface-enforceability/01-VALIDATION.md` — D-02.
- `.planning/phases/02-interface-discoverability/02-VALIDATION.md` — D-02.
- `.planning/phases/04-spine-curation-semantic-skill/04-VALIDATION.md` — D-03; rows at lines 45–48,
  and lines 52–61 explain why the adversarial row deliberately carries no automated command.
- `internal/retrievaleval/retrieval_eval_test.go:23-24, :94-95` — #355 IN-01, both anchors confirmed
  still drifted.
- `docs-site/src/content/docs/guides/embedding-instructions.md:106` — #355 IN-02.

### Why the phase's own premise is stale

- `.planning/milestones/v0.12.x-MILESTONE-AUDIT.md` §2 — the table that recorded the six-draft debt.
  Accurate when written; superseded by the reconciliation that landed on the v0.12.x branch **after**
  the audit and before the squash merge.
- `.planning/milestones/v0.12.x-phases/07-cli-cross-spine-wiring/07-VALIDATION.md:137-138` — the
  model of an honest reconciliation: it caught its own `ScopeGuard` drift and wrote it down rather
  than quietly repointing.
- `.planning/milestones/v0.12.x-phases/02-headless-cli-client/02-VALIDATION.md` — reconstructed
  retroactively by `/gsd-validate-phase 2`; this is the "one phase with none" the ROADMAP still
  lists as outstanding.
- Commit `c42b82d4` "docs(01-02): retract the D-17 catalog note and its guarding test" — the
  deletion that makes `TestCatalogDocumentsFlagParseExitCode` (present at `906a5cf6`) absent at HEAD.
  The worked example behind D-07.

### Why #355 cannot calibrate verify

- `internal/store/spine.go:296-360` — `CitationRecord` / `EnumerateCitations`: `verify`'s input is
  stored Qdrant payload citations, never source-file comments.
- `cmd/engram/spine_review_verify.go:622` — the command's own contract line.

### Contract and conventions

- `.planning/STATE.md:155` — the standing blocker entry that names this phase's own trap:
  *"This is v0.13.x Phase 5's own deliverable — do not let Phase 5 reproduce the bug it exists to
  close."* Durable record `bsbsvn4hbc`.
- `.planning/ROADMAP.md` §"Phase 5: Validation Debt Reconciliation" — the SC-1/SC-3 text D-05 and
  D-06 correct.
- `.planning/REQUIREMENTS.md` §"Validation Debt" — `REQ-nyquist-reconciled`,
  `REQ-citation-fixture-355`.

</canonical_refs>

<code_context>
## Existing Code Insights

### The audit, already run

The sweep SC-1 asks for was executed during this discussion. Method: extract every `-run` pattern
from each `VALIDATION.md`, undo markdown table escaping, skip boilerplate prose, and match the
top-level element against the names `go test -list '.*' ./...` reports (1,027 at HEAD, 741 at
`906a5cf6`).

| Scope | Result |
|---|---|
| v0.12.x, resolved at `906a5cf6` | **89/90** real rows clean; the one miss is 07's self-documented `ScopeGuard` |
| v0.13.x 01 / 02 / 03 | clean on real rows |
| v0.13.x 03.1 | one stale name (D-01) |
| v0.13.x 04 | zero `go test` rows — skill phase |

**Four traps the method has to dodge**, each of which produced a wrong answer before being fixed:

1. **Markdown table escaping.** Rows write `TestA\|TestB`; the `\|` is table escaping, not regex.
   Read literally, it turns an alternation into one impossible literal and reports ~10 phantom
   failures.
2. **Boilerplate vs. rows.** Every file's template carries `go test -run X ./pkg/...` as its
   *false-green cautionary example*, plus `-run <TestName>` placeholders in the Sampling Rate
   prose. A naive sweep flags the warning against false greens as a false green.
3. **`go test -list` does not enumerate subtests.** The second element of any `-run 'TestX/sub'` is
   unverifiable by this oracle. Validate the top-level element only, and say so.
4. **Wrong reference tree** — D-07.

### Established Patterns

- **Mechanical over reading-based guarantees** — `TestUpgradeGuideNamesEveryChangedCommand` derives
  its list from `exitCodeBaseline` rather than a second hand-maintained one. D-09 deliberately does
  **not** extend this pattern to `.planning/**`: the seven existing doc-binding tests
  (`supersededocs_test.go`, `verbtabledocs_test.go`, `conformance_test.go`, `docsync_test.go`, …)
  all bind to *shipped* surfaces. None reads planning artifacts, and this phase does not make it
  eight.
- **A negative assertion is only evidence when paired with a positive one** proving the matcher
  works on the target (`04-VALIDATION.md` correction #2). Trap 1 above is the same failure in a new
  costume — a pattern that cannot match anything reports clean.
- **`status: validated` is itself an unverified claim.** The frontmatter flag is the same class of
  assertion as a `-run` pattern: written by hand, parsed by a tool, never checked against reality
  until someone looks.

### Integration Points

- `.planning/**` only, plus one Go test-file comment and one docs-site line. **No `internal/`,
  `cmd/`, `proto/`, or `gen/` behavior changes** — nothing in this phase can affect the binary.

</code_context>

<specifics>
## Specific Ideas

- **The user cut this phase down mid-discussion.** After four questions had accumulated a per-
  milestone oracle, recorded match counts, an immutability posture and a report artifact, the
  challenge was direct: *"What are we trying to do here? Why are we auditing things, capturing
  things to files for CI later? Seems like a hell of a lot of over engineering."* That judgment was
  correct and it is the governing constraint on this phase. A plan that reintroduces a gate, a
  report artifact, or a retroactive rewrite has misread the intent.

- **The signal was there earlier.** Three consecutive answers — no CI gate, no rule, report-only —
  all pointed the same direction before the challenge landed. Downstream agents should read a run of
  minimizing answers as a scope instruction, not as three independent preferences.

- **Don't let the requirement's wording drive the design.** "nonzero, expected match count — not
  merely re-run and checked for exit 0" reads like it is asking for a mechanism. It is asking for
  someone to look once.

</specifics>

<deferred>
## Deferred Ideas

- **A durable validation-row gate** (`task validation:reconcile`, or a Go test over `.planning/**`).
  Declined in D-09, not lost. If a future milestone re-derives this debt from scratch, that
  recurrence is the evidence that would justify building it.

- **Recording the reconciliation method durably** (engram record, or a proposed repo rule). Offered
  and declined — the method lives in this CONTEXT.md's `<code_context>` instead.

- **Correcting the ROADMAP Progress table's own drift.** The table lists v0.13.x Phase 1 as
  "Not started 0/4" and Phase 2 as "0/4 Not started" while both are complete with every REQ checked,
  and v0.12.x Phase 1 as "In Progress". Same *class* of stale record as this phase's subject, but
  it is roadmap bookkeeping rather than validation debt. Noted, not folded.

### Reviewed Todos (not folded)

- **supersede_memory cannot merge two records into one without a delete** — matched by
  `todo.match-phase 5` at score 0.6 on keyword overlap only. Already delivered by Phase 03.1; the
  match is a false positive.

</deferred>

---

*Phase: 5-Validation Debt Reconciliation*
*Context gathered: 2026-08-11*
