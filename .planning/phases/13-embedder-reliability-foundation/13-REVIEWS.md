---
phase: 13
reviewers: [codex]
reviewed_at: 2026-07-11T11:53:53Z
review_round: 2
plans_reviewed: [13-01-PLAN.md, 13-02-PLAN.md, 13-03-PLAN.md]
prior_round: {commit: b1418d87, incorporated_via: aff8c061, high_blocker: resolved}
---

# Cross-AI Plan Review — Phase 13 (Round 2)

> Round 2 re-review of the revised plans (commit `aff8c061`) after Round 1 (`b1418d87`)
> feedback was incorporated. **Round 1's HIGH blocker (D-06 `json:"-"` wire leak) is
> CONFIRMED RESOLVED** — Codex now lists the `json:"-"` requirement as a strength, not a
> concern. No HIGH findings this round; all new findings are MEDIUM/LOW provenance-correctness
> refinements exposed once the wire-leak was fixed.

## Codex Review

**Summary**

The three plans are generally strong and source-aligned. They correctly identify the real embed timeout and URL-join seams, the clean write paths, and the reindex exception where raw Qdrant payloads are deliberately preserved. The main gap I found is in the identity hash canonicalization and verification: the plan handles JSON key order, but not semantically equivalent empty params, and it does not fully test that production builders actually compute and thread a non-empty identity.

**Strengths**

- 13-01 targets the right embed seams: `embed.New` hardcodes `30 * time.Second` at `internal/embed/embed.go:76-77`, and the request URL is still built as `c.baseURL+"/v1/embeddings"` at `internal/embed/embed.go:191`.
- The timeout design matches existing summarizer precedent: `summarize.WithTimeout` sets `http.Client.Timeout`, where `d <= 0` disables it, at `internal/summarize/summarize.go:69-72`.
- The config validation placement is correct: embed config is validated unconditionally near `internal/config/validate.go:48-59`, while summary timeout is gated by `c.Summarize.Model != ""` at `internal/config/validate.go:84-104`; the plan explicitly avoids copying that gate.
- 13-02 correctly recognizes that full MCP responses can serialize `store.Memory` directly: `shapeRecall(full=true)` returns raw `store.Memory` at `internal/server/summary.go:83-88`, `get_memory` returns `m` directly at `internal/server/tools.go:1075-1078`, and `listRules(full=true)` appends `m` at `internal/server/rules.go:212-214`. The `json:"-"` requirement is necessary.
- 13-03 correctly isolates the reindex mechanism. `Reindex` preserves raw payload maps at `internal/store/store.go:2139-2146`, and the doc comment explains why `Memory` round-tripping would corrupt absent-owner semantics at `internal/store/store.go:2004-2011`.
- The resume hazard in 13-03 is real: current resume skips on content only at `internal/store/store.go:2125-2130`, and `reindexTargetContents` returns only `id -> content` at `internal/store/store.go:2165-2190`.

**Concerns**

- **MEDIUM — identity hash can falsely drift on empty document params.**
  `config.ParseEmbedParams` returns `nil` for an empty string at `internal/config/embedparams.go:17-20`, while `"{}"` returns an empty map. The embed request path treats both equivalently because `len(params) == 0` uses the same struct body path at `internal/embed/embed.go:175-182`. The planned identity helper would marshal nil as `null` and `{}` as `{}`, causing different identity hashes for the same embedding-space behavior.

- **MEDIUM — production identity threading is source-asserted but under-tested.**
  13-02 tests handlers by setting `d.embedderIdentity` to a sentinel, which proves handlers persist a provided identity. It does not prove `buildDepsFromEnv` computes a non-empty production value. That matters because production writes flow through `buildDepsFromEnv` at `internal/server/tools.go:168-188`, then through handlers like `storeMemory` at `internal/server/tools.go:597-610`. A missed builder assignment would persist empty identity while sentinel handler tests still pass.

- **MEDIUM — reindex helper should behavior-test the returned identity, not just arity.**
  Current `StoreAndEmbedderFromEnvNoEnsure` loads config and discards it at `internal/server/tools.go:143-156`; 13-03 changes this to return identity for `cmd/engram/reindex.go:50` and `ReindexOptions` at `cmd/engram/reindex.go:67-77`. The plan updates compile tests, but should also assert the returned identity is non-empty and equals `config.EmbedderIdentity(cfg)` under default test config.

- **LOW — base URL shape validation remains permissive for query/fragment-bearing base URLs.**
  Existing validation checks scheme and host only at `internal/config/validate.go:61-72`. The proposed string join would behave poorly for a base URL with `?query` or `#fragment`. This is likely operator-error scope, but a table test or validation rejection would make the new join harder to misuse.

**Suggestions**

- Normalize document params before hashing: treat both `nil` and empty maps as the same canonical object, likely `{}`. Add a test proving `DocumentParams == ""` and `DocumentParams == "{}"` produce the same identity.
- Extend `TestBuildDepsFromEnvLoadsConfigOnce` or add a sibling test to assert `deps.embedderIdentity` is non-empty after `buildDepsFromEnv`.
- In 13-03, extend `TestStoreAndEmbedderFromEnvNoEnsureLoadsConfigOnce` to destructure the identity and assert it is non-empty while keeping the load count at exactly one.
- Consider rejecting query strings and fragments in `ENGRAM_OPENAI_BASE_URL` / `ENGRAM_OPENAI_EMBEDDINGS_URL`, or explicitly add tests documenting accepted behavior.

**Risk Assessment**

Overall risk: **MEDIUM**.

The implementation surface is well bounded, and the plans correctly handle the highest-risk source reality: reindex does not use `Memory`/`payload()`, and resume must become identity-aware. The remaining risks are mostly provenance correctness risks rather than runtime breakage: false identity drift from empty params and insufficient tests that production constructors actually provide the identity used by write paths. Fixing those before execution would make the phase low-risk.

---

## Consensus Summary

Single reviewer (Codex, source-grounded, round 2). Findings are Codex's verdict rather than a multi-reviewer consensus; every finding cites `file:line` traced against the live tree.

### Resolved Since Round 1
- **HIGH (D-06 wire leak) — RESOLVED.** Codex now lists `json:"-"` on the identity field as a *necessary strength*, confirming Round 1's blocker is fixed. The three full-response wire sites (`shapeRecall(full)`, `get_memory`, `listRules(full)`) and the reindex raw-payload / owner-invariant reality are all affirmed as correctly handled.

### Agreed Concerns (highest priority, all NEW this round)
1. **MEDIUM — identity hash false drift on empty params.** `config.ParseEmbedParams` returns `nil` for `""` but an empty map for `"{}"` (`embedparams.go:17-20`); the embed path treats both identically (`embed.go:175-182`), but the identity helper would hash `null` ≠ `{}`, producing two identities for behaviorally-identical config → spurious reindex churn / false provenance mismatch. Fix: canonicalize `nil` and empty maps before hashing; add a `"" == "{}"` identity-equality test.
2. **MEDIUM — production identity threading under-tested.** 13-02's sentinel tests prove handlers *persist a provided* identity but not that `buildDepsFromEnv` (`tools.go:168-188`) *computes a non-empty* one; a missed builder assignment ships empty identity while tests stay green. Fix: assert `deps.embedderIdentity` non-empty after `buildDepsFromEnv`.
3. **MEDIUM — reindex helper identity is arity-tested, not behavior-tested.** 13-03 should assert the identity returned by `StoreAndEmbedderFromEnvNoEnsure` is non-empty and `== config.EmbedderIdentity(cfg)` under default config, keeping load-count == 1.
4. **LOW — base-URL join permissive for query/fragment.** `?query`/`#fragment` base URLs join poorly (`validate.go:61-72` checks scheme/host only). Add a table test or reject at validation.

### Divergent Views
None — single reviewer.
