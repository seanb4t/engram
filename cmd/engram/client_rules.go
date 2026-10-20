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
	rulesScopes []string
	rulesTags   []string
	rulesFull   bool
)

// rulesCmd is `engram rules`, the client-tier verb over the ListRules RPC:
// the complete rule set for one or more rule:* scopes, oldest-first, in
// the compact index shape by default — the same read the list_rules MCP
// tool makes. With no --scope it is the all-scopes read, and the text lane
// reports the covered rule scopes through the same coverage footer engram
// list --cross-spine prints.
var rulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "List the rules in one or more rule scopes, or every readable rule scope",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, format, timeout, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		resp, err := client.ListRules(ctx, connect.NewRequest(&engramv1.ListRulesRequest{
			Scopes: rulesScopes,
			Tags:   rulesTags,
			Full:   rulesFull,
		}))
		if err != nil {
			return wrapRPCError(err)
		}
		if format == formatText {
			if err := renderMemoryTable(cmd.OutOrStdout(), resp.Msg.GetRules(), false); err != nil {
				return err
			}
			if a := resp.Msg.GetAdvisory(); a != "" {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "advisory: %s\n", sanitizeViewValue(a)); err != nil {
					return err
				}
			}
			// The coverage footer is gated on the request shape, exactly
			// as list gates it on --cross-spine: an all-scopes read is the
			// only one that carries searched_scopes.
			allScopes := len(rulesScopes) == 0
			return renderRecallFooters(cmd.OutOrStdout(), allScopes, resp.Msg.GetSearchedScopes(), resp.Msg.GetScopesTruncated(), resp.Msg.GetScopesUnknown(), nil)
		}
		return renderJSON(cmd.OutOrStdout(), resp.Msg)
	},
}

func init() {
	addClientFlags(rulesCmd)
	rulesCmd.Flags().StringSliceVar(&rulesScopes, "scope", nil, "rule scope (rule:repo:<repo> or rule:project:<project>); repeatable; omit to read every rule scope you can read in one call")
	rulesCmd.Flags().StringSliceVar(&rulesTags, "tag", nil, "tag filter (rules must carry ALL listed tags); repeatable")
	rulesCmd.Flags().BoolVar(&rulesFull, "full", false, "return full rule content instead of the compact index shape")
	rootCmd.AddCommand(rulesCmd)
}
