---
phase: 16-csrf-interceptor
plan: 01
subsystem: auth
tags: [csrf, hkdf, hmac, webauth, go-stdlib-crypto]

# Dependency graph
requires:
  - phase: 15-additive-proto-stub-write-handlers
    provides: the six write RPCs + Connect interceptor chain (subject/validate) this CSRF layer will slot into
provides:
  - webauth.DeriveCSRFKey(cookieKey) — HKDF sub-key derivation (engram-csrf-v1 label) from ui.cookie_key
  - webauth.CSRFSigner{Token,Verify} — HMAC-over-Owner double-submit token, hmac.Equal constant-time compare
  - webauth.CSRFCookieName ("engram_csrf") / webauth.CSRFHeaderName ("X-CSRF-Token") wire-contract constants
affects: [16-02-connect-csrf-interceptor, 16-03-csrf-cookie-issuance, 18-session-rotation]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "HKDF-Key sub-key derivation with a distinct info label for domain separation from an existing AEAD key (D-08)"
    - "HMAC-over-stable-identity (Owner only, never Owner+Expiry) so a token survives future session re-seal"

key-files:
  created:
    - internal/webauth/csrf.go
    - internal/webauth/csrf_test.go
  modified: []

key-decisions:
  - "Followed D-08 exactly: Token binds to Owner only, never Owner+Expiry, so the token is stable across the Phase-18 sliding session re-seal"
  - "NewCSRFSigner returns (*CSRFSigner, error) rather than a bare *CSRFSigner (deviates slightly from the RESEARCH.md snippet, matches PLAN.md's explicit constructor signature and NewSessionCodec's fail-fast convention)"

patterns-established:
  - "CSRFSigner mirrors SessionCodec's struct-wraps-crypto-primitive shape with a 32-byte fail-fast key guard"

requirements-completed: [REQ-connect-csrf]

coverage:
  - id: D1
    description: "DeriveCSRFKey returns a deterministic 32-byte HKDF sub-key of ui.cookie_key, distinct from the raw key (HKDF domain separation, D-08)"
    requirement: "REQ-connect-csrf"
    verification:
      - kind: unit
        ref: "internal/webauth/csrf_test.go#TestDeriveCSRFKey_DeterministicAndDistinct"
        status: pass
    human_judgment: false
  - id: D2
    description: "CSRFSigner.Token(owner) is byte-identical regardless of Session.Expiry (D-08 Owner-only binding survives Phase-18 sliding re-seal)"
    requirement: "REQ-connect-csrf"
    verification:
      - kind: unit
        ref: "internal/webauth/csrf_test.go#TestCSRFSigner_StableAcrossExpiry"
        status: pass
    human_judgment: false
  - id: D3
    description: "CSRFSigner.Verify constant-time-rejects a tampered token and a token minted for a different owner"
    requirement: "REQ-connect-csrf"
    verification:
      - kind: unit
        ref: "internal/webauth/csrf_test.go#TestCSRFSigner_TamperRejected"
        status: pass
    human_judgment: false
  - id: D4
    description: "NewCSRFSigner fails fast on a non-32-byte key, mirroring NewSessionCodec's guard"
    requirement: "REQ-connect-csrf"
    verification:
      - kind: unit
        ref: "internal/webauth/csrf_test.go#TestNewCSRFSigner_KeyGuard"
        status: pass
    human_judgment: false

duration: 10min
completed: 2026-07-11
status: complete
---

# Phase 16 Plan 01: CSRF Signer Foundation Summary

**HKDF-derived k_csrf sub-key of ui.cookie_key + HMAC-over-Owner double-submit CSRFSigner, stdlib-only, implementing D-08's Owner-only re-seal-stable token binding.**

## Performance

- **Duration:** ~10 min
- **Completed:** 2026-07-11T21:23:21-04:00
- **Tasks:** 2
- **Files modified:** 2 (both new)

## Accomplishments

- `webauth.DeriveCSRFKey` derives a 32-byte `k_csrf` from `ui.cookie_key` via `crypto/hkdf.Key` with a distinct `engram-csrf-v1` info label, cryptographically separating it from the AES-GCM session-seal key derived from the same raw material.
- `webauth.CSRFSigner` computes/verifies `HMAC-SHA256(k_csrf, Owner)` — bound to `Owner` only (D-08), so the token never rotates on a future Phase-18 sliding-expiry re-seal; `Verify` uses `hmac.Equal` (constant-time, never `==`).
- Exported `CSRFCookieName`/`CSRFHeaderName` wire-contract constants for the plan-02 interceptor and plan-03 cookie-issuance path to share.
- All four `must_haves.truths` proven by passing unit tests; zero new `go.mod` dependencies (stdlib `crypto/hkdf`/`crypto/hmac`/`crypto/sha256` only).

## Task Commits

1. **Task 1: Derive k_csrf via HKDF and define the CSRFSigner + wire-name constants** - `a70c15a1` (feat)
2. **Task 2: Unit-test D-08 Owner-only stability, tamper rejection, and the key guard** - `6fe63d3d` (test)

**Plan metadata:** (this commit, docs: complete plan)

## Files Created/Modified

- `internal/webauth/csrf.go` - `DeriveCSRFKey`, `CSRFSigner{Token,Verify}`, `NewCSRFSigner`, `CSRFCookieName`/`CSRFHeaderName` constants
- `internal/webauth/csrf_test.go` - D-08 stability, tamper/cross-owner rejection, HKDF determinism, key-guard tests

## Decisions Made

- `NewCSRFSigner` returns `(*CSRFSigner, error)` (matches the PLAN.md task spec and `NewSessionCodec`'s fail-fast convention) rather than the bare `*CSRFSigner` shown in RESEARCH.md's illustrative Pattern 4 snippet — the plan's explicit signature took precedence over the research sketch.
- No new `ENGRAM_` config keys; `k_csrf` derives from the existing `ui.cookie_key`, per D-08's "no second operator secret" intent.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None. `task lint:go` and `task test` both green. `task lint:markdown` fails on 899 pre-existing issues across `.planning/` files unrelated to this plan's changes (already tracked in STATE.md as "Systemic `.rumdl.toml` `.planning` exclude → Phase 21" tech debt) — out of scope per the executor's scope-boundary rule, not fixed here.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

`internal/webauth/csrf.go` is ready for plan 02 (`newConnectCSRFInterceptor`, which will take a `func(owner, token string) bool` verify closure — `CSRFSigner.Verify` satisfies this shape directly) and plan 03 (`webauth.Handler.Callback` minting the `engram_csrf` cookie alongside the session cookie via `CSRFSigner.Token`). No blockers.

---
*Phase: 16-csrf-interceptor*
*Completed: 2026-07-11*
