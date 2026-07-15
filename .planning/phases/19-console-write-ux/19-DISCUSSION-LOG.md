# Phase 19: Console Write UX - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-13
**Phase:** 19-console-write-ux
**Areas discussed:** Interaction model, Writable kinds & scope, Destructive & share confirms, Failure & re-auth UX

---

## Interaction Model

| Option | Description | Selected |
|--------|-------------|----------|
| Slide-over sheet | shadcn `sheet` create/edit form slides in from the side; list/detail stays visible; delete/visibility as inline row/detail actions | ✓ |
| Modal dialog | shadcn `dialog` centered overlay that blocks the list; simplest but loses context | |
| Dedicated form route | Full-page /memory/new + /memory/:id/edit; deep-linkable but a bigger nav change | |

**User's choice:** Slide-over sheet (recommended)
**Notes:** Reuses existing shadcn `sheet`; keeps operator context in a list+detail SPA. Inline row/detail actions for delete + visibility (D-02).

---

## Writable Kinds & Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Discovery create + delete + visibility | Discoveries creatable/deletable/re-shareable; citation/kind editing deferred (no update_discovery RPC). Memories get full CRUD + schedule | ✓ |
| Full discovery parity incl. edit | Discoveries also get edit; needs an update path for citations/kind/summary; larger, risks stretching the phase | |
| Memories only; discoveries deferred | Ship memory writes only; punt discoveries — but SC1 explicitly names discoveries | |

**User's choice:** Discovery create + delete + visibility (recommended)
**Notes:** Scheduling is memory-only (`schedule_memory`). Rules stay excluded by the memory contract (`store_rule` explicit-only, `set_visibility` rejected for rules).

---

## Destructive & Share Confirms

| Option | Description | Selected |
|--------|-------------|----------|
| Confirm dialog + share warning | Confirm dialog before delete; explicit inline warning on private→shared; no undo machinery | ✓ |
| Type-to-confirm delete | Delete requires typing the record short_id to arm; strongest guard, higher friction | |
| Optimistic + undo toast | Delete applies optimistically with an Undo toast; snappy but adds deferred-commit complexity + write-lane race | |

**User's choice:** Confirm dialog + share warning (recommended)
**Notes:** private→shared warning must state one-way exposure — readable by every authenticated caller, can't be narrowed back to hidden (D-07).

---

## Failure & Re-Auth UX

| Option | Description | Selected |
|--------|-------------|----------|
| Retry once + keep form open, resubmit | Transport interceptor retries once (server re-seals on retry); hard-fail keeps sheet open with inline re-auth + resubmit; optimistic rollback on error | ✓ |
| Same + draft persisted to storage | Also persist in-flight form to sessionStorage to survive a full /auth/login redirect; more robust, more moving parts | |
| Pessimistic (no optimistic update) | Spinner + refetch on success, no optimistic mutation; simpler, slightly less snappy | |

**User's choice:** Retry once + keep form open, resubmit (recommended)
**Notes:** Draft-to-storage deferred — keeping the form live covers the common re-seal case without a redirect (D-09). Optimistic list update rolls back on error (D-10).

---

## Claude's Discretion

- Primary "New memory" / "New discovery" entry-point placement (AppShell header vs per-scope).
- Create/edit form field set (content, scope, category, tags, visibility, optional summary, optional schedule window) + sensible defaults.
- Write/refetch mechanism (`@tanstack/svelte-query` `createMutation` + invalidate/`setQueryData`).
- CSRF attachment mechanism (Connect interceptor reading `engram_csrf` → `X-CSRF-Token` header).
- Tags/schedule input widgets (chip input, datetime pickers) from available shadcn primitives.

## Deferred Ideas

- Discovery edit UX (citations/kind/summary) — needs a new `update_discovery` RPC; own follow-up.
- Draft persistence across a full page redirect — deferred in favor of keeping the form live.
- Optimistic-undo for delete — rejected here in favor of pre-confirm dialog.
- Rule authoring from the console — permanently out (locked non-goal per memory contract).
