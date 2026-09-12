// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import "testing"

// allOutcomesPlusZero is the full cross-product domain
// TestAggregateOutcomeExhaustive drives: the five pinned Outcome constants
// plus the Go zero value — six values, thirty-six ordered pairs.
var allOutcomesPlusZero = []Outcome{
	OutcomeFailed,
	OutcomeWrote,
	OutcomeAlreadyCorrect,
	OutcomeWouldWrite,
	OutcomeNotPresent,
	Outcome(""),
}

// TestAggregateOutcomeExhaustive drives every ordered pair of the five
// outcome constants plus the zero value — thirty-six pairs, every one
// asserted, none skipped — with the expected winner stated LITERALLY per
// row rather than computed by the same precedence logic under test (D-06:
// failed > wrote > already-correct > would-write > not-present; the zero
// value or any unrecognized value aggregates to failed).
func TestAggregateOutcomeExhaustive(t *testing.T) {
	want := map[[2]Outcome]Outcome{
		{OutcomeFailed, OutcomeFailed}:         OutcomeFailed,
		{OutcomeFailed, OutcomeWrote}:          OutcomeFailed,
		{OutcomeFailed, OutcomeAlreadyCorrect}: OutcomeFailed,
		{OutcomeFailed, OutcomeWouldWrite}:     OutcomeFailed,
		{OutcomeFailed, OutcomeNotPresent}:     OutcomeFailed,
		{OutcomeFailed, Outcome("")}:           OutcomeFailed,

		{OutcomeWrote, OutcomeFailed}:         OutcomeFailed,
		{OutcomeWrote, OutcomeWrote}:          OutcomeWrote,
		{OutcomeWrote, OutcomeAlreadyCorrect}: OutcomeWrote,
		{OutcomeWrote, OutcomeWouldWrite}:     OutcomeWrote,
		{OutcomeWrote, OutcomeNotPresent}:     OutcomeWrote,
		{OutcomeWrote, Outcome("")}:           OutcomeFailed,

		{OutcomeAlreadyCorrect, OutcomeFailed}:         OutcomeFailed,
		{OutcomeAlreadyCorrect, OutcomeWrote}:          OutcomeWrote,
		{OutcomeAlreadyCorrect, OutcomeAlreadyCorrect}: OutcomeAlreadyCorrect,
		{OutcomeAlreadyCorrect, OutcomeWouldWrite}:     OutcomeAlreadyCorrect,
		{OutcomeAlreadyCorrect, OutcomeNotPresent}:     OutcomeAlreadyCorrect,
		{OutcomeAlreadyCorrect, Outcome("")}:           OutcomeFailed,

		{OutcomeWouldWrite, OutcomeFailed}:         OutcomeFailed,
		{OutcomeWouldWrite, OutcomeWrote}:          OutcomeWrote,
		{OutcomeWouldWrite, OutcomeAlreadyCorrect}: OutcomeAlreadyCorrect,
		{OutcomeWouldWrite, OutcomeWouldWrite}:     OutcomeWouldWrite,
		{OutcomeWouldWrite, OutcomeNotPresent}:     OutcomeWouldWrite,
		{OutcomeWouldWrite, Outcome("")}:           OutcomeFailed,

		{OutcomeNotPresent, OutcomeFailed}:         OutcomeFailed,
		{OutcomeNotPresent, OutcomeWrote}:          OutcomeWrote,
		{OutcomeNotPresent, OutcomeAlreadyCorrect}: OutcomeAlreadyCorrect,
		{OutcomeNotPresent, OutcomeWouldWrite}:     OutcomeWouldWrite,
		{OutcomeNotPresent, OutcomeNotPresent}:     OutcomeNotPresent,
		{OutcomeNotPresent, Outcome("")}:           OutcomeFailed,

		{Outcome(""), OutcomeFailed}:         OutcomeFailed,
		{Outcome(""), OutcomeWrote}:          OutcomeFailed,
		{Outcome(""), OutcomeAlreadyCorrect}: OutcomeFailed,
		{Outcome(""), OutcomeWouldWrite}:     OutcomeFailed,
		{Outcome(""), OutcomeNotPresent}:     OutcomeFailed,
		{Outcome(""), Outcome("")}:           OutcomeFailed,
	}

	if len(want) != 36 {
		t.Fatalf("table has %d entries, want 36 (6x6 ordered pairs)", len(want))
	}

	for pair, expected := range want {
		pair, expected := pair, expected
		t.Run(string(pair[0])+"_"+string(pair[1]), func(t *testing.T) {
			if got := AggregateOutcome(pair[0], pair[1]); got != expected {
				t.Errorf("AggregateOutcome(%q, %q) = %q, want %q", pair[0], pair[1], got, expected)
			}
		})
	}

	t.Run("symmetric", func(t *testing.T) {
		for _, a := range allOutcomesPlusZero {
			for _, b := range allOutcomesPlusZero {
				if got, rev := AggregateOutcome(a, b), AggregateOutcome(b, a); got != rev {
					t.Errorf("AggregateOutcome(%q, %q) = %q, AggregateOutcome(%q, %q) = %q, want symmetric", a, b, got, b, a, rev)
				}
			}
		}
	})

	t.Run("unrecognized_value_aggregates_to_failed", func(t *testing.T) {
		const bogus = Outcome("bogus")
		if got := AggregateOutcome(bogus, OutcomeWrote); got != OutcomeFailed {
			t.Errorf("AggregateOutcome(%q, %q) = %q, want %q", bogus, OutcomeWrote, got, OutcomeFailed)
		}
		if got := AggregateOutcome(OutcomeWrote, bogus); got != OutcomeFailed {
			t.Errorf("AggregateOutcome(%q, %q) = %q, want %q", OutcomeWrote, bogus, got, OutcomeFailed)
		}
	})
}

// TestAggregatePrecedenceIsAuthoredNotDerived asserts the one invariant
// that generalizes Phase 3's D-08 over the whole input space: for every
// pair containing OutcomeWrote and OutcomeAlreadyCorrect, the result is
// OutcomeWrote, in BOTH argument orders. Falsely reporting convergence is
// the failure this milestone keeps designing against; this test is its
// regression guard.
func TestAggregatePrecedenceIsAuthoredNotDerived(t *testing.T) {
	if got := AggregateOutcome(OutcomeWrote, OutcomeAlreadyCorrect); got != OutcomeWrote {
		t.Errorf("AggregateOutcome(wrote, already-correct) = %q, want %q", got, OutcomeWrote)
	}
	if got := AggregateOutcome(OutcomeAlreadyCorrect, OutcomeWrote); got != OutcomeWrote {
		t.Errorf("AggregateOutcome(already-correct, wrote) = %q, want %q", got, OutcomeWrote)
	}
}

// TestSkillsOutcomeExhaustive covers every bullet in 04-01-PLAN.md Task 2's
// <behavior> block, including both mutate values for the no-destination
// format.
func TestSkillsOutcomeExhaustive(t *testing.T) {
	t.Run("no_destination_format_ignores_mutate_and_counts", func(t *testing.T) {
		for _, mutate := range []bool{false, true} {
			for _, tc := range []struct {
				wrote, alreadyCorrect int
				failed                bool
			}{
				{0, 0, false},
				{5, 0, false},
				{0, 5, false},
				{0, 0, true},
			} {
				got := SkillsOutcome(SkillFormatNone, mutate, tc.wrote, tc.alreadyCorrect, tc.failed)
				if got != OutcomeWouldWrite {
					t.Errorf("SkillsOutcome(none, mutate=%v, wrote=%d, alreadyCorrect=%d, failed=%v) = %q, want %q",
						mutate, tc.wrote, tc.alreadyCorrect, tc.failed, got, OutcomeWouldWrite)
				}
			}
		}
	})

	t.Run("failure_wins_even_with_writes", func(t *testing.T) {
		got := SkillsOutcome(SkillFormatNative, true, 3, 0, true)
		if got != OutcomeFailed {
			t.Errorf("SkillsOutcome(native, mutate=true, wrote=3, failed=true) = %q, want %q", got, OutcomeFailed)
		}
	})

	t.Run("non_mutating_call_yields_would_write", func(t *testing.T) {
		got := SkillsOutcome(SkillFormatNative, false, 0, 0, false)
		if got != OutcomeWouldWrite {
			t.Errorf("SkillsOutcome(native, mutate=false) = %q, want %q", got, OutcomeWouldWrite)
		}
	})

	t.Run("any_writes_yield_wrote", func(t *testing.T) {
		got := SkillsOutcome(SkillFormatNative, true, 1, 4, false)
		if got != OutcomeWrote {
			t.Errorf("SkillsOutcome(native, mutate=true, wrote=1, alreadyCorrect=4) = %q, want %q", got, OutcomeWrote)
		}
	})

	t.Run("already_correct_when_no_writes", func(t *testing.T) {
		got := SkillsOutcome(SkillFormatNative, true, 0, 5, false)
		if got != OutcomeAlreadyCorrect {
			t.Errorf("SkillsOutcome(native, mutate=true, wrote=0, alreadyCorrect=5) = %q, want %q", got, OutcomeAlreadyCorrect)
		}
	})

	t.Run("zero_writes_zero_already_correct_no_failure_is_failed", func(t *testing.T) {
		got := SkillsOutcome(SkillFormatNative, true, 0, 0, false)
		if got != OutcomeFailed {
			t.Errorf("SkillsOutcome(native, mutate=true, wrote=0, alreadyCorrect=0, failed=false) = %q, want %q (an empty inventory is a broken embed)", got, OutcomeFailed)
		}
	})
}
