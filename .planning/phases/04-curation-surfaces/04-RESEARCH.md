# Phase 4: Curation Surfaces - Research

**Researched:** 2026-09-27
**Domain:** Svelte 5 / SvelteKit console curation UI (supersede, archive/restore, rules, scheduled) over already-shipped Connect RPCs
**Confidence:** HIGH

## Summary

Phase 4 adds zero new dependencies and zero new backend surface — every RPC it calls
(`SupersedeMemory`, `ArchiveMemory`, `RestoreMemory`, `ListRules`, `ListScheduled`) shipped in
Phase 3 and is already in `csrfWriteProcedures`
[VERIFIED: internal/server/connectcsrf.go:36-46] `var csrfWriteProcedures = map[string]bool{... EngramServiceArchiveMemoryProcedure: true, EngramServiceRestoreMemoryProcedure: true, EngramServiceSupersedeMemoryProcedure: true,}`.
The work is entirely `ui/`: three new dialogs (Supersede, Archive/Restore, Chain), a checkbox
multi-select added to the existing virtualized listbox, a bulk-bar header swap, two new routes
(`/rules`, `/scheduled`), a resume-envelope schema widening, and the deletion of `/observe`. Every
shadcn-svelte primitive the UI-SPEC calls for (`dialog`, `popover`, `command`, `tabs`, `checkbox`,
`badge`, `tooltip`, `select`, `scroll-area`, `item`, `sonner`) is already installed under
`ui/src/lib/components/ui/` [VERIFIED: ui/src/lib/components/ui/ directory listing, this session] —
confirmed present: `checkbox tabs item sonner dialog popover command badge button tooltip select
input textarea separator scroll-area`. No `shadcn-svelte add` runs this phase.

The highest-risk work is not new UI chrome, it is **retrofitting three already-shipped, narrowly-typed
modules to a wider contract**: `ui/src/lib/resume.ts`'s `ResumeEnvelope` is a flat interface hard-typed
to `kind: 'memory' | 'discovery'` and must grow a real discriminated union without breaking the
still-valid v1 shape; `ui/src/lib/components/ResultsList.svelte`'s keyboard model and
`aria-selected` usage currently conflate "active row" with "selected row" and must be split;
`ui/src/lib/mutations/memory.ts`'s optimistic-cache-patch pattern (`applyToMemoryCaches`) removes
rows on delete but must instead **patch-in-place** for archive/restore/supersede (D-10); and the
sketch's `.row.flash`/`@keyframes flash` CSS the UI-SPEC says to "reuse ... do not re-tune" **does
not exist yet** in shipped code — it must be added this phase, not reused.

**Primary recommendation:** build the three curation dialogs and the `CurationSurfaces.svelte` host
exactly as the UI-SPEC's Component Composition section prescribes (mirroring `WriteSurfaces.svelte`'s
exported-method contract), extend the three existing modules named above in place rather than
forking them, and budget a dedicated task for the `ResumeEnvelope` discriminated-union rewrite since
it is the one piece of this phase with no existing precedent to copy.

## User Constraints

<user_constraints>
### Locked Decisions (verbatim from 04-CONTEXT.md `## Implementation Decisions`)

**Keys, multi-select, and row actions**
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

**Supersede dialog (sketch 003 A, carried as designed)**
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

**Archive / restore ceremony**
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

**Rules and Scheduled views**
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

**Re-auth resume**
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

**Audit and test policy**
- **D-17:** DSYS-03: fix every **WCAG 2.2 AA** failure in this phase, including contrast on
  dimmed past-state rows and struck-through summaries. File every other finding (AAA, Web
  Interface Guidelines polish) as a GitHub issue, and record it in the phase verification.
- **D-18:** Testing follows the standing rules `m45p2b4bp7`, `3p0zsqrhmb` and preference
  `x0krpn67b0`: behaviour of code we own, no manual UAT. Anything code can exercise is a
  vitest-browser or chromedp test. The chromedp e2e (`internal/e2e/console_browser_test.go`)
  drives entry-point resolution, a supersede, and an archive → undo round trip against a live
  server and Qdrant.

**Carried forward (decided earlier, do not re-ask)**
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

### Claude's Discretion (verbatim)
- Dialog composition: whether the supersede, archive and chain dialogs share one host component
  or live on each route. The resume host rule (D-16) must hold either way.
- The exact toast duration, the flash timing (within the sketch's 1.6s), and bulk-bar copy.
- Whether the results header's "N hidden by recall gate" count becomes a one-click toggle for
  the hidden-state facets (sketch's `h` "show hidden"). If it gets a key, it must not collide
  with D-01.
- `/rules` and `/scheduled` empty states, following the honest-feedback rule.
- Where the add-target-by-short_id input in the supersede dialog gets its lookup (`GetMemory`
  per the shared classifier).

### Deferred Ideas (OUT OF SCOPE, verbatim)
- Scope browsing as its own surface (the always-visible scope list that `/observe` had) could
  come back, if wanted, with Phase 5's browse work. It is not planned.
- A key or toggle for "show hidden" beyond the Phase 2 facets is at Claude's discretion (above).
  A dedicated saved-search bulk flow stays CUR-06 (v2).

### UI-SPEC resolution of the Component Composition discretion item (04-UI-SPEC.md)

The UI-SPEC (mode `--auto`) resolves the "dialog composition" discretion item above: mirror
`WriteSurfaces.svelte`'s existing architecture exactly. A new sibling component,
`CurationSurfaces.svelte`, is instantiated once **per route** that needs it (`/search`, `/rules`,
`/scheduled`), exposing an explicit `bind:this` method contract (`openSupersede(ids)`,
`openArchive(ids)`, `openRestore(ids)`, `openChain(id)`) the same way `WriteSurfaces` exposes
`openCreate()`/`openEdit(id)`/`requestDelete(id, kind)`. Each instance owns its own dialog `open`
state, its own `idempotency_key` per draft, and its own `returnPath` for the resume envelope. The
Rules view's delete confirm reuses `DeleteConfirmDialog.svelte` directly (a third `kind: 'rule'`
copy entry) rather than routing through `CurationSurfaces`.

The four `04-CONTEXT.md`-over-`curation.md` deltas from 04-UI-SPEC.md (executor must apply the
override, not the sketch's literal text):

| Sketch says | `04-CONTEXT.md` says instead |
|---|---|
| Re-auth: "Re-authenticate & retry" auto-resubmits | **D-15**: dialog reopens and waits; manual Resend, not auto-retry |
| Bulk bar keys `s`/`e`/`u` | **D-01**: `⇧S`/`a`/`⇧A` — `s`/`e` are already Share/Edit |
| Row hover reveals a `⋯`-adjacent action set generically | **D-05**: exactly Supersede, Archive-or-Restore, Chain — no `⋯` menu |
| Archive/Restore ships with a single undo | **D-09**: undo offered twice (result-body + toast) |
</user_constraints>

## Phase Requirements

<phase_requirements>
| ID | Description | Research Support |
|----|-------------|------------------|
| CUR-01 | Supersede one or more records via preview-before-commit dialog; pane links forward/back | `SupersedeMemory{validate_only}` wire shape verified (proto); `DetailPane.svelte`'s existing (currently disabled) Supersede button and State-section predecessor/successor links verified at exact lines; Chain dialog pattern from `curation.md` |
| CUR-02 | Archive/restore owned records from row/pane/multi-select; state word updates in place; one-click undo toast | `ArchiveResult`/`ArchiveOutcome` wire shapes verified (proto); `applyToMemoryCaches` in-place-patch pattern identified as the extension point; svelte-sonner `duration`/`action` option verified in installed package types |
| CUR-03 | Rules view: one-line index, full text on demand, no visibility toggle, delete-only | `ListRules` wire shape verified (proto); `DeleteConfirmDialog.svelte`'s kind-copy table identified as the extension point for a third `kind: 'rule'` |
| CUR-04 | Scheduled view: scheduled/expired/all tabs, archive only on expired | `ListScheduled` wire shape verified (proto); `Tabs` component confirmed installed and unused since Phase 2 |
| CUR-05 | Every new write surface survives OIDC re-login via resume envelope | `resume.ts`'s current flat, memory/discovery-only `ResumeEnvelope` type read in full — this is the phase's highest architectural-risk item, detailed in Pitfalls and Code Examples below |
| DSYS-03 | WCAG 2.2 keyboard/contrast audit + Web Interface Guidelines review | Folded-todo skill-install gate (`fable-security-review` on three third-party skills) documented; fallback path (manual WCAG checklist + axe-core) documented |
| DSYS-04 | vitest-browser coverage + chromedp e2e (entry-point resolution, supersede, archive/restore round trip) | Existing `internal/e2e/console_browser_test.go` structure and `/observe`-as-sample-route usage confirmed at the two exact call sites that must change |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- VCS: branch + PR, never push to `main`; Conventional Commits required, PR titles CI-validated.
- `task` = lint + test — but note `ui/`'s tests are **not** wired into the Go `task test` target
  (verified: `Taskfile.yaml` has no `ui:test`/`pnpm test` step under `test`/`test:go`/`test:python`;
  `ui/`'s `pnpm test` runs only in `.github/workflows/ci.yaml`'s dedicated job, alongside a
  separate `ui-drift` job that rebuilds `ui/` and diffs `internal/webauth/static`). The phase's own
  verification must run `pnpm test` (and `pnpm test:browser` specifically for the new dialogs)
  directly under `ui/`, not assume `task` covers it.
  [VERIFIED: Taskfile.yaml (task:test/test:go/test:python blocks) + .github/workflows/ci.yaml:302-433, this session]
- License headers: `.planning/**`, `skill/**/SKILL.md`, and `docs-site/**` are excluded from the
  SPDX gate; `ui/src/**` is Go-license-gate-irrelevant (TS/Svelte, not in scope of `.licenserc.yaml`'s
  Go/Markdown gate) but new **Go** files this phase touches (`internal/e2e/console_browser_test.go`,
  `internal/webauth/static_test.go`) still need the header if missing.
- Migrations/schema-version: not applicable — this phase touches no stored-record shape, only
  console UI over existing RPCs.
- `internal/store` authz: unchanged this phase (STORE-01 shipped in Phase 1); no new authz surface.
- No new Go dependencies; **at most two small UI libraries** for the whole milestone, one already
  spent (`@humanspeak/svelte-virtual-list`, Phase 2) — Phase 4 spends **zero** more (verified: every
  UI-SPEC component is already-installed shadcn-svelte).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Supersede/archive/restore business rules (single-live-head, ownership, rule-immutability) | API / Backend (`internal/store`, Phase 1/3, already shipped) | — | Authz and invariants are enforced server-side only (`DEC-cgb`); the console never re-derives them, only renders the server's answer |
| Supersede preview (chain, per-target validity) | API / Backend (`SupersedeMemory{validate_only:true}`) | Browser (instant client-side per-target hints) | D-07: the server's `validate_only` answer is authoritative; client-side checks are UX-only fast feedback that can disagree with the server on race conditions, never the other way |
| Curation dialogs, bulk selection, row-in-place patching | Browser / Client (`ui/src/lib/components/*.svelte`) | — | Pure SPA state; SvelteKit here is static-adapter-only (no SSR tier in this project) |
| Resume envelope persistence across re-auth | Browser / Client (`sessionStorage` via `resume.ts`) | Frontend Server (OIDC redirect via `internal/webauth`) | The redirect round-trip is server-driven (`/auth/login` → IdP → `/auth/callback`), but envelope storage and restoration are client-only; the server has no resume-envelope awareness |
| CSRF double-submit on writes | Browser / Client (`attachCsrf` interceptor) + API / Backend (`internal/server/connectcsrf.go` verifier) | — | Client echoes, server verifies; the client never validates its own token (already-shipped, unchanged this phase) |
| Rules/Scheduled listing, coverage reporting | API / Backend (`ListRules`/`ListScheduled`, already shipped Phase 3) | Browser (URL-state codec, tab state) | Coverage triple (`searchedScopes`/`scopesTruncated`/`scopesUnknown`) is server-computed and rendered verbatim, never inferred client-side (honest-feedback rule) |
| WCAG/a11y audit tooling | Browser / Client (axe-core in vitest-browser, if the third-party skills fail `fable-security-review`) | — | Pure dev-time tooling, no runtime surface |

## Standard Stack

### Core — all already installed, zero new installs

| Library | Version (verified installed) | Purpose | Why no change needed |
|---------|---------|---------|--------------|
| svelte | `5.57.1` [VERIFIED: ui/package.json] | Component runtime | Matches UI-SPEC's "Svelte 5" |
| `@tanstack/svelte-query` | `^6.1.34` [VERIFIED: ui/package.json] | `createMutation`/`createQuery` for every curation RPC | Matches milestone's "TanStack Query 6" stack decision |
| `bits-ui` | `^2.18.1` [VERIFIED: ui/package.json] | Underlies every shadcn-svelte primitive (`Dialog`, `Command`, `Tabs`, `Checkbox`, …) | UI-SPEC explicitly flags: do not bump silently mid-phase |
| `svelte-sonner` | `1.2.1` [VERIFIED: ui/package.json] | The undo toast (D-09 surface 2) | `ExternalToast.duration`/`.action` confirmed present [VERIFIED: ui/node_modules/svelte-sonner/dist/types.d.ts:182] |
| `@connectrpc/connect` / `connect-web` | `^2.1.1` [VERIFIED: ui/package.json] | RPC transport, `ConnectError`/`Code` | Unchanged; curation RPCs are proxy methods on the same `EngramService` client, no new client construction |
| `@humanspeak/svelte-virtual-list` | `0.5.14` [VERIFIED: ui/package.json] | `ResultsList`'s virtualized viewport | Reused for `/rules`/`/scheduled`, not re-spent as a second "UI library" |
| `@lucide/svelte` | `1.47.0` [VERIFIED: ui/package.json] | Icons | — |
| shadcn-svelte components | `1.7.0`, 27 families [VERIFIED: `ls ui/src/lib/components/ui/`, this session — includes `dialog popover command tabs checkbox badge button tooltip select input textarea sonner separator scroll-area item`] | Every dialog/tab/confirm this phase needs | Zero `shadcn-svelte add` invocations required |

**Installation:** none. No `pnpm add`, no `shadcn-svelte add` this phase.

### Alternatives Considered

Not applicable — the milestone's own stack decision (`.planning/REQUIREMENTS.md`'s header) already
fixed the stack before Phase 1, and CONTEXT.md's Carried-forward decisions reconfirm "no new Go
dependencies, ... two small UI libraries" with one already spent. No alternative-library research
is in scope for this phase.

## Package Legitimacy Audit

**Not applicable this phase.** Zero external packages are installed (Standard Stack above; UI-SPEC
Registry Safety section confirms "Phase 4 needs zero new shadcn installs"). The three third-party
*skills* named in CONTEXT.md's Folded Todos (`pbakaus/impeccable`,
`vercel-labs/agent-skills@web-design-guidelines`, `addyosmani/web-quality-skills@accessibility`) are
coding-agent tooling consumed during the DSYS-03 audit, not runtime dependencies shipped into
`ui/` — they are gated by `fable-security-review`, not this audit protocol (UI-SPEC Registry Safety
section states this explicitly and the planner must schedule the security-review task per
CONTEXT.md's Folded Todos, not duplicate a package-legitimacy check for them).

**Packages removed due to [SLOP] verdict:** none.
**Packages flagged as suspicious [SUS]:** none.

## Architecture Patterns

### System flow — a supersede action end to end

```
Row hover/focus or bulk-bar "Supersede N…"
        │  (CurationSurfaces.openSupersede(ids))
        ▼
Supersede dialog opens
        │  ids resolved from selection/row → target chips (client-side issue check for instant feedback)
        ▼
On every keystroke/target change:
  engram.supersedeMemory({ ...fields, supersedes, validateOnly: true })   [read-shaped call on a write RPC — still CSRF'd]
        │
        ▼
  Chain preview + per-target chip state re-rendered from the server's validated/targets answer
        │  (client-side hints shown instantly, but the server's validate_only answer wins on conflict)
        ▼
Operator clicks "Supersede N → 1" (⌘↵)
        │
        ▼
  engramWrite.supersedeMemory({ ...fields, supersedes, idempotencyKey, validateOnly: false })
        │
        ├─ success ──▶ patch predecessors in-place (supersededBy set, dim+flash) via a new
        │              applySupersedeOptimistic-shaped cache patch (extends memory.ts's
        │              applyToMemoryCaches), insert/flash the new head row, invalidate
        │              searchMemories/listMemories/getMemory/listScheduled/relatedMemories/
        │              listTags in the background, show success body ("No undo"), and
        │              consumeResume() if this was a resumed draft
        │
        └─ Unauthenticated/PermissionDenied ──▶ persistResume({ kind: 'supersede', targets,
                       fields, idempotencyKey, returnPath }) → redirectToLogin() → (IdP) →
                       /auth/callback → lands on /ui/ → root route peeks envelope, goto()s
                       returnPath WITHOUT consuming → destination route's CurationSurfaces
                       reopens the SAME dialog pre-filled, re-runs validate_only, shows
                       "Signed in again — review and resend", operator clicks Resend manually
```

### Recommended file additions (relative to `ui/src/`)

```
lib/components/
├── CurationSurfaces.svelte       # route-hosted write host, mirrors WriteSurfaces.svelte exactly
├── SupersedeDialog.svelte        # large dialog (D-07); owns its own validate_only query + idempotency_key
├── ArchiveConfirmDialog.svelte   # small dialog, mode='archive'|'restore' prop (D-08)
├── ChainDialog.svelte            # medium dialog, per-node fetch-by-id peek (D-06)
├── BulkActionBar.svelte          # OR: extend ResultsHeader.svelte with a `selection` prop — see Pitfall below
lib/mutations/
└── curation.ts                   # useSupersedeMemory / useArchiveMemory / useRestoreMemory hooks,
                                   #   mirroring memory.ts's createMutation shape; new
                                   #   applySupersedeOptimistic / applyArchiveOptimistic
                                   #   in-place-patch cache functions (NOT remove-on-success)
routes/
├── rules/+page.svelte            # /rules — reuses ResultsList + DetailPane
└── scheduled/+page.svelte        # /scheduled — reuses ResultsList + DetailPane + Tabs
lib/search/
└── rules-params.ts / scheduled-params.ts   # one parse fn + one encode fn per route,
                                             #   per engram-connect-client's "URL state" convention
```

### Pattern 1: Extend `applyToMemoryCaches`, do not fork it

**What:** `ui/src/lib/mutations/memory.ts`'s `applyToMemoryCaches(queryClient, id, fn)`
[VERIFIED: ui/src/lib/mutations/memory.ts:204-231] already iterates every cached
`listMemories`/`searchMemories` page plus the `getMemory` cache and applies a per-record transform,
with `fn` returning `null` to remove the record from that cache entry. `applyDeleteOptimistic`
(line 251-253) uses `fn: () => null`; `applyUpdateOptimistic`/`applySetVisibilityOptimistic` patch
fields in place and only return `null` when a *filtered* list page's membership criterion no
longer matches.

**When to use:** For archive/restore/supersede's D-10 "rows stay in place" requirement, write new
functions (`applyArchiveOptimistic`, `applyRestoreOptimistic`, `applySupersedeOptimistic`) using
the exact same `applyToMemoryCaches` iteration, but the transform must **patch, never null out**
(a soft-hidden record does not disappear from a currently-rendered list until the *next* query,
per D-10) — the opposite of the existing delete precedent. `listScheduled`'s cache also needs a
prefix added to `MEMORY_LIST_PREFIXES` (currently only `['listMemories']`/`['searchMemories']`,
[VERIFIED: ui/src/lib/mutations/memory.ts:160] `const MEMORY_LIST_PREFIXES = [['listMemories'], ['searchMemories']] as const;`)
so a Scheduled-view archive also patches the Scheduled cache in place.

**Example (sketch, following the file's own established shape):**
```typescript
// ui/src/lib/mutations/curation.ts (new file)
export function applyArchiveResultsOptimistic(
  queryClient: QueryClient,
  results: { id: string; outcome: ArchiveOutcome }[]
): void {
  for (const r of results) {
    if (r.outcome !== ArchiveOutcome.ARCHIVED && r.outcome !== ArchiveOutcome.RESTORED) continue;
    applyToMemoryCaches(queryClient, r.id, (m) => ({
      ...m,
      archivedAt: r.outcome === ArchiveOutcome.ARCHIVED ? create(TimestampSchema, timestampFromDate(new Date())) : undefined
    }));
  }
}
```
(`applyToMemoryCaches` itself is not exported today [VERIFIED: ui/src/lib/mutations/memory.ts:204]
`function applyToMemoryCaches(` — the planner must export it, or move the new curation cache
functions into `memory.ts` itself rather than a separate `curation.ts`, to reuse it without
duplicating the cache-iteration logic.)

### Pattern 2: Keyboard model extension is a narrow, well-understood diff

**What:** `ResultsList.svelte`'s keydown guard is `if (event.metaKey || event.ctrlKey ||
event.altKey) return;` [VERIFIED: ui/src/lib/components/ResultsList.svelte:387] — **Shift is not
checked**, so a `⇧S`/`⇧A`/`⇧x`/`⇧click` combo already passes this guard today (proven by the
existing shipped `⇧C` copy-id key using the same mechanism [VERIFIED: ResultsList.svelte:312-315,
479] `<Kbd>⇧C</Kbd> copy id`). `HANDLED_KEYS`/`ROW_ACTION_KEYS` are plain `Set<string>` literals
[VERIFIED: ui/src/lib/components/ResultsList.svelte:330-334]
`const ROW_ACTION_KEYS = new Set(['e', 's', '#', 'c', 'C']);` — note `'C'` (not `'⇧C'`) is the
literal `event.key` value for Shift+c, so the new keys are literally `'S'`, `'A'`, `'x'`.

**When to use:** Add `'S'` and `'A'` to `ROW_ACTION_KEYS` (so `isTypingTarget` guarding still
applies to them) and add `'x'` to a new `SELECTION_KEYS` set folded into `HANDLED_KEYS`. The
`Escape` case in `handleListboxKey` [VERIFIED: ui/src/lib/components/ResultsList.svelte:281-291]
must be reordered: selection-clear (D-03: "Esc clears the selection before it closes any other
layer") becomes the **new outermost check**, ahead of the existing `cardOpen` check. This means
`ResultsList` (or whatever component owns selection state) must know the selection size before
deciding what `Escape` does — selection state most naturally lives in `ResultsList` itself,
co-located with the existing `activeId` by-ID-tracking pattern (`$state<string | undefined>`,
[VERIFIED: ResultsList.svelte:187]), exposed via a bindable prop or `onselectionchange` callback
to the route (which needs the ids for the bulk bar and for dispatching `⇧S`/`a`/`⇧A`).

**Don't:** Reuse `aria-selected` for both "active" and "selected." Today
`aria-selected={index === activeIndex}` [VERIFIED: ui/src/lib/components/ResultsList.svelte:459]
means "is the keyboard-active row" (a valid single-select-listbox pattern). D-02 requires
`aria-selected` to mean true multi-select membership, independent of `aria-activedescendant` — this
is a **semantic change to an existing line**, not an additive one; get this wrong and the a11y
audit (DSYS-03) will fail on a WAI-ARIA APG multiselect-listbox violation.

### Pattern 3: `ResumeEnvelope` needs a real discriminated union (highest-risk item)

**What:** The shipped type is flat and hard-typed to two kinds
[VERIFIED: ui/src/lib/resume.ts:26-34]:
```typescript
export interface ResumeEnvelope {
  v: number; ts: number; returnPath: string;
  kind: 'memory' | 'discovery';
  mode: 'create' | 'edit';
  recordId: string | null;
  values: Record<string, unknown>;
}
```
`isValidShape` [VERIFIED: ui/src/lib/resume.ts:92-102] hard-checks `o.kind !== 'memory' && o.kind
!== 'discovery'` and unconditionally requires `mode`/`recordId`/`values` — none of which fit a
`supersede` draft (`targets`, `fields`, `idempotencyKey` — no single `recordId`) or an `archive`
draft (`mode: 'archive'|'restore'` collides in *name* but not *meaning* with the existing
`mode: 'create'|'edit'` field). `RESUME_VERSION = 1` [VERIFIED: ui/src/lib/resume.ts:19] and
`ALLOWED_DESTINATIONS = ['/observe', '/search', '/discovery']` [VERIFIED: ui/src/lib/resume.ts:47]
both need changes D-16/D-14 specify.

**When to use:** This is the one module in the phase with no existing multi-kind precedent to
copy — budget it as its own task with its own resume-round-trip tests per kind (D-16's explicit
requirement: "Each new kind gets its own resume round-trip test"). A sketch of the shape (the
planner should verify the exact discriminant approach against TypeScript's narrowing behavior
before committing):

```typescript
export type ResumeEnvelope =
  | { v: 2; ts: number; returnPath: string; kind: 'memory' | 'discovery'; mode: 'create' | 'edit'; recordId: string | null; values: Record<string, unknown> }
  | { v: 2; ts: number; returnPath: string; kind: 'supersede'; targets: string[]; fields: Record<string, unknown>; idempotencyKey: string }
  | { v: 2; ts: number; returnPath: string; kind: 'archive'; mode: 'archive' | 'restore'; ids: string[] }
  | { v: 2; ts: number; returnPath: string; kind: 'delete'; id: string }
  // v1 back-compat: a stored envelope with v===1 has the OLD flat shape and must still
  // round-trip through the existing memory/discovery restore path (D-16: "additive, not a
  // breaking rewrite") — isValidShape must branch on `o.v` before applying the v2 per-kind checks.
  | { v: 1; ts: number; returnPath: string; kind: 'memory' | 'discovery'; mode: 'create' | 'edit'; recordId: string | null; values: Record<string, unknown> };
```
`normalizeReturnPath`/`isAllowedDestination`/`persistResume`/`peekResume`/`consumeResume`/
`redirectToLogin` are all kind-agnostic already [VERIFIED: ui/src/lib/resume.ts:49-148] and need
no restructuring beyond `ALLOWED_DESTINATIONS` gaining `'/rules'`/`'/scheduled'` and dropping
`'/observe'`, and `isValidShape` gaining per-kind branches.

### Pattern 4: `ResultsHeader` has no bulk-bar mode today — needs a new prop or a wrapper

**What:** `ResultsHeader.svelte` renders exactly one thing: `HeaderPart[]` from
`recall-header.ts`'s honest-feedback formatters [VERIFIED: ui/src/lib/components/ResultsHeader.svelte:1-22,
full file read]. It has no selection awareness and no alternate-content branch.

**When to use:** D-03's "results header becomes the violet bulk bar" needs either (a) a new
`selection` prop on `ResultsHeader` that swaps its entire rendered body when non-empty (matching
UI-SPEC's "Header swaps content, not height, when a selection exists"), or (b) a sibling
`BulkActionBar.svelte` the route conditionally renders in `ResultsHeader`'s place. Given
`ResultsHeader`'s current single-responsibility shape (pure renderer, no state), option (a) as an
additive prop keeps one header component and matches the "swap content" framing most directly;
either is compatible with `04-CONTEXT.md`'s D-16 resume rule since neither the header nor the bar
owns resume state.

### Pattern 5: `DetailPane`'s disabled curation buttons are the literal spots to wire live

**What:** `DetailPane.svelte` already renders Supersede/Archive-or-Restore buttons, currently
`disabled` and wrapped in a `Tooltip` reading `"Arrives with curation — Phase 4"`
[VERIFIED: ui/src/lib/components/DetailPane.svelte:19, 133-164] (`const CURATION_TOOLTIP =
'Arrives with curation — Phase 4';` ... `<Button variant="outline" size="sm" disabled>Supersede…</Button>`
... `<Button variant="outline" size="sm" disabled>{memory.archivedAt ? 'Restore' : 'Archive'}</Button>`).
D-05 says this disabled state is **removed** this phase. The pane already computes `isRule`,
`isDiscovery` [VERIFIED: DetailPane.svelte:57-58] to gate Edit/Share — the same fence pattern
applies to Supersede (never for `rule`) and Archive/Restore.

**When to use:** Replace the two `Tooltip.Provider`-wrapped disabled buttons with live `onclick`
handlers calling the new `CurationSurfaces` host's `openSupersede([memory.id])` /
`openArchive([memory.id])` / `openRestore([memory.id])`, following the exact prop-callback shape
`onedit`/`onvisibility`/`ondelete` already use on this component (`onedit?: (id: string) => void`,
etc. [VERIFIED: DetailPane.svelte:36-50]) — add `onsupersede`/`onarchive`/`onrestore` callbacks in
the same style rather than importing `CurationSurfaces` directly into `DetailPane` (keeping the
pane a pure presentation component, matching its existing dependency-injection-via-props pattern).

The pane's existing State-section predecessor/successor links (CUR-01's "pane... links forward to
its successor and back to each predecessor") are **already implemented** and need no new code —
[VERIFIED: DetailPane.svelte:182-195] `{#if stateWords.includes('superseded') && memory.supersededBy}
... superseded by <button onclick={() => onselect?.(memory!.supersededBy!)}>{memory.supersededBy}</button>
... {#if memory.supersedes.length > 0} ... supersedes {#each memory.supersedes as predId (predId)}
<button onclick={() => onselect?.(predId)}>{predId}</button>{/each}`. Only the Chain dialog
entry point (a new "Chain" button/link) is genuinely new here.

### Pattern 6: the `.row.flash` animation does not exist yet — it must be authored, not reused

**What:** `curation.md`'s CSS Patterns section defines `.row.flash { animation: flash 1.6s ease; }`
and `@keyframes flash { 0%, 30% { background: var(--color-primary-soft); } 100% { background:
transparent; } }`, and 04-UI-SPEC.md's Row-in-place-update section says to "reuse it, do not
re-tune the duration" as if it already shipped. **It has not** — a repo-wide search for `flash`
across `ui/src/app.css` and every component under `ui/src/lib/components/` found zero matches
outside a code comment [VERIFIED: `rg -n "flash" ui/src/app.css ui/src/lib/components/*.svelte`,
this session, only hit was the word "flash" inside an unrelated code comment in
`ResultsList.svelte:430`, not a class or keyframe]. `--primary-soft` (the token the animation
needs) does exist [VERIFIED: ui/src/lib/components/ResultsList.svelte pattern; token confirmed
present in `engram-console-conventions` skill's shipped-tokens table].

**When to use:** Author `@keyframes flash` and a `.flash`/`.result-row-line.flash` class in
`ResultRow.svelte`'s `<style>` block (or a shared stylesheet) this phase, apply it via a
transient `class:flash` toggled for ~1.6s after a successful in-place patch (Pattern 1 above),
and remove the class on a timer or `animationend`. This is new work, not a reuse, despite the
UI-SPEC's phrasing.

### Anti-Patterns to Avoid

- **Forking `applyToMemoryCaches` into a parallel curation-only cache walker.** The function
  already handles the list/search/getMemory triad correctly (including the filtered-list-page
  membership rule); a parallel implementation will drift and double-invalidate.
- **Treating the disabled-button removal in `DetailPane` as a delete-and-rebuild.** The
  fence conditions (`isRule`/`isDiscovery`), the tooltip pattern for a *different*, still-valid
  future disabled state (none currently needed, but the `Tooltip.Provider` import stays useful
  elsewhere in the file), and the callback-prop shape are all worth keeping as-is.
- **Building a second CSRF or resume mechanism for the new dialogs.** `engramWrite` +
  `persistResume`/`peekResume`/`consumeResume` cover every write this phase needs; the
  `engram-connect-client` skill explicitly forbids a second client or a second CSRF path.
- **Giving `SupersedeMemory{validate_only:true}` its own bespoke query-key scheme.** It is a
  call on a write Procedure (still routes through `engramWrite`, per Phase 3 D-08), not a
  `createQuery` — model it as a `createMutation`-driven manual call (or a debounced imperative
  call) that updates local dialog state, not a TanStack Query cache entry; it must never be
  treated as cacheable across different target sets.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| CSRF token attach/verify | A second double-submit implementation for curation writes | `engramWrite` (already carries `[retryOnce, attachCsrf]`) | `engram-connect-client` skill: "do not hand-roll a second CSRF mechanism anywhere in `ui/`" |
| Connect error classification | Ad hoc `err.message` string matching per dialog | `parseConnectError`/`fixRowsFor` (`ui/src/lib/errors/connect-error.ts`) [VERIFIED, full file read] | Already handles the ambiguous-short_id-before-envelope ordering pitfall (ambiguous check must run before the `field=/hint=` regex — both share `Code.FailedPrecondition`) |
| State-word derivation/order/dim rule | A per-dialog copy of archived/superseded/expired/scheduled logic | `memoryStateWords`/`isPastState`/`STATE_WORD_ORDER` (`ui/src/lib/memorystate.ts`) [VERIFIED, full file read] | One canonical derivation per side (Go/TS), each with its own agreement test; a second TS copy risks drifting from the Go side silently |
| Combobox filtering for the add-target-by-short_id input | bits-ui `Command`'s default `shouldFilter` | `Command.Root shouldFilter={false}` + manual filter, exactly `ScopeCombobox.svelte`'s pattern [VERIFIED, full file read] | UI-SPEC cites the same bits-ui 2.18.1 gotcha (`3tz15e733n`): default filter empties content under Svelte 5 |
| Toast undo affordance | A custom toast/snackbar component | `svelte-sonner`'s `toast(msg, { action: {...}, duration })`, already used for the auth-redirect toast in `WriteSurfaces.svelte` [VERIFIED, full file read] | `Toaster` is already mounted once at the root layout [VERIFIED: ui/src/routes/+layout.svelte:8,46] — a second toast host would double-render |
| URL param codec for `/rules`/`/scheduled` | Manual `URLSearchParams` reads scattered through the route | One parse fn + one encode fn per route, mirroring `ui/src/lib/search/params.ts` | `engram-connect-client` skill: "One parse function, one encode function, per route — declared once so the two cannot drift" |

**Key insight:** almost nothing in this phase is a genuinely new problem — it is wiring three new
UI surfaces onto patterns (mutation hooks, cache-patch functions, error classification, resume
envelope, URL codecs) that four earlier phases already solved once each. The actual net-new work
is the dialogs' own markup/CSS (fully specified in `curation.md`) and the `ResumeEnvelope`
discriminated-union rewrite (Pattern 3 above), which has no precedent to copy.

## Common Pitfalls

### Pitfall 1: `ResumeEnvelope`'s flat, two-kind-only shape breaks silently, not loudly

**What goes wrong:** Adding a `supersede`/`archive`/`delete` value to `kind` without updating
`isValidShape`'s exhaustive `o.kind !== 'memory' && o.kind !== 'discovery'` check
[VERIFIED: ui/src/lib/resume.ts:97] means `peekResume()` will `return null` for every new-kind
envelope — the resume round-trip silently fails (draft is lost, no error surfaced) rather than
throwing a visible bug. This is the exact failure mode CUR-05's success criterion is designed to
catch, so it is a real risk, not a theoretical one.
**Why it happens:** `isValidShape` is a hand-written type guard, not derived from the
`ResumeEnvelope` type — TypeScript's compiler will not catch a forgotten branch here.
**How to avoid:** Write the resume round-trip test for a new kind FIRST (TDD-red), confirming
`peekResume()` returns non-null for a hand-constructed v2 envelope, before wiring the dialog to
call `persistResume`.
**Warning signs:** A dialog that reopens empty (no targets/ids restored) after a simulated re-auth
in a vitest-browser test, with no console error.

### Pitfall 2: Shift-modified row-action keys collide with nothing today, but the guard order matters

**What goes wrong:** `ROW_ACTION_KEYS.has(event.key) && isTypingTarget(event.target)` runs only
for keys already in `ROW_ACTION_KEYS` [VERIFIED: ui/src/lib/components/ResultsList.svelte:393] —
if `'S'`/`'A'` are added to `HANDLED_KEYS` but forgotten from `ROW_ACTION_KEYS`, they will fire
`current.onKey('S')` even while a text field has focus, breaking `isTypingTarget`'s guarantee that
typing in the header search never fires a row action.
**Why it happens:** `HANDLED_KEYS` and `ROW_ACTION_KEYS` are two separate `Set`s that must be kept
in sync by hand [VERIFIED: ui/src/lib/components/ResultsList.svelte:330-334].
**How to avoid:** Add `'S'`/`'A'`/`'x'` to `ROW_ACTION_KEYS` (or a sibling `SELECTION_KEYS` set
folded into the same guard clause), never only to `HANDLED_KEYS`.
**Warning signs:** Typing "Supersede" in the search input triggers the archive dialog because a
capital `S` slipped through un-guarded.

### Pitfall 3: `SupersedeMemoryResponse.targets` is a compact `Memory` view, not the full record

**What goes wrong:** Prefilling the correcting-record form or rendering chain-preview chips
directly from `targets: repeated Memory` [VERIFIED: proto/engram/v1/engram.proto:497-503,
`message SupersedeMemoryResponse { string id = 1; string short_id = 2; bool validated = 3;
repeated string supersedes = 4; repeated Memory targets = 5; }`] without checking whether the
server's compact view clears `content` (the same summary-vs-full distinction `WriteSurfaces.openEdit`
already guards against for `GetMemory` [VERIFIED: ui/src/lib/components/WriteSurfaces.svelte:91-95]
`"list/search rows are summary-shaped: server clears content when full=false — prefilling from one
would let a Save overwrite the real body with empty content"`) risks the same class of bug: writing
back an emptied `content` field.
**Why it happens:** The proto comment for `SupersedeMemoryResponse` [VERIFIED:
proto/engram/v1/engram.proto:492-496] only says targets is a "compact summary view" without
specifying whether `content` is included — this needs to be confirmed against the actual Go
implementation (`internal/server` or `internal/surfaces`) at execution time, not assumed either way.
**How to avoid:** Verify server-side whether `targets[]` in a `validate_only` response includes
full `content`; if not, the correcting-record prefill (D-07: "Prefilled from newest predecessor")
must come from a `GetMemory` call on the newest predecessor, not from `targets[]` directly.
**Warning signs:** The prefilled correcting-record content field is empty even though the
predecessor visibly has content in the list.

### Pitfall 4: `RecallSplit`/`WriteSurfaces` placement precedent — do not let `CurationSurfaces` get torn down by a layout switch

**What goes wrong:** `search/+page.svelte` places `WriteSurfaces` **outside** `RecallSplit`'s
`{#snippet list()}`/`{#snippet detail()}` blocks specifically because those snippets are
re-rendered across `RecallSplit`'s narrow/wide layout branches, which would destroy and recreate
any component placed inside them — losing `bind:this` and one-shot `onMount` resume logic
[VERIFIED: ui/src/routes/search/+page.svelte:496-503, the comment explicitly documents this].
**Why it happens:** `{#if}`/`{:else}` branch switches in Svelte tear down and recreate their
contents; a component instantiated inside a conditionally-rendered snippet is not stable across
that switch.
**How to avoid:** Instantiate `CurationSurfaces` in the same stable location `WriteSurfaces`
already occupies (the `.search-toolbar` region, outside `RecallSplit`), for `/search`; for the new
`/rules`/`/scheduled` routes (which do not have `RecallSplit`'s narrow/wide switch), place it
anywhere stable in the route template.
**Warning signs:** A supersede dialog that loses its in-progress draft the moment the browser
window is resized across the narrow/wide breakpoint.

### Pitfall 5: `csrfWriteProcedures`'s count is pinned by a literal-count test — do not touch it this phase

**What goes wrong:** `TestCSRFWriteProcedureAllowlist` pins `len(csrfWriteProcedures)` to exactly
9 [VERIFIED: internal/server/connectcsrf_test.go:200-201] `if got := len(csrfWriteProcedures); got
!= 9 { t.Fatalf("csrfWriteProcedures has %d entries, want exactly 9", got) }`. `SupersedeMemory`,
`ArchiveMemory`, `RestoreMemory` are already 3 of those 9
[VERIFIED: internal/server/connectcsrf.go:36-46]. `ListRules`/`ListScheduled` are reads and must
**not** be added to this map. A plan task that "adds the curation RPCs to the CSRF allowlist" is
redundant work against an already-green test and risks breaking the pinned count if miscounted.
**Why it happens:** Phase 3 already did this; Phase 4 only consumes it.
**How to avoid:** Treat `internal/server/connectcsrf.go` as read-only reference material this
phase, not an edit target.
**Warning signs:** A diff touching `connectcsrf.go` in a Phase 4 plan.

### Pitfall 6: `/observe`'s two removal sites are test files, not just app code

**What goes wrong:** D-14 requires updating "`internal/e2e` and `internal/webauth/static_test.go`
(which uses `/observe` only as a sample client route)." The exact call sites are narrow and easy
to miss in a broad find-and-replace: `internal/webauth/static_test.go:24`
[VERIFIED: `h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/observe", nil))`] uses `/observe`
purely as an arbitrary SPA-client-route fixture (any client route would do — it is not testing
`/observe`'s existence, just that *some* client-side route 200s through the static handler).
`internal/e2e/console_browser_test.go` navigates `observeURL := fixture.srv.baseURL() +
"/ui/observe?scope=" + url.QueryEscape(fixtureScope)` [VERIFIED: internal/e2e/console_browser_test.go:438,
441] as its second of two navigations, explicitly to prove the scoped round trip works.
**Why it happens:** Both files predate this phase and treat `/observe` as a stable, arbitrary
example route rather than a route under test.
**How to avoid:** In `static_test.go`, swap the fixture path to any surviving route (e.g. `/search`).
In `console_browser_test.go`, swap the second navigation to the D-14-specified replacement
(`/search?q=scope=<x>`-shaped) or to one of the new `/rules`/`/scheduled` routes if that better
serves DSYS-04's "entry-point resolution" requirement — this is also where the phase's chromedp
supersede/archive-round-trip additions belong (D-18).
**Warning signs:** `go test ./internal/webauth/... ./internal/e2e/...` failing with a 404 after
`/observe`'s route is deleted, in a way that looks unrelated to the phase's own new code.

### Pitfall 7: `MEMORY_LIST_PREFIXES` omits `listRules`/`listScheduled` — a curation write can leave those caches stale

**What goes wrong:** `snapshotMemoryQueries`/`applyToMemoryCaches` only walk `['listMemories']` and
`['searchMemories']` [VERIFIED: ui/src/lib/mutations/memory.ts:160]
`const MEMORY_LIST_PREFIXES = [['listMemories'], ['searchMemories']] as const;`. The
`engram-connect-client` skill's own invalidation table says a successful archive/restore/supersede
must invalidate `'searchMemories'`, `'listMemories'`, `'getMemory'`, `'listScheduled'`,
`'relatedMemories'`, and `'listTags'` — `listScheduled` and (for rules-adjacent work) `listRules`
are outside the existing prefix list and will be invalidated only if the new curation mutation
hooks explicitly add those `invalidateQueries` calls (matching the pattern in `useDeleteMemory`'s
`onSettled` [VERIFIED: ui/src/lib/mutations/memory.ts:348-355]), not automatically via
`applyToMemoryCaches`'s existing prefix walk.
**Why it happens:** `MEMORY_LIST_PREFIXES` was written before Scheduled/Rules existed.
**How to avoid:** New curation mutation hooks' `onSettled` must explicitly
`queryClient.invalidateQueries({ queryKey: ['listScheduled'] })` (and `['listRules']` if a rule
delete affects rule visibility elsewhere) alongside the existing list/search/getMemory calls — do
not assume the shared cache-patch helper covers it.
**Warning signs:** Archiving a record from `/search` leaves a stale (non-expired-looking) row in
an already-open `/scheduled` tab until a manual refresh.

## Code Examples

### Extending the keyboard model (ResultsList.svelte)

```typescript
// Source: ui/src/lib/components/ResultsList.svelte:330-334, current shipped code
const NAV_KEYS = new Set(['j', 'k', 'ArrowDown', 'ArrowUp', 'Home', 'End', 'Enter']);
const ROW_ACTION_KEYS = new Set(['e', 's', '#', 'c', 'C']);
const HANDLED_KEYS = new Set([...NAV_KEYS, ...ROW_ACTION_KEYS, 'Escape']);

// Phase 4 extension (planner to verify final naming/placement):
const ROW_ACTION_KEYS = new Set(['e', 's', '#', 'c', 'C', 'S', 'A']); // ⇧S supersede, ⇧A restore
const SELECTION_KEYS = new Set(['x']); // x toggle, ⇧x range-select — event.key for Shift+x is 'X'
// 'a' (bare, no shift) is a NEW bare row-action key for Archive — add to ROW_ACTION_KEYS too.
const HANDLED_KEYS = new Set([...NAV_KEYS, ...ROW_ACTION_KEYS, ...SELECTION_KEYS, 'X', 'Escape']);
```

### The undo toast (D-09 surface 2), following `WriteSurfaces.svelte`'s existing toast-with-action precedent

```typescript
// Source: ui/src/lib/components/WriteSurfaces.svelte:146-148, the existing pattern for a
// toast carrying an action button — the archive-undo toast follows this shape verbatim,
// with duration confirmed available on svelte-sonner's ExternalToast (see Standard Stack).
toast(`${n} archived · Undo`, {
  duration: 8000,
  action: { label: 'Undo', onClick: () => restoreMutation.mutate({ ids }) }
});
```

### `DetailPane`'s existing predecessor/successor links — reused as-is for CUR-01's pane requirement

```svelte
<!-- Source: ui/src/lib/components/DetailPane.svelte:182-195, ALREADY SHIPPED, no change needed -->
{#if stateWords.includes('superseded') && memory.supersededBy}
  <div>
    superseded by
    <button type="button" class="d-link" onclick={() => onselect?.(memory!.supersededBy!)}>{memory.supersededBy}</button>
  </div>
{/if}
{#if memory.supersedes.length > 0}
  <div>
    supersedes
    {#each memory.supersedes as predId (predId)}
      <button type="button" class="d-link" onclick={() => onselect?.(predId)}>{predId}</button>
    {/each}
  </div>
{/if}
```

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `SupersedeMemoryResponse.targets`'s "compact summary view" clears `content` the same way a summary-shaped `Memory` from `ListMemories`/`SearchMemories` does | Pitfall 3 | If content is actually included, no harm (planner can simplify by using `targets[]` directly for prefill); if it is cleared and the planner assumes otherwise, the correcting-record prefill silently loses content |
| A2 | Selection state (D-02/D-03) is best owned by `ResultsList.svelte` itself (co-located with `activeId`) rather than lifted entirely to each route | Architecture Pattern 2 | If the planner instead lifts selection fully to the route, `ResultsList`'s own `Escape`-precedence reordering (D-03) becomes harder to implement locally and may need a prop-drilled `hasSelection` flag instead — a valid alternative, just a different task shape |
| A3 | `ResultsHeader.svelte` gaining a `selection` prop (rather than a sibling `BulkActionBar` swapped in by the route) is the lower-risk implementation of D-03's bulk bar | Architecture Pattern 4 | Both are UI-SPEC-compatible; picking the sibling-component route means the header/bar swap logic lives in the route template instead, a purely organizational difference |
| A4 | `/rules` and `/scheduled` each need their own `parse`/`encode` URL-codec module (mirroring `search/params.ts`) rather than reusing `search/params.ts`'s existing functions | Don't Hand-Roll table | If the two new routes' URL surface turns out to be simple enough (e.g. `/scheduled?tab=expired`), a single small shared helper might suffice instead of two new files — low risk either way, purely a file-count difference |

**If this table is empty:** N/A — see above.

## Open Questions

1. **Does `SupersedeMemoryResponse.targets` (validate_only response) include full `content`?**
   - What we know: the proto comment only says "compact summary view" without specifying which
     fields are cleared (unlike `ListMemoriesResponse`/`SearchMemoriesResponse`, whose summary-vs-
     full behavior is documented and already has a client-side guard in `WriteSurfaces.openEdit`).
   - What's unclear: whether the Go implementation (Phase 3, `internal/server`/`internal/surfaces`)
     clears `content` on this specific response field the same way it does for list/search rows.
   - Recommendation: the planner should grep the Phase 3 implementation
     (`internal/server/connectapi.go` or wherever `SupersedeMemory`'s validate_only path is
     implemented) before deciding whether the correcting-record prefill can read `targets[]`
     directly or must issue a `GetMemory` on the newest predecessor.

2. **Exact final home for bulk-selection state and the `Escape`-precedence reorder.**
   - What we know: D-02/D-03 require selection state, a checkbox column, and a reordered `Escape`
     precedence; `ResultsList.svelte` already owns `activeId`/`cardOpen` state in the same shape
     selection would need.
   - What's unclear: whether the planner should extend `ResultsList.svelte` directly (more
     cohesive, but grows an already-large file) or introduce a new co-located composable/module
     for selection logic that `ResultsList` consumes (better separation, more files to keep in
     sync).
   - Recommendation: extend `ResultsList.svelte` directly, following the file's existing
     single-component-owns-its-state pattern (`activeId`, `cardOpen` are both local `$state` in
     the same file) — this avoids a new prop-drilling surface between `ResultsList` and a
     hypothetical selection module for what is fundamentally listbox-internal state.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Chromium (Playwright) | `pnpm test:browser` (vitest-browser-svelte), CI's e2e job | not probed this session — CI installs via `pnpm exec playwright install --with-deps chromium` [VERIFIED: .github/workflows/ci.yaml:429] | — | CI-managed; local dev needs `pnpm exec playwright install` once |
| chromedp / a live `engram` binary + Qdrant | `internal/e2e/console_browser_test.go`'s DSYS-04 additions | Existing test harness already builds and runs a local binary against a live Qdrant (testcontainer or `ENGRAM_QDRANT_ADDR`) [CITED: internal/e2e package structure, existing `TestConsoleBundleRendersRecordInBrowser`] | — | None needed — this is the established pattern the new supersede/archive-round-trip test extends |
| Third-party a11y/design skills (`pbakaus/impeccable`, `vercel-labs/agent-skills@web-design-guidelines`, `addyosmani/web-quality-skills@accessibility`) | DSYS-03 | Not yet installed — gated behind `fable-security-review` per CONTEXT.md's Folded Todos | — | Fallback IS specified in CONTEXT.md D-18/Folded Todos itself: manual WCAG 2.2 checklist + axe-core in vitest-browser + Web Interface Guidelines read by hand, with the failed-skill verdict recorded |

**Missing dependencies with no fallback:** none — every dependency above has either an existing
CI-managed install path or an explicit fallback already decided in CONTEXT.md.

**Missing dependencies with fallback:** the three third-party audit skills (fallback: manual
WCAG 2.2 + axe-core + hand review, per D-18).

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | vitest `5.0.1` + `vitest-browser-svelte` `3.1.0` [VERIFIED: ui/package.json] (browser project = Chromium via Playwright) + Go `internal/e2e` chromedp harness |
| Config file | `ui/vitest.config.ts` (node + browser projects) |
| Quick run command | `cd ui && pnpm test:browser -- <specific test file>` for a single new dialog's vitest-browser test; `go test ./internal/e2e/ -run TestConsoleCuration -v` for a single new chromedp scenario once added |
| Full suite command | `cd ui && pnpm test` (runs both `test:node` and `test:browser` projects, per `package.json` scripts [VERIFIED: ui/package.json]); `go test ./internal/e2e/ -count=1 -v` (Taskfile `test:e2e` [VERIFIED: Taskfile.yaml:55-58]) |

**Note:** `task test` (the repo-wide gate) does **not** run `ui/`'s vitest suite — confirmed by
reading `Taskfile.yaml`'s `test`/`test:go`/`test:python` blocks in full, none of which touch `ui/`.
CI runs `ui/`'s `pnpm test` in a dedicated job [VERIFIED: .github/workflows/ci.yaml:410-433]. The
phase's own verification loop must invoke `pnpm test`/`pnpm test:browser` directly under `ui/`,
not rely on `task`.

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CUR-01 | Supersede dialog: preview, chain preview, commit, pane forward/back links | vitest-browser (dialog) + chromedp (full round trip) | `pnpm test:browser -- SupersedeDialog` / `go test ./internal/e2e/ -run TestConsoleSupersede -v` | ❌ Wave 0 — both new |
| CUR-02 | Archive/restore confirm, in-place patch, two undos | vitest-browser + chromedp | `pnpm test:browser -- ArchiveConfirmDialog` / `go test ./internal/e2e/ -run TestConsoleArchiveRestore -v` | ❌ Wave 0 — both new |
| CUR-03 | Rules view: index, on-demand full text, delete-only | vitest-browser | `pnpm test:browser -- rules` (route test, following `search/search.browser.test.ts`'s shape [VERIFIED: ui/src/routes/search/search.browser.test.ts exists]) | ❌ Wave 0 — new |
| CUR-04 | Scheduled view: tabs, archive-on-expired-only | vitest-browser | `pnpm test:browser -- scheduled` | ❌ Wave 0 — new |
| CUR-05 | Resume round-trip per new kind | vitest-browser (unit-level `resume.ts` tests, existing precedent: node-testable pure functions) | `pnpm test:node -- resume` | ❌ Wave 0 — new test cases; `resume.ts` itself has no dedicated existing test file found this session (planner to confirm/create) |
| DSYS-03 | WCAG 2.2 keyboard/contrast audit | axe-core in vitest-browser (fallback path) or third-party skill (primary path) | tool-dependent, decided by the `fable-security-review` outcome | ❌ Wave 0 |
| DSYS-04 | Entry-point resolution + supersede + archive/restore chromedp round trip | chromedp, extending `internal/e2e/console_browser_test.go` | `go test ./internal/e2e/ -count=1 -v` | ⚠️ File exists (`TestConsoleBundleRendersRecordInBrowser`), new test function(s) needed |

### Sampling Rate
- **Per task commit:** `pnpm test:browser -- <touched component>` (Svelte/dialog work); `go test
  ./internal/e2e/ -run <NewTest> -v` (e2e additions, run standalone — the harness is slow).
- **Per wave merge:** `pnpm test` (full ui/ suite) + `go test ./internal/webauth/... ./internal/server/...`
  (unaffected by this phase but cheap to confirm no accidental drift) + `go test ./internal/e2e/ -count=1 -v`.
- **Phase gate:** `pnpm test` + `go test ./internal/e2e/ -count=1 -v` + `task ui:build` (SPA rebuild)
  + the `ui-drift` CI job's diff check (or its local equivalent: rebuild and `git diff
  internal/webauth/static`) green before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `ui/src/lib/resume.browser.test.ts` or `.node.test.ts` — no dedicated resume-envelope test
      file was found this session; confirm whether resume logic is tested inline in each write
      surface's own test file or needs a new dedicated one for the v2 discriminated-union kinds.
- [ ] `ui/src/routes/rules/rules.browser.test.ts`, `ui/src/routes/scheduled/scheduled.browser.test.ts`
      — new route test files, following `ui/src/routes/search/search.browser.test.ts`'s shape.
- [ ] `internal/e2e/console_browser_test.go` — new test function(s) for supersede and
      archive/restore-undo round trips (extending, not replacing, `TestConsoleBundleRendersRecordInBrowser`).
- [ ] `internal/webauth/static_test.go` — the `/observe` fixture path swap (Pitfall 6) is a same-file
      edit, not a new file.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes (indirect) | Unchanged — OIDC bearer-token enforcement (`internal/auth`) already shipped; this phase only adds re-auth-resume UX around the existing flow, no new auth surface |
| V3 Session Management | yes | CSRF double-submit (`engram_csrf` cookie + `X-CSRF-Token` header) already shipped and already covers the three new write RPCs [VERIFIED: internal/server/connectcsrf.go:36-46]; the resume envelope (`sessionStorage`, 10-minute TTL) is client-side transient state, not a session credential — no new server-side session surface |
| V4 Access Control | yes (indirect) | Server-enforced only (`internal/store`, `DEC-cgb`); the console never re-derives ownership/authz client-side — it only renders the server's per-target issue codes (`not-owned`, `already-superseded`, etc.) |
| V5 Input Validation | yes | Curation dialogs' client-side per-target validity checks are UX-only fast feedback; the server's `validate_only` (supersede) and per-id outcome (archive/restore) responses are authoritative and must never be overridden by a client-side "looks fine" state (D-07) |
| V6 Cryptography | no | No new crypto surface this phase |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Open-redirect via a tampered `returnPath` in the resume envelope | Tampering | `isAllowedDestination`'s existing allowlist check [VERIFIED: ui/src/lib/resume.ts:59-65] — the new kinds must not bypass this; `/rules`/`/scheduled` are added to `ALLOWED_DESTINATIONS`, never a wildcard match |
| CSRF on a new write RPC forgotten from the allowlist | Tampering / Spoofing | Not a risk this phase — the three write RPCs are already in `csrfWriteProcedures` (Phase 3); the planner must NOT accidentally add `ListRules`/`ListScheduled` (reads) to this map (Pitfall 5) |
| Client-side "looks valid" state presented as if it were server-confirmed (e.g. a target chip shown "valid" client-side when the server would reject it on a race) | Tampering / Repudiation (of what was actually written) | D-07's explicit rule: server's `validate_only` answer wins on conflict with client-side per-target checks; never let the primary submit button's enabled state diverge from the server's last-known validation |
| Duplicate supersede/archive on a re-auth retry | Repudiation (duplicate write) | `idempotency_key` (one per draft, reused on retry) for supersede [D-07]; archive/restore's per-id outcomes are independently idempotent by design (`ArchiveOutcome.ALREADY_ARCHIVED`/`NOT_ARCHIVED` are non-error information, per D-09) — no client-side de-dup needed beyond reusing the same key/reusing the same dialog draft |

## Sources

### Primary (HIGH confidence — read in full this session)
- `.planning/phases/04-curation-surfaces/04-CONTEXT.md` — locked decisions D-01..D-18
- `.planning/phases/04-curation-surfaces/04-UI-SPEC.md` — visual/interaction contract
- `.planning/REQUIREMENTS.md`, `.planning/STATE.md` — milestone scope and decision history
- `proto/engram/v1/engram.proto` (curation RPC/message definitions, lines 404-742)
- `internal/server/connectcsrf.go`, `internal/server/connectcsrf_test.go` (CSRF allowlist + its pinned-count test)
- `ui/src/lib/resume.ts`, `ui/src/lib/components/WriteSurfaces.svelte`, `ui/src/lib/components/DeleteConfirmDialog.svelte`
- `ui/src/lib/components/ResultsList.svelte`, `ResultRow.svelte`, `ResultsHeader.svelte`, `DetailPane.svelte`, `ScopeCombobox.svelte`
- `ui/src/lib/mutations/memory.ts`, `ui/src/lib/memorystate.ts`, `ui/src/lib/errors/connect-error.ts`, `ui/src/lib/client.ts`
- `ui/src/lib/components/AppShell.svelte`, `CommandMenu.svelte`, `ui/src/routes/+page.svelte`, `ui/src/routes/search/+page.svelte`, `ui/src/routes/+layout.svelte`
- `ui/package.json`, `ui/src/lib/components/ui/` directory listing, `ui/node_modules/svelte-sonner/dist/types.d.ts`
- `Taskfile.yaml`, `.github/workflows/ci.yaml` (ui-drift and e2e job definitions)
- `internal/e2e/console_browser_test.go`, `internal/webauth/static_test.go` (`/observe` call sites)
- `.claude/skills/engram-connect-client/SKILL.md`, `.claude/skills/engram-console-conventions/SKILL.md`
- `.claude/skills/sketch-findings-engram/references/curation.md`, `.claude/skills/sketch-findings-engram/references/foundations.md`
- `CLAUDE.md` (project instructions)

### Secondary (MEDIUM confidence)
- None — no external/web sources were needed this session; the phase is entirely grounded in
  already-shipped code and already-locked design documents. `mcp__context7` was not consulted:
  no new library or API surface is introduced this phase.

### Tertiary (LOW confidence)
- Open Question 1 (`SupersedeMemoryResponse.targets` content-clearing behavior) — flagged as
  unverified in this session; the planner should confirm against the Go implementation before
  committing to a prefill strategy.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every dependency verified present at its exact version in `ui/package.json`
  and `ui/node_modules`; zero new packages needed.
- Architecture: HIGH — every extension point (resume.ts, ResultsList.svelte, memory.ts,
  DetailPane.svelte, ResultsHeader.svelte) was read in full this session with exact line citations.
- Pitfalls: HIGH — all seven are grounded in code actually read this session, not inferred from
  the design docs alone; two (Pitfall 3, and the resume-envelope back-compat requirement inside
  Pitfall 1) carry a residual LOW-confidence sub-claim flagged in the Assumptions Log / Open
  Questions rather than stated as fact.

**Research date:** 2026-09-27
**Valid until:** 30 days (stable, no external API surface; re-verify if Phase 3's `SupersedeMemory`
implementation changes before this phase executes)
