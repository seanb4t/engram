# Phase 7: CLI Cross-Spine Wiring - Context

**Gathered:** 2026-08-02
**Status:** Ready for planning

<domain>
## Phase Boundary

Make the already-shipped `cross_spine` capability reachable from the shipped CLI. v0.12.x Phase 3
landed `cross_spine` plus `searched_scopes` / `scopes_truncated` on the Connect API; v0.12.x Phase 2
shipped `engram search` / `list` / `store` and never wired any of it. This phase closes that seam and
nothing else — it is a CLI-only change against an already-tested server capability, adding no proto
fields, no handler behavior, and no store changes.

Two concrete defects define "done":

1. `engram search` and `engram list` cannot request cross-spine recall at all — no flag exists and
   neither request literal sets the field.
2. Neither command reads `searched_scopes` / `scopes_truncated`, so the provenance half of Phase 3
   is invisible to CLI callers.

A third, sharper problem was found during discussion and is in scope as a consequence: because
`effectiveSearchScope` (`internal/server/tools.go:1374-1382`) rejects an empty scope unless
`cross_spine` is true, `engram search --query x` with **no** `--scope` does not quietly search one
spine — it fails. The most natural invocation of the shipped CLI errors out today.

**Out of scope:** `engram store` (no scope-recall path); any change to the server, proto, or store;
the surface-wide interface audit (captured as backlog Phase 999.2).

</domain>

<decisions>
## Implementation Decisions

### Governing principle

- **D-00 (the CLI is correct-by-reading):** Stated by Sean 2026-08-02 as a general principle, not a
  local choice: every command, flag, and parameter must be discoverable and usable correctly from its
  help text and from the naming of operations and parameters. No teaching by example. No
  error-and-wait-to-see-how-it-works. No surprises. Verbatim: *"the flags, cli help, all of this
  should read well and be discoverable for an agent or human. no surprises, no error and wait to see
  how it works. The goal is to NOT need to teach by example how to use the cli, it should be evident
  from its help and the naming of its operations and parameters."*

  **Consequence for this phase:** flag help text and the self-describe catalog entry are
  **deliverables with acceptance criteria**, not documentation written after the fact. A plan that
  wires the field and leaves help text as a trailing chore has not satisfied this phase. Errors
  (D-01, D-04) are backstops for a caller who did not read; they are never the mechanism by which the
  interface is learned.

  — **Reversibility:** reversible — a principle governing this phase's acceptance bar; the surface-wide
  application is deferred to backlog 999.2.

### Flag design and the empty-scope failure

- **D-01 (passthrough bool plus a client-side pre-flight rejection):** `--cross-spine` maps 1:1 to
  the proto `cross_spine` field with no client-side inference. A missing `--scope` is **not** silently
  promoted to cross-spine — that would re-introduce, one layer up, exactly the inference Phase 3's
  D-04 banned and `TestConnectCrossSpineNotInferred` pins as intentional. Instead the CLI catches the
  empty-scope-without-cross-spine case itself, before dialing, and exits 2 (Phase 2 D-09/D-17 reserve
  exit 2 for the client's own validation). **Amended under D-00:** the error's wording is a pointer
  back to documented behavior, not a mini-tutorial — the help text does the teaching, the error only
  catches someone who skipped it.

  — **Reversibility:** reversible — local to `cmd/engram`; no wire contract changes.

- **D-02 (one shared guard helper, never per-command):** The pre-flight check lives in a single
  helper in `cmd/engram/client_common.go`, called by both `searchCmd` and `listCmd`. Follows Phase 2's
  D-10 precedent ("one shared mapper over the Connect error code, never per-command") so a future
  third command that grows a `--scope` cannot forget the guard.

- **D-03 (a parity test pins the client guard against the server's rule):** The client guard
  duplicates `effectiveSearchScope`'s rule, so it can drift into rejecting calls the server would
  accept. A test asserts the two agree across the full input matrix (empty/non-empty scope ×
  cross-spine on/off). Both sides live in this repo, so the test compiles against both and goes RED
  the moment either moves. Precedent: `client_common_test.go:29-53` already pins the exit-code table
  against `connect.Code` with a count assertion.

- **D-04 (`--scope` and `--cross-spine` are mutually exclusive):** Passing both exits 2 before
  dialing. The server's Phase 3 D-02 behavior is to ignore the scope and log at Info, but that log
  lands in server stderr where the calling agent never sees it — the caller just receives a wider
  result set than it asked for, with no signal its filter was discarded. Silent discard of an
  explicitly-typed filter is precisely the "surprise" D-00 forbids. Precedent for a
  mutually-exclusive pair in this file set: `--offset` / `--cursor-mode`, documented at
  `cmd/engram/client_list.go:86`. **Each flag's help names the other**, so the relationship is
  discoverable from either entry point.

### Provenance surfacing

- **D-05 (coverage footer on stdout, reusing the existing footer pattern):** On a cross-spine call
  the text renderer prints a coverage footer after the table. It reuses the pattern `engram list`
  already has at `client_list.go:70-75`, where `total:` / `next_page_token:` go to **stdout** as data
  rather than stderr as a diagnostic. This keeps text and JSON in agreement about what the coverage
  information *is*. Note: JSON already emits both fields today, because `renderJSON` sets
  `EmitDefaultValues: true` (`client_common.go:264-268`) — so only the text lane is genuinely blind.
  A count-based footer was chosen over naming every scope; naming them is unbounded in width and
  would need its own truncation rule layered on the server's `scopes_truncated`.

- **D-06 (the footer prints only on cross-spine calls):** On a scope-confined call the server
  returns `(nil, false, nil)` and issues no `ListScopes` query at all (Phase 3 D-13), so there is
  genuinely nothing to report. Printing a synthesized "searched: <the one scope>" would state
  something the server never reported. Consequence: **existing `engram search` and `engram list`
  output is byte-identical for every invocation that does not pass `--cross-spine`.**

### Discoverability and docs

- **D-07 (flag help and the self-describe catalog are the load-bearing surfaces):** These are the
  two surfaces an agent reads at runtime, so under D-00 they are the feature. The catalog is derived
  from the live command tree (`02-03-PLAN.md`), which means it *may* pick the new flag and its help
  text up automatically. **That must be verified, not assumed** — if it derives cleanly, this is
  nearly free; if it does not, it is a real task. Open question for research/planning.

- **D-08 (docs-site CLI reference and an upgrade note):** Both updated. The upgrade note follows the
  v0.12.0 precedent set in `05-03`, telling existing CLI users a capability became reachable.

- **D-09 (CLAUDE.md records that the capability is CLI-reachable, not how to use it):** The Memory
  contract section currently describes `cross_spine` as MCP-only. It gains the fact that the CLI can
  reach it; it does **not** gain flag syntax or usage examples. Sean, verbatim: *"include it, just not
  how to use it."* The "how" lives in the docs-site reference and the flag help, consistent with how
  CLAUDE.md treats every other client surface.

### Claude's Discretion

- Exact footer wording and how `scopes_truncated` is phrased in text mode.
- Whether the coverage footer and `list`'s existing `total:` footer render as one line or two.
- Where the D-03 parity test physically lives (`cmd/engram` vs. a shared test), provided it compiles
  against both the client guard and `effectiveSearchScope`.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### The gap and why it exists

- `.planning/v0.12.x-MILESTONE-AUDIT.md` — the cross-phase integration check that found this gap;
  §"Gap — the CLI never reaches cross-spine" carries the independent verification and the structural
  lesson about consumer inventory. Also enumerates 5 confirmed-integrated seams (no defect) so this
  phase does not re-audit them.

### Prior decisions this phase is bound by

- `.planning/phases/02-headless-cli-client/02-CONTEXT.md` — D-08 (JSON mirrors Connect response field
  names), D-09/D-17 (exit-code taxonomy; exit 2 is the client's own validation), D-10 (one shared
  mapper, never per-command), D-07 (data to stdout, diagnostics to stderr), D-15 (bare invocation
  returns the self-describe catalog).
- `.planning/phases/03-cross-spine-memory-recall/03-CONTEXT.md` — D-02 (cross_spine with a non-empty
  scope ignores the scope), D-04 (Connect never infers cross-spine from an empty scope; the explicit
  field is required), D-11 (per-result scope attribution needs no new field), D-13 (ListScopes is
  called only on the cross-spine path), D-14 (flat additive response, no coverage sub-message).

### Server contract being consumed

- `proto/engram/v1/engram.proto` — `cross_spine` on `SearchMemoriesRequest` (field 9) and
  `ListMemoriesRequest` (field 12); `searched_scopes` / `scopes_truncated` on both responses.
- `internal/server/connectapi.go` — `SearchMemories` / `ListMemories` handlers; the D-04 asymmetry
  comment block explicitly warns against making the three handlers agree.
- `internal/server/tools.go:1374-1382` — `effectiveSearchScope`, the rule the client guard mirrors
  and the D-03 parity test pins against.
- `internal/server/tools.go:1384-1404` — `searchedScopes`, documenting why the reported set is the
  authorized span rather than the set of scopes that produced hits.

### Files this phase edits

- `cmd/engram/client_search.go` — flag set at :70-81, request literal at :46-55.
- `cmd/engram/client_list.go` — flag set at :82-96, request literal at :44-56, existing footer at
  :70-75 (the D-05 pattern).
- `cmd/engram/client_common.go` — `renderJSON` :263-274, `renderMemoryTable` :279-307; new shared
  guard helper lands here per D-02.
- `cmd/engram/client_common_test.go:29-53` — the exit-code anti-drift gate, precedent for D-03.

### Standing conventions

- `CLAUDE.md` — Memory contract (the cross_spine paragraph D-09 amends); Session Completion gates.
- engram memory `yaj7dqz9qq` — "a new tool argument with no guidance is an incomplete feature." The
  audit names this as the convention Phase 3 honored for docs but not for the CLI.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- `renderJSON` (`client_common.go:263`) — already emits `searched_scopes` / `scopes_truncated` on
  every response because of `EmitDefaultValues: true`. **No JSON-lane work may be needed at all**;
  verify before planning a task for it.
- `renderMemoryTable` (`client_common.go:279`) — already prints a per-result `SCOPE` column, which is
  Phase 3's D-11 per-result attribution. Cross-spine attribution in text mode is therefore already
  satisfied; only the coverage footer is missing.
- `engram list`'s footer block (`client_list.go:70-75`) — the exact stdout-footer pattern D-05 reuses,
  including the conditional-on-presence shape.
- `usageErrorf` — the existing exit-2 path used by `searchCmd`'s empty-`--query` check
  (`client_search.go:35-37`); D-01 and D-04 both route through it.

### Established Patterns

- **Never infer cross-spine from an empty scope.** Phase 3 D-04 is enforced server-side and pinned by
  `TestConnectCrossSpineNotInferred`; `connectapi.go` carries an explicit "do not fix this
  inconsistency by making the three handlers agree" warning. D-01 keeps the client on the same side of
  that line.
- **One shared helper over per-command duplication** (Phase 2 D-10) — governs D-02.
- **Anti-drift gates over convention** — `client_common_test.go:29-53` pins the exit-code table with a
  count assertion. D-03 applies the same technique to the scope guard.
- **Explicit pathspec on every commit** — `git.branching_strategy` is `none`; rule `n6m4as49mr` and
  memory `r3bjakymtz` make this mandatory whenever plans may share a working directory.

### Integration Points

- Two request literals gain one field each; two `RunE` bodies gain one guard call each; one renderer
  gains a conditional footer. No new packages, no new dependencies — the milestone's zero-new-Go-deps
  constraint is trivially preserved.
- The self-describe catalog (`02-03`) derives from the live command tree and may absorb the new flag
  automatically — the phase's one genuine unknown (D-07).

</code_context>

<specifics>
## Specific Ideas

- Help text should make the `--scope` / `--cross-spine` relationship legible from **either** flag.
  Sketched during discussion and endorsed in principle:
  - `--scope` — "limit recall to one scope; omit and pass `--cross-spine` to span every scope you can
    read"
  - `--cross-spine` — "span every scope you can read; mutually exclusive with `--scope`"
  Exact wording is the planner's, but each flag naming the other is the requirement.

- The D-15 self-describe catalog should carry the same guidance strings as `--help`, not a thinner
  version — an agent's discovery path must not be strictly worse than a human's.

</specifics>

<deferred>
## Deferred Ideas

- **Surface-wide interface audit** → captured as backlog **Phase 999.2** (ROADMAP.md, committed
  `dfde73ca`): review the entire CLI and MCP surface under D-00, covering `cmd/engram/*` help
  strings, the self-describe catalog, and MCP tool descriptions / argument docs in `internal/server`.
  Raised by Sean during this discussion; deliberately scoped out of Phase 7 so this phase stays a
  seam-closing change rather than a sweep.

- **STATE.md milestone-count drift** — `progress.total_phases` still reads 6 with `percent: 100`,
  written before Phase 7 was added by the audit. Not a Phase 7 deliverable; belongs to whoever runs
  the milestone lifecycle. Flagged so it is not mistaken for a Phase 7 regression.

- **`.planning/v0.12.x-MILESTONE-AUDIT.md` was untracked** at the time of this discussion — the
  document that justified Phase 7's existence. Worth committing alongside this phase's work.

</deferred>

---

*Phase: 7-CLI Cross-Spine Wiring*
*Context gathered: 2026-08-02*
