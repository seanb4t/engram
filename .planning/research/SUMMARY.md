# Project Research Summary

**Project:** engram — milestone 2026-09-18.01 "Bounded Reads"
**Domain:** Bounded-size reads/pagination for a Go + Qdrant memory server (Connect API + MCP + gRPC client)
**Researched:** 2026-09-18
**Confidence:** HIGH

## Executive Summary

This is a hardening milestone, not a feature-landscape scan: the goal is that no Qdrant read
or provider HTTP response can fail because of unbounded size, and every such failure surfaces
as a clear, named error instead of an opaque Connect `internal`/HTTP 500. All four research
passes converge on the same root cause and the same fix shape. Root cause: `internal/server/tools.go`'s
one shared `*qdrant.Client` sets no `MaxCallRecvMsgSize`, so grpc-go's default 4 MiB client
receive cap applies uniformly, while `Store`'s read paths (`List` in all three modes,
`ListScheduled`, `Search`/`SearchDiscovery`'s `k`, and five 256-batch operator sweeps) request
full record payloads with no page-size-to-payload-size ceiling, and memory `content` itself has
no size cap at all (unlike `summary`, already capped by `ENGRAM_MEMORY_MAX_SUMMARY_BYTES`).
#583 already fixed exactly one instance of this shape (`Store.ListScopes`, via a payload
selector) and #585 immediately found four siblings — the architecture and pitfalls research
both treat this recurrence as diagnostic: fixing sites one at a time reproduces the sequence a
third time, so the recommended approach is one shared bounded-read mechanism (a
`WithPayloadInclude`/byte-budget-aware page helper, reusing the already-proven `scrollAllPoints`
pattern) applied to every call site, not five independent patches.

The recommended approach needs **zero new Go dependencies** — every capability (payload
selectors, `Config.GrpcOptions`, gRPC status/codes inspection, Connect's `CodeResourceExhausted`,
`io.LimitReader`/`io.CopyN`) already exists in the pinned stack and, in most cases, already has a
working precedent inside this codebase (`ListScopes`'s payload-scoped scroll, the
`status.FromError`/`grpccodes.AlreadyExists` idiom, `listByCursor`'s keyset paging). Feature
research against comparable systems (Qdrant, Weaviate, Milvus, Google AIP-158, mem0, Zep)
confirms this is squarely table-stakes work — every comparable system enforces a real page
ceiling, classifies a size-exceeded request with a named error rather than a generic failure,
and coerces down an oversized request rather than silently truncating it — and that engram's
`content` field is the one text field in its own schema left uncapped relative to its own
established precedent (`Citation.excerpt` 16 KiB, discovery `content` 64 KiB).

The primary risk this research surfaces is under-scoping: record-count caps alone (`maxListLimit
= 1000`, `reindexBatch = 256`) do not bound bytes while `content` is unbounded, so a small page
of a few huge records can still overflow the same cap a page of 1000 tiny records would not.
Two further risks compound this milestone specifically: adding several new >4 MiB real-Qdrant
regression fixtures increases exactly the CI memory pressure that `internal/store`'s testcontainer
already struggled under; and a casually-written `ResourceExhausted → clear error` mapping can
either leak raw gRPC/Qdrant internals to a caller or mis-catch legitimate server-capacity
exhaustion unrelated to this bug. All three are addressed below with concrete phase-level
mitigations. Two design questions — whether to cap `content` size, and whether `ListMemories`
keeps `limit: 0` = all + offset vs. moves to a hard cap + cursor — are deliberately **not**
resolved by this research; they are flagged for discuss-phase with the evidence needed to decide
them.

## Key Findings

### Recommended Stack

Zero new `go.mod` entries. Every fix uses an already-vendored API: `qdrant.NewWithPayloadInclude`/
`NewWithPayloadExclude` (already proven in this repo for `ListScopes`), `qdrant.Config.GrpcOptions`
to raise `MaxCallRecvMsgSize` as **defense-in-depth only** (explicitly not a fix — #583 already
rejected it as the primary mechanism), `google.golang.org/grpc/status`+`codes` to detect
`ResourceExhausted` (mirrors the existing `status.FromError`/`grpccodes.AlreadyExists` idiom at
`store.go:591`), `connect.CodeResourceExhausted` for the Connect-side mapping (maps to HTTP 429;
the CLI's `exitCodeForConnectErr` already has a row anticipating this), and stdlib `io.LimitReader`/
`io.CopyN` for bounding the embed/summarize HTTP drain calls.

**Core technologies:**
- `qdrant.NewWithPayloadInclude(...)` — shrink Scroll/Get responses to only needed fields — already the fix pattern for #583, reusable for `List`'s offset/cursor paths.
- `qdrant.Config.GrpcOptions` (`grpc.MaxCallRecvMsgSize`) — raise the receive ceiling as a second line of defense only, wired at the single `storeFromConfig` client-construction site.
- `google.golang.org/grpc/status`/`codes` + `connect.CodeResourceExhausted` — classify and map the transport's own `ResourceExhausted` status instead of pre-flight size estimation.
- `io.LimitReader`/`io.CopyN` (stdlib) — bound HTTP client response-body decode, error-body read, and post-error drain in `internal/embed`/`internal/summarize`, extending an idiom already partly shipped in those files.

### Expected Features

**Must have (table stakes, per FEATURES.md and comparable-system survey):**
- A real, enforced maximum page/result size independent of caller input, on every Qdrant scroll/search/sweep call site (AIP-158, Weaviate, Milvus, Qdrant's own REST cap all do this).
- A named, classified error (not opaque `internal`/500) when a request would exceed a bound — maps directly to `ResourceExhausted` → the existing `field=<name> hint=<code>` envelope.
- Coerce-down, not silent-truncate, on an oversized caller-supplied `limit`/`k`.
- Bounded provider/dependency HTTP I/O — never trust a downstream body or drain to be small (#347/#457).
- Cross-spine recall (#456) keeps already-successful hits when a downstream coverage call fails — independent bug, same milestone.

**Should have (engram-specific differentiators, not required by any comparable system):**
- Byte-budget-aware paging (stop a page by accumulated size, not just record count) — the strongest complement to a content-size cap.
- Reuse of engram's own `field=<name> hint=<code>` envelope for the new `ResourceExhausted` case — "finish the pattern," not invent one.
- Keep numeric-offset paging for console/CLI UX (every comparable system keeps this as a supported mode) while giving it a real ceiling underneath — do not force cursor-only paging.

**Defer / explicitly out of scope:**
- Any new pagination primitive (GraphQL-style connections, streaming RPCs) — the gap here is enforcement, not shape.
- Deprecating numeric-offset paging in console/CLI.
- The planted embedder provider-routing/failover seed (per PROJECT.md).
- A hard "exactly one `ScrollAndOffset` call site" AST gate — no such gate exists today and adding one is out of this milestone's scope (architecture research: the doc-comment claim is already scoped to `spine.go` alone, four other legitimate call sites exist).

### Architecture Approach

One production `*qdrant.Client` (`internal/server/tools.go:123`) feeds every recall-gated
`Store` method; all currently request full payload with no byte ceiling. The recommended
approach is two shared mechanisms, not one: (1) an ordered-page bounded-scroll helper for
`List`/`ListScheduled`/`ListScopes`-shaped reads (which use `OrderBy`+`Limit` semantics), and
(2) a byte-budget extension of the already-existing `scrollAllPoints` whole-spine iterator for
the five unordered operator sweeps (`migrate`, `revert`, `summarize-missing`, `spine-review`,
`reindex`). Error classification happens once in `internal/store` (a typed sentinel,
`store.ErrResponseTooLarge`), then mapped once per lane at each lane's existing single
chokepoint: `connectError` for Connect (add one `case`), and a new analogous single mapper for
MCP (which currently has none — each tool closure returns a raw Go error).

**Major components:**
1. `internal/store` (`store.go`, `spine.go`, `migrate.go`, `revert.go`, `summarize.go`) — owns every Qdrant read/sweep call site; gains the bounded-page/byte-budget mechanism and the `ResourceExhausted` sentinel classification.
2. `internal/server/connecterror.go` (`connectError`) — the single existing Connect-lane error mapper; gains one `ResourceExhausted` arm.
3. `internal/server/tools.go` (MCP closures + `storeFromConfig`) — needs a new MCP-side error mapper (does not exist today) and is the sole site for the defense-in-depth `MaxCallRecvMsgSize` dial option.
4. `internal/embed`, `internal/summarize` — independent HTTP clients; already bound the error-body read, need the drain (`io.Copy(io.Discard, resp.Body)`, 4 call sites) bounded by both bytes and time.
5. Two existing AST gates in `internal/store` (`schemaversion_recallgate_test.go`) — name-keyed, not line-keyed; safe to refactor around as long as a new shared helper's name is added to `recallTransmitters` with justification.

### Critical Pitfalls

1. **Fixing only the named sites reproduces #583→#585 a third time.** Every `WithPayload(true)` Scroll/Query call site in `internal/store` is in scope, not just the five named in PROJECT.md — grep once (`rg -n 'WithPayload\(true\)|NewWithPayload\(true\)' internal/store/*.go`) and land one shared mechanism, not five patches.
2. **Page-size caps alone don't bound bytes.** `content` is unbounded today; a small page of huge records overflows the same cap a large page of tiny records wouldn't. Test both a many-small-records fixture AND a few-large-records fixture per fixed path — treat "cap content size" (open decision A) as effectively required for a provable fix, not a nice-to-have.
3. **A casual `ResourceExhausted` mapping leaks internals or over-catches.** Echoing raw gRPC error text exposes the byte ceiling and backing-store details; a bare code-based catch can also mis-map a genuine server-capacity exhaustion. Match on both the status code and the specific "received message after decompression larger than max" message shape, and assert the response body is scrubbed (not just re-coded) in tests.
4. **`io.LimitReader` alone doesn't close #457.** It bounds bytes read, not time — a slow-trickle provider under `WithTimeout(0)` (the exact scenario the issue names) still hangs. Pair the byte bound with an explicit deadline on the drain, and test both axes (large-but-fast body, and slow-trickle body under `WithTimeout(0)`) separately.
5. **New >4 MiB regression fixtures compound existing CI instability (#497).** This milestone's own "done means" bar requires several new multi-MiB real-Qdrant fixtures, which increases exactly the memory pressure #497's own root-cause theory blames — orchestrator-verified: the shared-container CI fix (#498) already shipped and no `connection refused`/`Unavailable` has recurred in 60 scanned runs since, so the remaining risk is this milestone's own new fixtures destabilizing that now-stable job, not re-diagnosing the original flake. Gate new fixtures behind `testing.Short()` (existing precedent) as a matter of course.

## Implications for Roadmap

Architecture research proposes a 6-phase build order by hard dependency (not issue number);
this synthesis adopts it directly, folding in the orchestrator-verified facts that narrow scope.

### Phase 1: Test harness stability + oversized-fixture helper
**Rationale:** Every later phase's "done" bar is a real-Qdrant regression test holding >4 MiB of payload, RED before the fix. #497's shared-container CI fix already shipped (#498, 2026-08-22) and has held for 60+ runs since — this phase is about keeping that stability intact once this milestone's own new large fixtures land, not re-diagnosing a recurred flake. Extract `TestListScopesFullPayloadsOverGRPCLimit`'s fixture-seeding shape (n records × contentBytes, with a `<= 4<<20` self-check) into a shared, reusable test helper since the next several phases each need a copy of it.
**Delivers:** a reusable oversized-fixture test helper; `testing.Short()`-gated by convention; confidence the CI job stays green as new fixtures land.
**Addresses:** #497 (residual concern only — keep stable, not re-fix).
**Avoids:** Pitfall 5 (new fixtures compounding CI resource pressure), Pitfall 6/PITFALLS.md (fixture-size-vs-compression reasoning errors).

### Phase 2: Store-layer error classification + `ResourceExhausted` mapping (both lanes)
**Rationale:** A prerequisite for writing any Phase 1-style regression test that asserts the *right* failure mode (a clear, named error) rather than just "no longer a bare 500." Independent of the read-site fixes themselves — it classifies whatever error a Qdrant RPC returns today.
**Delivers:** a `store.ErrResponseTooLarge`-style typed sentinel; one new `connectError` arm (Connect lane); a new, equally singular MCP-side mapper (does not exist today); a registered hint code in the `field=<name> hint=<code>` envelope, scrubbed of raw upstream text.
**Uses:** `google.golang.org/grpc/status`/`codes`, `connect.CodeResourceExhausted` (Stack).
**Implements:** the "classify once, map once per lane at each lane's existing chokepoint" pattern (Architecture).
**Avoids:** Pitfall 3 (leaking internals or over-broad catch).

### Phase 3: Shared bounded-read mechanism
**Rationale:** The single largest phase and the load-bearing one — builds the ordered-page helper (List/ListScheduled/ListScopes-shaped reads) and extends the existing `scrollAllPoints` with a byte budget (whole-spine sweeps), rather than five independent per-site loops. Must land together with the AST-gate reclassification (`recallTransmitters`) in the same change, since that gate goes RED the moment a new helper is wired in.
**Delivers:** two reusable bounded-scroll primitives; updated AST-gate entries with justification.
**Addresses:** the shared root cause behind #585 and its siblings (Features, Architecture, Pitfalls all converge here).
**Avoids:** Pitfall 1 (five independent per-site patches), Pitfall 2 (count-only caps).

### Phase 4: Per-site migration onto the shared mechanism
**Rationale:** Ordered by exposure, matching #585's own report: `Store.List` (all three modes, highest exposure) → `ListScheduled` → `Search`/`SearchDiscovery` (cap `k` server-side too) → the five 256-record operator sweeps. Each site needs its own real-Qdrant regression test proving *that caller's* request shape stays under the cap. Add `MaxCallRecvMsgSize` as defense-in-depth here too (one-line, low-risk).
**Delivers:** every named read path in PROJECT.md's "Target features" bounded and regression-tested.
**Addresses:** #585 and named siblings, in full.
**Avoids:** Pitfall 2 (test both many-small and few-large record fixtures per path), Pitfall 8/PITFALLS.md (preserve `total`/exhaustion semantics unchanged by whatever batching is introduced).

### Phase 5: Cross-spine partial-result semantics (#456)
**Rationale:** Independent of the byte-bounding work (pure error-handling/proto-shape change), but sequenced after Phase 2 so `connectError`'s "typed sentinel, single mapper" discipline is already established to extend for the new field. Could run in parallel with Phase 3/4 if resourcing allows.
**Delivers:** already-successful search/list hits are returned even when the follow-up `ListScopes` coverage call fails; a new wire-visible sentinel (e.g. `scopes_unknown`) distinguishing "coverage unknown" from "coverage empty" — a proto change, not a store-layer-only fix.
**Addresses:** #456.
**Avoids:** Pitfall 11/PITFALLS.md (don't fix this by making `ListScopes` swallow its own errors — that reintroduces the exact ambiguity the design already avoids).

### Phase 6: Bounded provider responses (#457; #347 verification only)
**Rationale:** Fully independent of the Qdrant read-path work — no shared code with `internal/store`. Orchestrator-verified: #347's error-body bounding is **already shipped** (landed in v0.12.x, #464) — this phase's real work is #457's unbounded drain only. Sequenced last because it has zero dependency on, and zero risk to, the rest of the milestone's critical path.
**Delivers:** all four `io.Copy(io.Discard, resp.Body)` drain calls (embed.go:295,309; summarize.go:182,191) wrapped in `io.LimitReader`, paired with an explicit deadline independent of `http.Client.Timeout`; a regression test exercising `WithTimeout(0)` + a slow-trickle body, not just a large-but-fast one.
**Addresses:** #457 (real work); confirms #347 as already-fixed (close the tracking issue, or add a regression test only).
**Avoids:** Pitfall 4 (byte-only bound doesn't close the time axis #457 actually names).

### Phase Ordering Rationale

- Phase 1 before everything: every other phase's proof mechanism (an oversized real-Qdrant fixture) depends on a stable container and a reusable fixture helper.
- Phase 2 before Phase 3/4: writing a regression test that proves the *right* failure mode (a named error, not a changed byte count) requires the error-mapping to already exist, so RED states are legible.
- Phase 3 before Phase 4: one shared mechanism, proven once, is safer to reuse five times than to invent five times — directly derived from the #583→#585 recurrence pattern in Pitfalls research.
- Phase 5 after Phase 2, parallel-eligible with 3/4: it reuses the classification discipline but shares no code with the byte-bounding mechanism.
- Phase 6 last: zero shared code with the Qdrant read path; sequencing it last avoids blocking the critical path, and confirms #347 needs no code change (only #457 does).

### Research Flags

Needs deeper research during planning:
- **Phase 3 (shared bounded-read mechanism):** the two-phase ids→payload design that architecture/pitfalls research surfaces as a candidate for offset-mode deep paging carries real correctness risk (TOCTOU on delete/supersede/archive between phases; `GetPoints` does not preserve requested-id order — confirmed against Qdrant's own issue tracker). If this design is adopted rather than a pure byte-budget shrink, plan-phase should treat the TOCTOU/ordering fixtures as explicit acceptance criteria, not implementation detail.
- **Phase 4 (per-site migration):** the `total`/exhaustion-semantics interaction (Pitfall 8) needs explicit test design — verify `total` stays independent of whatever internal batching lands, and that a size-forced partial page is distinguishable from a truly-exhausted last page.
- **Discuss-phase (pre-roadmap):** both open design decisions — (A) a `content` size cap, and (B) `ListMemories`'s `limit: 0`=all+offset vs. hard-cap+cursor — need explicit resolution before Phase 3/4 plans can be finalized, since the chosen shape of (B) determines what the ordered-page helper's signature looks like.

Standard patterns (skip research-phase):
- **Phase 1 (test harness):** reuses an existing, already-shipped fixture pattern (`TestListScopesFullPayloadsOverGRPCLimit`) verbatim.
- **Phase 2 (error classification):** reuses two already-shipped idioms in this exact codebase (`status.FromError`/`grpccodes` pattern; `field=<name> hint=<code>` envelope).
- **Phase 6 (provider response bounding):** stdlib-only, and the error-body-bounding half is already shipped in the same files — this is a narrow, well-understood extension.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Every claim verified directly against the pinned module cache source (qdrant-go-client, grpc-go, connect-go, testcontainers-go) plus this repo's own working tree — no training-data recall used. |
| Features | MEDIUM-HIGH | Cross-checked against official docs for Qdrant/Weaviate/Milvus and the AIP-158 standard text; GitHub issues/blog posts used only for corroborating detail, not as primary claims. |
| Architecture | HIGH | All findings read directly from source at this branch's HEAD, plus GitHub issue bodies read verbatim via `gh issue view`. |
| Pitfalls | HIGH for repo-cited claims (own shipped code); MEDIUM for the #497 root-cause theory itself (the issue states "not investigated," though the orchestrator's 60-run scan since the CI fix corroborates it holding). |

**Overall confidence:** HIGH

### Gaps to Address

- **Content size cap (decision A) is unresolved by design.** Research leans toward "effectively required for a provable byte-ceiling guarantee" (Milvus's own `maxOutputSize` precedent, engram's own existing text-field cap pattern), but PROJECT.md holds this open for discuss-phase. Roadmap/plan-phase should surface this leaning explicitly rather than let the paging fix silently stand in for it.
- **Paging shape (decision B) is unresolved by design.** Research supports keeping offset/`limit` as a documented mode (matches every comparable system and engram's own ADR choosing offset-for-UI deliberately) while ending `limit: 0`="all" as the literal AIP-158 anti-pattern — but the final shape (hard cap only vs. hard cap + cursor) is a discuss-phase call, not a research conclusion.
- **Whether a two-phase ids→payload design is adopted at all for deep-offset paging** is not decided by this research — it is one candidate mechanism among others (byte-budget shrink being the simpler alternative), and carries the TOCTOU/ordering risks flagged above if chosen.
- **The `internal/surfaces`-equivalent single-declaration mechanism for a new error hint code** — architecture/pitfalls research assumes this convention extends to the new `ResourceExhausted` hint but notes "verify" — plan-phase should confirm `internal/surfaces` actually covers error hints, not just conditional-rule sentences, before assuming the pattern applies unmodified.

## Sources

### Primary (HIGH confidence)
- `github.com/qdrant/go-client@v1.19.2`, `google.golang.org/grpc@v1.83.2`, `connectrpc.com/connect@v1.21.0`, `github.com/testcontainers/testcontainers-go{,/modules/qdrant}@v0.44.0` — module cache source read directly (STACK.md).
- This repo's working tree at `feat/2026-09-18.01` HEAD `ec79d4bd`: `internal/store/*.go`, `internal/server/{tools,connectapi,connecterror}.go`, `internal/embed/embed.go`, `internal/summarize/summarize.go`, `cmd/engram/client_common.go`, `.github/workflows/ci.yaml`, `.planning/PROJECT.md` (ARCHITECTURE.md, PITFALLS.md).
- GitHub issues #585, #583, #456, #347, #457, #497 — bodies read verbatim via `gh issue view --json body`.
- Official docs/standards: Qdrant capacity-planning and payload docs, Weaviate GraphQL/search docs, Milvus Limitations docs, Google AIP-158 (FEATURES.md).

### Secondary (MEDIUM confidence)
- GitHub issue trackers for corroborating mechanism detail: `qdrant/migration`#66/#30, `qdrant/qdrant`#2537/#5071, `qdrant-client`#463, `weaviate/weaviate`#2929/#2302, `milvus-io/milvus`#39480/#44578, `grpc/grpc-go`#4761, `aip-dev/google.aip.dev`#1428.
- mem0 and Zep API reference docs (pagination shape comparison only, not load-bearing for engram's decision).

### Tertiary (LOW confidence)
- The #497 CI-flakiness root-cause theory (runner resource pressure) — the issue itself states "not investigated"; corroborated only indirectly by the orchestrator's 60-run post-fix scan finding zero recurrences, not by direct diagnosis.

---
*Research completed: 2026-09-18*
*Ready for roadmap: yes*
