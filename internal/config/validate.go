// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Validate reports whether c's data-plane fields are well-formed. It is pure
// (no I/O) and aggregates every failure via errors.Join, so a caller sees all
// problems at once. Each error names the ENGRAM_* env var, not the koanf key.
//
// Scope is the fields every command's store/embedder path consumes (Qdrant,
// embedder). Optional fields (ENGRAM_OPENAI_API_KEY), fields validated elsewhere
// (OIDC/UI creds via resolveUIConfig), and the serve-only listen address are
// intentionally NOT checked here. ClientConfig is another such elsewhere-
// validated group (see client_validate.go), kept deliberately separate so the
// client fields' zero value never reaches this function through one of this
// package's hand-built Config{} test literals. Validation lives outside
// config.Load on purpose: Load stays pure assembly so a Load error remains a
// programming error (a malformed koanf layer), never operator input.
func (c *Config) Validate() error {
	var errs []error

	switch host, portStr, err := net.SplitHostPort(c.Qdrant.Addr); {
	case c.Qdrant.Addr == "":
		errs = append(errs, errors.New("ENGRAM_QDRANT_ADDR is empty: must be host:port"))
	case err != nil:
		errs = append(errs, fmt.Errorf("ENGRAM_QDRANT_ADDR %q: must be host:port: %w", c.Qdrant.Addr, err))
	default:
		_ = host
		port, perr := strconv.Atoi(portStr)
		switch {
		case perr != nil:
			errs = append(errs, fmt.Errorf("ENGRAM_QDRANT_ADDR %q: port must be numeric: %w", c.Qdrant.Addr, perr))
		case port < 1 || port > 65535:
			errs = append(errs, fmt.Errorf("ENGRAM_QDRANT_ADDR %q: port %d out of range 1-65535", c.Qdrant.Addr, port))
		}
	}

	if c.Qdrant.Collection == "" {
		errs = append(errs, errors.New("ENGRAM_QDRANT_COLLECTION is empty"))
	}

	if c.Embed.Model == "" {
		errs = append(errs, errors.New("ENGRAM_EMBED_MODEL is empty"))
	}

	switch dim, err := strconv.ParseUint(c.Embed.Dim, 10, 64); {
	case c.Embed.Dim == "":
		errs = append(errs, errors.New("ENGRAM_EMBED_DIM is empty: must be a positive integer"))
	case err != nil:
		errs = append(errs, fmt.Errorf("ENGRAM_EMBED_DIM %q: must be a positive integer: %w", c.Embed.Dim, err))
	case dim == 0:
		errs = append(errs, errors.New("ENGRAM_EMBED_DIM must be greater than 0"))
	}

	// embed.timeout runs UNCONDITIONALLY (unlike summarize.timeout, which is
	// gated on Summarize.Model) — the embedder is always active, there is no
	// disabled state. Zero is accepted here and resolves to a configurable
	// ceiling in the client, per 07-bounded-provider-responses's D-07 — it no
	// longer means "no timeout (infinite)" as v0.10.x Phase 13's D-08 once
	// named it. THIS phase's own D-08 (07-bounded-provider-responses) is the
	// ceiling knob, embed.max_timeout, validated immediately below.
	switch d, err := time.ParseDuration(c.Embed.Timeout); {
	case err != nil:
		errs = append(errs, fmt.Errorf("ENGRAM_EMBED_TIMEOUT %q: must be a Go duration (e.g. 30s, 2m): %w", c.Embed.Timeout, err))
	case d < 0:
		errs = append(errs, fmt.Errorf("ENGRAM_EMBED_TIMEOUT %q: must not be negative", c.Embed.Timeout))
	}

	// embed.drain_bytes (07-bounded-provider-responses D-04, D-05): zero is a
	// deliberately supported operator setting — it skips the post-response
	// drain entirely rather than being honored as "disabled". A negative
	// value is rejected. Reuses ParseNonNegativeIntCap verbatim, the exact
	// "zero valid, negative rejected" shape ENGRAM_MEMORY_MAX_SUMMARY_BYTES
	// already uses below.
	if _, err := ParseNonNegativeIntCap(c.Embed.DrainBytes); err != nil {
		errs = append(errs, fmt.Errorf("ENGRAM_EMBED_DRAIN_BYTES %q: %w", c.Embed.DrainBytes, err))
	}

	// embed.drain_timeout (07-bounded-provider-responses D-04, D-05): the
	// same "zero valid, negative rejected" semantics as embed.drain_bytes
	// above, applied to a duration instead of a byte count — the exact shape
	// embed.timeout itself uses.
	switch d, err := time.ParseDuration(c.Embed.DrainTimeout); {
	case err != nil:
		errs = append(errs, fmt.Errorf("ENGRAM_EMBED_DRAIN_TIMEOUT %q: must be a Go duration (e.g. 30s, 2m): %w", c.Embed.DrainTimeout, err))
	case d < 0:
		errs = append(errs, fmt.Errorf("ENGRAM_EMBED_DRAIN_TIMEOUT %q: must not be negative", c.Embed.DrainTimeout))
	}

	// embed.max_timeout (07-bounded-provider-responses D-07, D-08): UNLIKE
	// the two drain bounds above, zero is always rejected here — this is the
	// ceiling a non-positive embed.timeout resolves to, and a zero ceiling
	// would silently reintroduce the unbounded request this phase exists to
	// remove. There is deliberately no way to express "unbounded".
	switch d, err := time.ParseDuration(c.Embed.MaxTimeout); {
	case err != nil:
		errs = append(errs, fmt.Errorf("ENGRAM_EMBED_MAX_TIMEOUT %q: must be a Go duration (e.g. 30s, 2m): %w", c.Embed.MaxTimeout, err))
	case d <= 0:
		errs = append(errs, fmt.Errorf("ENGRAM_EMBED_MAX_TIMEOUT %q: must be a positive duration", c.Embed.MaxTimeout))
	}

	// memory.max_summary_bytes (D-06a/D-18): a non-negative integer; "0"
	// disables the bound (validated unconditionally, mirroring
	// ENGRAM_CONNECT_HEADLESS below — a typo must fail startup, not silently
	// read as the compiled-in default). Parsed with ParseNonNegativeIntCap —
	// the same strconv.Atoi-width parser maxMemorySummaryBytes uses to build
	// the live bound (WR-01) — so a value that passes here can never
	// overflow that parser and fall back to the default silently.
	if _, err := ParseNonNegativeIntCap(c.Memory.MaxSummaryBytes); err != nil {
		errs = append(errs, fmt.Errorf("ENGRAM_MEMORY_MAX_SUMMARY_BYTES %q: %w", c.Memory.MaxSummaryBytes, err))
	}

	// memory.max_content_bytes / max_tags / max_tag_bytes (D-09/D-10): UNLIKE
	// ENGRAM_MEMORY_MAX_SUMMARY_BYTES above, these three are ALWAYS enforced —
	// "0" fails validation rather than disabling the bound, because the
	// read-side per-record ceiling (plan 03-02) is derived from these caps and
	// a disabled cap would silently remove that provable bound.
	if err := validatePositiveCap(c.Memory.MaxContentBytes, "ENGRAM_MEMORY_MAX_CONTENT_BYTES"); err != nil {
		errs = append(errs, err)
	}
	if err := validatePositiveCap(c.Memory.MaxTags, "ENGRAM_MEMORY_MAX_TAGS"); err != nil {
		errs = append(errs, err)
	}
	if err := validatePositiveCap(c.Memory.MaxTagBytes, "ENGRAM_MEMORY_MAX_TAG_BYTES"); err != nil {
		errs = append(errs, err)
	}

	switch u, err := url.Parse(c.OpenAI.BaseURL); {
	case c.OpenAI.BaseURL == "":
		errs = append(errs, errors.New("ENGRAM_OPENAI_BASE_URL is empty: must be an http(s) URL"))
	case err != nil:
		errs = append(errs, fmt.Errorf("ENGRAM_OPENAI_BASE_URL %q: must be a valid URL: %w", c.OpenAI.BaseURL, err))
	case u.Scheme != "http" && u.Scheme != "https":
		errs = append(errs, fmt.Errorf("ENGRAM_OPENAI_BASE_URL %q: scheme must be http or https", c.OpenAI.BaseURL))
	case u.Host == "":
		// url.Parse accepts scheme-only inputs like "http://"; reject them — a
		// hostless base URL is not a usable embeddings endpoint.
		errs = append(errs, fmt.Errorf("ENGRAM_OPENAI_BASE_URL %q: missing host", c.OpenAI.BaseURL))
	}

	// EmbeddingsURL is the D-11 operator override: self-gated no-op when empty
	// (the default — the join heuristic applies), validated the same way as
	// ENGRAM_OPENAI_BASE_URL when set.
	if c.OpenAI.EmbeddingsURL != "" {
		switch u, err := url.Parse(c.OpenAI.EmbeddingsURL); {
		case err != nil:
			errs = append(errs, fmt.Errorf("ENGRAM_OPENAI_EMBEDDINGS_URL %q: must be a valid URL: %w", c.OpenAI.EmbeddingsURL, err))
		case u.Scheme != "http" && u.Scheme != "https":
			errs = append(errs, fmt.Errorf("ENGRAM_OPENAI_EMBEDDINGS_URL %q: scheme must be http or https", c.OpenAI.EmbeddingsURL))
		case u.Host == "":
			errs = append(errs, fmt.Errorf("ENGRAM_OPENAI_EMBEDDINGS_URL %q: missing host", c.OpenAI.EmbeddingsURL))
		}
	}

	// ChatBaseURL (D-12/D-15) is a self-gated no-op when empty — unlike
	// ENGRAM_OPENAI_BASE_URL above, empty here is the documented, supported
	// "inherit the shared base URL" state, not an error. Do NOT copy
	// ENGRAM_OPENAI_BASE_URL's empty-string-is-an-error branch onto this field;
	// doing so would break every existing deployment on upgrade. When set, it is
	// validated the same way as ENGRAM_OPENAI_EMBEDDINGS_URL above.
	if c.OpenAI.ChatBaseURL != "" {
		switch u, err := url.Parse(c.OpenAI.ChatBaseURL); {
		case err != nil:
			errs = append(errs, fmt.Errorf("ENGRAM_OPENAI_CHAT_BASE_URL %q: must be a valid URL: %w", c.OpenAI.ChatBaseURL, err))
		case u.Scheme != "http" && u.Scheme != "https":
			errs = append(errs, fmt.Errorf("ENGRAM_OPENAI_CHAT_BASE_URL %q: scheme must be http or https", c.OpenAI.ChatBaseURL))
		case u.Host == "":
			errs = append(errs, fmt.Errorf("ENGRAM_OPENAI_CHAT_BASE_URL %q: missing host", c.OpenAI.ChatBaseURL))
		}
	}

	// service_auth.oidc_issuer/oidc_audience: self-gated no-op when empty
	// (D-03 — the client-credentials lane is simply not built), shape-checked
	// when set (mirrors the ENGRAM_OPENAI_EMBEDDINGS_URL idiom above). Only the
	// issuer is URL-shaped; the audience is an opaque string (client-id-shaped,
	// not a URL).
	if c.ServiceAuth.OIDCIssuer != "" {
		switch u, err := url.Parse(c.ServiceAuth.OIDCIssuer); {
		case err != nil:
			errs = append(errs, fmt.Errorf("ENGRAM_SERVICE_AUTH_OIDC_ISSUER %q: must be a valid URL: %w", c.ServiceAuth.OIDCIssuer, err))
		case u.Scheme != "http" && u.Scheme != "https":
			errs = append(errs, fmt.Errorf("ENGRAM_SERVICE_AUTH_OIDC_ISSUER %q: scheme must be http or https", c.ServiceAuth.OIDCIssuer))
		case u.Host == "":
			errs = append(errs, fmt.Errorf("ENGRAM_SERVICE_AUTH_OIDC_ISSUER %q: missing host", c.ServiceAuth.OIDCIssuer))
		}
	}

	// service_auth.owner_claims: reuses ParseOwnerClaims's own fail-fast rules
	// (D-05) — this runs unconditionally since the registry default
	// ("client_id,azp") is always present.
	if _, err := ParseOwnerClaims(c.ServiceAuth.OwnerClaims); err != nil {
		errs = append(errs, fmt.Errorf("ENGRAM_SERVICE_AUTH_OWNER_CLAIMS: %w", err))
	}

	// service_auth.static_tokens: empty is a valid no-op (the static-token
	// lane is off, D-03); a non-empty value that fails to parse is a fatal
	// validation error — a bad token map must fail startup, not silently
	// partial-load (T-23-10, ASVS V5).
	if c.ServiceAuth.StaticTokens != "" {
		if tokens, err := ParseServiceStaticTokens(c.ServiceAuth.StaticTokens); err != nil {
			errs = append(errs, fmt.Errorf("ENGRAM_SERVICE_AUTH_STATIC_TOKENS: %w", err))
		} else {
			// auth.chain's D-04 structural discriminator (looksLikeJWT) routes
			// any bearer with exactly two "." characters to the OIDC lane,
			// never the static lane. A configured static token shaped that way
			// can never reach the static comparator and would always be
			// rejected at runtime with no diagnostic pointing at the real
			// cause — so fail fast here instead (mirrors the duplicate-token
			// fail-fast discipline above).
			for token := range tokens {
				if strings.Count(token, ".") == 2 {
					errs = append(errs, fmt.Errorf("ENGRAM_SERVICE_AUTH_STATIC_TOKENS: token %q has exactly two \".\" characters, which the auth chain's JWT-shape discriminator routes to the OIDC lane, not the static-token lane — it would never authenticate; choose a token value without exactly two dots", token))
				}
			}
		}
	}

	// Query/document params: empty is a valid no-op (self-gated); a non-empty
	// value must be a JSON object with no reserved keys. Only well-formedness is
	// checked here — Load stays assembly-only (ADR engram-wtw).
	if _, err := ParseEmbedParams("ENGRAM_EMBED_QUERY_PARAMS", c.Embed.QueryParams); err != nil {
		errs = append(errs, err)
	}
	if _, err := ParseEmbedParams("ENGRAM_EMBED_DOCUMENT_PARAMS", c.Embed.DocumentParams); err != nil {
		errs = append(errs, err)
	}

	if c.Summarize.Model != "" {
		switch n, err := strconv.ParseUint(c.Summarize.MaxChars, 10, 64); {
		case err != nil:
			errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_MAX_CHARS %q: must be a positive integer: %w", c.Summarize.MaxChars, err))
		case n == 0:
			errs = append(errs, errors.New("ENGRAM_SUMMARY_MAX_CHARS must be greater than 0"))
		}

		// max_tokens is a non-negative ceiling; 0 omits the cap (gateway default).
		if _, err := strconv.ParseUint(c.Summarize.MaxTokens, 10, 64); err != nil {
			errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_MAX_TOKENS %q: must be a non-negative integer: %w", c.Summarize.MaxTokens, err))
		}

		// timeout is a non-negative Go duration; 0 disables the per-request timeout.
		switch d, err := time.ParseDuration(c.Summarize.Timeout); {
		case err != nil:
			errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_TIMEOUT %q: must be a Go duration (e.g. 30s, 2m): %w", c.Summarize.Timeout, err))
		case d < 0:
			errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_TIMEOUT %q: must not be negative", c.Summarize.Timeout))
		}

		// summarize.drain_bytes / summarize.drain_timeout / summarize.max_timeout
		// (07-bounded-provider-responses D-04, D-05, D-08): the summarize-lane
		// mirror of the embed.* trio above, gated the same way summarize.timeout
		// itself is — an empty summary model means no summarizer is ever built,
		// so these values are inert and unchecked until a model is configured.
		if _, err := ParseNonNegativeIntCap(c.Summarize.DrainBytes); err != nil {
			errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_DRAIN_BYTES %q: %w", c.Summarize.DrainBytes, err))
		}

		switch d, err := time.ParseDuration(c.Summarize.DrainTimeout); {
		case err != nil:
			errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_DRAIN_TIMEOUT %q: must be a Go duration (e.g. 30s, 2m): %w", c.Summarize.DrainTimeout, err))
		case d < 0:
			errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_DRAIN_TIMEOUT %q: must not be negative", c.Summarize.DrainTimeout))
		}

		switch d, err := time.ParseDuration(c.Summarize.MaxTimeout); {
		case err != nil:
			errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_MAX_TIMEOUT %q: must be a Go duration (e.g. 30s, 2m): %w", c.Summarize.MaxTimeout, err))
		case d <= 0:
			errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_MAX_TIMEOUT %q: must be a positive duration", c.Summarize.MaxTimeout))
		}
	}

	// decisions.provider (D-01): checked unconditionally, unlike every other
	// decisions.* field below — a typo in the provider enum must fail startup
	// even when the feature is otherwise off.
	if c.Decisions.Provider != "" && c.Decisions.Provider != "jev" {
		errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_PROVIDER %q: must be empty (off) or \"jev\"", c.Decisions.Provider))
	}

	// Gated like Summarize.Model above (D-01): a deployment that never sets
	// ENGRAM_DECISIONS_PROVIDER validates byte-identically to before this
	// block existed. ENGRAM_DECISIONS_API_KEY is deliberately not validated
	// here (it has no verifiable shape, and empty is meaningful: inherit
	// ENGRAM_OPENAI_API_KEY at the wiring seam, D-03).
	if c.Decisions.Provider == "jev" {
		switch u, err := url.Parse(c.Decisions.BaseURL); {
		case c.Decisions.BaseURL == "":
			errs = append(errs, errors.New("ENGRAM_DECISIONS_BASE_URL is empty: required when ENGRAM_DECISIONS_PROVIDER=jev (does not fall back to ENGRAM_OPENAI_BASE_URL)"))
		case err != nil:
			errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_BASE_URL %q: must be a valid URL: %w", c.Decisions.BaseURL, err))
		case u.Scheme != "http" && u.Scheme != "https":
			errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_BASE_URL %q: scheme must be http or https", c.Decisions.BaseURL))
		case u.Host == "":
			errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_BASE_URL %q: missing host", c.Decisions.BaseURL))
		}

		if c.Decisions.Model == "" {
			errs = append(errs, errors.New("ENGRAM_DECISIONS_MODEL is empty"))
		}

		// decisions.timeout: a non-negative Go duration; 0 resolves to the
		// max_timeout ceiling in the jev client, never unbounded.
		switch d, err := time.ParseDuration(c.Decisions.Timeout); {
		case err != nil:
			errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_TIMEOUT %q: must be a Go duration (e.g. 10s, 2m): %w", c.Decisions.Timeout, err))
		case d < 0:
			errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_TIMEOUT %q: must not be negative", c.Decisions.Timeout))
		}

		// decisions.max_timeout: UNLIKE decisions.timeout above, zero is
		// always rejected — this is the ceiling a non-positive timeout
		// resolves to, and a zero ceiling would be exactly the unbounded
		// request this rule exists to prevent.
		switch d, err := time.ParseDuration(c.Decisions.MaxTimeout); {
		case err != nil:
			errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_MAX_TIMEOUT %q: must be a Go duration (e.g. 10s, 2m): %w", c.Decisions.MaxTimeout, err))
		case d <= 0:
			errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_MAX_TIMEOUT %q: must be a positive duration", c.Decisions.MaxTimeout))
		}

		// decisions.drain_bytes / decisions.drain_timeout: zero is a
		// deliberately supported operator setting (skips the drain
		// entirely); negative is rejected — the same embed.*/summarize.*
		// drain-bound convention.
		if _, err := ParseNonNegativeIntCap(c.Decisions.DrainBytes); err != nil {
			errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_DRAIN_BYTES %q: %w", c.Decisions.DrainBytes, err))
		}

		switch d, err := time.ParseDuration(c.Decisions.DrainTimeout); {
		case err != nil:
			errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_DRAIN_TIMEOUT %q: must be a Go duration (e.g. 10s, 2m): %w", c.Decisions.DrainTimeout, err))
		case d < 0:
			errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_DRAIN_TIMEOUT %q: must not be negative", c.Decisions.DrainTimeout))
		}

		// decisions.concurrency: always positive — the DecideMany (plan
		// 02-04) worker-pool bound has no "0 means unbounded" escape hatch.
		if _, err := ParsePositiveIntCap(c.Decisions.Concurrency); err != nil {
			errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_CONCURRENCY %q: %w", c.Decisions.Concurrency, err))
		}

		// decisions.verdict_threshold (D-08): must parse as a probability in
		// [0, 1] via ParseProbability — the SAME exported parser
		// internal/server and the consolidate flag call, so the validated
		// range equals the enforced range (WR-01).
		if _, err := ParseProbability(c.Decisions.VerdictThreshold); err != nil {
			errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_VERDICT_THRESHOLD %q: %w", c.Decisions.VerdictThreshold, err))
		}

		// decisions.verdict_state_chars (D-09): always positive, like
		// decisions.concurrency above — no "0 means unbounded" escape hatch
		// (an unbounded per-record state would defeat the byte-budget
		// discipline this knob exists to enforce).
		if _, err := ParsePositiveIntCap(c.Decisions.VerdictStateChars); err != nil {
			errs = append(errs, fmt.Errorf("ENGRAM_DECISIONS_VERDICT_STATE_CHARS %q: %w", c.Decisions.VerdictStateChars, err))
		}
	}

	// search.ranker (D-01): checked unconditionally, unlike search.rerank_timeout
	// below — a typo in the ranker enum must fail startup even when reranking
	// is otherwise off.
	if c.Search.Ranker != "" && c.Search.Ranker != "lexical" && c.Search.Ranker != "jev" {
		errs = append(errs, fmt.Errorf("ENGRAM_SEARCH_RANKER %q: must be empty, \"lexical\", or \"jev\"", c.Search.Ranker))
	}

	// Gated on the ranker being "jev": a deployment that never sets
	// ENGRAM_SEARCH_RANKER=jev validates byte-identically to before this
	// block existed.
	if c.Search.Ranker == "jev" {
		if c.Decisions.Provider == "" {
			errs = append(errs, fmt.Errorf("ENGRAM_SEARCH_RANKER=jev requires ENGRAM_DECISIONS_PROVIDER to be set (naming both: ENGRAM_SEARCH_RANKER=%q, ENGRAM_DECISIONS_PROVIDER=%q)", c.Search.Ranker, c.Decisions.Provider))
		}

		// search.rerank_timeout: UNLIKE decisions.timeout, zero is always
		// rejected — a zero here would resolve to the 10m decisions max-timeout
		// ceiling on the synchronous search path, which is unacceptable (D-09).
		switch d, err := time.ParseDuration(c.Search.RerankTimeout); {
		case err != nil:
			errs = append(errs, fmt.Errorf("ENGRAM_SEARCH_RERANK_TIMEOUT %q: must be a Go duration (e.g. 2s, 500ms): %w", c.Search.RerankTimeout, err))
		case d <= 0:
			errs = append(errs, fmt.Errorf("ENGRAM_SEARCH_RERANK_TIMEOUT %q: must be a positive duration", c.Search.RerankTimeout))
		}
	}

	// These three run unconditionally (not gated by Summarize.Model), since the
	// fields carry safe defaults and the runtime "both model set AND on_write
	// true" AND-gate (D-01) is decided later in buildDepsFromEnv, not here.
	if _, err := strconv.ParseBool(c.Summarize.OnWrite); err != nil {
		errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_ON_WRITE %q: must be a boolean: %w", c.Summarize.OnWrite, err))
	}

	if _, err := strconv.ParseBool(c.Usage.Signals); err != nil {
		errs = append(errs, fmt.Errorf("ENGRAM_USAGE_SIGNALS %q: must be a boolean: %w", c.Usage.Signals, err))
	}

	// connect.headless (D-10) is validated at load, not only at point of use,
	// so a typo fails startup rather than silently reading as off.
	if _, err := strconv.ParseBool(c.Connect.Headless); err != nil {
		errs = append(errs, fmt.Errorf("ENGRAM_CONNECT_HEADLESS %q: must be a boolean: %w", c.Connect.Headless, err))
	}

	switch n, err := strconv.ParseUint(c.Summarize.Workers, 10, 64); {
	case err != nil:
		errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_WORKERS %q: must be a positive integer: %w", c.Summarize.Workers, err))
	case n == 0:
		errs = append(errs, errors.New("ENGRAM_SUMMARY_WORKERS must be greater than 0"))
	}

	switch n, err := strconv.ParseUint(c.Summarize.QueueSize, 10, 64); {
	case err != nil:
		errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_QUEUE_SIZE %q: must be a positive integer: %w", c.Summarize.QueueSize, err))
	case n == 0:
		errs = append(errs, errors.New("ENGRAM_SUMMARY_QUEUE_SIZE must be greater than 0"))
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("invalid configuration: %w", errors.Join(errs...))
}

// validatePositiveCap validates an always-enforced memory write cap (D-09):
// value must parse as a positive integer. Unlike ENGRAM_MEMORY_MAX_SUMMARY_BYTES,
// "0" is rejected outright rather than honored as "disabled" — these caps
// feed the read-side per-record ceiling (plan 03-02), and a disabled cap
// would silently remove that provable bound.
//
// Parses via ParsePositiveIntCap — the SAME strconv.Atoi-width parser
// internal/server's positiveIntOrDefault uses to build the live enforced
// cap (WR-01 fix) — rather than the wider strconv.ParseUint(value, 10, 64)
// this used before. ParseUint's uint64 range let a value like
// 9223372036854775808 (one more than math.MaxInt64) pass this check while
// positiveIntOrDefault's strconv.Atoi silently fell back to the documented
// default at runtime (only a slog.Warn, no error) — a validated-vs-enforced
// divergence D-09 exists specifically to prevent. Both sides now call the
// one exported parser so the validated range can never diverge from the
// enforced range again.
func validatePositiveCap(value, envName string) error {
	if _, err := ParsePositiveIntCap(value); err != nil {
		return fmt.Errorf("%s %q: %w: this cap is always enforced (unlike ENGRAM_MEMORY_MAX_SUMMARY_BYTES, 0 does not disable it)", envName, value, err)
	}
	return nil
}

// ParsePositiveIntCap parses value as a positive integer, using exactly the
// parser (strconv.Atoi, platform int width) that runtime enforcement builds
// the live cap with. Exported so internal/server's positiveIntOrDefault can
// call this SAME function rather than keeping a second parser in sync by
// convention (D-00: one shared helper over two parsers kept in sync by
// convention) — see validatePositiveCap's doc for the divergence this
// closes (WR-01).
func ParsePositiveIntCap(value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("must be a positive integer: %w", err)
	}
	if n <= 0 {
		return 0, errors.New("must be greater than 0")
	}
	return n, nil
}

// ParseProbability parses value as a probability in [0, 1], using
// strconv.ParseFloat and rejecting NaN, infinities, and anything outside the
// [0, 1] range. Exported so internal/server (the verdict-threshold resolver)
// and the consolidate command's --verdict-threshold flag call this SAME
// parser Config.Validate uses for ENGRAM_DECISIONS_VERDICT_THRESHOLD, so the
// validated range equals the enforced range (WR-01, mirroring
// ParsePositiveIntCap's doc and shape).
func ParseProbability(value string) (float64, error) {
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("must be a probability between 0 and 1: %w", err)
	}
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, errors.New("must be a probability between 0 and 1")
	}
	if n < 0 || n > 1 {
		return 0, errors.New("must be a probability between 0 and 1")
	}
	return n, nil
}

// ParseNonNegativeIntCap parses value as a non-negative integer, using
// exactly the parser (strconv.Atoi, platform int width) that runtime
// enforcement builds the live bound with (internal/server's
// maxMemorySummaryBytes). Zero is a valid result — callers that treat zero
// as an escape hatch (ENGRAM_MEMORY_MAX_SUMMARY_BYTES's "0 disables", D-18)
// decide that themselves; this function only bounds the parse.
func ParseNonNegativeIntCap(value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("must be a non-negative integer: %w", err)
	}
	if n < 0 {
		return 0, errors.New("must be a non-negative integer")
	}
	return n, nil
}
