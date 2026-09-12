// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

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

// TestSetupPackageIsStdlibOnlyLeaf mirrors
// internal/migrate/leafpurity_test.go's gate for this new package:
// internal/setup imports nothing but the Go standard library, and nothing
// from this module — it must not reach into internal/config,
// internal/store, or cmd/engram (its own doc comment's promised
// direction, made mechanical): cmd/engram/setup.go is this package's only
// consumer, never the reverse.
func TestSetupPackageIsStdlibOnlyLeaf(t *testing.T) {
	files := nonTestGoFiles(t, ".")
	if len(files) == 0 {
		t.Fatal("scanned zero non-test .go files in internal/setup — a scan matching nothing is vacuously green, which is a defect in the scan, not evidence the package is pure")
	}

	mod := modulePath(t)

	type offender struct {
		file       string
		importPath string
	}

	fset := token.NewFileSet()
	var allImports []string
	var nonStdlib []offender
	var sameModule []offender

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

			firstSeg, _, _ := strings.Cut(importPath, "/")
			if strings.Contains(firstSeg, ".") {
				nonStdlib = append(nonStdlib, offender{path, importPath})
			}
			if strings.HasPrefix(importPath, mod) {
				sameModule = append(sameModule, offender{path, importPath})
			}
		}
	}

	if len(nonStdlib) > 0 {
		t.Fatalf("internal/setup imports non-stdlib package(s): %+v — full collected import set: %v", nonStdlib, allImports)
	}
	if len(sameModule) > 0 {
		t.Fatalf("internal/setup imports from this module: %+v — the one-way import direction (cmd/engram imports internal/setup, never the reverse) is broken; full collected import set: %v", sameModule, allImports)
	}
}
