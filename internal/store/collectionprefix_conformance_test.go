// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file hosts D-20's third and fourth checkable claims for the
// shared-CI-Qdrant mitigation (CONTEXT.md, plans 01-04/01-05):
//
//   - TestEveryStoreConstructionRoutesThroughSeam (Task 1): a SOURCE-LEVEL
//     scan proving no live Store construction in any of the four
//     Qdrant-backed packages' test sources bypasses that package's
//     newTestStore seam by passing a raw collection-name literal.
//   - TestCollectionPrefixesAreDisjoint (Task 2): reads all four packages'
//     testCollectionPrefix constants directly out of their own test
//     sources and asserts they are pairwise disjoint, including the
//     leading-substring case.
//
// Both tests read the other three packages (internal/server, internal/e2e,
// internal/retrievaleval) as PLAIN SOURCE TEXT via go/parser, never by
// importing them — internal/store cannot import any of them without an
// import cycle (they already import internal/store), and this is what
// lets ONE test in internal/store's own test scope see all four packages'
// otherwise-unexported test-only declarations. This file lives here, in
// the package that owns the collection concept, rather than in a new
// tool or shared helper package; it adds no production dependency and no
// import edge. Do not move it somewhere that would create one.
//
// Division of labour with plan 01-05's runtime newTestStore seam: that
// seam checks the VALUE of whatever collection name a test passes it, at
// run time, and t.Fatalf's if it does not carry the package's own
// testCollectionPrefix. This file's scan does NOT re-check that value —
// an identifier argument (e.g. testCollection("foo"), or a seam's own
// `name` parameter) is always clean here regardless of what it resolves
// to at run time. What this file adds is the complementary half: forcing
// every LIVE construction through that runtime seam in the first place,
// so a contributor cannot simply assign a raw literal to a variable
// before newTestStore's own check would ever see it — an AST scan
// inspects the argument EXPRESSION at the call site, not a value that
// only exists once the test runs. A reader who mistakes either mechanism
// for the whole story will end up weakening the wrong one.
package store

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// qdrantPackage names one of the four Qdrant-backed packages this phase's
// shared-CI-Qdrant mitigation covers (CONTEXT.md D-16/D-20): internal/store
// and internal/server, whose collection names collided before plan 01-04,
// plus internal/e2e and internal/retrievaleval, which plan 01-05 extended
// the same testCollectionPrefix/newTestStore seam to. dir is relative to
// internal/store's own directory, since `go test` runs with the package
// directory as its working directory. allowUnqualified is true only for
// internal/store itself: Go permits at most one top-level func named New
// per package, so a BARE `New(...)` call inside internal/store's own test
// sources can only be that package's own constructor, while every other
// package must qualify it as `store.New(...)`. A package landing here with
// no testCollectionPrefix constant declared is a finding in
// TestCollectionPrefixesAreDisjoint, never a silent skip.
type qdrantPackage struct {
	name             string
	dir              string
	allowUnqualified bool
}

var qdrantBackedPackages = []qdrantPackage{
	{name: "internal/store", dir: ".", allowUnqualified: true},
	{name: "internal/server", dir: "../server", allowUnqualified: false},
	{name: "internal/e2e", dir: "../e2e", allowUnqualified: false},
	{name: "internal/retrievaleval", dir: "../retrievaleval", allowUnqualified: false},
}

// storeConstructionFinding is one violation surfaced by scanConstructions:
// a live Store construction whose collection argument is a raw string
// literal instead of routing through the package's newTestStore seam.
type storeConstructionFinding struct {
	path    string
	line    int
	literal string
}

func (f storeConstructionFinding) String() string {
	return fmt.Sprintf("%s:%d: raw collection literal %s bypasses this package's newTestStore seam — route it through testCollection()", f.path, f.line, f.literal)
}

// isNilIdent reports whether e is the literal `nil` identifier.
//
// Every EXISTING raw-collection-literal construction across all four
// Qdrant-backed packages' test sources (grep-verified against this repo at
// this plan's base commit: internal/store's decisionlog_test.go, the
// WithClock/WithAuthz pair in store_test.go, bench_test.go, rerank_test.go,
// reindex_test.go's TestReindexRejectsInvalidArgs, and the cross-package
// store_test package's spine_forgery_test.go) passes nil here — these are
// hermetic unit tests whose comments say so directly (rerank_test.go: "a
// nil client is safe — the round-trip tests Skip in that case"). A
// nil-client Store can never dial Qdrant, so it can never collide with
// another package's collection name on the shared CI instance (CONTEXT.md
// D-20's actual concern). Enforcing the seam on these would force
// out-of-scope edits to dozens of pre-existing hermetic tests this plan's
// files_modified list does not name, for zero collision-safety gain — so
// this gate deliberately scopes itself to constructions that could
// actually touch a real, possibly-shared Qdrant instance.
func isNilIdent(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "nil"
}

// isStoreConstructorCall reports whether call invokes the store package's
// New constructor: unqualified when allowUnqualified is true (only valid
// inside internal/store's own test sources), or qualified as store.New
// otherwise (every other Qdrant-backed package's confirmed convention, per
// all three of their own newTestStore bodies).
func isStoreConstructorCall(call *ast.CallExpr, allowUnqualified bool) bool {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return allowUnqualified && fn.Name == "New"
	case *ast.SelectorExpr:
		pkg, ok := fn.X.(*ast.Ident)
		return ok && pkg.Name == "store" && fn.Sel.Name == "New"
	default:
		return false
	}
}

// scanConstructions parses src (Go source bytes, not necessarily a file on
// disk — the fixtures pass synthetic bytes under a display-only path) and
// returns one finding per call to the store constructor whose collection
// argument (New's second parameter) is a string literal AND whose client
// argument (New's first parameter) is not the nil identifier. fset is
// shared with the caller so returned line numbers stay meaningful when
// scanning multiple files into one report.
func scanConstructions(fset *token.FileSet, src []byte, displayPath string, allowUnqualified bool) ([]storeConstructionFinding, error) {
	file, err := parser.ParseFile(fset, displayPath, src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", displayPath, err)
	}

	var findings []storeConstructionFinding
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isStoreConstructorCall(call, allowUnqualified) || len(call.Args) < 2 {
			return true
		}
		if isNilIdent(call.Args[0]) {
			return true
		}
		lit, ok := call.Args[1].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		pos := fset.Position(call.Pos())
		findings = append(findings, storeConstructionFinding{
			path:    displayPath,
			line:    pos.Line,
			literal: lit.Value,
		})
		return true
	})
	return findings, nil
}

// scanPackageDir walks every _test.go file directly inside dir (no
// recursion — each of the four Qdrant-backed packages' test files live
// flat in their own package directory) and returns every finding plus the
// count of files actually scanned. A missing directory surfaces as an
// error, not as a silent zero — the caller's zero-applicability guard
// (T-01-20) treats "err != nil" and "filesScanned == 0" as the same class
// of failure: a scan that cannot see what it was told to check must never
// report clean.
func scanPackageDir(fset *token.FileSet, dir string, allowUnqualified bool) (findings []storeConstructionFinding, filesScanned int, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, fmt.Errorf("read dir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, filesScanned, fmt.Errorf("read %s: %w", path, readErr)
		}
		fs, scanErr := scanConstructions(fset, src, path, allowUnqualified)
		if scanErr != nil {
			return nil, filesScanned, scanErr
		}
		findings = append(findings, fs...)
		filesScanned++
	}
	return findings, filesScanned, nil
}

// TestEveryStoreConstructionRoutesThroughSeam is D-20's third checkable
// claim: no test file in a Qdrant-backed package constructs a live Store
// over a raw collection-name literal, bypassing that package's
// newTestStore seam.
func TestEveryStoreConstructionRoutesThroughSeam(t *testing.T) {
	t.Run("good fixture yields zero findings", func(t *testing.T) {
		fset := token.NewFileSet()
		src, err := os.ReadFile(filepath.Join("testdata", "collectionprefix", "good_pkg_test.go.txt"))
		if err != nil {
			t.Fatalf("read good fixture: %v", err)
		}
		findings, err := scanConstructions(fset, src, "good_pkg_test.go.txt", false)
		if err != nil {
			t.Fatalf("scan good fixture: %v", err)
		}
		if len(findings) != 0 {
			t.Fatalf("good fixture: want zero findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("bad fixture yields at least one finding naming the offending literal", func(t *testing.T) {
		fset := token.NewFileSet()
		src, err := os.ReadFile(filepath.Join("testdata", "collectionprefix", "bad_pkg_test.go.txt"))
		if err != nil {
			t.Fatalf("read bad fixture: %v", err)
		}
		findings, err := scanConstructions(fset, src, "bad_pkg_test.go.txt", false)
		if err != nil {
			t.Fatalf("scan bad fixture: %v", err)
		}
		if len(findings) == 0 {
			t.Fatal("bad fixture: want at least one finding naming the offending literal, got none")
		}
		for _, f := range findings {
			t.Logf("bad fixture finding: %s", f)
		}
	})

	t.Run("zero-applicability guard: nonexistent package directory fails loudly", func(t *testing.T) {
		fset := token.NewFileSet()
		_, _, err := scanPackageDir(fset, "testdata/does-not-exist-zzz", false)
		if err == nil {
			t.Fatal("scanPackageDir(nonexistent dir) = nil error, want a failure — a gate that silently scans nothing must not report clean")
		}
	})

	t.Run("real packages", func(t *testing.T) {
		fset := token.NewFileSet()
		var all []storeConstructionFinding
		totalFiles := 0
		for _, pkg := range qdrantBackedPackages {
			findings, n, err := scanPackageDir(fset, pkg.dir, pkg.allowUnqualified)
			if err != nil {
				t.Fatalf("scan %s (%s): %v", pkg.name, pkg.dir, err)
			}
			if n == 0 {
				t.Fatalf("scan %s (%s): visited zero _test.go files — a package rename or empty directory must fail this gate, not report clean", pkg.name, pkg.dir)
			}
			totalFiles += n
			all = append(all, findings...)
		}
		if totalFiles == 0 {
			t.Fatal("scanned zero files across all four packages")
		}
		t.Logf("scanned %d _test.go files across %d packages", totalFiles, len(qdrantBackedPackages))
		for _, f := range all {
			t.Error(f)
		}
	})
}
