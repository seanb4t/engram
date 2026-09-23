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

// retrievalQuery is one query run against a case's seeded corpus. wantKey
// identifies the seedRecord.key this specific query should surface; an
// empty value marks a no-answer query (D-12) — excluded from recall@k/MRR
// and logged only (top hit and its score, per variant, never gated).
type retrievalQuery struct {
	name    string
	text    string
	wantKey string
}

// caseRole classifies what a retrievalCase's queries are FOR, so
// TestRetrievalEval can apply the right D-10 gate and D-05 eligibility
// clause to a case's queries without a name-based special case.
type caseRole int

const (
	// roleRegressionGuard marks the permanent GitHub #261 near-verbatim
	// crowding fixture: its queries carry D-10 gate 1 (shipped target at
	// rank 1) and feed D-05 eligibility clause (a).
	roleRegressionGuard caseRole = iota
	// roleParaphrase marks the independent, blind-authored paraphrase case:
	// its answer queries feed D-05 eligibility clause (b) and D-10 gate 2
	// (shipped paraphrase MRR >= vector-only paraphrase MRR); its no-answer
	// queries are logged only (D-12).
	roleParaphrase
)

// retrievalCase is a labeled retrieval scenario: a seeded corpus and one or
// more queries, each carrying its own target via retrievalQuery.wantKey.
type retrievalCase struct {
	name        string
	seedRecords []seedRecord
	queries     []retrievalQuery
	role        caseRole
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
// near-verbatim restatements of Record T. Plan 01 captured Record T's pre-fix
// rank as a t.Logf baseline; Plan 03 flipped "T within default k" to a hard,
// RANK-based t.Errorf acceptance bar; plan 04 tightens it further to rank 1
// specifically (D-10 gate 1) against the shipped ranking (the same shared
// store.SearchReranked helper deps.searchMemory/engramAPI.SearchMemories
// call) — a permanent regression guard. Content and query text are
// byte-identical to before this restructure (D-04); only the Go struct shape
// changed, per Pitfall 3's rationale for deleting the case-level wantKey.
var gh261Case = retrievalCase{
	name: "gh261-sticky-neighbor-crowding",
	role: roleRegressionGuard,
	seedRecords: append([]seedRecord{
		{
			key:     recordTKey,
			content: "Run `task lint` before every commit; golangci-lint's config lives in .golangci.yaml and must stay clean.",
			tags:    []string{"lint", "task", "golangci-lint"},
		},
	}, gh261Distractors...),
	queries: []retrievalQuery{
		{name: "query-a", text: "Before committing, run `task lint`; the golangci-lint config is .golangci.yaml and it needs to stay clean.", wantKey: recordTKey},
		{name: "query-b", text: "Run `task lint` prior to every commit — golangci-lint's config file is .golangci.yaml and must remain clean.", wantKey: recordTKey},
	},
}

// retrievalCases is the full labeled dataset TestRetrievalEval runs.
var retrievalCases = []retrievalCase{gh261Case}

// differProbe is the synthetic single-string fixture for
// TestRetrievalEval_AsymmetryDiffer (the Pitfall-12 correctness gate). It is
// embedded TWICE through the production embedder — query-side via
// em.EmbedQuery, document-side via em.Embed — and the two resulting vectors
// MUST differ once asymmetric embedding is correctly configured. Content is
// synthetic public-tooling text (mirrors the gh261Distractors "no secrets, no
// private content" convention): it names no real repo, person, or credential,
// so nothing sensitive leaves for the third-party embedding API during a live
// eval run (T-14-01).
//
// The differ probe is never seeded into Qdrant (no seedRecord/retrievalCase
// wrapper) — it only compares two in-memory vectors, so it lives here as a
// permanent sibling fixture to gh261Case per D-05, not inside retrievalCases.
//
// Correct mechanism: ENGRAM_EMBED_QUERY_INSTRUCTION / ENGRAM_EMBED_DOCUMENT_INSTRUCTION
// (the text-prefix path gemini-embedding-2 actually honors). The differ-case
// exists specifically to catch an operator wiring the no-op
// ENGRAM_EMBED_QUERY_PARAMS / ENGRAM_EMBED_DOCUMENT_PARAMS / task_type
// mechanism instead (D-03, research supersession) — that misconfiguration
// sends the same raw string both sides, so query and document vectors come
// back identical.
const differProbe = "Run `task lint` before every commit; golangci-lint's config lives in .golangci.yaml."

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
