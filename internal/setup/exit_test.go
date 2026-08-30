// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

import "testing"

// TestClassifySingleOutcome covers every Outcome value in isolation
// (D-07/D-09): a lone not-present, already-correct, would-write, or wrote
// result is a total success; a lone failed result is a total failure.
func TestClassifySingleOutcome(t *testing.T) {
	cases := []struct {
		outcome Outcome
		want    ExitClass
	}{
		{OutcomeNotPresent, ExitTotalSuccess},
		{OutcomeAlreadyCorrect, ExitTotalSuccess},
		{OutcomeWouldWrite, ExitTotalSuccess},
		{OutcomeWrote, ExitTotalSuccess},
		{OutcomeFailed, ExitTotalFailure},
	}
	for _, c := range cases {
		t.Run(string(c.outcome), func(t *testing.T) {
			got := Classify([]Result{{Outcome: c.outcome}})
			if got != c.want {
				t.Errorf("Classify([%s]) = %v, want %v", c.outcome, got, c.want)
			}
		})
	}
}

// TestClassifyBoundaryCases pins the D-07 boundary adjudications: a
// not-present runtime is never a failure and never an attempt, so it
// cannot turn a success into a partial or a partial into a total failure.
func TestClassifyBoundaryCases(t *testing.T) {
	cases := []struct {
		name     string
		outcomes []Outcome
		want     ExitClass
	}{
		{"no runtimes selected", nil, ExitTotalSuccess},
		{"all not-present", []Outcome{OutcomeNotPresent, OutcomeNotPresent, OutcomeNotPresent}, ExitTotalSuccess},
		{"not-present + wrote", []Outcome{OutcomeNotPresent, OutcomeWrote}, ExitTotalSuccess},
		{"not-present + failed (only attempt failed)", []Outcome{OutcomeNotPresent, OutcomeFailed}, ExitTotalFailure},
		{"failed + failed", []Outcome{OutcomeFailed, OutcomeFailed}, ExitTotalFailure},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			results := make([]Result, len(c.outcomes))
			for i, o := range c.outcomes {
				results[i] = Result{Outcome: o}
			}
			got := Classify(results)
			if got != c.want {
				t.Errorf("Classify(%v) = %v, want %v", c.outcomes, got, c.want)
			}
		})
	}
}

// TestClassifyMixedCases pins the ExitPartial cases: any failure alongside
// any non-failed attempt is a partial success, regardless of which
// non-failed outcome accompanies it.
func TestClassifyMixedCases(t *testing.T) {
	cases := []struct {
		name     string
		outcomes []Outcome
		want     ExitClass
	}{
		{"wrote + failed", []Outcome{OutcomeWrote, OutcomeFailed}, ExitPartial},
		{"already-correct + failed", []Outcome{OutcomeAlreadyCorrect, OutcomeFailed}, ExitPartial},
		{"would-write + failed", []Outcome{OutcomeWouldWrite, OutcomeFailed}, ExitPartial},
		{"not-present + wrote + failed", []Outcome{OutcomeNotPresent, OutcomeWrote, OutcomeFailed}, ExitPartial},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			results := make([]Result, len(c.outcomes))
			for i, o := range c.outcomes {
				results[i] = Result{Outcome: o}
			}
			got := Classify(results)
			if got != c.want {
				t.Errorf("Classify(%v) = %v, want %v", c.outcomes, got, c.want)
			}
		})
	}
}

// TestClassifyRejectsZeroValueOutcome proves a Result carrying the
// zero-valued Outcome ("") is never silently bucketed as a success — an
// unset outcome is a programming error, and Classify must not launder it
// into an exit 0 (see Outcome's doc comment in plan.go).
func TestClassifyRejectsZeroValueOutcome(t *testing.T) {
	if got := Classify([]Result{{}}); got == ExitTotalSuccess {
		t.Errorf("Classify([{}]) = %v, want anything but ExitTotalSuccess (a zero-valued Outcome must not launder into success)", got)
	}
}

// allOutcomes is every first-class Outcome value Classify must handle,
// used only to generate the exhaustiveness table below — never
// hand-transcribed into the expectation itself.
var allOutcomes = []Outcome{
	OutcomeNotPresent,
	OutcomeAlreadyCorrect,
	OutcomeWouldWrite,
	OutcomeWrote,
	OutcomeFailed,
}

// classifyExpected derives the expected ExitClass for outcomes from the
// three defining predicates stated in exit.go's doc comment, rather than
// transcribing a table — this is what keeps the gate from drifting when a
// sixth Outcome is ever added.
func classifyExpected(outcomes []Outcome) ExitClass {
	var hasFailure, hasNonFailedAttempt bool
	for _, o := range outcomes {
		switch o {
		case OutcomeFailed:
			hasFailure = true
		case OutcomeNotPresent:
			// not an attempt
		default:
			hasNonFailedAttempt = true
		}
	}
	switch {
	case !hasFailure:
		return ExitTotalSuccess
	case hasNonFailedAttempt:
		return ExitPartial
	default:
		return ExitTotalFailure
	}
}

// enumerateOutcomeTuples generates every ordered tuple of allOutcomes up to
// length maxLen — a generated cross-product, not a hand-typed list — via
// nested iteration rather than recursion, so the generation itself stays
// easy to audit.
func enumerateOutcomeTuples(maxLen int) [][]Outcome {
	var out [][]Outcome
	for length := 1; length <= maxLen; length++ {
		indices := make([]int, length)
		for {
			tuple := make([]Outcome, length)
			for i, idx := range indices {
				tuple[i] = allOutcomes[idx]
			}
			out = append(out, tuple)

			// Odometer increment over indices, base len(allOutcomes).
			pos := length - 1
			for pos >= 0 {
				indices[pos]++
				if indices[pos] < len(allOutcomes) {
					break
				}
				indices[pos] = 0
				pos--
			}
			if pos < 0 {
				break
			}
		}
	}
	return out
}

// TestClassifyExhaustiveOutcomeCombinations enumerates every non-empty
// tuple of the five Outcome values up to length 3 (a generated
// cross-product) and asserts Classify agrees with the three defining
// predicates on each — the exhaustiveness gate the plan requires.
func TestClassifyExhaustiveOutcomeCombinations(t *testing.T) {
	tuples := enumerateOutcomeTuples(3)
	if len(tuples) == 0 {
		t.Fatal("enumerateOutcomeTuples generated zero cases — a scan matching nothing is vacuously green")
	}
	for _, outcomes := range tuples {
		outcomes := outcomes
		results := make([]Result, len(outcomes))
		for i, o := range outcomes {
			results[i] = Result{Outcome: o}
		}
		want := classifyExpected(outcomes)
		got := Classify(results)
		if got != want {
			t.Errorf("Classify(%v) = %v, want %v (derived from the defining predicates)", outcomes, got, want)
		}
	}
}
