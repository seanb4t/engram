// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"strconv"
	"strings"
	"testing"
)

// decisionsField is one of the ten ENGRAM_DECISIONS_* registry rows (D-01,
// D-02, D-08).
type decisionsField struct {
	key string
	env string
	def string
	get func(*Config) string
}

// decisionsFields is the ten-key table both TestDecisionsRegistryEntries
// subtests drive, so no key is asserted by a hand-written one-off block.
var decisionsFields = []decisionsField{
	{"decisions.provider", "ENGRAM_DECISIONS_PROVIDER", "", func(c *Config) string { return c.Decisions.Provider }},
	{"decisions.base_url", "ENGRAM_DECISIONS_BASE_URL", "", func(c *Config) string { return c.Decisions.BaseURL }},
	{"decisions.api_key", "ENGRAM_DECISIONS_API_KEY", "", func(c *Config) string { return c.Decisions.APIKey }},
	{"decisions.model", "ENGRAM_DECISIONS_MODEL", "typesafe/jev-1.13", func(c *Config) string { return c.Decisions.Model }},
	{"decisions.timeout", "ENGRAM_DECISIONS_TIMEOUT", "10s", func(c *Config) string { return c.Decisions.Timeout }},
	{"decisions.max_timeout", "ENGRAM_DECISIONS_MAX_TIMEOUT", "10m", func(c *Config) string { return c.Decisions.MaxTimeout }},
	{"decisions.drain_bytes", "ENGRAM_DECISIONS_DRAIN_BYTES", "262144", func(c *Config) string { return c.Decisions.DrainBytes }},
	{"decisions.drain_timeout", "ENGRAM_DECISIONS_DRAIN_TIMEOUT", "2s", func(c *Config) string { return c.Decisions.DrainTimeout }},
	{"decisions.concurrency", "ENGRAM_DECISIONS_CONCURRENCY", "4", func(c *Config) string { return c.Decisions.Concurrency }},
	{"decisions.verdict_threshold", "ENGRAM_DECISIONS_VERDICT_THRESHOLD", "0.9", func(c *Config) string { return c.Decisions.VerdictThreshold }},
}

// TestDecisionsRegistryEntries pins D-01/D-02: the registry carries exactly
// one row per decisions.* key, each with an empty Legacy (brand-new — nothing
// retired to guard against), an empty Flag (a provider-tuning value, never
// typed at a prompt) and the expected Default; each field reaches its Config
// struct field from the environment through Load(nil); and each reads back
// as its registered Default when explicitly set to empty in the environment.
// Mirrors providerbounds_test.go's three-subtest shape.
func TestDecisionsRegistryEntries(t *testing.T) {
	t.Run("registry entry", func(t *testing.T) {
		for _, f := range decisionsFields {
			var matches int
			for _, r := range registry {
				if r.Key != f.key {
					continue
				}
				matches++
				if r.Legacy != "" {
					t.Errorf("%s has Legacy=%q, want empty (brand-new var)", f.key, r.Legacy)
				}
				if r.Flag != "" {
					t.Errorf("%s has Flag=%q, want empty (provider-tuning value, never typed at a prompt)", f.key, r.Flag)
				}
				if r.Default != f.def {
					t.Errorf("%s has Default=%q, want %q", f.key, r.Default, f.def)
				}
			}
			if matches != 1 {
				t.Errorf("registry has %d entries for %s, want exactly 1", matches, f.key)
			}
		}
	})

	t.Run("from env", func(t *testing.T) {
		t.Setenv("ENGRAM_QDRANT_ADDR", "localhost:6334")
		for _, f := range decisionsFields {
			t.Setenv(f.env, "999")
		}
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		for _, f := range decisionsFields {
			if got := f.get(cfg); got != "999" {
				t.Errorf("%s = %q, want %q", f.key, got, "999")
			}
		}
	})

	t.Run("unset", func(t *testing.T) {
		t.Setenv("ENGRAM_QDRANT_ADDR", "localhost:6334")
		for _, f := range decisionsFields {
			t.Setenv(f.env, "")
		}
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		for _, f := range decisionsFields {
			if got := f.get(cfg); got != f.def {
				t.Errorf("%s = %q, want default %q", f.key, got, f.def)
			}
		}
	})
}

// decisionsJevEnabled returns a valid Config with Decisions.Provider "jev"
// and every other decisions field set to a passing value, so a test can
// mutate a single field to exercise one Validate rule.
func decisionsJevEnabled() *Config {
	c := validConfig()
	c.Decisions = DecisionsConfig{
		Provider:         "jev",
		BaseURL:          "https://openrouter.ai/api",
		Model:            "typesafe/jev-1.13",
		Timeout:          "10s",
		MaxTimeout:       "10m",
		DrainBytes:       "262144",
		DrainTimeout:     "2s",
		Concurrency:      "4",
		VerdictThreshold: "0.9",
	}
	return c
}

// TestDecisionsValidate pins D-01/D-03: the provider enum is checked
// unconditionally; every other decisions field is checked only when the
// provider is "jev" (E01, the inert-when-off row); the base URL never falls
// back to ENGRAM_OPENAI_BASE_URL when empty (E05, D-03 as corrected); and the
// remaining fields follow the same zero-valid/negative-rejected or
// always-positive conventions embed.*/summarize.* already use.
func TestDecisionsValidate(t *testing.T) {
	t.Run("provider empty is inert even with every other field malformed (E01)", func(t *testing.T) {
		c := validConfig()
		c.Decisions = DecisionsConfig{
			Provider:         "",
			BaseURL:          "ftp://",
			Timeout:          "x",
			MaxTimeout:       "0",
			DrainBytes:       "-1",
			Concurrency:      "0",
			VerdictThreshold: "garbage",
		}
		if err := c.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil (provider empty disables the whole decisions block)", err)
		}
	})

	t.Run("base URL does not fall back to ENGRAM_OPENAI_BASE_URL (E05)", func(t *testing.T) {
		c := decisionsJevEnabled()
		c.Decisions.BaseURL = ""
		if c.OpenAI.BaseURL == "" {
			t.Fatal("test fixture bug: validConfig() must set a valid OpenAI.BaseURL")
		}
		err := c.Validate()
		if err == nil {
			t.Fatal("Validate() = nil, want an error naming ENGRAM_DECISIONS_BASE_URL")
		}
		for _, want := range []string{"ENGRAM_DECISIONS_BASE_URL is empty", "does not fall back to ENGRAM_OPENAI_BASE_URL"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("Validate() error = %q, want substring %q", err, want)
			}
		}
	})

	cases := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
		want    string
	}{
		{"provider bogus", func(c *Config) { c.Decisions.Provider = "bogus" }, true, "ENGRAM_DECISIONS_PROVIDER"},
		{"base scheme not http(s)", func(c *Config) { c.Decisions.BaseURL = "ftp://x" }, true, "ENGRAM_DECISIONS_BASE_URL"},
		{"base missing host", func(c *Config) { c.Decisions.BaseURL = "https://" }, true, "ENGRAM_DECISIONS_BASE_URL"},
		{"base valid", func(c *Config) { c.Decisions.BaseURL = "https://openrouter.ai/api" }, false, ""},
		{"timeout zero accepted", func(c *Config) { c.Decisions.Timeout = "0" }, false, ""},
		{"timeout negative rejected", func(c *Config) { c.Decisions.Timeout = "-1s" }, true, "ENGRAM_DECISIONS_TIMEOUT"},
		{"max_timeout zero rejected", func(c *Config) { c.Decisions.MaxTimeout = "0" }, true, "ENGRAM_DECISIONS_MAX_TIMEOUT"},
		{"max_timeout negative rejected", func(c *Config) { c.Decisions.MaxTimeout = "-1s" }, true, "ENGRAM_DECISIONS_MAX_TIMEOUT"},
		{"drain_bytes zero accepted", func(c *Config) { c.Decisions.DrainBytes = "0" }, false, ""},
		{"drain_bytes negative rejected", func(c *Config) { c.Decisions.DrainBytes = "-1" }, true, "ENGRAM_DECISIONS_DRAIN_BYTES"},
		{"drain_timeout zero accepted", func(c *Config) { c.Decisions.DrainTimeout = "0" }, false, ""},
		{"drain_timeout unparseable rejected", func(c *Config) { c.Decisions.DrainTimeout = "nope" }, true, "ENGRAM_DECISIONS_DRAIN_TIMEOUT"},
		{"concurrency valid", func(c *Config) { c.Decisions.Concurrency = "4" }, false, ""},
		{"concurrency zero rejected", func(c *Config) { c.Decisions.Concurrency = "0" }, true, "ENGRAM_DECISIONS_CONCURRENCY"},
		{"concurrency negative rejected", func(c *Config) { c.Decisions.Concurrency = "-2" }, true, "ENGRAM_DECISIONS_CONCURRENCY"},
		{"concurrency non-numeric rejected", func(c *Config) { c.Decisions.Concurrency = "abc" }, true, "ENGRAM_DECISIONS_CONCURRENCY"},
		{"model empty rejected", func(c *Config) { c.Decisions.Model = "" }, true, "ENGRAM_DECISIONS_MODEL"},
		{"api_key empty accepted (never validated)", func(c *Config) { c.Decisions.APIKey = "" }, false, ""},
		{"verdict_threshold above 1 rejected", func(c *Config) { c.Decisions.VerdictThreshold = "1.5" }, true, "ENGRAM_DECISIONS_VERDICT_THRESHOLD"},
		{"verdict_threshold NaN rejected", func(c *Config) { c.Decisions.VerdictThreshold = "NaN" }, true, "ENGRAM_DECISIONS_VERDICT_THRESHOLD"},
		{"verdict_threshold zero accepted", func(c *Config) { c.Decisions.VerdictThreshold = "0" }, false, ""},
		{"verdict_threshold one accepted", func(c *Config) { c.Decisions.VerdictThreshold = "1" }, false, ""},
		{"valid control, no mutation", func(*Config) {}, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := decisionsJevEnabled()
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

// TestParseProbability pins D-08/CUR-02's boundary/precision contract:
// ParseProbability accepts every value in [0, 1] and returns it unrounded
// (the exact float64 strconv.ParseFloat gives), and rejects anything outside
// that range, NaN, infinities, and unparseable input.
func TestParseProbability(t *testing.T) {
	accept := []string{"0", "1", "0.9", "0.95", "1.0"}
	for _, v := range accept {
		t.Run("accepts "+v, func(t *testing.T) {
			got, err := ParseProbability(v)
			if err != nil {
				t.Fatalf("ParseProbability(%q) = %v, want nil error", v, err)
			}
			want, werr := strconv.ParseFloat(v, 64)
			if werr != nil {
				t.Fatalf("test fixture bug: strconv.ParseFloat(%q): %v", v, werr)
			}
			if got != want {
				t.Errorf("ParseProbability(%q) = %v, want %v (unrounded)", v, got, want)
			}
		})
	}

	reject := []string{"-0.01", "1.01", "NaN", "nan", "Inf", "+Inf", "-Inf", "abc", ""}
	for _, v := range reject {
		t.Run("rejects "+v, func(t *testing.T) {
			if _, err := ParseProbability(v); err == nil {
				t.Errorf("ParseProbability(%q) = nil error, want non-nil", v)
			}
		})
	}
}
