// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package surfaces

import "fmt"

// Class is one operation's blast-radius classification: the four MCP
// ToolAnnotations hints, expressed as plain bools. go-sdk v1.7.0's
// mcp.ToolAnnotations models DestructiveHint/OpenWorldHint as *bool
// (nil defaults to the spec's true) and ReadOnlyHint/IdempotentHint as
// bare bool — that pointer-vs-value wire shape is a translation concern
// for internal/server's annotationsFor, not a taxonomy concern for this
// stdlib-only leaf package, so every field here is a plain bool.
type Class struct {
	// ReadOnly is true when the operation never modifies environment
	// state.
	ReadOnly bool
	// Destructive is true only if the operation may remove or overwrite
	// existing data under some valid invocation, rather than purely
	// adding to it — D-09's conservative stance: false only when EVERY
	// valid invocation is purely additive. Meaningful only when ReadOnly
	// is false.
	Destructive bool
	// Idempotent is true only if calling the operation repeatedly with
	// the same arguments has no additional effect on the environment
	// under EVERY valid invocation (D-09).
	Idempotent bool
	// OpenWorld is always false across this table: engram is a closed
	// memory domain, never an "open world of external entities" (D-10).
	// go-sdk's OpenWorldHint defaults to true when unset (nil), so this
	// field exists to be explicitly written, never omitted.
	OpenWorld bool
}

// Operation ties one blast-radius Class to its name on each of the two
// lanes that read this table: the MCP tool name and the CLI command name.
// Both key columns coexist because D-11 requires one taxonomy across two
// lanes whose names do not coincide (MCP registers search_memory, the CLI
// registers search) and because several operations exist on only one
// lane: schedule_memory has no CLI verb; the operator commands
// (reindex, migrate-remap-owner, prune-expired, summarize-missing,
// backfill-short-ids) and the bare self-describe invocation have no MCP
// tool at all. An empty string in either column means "this operation has
// no counterpart on that lane" — never "unclassified": every declared row
// carries a real, deliberately-chosen Class.
type Operation struct {
	MCPTool string
	// CLICommand is the command's qualified path relative to the binary
	// (D-01): a top-level command's bare name ("reindex"), or a nested
	// leaf's space-joined path ("spine-review scan") — never a bare cobra
	// Use name for a nested command, which would collide with any other
	// leaf sharing that name under a different group.
	CLICommand string
	Class      Class
}

// operations is the declared registry — the single source both
// internal/server's mcp.ToolAnnotations wiring and (a later plan's)
// engram catalog blast-radius column compose from. Every row applies
// D-09's conservative stance: a hint is true only if it holds under every
// valid invocation. The three worked examples CONTEXT.md names by hand —
// store_memory/schedule_memory not idempotent, update_memory destructive,
// supersede_memory not destructive — are spot-checked in toolclass_test.go.
var operations = []Operation{
	{
		MCPTool: "store_memory", CLICommand: "store",
		// idempotency_key is OPTIONAL: an unkeyed repeat creates a second,
		// distinct record, so Idempotent cannot be true under every valid
		// invocation.
		Class: Class{ReadOnly: false, Destructive: false, Idempotent: false, OpenWorld: false},
	},
	{
		MCPTool: "schedule_memory", CLICommand: "",
		// Same idempotency_key-is-optional reasoning as store_memory.
		Class: Class{ReadOnly: false, Destructive: false, Idempotent: false, OpenWorld: false},
	},
	{
		MCPTool: "search_memory", CLICommand: "search",
		Class: Class{ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false},
	},
	{
		MCPTool: "list_memory", CLICommand: "list",
		Class: Class{ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false},
	},
	{
		MCPTool: "list_scheduled", CLICommand: "",
		Class: Class{ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false},
	},
	{
		MCPTool: "get_memory", CLICommand: "",
		Class: Class{ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false},
	},
	{
		MCPTool: "update_memory", CLICommand: "",
		// Destructive: replaces content in place (CONTEXT.md's worked
		// example). Idempotent: repeating the SAME replace call twice
		// leaves the record in the same end state both times.
		Class: Class{ReadOnly: false, Destructive: true, Idempotent: true, OpenWorld: false},
	},
	{
		MCPTool: "delete_memory", CLICommand: "",
		// Destructive: a genuine, permanent removal — unlike
		// supersede_memory, no history is preserved. Idempotent: repeating
		// a delete against an already-removed id has no ADDITIONAL effect
		// on the environment (REST DELETE's idempotency contract), even
		// though the repeat itself is rejected as not-found.
		Class: Class{ReadOnly: false, Destructive: true, Idempotent: true, OpenWorld: false},
	},
	{
		MCPTool: "delete_all", CLICommand: "",
		// Same REST-DELETE-style idempotency reasoning as delete_memory,
		// applied to a whole scope: once cleared, repeating removes
		// nothing further.
		Class: Class{ReadOnly: false, Destructive: true, Idempotent: true, OpenWorld: false},
	},
	{
		MCPTool: "store_discovery", CLICommand: "",
		// No idempotency_key exists on this tool at all: the omit-id path
		// always creates a new record, even called twice identically, so
		// Idempotent cannot be true under every valid invocation.
		Class: Class{ReadOnly: false, Destructive: false, Idempotent: false, OpenWorld: false},
	},
	{
		MCPTool: "search_discovery", CLICommand: "",
		Class: Class{ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false},
	},
	{
		MCPTool: "set_visibility", CLICommand: "",
		// Destructive: false — unlike update_memory, this only flips a
		// boolean visibility flag; content, tags, and the vector are
		// untouched, and the change is always reversible by calling again.
		// Idempotent: repeating with the same `shared` value leaves state
		// unchanged.
		Class: Class{ReadOnly: false, Destructive: false, Idempotent: true, OpenWorld: false},
	},
	{
		MCPTool: "supersede_memory", CLICommand: "",
		// Destructive: false (CONTEXT.md's worked example) — additive
		// under all inputs, history preserved, target soft-hidden never
		// deleted. Idempotent: true — NOT via an idempotency_key (this
		// verb explicitly supports none), but structurally: the "single
		// live head" invariant rejects a second call targeting an
		// already-superseded record, so an identical repeat creates no
		// second record.
		Class: Class{ReadOnly: false, Destructive: false, Idempotent: true, OpenWorld: false},
	},
	{
		MCPTool: "store_rule", CLICommand: "",
		// Same no-idempotency_key reasoning as store_discovery: the
		// omit-id path always creates a new rule record.
		Class: Class{ReadOnly: false, Destructive: false, Idempotent: false, OpenWorld: false},
	},
	{
		MCPTool: "list_rules", CLICommand: "",
		Class: Class{ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false},
	},

	// CLI-only operations: no MCP tool exists for any of these, so
	// MCPTool is deliberately empty. Classified by the same conservative
	// stance as the 15 MCP rows above.
	{
		MCPTool: "", CLICommand: "reindex",
		// Additive: re-embeds into a NEW (new-dimension) collection,
		// leaving the source collection untouched. Idempotent: the
		// command itself documents skipping "points already present in
		// the target with identical content (cheap restart of an
		// interrupted reindex)".
		Class: Class{ReadOnly: false, Destructive: false, Idempotent: true, OpenWorld: false},
	},
	{
		MCPTool: "", CLICommand: "migrate-remap-owner",
		// Destructive: overwrites the owner field on existing records in
		// place (the old owner value is not retained). Idempotent:
		// re-running finds no records still matching the stale --from
		// value once they have been remapped to --to.
		Class: Class{ReadOnly: false, Destructive: true, Idempotent: true, OpenWorld: false},
	},
	{
		MCPTool: "", CLICommand: "prune-expired",
		// Destructive: permanent deletion of lapsed records. Idempotent:
		// already-pruned records are not found again; a repeat removes
		// nothing further.
		Class: Class{ReadOnly: false, Destructive: true, Idempotent: true, OpenWorld: false},
	},
	{
		MCPTool: "", CLICommand: "summarize-missing",
		// Additive: only fills an EMPTY summary field; the command
		// documents that "records that already have a summary ... are
		// skipped", so an existing summary is never overwritten.
		// Idempotent for the same reason.
		Class: Class{ReadOnly: false, Destructive: false, Idempotent: true, OpenWorld: false},
	},
	{
		MCPTool: "", CLICommand: "backfill-short-ids",
		// Additive: "Assign a short_id to every memory that LACKS one" —
		// an already-backfilled record's short_id is never touched.
		// Idempotent for the same reason.
		Class: Class{ReadOnly: false, Destructive: false, Idempotent: true, OpenWorld: false},
	},
	{
		MCPTool: "", CLICommand: "version",
		Class: Class{ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false},
	},
	{
		// Found missing during plan 02-05: buildCatalog emits a "serve" entry
		// (rootCmd.AddCommand(serveCmd, versionCmd), root.go) that this table
		// never classified, which the both-directions catalog gate
		// (TestCatalogBlastRadiusMatchesToolClasses) turns into a hard
		// failure rather than a silently-omitted classification.
		// ReadOnly: false — serving ensures the configured Qdrant collection
		// exists (server.StoreFromEnv/ensureStoreFromConfig may CREATE it),
		// and every mutating MCP/Connect call an agent makes for the
		// server's whole lifetime runs through this command; it is not a
		// pure read. Destructive: false — starting the listener does not
		// itself remove or overwrite existing data (each request it serves
		// carries its own, separately classified, blast radius). Idempotent:
		// false under D-09's "every valid invocation" test — running it a
		// second time with the same --listen-addr does not leave the
		// environment unchanged; it fails to bind the already-held port, a
		// genuine additional effect, not a no-op repeat.
		MCPTool: "", CLICommand: "serve",
		Class: Class{ReadOnly: false, Destructive: false, Idempotent: false, OpenWorld: false},
	},
	{
		// Same missing-row discovery as "serve" above: migrateSetOwnerCmd
		// (migrate.go) is a live, non-Hidden rootCmd.AddCommand registration
		// — cobra's Deprecated field only prints a warning, it does not hide
		// the command from root.Commands() or from buildCatalog's traversal.
		// migrate-set-owner is the deprecated alias for
		// `migrate-remap-owner --from-missing --to <owner>`: unlike
		// migrate-remap-owner's general --from/--from-anon forms (which CAN
		// overwrite an existing non-empty owner and are Destructive: true),
		// this alias's own doc comment states it is scoped to "memory
		// records written before per-actor isolation (which carry no owner
		// key)" and is "One-time, idempotent" — it only ever fills an EMPTY
		// owner field, the same additive reasoning as summarize-missing and
		// backfill-short-ids above.
		MCPTool: "", CLICommand: "migrate-set-owner",
		Class: Class{ReadOnly: false, Destructive: false, Idempotent: true, OpenWorld: false},
	},
	{
		// The bare `engram` self-describe invocation (no subcommand):
		// prints the catalog JSON and exits. CLICommand is the root
		// command's own Use ("engram") rather than empty, since
		// ValidateOperations rejects a row with both key columns empty.
		MCPTool: "", CLICommand: "engram",
		Class: Class{ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false},
	},
	{
		// spine-review is the operator tier's first nested cobra group
		// (D-01) — a bare `engram spine-review` invocation runs nothing but
		// its own help (no RunE), the same read-only, non-destructive,
		// idempotent, closed-world shape as the bare root invocation above.
		MCPTool: "", CLICommand: "spine-review",
		Class: Class{ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false},
	},
	{
		// spine-review scan reports an inventory of the memory spine —
		// counts, scopes and categories only, never a mutating Qdrant RPC
		// (T-03-07/T-03-24's mitigation). Idempotent: repeating the same
		// scan against unchanged data reports the same counts.
		MCPTool: "", CLICommand: "spine-review scan",
		Class: Class{ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false},
	},
	{
		// spine-review verify classifies every stored citation into one of
		// four tiers — reads citations and the local filesystem, never a
		// mutating Qdrant RPC. Idempotent: repeating the same verify run
		// against unchanged data and an unchanged working tree reports the
		// same tier counts.
		MCPTool: "", CLICommand: "spine-review verify",
		Class: Class{ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false},
	},
	{
		// spine-review consolidate reports ranked near-duplicate candidate
		// pairs using each record's already-stored vector — never a
		// mutating Qdrant RPC, never a merge, never a mutation on any path
		// (T-03-16's mitigation). Idempotent: repeating the same sweep
		// against unchanged data reports the same ranked pairs.
		MCPTool: "", CLICommand: "spine-review consolidate",
		Class: Class{ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false},
	},
	{
		// spine-review archive sets ONE server-owned state key (archived_at)
		// via a targeted SetPayload. This is a RECORDED DECISION, not the
		// comfortable-but-false claim that this verb "removes nothing":
		// restore (the sibling row below) issues a real Qdrant DeletePayload
		// RPC for that same key (internal/store/store.go's
		// defaultDeletePayloadKeys). Destructive: false rests on the SAME
		// precedent set_visibility already occupies above in this table —
		// that row is Destructive: false while OVERWRITING a payload field,
		// on the recorded grounds that "content, tags, and the vector are
		// untouched, and the change is always reversible by calling again".
		// archive/restore satisfy that identical test: neither reads, writes
		// or erases content, tags or the vector, and each is EXACTLY
		// reversible by the other verb. This table's operative meaning is
		// therefore "removes or overwrites the record's CONTENT", not
		// "touches any payload byte". Idempotent: true — archiving an
		// already-archived record reports the "already" outcome and issues
		// no write (idempotent by value, not by re-stamping).
		MCPTool: "", CLICommand: "spine-review archive",
		Class: Class{ReadOnly: false, Destructive: false, Idempotent: true, OpenWorld: false},
	},
	{
		// spine-review restore is archive's exact inverse — see the archive
		// row immediately above for the full set_visibility precedent this
		// classification rests on. Idempotent: true — restoring a
		// never-archived record reports the "already" outcome and mutates
		// nothing.
		MCPTool: "", CLICommand: "spine-review restore",
		Class: Class{ReadOnly: false, Destructive: false, Idempotent: true, OpenWorld: false},
	},
	{
		// spine-review purge is the phase's sharpest edge: a genuine,
		// permanent removal of one or more records (never reversible, unlike
		// archive), gated on rule 7smp8vy9hr's extract-before-delete
		// precondition and D-11's preview/apply intersection. Destructive:
		// true routes it through registerDestructive (D-03's derivation),
		// which is the entire payoff of Phase 2's D-11 blast-radius table --
		// this command is BORN classified rather than retrofitted.
		// Idempotent: true -- re-running --apply with the same manifest
		// after a successful apply deletes nothing further (the already-
		// deleted ids are simply absent from the fresh re-derivation), the
		// same REST-DELETE-style idempotency reasoning delete_memory/
		// delete_all already carry in this table.
		MCPTool: "", CLICommand: "spine-review purge",
		Class: Class{ReadOnly: false, Destructive: true, Idempotent: true, OpenWorld: false},
	},
}

// classByTool/classByCommand are built once from operations, backing
// ClassForTool/ClassForCommand — the same declared-slice/
// built-once-derived-map/exported-lookup-function shape
// internal/config/registry.go's flagToDefault/FlagDefault uses.
var classByTool = func() map[string]Class {
	m := make(map[string]Class, len(operations))
	for _, op := range operations {
		if op.MCPTool != "" {
			m[op.MCPTool] = op.Class
		}
	}
	return m
}()

var classByCommand = func() map[string]Class {
	m := make(map[string]Class, len(operations))
	for _, op := range operations {
		if op.CLICommand != "" {
			m[op.CLICommand] = op.Class
		}
	}
	return m
}()

// Operations returns a defensive copy of the declared operation registry,
// in declaration order.
func Operations() []Operation {
	out := make([]Operation, len(operations))
	copy(out, operations)
	return out
}

// ClassForTool looks up an MCP tool's blast-radius Class by its registered
// name.
func ClassForTool(name string) (Class, bool) {
	c, ok := classByTool[name]
	return c, ok
}

// ClassForCommand looks up a CLI command's blast-radius Class by its
// qualified command path (D-01: a top-level command's bare cobra Use name,
// e.g. "reindex"; a nested leaf's space-joined path, e.g.
// "spine-review scan"; or, for the bare self-describe invocation, "engram").
func ClassForCommand(name string) (Class, bool) {
	c, ok := classByCommand[name]
	return c, ok
}

// ValidateOperations rejects the declared registry if it has a duplicate
// non-empty MCPTool, a duplicate non-empty CLICommand, or a row with both
// key columns empty (which would carry a Class attributable to no named
// operation on either lane).
func ValidateOperations() error {
	return validateOperationSet(operations)
}

// validateOperationSet implements ValidateOperations' checks over an
// arbitrary operation slice, so toolclass_test.go can exercise every
// failure mode against a throwaway set without mutating the package-level
// registry.
func validateOperationSet(set []Operation) error {
	seenTools := make(map[string]bool, len(set))
	seenCommands := make(map[string]bool, len(set))
	for _, op := range set {
		if op.MCPTool == "" && op.CLICommand == "" {
			return fmt.Errorf("surfaces: operation has both MCPTool and CLICommand empty")
		}
		if op.MCPTool != "" {
			if seenTools[op.MCPTool] {
				return fmt.Errorf("surfaces: duplicate MCPTool %q", op.MCPTool)
			}
			seenTools[op.MCPTool] = true
		}
		if op.CLICommand != "" {
			if seenCommands[op.CLICommand] {
				return fmt.Errorf("surfaces: duplicate CLICommand %q", op.CLICommand)
			}
			seenCommands[op.CLICommand] = true
		}
	}
	return nil
}
