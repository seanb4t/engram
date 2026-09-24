// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package curationeval

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// writeLocalPairsFile writes content to a fresh temp file and returns its
// path.
func writeLocalPairsFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pairs.jsonl")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
	return path
}

func TestLoadLocalPairs(t *testing.T) {
	t.Parallel()

	content := strings.Join([]string{
		`{"record_a": "A note about the ledger service.", "record_b": "A newer note about the ledger service.", "label": "updates"}`,
		"",
		`{"record_a": "The search API returns paged results.", "record_b": "Paged results are what the search API returns.", "label": "duplicate"}`,
		`{"record_a": "The mobile app supports dark mode.", "record_b": "The billing API bills monthly.", "label": "unrelated"}`,
	}, "\n")
	path := writeLocalPairsFile(t, content)

	pairs, err := loadLocalPairs(path)
	if err != nil {
		t.Fatalf("loadLocalPairs: %v", err)
	}
	if len(pairs) != 3 {
		t.Fatalf("len(pairs) = %d, want 3", len(pairs))
	}

	wantIDs := []string{"L0001", "L0002", "L0003"}
	wantLabels := []string{"updates", "duplicate", "unrelated"}
	for i, p := range pairs {
		if p.id != wantIDs[i] {
			t.Errorf("pairs[%d].id = %q, want %q", i, p.id, wantIDs[i])
		}
		if p.label != wantLabels[i] {
			t.Errorf("pairs[%d].label = %q, want %q", i, p.label, wantLabels[i])
		}
		if p.recordA == "" || p.recordB == "" {
			t.Errorf("pairs[%d] has empty record text", i)
		}
	}
}

func TestLoadLocalPairsRejects(t *testing.T) {
	t.Parallel()

	strPtr := func(s string) *string { return &s }

	cases := []struct {
		name       string
		content    *string // nil means "file does not exist"
		wantLineNo int      // 0 means no line number is expected in the error
	}{
		{"missing_file", nil, 0},
		{"empty_file", strPtr(""), 0},
		{"malformed_json", strPtr("not json at all"), 1},
		{"unknown_field", strPtr(`{"record_a":"a","record_b":"b","label":"related","extra":"x"}`), 1},
		{"unknown_label", strPtr(`{"record_a":"a","record_b":"b","label":"friends"}`), 1},
		{"empty_record", strPtr(`{"record_a":"","record_b":"b","label":"related"}`), 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var path string
			if tc.content == nil {
				path = filepath.Join(t.TempDir(), "missing.jsonl")
			} else {
				path = writeLocalPairsFile(t, *tc.content)
			}

			_, err := loadLocalPairs(path)
			if err == nil {
				t.Fatalf("loadLocalPairs(%s) = nil error, want an error", tc.name)
			}
			if tc.wantLineNo != 0 {
				want := "line " + strconv.Itoa(tc.wantLineNo)
				if !strings.Contains(err.Error(), want) {
					t.Errorf("loadLocalPairs(%s) error = %q, want to contain %q", tc.name, err, want)
				}
			}
		})
	}
}

// TestLoadLocalPairsErrorsNeverEchoContent proves the T-03-06 mitigation: a
// load error never contains the offending line's own text, even a
// malformed one.
func TestLoadLocalPairsErrorsNeverEchoContent(t *testing.T) {
	t.Parallel()

	const marker = "zzqx-unique-marker-do-not-leak-9182"
	path := writeLocalPairsFile(t, marker+" this is not valid json")

	_, err := loadLocalPairs(path)
	if err == nil {
		t.Fatal("loadLocalPairs = nil error, want an error")
	}
	if strings.Contains(err.Error(), marker) {
		t.Errorf("loadLocalPairs error leaked the marker: %q", err)
	}
}
