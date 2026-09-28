// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file pins D-01/D-01a/D-16 (milestone 2026-09-25.01 Phase 6): the
// three ENGRAM_SEARCH_UNDERSTANDING* registry rows and Config.Validate's
// gating of them — the enum, the jev-requires-a-provider rule, the timeout
// validated only while understanding is effectively on, and the audit
// boolean validated unconditionally.

package config

import (
	"fmt"
	"strings"
	"testing"
)

// understandingFields is the three-key table TestUnderstandingRegistryEntries's
// subtests drive, mirroring searchFields' shape (search_config_test.go).
var understandingFields = []searchField{
	{"search.understanding", "ENGRAM_SEARCH_UNDERSTANDING", "", func(c *Config) string { return c.Search.Understanding }},
	{"search.understanding_timeout", "ENGRAM_SEARCH_UNDERSTANDING_TIMEOUT", "2s", func(c *Config) string { return c.Search.UnderstandingTimeout }},
	{"search.understanding_audit", "ENGRAM_SEARCH_UNDERSTANDING_AUDIT", "false", func(c *Config) string { return c.Search.UnderstandingAudit }},
}

// TestUnderstandingRegistryEntries mirrors TestSearchRegistryEntries: exactly
// one registry row per search.understanding* key, each with an empty Legacy
// (brand-new) and empty Flag (deployment-topology value), the expected
// Default; each field reaches its Config struct field from the environment
// through Load(nil); and each reads back as its registered Default when
// explicitly set to empty in the environment.
func TestUnderstandingRegistryEntries(t *testing.T) {
	t.Run("registry entry", func(t *testing.T) {
		for _, f := range understandingFields {
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
					t.Errorf("%s has Flag=%q, want empty (deployment-topology value, never typed at a prompt)", f.key, r.Flag)
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
		t.Setenv("ENGRAM_SEARCH_UNDERSTANDING", "jev")
		t.Setenv("ENGRAM_SEARCH_UNDERSTANDING_TIMEOUT", "999ms")
		t.Setenv("ENGRAM_SEARCH_UNDERSTANDING_AUDIT", "true")
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got := cfg.Search.Understanding; got != "jev" {
			t.Errorf("Search.Understanding = %q, want %q", got, "jev")
		}
		if got := cfg.Search.UnderstandingTimeout; got != "999ms" {
			t.Errorf("Search.UnderstandingTimeout = %q, want %q", got, "999ms")
		}
		if got := cfg.Search.UnderstandingAudit; got != "true" {
			t.Errorf("Search.UnderstandingAudit = %q, want %q", got, "true")
		}
	})

	t.Run("unset", func(t *testing.T) {
		t.Setenv("ENGRAM_QDRANT_ADDR", "localhost:6334")
		for _, f := range understandingFields {
			t.Setenv(f.env, "")
		}
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		for _, f := range understandingFields {
			if got := f.get(cfg); got != f.def {
				t.Errorf("%s = %q, want default %q", f.key, got, f.def)
			}
		}
	})
}

// TestUnderstandingConfigValidate pins D-01/D-01a/D-16: the understanding
// enum is checked unconditionally with exact-literal matching (no case
// folding or trimming); "jev" without a provider is rejected naming both
// variables; the timeout is validated only while understanding is
// effectively on (explicit "jev", or unset with Decisions.Provider "jev");
// the audit flag is validated as a boolean unconditionally, regardless of
// whether understanding is on. validConfig()/decisionsJevEnabled() already
// carry UnderstandingTimeout/UnderstandingAudit at passing defaults (Task
// 2's literal update), so each case below only overrides the field(s) under
// test.
func TestUnderstandingConfigValidate(t *testing.T) {
	t.Run("valid combinations", func(t *testing.T) {
		cases := []struct {
			name          string
			provider      string
			understanding string
		}{
			{"no provider, understanding unset", "", ""},
			{"no provider, understanding off", "", "off"},
			{"provider jev, understanding unset", "jev", ""},
			{"provider jev, understanding off", "jev", "off"},
			{"provider jev, understanding jev", "jev", "jev"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				var c *Config
				if tc.provider == "jev" {
					c = decisionsJevEnabled()
				} else {
					c = validConfig()
				}
				c.Search.Understanding = tc.understanding
				if err := c.Validate(); err != nil {
					t.Errorf("Validate() = %v, want nil", err)
				}
			})
		}
	})

	t.Run("bad understanding values rejected naming the field", func(t *testing.T) {
		for _, v := range []string{"JEV", "Jev", "on", "true", " off", "jev ", "off\t"} {
			t.Run(fmt.Sprintf("%q", v), func(t *testing.T) {
				c := decisionsJevEnabled()
				c.Search.Understanding = v
				err := c.Validate()
				if err == nil {
					t.Fatal("Validate() = nil, want error naming ENGRAM_SEARCH_UNDERSTANDING")
				}
				if !strings.Contains(err.Error(), "ENGRAM_SEARCH_UNDERSTANDING") {
					t.Errorf("Validate() error = %q, want substring ENGRAM_SEARCH_UNDERSTANDING", err)
				}
			})
		}
	})

	t.Run("jev without provider fails naming both vars", func(t *testing.T) {
		c := validConfig()
		c.Search.Understanding = "jev"
		err := c.Validate()
		if err == nil {
			t.Fatal("Validate() = nil, want error naming both ENGRAM_SEARCH_UNDERSTANDING and ENGRAM_DECISIONS_PROVIDER")
		}
		for _, want := range []string{"ENGRAM_SEARCH_UNDERSTANDING", "ENGRAM_DECISIONS_PROVIDER"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("Validate() error = %q, want substring %q", err, want)
			}
		}
	})

	badTimeouts := []string{"0", "-1s", "soon"}

	t.Run("bad timeout rejected while effectively on", func(t *testing.T) {
		for _, understanding := range []string{"", "jev"} {
			for _, tm := range badTimeouts {
				t.Run(fmt.Sprintf("understanding=%q timeout=%q", understanding, tm), func(t *testing.T) {
					c := decisionsJevEnabled()
					c.Search.Understanding = understanding
					c.Search.UnderstandingTimeout = tm
					err := c.Validate()
					if err == nil {
						t.Fatal("Validate() = nil, want error naming ENGRAM_SEARCH_UNDERSTANDING_TIMEOUT")
					}
					if !strings.Contains(err.Error(), "ENGRAM_SEARCH_UNDERSTANDING_TIMEOUT") {
						t.Errorf("Validate() error = %q, want substring ENGRAM_SEARCH_UNDERSTANDING_TIMEOUT", err)
					}
				})
			}
		}
	})

	t.Run("bad timeout accepted while off", func(t *testing.T) {
		for _, tm := range badTimeouts {
			t.Run("no provider, timeout="+tm, func(t *testing.T) {
				c := validConfig()
				c.Search.UnderstandingTimeout = tm
				if err := c.Validate(); err != nil {
					t.Errorf("Validate() = %v, want nil (no provider, understanding effectively off)", err)
				}
			})
			t.Run("provider jev, understanding off, timeout="+tm, func(t *testing.T) {
				c := decisionsJevEnabled()
				c.Search.Understanding = "off"
				c.Search.UnderstandingTimeout = tm
				if err := c.Validate(); err != nil {
					t.Errorf("Validate() = %v, want nil (understanding explicitly off)", err)
				}
			})
		}
	})

	t.Run("timeout 150ms with provider jev validates", func(t *testing.T) {
		c := decisionsJevEnabled()
		c.Search.Understanding = "jev"
		c.Search.UnderstandingTimeout = "150ms"
		if err := c.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})

	t.Run("bad audit rejected unconditionally", func(t *testing.T) {
		for _, audit := range []string{"yes please", ""} {
			t.Run(fmt.Sprintf("%q", audit), func(t *testing.T) {
				c := validConfig()
				c.Search.UnderstandingAudit = audit
				err := c.Validate()
				if err == nil {
					t.Fatal("Validate() = nil, want error naming ENGRAM_SEARCH_UNDERSTANDING_AUDIT")
				}
				if !strings.Contains(err.Error(), "ENGRAM_SEARCH_UNDERSTANDING_AUDIT") {
					t.Errorf("Validate() error = %q, want substring ENGRAM_SEARCH_UNDERSTANDING_AUDIT", err)
				}
			})
		}
	})
}
