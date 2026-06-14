// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"strings"
	"testing"
)

func TestCheckLegacyNoneSet(t *testing.T) {
	if err := CheckLegacy([]string{"PATH=/bin", "ENGRAM_OPENAI_BASE_URL=http://x"}); err != nil {
		t.Errorf("CheckLegacy with no MEM_* set = %v, want nil", err)
	}
}

func TestCheckLegacyReportsMapping(t *testing.T) {
	err := CheckLegacy([]string{
		"MEM_LITELLM_URL=http://old:4000",
		"MEM_QDRANT_ADDR=q:6334",
		"PATH=/bin",
	})
	if err == nil {
		t.Fatal("CheckLegacy with MEM_* set = nil, want error")
	}
	msg := err.Error()
	for _, want := range []string{
		"MEM_LITELLM_URL", "ENGRAM_OPENAI_BASE_URL",
		"MEM_QDRANT_ADDR", "ENGRAM_QDRANT_ADDR",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
}

func TestCheckLegacyCommandLocalVar(t *testing.T) {
	// Command-local legacy vars (not in the Config registry) are still caught.
	err := CheckLegacy([]string{"MEM_REINDEX_TARGET=foo"})
	if err == nil || !strings.Contains(err.Error(), "ENGRAM_REINDEX_TARGET") {
		t.Errorf("CheckLegacy(MEM_REINDEX_TARGET) = %v, want mapping to ENGRAM_REINDEX_TARGET", err)
	}
}
