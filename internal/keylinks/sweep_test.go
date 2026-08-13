// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package keylinks

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// v013Phase12MilestoneRoot is the milestone directory containing the two
// phases REQ-keylink-past-gates-reassessed names: the plans that passed
// verification while #479's escaping defect made their key-link gates
// silent no-ops.
const v013Phase12MilestoneRoot = ".planning/milestones/v0.13.x-phases"

// v013Phase12Dirs are the two phase directories under
// v013Phase12MilestoneRoot this one-time sweep re-resolves against HEAD.
var v013Phase12Dirs = []string{
	"01-interface-enforceability",
	"02-interface-discoverability",
}

// reassessVerdict is one of the five values in the closed set every
// v0.13.x Phase 1-2 key-link resolves to. No sixth value exists, and
// TestReassessmentTableIsComplete rejects anything outside this set.
type reassessVerdict string

const (
	// verdictPinned means the corrected pattern matched its own from
	// file's content at HEAD — D-11's plain case.
	verdictPinned reassessVerdict = "pinned"
	// verdictPinnedViaTarget means the pattern did not match its from
	// file but did match its to file, through the same fallback the
	// consuming tool applies. Still pinned by D-11's definition; recorded
	// separately so how many links are pinned only by the fallback leg
	// stays visible.
	verdictPinnedViaTarget reassessVerdict = "pinned-via-target"
	// verdictUnpinned means the pattern compiled and was escape-free, but
	// matched neither file's content, with a reason recorded by
	// inspection.
	verdictUnpinned reassessVerdict = "unpinned"
	// verdictUnreadable means neither the from file nor the to file
	// exists at HEAD — "we could not look", which this sweep records as
	// its own class rather than folding into pinned (a link nobody could
	// resolve is not evidence of anything) or unpinned (which claims a
	// resolution attempt actually ran against real file content).
	verdictUnreadable reassessVerdict = "unreadable"
	// verdictInvalid means ValidatePattern still rejects the pattern.
	// After plan 01-02's rewrite this bucket is expected to be empty; a
	// non-empty bucket here is a defect in that rewrite, not an accepted
	// state.
	verdictInvalid reassessVerdict = "invalid"
)

// reassessVerdictSet is the closed set TestReassessmentTableIsComplete
// checks every emitted verdict against.
var reassessVerdictSet = map[reassessVerdict]bool{
	verdictPinned:          true,
	verdictPinnedViaTarget: true,
	verdictUnpinned:        true,
	verdictUnreadable:      true,
	verdictInvalid:         true,
}

// reassessRow is one key-link's full verdict record: the link itself, the
// verdict, and the reason (mandatory for verdictUnpinned, informative for
// every other verdict).
type reassessRow struct {
	Link    KeyLink
	Verdict reassessVerdict
	Reason  string
}

// v013UnpinnedReasons hand-records, by inspection, the reason for every
// link this sweep finds unpinned (D-11 requires a reason, not merely a
// verdict — "symbol renamed", "routed through a wrapper", "file moved",
// or "symbol never present"). Keyed by "<repo-relative plan file>:<line>"
// so a reason cannot silently drift onto the wrong link if the input set
// is ever reordered. A verdictUnpinned row with no entry here gets a
// loud placeholder reason instead of an empty one, so a newly-discovered
// unpinned link cannot slip through unexamined.
var v013UnpinnedReasons = map[string]string{
	".planning/milestones/v0.13.x-phases/02-interface-discoverability/02-04-PLAN.md:48": "routed through a wrapper — internal/server/tools.go's mcp.AddTool calls annotationsFor(name) (internal/server/toolannotations.go:24), and annotationsFor is what calls surfaces.ClassForTool, not the from file directly. tools.go itself has never contained the literal call site the pattern names (confirmed: `git log --all -S\"ClassForTool\" -- internal/server/tools.go` returns no commits). The linked behavior is real and correctly wired; only the pattern's from-file assumption is stale.",
}

// collectV013Phase12Links walks every -PLAN.md under v013Phase12Dirs and
// returns every key-link found, sorted by file then line so a re-run
// diffs cleanly (mirrors ScanPlans's own determinism guarantee, D-07).
func collectV013Phase12Links(t *testing.T, repoRoot string) []KeyLink {
	t.Helper()

	var links []KeyLink
	for _, dir := range v013Phase12Dirs {
		root := filepath.Join(repoRoot, v013Phase12MilestoneRoot, dir)
		walkErr := filepath.Walk(root, func(p string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(info.Name(), "-PLAN.md") {
				return nil
			}
			found, perr := ParsePlanKeyLinks(p)
			if perr != nil {
				return perr
			}
			links = append(links, found...)
			return nil
		})
		if walkErr != nil {
			t.Fatalf("collectV013Phase12Links: walk %s: %v", root, walkErr)
		}
	}

	sort.Slice(links, func(i, j int) bool {
		if links[i].File != links[j].File {
			return links[i].File < links[j].File
		}
		return links[i].Line < links[j].Line
	})
	return links
}

// relKey renders link's file (an absolute path — ParsePlanKeyLinks
// records whatever path it was called with) as a repo-relative
// "file:line" string, matching the keys in v013UnpinnedReasons and the
// file column in every emitted line.
func relKey(repoRoot string, link KeyLink) string {
	rel, err := filepath.Rel(repoRoot, link.File)
	if err != nil {
		rel = link.File
	}
	return fmt.Sprintf("%s:%d", filepath.ToSlash(rel), link.Line)
}

// classifyV013Link resolves link's corrected pattern against HEAD and
// returns its verdict. It reuses CheckSatisfiable — the guard's own
// matcher — rather than a second implementation of pattern resolution
// (D-12): CheckSatisfiable alone decides whether the pattern is
// satisfied, and this function only reads off, via the same compiled
// *regexp.Regexp CheckSatisfiable already validated, which of the two
// legs (from, then to) actually matched, so pinned and pinned-via-target
// can be told apart.
func classifyV013Link(repoRoot string, link KeyLink) reassessRow {
	re, off := ValidatePattern(link.Pattern)
	if off != nil {
		return reassessRow{
			Link:    link,
			Verdict: verdictInvalid,
			Reason:  fmt.Sprintf("%s: %s", off.Shape, off.Fix),
		}
	}

	fromPath := filepath.Join(repoRoot, link.From)
	toPath := filepath.Join(repoRoot, link.To)
	fromContent, fromErr := os.ReadFile(fromPath)
	toContent, toErr := os.ReadFile(toPath)
	fromExists := fromErr == nil
	toExists := toErr == nil

	if !fromExists && !toExists {
		return reassessRow{
			Link:    link,
			Verdict: verdictUnreadable,
			Reason:  fmt.Sprintf("neither the from file (%s) nor the to file (%s) exists at HEAD", link.From, link.To),
		}
	}

	// CheckSatisfiable is the guard's single source of truth for
	// "satisfied or not" (D-11, D-12). Its own from-missing shortcut
	// (silent nil, meant for a plan that has not executed yet, D-04)
	// cannot fire here undetected: every link in this sweep's input set
	// belongs to an already-shipped v0.13.x plan, and the fromExists /
	// toExists check above has already classified a from-and-to-both-
	// missing link as unreadable before CheckSatisfiable is even called.
	_ = CheckSatisfiable(link, re, repoRoot)

	switch {
	case fromExists && re.Match(fromContent):
		return reassessRow{
			Link:    link,
			Verdict: verdictPinned,
			Reason:  fmt.Sprintf("matched its from file, %s", link.From),
		}
	case toExists && re.Match(toContent):
		return reassessRow{
			Link:    link,
			Verdict: verdictPinnedViaTarget,
			Reason:  fmt.Sprintf("matched its to file, %s, by the from-file fallback; the from file did not match", link.To),
		}
	}

	reason, known := v013UnpinnedReasons[relKey(repoRoot, link)]
	if !known {
		reason = "UNRECORDED — this link was found unpinned but has no hand-inspected reason in v013UnpinnedReasons; investigate before trusting this verdict"
	}
	return reassessRow{Link: link, Verdict: verdictUnpinned, Reason: reason}
}

// reassessV013Phase12 collects the input set and classifies every link,
// one row per link with no filtering — the count equality
// TestReassessmentTableIsComplete asserts holds by construction unless
// this loop, or collectV013Phase12Links above it, is deliberately made to
// skip an entry.
func reassessV013Phase12(t *testing.T, repoRoot string) (links []KeyLink, rows []reassessRow) {
	t.Helper()
	links = collectV013Phase12Links(t, repoRoot)
	rows = make([]reassessRow, 0, len(links))
	for _, link := range links {
		rows = append(rows, classifyV013Link(repoRoot, link))
	}
	return links, rows
}

// reassessLine renders one row as a single deterministic line carrying
// the plan file and line, from, to, pattern, verdict, and reason —
// modeled on OffenderLine's shape (keylinks.go), extended with the
// verdict column this reporting test needs.
func reassessLine(repoRoot string, row reassessRow) string {
	link := row.Link
	rel, err := filepath.Rel(repoRoot, link.File)
	if err != nil {
		rel = link.File
	}
	return fmt.Sprintf(
		"file=%s:%d from=%s to=%s pattern=%q verdict=%s reason=%q",
		filepath.ToSlash(rel), link.Line, link.From, link.To, link.Pattern, row.Verdict, row.Reason,
	)
}

// TestReassessV013Phase12 is the one-time v0.13.x Phase 1-2 reassessment
// (REQ-keylink-past-gates-reassessed): every key-link in every -PLAN.md
// under v013Phase12Dirs is re-resolved against HEAD and its verdict
// logged. This is a reporting test — its own assertions are about
// completeness (TestReassessmentTableIsComplete below), not about the
// verdicts themselves. A link being unpinned is a finding to record, not
// a build failure (D-14): nothing found unpinned here is repaired in this
// phase.
func TestReassessV013Phase12(t *testing.T) {
	repoRoot := gateRepoRoot(t)
	_, rows := reassessV013Phase12(t, repoRoot)

	counts := map[reassessVerdict]int{}
	for _, row := range rows {
		t.Logf("%s", reassessLine(repoRoot, row))
		counts[row.Verdict]++
	}
	t.Logf(
		"total v0.13.x Phase 1-2 key-links reassessed: %d (pinned=%d pinned-via-target=%d unpinned=%d unreadable=%d invalid=%d)",
		len(rows), counts[verdictPinned], counts[verdictPinnedViaTarget],
		counts[verdictUnpinned], counts[verdictUnreadable], counts[verdictInvalid],
	)
}

// TestReassessmentTableIsComplete asserts that no v0.13.x Phase 1-2
// key-link is silently omitted from the reassessment, and that every
// emitted verdict is a member of the closed set — the mechanical
// guarantee behind "every link carries exactly one recorded verdict".
// Its red direction was proven by temporarily adding an early `continue`
// inside reassessV013Phase12's loop before shipping this test; see the
// SUMMARY for the verbatim failing output.
func TestReassessmentTableIsComplete(t *testing.T) {
	repoRoot := gateRepoRoot(t)
	links, rows := reassessV013Phase12(t, repoRoot)

	if len(rows) != len(links) {
		t.Errorf(
			"TestReassessmentTableIsComplete: parsed %d key-links under %v but produced %d verdicts — at least one link was silently omitted from the sweep",
			len(links), v013Phase12Dirs, len(rows),
		)
	}

	for _, row := range rows {
		if !reassessVerdictSet[row.Verdict] {
			t.Errorf("TestReassessmentTableIsComplete: %s — verdict %q is not a member of the closed set", reassessLine(repoRoot, row), row.Verdict)
		}
		if row.Verdict == verdictUnpinned && strings.TrimSpace(row.Reason) == "" {
			t.Errorf("TestReassessmentTableIsComplete: %s — unpinned verdict carries an empty reason", reassessLine(repoRoot, row))
		}
	}
}
