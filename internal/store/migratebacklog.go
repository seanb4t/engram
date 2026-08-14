// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"fmt"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/migrate"
)

// backlogFilter derives the set of records still below target: those whose
// stored schema_version is strictly less than target, OR whose
// schema_version key is genuinely ABSENT (every pre-migration record, as
// written before Phase 2 shipped). Qdrant evaluates a Range condition on an
// ABSENT key as FALSE, not as "below everything" — so the Range arm alone
// cannot see a pre-migration record. That is this phase's single
// highest-risk line: without the IsEmpty arm, the derived backlog would
// contain only records that already carry an explicit below-target
// schema_version, the sweep would converge to zero having migrated
// nothing, and every existing test would still be green.
//
// The two conditions are combined in the NESTED form activeWindowConditions
// (store.go:1011-1018) already establishes as this repo's in-repo
// precedent for exactly this shape: a top-level Must holding exactly one
// filter-as-condition wrapper (see the call below) whose inner Should holds
// the Range and IsEmpty conditions. A bare top-level Should with no Must
// has no in-repo precedent anywhere in this file and would rest on an
// external reading of whether Qdrant's server treats an unwrapped Should as
// a hard restriction or merely a ranking hint; the nested form removes that
// dependency at zero cost. Flattening the two conditions directly under
// Must instead would AND them together and match nothing — do not do that
// either.
//
// NewIsEmpty's actual semantics are three-valued: it matches a MISSING
// key, a null value, AND an empty array — not only a missing key. That is
// a slightly broader predicate than "pre-Phase-2 legacy record" (a record
// carrying an explicit schema_version: null also matches, and genuinely
// does need migrating, so the breadth is harmless and arguably desirable
// here), matching ownerlessFilter (store.go:2631-2636) and
// missingShortIDFilter (store.go:2726-2731), whose doc comments already
// state exactly this caveat for their own IsEmpty arms.
//
// backlogFilter is operator-tier only and is never reachable from any
// recall entry point — Search/List/SearchDiscovery/ListScheduled all apply
// activeWindowConditions or no temporal gate at all, never this filter —
// so Phase 2's TestSchemaVersionNeverGatesRecall reachability derivation
// continues to hold with this filter's addition.
//
// backlogFilter ALONE is deliberately broad at target == 0: the IsEmpty
// arm matches every legacy record regardless of target, so a caller that
// builds this filter at target 0 and scrolls it directly gets the WHOLE
// legacy collection back — correct behavior for a filter considered in
// isolation. PA-4's safety property (a v0 target requires no work) is NOT
// owned by this function; it is owned by Store.Migrate's target<=0
// short-circuit, which returns before this filter is ever built.
func backlogFilter(target migrate.Version) *qdrant.Filter {
	return &qdrant.Filter{
		Must: []*qdrant.Condition{
			qdrant.NewFilterAsCondition(&qdrant.Filter{
				Should: []*qdrant.Condition{
					qdrant.NewRange(schemaVersionKey, &qdrant.Range{Lt: qdrant.PtrOf(float64(target))}),
					qdrant.NewIsEmpty(schemaVersionKey),
				},
			}),
		},
	}
}

// versionOf reads a record's schema_version from its raw stored payload
// exactly as fromPayload does (store.go:741-743): an absent key returns
// migrate.Version(0), per Version's own "the zero value IS v0 IS absent"
// doc comment — no pointer indirection is needed to distinguish "unset"
// from "v0" because they are defined to be the same state.
func versionOf(p map[string]*qdrant.Value) migrate.Version {
	if v, ok := p[schemaVersionKey]; ok {
		return migrate.Version(v.GetIntegerValue())
	}
	return migrate.Version(0)
}

// payloadToMap converts a raw stored Qdrant payload into the
// map[string]any an ApplyFunc consumes. It type-switches on each value's
// oneof kind and handles all SEVEN qdrant.Value variants BY NAME —
// NullValue, DoubleValue, IntegerValue, StringValue, BoolValue,
// StructValue, ListValue, verified at plan time against the pinned
// go-client@v1.18.3 json_with_int.pb.go oneof — recursing for the struct
// and list variants. An unrecognized kind returns an error naming the key
// and the Go type — never a silent drop, never a zero value — because an
// eighth variant introduced by a future client upgrade is a hole a
// maintainer must consciously fill, not one that should quietly corrupt a
// migrated record.
func payloadToMap(p map[string]*qdrant.Value) (map[string]any, error) {
	out := make(map[string]any, len(p))
	for k, v := range p {
		conv, err := valueToAny(k, v)
		if err != nil {
			return nil, err
		}
		out[k] = conv
	}
	return out, nil
}

// valueToAny converts one *qdrant.Value at key (used only to build a
// readable error path for a nested struct/list value) into its Go
// equivalent.
func valueToAny(key string, v *qdrant.Value) (any, error) {
	if v == nil {
		return nil, nil
	}
	switch kind := v.GetKind().(type) {
	case *qdrant.Value_NullValue:
		return nil, nil
	case *qdrant.Value_DoubleValue:
		return v.GetDoubleValue(), nil
	case *qdrant.Value_IntegerValue:
		return v.GetIntegerValue(), nil
	case *qdrant.Value_StringValue:
		return v.GetStringValue(), nil
	case *qdrant.Value_BoolValue:
		return v.GetBoolValue(), nil
	case *qdrant.Value_StructValue:
		fields := v.GetStructValue().GetFields()
		out := make(map[string]any, len(fields))
		for fk, fv := range fields {
			conv, err := valueToAny(key+"."+fk, fv)
			if err != nil {
				return nil, err
			}
			out[fk] = conv
		}
		return out, nil
	case *qdrant.Value_ListValue:
		vals := v.GetListValue().GetValues()
		out := make([]any, len(vals))
		for i, lv := range vals {
			conv, err := valueToAny(fmt.Sprintf("%s[%d]", key, i), lv)
			if err != nil {
				return nil, err
			}
			out[i] = conv
		}
		return out, nil
	default:
		return nil, fmt.Errorf("migrate: payloadToMap: key %q has unrecognized qdrant.Value kind %T", key, kind)
	}
}
