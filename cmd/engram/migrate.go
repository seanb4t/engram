// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/seanb4t/engram/internal/server"
)

var migrateOwner string

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
		n, err := st.MigrateSetOwner(context.Background(), migrateOwner)
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
	rootCmd.AddCommand(migrateSetOwnerCmd)
}
