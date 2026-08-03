// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"testing"

	flag "github.com/spf13/pflag"
)

// TestClientConfigLoadPrecedence locks in the client.* registry rows'
// wiring into cfg.Client: registry defaults, the ENGRAM_ env layer (including
// the empty-env-preserves-default guard), and the deliberate absence of an
// ENGRAM_TOKEN row (D-13 — the credential never routes through koanf).
func TestClientConfigLoadPrecedence(t *testing.T) {
	t.Run("defaults with no env set", func(t *testing.T) {
		t.Setenv("ENGRAM_SERVER_URL", "")
		t.Setenv("ENGRAM_TIMEOUT", "")
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Client.Timeout != "30s" {
			t.Errorf("Client.Timeout = %q, want 30s default", cfg.Client.Timeout)
		}
		if cfg.Client.Insecure != "false" {
			t.Errorf("Client.Insecure = %q, want false default", cfg.Client.Insecure)
		}
	})

	t.Run("ENGRAM_SERVER_URL sets ServerURL", func(t *testing.T) {
		t.Setenv("ENGRAM_SERVER_URL", "https://x")
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Client.ServerURL != "https://x" {
			t.Errorf("Client.ServerURL = %q, want https://x", cfg.Client.ServerURL)
		}
	})

	t.Run("ENGRAM_TIMEOUT sets Timeout", func(t *testing.T) {
		t.Setenv("ENGRAM_TIMEOUT", "45s")
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Client.Timeout != "45s" {
			t.Errorf("Client.Timeout = %q, want 45s", cfg.Client.Timeout)
		}
	})

	t.Run("empty ENGRAM_TIMEOUT preserves default", func(t *testing.T) {
		t.Setenv("ENGRAM_TIMEOUT", "")
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Client.Timeout != "30s" {
			t.Errorf("Client.Timeout = %q, want 30s (empty env preserves default)", cfg.Client.Timeout)
		}
	})

	t.Run("no ENGRAM_TOKEN registry row", func(t *testing.T) {
		for _, f := range registry {
			if f.Env == "ENGRAM_TOKEN" {
				t.Fatalf("registry has an ENGRAM_TOKEN row (key %q) — the credential must never reach koanf (D-13)", f.Key)
			}
		}
	})
}

// TestLoadOverlayBindsNonStringFlag locks in that Load's changed-flag overlay
// reads a flag of any pflag type via Value.String(), not GetString — D-04
// puts a bool (--insecure) into the registry, so the overlay must not error
// on a non-string flag.
func TestLoadOverlayBindsNonStringFlag(t *testing.T) {
	t.Run("bool flag supplied", func(t *testing.T) {
		f := flag.NewFlagSet("client", flag.ContinueOnError)
		f.Bool("insecure", false, "")
		if err := f.Parse([]string{"--insecure"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		cfg, err := Load(f)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Client.Insecure != "true" {
			t.Errorf("Client.Insecure = %q, want true", cfg.Client.Insecure)
		}
	})

	t.Run("bool flag not supplied", func(t *testing.T) {
		f := flag.NewFlagSet("client", flag.ContinueOnError)
		f.Bool("insecure", false, "")
		if err := f.Parse([]string{}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		cfg, err := Load(f)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Client.Insecure != "false" {
			t.Errorf("Client.Insecure = %q, want false default", cfg.Client.Insecure)
		}
	})

	t.Run("string flag still overlays", func(t *testing.T) {
		f := flag.NewFlagSet("client", flag.ContinueOnError)
		f.String("server", "", "")
		if err := f.Parse([]string{"--server", "https://x"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		cfg, err := Load(f)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Client.ServerURL != "https://x" {
			t.Errorf("Client.ServerURL = %q, want https://x", cfg.Client.ServerURL)
		}
	})

	t.Run("existing string-flag row still overlays (regression)", func(t *testing.T) {
		t.Setenv("ENGRAM_LISTEN_ADDR", "")
		f := flag.NewFlagSet("serve", flag.ContinueOnError)
		f.String("listen-addr", "", "")
		if err := f.Parse([]string{"--listen-addr", ":9090"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		cfg, err := Load(f)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Server.ListenAddr != ":9090" {
			t.Errorf("Server.ListenAddr = %q, want :9090", cfg.Server.ListenAddr)
		}
	})
}
