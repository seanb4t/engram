// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

var (
	storeContent   string
	storeScope     string
	storeSource    string
	storeCategory  string
	storeTags      []string
	storeRepo      string
	storeWorkspace string
	storeWorktree  string
	storeBaseDir   string
	storeSummary   string
)

// storeCmd is `engram store`, the phase's only write verb. It issues
// exactly one StoreMemory call per invocation and never retries, on any
// error class (T-02-09): a mutating call retried on an ambiguous failure
// can duplicate a record, and StoreMemoryRequest carries no idempotency
// key to make that safe.
var storeCmd = &cobra.Command{
	Use:   "store",
	Short: "Store a memory on a remote engram server",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		// Reject an empty --content or --scope before building anything:
		// the client's own semantic validation, which is exactly what
		// D-17 reserves exit 2 for. This duplicates the wire's
		// min_len=1 constraint deliberately, as a no-round-trip
		// ergonomic pre-check; the server's rejection remains
		// authoritative and maps to the same exit code either way.
		if storeContent == "" {
			return usageErrorf("--content is required")
		}
		if storeScope == "" {
			return usageErrorf("--scope is required")
		}
		// Timeout is resolved here but not yet applied to the RPC context —
		// plan 01-08 wires it into context.WithTimeout; this plan only
		// makes it exist and be validated (D-05).
		client, format, _, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		resp, err := client.StoreMemory(cmd.Context(), connect.NewRequest(&engramv1.StoreMemoryRequest{
			Content:   storeContent,
			Scope:     storeScope,
			Source:    storeSource,
			Category:  storeCategory,
			Tags:      storeTags,
			Repo:      storeRepo,
			Workspace: storeWorkspace,
			Worktree:  storeWorktree,
			BaseDir:   storeBaseDir,
			Summary:   storeSummary,
		}))
		if err != nil {
			// No retry, on any error class. Return wrapRPCError(err)
			// immediately and let Execute() map it to a process exit
			// code (D-09/D-10).
			return wrapRPCError(err)
		}
		if format == formatText {
			_, err := cmd.OutOrStdout().Write([]byte("stored: " + resp.Msg.GetShortId() + " (" + resp.Msg.GetId() + ")\n"))
			return err
		}
		return renderJSON(cmd.OutOrStdout(), resp.Msg)
	},
}

func init() {
	addClientFlags(storeCmd)
	storeCmd.Flags().StringVar(&storeContent, "content", "", "memory content (required)")
	storeCmd.Flags().StringVar(&storeScope, "scope", "", "memory scope (required)")
	storeCmd.Flags().StringVar(&storeSource, "source", "", "source of the memory")
	storeCmd.Flags().StringVar(&storeCategory, "category", "",
		"category: one of decision, preference, convention, gotcha")
	storeCmd.Flags().StringSliceVar(&storeTags, "tags", nil, "tags to attach to the memory")
	storeCmd.Flags().StringVar(&storeRepo, "repo", "", "repo identifier")
	storeCmd.Flags().StringVar(&storeWorkspace, "workspace", "", "workspace identifier")
	storeCmd.Flags().StringVar(&storeWorktree, "worktree", "", "worktree identifier")
	storeCmd.Flags().StringVar(&storeBaseDir, "base-dir", "", "base directory")
	storeCmd.Flags().StringVar(&storeSummary, "summary", "", "client-authored summary")
	rootCmd.AddCommand(storeCmd)
}
