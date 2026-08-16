// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
)

// storeJSONVisibleFields returns the json tag name (before the comma) for
// every VISIBLE field of t that carries a json tag other than "-" — mirrors
// jsonschemaExposedFields (surfaces_test.go) exactly, the established idiom
// for deriving a wire-visible field-name set from struct reflection rather
// than a hand-maintained list. reflect.VisibleFields (not a shallow
// t.NumField() walk) is required so a future embedded field would be seen
// exactly as it would be promoted.
func storeJSONVisibleFields(t reflect.Type) []string {
	var out []string
	for _, f := range reflect.VisibleFields(t) {
		jsonTag := f.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		name := strings.SplitN(jsonTag, ",", 2)[0]
		if name == "" {
			continue
		}
		out = append(out, name)
	}
	return out
}

// storeToProtoFieldAlias is a RENAME RECORD, not an exemption list: it
// records the sole store->proto json-name divergence across the 30
// json-visible store.Memory fields (Worktree's json:"worktree_path" versus
// the proto field's "worktree"). A store field whose name is neither found
// verbatim among the proto field names nor present as a key here is a
// parity FAILURE — the fix is a mapping or a json:"-" exclusion on the store
// struct, never a new entry here (T-05-05). This map's WIDTH is asserted
// below by whole-map equality (both width and content in one check): a
// second entry would convert a missing field into an "accepted rename" the
// name detector then resolves successfully, and without that width
// assertion the widening would leave no trace.
var storeToProtoFieldAlias = map[string]string{
	"worktree_path": "worktree",
}

// unmappedStoreFields is THE shared detector, declared exactly once in this
// package: for every json-visible field of storeType, resolve its name
// through storeToProtoFieldAlias if present, then require the resolved name
// to appear, byte-for-byte, among desc's field names via a plain map lookup.
// No case folding, no partial matching of any kind is applied on either
// side — a name that merely touches a real proto name (shared prefix,
// shared substring, differing case) and is neither an exact match nor an
// alias is reported unmapped, never silently paired. The returned list is
// sorted so a multi-field failure message is stable across runs; nil/empty
// means every field is mapped.
func unmappedStoreFields(storeType reflect.Type, desc protoreflect.MessageDescriptor) []string {
	protoNames := make(map[string]struct{}, desc.Fields().Len())
	for i := 0; i < desc.Fields().Len(); i++ {
		protoNames[string(desc.Fields().Get(i).Name())] = struct{}{}
	}

	var unmapped []string
	for _, name := range storeJSONVisibleFields(storeType) {
		resolved := name
		if alias, ok := storeToProtoFieldAlias[name]; ok {
			resolved = alias
		}
		if _, ok := protoNames[resolved]; !ok {
			unmapped = append(unmapped, name)
		}
	}
	slices.Sort(unmapped)
	return unmapped
}

// negativeFixtureMemory is the permanent negative fixture (k000pn14qp
// discipline): it exists solely so unmappedStoreFields' ability to REJECT is
// re-proven on every CI run. An anti-vacuity guard must prove the detector
// CAN fail, not merely that it produces rows; deleting, renaming out of the
// run, or t.Skip-gating this fixture silently converts the parity test into
// a gate that can only ever pass. Two of its three fields carry json names
// that DO exist on engramv1.Memory (so the fixture is not trivially
// all-unmapped); the third carries a json name that cannot exist on the
// proto message.
type negativeFixtureMemory struct {
	ID                        string `json:"id"`
	Content                   string `json:"content"`
	DeliberatelyUnmappedField string `json:"deliberately_unmapped_field"`
}

// nearMissFixtureMemory proves unmappedStoreFields applies exact byte
// equality only. None of these three json names exist verbatim among
// engramv1.Memory's field names — each is only a prefix, a substring, or a
// case-different variant of the real proto field name "content" — so every
// one of them must be reported unmapped rather than silently paired.
type nearMissFixtureMemory struct {
	PrefixOfContent    string `json:"contentx"`
	ContainsContent    string `json:"xcontentx"`
	CaseVariantContent string `json:"CONTENT"`
}

func TestConnectMemoryParityDetector(t *testing.T) {
	storeType := reflect.TypeOf(store.Memory{})
	desc := (&engramv1.Memory{}).ProtoReflect().Descriptor()

	// This sub-test exists because the field accounting in "walker accounts
	// for every visible field" below is only meaningful while store.Memory
	// is flat: reflect.VisibleFields and NumField diverge the moment an
	// embedded field appears — exactly the "future addition" case this
	// detector advertises that it covers. On failure: keep counting over
	// VisibleFields and re-derive the exclusion count for the promoted
	// fields; do not revert to NumField.
	t.Run("store.Memory is flat", func(t *testing.T) {
		visible := reflect.VisibleFields(storeType)
		if len(visible) != storeType.NumField() {
			t.Fatalf("store.Memory has %d visible fields but %d direct fields — an embedded field has appeared; re-derive the json:\"-\" exclusion accounting over VisibleFields for the promoted fields", len(visible), storeType.NumField())
		}
		for _, f := range visible {
			if f.Anonymous || len(f.Index) > 1 {
				t.Fatalf("store.Memory field %s is anonymous/promoted — the struct is no longer flat", f.Name)
			}
		}
	})

	// The anti-vacuity guard for the empty case: an empty or
	// silently-truncated walk cannot register as a pass. Both terms are
	// counted over VisibleFields, never mixed with NumField.
	t.Run("walker accounts for every visible field", func(t *testing.T) {
		visible := reflect.VisibleFields(storeType)
		jsonVisible := storeJSONVisibleFields(storeType)
		if len(jsonVisible) == 0 {
			t.Fatal("storeJSONVisibleFields returned an empty list")
		}
		dashCount := 0
		for _, f := range visible {
			if f.Tag.Get("json") == "-" {
				dashCount++
			}
		}
		if len(jsonVisible)+dashCount != len(visible) {
			t.Fatalf("json-visible (%d) + json:\"-\" (%d) = %d, want %d (len(VisibleFields))",
				len(jsonVisible), dashCount, len(jsonVisible)+dashCount, len(visible))
		}
	})

	// Pins WIDTH and CONTENT in one assertion (T-05-05). A source-occurrence
	// count cannot do this: adding a second entry leaves every such count
	// satisfied while silently converting this rename record into an
	// exemption list.
	t.Run("alias map is exactly one entry", func(t *testing.T) {
		want := map[string]string{"worktree_path": "worktree"}
		if !maps.Equal(storeToProtoFieldAlias, want) {
			t.Fatalf("storeToProtoFieldAlias = %v, want %v", storeToProtoFieldAlias, want)
		}
	})

	t.Run("every json-visible store field is mapped", func(t *testing.T) {
		got := unmappedStoreFields(storeType, desc)
		if len(got) != 0 {
			t.Fatalf("unmapped store.Memory fields: %v", got)
		}
	})

	t.Run("permanent negative fixture is rejected", func(t *testing.T) {
		got := unmappedStoreFields(reflect.TypeOf(negativeFixtureMemory{}), desc)
		if len(got) != 1 || got[0] != "deliberately_unmapped_field" {
			t.Fatalf("got %v, want exactly [deliberately_unmapped_field]", got)
		}
	})

	t.Run("near-miss names are not fuzzily paired", func(t *testing.T) {
		got := unmappedStoreFields(reflect.TypeOf(nearMissFixtureMemory{}), desc)
		want := []string{"CONTENT", "contentx", "xcontentx"}
		if !slices.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("failure list is deterministic", func(t *testing.T) {
		got1 := unmappedStoreFields(reflect.TypeOf(nearMissFixtureMemory{}), desc)
		got2 := unmappedStoreFields(reflect.TypeOf(nearMissFixtureMemory{}), desc)
		if !slices.Equal(got1, got2) {
			t.Fatalf("non-deterministic result across two calls: %v vs %v", got1, got2)
		}
	})
}
