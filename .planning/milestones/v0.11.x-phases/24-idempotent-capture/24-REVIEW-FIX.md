---
phase: 24-idempotent-capture
fixed_at: 2026-07-18T21:10:00Z
review_path: .planning/phases/24-idempotent-capture/24-REVIEW.md
iteration: 2
findings_in_scope: 3
fixed: 1
skipped: 0
no_change_needed: 2
status: all_fixed
---

# Phase 24: Code Review Fix Report (Iteration 2)

**Fixed at:** 2026-07-18T21:10:00Z
**Source review:** .planning/phases/24-idempotent-capture/24-REVIEW.md
**Iteration:** 2

**Summary:**
- Findings in scope: 3 (0 Critical, 1 Warning, 2 Info)
- Fixed: 1
- Skipped: 0
- No-change-needed (documented, deliberate): 2

## Fixed Issues

### WR-01: CR-01's short_id read-back now runs unconditionally on every write, not just the keyed race it targets

**Files modified:** `internal/server/tools.go`
**Commit:** `e007eb54`
**Applied fix:** Gated the post-`Upsert` re-`Get` in `persistAndEnqueue` on
`m.IdempotencyFingerprint != ""`, which is only ever non-empty on the keyed
idempotency create path (set in `storeMemory`/`scheduleMemory` immediately
before calling `persistAndEnqueue`). Keyless writes (the overwhelmingly
common case) now skip the extra Qdrant round trip and return the
locally-minted `short_id` exactly as before this phase, matching the fix's
own doc comment's stated intent ("a concurrent keyed racer"). No new
parameter was needed — the condition uses state `persistAndEnqueue` already
receives.

**Verification:** `go build ./...`, `go vet ./internal/...`, and
`CGO_ENABLED=1 go test -race -count=1 ./internal/server/... ./internal/store/...`
all pass, including `TestStoreMemoryIdempotentConcurrentIdenticalOnePoint`
(SC4, the keyed-race test this gate must not regress) under `-race`. No
mock-call-count test was added for the gating itself per the guidance
(would be brittle); the existing SC4 concurrency test already exercises the
keyed path this gate preserves, and the keyless path is exercised by every
other `storeMemory`/`scheduleMemory` test in the suite.

## No-Change-Needed (Documented, Deliberate)

### IN-01: Idempotency key namespace is shared across store_memory and schedule_memory, untested for the cross-tool case

**File:** `internal/server/tools.go:678` (`checkIdempotentReplay`)
**Commit:** `eadfee3e`
**Rationale:** The reviewer's fix menu offered two options: (a) add a test
pinning the cross-tool behavior as intentional, or (b) fold a tool-identity
discriminator into the point-ID hash input. Option (b) was explicitly
**not** applied — the deterministic UUIDv5 point-ID derivation over
`(owner, scope, key)` is a locked phase design decision (D-07/D-08); altering
its hash input is a design change that must not be made unilaterally in an
auto-fix loop, and would silently un-dedup every previously keyed record on
its next replay. Applied option (a) instead: documented the cross-tool
namespace-sharing behavior directly in `checkIdempotentReplay`'s doc comment,
and added `TestCheckIdempotentReplayCrossToolNamespaceShared`, which pins the
current behavior (a `store_memory` write followed by a `schedule_memory`
retry with the same scope+key+content returns the original unscheduled
record, with no window applied) so a future change to either handler can't
silently alter it unnoticed.

**Follow-up candidate:** Whether cross-tool key sharing should instead be
disjoint (requiring a hash-input change, i.e. option (b)) is a product/design
question, not an auto-fix-appropriate change. Recommend filing a GitHub issue
to decide deliberately whether `store_memory`/`schedule_memory` should share
one idempotency-key namespace or two, since the current behavior — while now
pinned and documented — may still surprise callers who don't read the tool
descriptions closely.

### IN-02: Anonymous callers share a single idempotency-key bucket

**File:** `internal/server/idempotency.go:24` (`idempotencyPointID`)
**Commit:** `caa0991c`
**Rationale:** This is documentation-only per the reviewer's own "Fix"
guidance (marked Optional) — the underlying behavior is consistent with the
project's existing documented single-anonymous-bucket invariant
(CLAUDE.md: "No issuer → single anonymous bucket (`owner==""`)"), not a new
defect introduced by this phase. Added a one-line cross-reference in
`idempotencyPointID`'s doc comment clarifying that the "cross-owner
collision is structurally impossible (D-09)" guarantee holds only between
two *authenticated* owners, and collapses to a single shared bucket when no
OIDC issuer is configured, so a future reader doesn't assume owner-scoping
holds unconditionally. No behavior change.

## Skipped Issues

None.

---

_Fixed: 2026-07-18T21:10:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_
