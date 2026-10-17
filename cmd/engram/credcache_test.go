// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestCredentialsPathHonoursAbsoluteXDGConfigHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	got, err := credentialsPath()
	if err != nil {
		t.Fatalf("credentialsPath: %v", err)
	}
	if want := filepath.Join(dir, "engram", "credentials.json"); got != want {
		t.Errorf("credentialsPath() = %q, want %q", got, want)
	}
}

func TestCredentialsPathIgnoresRelativeXDGConfigHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "relative/dir")
	got, err := credentialsPath()
	if err != nil {
		t.Fatalf("credentialsPath: %v", err)
	}
	if want := filepath.Join(home, ".config", "engram", "credentials.json"); got != want {
		t.Errorf("credentialsPath() = %q, want %q", got, want)
	}
}

func TestCacheMissingFileIsEmpty(t *testing.T) {
	c, err := loadCredentialCache(filepath.Join(t.TempDir(), "nope", "credentials.json"))
	if err != nil {
		t.Fatalf("loadCredentialCache: %v", err)
	}
	if c.DefaultServer != "" || len(c.Servers) != 0 {
		t.Errorf("missing file loaded as %+v, want an empty cache", c)
	}
}

func TestCacheRoundTripTwoServers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "engram", "credentials.json")
	expiry := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	c := &credentialCache{
		DefaultServer: "https://b.example",
		Servers: map[string]*cachedLogin{
			"https://a.example": {Issuer: "https://idp/a", ClientID: "ca", Registered: true, TokenURL: "https://idp/a/token",
				Token: &oauth2.Token{AccessToken: "at-a", RefreshToken: "rt-a", Expiry: expiry}},
			"https://b.example": {Issuer: "https://idp/b", ClientID: "cb", TokenURL: "https://idp/b/token",
				Token: &oauth2.Token{AccessToken: "at-b"}},
		},
	}
	if err := c.save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := loadCredentialCache(path)
	if err != nil {
		t.Fatalf("loadCredentialCache: %v", err)
	}
	if got.DefaultServer != "https://b.example" {
		t.Errorf("DefaultServer = %q, want https://b.example", got.DefaultServer)
	}
	a := got.Servers["https://a.example"]
	if a == nil || a.ClientID != "ca" || !a.Registered || a.Token.RefreshToken != "rt-a" || !a.Token.Expiry.Equal(expiry) {
		t.Errorf("server a round-tripped as %+v", a)
	}
	if b := got.Servers["https://b.example"]; b == nil || b.Token.AccessToken != "at-b" || b.Registered {
		t.Errorf("server b round-tripped as %+v", b)
	}
}

func TestCacheSaveIsOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	path := filepath.Join(t.TempDir(), "engram", "credentials.json")
	if err := (&credentialCache{Servers: map[string]*cachedLogin{}}).save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("cache file mode = %04o, want 0600", perm)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("cache dir mode = %04o, want 0700", perm)
	}
}

func TestCacheRefusesFileOthersCanRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	path := filepath.Join(t.TempDir(), "credentials.json")
	if err := os.WriteFile(path, []byte(`{"servers":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadCredentialCache(path)
	if got := exitCodeFromError(err); got != exitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", got, exitUsage, err)
	}
	if msg := err.Error(); !strings.Contains(msg, path) || !strings.Contains(msg, "chmod 600") {
		t.Errorf("error %q does not name the file and the chmod fix", msg)
	}
}

func TestCacheSaveReplacesAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "engram", "credentials.json")
	first := &credentialCache{DefaultServer: "https://a.example", Servers: map[string]*cachedLogin{}}
	if err := first.save(path); err != nil {
		t.Fatalf("first save: %v", err)
	}
	second := &credentialCache{DefaultServer: "https://b.example", Servers: map[string]*cachedLogin{}}
	if err := second.save(path); err != nil {
		t.Fatalf("second save: %v", err)
	}
	got, err := loadCredentialCache(path)
	if err != nil {
		t.Fatalf("loadCredentialCache: %v", err)
	}
	if got.DefaultServer != "https://b.example" {
		t.Errorf("DefaultServer = %q after replace, want https://b.example", got.DefaultServer)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("cache dir holds %v, want only credentials.json (no temporary file left behind)", names)
	}
}

func TestCacheKeyTrimsTrailingSlash(t *testing.T) {
	if got := cacheKey("https://engram.example/"); got != "https://engram.example" {
		t.Errorf("cacheKey = %q, want https://engram.example", got)
	}
}
