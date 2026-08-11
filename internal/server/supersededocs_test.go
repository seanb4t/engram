// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanb4t/engram/internal/store"
)

// This file closes a failure mode this repository has hit before:
// TestSpineReviewPurgeSameRunNoticePublished pins purgeSameRunLimitationNotice
// / purgeIntersectionScopingNotice in the purge preview output and cobra Long
// help, but docs-site/guides/cli.md's purge section mirrors those constants
// BY HAND — edit either constant and the published guide goes stale with
// every test still green, because nothing ever reads the guide back. This
// gate is the opposite shape: the documented multi-target rejection examples
// are asserted against renderTargetRejection's ACTUAL output, never a
// re-typed transcription of it, so the published text and the shipped
// behaviour cannot drift apart silently.
const (
	docsErrorsPath = "../../docs-site/src/content/docs/reference/errors.md"
	docsToolsPath  = "../../docs-site/src/content/docs/reference/tools.md"
	claudeMdPath   = "../../CLAUDE.md"
	skillMdPath    = "../../skill/engram/skills/curating-memory/SKILL.md"
)

// normalizeWS collapses all whitespace runs (including line breaks
// introduced by hand-wrapped markdown prose) into single spaces, so a
// substring assertion is not accidentally broken by where a doc author
// chose to wrap a line.
func normalizeWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func mustReadDoc(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// TestSupersedeDocsMatchShippedContract binds every shipped surface this
// phase touches — the error reference's worked examples, the tool
// reference's declared argument type and bounds, the project contract, and
// the agent-facing skill — to the code that actually implements the
// contract, so a future edit to any of them fails the build instead of
// drifting quietly.
func TestSupersedeDocsMatchShippedContract(t *testing.T) {
	t.Run("RenderedRejectionExamplesAppearVerbatim", func(t *testing.T) {
		doc := normalizeWS(mustReadDoc(t, docsErrorsPath))

		notFoundErr := renderTargetRejection(store.ErrNotFound, []string{"a1b2c3d4e5", "f6g7h8j9k0"})
		if want := normalizeWS(notFoundErr.Error()); !strings.Contains(doc, want) {
			t.Errorf("rendered addressability rejection %q not found verbatim in %s — update the worked example to match renderTargetRejection's actual output", notFoundErr.Error(), docsErrorsPath)
		}

		alreadySupersededErr := renderTargetRejection(store.ErrAlreadySuperseded, []string{"a1b2c3d4e5", "m1n2p3q4r5"})
		if want := normalizeWS(alreadySupersededErr.Error()); !strings.Contains(doc, want) {
			t.Errorf("rendered already-superseded rejection %q not found verbatim in %s — update the worked example to match renderTargetRejection's actual output", alreadySupersededErr.Error(), docsErrorsPath)
		}
	})

	t.Run("SchemaTypeAndBoundsAgreeWithDocumentedContract", func(t *testing.T) {
		schema, err := supersedeMemoryInputSchema()
		if err != nil {
			t.Fatalf("supersedeMemoryInputSchema: %v", err)
		}
		prop, ok := schema.Properties["supersedes"]
		if !ok {
			t.Fatal("generated schema is missing the supersedes property")
		}
		if prop.Type != "array" && !slices.Contains(prop.Types, "array") {
			t.Fatalf("supersedes schema type = %q (Types=%v), want array", prop.Type, prop.Types)
		}
		if _, ok := schema.Properties["idempotency_key"]; !ok {
			t.Errorf("generated schema does not advertise idempotency_key on supersede_memory")
		}

		toolsDoc := mustReadDoc(t, docsToolsPath)
		if !strings.Contains(toolsDoc, "array of string") {
			t.Errorf("%s does not declare supersedes as an array of string, want it to agree with the schema's array type", docsToolsPath)
		}

		// The bound. minItems is unconditional (PD-07 keeps it at 1
		// regardless of whether a cap was adopted). maxItems is read FROM
		// THE SCHEMA, never from a literal in this test and never from the
		// maxSupersedeTargets constant — plan 03's bounds test already
		// binds the constant to the schema, so re-binding it here would
		// just add a third place to update.
		if prop.MinItems == nil || *prop.MinItems != 1 {
			t.Fatalf("supersedes MinItems = %v, want 1", prop.MinItems)
		}

		anchorRe := regexp.MustCompile(`\*\*Maximum targets per call:\*\* (\d+)`)
		anchor := anchorRe.FindStringSubmatch(toolsDoc)

		switch {
		case prop.MaxItems == nil && anchor == nil:
			// PD-07 (03.1-00-SUMMARY.md) DROPPED the target-set cap: the
			// schema advertises no maxItems and the tool reference
			// publishes no maximum. Both sides agree by absence — this is
			// the shipped state.
		case prop.MaxItems != nil && anchor == nil:
			t.Errorf("schema declares supersedes MaxItems=%d but %s has no anchored '**Maximum targets per call:** N' line — the published contract is silently narrower than what the schema advertises", *prop.MaxItems, docsToolsPath)
		case prop.MaxItems == nil && anchor != nil:
			t.Errorf("%s publishes an anchored maximum (%q) but the generated schema declares no MaxItems — the published contract is silently narrower than what the schema advertises, or the schema regressed to a cap nobody documented", docsToolsPath, anchor[0])
		default:
			published, convErr := strconv.Atoi(anchor[1])
			if convErr != nil {
				t.Fatalf("parsing anchored maximum %q: %v", anchor[1], convErr)
			}
			if published != *prop.MaxItems {
				t.Errorf("published maximum %d in %s does not match schema MaxItems %d", published, docsToolsPath, *prop.MaxItems)
			}
		}
	})

	t.Run("NoShippedDocClaimsKeyUnsupportedAndEachPositivelyStatesSupport", func(t *testing.T) {
		// These are historical strings this repository shipped before
		// plan 03.1-04 added idempotency_key support to supersede_memory —
		// kept here ONLY as named constants so their reappearance fails
		// this test. If you are editing one of the three files below and
		// this test starts failing on the "stale claim" branch, you most
		// likely reintroduced one of these phrases rather than the current
		// key-support statement.
		const staleToolsPhrase = "is **not** supported on this verb"
		const staleClaudePhrase = "is not supported on this verb"
		const staleSkillPhrase = "is **not** supported here"

		// The CURRENT, correct statement each file must positively carry.
		// A sweep for the stale phrase alone is one-directional: a file
		// that deletes the stale sentence and says nothing in its place
		// would pass a phrase-only test while leaving an agent with no
		// statement at all (cycle-1 review, LOW). Both directions are
		// asserted per file below.
		const acceptedOnThisVerb = "idempotency_key` is accepted on this verb"
		const isSupportedHere = "idempotency_key` **is** supported here"

		cases := []struct {
			name           string
			path           string
			stalePhrase    string
			positivePhrase string
		}{
			{"tool reference", docsToolsPath, staleToolsPhrase, acceptedOnThisVerb},
			{"project contract", claudeMdPath, staleClaudePhrase, acceptedOnThisVerb},
			{"curating-memory skill", skillMdPath, staleSkillPhrase, isSupportedHere},
		}
		for _, c := range cases {
			doc := normalizeWS(mustReadDoc(t, c.path))
			if strings.Contains(doc, normalizeWS(c.stalePhrase)) {
				t.Errorf("%s (%s) still contains the stale claim %q that supersede_memory rejects idempotency keys", c.name, c.path, c.stalePhrase)
			}
			if !strings.Contains(doc, normalizeWS(c.positivePhrase)) {
				t.Errorf("%s (%s) does not positively state that idempotency_key is accepted — want %q", c.name, c.path, c.positivePhrase)
			}
		}
	})

	t.Run("FailureBehaviorParagraphIsHonestNotOverstated", func(t *testing.T) {
		doc := normalizeWS(mustReadDoc(t, docsToolsPath))

		// The required cost sentence (cycle-1 review, HIGH): a failed
		// merge's compensation talks to the same store that just failed,
		// so it can leave the caller with an orphaned survivor, dangling
		// links, or both, and operator logging is the only remediation —
		// this is narrower than "it's always cleaned up".
		requiredFragments := []string{
			"orphaned new record",
			"logs the affected ids for the operator",
			"that log is the only remediation path",
		}
		for _, frag := range requiredFragments {
			if !strings.Contains(doc, normalizeWS(frag)) {
				t.Errorf("%s is missing the cleanup-failure cost fragment %q — the failed-merge paragraph must name what a cleanup failure can leave behind, not just the usual no-op outcome", docsToolsPath, frag)
			}
		}

		overstatedForms := []string{"never observable", "cannot be observed", "no reader"}
		for _, form := range overstatedForms {
			if strings.Contains(doc, form) {
				t.Errorf("%s contains the overstated claim %q — a partially-applied merge is observable by a concurrent, lock-free reader mid-window; only the TERMINAL state after a successful merge attempt is provably clean", docsToolsPath, form)
			}
		}
	})
}
