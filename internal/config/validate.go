// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"
)

// Validate reports whether c's data-plane fields are well-formed. It is pure
// (no I/O) and aggregates every failure via errors.Join, so a caller sees all
// problems at once. Each error names the ENGRAM_* env var, not the koanf key.
//
// Scope is the fields every command's store/embedder path consumes (Qdrant,
// embedder). Optional fields (ENGRAM_OPENAI_API_KEY), fields validated elsewhere
// (OIDC/UI creds via resolveUIConfig), and the serve-only listen address are
// intentionally NOT checked here. Validation lives outside config.Load on
// purpose: Load stays pure assembly so a Load error remains a programming error
// (a malformed koanf layer), never operator input.
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
	}

	// These three run unconditionally (not gated by Summarize.Model), since the
	// fields carry safe defaults and the runtime "both model set AND on_write
	// true" AND-gate (D-01) is decided later in buildDepsFromEnv, not here.
	if _, err := strconv.ParseBool(c.Summarize.OnWrite); err != nil {
		errs = append(errs, fmt.Errorf("ENGRAM_SUMMARY_ON_WRITE %q: must be a boolean: %w", c.Summarize.OnWrite, err))
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
