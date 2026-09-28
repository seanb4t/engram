# Pitfalls Research

**Domain:** Recall-first search UX + curation workbench + related-memories graph, added to an existing
SvelteKit 2 / Svelte 5 / shadcn-svelte / bits-ui / Tailwind 4 / TanStack Query 6 operator console
vendored as a static SPA into a Go binary, over a ConnectRPC API backed by Qdrant with
store-layer-enforced authz (Cedar) and provider-neutral, advisory-only typed decisions (Jev).
**Researched:** 2026-09-25
**Confidence:** HIGH for engram-specific/repo-grounded findings (direct file:line evidence); MEDIUM
for external library specifics not confirmed against first-party version-pinned docs (flagged inline)

## Critical Pitfalls

### Pitfall 1: The command palette lies — client-side filtering masquerading as search

**What goes wrong:**
The palette shows "no matches" for a query the store actually has thousands of hits for, because
it is filtering a small static list of menu items against the typed text instead of asking the
server anything.

**Why it happens:**
bits-ui's `Command.Root` defaults `shouldFilter` to `true` (confirmed against bits-ui docs,
2026-09-25) — every `Command.Item` is scored against the input text with `computeCommandScore`
and hidden if it doesn't match. `CommandPalette.svelte` never sets `shouldFilter`, so it inherits
`true`. The single "Search memories for …" item's label is computed once at mount
(`Search memories for "{q}"` snapshots `q` while it's still `''`), so with any typed text the
label no longer scores against the input and the item itself gets filtered out — the exact live
bug logged in `notes/console-overhaul-exploration.md` ("Palette says 'no matches' for `github`").
This is a general bits-ui/cmdk trap, not engram-specific: any `Command.Root` wrapping
server-driven or dynamically-labelled items needs filtering disabled explicitly.

**How to avoid:**
Set `Command.Root shouldFilter={false}` on every palette/combobox that drives results from a
server call, and feed `Command.List` from the live `SearchMemories`/`ListTags`/`ListScopes`
response, never from a static item array scored against the query text. Any item whose visible
label depends on reactive state (`q`) must be a `$derived`, not a value read once at mount.

**Warning signs:**
- A palette/combobox "shows no results" for a term that manual `search_memory` calls confirm the
  store has.
- `Command.Empty` renders while a network request for the same term is still in flight or was
  never fired (check devtools Network tab, not just the UI).
- Any `Command.Item` whose `onSelect`/label references a variable captured before `bind:value`
  updates it.

**Phase to address:**
Recall-first search phase (first phase per the milestone's stated build order) — this is the bug
that motivated the milestone; a regression test that types text unlikely to match any static
label and asserts a real Connect call fired is the first test to write.

---

### Pitfall 2: Search-box race — a stale response overwrites a newer query

**What goes wrong:**
User types `git`, then `github`; the `git` response (broader, arrives late) lands after the
`github` response and overwrites it, so the visible results don't match the visible input.

**Why it happens:**
`createQuery` keys must encode every input that changes the response, and in-flight requests for
an abandoned key must be cancelled — neither happens automatically. `search/+page.svelte` already
does the query-key part correctly (`queryKey: ['searchMemories', query, scope]` at
`ui/src/routes/search/+page.svelte:18` — the raw text is IN the key, so a new keystroke is a new
cache entry, not a mutation of the old one), but it does **not** thread an `AbortSignal` into
`engram.searchMemories`. TanStack Query passes each `queryFn` an `AbortSignal` that fires when the
query becomes stale/inactive; connect-web's generated client methods accept `signal` in their
`CallOptions` second argument. Without wiring it through, an abandoned request for `git` keeps
running server-side and its promise can still resolve and (depending on cache/key nuances, e.g. a
debounced key that coalesces near-identical text) clobber fresher state. This gets materially more
dangerous once the palette/entry point starts debouncing free text server-side (Pitfall 1's fix) —
debounced input means more overlapping in-flight requests, not fewer.

**How to avoid:**
`queryFn: ({ signal }) => engram.searchMemories(req, { signal })` on every server-driven
search/facet/autocomplete query. Keep the full query text (and every filter chip, facet selection,
`cross_spine` flag) in the query key — never key on a debounced/throttled proxy that can alias two
different inputs to the same cache entry.

**Warning signs:**
- Results visibly "flicker back" to an earlier state after the input has moved on.
- Network tab shows overlapping in-flight requests for the same query family with no cancellation.
- A `queryFn` that ignores its `{ signal }` argument entirely.

**Phase to address:**
Recall-first search phase — write the race test (fire two overlapping queries, resolve the first
after the second, assert final rendered state matches the second) before building facets/palette
on top of the same query function.

---

### Pitfall 3: Flash-to-empty and "no results" shown while still loading

**What goes wrong:**
Between keystrokes the result list flashes to an empty/"no matches" state before the new results
arrive, reading as "nothing found" for a beat even though the server hasn't answered yet — the
opposite of the milestone's stated honesty rule ("report honestly what it searched").

**Why it happens:**
Default `createQuery` behavior treats every new query key as a fresh fetch: `data` is `undefined`
and `isLoading` is `true` until the first result for *that specific key* arrives, so a naive
`{#if data.memories.length === 0}` empty-state check fires on every keystroke. TanStack Query v5
retired `keepPreviousData`/`isPreviousData` in favor of `placeholderData` accepting an identity
function (confirmed via TanStack's v5 migration guide and GitHub discussion #6460) —
`placeholderData: keepPreviousData` (importing the `keepPreviousData` helper) keeps the prior
page's data visible while the new one loads, with `isPlaceholderData` distinguishing "this is
stale" from "this is fresh." **v6-specific confirmation gap (MEDIUM confidence):** engram's stack
is already on the v6 Svelte adapter (`@tanstack/svelte-query: ^6.1.34`), which the official
migrate-from-v5-to-v6 guide describes as dropping stores in favor of a rune-returning thunk
(`createQuery(() => ({...}))`, already the pattern in `search/+page.svelte:15-19`); the guide does
not document a further rename of `placeholderData`, so treat it as carried forward but verify
against the pinned 6.1.34 changelog before relying on it.

**How to avoid:**
`placeholderData: keepPreviousData` on the search query; render an explicit "searching…" state
keyed off `isFetching && !isPlaceholderData` (not off `data` presence) distinct from the honest
"no memories match `X` in any scope you can read" empty state, which must only render once
`isFetching` is false. Never let `memories.length === 0` alone decide the empty state.

**Warning signs:**
- Visual flash of "no results" on every keystroke in manual testing.
- An empty state that renders identically whether the query errored, is loading, or genuinely
  found nothing (the honesty rule requires these to say different things).

**Phase to address:**
Recall-first search phase — this is the same phase as Pitfall 1/2; the "truthful result feedback"
requirement in `PROJECT.md`'s target-features list depends on getting all three (filtering,
races, flash) right together.

---

### Pitfall 4: Focus loss and a mixed keyboard model on hover-expand rows

**What goes wrong:**
`j`/`k` traversal moves the "selected" row visually, but real DOM focus is elsewhere (or nowhere),
so screen readers announce nothing, browser find-in-page/extension shortcuts fight the app, and
hovering a *different* row than the keyboard-selected one to read its expanded content silently
steals the "current" row out from under the keyboard user.

**Why it happens:**
Two different WAI-ARIA APG-endorsed patterns solve "select one of many rows with the keyboard,"
and mixing them is the classic trap: **roving tabindex** (DOM focus physically moves row to row,
each row is a real tab stop) vs **`aria-activedescendant`** (DOM focus stays on one container
element; the container's `aria-activedescendant` attribute points at the id of the "active" row,
which gets a visual highlight only). engram's existing `MemoryRow.svelte` has no `tabindex`/focus
management at all today — selection is driven entirely by `?sel=` in the URL and mouse `onclick`
(`ui/src/lib/components/MemoryRow.svelte:62-65`), so `j`/`k` traversal is new keyboard-model
surface, not a retrofit. Layering **hover**-driven expansion on top of either model is the second
trap: if hover changes which row is "active" (as MemoryRow's `group-hover:opacity-100` action menu
already hints at, line 82), a mouse resting on row 12 while `j`/`k` has logically selected row 3
creates two different "current rows" simultaneously — assistive tech and the visible highlight can
disagree about which one is which.

**How to avoid:**
Follow WAI-ARIA APG's Listbox pattern (confirmed via exploration note, sourced from
w3.org/WAI/ARIA/apg/patterns/listbox): keep real DOM focus on the list container, drive
`aria-activedescendant` from `j`/`k` state only, and never let a `mouseenter`/`mouseleave`
hover-expand handler mutate the same "active id" state `j`/`k` writes — hover expansion should be
a purely visual/local affordance (e.g. a `:hover`/`focus-within` CSS reveal, matching the existing
`opacity-0 group-hover:opacity-100` pattern already used for the row action menu) that never
touches keyboard-selection state. Detail-pane content should follow keyboard selection, not mouse
position.

**Warning signs:**
- Tab key moves focus through 30 individual row elements (roving tabindex leaking into a listbox
  that should use activedescendant), or focus disappears entirely after a `j`/`k` press.
- Moving the mouse over a different row than the one `j`/`k` selected changes what the detail pane
  shows.
- A screen reader announces nothing when `j`/`k` is pressed.

**Phase to address:**
Recall-first search phase — keyboard traversal and hover-expand are both target features of the
same phase per `PROJECT.md`; write the activedescendant-vs-hover independence as an explicit
assertion, not just a visual check.

---

### Pitfall 5: Variable-height hover-expanded rows break virtualization

**What goes wrong:**
A virtualized list of dense rows that expand to a taller size on hover/focus either mis-measures
scroll position (jumping, overlapping rows, blank gaps) or silently renders nothing at all on
first paint under Svelte 5.

**Why it happens:**
Two independent problems compound. First, hover-expand rows are variable-height by definition (a
collapsed row is one line; an expanded row shows content preview + full tag set), and virtualizers
that assume fixed row height, or that measure height once and cache it, will mis-position rows the
instant a hover toggles a neighbor's height — the expanding row is exactly the one whose
measurement the virtualizer must re-run, not skip. Second, and more acute for this stack:
`@tanstack/svelte-virtual` (3.13.39, already a documented candidate in
`notes/console-overhaul-exploration.md`) has an **open, unresolved GitHub issue for Svelte 5**
(TanStack/virtual#866, opened 2024-10-28, confirmed still open 2026-09-25): the virtualizer loses
track of the initial scroll-element binding under runes mode and renders an empty list on first
load; the documented workaround is manually tracking a `mounted` boolean and calling
`$virtualizer._willUpdate()` — a private API, not a supported integration path. This is **not**
a turnkey dependency add for this stack today.

**How to avoid:**
Decide virtualization need *before* adopting `@tanstack/svelte-virtual`: with "~30 rows on screen"
as the stated dense-row target (`notes/console-overhaul-exploration.md`), a bounded page of ~30-50
rows may not need virtualization at all if pagination/cursor limits keep the DOM list short — check
this against the actual `k`/limit ceiling before reaching for a library carrying an open Svelte-5
blocker. If virtualization is needed (e.g. an unbounded browse view later), either (a) pin to a
version/commit with the `_willUpdate()` workaround applied and covered by a regression test proving
first-paint renders rows, or (b) measure real row heights via `ResizeObserver` per row and feed
them to `estimateSize`, re-measuring on hover/expand toggle rather than caching a static estimate.

**Warning signs:**
- Blank list on first load that only appears after a scroll event or window resize.
- Rows overlapping or leaving gaps immediately after a hover expands/collapses a neighbor.
- Any private/underscore-prefixed virtualizer method (`_willUpdate`, etc.) appearing in app code —
  a sign the public API isn't sufficient yet for this Svelte version.

**Phase to address:**
Recall-first search phase, as a design decision gate before implementation ("do we virtualize at
all, given the ~30-row target") — do not silently add the dependency as a follow-on to a later
phase without re-confirming issue #866's status at implementation time.

---

### Pitfall 6: A new Connect RPC enforces authz in the handler instead of the store

**What goes wrong:**
`SupersedeMemory`/`ArchiveMemory`/`RestoreMemory`/`RelatedMemories` get a quick permission check
in `internal/server`'s handler function (e.g. "if owner != caller, reject") that looks correct in
every test written against it, but diverges from every other read/write path's enforcement and
eventually admits a bypass some other caller (MCP, CLI, a future RPC) doesn't share.

**Why it happens:**
This is a **locked** architectural invariant, not a style preference: DEC-cgb
(`docs/adr/engram-cgb-*.md`, `.planning/PROJECT.md:738,759-764`) requires per-actor authorization
to be enforced *inside* `internal/store` via Qdrant read filters and owner-gate primitives, "never
in handlers" — precisely because handler-level checks drift out of sync across surfaces (MCP tool,
Connect RPC, CLI) that all need the same guarantee. `RelatedMemories` is explicitly scoped to reuse
`qdrant.NewQueryID` sub-queries already living in `internal/store/spine.go:640`, and
`SupersedeMemory`/archive/restore are explicitly scoped to reuse existing MCP `supersede_memory`
and `spine-review archive/restore` **store** paths (`notes/console-overhaul-exploration.md`'s API
gap table) — the intended shape already routes through the store; the pitfall is a well-meaning
"quick guard" added in `connectapi.go` on top of that, which becomes a second, divergent
enforcement point.

**How to avoid:**
Every new Connect handler should be a thin `protoconv`-shaped translation that calls the *same*
store method the corresponding MCP tool calls, with authorization living entirely inside that
store method (owner/visibility gates, DEC-xa6's not-found-for-unauthorized rule). Code review gate:
grep the new handler for any `if` referencing `caller`/`owner`/`visibility` that isn't just passing
those values into a store call — that's the smell.

**Warning signs:**
- A Connect handler contains its own owner/visibility comparison instead of delegating entirely to
  `internal/store`.
- The MCP tool and the new Connect RPC for the same operation return different errors (one 404,
  one 403) for the same unauthorized-id case — DEC-xa6 requires both to be indistinguishable
  not-found.
- A test exists for the Connect RPC's authz behavior that does **not** also exist, byte-identical
  in intent, for the underlying store method.

**Phase to address:**
Curation RPCs phase (new Connect write/read RPCs) — gate this with the existing
`internal/server/connectapi_write_parity_test.go` pattern (see Pitfall 11) extended to cover the
new RPCs, plus a targeted unauthorized-id-returns-404 test per new RPC.

---

### Pitfall 7: A new mutating RPC ships CSRF-unprotected (or unusable) because the allowlist is two-sided

**What goes wrong:**
Either (a) the new RPC (`SupersedeMemory`, `ArchiveMemory`, `RestoreMemory`) is wired through the
UI's default `engram` client and every call fails with a confusing `Unauthenticated`/network error
because it's missing the session-bound CSRF header, or (b) it's wired correctly on the client but
the server never enforces CSRF on it, silently shipping a same-origin forgeable mutation.

**Why it happens:**
This repo's CSRF contract is split across **two independent allowlists that must be updated
together**, and neither failure mode is obviously connected to "I forgot to register a new RPC":
- **Client side:** `ui/src/lib/client.ts` exports two separate Connect clients — `engram` (read,
  plain transport, `ui/src/lib/client.ts:12-14`) and `engramWrite` (write-only,
  `[retryOnce, attachCsrf]` interceptors, `ui/src/lib/client.ts:16-24`). A new mutating call made
  through `engram` instead of `engramWrite` never attaches the `X-CSRF-Token` header at all.
- **Server side:** `internal/server/connectcsrf.go:33`'s `csrfWriteProcedures` map is a hardcoded,
  by-name allowlist of exactly the write Procedures the CSRF interceptor checks
  (`if !csrfWriteProcedures[req.Spec().Procedure] { ...skip... }`, line 62) — every other
  Procedure, including any new one, is CSRF-**exempt by default** unless explicitly added. Silence
  here is the dangerous direction: a forgotten entry doesn't fail loudly, it just leaves the new
  mutation forgeable.

**How to avoid:**
Treat "add a mutating Connect RPC" as a two-line checklist, not one: (1) client call goes through
`engramWrite`, never `engram`; (2) the new Procedure constant is added to
`csrfWriteProcedures` in `connectcsrf.go`. Write a test in the spirit of
`internal/server/connectcsrf_test.go` that asserts the new procedure is present in the map (a
positive assertion, not just "existing tests still pass" — the map's whole failure mode is a
*missing* entry that no existing test would catch).

**Warning signs:**
- A new mutation "works in dev" (same browser session, cookies already fresh) but the manual
  double-submit contract was never exercised because `engram` (not `engramWrite`) happened to
  still succeed on a GET-shaped read-only test harness.
- `csrfWriteProcedures` in `connectcsrf.go` has not grown a new entry in the same PR that added a
  new `rpc … returns (…)` write method to `engram.proto`.

**Phase to address:**
Curation RPCs phase — this is the single highest-leverage first test to write in that phase, before
any UI is built against the new RPCs: prove every new mutating Procedure is both CSRF-registered
server-side and routed through `engramWrite` client-side.

---

### Pitfall 8: The related-memories graph is decorative, not useful

**What goes wrong:**
A force-directed node-link diagram renders on the detail pane, looks impressive in a screenshot,
and is immediately useless: it's a hairball at any real record count, never settles (nodes drift
forever), has no click-to-focus/expand-neighbor interaction, is canvas-rendered with zero keyboard
or screen-reader path, and its edge/node colors are unreadable in dark mode.

**Why it happens:**
The milestone explicitly wants "a visual (non-text) view of related memories" using edge types the
store already knows — supersession chain, shared tags, shared citations, vector neighbours
(`notes/console-overhaul-exploration.md`) — but a naive implementation (drop all edges from
`RelatedMemories(id)` into a generic force-layout library with default settings) produces exactly
the decorative failure mode: every record sharing a tag becomes an edge, so a popular tag creates a
hub with dozens of crossing lines; force simulations without a `alphaDecay`/settle threshold or a
fixed iteration budget never visually stabilize; and force-layout libraries are near-universally
canvas or SVG-transform based with no semantic DOM structure, so they're invisible to
assistive tech by default.

**How to avoid:**
Bound the graph deliberately: cap edges per node (e.g. top-N by Jev's advisory `same_subject`
score where available, else recency/score), distinguish edge *types* visually (supersession vs
shared-tag vs vector-neighbour must not look identical), give the layout a fixed settle
budget/iteration cap so it visibly stops, and require every node to be reachable and actionable via
keyboard (Tab/arrow to a node, Enter to focus/expand it, matching the same activedescendant
discipline as Pitfall 4) with an off-canvas or `aria-live` textual equivalent ("supersedes 2
records, shares tags with 5") for anyone not using the pointer. Theme every edge/node color through
the same CSS custom-property mechanism `MemoryRow.svelte` already uses for category color
(`style="--c:var(--cat-{memory.category})"`, line 58) so dark mode isn't a second unthemed pass.

**Warning signs:**
- The graph has no interaction beyond pan/zoom — clicking a node does nothing.
- Node/edge count grows unbounded with the record's tag popularity rather than being capped.
- The graph is unusable/untestable via `agent-browser`/keyboard-only exploration.
- Dark mode toggling changes the page chrome but not the graph's canvas colors.

**Phase to address:**
Graph/discovery phase (per the milestone's stated third build priority — "developer recall first,
operator curation second, newcomer browsing third," and the graph is explicitly "beyond search").
Ship the edge-capping and keyboard-equivalence decisions in the phase's design/UI-spec step, before
any rendering library is chosen — a library choice made before these constraints are set tends to
lock in the hairball.

---

### Pitfall 9: Advisory NL query understanding quietly stops being advisory

**What goes wrong:**
A natural-language query gets parsed into filter chips that are applied to the actual search
*before* the user sees or confirms them (silent auto-apply), the parse step adds enough latency
that the search itself feels broken, or the raw query text ends up in a log/trace/telemetry
attribute — any of which contradicts a standing, already-enforced project rule for exactly this
class of feature.

**Why it happens:**
The milestone is explicit that NL query understanding must render as "removable, user-confirmed
filter chips — provider-neutral, off by default, advisory only," matching the same standing rule
that already governs Jev typed decisions (`.planning/PROJECT.md:759-781` and memory `rwtzp3m7y8`:
a decision/verdict never mutates or auto-applies, and a reranker failure never fails a search). The
repo already has the concrete precedent to mirror and the concrete failure to avoid re-introducing:
`ENGRAM_SEARCH_RERANK_AUDIT` (`internal/config/registry.go:140`, `internal/server/decider.go:168`)
is an **opt-in, off-by-default** audit flag that logs query text plus candidate ids —
**explicitly never content** — specifically so query-text logging stays an operator's deliberate
choice, not a default. The `internal/decide` package also already enforces a bounded latency
contract for this class of call: `DecideMany` runs through a bounded worker pool
(`internal/decide/many.go:21`) under an explicit `ErrDecisionTimeout` (`internal/decide/errors.go:34`)
and the search reranker's own budget is a documented "2s no-retry" ceiling — an NL-query-understanding
call sitting in front of search needs the identical shape (bounded, timed-out, never blocking the
underlying search on failure) or it becomes the slowest part of every search.

**How to avoid:**
Reuse, don't reinvent: gate query-text logging behind an explicit, off-by-default flag in the same
family as `ENGRAM_SEARCH_RERANK_AUDIT` (never log content, ever); emit a `decide`-shaped span with
status/cost/latency attributes and no raw text, mirroring `internal/decide/jev/jev.go:230-277`;
render inferred filters as inert chips the user must click to apply — the underlying
`SearchMemories` call must be unaffected until that click; and put a hard client-side timeout on
the NL-parse call so a slow/unavailable provider degrades to "no chips suggested," never a blocked
search.

**Warning signs:**
- A filter chip changes the visible result set before the user interacted with it.
- Query text appears in any log line, span attribute, or telemetry payload with no accompanying
  audit-flag gate.
- Typing a query feels slower with NL understanding enabled than with it disabled, because the
  search request itself is waiting on the parse step rather than running in parallel.

**Phase to address:**
Query-understanding phase (the milestone's stated last-priority, advisory-only capability) — but
the *logging* and *latency-budget* conventions it must follow should be written down as house rules
(in the planned `engram-console-conventions` skill) during the recall-first phase, since
`ENGRAM_SEARCH_RERANK_AUDIT` and `internal/decide`'s bounded-pool pattern already exist and should
be referenced, not rediscovered, when this phase starts.

---

### Pitfall 10: New curation surfaces silently fall outside the re-auth resume envelope

**What goes wrong:**
A user editing a rule, or mid-supersede on a curation dialog, gets bounced to `/auth/login` (401)
and back — and their in-progress edit is gone, with no error, because the resume mechanism quietly
declined to restore it.

**Why it happens:**
`ui/src/lib/resume.ts` implements a single, deliberately narrow resume envelope for the D-09
re-auth flow: `ALLOWED_DESTINATIONS = ['/observe', '/search', '/discovery']`
(`ui/src/lib/resume.ts:52`) and `kind: 'memory' | 'discovery'`
(`ui/src/lib/resume.ts:32`) are both closed unions, and `isAllowedDestination` rejects anything not
matching (an intentional open-redirect defense, per the surrounding comment). Both are exhaustive
by construction — exactly the "MUST NOT invent structure" hazard in reverse: this is a
tool-*adjacent* app-owned contract, and extending it correctly means widening the union, not
routing around it. A new curation surface (rules editor, scheduled-memory editor, a
supersede-with-history-chain dialog) that persists a resume draft with a `kind`/`returnPath` not in
these unions will have `persistResume` succeed silently (it only stamps `v`/`ts`, it doesn't
validate against the allowlist) but `peekResume`/`isAllowedDestination` on the other end will
reject it just as silently — the form's draft is gone with zero user-visible signal, and no test
written against the *existing* two kinds would catch a regression in a *new* one.

**How to avoid:**
Any new write surface (rule editor, scheduled-memory editor, supersede dialog) that can trigger a
re-auth mid-edit must (1) add its route to `ALLOWED_DESTINATIONS`, (2) extend the `kind` union in
`ResumeEnvelope`/`ResumeDraft`, and (3) have the route/host that calls `peekResume`/`consumeResume`
(currently only `search`/`observe`/`discovery` pages, per `resume.ts`'s doc comment on ownership)
extended to handle the new `kind`. Write the resume round-trip test for the new kind explicitly —
don't assume the existing memory/discovery tests generalize.

**Warning signs:**
- A curation form's draft disappears after a session-expiry redirect with no toast/error.
- `persistResume` is called with a `kind` or `returnPath` that isn't yet in the two unions above (a
  TypeScript error should catch this at compile time if the unions are widened correctly — if it
  doesn't, the union wasn't actually widened, just the call site was cast around it).

**Phase to address:**
Curation surfaces phase (rules, scheduled, supersede/archive/restore UI) — treat "does this form's
resume round-trip survive a re-auth" as a UAT check for every new write surface in that phase, not
just the two that already exist.

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|-----------------|-----------------|
| Using `$effect` to compute a row's derived display value instead of `$derived` | Feels natural coming from `onMount`/lifecycle habits | Svelte 5's documented `$effect`-updates-state-it-reads infinite-loop trap, plus effects don't memoize the way `$derived` does across ~30+ list rows | Never for pure derivations (`memoryStateWords`, dim flags, tag overflow) — `MemoryRow.svelte` already does this correctly with `$derived` (lines 37-54); new row-level computed values must follow the same pattern |
| Skipping `AbortSignal` wiring on a new query "since it's fast in dev" | Ships faster | Race conditions (Pitfall 2) only surface under real network latency/jitter, so they pass local dev and fail in CI/prod intermittently | Never on a text-search or facet query; acceptable only for a query that can never re-fire with a different key while a prior call is outstanding (rare) |
| Treating Qdrant Facet's default `exact:false` approximate counts as ground truth in a UI label | Cheap, fast facet counts | Approximate counts can visibly disagree with the actual filtered result count, undermining "truthful result feedback" | Acceptable for a tag cloud's relative sizing; not acceptable for a facet count rendered next to an exact "N of M" total |
| A quick per-handler authz check "just to be safe" alongside the store-layer gate | Feels like defense-in-depth | Two enforcement points drift (Pitfall 6); the DEC-cgb invariant exists precisely to prevent this pattern | Never — the ADR is locked precedence-0 |

## Integration Gotchas

Common mistakes when connecting to external services/libraries newly pulled into this milestone.

| Integration | Common Mistake | Correct Approach |
|-------------|-----------------|-------------------|
| Qdrant Facet API (`ListTags`) | Assuming facet counts ignore the active search/scope filter | Facet requests accept a `filter` field and the computation respects it (confirmed via Qdrant API reference, 2026-09-25) — always pass the same authz+scope filter the search itself uses, or tag counts will include records the caller can't even read |
| Qdrant Facet API | Requesting facets on an unindexed/non-keyword payload field | Facet counting only works on fields supporting match conditions (keyword-indexed); `tags` and `category` need a payload index before `ListTags` can facet on them — verify the index exists rather than assuming |
| buf breaking (`buf.yaml`: `breaking.use: [FILE]`) | Reusing a proto field number, or adding a field without checking the *whole file's* number history, assuming per-message isolation | `FILE`-level breaking detection catches cross-message renumbering too; always append new fields at the next unused number and let `task proto:lint`/`buf breaking` gate the PR — don't hand-pick a number that "looks free" |
| MCP↔Connect parity (`internal/server/connectapi_write_parity_test.go`) | Adding a new Connect write RPC (`SupersedeMemory`, `ArchiveMemory`, `RestoreMemory`) without a corresponding parity-table row | This repo already has an explicit MCP-vs-Connect-vs-direct-store parity harness (`newParityLane`, `parityMCPCaller`) proving the two lanes agree; a new write RPC needs a new row, not just handler-level tests |
| Vendored-SPA drift gate (`ui-drift` CI job, `.github/workflows/ci.yaml:302`) | Editing `ui/src` and forgetting the built SPA under `internal/webauth/static/` is a **committed, CI-diffed artifact** | `pnpm build` output must be regenerated and committed alongside any `ui/src` change touching new routes/RPC calls, or CI fails on drift (self-heals only on Renovate branches, not normal feature work) |
| `gen/ts` regeneration (`task surfaces:gen` / `interface-surface drift` CI step) | Hand-editing generated TS types in `ui/src/lib/gen` after adding a new RPC to `engram.proto` | The CI drift step does `go tool buf generate` then `cp -R gen/ts/. ui/src/lib/gen/` — any manual edit under `ui/src/lib/gen` is guaranteed to be overwritten/flagged; regenerate via `task proto:gen`/`task surfaces:gen`, never hand-patch |
| TanStack Query v6 Svelte adapter | Assuming v5 React-ecosystem docs/blog posts apply verbatim | v6's Svelte adapter dropped the store-based API in favor of runes (`createQuery(() => ({...}))`, no `$` prefix needed) — already the pattern in `search/+page.svelte:15`; verify any v5-sourced snippet (including `placeholderData`/`keepPreviousData` specifics) against the pinned `6.1.34` behavior before trusting it |

## Performance Traps

Patterns that work at small scale but fail as the console's usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|-----------------|
| Unvirtualized dense list rendering every hit | Fine at ~30 rows; scroll jank and high Interaction to Next Paint (INP) once a browse view or a wide `k`/limit returns hundreds | Confirm the actual row-count ceiling for each view before deciding whether to virtualize at all (see Pitfall 5); if unbounded, virtualize deliberately rather than letting a "just show everything" list grow unnoticed | Beyond roughly 100-200 simultaneously-mounted interactive rows (each with hover listeners, dropdown menus, badges) |
| Force-directed graph layout with no settle/iteration cap | Looks fine with 5-10 nodes; CPU-pegs and never stops animating once a popular tag's related-memories set grows | Cap edges per node and set a fixed simulation iteration budget (Pitfall 8) | Any node with more than a handful of shared-tag/vector-neighbour edges |
| `exact:true` Qdrant facet counts on a large collection | Fast in a dev seed dataset; slow/blocking on a production-sized collection | Default to `exact:false` (approximate) for UI-facing tag clouds/facets; reserve `exact:true` for an explicit operator "recount" action, not the default render path | Collections large enough that exact per-value counting becomes a full scan-shaped cost |
| Hover listeners on every dense row (expand-on-hover) | Smooth with 30 rows; jank with hundreds if virtualization is added later without also scoping listeners to visible rows | Attach hover/expand listeners only to mounted (visible) rows — virtualization should already guarantee this, but a "quick fix" that renders all rows off-screen to avoid Pitfall 5 defeats it | Same threshold as the unvirtualized-list trap above |

## Security Mistakes

Domain-specific security issues beyond general web security, tied to this milestone's new surfaces.

| Mistake | Risk | Prevention |
|---------|------|------------|
| Authz check duplicated in a Connect handler instead of solely in `internal/store` | Enforcement drifts across MCP/Connect/CLI surfaces over time; a future change to the store's owner gate silently stops covering the handler's parallel check | DEC-cgb: store-layer-only enforcement (Pitfall 6) |
| New mutating RPC missing from `csrfWriteProcedures` | Same-origin forgeable mutation (no CSRF protection) on a write endpoint, shipped silently — no error, no test failure, just an unprotected RPC | Positive-assertion test that every write RPC name is present in the allowlist (Pitfall 7) |
| NL query text logged without the existing audit-flag gate | Query text can carry sensitive operator intent/content fragments; the project's own convention (`ENGRAM_SEARCH_RERANK_AUDIT`) already treats this as opt-in-only, never-content | Mirror the existing audit-flag pattern exactly; never add a second, ungated logging path for query text (Pitfall 9) |
| Resume envelope's `returnPath`/`kind` unions widened carelessly for a new curation surface | `isAllowedDestination`'s open-redirect defense (`resume.ts:64-70`) only protects the destinations it enumerates; widening it to a route pattern instead of exact routes could reopen the same class of issue it was built to close | Extend the allowlist with exact new routes, not a wildcard/pattern match (Pitfall 10) |

## UX Pitfalls

Common user experience mistakes specific to this console overhaul.

| Pitfall | User Impact | Better Approach |
|---------|-------------|-------------------|
| Losing URL-as-state when adding new panels (facets, related-memories graph, tag cloud) | Back/forward navigation and shareable/bookmarkable search state breaks; a refresh loses the user's current filter/selection | Every new filter/facet/selection state should be a URL search param, following the existing `?q=`/`?scope=`/`?sel=`/`observeSearch`/`parseObserveParams` pattern (`ui/src/lib/queries.ts:18-44`) rather than component-local `$state` |
| New nested panes omitting `min-h-0` somewhere in the flex chain | The pane's inner content scrolls the whole page instead of just itself — jarring, breaks the fixed app shell | The existing chain is `AppShell.svelte:22` (`h-dvh`) → route's outer `flex h-full min-h-0` (`search/+page.svelte:41`) → `Resizable.Pane ... min-h-0` → any nested `Tabs.Content`/`ScrollArea` (`MemoryDetail.svelte:105,129`); every new nested scrollable region (graph pane, tag cloud, facet sidebar) must repeat `min-h-0` at each flex ancestor, not just the outermost one |
| Badge/state-word text is lowercase in the DOM but visually uppercase via CSS | Confuses both accessibility tooling (screen readers may or may not apply text-transform to speech) and test authors who assert on visible-looking text | `MemoryRow.svelte:75` already does this (`class="text-[10px] uppercase"` wrapping literal `{word}`, e.g. `archived`) — DOM/testing-library queries must match the lowercase source text (`archived`), not the rendered `ARCHIVED`; any new badge/graph-label copying this `uppercase` utility class inherits the same trap |
| `opacity-60` dim-iff-past treatment applied to text that also needs to pass contrast | A dimmed past-state row (archived/superseded/expired) can drop below WCAG contrast minimums, especially stacked with muted-foreground colors already in use | `MemoryRow.svelte`'s existing carve-out (badges stay full opacity even inside a dimmed row, `dimCls` applied per-element not to a shared ancestor, lines 48-54) is the model to extend — any new dimmable UI (graph nodes for archived records, tag-cloud entries) needs the same per-element opacity discipline and a contrast check against the actual rendered color, not just the un-dimmed one |

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces for this milestone.

- [ ] **Command palette "search works":** Often missing a real server call — verify by typing a
  term with zero matching static menu-item labels and confirming a `SearchMemories` network
  request fires (Pitfall 1).
- [ ] **Keyboard `j`/`k` traversal "works":** Often missing activedescendant/focus discipline —
  verify with a screen reader or accessibility tree inspector that the announced "current" row
  matches the visually highlighted one after both a keypress and an unrelated mouse hover
  elsewhere (Pitfall 4).
- [ ] **New mutating RPC "works" (manual click-through in dev):** Often missing the server-side
  CSRF allowlist entry — verify by checking `csrfWriteProcedures` contains the new Procedure name,
  not just that a browser session with fresh cookies succeeded (Pitfall 7).
- [ ] **Facet/tag counts "look right":** Often missing the active filter — verify counts change
  when a scope/category filter is applied, not just when nothing is filtered (Integration
  Gotchas: Qdrant Facet).
- [ ] **NL query chips "are helpful":** Often missing the confirm-before-apply gate — verify the
  underlying search results are unchanged until a suggested chip is explicitly clicked (Pitfall 9).
- [ ] **Related-memories graph "renders":** Often missing any keyboard or screen-reader path —
  verify every node is reachable via Tab/arrow keys and has a text equivalent, not just that it
  paints on canvas (Pitfall 8).
- [ ] **Curation form "handles re-auth":** Often missing from the resume envelope's unions —
  verify a session-expiry redirect mid-edit on the *new* form restores the draft, not just on the
  existing memory/discovery forms (Pitfall 10).

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|----------------|------------------|
| Command palette client-side filtering shipped | LOW | Set `shouldFilter={false}` and wire the list to the real query — isolated to `CommandPalette.svelte`, no data-model change |
| CSRF allowlist entry missing in production | MEDIUM | Add the Procedure to `csrfWriteProcedures`, ship a patch release; audit whether the exposure window saw any forged same-origin write (check access logs for the Procedure with no prior legitimate session activity) |
| Authz duplicated/drifted between handler and store | MEDIUM | Delete the handler-level check, route the handler through the existing store method entirely, add the missing store-layer test coverage; audit for any behavior gap the handler check was silently covering that the store doesn't |
| Virtualization library breaks on Svelte 5 after adoption | MEDIUM-HIGH | Fall back to unvirtualized rendering bounded by a hard page-size limit while re-evaluating the library's issue tracker, or invest in the manual `ResizeObserver`-based measurement approach (Pitfall 5) |
| Graph view ships as a hairball with no keyboard path | HIGH | Requires a UI-spec redo (edge capping, interaction model, accessible equivalent) — treat as a design defect, not a bug fix; budget it as new phase work rather than a quick patch |

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls, following the milestone's stated build order
(developer recall first, operator curation second, newcomer browsing/graph/NL third).

| Pitfall | Prevention Phase | Verification |
|---------|-------------------|----------------|
| Command palette client-side filtering (1) | Recall-first search phase | Test asserts a real `SearchMemories` call for a term absent from static menu labels |
| Search-box race / stale response (2) | Recall-first search phase | Test resolves an earlier query after a later one and asserts final state matches the later query |
| Flash-to-empty / dishonest loading state (3) | Recall-first search phase | Test asserts distinct rendered states for loading, error, and genuine-empty-result |
| Focus loss / roving-tabindex vs activedescendant (4) | Recall-first search phase | Accessibility-tree assertion that keyboard selection and hover are independent |
| Virtualization measurement / svelte-virtual #866 (5) | Recall-first search phase (design gate before adoption) | First-paint test proves the row list renders without a scroll/resize trigger; re-check issue #866's status at implementation time |
| Authz in handler vs store (6) | Curation RPCs phase | Extend `connectapi_write_parity_test.go`-style parity table to every new RPC; unauthorized-id-returns-404 test per new RPC |
| CSRF allowlist omission (7) | Curation RPCs phase | Positive test enumerating `csrfWriteProcedures` against every write Procedure defined in the proto |
| Decorative/hairball graph (8) | Graph/discovery phase | UI-spec sets edge-cap and keyboard-equivalence constraints before a layout library is chosen; keyboard-only walkthrough as UAT |
| NL query understanding not staying advisory (9) | Query-understanding phase (conventions written earlier) | Test proves search results are unchanged until a chip is clicked; log-output test proves no query text appears without the audit flag set |
| Resume envelope gaps for new curation surfaces (10) | Curation surfaces phase | Resume round-trip UAT check per new write surface (rules, scheduled, supersede) |
| buf breaking / field-number reuse | Curation RPCs phase | `task proto:lint` / `buf breaking` in CI (already gates every proto change) |
| MCP↔Connect parity drift | Curation RPCs phase | New row per new write RPC in the existing parity test file |
| ui-drift / gen/ts regeneration | Curation RPCs phase | `task surfaces:gen` run and committed before opening the PR; CI `ui-drift`/`interface-surface drift` jobs as the backstop |
| Qdrant Facet filter/cardinality/cost | Recall-first search phase (facets) / Graph-discovery phase (tag cloud) | Facet count changes when the active filter changes; `exact:false` default confirmed in the request payload |
| URL-as-state loss on new panels | Every phase adding a new panel | New state lands in `page.url.searchParams`, following `queries.ts`'s parse/encode pattern, not component-local `$state` |
| Height-chain (`min-h-0`) breakage | Every phase adding a nested scrollable pane | Manual scroll check: only the intended pane scrolls, never the whole page |
| Badge uppercase-CSS / lowercase-DOM mismatch | Every phase adding a new badge/label | Tests assert against the lowercase source text, not the CSS-rendered case |

## Sources

**Repository evidence (HIGH confidence — direct file:line citations, read 2026-09-25):**
- `notes/console-overhaul-exploration.md` — the two live bugs, research dispositions, API gap table, skills plan
- `.planning/PROJECT.md:735-788` — Constraints and locked ADRs (DEC-cgb, DEC-xa6, DEC-kyz)
- `ui/src/lib/components/CommandPalette.svelte:9-14`
- `ui/src/lib/components/MemoryRow.svelte:37-82` (derived state, dim treatment, uppercase badge)
- `ui/src/routes/search/+page.svelte:15-22` (query key shape, no AbortSignal)
- `ui/src/lib/client.ts:1-31` (dual `engram`/`engramWrite` transport split)
- `ui/src/lib/resume.ts:1-70` (ALLOWED_DESTINATIONS/kind unions, D-09 resume envelope)
- `ui/src/lib/queries.ts:1-49` (URL-as-state parse/encode pattern)
- `internal/server/connectcsrf.go:17-108` (`csrfWriteProcedures` allowlist, interceptor ordering)
- `internal/server/connectapi_write_parity_test.go:1-60` (MCP↔Connect parity harness)
- `internal/store/spine.go:553-640` (`qdrant.NewQueryID` neighbour sub-query, reused by `RelatedMemories`)
- `internal/config/registry.go:140`, `internal/server/decider.go:168-191` (`ENGRAM_SEARCH_RERANK_AUDIT`)
- `internal/decide/jev/jev.go:230-277`, `internal/decide/many.go:12-21`, `internal/decide/errors.go:34` (`decide` span, bounded pool, timeout sentinel)
- `.github/workflows/ci.yaml:280-392` (`interface-surface drift` and `ui-drift` CI jobs)
- `buf.yaml` (`breaking.use: [FILE]`)
- `ui/package.json` (pinned versions: svelte 5.57.1, `@tanstack/svelte-query` ^6.1.34, bits-ui ^2.18.1; no `@tanstack/svelte-virtual` dependency present)
- `ui/src/lib/components/MemoryList.svelte` (current unvirtualized `#each` rendering)

**External sources (dates checked 2026-09-25):**
- bits-ui Command docs — `shouldFilter` defaults to `true` (https://www.bits-ui.com/docs/components/command)
- TanStack Query v5 migration guide + discussion #6460 — `keepPreviousData` → `placeholderData: keepPreviousData` (https://tanstack.com/query/latest/docs/framework/react/guides/migrating-to-v5, https://github.com/TanStack/query/discussions/6460)
- TanStack Query Svelte v5→v6 migration guide — store API dropped in favor of runes thunk (https://tanstack.com/query/latest/docs/framework/svelte/migrate-from-v5-to-v6) — MEDIUM confidence on `placeholderData` carrying forward unchanged into v6; not explicitly re-confirmed in this guide
- TanStack/virtual issue #866 — open Svelte 5 support gap, confirmed still open (https://github.com/TanStack/virtual/issues/866)
- WAI-ARIA APG Listbox pattern — activedescendant vs roving tabindex (https://www.w3.org/WAI/ARIA/apg/patterns/listbox), as cited in `notes/console-overhaul-exploration.md`
- Qdrant Facet API reference — `filter` respected, default `limit: 10`, `exact` default `false` (https://api.qdrant.tech/api-reference/points/facet)
- Svelte 5 `$effect` pitfalls (infinite loops on self-read state, `$derived` memory leaks in components) — multiple sveltejs/svelte GitHub issues (#16224, #11817, #18781) and svelte.dev `$effect` docs

---
*Pitfalls research for: engram operator console — recall-first search, curation, graph (milestone 2026-09-25.01)*
*Researched: 2026-09-25*
