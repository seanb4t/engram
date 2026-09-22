// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file closes the "correct by reading" property Decision B (D-01,
// 04-CONTEXT.md) depends on: a caller must be able to learn the recall
// count maximum from the wire schema, the CLI help, and the two published
// docs pages WITHOUT ever triggering a rejection. That property only stays
// true if every one of those surfaces states the SAME number as
// store.MaxRecallLimit forever -- so this gate derives the expected number
// from that one Go constant (never a second hardcoded literal) and asserts
// it, in numbers, on every documented surface, while also asserting that
// neither of D-01's two retired zero-value wordings has come back anywhere
// under internal/, cmd/, docs-site/src/, proto/, or CLAUDE.md -- see
// retiredPhraseServerDefault and retiredPhraseUnsetMeansAll below for the
// exact wordings, deliberately not quoted verbatim in this comment.

package server

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/seanb4t/engram/internal/store"
)

const (
	protoEngramPath = "../../proto/engram/v1/engram.proto"
	cliListGoPath   = "../../cmd/engram/client_list.go"
	cliSearchGoPath = "../../cmd/engram/client_search.go"
	docsCliPath     = "../../docs-site/src/content/docs/guides/cli.md"
)

// retiredPhraseServerDefault and retiredPhraseUnsetMeansAll reconstruct
// D-01's two retired wordings from fragments joined at runtime with "+",
// rather than as verbatim literals. Written as one contiguous literal,
// either phrase would make THIS FILE'S OWN negative sweep below match its
// own source the moment it is compiled into the sweep's target tree
// (internal/) -- the identical self-match trap
// 03-04-SUMMARY.md's "Issues Encountered" records for a different retired
// phrase. Do not "simplify" these back into single string literals; the
// split is load-bearing, not stylistic.
var (
	retiredPhraseServerDefault = "0 = " + "server default"
	retiredPhraseUnsetMeansAll = "an unset limit " + "means all"
)

// mustReadFile reads path relative to this test file's directory and fails
// the test (rather than silently skipping) if it cannot -- a documented
// surface this gate cannot read is a gate that proves nothing about it.
func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// protoMessageBlock extracts the body of `message <msgName> { ... }` from
// content, so a subtest can assert against the ONE field's comment site
// rather than the whole file -- load-bearing since engram.proto declares
// two DIFFERENT messages (SearchMemoriesRequest, SearchDiscoveriesRequest)
// with the textually-identical field signature `uint64 k = 3;`, and a
// whole-file search cannot distinguish which one drifted.
func protoMessageBlock(t *testing.T, content, msgName string) string {
	t.Helper()
	marker := "message " + msgName + " {"
	start := strings.Index(content, marker)
	if start < 0 {
		t.Fatalf("%s: no %q found", protoEngramPath, marker)
	}
	rest := content[start:]
	end := strings.Index(rest, "\n}")
	if end < 0 {
		t.Fatalf("%s: %s has no closing brace", protoEngramPath, msgName)
	}
	return rest[:end]
}

// lineContaining returns the first line of content containing marker, or
// fails the test if none exists -- a renamed marker must fail loudly
// rather than silently checking nothing (zero-applicability discipline,
// matching hintcodedocs_test.go's own guards).
func lineContaining(t *testing.T, content, marker, path string) string {
	t.Helper()
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, marker) {
			return line
		}
	}
	t.Fatalf("%s: no line contains marker %q", path, marker)
	return ""
}

// recallDocSurface is one documented location the recall count maximum
// must be stated at, numerically.
type recallDocSurface struct {
	name string
	// resolve returns the exact text this surface's assertion runs
	// against -- a whole file for the docs pages, or a narrower slice
	// (a proto message block, a single Go source line) for the sites
	// that share a file with another surface.
	resolve func(t *testing.T) (text, path string)
}

func recallDocSurfaces(t *testing.T) []recallDocSurface {
	t.Helper()
	surfaces := []recallDocSurface{
		{
			name: "proto_list_limit",
			resolve: func(t *testing.T) (string, string) {
				content := mustReadFile(t, protoEngramPath)
				block := protoMessageBlock(t, content, "ListMemoriesRequest")
				return lineContaining(t, block, "uint64 limit = 2;", protoEngramPath), protoEngramPath
			},
		},
		{
			name: "proto_search_memories_k",
			resolve: func(t *testing.T) (string, string) {
				content := mustReadFile(t, protoEngramPath)
				block := protoMessageBlock(t, content, "SearchMemoriesRequest")
				return lineContaining(t, block, "uint64 k = 3;", protoEngramPath), protoEngramPath
			},
		},
		{
			name: "proto_search_discoveries_k",
			resolve: func(t *testing.T) (string, string) {
				content := mustReadFile(t, protoEngramPath)
				block := protoMessageBlock(t, content, "SearchDiscoveriesRequest")
				return lineContaining(t, block, "uint64 k = 3;", protoEngramPath), protoEngramPath
			},
		},
		{
			name: "cli_list_limit_flag",
			resolve: func(t *testing.T) (string, string) {
				content := mustReadFile(t, cliListGoPath)
				return lineContaining(t, content, `"limit", 0,`, cliListGoPath), cliListGoPath
			},
		},
		{
			name: "cli_search_k_flag",
			resolve: func(t *testing.T) (string, string) {
				content := mustReadFile(t, cliSearchGoPath)
				return lineContaining(t, content, `"k", 0,`, cliSearchGoPath), cliSearchGoPath
			},
		},
		{
			name: "tools_md",
			resolve: func(t *testing.T) (string, string) {
				return mustReadFile(t, docsToolsPath), docsToolsPath
			},
		},
		{
			name: "cli_md",
			resolve: func(t *testing.T) (string, string) {
				return mustReadFile(t, docsCliPath), docsCliPath
			},
		},
	}
	if len(surfaces) == 0 {
		t.Fatal("recallDocSurfaces: zero surfaces declared -- zero-applicability guard tripped")
	}
	return surfaces
}

// TestRecallMaximumIsStatedNumerically is the D-01/D-02/D-03 gate: every
// documented recall-count surface states store.MaxRecallLimit numerically,
// and none of D-01's retired wordings has come back anywhere under
// internal/, cmd/, docs-site/src/, proto/, or CLAUDE.md.
func TestRecallMaximumIsStatedNumerically(t *testing.T) {
	maxStr := strconv.FormatUint(store.MaxRecallLimit, 10)
	if maxStr == "" {
		t.Fatal("strconv.FormatUint(store.MaxRecallLimit, 10) returned an empty string")
	}

	for _, s := range recallDocSurfaces(t) {
		s := s
		t.Run(s.name, func(t *testing.T) {
			text, path := s.resolve(t)
			if !strings.Contains(text, maxStr) {
				t.Errorf("%s: surface %q does not contain the documented maximum %q -- got %q", path, s.name, maxStr, text)
			}
		})
	}

	t.Run("negative_sweep_retired_wording", func(t *testing.T) {
		roots := []string{"../../internal", "../../cmd", "../../docs-site/src", "../../proto", "../../CLAUDE.md"}
		scanned := 0
		for _, root := range roots {
			info, err := os.Stat(root)
			if err != nil {
				t.Fatalf("stat %s: %v", root, err)
			}
			if !info.IsDir() {
				scanned++
				checkFileForRetiredWording(t, root)
				continue
			}
			walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if d.IsDir() {
					return nil
				}
				switch filepath.Ext(path) {
				case ".go", ".md", ".mdx", ".proto":
				default:
					return nil
				}
				scanned++
				checkFileForRetiredWording(t, path)
				return nil
			})
			if walkErr != nil {
				t.Fatalf("walk %s: %v", root, walkErr)
			}
		}
		if scanned == 0 {
			t.Fatal("negative sweep resolved zero files -- zero-applicability guard tripped")
		}
	})
}

// checkFileForRetiredWording fails t if path contains either of D-01's
// retired wordings, including path itself -- this file's own source must
// clear this check, which is exactly why the phrases above are built from
// fragments rather than written as contiguous literals.
func checkFileForRetiredWording(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	content := string(data)
	if strings.Contains(content, retiredPhraseServerDefault) {
		t.Errorf("%s: retired wording %q found", path, retiredPhraseServerDefault)
	}
	if strings.Contains(content, retiredPhraseUnsetMeansAll) {
		t.Errorf("%s: retired wording %q found", path, retiredPhraseUnsetMeansAll)
	}
}
