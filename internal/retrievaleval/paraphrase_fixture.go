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
// The full corpus (plan 01-03, tasks 1-2) spans six domains — tooling, auth,
// deploy, datamodel, config and testing — 96 records in total. 20 of them are
// answer targets, each with at least 3 same-domain sticky neighbours sharing
// a tag and tool vocabulary but answering a different question (the spike
// 004 failure shape); the remaining 24 topics split into those 20 plus 4
// no-answer topics (D-12) whose subject exists nowhere in the corpus.

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

// paraphraseSeeds is the synthetic multi-domain corpus: 96 records across six
// domains (tooling, auth, deploy, datamodel, config, testing), each about a
// fictional project ("the ledger service", "the ingest worker", "the catalog
// service", "the web app", "the search API") using generic public-style
// vocabulary — never an engram-specific identifier.
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

	// --- auth domain: the ledger service's authentication conventions. ---

	// Cluster: access/refresh token lifetimes (target auth-01).
	{key: "auth-01", content: "Access tokens for the ledger service expire after fifteen minutes, and refresh tokens stay valid for seven days.", tags: []string{"token", "lifetime", "auth"}},
	{key: "auth-02", content: "Refresh tokens are stored hashed in the database so a leaked table alone cannot be replayed.", tags: []string{"refresh", "token", "storage"}},
	{key: "auth-03", content: "A refresh token rotates on every use and the previous one is revoked the moment the new one issues.", tags: []string{"refresh", "rotation", "token"}},
	{key: "auth-04", content: "Access tokens are signed JWTs, while refresh tokens are opaque strings looked up on the server.", tags: []string{"token", "jwt", "auth"}},

	// Cluster: service account credentials (target auth-05).
	{key: "auth-05", content: "Service accounts for the ledger service authenticate with a long-lived API key instead of a user login flow.", tags: []string{"service-account", "credential", "auth"}},
	{key: "auth-06", content: "A service account key can be rotated from the admin panel without downtime for the caller.", tags: []string{"service-account", "credential", "rotation"}},
	{key: "auth-07", content: "Service accounts are scoped to a single project and cannot read across project boundaries.", tags: []string{"service-account", "scope", "auth"}},
	{key: "auth-08", content: "A revoked service account key returns an unauthorized response immediately, with no propagation delay.", tags: []string{"service-account", "credential", "auth"}},

	// Cluster: audience claim validation (target auth-09).
	{key: "auth-09", content: "Every access token's audience claim is checked against the calling service's expected identifier before it's trusted.", tags: []string{"audience", "claim", "auth"}},
	{key: "auth-10", content: "A token issued for one audience is rejected outright if presented to a different service.", tags: []string{"audience", "token", "auth"}},
	{key: "auth-11", content: "The issuer claim is checked alongside the audience claim, both against a fixed allowlist.", tags: []string{"audience", "issuer", "auth"}},
	{key: "auth-12", content: "Audience validation happens before signature verification finishes, as a cheap early rejection.", tags: []string{"audience", "claim", "validation"}},

	// Cluster: session cookie flags (target auth-13).
	{key: "auth-13", content: "Session cookies for the ledger service's web console are marked httpOnly, secure, and SameSite=Strict.", tags: []string{"session", "cookie", "auth"}},
	{key: "auth-14", content: "The session cookie's expiry is refreshed on every request, sliding forward rather than fixed.", tags: []string{"session", "cookie", "expiry"}},
	{key: "auth-15", content: "A session cookie is invalidated server-side on logout, not just cleared from the browser.", tags: []string{"session", "cookie", "logout"}},
	{key: "auth-16", content: "Session cookies are never readable from JavaScript, only sent automatically by the browser.", tags: []string{"session", "cookie", "auth"}},

	// --- deploy domain: the ingest worker's release conventions. ---

	// Cluster: canary rollout strategy (target deploy-01).
	{key: "deploy-01", content: "New releases of the ingest worker roll out canary-first: five percent of traffic for ten minutes before a full rollout.", tags: []string{"rollout", "canary", "deploy"}},
	{key: "deploy-02", content: "A canary that trips the error-rate alarm automatically pauses the rollout and pages the on-call engineer.", tags: []string{"rollout", "canary", "alarm"}},
	{key: "deploy-03", content: "The rollout percentage steps up in four stages: five, twenty five, fifty, then one hundred percent.", tags: []string{"rollout", "stages", "deploy"}},
	{key: "deploy-04", content: "Traffic for the canary stage is picked by a sticky hash on the request's tenant ID.", tags: []string{"rollout", "canary", "routing"}},
	{key: "deploy-05", content: "A manual override can skip straight to full rollout for an urgent hotfix, bypassing the canary stages.", tags: []string{"rollout", "override", "deploy"}},
	{key: "deploy-06", content: "Rollout progress is visible on a dashboard that shows the current traffic percentage per version.", tags: []string{"rollout", "dashboard", "deploy"}},

	// Cluster: rollback procedure (target deploy-07).
	{key: "deploy-07", content: "Rolling back the ingest worker redeploys the previous image tag and reverts the rollout percentage to zero for the new one.", tags: []string{"rollback", "deploy", "image"}},
	{key: "deploy-08", content: "A rollback can be triggered from the CLI with a single command naming the target version.", tags: []string{"rollback", "cli", "deploy"}},
	{key: "deploy-09", content: "Database migrations do not automatically roll back; a rollback only reverts application code.", tags: []string{"rollback", "migration", "deploy"}},
	{key: "deploy-10", content: "Rollback history keeps the last ten deployed versions available for a one-command revert.", tags: []string{"rollback", "history", "deploy"}},
	{key: "deploy-11", content: "A failed rollback leaves the worker on the last known-good version rather than a partial state.", tags: []string{"rollback", "deploy", "safety"}},

	// Cluster: migration ordering (target deploy-12).
	{key: "deploy-12", content: "Schema migrations for the ingest worker always run before the new application version starts serving traffic.", tags: []string{"migration", "order", "deploy"}},
	{key: "deploy-13", content: "A migration that would lock a large table runs in a maintenance window, not during a normal deploy.", tags: []string{"migration", "schema", "deploy"}},
	{key: "deploy-14", content: "Migrations are additive-only in this pipeline; a destructive change ships as two separate deploys.", tags: []string{"migration", "order", "schema"}},
	{key: "deploy-15", content: "The migration runner records which migrations already applied, so a rerun is always safe.", tags: []string{"migration", "deploy", "idempotent"}},
	{key: "deploy-16", content: "A migration failure blocks the deploy pipeline entirely rather than partially applying the new version.", tags: []string{"migration", "order", "deploy"}},

	// --- datamodel domain: the catalog service's data conventions. ---

	// Cluster: soft delete (target datamodel-01).
	{key: "datamodel-01", content: "Deleting a catalog item sets a deleted_at timestamp instead of removing the row, so it can be restored.", tags: []string{"softdelete", "delete", "datamodel"}},
	{key: "datamodel-02", content: "Queries against the catalog table filter out soft-deleted rows by default unless explicitly included.", tags: []string{"softdelete", "query", "datamodel"}},
	{key: "datamodel-03", content: "A soft-deleted item is purged for real by a nightly job after ninety days.", tags: []string{"softdelete", "purge", "datamodel"}},
	{key: "datamodel-04", content: "Restoring a soft-deleted item just clears the deleted_at column back to null.", tags: []string{"softdelete", "restore", "datamodel"}},
	{key: "datamodel-05", content: "Soft delete applies to catalog items and orders, but not to audit log rows, which are append-only.", tags: []string{"softdelete", "scope", "datamodel"}},

	// Cluster: ID format (target datamodel-06).
	{key: "datamodel-06", content: "Every catalog item gets a UUIDv7 primary key instead of an auto-incrementing integer, so IDs sort roughly by creation time.", tags: []string{"id", "uuid", "datamodel"}},
	{key: "datamodel-07", content: "Foreign keys reference the UUID primary key directly; no separate integer surrogate key exists.", tags: []string{"id", "foreignkey", "datamodel"}},
	{key: "datamodel-08", content: "Client-generated IDs are rejected; the server always mints the UUID on insert.", tags: []string{"id", "uuid", "validation"}},
	{key: "datamodel-09", content: "The UUID format was chosen partly so IDs never leak the total row count to a caller.", tags: []string{"id", "uuid", "privacy"}},
	{key: "datamodel-10", content: "Legacy integer IDs from the old system are kept in a separate legacy_id column for lookups.", tags: []string{"id", "legacy", "datamodel"}},

	// Cluster: payload schema versioning (target datamodel-11).
	{key: "datamodel-11", content: "Every catalog item payload carries a schema_version field so old and new shapes can coexist during a migration.", tags: []string{"version", "payload", "datamodel"}},
	{key: "datamodel-12", content: "A reader that sees an unknown schema_version falls back to treating unrecognized fields as opaque.", tags: []string{"version", "payload", "compat"}},
	{key: "datamodel-13", content: "Bumping schema_version is additive-only; no field is ever repurposed to a new meaning.", tags: []string{"version", "schema", "datamodel"}},
	{key: "datamodel-14", content: "Old payload versions are never rewritten in place; a migration job stamps a new version going forward.", tags: []string{"version", "migration", "datamodel"}},
	{key: "datamodel-15", content: "The schema_version field defaults to zero for rows written before the field existed.", tags: []string{"version", "default", "datamodel"}},
	{key: "datamodel-16", content: "A payload version mismatch between reader and writer is logged but does not fail the request.", tags: []string{"version", "payload", "datamodel"}},

	// --- config domain: the web app's configuration conventions. ---

	// Cluster: env var prefix convention (target config-01).
	{key: "config-01", content: "All environment variables for the web app's config are prefixed with WEBAPP_ so they never collide with unrelated tools.", tags: []string{"prefix", "envvar", "config"}},
	{key: "config-02", content: "A config key without the WEBAPP_ prefix is silently ignored by the loader, never an error.", tags: []string{"prefix", "loader", "config"}},
	{key: "config-03", content: "The prefix convention was chosen so a shared host could run several services without variable collisions.", tags: []string{"prefix", "config", "convention"}},
	{key: "config-04", content: "Legacy unprefixed variables are supported for one release behind a deprecation warning, then removed.", tags: []string{"prefix", "legacy", "config"}},
	{key: "config-05", content: "Prefix matching is case-sensitive; a lowercase webapp_ variable is not recognized.", tags: []string{"prefix", "casing", "config"}},

	// Cluster: config precedence order (target config-06).
	{key: "config-06", content: "Config precedence for the web app is flag, then environment variable, then file, then built-in default, in that order.", tags: []string{"precedence", "config", "loader"}},
	{key: "config-07", content: "A flag always wins over an environment variable, even if the environment variable was set more recently.", tags: []string{"precedence", "flag", "config"}},
	{key: "config-08", content: "An empty environment variable value is treated as unset and falls through to the next precedence layer.", tags: []string{"precedence", "envvar", "config"}},
	{key: "config-09", content: "The config file layer is optional; its absence is not an error, just an empty layer in precedence.", tags: []string{"precedence", "file", "config"}},
	{key: "config-10", content: "Precedence order is documented once in the loader's own doc comment, not duplicated elsewhere.", tags: []string{"precedence", "config", "docs"}},
	{key: "config-11", content: "A debug flag prints which precedence layer supplied each config value, for troubleshooting.", tags: []string{"precedence", "debug", "config"}},

	// Cluster: secrets read from files (target config-12).
	{key: "config-12", content: "Secrets for the web app are read from a mounted file path, never passed as a plain environment variable value.", tags: []string{"secret", "file", "config"}},
	{key: "config-13", content: "A secret-from-file value can itself be a path pointing at another file, resolved one level deep.", tags: []string{"secret", "file", "resolution"}},
	{key: "config-14", content: "Rotating a mounted secret file does not require a restart; the loader re-reads it on each access.", tags: []string{"secret", "file", "rotation"}},
	{key: "config-15", content: "An unreadable secret file fails startup loudly rather than falling back to an empty value.", tags: []string{"secret", "file", "config"}},
	{key: "config-16", content: "Secret file paths are themselves configured through a normal environment variable, just not the secret value.", tags: []string{"secret", "file", "envvar"}},

	// --- testing domain: the search API's test-suite conventions. ---

	// Cluster: container-backed integration tests (target testing-01).
	{key: "testing-01", content: "Integration tests for the search API spin up a real Postgres container instead of mocking the database layer.", tags: []string{"integration", "container", "testing"}},
	{key: "testing-02", content: "The test container is reused across the whole package's tests to keep the suite fast.", tags: []string{"container", "testing", "performance"}},
	{key: "testing-03", content: "A missing Docker daemon causes integration tests to skip cleanly instead of failing the whole suite.", tags: []string{"container", "skip", "testing"}},
	{key: "testing-04", content: "Each integration test gets its own uniquely named schema inside the shared test container.", tags: []string{"integration", "container", "testing"}},
	{key: "testing-05", content: "CI runs the container-backed suite in a separate job from the fast unit-test job.", tags: []string{"container", "ci", "testing"}},

	// Cluster: flaky test quarantine (target testing-06).
	{key: "testing-06", content: "A flaky test in the search API's suite is quarantined behind a skip tag rather than deleted outright.", tags: []string{"flaky", "quarantine", "testing"}},
	{key: "testing-07", content: "Quarantined tests still run nightly in a separate job so a real regression isn't silently hidden forever.", tags: []string{"flaky", "quarantine", "ci"}},
	{key: "testing-08", content: "A test is only quarantined after failing intermittently at least three times with no code change.", tags: []string{"flaky", "policy", "testing"}},
	{key: "testing-09", content: "Removing a test from quarantine requires the original flaky failure to be root-caused and fixed.", tags: []string{"flaky", "quarantine", "testing"}},
	{key: "testing-10", content: "The quarantine list is reviewed monthly so it never grows without anyone noticing.", tags: []string{"flaky", "review", "testing"}},

	// Cluster: golden-file comparison (target testing-11).
	{key: "testing-11", content: "Golden-file tests for the search API compare generated output byte-for-byte against a checked-in fixture file.", tags: []string{"golden", "fixture", "testing"}},
	{key: "testing-12", content: "Golden files are regenerated with an update flag rather than hand-edited when the format intentionally changes.", tags: []string{"golden", "fixture", "workflow"}},
	{key: "testing-13", content: "A golden-file diff in review is treated as a signal to double check the change was intentional.", tags: []string{"golden", "review", "testing"}},
	{key: "testing-14", content: "Binary golden files are avoided in favor of a human-readable text format wherever possible.", tags: []string{"golden", "fixture", "format"}},
	{key: "testing-15", content: "Golden files live next to the test that consumes them, not in a shared fixtures directory.", tags: []string{"golden", "fixture", "testing"}},
	{key: "testing-16", content: "A stale golden file that no test references anymore is deleted as part of the same change.", tags: []string{"golden", "cleanup", "testing"}},
}

// paraphraseTopics lists the blind-author-facing topics: one per answer
// target across all six domains, plus 4 no-answer topics (D-12, wantKey "")
// whose subjects exist nowhere in the corpus. Every label starts with "the
// record about " and is at most 12 words (D-01), sharing at most 2 content
// words with its target (for answer topics) or with any seed at all (for
// no-answer topics) so a reader of the label alone cannot reproduce a
// record's wording. IDs are interleaved by domain so a topic's position
// reveals nothing about whether it has an answer.
var paraphraseTopics = []paraphraseTopic{
	{id: "T01", label: "the record about what happens to code before it lands", wantKey: "tooling-01"},
	{id: "T02", label: "the record about what order the style tools run in", wantKey: "tooling-05"},
	{id: "T03", label: "the record about where the style checker settings live", wantKey: "tooling-09"},
	{id: "T04", label: "the record about which pipeline step happens first", wantKey: "tooling-13"},
	{id: "T05", label: "the record about how long a login stays authorized", wantKey: "auth-01"},
	{id: "T06", label: "the record about how automated callers prove who they are", wantKey: "auth-05"},
	{id: "T07", label: "the record about verifying a token belongs to us", wantKey: "auth-09"},
	{id: "T08", label: "the record about how the web console keeps you signed in", wantKey: "auth-13"},
	{id: "T09", label: "the record about how new versions get let out slowly", wantKey: "deploy-01"},
	{id: "T10", label: "the record about undoing a bad release", wantKey: "deploy-07"},
	{id: "T11", label: "the record about what runs before the app comes up", wantKey: "deploy-12"},
	{id: "T12", label: "the record about what a delete actually does to the row", wantKey: "datamodel-01"},
	{id: "T13", label: "the record about what shape a thing's unique identifier takes", wantKey: "datamodel-06"},
	{id: "T14", label: "the record about how old and new formats coexist", wantKey: "datamodel-11"},
	{id: "T15", label: "the record about which prefix every setting name uses", wantKey: "config-01"},
	{id: "T16", label: "the record about which config source wins when several are set", wantKey: "config-06"},
	{id: "T17", label: "the record about where sensitive values are loaded from", wantKey: "config-12"},
	{id: "T18", label: "the record about what backs the database during a test run", wantKey: "testing-01"},
	{id: "T19", label: "the record about how an unreliable test gets set aside", wantKey: "testing-06"},
	{id: "T20", label: "the record about how a saved reference catches unintended changes", wantKey: "testing-11"},
	{id: "T21", label: "the record about the on-call rotation schedule for the week", wantKey: ""},
	{id: "T22", label: "the record about which GPU driver revision is pinned for training", wantKey: ""},
	{id: "T23", label: "the record about the retention period required for a legal hold", wantKey: ""},
	{id: "T24", label: "the record about the push notification quota for the mobile app", wantKey: ""},
}

// paraphraseCase wraps paraphraseSeeds with the 24 blind queries recorded in
// .planning/phases/01-eval-foundation-lexical-reranker-fix/01-BLIND-QUERIES.md
// (D-01 procedure, RANK-01):
//
//   - The corpus (paraphraseSeeds) and its 24 topic labels (paraphraseTopics)
//     were authored in plan 01-03, independently of any query text.
//   - The blind query author saw ONLY .planning/phases/
//     01-eval-foundation-lexical-reranker-fix/01-BLIND-QUERY-PROMPT.md's
//     labels-only prompt (the text below its "---8<---" marker, pasted
//     verbatim) — never paraphraseSeeds, never a record's wording. The
//     prompt's content is pinned at commit 2606c358.
//   - Author: a fresh general-purpose subagent dispatched by the
//     orchestrator, with no repository context and no tool use (0 tool
//     calls). Date: 2026-09-22.
//   - Every query text below is transcribed VERBATIM from
//     01-BLIND-QUERIES.md's `- Tnn: <query>` reply — no wording was edited
//     after the blind pass, in either direction.
//   - The mapping from each query to its wantKey target is a SEPARATE,
//     non-blind pass (this file, plan 01-04): matching by topic ID against
//     paraphraseTopics, never by re-reading the blind author's intent. A
//     no-answer topic (T21-T24, D-12) carries an empty wantKey, unchanged
//     from paraphraseTopics.
//
// This case is written independently of gh261Case's target (RANK-01): its
// corpus, its labels, and its queries share no authorship or visibility
// with Record T or gh261Distractors. Its no-answer queries serve Phase 4's
// per-hit relevance signal (D-12) — this plan excludes them from
// recall@k/MRR and logs them only.
var paraphraseCase = retrievalCase{
	name:        "paraphrase-blind-multidomain",
	role:        roleParaphrase,
	seedRecords: paraphraseSeeds,
	queries: []retrievalQuery{
		{name: "T01", text: "what did we decide has to happen to code before it gets merged into main", wantKey: "tooling-01"},
		{name: "T02", text: "in what order do the formatter and linter run, which one goes first", wantKey: "tooling-05"},
		{name: "T03", text: "where did we put the config file for the linter settings", wantKey: "tooling-09"},
		{name: "T04", text: "which step runs first in the CI pipeline before everything else", wantKey: "tooling-13"},
		{name: "T05", text: "how long does a login session or access token stay valid before expiring", wantKey: "auth-01"},
		{name: "T06", text: "how do service accounts and bots authenticate to our API", wantKey: "auth-05"},
		{name: "T07", text: "how do we check that an incoming token was actually issued for our service", wantKey: "auth-09"},
		{name: "T08", text: "how does the admin dashboard keep users logged in between visits", wantKey: "auth-13"},
		{name: "T09", text: "how do we roll out new versions gradually to a small slice of users first", wantKey: "deploy-01"},
		{name: "T10", text: "what is the process for rolling back when a release goes bad", wantKey: "deploy-07"},
		{name: "T11", text: "what has to run at startup before the service starts accepting traffic", wantKey: "deploy-12"},
		{name: "T12", text: "when something is deleted do we actually remove the row or just flag it", wantKey: "datamodel-01"},
		{name: "T13", text: "what format do we use for record IDs, UUIDs or something shorter", wantKey: "datamodel-06"},
		{name: "T14", text: "how do we handle old and new data formats living side by side during a migration", wantKey: "datamodel-11"},
		{name: "T15", text: "what prefix do all our environment variable names start with", wantKey: "config-01"},
		{name: "T16", text: "when a setting comes from a flag, env var and config file, which one takes priority", wantKey: "config-06"},
		{name: "T17", text: "where do secrets and credentials get loaded from at runtime", wantKey: "config-12"},
		{name: "T18", text: "what database do the tests run against, a real one or an in-memory fake", wantKey: "testing-01"},
		{name: "T19", text: "what do we do with a flaky test, skip it or quarantine it somewhere", wantKey: "testing-06"},
		{name: "T20", text: "how do snapshot or golden file tests catch output changes we did not intend", wantKey: "testing-11"},
		{name: "T21", text: "who is on call this week and how does the rotation work", wantKey: ""},
		{name: "T22", text: "which GPU driver version did we pin for the training machines", wantKey: ""},
		{name: "T23", text: "how long do we have to keep data under a legal hold", wantKey: ""},
		{name: "T24", text: "what is the push notification limit for the mobile app", wantKey: ""},
	},
}
