---
phase: 23-service-auth-chain-tenancy-isolation
fixed_at: 2026-07-17T23:05:00Z
review_path: .planning/phases/23-service-auth-chain-tenancy-isolation/23-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 23: Code Review Fix Report

**Fixed at:** 2026-07-17T23:05:00Z
**Source review:** .planning/phases/23-service-auth-chain-tenancy-isolation/23-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 3 (CR-01, WR-01, WR-02 — `critical_warning` scope)
- Fixed: 3
- Skipped: 0

## Fixed Issues

### CR-01: Static-token lane wiring inverts the token↔owner map orientation

**Files modified:** `internal/auth/static_token.go`, `internal/auth/static_token_test.go`, `internal/server/connectapi_service_auth_parity_test.go`
**Commit:** `43f49c49`
**Applied fix:** Applied the reviewer's recommended "cleaner fix" (not the serve.go-inversion alternative): re-oriented `StaticTokenVerifier` to natively accept the token-keyed map (`token -> ownerID`) that `config.ParseServiceStaticTokens` already produces and `cmd/engram/serve.go` already passes through unmodified. Changed the struct field comment, constructor doc comment, and the verify loop (`for candidateToken, ownerID := range v.tokens`, comparing the presented token against the map KEY via `crypto/subtle.ConstantTimeCompare`, matched owner taken from the map VALUE) — preserving the existing no-early-return / constant-time / no-leak discipline exactly. `serve.go` and `config.ParseServiceStaticTokens` were left unchanged per the reviewer's note that inverting at the call site would collapse rotation pairs (two tokens -> one owner) via last-write-wins.

Updated every `NewStaticTokenVerifier(map[...]...)` literal in `static_token_test.go` from `owner->token` to `token->owner`, and rewrote `TestStaticTokenRotationSameOwnerMultipleTokens` to model rotation correctly as two distinct token keys mapping to the same owner value, asserting both resolve to the identical namespaced owner (strengthened from a weak non-empty-UserID check). Also flipped the map orientation in `internal/server/connectapi_service_auth_parity_test.go`'s `TestServiceAuthChainParity`, which constructs a real (non-stub) `StaticTokenVerifier` and would otherwise have silently broken from this same fix.

### WR-01: No test exercises the config-string → `withAuth` → live verify path for the static-token lane

**Files modified:** `cmd/engram/serve_test.go`, `internal/auth/static_token.go`
**Commit:** `56f227ad`
**Applied fix:** Added `TestWithAuth_StaticTokenLane_AuthenticatesConfiguredToken` in `cmd/engram/serve_test.go`, setting `config.ServiceAuthConfig{StaticTokens: "owner-x=secret-token-value"}` (a real config string) and driving it through `withAuth` — exercising `ParseServiceStaticTokens` → `NewStaticTokenVerifier` → a live `mcpauth.RequireBearerToken`-wrapped verify, the exact seam CR-01 broke. Asserts a request bearing `Authorization: Bearer secret-token-value` reaches the inner handler and resolves to the `owner-x` namespaced owner claim, while an unrelated bearer is rejected 401.

Writing this test surfaced a second, independent defect that the CR-01 fix alone did not cover: `mcpauth.RequireBearerToken` hard-rejects any `TokenInfo` with a zero `Expiration` ("token missing expiration"), and the static-token lane's `TokenInfo` never set one — so the lane remained unusable through the production middleware even with the map orientation corrected. This was invisible to every existing unit test because they all call the verifier function directly, bypassing `RequireBearerToken`'s wrapper. Fixed by giving static tokens a practically-permanent expiration horizon (100 years from issuance) in `static_token.go`, preserving the "revoke by removing the token from `ENGRAM_SERVICE_AUTH_STATIC_TOKENS`" operator model (this also satisfies IN-01's request for a comment noting the omission is deliberate, superseded by the concrete horizon and its doc comment). The new test fails on the pre-CR-01 code and passes after both fixes.

### WR-02: `discriminate`'s JWT-shape heuristic can silently misroute a static token that happens to contain exactly two literal dots

**Files modified:** `internal/config/validate.go`, `internal/config/service_auth_test.go`
**Commit:** `c3855a08`
**Applied fix:** Chose option (a) from the review's fix guidance: added a `Validate()` check (mirroring the existing duplicate-token fail-fast discipline) that iterates the parsed `ParseServiceStaticTokens` map and rejects any token value containing exactly two `.` characters, naming `ENGRAM_SERVICE_AUTH_STATIC_TOKENS` and explaining the JWT-shape collision with `auth.chain`'s `looksLikeJWT` discriminator in the error text. Added `TestServiceAuthValidate_StaticTokenTwoDotsRejected`, which asserts a two-dot token is rejected and that one-dot, three-dot, and no-dot tokens all pass.

## Skipped Issues

None — all in-scope findings were fixed.

---

_Fixed: 2026-07-17T23:05:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
