// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package migrate

import (
	"fmt"
	"sort"
	"strings"
)

// AddedKeys returns the keys present in after but absent from before,
// SORTED so every caller's comparison is order-independent — map iteration
// order is not stable, and neither this function's callers nor its own
// tests may depend on it.
func AddedKeys(before, after map[string]any) []string {
	return keyDiff(after, before)
}

// RemovedKeys returns the keys present in before but absent from after —
// the mirror of AddedKeys — also sorted.
func RemovedKeys(before, after map[string]any) []string {
	return keyDiff(before, after)
}

// keyDiff returns, sorted, the keys of a that are absent from b.
func keyDiff(a, b map[string]any) []string {
	var out []string
	for k := range a {
		if _, ok := b[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// CheckAdditive proves the additive-only invariant a registered step
// declares (D-04): it returns nil only when BOTH hold between before and
// after — a step's actual observed effect on one record's payload:
//
//  1. RemovedKeys(before, after) is empty — nothing was removed or
//     renamed. On failure the error names every vanished key.
//  2. AddedKeys(before, after) is SET-EQUAL to s.AddsKeys() — not a
//     subset, not a superset. A superset check would pass a step that
//     quietly adds an undeclared key; a subset check would pass a step
//     that declares keys it never actually writes. Either way the
//     declaration would stop describing the step's real behavior, and the
//     declaration is the entire mechanism this function exists to
//     enforce. On failure the error names both differences separately:
//     keys added but not declared, and keys declared but never added. A
//     declared key already carried by before is not a declaration
//     failure — the step's job is to ENSURE the key exists, and it
//     already does; a declared key already PRESENT in before is treated
//     as satisfied, even when after does not additionally "add" it
//     (REVIEWS.md H1). This carve-out is task-limited: a step that
//     declares a key neither adds it NOR finds it pre-existing in before
//     is still caught as "declared key never added" — the carve-out only
//     recognizes a key that is genuinely already there, never one that is
//     simply absent from both before and after.
//
// This comparison is over KEY SETS ONLY. A step that overwrites an
// EXISTING key's VALUE in place — leaving the key set itself unchanged —
// is NOT caught here; that limit is real and is documented at every call
// site rather than left for a future reader to assume a stronger
// guarantee than this function actually provides (T-03-12). Downstream,
// Store.Migrate contains that gap by building its Qdrant write map from
// AddedKeys(original, current) alone, never from the step's full returned
// payload — see TestMigrateWritesOnlyAddedKeys.
func CheckAdditive(s Step, before, after map[string]any) error {
	var parts []string

	if removed := RemovedKeys(before, after); len(removed) > 0 {
		parts = append(parts, fmt.Sprintf("removed key(s) not permitted: %v", removed))
	}

	added := AddedKeys(before, after)
	declared := s.AddsKeys()
	sort.Strings(declared)

	declaredSet := make(map[string]struct{}, len(declared))
	for _, k := range declared {
		declaredSet[k] = struct{}{}
	}
	addedSet := make(map[string]struct{}, len(added))
	for _, k := range added {
		addedSet[k] = struct{}{}
	}

	var undeclared, missing []string
	for _, k := range added {
		if _, ok := declaredSet[k]; !ok {
			undeclared = append(undeclared, k)
		}
	}
	for _, k := range declared {
		if _, ok := addedSet[k]; ok {
			continue
		}
		// A declared key already present in before is satisfied by its
		// pre-existing presence: the step's declaration promises the key
		// exists after applying, and it did before the step ever ran
		// (REVIEWS.md H1's pre-existing-short_id carve-out).
		if _, ok := before[k]; ok {
			continue
		}
		missing = append(missing, k)
	}
	if len(undeclared) > 0 {
		parts = append(parts, fmt.Sprintf("added key(s) not declared in AddsKeys: %v", undeclared))
	}
	if len(missing) > 0 {
		parts = append(parts, fmt.Sprintf("declared key(s) in AddsKeys never added: %v", missing))
	}

	if len(parts) == 0 {
		return nil
	}
	return fmt.Errorf("migrate: step (From=%d To=%d) failed additive-only check: %s",
		s.From(), s.To(), strings.Join(parts, "; "))
}
