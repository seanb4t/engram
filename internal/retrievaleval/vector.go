// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package retrievaleval

import (
	"fmt"
	"math"
)

// differMinCosineDistance is the D-13 epsilon: the differ gate passes only
// strictly above this cosine distance. A hosted embedder's float jitter
// between two identical-string embed calls sits far below it, so a genuine
// asymmetric instruction-prefix effect is the only thing that clears the
// bar (2026-09-22.01 Phase 1, #353).
const differMinCosineDistance = 1e-3

// cosineDistance returns 1 minus the cosine similarity of a and b. This is
// the 2026-09-22.01 Phase 1 replacement for the differ gate's bit-identity
// comparison (#353): a hosted embedder's harmless float jitter between two
// calls on the identical string no longer registers as "differs", while a
// genuine instruction-prefix effect still clears differMinCosineDistance.
//
// It returns an error, and never a number, for degenerate input: mismatched
// or zero lengths, a zero-norm vector, or any NaN/±Inf component in either
// vector. Degenerate input is an error because a NaN distance compares
// false against every threshold and would otherwise silently read as a
// pass.
func cosineDistance(a, b []float32) (float64, error) {
	if len(a) == 0 || len(b) == 0 {
		return 0, fmt.Errorf("cosineDistance: empty vector (len(a)=%d len(b)=%d)", len(a), len(b))
	}
	if len(a) != len(b) {
		return 0, fmt.Errorf("cosineDistance: length mismatch (len(a)=%d len(b)=%d)", len(a), len(b))
	}
	for i, v := range a {
		if f := float64(v); math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, fmt.Errorf("cosineDistance: a[%d]=%v is not finite", i, v)
		}
	}
	for i, v := range b {
		if f := float64(v); math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, fmt.Errorf("cosineDistance: b[%d]=%v is not finite", i, v)
		}
	}

	var dot, na, nb float64
	for i := range a {
		av, bv := float64(a[i]), float64(b[i])
		dot += av * bv
		na += av * av
		nb += bv * bv
	}
	if na == 0 || nb == 0 {
		return 0, fmt.Errorf("cosineDistance: zero-norm vector (|a|^2=%v |b|^2=%v)", na, nb)
	}

	return 1 - dot/(math.Sqrt(na)*math.Sqrt(nb)), nil
}
