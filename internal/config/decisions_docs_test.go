// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file is the D-04/D-03/P4 docs gate: the docs-site config reference
// must document every ENGRAM_DECISIONS_* var the registry actually carries,
// and must disclose what a decisions call sends, to whom, and under what
// data policy, before an operator enables the feature. The env set is
// derived from the registry (never a second hand-maintained list), so a
// registry addition without a matching docs row fails loudly here instead
// of shipping silently undocumented.

package config

import (
	"os"
	"strings"
	"testing"
)

// decisionsConfigureDocPath is the config reference, relative to this
// package's own directory (internal/config). Two levels up reaches the repo
// root (internal/config -> internal -> repo root) -- `go test` always runs
// with the package directory as its working directory, so a plain relative
// read resolves correctly with no repo-root lookup needed. Mirrors
// internal/server/recallmaxdocs_test.go's own relative-path convention.
const decisionsConfigureDocPath = "../../docs-site/src/content/docs/guides/configure.md"

// decisionsSectionHeading and decisionsNextHeadingPrefix bound the section
// this gate reads: everything from just after the heading line to the next
// "## " heading (or end of file if there is none).
const decisionsSectionHeading = "## Typed decisions (Jev)"

const decisionsNextHeadingPrefix = "\n## "

// decisionsRegistryEnvNames returns every registry Env carrying the
// ENGRAM_DECISIONS_ prefix, in registry order.
func decisionsRegistryEnvNames() []string {
	var envs []string
	for _, r := range registry {
		if strings.HasPrefix(r.Env, "ENGRAM_DECISIONS_") {
			envs = append(envs, r.Env)
		}
	}
	return envs
}

// extractDecisionsSection returns the body of doc's "## Typed decisions
// (Jev)" section, or "" with ok=false if the heading is absent.
func extractDecisionsSection(doc string) (section string, ok bool) {
	idx := strings.Index(doc, decisionsSectionHeading)
	if idx < 0 {
		return "", false
	}
	rest := doc[idx+len(decisionsSectionHeading):]
	if next := strings.Index(rest, decisionsNextHeadingPrefix); next >= 0 {
		return rest[:next], true
	}
	return rest, true
}

// missingDecisionsDocs reports every env in envs that has no table row
// starting "| `<ENV>` |" inside section.
func missingDecisionsDocs(section string, envs []string) []string {
	var missing []string
	for _, env := range envs {
		marker := "| `" + env + "` |"
		if !strings.Contains(section, marker) {
			missing = append(missing, env)
		}
	}
	return missing
}

// TestDecisionsVarsDocumented is the D-04/D-03/P4 gate: the config
// reference documents every registered ENGRAM_DECISIONS_* var and carries
// the disclosure anchors an operator needs before enabling the feature.
func TestDecisionsVarsDocumented(t *testing.T) {
	envs := decisionsRegistryEnvNames()
	if len(envs) != 10 {
		t.Fatalf("decisionsRegistryEnvNames() returned %d names, want 10 (positive control -- an empty or short derivation must not pass vacuously): %v", len(envs), envs)
	}

	t.Run("red control: missingDecisionsDocs catches an omitted var", func(t *testing.T) {
		synthetic := "\n" +
			"| Environment variable | Flag | Default | Description |\n" +
			"|---------------------|------|---------|-------------|\n" +
			"| `ENGRAM_DECISIONS_PROVIDER` | — | _(empty)_ | ... |\n" +
			"| `ENGRAM_DECISIONS_BASE_URL` | — | _(empty)_ | ... |\n" +
			"| `ENGRAM_DECISIONS_API_KEY` | — | _(empty)_ | ... |\n" +
			"| `ENGRAM_DECISIONS_MODEL` | — | typesafe/jev-1.13 | ... |\n" +
			"| `ENGRAM_DECISIONS_TIMEOUT` | — | 10s | ... |\n" +
			"| `ENGRAM_DECISIONS_MAX_TIMEOUT` | — | 10m | ... |\n" +
			"| `ENGRAM_DECISIONS_DRAIN_BYTES` | — | 262144 | ... |\n" +
			"| `ENGRAM_DECISIONS_DRAIN_TIMEOUT` | — | 2s | ... |\n" +
			"| `ENGRAM_DECISIONS_VERDICT_THRESHOLD` | — | 0.9 | ... |\n"
		// deliberately omits ENGRAM_DECISIONS_CONCURRENCY
		got := missingDecisionsDocs(synthetic, envs)
		want := []string{"ENGRAM_DECISIONS_CONCURRENCY"}
		if len(got) != len(want) || (len(got) > 0 && got[0] != want[0]) {
			t.Fatalf("missingDecisionsDocs() = %v, want %v (proves this gate can go red)", got, want)
		}
	})

	data, err := os.ReadFile(decisionsConfigureDocPath)
	if err != nil {
		t.Fatalf("read %s: %v", decisionsConfigureDocPath, err)
	}
	doc := string(data)

	section, ok := extractDecisionsSection(doc)
	if !ok {
		t.Fatalf("%s: no %q heading found", decisionsConfigureDocPath, decisionsSectionHeading)
	}

	t.Run("every registered var has a doc row", func(t *testing.T) {
		if missing := missingDecisionsDocs(section, envs); len(missing) > 0 {
			t.Errorf("%s: %q section is missing doc rows for: %v", decisionsConfigureDocPath, decisionsSectionHeading, missing)
		}
	})

	t.Run("disclosure anchors present", func(t *testing.T) {
		for _, anchor := range []string{"TypeSafe", "retention", "ENGRAM_OPENAI_API_KEY", "/alpha/decisions"} {
			if !strings.Contains(section, anchor) {
				t.Errorf("%s: %q section is missing disclosure anchor %q", decisionsConfigureDocPath, decisionsSectionHeading, anchor)
			}
		}
	})
}
