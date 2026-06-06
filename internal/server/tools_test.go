// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import "testing"

func TestValidateStoreDiscovery(t *testing.T) {
	good := storeDiscoveryArgs{
		Content: "x", Kind: "map", Scope: "discovery:repo:X",
		Citations: []citationArg{{Kind: "file", Ref: "f.go"}},
	}
	if err := validateStoreDiscovery(good); err != nil {
		t.Errorf("valid args rejected: %v", err)
	}
	bad := []struct {
		name string
		a    storeDiscoveryArgs
	}{
		{"bad kind", storeDiscoveryArgs{Content: "x", Kind: "blob", Scope: "s", Citations: []citationArg{{Kind: "file", Ref: "f"}}}},
		{"empty kind", storeDiscoveryArgs{Content: "x", Kind: "", Scope: "s", Citations: []citationArg{{Kind: "file", Ref: "f"}}}},
		{"no citations", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: "s"}},
		{"empty content", storeDiscoveryArgs{Content: "", Kind: "fact", Scope: "s", Citations: []citationArg{{Kind: "file", Ref: "f"}}}},
		{"empty scope", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: "", Citations: []citationArg{{Kind: "file", Ref: "f"}}}},
		{"empty citation ref", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: "s", Citations: []citationArg{{Kind: "file", Ref: ""}}}},
		{"invalid citation kind", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: "s", Citations: []citationArg{{Kind: "blob", Ref: "f"}}}},
		{"empty citation kind", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: "s", Citations: []citationArg{{Kind: "", Ref: "f"}}}},
		{"second citation bad", storeDiscoveryArgs{Content: "x", Kind: "fact", Scope: "s", Citations: []citationArg{{Kind: "file", Ref: "ok"}, {Kind: "url", Ref: ""}}}},
	}
	for _, tc := range bad {
		if err := validateStoreDiscovery(tc.a); err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}

func TestEffectiveDiscoveryScope(t *testing.T) {
	// cross_spine=false requires a scope.
	if _, err := effectiveDiscoveryScope(searchDiscoveryArgs{CrossSpine: false, Scope: ""}); err == nil {
		t.Error("expected error: scope required when cross_spine=false")
	}
	got, err := effectiveDiscoveryScope(searchDiscoveryArgs{CrossSpine: false, Scope: "discovery:repo:X"})
	if err != nil || got != "discovery:repo:X" {
		t.Errorf("scoped: got %q err %v", got, err)
	}
	// cross_spine=true spans all scopes (effective scope empty), scope ignored.
	got, err = effectiveDiscoveryScope(searchDiscoveryArgs{CrossSpine: true, Scope: "discovery:repo:X"})
	if err != nil || got != "" {
		t.Errorf("cross_spine: got %q err %v", got, err)
	}
}
