// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"strings"
	"testing"
)

// searchField is one of the three ENGRAM_SEARCH_* registry rows (D-01, D-09, #618).
type searchField struct {
	key string
	env string
	def string
	get func(*Config) string
}

// searchFields is the three-key table TestSearchRegistryEntries's subtests
// drive, mirroring decisionsFields' shape.
var searchFields = []searchField{
	{"search.ranker", "ENGRAM_SEARCH_RANKER", "lexical", func(c *Config) string { return c.Search.Ranker }},
	{"search.rerank_timeout", "ENGRAM_SEARCH_RERANK_TIMEOUT", "2s", func(c *Config) string { return c.Search.RerankTimeout }},
	{"search.rerank_audit", "ENGRAM_SEARCH_RERANK_AUDIT", "false", func(c *Config) string { return c.Search.RerankAudit }},
}

// TestSearchRegistryEntries mirrors TestDecisionsRegistryEntries: exactly one
// registry row per search.* key, each with an empty Legacy (brand-new) and
// empty Flag (deployment-topology value), the expected Default; each field
// reaches its Config struct field from the environment through Load(nil);
// and each reads back as its registered Default when explicitly set to empty
// in the environment.
func TestSearchRegistryEntries(t *testing.T) {
	t.Run("registry entry", func(t *testing.T) {
		for _, f := range searchFields {
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
		t.Setenv("ENGRAM_SEARCH_RANKER", "jev")
		t.Setenv("ENGRAM_SEARCH_RERANK_TIMEOUT", "999ms")
		t.Setenv("ENGRAM_SEARCH_RERANK_AUDIT", "true")
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got := cfg.Search.Ranker; got != "jev" {
			t.Errorf("Search.Ranker = %q, want %q", got, "jev")
		}
		if got := cfg.Search.RerankTimeout; got != "999ms" {
			t.Errorf("Search.RerankTimeout = %q, want %q", got, "999ms")
		}
		if got := cfg.Search.RerankAudit; got != "true" {
			t.Errorf("Search.RerankAudit = %q, want %q", got, "true")
		}
	})

	t.Run("unset", func(t *testing.T) {
		t.Setenv("ENGRAM_QDRANT_ADDR", "localhost:6334")
		for _, f := range searchFields {
			t.Setenv(f.env, "")
		}
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		for _, f := range searchFields {
			if got := f.get(cfg); got != f.def {
				t.Errorf("%s = %q, want default %q", f.key, got, f.def)
			}
		}
	})
}

// TestSearchConfigValidate pins D-01/D-09: the ranker enum is checked
// unconditionally; ranker=jev requires Decisions.Provider to be set, naming
// both variables; the rerank timeout is checked only when the ranker is
// "jev", and must be a strictly positive Go duration.
func TestSearchConfigValidate(t *testing.T) {
	t.Run("empty and lexical ranker validate with no provider", func(t *testing.T) {
		for _, ranker := range []string{"", "lexical"} {
			c := validConfig()
			c.Search = SearchConfig{Ranker: ranker, RerankTimeout: "soon", RerankAudit: "false"}
			if err := c.Validate(); err != nil {
				t.Errorf("ranker=%q: Validate() = %v, want nil (rerank_timeout gated on ranker=jev)", ranker, err)
			}
		}
	})

	t.Run("jev with provider and base URL validates", func(t *testing.T) {
		c := decisionsJevEnabled()
		c.Search = SearchConfig{Ranker: "jev", RerankTimeout: "2s", RerankAudit: "true"}
		if err := c.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
	})

	t.Run("non-boolean rerank_audit fails even with the ranker off", func(t *testing.T) {
		c := validConfig()
		c.Search = SearchConfig{Ranker: "lexical", RerankTimeout: "2s", RerankAudit: "yes please"}
		err := c.Validate()
		if err == nil {
			t.Fatal("Validate() = nil, want error naming ENGRAM_SEARCH_RERANK_AUDIT")
		}
		if !strings.Contains(err.Error(), "ENGRAM_SEARCH_RERANK_AUDIT") {
			t.Errorf("Validate() error = %q, want substring ENGRAM_SEARCH_RERANK_AUDIT", err)
		}
	})

	badRankers := []string{"JEV", "vector", "jev "}
	for _, r := range badRankers {
		t.Run("bad ranker "+r, func(t *testing.T) {
			c := validConfig()
			c.Search = SearchConfig{Ranker: r, RerankAudit: "false"}
			err := c.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want error naming ENGRAM_SEARCH_RANKER")
			}
			if !strings.Contains(err.Error(), "ENGRAM_SEARCH_RANKER") {
				t.Errorf("Validate() error = %q, want substring ENGRAM_SEARCH_RANKER", err)
			}
		})
	}

	t.Run("jev ranker without provider fails naming both vars", func(t *testing.T) {
		c := validConfig()
		c.Search = SearchConfig{Ranker: "jev", RerankTimeout: "2s", RerankAudit: "false"}
		err := c.Validate()
		if err == nil {
			t.Fatal("Validate() = nil, want error naming both ENGRAM_SEARCH_RANKER and ENGRAM_DECISIONS_PROVIDER")
		}
		for _, want := range []string{"ENGRAM_SEARCH_RANKER", "ENGRAM_DECISIONS_PROVIDER"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("Validate() error = %q, want substring %q", err, want)
			}
		}
	})

	badTimeouts := []string{"0", "-1s", "soon"}
	for _, tm := range badTimeouts {
		t.Run("jev with bad rerank_timeout "+tm, func(t *testing.T) {
			c := decisionsJevEnabled()
			c.Search = SearchConfig{Ranker: "jev", RerankTimeout: tm, RerankAudit: "false"}
			err := c.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want error naming ENGRAM_SEARCH_RERANK_TIMEOUT")
			}
			if !strings.Contains(err.Error(), "ENGRAM_SEARCH_RERANK_TIMEOUT") {
				t.Errorf("Validate() error = %q, want substring ENGRAM_SEARCH_RERANK_TIMEOUT", err)
			}
		})
	}

	t.Run("jev with rerank_timeout 150ms validates", func(t *testing.T) {
		c := decisionsJevEnabled()
		c.Search = SearchConfig{Ranker: "jev", RerankTimeout: "150ms", RerankAudit: "false"}
		if err := c.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
	})
}
