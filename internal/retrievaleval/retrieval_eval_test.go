// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package retrievaleval

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	tcqdrant "github.com/testcontainers/testcontainers-go/modules/qdrant"
)

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
//
// Fixture seeding, measurement, and assertions land in Task 2.
func TestRetrievalEval(t *testing.T) {
	if os.Getenv("ENGRAM_RETRIEVAL_EVAL") != "1" {
		t.Skip("set ENGRAM_RETRIEVAL_EVAL=1 (and the gateway/model env) to run the retrieval eval")
	}
	if testQdrantAddr == "" {
		t.Skip("no Qdrant available: set ENGRAM_QDRANT_TEST_ADDR or start Docker (testcontainers)")
	}
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
