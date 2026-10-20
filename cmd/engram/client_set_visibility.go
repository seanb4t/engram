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
	setVisibilityShared  bool
	setVisibilityPrivate bool
)

// setVisibilityCmd is `engram set-visibility <id>`, the client-tier verb
// over the SetVisibility RPC: the same toggle the set_visibility MCP tool
// makes. Exactly one of --shared / --private is required; the pair is a
// mutually-exclusive cobra group and the at-least-one half is checked
// here, before any request, so neither direction is ever implied.
var setVisibilityCmd = &cobra.Command{
	Use:   "set-visibility <id>",
	Short: "Share or unshare a memory you own",
	Args:  exactPositionalArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !setVisibilityShared && !setVisibilityPrivate {
			return usageErrorf("exactly one of --shared or --private is required")
		}
		visibility := engramv1.Visibility_VISIBILITY_PRIVATE
		if setVisibilityShared {
			visibility = engramv1.Visibility_VISIBILITY_SHARED
		}
		client, format, timeout, err := clientFromFlags(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		resp, err := client.SetVisibility(ctx, connect.NewRequest(&engramv1.SetVisibilityRequest{
			Id:         args[0],
			Visibility: visibility,
		}))
		if err != nil {
			return wrapRPCError(err)
		}
		if format == formatText {
			return renderIDLine(cmd.OutOrStdout(), "visibility set", resp.Msg.GetShortId(), resp.Msg.GetId())
		}
		return renderJSON(cmd.OutOrStdout(), resp.Msg)
	},
}

func init() {
	addClientFlags(setVisibilityCmd)
	setVisibilityCmd.Flags().BoolVar(&setVisibilityShared, "shared", false, "make the record readable by every authenticated caller; mutually exclusive with --private")
	setVisibilityCmd.Flags().BoolVar(&setVisibilityPrivate, "private", false, "make the record private again; rejected for a rule (delete the rule instead); mutually exclusive with --shared")
	setVisibilityCmd.MarkFlagsMutuallyExclusive("shared", "private")
	rootCmd.AddCommand(setVisibilityCmd)
}
