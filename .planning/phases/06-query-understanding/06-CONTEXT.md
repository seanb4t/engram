# Phase 6: Query Understanding - Context

**Gathered:** 2026-09-28
**Status:** Ready for planning

<domain>
## Phase Boundary

A prose query on the console's `/search` route yields **suggested, unapplied filter chips**
(categories, time window, scope, tags) from a new server-side Connect RPC `UnderstandQuery`,
built on `internal/decide` under a bounded, no-retry timeout. Suggestions never change results
until the user accepts one. An accepted chip is exactly a manually added chip: same `onchange`
path, same URL params, same `SearchMemoriesRequest`. A decision failure or timeout yields zero
suggestions. It never fails the search and never delays it.

Out of scope: an MCP tool or CLI verb for query understanding (D-02), the chat-LLM emulator
backend (NLQ-05, v2), suggestions in the header search or the Cmd-K menu, auto-applying any
chip, and showing probabilities in the UI.

</domain>

<decisions>
## Implementation Decisions

### Server contract & config
- **D-01 (default on, where possible — user override of NLQ-01's "off by default"):** new key
  `ENGRAM_SEARCH_UNDERSTANDING=off|jev`. When it is **unset**, the value resolves to `jev`
  if `ENGRAM_DECISIONS_PROVIDER=jev` and to `off` otherwise. An explicit `off` opts out.
  `Config.Validate` rejects an explicit `jev` when no decisions provider is configured, and
  rejects any other value. Sean chose this knowing the trade-off: an operator who configured
  a provider only for consolidate now sends console query text to that provider by default.
  The mitigation is the startup disclosure (D-15), not an opt-in gate. NLQ-01 and ROADMAP
  Phase 6 criterion 1 are amended to match. "Off" still means no decision call and search
  behaviour byte-identical to before this phase.
- **D-01a:** `ENGRAM_SEARCH_UNDERSTANDING_TIMEOUT` (default `2s`) uses the same
  parse/validate/default pattern as `searchRerankTimeout`: a non-positive or invalid value
  falls back to 2s, and Validate rejects a non-positive value while understanding is on. A
  dedicated Jev client, like `searchDeciderFromConfig`: `jev.WithTimeout(understandingTimeout)`
  + `jev.WithNoRetry()`. It never shares the sweep client.
- **D-02 (lanes):** Connect `UnderstandQuery` only. No MCP tool and no CLI verb. This is a
  deliberate exception to Sean's lane-symmetry default (`avxpsyr28d`), for a concrete reason:
  suggestion chips are a console affordance. An agent builds filters directly, and a second
  lane would add another path that sends query text to the provider. It is a read RPC, so it
  needs no CSRF write-list entry. It is authenticated like every other read.
- **D-03 (response shape):** `UnderstandQueryResponse { bool enabled; repeated
  FilterSuggestion suggestions; }`, where `FilterSuggestion { oneof kind { category |
  time_window{created_after, created_before, label} | scope | tag }; source: DECIDED |
  MATCHED; }`. There is one suggestion per chip. This is the same oneof style as
  `RelatedEdge` evidence. There is no `buf.validate` on the new request (follows `eneecjyzxh`);
  the shared core does validation. The request carries the query text plus the currently
  applied filters (scope / cross_spine / categories / tags / window), so the server can skip
  questions whose answer is already applied.
- **D-04 (capability discovery):** the RPC always exists. When understanding is off, it
  returns `{enabled:false}` at once, with no decision call and no ListScopes/ListTags work. On
  `enabled:false` the UI stops calling for the rest of the session. No new info/capabilities
  RPC.

### What gets suggested
- **D-05:** at most **one `Decide` call per query**. The State is the query text only (and
  never record content). All questions are batched into that one Request (the
  `internal/relevance` batching precedent). A decided suggestion is emitted only when its top
  probability is **≥ 0.9**, spike-003's calibration band (accuracy 1.00). It is a fixed
  constant, not a config knob.
- **D-06 (categories):** one `Noul` per console category (`convention`, `gotcha`, `decision`,
  `preference` — `CATEGORIES` in `ui/src/lib/queries.ts`). Several can be suggested.
- **D-07 (time window):** one `Choice` over `{none, today, past_week, past_month,
  past_year}`. The server converts the answer to `created_after`/`created_before` (RFC3339,
  UTC, relative to request time). `none` → no suggestion. `label` carries the bucket name for
  the chip text.
- **D-08 (scope):** one `Choice` over the caller's own `ListScopes` result plus `none`, so the
  model can only pick a scope the caller can read. It is asked only when the caller has ≤ 254
  readable scopes (the Jev 255-choice ceiling) and the request has no scope applied.
- **D-09 (tags):** no decide call. The server matches query tokens against the caller's
  `ListTags` vocabulary, case-insensitive, as a full token or a hyphen-part match. Such a
  suggestion is `source: MATCHED`. It is kept outside the advisory decide contract, and tags
  already applied are skipped.

### Console behaviour
- **D-10 (trigger):** fires on a committed `/search` query (the URL `q` changes), never per
  keystroke. It fires only for prose: not UUID- or short_id-shaped (reuse
  `ui/src/lib/search/classify.ts`), no `#`/`tag:` operator, 2+ words. It runs in parallel
  with `SearchMemories` and never gates or awaits it. Nothing in the header search or the
  Cmd-K menu.
- **D-11 (rendering):** a **"Suggested" row directly under `FacetStrip`**, with dashed-outline
  chips that mean "not applied". A click accepts through the exact `onchange` → URL path a
  manual chip uses, so the resulting URL and request are identical. `×` dismisses. No "Apply
  all". No probabilities shown.
- **D-12 (filtering & failure):** suggestions already applied are hidden. Dismissals are held
  per `q` in memory, not in the URL. On an error, a timeout, or zero suggestions the row is
  absent: there is no error UI and the search is untouched.
- **D-13 (keyboard & a11y):** the row is a roving-tabindex toolbar placed after `FacetStrip`.
  Enter/Space accepts; Delete/Backspace dismisses. Each chip's `aria-label` is
  `Suggested filter, not applied: <label>`. Follow `engram-console-conventions` (keyboard
  model, tokens, honest feedback).

### Telemetry, audit & proof
- **D-14 (always-on telemetry):** `engram.understand.*` attributes on the RPC span:
  `outcome`, `fallback_class`, `suggestion_count`, `questions_asked`. Never the query, scope
  names, or tags. This is tier 1 of the rerank telemetry scheme (`0gxjp4xwgf`).
- **D-15 (disclosure):** a startup log line when understanding is on: `search understanding
  enabled: console query text is sent to <host>`. The line states whether that came from the
  default (provider=jev) or was set explicitly. It follows the `SearchRerankInfo` /
  `logSearchRerankAuditEnabled` pattern.
- **D-16 (audit flag, NLQ-04):** `ENGRAM_SEARCH_UNDERSTANDING_AUDIT` (default `false`;
  Validate rejects a non-boolean). When `true`, it logs the query text plus the suggestion
  labels at info level, and never content. It also logs a startup Warn disclosure, and warns
  when set while understanding is off (the flag then does nothing).
- **D-17 (proof tests):**
  - Server: when off, a fake `Decider` is never called and the response is `{enabled:false}`.
  - Slog capture: no query text without the audit flag, and present with it.
  - An error or timeout gives zero suggestions within the budget, never an RPC error.
  - The 0.9 threshold and the ≤254-scope gate are exercised.
  - UI browser test: the `SearchMemories` request is unchanged until a chip is clicked, and an
    accepted chip gives the same URL and request as the manual chip.
  - Per `3p0zsqrhmb` / `m45p2b4bp7`: behaviour tests only, with no tests of Jev itself and no
    tests of tests.

### Claude's Discretion
- Exact proto field numbers and names beyond D-03's shape; the package for the understanding
  core (e.g. `internal/understand`, beside `internal/relevance`); Noul/Choice instruction
  wording; the tokenization details for D-09; the chip copy for time-window labels.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/decide` (`Decider`, `Noul`, `Choice`, `Request`, the D-12 error classes) — a
  closed question vocabulary; tags do not fit it, hence D-09.
- `internal/server/decider.go` — `searchDeciderFromConfig` (dedicated no-retry client),
  `searchRerankTimeout`, `searchRerankAudit`, `logSearchRerankAuditEnabled`,
  `SearchRerankInfo`: copy these patterns for the understanding client, timeout, audit and
  disclosure.
- `internal/config/registry.go` `search.*` keys — add `search.understanding`,
  `search.understanding_timeout`, `search.understanding_audit` here.
- `internal/relevance` / `internal/verdict` — consumer precedents (Noul batching, Choice
  usage, "hook error never fails the surrounding read").
- Store `ListScopes` / `ListTags` (Phase 1/3) — the authz-scoped option sets for D-08 and D-09.
- UI: `ui/src/lib/search/params.ts` (the one URL/request codec — an accepted chip must go
  through it), `FacetStrip.svelte` (chip styles, `onchange`), `ui/src/lib/search/classify.ts`
  (id/operator classification for D-10), `ui/src/lib/queries.ts` `CATEGORIES`.

### Established Patterns
- Advisory decisions: surfaced, never acted on (`rwtzp3m7y8`). A decision failure degrades to
  "no decision".
- TanStack Query key convention `[rpcName, ...args]` → `['understandQuery', q, …applied
  filters]`. Reads go through the read-only `engram` client (see the `engram-connect-client`
  skill).
- The telemetry tiers: always-on bounded attrs, and query text only behind an opt-in audit
  flag (`0gxjp4xwgf`).

### Integration Points
- `proto/engram/v1/engram.proto` `EngramService` — the new `UnderstandQuery` rpc. Run
  `task proto:gen`, then commit `gen/` and `ui/src/lib/gen`.
- The Connect handler in `internal/server` delegates to a shared core. There is no MCP tool
  (D-02).
- `ui/src/routes/search/+page.svelte` — mount the Suggested row under `FacetStrip`.
- Docs-site: the configure guide (new env keys, the default-on behaviour, the disclosure) and
  the Connect API reference.

</code_context>

<specifics>
## Specific Ideas

- Sean: "default jev on where possible" — understanding follows the decisions provider unless
  explicitly turned off.
- Research: `.planning/research/ARCHITECTURE.md` ("query-understanding as a server-side,
  budgeted `internal/decide` consumer") and PITFALLS.md Pitfall 9 are the design basis. The
  spike-001 limits apply: ≤255 choices, ~1.26s at 255 options.

</specifics>

<deferred>
## Deferred Ideas

- MCP `understand_query` tool and CLI verb (rejected for now by D-02; revisit if agents need
  it).
- Showing suggestion confidence in the UI.
- Chat-LLM emulator backend (NLQ-05, already v2).

</deferred>
