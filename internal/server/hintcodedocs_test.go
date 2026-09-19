// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// errors.md's own prose claims its hint-code table is "checked off one by
// one" against argerror.go's HintCode constants (RESEARCH.md Pitfall 5) —
// but nothing enforced that claim until this file. internal/surfaces
// declares conditional-rule SENTENCES only (ConditionalRule.Hint is a plain
// string referencing the vocabulary); the hint-code VOCABULARY's single
// declaration is argerror.go's const block, and surfaces' single-declaration
// convention does not reach it (D-05 verification, 02-CONTEXT.md). This gate
// closes that gap: it derives the vocabulary from source (go/parser over
// argerror.go, never a second hand-typed list) and compares it, in both
// directions, against errors.md's published table — plus the heading's count
// word, every other count-word phrase on the page, the rendered
// response-too-large envelope example, and every cross-page anchor link to
// this heading.

package server

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// numberWords maps an integer count to its English word. Capped at 20 -- a
// generous margin over any hint-code count this project is realistically
// expected to reach before this map needs extending; a count outside this
// range fails TestErrorsDocHintCodesMatchArgErrorConstants with a clear
// message naming the gap, rather than silently mis-comparing.
var numberWords = map[int]string{
	1: "one", 2: "two", 3: "three", 4: "four", 5: "five",
	6: "six", 7: "seven", 8: "eight", 9: "nine", 10: "ten",
	11: "eleven", 12: "twelve", 13: "thirteen", 14: "fourteen", 15: "fifteen",
	16: "sixteen", 17: "seventeen", 18: "eighteen", 19: "nineteen", 20: "twenty",
}

// hintCodeConstants parses argerror.go with go/parser (mirroring
// conditionalsweep_test.go's house style of source-scanning this package)
// and returns every string value of a HintCode-typed const declaration, in
// source order. This is the vocabulary's single point of truth -- never a
// second hand-typed list. Fails the test outright on zero constants parsed
// (the zero-applicability guard: a gate that can never see any input proves
// nothing).
func hintCodeConstants(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, "argerror.go", nil, 0)
	if err != nil {
		t.Fatalf("parser.ParseFile(argerror.go): %v", err)
	}

	var codes []string
	for _, decl := range af.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			ident, ok := vs.Type.(*ast.Ident)
			if !ok || ident.Name != "HintCode" {
				continue
			}
			for _, v := range vs.Values {
				lit, ok := v.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				unquoted, uerr := strconv.Unquote(lit.Value)
				if uerr != nil {
					t.Fatalf("unquote HintCode literal %s: %v", lit.Value, uerr)
				}
				codes = append(codes, unquoted)
			}
		}
	}
	if len(codes) == 0 {
		t.Fatal("hintCodeConstants: parsed zero HintCode constants from argerror.go -- zero-applicability guard tripped")
	}
	return codes
}

// hintHeadingPattern matches errors.md's "## The <word> hint codes" heading,
// capturing the count word.
var hintHeadingPattern = regexp.MustCompile(`(?m)^## The (\S+) hint codes$`)

// hintTableRowPattern matches a markdown table row whose first cell is
// backticked, capturing that cell's content.
var hintTableRowPattern = regexp.MustCompile("(?m)^\\|\\s*`([^`]+)`")

// parseHintCodeTable finds the "## The <word> hint codes" heading in doc,
// cuts the section at the next line starting "## " (so a table under a
// LATER heading is never counted), and collects the first backticked cell
// of every table row in that section, in document order. It errors if the
// heading is absent, or if the heading's own section carries zero
// backticked table rows.
func parseHintCodeTable(doc string) (countWord string, codes []string, err error) {
	loc := hintHeadingPattern.FindStringSubmatchIndex(doc)
	if loc == nil {
		return "", nil, errParseHintCodeTableNoHeading
	}
	countWord = doc[loc[2]:loc[3]]

	section := doc[loc[1]:]
	if idx := strings.Index(section, "\n## "); idx >= 0 {
		section = section[:idx]
	}

	for _, m := range hintTableRowPattern.FindAllStringSubmatch(section, -1) {
		codes = append(codes, m[1])
	}
	if len(codes) == 0 {
		return "", nil, errParseHintCodeTableNoRows
	}
	return countWord, codes, nil
}

var (
	errParseHintCodeTableNoHeading = parseHintCodeTableError("no heading matching \"## The <word> hint codes\" found")
	errParseHintCodeTableNoRows    = parseHintCodeTableError("heading found but its section carries zero backticked table rows")
)

// parseHintCodeTableError is a plain string-backed error type, matching this
// package's existing sentinel idiom (errors.New-shaped) rather than adding a
// second error-construction style for a two-value function with no wrapped
// cause.
type parseHintCodeTableError string

func (e parseHintCodeTableError) Error() string { return string(e) }

// TestParseHintCodeTable is the pure, inline-fixture self-test of
// parseHintCodeTable's own parsing logic, independent of the real
// errors.md -- so a change to the live doc can never mask a bug in the
// parser itself, and vice versa.
func TestParseHintCodeTable(t *testing.T) {
	t.Run("heading with two rows", func(t *testing.T) {
		doc := "# doc\n\n## The two hint codes\n\n| Hint code | Meaning |\n|---|---|\n| `alpha` | first |\n| `beta` | second |\n"
		word, codes, err := parseHintCodeTable(doc)
		if err != nil {
			t.Fatalf("parseHintCodeTable: %v", err)
		}
		if word != "two" {
			t.Errorf("countWord = %q, want %q", word, "two")
		}
		if want := []string{"alpha", "beta"}; !reflect.DeepEqual(codes, want) {
			t.Errorf("codes = %v, want %v", codes, want)
		}
	})

	t.Run("no matching heading", func(t *testing.T) {
		doc := "# doc\n\nno heading of that shape here.\n"
		if _, _, err := parseHintCodeTable(doc); err == nil {
			t.Error("parseHintCodeTable: expected an error for a doc with no matching heading, got nil")
		}
	})

	t.Run("heading present, zero rows", func(t *testing.T) {
		doc := "## The two hint codes\n\nno table follows this heading at all.\n"
		if _, _, err := parseHintCodeTable(doc); err == nil {
			t.Error("parseHintCodeTable: expected an error for a heading section with zero rows, got nil")
		}
	})

	t.Run("row after the next heading is not counted", func(t *testing.T) {
		doc := "## The two hint codes\n\n| `alpha` | first |\n\n## Something else entirely\n\n| `beta` | second |\n"
		_, codes, err := parseHintCodeTable(doc)
		if err != nil {
			t.Fatalf("parseHintCodeTable: %v", err)
		}
		if want := []string{"alpha"}; !reflect.DeepEqual(codes, want) {
			t.Errorf("codes = %v, want %v (a row placed after the NEXT heading must not be counted)", codes, want)
		}
	})
}

// hintAnchorPattern matches a link to the hint-code heading's anchor
// anywhere in docs-site, capturing the count word the anchor slug carries.
var hintAnchorPattern = regexp.MustCompile(`/reference/errors/#the-([a-z]+)-hint-codes`)

// hintCountWordPhrasePatterns are the two phrase shapes a count word can
// appear in on errors.md: "<word>-code" (e.g. "ten-code table") and
// "<word> hint codes" (e.g. "the ten hint codes" -- also matches the
// heading itself). Both are case-insensitive since the heading capitalizes
// "The" but body prose does not.
var hintCountWordPhrasePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b([a-z]+)-code\b`),
	regexp.MustCompile(`(?i)\b([a-z]+) hint codes\b`),
}

// TestErrorsDocHintCodesMatchArgErrorConstants is the D-05 gate: it fails
// when errors.md's published hint-code table and argerror.go's HintCode
// constants differ in either direction, when the heading's count word is
// wrong, when any stale count-word phrase remains anywhere on the page
// (frontmatter included), when the rendered response-too-large envelope
// example is missing, or when any docs-site link to this heading's anchor
// targets a stale slug.
func TestErrorsDocHintCodesMatchArgErrorConstants(t *testing.T) {
	constants := hintCodeConstants(t)

	wantWord, ok := numberWords[len(constants)]
	if !ok {
		t.Fatalf("hintCodeConstants returned %d entries -- numberWords has no entry for that count; extend the map", len(constants))
	}

	doc := mustReadDoc(t, docsErrorsPath)
	gotWord, tableCodes, err := parseHintCodeTable(doc)
	if err != nil {
		t.Fatalf("parseHintCodeTable(%s): %v", docsErrorsPath, err)
	}
	if len(tableCodes) == 0 {
		t.Fatal("parseHintCodeTable returned zero rows -- zero-applicability guard tripped")
	}

	constSet := make(map[string]bool, len(constants))
	for _, c := range constants {
		constSet[c] = true
	}
	tableSet := make(map[string]bool, len(tableCodes))
	for _, c := range tableCodes {
		tableSet[c] = true
	}

	var missingFromDoc, inventedByDoc []string
	for c := range constSet {
		if !tableSet[c] {
			missingFromDoc = append(missingFromDoc, c)
		}
	}
	for c := range tableSet {
		if !constSet[c] {
			inventedByDoc = append(inventedByDoc, c)
		}
	}
	sort.Strings(missingFromDoc)
	sort.Strings(inventedByDoc)
	if len(missingFromDoc) > 0 {
		t.Errorf("%s is missing hint code(s) argerror.go declares: %v", docsErrorsPath, missingFromDoc)
	}
	if len(inventedByDoc) > 0 {
		t.Errorf("%s lists hint code(s) argerror.go does not declare: %v", docsErrorsPath, inventedByDoc)
	}

	if gotWord != wantWord {
		t.Errorf("%s heading count word = %q, want %q (argerror.go declares %d HintCode constants)",
			docsErrorsPath, gotWord, wantWord, len(constants))
	}

	// Every count-word phrase anywhere on the page (frontmatter included)
	// must read the CURRENT word -- catches a stale "ten-code"/"ten hint
	// codes" phrase left behind elsewhere on the page after the constant
	// count changed, not just at the heading itself.
	numberWordSet := make(map[string]bool, len(numberWords))
	for _, w := range numberWords {
		numberWordSet[w] = true
	}
	for _, p := range hintCountWordPhrasePatterns {
		for _, m := range p.FindAllStringSubmatch(doc, -1) {
			word := strings.ToLower(m[1])
			if numberWordSet[word] && word != wantWord {
				t.Errorf("%s: stale count word %q found via pattern %q -- every count word on the page must read %q",
					docsErrorsPath, m[1], p.String(), wantWord)
			}
		}
	}

	if want := normalizeWS(responseTooLargeEnvelope()); !strings.Contains(normalizeWS(doc), want) {
		t.Errorf("%s does not contain the rendered response-too-large envelope verbatim: %q", docsErrorsPath, want)
	}

	// Every cross-page anchor link to this heading, anywhere under
	// docs-site, must use the CURRENT word -- e.g. guides/upgrade.md's
	// link following the heading rename.
	docsRoot := "../../docs-site/src/content/docs"
	if _, statErr := os.Stat(docsRoot); statErr != nil {
		t.Fatalf("docs root %s: %v", docsRoot, statErr)
	}
	walkErr := filepath.WalkDir(docsRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") && !strings.HasSuffix(path, ".mdx") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, m := range hintAnchorPattern.FindAllStringSubmatch(string(data), -1) {
			if m[1] != wantWord {
				t.Errorf("%s: stale hint-code anchor %q, want word %q", path, m[0], wantWord)
			}
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk %s: %v", docsRoot, walkErr)
	}
}
