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

// anchorSpan records where one anchor (start or end) was found.
type anchorSpan struct {
	lineIdx    int
	spanStart  int
	spanEnd    int
	foundOnAny bool
}

// findAnchors scans path once, locating the first line carrying rule
// ruleID's start anchor and the first line carrying its end anchor. It
// returns the raw lines (for reconstruction) alongside both spans.
func findAnchors(path, ruleID string) (lines []string, start, end anchorSpan, err error) {
	f, openErr := os.Open(path)
	if openErr != nil {
		return nil, start, end, fmt.Errorf("surfaces: open %s: %w", path, openErr)
	}
	defer func() { _ = f.Close() }()

	htmlStart, htmlEnd, protoStart, protoEnd := anchorLiterals(ruleID)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !start.foundOnAny {
			if s, e, ok := anchorPos(line, htmlStart, protoStart); ok {
				start = anchorSpan{lineIdx: len(lines), spanStart: s, spanEnd: e, foundOnAny: true}
			}
		}
		if !end.foundOnAny {
			if s, e, ok := anchorPos(line, htmlEnd, protoEnd); ok {
				end = anchorSpan{lineIdx: len(lines), spanStart: s, spanEnd: e, foundOnAny: true}
			}
		}
		lines = append(lines, line)
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return nil, start, end, fmt.Errorf("surfaces: scan %s: %w", path, scanErr)
	}
	return lines, start, end, nil
}

// ReadRegion returns the body currently written between rule ruleID's
// anchor pair in path, and whether that anchor pair was found. It returns a
// non-nil error only for an I/O failure or a structurally invalid anchor
// pair (end anchor strictly before start anchor's line) — a simply-absent
// anchor pair returns ("", false, nil).
func ReadRegion(path, ruleID string) (string, bool, error) {
	lines, start, end, err := findAnchors(path, ruleID)
	if err != nil {
		return "", false, err
	}
	if !start.foundOnAny || !end.foundOnAny {
		return "", false, nil
	}
	if end.lineIdx < start.lineIdx {
		return "", false, fmt.Errorf("surfaces: %s: end anchor for rule %q precedes start anchor", path, ruleID)
	}
	if start.lineIdx == end.lineIdx {
		line := lines[start.lineIdx]
		return line[start.spanEnd:end.spanStart], true, nil
	}
	return strings.Join(lines[start.lineIdx+1:end.lineIdx], "\n"), true, nil
}

// WriteRegion rewrites the body between rule ruleID's anchor pair in path
// to body, writing atomically — building the new content in memory,
// os.CreateTemp in the same directory, then os.Rename over the target — so
// an interrupted run leaves either the previous or the new content, never a
// truncated file. WriteRegion returns an error, never a silent no-op, when
// the start anchor is absent, the end anchor is absent, or the end anchor
// precedes the start anchor.
func WriteRegion(path, ruleID, body string) error {
	lines, start, end, err := findAnchors(path, ruleID)
	if err != nil {
		return err
	}
	if !start.foundOnAny {
		return fmt.Errorf("surfaces: WriteRegion: %s: start anchor for rule %q not found", path, ruleID)
	}
	if !end.foundOnAny {
		return fmt.Errorf("surfaces: WriteRegion: %s: end anchor for rule %q not found", path, ruleID)
	}
	if end.lineIdx < start.lineIdx {
		return fmt.Errorf("surfaces: WriteRegion: %s: end anchor for rule %q precedes start anchor", path, ruleID)
	}

	var newLines []string
	if start.lineIdx == end.lineIdx {
		line := lines[start.lineIdx]
		newLine := line[:start.spanEnd] + body + line[end.spanStart:]
		newLines = append(newLines, lines[:start.lineIdx]...)
		newLines = append(newLines, newLine)
		newLines = append(newLines, lines[start.lineIdx+1:]...)
	} else {
		newLines = append(newLines, lines[:start.lineIdx+1]...)
		if body != "" {
			newLines = append(newLines, strings.Split(body, "\n")...)
		}
		newLines = append(newLines, lines[end.lineIdx:]...)
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
