// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"slices"
	"testing"
	"time"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
	"pgregory.net/rapid"
)

var canonicalStateOrder = []string{"archived", "superseded", "expired", "scheduled"}

// windowOffset draws an optional bound around now, with now itself as a
// likely value because the boundaries are where the rules differ.
func windowOffset(t *rapid.T, label string, now time.Time) *timestamppb.Timestamp {
	if !rapid.Bool().Draw(t, label+"_set") {
		return nil
	}
	secs := rapid.OneOf(rapid.Just(int64(0)), rapid.Int64Range(-3, 3), rapid.Int64Range(-1e6, 1e6)).Draw(t, label)
	return timestamppb.New(now.Add(time.Duration(secs) * time.Second))
}

func TestPropertyDerivedState(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	rapid.Check(t, func(t *rapid.T) {
		m := &engramv1.Memory{NotBefore: windowOffset(t, "not_before", now), NotAfter: windowOffset(t, "not_after", now)}
		if rapid.Bool().Draw(t, "archived") {
			m.ArchivedAt = timestamppb.New(now)
		}
		if rapid.Bool().Draw(t, "superseded") {
			succ := "0123456789"
			m.SupersededBy = &succ
		}

		expired := m.NotAfter != nil && !m.NotAfter.AsTime().After(now)
		var want []string
		for _, w := range canonicalStateOrder {
			switch {
			case w == "archived" && m.ArchivedAt != nil,
				w == "superseded" && m.SupersededBy != nil,
				w == "expired" && expired,
				w == "scheduled" && !expired && m.NotBefore != nil && m.NotBefore.AsTime().After(now):
				want = append(want, w)
			}
		}
		got := memoryStateWords(m, now)
		if !slices.Equal(got, want) {
			t.Fatalf("memoryStateWords = %v, want %v", got, want)
		}
		if slices.Contains(got, "expired") && slices.Contains(got, "scheduled") {
			t.Fatalf("expired did not suppress scheduled: %v", got)
		}
	})
}
