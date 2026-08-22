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
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	return fmt.Sprintf("%s:%d: live store construction with collection argument %s bypasses this package's newTestStore seam — route it through newTestStore()/testCollection()", f.path, f.line, f.literal)
}

// seamFuncName is the one function per package permitted to call the raw
// store constructor. Every other live construction is a finding
// REGARDLESS of how its collection argument is spelled.
//
// The original rule flagged only a *string literal* collection argument,
// which was unsound in both directions. It let a bypass through —
//
//	name := "raw_unprefixed"
//	s := store.New(client, name)   // *ast.Ident, not *ast.BasicLit: invisible
//
// — and it "excluded" the seam's own legitimate `New(c, name, opts...)`
// call for exactly the same reason (`name` is a parameter identifier),
// not because the scan recognized it as the seam. So the check never
// distinguished "routes through the seam" from "argument is not a
// literal"; it only ever looked like it did. Anchoring on the enclosing
// function name is both stronger and simpler than the dataflow analysis
// the literal-shaped rule would otherwise need.
const seamFuncName = "newTestStore"

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

	// Walk per-declaration rather than over the whole file, so each call
	// is attributed to its enclosing function and the seam's own call can
	// be excluded by IDENTITY instead of by the shape of its argument.
	inspect := func(n ast.Node, insideSeam bool) {
		ast.Inspect(n, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isStoreConstructorCall(call, allowUnqualified) || len(call.Args) < 2 {
				return true
			}
			// A nil client never dials Qdrant, so it cannot collide on the
			// shared instance — the pre-existing hermetic unit tests stay
			// out of scope (unchanged from the original rule).
			if isNilIdent(call.Args[0]) {
				return true
			}
			if insideSeam {
				return true
			}
			pos := fset.Position(call.Pos())
			findings = append(findings, storeConstructionFinding{
				path:    displayPath,
				line:    pos.Line,
				literal: types.ExprString(call.Args[1]),
			})
			return true
		})
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			// Package-level var/const initializers can construct too, and
			// are inside no function — never the seam.
			inspect(decl, false)
			continue
		}
		inspect(fn, fn.Name != nil && fn.Name.Name == seamFuncName)
	}
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

	// Asserts SET EQUALITY over both bypass shapes, not `len(findings) > 0`.
	// An "at least one finding" assertion passes while catching only the
	// literal form — which is precisely the state this gate shipped in, and
	// precisely what a count-agnostic assertion failed to reveal.
	t.Run("bad fixture yields a finding for BOTH bypass shapes", func(t *testing.T) {
		fset := token.NewFileSet()
		src, err := os.ReadFile(filepath.Join("testdata", "collectionprefix", "bad_pkg_test.go.txt"))
		if err != nil {
			t.Fatalf("read bad fixture: %v", err)
		}
		findings, err := scanConstructions(fset, src, "bad_pkg_test.go.txt", false)
		if err != nil {
			t.Fatalf("scan bad fixture: %v", err)
		}
		got := make(map[string]bool, len(findings))
		for _, f := range findings {
			got[f.literal] = true
			t.Logf("bad fixture finding: %s", f)
		}
		// The inline literal, and the same raw name bound to a variable
		// first. The second is invisible to any rule keyed on the
		// argument's syntactic shape.
		want := []string{`"hardcoded-bad-pkg-collection"`, "name"}
		for _, w := range want {
			if !got[w] {
				t.Errorf("bad fixture: no finding for collection argument %s — got %v. "+
					"A bypass this gate cannot see is the defect it exists to prevent.", w, sortedFindingArgs(got))
			}
		}
		if len(got) != len(want) {
			t.Errorf("bad fixture: got %d distinct findings %v, want exactly %d %v", len(got), sortedFindingArgs(got), len(want), want)
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

// extractTestCollectionPrefix parses every _test.go file in dir looking
// for a package-level `const testCollectionPrefix = "..."` declaration and
// returns its string value. Reading it from source rather than importing
// the package is what lets this one test see all four values: the
// constants live in four separate test-only scopes that no single package
// can import together (internal/server, internal/e2e, and
// internal/retrievaleval all already import internal/store; internal/store
// importing any of them back would be a cycle).
func extractTestCollectionPrefix(fset *token.FileSet, dir string) (value string, found bool, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false, fmt.Errorf("read dir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return "", false, fmt.Errorf("read %s: %w", path, readErr)
		}
		file, parseErr := parser.ParseFile(fset, path, src, 0)
		if parseErr != nil {
			return "", false, fmt.Errorf("parse %s: %w", path, parseErr)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vspec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range vspec.Names {
					if name.Name != "testCollectionPrefix" || i >= len(vspec.Values) {
						continue
					}
					lit, ok := vspec.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					unquoted, unquoteErr := strconv.Unquote(lit.Value)
					if unquoteErr != nil {
						return "", false, fmt.Errorf("%s: unquote testCollectionPrefix literal %s: %w", path, lit.Value, unquoteErr)
					}
					return unquoted, true, nil
				}
			}
		}
	}
	return "", false, nil
}

// resolvedPrefix pairs a package's name with its extracted
// testCollectionPrefix value, for TestCollectionPrefixesAreDisjoint's
// pairwise comparison and its -v output.
type resolvedPrefix struct {
	name   string
	prefix string
}

// TestCollectionPrefixesAreDisjoint is D-20's fourth checkable claim: the
// four Qdrant-backed packages' collection-name prefixes are pairwise
// disjoint, so two packages can never concatenate to the same final
// collection name on the shared CI Qdrant instance.
func TestCollectionPrefixesAreDisjoint(t *testing.T) {
	fset := token.NewFileSet()
	var prefixes []resolvedPrefix
	for _, pkg := range qdrantBackedPackages {
		value, found, err := extractTestCollectionPrefix(fset, pkg.dir)
		if err != nil {
			t.Fatalf("extract testCollectionPrefix for %s (%s): %v", pkg.name, pkg.dir, err)
		}
		if !found {
			// A package with no declared prefix is a finding, not a skip:
			// treating "not found" as "nothing to check" would let a fifth
			// Qdrant-backed package join the shared instance unnamespaced
			// and silent, exactly the state this gate exists to prevent.
			t.Errorf("%s (%s): no testCollectionPrefix constant found", pkg.name, pkg.dir)
			continue
		}
		prefixes = append(prefixes, resolvedPrefix{name: pkg.name, prefix: value})
	}
	if len(prefixes) != len(qdrantBackedPackages) {
		t.Fatalf("resolved %d of %d package prefixes; see errors above", len(prefixes), len(qdrantBackedPackages))
	}

	sort.Slice(prefixes, func(i, j int) bool { return prefixes[i].name < prefixes[j].name })
	for _, p := range prefixes {
		t.Logf("resolved prefix: %s = %q", p.name, p.prefix)
	}

	// Every ORDERED pair, not just unordered pairs: HasPrefix is not
	// symmetric, and reporting only unordered pairs would still name both
	// values, but walking ordered pairs is what makes the comparison count
	// (len(prefixes) * (len(prefixes)-1)) directly checkable against "four
	// packages" in the SUMMARY rather than requiring the reader to redo
	// the combinatorics themselves.
	comparisons := 0
	for i := range prefixes {
		for j := range prefixes {
			if i == j {
				continue
			}
			comparisons++
			a, b := prefixes[i], prefixes[j]
			if a.prefix == b.prefix {
				t.Errorf("%s and %s share the identical prefix %q", a.name, b.name, a.prefix)
				continue
			}
			// The leading-substring case: two DISTINCT prefixes can still
			// concatenate to the same final collection name when one is a
			// prefix of the other and the shorter package's collection
			// happens to start with the longer prefix's remainder (e.g.
			// prefixes "e2e_" and "e2e_x_" would collide on collection
			// name "x_foo" in the "e2e_" package: "e2e_" + "x_foo" ==
			// "e2e_x_" + "foo"). A plain set-equality check over the four
			// values would miss this entirely; naming it here so a future
			// reader does not simplify this comparison down to one.
			if strings.HasPrefix(b.prefix, a.prefix) {
				t.Errorf("%s's prefix %q is a leading substring of %s's prefix %q: a collection name in %s beginning with %q could collide with one in %s",
					a.name, a.prefix, b.name, b.prefix, b.name, strings.TrimPrefix(b.prefix, a.prefix), a.name)
			}
		}
	}
	t.Logf("compared %d ordered pairs across %d packages", comparisons, len(prefixes))
}

// sortedFindingArgs renders a finding set deterministically for failure
// messages. Named distinctly from this package's pre-existing keysOf,
// which is generic over a different map type.
func sortedFindingArgs(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
