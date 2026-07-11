---
phase: 13
reviewers: [codex]
reviewed_at: 2026-07-11T11:13:30Z
plans_reviewed: [13-01-PLAN.md, 13-02-PLAN.md, 13-03-PLAN.md]
---

# Cross-AI Plan Review — Phase 13

## Codex Review

## Summary

The phase plan set is mostly well-scoped and matches the current code, but I found one significant correctness gap: 13-02's proposed `Memory.EmbedderIdentity string \`json:"embedder_identity,omitempty"\`` is not payload-only in this codebase. MCP full responses return `store.Memory` directly, so that tag would surface the audit field despite D-06.

## 13-01 Plan Review

### Summary
Strong plan. It targets real seams: `embed.New` hardcodes `30 * time.Second` at `internal/embed/embed.go:76`, and `embed()` still posts to `c.baseURL + "/v1/embeddings"` at `internal/embed/embed.go:191`. The config and validation approach matches existing patterns.

### Strengths
- Correctly mirrors the summarizer timeout pattern: `defaultTimeout` and `WithTimeout` already exist at `internal/summarize/summarize.go:35` and `internal/summarize/summarize.go:69`.
- Correctly places unconditional embed validation near existing always-on embed checks at `internal/config/validate.go:48`, not inside the optional summary block at `internal/config/validate.go:84`.
- D-09 is correctly scoped as assert-only: production summary queue wiring passes `summaryTimeout(cfg)` at `internal/server/tools.go:227`, and `maxElapsed` derives from the supplied attempt timeout at `internal/server/summaryqueue.go:97`.

### Concerns
- LOW: The timeout test described covers slow response bodies, but not connect stalls or custom transport behavior. `http.Client.Timeout` should cover the whole request, but a direct assertion that `WithTimeout` preserves timeout after `WithHTTPTransport` would lock the intended option composition.
- LOW: The override is validated as a URL, but not as an embeddings endpoint. This is probably acceptable because it is operator-trusted config, but the error mode will be runtime 404/405 rather than startup failure.

### Suggestions
- Add a small unit assertion that `New(..., WithHTTPTransport(...), WithTimeout(x))` leaves `c.http.Timeout == x`.
- In the URL-join test, assert the actual request path from `Embed`, not only the pure helper, so the resolved-once field is proven to be used.

### Risk Assessment
LOW. This plan is source-aligned and isolated.

## 13-02 Plan Review

### Summary
The helper, payload codec, and write-site strategy are directionally right, but the JSON representation is a blocker. `store.Memory` is a wire shape in MCP full responses, so adding a normal JSON tag violates "payload-only."

### Strengths
- Correctly avoids putting identity in `embed.Client`, which lacks dim; config has the needed fields at `internal/config/config.go:48`.
- Correctly uses manual payload/fromPayload codec points at `internal/store/store.go:282` and `internal/store/store.go:337`.
- The non-reindex write sites are correctly identified: store/schedule at `internal/server/tools.go:597`, discovery at `internal/server/tools.go:661`, update at `internal/server/tools.go:932`, and rule at `internal/server/rules.go:92`.

### Concerns
- HIGH: `json:"embedder_identity,omitempty"` is not payload-only. `shapeRecall(full=true)` returns `store.Memory` directly at `internal/server/summary.go:83`, `get_memory` returns `m` directly at `internal/server/tools.go:1075`, and `listRules(full=true)` appends `store.Memory` at `internal/server/rules.go:213`. A `recallView` negative test alone will miss this.
- MEDIUM: The plan lacks positive handler tests that persisted records from each write site actually carry the identity. `Store.Update` will preserve `cur.EmbedderIdentity` because it calls `s.Upsert(ctx, cur, vec)` at `internal/store/store.go:1385`, but a missed assignment in one handler would compile and may not fail existing tests.

### Suggestions
- Change the field tag to `json:"-"`, and keep payload persistence manual through `payload()` / `fromPayload()`.
- Add negative JSON tests for `shapeRecall(full=true)`, `get_memory`, and `listRules(full=true)`, not just `toRecallView`.
- Add positive integration tests with `d.embedderIdentity = "v1:..."`, then store/schedule/discovery/rule/update and re-read via `d.st.Get` to assert the field is persisted.

### Risk Assessment
MEDIUM-HIGH until the JSON tag is changed. The implementation goal is sound, but the current plan would violate D-06.

## 13-03 Plan Review

### Summary
Good separation of the reindex landmine. The raw-payload mechanism is real: `Reindex` upserts `Payload: p.Payload` directly at `internal/store/store.go:2139`, and the doc comment explains why `Memory` round-trip would synthesize `owner==""` at `internal/store/store.go:2004`.

### Strengths
- Correctly extends the existing `StoreAndEmbedderFromEnvNoEnsure` seam, which currently discards cfg after building store/embedder at `internal/server/tools.go:143`.
- Correctly updates the reindex command call path at `cmd/engram/reindex.go:50` and `ReindexOptions` literal at `cmd/engram/reindex.go:67`.
- Correctly preserves the owner-key invariant by adding only one raw payload key.

### Concerns
- MEDIUM: Resume mode can skip before the new stamp is written. The skip predicate only compares target content at `internal/store/store.go:2125`, and `reindexTargetContents` only returns `id -> content` at `internal/store/store.go:2165`. If a target exists with matching content but no/mismatched identity, `Resume:true` will leave it unstamped.
- LOW: The plan says to add the reindex test to `internal/store/store_test.go`, but current reindex scaffolding is in `internal/store/reindex_test.go:210`.

### Suggestions
- Make resume skip identity-aware: skip only when content matches and `embedder_identity == opts.Identity` when `opts.Identity != ""`. Otherwise re-embed/upsert, or at least patch the payload.
- Add a resume-specific test where the target already has matching content but lacks `embedder_identity`; assert the run stamps or rewrites it.
- Put new reindex tests in `internal/store/reindex_test.go`.

### Risk Assessment
MEDIUM. The core mechanism is correct, but resume behavior can undercut SC3 unless identity becomes part of the resume predicate.

---

## Consensus Summary

Only one reviewer (Codex) was invoked, so the findings below are Codex's source-grounded verdict rather than a multi-reviewer consensus. Every finding cites concrete `file:line` evidence traced against the live tree.

### Agreed Strengths
- All three plans target real, correctly-identified code seams (embed timeout at `embed.go:76`, baseURL join at `embed.go:191`, manual payload codec at `store.go:282/337`, raw-payload reindex at `store.go:2139`).
- Patterns mirror existing conventions (summarizer timeout, always-on embed validation placement, owner-key invariant preservation).

### Agreed Concerns (highest priority)
1. **HIGH — 13-02 D-06 leak:** `json:"embedder_identity,omitempty"` is NOT payload-only. `store.Memory` is returned on the wire by `shapeRecall(full=true)` (`summary.go:83`), `get_memory` (`tools.go:1075`), and `listRules(full=true)` (`rules.go:213`). Fix: tag as `json:"-"` and persist manually via `payload()`/`fromPayload()`. A `recallView` negative test alone will not catch this — add negative JSON tests at all three full-response sites.
2. **MEDIUM — 13-03 resume gap:** The reindex resume skip predicate (`store.go:2125`) compares content only (`reindexTargetContents` returns `id->content`, `store.go:2165`), so a matching-content record with missing/mismatched identity stays unstamped under `Resume:true` — undercuts SC3. Fix: make the skip predicate identity-aware.
3. **MEDIUM — 13-02 test coverage:** No positive handler tests proving each write site (store/schedule/discovery/update/rule) actually persists the identity; a missed assignment would compile and pass existing tests.
4. **LOW — 13-03 test location:** New reindex tests should go in `internal/store/reindex_test.go` (existing scaffolding at :210), not `store_test.go`.

### Divergent Views
None — single reviewer.
