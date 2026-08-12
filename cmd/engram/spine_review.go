// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import "github.com/spf13/cobra"

// spineReviewCmd groups the operator tier's inventory, verification and
// curation commands for the memory spine — the operator tier's first
// nested cobra subcommand tree (D-01). It carries no RunE: cobra prints
// its own usage and exits 0 for a bare, non-runnable parent command
// (cobra v1.10.2 command.go:935, :1152), but that default is pinned
// explicitly by TestSpineReviewBareInvocationPrintsHelp rather than
// trusted, the same discipline TestRootBareInvocationEmitsCatalog already
// applies to rootCmd's own bare invocation.
var spineReviewCmd = &cobra.Command{
	Use:   "spine-review",
	Short: "Inventory, verify, and curate the memory spine",
}

func init() {
	rootCmd.AddCommand(spineReviewCmd)
}
