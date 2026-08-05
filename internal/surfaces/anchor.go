// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package surfaces

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// anchorLiterals returns the two comment flavors this package recognizes
// for rule ruleID's start/end anchors: an HTML comment (markdown/prose
// surfaces) and a `//` line comment (.proto surfaces). ReadRegion and
// WriteRegion try both per line, so one scanner works across every prose
// file type with no per-file-type dispatch — the caller picks which flavor
// to WRITE (via the body it passes in for a fresh anchor pair authored by a
// human), but detection is flavor-agnostic.
func anchorLiterals(ruleID string) (htmlStart, htmlEnd, protoStart, protoEnd string) {
	htmlStart = "<!-- engram:rule:start " + ruleID + " -->"
	htmlEnd = "<!-- engram:rule:end " + ruleID + " -->"
	protoStart = "// engram:rule:start " + ruleID
	protoEnd = "// engram:rule:end " + ruleID
	return
}

// anchorPos locates one anchor literal within line, returning the [start,
// end) byte span the anchor comment itself occupies. The proto flavor is a
// `//` line comment with nothing legal after it on the same line, so its
// span always extends to the end of the line; the HTML flavor's span is
// exactly the comment text, so content may sit inline before and after it
// on the same line (the markdown-table-cell case).
func anchorPos(line, htmlLiteral, protoLiteral string) (start, end int, ok bool) {
	if idx := strings.Index(line, htmlLiteral); idx != -1 {
		return idx, idx + len(htmlLiteral), true
	}
	if idx := strings.Index(line, protoLiteral); idx != -1 {
		return idx, len(line), true
	}
	return 0, 0, false
}

// anchorPair records one matched start/end anchor pair for a rule within a
// file — start/end line indices plus the byte offsets needed to slice an
// inline (same-line) pair. A single file may carry more than one anchor
// pair for the SAME rule ID (e.g. a proto field comment restated on two
// separate messages), so scanAnchors returns every pair it finds, in file
// order.
type anchorPair struct {
	startLineIdx int
	startSpanEnd int
	endLineIdx   int
	endSpanStart int
}

// scanAnchors scans path once, locating every well-formed start/end anchor
// pair for rule ruleID, in file order, alongside whether a start literal
// and an end literal were seen anywhere at all (sawStart/sawEnd — needed to
// distinguish "absent" from "malformed" once pairing is applied). Anchors
// must appear in matching start,end,start,end,... order — a second start
// before the first's matching end, or an end with no open start, is a
// malformed-pairing error.
func scanAnchors(path, ruleID string) (lines []string, pairs []anchorPair, sawStart, sawEnd bool, err error) {
	f, openErr := os.Open(path)
	if openErr != nil {
		return nil, nil, false, false, fmt.Errorf("surfaces: open %s: %w", path, openErr)
	}
	defer func() { _ = f.Close() }()

	htmlStart, htmlEnd, protoStart, protoEnd := anchorLiterals(ruleID)

	var pending *anchorPair
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if _, e, ok := anchorPos(line, htmlStart, protoStart); ok {
			sawStart = true
			if pending != nil {
				return nil, nil, false, false, fmt.Errorf(
					"surfaces: %s: a second start anchor for rule %q appears before the first's matching end anchor", path, ruleID)
			}
			pending = &anchorPair{startLineIdx: len(lines), startSpanEnd: e}
		}
		if s, _, ok := anchorPos(line, htmlEnd, protoEnd); ok {
			sawEnd = true
			if pending == nil {
				return nil, nil, false, false, fmt.Errorf(
					"surfaces: %s: end anchor for rule %q precedes its start anchor", path, ruleID)
			}
			pending.endLineIdx = len(lines)
			pending.endSpanStart = s
			pairs = append(pairs, *pending)
			pending = nil
		}
		lines = append(lines, line)
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return nil, nil, false, false, fmt.Errorf("surfaces: scan %s: %w", path, scanErr)
	}
	if pending != nil {
		return nil, nil, false, false, fmt.Errorf(
			"surfaces: %s: start anchor for rule %q has no matching end anchor", path, ruleID)
	}
	return lines, pairs, sawStart, sawEnd, nil
}

// leadingWhitespace returns s's leading run of spaces/tabs.
func leadingWhitespace(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[:i]
}

// regionBody extracts the text currently written inside anchor pair p.
func regionBody(lines []string, p anchorPair) string {
	if p.startLineIdx == p.endLineIdx {
		line := lines[p.startLineIdx]
		return line[p.startSpanEnd:p.endSpanStart]
	}
	return strings.Join(lines[p.startLineIdx+1:p.endLineIdx], "\n")
}

// ReadRegion returns the body currently written inside rule ruleID's FIRST
// anchor pair in path, and whether an anchor pair was found. It returns a
// non-nil error only for an I/O failure or a structurally invalid anchor
// arrangement (an end anchor with no matching start, a second start before
// the first's end, or an unterminated start) — a simply-absent anchor pair
// returns ("", false, nil).
func ReadRegion(path, ruleID string) (string, bool, error) {
	lines, pairs, sawStart, sawEnd, err := scanAnchors(path, ruleID)
	if err != nil {
		return "", false, err
	}
	if !sawStart && !sawEnd {
		return "", false, nil
	}
	if len(pairs) == 0 {
		return "", false, fmt.Errorf("surfaces: %s: rule %q anchor markers present but not well-paired", path, ruleID)
	}
	return regionBody(lines, pairs[0]), true, nil
}

// WriteRegion rewrites the body inside EVERY one of rule ruleID's anchor
// pairs in path to body (a rule restated at more than one point in the same
// file — e.g. two proto messages both documenting the same cross_spine
// field — gets the same canonical text at each), writing atomically:
// building the new content in memory, os.CreateTemp in the same directory,
// then os.Rename over the target, so an interrupted run leaves either the
// previous or the new content, never a truncated file. WriteRegion returns
// an error, never a silent no-op, when the start anchor is absent, the end
// anchor is absent, or the pair is malformed (end anchor with no preceding
// start, or vice versa).
func WriteRegion(path, ruleID, body string) error {
	lines, pairs, sawStart, sawEnd, err := scanAnchors(path, ruleID)
	if err != nil {
		return err
	}
	if !sawStart {
		return fmt.Errorf("surfaces: WriteRegion: %s: start anchor for rule %q not found", path, ruleID)
	}
	if !sawEnd {
		return fmt.Errorf("surfaces: WriteRegion: %s: end anchor for rule %q not found", path, ruleID)
	}
	if len(pairs) == 0 {
		return fmt.Errorf("surfaces: WriteRegion: %s: rule %q anchor markers present but not well-paired", path, ruleID)
	}

	newLines := append([]string(nil), lines...)
	// Rewrite from the LAST pair backward: a multi-line body replacement can
	// change the line count, which would otherwise invalidate the line
	// indices recorded for every pair still to come.
	for i := len(pairs) - 1; i >= 0; i-- {
		p := pairs[i]
		if p.startLineIdx == p.endLineIdx {
			line := newLines[p.startLineIdx]
			newLines[p.startLineIdx] = line[:p.startSpanEnd] + body + line[p.endSpanStart:]
			continue
		}
		var replacement []string
		if body != "" {
			// Indent every replacement line to match the start anchor
			// line's own leading whitespace, so a multi-line region (e.g.
			// a proto `//` comment block) stays visually consistent with
			// its surrounding source rather than starting at column 0.
			indent := leadingWhitespace(newLines[p.startLineIdx])
			for _, l := range strings.Split(body, "\n") {
				replacement = append(replacement, indent+l)
			}
		}
		merged := make([]string, 0, len(newLines))
		merged = append(merged, newLines[:p.startLineIdx+1]...)
		merged = append(merged, replacement...)
		merged = append(merged, newLines[p.endLineIdx:]...)
		newLines = merged
	}

	content := strings.Join(newLines, "\n")
	if len(newLines) > 0 {
		content += "\n"
	}

	return writeFileAtomic(path, content)
}

// writeFileAtomic writes content to path via a same-directory temp file and
// os.Rename, preserving path's existing file mode where possible.
func writeFileAtomic(path, content string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".surfaces-*.tmp")
	if err != nil {
		return fmt.Errorf("surfaces: WriteRegion: create temp in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	cleanupOnErr := true
	defer func() {
		if cleanupOnErr {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("surfaces: WriteRegion: write temp for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("surfaces: WriteRegion: close temp for %s: %w", path, err)
	}
	if info, statErr := os.Stat(path); statErr == nil {
		_ = os.Chmod(tmpName, info.Mode())
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("surfaces: WriteRegion: rename temp over %s: %w", path, err)
	}
	cleanupOnErr = false
	return nil
}
