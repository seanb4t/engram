---
status: partial
phase: 19-console-write-ux
source: [19-VERIFICATION.md]
started: 2026-07-15T15:33:00Z
updated: 2026-07-15T16:05:00Z
---

## Current Test

[testing complete — all items blocked on live environment]

## Tests

### 1. End-to-end write CRUD against a live server
expected: Against a live `engram serve` + Qdrant + real OIDC session, create/edit/delete/change-visibility/schedule a memory and create/delete/change-visibility a discovery. Each operation lands the corresponding write RPC (StoreMemory/UpdateMemory/DeleteMemory/SetVisibility/ScheduleMemory/StoreDiscovery), the list/detail panes reflect the change, and the console never silently drops a write.
result: blocked
blocked_by: server
reason: "No live engram serve + Qdrant + OIDC stack available (nothing on common ports, no Qdrant on 6333, no engram process). No automated full-stack browser→server e2e harness exists in the repo (playwright powers vitest component tests only; no compose/local-stack file). Server-half persistence IS automatable via the existing testcontainers Qdrant integration tests. Deferred to live/ship-time verification per user decision 2026-07-15."

### 2. CSRF header accepted server-side on a real write
expected: A real write request carries `X-CSRF-Token` matching the `engram_csrf` cookie and the server's double-submit check (`connectcsrf.go`) accepts it (not just asserted in the interceptor's own unit test).
result: blocked
blocked_by: server
reason: "Requires a live server round-trip. Client-side header-set is unit-tested; server-side double-submit acceptance is automatable via a testcontainers/handler integration test but the real browser cookie→header round-trip has no e2e harness. Deferred to live/ship-time verification."

### 3. Create-as-shared lands visibility=shared on a live store
expected: Trigger create-as-shared (or schedule-as-shared) against a live server; the two-call composite (Store*/Schedule then SetVisibility) is observed on the wire and the record is queryable as `shared` afterward.
result: blocked
blocked_by: server
reason: "Composite call-count/sequencing is unit-tested against a fake client; server-side visibility persistence is automatable via the existing testcontainers Qdrant integration tests but has no wired e2e assertion yet. Deferred to live/ship-time verification."

### 4. Session rotation/expiry mid-write: retry, form re-auth, delete/share scope
expected: Force a real session rotation/expiry mid-write and observe (a) the transparent single auth-race retry is invisible on a recoverable race; (b) on terminal failure, the inline re-auth prompt for a form (create/edit) preserves typed input across the `/auth/login` → `/ui/` OIDC round-trip via the resume envelope; (c) on terminal failure of an inline delete/share, the console lands on `/ui/` home (not the originating filtered route) per the documented in-SPA-only scope (WR-03 in 19-REVIEW.md) — delete/share do not auto-replay and the operator re-initiates them.
result: blocked
blocked_by: server
reason: "Requires manipulating a real session's expiry/rotation against the live auth stack. There is NO mock OIDC provider (dex/mockoidc) in the test stack — auth is tested with fake token verifiers — so the real /auth/login→/ui/ OIDC round-trip and session rotation cannot be automated without net-new infrastructure. Unit tests substitute fake ConnectError codes. Deferred to live/ship-time verification."

## Summary

total: 4
passed: 0
issues: 0
pending: 0
skipped: 0
blocked: 4

## Gaps

[none — the 4 items are prerequisite-gated on a live environment, not code defects. Automated coverage: 204 UI tests (fake transport) + Go testcontainers Qdrant integration tests cover both halves in isolation; only the real browser↔server↔OIDC wire is unverified. Follow-up infra for full automation: a local compose stack (engram + Qdrant) + a mock OIDC IdP (mockoidc/dex) + a Playwright full-stack e2e suite.]
