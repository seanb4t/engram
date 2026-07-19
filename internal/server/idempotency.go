// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
)

// engramIdempotencyNS is the fixed UUIDv5 namespace for deterministic
// idempotency point IDs (Phase 24, D-04). This value is LOAD-BEARING and MUST
// NEVER change once shipped: changing it silently un-dedups every previously
// keyed record on its next replay (a new namespace produces a wholly
// different point ID for the same (owner, scope, key) tuple, so the old
// point is orphaned and a fresh one is minted instead of replaced).
var engramIdempotencyNS = uuid.MustParse("69fbe3e4-a53b-4d6e-971a-cad2f107e23c")

// idempotencyPointID derives the deterministic Qdrant point ID for a keyed
// store_memory/schedule_memory call (Phase 24, D-02/D-03). Same (owner,
// scope, key) always yields the same ID (so a replay is a native
// Upsert-replace, SC1/SC4); the three components are combined via an
// injective length-prefixed encoding — extending internal/auth's
// namespacedOwner discipline from two components to three — so a boundary
// shift (owner="a",scope="bc" vs owner="ab",scope="c") can never collide
// (D-04, T-24-01). Owner is part of the hash input, not a filter, so
// cross-owner collision is structurally impossible (D-09) between two
// AUTHENTICATED owners. This guarantee collapses to a single shared bucket
// when no OIDC issuer is configured: every anonymous caller has owner=="",
// so anonymous callers using the same scope+key CAN collide on this point ID
// (IN-02) — consistent with the project's documented single-anonymous-bucket
// invariant (CLAUDE.md: "No issuer → single anonymous bucket"), not a defect
// specific to idempotency.
func idempotencyPointID(owner, scope, key string) string {
	name := fmt.Sprintf("%d:%s:%d:%s:%d:%s",
		len(owner), owner, len(scope), scope, len(key), key)
	return uuid.NewSHA1(engramIdempotencyNS, []byte(name)).String()
}

// contentFingerprint hashes the client-authored identity fields of a
// store_memory/schedule_memory call (Phase 24, D-06/D-07). It is computed
// from the incoming args at write time and compared against the stored
// IdempotencyFingerprint on replay to detect same-key/different-content. Tags
// are sorted (a copy — the caller's slice is never mutated) and every field
// is length-prefixed in a FIXED order, so the result is deterministic across
// process restarts and Go slice/map iteration order. The idempotency_key
// itself is deliberately NOT an input — its job ends once hashed into the
// point ID (idempotencyPointID above) — and neither is any server-set field
// (short_id, embedder identity, summary fill), so a legitimate replay after
// an async summary fill still matches.
func contentFingerprint(a storeArgs) string {
	tags := slices.Clone(a.Tags)
	slices.Sort(tags)

	// Tags are encoded with the SAME per-element length-prefixed discipline
	// idempotencyPointID uses for its own components (WR-01): a raw
	// strings.Join is not injective over the tag slice — a tag containing a
	// literal separator byte could collapse two distinct tag sets into the
	// same joined string. Length-prefixing each tag individually before it
	// ever reaches the outer field-length-prefix below closes that gap.
	var tagsEnc strings.Builder
	for _, t := range tags {
		fmt.Fprintf(&tagsEnc, "%d:%s:", len(t), t)
	}

	var b strings.Builder
	for _, f := range []string{
		a.Content, a.Category, tagsEnc.String(),
		a.Source, a.Repo, a.Workspace, a.Worktree, a.BaseDir, a.Summary,
	} {
		fmt.Fprintf(&b, "%d:%s:", len(f), f)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
