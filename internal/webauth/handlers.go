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
	if auth == nil {
		panic("webauth: NewHandler requires a non-nil Authenticator")
	}
	if codec == nil {
		panic("webauth: NewHandler requires a non-nil SessionCodec")
	}
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
		slog.ErrorContext(r.Context(), "marshal flow state", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sealed, err := h.codec.sealBytes(fs)
	if err != nil {
		slog.ErrorContext(r.Context(), "seal flow cookie", "err", err)
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

// Callback completes the flow: recover state+verifier from the flow cookie,
// enforce state equality (CSRF), exchange the code, verify the ID token, and
// seal the session cookie. On success it redirects to "/ui/".
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(flowCookieName)
	if err != nil {
		http.Error(w, "missing or expired login flow", http.StatusBadRequest)
		return
	}
	raw, err := h.codec.unsealBytes(c.Value)
	if err != nil {
		http.Error(w, "invalid login flow", http.StatusBadRequest)
		return
	}
	var fs flowState
	if err := json.Unmarshal(raw, &fs); err != nil {
		http.Error(w, "invalid login flow", http.StatusBadRequest)
		return
	}
	// Clear the flow cookie regardless of outcome (single use).
	h.clearCookie(w, flowCookieName)

	if r.URL.Query().Get("state") != fs.State || fs.State == "" {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	_, sub, err := h.auth.exchange(r.Context(), code, fs.Verifier)
	if err != nil {
		slog.WarnContext(r.Context(), "oauth callback exchange failed", "err", err)
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	sealed, err := h.codec.Seal(Session{
		Sub:    sub,
		Expiry: nowUTC().Add(sessionTTL),
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "seal session cookie", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.setCookie(w, sessionCookieName, sealed, sessionTTL)
	http.Redirect(w, r, "/ui/", http.StatusFound)
}

// nowUTC is a seam for tests; production uses the wall clock.
var nowUTC = func() time.Time { return time.Now().UTC() }
