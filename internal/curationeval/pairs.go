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
//   - Task 2 extended the corpus to 80 pairs (P01-P80), 16 per relation.
//   - Task 3 records the blind-labeling outcome here: pairs blind-labeled,
//     pairs kept (per-relation counts), dropped pair ids, the labeling
//     date, and the prompt's commit SHA. Until Task 3 runs, syntheticPairs
//     holds every authored pair, not yet filtered by agreement.

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
	{id: "P01", label: "duplicate", recordA: "The mobile client retries a failed upload three times with exponential backoff before giving up.", recordB: "Uploads from the mobile client get retried three times with backoff, and then the client gives up."},
	{id: "P02", label: "contradicts", recordA: "The billing API's request timeout for calls to the payment processor is five seconds.", recordB: "The billing API's request timeout for calls to the payment processor is thirty seconds, not five; the five-second figure was wrong."},
	{id: "P03", label: "updates", recordA: "The ledger service's payout table has no currency column, so every amount is assumed to be in USD.", recordB: "The ledger service's payout table now has a currency column, added in this quarter's schema migration."},
	{id: "P04", label: "related", recordA: "The ingest worker logs each batch's record count at info level after every successful flush.", recordB: "The ingest worker also emits a counter metric for the number of batches flushed, alongside the log line."},
	{id: "P05", label: "unrelated", recordA: "The catalog service rebuilds its search index nightly from the primary Postgres table.", recordB: "The mobile client resets its push notification badge count to zero once the app is foregrounded."},
	{id: "P06", label: "duplicate", recordA: "The ledger service reconciles each day's transactions against the bank statement before markets close.", recordB: "Every day, before the market closes, the ledger service checks that its transactions match the bank statement."},
	{id: "P07", label: "contradicts", recordA: "The ledger service's daily reconciliation job runs at midnight UTC.", recordB: "The ledger service's daily reconciliation job runs at six in the morning UTC, not midnight; midnight was never correct."},
	{id: "P08", label: "updates", recordA: "The ledger service supports only USD-denominated accounts.", recordB: "The ledger service now also supports EUR and GBP accounts, added alongside the existing USD support."},
	{id: "P09", label: "related", recordA: "The ledger service logs every failed reconciliation to a dedicated Slack channel.", recordB: "The ledger service also pages the on-call engineer when three reconciliation failures happen in a row."},
	{id: "P10", label: "unrelated", recordA: "The ledger service reconciles transactions against the bank statement every night.", recordB: "The mobile client's dark mode setting can be toggled manually in the settings screen."},
	{id: "P11", label: "duplicate", recordA: "The ingest worker drops a batch entirely if any single record in it fails validation.", recordB: "If one record in a batch fails validation, the ingest worker discards the whole batch."},
	{id: "P12", label: "contradicts", recordA: "The ingest worker processes batches in the order they arrive.", recordB: "The ingest worker actually processes batches by priority, not arrival order; the arrival-order claim is wrong."},
	{id: "P13", label: "updates", recordA: "The ingest worker validates a record's schema but does not check for duplicate ids.", recordB: "The ingest worker now also checks incoming records for duplicate ids, on top of the existing schema validation."},
	{id: "P14", label: "related", recordA: "A malformed JSON payload causes the ingest worker to reject a record with a parse error.", recordB: "A record that fails a required-field check is rejected by the ingest worker with a validation error."},
	{id: "P15", label: "unrelated", recordA: "The ingest worker retries a failed batch up to three times before giving up.", recordB: "The catalog service caps a product description at two thousand characters."},
	{id: "P16", label: "duplicate", recordA: "The billing API rejects a charge request when the currency code is not in its supported list.", recordB: "A charge request with an unsupported currency code is rejected by the billing API."},
	{id: "P17", label: "contradicts", recordA: "The billing API's maximum charge amount per request is one thousand dollars.", recordB: "The billing API's maximum charge amount per request is ten thousand dollars, not one thousand; that figure was incorrect."},
	{id: "P18", label: "updates", recordA: "The billing API supports card payments only.", recordB: "The billing API now also supports ACH bank transfers, in addition to the card payments it already supported."},
	{id: "P19", label: "related", recordA: "The billing API charges a flat two percent fee on every successful transaction.", recordB: "The billing API also charges a fixed thirty-cent fee per transaction, on top of the percentage fee."},
	{id: "P20", label: "unrelated", recordA: "The billing API signs webhook payloads with an HMAC for authenticity.", recordB: "The search API returns at most fifty results per query."},
	{id: "P21", label: "duplicate", recordA: "The mobile client caches the user's profile locally so the profile screen loads instantly offline.", recordB: "Because the profile is cached locally, the mobile client's profile screen renders instantly without a network connection."},
	{id: "P22", label: "contradicts", recordA: "The mobile client supports biometric login on every device it runs on.", recordB: "The mobile client does not support biometric login on every device; some older devices lack the hardware and it is unsupported there."},
	{id: "P23", label: "updates", recordA: "The mobile client's dark mode setting is a manual toggle in settings.", recordB: "The mobile client's dark mode setting now also follows the system theme automatically, in addition to the manual toggle."},
	{id: "P24", label: "related", recordA: "The mobile client shows a spinner while a network request is in flight.", recordB: "The mobile client shows a toast message when a network request ultimately times out."},
	{id: "P25", label: "unrelated", recordA: "The notification service batches low-priority pushes into a digest.", recordB: "The analytics pipeline buckets events into five-minute windows before aggregating them."},
	{id: "P26", label: "duplicate", recordA: "The catalog service hides an item from search results once its stock count reaches zero.", recordB: "Once an item's stock reaches zero, the catalog service stops showing it in search results."},
	{id: "P27", label: "contradicts", recordA: "The catalog service treats an item's SKU as globally unique across all warehouses.", recordB: "SKUs in the catalog service are not globally unique; they are only unique per warehouse, contrary to the earlier claim."},
	{id: "P28", label: "updates", recordA: "The catalog service indexes product titles for search.", recordB: "The catalog service now also indexes product descriptions for search, beyond just titles."},
	{id: "P29", label: "related", recordA: "The catalog service's search ranks in-stock items above out-of-stock ones.", recordB: "The catalog service's search also boosts items with more customer reviews, independent of stock status."},
	{id: "P30", label: "unrelated", recordA: "The mobile client caches the user's profile locally for offline access.", recordB: "The ledger service assigns each transaction a UUID as its unique identifier."},
	{id: "P31", label: "duplicate", recordA: "The search API caps every query response at fifty results regardless of how many matches exist.", recordB: "No matter how many matches a query has, the search API never returns more than fifty results."},
	{id: "P32", label: "contradicts", recordA: "The search API ranks results purely by text relevance, with no popularity signal.", recordB: "The search API does factor in a popularity signal when ranking results; the text-relevance-only description was wrong."},
	{id: "P33", label: "updates", recordA: "The search API returns results ranked by relevance alone.", recordB: "The search API now blends a recency boost into its relevance ranking, an addition to the plain relevance scoring."},
	{id: "P34", label: "related", recordA: "The search API supports filtering results by category.", recordB: "The search API also supports filtering results by price range, alongside the category filter."},
	{id: "P35", label: "unrelated", recordA: "The catalog service hides an out-of-stock item from search results.", recordB: "The billing API requires a CVV on every new card charge."},
	{id: "P36", label: "duplicate", recordA: "The notification service retries a failed push delivery up to three times before giving up.", recordB: "A push notification that fails to deliver is retried three times by the notification service before it gives up."},
	{id: "P37", label: "contradicts", recordA: "The notification service sends push notifications instantly, with no batching delay.", recordB: "Push notifications from the notification service are batched with up to a one-minute delay, not sent instantly as previously stated."},
	{id: "P38", label: "updates", recordA: "The notification service supports push and email channels.", recordB: "The notification service now also supports SMS as a third delivery channel, alongside push and email."},
	{id: "P39", label: "related", recordA: "The notification service batches low-priority push notifications into a single digest.", recordB: "The notification service sends high-priority push notifications immediately, bypassing the digest batching."},
	{id: "P40", label: "unrelated", recordA: "The search API logs the latency of every query it serves.", recordB: "The ingest worker's dead-letter queue holds a rejected record for thirty days."},
	{id: "P41", label: "duplicate", recordA: "The analytics pipeline buckets events into five-minute windows before aggregating them.", recordB: "Events are grouped into five-minute windows by the analytics pipeline prior to aggregation."},
	{id: "P42", label: "contradicts", recordA: "The analytics pipeline stores raw events for exactly ninety days before deleting them.", recordB: "The analytics pipeline actually keeps raw events for one year, not ninety days; the ninety-day figure was a mistake."},
	{id: "P43", label: "updates", recordA: "The analytics pipeline computes daily active user counts.", recordB: "The analytics pipeline now also computes weekly and monthly active user counts, in addition to the daily figure."},
	{id: "P44", label: "related", recordA: "The analytics pipeline exports daily reports to a shared spreadsheet.", recordB: "The analytics pipeline also posts a daily summary to a chat channel, separate from the spreadsheet export."},
	{id: "P45", label: "unrelated", recordA: "The analytics pipeline exports a daily report to a shared spreadsheet.", recordB: "The mobile client shows a low-battery banner under ten percent charge."},
	{id: "P46", label: "duplicate", recordA: "A refund in the ledger service always references the original transaction's id.", recordB: "Every refund the ledger service processes carries a reference back to the original transaction's id."},
	{id: "P47", label: "contradicts", recordA: "A negative balance is impossible in the ledger service; every account is clamped to zero.", recordB: "Accounts in the ledger service can go negative under an overdraft; the claim that balances are clamped to zero was wrong."},
	{id: "P48", label: "updates", recordA: "Two-factor authentication is optional for ledger service accounts.", recordB: "Two-factor authentication became mandatory for every ledger service account this quarter, no longer optional."},
	{id: "P49", label: "related", recordA: "The ledger service assigns a unique transaction id using a UUID.", recordB: "The ledger service stores a human-readable reference number alongside each transaction's UUID, for support tickets."},
	{id: "P50", label: "unrelated", recordA: "The notification service includes an unsubscribe link in every email footer.", recordB: "The catalog service groups similar items into a listing with variants."},
	{id: "P51", label: "duplicate", recordA: "The ingest worker's dead-letter queue keeps a rejected record for thirty days before purging it.", recordB: "Rejected records sit in the ingest worker's dead-letter queue for thirty days before being purged."},
	{id: "P52", label: "contradicts", recordA: "The ingest worker runs as a single instance with no horizontal scaling.", recordB: "The ingest worker does scale horizontally across multiple instances; it was never limited to a single instance."},
	{id: "P53", label: "updates", recordA: "The ingest worker's retry limit for a failed batch is three attempts.", recordB: "The ingest worker's retry limit for a failed batch was raised to five attempts after three proved too low."},
	{id: "P54", label: "related", recordA: "The ingest worker exposes a metrics endpoint reporting records processed per second.", recordB: "The ingest worker's metrics endpoint also reports the current size of its dead-letter queue."},
	{id: "P55", label: "unrelated", recordA: "The billing API's rate limit is one hundred requests per minute per key.", recordB: "The search API supports filtering results by category and price range."},
	{id: "P56", label: "duplicate", recordA: "The billing API signs every webhook payload with an HMAC so receivers can verify authenticity.", recordB: "Webhook payloads from the billing API carry an HMAC signature that lets receivers verify they are authentic."},
	{id: "P57", label: "contradicts", recordA: "The billing API requires a CVV on every card charge.", recordB: "The billing API does not require a CVV for a saved card on a repeat charge; the every-charge claim was incorrect."},
	{id: "P58", label: "updates", recordA: "The billing API's rate limit is one hundred requests per minute per key.", recordB: "The billing API's rate limit was raised to five hundred requests per minute per key after customer feedback."},
	{id: "P59", label: "related", recordA: "The billing API supports refunding a transaction in full.", recordB: "The billing API also supports partial refunds, up to the original transaction amount."},
	{id: "P60", label: "unrelated", recordA: "The ledger service pages the on-call engineer after three reconciliation failures.", recordB: "The notification service lets a user mute a specific sender."},
	{id: "P61", label: "duplicate", recordA: "The mobile client shows a low-battery banner once the device drops below ten percent charge.", recordB: "Once the device's battery falls under ten percent, the mobile client displays a low-battery banner."},
	{id: "P62", label: "contradicts", recordA: "The mobile client's offline mode allows the user to place new orders while offline.", recordB: "The mobile client's offline mode does not allow placing new orders; only browsing is available offline, contrary to the earlier claim."},
	{id: "P63", label: "updates", recordA: "The mobile client requires a full app restart to apply a new language setting.", recordB: "The mobile client now applies a new language setting without a restart, a change from the earlier restart requirement."},
	{id: "P64", label: "related", recordA: "The mobile client supports light and dark visual themes.", recordB: "The mobile client also supports a high-contrast accessibility theme, separate from light and dark."},
	{id: "P65", label: "unrelated", recordA: "The ingest worker exposes a metrics endpoint reporting records processed per second.", recordB: "The mobile client requires a full restart to apply a new language setting."},
	{id: "P66", label: "duplicate", recordA: "A product description in the catalog service is limited to two thousand characters.", recordB: "The catalog service caps a product's description at two thousand characters."},
	{id: "P67", label: "contradicts", recordA: "Deleting a category in the catalog service also deletes every item inside it.", recordB: "Deleting a category in the catalog service does not delete its items; they are moved to an uncategorized bucket instead."},
	{id: "P68", label: "updates", recordA: "The catalog service's bulk import tool accepts CSV files only.", recordB: "The catalog service's bulk import tool now also accepts JSON files, in addition to the CSV support it already had."},
	{id: "P69", label: "related", recordA: "The catalog service groups similar items into a single product listing with variants.", recordB: "The catalog service lets a customer filter variants by size and color within a listing."},
	{id: "P70", label: "unrelated", recordA: "The catalog service's bulk import tool accepts CSV and JSON files.", recordB: "The billing API charges a flat two percent fee on every transaction."},
	{id: "P71", label: "duplicate", recordA: "The search API logs every query's latency so slow queries can be found later.", recordB: "Every query the search API serves has its latency logged, making slow queries easy to find later."},
	{id: "P72", label: "contradicts", recordA: "The search API's autocomplete suggestions come only from past searches by the same user.", recordB: "Autocomplete suggestions actually come from all users' past searches, not just the same user; the earlier scoping claim was wrong."},
	{id: "P73", label: "updates", recordA: "The search API has no support for typo correction in queries.", recordB: "The search API added typo correction for queries this release, closing the gap the earlier note described."},
	{id: "P74", label: "related", recordA: "A query timeout in the search API returns a partial result set rather than an error.", recordB: "A query that exceeds the search API's result-size limit is truncated rather than rejected outright."},
	{id: "P75", label: "unrelated", recordA: "The search API added typo correction for queries this release.", recordB: "The ledger service clamps every account's balance to zero, never allowing it to go negative."},
	{id: "P76", label: "duplicate", recordA: "An email sent by the notification service always includes an unsubscribe link in the footer.", recordB: "The notification service puts an unsubscribe link in the footer of every email it sends."},
	{id: "P77", label: "contradicts", recordA: "Users cannot opt out of transactional emails from the notification service.", recordB: "Users can opt out of transactional emails through a preference center; the cannot-opt-out claim was incorrect."},
	{id: "P78", label: "updates", recordA: "The notification service's quiet hours feature silences push notifications from ten at night to seven in the morning.", recordB: "The notification service's quiet hours window was extended to run from nine at night to eight in the morning, replacing the earlier window."},
	{id: "P79", label: "related", recordA: "The notification service lets a user mute notifications from a specific sender.", recordB: "The notification service also lets a user mute an entire notification category, not just one sender."},
	{id: "P80", label: "unrelated", recordA: "The analytics pipeline computes daily, weekly and monthly active user counts.", recordB: "The notification service supports push, email and SMS delivery channels."},
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
