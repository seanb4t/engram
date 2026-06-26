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
	{Key: "server.listen_addr", Env: "ENGRAM_LISTEN_ADDR", Legacy: "MEM_LISTEN_ADDR", Flag: "listen-addr", Default: ":8080"},
	{Key: "server.mcp_path", Env: "ENGRAM_MCP_PATH", Legacy: "MEM_MCP_PATH", Flag: "mcp-path"},
	{Key: "qdrant.addr", Env: "ENGRAM_QDRANT_ADDR", Legacy: "MEM_QDRANT_ADDR", Default: "localhost:6334"},
	{Key: "qdrant.collection", Env: "ENGRAM_QDRANT_COLLECTION", Legacy: "MEM_QDRANT_COLLECTION", Default: "mem_eval"},
	{Key: "embed.model", Env: "ENGRAM_EMBED_MODEL", Legacy: "MEM_EMBED_MODEL", Default: "ollama/bge-m3"},
	{Key: "embed.dim", Env: "ENGRAM_EMBED_DIM", Legacy: "MEM_EMBED_DIM", Default: "1024"},
	{Key: "summarize.model", Env: "ENGRAM_SUMMARY_MODEL"},
	{Key: "summarize.max_chars", Env: "ENGRAM_SUMMARY_MAX_CHARS", Default: "280"},
	{Key: "openai.base_url", Env: "ENGRAM_OPENAI_BASE_URL", Legacy: "MEM_LITELLM_URL", Default: "http://localhost:4000"},
	{Key: "openai.api_key", Env: "ENGRAM_OPENAI_API_KEY", Legacy: "MEM_LITELLM_KEY"},
	{Key: "oidc.issuer", Env: "ENGRAM_OIDC_ISSUER", Legacy: "MEM_OIDC_ISSUER", Flag: "oidc-issuer"},
	{Key: "oidc.audience", Env: "ENGRAM_OIDC_AUDIENCE", Legacy: "MEM_OIDC_AUDIENCE", Flag: "oidc-audience"},
	{Key: "oidc.client_id", Env: "ENGRAM_OIDC_CLIENT_ID", Legacy: "MEM_OIDC_CLIENT_ID", Flag: "oidc-client-id"},
	{Key: "oidc.client_secret", Env: "ENGRAM_OIDC_CLIENT_SECRET", Legacy: "MEM_OIDC_CLIENT_SECRET", Flag: "oidc-client-secret"},
	{Key: "oidc.resource_metadata", Env: "ENGRAM_OIDC_RESOURCE_METADATA", Legacy: "MEM_OIDC_RESOURCE_METADATA", Flag: "oidc-resource-metadata"},
	{Key: "ui.enabled", Env: "ENGRAM_UI_ENABLED", Legacy: "MEM_UI_ENABLED", Flag: "ui-enabled"},
	{Key: "ui.issuer", Env: "ENGRAM_UI_ISSUER", Legacy: "MEM_UI_ISSUER", Flag: "ui-issuer"},
	{Key: "ui.redirect_url", Env: "ENGRAM_UI_REDIRECT_URL", Legacy: "MEM_UI_REDIRECT_URL", Flag: "ui-redirect-url"},
	{Key: "ui.cookie_key", Env: "ENGRAM_UI_COOKIE_KEY", Legacy: "MEM_UI_COOKIE_KEY", Flag: "ui-cookie-key"},
	{Key: "log.level", Env: "ENGRAM_LOG_LEVEL", Legacy: "MEM_LOG_LEVEL", Default: "info"},
	{Key: "log.format", Env: "ENGRAM_LOG_FORMAT", Legacy: "MEM_LOG_FORMAT", Default: "json"},
	{Key: "log.stdout", Env: "ENGRAM_LOG_STDOUT", Legacy: "MEM_LOG_STDOUT", Default: "true"},
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

// flagToDefault maps a cobra flag name directly to its registry default.
var flagToDefault = func() map[string]string {
	m := make(map[string]string)
	for _, f := range registry {
		if f.Flag != "" {
			m[f.Flag] = f.Default
		}
	}
	return m
}()

// FlagDefault returns the registry default for the field bound to flag name, so
// cobra flag registration shows accurate --help defaults without duplicating
// literals. Returns "" when the flag is unknown or its field has no default.
func FlagDefault(flagName string) string {
	return flagToDefault[flagName]
}
