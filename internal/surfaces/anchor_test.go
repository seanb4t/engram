// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package surfaces

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFixture writes content to a fresh file under t.TempDir() and returns
// its path — every test below starts from a synthetic fixture it controls,
// never the live repo tree (that is conformance_test.go's job).
func writeFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// TestReadRegionReversedSameLinePairIsMalformed is CR-01's read-side repro:
// a same-line anchor pair with the end literal appearing BEFORE the start
// literal must return a malformed-pairing error — never panic with a slice
// bounds out of range, and never silently return a garbled body.
func TestReadRegionReversedSameLinePairIsMalformed(t *testing.T) {
	content := "| flag | <!-- engram:rule:end r --> stuff <!-- engram:rule:start r --> |\n"
	path := writeFixture(t, content)

	body, ok, err := ReadRegion(path, "r")
	if err == nil {
		t.Fatalf("ReadRegion(reversed same-line pair) = (%q, %v, nil), want a malformed-pairing error", body, ok)
	}
	if ok {
		t.Errorf("ReadRegion(reversed same-line pair) ok = true, want false alongside the error")
	}
}

// TestWriteRegionReversedSameLinePairRefusesAndLeavesFileUntouched is CR-01's
// write-side repro: WriteRegion must REFUSE to write on the identical
// malformed input (never a silent nil-error corruption), and the refusal
// must leave the original file byte-for-byte untouched — the atomic-write
// contract the type's doc comment promises.
func TestWriteRegionReversedSameLinePairRefusesAndLeavesFileUntouched(t *testing.T) {
	content := "| flag | <!-- engram:rule:end r --> stuff <!-- engram:rule:start r --> |\n"
	path := writeFixture(t, content)

	err := WriteRegion(path, "r", "NEW TEXT")
	if err == nil {
		t.Fatal("WriteRegion(reversed same-line pair) = nil, want a malformed-pairing error")
	}

	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read fixture after refused write: %v", readErr)
	}
	if string(after) != content {
		t.Errorf("file content after refused write = %q, want unchanged %q", after, content)
	}
}

// TestScanAnchorsStartWithNoEnd proves an unterminated start anchor is a
// distinct error from the reversed-pairing case above, on both ReadRegion
// and WriteRegion.
func TestScanAnchorsStartWithNoEnd(t *testing.T) {
	content := "before\n<!-- engram:rule:start r -->\nbody\nafter\n"
	path := writeFixture(t, content)

	if _, _, err := ReadRegion(path, "r"); err == nil {
		t.Error("ReadRegion(start with no end) = nil error, want an error")
	}
	if err := WriteRegion(path, "r", "x"); err == nil {
		t.Error("WriteRegion(start with no end) = nil, want an error")
	}
}

// TestScanAnchorsEndWithNoStart proves an end anchor with no preceding start
// anchor is rejected, on both ReadRegion and WriteRegion.
func TestScanAnchorsEndWithNoStart(t *testing.T) {
	content := "before\n<!-- engram:rule:end r -->\nafter\n"
	path := writeFixture(t, content)

	if _, _, err := ReadRegion(path, "r"); err == nil {
		t.Error("ReadRegion(end with no start) = nil error, want an error")
	}
	if err := WriteRegion(path, "r", "x"); err == nil {
		t.Error("WriteRegion(end with no start) = nil, want an error")
	}
}

// TestScanAnchorsTwoStartsBeforeEnd proves a second start anchor appearing
// before the first's matching end anchor is rejected as malformed, on both
// ReadRegion and WriteRegion.
func TestScanAnchorsTwoStartsBeforeEnd(t *testing.T) {
	content := "<!-- engram:rule:start r -->\nfirst\n<!-- engram:rule:start r -->\nsecond\n<!-- engram:rule:end r -->\n"
	path := writeFixture(t, content)

	if _, _, err := ReadRegion(path, "r"); err == nil {
		t.Error("ReadRegion(two starts before end) = nil error, want an error")
	}
	if err := WriteRegion(path, "r", "x"); err == nil {
		t.Error("WriteRegion(two starts before end) = nil, want an error")
	}
}

// TestReadRegionAbsentAnchorReturnsFalseNotError proves a rule with no
// anchor at all in the file is NOT an error — ReadRegion's doc comment
// draws this distinction explicitly (absent vs malformed).
func TestReadRegionAbsentAnchorReturnsFalseNotError(t *testing.T) {
	content := "nothing relevant here\n"
	path := writeFixture(t, content)

	body, ok, err := ReadRegion(path, "r")
	if err != nil {
		t.Fatalf("ReadRegion(absent anchor) err = %v, want nil", err)
	}
	if ok {
		t.Errorf("ReadRegion(absent anchor) ok = true, want false")
	}
	if body != "" {
		t.Errorf("ReadRegion(absent anchor) body = %q, want empty", body)
	}
}

// TestReadWriteRegionInlineSameLinePair proves a well-formed inline
// (same-line, markdown-table-cell) pair round-trips correctly — the live
// pattern docs-site/src/content/docs/guides/cli.md and reference/tools.md
// both use — and that CR-01's fix does not regress it.
func TestReadWriteRegionInlineSameLinePair(t *testing.T) {
	content := "| flag | <!-- engram:rule:start r -->old text<!-- engram:rule:end r --> |\n"
	path := writeFixture(t, content)

	body, ok, err := ReadRegion(path, "r")
	if err != nil {
		t.Fatalf("ReadRegion(inline pair) err = %v, want nil", err)
	}
	if !ok {
		t.Fatal("ReadRegion(inline pair) ok = false, want true")
	}
	if body != "old text" {
		t.Errorf("ReadRegion(inline pair) body = %q, want %q", body, "old text")
	}

	if err := WriteRegion(path, "r", "new text"); err != nil {
		t.Fatalf("WriteRegion(inline pair) err = %v, want nil", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after write: %v", err)
	}
	want := "| flag | <!-- engram:rule:start r -->new text<!-- engram:rule:end r --> |\n"
	if string(after) != want {
		t.Errorf("file content after WriteRegion(inline pair) = %q, want %q", after, want)
	}
}

// TestReadWriteRegionMultiLinePair proves a well-formed multi-line pair (the
// proto `//` comment block pattern) round-trips correctly, including a
// replacement body that changes the region's own line count, and that the
// replacement is indented to match the start anchor line's own leading
// whitespace.
func TestReadWriteRegionMultiLinePair(t *testing.T) {
	content := "  // engram:rule:start r\n  // old line one\n  // engram:rule:end r\n  next_field = 1;\n"
	path := writeFixture(t, content)

	body, ok, err := ReadRegion(path, "r")
	if err != nil {
		t.Fatalf("ReadRegion(multi-line pair) err = %v, want nil", err)
	}
	if !ok {
		t.Fatal("ReadRegion(multi-line pair) ok = false, want true")
	}
	if body != "  // old line one" {
		t.Errorf("ReadRegion(multi-line pair) body = %q, want %q", body, "  // old line one")
	}

	if err := WriteRegion(path, "r", "new line one\nnew line two"); err != nil {
		t.Fatalf("WriteRegion(multi-line pair, line-count-changing body) err = %v, want nil", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after write: %v", err)
	}
	want := "  // engram:rule:start r\n  new line one\n  new line two\n  // engram:rule:end r\n  next_field = 1;\n"
	if string(after) != want {
		t.Errorf("file content after WriteRegion(multi-line pair) = %q, want %q", after, want)
	}
}

// TestWriteRegionMultiplePairsSameRuleID proves the proto case — the same
// rule ID anchored at more than one point in a single file (e.g. restated
// on two separate messages) — rewrites EVERY pair to the same new body, and
// that rewriting from the LAST pair backward does not corrupt an earlier
// pair's recorded line indices when a body replacement changes line count.
func TestWriteRegionMultiplePairsSameRuleID(t *testing.T) {
	content := "" +
		"message A {\n" +
		"  // engram:rule:start r\n" +
		"  // old\n" +
		"  // engram:rule:end r\n" +
		"  string category = 1;\n" +
		"}\n" +
		"message B {\n" +
		"  // engram:rule:start r\n" +
		"  // old\n" +
		"  // engram:rule:end r\n" +
		"  string category = 1;\n" +
		"}\n"
	path := writeFixture(t, content)

	if err := WriteRegion(path, "r", "new one\nnew two"); err != nil {
		t.Fatalf("WriteRegion(multiple pairs) err = %v, want nil", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after write: %v", err)
	}
	want := "" +
		"message A {\n" +
		"  // engram:rule:start r\n" +
		"  new one\n" +
		"  new two\n" +
		"  // engram:rule:end r\n" +
		"  string category = 1;\n" +
		"}\n" +
		"message B {\n" +
		"  // engram:rule:start r\n" +
		"  new one\n" +
		"  new two\n" +
		"  // engram:rule:end r\n" +
		"  string category = 1;\n" +
		"}\n"
	if string(after) != want {
		t.Errorf("file content after WriteRegion(multiple pairs) =\n%s\nwant\n%s", after, want)
	}

	// ReadRegion returns only the FIRST pair's body, by contract.
	body, ok, err := ReadRegion(path, "r")
	if err != nil {
		t.Fatalf("ReadRegion after multi-pair write err = %v, want nil", err)
	}
	if !ok {
		t.Fatal("ReadRegion after multi-pair write ok = false, want true")
	}
	if body != "  new one\n  new two" {
		t.Errorf("ReadRegion after multi-pair write body = %q, want %q", body, "  new one\n  new two")
	}
}

// TestWriteRegionUnwritableDirectoryLeavesOriginalIntact proves the
// atomic-rename contract holds under a real OS-level write failure, not
// just the malformed-input refusal case above: WriteRegion's doc comment
// claims an interrupted run "leaves either the previous or the new
// content, never a truncated file"; this drives that guarantee by making
// the temp-file creation itself fail (a read-only directory) and asserting
// the original file's bytes are unchanged.
func TestWriteRegionUnwritableDirectoryLeavesOriginalIntact(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root — a read-only directory does not block writes")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.md")
	content := "<!-- engram:rule:start r -->old<!-- engram:rule:end r -->\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod dir read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	err := WriteRegion(path, "r", "new")
	if err == nil {
		t.Fatal("WriteRegion into a read-only directory = nil, want an error")
	}

	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read fixture after failed write: %v", readErr)
	}
	if string(after) != content {
		t.Errorf("file content after failed write = %q, want unchanged %q", after, content)
	}
}
