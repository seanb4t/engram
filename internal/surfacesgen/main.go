// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Command surfacesgen regenerates every anchored interface-surface region
// from the internal/surfaces rule registry — the single generator
// `task surfaces:gen` runs (D-06/D-07). Its only content source is
// surfaces.Rules(): it reads no environment variable, no flag, and no file
// content other than the anchored target files it rewrites, so nothing a
// caller supplies at generation time can reach a published surface.
package main

import (
	"fmt"
	"os"

	"github.com/seanb4t/engram/internal/surfaces"
)

// surfaceKind selects how a rule's canonical Sentence is rendered into an
// anchored region.
type surfaceKind int

const (
	// kindMarkdown renders the bare canonical sentence as a single line —
	// the anchor pair itself (HTML comments) carries the comment syntax, so
	// the body between them is plain prose text.
	kindMarkdown surfaceKind = iota
	// kindProto renders the canonical sentence prefixed with "// ", since a
	// .proto anchor pair (`//` line comments) brackets a region whose body
	// must itself be valid proto source — a bare sentence with no comment
	// marker would not compile.
	kindProto
)

// target names one file carrying a rule's anchor pair, and how to render
// that rule's Sentence for this file's surrounding syntax.
type target struct {
	path string
	kind surfaceKind
}

// ruleTargets maps each declared rule ID to every anchored file carrying its
// region. Extending this table — never adding a second generator — is how
// later plans widen coverage to the rest of D-05's six bound surfaces;
// D-07's whole point is that contributors learn one pattern and one command
// follows any interface change.
var ruleTargets = map[string][]target{
	surfaces.RuleScopeRequiredUnlessCrossSpine: {
		{path: "docs-site/src/content/docs/reference/tools.md", kind: kindMarkdown},
		{path: "docs-site/src/content/docs/guides/cli.md", kind: kindMarkdown},
		{path: "skill/engram/skills/curating-memory/SKILL.md", kind: kindMarkdown},
		{path: "skill/engram/skills/discovering/SKILL.md", kind: kindMarkdown},
		// proto/engram/v1/engram.proto carries this rule's anchors TWICE —
		// once on ListMemoriesRequest.cross_spine, once on
		// SearchMemoriesRequest.cross_spine — so this one path entry drives
		// WriteRegion rewriting both occurrences (it rewrites every
		// well-formed anchor pair it finds for a rule, not just the first).
		{path: "proto/engram/v1/engram.proto", kind: kindProto},
	},
}

// render returns rule sentence's on-disk form for the given surface kind.
func render(sentence string, kind surfaceKind) string {
	if kind == kindProto {
		return "// " + sentence
	}
	return sentence
}

// run regenerates every anchored region ruleTargets names, in
// surfaces.Rules() declaration order so output is stable across runs.
func run() error {
	if err := surfaces.ValidateRules(); err != nil {
		return fmt.Errorf("surfacesgen: registry invalid: %w", err)
	}
	for _, rule := range surfaces.Rules() {
		for _, t := range ruleTargets[rule.ID] {
			body := render(rule.Sentence, t.kind)
			if err := surfaces.WriteRegion(t.path, rule.ID, body); err != nil {
				return fmt.Errorf("surfacesgen: rule %q: %w", rule.ID, err)
			}
		}
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
