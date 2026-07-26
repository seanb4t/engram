---
phase: 23-service-auth-chain-tenancy-isolation
reviewed: 2026-07-17T00:00:00Z
depth: standard
files_reviewed: 13
files_reviewed_list:
  - internal/auth/auth.go
  - internal/auth/chain.go
  - internal/auth/static_token.go
  - internal/config/config.go
  - internal/config/registry.go
  - internal/config/validate.go
  - cmd/engram/serve.go
  - internal/auth/service_owner_failclosed_test.go
  - internal/auth/chain_test.go
  - internal/auth/static_token_test.go
  - internal/config/service_auth_test.go
  - internal/store/service_principal_isolation_test.go
  - internal/server/connectapi_service_auth_parity_test.go
  - cmd/engram/serve_test.go
findings:
  critical: 1
  warning: 2
  info: 1
  total: 4
status: issues_found
---

# Phase 23: Code Review Report

**Reviewed:** 2026-07-17T00:00:00Z
**Depth:** standard
**Files Reviewed:** 13
**Status:** issues_found

## Summary

The fail-closed empty-owner logic in `internal/auth/auth.go` (`TokenVerifier`/`failClosed`) is correct and well-tested with real signature-verified tokens (`service_owner_failclosed_test.go`). `chain.go`'s deny-by-default routing and nil-verifier guards are correct and thoroughly exercised (`chain_test.go`). The `StaticTokenVerifier.TokenVerifier` comparison itself uses `crypto/subtle.ConstantTimeCompare` correctly over the full token, never short-circuits, and never interpolates the raw token into an error (`static_token_test.go`'s `TestStaticTokenNoLeak` proves this in isolation).

However, there is one critical defect in how the pieces are wired together in production: **`cmd/engram/serve.go` feeds `config.ParseServiceStaticTokens`'s token-keyed map directly into `auth.NewStaticTokenVerifier`, which expects an owner-keyed map — the two functions have inverted map orientations, and nothing in the codebase adapts between them.** This single wiring point is exercised by zero end-to-end tests: every unit test either tests `ParseServiceStaticTokens` in isolation (proving it returns a token-keyed map) or tests `NewStaticTokenVerifier` in isolation by hand-constructing an owner-keyed map that bypasses the parser entirely. The result is that the static-token auth lane, as actually deployed via `ENGRAM_SERVICE_AUTH_STATIC_TOKENS`, is broken: legitimately configured tokens can never authenticate, and in the pathological case where a presented bearer happens to equal a configured owner name, the granted "owner" becomes the raw secret token value — a secret leak into the persisted `owner`/`UserID` fields, contradicting the D-12 no-leak discipline the code otherwise upholds carefully.

## Critical Issues

### CR-01: Static-token lane wiring inverts the token↔owner map orientation — feature is non-functional and can leak the raw secret into the owner claim

**File:** `cmd/engram/serve.go:326-333` (root cause), also implicating `internal/config/config.go:313-337` and `internal/auth/static_token.go:28-71`

**Issue:**

`config.ParseServiceStaticTokens` documents and tests (`internal/config/service_auth_test.go:105-128`, `TestParseServiceStaticTokens_WellFormed`) that it returns a **token-keyed** map: `map[string]string{"tok_abc": "ci", ...}` — the presented bearer token is the map key, the owner is the value ("the token is the map key, since verification looks up by the presented token").

`auth.NewStaticTokenVerifier` documents and is tested (`internal/auth/static_token_test.go:17-21`, e.g. `NewStaticTokenVerifier(map[string]string{"owner-a": "token-aaaa"})`) as expecting the **opposite** orientation: an **owner-keyed** map, `ownerID -> token`. Its internal loop makes this explicit:

```go
// internal/auth/static_token.go
for ownerID, candidate := range v.tokens {
    if candidate == "" {
        continue
    }
    if subtle.ConstantTimeCompare([]byte(token), []byte(candidate)) == 1 {
        matchedOwner = ownerID
        matched = true
    }
}
```

`cmd/engram/serve.go` is the ONLY production call site that connects these two functions, and it passes the token-keyed map straight through with no inversion:

```go
// cmd/engram/serve.go:326-333
if svcAuth.StaticTokens != "" {
    tokens, err := config.ParseServiceStaticTokens(svcAuth.StaticTokens)
    if err != nil {
        return nil, fmt.Errorf("service-auth static-tokens config invalid: %w", err)
    }
    staticVerifier = auth.NewStaticTokenVerifier(tokens).TokenVerifier()
    ...
}
```

Concrete failure trace: given `ENGRAM_SERVICE_AUTH_STATIC_TOKENS=ci=tok_abc`, `ParseServiceStaticTokens` returns `{"tok_abc": "ci"}` (key=token, value=owner). `NewStaticTokenVerifier` stores this as-is. In the verify loop, `ownerID` is bound to `"tok_abc"` and `candidate` is bound to `"ci"`. A caller presenting the legitimately-configured bearer `"tok_abc"` is compared via `ConstantTimeCompare("tok_abc", "ci")`, which never matches — **the configured token can never authenticate**. The static-token lane is deny-everything for every legitimately configured credential, silently defeating the entire feature this phase implements.

Worse, if a presented bearer ever happens to equal a configured *owner name* (`"ci"` in this example — plausible for short, human-chosen owner identifiers like `ci`, `deploy-bot`, `svc-a`), the match succeeds and `matchedOwner` is set to the map key, which is the **raw secret token value** (`"tok_abc"`). That value then becomes `TokenInfo.UserID` and is fed into `namespacedOwner("static_token", matchedOwner)`, meaning **the raw secret token is written into the persisted `owner` field of every memory record created under that session**, and into `TokenInfo.UserID`, which downstream logging/telemetry/attribution paths read as the caller's identity. This directly violates the no-leak requirement this phase's own tests (`TestStaticTokenNoLeak`) were written to guard — but that test only exercises `NewStaticTokenVerifier` in isolation with a correctly-oriented map, so it never observes the inversion.

No test in the reviewed set (`serve_test.go`, `static_token_test.go`, `service_auth_test.go`, `connectapi_service_auth_parity_test.go`) drives `ENGRAM_SERVICE_AUTH_STATIC_TOKENS` through `config.Load` → `withAuth` → an actual `TokenVerifier()` call. `connectapi_service_auth_parity_test.go:85` constructs `auth.NewStaticTokenVerifier(map[string]string{staticOwnerID: staticToken})` directly (owner-keyed, bypassing the parser), so it also cannot catch this.

**Fix:** Invert the map at the `serve.go` call site (simplest fix — keeps both functions' existing, independently-tested and independently-documented orientations intact):

```go
if svcAuth.StaticTokens != "" {
    tokenToOwner, err := config.ParseServiceStaticTokens(svcAuth.StaticTokens)
    if err != nil {
        return nil, fmt.Errorf("service-auth static-tokens config invalid: %w", err)
    }
    ownerToToken := make(map[string]string, len(tokenToOwner))
    for token, owner := range tokenToOwner {
        ownerToToken[owner] = token // rotation: two tokens per owner is fine, map collapses fine since key is owner+token pair is 1:1 per token
    }
    staticVerifier = auth.NewStaticTokenVerifier(ownerToToken).TokenVerifier()
    ...
}
```

Note the inversion above has its own pitfall worth checking: `ParseServiceStaticTokens` explicitly permits rotation — two DISTINCT tokens mapping to the SAME owner (`ci=tok_old,ci=tok_new`). Inverting `token->owner` into `owner->token` at this call site would collapse rotation pairs down to a single token per owner (last-write-wins), silently dropping one of the two valid rotation tokens. The cleaner fix is instead to change `StaticTokenVerifier`'s internal representation to natively accept a **token-keyed** map (matching what `ParseServiceStaticTokens` already produces and what the verify loop actually needs — it looks up by presented token, not by owner), e.g.:

```go
// static_token.go
type StaticTokenVerifier struct {
    tokens map[string]string // token -> ownerID
}

func (v *StaticTokenVerifier) TokenVerifier() mcpauth.TokenVerifier {
    return func(_ context.Context, token string, _ *http.Request) (*mcpauth.TokenInfo, error) {
        if token == "" {
            return nil, errors.Join(mcpauth.ErrInvalidToken, errStaticTokenNotRecognized)
        }
        var matchedOwner string
        matched := false
        for candidate, ownerID := range v.tokens {
            if candidate == "" {
                continue
            }
            if subtle.ConstantTimeCompare([]byte(token), []byte(candidate)) == 1 {
                matchedOwner = ownerID
                matched = true
            }
        }
        ...
    }
}
```

This preserves rotation (multiple token keys can map to the same owner value) and requires no adapter at the `serve.go` call site — `tokens` from `ParseServiceStaticTokens` can be passed straight through. Whichever fix is chosen, **add an end-to-end test** that drives `ENGRAM_SERVICE_AUTH_STATIC_TOKENS` through `config.Load` and `withAuth` and asserts a legitimately configured token actually authenticates (the current test suite has no such test, which is how this shipped).

## Warnings

### WR-01: No test exercises the config-string → `withAuth` → live verify path for the static-token lane

**File:** `cmd/engram/serve_test.go` (whole file), `internal/config/service_auth_test.go` (whole file)
**Issue:** `serve_test.go` has integration-style tests for the no-lane and human-OIDC-only cases (`TestWithAuth_NoLaneConfigured_ReturnsHandlerUnchanged`, `TestWithAuth_HumanOnlyConfig_RejectsUnauthenticated`) but nothing analogous for `svcAuth.StaticTokens` or `svcAuth.OIDCIssuer`. This gap is precisely what let CR-01 ship: every existing test either stops at string-parsing (`config` package) or starts from an already-correctly-oriented map (`auth` package), so the seam between them was never exercised.
**Fix:** Add a `TestWithAuth_StaticTokenLane_AuthenticatesConfiguredToken` (and ideally a client-credentials-OIDC equivalent) in `cmd/engram/serve_test.go` that sets `config.ServiceAuthConfig{StaticTokens: "owner=secret"}`, calls `withAuth`, and asserts a request bearing `Authorization: Bearer secret` reaches the inner handler while an unrelated bearer is rejected 401. This is the only way to catch a future regression of the same shape.

### WR-02: `discriminate`'s JWT-shape heuristic can silently misroute a static token that happens to contain exactly two literal dots

**File:** `internal/auth/chain.go:41-57`
**Issue:** `looksLikeJWT` routes any bearer with exactly two `.` characters to the OIDC lane, never the static lane. If an operator configures a static token containing exactly two dots (plausible for hand-picked tokens, e.g. `ci.deploy.v2` or a token borrowed from another system's naming convention), that token will always route to the OIDC branch, fail there, and be rejected — with no clear diagnostic pointing at the real cause (the token never reaches `static_token.go` at all). This is a documented, deliberate structural discriminator (D-04) and not incorrect per se, but there is no validation in `ParseServiceStaticTokens`/`Validate()` that warns or rejects a configured static-token value containing two dots, so this failure mode is silent and hard to debug in production.
**Fix:** Either (a) add a `Validate()` check that rejects/warns on a configured static-token value containing exactly two `.` characters (mirroring the fail-fast discipline already applied to duplicate tokens), or (b) document the constraint prominently in the `ENGRAM_SERVICE_AUTH_STATIC_TOKENS` operator-facing docs so it isn't discovered via a production 401.

## Info

### IN-01: `StaticTokenVerifier.TokenInfo` omits `Expiration`

**File:** `internal/auth/static_token.go:66-69`
**Issue:** The returned `mcpauth.TokenInfo` never sets `Expiration` (zero value / no expiry), unlike the OIDC lanes which set it from `idt.Expiry`. This appears intentional (static tokens are long-lived by design, rotated rather than expired), but is worth a one-line doc comment noting the omission is deliberate, so a future reader doesn't "fix" it as an oversight.
**Fix:** Add a short comment on the `TokenInfo` literal: `// No Expiration: static tokens are long-lived by design; revoke by removing the token from ENGRAM_SERVICE_AUTH_STATIC_TOKENS.`

---

_Reviewed: 2026-07-17T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
