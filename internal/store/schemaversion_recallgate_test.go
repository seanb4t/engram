// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves ROADMAP success criterion 4: schema_version never appears
// in any Qdrant recall or authz filter condition transmitted by Search,
// SearchReranked, SearchDiscovery, List, ListScheduled or ListScopes.
//
// THE AUTHORITATIVE PROOF is TestSchemaVersionNeverGatesRecall (Task 3): a
// gRPC unary interceptor captures the *qdrant.Filter objects the six
// caller-facing recall entry points actually TRANSMIT to a real Qdrant, and
// a recursive walker (Task 1, walkFilterKeys) proves schema_version is
// absent from every one of them. This is evidence, not inference: the
// interceptor observes the wire, it does not reconstruct a filter by
// re-calling ownerScopeFilter/listFilter and re-appending conditions.
//
// TestRecallEmissionSetIsCompleteAndClassified (Task 2) is a SECONDARY,
// static layer: a go/ast derivation of every place internal/store transmits
// a Query/QueryBatch/Scroll/ScrollAndOffset/Count call, closed over a
// same-package call graph from the six recall entry points, with every
// emission site landing in exactly one of three explicitly justified
// categories. Its job is to catch TOMORROW'S new write path — not today's,
// which Task 3 already proves directly — and it is stated at exactly the
// strength go/ast establishes, no further:
//
//  1. Function values, interface dispatch and reflection are not followed
//     by the call graph. The three-way classification IS the backstop for
//     this limit: an emission site the call graph cannot reach still
//     surfaces in the derived set (the scan is reachability-independent)
//     and must be written into one of the three lists by hand, with a
//     justification. The failure mode is an active, reviewable
//     misclassification, never silence.
//  2. The method vocabulary ({Query, QueryBatch, Scroll, ScrollAndOffset,
//     Count}) is a MAINTAINED LIST, and the classification CANNOT backstop
//     it: an emission behind an unenumerated method name produces no
//     subject at all, reaches none of the three lists, and causes no set
//     difference. ScrollAndOffset was exactly this case until this
//     revision — see prove-RED direction B below, the regression guard
//     that proves the widening actually took effect.
//  3. No type identity: go/ast alone cannot verify a matched selector call
//     belongs to *qdrant.Client, and the name-only call graph cannot
//     distinguish a same-named method on an unrelated type from the real
//     one. This biases BOTH the emission scan and the call graph toward
//     OVER-approximation (a spurious subject costs one line of
//     classification; a collision in the call graph can only invent an
//     edge, never erase one, which pushes MORE sites into the stricter
//     recallTransmitters bucket) — the correct bias for a gate, since a
//     false positive costs a classification line and a false negative is
//     the hole a Phase 1 gate shipped with (durable record x6v6qxqd6f).
//
// Do not read this file as claiming "an unreachable emission always
// surfaces for justification" unconditionally — that sentence is true only
// within the scanned vocabulary (limit 2 above), and overclaiming it is
// precisely the mistake this phase's cross-AI review caught and this
// revision corrects.
//
// This file reuses (does not re-implement) plan 02-02's scanQdrantCalls,
// scanPackageDirForCalls and receiverText from schemaversion_stamp_gate_test.go
// — the whole point of that plan pinning them to a caller-supplied method
// set.
package store

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
)

// ============================================================================
// Task 1: the recursive filter-key walker and its positive/negative controls
// ============================================================================

// walkFilterKeys returns, as a sorted slice, every payload field key f
// references — recursing through Must, Should AND MustNot (a MustNot
// reference is just as capable of narrowing recall as a Must one) and
// through every one of the pinned go-client@v1.18.3 client's SEVEN
// Condition oneof variants, read from the module cache rather than guessed:
// Field, IsEmpty, IsNull, Nested (its own key, AND its wrapped sub-filter,
// if any), Filter (a wrapped sub-filter — the recursion), HasId and
// HasVector (both carry no payload key and are handled explicitly so that
// "handled" is a checked property, not an unwritten assumption).
//
// All seven are named here by design: an eighth variant added by a future
// client upgrade is a hole, so walkCondition fails the test loudly via
// t.Fatalf rather than silently falling through an unnamed default.
//
// f == nil is a documented no-op: it returns an empty slice without
// panicking.
//
// The returned slice is sorted so every assertion built on it is
// order-independent — reordering the f.Must appends in Search (or any
// other builder) must not be able to change any verdict this file reaches.
func walkFilterKeys(t *testing.T, f *qdrant.Filter) []string {
	t.Helper()
	keys := map[string]bool{}
	walkFilter(t, f, keys)
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// walkFilter recurses into f's three condition slices. Load-bearing, not
// defensive: categoryMatchCondition wraps its OR group via
// qdrant.NewFilterAsCondition, so a walker that only scanned the top level
// would miss any key buried in that group and the whole gate would pass
// vacuously.
func walkFilter(t *testing.T, f *qdrant.Filter, keys map[string]bool) {
	t.Helper()
	if f == nil {
		return
	}
	walkConditions(t, f.GetMust(), keys)
	walkConditions(t, f.GetShould(), keys)
	walkConditions(t, f.GetMustNot(), keys)
}

func walkConditions(t *testing.T, conds []*qdrant.Condition, keys map[string]bool) {
	t.Helper()
	for _, c := range conds {
		walkCondition(t, c, keys)
	}
}

// walkCondition handles all seven Condition oneof variants of
// go-client@v1.18.3 (qdrant/qdrant_common.pb.go:292-306) by concrete
// wrapper type, matched with an EXHAUSTIVE type switch — never a default
// that silently ignores an unrecognized shape. A nil oneof (a legitimately
// possible zero-value Condition) contributes nothing. Any wrapper type this
// switch does not recognize is a client-version mismatch and fails the
// test loudly, naming the unrecognized Go type.
func walkCondition(t *testing.T, c *qdrant.Condition, keys map[string]bool) {
	t.Helper()
	if c == nil {
		return
	}
	switch v := c.GetConditionOneOf().(type) {
	case *qdrant.Condition_Field:
		keys[v.Field.GetKey()] = true
	case *qdrant.Condition_IsEmpty:
		keys[v.IsEmpty.GetKey()] = true
	case *qdrant.Condition_IsNull:
		keys[v.IsNull.GetKey()] = true
	case *qdrant.Condition_HasId:
		// HasId carries no payload key — handled explicitly, contributes
		// nothing. Pinned by TestFilterWalkerSeesEveryPosition's
		// has-id/has-vector subtest.
	case *qdrant.Condition_HasVector:
		// HasVector carries no payload key — same as HasId, above.
	case *qdrant.Condition_Nested:
		keys[v.Nested.GetKey()] = true
		walkFilter(t, v.Nested.GetFilter(), keys)
	case *qdrant.Condition_Filter:
		walkFilter(t, v.Filter, keys)
	case nil:
		// An empty oneof — legitimately possible for a zero-value
		// Condition. Contributes nothing.
	default:
		t.Fatalf("walkCondition: unhandled Condition oneof variant %T — the pinned go-client@v1.18.3 exposes exactly seven (Field, IsEmpty, HasId, Filter, IsNull, Nested, HasVector; qdrant/qdrant_common.pb.go:292-306); an eighth means the module version moved and this walker must be updated before this gate can be trusted", v)
	}
}

// assertKeysEqual compares got (already sorted by walkFilterKeys) against
// want as a SET, never as a bare non-zero-length check — an "at least one
// key" assertion would pass a walker that returns the wrong keys entirely.
func assertKeysEqual(t *testing.T, got []string, want []string) {
	t.Helper()
	wantSorted := append([]string(nil), want...)
	sort.Strings(wantSorted)
	if !slices.Equal(got, wantSorted) {
		t.Errorf("walked key set = %v, want %v", got, wantSorted)
	}
}

// TestFilterWalkerSeesEveryPosition proves walkFilterKeys sees every
// position a schema_version condition could hide in — both directions: ten
// positive/structural subtests plus two negative controls (adjacency and a
// clean filter), so a walker that returned an empty set for every input
// could not pass this file.
func TestFilterWalkerSeesEveryPosition(t *testing.T) {
	// A nil filter must not panic. Checked once here, outside any subtest,
	// so it does not perturb the ten-subtest count this task's <verify>
	// command greps for.
	if got := walkFilterKeys(t, nil); len(got) != 0 {
		t.Fatalf("walkFilterKeys(nil) = %v, want an empty slice (and must not panic)", got)
	}

	t.Run("top-level Must", func(t *testing.T) {
		f := &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch(schemaVersionKey, "irrelevant-value")}}
		assertKeysEqual(t, walkFilterKeys(t, f), []string{schemaVersionKey})
	})

	t.Run("MustNot", func(t *testing.T) {
		f := &qdrant.Filter{MustNot: []*qdrant.Condition{qdrant.NewMatch(schemaVersionKey, "irrelevant-value")}}
		assertKeysEqual(t, walkFilterKeys(t, f), []string{schemaVersionKey})
	})

	t.Run("is-empty", func(t *testing.T) {
		// The exact condition shape the inverted-cardinality trap would
		// introduce: absence is the rare state for superseded_by/archived_at
		// but the MAJORITY state for schema_version at adoption.
		f := &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewIsEmpty(schemaVersionKey)}}
		assertKeysEqual(t, walkFilterKeys(t, f), []string{schemaVersionKey})
	})

	t.Run("is-null", func(t *testing.T) {
		f := &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewIsNull(schemaVersionKey)}}
		assertKeysEqual(t, walkFilterKeys(t, f), []string{schemaVersionKey})
	})

	t.Run("nested object condition", func(t *testing.T) {
		// qdrant.NewNestedCondition sets Nested.Key itself AND wraps the
		// given condition inside Nested.Filter.Must — both halves are
		// asserted in this one subtest, per the plan's instruction.
		f := &qdrant.Filter{Must: []*qdrant.Condition{
			qdrant.NewNestedCondition("metadata", qdrant.NewMatch(schemaVersionKey, "irrelevant-value")),
		}}
		assertKeysEqual(t, walkFilterKeys(t, f), []string{"metadata", schemaVersionKey})
	})

	t.Run("has-id and has-vector carry no payload key", func(t *testing.T) {
		// Pins the two non-key-bearing variants as a CHECKED property: the
		// walked key set is empty and the walker does not panic. A walker
		// that panicked or mis-keyed either variant fails here.
		f := &qdrant.Filter{Must: []*qdrant.Condition{
			qdrant.NewHasID(qdrant.NewID("00000000-0000-0000-0000-000000000000")),
			qdrant.NewHasVector("dense"),
		}}
		got := walkFilterKeys(t, f)
		if len(got) != 0 {
			t.Errorf("walked key set = %v, want empty — HasId/HasVector carry no payload key", got)
		}
	})

	t.Run("nested Should group", func(t *testing.T) {
		// THE LOAD-BEARING SUBTEST: this is exactly categoryMatchCondition's
		// shape (qdrant.NewFilterAsCondition(&qdrant.Filter{Should: ...})).
		// A non-recursive walker — one that only scanned the top-level
		// Must/Should/MustNot without following Condition_Filter — would
		// report this filter clean while a schema_version condition sits
		// inside the OR group, and the whole gate would pass vacuously.
		f := &qdrant.Filter{Must: []*qdrant.Condition{
			qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
				qdrant.NewMatch(schemaVersionKey, "irrelevant-value"),
				qdrant.NewMatch("other_key", "x"),
			}}),
		}}
		assertKeysEqual(t, walkFilterKeys(t, f), []string{schemaVersionKey, "other_key"})
	})

	t.Run("doubly nested", func(t *testing.T) {
		// Proves the recursion is UNBOUNDED, not merely one level deep.
		inner := qdrant.NewFilterAsCondition(&qdrant.Filter{Should: []*qdrant.Condition{
			qdrant.NewMatch(schemaVersionKey, "irrelevant-value"),
		}})
		outer := qdrant.NewFilterAsCondition(&qdrant.Filter{Must: []*qdrant.Condition{inner}})
		f := &qdrant.Filter{Must: []*qdrant.Condition{outer}}
		assertKeysEqual(t, walkFilterKeys(t, f), []string{schemaVersionKey})
	})

	t.Run("adjacency negative control", func(t *testing.T) {
		// Matching is exact string equality: a key that merely contains or
		// is contained by the target must not register as a hit.
		f := &qdrant.Filter{Must: []*qdrant.Condition{
			qdrant.NewMatch("scope", "x"),
			qdrant.NewMatch("owner", "y"),
			qdrant.NewMatch("schema", "z"),
			qdrant.NewMatch("schema_version_legacy", "w"),
		}}
		got := walkFilterKeys(t, f)
		if slices.Contains(got, schemaVersionKey) {
			t.Errorf("walked key set %v contains %q — matching must be exact string equality, not substring/prefix/suffix", got, schemaVersionKey)
		}
		assertKeysEqual(t, got, []string{"scope", "owner", "schema", "schema_version_legacy"})
	})

	t.Run("clean filter negative control", func(t *testing.T) {
		// A realistic recall-shaped filter with no version reference. A
		// walker that returned an EMPTY set for every input would pass a
		// naive absence check trivially — asserting non-emptiness here is
		// what stops that.
		f := &qdrant.Filter{Must: []*qdrant.Condition{
			qdrant.NewMatch("scope", "x"),
			qdrant.NewMatch("owner", "y"),
			qdrant.NewIsEmpty("superseded_by"),
			qdrant.NewIsEmpty("archived_at"),
		}}
		got := walkFilterKeys(t, f)
		if len(got) == 0 {
			t.Fatal("walked key set is empty for a realistic recall filter — a walker that always returns empty would pass a naive absence check")
		}
		if slices.Contains(got, schemaVersionKey) {
			t.Errorf("walked key set %v unexpectedly contains %q", got, schemaVersionKey)
		}
	})
}

// ============================================================================
// Task 2: derive the recall emission set by reachability, and assert the
// three-way classification is complete
// ============================================================================

// recallEmissionMethods is the TRANSMITTED-BOUNDARY method vocabulary this
// file's completeness derivation scans for: every Qdrant client method that
// can carry a *qdrant.Filter and is called somewhere in internal/store's
// package directory. It is a MAINTAINED LIST whose incompleteness the
// three-way classification CANNOT backstop (limit 2 in this file's package
// doc comment): an emission behind a method name not in this set produces
// no derived subject at all, reaches none of the three classification
// lists, and causes no set difference — the suite stays green with nothing
// to review.
//
// ScrollAndOffset's presence is load-bearing, not decorative: cycle-2
// review found four non-test ScrollAndOffset call sites (three carrying
// real filters), which the prior {Query, QueryBatch, Scroll, Count}
// vocabulary silently missed, making the previous revision's
// union-completeness claim false against real source. See prove-RED
// direction B below — the regression guard proving the widening took
// effect: it would have passed GREEN before ScrollAndOffset joined this
// set.
var recallEmissionMethods = map[string]bool{
	"Query":           true,
	"QueryBatch":      true,
	"Scroll":          true,
	"ScrollAndOffset": true,
	"Count":           true,
}

// recallEntryPointSeeds is the six caller-facing recall entry points this
// whole gate is anchored on — declared ONCE, here, and shared with
// TestSchemaVersionNeverGatesRecall's invocation table (Task 3) via each
// row's entryPoint field. The classification-coverage linkage subtest below
// asserts this list is set-equal to the distinct entry points
// recallInvocationRows actually invokes, so a function classified
// recall-transmitted but never walked by any row cannot stay invisible.
//
// ListScopes is IN: it is served to callers through BOTH Connect
// (internal/server/connectapi.go) and MCP (internal/server/tools.go), so
// D-16's operator-tier exclusion rationale does not reach it — a
// schema_version condition here would narrow what a user can see, exactly
// what criterion 4 forbids.
var recallEntryPointSeeds = []string{
	"Store.Search",
	"Store.SearchReranked", // delegates to Store.Search; builds no filter of its own — see the subset assertion below
	"Store.SearchDiscovery",
	"Store.List",
	"Store.ListScheduled",
	"Store.ListScopes",
}

// buildSamePackageCallGraph walks every non-test .go file matching
// suffix/excludeSuffix in dir and, for each enclosing function (keyed by
// enclosingFuncDisplayName — exactly the identity space scanQdrantCalls
// uses), records the set of callee names it invokes:
//
//   - a plain identifier call (a package-level function, e.g.
//     matchNothing(...)) records that identifier's bare name;
//   - a selector call whose receiver expression is PRECISELY the enclosing
//     declaration's own receiver identifier (e.g. s.ownerScopeFilter(...)
//     inside a method on *Store, where the method's own receiver parameter
//     is named s) records "RecvType.Sel" — the same display name
//     enclosingFuncDisplayName gives that function if it is itself a
//     method.
//
// A callee that is neither of these two shapes produces NO EDGE AT ALL —
// this is limit 1 from the file's package doc comment: function values,
// interface dispatch, and deeper chains such as s.client.Scroll(...) (whose
// receiver expression s.client is itself a SelectorExpr, not a bare
// identifier matching the receiver name) are not followed. The three-way
// classification is the documented backstop for this limit.
//
// This is also where limit 3 (no type identity) shows its bias: the match
// is on the IDENTIFIER NAME alone, never verified against a real type, so a
// same-named receiver variable belonging to an unrelated type collides into
// an edge. A collision can only INVENT an edge — never erase one — which
// enlarges the reachable set and pushes MORE sites into the stricter
// recallTransmitters bucket. That is the correct bias for a gate.
func buildSamePackageCallGraph(fset *token.FileSet, dir, suffix, excludeSuffix string) (graph map[string]map[string]bool, filesScanned int, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, fmt.Errorf("read dir %s: %w", dir, err)
	}
	graph = map[string]map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		if excludeSuffix != "" && strings.HasSuffix(e.Name(), excludeSuffix) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, filesScanned, fmt.Errorf("read %s: %w", path, readErr)
		}
		file, parseErr := parser.ParseFile(fset, path, src, 0)
		if parseErr != nil {
			return nil, filesScanned, fmt.Errorf("parse %s: %w", path, parseErr)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			name := enclosingFuncDisplayName(fn)
			recvType, recvName := "", ""
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				recvType = recvTypeName(fn.Recv.List[0].Type)
				if len(fn.Recv.List[0].Names) > 0 {
					recvName = fn.Recv.List[0].Names[0].Name
				}
			}
			callees := map[string]bool{}
			ast.Inspect(fn, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fnExpr := call.Fun.(type) {
				case *ast.Ident:
					callees[fnExpr.Name] = true
				case *ast.SelectorExpr:
					if id, ok := fnExpr.X.(*ast.Ident); ok && recvName != "" && id.Name == recvName {
						callees[recvType+"."+fnExpr.Sel.Name] = true
					}
					// else: not a direct receiver-selector call — no edge.
				}
				return true
			})
			graph[name] = callees
		}
		filesScanned++
	}
	return graph, filesScanned, nil
}

// reachableFrom computes the transitive closure of graph starting from
// every name in seeds (seeds themselves are included in the result), using
// graph's own key space (enclosingFuncDisplayName) throughout.
func reachableFrom(graph map[string]map[string]bool, seeds []string) map[string]bool {
	reached := map[string]bool{}
	var visit func(string)
	visit = func(name string) {
		if reached[name] {
			return
		}
		reached[name] = true
		for callee := range graph[name] {
			visit(callee)
		}
	}
	for _, s := range seeds {
		visit(s)
	}
	return reached
}

// recallEmissionClassification pairs one derived enclosing function with a
// justification for its classification bucket. A shared type across all
// three lists below (recallTransmitters/operatorMigrationEmitters/
// otherNonRecallEmitters), each entry's justification names the emission
// method(s) it performs, the source location, and (for recallTransmitters)
// the entry point(s) it serves.
type recallEmissionClassification struct {
	enclosingFunc string
	justification string
}

// recallTransmitters — reachable from recallEntryPointSeeds, six entries.
// Re-derived at revision time against current source (line numbers below
// are as-observed, not the plan's — enclosing FUNCTION NAME is this gate's
// identity key, never a line number, since lines shift on every edit).
var recallTransmitters = []recallEmissionClassification{
	{
		enclosingFunc: "Store.Search",
		justification: "Emits Query (store.go:1105), its own transmission. Serves Search directly and SearchReranked via delegation — SearchReranked builds no filter of its own; see the subset assertion below.",
	},
	{
		enclosingFunc: "Store.SearchDiscovery",
		justification: "Emits Query (store.go:1196), its own transmission. Serves SearchDiscovery.",
	},
	{
		enclosingFunc: "Store.List",
		justification: "Emits Count (store.go:1339) then Scroll (store.go:1367), its own transmissions. Serves List's offset-mode path.",
	},
	{
		enclosingFunc: "Store.listByCursor",
		justification: "Emits Scroll (store.go:1425). NOT a seed itself — reachable from Store.List, which dispatches into it for cursor-mode paging (store.go:1346-1348) — but a derived member of the recall-transmitted set all the same. Serves List's cursor-mode path.",
	},
	{
		enclosingFunc: "Store.ListScheduled",
		justification: "Emits Scroll (store.go:1573), its own transmission. Serves ListScheduled.",
	},
	{
		enclosingFunc: "Store.ListScopes",
		justification: "Emits Scroll (store.go:1616), its own transmission. Serves ListScopes — exposed to callers through BOTH Connect (internal/server/connectapi.go) and MCP (internal/server/tools.go), so D-16's operator-tier exclusion rationale does not reach it.",
	},
}

// operatorMigrationEmitters — D-16's deliberately excluded tier, not
// reachable from recallEntryPointSeeds, ten entries.
var operatorMigrationEmitters = []recallEmissionClassification{
	{
		enclosingFunc: "Store.CountOwnerless",
		justification: "Emits Count (store.go:2656). D-16 operator diagnostic: Phase 3's migration sweep must be able to filter/count by schema_version to find its backlog, so a blanket ban across every Qdrant query in this package would make Phase 3 unimplementable.",
	},
	{
		enclosingFunc: "Store.BackfillShortIDs",
		justification: "Emits Count (store.go:2756) and ScrollAndOffset (store.go:2765). D-16 operator command (`engram backfill-short-ids`); same Phase 3 rationale as Store.CountOwnerless.",
	},
	{
		enclosingFunc: "Store.CountAnonymousBucket",
		justification: "Emits Count (store.go:2819). D-16 operator diagnostic; same Phase 3 rationale.",
	},
	{
		enclosingFunc: "Store.MigrateSetOwner",
		justification: "Emits Count (store.go:2859). D-16 operator command (`engram migrate-remap-owner`); same Phase 3 rationale.",
	},
	{
		enclosingFunc: "Store.RemapOwner",
		justification: "Emits Count (store.go:2983). D-16 operator command (`engram migrate-remap-owner`); same Phase 3 rationale.",
	},
	{
		enclosingFunc: "Store.Reindex",
		justification: "Emits ScrollAndOffset (store.go:3199). D-16 operator command (`engram reindex`). Plan 02-01 already corrected Reindex's doc comment: its per-point write copies the source payload map verbatim and never advances schema_version, and its scroll read is likewise outside the recall boundary.",
	},
	{
		enclosingFunc: "Store.Migrate",
		justification: "Emits Count (internal/store/migrate.go) and ScrollAndOffset (internal/store/migrate.go), both against backlogFilter. D-16 operator command — Phase 3's migration sweep must be able to filter/count by schema_version to find its own backlog, so a blanket ban across every Qdrant query in this package would make Phase 3 unimplementable. backlogFilter is never reachable from any recallEntryPointSeeds member.",
	},
	{
		enclosingFunc: "Store.scrollAllPoints",
		justification: "Emits ScrollAndOffset (spine.go:49). The package's ONE shared paginated whole-spine iterator, behind ScanSpine/EnumerateCitations/NearDuplicates/derivePurgeEligible — all operator-tier today. This entry buys something specific: if a future recall path ever routes through it, reachability pulls it into the reachable set, it stops matching this entry, and the suite goes RED.",
	},
	{
		enclosingFunc: "Store.CountExpired",
		justification: "Emits Count (spine.go:128). D-16 operator sweep (`engram prune-expired`); same Phase 3 rationale.",
	},
	{
		enclosingFunc: "Store.NearDuplicates",
		justification: "Emits QueryBatch (spine.go:589). Operator/diagnostic near-duplicate detection, not a caller-facing recall surface; same Phase 3 rationale.",
	},
	{
		enclosingFunc: "Store.SummarizeMissing",
		justification: "Emits ScrollAndOffset (summarize.go:145). D-16 operator command (`engram summarize-missing`); same Phase 3 rationale.",
	},
	{
		enclosingFunc: "Store.MigrateStatus",
		justification: "Emits Count (internal/store/migrate_status.go, twice: an IsEmpty(schema_version) exact Count for the absent/legacy bucket, and an unfiltered exact Count for the whole-collection total). D-16 operator diagnostic behind `engram migrate status`; same Phase 3 rationale as Store.CountOwnerless — the histogram must be able to count by schema_version presence/absence to report the migration backlog's shape. Never reachable from any recallEntryPointSeeds member.",
	},
	{
		enclosingFunc: "Store.revertWithSteps",
		justification: "Emits Count (internal/store/revert.go) and ScrollAndOffset (internal/store/revert.go), both against aboveTargetFilter — the write loop's own re-derivation, mirroring Store.Migrate's row above. D-16 operator command (`engram migrate revert`); same Phase 3 rationale — the revert sweep must be able to filter/count by schema_version to find its own above-target backlog. Store.previewRevertWithSteps enumerates the SAME range but through Store.scrollAllPoints (already classified below), so it needs no row of its own. Never reachable from any recallEntryPointSeeds member.",
	},
}

// otherNonRecallEmitters — the third category the previous binary partition
// had no home for: neither recall nor operator, two entries. Each
// justification is per-item and specific, never shared boilerplate.
var otherNonRecallEmitters = []recallEmissionClassification{
	{
		enclosingFunc: "Store.ResolvePointID",
		justification: "Emits Scroll (store.go:1692). Id-addressed lookup: the caller already supplies an exact identifier and this function resolves it to the canonical point id. Not a recall surface — there is no result set to narrow, only a hit or a miss on an id the caller already named. A version condition here would break id resolution loudly, not silently shrink recall.",
	},
	{
		enclosingFunc: "Store.MintShortID",
		justification: "Emits Count (store.go:2705). A collision probe on a candidate short id during minting. It reads no caller-visible result set.",
	},
}

func classificationNames(cs []recallEmissionClassification) map[string]bool {
	out := map[string]bool{}
	for _, c := range cs {
		out[c.enclosingFunc] = true
	}
	return out
}

func intersectNameSets(a, b map[string]bool) map[string]bool {
	out := map[string]bool{}
	for k := range a {
		if b[k] {
			out[k] = true
		}
	}
	return out
}

// assertNameSetEqual compares got against want as SETS, printing BOTH
// difference directions by name on failure — so a reordered append or an
// unrelated map iteration can never change the verdict, and a failure names
// exactly what is missing versus what is extra.
func assertNameSetEqual(t *testing.T, context string, got, want map[string]bool) {
	t.Helper()
	var missing, extra []string
	for name := range want {
		if !got[name] {
			missing = append(missing, name)
		}
	}
	for name := range got {
		if !want[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 {
		t.Errorf("%s: missing (expected but not observed): %v", context, missing)
	}
	if len(extra) > 0 {
		t.Errorf("%s: extra (observed but not expected): %v", context, extra)
	}
}

// deriveRecallEmissionSites scans internal/store's own package directory
// for every call site matching recallEmissionMethods, failing loudly on a
// read/parse error and on a zero scanned-file count — a scan that cannot
// see what it was told to check must never report clean.
func deriveRecallEmissionSites(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	sites, filesScanned, err := scanPackageDirForCalls(fset, ".", ".go", "_test.go", recallEmissionMethods)
	if err != nil {
		t.Fatalf("scan internal/store for recall emission sites: %v", err)
	}
	if filesScanned == 0 {
		t.Fatal("scanned zero non-test .go files in internal/store — a scan that sees nothing must not report clean")
	}
	names := map[string]bool{}
	for _, s := range sites {
		names[s.enclosingFunc] = true
	}
	return names
}

// buildRecallCallGraph builds the same-package call graph over
// internal/store's own package directory, failing loudly exactly like
// deriveRecallEmissionSites.
func buildRecallCallGraph(t *testing.T) map[string]map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	graph, filesScanned, err := buildSamePackageCallGraph(fset, ".", ".go", "_test.go")
	if err != nil {
		t.Fatalf("build same-package call graph over internal/store: %v", err)
	}
	if filesScanned == 0 {
		t.Fatal("scanned zero non-test .go files building the call graph — a scan that sees nothing must not report clean")
	}
	return graph
}

// TestRecallEmissionSetIsCompleteAndClassified is D-14 applied to the
// recall boundary: the emission set is DERIVED and asserted COMPLETE, never
// hand-listed, with the derivation running over the transmitted boundary
// and reachability (per review findings H3/H4) rather than over filter
// literals. See this file's package doc comment for the three limits this
// derivation is stated at.
func TestRecallEmissionSetIsCompleteAndClassified(t *testing.T) {
	t.Run("reachable emission completeness", func(t *testing.T) {
		derived := deriveRecallEmissionSites(t)
		graph := buildRecallCallGraph(t)
		reachable := reachableFrom(graph, recallEntryPointSeeds)
		reachableEmission := intersectNameSets(derived, reachable)
		assertNameSetEqual(t, "reachable-from-seed emission sites vs recallTransmitters", reachableEmission, classificationNames(recallTransmitters))
	})

	t.Run("three-way classification of the remainder", func(t *testing.T) {
		derived := deriveRecallEmissionSites(t)

		rt := classificationNames(recallTransmitters)
		ome := classificationNames(operatorMigrationEmitters)
		onre := classificationNames(otherNonRecallEmitters)

		for name := range rt {
			if ome[name] {
				t.Errorf("%s is classified in BOTH recallTransmitters and operatorMigrationEmitters", name)
			}
			if onre[name] {
				t.Errorf("%s is classified in BOTH recallTransmitters and otherNonRecallEmitters", name)
			}
		}
		for name := range ome {
			if onre[name] {
				t.Errorf("%s is classified in BOTH operatorMigrationEmitters and otherNonRecallEmitters", name)
			}
		}

		union := map[string]bool{}
		for n := range rt {
			union[n] = true
		}
		for n := range ome {
			union[n] = true
		}
		for n := range onre {
			union[n] = true
		}
		assertNameSetEqual(t, "union of the three classification lists vs the full derived emission set — a function appearing in two lists or in none fails this", union, derived)

		for _, c := range operatorMigrationEmitters {
			if c.justification == "" {
				t.Errorf("operatorMigrationEmitters entry %s has no justification", c.enclosingFunc)
			}
		}
		foundPhase3Rationale := false
		for _, c := range operatorMigrationEmitters {
			if strings.Contains(c.justification, "Phase 3") {
				foundPhase3Rationale = true
				break
			}
		}
		if !foundPhase3Rationale {
			t.Error("no operatorMigrationEmitters justification states the Phase 3 rationale (the migration sweep must be able to filter by schema_version) for D-16's exclusion")
		}
		foundScrollAllPointsRationale := false
		for _, c := range operatorMigrationEmitters {
			if c.enclosingFunc == "Store.scrollAllPoints" && strings.Contains(c.justification, "reachable set") {
				foundScrollAllPointsRationale = true
			}
		}
		if !foundScrollAllPointsRationale {
			t.Error("Store.scrollAllPoints's justification does not name what its entry buys — a recall path routing through it must move it into the reachable set and turn the suite RED")
		}

		seenJustifications := map[string]bool{}
		for _, c := range otherNonRecallEmitters {
			if c.justification == "" {
				t.Errorf("otherNonRecallEmitters entry %s has no justification", c.enclosingFunc)
			}
			if seenJustifications[c.justification] {
				t.Errorf("otherNonRecallEmitters entry %s reuses another entry's justification verbatim — each must be per-item and specific, not shared boilerplate", c.enclosingFunc)
			}
			seenJustifications[c.justification] = true
		}
	})

	t.Run("SearchReranked delegates without self-comparison", func(t *testing.T) {
		derived := deriveRecallEmissionSites(t)
		graph := buildRecallCallGraph(t)

		// Obtained from ITS OWN reachableFrom call — never aliased from
		// Search's result, which would let a self-comparison trivially
		// satisfy the subset relation.
		reachSearch := reachableFrom(graph, []string{"Store.Search"})
		reachReranked := reachableFrom(graph, []string{"Store.SearchReranked"})

		emitSearch := intersectNameSets(reachSearch, derived)
		emitReranked := intersectNameSets(reachReranked, derived)

		for name := range emitReranked {
			if !emitSearch[name] {
				t.Errorf("Store.SearchReranked's reachable emission set contains %s, which Store.Search's reachable emission set does not — SearchReranked may have grown its own query and needs its own classification entry", name)
			}
		}
		// The NON-TRIVIAL fact a self-comparison could not satisfy:
		// SearchReranked itself must not be a member of the derived
		// emission set (it builds no query of its own).
		if derived["Store.SearchReranked"] {
			t.Error("Store.SearchReranked appears in the derived emission set — it is supposed to delegate to Store.Search and build no query of its own; if this fires, give it its own classification entry rather than treating it as a delegation")
		}
	})

	t.Run("classification-coverage linkage", func(t *testing.T) {
		// Compares the seed set (declared once, above) against the
		// DISTINCT entry points Task 3's invocation table actually
		// invokes. Without this, a function could be classified
		// recall-transmitted, never walked by any row, and both this test
		// and TestSchemaVersionNeverGatesRecall would stay green — the
		// classification-without-coverage hole cross-AI review found.
		seeds := map[string]bool{}
		for _, s := range recallEntryPointSeeds {
			seeds[s] = true
		}
		invoked := map[string]bool{}
		for _, r := range recallInvocationRows {
			invoked[r.entryPoint] = true
		}
		assertNameSetEqual(t, "recallEntryPointSeeds vs recallInvocationRows' distinct entry points (classification-coverage linkage)", invoked, seeds)
	})
}

// ============================================================================
// Task 3: walk the filter actually transmitted to Qdrant by all six recall
// entry points
// ============================================================================

// capturedFilter pairs one transmitted *qdrant.Filter with the gRPC method
// name of the request that carried it.
type capturedFilter struct {
	method string
	filter *qdrant.Filter
}

// recallCapture is a mutex-guarded, resettable sink for capturedFilter
// entries recorded by the interceptor dialCapturingTestClient installs. The
// reset ensures seeding writes and index creation (EnsureCollection) do not
// pollute a row's recall capture.
type recallCapture struct {
	mu      sync.Mutex
	entries []capturedFilter
}

func (c *recallCapture) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = nil
}

func (c *recallCapture) record(method string, f *qdrant.Filter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, capturedFilter{method: method, filter: f})
}

func (c *recallCapture) snapshot() []capturedFilter {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]capturedFilter, len(c.entries))
	copy(out, c.entries)
	return out
}

// filterCarryingRequest is implemented by any top-level gRPC request type
// that carries a *qdrant.Filter directly (QueryPoints, ScrollPoints,
// CountPoints, and several types this codebase never sends, e.g.
// SearchPoints/RecommendPoints/DiscoverPoints). It is used ONLY by the
// interceptor's default branch to detect an unrecognized filter-carrying
// request rather than silently dropping it — QueryBatchPoints (used by the
// operator-tier Store.NearDuplicates, never a recall entry point) does NOT
// implement this interface, since its filter lives one level down inside
// each of its own QueryPoints elements, not on QueryBatchPoints itself.
type filterCarryingRequest interface {
	GetFilter() *qdrant.Filter
}

// recognizedFilterCarryingRequestMethods is the interceptor's own
// completeness BOUNDARY, declared explicitly rather than left to an
// anonymous inline type-switch: the gRPC method names this file knows how
// to extract a *qdrant.Filter and method name from. If a recall path later
// switches RPC type, the interceptor must fail here, naming the
// unrecognized request type, rather than silently capturing zero and
// letting the gate pass vacuously.
//
// THIS JOIN IS NOT INDEPENDENT CORROBORATION (cycle-2 actionable #9):
// recallTransmitters is itself derived from the same {Query, QueryBatch,
// Scroll, ScrollAndOffset, Count} method vocabulary this type switch
// encodes, so the "interceptor recognized types cover every
// recallTransmitters emission method" subtest below is a CONSISTENCY check
// between two views of one enumeration — it cannot reveal an RPC family
// excluded at the scanner's first step. The independent evidence in this
// file is the LIVE CAPTURE itself, not this join.
//
// ScrollAndOffset needs no separate recognized type: it issues the
// IDENTICAL /qdrant.Points/Scroll gRPC method with the IDENTICAL
// *qdrant.ScrollPoints request as Scroll (go-client@v1.18.3
// qdrant/points.go:70-71 vs :88-89) — grpcMethodForEmission below records
// that equivalence explicitly.
var recognizedFilterCarryingRequestMethods = map[string]bool{
	"Query":  true, // *qdrant.QueryPoints
	"Scroll": true, // *qdrant.ScrollPoints (also covers ScrollAndOffset — see grpcMethodForEmission)
	"Count":  true, // *qdrant.CountPoints
}

// grpcMethodForEmission maps an internal/store call-site method name (as
// scanQdrantCalls reports it) to the gRPC method name
// dialCapturingTestClient's interceptor actually observes on the wire.
func grpcMethodForEmission(callSiteMethod string) string {
	if callSiteMethod == "ScrollAndOffset" {
		return "Scroll"
	}
	return callSiteMethod
}

// recallCaptureInterceptor returns a grpc.UnaryClientInterceptor that
// type-switches req over the three recognized filter-carrying request
// types, records the *qdrant.Filter (possibly nil) plus a normalized method
// name into capture, and fails the test loudly via t.Fatalf if req carries
// a *qdrant.Filter (per filterCarryingRequest) but is not one of the three
// recognized types — never dropping an unrecognized filter-carrying
// request silently.
func recallCaptureInterceptor(t *testing.T, capture *recallCapture) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		switch r := req.(type) {
		case *qdrant.QueryPoints:
			capture.record("Query", r.GetFilter())
		case *qdrant.ScrollPoints:
			capture.record("Scroll", r.GetFilter())
		case *qdrant.CountPoints:
			capture.record("Count", r.GetFilter())
		default:
			if fc, ok := req.(filterCarryingRequest); ok && fc.GetFilter() != nil {
				t.Fatalf("recallCaptureInterceptor: gRPC method %s (request type %T) carries a *qdrant.Filter but is not in the interceptor's recognized set — widen recognizedFilterCarryingRequestMethods and this type switch", method, req)
			}
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// dialCapturingTestClient is dialTestClient's capturing sibling: it wires a
// grpc.WithUnaryInterceptor into the client's dial options so capture
// records every outgoing request the interceptor recognizes as
// filter-carrying. Skips exactly like dialTestClient when no Qdrant is
// available.
func dialCapturingTestClient(t *testing.T, capture *recallCapture) *qdrant.Client {
	t.Helper()
	if testQdrantAddr == "" {
		required, err := requireQdrant()
		if err != nil {
			t.Fatalf("%v", err)
		}
		if required {
			t.Fatal("no Qdrant available and ENGRAM_REQUIRE_QDRANT is set: failing instead of skipping")
		}
		t.Skip("no Qdrant available: set ENGRAM_QDRANT_TEST_ADDR or start Docker (testcontainers)")
	}
	host, portStr, err := net.SplitHostPort(testQdrantAddr)
	if err != nil {
		t.Fatalf("invalid Qdrant address %q: %v", testQdrantAddr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		t.Fatalf("invalid Qdrant port %q (from %q): %v", portStr, testQdrantAddr, err)
	}
	c, err := qdrant.NewClient(&qdrant.Config{
		Host: host, Port: port,
		GrpcOptions: []grpc.DialOption{grpc.WithUnaryInterceptor(recallCaptureInterceptor(t, capture))},
	})
	if err != nil {
		t.Fatalf("capturing client: %v", err)
	}
	return c
}

// The invocation table's fixed inputs: one scope, one query vector (shared
// by every vector-search row), and Search/List option sets that include tag
// and category filters so categoryMatchCondition's nested OR group is
// actually present in at least one captured filter.
const recallGateScope = "schemaversion:project:recallgate"

var recallGateVector = []float32{0.11, 0.22, 0.33}

var recallGateSearchOptions = SearchOptions{
	Tags:       []string{"recallgate-tag"},
	Categories: []string{"decision"},
}

// recallGateListOffsetOptions carries a NONZERO Limit deliberately:
// store.go's List short-circuits to an empty page without ever issuing the
// Scroll when Limit==0 ("all") and the filtered Count returns total==0
// (Qdrant's Scroll rejects Limit=0). Without a nonzero Limit here, this
// row's declared expectation of 2 captures (Count then Scroll) would
// silently reduce to 1 against an empty test collection.
var recallGateListOffsetOptions = ListOptions{
	Limit:      10,
	Categories: []string{"decision"},
	Tags:       []string{"recallgate-tag"},
}

// recallGateListCursorOptions: same nonzero-Limit reasoning as
// recallGateListOffsetOptions above — List's Count-then-Scroll precondition
// applies identically to the cursor-mode path.
var recallGateListCursorOptions = ListOptions{
	Limit:      10,
	CursorMode: true,
}

var recallGateAnonymousSubject = Anonymous()
var recallGateOwnerSubject = Authenticated("schemaversion-recallgate-owner")

// recallInvocationRow is one row of the invocation table: an entry point
// (from recallEntryPointSeeds), a declared EXACT expected capture count and
// gRPC method multiset (both derived from source, never a minimum or a
// default of one), and the closure that drives it.
type recallInvocationRow struct {
	name          string
	entryPoint    string
	expectCount   int
	expectMethods []string
	invoke        func(t *testing.T, ctx context.Context, s *Store)
}

// recallInvocationRows enumerates all FOURTEEN rows explicitly (seven
// invocation shapes crossed with two representative subjects), rather than
// computing the cross product in code, so a reader can count them.
var recallInvocationRows = []recallInvocationRow{
	{
		name: "Search/anonymous", entryPoint: "Store.Search",
		expectCount: 1, expectMethods: []string{"Query"},
		invoke: func(t *testing.T, ctx context.Context, s *Store) {
			t.Helper()
			if _, err := s.Search(ctx, recallGateScope, recallGateAnonymousSubject, recallGateVector, 5, recallGateSearchOptions); err != nil {
				t.Fatalf("Search(anonymous): %v", err)
			}
		},
	},
	{
		name: "Search/owner", entryPoint: "Store.Search",
		expectCount: 1, expectMethods: []string{"Query"},
		invoke: func(t *testing.T, ctx context.Context, s *Store) {
			t.Helper()
			if _, err := s.Search(ctx, recallGateScope, recallGateOwnerSubject, recallGateVector, 5, recallGateSearchOptions); err != nil {
				t.Fatalf("Search(owner): %v", err)
			}
		},
	},
	{
		name: "SearchReranked/anonymous", entryPoint: "Store.SearchReranked",
		expectCount: 1, expectMethods: []string{"Query"},
		invoke: func(t *testing.T, ctx context.Context, s *Store) {
			t.Helper()
			if _, err := s.SearchReranked(ctx, recallGateScope, recallGateAnonymousSubject, "recall gate query text", recallGateVector, 5, SearchOptions{}); err != nil {
				t.Fatalf("SearchReranked(anonymous): %v", err)
			}
		},
	},
	{
		name: "SearchReranked/owner", entryPoint: "Store.SearchReranked",
		expectCount: 1, expectMethods: []string{"Query"},
		invoke: func(t *testing.T, ctx context.Context, s *Store) {
			t.Helper()
			if _, err := s.SearchReranked(ctx, recallGateScope, recallGateOwnerSubject, "recall gate query text", recallGateVector, 5, SearchOptions{}); err != nil {
				t.Fatalf("SearchReranked(owner): %v", err)
			}
		},
	},
	{
		name: "SearchDiscovery/anonymous", entryPoint: "Store.SearchDiscovery",
		expectCount: 1, expectMethods: []string{"Query"},
		invoke: func(t *testing.T, ctx context.Context, s *Store) {
			t.Helper()
			if _, err := s.SearchDiscovery(ctx, recallGateScope, "", recallGateAnonymousSubject, recallGateVector, 5); err != nil {
				t.Fatalf("SearchDiscovery(anonymous): %v", err)
			}
		},
	},
	{
		name: "SearchDiscovery/owner", entryPoint: "Store.SearchDiscovery",
		expectCount: 1, expectMethods: []string{"Query"},
		invoke: func(t *testing.T, ctx context.Context, s *Store) {
			t.Helper()
			if _, err := s.SearchDiscovery(ctx, recallGateScope, "", recallGateOwnerSubject, recallGateVector, 5); err != nil {
				t.Fatalf("SearchDiscovery(owner): %v", err)
			}
		},
	},
	{
		// store.go:1294 short-circuit: see recallGateListOffsetOptions'
		// doc comment — this row's nonzero Limit is what keeps this row at
		// 2 captures instead of silently dropping to 1.
		name: "List-offset/anonymous", entryPoint: "Store.List",
		expectCount: 2, expectMethods: []string{"Count", "Scroll"},
		invoke: func(t *testing.T, ctx context.Context, s *Store) {
			t.Helper()
			if _, _, _, err := s.List(ctx, recallGateScope, recallGateAnonymousSubject, recallGateListOffsetOptions); err != nil {
				t.Fatalf("List-offset(anonymous): %v", err)
			}
		},
	},
	{
		name: "List-offset/owner", entryPoint: "Store.List",
		expectCount: 2, expectMethods: []string{"Count", "Scroll"},
		invoke: func(t *testing.T, ctx context.Context, s *Store) {
			t.Helper()
			if _, _, _, err := s.List(ctx, recallGateScope, recallGateOwnerSubject, recallGateListOffsetOptions); err != nil {
				t.Fatalf("List-offset(owner): %v", err)
			}
		},
	},
	{
		// Same store.go:1294 short-circuit precondition as the offset-mode
		// rows above: List always issues its Count first, regardless of
		// which paging mode follows.
		name: "List-cursor/anonymous", entryPoint: "Store.List",
		expectCount: 2, expectMethods: []string{"Count", "Scroll"},
		invoke: func(t *testing.T, ctx context.Context, s *Store) {
			t.Helper()
			if _, _, _, err := s.List(ctx, recallGateScope, recallGateAnonymousSubject, recallGateListCursorOptions); err != nil {
				t.Fatalf("List-cursor(anonymous): %v", err)
			}
		},
	},
	{
		name: "List-cursor/owner", entryPoint: "Store.List",
		expectCount: 2, expectMethods: []string{"Count", "Scroll"},
		invoke: func(t *testing.T, ctx context.Context, s *Store) {
			t.Helper()
			if _, _, _, err := s.List(ctx, recallGateScope, recallGateOwnerSubject, recallGateListCursorOptions); err != nil {
				t.Fatalf("List-cursor(owner): %v", err)
			}
		},
	},
	{
		name: "ListScheduled/anonymous", entryPoint: "Store.ListScheduled",
		expectCount: 1, expectMethods: []string{"Scroll"},
		invoke: func(t *testing.T, ctx context.Context, s *Store) {
			t.Helper()
			if _, err := s.ListScheduled(ctx, recallGateScope, recallGateAnonymousSubject, ScheduledPending, ListOptions{}); err != nil {
				t.Fatalf("ListScheduled(anonymous): %v", err)
			}
		},
	},
	{
		name: "ListScheduled/owner", entryPoint: "Store.ListScheduled",
		expectCount: 1, expectMethods: []string{"Scroll"},
		invoke: func(t *testing.T, ctx context.Context, s *Store) {
			t.Helper()
			if _, err := s.ListScheduled(ctx, recallGateScope, recallGateOwnerSubject, ScheduledPending, ListOptions{}); err != nil {
				t.Fatalf("ListScheduled(owner): %v", err)
			}
		},
	},
	{
		name: "ListScopes/anonymous", entryPoint: "Store.ListScopes",
		expectCount: 1, expectMethods: []string{"Scroll"},
		invoke: func(t *testing.T, ctx context.Context, s *Store) {
			t.Helper()
			if _, _, err := s.ListScopes(ctx, recallGateAnonymousSubject); err != nil {
				t.Fatalf("ListScopes(anonymous): %v", err)
			}
		},
	},
	{
		name: "ListScopes/owner", entryPoint: "Store.ListScopes",
		expectCount: 1, expectMethods: []string{"Scroll"},
		invoke: func(t *testing.T, ctx context.Context, s *Store) {
			t.Helper()
			if _, _, err := s.ListScopes(ctx, recallGateOwnerSubject); err != nil {
				t.Fatalf("ListScopes(owner): %v", err)
			}
		},
	},
}

// TestSchemaVersionNeverGatesRecall is criterion 4's AUTHORITATIVE proof
// (see this file's package doc comment): schema_version is absent from
// every *qdrant.Filter the six caller-facing recall entry points actually
// TRANSMIT to a real Qdrant, under two representative subjects, with
// per-row exact capture counts and gRPC method multisets derived from
// source.
func TestSchemaVersionNeverGatesRecall(t *testing.T) {
	capture := &recallCapture{}
	c := dialCapturingTestClient(t, capture)
	ctx := context.Background()
	collection := testCollection("schemaversion_recallgate")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	capture.reset() // defensive: EnsureCollection issues no filter-carrying request, but never assume that silently
	if err := s.EnsureCollection(ctx, uint64(len(recallGateVector))); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	t.Run("interceptor recognized types cover every recallTransmitters emission method", func(t *testing.T) {
		fset := token.NewFileSet()
		sites, filesScanned, err := scanPackageDirForCalls(fset, ".", ".go", "_test.go", recallEmissionMethods)
		if err != nil {
			t.Fatalf("scan internal/store for recall emission sites: %v", err)
		}
		if filesScanned == 0 {
			t.Fatal("scanned zero non-test .go files in internal/store — a scan that sees nothing must not report clean")
		}
		rt := classificationNames(recallTransmitters)
		for _, site := range sites {
			if !rt[site.enclosingFunc] {
				continue
			}
			grpcMethod := grpcMethodForEmission(site.method)
			if !recognizedFilterCarryingRequestMethods[grpcMethod] {
				t.Errorf("recallTransmitters site %s emits %s (gRPC method %s), which the interceptor does not recognize — widen recognizedFilterCarryingRequestMethods and recallCaptureInterceptor's type switch", site, site.method, grpcMethod)
			}
		}
	})

	var totalWalked int
	aggregateMethods := map[string]int{}
	sawCategoryKey := false

	for _, row := range recallInvocationRows {
		row := row
		t.Run(row.name, func(t *testing.T) {
			capture.reset()
			row.invoke(t, ctx, s)
			got := capture.snapshot()
			if len(got) != row.expectCount {
				t.Fatalf("row %s: captured %d filter(s), want exactly %d", row.name, len(got), row.expectCount)
			}
			gotMethods := make([]string, len(got))
			for i, e := range got {
				gotMethods[i] = e.method
			}
			sort.Strings(gotMethods)
			wantMethods := append([]string(nil), row.expectMethods...)
			sort.Strings(wantMethods)
			if !slices.Equal(gotMethods, wantMethods) {
				t.Fatalf("row %s: captured gRPC method multiset = %v, want %v", row.name, gotMethods, wantMethods)
			}
			for _, e := range got {
				keys := walkFilterKeys(t, e.filter)
				if len(keys) == 0 {
					t.Fatalf("row %s: captured %s filter walked to an EMPTY key set — this would make the schema_version absence assertion below vacuous", row.name, e.method)
				}
				if slices.Contains(keys, "category") {
					sawCategoryKey = true
				}
				if slices.Contains(keys, schemaVersionKey) {
					t.Errorf("row %s: captured %s filter's walked key set %v contains %q — schema_version must NEVER gate recall", row.name, e.method, keys, schemaVersionKey)
				}
				totalWalked++
				aggregateMethods[e.method]++
			}
		})
	}

	wantTotal := 0
	for _, row := range recallInvocationRows {
		wantTotal += row.expectCount
	}
	if totalWalked != wantTotal {
		t.Errorf("total filters walked across all rows = %d, want %d (the sum of declared per-row expectations)", totalWalked, wantTotal)
	}
	if totalWalked == 0 {
		t.Fatal("total filters walked is zero — the gate proved nothing")
	}
	t.Logf("total filters walked: %d; aggregate captured-method multiset: %v", totalWalked, aggregateMethods)
	if !sawCategoryKey {
		t.Error("no captured filter's walked key set contained \"category\" — Search/List's Categories option must produce categoryMatchCondition's nested OR group in at least one LIVE-PIPELINE filter, proving the walker's recursion is exercised in production shape, not only by TestFilterWalkerSeesEveryPosition's synthetic controls")
	}
}
