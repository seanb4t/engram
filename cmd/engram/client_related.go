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
	relatedK    uint64
	relatedFull bool
)

// relatedCmd is `engram related <id>`, the client-tier verb over the
// RelatedMemories RPC: one record's neighbourhood — supersession chain,
// shared tags, shared citations and vector neighbours, each edge typed
// with its evidence — the same read the related_memories MCP tool makes,
// with no CLI-side filter of its own. It takes exactly one positional id
// (full UUID or short_id), as engram get does.
var relatedCmd = &cobra.Command{
	Use:   "related <id>",
	Short: "Show one memory's neighbourhood: supersession, shared tags, shared citations and vector neighbours",
	Args:  exactlyOneID,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, format, timeout, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		resp, err := client.RelatedMemories(ctx, connect.NewRequest(&engramv1.RelatedMemoriesRequest{
			Id:   args[0],
			K:    relatedK,
			Full: relatedFull,
		}))
		if err != nil {
			// One call, no retry; Execute() maps the Connect code to the
			// exit code. A not-found anchor echoes only the caller's input.
			return wrapRPCError(err)
		}
		if format == formatText {
			headline := fmt.Sprintf("related to %s: %d record(s), truncated: %t",
				resp.Msg.GetAnchor().GetShortId(), len(resp.Msg.GetRelated()), resp.Msg.GetTruncated())
			return renderProtoView(cmd.OutOrStdout(), headline, resp.Msg)
		}
		return renderJSON(cmd.OutOrStdout(), resp.Msg)
	},
}

func init() {
	addClientFlags(relatedCmd)
	relatedCmd.Flags().Uint64Var(&relatedK, "k", 0, "max vector neighbours; 0 resolves to the default, 8; a value above the maximum, 1000, is rejected")
	relatedCmd.Flags().BoolVar(&relatedFull, "full", false, "return full content for the anchor and every related record instead of summaries")
	rootCmd.AddCommand(relatedCmd)
}
