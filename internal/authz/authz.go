// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authz

import (
	"github.com/cedar-policy/cedar-go"
)

// PDP is a policy decision point: an immutable wrapper over the parsed Cedar
// policy corpus. It holds no mutable state after construction, so a single
// *PDP is safe to share across concurrent DecideBucket/DecideRecord calls —
// Decide is a pure function of (principal, action, resource).
type PDP struct {
	policies *cedar.PolicySet
}

// Action is the engram authorization verb vocabulary. The full verb list
// ships from day one (D-05) so later ABAC phases add policies, never actions.
type Action string

// The five engram authorization actions.
const (
	ActionRead     Action = "read"
	ActionWrite    Action = "write"
	ActionDelete   Action = "delete"
	ActionShare    Action = "share"
	ActionSchedule Action = "schedule"
)

// Bucket names an enumerable candidate set the store's bulk recall paths
// (Search/List/etc.) filter over. DecideBucket answers "is this bucket
// reachable for this action," never "is record X reachable" — bucket-level
// decisions only, never per-record on the hot recall path (D-03).
type Bucket int

// The two engram buckets: records the principal owns, and records any
// authenticated caller may read via shared visibility.
const (
	BucketOwn Bucket = iota
	BucketShared
)

// Decision is the result of a policy evaluation. Allow is the only field
// callers should branch on; diag (the raw cedar.Diagnostic) is unexported and
// never surfaced to a caller-facing error (D-10) — it exists solely for
// future debug-level logging / OTel span attachment by internal/store.
type Decision struct {
	Allow bool
	diag  cedar.Diagnostic
}

// DecideRecord answers a single per-record authorization decision for the
// id-addressed gate path (GetReadable/getWritable/OwnedOrAbsent, after the
// record has already been fetched). It is off the hot bulk-recall path, so
// one cedar.Authorize call per invocation is the correct granularity (D-03).
func (p *PDP) DecideRecord(owner, kind string, action Action, memoryOwner, category, visibility, scope string) Decision {
	principalUID, principalEnt := principalEntity(owner, kind)
	resourceUID, resourceEnt := memoryEntity(memoryOwner, category, visibility, scope)
	entities := cedar.EntityMap{
		principalUID: principalEnt,
		resourceUID:  resourceEnt,
	}
	req := cedar.Request{
		Principal: principalUID,
		Action:    actionEntityUID(action),
		Resource:  resourceUID,
	}
	decision, diag := cedar.Authorize(p.policies, entities, req)
	return Decision{Allow: bool(decision), diag: diag}
}

// DecideBucket answers whether the given bucket is reachable for this
// principal+action, via a canonical per-bucket probe resource routed through
// the SAME cedar.Authorize path DecideRecord uses (one policy corpus, two
// call shapes). BucketOwn probes with the principal's own owner (so
// resource.owner == principal.owner naturally matches — the anonymous
// owner=="" case matches ""=="" and stays reachable, D-11). BucketShared
// probes with a reserved sentinel owner distinct from any real owner and
// from "" (mirroring store.go's matchNothing sentinel style), so shared_read
// matches for a non-empty-owner principal only and own_records never matches
// the shared probe.
func (p *PDP) DecideBucket(owner, kind string, action Action, bucket Bucket) Decision {
	switch bucket {
	case BucketOwn:
		return p.DecideRecord(owner, kind, action, owner, "", "", "")
	case BucketShared:
		return p.DecideRecord(owner, kind, action, sharedProbeOwner, "", visibilityShared, "")
	default:
		// Unknown bucket value — a programming error, never a silent allow.
		return Decision{Allow: false}
	}
}
