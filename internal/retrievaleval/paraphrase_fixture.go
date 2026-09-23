// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package retrievaleval

// This file is the independent paraphrase corpus for RANK-01 (#605), written
// with synthetic public-style content only (D-02) about fictional projects —
// never engram's own tooling. It exists so RANK-02's keep/demote/replace
// decision for the lexical reranker rests on a corpus and a query set nobody
// tuned to make a particular ranker look good: the spike 004 fixture failed
// this bar because its queries were written by someone who could see the
// targets (see .planning/spikes/004-jev-rerank-eval/README.md).
//
// paraphraseTopics is the ONLY thing the blind query author (D-01) sees —
// generated into 01-BLIND-QUERY-PROMPT.md as labels alone, with no seed text,
// key, or tag included. Plan 01-04 wraps this corpus and the blind queries
// recorded in 01-BLIND-QUERIES.md into a paraphraseCase retrievalCase once
// the blind pass is done.
//
// This slice (task 1 of plan 01-03) is one domain — tooling — proving the
// whole shape end to end: 16 records, 4 answer targets each with at least 3
// sticky same-domain neighbours sharing a tag and tool vocabulary but
// answering a different question (the spike 004 failure shape). Task 2 adds
// five more domains for a 96-record, 24-topic corpus.

// paraphraseTopic is a single blind-query target. label is what the blind
// author sees; wantKey is the paraphraseSeeds key the answer should surface,
// or "" for a no-answer topic (D-12) whose subject exists nowhere in the
// corpus.
type paraphraseTopic struct {
	id, label, wantKey string
}

// paraphraseDomain returns the domain prefix of a paraphraseSeeds key — the
// text before its first hyphen, e.g. "tooling" for "tooling-01".
func paraphraseDomain(key string) string {
	for i := 0; i < len(key); i++ {
		if key[i] == '-' {
			return key[:i]
		}
	}
	return key
}

// paraphraseSeeds is the synthetic multi-domain corpus. Every record
// describes a fictional project ("the ledger service") using generic public
// OSS tooling vocabulary (golangci-lint, task runners, CI pipelines) — never
// an engram-specific identifier. This slice holds only the tooling domain;
// task 2 appends auth, deploy, datamodel, config and testing.
var paraphraseSeeds = []seedRecord{
	// Cluster: pre-commit hook (target tooling-01).
	{key: "tooling-01", content: "The pre-commit hook in the ledger service runs gofmt and golangci-lint before it allows a commit to land.", tags: []string{"precommit", "hook", "lint"}},
	{key: "tooling-02", content: "CI runs the same lint step the pre-commit hook runs, but it blocks the merge instead of the commit.", tags: []string{"ci", "hook", "lint"}},
	{key: "tooling-03", content: "Developers can skip the pre-commit hook locally with a flag, but CI still enforces the same checks.", tags: []string{"precommit", "hook", "ci"}},
	{key: "tooling-04", content: "The pre-commit hook was rewritten from a shell script into a small Go binary for startup speed.", tags: []string{"precommit", "hook", "tooling"}},

	// Cluster: formatter task order (target tooling-05).
	{key: "tooling-05", content: "The formatter runs gofmt first, then a JSON and YAML formatter, in that fixed order, as the format task.", tags: []string{"formatter", "task", "order"}},
	{key: "tooling-06", content: "The lint task always runs after the format task in the aggregate task target, never before it.", tags: []string{"lint", "task", "order"}},
	{key: "tooling-07", content: "The test task runs the full suite, including the integration tests behind a build tag.", tags: []string{"test", "task", "tooling"}},
	{key: "tooling-08", content: "The format task also strips trailing whitespace from every Markdown file in the repository.", tags: []string{"formatter", "task", "markdown"}},

	// Cluster: lint config location (target tooling-09).
	{key: "tooling-09", content: "The lint config file for the ledger service lives at the repo root as a YAML file and must stay in sync with CI.", tags: []string{"lint", "config", "ci"}},
	{key: "tooling-10", content: "The formatter's own config lives in a separate JSON file, also kept at the repo root.", tags: []string{"formatter", "config", "tooling"}},
	{key: "tooling-11", content: "CI caches the lint tool's binary between runs so it does not re-download it on every build.", tags: []string{"lint", "ci", "cache"}},
	{key: "tooling-12", content: "A stale lint config once caused a false pass on the release branch until someone noticed the version drift.", tags: []string{"lint", "config", "release"}},

	// Cluster: CI job order (target tooling-13).
	{key: "tooling-13", content: "CI runs the lint job before the test job in the ledger service pipeline, so a lint failure blocks the tests from starting.", tags: []string{"ci", "order", "lint"}},
	{key: "tooling-14", content: "The test job in CI runs with the race detector turned on by default for every package.", tags: []string{"ci", "test", "tooling"}},
	{key: "tooling-15", content: "The build job runs before both lint and test in CI, so a compile error fails fast.", tags: []string{"ci", "order", "build"}},
	{key: "tooling-16", content: "A flaky integration test in the test job was quarantined behind a skip flag until someone fixes it.", tags: []string{"ci", "test", "flaky"}},
}

// paraphraseTopics lists the blind-author-facing topics for the tooling
// domain: one per answer target above. Every label starts with "the record
// about " and is at most 12 words (D-01), sharing at most 2 content words
// with its target so a reader of the label alone cannot reproduce the
// record's wording.
var paraphraseTopics = []paraphraseTopic{
	{id: "T01", label: "the record about what happens to code before it lands", wantKey: "tooling-01"},
	{id: "T02", label: "the record about what order the style tools run in", wantKey: "tooling-05"},
	{id: "T03", label: "the record about where the style checker settings live", wantKey: "tooling-09"},
	{id: "T04", label: "the record about which pipeline step happens first", wantKey: "tooling-13"},
}
