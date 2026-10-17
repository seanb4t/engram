// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/oauth2"
)

// credentialCache is the `engram login` cache: one
// file per user, one entry per server, plus the server the last login
// targeted. It holds long-lived refresh tokens, so it is only ever written
// owner-only and read only when no other user can read it.
type credentialCache struct {
	DefaultServer string                  `json:"default_server,omitempty"`
	Servers       map[string]*cachedLogin `json:"servers"`
}

// cachedLogin is one server's login. TokenURL and RevocationURL are saved
// at login so a refresh or a logout needs no discovery round trip.
type cachedLogin struct {
	Issuer   string `json:"issuer"`
	ClientID string `json:"client_id"`
	// Registered is true when this CLI created ClientID through dynamic
	// client registration. Only a registered client is ever deleted by
	// logout; a pre-registered one belongs to the operator.
	Registered              bool          `json:"registered"`
	RegistrationAccessToken string        `json:"registration_access_token,omitempty"`
	RegistrationClientURI   string        `json:"registration_client_uri,omitempty"`
	TokenURL                string        `json:"token_url"`
	RevocationURL           string        `json:"revocation_url,omitempty"`
	Token                   *oauth2.Token `json:"token,omitempty"`
}

// cacheKey normalises a server URL into its cache key, so a trailing slash
// on --server does not split one server into two entries.
func cacheKey(serverURL string) string {
	return strings.TrimRight(serverURL, "/")
}

// credentialsPath resolves the cache file: $XDG_CONFIG_HOME when it is an
// absolute path, else ~/.config, on every platform — the same root rule
// internal/setup's opencodeConfigRoot follows, rather than
// os.UserConfigDir, which is ~/Library/Application Support on macOS.
func credentialsPath() (string, error) {
	root := os.Getenv("XDG_CONFIG_HOME")
	if root == "" || !filepath.IsAbs(root) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("locate the login cache: %w", err)
		}
		root = filepath.Join(home, ".config")
	}
	return filepath.Join(root, "engram", "credentials.json"), nil
}

// loadCredentialCache reads the cache at path. A missing file is an empty
// cache, not an error. On POSIX a file the group or others can read is
// refused (exitUsage) before its content is read, naming the fix.
func loadCredentialCache(path string) (*credentialCache, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &credentialCache{Servers: map[string]*cachedLogin{}}, nil
	}
	if err != nil {
		return nil, usageErrorf("read the login cache %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	if runtime.GOOS != "windows" {
		info, err := f.Stat()
		if err != nil {
			return nil, usageErrorf("read the login cache %s: %w", path, err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			return nil, usageErrorf(
				"the login cache %s can be read by other users (mode %04o); fix it with: chmod 600 %s",
				path, info.Mode().Perm(), path)
		}
	}

	var c credentialCache
	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return nil, usageErrorf("parse the login cache %s: %w (run engram logout or remove the file)", path, err)
	}
	if c.Servers == nil {
		c.Servers = map[string]*cachedLogin{}
	}
	return &c, nil
}

// save writes the cache to path atomically: a temporary file in the same
// directory, then rename, so a concurrent reader sees the old file or the
// new one, never a partial one. os.CreateTemp creates the file 0600.
func (c *credentialCache) save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encode the login cache: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".credentials-*.json")
	if err != nil {
		return fmt.Errorf("write the login cache: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write the login cache: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write the login cache: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write the login cache: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("write the login cache: %w", err)
	}
	return nil
}
