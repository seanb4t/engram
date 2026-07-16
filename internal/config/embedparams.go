// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"encoding/json"
	"fmt"
)

// ReservedEmbedParamKeys are the request-body keys the embedder sets
// authoritatively; operator-supplied params (ENGRAM_EMBED_*_PARAMS) must
// never override them. This is the single source of truth for the reserved
// key list (#304): ParseEmbedParams enforces it directly, and
// internal/embed.ReservedParamKeys aliases it so the embedder's wire
// contract and this validation cannot silently desync.
//
// Canonically this list would live in internal/embed (the package that
// defines the wire contract), with internal/config importing it — but
// internal/embed already imports internal/telemetry, which imports
// internal/config, so a config -> embed edge would create an import cycle.
// Declaring the list here (config has no internal dependents) and having
// embed depend on config instead avoids the cycle while keeping a single
// shared source.
var ReservedEmbedParamKeys = []string{"model", "input"}

// ParseEmbedParams parses an ENGRAM_EMBED_*_PARAMS value (name is the env var,
// used in errors) into a request-body param map. An empty string is a valid
// no-op and returns (nil, nil). A non-empty value must be a JSON object; the
// reserved keys "model" and "input" are rejected because the embedder sets them
// authoritatively and operator params must not override them. `null`, arrays,
// and scalars are valid JSON but not objects and are rejected.
func ParseEmbedParams(name, s string) (map[string]any, error) {
	if s == "" {
		return nil, nil
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, fmt.Errorf("%s: must be a JSON object: %w", name, err)
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: must be a JSON object, got %T", name, v)
	}
	for _, k := range ReservedEmbedParamKeys {
		if _, exists := m[k]; exists {
			return nil, fmt.Errorf("%s: must not contain reserved key %q", name, k)
		}
	}
	return m, nil
}
