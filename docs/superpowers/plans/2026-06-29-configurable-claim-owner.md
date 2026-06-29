<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Configurable-claim owner + general owner remap — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move engram's per-record authorization key (`owner`) from the OIDC `sub` to a configurable identity claim (default `email`), fail-closed, and add a general `migrate-remap-owner` CLI verb to re-stamp existing records.

**Architecture:** `owner` becomes the value of `ENGRAM_OWNER_CLAIM` (default `email`), resolved at the single read seam `SubjectFromTokenInfo` and produced by both identity-extraction points (bearer lane `internal/auth`, web-console cookie lane `internal/webauth`). The Qdrant read filter is unchanged. A new `Store.RemapOwner` mirrors `MigrateSetOwner` (Count-then-`SetPayload`-by-filter). Supersedes ADR `engram-hvg`.

**Tech Stack:** Go, cobra CLI, koanf config (field registry), `coreos/go-oidc/v3`, Qdrant Go client, `task` runner (`task` = lint + test).

**Spec:** `docs/superpowers/specs/2026-06-29-configurable-claim-owner-design.md` (bead engram-8bsz).

**Build-green ordering note:** Task 2 makes the bearer lane emit `owner_claim` *additively* (keeps `sub`); the build stays green because the read seam still reads `sub`. Task 3 is the atomic pivot: it converts the cookie lane, flips the read seam to `owner_claim`, and migrates every direct-`TokenInfo` test stub in one commit. Tasks 4–6 (store/CLI/docs) are independent of the auth pivot.

---

### Task 1: Config field `ENGRAM_OWNER_CLAIM`

**Files:**

- Modify: `internal/config/registry.go` (registry slice, after the `oidc.*` rows ~line 42)
- Modify: `internal/config/config.go:83-89` (`OIDCConfig` struct)
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestOwnerClaimDefaultAndOverride(t *testing.T) {
	// Default: no env, no flag → "email".
	c, err := Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.OIDC.OwnerClaim != "email" {
		t.Errorf("default owner_claim = %q, want %q", c.OIDC.OwnerClaim, "email")
	}

	// Env override.
	t.Setenv("ENGRAM_OWNER_CLAIM", "preferred_username")
	c, err = Load(nil)
	if err != nil {
		t.Fatalf("Load with env: %v", err)
	}
	if c.OIDC.OwnerClaim != "preferred_username" {
		t.Errorf("env owner_claim = %q, want %q", c.OIDC.OwnerClaim, "preferred_username")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestOwnerClaimDefaultAndOverride -v`
Expected: FAIL — `c.OIDC.OwnerClaim` undefined (field does not exist).

- [ ] **Step 3: Add the struct field**

In `internal/config/config.go`, add to `OIDCConfig` (after `ResourceMetadata`):

```go
// OIDCConfig holds the MCP bearer-token issuer settings and the web-UI
// confidential-client credentials.
type OIDCConfig struct {
	Issuer           string `koanf:"issuer"`
	Audience         string `koanf:"audience"`
	ClientID         string `koanf:"client_id"`
	ClientSecret     string `koanf:"client_secret"`
	ResourceMetadata string `koanf:"resource_metadata"`
	// OwnerClaim is the OIDC claim whose value becomes a record's owner (the
	// authz key) when auth is enabled. Shared by both auth lanes. Default "email".
	OwnerClaim string `koanf:"owner_claim"`
}
```

- [ ] **Step 4: Register the field**

In `internal/config/registry.go`, add to the `registry` slice immediately after the `oidc.resource_metadata` row:

```go
	{Key: "oidc.owner_claim", Env: "ENGRAM_OWNER_CLAIM", Flag: "owner-claim", Default: "email"},
```

(`Legacy` is empty — this var is brand new. The `Env` is `ENGRAM_OWNER_CLAIM`, not `ENGRAM_OIDC_OWNER_CLAIM`, matching the spec's config table; the registry decouples `Env` from `Key`.)

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestOwnerClaimDefaultAndOverride -v`
Expected: PASS.

- [ ] **Step 6: Register the `--owner-claim` serve flag**

In `cmd/engram/serve.go`, find the OIDC flag-registration block (the other `oidc-*` flags via `cmd.Flags().String("oidc-...", config.FlagDefault("oidc-..."), ...)`). Add alongside them:

```go
	cmd.Flags().String("owner-claim", config.FlagDefault("owner-claim"),
		"OIDC claim whose value becomes the record owner / authz key (default email)")
```

Run: `go build ./... && go test ./cmd/engram/ -run TestServe -v` (or the existing serve flag test if present).
Expected: builds; existing serve tests PASS.

- [ ] **Step 7: Commit**

Commit per `references/vcs-preamble.md` (jj): `jj commit -m "feat(config): add ENGRAM_OWNER_CLAIM (oidc.owner_claim, default email) (engram-8bsz)"`

---

### Task 2: Bearer lane emits `owner_claim` (additive) + `email_verified` gate

**Files:**

- Modify: `internal/auth/auth.go` (`Verifier`, `New`, `TokenVerifier`, `identityClaims`)
- Modify: `cmd/engram/serve.go:217` (`auth.New` call in `withAuth`)
- Test: `internal/auth/auth_test.go`

This is **additive**: `Extra["sub"]` is preserved; `Extra["owner_claim"]` is added. The read seam still reads `sub` until Task 3, so the build stays green.

**Testability decision:** `oidc.IDToken.Claims(&v)` only decodes when the token was produced from a real signed JWT — the existing test helper `fakeIDV{tok: &oidc.IDToken{Subject:…}}` (auth_test.go:66-73) builds a token with **no claims payload**, so `Claims()` returns an error and populates nothing. Rather than force the heavyweight `oidctest` signed-token round-trip into every test, extract the claim-selection + `email_verified` logic into a **pure helper** `ClaimIdentity(raw map[string]any, ownerClaim string)` that is unit-tested directly from a map. The helper is **exported** so the webauth lane (Task 3) reuses it (no import cycle: `auth` does not import `webauth`).

- [ ] **Step 1: Write the failing unit tests for the pure helper**

Add to `internal/auth/auth_test.go`:

```go
func TestClaimIdentity(t *testing.T) {
	// email claim: verified → owner is the email; email/username also returned.
	owner, email, user, err := ClaimIdentity(map[string]any{
		"email": "u1@example.com", "email_verified": true, "preferred_username": "u1",
	}, "email")
	if err != nil || owner != "u1@example.com" || email != "u1@example.com" || user != "u1" {
		t.Fatalf("verified email: owner=%q email=%q user=%q err=%v", owner, email, user, err)
	}

	// email claim: unverified and absent email_verified both reject.
	for name, raw := range map[string]map[string]any{
		"explicit false": {"email": "u@e.com", "email_verified": false},
		"absent":         {"email": "u@e.com"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, _, err := ClaimIdentity(raw, "email"); err == nil {
				t.Error("expected rejection when email_verified is not true")
			}
		})
	}

	// custom claim: no email_verified gate; owner is that claim's value.
	owner, _, _, err = ClaimIdentity(map[string]any{"preferred_username": "alice"}, "preferred_username")
	if err != nil || owner != "alice" {
		t.Fatalf("custom claim: owner=%q err=%v", owner, err)
	}

	// missing owner claim → empty owner, no error (the read seam fails closed on empty).
	owner, _, _, err = ClaimIdentity(map[string]any{"preferred_username": "x"}, "email")
	// "email" gate: email_verified absent → rejected, so this returns an error.
	if err == nil {
		t.Error("email claim with no email_verified: expected rejection")
	}

	// non-email missing claim → empty owner, nil error.
	owner, _, _, err = ClaimIdentity(map[string]any{}, "some_claim")
	if err != nil || owner != "" {
		t.Fatalf("missing non-email claim: owner=%q err=%v, want \"\",nil", owner, err)
	}
}
```

- [ ] **Step 1b: Write the failing wiring assertion**

Add a test that `Verifier` carries `ownerClaim` and stamps `Extra["owner_claim"]`, using the existing `fakeIDV` (note: with a hand-built token `Claims` yields nothing, so set `ownerClaim:""` here — this only asserts the key is stamped and `sub` is preserved; claim *selection* is covered by `TestClaimIdentity`):

```go
func TestTokenVerifierStampsOwnerClaimKey(t *testing.T) {
	v := &Verifier{idv: fakeIDV{tok: &oidc.IDToken{Subject: "user-1"}}, ownerClaim: ""}
	info, err := v.TokenVerifier()(context.Background(), "tok", nil)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if _, ok := info.Extra["owner_claim"]; !ok {
		t.Error("Extra must carry an owner_claim key")
	}
	if info.Extra["sub"] != "user-1" {
		t.Errorf("sub = %v, want user-1 (preserved)", info.Extra["sub"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/auth/ -run 'TestClaimIdentity|TestTokenVerifierStampsOwnerClaimKey' -v`
Expected: FAIL — `ClaimIdentity` undefined; `Verifier` has no `ownerClaim` field.

- [ ] **Step 3: Add `ownerClaim` to `Verifier` and `New`**

In `internal/auth/auth.go`:

```go
type Verifier struct {
	idv        idVerifier
	ownerClaim string
}

// New performs OIDC discovery against issuer and returns a Verifier. ownerClaim
// selects the claim whose value becomes the record owner (default "email" supplied
// by the caller). If audience is non-empty it becomes the required `aud` claim.
func New(ctx context.Context, issuer, audience, ownerClaim string) (*Verifier, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery %q: %w", issuer, err)
	}
	return &Verifier{
		idv: provider.Verifier(&oidc.Config{
			ClientID:          audience,
			SkipClientIDCheck: audience == "",
		}),
		ownerClaim: ownerClaim,
	}, nil
}
```

- [ ] **Step 4: Add the pure `ClaimIdentity` helper**

In `internal/auth/auth.go`, add (replacing the now-unused `identityClaims` struct at lines 66-69):

```go
// ClaimIdentity extracts identity fields from a decoded ID-token payload and
// enforces email_verified when ownerClaim=="email". It is pure (no I/O) so both
// auth lanes can share it and unit-test it from a map. owner MAY be "" — the
// caller decides whether an empty owner is fatal (the bearer lane defers that to
// SubjectFromTokenInfo; the cookie lane rejects at login). A non-nil error means
// the token must be rejected (currently only the email_verified gate). An ABSENT
// email_verified decodes to the bool zero value (false) and is rejected.
func ClaimIdentity(raw map[string]any, ownerClaim string) (owner, email, username string, err error) {
	email, _ = raw["email"].(string)
	username, _ = raw["preferred_username"].(string)
	owner, _ = raw[ownerClaim].(string)
	if ownerClaim == "email" {
		if verified, _ := raw["email_verified"].(bool); !verified {
			return "", "", "", fmt.Errorf("email not verified")
		}
	}
	return owner, email, username, nil
}
```

- [ ] **Step 5: Use the helper in `TokenVerifier`**

Replace the claims-decode + return block in `TokenVerifier` (currently `var claims identityClaims; _ = idt.Claims(&claims); return &mcpauth.TokenInfo{...}`):

```go
		// Decode the full payload so the configured owner-claim can be read by
		// name. ClaimIdentity enforces email_verified; identity (UserID) is still
		// best-effort. The owner-claim value may be empty here — SubjectFromTokenInfo
		// fails closed on an empty owner_claim downstream.
		var raw map[string]any
		_ = idt.Claims(&raw)
		ownerVal, email, username, cerr := ClaimIdentity(raw, v.ownerClaim)
		if cerr != nil {
			err = errors.Join(mcpauth.ErrInvalidToken, cerr)
			return nil, err
		}
		return &mcpauth.TokenInfo{
			UserID:     identity(idt.Subject, email, username),
			Expiration: idt.Expiry,
			Extra:      map[string]any{"sub": idt.Subject, "email": email, "owner_claim": ownerVal},
		}, nil
```

(The `identity` helper still takes `email, username` strings, unchanged. Confirm `errors` is imported — it already is.)

- [ ] **Step 6: Wire the serve.go call site**

In `cmd/engram/serve.go`, `withAuth` (line 217):

```go
	verifier, err := auth.New(ctx, oidc.Issuer, oidc.Audience, oidc.OwnerClaim)
```

(`oidc` here is the `config.OIDCConfig` passed into `withAuth`, so `oidc.OwnerClaim` is in scope.)

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./internal/auth/ ./cmd/engram/ -v` and `go build ./...`
Expected: new tests PASS; existing `auth_test.go` assertion on `Extra["sub"]` still PASS; build succeeds.

- [ ] **Step 8: Commit**

`jj commit -m "feat(auth): bearer lane stamps owner_claim + enforces email_verified (engram-8bsz)"`

---

### Task 3: Pivot — converge both lanes on `owner_claim`

**Files:**

- Modify: `internal/server/identity.go:23-24` (`SubjectFromTokenInfo` reads `owner_claim`)
- Modify: `internal/webauth/oidc.go` (`NewAuthenticator`, `exchange`)
- Modify: `internal/webauth/session.go:25-28` (`Session.Sub` → `Session.Owner`)
- Modify: `internal/webauth/handlers.go:138,145-146` (callback binds + seals `Owner`)
- Modify: `internal/webauth/resolver.go:49,52` (emit `Extra["owner_claim"]`)
- Modify: `internal/store/subject.go` (doc comments only)
- Modify: `cmd/engram/serve.go:126` (`webauth.NewAuthenticator` gets owner-claim)
- Test: `internal/server/identity_test.go`, plus the direct-`TokenInfo` stub sweep across `internal/server/*_test.go` and `internal/webauth/*_test.go`

This is the atomic pivot. After Task 2 the bearer lane already emits `owner_claim`; this task converts the cookie lane and flips the read seam in the same commit so no lane is ever broken between commits.

- [ ] **Step 1: Write the failing read-seam test**

Replace/extend `internal/server/identity_test.go` (the existing `Extra["sub"]` stub at line 19 must move to `owner_claim`):

```go
func TestSubjectFromTokenInfo(t *testing.T) {
	// Authenticated: owner_claim present → Authenticated(value).
	ti := &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "u1@example.com"}}
	subj, err := SubjectFromTokenInfo(ti)
	if err != nil {
		t.Fatalf("authenticated: %v", err)
	}
	if subj.Owner() != "u1@example.com" {
		t.Errorf("Owner() = %q, want u1@example.com", subj.Owner())
	}

	// Missing/empty owner_claim → fail closed.
	for name, ex := range map[string]map[string]any{
		"absent": {"sub": "x"},
		"empty":  {"owner_claim": ""},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := SubjectFromTokenInfo(&mcpauth.TokenInfo{Extra: ex}); err == nil {
				t.Error("expected fail-closed error")
			}
		})
	}

	// nil TokenInfo (auth disabled) → anonymous.
	subj, err = SubjectFromTokenInfo(nil)
	if err != nil {
		t.Fatalf("nil: %v", err)
	}
	if subj.Owner() != "" {
		t.Errorf("anonymous Owner() = %q, want \"\"", subj.Owner())
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/server/ -run TestSubjectFromTokenInfo -v`
Expected: FAIL — seam still reads `Extra["sub"]`.

- [ ] **Step 3: Flip the read seam**

In `internal/server/identity.go`, change the authenticated branch of `SubjectFromTokenInfo`:

```go
func SubjectFromTokenInfo(ti *mcpauth.TokenInfo) (store.Subject, error) {
	if ti == nil {
		return store.Anonymous(), nil
	}
	if v, ok := ti.Extra["owner_claim"].(string); ok && v != "" {
		return store.Authenticated(v), nil
	}
	return nil, fmt.Errorf("validated token missing owner claim")
}
```

Update the doc comment on the function (it says "missing/empty sub fails closed") to "missing/empty owner-claim value fails closed".

- [ ] **Step 4: Convert the webauth OIDC exchange**

In `internal/webauth/oidc.go`, add `ownerClaim` to the authenticator and extract it in `exchange`:

```go
type Authenticator struct {
	clientID     string
	clientSecret string
	redirectURL  string
	ownerClaim   string
	endpoint     oauth2.Endpoint
	verifier     *oidc.IDTokenVerifier
}

func NewAuthenticator(ctx context.Context, issuer, clientID, clientSecret, redirectURL, ownerClaim string) (*Authenticator, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	return &Authenticator{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		ownerClaim:   ownerClaim,
		endpoint:     provider.Endpoint(),
		verifier:     provider.Verifier(&oidc.Config{ClientID: clientID}),
	}, nil
}
```

Change `exchange` to return the owner-claim value (rename the second return from the subject), enforcing `email_verified`:

```go
// exchange trades an auth code (with its PKCE verifier) for tokens, verifies the
// ID token, and returns the configured owner-claim value (the authz key sealed
// into the session cookie).
func (a *Authenticator) exchange(ctx context.Context, code, verifier string) (*oauth2.Token, string, error) {
	tok, err := a.oauthConfig().Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, "", fmt.Errorf("code exchange: %w", err)
	}
	rawID, ok := tok.Extra("id_token").(string)
	if !ok || rawID == "" {
		return nil, "", fmt.Errorf("token response missing id_token")
	}
	idTok, err := a.verifier.Verify(ctx, rawID)
	if err != nil {
		return nil, "", fmt.Errorf("verify id_token: %w", err)
	}
	var raw map[string]any
	_ = idTok.Claims(&raw)
	// Reuse the bearer lane's pure helper for claim selection + email_verified
	// enforcement (import "github.com/seanb4t/engram/internal/auth").
	owner, _, _, err := auth.ClaimIdentity(raw, a.ownerClaim)
	if err != nil {
		return nil, "", err
	}
	// Unlike the bearer lane (which defers to SubjectFromTokenInfo), the cookie
	// lane seals identity now, so an empty owner must fail closed here.
	if owner == "" {
		return nil, "", fmt.Errorf("verified id_token missing owner claim %q", a.ownerClaim)
	}
	return tok, owner, nil
}
```

(Add `"github.com/seanb4t/engram/internal/auth"` to `internal/webauth/oidc.go` imports. No import cycle: `auth` does not import `webauth`.)

- [ ] **Step 5: Rename `Session.Sub` → `Session.Owner`**

In `internal/webauth/session.go`:

```go
// Session is the decrypted payload of the engram session cookie. Owner is the
// authz key (the configured owner-claim value, default email); Expiry bounds the
// session lifetime.
type Session struct {
	Owner  string    `json:"owner"`
	Expiry time.Time `json:"exp"`
}
```

(Cookie-compat: old sealed cookies carry the `"sub"` JSON key, so they unmarshal to an empty `Owner` and are rejected by the resolver — a one-time forced re-login on upgrade. Expected; documented in Task 6 release notes.)

- [ ] **Step 6: Update the callback handler**

In `internal/webauth/handlers.go` (~138-148):

```go
	_, owner, err := h.auth.exchange(r.Context(), code, fs.Verifier)
	if err != nil {
		slog.WarnContext(r.Context(), "oauth callback exchange failed", "err", err)
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	sealed, err := h.codec.Seal(Session{
		Owner:  owner,
		Expiry: nowUTC().Add(sessionTTL),
	})
```

- [ ] **Step 7: Update the resolver to emit `owner_claim`**

In `internal/webauth/resolver.go` (~49-52):

```go
	if sess.Owner == "" {
		return nil, fmt.Errorf("session has empty owner")
	}
	return &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": sess.Owner}}, nil
```

Update the `Resolver` doc comment ("the cookie's verified sub is the only identity") to "the cookie's verified owner-claim value".

- [ ] **Step 8: Wire serve.go:126**

In `cmd/engram/serve.go`, the `webauth.NewAuthenticator` call (`cfg` is in scope in `runServe`):

```go
		authr, err := webauth.NewAuthenticator(oidcCtx, uiCfg.Issuer, uiCfg.ClientID, uiCfg.ClientSecret, uiCfg.RedirectURL, cfg.OIDC.OwnerClaim)
```

- [ ] **Step 9: Update `subject.go` doc comments (no behavior change)**

In `internal/store/subject.go`, update the comments/panic wording that say "OIDC sub" to reflect the value is now the resolved owner-claim value. The internal field name `sub` stays (no churn):

```go
// Authenticated wraps the caller's resolved owner-claim value (default email),
// the authorization key written onto Memory.Owner. Panics on empty — callers
// (SubjectFromTokenInfo) must fail closed before reaching here.
func Authenticated(sub string) Subject {
	if sub == "" {
		panic("store.Authenticated: owner value must be non-empty")
	}
	...
}
```

- [ ] **Step 10: Migrate every direct-`TokenInfo` / `Session` test stub**

Run the authoritative sweeps and migrate each **construction** site from `sub` to `owner_claim` (and `Session{Sub:…}` → `Session{Owner:…}`):

Run: `rg -n 'Extra.*"sub"' internal/**/*_test.go`
Run: `rg -n '\.Sub\b|"sub"|Session\{' internal/webauth/*_test.go`

Migrate (current sites — treat the `rg` output as authoritative, new tests may have landed):
- `internal/server/tools_test.go:214` — the `authedContext()` helper: `Extra: map[string]any{"owner_claim": sub}` (backs ~20 handler tests; fix once here).
- `internal/server/connectauth_test.go:17`, `internal/server/connectapi_test.go:67,177,214`, `internal/server/connectapi_cookie_test.go:47,49` — change the injected key `"sub"` → `"owner_claim"`.
- `internal/webauth/*_test.go` — `Session{Sub:…}` → `Session{Owner:…}`; resolver tests asserting `Extra["sub"]` → `Extra["owner_claim"]`; `exchange` tests now assert the returned owner value + `email_verified` rejection; add a test that an old `"sub"`-keyed sealed cookie is rejected (forced re-login).

**Exception (do NOT change):** the **assertion** at `internal/auth/auth_test.go:97-98` on `Extra["sub"]` stays valid — `auth.go` still populates `sub` alongside `owner_claim`.

- [ ] **Step 11: Run the full affected suites**

Run: `go build ./... && go test ./internal/server/ ./internal/webauth/ ./internal/auth/ -v`
Expected: all PASS; both lanes resolve via `owner_claim`.

- [ ] **Step 12: Commit**

`jj commit -m "feat(auth): resolve owner from configurable claim across both lanes (engram-8bsz)"`

---

### Task 4: `Store.RemapOwner` + `OwnerRemapSource`

**Files:**

- Modify: `internal/store/store.go` (add `OwnerRemapSource`, `RemapOwner`; update `Memory.Owner` doc at line 84)
- Test: `internal/store/store_test.go`

Mirrors `MigrateSetOwner` (Count-then-`SetPayload`-by-filter, bounded/cancellable). Validation lives in the method.

- [ ] **Step 1: Write the failing tests**

Add to `internal/store/store_test.go` (mirror `TestMigrateSetOwner`'s `testStore`/`payload`/`DeleteAllRaw` patterns):

```go
func TestRemapOwner(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:remap"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	mk := func(id, owner string) {
		if err := s.Upsert(ctx, Memory{ID: id, Content: "c", Scope: scope, Owner: owner, CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	subID := "e0e0e0e0-0000-0000-0000-000000000001"
	emailID := "e0e0e0e0-0000-0000-0000-000000000002"
	mk(subID, "old-sub-123")
	mk(emailID, "old@example.com")

	// Validation: empty `to`, From==to, and zero-value source all error.
	if _, err := s.RemapOwner(ctx, OwnerRemapSource{From: "x"}, "", false); err == nil {
		t.Error("empty to: expected error")
	}
	if _, err := s.RemapOwner(ctx, OwnerRemapSource{From: "a"}, "a", false); err == nil {
		t.Error("From==to: expected error")
	}
	if _, err := s.RemapOwner(ctx, OwnerRemapSource{}, "to", false); err == nil {
		t.Error("zero-value source: expected error")
	}
	if _, err := s.RemapOwner(ctx, OwnerRemapSource{Missing: true, From: "x"}, "to", false); err == nil {
		t.Error("multi-select source: expected error")
	}

	// Dry-run counts without mutating.
	n, err := s.RemapOwner(ctx, OwnerRemapSource{From: "old-sub-123"}, "sean@example.com", true)
	if err != nil || n != 1 {
		t.Fatalf("dry-run: n=%d err=%v, want 1,nil", n, err)
	}
	if got, _ := s.Get(ctx, subID); got.Owner != "old-sub-123" {
		t.Errorf("dry-run mutated owner to %q", got.Owner)
	}

	// sub → email.
	if n, err = s.RemapOwner(ctx, OwnerRemapSource{From: "old-sub-123"}, "sean@example.com", false); err != nil || n != 1 {
		t.Fatalf("sub→email: n=%d err=%v", n, err)
	}
	if got, _ := s.Get(ctx, subID); got.Owner != "sean@example.com" {
		t.Errorf("sub→email owner = %q", got.Owner)
	}

	// email → email.
	if n, err = s.RemapOwner(ctx, OwnerRemapSource{From: "old@example.com"}, "new@example.com", false); err != nil || n != 1 {
		t.Fatalf("email→email: n=%d err=%v", n, err)
	}
	if got, _ := s.Get(ctx, emailID); got.Owner != "new@example.com" {
		t.Errorf("email→email owner = %q", got.Owner)
	}
}

func TestRemapOwnerHonorsCancel(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.RemapOwner(ctx, OwnerRemapSource{From: "x"}, "y", false); err == nil {
		t.Error("cancelled context: expected error")
	}
}
```

> The owner-less (`Missing`) and anonymous-bucket (`Anon`) filters are exercised exactly like `TestMigrateSetOwner` seeds them (raw upsert with `delete(p,"owner")` for missing; `Owner:""` for anon). Add `Missing`/`Anon` sub-cases mirroring that test's seeding if Qdrant is available in CI. Note: `testStore(t)` (used by `TestRemapOwnerHonorsCancel`, mirroring `TestMigrateSetOwnerHonorsCancel`) **skips when Qdrant is unavailable**, so the cancel test provides signal in CI but not in a backend-less local run — that matches the existing cancel test's behavior and is acceptable.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/store/ -run 'TestRemapOwner' -v`
Expected: FAIL — `OwnerRemapSource` / `RemapOwner` undefined.

- [ ] **Step 3: Implement the type and method**

Add to `internal/store/store.go` (near `MigrateSetOwner`):

```go
// OwnerRemapSource selects which records RemapOwner re-stamps. Exactly one of
// the three must be set (validated in RemapOwner): Missing matches owner-less
// (pre-isolation) records via IsEmpty; Anon matches the explicit anonymous
// bucket (owner==""); From matches a specific current owner value (a sub or an
// email). Missing and Anon are distinct because IsEmpty("owner") and
// Match("owner","") target different record sets.
type OwnerRemapSource struct {
	Missing bool
	Anon    bool
	From    string
}

// RemapOwner re-stamps owner=<selected source> → owner=to across the WHOLE
// collection (operator sweep; no subject authz, like PruneExpired). dryRun
// returns the matched count without writing. Validation runs before any Qdrant
// call. Non-transactional: the reported count is a best-effort snapshot taken
// just before the filtered SetPayload (which is itself exact); concurrent writes
// can drift the tally. Idempotent: re-running after a successful remap matches 0.
func (s *Store) RemapOwner(ctx context.Context, src OwnerRemapSource, to string, dryRun bool) (n uint64, err error) {
	ctx, span := tracer.Start(ctx, "store.RemapOwner")
	defer span.End()
	start := time.Now()
	defer func() {
		telemetry.RecordStoreOp(ctx, "RemapOwner", start, err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetAttributes(attribute.Int64("engram.result_count", int64(n)))
		}
	}()

	if to == "" {
		return 0, fmt.Errorf("to must be non-empty")
	}
	selected := 0
	if src.Missing {
		selected++
	}
	if src.Anon {
		selected++
	}
	if src.From != "" {
		selected++
	}
	if selected != 1 {
		return 0, fmt.Errorf("exactly one source required (Missing | Anon | From)")
	}
	if src.From != "" && src.From == to {
		return 0, fmt.Errorf("from and to are identical (%q)", to)
	}

	var filter *qdrant.Filter
	switch {
	case src.Missing:
		filter = ownerlessFilter()
	case src.Anon:
		filter = &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("owner", "")}}
	default:
		filter = &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("owner", src.From)}}
	}

	cnt, err := s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: s.collection, Filter: filter, Exact: qdrant.PtrOf(true),
	})
	if err != nil {
		return 0, err
	}
	if cnt == 0 || dryRun {
		return cnt, nil
	}
	if _, err = s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection, Wait: qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{"owner": to}),
		PointsSelector: qdrant.NewPointsSelectorFilter(filter),
	}); err != nil {
		return 0, err
	}
	return cnt, nil
}
```

- [ ] **Step 4: Update the `Memory.Owner` doc comment**

In `internal/store/store.go` (the `Owner` field comment at lines 82-85), change it from "the stable OIDC subject (`sub`)" to:

```go
	// Owner is the caller's configured owner-claim value (default email) — the
	// authorization key. Server-set, never client-supplied.
	Owner string `json:"owner"`
```

- [ ] **Step 5: Run to verify pass**

Run: `go test ./internal/store/ -run 'TestRemapOwner' -v`
Expected: PASS (cancel test always; the Qdrant-backed cases pass when a backend is available — same gating as `TestMigrateSetOwner`).

- [ ] **Step 6: Commit**

`jj commit -m "feat(store): RemapOwner general owner remap (missing|anon|from→to, dry-run) (engram-8bsz)"`

---

### Task 5: `migrate-remap-owner` CLI + deprecate `migrate-set-owner`

**Files:**

- Modify: `cmd/engram/migrate.go` (new command; deprecate the old one)
- Modify: `internal/server/tools.go` (`warnOwnerlessRecords` message → point at new verb)
- Test: `cmd/engram/migrate_test.go` (create if absent; mirror `prune_test.go`)

- [ ] **Step 1: Write the failing flag-validation test**

Create/extend `cmd/engram/migrate_test.go`:

```go
func TestRemapOwnerFlagValidation(t *testing.T) {
	// Build the source from flags via the helper the command uses; assert
	// mutual-exclusion and required --to are caught before any store call.
	cases := []struct {
		name              string
		from              string
		missing, anon     bool
		to                string
		wantErr           bool
	}{
		{"no source", "", false, false, "x", true},
		{"missing ok", "", true, false, "x", false},
		{"anon ok", "", false, true, "x", false},
		{"from ok", "old", false, false, "x", false},
		{"two sources", "old", true, false, "x", true},
		{"empty to", "", true, false, "", true},
		{"ambiguous empty from", "", false, false, "x", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := buildRemapSource(c.from, c.missing, c.anon, c.to)
			if (err != nil) != c.wantErr {
				t.Errorf("buildRemapSource(%+v) err=%v, wantErr=%v", c, err, c.wantErr)
			}
		})
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./cmd/engram/ -run TestRemapOwnerFlagValidation -v`
Expected: FAIL — `buildRemapSource` undefined.

- [ ] **Step 3: Implement the command + helper**

In `cmd/engram/migrate.go`, add flags and the new command (keep the existing `migrateSetOwnerCmd`):

```go
var (
	remapFrom     string
	remapMissing  bool
	remapAnon     bool
	remapTo       string
	remapDryRun   bool
	remapTimeout  time.Duration
)

// buildRemapSource validates the mutually-exclusive source flags and required
// --to, returning the store source. Pure (no I/O) so it is unit-testable.
func buildRemapSource(from string, missing, anon bool, to string) (store.OwnerRemapSource, error) {
	if to == "" {
		return store.OwnerRemapSource{}, fmt.Errorf("--to is required and must be non-empty")
	}
	selected := 0
	if missing {
		selected++
	}
	if anon {
		selected++
	}
	if from != "" {
		selected++
	}
	if selected != 1 {
		return store.OwnerRemapSource{}, fmt.Errorf("exactly one source required: --from <value> | --from-missing | --from-anon")
	}
	return store.OwnerRemapSource{Missing: missing, Anon: anon, From: from}, nil
}

var migrateRemapOwnerCmd = &cobra.Command{
	Use:   "migrate-remap-owner",
	Short: "Re-stamp record owner across the collection (sub→email, email→email, owner-less, or anonymous bucket)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		src, err := buildRemapSource(remapFrom, remapMissing, remapAnon, remapTo)
		if err != nil {
			return err
		}
		st, err := server.StoreFromEnv()
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if remapTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, remapTimeout)
			defer cancel()
		}
		n, err := st.RemapOwner(ctx, src, remapTo, remapDryRun)
		if err != nil {
			return err
		}
		if remapDryRun {
			cmd.Printf("[dry-run] would remap %d record(s) to owner=%s\n", n, remapTo)
		} else {
			cmd.Printf("remapped %d record(s) to owner=%s\n", n, remapTo)
		}
		return nil
	},
}
```

Register the flags and deprecate the old command in `init()`:

```go
	migrateRemapOwnerCmd.Flags().StringVar(&remapFrom, "from", "", "current owner value to remap (a sub or email); mutually exclusive with --from-missing/--from-anon")
	migrateRemapOwnerCmd.Flags().BoolVar(&remapMissing, "from-missing", false, "remap owner-less (pre-isolation) records")
	migrateRemapOwnerCmd.Flags().BoolVar(&remapAnon, "from-anon", false, "remap the explicit anonymous bucket (owner==\"\")")
	migrateRemapOwnerCmd.Flags().StringVar(&remapTo, "to", "", "new owner value to stamp (required)")
	migrateRemapOwnerCmd.Flags().BoolVar(&remapDryRun, "dry-run", false, "count matching records without writing")
	migrateRemapOwnerCmd.Flags().DurationVar(&remapTimeout, "timeout", 5*time.Minute, "max wall-clock (0 disables); also cancellable via Ctrl-C")
	rootCmd.AddCommand(migrateRemapOwnerCmd)

	// migrate-set-owner is now a deprecated alias for the owner-less case.
	migrateSetOwnerCmd.Deprecated = "use: migrate-remap-owner --from-missing --to <owner>"
```

(`store` must be imported in `migrate.go`: add `"github.com/seanb4t/engram/internal/store"`.)

- [ ] **Step 4: Update the startup warning text**

In `internal/server/tools.go`, update **both** references to `migrate-set-owner` to name the new verb: the `warnOwnerlessRecords` slog message (line ~244) and the doc comment above it (line ~234) / the `ensureStoreFromConfig` comment (line ~109). The message becomes:

```go
		slog.Warn("pre-isolation records have no owner — invisible to reads and not removable by delete_all until you run: engram migrate-remap-owner --from-missing --to <owner>",
			"count", n)
```

- [ ] **Step 5: Run to verify pass**

Run: `go test ./cmd/engram/ -run TestRemapOwnerFlagValidation -v && go build ./...`
Expected: PASS; build succeeds.

- [ ] **Step 6: Commit**

`jj commit -m "feat(cli): migrate-remap-owner verb; deprecate migrate-set-owner (engram-8bsz)"`

---

### Task 6: Docs, Helm chart, breaking-change surface

**Files:**

- Modify: `docs-site/src/content/docs/reference/auth.md` (isolation model)
- Modify: `docs-site/src/content/docs/reference/memory-record.md` (owner field)
- Modify: `docs-site/src/content/docs/guides/configure.md` (config table)
- Modify: `charts/engram/values.yaml` + the deployment env template under `charts/engram/templates/`

- [ ] **Step 1: Docs — auth.md**

In `docs-site/src/content/docs/reference/auth.md`, replace the "Isolation model" statement that "Each authenticated caller is identified by the stable OIDC `sub` claim, stored as the record's `owner`" with: the `owner` is the value of the configured `ENGRAM_OWNER_CLAIM` (default `email`); auth fails closed if the claim is absent; `email_verified` is required when the claim is `email`. Update the "Upgrading an existing deployment" section to use `engram migrate-remap-owner` (note `migrate-set-owner` is deprecated) and document the one-time web-console re-login.

- [ ] **Step 2: Docs — memory-record.md + configure.md**

In `memory-record.md`, change "The `owner` is always the stable OIDC `sub` claim" to the configured-claim wording. In `guides/configure.md`, add a row to the OIDC/Auth table:

```markdown
| `ENGRAM_OWNER_CLAIM` | `--owner-claim` | `email` | OIDC claim whose value becomes the record `owner` (authz key); fail-closed if absent; requires `email_verified` when `email` |
```

- [ ] **Step 3: Helm chart**

In `charts/engram/values.yaml`, add a documented `ownerClaim: ""` (or `email`) under the OIDC/auth values block, and surface it as `ENGRAM_OWNER_CLAIM` in the deployment env template (follow the pattern of `ENGRAM_OIDC_ISSUER`). Default empty → engram's built-in `email` default applies; setting it overrides.

- [ ] **Step 4: Verify docs build + chart lint**

Run: `task lint` (covers rumdl/markdown + chart) and, if present, the docs-site build target; `task chart:lint` or `helm lint charts/engram` if defined in the Taskfile.
Expected: clean.

- [ ] **Step 5: Commit**

`jj commit -m "docs(auth): configurable owner claim + migrate-remap-owner; chart ENGRAM_OWNER_CLAIM (engram-8bsz)"`

---

## Final verification (after all tasks)

- [ ] Run `task` (lint + test) — full suite green.
- [ ] Run `task license:check` — SPDX headers present on any new files.
- [ ] Confirm `BREAKING CHANGE:` trailer is on the implementing commit/PR and the PR opens with the migration-required + forced-re-login callout (per the spec's "Release notes — BREAKING, announce loudly" section). This is an acceptance criterion on the implementing bead.
- [ ] Manual unblock rehearsal (single-user): `engram migrate-remap-owner --from <old-sub> --to <email> --dry-run`, confirm count, then run for real; verify recall returns the records.
<!-- adr-capture: sha256=f4383726375f533a; session=cli; ts=2026-06-29T20:33:49Z; adrs=engram-g37x -->
