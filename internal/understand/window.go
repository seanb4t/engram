// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file implements D-07's time-window bucket vocabulary and its
// day-aligned bucket-to-window conversion. Day-aligned on purpose: an
// accepted time chip's created_after must equal FacetStrip's own
// created-after date-input encoding (toRfc3339Midnight,
// ui/src/lib/components/FacetStrip.svelte lines 48-54) byte for byte, so a
// decided time-window suggestion is indistinguishable from a manual one
// once applied (D-11).

package understand

import (
	"strings"
	"time"
)

// QuestionTimeWindow is the decide.Request question name for the D-07
// time-window Choice.
const QuestionTimeWindow = "time_window"

// Buckets is the D-07 time-window vocabulary, in display order.
var Buckets = []string{"today", "past_week", "past_month", "past_year"}

// WindowOptions returns the D-07 time_window Choice's options: NoneOption
// plus one entry per Buckets value.
func WindowOptions() map[string]string {
	return map[string]string{
		NoneOption:   "The query does not say when the memories were recorded.",
		"today":      "The memories were recorded today.",
		"past_week":  "The memories were recorded within the past week.",
		"past_month": "The memories were recorded within the past month.",
		"past_year":  "The memories were recorded within the past year.",
	}
}

// Window converts bucket to its D-07/D-11 half-open window relative to
// now: midnight is the UTC calendar day of now at 00:00:00 (now is
// converted to UTC first, so a non-UTC now still resolves to the correct
// UTC day). after is midnight minus the bucket's span — 0 days for
// "today", 7 days for "past_week", one calendar month for "past_month"
// (AddDate(0, -1, 0)), one calendar year for "past_year" (AddDate(-1, 0,
// 0)) — formatted RFC3339 (renders the UTC "Z" suffix, matching
// FacetStrip's encoding exactly). before is always "" (open-ended). label
// is bucket with its underscore replaced by a space (e.g. "past week").
// ok is false for any bucket not in Buckets, including NoneOption.
func Window(bucket string, now time.Time) (after, before, label string, ok bool) {
	nowUTC := now.UTC()
	midnight := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)

	var t time.Time
	switch bucket {
	case "today":
		t = midnight
	case "past_week":
		t = midnight.AddDate(0, 0, -7)
	case "past_month":
		t = midnight.AddDate(0, -1, 0)
	case "past_year":
		t = midnight.AddDate(-1, 0, 0)
	default:
		return "", "", "", false
	}
	return t.Format(time.RFC3339), "", strings.ReplaceAll(bucket, "_", " "), true
}
