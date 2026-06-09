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
			return fmt.Errorf("--owner (or MEM_MIGRATE_OWNER) is required and must be a non-empty OIDC sub")
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

func init() {
	migrateSetOwnerCmd.Flags().StringVar(&migrateOwner, "owner",
		server.EnvOr("MEM_MIGRATE_OWNER", ""),
		"OIDC sub to stamp onto owner-less records (required, non-empty)")
	migrateSetOwnerCmd.Flags().DurationVar(&migrateTimeout, "timeout", 5*time.Minute,
		"max wall-clock for the backfill (0 disables); also cancellable via Ctrl-C")
	rootCmd.AddCommand(migrateSetOwnerCmd)
}
