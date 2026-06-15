// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
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
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("invalid configuration: %w", errors.Join(errs...))
}
