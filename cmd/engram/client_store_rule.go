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
	storeRuleContent string
	storeRuleScope   string
	storeRuleSummary string
	storeRuleTags    []string
	storeRuleID      string
)

// storeRuleCmd is `engram store-rule`, the client-tier verb over the
// StoreRule RPC: a normative, always-shared rule in a rule:repo:* or
// rule:project:* scope, the same write the store_rule MCP tool makes. It
// runs only when a person invokes it — there is no promotion path from
// any other verb, and no agent-side automation calls it.
var storeRuleCmd = &cobra.Command{
	Use:   "store-rule",
	Short: "Store a user-blessed rule (always shared) in a rule scope",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if storeRuleContent == "" {
			return usageErrorf("--content is required")
		}
		client, format, timeout, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		resp, err := client.StoreRule(ctx, connect.NewRequest(&engramv1.StoreRuleRequest{
			Content: storeRuleContent,
			Scope:   storeRuleScope,
			Summary: storeRuleSummary,
			Tags:    storeRuleTags,
			Id:      storeRuleID,
		}))
		if err != nil {
			return wrapRPCError(err)
		}
		if format == formatText {
			return renderIDLine(cmd.OutOrStdout(), "stored rule", resp.Msg.GetShortId(), resp.Msg.GetId())
		}
		return renderJSON(cmd.OutOrStdout(), resp.Msg)
	},
}

func init() {
	addClientFlags(storeRuleCmd)
	storeRuleCmd.Flags().StringVar(&storeRuleContent, "content", "", "the full rule text, max 8 KiB (required)")
	storeRuleCmd.Flags().StringVar(&storeRuleScope, "scope", "", "rule scope: rule:repo:<repo> or rule:project:<project> (required)")
	storeRuleCmd.Flags().StringVar(&storeRuleSummary, "summary", "", "one-line index entry: a single physical line, max 256 bytes (required)")
	storeRuleCmd.Flags().StringSliceVar(&storeRuleTags, "tags", nil, "concern-area labels, e.g. vcs, deploy, authz")
	storeRuleCmd.Flags().StringVar(&storeRuleID, "id", "", "id (full UUID or short id) of an existing rule to replace in place, preserving its short id; omit to create")
	rootCmd.AddCommand(storeRuleCmd)
}
