// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import (
	"bufio"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// findGoMod, modulePath, and nonTestGoFiles are copied verbatim from
// internal/setup/leafpurity_test.go and retargeted at this package —
// the same source-of-truth-not-hardcoded-module-path and
// scan-matching-nothing-is-not-evidence-of-purity discipline applies
// here unchanged.

// findGoMod walks up from dir looking for go.mod, so this gate reads the
// module path from its actual source of truth rather than hardcoding it —
// a module rename cannot silently disable this check.
func findGoMod(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(abs, "go.mod")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("no go.mod found walking up from %s", dir)
		}
		abs = parent
	}
}

// modulePath reads the `module` directive out of go.mod.
func modulePath(t *testing.T) string {
	t.Helper()
	path, err := findGoMod(".")
	if err != nil {
		t.Fatalf("locate go.mod: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if after, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(after)
		}
	}
	t.Fatalf("%s has no `module` directive", path)
	return ""
}

// nonTestGoFiles returns every non-test .go file directly in dir.
func nonTestGoFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		out = append(out, filepath.Join(dir, name))
	}
	return out
}

// skillsThirdPartyAllowlist names every third-party (non-stdlib,
// non-same-module) import path internal/skills may use. This is the
// package's ENTIRE third-party budget: exactly one entry,
// "go.yaml.in/yaml/v3", added in 04-02 for frontmatter.go's SKILL.md
// parsing (D-14's summary-line extraction). The same-module import ban
// below is UNCHANGED and is the property that actually forces the
// declare/install split (D-05) — this allowlist only narrows the
// third-party ban from "none" (04-01) to "exactly this one, audited,
// already-resolved entry". Any second entry is a new decision requiring
// its own justification in a future plan, never a mechanical edit to
// this map.
//
// Resolution of 04-CONTEXT.md's "Claude's Discretion" leaf-purity
// question: internal/skills is deliberately NOT held to
// internal/setup's stdlib-only rule, because the locked decision to
// parse SKILL.md frontmatter with a real YAML library (04-RESEARCH.md
// Open Question 2, resolved to go.yaml.in/yaml/v3) forecloses a
// stdlib-only posture for whichever package does that parsing — and
// splitting this package in two just to preserve a stdlib-only half
// for the non-parsing code would be ceremony around a single import.
// The property that actually matters, and that THIS gate keeps
// enforced verbatim, is the SAME-MODULE import ban: it is what makes
// the declare/install split (D-05) one-directional and keeps
// internal/setup structurally unable to reach internal/skills. The
// third-party ban is narrowed from "none" to "exactly what this
// allowlist names", so dependency creep still fails the build while
// the one audited, already-resolved parser is permitted. The tradeoff,
// stated plainly: the skills write path is no longer provably
// third-party-free from this gate alone — the allowlist is the
// mechanism that bounds it.
var skillsThirdPartyAllowlist = map[string]bool{
	"go.yaml.in/yaml/v3": true,
}

// TestSkillsPackageImportsAreGated asserts two properties over every
// non-test Go file in this package: no import path is prefixed by this
// module's own path (the one-directional D-05 split), and every
// non-standard-library import path appears in skillsThirdPartyAllowlist
// above.
func TestSkillsPackageImportsAreGated(t *testing.T) {
	files := nonTestGoFiles(t, ".")
	if len(files) == 0 {
		t.Fatal("scanned zero non-test .go files in internal/skills — a scan matching nothing is vacuously green, which is a defect in the scan, not evidence the package is pure")
	}

	mod := modulePath(t)

	type offender struct {
		file       string
		importPath string
	}

	fset := token.NewFileSet()
	var allImports []string
	var sameModule []offender
	var notAllowlisted []offender

	for _, path := range files {
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, imp := range f.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("%s: unquote import literal %s: %v", path, imp.Path.Value, err)
			}
			allImports = append(allImports, importPath)

			if strings.HasPrefix(importPath, mod) {
				sameModule = append(sameModule, offender{path, importPath})
				continue
			}

			firstSeg, _, _ := strings.Cut(importPath, "/")
			isThirdParty := strings.Contains(firstSeg, ".")
			if isThirdParty && !skillsThirdPartyAllowlist[importPath] {
				notAllowlisted = append(notAllowlisted, offender{path, importPath})
			}
		}
	}

	if len(sameModule) > 0 {
		t.Fatalf("internal/skills imports from this module: %+v — the declare/install split is one-directional (D-05); internal/setup must never be reachable from here. Full collected import set: %v", sameModule, allImports)
	}
	if len(notAllowlisted) > 0 {
		t.Fatalf("internal/skills imports third-party package(s) absent from skillsThirdPartyAllowlist: %+v — add exactly the audited entry if this is an intentional, reviewed addition. Full collected import set: %v", notAllowlisted, allImports)
	}
}
