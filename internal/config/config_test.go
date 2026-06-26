// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"testing"

	flag "github.com/spf13/pflag"
)

func TestLoadDefaults(t *testing.T) {
	// Isolate from ambient ENGRAM_* in the dev/CI shell. Empty values preserve
	// the registry default (the documented empty-env invariant), so this both
	// clears inherited overrides and keeps the assertions deterministic.
	t.Setenv("ENGRAM_OPENAI_BASE_URL", "")
	t.Setenv("ENGRAM_EMBED_MODEL", "")
	t.Setenv("ENGRAM_LISTEN_ADDR", "")
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OpenAI.BaseURL != "http://localhost:4000" {
		t.Errorf("BaseURL default = %q, want http://localhost:4000", cfg.OpenAI.BaseURL)
	}
	if cfg.Embed.Model != "ollama/bge-m3" {
		t.Errorf("Embed.Model default = %q", cfg.Embed.Model)
	}
	if cfg.Server.ListenAddr != ":8080" {
		t.Errorf("ListenAddr default = %q", cfg.Server.ListenAddr)
	}
}

// TestLoadEmptyEnvPreservesDefault locks in the invariant that an explicitly
// empty ENGRAM_* var falls through to the registry default (matching the retired
// EnvOr semantics), rather than overriding the default with "". This is the
// behavioral contract the env TransformFunc's empty-value guard provides.
func TestLoadEmptyEnvPreservesDefault(t *testing.T) {
	t.Setenv("ENGRAM_OPENAI_BASE_URL", "")
	t.Setenv("ENGRAM_QDRANT_COLLECTION", "")
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OpenAI.BaseURL != "http://localhost:4000" {
		t.Errorf("BaseURL = %q, want default preserved when env is empty", cfg.OpenAI.BaseURL)
	}
	if cfg.Qdrant.Collection != "mem_eval" {
		t.Errorf("Collection = %q, want default preserved when env is empty", cfg.Qdrant.Collection)
	}
}

func TestLoadEnvOverridesDefault(t *testing.T) {
	t.Setenv("ENGRAM_OPENAI_BASE_URL", "http://embed.internal:9000")
	t.Setenv("ENGRAM_QDRANT_COLLECTION", "prod")
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OpenAI.BaseURL != "http://embed.internal:9000" {
		t.Errorf("BaseURL = %q, want env value", cfg.OpenAI.BaseURL)
	}
	if cfg.Qdrant.Collection != "prod" {
		t.Errorf("Collection = %q, want prod", cfg.Qdrant.Collection)
	}
}

func TestLoadChangedFlagOverridesEnv(t *testing.T) {
	t.Setenv("ENGRAM_OIDC_ISSUER", "https://env-issuer")
	f := flag.NewFlagSet("serve", flag.ContinueOnError)
	f.String("oidc-issuer", "", "")
	if err := f.Parse([]string{"--oidc-issuer", "https://flag-issuer"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OIDC.Issuer != "https://flag-issuer" {
		t.Errorf("Issuer = %q, want flag to override env", cfg.OIDC.Issuer)
	}
}

func TestLoadUnsetFlagDoesNotClobberEnv(t *testing.T) {
	t.Setenv("ENGRAM_OIDC_ISSUER", "https://env-issuer")
	f := flag.NewFlagSet("serve", flag.ContinueOnError)
	f.String("oidc-issuer", "", "")
	if err := f.Parse([]string{}); err != nil { // flag NOT set
		t.Fatalf("parse: %v", err)
	}
	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OIDC.Issuer != "https://env-issuer" {
		t.Errorf("Issuer = %q, want env value preserved (unset flag must not clobber)", cfg.OIDC.Issuer)
	}
}

func TestSummarizeConfigDefaultsAndEnv(t *testing.T) {
	t.Setenv("ENGRAM_SUMMARY_MODEL", "summary-cheap")
	c, err := Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Summarize.Model != "summary-cheap" {
		t.Errorf("Summarize.Model = %q, want summary-cheap", c.Summarize.Model)
	}
	if c.Summarize.MaxChars != "280" {
		t.Errorf("Summarize.MaxChars = %q, want default 280", c.Summarize.MaxChars)
	}
}

func TestValidateRejectsBadSummaryMaxCharsWhenEnabled(t *testing.T) {
	c := &Config{
		Qdrant:    QdrantConfig{Addr: "localhost:6334", Collection: "c"},
		Embed:     EmbedConfig{Model: "m", Dim: "1024"},
		OpenAI:    OpenAIConfig{BaseURL: "http://localhost:4000"},
		Summarize: SummarizeConfig{Model: "summary-cheap", MaxChars: "0"},
	}
	if err := c.Validate(); err == nil {
		t.Fatal("want error for ENGRAM_SUMMARY_MAX_CHARS=0 with model set, got nil")
	}
}

func TestValidateIgnoresSummaryWhenDisabled(t *testing.T) {
	c := &Config{
		Qdrant:    QdrantConfig{Addr: "localhost:6334", Collection: "c"},
		Embed:     EmbedConfig{Model: "m", Dim: "1024"},
		OpenAI:    OpenAIConfig{BaseURL: "http://localhost:4000"},
		Summarize: SummarizeConfig{Model: "", MaxChars: "garbage"},
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("disabled summarize must not fail validation: %v", err)
	}
}
