// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// errViewNotObject is returned by viewFields when the marshaled document's
// top-level JSON value is not an object (an array or a bare scalar) —
// renderOperatorView treats that as "no field table", not an error.
var errViewNotObject = errors.New("operator view: marshaled document is not a JSON object")

// viewField is one top-level rendered field of an operator view. Key is the
// raw JSON key, verbatim, exactly as the json marshaler emitted it — never
// derived from a struct field name, so it stays correct across omitempty,
// json:"-", and embedded-struct promotion. Label is humanizeKey(Key), used
// only for the top-level rendering. Value is the rendered scalar (empty for
// a container-valued key). Rows is one rendered line per array element, or
// a single rendered line for an object-valued key; nil for a scalar key.
type viewField struct {
	Key   string
	Label string
	Value string
	Rows  []string
}

// viewFields walks the bytes doc marshals to — not the Go struct — so
// that whatever the json lane actually emits is exactly what the text
// lane renders: omitempty drops, json:"-" exclusions, embedded struct
// promotion, and any custom MarshalJSON (e.g. time.Time) are all already
// resolved by the time this function sees the bytes. It marshals doc
// exactly once (06-CONTEXT.md D-02's identity-by-construction property
// depends on there being no second path from doc to rendered text), then
// mixes (*json.Decoder).Token and Decode — the documented streaming idiom
// for walking a JSON object while preserving its own key order, which
// encoding/json does not otherwise expose.
func viewFields(doc any) ([]viewField, error) {
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

	var fields []viewField
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("operator view: unexpected key token %v", keyTok)
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}

		field := viewField{Key: key, Label: humanizeKey(key)}
		switch valueKind(raw) {
		case '[':
			var elems []json.RawMessage
			if err := json.Unmarshal(raw, &elems); err != nil {
				return nil, err
			}
			rows := make([]string, 0, len(elems))
			for _, elem := range elems {
				if valueKind(elem) == '{' {
					row, err := viewRow(elem)
					if err != nil {
						return nil, err
					}
					rows = append(rows, row)
				} else {
					rows = append(rows, viewScalar(elem))
				}
			}
			field.Rows = rows
		case '{':
			row, err := viewRow(raw)
			if err != nil {
				return nil, err
			}
			field.Rows = []string{row}
		default:
			field.Value = viewScalar(raw)
		}
		fields = append(fields, field)
	}
	return fields, nil
}

// valueKind classifies a marshaled JSON value by its first non-whitespace
// byte: '[' for an array, '{' for an object, and 0 for any scalar (string,
// number, bool, or null). json.Marshal's own output never carries leading
// whitespace inside a value, but json.RawMessage values decoded through
// (*json.Decoder).Decode are trimmed to the value's own bytes regardless,
// so this is a plain first-byte switch rather than a whitespace scan.
func valueKind(raw json.RawMessage) byte {
	if len(raw) == 0 {
		return 0
	}
	switch raw[0] {
	case '[', '{':
		return raw[0]
	default:
		return 0
	}
}

// viewScalar renders a single JSON scalar value for display. A JSON string
// is unmarshaled into a Go string and passed through sanitizeViewValue so
// no stored value can forge report structure. A JSON null renders as the
// empty string. Every other scalar (number, bool) renders as its verbatim
// raw text — deliberately never round-tripped through float64, so a large
// uint64 counter keeps the exact digits encoding/json produced.
func viewScalar(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			// json.Marshal produced this value; a decode failure here would
			// mean the bytes were tampered with between the two calls, which
			// cannot happen for a value we just marshaled ourselves. Render
			// the raw bytes sanitized rather than panicking.
			return sanitizeViewValue(string(raw))
		}
		return sanitizeViewValue(s)
	}
	if string(raw) == "null" {
		return ""
	}
	return string(raw)
}

// viewRow renders one JSON object as a single "key=value ..." line, walking
// its keys in document order with the same Token-plus-Decode idiom
// viewFields uses. It deliberately uses the RAW key, never humanizeKey —
// D-05's asymmetry: top-level fields are read and get humanized labels,
// nested rows are scanned and keep raw keys so they stay dense and
// grep-friendly. This is not an inconsistency to "fix"; it is the
// documented shape (06-CONTEXT.md D-05, D-07).
func viewRow(raw json.RawMessage) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return "", err
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return "", fmt.Errorf("operator view: row value %s is not a JSON object", raw)
	}
	var parts []string
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return "", err
		}
		key, ok := keyTok.(string)
		if !ok {
			return "", fmt.Errorf("operator view: unexpected row key token %v", keyTok)
		}
		var val json.RawMessage
		if err := dec.Decode(&val); err != nil {
			return "", err
		}
		parts = append(parts, key+"="+viewScalar(val))
	}
	return strings.Join(parts, " "), nil
}

// humanizeKey turns a JSON key into a top-level display label: underscores
// become spaces, and the first rune is upper-cased. Nothing else —
// deliberately no initialism/acronym table. No top-level operator report
// key is an initialism today, and nested row keys render raw (D-05), so a
// table here would be untested dead code guarding against a case that does
// not occur. "top_k" therefore renders as "Top k"; that is accepted on an
// explicitly unstable lane (D-03).
func humanizeKey(key string) string {
	s := strings.ReplaceAll(key, "_", " ")
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return strings.ToUpper(string(r)) + s[size:]
}

// sanitizeViewValue replaces every rune strictly below 0x20 and the DEL
// rune 0x7f with a single space, leaving all other runes untouched. This
// covers \n, \r, \t, and ESC (0x1b): a stored scope, category, tag, id, or
// free-form reason value can never forge an extra report line or emit a
// terminal escape sequence into an operator's console (threat T-06-03).
// The json lane needs no equivalent sanitization — encoding/json already
// escapes control characters in its own string encoding.
//
// The guarantee is narrower than a blanket statement over "every value"
// would imply: it only ever runs on a JSON string value viewScalar
// recognizes by its own kind check (raw[0] == '"'). A value that is itself
// a JSON array or object two levels deep from the doc root (a nested array
// element, or a row-level object/array field inside viewRow) bypasses
// viewScalar's sanitizing branch entirely and renders verbatim (WR-02,
// 06-REVIEW.md). No operator report struct produces such a shape today —
// see TestOperatorViewFixturesHaveNoUnsanitizedNesting (operator_output_test.go),
// which fails loudly the day one does.
func sanitizeViewValue(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			b.WriteRune(' ')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// renderOperatorView writes headline to w, followed by a rendered view of
// doc: one aligned "Label   value" line per top-level JSON key doc marshals
// to, with any array/object-valued key's rows indented beneath it. It is
// the ONLY function that turns a document value into text in the operator
// tier (renderOperator, operator_output.go, delegates here); there is no
// second serialization, so text and json cannot diverge by construction
// (D-01, D-02).
//
// BOTH the headline and every field value pass through sanitizeViewValue
// before being written (issue #505, D-11, threat T-07-01): the guarantee is
// therefore independent of what any individual headline producer happens to
// interpolate. Before this phase the headline was written raw, which was
// safe only because every existing headline producer interpolated CLI flag
// values and counts, never stored record content — an invariant that was
// never enforced and, for fields like scope/tags (user-supplied) and owner
// (from the IdP), was narrower than it looked even before `engram get`
// (plan 07-05) made it exploitable: the first headline producer built from
// a live record's state.
//
// Column padding is computed once from the widest label and counts runes
// via utf8.RuneCountInString — the same measure text/tabwriter itself
// uses. This is an accepted, documented limitation, not a bug: an
// East-Asian-wide or emoji value occupies more display cells than runes
// and will misalign under this scheme, and the text lane is explicitly not
// a stable interface (D-03), so that cost is accepted rather than solved.
//
// If doc's top-level JSON value is not an object (errViewNotObject),
// rendering stops after the headline — the headline alone is the whole
// output. Otherwise the output ends with exactly one trailing newline and
// never a trailing blank line.
func renderOperatorView(w io.Writer, headline string, doc any) error {
	if _, err := fmt.Fprintln(w, sanitizeViewValue(headline)); err != nil {
		return err
	}
	fields, err := viewFields(doc)
	if err != nil {
		if errors.Is(err, errViewNotObject) {
			return nil
		}
		return err
	}
	if len(fields) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	width := 0
	for _, f := range fields {
		if n := utf8.RuneCountInString(f.Label); n > width {
			width = n
		}
	}

	for _, f := range fields {
		pad := width - utf8.RuneCountInString(f.Label) + 3
		line := "  " + f.Label + strings.Repeat(" ", pad)
		if f.Value == "" {
			line = strings.TrimRight(line, " ")
		} else {
			line += f.Value
		}
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
		for _, row := range f.Rows {
			if _, err := fmt.Fprintln(w, "    "+row); err != nil {
				return err
			}
		}
	}
	return nil
}
