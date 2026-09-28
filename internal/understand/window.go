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
// "today", 7 days for "past_week", one calendar month for "past_month",
// one calendar year for "past_year" — formatted RFC3339 (renders the UTC
// "Z" suffix, matching FacetStrip's encoding exactly). before is always ""
// (open-ended). label is bucket with its underscore replaced by a space
// (e.g. "past week"). ok is false for any bucket not in Buckets, including
// NoneOption.
//
// past_month/past_year subtract calendar months/years by clamping the
// day-of-month to the target month's last day, rather than via
// time.Time.AddDate, which normalizes an overflowing day by rolling
// forward into the following month instead of landing in the target one
// (e.g. 2026-05-31 minus one calendar month via AddDate(0,-1,0) yields
// 2026-05-01, not late April, because April has only 30 days). Clamping
// keeps the suggested window close to its stated span on every date,
// including the ~15-16 days/year whose day-of-month exceeds the previous
// month's length and the Feb 29 leap-day boundary for past_year.
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
		t = subtractCalendarClamped(midnight, 0, 1)
	case "past_year":
		t = subtractCalendarClamped(midnight, 1, 0)
	default:
		return "", "", "", false
	}
	return t.Format(time.RFC3339), "", strings.ReplaceAll(bucket, "_", " "), true
}

// subtractCalendarClamped subtracts years and months from midnight (a UTC
// day-aligned midnight), clamping the result's day-of-month to the target
// month's last day if the source day-of-month overflows it — never rolling
// forward into the following month the way time.Time.AddDate does.
func subtractCalendarClamped(midnight time.Time, years, months int) time.Time {
	y, m, d := midnight.Date()
	y -= years

	// Subtract months, borrowing a year on underflow. time.Month is 1-12;
	// treat it as a 0-based index for the arithmetic.
	mi := int(m) - 1 - months
	for mi < 0 {
		mi += 12
		y--
	}
	targetMonth := time.Month(mi + 1)

	if last := lastDayOfMonth(y, targetMonth); d > last {
		d = last
	}
	return time.Date(y, targetMonth, d, 0, 0, 0, 0, time.UTC)
}

// lastDayOfMonth returns the number of days in month of year, via the
// standard Go idiom of asking for "day 0" of the following month.
func lastDayOfMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
