// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authz

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/cedar-policy/cedar-go"
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

// TestDecisionLogCarriesOnlyAllowlistedFields pins DecisionLog to exactly the
// D-02 allowlist by comparing the full field-NAME set, not merely a count —
// a field renamed to something else with the same count would slip past a
// bare NumField() check.
func TestDecisionLogCarriesOnlyAllowlistedFields(t *testing.T) {
	typ := reflect.TypeOf(DecisionLog{})
	if got, want := typ.NumField(), 3; got != want {
		t.Fatalf("DecisionLog has %d fields, want %d", got, want)
	}
	want := map[string]bool{"Allow": true, "PolicyIDs": true, "ErrorCount": true}
	got := map[string]bool{}
	for i := range typ.NumField() {
		got[typ.Field(i).Name] = true
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DecisionLog field set = %v, want %v", got, want)
	}
}

// TestDecisionLogNeverLeaksExpressionTrace is the D-02 negative gate: no
// DiagnosticError.Message-shaped content ever reaches Log()'s output, across
// an allow, a deny, a multi-policy decision, and a decision carrying
// diagnostic errors.
//
// Route taken (see 04-02-SUMMARY.md for the full rationale): Decision.diag is
// unexported by design (D-03), and every attribute this package's policies
// read is either `has`-guarded or a plain cedar.String built from a Go
// string (entities.go) — no input reachable through the public
// DecideBucket/DecideRecord API against the embedded four-policy corpus can
// ever produce a Cedar evaluation error, so the "error-carrying" case has no
// real-world reachable input. This test constructs Decision values directly
// instead — only this package's own test file can do that, since diag is
// unexported outside internal/authz — to exercise all four shapes regardless
// of what the shipped policies can currently trigger.
func TestDecisionLogNeverLeaksExpressionTrace(t *testing.T) {
	const marker = "SENTINEL-caller-entity-value-9f3c2a"
	cases := []struct {
		name string
		d    Decision
	}{
		{
			name: "allow",
			d: Decision{Allow: true, diag: cedar.Diagnostic{
				Reasons: []cedar.DiagnosticReason{{PolicyID: "own-records"}},
			}},
		},
		{
			name: "deny",
			d: Decision{Allow: false, diag: cedar.Diagnostic{
				Reasons: []cedar.DiagnosticReason{{PolicyID: "defense-empty-owner"}},
			}},
		},
		{
			name: "multi-policy",
			d: Decision{Allow: true, diag: cedar.Diagnostic{
				Reasons: []cedar.DiagnosticReason{
					{PolicyID: "own-records"},
					{PolicyID: "shared-read"},
				},
			}},
		},
		{
			// The case no real evaluation against the shipped policies can
			// produce (see doc comment) — diagnostic errors, carrying the
			// sentinel marker the way a real entity-value leak would.
			name: "error-carrying",
			d: Decision{Allow: false, diag: cedar.Diagnostic{
				Reasons: []cedar.DiagnosticReason{{PolicyID: "tenant-isolate"}},
				Errors: []cedar.DiagnosticError{
					{PolicyID: "tenant-isolate", Message: "attribute `" + marker + "` not found on entity"},
				},
			}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dl := tc.d.Log()
			b, err := json.Marshal(dl)
			if err != nil {
				t.Fatalf("marshal DecisionLog: %v", err)
			}
			if strings.Contains(string(b), marker) {
				t.Fatalf("DecisionLog leaked the sentinel marker: %s", b)
			}
			if got, want := dl.Allow, tc.d.Allow; got != want {
				t.Errorf("Allow = %v, want %v", got, want)
			}
			if got, want := dl.ErrorCount, len(tc.d.diag.Errors); got != want {
				t.Errorf("ErrorCount = %d, want %d", got, want)
			}
		})
	}
}
