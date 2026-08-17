// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
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
