---
status: testing
phase: 19-console-write-ux
source: [19-VERIFICATION.md]
started: 2026-07-15T15:33:00Z
updated: 2026-07-15T15:33:00Z
---

## Current Test

number: 1
name: End-to-end write CRUD against a live server
expected: |
  Against a live `engram serve` + Qdrant + real OIDC session: create a memory, edit
  it, delete it, change its visibility (private→shared), and schedule a memory; create
  a discovery, delete it, and change its visibility. Each operation lands the
  corresponding write RPC (StoreMemory / UpdateMemory / DeleteMemory / SetVisibility /
  ScheduleMemory / StoreDiscovery), the list/detail panes reflect the change, and the
  console never silently drops a write.
awaiting: user response

## Tests

### 1. End-to-end write CRUD against a live server
expected: Against a live `engram serve` + Qdrant + real OIDC session, create/edit/delete/change-visibility/schedule a memory and create/delete/change-visibility a discovery. Each operation lands the corresponding write RPC (StoreMemory/UpdateMemory/DeleteMemory/SetVisibility/ScheduleMemory/StoreDiscovery), the list/detail panes reflect the change, and the console never silently drops a write.
result: [pending]

### 2. CSRF header accepted server-side on a real write
expected: A real write request carries `X-CSRF-Token` matching the `engram_csrf` cookie and the server's double-submit check (`connectcsrf.go`) accepts it (not just asserted in the interceptor's own unit test).
result: [pending]

### 3. Create-as-shared lands visibility=shared on a live store
expected: Trigger create-as-shared (or schedule-as-shared) against a live server; the two-call composite (Store*/Schedule then SetVisibility) is observed on the wire and the record is queryable as `shared` afterward.
result: [pending]

### 4. Session rotation/expiry mid-write: retry, form re-auth, delete/share scope
expected: Force a real session rotation/expiry mid-write and observe (a) the transparent single auth-race retry is invisible on a recoverable race; (b) on terminal failure, the inline re-auth prompt for a form (create/edit) preserves typed input across the `/auth/login` → `/ui/` OIDC round-trip via the resume envelope; (c) on terminal failure of an inline delete/share, the console lands on `/ui/` home (not the originating filtered route) per the documented in-SPA-only scope (WR-01 in 19-REVIEW.md) — delete/share do not auto-replay and the operator re-initiates them.
result: [pending]

## Summary

total: 4
passed: 0
issues: 0
pending: 4
skipped: 0
blocked: 0

## Gaps
