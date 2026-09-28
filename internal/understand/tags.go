// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file implements D-09's local tag matching: tag suggestions come
// from matching the query against the caller's own tag vocabulary locally
// — no decision call, ever. Tags are open-vocabulary and sit outside the
// advisory decide contract: unlike categories, scope and time_window, a
// tag can never become a decide.Question.

package understand

import (
	"slices"
	"strings"
	"unicode"
)

// MaxTagSuggestions bounds the number of tag suggestions MatchTags
// returns — mirrors the console tag picker's top-8 display.
const MaxTagSuggestions = 8

// queryTokens splits q into its maximal runs of Unicode letters, digits,
// '-' and '_', lowercased. A hyphenated compound (e.g. "qdrant-ops")
// written contiguously in the query stays one token; queryTokens does not
// itself split on hyphens (tagMatches does that, against the vocabulary
// tag).
func queryTokens(q string) map[string]struct{} {
	fields := strings.FieldsFunc(q, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_'
	})
	out := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		out[strings.ToLower(f)] = struct{}{}
	}
	return out
}

// tagMatches reports whether tag matches any token in tokens per D-09's
// rule: the whole tag (case-insensitive) equals a token, or one of the
// tag's hyphen-separated parts of 3 or more runes equals a token — so
// "qdrant-ops" matches a query containing "qdrant" (a 6-rune part), but
// "go-to" does not match a query containing "go" or "to" alone (both
// 2-rune parts).
func tagMatches(tag string, tokens map[string]struct{}) bool {
	lower := strings.ToLower(tag)
	if _, ok := tokens[lower]; ok {
		return true
	}
	for _, part := range strings.Split(lower, "-") {
		if len([]rune(part)) < 3 {
			continue
		}
		if _, ok := tokens[part]; ok {
			return true
		}
	}
	return false
}

// MatchTags matches query's tokens against vocab (D-09), skipping any tag
// already present in applied (exact string compare). Matches are returned
// in vocab order, capped at MaxTagSuggestions, each carrying the tag's
// original case (Kind KindTag, Source SourceMatched). No decision call is
// ever made — see tagMatches for the match rule.
func MatchTags(query string, vocab, applied []string) []Suggestion {
	if len(vocab) == 0 {
		return nil
	}
	tokens := queryTokens(query)
	var out []Suggestion
	for _, tag := range vocab {
		if len(out) >= MaxTagSuggestions {
			break
		}
		if slices.Contains(applied, tag) {
			continue
		}
		if tagMatches(tag, tokens) {
			out = append(out, Suggestion{Kind: KindTag, Value: tag, Source: SourceMatched})
		}
	}
	return out
}
