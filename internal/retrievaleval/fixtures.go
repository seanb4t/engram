// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package retrievaleval

// seedRecord is one document seeded into the eval's Qdrant collection through
// the exact production doc-embed sequence (store.EmbedText -> em.Embed ->
// st.Upsert). key is a fixture-local identifier — NOT the Qdrant point id
// (Qdrant requires a real UUID or uint64; the eval mints one at seed time) —
// used only to track which seeded record a case's query expects back.
type seedRecord struct {
	key     string
	content string
	tags    []string
}

// retrievalQuery is one query run against a case's seeded corpus.
type retrievalQuery struct {
	name string
	text string
}

// retrievalCase is a labeled retrieval scenario: a seeded corpus, one or more
// queries, and the seedRecord.key every query in the case should surface.
type retrievalCase struct {
	name        string
	seedRecords []seedRecord
	queries     []retrievalQuery
	wantKey     string
}

// recordTKey identifies Record T — the GitHub #261 target record — within
// gh261Case.seedRecords.
const recordTKey = "record-t"

// gh261Distractors are sticky topical neighbors: engram's own public tooling
// conventions that share vocabulary with Record T (task/lint/CI) but describe
// different tools or targets, so they crowd Record T out of a naive top-k
// without duplicating it. 15 distractors — comfortably above the
// defaultK+1 (9) floor review finding 4 requires, so "T within default k" is a
// genuinely hard bar, not trivially true. Content is synthetic (public OSS
// tooling conventions) — no secrets.
var gh261Distractors = []seedRecord{
	{key: "distractor-01", content: "Run `task fmt` before committing; it applies gofmt, dprint, and yamlfmt.", tags: []string{"fmt", "task"}},
	{key: "distractor-02", content: "The bare `task` target runs lint then test; CI invokes it directly.", tags: []string{"task", "ci"}},
	{key: "distractor-03", content: "yamlfmt formatting is enforced via `task fmt`; a diff there fails CI.", tags: []string{"yamlfmt", "task"}},
	{key: "distractor-04", content: "actionlint checks GitHub Actions workflow YAML; wired into `task lint`.", tags: []string{"actionlint", "task"}},
	{key: "distractor-05", content: "rumdl lints Markdown files; part of the `task lint` aggregate target.", tags: []string{"rumdl", "task"}},
	{key: "distractor-06", content: "`task test` runs `go test ./...`, including the Qdrant-backed integration suites.", tags: []string{"test", "task"}},
	{key: "distractor-07", content: "`task proto:lint` runs buf lint against the proto/ tree before generation.", tags: []string{"buf", "task"}},
	{key: "distractor-08", content: "CI's required test job runs `go test ./...` plus gofmt and go mod tidy checks.", tags: []string{"ci", "test"}},
	{key: "distractor-09", content: "`task license:check` verifies every Go and Markdown file carries the Apache-2.0 SPDX header.", tags: []string{"license", "task"}},
	{key: "distractor-10", content: "`task chart:lint` runs helm lint against charts/engram.", tags: []string{"helm", "task"}},
	{key: "distractor-11", content: "`golangci-lint run ./...` is the single entrypoint `task lint` wraps for Go static analysis.", tags: []string{"golangci-lint", "task"}},
	{key: "distractor-12", content: "`task bench` runs `go test -bench=. -benchmem ./...` for performance regressions.", tags: []string{"bench", "task"}},
	{key: "distractor-13", content: "Prefer `task fmt` before `task lint` locally; CI runs them in that order too.", tags: []string{"fmt", "lint", "task"}},
	{key: "distractor-14", content: "dprint formats non-Go files (JSON, TOML, Markdown) as part of `task fmt`.", tags: []string{"dprint", "task"}},
	{key: "distractor-15", content: "`task proto:gen` regenerates the committed gen/ tree via buf; CI checks for drift.", tags: []string{"buf", "task"}},
}

// gh261Case is the permanent GitHub #261 regression fixture: Record T plus the
// 15 sticky topical-neighbor distractors above, and two queries (A/B) that are
// near-verbatim restatements of Record T. This plan captures Record T's
// pre-fix rank as the baseline (t.Logf); Plan 03 flips "T within default k" to
// a hard rank assertion once the eval-selected ranking fix lands.
var gh261Case = retrievalCase{
	name: "gh261-sticky-neighbor-crowding",
	seedRecords: append([]seedRecord{
		{
			key:     recordTKey,
			content: "Run `task lint` before every commit; golangci-lint's config lives in .golangci.yaml and must stay clean.",
			tags:    []string{"lint", "task", "golangci-lint"},
		},
	}, gh261Distractors...),
	queries: []retrievalQuery{
		{name: "query-a", text: "Before committing, run `task lint`; the golangci-lint config is .golangci.yaml and it needs to stay clean."},
		{name: "query-b", text: "Run `task lint` prior to every commit — golangci-lint's config file is .golangci.yaml and must remain clean."},
	},
	wantKey: recordTKey,
}

// retrievalCases is the full labeled dataset TestRetrievalEval runs.
var retrievalCases = []retrievalCase{gh261Case}

// recallAtK reports whether wantID appears anywhere in results (results is
// assumed already limited to k by the caller's Search call).
func recallAtK(results []string, wantID string) bool {
	for _, id := range results {
		if id == wantID {
			return true
		}
	}
	return false
}

// reciprocalRank returns 1/rank of wantID within results (1-indexed), or 0 if
// wantID is absent.
func reciprocalRank(results []string, wantID string) float64 {
	for i, id := range results {
		if id == wantID {
			return 1.0 / float64(i+1)
		}
	}
	return 0
}
