// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc/oidctest"
	"golang.org/x/oauth2"

	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
)

const fakeIssuerKeyID = "login-test-key"

// fakeIssuer is an in-process OIDC issuer with the endpoints engram login
// and logout use: discovery, JWKS, RFC 7591 registration, RFC 7592
// deletion, device authorization, token (refresh only — the device poll is
// answered by stubDevicePoll) and RFC 7009 revocation.
type fakeIssuer struct {
	srv  *httptest.Server
	key  *rsa.PrivateKey
	keys *oidctest.Server

	noRegistration bool
	noDevice       bool
	refreshError   string // OAuth error code the token endpoint returns for a refresh

	mu             sync.Mutex
	clients        map[string]bool
	registrations  int
	deviceRequests int
	refreshes      int
	revoked        []string
	deleted        []string
	minted         int
}

func newFakeIssuer(t *testing.T) *fakeIssuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	f := &fakeIssuer{
		key:     key,
		keys:    &oidctest.Server{PublicKeys: []oidctest.PublicKey{{PublicKey: key.Public(), KeyID: fakeIssuerKeyID, Algorithm: oidc.RS256}}},
		clients: map[string]bool{"engram-cli": true},
	}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	f.keys.SetIssuer(f.srv.URL)
	return f
}

func (f *fakeIssuer) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.URL.Path == "/.well-known/openid-configuration":
		doc := map[string]any{
			"issuer":                                f.srv.URL,
			"authorization_endpoint":                f.srv.URL + "/auth",
			"token_endpoint":                        f.srv.URL + "/token",
			"jwks_uri":                              f.srv.URL + "/keys",
			"revocation_endpoint":                   f.srv.URL + "/revoke",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}
		if !f.noDevice {
			doc["device_authorization_endpoint"] = f.srv.URL + "/device"
		}
		if !f.noRegistration {
			doc["registration_endpoint"] = f.srv.URL + "/register"
		}
		writeJSON(w, http.StatusOK, doc)
	case r.URL.Path == "/keys":
		f.keys.ServeHTTP(w, r)
	case r.URL.Path == "/register" && r.Method == http.MethodPost:
		f.registrations++
		id := fmt.Sprintf("dcr-client-%d", f.registrations)
		f.clients[id] = true
		writeJSON(w, http.StatusCreated, map[string]any{
			"client_id":                 id,
			"registration_access_token": "rat-" + id,
			"registration_client_uri":   f.srv.URL + "/clients/" + id,
		})
	case strings.HasPrefix(r.URL.Path, "/clients/") && r.Method == http.MethodDelete:
		id := strings.TrimPrefix(r.URL.Path, "/clients/")
		if r.Header.Get("Authorization") != "Bearer rat-"+id {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		delete(f.clients, id)
		f.deleted = append(f.deleted, id)
		w.WriteHeader(http.StatusNoContent)
	case r.URL.Path == "/device":
		_ = r.ParseForm()
		if !f.clients[r.Form.Get("client_id")] {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_client"})
			return
		}
		f.deviceRequests++
		writeJSON(w, http.StatusOK, map[string]any{
			"device_code":               "device-code",
			"user_code":                 "ABCD-EFGH",
			"verification_uri":          f.srv.URL + "/activate",
			"verification_uri_complete": f.srv.URL + "/activate?user_code=ABCD-EFGH",
			"expires_in":                600,
			"interval":                  5,
		})
	case r.URL.Path == "/token":
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "refresh_token" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported_grant_type"})
			return
		}
		if f.refreshError != "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": f.refreshError})
			return
		}
		f.refreshes++
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token":  fmt.Sprintf("refreshed-access-%d", f.refreshes),
			"refresh_token": fmt.Sprintf("refreshed-refresh-%d", f.refreshes),
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	case r.URL.Path == "/revoke":
		_ = r.ParseForm()
		f.revoked = append(f.revoked, r.Form.Get("token"))
		w.WriteHeader(http.StatusOK)
	default:
		http.NotFound(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// mint returns the token set an approved device login would get, with an
// ID token signed for clientID.
func (f *fakeIssuer) mint(clientID string, withRefresh bool) *oauth2.Token {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.minted++
	now := time.Now()
	claims := fmt.Sprintf(`{"iss":%q,"aud":%q,"sub":"user-1","email":"user@example.com","email_verified":true,"iat":%d,"exp":%d}`,
		f.srv.URL, clientID, now.Unix(), now.Add(time.Hour).Unix())
	tok := &oauth2.Token{
		AccessToken: fmt.Sprintf("device-access-%d", f.minted),
		TokenType:   "Bearer",
		Expiry:      now.Add(time.Hour),
	}
	if withRefresh {
		tok.RefreshToken = fmt.Sprintf("device-refresh-%d", f.minted)
	}
	return tok.WithExtra(map[string]any{"id_token": oidctest.SignIDToken(f.key, fakeIssuerKeyID, oidc.RS256, claims)})
}

func (f *fakeIssuer) counts() (registrations, deviceRequests int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.registrations, f.deviceRequests
}

// newFakeEngramServer serves the protected-resource metadata that names
// issuer (or no issuer when issuer is ""), and optionally the Connect API.
func newFakeEngramServer(t *testing.T, issuer string, stub *stubEngramService) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(wellKnownPRMBase, func(w http.ResponseWriter, _ *http.Request) {
		doc := protectedResourceMetadata{Resource: "https://engram.example/mcp", BearerMethodsSupported: []string{"header"}}
		if issuer != "" {
			doc.AuthorizationServers = []string{issuer}
		}
		writeJSON(w, http.StatusOK, doc)
	})
	if stub != nil {
		path, handler := engramv1connect.NewEngramServiceHandler(stub)
		mux.Handle(path, handler)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

// stubDevicePoll answers the device poll at once with result, or with the
// issuer's minted token when result is nil.
func stubDevicePoll(t *testing.T, f *fakeIssuer, withRefresh bool, result error) {
	t.Helper()
	orig := pollDeviceToken
	pollDeviceToken = func(_ context.Context, conf *oauth2.Config, _ *oauth2.DeviceAuthResponse) (*oauth2.Token, error) {
		if result != nil {
			return nil, result
		}
		return f.mint(conf.ClientID, withRefresh), nil
	}
	t.Cleanup(func() { pollDeviceToken = orig })
}

// loginTestEnv isolates one test's login cache and blanks the client env
// vars a developer's shell might set.
func loginTestEnv(t *testing.T) (cachePath string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("ENGRAM_SERVER_URL", "")
	t.Setenv("ENGRAM_TOKEN", "")
	t.Setenv("ENGRAM_CLIENT_ID", "")
	resetClientFlags(t)
	return filepath.Join(dir, "engram", "credentials.json")
}

func mustLoadCache(t *testing.T, path string) *credentialCache {
	t.Helper()
	c, err := loadCredentialCache(path)
	if err != nil {
		t.Fatalf("loadCredentialCache: %v", err)
	}
	return c
}

func TestLoginApprovedCachesCredentialAndDefaultServer(t *testing.T) {
	cachePath := loginTestEnv(t)
	issuer := newFakeIssuer(t)
	server := newFakeEngramServer(t, issuer.srv.URL, nil)
	stubDevicePoll(t, issuer, true, nil)

	stdout, stderr, err := runClient(t, "login", "--server", server+"/")
	if err != nil {
		t.Fatalf("login: %v (stderr=%q)", err, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, issuer.srv.URL+"/activate?user_code=ABCD-EFGH") || !strings.Contains(stderr, "ABCD-EFGH") {
		t.Errorf("stderr does not show the verification URL and code: %q", stderr)
	}
	if want := "Logged in to " + server + " as user@example.com."; !strings.Contains(stderr, want) {
		t.Errorf("stderr = %q, want it to contain %q", stderr, want)
	}
	for _, secret := range []string{"device-access-", "device-refresh-", "rat-dcr-client"} {
		if strings.Contains(stdout+stderr, secret) {
			t.Errorf("output contains token material %q", secret)
		}
	}

	c := mustLoadCache(t, cachePath)
	if c.DefaultServer != server {
		t.Errorf("DefaultServer = %q, want %q", c.DefaultServer, server)
	}
	e := c.Servers[server]
	if e == nil {
		t.Fatalf("no cache entry for %s: %+v", server, c.Servers)
	}
	if e.Issuer != issuer.srv.URL || e.ClientID != "dcr-client-1" || !e.Registered ||
		e.RegistrationAccessToken != "rat-dcr-client-1" || e.TokenURL != issuer.srv.URL+"/token" ||
		e.RevocationURL != issuer.srv.URL+"/revoke" || e.Token == nil || e.Token.RefreshToken == "" {
		t.Errorf("cache entry = %+v", e)
	}
}

func TestLoginWithoutServerSendsNoRequest(t *testing.T) {
	loginTestEnv(t)
	issuer := newFakeIssuer(t)
	_, _, err := runClient(t, "login")
	if got := exitCodeFromError(err); got != exitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, exitUsage, err)
	}
	if !strings.Contains(err.Error(), "--server") || !strings.Contains(err.Error(), "ENGRAM_SERVER_URL") {
		t.Errorf("error %q does not name --server and ENGRAM_SERVER_URL", err)
	}
	if reg, dev := issuer.counts(); reg != 0 || dev != 0 {
		t.Errorf("issuer saw %d registrations and %d device requests, want none", reg, dev)
	}
}

func TestLoginServerWithoutAuthorizationServer(t *testing.T) {
	cachePath := loginTestEnv(t)
	server := newFakeEngramServer(t, "", nil)
	_, _, err := runClient(t, "login", "--server", server)
	if got := exitCodeFromError(err); got != exitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, exitUsage, err)
	}
	if !strings.Contains(err.Error(), "anonymous") {
		t.Errorf("error %q does not say the server accepts anonymous calls", err)
	}
	if c := mustLoadCache(t, cachePath); len(c.Servers) != 0 {
		t.Errorf("cache = %+v, want empty", c)
	}
}

func TestLoginServerMetadataUnavailable(t *testing.T) {
	loginTestEnv(t)
	notFound := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(notFound.Close)

	for name, server := range map[string]string{"metadata 404": notFound.URL, "transport failure": deadServer} {
		t.Run(name, func(t *testing.T) {
			resetClientFlags(t)
			_, _, err := runClient(t, "login", "--server", server)
			if got := exitCodeFromError(err); got != exitUnavailable {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, exitUnavailable, err)
			}
			if !strings.Contains(err.Error(), wellKnownPRMBase) {
				t.Errorf("error %q does not name the metadata URL", err)
			}
		})
	}
}

func TestLoginInsecureNeverAppliesToIssuer(t *testing.T) {
	loginTestEnv(t)
	tlsIssuer := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(tlsIssuer.Close)
	mux := http.NewServeMux()
	mux.HandleFunc(wellKnownPRMBase, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, protectedResourceMetadata{AuthorizationServers: []string{tlsIssuer.URL}})
	})
	tlsServer := httptest.NewTLSServer(mux)
	t.Cleanup(tlsServer.Close)

	_, stderr, err := runClient(t, "login", "--server", tlsServer.URL, "--insecure")
	if got := exitCodeFromError(err); got != exitUnavailable {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, exitUnavailable, err)
	}
	if !strings.Contains(err.Error(), tlsIssuer.URL) {
		t.Errorf("error %q does not name the issuer", err)
	}
	if !strings.Contains(stderr, "WARNING") {
		t.Errorf("stderr = %q, want the --insecure warning", stderr)
	}
}

func TestLoginRegistersOnceAndReuses(t *testing.T) {
	cachePath := loginTestEnv(t)
	issuer := newFakeIssuer(t)
	server := newFakeEngramServer(t, issuer.srv.URL, nil)
	stubDevicePoll(t, issuer, true, nil)

	if _, stderr, err := runClient(t, "login", "--server", server); err != nil {
		t.Fatalf("first login: %v (stderr=%q)", err, stderr)
	}
	if reg, _ := issuer.counts(); reg != 1 {
		t.Fatalf("registrations after first login = %d, want 1", reg)
	}

	resetClientFlags(t)
	if _, stderr, err := runClient(t, "login", "--server", server); err != nil {
		t.Fatalf("second login: %v (stderr=%q)", err, stderr)
	}
	if reg, _ := issuer.counts(); reg != 1 {
		t.Errorf("registrations after second login = %d, want 1 (client reused)", reg)
	}

	// The issuer forgets the client: the next login registers once more.
	issuer.mu.Lock()
	delete(issuer.clients, "dcr-client-1")
	issuer.mu.Unlock()
	resetClientFlags(t)
	if _, stderr, err := runClient(t, "login", "--server", server); err != nil {
		t.Fatalf("login after the client was deleted: %v (stderr=%q)", err, stderr)
	}
	if reg, _ := issuer.counts(); reg != 2 {
		t.Errorf("registrations after invalid_client = %d, want 2", reg)
	}
	if e := mustLoadCache(t, cachePath).Servers[server]; e == nil || e.ClientID != "dcr-client-2" {
		t.Errorf("cache entry after re-registration = %+v, want client dcr-client-2", e)
	}
}

func TestLoginPreRegisteredClientSkipsRegistration(t *testing.T) {
	cachePath := loginTestEnv(t)
	issuer := newFakeIssuer(t)
	server := newFakeEngramServer(t, issuer.srv.URL, nil)
	stubDevicePoll(t, issuer, true, nil)

	if _, stderr, err := runClient(t, "login", "--server", server, "--client-id", "engram-cli"); err != nil {
		t.Fatalf("login: %v (stderr=%q)", err, stderr)
	}
	if reg, dev := issuer.counts(); reg != 0 || dev != 1 {
		t.Errorf("registrations = %d, device requests = %d, want 0 and 1", reg, dev)
	}
	if e := mustLoadCache(t, cachePath).Servers[server]; e == nil || e.ClientID != "engram-cli" || e.Registered {
		t.Errorf("cache entry = %+v, want pre-registered client engram-cli", e)
	}
}

func TestLoginIssuerWithoutRegistration(t *testing.T) {
	cachePath := loginTestEnv(t)
	issuer := newFakeIssuer(t)
	issuer.noRegistration = true
	server := newFakeEngramServer(t, issuer.srv.URL, nil)

	_, _, err := runClient(t, "login", "--server", server)
	if got := exitCodeFromError(err); got != exitAuth {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, exitAuth, err)
	}
	if !strings.Contains(err.Error(), "--client-id") || !strings.Contains(err.Error(), "ENGRAM_CLIENT_ID") {
		t.Errorf("error %q does not name --client-id and ENGRAM_CLIENT_ID", err)
	}
	if c := mustLoadCache(t, cachePath); len(c.Servers) != 0 {
		t.Errorf("cache = %+v, want empty", c)
	}
}

func TestLoginIssuerWithoutDeviceGrant(t *testing.T) {
	loginTestEnv(t)
	issuer := newFakeIssuer(t)
	issuer.noDevice = true
	server := newFakeEngramServer(t, issuer.srv.URL, nil)

	_, _, err := runClient(t, "login", "--server", server)
	if got := exitCodeFromError(err); got != exitAuth {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, exitAuth, err)
	}
	if !strings.Contains(err.Error(), "device authorization grant") {
		t.Errorf("error %q does not name the device authorization grant", err)
	}
}

func TestLoginDeniedOrExpiredCachesNothing(t *testing.T) {
	for name, tc := range map[string]struct {
		pollErr error
		want    string
	}{
		"denied":  {&oauth2.RetrieveError{ErrorCode: "access_denied"}, "denied"},
		"expired": {context.DeadlineExceeded, "engram login again"},
	} {
		t.Run(name, func(t *testing.T) {
			cachePath := loginTestEnv(t)
			issuer := newFakeIssuer(t)
			server := newFakeEngramServer(t, issuer.srv.URL, nil)
			stubDevicePoll(t, issuer, true, tc.pollErr)

			_, _, err := runClient(t, "login", "--server", server)
			if got := exitCodeFromError(err); got != exitAuth {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, exitAuth, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err, tc.want)
			}
			if c := mustLoadCache(t, cachePath); len(c.Servers) != 0 || c.DefaultServer != "" {
				t.Errorf("cache = %+v, want empty", c)
			}
		})
	}
}

func TestLoginWithoutRefreshTokenWarns(t *testing.T) {
	loginTestEnv(t)
	issuer := newFakeIssuer(t)
	server := newFakeEngramServer(t, issuer.srv.URL, nil)
	stubDevicePoll(t, issuer, false, nil)

	_, stderr, err := runClient(t, "login", "--server", server)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if !strings.Contains(stderr, "no refresh token") {
		t.Errorf("stderr = %q, want the no-refresh-token warning", stderr)
	}
}
