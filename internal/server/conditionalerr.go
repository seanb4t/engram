// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"

	"github.com/seanb4t/engram/internal/surfaces"
)

// conditionalErrf constructs an argError from a declared conditional rule —
// D-04's compiler-enforced path. Its parameter type, surfaces.ConditionalRule,
// is a plain exported struct — by itself that only stops a call site from
// hand-typing Fields/Hint/Sentence as inline literals of the WRONG TYPE, not
// from constructing a same-shaped surfaces.ConditionalRule{...} literal that
// was never registered in internal/surfaces' rules slice. The actual
// unforgeability guarantee is rule.IsDeclared(), checked below: it reads an
// unexported provenance marker that only internal/surfaces' own rules
// literal can set, so any surfaces.ConditionalRule value built outside that
// package — no matter how faithfully its Fields/Hint/Sentence are copied —
// carries the marker's zero value and fails this check. A rejection can
// therefore never be built from a loose literal at the call site.
//
// A failing check panics rather than returning a user-facing rejection: an
// undeclared rule reaching a caller as a normal field=/hint= envelope is
// exactly the failure this function exists to prevent, and every real call
// site sources its rule via surfaces.RuleByID against the registry, making
// this branch unreachable by construction — the same gated-unreachable
// shape cmd/engram/catalog.go's catalogBlastRadius panic uses for a command
// missing its surfaces classification.
//
// It forwards to argErrFieldsf, so argFieldsOf/argHintOf/argClassOf and
// connectError's errors.As(&argError{}) handling need zero changes — only
// construction call sites change.
func conditionalErrf(class argClass, rule surfaces.ConditionalRule) error {
	if !rule.IsDeclared() {
		panic(fmt.Sprintf(
			"conditionalErrf: rule %q is not a declared internal/surfaces rule — "+
				"construct it via surfaces.RuleByID against the registry, never as a literal",
			rule.ID,
		))
	}
	return argErrFieldsf(class, HintCode(rule.Hint), rule.Fields, rule.Sentence)
}
