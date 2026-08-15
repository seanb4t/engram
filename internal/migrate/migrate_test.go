// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package migrate

import (
	"strings"
	"testing"
)

// var _ Version = CurrentVersion is a compile-time assertion that the
// constant carries the named type, not a bare int.
var _ Version = CurrentVersion

// TestCurrentVersionValue pins the cross-phase decision: raising this
// constant is a Phase 3/4 action taken together with registering the step
// that defines the new version, and backfill-short-ids is the registered
// v0->v1 step.
func TestCurrentVersionValue(t *testing.T) {
	if CurrentVersion != Version(1) {
		t.Fatalf("migrate.CurrentVersion = %d, want 1 — raising this constant is a Phase 3/4 action taken together with registering the step that defines the new version (backfill-short-ids is the registered v0->v1 step, registered in this same change), never a standalone bump", CurrentVersion)
	}
}

// TestNewMintingStep proves NewMintingStep's nil-check discipline mirrors
// NewStep's verbatim, and that Step.ApplyMinter() is the only observable
// discriminator between a plain NewStep step and a minter-aware
// NewMintingStep step.
func TestNewMintingStep(t *testing.T) {
	t.Run("panics on nil rev", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("NewMintingStep did not panic on nil rev")
			}
		}()
		NewMintingStep(0, 1, []string{"x"}, nil, func(m map[string]any, mint func() (string, error)) (map[string]any, error) {
			return m, nil
		})
	})

	t.Run("panics on nil applyMinter", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("NewMintingStep did not panic on nil applyMinter")
			}
		}()
		NewMintingStep(0, 1, []string{"x"}, Irreversible("x"), nil)
	})

	t.Run("plain NewStep is not minter-aware", func(t *testing.T) {
		plain := NewStep(0, 1, []string{"x"},
			Reversible(func(p map[string]any) (map[string]any, error) { return p, nil }),
			func(p map[string]any) (map[string]any, error) { return p, nil })
		if fn, ok := plain.ApplyMinter(); ok || fn != nil {
			t.Fatalf("plain.ApplyMinter() = (%v, %v), want (nil, false)", fn, ok)
		}
	})

	t.Run("NewMintingStep is minter-aware", func(t *testing.T) {
		minting := NewMintingStep(0, 1, []string{"x"}, Irreversible("x"),
			func(m map[string]any, mint func() (string, error)) (map[string]any, error) { return m, nil })
		fn, ok := minting.ApplyMinter()
		if !ok || fn == nil {
			t.Fatalf("minting.ApplyMinter() = (%v, %v), want (non-nil, true)", fn, ok)
		}
	})
}

// TestV1FillShortID proves v1FillShortID is idempotent when short_id is
// already present (and never calls mint in that case) and mints-and-adds
// when short_id is absent.
func TestV1FillShortID(t *testing.T) {
	t.Run("idempotent when short_id already present", func(t *testing.T) {
		before := map[string]any{"short_id": "abc123"}
		mint := func() (string, error) {
			t.Fatal("mint must not be called when short_id is already present")
			return "", nil
		}
		after, err := v1FillShortID(before, mint)
		if err != nil {
			t.Fatalf("v1FillShortID() error = %v, want nil", err)
		}
		if after["short_id"] != "abc123" {
			t.Fatalf("v1FillShortID() short_id = %v, want unchanged %q", after["short_id"], "abc123")
		}
	})

	t.Run("mints and adds when absent", func(t *testing.T) {
		before := map[string]any{"content": "x"}
		mint := func() (string, error) { return "newsid0001", nil }
		after, err := v1FillShortID(before, mint)
		if err != nil {
			t.Fatalf("v1FillShortID() error = %v, want nil", err)
		}
		if after["short_id"] != "newsid0001" {
			t.Fatalf("v1FillShortID() short_id = %v, want %q", after["short_id"], "newsid0001")
		}
		if _, ok := before["short_id"]; ok {
			t.Fatal("v1FillShortID mutated the input map in place — must clone before writing")
		}
	})
}

// TestCheckAdditivePreExistingKey proves both directions of REVIEWS.md H1's
// pre-existing-key carve-out: a declared key already present in before
// passes CheckAdditive without the step having added anything, while a
// declared key that is genuinely absent from both before and after still
// fails as "declared key never added" — the carve-out recognizes only
// genuine pre-existence, never mere absence. It also proves the
// undeclared-added-key and removed-key branches are unchanged.
func TestCheckAdditivePreExistingKey(t *testing.T) {
	step := NewMintingStep(0, 1, []string{"short_id"}, Irreversible("x"),
		func(m map[string]any, mint func() (string, error)) (map[string]any, error) { return m, nil })

	t.Run("pre-existing short_id, no schema_version: passes", func(t *testing.T) {
		before := map[string]any{"short_id": "abc123"}
		after := map[string]any{"short_id": "abc123"}
		if err := CheckAdditive(step, before, after); err != nil {
			t.Fatalf("CheckAdditive() = %v, want nil — short_id was already present in before, satisfying the declaration", err)
		}
	})

	t.Run("short_id absent from before and after: still fails", func(t *testing.T) {
		before := map[string]any{}
		after := map[string]any{}
		err := CheckAdditive(step, before, after)
		if err == nil {
			t.Fatal("CheckAdditive() = nil, want an error — the carve-out must not paper over a step that genuinely never adds nor finds short_id")
		}
		if !strings.Contains(err.Error(), "declared key(s) in AddsKeys never added") {
			t.Fatalf("CheckAdditive() error = %q, want it to name the never-added declared key", err.Error())
		}
	})

	t.Run("undeclared added key still fails", func(t *testing.T) {
		before := map[string]any{}
		after := map[string]any{"short_id": "abc", "extra": "y"}
		err := CheckAdditive(step, before, after)
		if err == nil {
			t.Fatal("CheckAdditive() = nil, want an error — an undeclared added key must still be caught")
		}
		if !strings.Contains(err.Error(), "added key(s) not declared") {
			t.Fatalf("CheckAdditive() error = %q, want it to name the undeclared added key", err.Error())
		}
	})

	t.Run("removed key still fails", func(t *testing.T) {
		before := map[string]any{"short_id": "abc", "gone": "x"}
		after := map[string]any{"short_id": "abc"}
		err := CheckAdditive(step, before, after)
		if err == nil {
			t.Fatal("CheckAdditive() = nil, want an error — a removed key must still be caught")
		}
		if !strings.Contains(err.Error(), "removed key(s) not permitted") {
			t.Fatalf("CheckAdditive() error = %q, want it to name the removed key", err.Error())
		}
	})
}
