// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

var deleteApply bool

// deleteDoc is the hand-declared document `engram delete` emits on both
// lanes: Deleted says which path ran, ID is the id the caller gave, and
// Memory is the fetched record on the preview path only (protojson, the
// same serialization engram get emits). Both lanes serialize this one
// value, so the text lane cannot show a field the json lane lacks.
type deleteDoc struct {
	Deleted bool            `json:"deleted"`
	ID      string          `json:"id"`
	Memory  json.RawMessage `json:"memory,omitempty"`
}

// deleteCmd is `engram delete <id>`, the client-tier verb over the
// DeleteMemory RPC, routed through registerDestructive: a bare invocation
// fetches the record with GetMemory and renders what WOULD be deleted,
// exiting 0 without deleting; --apply calls DeleteMemory exactly once. A
// record the caller cannot read exits 4 on both paths, so a non-owner
// learns nothing from the preview. This is the only destructive client
// verb; there is deliberately no verb that deletes every record.
var deleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete one memory you own: preview by default, delete under --apply",
	Args:  exactPositionalArgs(1),
}

func deletePreview(ctx context.Context, cmd *cobra.Command) error {
	id := cmd.Flags().Args()[0]
	client, format, timeout, err := clientFromFlags(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := client.GetMemory(ctx, connect.NewRequest(&engramv1.GetMemoryRequest{Id: id}))
	if err != nil {
		return wrapRPCError(err)
	}
	b, err := protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}.Marshal(resp.Msg.GetMemory())
	if err != nil {
		return err
	}
	doc := deleteDoc{Deleted: false, ID: id, Memory: b}
	if format == formatText {
		headline := fmt.Sprintf("would delete record %s; run again with --apply to delete it", resp.Msg.GetMemory().GetShortId())
		return renderOperatorView(cmd.OutOrStdout(), headline, doc)
	}
	return renderJSONDoc(cmd, doc)
}

func deleteApplyRun(ctx context.Context, cmd *cobra.Command) error {
	id := cmd.Flags().Args()[0]
	client, format, timeout, err := clientFromFlags(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if _, err := client.DeleteMemory(ctx, connect.NewRequest(&engramv1.DeleteMemoryRequest{Id: id})); err != nil {
		return wrapRPCError(err)
	}
	doc := deleteDoc{Deleted: true, ID: id}
	if format == formatText {
		return renderOperatorView(cmd.OutOrStdout(), "deleted "+id, doc)
	}
	return renderJSONDoc(cmd, doc)
}

// renderJSONDoc writes a hand-declared document as one JSON object plus a
// trailing newline, the same single-document contract renderJSON keeps for
// proto messages.
func renderJSONDoc(cmd *cobra.Command, doc any) error {
	b, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(b))
	return err
}

func init() {
	addClientFlags(deleteCmd)
	registerDestructive(deleteCmd, &deleteApply, deletePreview, deleteApplyRun)
	rootCmd.AddCommand(deleteCmd)
}
