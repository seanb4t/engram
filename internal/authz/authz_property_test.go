// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authz

import (
	"testing"

	"pgregory.net/rapid"
)

// modelAllows is the memory contract's isolation rule: the owner reads and
// writes, another authenticated caller reads a shared record and nothing
// else, and an anonymous caller reaches only the ownerless bucket.
func modelAllows(principal, recordOwner, visibility string, action Action) bool {
	if principal == "" {
		return recordOwner == ""
	}
	return principal == recordOwner || (action == ActionRead && visibility == "shared")
}

func TestPropertyAuthz(t *testing.T) {
	pdp := MustDefault()
	owners := []string{"", "alice", "bob"}
	actions := []Action{ActionRead, ActionWrite, ActionDelete, ActionShare, ActionSchedule, ActionArchive}
	rapid.Check(t, func(t *rapid.T) {
		principal := rapid.SampledFrom(owners).Draw(t, "principal")
		kind := rapid.SampledFrom([]string{"human", "agent"}).Draw(t, "kind")
		recordOwner := rapid.SampledFrom(owners).Draw(t, "record_owner")
		visibility := rapid.SampledFrom([]string{"private", "shared"}).Draw(t, "visibility")
		category := rapid.SampledFrom([]string{"decision", "gotcha", "rule", "discovery", "handoff"}).Draw(t, "category")
		scope := rapid.SampledFrom([]string{"", "repo:github.com/o/r", "rule:repo:github.com/o/r"}).Draw(t, "scope")
		action := rapid.SampledFrom(actions).Draw(t, "action")

		got := pdp.DecideRecord(principal, kind, action, recordOwner, category, visibility, scope).Allow
		if want := modelAllows(principal, recordOwner, visibility, action); got != want {
			t.Fatalf("DecideRecord(principal=%q, %s, owner=%q, %s) = %v, want %v",
				principal, action, recordOwner, visibility, got, want)
		}
	})
}
