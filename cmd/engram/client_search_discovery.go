// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

var (
	searchDiscoveryQuery string
	searchDiscoveryScope string
	searchDiscoveryK     uint64
)

// searchDiscoveryCmd is `engram search-discovery`, the client-tier verb
// over the SearchDiscoveries RPC: semantic search over the discovery pool
// in one discovery:* scope — the same read the search_discovery MCP tool
// makes, with archived and superseded discoveries absent on both lanes.
// The text lane renders the whole response through the operator view, so
// each discovery's kind, citations and score are shown.
var searchDiscoveryCmd = &cobra.Command{
	Use:   "search-discovery",
	Short: "Search citation-backed discoveries in a discovery scope",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if searchDiscoveryQuery == "" {
			return usageErrorf("--query is required")
		}
		client, format, timeout, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		resp, err := client.SearchDiscoveries(ctx, connect.NewRequest(&engramv1.SearchDiscoveriesRequest{
			Query: searchDiscoveryQuery,
			Scope: searchDiscoveryScope,
			K:     searchDiscoveryK,
		}))
		if err != nil {
			return wrapRPCError(err)
		}
		if format == formatText {
			headline := fmt.Sprintf("discoveries: %d", len(resp.Msg.GetDiscoveries()))
			return renderProtoView(cmd.OutOrStdout(), headline, resp.Msg)
		}
		return renderJSON(cmd.OutOrStdout(), resp.Msg)
	},
}

func init() {
	addClientFlags(searchDiscoveryCmd)
	searchDiscoveryCmd.Flags().StringVar(&searchDiscoveryQuery, "query", "", "search query (required)")
	searchDiscoveryCmd.Flags().StringVar(&searchDiscoveryScope, "scope", "", "discovery scope (discovery:repo:<repo>); omit to span every discovery scope you can read")
	searchDiscoveryCmd.Flags().Uint64Var(&searchDiscoveryK, "k", 0, "max results; 0 resolves to the default, 20; a value above the maximum, 1000, is rejected")
	rootCmd.AddCommand(searchDiscoveryCmd)
}
