// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// pruneViewFixtures returns the prune-expired fixtures the identity gate
// runs against: both the preview shape and the applied shape of
// pruneOutputDoc, each built from fixed values.
func pruneViewFixtures() map[string][]any {
	before := time.Date(2031, 6, 15, 12, 0, 0, 0, time.UTC)
	return map[string][]any{
		"prune-expired": {
			prunePreviewDoc(31, before),
			pruneReportDoc(31, before),
		},
	}
}

// jsonTopLevelKeys is an INDEPENDENTLY authored walk of doc's top-level
// JSON keys, in document order. It deliberately does NOT call viewFields —
// two separately authored walks over the same marshaled bytes is what
// makes the identity gate able to actually fail on a correspondence bug,
// rather than trivially agreeing with itself (06-CONTEXT.md D-06).
func jsonTopLevelKeys(doc any) ([]string, error) {
	b, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil, errViewNotObject
	}
	var keys []string
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("jsonTopLevelKeys: unexpected key token %v", keyTok)
		}
		keys = append(keys, key)
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil, err
		}
	}
	return keys, nil
}

// orderedKeyDiff compares want against got positionally and returns one
// error per position where they disagree, plus one error for a length
// mismatch. It is deliberately about ORDER and CORRESPONDENCE, not set
// membership: a reordered pair of otherwise-identical keys must fail this
// check (06-CONTEXT.md's "ordering" edge probe).
func orderedKeyDiff(want, got []string) []error {
	var errs []error
	if len(want) != len(got) {
		errs = append(errs, fmt.Errorf("orderedKeyDiff: length mismatch: want %d keys %v, got %d keys %v", len(want), want, len(got), got))
	}
	for i := 0; i < max(len(want), len(got)); i++ {
		var w, g string
		if i < len(want) {
			w = want[i]
		}
		if i < len(got) {
			g = got[i]
		}
		if w != g {
			errs = append(errs, fmt.Errorf("orderedKeyDiff: position %d: want %q, got %q", i, w, g))
		}
	}
	return errs
}

// countTopLevelFieldLines counts lines in a rendered operator view that
// begin with EXACTLY two leading spaces followed by a non-space rune —
// renderOperatorView's own top-level field-line shape. The headline has
// zero leading spaces and a nested row line has four, so neither is
// counted; a blank line (zero characters) is not counted either.
func countTopLevelFieldLines(out string) int {
	n := 0
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 3 {
			continue
		}
		if line[0] != ' ' || line[1] != ' ' {
			continue
		}
		if line[2] == ' ' {
			continue
		}
		n++
	}
	return n
}

// assertViewIdentity is the shared identity-gate body: it asserts (a)
// jsonTopLevelKeys and viewFields agree on key order — correspondence,
// per D-06, invariant to label text by construction — and (b) the
// rendered output carries exactly one top-level field line per json key.
// name is the fixture's commandKey, used only to annotate failures.
func assertViewIdentity(t *testing.T, name string, doc any) {
	t.Helper()

	jsonKeys, err := jsonTopLevelKeys(doc)
	if err != nil {
		t.Fatalf("%s: jsonTopLevelKeys: %v", name, err)
	}

	fields, err := viewFields(doc)
	if err != nil {
		t.Fatalf("%s: viewFields: %v", name, err)
	}
	fieldKeys := make([]string, len(fields))
	for i, f := range fields {
		fieldKeys[i] = f.Key
	}
	for _, diffErr := range orderedKeyDiff(jsonKeys, fieldKeys) {
		t.Errorf("%s: %v", name, diffErr)
	}

	var buf bytes.Buffer
	if err := renderOperatorView(&buf, "headline", doc); err != nil {
		t.Fatalf("%s: renderOperatorView: %v", name, err)
	}
	if got, want := countTopLevelFieldLines(buf.String()), len(jsonKeys); got != want {
		t.Errorf("%s: countTopLevelFieldLines(rendered) = %d, want %d (one line per json key); rendered=%q", name, got, want, buf.String())
	}
}

// TestOperatorViewIdentity runs the identity gate over every prune-expired
// fixture: the mechanism this phase proves before the other 14 operator
// reports convert to it (plan 06-01's objective).
func TestOperatorViewIdentity(t *testing.T) {
	for name, docs := range pruneViewFixtures() {
		for i, doc := range docs {
			t.Run(fmt.Sprintf("%s/%d", name, i), func(t *testing.T) {
				assertViewIdentity(t, name, doc)
			})
		}
	}
}

// TestOrderedKeyDiffDetectsDivergence is the committed proof that the
// correspondence half of the identity gate can actually fail: a dropped
// key, an extra key, a reordered pair, and a renamed key must each produce
// a non-empty diff, while two identical (or two empty) slices must not.
// The red-evidence patch harness (internal/store/redevidence_harness_test.go)
// is explicitly deferred for this phase (06-CONTEXT.md <deferred>) and is
// not used here; this table-driven negative test is the committed
// substitute proof of non-vacuity.
func TestOrderedKeyDiffDetectsDivergence(t *testing.T) {
	cases := []struct {
		name      string
		want, got []string
		wantEmpty bool
	}{
		{"identical", []string{"a", "b", "c"}, []string{"a", "b", "c"}, true},
		{"both empty", nil, nil, true},
		{"dropped key", []string{"a", "b", "c"}, []string{"a", "b"}, false},
		{"extra key", []string{"a", "b"}, []string{"a", "b", "c"}, false},
		{"reordered pair", []string{"a", "b"}, []string{"b", "a"}, false},
		{"renamed key", []string{"a", "b"}, []string{"a", "renamed"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := orderedKeyDiff(tc.want, tc.got)
			if tc.wantEmpty && len(errs) != 0 {
				t.Errorf("orderedKeyDiff(%v, %v) = %v, want empty", tc.want, tc.got, errs)
			}
			if !tc.wantEmpty && len(errs) == 0 {
				t.Errorf("orderedKeyDiff(%v, %v) = empty, want a non-empty diff", tc.want, tc.got)
			}
		})
	}
}

// TestCountTopLevelFieldLines is the committed proof that the
// rendered-output half of the identity gate can actually fail: it pins
// the exact-two-leading-spaces shape against fixture strings covering the
// cases that must and must not count.
func TestCountTopLevelFieldLines(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want int
	}{
		{"headline only", "headline\n", 0},
		{"headline plus blank plus one field", "headline\n\n  Field   value\n", 1},
		{"four-space nested rows not counted", "headline\n\n  Field\n    id=1 outcome=ok\n    id=2 outcome=ok\n", 1},
		{"blank lines not counted", "headline\n\n  A   1\n\n  B   2\n", 2},
		{"three leading spaces not counted", "headline\n\n   not a field line\n", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := countTopLevelFieldLines(tc.out); got != tc.want {
				t.Errorf("countTopLevelFieldLines(%q) = %d, want %d", tc.out, got, tc.want)
			}
		})
	}
}

// TestHumanizeKey pins humanizeKey on fixed input/output pairs, with no
// reference to any renderer output. This is the ONLY check in the phase
// that can catch a humanizer bug: per D-06 the identity gate is invariant
// to label text by design (it checks Key correspondence, never Label), so
// these two checks are deliberately decomposed and cannot move together —
// the recorded mutation probe in this plan's SUMMARY.md demonstrates that
// a humanizer bug fails THIS test while TestOperatorViewIdentity stays
// green.
func TestHumanizeKey(t *testing.T) {
	cases := []struct {
		key  string
		want string
	}{
		{"older_than", "Older than"},
		{"dry_run", "Dry run"},
		{"eligible_count", "Eligible count"},
		{"best_effort", "Best effort"},
		{"all_scopes", "All scopes"},
		{"top_k", "Top k"},
		{"dim", "Dim"},
		{"by_scope_category", "By scope category"},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			if got := humanizeKey(tc.key); got != tc.want {
				t.Errorf("humanizeKey(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

// TestOperatorViewEmptyShapes covers the "empty" edge probe: a nil slice
// field, an explicitly empty non-nil slice field, a zero-valued numeric
// field, a zero-valued bool field, an empty string field, and a field
// whose marshaled value is JSON null (a nil pointer) all produce a
// top-level field line — one per json key, matching the identity
// property even when there is nothing to show. A container key with no
// elements (empty_slice) carries zero rows beneath its label; a
// null-valued key renders an empty value, never a panic or a dropped
// line.
func TestOperatorViewEmptyShapes(t *testing.T) {
	type emptyShapesDoc struct {
		NilSlice   []string `json:"nil_slice"`
		EmptySlice []string `json:"empty_slice"`
		ZeroNum    int      `json:"zero_num"`
		ZeroBool   bool     `json:"zero_bool"`
		EmptyStr   string   `json:"empty_str"`
		NullPtr    *string  `json:"null_ptr"`
	}
	doc := emptyShapesDoc{EmptySlice: []string{}}

	jsonKeys, err := jsonTopLevelKeys(doc)
	if err != nil {
		t.Fatalf("jsonTopLevelKeys: %v", err)
	}

	var buf bytes.Buffer
	if err := renderOperatorView(&buf, "headline", doc); err != nil {
		t.Fatalf("renderOperatorView: %v", err)
	}
	if got, want := countTopLevelFieldLines(buf.String()), len(jsonKeys); got != want {
		t.Errorf("countTopLevelFieldLines(%q) = %d, want %d (one line per json key, even for empty values)", buf.String(), got, want)
	}

	fields, err := viewFields(doc)
	if err != nil {
		t.Fatalf("viewFields: %v", err)
	}
	byKey := make(map[string]viewField, len(fields))
	for _, f := range fields {
		byKey[f.Key] = f
	}

	emptySlice, ok := byKey["empty_slice"]
	if !ok {
		t.Fatal("viewFields(doc) did not return a field for empty_slice")
	}
	if len(emptySlice.Rows) != 0 {
		t.Errorf("empty_slice field.Rows = %v, want zero rows (container key with no elements)", emptySlice.Rows)
	}

	for _, key := range []string{"nil_slice", "null_ptr"} {
		f, ok := byKey[key]
		if !ok {
			t.Fatalf("viewFields(doc) did not return a field for %s", key)
		}
		if f.Value != "" {
			t.Errorf("%s field.Value = %q, want empty (a JSON null renders as an empty value)", key, f.Value)
		}
		if len(f.Rows) != 0 {
			t.Errorf("%s field.Rows = %v, want nil (a null value is a scalar, not a container)", key, f.Rows)
		}
	}
}

// TestOperatorViewOrdering covers the "ordering" edge probe: rendered
// field order matches declaration order for a plain struct; an embedded
// struct's fields render promoted at the embedding position, exactly
// where encoding/json places them; a json:"-" field appears in neither
// lane; and an omitempty field left at its zero value is dropped from
// both lanes together.
func TestOperatorViewOrdering(t *testing.T) {
	t.Run("plain struct matches declaration order", func(t *testing.T) {
		type orderedDoc struct {
			First  string `json:"first"`
			Second string `json:"second"`
			Third  string `json:"third"`
		}
		doc := orderedDoc{First: "a", Second: "b", Third: "c"}
		jsonKeys, err := jsonTopLevelKeys(doc)
		if err != nil {
			t.Fatalf("jsonTopLevelKeys: %v", err)
		}
		fields, err := viewFields(doc)
		if err != nil {
			t.Fatalf("viewFields: %v", err)
		}
		fieldKeys := make([]string, len(fields))
		for i, f := range fields {
			fieldKeys[i] = f.Key
		}
		for _, e := range orderedKeyDiff([]string{"first", "second", "third"}, fieldKeys) {
			t.Error(e)
		}
		for _, e := range orderedKeyDiff(jsonKeys, fieldKeys) {
			t.Error(e)
		}
	})

	t.Run("embedded struct promotes fields at embedding position", func(t *testing.T) {
		type inner struct {
			Middle string `json:"middle"`
		}
		type outer struct {
			First string `json:"first"`
			inner
			Last string `json:"last"`
		}
		doc := outer{First: "a", inner: inner{Middle: "b"}, Last: "c"}
		jsonKeys, err := jsonTopLevelKeys(doc)
		if err != nil {
			t.Fatalf("jsonTopLevelKeys: %v", err)
		}
		fields, err := viewFields(doc)
		if err != nil {
			t.Fatalf("viewFields: %v", err)
		}
		fieldKeys := make([]string, len(fields))
		for i, f := range fields {
			fieldKeys[i] = f.Key
		}
		for _, e := range orderedKeyDiff(jsonKeys, fieldKeys) {
			t.Error(e)
		}
		for _, e := range orderedKeyDiff([]string{"first", "middle", "last"}, fieldKeys) {
			t.Error(e)
		}
	})

	t.Run("json dash field excluded from both lanes", func(t *testing.T) {
		type secretDoc struct {
			Public string `json:"public"`
			Secret string `json:"-"`
		}
		doc := secretDoc{Public: "a", Secret: "b"}
		jsonKeys, err := jsonTopLevelKeys(doc)
		if err != nil {
			t.Fatalf("jsonTopLevelKeys: %v", err)
		}
		fields, err := viewFields(doc)
		if err != nil {
			t.Fatalf("viewFields: %v", err)
		}
		for _, k := range jsonKeys {
			if k == "Secret" || k == "-" {
				t.Errorf("jsonTopLevelKeys(doc) = %v, unexpectedly carries the json:\"-\" field", jsonKeys)
			}
		}
		for _, f := range fields {
			if f.Key == "Secret" || f.Key == "-" {
				t.Errorf("viewFields(doc) = %+v, unexpectedly carries the json:\"-\" field", fields)
			}
		}
	})

	t.Run("omitempty zero value dropped from both lanes together", func(t *testing.T) {
		type sparseDoc struct {
			Present string `json:"present"`
			Absent  string `json:"absent,omitempty"`
		}
		doc := sparseDoc{Present: "a"}
		jsonKeys, err := jsonTopLevelKeys(doc)
		if err != nil {
			t.Fatalf("jsonTopLevelKeys: %v", err)
		}
		fields, err := viewFields(doc)
		if err != nil {
			t.Fatalf("viewFields: %v", err)
		}
		if len(jsonKeys) != 1 || jsonKeys[0] != "present" {
			t.Errorf(`jsonTopLevelKeys(doc) = %v, want ["present"] (omitempty zero value dropped)`, jsonKeys)
		}
		if len(fields) != 1 || fields[0].Key != "present" {
			t.Errorf(`viewFields(doc) = %+v, want one field keyed "present"`, fields)
		}
	})
}

// TestOperatorViewDuplicateKeyAdjacency covers the "adjacency" edge probe:
// a throwaway struct declares two fields carrying the same json tag at
// the same depth. encoding/json's own field-conflict rule decides what
// gets emitted (both same-depth, same-name fields are excluded together);
// the view inherits that decision rather than second-guessing it, so
// len(viewFields(doc)) must equal len(jsonTopLevelKeys(doc)) — one line
// per key encoding/json actually emitted, never one per declared Go
// field.
func TestOperatorViewDuplicateKeyAdjacency(t *testing.T) {
	type dupKeyDoc struct {
		A string `json:"dup"`
		B string `json:"dup"` //nolint:govet // deliberate duplicate tag: exercises encoding/json's own same-depth field-conflict rule (the adjacency edge probe)
		C string `json:"unique"`
	}
	doc := dupKeyDoc{A: "x", B: "y", C: "z"}

	jsonKeys, err := jsonTopLevelKeys(doc)
	if err != nil {
		t.Fatalf("jsonTopLevelKeys: %v", err)
	}
	fields, err := viewFields(doc)
	if err != nil {
		t.Fatalf("viewFields: %v", err)
	}
	if got, want := len(fields), len(jsonKeys); got != want {
		t.Errorf("len(viewFields(doc)) = %d, len(jsonTopLevelKeys(doc)) = %d, want equal — one line per key encoding/json emitted, never one per declared Go field", got, want)
	}
	// dupKeyDoc declares 3 Go fields but encoding/json's own conflict rule
	// excludes BOTH same-depth, same-tag fields together, so only "unique"
	// survives to the marshaled document.
	if got, want := jsonKeys, []string{"unique"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf(`jsonTopLevelKeys(doc) = %v, want %v (encoding/json's own same-depth conflict rule)`, got, want)
	}
}

// TestOperatorViewEncoding covers the "encoding" edge probe: two fields
// whose LABELS have equal rune counts ("Name", "Code") must have their
// value columns start at the same rune offset regardless of what the
// values themselves contain — column width is computed from the label,
// never the value, so a non-ASCII value cannot throw off alignment for
// its own row.
func TestOperatorViewEncoding(t *testing.T) {
	type encodingDoc struct {
		Name string `json:"name"`
		Code string `json:"code"`
	}
	nameVal := "ascii"
	codeVal := "légïnûs"
	doc := encodingDoc{Name: nameVal, Code: codeVal}

	var buf bytes.Buffer
	if err := renderOperatorView(&buf, "headline", doc); err != nil {
		t.Fatalf("renderOperatorView: %v", err)
	}

	var nameLine, codeLine string
	for _, line := range strings.Split(buf.String(), "\n") {
		switch {
		case strings.HasPrefix(line, "  Name"):
			nameLine = line
		case strings.HasPrefix(line, "  Code"):
			codeLine = line
		}
	}
	if nameLine == "" || codeLine == "" {
		t.Fatalf("did not find both field lines in rendered output: %q", buf.String())
	}

	nameOffset := utf8.RuneCountInString(nameLine) - utf8.RuneCountInString(nameVal)
	codeOffset := utf8.RuneCountInString(codeLine) - utf8.RuneCountInString(codeVal)
	if nameOffset != codeOffset {
		t.Errorf("value columns start at rune offsets %d (name) and %d (code), want equal — equal-rune-count labels must align regardless of the value's own byte length", nameOffset, codeOffset)
	}

	// Accepted, documented limitation (D-03; see renderOperatorView's doc
	// comment): padding counts runes via utf8.RuneCountInString, which
	// counts an East-Asian-wide or emoji rune as ONE unit even though a
	// terminal renders it across TWO display cells. Such a value would
	// visually misalign despite passing the rune-offset check above; this
	// is accepted, not fixed, because the text lane is explicitly not a
	// stable interface.
}

// TestOperatorViewSanitizesControlCharacters covers threat T-06-03: a
// value containing \n, \t, \r, and ESC (\x1b) must render on exactly one
// line, must not change the field-line count, and the rendered bytes must
// contain no rune below 0x20 (other than the line's own terminating \n)
// and no 0x7f — a stored value cannot forge an extra report line or emit
// a terminal escape sequence.
func TestOperatorViewSanitizesControlCharacters(t *testing.T) {
	type controlDoc struct {
		Reason string `json:"reason"`
	}
	doc := controlDoc{Reason: "line1\nline2\tindented\rcr\x1bESC"}

	var buf bytes.Buffer
	if err := renderOperatorView(&buf, "headline", doc); err != nil {
		t.Fatalf("renderOperatorView: %v", err)
	}
	out := buf.String()

	jsonKeys, err := jsonTopLevelKeys(doc)
	if err != nil {
		t.Fatalf("jsonTopLevelKeys: %v", err)
	}
	if got, want := countTopLevelFieldLines(out), len(jsonKeys); got != want {
		t.Errorf("countTopLevelFieldLines(%q) = %d, want %d — an embedded newline must not add a report line", out, got, want)
	}

	found := false
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "line1") {
			continue
		}
		found = true
		for _, want := range []string{"line2", "indented", "cr", "ESC"} {
			if !strings.Contains(line, want) {
				t.Errorf("rendered field line %q does not carry sanitized fragment %q — the whole value must occupy one line", line, want)
			}
		}
	}
	if !found {
		t.Fatalf("rendered output %q does not contain the field's sanitized value", out)
	}

	for i, r := range out {
		if r == '\n' {
			continue
		}
		if r < 0x20 || r == 0x7f {
			t.Errorf("rendered output contains control rune %U at byte index %d: %q", r, i, out)
		}
	}
}

// TestRenderOperatorViewSanitizesHeadline covers issue #505 (D-11, threat
// T-07-01): renderOperatorView's headline write is routed through
// sanitizeViewValue, the SAME sanitizer the field path already uses, so the
// guarantee holds by construction rather than by every headline producer
// happening to interpolate flag values and counts. Control runes are built
// from their rune values (not raw literal bytes) so this test file itself
// carries no raw control bytes.
func TestRenderOperatorViewSanitizesHeadline(t *testing.T) {
	t.Run("C0 control rune (ESC, the escape-sequence introducer) replaced with a single space", func(t *testing.T) {
		esc := string(rune(0x1b))
		headline := "record" + esc + "ABCDE12345"
		var buf bytes.Buffer
		if err := renderOperatorView(&buf, headline, 42); err != nil {
			t.Fatalf("renderOperatorView: %v", err)
		}
		if got, want := buf.String(), "record ABCDE12345\n"; got != want {
			t.Errorf("renderOperatorView(headline) = %q, want %q", got, want)
		}
	})

	t.Run("DEL rune replaced with a single space", func(t *testing.T) {
		del := string(rune(0x7f))
		headline := "record" + del + "ABCDE12345"
		var buf bytes.Buffer
		if err := renderOperatorView(&buf, headline, 42); err != nil {
			t.Fatalf("renderOperatorView: %v", err)
		}
		if got, want := buf.String(), "record ABCDE12345\n"; got != want {
			t.Errorf("renderOperatorView(headline) = %q, want %q", got, want)
		}
	})

	t.Run("ordinary printable headline renders byte-identical", func(t *testing.T) {
		headline := "record ABCDE12345 (archived, superseded)"
		var buf bytes.Buffer
		if err := renderOperatorView(&buf, headline, 42); err != nil {
			t.Fatalf("renderOperatorView: %v", err)
		}
		if got, want := buf.String(), headline+"\n"; got != want {
			t.Errorf("renderOperatorView(headline) = %q, want %q (idempotent on ordinary text)", got, want)
		}
	})
}

// TestOperatorViewNonObjectDocument proves a marshaled-array or
// marshaled-scalar document is handled gracefully rather than panicking:
// viewFields returns errViewNotObject, and renderOperatorView still
// writes the headline plus exactly one trailing newline.
func TestOperatorViewNonObjectDocument(t *testing.T) {
	cases := []struct {
		name string
		doc  any
	}{
		{"array", []string{"a", "b"}},
		{"scalar", 42},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := viewFields(tc.doc); !errors.Is(err, errViewNotObject) {
				t.Errorf("viewFields(%v) error = %v, want errViewNotObject", tc.doc, err)
			}
			var buf bytes.Buffer
			if err := renderOperatorView(&buf, "headline", tc.doc); err != nil {
				t.Fatalf("renderOperatorView: %v", err)
			}
			if got, want := buf.String(), "headline\n"; got != want {
				t.Errorf("renderOperatorView(non-object doc) = %q, want %q", got, want)
			}
		})
	}
}
