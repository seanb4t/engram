// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func FuzzParseConnectWindowBound(f *testing.F) {
	for _, seed := range []string{"", "2026-10-31T12:00:00Z", "2026-10-31T12:00:00.123456789+05:30", "2026-13-01T00:00:00Z", "yesterday"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		for _, field := range []string{"created_after", "created_before"} {
			got, err := parseConnectWindowBound(field, raw)
			if err != nil {
				var ae *argError
				if !errors.As(err, &ae) || !slices.Equal(ae.Fields, []string{field}) || ae.Hint != HintFormat {
					t.Fatalf("parseConnectWindowBound(%q, %q) = %v, want field=%s hint=format", field, raw, err, field)
				}
				continue
			}
			if raw == "" {
				if !got.IsZero() {
					t.Fatalf("empty input parsed to %v", got)
				}
				continue
			}
			back, err := time.Parse(time.RFC3339, got.Format(time.RFC3339Nano))
			if err != nil || !back.Equal(got) {
				t.Fatalf("%q parsed to %v, which formats back to %v (err %v)", raw, got, back, err)
			}
		}
	})
}
