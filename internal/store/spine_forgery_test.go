// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package store_test is an EXTERNAL test package for internal/store,
// deliberately not `package store`: the whole point of this file is that a
// same-package composite literal proves nothing here (Go only forbids
// setting an unexported field ACROSS package boundaries), so the
// cross-package forgery proof this plan requires (memory 55zra87def) must
// itself live outside the package it is testing.
package store_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/seanb4t/engram/internal/store"
)

// TestPurgeManifestForgeryRejected is the cross-package proof that a
// hand-built store.PurgeManifest{} literal is rejected before any RPC.
// PurgeManifest's three fields are ALL unexported, so this composite
// literal -- built in this DIFFERENT package -- can only ever produce the
// zero value: verified=false, no matter how faithfully a caller tried to
// reconstruct a real manifest. ApplyPurge's unverified-manifest check runs
// BEFORE derivePurgeEligible touches s.client, so a nil-client Store is
// safe to use here -- this test needs no live Qdrant at all (the plan's
// precondition: "the manifest-forgery and set-math tests are pure and run
// without it").
func TestPurgeManifestForgeryRejected(t *testing.T) {
	forged := store.PurgeManifest{}
	if forged.IsVerified() {
		t.Fatal("a composite literal built outside internal/store reports IsVerified() = true -- the unexported marker was forged")
	}

	s := store.New(nil, "irrelevant-forgery-test-collection")
	_, err := s.ApplyPurge(context.Background(), forged, store.PurgeOptions{})
	if err == nil {
		t.Fatal("ApplyPurge accepted an unverified (forged) manifest -- want an invalid-argument-class rejection before any RPC")
	}
	if !errors.Is(err, store.ErrInvalidArgument) {
		t.Fatalf("ApplyPurge(forged manifest) error = %v, want it to wrap store.ErrInvalidArgument", err)
	}
}

// TestPurgeManifestExportedFieldsEmpty enumerates PurgeManifest's exported
// FIELDS via reflection and asserts the set is empty -- every field is
// unexported, so no composite literal outside internal/store can set any of
// them. This subsumes and replaces a grep for a declaration line, which
// would pass even if an exported twin field were added elsewhere in the
// struct.
func TestPurgeManifestExportedFieldsEmpty(t *testing.T) {
	typ := reflect.TypeOf(store.PurgeManifest{})
	var exported []string
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.IsExported() {
			exported = append(exported, f.Name)
		}
	}
	if len(exported) != 0 {
		t.Fatalf("PurgeManifest exported fields = %v, want none", exported)
	}
}

// TestPurgeManifestExportedMethodSet is the whole gate on the absence of a
// serialization surface: reflect.TypeOf (value receiver methods) and
// reflect.TypeOf of the pointer type (so pointer-receiver methods are
// counted too) must together expose EXACTLY {IsVerified, IDs, DerivedAt} --
// no encoder, no MarshalJSON, no String(), no accessor yielding
// transportable bytes. Adding one later fails this equality. The previous
// revision paired this with "a compile-time-checked file that must NOT
// build"; that criterion is deliberately NOT reintroduced here (a
// non-building file cannot live in the tree, so it reduces to a SUMMARY
// narrative about a `go build` the executor says they ran) -- this
// reflection assertion is the real, durable gate.
func TestPurgeManifestExportedMethodSet(t *testing.T) {
	want := map[string]bool{"IsVerified": true, "IDs": true, "DerivedAt": true}
	got := map[string]bool{}

	valType := reflect.TypeOf(store.PurgeManifest{})
	for i := 0; i < valType.NumMethod(); i++ {
		got[valType.Method(i).Name] = true
	}
	ptrType := reflect.TypeOf(&store.PurgeManifest{})
	for i := 0; i < ptrType.NumMethod(); i++ {
		got[ptrType.Method(i).Name] = true
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PurgeManifest exported method set = %v, want exactly %v", got, want)
	}
}
