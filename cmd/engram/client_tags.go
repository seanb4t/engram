// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"text/tabwriter"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

var (
	tagsScope string
	tagsLimit uint64
)

// tagsCmd is `engram tags`, the client-tier verb over the ListTags RPC:
// exact tag counts over the caller's recall-visible records in one scope,
// or across every readable scope when --scope is omitted, most-used first,
// with `more` reporting a truncated set — the same read the list_tags MCP
// tool makes.
var tagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "List tag counts for a scope, or every readable scope, most-used first",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, format, timeout, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		resp, err := client.ListTags(ctx, connect.NewRequest(&engramv1.ListTagsRequest{
			Scope: tagsScope,
			Limit: tagsLimit,
		}))
		if err != nil {
			return wrapRPCError(err)
		}
		if format == formatText {
			if err := renderTagTable(cmd, resp.Msg.GetTags()); err != nil {
				return err
			}
			// The footer is data, not a diagnostic: it names the proto
			// field verbatim, so both lanes call the fact the same thing.
			if resp.Msg.GetMore() {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "more: true")
				return err
			}
			return nil
		}
		return renderJSON(cmd.OutOrStdout(), resp.Msg)
	},
}

// renderTagTable writes tags as a TAG/COUNT table through text/tabwriter,
// in the server's order (count descending, then tag ascending).
func renderTagTable(cmd *cobra.Command, tags []*engramv1.TagCount) error {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintf(tw, "TAG\tCOUNT\n"); err != nil {
		return err
	}
	for _, tc := range tags {
		if _, err := fmt.Fprintf(tw, "%s\t%d\n", tc.GetTag(), tc.GetCount()); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func init() {
	addClientFlags(tagsCmd)
	tagsCmd.Flags().StringVar(&tagsScope, "scope", "", "scope to count tags in; omit to span every scope you can read")
	tagsCmd.Flags().Uint64Var(&tagsLimit, "limit", 0, "max tags; 0 resolves to the default, 100; a value above the maximum, 1000, is rejected")
	rootCmd.AddCommand(tagsCmd)
}
