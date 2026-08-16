// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package e2e

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/seanb4t/engram/internal/webauth"
)

// sessionCookieName restates internal/webauth/handlers.go:17's unexported
// sessionCookieName ("engram_session"). It is unexported there, so this test
// necessarily restates the literal; a key_links entry in 05-04-PLAN.md binds
// the two so a rename on either side is caught by inspection.
const sessionCookieName = "engram_session"

// testFixtureOwner is the identity the sealed session cookie carries.
// Resolver.Resolve rejects an empty owner (verified_facts item 5), so this
// must be non-empty; it also must not collide with any other e2e fixture's
// owner, since the fixture record this test writes is scoped to it.
const testFixtureOwner = "console-e2e-owner@example.com"

// requireBrowser mirrors requireQdrant/requireBrowser's harness_test.go
// sibling byte-for-byte in shape: ENGRAM_REQUIRE_BROWSER makes a missing
// browser fatal rather than a skip, so CI cannot go green with this tier
// silently sitting out. An unparseable value is an error, never coerced to
// false.
func requireBrowser() (bool, error) {
	v := os.Getenv("ENGRAM_REQUIRE_BROWSER")
	if v == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("ENGRAM_REQUIRE_BROWSER: invalid value %q: %w", v, err)
	}
	return b, nil
}

// findChrome returns the path to a usable Chrome/Chromium executable, or ""
// if none was found.
//
// ENGRAM_CHROME_PATH, when set, is used VERBATIM with NO fallback search: if
// it does not stat cleanly, findChrome reports "no browser found" rather than
// falling back to a PATH search. This is what makes the fail-closed RED proof
// in task 3 possible — pointing ENGRAM_CHROME_PATH at a nonexistent path must
// be indistinguishable from "no browser available", not silently rescued by
// discovering a real one elsewhere on PATH.
func findChrome() string {
	if p := os.Getenv("ENGRAM_CHROME_PATH"); p != "" {
		if _, err := os.Stat(p); err != nil {
			return ""
		}
		return p
	}
	candidates := []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "headless-shell"}
	for _, c := range candidates {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	// macOS dev fallback: ubuntu-latest (CI) never reaches this branch, since
	// google-chrome/chromium are already on PATH there (verified_facts item 11).
	const macChrome = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	if _, err := os.Stat(macChrome); err == nil {
		return macChrome
	}
	return ""
}

// skipOrFailNoBrowser mirrors skipOrFailNoQdrant: a missing browser skips the
// test by default, but fails it when ENGRAM_REQUIRE_BROWSER is set — so CI
// cannot go green with this tier silently sitting out. Returns the resolved
// browser path; Skip/Fatal both exit the test via runtime.Goexit, so a caller
// reaching the return always holds a non-empty, usable path.
func skipOrFailNoBrowser(t *testing.T) string {
	t.Helper()
	required, err := requireBrowser()
	if err != nil {
		t.Fatalf("%v", err)
	}
	path := findChrome()
	if path == "" {
		if required {
			t.Fatal("no usable Chrome/Chromium found and ENGRAM_REQUIRE_BROWSER is set: failing instead of skipping (checked ENGRAM_CHROME_PATH, then PATH for google-chrome/chromium)")
		}
		t.Skip("no usable Chrome/Chromium found: set ENGRAM_CHROME_PATH or install google-chrome/chromium, or set ENGRAM_REQUIRE_BROWSER=1 to fail instead of skip")
	}
	return path
}

// oidcDiscoveryDoc is the minimal OpenID Connect discovery document go-oidc's
// provider fetch requires (verified_facts item 1): issuer plus the endpoint
// set oidc.NewProvider reads, and a signing-alg list containing RS256.
type oidcDiscoveryDoc struct {
	Issuer                           string   `json:"issuer"`
	AuthorizationEndpoint            string   `json:"authorization_endpoint"`
	TokenEndpoint                    string   `json:"token_endpoint"`
	DeviceAuthorizationEndpoint      string   `json:"device_authorization_endpoint"`
	JWKSURI                          string   `json:"jwks_uri"`
	UserInfoEndpoint                 string   `json:"userinfo_endpoint"`
	IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
}

// stubOIDCProvider serves ONLY a discovery document at
// /.well-known/openid-configuration, mirroring stubEmbedder's shape: an
// in-process httptest.Server reached by the engram SUBPROCESS over a real
// socket, registered with t.Cleanup.
//
// This stub is unavoidable, not a shortcut: cmd/engram/serve.go boots OIDC
// discovery (oidc.NewProvider) at STARTUP whenever ENGRAM_UI_ENABLED is set —
// not lazily at login — so the web UI cannot be enabled against an
// unreachable issuer (verified_facts item 1). No JWKS is ever fetched and no
// token is ever exchanged: this test performs no login, it mints a session
// directly with the same key the server was booted with (verified_facts
// item 4).
func stubOIDCProvider(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(oidcDiscoveryDoc{
			Issuer:                           srv.URL,
			AuthorizationEndpoint:            srv.URL + "/authorize",
			TokenEndpoint:                    srv.URL + "/token",
			DeviceAuthorizationEndpoint:      srv.URL + "/device",
			JWKSURI:                          srv.URL + "/jwks",
			UserInfoEndpoint:                 srv.URL + "/userinfo",
			IDTokenSigningAlgValuesSupported: []string{"RS256"},
		})
	})
	return srv
}

// consoleFixture is a running engram subprocess with the web UI enabled, plus
// the sealed session and CSRF credentials for testFixtureOwner — everything
// both the browser and a Connect client need to act as that identity.
type consoleFixture struct {
	srv           *serverProc
	owner         string
	sessionCookie string // sealed webauth.Session, engram_session cookie value
	csrfToken     string // engram_csrf cookie value AND X-CSRF-Token header value
}

// startConsoleServer boots an engram subprocess with the web UI activated
// against an in-process OIDC discovery stub, then mints a session directly
// (never through a login flow — verified_facts item 4) for testFixtureOwner.
//
// ENGRAM_OIDC_ISSUER is deliberately left unset: only ENGRAM_UI_ISSUER points
// at the stub, so the MCP bearer lane stays anonymous and no second stub is
// needed (verified_facts item 2).
func startConsoleServer(t *testing.T) *consoleFixture {
	t.Helper()

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate cookie key: %v", err)
	}

	oidc := stubOIDCProvider(t)

	srv := startServer(t, map[string]string{
		"ENGRAM_UI_ENABLED":         "true",
		"ENGRAM_UI_ISSUER":          oidc.URL,
		"ENGRAM_OIDC_CLIENT_ID":     "console-e2e-client",
		"ENGRAM_OIDC_CLIENT_SECRET": "console-e2e-secret",
		"ENGRAM_UI_REDIRECT_URL":    "http://127.0.0.1/auth/callback",
		"ENGRAM_UI_COOKIE_KEY":      hex.EncodeToString(key), // decodeCookieKey tries hex first (verified_facts item 3)
	})

	codec, err := webauth.NewSessionCodec(key)
	if err != nil {
		t.Fatalf("session codec: %v", err)
	}
	sealed, err := codec.Seal(webauth.Session{Owner: testFixtureOwner, Expiry: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatalf("seal session: %v", err)
	}

	kcsrf, err := webauth.DeriveCSRFKey(key)
	if err != nil {
		t.Fatalf("derive csrf key: %v", err)
	}
	signer, err := webauth.NewCSRFSigner(kcsrf)
	if err != nil {
		t.Fatalf("csrf signer: %v", err)
	}

	return &consoleFixture{
		srv:           srv,
		owner:         testFixtureOwner,
		sessionCookie: sealed,
		csrfToken:     signer.Token(testFixtureOwner),
	}
}

// setConsoleCookies injects the sealed session cookie and the CSRF cookie
// into the browser BEFORE navigation, matching how the server itself issues
// each: the session cookie HttpOnly (never readable by page JS), the CSRF
// cookie NOT HttpOnly (same-origin page JS must be able to read it for the
// double-submit pattern). Secure is deliberately false: the server never
// inspects the Secure attribute, which removes any dependence on Chrome's
// trustworthy-origin handling of http://127.0.0.1.
func setConsoleCookies(ctx context.Context, fixture *consoleFixture) error {
	if err := network.SetCookie(sessionCookieName, fixture.sessionCookie).
		WithDomain("127.0.0.1").WithPath("/").WithSecure(false).WithHTTPOnly(true).
		Do(ctx); err != nil {
		return fmt.Errorf("set session cookie: %w", err)
	}
	if err := network.SetCookie(webauth.CSRFCookieName, fixture.csrfToken).
		WithDomain("127.0.0.1").WithPath("/").WithSecure(false).WithHTTPOnly(false).
		Do(ctx); err != nil {
		return fmt.Errorf("set csrf cookie: %w", err)
	}
	return nil
}

// hydrationPollExpr is satisfied ONLY after SvelteKit hydrates: the shipped
// static shell (internal/webauth/static/index.html) carries the console
// heading string exactly once, inside <title>, never as an <h1> — see
// ui/src/routes/+page.svelte and verified_facts item 8. A selector on
// document.title would be trivially satisfied by the static shell alone and
// would prove nothing; querying for an <h1> is the load-bearing choice here.
const hydrationPollExpr = `(() => {
	const h1 = document.querySelector('h1');
	return !!h1 && h1.textContent.includes('operator console');
})()`

// TestConsoleBundleRendersRecordInBrowser drives a REAL headless Chrome
// against the REAL engram binary serving its REAL embedded console bundle,
// and asserts the SvelteKit SPA hydrates. See G-05-9 / GH #106: nothing else
// in this repo renders the embedded bundle in a browser, so nothing else can
// catch a dangling chunk reference or a dropped `all:` embed prefix.
func TestConsoleBundleRendersRecordInBrowser(t *testing.T) {
	chromePath := skipOrFailNoBrowser(t) // before startServer: a browser-less run must not pay for a Qdrant boot.
	fixture := startConsoleServer(t)

	allocOpts := append(append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...),
		chromedp.NoSandbox, // Ubuntu 24.04 restricts unprivileged user namespaces for a non-root CI user (verified_facts item 10).
		chromedp.ExecPath(chromePath),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	t.Cleanup(allocCancel)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	t.Cleanup(browserCancel)

	runCtx, runCancel := context.WithTimeout(browserCtx, 90*time.Second)
	defer runCancel()

	setCookies := chromedp.ActionFunc(func(ctx context.Context) error {
		return setConsoleCookies(ctx, fixture)
	})

	var hydrated bool
	pollErr := chromedp.Run(runCtx,
		setCookies,
		chromedp.Navigate(fixture.srv.baseURL()+"/ui/"),
		chromedp.Poll(hydrationPollExpr, &hydrated,
			chromedp.WithPollingTimeout(45*time.Second),
			chromedp.WithPollingInterval(200*time.Millisecond),
		),
	)
	if pollErr != nil {
		logConsoleDiagnostics(t, browserCtx)
		// A redirect to /auth/login (a rejected cookie) is otherwise
		// indistinguishable from a broken bundle without this dump.
		t.Fatalf("hydration wait failed: %v", pollErr)
	}
	if !hydrated {
		t.Fatal("hydration poll returned without error but hydrated=false")
	}
}

// logConsoleDiagnostics captures the final page location and visible body
// text on failure. It uses browserCtx directly (not the already-expired
// timed-out run context) bounded by its own short timeout, so a Poll timeout
// does not also prevent the diagnostic capture from running.
func logConsoleDiagnostics(t *testing.T, browserCtx context.Context) {
	t.Helper()
	diagCtx, diagCancel := context.WithTimeout(browserCtx, 5*time.Second)
	defer diagCancel()
	var loc, body string
	_ = chromedp.Run(diagCtx, chromedp.Location(&loc))
	_ = chromedp.Run(diagCtx, chromedp.Evaluate(`document.body.innerText`, &body))
	t.Logf("diagnostics: location=%q body=%q", loc, body)
}
