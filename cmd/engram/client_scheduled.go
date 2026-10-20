// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/surfaces"
)

var (
	scheduledScope         string
	scheduledCrossSpine    bool
	scheduledState         string
	scheduledLimit         uint64
	scheduledCreatedAfter  string
	scheduledCreatedBefore string
	scheduledCursor        string
)

// scheduledCmd is `engram scheduled`, the client-tier verb over the
// ListScheduled RPC: the caller's own windowed records the recall gate is
// hiding (not yet active by default; expired or all by --state), paged by
// an opaque cursor — the same read the list_scheduled MCP tool makes. The
// scope-unless-cross-spine guard and the scope/cross-spine flag group are
// the ones search and list already declare.
var scheduledCmd = &cobra.Command{
	Use:   "scheduled",
	Short: "List your windowed memories the recall gate is hiding",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if err := requireScopeUnlessCrossSpine(scheduledScope, scheduledCrossSpine); err != nil {
			return err
		}
		client, format, timeout, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		resp, err := client.ListScheduled(ctx, connect.NewRequest(&engramv1.ListScheduledRequest{
			Scope:         scheduledScope,
			State:         scheduledState,
			Limit:         scheduledLimit,
			CreatedAfter:  scheduledCreatedAfter,
			CreatedBefore: scheduledCreatedBefore,
			CrossSpine:    scheduledCrossSpine,
			PageToken:     scheduledCursor,
		}))
		if err != nil {
			return wrapRPCError(err)
		}
		if format == formatText {
			if err := renderMemoryTable(cmd.OutOrStdout(), resp.Msg.GetMemories(), false); err != nil {
				return err
			}
			if next := resp.Msg.GetNextPageToken(); next != "" {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "next_page_token: %s\n", sanitizeViewValue(next)); err != nil {
					return err
				}
			}
			// ListScheduledResponse carries the coverage trio but no
			// recall_gate_hidden: this listing IS the hidden set.
			return renderRecallFooters(cmd.OutOrStdout(), scheduledCrossSpine, resp.Msg.GetSearchedScopes(), resp.Msg.GetScopesTruncated(), resp.Msg.GetScopesUnknown(), nil)
		}
		return renderJSON(cmd.OutOrStdout(), resp.Msg)
	},
}

func init() {
	addClientFlags(scheduledCmd)
	scopeRule, _ := surfaces.RuleByID(surfaces.RuleScopeRequiredUnlessCrossSpine)
	scheduledCmd.Flags().StringVar(&scheduledScope, "scope", "",
		scopeRule.Sentence+"; omit and pass --cross-spine to span every scope you can read; mutually exclusive with --cross-spine")
	scheduledCmd.Flags().BoolVar(&scheduledCrossSpine, "cross-spine", false,
		"span every scope you can read (still only your own records); mutually exclusive with --scope")
	scheduledCmd.Flags().StringVar(&scheduledState, "state", "", `which windowed records: "scheduled" (default: not yet active), "expired", or "all"`)
	scheduledCmd.Flags().Uint64Var(&scheduledLimit, "limit", 0, "max results per page; 0 resolves to the default, 20; a value above the maximum, 1000, is rejected")
	scheduledCmd.Flags().StringVar(&scheduledCreatedAfter, "created-after", "", "RFC3339 inclusive lower bound on created_at")
	scheduledCmd.Flags().StringVar(&scheduledCreatedBefore, "created-before", "", "RFC3339 exclusive upper bound on created_at")
	scheduledCmd.Flags().StringVar(&scheduledCursor, "cursor", "", "opaque cursor from a previous response's next_page_token")
	scheduledCmd.MarkFlagsMutuallyExclusive("scope", "cross-spine")
	rootCmd.AddCommand(scheduledCmd)
}
