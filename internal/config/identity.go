// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// EmbedderIdentity computes the v1 embedder-config-identity stamp: a short,
// version-prefixed hash over the fields that change the STORED DOCUMENT
// vector (model, dim, document_instruction, document_params). Query-side
// fields (query_instruction/query_params), base_url, api_key, and timeout
// are EXCLUDED by construction — they never alter what gets written for a
// document embed (D-01/D-02/D-03).
//
// Canonical serialization: document_params is parsed via ParseEmbedParams
// then re-marshaled (encoding/json sorts map keys on marshal, so two
// key-order-different-but-equal DocumentParams strings hash identically).
// Semantically-empty params — "" (which ParseEmbedParams returns as a nil
// map) and "{}" (an explicit empty JSON object) — are BOTH normalized to an
// empty map{} before marshal, so they hash identically too: the embed
// request path (internal/embed) treats both the same way via
// len(params)==0, so the identity must not falsely drift between the two
// empty spellings (a populated DocumentParams still differs from either).
// The preimage joins model, dim, document_instruction, and the canonical
// params JSON with the ASCII unit-separator byte (\x1f, so a value
// containing ':' or '|' cannot forge a collision), is hashed with SHA-256,
// and rendered as "v1:" + the first 16 hex characters of the digest — the
// "v1:" scheme prefix lets the hashing scheme evolve later without an old
// record ambiguously reading as embedding-space drift.
func EmbedderIdentity(cfg *Config) (string, error) {
	docParams, err := ParseEmbedParams("ENGRAM_EMBED_DOCUMENT_PARAMS", cfg.Embed.DocumentParams)
	if err != nil {
		return "", err // already validated at startup; defensive only
	}
	if len(docParams) == 0 {
		docParams = map[string]any{}
	}
	canonicalParams, err := json.Marshal(docParams)
	if err != nil {
		return "", err
	}
	preimage := strings.Join([]string{
		cfg.Embed.Model, cfg.Embed.Dim, cfg.Embed.DocumentInstruction, string(canonicalParams),
	}, "\x1f")
	sum := sha256.Sum256([]byte(preimage))
	return "v1:" + hex.EncodeToString(sum[:])[:16], nil
}
