// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file hosts D-11's convergence gate (CONTEXT.md): every Qdrant client
// CONSTRUCTION in the module — production and test code alike — must go
// through store.NewQdrantClient, so a regression test can never silently
// dial Qdrant with options production does not use (REQ-test-client-parity).
//
// It complements TestQdrantClientIsHeldOnlyByStorePackage
// (schemaversion_stamp_gate_test.go), which tracks who HOLDS a *qdrant.Client,
// with a narrower CALL-SITE gate: naming the *qdrant.Client TYPE stays legal
// everywhere (every dial helper's own signature returns one) — only INVOKING
// one of the underlying constructors outside NewQdrantClient's own body is a
// violation. This is deliberately a NEW, narrower walker rather than a reuse
// of fileRefsQdrantClient/scanRepoForQdrantClientRefs: those conflate type
// references with calls and explicitly exclude _test.go files, and D-11
// requires scanning _test.go files too (RESEARCH.md Pitfall 5) — a
// regression test's own dial helper is exactly the kind of bypass this gate
// must catch.
//
// What this gate does NOT close: an interface-carried or reflection-built
// client, and the generated conn-wrapping service-client constructors
// (qdrant's own NewQdrantClient/NewPointsClient family, which wrap an
// ALREADY-DIALED grpc.ClientConnInterface rather than dialing one) are out of
// scope — they need a connection this gate's constructors themselves
// produce.
package store

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// qdrantImportPath is the import path of the client package whose
// constructors this gate polices.
const qdrantImportPath = "github.com/qdrant/go-client/qdrant"

// qdrantClientConstructors is the set of qdrant-package functions that dial
// (construct) a client. Verified against qdrant-go-client@v1.19.2 (this
// phase's 01-05-PLAN.md interfaces section): NewClient (client.go:26),
// NewDefaultGrpcClient (grpc_client.go:28), NewGrpcClient (grpc_client.go:33),
// NewGrpcClientFromConn (grpc_client.go:83). The generated NewQdrantClient/
// NewPointsClient family is deliberately excluded — see the package doc
// comment above.
var qdrantClientConstructors = map[string]bool{
	"NewClient":             true,
	"NewGrpcClient":         true,
	"NewDefaultGrpcClient":  true,
	"NewGrpcClientFromConn": true,
}

// clientConstruction is one call site this gate found: either a genuine
// qdrantClientConstructors call through a locally-bound name of
// qdrantImportPath, a BARE (non-call) reference to one of those constructors
// as a VALUE (the function-value-alias bypass named in 01-REVIEW.md WR-01,
// e.g. `var dial = qdrant.NewClient`, recorded under the same callee name
// as a call would be, since the escape happens at the point the raw
// constructor is bound to something other than a direct call), or
// (callee == "dot-import") a dot-import of that path itself, recorded as a
// violation in its own right — a dot-import makes an unqualified
// constructor call invisible to this scanner, so the import spec is
// flagged rather than silently trusted.
type clientConstruction struct {
	path          string
	line          int
	enclosingFunc string
	callee        string
}

func (c clientConstruction) String() string {
	return fmt.Sprintf("%s:%d: %s in %s", c.path, c.line, c.callee, c.enclosingFunc)
}

// isSanctionedConstruction is the one legal call site, repo-wide: the
// receiver-less NewQdrantClient function defined in internal/store/store.go
// itself. Keyed on BOTH the display path and the enclosing FuncDecl name
// together — never on file identity alone — so a method also named
// NewQdrantClient (which renders via enclosingFuncDisplayName as
// "Type.NewQdrantClient") and a second, differently-named construction
// elsewhere in store.go both still trip this gate (Pitfall 5).
func isSanctionedConstruction(c clientConstruction) bool {
	return c.path == "internal/store/store.go" && c.enclosingFunc == "NewQdrantClient"
}

// scanQdrantClientConstructions parses src (Go source bytes, not necessarily
// a file on disk — fixtures pass synthetic bytes under a display-only path)
// and returns every clientConstruction found. It collects the local name of
// EVERY import spec whose path is qdrantImportPath (the default name
// "qdrant", an explicit alias, or "_" which is ignored — a blank import
// binds no usable name and permits no calls through it); a "." import is
// itself recorded as a construction (enclosing function "<import>", callee
// "dot-import"). A file with no qdrantImportPath import at all returns nil.
// Each top-level declaration is walked like scanQdrantCalls
// (schemaversion_stamp_gate_test.go) does — enclosingFuncDisplayName for
// FuncDecls, "<package-level>" otherwise — recording every *ast.CallExpr
// whose Fun is a selector on one of the collected local names with Sel.Name
// in qdrantClientConstructors, AND every BARE (non-call) *ast.SelectorExpr
// reference to one of those same names (e.g. `var dial = qdrant.NewClient`)
// — the function-value-alias bypass named in 01-REVIEW.md WR-01: a later
// call through such an alias (`dial(cfg)`) calls a plain *ast.Ident, never
// a *ast.SelectorExpr, so it stays permanently invisible to the CallExpr
// case; catching the point where the raw constructor escapes as a value is
// what closes the gap instead, without needing full value-flow analysis. A
// selector already recorded via the CallExpr case is never double-counted
// (tracked by node identity, not by value equality).
func scanQdrantClientConstructions(fset *token.FileSet, src []byte, displayPath string) ([]clientConstruction, error) {
	file, err := parser.ParseFile(fset, displayPath, src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", displayPath, err)
	}

	localNames := map[string]bool{}
	var sites []clientConstruction
	foundImport := false
	for _, imp := range file.Imports {
		path, uqErr := strconv.Unquote(imp.Path.Value)
		if uqErr != nil || path != qdrantImportPath {
			continue
		}
		foundImport = true
		switch {
		case imp.Name == nil:
			localNames["qdrant"] = true
		case imp.Name.Name == "_":
			// Blank import: binds no usable name, permits no calls.
		case imp.Name.Name == ".":
			pos := fset.Position(imp.Pos())
			sites = append(sites, clientConstruction{
				path:          displayPath,
				line:          pos.Line,
				enclosingFunc: "<import>",
				callee:        "dot-import",
			})
		default:
			localNames[imp.Name.Name] = true
		}
	}
	if !foundImport {
		return nil, nil
	}

	visit := func(enclosing string, n ast.Node) {
		// Pre-pass: mark every CallExpr's own Fun selector by node identity,
		// so the bare-value-reference case below never double-counts the
		// ordinary `qdrant.NewClient(cfg)` call shape as a second, spurious
		// violation (that selector is also visited independently as a child
		// node of the CallExpr).
		callFuncSelectors := map[*ast.SelectorExpr]bool{}
		ast.Inspect(n, func(inner ast.Node) bool {
			if call, ok := inner.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					callFuncSelectors[sel] = true
				}
			}
			return true
		})

		record := func(pos token.Pos, callee string) {
			p := fset.Position(pos)
			sites = append(sites, clientConstruction{
				path:          displayPath,
				line:          p.Line,
				enclosingFunc: enclosing,
				callee:        callee,
			})
		}

		ast.Inspect(n, func(inner ast.Node) bool {
			switch node := inner.(type) {
			case *ast.CallExpr:
				sel, ok := node.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				ident, ok := sel.X.(*ast.Ident)
				if !ok || !localNames[ident.Name] || !qdrantClientConstructors[sel.Sel.Name] {
					return true
				}
				record(node.Pos(), sel.Sel.Name)
			case *ast.SelectorExpr:
				if callFuncSelectors[node] {
					return true // already recorded by the CallExpr case above
				}
				ident, ok := node.X.(*ast.Ident)
				if !ok || !localNames[ident.Name] || !qdrantClientConstructors[node.Sel.Name] {
					return true
				}
				record(node.Pos(), node.Sel.Name)
			}
			return true
		})
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			// Package-level var/const initializers can construct too, and
			// are inside no function.
			visit("<package-level>", decl)
			continue
		}
		visit(enclosingFuncDisplayName(fn), fn)
	}
	return sites, nil
}

// scanRepoForClientConstructions locates the module root, then walks every
// .go file under it via filepath.WalkDir — skipping gen/, vendor/, testdata/,
// and dot-prefixed directories (root excluded from that check) — scanning
// EVERY .go file, _test.go INCLUDED (the one respect in which this gate must
// NOT reuse scanRepoForQdrantClientRefs's exclusion; D-11), and reports the
// module-root-relative forward-slash-separated display path for each. Counts
// non-test and test .go files scanned separately so the caller's
// zero-applicability guard can prove _test.go files were actually in scope.
func scanRepoForClientConstructions(fset *token.FileSet) (sites []clientConstruction, nonTestFiles, testFiles int, err error) {
	root, err := findModuleRoot()
	if err != nil {
		return nil, 0, 0, err
	}
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			name := d.Name()
			if name == "gen" || name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", path, readErr)
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)
		found, scanErr := scanQdrantClientConstructions(fset, src, rel)
		if scanErr != nil {
			return fmt.Errorf("scan %s: %w", rel, scanErr)
		}
		sites = append(sites, found...)
		if strings.HasSuffix(d.Name(), "_test.go") {
			testFiles++
		} else {
			nonTestFiles++
		}
		return nil
	})
	if walkErr != nil {
		return nil, nonTestFiles, testFiles, walkErr
	}
	return sites, nonTestFiles, testFiles, nil
}

// calleePair identifies a clientConstruction by (enclosingFunc, callee) —
// the SET EQUALITY key the bad-fixture subtests assert against, deliberately
// dropping path/line so the assertion is not accidentally coupled to
// display-path text.
type calleePair struct {
	enclosingFunc string
	callee        string
}

func pairSet(sites []clientConstruction) map[calleePair]bool {
	set := map[calleePair]bool{}
	for _, s := range sites {
		set[calleePair{enclosingFunc: s.enclosingFunc, callee: s.callee}] = true
	}
	return set
}

// assertPairSetEqual fails t with the full got/want detail when got and want
// are not set-equal — never a count-only or contains check (RESEARCH.md
// Pitfall 5 house style, mirroring TestEveryStoreConstructionRoutesThroughSeam).
func assertPairSetEqual(t *testing.T, got, want map[calleePair]bool, sites []clientConstruction) {
	t.Helper()
	for p := range got {
		if !want[p] {
			t.Errorf("unexpected construction (%s, %s) — full sites: %v", p.enclosingFunc, p.callee, sites)
		}
	}
	for p := range want {
		if !got[p] {
			t.Errorf("missing expected construction (%s, %s) — full sites: %v", p.enclosingFunc, p.callee, sites)
		}
	}
}

// TestQdrantClientConstructedOnlyByNewQdrantClient is D-11's gate.
func TestQdrantClientConstructedOnlyByNewQdrantClient(t *testing.T) {
	t.Run("good fixture yields one sanctioned site", func(t *testing.T) {
		fset := token.NewFileSet()
		src, err := os.ReadFile(filepath.Join("testdata", "qdrantclient", "good_store.go.txt"))
		if err != nil {
			t.Fatalf("read good fixture: %v", err)
		}
		sites, err := scanQdrantClientConstructions(fset, src, "internal/store/store.go")
		if err != nil {
			t.Fatalf("scan good fixture: %v", err)
		}
		if len(sites) != 1 {
			t.Fatalf("good fixture: want exactly 1 construction, got %d: %v", len(sites), sites)
		}
		if !isSanctionedConstruction(sites[0]) {
			t.Errorf("good fixture: %v should be sanctioned", sites[0])
		}
	})

	t.Run("bad test-helper fixture", func(t *testing.T) {
		fset := token.NewFileSet()
		src, err := os.ReadFile(filepath.Join("testdata", "qdrantclient", "bad_test.go.txt"))
		if err != nil {
			t.Fatalf("read bad test-helper fixture: %v", err)
		}
		sites, err := scanQdrantClientConstructions(fset, src, "internal/example/example_test.go")
		if err != nil {
			t.Fatalf("scan bad test-helper fixture: %v", err)
		}
		want := map[calleePair]bool{
			{enclosingFunc: "dialDirect", callee: "NewClient"}:   true,
			{enclosingFunc: "dialGrpc", callee: "NewGrpcClient"}: true,
		}
		assertPairSetEqual(t, pairSet(sites), want, sites)
		for _, s := range sites {
			if isSanctionedConstruction(s) {
				t.Errorf("bad test-helper fixture: %v must not be sanctioned", s)
			}
		}
	})

	t.Run("bad aliased fixture", func(t *testing.T) {
		fset := token.NewFileSet()
		src, err := os.ReadFile(filepath.Join("testdata", "qdrantclient", "bad_aliased_test.go.txt"))
		if err != nil {
			t.Fatalf("read bad aliased fixture: %v", err)
		}
		sites, err := scanQdrantClientConstructions(fset, src, "internal/example/aliased_test.go")
		if err != nil {
			t.Fatalf("scan bad aliased fixture: %v", err)
		}
		want := map[calleePair]bool{
			{enclosingFunc: "dialAliased", callee: "NewClient"}: true,
			{enclosingFunc: "<import>", callee: "dot-import"}:   true,
		}
		assertPairSetEqual(t, pairSet(sites), want, sites)
		for _, s := range sites {
			if isSanctionedConstruction(s) {
				t.Errorf("bad aliased fixture: %v must not be sanctioned", s)
			}
		}
	})

	t.Run("bad second construction in store.go", func(t *testing.T) {
		fset := token.NewFileSet()
		src, err := os.ReadFile(filepath.Join("testdata", "qdrantclient", "bad_store.go.txt"))
		if err != nil {
			t.Fatalf("read bad store fixture: %v", err)
		}
		sites, err := scanQdrantClientConstructions(fset, src, "internal/store/store.go")
		if err != nil {
			t.Fatalf("scan bad store fixture: %v", err)
		}
		var sanctioned, violations []clientConstruction
		for _, s := range sites {
			if isSanctionedConstruction(s) {
				sanctioned = append(sanctioned, s)
			} else {
				violations = append(violations, s)
			}
		}
		if len(sanctioned) != 1 {
			t.Errorf("bad store fixture: want exactly 1 sanctioned site, got %d: %v", len(sanctioned), sanctioned)
		}
		want := map[calleePair]bool{
			{enclosingFunc: "sneakyDial", callee: "NewClient"}: true,
		}
		assertPairSetEqual(t, pairSet(violations), want, violations)
	})

	t.Run("bad function-value-alias fixture", func(t *testing.T) {
		fset := token.NewFileSet()
		src, err := os.ReadFile(filepath.Join("testdata", "qdrantclient", "bad_valueref_test.go.txt"))
		if err != nil {
			t.Fatalf("read bad value-ref fixture: %v", err)
		}
		sites, err := scanQdrantClientConstructions(fset, src, "internal/example/valueref_test.go")
		if err != nil {
			t.Fatalf("scan bad value-ref fixture: %v", err)
		}
		want := map[calleePair]bool{
			{enclosingFunc: "<package-level>", callee: "NewClient"}: true,
		}
		assertPairSetEqual(t, pairSet(sites), want, sites)
		for _, s := range sites {
			if isSanctionedConstruction(s) {
				t.Errorf("bad value-ref fixture: %v must not be sanctioned", s)
			}
		}
	})

	t.Run("real module", func(t *testing.T) {
		fset := token.NewFileSet()
		sites, nonTestFiles, testFiles, err := scanRepoForClientConstructions(fset)
		if err != nil {
			t.Fatalf("scanRepoForClientConstructions: %v", err)
		}
		if nonTestFiles == 0 || testFiles == 0 {
			t.Fatalf("scanned %d non-test and %d test .go files across the module — a scan that sees nothing must not report clean", nonTestFiles, testFiles)
		}
		t.Logf("scanned %d non-test and %d test .go files", nonTestFiles, testFiles)

		var sanctioned, violations []clientConstruction
		for _, s := range sites {
			if isSanctionedConstruction(s) {
				sanctioned = append(sanctioned, s)
			} else {
				violations = append(violations, s)
			}
		}
		for _, v := range violations {
			t.Errorf("Qdrant client construction outside store.NewQdrantClient: %v", v)
		}
		if len(sanctioned) != 1 {
			t.Errorf("want exactly 1 sanctioned construction site, got %d: %v", len(sanctioned), sanctioned)
		}
	})
}
