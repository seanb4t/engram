// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
)

// BlockStartMarker and BlockEndMarker delimit the single skills index
// block engram splices into an AGENTS.md-shaped file (D-13/D-15/D-16),
// following this repo's existing HTML-comment anchor convention
// (internal/surfaces' `engram:rule:start <id>` / `engram:rule:end <id>`)
// with the domain "skills". There is exactly one skills block per file,
// so — unlike the per-rule anchors — no identifier parameter is needed.
//
// These two literals are a PUBLISHED ON-DISK SURFACE: once written, they
// persist verbatim inside a file engram does not own (a user's own
// AGENTS.md, or a dotfiles-managed symlink target), on every machine
// that has ever run `engram setup --apply`. Changing either literal
// later orphans every block already written under the old spelling —
// scanBlock would report those old blocks as absent, not malformed, and
// a second block would be appended alongside the orphan.
const (
	BlockStartMarker = "<!-- engram:skills:start -->"
	BlockEndMarker   = "<!-- engram:skills:end -->"
)

// skillsManagedComment is the one explanatory line RenderBlock emits
// just inside BlockStartMarker, telling a human reader (or another tool)
// that hand edits inside the block do not survive a re-run.
const skillsManagedComment = "<!-- This block is managed by `engram setup` — edits made inside it are overwritten on the next run. -->"

// blockState classifies a scanBlock result. It is a three-value enum,
// declared as explicit named values rather than modeled on an absence or
// the zero value (internal/setup's Outcome doc-comment discipline,
// plan.go) — a blockState variable that is never assigned is a
// programming error, not a silent "absent".
type blockState int

const (
	// blockAbsent means neither BlockStartMarker nor BlockEndMarker
	// appears anywhere in the scanned content.
	blockAbsent blockState = iota + 1
	// blockWellFormed means exactly one start marker is followed, later
	// in the content, by exactly one end marker, with no other marker
	// occurrence anywhere.
	blockWellFormed
	// blockMalformed means the content carries markers in any other
	// shape: two or more well-paired blocks, a start with no matching
	// end, an end with no preceding start, or a second start before the
	// first's end. D-15: this is the ONLY other writable... rather,
	// UNWRITABLE state — Splice refuses to produce content for it.
	blockMalformed
)

// lineSpan is one line of scanned content: text is the line's own bytes
// (never including its trailing "\n"), and offset is the byte position
// of text's first byte within the original content.
type lineSpan struct {
	text   []byte
	offset int
}

// splitLinesWithOffsets splits content on "\n", recording each line's own
// starting byte offset within content. The final line (whether or not
// content ends with "\n") is included, so a caller can distinguish "ends
// with a trailing newline" from "does not" by comparing offsets against
// len(content) — Splice's absent-state append logic depends on exactly
// that distinction.
func splitLinesWithOffsets(content []byte) []lineSpan {
	var out []lineSpan
	offset := 0
	for {
		idx := bytes.IndexByte(content[offset:], '\n')
		if idx == -1 {
			out = append(out, lineSpan{text: content[offset:], offset: offset})
			return out
		}
		out = append(out, lineSpan{text: content[offset : offset+idx], offset: offset})
		offset += idx + 1
	}
}

// scanBlock scans content once, byte-offset by byte-offset (D-15 requires
// the offending marker OFFSETS in a malformed report's text, which line
// indices alone cannot express), classifying the skills block's state.
//
// This imitates internal/surfaces.scanAnchors' pending-pair state
// machine — tracking whether a start has been seen with no matching end
// yet, so "neither seen" is distinguishable from "seen but ill-paired" —
// but is a fresh, local implementation rather than a shared call: that
// scanner's own caller, WriteRegion, rewrites the body inside EVERY
// matched pair and stages-and-swaps its write, both the OPPOSITE of what
// D-15 (hard-fail on ambiguity) and D-16 (in-place, symlink-preserving
// write) require here. A future contributor must not "unify" these two
// implementations and silently inherit the wrong invariant.
//
// A marker is recognized when a line, after trimming leading and
// trailing whitespace, equals the marker literal exactly.
//
// Returns the state; for blockWellFormed, the byte offset and length of
// the start marker line and of the end marker line (lengths exclude the
// line's own trailing "\n"); and, for every state, the byte offsets of
// every marker occurrence found in file order (empty for blockAbsent).
func scanBlock(content []byte) (state blockState, startOffset, startLen, endOffset, endLen int, offsets []int) {
	const (
		pendingNone = -1
	)
	pendingStart := pendingNone
	pendingStartLen := 0
	malformed := false
	pairCount := 0

	for _, line := range splitLinesWithOffsets(content) {
		trimmed := string(bytes.TrimSpace(line.text))
		isStart := trimmed == BlockStartMarker
		isEnd := trimmed == BlockEndMarker

		if isStart {
			offsets = append(offsets, line.offset)
			if pendingStart != pendingNone {
				// A second start before the first's matching end.
				malformed = true
			}
			pendingStart = line.offset
			pendingStartLen = len(line.text)
		}
		if isEnd {
			offsets = append(offsets, line.offset)
			if pendingStart == pendingNone {
				// An end with no preceding, still-open start.
				malformed = true
			} else {
				pairCount++
				startOffset, startLen = pendingStart, pendingStartLen
				endOffset, endLen = line.offset, len(line.text)
				pendingStart = pendingNone
			}
		}
	}
	if pendingStart != pendingNone {
		// A start never closed before the end of the content.
		malformed = true
	}

	if len(offsets) == 0 {
		return blockAbsent, 0, 0, 0, 0, nil
	}
	if malformed || pairCount != 1 {
		return blockMalformed, 0, 0, 0, 0, offsets
	}
	return blockWellFormed, startOffset, startLen, endOffset, endLen, offsets
}

// RenderBlock produces the deterministic, newline-terminated skills index
// block for skills: the start marker line, one line stating the block is
// engram-managed, one line per skill (in the order skills is given —
// callers pass Inventory()'s own deterministic order) naming that
// skill's Name, its authored Summary index entry, and the absolute path
// of its SKILL.md (skillsDir joined with the skill's name and
// "SKILL.md"), then the end marker line.
//
// The full text of a skill is NEVER inlined here — D-13's entire point is
// that an agent fetches a skill's procedure on demand from the path this
// block names, rather than paying for all of it in every session on a
// runtime with no native skill format.
func RenderBlock(skills []Skill, skillsDir string) string {
	var b strings.Builder
	b.WriteString(BlockStartMarker)
	b.WriteByte('\n')
	b.WriteString(skillsManagedComment)
	b.WriteByte('\n')
	for _, s := range skills {
		path := filepath.Join(skillsDir, s.Name, "SKILL.md")
		fmt.Fprintf(&b, "- **%s**: %s (%s)\n", s.Name, s.Summary, path)
	}
	b.WriteString(BlockEndMarker)
	b.WriteByte('\n')
	return b.String()
}

// Splice returns content with block inserted or substituted, according to
// scanBlock's classification of content:
//
//   - blockAbsent: block is appended. When content is empty, the result
//     is exactly block. Otherwise the result is content, then a newline
//     if content does not already end with one, then a blank line if
//     content does not already end with one, then block — so the new
//     block always sits after exactly one blank line, and every
//     preceding byte is unchanged.
//   - blockWellFormed: only the bytes strictly between the end of the
//     start marker line and the beginning of the end marker line are
//     replaced, with block's own interior lines. Both marker lines, and
//     every byte before the start marker or after the end marker, are
//     copied verbatim from content — never re-rendered — so a
//     pre-existing block's exact marker-line formatting survives.
//   - blockMalformed: Splice returns a nil slice and an error naming the
//     malformed state and every offending byte offset. A caller is
//     structurally unable to obtain content to write from a malformed
//     input — this is D-15's hard-fail, and the reason Install must
//     never derive writable bytes from this branch.
func Splice(content []byte, block string) ([]byte, error) {
	state, startOffset, startLen, endOffset, _, offsets := scanBlock(content)

	switch state {
	case blockAbsent:
		return spliceAppend(content, block), nil
	case blockWellFormed:
		return spliceReplace(content, startOffset, startLen, endOffset, block)
	case blockMalformed:
		return nil, fmt.Errorf(
			"skills: AGENTS.md skills block is malformed — engram will not repair or guess at a region it did not author (D-15); offending marker byte offsets: %v", offsets)
	default:
		return nil, fmt.Errorf("skills: Splice: unrecognized block state %d", state)
	}
}

// spliceAppend implements Splice's blockAbsent case.
func spliceAppend(content []byte, block string) []byte {
	if len(content) == 0 {
		return []byte(block)
	}

	endsWithNewline := bytes.HasSuffix(content, []byte("\n"))
	endsWithBlankLine := bytes.HasSuffix(content, []byte("\n\n"))

	var buf bytes.Buffer
	buf.Write(content)
	if !endsWithNewline {
		buf.WriteByte('\n')
	}
	if !endsWithBlankLine {
		buf.WriteByte('\n')
	}
	buf.WriteString(block)
	return buf.Bytes()
}

// spliceReplace implements Splice's blockWellFormed case. startOffset and
// startLen locate the ORIGINAL content's start marker line (preserved
// verbatim); endOffset locates the ORIGINAL content's end marker line
// (also preserved verbatim, along with everything after it). block's own
// interior — the bytes strictly between block's own start and end marker
// lines — is extracted by scanning block the same way, so the interior
// substituted in is exactly what RenderBlock rendered, never re-derived.
func spliceReplace(content []byte, startOffset, startLen, endOffset int, block string) ([]byte, error) {
	blockBytes := []byte(block)
	bState, bStart, bStartLen, bEnd, _, _ := scanBlock(blockBytes)
	if bState != blockWellFormed {
		return nil, fmt.Errorf("skills: Splice: rendered block is not well-formed (internal invariant violated)")
	}
	interior := blockBytes[bStart+bStartLen+1 : bEnd]

	var buf bytes.Buffer
	buf.Write(content[:startOffset+startLen+1])
	buf.Write(interior)
	buf.Write(content[endOffset:])
	return buf.Bytes(), nil
}
