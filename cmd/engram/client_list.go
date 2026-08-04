// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

var (
	listScope         string
	listCrossSpine    bool
	listLimit         uint64
	listOffset        uint64
	listCategories    []string
	listVisibility    string
	listTags          []string
	listFull          bool
	listCreatedAfter  string
	listCreatedBefore string
	listPageToken     string
	listCursorMode    bool
)

// listCmd is `engram list`, the second of the three D-01 subcommands.
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List memories on a remote engram server",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		// D-01/D-02/D-04 pre-flight guard, fired before any dialing.
		if err := requireScopeUnlessCrossSpine(listScope, listCrossSpine); err != nil {
			return err
		}
		// Timeout is resolved here but not yet applied to the RPC context —
		// plan 01-08 wires it into context.WithTimeout; this plan only
		// makes it exist and be validated (D-05).
		client, format, _, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		resp, err := client.ListMemories(cmd.Context(), connect.NewRequest(&engramv1.ListMemoriesRequest{
			Scope:         listScope,
			CrossSpine:    listCrossSpine,
			Limit:         listLimit,
			Offset:        listOffset,
			Categories:    listCategories,
			Visibility:    listVisibility,
			Tags:          listTags,
			Full:          listFull,
			CreatedAfter:  listCreatedAfter,
			CreatedBefore: listCreatedBefore,
			PageToken:     listPageToken,
			CursorMode:    listCursorMode,
		}))
		if err != nil {
			// Do not retry. Return wrapRPCError(err) and let Execute() map
			// it to a process exit code (D-09/D-10).
			return wrapRPCError(err)
		}
		// D-12: return nil regardless of how many memories came back — an
		// empty result set is a legitimate answer, not a failure.
		if format == formatText {
			if err := renderMemoryTable(cmd.OutOrStdout(), resp.Msg.GetMemories(), false); err != nil {
				return err
			}
			// The footer is data (total, next page token), not a
			// diagnostic, so it goes to stdout alongside the table.
			if resp.Msg.GetNextPageToken() != "" {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "total: %d  next_page_token: %s\n",
					resp.Msg.GetTotal(), resp.Msg.GetNextPageToken())
			} else {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "total: %d\n", resp.Msg.GetTotal())
			}
			if err != nil {
				return err
			}
			return renderCoverageFooter(cmd.OutOrStdout(), listCrossSpine, resp.Msg.GetSearchedScopes(), resp.Msg.GetScopesTruncated())
		}
		return renderJSON(cmd.OutOrStdout(), resp.Msg)
	},
}

func init() {
	addClientFlags(listCmd)
	listCmd.Flags().StringVar(&listScope, "scope", "",
		"limit recall to one scope; omit and pass --cross-spine to span every scope you can read; mutually exclusive with --cross-spine")
	listCmd.Flags().BoolVar(&listCrossSpine, "cross-spine", false,
		"span every scope you can read; mutually exclusive with --scope")
	listCmd.Flags().Uint64Var(&listLimit, "limit", 0, "max results (0 = server default)")
	listCmd.Flags().Uint64Var(&listOffset, "offset", 0, "offset-for-UI paging; mutually exclusive with --cursor-mode and --page-token")
	listCmd.Flags().StringSliceVar(&listCategories, "categories", nil, "category filter (empty = all categories)")
	listCmd.Flags().StringVar(&listVisibility, "visibility", "", `visibility filter: "" (all), "private", or "shared"`)
	listCmd.Flags().StringSliceVar(&listTags, "tags", nil, "tag filter (records must carry ALL listed tags)")
	listCmd.Flags().BoolVar(&listFull, "full", false, "return full content instead of summaries")
	listCmd.Flags().StringVar(&listCreatedAfter, "created-after", "", "RFC3339 inclusive lower bound on created_at")
	listCmd.Flags().StringVar(&listCreatedBefore, "created-before", "", "RFC3339 exclusive upper bound on created_at")
	listCmd.Flags().StringVar(&listPageToken, "page-token", "", "opaque cursor from a previous response's next_page_token; mutually exclusive with --offset and --cursor-mode")
	listCmd.Flags().BoolVar(&listCursorMode, "cursor-mode", false, "opt into cursor paging on the first (tokenless) page; mutually exclusive with --offset and --page-token")
	// D-07/D-08: the paging trio is mutually exclusive as a declared cobra
	// flag group, validated centrally by rootCmd.PersistentPreRunE calling
	// cmd.ValidateFlagGroups() before RunE ever dials. cobra's flag groups
	// count a *supplied* flag, not its value, so --offset 0 --page-token ""
	// is rejected too (D-08's widened blast radius) — this is what closes
	// the previously-unenforced trio without a fourth hand-rolled guard.
	listCmd.MarkFlagsMutuallyExclusive("offset", "cursor-mode", "page-token")
	// D-07: the second of the three exclusivity claim sites, shared with
	// searchCmd's declaration above — MarkFlagsMutuallyExclusive is a
	// per-Command method, so the group must be declared here too, not just
	// once on searchCmd. requireScopeUnlessCrossSpine (client_common.go)
	// still enforces the surviving asymmetric half.
	listCmd.MarkFlagsMutuallyExclusive("scope", "cross-spine")
	rootCmd.AddCommand(listCmd)
}
