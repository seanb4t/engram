// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package retrievaleval

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/config"
	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)

// defaultK mirrors the k a real MCP client experiences when it omits the arg.
// The default is applied by the search_memory tool closure in server.Register,
// NOT by deps.searchMemory — the core deliberately applies no internal k
// default, so each adapter supplies its own (MCP: 8, Connect: 20).
const defaultK = 8

// testCollectionPrefix namespaces this package's integration-test Qdrant
// collection names so a single shared Qdrant instance (CI's
// ENGRAM_QDRANT_TEST_ADDR path) can host every Qdrant-backed package's test
// suite concurrently without cross-package name collisions (CONTEXT.md D-16).
// This package already generated collision-safe per-UUID names before this
// constant existed (newTestcontainerStore's "retrievaleval_" + uuid); the
// constant makes that namespace the single source of truth rather than a
// comment about a literal elsewhere in the line.
const testCollectionPrefix = "retrievaleval_"

// testCollection returns name namespaced into this package's collection space
// on the shared test Qdrant instance.
func testCollection(name string) string {
	return testCollectionPrefix + name
}

// newTestStore is the prefix-enforcing construction seam for every test Store
// built against a real Qdrant instance in this package. It asserts name
// carries this package's testCollectionPrefix before constructing the store,
// so a collection name that skips testCollection() fails the test naming the
// offending value — a runtime assertion a source-level check could be routed
// around, but a t.Fatalf inside the one function every test store is built by
// cannot (CONTEXT.md D-16, plan 01-05).
func newTestStore(t testing.TB, c *qdrant.Client, name string) *store.Store {
	t.Helper()
	if !strings.HasPrefix(name, testCollectionPrefix) {
		t.Fatalf("collection name %q does not carry this package's prefix %q: route it through testCollection()", name, testCollectionPrefix)
	}
	return store.New(c, name)
}

// requireEvalEnabled skips t unless the resolved koanf gate (D-15) is
// enabled, and fails t if the gate itself is malformed — mirroring every
// other gated test in this package, now sourced from resolveEvalGate's
// package-local koanf load instead of the retired raw process-environment
// read.
func requireEvalEnabled(t *testing.T) {
	t.Helper()
	enabled, err := retrievalEvalEnabled()
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !enabled {
		t.Skip("set ENGRAM_RETRIEVAL_EVAL=1 (and the gateway/model env) to run the retrieval eval")
	}
}

// symmetricEmbedConfig reports whether e carries no query/document asymmetry:
// all four instruction/params fields are empty. A symmetric embedder config
// legitimately yields query == document (review B3), and the decision is
// made from the resolved config the embedder was built from, never an
// independent read (D-14, #354).
func symmetricEmbedConfig(e config.EmbedConfig) bool {
	return e.QueryInstruction == "" &&
		e.DocumentInstruction == "" &&
		e.QueryParams == "" &&
		e.DocumentParams == ""
}

// TestSymmetricEmbedConfig proves symmetricEmbedConfig decides purely from
// the four embed instruction/params fields on a resolved *config.Config,
// covering the row combinations the differ gate's skip depends on (D-14).
func TestSymmetricEmbedConfig(t *testing.T) {
	cases := []struct {
		name                string
		queryInstruction    string
		documentInstruction string
		queryParams         string
		documentParams      string
		want                bool
	}{
		{"all empty -> symmetric", "", "", "", "", true},
		{"query instruction only -> asymmetric", "prefix: ", "", "", "", false},
		{"document instruction only -> asymmetric", "", "prefix: ", "", "", false},
		{"query params only -> asymmetric", "", "", `{"input_type":"search_query"}`, "", false},
		{"document params only -> asymmetric", "", "", "", `{"input_type":"search_document"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ENGRAM_EMBED_QUERY_INSTRUCTION", tc.queryInstruction)
			t.Setenv("ENGRAM_EMBED_DOCUMENT_INSTRUCTION", tc.documentInstruction)
			t.Setenv("ENGRAM_EMBED_QUERY_PARAMS", tc.queryParams)
			t.Setenv("ENGRAM_EMBED_DOCUMENT_PARAMS", tc.documentParams)

			loadedCfg, err := config.Load(nil)
			if err != nil {
				t.Fatalf("config.Load: %v", err)
			}
			if got := symmetricEmbedConfig(loadedCfg.Embed); got != tc.want {
				t.Errorf("symmetricEmbedConfig(%+v) = %v, want %v", loadedCfg.Embed, got, tc.want)
			}
		})
	}
}

// recordLabel resolves a Qdrant point id back to its fixture-local key for
// human-readable logging (idByKey's inverse), falling back to the raw id
// when no seedRecord.key maps to it (should not happen for hits inside a
// case's own seeded collection, but logging must never panic on a lookup
// miss).
func recordLabel(keyByID map[string]string, id string) string {
	if key, ok := keyByID[id]; ok {
		return key
	}
	return id
}

// TestRetrievalEval is the retrieval-quality eval: it seeds the labeled
// dataset in fixtures.go through the exact production doc-embed sequence,
// searches it through the production query path, and measures every
// pluggable named ranker (D-11, rankers.go) over the exact bounded-over-fetch
// candidate pool store.SearchReranked would rank, alongside the shipped
// ranking itself. It aggregates recall@k/MRR per (ranker, case-role)
// pair (D-09), logs a per-variant Markdown table (formatVariantTable), and
// applies D-10's two hard gates: gh261Case's shipped target at rank 1 for
// both its queries, and (from plan 01-04 Task 2) shipped paraphrase MRR at
// least vector-only's. No-answer queries (retrievalQuery.wantKey == "") are
// excluded from every aggregate and only logged (D-12). The gate mirrors
// internal/summarize/fidelity_test.go's TestSummaryFidelity —
// defense-in-depth retained even though TestMain already short-circuits
// before Docker.
func TestRetrievalEval(t *testing.T) {
	requireEvalEnabled(t)
	if storetest.Addr() == "" {
		t.Skip("no Qdrant available: set ENGRAM_QDRANT_TEST_ADDR or start Docker (testcontainers)")
	}

	ctx := context.Background()

	// Build the embedder with FULL prod parity (all four embed options wired by
	// embedderFromConfig) by reusing the exported prod builder. Its own store is
	// discarded on purpose: that store dials cfg.Qdrant.Addr (defaulting to
	// ambient ENGRAM_QDRANT_ADDR), which is NEVER where this eval seeds/searches
	// (round-2 finding 1) — the eval store below is built directly from
	// storetest's resolved test Qdrant address instead.
	_, dim, em, _, _, err := server.StoreAndEmbedderFromEnvNoEnsure()
	if err != nil {
		t.Fatalf("build prod-parity embedder: %v", err)
	}

	subj := store.Authenticated("retrieval-eval@engram.dev")
	roster := evalRankers()

	// Per-(ranker, role) aggregates, plus the shipped row's own aggregates —
	// keyed by roster entry name, populated only from ANSWER queries
	// (wantKey != ""). The shipped row is measured through
	// st.SearchReranked itself, never through a local rank function.
	guardMetrics := make(map[string]variantMetrics, len(roster))
	paraphraseMetrics := make(map[string]variantMetrics, len(roster))
	var shippedGuard, shippedParaphrase variantMetrics

	// variantMatchesShipped tracks, per enabled roster entry, whether its
	// id list equaled the shipped id list on EVERY answer query seen so
	// far — the "shipped (SearchReranked) matches" diagnostic (T-01-10).
	variantMatchesShipped := make(map[string]bool, len(roster))
	for _, r := range roster {
		if r.rank != nil {
			variantMatchesShipped[r.name] = true
		}
	}

	for _, tc := range retrievalCases {
		t.Run(tc.name, func(t *testing.T) {
			// A fresh, uniquely-named collection per case avoids cross-case /
			// cross-run contamination without needing cleanup.
			st := newTestcontainerStore(t, dim)
			scope := "retrieval-eval:project:" + tc.name

			idByKey := make(map[string]string, len(tc.seedRecords))
			keyByID := make(map[string]string, len(tc.seedRecords))
			for _, rec := range tc.seedRecords {
				m := store.Memory{
					ID:        uuid.NewString(),
					Content:   rec.content,
					Scope:     scope,
					Source:    "user-said",
					Category:  "convention",
					Tags:      rec.tags,
					Actor:     subj.Owner(),
					Owner:     subj.Owner(),
					CreatedAt: time.Now().UTC(),
				}
				idByKey[rec.key] = m.ID
				keyByID[m.ID] = rec.key

				// Exact prod doc-embed sequence (round-2 finding 2):
				// store.EmbedText folds tags in, then Embed, then Upsert takes
				// the precomputed vector — never a raw/bespoke shortcut.
				vec, err := em.Embed(ctx, store.EmbedText(m.Content, m.Tags))
				if err != nil {
					t.Fatalf("harness: seed %s: embed: %v", rec.key, err)
				}
				if err := st.Upsert(ctx, m, vec); err != nil {
					t.Fatalf("harness: seed %s: upsert: %v", rec.key, err)
				}
			}
			ceilingK := uint64(len(tc.seedRecords)) + 5

			for _, q := range tc.queries {
				// (a) Embed the query through the production query-side path.
				vec, err := em.EmbedQuery(ctx, q.text)
				if err != nil {
					t.Fatalf("harness: %s/%s: embed query: %v", tc.name, q.name, err)
				}

				// (b) Shipped: the real seam — SAME shared helper
				// deps.searchMemory (MCP) and engramAPI.SearchMemories
				// (Connect) call, so this measures exactly what ships and
				// cannot drift from a raw Store.Search shortcut.
				shipped, err := st.SearchReranked(ctx, scope, subj, q.text, vec, defaultK, store.SearchOptions{})
				if err != nil {
					t.Fatalf("harness: %s/%s: search: %v", tc.name, q.name, err)
				}
				if len(shipped) == 0 {
					t.Errorf("harness: %s/%s: search returned no results", tc.name, q.name)
				}
				for i, m := range shipped {
					if m.Score == 0 {
						t.Errorf("harness: %s/%s: result %d (%s) has zero score", tc.name, q.name, i, m.ID)
					}
				}
				shippedIDs := make([]string, len(shipped))
				for i, m := range shipped {
					shippedIDs[i] = m.ID
				}

				// (c) Pool: the exact candidate set SearchReranked itself
				// would rank, so every non-shipped variant below ranks over
				// an apples-to-apples candidate set.
				pool, err := st.Search(ctx, scope, subj, vec, store.CandidateK(defaultK), store.SearchOptions{Full: true})
				if err != nil {
					t.Fatalf("harness: %s/%s: candidate pool search: %v", tc.name, q.name, err)
				}

				// (d) Rank the pool with every enabled roster entry.
				variantRanked := make(map[string][]store.Memory, len(roster))
				variantIDs := make(map[string][]string, len(roster))
				for _, r := range roster {
					if r.rank == nil {
						continue // disabled (Jev stub): no ranking, no gating.
					}
					ranked := r.rank(q.text, pool, defaultK)
					ids := make([]string, len(ranked))
					for i, m := range ranked {
						ids[i] = m.ID
					}
					variantRanked[r.name] = ranked
					variantIDs[r.name] = ids
				}

				// (e) No-answer query (D-12): logged only, never gated —
				// excluded from every recall@k/MRR aggregate above.
				if q.wantKey == "" {
					shippedTop, shippedScore := "none", float32(0)
					if len(shipped) > 0 {
						shippedTop, shippedScore = recordLabel(keyByID, shipped[0].ID), shipped[0].Score
					}
					t.Logf("no-answer %s/%s: %s top=%s score=%f", tc.name, q.name, shippedRowName, shippedTop, shippedScore)
					for _, r := range roster {
						if r.rank == nil {
							continue
						}
						ranked := variantRanked[r.name]
						top, score := "none", float32(0)
						if len(ranked) > 0 {
							top, score = recordLabel(keyByID, ranked[0].ID), ranked[0].Score
						}
						t.Logf("no-answer %s/%s: %s top=%s score=%f", tc.name, q.name, r.name, top, score)
					}
					continue
				}

				// (f) Answer query: resolve wantID, aggregate every
				// variant's metrics, and update the "matches shipped"
				// diagnostic.
				wantID, ok := idByKey[q.wantKey]
				if !ok {
					t.Fatalf("harness: fixture bug: wantKey %q not among seeded records for %s/%s", q.wantKey, tc.name, q.name)
				}

				shippedTarget := &shippedGuard
				variantTarget := guardMetrics
				if tc.role == roleParaphrase {
					shippedTarget = &shippedParaphrase
					variantTarget = paraphraseMetrics
				}
				shippedTarget.add(shippedIDs, wantID)
				for _, r := range roster {
					if r.rank == nil {
						continue
					}
					m := variantTarget[r.name]
					m.add(variantIDs[r.name], wantID)
					variantTarget[r.name] = m

					if !slices.Equal(variantIDs[r.name], shippedIDs) {
						variantMatchesShipped[r.name] = false
					}
				}

				// (g) Prove the harness itself can find the target with a
				// generous ceiling — this is NOT a ranking-quality bar; it
				// only proves the fixture/harness are sound, independent of
				// ranking quality, so it deliberately uses raw Store.Search
				// rather than any reranked/local-ranked path.
				ceiling, err := st.Search(ctx, scope, subj, vec, ceilingK, store.SearchOptions{})
				if err != nil {
					t.Fatalf("harness: %s/%s: ceiling search: %v", tc.name, q.name, err)
				}
				ceilingIDs := make([]string, len(ceiling))
				for i, m := range ceiling {
					ceilingIDs[i] = m.ID
				}
				if !recallAtK(ceilingIDs, wantID) {
					t.Errorf("harness: %s/%s: target not found even at ceiling k=%d — harness/fixture bug, not a ranking result", tc.name, q.name, ceilingK)
				}

				// (h) D-10 gate 1: roleRegressionGuard's shipped target MUST
				// be at rank 1, not merely "within default k".
				if tc.role == roleRegressionGuard {
					rank := 0
					for i, id := range shippedIDs {
						if id == wantID {
							rank = i + 1
							break
						}
					}
					if rank != 1 {
						t.Errorf("D-10 gate FAILED: #261 target at rank %d (want 1) for %s under the shipped ranking", rank, q.name)
					}
				}

				// (i) Raw-score gap: diagnostic only, never a hard gate — a
				// lexical/gated reranker can promote a hit's rank without
				// necessarily raising its raw Qdrant score above every
				// sticky neighbor (Score is raw first-stage dense
				// similarity, unchanged by reranking).
				var wantScore, bestOtherScore float32
				for _, m := range shipped {
					if m.ID == wantID {
						wantScore = m.Score
					} else if m.Score > bestOtherScore {
						bestOtherScore = m.Score
					}
				}
				t.Logf("%s/%s: score(target)=%f best-distractor-score=%f gap=%f (diagnostic only, never a hard gate)",
					tc.name, q.name, wantScore, bestOtherScore, wantScore-bestOtherScore)
			}
		})
	}

	rows := buildSummaries(roster, guardMetrics, paraphraseMetrics, shippedGuard, shippedParaphrase)
	t.Logf("\n%s", formatVariantTable(rows, rankingDecision{}))

	if shippedGuard.allRank1() {
		t.Logf("D-10 gate PASS: #261 target at rank 1 for both queries under the shipped ranking")
	}

	var matchNames []string
	for _, r := range roster {
		if r.rank == nil {
			continue
		}
		if variantMatchesShipped[r.name] {
			matchNames = append(matchNames, r.name)
		}
	}
	matches := "none"
	if len(matchNames) > 0 {
		matches = strings.Join(matchNames, ", ")
	}
	t.Logf("shipped (SearchReranked) matches: %s", matches)
}

// TestRetrievalEval_AsymmetryDiffer is the Pitfall-12 correctness gate
// (REQ-embed-gemini-direct): it embeds differProbe through BOTH the
// query-side (em.EmbedQuery) and document-side (em.Embed) production paths
// and asserts the two vectors differ. The "TestRetrievalEval" prefix is
// load-bearing — task eval:retrieval's `go test -run TestRetrievalEval`
// substring-matches this name too, so the documented eval command genuinely
// reaches the differ assertion without any Taskfile change.
//
// It never touches Qdrant (no storetest.Addr / seedRecord involved), but it
// still gates on ENGRAM_RETRIEVAL_EVAL as its first statement (defense in
// depth mirroring TestRetrievalEval, even though TestMain already
// short-circuits before Docker startup when the gate is unset). NOTE
// (accepted, not fixed — review B9): TestMain always starts the Qdrant
// testcontainer when ENGRAM_RETRIEVAL_EVAL=1, even for a -run selection that
// only exercises this Qdrant-free test, because TestMain cannot cheaply
// inspect the active -run selection. A live run of this test therefore still
// requires Docker (or ENGRAM_QDRANT_TEST_ADDR) as a package-level
// prerequisite; documented here and in plan 14-03.
func TestRetrievalEval_AsymmetryDiffer(t *testing.T) {
	requireEvalEnabled(t)

	ctx := context.Background()

	// Prod-parity embedder — the SAME builder/path TestRetrievalEval uses
	// (D-03: never a bespoke embed shortcut). Its own store is discarded; this
	// test never touches Qdrant. dim is KEPT (not discarded) to assert the
	// vectors are correctly sized, not just non-empty (review B4). Building
	// happens BEFORE the symmetric-config skip below (D-14): the eval is
	// enabled here, so a config/embedder build failure is a real failure, not
	// something to skip past.
	_, dim, em, _, cfg, err := server.StoreAndEmbedderFromEnvNoEnsure()
	if err != nil {
		t.Fatalf("build prod-parity embedder: %v", err)
	}

	// Symmetric-config guard (review B3, D-14): a symmetric embedder config
	// (all four instruction/params fields empty on the SAME resolved config
	// the embedder above was built from — never an independent
	// process-environment read, #354) legitimately produces query==document
	// — e.g. OpenAI text-embedding-3-small, bare bge-m3. The inequality
	// assertion below does not apply to those configs, so skip it rather
	// than fail the suite for a valid symmetric setup.
	if symmetricEmbedConfig(cfg.Embed) {
		t.Skip("symmetric embedder config (no QUERY/DOCUMENT instruction or params set): query == document is valid here, asymmetry assertion does not apply")
	}

	queryVec, err := em.EmbedQuery(ctx, differProbe)
	if err != nil {
		t.Fatalf("embed query-side: %v", err)
	}
	documentVec, err := em.Embed(ctx, differProbe)
	if err != nil {
		t.Fatalf("embed document-side: %v", err)
	}

	// Dimension contract (review B4): reject an empty or wrong-sized vector
	// before comparing — embed.go only checks a `data` entry exists, so a
	// naive inequality check alone could pass on two malformed vectors.
	if len(queryVec) == 0 || len(documentVec) == 0 ||
		len(queryVec) != int(dim) || len(documentVec) != int(dim) {
		t.Fatalf("asymmetry differ: vector dimension contract violated: query=%d document=%d want=%d (empty or wrong-sized vector)",
			len(queryVec), len(documentVec), dim)
	}

	// THE Pitfall-12 correctness gate (D-04/D-13, hard t.Fatal — not
	// t.Errorf — so this stops immediately and cannot be obscured by later
	// output, review B4): the query-side and document-side vectors of the
	// SAME string MUST differ MATERIALLY, by cosine distance above
	// differMinCosineDistance — replacing the retired bit-identity
	// comparison (#353), which a hosted embedder's harmless float jitter
	// between two calls could trip even with no real asymmetric effect.
	distance, err := cosineDistance(queryVec, documentVec)
	if err != nil {
		t.Fatalf("asymmetry differ: malformed embedding vector (dim=%d): %v", dim, err)
	}
	// Written so a NaN distance can never satisfy the pass condition.
	if !(distance > differMinCosineDistance) {
		t.Fatalf("asymmetry differ FAIL: cosine distance %.6g is not above %g — query and document vectors are materially the same (dim=%d) — the asymmetric instruction-prefix had no effect; the operator likely wired the no-op ENGRAM_EMBED_QUERY_PARAMS/ENGRAM_EMBED_DOCUMENT_PARAMS/task_type mechanism instead of ENGRAM_EMBED_QUERY_INSTRUCTION/ENGRAM_EMBED_DOCUMENT_INSTRUCTION", distance, differMinCosineDistance, dim)
	}

	t.Logf("asymmetry differ PASS: vectors differ materially (cosine distance=%.6g, dim=%d)", distance, dim)
}

// newTestcontainerStore builds a *store.Store pinned to storetest's resolved
// test Qdrant address — NEVER the ambient ENGRAM_QDRANT_ADDR a developer's
// prod-like env might set (round-2 finding 1) — in a fresh, uniquely-named
// collection ensured at dim, routed through newTestStore so the collection
// name is prefix-asserted at runtime like every other Qdrant-backed
// package's test stores (plan 01-05).
func newTestcontainerStore(t testing.TB, dim uint64) *store.Store {
	t.Helper()
	qc := storetest.Dial(t, storetest.RecvLimit)
	st := newTestStore(t, qc, testCollection(uuid.NewString()))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := st.EnsureCollection(ctx, dim); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	return st
}

// TestMain gates the whole package on the resolved ENGRAM_RETRIEVAL_EVAL
// koanf gate (D-15) as its FIRST statement, before any testcontainer/Docker
// startup (review finding 1): when the gate is off, the required `test`
// job's `go test ./...` pays zero ADDITIONAL Docker/Qdrant cost from this
// package (round-2 finding 8). A malformed gate value fails loudly instead
// of silently reading as off. Once the gate is enabled, TestMain delegates
// the rest of the Qdrant lifecycle to storetest.Run with
// IgnoreRequireQdrant: this package never consulted ENGRAM_REQUIRE_QDRANT
// before this phase and must not gain that fail-closed behavior now — a
// missing Qdrant with the eval gate set still only skips (RESEARCH.md
// Pitfall 6, this plan's recorded decision).
func TestMain(m *testing.M) {
	enabled, err := retrievalEvalEnabled()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
	if !enabled {
		os.Exit(m.Run())
	}
	os.Exit(storetest.Run(m, storetest.IgnoreRequireQdrant()))
}

// TestSharedQdrantAddressHonored proves this package took the CI shared-Qdrant
// fast path rather than booting its own testcontainer, whenever
// ENGRAM_QDRANT_TEST_ADDR is set. Gates on ENGRAM_RETRIEVAL_EVAL first,
// mirroring every other test in this package (TestMain never touches Qdrant
// at all unless that gate is "1" — see TestMain's first statement) — this is
// not the shared-Qdrant skip, it is the package's existing opt-in-eval skip,
// and conflating the two would make this test fail whenever someone runs the
// CI `test` job without ENGRAM_RETRIEVAL_EVAL=1, which is the normal case.
// Delegates to storetest for the shared-address assertion itself, which
// skips (does not fail) when the shared-address env var is unset.
func TestSharedQdrantAddressHonored(t *testing.T) {
	requireEvalEnabled(t)
	storetest.AssertSharedAddressHonored(t)
}
