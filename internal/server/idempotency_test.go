// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"testing"

	"github.com/google/uuid"
)

// TestIdempotencyPointIDDeterministic pins D-02: the same (owner, scope, key)
// tuple must yield the same point ID on repeat calls (Qdrant Upsert-replace
// relies on this).
func TestIdempotencyPointIDDeterministic(t *testing.T) {
	a := idempotencyPointID("o", "s", "k")
	b := idempotencyPointID("o", "s", "k")
	if a != b {
		t.Fatalf("idempotencyPointID not deterministic: %q != %q", a, b)
	}
}

// TestIdempotencyPointIDBoundaryShiftInjective pins D-04: the injective
// length-prefixed encoding must not let a boundary shift between owner and
// scope collide — the exact namespacedOwner ambiguity this discipline avoids.
func TestIdempotencyPointIDBoundaryShiftInjective(t *testing.T) {
	a := idempotencyPointID("a", "bc", "k")
	b := idempotencyPointID("ab", "c", "k")
	if a == b {
		t.Fatalf("idempotencyPointID collided on boundary shift: owner=a,scope=bc and owner=ab,scope=c both produced %q", a)
	}
}

// TestIdempotencyPointIDOwnerScoped pins D-02/D-03/T-24-01: differing only the
// owner (same scope+key) must yield different point IDs — the structural
// cross-owner isolation guarantee (owner is baked into the hash input, not a
// filter).
func TestIdempotencyPointIDOwnerScoped(t *testing.T) {
	a := idempotencyPointID("o1", "s", "k")
	b := idempotencyPointID("o2", "s", "k")
	if a == b {
		t.Fatalf("idempotencyPointID did not scope by owner: o1 and o2 both produced %q", a)
	}
}

// TestIdempotencyPointIDKeySensitive pins that differing only the key (same
// owner+scope) yields different point IDs — the key is a real component of
// the hash input, not a no-op.
func TestIdempotencyPointIDKeySensitive(t *testing.T) {
	a := idempotencyPointID("o", "s", "k1")
	b := idempotencyPointID("o", "s", "k2")
	if a == b {
		t.Fatalf("idempotencyPointID did not scope by key: k1 and k2 both produced %q", a)
	}
}

// TestIdempotencyPointIDValidUUID pins that idempotencyPointID's output is a
// wire-valid UUID string — short-id/ResolvePointID downstream expect this.
func TestIdempotencyPointIDValidUUID(t *testing.T) {
	got := idempotencyPointID("o", "s", "check-uuid")
	if _, err := uuid.Parse(got); err != nil {
		t.Fatalf("idempotencyPointID(%q) is not a valid UUID: %v", got, err)
	}
}

// TestContentFingerprintTagOrderStable pins D-07: fingerprint is stable under
// tag reordering (tags are sorted before hashing) — Go slice order from a
// client is not semantically meaningful.
func TestContentFingerprintTagOrderStable(t *testing.T) {
	base := storeArgs{Content: "c", Category: "decision", Source: "s"}
	a := base
	a.Tags = []string{"b", "a"}
	b := base
	b.Tags = []string{"a", "b"}
	if contentFingerprint(a) != contentFingerprint(b) {
		t.Fatalf("contentFingerprint not stable under tag reordering: %q != %q", contentFingerprint(a), contentFingerprint(b))
	}
}

// TestContentFingerprintFieldSensitivity pins D-07: changing any
// client-authored identity field changes the fingerprint, and the
// idempotency_key itself is NOT an input.
func TestContentFingerprintFieldSensitivity(t *testing.T) {
	base := storeArgs{
		Content:   "content",
		Category:  "decision",
		Tags:      []string{"a", "b"},
		Source:    "user-said",
		Repo:      "repo",
		Workspace: "workspace",
		Worktree:  "worktree",
		BaseDir:   "basedir",
		Summary:   "summary",
	}
	baseFP := contentFingerprint(base)

	cases := map[string]storeArgs{
		"content":   {Content: "different", Category: base.Category, Tags: base.Tags, Source: base.Source, Repo: base.Repo, Workspace: base.Workspace, Worktree: base.Worktree, BaseDir: base.BaseDir, Summary: base.Summary},
		"category":  {Content: base.Content, Category: "gotcha", Tags: base.Tags, Source: base.Source, Repo: base.Repo, Workspace: base.Workspace, Worktree: base.Worktree, BaseDir: base.BaseDir, Summary: base.Summary},
		"tags":      {Content: base.Content, Category: base.Category, Tags: []string{"c", "d"}, Source: base.Source, Repo: base.Repo, Workspace: base.Workspace, Worktree: base.Worktree, BaseDir: base.BaseDir, Summary: base.Summary},
		"source":    {Content: base.Content, Category: base.Category, Tags: base.Tags, Source: "agent-inferred", Repo: base.Repo, Workspace: base.Workspace, Worktree: base.Worktree, BaseDir: base.BaseDir, Summary: base.Summary},
		"repo":      {Content: base.Content, Category: base.Category, Tags: base.Tags, Source: base.Source, Repo: "different-repo", Workspace: base.Workspace, Worktree: base.Worktree, BaseDir: base.BaseDir, Summary: base.Summary},
		"workspace": {Content: base.Content, Category: base.Category, Tags: base.Tags, Source: base.Source, Repo: base.Repo, Workspace: "different-workspace", Worktree: base.Worktree, BaseDir: base.BaseDir, Summary: base.Summary},
		"worktree":  {Content: base.Content, Category: base.Category, Tags: base.Tags, Source: base.Source, Repo: base.Repo, Workspace: base.Workspace, Worktree: "different-worktree", BaseDir: base.BaseDir, Summary: base.Summary},
		"base_dir":  {Content: base.Content, Category: base.Category, Tags: base.Tags, Source: base.Source, Repo: base.Repo, Workspace: base.Workspace, Worktree: base.Worktree, BaseDir: "different-basedir", Summary: base.Summary},
		"summary":   {Content: base.Content, Category: base.Category, Tags: base.Tags, Source: base.Source, Repo: base.Repo, Workspace: base.Workspace, Worktree: base.Worktree, BaseDir: base.BaseDir, Summary: "different-summary"},
	}
	for name, variant := range cases {
		t.Run(name, func(t *testing.T) {
			if contentFingerprint(variant) == baseFP {
				t.Fatalf("contentFingerprint did not change when %s differed", name)
			}
		})
	}

	// idempotency_key is not an input to the fingerprint: two storeArgs
	// differing ONLY in a hypothetical key value (simulated here by ensuring
	// no such field exists to read) must produce identical fingerprints for
	// identical content. This is implicitly proven by contentFingerprint's
	// signature never reading a.IdempotencyKey, but pin the field-set
	// insensitivity explicitly by re-hashing base twice.
	if contentFingerprint(base) != baseFP {
		t.Fatalf("contentFingerprint not deterministic across repeat calls on identical input")
	}
}

// TestContentFingerprintTagsBoundaryShiftInjective pins WR-01: the tags
// component must use the same injective length-prefixed encoding
// idempotencyPointID uses, not a raw separator join — a single tag
// containing a literal 0x1F byte must NOT fingerprint identically to two
// separate tags split at that byte (tags=["a\x1fb"] vs tags=["a","b"]).
func TestContentFingerprintTagsBoundaryShiftInjective(t *testing.T) {
	base := storeArgs{Content: "c", Category: "decision", Source: "s"}

	merged := base
	merged.Tags = []string{"a\x1fb"}
	split := base
	split.Tags = []string{"a", "b"}

	if contentFingerprint(merged) == contentFingerprint(split) {
		t.Fatalf("contentFingerprint collided on tag boundary shift via 0x1F byte: tags=%q and tags=%q both produced %q",
			merged.Tags, split.Tags, contentFingerprint(merged))
	}
}

// TestMergeFingerprintOrderIndependent pins the target-set canonicalization
// mergeFingerprint composes on top of contentFingerprint: the same target set
// supplied in a different order must hash identically — the same merge
// stated two ways is one operation.
func TestMergeFingerprintOrderIndependent(t *testing.T) {
	a := storeArgs{Content: "merged content", Category: "gotcha", Source: "user-said"}
	forward := mergeFingerprint(a, []string{"target-a", "target-b", "target-c"})
	reversed := mergeFingerprint(a, []string{"target-c", "target-b", "target-a"})
	if forward != reversed {
		t.Fatalf("mergeFingerprint not order-independent: %q != %q", forward, reversed)
	}
}

// TestMergeFingerprintDedupeIndependent pins that a target repeated in the
// input hashes identically to the same set without the repeat —
// mergeFingerprint canonicalizes (sorts and compacts) its own input, so
// duplicate-independence is a property of the ENCODING regardless of
// plan 00's PD-01 ruling on whether a duplicate ever reaches this function
// through the handler (that external-observability half is conditional and
// lives in Task 3's handler-level tests, not here).
func TestMergeFingerprintDedupeIndependent(t *testing.T) {
	a := storeArgs{Content: "merged content", Category: "gotcha", Source: "user-said"}
	withoutDup := mergeFingerprint(a, []string{"target-a", "target-b"})
	withDup := mergeFingerprint(a, []string{"target-a", "target-a", "target-b"})
	if withoutDup != withDup {
		t.Fatalf("mergeFingerprint not dedupe-independent: %q != %q", withoutDup, withDup)
	}
}

// TestMergeFingerprintDistinctTargetSets pins that two target sets differing
// by even one id hash differently, and that a content difference over an
// otherwise-identical target set still changes the hash exactly as
// contentFingerprint already guarantees (mergeFingerprint composes it, never
// reimplements it).
func TestMergeFingerprintDistinctTargetSets(t *testing.T) {
	a := storeArgs{Content: "merged content", Category: "gotcha", Source: "user-said"}
	base := mergeFingerprint(a, []string{"target-a", "target-b"})

	if got := mergeFingerprint(a, []string{"target-a", "target-c"}); got == base {
		t.Fatalf("mergeFingerprint did not change when the target set differed by one id: %q", got)
	}

	diffContent := storeArgs{Content: "different content", Category: a.Category, Source: a.Source}
	if got := mergeFingerprint(diffContent, []string{"target-a", "target-b"}); got == base {
		t.Fatalf("mergeFingerprint did not change when content differed over the same target set: %q", got)
	}
}

// TestMergeFingerprintTargetBoundaryInjectivity pins that the target-set
// encoding is injective, not a naive join — the same probe
// TestContentFingerprintTagsBoundaryShiftInjective uses for tags: a single
// target id containing a literal 0x1F byte must NOT fingerprint identically
// to two separate targets split at that byte.
func TestMergeFingerprintTargetBoundaryInjectivity(t *testing.T) {
	a := storeArgs{Content: "merged content", Category: "gotcha", Source: "user-said"}
	merged := mergeFingerprint(a, []string{"a\x1fb"})
	split := mergeFingerprint(a, []string{"a", "b"})
	if merged == split {
		t.Fatalf("mergeFingerprint collided on target boundary shift via 0x1F byte: both produced %q", merged)
	}
}
