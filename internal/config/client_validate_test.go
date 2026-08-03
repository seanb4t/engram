// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"strings"
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

// validClientConfig returns a ClientConfig whose fields all pass
// ValidateClient. Tests mutate one field to exercise a single rule.
// Deliberately a fresh helper local to this file (zero pre-existing call
// sites), so the hand-built-literal tax this package's Config-shaped test
// helper already carries stays confined to Config, never spreading to
// ClientConfig.
func validClientConfig() ClientConfig {
	return ClientConfig{
		Timeout:  "30s",
		Insecure: "false",
	}
}

// TestClientConfigValidateFieldRules locks in ValidateClient's rules,
// including D-05's central claim: 0 (in either duration spelling) is a
// rejected usage error, never treated as unbounded.
func TestClientConfigValidateFieldRules(t *testing.T) {
	if err := ValidateClient(validClientConfig()); err != nil {
		t.Fatalf("ValidateClient(valid) = %v, want nil", err)
	}

	cases := []struct {
		name   string
		mutate func(*ClientConfig)
		want   string
	}{
		{"timeout zero", func(c *ClientConfig) { c.Timeout = "0" }, "ENGRAM_TIMEOUT"},
		{"timeout zero (0s spelling)", func(c *ClientConfig) { c.Timeout = "0s" }, "ENGRAM_TIMEOUT"},
		{"timeout negative", func(c *ClientConfig) { c.Timeout = "-1s" }, "ENGRAM_TIMEOUT"},
		{"timeout not a duration", func(c *ClientConfig) { c.Timeout = "abc" }, "ENGRAM_TIMEOUT"},
		{"timeout empty", func(c *ClientConfig) { c.Timeout = "" }, "ENGRAM_TIMEOUT"},
		{"insecure non-bool", func(c *ClientConfig) { c.Insecure = "maybe" }, "--insecure"},
		{"output bogus", func(c *ClientConfig) { c.Output = "bogus" }, "--output"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validClientConfig()
			tc.mutate(&c)
			err := ValidateClient(c)
			if err == nil {
				t.Fatalf("ValidateClient() = nil, want error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("ValidateClient() error = %q, want substring %q", err, tc.want)
			}
		})
	}

	t.Run("insecure true and false both valid", func(t *testing.T) {
		for _, v := range []string{"true", "false"} {
			c := validClientConfig()
			c.Insecure = v
			if err := ValidateClient(c); err != nil {
				t.Errorf("ValidateClient(Insecure=%q) = %v, want nil", v, err)
			}
		}
	})

	t.Run("output json, text, and empty all valid", func(t *testing.T) {
		for _, v := range []string{"json", "text", ""} {
			c := validClientConfig()
			c.Output = v
			if err := ValidateClient(c); err != nil {
				t.Errorf("ValidateClient(Output=%q) = %v, want nil", v, err)
			}
		}
	})

	t.Run("multiple bad fields aggregate into one error", func(t *testing.T) {
		c := ClientConfig{Timeout: "0", Insecure: "maybe", Output: "bogus"}
		err := ValidateClient(c)
		if err == nil {
			t.Fatal("ValidateClient() = nil, want aggregated error")
		}
		for _, want := range []string{"ENGRAM_TIMEOUT", "--insecure", "--output"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("ValidateClient() error = %q, want substring %q", err, want)
			}
		}
	})
}
