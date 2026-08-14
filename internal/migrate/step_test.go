// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package migrate

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// someApplyForPanicTests is a placeholder ApplyFunc used only by the panic
// tests below, where the point under test is the OTHER parameter — a real
// apply body would just add noise to what each test is proving.
func someApplyForPanicTests(payload map[string]any) (map[string]any, error) {
	return payload, nil
}

// TestNewStepPanicsOnNilReversibility proves the load-bearing line SC3
// actually rests on. Positional-required parameters stop a caller from
// OMITTING rev — the Go compiler already rejects a call with too few
// arguments on its own, trivially. What the compiler does NOT reject is an
// explicit nil assigned to rev: Go permits nil for any interface-typed
// parameter no matter how sealed that interface is (RESEARCH Pitfall 1). A
// test whose only reversibility case is argument omission proves nothing
// about this; this test exercises the explicit-nil case, which is the one
// that actually makes "nobody declared reversibility" unrepresentable.
func TestNewStepPanicsOnNilReversibility(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewStep(from, to, addsKeys, nil, apply) did not panic — the Go compiler already rejects a call that OMITS rev, but it accepts this explicit nil for an interface-typed parameter regardless of sealing; this panic, not the signature and not the seal, is what makes SC3's 'nobody thought about reversibility' unrepresentable")
		}
	}()
	NewStep(0, 1, []string{"k"}, nil, someApplyForPanicTests)
}

// TestNewStepPanicsOnNilApplyFunc mirrors the above for the other
// interface-shaped required parameter: an explicit nil ApplyFunc is
// assignable regardless of NewStep's positional signature, so the nil check
// is what closes this hatch too.
func TestNewStepPanicsOnNilApplyFunc(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewStep(from, to, addsKeys, rev, nil) did not panic — a step with no declared transformation is not a representable state")
		}
	}()
	NewStep(0, 1, []string{"k"}, Irreversible("x"), nil)
}

// TestIrreversiblePanicsOnEmptyReason copies TestRemapFromPanicsOnEmptyValue's
// shape exactly (internal/store/store_test.go): a defer/recover wrapper, one
// direct call, no table. This proves only the panics-on-bad-input half of
// D-03 (PA-1) — it does NOT prove the package-init-order claim, which
// remains Phase 4's to keep true once the registry is non-empty.
func TestIrreversiblePanicsOnEmptyReason(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Irreversible(\"\") did not panic; an irreversible step naming no reason leaves an operator staring at a failed revert request with no explanation of why it refused")
		}
	}()
	Irreversible("")
}

// TestReversiblePanicsOnNilInverse mirrors Irreversible's panic proof for
// the other constructor: a step claiming to be reversible with no actual
// inverse ApplyFunc is not a representable state.
func TestReversiblePanicsOnNilInverse(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Reversible(nil) did not panic; a step claiming to be reversible with no actual inverse is not a representable state")
		}
	}()
	Reversible(nil)
}

// receiverBaseTypeName returns the unqualified base type name of a method
// receiver (fn.Recv), unwrapping a pointer receiver's *ast.StarExpr, and
// false if the receiver shape is not a single identifier (a generic
// receiver, or something this gate does not expect).
func receiverBaseTypeName(recv *ast.FieldList) (string, bool) {
	if recv == nil || len(recv.List) != 1 {
		return "", false
	}
	typ := recv.List[0].Type
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}
	// A generic receiver like `T[P]` parses as *ast.IndexExpr /
	// *ast.IndexListExpr wrapping the base ident; unwrap those too so a
	// future generic carrier type does not silently escape this gate.
	switch e := typ.(type) {
	case *ast.Ident:
		return e.Name, true
	case *ast.IndexExpr:
		if ident, ok := e.X.(*ast.Ident); ok {
			return ident.Name, true
		}
	case *ast.IndexListExpr:
		if ident, ok := e.X.(*ast.Ident); ok {
			return ident.Name, true
		}
	}
	return "", false
}

// nonTestGoFiles returns the non-test .go files directly inside dir
// (relative to the package directory the test process runs from — go test
// sets the working directory to the package under test). A missing
// directory is a t.Fatal in the caller, never a silent empty result: a scan
// that cannot see what it was told to check must never report clean
// (durable record x6v6qxqd6f, and internal/store's scanPackageDir applies
// the identical discipline).
func nonTestGoFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	return files
}

// TestReversibilityIsSealedToThisPackage proves the seal three ways, from
// weakest to strongest evidence. Its limits are stated at their real
// strength (PA-3): it says nothing about a future exported FUNCTION
// returning one of the unexported carrier types (harmless — the value is
// still constructed inside this package), and nothing about reflection
// (calling an unexported method via reflection from another package fails
// the same way a static call would).
func TestReversibilityIsSealedToThisPackage(t *testing.T) {
	t.Run("interface_shape_is_exactly_one_unexported_method", func(t *testing.T) {
		typ := reflect.TypeOf((*Reversibility)(nil)).Elem()
		if typ.NumMethod() != 1 {
			t.Fatalf("migrate.Reversibility has %d methods, want exactly 1 — a sealed interface that silently grew a second, exported method would stop being sealed", typ.NumMethod())
		}
		m := typ.Method(0)
		if m.PkgPath == "" {
			t.Fatalf("migrate.Reversibility's sole method %q has an empty PkgPath, meaning it is EXPORTED — an exported method is satisfiable from any package and would void the seal entirely", m.Name)
		}
	})

	// The mechanical half: no exported carrier of the marker method exists
	// in this package today. This closes the embedding escape hatch (an
	// exported struct embedding a carrier would re-open the seal via
	// promoted-method interface satisfaction) but proves nothing about an
	// out-of-package TYPE implementing the seal — that is the build probe
	// below.
	t.Run("no_exported_carrier_of_isReversibility", func(t *testing.T) {
		files := nonTestGoFiles(t, ".")
		if len(files) == 0 {
			t.Fatal("scanned zero non-test .go files in internal/migrate — a scan that sees nothing is vacuously green (durable record x6v6qxqd6f); this is a defect in the scan, not evidence the seal holds")
		}

		fset := token.NewFileSet()
		var found int
		for _, path := range files {
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv == nil || fn.Name == nil || fn.Name.Name != "isReversibility" {
					continue
				}
				found++
				name, ok := receiverBaseTypeName(fn.Recv)
				if !ok {
					t.Fatalf("%s: isReversibility has a receiver shape this gate cannot parse; update receiverBaseTypeName", path)
				}
				if ast.IsExported(name) {
					t.Fatalf("%s: isReversibility is declared on EXPORTED type %q — an exported carrier of the marker method would re-open the seal via embedding, the one documented escape hatch this pattern has (RESEARCH Pattern 1)", path, name)
				}
			}
		}
		if found == 0 {
			t.Fatal("found zero isReversibility declarations in internal/migrate — a gate that found nothing to check is vacuously green, the exact defect class durable record x6v6qxqd6f already caught once this milestone")
		}
		if found != 2 {
			t.Fatalf("found %d isReversibility declarations, want exactly 2 (reversibleStep, irreversibleStep) — a count that has drifted from 2 means a carrier type was added or removed without this gate being updated to match", found)
		}
	})

	// The strong half: an out-of-package TYPE declaring its own method
	// named isReversibility does not satisfy migrate.Reversibility, proven
	// by an OBSERVED build failure rather than by reasoning about Go's
	// unexported-method identity rule. Three outcomes are discriminated by
	// three distinct messages (review cycle 1) so an environment problem is
	// never read as a broken seal, and a broken seal is never read as an
	// unrelated build failure.
	t.Run("out_of_package_implementor_fails_to_build", func(t *testing.T) {
		// First, probe the toolchain itself. If this fails, the seal's
		// status is UNKNOWN — not proven and not disproven — and PA-2
		// requires this to be a loud failure, never a skip.
		if out, err := exec.Command("go", "env", "GOROOT").CombinedOutput(); err != nil {
			t.Fatalf("could not execute the Go toolchain (`go env GOROOT` failed: %v; output: %s) — the seal's status is UNKNOWN, neither proven nor disproven, by this run", err, out)
		}

		probeSrc, err := os.ReadFile(filepath.Join("testdata", "sealedprobe", "probe.go.txt"))
		if err != nil {
			t.Fatalf("read testdata/sealedprobe/probe.go.txt: %v", err)
		}

		// The probe directory must live INSIDE this module (under
		// internal/migrate/) for its import of
		// github.com/seanb4t/engram/internal/migrate to resolve (PA-2).
		tmpDir, err := os.MkdirTemp(".", "sealedprobe-")
		if err != nil {
			t.Fatalf("create temp probe directory under internal/migrate/: %v", err)
		}
		t.Cleanup(func() {
			if rmErr := os.RemoveAll(tmpDir); rmErr != nil {
				t.Errorf("cleanup: remove %s: %v", tmpDir, rmErr)
			}
		})

		if err := os.WriteFile(filepath.Join(tmpDir, "probe.go"), probeSrc, 0o644); err != nil {
			t.Fatalf("write probe.go into %s: %v", tmpDir, err)
		}

		cmd := exec.Command("go", "build", ".")
		cmd.Dir = tmpDir
		output, buildErr := cmd.CombinedOutput()

		if buildErr == nil {
			t.Fatalf("out-of-package implementor at testdata/sealedprobe/probe.go.txt COMPILED with no error — the seal is OPEN: a type outside internal/migrate satisfied migrate.Reversibility by declaring its own isReversibility method")
		}
		if !strings.Contains(string(output), "isReversibility") {
			t.Fatalf("probe build failed, but its output does not name isReversibility — this is NOT evidence the seal holds, it is a build failure for an unrelated reason (e.g. a fixture typo or a stale temp directory); full output:\n%s", output)
		}
		// Non-zero exit naming isReversibility: the seal holds, exactly as
		// expected — Go's identity for an unexported method includes the
		// declaring package, so a method named isReversibility declared in
		// package sealedprobe is a DIFFERENT method from the one
		// migrate.Reversibility names, and does not satisfy it.
	})
}
