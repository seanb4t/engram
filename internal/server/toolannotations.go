// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/seanb4t/engram/internal/surfaces"
)

// annotationsFor builds the mcp.ToolAnnotations for the MCP tool named
// name, reading its blast-radius Class from the single shared
// internal/surfaces table (D-10). It returns nil — never a zero-valued
// &mcp.ToolAnnotations{} — when name has no table entry, so a missing row
// surfaces as a nil Annotations the both-directions gate
// (toolannotations_test.go) catches, rather than as a silently
// zero-valued (and therefore falsely "safe-looking") struct.
//
// go-sdk v1.7.0's mcp.ToolAnnotations models DestructiveHint/OpenWorldHint
// as *bool (nil defaults to the spec's true) while ReadOnlyHint/
// IdempotentHint are bare bool — a literal `false` does not compile for
// the pointer fields, hence boolPtr.
func annotationsFor(name string) *mcp.ToolAnnotations {
	class, ok := surfaces.ClassForTool(name)
	if !ok {
		return nil
	}
	return &mcp.ToolAnnotations{
		ReadOnlyHint:    class.ReadOnly,
		DestructiveHint: boolPtr(class.Destructive),
		IdempotentHint:  class.Idempotent,
		OpenWorldHint:   boolPtr(class.OpenWorld),
	}
}

// boolPtr allocates a fresh bool and returns its address. Deliberately NOT
// a shared package-level boolean variable whose address is handed out to
// every caller: a caller mutating through a shared pointer would corrupt
// every other tool's hint that happened to share it.
func boolPtr(v bool) *bool {
	return &v
}
