// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

var (
	updateContent string
	updateSummary string
	updateTags    []string
	updateShared  bool
	updatePrivate bool
	updateApply   bool
)

// updateFieldFlags are the flags that each add their path to the update
// mask; at least one must be given.
var updateFieldFlags = []string{"content", "summary", "tags", "shared", "private"}

// updateDoc is the hand-declared document `engram update` emits on both
// lanes: Applied says which path ran, ID is the id the caller gave,
// UpdateMask is the set of fields the flags selected, ShortID is the
// server's handle after a write, and Memory is the fetched record on the
// preview path only (protojson, the same serialization engram get emits).
// Both lanes serialize this one value.
type updateDoc struct {
	Applied    bool            `json:"applied"`
	ID         string          `json:"id"`
	UpdateMask []string        `json:"update_mask"`
	ShortID    string          `json:"short_id,omitempty"`
	Memory     json.RawMessage `json:"memory,omitempty"`
}

// updateMaskFromFlags builds the update mask from the flags actually set,
// so an omitted flag leaves its field unchanged; `--tags ""` clears the tag
// set (an explicit empty value, not an omitted flag). --shared and
// --private are a mutually-exclusive cobra group and both map to the
// `shared` path. No field flag at all is rejected locally: the server's
// empty-mask rejection would be the same exit code one round trip later.
func updateMaskFromFlags(cmd *cobra.Command) ([]string, error) {
	var paths []string
	for _, name := range updateFieldFlags {
		if cmd.Flags().Changed(name) {
			path := name
			if name == "private" {
				path = "shared"
			}
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return nil, usageErrorf("at least one of --content, --summary, --tags, --shared or --private is required")
	}
	return paths, nil
}

// updateCmd is `engram update <id>`, the client-tier verb over the
// UpdateMemory RPC: an in-place replacement of the named fields, the same
// write the update_memory MCP tool makes. update_memory is classified
// destructive (it overwrites content in place), so like every destructive
// command this verb routes through registerDestructive: a bare invocation
// fetches the record and renders which fields WOULD change, exiting 0
// without writing; --apply calls UpdateMemory exactly once.
var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Replace a memory's content, summary, tags or visibility in place: preview by default, write under --apply",
	Args:  exactlyOneID,
}

func updatePreview(ctx context.Context, cmd *cobra.Command) error {
	id := cmd.Flags().Args()[0]
	paths, err := updateMaskFromFlags(cmd)
	if err != nil {
		return err
	}
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
	doc := updateDoc{Applied: false, ID: id, UpdateMask: paths, Memory: b}
	if format == formatText {
		headline := fmt.Sprintf("would update record %s (%s); run again with --apply to write it",
			resp.Msg.GetMemory().GetShortId(), strings.Join(paths, ", "))
		return renderOperatorView(cmd.OutOrStdout(), headline, doc)
	}
	return renderJSONDoc(cmd, doc)
}

func updateApplyRun(ctx context.Context, cmd *cobra.Command) error {
	id := cmd.Flags().Args()[0]
	paths, err := updateMaskFromFlags(cmd)
	if err != nil {
		return err
	}
	client, format, timeout, err := clientFromFlags(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := client.UpdateMemory(ctx, connect.NewRequest(&engramv1.UpdateMemoryRequest{
		Id:         id,
		Content:    updateContent,
		Summary:    updateSummary,
		Tags:       updateTags,
		Shared:     updateShared,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: paths},
	}))
	if err != nil {
		return wrapRPCError(err)
	}
	doc := updateDoc{Applied: true, ID: resp.Msg.GetId(), UpdateMask: paths, ShortID: resp.Msg.GetShortId()}
	if format == formatText {
		return renderOperatorView(cmd.OutOrStdout(), fmt.Sprintf("updated: %s (%s)", resp.Msg.GetShortId(), resp.Msg.GetId()), doc)
	}
	return renderJSONDoc(cmd, doc)
}

func init() {
	addClientFlags(updateCmd)
	updateCmd.Flags().StringVar(&updateContent, "content", "", "replace the content (re-embeds)")
	updateCmd.Flags().StringVar(&updateSummary, "summary", "", "replace the client-authored summary")
	updateCmd.Flags().StringSliceVar(&updateTags, "tags", nil, `replace the tag set (re-embeds); --tags "" clears it; omit to preserve it`)
	updateCmd.Flags().BoolVar(&updateShared, "shared", false, "make the record readable by every authenticated caller; mutually exclusive with --private")
	updateCmd.Flags().BoolVar(&updatePrivate, "private", false, "make the record private again; rejected for a rule (delete the rule instead); mutually exclusive with --shared")
	updateCmd.MarkFlagsMutuallyExclusive("shared", "private")
	registerDestructive(updateCmd, &updateApply, updatePreview, updateApplyRun)
	rootCmd.AddCommand(updateCmd)
}
