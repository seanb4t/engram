# Phase 23: Service Auth Chain & Tenancy Isolation - Pattern Map

**Mapped:** 2026-07-17
**Files analyzed:** 8 (3 new `internal/auth` files, 2 modified config files, 1 modified serve.go, 2 test-suite extensions)
**Analogs found:** 8 / 8 (all in-repo, zero external references — this phase's domain is entirely in `internal/auth`, `internal/config`, `internal/server`, `internal/store`)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/auth/chain.go` (NEW: `chainVerifier`, JWT-shape discriminator) | middleware/combinator | request-response | `internal/auth/auth.go` `TokenVerifier()` (:171-212) | role-match (wraps the same `mcpauth.TokenVerifier` func type) |
| `internal/auth/static_token.go` (NEW: static-token verifier) | middleware/verifier | request-response | `internal/auth/auth.go` `TokenVerifier()` + `namespacedOwner` (:92-94, :171-212) | role-match (same func-type contract, different credential source) |
| `internal/auth/auth.go` (MODIFIED: `New`/`NewFromProvider` for D-14 per-lane audience) | service (verifier constructor) | request-response | itself, `New` (:69-81) | exact (extending existing constructor pattern) |
| `internal/config/registry.go` (MODIFIED: `service_auth.*` rows) | config | CRUD (static table) | `oidc.*` rows (:47-52) | exact |
| `internal/config/validate.go` (MODIFIED: service-auth validation) | config/validator | transform | `Validate()` OIDC/URL-shaped field blocks (:71-96) | role-match |
| `cmd/engram/serve.go` (MODIFIED: `withAuth`, the ONE call site) | wiring/bootstrap | request-response | itself, `withAuth` (:290-305) | exact |
| `internal/auth/chain_test.go`, `static_token_test.go`, `service_owner_failclosed_test.go` (NEW) | test | request-response | `internal/auth/auth_test.go` (stub `idVerifier`, `TestTokenVerifierStampsOwnerClaimKey`-style) | exact |
| `internal/server/*_service_auth_parity_test.go` + `internal/store/*_isolation_test.go` (NEW/extended) | test | CRUD / parity | `internal/server/connectapi_write_parity_test.go` `TestWriteParity` (:172); `internal/store/store_test.go` `TestSearchListOwnerIsolation` (:563), `TestAnonBucketReadIsolation` (:1131) | exact |

## Pattern Assignments

### `internal/auth/chain.go` (NEW combinator)

**Analog:** `internal/auth/auth.go` — the `mcpauth.TokenVerifier` function-type contract and its `errors.Join` deny pattern.

**Imports pattern** (mirror `auth.go:15-31`):
```go
import (
	"context"
	"errors"
	"net/http"
	"strings"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)
```

**Core combinator pattern** (D-01/D-02/D-04, adapted from `auth.go:171-212`'s `TokenVerifier()` shape — a closure returning `mcpauth.TokenVerifier`):
```go
// TokenVerifier func(ctx, token, *http.Request) (*mcpauth.TokenInfo, error) — auth.go:171
func (v *Verifier) TokenVerifier() mcpauth.TokenVerifier {
	return func(ctx context.Context, token string, req *http.Request) (info *mcpauth.TokenInfo, err error) {
		idt, verr := v.idv.Verify(ctx, token)
		if verr != nil {
			err = errors.Join(mcpauth.ErrInvalidToken, verr) // auth.go:192
			return nil, err
		}
		...
	}
}
```
D-04 discriminator (structural, no parse — mirrors "Don't Hand-Roll" guidance in RESEARCH.md):
```go
func looksLikeJWT(token string) bool {
	return strings.Count(token, ".") == 2
}
```
D-03 nil-mechanism guard (a routed branch's verifier may be `nil` when unconfigured — must resolve to `ErrInvalidToken`, never a nil-pointer panic):
```go
if oidcService == nil {
	return nil, errors.Join(mcpauth.ErrInvalidToken, errors.New("client-credentials lane not configured"))
}
```

**Error handling pattern** (deny-by-default, `auth.go:192,203` exact idiom):
```go
err = errors.Join(mcpauth.ErrInvalidToken, cerr)
return nil, err
```

---

### `internal/auth/static_token.go` (NEW static-token verifier)

**Analog:** `internal/auth/auth.go` `namespacedOwner` (:92-94) + `TokenVerifier()` closure shape (:171-212).

**Imports pattern:**
```go
import (
	"context"
	"crypto/subtle" // genuinely new — zero existing crypto/subtle usage in-repo (RESEARCH.md verified)
	"errors"
	"net/http"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)
```

**Core pattern** (D-11/D-12 — token→owner map, constant-time compare, reuse `namespacedOwner` verbatim, synthesize `TokenInfo` directly, no JWKS):
```go
// namespacedOwner reused verbatim — auth.go:92-94, D-06/D-11
owner := namespacedOwner("static_token", ownerID)

func (v *staticTokenVerifier) verify(ctx context.Context, token string, _ *http.Request) (*mcpauth.TokenInfo, error) {
	for candidateOwner, candidateToken := range v.tokens {
		if subtle.ConstantTimeCompare([]byte(token), []byte(candidateToken)) == 1 {
			return &mcpauth.TokenInfo{
				UserID: candidateOwner,
				Extra:  map[string]any{auth.OwnerClaimExtraKey: namespacedOwner("static_token", candidateOwner)},
			}, nil
		}
	}
	// D-12: never interpolate the raw token value into this error.
	return nil, errors.Join(mcpauth.ErrInvalidToken, errors.New("static token not recognized"))
}
```
Note: constant-time compare MUST still iterate every candidate (no early-return short-circuit that leaks which prefix matched) — table-driven test in `static_token_test.go` should assert timing-insensitive behavior isn't undermined by loop-order.

**No-leak discipline (D-12, DEC-wot):** mirror `auth.go:189` (`slog.WarnContext(ctx, "token rejected", "err", verr)`) but the logged `err`/`verr` must NEVER embed the raw token literal — assert this in `TestStaticTokenNoLeak`.

---

### `internal/auth/auth.go` (MODIFIED — D-14 per-lane audience)

**Analog:** itself, `New` (:69-81).

**Pattern (Pattern 3 from RESEARCH.md, verified against go-oidc v3.20.0 — audience is baked into `oidc.Config` at construction, no per-call override):**
```go
// Source: internal/auth/auth.go:69-81 (existing New), generalized per D-14
func NewFromProvider(provider *oidc.Provider, audience string, ownerClaims []string) *Verifier {
	return &Verifier{
		idv: provider.Verifier(&oidc.Config{
			ClientID:          audience,
			SkipClientIDCheck: audience == "",
		}),
		ownerClaims: ownerClaims,
	}
}
```
Same-issuer reuse avoids a second discovery round-trip; distinct-issuer service lane uses a plain second `auth.New(ctx, serviceIssuer, serviceAudience, serviceOwnerClaims)` call — planner's discretion per CONTEXT.md.

---

### `internal/config/registry.go` (MODIFIED — `service_auth.*` rows)

**Analog:** `oidc.*` rows, `registry.go:47-52` (exact field-struct pattern to replicate).

```go
// Existing exact shape to mirror (registry.go:47-52):
{Key: "oidc.issuer", Env: "ENGRAM_OIDC_ISSUER", Legacy: "MEM_OIDC_ISSUER", Flag: "oidc-issuer"},
{Key: "oidc.owner_claim", Env: "ENGRAM_OWNER_CLAIM", Flag: "owner-claim", Default: "email"},

// NEW rows to add, same struct/pattern:
{Key: "service_auth.oidc_issuer", Env: "ENGRAM_SERVICE_AUTH_OIDC_ISSUER"},               // Flag optional, D-14
{Key: "service_auth.oidc_audience", Env: "ENGRAM_SERVICE_AUTH_OIDC_AUDIENCE"},
{Key: "service_auth.owner_claims", Env: "ENGRAM_SERVICE_AUTH_OWNER_CLAIMS", Default: "client_id,azp"}, // D-05
{Key: "service_auth.static_tokens", Env: "ENGRAM_SERVICE_AUTH_STATIC_TOKENS"},           // no Flag — secret map, D-11 discretion on serialization
```
Owner-claims comma-list parsing reuses `config.ParseOwnerClaims` (`config.go:237`) verbatim for `service_auth.owner_claims` — same split/trim/empty-list-preserving semantics, applied with the `["client_id","azp"]` default per D-05.

---

### `internal/config/validate.go` (MODIFIED)

**Analog:** the OIDC/URL-shaped `Validate()` blocks (:71-96) — the "self-gated no-op when empty, validate shape when set" idiom already used for `OpenAI.EmbeddingsURL` (:87-96).

```go
// Mirror this exact self-gating shape for service_auth.oidc_issuer/audience (both optional — D-03 independent enablement):
if c.ServiceAuth.OIDCIssuer != "" {
	switch u, err := url.Parse(c.ServiceAuth.OIDCIssuer); {
	case err != nil:
		errs = append(errs, fmt.Errorf("ENGRAM_SERVICE_AUTH_OIDC_ISSUER %q: must be a valid URL: %w", c.ServiceAuth.OIDCIssuer, err))
	case u.Scheme != "http" && u.Scheme != "https":
		errs = append(errs, fmt.Errorf("ENGRAM_SERVICE_AUTH_OIDC_ISSUER %q: scheme must be http or https", c.ServiceAuth.OIDCIssuer))
	}
}
```

---

### `cmd/engram/serve.go` (MODIFIED `withAuth`, the ONE call site)

**Analog:** itself (:286-305).

**Current shape to extend (exact, verified):**
```go
func withAuth(handler http.Handler, oidc config.OIDCConfig, ownerClaims []string) (http.Handler, error) {
	if oidc.Issuer == "" {
		slog.Warn("OIDC validation DISABLED (no --oidc-issuer / ENGRAM_OIDC_ISSUER); all requests accepted")
		return handler, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	verifier, err := auth.New(ctx, oidc.Issuer, oidc.Audience, ownerClaims)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("oidc verifier init: %w", err)
	}
	slog.Info("OIDC bearer-token validation enabled", "issuer", oidc.Issuer)
	return mcpauth.RequireBearerToken(verifier.TokenVerifier(), &mcpauth.RequireBearerTokenOptions{
		ResourceMetadataURL: oidc.ResourceMetadata,
	})(handler), nil
}
```
D-03: build each of the 3 verifiers conditionally on its own config presence (human OIDC / service OIDC / static-token), then wrap with the new `chainVerifier` instead of `verifier.TokenVerifier()` directly. Preserve the existing `slog.Warn`/`slog.Info` logging idiom per mechanism enabled (mirrors `ownerClaimGuard`'s "warn, don't block" posture, serve.go:279-282).

---

### Tests: `internal/auth/*_test.go` (NEW)

**Analog:** `internal/auth/auth_test.go` — stub `idVerifier` injection pattern (per `auth.go:51-53`'s `idVerifier` interface, extracted specifically so tests can fake the OIDC verifier without a live IdP).

D-10 (FIRST test of the phase, `service_owner_failclosed_test.go`): assert rejection AT THE VERIFIER-CHAIN LAYER (`errors.Is(err, mcpauth.ErrInvalidToken)`), not merely "eventually errors somewhere" — RESEARCH.md Summary #2 is explicit that `SubjectFromTokenInfo` (`internal/server/identity.go:22-30`) already has a generic-but-late reject; the NEW test must prove the early one.

### Tests: parity/isolation (`internal/server`, `internal/store`)

**Analog:** `internal/server/connectapi_write_parity_test.go` `TestWriteParity` (:172-213) — dual-lane invocation (`dMCP.storeMemory` vs `api.StoreMemory`), `assertCodeParity`, comparing stored effects; mirror this shape but across auth-chain branches (human OIDC vs client-creds vs static-token) instead of across MCP/Connect lanes.

**Analog:** `internal/store/store_test.go` `TestSearchListOwnerIsolation` (:563-593) and `TestAnonBucketReadIsolation` (:1131+) — exact `mk(id, owner, vis)` helper + `s.Search(ctx, scope, Authenticated(owner), ...)` assertion shape to replicate for:
- `TestServicePrincipalIsolation` — a client-creds/static-token-resolved owner cannot see another service principal's or human's private records, doesn't collide with `Anonymous()`.
- `TestSharedCrossTenantReadIntended` (D-15/D-16) — two service-tenant owners, one `shared` record, the OTHER tenant's principal CAN read it (permanent, intentional — same `Search`/`hits` assertion shape, inverted expectation from the isolation tests).

```go
// Exact helper shape to reuse, store_test.go:569-575:
mk := func(id, owner, vis string) {
	m := Memory{ID: id, Content: "x", Scope: scope, Owner: owner, Visibility: vis, CreatedAt: time.Now().UTC()}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert %s: %v", id, err)
	}
}
```

## Shared Patterns

### `errors.Join(mcpauth.ErrInvalidToken, ...)` deny-by-default
**Source:** `internal/auth/auth.go:192,203`
**Apply to:** `chain.go` (discriminator no-match, nil-mechanism branch), `static_token.go` (no-match), the D-08 service-lane empty-owner reject.

### `namespacedOwner(claim, value)` injective encoding — REUSE, never reinvent
**Source:** `internal/auth/auth.go:83-94`
**Apply to:** client-credentials lane owner resolution (via existing `ClaimIdentity`, zero new code) AND the static-token verifier's `namespacedOwner("static_token", ownerID)` (D-11) — same collision-safety proof, same call.

### `OwnerClaimExtraKey` contract — the ONE thing downstream reads
**Source:** `internal/auth/auth.go:41-46`; consumed unmodified by `internal/server/identity.go:22-30` (`SubjectFromTokenInfo`)
**Apply to:** every new verifier (chain, static-token, client-creds) MUST stamp `TokenInfo.Extra[auth.OwnerClaimExtraKey]` with the resolved owner string — this is the sole seam `internal/server`/`internal/store` read; nothing else needs to change downstream (D-01/D-06 guarantee).

### koanf field-registry `{Key,Env,Legacy,Flag,Default}` row pattern
**Source:** `internal/config/registry.go:47-52` (`oidc.*` block)
**Apply to:** every new `service_auth.*` row — no Legacy value (brand-new vars), Flag omitted for the secret-bearing `static_tokens` map per RESEARCH.md Open Question #2 recommendation.

### `Validate()` self-gated-when-empty / shape-checked-when-set idiom
**Source:** `internal/config/validate.go:87-96` (`OpenAI.EmbeddingsURL` block) and `:108-128` (`Summarize.Model`-gated block)
**Apply to:** `service_auth.oidc_issuer`/`oidc_audience` (optional URL fields, D-03 independent enablement) and `service_auth.static_tokens` (optional map, malformed-when-present = fatal per V5 input-validation guidance in RESEARCH.md).

## No Analog Found

None — every file this phase touches has a direct, concretely-cited in-repo analog. RESEARCH.md independently confirms this phase introduces zero new architectural surface (chain composition + one new stdlib import, `crypto/subtle`, over existing seams).

## Metadata

**Analog search scope:** `internal/auth/`, `internal/server/`, `internal/config/`, `internal/store/`, `cmd/engram/serve.go`
**Files scanned:** `internal/auth/auth.go` (full), `internal/server/identity.go` (full), `cmd/engram/serve.go:260-306`, `internal/config/registry.go:1-60`, `internal/config/validate.go` (full), `internal/config/config.go:117-250` (OIDCConfig + ParseOwnerClaims), `internal/server/connectapi_write_parity_test.go:172-213`, `internal/store/store_test.go:563-620,1131-1170`
**Pattern extraction date:** 2026-07-17
