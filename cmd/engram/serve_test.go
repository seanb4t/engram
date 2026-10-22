// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc/oidctest"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"

	"github.com/seanb4t/engram/internal/auth"
	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/telemetry"
)

// wantNamespacedOwner mirrors auth.go's DOCUMENTED namespacedOwner encoding
// (`<len(claim)>:<claim>:<len(value)>:<value>`), used as an independent test
// oracle rather than reaching into the unexported namespacedOwner helper.
func wantNamespacedOwner(claim, value string) string {
	return fmt.Sprintf("%d:%s:%d:%s", len(claim), claim, len(value), value)
}

func TestOwnerClaimGuard(t *testing.T) {
	cases := []struct {
		name         string
		bearerIssuer string
		uiEnabled    bool
		ownerClaims  []string
		wantErr      bool
	}{
		{"no auth lane, empty claim ok", "", false, nil, false},
		{"bearer lane, default claim ok", "https://issuer", false, []string{"email"}, false},
		{"bearer lane, non-default claim ok (warns, no error)", "https://issuer", false, []string{"preferred_username"}, false},
		{"bearer lane, ordered list including email ok", "https://issuer", false, []string{"email", "sub"}, false},
		{"bearer lane, empty claim rejected", "https://issuer", false, nil, true},
		{"ui lane, empty claim rejected", "", true, nil, true},
		{"ui lane, default claim ok", "", true, []string{"email"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ownerClaimGuard(tc.bearerIssuer, tc.uiEnabled, tc.ownerClaims)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ownerClaimGuard(%q, %v, %v) err=%v, wantErr=%v",
					tc.bearerIssuer, tc.uiEnabled, tc.ownerClaims, err, tc.wantErr)
			}
		})
	}
}

// TestOwnerClaimGuardEmptyFlagRejected proves the round-3 review's finding-9
// acceptance path: an explicit empty CLI flag (--owner-claim="") parses to an
// empty list via config.ParseOwnerClaims, and the guard rejects it when an
// auth lane is active. This is deliberately NOT tested via
// ENGRAM_OWNER_CLAIM="" — the env TransformFunc's empty-value guard makes an
// empty ENV var preserve the registry default ("email"), never an empty list
// (see TestLoadEmptyEnvPreservesDefault in internal/config).
func TestOwnerClaimGuardEmptyFlagRejected(t *testing.T) {
	claims, err := config.ParseOwnerClaims("") // simulates the parsed --owner-claim="" flag value
	if err != nil {
		t.Fatalf("ParseOwnerClaims(\"\"): %v", err)
	}
	if len(claims) != 0 {
		t.Fatalf("ParseOwnerClaims(\"\") = %v, want empty list", claims)
	}
	if err := ownerClaimGuard("https://issuer", false, claims); err == nil {
		t.Error("expected rejection for an explicit empty owner-claim with the bearer lane active")
	}
}

// TestOwnerClaimGuardUnsetDefaultNoWarn proves the companion half of the
// finding-9 acceptance path: an unset ENGRAM_OWNER_CLAIM resolves via the
// registry default to ["email"] and does NOT trigger the missing-"email" warn.
func TestOwnerClaimGuardUnsetDefaultNoWarn(t *testing.T) {
	claims, err := config.ParseOwnerClaims("email") // the registry's Default: "email" when unset
	if err != nil {
		t.Fatalf("ParseOwnerClaims(\"email\"): %v", err)
	}

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	if err := ownerClaimGuard("https://issuer", false, claims); err != nil {
		t.Fatalf("ownerClaimGuard: %v", err)
	}
	if strings.Contains(buf.String(), "owner-claim") {
		t.Errorf("expected no warning for the default [\"email\"] claim list, got %q", buf.String())
	}
}

// TestWithAuth_NoLaneConfigured_ReturnsHandlerUnchanged proves the SC1/D-03
// behavior-preservation guarantee: with no human OIDC issuer AND no
// service_auth.* config present, withAuth returns the handler untouched (the
// pre-chain "validation DISABLED" path) — the inner handler's response is
// observed unmodified, with no bearer-token gate in front of it.
func TestWithAuth_NoLaneConfigured_ReturnsHandlerUnchanged(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	chain, err := buildAuthChain(config.OIDCConfig{}, config.ServiceAuthConfig{}, nil)
	if err != nil {
		t.Fatalf("buildAuthChain: %v", err)
	}
	handler := withAuth(inner, chain, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("expected the unwrapped handler to run untouched (no auth lane configured), got status %d", rec.Code)
	}
}

// newServeTestOIDCServer starts a minimal OIDC discovery+JWKS server (mirrors
// internal/auth/service_owner_failclosed_test.go's fixture) so withAuth's
// human-lane branch can perform real discovery against a live issuer.
func newServeTestOIDCServer(t *testing.T) (issuer string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	discoveryAndKeys := &oidctest.Server{
		PublicKeys: []oidctest.PublicKey{{PublicKey: key.Public(), KeyID: "serve-test-key", Algorithm: oidc.RS256}},
	}
	srv := httptest.NewServer(discoveryAndKeys)
	t.Cleanup(srv.Close)
	discoveryAndKeys.SetIssuer(srv.URL)
	return srv.URL
}

// TestWithAuth_StaticTokenLane_AuthenticatesConfiguredToken proves the
// config-string -> withAuth -> live verify seam that CR-01 broke: it sets
// svcAuth.StaticTokens to a real "owner=token" config STRING (not a
// hand-constructed map), driving it through
// config.ParseServiceStaticTokens exactly as withAuth does in production. A
// request bearing the configured token must reach the inner handler and
// resolve to the namespaced owner; an unrelated bearer must be rejected 401.
func TestWithAuth_StaticTokenLane_AuthenticatesConfiguredToken(t *testing.T) {
	var gotOwner string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ti := mcpauth.TokenInfoFromContext(r.Context()); ti != nil {
			gotOwner, _ = ti.Extra[auth.OwnerClaimExtraKey].(string)
		}
		w.WriteHeader(http.StatusTeapot)
	})
	svcAuth := config.ServiceAuthConfig{StaticTokens: "owner-x=secret-token-value"}
	chain, err := buildAuthChain(config.OIDCConfig{}, svcAuth, nil)
	if err != nil {
		t.Fatalf("buildAuthChain: %v", err)
	}
	handler := withAuth(inner, chain, nil)

	t.Run("configured token authenticates", func(t *testing.T) {
		gotOwner = ""
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer secret-token-value")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusTeapot {
			t.Fatalf("expected the configured token to authenticate and reach the inner handler, got status %d", rec.Code)
		}
		want := wantNamespacedOwner("static_token", "owner-x")
		if gotOwner != want {
			t.Fatalf("resolved owner = %q, want %q", gotOwner, want)
		}
	})

	t.Run("unrelated bearer rejected", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer not-the-configured-token")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected an unrelated bearer to be rejected 401, got %d", rec.Code)
		}
	})
}

// TestWithAuth_HumanOnlyConfig_RejectsUnauthenticated proves the human-only
// lane (issuer set, no service_auth.* config) constructs a chain that gates
// the inner handler behind bearer-token validation — an unauthenticated
// request is rejected 401 rather than reaching the inner handler, in
// contrast to TestWithAuth_NoLaneConfigured_ReturnsHandlerUnchanged above.
func TestWithAuth_HumanOnlyConfig_RejectsUnauthenticated(t *testing.T) {
	issuer := newServeTestOIDCServer(t)
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	chain, err := buildAuthChain(config.OIDCConfig{Issuer: issuer}, config.ServiceAuthConfig{}, []string{"email"})
	if err != nil {
		t.Fatalf("buildAuthChain: %v", err)
	}
	handler := withAuth(inner, chain, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for an unauthenticated request against the human-only chain, got %d", rec.Code)
	}
}

// newStaticTokenChain returns a chain whose only lane accepts the bearer
// "secret-token-value".
func newStaticTokenChain(t *testing.T) mcpauth.TokenVerifier {
	t.Helper()
	chain, err := buildAuthChain(config.OIDCConfig{}, config.ServiceAuthConfig{StaticTokens: "owner-x=secret-token-value"}, nil)
	if err != nil {
		t.Fatalf("buildAuthChain: %v", err)
	}
	return chain
}

// challengeThrough POSTs to h without a token through origin (scheme and
// host) and returns the response.
func challengeThrough(h http.Handler, scheme, host string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Host = host
	req.Header.Set("X-Forwarded-Proto", scheme)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestWithAuth_DerivedChallengePerRequest(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	chain := newStaticTokenChain(t)

	t.Run("each origin gets its own document", func(t *testing.T) {
		h := withAuth(inner, chain, mustResource(t, "", "", "", "/mcp"))
		for _, host := range []string{"a.example", "b.example"} {
			rec := challengeThrough(h, "https", host)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("%s: status=%d want 401", host, rec.Code)
			}
			want := `Bearer resource_metadata="https://` + host + `/.well-known/oauth-protected-resource/mcp"`
			if got := rec.Header().Get("WWW-Authenticate"); got != want {
				t.Fatalf("%s: WWW-Authenticate=%q want %q", host, got, want)
			}
		}
	})

	t.Run("root MCP path uses the host-only document", func(t *testing.T) {
		h := withAuth(inner, chain, mustResource(t, "", "", "", "/"))
		rec := challengeThrough(h, "https", "a.example")
		want := `Bearer resource_metadata="https://a.example/.well-known/oauth-protected-resource"`
		if got := rec.Header().Get("WWW-Authenticate"); got != want {
			t.Fatalf("WWW-Authenticate=%q want %q", got, want)
		}
	})

	t.Run("configured metadata URL wins", func(t *testing.T) {
		h := withAuth(inner, chain, mustResource(t, "", "", "https://meta.example/prm", "/mcp"))
		for _, host := range []string{"a.example", "b.example"} {
			rec := challengeThrough(h, "https", host)
			if got, want := rec.Header().Get("WWW-Authenticate"), `Bearer resource_metadata="https://meta.example/prm"`; got != want {
				t.Fatalf("%s: WWW-Authenticate=%q want %q", host, got, want)
			}
		}
	})

	t.Run("pinned resource URL names its own origin", func(t *testing.T) {
		h := withAuth(inner, chain, mustResource(t, "https://a.example/mcp", "", "", "/mcp"))
		rec := challengeThrough(h, "https", "b.example")
		want := `Bearer resource_metadata="https://a.example/.well-known/oauth-protected-resource/mcp"`
		if got := rec.Header().Get("WWW-Authenticate"); got != want {
			t.Fatalf("WWW-Authenticate=%q want %q", got, want)
		}
	})

	t.Run("a valid token is served", func(t *testing.T) {
		h := withAuth(inner, chain, mustResource(t, "", "", "", "/mcp"))
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer secret-token-value")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusTeapot {
			t.Fatalf("status=%d want %d", rec.Code, http.StatusTeapot)
		}
	})
}

func TestWithAuth_BareChallengeWhenNoParams(t *testing.T) {
	chain := newStaticTokenChain(t)

	t.Run("no challenge value still sends a bare Bearer challenge", func(t *testing.T) {
		inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
		for name, h := range map[string]http.Handler{
			"no resource":    withAuth(inner, chain, nil),
			"refused origin": withAuth(inner, chain, mustResource(t, "", "https://b.example", "", "/mcp")),
		} {
			rec := challengeThrough(h, "https", "a.example")
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("%s: status=%d want 401", name, rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Fatalf("%s: WWW-Authenticate=%q want %q", name, got, "Bearer")
			}
		}
	})

	t.Run("the wrapped handler's own 401 and 403 are untouched", func(t *testing.T) {
		for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
			inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) })
			h := withAuth(inner, chain, nil)
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			req.Header.Set("Authorization", "Bearer secret-token-value")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != code {
				t.Fatalf("status=%d want %d", rec.Code, code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "" {
				t.Fatalf("%d: WWW-Authenticate=%q want absent", code, got)
			}
		}
	})
}

// TestBuildAuthChainNoLaneConfiguredReturnsNil pins the D-06/D-03 baseline:
// with no OIDC issuer and no service_auth.* config, buildAuthChain returns
// a nil chain and a nil error — "no lane configured" is not an error case.
func TestBuildAuthChainNoLaneConfiguredReturnsNil(t *testing.T) {
	chain, err := buildAuthChain(config.OIDCConfig{}, config.ServiceAuthConfig{}, nil)
	if err != nil {
		t.Fatalf("buildAuthChain: %v", err)
	}
	if chain != nil {
		t.Fatal("buildAuthChain with no lane configured returned a non-nil chain")
	}
}

// TestBuildAuthChainStaticLaneAcceptsConfiguredToken proves buildAuthChain's
// static-lane construction, driven through the real config-string ->
// ParseServiceStaticTokens -> NewStaticTokenVerifier path (mirroring
// TestWithAuth_StaticTokenLane_AuthenticatesConfiguredToken, but exercising
// the extracted builder directly): a chain built from a static-tokens
// config resolves the configured token to its namespaced owner.
func TestBuildAuthChainStaticLaneAcceptsConfiguredToken(t *testing.T) {
	svcAuth := config.ServiceAuthConfig{StaticTokens: "owner-y=another-secret"}
	chain, err := buildAuthChain(config.OIDCConfig{}, svcAuth, nil)
	if err != nil {
		t.Fatalf("buildAuthChain: %v", err)
	}
	if chain == nil {
		t.Fatal("buildAuthChain returned nil for a configured static lane")
	}
	ti, err := chain(context.Background(), "another-secret", httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("chain rejected the configured static token: %v", err)
	}
	owner, _ := ti.Extra[auth.OwnerClaimExtraKey].(string)
	want := wantNamespacedOwner("static_token", "owner-y")
	if owner != want {
		t.Fatalf("resolved owner = %q, want %q", owner, want)
	}
}

// TestComposeAuthChainWrapsWithExpiry proves the expiry-enforcement
// obligation REVIEWS.md MED-9 flags as infeasible against buildAuthChain's
// config-only signature: composeAuthChain(stub, nil, nil), where stub
// returns a past-Expiration TokenInfo with a nil error, rejects with
// errors.Is(err, auth.ErrTokenExpired). buildAuthChain itself can only
// construct real OIDC/static verifiers from configuration, so no test
// against it could produce a past-expiration TokenInfo to prove this with;
// this test carries that obligation instead, against the pure composition
// step.
func TestComposeAuthChainWrapsWithExpiry(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	stub := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{Expiration: past}, nil
	}
	chain := composeAuthChain(stub, nil, nil)
	if chain == nil {
		t.Fatal("composeAuthChain(stub, nil, nil) = nil, want a non-nil verifier")
	}
	// JWT-shaped (exactly two dots) so auth.ChainVerifier's structural
	// discriminator routes the bearer to the human lane, where stub runs.
	_, err := chain(context.Background(), "aaa.bbb.ccc", httptest.NewRequest(http.MethodGet, "/", nil))
	if err == nil {
		t.Fatal("expected the composed verifier to reject a past-Expiration TokenInfo, got nil error")
	}
	if !errors.Is(err, auth.ErrTokenExpired) {
		t.Fatalf("err = %v, want errors.Is(err, auth.ErrTokenExpired)", err)
	}
}

// TestComposeAuthChainAllNilReturnsNil proves "no lane configured" still
// propagates as a nil chain through the extracted composition step.
func TestComposeAuthChainAllNilReturnsNil(t *testing.T) {
	if chain := composeAuthChain(nil, nil, nil); chain != nil {
		t.Fatal("composeAuthChain(nil, nil, nil) != nil, want nil")
	}
}

// TestBuildAuthChainDelegatesComposition is a structural pin (REVIEWS.md
// MED-9's companion obligation): buildAuthChain calls composeAuthChain
// exactly once and applies no auth.EnforceExpiry wrapping of its own —
// expiry wrapping is composeAuthChain's sole responsibility. Recorded as a
// named test so this obligation is visible in the suite even though its
// evidence is a source read, not a runtime assertion.
func TestBuildAuthChainDelegatesComposition(t *testing.T) {
	src, err := os.ReadFile("serve.go")
	if err != nil {
		t.Fatalf("read serve.go: %v", err)
	}
	text := string(src)

	buildStart := strings.Index(text, "\nfunc buildAuthChain(")
	composeStart := strings.Index(text, "\nfunc composeAuthChain(")
	withAuthStart := strings.Index(text, "\nfunc withAuth(")
	if buildStart < 0 || composeStart < 0 || withAuthStart < 0 {
		t.Fatal("could not locate buildAuthChain/composeAuthChain/withAuth in serve.go")
	}
	if buildStart >= composeStart || composeStart >= withAuthStart {
		t.Fatalf("expected declaration order buildAuthChain < composeAuthChain < withAuth, got offsets %d/%d/%d", buildStart, composeStart, withAuthStart)
	}

	buildBody := text[buildStart:composeStart]
	composeBody := text[composeStart:withAuthStart]

	if n := strings.Count(buildBody, "composeAuthChain("); n != 1 {
		t.Fatalf("buildAuthChain body calls composeAuthChain( %d times, want exactly 1", n)
	}
	if strings.Count(buildBody, "auth.EnforceExpiry(") != 0 {
		t.Fatal("buildAuthChain must not call auth.EnforceExpiry directly -- that belongs solely to composeAuthChain")
	}
	if n := strings.Count(composeBody, "auth.EnforceExpiry("); n != 1 {
		t.Fatalf("composeAuthChain body calls auth.EnforceExpiry( %d times, want exactly 1", n)
	}
}

// TestAuthChainSharedBetweenLanes is the D-06 drift-impossibility proof: one
// buildAuthChain call produces one verifier value, and that SAME value is
// handed to withAuth (the MCP wrapper) and to server.NewConnectResolver
// (the Connect bearer half) — no second construction call. The same
// configured static token authenticates through both: the MCP handler
// returns non-401, and the composed Connect resolver returns a TokenInfo
// stamped auth.LaneBearer.
func TestAuthChainSharedBetweenLanes(t *testing.T) {
	svcAuth := config.ServiceAuthConfig{StaticTokens: "owner-x=shared-secret-token"}
	chain, err := buildAuthChain(config.OIDCConfig{}, svcAuth, nil)
	if err != nil {
		t.Fatalf("buildAuthChain: %v", err)
	}
	if chain == nil {
		t.Fatal("buildAuthChain returned a nil chain for a configured static lane")
	}

	// MCP half: the same chain value wrapped by withAuth.
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	mcpHandler := withAuth(inner, chain, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer shared-secret-token")
	mcpHandler.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("MCP lane rejected the configured token, got %d", rec.Code)
	}

	// Connect half: the SAME chain value, no second buildAuthChain call.
	resolve := server.NewConnectResolver(chain, nil)
	creq := connect.NewRequest(&struct{}{})
	creq.Header().Set("Authorization", "Bearer shared-secret-token")
	ti, lane, err := resolve(context.Background(), creq)
	if err != nil {
		t.Fatalf("Connect resolver rejected the configured token: %v", err)
	}
	if lane != auth.LaneBearer {
		t.Fatalf("lane = %v, want LaneBearer", lane)
	}
	if ti == nil {
		t.Fatal("expected a non-nil TokenInfo from the Connect lane")
	}
}

// stubConnectVerifier returns an mcpauth.TokenVerifier that accepts any
// non-empty token and resolves it to a fixed TokenInfo. Used where a test
// only needs "a non-nil configured chain that authenticates", not a
// specific credential check (connectResolverFor's own tests exercise the
// credential-shape routing; these fixtures stay deliberately simple).
func stubConnectVerifier() mcpauth.TokenVerifier {
	return func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{UserID: "stub-bearer-user"}, nil
	}
}

// stubConnectCookieResolve returns a cookie-lane resolver stub that always
// succeeds, for tests exercising connectResolverFor's cookie half.
func stubConnectCookieResolve() func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error) {
	return func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{UserID: "stub-cookie-user"}, nil
	}
}

// TestHeadlessRefusesStartWithoutAuthLane pins D-11: headless with a nil
// chain refuses to start, naming ENGRAM_CONNECT_HEADLESS; headless with a
// configured chain is fine; and non-headless with no chain is fine too --
// this guard constrains only the new flag, so no existing deployment can
// fail to boot as a result of this phase.
func TestHeadlessRefusesStartWithoutAuthLane(t *testing.T) {
	err := connectHeadlessGuard(true, nil)
	if err == nil {
		t.Fatal("connectHeadlessGuard(true, nil): want error, got nil")
	}
	if !strings.Contains(err.Error(), "ENGRAM_CONNECT_HEADLESS") {
		t.Fatalf("error %v does not name ENGRAM_CONNECT_HEADLESS", err)
	}
	if err := connectHeadlessGuard(true, stubConnectVerifier()); err != nil {
		t.Fatalf("connectHeadlessGuard(true, chain): want nil, got %v", err)
	}
	if err := connectHeadlessGuard(false, nil); err != nil {
		t.Fatalf("connectHeadlessGuard(false, nil): want nil, got %v", err)
	}
}

// TestConnectResolverForDefaultOff is the Pitfall-5 regression guard (D-12 /
// SC5): a configured auth lane alone -- with no UI and no headless flag --
// must never mount Connect. This closes REQUIREMENTS.md:197's false-pass,
// where loosening the mount guard with an OR would keep every "UI enabled
// -> mounted" test green even though the new condition is wrong.
func TestConnectResolverForDefaultOff(t *testing.T) {
	if got := connectResolverFor(nil, false, nil); got != nil {
		t.Fatal("connectResolverFor(nil, false, nil) != nil, want nil")
	}
	chain := stubConnectVerifier()
	if got := connectResolverFor(chain, false, nil); got != nil {
		t.Fatal("connectResolverFor(chain, false, nil) != nil, want nil -- a configured chain alone must never mount Connect")
	}
}

// TestConnectResolverForHeadlessOnly proves ENGRAM_CONNECT_HEADLESS=true
// with no UI mounts Connect and authenticates via the bearer half alone.
func TestConnectResolverForHeadlessOnly(t *testing.T) {
	resolve := connectResolverFor(stubConnectVerifier(), true, nil)
	if resolve == nil {
		t.Fatal("connectResolverFor(chain, true, nil) = nil, want non-nil")
	}
	req := connect.NewRequest(&struct{}{})
	req.Header().Set("Authorization", "Bearer stub-token")
	ti, lane, err := resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if lane != auth.LaneBearer {
		t.Fatalf("lane = %v, want LaneBearer", lane)
	}
	if ti == nil {
		t.Fatal("expected a non-nil TokenInfo")
	}
}

// TestConnectResolverForUIOnlyNoChain proves a deployment with no auth lane
// configured at all -- UI on, chain nil -- mounts Connect and stamps
// auth.LaneCookie for every request, byte-for-byte today's UI-only
// behavior.
func TestConnectResolverForUIOnlyNoChain(t *testing.T) {
	resolve := connectResolverFor(nil, false, stubConnectCookieResolve())
	if resolve == nil {
		t.Fatal("connectResolverFor(nil, false, cookieResolve) = nil, want non-nil")
	}
	req := connect.NewRequest(&struct{}{})
	ti, lane, err := resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if lane != auth.LaneCookie {
		t.Fatalf("lane = %v, want LaneCookie", lane)
	}
	if ti == nil {
		t.Fatal("expected a non-nil TokenInfo")
	}
}

// TestConnectResolverForUIEnabledIncludesBearerHalf is the REVIEWS.md
// HIGH-3 / SC1 regression guard: UI on, headless UNSET, a chain configured
// -- the resolver must be non-nil AND route a well-formed bearer credential
// to auth.LaneBearer through chain, while a non-bearer request still stamps
// auth.LaneCookie. Gating the bearer half on headless would leave this
// deployment cookie-only, so a token accepted on MCP would be rejected on
// Connect -- the exact defect this test exists to catch.
func TestConnectResolverForUIEnabledIncludesBearerHalf(t *testing.T) {
	resolve := connectResolverFor(stubConnectVerifier(), false, stubConnectCookieResolve())
	if resolve == nil {
		t.Fatal("connectResolverFor(chain, false, cookieResolve) = nil, want non-nil")
	}

	breq := connect.NewRequest(&struct{}{})
	breq.Header().Set("Authorization", "Bearer stub-token")
	ti, lane, err := resolve(context.Background(), breq)
	if err != nil {
		t.Fatalf("resolve(bearer): %v", err)
	}
	if lane != auth.LaneBearer {
		t.Fatalf("lane = %v, want LaneBearer (a UI-enabled deployment with a configured chain must accept bearer credentials even with headless unset)", lane)
	}
	if ti == nil {
		t.Fatal("expected a non-nil TokenInfo for the bearer request")
	}

	creq := connect.NewRequest(&struct{}{})
	_, lane2, err := resolve(context.Background(), creq)
	if err != nil {
		t.Fatalf("resolve(no credential): %v", err)
	}
	if lane2 != auth.LaneCookie {
		t.Fatalf("lane = %v, want LaneCookie for a non-bearer request", lane2)
	}
}

// TestConnectResolverForBothLanes proves headless AND UI-enabled together
// still compose correctly: a bearer credential routes to auth.LaneBearer,
// everything else to auth.LaneCookie.
func TestConnectResolverForBothLanes(t *testing.T) {
	resolve := connectResolverFor(stubConnectVerifier(), true, stubConnectCookieResolve())
	if resolve == nil {
		t.Fatal("connectResolverFor(chain, true, cookieResolve) = nil, want non-nil")
	}

	breq := connect.NewRequest(&struct{}{})
	breq.Header().Set("Authorization", "Bearer stub-token")
	_, lane, err := resolve(context.Background(), breq)
	if err != nil {
		t.Fatalf("resolve(bearer): %v", err)
	}
	if lane != auth.LaneBearer {
		t.Fatalf("lane = %v, want LaneBearer", lane)
	}

	creq := connect.NewRequest(&struct{}{})
	_, lane2, err := resolve(context.Background(), creq)
	if err != nil {
		t.Fatalf("resolve(no credential): %v", err)
	}
	if lane2 != auth.LaneCookie {
		t.Fatalf("lane = %v, want LaneCookie", lane2)
	}
}

// TestConnectResolverForHeadlessWithoutChainIsNil pins the defense-in-depth
// backstop: headless=true with a nil chain is unreachable in runServe
// (connectHeadlessGuard refuses startup first), but the composition itself
// must never mount an unauthenticatable surface if that guarantee is ever
// broken.
func TestConnectResolverForHeadlessWithoutChainIsNil(t *testing.T) {
	if got := connectResolverFor(nil, true, nil); got != nil {
		t.Fatal("connectResolverFor(nil, true, nil) != nil, want nil")
	}
}

// TestServePreflightGuardsExitUsage proves the D-03 bucket-1 classification
// end-to-end: `engram serve` driven through the real rootCmd.Execute() path
// (mirroring exitcode_baseline_test.go's TestExitCodeBaseline, not a direct
// runServe(cmd) call) for guards reachable WITHOUT a live Qdrant -- each
// trips before runServe ever reaches server.Register (the bucket-2
// store-construction call) -- and checks that each message names the
// settings at fault. Bucket 3 (a listener's bind or serve failure) is not
// exercised here; TestListenAllClosesBoundListenersOnFailure and
// TestServeAllServeFailure cover it.
func TestServePreflightGuardsExitUsage(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		env      map[string]string
		wantText []string
	}{
		{
			// The D-09 baseline row this test complements: serve's own
			// listen-addr guard, the first bucket-1 site in runServe.
			name:     "empty listen addr",
			args:     []string{"serve", "--listen-addr", ""},
			wantText: []string{"ENGRAM_LISTEN_ADDR"},
		},
		{
			// resolveUIConfig (uiconfig.go): ENGRAM_UI_ENABLED=true with
			// none of the four required web-UI creds set.
			name: "ui enabled without required creds",
			args: []string{"serve", "--listen-addr", "127.0.0.1:0"},
			env:  map[string]string{"ENGRAM_UI_ENABLED": "true"},
		},
		{
			// connectHeadlessGuard: --connect-headless with no OIDC issuer
			// and no service_auth.* config, so chain is nil.
			name:     "connect-headless without a configured auth lane",
			args:     []string{"serve", "--listen-addr", "127.0.0.1:0", "--connect-headless", "true"},
			wantText: []string{"ENGRAM_CONNECT_HEADLESS"},
		},
		{
			name: "resource origins and a pinned resource URL",
			args: []string{"serve", "--listen-addr", "127.0.0.1:0"},
			env: map[string]string{
				"ENGRAM_MCP_RESOURCE_URL":     "https://a.example/mcp",
				"ENGRAM_MCP_RESOURCE_ORIGINS": "https://a.example",
			},
			wantText: []string{"ENGRAM_MCP_RESOURCE_ORIGINS", "ENGRAM_MCP_RESOURCE_URL"},
		},
		{
			name:     "same address on both listeners",
			args:     []string{"serve", "--listen-addr", "127.0.0.1:18080", "--mcp-listen-addr", "127.0.0.1:18080"},
			wantText: []string{"ENGRAM_MCP_LISTEN_ADDR", "ENGRAM_LISTEN_ADDR"},
		},
		{
			name:     "all interfaces and one host on the same port",
			args:     []string{"serve", "--listen-addr", ":18080", "--mcp-listen-addr", "127.0.0.1:18080"},
			wantText: []string{"ENGRAM_MCP_LISTEN_ADDR", "ENGRAM_LISTEN_ADDR"},
		},
		{
			name:     "MCP listen address without a port",
			args:     []string{"serve", "--listen-addr", "127.0.0.1:0", "--mcp-listen-addr", "18081"},
			wantText: []string{"ENGRAM_MCP_LISTEN_ADDR", `"18081"`},
		},
		{
			name:     "MCP-only listener without a configured auth lane",
			args:     []string{"serve", "--listen-addr", "127.0.0.1:0", "--mcp-listen-addr", "127.0.0.1:0"},
			wantText: []string{"ENGRAM_MCP_LISTEN_ADDR", "no auth lane"},
		},
		{
			name:     "resource origins entry with a path",
			args:     []string{"serve", "--listen-addr", "127.0.0.1:0"},
			env:      map[string]string{"ENGRAM_MCP_RESOURCE_ORIGINS": "https://a.example/mcp"},
			wantText: []string{"ENGRAM_MCP_RESOURCE_ORIGINS", `"https://a.example/mcp"`},
		},
		{
			name:     "malformed MCP path",
			args:     []string{"serve", "--listen-addr", "127.0.0.1:0", "--mcp-path", "mcp"},
			wantText: []string{"ENGRAM_MCP_PATH"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			resetClientFlags(t)
			resetEveryCommandFlagState(t, rootCmd)

			_, _, err := runClient(t, c.args...)
			if got := exitCodeFromError(err); got != exitUsage {
				t.Fatalf("%v: exitCodeFromError(err) = %d, want exitUsage (%d); err=%v", c.args, got, exitUsage, err)
			}
			for _, want := range c.wantText {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("%v: error %q does not contain %q", c.args, err, want)
				}
			}
		})
	}
}

func TestCheckMCPListenAddr(t *testing.T) {
	cases := []struct {
		listen, mcp string
		wantErr     bool
	}{
		{":8080", "", false},
		{":8080", ":8081", false},
		{":8080", ":8080", true},
		{":8080", "127.0.0.1:8080", true},
		{"0.0.0.0:8080", "127.0.0.1:8080", true},
		{"[::]:8080", "127.0.0.1:8080", true},
		{"127.0.0.1:8080", "127.0.0.2:8080", false},
		{"127.0.0.1:8080", "127.0.0.1:8080", true},
		{":0", ":0", false},
		{":http", ":80", true},
		{"bad", ":8080", false},
		{":8080", "8081", true},
		{":8080", ":notaport", true},
	}
	for _, tc := range cases {
		err := checkMCPListenAddr(tc.listen, tc.mcp)
		if (err != nil) != tc.wantErr {
			t.Errorf("checkMCPListenAddr(%q, %q) err=%v, wantErr=%v", tc.listen, tc.mcp, err, tc.wantErr)
		}
		if err != nil && exitCodeFromError(err) != exitUsage {
			t.Errorf("checkMCPListenAddr(%q, %q): exit code %d, want exitUsage", tc.listen, tc.mcp, exitCodeFromError(err))
		}
	}
}

func TestMCPListenerGuard(t *testing.T) {
	if err := mcpListenerGuard(":8081", nil); err == nil || !strings.Contains(err.Error(), "ENGRAM_MCP_LISTEN_ADDR") {
		t.Fatalf(`mcpListenerGuard(":8081", nil)=%v, want an error naming ENGRAM_MCP_LISTEN_ADDR`, err)
	}
	if err := mcpListenerGuard(":8081", stubConnectVerifier()); err != nil {
		t.Fatalf(`mcpListenerGuard(":8081", chain)=%v, want nil`, err)
	}
	if err := mcpListenerGuard("", nil); err != nil {
		t.Fatalf(`mcpListenerGuard("", nil)=%v, want nil`, err)
	}
}

func TestWithAuth_UnlistedOriginNoHint(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := withAuth(inner, newStaticTokenChain(t), mustResource(t, "", "https://a.example", "", "/mcp"))

	t.Run("listed origin gets the hint", func(t *testing.T) {
		rec := challengeThrough(h, "https", "a.example")
		want := `Bearer resource_metadata="https://a.example/.well-known/oauth-protected-resource/mcp"`
		if got := rec.Header().Get("WWW-Authenticate"); got != want {
			t.Fatalf("WWW-Authenticate=%q want %q", got, want)
		}
	})

	t.Run("unlisted origin without a token gets a bare challenge", func(t *testing.T) {
		rec := challengeThrough(h, "https", "c.example")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d want 401", rec.Code)
		}
		if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
			t.Fatalf("WWW-Authenticate=%q want %q", got, "Bearer")
		}
	})

	t.Run("unlisted origin with a valid token is served without a warning", func(t *testing.T) {
		logs := captureLogs(t)
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Host = "c.example"
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("Authorization", "Bearer secret-token-value")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusTeapot {
			t.Fatalf("status=%d want %d", rec.Code, http.StatusTeapot)
		}
		if strings.Contains(logs.String(), "ENGRAM_MCP_RESOURCE_ORIGINS") {
			t.Fatalf("an authenticated request logged a refused-origin warning: %s", logs.String())
		}
	})
}

// initializeRequest is an MCP initialize call for a streamable-HTTP POST.
const initializeRequest = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"serve-test","version":"1"}}}`

func postMCP(t *testing.T, base, token string, header map[string]string) (*http.Response, string) {
	t.Helper()
	return postMCPBody(t, base, token, initializeRequest, header)
}

func postMCPBody(t *testing.T, base, token, body string, header map[string]string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s/mcp: %v", base, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, string(b)
}

// openEventStream starts an MCP session on base and opens its GET event
// stream, returning the open response.
func openEventStream(t *testing.T, base, token string) *http.Response {
	t.Helper()
	resp, _ := postMCP(t, base, token, nil)
	session := resp.Header.Get("Mcp-Session-Id")
	if session == "" {
		t.Fatalf("initialize on %s returned no Mcp-Session-Id", base)
	}
	headers := map[string]string{"Mcp-Session-Id": session, "Mcp-Protocol-Version": "2025-06-18"}
	postMCPBody(t, base, token, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, headers)
	req, err := http.NewRequest(http.MethodGet, base+"/mcp", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+token)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	stream, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s/mcp: %v", base, err)
	}
	if stream.StatusCode != http.StatusOK || !strings.HasPrefix(stream.Header.Get("Content-Type"), "text/event-stream") {
		_ = stream.Body.Close()
		t.Fatalf("GET %s/mcp: status=%d Content-Type=%q, want an open event stream", base, stream.StatusCode, stream.Header.Get("Content-Type"))
	}
	return stream
}

func TestServeBothListeners(t *testing.T) {
	mcpSrv := mcp.NewServer(&mcp.Implementation{Name: "engram", Version: "test"}, nil)
	streams, endStreams := context.WithCancel(context.Background())
	defer endStreams()
	resource := mustResource(t, "", "", "", "/mcp")
	h := mcpTransport(streams, mcpSrv, newStaticTokenChain(t), resource, telemetry.NewToolMetrics(otel.Meter("test")))
	prm := protectedResourceHandler(resource, "")

	// A request that is still running when shutdown starts and finishes on
	// its own once shutdown has started: drain must see it finished.
	started, release := make(chan struct{}), make(chan struct{})
	var finished atomic.Bool
	primary := http.NewServeMux()
	primary.HandleFunc("/slow", func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
		finished.Store(true)
	})
	mountWellKnownRoutes(primary, prm, "/mcp")
	mountMCPRoutes(primary, h, true, "/mcp")
	var releaseOnce sync.Once
	listeners := newListeners(config.ServerConfig{ListenAddr: "127.0.0.1:0", MCPListenAddr: "127.0.0.1:0"}, primary, h, prm, "/mcp", func() {
		endStreams()
		releaseOnce.Do(func() { close(release) })
	})
	if len(listeners) != 2 {
		t.Fatalf("newListeners returned %d listeners, want 2", len(listeners))
	}
	if err := listenAll(listeners); err != nil {
		t.Fatalf("listenAll: %v", err)
	}
	bases := make([]string, len(listeners))
	for i, l := range listeners {
		bases[i] = "http://" + l.ln.Addr().String()
	}

	sigCtx, stop := context.WithCancel(context.Background())
	defer stop()
	var drainSawFinished bool
	done := make(chan error, 1)
	go func() {
		done <- serveAll(sigCtx, listeners, func(context.Context) { drainSawFinished = finished.Load() })
	}()

	for _, base := range bases {
		resp, body := postMCP(t, base, "secret-token-value", nil)
		if resp.StatusCode != http.StatusOK || !strings.Contains(body, `"serverInfo"`) {
			t.Fatalf("%s: initialize status=%d body=%q, want a 200 handshake", base, resp.StatusCode, body)
		}
	}

	resp, _ := postMCP(t, bases[1], "", nil)
	challenge := `Bearer resource_metadata="` + bases[1] + `/.well-known/oauth-protected-resource/mcp"`
	if resp.StatusCode != http.StatusUnauthorized || resp.Header.Get("WWW-Authenticate") != challenge {
		t.Fatalf("MCP-only listener without a token: status=%d WWW-Authenticate=%q, want 401 %q", resp.StatusCode, resp.Header.Get("WWW-Authenticate"), challenge)
	}
	doc, err := http.Get(bases[1] + "/.well-known/oauth-protected-resource/mcp")
	if err != nil {
		t.Fatalf("GET the challenge's document: %v", err)
	}
	var meta protectedResourceMetadata
	if err := json.NewDecoder(doc.Body).Decode(&meta); err != nil {
		t.Fatalf("decode the challenge's document: %v", err)
	}
	_ = doc.Body.Close()
	if doc.StatusCode != http.StatusOK || meta.Resource != bases[1]+"/mcp" {
		t.Fatalf("challenge's document: status=%d resource=%q, want 200 %q", doc.StatusCode, meta.Resource, bases[1]+"/mcp")
	}

	resp, _ = postMCP(t, bases[1], "secret-token-value", map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("MCP-only listener cross-site POST: status=%d want 403", resp.StatusCode)
	}

	openStreams := []*http.Response{openEventStream(t, bases[0], "secret-token-value"), openEventStream(t, bases[1], "secret-token-value")}
	defer func() {
		for _, s := range openStreams {
			_ = s.Body.Close()
		}
	}()
	slow := make(chan error, 1)
	go func() {
		r, err := http.Get(bases[0] + "/slow")
		if err == nil {
			_ = r.Body.Close()
		}
		slow <- err
	}()
	<-started

	stop()
	if err := <-done; err != nil {
		t.Fatalf("serveAll with open event streams: %v", err)
	}
	if err := <-slow; err != nil {
		t.Fatalf("the running request failed during shutdown: %v", err)
	}
	if !drainSawFinished {
		t.Fatal("drain ran before the running request finished")
	}
	for _, l := range listeners {
		if conn, err := net.Dial("tcp", l.ln.Addr().String()); err == nil {
			_ = conn.Close()
			t.Fatalf("%s still accepts connections after shutdown", l.ln.Addr())
		}
	}
}

func TestNewListenersDefaultIsOnePort(t *testing.T) {
	listeners := newListeners(config.ServerConfig{ListenAddr: ":8080"}, http.NewServeMux(), http.NotFoundHandler(), http.NotFoundHandler(), "/mcp", func() {})
	if len(listeners) != 1 || listeners[0].setting != "ENGRAM_LISTEN_ADDR" {
		t.Fatalf("newListeners without ENGRAM_MCP_LISTEN_ADDR = %d listeners, want only ENGRAM_LISTEN_ADDR", len(listeners))
	}
}

func TestListenAllClosesBoundListenersOnFailure(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer busy.Close()

	listeners := []*listener{
		{setting: "ENGRAM_LISTEN_ADDR", srv: newHTTPServer("127.0.0.1:0", http.NewServeMux())},
		{setting: "ENGRAM_MCP_LISTEN_ADDR", srv: newHTTPServer(busy.Addr().String(), http.NewServeMux())},
	}
	err = listenAll(listeners)
	if err == nil {
		t.Fatal("listenAll on a busy address: want an error")
	}
	if !strings.Contains(err.Error(), "ENGRAM_MCP_LISTEN_ADDR") || !strings.Contains(err.Error(), busy.Addr().String()) {
		t.Fatalf("error %q does not name ENGRAM_MCP_LISTEN_ADDR and %s", err, busy.Addr())
	}
	if got := exitCodeFromError(err); got != exitGeneric {
		t.Fatalf("exit code %d, want exitGeneric", got)
	}
	first := listeners[0].ln.Addr().String()
	if conn, err := net.Dial("tcp", first); err == nil {
		_ = conn.Close()
		t.Fatalf("first listener %s still accepts connections after the second bind failed", first)
	}
}

func TestServeAllServeFailure(t *testing.T) {
	listeners := []*listener{
		{setting: "ENGRAM_LISTEN_ADDR", srv: newHTTPServer("127.0.0.1:0", http.NewServeMux())},
		{setting: "ENGRAM_MCP_LISTEN_ADDR", srv: newHTTPServer("127.0.0.1:0", http.NewServeMux())},
	}
	if err := listenAll(listeners); err != nil {
		t.Fatalf("listenAll: %v", err)
	}
	_ = listeners[1].ln.Close()

	var drained bool
	err := serveAll(context.Background(), listeners, func(context.Context) { drained = true })
	if err == nil || !strings.Contains(err.Error(), "serve ENGRAM_MCP_LISTEN_ADDR") {
		t.Fatalf("serveAll=%v, want the serve failure named with ENGRAM_MCP_LISTEN_ADDR", err)
	}
	if got := exitCodeFromError(err); got != exitGeneric {
		t.Fatalf("exit code %d, want exitGeneric", got)
	}
	if !drained {
		t.Fatal("serveAll returned without running drain")
	}
	if conn, dialErr := net.Dial("tcp", listeners[0].ln.Addr().String()); dialErr == nil {
		_ = conn.Close()
		t.Fatalf("%s still accepts connections after the other listener failed", listeners[0].ln.Addr())
	}
}
