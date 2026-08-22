// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package e2e

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
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

// fixtureScope is the scope the seed record is written under, and the scope
// this test navigates the browser to via /ui/observe?scope=. It is shared
// between the write and the navigation so the two never drift apart.
const fixtureScope = "repo:e2e-console-roundtrip"

// consoleAssetPathPrefix is the served path prefix for every immutable SPA
// asset — the segment a key_links entry in 05-04-PLAN.md binds to
// internal/webauth/static.go through. It is the SINGLE source for the
// browser-observed-failure filter, the success counter, and the asset sweep
// below — never reassembled from fragments at any of those three sites.
const consoleAssetPathPrefix = "/ui/_app/immutable/"

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

// mintFixtureMarker returns a per-run-unique ASCII marker with enough
// entropy that it cannot collide with anything in the bundle, the shell, or
// a previous run's residue. Derived from crypto/rand, not the clock, so two
// runs inside the same nanosecond cannot collide.
func mintFixtureMarker(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("mint fixture marker: %v", err)
	}
	return "engram-e2e-marker-" + hex.EncodeToString(b)
}

// seedFixtureRecord writes one memory through the Connect API using the SAME
// identity the browser session carries (fixture.owner), so the record the
// browser later renders is reachable by the cookie-authenticated read lane —
// an MCP-written record lands in the owner=="" bucket and would be invisible
// to the console (verified_facts item 5).
//
// The write lane requires the CSRF cookie AND header to be present, equal,
// and verifiable (verified_facts item 6); both are set here. A non-empty
// response id is asserted so a silently-degraded write cannot leave the
// render assertion below to fail later for the wrong reason.
func seedFixtureRecord(ctx context.Context, t *testing.T, fixture *consoleFixture, marker string) {
	t.Helper()
	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, fixture.srv.baseURL())

	req := connect.NewRequest(&engramv1.StoreMemoryRequest{
		Content:  "e2e console round-trip fixture record",
		Scope:    fixtureScope,
		Source:   "agent-inferred",
		Category: "convention",
		Summary:  marker,
	})
	req.Header().Set("Cookie", consoleCookieHeader(fixture))
	req.Header().Set(webauth.CSRFHeaderName, fixture.csrfToken)

	resp, err := client.StoreMemory(ctx, req)
	if err != nil {
		t.Fatalf("seed fixture record: %v", err)
	}
	if resp.Msg.GetId() == "" {
		t.Fatal("seed fixture record: response carried an empty id")
	}
}

// consoleCookieHeader renders the sealed session cookie and the CSRF cookie
// as a single Cookie header value, the same shape a browser would send.
func consoleCookieHeader(fixture *consoleFixture) string {
	return (&http.Cookie{Name: sessionCookieName, Value: fixture.sessionCookie}).String() +
		"; " + (&http.Cookie{Name: webauth.CSRFCookieName, Value: fixture.csrfToken}).String()
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

// markerPollExpr is satisfied ONLY once marker is visible in the live
// rendered page text. Reading document.body.innerText (not the HTML source)
// asserts over what a human would actually see.
//
// marker is the load-bearing hook: it cannot appear in index.html, cannot
// appear in any bundled chunk, and reaches the DOM only if the bundle
// loaded, SvelteKit hydrated, the session cookie authenticated the Connect
// lane, and ListMemories returned the record — a strictly stronger proof
// than any data-testid could offer. No data-testid was added to ui/, and the
// vendored bundle is deliberately untouched by this plan.
func markerPollExpr(marker string) string {
	markerJSON, _ := json.Marshal(marker)
	return fmt.Sprintf(`(() => document.body.innerText.includes(%s))()`, markerJSON)
}

// TestConsoleBundleRendersRecordInBrowser drives a REAL headless Chrome
// against the REAL engram binary serving its REAL embedded console bundle,
// and asserts the round trip: a record written moments earlier through the
// Connect API by the SAME identity the browser session carries is visible in
// the rendered DOM. See G-05-9 / GH #106: nothing else in this repo renders
// the embedded bundle in a browser, so nothing else can catch a dangling
// chunk reference, a dropped `all:` embed prefix, or a broken data path
// between the Connect write lane and the console read lane.
//
// The browser visits TWO routes in the same session, deliberately:
//  1. /ui/ (the root route) proves hydration via the <h1> hook.
//  2. /ui/observe?scope=<fixtureScope> — the SAME link the root route's own
//     scope tile navigates to on click — proves the round trip by rendering
//     the seeded record's marker.
//
// Route 2 is required because the root route's own "Recent memories" query
// (ui/src/routes/+page.svelte's recentQ, calling listMemories with an empty
// scope and no cross_spine) predates the "scope required unless cross_spine"
// server-side constraint (proto/engram/v1/engram.proto, commit 9ba6449b,
// 2026-08-12) — SearchMemories/ListMemories deliberately do NOT infer
// cross_spine from an empty scope (internal/server/connectapi.go's D-04
// note), unlike SearchDiscoveries. That predates-the-constraint gap is a
// REAL, currently-shipped console bug this test discovered (its "recent
// memories" panel always errors "scope is required unless cross_spine is
// true" — confirmed live in this run's diagnostic dump), which is exactly
// what an e2e test that actually renders is FOR (G9-D3). Fixing ui/ source
// is out of this plan's file scope (frontmatter prohibitions), so this test
// proves the round trip via the scoped /observe route instead — a real,
// already-working, user-reachable path — and the root-route regression is
// filed separately (05-04-SUMMARY.md deviations; follow-up GitHub issue).
func TestConsoleBundleRendersRecordInBrowser(t *testing.T) {
	chromePath := skipOrFailNoBrowser(t) // before startServer: a browser-less run must not pay for a Qdrant boot.
	fixture := startConsoleServer(t)

	marker := mintFixtureMarker(t)
	seedFixtureRecord(context.Background(), t, fixture, marker) // BEFORE navigation, so the record exists when the SPA's first listMemories fires.

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

	obs := newBrowserObserver()
	obs.attach(runCtx) // BEFORE any navigation, so the first request's events are not missed.

	var hydrated bool
	hydrateErr := chromedp.Run(runCtx,
		setCookies,
		chromedp.Navigate(fixture.srv.baseURL()+"/ui/"),
		chromedp.Poll(hydrationPollExpr, &hydrated,
			chromedp.WithPollingTimeout(45*time.Second),
			chromedp.WithPollingInterval(200*time.Millisecond),
		),
	)
	if hydrateErr != nil {
		logConsoleDiagnostics(browserCtx, t)
		// A redirect to /auth/login (a rejected cookie) is otherwise
		// indistinguishable from a broken bundle without this dump.
		t.Fatalf("hydration wait failed: %v", hydrateErr)
	}
	if !hydrated {
		t.Fatal("hydration poll returned without error but hydrated=false")
	}

	observeURL := fixture.srv.baseURL() + "/ui/observe?scope=" + url.QueryEscape(fixtureScope)
	var rendered bool
	renderErr := chromedp.Run(runCtx,
		chromedp.Navigate(observeURL),
		chromedp.Poll(markerPollExpr(marker), &rendered,
			chromedp.WithPollingTimeout(45*time.Second),
			chromedp.WithPollingInterval(200*time.Millisecond),
		),
	)
	if renderErr != nil {
		logConsoleDiagnostics(browserCtx, t)
		t.Fatalf("render wait failed: %v", renderErr)
	}
	if !rendered {
		t.Fatal("render poll returned without error but rendered=false")
	}

	obs.assertClean(t)
	sweepConsoleAssets(t, fixture.srv.baseURL())
}

// logConsoleDiagnostics captures the final page location and visible body
// text on failure. It uses browserCtx directly (not the already-expired
// timed-out run context) bounded by its own short timeout, so a Poll timeout
// does not also prevent the diagnostic capture from running.
func logConsoleDiagnostics(browserCtx context.Context, t *testing.T) {
	t.Helper()
	diagCtx, diagCancel := context.WithTimeout(browserCtx, 5*time.Second)
	defer diagCancel()
	var loc, body string
	_ = chromedp.Run(diagCtx, chromedp.Location(&loc))
	_ = chromedp.Run(diagCtx, chromedp.Evaluate(`document.body.innerText`, &body))
	t.Logf("diagnostics: location=%q body=%q", loc, body)
}

// browserObserver records every network failure and uncaught JS exception the
// browser reports across BOTH navigations in this test. Its callback runs
// SYNCHRONOUSLY on chromedp's event goroutine (chromedp.ListenTarget's
// contract), so it does nothing but append to guarded maps/slices — never a
// CDP action, never a blocking call.
type browserObserver struct {
	mu sync.Mutex

	urlsByRequest  map[network.RequestID]string
	failedURLs     map[string]string   // url -> reason (HTTP status or loading-failed error text)
	successAppURLs map[string]struct{} // distinct _app/immutable URLs that loaded with a non-error status
	exceptions     []string
}

func newBrowserObserver() *browserObserver {
	return &browserObserver{
		urlsByRequest:  make(map[network.RequestID]string),
		failedURLs:     make(map[string]string),
		successAppURLs: make(map[string]struct{}),
	}
}

// attach registers the observer on ctx. Must be called BEFORE navigation:
// chromedp enables the network/runtime CDP domains by default, so events for
// the very first request are only visible if the listener is already
// registered when that request fires.
func (o *browserObserver) attach(ctx context.Context) {
	chromedp.ListenTarget(ctx, func(ev any) {
		o.mu.Lock()
		defer o.mu.Unlock()
		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			if e.Request != nil {
				o.urlsByRequest[e.RequestID] = e.Request.URL
			}
		case *network.EventResponseReceived:
			if e.Response == nil {
				return
			}
			u := e.Response.URL
			switch {
			case e.Response.Status >= 400:
				o.failedURLs[u] = fmt.Sprintf("HTTP %d", e.Response.Status)
			case strings.Contains(u, consoleAssetPathPrefix):
				o.successAppURLs[u] = struct{}{}
			}
		case *network.EventLoadingFailed:
			u := o.urlsByRequest[e.RequestID]
			if u == "" {
				u = string(e.RequestID)
			}
			o.failedURLs[u] = e.ErrorText
		case *runtime.EventExceptionThrown:
			if e.ExceptionDetails != nil {
				o.exceptions = append(o.exceptions, e.ExceptionDetails.Error())
			}
		}
	})
}

// assertClean fails t if the browser observed any failed request under
// consoleAssetPathPrefix, any uncaught JS exception, or — the non-vacuity
// check — ZERO successfully-loaded _app/immutable URLs. Zero observed
// requests trivially yields zero failures, and shipping that shape is
// exactly how this repo has produced false-green gates before; the
// non-emptiness assertion is part of the gate, not an extra.
func (o *browserObserver) assertClean(t *testing.T) {
	t.Helper()
	o.mu.Lock()
	defer o.mu.Unlock()

	var offenders []string
	for u, reason := range o.failedURLs {
		if strings.Contains(u, consoleAssetPathPrefix) {
			offenders = append(offenders, fmt.Sprintf("%s: %s", u, reason))
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("browser observed failed requests under %s: %v", consoleAssetPathPrefix, offenders)
	}
	if len(o.exceptions) > 0 {
		t.Fatalf("browser observed uncaught JS exceptions: %v", o.exceptions)
	}
	if len(o.successAppURLs) == 0 {
		t.Fatalf("zero %s URLs were observed loading successfully — non-vacuity check failed (zero requests trivially yields zero failures)", consoleAssetPathPrefix)
	}
}

// immutableAssetRefPattern extracts every quoted string containing
// consoleAssetPathPrefix from served HTML — this catches both
// href="/ui/_app/immutable/...".modulepreload/stylesheet links AND the
// bootstrap script's import("/ui/_app/immutable/...") calls, both of which
// internal/webauth/static/index.html carries (see its source).
var immutableAssetRefPattern = regexp.MustCompile(`["']([^"']*` + regexp.QuoteMeta(consoleAssetPathPrefix) + `[^"']*)["']`)

// extractImmutableAssetRefs returns the distinct, in-order set of
// consoleAssetPathPrefix references found in html.
func extractImmutableAssetRefs(html string) []string {
	matches := immutableAssetRefPattern.FindAllStringSubmatch(html, -1)
	seen := make(map[string]struct{}, len(matches))
	refs := make([]string, 0, len(matches))
	for _, m := range matches {
		ref := m[1]
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		refs = append(refs, ref)
	}
	return refs
}

// sweepConsoleAssets closes the UAT's literal `missing:` item (G-05-9):
// parses the SERVED index.html for every immutable-asset reference and
// requests each one, asserting HTTP 200 and a non-empty body. This is
// supplementary to the render assertions above, never a substitute for them
// (G9-D3) — it is what would have caught GH #106 MECHANICALLY, while the
// render half above is what proves the console actually works.
func sweepConsoleAssets(t *testing.T, baseURL string) {
	t.Helper()

	resp, err := http.Get(baseURL + "/ui/")
	if err != nil {
		t.Fatalf("GET /ui/: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read /ui/ body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /ui/: status %d", resp.StatusCode)
	}

	refs := extractImmutableAssetRefs(string(body))
	if len(refs) == 0 {
		t.Fatal("extracted zero immutable-asset references from the served index.html — an empty parse is a FAILURE, not a pass")
	}

	var offenders []string
	for _, ref := range refs {
		assetURL := baseURL + ref
		aResp, aErr := http.Get(assetURL)
		if aErr != nil {
			offenders = append(offenders, fmt.Sprintf("%s: %v", assetURL, aErr))
			continue
		}
		aBody, _ := io.ReadAll(aResp.Body)
		_ = aResp.Body.Close()
		if aResp.StatusCode != http.StatusOK || len(aBody) == 0 {
			offenders = append(offenders, fmt.Sprintf("%s: status=%d bodyLen=%d", assetURL, aResp.StatusCode, len(aBody)))
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("stale or missing immutable asset references: %v", offenders)
	}
}
