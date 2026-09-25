// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file is the D-01/D-09 docs gate: the docs-site config reference must
// document every ENGRAM_SEARCH_* var the registry actually carries, and must
// disclose what a search reranking call sends and how it fails, before an
// operator enables the feature. The env set is derived from the registry
// (never a second hand-maintained list), so a registry addition without a
// matching docs row fails loudly here instead of shipping silently
// undocumented. Mirrors decisions_docs_test.go's registry-driven shape.

package config

import (
	"os"
	"strings"
	"testing"
)

// searchConfigureDocPath mirrors decisionsConfigureDocPath.
const searchConfigureDocPath = "../../docs-site/src/content/docs/guides/configure.md"

// searchSectionHeading and searchNextHeadingPrefix bound the section this
// gate reads: everything from just after the heading line to the next "## "
// heading (or end of file if there is none).
const searchSectionHeading = "## Search reranking (Jev)"

const searchNextHeadingPrefix = "\n## "

// searchRegistryEnvNames returns every registry Env carrying the
// ENGRAM_SEARCH_ prefix, in registry order.
func searchRegistryEnvNames() []string {
	var envs []string
	for _, r := range registry {
		if strings.HasPrefix(r.Env, "ENGRAM_SEARCH_") {
			envs = append(envs, r.Env)
		}
	}
	return envs
}

// extractSearchSection returns the body of doc's "## Search reranking (Jev)"
// section, or "" with ok=false if the heading is absent.
func extractSearchSection(doc string) (section string, ok bool) {
	idx := strings.Index(doc, searchSectionHeading)
	if idx < 0 {
		return "", false
	}
	rest := doc[idx+len(searchSectionHeading):]
	if next := strings.Index(rest, searchNextHeadingPrefix); next >= 0 {
		return rest[:next], true
	}
	return rest, true
}

// missingSearchDocs reports every env in envs that has no table row starting
// "| `<ENV>` |" inside section.
func missingSearchDocs(section string, envs []string) []string {
	var missing []string
	for _, env := range envs {
		marker := "| `" + env + "` |"
		if !strings.Contains(section, marker) {
			missing = append(missing, env)
		}
	}
	return missing
}

// TestSearchVarsDocumented is the D-01/D-09 gate: the config reference
// documents every registered ENGRAM_SEARCH_* var and carries the disclosure
// anchors an operator needs before enabling search-path reranking.
func TestSearchVarsDocumented(t *testing.T) {
	envs := searchRegistryEnvNames()
	if len(envs) != 3 {
		t.Fatalf("searchRegistryEnvNames() returned %d names, want 3 (positive control -- an empty or short derivation must not pass vacuously): %v", len(envs), envs)
	}

	t.Run("red control: missingSearchDocs catches an omitted var", func(t *testing.T) {
		synthetic := "\n" +
			"| Environment variable | Flag | Default | Description |\n" +
			"|---------------------|------|---------|-------------|\n" +
			"| `ENGRAM_SEARCH_RANKER` | — | `lexical` | ... |\n"
		// deliberately omits ENGRAM_SEARCH_RERANK_TIMEOUT and ENGRAM_SEARCH_RERANK_AUDIT
		got := missingSearchDocs(synthetic, envs)
		want := []string{"ENGRAM_SEARCH_RERANK_TIMEOUT", "ENGRAM_SEARCH_RERANK_AUDIT"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("missingSearchDocs() = %v, want %v (proves this gate can go red)", got, want)
		}
	})

	data, err := os.ReadFile(searchConfigureDocPath)
	if err != nil {
		t.Fatalf("read %s: %v", searchConfigureDocPath, err)
	}
	doc := string(data)

	section, ok := extractSearchSection(doc)
	if !ok {
		t.Fatalf("%s: no %q heading found", searchConfigureDocPath, searchSectionHeading)
	}

	t.Run("every registered var has a doc row", func(t *testing.T) {
		if missing := missingSearchDocs(section, envs); len(missing) > 0 {
			t.Errorf("%s: %q section is missing doc rows for: %v", searchConfigureDocPath, searchSectionHeading, missing)
		}
	})

	t.Run("disclosure anchors present", func(t *testing.T) {
		for _, anchor := range []string{"What leaves your deployment", "falls back"} {
			if !strings.Contains(section, anchor) {
				t.Errorf("%s: %q section is missing disclosure anchor %q", searchConfigureDocPath, searchSectionHeading, anchor)
			}
		}
	})
}
