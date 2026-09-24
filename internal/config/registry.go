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
	// server.mcp_resource_url (D-03, GH-526): a brand-new key, no Legacy value
	// (nothing retired to guard against) and no Flag (env-only — a deployment-
	// topology value, never typed at a prompt).
	{Key: "server.mcp_resource_url", Env: "ENGRAM_MCP_RESOURCE_URL"},
	{Key: "qdrant.addr", Env: "ENGRAM_QDRANT_ADDR", Legacy: "MEM_QDRANT_ADDR", Default: "localhost:6334"},
	{Key: "qdrant.collection", Env: "ENGRAM_QDRANT_COLLECTION", Legacy: "MEM_QDRANT_COLLECTION", Default: "mem_eval"},
	{Key: "embed.model", Env: "ENGRAM_EMBED_MODEL", Legacy: "MEM_EMBED_MODEL", Default: "ollama/bge-m3"},
	{Key: "embed.dim", Env: "ENGRAM_EMBED_DIM", Legacy: "MEM_EMBED_DIM", Default: "1024"},
	{Key: "embed.query_instruction", Env: "ENGRAM_EMBED_QUERY_INSTRUCTION"},
	{Key: "embed.query_params", Env: "ENGRAM_EMBED_QUERY_PARAMS"},
	{Key: "embed.document_params", Env: "ENGRAM_EMBED_DOCUMENT_PARAMS"},
	{Key: "embed.document_instruction", Env: "ENGRAM_EMBED_DOCUMENT_INSTRUCTION"},
	{Key: "embed.timeout", Env: "ENGRAM_EMBED_TIMEOUT", Default: "30s"},
	// embed.drain_bytes / embed.drain_timeout (07-bounded-provider-responses
	// D-04, D-05): brand-new keys, no Legacy value (nothing retired to guard
	// against) and no Flag (a provider-tuning value, never typed at a
	// prompt). Zero is a deliberately supported operator setting on either —
	// it closes the connection immediately after draining nothing rather
	// than let it be reused — while a negative value is rejected outright
	// (the same "zero valid, negative rejected" framing memory.max_summary_bytes
	// and embed.timeout's own validation already use). There is no value
	// meaning "no bound".
	{Key: "embed.drain_bytes", Env: "ENGRAM_EMBED_DRAIN_BYTES", Default: "262144"},
	{Key: "embed.drain_timeout", Env: "ENGRAM_EMBED_DRAIN_TIMEOUT", Default: "2s"},
	// embed.max_timeout (07-bounded-provider-responses D-07, D-08): a
	// brand-new key, no Legacy value, no Flag. UNLIKE the two drain bounds
	// immediately above, zero (and any non-positive value) is ALWAYS
	// rejected — this is the ceiling a non-positive embed.timeout now
	// resolves to, and a zero ceiling would silently reintroduce the
	// unbounded request this phase exists to remove.
	{Key: "embed.max_timeout", Env: "ENGRAM_EMBED_MAX_TIMEOUT", Default: "10m"},
	// memory.max_summary_bytes (D-06a/D-18): a brand-new key, no Legacy value —
	// the bound did not exist before this phase, so there is nothing retired to
	// guard against.
	{Key: "memory.max_summary_bytes", Env: "ENGRAM_MEMORY_MAX_SUMMARY_BYTES", Default: "512"},
	// memory.max_content_bytes (D-01/D-09): a brand-new key, no Legacy value —
	// this bound did not exist before this phase. UNLIKE memory.max_summary_bytes,
	// it is ALWAYS enforced: Config.Validate rejects "0" and non-positive values
	// rather than honoring them as "disabled", because plan 03-02 derives the
	// read-side per-record ceiling from this cap and a disabled cap would
	// silently remove that provable bound.
	{Key: "memory.max_content_bytes", Env: "ENGRAM_MEMORY_MAX_CONTENT_BYTES", Default: "65536"},
	// memory.max_tags / memory.max_tag_bytes (D-10): the tag-COUNT cap and the
	// per-tag byte cap, both brand-new, both always enforced — same D-09
	// divergence as memory.max_content_bytes above ("0" is rejected, never
	// honored as disabled).
	{Key: "memory.max_tags", Env: "ENGRAM_MEMORY_MAX_TAGS", Default: "128"},
	{Key: "memory.max_tag_bytes", Env: "ENGRAM_MEMORY_MAX_TAG_BYTES", Default: "128"},
	{Key: "summarize.model", Env: "ENGRAM_SUMMARY_MODEL"},
	{Key: "summarize.max_chars", Env: "ENGRAM_SUMMARY_MAX_CHARS", Default: "280"},
	{Key: "summarize.max_tokens", Env: "ENGRAM_SUMMARY_MAX_TOKENS", Default: "1024"},
	{Key: "summarize.timeout", Env: "ENGRAM_SUMMARY_TIMEOUT", Default: "30s"},
	// summarize.drain_bytes / summarize.drain_timeout / summarize.max_timeout
	// (07-bounded-provider-responses D-04, D-05, D-08): the summarize-lane
	// mirror of the embed.* trio above — same brand-new/no-Legacy/no-Flag
	// shape, same "zero is a supported skip-the-drain setting, negative is
	// rejected" framing for the drain pair, and the same "always rejects a
	// non-positive value" ceiling framing for max_timeout. Note the env-var
	// prefix: ENGRAM_SUMMARY_*, not ENGRAM_SUMMARIZE_*, matching the existing
	// summarize.timeout / ENGRAM_SUMMARY_TIMEOUT convention.
	{Key: "summarize.drain_bytes", Env: "ENGRAM_SUMMARY_DRAIN_BYTES", Default: "262144"},
	{Key: "summarize.drain_timeout", Env: "ENGRAM_SUMMARY_DRAIN_TIMEOUT", Default: "2s"},
	{Key: "summarize.max_timeout", Env: "ENGRAM_SUMMARY_MAX_TIMEOUT", Default: "10m"},
	{Key: "summarize.on_write", Env: "ENGRAM_SUMMARY_ON_WRITE", Default: "false"},
	{Key: "summarize.workers", Env: "ENGRAM_SUMMARY_WORKERS", Default: "2"},
	{Key: "summarize.queue_size", Env: "ENGRAM_SUMMARY_QUEUE_SIZE", Default: "256"},
	{Key: "openai.base_url", Env: "ENGRAM_OPENAI_BASE_URL", Legacy: "MEM_LITELLM_URL", Default: "http://localhost:4000"},
	{Key: "openai.api_key", Env: "ENGRAM_OPENAI_API_KEY", Legacy: "MEM_LITELLM_KEY"},
	{Key: "openai.embeddings_url", Env: "ENGRAM_OPENAI_EMBEDDINGS_URL"},
	{Key: "openai.chat_base_url", Env: "ENGRAM_OPENAI_CHAT_BASE_URL"},
	{Key: "openai.chat_api_key", Env: "ENGRAM_OPENAI_CHAT_API_KEY"},
	// decisions.* (DEC-01/DEC-02/DEC-03, D-01/D-02/D-03): brand-new keys, no
	// Legacy value (nothing retired to guard against) and no Flag
	// (provider-tuning values, never typed at a prompt — the same
	// embed.drain_bytes precedent). Presence enables the feature: an empty
	// decisions.provider constructs no client and makes no call (D-01),
	// mirroring summarize.model's presence-enables convention. Only
	// decisions.api_key falls back — to ENGRAM_OPENAI_API_KEY, resolved at
	// the wiring seam (cmp.Or in internal/server/decider.go), not here,
	// mirroring openai.chat_api_key's own fallback precedent.
	// decisions.base_url deliberately does NOT get this treatment: it has no
	// Default and fails Config.Validate when empty and provider=jev, rather
	// than silently inheriting the chat/embeddings base URL (D-03). The
	// success-path response-bytes bound is an internal constant in
	// internal/decide/jev, not a registry row (RESEARCH.md Pitfall 4).
	//
	// decisions.verdict_threshold (D-08) and decisions.verdict_state_chars
	// (D-09) are consumed by the operator CLI's verdict pass (spine-review
	// consolidate), not the server: env-only like their siblings above, since
	// operator commands load config with no flag overlay (config.Load(nil));
	// --verdict-threshold (plan 03-05) is a command-local override, not a
	// second config source.
	{Key: "decisions.provider", Env: "ENGRAM_DECISIONS_PROVIDER"},
	{Key: "decisions.base_url", Env: "ENGRAM_DECISIONS_BASE_URL"},
	{Key: "decisions.api_key", Env: "ENGRAM_DECISIONS_API_KEY"},
	{Key: "decisions.model", Env: "ENGRAM_DECISIONS_MODEL", Default: "typesafe/jev-1.13"},
	{Key: "decisions.timeout", Env: "ENGRAM_DECISIONS_TIMEOUT", Default: "10s"},
	{Key: "decisions.max_timeout", Env: "ENGRAM_DECISIONS_MAX_TIMEOUT", Default: "10m"},
	{Key: "decisions.drain_bytes", Env: "ENGRAM_DECISIONS_DRAIN_BYTES", Default: "262144"},
	{Key: "decisions.drain_timeout", Env: "ENGRAM_DECISIONS_DRAIN_TIMEOUT", Default: "2s"},
	{Key: "decisions.concurrency", Env: "ENGRAM_DECISIONS_CONCURRENCY", Default: "4"},
	{Key: "decisions.verdict_threshold", Env: "ENGRAM_DECISIONS_VERDICT_THRESHOLD", Default: "0.9"},
	{Key: "decisions.verdict_state_chars", Env: "ENGRAM_DECISIONS_VERDICT_STATE_CHARS", Default: "1500"},
	{Key: "oidc.issuer", Env: "ENGRAM_OIDC_ISSUER", Legacy: "MEM_OIDC_ISSUER", Flag: "oidc-issuer"},
	{Key: "oidc.audience", Env: "ENGRAM_OIDC_AUDIENCE", Legacy: "MEM_OIDC_AUDIENCE", Flag: "oidc-audience"},
	{Key: "oidc.client_id", Env: "ENGRAM_OIDC_CLIENT_ID", Legacy: "MEM_OIDC_CLIENT_ID", Flag: "oidc-client-id"},
	{Key: "oidc.client_secret", Env: "ENGRAM_OIDC_CLIENT_SECRET", Legacy: "MEM_OIDC_CLIENT_SECRET", Flag: "oidc-client-secret"},
	{Key: "oidc.resource_metadata", Env: "ENGRAM_OIDC_RESOURCE_METADATA", Legacy: "MEM_OIDC_RESOURCE_METADATA", Flag: "oidc-resource-metadata"},
	{Key: "oidc.owner_claim", Env: "ENGRAM_OWNER_CLAIM", Flag: "owner-claim", Default: "email"},
	// service_auth.* is the machine-to-machine lane (D-11): the client-credentials
	// issuer/audience/owner-claims are independent of the human oidc.* lane
	// (D-14 — never shared), and static_tokens is the operator-managed
	// token→owner map (ENGRAM_-only, no Flag — it is a secret). No Legacy value:
	// these are brand-new vars.
	{Key: "service_auth.oidc_issuer", Env: "ENGRAM_SERVICE_AUTH_OIDC_ISSUER"},
	{Key: "service_auth.oidc_audience", Env: "ENGRAM_SERVICE_AUTH_OIDC_AUDIENCE"},
	{Key: "service_auth.owner_claims", Env: "ENGRAM_SERVICE_AUTH_OWNER_CLAIMS", Default: "client_id,azp"},
	{Key: "service_auth.static_tokens", Env: "ENGRAM_SERVICE_AUTH_STATIC_TOKENS"},
	{Key: "ui.enabled", Env: "ENGRAM_UI_ENABLED", Legacy: "MEM_UI_ENABLED", Flag: "ui-enabled"},
	{Key: "ui.issuer", Env: "ENGRAM_UI_ISSUER", Legacy: "MEM_UI_ISSUER", Flag: "ui-issuer"},
	{Key: "ui.redirect_url", Env: "ENGRAM_UI_REDIRECT_URL", Legacy: "MEM_UI_REDIRECT_URL", Flag: "ui-redirect-url"},
	{Key: "ui.cookie_key", Env: "ENGRAM_UI_COOKIE_KEY", Legacy: "MEM_UI_COOKIE_KEY", Flag: "ui-cookie-key"},
	// This entry names the MODE (headless operation), not the surface (D-10):
	// an "enabled" name would invite the misreading that turning it off
	// unmounts Connect even when the UI is on, which is not what it does. It
	// defaults off independently of every ui.* and service_auth.* key below.
	// No Legacy value: this is a brand-new var.
	{Key: "connect.headless", Env: "ENGRAM_CONNECT_HEADLESS", Flag: "connect-headless", Default: "false"},
	{Key: "log.level", Env: "ENGRAM_LOG_LEVEL", Legacy: "MEM_LOG_LEVEL", Default: "info"},
	{Key: "log.format", Env: "ENGRAM_LOG_FORMAT", Legacy: "MEM_LOG_FORMAT", Default: "json"},
	{Key: "log.stdout", Env: "ENGRAM_LOG_STDOUT", Legacy: "MEM_LOG_STDOUT", Default: "true"},
	{Key: "usage.signals", Env: "ENGRAM_USAGE_SIGNALS", Default: "true"},
	// client.* is the caller-side lane (D-04): every client-command flag now
	// routes through this same registry rather than a fourth hand-rolled
	// resolver. Brand-new — no Legacy value, since nothing retired precedes it.
	{Key: "client.server_url", Env: "ENGRAM_SERVER_URL", Flag: "server"},
	// token_file carries only the credential's PATH through koanf; the
	// credential itself stays with resolveToken's existing env-var read, with
	// deliberately no row of its own here (D-13 — a credential must never
	// reach argv, and routing it through this registry would put it one
	// --help-adjacent step from being logged).
	{Key: "client.token_file", Flag: "token-file"},
	// output is a per-invocation rendering choice, not a deployment setting —
	// deliberately no Env.
	{Key: "client.output", Flag: "output"},
	// insecure deliberately carries no Env: the flag's own help text already
	// promises no environment fallback for the TLS gate, and adding one here
	// would weaken that promise silently.
	{Key: "client.insecure", Flag: "insecure", Default: "false"},
	{Key: "client.timeout", Env: "ENGRAM_TIMEOUT", Flag: "timeout", Default: "30s"},
	// setup.* backs `engram setup` (D-04): --url and --auth are deployment
	// facts, enrolled env-first with flag override like the 45/48 majority
	// of this registry — unlike client.token_file/client.output/
	// client.insecure above, neither rationale for omitting an Env row
	// applies here. --apply, --token-file, and --runtime deliberately get
	// NO row: --apply on the client.insecure precedent (an exported env var
	// silently flipping a preview into a mutation is the same class of
	// harm); --token-file on the client.token_file D-13 precedent (a
	// credential must never reach argv); --runtime because pflag's
	// StringSliceVar.Value.String() returns the bracketed display form
	// ("[a b]"), which the changed-flag overlay cannot round-trip — its own
	// env default (ENGRAM_RUNTIME) is read directly via os.Getenv in
	// cmd/engram/setup.go's init(), mirroring reindex.go --target.
	{Key: "setup.url", Env: "ENGRAM_URL", Flag: "url"},
	{Key: "setup.auth", Env: "ENGRAM_AUTH", Flag: "auth", Default: "oauth"},
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
