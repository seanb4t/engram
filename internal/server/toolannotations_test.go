// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"reflect"
	"testing"

	"github.com/seanb4t/engram/internal/surfaces"
)

// TestToolAnnotationsBothDirections is D-10's anti-drift gate, copying
// TestCatalogExitCodesMatchMapper's shape (cmd/engram/catalog_test.go):
// the set of REGISTERED tool names (read through registeredTools' real
// in-memory-transport ListTools round trip, never a hand-duplicated list)
// must equal, as a set, the set of non-empty MCPTool values
// surfaces.Operations() declares. reflect.DeepEqual on two independently
// built maps makes the comparison immune to ListTools' return order.
//
// A second assertion loop then proves every registered tool's Annotations
// is non-nil, its DestructiveHint/OpenWorldHint pointers are non-nil, and
// all four hint values equal surfaces.ClassForTool(tool.Name) — this is
// what catches a zero-valued &mcp.ToolAnnotations{} masquerading as an
// annotated tool, and a nil OpenWorldHint silently defaulting to the
// spec's true.
func TestToolAnnotationsBothDirections(t *testing.T) {
	tools := registeredTools(t)

	registered := make(map[string]bool, len(tools))
	for _, tool := range tools {
		registered[tool.Name] = true
	}

	declared := make(map[string]bool)
	for _, op := range surfaces.Operations() {
		if op.MCPTool != "" {
			declared[op.MCPTool] = true
		}
	}

	if !reflect.DeepEqual(registered, declared) {
		t.Errorf("registered tool names = %v, surfaces.Operations() MCPTool entries = %v",
			sortedNames(registered), sortedNames(declared))
	}

	for _, tool := range tools {
		if tool.Annotations == nil {
			t.Errorf("tool %q: Annotations is nil", tool.Name)
			continue
		}
		if tool.Annotations.DestructiveHint == nil {
			t.Errorf("tool %q: Annotations.DestructiveHint is nil", tool.Name)
		}
		if tool.Annotations.OpenWorldHint == nil {
			t.Errorf("tool %q: Annotations.OpenWorldHint is nil", tool.Name)
		} else if *tool.Annotations.OpenWorldHint {
			t.Errorf("tool %q: OpenWorldHint = true, want false on every tool (D-10)", tool.Name)
		}

		want, ok := surfaces.ClassForTool(tool.Name)
		if !ok {
			t.Errorf("tool %q: no surfaces.ClassForTool entry", tool.Name)
			continue
		}
		if tool.Annotations.ReadOnlyHint != want.ReadOnly {
			t.Errorf("tool %q: ReadOnlyHint = %v, want %v", tool.Name, tool.Annotations.ReadOnlyHint, want.ReadOnly)
		}
		if tool.Annotations.DestructiveHint != nil && *tool.Annotations.DestructiveHint != want.Destructive {
			t.Errorf("tool %q: DestructiveHint = %v, want %v", tool.Name, *tool.Annotations.DestructiveHint, want.Destructive)
		}
		if tool.Annotations.IdempotentHint != want.Idempotent {
			t.Errorf("tool %q: IdempotentHint = %v, want %v", tool.Name, tool.Annotations.IdempotentHint, want.Idempotent)
		}
	}
}

// TestAnnotationsForMissingEntryReturnsNil proves annotationsFor returns
// nil — never a zero-valued &mcp.ToolAnnotations{} — when a name has no
// surfaces.ClassForTool entry, so a missing table row surfaces as a nil
// Annotations the both-directions gate above catches.
func TestAnnotationsForMissingEntryReturnsNil(t *testing.T) {
	if got := annotationsFor("no_such_tool"); got != nil {
		t.Errorf("annotationsFor(%q) = %+v, want nil", "no_such_tool", got)
	}
}
