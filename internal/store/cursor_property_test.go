// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"testing"

	"pgregory.net/rapid"
)

func TestPropertyCursorRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		c := listCursor{
			C:    rapid.String().Draw(t, "c"),
			Seen: rapid.SliceOf(rapid.String()).Draw(t, "seen"),
		}
		tok := encodeCursor(c)
		decoded, err := decodeCursor(tok)
		if err != nil {
			t.Fatalf("decodeCursor(encodeCursor(%#v)): %v", c, err)
		}
		if again := encodeCursor(decoded); again != tok {
			t.Fatalf("token %q re-encoded as %q", tok, again)
		}
	})
}
