// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package retrievaleval

import (
	"regexp"
	"strings"
	"testing"
)

// TestParaphraseCorpusIntegrity is hermetic (no Qdrant, no embedder, no
// ENGRAM_RETRIEVAL_EVAL gate) and enforces D-01..D-03 on paraphraseSeeds and
// paraphraseTopics mechanically, so the paraphrase corpus cannot silently
// regress. It never seeds Qdrant; it only walks the in-memory fixture data.
func TestParaphraseCorpusIntegrity(t *testing.T) {
	t.Parallel()

	t.Run("keys", func(t *testing.T) {
		t.Parallel()
		keyPattern := regexp.MustCompile(`^[a-z]+-[0-9]{2}$`)
		seen := make(map[string]bool, len(paraphraseSeeds))
		for _, rec := range paraphraseSeeds {
			if seen[rec.key] {
				t.Errorf("duplicate key %q", rec.key)
			}
			seen[rec.key] = true

			if !keyPattern.MatchString(rec.key) {
				t.Errorf("key %q does not match %s", rec.key, keyPattern.String())
			}

			n := len(rec.content)
			if n < 60 || n > 400 {
				t.Errorf("%s: content is %d bytes, want 60..400", rec.key, n)
			}
			if strings.TrimSpace(rec.content) == "" {
				t.Errorf("%s: content is empty", rec.key)
			}

			if len(rec.tags) < 2 || len(rec.tags) > 4 {
				t.Errorf("%s: has %d tags, want 2..4", rec.key, len(rec.tags))
			}
			for _, tag := range rec.tags {
				if tag == "" {
					t.Errorf("%s: has an empty tag", rec.key)
				}
				if tag != strings.ToLower(tag) {
					t.Errorf("%s: tag %q is not lowercase", rec.key, tag)
				}
			}
		}
	})

	t.Run("topics", func(t *testing.T) {
		t.Parallel()
		idPattern := regexp.MustCompile(`^T[0-9]{2}$`)
		seedByKey := make(map[string]bool, len(paraphraseSeeds))
		for _, rec := range paraphraseSeeds {
			seedByKey[rec.key] = true
		}

		seenID := make(map[string]bool, len(paraphraseTopics))
		seenWantKey := make(map[string]string, len(paraphraseTopics))
		for _, topic := range paraphraseTopics {
			if seenID[topic.id] {
				t.Errorf("duplicate topic id %q", topic.id)
			}
			seenID[topic.id] = true

			if !idPattern.MatchString(topic.id) {
				t.Errorf("topic id %q does not match %s", topic.id, idPattern.String())
			}

			const labelPrefix = "the record about "
			if !strings.HasPrefix(topic.label, labelPrefix) {
				t.Errorf("%s: label %q does not start with %q", topic.id, topic.label, labelPrefix)
			}
			if words := len(strings.Fields(topic.label)); words > 12 {
				t.Errorf("%s: label has %d words, want at most 12", topic.id, words)
			}

			if topic.wantKey == "" {
				continue // no-answer topic (D-12) — nothing further to check here.
			}
			if !seedByKey[topic.wantKey] {
				t.Errorf("%s: wantKey %q does not exist in paraphraseSeeds", topic.id, topic.wantKey)
			}
			if other, ok := seenWantKey[topic.wantKey]; ok {
				t.Errorf("wantKey %q is shared by topics %s and %s", topic.wantKey, other, topic.id)
			}
			seenWantKey[topic.wantKey] = topic.id
		}
	})

	t.Run("sticky-neighbours", func(t *testing.T) {
		t.Parallel()
		seedByKey := make(map[string]seedRecord, len(paraphraseSeeds))
		for _, rec := range paraphraseSeeds {
			seedByKey[rec.key] = rec
		}

		for _, topic := range paraphraseTopics {
			if topic.wantKey == "" {
				continue
			}
			target, ok := seedByKey[topic.wantKey]
			if !ok {
				t.Errorf("%s: wantKey %q not found among paraphraseSeeds (see topics subtest)", topic.id, topic.wantKey)
				continue
			}
			targetTags := make(map[string]bool, len(target.tags))
			for _, tag := range target.tags {
				targetTags[tag] = true
			}
			domain := paraphraseDomain(target.key)

			neighbours := 0
			for _, rec := range paraphraseSeeds {
				if rec.key == target.key || paraphraseDomain(rec.key) != domain {
					continue
				}
				for _, tag := range rec.tags {
					if targetTags[tag] {
						neighbours++
						break
					}
				}
			}
			if neighbours < 3 {
				t.Errorf("%s: target %q has only %d same-domain sticky neighbours (sharing a tag), want at least 3", topic.id, target.key, neighbours)
			}
		}
	})

	t.Run("label-leak", func(t *testing.T) {
		t.Parallel()
		seedByKey := make(map[string]seedRecord, len(paraphraseSeeds))
		for _, rec := range paraphraseSeeds {
			seedByKey[rec.key] = rec
		}

		for _, topic := range paraphraseTopics {
			if topic.wantKey == "" {
				continue
			}
			target, ok := seedByKey[topic.wantKey]
			if !ok {
				continue // reported by topics subtest.
			}
			labelTokens := leakTokens(topic.label)
			contentTokens := leakTokens(target.content)

			shared := 0
			for tok := range labelTokens {
				if contentTokens[tok] {
					shared++
				}
			}
			if shared > 2 {
				t.Errorf("%s: label shares %d qualifying tokens with target %q's content (want at most 2)", topic.id, shared, target.key)
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

		// Positive controls: each regexp must match its own planted example,
		// so a future edit that loosens a pattern into a no-op is caught
		// here rather than by an absence of findings below.
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

		// Negative controls: ordinary hyphenated tooling vocabulary must
		// never trip the denylist, proving the patterns are anchored rather
		// than bare substrings.
		negativeControls := []string{"task-runner", "disk-backed"}
		for _, neg := range negativeControls {
			for _, re := range denylist {
				if re.MatchString(neg) {
					t.Errorf("negative control failed: %q unexpectedly matched %s", neg, re.String())
				}
			}
		}

		gh261Content := make(map[string]bool, len(gh261Case.seedRecords))
		for _, rec := range gh261Case.seedRecords {
			gh261Content[rec.content] = true
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

		for _, rec := range paraphraseSeeds {
			checkText(rec.key+" content", rec.content)
			for _, tag := range rec.tags {
				checkText(rec.key+" tag", tag)
			}
			if gh261Content[rec.content] {
				t.Errorf("%s: content is verbatim gh261Case content — the paraphrase corpus must be independent", rec.key)
			}
		}
	})
}

// leakTokens tokenizes s into lowercase alphanumeric runs, keeping only
// tokens of 4 or more letters and dropping "record" and "about" — the same
// rule the label-leak subtest and D-01 use to decide whether a topic label
// leaks its target's wording.
func leakTokens(s string) map[string]bool {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	out := make(map[string]bool, len(fields))
	for _, f := range fields {
		if len(f) < 4 || f == "record" || f == "about" {
			continue
		}
		out[f] = true
	}
	return out
}
