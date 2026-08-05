// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import "github.com/seanb4t/engram/internal/surfaces"

// conditionalErrf constructs an argError from a declared conditional rule —
// D-04's compiler-enforced path. It accepts only a surfaces.ConditionalRule
// value, so a conditional rejection can never be built from a loose literal
// at the call site; the rule's Fields/Hint/Sentence are the ONLY inputs, and
// they come from the single registry internal/surfaces declares. It
// forwards to argErrFieldsf, so argFieldsOf/argHintOf/argClassOf and
// connectError's errors.As(&argError{}) handling need zero changes — only
// construction call sites change.
func conditionalErrf(class argClass, rule surfaces.ConditionalRule) error {
	return argErrFieldsf(class, HintCode(rule.Hint), rule.Fields, rule.Sentence)
}
