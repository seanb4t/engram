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
	"io/fs"
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

// partialWriteMethods is the method-name set for the partial-write boundary
// scan: every targeted payload mutation, which D-02 requires to NEVER stamp
// schema_version (a one-key merge cannot honestly claim currency).
var partialWriteMethods = map[string]bool{
	"SetPayload":       true,
	"DeletePayload":    true,
	"OverwritePayload": true,
}

// writeDisposition classifies a derived full-write call site.
type writeDisposition int

const (
	directConforming writeDisposition = iota
	conformingDelegation
	namedException
)

// writeClassification pairs one derived enclosing function with its expected
// receiver text and (for full writes) disposition, plus a non-empty
// justification string.
type writeClassification struct {
	enclosingFunc string
	receiver      string
	disposition   writeDisposition
	justification string
}

// fullWriteClassification is the write-boundary classification: exactly
// FOUR entries in THREE dispositions, derived from
// `rg -n '\.Upsert\(' internal/store/*.go --glob '!*_test.go'` at revision
// time (the scanner's OWN matching rule — no `client\.` prefix, since the
// receiver is deliberately ignored by the match). Two entries
// (Store.Update, Store.Supersede) are DELEGATIONS, not direct transmissions
// — the scanner's over-approximating match derives them as subjects too,
// and a delegating call is genuinely conforming: it routes through
// Store.Upsert and therefore through payload(), exactly as D-01 states.
var fullWriteClassification = []writeClassification{
	{
		enclosingFunc: "Store.Upsert", receiver: "s.client", disposition: directConforming,
		justification: "Its request payload derives from payload(), so every full record it transmits carries the monotonic stamp. Subject to payloadDerivesFromCodec below.",
	},
	{
		enclosingFunc: "Store.Update", receiver: "s", disposition: conformingDelegation,
		justification: "return s.Upsert(ctx, cur, vec) — it transmits nothing itself; it routes through Store.Upsert and therefore through payload(), exactly as D-01 states.",
	},
	{
		enclosingFunc: "Store.Supersede", receiver: "s", disposition: conformingDelegation,
		justification: "s.Upsert(ctx, newMem, vec) for the new correcting record — same reasoning as Store.Update.",
	},
	{
		enclosingFunc: "Store.Reindex", receiver: "s.client", disposition: namedException,
		justification: "It copies the raw payload map scrolled off the source collection verbatim (Payload: p.Payload) rather than rebuilding it from a decoded Memory, mirroring the existing embedder_identity exception comment at this write — which is what makes it preserve every key including ones this binary does not know about. Routing it through payload() would defeat that contract.",
	},
}

// partialWriteClassification is the partial-write classification: every
// SetPayload/DeletePayload/OverwritePayload call site in internal/store's
// package directory (store.go, spine.go, summarize.go), derived at revision
// time with the scanner's own matching rule — ten entries, named at the
// level the derivation reports (seams included), not at the level D-02's
// prose describes the callers. Every entry carries the same underlying
// justification: a one-key merge cannot honestly claim currency, so a
// partial write that stamped schema_version would assert "every key of this
// version is present" while having written only one — a false-currency
// claim that would make a future migration sweep skip records still needing
// migration (D-02).
var partialWriteClassification = []writeClassification{
	{enclosingFunc: "Store.UpdatePayload", receiver: "s.client", justification: "D-02: caller transmits directly; a partial write must never stamp currency it cannot honor."},
	{enclosingFunc: "Store.defaultDeletePayloadKeys", receiver: "s.client", justification: "D-02: the deletePayloadKeys SEAM's production implementation; a partial write must never stamp currency it cannot honor."},
	{enclosingFunc: "Store.defaultSetPayloadKeys", receiver: "s.client", justification: "D-02: the setPayloadKeys SEAM's production implementation; a partial write must never stamp currency it cannot honor."},
	{enclosingFunc: "Store.SetVisibility", receiver: "s.client", justification: "D-02: caller transmits directly; a partial write must never stamp currency it cannot honor."},
	{enclosingFunc: "Store.IncrementAccess", receiver: "s.client", justification: "D-02: caller transmits directly; a partial write must never stamp currency it cannot honor."},
	{enclosingFunc: "Store.BackfillShortIDs", receiver: "s.client", justification: "D-02: operator command; a partial write must never stamp currency it cannot honor."},
	{enclosingFunc: "Store.MigrateSetOwner", receiver: "s.client", justification: "D-02: operator command; a partial write must never stamp currency it cannot honor."},
	{enclosingFunc: "Store.RemapOwner", receiver: "s.client", justification: "D-02: operator command; a partial write must never stamp currency it cannot honor."},
	{enclosingFunc: "Store.Archive", receiver: "s.client", justification: "D-02: outside store.go (spine.go); a partial write must never stamp currency it cannot honor."},
	{enclosingFunc: "Store.SetSummary", receiver: "s.client", justification: "D-02: outside store.go (summarize.go); a partial write must never stamp currency it cannot honor."},
	{enclosingFunc: "Store.Migrate", receiver: "s.client", justification: "Phase 3's SANCTIONED exception to D-02's general rule, not a violation of it: every other entry here is a one-key merge that would falsely CLAIM currency it never verified, but Migrate's SetPayload runs only after applying every step in the record's own chain up to target, and its write map is built from AddedKeys(original, current) plus schemaVersionKey — the exact keys that chain declared and added. Stamping schema_version here is the correctly-earned currency claim this whole migration mechanism exists to make, not a shortcut around it."},
}

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

	// Task 2: apply the boundary scan to internal/store's own directory and
	// match the derived set against fullWriteClassification by SET
	// EQUALITY in both directions, plus per-entry receiver-text equality
	// and the conformance predicate on every direct-conforming entry.
	t.Run("real package", func(t *testing.T) {
		fset := token.NewFileSet()
		sites, filesScanned, err := scanPackageDirForCalls(fset, ".", ".go", "_test.go", fullWriteMethods)
		if err != nil {
			t.Fatalf("scan internal/store: %v", err)
		}
		if filesScanned == 0 {
			t.Fatal("scanned zero non-test .go files in internal/store — a package rename or empty directory must fail this gate, not report clean")
		}
		t.Logf("scanned %d non-test .go files in internal/store", filesScanned)

		got := map[string]qdrantCallSite{}
		for _, s := range sites {
			got[s.enclosingFunc] = s
		}
		want := map[string]writeClassification{}
		for _, c := range fullWriteClassification {
			want[c.enclosingFunc] = c
		}

		// Completeness by set equality, both directions.
		for name, site := range got {
			c, ok := want[name]
			if !ok {
				t.Errorf("derived write site %s (%s:%d) has no classification entry — every Upsert call site in internal/store's package directory must be classified", name, site.path, site.line)
				continue
			}
			if site.receiver != c.receiver {
				t.Errorf("classification entry %s: declared receiver %q, observed %q — a delegation entry whose receiver became s.client must be RECLASSIFIED as direct-conforming rather than have its expectation edited", name, c.receiver, site.receiver)
			}
		}
		for name := range want {
			if _, ok := got[name]; !ok {
				t.Errorf("classification entry %s has no matching derived site — stale classification entry (e.g. Store.Reindex refactored to route through payload() would leave this entry stale, hiding that it is no longer an exception)", name)
			}
		}

		// Conformance of the direct-conforming site(s), applied to every
		// direct-conforming entry and to NO conforming-delegation entry —
		// the exemption is checked, not assumed.
		storeFile := parseGoFile(t, "store.go")
		var directChecked, delegationSkipped int
		var wantDirect, wantDelegation int
		for _, c := range fullWriteClassification {
			switch c.disposition {
			case directConforming:
				wantDirect++
			case conformingDelegation:
				wantDelegation++
			}
		}
		for _, c := range fullWriteClassification {
			fn := findFuncDeclByDisplayName(storeFile, c.enclosingFunc)
			if fn == nil {
				t.Errorf("could not find FuncDecl for classification entry %s in store.go", c.enclosingFunc)
				continue
			}
			switch c.disposition {
			case directConforming:
				directChecked++
				if !payloadDerivesFromCodec(fn) {
					t.Errorf("%s: direct-conforming entry does not pass the conformance predicate — payload(m) may have been replaced with a hand-built map at the legitimate call site", c.enclosingFunc)
				}
			case conformingDelegation:
				delegationSkipped++
				// Exempt by construction: a delegation entry builds no
				// point of its own, so the conformance predicate is
				// deliberately never invoked on it in this branch.
			case namedException:
				// Reindex: not subject to the conformance predicate either
				// — its whole point is that it does NOT route through
				// payload().
			}
		}
		if directChecked != wantDirect {
			t.Errorf("applied the conformance predicate to %d direct-conforming entries, want %d", directChecked, wantDirect)
		}
		if delegationSkipped != wantDelegation {
			t.Errorf("skipped the conformance predicate for %d delegation entries, want %d", delegationSkipped, wantDelegation)
		}
	})
}

// TestPartialWritePathsAreClassifiedNonStamping is D-02's structural half:
// every SetPayload/DeletePayload/OverwritePayload call site in
// internal/store's package directory is derived and classified
// non-stamping with a D-02 justification, by set equality in both
// directions.
func TestPartialWritePathsAreClassifiedNonStamping(t *testing.T) {
	fset := token.NewFileSet()
	sites, filesScanned, err := scanPackageDirForCalls(fset, ".", ".go", "_test.go", partialWriteMethods)
	if err != nil {
		t.Fatalf("scan internal/store: %v", err)
	}
	if filesScanned == 0 {
		t.Fatal("scanned zero non-test .go files in internal/store — a scan that sees nothing must not report clean")
	}
	t.Logf("scanned %d non-test .go files in internal/store", filesScanned)

	got := map[string]qdrantCallSite{}
	for _, s := range sites {
		got[s.enclosingFunc] = s
	}
	want := map[string]writeClassification{}
	for _, c := range partialWriteClassification {
		want[c.enclosingFunc] = c
	}

	for name, site := range got {
		c, ok := want[name]
		if !ok {
			t.Errorf("derived partial-write site %s (%s:%d) has no classification entry — every SetPayload/DeletePayload/OverwritePayload site must be classified non-stamping with a D-02 justification", name, site.path, site.line)
			continue
		}
		if site.receiver != c.receiver {
			t.Errorf("partial-write classification entry %s: declared receiver %q, observed %q", name, c.receiver, site.receiver)
		}
	}
	for name, c := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("partial-write classification entry %s has no matching derived site — stale classification entry", name)
		}
		if c.justification == "" {
			t.Errorf("partial-write classification entry %s has no justification", name)
		}
	}
}

// qdrantClientHolder pairs one non-test .go file that names or constructs a
// *qdrant.Client with the justification for its allowlist membership.
type qdrantClientHolder struct {
	file          string // relative to the module root, forward-slash separated
	justification string
}

// qdrantClientHolderAllowlist is verified against source at revision time:
// exactly TWO members. internal/store/store.go is the one holder the
// write-boundary gate above scans. internal/server/tools.go is a
// COMPOSITION ROOT ONLY — it constructs the client and hands it straight to
// store.New without issuing a single Qdrant operation on it directly (its
// own d.st.Upsert(...) calls are calls to *store.Store's already-gated
// Upsert method, not to the qdrant.Client it briefly holds — see
// qdrantClientLocalNames's doc comment for why this distinction matters).
var qdrantClientHolderAllowlist = []qdrantClientHolder{
	{
		file:          "internal/store/store.go",
		justification: "The one holder: client *qdrant.Client field and New(c *qdrant.Client, ...) constructor. This is the package the write-boundary gate scans.",
	},
	{
		file:          "internal/server/tools.go",
		justification: "Composition root only: storeFromConfig constructs the client via qdrant.NewClient and hands it straight to store.New without issuing a single Qdrant operation on the client itself.",
	},
}

// findModuleRoot walks up from the working directory until go.mod is found,
// failing loudly if it never is — never silently scanning nothing.
func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found walking up from %s", dir)
		}
		dir = parent
	}
}

// fileRefsQdrantClient reports whether file either names the type
// qdrant.Client anywhere (a field, parameter, result, or variable
// declaration — ast.Inspect recurses through a leading *ast.StarExpr for
// the pointer form automatically) or calls qdrant.NewClient.
func fileRefsQdrantClient(file *ast.File) bool {
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if found {
			return false
		}
		switch t := n.(type) {
		case *ast.CallExpr:
			if sel, ok := t.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "qdrant" && sel.Sel.Name == "NewClient" {
					found = true
					return false
				}
			}
		case *ast.SelectorExpr:
			if pkg, ok := t.X.(*ast.Ident); ok && pkg.Name == "qdrant" && t.Sel.Name == "Client" {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// scanRepoForQdrantClientRefs locates the module root, then walks every .go
// file under it via filepath.WalkDir, skipping _test.go files and any gen/
// or vendor/ directory (plus dot-directories, for hygiene — .git and
// friends carry no Go source relevant here), and reports the module-root
// relative file paths that fileRefsQdrantClient flags, plus the count of
// non-test .go files actually scanned.
//
// This does NOT close the cross-package escape — a package could still
// receive a client through an interface or a wrapper without ever naming
// *qdrant.Client directly — but it converts the common, realistic form of
// it (a struct field or a direct dial) from silent to loud.
func scanRepoForQdrantClientRefs(fset *token.FileSet) (files []string, filesScanned int, err error) {
	root, err := findModuleRoot()
	if err != nil {
		return nil, 0, err
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
			if name == "gen" || name == "vendor" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", path, readErr)
		}
		file, parseErr := parser.ParseFile(fset, path, src, 0)
		if parseErr != nil {
			return fmt.Errorf("parse %s: %w", path, parseErr)
		}
		filesScanned++
		if fileRefsQdrantClient(file) {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = path
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if walkErr != nil {
		return nil, filesScanned, walkErr
	}
	sort.Strings(files)
	return files, filesScanned, nil
}

// qdrantClientLocalNames returns the identifier names in file that are bound
// DIRECTLY to a *qdrant.Client value returned by qdrant.NewClient(...) (the
// first assigned name in a `name, err := qdrant.NewClient(...)` shape). It
// exists so TestQdrantClientIsHeldOnlyByStorePackage's composition-root
// check can tell "the client variable itself issued a write" from "some
// OTHER value built from that client (e.g. the *store.Store returned by
// store.New) issued a write" — the latter is already covered by
// internal/store's own write-boundary gate above and must not be
// double-counted (and falsely flagged) here. Without this distinction, a
// blind method-name scan over internal/server/tools.go would flag its
// entirely legitimate d.st.Upsert(...) calls — calls to *store.Store's
// already-gated Upsert method — as if they transmitted directly to Qdrant.
func qdrantClientLocalNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, rhs := range assign.Rhs {
			call, ok := rhs.(*ast.CallExpr)
			if !ok {
				continue
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "qdrant" || sel.Sel.Name != "NewClient" {
				continue
			}
			// qdrant.NewClient returns (*Client, error); in a
			// `name, err := qdrant.NewClient(...)` shape, len(Rhs)==1 but
			// len(Lhs)==2 — index 0 of Lhs is the *Client, positionally.
			if i < len(assign.Lhs) {
				if id, ok := assign.Lhs[i].(*ast.Ident); ok {
					names[id.Name] = true
				}
			}
		}
		return true
	})
	return names
}

// TestQdrantClientIsHeldOnlyByStorePackage is the cross-package client
// guard: the largest escape the narrowed write-boundary claim leaves open
// is a writer in another package, and this converts it from silent to
// loud. It asserts the derived qdrant.Client-holder file set is set-equal
// to qdrantClientHolderAllowlist, AND that the composition-root allowlist
// entry (internal/server/tools.go) never itself calls a full-write or
// partial-write method ON THE CLIENT VARIABLE IT HOLDS — an allowlist entry
// that started transmitting would otherwise be permitted forever.
func TestQdrantClientIsHeldOnlyByStorePackage(t *testing.T) {
	fset := token.NewFileSet()
	files, filesScanned, err := scanRepoForQdrantClientRefs(fset)
	if err != nil {
		t.Fatalf("scanRepoForQdrantClientRefs: %v", err)
	}
	if filesScanned == 0 {
		t.Fatal("scanned zero non-test .go files across the module — a scan that sees nothing must not report clean")
	}
	t.Logf("scanned %d non-test .go files across the module", filesScanned)

	got := map[string]bool{}
	for _, f := range files {
		got[f] = true
	}
	want := map[string]qdrantClientHolder{}
	for _, e := range qdrantClientHolderAllowlist {
		want[e.file] = e
	}
	for f := range got {
		if _, ok := want[f]; !ok {
			t.Errorf("file %s holds/constructs a *qdrant.Client but is not in the allowlist — a write path outside internal/store may now exist", f)
		}
	}
	for name, e := range want {
		if !got[name] {
			t.Errorf("allowlist entry %s (%s) has no matching derived holder — stale allowlist entry", name, e.justification)
		}
		if e.justification == "" {
			t.Errorf("allowlist entry %s has no justification", name)
		}
	}

	// The composition-root entry must never itself issue a write ON THE
	// CLIENT VARIABLE IT HOLDS.
	root, err := findModuleRoot()
	if err != nil {
		t.Fatalf("findModuleRoot: %v", err)
	}
	toolsPath := filepath.Join(root, "internal", "server", "tools.go")
	toolsSrc, err := os.ReadFile(toolsPath)
	if err != nil {
		t.Fatalf("read %s: %v", toolsPath, err)
	}
	toolsFset := token.NewFileSet()
	toolsFile, err := parser.ParseFile(toolsFset, toolsPath, toolsSrc, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", toolsPath, err)
	}
	clientNames := qdrantClientLocalNames(toolsFile)
	if len(clientNames) == 0 {
		t.Fatalf("found no *qdrant.Client-bound identifier in %s — this composition-root check has nothing to verify against; the allowlist entry's own premise may be stale", toolsPath)
	}

	writeMethods := map[string]bool{}
	for m := range fullWriteMethods {
		writeMethods[m] = true
	}
	for m := range partialWriteMethods {
		writeMethods[m] = true
	}
	sites, scanErr := scanQdrantCalls(toolsFset, toolsSrc, toolsPath, writeMethods)
	if scanErr != nil {
		t.Fatalf("scan %s: %v", toolsPath, scanErr)
	}
	for _, s := range sites {
		if !clientNames[s.receiver] {
			continue // a call on some other value (e.g. *store.Store), not the qdrant.Client itself
		}
		t.Errorf("composition-root file %s issues a %s call on its own qdrant.Client (%s) at line %d (enclosing %s) — an allowlisted composition root must never itself transmit a write", toolsPath, s.method, s.receiver, s.line, s.enclosingFunc)
	}
}
