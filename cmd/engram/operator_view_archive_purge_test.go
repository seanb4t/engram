// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/seanb4t/engram/internal/store"
)

// This file holds ONLY this plan's group fixtures (the three two-level
// spine-review reports: archive, restore, purge). Plan 06-07 merges every
// group's fixture function together and gates the merged key set against
// operatorCommands() in both directions.

// archivePurgeViewFixtures returns the identity-gate fixtures for this
// group's three commandKey entries. Every sample is built by calling the
// report's own real converter (archiveDoc / purgePreviewDoc / purgeAppliedDoc)
// -- never a hand-built doc literal -- reusing the fixed input values
// operator_output_test.go's operatorParityRows entries for these commands
// used, but none of that other test's declared-string-list assertions or
// row structure.
func archivePurgeViewFixtures() map[string][]any {
	return map[string][]any{
		"spine-review archive": {
			// Mixes a resolved id (carries id=) with an unresolved one (the
			// omitempty drop, exercised in a rendered row).
			archiveDoc([]store.ArchiveResult{
				{Requested: "id-changed", ID: "id-changed", Outcome: store.ArchiveOutcomeChanged},
				{Requested: "zzzznotfound", Outcome: store.ArchiveOutcomeNotFound},
			}, "archive"),
			// Empty result slice -- the "results" label line with zero rows.
			archiveDoc(nil, "archive"),
		},
		"spine-review restore": {
			archiveDoc([]store.ArchiveResult{
				{Requested: "id-restored", ID: "id-restored", Outcome: store.ArchiveOutcomeChanged},
				{Requested: "id-unknown", Outcome: store.ArchiveOutcomeNotFound},
			}, "restore"),
		},
		"spine-review purge": {
			// Preview: non-empty eligible list, a scope, a category, tags,
			// and a non-zero --older-than, so every omitempty key is present
			// and rerun is populated.
			purgePreviewDoc([]string{"id-eligible-1", "id-eligible-2"}, store.PurgeOptions{
				Classes: []store.PurgeClass{store.PurgeClassExpired}, Scope: "s",
				Category: "note", Tags: []string{"t1"}, OlderThan: 24 * time.Hour,
			}),
			// Applied: non-empty deleted/spared/appeared lists so all three
			// independent row lists render and rerun is absent.
			purgeAppliedDoc([]string{"id-deleted", "id-spared"}, store.PurgeResult{
				Deleted: []string{"id-deleted"}, Spared: []string{"id-spared"}, Appeared: []string{"id-appeared"},
			}, store.PurgeOptions{Classes: []store.PurgeClass{store.PurgeClassExpired}, Scope: "s"}),
		},
	}
}

// TestArchivePurgeViewIdentity runs the shared identity gate over every
// fixture in this group, plus one assertion specific to this group -- it is
// the group that exercises D-07's nested rendering rule.
func TestArchivePurgeViewIdentity(t *testing.T) {
	for name, docs := range archivePurgeViewFixtures() {
		for i, doc := range docs {
			t.Run(fmt.Sprintf("%s/%d", name, i), func(t *testing.T) {
				assertViewIdentity(t, name, doc)
			})
		}
	}

	// D-07 proof: for the archive fixture containing an unresolved id,
	// assert the rendered output contains a four-space row line that has
	// requested= and outcome= but not id= -- proving the omitempty key
	// omission reaches the rendered row, not merely the marshaled bytes.
	t.Run("unresolved id omits id= at the rendered-row level", func(t *testing.T) {
		doc := archiveDoc([]store.ArchiveResult{
			{Requested: "id-changed", ID: "id-changed", Outcome: store.ArchiveOutcomeChanged},
			{Requested: "zzzznotfound", Outcome: store.ArchiveOutcomeNotFound},
		}, "archive")

		var buf bytes.Buffer
		if err := renderOperatorView(&buf, "headline", doc); err != nil {
			t.Fatalf("renderOperatorView: %v", err)
		}

		found := false
		for _, line := range strings.Split(buf.String(), "\n") {
			if !strings.HasPrefix(line, "    ") {
				continue
			}
			if !strings.Contains(line, "requested=zzzznotfound") {
				continue
			}
			found = true
			if !strings.Contains(line, "outcome=") {
				t.Errorf("unresolved-id row line %q does not contain outcome=", line)
			}
			if strings.Contains(line, "id=") {
				t.Errorf("unresolved-id row line %q unexpectedly contains id=", line)
			}
		}
		if !found {
			t.Fatalf("rendered output %q does not contain the unresolved-id row", buf.String())
		}
	})
}
