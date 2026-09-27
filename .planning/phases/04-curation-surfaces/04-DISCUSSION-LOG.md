# Phase 4: Curation Surfaces - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-27
**Phase:** 04-curation-surfaces
**Areas discussed:** Keys & multi-select, Archive ceremony & undo, Rules & Scheduled views, Re-auth resume behavior

---

## Todo cross-reference

| Option | Description | Selected |
|--------|-------------|----------|
| Fold in | Vet the three design/a11y skills with fable-security-review, install those that pass, use them for DSYS-03 | ✓ |
| Keep separate | Handle outside the phase | |

## Keys & multi-select

| Option | Description | Selected |
|--------|-------------|----------|
| ⇧S / a / ⇧A | Supersede gets a modifier; no shipped key changes | ✓ |
| S / a / r | Short, but r is easy to hit | |
| a toggles | One key for archive and restore | |

| Option | Description | Selected |
|--------|-------------|----------|
| x + checkbox + ⇧click | Linear-style; multiselectable listbox | ✓ |
| Checkbox column only | Mouse only | |
| ⇧j/⇧k extend + x | Vim-style | |

| Option | Description | Selected |
|--------|-------------|----------|
| Selection wins | Keys act on the selection; header becomes the bulk bar; Esc clears the selection first | ✓ |
| Active row always | Bulk actions only from bar buttons | |

| Option | Description | Selected |
|--------|-------------|----------|
| Clear on query change | Kept on Show more and post-write updates; not in the URL | ✓ |
| Persist across queries | Collect targets across searches | |

| Option | Description | Selected |
|--------|-------------|----------|
| Hover action buttons | Sketch 003 row actions count as the "row menu" | ✓ |
| ⋯ menu on the row | Dropdown per row | |

| Option | Description | Selected |
|--------|-------------|----------|
| Chain dialog + pane links | Sketch 003 chain dialog plus forward/back links | ✓ |
| Pane links only | No chain dialog | |

## Archive ceremony & undo

| Option | Description | Selected |
|--------|-------------|----------|
| Both | Confirm → result-body Undo → toast with Undo on Done | ✓ |
| Dialog Undo only | Reinterpret CUR-02's "toast" | |
| Toast only for 1 row | No confirm for a single row | |

| Option | Description | Selected |
|--------|-------------|----------|
| Same confirm | Restore uses the same small dialog | ✓ |
| No confirm | Restore runs immediately with a toast | |

| Option | Description | Selected |
|--------|-------------|----------|
| Stay dimmed until re-query | Patched in place, flashed, leaves at the next query | ✓ |
| Drop out immediately | Invalidate and re-fetch | |

## Rules & Scheduled views

| Option | Description | Selected |
|--------|-------------|----------|
| New /rules and /scheduled | Top-level routes reusing the shared list and pane | ✓ |
| Tabs inside /observe | Observe becomes a curation hub | |
| Facets on /search | No dedicated view | |

| Option | Description | Selected |
|--------|-------------|----------|
| All readable, grouped | Empty scopes = every rule:* scope, grouped by scope, delete only | ✓ |
| Scope picker first | One rule scope at a time | |

| Option | Description | Selected |
|--------|-------------|----------|
| Archive on expired only | State tabs, cross_spine, cursor, window column | ✓ |
| Archive on any row | Beyond CUR-04 | |

| Option | Description | Selected |
|--------|-------------|----------|
| Neither | No supersede from Rules or Scheduled | ✓ |
| Scheduled rows only | Supersede from the Scheduled view | |

**/observe:** Sean asked what `/observe` is for. I explained that it is the original scope-first
browser (sidebar + paginated ListMemories + create/edit) and that Phase 2 made `/search` cover
it. Options offered: retire with a redirect / keep as a scope browser / keep but defer.
**User's choice (free text):** "why even redirect? can we just remove it from the UI entirely?"
→ Remove entirely, with no redirect (CONTEXT D-14).

## Re-auth resume behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Reopen, wait for confirm | Draft restored with the same idempotency_key; supersede re-previews | ✓ |
| Auto-resubmit | Sketch's "Re-authenticate & retry" | |
| Auto for archive, confirm for supersede | Split by risk | |

| Option | Description | Selected |
|--------|-------------|----------|
| One kind per dialog | supersede / archive / delete kinds; /rules, /scheduled destinations; v2 envelope | ✓ |
| Only supersede + archive | Rules delete not resumable | |

## Claude's Discretion

- Dialog host composition, toast/flash timing, bulk-bar copy, the "show hidden" toggle and its
  key, empty states for /rules and /scheduled, the add-target lookup.
- DSYS-03 audit policy defaulted (Sean chose "ready for context"): fix every WCAG 2.2 AA failure;
  file the rest as GitHub issues.

## Deferred Ideas

- A scope-browsing surface could come back with Phase 5's browse work (not planned).
