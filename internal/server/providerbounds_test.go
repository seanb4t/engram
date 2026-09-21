// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// providerbounds_test.go proves two things about the six provider-bound
// config helpers (embedDrainBytes/embedDrainTimeout/embedMaxTimeout,
// summaryDrainBytes/summaryDrainTimeout/summaryMaxTimeout): each parses and
// defaults correctly in isolation, and each is actually PASSED to the
// client it belongs to at construction.
//
// TestProviderBoundOptionsWiredIntoBothClients parses this package's own
// tools.go with go/parser rather than asserting on a constructed
// *embed.Client/*summarize.Client, because the fields those options set
// (drainBytes, drainTimeout, maxTimeout) are unexported and live in another
// package — from here, a helper that exists but is never passed to
// embed.New/summarize.New is indistinguishable from one that is correctly
// wired. A test built on the helpers alone would pass while the bounds
// never reached the client at all (T-07-04-01).
package server

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"time"

	"github.com/seanb4t/engram/internal/config"
)

func TestProviderBoundHelpersParseAndDefault(t *testing.T) {
	t.Run("embedDrainBytes", func(t *testing.T) {
		for _, c := range []struct {
			name  string
			value string
			want  int64
		}{
			{"empty uses default", "", 262144},
			{"well-formed value returned verbatim", "500000", 500000},
			{"malformed value falls back to default", "not-a-number", 262144},
			{"negative value falls back to default", "-1", 262144},
			{"zero is honored, not swallowed", "0", 0},
		} {
			t.Run(c.name, func(t *testing.T) {
				cfg := &config.Config{Embed: config.EmbedConfig{DrainBytes: c.value}}
				if got := embedDrainBytes(cfg); got != c.want {
					t.Errorf("embedDrainBytes(%q) = %d, want %d", c.value, got, c.want)
				}
			})
		}
	})

	t.Run("summaryDrainBytes", func(t *testing.T) {
		for _, c := range []struct {
			name  string
			value string
			want  int64
		}{
			{"empty uses default", "", 262144},
			{"well-formed value returned verbatim", "500000", 500000},
			{"malformed value falls back to default", "not-a-number", 262144},
			{"negative value falls back to default", "-1", 262144},
			{"zero is honored, not swallowed", "0", 0},
		} {
			t.Run(c.name, func(t *testing.T) {
				cfg := &config.Config{Summarize: config.SummarizeConfig{DrainBytes: c.value}}
				if got := summaryDrainBytes(cfg); got != c.want {
					t.Errorf("summaryDrainBytes(%q) = %d, want %d", c.value, got, c.want)
				}
			})
		}
	})

	t.Run("embedDrainTimeout", func(t *testing.T) {
		for _, c := range []struct {
			name  string
			value string
			want  time.Duration
		}{
			{"empty uses default", "", 2 * time.Second},
			{"well-formed value returned verbatim", "5s", 5 * time.Second},
			{"malformed value falls back to default", "not-a-duration", 2 * time.Second},
			{"negative value falls back to default", "-1s", 2 * time.Second},
			{"zero is honored, not swallowed", "0s", 0},
		} {
			t.Run(c.name, func(t *testing.T) {
				cfg := &config.Config{Embed: config.EmbedConfig{DrainTimeout: c.value}}
				if got := embedDrainTimeout(cfg); got != c.want {
					t.Errorf("embedDrainTimeout(%q) = %v, want %v", c.value, got, c.want)
				}
			})
		}
	})

	t.Run("summaryDrainTimeout", func(t *testing.T) {
		for _, c := range []struct {
			name  string
			value string
			want  time.Duration
		}{
			{"empty uses default", "", 2 * time.Second},
			{"well-formed value returned verbatim", "5s", 5 * time.Second},
			{"malformed value falls back to default", "not-a-duration", 2 * time.Second},
			{"negative value falls back to default", "-1s", 2 * time.Second},
			{"zero is honored, not swallowed", "0s", 0},
		} {
			t.Run(c.name, func(t *testing.T) {
				cfg := &config.Config{Summarize: config.SummarizeConfig{DrainTimeout: c.value}}
				if got := summaryDrainTimeout(cfg); got != c.want {
					t.Errorf("summaryDrainTimeout(%q) = %v, want %v", c.value, got, c.want)
				}
			})
		}
	})

	t.Run("embedMaxTimeout", func(t *testing.T) {
		for _, c := range []struct {
			name  string
			value string
			want  time.Duration
		}{
			{"empty uses default", "", 10 * time.Minute},
			{"well-formed value returned verbatim", "15m", 15 * time.Minute},
			{"malformed value falls back to default", "not-a-duration", 10 * time.Minute},
			{"negative value falls back to default", "-1m", 10 * time.Minute},
			{"zero falls back to default (ceiling never honors zero)", "0s", 10 * time.Minute},
		} {
			t.Run(c.name, func(t *testing.T) {
				cfg := &config.Config{Embed: config.EmbedConfig{MaxTimeout: c.value}}
				if got := embedMaxTimeout(cfg); got != c.want {
					t.Errorf("embedMaxTimeout(%q) = %v, want %v", c.value, got, c.want)
				}
			})
		}
	})

	t.Run("summaryMaxTimeout", func(t *testing.T) {
		for _, c := range []struct {
			name  string
			value string
			want  time.Duration
		}{
			{"empty uses default", "", 10 * time.Minute},
			{"well-formed value returned verbatim", "15m", 15 * time.Minute},
			{"malformed value falls back to default", "not-a-duration", 10 * time.Minute},
			{"negative value falls back to default", "-1m", 10 * time.Minute},
			{"zero falls back to default (ceiling never honors zero)", "0s", 10 * time.Minute},
		} {
			t.Run(c.name, func(t *testing.T) {
				cfg := &config.Config{Summarize: config.SummarizeConfig{MaxTimeout: c.value}}
				if got := summaryMaxTimeout(cfg); got != c.want {
					t.Errorf("summaryMaxTimeout(%q) = %v, want %v", c.value, got, c.want)
				}
			})
		}
	})
}

// wiredOption is one embed.With*/summarize.With* call expression found
// inside a constructor function, along with the name of the function called
// to produce its single argument (empty if the argument is not a simple
// call to a named function — e.g. a literal).
type wiredOption struct {
	optionName string
	argHelper  string
}

// collectWiredOptions parses tools.go's declarations for the function named
// funcName and walks its ENTIRE body (ast.Inspect), collecting every call
// expression of the shape pkgName.With*(...) — regardless of whether it
// appears inside a composite literal (opts := []embed.Option{...}), a plain
// call argument list (summarize.New(..., opt1, opt2)), or an
// opts = append(opts, ...) call. Walking the whole function body rather than
// only its top-level statements is what makes this a gate on the real call
// expressions the function executes, not a shape-specific string search.
func collectWiredOptions(t *testing.T, af *ast.File, funcName, pkgName string) []wiredOption {
	t.Helper()
	var fn *ast.FuncDecl
	for _, decl := range af.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == funcName {
			fn = fd
			break
		}
	}
	if fn == nil {
		t.Fatalf("function %s not found in tools.go — gate cannot exercise anything", funcName)
	}

	var opts []wiredOption
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkgIdent, ok := sel.X.(*ast.Ident)
		if !ok || pkgIdent.Name != pkgName {
			return true
		}
		if !strings.HasPrefix(sel.Sel.Name, "With") {
			return true
		}
		argHelper := ""
		if len(call.Args) == 1 {
			if inner, ok := call.Args[0].(*ast.CallExpr); ok {
				if ident, ok := inner.Fun.(*ast.Ident); ok {
					argHelper = ident.Name
				}
			}
		}
		opts = append(opts, wiredOption{optionName: sel.Sel.Name, argHelper: argHelper})
		return true
	})
	if len(opts) == 0 {
		t.Fatalf("%s: zero %s.With* option calls found — gate cannot exercise anything", funcName, pkgName)
	}
	return opts
}

// assertWired fails for any (option name -> helper name) pair in want that
// is either absent from got entirely (the option is never passed to the
// client) or present with a DIFFERENT argument helper than the one named
// (a crossed pairing — e.g. the summarize ceiling handed the embed helper).
// Checking the argument, not only the option name, is what a plain text
// search over the file cannot do.
func assertWired(t *testing.T, fnName string, got []wiredOption, want map[string]string) {
	t.Helper()
	found := make(map[string]string, len(got))
	for _, o := range got {
		found[o.optionName] = o.argHelper
	}
	for optName, wantHelper := range want {
		gotHelper, ok := found[optName]
		if !ok {
			t.Errorf("%s does not pass %s to its client constructor", fnName, optName)
			continue
		}
		if gotHelper != wantHelper {
			t.Errorf("%s's %s argument is a call to %q, want a call to %q (crossed helper pairing)",
				fnName, optName, gotHelper, wantHelper)
		}
	}
}

// TestProviderBoundOptionsWiredIntoBothClients is the T-07-04-01 gate: it
// fails if embedderFromConfig or summarizerFromConfig fails to pass any of
// its lane's three new options to its client constructor, or passes one
// with the WRONG lane's helper as its argument (a crossed pairing that a
// string search over the file's contents would not catch, since both
// helper names and both option names would still be present in the file
// somewhere).
func TestProviderBoundOptionsWiredIntoBothClients(t *testing.T) {
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, "tools.go", nil, 0)
	if err != nil {
		t.Fatalf("parser.ParseFile(tools.go): %v", err)
	}

	wantEmbed := map[string]string{
		"WithDrainBytes":   "embedDrainBytes",
		"WithDrainTimeout": "embedDrainTimeout",
		"WithMaxTimeout":   "embedMaxTimeout",
	}
	wantSummarize := map[string]string{
		"WithDrainBytes":   "summaryDrainBytes",
		"WithDrainTimeout": "summaryDrainTimeout",
		"WithMaxTimeout":   "summaryMaxTimeout",
	}

	assertWired(t, "embedderFromConfig", collectWiredOptions(t, af, "embedderFromConfig", "embed"), wantEmbed)
	assertWired(t, "summarizerFromConfig", collectWiredOptions(t, af, "summarizerFromConfig", "summarize"), wantSummarize)
}
