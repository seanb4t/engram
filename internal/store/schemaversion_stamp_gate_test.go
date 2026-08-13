// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file hosts D-03's structural half of the schema-version stamping
// guarantee (CONTEXT.md, 02-02-PLAN.md): a source-level conformance gate
// proving that payload() is the only door for full-record Qdrant writes.
//
// The gate is anchored on the TRANSMITTED WRITE BOUNDARY, not on
// qdrant.PointStruct composite-literal construction (see 02-02-PLAN.md's
// "gate redesign" section for the full rationale). scanQdrantCalls derives
// its subject set from ast.CallExpr nodes whose callee is a selector call
// naming one of a caller-supplied method set — matched on the METHOD NAME
// ALONE, deliberately ignoring the receiver expression. This
// over-approximates: it catches a bypass whose request came from a helper,
// a clone, a parameter, or a generated builder, even though it constructs no
// new qdrant.PointStruct literal.
//
// What this gate proves, and what it does not (narrowed deliberately, per
// durable record x6v6qxqd6f: a gate must not claim more than it proves):
// every DIRECT SELECTOR-CALL transmission in the internal/store package
// directory is classified and, where direct-conforming, routes its payload
// through payload(). It does NOT establish that the matched method belongs
// to *qdrant.Client — go/ast carries no type identity. Five escapes follow
// from that:
//
//  1. A method value (`write := c.Upsert; write(...)`) — PINNED as a
//     tested limit by limits_pkg.go.txt's asserted-empty derived set.
//  2. A method expression (`(*qdrant.Client).Upsert`) — same fixture,
//     same assertion.
//  3. A cross-package writer holding its own *qdrant.Client — CONVERTED
//     from silent to loud by TestQdrantClientIsHeldOnlyByStorePackage
//     (Task 2), never fully closed (an interface-carried client still
//     escapes it).
//  4. A differently-named in-package wrapper — already caught: the
//     wrapper's own inner .Upsert(...) call site is in the scanned
//     directory and surfaces unclassified.
//  5. An alternate Qdrant mutation verb this file's method sets do not
//     enumerate — OPEN and disclaimed; the method vocabulary is a
//     maintained list whose incompleteness is invisible to the gate.
package store

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// qdrantCallSite is one call site whose selector method name matched the
// caller-supplied method set passed to scanQdrantCalls.
type qdrantCallSite struct {
	enclosingFunc string // e.g. "Store.Upsert"; "<package-level>" outside any func
	method        string // the matched method name, e.g. "Upsert"
	receiver      string // receiverText(...) rendering of the selector's X — METADATA, never a filter
	path          string
	line          int
}

func (c qdrantCallSite) String() string {
	return fmt.Sprintf("%s:%d: %s.%s(...) in %s", c.path, c.line, c.receiver, c.method, c.enclosingFunc)
}

// receiverText renders a SelectorExpr's X (the receiver expression) back to
// source text for the common shapes this scanner needs to distinguish a
// direct transmission from a delegation: an *ast.Ident renders as its name
// (s), a nested *ast.SelectorExpr renders dotted (s.client), and anything
// else renders as a fixed sentinel so this never panics or silently
// mis-renders an unexpected shape.
//
// This text is classification METADATA, never a match filter: scanQdrantCalls
// matches on the selector's method name alone, deliberately ignoring the
// receiver, and this function's output is recorded on the resulting
// qdrantCallSite for a classification table to key on afterward. A helper
// that FILTERS call sites by receiver defeats the point of
// over-approximating the match.
func receiverText(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return receiverText(x.X) + "." + x.Sel.Name
	default:
		return "<expr>"
	}
}

// recvTypeName renders a method's receiver TYPE (not value) name, unwrapping
// a leading pointer star, for enclosingFuncDisplayName's use.
func recvTypeName(e ast.Expr) string {
	if star, ok := e.(*ast.StarExpr); ok {
		e = star.X
	}
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// enclosingFuncDisplayName is the identity key scanQdrantCalls attributes
// every call site to: the innermost enclosing ast.FuncDecl's name, prefixed
// with the receiver type name when there is one (e.g. "Store.Reindex"). This
// is a FUNCTION NAME, never a line number — store.go's lines shift on every
// edit, which would make a line-keyed classification stale by accident.
func enclosingFuncDisplayName(fn *ast.FuncDecl) string {
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		if t := recvTypeName(fn.Recv.List[0].Type); t != "" {
			return t + "." + fn.Name.Name
		}
	}
	return fn.Name.Name
}

// scanQdrantCalls parses src (Go source bytes, not necessarily a file on
// disk — the fixtures pass synthetic bytes under a display-only path) and
// returns one qdrantCallSite per ast.CallExpr whose callee is an
// *ast.SelectorExpr whose Sel.Name is a member of methods.
//
// It matches on the METHOD NAME ALONE, deliberately ignoring the receiver
// expression. This over-approximates the true subject set: it catches
// s.client.Upsert(...), a local alias c := s.client; c.Upsert(...), a
// parameter cl.Upsert(...), a delegating self-call s.Upsert(...), and a
// same-named method on some unrelated type. Over-approximation is the
// correct bias for a conformance gate — a false positive costs one line of
// explicit classification, while a false negative is exactly the hole a
// Phase 1 gate shipped with (durable record x6v6qxqd6f). Do NOT narrow this
// match to a receiver-expression pattern to reduce classification noise —
// that reintroduces the syntax-fragility this design removes.
//
// fset is shared with the caller so returned line numbers stay meaningful
// when scanning multiple files into one report.
func scanQdrantCalls(fset *token.FileSet, src []byte, displayPath string, methods map[string]bool) ([]qdrantCallSite, error) {
	file, err := parser.ParseFile(fset, displayPath, src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", displayPath, err)
	}

	var sites []qdrantCallSite
	visit := func(enclosing string, n ast.Node) {
		ast.Inspect(n, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !methods[sel.Sel.Name] {
				return true
			}
			pos := fset.Position(call.Pos())
			sites = append(sites, qdrantCallSite{
				enclosingFunc: enclosing,
				method:        sel.Sel.Name,
				receiver:      receiverText(sel.X),
				path:          displayPath,
				line:          pos.Line,
			})
			return true
		})
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			// Package-level var/const initializers are inside no function —
			// never a classified write path in this package, but visited so
			// a stray call there is never silently dropped from the scan.
			visit("<package-level>", decl)
			continue
		}
		visit(enclosingFuncDisplayName(fn), fn)
	}
	return sites, nil
}

// scanPackageDirForCalls walks dir NON-RECURSIVELY (Go treats a subdirectory
// as a different package anyway, so non-recursion is correct, not merely
// convenient) and returns every qdrantCallSite found in every entry whose
// name ends in suffix and does NOT end in excludeSuffix (when non-empty),
// plus the count of files actually scanned. The suffix/excludeSuffix pair
// lets this one function serve both the real package ("*.go", excluding
// "_test.go") and a fixture directory (".go.txt", no exclusion).
//
// A missing directory surfaces as an error, not a silent zero — and every
// caller's zero-applicability guard treats "err != nil" and "filesScanned ==
// 0" as the SAME class of failure: a scan that cannot see what it was told
// to check must never report clean (T-02-06).
//
// Because it is non-recursive, its stated scope is exactly "this one
// directory's own files matching suffix" — never "every adapter used by
// this package". Overstating that scope in a doc comment is how a future
// reader concludes a hole is covered when it is not.
func scanPackageDirForCalls(fset *token.FileSet, dir, suffix, excludeSuffix string, methods map[string]bool) (sites []qdrantCallSite, filesScanned int, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, fmt.Errorf("read dir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		if excludeSuffix != "" && strings.HasSuffix(e.Name(), excludeSuffix) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, filesScanned, fmt.Errorf("read %s: %w", path, readErr)
		}
		fs, scanErr := scanQdrantCalls(fset, src, path, methods)
		if scanErr != nil {
			return nil, filesScanned, scanErr
		}
		sites = append(sites, fs...)
		filesScanned++
	}
	return sites, filesScanned, nil
}

// payloadDerivesFromCodec is the CONFORMANCE check, applied ONLY to a call
// site the boundary scan has already classified DIRECT-CONFORMING (a
// delegation entry builds no point of its own and is exempt by
// construction). Given the enclosing ast.FuncDecl, it locates a composite
// literal carrying a "Payload:" key-value element and reports conforming
// when and only when that element's value is a call whose callee is the
// selector qdrant.NewValueMap AND that call's single argument is itself a
// call whose callee is the bare identifier payload. It does not inspect
// what is inside payload(...)'s own argument list — an argument-shape rule
// is precisely the check a local variable defeated in Phase 1 (durable
// record x6v6qxqd6f).
//
// This is a conformance check, not a completeness derivation: it answers
// "does the site the boundary scan found route its payload through the
// codec", never "which sites exist" — that is scanQdrantCalls's job.
// Conflating the two is the defect this gate's redesign removes.
func payloadDerivesFromCodec(fn *ast.FuncDecl) bool {
	if fn == nil {
		return false
	}
	var found, conforms bool
	ast.Inspect(fn, func(n ast.Node) bool {
		if found {
			return false
		}
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "Payload" {
				continue
			}
			found = true
			conforms = isNewValueMapOfPayload(kv.Value)
			return false
		}
		return true
	})
	return found && conforms
}

// isNewValueMapOfPayload reports whether e is a call to qdrant.NewValueMap
// whose single argument is itself a call to the bare identifier payload.
func isNewValueMapOfPayload(e ast.Expr) bool {
	outer, ok := e.(*ast.CallExpr)
	if !ok || len(outer.Args) != 1 {
		return false
	}
	sel, ok := outer.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "NewValueMap" {
		return false
	}
	if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "qdrant" {
		return false
	}
	inner, ok := outer.Args[0].(*ast.CallExpr)
	if !ok {
		return false
	}
	fnIdent, ok := inner.Fun.(*ast.Ident)
	return ok && fnIdent.Name == "payload"
}

// fullWriteMethods is the method-name set for the full-write boundary scan:
// the transmitted write boundary this gate's headline guarantee covers.
var fullWriteMethods = map[string]bool{"Upsert": true}

// parseGoFile is a test-only helper: parse path (relative to this package's
// directory, matching go test's own working directory convention) and
// return its *ast.File, failing the test loudly on any read/parse error.
func parseGoFile(t *testing.T, path string) *ast.File {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return file
}

// findFuncDeclByDisplayName returns the *ast.FuncDecl in file whose
// enclosingFuncDisplayName equals name, or nil if none matches.
func findFuncDeclByDisplayName(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if enclosingFuncDisplayName(fn) == name {
			return fn
		}
	}
	return nil
}

// sortedSiteNames renders a call-site-name map's keys deterministically for
// failure messages.
func sortedSiteNames(m map[string]qdrantCallSite) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

const schemaVersionStampFixtureDir = "testdata/schemaversionstamp"

// TestEveryPointWriteRoutesThroughPayload is D-03's structural half:
// derives, classifies, and conformance-checks the full-write boundary.
// Task 1's fixture subtests below pin the scanner's contract in isolation;
// Task 2 adds a "real package" subtest applying it to internal/store
// itself.
func TestEveryPointWriteRoutesThroughPayload(t *testing.T) {
	t.Run("good fixture: every direct Upsert site is conforming", func(t *testing.T) {
		fset := token.NewFileSet()
		path := filepath.Join(schemaVersionStampFixtureDir, "good_pkg.go.txt")
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read good fixture: %v", err)
		}
		sites, err := scanQdrantCalls(fset, src, path, fullWriteMethods)
		if err != nil {
			t.Fatalf("scan good fixture: %v", err)
		}
		if len(sites) != 3 {
			t.Fatalf("good fixture: got %d sites, want 3: %v", len(sites), sites)
		}
		byName := map[string]qdrantCallSite{}
		for _, s := range sites {
			byName[s.enclosingFunc] = s
		}

		file := parseGoFile(t, path)
		for _, name := range []string{"writer.Upsert", "writer.directViaLocal"} {
			s, ok := byName[name]
			if !ok {
				t.Errorf("good fixture: missing direct-conforming site %s — got %v", name, sortedSiteNames(byName))
				continue
			}
			if !strings.HasSuffix(s.receiver, ".client") {
				t.Errorf("good fixture: %s receiver = %q, want a .client-suffixed receiver", name, s.receiver)
			}
			fn := findFuncDeclByDisplayName(file, name)
			if !payloadDerivesFromCodec(fn) {
				t.Errorf("good fixture: %s does not pass the conformance predicate", name)
			}
		}

		delSite, ok := byName["writer.delegatingWrapper"]
		if !ok {
			t.Fatalf("good fixture: missing delegation site writer.delegatingWrapper — got %v", sortedSiteNames(byName))
		}
		if delSite.receiver != "w" {
			t.Errorf("good fixture: delegation receiver = %q, want bare %q", delSite.receiver, "w")
		}
	})

	// Asserts SET EQUALITY over both bypass shapes, not `len(sites) > 0` — an
	// "at least one finding" assertion passes while catching only one of two
	// bypass shapes, which is exactly the state a Phase 1 gate shipped in.
	t.Run("bad fixture yields a finding for BOTH bypass shapes", func(t *testing.T) {
		fset := token.NewFileSet()
		path := filepath.Join(schemaVersionStampFixtureDir, "bad_pkg.go.txt")
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read bad fixture: %v", err)
		}
		sites, err := scanQdrantCalls(fset, src, path, fullWriteMethods)
		if err != nil {
			t.Fatalf("scan bad fixture: %v", err)
		}
		got := map[string]qdrantCallSite{}
		for _, s := range sites {
			got[s.enclosingFunc] = s
			t.Logf("bad fixture finding: %s", s)
		}
		file := parseGoFile(t, path)
		want := []string{"writer.handBuiltPayload", "writer.helperBuiltWrite"}
		for _, w := range want {
			s, ok := got[w]
			if !ok {
				t.Errorf("bad fixture: no finding for %s — got %v. A bypass this gate cannot see is the defect it exists to prevent.", w, sortedSiteNames(got))
				continue
			}
			_ = s
			fn := findFuncDeclByDisplayName(file, w)
			if payloadDerivesFromCodec(fn) {
				t.Errorf("bad fixture: %s unexpectedly passes the conformance predicate", w)
			}
		}
		if len(got) != len(want) {
			t.Errorf("bad fixture: got %d distinct findings %v, want exactly %d %v", len(got), sortedSiteNames(got), len(want), want)
		}
	})

	// The H1 regression guard, named explicitly: a scanner reverted to
	// construction-literal scanning fails HERE, by name, rather than
	// silently going quiet.
	t.Run("helper-built request is still seen", func(t *testing.T) {
		fset := token.NewFileSet()
		path := filepath.Join(schemaVersionStampFixtureDir, "bad_pkg.go.txt")
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read bad fixture: %v", err)
		}
		sites, err := scanQdrantCalls(fset, src, path, fullWriteMethods)
		if err != nil {
			t.Fatalf("scan bad fixture: %v", err)
		}
		for _, s := range sites {
			if s.enclosingFunc == "writer.helperBuiltWrite" {
				return
			}
		}
		t.Fatal("bad fixture: writer.helperBuiltWrite (the H1 bypass — a request built by a separate helper function) was not seen; a scanner anchored on qdrant.PointStruct construction rather than the transmitted Upsert call would miss this write entirely")
	})

	// The cycle-2 regression guard, named explicitly: a scanner narrowed to
	// a .client receiver pattern fails HERE, by name.
	t.Run("delegation self-call is seen and distinguished by receiver path", func(t *testing.T) {
		fset := token.NewFileSet()
		path := filepath.Join(schemaVersionStampFixtureDir, "good_pkg.go.txt")
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read good fixture: %v", err)
		}
		sites, err := scanQdrantCalls(fset, src, path, fullWriteMethods)
		if err != nil {
			t.Fatalf("scan good fixture: %v", err)
		}
		byName := map[string]qdrantCallSite{}
		for _, s := range sites {
			byName[s.enclosingFunc] = s
		}
		delegation, ok := byName["writer.delegatingWrapper"]
		if !ok {
			t.Fatalf("good fixture: missing writer.delegatingWrapper")
		}
		direct, ok := byName["writer.Upsert"]
		if !ok {
			t.Fatalf("good fixture: missing writer.Upsert")
		}
		if delegation.receiver == direct.receiver {
			t.Errorf("delegation receiver %q must differ from direct receiver %q — a scanner narrowed to a .client receiver pattern would fail here", delegation.receiver, direct.receiver)
		}
	})

	t.Run("documented limit: method value and method expression are NOT seen", func(t *testing.T) {
		fset := token.NewFileSet()
		path := filepath.Join(schemaVersionStampFixtureDir, "limits_pkg.go.txt")
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read limits fixture: %v", err)
		}
		sites, err := scanQdrantCalls(fset, src, path, fullWriteMethods)
		if err != nil {
			t.Fatalf("scan limits fixture: %v", err)
		}
		if len(sites) != 0 {
			t.Errorf("limits fixture: got %d sites, want 0 (documented blind spot — a method value and a method expression are provably invisible to a selector-name scan; see this file's package doc comment): %v", len(sites), sites)
		}
	})

	t.Run("zero-applicability guard: nonexistent directory fails loudly", func(t *testing.T) {
		fset := token.NewFileSet()
		_, _, err := scanPackageDirForCalls(fset, filepath.Join(schemaVersionStampFixtureDir, "does-not-exist-zzz"), ".go.txt", "", fullWriteMethods)
		if err == nil {
			t.Fatal("scanPackageDirForCalls(nonexistent dir) = nil error, want a failure — a gate that silently scans nothing must not report clean")
		}
	})

	t.Run("zero-applicability guard: a directory with no matching files fails loudly", func(t *testing.T) {
		fset := token.NewFileSet()
		_, n, err := scanPackageDirForCalls(fset, schemaVersionStampFixtureDir, ".this-suffix-matches-nothing", "", fullWriteMethods)
		if err != nil {
			t.Fatalf("scanPackageDirForCalls with an impossible suffix returned an error instead of reporting zero: %v", err)
		}
		if n != 0 {
			t.Fatalf("scanPackageDirForCalls with an impossible suffix scanned %d files, want 0", n)
		}
		// The zero count itself is the guard's signal: every real caller of
		// this function (added in Task 2: the "real package" subtest,
		// TestPartialWritePathsAreClassifiedNonStamping,
		// TestQdrantClientIsHeldOnlyByStorePackage) treats filesScanned==0
		// as a t.Fatalf, never as "clean" — this subtest pins that
		// scanPackageDirForCalls itself reports the zero accurately rather
		// than masking it as an error or silently returning a nonzero
		// count.
	})
}
