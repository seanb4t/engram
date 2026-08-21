// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"strings"
	"time"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// memoryStateWords is the ONLY place the Go surface computes a record's
// state label (D-13). It derives from the same wire fields the console's TS
// derivation reads, so neither surface can invent a state the other cannot
// express: archived_at present -> "archived", superseded_by present ->
// "superseded", the validity window closed -> "expired" or "scheduled".
// Words are appended in a fixed canonical order (descending operator-action
// finality, per the UI-SPEC): archived, superseded, expired, scheduled.
//
// expired and scheduled are mutually exclusive by construction: expired is
// evaluated FIRST and, when emitted, suppresses scheduled. This precedence
// is defined HERE, in the derivation — not inherited from write-time
// validation. RuleWindowOrdering (internal/surfaces/rules.go) forbids
// WRITING not_before >= not_after, but that is a write-time rule; the wire
// can still carry an inverted window (not_before and not_after are
// independent proto fields, proto/engram/v1/engram.proto), for example from
// a legacy record predating the rule. Without an explicit precedence such a
// record would render both "expired" and "scheduled", breaking the
// mutual-exclusion invariant the console's badge cap and this vocabulary
// lock both rest on.
//
// Boundary comparisons mirror internal/store's activeWindowConditions
// exactly: not_before uses an inclusive Lte bound (not_before == now yields
// NO "scheduled" word), not_after uses an exclusive Gt bound (not_after ==
// now yields "expired").
//
// Returns nil (not a non-nil empty slice) when the record carries no state.
func memoryStateWords(m *engramv1.Memory, now time.Time) []string {
	var words []string
	if m.GetArchivedAt() != nil {
		words = append(words, "archived")
	}
	// superseded_by is a generated *string field with no Has-shaped
	// accessor; GetSupersededBy() != "" is the presence check. An
	// empty-string superseded_by never legitimately occurs — the store only
	// ever sets it to a non-empty target id — so this is presence, not a
	// value comparison.
	if m.GetSupersededBy() != "" {
		words = append(words, "superseded")
	}
	expired := false
	if na := m.GetNotAfter(); na != nil && !na.AsTime().After(now) {
		// not_after set and <= now (exclusive Gt bound in the store's gate:
		// not_after == now is already expired).
		words = append(words, "expired")
		expired = true
	}
	if !expired {
		if nb := m.GetNotBefore(); nb != nil && nb.AsTime().After(now) {
			// not_before set and strictly after now (inclusive Lte bound in
			// the store's gate: not_before == now is already active, no word).
			words = append(words, "scheduled")
		}
	}
	return words
}

// memoryStateCell renders memoryStateWords as the STATE table cell: a
// comma-joined token with NO space, so the cell reads as one token to
// cut/awk-style downstream processing, and no Unicode glyphs or ANSI escape
// sequences — text/tabwriter counts an embedded escape sequence as
// visible-width runes and would misalign every other row sharing the
// column, and skipping colour entirely means the text lane renders
// byte-identically whether stdout is a TTY, piped, or run under NO_COLOR.
func memoryStateCell(m *engramv1.Memory, now time.Time) string {
	return strings.Join(memoryStateWords(m, now), ",")
}
