// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package migrate

import "testing"

// NewStep's signature is pinned by this compile-time function-value
// assignment: a future phase that threads a decoder parameter through the
// constructor — the alternative D-11 rejected — stops compiling here rather
// than silently landing.
var _ func(Version, Version, []string, Reversibility, ApplyFunc) Step = NewStep

// decodingStep is a test-only type demonstrating the extension D-11
// describes is purely additive: it embeds a plain Step (constructed the
// normal way, through NewStep) and adds a DecodeAt method, which is
// entirely sufficient to satisfy Decoder — no change to Step or NewStep is
// required.
type decodingStep struct {
	Step
	decoded map[string]any
}

func (d decodingStep) DecodeAt(_ Version, _ map[string]any) (map[string]any, error) {
	return d.decoded, nil
}

// TestDecoderDoorIsOpenAndUnclaimed proves the per-version decoder door D-11
// leaves open: unclaimed today (a plain Step does not satisfy Decoder) and
// reachable without any change to NewStep or Step (embedding a plain Step
// and adding one method is enough).
func TestDecoderDoorIsOpenAndUnclaimed(t *testing.T) {
	t.Run("Step does not satisfy Decoder today", func(t *testing.T) {
		s := NewStep(0, 1, nil, Irreversible("x"), func(p map[string]any) (map[string]any, error) { return p, nil })
		if _, ok := any(s).(Decoder); ok {
			t.Fatal("a plain migrate.Step satisfies Decoder — nothing in this phase declares a decoding need, so this must stay unclaimed until a future phase actually adds DecodeAt")
		}
	})

	t.Run("embedding Step and adding DecodeAt reaches the interface", func(t *testing.T) {
		s := NewStep(0, 1, nil, Irreversible("x"), func(p map[string]any) (map[string]any, error) { return p, nil })
		want := map[string]any{"k": "v"}
		ds := decodingStep{Step: s, decoded: want}

		d, ok := any(ds).(Decoder)
		if !ok {
			t.Fatal("decodingStep (Step embedded + DecodeAt added) does not satisfy Decoder — the extension is supposed to be purely additive, reachable by type assertion at the point of use")
		}
		got, err := d.DecodeAt(0, nil)
		if err != nil {
			t.Fatalf("DecodeAt through the Decoder interface returned an error: %v", err)
		}
		if len(got) != len(want) || got["k"] != want["k"] {
			t.Fatalf("DecodeAt through the Decoder interface returned %v, want %v", got, want)
		}
		// decodingStep's embedded Step still behaves like any other Step —
		// the extension changed nothing about the base type.
		if ds.From() != 0 || ds.To() != 1 {
			t.Fatalf("decodingStep's embedded Step lost its From/To — got From=%d To=%d, want From=0 To=1", ds.From(), ds.To())
		}
	})
}
