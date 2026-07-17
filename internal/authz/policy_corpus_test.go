// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authz

import (
	"testing"

	"github.com/cedar-policy/cedar-go"
)

// allActions is the full D-05 verb list, used by fixtures that must assert
// across every action rather than a single one.
var allActions = []Action{ActionRead, ActionWrite, ActionDelete, ActionShare, ActionSchedule}

// TestPolicyCorpus_OwnRecordAllow is a PERMANENT D-08 regression test: it
// parses the SAME embedded .cedar bytes (via MustDefault, not a mock) and
// asserts a principal is Allow for every action on a record it owns.
func TestPolicyCorpus_OwnRecordAllow(t *testing.T) {
	pdp := MustDefault()
	for _, action := range allActions {
		got := pdp.DecideRecord("alice", "human", action, "alice", "note", "private", "")
		if !got.Allow {
			t.Fatalf("DecideRecord(owner=alice, action=%s, resource.owner=alice) = Deny, want Allow", action)
		}
	}
}

// TestPolicyCorpus_SharedReadOnly is the direct backstop for Pitfall 3 (an
// over-broad shared-read policy silently granting write/delete/share/
// schedule): asserts read Allow AND every other action Deny against the real
// embedded policy text (DEC-kyz).
func TestPolicyCorpus_SharedReadOnly(t *testing.T) {
	pdp := MustDefault()
	if got := pdp.DecideRecord("alice", "human", ActionRead, "bob", "note", "shared", ""); !got.Allow {
		t.Fatalf("DecideRecord(owner=alice, action=read, resource=bob/shared) = Deny, want Allow")
	}
	for _, action := range []Action{ActionWrite, ActionDelete, ActionShare, ActionSchedule} {
		if got := pdp.DecideRecord("alice", "human", action, "bob", "note", "shared", ""); got.Allow {
			t.Fatalf("DecideRecord(owner=alice, action=%s, resource=bob/shared) = Allow, want Deny (DEC-kyz)", action)
		}
	}
}

// TestPolicyCorpus_CrossOwnerWriteDeny asserts a principal cannot write
// another owner's private record.
func TestPolicyCorpus_CrossOwnerWriteDeny(t *testing.T) {
	pdp := MustDefault()
	got := pdp.DecideRecord("alice", "human", ActionWrite, "bob", "note", "private", "")
	if got.Allow {
		t.Fatalf("DecideRecord(owner=alice, action=write, resource=bob/private) = Allow, want Deny")
	}
}

// TestPolicyCorpus_EmptyOwnerDenyAll is the direct backstop for Pitfall 4 (the
// milestone's #1 risk): a synthetic Principal with owner=="" must be Deny
// across the FULL action set on every non-empty-owner resource, both private
// and shared.
func TestPolicyCorpus_EmptyOwnerDenyAll(t *testing.T) {
	pdp := MustDefault()
	resources := []struct {
		name              string
		owner, visibility string
	}{
		{"private-other-owner", "bob", "private"},
		{"shared-other-owner", "bob", "shared"},
	}
	for _, r := range resources {
		for _, action := range allActions {
			got := pdp.DecideRecord("", "human", action, r.owner, "note", r.visibility, "")
			if got.Allow {
				t.Fatalf("DecideRecord(owner=\"\", action=%s, resource=%s) = Allow, want Deny", action, r.name)
			}
		}
	}
}

// TestPolicyCorpus_AnonOwnBucketReachable proves the scoped defense-in-depth
// forbid does NOT collaterally deny the legitimate anonymous owner=="" bucket
// (D-11) — the companion assertion to EmptyOwnerDenyAll.
func TestPolicyCorpus_AnonOwnBucketReachable(t *testing.T) {
	pdp := MustDefault()
	for _, action := range []Action{ActionRead, ActionWrite} {
		got := pdp.DecideRecord("", "human", action, "", "note", "private", "")
		if !got.Allow {
			t.Fatalf("DecideRecord(owner=\"\", action=%s, resource.owner=\"\") = Deny, want Allow", action)
		}
	}
}

// TestPolicyCorpus_ForbidOverridesPermit proves cedar-go's forbid-wins
// semantics through this package's own DecideRecord plumbing (edge 3):
// a request a permit-all policy would allow is still Deny once a forbid-all
// policy also matches, regardless of Add order. This is the load-bearing
// invariant defense_empty_owner's whole design depends on.
func TestPolicyCorpus_ForbidOverridesPermit(t *testing.T) {
	const permitAllCedar = `permit (principal, action, resource);`
	const forbidAllCedar = `forbid (principal, action, resource);`

	var permitAll, forbidAll cedar.Policy
	if err := permitAll.UnmarshalCedar([]byte(permitAllCedar)); err != nil {
		t.Fatalf("parse permit-all: %v", err)
	}
	if err := forbidAll.UnmarshalCedar([]byte(forbidAllCedar)); err != nil {
		t.Fatalf("parse forbid-all: %v", err)
	}

	ps := cedar.NewPolicySet()
	ps.Add("permit-all", &permitAll)
	ps.Add("forbid-all", &forbidAll)
	pdp := &PDP{policies: ps}

	got := pdp.DecideRecord("alice", "human", ActionRead, "alice", "note", "private", "")
	if got.Allow {
		t.Fatalf("DecideRecord with a matching permit AND a matching forbid = Allow, want Deny (forbid must win)")
	}
}
