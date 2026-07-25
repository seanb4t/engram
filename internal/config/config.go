// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package config loads engram's configuration. It is env-first (the ENGRAM_
// prefix) with CLI-flag overrides, realized as koanf layers: registry defaults,
// then the ENGRAM_ env layer, then a changed-flags overlay. No viper, no config
// file. The field registry (registry.go) is the single source of truth.
package config

import (
	"fmt"
	"strings"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/v2"
	flag "github.com/spf13/pflag"
)

// Config is engram's fully-resolved configuration. Values are kept as strings
// where the consumer already validates them (e.g. embed.dim is parsed by the
// store with a fail-fast error), so unmarshal never silently coerces.
type Config struct {
	Server      ServerConfig      `koanf:"server"`
	Qdrant      QdrantConfig      `koanf:"qdrant"`
	Embed       EmbedConfig       `koanf:"embed"`
	Summarize   SummarizeConfig   `koanf:"summarize"`
	OpenAI      OpenAIConfig      `koanf:"openai"`
	OIDC        OIDCConfig        `koanf:"oidc"`
	ServiceAuth ServiceAuthConfig `koanf:"service_auth"`
	UI          UIConfig          `koanf:"ui"`
	Log         LogConfig         `koanf:"log"`
	Usage       UsageConfig       `koanf:"usage"`
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
	// QueryInstruction is prepended to search-query embeddings for
	// instruction-tuned asymmetric models (e.g. Qwen3-Embedding). Empty (default)
	// keeps queries raw. Documents are never wrapped, so setting this needs no
	// reindex.
	QueryInstruction string `koanf:"query_instruction"`
	// QueryParams / DocumentParams are JSON objects (as raw strings) merged into
	// the /v1/embeddings request body for query vs document embeds respectively —
	// e.g. {"input_type":"search_query"}. Empty = none. Parsed/validated in
	// Config.Validate() and again in the embedder builder (cf. Dim), never
	// coerced by the koanf unmarshal.
	QueryParams    string `koanf:"query_params"`
	DocumentParams string `koanf:"document_params"`
	// DocumentInstruction is the document-side mirror of QueryInstruction: a text
	// prefix/template applied to documents at store + reindex (empty = raw).
	DocumentInstruction string `koanf:"document_instruction"`
	// Timeout is the per-request embed HTTP client timeout (ENGRAM_EMBED_TIMEOUT,
	// default "30s"); "0" disables it (no timeout). Validated unconditionally in
	// Config.Validate — the embedder is always active, unlike Summarize.Timeout
	// which is gated on Summarize.Model.
	Timeout string `koanf:"timeout"`
}

// SummarizeConfig selects the recall-summary model and the character cap shared
// by the summarizer and recall truncation. Empty Model disables auto-summary
// (presence-enables, like OIDC issuer); MaxChars defaults to "280".
//
// MaxTokens is the chat-completion generation ceiling (default "1024"). It is
// deliberately decoupled from MaxChars: the visible summary is already hard-
// capped at MaxChars runes, so this ceiling only bounds the model's token
// budget. Reasoning models spend tokens on hidden reasoning before emitting an
// answer, so a tight ceiling starves them into an empty response; a generous
// one is free for non-reasoning models (they stop at EOS well below it). "0"
// omits the cap entirely (gateway default). Timeout is the per-request HTTP
// client timeout (default "30s"); "0" disables it. Neither is the same as the
// summarize-missing command's --timeout, which bounds the whole sweep.
type SummarizeConfig struct {
	Model     string `koanf:"model"`
	MaxChars  string `koanf:"max_chars"`
	MaxTokens string `koanf:"max_tokens"`
	Timeout   string `koanf:"timeout"`
	// OnWrite enables the async-on-write summary worker (default "false"),
	// parsed with strconv.ParseBool at point of use. Runtime enablement is an
	// AND-gate with Model being non-empty (D-01), decided in buildDepsFromEnv.
	OnWrite string `koanf:"on_write"`
	// Workers is the async summary worker pool size (default "2").
	Workers string `koanf:"workers"`
	// QueueSize is the async summary enqueue channel bound (default "256").
	QueueSize string `koanf:"queue_size"`
}

// OpenAIConfig is the OpenAI-compatible /v1/embeddings endpoint engram calls to
// vectorize content (any backend speaking that protocol: LiteLLM, Ollama, vLLM, …).
type OpenAIConfig struct {
	BaseURL string `koanf:"base_url"`
	APIKey  string `koanf:"api_key"`
	// EmbeddingsURL is the ENGRAM_OPENAI_EMBEDDINGS_URL full-URL override that
	// bypasses joinEmbeddingsURL's shape-aware heuristic and is used verbatim as
	// the embeddings endpoint (D-11). Empty (the default) keeps the heuristic.
	EmbeddingsURL string `koanf:"embeddings_url"`
	// ChatBaseURL is the ENGRAM_OPENAI_CHAT_BASE_URL base URL for the
	// chat/summarize lane. Empty (the default; no registry default is set) means
	// inherit BaseURL — the fallback is resolved once, at the summarizer
	// construction site (cmp.Or in summarizerFromConfig), not here, so this
	// field always faithfully reflects what the operator set (D-12).
	ChatBaseURL string `koanf:"chat_base_url"`
}

// OIDCConfig holds the MCP bearer-token issuer settings and the web-UI
// confidential-client credentials.
type OIDCConfig struct {
	Issuer           string `koanf:"issuer"`
	Audience         string `koanf:"audience"`
	ClientID         string `koanf:"client_id"`
	ClientSecret     string `koanf:"client_secret"`
	ResourceMetadata string `koanf:"resource_metadata"`
	// OwnerClaim is the OIDC claim whose value becomes a record's owner (the
	// authz key) when auth is enabled. Shared by both auth lanes. Default "email".
	OwnerClaim string `koanf:"owner_claim"`
}

// ServiceAuthConfig holds the machine-to-machine auth lane (D-11): the
// client-credentials issuer/audience/owner-claims are wholly independent of
// the human OIDCConfig lane (D-14 — distinct issuer/audience, never shared),
// and StaticTokens is the operator-managed token→owner map for the
// static-token lane (parsed by ParseServiceStaticTokens; ENGRAM_-only, no
// CLI flag, since it carries secrets). Each lane is independently enabled by
// config presence (D-03): empty OIDCIssuer/OIDCAudience means the
// client-credentials lane is off; empty StaticTokens means the static-token
// lane is off. Neither lane is required for the other.
type ServiceAuthConfig struct {
	OIDCIssuer   string `koanf:"oidc_issuer"`
	OIDCAudience string `koanf:"oidc_audience"`
	// OwnerClaims is a comma-separated ordered claim-name list parsed via
	// ParseOwnerClaims, defaulting to "client_id,azp" (D-05) — the service
	// lane's owner-claim precedence must NEVER default to "email", the human
	// lane's default, so a client-credentials token cannot accidentally
	// resolve an owner via a claim meant for a human identity.
	OwnerClaims string `koanf:"owner_claims"`
	// StaticTokens is the raw ENGRAM_SERVICE_AUTH_STATIC_TOKENS value: a
	// comma-separated list of "owner=token" pairs (parsed by
	// ParseServiceStaticTokens into a token→ownerID map). Empty disables the
	// static-token lane entirely.
	StaticTokens string `koanf:"static_tokens"`
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

// UsageConfig gates the get-path async access_count / last_accessed_at payload
// write (D-09) — its purpose is to eliminate read-path write amplification, not
// to disable all counting. When false, buildUsageQueue returns a nil usageQueue
// so get_memory performs zero extra writes. The update_memory bump in
// store.Update is intentionally NOT gated: it piggybacks the already-in-flight
// Upsert (D-04), adds no extra store round-trip, and so causes no read-path
// amplification regardless of this flag.
// Signals defaults to "true" — unlike summarize.on_write, this is local,
// non-egressing curation metadata, so it is on by default. Parsed with
// strconv.ParseBool at point of use in buildUsageQueue, never a native bool
// at Load() time (mirrors ui.enabled / summarize.on_write). The D-06 OTLP
// recall-id spans are independent of this flag.
type UsageConfig struct {
	Signals string `koanf:"signals"`
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

// ParseOwnerClaims comma-splits and trims raw into an ordered list of OIDC
// claim names tried, in order, to resolve a record's owner (see
// auth.ClaimIdentity). Parsing is deliberately separated from defaulting: the
// registry already supplies "email" when ENGRAM_OWNER_CLAIM is unset (and an
// empty ENV var preserves that default via Load's TransformFunc guard) — this
// function only parses whatever string reaches it. A wholly-empty (or
// after-trim-empty, e.g. ",", ",,", whitespace) value returns an EMPTY list
// with a nil error — it does NOT default to ["email"] — so a caller's startup
// guard can still observe an operator's explicit --owner-claim="".
//
// A malformed non-empty list is a fail-fast error, never silently normalized
// (this is an auth key): a duplicate claim name is rejected (not deduped), an
// empty entry among non-empty ones (e.g. "email,,sub") is rejected (not
// dropped), and a claim name containing ':' or ',' or starting with a digit
// is rejected (both would break or spoof the length-prefixed namespace
// encoding in auth.ClaimIdentity).
func ParseOwnerClaims(raw string) ([]string, error) {
	fields := strings.Split(raw, ",")
	trimmed := make([]string, len(fields))
	allEmpty := true
	anyEmpty := false
	for i, f := range fields {
		trimmed[i] = strings.TrimSpace(f)
		if trimmed[i] == "" {
			anyEmpty = true
		} else {
			allEmpty = false
		}
	}
	if allEmpty {
		return nil, nil
	}
	if anyEmpty {
		return nil, fmt.Errorf("owner-claim list %q has an empty entry", raw)
	}

	seen := make(map[string]bool, len(trimmed))
	claims := make([]string, 0, len(trimmed))
	for _, c := range trimmed {
		if seen[c] {
			return nil, fmt.Errorf("owner-claim list %q has a duplicate claim %q", raw, c)
		}
		seen[c] = true
		if strings.ContainsAny(c, ":,") {
			return nil, fmt.Errorf("owner-claim %q must not contain ':' or ','", c)
		}
		if first := c[0]; first >= '0' && first <= '9' {
			return nil, fmt.Errorf("owner-claim %q must not start with a digit", c)
		}
		claims = append(claims, c)
	}
	return claims, nil
}

// ParseServiceStaticTokens parses ENGRAM_SERVICE_AUTH_STATIC_TOKENS (D-11):
// a comma-separated list of "owner=token" pairs, returned as a token→ownerID
// map (the token is the map key, since verification looks up by the
// presented token). An empty raw value returns an empty map and a nil error
// (the static-token lane is simply disabled — D-03).
//
// Parsing is fail-fast, mirroring ParseOwnerClaims's discipline (this is an
// auth key): an entry missing the "=" separator, or with an empty owner or
// empty token half, is rejected. A duplicate TOKEN is rejected (ambiguous —
// which owner would it resolve to?). Two distinct tokens mapping to the SAME
// owner are explicitly allowed (token rotation: an operator can mint a new
// token for a principal before revoking the old one).
func ParseServiceStaticTokens(raw string) (map[string]string, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]string{}, nil
	}

	pairs := strings.Split(raw, ",")
	tokens := make(map[string]string, len(pairs))
	for _, p := range pairs {
		p = strings.TrimSpace(p)
		if p == "" {
			return nil, fmt.Errorf("static-tokens list %q has an empty entry", raw)
		}
		owner, token, ok := strings.Cut(p, "=")
		owner = strings.TrimSpace(owner)
		token = strings.TrimSpace(token)
		if !ok || owner == "" || token == "" {
			return nil, fmt.Errorf("static-tokens entry %q must be \"owner=token\" with both non-empty", p)
		}
		if _, dup := tokens[token]; dup {
			return nil, fmt.Errorf("static-tokens list %q has a duplicate token", raw)
		}
		tokens[token] = owner
	}
	return tokens, nil
}
