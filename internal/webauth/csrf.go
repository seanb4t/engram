// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// csrfInfoLabel provides cryptographic domain separation from the AES-GCM
// session-seal key derived from the same ui.cookie_key raw material (D-08).
const csrfInfoLabel = "engram-csrf-v1"

// CSRFCookieName and CSRFHeaderName are the exported wire-contract constants
// the Connect CSRF interceptor (plan 02) and the Callback cookie-issuance
// path (plan 03) both agree on.
const (
	CSRFCookieName = "engram_csrf"
	CSRFHeaderName = "X-CSRF-Token"
)

// DeriveCSRFKey derives k_csrf, a 32-byte HKDF sub-key of cookieKey, using a
// distinct info label (csrfInfoLabel) so k_csrf is cryptographically
// separated from the AES-GCM session-seal key derived from the same
// ui.cookie_key (D-08 domain separation).
func DeriveCSRFKey(cookieKey []byte) ([]byte, error) {
	key, err := hkdf.Key(sha256.New, cookieKey, nil, csrfInfoLabel, 32)
	if err != nil {
		return nil, fmt.Errorf("derive csrf key: %w", err)
	}
	return key, nil
}

// CSRFSigner computes and verifies the session-bound double-submit CSRF
// token: HMAC(k_csrf, Owner). The token is bound to Owner ONLY — never
// Owner+Expiry — so it survives the Phase-18 sliding session re-seal without
// churning on every request (D-08). k_csrf is an HKDF sub-key of
// ui.cookie_key (see DeriveCSRFKey), domain-separated from the AES-GCM
// session-seal key derived from the same raw key material.
type CSRFSigner struct {
	key []byte
}

// NewCSRFSigner builds a signer from a 32-byte key (typically the output of
// DeriveCSRFKey).
func NewCSRFSigner(key []byte) (*CSRFSigner, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("csrf key must be 32 bytes, got %d", len(key))
	}
	return &CSRFSigner{key: key}, nil
}

// Token returns the double-submit CSRF token for owner: a URL-safe
// base64-encoded HMAC-SHA256 over Owner only.
func (s *CSRFSigner) Token(owner string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(owner))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Verify reports whether token is the genuine CSRF token for owner, using a
// constant-time comparison (hmac.Equal) to avoid a timing side-channel.
func (s *CSRFSigner) Verify(owner, token string) bool {
	want := s.Token(owner)
	return hmac.Equal([]byte(want), []byte(token))
}
