// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import "testing"

func TestParseEmbedParams(t *testing.T) {
	t.Run("empty is a valid no-op", func(t *testing.T) {
		m, err := ParseEmbedParams("ENGRAM_EMBED_QUERY_PARAMS", "")
		if err != nil {
			t.Fatalf("empty: unexpected error %v", err)
		}
		if m != nil {
			t.Fatalf("empty: got %v, want nil map", m)
		}
	})

	t.Run("valid object parses", func(t *testing.T) {
		m, err := ParseEmbedParams("ENGRAM_EMBED_QUERY_PARAMS", `{"input_type":"search_query"}`)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		if m["input_type"] != "search_query" {
			t.Errorf("got %v, want input_type=search_query", m)
		}
	})

	// null, arrays, and scalars are valid JSON but not objects — all rejected.
	for _, bad := range []string{`not json`, `null`, `[1,2]`, `"str"`, `42`, `true`} {
		t.Run("rejects non-object: "+bad, func(t *testing.T) {
			if _, err := ParseEmbedParams("ENGRAM_EMBED_QUERY_PARAMS", bad); err == nil {
				t.Errorf("%s: expected error, got nil", bad)
			}
		})
	}

	for _, k := range ReservedEmbedParamKeys {
		t.Run("rejects reserved key "+k, func(t *testing.T) {
			if _, err := ParseEmbedParams("ENGRAM_EMBED_QUERY_PARAMS", `{"`+k+`":"x"}`); err == nil {
				t.Errorf("reserved key %s: expected error, got nil", k)
			}
		})
	}
}
