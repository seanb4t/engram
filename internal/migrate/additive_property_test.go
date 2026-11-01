// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package migrate

import (
	"maps"
	"testing"

	"pgregory.net/rapid"
)

// v0PayloadKeys are keys a version-0 record can carry. short_id is among
// them because backfill-short-ids could stamp it before any migration ran.
var v0PayloadKeys = []string{
	"content", "scope", "category", "tags", "source", "summary", "summary_source",
	"actor", "owner", "visibility", "created_at", "repo", "workspace", "worktree",
	"base_dir", "superseded_by", "not_before", "not_after", "archived_at", "short_id",
}

func v0Payload() *rapid.Generator[map[string]any] {
	value := rapid.OneOf(
		rapid.Map(rapid.String(), func(s string) any { return s }),
		rapid.Map(rapid.Bool(), func(b bool) any { return b }),
		rapid.Map(rapid.Float64(), func(f float64) any { return f }),
		rapid.Map(rapid.SliceOf(rapid.String()), func(s []string) any { return s }),
		rapid.Just[any](nil),
	)
	return rapid.MapOf(rapid.OneOf(rapid.SampledFrom(v0PayloadKeys), rapid.String()), value)
}

func TestPropertyStepAdditive(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		before := v0Payload().Draw(t, "payload")
		for _, step := range Registry {
			input := maps.Clone(before)
			var after map[string]any
			var err error
			if _, ok := step.ApplyMinter(); ok {
				after, err = step.ApplyMinterMint(input, func() (string, error) { return "0123456789", nil })
			} else {
				after, err = step.Apply(input)
			}
			if err != nil {
				t.Fatalf("step %d->%d: %v", step.From(), step.To(), err)
			}
			if err := CheckAdditive(step, before, after); err != nil {
				t.Fatalf("step %d->%d on %v: %v", step.From(), step.To(), before, err)
			}
			before = after
		}
	})
}
