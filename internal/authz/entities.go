// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authz

import (
	"github.com/cedar-policy/cedar-go"
)

// Entity type names. Namespaced to match schema.json's Engram namespace
// (reference-only, D-06) even though the embedded policies never compare on
// resource/principal TYPE — they are purely attribute-based (Policy
// Reconciliation, Correction 1), so these names are documentation, not a
// runtime constraint.
const (
	principalEntityType = cedar.EntityType("Engram::Principal")
	memoryEntityType    = cedar.EntityType("Engram::Memory")
	actionEntityType    = cedar.EntityType("Action")
)

// visibilityShared is the Memory.visibility value that makes shared_read
// eligible to match (mirrors internal/store's visibilityShared constant —
// internal/authz never imports internal/store, so the value is duplicated
// here as a plain string literal, not a shared symbol).
const visibilityShared = "shared"

// sharedProbeOwner is a reserved sentinel owner used to build the canonical
// BucketShared probe resource (DecideBucket). It is distinct from any real
// owner-claim value and from the empty-owner anonymous bucket ("") by
// construction — a NUL-prefixed string can never be a real owner claim — so
// own_records never matches the shared probe, and defense_empty_owner's
// resource-has-non-empty-owner guard behaves correctly for it. Mirrors
// internal/store's matchNothing sentinel-condition style.
const sharedProbeOwner = "\x00shared-bucket-probe"

// probeResourceID is the fixed entity ID used for every Memory/probe resource
// this package builds. Cedar policies here never reference resource identity
// (only attributes — resource.owner, resource.visibility), so a stable
// placeholder ID is sufficient; a fresh EntityMap is built per Decide call, so
// there is no cross-call collision risk.
const probeResourceID = "probe"

// principalEntity builds the Principal entity for a request. tenant/roles are
// intentionally OMITTED (not set as attributes at all, not set-to-empty) and
// Parents is an empty set — so the has-guarded tenant_isolate policy stays
// vacuous and SC5's forward-compat reservation holds with no breaking schema
// change when a later milestone starts populating them.
func principalEntity(owner, kind string) (cedar.EntityUID, cedar.Entity) {
	uid := cedar.NewEntityUID(principalEntityType, cedar.String(owner))
	attrs := cedar.NewRecord(cedar.RecordMap{
		"owner": cedar.String(owner),
		"kind":  cedar.String(kind),
	})
	return uid, cedar.Entity{
		UID:        uid,
		Parents:    cedar.NewEntityUIDSet(),
		Attributes: attrs,
	}
}

// memoryEntity builds the Memory/bucket-probe resource entity from primitive
// attribute values. tenant is intentionally OMITTED — see principalEntity.
func memoryEntity(owner, category, visibility, scope string) (cedar.EntityUID, cedar.Entity) {
	uid := cedar.NewEntityUID(memoryEntityType, cedar.String(probeResourceID))
	attrs := cedar.NewRecord(cedar.RecordMap{
		"owner":      cedar.String(owner),
		"category":   cedar.String(category),
		"visibility": cedar.String(visibility),
		"scope":      cedar.String(scope),
	})
	return uid, cedar.Entity{
		UID:        uid,
		Parents:    cedar.NewEntityUIDSet(),
		Attributes: attrs,
	}
}

// actionEntityUID builds the Action entity UID a Request references.
func actionEntityUID(action Action) cedar.EntityUID {
	return cedar.NewEntityUID(actionEntityType, cedar.String(action))
}
