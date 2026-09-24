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

	// queries proves paraphraseCase.queries (plan 01-04) is in lockstep with
	// paraphraseTopics: one query per topic, mapped by topic ID, carrying
	// the exact wantKey the topic declares.
	t.Run("queries", func(t *testing.T) {
		t.Parallel()
		if got, want := len(paraphraseCase.queries), len(paraphraseTopics); got != want {
			t.Fatalf("paraphraseCase.queries has %d entries, want %d (len(paraphraseTopics))", got, want)
		}

		wantKeyByTopic := make(map[string]string, len(paraphraseTopics))
		for _, topic := range paraphraseTopics {
			wantKeyByTopic[topic.id] = topic.wantKey
		}

		seenName := make(map[string]bool, len(paraphraseCase.queries))
		for _, q := range paraphraseCase.queries {
			if seenName[q.name] {
				t.Errorf("duplicate query name %q", q.name)
			}
			seenName[q.name] = true

			wantKey, ok := wantKeyByTopic[q.name]
			if !ok {
				t.Errorf("query name %q is not a topic id in paraphraseTopics", q.name)
				continue
			}
			if q.wantKey != wantKey {
				t.Errorf("%s: wantKey = %q, want %q (paraphraseTopics)", q.name, q.wantKey, wantKey)
			}

			if strings.TrimSpace(q.text) == "" {
				t.Errorf("%s: text is empty", q.name)
			}
			if strings.ContainsAny(q.text, "\"`") {
				t.Errorf("%s: text %q contains a quote or backtick", q.name, q.text)
			}
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

	t.Run("size", func(t *testing.T) {
		t.Parallel()

		if n := len(paraphraseSeeds); n < 80 || n > 120 {
			t.Errorf("paraphraseSeeds has %d records, want 80..120", n)
		}

		domains := make(map[string]bool, 8)
		for _, rec := range paraphraseSeeds {
			domains[paraphraseDomain(rec.key)] = true
		}
		if n := len(domains); n < 6 {
			t.Errorf("paraphraseSeeds spans %d distinct domains, want at least 6", n)
		}

		var answerTopics, noAnswerTopics int
		for _, topic := range paraphraseTopics {
			if topic.wantKey == "" {
				noAnswerTopics++
			} else {
				answerTopics++
			}
		}
		if answerTopics < 18 || answerTopics > 22 {
			t.Errorf("paraphraseTopics has %d answer topics, want 18..22", answerTopics)
		}
		if noAnswerTopics < 3 || noAnswerTopics > 5 {
			t.Errorf("paraphraseTopics has %d no-answer topics, want 3..5", noAnswerTopics)
		}

		// No no-answer topic's label may leak the wording of ANY seed's
		// content — the same tokenizer rule as label-leak, applied against
		// the whole corpus rather than a single target (D-12).
		for _, topic := range paraphraseTopics {
			if topic.wantKey != "" {
				continue
			}
			labelTokens := leakTokens(topic.label)
			for _, rec := range paraphraseSeeds {
				contentTokens := leakTokens(rec.content)
				shared := 0
				for tok := range labelTokens {
					if contentTokens[tok] {
						shared++
					}
				}
				if shared > 2 {
					t.Errorf("%s (no-answer): label shares %d qualifying tokens with %s's content (want at most 2)", topic.id, shared, rec.key)
				}
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

// TestRetrievalCasesRoles is hermetic (no Qdrant, no embedder) and pins the
// dataset's two roles (plan 01-04): retrievalCases holds exactly one
// roleRegressionGuard case (gh261Case, with two queries both targeting
// recordTKey) and exactly one roleParaphrase case (paraphraseCase).
func TestRetrievalCasesRoles(t *testing.T) {
	t.Parallel()

	var guardCases, paraphraseCases []retrievalCase
	for _, tc := range retrievalCases {
		switch tc.role {
		case roleRegressionGuard:
			guardCases = append(guardCases, tc)
		case roleParaphrase:
			paraphraseCases = append(paraphraseCases, tc)
		}
	}

	if got, want := len(guardCases), 1; got != want {
		t.Fatalf("retrievalCases holds %d roleRegressionGuard case(s), want %d", got, want)
	}
	if guardCases[0].name != gh261Case.name {
		t.Errorf("the sole roleRegressionGuard case is %q, want %q (gh261Case)", guardCases[0].name, gh261Case.name)
	}
	if got, want := len(guardCases[0].queries), 2; got != want {
		t.Fatalf("gh261Case has %d queries, want %d", got, want)
	}
	for _, q := range guardCases[0].queries {
		if q.wantKey != recordTKey {
			t.Errorf("gh261Case query %q has wantKey %q, want %q (recordTKey)", q.name, q.wantKey, recordTKey)
		}
	}

	if got, want := len(paraphraseCases), 1; got != want {
		t.Fatalf("retrievalCases holds %d roleParaphrase case(s), want %d", got, want)
	}
	if paraphraseCases[0].name != paraphraseCase.name {
		t.Errorf("the sole roleParaphrase case is %q, want %q (paraphraseCase)", paraphraseCases[0].name, paraphraseCase.name)
	}
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
