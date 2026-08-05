// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package surfaces

import "strings"

// Surface identifies one of the six interface surfaces D-05 binds a
// declared conditional rule to.
type Surface string

const (
	// SurfaceCobraUsage is a cobra flag's Usage string (also the source
	// `engram catalog` and --help render from).
	SurfaceCobraUsage Surface = "cobra_usage"
	// SurfaceJSONSchemaTag is an MCP arg struct's `jsonschema:"..."` tag —
	// the one surface whose text is necessarily re-typed rather than
	// referenced, since Go struct tags are literal-only (D-03).
	SurfaceJSONSchemaTag Surface = "jsonschema_tag"
	// SurfaceMCPDescription is a registered mcp.Tool's Description prose.
	SurfaceMCPDescription Surface = "mcp_description"
	// SurfaceProtoComment is a proto field's leading comment.
	SurfaceProtoComment Surface = "proto_comment"
	// SurfaceDocsSite is the published docs-site markdown.
	SurfaceDocsSite Surface = "docs_site"
	// SurfaceSkill is the distributed skill/engram SKILL.md markdown.
	SurfaceSkill Surface = "skill"
)

// allSurfaces is the fixed declaration order AllSurfaces returns and
// ApplicableSurfaces iterates. This fixed order — not map-iteration order —
// is what makes surface resolution, and therefore gate failure output,
// deterministic across runs (D-05's determinism requirement).
var allSurfaces = []Surface{
	SurfaceCobraUsage,
	SurfaceJSONSchemaTag,
	SurfaceMCPDescription,
	SurfaceProtoComment,
	SurfaceDocsSite,
	SurfaceSkill,
}

// AllSurfaces returns a defensive copy of the six bound surfaces, in fixed
// declaration order.
func AllSurfaces() []Surface {
	out := make([]Surface, len(allSurfaces))
	copy(out, allSurfaces)
	return out
}

// NormalizeField reduces a field/flag name to its canonical comparison
// key: strip a leading "--", replace every "-" with "_", then lowercase.
// That is the whole transform — RESEARCH.md's naming table shows
// kebab-case cobra flags and snake_case JSON/proto field names collapse to
// the same key for every field in the inventory except the paging trio,
// which genuinely has no MCP counterpart and is deliberately NOT
// special-cased here (see ApplicableSurfaces' doc comment): forcing a
// `cursor` mapping for `offset`/`cursor_mode` would fabricate a surface
// that does not exist.
func NormalizeField(name string) string {
	name = strings.TrimPrefix(name, "--")
	name = strings.ReplaceAll(name, "-", "_")
	return strings.ToLower(name)
}

// ApplicableSurfaces returns the surfaces (in AllSurfaces order) where
// EVERY field rule names is present in that surface's normalized exposed
// field set. exposed is keyed by Surface and holds that surface's real,
// unnormalized field/flag names as observed on the live interface (a
// cobra flag name, a JSON/jsonschema tag name, a proto field name, ...);
// ApplicableSurfaces normalizes both the rule's fields and each surface's
// exposed fields before comparing existence — it never renames or
// special-cases a field to force a match.
//
// This existence check — not a blind rename — is what correctly excludes a
// surface that genuinely does not carry a rule's fields. D-08's worked
// example: a rule naming `cursor_mode` and `offset` resolves to an EMPTY
// set on the MCP jsonschema-tag surface (`listArgs` exposes neither field)
// while still resolving non-empty on cobra Usage and the proto comment —
// the rule truly doesn't apply to MCP jsonschema, so ApplicableSurfaces
// correctly drops it there rather than papering over the gap.
func ApplicableSurfaces(rule ConditionalRule, exposed map[Surface][]string) []Surface {
	var out []Surface
	for _, s := range allSurfaces {
		if surfaceExposesAllFields(exposed[s], rule.Fields) {
			out = append(out, s)
		}
	}
	return out
}

// surfaceExposesAllFields reports whether every field in ruleFields is
// present (after normalization) in surfaceFields.
func surfaceExposesAllFields(surfaceFields, ruleFields []string) bool {
	normalized := make(map[string]bool, len(surfaceFields))
	for _, f := range surfaceFields {
		normalized[NormalizeField(f)] = true
	}
	for _, f := range ruleFields {
		if !normalized[NormalizeField(f)] {
			return false
		}
	}
	return true
}
