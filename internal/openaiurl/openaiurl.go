// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package openaiurl builds OpenAI-compatible endpoint URLs from an operator
// base URL and a request-path suffix (e.g. "embeddings", "chat/completions").
// It is the single shared implementation of the shape-aware provider-endpoint
// join used by both internal/embed and internal/summarize (D-14) — a stdlib-
// only leaf package deliberately so either lane can import it with zero cycle
// risk.
package openaiurl

import "strings"

// Join resolves baseURL to its OpenAI-compatible endpoint for suffix,
// shape-aware across documented provider bases: trims a trailing slash, then
// appends "/" + suffix directly if the path already terminates at an
// OpenAI-compat root ("/v1" or "/v1beta/openai" — Gemini's shape), else
// appends "/v1/" + suffix (the prior naive-concat behavior, still correct for
// bare-host gateways like a plain OpenAI or LiteLLM base URL). Does not
// canonicalize query/fragment-bearing base URLs — see TestJoin's
// query/fragment case for the accepted, operator-error-scope behavior (T-13-01
// trust boundary).
func Join(baseURL, suffix string) string {
	trimmed := strings.TrimRight(baseURL, "/")
	switch {
	case strings.HasSuffix(trimmed, "/v1beta/openai"):
		return trimmed + "/" + suffix
	case strings.HasSuffix(trimmed, "/v1"):
		return trimmed + "/" + suffix
	default:
		return trimmed + "/v1/" + suffix
	}
}
