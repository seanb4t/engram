// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

var (
	searchQuery         string
	searchScope         string
	searchCrossSpine    bool
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
		// D-01/D-02/D-04 pre-flight guard, fired before any dialing.
		if err := requireScopeUnlessCrossSpine(searchScope, searchCrossSpine); err != nil {
			return err
		}
		client, format, timeout, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		// The deadline comes from clientFromFlags' resolved timeout — the
		// single resolution point (D-05) — never a second resolution here.
		// cmd.Context() is only ever the PARENT of this derived context,
		// never passed to the Connect call directly (E-13).
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		resp, err := client.SearchMemories(ctx, connect.NewRequest(&engramv1.SearchMemoriesRequest{
			Query:         searchQuery,
			Scope:         searchScope,
			CrossSpine:    searchCrossSpine,
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
			if err := renderMemoryTable(cmd.OutOrStdout(), resp.Msg.GetMemories(), true); err != nil {
				return err
			}
			return renderCoverageFooter(cmd.OutOrStdout(), searchCrossSpine, resp.Msg.GetSearchedScopes(), resp.Msg.GetScopesTruncated())
		}
		return renderJSON(cmd.OutOrStdout(), resp.Msg)
	},
}

func init() {
	addClientFlags(searchCmd)
	searchCmd.Flags().StringVar(&searchQuery, "query", "", "search query (required)")
	searchCmd.Flags().StringVar(&searchScope, "scope", "",
		"limit recall to one scope; omit and pass --cross-spine to span every scope you can read; mutually exclusive with --cross-spine")
	searchCmd.Flags().BoolVar(&searchCrossSpine, "cross-spine", false,
		"span every scope you can read; mutually exclusive with --scope")
	searchCmd.Flags().Uint64Var(&searchK, "k", 0, "max results (0 = server default)")
	searchCmd.Flags().StringSliceVar(&searchTags, "tags", nil, "tag filter (records must carry ALL listed tags)")
	searchCmd.Flags().BoolVar(&searchFull, "full", false, "return full content instead of summaries")
	searchCmd.Flags().StringVar(&searchCreatedAfter, "created-after", "", "RFC3339 inclusive lower bound on created_at")
	searchCmd.Flags().StringVar(&searchCreatedBefore, "created-before", "", "RFC3339 exclusive upper bound on created_at")
	searchCmd.Flags().StringSliceVar(&searchCategories, "categories", nil, "category filter (ANY listed category)")
	// D-07: the second of the three exclusivity claim sites. cobra's flag
	// groups count a *supplied* flag, not its value, so --cross-spine=false
	// is rejected too (the search/scope+cross-spine-false baseline row) —
	// the same D-08 widened-blast-radius semantics plan 01-02 established
	// for the paging trio. requireScopeUnlessCrossSpine (client_common.go)
	// still enforces the surviving asymmetric half.
	searchCmd.MarkFlagsMutuallyExclusive("scope", "cross-spine")
	rootCmd.AddCommand(searchCmd)
}
