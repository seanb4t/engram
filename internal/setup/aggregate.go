// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

// precedenceOrder states D-06's authored aggregation precedence, highest
// first: failed > wrote > already-correct > would-write > not-present.
// Declared once here so AggregateOutcome's own loop and its doc comment
// cannot drift from each other.
var precedenceOrder = []Outcome{
	OutcomeFailed,
	OutcomeWrote,
	OutcomeAlreadyCorrect,
	OutcomeWouldWrite,
	OutcomeNotPresent,
}

// isRecognizedOutcome reports whether o is one of the five pinned Outcome
// constants — never true for the zero value ("") or for any other string.
func isRecognizedOutcome(o Outcome) bool {
	for _, candidate := range precedenceOrder {
		if o == candidate {
			return true
		}
	}
	return false
}

// AggregateOutcome folds two facet outcomes (e.g. a runtime's
// registration outcome and its skills outcome) into the ONE Outcome a
// report row carries, by the authored, documented precedence: failed >
// wrote > already-correct > would-write > not-present. The function is
// symmetric — AggregateOutcome(a, b) == AggregateOutcome(b, a) for every
// pair — and treats the zero value or any unrecognized value in EITHER
// argument as failed, so an unset outcome can never be laundered into a
// success.
//
// The generalized invariant this function exists to hold: mixed state
// resolves UP to wrote, never down to already-correct. Falsely reporting
// convergence is the failure this milestone keeps designing against —
// TestAggregatePrecedenceIsAuthoredNotDerived (aggregate_test.go) is that
// invariant's regression guard.
func AggregateOutcome(a, b Outcome) Outcome {
	if !isRecognizedOutcome(a) || !isRecognizedOutcome(b) {
		return OutcomeFailed
	}
	for _, candidate := range precedenceOrder {
		if a == candidate || b == candidate {
			return candidate
		}
	}
	// Unreachable: both a and b passed isRecognizedOutcome above, so one
	// of them equals some entry in precedenceOrder, which the loop above
	// always finds first.
	return OutcomeFailed
}

// SkillsOutcome classifies one runtime's skills-install facet from its
// raw install signals, in this precedence:
//
//  1. format == SkillFormatNone always yields OutcomeWouldWrite, in BOTH
//     the preview and the apply lane (mutate is not consulted at all) —
//     this runtime never writes and never converges, the same reasoning
//     the shared executor already applies to a zero-Actions Plan.
//  2. failed yields OutcomeFailed, even when writes also succeeded —
//     partial success within a single Install call is still reported as
//     a failure at the facet level (D-07's independent-failure model
//     folds into ONE outcome here; internal/skills.Report's own
//     Wrote/AlreadyCorrect slices retain the finer-grained detail for
//     Reason).
//  3. A non-mutating call (a preview) yields OutcomeWouldWrite — a single
//     read has no honest basis for claiming convergence.
//  4. Any writes yield OutcomeWrote.
//  5. Otherwise, any already-correct files yield OutcomeAlreadyCorrect.
//  6. zero writes, zero already-correct files, and no failure yields
//     OutcomeFailed: an empty inventory means the embed is broken, and
//     that must never be laundered into success.
func SkillsOutcome(format SkillFormat, mutate bool, wrote int, alreadyCorrect int, failed bool) Outcome {
	if format == SkillFormatNone {
		return OutcomeWouldWrite
	}
	if failed {
		return OutcomeFailed
	}
	if !mutate {
		return OutcomeWouldWrite
	}
	if wrote > 0 {
		return OutcomeWrote
	}
	if alreadyCorrect > 0 {
		return OutcomeAlreadyCorrect
	}
	return OutcomeFailed
}
