// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package surfaces

import (
	"fmt"
	"os"
	"os/exec"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// ProseSurfaceRegion is a thin wrapper over ReadRegion, named for its use by
// the docs-site, skill, and proto surface readers — every caller that just
// needs "what does rule ruleID's anchored region in path currently say".
func ProseSurfaceRegion(path, ruleID string) (string, bool, error) {
	return ReadRegion(path, ruleID)
}

// fileMessageTypeFieldNumber and messageFieldFieldNumber are the
// descriptor.proto field numbers SourceCodeInfo.Location.Path segments
// encode against: FileDescriptorProto.message_type is field 4,
// DescriptorProto.field is field 2. This is the standard, stable
// descriptor.proto path convention (unchanged since proto2), not an
// engram-specific number.
const (
	fileMessageTypeFieldNumber = 4
	messageFieldFieldNumber    = 2
)

// ProtoFieldComments unmarshals the FileDescriptorSet at fdsPath (produced
// by BufDescriptorSet) and walks each FileDescriptorProto's
// SourceCodeInfo.Location entries, matching each entry's Path against a
// top-level message field's declaration path ([4, messageIndex, 2,
// fieldIndex] — the descriptor.proto convention for
// FileDescriptorProto.message_type[i].field[j]) to recover that field's
// LeadingComments. The returned map is keyed "<message>.<field>".
//
// protoc-gen-go strips SourceCodeInfo from the descriptor it embeds in
// generated .pb.go files, so this cannot be read via protoreflect on
// generated code — only via buf's own FileDescriptorSet output, which
// includes source info by default.
func ProtoFieldComments(fdsPath string) (map[string]string, error) {
	data, err := os.ReadFile(fdsPath)
	if err != nil {
		return nil, fmt.Errorf("surfaces: read %s: %w", fdsPath, err)
	}
	var fds descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(data, &fds); err != nil {
		return nil, fmt.Errorf("surfaces: unmarshal FileDescriptorSet at %s: %w", fdsPath, err)
	}

	out := make(map[string]string)
	for _, f := range fds.GetFile() {
		messages := f.GetMessageType()
		for _, loc := range f.GetSourceCodeInfo().GetLocation() {
			comment := loc.GetLeadingComments()
			if comment == "" {
				continue
			}
			path := loc.GetPath()
			if len(path) != 4 || path[0] != fileMessageTypeFieldNumber || path[2] != messageFieldFieldNumber {
				continue
			}
			msgIdx, fieldIdx := int(path[1]), int(path[3])
			if msgIdx < 0 || msgIdx >= len(messages) {
				continue
			}
			fields := messages[msgIdx].GetField()
			if fieldIdx < 0 || fieldIdx >= len(fields) {
				continue
			}
			key := messages[msgIdx].GetName() + "." + fields[fieldIdx].GetName()
			out[key] = comment
		}
	}
	return out, nil
}

// BufDescriptorSet invokes `go tool buf build <protoDir> --as-file-descriptor-set
// -o <outPath>` via exec.Command with a fixed argument slice and no shell —
// no caller- or environment-derived value ever reaches a shell
// interpolation point (T-02-05). It returns the subprocess's combined
// output wrapped into the error on failure; a missing `go tool buf` is a
// loud failure here, never a silent skip, since the conformance gate's
// entire proto-comment surface depends on this succeeding.
func BufDescriptorSet(protoDir, outPath string) error {
	// #nosec G204 -- fixed argument list, no shell, no caller/environment
	// interpolation into the command line.
	cmd := exec.Command("go", "tool", "buf", "build", protoDir, "--as-file-descriptor-set", "-o", outPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("surfaces: go tool buf build %s: %w\n%s", protoDir, err, out)
	}
	return nil
}
