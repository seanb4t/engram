// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package surfaces

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// proseTargets is the fixed docs-site/skill file set CONTEXT.md's D-05
// names as two of the six bound surfaces — the exact anchor-bearing prose
// trees internal/surfacesgen's ruleTargets table writes into (verified
// against that table's paths).
var proseTargets = []struct {
	path    string
	surface Surface
}{
	{"../../docs-site/src/content/docs/reference/tools.md", SurfaceDocsSite},
	{"../../docs-site/src/content/docs/guides/cli.md", SurfaceDocsSite},
	{"../../skill/engram/skills/curating-memory/SKILL.md", SurfaceSkill},
	{"../../skill/engram/skills/discovering/SKILL.md", SurfaceSkill},
}

// protoDir is the directory `go tool buf build` reads; protoFile is the
// one file this phase anchors rule regions into.
const (
	protoDir  = "../../proto"
	protoFile = "../../proto/engram/v1/engram.proto"
)

// exposedFileFields returns the subset of fields (original case
// preserved) that appear — in either snake_case or --kebab-case form —
// anywhere in path's raw content. This is prose's analog of a jsonschema
// tag: a file that never mentions a field cannot be documenting a rule
// that names it. It is a real read of the live file, never a declared
// per-rule list — D-08's applicability derivation applies to prose
// surfaces exactly as it does to schema-backed ones.
func exposedFileFields(t *testing.T, path string, fields []string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	content := string(data)
	var exposed []string
	for _, f := range fields {
		nf := NormalizeField(f)
		kebab := strings.ReplaceAll(nf, "_", "-")
		if strings.Contains(content, nf) || strings.Contains(content, kebab) {
			exposed = append(exposed, f)
		}
	}
	return exposed
}

// buildProseExposed derives rule's exposed field set on the docs-site,
// skill, and proto-comment surfaces from the REAL prose/proto trees on
// disk — never a declared per-rule list — so ApplicableSurfaces resolves
// applicability from what these files actually mention.
func buildProseExposed(t *testing.T, rule ConditionalRule) map[Surface][]string {
	t.Helper()
	exposed := map[Surface][]string{}
	for _, target := range proseTargets {
		exposed[target.surface] = append(exposed[target.surface], exposedFileFields(t, target.path, rule.Fields)...)
	}
	exposed[SurfaceProtoComment] = exposedFileFields(t, protoFile, rule.Fields)
	return exposed
}

// assertRuleAppliesSomewhere is the check every real gate assertion in
// this phase relies on: a declared rule resolving to ZERO applicable
// surfaces across all six bound surfaces is exactly the "silently passes
// nowhere" failure D-08 exists to prevent. It returns an error (never
// calling t.Fatal/t.Error directly) so TestZeroApplicableSurfacesFailsGate
// can assert the FAILURE MODE itself, exercising the identical code path
// the real per-rule gate calls first.
func assertRuleAppliesSomewhere(rule ConditionalRule, applicable []Surface) error {
	if len(applicable) == 0 {
		return fmt.Errorf("rule %q: ApplicableSurfaces returned zero surfaces across all six bound surfaces — a rule that applies nowhere must fail the gate, not pass silently", rule.ID)
	}
	return nil
}

// violationLine formats one gate divergence deterministically: rule ID,
// surface, the expected canonical sentence, and what was actually found.
// Every caller in this phase's three conformance-test files formats a
// divergence through a function shaped exactly like this one, so two
// consecutive runs against the same state produce byte-identical output.
func violationLine(ruleID string, surface Surface, expected, found string) string {
	return fmt.Sprintf("rule=%s surface=%s expected=%q found=%q", ruleID, surface, expected, found)
}

// checkProseSurface asserts every path's FIRST anchored region for rule
// (if any) is an EXACT match for rule.Sentence — region equality, not a
// substring search over the whole file, so one rule's text cannot satisfy
// another rule's check (this is why plan 02-01's non-containment
// validation and this comparison are complementary, not redundant). A
// path with no anchor for this rule is not itself a violation (a rule
// need not be anchored in every file under a surface); finding NO anchor
// in ANY of paths is — that is the gate silently applying nowhere.
func checkProseSurface(paths []string, rule ConditionalRule, surface Surface) []string {
	var violations []string
	found := 0
	for _, path := range paths {
		body, ok, err := ReadRegion(path, rule.ID)
		if err != nil {
			violations = append(violations, violationLine(rule.ID, surface, rule.Sentence, "error: "+err.Error()))
			continue
		}
		if !ok {
			continue
		}
		found++
		if body != rule.Sentence {
			violations = append(violations, violationLine(rule.ID, surface, rule.Sentence, body))
		}
	}
	if found == 0 {
		violations = append(violations, violationLine(rule.ID, surface, rule.Sentence, "no anchored region found in any file under this surface"))
	}
	return violations
}

// checkProtoSurface asserts, for every proto message field whose
// (normalized) name is one of rule.Fields and carries a leading comment,
// that comment CONTAINS rule.Sentence (not exact equality — a field's
// leading comment legitimately carries prose ahead of the anchored
// sentence, e.g. cross_spine's "spans every scope..." explanation). At
// least one matching field must be found, or the check itself reports a
// violation — the gate silently checking zero fields is the same
// no-op failure mode assertRuleAppliesSomewhere guards against one layer
// up.
func checkProtoSurface(rule ConditionalRule, comments map[string]string) []string {
	ruleFields := make(map[string]bool, len(rule.Fields))
	for _, f := range rule.Fields {
		ruleFields[NormalizeField(f)] = true
	}

	var violations []string
	matched := 0
	for key, comment := range comments {
		parts := strings.SplitN(key, ".", 2)
		if len(parts) != 2 || !ruleFields[NormalizeField(parts[1])] {
			continue
		}
		matched++
		if !strings.Contains(comment, rule.Sentence) {
			violations = append(violations, violationLine(rule.ID, SurfaceProtoComment, rule.Sentence, comment))
		}
	}
	if matched == 0 {
		violations = append(violations, violationLine(rule.ID, SurfaceProtoComment, rule.Sentence, "no proto field comment matched this rule's fields"))
	}
	return violations
}

// runGate runs the full docs-site/skill/proto-comment gate against the
// live tree, in Rules()/AllSurfaces() declaration order, and returns every
// divergence as a deterministic, formatted line. It is the single
// production code path both TestSurfaceConformanceProseFiles (the real
// gate) and TestSurfaceConformanceDeterministic (the determinism proof)
// call — never a parallel copy.
func runGate(t *testing.T) []string {
	t.Helper()

	fdsPath := filepath.Join(t.TempDir(), "engram-fds.binpb")
	if err := BufDescriptorSet(protoDir, fdsPath); err != nil {
		t.Fatalf("BufDescriptorSet (go tool buf build) failed — the proto-comment surface cannot be read: %v", err)
	}
	comments, err := ProtoFieldComments(fdsPath)
	if err != nil {
		t.Fatalf("ProtoFieldComments: %v", err)
	}

	var violations []string
	for _, rule := range Rules() {
		exposed := buildProseExposed(t, rule)
		applicable := ApplicableSurfaces(rule, exposed)
		if err := assertRuleAppliesSomewhere(rule, applicable); err != nil {
			violations = append(violations, err.Error())
			continue
		}
		for _, surface := range AllSurfaces() {
			if !containsSurface(applicable, surface) {
				continue
			}
			switch surface {
			case SurfaceDocsSite, SurfaceSkill:
				var paths []string
				for _, target := range proseTargets {
					if target.surface == surface {
						paths = append(paths, target.path)
					}
				}
				violations = append(violations, checkProseSurface(paths, rule, surface)...)
			case SurfaceProtoComment:
				violations = append(violations, checkProtoSurface(rule, comments)...)
			case SurfaceCobraUsage, SurfaceJSONSchemaTag, SurfaceMCPDescription:
				// Covered by internal/server/surfaces_test.go and
				// cmd/engram/surfaces_test.go — this package cannot import
				// either without creating an import cycle or crossing
				// TestClientFilesImportBoundary's denylist.
			}
		}
	}
	return violations
}

// TestSurfaceConformanceProseFiles is D-05's docs-site/skill/proto-comment
// gate: every declared rule's canonical Sentence, referenced from the
// registry (never retyped here), must be the exact content of its
// anchored region in every docs-site/skill file that mentions its fields,
// and must appear in the corresponding proto field's leading comment.
func TestSurfaceConformanceProseFiles(t *testing.T) {
	for _, v := range runGate(t) {
		t.Error(v)
	}
}

// TestSurfaceConformanceDeterministic proves the gate's failure output is
// deterministic: running the identical checkProseSurface code path twice
// against the SAME synthetic divergence (a fixture file whose anchored
// region has been corrupted) produces byte-identical violation lines.
func TestSurfaceConformanceDeterministic(t *testing.T) {
	rule, ok := RuleByID(RuleScopeRequiredUnlessCrossSpine)
	if !ok {
		t.Fatal("RuleScopeRequiredUnlessCrossSpine not found in registry")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.md")
	corrupted := "before\n<!-- engram:rule:start " + rule.ID + " -->WRONG TEXT<!-- engram:rule:end " + rule.ID + " -->\nafter\n"
	if err := os.WriteFile(path, []byte(corrupted), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	run := func() []string { return checkProseSurface([]string{path}, rule, SurfaceDocsSite) }
	first := run()
	second := run()

	if len(first) == 0 {
		t.Fatal("expected at least one violation from the synthetic divergence, got none")
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("gate output not deterministic across two runs:\nfirst:  %v\nsecond: %v", first, second)
	}
}

// TestZeroApplicableSurfacesFailsGate constructs a SYNTHETIC rule naming a
// field present on no surface and proves assertRuleAppliesSomewhere — the
// exact helper the real gate calls for every rule — reports a failure. A
// rule that applies nowhere passing silently is this gate's worst failure
// mode; this test proves it cannot happen, exercising the real code path
// rather than a parallel copy.
func TestZeroApplicableSurfacesFailsGate(t *testing.T) {
	ghostRule := ConditionalRule{
		ID:       "test-ghost-zero-surfaces",
		Sentence: "ghost_field_zzz is required unless never_true",
		Fields:   []string{"ghost_field_zzz"},
		Hint:     "conditional_required",
	}
	applicable := ApplicableSurfaces(ghostRule, map[Surface][]string{})
	if err := assertRuleAppliesSomewhere(ghostRule, applicable); err == nil {
		t.Fatal("assertRuleAppliesSomewhere(ghost rule) = nil, want a failure — a rule applying nowhere must fail the gate, not pass")
	}
}
