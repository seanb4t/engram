// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package surfaces

import "testing"

// TestValidateOperations proves the live registry passes its own
// well-formedness gate: no duplicate MCPTool, no duplicate CLICommand, and
// no row with both key columns empty.
func TestValidateOperations(t *testing.T) {
	if err := ValidateOperations(); err != nil {
		t.Fatalf("ValidateOperations() = %v, want nil", err)
	}
}

// TestValidateOperationsCatchesDuplicateTool proves the gate fails on a
// duplicate MCPTool.
func TestValidateOperationsCatchesDuplicateTool(t *testing.T) {
	set := []Operation{
		{MCPTool: "store_memory", Class: Class{}},
		{MCPTool: "store_memory", Class: Class{}},
	}
	if err := validateOperationSet(set); err == nil {
		t.Fatal("validateOperationSet() = nil, want error on duplicate MCPTool")
	}
}

// TestValidateOperationsCatchesDuplicateCommand proves the gate fails on a
// duplicate CLICommand.
func TestValidateOperationsCatchesDuplicateCommand(t *testing.T) {
	set := []Operation{
		{CLICommand: "search", Class: Class{}},
		{CLICommand: "search", Class: Class{}},
	}
	if err := validateOperationSet(set); err == nil {
		t.Fatal("validateOperationSet() = nil, want error on duplicate CLICommand")
	}
}

// TestValidateOperationsCatchesBothEmpty proves the gate fails on a row
// naming no operation on either lane.
func TestValidateOperationsCatchesBothEmpty(t *testing.T) {
	set := []Operation{
		{MCPTool: "", CLICommand: "", Class: Class{}},
	}
	if err := validateOperationSet(set); err == nil {
		t.Fatal("validateOperationSet() = nil, want error on a row with both key columns empty")
	}
}

// TestOperationsCoverEveryTool proves the registry carries exactly one
// entry per registered MCP tool — 15, matching
// registertools_test.go's wantRegisteredToolNames inventory — with no
// duplicate and no missing tool.
func TestOperationsCoverEveryTool(t *testing.T) {
	const wantToolCount = 15

	got := 0
	for _, op := range Operations() {
		if op.MCPTool != "" {
			got++
		}
	}
	if got != wantToolCount {
		t.Errorf("distinct non-empty MCPTool count = %d, want %d", got, wantToolCount)
	}
}

// TestConservativeStanceWorkedExamples spot-checks the three tools
// CONTEXT.md's D-09 names by hand: store_memory/schedule_memory are not
// idempotent (the idempotency key is optional, so an unkeyed repeat
// duplicates); update_memory is destructive (replaces content in place);
// supersede_memory is not destructive (additive under all inputs — history
// preserved, target soft-hidden, never deleted).
func TestConservativeStanceWorkedExamples(t *testing.T) {
	cases := []struct {
		tool string
		want Class
		desc string
	}{
		{"store_memory", Class{Idempotent: false}, "idempotency key is optional"},
		{"schedule_memory", Class{Idempotent: false}, "idempotency key is optional"},
		{"update_memory", Class{Destructive: true}, "replaces content in place"},
		{"supersede_memory", Class{Destructive: false}, "additive, history preserved"},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			got, ok := ClassForTool(tc.tool)
			if !ok {
				t.Fatalf("ClassForTool(%q) not found", tc.tool)
			}
			switch tc.tool {
			case "store_memory", "schedule_memory":
				if got.Idempotent != tc.want.Idempotent {
					t.Errorf("%s.Idempotent = %v, want %v (%s)", tc.tool, got.Idempotent, tc.want.Idempotent, tc.desc)
				}
			case "update_memory", "supersede_memory":
				if got.Destructive != tc.want.Destructive {
					t.Errorf("%s.Destructive = %v, want %v (%s)", tc.tool, got.Destructive, tc.want.Destructive, tc.desc)
				}
			}
		})
	}
}

// TestEveryOperationIsClosedWorld proves D-10's "openWorldHint set
// explicitly to false, never omitted" requirement holds across the WHOLE
// table with no exception — a nil-defaults-to-true *bool on the wire would
// otherwise silently advertise the opposite of the truth for this closed
// memory domain.
func TestEveryOperationIsClosedWorld(t *testing.T) {
	for _, op := range Operations() {
		if op.Class.OpenWorld {
			t.Errorf("operation MCPTool=%q CLICommand=%q has OpenWorld=true, want false on every row", op.MCPTool, op.CLICommand)
		}
	}
}

// TestClassForCommandKnowsCLIOnlyOperations spot-checks a couple of the
// CLI-only rows (no MCP tool counterpart) resolve by their cobra Use name.
func TestClassForCommandKnowsCLIOnlyOperations(t *testing.T) {
	cases := []struct {
		command         string
		wantDestructive bool
		wantReadOnly    bool
	}{
		{"prune-expired", true, false},
		{"migrate-remap-owner", true, false},
		{"reindex", false, false},
		{"version", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			got, ok := ClassForCommand(tc.command)
			if !ok {
				t.Fatalf("ClassForCommand(%q) not found", tc.command)
			}
			if got.Destructive != tc.wantDestructive {
				t.Errorf("%s.Destructive = %v, want %v", tc.command, got.Destructive, tc.wantDestructive)
			}
			if got.ReadOnly != tc.wantReadOnly {
				t.Errorf("%s.ReadOnly = %v, want %v", tc.command, got.ReadOnly, tc.wantReadOnly)
			}
		})
	}
}
