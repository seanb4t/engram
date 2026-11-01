// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"reflect"
	"testing"
)

func FuzzDecodeCursor(f *testing.F) {
	f.Add("")
	f.Add("e30")
	f.Add("not base64!")
	f.Add(encodeCursor(listCursor{C: "2026-10-31T12:00:00Z", Seen: []string{"a", "b"}}))
	f.Fuzz(func(t *testing.T, tok string) {
		c, err := decodeCursor(tok)
		if err != nil {
			return
		}
		again, err := decodeCursor(encodeCursor(c))
		if err != nil {
			t.Fatalf("re-decode of %q failed: %v", tok, err)
		}
		if !reflect.DeepEqual(normalizeSeen(c), normalizeSeen(again)) {
			t.Fatalf("round trip of %q changed the cursor: %#v != %#v", tok, c, again)
		}
	})
}

// normalizeSeen treats a nil and an empty Seen as equal, because JSON writes
// both as null or [] and reads them back as the other.
func normalizeSeen(c listCursor) listCursor {
	if len(c.Seen) == 0 {
		c.Seen = nil
	}
	return c
}
