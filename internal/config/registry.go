// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

// Prefix is the env-var namespace for all engram configuration.
const Prefix = "ENGRAM_"

// field is one configuration key. The registry of fields is the single source
// of truth: the env-var transform, the defaults layer, the flag overlay, and
// the legacy-env guard are all derived from it. Renaming a var is a one-line
// edit here.
type field struct {
	Key     string // koanf key path, e.g. "openai.base_url"
	Env     string // current env var, e.g. "ENGRAM_OPENAI_BASE_URL"
	Legacy  string // retired env var, e.g. "MEM_LITELLM_URL" ("" if brand-new)
	Flag    string // cobra flag name that overrides it ("" if env-only)
	Default string // default value ("" if none)
}

// registry holds every server-config key. Command-local vars (migrate/reindex
// targets, the test-only Qdrant addr) are NOT here — they are read directly by
// their command, but their legacy names are registered for the guard in
// legacy.go.
var registry = []field{
	{"server.listen_addr", "ENGRAM_LISTEN_ADDR", "MEM_LISTEN_ADDR", "listen-addr", ":8080"},
	{"server.mcp_path", "ENGRAM_MCP_PATH", "MEM_MCP_PATH", "mcp-path", ""},
	{"qdrant.addr", "ENGRAM_QDRANT_ADDR", "MEM_QDRANT_ADDR", "", "localhost:6334"},
	{"qdrant.collection", "ENGRAM_QDRANT_COLLECTION", "MEM_QDRANT_COLLECTION", "", "mem_eval"},
	{"embed.model", "ENGRAM_EMBED_MODEL", "MEM_EMBED_MODEL", "", "ollama/bge-m3"},
	{"embed.dim", "ENGRAM_EMBED_DIM", "MEM_EMBED_DIM", "", "1024"},
	{"openai.base_url", "ENGRAM_OPENAI_BASE_URL", "MEM_LITELLM_URL", "", "http://localhost:4000"},
	{"openai.api_key", "ENGRAM_OPENAI_API_KEY", "MEM_LITELLM_KEY", "", ""},
	{"oidc.issuer", "ENGRAM_OIDC_ISSUER", "MEM_OIDC_ISSUER", "oidc-issuer", ""},
	{"oidc.audience", "ENGRAM_OIDC_AUDIENCE", "MEM_OIDC_AUDIENCE", "oidc-audience", ""},
	{"oidc.client_id", "ENGRAM_OIDC_CLIENT_ID", "MEM_OIDC_CLIENT_ID", "oidc-client-id", ""},
	{"oidc.client_secret", "ENGRAM_OIDC_CLIENT_SECRET", "MEM_OIDC_CLIENT_SECRET", "oidc-client-secret", ""},
	{"oidc.resource_metadata", "ENGRAM_OIDC_RESOURCE_METADATA", "MEM_OIDC_RESOURCE_METADATA", "oidc-resource-metadata", ""},
	{"ui.enabled", "ENGRAM_UI_ENABLED", "MEM_UI_ENABLED", "ui-enabled", ""},
	{"ui.issuer", "ENGRAM_UI_ISSUER", "MEM_UI_ISSUER", "ui-issuer", ""},
	{"ui.redirect_url", "ENGRAM_UI_REDIRECT_URL", "MEM_UI_REDIRECT_URL", "ui-redirect-url", ""},
	{"ui.cookie_key", "ENGRAM_UI_COOKIE_KEY", "MEM_UI_COOKIE_KEY", "ui-cookie-key", ""},
	{"log.level", "ENGRAM_LOG_LEVEL", "MEM_LOG_LEVEL", "", "info"},
	{"log.format", "ENGRAM_LOG_FORMAT", "MEM_LOG_FORMAT", "", "json"},
	{"log.stdout", "ENGRAM_LOG_STDOUT", "MEM_LOG_STDOUT", "", "true"},
}

// envToKey maps each ENGRAM_* env var to its koanf key.
var envToKey = func() map[string]string {
	m := make(map[string]string, len(registry))
	for _, f := range registry {
		m[f.Env] = f.Key
	}
	return m
}()

// defaultsMap is the registry's defaults as a koanf confmap (empty defaults omitted).
func defaultsMap() map[string]any {
	m := make(map[string]any, len(registry))
	for _, f := range registry {
		if f.Default != "" {
			m[f.Key] = f.Default
		}
	}
	return m
}

// flagToKey maps a cobra flag name to its koanf key (only fields that have a flag).
var flagToKey = func() map[string]string {
	m := make(map[string]string)
	for _, f := range registry {
		if f.Flag != "" {
			m[f.Flag] = f.Key
		}
	}
	return m
}()

// FlagDefault returns the registry default for the field bound to flag name, so
// cobra flag registration shows accurate --help defaults without duplicating
// literals. Returns "" when the flag is unknown or its field has no default.
func FlagDefault(flagName string) string {
	key, ok := flagToKey[flagName]
	if !ok {
		return ""
	}
	for _, f := range registry {
		if f.Key == key {
			return f.Default
		}
	}
	return ""
}
