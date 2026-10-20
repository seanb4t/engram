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
	storeDiscoveryContent   string
	storeDiscoveryKind      string
	storeDiscoveryCitations []string
	storeDiscoveryScope     string
	storeDiscoveryTags      []string
	storeDiscoverySummary   string
	storeDiscoveryID        string
)

// storeDiscoveryCmd is `engram store-discovery`, the client-tier verb over
// the StoreDiscovery RPC: a citation-backed discovery in a discovery:*
// scope, the same write the store_discovery MCP tool makes. --id replaces
// an existing discovery in place. Every rejection (missing citations, a
// kind outside map/fact, a non-discovery scope) comes from the shared
// server-side core with its field=/hint= envelope.
var storeDiscoveryCmd = &cobra.Command{
	Use:   "store-discovery",
	Short: "Store a citation-backed discovery in a discovery scope",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if storeDiscoveryContent == "" {
			return usageErrorf("--content is required")
		}
		citations, err := parseCitationFlags(storeDiscoveryCitations)
		if err != nil {
			return err
		}
		client, format, timeout, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		resp, err := client.StoreDiscovery(ctx, connect.NewRequest(&engramv1.StoreDiscoveryRequest{
			Content:   storeDiscoveryContent,
			Kind:      storeDiscoveryKind,
			Citations: citations,
			Scope:     storeDiscoveryScope,
			Tags:      storeDiscoveryTags,
			Summary:   storeDiscoverySummary,
			Id:        storeDiscoveryID,
		}))
		if err != nil {
			return wrapRPCError(err)
		}
		if format == formatText {
			return renderIDLine(cmd.OutOrStdout(), "stored discovery", resp.Msg.GetShortId(), resp.Msg.GetId())
		}
		return renderJSON(cmd.OutOrStdout(), resp.Msg)
	},
}

func init() {
	addClientFlags(storeDiscoveryCmd)
	storeDiscoveryCmd.Flags().StringVar(&storeDiscoveryContent, "content", "", "discovery content (required)")
	storeDiscoveryCmd.Flags().StringVar(&storeDiscoveryKind, "kind", "", `discovery kind: "map" or "fact" (required)`)
	storeDiscoveryCmd.Flags().StringSliceVar(&storeDiscoveryCitations, "citation", nil,
		"citation as <kind>:<ref> (kind: file, commit, url or repo); repeatable; at least one is required")
	storeDiscoveryCmd.Flags().StringVar(&storeDiscoveryScope, "scope", "", "discovery scope (discovery:repo:<repo>) (required)")
	storeDiscoveryCmd.Flags().StringSliceVar(&storeDiscoveryTags, "tags", nil, "tags to attach to the discovery")
	storeDiscoveryCmd.Flags().StringVar(&storeDiscoverySummary, "summary", "", "client-authored summary")
	storeDiscoveryCmd.Flags().StringVar(&storeDiscoveryID, "id", "", "id (full UUID or short id) of an existing discovery to replace in place; omit to create")
	rootCmd.AddCommand(storeDiscoveryCmd)
}
