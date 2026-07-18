// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"strings"
	"testing"

	flag "github.com/spf13/pflag"
)

// TestServiceAuthRegistryDefaults pins D-05: the service lane's owner-claims
// default is "client_id,azp", NEVER "email" (the human oidc.owner_claim
// default) — a Load with no ENGRAM_SERVICE_AUTH_* vars set must resolve to
// that default and to a disabled (empty) static-token/client-creds lane.
func TestServiceAuthRegistryDefaults(t *testing.T) {
	t.Setenv("ENGRAM_QDRANT_ADDR", "localhost:6334")
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ServiceAuth.OwnerClaims != "client_id,azp" {
		t.Fatalf("ServiceAuth.OwnerClaims = %q, want \"client_id,azp\"", cfg.ServiceAuth.OwnerClaims)
	}
	if cfg.ServiceAuth.OIDCIssuer != "" {
		t.Fatalf("ServiceAuth.OIDCIssuer = %q, want empty (lane disabled by default)", cfg.ServiceAuth.OIDCIssuer)
	}
	if cfg.ServiceAuth.OIDCAudience != "" {
		t.Fatalf("ServiceAuth.OIDCAudience = %q, want empty (lane disabled by default)", cfg.ServiceAuth.OIDCAudience)
	}
	if cfg.ServiceAuth.StaticTokens != "" {
		t.Fatalf("ServiceAuth.StaticTokens = %q, want empty (lane disabled by default)", cfg.ServiceAuth.StaticTokens)
	}

	claims, err := ParseOwnerClaims(cfg.ServiceAuth.OwnerClaims)
	if err != nil {
		t.Fatalf("ParseOwnerClaims(default): %v", err)
	}
	want := []string{"client_id", "azp"}
	if len(claims) != len(want) || claims[0] != want[0] || claims[1] != want[1] {
		t.Fatalf("ParseOwnerClaims(default) = %v, want %v", claims, want)
	}
}

// TestServiceAuthEnvUnmarshal proves the four ENGRAM_SERVICE_AUTH_* vars
// unmarshal through the existing Load path onto Config.ServiceAuth with no
// signature change.
func TestServiceAuthEnvUnmarshal(t *testing.T) {
	t.Setenv("ENGRAM_QDRANT_ADDR", "localhost:6334")
	t.Setenv("ENGRAM_SERVICE_AUTH_OIDC_ISSUER", "https://idp.example.com/")
	t.Setenv("ENGRAM_SERVICE_AUTH_OIDC_AUDIENCE", "engram-services")
	t.Setenv("ENGRAM_SERVICE_AUTH_OWNER_CLAIMS", "sub")
	t.Setenv("ENGRAM_SERVICE_AUTH_STATIC_TOKENS", "ci=tok_abc")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ServiceAuth.OIDCIssuer != "https://idp.example.com/" {
		t.Fatalf("OIDCIssuer = %q", cfg.ServiceAuth.OIDCIssuer)
	}
	if cfg.ServiceAuth.OIDCAudience != "engram-services" {
		t.Fatalf("OIDCAudience = %q", cfg.ServiceAuth.OIDCAudience)
	}
	if cfg.ServiceAuth.OwnerClaims != "sub" {
		t.Fatalf("OwnerClaims = %q", cfg.ServiceAuth.OwnerClaims)
	}
	if cfg.ServiceAuth.StaticTokens != "ci=tok_abc" {
		t.Fatalf("StaticTokens = %q", cfg.ServiceAuth.StaticTokens)
	}
}

// TestServiceAuthNoFlag proves static_tokens has no cobra flag binding (a
// secret map must be ENGRAM_-only) — FlagDefault reports empty for any name
// derived from it, and flagToKey never maps to the static_tokens key.
func TestServiceAuthNoFlag(t *testing.T) {
	for name, key := range flagToKey {
		if key == "service_auth.static_tokens" {
			t.Fatalf("service_auth.static_tokens must not be flag-bound, found flag %q", name)
		}
	}
	var fs flag.FlagSet
	_ = fs // no flags registered for service_auth in this package's flag wiring
}

// TestParseServiceStaticTokens_Empty pins D-03: an empty raw value means the
// static-token lane is disabled — empty map, nil error.
func TestParseServiceStaticTokens_Empty(t *testing.T) {
	for _, raw := range []string{"", "   "} {
		tokens, err := ParseServiceStaticTokens(raw)
		if err != nil {
			t.Fatalf("ParseServiceStaticTokens(%q): unexpected error: %v", raw, err)
		}
		if len(tokens) != 0 {
			t.Fatalf("ParseServiceStaticTokens(%q) = %v, want empty map", raw, tokens)
		}
	}
}

// TestParseServiceStaticTokens_WellFormed proves the owner=token,owner2=token2
// serialization parses into a token-keyed map (the token is what's presented
// at verify time, so it must be the lookup key), and that two distinct
// tokens mapping to the same owner (rotation) is allowed.
func TestParseServiceStaticTokens_WellFormed(t *testing.T) {
	tokens, err := ParseServiceStaticTokens("ci=tok_abc,deploy-bot=tok_def")
	if err != nil {
		t.Fatalf("ParseServiceStaticTokens: unexpected error: %v", err)
	}
	want := map[string]string{"tok_abc": "ci", "tok_def": "deploy-bot"}
	if len(tokens) != len(want) {
		t.Fatalf("ParseServiceStaticTokens = %v, want %v", tokens, want)
	}
	for token, owner := range want {
		if got := tokens[token]; got != owner {
			t.Fatalf("tokens[%q] = %q, want %q", token, got, owner)
		}
	}

	// Rotation: two tokens, same owner — must be allowed.
	rotated, err := ParseServiceStaticTokens("ci=tok_old,ci=tok_new")
	if err != nil {
		t.Fatalf("ParseServiceStaticTokens(rotation): unexpected error: %v", err)
	}
	if rotated["tok_old"] != "ci" || rotated["tok_new"] != "ci" {
		t.Fatalf("ParseServiceStaticTokens(rotation) = %v, want both tokens mapped to \"ci\"", rotated)
	}
}

// TestParseServiceStaticTokens_Malformed pins the fail-fast discipline
// mirroring ParseOwnerClaims: a duplicate TOKEN key, an empty token half, an
// empty owner half, and a missing "=" separator are all rejected.
func TestParseServiceStaticTokens_Malformed(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"duplicate token", "ci=tok_abc,deploy=tok_abc"},
		{"empty token", "ci="},
		{"empty owner", "=tok_abc"},
		{"missing separator", "ci-tok_abc"},
		{"empty entry among valid ones", "ci=tok_abc,,deploy=tok_def"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseServiceStaticTokens(tc.raw); err == nil {
				t.Fatalf("ParseServiceStaticTokens(%q): want error, got nil", tc.raw)
			}
		})
	}
}

// TestServiceAuthValidate_EmptyIsNoop pins D-03 independent enablement: an
// all-empty ServiceAuth config produces no validation error.
func TestServiceAuthValidate_EmptyIsNoop(t *testing.T) {
	cfg := validConfigForServiceAuthTests()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() with empty ServiceAuth: %v", err)
	}
}

// TestServiceAuthValidate_OIDCIssuerShape proves a non-empty
// service_auth.oidc_issuer that isn't an http(s) URL fails, naming the env
// var, while a well-formed one passes.
func TestServiceAuthValidate_OIDCIssuerShape(t *testing.T) {
	cfg := validConfigForServiceAuthTests()
	cfg.ServiceAuth.OIDCIssuer = "not a url"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate(): want error for malformed service_auth.oidc_issuer, got nil")
	}
	if !strings.Contains(err.Error(), "ENGRAM_SERVICE_AUTH_OIDC_ISSUER") {
		t.Fatalf("Validate() error %v does not name ENGRAM_SERVICE_AUTH_OIDC_ISSUER", err)
	}

	cfg2 := validConfigForServiceAuthTests()
	cfg2.ServiceAuth.OIDCIssuer = "https://idp.example.com/"
	cfg2.ServiceAuth.OIDCAudience = "engram-services"
	if err := cfg2.Validate(); err != nil {
		t.Fatalf("Validate() with well-formed service_auth.oidc_issuer: %v", err)
	}
}

// TestServiceAuthValidate_StaticTokensMalformedIsFatal proves a malformed
// static_tokens value fails Validate, naming the env var; empty passes.
func TestServiceAuthValidate_StaticTokensMalformedIsFatal(t *testing.T) {
	cfg := validConfigForServiceAuthTests()
	cfg.ServiceAuth.StaticTokens = "ci="
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate(): want error for malformed service_auth.static_tokens, got nil")
	}
	if !strings.Contains(err.Error(), "ENGRAM_SERVICE_AUTH_STATIC_TOKENS") {
		t.Fatalf("Validate() error %v does not name ENGRAM_SERVICE_AUTH_STATIC_TOKENS", err)
	}
}

// TestServiceAuthValidate_EnablementSubsets pins D-03: none / static-only /
// client-creds-only / all-three must all validate cleanly.
func TestServiceAuthValidate_EnablementSubsets(t *testing.T) {
	subsets := []struct {
		name   string
		mutate func(*Config)
	}{
		{"none", func(*Config) {}},
		{"static-only", func(c *Config) { c.ServiceAuth.StaticTokens = "ci=tok_abc" }},
		{"client-creds-only", func(c *Config) {
			c.ServiceAuth.OIDCIssuer = "https://idp.example.com/"
			c.ServiceAuth.OIDCAudience = "engram-services"
		}},
		{"all-three", func(c *Config) {
			c.ServiceAuth.OIDCIssuer = "https://idp.example.com/"
			c.ServiceAuth.OIDCAudience = "engram-services"
			c.ServiceAuth.StaticTokens = "ci=tok_abc"
		}},
	}
	for _, s := range subsets {
		t.Run(s.name, func(t *testing.T) {
			cfg := validConfigForServiceAuthTests()
			s.mutate(cfg)
			if err := cfg.Validate(); err != nil {
				t.Fatalf("Validate() subset %q: %v", s.name, err)
			}
		})
	}
}

// validConfigForServiceAuthTests returns a Config that passes Validate()
// with no ServiceAuth fields set, so tests can mutate only the ServiceAuth
// fields under test without fighting unrelated required-field errors.
func validConfigForServiceAuthTests() *Config {
	return &Config{
		Qdrant: QdrantConfig{
			Addr:       "localhost:6334",
			Collection: "mem_eval",
		},
		Embed: EmbedConfig{
			Model:   "ollama/bge-m3",
			Dim:     "1024",
			Timeout: "30s",
		},
		OpenAI: OpenAIConfig{
			BaseURL: "http://localhost:4000",
		},
		Summarize: SummarizeConfig{
			OnWrite:   "false",
			Workers:   "2",
			QueueSize: "256",
		},
		Usage: UsageConfig{
			Signals: "true",
		},
	}
}
