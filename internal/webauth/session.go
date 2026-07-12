// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package webauth implements engram's web-UI auth lane: an OIDC confidential
// client login flow, a stateless AES-GCM-sealed session cookie, and a Connect
// resolver that turns the cookie into the verified Subject the EngramService
// handlers authorize on. See docs/superpowers/specs/2026-06-09-engram-web-ui-design.md.
package webauth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// sessionPayloadVersion is the current session payload version.
// SessionCodec.Seal auto-injects this into every minted session (below) so
// every mint site is version-consistent by default with no per-site stamp.
// Resolver.Resolve rejects any session whose version does not match, which
// invalidates every pre-upgrade (legacy / version-0) cookie during the
// owner-encoding rollout (T-17-14): no bare-owner cookie can be forwarded
// into the new namespaced owner space. The field is additive and
// forward-compatible with the Phase-18 sliding re-seal (D-08) — a re-seal
// that goes through Seal stamps the current version by construction.
const sessionPayloadVersion = 1

// Session is the decrypted payload of the engram session cookie. Owner is the
// authz key (the configured owner-claim value, default email); Expiry bounds the
// session lifetime. V is the payload version (see sessionPayloadVersion) — Seal
// always overwrites it, so callers never set it themselves; a cookie sealed
// before this field existed decodes with V at its JSON zero value (0), which
// Resolver.Resolve treats as legacy and rejects. The payload is otherwise
// deliberately minimal: the future write phase will reintroduce token handling
// server-side rather than shipping credentials to the browser.
type Session struct {
	Owner  string    `json:"owner"`
	Expiry time.Time `json:"exp"`
	V      int       `json:"v"`
}

// SessionCodec seals/unseals Session values with AES-256-GCM. The key MUST be
// exactly 32 bytes (AES-256). Output is URL-safe base64 (cookie-value safe).
type SessionCodec struct {
	aead cipher.AEAD
}

// NewSessionCodec builds a codec from a 32-byte key.
func NewSessionCodec(key []byte) (*SessionCodec, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("session key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return &SessionCodec{aead: aead}, nil
}

// Seal serializes and encrypts s. A fresh random nonce is prepended to the
// ciphertext; the whole blob is base64url-encoded. s.V is always overwritten
// with the current sessionPayloadVersion before marshalling — every mint site
// is version-consistent by default, with no per-site stamp required (round-3
// LOW-9).
func (c *SessionCodec) Seal(s Session) (string, error) {
	s.V = sessionPayloadVersion
	plain, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("marshal session: %w", err)
	}
	return c.sealBytes(plain)
}

// Unseal decodes, decrypts, and deserializes a sealed cookie value. Any
// tamper/wrong-key/short-input condition returns an error (fail closed).
// Callers are responsible for checking Session.Expiry against the current time.
func (c *SessionCodec) Unseal(v string) (Session, error) {
	plain, err := c.unsealBytes(v)
	if err != nil {
		return Session{}, err
	}
	var s Session
	if err := json.Unmarshal(plain, &s); err != nil {
		return Session{}, fmt.Errorf("unmarshal session: %w", err)
	}
	return s, nil
}

// sealBytes/unsealBytes seal arbitrary bytes with the same AEAD, for the
// short-lived OAuth flow cookie (state+verifier) that is not a Session.
func (c *SessionCodec) sealBytes(plain []byte) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(c.aead.Seal(nonce, nonce, plain, nil)), nil
}

func (c *SessionCodec) unsealBytes(v string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	ns := c.aead.NonceSize()
	if len(raw) < ns {
		return nil, fmt.Errorf("sealed value too short")
	}
	return c.aead.Open(nil, raw[:ns], raw[ns:], nil)
}
