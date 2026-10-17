// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestLogoutRegisteredClientRevokesAndDeletes(t *testing.T) {
	cachePath := loginTestEnv(t)
	issuer := newFakeIssuer(t)
	server := newFakeEngramServer(t, issuer.srv.URL, nil)
	stubDevicePoll(t, issuer, true, nil)
	if _, stderr, err := runClient(t, "login", "--server", server); err != nil {
		t.Fatalf("login: %v (stderr=%q)", err, stderr)
	}
	refresh := mustLoadCache(t, cachePath).Servers[server].Token.RefreshToken

	resetClientFlags(t)
	_, stderr, err := runClient(t, "logout", "--server", server)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	if !strings.Contains(stderr, "Logged out of "+server) {
		t.Errorf("stderr = %q, want the logged-out line", stderr)
	}
	issuer.mu.Lock()
	revoked, deleted := issuer.revoked, issuer.deleted
	issuer.mu.Unlock()
	if len(revoked) != 1 || revoked[0] != refresh {
		t.Errorf("revoked = %v, want the cached refresh token", revoked)
	}
	if len(deleted) != 1 || deleted[0] != "dcr-client-1" {
		t.Errorf("deleted clients = %v, want [dcr-client-1]", deleted)
	}
	if c := mustLoadCache(t, cachePath); c.Servers[server] != nil || c.DefaultServer != "" {
		t.Errorf("cache after logout = %+v, want no entry and no default server", c)
	}
}

func TestLogoutPreRegisteredClientIsNeverDeleted(t *testing.T) {
	cachePath := loginTestEnv(t)
	issuer := newFakeIssuer(t)
	c := &credentialCache{DefaultServer: "https://engram.example", Servers: map[string]*cachedLogin{
		"https://engram.example": {Issuer: issuer.srv.URL, ClientID: "engram-cli", TokenURL: issuer.srv.URL + "/token",
			RevocationURL: issuer.srv.URL + "/revoke",
			Token:         &oauth2.Token{AccessToken: "a", RefreshToken: "r", Expiry: time.Now().Add(time.Hour)}},
	}}
	if err := c.save(cachePath); err != nil {
		t.Fatal(err)
	}

	if _, _, err := runClient(t, "logout"); err != nil {
		t.Fatalf("logout: %v", err)
	}
	issuer.mu.Lock()
	revoked, deleted := issuer.revoked, issuer.deleted
	issuer.mu.Unlock()
	if len(deleted) != 0 {
		t.Errorf("deleted clients = %v, want none for a pre-registered client", deleted)
	}
	if len(revoked) != 1 {
		t.Errorf("revoked = %v, want the refresh token revoked", revoked)
	}
	if got := mustLoadCache(t, cachePath); len(got.Servers) != 0 {
		t.Errorf("cache after logout = %+v, want empty", got)
	}
}

func TestLogoutNotLoggedIn(t *testing.T) {
	loginTestEnv(t)
	_, stderr, err := runClient(t, "logout", "--server", "https://engram.example")
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	if !strings.Contains(stderr, "nothing to log out of") {
		t.Errorf("stderr = %q, want the nothing-to-log-out-of line", stderr)
	}
}

func TestLogoutIssuerUnreachableStillRemovesEntry(t *testing.T) {
	cachePath := loginTestEnv(t)
	c := &credentialCache{Servers: map[string]*cachedLogin{
		"https://engram.example": {Issuer: deadServer, ClientID: "dcr-client-1", Registered: true,
			RegistrationAccessToken: "rat", RegistrationClientURI: deadServer + "/clients/dcr-client-1",
			TokenURL: deadServer + "/token", RevocationURL: deadServer + "/revoke",
			Token: &oauth2.Token{AccessToken: "a", RefreshToken: "r"}},
	}}
	if err := c.save(cachePath); err != nil {
		t.Fatal(err)
	}

	_, stderr, err := runClient(t, "logout", "--server", "https://engram.example")
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	if !strings.Contains(stderr, "could not revoke") || !strings.Contains(stderr, "could not delete") {
		t.Errorf("stderr = %q, want a warning for each failed step", stderr)
	}
	if got := mustLoadCache(t, cachePath); len(got.Servers) != 0 {
		t.Errorf("cache after logout = %+v, want empty", got)
	}
}
