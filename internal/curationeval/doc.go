// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package curationeval measures the advisory relation verdicts
// internal/verdict produces (spine-review consolidate's verdict pass)
// against a committed, synthetic, blind-checked corpus of labeled note
// pairs (D-01, D-02) — see pairs.go's file comment for the corpus and its
// authoring/blind-labeling procedure. The live measurement that sends
// every pair to a real decider and scores accuracy/Brier is gated and
// added in plan 03-07; this file stays short until then.
package curationeval
