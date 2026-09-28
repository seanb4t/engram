// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file is the D-01 docs gate for query understanding (milestone
// 2026-09-25.01 Phase 6): the docs-site config reference must document every
// ENGRAM_SEARCH_UNDERSTANDING* var the registry actually carries, and must
// disclose the default-follows-provider rule, what a query-understanding
// call sends, how it fails, and the explicit off switch, before an operator
// deploys with a decisions provider configured. The env set is derived from
// the registry (never a second hand-maintained list), so a registry addition
// without a matching docs row fails loudly here instead of shipping silently
// undocumented. Structurally mirrors search_docs_test.go, minus its
// synthetic red-control subtest (rule 3p0zsqrhmb: no tests of tests) — this
// gate was observed red once against a temporary, uncommitted docs edit; see
// 06-06-SUMMARY.md.

package config

import (
	"os"
	"strings"
	"testing"
)

// understandingConfigureDocPath mirrors searchConfigureDocPath.
const understandingConfigureDocPath = "../../docs-site/src/content/docs/guides/configure.md"

// understandingSectionHeading and understandingNextHeadingPrefix bound the
// section this gate reads: everything from just after the heading line to
// the next "## " heading (or end of file if there is none).
const understandingSectionHeading = "## Query understanding (Jev)"

const understandingNextHeadingPrefix = "\n## "

// understandingRegistryEnvNames returns every registry Env carrying the
// ENGRAM_SEARCH_UNDERSTANDING prefix, in registry order — the complement of
// searchRegistryEnvNames' exclusion.
func understandingRegistryEnvNames() []string {
	var envs []string
	for _, r := range registry {
		if !strings.HasPrefix(r.Env, "ENGRAM_SEARCH_UNDERSTANDING") {
			continue
		}
		envs = append(envs, r.Env)
	}
	return envs
}

// extractUnderstandingSection returns the body of doc's
// "## Query understanding (Jev)" section, or "" with ok=false if the heading
// is absent.
func extractUnderstandingSection(doc string) (section string, ok bool) {
	idx := strings.Index(doc, understandingSectionHeading)
	if idx < 0 {
		return "", false
	}
	rest := doc[idx+len(understandingSectionHeading):]
	if next := strings.Index(rest, understandingNextHeadingPrefix); next >= 0 {
		return rest[:next], true
	}
	return rest, true
}

// missingUnderstandingDocs reports every env in envs that has no table row
// starting "| `<ENV>` |" inside section.
func missingUnderstandingDocs(section string, envs []string) []string {
	var missing []string
	for _, env := range envs {
		marker := "| `" + env + "` |"
		if !strings.Contains(section, marker) {
			missing = append(missing, env)
		}
	}
	return missing
}

// TestUnderstandingVarsDocumented is the D-01 gate: the config reference
// documents every registered ENGRAM_SEARCH_UNDERSTANDING* var and carries
// the disclosure anchors an operator needs before deploying with a decisions
// provider configured.
func TestUnderstandingVarsDocumented(t *testing.T) {
	envs := understandingRegistryEnvNames()
	if len(envs) != 3 {
		t.Fatalf("understandingRegistryEnvNames() returned %d names, want 3 (positive control -- an empty or short derivation must not pass vacuously): %v", len(envs), envs)
	}

	data, err := os.ReadFile(understandingConfigureDocPath)
	if err != nil {
		t.Fatalf("read %s: %v", understandingConfigureDocPath, err)
	}
	doc := string(data)

	section, ok := extractUnderstandingSection(doc)
	if !ok {
		t.Fatalf("%s: no %q heading found", understandingConfigureDocPath, understandingSectionHeading)
	}

	t.Run("every registered var has a doc row", func(t *testing.T) {
		if missing := missingUnderstandingDocs(section, envs); len(missing) > 0 {
			t.Errorf("%s: %q section is missing doc rows for: %v", understandingConfigureDocPath, understandingSectionHeading, missing)
		}
	})

	t.Run("disclosure anchors present", func(t *testing.T) {
		for _, anchor := range []string{
			"What leaves your deployment",
			"zero suggestions",
			"ENGRAM_SEARCH_UNDERSTANDING=off",
			"search understanding enabled: console query text is sent to",
		} {
			if !strings.Contains(section, anchor) {
				t.Errorf("%s: %q section is missing disclosure anchor %q", understandingConfigureDocPath, understandingSectionHeading, anchor)
			}
		}
	})
}
