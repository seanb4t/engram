// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package config loads engram's configuration. It is env-first (the ENGRAM_
// prefix) with CLI-flag overrides, realized as koanf layers: registry defaults,
// then the ENGRAM_ env layer, then a changed-flags overlay. No viper, no config
// file. The field registry (registry.go) is the single source of truth.
package config

import (
	"fmt"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/v2"
	flag "github.com/spf13/pflag"
)

// Config is engram's fully-resolved configuration. Values are kept as strings
// where the consumer already validates them (e.g. embed.dim is parsed by the
// store with a fail-fast error), so unmarshal never silently coerces.
type Config struct {
	Server    ServerConfig    `koanf:"server"`
	Qdrant    QdrantConfig    `koanf:"qdrant"`
	Embed     EmbedConfig     `koanf:"embed"`
	Summarize SummarizeConfig `koanf:"summarize"`
	OpenAI    OpenAIConfig    `koanf:"openai"`
	OIDC      OIDCConfig      `koanf:"oidc"`
	UI        UIConfig        `koanf:"ui"`
	Log       LogConfig       `koanf:"log"`
}

// ServerConfig is engram's HTTP-listener surface: where the process binds and
// the route the MCP transport mounts under.
type ServerConfig struct {
	ListenAddr string `koanf:"listen_addr"`
	MCPPath    string `koanf:"mcp_path"`
}

// QdrantConfig points engram at its vector backend: the gRPC endpoint to dial
// and the collection memories are stored in.
type QdrantConfig struct {
	Addr       string `koanf:"addr"`
	Collection string `koanf:"collection"`
}

// EmbedConfig selects the embedding model and the vector dimension the memory
// collection must be created at to match its output.
type EmbedConfig struct {
	Model string `koanf:"model"`
	Dim   string `koanf:"dim"`
}

// SummarizeConfig selects the recall-summary model and the character cap shared
// by the summarizer and recall truncation. Empty Model disables auto-summary
// (presence-enables, like OIDC issuer); MaxChars defaults to "280".
type SummarizeConfig struct {
	Model    string `koanf:"model"`
	MaxChars string `koanf:"max_chars"`
}

// OpenAIConfig is the OpenAI-compatible /v1/embeddings endpoint engram calls to
// vectorize content (any backend speaking that protocol: LiteLLM, Ollama, vLLM, …).
type OpenAIConfig struct {
	BaseURL string `koanf:"base_url"`
	APIKey  string `koanf:"api_key"`
}

// OIDCConfig holds the MCP bearer-token issuer settings and the web-UI
// confidential-client credentials.
type OIDCConfig struct {
	Issuer           string `koanf:"issuer"`
	Audience         string `koanf:"audience"`
	ClientID         string `koanf:"client_id"`
	ClientSecret     string `koanf:"client_secret"`
	ResourceMetadata string `koanf:"resource_metadata"`
}

// UIConfig holds the web-console lane settings (enable tri-state, login issuer,
// redirect URL, session-cookie key).
type UIConfig struct {
	Enabled     string `koanf:"enabled"`
	Issuer      string `koanf:"issuer"`
	RedirectURL string `koanf:"redirect_url"`
	CookieKey   string `koanf:"cookie_key"`
}

// LogConfig controls structured-log output: verbosity, encoding, and whether
// records are written to stdout.
type LogConfig struct {
	Level  string `koanf:"level"`
	Format string `koanf:"format"`
	Stdout string `koanf:"stdout"`
}

// Load builds Config from registry defaults, the ENGRAM_ env layer, and — when
// flags is non-nil — an overlay of CLI flags that were explicitly set (env-first,
// changed flags override). Pass nil for env-only consumers (store, embedder,
// telemetry); pass cmd.Flags() from serve.
func Load(flags *flag.FlagSet) (*Config, error) {
	k := koanf.New(".")

	if err := k.Load(confmap.Provider(defaultsMap(), "."), nil); err != nil {
		return nil, fmt.Errorf("config defaults: %w", err)
	}

	// First arg is the koanf key-path DELIMITER (not the prefix). It is
	// load-bearing: env.Read feeds it to maps.Unflatten, which expands the dotted
	// keys returned by TransformFunc (e.g. "openai.base_url") into the nested map
	// that Unmarshal needs. Empty delimiter would skip unflatten and zero every
	// nested field.
	if err := k.Load(env.Provider(".", env.Opt{
		Prefix: Prefix,
		TransformFunc: func(key, val string) (string, any) {
			if val == "" {
				// Empty env var preserves the registry default — same invariant
				// as the retired env-or helpers (os.Getenv=="" fell through
				// to the default). To force an empty value, set the CLI flag
				// (the changed-flag overlay bypasses this guard).
				return "", nil
			}
			if mapped, ok := envToKey[key]; ok {
				return mapped, val
			}
			return "", nil // ignore unknown ENGRAM_* vars
		},
	}), nil); err != nil {
		return nil, fmt.Errorf("config env: %w", err)
	}

	if flags != nil {
		overlay := map[string]any{}
		for name, key := range flagToKey {
			if flags.Changed(name) {
				v, err := flags.GetString(name)
				if err != nil {
					return nil, fmt.Errorf("config flag %q: %w", name, err)
				}
				overlay[key] = v
			}
		}
		if len(overlay) > 0 {
			if err := k.Load(confmap.Provider(overlay, "."), nil); err != nil {
				return nil, fmt.Errorf("config flags: %w", err)
			}
		}
	}

	var c Config
	if err := k.Unmarshal("", &c); err != nil {
		return nil, fmt.Errorf("config unmarshal: %w", err)
	}
	return &c, nil
}
