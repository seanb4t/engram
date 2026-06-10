// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/oauth2"
)

const (
	// sessionCookieName holds the sealed Session after login.
	sessionCookieName = "engram_session"
	// flowCookieName holds the sealed in-flight {state, verifier} between
	// /auth/login and /auth/callback.
	flowCookieName = "engram_oauth_flow"
	flowTTL        = 10 * time.Minute
	sessionTTL     = 12 * time.Hour
)

// Handler serves the OIDC login endpoints and seals/clears the session cookie.
type Handler struct {
	auth   *Authenticator
	codec  *SessionCodec
	secure bool // Secure attribute on Set-Cookie (false only for plaintext local dev)
}

// NewHandler builds a Handler over an Authenticator and SessionCodec; secure
// sets the Secure attribute on issued cookies (false only for plaintext dev).
func NewHandler(auth *Authenticator, codec *SessionCodec, secure bool) *Handler {
	return &Handler{auth: auth, codec: codec, secure: secure}
}

// flowState is sealed into the flow cookie so state+verifier survive the
// round-trip to the IdP without server-side storage (stateless, per D4).
type flowState struct {
	State    string `json:"s"`
	Verifier string `json:"v"`
}

// Login begins the auth-code+PKCE flow: generate a verifier + state, seal them
// into a short-lived flow cookie, and redirect to the issuer.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	verifier := oauth2.GenerateVerifier()
	state := oauth2.GenerateVerifier() // reuse the CSPRNG helper for an opaque state token

	fs, err := json.Marshal(flowState{State: state, Verifier: verifier})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sealed, err := h.codec.sealBytes(fs)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.setCookie(w, flowCookieName, sealed, flowTTL)

	url := h.auth.oauthConfig().AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(verifier))
	http.Redirect(w, r, url, http.StatusFound)
}

// setCookie writes an httpOnly, SameSite=Lax cookie scoped to "/".
func (h *Handler) setCookie(w http.ResponseWriter, name, value string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  nowUTC().Add(ttl),
		MaxAge:   int(ttl.Seconds()),
	})
}

// clearCookie expires a cookie immediately.
func (h *Handler) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: "", Path: "/", HttpOnly: true, Secure: h.secure,
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

// Logout clears the session cookie. Coarse (no IdP back-channel logout); the
// sealed cookie simply stops being presented. 204 so the SPA can fire-and-forget.
func (h *Handler) Logout(w http.ResponseWriter, _ *http.Request) {
	h.clearCookie(w, sessionCookieName)
	w.WriteHeader(http.StatusNoContent)
}

// nowUTC is a seam for tests; production uses the wall clock.
var nowUTC = func() time.Time { return time.Now().UTC() }

var _ = slog.Default // retained: callback/logout log via slog
