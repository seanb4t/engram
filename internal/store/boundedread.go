// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file hosts the shared bounded-read mechanism's sizing (03-CONTEXT.md
// D-02, D-03, D-04, D-06): a per-record payload ceiling, derived per VIEW
// from the configured write caps (RecordCaps), and two fixed byte budgets
// (rpcByteBudget for one Qdrant RPC, pageByteBudget for one caller-facing
// page). scrollAllPoints (spine.go) sizes every ScrollAndOffset request from
// these — sweepLimit(view) — so no single RPC can overflow for a record that
// respects the caps.
//
// Content is deliberately NOT the dominant term once capped: at
// DefaultRecordCaps(), 50 citations x 16 KiB excerpts each (~850 KiB) dwarf
// the 64 KiB content cap by an order of magnitude. fullRecordCeiling and
// summaryRecordCeiling both account for citations and tags explicitly —
// never derive a ceiling from content alone.
//
// Several payload fields carry NO write cap even after this milestone's D-01/
// D-10 content and tags caps: scope, repo, workspace, worktree_path, base_dir,
// source, the supersedes list, and each citation's ref/locator/pin.
// uncappedFieldsAllowance and citationEntryAllowance budget for these by a
// documented allowance, never a proven bound — capping them is tracked as
// GitHub #589. A record whose actual bytes exceed its view's ceiling because
// of one of these fields is caught by scrollAllPoints' batch-of-1 fallback
// (D-07) and, at worst, fails the sweep with the already-named
// store.ErrResponseTooLarge — it is never silently skipped or truncated.
//
// D-05 (04-CONTEXT.md, Phase 4) revises the scope of pageByteBudget below:
// it bounds cursor-mode responses ONLY. Offset-mode Store.List and
// Store.ListScheduled assemble the full requested count across several
// ordered pages instead of stopping at one page's byte budget — they are
// bounded by count alone (at most MaxRecallLimit records times the view's
// per-record ceiling), never by pageByteBudget.

package store

import (
	"github.com/qdrant/go-client/qdrant"
)

// RecordCaps names the write-side caps a Store's read-side record ceiling is
// derived from. Each field mirrors a registry-declared ENGRAM_MEMORY_MAX_*
// variable (plan 03-01) enforced on the write path in internal/server; this
// package never enforces them itself — it only sizes reads from them.
type RecordCaps struct {
	// ContentBytes mirrors ENGRAM_MEMORY_MAX_CONTENT_BYTES (D-01) — always
	// enforced on write, never disabled.
	ContentBytes int
	// SummaryBytes mirrors ENGRAM_MEMORY_MAX_SUMMARY_BYTES. Unlike
	// ContentBytes/Tags/TagBytes, 0 is a legitimate configured value meaning
	// the write-side bound is disabled — summaryTerm falls back to
	// ContentBytes in that case, since no proof otherwise bounds a summary.
	SummaryBytes int
	// Tags mirrors ENGRAM_MEMORY_MAX_TAGS (D-10) — always enforced.
	Tags int
	// TagBytes mirrors ENGRAM_MEMORY_MAX_TAG_BYTES (D-10) — always enforced.
	TagBytes int
	// Citations mirrors the server's citation count bound
	// (maxDiscoveryCitations, internal/server/tools.go) — always enforced on
	// every memory write path per RESEARCH.md's citations finding.
	Citations int
	// CitationExcerptBytes mirrors the server's per-citation excerpt bound
	// (maxCitationExcerptBytes) — always enforced.
	CitationExcerptBytes int
}

// DefaultRecordCaps returns the registry defaults (plan 03-01) and the
// server's existing citation constants: ContentBytes 65536, SummaryBytes 512,
// Tags 128, TagBytes 128, Citations 50, CitationExcerptBytes 16384. Plan
// 03-03 test-binds this against the registry's own defaults so the two never
// silently drift apart.
func DefaultRecordCaps() RecordCaps {
	return RecordCaps{
		ContentBytes:         65536,
		SummaryBytes:         512,
		Tags:                 128,
		TagBytes:             128,
		Citations:            50,
		CitationExcerptBytes: 16384,
	}
}

// normalizeRecordCaps replaces every non-positive field of c with its
// DefaultRecordCaps() value, EXCEPT SummaryBytes: a value of exactly 0 is
// preserved (the write-side "bound disabled" convention); only a NEGATIVE
// SummaryBytes is replaced, since a negative byte count is never a
// deliberate configuration choice.
func normalizeRecordCaps(c RecordCaps) RecordCaps {
	d := DefaultRecordCaps()
	if c.ContentBytes <= 0 {
		c.ContentBytes = d.ContentBytes
	}
	if c.SummaryBytes < 0 {
		c.SummaryBytes = d.SummaryBytes
	}
	if c.Tags <= 0 {
		c.Tags = d.Tags
	}
	if c.TagBytes <= 0 {
		c.TagBytes = d.TagBytes
	}
	if c.Citations <= 0 {
		c.Citations = d.Citations
	}
	if c.CitationExcerptBytes <= 0 {
		c.CitationExcerptBytes = d.CitationExcerptBytes
	}
	return c
}

// WithRecordCaps configures the Store's read-side record ceiling from c,
// normalized via normalizeRecordCaps. Plan 03-03 wires this with the
// production config-derived caps; a Store built without this option reports
// DefaultRecordCaps() from RecordCaps().
func WithRecordCaps(c RecordCaps) Option {
	norm := normalizeRecordCaps(c)
	return func(s *Store) { s.caps = &norm }
}

// RecordCaps reports the caps this Store's read-side ceilings are derived
// from: the normalized value WithRecordCaps set, or DefaultRecordCaps() when
// unset.
func (s *Store) RecordCaps() RecordCaps {
	if s.caps != nil {
		return *s.caps
	}
	return DefaultRecordCaps()
}

// rpcByteBudget bounds the bytes a single Qdrant RPC (one ScrollAndOffset
// call) may receive, under storetest.RecvLimit's 4 MiB — the D-06 fixed
// budget. A package var, not a const, mirroring spineScrollBatch's own
// test-overridable seam. Independent of Phase 5's MaxCallRecvMsgSize
// backstop (which stays pure defense in depth): this budget shapes the
// REQUEST (the Limit sent), not a client-side receive cap.
var rpcByteBudget = 2 << 20

// pageByteBudget bounds the bytes a caller-facing logical page (potentially
// several RPCs) may accumulate — the D-06 sibling budget to rpcByteBudget.
// Read-side callers that compose several bounded RPCs into one page (the
// ordered-page helper, plan 03-04) stop at this ceiling.
var pageByteBudget = 2 << 20

// tagFramingBytes is the per-tag protobuf/map framing allowance added on top
// of TagBytes when deriving tagsTerm — not itself a write cap.
const tagFramingBytes = 8

// citationEntryAllowance is the allowance for one citation's Kind/Ref/
// Locator/Pin strings plus its struct framing, on top of
// CitationExcerptBytes. Ref/Locator/Pin carry NO write cap (RESEARCH.md's
// citations finding) — this is a documented allowance, not a proven bound;
// capping them is tracked as GitHub #589.
const citationEntryAllowance = 1 << 10

// uncappedFieldsAllowance budgets every payload key with no per-field write
// cap — scope, repo, workspace, worktree_path, base_dir, source, the
// supersedes list, plus fixed-size fields (timestamps, ids, flags,
// schema_version) and protobuf map framing. A documented allowance, not a
// proven bound; capping the uncapped fields is tracked as GitHub #589. A
// record whose actual bytes exceed this allowance is caught by
// scrollAllPoints' batch-of-1 fallback (D-07), never silently skipped.
const uncappedFieldsAllowance = 16 << 10

// keysRecordCeiling is the per-record byte allowance for keysView (D-07): a
// point carrying nothing but its id (a 36-byte UUID, returned unconditionally
// as p.Id — never part of the payload selector) and one RFC3339 created_at
// string, plus protobuf point/map framing. Deliberately NOT derived from
// RecordCaps like fullRecordCeiling/summaryRecordCeiling — the selector
// excludes every capped field, so there is nothing left to derive a ceiling
// from. A small fixed allowance in the low hundreds of bytes comfortably
// covers an RFC3339 timestamp plus framing; scrollOrderedPage's own
// proto.Size measurement (the D-07 batch-of-1 fallback) corrects this at
// runtime if a future encoding is ever tighter than assumed, exactly like
// uncappedFieldsAllowance and citationEntryAllowance above.
const keysRecordCeiling = 256

// summaryTerm is the ceiling contribution of the summary field: SummaryBytes
// when the write-side bound is enabled (> 0), else ContentBytes — with the
// bound disabled, no proof otherwise bounds a summary, so the content cap is
// the budgeted allowance (RESEARCH.md's disabled-summary-bound finding).
func summaryTerm(c RecordCaps) int {
	if c.SummaryBytes > 0 {
		return c.SummaryBytes
	}
	return c.ContentBytes
}

// tagsTerm is the ceiling contribution of the tags list: every tag at its
// byte cap, plus tagFramingBytes of framing each.
func tagsTerm(c RecordCaps) int {
	return c.Tags * (c.TagBytes + tagFramingBytes)
}

// fullRecordCeiling is the TRUE per-record payload ceiling for a full-view
// (or sweep-view) read under c: content + the summary term + the tags term +
// every citation at its excerpt cap plus its entry allowance + the uncapped
// fields allowance. At DefaultRecordCaps() this is 970240 bytes.
func fullRecordCeiling(c RecordCaps) int {
	return c.ContentBytes + summaryTerm(c) + tagsTerm(c) +
		c.Citations*(c.CitationExcerptBytes+citationEntryAllowance) +
		uncappedFieldsAllowance
}

// summaryRecordCeiling is the TRUE per-record payload ceiling for a
// summary-view read under c (content and citations excluded from the
// payload selector, D-04): the summary term + the tags term + the uncapped
// fields allowance. At DefaultRecordCaps() this is 34304 bytes — roughly
// three orders of magnitude below fullRecordCeiling, which is why the two
// views need materially different per-RPC record counts.
func summaryRecordCeiling(c RecordCaps) int {
	return summaryTerm(c) + tagsTerm(c) + uncappedFieldsAllowance
}

// scanRecordCeiling is the TRUE per-record payload ceiling for a scan-view
// (D-04) read under c: the summary term + the tags term + every citation at
// its excerpt cap plus its entry allowance + the uncapped fields allowance —
// content and tags are the two terms scanView's callback never reads.
// Citations still travel (and still cost bytes) even though ScanSpine only
// ever reads their COUNT: Qdrant cannot return an array's length without
// returning the array, so the field cannot be excluded from the selector.
func scanRecordCeiling(c RecordCaps) int {
	return summaryTerm(c) + c.Citations*(c.CitationExcerptBytes+citationEntryAllowance) + uncappedFieldsAllowance
}

// perRPCLimit is floor(rpcByteBudget / maxRecordBytes), floored at 1 so a
// per-record ceiling exceeding the budget still requests exactly one record
// per RPC rather than zero.
func perRPCLimit(maxRecordBytes int) int {
	if maxRecordBytes <= 0 {
		return 1
	}
	return max(1, rpcByteBudget/maxRecordBytes)
}

// readView bundles a payload selector with its per-record byte ceiling
// (D-04): a projection can never be paired with the wrong ceiling, because
// the two travel together as one value. maxRecordBytes == 0 marks an
// unbudgeted view — the count-only, pre-Phase-3 sweep behavior.
type readView struct {
	selector       *qdrant.WithPayloadSelector
	maxRecordBytes int
}

// budgeted reports whether v carries a byte-derived ceiling.
func (v readView) budgeted() bool { return v.maxRecordBytes > 0 }

// fullView is the full-payload readView, sized from s.RecordCaps().
func (s *Store) fullView() readView {
	return readView{selector: qdrant.NewWithPayload(true), maxRecordBytes: fullRecordCeiling(s.RecordCaps())}
}

// summaryView excludes content and citations (D-04) and is sized from
// s.RecordCaps().
func (s *Store) summaryView() readView {
	return readView{
		selector:       qdrant.NewWithPayloadExclude("content", "citations"),
		maxRecordBytes: summaryRecordCeiling(s.RecordCaps()),
	}
}

// scanView excludes content and tags (D-04) and is sized from
// s.RecordCaps() via scanRecordCeiling. Safe for ScanSpine's callback,
// which reads Scope, Category, Summary (presence only), SupersededBy,
// NotAfter, NotBefore, ArchivedAt, Citations (count only, via len), and
// Owner — never Content or Tags. Citations still travel even though the
// callback only reads their count: Qdrant cannot return an array's length
// server-side, so the field itself has to cross the wire. NOT safe for any
// caller reading content or tags.
func (s *Store) scanView() readView {
	return readView{
		selector:       qdrant.NewWithPayloadExclude("content", "tags"),
		maxRecordBytes: scanRecordCeiling(s.RecordCaps()),
	}
}

// keysView is the keys-only readView (D-07) the deep-offset prefix walk
// (walkOffsetPrefix, store.go) uses to skip a large Offset cheaply: only
// created_at travels over the wire (id is never part of a payload selector —
// Qdrant always returns it), at keysRecordCeiling's fixed per-record
// ceiling. A package-level function, not a method, since it depends on no
// Store state.
func keysView() readView {
	return readView{selector: qdrant.NewWithPayloadInclude("created_at"), maxRecordBytes: keysRecordCeiling}
}

// unbudgetedView wraps sel with no byte-derived ceiling — today's
// count-only scrollAllPoints loop, kept byte-for-byte for callers this
// phase has not yet migrated (EnumerateCitations, NearDuplicates' id
// enumeration, derivePurgeEligible, previewRevertWithSteps, and
// spine_test.go's snapshotCollection — ScanSpine moved onto scanView in
// plan 05-01). Phase 5 replaces every remaining call and deletes this
// constructor; its closing check is that no unbudgetedView( call remains.
func unbudgetedView(sel *qdrant.WithPayloadSelector) readView {
	return readView{selector: sel, maxRecordBytes: 0}
}

// sweepLimit is the per-RPC record count scrollAllPoints requests for v:
// spineScrollBatch when v is unbudgeted (today's behavior, unchanged), else
// the smaller of spineScrollBatch and v's byte-derived perRPCLimit.
func sweepLimit(v readView) uint32 {
	if !v.budgeted() {
		return spineScrollBatch
	}
	lim := uint32(perRPCLimit(v.maxRecordBytes))
	return min(spineScrollBatch, lim)
}
