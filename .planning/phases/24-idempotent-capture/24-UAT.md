---
status: complete
phase: 24-idempotent-capture
source: [24-01-SUMMARY.md, 24-02-SUMMARY.md]
started: 2026-07-18T21:23:29Z
updated: 2026-07-18T21:24:30Z
---

## Current Test

[testing complete]

## Tests

### 1. IdempotencyFingerprint payload round-trip (wire-invisible)
expected: Memory.IdempotencyFingerprint round-trips through payload()/fromPayload() and is wire-invisible (json:"-"); a legacy payload missing the key decodes to ""
result: pass
source: automated
coverage_id: 24-01-D1

### 2. ErrIdempotencyConflict is a distinct sentinel
expected: store.ErrIdempotencyConflict is a distinct sentinel, not ErrNotFound, and maps to a Connect code for future parity
result: pass
source: automated
coverage_id: 24-01-D2

### 3. idempotencyPointID determinism & injectivity
expected: idempotencyPointID is deterministic, injective across owner/scope boundary shifts, owner-scoped, key-sensitive, and emits a valid UUID string
result: pass
source: automated
coverage_id: 24-01-D3

### 4. contentFingerprint stability & field-sensitivity
expected: contentFingerprint is stable under tag reordering and sensitive to every client-authored field (incl. injective per-tag encoding)
result: pass
source: automated
coverage_id: 24-01-D4

### 5. Optional idempotency_key arg; keyless unchanged (SC5, D-01)
expected: storeArgs gains an optional idempotency_key arg documented on both store_memory and schedule_memory; omitting it preserves fresh-uuid-every-time behavior byte-for-byte
result: pass
source: automated
coverage_id: 24-02-D1

### 6. Keyed replay returns original with zero side-effects (SC1)
expected: A keyed store_memory replay with identical content returns the ORIGINAL (id, short_id) — no second Embed, no duplicate Qdrant point
result: pass
source: automated
coverage_id: 24-02-D2

### 7. Same key + different content rejected (SC2)
expected: Same key + different content is rejected with store.ErrIdempotencyConflict (errors.Is true), never a silent overwrite; original record left unchanged
result: pass
source: automated
coverage_id: 24-02-D3

### 8. Idempotency key is owner-scoped (SC3)
expected: Two owners reusing the identical key value get two independent, cross-invisible records (no cross-tenant point-ID poisoning)
result: pass
source: automated
coverage_id: 24-02-D4

### 9. N concurrent identical keyed writes → exactly one point (SC4, -race)
expected: N concurrent identical keyed store_memory calls resolve to exactly one Qdrant point (no-duplicate invariant), clean under go test -race
result: pass
source: automated
coverage_id: 24-02-D5

### 10. schedule_memory replay ignores window change
expected: schedule_memory replay with the same key + identical content but a CHANGED not_before/not_after window returns the original record (window excluded from the replay decision)
result: pass
source: automated
coverage_id: 24-02-D6

## Summary

total: 10
passed: 10
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none]
