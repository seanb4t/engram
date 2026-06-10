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

// Session is the decrypted payload of the engram session cookie. Sub is the
// authz key (OIDC subject); Access/Refresh ride along for the future write
// phase; Expiry bounds the session lifetime.
type Session struct {
	Sub     string    `json:"sub"`
	Access  string    `json:"at"`
	Refresh string    `json:"rt"`
	Expiry  time.Time `json:"exp"`
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
// ciphertext; the whole blob is base64url-encoded.
func (c *SessionCodec) Seal(s Session) (string, error) {
	plain, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("marshal session: %w", err)
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	sealed := c.aead.Seal(nonce, nonce, plain, nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Unseal decodes, decrypts, and deserializes a sealed cookie value. Any
// tamper/wrong-key/short-input condition returns an error (fail closed).
func (c *SessionCodec) Unseal(v string) (Session, error) {
	raw, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return Session{}, fmt.Errorf("decode: %w", err)
	}
	ns := c.aead.NonceSize()
	if len(raw) < ns {
		return Session{}, fmt.Errorf("sealed value too short")
	}
	nonce, ct := raw[:ns], raw[ns:]
	plain, err := c.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return Session{}, fmt.Errorf("decrypt: %w", err)
	}
	var s Session
	if err := json.Unmarshal(plain, &s); err != nil {
		return Session{}, fmt.Errorf("unmarshal session: %w", err)
	}
	return s, nil
}
