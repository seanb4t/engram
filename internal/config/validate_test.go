// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"strings"
	"testing"
)

// validConfig returns a Config whose data-plane fields all pass Validate.
// Tests mutate one field to exercise a single rule.
func validConfig() *Config {
	return &Config{
		Qdrant: QdrantConfig{Addr: "localhost:6334", Collection: "mem_eval"},
		Embed:  EmbedConfig{Model: "ollama/bge-m3", Dim: "1024"},
		OpenAI: OpenAIConfig{BaseURL: "http://localhost:4000"},
	}
}

func TestValidateHappyPath(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("Validate(valid) = %v, want nil", err)
	}
}

func TestValidateFieldRules(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string // substring expected in the error
	}{
		{"qdrant addr empty", func(c *Config) { c.Qdrant.Addr = "" }, "ENGRAM_QDRANT_ADDR"},
		{"qdrant addr no port", func(c *Config) { c.Qdrant.Addr = "localhost" }, "ENGRAM_QDRANT_ADDR"},
		{"qdrant addr non-numeric port", func(c *Config) { c.Qdrant.Addr = "localhost:nope" }, "ENGRAM_QDRANT_ADDR"},
		{"qdrant addr port out of range", func(c *Config) { c.Qdrant.Addr = "localhost:70000" }, "out of range"},
		{"qdrant collection empty", func(c *Config) { c.Qdrant.Collection = "" }, "ENGRAM_QDRANT_COLLECTION"},
		{"embed model empty", func(c *Config) { c.Embed.Model = "" }, "ENGRAM_EMBED_MODEL"},
		{"embed dim empty", func(c *Config) { c.Embed.Dim = "" }, "ENGRAM_EMBED_DIM"},
		{"embed dim non-numeric", func(c *Config) { c.Embed.Dim = "abc" }, "ENGRAM_EMBED_DIM"},
		{"embed dim zero", func(c *Config) { c.Embed.Dim = "0" }, "ENGRAM_EMBED_DIM"},
		{"openai base_url empty", func(c *Config) { c.OpenAI.BaseURL = "" }, "ENGRAM_OPENAI_BASE_URL"},
		{"openai base_url bad scheme", func(c *Config) { c.OpenAI.BaseURL = "ftp://x" }, "scheme must be http"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validConfig()
			tc.mutate(c)
			err := c.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Validate() error = %q, want substring %q", err, tc.want)
			}
		})
	}
}

func TestValidateAggregatesAllFailures(t *testing.T) {
	c := validConfig()
	c.Qdrant.Addr = ""
	c.OpenAI.BaseURL = "ftp://x"
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want aggregated error")
	}
	for _, want := range []string{"ENGRAM_QDRANT_ADDR", "ENGRAM_OPENAI_BASE_URL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("aggregated error %q missing %q (should report ALL failures)", err, want)
		}
	}
}
