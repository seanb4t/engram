# Milestones — engram

Historical record of shipped milestones. Newest first. Full per-milestone detail
in `.planning/milestones/vX.Y-ROADMAP.md` and `vX.Y-REQUIREMENTS.md`.

---

## v0.9.x — Recall Quality — ✅ SHIPPED 2026-07-10

**Phases:** 9–12 (4) · **Plans:** 12 · **Tasks:** 27 · **Requirements:** 6/6 satisfied
**Shipped:** PR #336 (squash-merge, commit `658795e9`) → `main` 2026-07-10
**Cycle:** opened 2026-07-09 → shipped 2026-07-10 (~2 days)
**Diffstat:** 96 files, +13,811 / −117 (Go deliverable + `gen/` regen + planning docs)
**Audit:** ✅ PASSED 2026-07-11 (`milestones/v0.9.x-MILESTONE-AUDIT.md`) — 11/11 integration links WIRED, 4/4 E2E flows COMPLETE, security 0-open, Nyquist compliant
**Closeout:** override_closeout — 1 acknowledged deferral (docs todo → GitHub #337); see STATE.md Deferred Items

**Delivered:** Recall is now measurably trustworthy — a labeled retrieval-quality eval
harness with always-on similarity scores and a dependency-free reranker (kills the #261
phrasing-sensitive miss: recall@8=1.00), summaries filled asynchronously off the write path,
and per-record usage signals that never touch ranking.

**Key accomplishments:**

1. **Retrieval eval harness** (`internal/retrievaleval`, `task eval:retrieval`) — env-gated Go test seeds the permanent #261 regression fixture through the prod doc-embed path into a Qdrant testcontainer and reports recall@k/MRR + baseline rank/score gap, so ranking/embedding changes are measured, not guessed.
2. **Ranking precision on the lightest lever** — a stdlib-only lexical-overlap reranker shared via `store.SearchReranked` on both MCP and Connect paths surfaces the #261 target at rank 1/8 for both near-verbatim queries (recall@8=1.00, MRR=1.000); no schema change, no reindex, no new dependency. Always-on `search_memory` similarity score shipped alongside.
3. **Asymmetric query/document embeddings reconciled** — `REQ-embedder-native-params` (#305) found already shipped under Phase 4 (native param passthrough + E5/nomic doc prefix); verified and closed, no plans built.
4. **Async-on-write summaries** (`internal/server/summaryqueue.go`) — bounded, non-blocking worker pool drains `Store.FillSummary` after upsert behind a two-switch AND-gate; a gateway outage degrades to "no summary yet" and never fails `store_memory`; drained after HTTP shutdown under a reusable RWMutex+closed concurrency kernel (CR-01), observable on OTLP.
5. **Per-memory usage signals** (`usagequeue.go`, proto 19/20) — `access_count`/`last_accessed_at` incremented only on get-by-id/update, hybrid OTLP-span + payload storage, exposed read-only on recall + Connect, with a hard D-08 invariant (backed by a negative-space test) that usage **never** affects ranking.
6. **Two durable Go kernels from code review** — CR-01 shutdown-safety (`net/http.Server.Shutdown` does not kill active handlers → guard close with RWMutex+closed) and the `*time.Time`-for-optional-timestamps convention (`omitempty` is a no-op on struct values), reused across both async queues.

**Known deferrals / tech debt (all tracked):** GitHub #334 (prod-parity #261 re-confirm on qwen3 @4096, blocked by #333), #335 (P11 review residuals), #333/#332/#331 (embed subsystem), #337 (embedding-model docs). Systemic: `.rumdl.toml` lacks a `.planning` exclude (331 markdown-lint failures on planning docs; Go deliverables clean).

**Deferred to v0.10.x (out of scope):** GitHub #322 (Connect write-lane + CSRF), #323 (session refresh-token rotation).

---

## Prior shipped (pre-GSD-milestone baseline)

These landed before GSD milestone tracking and are recorded here for continuity;
full detail in `.planning/PROJECT.md` (Validated) and `.planning/ROADMAP.md`.

- **v0.8.x Baseline** — Phases 1–7 (shipped). Authorization & isolation, recall semantics, memory kinds & tools (discovery/rule/short_id), embedder, ENGRAM_ koanf config, OTLP telemetry, web console + docs site + bundled plugin. 24 requirements, 56 ADR-locked decisions.
- **Connect Observe-Lane Auth Hardening** — Phase 8 (shipped PR #248/#266; R1–R4 verified 2026-07-08). Cookie/OIDC observe lane replaced the interim anonymous Connect mount.

---

*Latest milestone: v0.9.x — Recall Quality (2026-07-10).*
