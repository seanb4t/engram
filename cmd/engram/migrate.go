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
	migrateOwner   string
	migrateTimeout time.Duration
)

// migrateSetOwnerCmd backfills the stable OIDC `sub` onto memory records written
// before per-actor isolation (which carry no `owner` key). One-time, idempotent.
// Run it with OIDC enabled and your real `sub`, so enabling auth keeps the
// records yours rather than orphaning them in the anonymous bucket.
var migrateSetOwnerCmd = &cobra.Command{
	Use:   "migrate-set-owner",
	Short: "Backfill owner (OIDC sub) onto pre-isolation memory records",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if migrateOwner == "" {
			return fmt.Errorf("--owner (or ENGRAM_MIGRATE_OWNER) is required and must be a non-empty OIDC sub")
		}
		st, err := server.StoreFromEnv()
		if err != nil {
			return err
		}
		// Bound the backfill so a hung Qdrant cannot block forever, and let the
		// operator abort with Ctrl-C / SIGTERM. context.Background() previously
		// gave the underlying gRPC calls no deadline or cancellation path.
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if migrateTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, migrateTimeout)
			defer cancel()
		}
		n, err := st.MigrateSetOwner(ctx, migrateOwner)
		if err != nil {
			return err
		}
		cmd.Printf("stamped owner=%s onto %d owner-less record(s)\n", migrateOwner, n)
		return nil
	},
}

var (
	remapFrom    string
	remapMissing bool
	remapAnon    bool
	remapTo      string
	remapDryRun  bool
	remapTimeout time.Duration
)

// buildRemapSource validates the mutually-exclusive source flags and required
// --to, returning the store source. Pure (no I/O) so it is unit-testable.
func buildRemapSource(from string, missing, anon bool, to string) (store.OwnerRemapSource, error) {
	if to == "" {
		return store.OwnerRemapSource{}, fmt.Errorf("--to is required and must be non-empty")
	}
	selected := 0
	if missing {
		selected++
	}
	if anon {
		selected++
	}
	if from != "" {
		selected++
	}
	if selected != 1 {
		return store.OwnerRemapSource{}, fmt.Errorf("exactly one source required: --from <value> | --from-missing | --from-anon")
	}
	// Mirror RemapOwner's check here so a no-op --from X --to X fails fast with a
	// friendly error before opening a Qdrant connection.
	if from != "" && from == to {
		return store.OwnerRemapSource{}, fmt.Errorf("--from and --to are identical (%q)", to)
	}
	return store.OwnerRemapSource{Missing: missing, Anon: anon, From: from}, nil
}

var migrateRemapOwnerCmd = &cobra.Command{
	Use:   "migrate-remap-owner",
	Short: "Re-stamp record owner across the collection (sub->email, email->email, owner-less, or anonymous bucket)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		src, err := buildRemapSource(remapFrom, remapMissing, remapAnon, remapTo)
		if err != nil {
			return err
		}
		st, err := server.StoreFromEnv()
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if remapTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, remapTimeout)
			defer cancel()
		}
		n, err := st.RemapOwner(ctx, src, remapTo, remapDryRun)
		if err != nil {
			return err
		}
		if remapDryRun {
			cmd.Printf("[dry-run] would remap %d record(s) to owner=%s\n", n, remapTo)
		} else {
			cmd.Printf("remapped %d record(s) to owner=%s\n", n, remapTo)
		}
		return nil
	},
}

func init() {
	migrateSetOwnerCmd.Flags().StringVar(&migrateOwner, "owner",
		os.Getenv("ENGRAM_MIGRATE_OWNER"),
		"OIDC sub to stamp onto owner-less records (required, non-empty)")
	migrateSetOwnerCmd.Flags().DurationVar(&migrateTimeout, "timeout", 5*time.Minute,
		"max wall-clock for the backfill (0 disables); also cancellable via Ctrl-C")
	rootCmd.AddCommand(migrateSetOwnerCmd)

	migrateRemapOwnerCmd.Flags().StringVar(&remapFrom, "from", "", "current owner value to remap (a sub or email); mutually exclusive with --from-missing/--from-anon")
	migrateRemapOwnerCmd.Flags().BoolVar(&remapMissing, "from-missing", false, "remap owner-less (pre-isolation) records")
	migrateRemapOwnerCmd.Flags().BoolVar(&remapAnon, "from-anon", false, "remap the explicit anonymous bucket (owner==\"\")")
	migrateRemapOwnerCmd.Flags().StringVar(&remapTo, "to", "", "new owner value to stamp (required)")
	migrateRemapOwnerCmd.Flags().BoolVar(&remapDryRun, "dry-run", false, "count matching records without writing")
	migrateRemapOwnerCmd.Flags().DurationVar(&remapTimeout, "timeout", 5*time.Minute, "max wall-clock (0 disables); also cancellable via Ctrl-C")
	rootCmd.AddCommand(migrateRemapOwnerCmd)

	// migrate-set-owner is now a deprecated alias for the owner-less case.
	migrateSetOwnerCmd.Deprecated = "use: migrate-remap-owner --from-missing --to <owner>"
}
