// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package migrate

import "slices"

// ApplyFunc is the transformation a migration step performs on one record's
// decoded raw payload. It receives the payload map decoded from a stored
// Qdrant record (see internal/store's payloadToMap) and returns the
// payload it wants stored. The CALLER — Store.Migrate, never the step
// itself — decides what actually reaches Qdrant: it diffs the returned
// payload against the input via migrate.AddedKeys and writes only the
// added keys, per the additive-only discipline this package enforces
// (D-04). A step is therefore free to compute intermediate state however
// it likes; only the final key-set delta against its input is observable
// to the write path.
type ApplyFunc func(payload map[string]any) (map[string]any, error)

// Reversibility is a sealed interface: the only two types that satisfy it
// are the unexported reversibleStep and irreversibleStep this package
// constructs via Reversible and Irreversible. The unexported isReversibility
// marker method closes the one real escape hatch this sealing prevents — a
// foreign type outside this package satisfying Reversibility to register
// an undeclared step (T-03-05) — because Go requires an unexported method
// to be declared in the same package as the interface that names it, so no
// external type can ever implement this interface. Sealing does NOT close
// the other hatch, an explicit nil interface value: Go permits assigning
// nil to any interface-typed parameter regardless of sealing, so
// NewStep's own nil check (not this interface) is what makes "nobody
// declared reversibility" unrepresentable (RESEARCH Pitfall 1). No
// exported type in this package may ever embed a carrier of
// isReversibility(): embedding an exported struct that itself satisfies
// the marker method would re-open the seal, because a promoted method
// counts for interface satisfaction the same as a directly-declared one.
type Reversibility interface {
	isReversibility()
}

// reversibleStep is one of the two sealed Reversibility implementations: a
// step whose effect can be undone by applying inverse.
type reversibleStep struct {
	inverse ApplyFunc
}

func (reversibleStep) isReversibility() {}

// irreversibleStep is the other sealed Reversibility implementation: a
// step whose effect cannot be undone, with reason stating why not.
type irreversibleStep struct {
	reason string
}

func (irreversibleStep) isReversibility() {}

// Reversible declares a step reversible by inverse, the ApplyFunc that
// undoes its effect. Reversible panics when inverse is nil: a step
// claiming to be reversible with no actual inverse is not a representable
// state, mirroring NewStep's own nil checks (D-02).
func Reversible(inverse ApplyFunc) Reversibility {
	if inverse == nil {
		panic("migrate.Reversible: inverse must be non-nil")
	}
	return reversibleStep{inverse: inverse}
}

// Irreversible declares a step irreversible, naming reason so an operator
// staring at a failed revert request knows WHY the step refused rather
// than merely that it did. Irreversible panics on an empty reason. For a
// step registered inside migrate.Registry — which must remain a
// package-level var literal per the // PHASE4: marker in registry.go — this
// panic fires at package initialization, before any test in the binary
// runs (D-03). PA-1 records that this guarantee depends on that
// placement: nothing in this phase's empty production Registry exercises
// the init-time half, because this phase registers no steps at all.
func Irreversible(reason string) Reversibility {
	if reason == "" {
		panic("migrate.Irreversible: reason must be non-empty")
	}
	return irreversibleStep{reason: reason}
}

// Inverse returns r's inverse ApplyFunc and true when r is reversible, or
// (nil, false) for an irreversible r or a nil r. Exists so the two sealed
// types stay unexported: a caller (Phase 4's revert path) reads a step's
// reversibility through this accessor rather than a type assertion against
// an exported type.
func Inverse(r Reversibility) (ApplyFunc, bool) {
	if rs, ok := r.(reversibleStep); ok {
		return rs.inverse, true
	}
	return nil, false
}

// IrreversibleReason returns r's stated reason and true when r is
// irreversible, or ("", false) for a reversible r or a nil r. Mirrors
// Inverse for the other sealed variant.
func IrreversibleReason(r Reversibility) (string, bool) {
	if is, ok := r.(irreversibleStep); ok {
		return is.reason, true
	}
	return "", false
}

// Step is one registered migration step: a From/To version transition, the
// payload keys it declares it adds, its Reversibility, and its ApplyFunc.
// Every field is unexported — there is no exported struct-literal path —
// so NewStep is the only way to construct a Step, and every field it
// requires is therefore guaranteed present (D-02).
type Step struct {
	from     Version
	to       Version
	addsKeys []string
	rev      Reversibility
	apply    ApplyFunc
}

// NewStep is the only exported constructor for Step. from, to, addsKeys,
// rev and apply are all positional required parameters, so a caller cannot
// OMIT addsKeys or rev. Positional-required alone is not sufficient,
// though: Go permits an explicit nil for an interface- or func-typed
// parameter no matter how the signature is shaped, so without an explicit
// check "nobody declared reversibility" and "nobody declared how to apply
// this step" would both remain representable states. The nil checks below
// are what actually make SC3 true: NewStep panics when rev is nil and when
// apply is nil. addsKeys is defensively cloned via slices.Clone so a
// caller mutating its own slice after registration cannot desynchronize
// the declaration AddsKeys() returns from what was recorded at
// construction time.
func NewStep(from, to Version, addsKeys []string, rev Reversibility, apply ApplyFunc) Step {
	if rev == nil {
		panic("migrate.NewStep: rev must be non-nil")
	}
	if apply == nil {
		panic("migrate.NewStep: apply must be non-nil")
	}
	return Step{
		from:     from,
		to:       to,
		addsKeys: slices.Clone(addsKeys),
		rev:      rev,
		apply:    apply,
	}
}

// From returns the version this step transitions from.
func (s Step) From() Version { return s.from }

// To returns the version this step transitions to.
func (s Step) To() Version { return s.to }

// AddsKeys returns a clone of the payload keys this step declares it adds,
// so a caller cannot mutate the Step's own record of its declaration
// through the returned slice.
func (s Step) AddsKeys() []string { return slices.Clone(s.addsKeys) }

// Reversibility returns this step's declared Reversibility.
func (s Step) Reversibility() Reversibility { return s.rev }

// Apply runs this step's ApplyFunc against payload.
func (s Step) Apply(payload map[string]any) (map[string]any, error) {
	return s.apply(payload)
}
