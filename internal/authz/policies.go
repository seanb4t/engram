// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package authz is a self-contained cedar-go policy decision point (PDP). It
// is an oracle the store consults to decide whether a (principal, action,
// resource) triple is allowed — it is never a handler-level gate itself
// (internal/server never imports this package). internal/store is the sole
// enforcement chokepoint: it asks authz for a decision and translates that
// decision into the Qdrant filter/gate it already builds today.
package authz

import (
	"embed"
	"fmt"

	"github.com/cedar-policy/cedar-go"
)

// all four default policies are compiled directly into the binary — this
// phase ships embedded defaults only, no ENGRAM_AUTHZ_POLICY_DIR, no
// hot-reload, no config surface (D-09).
//
//go:embed policies/*.cedar
var policyFS embed.FS

// policyFiles maps each embedded file to the policy ID its Diagnostic should
// report — named ids make debug-level diagnostic logging actually useful for
// operators, instead of anonymous "policy0"/"policy1" auto-ids.
var policyFiles = map[string]cedar.PolicyID{
	"policies/own_records.cedar":         "own-records",
	"policies/shared_read.cedar":         "shared-read",
	"policies/tenant_isolate.cedar":      "tenant-isolate",
	"policies/defense_empty_owner.cedar": "defense-empty-owner",
}

// loadDefault parses the embedded four-policy corpus into one PolicySet with
// named policy IDs.
func loadDefault() (*cedar.PolicySet, error) {
	ps := cedar.NewPolicySet()
	for path, id := range policyFiles {
		b, err := policyFS.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("authz: read %s: %w", path, err)
		}
		var p cedar.Policy
		if err := p.UnmarshalCedar(b); err != nil {
			return nil, fmt.Errorf("authz: parse %s: %w", path, err)
		}
		ps.Add(id, &p)
	}
	return ps, nil
}

// MustDefault parses the embedded default policy corpus and returns a PDP
// backed by it. It panics on a corpus parse failure rather than returning a
// broken or nil PolicySet that would default-deny everything at runtime. This
// is safe by construction: the SAME embedded bytes are parsed by
// policy_corpus_test.go on every CI run, so a parse failure is caught before
// any binary reaches a deploy path — mirrors internal/webauth/static.go's
// panic-on-build-time-impossible-failure convention for a compiled-in asset.
func MustDefault() *PDP {
	ps, err := loadDefault()
	if err != nil {
		panic(fmt.Sprintf("internal/authz: embedded default policy corpus failed to parse: %v", err))
	}
	return &PDP{policies: ps}
}
