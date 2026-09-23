// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package retrievaleval

import (
	"math"
	"testing"
)

func TestCosineDistance(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name string
			a, b []float32
			want float64
		}{
			{"identical", []float32{1, 0}, []float32{1, 0}, 0},
			{"orthogonal", []float32{1, 0}, []float32{0, 1}, 1},
			{"opposite", []float32{1, 0}, []float32{-1, 0}, 2},
			{"scaled identical", []float32{1, 2}, []float32{2, 4}, 0},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				got, err := cosineDistance(tc.a, tc.b)
				if err != nil {
					t.Fatalf("cosineDistance(%v, %v) unexpected error: %v", tc.a, tc.b, err)
				}
				if math.Abs(got-tc.want) > 1e-9 {
					t.Errorf("cosineDistance(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
				}
			})
		}
	})

	t.Run("near-identical jitter fails the gate", func(t *testing.T) {
		t.Parallel()
		got, err := cosineDistance([]float32{1, 0}, []float32{1, 1e-4})
		if err != nil {
			t.Fatalf("cosineDistance unexpected error: %v", err)
		}
		if !(got > 0) {
			t.Errorf("cosineDistance near-identical = %v, want strictly > 0", got)
		}
		if !(got < differMinCosineDistance) {
			t.Errorf("cosineDistance near-identical = %v, want strictly < differMinCosineDistance (%v) — jitter this small must FAIL the differ gate", got, differMinCosineDistance)
		}
	})

	t.Run("degenerate input errors", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name string
			a, b []float32
		}{
			{"NaN in a", []float32{float32(math.NaN()), 0}, []float32{1, 0}},
			{"+Inf in b", []float32{1, 0}, []float32{float32(math.Inf(1)), 0}},
			{"-Inf in a", []float32{float32(math.Inf(-1)), 0}, []float32{1, 0}},
			{"zero-norm vector", []float32{0, 0}, []float32{1, 0}},
			{"length mismatch", []float32{1, 0, 0}, []float32{1, 0}},
			{"two empty vectors", []float32{}, []float32{}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				if _, err := cosineDistance(tc.a, tc.b); err == nil {
					t.Errorf("cosineDistance(%v, %v) = nil error, want an error", tc.a, tc.b)
				}
			})
		}
	})
}
