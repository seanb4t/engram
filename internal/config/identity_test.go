// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import "testing"

func baseEmbedderIdentityConfig() *Config {
	return &Config{
		Embed: EmbedConfig{
			Model:               "ollama/bge-m3",
			Dim:                 "1024",
			QueryInstruction:    "search_query: ",
			QueryParams:         `{"input_type":"search_query"}`,
			DocumentParams:      `{"input_type":"search_document"}`,
			DocumentInstruction: "search_document: ",
			Timeout:             "30s",
		},
		OpenAI: OpenAIConfig{
			BaseURL: "https://api.openai.com/v1",
			APIKey:  "sk-secret",
		},
	}
}

// TestEmbedderIdentityIsDeterministic pins that the hash is stable across
// repeated calls for identical config and carries the v1: scheme prefix with
// a 16-hex-char suffix.
func TestEmbedderIdentityIsDeterministic(t *testing.T) {
	cfg := baseEmbedderIdentityConfig()
	a, err := EmbedderIdentity(cfg)
	if err != nil {
		t.Fatalf("EmbedderIdentity: %v", err)
	}
	b, err := EmbedderIdentity(cfg)
	if err != nil {
		t.Fatalf("EmbedderIdentity: %v", err)
	}
	if a != b {
		t.Fatalf("identity not deterministic: %q != %q", a, b)
	}
	const prefix = "v1:"
	if len(a) != len(prefix)+16 || a[:len(prefix)] != prefix {
		t.Fatalf("identity %q does not match v1:<16 hex chars> shape", a)
	}
}

// TestEmbedderIdentityFieldExclusion pins D-01: only model/dim/
// document_instruction/document_params change the hash; query-side fields,
// base_url, api_key, and timeout are excluded by construction.
func TestEmbedderIdentityFieldExclusion(t *testing.T) {
	base := baseEmbedderIdentityConfig()
	want, err := EmbedderIdentity(base)
	if err != nil {
		t.Fatalf("EmbedderIdentity(base): %v", err)
	}

	changes := []struct {
		name    string
		mutate  func(*Config)
		differs bool
	}{
		{"model", func(c *Config) { c.Embed.Model = "other/model" }, true},
		{"dim", func(c *Config) { c.Embed.Dim = "768" }, true},
		{"document_instruction", func(c *Config) { c.Embed.DocumentInstruction = "different: " }, true},
		{"document_params", func(c *Config) { c.Embed.DocumentParams = `{"input_type":"other"}` }, true},
		{"query_instruction", func(c *Config) { c.Embed.QueryInstruction = "different: " }, false},
		{"query_params", func(c *Config) { c.Embed.QueryParams = `{"input_type":"other"}` }, false},
		{"base_url", func(c *Config) { c.OpenAI.BaseURL = "https://openrouter.ai/api/v1" }, false},
		{"api_key", func(c *Config) { c.OpenAI.APIKey = "sk-different" }, false},
		{"timeout", func(c *Config) { c.Embed.Timeout = "5s" }, false},
	}

	for _, tc := range changes {
		t.Run(tc.name, func(t *testing.T) {
			mutated := baseEmbedderIdentityConfig()
			tc.mutate(mutated)
			got, err := EmbedderIdentity(mutated)
			if err != nil {
				t.Fatalf("EmbedderIdentity(mutated): %v", err)
			}
			differs := got != want
			if differs != tc.differs {
				t.Fatalf("field %s: identity changed=%v, want changed=%v (base=%q mutated=%q)",
					tc.name, differs, tc.differs, want, got)
			}
		})
	}
}

// TestEmbedderIdentityCanonicalization pins the canonical-serialization
// group: key-order-different DocumentParams hash identically, and (review
// round-2 MEDIUM) the two semantically-empty spellings "" and "{}" hash
// identically to each other but still differ from a populated value.
func TestEmbedderIdentityCanonicalization(t *testing.T) {
	keyOrderA := baseEmbedderIdentityConfig()
	keyOrderA.Embed.DocumentParams = `{"a":1,"b":2}`
	keyOrderB := baseEmbedderIdentityConfig()
	keyOrderB.Embed.DocumentParams = `{"b":2,"a":1}`

	idA, err := EmbedderIdentity(keyOrderA)
	if err != nil {
		t.Fatalf("EmbedderIdentity(keyOrderA): %v", err)
	}
	idB, err := EmbedderIdentity(keyOrderB)
	if err != nil {
		t.Fatalf("EmbedderIdentity(keyOrderB): %v", err)
	}
	if idA != idB {
		t.Fatalf("key-order-different DocumentParams must hash identically: %q != %q", idA, idB)
	}

	emptyStr := baseEmbedderIdentityConfig()
	emptyStr.Embed.DocumentParams = ""
	emptyObj := baseEmbedderIdentityConfig()
	emptyObj.Embed.DocumentParams = "{}"
	populated := baseEmbedderIdentityConfig()
	populated.Embed.DocumentParams = `{"x":1}`

	idEmptyStr, err := EmbedderIdentity(emptyStr)
	if err != nil {
		t.Fatalf("EmbedderIdentity(emptyStr): %v", err)
	}
	idEmptyObj, err := EmbedderIdentity(emptyObj)
	if err != nil {
		t.Fatalf("EmbedderIdentity(emptyObj): %v", err)
	}
	idPopulated, err := EmbedderIdentity(populated)
	if err != nil {
		t.Fatalf("EmbedderIdentity(populated): %v", err)
	}

	if idEmptyStr != idEmptyObj {
		t.Fatalf(`DocumentParams=="" and DocumentParams=="{}" must hash identically (no null vs {} drift): %q != %q`,
			idEmptyStr, idEmptyObj)
	}
	if idEmptyStr == idPopulated {
		t.Fatalf("populated DocumentParams must still differ from the canonicalized-empty form: both hashed %q", idEmptyStr)
	}
}
