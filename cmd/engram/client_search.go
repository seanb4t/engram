// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"os"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

var (
	searchQuery         string
	searchScope         string
	searchK             uint64
	searchTags          []string
	searchFull          bool
	searchCreatedAfter  string
	searchCreatedBefore string
	searchCategories    []string
)

// searchCmd is `engram search`, the phase's tracer command.
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search memories on a remote engram server",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		// Reject an empty --query before building anything: the client's
		// own semantic validation, which is exactly what D-17 reserves
		// exit 2 for.
		if searchQuery == "" {
			return usageErrorf("--query is required")
		}
		format, err := resolveOutputFormat(clientOutput, isTerminal(os.Stdout))
		if err != nil {
			return err
		}
		client, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		resp, err := client.SearchMemories(cmd.Context(), connect.NewRequest(&engramv1.SearchMemoriesRequest{
			Query:         searchQuery,
			Scope:         searchScope,
			K:             searchK,
			Tags:          searchTags,
			Full:          searchFull,
			CreatedAfter:  searchCreatedAfter,
			CreatedBefore: searchCreatedBefore,
			Categories:    searchCategories,
		}))
		if err != nil {
			// Do not retry. Return wrapRPCError(err) and let Execute() map
			// it to a process exit code (D-09/D-10).
			return wrapRPCError(err)
		}
		// D-12: return nil regardless of how many memories came back — an
		// empty result set is a legitimate answer, not a failure.
		if format == formatText {
			return renderMemoryTable(cmd.OutOrStdout(), resp.Msg.GetMemories(), true)
		}
		return renderJSON(cmd.OutOrStdout(), resp.Msg)
	},
}

func init() {
	addClientFlags(searchCmd)
	searchCmd.Flags().StringVar(&searchQuery, "query", "", "search query (required)")
	searchCmd.Flags().StringVar(&searchScope, "scope", "", "scope filter")
	searchCmd.Flags().Uint64Var(&searchK, "k", 0, "max results (0 = server default)")
	searchCmd.Flags().StringSliceVar(&searchTags, "tags", nil, "tag filter (records must carry ALL listed tags)")
	searchCmd.Flags().BoolVar(&searchFull, "full", false, "return full content instead of summaries")
	searchCmd.Flags().StringVar(&searchCreatedAfter, "created-after", "", "RFC3339 inclusive lower bound on created_at")
	searchCmd.Flags().StringVar(&searchCreatedBefore, "created-before", "", "RFC3339 exclusive upper bound on created_at")
	searchCmd.Flags().StringSliceVar(&searchCategories, "categories", nil, "category filter (ANY listed category)")
	rootCmd.AddCommand(searchCmd)
}
