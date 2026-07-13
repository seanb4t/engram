# Requirements: engram — v0.10.x Hardening & Write Lane

**Defined:** 2026-07-10
**Active milestone:** v0.10.x — Hardening & Write Lane (opened 2026-07-10; see Phases in ROADMAP.md)
**Core Value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its context, and wrong/stale memories can be corrected or superseded.

> **Milestone theme.** Make engram production-solid and writable over Connect: harden the
> embedder-reliability gaps surfaced by the v0.9.x eval brownouts, ship the deferred Connect
> **write lane** with CSRF + stateless session-rotation hardening, and clear the
> promoted-but-unbuilt correctness/CI backlog. Research: `.planning/research/SUMMARY.md`.
> **Zero new Go dependencies** — the write lane is wiring over the existing store-layer authz +
> `deps.*` handler logic, not new invention.

**Milestone decisions (resolved at scoping — 2026-07-10):**

- **DECISION 1 — Write-lane CRUD scope: FULL CRUD + Schedule.** All six write RPCs
  (StoreMemory, StoreDiscovery, UpdateMemory, DeleteMemory, SetVisibility, ScheduleMemory) ship
  over Connect this milestone.

- **DECISION 2 — Session rotation: STATELESS sliding-expiry re-seal.** Re-seal the existing
  AES-GCM `{owner, expiry}` cookie with a fresh expiry on each authenticated request. No
  server-side state; honors DEC-u9v (no new ADR for a token store). Revocation limits documented.

- **DECISION 3 — Reindex boundary: DOCUMENT + payload-stamp embedder identity.** Docs/Helm
  recipes pair every `ENGRAM_EMBED_*` change with a reindex callout **and** each record is
  stamped with an embedder-config-identity hash, enabling future mismatch audit/enforcement.

## v0.10.x Requirements

Grouped by category. Each maps to exactly one phase (filled in the ROADMAP traceability).

### Embedder Reliability & Options

- [x] **REQ-embed-timeout**: The embedder HTTP client timeout is operator-configurable via `ENGRAM_EMBED_TIMEOUT` (koanf, validated), replacing the hardcoded 30s that browned out under provider 529s. Raising it must not silently break the async summary-queue backoff budget (re-derive from the new timeout). *(GitHub #333)*
- [x] **REQ-embed-baseurl-join**: `ENGRAM_OPENAI_BASE_URL` joins to the embeddings path correctly across all provider base-URL shapes — trailing `/v1` (OpenRouter → no `/v1/v1` 404), no trailing `/v1` (OpenAI), and the `/v1beta/openai` Gemini shape — proven by a provider-shape table test. *(GitHub #332)*
- [x] **REQ-embed-gemini-direct**: engram can embed against the Google Gemini embeddings API via its OpenAI-compatibility endpoint, with the exact wire shape verified against live docs and the `task_type`/dimension behavior confirmed by a Phase-9 eval-harness run (a silent `task_type` no-op is a recall regression with no error to catch it — this is a correctness gate, not a docs note). *(GitHub #331)*
- [x] **REQ-embed-prod-parity-eval**: The #261 rank bar is re-confirmed on the prod-parity `qwen3-embedding-8b` @4096 config (with `ENGRAM_EMBED_QUERY_INSTRUCTION`) once the embed-timeout knob makes the run reliable — closing the last v0.9.x follow-up. *(GitHub #334; closes #261)*
- [x] **REQ-embed-model-docs**: A docs-site guide + commented Helm `values.yaml` recipes document the supported embedding models (OpenRouter / Gemini / OpenAI / local TEI-Ollama-vLLM), each pairing base URL + model + vector dim + query instruction, and every model/dim change is called out as requiring `engram reindex` (cross-linking `guides/reindex`). *(GitHub #337)*
- [x] **REQ-embed-config-identity**: Each stored record is stamped with an embedder-config-identity hash (model + dim + relevant params) so a later `reindex`-boundary audit can detect records written under a different embedding configuration. *(DECISION 3; guards the reindex boundary the three new embed levers make easier to violate)*

### Connect Write Lane

- [x] **REQ-connect-write-rpcs**: The Connect `EngramService` exposes six additive write RPCs — `StoreMemory`, `StoreDiscovery`, `UpdateMemory`, `DeleteMemory`, `SetVisibility`, `ScheduleMemory` — defined as additive-only proto changes (no field renumbering; `gen/` regenerated and drift-checked), with **no** `idempotency_level = NO_SIDE_EFFECTS` annotation on any of them (CI lint gate enforces this — that option would make a mutating RPC GET-reachable and CSRF-exploitable). *(GitHub #322; DECISION 1 = full CRUD + Schedule)*
- [x] **REQ-connect-write-authz-parity**: Every Connect write handler is a thin proto/args adapter that delegates to the **same** `deps.*` method the MCP tool calls (via a subject/actor-as-explicit-params refactor), so store-layer per-actor authz, rule immutability (DEC-iedk), summary reconciliation (DEC-ddiw), and the existence-leak not-found re-wrap (DEC-xa6) are preserved with zero duplication — proven by MCP↔Connect parity tests per RPC. *(GitHub #322; research Pitfall 1 — the #1 milestone risk)*
- [x] **REQ-connect-csrf**: All state-changing Connect RPCs are CSRF-protected using Go 1.26 stdlib `net/http.CrossOriginProtection` (Origin/Sec-Fetch-Site) as the primary defense plus a session-bound double-submit token as defense-in-depth; reads are untouched; the same-origin posture (no permissive CORS) is preserved as a permanent CI gate. *(GitHub #322; security-critical — `/gsd-secure-phase`)*

### Session Hardening

- [x] **REQ-session-rotation**: Authenticated sessions renew via stateless sliding-expiry re-seal — the AES-GCM `{owner, expiry}` cookie is re-sealed with a fresh forward-only expiry on each authenticated request — keeping a write-capable session alive without dropping an in-flight write, introducing no server-side state (honors DEC-u9v). The no-revocation limitation of a stateless cookie is explicitly documented; hard expiry stays strict with a bounded clock-skew budget. *(GitHub #323; DECISION 2; security-sensitive — `/gsd-secure-phase`)*

### Console Write UX

- [ ] **REQ-console-write-ux**: The operator console can create, edit, delete, re-share (visibility), and schedule memories/discoveries over the Connect write lane, attaching the CSRF token client-side and silently retrying once through a session re-seal (falling back to re-auth) without losing the in-flight write's input.

### Correctness & Polish

- [ ] **REQ-discovery-proto-fidelity**: `SearchDiscoveries` carries `kind` / `citations` / `summary` on the Connect wire instead of silently dropping them. *(GitHub #307)*
- [ ] **REQ-shortid-mint-cap**: `MintShortID` has a bounded collision-retry attempt cap and returns an explicit exhaustion error instead of looping. *(GitHub #308)*
- [ ] **REQ-embed-param-key-sharing**: The embed package exports a single shared reserved-param-key list so `config.ParseEmbedParams` cannot silently desync from `embedReq`'s wire contract. *(GitHub #304)*
- [ ] **REQ-embed-body-build-collapse**: The `embed.Client.embed()` two-path body build (struct-marshal vs map-merge) is collapsed into a single map-based path. *(GitHub #302)*
- [ ] **REQ-discovery-shortid-schema**: `storeDiscoveryArgs.ID` jsonschema advertises `short_id` support, matching the skill docs. *(GitHub #303)*
- [ ] **REQ-summarize-cronjob**: The Helm chart ships the `engram summarize-missing` sweep as a `batch/v1` CronJob, reusing the Deployment's image/env plumbing (factor the shared env block into `_helpers.tpl`). *(GitHub #269)*

### CI / Maintenance Hygiene

- [ ] **REQ-ci-renovate-spa-drift**: The vendored-SPA drift that reddens `main` on Renovate bumps is resolved (in-repo self-healing fallback for the inert `postUpgradeTasks` rule). *(GitHub #301)*
- [ ] **REQ-p11-review-residuals**: The Phase-11 async-summary code-review residuals are resolved — WR-03 (`Wait` misuse), IN-01 (duplicate depth-gauge registration), IN-02 (test hermeticity). *(GitHub #335)*
- [ ] **REQ-lint-planning-exclude**: `.rumdl.toml` excludes `.planning/**` so `task lint:markdown` stops failing on planning docs (the systemic 331-failure noise), while still linting shipped Markdown.

## Future Requirements (deferred beyond v0.10.x)

- Batch/bulk write RPCs over Connect (no MCP-side precedent; needs co-design).
- Runtime **enforcement** of the reindex boundary (reject/quarantine reads of records whose embedder-identity hash mismatches the live config) — v0.10.x stamps the identity; enforcement is a later decision.
- The remaining from-beads refactor cluster (#306, #309, #310, #312, #313, #315, #316, #318, #319) — opportunistic; pull into Correctness & Polish only if a phase touches the same code.

## Out of Scope

Explicitly excluded (anti-features surfaced by research — several conflict with locked ADRs).

| Feature | Reason |
|---------|--------|
| True refresh-token custody / server-side session store | DECISION 2 chose stateless sliding re-seal; a live-credential store reverses DEC-u9v/DEC-8q3 and needs its own ADR (not this milestone) |
| Handler-level authz duplicating store-layer gates | Anti-pattern — DEC-cgb/DEC-12c keep the store the single default-deny chokepoint; write handlers delegate, never re-gate |
| Permissive CORS on the Connect mux | Same-origin (not SameSite alone) is the load-bearing CSRF mitigation; `TestConnectNoCORSHeaders` stays a permanent gate |
| Auto-extraction / auto-capture from console activity | Core zero-junk invariant — capture stays explicit and user-blessed |
| Usage-signal feedback into ranking | D-08 invariant — usage signals are curation metadata, never a ranking input |
| Per-provider embedder config profiles | DEC-zyhq keeps a generic param-map passthrough, not per-vendor profiles |
| `google.golang.org/genai` native SDK | Gemini rides the existing OpenAI-compat `embed.Client`; no second SDK |
| `gorilla/csrf` / `filippo.io/csrf` | Go 1.26 stdlib `CrossOriginProtection` supersedes them |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| REQ-embed-timeout | Phase 13 | Planned |
| REQ-embed-baseurl-join | Phase 13 | Planned |
| REQ-embed-config-identity | Phase 13 | Planned |
| REQ-embed-gemini-direct | Phase 14 | Planned |
| REQ-embed-prod-parity-eval | Phase 14 | Planned |
| REQ-embed-model-docs | Phase 14 | Planned |
| REQ-connect-write-rpcs | Phase 15 | Planned |
| REQ-connect-csrf | Phase 16 | Planned |
| REQ-connect-write-authz-parity | Phase 17 | Planned |
| REQ-session-rotation | Phase 18 | Planned |
| REQ-console-write-ux | Phase 19 | Planned |
| REQ-discovery-proto-fidelity | Phase 20 | Planned |
| REQ-shortid-mint-cap | Phase 20 | Planned |
| REQ-embed-param-key-sharing | Phase 20 | Planned |
| REQ-embed-body-build-collapse | Phase 20 | Planned |
| REQ-discovery-shortid-schema | Phase 20 | Planned |
| REQ-summarize-cronjob | Phase 20 | Planned |
| REQ-ci-renovate-spa-drift | Phase 21 | Planned |
| REQ-p11-review-residuals | Phase 21 | Planned |
| REQ-lint-planning-exclude | Phase 21 | Planned |

**Coverage:** 20 requirements across 6 categories. Mapped to phases: 20/20 (roadmap complete — Phases 13–21).

---

*Requirements defined: 2026-07-10 — milestone v0.10.x — Hardening & Write Lane. Research: `.planning/research/SUMMARY.md`. Roadmap: `.planning/ROADMAP.md` (Phases 13–21, created 2026-07-10).*
