// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
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
			return usageErrorf("--owner (or ENGRAM_MIGRATE_OWNER) is required and must be a non-empty OIDC sub")
		}
		// D-05 reconciliation: this binary's other --timeout (client.timeout,
		// internal/config/client_validate.go) rejects 0 as a usage error
		// rather than treating it as unbounded, so migrate-set-owner's own
		// --timeout of the same name converges onto the same rule -- the
		// binary must not ship two --timeout flags with opposite semantics.
		if migrateTimeout <= 0 {
			return usageErrorf("--timeout must be greater than 0 -- a timeout of 0 is not treated as unbounded")
		}
		st, err := server.StoreFromEnv()
		if err != nil {
			return classifyOperatorErrConstruction(err)
		}
		// Bound the backfill so a hung Qdrant cannot block forever, and let the
		// operator abort with Ctrl-C / SIGTERM. context.Background() previously
		// gave the underlying gRPC calls no deadline or cancellation path.
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		ctx, cancel := context.WithTimeout(ctx, migrateTimeout)
		defer cancel()
		n, err := st.MigrateSetOwner(ctx, migrateOwner)
		if err != nil {
			return classifyOperatorErr(err)
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

// buildRemapSource constructs the sealed store.OwnerRemapSource and runs the
// shared store.ValidateOwnerRemap (--to non-empty, no-op --from X --to X) so
// both fail fast before opening a Qdrant connection. Pure (no I/O) so it is
// unit-testable.
//
// migrateRemapOwnerCmd declares both MarkFlagsMutuallyExclusive and
// MarkFlagsOneRequired over from/from-missing/from-anon, validated centrally
// by rootCmd.PersistentPreRunE before RunE — and therefore this function —
// ever runs, so exactly one of the three is always supplied here (D-07).
// This function no longer counts selected sources.
//
// The one case cobra's flag groups CANNOT express: a supplied --from with an
// empty value. MarkFlagsOneRequired only tracks whether the flag was
// SUPPLIED (pflag.Flag.Changed), not whether its value is usable — "--from
// ''" satisfies OneRequired but yields a source no different from "no source
// at all". Rejected explicitly below rather than silently accepted as
// RemapFrom("").
func buildRemapSource(from string, missing, anon bool, to string) (store.OwnerRemapSource, error) {
	var src store.OwnerRemapSource
	switch {
	case missing:
		src = store.RemapMissing()
	case anon:
		src = store.RemapAnon()
	case from == "":
		return nil, usageErrorf("--from requires a non-empty value")
	default:
		src = store.RemapFrom(from)
	}
	if err := store.ValidateOwnerRemap(src, to); err != nil {
		// Wrap, don't reword (D-02/D-03): store.ValidateOwnerRemap's three
		// bare errors (nil source, empty --to, identical --from/--to) become
		// exit 2 via the carrier alone.
		return nil, usageErrorf("%w", err)
	}
	return src, nil
}

var migrateRemapOwnerCmd = &cobra.Command{
	Use:   "migrate-remap-owner",
	Short: "Re-stamp record owner across the collection (sub->email, email->email, owner-less, or anonymous bucket)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		src, err := buildRemapSource(remapFrom, remapMissing, remapAnon, remapTo)
		if err != nil {
			// buildRemapSource already carries exitUsage (plan 01-04);
			// return unchanged rather than double-classify.
			return err
		}
		// D-05 reconciliation: see migrate-set-owner's identical guard above.
		if remapTimeout <= 0 {
			return usageErrorf("--timeout must be greater than 0 -- a timeout of 0 is not treated as unbounded")
		}
		st, err := server.StoreFromEnv()
		if err != nil {
			return classifyOperatorErrConstruction(err)
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		ctx, cancel := context.WithTimeout(ctx, remapTimeout)
		defer cancel()
		n, err := st.RemapOwner(ctx, src, remapTo, remapDryRun)
		if err != nil {
			return classifyOperatorErr(err)
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
		"max wall-clock for the backfill (must be > 0); also cancellable via Ctrl-C")
	rootCmd.AddCommand(migrateSetOwnerCmd)

	migrateRemapOwnerCmd.Flags().StringVar(&remapFrom, "from", "", "current owner value to remap (a sub or email); mutually exclusive with --from-missing/--from-anon")
	migrateRemapOwnerCmd.Flags().BoolVar(&remapMissing, "from-missing", false, "remap owner-less (pre-isolation) records")
	migrateRemapOwnerCmd.Flags().BoolVar(&remapAnon, "from-anon", false, "remap the explicit anonymous bucket (owner==\"\")")
	migrateRemapOwnerCmd.Flags().StringVar(&remapTo, "to", "", "new owner value to stamp (required)")
	migrateRemapOwnerCmd.Flags().BoolVar(&remapDryRun, "dry-run", false, "count matching records without writing")
	migrateRemapOwnerCmd.Flags().DurationVar(&remapTimeout, "timeout", 5*time.Minute, "max wall-clock (must be > 0); also cancellable via Ctrl-C")
	// D-07: the third exclusivity claim site, and the only one needing
	// exactly-one-of rather than plain mutual exclusivity.
	// MarkFlagsMutuallyExclusive alone permits zero flags supplied, so it is
	// paired with MarkFlagsOneRequired to make exactly one the only
	// accepted shape. buildRemapSource no longer counts selected sources —
	// cobra guarantees the count before RunE ever runs.
	migrateRemapOwnerCmd.MarkFlagsMutuallyExclusive("from", "from-missing", "from-anon")
	migrateRemapOwnerCmd.MarkFlagsOneRequired("from", "from-missing", "from-anon")
	rootCmd.AddCommand(migrateRemapOwnerCmd)

	// migrate-set-owner is now a deprecated alias for the owner-less case.
	migrateSetOwnerCmd.Deprecated = "use: migrate-remap-owner --from-missing --to <owner>"
}
