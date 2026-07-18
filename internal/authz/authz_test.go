// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authz

import (
	"sync"
	"testing"
)

// TestPDP_DecideBucketWiring exercises DecideBucket's own/shared probe
// semantics across authenticated and anonymous owners.
func TestPDP_DecideBucketWiring(t *testing.T) {
	pdp := MustDefault()
	cases := []struct {
		name      string
		owner     string
		action    Action
		bucket    Bucket
		wantAllow bool
	}{
		{"own-bucket-authenticated-read", "alice", ActionRead, BucketOwn, true},
		{"own-bucket-authenticated-write", "alice", ActionWrite, BucketOwn, true},
		{"own-bucket-anonymous-read", "", ActionRead, BucketOwn, true},
		{"own-bucket-anonymous-write", "", ActionWrite, BucketOwn, true},
		{"shared-bucket-authenticated-read", "alice", ActionRead, BucketShared, true},
		{"shared-bucket-authenticated-write", "alice", ActionWrite, BucketShared, false},
		{"shared-bucket-anonymous-read", "", ActionRead, BucketShared, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := pdp.DecideBucket(c.owner, "human", c.action, c.bucket)
			if got.Allow != c.wantAllow {
				t.Fatalf("DecideBucket(owner=%q, action=%s, bucket=%v) = %v, want %v", c.owner, c.action, c.bucket, got.Allow, c.wantAllow)
			}
		})
	}
}

// TestPDP_DecideRecordWiring exercises the DecideRecord Memory-entity path
// across authenticated and anonymous owners.
func TestPDP_DecideRecordWiring(t *testing.T) {
	pdp := MustDefault()
	cases := []struct {
		name                                     string
		owner, kind                              string
		action                                   Action
		memoryOwner, category, visibility, scope string
		wantAllow                                bool
	}{
		{"authenticated-own-read", "alice", "human", ActionRead, "alice", "note", "private", "", true},
		{"authenticated-cross-owner-private-read", "alice", "human", ActionRead, "bob", "note", "private", "", false},
		{"anonymous-own-read", "", "human", ActionRead, "", "note", "private", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := pdp.DecideRecord(c.owner, c.kind, c.action, c.memoryOwner, c.category, c.visibility, c.scope)
			if got.Allow != c.wantAllow {
				t.Fatalf("DecideRecord(...) = %v, want %v", got.Allow, c.wantAllow)
			}
		})
	}
}

// TestPDP_ConcurrentDecideRace proves the PDP is immutable after construction
// and safe for concurrent use: parallel DecideBucket/DecideRecord calls on
// one shared *PDP under -race produce no race and stable decisions.
func TestPDP_ConcurrentDecideRace(t *testing.T) {
	pdp := MustDefault()
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if got := pdp.DecideBucket("alice", "human", ActionRead, BucketOwn); !got.Allow {
				t.Errorf("concurrent DecideBucket(own) = Deny, want Allow")
			}
		}()
		go func() {
			defer wg.Done()
			if got := pdp.DecideRecord("bob", "human", ActionWrite, "carol", "note", "private", ""); got.Allow {
				t.Errorf("concurrent DecideRecord(cross-owner write) = Allow, want Deny")
			}
		}()
	}
	wg.Wait()
}
