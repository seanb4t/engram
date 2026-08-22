// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package migrate

import (
	"errors"
	"fmt"
)

// Registry is the production migration-step chain, in From-ascending order.
// It holds the v0->v1 step: Phase 4 registered backfill-short-ids here,
// raising migrate.CurrentVersion to 1 in the same change (see migrate.go).
//
// PHASE4: this var MUST remain declared at PACKAGE SCOPE as a literal —
// `var Registry = []Step{ NewStep(...), ... }` — never built inside a
// function body (an init() func, a lazily-populated getter, or any other
// indirection). This placement is LOAD-BEARING FOR D-03: Irreversible
// panics on an empty reason, and a package-level var literal evaluates at
// package initialization time, before any test in this binary runs — so a
// bad Irreversible("") in a registered step fails the WHOLE BINARY at
// import, not merely a specific test path. Moving Registry's construction
// into a function weakens that guarantee to "whenever the function happens
// to run", which could be never if the function is unreachable from a
// given binary's call graph. Plan 03-02's
// TestRegistryIsAPackageLevelVarWithPhase4Marker asserts both this
// declaration shape and this marker's presence, so violating this
// obligation is a build-breaking mechanical failure, not a review note.
var Registry = []Step{
	NewMintingStep(0, 1, []string{"short_id"},
		Irreversible("snapshot recovery: minted short_ids may already be cited by get_memory/supersede_memory (D-03)"),
		v1FillShortID),
}

// Validate checks the three ordering invariants a step chain must hold for
// StepsFrom to select a sensible sub-chain from it. An empty chain is
// valid. Every violation found is accumulated and returned together via
// errors.Join, rather than stopping at the first — so a fixture can
// exercise each rule independently, and so an operator staring at a broken
// registry sees every problem in one pass rather than fixing them one
// commit at a time. Each message names the offending step index and both
// versions involved.
//
// The three rules, in the order checked:
//
//  1. Transition uniqueness: no two steps may share a From version, and no
//     two may share a To version. This is the STRUCTURAL PRECONDITION for
//     idempotence — with it, no chain can contain two different steps
//     both claiming the same transition — but Validate never calls a
//     step's ApplyFunc, so it says NOTHING about whether applying a step
//     twice is safe. A nondeterministic or incrementing ApplyFunc would
//     pass this rule every single time. The behavioral half of SC1's
//     "idempotency" word is proven executably in two other places, never
//     here: an apply-twice assertion over every conforming fixture in
//     internal/migrate/additive_test.go (plan 03-03, step level), and the
//     second-run no-op assertion in internal/store's
//     TestMigrateTracerLegacyRecordEndToEnd (plan 03-01, sweep level). A
//     reader must not take transition uniqueness alone as that proof.
//  2. Advance: every step's To must be strictly greater than its From.
//  3. Contiguity: from index 1 onward, a step's From must equal the
//     previous step's To — the chain is a single linear sequence, never a
//     graph. Contiguity was chosen over a topological sort of an arbitrary
//     step DAG because every other locked decision in this phase assumes
//     a linear v0->v1->v2 progression, and migrate.CurrentVersion is a
//     single scalar, not a graph head.
func Validate(steps []Step) error {
	var errs []error

	fromSeen := make(map[Version]int, len(steps))
	toSeen := make(map[Version]int, len(steps))
	for i, st := range steps {
		if j, dup := fromSeen[st.from]; dup {
			errs = append(errs, fmt.Errorf("migrate: transition uniqueness violated: steps %d and %d share From=%d", j, i, st.from))
		} else {
			fromSeen[st.from] = i
		}
		if j, dup := toSeen[st.to]; dup {
			errs = append(errs, fmt.Errorf("migrate: transition uniqueness violated: steps %d and %d share To=%d", j, i, st.to))
		} else {
			toSeen[st.to] = i
		}
		if st.to <= st.from {
			errs = append(errs, fmt.Errorf("migrate: step %d does not advance the version: From=%d To=%d", i, st.from, st.to))
		}
	}
	for i := 1; i < len(steps); i++ {
		if steps[i].from != steps[i-1].to {
			errs = append(errs, fmt.Errorf("migrate: step %d is not contiguous with step %d: step %d.From=%d, step %d.To=%d",
				i, i-1, i, steps[i].from, i-1, steps[i-1].to))
		}
	}
	return errors.Join(errs...)
}

// StepsFrom returns the contiguous sub-chain of steps starting at from and
// ending at to, letting one Store.Migrate pass carry records that sit at
// different starting versions through exactly the remaining portion of the
// chain each one needs. from == to returns an empty slice and no error
// (the record is already there). A missing link — no step in steps whose
// From equals the version the selection has reached — returns an error
// naming the version at which the chain broke. The search is bounded by
// len(steps) hops so a malformed (non-Validate'd) chain containing a cycle
// cannot loop StepsFrom forever; it instead returns the same broken-link
// error once the bound is exceeded.
func StepsFrom(steps []Step, from, to Version) ([]Step, error) {
	if from == to {
		return nil, nil
	}
	byFrom := make(map[Version]Step, len(steps))
	for _, st := range steps {
		byFrom[st.from] = st
	}
	var out []Step
	cur := from
	for range steps {
		if cur == to {
			return out, nil
		}
		st, ok := byFrom[cur]
		if !ok {
			return nil, fmt.Errorf("migrate: no step chain from version %d to %d: broke at %d", from, to, cur)
		}
		out = append(out, st)
		cur = st.to
	}
	if cur == to {
		return out, nil
	}
	return nil, fmt.Errorf("migrate: no step chain from version %d to %d: broke at %d", from, to, cur)
}
