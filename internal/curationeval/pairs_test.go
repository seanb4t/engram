// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package curationeval

import (
	"flag"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/seanb4t/engram/internal/verdict"
)

// promptOut writes the blind label prompt to this path (plan 03-04); empty
// (the default) skips TestWriteBlindLabelPrompt entirely — regeneration is
// never a side effect of an ordinary test run.
var promptOut = flag.String("prompt-out", "", "write the blind label prompt to this path (plan 03-04)")

// TestPairFixtureIntegrity is hermetic (no network, no decider) and
// enforces D-01/D-02/CUR-03's mechanical rules on syntheticPairs, so the
// corpus cannot silently regress.
func TestPairFixtureIntegrity(t *testing.T) {
	t.Parallel()

	t.Run("ids", func(t *testing.T) {
		t.Parallel()
		if len(syntheticPairs) == 0 {
			t.Fatal("syntheticPairs is empty")
		}
		idPattern := regexp.MustCompile(`^P[0-9]{2}$`)
		last := ""
		for i, p := range syntheticPairs {
			if !idPattern.MatchString(p.id) {
				t.Errorf("pair %d: id %q does not match %s", i, p.id, idPattern.String())
			}
			if i > 0 && p.id <= last {
				t.Errorf("pair %d: id %q is not strictly ascending after %q", i, p.id, last)
			}
			last = p.id
		}
	})

	t.Run("labels", func(t *testing.T) {
		t.Parallel()
		if len(syntheticPairs) == 0 {
			t.Fatal("syntheticPairs is empty")
		}
		valid := make(map[string]bool, len(verdict.Relations()))
		for _, r := range verdict.Relations() {
			valid[r] = true
		}
		for _, p := range syntheticPairs {
			if !valid[p.label] {
				t.Errorf("pair %s: label %q is not one of verdict.Relations()", p.id, p.label)
			}
		}
	})

	t.Run("text", func(t *testing.T) {
		t.Parallel()
		for _, p := range syntheticPairs {
			checkPairText(t, p.id, "recordA", p.recordA)
			checkPairText(t, p.id, "recordB", p.recordB)
			if p.recordA == p.recordB {
				t.Errorf("pair %s: recordA and recordB are byte-identical", p.id)
			}
		}
	})

	t.Run("interleave", func(t *testing.T) {
		t.Parallel()
		if len(syntheticPairs) == 0 {
			t.Fatal("syntheticPairs is empty")
		}
		// Consecutive here means originally adjacent in the P01..P80
		// authored sequence (numeric id difference of 1), not merely
		// adjacent in the (possibly filtered) slice: after Task 3 drops
		// disagreed pairs without renumbering (D-02), two agreed pairs
		// that were never adjacent when the corpus was authored and
		// interleaved can become slice-adjacent purely because a pair
		// between them was removed. That incidental adjacency carries no
		// position-reveals-nothing information about the original
		// sequence, so it must not count toward a run.
		run, longest, prevLabel, prevNum := 0, 0, "", -2
		for _, p := range syntheticPairs {
			num := pairIDNumber(t, p.id)
			if p.label == prevLabel && num == prevNum+1 {
				run++
			} else {
				run = 1
			}
			prevLabel, prevNum = p.label, num
			if run > longest {
				longest = run
			}
		}
		if longest > 2 {
			t.Errorf("longest run of consecutive (originally-adjacent) same-label pairs is %d, want at most 2", longest)
		}
	})

	t.Run("size", func(t *testing.T) {
		t.Parallel()
		if n := len(syntheticPairs); n < 50 || n > 80 {
			t.Errorf("syntheticPairs has %d pairs, want 50..80", n)
		}
		counts := make(map[string]int, len(verdict.Relations()))
		for _, p := range syntheticPairs {
			counts[p.label]++
		}
		for _, r := range verdict.Relations() {
			if counts[r] < 7 {
				t.Errorf("relation %q has %d pairs, want at least 7", r, counts[r])
			}
		}
	})

	t.Run("denylist", func(t *testing.T) {
		t.Parallel()

		skOrGhpToken := regexp.MustCompile(`\b(?:sk-|ghp_)[A-Za-z0-9]{16,}\b`)
		githubPAT := regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]*`)
		akiaKey := regexp.MustCompile(`\bAKIA[A-Z0-9]{16}\b`)
		privateKeyMarker := regexp.MustCompile(`PRIVATE KEY`)
		passwordOrSecret := regexp.MustCompile(`(?i)\b(?:password|secret)\s*=`)
		emailAddress := regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)

		denylist := []*regexp.Regexp{skOrGhpToken, githubPAT, akiaKey, privateKeyMarker, passwordOrSecret, emailAddress}

		// Positive controls: each regexp must match its own planted
		// example, so a future edit that loosens a pattern into a no-op is
		// caught here rather than by an absence of findings below.
		positiveControls := []struct {
			pattern *regexp.Regexp
			example string
		}{
			{skOrGhpToken, "token sk-abcdefghijklmnop end"},
			{skOrGhpToken, "token ghp_abcdefghijklmnop end"},
			{githubPAT, "uses github_pat_11ABCDEFG0abcdefghijkl"},
			{akiaKey, "key AKIAABCDEFGHIJKLMNOP"},
			{privateKeyMarker, "-----BEGIN PRIVATE KEY-----"},
			{passwordOrSecret, "password = hunter2"},
			{passwordOrSecret, "secret=hunter2"},
			{emailAddress, "contact ops@example.com for access"},
		}
		for _, pc := range positiveControls {
			if !pc.pattern.MatchString(pc.example) {
				t.Errorf("positive control failed: %q should match %s", pc.example, pc.pattern.String())
			}
		}

		// Negative controls: ordinary hyphenated vocabulary must never trip
		// the denylist, proving the patterns are anchored rather than bare
		// substrings.
		negativeControls := []string{"task-runner", "disk-backed"}
		for _, neg := range negativeControls {
			for _, re := range denylist {
				if re.MatchString(neg) {
					t.Errorf("negative control failed: %q unexpectedly matched %s", neg, re.String())
				}
			}
		}

		bannedWords := []string{"engram", "qdrant", "koanf"}
		checkText := func(where, text string) {
			for _, re := range denylist {
				if re.MatchString(text) {
					t.Errorf("%s: matched denylisted pattern %s: %q", where, re.String(), text)
				}
			}
			lower := strings.ToLower(text)
			for _, word := range bannedWords {
				if strings.Contains(lower, word) {
					t.Errorf("%s: contains banned identifier %q", where, word)
				}
			}
		}

		for _, p := range syntheticPairs {
			checkText(p.id+" recordA", p.recordA)
			checkText(p.id+" recordB", p.recordB)
		}
	})
}

// pairIDNumber parses the numeric suffix of a "Pnn" id (already validated by
// the "ids" subtest's pattern), for the "interleave" subtest's originally-
// adjacent check.
func pairIDNumber(t *testing.T, id string) int {
	t.Helper()
	n, err := strconv.Atoi(strings.TrimPrefix(id, "P"))
	if err != nil {
		t.Fatalf("id %q: %v", id, err)
	}
	return n
}

// checkPairText enforces the text subtest's per-record rules: 40 to 600
// Unicode runes, and equal to its own strings.TrimSpace (no leading or
// trailing whitespace).
func checkPairText(t *testing.T, id, field, text string) {
	t.Helper()
	if n := utf8.RuneCountInString(text); n < 40 || n > 600 {
		t.Errorf("pair %s: %s is %d runes, want 40..600", id, field, n)
	}
	if text != strings.TrimSpace(text) {
		t.Errorf("pair %s: %s has leading or trailing whitespace", id, field)
	}
}

// TestBlindLabelPromptIsLabelIndependent proves blindLabelPrompt never
// reads label: rendering syntheticPairs and a copy with every label
// rotated to the next relation produces byte-identical output, which also
// carries each pair's id heading and both texts verbatim, every relation
// criterion (verdict.RelationCriteria()) verbatim, and no "label:" line.
func TestBlindLabelPromptIsLabelIndependent(t *testing.T) {
	t.Parallel()

	original := blindLabelPrompt(syntheticPairs)

	relations := verdict.Relations()
	rotated := make([]labeledPair, len(syntheticPairs))
	for i, p := range syntheticPairs {
		idx := slices.Index(relations, p.label)
		if idx < 0 {
			t.Fatalf("pair %s: label %q is not one of verdict.Relations()", p.id, p.label)
		}
		rotated[i] = labeledPair{
			id:      p.id,
			label:   relations[(idx+1)%len(relations)],
			recordA: p.recordA,
			recordB: p.recordB,
		}
	}
	rotatedPrompt := blindLabelPrompt(rotated)

	if original != rotatedPrompt {
		t.Fatal("blindLabelPrompt is not label-independent: rotating every label changed the output")
	}

	for _, p := range syntheticPairs {
		if !strings.Contains(original, "### "+p.id) {
			t.Errorf("prompt missing heading for %s", p.id)
		}
		if !strings.Contains(original, p.recordA) {
			t.Errorf("prompt missing recordA verbatim for %s", p.id)
		}
		if !strings.Contains(original, p.recordB) {
			t.Errorf("prompt missing recordB verbatim for %s", p.id)
		}
	}

	for name, criteria := range verdict.RelationCriteria() {
		if !strings.Contains(original, criteria) {
			t.Errorf("prompt missing criteria for %q verbatim", name)
		}
	}

	for _, line := range strings.Split(original, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "label:") {
			t.Errorf("prompt leaks a label: line: %q", line)
		}
	}
}

// TestWriteBlindLabelPrompt skips unless -prompt-out is set. When set, it
// writes the provenance paragraph, blindLabelMarker, and
// blindLabelPrompt(syntheticPairs) to that path — the standard Go golden
// idiom (cmd/engram/golden_test.go's -update), never a side effect of an
// ordinary test run.
func TestWriteBlindLabelPrompt(t *testing.T) {
	if *promptOut == "" {
		t.Skip("set -prompt-out=<path> to regenerate the blind label prompt (plan 03-04)")
	}

	var b strings.Builder
	b.WriteString("Generated by TestWriteBlindLabelPrompt from syntheticPairs in internal/curationeval/pairs.go. This file carries no intended label (D-02) — the entire text below the marker line is sent to the blind labeler verbatim.\n\n")
	b.WriteString(blindLabelMarker)
	b.WriteString("\n\n")
	b.WriteString(blindLabelPrompt(syntheticPairs))

	if err := os.WriteFile(*promptOut, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write %s: %v", *promptOut, err)
	}
}
