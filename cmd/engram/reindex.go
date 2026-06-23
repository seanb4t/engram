// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/server"
	"github.com/seanb4t/engram/internal/store"
)

var (
	reindexTarget  string
	reindexDryRun  bool
	reindexTimeout time.Duration
)

// reindexCmd re-embeds every stored memory into a new target collection at the
// currently-configured embedder's dimension (ENGRAM_EMBED_DIM). Qdrant vector size
// is immutable and vectors from different models aren't comparable, so migrating
// to an embedder with a different output dimension requires a fresh collection.
// The source (ENGRAM_QDRANT_COLLECTION) is left untouched so the operator can verify
// the target before cutting ENGRAM_QDRANT_COLLECTION over and restarting.
//
// Operator flow: point ENGRAM_EMBED_MODEL / ENGRAM_EMBED_DIM / ENGRAM_OPENAI_BASE_URL at the
// new embedder, then `engram reindex --target <new> --dry-run` to sanity-check
// the count, then `engram reindex --target <new>` to populate it.
var reindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Re-embed memories into a new (new-dimension) collection for embedder migration",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if reindexTarget == "" {
			return fmt.Errorf("--target (new collection name) is required")
		}
		// Build the source store WITHOUT creating its collection: reindex must read
		// an EXISTING source (Reindex errors if it's absent), not conjure an empty
		// one at the new dimension. dim is the currently-configured embedder's
		// ENGRAM_EMBED_DIM — reused as the target collection's dimension. Store and
		// embedder come from a single config load.
		st, dim, em, err := server.StoreAndEmbedderFromEnvNoEnsure()
		if err != nil {
			return err
		}

		// Bound the reindex so a hung Qdrant/embedder cannot block forever, and let
		// the operator abort with Ctrl-C / SIGTERM.
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if reindexTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, reindexTimeout)
			defer cancel()
		}

		res, err := st.Reindex(ctx, store.ReindexOptions{
			Target: reindexTarget,
			Dim:    dim,
			DryRun: reindexDryRun,
		}, em.Embed)
		if err != nil {
			return err
		}
		if reindexDryRun {
			cmd.Printf("dry-run: %d record(s) would be re-embedded into %q at dim %d\n",
				res.Scanned, reindexTarget, dim)
			return nil
		}
		cmd.Printf("re-embedded %d/%d record(s) into %q at dim %d (%d skipped, no content); source left untouched — verify, then set ENGRAM_QDRANT_COLLECTION=%s and restart to cut over\n",
			res.Upserted, res.Scanned, reindexTarget, dim, res.Skipped, reindexTarget)
		return nil
	},
}

func init() {
	reindexCmd.Flags().StringVar(&reindexTarget, "target",
		os.Getenv("ENGRAM_REINDEX_TARGET"),
		"target collection to create and populate (required)")
	reindexCmd.Flags().BoolVar(&reindexDryRun, "dry-run", false,
		"scan and count without creating the target or writing")
	reindexCmd.Flags().DurationVar(&reindexTimeout, "timeout", 30*time.Minute,
		"max wall-clock for the reindex (0 disables); also cancellable via Ctrl-C")
	rootCmd.AddCommand(reindexCmd)
}
