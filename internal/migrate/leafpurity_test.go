// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package migrate

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

// TestMigratePackageIsStdlibOnlyLeaf is the in-repo, `go test ./...`-
// resident half of SC1: internal/migrate imports nothing but the Go
// standard library, and nothing from this module (the one-way import
// direction internal/migrate.go's package doc comment already promises,
// made mechanical). The acceptance criterion's `go list -deps` command is
// an independent, out-of-band cross-check — deliberately a separate
// derivation from this AST scan, not a duplicate of it.
func TestMigratePackageIsStdlibOnlyLeaf(t *testing.T) {
	files := nonTestGoFiles(t, ".")
	if len(files) == 0 {
		t.Fatal("scanned zero non-test .go files in internal/migrate — a scan matching nothing is vacuously green, and this milestone has already shipped exactly that defect once (durable record x6v6qxqd6f); this is a defect in the scan, not evidence the package is pure")
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
		t.Fatalf("internal/migrate imports non-stdlib package(s): %+v — full collected import set: %v", nonStdlib, allImports)
	}
	if len(sameModule) > 0 {
		t.Fatalf("internal/migrate imports from this module: %+v — the one-way import direction (internal/store imports internal/migrate, never the reverse) is broken; full collected import set: %v", sameModule, allImports)
	}
}
