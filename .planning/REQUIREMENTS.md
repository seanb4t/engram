# Requirements: engram — v0.11.x Capture & Service Identity

**Defined:** 2026-07-16
**Core Value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its
context, and wrong/stale memories can be corrected or superseded, so recall stays trustworthy as
the store grows.

**Milestone goal:** Make programmatic capture correct and re-runnable, and give headless service
principals a first-class, isolated identity — so agents can write memory mechanically and safely
into shared stores. Grounded in the v0.11.x research set (`.planning/research/{STACK,FEATURES,
ARCHITECTURE,PITFALLS,SUMMARY,CEDAR}.md`).

## v0.11.x Requirements

Each maps to exactly one roadmap phase (see Traceability). All extend existing seams — no new
store-layer authz **primitive**, and the memory contract stays additive.

### Service Identity & Access

- [x] **REQ-service-auth-chain**: A pluggable verifier chain authenticates callers in a defined
  order — OIDC user tokens → OIDC client-credentials (service accounts) → static provisioned
  tokens — with each mechanism resolving to the same `TokenInfo{Extra[owner]}` / `Subject`
  contract the store already gates on. Which mechanisms are enabled is config-selectable
  (ENGRAM_ koanf), and the chain composes in front of the existing `mcpauth.TokenVerifier` seam
  without changing store-layer authz.

- [x] **REQ-static-token-auth**: An operator can provision static bearer tokens, each mapped to a
  fixed `owner`, via ENGRAM_ config. Tokens are verified with a constant-time compare
  (`crypto/subtle`), a token maps to exactly one owner (never a single shared owner for all
  tokens), and a token value never appears in logs, spans, or error messages.

- [x] **REQ-service-owner-failclosed**: An authenticated caller whose configured owner claim
  cannot be resolved is **rejected** (fail-closed error), never silently mapped to the anonymous
  empty-owner bucket. This holds for OIDC client-credentials tokens (which carry no `email`) and
  is proven by a test asserting "an authenticated service principal never resolves to `owner==""`"
  — the first test of the service-auth work. (Closes the milestone's #1 risk.)

### Tenancy & Authorization — Cedar foundation

- [x] **REQ-cedar-pdp-foundation**: A new `internal/authz` package embeds a cedar-go (v1.8.0,
  Apache-2.0) policy decision point with a single forward-compatible `Principal` entity (`owner`
  required; `tenant` and `roles` present as reserved-optional attributes so full tenant/group/role
  ABAC can be added later with no breaking schema change). It ships three core policies
  (own-records, shared-read, tenant-isolate) plus a defense-in-depth `forbid ... unless
  principal.owner != ""` policy; default policies are compiled in via `go:embed`.

- [x] **REQ-cedar-store-enforcement**: The PDP decides authorization over an enumerable set of
  buckets (own / shared / tenant), and `internal/store` compiles those decisions into the Qdrant
  read filter and remains the single enforcement point — recall stays filter-based with no
  per-record Cedar evaluation. This refines LOCKED `DEC-cgb` (authz enforced in the store) via a
  new ADR ("PDP decides the predicate; the store enforces it as the Qdrant filter"), and preserves
  `DEC-xa6` (no existence leak) and `DEC-kyz` (sharing grants read, never write). The change is
  behavior-preserving for existing human callers.

- [x] **REQ-service-principal-isolation**: A headless service principal is isolated to its own
  `owner` bucket by default (never the anonymous bucket, never colliding with a human owner), and
  the Cedar schema + PDP seam are demonstrably forward-compatible to a full tenant/group/role ABAC
  model without reworking the foundation.

### Capture Ergonomics & Correctness

- [x] **REQ-idempotent-capture**: `store_memory` accepts an optional idempotency key and provides
  **strict replay-safety**: a repeat call with the same key + owner returns the original record /
  result unchanged, and a repeat with the same key but different content is rejected with an
  explicit mismatch error. Idempotency is owner-scoped (a key never collides across owners) and
  race-safe (concurrent retries do not produce duplicate records in Qdrant).

- [x] **REQ-supersession-links**: A memory can supersede another via additive `supersedes` /
  `superseded_by` payload links. Superseded records are soft-hidden from recall (reusing the
  `DEC-ufz` recall gate) but remain fetchable by id (`get_memory`), and the supersede operation
  routes through the ownership **write** gate (`getWritable`/`OwnedOrAbsent`), never a read grant.
  Correction is explicit and preserves history — it never deletes or silently overwrites.

- [ ] **REQ-memory-citations**: A curated `memory`-category record may optionally carry structured
  provenance/citations using the existing discovery `Citation` shape verbatim (no new struct). The
  `payload()` write gate is relaxed from discovery-only to any category; `kind` stays
  discovery-specific. Citations are optional and never auto-populated (preserves the explicit
  zero-junk / no-auto-extraction invariant).

### Recall

- [x] **REQ-category-filter**: `search_memory` and `list_memory` accept an optional `category`
  filter over the MCP surface, at parity with the Connect `ListMemories` RPC (which already
  supports it). The filter composes onto the existing owner + scope + tags Qdrant pre-filter under
  the same authz-outer-`Must` invariant (mirrors the `DEC-4xt7` tag filter); on `search_memory` it
  is a hard pre-filter applied before vector ranking.

### Embedder / Chat Config

- [x] **REQ-chat-base-url**: The chat/summarize client can use a base URL distinct from the
  embedder's, via a new `ENGRAM_OPENAI_CHAT_BASE_URL` (koanf `openai.chat_base_url`) that defaults
  to the shared `ENGRAM_OPENAI_BASE_URL` when empty. Resolved only in the summarizer path; the
  embedder path is untouched. Unblocks per-lane providers (e.g. a local embedder + a hosted chat
  model).

## Deferred (future milestone)

Tracked, acknowledged, not in the v0.11.x roadmap.

- **Operator-editable / hot-reload Cedar policies** — v0.11.x ships embedded default policies; an
  ENGRAM_ policy-override path + reload is a later admin-UX addition.

- **Full tenant/group/role ABAC** — per-tenant `shared` scoping, group/role attributes, and
  richer policy sets. The v0.11.x foundation reserves the `tenant`/`roles` attributes so this
  layers on without rework.

- **Idempotency / citations on the Connect write lane & other write RPCs** — v0.11.x lands these
  on the MCP `store_memory` path first; Connect parity follows.

## Out of Scope

Explicitly excluded for v0.11.x, with reasoning.

| Feature | Reason |
|---------|--------|
| Auto-extraction / auto-population of memories or citations | Core invariant — capture is explicit and user-blessed; auto-harvesting is deliberately excluded to keep recall zero-junk. |
| Cedar partial-evaluation / residual-to-Qdrant-filter compilation | Not in cedar-go's stable core (confirmed); the bucket-decision → store-filter pattern is used instead. |
| SPIFFE/SPIRE workload-identity federation | Emerging M2M-identity direction; out of scope for a self-hosted single-binary server this milestone. |
| bcrypt/argon2 hashing of static tokens at rest | Inconsistent with engram's existing plaintext-secret handling (`client_secret`, `cookie_key`); constant-time compare against config is the v0.11.x approach. |
| Per-tenant `shared`-read scoping | Deferred to full ABAC; the foundation reserves the attribute but ships global shared-read (Cedar-expressed) now. |
| New Qdrant collection or a separate authz store | Violates `DEC-2bv`; all new payload fields (idempotency, supersession, citations, tenant) live on the single Memory collection. |

## Traceability

Which phases cover which requirements. Populated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| REQ-cedar-pdp-foundation | Phase 22 | Complete |
| REQ-cedar-store-enforcement | Phase 22 | Complete |
| REQ-service-auth-chain | Phase 23 | Complete |
| REQ-static-token-auth | Phase 23 | Complete |
| REQ-service-owner-failclosed | Phase 23 | Complete |
| REQ-service-principal-isolation | Phase 23 | Complete |
| REQ-idempotent-capture | Phase 24 | Complete |
| REQ-supersession-links | Phase 25 | Complete |
| REQ-memory-citations | Phase 26 | Pending |
| REQ-category-filter | Phase 26 | Complete |
| REQ-chat-base-url | Phase 26 | Complete |

**Coverage:**

- v0.11.x requirements: 11 total
- Mapped to phases: 11 (Phases 22–26)
- Unmapped: 0 ✓

---
*Requirements defined: 2026-07-16*
*Last updated: 2026-07-16 after roadmap creation (`/gsd-new-project` roadmapper) — mapped all 11 requirements to Phases 22–26; 100% coverage, no orphans.*
