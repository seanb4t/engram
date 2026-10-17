// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// listStub is a Connect stub whose ListMemories answers with no records.
func listStub() *stubEngramService {
	return &stubEngramService{
		listFn: func(context.Context, *engramv1.ListMemoriesRequest) (*engramv1.ListMemoriesResponse, error) {
			return &engramv1.ListMemoriesResponse{}, nil
		},
	}
}

// writeLogin saves a cache holding one login for server, issued by issuer,
// whose access token expires at expiry.
func writeLogin(t *testing.T, cachePath, server string, issuer *fakeIssuer, expiry time.Time, setDefault bool) {
	t.Helper()
	c := &credentialCache{Servers: map[string]*cachedLogin{
		cacheKey(server): {
			Issuer:   issuer.srv.URL,
			ClientID: "dcr-client-1",
			TokenURL: issuer.srv.URL + "/token",
			Token:    &oauth2.Token{AccessToken: "cached-access", RefreshToken: "cached-refresh", TokenType: "Bearer", Expiry: expiry},
		},
	}}
	if setDefault {
		c.DefaultServer = cacheKey(server)
	}
	if err := c.save(cachePath); err != nil {
		t.Fatalf("save cache: %v", err)
	}
}

func refreshCount(f *fakeIssuer) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.refreshes
}

func TestCredentialEnvironmentTokenWins(t *testing.T) {
	cachePath := loginTestEnv(t)
	issuer := newFakeIssuer(t)
	stub := listStub()
	server := startStubServer(t, stub)
	writeLogin(t, cachePath, server, issuer, time.Now().Add(-time.Hour), false)
	t.Setenv("ENGRAM_TOKEN", "env-token")

	if _, _, err := runClient(t, "list", "--server", server, "--scope", "s"); err != nil {
		t.Fatalf("list: %v", err)
	}
	if stub.lastAuthHeader != "Bearer env-token" {
		t.Errorf("Authorization = %q, want Bearer env-token", stub.lastAuthHeader)
	}
	if n := refreshCount(issuer); n != 0 {
		t.Errorf("issuer saw %d refreshes, want none", n)
	}
}

func TestCredentialTokenFileWinsAndCacheIsNotRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	cachePath := loginTestEnv(t)
	issuer := newFakeIssuer(t)
	stub := listStub()
	server := startStubServer(t, stub)
	writeLogin(t, cachePath, server, issuer, time.Now().Add(time.Hour), false)
	// A cache others can read would be refused if it were read at all.
	if err := os.Chmod(cachePath, 0o644); err != nil {
		t.Fatal(err)
	}
	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte("file-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, _, err := runClient(t, "list", "--server", server, "--scope", "s", "--token-file", tokenFile); err != nil {
		t.Fatalf("list: %v", err)
	}
	if stub.lastAuthHeader != "Bearer file-token" {
		t.Errorf("Authorization = %q, want Bearer file-token", stub.lastAuthHeader)
	}
}

func TestCredentialCacheUsedWithoutRefresh(t *testing.T) {
	cachePath := loginTestEnv(t)
	issuer := newFakeIssuer(t)
	stub := listStub()
	server := startStubServer(t, stub)
	writeLogin(t, cachePath, server, issuer, time.Now().Add(time.Hour), false)

	if _, _, err := runClient(t, "list", "--server", server, "--scope", "s"); err != nil {
		t.Fatalf("list: %v", err)
	}
	if stub.lastAuthHeader != "Bearer cached-access" {
		t.Errorf("Authorization = %q, want Bearer cached-access", stub.lastAuthHeader)
	}
	if n := refreshCount(issuer); n != 0 {
		t.Errorf("issuer saw %d refreshes for an unexpired token, want none", n)
	}
}

func TestCredentialNothingSetSendsNoHeader(t *testing.T) {
	loginTestEnv(t)
	stub := listStub()
	server := startStubServer(t, stub)

	if _, _, err := runClient(t, "list", "--server", server, "--scope", "s"); err != nil {
		t.Fatalf("list: %v", err)
	}
	if stub.lastAuthHeader != "" {
		t.Errorf("Authorization = %q, want no header", stub.lastAuthHeader)
	}
}

func TestRefreshSilentAndSaved(t *testing.T) {
	cachePath := loginTestEnv(t)
	issuer := newFakeIssuer(t)
	stub := listStub()
	server := startStubServer(t, stub)
	writeLogin(t, cachePath, server, issuer, time.Now().Add(-time.Minute), false)

	_, stderr, err := runClient(t, "list", "--server", server, "--scope", "s")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if stub.lastAuthHeader != "Bearer refreshed-access-1" {
		t.Errorf("Authorization = %q, want Bearer refreshed-access-1", stub.lastAuthHeader)
	}
	if strings.Contains(strings.ToLower(stderr), "refresh") {
		t.Errorf("stderr mentions the refresh: %q", stderr)
	}
	e := mustLoadCache(t, cachePath).Servers[server]
	if e == nil || e.Token.AccessToken != "refreshed-access-1" || e.Token.RefreshToken != "refreshed-refresh-1" {
		t.Errorf("cache entry after refresh = %+v, want the refreshed tokens saved", e)
	}
}

func TestRefreshRejectedDoesNotCallServer(t *testing.T) {
	for _, code := range []string{"invalid_grant", "invalid_client"} {
		t.Run(code, func(t *testing.T) {
			cachePath := loginTestEnv(t)
			issuer := newFakeIssuer(t)
			issuer.refreshError = code
			stub := listStub()
			server := startStubServer(t, stub)
			writeLogin(t, cachePath, server, issuer, time.Now().Add(-time.Minute), false)

			_, _, err := runClient(t, "list", "--server", server, "--scope", "s")
			if got := exitCodeFromError(err); got != exitAuth {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, exitAuth, err)
			}
			if want := "engram login --server " + server; !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not contain %q", err, want)
			}
			if stub.listCalls != 0 {
				t.Errorf("server saw %d list calls, want none", stub.listCalls)
			}
		})
	}
}

func TestRefreshIssuerUnreachableDoesNotCallServer(t *testing.T) {
	cachePath := loginTestEnv(t)
	stub := listStub()
	server := startStubServer(t, stub)
	c := &credentialCache{Servers: map[string]*cachedLogin{
		server: {Issuer: deadServer, ClientID: "c", TokenURL: deadServer + "/token",
			Token: &oauth2.Token{AccessToken: "old", RefreshToken: "r", Expiry: time.Now().Add(-time.Minute)}},
	}}
	if err := c.save(cachePath); err != nil {
		t.Fatal(err)
	}

	_, _, err := runClient(t, "list", "--server", server, "--scope", "s")
	if got := exitCodeFromError(err); got != exitUnavailable {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, exitUnavailable, err)
	}
	if !strings.Contains(err.Error(), deadServer) {
		t.Errorf("error %q does not name the issuer", err)
	}
	if stub.listCalls != 0 {
		t.Errorf("server saw %d list calls, want none", stub.listCalls)
	}
}

func TestDefaultServerUsedWhenNoServerSet(t *testing.T) {
	cachePath := loginTestEnv(t)
	issuer := newFakeIssuer(t)
	stub := listStub()
	server := startStubServer(t, stub)
	writeLogin(t, cachePath, server, issuer, time.Now().Add(time.Hour), true)

	if _, _, err := runClient(t, "list", "--scope", "s"); err != nil {
		t.Fatalf("list: %v", err)
	}
	if stub.listCalls != 1 || stub.lastAuthHeader != "Bearer cached-access" {
		t.Errorf("default server saw %d calls with Authorization %q, want 1 call with Bearer cached-access",
			stub.listCalls, stub.lastAuthHeader)
	}
}

func TestDefaultServerLosesToEnvironment(t *testing.T) {
	cachePath := loginTestEnv(t)
	issuer := newFakeIssuer(t)
	stubA, stubB := listStub(), listStub()
	serverA, serverB := startStubServer(t, stubA), startStubServer(t, stubB)
	writeLogin(t, cachePath, serverA, issuer, time.Now().Add(time.Hour), true)
	t.Setenv("ENGRAM_SERVER_URL", serverB)

	if _, _, err := runClient(t, "list", "--scope", "s"); err != nil {
		t.Fatalf("list: %v", err)
	}
	if stubA.listCalls != 0 || stubB.listCalls != 1 {
		t.Errorf("default server saw %d calls and ENGRAM_SERVER_URL saw %d, want 0 and 1", stubA.listCalls, stubB.listCalls)
	}
}

func TestDefaultServerClearedByLogout(t *testing.T) {
	cachePath := loginTestEnv(t)
	issuer := newFakeIssuer(t)
	server := startStubServer(t, listStub())
	writeLogin(t, cachePath, server, issuer, time.Now().Add(time.Hour), true)

	if _, stderr, err := runClient(t, "logout"); err != nil {
		t.Fatalf("logout: %v (stderr=%q)", err, stderr)
	}
	resetClientFlags(t)
	_, _, err := runClient(t, "list", "--scope", "s")
	if got := exitCodeFromError(err); got != exitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, exitUsage, err)
	}
	if !strings.Contains(err.Error(), "--server") || !strings.Contains(err.Error(), "ENGRAM_SERVER_URL") {
		t.Errorf("error %q does not name --server and ENGRAM_SERVER_URL", err)
	}
}
