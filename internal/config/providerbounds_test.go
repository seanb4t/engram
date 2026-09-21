// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"strings"
	"testing"
)

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

// TestValidateProviderBounds pins D-04/D-05/D-08 (07-bounded-provider-responses):
// zero is accepted on all four drain bounds and rejected (alongside negative
// and unparseable) on both ceilings; negative is rejected on all four drain
// bounds; unparseable is rejected on both drain timeouts and both ceilings;
// and the three summarize bounds are ignored entirely when Summarize.Model is
// empty, enforced the moment it is set — the same gating summarize.timeout
// itself already has. Each case mutates exactly one field on a shared valid
// fixture (validConfig for the embed cases, summarizeEnabled for the
// summarize cases) and asserts on the resulting error.
func TestValidateProviderBounds(t *testing.T) {
	cases := []struct {
		name    string
		base    func() *Config
		mutate  func(*Config)
		wantErr bool
		want    string // substring expected in the error, when wantErr
	}{
		// --- embed.drain_bytes ---
		{"embed drain_bytes zero accepted", validConfig, func(c *Config) { c.Embed.DrainBytes = "0" }, false, ""},
		{"embed drain_bytes negative rejected", validConfig, func(c *Config) { c.Embed.DrainBytes = "-1" }, true, "ENGRAM_EMBED_DRAIN_BYTES"},

		// --- embed.drain_timeout ---
		{"embed drain_timeout zero accepted", validConfig, func(c *Config) { c.Embed.DrainTimeout = "0" }, false, ""},
		{"embed drain_timeout negative rejected", validConfig, func(c *Config) { c.Embed.DrainTimeout = "-1s" }, true, "ENGRAM_EMBED_DRAIN_TIMEOUT"},
		{"embed drain_timeout unparseable rejected", validConfig, func(c *Config) { c.Embed.DrainTimeout = "not-a-duration" }, true, "ENGRAM_EMBED_DRAIN_TIMEOUT"},

		// --- embed.max_timeout (always rejects non-positive) ---
		{"embed max_timeout zero rejected", validConfig, func(c *Config) { c.Embed.MaxTimeout = "0" }, true, "ENGRAM_EMBED_MAX_TIMEOUT"},
		{"embed max_timeout negative rejected", validConfig, func(c *Config) { c.Embed.MaxTimeout = "-1s" }, true, "ENGRAM_EMBED_MAX_TIMEOUT"},
		{"embed max_timeout unparseable rejected", validConfig, func(c *Config) { c.Embed.MaxTimeout = "not-a-duration" }, true, "ENGRAM_EMBED_MAX_TIMEOUT"},

		// --- summarize.drain_bytes (Model set: gated block active) ---
		{"summarize drain_bytes zero accepted", summarizeEnabled, func(c *Config) { c.Summarize.DrainBytes = "0" }, false, ""},
		{"summarize drain_bytes negative rejected", summarizeEnabled, func(c *Config) { c.Summarize.DrainBytes = "-1" }, true, "ENGRAM_SUMMARY_DRAIN_BYTES"},

		// --- summarize.drain_timeout ---
		{"summarize drain_timeout zero accepted", summarizeEnabled, func(c *Config) { c.Summarize.DrainTimeout = "0" }, false, ""},
		{"summarize drain_timeout negative rejected", summarizeEnabled, func(c *Config) { c.Summarize.DrainTimeout = "-1s" }, true, "ENGRAM_SUMMARY_DRAIN_TIMEOUT"},
		{"summarize drain_timeout unparseable rejected", summarizeEnabled, func(c *Config) { c.Summarize.DrainTimeout = "not-a-duration" }, true, "ENGRAM_SUMMARY_DRAIN_TIMEOUT"},

		// --- summarize.max_timeout (always rejects non-positive, when Model set) ---
		{"summarize max_timeout zero rejected", summarizeEnabled, func(c *Config) { c.Summarize.MaxTimeout = "0" }, true, "ENGRAM_SUMMARY_MAX_TIMEOUT"},
		{"summarize max_timeout negative rejected", summarizeEnabled, func(c *Config) { c.Summarize.MaxTimeout = "-1s" }, true, "ENGRAM_SUMMARY_MAX_TIMEOUT"},
		{"summarize max_timeout unparseable rejected", summarizeEnabled, func(c *Config) { c.Summarize.MaxTimeout = "not-a-duration" }, true, "ENGRAM_SUMMARY_MAX_TIMEOUT"},

		// --- gating: summarize.timeout's own Summarize.Model gate applies identically here ---
		{"summarize bounds ignored when model empty", validConfig, func(c *Config) {
			c.Summarize.Model = "" // validConfig already leaves this empty; explicit for clarity
			c.Summarize.DrainBytes = "-1"
			c.Summarize.DrainTimeout = "not-a-duration"
			c.Summarize.MaxTimeout = "0"
		}, false, ""},
		{"summarize bounds enforced when model set (control)", summarizeEnabled, func(*Config) {}, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.base()
			tc.mutate(c)
			err := c.Validate()
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Validate() error = %q, want substring %q", err, tc.want)
			}
		})
	}
}
