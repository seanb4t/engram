// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package setup

// ExitClass is the three-way classification of a completed engram setup
// run over its per-runtime Results (REQ-setup-partial-failure-legible).
// cmd/engram/setup.go maps each value to a process exit code; internal/setup
// itself never returns an int, keeping this package free of cmd/engram's
// exit-code vocabulary (the leaf-purity gate, leafpurity_test.go).
type ExitClass int

const (
	// ExitTotalSuccess means no runtime attempted registration and failed —
	// including the zero-attempts case (no runtime selected, or every
	// selected runtime was OutcomeNotPresent).
	ExitTotalSuccess ExitClass = iota
	// ExitPartial means at least one runtime failed alongside at least one
	// runtime that succeeded or was already correct.
	ExitPartial
	// ExitTotalFailure means every runtime engram actually attempted
	// failed.
	ExitTotalFailure
)

// Classify is a pure function over an already-built set of per-runtime
// Results: no I/O, no context, no filesystem or network access, so the
// whole outcome-combination table is unit-testable with no fake and no
// harness (exit_test.go). It mirrors verifyFailOnErr's shape
// (cmd/engram/spine_review_verify.go): a pure classification returning a
// typed decision the caller maps to a process exit code.
//
// The classification rule, stated once here and encoded once here:
//   - An attempt is any Result whose Outcome is not OutcomeNotPresent. A
//     not-present runtime is an expected outcome, never a failure (D-07):
//     "there was nothing to do" must never be reported to a script as "I
//     failed."
//   - A failure is any Result whose Outcome is OutcomeFailed, OR whose
//     Outcome is the zero value (""). A Result carrying the zero-valued
//     Outcome is a programming error, not a data case: silently bucketing
//     an unset outcome as success is exactly how an unreported error
//     becomes an exit 0, so it is treated as a failure rather than
//     ignored.
//   - Zero failures -> ExitTotalSuccess, including the zero-attempts case.
//   - At least one failure alongside at least one non-failed attempt ->
//     ExitPartial.
//   - At least one failure and no non-failed attempt -> ExitTotalFailure.
func Classify(results []Result) ExitClass {
	var hasFailure, hasNonFailedAttempt bool
	for _, r := range results {
		switch r.Outcome {
		case OutcomeNotPresent:
			// Not an attempt (D-07): contributes to neither class.
		case OutcomeFailed:
			hasFailure = true
		case OutcomeAlreadyCorrect, OutcomeWouldWrite, OutcomeWrote:
			hasNonFailedAttempt = true
		default:
			// The zero-valued Outcome ("") or any other unrecognized
			// value: a programming error, classified as a failure so it
			// is never silently laundered into ExitTotalSuccess.
			hasFailure = true
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
