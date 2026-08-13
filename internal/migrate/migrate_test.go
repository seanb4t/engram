// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package migrate

import "testing"

// var _ Version = CurrentVersion is a compile-time assertion that the
// constant carries the named type, not a bare int.
var _ Version = CurrentVersion

// TestCurrentVersionValue pins the cross-phase decision: raising this
// constant is a Phase 3/4 action taken together with registering the step
// that defines the new version, and backfill-short-ids is the registered
// v0->v1 step.
func TestCurrentVersionValue(t *testing.T) {
	if CurrentVersion != Version(0) {
		t.Fatalf("migrate.CurrentVersion = %d, want 0 — raising this constant is a Phase 3/4 action taken together with registering the step that defines the new version (backfill-short-ids is the registered v0->v1 step), never a standalone bump", CurrentVersion)
	}
}
