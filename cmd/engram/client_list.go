// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"os"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

var (
	listScope         string
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
		format, err := resolveOutputFormat(clientOutput, isTerminal(os.Stdout))
		if err != nil {
			return err
		}
		client, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		resp, err := client.ListMemories(cmd.Context(), connect.NewRequest(&engramv1.ListMemoriesRequest{
			Scope:         listScope,
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
			return err
		}
		return renderJSON(cmd.OutOrStdout(), resp.Msg)
	},
}

func init() {
	addClientFlags(listCmd)
	listCmd.Flags().StringVar(&listScope, "scope", "", "scope filter")
	listCmd.Flags().Uint64Var(&listLimit, "limit", 0, "max results (0 = server default)")
	listCmd.Flags().Uint64Var(&listOffset, "offset", 0, "offset-for-UI paging; mutually exclusive with --cursor-mode")
	listCmd.Flags().StringSliceVar(&listCategories, "categories", nil, "category filter (empty = all categories)")
	listCmd.Flags().StringVar(&listVisibility, "visibility", "", `visibility filter: "" (all), "private", or "shared"`)
	listCmd.Flags().StringSliceVar(&listTags, "tags", nil, "tag filter (records must carry ALL listed tags)")
	listCmd.Flags().BoolVar(&listFull, "full", false, "return full content instead of summaries")
	listCmd.Flags().StringVar(&listCreatedAfter, "created-after", "", "RFC3339 inclusive lower bound on created_at")
	listCmd.Flags().StringVar(&listCreatedBefore, "created-before", "", "RFC3339 exclusive upper bound on created_at")
	listCmd.Flags().StringVar(&listPageToken, "page-token", "", "opaque cursor from a previous response's next_page_token; when set, cursor paging (ignores --offset)")
	listCmd.Flags().BoolVar(&listCursorMode, "cursor-mode", false, "opt into cursor paging on the first (tokenless) page; mutually exclusive with a non-zero --offset")
	rootCmd.AddCommand(listCmd)
}
