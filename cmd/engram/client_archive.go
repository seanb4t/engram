// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// archiveCmd and restoreCmd are `engram archive <id>...` and `engram
// restore <id>...`, the client-tier verbs over the ArchiveMemory and
// RestoreMemory RPCs: the same batch the archive_memory / restore_memory
// MCP tools run, one outcome per id in the caller's order, with a record
// the caller does not own reading not_found and never echoing its UUID.
// They are distinct from the operator-tier `spine-review archive` /
// `spine-review restore`, which talk to Qdrant directly and stay
// unchanged.
var archiveCmd = &cobra.Command{
	Use:   "archive <id>...",
	Short: "Archive one or more memories you own (reversible with restore)",
	Args:  minPositionalArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, format, timeout, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		resp, err := client.ArchiveMemory(ctx, connect.NewRequest(&engramv1.ArchiveMemoryRequest{Ids: args}))
		if err != nil {
			return wrapRPCError(err)
		}
		return renderBatchOutcomes(cmd, format, "archive", len(resp.Msg.GetResults()), resp.Msg)
	},
}

var restoreCmd = &cobra.Command{
	Use:   "restore <id>...",
	Short: "Restore one or more archived memories you own",
	Args:  minPositionalArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, format, timeout, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		resp, err := client.RestoreMemory(ctx, connect.NewRequest(&engramv1.RestoreMemoryRequest{Ids: args}))
		if err != nil {
			return wrapRPCError(err)
		}
		return renderBatchOutcomes(cmd, format, "restore", len(resp.Msg.GetResults()), resp.Msg)
	},
}

// renderBatchOutcomes renders a per-id batch response: the text lane walks
// the response through the operator view (one row per id, in the caller's
// order, carrying requested/id/outcome) under a headline that counts the
// rows; the json lane emits the response verbatim.
func renderBatchOutcomes(cmd *cobra.Command, format outputFormat, verb string, n int, msg proto.Message) error {
	if format == formatText {
		return renderProtoView(cmd.OutOrStdout(), fmt.Sprintf("%s: %d id(s)", verb, n), msg)
	}
	return renderJSON(cmd.OutOrStdout(), msg)
}

func init() {
	addClientFlags(archiveCmd)
	addClientFlags(restoreCmd)
	rootCmd.AddCommand(archiveCmd, restoreCmd)
}
