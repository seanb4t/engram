// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"testing"

	flag "github.com/spf13/pflag"
)

func TestLoadDefaults(t *testing.T) {
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
