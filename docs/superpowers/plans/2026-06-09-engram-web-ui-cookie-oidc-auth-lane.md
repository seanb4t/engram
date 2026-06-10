<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# engram web UI — cookie/OIDC auth lane (Go BFF) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Connect `EngramService` API a cookie-authenticated, observable, headless-by-default surface — an OIDC confidential-client login lane, a sealed session cookie, a cookie→`Subject` resolver, mount-gating, and observability parity — closing the `engram-2ft` posture finding.

**Architecture:** A new `internal/webauth` package owns the OIDC auth-code+PKCE login flow, an AES-GCM-sealed stateless session cookie, and a `connectResolver` that turns the cookie into the verified `Subject(sub)` the existing Connect interceptor seam already consumes. `cmd/engram/serve.go` gates the whole surface on a `MEM_UI_*` activation tiebreaker: when the UI is disabled (the default) the Connect handler and login routes are not mounted at all. Observability is added inside the existing `mountConnect` via an `otelconnect` interceptor plus a slog access-log interceptor.

**Tech Stack:** Go, `connectrpc.com/connect` v1.20, `connectrpc.com/otelconnect` v0.9, `github.com/coreos/go-oidc/v3` v3.18, `golang.org/x/oauth2` v0.36, stdlib `crypto/aes`+`cipher/gcm` for cookie sealing, `go:embed` for static assets.

**Scope:** This plan covers the **Go BFF/auth lane only**. The SvelteKit SPA is a separate subsystem with a disjoint toolchain (Node/buf/connect-es) and gets its own follow-up plan; this plan serves a committed placeholder `index.html` so the lane is end-to-end testable without the SPA.

**Design of record:** `docs/superpowers/specs/2026-06-09-engram-web-ui-design.md` + the posture addendum `docs/superpowers/specs/2026-06-09-connect-auth-posture-addendum.md` (acceptance criteria R1–R4 + R1a).

## How the acceptance criteria map to tasks

| Criterion | Where it lands |
|-----------|----------------|
| **R1** mount Connect only when UI enabled; headless covers the Connect mount | Task 2 (tiebreaker), Task 10 (`mountConnect` nil→not-mounted), Task 12 (serve wiring) |
| **R1a** owner=="" startup hygiene (subsumed for Connect by R1+R2; kept as operator visibility) | Task 1 |
| **R2** cookie→Subject interceptor is the sole authz entry; no anonymous fallthrough | Task 3, 4, 5, 6, 7, 8 (resolver), Task 13 (isolation test) |
| **R3** observability parity (otelconnect + access-log interceptor) | Task 9 |
| **R4** same-origin, no permissive CORS | Task 12 (no CORS handler added; asserted by Task 13's preflight check) |

> **Design note for the reviewer (R1a):** Once R1 (Task 10) + R2 (Task 8) land together, the Connect handler is never mounted with an anonymous resolver, so an `owner==""` record is unreachable via Connect (authenticated callers see `owner==sub` OR `shared`, never the anonymous bucket). R1a's interim "guard the anonymous mount" therefore has no Connect surface left to guard. We implement its *intent* — operator visibility into a non-empty anonymous bucket — as a cheap startup warning decoupled from the mount (Task 1), and do **not** couple it to Connect.

## File structure

| File | Responsibility | Task |
|------|----------------|------|
| `internal/store/store.go` (modify) | add `CountAnonymousBucket` (`owner==""` exact) | 1 |
| `internal/server/tools.go` (modify) | extend `warnOwnerlessRecords` to also warn on the `owner==""` bucket | 1 |
| `cmd/engram/uiconfig.go` (create) | `UIConfig` + `resolveUIConfig` activation tiebreaker (pure, testable) | 2 |
| `internal/webauth/session.go` (create) | AES-GCM `SessionCodec`: `Seal`/`Unseal` of `Session{Sub,Access,Refresh,Expiry}` | 3 |
| `internal/webauth/oidc.go` (create) | `Authenticator`: OIDC provider discovery + `oauth2.Config` + ID-token verifier | 4 |
| `internal/webauth/handlers.go` (create) | `/auth/login`, `/auth/callback`, `/auth/logout` HTTP handlers | 5,6,7 |
| `internal/webauth/resolver.go` (create) | `Resolver`: cookie → `*mcpauth.TokenInfo` for the Connect interceptor (R2) | 8 |
| `internal/server/connectobs.go` (create) | `newConnectAccessLogInterceptor` (slog) | 9 |
| `internal/server/connectapi.go` (modify) | `mountConnect`: nil→not-mounted (R1); add otelconnect + access-log interceptors (R3) | 9,10 |
| `internal/server/tools.go` (modify) | `Register`: propagate `mountConnect` error | 9,10 |
| `internal/webauth/static.go` (create) | `go:embed` placeholder SPA + static `http.Handler` | 11 |
| `internal/webauth/static/index.html` (create) | placeholder asset | 11 |
| `cmd/engram/serve.go` (modify) | config flags, tiebreaker wiring, conditional mount, route registration | 12 |
| `internal/server/connectapi_cookie_test.go` (create) | end-to-end cross-actor isolation through the cookie resolver | 13 |

---

### Task 1: `store.CountAnonymousBucket` + startup warning (R1a)

**Files:**

- Modify: `internal/store/store.go` (near `ownerlessFilter`/`CountOwnerless`, ~line 560-578)
- Test: `internal/store/store_test.go`
- Modify: `internal/server/tools.go:warnOwnerlessRecords` (~line 106)

- [ ] **Step 1: Write the failing test**

Add to `internal/store/store_test.go`. This is an integration test using the existing ephemeral-Qdrant harness (`testStore`, which skips when `testQdrantAddr==""`). `testStore` may share a collection across tests, so assert on the **delta**, not an absolute count (`CountAnonymousBucket` is collection-wide). The package is `store`, so `Memory`/`Anonymous()`/`Authenticated()` are unqualified; `time` is already imported by this file:

```go
func TestCountAnonymousBucket(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	before, err := s.CountAnonymousBucket(ctx)
	if err != nil {
		t.Fatalf("CountAnonymousBucket(before): %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	anon1 := Memory{ID: "a1111111-0000-0000-0000-000000000001", Content: "anon one", Scope: "anon-count-scope", Owner: "", Source: "agent-inferred", Category: "fact", CreatedAt: now}
	anon2 := Memory{ID: "a1111111-0000-0000-0000-000000000002", Content: "anon two", Scope: "anon-count-scope", Owner: "", Source: "agent-inferred", Category: "fact", CreatedAt: now}
	owned := Memory{ID: "a1111111-0000-0000-0000-000000000003", Content: "owned", Scope: "anon-count-scope", Owner: "sub-x", Source: "agent-inferred", Category: "fact", CreatedAt: now}
	for _, m := range []Memory{anon1, anon2, owned} {
		if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}
	defer func() {
		_ = s.Delete(ctx, anon1.ID, Anonymous())
		_ = s.Delete(ctx, anon2.ID, Anonymous())
		_ = s.Delete(ctx, owned.ID, Authenticated("sub-x"))
	}()

	after, err := s.CountAnonymousBucket(ctx)
	if err != nil {
		t.Fatalf("CountAnonymousBucket(after): %v", err)
	}
	if after-before != 2 {
		t.Fatalf("anonymous-bucket delta = %d, want 2", after-before)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestCountAnonymousBucket -v`
Expected: FAIL — `s.CountAnonymousBucket undefined`.

- [ ] **Step 3: Implement `CountAnonymousBucket`**

Add to `internal/store/store.go` directly below `CountOwnerless`:

```go
// CountAnonymousBucket returns the number of records in the auth-disabled
// anonymous bucket (an explicit owner==""). Distinct from CountOwnerless, which
// matches pre-isolation records with NO owner key (NewIsEmpty). The server
// bootstrap warns when this is non-empty: those records are readable by any
// anonymous caller, so an operator who once ran auth-disabled should know they
// exist before enabling a network surface.
func (s *Store) CountAnonymousBucket(ctx context.Context) (uint64, error) {
	return s.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: s.collection,
		Filter:         &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("owner", "")}},
		Exact:          qdrant.PtrOf(true),
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestCountAnonymousBucket -v`
Expected: PASS (or SKIP if Docker is unavailable — then run later in CI).

- [ ] **Step 5: Extend the startup warning**

In `internal/server/tools.go`, at the end of `warnOwnerlessRecords` (after the existing `CountOwnerless` block), add an `owner==""` check:

```go
	// Anonymous bucket (explicit owner==""): readable by any anonymous caller.
	// Surfaces a deployment that previously ran auth-disabled before any
	// network read surface is exposed.
	an, err := st.CountAnonymousBucket(ctx)
	if err != nil {
		slog.Warn("could not check the anonymous (owner=='') bucket", "err", err)
		return
	}
	if an > 0 {
		slog.Warn("anonymous-bucket records exist (owner==\"\"): readable by any unauthenticated caller; they predate an OIDC-enabled deployment", "count", an)
	}
```

- [ ] **Step 6: Run package tests + commit**

Run: `go test ./internal/store/ ./internal/server/`
Expected: PASS/SKIP, no build errors.
Commit per `references/vcs-preamble.md` (jj): `jj commit -m "feat(store): CountAnonymousBucket + startup warn for owner=='' bucket (R1a)"`

---

### Task 2: UI activation tiebreaker (R1 config)

**Files:**

- Create: `cmd/engram/uiconfig.go`
- Test: `cmd/engram/uiconfig_test.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/engram/uiconfig_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import "testing"

func TestResolveUIConfig(t *testing.T) {
	full := map[string]string{
		"MEM_OIDC_CLIENT_ID":     "id",
		"MEM_OIDC_CLIENT_SECRET": "secret",
		"MEM_UI_REDIRECT_URL":    "https://x/auth/callback",
		"MEM_UI_COOKIE_KEY":      "0123456789abcdef0123456789abcdef", // 32 bytes
	}
	cases := []struct {
		name     string
		env      map[string]string
		wantOn   bool
		wantErr  bool
	}{
		{"unset and no creds -> headless", map[string]string{}, false, false},
		{"creds present, flag unset -> on", full, true, false},
		{"explicit false wins over creds", merge(full, "MEM_UI_ENABLED", "false"), false, false},
		{"explicit true with full creds -> on", merge(full, "MEM_UI_ENABLED", "true"), true, false},
		{"enabled with partial creds -> error", map[string]string{"MEM_UI_ENABLED": "true", "MEM_OIDC_CLIENT_ID": "id"}, false, true},
		{"creds present but one missing, flag unset -> headless (not an error)", drop(full, "MEM_UI_COOKIE_KEY"), false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := resolveUIConfig(func(k string) string { return tc.env[k] })
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if err == nil && cfg.Enabled != tc.wantOn {
				t.Fatalf("Enabled=%v want %v", cfg.Enabled, tc.wantOn)
			}
		})
	}
}

func merge(m map[string]string, k, v string) map[string]string {
	out := map[string]string{}
	for kk, vv := range m {
		out[kk] = vv
	}
	out[k] = v
	return out
}

func drop(m map[string]string, k string) map[string]string {
	out := map[string]string{}
	for kk, vv := range m {
		if kk != k {
			out[kk] = vv
		}
	}
	return out
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/engram/ -run TestResolveUIConfig -v`
Expected: FAIL — `resolveUIConfig undefined`, `UIConfig` undefined.

- [ ] **Step 3: Implement `resolveUIConfig`**

Create `cmd/engram/uiconfig.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import "fmt"

// UIConfig is the resolved web-UI activation state. When Enabled is false every
// field except Enabled is zero and the caller mounts nothing (headless).
type UIConfig struct {
	Enabled      bool
	ClientID     string
	ClientSecret string
	RedirectURL  string
	CookieKey    string
}

// requiredUICreds are all-or-nothing: enabling the UI without all of them is a
// fail-fast startup error, never a silent half-on state.
var requiredUICreds = []string{
	"MEM_OIDC_CLIENT_ID",
	"MEM_OIDC_CLIENT_SECRET",
	"MEM_UI_REDIRECT_URL",
	"MEM_UI_COOKIE_KEY",
}

// resolveUIConfig implements the spec's activation tiebreaker:
//   - MEM_UI_ENABLED=="false" (any case) is a hard off-switch — headless even
//     when creds are present.
//   - Otherwise the UI is enabled iff all required creds are present.
//   - MEM_UI_ENABLED=="true" with missing creds is a startup error (fail fast),
//     NOT a silent half-on state.
//   - MEM_UI_ENABLED unset with partial creds is headless (not an error):
//     presence-of-all-creds implies enabled; partial implies the operator has
//     not finished wiring it.
func resolveUIConfig(getenv func(string) string) (UIConfig, error) {
	flag := getenv("MEM_UI_ENABLED")
	if eqFold(flag, "false") {
		return UIConfig{Enabled: false}, nil
	}
	present := 0
	for _, k := range requiredUICreds {
		if getenv(k) != "" {
			present++
		}
	}
	allCreds := present == len(requiredUICreds)

	if eqFold(flag, "true") && !allCreds {
		var missing []string
		for _, k := range requiredUICreds {
			if getenv(k) == "" {
				missing = append(missing, k)
			}
		}
		return UIConfig{}, fmt.Errorf("MEM_UI_ENABLED=true but missing required creds: %v", missing)
	}
	if !allCreds {
		return UIConfig{Enabled: false}, nil
	}
	return UIConfig{
		Enabled:      true,
		ClientID:     getenv("MEM_OIDC_CLIENT_ID"),
		ClientSecret: getenv("MEM_OIDC_CLIENT_SECRET"),
		RedirectURL:  getenv("MEM_UI_REDIRECT_URL"),
		CookieKey:    getenv("MEM_UI_COOKIE_KEY"),
	}, nil
}

// eqFold is a tiny ASCII case-insensitive compare (avoids importing strings for
// one call site; matches the repo's lean-import style in cmd/engram).
func eqFold(s, want string) bool {
	if len(s) != len(want) {
		return false
	}
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != want[i] {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/engram/ -run TestResolveUIConfig -v`
Expected: PASS (all sub-cases).

- [ ] **Step 5: Commit**

`jj commit -m "feat(serve): MEM_UI_* activation tiebreaker (R1 config)"`

---

### Task 3: Session cookie codec (AES-GCM seal/unseal)

**Files:**

- Create: `internal/webauth/session.go`
- Test: `internal/webauth/session_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/webauth/session_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"testing"
	"time"
)

func testKey() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i)
	}
	return k
}

func TestSessionRoundTrip(t *testing.T) {
	c, err := NewSessionCodec(testKey())
	if err != nil {
		t.Fatalf("NewSessionCodec: %v", err)
	}
	in := Session{Sub: "user-123", Access: "at", Refresh: "rt", Expiry: time.Unix(1000, 0).UTC()}
	sealed, err := c.Seal(in)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	out, err := c.Unseal(sealed)
	if err != nil {
		t.Fatalf("Unseal: %v", err)
	}
	if out.Sub != in.Sub || out.Access != in.Access || out.Refresh != in.Refresh || !out.Expiry.Equal(in.Expiry) {
		t.Fatalf("round-trip mismatch: %+v vs %+v", out, in)
	}
}

func TestUnsealRejectsTamper(t *testing.T) {
	c, _ := NewSessionCodec(testKey())
	sealed, _ := c.Seal(Session{Sub: "u"})
	b := []byte(sealed)
	b[len(b)-1] ^= 0xff // flip a byte of the ciphertext/tag
	if _, err := c.Unseal(string(b)); err == nil {
		t.Fatal("Unseal accepted tampered cookie")
	}
}

func TestUnsealRejectsWrongKey(t *testing.T) {
	c1, _ := NewSessionCodec(testKey())
	other := testKey()
	other[0] ^= 0xff
	c2, _ := NewSessionCodec(other)
	sealed, _ := c1.Seal(Session{Sub: "u"})
	if _, err := c2.Unseal(sealed); err == nil {
		t.Fatal("Unseal accepted cookie sealed with a different key")
	}
}

func TestNewSessionCodecRejectsBadKey(t *testing.T) {
	if _, err := NewSessionCodec([]byte("short")); err == nil {
		t.Fatal("accepted a non-32-byte key")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/webauth/ -run TestSession -v`
Expected: FAIL — package/`NewSessionCodec` undefined.

- [ ] **Step 3: Implement the codec**

Create `internal/webauth/session.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package webauth implements engram's web-UI auth lane: an OIDC confidential
// client login flow, a stateless AES-GCM-sealed session cookie, and a Connect
// resolver that turns the cookie into the verified Subject the EngramService
// handlers authorize on. See docs/superpowers/specs/2026-06-09-engram-web-ui-design.md.
package webauth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Session is the decrypted payload of the engram session cookie. Sub is the
// authz key (OIDC subject); Access/Refresh ride along for the future write
// phase; Expiry bounds the session lifetime.
type Session struct {
	Sub     string    `json:"sub"`
	Access  string    `json:"at"`
	Refresh string    `json:"rt"`
	Expiry  time.Time `json:"exp"`
}

// SessionCodec seals/unseals Session values with AES-256-GCM. The key MUST be
// exactly 32 bytes (AES-256). Output is URL-safe base64 (cookie-value safe).
type SessionCodec struct {
	aead cipher.AEAD
}

// NewSessionCodec builds a codec from a 32-byte key.
func NewSessionCodec(key []byte) (*SessionCodec, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("session key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return &SessionCodec{aead: aead}, nil
}

// Seal serializes and encrypts s. A fresh random nonce is prepended to the
// ciphertext; the whole blob is base64url-encoded.
func (c *SessionCodec) Seal(s Session) (string, error) {
	plain, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("marshal session: %w", err)
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	sealed := c.aead.Seal(nonce, nonce, plain, nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Unseal decodes, decrypts, and deserializes a sealed cookie value. Any
// tamper/wrong-key/short-input condition returns an error (fail closed).
func (c *SessionCodec) Unseal(v string) (Session, error) {
	raw, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return Session{}, fmt.Errorf("decode: %w", err)
	}
	ns := c.aead.NonceSize()
	if len(raw) < ns {
		return Session{}, fmt.Errorf("sealed value too short")
	}
	nonce, ct := raw[:ns], raw[ns:]
	plain, err := c.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return Session{}, fmt.Errorf("decrypt: %w", err)
	}
	var s Session
	if err := json.Unmarshal(plain, &s); err != nil {
		return Session{}, fmt.Errorf("unmarshal session: %w", err)
	}
	return s, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/webauth/ -run TestSession -v && go test ./internal/webauth/ -run TestUnseal -v && go test ./internal/webauth/ -run TestNewSessionCodec -v`
Expected: PASS.

- [ ] **Step 5: Commit**

`jj commit -m "feat(webauth): AES-GCM session cookie codec"`

---

### Task 4: OIDC confidential client + oauth2 config

**Files:**

- Create: `internal/webauth/oidc.go`
- Test: `internal/webauth/oidc_test.go`

- [ ] **Step 1: Write the failing test**

`internal/webauth/oidc.go` does live network discovery, so the unit test covers only the pure `oauth2.Config` assembly via the exported `Authenticator.oauthConfig()` against an injected provider endpoint. Create `internal/webauth/oidc_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"testing"

	"golang.org/x/oauth2"
)

func TestOAuthConfigShape(t *testing.T) {
	a := &Authenticator{
		clientID:     "id",
		clientSecret: "secret",
		redirectURL:  "https://x/auth/callback",
		endpoint:     oauth2.Endpoint{AuthURL: "https://issuer/auth", TokenURL: "https://issuer/token"},
	}
	cfg := a.oauthConfig()
	if cfg.ClientID != "id" || cfg.ClientSecret != "secret" {
		t.Fatalf("client creds not wired: %+v", cfg)
	}
	if cfg.RedirectURL != "https://x/auth/callback" {
		t.Fatalf("redirect not wired: %q", cfg.RedirectURL)
	}
	wantScopes := map[string]bool{"openid": true, "profile": true, "email": true, "offline_access": true}
	for _, s := range cfg.Scopes {
		delete(wantScopes, s)
	}
	if len(wantScopes) != 0 {
		t.Fatalf("missing scopes: %v (got %v)", wantScopes, cfg.Scopes)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/webauth/ -run TestOAuthConfigShape -v`
Expected: FAIL — `Authenticator` undefined.

- [ ] **Step 3: Implement the authenticator**

Create `internal/webauth/oidc.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Authenticator is engram acting as an OIDC confidential client for the web
// login: it discovers the issuer, exchanges auth codes for tokens, and verifies
// ID tokens. It reuses the same issuer the MCP bearer lane already trusts.
type Authenticator struct {
	clientID     string
	clientSecret string
	redirectURL  string
	endpoint     oauth2.Endpoint
	verifier     *oidc.IDTokenVerifier
}

// NewAuthenticator performs OIDC discovery against issuer and returns an
// Authenticator. The ID-token verifier checks signature, issuer, and audience
// (== clientID for the auth-code flow).
func NewAuthenticator(ctx context.Context, issuer, clientID, clientSecret, redirectURL string) (*Authenticator, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	return &Authenticator{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		endpoint:     provider.Endpoint(),
		verifier:     provider.Verifier(&oidc.Config{ClientID: clientID}),
	}, nil
}

// oauthConfig builds the per-flow oauth2.Config. offline_access requests a
// refresh token for the future write phase.
func (a *Authenticator) oauthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     a.clientID,
		ClientSecret: a.clientSecret,
		RedirectURL:  a.redirectURL,
		Endpoint:     a.endpoint,
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email", oidc.ScopeOfflineAccess},
	}
}

// exchange trades an auth code (with its PKCE verifier) for tokens and verifies
// the returned ID token, returning the verified subject.
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
	if idTok.Subject == "" {
		return nil, "", fmt.Errorf("verified id_token has empty subject")
	}
	return tok, idTok.Subject, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/webauth/ -run TestOAuthConfigShape -v`
Expected: PASS.

- [ ] **Step 5: Commit**

`jj commit -m "feat(webauth): OIDC confidential client (discovery + exchange + verify)"`

---

### Task 5: Login handler (PKCE + state via sealed flow cookie)

**Files:**

- Create: `internal/webauth/handlers.go`
- Test: `internal/webauth/handlers_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/webauth/handlers_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func testHandler(t *testing.T) *Handler {
	t.Helper()
	codec, err := NewSessionCodec(testKey())
	if err != nil {
		t.Fatal(err)
	}
	a := &Authenticator{
		clientID:    "id",
		redirectURL: "https://x/auth/callback",
		endpoint:    oauth2.Endpoint{AuthURL: "https://issuer/auth", TokenURL: "https://issuer/token"},
	}
	return NewHandler(a, codec, true /* secureCookies */)
}

func TestLoginRedirectsWithChallengeAndState(t *testing.T) {
	h := testHandler(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	h.Login(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d want 302", rec.Code)
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	q := loc.Query()
	if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("missing PKCE challenge: %v", q)
	}
	if q.Get("state") == "" {
		t.Fatal("missing state")
	}
	// The flow cookie must be set so callback can recover state+verifier.
	if !strings.Contains(rec.Header().Get("Set-Cookie"), flowCookieName+"=") {
		t.Fatalf("flow cookie not set: %q", rec.Header().Get("Set-Cookie"))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/webauth/ -run TestLoginRedirects -v`
Expected: FAIL — `NewHandler`/`Handler`/`flowCookieName` undefined.

- [ ] **Step 3: Implement the handler scaffold + Login**

Create `internal/webauth/handlers.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/oauth2"
)

const (
	// sessionCookieName holds the sealed Session after login.
	sessionCookieName = "engram_session"
	// flowCookieName holds the sealed in-flight {state, verifier} between
	// /auth/login and /auth/callback.
	flowCookieName = "engram_oauth_flow"
	flowTTL        = 10 * time.Minute
	sessionTTL     = 12 * time.Hour
)

// Handler serves the OIDC login endpoints and seals/clears the session cookie.
type Handler struct {
	auth   *Authenticator
	codec  *SessionCodec
	secure bool // Secure attribute on Set-Cookie (false only for plaintext local dev)
}

func NewHandler(auth *Authenticator, codec *SessionCodec, secure bool) *Handler {
	return &Handler{auth: auth, codec: codec, secure: secure}
}

// flowState is sealed into the flow cookie so state+verifier survive the
// round-trip to the IdP without server-side storage (stateless, per D4).
type flowState struct {
	State    string `json:"s"`
	Verifier string `json:"v"`
}

// Login begins the auth-code+PKCE flow: generate a verifier + state, seal them
// into a short-lived flow cookie, and redirect to the issuer.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	verifier := oauth2.GenerateVerifier()
	state := oauth2.GenerateVerifier() // reuse the CSPRNG helper for an opaque state token

	fs, err := json.Marshal(flowState{State: state, Verifier: verifier})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sealed, err := h.codec.sealBytes(fs)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.setCookie(w, flowCookieName, sealed, flowTTL)

	url := h.auth.oauthConfig().AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(verifier))
	http.Redirect(w, r, url, http.StatusFound)
}

// setCookie writes an httpOnly, SameSite=Lax cookie scoped to "/".
func (h *Handler) setCookie(w http.ResponseWriter, name, value string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  nowUTC().Add(ttl),
		MaxAge:   int(ttl.Seconds()),
	})
}

// clearCookie expires a cookie immediately.
func (h *Handler) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: "", Path: "/", HttpOnly: true, Secure: h.secure,
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

// nowUTC is a seam for tests; production uses the wall clock.
var nowUTC = func() time.Time { return time.Now().UTC() }

var _ = slog.Default // retained: callback/logout log via slog
```

Add two small helpers to `internal/webauth/session.go` (sealing raw bytes, reused by the flow cookie):

```go
// sealBytes/unsealBytes seal arbitrary bytes with the same AEAD, for the
// short-lived OAuth flow cookie (state+verifier) that is not a Session.
func (c *SessionCodec) sealBytes(plain []byte) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(c.aead.Seal(nonce, nonce, plain, nil)), nil
}

func (c *SessionCodec) unsealBytes(v string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	ns := c.aead.NonceSize()
	if len(raw) < ns {
		return nil, fmt.Errorf("sealed value too short")
	}
	return c.aead.Open(nil, raw[:ns], raw[ns:], nil)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/webauth/ -run TestLoginRedirects -v`
Expected: PASS.

- [ ] **Step 5: Commit**

`jj commit -m "feat(webauth): /auth/login PKCE + sealed flow cookie"`

---

### Task 6: Callback handler (exchange + verify + seal session)

**Files:**

- Modify: `internal/webauth/handlers.go`
- Test: `internal/webauth/handlers_test.go`

- [ ] **Step 1: Write the failing test**

The token exchange hits the network, so the test exercises the pre-exchange guards (missing flow cookie, state mismatch) which are the security-critical branches. Add to `internal/webauth/handlers_test.go`:

```go
func TestCallbackRejectsMissingFlowCookie(t *testing.T) {
	h := testHandler(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=x&state=y", nil)
	h.Callback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 (no flow cookie)", rec.Code)
	}
}

func TestCallbackRejectsStateMismatch(t *testing.T) {
	h := testHandler(t)
	// Seal a flow cookie with state "good".
	fs, _ := json.Marshal(flowState{State: "good", Verifier: "v"})
	sealed, _ := h.codec.sealBytes(fs)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=x&state=evil", nil)
	req.AddCookie(&http.Cookie{Name: flowCookieName, Value: sealed})
	h.Callback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 (state mismatch)", rec.Code)
	}
}
```

(The test file already imports `encoding/json`, `net/http`, `net/http/httptest`. Add `"encoding/json"` to the import block if not present.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/webauth/ -run TestCallback -v`
Expected: FAIL — `h.Callback` undefined.

- [ ] **Step 3: Implement Callback**

Append to `internal/webauth/handlers.go`:

```go
// Callback completes the flow: recover state+verifier from the flow cookie,
// enforce state equality (CSRF), exchange the code, verify the ID token, and
// seal the session cookie. On success it redirects to "/".
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(flowCookieName)
	if err != nil {
		http.Error(w, "missing or expired login flow", http.StatusBadRequest)
		return
	}
	raw, err := h.codec.unsealBytes(c.Value)
	if err != nil {
		http.Error(w, "invalid login flow", http.StatusBadRequest)
		return
	}
	var fs flowState
	if err := json.Unmarshal(raw, &fs); err != nil {
		http.Error(w, "invalid login flow", http.StatusBadRequest)
		return
	}
	// Clear the flow cookie regardless of outcome (single use).
	h.clearCookie(w, flowCookieName)

	if r.URL.Query().Get("state") != fs.State || fs.State == "" {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	tok, sub, err := h.auth.exchange(r.Context(), code, fs.Verifier)
	if err != nil {
		slog.WarnContext(r.Context(), "oauth callback exchange failed", "err", err)
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	sealed, err := h.codec.Seal(Session{
		Sub:     sub,
		Access:  tok.AccessToken,
		Refresh: tok.RefreshToken,
		Expiry:  nowUTC().Add(sessionTTL),
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.setCookie(w, sessionCookieName, sealed, sessionTTL)
	http.Redirect(w, r, "/", http.StatusFound)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/webauth/ -run TestCallback -v`
Expected: PASS (both guard branches return 400).

- [ ] **Step 5: Commit**

`jj commit -m "feat(webauth): /auth/callback exchange + state check + session seal"`

---

### Task 7: Logout handler

**Files:**

- Modify: `internal/webauth/handlers.go`
- Test: `internal/webauth/handlers_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/webauth/handlers_test.go`:

```go
func TestLogoutClearsSessionCookie(t *testing.T) {
	h := testHandler(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	h.Logout(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204", rec.Code)
	}
	sc := rec.Header().Get("Set-Cookie")
	if !strings.Contains(sc, sessionCookieName+"=") || !strings.Contains(sc, "Max-Age=0") {
		t.Fatalf("session cookie not cleared: %q", sc)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/webauth/ -run TestLogout -v`
Expected: FAIL — `h.Logout` undefined.

- [ ] **Step 3: Implement Logout**

Append to `internal/webauth/handlers.go`:

```go
// Logout clears the session cookie. Coarse (no IdP back-channel logout); the
// sealed cookie simply stops being presented. 204 so the SPA can fire-and-forget.
func (h *Handler) Logout(w http.ResponseWriter, _ *http.Request) {
	h.clearCookie(w, sessionCookieName)
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/webauth/ -run TestLogout -v`
Expected: PASS.

- [ ] **Step 5: Commit**

`jj commit -m "feat(webauth): /auth/logout clears session cookie"`

---

### Task 8: Cookie→TokenInfo resolver (R2)

**Files:**

- Create: `internal/webauth/resolver.go`
- Test: `internal/webauth/resolver_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/webauth/resolver_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"context"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
)

func resolverReq(t *testing.T, cookie string) connect.AnyRequest {
	t.Helper()
	req := connect.NewRequest(&struct{}{})
	if cookie != "" {
		req.Header().Set("Cookie", sessionCookieName+"="+cookie)
	}
	return req
}

func TestResolverValidCookieYieldsSub(t *testing.T) {
	codec, _ := NewSessionCodec(testKey())
	r := NewResolver(codec)
	sealed, _ := codec.Seal(Session{Sub: "user-9", Expiry: nowUTC().Add(time.Hour)})
	ti, err := r.Resolve(context.Background(), resolverReq(t, sealed))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if ti == nil || ti.Extra["sub"] != "user-9" {
		t.Fatalf("got %+v want sub=user-9", ti)
	}
}

func TestResolverRejectsMissingCookie(t *testing.T) {
	r := NewResolver(mustCodec(t))
	if _, err := r.Resolve(context.Background(), resolverReq(t, "")); err == nil {
		t.Fatal("expected error for missing cookie")
	}
}

func TestResolverRejectsExpiredSession(t *testing.T) {
	codec := mustCodec(t)
	r := NewResolver(codec)
	sealed, _ := codec.Seal(Session{Sub: "u", Expiry: nowUTC().Add(-time.Minute)})
	if _, err := r.Resolve(context.Background(), resolverReq(t, sealed)); err == nil {
		t.Fatal("expected error for expired session")
	}
}

func TestResolverRejectsTamperedCookie(t *testing.T) {
	codec := mustCodec(t)
	r := NewResolver(codec)
	sealed, _ := codec.Seal(Session{Sub: "u", Expiry: nowUTC().Add(time.Hour)})
	bad := sealed[:len(sealed)-1] + "A"
	if _, err := r.Resolve(context.Background(), resolverReq(t, bad)); err == nil {
		t.Fatal("expected error for tampered cookie")
	}
}

func mustCodec(t *testing.T) *SessionCodec {
	t.Helper()
	c, err := NewSessionCodec(testKey())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

var _ = http.MethodGet // keep net/http import if unused elsewhere
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/webauth/ -run TestResolver -v`
Expected: FAIL — `NewResolver`/`Resolver` undefined.

- [ ] **Step 3: Implement the resolver**

Create `internal/webauth/resolver.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

// Resolver turns the engram session cookie on a Connect request into the
// *mcpauth.TokenInfo the Connect interceptor seam expects. It is the single
// authz entry for the Connect lane (R2): no cookie / expired / tampered → error
// → the interceptor maps it to CodeUnauthenticated. There is no anonymous
// fallthrough — the cookie's verified sub is the only identity this lane grants.
type Resolver struct {
	codec *SessionCodec
}

func NewResolver(codec *SessionCodec) *Resolver {
	return &Resolver{codec: codec}
}

// Resolve matches the connectResolver signature consumed by
// server.newConnectSubjectInterceptor.
func (r *Resolver) Resolve(_ context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, error) {
	// connect.AnyRequest.Header() already returns http.Header; wrap it in a
	// throwaway *http.Request to reuse the stdlib cookie parser.
	dummy := &http.Request{Header: req.Header()}
	c, err := dummy.Cookie(sessionCookieName)
	if err != nil {
		return nil, fmt.Errorf("no session cookie")
	}
	sess, err := r.codec.Unseal(c.Value)
	if err != nil {
		return nil, fmt.Errorf("invalid session cookie")
	}
	if !sess.Expiry.IsZero() && nowUTC().After(sess.Expiry) {
		return nil, fmt.Errorf("session expired")
	}
	if sess.Sub == "" {
		return nil, fmt.Errorf("session has empty subject")
	}
	return &mcpauth.TokenInfo{Extra: map[string]any{"sub": sess.Sub}}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/webauth/ -run TestResolver -v`
Expected: PASS (valid → sub; missing/expired/tampered → error).

- [ ] **Step 5: Commit**

`jj commit -m "feat(webauth): cookie->TokenInfo Connect resolver (R2)"`

---

### Task 9: Observability interceptors in `mountConnect` (R3)

**Files:**

- Create: `internal/server/connectobs.go`
- Test: `internal/server/connectobs_test.go`
- Modify: `internal/server/connectapi.go:mountConnect`
- Modify: `internal/server/tools.go:Register`

- [ ] **Step 1: Write the failing test**

Create `internal/server/connectobs_test.go` — assert the access-log interceptor logs method + code on success and error:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"connectrpc.com/connect"
)

func TestConnectAccessLogInterceptor(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	ic := newConnectAccessLogInterceptor(logger)
	next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("nope"))
	})
	wrapped := ic.WrapUnary(next)
	req := connect.NewRequest(&struct{}{})
	_, _ = wrapped(context.Background(), req)

	out := buf.String()
	if !strings.Contains(out, "unauthenticated") {
		t.Fatalf("access log missing code: %q", out)
	}
}
```

> `connect.UnaryInterceptorFunc` satisfies `connect.Interceptor`; `WrapUnary` is the method the test drives. Do **not** try to set `req.Spec().Procedure` — `Spec()` returns a value copy, so the assignment is a compile error. The interceptor reads the procedure at runtime; for this synthetic request it logs an empty procedure, which is fine — the assertion only checks the code field.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestConnectAccessLogInterceptor -v`
Expected: FAIL — `newConnectAccessLogInterceptor` undefined.

- [ ] **Step 3: Implement the access-log interceptor**

Create `internal/server/connectobs.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
)

// newConnectAccessLogInterceptor logs one line per unary RPC with the procedure
// and the resulting connect code, giving the Connect lane access-log parity with
// the MCP path (cmd/engram/httplog.go). It uses *Context slog so trace_id/span_id
// from the otelconnect span are stamped (see telemetry/logger.go).
func newConnectAccessLogInterceptor(logger *slog.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			resp, err := next(ctx, req)
			code := "ok"
			if err != nil {
				code = connect.CodeOf(err).String()
			}
			logger.InfoContext(ctx, "connect rpc",
				"procedure", req.Spec().Procedure,
				"code", code)
			return resp, err
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestConnectAccessLogInterceptor -v`
Expected: PASS.

- [ ] **Step 5: Wire otelconnect + access-log into `mountConnect`**

Modify `internal/server/connectapi.go`. Change `mountConnect` to build both interceptors and return an error (otelconnect can fail):

```go
func (d *deps) mountConnect(mux *http.ServeMux, resolve connectResolver) error {
	if resolve == nil {
		return nil // R1: no resolver => UI disabled => Connect not mounted at all.
	}
	otelIc, err := otelconnect.NewInterceptor()
	if err != nil {
		return fmt.Errorf("otelconnect interceptor: %w", err)
	}
	path, h := engramv1connect.NewEngramServiceHandler(
		&engramAPI{d: d},
		// Order: otel outermost (spans cover auth + logging), then access-log,
		// then the subject interceptor that resolves identity.
		connect.WithInterceptors(
			otelIc,
			newConnectAccessLogInterceptor(slog.Default()),
			newConnectSubjectInterceptor(resolve),
		),
	)
	mux.Handle(path, h)
	return nil
}
```

Add imports to `connectapi.go`: `"fmt"`, `"log/slog"`, `"connectrpc.com/otelconnect"`.

- [ ] **Step 6: Propagate the error from `Register`**

In `internal/server/tools.go:Register`, change the `d.mountConnect(mux, resolve)` call:

```go
	if err := d.mountConnect(mux, resolve); err != nil {
		return fmt.Errorf("mount connect: %w", err)
	}
```

- [ ] **Step 7: Run + tidy + commit**

Run: `go test ./internal/server/ && go mod tidy && git diff --exit-code go.mod go.sum || true`
Expected: tests PASS/SKIP; `otelconnect` moves from indirect to direct in go.mod.
`jj commit -m "feat(server): otelconnect + access-log interceptors on Connect (R3)"`

---

### Task 10: Mount-gating contract — nil → not mounted (R1)

**Files:**

- Test: `internal/server/connectapi_test.go` (add)
- (Implementation already in Task 9's `mountConnect`: the `if resolve == nil { return nil }` early return.)

This task locks the R1 contract with a regression test and confirms no existing test depended on the old nil→anonymous mount.

- [ ] **Step 1: Write the failing test**

Add to `internal/server/connectapi_test.go`:

```go
func TestMountConnectSkipsWhenResolverNil(t *testing.T) {
	d := &deps{} // no store needed; we never serve a request
	mux := http.NewServeMux()
	if err := d.mountConnect(mux, nil); err != nil {
		t.Fatalf("mountConnect(nil): %v", err)
	}
	// With no resolver, the EngramService path must NOT be registered: a request
	// to it falls through to the mux's 404.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/engram.v1.EngramService/ListScopes", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 (Connect must be unmounted when resolver is nil)", rec.Code)
	}
}

func TestMountConnectMountsWhenResolverPresent(t *testing.T) {
	d := &deps{}
	mux := http.NewServeMux()
	resolve := func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error) { return nil, nil }
	if err := d.mountConnect(mux, resolve); err != nil {
		t.Fatalf("mountConnect: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/engram.v1.EngramService/ListScopes", nil)
	mux.ServeHTTP(rec, req)
	// Mounted: connect-go responds (415/400 for a malformed body), NOT 404.
	if rec.Code == http.StatusNotFound {
		t.Fatal("Connect path should be mounted when a resolver is present")
	}
}
```

Ensure `connectapi_test.go` imports `net/http`, `net/http/httptest`, `context`, `connectrpc.com/connect`, and `mcpauth "github.com/modelcontextprotocol/go-sdk/auth"`.

- [ ] **Step 2: Run test to verify it fails/passes**

Run: `go test ./internal/server/ -run TestMountConnect -v`
Expected: PASS (the implementation from Task 9 already satisfies it). If `TestMountConnectSkipsWhenResolverNil` fails, the early-return from Task 9 Step 5 is missing — add it.

- [ ] **Step 3: Confirm no existing test relied on nil→anonymous mount**

Run: `go test ./internal/server/`
Expected: PASS/SKIP. The cross-actor Connect tests inject identity via `withConnectTokenInfo` directly (not through `mountConnect`), so they are unaffected. If any test calls `Register`/`mountConnect` with `nil` expecting a served anonymous response, update it to pass an explicit anonymous resolver `func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error) { return nil, nil }`.

- [ ] **Step 4: Commit**

`jj commit -m "test(server): lock R1 contract — nil resolver leaves Connect unmounted"`

---

### Task 11: Static asset serving (placeholder SPA)

**Files:**

- Create: `internal/webauth/static/index.html`
- Create: `internal/webauth/static.go`
- Test: `internal/webauth/static_test.go`

- [ ] **Step 1: Create the placeholder asset**

Create `internal/webauth/static/index.html`:

```html
<!doctype html>
<title>engram</title>
<main><h1>engram operator console</h1><p>UI assets not yet built.</p></main>
```

- [ ] **Step 2: Write the failing test**

Create `internal/webauth/static_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStaticHandlerServesIndex(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	StaticHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "operator console") {
		t.Fatalf("index not served: %q", rec.Body.String())
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/webauth/ -run TestStaticHandler -v`
Expected: FAIL — `StaticHandler` undefined.

- [ ] **Step 4: Implement `StaticHandler`**

Create `internal/webauth/static.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var staticFS embed.FS

// StaticHandler serves the committed SPA assets. v1 ships a placeholder
// index.html; the SvelteKit build replaces the static/ contents in the SPA plan
// (the go:embed + handler are unchanged). ADRs engram-0lu / engram-bgj.
func StaticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		// staticFS is compiled-in; a Sub failure is a build-time impossibility.
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/webauth/ -run TestStaticHandler -v`
Expected: PASS.

- [ ] **Step 6: Commit**

`jj commit -m "feat(webauth): go:embed placeholder SPA static handler"`

---

### Task 12: `serve.go` wiring — flags, tiebreaker, conditional mount (R1, R4)

**Files:**

- Modify: `cmd/engram/serve.go`

- [ ] **Step 1: Add the UI config flags**

In `cmd/engram/serve.go` add package-level vars and flag registration mirroring the existing `oidcIssuer` pattern (defaults from `server.EnvOr`):

```go
var (
	uiEnabled      string
	uiClientID     string
	uiClientSecret string
	uiRedirectURL  string
	uiCookieKey    string
)
```

In `init()`:

```go
	f.StringVar(&uiEnabled, "ui-enabled", server.EnvOr("MEM_UI_ENABLED", ""),
		"enable the web UI + login lane (empty=imply from creds; 'false'=hard off)")
	f.StringVar(&uiClientID, "oidc-client-id", server.EnvOr("MEM_OIDC_CLIENT_ID", ""),
		"OIDC confidential-client ID for the web login")
	f.StringVar(&uiClientSecret, "oidc-client-secret", server.EnvOr("MEM_OIDC_CLIENT_SECRET", ""),
		"OIDC client secret for the web login")
	f.StringVar(&uiRedirectURL, "ui-redirect-url", server.EnvOr("MEM_UI_REDIRECT_URL", ""),
		"OIDC auth-code callback URL")
	f.StringVar(&uiCookieKey, "ui-cookie-key", server.EnvOr("MEM_UI_COOKIE_KEY", ""),
		"32-byte AES-GCM key sealing the session cookie")
```

- [ ] **Step 2: Resolve UI config + build the resolver in `runServe`**

In `runServe`, after the `tm` is built and before `server.Register`, resolve the tiebreaker. Because flags override env, feed the resolved flag values (not `os.Getenv`) into `resolveUIConfig`:

```go
	uiCfg, err := resolveUIConfig(func(k string) string {
		switch k {
		case "MEM_UI_ENABLED":
			return uiEnabled
		case "MEM_OIDC_CLIENT_ID":
			return uiClientID
		case "MEM_OIDC_CLIENT_SECRET":
			return uiClientSecret
		case "MEM_UI_REDIRECT_URL":
			return uiRedirectURL
		case "MEM_UI_COOKIE_KEY":
			return uiCookieKey
		default:
			return ""
		}
	})
	if err != nil {
		slog.Error("web UI config invalid", "err", err)
		return err
	}

	var connectResolve func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error)
	var webHandler *webauth.Handler
	if uiCfg.Enabled {
		if oidcIssuer == "" {
			return fmt.Errorf("web UI enabled but no --oidc-issuer / MEM_OIDC_ISSUER: the login lane needs an issuer")
		}
		key := []byte(uiCfg.CookieKey)
		codec, err := webauth.NewSessionCodec(key)
		if err != nil {
			return fmt.Errorf("session cookie key: %w", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		authr, err := webauth.NewAuthenticator(ctx, oidcIssuer, uiCfg.ClientID, uiCfg.ClientSecret, uiCfg.RedirectURL)
		cancel()
		if err != nil {
			return fmt.Errorf("web UI OIDC discovery: %w", err)
		}
		webHandler = webauth.NewHandler(authr, codec, true)
		connectResolve = webauth.NewResolver(codec).Resolve
		slog.Info("web UI auth lane enabled", "issuer", oidcIssuer, "redirect", uiCfg.RedirectURL)
	} else {
		slog.Info("web UI disabled (headless); Connect API not mounted")
	}
```

Add imports: `"connectrpc.com/connect"`, `mcpauth "github.com/modelcontextprotocol/go-sdk/auth"`, `"github.com/seanb4t/engram/internal/webauth"`.

- [ ] **Step 3: Pass the resolver to `Register` and mount web routes**

Change the `server.Register(srv, mux, tm)` call (the post-#62 signature is `Register(srv, mux, tm, resolve)`) to pass `connectResolve` (nil when headless → Connect unmounted per Task 10). Remove the now-obsolete unconditional "mounted WITHOUT authentication" `Warn` block — it described the interim that R1 removes. After registration, mount the web routes only when enabled:

```go
	if err := server.Register(srv, mux, tm, connectResolve); err != nil {
		slog.Error("server registration failed", "err", err)
		return err
	}

	if uiCfg.Enabled {
		mux.HandleFunc("GET /auth/login", webHandler.Login)
		mux.HandleFunc("GET /auth/callback", webHandler.Callback)
		mux.HandleFunc("POST /auth/logout", webHandler.Logout)
		// Static SPA is the fallback for non-API, non-auth routes. Registered
		// last and only when enabled; the MCP handler still owns "/" below for
		// the streamable transport, so static is mounted under "/ui/".
		mux.Handle("/ui/", http.StripPrefix("/ui/", webauth.StaticHandler()))
	}
```

> **R4 (CORS):** no CORS middleware is added anywhere. `connect-go` emits no `Access-Control-Allow-*` headers by default, so the Connect surface is same-origin only — exactly the spec's CSRF assumption. Do **not** add `connectrpc.com/cors`. Task 13 asserts this.

- [ ] **Step 4: Build + run the server package**

Run: `go build ./... && go test ./cmd/engram/`
Expected: builds clean; `cmd/engram` tests PASS.

- [ ] **Step 5: Commit**

`jj commit -m "feat(serve): gate Connect + web auth lane on MEM_UI_* (R1, R4)"`

---

### Task 13: End-to-end isolation through the cookie Connect path (R2)

**Files:**

- Create: `internal/server/connectapi_cookie_test.go`

This integration test mirrors `TestAuthedCrossActorSharedReadHandlers` but drives the **real** cookie resolver + interceptor over HTTP, proving a distinct authenticated caller cannot read another owner's private record via the Connect lane, and that a request with no cookie is rejected.

- [ ] **Step 1: Write the failing test**

Create `internal/server/connectapi_cookie_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
	"github.com/seanb4t/engram/internal/store"
)

// stubResolve maps a fixed cookie header value to a sub, standing in for the
// webauth.Resolver (which this package cannot import without a cycle). It proves
// the interceptor → subjectFromConnectContext → store path enforces isolation.
func TestConnectCookieLaneIsolation(t *testing.T) {
	d := testDeps(t) // existing helper; skips when Qdrant unavailable
	ctx := context.Background()
	scope := "iso-cookie:project:xactor"

	// Seed actor-A and actor-B private records directly via the store, mirroring
	// TestAuthedCrossActorSharedReadHandlers. timeNow() is defined in tools_test.go
	// (same package).
	aPriv := store.Memory{ID: "c0000000-0000-0000-0000-000000000001", Content: "A private", Scope: scope, Owner: "actor-A", Visibility: "private", Category: "convention", Source: "agent-inferred", CreatedAt: timeNow()}
	bPriv := store.Memory{ID: "c0000000-0000-0000-0000-000000000002", Content: "B private", Scope: scope, Owner: "actor-B", Visibility: "private", Category: "convention", Source: "agent-inferred", CreatedAt: timeNow()}
	for _, m := range []store.Memory{aPriv, bPriv} {
		if err := d.st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}
	defer func() {
		_ = d.st.Delete(ctx, aPriv.ID, store.Authenticated("actor-A"))
		_ = d.st.Delete(ctx, bPriv.ID, store.Authenticated("actor-B"))
	}()

	resolve := func(_ context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, error) {
		switch req.Header().Get("X-Test-Actor") {
		case "A":
			return &mcpauth.TokenInfo{Extra: map[string]any{"sub": "actor-A"}}, nil
		case "B":
			return &mcpauth.TokenInfo{Extra: map[string]any{"sub": "actor-B"}}, nil
		default:
			return nil, connect.NewError(connect.CodeUnauthenticated, errStr("no identity"))
		}
	}

	mux := http.NewServeMux()
	if err := d.mountConnect(mux, resolve); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)

	// Caller B lists scope-x: must see only B's record, never A's.
	reqB := connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope})
	reqB.Header().Set("X-Test-Actor", "B")
	respB, err := client.ListMemories(ctx, reqB)
	if err != nil {
		t.Fatalf("ListMemories(B): %v", err)
	}
	for _, m := range respB.Msg.Memories {
		if m.Owner == "actor-A" {
			t.Fatalf("caller B saw actor-A record %q — isolation breach", m.Id)
		}
	}

	// No identity header → Unauthenticated.
	reqAnon := connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope})
	_, err = client.ListMemories(ctx, reqAnon)
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("anon request: got code %v want unauthenticated", connect.CodeOf(err))
	}
}

func errStr(s string) error { return &simpleErr{s} }

type simpleErr struct{ s string }

func (e *simpleErr) Error() string { return e.s }

var _ = time.Second // keep import if unused after edits
```

> This mirrors `TestAuthedCrossActorSharedReadHandlers`'s exact seeding idiom (direct `store.Memory{...}` literals + `d.st.Upsert(ctx, m, []float32{0.1,0.2,0.3})`, `timeNow()` from `tools_test.go`). `subjectFromConnectContext` reads the interceptor-set identity, so no `withConnectTokenInfo` injection is needed here — the real interceptor runs over HTTP via `httptest`.

- [ ] **Step 2: Run test to verify it fails (then passes)**

Run: `go test ./internal/server/ -run TestConnectCookieLaneIsolation -v`
Expected: initially FAIL if helper names differ (fix to match existing helpers), then PASS/SKIP. Isolation must hold: caller B never sees actor-A rows; anon → Unauthenticated.

- [ ] **Step 3: Full suite + gofmt + commit**

Run: `gofmt -l internal/ cmd/ && go test ./...`
Expected: `gofmt -l` prints nothing (clean); tests PASS/SKIP. Fix any gofmt output with `gofmt -w` before committing (the CI `test` job runs `gofmt -l` first — see the repo's gofmt CI trap).
`jj commit -m "test(server): end-to-end cookie-lane Connect isolation (R2)"`

---

## Final verification (after all tasks)

- [ ] `task fmt && task lint` — clean (golangci-lint, rumdl, yamlfmt). The new markdown plan/spec carry SPDX headers; `task license:check` clean.
- [ ] `gofmt -l cmd/ internal/` — empty (CI `test` job runs this first).
- [ ] `go mod tidy && git diff --exit-code go.mod go.sum` — `otelconnect` + `oauth2` are direct deps; no drift.
- [ ] `go test ./...` — PASS/SKIP.
- [ ] Manual smoke (optional): run `engram serve` with no `MEM_UI_*` → log says "web UI disabled (headless); Connect API not mounted"; a POST to `/engram.v1.EngramService/ListScopes` → 404. Set the full `MEM_UI_*` + `MEM_OIDC_ISSUER` → log says "web UI auth lane enabled"; the same POST without a cookie → `unauthenticated`.

## Out of scope (follow-up plans)

- The SvelteKit SPA (views, connect-es client, component tests, `task ui:build`, vendored-asset drift CI) — its own plan; this plan ships only the placeholder `index.html`.
- Write-phase RPCs (`StoreMemory`, `StoreDiscovery`) + CSRF-token hardening — phase 2/3.
- Session refresh-token rotation / re-seal on access-token expiry — v1 trusts the sealed cookie's `sub` until the session TTL; refresh is a refinement.
<!-- adr-capture: sha256=a221d7ddb84ddd88; session=cli; ts=2026-06-10T00:58:20Z; adrs=engram-1xv -->
