// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"cmp"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/decide"
	"github.com/seanb4t/engram/internal/decide/jev"
	"github.com/seanb4t/engram/internal/relevance"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/verdict"
)

// deciderFromConfig builds the typed-decision Decider from an already-loaded
// config. An empty provider returns (nil, nil): it constructs nothing and
// touches no network (D-01). Provider "jev" builds the Jev backend with the
// key-only fallback resolved here at the wiring seam — cfg.Decisions.APIKey
// wins when set, otherwise cfg.OpenAI.APIKey (D-03), mirroring
// summarizerFromConfig's ChatAPIKey precedent. The base URL does NOT get this
// treatment: it fails Config.Validate when empty and provider=jev instead of
// falling back here. Every other ENGRAM_DECISIONS_* knob (timeout,
// max_timeout, drain_bytes, drain_timeout, concurrency) is resolved by the
// decisions* helpers below and passed through to jev.New, mirroring the
// summary* resolver shape (D-02, D-10). Any other provider value is a
// configuration error.
func deciderFromConfig(cfg *config.Config) (decide.Decider, error) {
	switch cfg.Decisions.Provider {
	case "":
		return nil, nil
	case "jev":
		apiKey := cmp.Or(cfg.Decisions.APIKey, cfg.OpenAI.APIKey)
		return jev.New(cfg.Decisions.BaseURL, apiKey, cfg.Decisions.Model,
			jev.WithHTTPTransport(otelhttp.NewTransport(http.DefaultTransport)),
			jev.WithTimeout(decisionsTimeout(cfg)),
			jev.WithMaxTimeout(decisionsMaxTimeout(cfg)),
			jev.WithDrainBytes(decisionsDrainBytes(cfg)),
			jev.WithDrainTimeout(decisionsDrainTimeout(cfg)),
			jev.WithConcurrency(decisionsConcurrency(cfg)),
		), nil
	default:
		return nil, fmt.Errorf("ENGRAM_DECISIONS_PROVIDER %q: unknown provider (want \"\" or \"jev\")", cfg.Decisions.Provider)
	}
}

// decisionsTimeout parses the per-call HTTP timeout, defaulting to 10s on
// empty/invalid. 0 is honored by this helper (passed through unchanged);
// negatives fall back to the default — jev.New resolves a non-positive
// timeout to decisionsMaxTimeout's ceiling, never unbounded. Mirrors
// summaryTimeout.
func decisionsTimeout(cfg *config.Config) time.Duration {
	d, err := time.ParseDuration(cfg.Decisions.Timeout)
	if err != nil || d < 0 {
		if cfg.Decisions.Timeout != "" {
			slog.Warn("ENGRAM_DECISIONS_TIMEOUT is set but unparseable or negative; using default 10s",
				"value", cfg.Decisions.Timeout)
		}
		return 10 * time.Second
	}
	return d
}

// decisionsMaxTimeout parses the ceiling a non-positive decisions timeout
// resolves to, defaulting to 10m on empty/invalid. UNLIKE
// decisionsDrainBytes/decisionsDrainTimeout below, 0 is NOT a legitimate
// value here — Config.Validate rejects a non-positive
// ENGRAM_DECISIONS_MAX_TIMEOUT outright, so any non-positive value reaching
// this helper falls back to the default rather than being passed on. Mirrors
// summaryMaxTimeout.
func decisionsMaxTimeout(cfg *config.Config) time.Duration {
	d, err := time.ParseDuration(cfg.Decisions.MaxTimeout)
	if err != nil || d <= 0 {
		if cfg.Decisions.MaxTimeout != "" {
			slog.Warn("ENGRAM_DECISIONS_MAX_TIMEOUT is set but unparseable or non-positive; using default 10m",
				"value", cfg.Decisions.MaxTimeout)
		}
		return 10 * time.Minute
	}
	return d
}

// decisionsDrainBytes parses the byte bound on decide's shared post-response
// drain, defaulting to 262144 (256 KiB) on empty/invalid. Uses
// config.ParseNonNegativeIntCap — the SAME exported parser Config.Validate
// calls for ENGRAM_DECISIONS_DRAIN_BYTES — so the validated range and the
// enforced range cannot diverge. 0 is a legitimate value (skips the drain
// entirely) and is honored by this helper; only a negative or unparseable
// value falls back to the default. Mirrors summaryDrainBytes.
func decisionsDrainBytes(cfg *config.Config) int64 {
	n, err := config.ParseNonNegativeIntCap(cfg.Decisions.DrainBytes)
	if err != nil {
		if cfg.Decisions.DrainBytes != "" {
			slog.Warn("ENGRAM_DECISIONS_DRAIN_BYTES is set but unparseable or negative; using default 262144",
				"value", cfg.Decisions.DrainBytes)
		}
		return 262144
	}
	return int64(n)
}

// decisionsDrainTimeout parses the time bound on decide's shared
// post-response drain, defaulting to 2s on empty/invalid. 0 is a legitimate
// value (skips the drain entirely) and is honored by this helper; only a
// negative or unparseable value falls back to the default. Mirrors
// summaryDrainTimeout.
func decisionsDrainTimeout(cfg *config.Config) time.Duration {
	d, err := time.ParseDuration(cfg.Decisions.DrainTimeout)
	if err != nil || d < 0 {
		if cfg.Decisions.DrainTimeout != "" {
			slog.Warn("ENGRAM_DECISIONS_DRAIN_TIMEOUT is set but unparseable or negative; using default 2s",
				"value", cfg.Decisions.DrainTimeout)
		}
		return 2 * time.Second
	}
	return d
}

// decisionsConcurrency parses the DecideMany worker-pool bound (D-10),
// defaulting to 4 on empty/invalid. Uses config.ParsePositiveIntCap — the
// SAME exported parser Config.Validate calls for
// ENGRAM_DECISIONS_CONCURRENCY — so the validated range and the enforced
// range cannot diverge. UNLIKE decisionsDrainBytes/decisionsDrainTimeout
// above, 0 is NOT a legitimate value here: concurrency has no "0 means
// unbounded" escape hatch, so any non-positive value falls back to the
// default.
func decisionsConcurrency(cfg *config.Config) int {
	n, err := config.ParsePositiveIntCap(cfg.Decisions.Concurrency)
	if err != nil {
		if cfg.Decisions.Concurrency != "" {
			slog.Warn("ENGRAM_DECISIONS_CONCURRENCY is set but unparseable or non-positive; using default 4",
				"value", cfg.Decisions.Concurrency)
		}
		return 4
	}
	return n
}

// searchRerankTimeout parses the per-search decision-call timeout
// (ENGRAM_SEARCH_RERANK_TIMEOUT), defaulting to 2s on empty/invalid. UNLIKE
// decisionsTimeout, a non-positive value is NEVER honored — Config.Validate
// already rejects a non-positive ENGRAM_SEARCH_RERANK_TIMEOUT whenever the
// ranker is jev, so any non-positive value reaching this helper (an
// out-of-band call that bypassed Validate) falls back to the default rather
// than resolving to searchDeciderFromConfig's jev.WithMaxTimeout ceiling
// (10m) — unacceptable on the synchronous search path (D-09).
func searchRerankTimeout(cfg *config.Config) time.Duration {
	d, err := time.ParseDuration(cfg.Search.RerankTimeout)
	if err != nil || d <= 0 {
		if cfg.Search.RerankTimeout != "" {
			slog.Warn("ENGRAM_SEARCH_RERANK_TIMEOUT is set but unparseable or non-positive; using default 2s",
				"value", cfg.Search.RerankTimeout)
		}
		return 2 * time.Second
	}
	return d
}

// searchDeciderFromConfig builds a SECOND, dedicated Jev client for the
// search-reranking path (D-09): the same base URL, key fallback, model, OTel
// transport, max-timeout ceiling and drain bounds as deciderFromConfig's
// consolidate-path client, but its own timeout (searchRerankTimeout, never
// decisionsTimeout) plus the no-retry Option below — the search path must
// never share or double the sweep client's single retry. Gated on
// cfg.Decisions.Provider, NOT cfg.Search.Ranker: the retrieval eval (D-02,
// SearchRankHookFromEnv below) needs this client whenever a provider is
// configured, regardless of what the ranker is set to.
func searchDeciderFromConfig(cfg *config.Config) (decide.Decider, error) {
	switch cfg.Decisions.Provider {
	case "":
		return nil, nil
	case "jev":
		apiKey := cmp.Or(cfg.Decisions.APIKey, cfg.OpenAI.APIKey)
		return jev.New(cfg.Decisions.BaseURL, apiKey, cfg.Decisions.Model,
			jev.WithHTTPTransport(otelhttp.NewTransport(http.DefaultTransport)),
			jev.WithTimeout(searchRerankTimeout(cfg)),
			jev.WithMaxTimeout(decisionsMaxTimeout(cfg)),
			jev.WithDrainBytes(decisionsDrainBytes(cfg)),
			jev.WithDrainTimeout(decisionsDrainTimeout(cfg)),
			jev.WithNoRetry(),
		), nil
	default:
		return nil, fmt.Errorf("ENGRAM_DECISIONS_PROVIDER %q: unknown provider (want \"\" or \"jev\")", cfg.Decisions.Provider)
	}
}

// searchRankHook builds the search-path store.RankHook from cfg (D-01): nil,
// nil unless cfg.Search.Ranker is "jev" — the ranker enum is what turns
// search-path reranking on, never the presence of a decisions provider alone
// (T-04-01: a provider configured for consolidate must never silently start
// egressing search candidates). When the ranker is jev, the hook is built
// over searchDeciderFromConfig's dedicated no-retry client; a nil decider
// there (empty provider, a misconfiguration Config.Validate rejects before
// this is ever reached in production) still yields a nil hook, never a
// no-op relevance.Hook wrapper.
func searchRankHook(cfg *config.Config) (store.RankHook, error) {
	if cfg.Search.Ranker != "jev" {
		return nil, nil
	}
	dec, err := searchDeciderFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	if dec == nil {
		return nil, nil
	}
	return relevance.Hook(dec, relevance.DefaultBudget()), nil
}

// SearchRerankInfo reports whether the search-path reranker's underlying
// client is available and, when it is, the model and endpoint host it will
// egress candidate content to (never the key) and the per-search timeout —
// provenance for the retrieval eval's Jev row (D-02).
type SearchRerankInfo struct {
	Enabled      bool
	Model        string
	EndpointHost string
	Timeout      time.Duration
}

// SearchRankHookFromEnv builds the search-path store.RankHook and its
// SearchRerankInfo from a SINGLE config load, for the retrieval eval (D-02).
// UNLIKE searchRankHook, this returns a non-nil hook whenever
// ENGRAM_DECISIONS_PROVIDER is set, REGARDLESS of ENGRAM_SEARCH_RANKER — the
// eval measures the Jev reranking row even when the ranker is off in
// production. An unset provider returns a nil hook, a zero SearchRerankInfo,
// and a nil error.
func SearchRankHookFromEnv() (store.RankHook, SearchRerankInfo, error) {
	cfg, err := loadAndValidate()
	if err != nil {
		return nil, SearchRerankInfo{}, err
	}
	dec, err := searchDeciderFromConfig(cfg)
	if err != nil {
		return nil, SearchRerankInfo{}, err
	}
	if dec == nil {
		return nil, SearchRerankInfo{}, nil
	}
	var host string
	if u, err := url.Parse(cfg.Decisions.BaseURL); err == nil {
		host = u.Host
	}
	info := SearchRerankInfo{
		Enabled:      true,
		Model:        cfg.Decisions.Model,
		EndpointHost: host,
		Timeout:      searchRerankTimeout(cfg),
	}
	return relevance.Hook(dec, relevance.DefaultBudget()), info, nil
}

// logSearchRankerEnabled logs one Info line naming that search-path
// reranking is enabled: the ranker, model, the base URL's host ONLY (never
// any userinfo, path or query — T-04-01/T-04-07), the rerank timeout, and
// which env var supplied the API key — never the key's value itself. This is
// the operator-visible disclosure that every search now egresses candidate
// text (T-04-01), mirroring logDeciderEnabled's shape.
func logSearchRankerEnabled(cfg *config.Config) {
	var host string
	if u, err := url.Parse(cfg.Decisions.BaseURL); err == nil {
		host = u.Host
	}
	apiKeySource := "none"
	switch {
	case cfg.Decisions.APIKey != "":
		apiKeySource = "ENGRAM_DECISIONS_API_KEY"
	case cfg.OpenAI.APIKey != "":
		apiKeySource = "ENGRAM_OPENAI_API_KEY"
	}
	slog.Info("search reranking enabled",
		"ranker", cfg.Search.Ranker,
		"model", cfg.Decisions.Model,
		"endpoint_host", host,
		"rerank_timeout", searchRerankTimeout(cfg),
		"api_key_source", apiKeySource,
	)
}

// VerdictSettings configures the curation-verdict pass: Threshold is the
// D-08 boundary below which a verdict's relation probability is flagged
// needs_review, and StateChars is the D-09 per-record state truncation
// length. Provider, Model and EndpointHost identify what will receive
// record content — consolidate's disclosure line (plan 03-05) names them
// before anything is sent. EndpointHost is the base URL's host ONLY — never
// userinfo, path or query — mirroring logDeciderEnabled's own host-only
// rule.
type VerdictSettings struct {
	Threshold    float64
	StateChars   int
	Provider     string
	Model        string
	EndpointHost string
}

// verdictSettings resolves VerdictSettings from the registered
// ENGRAM_DECISIONS_VERDICT_THRESHOLD/ENGRAM_DECISIONS_VERDICT_STATE_CHARS
// knobs (plan 03-02), each parsed with the SAME exported parser
// Config.Validate uses (config.ParseProbability/config.ParsePositiveIntCap)
// — the validated range and the enforced range cannot diverge (WR-01).
// Falls back to the internal/verdict package defaults with a slog.Warn
// naming the var when it is set but unparseable, mirroring
// decisionsConcurrency's shape above; an unset var (empty string) is
// silently defaulted — Load's env TransformFunc already preserves the
// registry default in that case, so this function only ever sees "set but
// unparseable" as a distinct case from "unset". Provider and Model are
// copied from cfg.Decisions verbatim; EndpointHost is
// cfg.Decisions.BaseURL's host only (empty on a parse error).
func verdictSettings(cfg *config.Config) VerdictSettings {
	threshold, err := config.ParseProbability(cfg.Decisions.VerdictThreshold)
	if err != nil {
		if cfg.Decisions.VerdictThreshold != "" {
			slog.Warn("ENGRAM_DECISIONS_VERDICT_THRESHOLD is set but unparseable or out of range; using default",
				"value", cfg.Decisions.VerdictThreshold, "default", verdict.DefaultThreshold)
		}
		threshold = verdict.DefaultThreshold
	}
	stateChars, err := config.ParsePositiveIntCap(cfg.Decisions.VerdictStateChars)
	if err != nil {
		if cfg.Decisions.VerdictStateChars != "" {
			slog.Warn("ENGRAM_DECISIONS_VERDICT_STATE_CHARS is set but unparseable or non-positive; using default",
				"value", cfg.Decisions.VerdictStateChars, "default", verdict.DefaultStateChars)
		}
		stateChars = verdict.DefaultStateChars
	}
	var host string
	if u, err := url.Parse(cfg.Decisions.BaseURL); err == nil {
		host = u.Host
	}
	return VerdictSettings{
		Threshold:    threshold,
		StateChars:   stateChars,
		Provider:     cfg.Decisions.Provider,
		Model:        cfg.Decisions.Model,
		EndpointHost: host,
	}
}

// StoreAndDeciderFromEnv builds the store, the typed-decision Decider and
// the curation-verdict settings from a SINGLE config load — the same
// StoreAndXFromEnv wiring-seam shape as StoreAndSummarizerFromEnv, but
// WITHOUT that function's "feature unset means error" branch: an empty
// ENGRAM_DECISIONS_PROVIDER resolves to a nil Decider and a nil error
// (D-04), never a startup error — the curation-verdict pass is off by
// default, not a required dependency.
func StoreAndDeciderFromEnv() (*store.Store, decide.Decider, VerdictSettings, error) {
	cfg, err := loadAndValidate()
	if err != nil {
		return nil, nil, VerdictSettings{}, err
	}
	st, err := ensureStoreFromConfig(cfg)
	if err != nil {
		return nil, nil, VerdictSettings{}, err
	}
	dec, err := deciderFromConfig(cfg)
	if err != nil {
		return nil, nil, VerdictSettings{}, err
	}
	return st, dec, verdictSettings(cfg), nil
}

// DeciderFromEnv builds the typed-decision Decider and the curation-verdict
// settings from a SINGLE config load, WITHOUT a store — the curation eval
// (plan 03-07) needs no Qdrant, unlike StoreAndDeciderFromEnv above. An
// empty ENGRAM_DECISIONS_PROVIDER resolves to a nil Decider and a nil error
// (D-04), never a startup error, exactly like deciderFromConfig itself.
func DeciderFromEnv() (decide.Decider, VerdictSettings, error) {
	cfg, err := loadAndValidate()
	if err != nil {
		return nil, VerdictSettings{}, err
	}
	dec, err := deciderFromConfig(cfg)
	if err != nil {
		return nil, VerdictSettings{}, err
	}
	return dec, verdictSettings(cfg), nil
}

// logDeciderEnabled logs one Info line naming that typed decisions are
// enabled, the provider, model, the base URL's host ONLY (never any
// userinfo, path or query — T-02-05), and which env var supplied the API key
// (ENGRAM_DECISIONS_API_KEY, ENGRAM_OPENAI_API_KEY, or "none") — never the
// key's value itself (T-02-08: the D-03 fallback is made visible by naming
// its source, not by ever surfacing the secret).
func logDeciderEnabled(cfg *config.Config) {
	var host string
	if u, err := url.Parse(cfg.Decisions.BaseURL); err == nil {
		host = u.Host
	}
	apiKeySource := "none"
	switch {
	case cfg.Decisions.APIKey != "":
		apiKeySource = "ENGRAM_DECISIONS_API_KEY"
	case cfg.OpenAI.APIKey != "":
		apiKeySource = "ENGRAM_OPENAI_API_KEY"
	}
	slog.Info("typed decisions enabled",
		"provider", cfg.Decisions.Provider,
		"model", cfg.Decisions.Model,
		"endpoint_host", host,
		"api_key_source", apiKeySource,
	)
}
