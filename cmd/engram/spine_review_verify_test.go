// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"testing"

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
