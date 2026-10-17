// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// oauthRequestTimeout bounds each single request `engram login` and
// `engram logout` make. Waiting for the user's approval is bounded by the
// device code's own expiry instead.
const oauthRequestTimeout = 30 * time.Second

const deviceCodeGrantType = "urn:ietf:params:oauth:grant-type:device_code"

// loginScopes: openid for the ID token that names the signed-in user,
// email for the owner claim engram keys records by (ENGRAM_OWNER_CLAIM
// defaults to email), and offline_access for a refresh token that outlives
// the session.
var loginScopes = []string{oidc.ScopeOpenID, "email", oidc.ScopeOfflineAccess}

// errInvalidClient is the issuer's invalid_client answer to a device
// authorization request. Login uses it to re-register a cached client that
// the issuer no longer knows.
var errInvalidClient = errors.New("the issuer does not recognise the client")

// newIssuerHTTPClient builds the client for every request to the issuer.
// It never takes --insecure (cli-login design D7): that flag exists for a
// lab engram server's self-signed certificate, and the party that hands out
// credentials is never talked to without verification.
func newIssuerHTTPClient() *http.Client {
	return &http.Client{Timeout: oauthRequestTimeout}
}

// discoverIssuer reads the engram server's RFC 9728 protected-resource
// metadata and returns its first authorization server.
func discoverIssuer(ctx context.Context, hc *http.Client, serverURL string) (string, error) {
	u := cacheKey(serverURL) + wellKnownPRMBase
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", usageErrorf("--server %q: %w", serverURL, err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return "", unavailableErrorf("fetch %s: %w", u, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", unavailableErrorf("fetch %s: HTTP %d", u, resp.StatusCode)
	}
	var doc protectedResourceMetadata
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc); err != nil {
		return "", unavailableErrorf("parse %s: %w", u, err)
	}
	if len(doc.AuthorizationServers) == 0 || doc.AuthorizationServers[0] == "" {
		return "", usageErrorf("%s names no authorization server: it accepts anonymous calls, so there is nothing to log in to", serverURL)
	}
	issuer := doc.AuthorizationServers[0]
	if err := requireSecureOAuthURL("issuer", issuer); err != nil {
		return "", err
	}
	return issuer, nil
}

// requireSecureOAuthURL refuses an issuer or OAuth endpoint that is not an
// absolute https URL, so a device code, a refresh token or a registration
// token never crosses the network in clear text. Plain http is allowed
// only on a loopback host, for a local IdP.
func requireSecureOAuthURL(what, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return authErrorf("the %s %q is not an absolute URL", what, raw)
	}
	switch {
	case u.Scheme == "https":
		return nil
	case u.Scheme == "http" && isLoopbackHost(u.Hostname()):
		return nil
	default:
		return authErrorf("the %s %s does not use https; engram login sends credentials only over https (plain http only to a loopback host)", what, raw)
	}
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// issuerMetadata is what login and logout need from the issuer's OIDC
// discovery document. go-oidc exposes the device endpoint through
// Endpoint() but not the registration or revocation endpoints, which are
// read from the raw document.
type issuerMetadata struct {
	provider     *oidc.Provider
	endpoint     oauth2.Endpoint
	registration string
	revocation   string
}

// discoverIssuerMetadata reads the issuer's discovery document. ctx must
// outlive the returned provider: go-oidc keeps it for later JWKS fetches,
// so callers pass an uncancelled context and rely on hc's timeout.
func discoverIssuerMetadata(ctx context.Context, hc *http.Client, issuer string) (*issuerMetadata, error) {
	p, err := oidc.NewProvider(oidc.ClientContext(ctx, hc), issuer)
	if err != nil {
		return nil, unavailableErrorf("read the OIDC discovery document of %s: %w", issuer, err)
	}
	var extra struct {
		Registration string `json:"registration_endpoint"`
		Revocation   string `json:"revocation_endpoint"`
	}
	if err := p.Claims(&extra); err != nil {
		return nil, unavailableErrorf("parse the OIDC discovery document of %s: %w", issuer, err)
	}
	// A public client has no secret to put in a Basic header, so the client
	// ID always goes in the form body; auto-detection would first try a
	// Basic header with an empty secret.
	endpoint := p.Endpoint()
	endpoint.AuthStyle = oauth2.AuthStyleInParams
	for what, u := range map[string]string{
		"token endpoint":                endpoint.TokenURL,
		"device authorization endpoint": endpoint.DeviceAuthURL,
		"registration endpoint":         extra.Registration,
		"revocation endpoint":           extra.Revocation,
	} {
		if u == "" {
			continue
		}
		if err := requireSecureOAuthURL(what, u); err != nil {
			return nil, err
		}
	}
	return &issuerMetadata{provider: p, endpoint: endpoint, registration: extra.Registration, revocation: extra.Revocation}, nil
}

// clientRegistration is the part of an RFC 7591 registration response the
// cache keeps.
type clientRegistration struct {
	ClientID                string `json:"client_id"`
	RegistrationAccessToken string `json:"registration_access_token"`
	RegistrationClientURI   string `json:"registration_client_uri"`
}

// registrationClientName names the machine, so an IdP administrator can
// tell which machine a registered client belongs to.
func registrationClientName() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return "engram CLI (" + h + ")"
	}
	return "engram CLI"
}

// registerClient registers a public client for the device grant (RFC
// 7591). The body is fixed, so this is one JSON POST rather than a
// dependency (cli-login design D2).
func registerClient(ctx context.Context, hc *http.Client, endpoint string) (*clientRegistration, error) {
	body, err := json.Marshal(map[string]any{
		"client_name":                registrationClientName(),
		"grant_types":                []string{deviceCodeGrantType, "refresh_token"},
		"token_endpoint_auth_method": "none",
		"scope":                      strings.Join(loginScopes, " "),
	})
	if err != nil {
		return nil, fmt.Errorf("encode the client registration: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, unavailableErrorf("register a client at %s: %w", endpoint, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, unavailableErrorf("register a client at %s: %w", endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, unavailableErrorf("register a client at %s: %w", endpoint, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, authErrorf("the issuer refused client registration at %s: %s", endpoint, oauthErrorText(resp.StatusCode, b))
	}
	var reg clientRegistration
	if err := json.Unmarshal(b, &reg); err != nil || reg.ClientID == "" {
		return nil, authErrorf("the issuer's client registration at %s returned no client_id", endpoint)
	}
	if reg.RegistrationClientURI != "" {
		if err := requireSecureOAuthURL("registration client URI", reg.RegistrationClientURI); err != nil {
			return nil, err
		}
	}
	return &reg, nil
}

// deleteRegisteredClient deletes a client this CLI registered (RFC 7592).
func deleteRegisteredClient(ctx context.Context, hc *http.Client, clientURI, registrationAccessToken string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, clientURI, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+registrationAccessToken)
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return errors.New(oauthErrorText(resp.StatusCode, b))
	}
	return nil
}

// revokeRefreshToken revokes a refresh token for a public client (RFC 7009).
func revokeRefreshToken(ctx context.Context, hc *http.Client, revocationURL, clientID, refreshToken string) error {
	form := url.Values{
		"token":           {refreshToken},
		"token_type_hint": {"refresh_token"},
		"client_id":       {clientID},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, revocationURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return errors.New(oauthErrorText(resp.StatusCode, b))
	}
	return nil
}

// oauthErrorText renders an issuer's error response as "HTTP <status>",
// plus the RFC 6749 error and description when the body carries them.
func oauthErrorText(status int, body []byte) string {
	var e struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error != "" {
		if e.Description != "" {
			return fmt.Sprintf("HTTP %d: %s: %s", status, e.Error, e.Description)
		}
		return fmt.Sprintf("HTTP %d: %s", status, e.Error)
	}
	return fmt.Sprintf("HTTP %d", status)
}

func retrieveErrorText(re *oauth2.RetrieveError) string {
	if re.ErrorDescription != "" {
		return re.ErrorCode + ": " + re.ErrorDescription
	}
	if re.ErrorCode != "" {
		return re.ErrorCode
	}
	if re.Response != nil {
		return fmt.Sprintf("HTTP %d", re.Response.StatusCode)
	}
	return re.Error()
}

// requestDeviceCode starts the device authorization grant (RFC 8628).
func requestDeviceCode(ctx context.Context, conf *oauth2.Config) (*oauth2.DeviceAuthResponse, error) {
	da, err := conf.DeviceAuth(ctx)
	if err != nil {
		var re *oauth2.RetrieveError
		if errors.As(err, &re) {
			if re.ErrorCode == "invalid_client" {
				return nil, errInvalidClient
			}
			return nil, authErrorf("the issuer refused the device login: %s", retrieveErrorText(re))
		}
		return nil, unavailableErrorf("start the device login at %s: %w", conf.Endpoint.DeviceAuthURL, err)
	}
	return da, nil
}

// pollDeviceToken waits for the user to approve the device login. It is a
// variable so a test can answer at once instead of waiting out the
// issuer's polling interval, which RFC 8628 floors at one second.
var pollDeviceToken = func(ctx context.Context, conf *oauth2.Config, da *oauth2.DeviceAuthResponse) (*oauth2.Token, error) {
	return conf.DeviceAccessToken(ctx, da)
}

// classifyDeviceTokenError maps a failed wait for approval onto the exit
// code taxonomy (cli-login design D8).
func classifyDeviceTokenError(err error) error {
	var re *oauth2.RetrieveError
	isRetrieve := errors.As(err, &re)
	switch {
	case errors.Is(err, context.DeadlineExceeded), isRetrieve && re.ErrorCode == "expired_token":
		return authErrorf("the login code expired before it was approved; run engram login again")
	case isRetrieve && re.ErrorCode == "access_denied":
		return authErrorf("the login was denied at the issuer")
	case isRetrieve:
		return authErrorf("the issuer refused the login: %s", retrieveErrorText(re))
	default:
		return unavailableErrorf("wait for the device login: %w", err)
	}
}

// loginIdentity names the signed-in user for the success line, from the
// verified ID token. It is display only — engram checks the access token,
// never this — so any failure yields "" and the line omits the name.
func loginIdentity(ctx context.Context, md *issuerMetadata, clientID string, tok *oauth2.Token) string {
	raw, _ := tok.Extra("id_token").(string)
	if raw == "" {
		return ""
	}
	idt, err := md.provider.Verifier(&oidc.Config{ClientID: clientID}).Verify(ctx, raw)
	if err != nil {
		return ""
	}
	var c struct {
		Email             string `json:"email"`
		PreferredUsername string `json:"preferred_username"`
	}
	if idt.Claims(&c) != nil {
		return ""
	}
	switch {
	case c.Email != "":
		return c.Email
	case c.PreferredUsername != "":
		return c.PreferredUsername
	default:
		return idt.Subject
	}
}
