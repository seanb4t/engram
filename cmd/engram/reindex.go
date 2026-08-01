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
	reindexSource  string
	reindexDryRun  bool
	reindexResume  bool
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
		st, dim, em, identity, err := server.StoreAndEmbedderFromEnvNoEnsure()
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

		// Per-batch progress goes to stderr so it never pollutes the single
		// parseable summary line on stdout (engram-xddn).
		res, err := st.Reindex(ctx, store.ReindexOptions{
			Target:   reindexTarget,
			Source:   reindexSource,
			Dim:      dim,
			DryRun:   reindexDryRun,
			Resume:   reindexResume,
			Identity: identity,
			Progress: func(r store.ReindexResult) {
				cmd.PrintErrf("reindex progress: scanned %d, upserted %d, skipped %d, unchanged %d\n",
					r.Scanned, r.Upserted, r.Skipped, r.Unchanged)
			},
		}, em.Embed)
		if err != nil {
			return err
		}
		cmd.Println(reindexSummary(res, reindexTarget, dim, reindexDryRun, reindexResume))
		return nil
	},
}

// reindexSummary renders the operator-facing one-line result of a reindex run.
// Kept pure (no I/O) so the dry-run vs cutover wording is unit-testable without a
// live Qdrant. dry-run-without-resume returns the pre-D-14 format string
// unchanged, character for character — that is what keeps this change
// additive for anyone parsing that line; dry-run-with-resume returns a
// different line sizing the repair (D-14, REQ-reindex-stale-repair).
func reindexSummary(res store.ReindexResult, target string, dim uint64, dryRun, resume bool) string {
	if dryRun {
		if resume {
			return fmt.Sprintf("dry-run --resume: %d would be re-embedded, %d would be skipped (unchanged), "+
				"%d skipped (no content), %d scanned, into %q at dim %d",
				res.WouldUpsert, res.Unchanged, res.Skipped, res.Scanned, target, dim)
		}
		return fmt.Sprintf("dry-run: %d record(s) would be re-embedded into %q at dim %d",
			res.Scanned, target, dim)
	}
	return fmt.Sprintf("re-embedded %d/%d record(s) into %q at dim %d "+
		"(%d skipped, no content; %d unchanged); "+
		"source left untouched — verify, then set ENGRAM_QDRANT_COLLECTION=%s and restart to cut over",
		res.Upserted, res.Scanned, target, dim, res.Skipped, res.Unchanged, target)
}

func init() {
	reindexCmd.Flags().StringVar(&reindexTarget, "target",
		os.Getenv("ENGRAM_REINDEX_TARGET"),
		"target collection to create and populate (required)")
	reindexCmd.Flags().StringVar(&reindexSource, "source", "",
		"source collection to read from (default: ENGRAM_QDRANT_COLLECTION)")
	reindexCmd.Flags().BoolVar(&reindexDryRun, "dry-run", false,
		"scan and count without creating the target or writing")
	reindexCmd.Flags().BoolVar(&reindexResume, "resume", false,
		"skip points already present in the target with identical content (cheap restart of an interrupted reindex)")
	reindexCmd.Flags().DurationVar(&reindexTimeout, "timeout", 30*time.Minute,
		"max wall-clock for the reindex; 0 means no deadline (Ctrl-C or SIGTERM aborts either way)")
	rootCmd.AddCommand(reindexCmd)
}
