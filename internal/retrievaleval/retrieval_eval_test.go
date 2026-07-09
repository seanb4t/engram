// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package retrievaleval

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/store"
	tcqdrant "github.com/testcontainers/testcontainers-go/modules/qdrant"
)

// defaultK mirrors deps.searchMemory's production default (tools.go:706) — the
// k a real MCP client experiences when it omits the arg.
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
	_, dim, em, err := server.StoreAndEmbedderFromEnvNoEnsure()
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

				// Exact prod doc-embed sequence (tools.go:508-515, round-2 finding
				// 2): EmbedText folds tags in, then Embed, then Upsert takes the
				// precomputed vector — never a raw/bespoke shortcut.
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
				// Production query path: EmbedQuery (prod-parity instruction +
				// params) -> Store.Search at the production default k.
				vec, err := em.EmbedQuery(ctx, q.text)
				if err != nil {
					t.Fatalf("%s: embed query: %v", q.name, err)
				}
				ranked, err := st.Search(ctx, scope, subj, vec, defaultK, nil, time.Time{}, time.Time{})
				if err != nil {
					t.Fatalf("%s: search: %v", q.name, err)
				}

				// Harness-correctness assertions (hard, this plan): the RAW
				// Store.Search output is score-populated and non-increasing.
				// Scoped to raw output only — Plan 03 drops the descending
				// assumption post-rerank (review finding 3).
				if len(ranked) == 0 {
					t.Errorf("%s: search returned no results", q.name)
				}
				for i, m := range ranked {
					if m.Score == 0 {
						t.Errorf("%s: result %d (%s) has zero score", q.name, i, m.ID)
					}
					if i > 0 && ranked[i-1].Score < m.Score {
						t.Errorf("%s: raw results not non-increasing at index %d: %f < %f", q.name, i, ranked[i-1].Score, m.Score)
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

				// Diagnostic-only baseline capture (review finding 3): report,
				// never assert, Record T's pre-fix rank and its raw score versus
				// the best distractor's score. For #261's crowding case T's score
				// may legitimately sit BELOW its sticky neighbors by
				// construction — that is exactly the gap Plan 03 must close.
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
					t.Logf("%s/%s: MISS within default k=%d (baseline pre-fix: not recalled)", tc.name, q.name, defaultK)
				} else {
					t.Logf("%s/%s: rank=%d/%d (baseline pre-fix rank Plan 03 must improve)", tc.name, q.name, rank, defaultK)
				}
				t.Logf("%s/%s: score(T)=%f best-distractor-score=%f gap=%f (diagnostic only, not a hard gate)",
					tc.name, q.name, wantScore, bestOtherScore, wantScore-bestOtherScore)

				// Prove the harness itself can find T with a generous ceiling —
				// this is NOT the #261 quality bar (that stays a Plan 03 hard
				// gate on default k); it only proves the fixture/harness are
				// sound, independent of ranking quality.
				ceiling, err := st.Search(ctx, scope, subj, vec, ceilingK, nil, time.Time{}, time.Time{})
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
			t.Logf("%s: recall@%d=%.2f MRR=%.3f (post-#262 baseline; Plan 03's ranking fix must improve this)",
				tc.name, defaultK, recallAtDefaultK, mrr)
		})
	}
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
