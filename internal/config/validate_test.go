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
		Qdrant:    QdrantConfig{Addr: "localhost:6334", Collection: "mem_eval"},
		Embed:     EmbedConfig{Model: "ollama/bge-m3", Dim: "1024", Timeout: "30s"},
		OpenAI:    OpenAIConfig{BaseURL: "http://localhost:4000"},
		Summarize: SummarizeConfig{OnWrite: "false", Workers: "2", QueueSize: "256"},
		Usage:     UsageConfig{Signals: "true"},
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
		{"openai base_url scheme only, no host", func(c *Config) { c.OpenAI.BaseURL = "http://" }, "missing host"},
		{"embed timeout not a duration", func(c *Config) { c.Embed.Timeout = "30" }, "ENGRAM_EMBED_TIMEOUT"},
		{"embed timeout negative", func(c *Config) { c.Embed.Timeout = "-5s" }, "ENGRAM_EMBED_TIMEOUT"},
		{"embed timeout garbage", func(c *Config) { c.Embed.Timeout = "garbage" }, "ENGRAM_EMBED_TIMEOUT"},
		{"openai embeddings_url bad scheme", func(c *Config) { c.OpenAI.EmbeddingsURL = "ftp://x/embeddings" }, "ENGRAM_OPENAI_EMBEDDINGS_URL"},
		{"openai embeddings_url no host", func(c *Config) { c.OpenAI.EmbeddingsURL = "http://" }, "ENGRAM_OPENAI_EMBEDDINGS_URL"},
		{"openai chat_base_url unparseable", func(c *Config) { c.OpenAI.ChatBaseURL = "http://[::1" }, "ENGRAM_OPENAI_CHAT_BASE_URL"},
		{"openai chat_base_url bad scheme", func(c *Config) { c.OpenAI.ChatBaseURL = "ftp://x/v1" }, "ENGRAM_OPENAI_CHAT_BASE_URL"},
		{"openai chat_base_url no host", func(c *Config) { c.OpenAI.ChatBaseURL = "http://" }, "ENGRAM_OPENAI_CHAT_BASE_URL"},
		{"summary on_write non-bool", func(c *Config) { c.Summarize.OnWrite = "yes" }, "ENGRAM_SUMMARY_ON_WRITE"},
		{"summary workers non-numeric", func(c *Config) { c.Summarize.Workers = "abc" }, "ENGRAM_SUMMARY_WORKERS"},
		{"summary workers zero", func(c *Config) { c.Summarize.Workers = "0" }, "ENGRAM_SUMMARY_WORKERS"},
		{"summary queue_size non-numeric", func(c *Config) { c.Summarize.QueueSize = "abc" }, "ENGRAM_SUMMARY_QUEUE_SIZE"},
		{"summary queue_size zero", func(c *Config) { c.Summarize.QueueSize = "0" }, "ENGRAM_SUMMARY_QUEUE_SIZE"},
		{"usage signals non-bool", func(c *Config) { c.Usage.Signals = "yes" }, "ENGRAM_USAGE_SIGNALS"},
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

// summarizeEnabled returns a valid Config with auto-summary turned on, so a test
// can mutate a single summarize field to exercise one rule.
func summarizeEnabled() *Config {
	c := validConfig()
	c.Summarize = SummarizeConfig{
		Model: "summary-cheap", MaxChars: "280", MaxTokens: "1024", Timeout: "30s",
		OnWrite: "false", Workers: "2", QueueSize: "256",
	}
	return c
}

func TestValidateSummarizeEnabledHappyPath(t *testing.T) {
	if err := summarizeEnabled().Validate(); err != nil {
		t.Fatalf("Validate(summarize enabled) = %v, want nil", err)
	}
}

func TestValidateSummarizeFieldRules(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"max_tokens non-numeric", func(c *Config) { c.Summarize.MaxTokens = "abc" }, "ENGRAM_SUMMARY_MAX_TOKENS"},
		{"max_tokens negative", func(c *Config) { c.Summarize.MaxTokens = "-1" }, "ENGRAM_SUMMARY_MAX_TOKENS"},
		{"max_tokens zero is allowed", func(c *Config) { c.Summarize.MaxTokens = "0" }, ""},
		{"timeout not a duration", func(c *Config) { c.Summarize.Timeout = "30" }, "ENGRAM_SUMMARY_TIMEOUT"},
		{"timeout negative", func(c *Config) { c.Summarize.Timeout = "-5s" }, "ENGRAM_SUMMARY_TIMEOUT"},
		{"timeout zero is allowed", func(c *Config) { c.Summarize.Timeout = "0s" }, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := summarizeEnabled()
			tc.mutate(c)
			err := c.Validate()
			if tc.want == "" {
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

func TestValidateIgnoresSummaryTokensAndTimeoutWhenDisabled(t *testing.T) {
	c := summarizeEnabled()
	c.Summarize.Model = "" // disable
	c.Summarize.MaxTokens = "garbage"
	c.Summarize.Timeout = "not-a-duration"
	if err := c.Validate(); err != nil {
		t.Fatalf("disabled summarize must not validate token/timeout: %v", err)
	}
}

func TestValidateUsageSignalsFalseAccepted(t *testing.T) {
	c := validConfig()
	c.Usage.Signals = "false"
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate(usage.signals=false) = %v, want nil", err)
	}
}

func TestValidateEmbedParams(t *testing.T) {
	// Empty (the default) is valid — validConfig leaves these unset.
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("empty embed params: Validate() = %v, want nil", err)
	}

	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"query_params not an object", func(c *Config) { c.Embed.QueryParams = `"nope"` }, "ENGRAM_EMBED_QUERY_PARAMS"},
		{"query_params invalid json", func(c *Config) { c.Embed.QueryParams = `{bad` }, "ENGRAM_EMBED_QUERY_PARAMS"},
		{"query_params reserved key", func(c *Config) { c.Embed.QueryParams = `{"model":"x"}` }, "reserved key"},
		{"document_params not an object", func(c *Config) { c.Embed.DocumentParams = `[1]` }, "ENGRAM_EMBED_DOCUMENT_PARAMS"},
		{"document_params valid is ok", func(c *Config) { c.Embed.DocumentParams = `{"input_type":"search_document"}` }, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validConfig()
			tc.mutate(c)
			err := c.Validate()
			if tc.want == "" {
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

// TestValidateEmbedTimeoutUngated proves the embed.timeout check fires (and
// accepts valid values) regardless of ENGRAM_SUMMARY_MODEL — it is NOT gated
// like summarize.timeout, since the embedder is always active.
func TestValidateEmbedTimeoutUngated(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"valid 30s", func(c *Config) { c.Embed.Timeout = "30s" }, ""},
		{"valid 2m", func(c *Config) { c.Embed.Timeout = "2m" }, ""},
		{"zero accepted (infinite)", func(c *Config) { c.Embed.Timeout = "0" }, ""},
		{"negative rejected", func(c *Config) { c.Embed.Timeout = "-1s" }, "ENGRAM_EMBED_TIMEOUT"},
		{"garbage rejected", func(c *Config) { c.Embed.Timeout = "not-a-duration" }, "ENGRAM_EMBED_TIMEOUT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validConfig()
			c.Summarize.Model = "" // proves the check is unconditional
			tc.mutate(c)
			err := c.Validate()
			if tc.want == "" {
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

// TestValidateEmbeddingsURLOverride covers the D-11 operator override: empty
// is a self-gated no-op, a valid http(s) URL is accepted, malformed values are
// rejected.
func TestValidateEmbeddingsURLOverride(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"empty accepted (no-op)", func(c *Config) { c.OpenAI.EmbeddingsURL = "" }, ""},
		{"valid https URL accepted", func(c *Config) { c.OpenAI.EmbeddingsURL = "https://api.openai.com/v1/embeddings" }, ""},
		{"hostless rejected", func(c *Config) { c.OpenAI.EmbeddingsURL = "http://" }, "ENGRAM_OPENAI_EMBEDDINGS_URL"},
		{"non-http(s) scheme rejected", func(c *Config) { c.OpenAI.EmbeddingsURL = "ftp://x/embeddings" }, "ENGRAM_OPENAI_EMBEDDINGS_URL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validConfig()
			tc.mutate(c)
			err := c.Validate()
			if tc.want == "" {
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

// TestValidateChatBaseURLOverride covers the D-12/D-15 chat lane override:
// empty is a self-gated no-op (means "inherit BaseURL"), a valid http(s) URL
// is accepted, malformed values are rejected — mirroring
// TestValidateEmbeddingsURLOverride, except empty is VALID here (unlike
// ENGRAM_OPENAI_BASE_URL's own empty-string-is-an-error rule).
func TestValidateChatBaseURLOverride(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"empty accepted (no-op, inherits BaseURL)", func(c *Config) { c.OpenAI.ChatBaseURL = "" }, ""},
		{"valid https URL accepted", func(c *Config) { c.OpenAI.ChatBaseURL = "https://api.openai.com/v1" }, ""},
		{"unparseable rejected", func(c *Config) { c.OpenAI.ChatBaseURL = "http://[::1" }, "ENGRAM_OPENAI_CHAT_BASE_URL"},
		{"hostless rejected", func(c *Config) { c.OpenAI.ChatBaseURL = "http://" }, "ENGRAM_OPENAI_CHAT_BASE_URL"},
		{"non-http(s) scheme rejected", func(c *Config) { c.OpenAI.ChatBaseURL = "ftp://x/v1" }, "ENGRAM_OPENAI_CHAT_BASE_URL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validConfig()
			tc.mutate(c)
			err := c.Validate()
			if tc.want == "" {
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
