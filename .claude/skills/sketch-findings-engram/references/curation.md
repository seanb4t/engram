# Curation: supersede with chain, archive and restore

How the console corrects records (sketch 003, winner A: modal dialog). Read `foundations.md`
for tokens, state chips and keys. The semantics here are the server's (see project `CLAUDE.md`,
"Supersession" and "Archived state"); the UI must not promise anything the server does not do.

## Design Decisions

### Ceremony matches risk

- **Supersede cannot be undone.** It stores a new correcting record and stamps `superseded_by`
  on every predecessor. It is additive history; the only fix is to supersede again. So it gets
  a **full modal dialog** with validation and a live preview.
- **Archive is always reversible.** It stamps `archived_at`; Restore clears it. So it gets a
  **small confirm dialog** and an **Undo on the result**. Not zero-ceremony: the confirm names
  exactly which records change and blocks the ones you do not own.

### Supersede dialog (large, about 920px)

- **Header:** "Supersede N records into one" and one line: "Stores a new correcting record and
  stamps `superseded_by` on every predecessor. Additive — nothing is deleted."
- **Body, two columns:**
  - **Left: Predecessors (N).** One target chip per record: category dot, short_id, summary,
    remove `×`. Under a valid chip: created date · scope · "has N predecessor(s)". Under an
    invalid chip, an inline issue line with a mono code pill, in the server's evaluation order:
    1. `not-owned` — "shared by priya — readable, but only the owner can supersede"
    2. `rule` — "rules cannot be superseded — delete it instead"
    3. `already-superseded` — "not the live head — current head is k3m9q2x7aa" with a
       **"use head k3m9q2x7aa"** link that swaps the target for its head
    4. `server` — "already superseded by another session" (after a race rejection)

    Below the chips: "add target by short_id…" input with id autocomplete.
  - **Right: Correcting record, prefilled** from the **newest** predecessor (summary, content,
    category, tags, scope), with a note "Prefilled from newest predecessor k3m9… · write the
    example correction". Fields: summary (with a live `N/512 B` byte counter that turns danger
    over the limit), content (required; empty on blur shows
    `field=content hint=required: content must not be empty`), category and scope selects side
    by side, tags. A warning line when targets span several scopes: "Targets span 2 scopes —
    the new record lands in one." A rule's `rule:*` scope is never offered as the destination.
- **Chain preview, live** (full-width strip under the body): columns **oldest on the left, head
  on the right**, each column headed `d2`, `d1`, `head · d0`; arrows read "superseded by". The
  new record is a dashed violet node `(new)` at the head; invalid targets render struck through
  in danger colour. Updates on every keystroke and target change without re-rendering inputs.
  Legend: "← predecessors (superseded, hidden from recall) · arrows read 'superseded by' · head
  is the live record".
- **Footer:** a gate message on the left, `⌘↵` hint, Cancel, primary **"Supersede N → 1"**.
  The button stays disabled while any gate holds, and the gate says why:
  - "Add at least one target"
  - "Remove 1 invalid target — an invalid set rejects the whole call"
  - "content is required"
  - otherwise, in faint text: "Additive: predecessors get superseded_by, nothing is deleted"
- **Status block** (between body and footer) for the call:
  - loading: spinner + `supersede_memory · 2 targets…`
  - validation: "Rejected — fix the named field and resend" + pills `field=summary`
    `hint=too_long` + the text + the raw envelope in faint mono; the named field gets the error
    ring and its inline `ferr` line.
  - server rejection: "Server rejected the call — nothing was written" + pill
    `target is already superseded` + the target pill + the explanation ("Another session
    superseded this target after you opened the editor. Supersede its current head instead.");
    the rejected target's chip turns bad.
  - re-auth: warning block "Session expired. Nothing was written; your draft is kept." with
    **"Re-authenticate & retry"**, which re-runs the OIDC flow and resubmits the same draft.
- **Success replaces the dialog body** (medium width): "✓ Superseded 2 records → p4n7…",
  "Stored p4n7… and stamped superseded_by on k3m9…, h8r2…. They drop out of recall but stay
  fetchable by id.", the resulting chain (same columns, new head highlighted), and a note
  **"No undo. Supersession is additive history, not an edit. To change it, supersede p4n7…
  again."** Footer: "View superseded (N)", "Open p4n7…", primary Done.
  - **View superseded** closes the dialog, turns on "show hidden" in the list, flashes the
    predecessors and the new record, and scrolls to the first predecessor.
- **Chain dialog** (from a row's Chain action or `Enter` on a chained row): header
  "Chain · short_id" with "depth d1 below head … · N direct predecessor(s) · successor …" and
  the same columns with the row highlighted. Every node is clickable and opens a fetch-by-id
  peek ("get_memory k3m9… · fetch-by-id ignores the recall gate · superseded_by …"). Footer
  offers "Supersede head…" when the head is supersedable.

### Archive / Restore confirm (small, about 440px)

- **Header:** "Archive 2 records?" + "Stamps archived_at. Drops out of search, list, discovery
  and scheduled recall; still fetchable by id. Reversible — Restore clears it." (Restore:
  "Clears archived_at. The record returns to normal recall.")
- **Body:** one chip per record with its state chips; `not-owned` issue on records you do not
  own ("shared by priya — archive is owner-only"); a rule shows the note "owners may archive
  rules (they cannot be superseded)". Records can be removed from the set with `×`.
- **Footer:** gate "Remove 1 record you don't own", Cancel, and the verb button (**Archive** in
  the danger-outline style, **Restore** in primary).
- **Result:** "✓ 2 records archived" + "Hidden from recall; get_memory still returns them."
  Idempotent cases are reported, not errors: "h8r2… already archived — idempotent, no change."
  Footer: **"Undo — restore 2"** (runs the inverse call through the same dialog) and Done.
- Same loading / validation / rejection / re-auth status blocks as supersede; rejections for
  archive read `not found: <sid>` with the note "Not found, not owned and ambiguous short id
  are one rejection by design. Nothing was changed."

### List affordances

- Row hover (or keyboard focus) reveals right-aligned action buttons over a gradient fade:
  **Supersede**, **Archive** or **Restore**, and **Chain** when the row is in a chain.
- A checkbox column (faint until hover or selection) enables bulk actions; a selection turns the
  list header into a violet bulk bar: "N selected · Supersede N into one… `s` · Archive `e` ·
  Restore `u` · clear".
- Rows with state are dimmed with the summary struck through; a chain icon marks chain members;
  a dashed owner pill marks someone else's shared record (read-only for you).
- "show hidden N" toggle (`h`) in the list header reveals superseded and archived rows.
- New or changed rows flash violet-soft for about 1.6s after a successful call.

### Semantics the UI must reflect

- A superseded or archived record is **soft-hidden**: gone from `search_memory`, `list_memory`,
  `search_discovery` and `list_scheduled`, still returned by `get_memory`. Say so after every
  success and on every chain node.
- `supersedes` is a **set**; an invalid member rejects the whole call. The single-live-head
  rule is enforced per target. Each predecessor has exactly one successor; the new record may
  have several predecessors.
- A non-owned, nonexistent, or ambiguous target produce one indistinguishable rejection. Do not
  invent a distinction the server hides.
- Rules cannot be superseded (delete instead); `set_visibility` does not apply to rules.
- `idempotency_key` is accepted on supersede: a retry after an ambiguous failure (for example
  the re-auth path) should replay, not duplicate. Send one key per draft.
- Archive never writes an expiry or a supersession link; archived / superseded / expired /
  scheduled clear independently.

### Stack mapping

| Sketch piece | Real stack |
|---|---|
| Supersede, archive, chain dialogs | shadcn-svelte `Dialog` (sizes via `class` on `Dialog.Content`); `Esc` and scrim click close; focus trapped |
| Target add-by-short_id | `Command`/`Combobox` fed by a server lookup, not a `<datalist>` |
| Per-target issues | computed client-side from the loaded records for instant feedback, then trusted to the server's response (sentinel errors map back onto chips) |
| Envelope parsing | one helper that splits `field=<f> hint=<h>: <text>` into pills, and falls back to `<sentinel>: <args>` |
| Re-auth resume | the console's existing CSRF + re-auth resume envelope; the dialog keeps its draft across the OIDC round trip |
| Mutations | TanStack Query `createMutation`; on success invalidate recall queries, then flash |
| Undo / confirmations outside dialogs | svelte-sonner toasts (for example after "View superseded"), never as the only record of a supersede |

The new Connect write RPCs (`SupersedeMemory`, `ArchiveMemory`, `RestoreMemory`) are prerequisites;
the sketch fakes them with the MCP / spine-review shapes.

## CSS Patterns

```css
.scrim { position: absolute; inset: 0; z-index: 40; display: flex; align-items: center; justify-content: center;
  background: color-mix(in srgb, var(--color-bg) 70%, transparent); opacity: 0; transition: opacity .15s ease; }
.scrim.open { opacity: 1; }
.dialog { background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg); display: flex; flex-direction: column; max-height: 100%;
  transform: translateY(calc(6*var(--u))) scale(.985); transition: transform .15s ease; }
.scrim.open .dialog { transform: none; }
.dialog.lg { max-width: calc(920*var(--u)); } .dialog.md { max-width: calc(640*var(--u)); } .dialog.sm { max-width: calc(440*var(--u)); }
.dlg-b.two { display: grid; grid-template-columns: minmax(0, calc(300*var(--u))) minmax(0, 1fr); gap: calc(16*var(--u)); }
.dlg-chain { padding: calc(10*var(--u)) calc(14*var(--u)); border-top: 1px solid var(--color-border); background: var(--color-bg); }
@container frame (max-width: 700px) { .dlg-b.two { grid-template-columns: 1fr; } }

/* target chip with inline issue */
.tchip { border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-surface);
  padding: calc(6*var(--u)) calc(6*var(--u)) calc(6*var(--u)) calc(8*var(--u)); }
.tchip.bad { border-color: color-mix(in srgb, var(--color-danger) 55%, var(--color-border));
  background: color-mix(in srgb, var(--color-danger) 7%, var(--color-surface)); }
.tissue { font-size: var(--text-xs); color: var(--color-danger); padding-left: calc(13*var(--u)); }
.tissue .code { font-family: var(--font-mono); font-size: var(--text-2xs); border: 1px solid currentColor;
  border-radius: calc(3*var(--u)); padding: 0 calc(4*var(--u)); margin-right: calc(4*var(--u)); }

/* chain columns: oldest left -> head right */
.chain-cols { display: flex; align-items: stretch; gap: calc(6*var(--u)); overflow-x: auto; }
.ccol { display: flex; flex-direction: column; gap: calc(4*var(--u)); justify-content: center; flex: none; max-width: calc(280*var(--u)); }
.cnode { display: inline-flex; align-items: center; gap: calc(6*var(--u)); height: calc(24*var(--u)); padding: 0 calc(8*var(--u));
  border: 1px solid var(--color-border); border-radius: var(--radius-sm); background: var(--color-surface); font-size: var(--text-xs); }
.cnode.hl  { border-color: var(--color-primary); box-shadow: 0 0 0 2px var(--color-primary-soft); }
.cnode.new { border-style: dashed; border-color: var(--color-primary); }
.cnode.bad { border-color: var(--color-danger); } .cnode.bad .csum { text-decoration: line-through; }

/* status + envelope pills */
.status.err  { border-color: color-mix(in srgb, var(--color-danger) 55%, var(--color-border)); background: color-mix(in srgb, var(--color-danger) 8%, var(--color-surface)); }
.status.warn { border-color: color-mix(in srgb, var(--color-warning) 55%, var(--color-border)); background: color-mix(in srgb, var(--color-warning) 8%, var(--color-surface)); }
.pill.k { font-family: var(--font-mono); font-size: var(--text-2xs); color: var(--color-danger);
  border: 1px solid color-mix(in srgb, var(--color-danger) 45%, var(--color-border)); border-radius: calc(3*var(--u)); }
.fld.err .in { border-color: var(--color-danger); box-shadow: 0 0 0 calc(3*var(--u)) color-mix(in srgb, var(--color-danger) 15%, transparent); }
.btn.warnish { color: var(--color-danger); border-color: color-mix(in srgb, var(--color-danger) 45%, var(--color-border)); }  /* Archive */

/* row actions revealed on hover/focus over a fade */
.acts { position: absolute; right: calc(6*var(--u)); top: 50%; transform: translateY(-50%); opacity: 0; pointer-events: none;
  padding-left: calc(24*var(--u)); background: linear-gradient(90deg, transparent, var(--color-hover) calc(20*var(--u))); transition: opacity .15s ease; }
.row:is(:hover, .focus) .acts { opacity: 1; pointer-events: auto; }
.row.dimrow .sumt { color: var(--color-text-faint); text-decoration: line-through; }
.row.flash { animation: flash 1.6s ease; }
@keyframes flash { 0%, 30% { background: var(--color-primary-soft); } 100% { background: transparent; } }
```

## HTML Structures

```html
<div class="dialog lg" role="dialog" aria-labelledby="sup-h">
  <div class="dlg-h"><div><b id="sup-h">Supersede 2 records into one</b>
    <div class="dim">Stores a new correcting record and stamps <span class="mono">superseded_by</span> on every predecessor. Additive — nothing is deleted.</div></div>
    <button class="x" aria-label="Close">×</button></div>
  <div class="dlg-b two">
    <div><div class="lbl">Predecessors <span class="mono">2</span></div>
      <div class="tchip"><div class="tchip-top"><i class="dot"></i><span class="mono sid">k3m9q2x7aa</span><span class="tsum">…</span><button class="x">×</button></div>
        <div class="tok">2026-09-24 · repo:engram · has 1 predecessor(s)</div></div>
      <div class="tchip bad"><div class="tchip-top">…h8r2kz5m0v…</div>
        <div class="tissue"><span class="code">already-superseded</span>not the live head — current head is k3m9q2x7aa · <button class="link">use head k3m9q2x7aa</button></div></div>
      <div class="add-t"><input class="in mono" placeholder="add target by short_id…"></div></div>
    <div><div class="lbl">Correcting record</div>
      <div class="prefill">Prefilled from newest predecessor <span class="mono">k3m9q2x7aa</span></div>
      <label class="fld"><span class="lbl">summary <span class="cnt mono">86/512 B</span></span><input class="in"><span class="ferr"></span></label>
      <label class="fld"><span class="lbl">content</span><textarea class="in" rows="5"></textarea><span class="ferr"></span></label>
      <div class="frow"><label class="fld">category <select class="in">…</select></label><label class="fld">scope <select class="in mono">…</select></label></div>
      <label class="fld"><span class="lbl">tags</span><input class="in"></label></div>
  </div>
  <div class="dlg-chain"><div class="lbl">Chain preview <span>live</span></div>
    <div class="chain-cols">
      <div class="ccol"><span class="ccol-h">d2</span><span class="cnode bad">…h8r2…</span></div><span class="carrow">→</span>
      <div class="ccol"><span class="ccol-h">d1</span><span class="cnode">…k3m9…</span></div><span class="carrow">→</span>
      <div class="ccol"><span class="ccol-h">head · d0</span><span class="cnode new">(new) …</span></div>
    </div></div>
  <div class="dlg-status"><!-- loading / err / warn(reauth) status block --></div>
  <div class="dlg-f"><span class="grow gate">Remove 1 invalid target — an invalid set rejects the whole call</span>
    <kbd>⌘↵</kbd><button class="btn">Cancel</button><button class="btn primary" disabled>Supersede 2 → 1</button></div>
</div>
```

## What to Avoid

- **Instant archive with only an undo toast (003 C).** It felt light, but for a bulk archive
  the toast is the only record of what changed, it disappears, and a failed optimistic write
  has to visibly put rows back. Keep the small confirm plus Undo on the result.
- **Inline supersede editor under the rows (003 C).** Too light for a change with no undo, and
  the breadcrumb-per-lineage chain repeats nodes and does not show "several predecessors, one
  head".
- **Side sheet with a two-step wizard (003 B).** Using the list as the target picker was neat,
  but step 1 / step 2 hid the correction while picking and the targets while writing; the
  vertical indented timeline was the hardest of the three to read at depth 2. Restore with no
  confirm in a banner was inconsistent with archive.
- **Submitting an invalid set and letting the server say no.** Validate per target inline and
  block submit; the server still has the final word.
- **Implying undo for supersede,** or labelling it "edit". Say "No undo" and how to change it.
- **Losing the draft on re-auth,** or retrying without the same idempotency key.
- **Distinguishing not-found from not-owned** in copy; the server deliberately does not.

## Origin

Synthesized from sketches: 003 (winner A).
Source files available in: `sources/003-curation-dialogs/index.html`.
