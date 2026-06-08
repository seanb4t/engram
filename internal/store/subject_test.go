// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import "testing"

func TestSubjectOwner(t *testing.T) {
	if got := Anonymous().Owner(); got != "" {
		t.Errorf("Anonymous().Owner() = %q, want \"\"", got)
	}
	if got := Authenticated("sub-A").Owner(); got != "sub-A" {
		t.Errorf("Authenticated(sub-A).Owner() = %q, want \"sub-A\"", got)
	}
}

// TestSubjectZeroValueIsNil documents the load-bearing property: the zero value
// of the Subject interface is nil, NOT Anonymous — so discarding the extraction
// error (subj, _ := subjectFromContext(...)) yields nil, which fails closed at
// the store default arm rather than silently granting the anonymous bucket.
func TestSubjectZeroValueIsNil(t *testing.T) {
	var zero Subject
	if zero != nil {
		t.Errorf("zero Subject = %v, want nil", zero)
	}
}
