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
	"strconv"
	"strings"

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
	// The four rules 02-03-PLAN.md's Task 1 declares. Each is anchored only
	// where its fields are genuinely advertised (D-08) — schedule_memory has
	// no CLI verb, so none of the three schedule-only rules below carry a
	// cobra/client_*.go composition or a cli.md anchor; the paging rule has
	// no MCP jsonschema-tag counterpart, so it carries no anchor on any
	// MCP-only surface.
	surfaces.RuleScheduleWindowAtLeastOne: {
		{path: "docs-site/src/content/docs/reference/tools.md", kind: kindMarkdown},
		{path: "skill/engram/skills/curating-memory/SKILL.md", kind: kindMarkdown},
		{path: "proto/engram/v1/engram.proto", kind: kindProto},
	},
	surfaces.RuleDiscoveryNotSchedulable: {
		{path: "docs-site/src/content/docs/reference/tools.md", kind: kindMarkdown},
		{path: "skill/engram/skills/curating-memory/SKILL.md", kind: kindMarkdown},
		{path: "proto/engram/v1/engram.proto", kind: kindProto},
	},
	surfaces.RuleWindowOrdering: {
		{path: "docs-site/src/content/docs/reference/tools.md", kind: kindMarkdown},
		{path: "skill/engram/skills/curating-memory/SKILL.md", kind: kindMarkdown},
		{path: "proto/engram/v1/engram.proto", kind: kindProto},
	},
	// RulePagingMutuallyExclusive: only the two surfaces that actually
	// advertise cursor_mode/offset/page_token as fields (D-08) —
	// ListMemoriesRequest's proto comment and cli.md's new Paging section.
	// Not docs-site/reference/tools.md (that page documents MCP tool
	// arguments; list_memory's MCP arg struct carries none of these three
	// fields, only the single unified `cursor`) and not either skill file
	// (neither mentions offset/page_token/cursor_mode by name today).
	surfaces.RulePagingMutuallyExclusive: {
		{path: "docs-site/src/content/docs/guides/cli.md", kind: kindMarkdown},
		{path: "proto/engram/v1/engram.proto", kind: kindProto},
	},
	// RuleDestructiveRequiresApply: no proto anchor — "apply" is not a proto
	// field on any message (03-03-PLAN.md Task 2), so the proto-comment
	// surface correctly resolves not-applicable rather than getting an
	// anchor with nothing to attach to. Anchored on cli.md (the CLI guide's
	// new destructive-commands section) and curating-memory's SKILL.md
	// (where an agent already learns the deletion contract).
	surfaces.RuleDestructiveRequiresApply: {
		{path: "docs-site/src/content/docs/guides/cli.md", kind: kindMarkdown},
		{path: "skill/engram/skills/curating-memory/SKILL.md", kind: kindMarkdown},
	},
	// RuleVerifyFailOnValues: no proto anchor and no skill anchor -- "fail-on"
	// is not a proto field on any message, and neither skill file mentions it
	// by name. Anchored only on cli.md's new spine-review section, the one
	// place this flag is actually documented (03-04-PLAN.md Task 3).
	surfaces.RuleVerifyFailOnValues: {
		{path: "docs-site/src/content/docs/guides/cli.md", kind: kindMarkdown},
	},
	// RulePurgeFilterRequiresScope: no proto anchor -- "class"/"older-than"
	// (purge's own flags) are not proto fields on any message. Anchored on
	// all THREE prose surfaces the live tree measures as exposing
	// scope+category+tags+older-than together (03-07-PLAN.md Task 3): the
	// CLI guide's own purge subsection, the tools reference (where the
	// filter vocabulary is published), and curating-memory's SKILL.md
	// (where an agent learns the deletion contract).
	surfaces.RulePurgeFilterRequiresScope: {
		{path: "docs-site/src/content/docs/guides/cli.md", kind: kindMarkdown},
		{path: "docs-site/src/content/docs/reference/tools.md", kind: kindMarkdown},
		{path: "skill/engram/skills/curating-memory/SKILL.md", kind: kindMarkdown},
	},
	// RuleSweepScopeOrAllScopesRequired: no proto anchor -- all_scopes is not
	// a proto field on any message. No skill anchor -- neither SKILL.md
	// mentions this rule's SurfaceFields (specifically dry-run, the field
	// that narrows cobra_usage resolution to summarize-missing alone).
	// No guides/cli.md anchor either, even though it is a SurfaceDocsSite
	// target file: the narrowed applicability set (Fields plus dry-run)
	// resolves cobra_usage to summarize-missing ONLY, and cli.md documents
	// none of the three sweep leaves' scope/all-scopes contract in a way
	// that names this rule -- checkProseSurface requires just one anchored
	// file per applicable surface (08-01-PLAN.md fact 6), so tools.md alone
	// satisfies SurfaceDocsSite.
	surfaces.RuleSweepScopeOrAllScopesRequired: {
		{path: "docs-site/src/content/docs/reference/tools.md", kind: kindMarkdown},
	},
}

// render returns rule sentence's on-disk form for the given surface kind.
func render(sentence string, kind surfaceKind) string {
	if kind == kindProto {
		return "// " + sentence
	}
	return sentence
}

// toolBlastRadiusPath is the one prose file carrying the tool-blast-radius
// region (02-04-PLAN.md Task 2). It reuses the same
// engram:rule:start/engram:rule:end anchor machinery ruleTargets' regions
// do, under a distinct region id that is not a surfaces.ConditionalRule —
// WriteRegion/ReadRegion key on an arbitrary string, not exclusively a
// declared rule ID.
const toolBlastRadiusPath = "docs-site/src/content/docs/reference/tools.md"

const toolBlastRadiusRegionID = "tool-blast-radius"

// renderToolBlastRadius renders the markdown table body for the
// tool-blast-radius region: one row per MCP tool in surfaces.Operations()
// declaration order, listing all four blast-radius hints.
func renderToolBlastRadius() string {
	var b strings.Builder
	b.WriteString("| Tool | `readOnlyHint` | `destructiveHint` | `idempotentHint` | `openWorldHint` |\n")
	b.WriteString("|------|----------------|--------------------|-------------------|------------------|\n")
	for _, op := range surfaces.Operations() {
		if op.MCPTool == "" {
			continue
		}
		b.WriteString("| `" + op.MCPTool + "` | " +
			strconv.FormatBool(op.Class.ReadOnly) + " | " +
			strconv.FormatBool(op.Class.Destructive) + " | " +
			strconv.FormatBool(op.Class.Idempotent) + " | " +
			strconv.FormatBool(op.Class.OpenWorld) + " |\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// run regenerates every anchored region ruleTargets names, in
// surfaces.Rules() declaration order so output is stable across runs, then
// regenerates the tool-blast-radius region from surfaces.Operations().
func run() error {
	if err := surfaces.ValidateRules(); err != nil {
		return fmt.Errorf("surfacesgen: registry invalid: %w", err)
	}
	if err := surfaces.ValidateOperations(); err != nil {
		return fmt.Errorf("surfacesgen: operations registry invalid: %w", err)
	}
	for _, rule := range surfaces.Rules() {
		for _, t := range ruleTargets[rule.ID] {
			body := render(rule.Sentence, t.kind)
			if err := surfaces.WriteRegion(t.path, rule.ID, body); err != nil {
				return fmt.Errorf("surfacesgen: rule %q: %w", rule.ID, err)
			}
		}
	}
	if err := surfaces.WriteRegion(toolBlastRadiusPath, toolBlastRadiusRegionID, renderToolBlastRadius()); err != nil {
		return fmt.Errorf("surfacesgen: tool-blast-radius: %w", err)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
