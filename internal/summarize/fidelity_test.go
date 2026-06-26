// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package summarize

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
)

// fidelityCase pairs realistic caveat-heavy content with tokens the summary MUST
// preserve (negations, identifiers, numbers). Expand with anonymized real memories.
type fidelityCase struct {
	name        string
	content     string
	mustContain []string
}

var fidelityCases = []fidelityCase{
	{
		name:        "decline-suggestion",
		content:     "The #_top selector beats the h1 rules directly. Review bots suggest `#_top h1` which matches NOTHING and would strip the styling. DECLINE that suggestion.",
		mustContain: []string{"DECLINE", "#_top"},
	},
	{
		name:        "dep-type-flag",
		content:     "bd dep add <epic> <decision> with the default blocks type is REJECTED. Use --type related for the provenance edge, never blocks.",
		mustContain: []string{"--type related", "never", "blocks"},
	},
}

// TestSummaryFidelity runs the configured cheap model over the cases and reports
// a preservation score. Gate: set ENGRAM_SUMMARY_EVAL=1 plus ENGRAM_OPENAI_BASE_URL,
// ENGRAM_OPENAI_API_KEY, ENGRAM_SUMMARY_MODEL, ENGRAM_SUMMARY_MAX_CHARS.
func TestSummaryFidelity(t *testing.T) {
	if os.Getenv("ENGRAM_SUMMARY_EVAL") != "1" {
		t.Skip("set ENGRAM_SUMMARY_EVAL=1 (and the gateway/model env) to run the fidelity eval")
	}
	maxChars, _ := strconv.Atoi(os.Getenv("ENGRAM_SUMMARY_MAX_CHARS"))
	if maxChars <= 0 {
		maxChars = 280
	}
	c := New(os.Getenv("ENGRAM_OPENAI_BASE_URL"), os.Getenv("ENGRAM_OPENAI_API_KEY"), os.Getenv("ENGRAM_SUMMARY_MODEL"), maxChars)

	var checks, passed int
	for _, tc := range fidelityCases {
		sum, err := c.Summarize(context.Background(), tc.content)
		if err != nil {
			t.Errorf("%s: summarize error: %v", tc.name, err)
			continue
		}
		for _, tok := range tc.mustContain {
			checks++
			if strings.Contains(sum, tok) {
				passed++
			} else {
				t.Errorf("%s: summary dropped %q\n  summary: %s", tc.name, tok, sum)
			}
		}
	}
	t.Logf("fidelity: %d/%d required tokens preserved", passed, checks)
}
