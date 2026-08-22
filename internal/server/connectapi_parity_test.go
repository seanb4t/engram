// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/migrate"
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

// autoFillBaseFillTime anchors the distinct-instant sequence autoFillMemory
// derives every time.Time/*time.Time field from.
var autoFillBaseFillTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

var citationType = reflect.TypeOf([]store.Citation{})

// autoFillMemory is D-06's reflection auto-fill: it walks every VISIBLE
// field of store.Memory and assigns each a type-appropriate, DISTINCTIVE
// non-zero value derived from the field's own name/position, so no two
// fields share a rendered value and a cross-wired mapping cannot pass by
// coincidence. It never builds a hand-keyed store.Memory{...} literal — a
// future field defaults to zero in one and the population assertion below
// would then pass vacuously, which is exactly what D-06 rejects. A field
// kind this filler has no branch for is a t.Fatalf naming the field and its
// type, never a silent zero.
func autoFillMemory(t *testing.T) store.Memory {
	t.Helper()
	var m store.Memory
	v := reflect.ValueOf(&m).Elem()
	typ := v.Type()

	offset := 0
	for _, f := range reflect.VisibleFields(typ) {
		offset++
		fv := v.FieldByIndex(f.Index)
		if !fv.CanSet() {
			continue
		}

		switch {
		case f.Type == reflect.TypeOf(time.Time{}):
			fv.Set(reflect.ValueOf(autoFillBaseFillTime.Add(time.Duration(offset) * time.Second)))
			continue
		case f.Type == reflect.TypeOf(&time.Time{}):
			tv := autoFillBaseFillTime.Add(time.Duration(offset) * time.Second)
			fv.Set(reflect.ValueOf(&tv))
			continue
		case f.Type == citationType:
			// Each of the five Citation subfields gets its own distinct,
			// non-empty literal — distinct from each other AND from every
			// other top-level field's rendering (all of which are prefixed
			// "val-"/derived from a field name, never "citation-...-value")
			// — so a citation subfield cross-wire is observable by the
			// decode-back comparator (task 2's later sub-test).
			fv.Set(reflect.ValueOf([]store.Citation{{
				Kind:    "citation-kind-value",
				Ref:     "citation-ref-value",
				Locator: "citation-locator-value",
				Pin:     "citation-pin-value",
				Excerpt: "citation-excerpt-value",
			}}))
			continue
		}

		switch fv.Kind() {
		case reflect.String:
			fv.SetString("val-" + f.Name)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			fv.SetInt(int64(offset))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			fv.SetUint(uint64(offset))
		case reflect.Float32, reflect.Float64:
			fv.SetFloat(float64(offset) + 0.5)
		case reflect.Bool:
			fv.SetBool(true)
		case reflect.Pointer:
			if fv.Type().Elem().Kind() != reflect.String {
				t.Fatalf("autoFillMemory: field %s has an unbranched pointer-element kind %s (type %s)", f.Name, fv.Type().Elem().Kind(), fv.Type())
			}
			sv := "val-" + f.Name
			fv.Set(reflect.ValueOf(&sv))
		case reflect.Slice:
			if fv.Type().Elem().Kind() != reflect.String {
				t.Fatalf("autoFillMemory: field %s has an unbranched slice-element kind %s (type %s)", f.Name, fv.Type().Elem().Kind(), fv.Type())
			}
			// Tags and Supersedes are the same Go type ([]string); deriving
			// each element from the FIELD's own name keeps the two element
			// SETS disjoint, so a mapping that feeds one from the other is
			// observable rather than passing by coincidence.
			fv.Set(reflect.ValueOf([]string{f.Name + "-elem-a", f.Name + "-elem-b"}))
		default:
			t.Fatalf("autoFillMemory: field %s has an unbranched kind %s (type %s)", f.Name, fv.Kind(), fv.Type())
		}
	}
	return m
}

// canonicalFieldRendering renders a single store.Memory field value to a
// string for the pairwise-distinctness check below. It is deliberately NOT
// exhaustive over every possible Go type — only the kinds autoFillMemory
// itself can produce — because a type this function cannot render is
// exactly as informative a failure (via the default fmt.Sprintf branch,
// still distinguishable across fields by content) as a dedicated branch
// would be for this narrow, closed purpose.
func canonicalFieldRendering(fv reflect.Value) string {
	switch {
	case fv.Type() == reflect.TypeOf(time.Time{}):
		return fv.Interface().(time.Time).Format(time.RFC3339Nano)
	case fv.Type() == reflect.TypeOf(&time.Time{}):
		if fv.IsNil() {
			return "<nil>"
		}
		return fv.Interface().(*time.Time).Format(time.RFC3339Nano)
	case fv.Type() == citationType:
		cs := fv.Interface().([]store.Citation)
		parts := make([]string, 0, len(cs))
		for _, c := range cs {
			parts = append(parts, strings.Join([]string{c.Kind, c.Ref, c.Locator, c.Pin, c.Excerpt}, "|"))
		}
		return strings.Join(parts, ";")
	}
	switch fv.Kind() {
	case reflect.String:
		return fv.String()
	case reflect.Pointer:
		if fv.IsNil() {
			return "<nil>"
		}
		return fmt.Sprintf("%v", fv.Elem().Interface())
	case reflect.Slice:
		parts := make([]string, 0, fv.Len())
		for i := 0; i < fv.Len(); i++ {
			parts = append(parts, fmt.Sprintf("%v", fv.Index(i).Interface()))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprintf("%v", fv.Interface())
	}
}

// assertDecodeBackCoversAllFields is the decode-back comparator's own
// exhaustiveness gate — without it the comparator is invisible to both the
// descriptor-exhaustive name detector and the descriptor-exhaustive Has(fd)
// population loop, since both walk DESCRIPTORS and neither observes what
// the comparator chose to look at. It asserts the sorted set of store json
// names the comparator recorded (one append per comparison, immediately
// adjacent to it, in "values decode back to their source" below) is EXACTLY
// storeJSONVisibleFields(store.Memory) — a comparator covering 25 of 30
// fields fails HERE, on coverage, independently of whether any value it did
// look at was correct. A name recorded twice (masking a field never
// recorded at all) is also rejected.
func assertDecodeBackCoversAllFields(t *testing.T, compared []string) {
	t.Helper()

	got := slices.Clone(compared)
	slices.Sort(got)
	for i := 1; i < len(got); i++ {
		if got[i] == got[i-1] {
			t.Fatalf("assertDecodeBackCoversAllFields: %q was compared more than once", got[i])
		}
	}

	want := storeJSONVisibleFields(reflect.TypeOf(store.Memory{}))
	slices.Sort(want)

	if !slices.Equal(got, want) {
		t.Fatalf("decode-back comparator coverage mismatch: compared %v, want (storeJSONVisibleFields) %v", got, want)
	}
}

func TestConnectMemoryFieldsPopulated(t *testing.T) {
	// The guard that keeps the population assertion itself non-vacuous: a
	// field the filler silently left zero would let a missing memoryToProto
	// mapping pass "every proto field is populated" by coincidence (an
	// implicit-presence zero value is legitimately unpopulated).
	t.Run("auto-fill covers every field", func(t *testing.T) {
		m := autoFillMemory(t)
		v := reflect.ValueOf(m)
		for _, f := range reflect.VisibleFields(v.Type()) {
			jsonTag := f.Tag.Get("json")
			if jsonTag == "" || jsonTag == "-" {
				continue
			}
			fv := v.FieldByIndex(f.Index)
			if fv.IsZero() {
				t.Fatalf("autoFillMemory left json-visible field %s at its zero value", f.Name)
			}
		}
	})

	// Covers EVERY json-visible field, not scalars only (the cycle-1
	// limitation): the two []string fields render to distinct joined forms
	// with disjoint element sets, and the []Citation element's five
	// subfields render jointly as one distinct value. Cross-wiring between
	// Tags/Supersedes and between two citation subfields is exactly what
	// this makes observable.
	t.Run("auto-fill values are pairwise distinct", func(t *testing.T) {
		m := autoFillMemory(t)
		v := reflect.ValueOf(m)
		seen := make(map[string]string)
		for _, f := range reflect.VisibleFields(v.Type()) {
			jsonTag := f.Tag.Get("json")
			if jsonTag == "" || jsonTag == "-" {
				continue
			}
			fv := v.FieldByIndex(f.Index)
			rendered := canonicalFieldRendering(fv)
			if prev, ok := seen[rendered]; ok {
				t.Fatalf("fields %s and %s render identically (%q)", prev, f.Name, rendered)
			}
			seen[rendered] = f.Name
		}
	})

	// Calls memoryToProto DIRECTLY, never the non-full recall shaper further
	// down connectapi.go: that shaper deliberately clears Content/Citations/
	// Kind and rewrites Summary when full=false, which would masquerade as a
	// parity failure. Has(fd) is
	// the "populated, not merely present" predicate — true for every
	// implicit-presence scalar the auto-filled (non-zero) source produces,
	// and true for the three D-14 optional scalars because ANY assignment
	// (including an assigned zero) reports Has()==true; that is why this
	// sub-test alone cannot see a CONDITIONAL assignment (RED PROOF 3
	// below exercises that gap, which only the zero-value sub-test closes).
	t.Run("every proto field is populated", func(t *testing.T) {
		m := autoFillMemory(t)
		msg := memoryToProto(m)
		fields := msg.ProtoReflect().Descriptor().Fields()
		var unpopulated []string
		for i := 0; i < fields.Len(); i++ {
			fd := fields.Get(i)
			if !msg.ProtoReflect().Has(fd) {
				unpopulated = append(unpopulated, string(fd.Name()))
			}
		}
		if len(unpopulated) > 0 {
			slices.Sort(unpopulated)
			t.Fatalf("proto fields not populated: %v", unpopulated)
		}
	})

	// Per D-08 the decode lives in the assertion body — no named production
	// proto->store mapper is introduced, since a second production mapping
	// call site would be free to drift from memoryToProto unobserved. This
	// sub-test carries its own exhaustiveness gate (assertDecodeBackCoversAllFields):
	// without it a comparator covering 25 of 30 fields is invisible to both
	// the name detector and the Has(fd) loop above, since both are
	// exhaustive over DESCRIPTORS and neither observes what THIS comparator
	// chose to look at.
	t.Run("values decode back to their source", func(t *testing.T) {
		m := autoFillMemory(t)
		msg := memoryToProto(m)
		compared := make([]string, 0, 30)

		if msg.Id != m.ID {
			t.Errorf("id: got %q want %q", msg.Id, m.ID)
		}
		compared = append(compared, "id")

		if msg.ShortId != m.ShortID {
			t.Errorf("short_id: got %q want %q", msg.ShortId, m.ShortID)
		}
		compared = append(compared, "short_id")

		if msg.Content != m.Content {
			t.Errorf("content: got %q want %q", msg.Content, m.Content)
		}
		compared = append(compared, "content")

		if msg.Scope != m.Scope {
			t.Errorf("scope: got %q want %q", msg.Scope, m.Scope)
		}
		compared = append(compared, "scope")

		if msg.Repo != m.Repo {
			t.Errorf("repo: got %q want %q", msg.Repo, m.Repo)
		}
		compared = append(compared, "repo")

		if msg.Workspace != m.Workspace {
			t.Errorf("workspace: got %q want %q", msg.Workspace, m.Workspace)
		}
		compared = append(compared, "workspace")

		if msg.Worktree != m.Worktree {
			t.Errorf("worktree_path (proto worktree): got %q want %q", msg.Worktree, m.Worktree)
		}
		compared = append(compared, "worktree_path")

		if msg.BaseDir != m.BaseDir {
			t.Errorf("base_dir: got %q want %q", msg.BaseDir, m.BaseDir)
		}
		compared = append(compared, "base_dir")

		if msg.Source != m.Source {
			t.Errorf("source: got %q want %q", msg.Source, m.Source)
		}
		compared = append(compared, "source")

		if msg.Category != m.Category {
			t.Errorf("category: got %q want %q", msg.Category, m.Category)
		}
		compared = append(compared, "category")

		if !slices.Equal(msg.Tags, m.Tags) {
			t.Errorf("tags: got %v want %v", msg.Tags, m.Tags)
		}
		compared = append(compared, "tags")

		if msg.Actor != m.Actor {
			t.Errorf("actor: got %q want %q", msg.Actor, m.Actor)
		}
		compared = append(compared, "actor")

		if msg.Owner != m.Owner {
			t.Errorf("owner: got %q want %q", msg.Owner, m.Owner)
		}
		compared = append(compared, "owner")

		if msg.Visibility != m.Visibility {
			t.Errorf("visibility: got %q want %q", msg.Visibility, m.Visibility)
		}
		compared = append(compared, "visibility")

		if msg.CreatedAt == nil || !msg.CreatedAt.AsTime().Equal(m.CreatedAt) {
			t.Errorf("created_at: got %v want %v", msg.CreatedAt, m.CreatedAt)
		}
		compared = append(compared, "created_at")

		if m.NotBefore == nil || msg.NotBefore == nil || !msg.NotBefore.AsTime().Equal(*m.NotBefore) {
			t.Errorf("not_before: got %v want %v", msg.NotBefore, m.NotBefore)
		}
		compared = append(compared, "not_before")

		if m.NotAfter == nil || msg.NotAfter == nil || !msg.NotAfter.AsTime().Equal(*m.NotAfter) {
			t.Errorf("not_after: got %v want %v", msg.NotAfter, m.NotAfter)
		}
		compared = append(compared, "not_after")

		if !slices.Equal(msg.Supersedes, m.Supersedes) {
			t.Errorf("supersedes: got %v want %v", msg.Supersedes, m.Supersedes)
		}
		compared = append(compared, "supersedes")

		if m.SupersededBy == nil || msg.SupersededBy == nil || *msg.SupersededBy != *m.SupersededBy {
			t.Errorf("superseded_by: got %v want %v", msg.SupersededBy, m.SupersededBy)
		}
		compared = append(compared, "superseded_by")

		if m.ArchivedAt == nil || msg.ArchivedAt == nil || !msg.ArchivedAt.AsTime().Equal(*m.ArchivedAt) {
			t.Errorf("archived_at: got %v want %v", msg.ArchivedAt, m.ArchivedAt)
		}
		compared = append(compared, "archived_at")

		if msg.AccessCount != m.AccessCount {
			t.Errorf("access_count: got %d want %d", msg.AccessCount, m.AccessCount)
		}
		compared = append(compared, "access_count")

		if m.LastAccessedAt == nil || msg.LastAccessedAt == nil || !msg.LastAccessedAt.AsTime().Equal(*m.LastAccessedAt) {
			t.Errorf("last_accessed_at: got %v want %v", msg.LastAccessedAt, m.LastAccessedAt)
		}
		compared = append(compared, "last_accessed_at")

		if msg.Kind != m.Kind {
			t.Errorf("kind: got %q want %q", msg.Kind, m.Kind)
		}
		compared = append(compared, "kind")

		if len(msg.Citations) != len(m.Citations) {
			t.Fatalf("citations: got %d entries want %d", len(msg.Citations), len(m.Citations))
		}
		for i, c := range m.Citations {
			pc := msg.Citations[i]
			if pc.Kind != c.Kind || pc.Ref != c.Ref || pc.Locator != c.Locator || pc.Pin != c.Pin || pc.Excerpt != c.Excerpt {
				t.Errorf("citations[%d]: got %+v want %+v", i, pc, c)
			}
		}
		compared = append(compared, "citations")

		if msg.Summary != m.Summary {
			t.Errorf("summary: got %q want %q", msg.Summary, m.Summary)
		}
		compared = append(compared, "summary")

		if msg.SummarySource != string(m.SummarySource) {
			t.Errorf("summary_source: got %q want %q", msg.SummarySource, m.SummarySource)
		}
		compared = append(compared, "summary_source")

		if msg.SummaryModel == nil {
			t.Errorf("summary_model: got nil want %q", m.SummaryModel)
		} else if *msg.SummaryModel != m.SummaryModel {
			t.Errorf("summary_model: got %q want %q", *msg.SummaryModel, m.SummaryModel)
		}
		compared = append(compared, "summary_model")

		if msg.SummaryEgressAt == nil || !msg.SummaryEgressAt.AsTime().Equal(m.SummaryEgressAt) {
			t.Errorf("summary_egress_at: got %v want %v", msg.SummaryEgressAt, m.SummaryEgressAt)
		}
		compared = append(compared, "summary_egress_at")

		if msg.Score != m.Score {
			t.Errorf("score: got %v want %v", msg.Score, m.Score)
		}
		compared = append(compared, "score")

		if msg.SchemaVersion == nil || migrate.Version(*msg.SchemaVersion) != m.SchemaVersion {
			t.Errorf("schema_version: got %v want %v", msg.SchemaVersion, m.SchemaVersion)
		}
		compared = append(compared, "schema_version")

		assertDecodeBackCoversAllFields(t, compared)
	})

	t.Run("supersedes preserves order", func(t *testing.T) {
		m := autoFillMemory(t)
		msg := memoryToProto(m)
		if !slices.Equal(msg.Supersedes, m.Supersedes) {
			t.Fatalf("supersedes order mismatch: got %v want %v", msg.Supersedes, m.Supersedes)
		}
		// Tags and Supersedes are the same Go type with disjoint element
		// sets (autoFillMemory); a swap between them is directly observable
		// here.
		if slices.Equal(msg.Tags, m.Supersedes) {
			t.Fatal("proto tags equals source supersedes — a Tags/Supersedes cross-wire would pass undetected")
		}
	})

	// Non-mutation only (review cycle 1, LOW): memoryToProto deliberately
	// assigns slices directly (Tags: m.Tags), so the proto message and the
	// source share backing arrays by design — this sub-test does not, and
	// must not, assert isolation.
	t.Run("memoryToProto does not mutate its input", func(t *testing.T) {
		m := autoFillMemory(t)
		pre := m
		msg1 := memoryToProto(m)
		msg2 := memoryToProto(m)
		if !proto.Equal(msg1, msg2) {
			t.Fatal("memoryToProto is non-deterministic across two calls on the same source")
		}
		if !reflect.DeepEqual(m, pre) {
			t.Fatal("memoryToProto mutated its input")
		}
	})

	// D-14 §3: this is the ONLY place in the phase that observes the
	// assign-always requirement on schema_version/summary_model, spelled at
	// the pointer/Has(fd) level — never through GetSchemaVersion()/
	// GetSummaryModel()/GetSupersededBy(), whose value-typed, nil-safe
	// returns report the same thing for "assigned zero" and "never
	// assigned", which is precisely the state pair this sub-test exists to
	// separate.
	t.Run("zero-value source: timestamps unset, optional scalars still assigned", func(t *testing.T) {
		msg := memoryToProto(store.Memory{})

		if msg.NotBefore != nil {
			t.Errorf("not_before: got %v, want nil", msg.NotBefore)
		}
		if msg.NotAfter != nil {
			t.Errorf("not_after: got %v, want nil", msg.NotAfter)
		}
		if msg.ArchivedAt != nil {
			t.Errorf("archived_at: got %v, want nil", msg.ArchivedAt)
		}
		if msg.SummaryEgressAt != nil {
			t.Errorf("summary_egress_at: got %v, want nil", msg.SummaryEgressAt)
		}
		if msg.Supersedes != nil {
			t.Errorf("supersedes: got %v, want nil (not an empty non-nil slice)", msg.Supersedes)
		}

		fields := msg.ProtoReflect().Descriptor().Fields()
		supersededByFD := fields.ByName("superseded_by")
		schemaVersionFD := fields.ByName("schema_version")
		summaryModelFD := fields.ByName("summary_model")

		if msg.SupersededBy != nil {
			t.Errorf("superseded_by pointer: got %v, want nil", msg.SupersededBy)
		}
		if msg.ProtoReflect().Has(supersededByFD) {
			t.Error("superseded_by: Has() reports true for a zero-value source, want false (mirrors the nil source pointer)")
		}

		if msg.SchemaVersion == nil {
			t.Fatal("schema_version pointer: got nil, want non-nil (assigned zero)")
		}
		if *msg.SchemaVersion != 0 {
			t.Errorf("schema_version: got %d, want 0", *msg.SchemaVersion)
		}
		if !msg.ProtoReflect().Has(schemaVersionFD) {
			t.Error("schema_version: Has() reports false for a zero-value source, want true (D-14 §3 assign-always)")
		}

		if msg.SummaryModel == nil {
			t.Fatal("summary_model pointer: got nil, want non-nil (assigned empty string)")
		}
		if *msg.SummaryModel != "" {
			t.Errorf("summary_model: got %q, want \"\"", *msg.SummaryModel)
		}
		if !msg.ProtoReflect().Has(summaryModelFD) {
			t.Error("summary_model: Has() reports false for a zero-value source, want true (D-14 §3 assign-always)")
		}
	})
}
