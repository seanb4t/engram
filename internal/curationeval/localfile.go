// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package curationeval

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/seanb4t/engram/internal/verdict"
)

// localPairLine is the strict JSON Lines record shape loadLocalPairs reads:
// exactly record_a, record_b and label — no other field is accepted.
type localPairLine struct {
	RecordA string `json:"record_a"`
	RecordB string `json:"record_b"`
	Label   string `json:"label"`
}

// loadLocalPairs reads a private, gitignored JSON Lines file of labeled
// pairs for D-01's real-spine local corpus mode (ENGRAM_CURATION_EVAL_PAIRS):
// one JSON object per line (record_a, record_b, label), blank lines
// skipped, strict decoding (an unrecognized field is rejected, not
// ignored). record_b must be the later note — the same ordering
// consolidate's verdict pass uses. IDs are assigned L0001 onward in file
// order. Every error names the file's line number where one applies, but
// NEVER a line's own content: this file holds private real-spine memory
// text and must never be echoed into a test failure, a log line, or any
// committed artifact. The recommended location,
// internal/curationeval/testdata/local/, is excluded by .gitignore — this
// loader itself accepts any path.
func loadLocalPairs(path string) ([]labeledPair, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("curationeval: open local pairs file: %w", err)
	}
	defer func() { _ = f.Close() }()

	var pairs []labeledPair
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var rec localPairLine
		dec := json.NewDecoder(strings.NewReader(line))
		dec.DisallowUnknownFields()
		if derr := dec.Decode(&rec); derr != nil {
			reason := "malformed JSON"
			if strings.Contains(derr.Error(), "unknown field") {
				reason = "unknown field"
			}
			return nil, fmt.Errorf("curationeval: local pairs file line %d: %s", lineNo, reason)
		}
		if rec.RecordA == "" || rec.RecordB == "" {
			return nil, fmt.Errorf("curationeval: local pairs file line %d: record_a and record_b must both be non-empty", lineNo)
		}
		if !slices.Contains(verdict.Relations(), rec.Label) {
			return nil, fmt.Errorf("curationeval: local pairs file line %d: unknown label", lineNo)
		}

		pairs = append(pairs, labeledPair{
			id:      fmt.Sprintf("L%04d", len(pairs)+1),
			label:   rec.Label,
			recordA: rec.RecordA,
			recordB: rec.RecordB,
		})
	}
	if serr := scanner.Err(); serr != nil {
		return nil, fmt.Errorf("curationeval: read local pairs file: %w", serr)
	}
	if len(pairs) == 0 {
		return nil, errors.New("curationeval: local pairs file contains no pairs")
	}
	return pairs, nil
}
