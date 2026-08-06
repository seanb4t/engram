// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/seanb4t/engram/internal/store"
)

// Citation verification tiers (D-05/D-06/D-08): the four buckets
// `spine-review verify` classifies every stored citation into. valid means
// the excerpt is exactly where the citation's Locator says it is; moved
// means the excerpt still exists in the SAME file, at a different byte
// offset -- ordinary drift from an edit above the cited range (GitHub issue
// #355's shape); broken means the file is gone or the excerpt is gone from
// it entirely; unverifiable means the classifier did not actually check the
// citation at all (a non-file kind, an empty cached excerpt, a different
// repo, or a ref this command refuses to read) -- every such citation
// carries the reason it was not checked, so a clean report can never be
// read as coverage the run did not have (REQ-citation-drift-verify's
// transparency requirement).
const (
	tierValid        = "valid"
	tierMoved        = "moved"
	tierBroken       = "broken"
	tierUnverifiable = "unverifiable"
)

// The two reasons a file citation classifies broken (D-08): the file itself
// is missing, or the file exists but the cached excerpt is nowhere in it.
// Kept as distinct constants so a caller can distinguish the two causes
// without string-matching a formatted sentence, even though both share the
// one "broken" tier name.
const (
	reasonFileMissing = "file missing"
	reasonExcerptGone = "excerpt gone"
	reasonNoExcerpt   = "no cached excerpt"
)

// citationVerdict is one citation's classification result: the tier and
// reason verifyFileCitation computes, plus enough identity (RecordID,
// ShortID, Index, Ref) for a report row. verifyFileCitation itself only
// ever sets Tier, Reason, and Ref -- the caller (this file's RunE, plan
// 03-04 Task 2) fills RecordID/ShortID/Index once it knows which owning
// record and citation index produced this verdict.
type citationVerdict struct {
	Tier     string
	Reason   string
	RecordID string
	ShortID  string
	Index    int
	Ref      string
}

// verifyFileCitation classifies a single citation against already-read file
// content -- a pure function with zero filesystem access, so the tier logic
// is unit-testable without a live filesystem or store. The caller reads
// each unique Ref once and passes the bytes in.
//
// Tiers are evaluated in EXACTLY this order -- file-missing, then
// no-excerpt, then at-locator, then same-file search, then excerpt-gone --
// which is what keeps a missing file from ever reporting "excerpt gone"
// (D-08's ordering requirement).
//
// The at-locator check is START-ANCHORED: an excerpt is "at locator" when
// fileContent has the excerpt as a prefix beginning at the locator's
// resolved offset -- the excerpt is NOT required to terminate before the
// locator's end line. A citation whose cached excerpt is longer than the
// line range it advertises is common and is not drift; requiring complete
// containment within the locator's range would misclassify that ordinary
// case as moved.
//
// The moved tier is found via a single in-file search for the excerpt,
// reporting the byte offset found -- never a whole-tree search and never a
// fuzzy match. A short excerpt risking a confident wrong match outside its
// own file is the tradeoff this narrow search deliberately accepts, per
// this phase's CONTEXT.md.
//
// A Locator that does not parse, or that resolves past the end of the
// file, falls through to the same-file search rather than erroring -- a
// stale line range is drift, not a malformed citation.
func verifyFileCitation(c store.Citation, fileContent string, fileExists bool) citationVerdict {
	v := citationVerdict{Ref: c.Ref}
	if c.Kind != "file" {
		v.Tier = tierUnverifiable
		v.Reason = fmt.Sprintf("citation kind %q is not independently verifiable", c.Kind)
		return v
	}
	if !fileExists {
		v.Tier = tierBroken
		v.Reason = reasonFileMissing
		return v
	}
	if c.Excerpt == "" {
		v.Tier = tierUnverifiable
		v.Reason = reasonNoExcerpt
		return v
	}
	if offset := excerptOffsetAt(fileContent, c.Locator); offset >= 0 && strings.HasPrefix(fileContent[offset:], c.Excerpt) {
		v.Tier = tierValid
		return v
	}
	// The locator match wins even when the same excerpt also appears
	// elsewhere -- the check above already returned before reaching this
	// search when the locator matched, so an excerpt present at BOTH the
	// locator and elsewhere still classifies valid.
	if idx := strings.Index(fileContent, c.Excerpt); idx >= 0 {
		v.Tier = tierMoved
		v.Reason = fmt.Sprintf("excerpt found at byte offset %d, not at the cited locator", idx)
		return v
	}
	v.Tier = tierBroken
	v.Reason = reasonExcerptGone
	return v
}

// excerptOffsetAt returns the byte offset content's locator-named line
// begins at, or -1 when locator is empty, unparseable, names a line before
// 1, or names a line past the end of content. Supports the two Locator
// forms the repo already writes: a bare line number ("200") and a
// "start-end" line range ("200-240") -- only the start line matters for the
// start-anchored at-locator check, so a range's end value is ignored. Do
// not invent additional forms.
func excerptOffsetAt(content, locator string) int {
	if locator == "" {
		return -1
	}
	start, ok := parseLocatorStart(locator)
	if !ok || start < 1 {
		return -1
	}
	lines := strings.Split(content, "\n")
	if start > len(lines) {
		return -1
	}
	offset := 0
	for i := 0; i < start-1; i++ {
		offset += len(lines[i]) + 1
	}
	return offset
}

// parseLocatorStart parses the leading line number out of a bare-line-number
// or "start-end" range Locator, ignoring everything from the first "-"
// onward (the range's end value is never needed by the start-anchored
// at-locator check).
func parseLocatorStart(locator string) (int, bool) {
	first := locator
	if i := strings.IndexByte(locator, '-'); i >= 0 {
		first = locator[:i]
	}
	n, err := strconv.Atoi(first)
	if err != nil {
		return 0, false
	}
	return n, true
}
