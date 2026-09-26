---
sketch: 003
name: curation-dialogs
question: "How should supersede-with-chain and archive/restore feel — how much ceremony, how the chain is shown, and how reversal is offered?"
winner: "A"
tags: [curation, supersede, archive, restore, chain, dialogs]
---

# Sketch 003: Curation Dialogs

## Design Question

How should supersede-with-chain and archive/restore feel — how much ceremony, how the chain is shown, and how reversal is offered?

## How to View

`open .planning/sketches/003-curation-dialogs/index.html`

Use the "Next call returns" bar to choose success, validation error, server rejection or re-auth required for the next fake call. Each tab starts from the same seed data. "reset data" restores it.

## Variants

- **A: Modal dialog** — supersede opens a centred dialog: predecessor chips with inline per-target validation on the left, the correcting record prefilled from the newest predecessor on the right, and a live column chain preview at the bottom. Archive/Restore opens a small confirm dialog with an "Undo" on its result (shadcn-svelte Dialog).
- **B: Side sheet** — a right sheet keeps the list visible. In step 1 the list itself picks the targets; step 2 writes the correction. The chain is a vertical timeline. Archive confirms inside the sheet, and Restore sits in the archived banner with no confirm.
- **C: Inline + undo toast** — `e`/`u` or the row menu archive or restore instantly. The change is optimistic: a toast offers Undo (and `⌘Z`), and a failure puts the row back. Supersede expands an inline editor under the rows, with the chain as a breadcrumb per lineage path.

## What to Look For

1. **Ceremony vs. risk.** Supersede cannot be undone: it is additive history, and the only fix is to supersede again. Archive is always reversible. Check whether A's confirm on a reversible archive feels like friction next to C's undo toast, and whether C's inline supersede feels too light for a change with no undo.
2. **Chain legibility.** Load the two "Rerank telemetry" duplicates (`k3m9q2x7aa` already supersedes `h8r2kz5m0v`, so the result has depth 2). Compare A's columns (oldest left to head), B's indented timeline and C's breadcrumbs. Which one best shows "several predecessors, one head"?
3. **Invalid targets.** Add `p1xw8r5jbk` (shared by priya), `st74vdk0gh` (a rule) or `h8r2kz5m0v` (not the live head, with a "use head" swap). Validation is inline per target, and submit stays blocked because an invalid set rejects the whole call. Look at where each variant puts the reason.
4. **After success.** Superseded records drop out of recall. Compare how "View superseded" brings them back (show-hidden plus a flash, the new record's detail, or a result strip) and how clearly each variant says "fetchable by id" (click any chain node).
5. **Failure honesty.** Server rejections use the real engram shapes: a sentinel `target is already superseded: …` or `not found: …` for targets, and `field=summary hint=too_long` for fields. Re-auth keeps the draft and retries. In C, a failed optimistic archive must visibly put the row back.
