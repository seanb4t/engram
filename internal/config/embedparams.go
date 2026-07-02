// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"encoding/json"
	"fmt"
)

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
	for _, k := range []string{"model", "input"} {
		if _, exists := m[k]; exists {
			return nil, fmt.Errorf("%s: must not contain reserved key %q", name, k)
		}
	}
	return m, nil
}
