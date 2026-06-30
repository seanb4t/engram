// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// listCursor is the opaque keyset cursor for List. C is the oldest created_at
// (RFC3339) emitted so far; Seen is every record id already returned at exactly
// C. Resume drops Seen by id membership, making page boundaries independent of
// Qdrant's intra-timestamp order. See the spec §3.
type listCursor struct {
	C    string   `json:"c"`
	Seen []string `json:"seen"`
}

func encodeCursor(c listCursor) string {
	b, _ := json.Marshal(c) // listCursor has only string fields; marshal cannot fail
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(tok string) (listCursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil {
		return listCursor{}, fmt.Errorf("decode cursor: %w", err)
	}
	var c listCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return listCursor{}, fmt.Errorf("decode cursor: %w", err)
	}
	return c, nil
}
