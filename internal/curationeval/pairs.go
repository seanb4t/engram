// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package curationeval

import (
	"fmt"
	"strings"

	"github.com/seanb4t/engram/internal/verdict"
)

// This file is CUR-03's committed labeled pair corpus (D-01, D-02): short,
// synthetic notes about fictional software projects, in pairs, each
// carrying an intended relation label from verdict.Relations(). D-02's
// independence discipline: the executor that writes syntheticPairs knows
// every intended label, so it cannot also be the blind labeler. Instead, a
// fresh, tool-less context labels every pair from blindLabelPrompt's text
// alone (id, recordA, recordB — never label, proven by
// TestBlindLabelPromptIsLabelIndependent), and only pairs where the
// author's intended label and the blind label agree are kept.
//
// Procedure as executed (plan 03-04):
//   - Task 1 authored 5 pairs (P01-P05), one per relation, proving the
//     corpus shape and the integrity rules.
//   - Task 2 extended the corpus to 80 pairs (P01-P80), 16 per relation,
//     in a fixed class-interleave order (duplicate, contradicts, updates,
//     related, unrelated, repeating).
//   - Round 1 (2026-09-23): a fresh, tool-less subagent blind-labeled all
//     80 pairs and agreed with every intended label — but the fixed
//     interleave order and several recordB cue phrases ("was wrong",
//     "contrary to the earlier claim", "as previously stated", "now
//     also", "in addition to", "no longer") let position and wording leak
//     the class, so 100% agreement did not prove the pairs were hard. The
//     round-1 reply is preserved for provenance at
//     03-BLIND-LABELS-ROUND1.md; none of its labels were used to keep or
//     drop a pair. User decision: harden the corpus and run a second
//     blind round rather than accept round 1 (no external/OSS dataset
//     considered — synthetic-only per D-01).
//   - Hardening (2026-09-23, this commit): every recordB was rewritten to
//     state its own fact plainly, with the announcement/cue phrases above
//     removed; contradicts and updates pairs were rewritten to share a
//     sentence frame and differ only in the value/state, so the relation
//     must be inferred from which fact is plausible, not from wording;
//     several related pairs were written as near-misses that read like
//     updates (P13, P27, P53, P80 — a different but compatible fact on
//     the same subject, not the same fact evolving); several duplicate
//     pairs were written as true paraphrases with low lexical overlap
//     (P40, P56, P75). The 80 pairs were then shuffled with a
//     deterministic, fixed-seed permutation (math/rand, seed 20260923,
//     accepted on the 32nd successive Shuffle call from that single
//     seeded source once the run<=2 interleave invariant held) and
//     renumbered P01-P80 in the shuffled order, so neither pair position
//     nor id encodes its class.
//   - Task 3, round 2 (2026-09-23): a second fresh, tool-less subagent
//     (dispatched by the orchestrator, no repository context, 0 tool
//     calls) blind-labeled the hardened, shuffled 80-pair corpus from
//     commit ee010289's 03-BLIND-LABEL-PROMPT.md text alone. Its reply is
//     recorded verbatim at 03-BLIND-LABELS.md. 70 of 80 pairs agreed and
//     are kept; 10 disagreed and were dropped (ids not renumbered):
//     P03, P06, P07, P25, P36, P57, P60, P61, P62, P72 — every
//     disagreement was a contradicts/updates confusion in either
//     direction (intended contradicts, blind updates: P03, P07, P61,
//     P62; intended updates, blind contradicts: P06, P25, P36, P57, P60,
//     P72), matching this plan's flagged D-06 criteria-overlap risk
//     ("duplicate" "one may be more complete" vs "updates" "a more
//     complete version" — here it was contradicts/updates that actually
//     collided instead). No related/unrelated/duplicate pair was
//     dropped, including the deliberately hard related-near-miss (P13,
//     P27, P53, P80) and low-overlap duplicate (P40, P56, P75) pairs
//     from the hardening pass above, which the blind pass confirmed
//     correctly. Kept per-relation counts: duplicate 16, contradicts 12,
//     updates 10, related 16, unrelated 16 (70 total) — all at or above
//     the 7-per-class floor, within the 50-80 total band.

// labeledPair is one CUR-03 fixture pair: two short notes about the same
// or different fictional projects, recordB written after recordA, and the
// relation label from verdict.Relations() the author intended (and, after
// Task 3, the blind pass agreed with).
type labeledPair struct {
	id, label, recordA, recordB string
}

// syntheticPairs is the committed CUR-03 corpus (D-01). See this file's
// header comment for the authoring/blind-check procedure.
var syntheticPairs = []labeledPair{
	{id: "P01", label: "updates", recordA: "The ledger service supports only USD-denominated accounts.", recordB: "The ledger service supports USD, EUR, and GBP-denominated accounts."},
	{id: "P02", label: "related", recordA: "The search API logs the latency of every query it serves.", recordB: "The search API exposes a dashboard showing the slowest queries from the past hour."},
	{id: "P04", label: "unrelated", recordA: "The analytics pipeline sends a daily report to a shared spreadsheet.", recordB: "The mobile client displays a low-battery banner under ten percent charge."},
	{id: "P05", label: "duplicate", recordA: "The billing API signs every webhook payload with an HMAC so receivers can verify authenticity.", recordB: "Every webhook payload from the billing API carries an HMAC signature that lets the receiving system confirm it is authentic."},
	{id: "P08", label: "contradicts", recordA: "The mobile client's offline mode allows the user to place new orders while offline.", recordB: "The mobile client's offline mode only allows browsing; placing a new order requires a live connection."},
	{id: "P09", label: "updates", recordA: "The billing API's rate limit is one hundred requests per minute per key.", recordB: "The billing API's rate limit is five hundred requests per minute per key."},
	{id: "P10", label: "duplicate", recordA: "The billing API rejects a charge request whose currency code is not on its supported list.", recordB: "A charge request naming an unsupported currency code gets rejected by the billing API."},
	{id: "P11", label: "duplicate", recordA: "An email sent by the notification service always includes an unsubscribe link in the footer.", recordB: "Every email the notification service sends carries an unsubscribe link at the bottom."},
	{id: "P12", label: "updates", recordA: "The notification service supports push and email channels.", recordB: "The notification service supports push, email, and SMS channels."},
	{id: "P13", label: "related", recordA: "The ledger service assigns a unique transaction id using a UUID.", recordB: "The ledger service stores a human-readable reference number alongside each transaction's UUID, for use on support tickets."},
	{id: "P14", label: "related", recordA: "The ingest worker logs each batch's record count at info level after every successful flush.", recordB: "The ingest worker exposes a metrics endpoint reporting the number of batches flushed per minute."},
	{id: "P15", label: "unrelated", recordA: "The billing API allows one hundred requests per minute for each key.", recordB: "The search API lets a customer filter results by category and price range."},
	{id: "P16", label: "related", recordA: "The mobile client supports light and dark visual themes.", recordB: "The mobile client offers a high-contrast theme for users who need greater visual accessibility."},
	{id: "P17", label: "contradicts", recordA: "The catalog service treats an item's SKU as globally unique across all warehouses.", recordB: "SKUs in the catalog service are unique only within a single warehouse, not globally."},
	{id: "P18", label: "related", recordA: "The billing API logs a structured audit event for every refund it processes.", recordB: "The billing API retains audit events for refunds for seven years to satisfy compliance requirements."},
	{id: "P19", label: "updates", recordA: "The billing API supports card payments only.", recordB: "The billing API supports card payments and ACH bank transfers."},
	{id: "P20", label: "updates", recordA: "The analytics pipeline computes daily active user counts.", recordB: "The analytics pipeline computes daily, weekly, and monthly active user counts."},
	{id: "P21", label: "unrelated", recordA: "The notification service groups low-priority pushes into a digest.", recordB: "The analytics pipeline sorts events into five-minute windows before aggregating them."},
	{id: "P22", label: "contradicts", recordA: "The notification service sends push notifications instantly, with no batching delay.", recordB: "Push notifications from the notification service are held in a batch for up to a minute before sending."},
	{id: "P23", label: "related", recordA: "The billing API supports refunding a transaction in full.", recordB: "The billing API sends a refund confirmation email to the customer once a refund is processed."},
	{id: "P24", label: "unrelated", recordA: "The catalog service rebuilds its search index nightly from the primary Postgres table.", recordB: "The mobile client resets its push notification badge count to zero once the app is opened."},
	{id: "P26", label: "unrelated", recordA: "The ledger service notifies the on-call engineer after three reconciliation failures in a row.", recordB: "The notification service lets a user silence a specific sender."},
	{id: "P27", label: "related", recordA: "The notification service lets a user mute notifications from a specific sender.", recordB: "The notification service lets a user mute an entire notification category, not tied to any single sender."},
	{id: "P28", label: "updates", recordA: "The notification service's quiet hours feature silences push notifications from ten at night to seven in the morning.", recordB: "The notification service's quiet hours feature silences push notifications from nine at night to eight in the morning."},
	{id: "P29", label: "updates", recordA: "The ingest worker's retry limit for a failed batch is three attempts.", recordB: "The ingest worker's retry limit for a failed batch is five attempts."},
	{id: "P30", label: "unrelated", recordA: "The catalog service removes an out-of-stock item from search results.", recordB: "The billing API asks for a CVV on every new card charge."},
	{id: "P31", label: "contradicts", recordA: "The ingest worker runs as a single instance with no horizontal scaling.", recordB: "The ingest worker runs as multiple instances that scale horizontally with load."},
	{id: "P32", label: "duplicate", recordA: "The mobile client retries a failed upload three times with exponential backoff before giving up.", recordB: "Failed uploads from the mobile client get three backoff-spaced retries and then stop."},
	{id: "P33", label: "unrelated", recordA: "The ingest worker exposes a metrics endpoint that reports records processed per second.", recordB: "The mobile client needs a full restart before a new language setting takes effect."},
	{id: "P34", label: "related", recordA: "The ledger service logs every failed reconciliation to a dedicated Slack channel.", recordB: "The ledger service pages the on-call engineer when three reconciliation failures happen in a row."},
	{id: "P35", label: "duplicate", recordA: "The mobile client caches the user's profile locally so the profile screen loads instantly offline.", recordB: "Because the profile is stored on the device, the mobile client's profile screen appears instantly without network access."},
	{id: "P37", label: "contradicts", recordA: "The billing API requires a CVV on every card charge.", recordB: "The billing API does not require a CVV when charging a card that was saved on a prior transaction."},
	{id: "P38", label: "updates", recordA: "The catalog service's bulk import tool accepts CSV files only.", recordB: "The catalog service's bulk import tool accepts CSV and JSON files."},
	{id: "P39", label: "unrelated", recordA: "The billing API signs each webhook payload with an HMAC for authenticity.", recordB: "The search API returns no more than fifty results for a single query."},
	{id: "P40", label: "duplicate", recordA: "The ledger service compares each day's transactions to the bank statement before markets close.", recordB: "Prior to market close, the day's recorded transactions are checked for agreement with the bank statement."},
	{id: "P41", label: "contradicts", recordA: "The mobile client supports biometric login on every device it runs on.", recordB: "The mobile client's biometric login is unavailable on devices that lack the required hardware."},
	{id: "P42", label: "duplicate", recordA: "The analytics pipeline buckets events into five-minute windows before aggregating them.", recordB: "Events are grouped into five-minute windows by the analytics pipeline prior to aggregation."},
	{id: "P43", label: "duplicate", recordA: "The search API logs every query's latency so slow queries can be found later.", recordB: "Every query the search API serves has its response time recorded, which makes finding slow queries possible afterward."},
	{id: "P44", label: "contradicts", recordA: "The search API's autocomplete suggestions come only from past searches by the same user.", recordB: "The search API's autocomplete suggestions draw from past searches across all users, not just the current one."},
	{id: "P45", label: "related", recordA: "The mobile client shows a spinner while a network request is in flight.", recordB: "The mobile client shows a toast message when a network request times out."},
	{id: "P46", label: "unrelated", recordA: "The mobile client keeps a local copy of the user's profile for offline access.", recordB: "The ledger service assigns a UUID to each transaction as its identifier."},
	{id: "P47", label: "related", recordA: "The notification service batches low-priority push notifications into a single digest.", recordB: "The notification service sends high-priority push notifications immediately, bypassing the digest."},
	{id: "P48", label: "contradicts", recordA: "The ingest worker processes batches in the order they arrive.", recordB: "The ingest worker processes batches by priority rather than arrival order."},
	{id: "P49", label: "related", recordA: "The catalog service caches category pages in a CDN for faster loading.", recordB: "The catalog service invalidates its CDN cache automatically whenever a category's item list changes."},
	{id: "P50", label: "related", recordA: "The billing API charges a flat two percent fee on every successful transaction.", recordB: "The billing API charges a fixed thirty-cent fee on every transaction, separate from its percentage-based fee."},
	{id: "P51", label: "contradicts", recordA: "A negative balance is impossible in the ledger service; every account is clamped to zero.", recordB: "Ledger service accounts with an approved overdraft can carry a negative balance."},
	{id: "P52", label: "unrelated", recordA: "The notification service puts an unsubscribe link in every email footer.", recordB: "The catalog service groups similar items into a listing with variants."},
	{id: "P53", label: "related", recordA: "The search API supports filtering results by category.", recordB: "The search API supports filtering results by price range."},
	{id: "P54", label: "duplicate", recordA: "The mobile client shows a low-battery banner once the device drops below ten percent charge.", recordB: "Once the device's charge falls under ten percent, a low-battery banner appears in the mobile client."},
	{id: "P55", label: "contradicts", recordA: "The search API ranks results purely by text relevance, with no popularity signal.", recordB: "The search API's ranking blends a popularity signal in with text relevance."},
	{id: "P56", label: "duplicate", recordA: "A refund in the ledger service always references the original transaction's id.", recordB: "Every refund the ledger service issues carries a pointer back to the id of the transaction it corrects."},
	{id: "P58", label: "duplicate", recordA: "The ingest worker drops an entire batch when any single record inside it fails validation.", recordB: "If one record in a batch fails validation, the whole batch is discarded by the ingest worker."},
	{id: "P59", label: "duplicate", recordA: "The notification service retries a failed push delivery up to three times before giving up.", recordB: "A push notification that fails to deliver gets three retry attempts from the notification service before it stops trying."},
	{id: "P63", label: "updates", recordA: "The mobile client's dark mode setting is a manual toggle in settings.", recordB: "The mobile client's dark mode setting follows the device's system theme, with a manual override still available in settings."},
	{id: "P64", label: "contradicts", recordA: "Deleting a category in the catalog service also deletes every item inside it.", recordB: "Deleting a category in the catalog service moves its items into an uncategorized bucket rather than deleting them."},
	{id: "P65", label: "duplicate", recordA: "The catalog service hides an item from search results once its stock count reaches zero.", recordB: "An item drops out of catalog search results as soon as its stock count hits zero."},
	{id: "P66", label: "unrelated", recordA: "The ledger service checks its transactions against the bank statement every night.", recordB: "The mobile client lets a user toggle dark mode manually from the settings screen."},
	{id: "P67", label: "related", recordA: "The catalog service's search ranks in-stock items above out-of-stock ones.", recordB: "The catalog service's search ranking factors in how many customer reviews an item has, independent of stock status."},
	{id: "P68", label: "unrelated", recordA: "The ingest worker retries a failed batch up to three times before giving up.", recordB: "The catalog service limits a product description to two thousand characters."},
	{id: "P69", label: "duplicate", recordA: "The search API caps every query response at fifty results regardless of how many matches exist.", recordB: "No matter how many matches exist, the search API never returns more than fifty results for a single query."},
	{id: "P70", label: "updates", recordA: "The catalog service indexes product titles for search.", recordB: "The catalog service indexes product titles and product descriptions for search."},
	{id: "P71", label: "unrelated", recordA: "The search API records the latency of every query it serves.", recordB: "The ingest worker keeps a rejected record in its dead-letter queue for thirty days."},
	{id: "P73", label: "unrelated", recordA: "The search API corrects a misspelled word in a query before it is matched against the index.", recordB: "The ledger service never allows an account's balance to fall below zero."},
	{id: "P74", label: "contradicts", recordA: "Users cannot opt out of transactional emails from the notification service.", recordB: "Users can opt out of transactional emails from the notification service through a preference center."},
	{id: "P75", label: "duplicate", recordA: "Records that fail validation sit inside the ingest worker's dead-letter queue for thirty days before removal.", recordB: "The ingest worker's dead-letter queue keeps a rejected record for thirty days before purging it."},
	{id: "P76", label: "unrelated", recordA: "The catalog service's bulk import tool reads CSV and JSON files.", recordB: "The billing API takes a flat two percent cut of every transaction."},
	{id: "P77", label: "unrelated", recordA: "The analytics pipeline reports active user counts by day, week, and month.", recordB: "The notification service can deliver a message by push, email, or SMS."},
	{id: "P78", label: "duplicate", recordA: "A product description in the catalog service is limited to two thousand characters.", recordB: "The catalog service will not accept a product description longer than two thousand characters."},
	{id: "P79", label: "related", recordA: "The catalog service groups similar items into a single product listing with variants.", recordB: "A customer can filter a listing's variants by size and color."},
	{id: "P80", label: "related", recordA: "A payload that fails to parse as valid JSON gets rejected by the ingest worker with a parse error.", recordB: "A record that fails a required-field check gets rejected by the ingest worker with a validation error."},
}

// blindLabelMarker delimits the orchestrator's own provenance prose (above
// the line) from the text a blind labeler must receive verbatim (below
// it) — the same convention 01-BLIND-QUERY-PROMPT.md established.
const blindLabelMarker = "---8<--- SEND EVERYTHING BELOW THIS LINE VERBATIM AS THE ENTIRE PROMPT ---8<---"

// blindLabelPrompt renders pairs into the D-02 blind labeler's entire
// input: an instruction paragraph, every relation's name and criteria
// (verdict.RelationCriteria(), verbatim, in verdict.Relations() order),
// then one heading and both record texts per pair. It reads only id,
// recordA and recordB from each pair — never label — so its output cannot
// vary with any pair's intended label (proven by
// TestBlindLabelPromptIsLabelIndependent).
func blindLabelPrompt(pairs []labeledPair) string {
	var b strings.Builder

	b.WriteString("Below are pairs of short notes from software teams' shared project memory. You have not seen where they came from. In every pair, record_b was written after record_a. For each pair, choose exactly ONE relation describing how record_b relates to record_a, using only the five relation names below. Do not use any tools and do not read any files. Answer from this message alone. Reply with exactly one line per pair in the form `- Pnn: <relation>`, in the same order, using only the five names, and nothing else.\n\n")

	criteria := verdict.RelationCriteria()
	for _, name := range verdict.Relations() {
		fmt.Fprintf(&b, "- %s: %s\n", name, criteria[name])
	}
	b.WriteString("\n")

	for _, p := range pairs {
		fmt.Fprintf(&b, "### %s\n", p.id)
		fmt.Fprintf(&b, "record_a: %s\n", p.recordA)
		fmt.Fprintf(&b, "record_b: %s\n\n", p.recordB)
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}
