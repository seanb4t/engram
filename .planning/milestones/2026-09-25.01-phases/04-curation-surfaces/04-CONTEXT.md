# Phase 4: Curation Surfaces - Context

**Gathered:** 2026-09-27
**Status:** Ready for planning

<domain>
## Phase Boundary

The console becomes a curation workbench on top of the Phase 3 RPCs:

- Supersede one or more records through a preview-before-commit dialog, with the history chain
  visible from the successor and from each predecessor (CUR-01).
- Archive and restore owned records from the row, the detail pane, or a multi-select, with an
  undo (CUR-02).
- A Rules view (CUR-03) and a Scheduled view (CUR-04).
- Every new write surface participates in the re-auth resume envelope (CUR-05).
- A WCAG 2.2 keyboard and contrast audit plus a Web Interface Guidelines review (DSYS-03).
- vitest-browser coverage with screenshots for every new surface, and a chromedp e2e covering
  entry-point resolution, a supersede, and an archive/restore round trip (DSYS-04).

The phase also removes `/observe`, which is redundant after Phase 2 (D-14).

Out of scope: the related-memories graph and tag cloud (Phase 5), NL query understanding
(Phase 6), bulk curation from a saved search (CUR-06, v2), and client-tier CLI verbs (#630).

</domain>

<decisions>
## Implementation Decisions

### Keys, multi-select, and row actions
- **D-01:** New row keys: **`⇧S` supersede**, **`a` archive**, **`⇧A` restore**. Supersede gets
  a modifier because it has no undo. No shipped key changes (`e` edit, `s` share, `#` delete,
  `c`/`⇧C` copy). The `isTypingTarget` and modifier guards apply as they do today, so `⇧` is
  added to the allowed modifiers only for these keys. Every new key appears as a `<kbd>` hint
  in the listbox legend, and the `engram-console-conventions` keyboard table is updated.
- **D-02:** Multi-select follows Linear:
  - `x` toggles the active row.
  - `⇧x` and `⇧click` select a range from the anchor.
  - A checkbox column stays faint until hover or until a selection exists.
  - The listbox becomes `aria-multiselectable="true"`, with `aria-selected` on each option.
    `aria-activedescendant` still tracks the keyboard-active row, which is separate from the
    selection.
- **D-03:** **The selection wins.** With one or more rows selected, `⇧S`/`a`/`⇧A` act on the
  whole selection. The results header becomes the violet bulk bar from sketch 003:
  `N selected · Supersede N into one… ⇧S · Archive a · Restore ⇧A · clear`. `Esc` clears the
  selection before it closes any other layer. With no selection, the keys act on the active row.
- **D-04:** The selection **clears on a query or facet change**. It persists across Show more
  (a bigger `k`) and across in-place updates after a write. It is not stored in the URL.
- **D-05:** The ROADMAP's "row menu" means the **hover/focus action buttons** from sketch 003:
  Supersede, Archive or Restore, and Chain for chain members, revealed over a gradient fade on
  hover or keyboard focus. There is no `⋯` menu. The pane's inline buttons (Phase 2 D-15) become
  live: the D-16 disabled "Arrives with curation" state is removed.
- **D-06:** History has two surfaces. The pane's State section links forward to the successor
  and back to each predecessor (CUR-01). A **Chain dialog** (sketch 003) opens from a row's
  Chain button or from those pane links. It shows oldest-left → head-right columns, a
  fetch-by-id peek on every node, and a "Supersede head…" footer when the head can be
  superseded.

### Supersede dialog (sketch 003 A, carried as designed)
- **D-07:** The dialog follows `references/curation.md`:
  - a large modal with two columns: target chips with inline per-target issues on the left, the
    correcting record prefilled from the newest predecessor on the right;
  - a live chain preview below;
  - a gated primary button `Supersede N → 1`, with `⌘↵` to submit;
  - status blocks for loading, validation, server rejection, and re-auth;
  - a success body that says "No undo".

  The preview comes from **`SupersedeMemory{validate_only:true}`** (Phase 3 D-08), so the preview
  cannot disagree with the commit. The client computes per-target issues for instant feedback
  and trusts the server's answer. **One `idempotency_key` per draft**, reused on retry.

### Archive / restore ceremony
- **D-08:** Archive and Restore **both** use the small confirm dialog from sketch 003. It shows
  one chip per record and blocks records the caller does not own with `not-owned`. Restore uses
  the same dialog with a primary Restore button. No path skips the confirm, not even for a
  single owned row.
- **D-09:** Undo is offered **twice**:
  1. The dialog's result body shows `✓ N archived` with **"Undo — restore N"**, which runs the
     inverse call through the same dialog. This stays the durable record of what changed.
  2. On Done, a **svelte-sonner toast** `N archived · Undo` (about 8s) gives one-click undo
     after the dialog closes. This meets CUR-02's "one-click undo toast" as written.

  Per-id outcomes (`archived` / `already_archived` / `not_found` …, Phase 3 D-06/D-07) appear in
  the result body. Idempotent cases are reported as information, not as errors. The copy never
  distinguishes not-found from not-owned.
- **D-10:** After a successful archive or restore, the affected rows **stay in place**. They are
  patched locally from the response, show the new state word with the dim style, and flash
  violet-soft (about 1.6s). They leave at the next query. Recall queries are invalidated in the
  background, but the visible list does not jump. A successful supersede does the same: the
  predecessors stay dimmed with `superseded`, and the new record is flashed.

### Rules and Scheduled views
- **D-11:** Two new top-level routes, **`/rules`** and **`/scheduled`**, sit in the app-shell nav
  and the ⌘K menu. Each reuses the shared `ResultsList` and `DetailPane`, has its own URL state,
  and is a resume destination.
- **D-12:** **`/rules`**:
  - Default = every readable `rule:*` scope (Phase 3 D-10, empty `scopes`), grouped under a scope
    header.
  - Each row is one line: summary and tags, plus a fixed `shared` chip.
  - Full text loads on demand in the pane via `GetMemory`.
  - Coverage is reported the way cross-spine recall reports it.
  - The only actions are **Delete** (owner only, through the existing confirm) and copying ids:
    no edit, no visibility toggle, no archive button, no supersede.
- **D-13:** **`/scheduled`**:
  - State tabs: `scheduled` (default) | `expired` | `all`, with `cross_spine` on and cursor-driven
    infinite scroll (Phase 3 D-11).
  - Each row shows `not_before → not_after`, a relative phrase ("reveals in 3d" / "expired 2d
    ago"), and the state word.
  - **Archive, single or bulk, is offered only on expired rows** (CUR-04). An archived row leaves
    the view at the next query because archive soft-hides it.
  - No supersede. Correct a hidden record from `/search` with the hidden-state facets on.
- **D-14:** **Remove `/observe` entirely, with no redirect** (Sean's call). Phase 2 made it
  redundant: `/search` with operator-only input is the same unranked `ListMemories` listing
  (Phase 2 D-09), it hosts the same create/edit surfaces, and `ScopeCombobox` shows per-scope
  counts (ROW-06).
  - Delete the route, `ScopesSidebar`, and page-number pagination.
  - Remove the nav and ⌘K items.
  - Point internal links (home scope tiles, `DetailPane`, `HeaderSearch`, discovery) at
    `/search?q=scope:<x>`.
  - Drop `'/observe'` from the resume `ALLOWED_DESTINATIONS`. A stale envelope falls back to
    `/` through the existing rejection path.
  - Update `internal/e2e` and `internal/webauth/static_test.go` (which uses `/observe` only as a
    sample client route).
  - An old bookmark gets the SPA's not-found page. Historical `docs/superpowers/**` specs are
    left as written.

  — **Reversibility:** costly — restoring the route means rebuilding its page, sidebar and
  tests, though nothing on the server depends on it.

### Re-auth resume
- **D-15:** After the OIDC round trip, a curation draft **reopens and waits for confirmation**.
  Nothing is resent automatically.
  - The dialog comes back with targets, correcting-record fields, the archive/restore id set,
    and the same `idempotency_key`, plus a note "Signed in again — review and resend".
  - Supersede re-runs its `validate_only` preview on reopen, so the chain reflects any change
    made in the meantime.
  - This replaces the sketch's "Re-authenticate & retry" auto-resubmit.
- **D-16:** The envelope gains **one kind per dialog**:
  - `supersede` (targets + fields + `idempotency_key`);
  - `archive` (mode `archive|restore` + ids), which the Scheduled view reuses with
    `returnPath=/scheduled`;
  - `delete` for the Rules view's delete confirm.

  New destinations: `/rules`, `/scheduled`. `RESUME_VERSION` goes to 2. A v1 envelope still
  restores memory/discovery forms. The route host stays the only owner of
  `peekResume`/`consumeResume`, and a dialog calls only `persistResume`. Each kind gets a
  resume round-trip test (CUR-05, success criterion 4).

### Audit and test policy
- **D-17:** DSYS-03: fix every **WCAG 2.2 AA** failure in this phase, including contrast on
  dimmed past-state rows and struck-through summaries. File every other finding (AAA, Web
  Interface Guidelines polish) as a GitHub issue, and record it in the phase verification.
- **D-18:** Testing follows the standing rules `m45p2b4bp7`, `3p0zsqrhmb` and preference
  `x0krpn67b0`: behaviour of code we own, no manual UAT. Anything code can exercise is a
  vitest-browser or chromedp test. The chromedp e2e (`internal/e2e/console_browser_test.go`)
  drives entry-point resolution, a supersede, and an archive → undo round trip against a live
  server and Qdrant.

### Carried forward (decided earlier, do not re-ask)
- Sketch winners `6akjphx3k7`: modal curation dialog (003 A), fixed rows, 250ms hover card,
  second click closes the pane, state chips in canonical order along the bottom.
- Phase 3 wire contracts (`eneecjyzxh`, `vkt0aam9xk`): `ArchiveResult{requested,id,outcome}` per
  id in input order (≤1000 ids), `SupersedeMemoryResponse{id,short_id,validated,supersedes,
  targets}`, ListRules empty scopes = one cross-scope read capped at 1000 total, ListScheduled
  cursor.
- Supersede semantics: `supersedes` is a set, and an invalid member rejects the whole call. Rules
  cannot be superseded. The single-live-head rule applies per target. Not-owned, nonexistent and
  ambiguous targets are indistinguishable.
- Every mutation goes through `engramWrite` (CSRF). Authz lives only in `internal/store`. There
  are no new Go dependencies, and the milestone allows two small UI libraries (one is already
  used by the Phase 2 virtual list).
- "The entry point must not lie" (`st74vdk0gh`). Rejections render as
  `field=<f> hint=<code>: <text>`.

### Claude's Discretion
- Dialog composition: whether the supersede, archive and chain dialogs share one host component
  or live on each route. The resume host rule (D-16) must hold either way.
- The exact toast duration, the flash timing (within the sketch's 1.6s), and bulk-bar copy.
- Whether the results header's "N hidden by recall gate" count becomes a one-click toggle for
  the hidden-state facets (sketch's `h` "show hidden"). If it gets a key, it must not collide
  with D-01.
- `/rules` and `/scheduled` empty states, following the honest-feedback rule.
- Where the add-target-by-short_id input in the supersede dialog gets its lookup (`GetMemory`
  per the shared classifier).

### Folded Todos
- **Security-review then install the three design/a11y registry skills**
  (`.planning/todos/pending/2026-09-25-security-review-and-install-design-skills.md`):
  `pbakaus/impeccable`, `vercel-labs/agent-skills@web-design-guidelines`,
  `addyosmani/web-quality-skills@accessibility`.
  - Run the `fable-security-review` skill on each, with the intended use "design guidance and
    review skill loaded into coding-agent sessions on this repo".
  - Install those that pass via `npx skills add` into `~/.agents/skills`, and keep the chezmoi
    restore selection current.
  - Record the verdicts in `.planning/notes/console-overhaul-exploration.md`.
  - DSYS-03 uses the skills that pass. If a skill fails, the audit falls back to the WCAG 2.2
    checklist, axe-core in vitest-browser, and the Web Interface Guidelines read by hand. The
    failed verdict is recorded; nothing is installed around it.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope
- `.planning/REQUIREMENTS.md`: CUR-01..05, DSYS-03, DSYS-04; out-of-scope table (no rule edit,
  no `delete_all`, CUR-06 is v2)
- `.planning/ROADMAP.md`: Phase 4 goal and success criteria 1–5
- `.planning/notes/console-overhaul-exploration.md`: milestone decisions; the design-skill list
  (and where the D-folded verdicts go)
- `.planning/research/PITFALLS.md`, `.planning/research/FEATURES.md`: CSRF/resume pitfalls,
  curation feature survey

### Design direction (locked by Phase 01.1 sketches)
- `.claude/skills/sketch-findings-engram/references/curation.md`: supersede dialog, archive
  confirm, chain dialog, row actions, bulk bar, what to avoid (**primary design source**)
- `.claude/skills/sketch-findings-engram/references/foundations.md`: tokens, state chips,
  timings, keys
- `.claude/skills/sketch-findings-engram/sources/003-curation-dialogs/index.html`: runnable
  winner A
- `.claude/skills/engram-console-conventions/SKILL.md`: keyboard model (extend with D-01/D-02),
  state words, honest feedback
- `.claude/skills/engram-connect-client/SKILL.md`: `engram` vs `engramWrite`, CSRF, resume
  envelope rules, per-RPC table for the Phase 3 curation RPCs

### Prior phase decisions
- `.planning/phases/03-curation-rpcs-mcp-tools/03-CONTEXT.md`: D-06..D-14 wire contracts
  (archive batch outcomes, `validate_only`, ListRules/ListScheduled widening)
- `.planning/phases/02-recall-first-search/02-CONTEXT.md`: D-07 listbox/virtual list, D-09
  unranked listing, D-11 ⌘K vs `/`, D-12 `/observe`, D-14..D-17 pane and row keys
- `.planning/phases/01-store-prerequisites/01-CONTEXT.md`: archive authz (not-owned → NotFound)

### API contract
- `proto/engram/v1/engram.proto`: `SupersedeMemory`, `ArchiveMemory`, `RestoreMemory`,
  `ListRules`, `ListScheduled`
- `ui/src/lib/gen/`: generated TS client
- `CLAUDE.md` §Memory contract: supersession, archived state, scheduled, rules semantics

### Folded todo
- `.planning/todos/pending/2026-09-25-security-review-and-install-design-skills.md`

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `ui/src/lib/components/ResultsList.svelte`: the listbox, keyboard model and `isTypingTarget`
  guard. Multi-select and the new keys land here.
- `ResultRow.svelte`, `ResultHoverCard.svelte`, `DetailPane.svelte`, `ResultsHeader.svelte`:
  the row actions, the pane links and buttons, and the bulk bar (header swap).
- `DeleteConfirmDialog.svelte`: the pattern for the small confirm dialog and the Rules delete.
- `WriteSurfaces.svelte`: the pattern for a route-hosted write surface with `onresumeapplied`.
- `ui/src/lib/resume.ts`: `persistResume`/`peekResume`/`consumeResume`,
  `ALLOWED_DESTINATIONS`, `RESUME_VERSION` (D-14, D-16).
- `ui/src/lib/mutations/memory.ts`: the TanStack `createMutation` pattern. Add curation
  mutations alongside it.
- `ui/src/lib/memorystate.ts`: `STATE_WORD_ORDER`, `isPastState` for the dim rule.
- `ui/src/lib/errors.ts`: `describeError` and envelope parsing into pills.
- `ScopeCombobox.svelte`: the scope entry that replaces `ScopesSidebar` (D-14).

### Established Patterns
- svelte-query v6 option functions. `placeholderData: keepPreviousData`. A per-query
  `AbortSignal`.
- A route host owns resume consumption, and forms and dialogs only persist.
- vitest-browser tests with screenshots beside each component (`__screenshots__/`).
- The bits-ui gotchas (`3tz15e733n`): `Command` needs `shouldFilter={false}`, and portalled
  popovers lose keyboard navigation. This matters for the add-target combobox inside a Dialog.
- Grid drop-outs must swap `--cols` (`xx98my50ng`). This matters when the checkbox column is
  added.

### Integration Points
- `AppShell.svelte` nav and `CommandMenu.svelte` items: add Rules/Scheduled, remove Observe,
  and add row actions to ⌘K (Phase 2 D-11 reserved them).
- `ui/src/routes/+page.svelte` (landing and resume router): add the new destinations and
  retarget the scope tiles.
- `internal/e2e/console_browser_test.go` (`TestConsoleBundleRendersRecordInBrowser` and its
  harness): extend with the supersede and archive/undo flows.
- The vendored SPA bundle and the `ui-drift` gate: rebuild and commit after UI changes.

</code_context>

<specifics>
## Specific Ideas

- Ceremony matches risk. Supersede (no undo) gets the big dialog and a modifier key. Archive
  (reversible) gets a small confirm plus two undos.
- Sean questioned why `/observe` still exists, and chose deletion over a redirect. Prefer
  removing a redundant surface to keeping a compatibility shim for it.

</specifics>

<deferred>
## Deferred Ideas

- Scope browsing as its own surface (the always-visible scope list that `/observe` had) could
  come back, if wanted, with Phase 5's browse work. It is not planned.
- A key or toggle for "show hidden" beyond the Phase 2 facets is at Claude's discretion (above).
  A dedicated saved-search bulk flow stays CUR-06 (v2).

</deferred>

---

*Phase: 04-curation-surfaces*
*Context gathered: 2026-09-27*
