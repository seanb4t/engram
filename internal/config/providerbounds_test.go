// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import "testing"

// providerBound is one of the six 07-bounded-provider-responses knobs (D-04,
// D-05, D-08): the four per-client drain bounds and the two per-client
// request-timeout ceilings.
type providerBound struct {
	key string
	env string
	def string
	get func(*Config) string
}

// providerBoundFields is the six-key table both TestProviderBoundRegistryEntries
// and TestValidateProviderBounds drive, so no key is asserted by a
// hand-written one-off block.
var providerBoundFields = []providerBound{
	{"embed.drain_bytes", "ENGRAM_EMBED_DRAIN_BYTES", "262144", func(c *Config) string { return c.Embed.DrainBytes }},
	{"embed.drain_timeout", "ENGRAM_EMBED_DRAIN_TIMEOUT", "2s", func(c *Config) string { return c.Embed.DrainTimeout }},
	{"embed.max_timeout", "ENGRAM_EMBED_MAX_TIMEOUT", "10m", func(c *Config) string { return c.Embed.MaxTimeout }},
	{"summarize.drain_bytes", "ENGRAM_SUMMARY_DRAIN_BYTES", "262144", func(c *Config) string { return c.Summarize.DrainBytes }},
	{"summarize.drain_timeout", "ENGRAM_SUMMARY_DRAIN_TIMEOUT", "2s", func(c *Config) string { return c.Summarize.DrainTimeout }},
	{"summarize.max_timeout", "ENGRAM_SUMMARY_MAX_TIMEOUT", "10m", func(c *Config) string { return c.Summarize.MaxTimeout }},
}

// TestProviderBoundRegistryEntries pins D-04/D-05/D-08
// (07-bounded-provider-responses): the registry carries exactly one row per
// key for all six drain-bound/timeout-ceiling knobs, each with an empty
// Legacy (brand-new — nothing retired to guard against) and an empty Flag (a
// provider-tuning value, never typed at a prompt) and the expected Default;
// each field reaches its Config struct field from the environment through
// Load(nil); and each reads back as its registered Default when unset.
// Mirrors mcp_resource_test.go's three-subtest shape, extended to assert
// Default too since these six keys have one.
func TestProviderBoundRegistryEntries(t *testing.T) {
	t.Run("registry entry", func(t *testing.T) {
		for _, b := range providerBoundFields {
			var matches int
			for _, f := range registry {
				if f.Key != b.key {
					continue
				}
				matches++
				if f.Legacy != "" {
					t.Errorf("%s has Legacy=%q, want empty (brand-new var)", b.key, f.Legacy)
				}
				if f.Flag != "" {
					t.Errorf("%s has Flag=%q, want empty (provider-tuning value, never typed at a prompt)", b.key, f.Flag)
				}
				if f.Default != b.def {
					t.Errorf("%s has Default=%q, want %q", b.key, f.Default, b.def)
				}
			}
			if matches != 1 {
				t.Errorf("registry has %d entries for %s, want exactly 1", matches, b.key)
			}
		}
	})

	t.Run("from env", func(t *testing.T) {
		t.Setenv("ENGRAM_QDRANT_ADDR", "localhost:6334")
		for _, b := range providerBoundFields {
			t.Setenv(b.env, "999")
		}
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		for _, b := range providerBoundFields {
			if got := b.get(cfg); got != "999" {
				t.Errorf("%s = %q, want %q", b.key, got, "999")
			}
		}
	})

	t.Run("unset", func(t *testing.T) {
		t.Setenv("ENGRAM_QDRANT_ADDR", "localhost:6334")
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		for _, b := range providerBoundFields {
			if got := b.get(cfg); got != b.def {
				t.Errorf("%s = %q, want default %q", b.key, got, b.def)
			}
		}
	})
}
