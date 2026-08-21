// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import "github.com/seanb4t/engram/internal/surfaces"

// sweepScopeRule looks up the registered
// surfaces.RuleSweepScopeOrAllScopesRequired rule, the one composition
// point requireSweepScope and every sweep leaf's --all-scopes Usage
// composition share -- so the rejection and the published help text can
// never drift from each other. Mirrors spine_review_purge.go's
// requirePurgeFilterScope/init() panic-on-missing template.
func sweepScopeRule() surfaces.ConditionalRule {
	rule, ok := surfaces.RuleByID(surfaces.RuleSweepScopeOrAllScopesRequired)
	if !ok {
		panic("sweep leaves: surfaces.RuleSweepScopeOrAllScopesRequired is not registered in internal/surfaces/rules.go")
	}
	return rule
}

// requireSweepScope enforces the sweep --scope-or-all-scopes constraint
// shared by every sweep-style operator leaf (spine-review scan,
// spine-review verify, summarize-missing): an explicit --scope or
// --all-scopes is required. Composes the registered
// surfaces.RuleSweepScopeOrAllScopesRequired rule's Sentence into the
// rejection -- never a bare, unregistered usage check.
func requireSweepScope(scope string, allScopes bool) error {
	if scope != "" || allScopes {
		return nil
	}
	return usageErrorf("%s", sweepScopeRule().Sentence)
}
