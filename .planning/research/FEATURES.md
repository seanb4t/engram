# Feature Research

**Domain:** Bounded-size reads/pagination for a vector-DB-backed memory/RAG server (Qdrant + Connect API)
**Researched:** 2026-09-18
**Confidence:** MEDIUM-HIGH (cross-checked against official docs for Qdrant, Weaviate, Milvus, and the Google AIP-158 standard text; GitHub issues/blog posts used for corroborating detail only)

## Context

This is a **subsequent-milestone** feature scan for engram's "Bounded Reads" milestone
(#585 and siblings #456/#347/#457/#497), not a whole-product feature landscape. The question
is narrow and technical: how do comparable systems bound list/search/pagination response
size, and what should a caller see when a request would exceed the bound. Findings below are
scoped to that question and mapped onto engram's two open discuss-phase decisions:

- **(A)** cap memory `content` size?
- **(B)** keep Connect `ListMemories` `limit: 0` = all + numeric offset (paged internally),
  or move to a hard cap + cursor paging?

## Feature Landscape

### Table Stakes (Every Comparable System Already Does This)

Features every vector DB / memory API in this survey already has. Missing these on a read
path is a correctness bug, not a stylistic gap — this is the class of thing #585 exists to fix.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Enforced maximum page/result size, independent of what the caller asks for | AIP-158: a paginated RPC "must be actually implemented with a non-infinite default value"; Weaviate (`QUERY_MAXIMUM_RESULTS`, default 100000), Milvus (`topk`/`nq` capped at 16,384; query `offset+limit` capped at 65,536), and Qdrant (server-side JSON body cap, default 32 MiB) all enforce a real ceiling regardless of client input | LOW–MEDIUM | Today's `Limit: 0` = all is the anti-pattern AIP-158 calls out by name; a hard ceiling closes #585's root cause, not just its symptom |
| Named, non-opaque error when a request would exceed a bound | Milvus rejects with `query results exceed the limit size` (explicit reason, not a 500); AIP-158 requires `INVALID_ARGUMENT` for a negative/invalid `page_size`; gRPC's own status-code table names `RESOURCE_EXHAUSTED` for exactly "sent or received message was larger than configured limit" | LOW | This is #585 exactly — replace opaque Connect `internal`/HTTP 500 with a classified, named error |
| Coerce-down (not silently truncate-and-hope) an oversized `page_size` request | AIP-158: "If the user specifies `page_size` greater than the maximum permitted... the API should coerce down to the maximum permitted page size" — never silently drop rows without a signal | LOW | Applies directly to `ListMemoriesRequest.limit` / `SearchMemoriesRequest.k` |
| Opaque page token; server owns pagination state | AIP-158: page tokens "must be opaque... and must not be user-parseable"; must not encode authorization | LOW | engram's `page_token` cursor mode already does this; carry the same rule into any new cursor path |
| Bounded provider/dependency I/O — never trust a downstream response body or drain to be small | General gRPC/HTTP hardening guidance (grpc.io error docs, oneuptime.com, statuscodefyi.com): treat any unary response as attacker- or bug-controlled size until proven bounded | LOW–MEDIUM | Maps directly to #347/#457 (embed/summarize client error-body and drain bounds) |
| Iterator/cursor escape hatch for full-collection scans | Milvus's `query_iterator` and Weaviate's `after` cursor exist specifically because offset-based paging degrades or hard-fails past a threshold (Weaviate: `offset+limit` cannot exceed `QUERY_MAXIMUM_RESULTS`; Milvus: `offset+limit` capped at 65,536) | MEDIUM | engram already has `cursor_mode` on `ListMemoriesRequest`; the gap is that offset mode has no page-count/byte ceiling today |

### Differentiators (Where Engram Can Do Better Than the Baseline)

Not required by any comparable system, but consistent with engram's existing design
invariants (explicit, named-error, zero-silent-data-loss) and worth building into this
milestone rather than deferring.

| Feature | Value Proposition | Complexity | Notes |
|---------|--------------------|------------|-------|
| Byte-budget-aware paging (stop a scroll page before it crosses a size ceiling, not just at a record count) | A fixed record-count cap (e.g. "500 records") is not a size guarantee if per-record `content` is unbounded — Milvus learned this the hard way (`maxOutputSize` quota, default 100 MiB, is a **byte** budget, not a row-count budget, precisely because row-count-only caps still OOM'd on wide rows) | MEDIUM–HIGH | This is the strongest argument for pairing decision (B)'s page cap with decision (A)'s content cap — see Feature Dependencies below |
| Named `field=<name> hint=<code>` classification for `ResourceExhausted` | No comparable system in this survey ties a size-exceeded error into a structured, machine-parseable envelope the way engram's `reference/errors.md` already does for every other rejection class | LOW | Pure reuse of an existing engram mechanism (`argError`) — this is "finish the pattern," not "invent one" |
| Partial-success on cross-spine follow-up failure (#456) | No system surveyed does two-phase (search + follow-up ListScopes) recall, so there is no external precedent — but the general principle ("a request's already-succeeded work must not be discarded by a downstream metadata call's failure") matches every read-path hardening doc found | MEDIUM | Independent of the pagination-bound work; ships in the same milestone because it's another instance of "one exhausted subsystem should not turn a good partial result into a 500" |
| Keep numeric-offset paging for the console/CLI UX while giving it a real ceiling internally | AIP-158 explicitly allows offset/`skip`-style paging as a documented option (not just cursor-only), and Weaviate/Milvus both keep offset paging as the default UX while bounding it — engram doesn't need to force cursor-only paging onto console/CLI callers to fix #585 | LOW–MEDIUM | Directly informs decision (B): the research does **not** support ripping out `limit: 0`/offset in favor of cursor-only; it supports capping what `limit: 0`/oversized `limit` actually does server-side |

### Anti-Features (Attractive-Looking, Wrong for This Milestone)

| Anti-Feature | Why It Looks Appealing | Why Problematic | Alternative |
|---|---|---|---|
| Just raise `MaxCallRecvMsgSize` (e.g. to 64 MiB or 128 MiB, as several Qdrant-client wrappers have done — Bifrost's Qdrant integration defaults to 64 MiB) | One-line fix, no proto/behavior change | Only moves the ceiling — the exact conclusion PROJECT.md and #583 already reached ("Raising `MaxCallRecvMsgSize` alone only moves the ceiling... it is defense in depth at most"); a collection that grows past the new ceiling reproduces the same opaque failure | Bound the *query* (page size, content size), not just the transport frame; raising the recv limit is fine as a secondary safety margin, never the fix |
| Replace unary `List`/`Search` with server-streaming RPCs to sidestep size limits entirely | gRPC's own "chunking large messages" guidance suggests streaming for genuinely unbounded responses | Backwards-incompatible proto surface change (new RPC shapes, new client code in console + CLI + MCP lane) far outside this milestone's stated scope; AIP-158's whole point is that paginated *unary* RPCs are the standard shape — streaming is for a different problem (continuous/large blobs), not "my page is occasionally too big" | Bound the page; keep unary RPCs (matches AIP-158 and every comparable system surveyed) |
| Force cursor-only paging everywhere, deprecating numeric offset | "Cursor is the correct design" is a common purist take (Weaviate's own docs push `after` for full scans) | Breaking change to the console's URL-shareable offset UX and the CLI's existing flags; ADR `engram-1frj` already chose offset-for-UI deliberately; AIP-158 tolerates `skip`-style paging as a first-class option, it does not mandate cursor-only | Keep both modes (already mutually-exclusive by rule `paging-trio-mutually-exclusive`); just bound both |
| A silent, unsignaled truncation of oversized `limit`/`k` requests (return fewer rows with no indication a cap was applied) | Simplest implementation — clamp and return | AIP-158 requires the *coercion* to happen, but page metadata should still let a caller detect they hit a ceiling (via `next_page_token` presence, or a documented max); a totally silent clamp makes a "why did I get fewer than I asked for" bug report indistinguishable from "that's just how many there are" | Coerce `limit`/`k` down to the documented maximum; rely on `next_page_token`/`total` (already on the wire) to signal more exists — no new field needed |
| Treat this milestone as an excuse to redesign the whole pagination model (e.g. add GraphQL-style connections, invent a new envelope) | Bounded Reads touches pagination code anyway | Out of stated scope (`Not in scope: the planted embedder provider-routing / failover seed stays planted` — same discipline applies here); every comparable system's pagination *shape* (page_size/page_token, offset/limit, cursor) is unremarkable — the gap is enforcement, not design | Reuse the existing `ListMemoriesRequest`/`ListMemoriesResponse` shape; add bounds and errors, not new pagination primitives |

## Feature Dependencies

```
Bounded page (record-count cap on ListMemories/ListScheduled/Search k)
    └──insufficient alone for a byte guarantee, unless──> Content size cap (open decision A)
                                                               (a per-record content ceiling turns
                                                               "cap records per page" into an actual
                                                               "cap bytes per page" guarantee — see
                                                               Milvus's maxOutputSize precedent)

Bounded page ──requires──> ResourceExhausted → named error mapping
                               (the store layer must catch qdrant client's grpc status and
                               translate through the existing field=/hint= envelope, not let
                               Connect's default internal-error mapping apply)

Operator sweeps (migrate/revert/summarize-missing/spine-review/reindex 256-batch scrolls)
    ──shares the same root cause as──> Store.List / ListScheduled / Search
        (all are qdrant.ScrollPoints/Search calls with unbounded per-record payload;
         fixing the shared scanCap/page-budget mechanism in internal/store fixes all of them
         without a per-command special case)

Cross-spine partial-success (#456) ──independent of──> page-size bounding
    (different failure class: a *second* call — ListScopes — failing after a *first* call —
     search/list — already succeeded; fixed by decoupling the two calls' error handling,
     not by bounding either call's page size)

Bounded provider error body/drain (#347/#457) ──independent of──> Qdrant page bounding
    (different subsystem — internal/embed's HTTP client — but same design principle:
     never trust an unbounded downstream response)

Qdrant testcontainer stability (#497) ──blocks──> every regression test this milestone needs
    ("Done means... a real-Qdrant regression test holding more than 4 MiB of payload" —
     a flaky testcontainer makes that gate unreliable, so #497 is a prerequisite for
     proving any of the above fixed, not merely nice-to-have CI hygiene)
```

### Dependency Notes

- **Content cap (A) strengthens the record-count-cap-alone approach:** Milvus's own history is
  the cautionary tale — a fixed row/topk cap (`16,384`) was not sufficient on its own; Milvus
  additionally enforces a **byte**-budget quota (`maxOutputSize`, default 100 MiB) precisely
  because wide rows blow past a row-count-only cap. engram's own schema already accepts this
  logic elsewhere: `Citation.excerpt` is capped at `max_bytes: 16384` and `StoreDiscoveryRequest.content`
  at `max_bytes: 65536` — `content` on a plain memory is the one text field left uncapped. A
  record-count page cap on `ListMemories`/`Search` cannot promise "this page stays under 4 MiB"
  without either (a) a content ceiling, or (b) per-page running-byte-total accounting during the
  scroll (more code, no proto change, weaker guarantee under concurrent large writes). Recommend
  the roadmap treat (A) as effectively required for a *provable* fix, not merely a nice-to-have
  alongside (B).
- **(B) offset-vs-cursor is not the load-bearing decision; the cap is.** Every comparable system
  surveyed keeps offset/`skip`-style paging as a supported, documented option (AIP-158 explicitly
  allows it) while still enforcing a hard ceiling underneath. The research does not support
  discarding `limit: 0`/offset paging in the console and CLI to fix #585 — it supports ending the
  `limit: 0` = "return everything" behavior (the literal AIP-158 anti-pattern) and adding an
  enforced max regardless of paging mode.
- **Operator sweeps share the fix, not a parallel one.** `migrate`, `revert`, `summarize-missing`,
  `spine-review`, and `reindex` all reuse `qdrant.ScrollPoints` with the same unbounded-payload
  shape as `Store.List`. A phase that lands the byte-budget/record-cap mechanism in
  `internal/store` once and threads it through every scroll call site avoids five one-off fixes
  and five sets of regression tests duplicating the same `TestListScopesFullPayloadsOverGRPCLimit`
  pattern.

## MVP Definition

### Must Ship This Milestone (per PROJECT.md's stated "Done means")

- [ ] Every Qdrant scroll/search call site in `internal/store` (List, ListScheduled, Search k,
      and the five operator sweeps) enforces a real page/byte ceiling instead of `Limit: 0` — table
      stakes, closes #585's root cause per every comparable system surveyed.
- [ ] `ResourceExhausted` from the qdrant client is caught and mapped to the existing
      `field=<name> hint=<code>` envelope, never left to fall through to Connect `internal` —
      table stakes, matches the gRPC status-code table's own guidance.
- [ ] Cross-spine recall (#456) returns already-successful search/list hits even when the
      follow-up `ListScopes` call fails — independent of the paging fix, ships alongside it.
- [ ] Embed/summarize HTTP clients bound their error-body read and their drain independently
      of `http.Client.Timeout` (#347/#457) — table stakes per general HTTP/gRPC hardening
      guidance.
- [ ] Stable Qdrant testcontainer (#497) — prerequisite for proving any of the above with a
      real-Qdrant regression test holding >4 MiB of payload, per the milestone's own "Done means."

### Decide in Discuss-Phase (this research informs, does not decide)

- [ ] **(A) Content size cap** — research leans toward yes, sized in the same family as
      engram's existing text-field caps (`Citation.excerpt` 16 KiB, discovery `content` 64 KiB),
      because a record-count-only page cap cannot *guarantee* a byte ceiling without one (see
      Milvus precedent above). An `ENGRAM_MEMORY_MAX_CONTENT_BYTES` analogue of
      `ENGRAM_MEMORY_MAX_SUMMARY_BYTES` is the shape already established in this codebase.
- [ ] **(B) Connect `ListMemories` paging shape** — research supports keeping `limit`/offset as a
      supported mode (matches AIP-158 and every surveyed system) while ending `limit: 0` = "all"
      and enforcing a real coerced-down maximum server-side, regardless of which paging mode
      (`offset`, `page_token` cursor, or unset) the caller uses.

### Explicitly Out of This Milestone

- Any new pagination primitive (GraphQL-style connections, streaming RPCs) — no comparable
  system's *shape* is the gap here, only its enforcement.
- Deprecating numeric-offset paging in the console/CLI.
- The planted embedder provider-routing/failover seed (per PROJECT.md's own "Not in scope").

## Comparable-System Reference Table

| System | Default page/result cap | Max/hard ceiling | Behavior on exceed | Byte-level guard |
|---|---|---|---|---|
| **Qdrant** (gRPC client, e.g. grpc-go) | scroll `limit` defaults to 10 in client SDKs; server sets no receive cap itself | grpc-go client default `MaxCallRecvMsgSize` = 4 MiB (4,194,304 bytes) unless overridden | `ResourceExhausted`: "received message larger than max" | REST JSON body capped at 32 MiB (`33554432 bytes`) by default, configurable |
| **Qdrant** (REST/JSON insert) | — | 32 MiB request body (default) | `400`, "Payload error: JSON payload (...) is larger than allowed" | Same 32 MiB body cap |
| **Weaviate** | `QUERY_DEFAULTS_LIMIT` = 10 | `QUERY_MAXIMUM_RESULTS` = 100,000 (soft cap on `offset+limit`) | GraphQL error "query maximum results exceeded" (a REST-endpoint variant of this was filed as a bug for returning `500` instead of `4xx`) | None separate from the count cap; `after` cursor exists specifically to bypass it for full scans |
| **Milvus** | — | `topk`/`nq` hard cap 16,384; `query` `offset+limit` hard cap 65,536; per-RPC input/output cap 64 MB each | `query results exceed the limit size` (rejected, not truncated); tunable `quotaAndLimits.limits.maxOutputSize` (default 100 MiB, "definitely don't recommend higher than 10 GB") | Yes — `maxOutputSize` is an explicit **byte** budget, independent of the row-count cap |
| **Google AIP-158 (standard, not a product)** | API-documented default (example: 50) | API-documented max (example: 1000); oversized `page_size` **coerced down**, not rejected (contested — see aip-dev issue #1428 proposing rejection instead) | Negative `page_size` → `INVALID_ARGUMENT`; end of collection signaled only by an empty `next_page_token` | Not addressed directly — AIP-158 is a shape standard, not a resource-limit standard |
| **mem0** (v3 API) | `page_size` default 100 | `page_size` max 200; `top_k` (search) 1–1000, default 10 | Not documented as erroring — page/page_size are validated query params with min/max | Not documented |
| **Zep** | `lastn` (message count) bounds session-memory recall by recency, not a byte cap | Client-side `limit`/`cursor` (sessions list) — server max undocumented in surveyed pages | Not documented | Not documented — recency-bounding (`lastn`) is Zep's substitute for a byte cap on the hot "get memory" path |
| **engram (today, pre-milestone)** | `scanCap` = 1000 records (List's internal aggregate scan); `Limit: 0` on `ListMemoriesRequest` = "all" | None enforced on per-record `content` size or total page bytes | Opaque Connect `internal` / HTTP 500 (the bug this milestone fixes) | None — `content` has no cap; `summary` capped at `ENGRAM_MEMORY_MAX_SUMMARY_BYTES` (512 B default) |

## Sources

- Qdrant gRPC message-size behavior: [qdrant/migration PR #66](https://github.com/qdrant/migration/pull/66), [qdrant/migration issue #30](https://github.com/qdrant/migration/issues/30) — both confirm grpc-go's default 4 MiB (`4194304` byte) client receive cap and that Qdrant's own server disables the limit internally.
- Qdrant REST JSON payload cap: [qdrant/qdrant issue #2537](https://github.com/qdrant/qdrant/issues/2537), [qdrant-client issue #463](https://github.com/qdrant/qdrant-client/issues/463) — both show the literal `"limit: 33554432 bytes"` (32 MiB) error text.
- Qdrant capacity/payload docs: [Qdrant Capacity Planning](https://qdrant.tech/documentation/capacity-planning/), [Qdrant Payload docs](https://qdrant.tech/documentation/manage-data/payload/) — default `hits` limit of 10, payload sizing guidance.
- Qdrant client scroll defaults: [qdrant-client `qdrant_client.py`](https://github.com/qdrant/qdrant-client/blob/cd5eb259/qdrant_client/qdrant_client.py) — `limit: int = 10` default, offset described as point-ID continuation (not a skip count).
- Qdrant configurable recv-size precedent: [Bifrost Qdrant integration commit](https://github.com/maximhq/bifrost/commit/78778fb0bd009184ab946e4feee75393108ea946) — real-world example of a 64 MiB `max_recv_msg_size_mb` knob, i.e. "raise the ceiling" as a config option, not a fix.
- Weaviate pagination: [Weaviate GraphQL Additional operators](https://docs.weaviate.io/weaviate/api/graphql/additional-operators), [Weaviate Default cluster settings](https://docs.weaviate.io/cloud/manage-clusters/default-settings), [Weaviate Search basics](https://docs.weaviate.io/weaviate/search/basics) — `QUERY_DEFAULTS_LIMIT`/`QUERY_MAXIMUM_RESULTS` values and cursor (`after`) design.
- Weaviate exceed-limit behavior: [weaviate/weaviate issue #2929](https://github.com/weaviate/weaviate/issues/2929) — "query maximum results exceeded" error and the REST-endpoint 500-vs-4xx follow-up bug report; [weaviate/weaviate issue #2302](https://github.com/weaviate/weaviate/issues/2302) — cursor design rationale.
- Milvus limits: [Milvus Limitations docs](https://milvus.io/docs/limitations.md) — per-RPC 64 MB input/output caps, `topk`/`nq` 16,384 cap.
- Milvus query-window and maxOutputSize: [milvus-io/milvus issue #39480](https://github.com/milvus-io/milvus/issues/39480) (offset+limit 65,536 window), [milvus-io/milvus issue #44578](https://github.com/milvus-io/milvus/issues/44578) (`quotaAndLimits.limits.maxOutputSize`, default 100 MiB), [milvus-io/milvus `search_reduce_util.go`](https://github.com/milvus-io/milvus/blob/5def5ced/internal/proxy/search_reduce_util.go) (byte-budget enforcement in code), [milvus-sdk-java issue #1287](https://github.com/milvus-io/milvus-sdk-java/issues/1287) (iterator pattern for full scans).
- Google AIP-158 (pagination standard): [google.aip.dev/158](https://google.aip.dev/158), [aip-dev/google.aip.dev source](https://github.com/aip-dev/google.aip.dev/blob/master/aip/general/0158.md), [AIP-132 List method](https://google.aip.dev/132) — page_size/page_token/next_page_token contract, coercion-not-rejection default (contested: [aip-dev issue #1428](https://github.com/aip-dev/google.aip.dev/issues/1428) proposes rejecting oversized `page_size` instead).
- mem0 pagination: [mem0 Get Memories API reference](https://docs.mem0.ai/api-reference/memory/get-memories), [mem0 v2→v3 migration guide](https://docs.mem0.ai/migration/platform-v2-to-v3), [mem0 Search docs](https://docs.mem0.ai/core-concepts/memory-operations/search) — `page`/`page_size` envelope, `top_k` 1–1000 bound.
- Zep pagination/recency-bounding: [Zep `list_sessions` client reference](https://getzep.github.io/zep-python/memory/client.html), [Zep Sessions guide](https://help.getzep.com/v2/sessions.mdx), [Zep Get Session Memory reference](https://help.getzep.com/v2/sdk-reference/memory/get.mdx) — `limit`/`cursor` and `page_size`/`page_number` shapes; `lastn` as a recency-bound substitute for a byte cap.
- gRPC size-limit and pagination hardening guidance: [grpc.io Error handling guide](https://grpc.io/docs/guides/error/) (status-code table naming `RESOURCE_EXHAUSTED`), [grpc/grpc `doc/statuscodes.md`](https://github.com/grpc/grpc/blob/master/doc/statuscodes.md), [grpc-go issue #7024](https://github.com/grpc/grpc-go/issues/7024) (ResourceExhausted detail gap), [oneuptime.com gRPC "Message Too Large" guide](https://oneuptime.com/blog/post/2026-01-24-fix-message-too-large-errors-grpc/view), [statuscodefyi.com RESOURCE_EXHAUSTED scenario](https://statuscodefyi.com/scenarios/grpc/grpc-resource-exhausted-message-size/) — clamp-vs-reject-vs-page tradeoffs, pagination/streaming as the "correct long-term fix" vs raising limits as a stopgap.
- engram's own precedent for text-field size caps: `proto/engram/v1/engram.proto` (`Citation.excerpt` `max_bytes: 16384`, `StoreDiscoveryRequest.content` `max_bytes: 65536`), `internal/config/validate.go` (`ENGRAM_MEMORY_MAX_SUMMARY_BYTES`), `internal/store/store.go` (`scanCap = 1000`, gRPC 4 MiB overflow comment at `store.go:1670-1672`).

---
*Feature research for: engram — Bounded Reads milestone (2026-09-18.01)*
*Researched: 2026-09-18*
