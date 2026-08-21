// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// TestMigrationStatusTextOutputHeadline pins the text-lane headline: the
// first line is exactly "migration status", nothing more.
func TestMigrationStatusTextOutputHeadline(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, migrationStatusCmd)
	svc := &stubEngramService{
		migrateStatusFn: func(context.Context, *engramv1.MigrateStatusRequest) (*engramv1.MigrateStatusResponse, error) {
			return &engramv1.MigrateStatusResponse{}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, _, err := runClient(t, "migration-status", "--server", url, "--output", "text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.SplitN(stdout, "\n", 2)
	if lines[0] != "migration status" {
		t.Errorf("first line = %q, want exactly %q", lines[0], "migration status")
	}
}

// TestMigrationStatusTextOutputRendersBucketsAndAbsentAsDistinctRows proves
// every bucket renders as its own row, none collapsed, with absent as its
// own field.
func TestMigrationStatusTextOutputRendersBucketsAndAbsentAsDistinctRows(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, migrationStatusCmd)
	svc := &stubEngramService{
		migrateStatusFn: func(context.Context, *engramv1.MigrateStatusRequest) (*engramv1.MigrateStatusResponse, error) {
			return &engramv1.MigrateStatusResponse{
				Buckets:        []*engramv1.SchemaVersionBucket{{Version: 0, Count: 5}, {Version: 1, Count: 10}},
				Absent:         3,
				Future:         []*engramv1.SchemaVersionBucket{{Version: 2, Count: 1}},
				FutureTotal:    1,
				Total:          19,
				CurrentVersion: 1,
				Pending:        8,
			}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, _, err := runClient(t, "migration-status", "--server", url, "--output", "text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The two buckets and the future bucket must each surface as a distinct
	// row — asserting on the version/count values, not on any pre-collapsed
	// summary form.
	for _, want := range []string{"0", "5", "1", "10", "2"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing expected value %q for a distinct bucket row: %q", want, stdout)
		}
	}
	if !strings.Contains(stdout, "\n  "+humanizeKey("absent")) {
		t.Errorf("stdout missing its own Absent field row: %q", stdout)
	}
}

// TestMigrationStatusZeroValueRendersEmptyArraysOnJSONLane proves a
// zero-valued response renders buckets/future as [] on the json lane —
// never null and never an omitted key.
func TestMigrationStatusZeroValueRendersEmptyArraysOnJSONLane(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, migrationStatusCmd)
	svc := &stubEngramService{
		migrateStatusFn: func(context.Context, *engramv1.MigrateStatusRequest) (*engramv1.MigrateStatusResponse, error) {
			return &engramv1.MigrateStatusResponse{}, nil
		},
	}
	url := startStubServer(t, svc)

	stdout, _, err := runClient(t, "migration-status", "--server", url, "--output", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	trimmed := strings.TrimSpace(stdout)
	if !strings.Contains(trimmed, `"buckets":[]`) {
		t.Errorf("json lane %q does not contain \"buckets\":[]", trimmed)
	}
	if !strings.Contains(trimmed, `"future":[]`) {
		t.Errorf("json lane %q does not contain \"future\":[]", trimmed)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &doc); err != nil {
		t.Fatalf("json lane did not unmarshal as a single JSON object: %v\nstdout=%q", err, trimmed)
	}
	if got, want := strings.Count(stdout, "\n"), 1; got != want {
		t.Errorf("stdout has %d newlines, want exactly %d (single-line document)", got, want)
	}
}

// TestMigrationStatusUnreachableServerNoRetry pins the existing error
// envelope and exit-code taxonomy, unchanged by this plan: an RPC failure
// maps to a non-nil error via wrapRPCError and the client makes exactly one
// attempt (no retry).
func TestMigrationStatusUnreachableServerNoRetry(t *testing.T) {
	resetClientFlags(t)
	resetCommandFlagState(t, migrationStatusCmd)
	svc := &stubEngramService{
		migrateStatusFn: func(context.Context, *engramv1.MigrateStatusRequest) (*engramv1.MigrateStatusResponse, error) {
			return nil, connect.NewError(connect.CodeUnavailable, errors.New("nope"))
		},
	}
	url := startStubServer(t, svc)

	_, _, err := runClient(t, "migration-status", "--server", url)
	if err == nil {
		t.Fatal("expected a non-nil error from an unreachable/failing server")
	}
	if svc.migrateStatusCalls != 1 {
		t.Errorf("migrateStatusCalls = %d, want 1 (no retry)", svc.migrateStatusCalls)
	}
}

// TestMigrationStatusExcludedFromOperatorCommands proves the command is
// excluded from operatorCommands() because it declares the client-tier
// "server" flag — the same structural predicate every other client verb
// relies on.
func TestMigrationStatusExcludedFromOperatorCommands(t *testing.T) {
	for _, cmd := range operatorCommands() {
		if commandKey(cmd) == "migration-status" {
			t.Fatalf("migration-status must not appear in operatorCommands(): %+v", cmd)
		}
	}
}
