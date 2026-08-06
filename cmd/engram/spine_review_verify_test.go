// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanb4t/engram/internal/store"
)

// TestVerifyFileCitation is the table test over verifyFileCitation's tier
// classification, covering every behavior-list row plus a case reproducing
// GitHub issue #355's shape (an excerpt whose cited line range drifted
// because lines were inserted above it). No filesystem access anywhere in
// this test body -- every fileContent value is built inline.
func TestVerifyFileCitation(t *testing.T) {
	cases := []struct {
		name        string
		citation    store.Citation
		fileContent string
		fileExists  bool
		wantTier    string
		wantReason  string // empty means "don't check the exact reason"
	}{
		{
			name:     "kind commit is unverifiable, never skipped or valid",
			citation: store.Citation{Kind: "commit", Ref: "abc123"},
			wantTier: tierUnverifiable,
		},
		{
			name:     "kind url is unverifiable, never skipped or valid",
			citation: store.Citation{Kind: "url", Ref: "https://example.com"},
			wantTier: tierUnverifiable,
		},
		{
			name:     "kind repo is unverifiable, never skipped or valid",
			citation: store.Citation{Kind: "repo", Ref: "github.com/o/r"},
			wantTier: tierUnverifiable,
		},
		{
			name:       "file missing",
			citation:   store.Citation{Kind: "file", Ref: "gone.go", Excerpt: "func foo() {}", Locator: "1"},
			fileExists: false,
			wantTier:   tierBroken,
			wantReason: reasonFileMissing,
		},
		{
			name:        "empty excerpt is unverifiable, never valid",
			citation:    store.Citation{Kind: "file", Ref: "a.go", Locator: "1"},
			fileContent: "package a\n",
			fileExists:  true,
			wantTier:    tierUnverifiable,
			wantReason:  reasonNoExcerpt,
		},
		{
			name:        "at locator is valid",
			citation:    store.Citation{Kind: "file", Ref: "a.go", Locator: "2", Excerpt: "line two"},
			fileContent: "line one\nline two\nline three\n",
			fileExists:  true,
			wantTier:    tierValid,
		},
		{
			name: "excerpt starts at the locator but overruns its end line -- valid, not moved",
			citation: store.Citation{
				Kind: "file", Ref: "a.go", Locator: "2-2",
				Excerpt: "line two\nline three",
			},
			fileContent: "line one\nline two\nline three\n",
			fileExists:  true,
			wantTier:    tierValid,
		},
		{
			name:        "moved: excerpt found elsewhere in the same file",
			citation:    store.Citation{Kind: "file", Ref: "a.go", Locator: "1", Excerpt: "line three"},
			fileContent: "line one\nline two\nline three\n",
			fileExists:  true,
			wantTier:    tierMoved,
		},
		{
			name:        "excerpt gone entirely from the file",
			citation:    store.Citation{Kind: "file", Ref: "a.go", Locator: "1", Excerpt: "nowhere to be found"},
			fileContent: "line one\nline two\nline three\n",
			fileExists:  true,
			wantTier:    tierBroken,
			wantReason:  reasonExcerptGone,
		},
		{
			name:        "unparseable locator falls through to the same-file search, not an error",
			citation:    store.Citation{Kind: "file", Ref: "a.go", Locator: "not-a-number", Excerpt: "line two"},
			fileContent: "line one\nline two\nline three\n",
			fileExists:  true,
			wantTier:    tierMoved,
		},
		{
			name:        "locator past the end of the file falls through to the same-file search",
			citation:    store.Citation{Kind: "file", Ref: "a.go", Locator: "999", Excerpt: "line two"},
			fileContent: "line one\nline two\nline three\n",
			fileExists:  true,
			wantTier:    tierMoved,
		},
		{
			// GitHub issue #355: lines inserted above the cited range shift
			// the excerpt to a later byte offset in the SAME file --
			// ordinary drift, not breakage.
			name:        "issue 355: excerpt drifted after lines were inserted above it",
			citation:    store.Citation{Kind: "file", Ref: "a.go", Locator: "1", Excerpt: "target line"},
			fileContent: "inserted line one\ninserted line two\ntarget line\n",
			fileExists:  true,
			wantTier:    tierMoved,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := verifyFileCitation(tc.citation, tc.fileContent, tc.fileExists)
			if got.Tier != tc.wantTier {
				t.Errorf("Tier = %q, want %q (reason=%q)", got.Tier, tc.wantTier, got.Reason)
			}
			if tc.wantReason != "" && got.Reason != tc.wantReason {
				t.Errorf("Reason = %q, want %q", got.Reason, tc.wantReason)
			}
			if got.Ref != tc.citation.Ref {
				t.Errorf("Ref = %q, want %q", got.Ref, tc.citation.Ref)
			}
		})
	}
}

// TestExcerptOffsetAt is the direct unit test over excerptOffsetAt's
// locator parsing: a bare line number, a "start-end" range (only the start
// matters), and every -1 fallthrough case (empty, unparseable, zero, past
// end of file).
func TestExcerptOffsetAt(t *testing.T) {
	content := "line one\nline two\nline three\n"
	cases := []struct {
		name    string
		locator string
		want    int
	}{
		{"empty locator", "", -1},
		{"bare line one", "1", 0},
		{"bare line two", "2", len("line one\n")},
		{"range uses only the start line", "2-3", len("line one\n")},
		{"unparseable", "abc", -1},
		{"past end of file", "100", -1},
		{"zero is invalid (1-indexed)", "0", -1},
		{"negative is invalid", "-5", -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := excerptOffsetAt(content, tc.locator); got != tc.want {
				t.Errorf("excerptOffsetAt(_, %q) = %d, want %d", tc.locator, got, tc.want)
			}
		})
	}
}

// TestNormalizeGitRemote is the table test over repoIdentityFromCWD's
// normaliser: every form the plan names -- HTTPS, SCP-style, ssh://, and a
// bare already-normalised path -- must produce the IDENTICAL host/path
// string when they name the same repo.
func TestNormalizeGitRemote(t *testing.T) {
	const want = "github.com/o/r"
	cases := []string{
		"https://github.com/o/r.git",
		"git@github.com:o/r.git",
		"ssh://git@github.com/o/r",
		"github.com/o/r",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			if got := normalizeGitRemote(c); got != want {
				t.Errorf("normalizeGitRemote(%q) = %q, want %q", c, got, want)
			}
		})
	}
}

// TestSameRepoAsCWD is the table test over the EXACT-SEGMENT scope
// comparison rule, using THIS repo's real scope shapes so the verifier's
// first live run on the #355 fixture cannot cry "different repo" at its own
// calibration data.
func TestSameRepoAsCWD(t *testing.T) {
	const identity = "github.com/seanb4t/engram"

	sameRepo := []string{
		"repo:github.com/seanb4t/engram",
		"discovery:repo:github.com/seanb4t/engram",
		"rule:repo:github.com/seanb4t/engram",
		"repo:github.com/seanb4t/engram:ws:worktree-engram-mbnw",
	}
	for _, scope := range sameRepo {
		t.Run("same_repo/"+scope, func(t *testing.T) {
			if !sameRepoAsCWD(scope, identity) {
				t.Errorf("sameRepoAsCWD(%q, %q) = false, want true", scope, identity)
			}
		})
	}

	differentRepo := []string{
		// The substring false-positive this rule exists to prevent: no
		// segment is EXACTLY "repo".
		"myrepo:github.com/seanb4t/engram",
		"repo:github.com/other/thing",
		"project:engram",
	}
	for _, scope := range differentRepo {
		t.Run("different_repo/"+scope, func(t *testing.T) {
			if sameRepoAsCWD(scope, identity) {
				t.Errorf("sameRepoAsCWD(%q, %q) = true, want false", scope, identity)
			}
		})
	}
}

// resolvedTempDir returns a fresh, symlink-resolved temp directory.
// t.TempDir() alone is not enough on macOS, where TMPDIR sits under a
// symlinked path (/tmp -> /private/tmp): resolveContainedRef's containment
// check compares an EvalSymlinks-resolved candidate against root, so a raw,
// unresolved root would make every same-directory comparison in this file's
// tests fail containment for a reason that has nothing to do with what each
// test actually means to exercise. Mirrors verifyWorkingRoot's own
// EvalSymlinks step in production.
func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("filepath.EvalSymlinks(%q): %v", dir, err)
	}
	return resolved
}

// TestVerifyRejectsAbsoluteRef proves an absolute Ref classifies
// unverifiable, never a confident wrong verdict.
func TestVerifyRejectsAbsoluteRef(t *testing.T) {
	root := resolvedTempDir(t)
	records := []store.CitationRecord{{
		ID: "rec-1", Scope: "repo:github.com/o/r",
		Citations: []store.Citation{{Kind: "file", Ref: "/etc/passwd", Excerpt: "root:x"}},
	}}
	report := runVerify(records, "github.com/o/r", root, time.Now())
	if report.UnverifiableCount != 1 {
		t.Fatalf("UnverifiableCount = %d, want 1 (absolute ref must be rejected)", report.UnverifiableCount)
	}
}

// TestVerifyRejectsTraversalEscape proves a relative Ref containing
// parent-directory segments that escape the working directory classifies
// unverifiable.
func TestVerifyRejectsTraversalEscape(t *testing.T) {
	root := resolvedTempDir(t)
	records := []store.CitationRecord{{
		ID: "rec-1", Scope: "repo:github.com/o/r",
		Citations: []store.Citation{{Kind: "file", Ref: "../outside.txt", Excerpt: "secret"}},
	}}
	report := runVerify(records, "github.com/o/r", root, time.Now())
	if report.UnverifiableCount != 1 {
		t.Fatalf("UnverifiableCount = %d, want 1 (traversal escape must be rejected)", report.UnverifiableCount)
	}
}

// TestVerifyRejectsSymlinkEscape proves the path-safety gate is RESOLVED,
// not lexical: a relative Ref containing no ".." segments at all, but
// traversing a symlink that points outside the working tree, must still be
// rejected -- and must never reach a read.
//
// This is a MUTATION CHECK (inject-and-revert), not RED-first: Task 2
// builds the RESOLVED gate directly, so the lexical-only gate this proves
// falsifiable against is never built in task order and the failure state
// never arises naturally. See the plan SUMMARY for the observed failure
// line from temporarily swapping resolveContainedRef's body for the
// lexical absolute-plus-".." check.
func TestVerifyRejectsSymlinkEscape(t *testing.T) {
	outsideDir := resolvedTempDir(t)
	if err := os.WriteFile(filepath.Join(outsideDir, "passwd"), []byte("secret"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	root := resolvedTempDir(t)
	if err := os.Symlink(outsideDir, filepath.Join(root, "link")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	calls := 0
	orig := citationFileReader
	citationFileReader = func(path string) (string, bool) {
		calls++
		return orig(path)
	}
	t.Cleanup(func() { citationFileReader = orig })

	records := []store.CitationRecord{{
		ID: "rec-1", Scope: "repo:github.com/o/r",
		Citations: []store.Citation{{Kind: "file", Ref: "link/passwd", Excerpt: "secret"}},
	}}
	report := runVerify(records, "github.com/o/r", root, time.Now())
	if report.UnverifiableCount != 1 {
		t.Fatalf("UnverifiableCount = %d, want 1 (symlink escape must be rejected)", report.UnverifiableCount)
	}
	if calls != 0 {
		t.Errorf("citationFileReader was called %d time(s), want 0 -- a symlink-escaping ref must never reach a read", calls)
	}
}

// TestVerifyReadsRefAtMostOnce proves a Ref cited by three different
// records triggers exactly one file read.
func TestVerifyReadsRefAtMostOnce(t *testing.T) {
	root := resolvedTempDir(t)
	if err := os.WriteFile(filepath.Join(root, "shared.go"), []byte("package p\nfunc F() {}\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	calls := 0
	orig := citationFileReader
	citationFileReader = func(path string) (string, bool) {
		calls++
		return orig(path)
	}
	t.Cleanup(func() { citationFileReader = orig })

	cite := store.Citation{Kind: "file", Ref: "shared.go", Locator: "2", Excerpt: "func F"}
	records := []store.CitationRecord{
		{ID: "rec-1", Scope: "repo:github.com/o/r", Citations: []store.Citation{cite}},
		{ID: "rec-2", Scope: "repo:github.com/o/r", Citations: []store.Citation{cite}},
		{ID: "rec-3", Scope: "repo:github.com/o/r", Citations: []store.Citation{cite}},
	}
	report := runVerify(records, "github.com/o/r", root, time.Now())
	if report.ValidCount != 3 {
		t.Fatalf("ValidCount = %d, want 3", report.ValidCount)
	}
	if calls != 1 {
		t.Errorf("citationFileReader was called %d time(s), want 1 (three records cite the same Ref)", calls)
	}
}

// TestVerifyReportNeverIncludesExcerpt is T-03-14's regression: neither the
// text summary nor the JSON document may ever contain a citation's Excerpt
// text, proven against a distinctive sentinel string.
func TestVerifyReportNeverIncludesExcerpt(t *testing.T) {
	const sentinel = "TOTALLY-SECRET-EXCERPT-CONTENT-zzzqxr"
	report := runVerify([]store.CitationRecord{
		{ID: "rec-1", Scope: "repo:github.com/o/r", Citations: []store.Citation{
			{Kind: "file", Ref: "gone.go", Excerpt: sentinel},
		}},
	}, "github.com/o/r", resolvedTempDir(t), time.Now())
	if report.BrokenCount != 1 {
		t.Fatalf("BrokenCount = %d, want 1", report.BrokenCount)
	}

	summary := verifySummary(report)
	if strings.Contains(summary, sentinel) {
		t.Errorf("verifySummary leaked the excerpt sentinel: %q", summary)
	}

	b, err := json.Marshal(verifyDoc(report))
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(b), sentinel) {
		t.Errorf("verifyDoc leaked the excerpt sentinel: %s", b)
	}
}

// TestVerifyDocEmptyResultMarshalsEmptyArrays proves every entry slice is
// non-nil even for a zero-finding report: JSON mode must emit "[]", never
// "null".
func TestVerifyDocEmptyResultMarshalsEmptyArrays(t *testing.T) {
	b, err := json.Marshal(verifyDoc(verifyReport{}))
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	for _, want := range []string{`"moved_entries":[]`, `"broken_entries":[]`, `"unverifiable_entries":[]`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("marshaled doc = %s, want it to contain %q", b, want)
		}
	}
}

// TestVerifySummaryFormat pins the pure text formatter's shape: a single
// trailing-newline-free string naming tier counts and every non-valid
// entry.
func TestVerifySummaryFormat(t *testing.T) {
	report := verifyReport{
		ValidCount: 3, MovedCount: 1, BrokenCount: 1, UnverifiableCount: 1,
		Moved:        []verifyEntry{{RecordID: "r1", ShortID: "s1", Ref: "a.go", Reason: "excerpt found at byte offset 5, not at the cited locator"}},
		Broken:       []verifyEntry{{RecordID: "r2", ShortID: "s2", Ref: "b.go", Reason: reasonFileMissing}},
		Unverifiable: []verifyEntry{{RecordID: "r3", ShortID: "s3", Ref: "c.go", Reason: "different repo"}},
	}
	got := verifySummary(report)
	if strings.HasSuffix(got, "\n") {
		t.Errorf("verifySummary result ends with a trailing newline, want none: %q", got)
	}
	for _, want := range []string{"valid=3", "moved=1", "broken=1", "unverifiable=1", "r1", "r2", "r3", "different repo"} {
		if !strings.Contains(got, want) {
			t.Errorf("verifySummary(report) = %q, want it to contain %q", got, want)
		}
	}
}

// TestSpineReviewVerifyRejectsInvalidOutput proves `spine-review verify`
// validates --output through operatorOutputFormat, exactly as `scan` does.
func TestSpineReviewVerifyRejectsInvalidOutput(t *testing.T) {
	resetClientFlags(t)
	_, _, err := runClient(t, "spine-review", "verify", "--all-scopes", "--output", "yaml")
	if err == nil {
		t.Fatal("expected an error for --output yaml, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}

// TestSpineReviewVerifyRequiresScopeOrAllScopes pins the bare usageErrorf
// guard (mirroring spine-review scan's exact wording) that rejects a verify
// invocation naming neither --scope nor --all-scopes.
func TestSpineReviewVerifyRequiresScopeOrAllScopes(t *testing.T) {
	resetClientFlags(t)
	_, _, err := runClient(t, "spine-review", "verify")
	if err == nil {
		t.Fatal("expected an error when neither --scope nor --all-scopes is supplied, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}
