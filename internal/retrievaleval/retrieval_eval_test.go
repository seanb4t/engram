// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package retrievaleval

import (
	"context"
	"fmt"
	"net"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/store"
	tcqdrant "github.com/testcontainers/testcontainers-go/modules/qdrant"
)

// defaultK mirrors the k a real MCP client experiences when it omits the arg.
// The default is applied by the search_memory tool closure in server.Register,
// NOT by deps.searchMemory — the core deliberately applies no internal k
// default, so each adapter supplies its own (MCP: 8, Connect: 20).
const defaultK = 8

// qdrantImageTag mirrors internal/store/store_test.go's pinned Qdrant image so
// the eval measures against the same server version the store integration
// suite is verified against.
const qdrantImageTag = "qdrant/qdrant:v1.18.2"

// testQdrantAddr is the gRPC host:port the eval seeds and searches against. Set
// by TestMain: ENGRAM_QDRANT_TEST_ADDR if provided (fast-path override), else
// an ephemeral testcontainer. The eval store is ALWAYS built from this
// address — never the ambient ENGRAM_QDRANT_ADDR a developer's prod-like env
// might set (round-2 finding 1) — so a configured production/dev Qdrant is
// never read from or written to by this package.
var testQdrantAddr string

// TestRetrievalEval is the retrieval-quality eval: it seeds the labeled dataset
// in fixtures.go through the exact production doc-embed sequence, searches it
// through the production query path, and reports recall@k / MRR plus the
// GitHub #261 baseline. The gate mirrors
// internal/summarize/fidelity_test.go's TestSummaryFidelity — defense-in-depth
// retained even though TestMain already short-circuits before Docker.
func TestRetrievalEval(t *testing.T) {
	if os.Getenv("ENGRAM_RETRIEVAL_EVAL") != "1" {
		t.Skip("set ENGRAM_RETRIEVAL_EVAL=1 (and the gateway/model env) to run the retrieval eval")
	}
	if testQdrantAddr == "" {
		t.Skip("no Qdrant available: set ENGRAM_QDRANT_TEST_ADDR or start Docker (testcontainers)")
	}

	ctx := context.Background()

	// Build the embedder with FULL prod parity (all four embed options wired by
	// embedderFromConfig) by reusing the exported prod builder. Its own store is
	// discarded on purpose: that store dials cfg.Qdrant.Addr (defaulting to
	// ambient ENGRAM_QDRANT_ADDR), which is NEVER where this eval seeds/searches
	// (round-2 finding 1) — the eval store below is built directly from
	// testQdrantAddr instead.
	_, dim, em, _, err := server.StoreAndEmbedderFromEnvNoEnsure()
	if err != nil {
		t.Fatalf("build prod-parity embedder: %v", err)
	}

	subj := store.Authenticated("retrieval-eval@engram.dev")

	for _, tc := range retrievalCases {
		t.Run(tc.name, func(t *testing.T) {
			// A fresh, uniquely-named collection per case avoids cross-case /
			// cross-run contamination without needing cleanup.
			st, err := newTestcontainerStore(testQdrantAddr, dim)
			if err != nil {
				t.Fatalf("build eval store: %v", err)
			}
			scope := "retrieval-eval:project:" + tc.name

			idByKey := make(map[string]string, len(tc.seedRecords))
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

				// Exact prod doc-embed sequence (round-2 finding 2):
				// store.EmbedText folds tags in, then Embed, then Upsert takes
				// the precomputed vector — never a raw/bespoke shortcut.
				vec, err := em.Embed(ctx, store.EmbedText(m.Content, m.Tags))
				if err != nil {
					t.Fatalf("seed %s: embed: %v", rec.key, err)
				}
				if err := st.Upsert(ctx, m, vec); err != nil {
					t.Fatalf("seed %s: upsert: %v", rec.key, err)
				}
			}
			wantID, ok := idByKey[tc.wantKey]
			if !ok {
				t.Fatalf("fixture bug: wantKey %q not among seeded records", tc.wantKey)
			}
			ceilingK := uint64(len(tc.seedRecords)) + 5

			var recallHits, mrrSum float64
			for _, q := range tc.queries {
				// The SHIPPED ranking path (review finding 2/5): EmbedQuery
				// (prod-parity instruction + params) -> Store.SearchReranked at
				// the production default k — the SAME shared helper
				// deps.searchMemory (MCP) and engramAPI.SearchMemories (Connect)
				// call, so this measures exactly what ships and cannot drift
				// from a raw Store.Search shortcut.
				vec, err := em.EmbedQuery(ctx, q.text)
				if err != nil {
					t.Fatalf("%s: embed query: %v", q.name, err)
				}
				ranked, err := st.SearchReranked(ctx, scope, subj, q.text, vec, defaultK, store.SearchOptions{})
				if err != nil {
					t.Fatalf("%s: search: %v", q.name, err)
				}

				// Harness-correctness assertions (hard): non-empty and
				// score-populated. NOT asserted non-increasing / score-descending
				// here (review finding 3): reranking legitimately promotes a
				// higher-lexical-overlap hit ahead of a higher-raw-score one, so
				// the reranked result set is not guaranteed score-descending.
				if len(ranked) == 0 {
					t.Errorf("%s: search returned no results", q.name)
				}
				for i, m := range ranked {
					if m.Score == 0 {
						t.Errorf("%s: result %d (%s) has zero score", q.name, i, m.ID)
					}
				}

				ids := make([]string, len(ranked))
				for i, m := range ranked {
					ids[i] = m.ID
				}
				hit := recallAtK(ids, wantID)
				rr := reciprocalRank(ids, wantID)
				if hit {
					recallHits++
				}
				mrrSum += rr

				// THE #261 ACCEPTANCE BAR (hard, RANK-based — review finding
				// 3/4, D-03 supersession per round-2 finding 4): Record T MUST
				// surface within default k, by POSITION, for this query. Raw-
				// score separation is reported below as a t.Logf DIAGNOSTIC
				// ONLY — never a hard gate — because a lexical reranker can
				// promote T's rank without necessarily raising its raw Qdrant
				// score above every sticky neighbor (Score is raw first-stage
				// dense similarity, store.go, unchanged by rerank).
				var wantScore, bestOtherScore float32
				rank := 0
				for i, m := range ranked {
					if m.ID == wantID {
						wantScore = m.Score
						rank = i + 1
					} else if m.Score > bestOtherScore {
						bestOtherScore = m.Score
					}
				}
				if rank == 0 {
					t.Errorf("%s/%s: Record T did NOT surface within default k=%d (hard rank bar FAILED — D-06 insufficient for this query, evidence for a D-07/D-08 escalation)", tc.name, q.name, defaultK)
				} else {
					t.Logf("%s/%s: rank=%d/%d (hard rank bar: PASS)", tc.name, q.name, rank, defaultK)
				}
				t.Logf("%s/%s: score(T)=%f best-distractor-score=%f gap=%f (diagnostic only, never a hard gate — review finding 3)",
					tc.name, q.name, wantScore, bestOtherScore, wantScore-bestOtherScore)

				// Prove the harness itself can find T with a generous ceiling —
				// this is NOT the #261 quality bar (that is the hard rank
				// assertion above); it only proves the fixture/harness are
				// sound, independent of ranking quality, so it deliberately
				// uses raw Store.Search rather than the reranked path.
				ceiling, err := st.Search(ctx, scope, subj, vec, ceilingK, store.SearchOptions{})
				if err != nil {
					t.Fatalf("%s: ceiling search: %v", q.name, err)
				}
				ceilingIDs := make([]string, len(ceiling))
				for i, m := range ceiling {
					ceilingIDs[i] = m.ID
				}
				if !recallAtK(ceilingIDs, wantID) {
					t.Errorf("%s: Record T not found even at ceiling k=%d — harness/fixture bug, not a ranking result", q.name, ceilingK)
				}
			}

			recallAtDefaultK := recallHits / float64(len(tc.queries))
			mrr := mrrSum / float64(len(tc.queries))
			t.Logf("%s: recall@%d=%.2f MRR=%.3f (post-D-06-fix; compare against the 09-01 post-#262 baseline)",
				tc.name, defaultK, recallAtDefaultK, mrr)
		})
	}
}

// TestRetrievalEval_AsymmetryDiffer is the Pitfall-12 correctness gate
// (REQ-embed-gemini-direct): it embeds differProbe through BOTH the
// query-side (em.EmbedQuery) and document-side (em.Embed) production paths
// and asserts the two vectors differ. The "TestRetrievalEval" prefix is
// load-bearing — task eval:retrieval's `go test -run TestRetrievalEval`
// substring-matches this name too, so the documented eval command genuinely
// reaches the differ assertion without any Taskfile change.
//
// It never touches Qdrant (no testQdrantAddr / seedRecord involved), but it
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
	if os.Getenv("ENGRAM_RETRIEVAL_EVAL") != "1" {
		t.Skip("set ENGRAM_RETRIEVAL_EVAL=1 (and the gateway/model env) to run the retrieval eval")
	}

	// Symmetric-config guard (review B3): a symmetric embedder config (all
	// four instruction/params env vars empty) legitimately produces
	// query==document — e.g. OpenAI text-embedding-3-small, bare bge-m3. The
	// inequality assertion below does not apply to those configs, so skip it
	// rather than fail the suite for a valid symmetric setup.
	if os.Getenv("ENGRAM_EMBED_QUERY_INSTRUCTION") == "" &&
		os.Getenv("ENGRAM_EMBED_DOCUMENT_INSTRUCTION") == "" &&
		os.Getenv("ENGRAM_EMBED_QUERY_PARAMS") == "" &&
		os.Getenv("ENGRAM_EMBED_DOCUMENT_PARAMS") == "" {
		t.Skip("symmetric embedder config (no QUERY/DOCUMENT instruction or params set): query == document is valid here, asymmetry assertion does not apply")
	}

	ctx := context.Background()

	// Prod-parity embedder — the SAME builder/path TestRetrievalEval uses
	// (D-03: never a bespoke embed shortcut). Its own store is discarded; this
	// test never touches Qdrant. dim is KEPT (not discarded) to assert the
	// vectors are correctly sized, not just non-empty (review B4).
	_, dim, em, _, err := server.StoreAndEmbedderFromEnvNoEnsure()
	if err != nil {
		t.Fatalf("build prod-parity embedder: %v", err)
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

	// THE Pitfall-12 correctness gate (D-04, hard t.Fatal — not t.Errorf — so
	// this stops immediately and cannot be obscured by later output, review
	// B4): the query-side and document-side vectors of the SAME string MUST
	// differ once asymmetric embedding is correctly configured.
	if reflect.DeepEqual(queryVec, documentVec) {
		t.Fatalf("asymmetry differ FAIL: query vector == document vector (dim=%d) — the asymmetric instruction-prefix had no effect; the operator likely wired the no-op ENGRAM_EMBED_QUERY_PARAMS/ENGRAM_EMBED_DOCUMENT_PARAMS/task_type mechanism instead of ENGRAM_EMBED_QUERY_INSTRUCTION/ENGRAM_EMBED_DOCUMENT_INSTRUCTION", dim)
	}

	t.Logf("asymmetry differ PASS: query vector != document vector (dim=%d) — instruction-prefix took effect", dim)
}

// newTestcontainerStore builds a *store.Store pinned to addr — the eval's
// testcontainer, NEVER the ambient ENGRAM_QDRANT_ADDR a developer's prod-like
// env might set (round-2 finding 1) — in a fresh, uniquely-named collection
// ensured at dim.
func newTestcontainerStore(addr string, dim uint64) (*store.Store, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid test Qdrant address %q: %w", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid test Qdrant port %q: %w", portStr, err)
	}
	qc, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: port})
	if err != nil {
		return nil, fmt.Errorf("qdrant client: %w", err)
	}
	st := store.New(qc, "retrievaleval_"+uuid.NewString())
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := st.EnsureCollection(ctx, dim); err != nil {
		return nil, fmt.Errorf("EnsureCollection: %w", err)
	}
	return st, nil
}

// TestMain gates the whole package on ENGRAM_RETRIEVAL_EVAL as its FIRST
// statement, before any testcontainer/Docker startup (review finding 1): when
// the gate is unset, the required `test` job's `go test ./...` pays zero
// ADDITIONAL Docker/Qdrant cost from this package (round-2 finding 8).
func TestMain(m *testing.M) {
	if os.Getenv("ENGRAM_RETRIEVAL_EVAL") != "1" {
		os.Exit(m.Run())
	}
	if addr := os.Getenv("ENGRAM_QDRANT_TEST_ADDR"); addr != "" {
		testQdrantAddr = addr
		os.Exit(m.Run())
	}
	// Bound startup so an unreachable daemon or a stalled image pull fails fast
	// instead of hanging the suite. os.Exit skips defers, so cancel explicitly.
	startCtx, startCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	container, err := tcqdrant.Run(startCtx, qdrantImageTag)
	if err != nil {
		startCancel()
		fmt.Fprintf(os.Stderr, "qdrant testcontainer unavailable (%v); the retrieval eval will skip — set ENGRAM_QDRANT_TEST_ADDR or start Docker\n", err)
		os.Exit(m.Run())
	}
	testQdrantAddr, err = container.GRPCEndpoint(startCtx)
	startCancel()
	if err != nil {
		terminateQdrant(container)
		fmt.Fprintf(os.Stderr, "qdrant grpc endpoint: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	terminateQdrant(container)
	os.Exit(code)
}

// terminateQdrant tears down the container under a bounded context so a slow
// Docker shutdown cannot hang the suite.
func terminateQdrant(c *tcqdrant.QdrantContainer) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = c.Terminate(ctx)
}
