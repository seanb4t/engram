// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2"

	"github.com/seanb4t/engram/internal/config"
)

// loginCmd gets a refreshable credential for one server through the OAuth
// device authorization grant and caches it, so every client command can
// call that server with no token handling (cli-login, #818).
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to an engram server with your browser and cache a refreshable credential",
	Long: `Log in to an engram server and cache a credential that client commands use and refresh.

The command finds the server's authorization server from its protected-resource
metadata, registers a client there once per server (or uses --client-id), and
prints a URL and a code. Open the URL in a browser on any device and approve the
login. The credential is cached in $XDG_CONFIG_HOME/engram/credentials.json (else
~/.config/engram/credentials.json), readable only by you, and this server becomes
the default server for client commands that set neither --server nor
ENGRAM_SERVER_URL.

ENGRAM_TOKEN and --token-file still take precedence over the cached login.
Remove the login with engram logout.`,
	Args: cobra.NoArgs,
	RunE: runLogin,
}

// logoutCmd removes one server's cached login, revoking it at the issuer
// and deleting the client login registered.
var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove the cached login for an engram server and revoke it at the issuer",
	Long: `Remove the cached login for an engram server.

The command revokes the cached refresh token at the issuer, deletes the client
that engram login registered (never a --client-id client), and removes the cache
entry. With neither --server nor ENGRAM_SERVER_URL it logs out of the default
server. A failure at the issuer is a warning: the cache entry is removed anyway.`,
	Args: cobra.NoArgs,
	RunE: runLogout,
}

func init() {
	lf := loginCmd.Flags()
	lf.String("server", config.FlagDefault("server"),
		"engram server base URL to log in to (default: ENGRAM_SERVER_URL)")
	lf.String("client-id", config.FlagDefault("client-id"),
		"pre-registered OAuth client ID, for an issuer without dynamic client registration (default: ENGRAM_CLIENT_ID)")
	lf.Bool("insecure", false,
		"skip TLS certificate verification for the engram server only, never for the issuer (always warns on stderr; no environment fallback)")

	logoutCmd.Flags().String("server", config.FlagDefault("server"),
		"engram server base URL to log out of (default: ENGRAM_SERVER_URL, else the default server)")

	rootCmd.AddCommand(loginCmd, logoutCmd)
}

func runLogin(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load(cmd.Flags())
	if err != nil {
		return usageErrorf("load client configuration: %w", err)
	}
	if err := config.ValidateClient(cfg.Client); err != nil {
		return usageErrorf("%w", err)
	}
	if cfg.Client.ServerURL == "" {
		return usageErrorf("--server or ENGRAM_SERVER_URL is required")
	}
	// ValidateClient has already confirmed Insecure parses as a bool.
	insecure, _ := strconv.ParseBool(cfg.Client.Insecure)
	stderr := cmd.ErrOrStderr()
	if insecure {
		_, _ = fmt.Fprintln(stderr,
			"WARNING: TLS certificate verification is disabled for the engram server (--insecure); "+
				"the issuer is still verified")
	}

	server := cacheKey(cfg.Client.ServerURL)
	path, err := credentialsPath()
	if err != nil {
		return usageErrorf("%w", err)
	}
	cache, err := loadCredentialCache(path)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	serverHTTP := newHTTPClient(insecure)
	serverHTTP.Timeout = oauthRequestTimeout
	issuer, err := discoverIssuer(ctx, serverHTTP, server)
	if err != nil {
		return err
	}
	if insecure {
		// The metadata that named the issuer came over an unverified
		// connection, so a man-in-the-middle could have named a fake one.
		// The user is the only check left (the user's 2026-10-17 ruling:
		// keep --insecure on login and warn, rather than drop it).
		_, _ = fmt.Fprintf(stderr,
			"WARNING: the engram server named the issuer %s over an unverified connection (--insecure). "+
				"Approve the login only if the URL below is on the issuer host you expect.\n", issuer)
	}
	issuerHTTP := newIssuerHTTPClient()
	md, err := discoverIssuerMetadata(ctx, issuerHTTP, issuer)
	if err != nil {
		return err
	}
	if md.endpoint.DeviceAuthURL == "" {
		return authErrorf("the issuer %s does not support the device authorization grant (its discovery document has no device_authorization_endpoint)", issuer)
	}

	entry := &cachedLogin{Issuer: issuer, TokenURL: md.endpoint.TokenURL, RevocationURL: md.revocation}
	reused := false
	switch prev := cache.Servers[server]; {
	case cfg.Client.ClientID != "":
		entry.ClientID = cfg.Client.ClientID
	case prev != nil && prev.Issuer == issuer && prev.ClientID != "":
		entry.ClientID = prev.ClientID
		entry.Registered = prev.Registered
		entry.RegistrationAccessToken = prev.RegistrationAccessToken
		entry.RegistrationClientURI = prev.RegistrationClientURI
		reused = true
	default:
		if err := registerLoginClient(ctx, issuerHTTP, md, entry); err != nil {
			return err
		}
	}

	oauthCtx := context.WithValue(ctx, oauth2.HTTPClient, issuerHTTP)
	conf := &oauth2.Config{ClientID: entry.ClientID, Endpoint: md.endpoint, Scopes: loginScopes}
	da, err := requestDeviceCode(oauthCtx, conf)
	if errors.Is(err, errInvalidClient) && reused && entry.Registered {
		// The cached client is gone at the issuer (an administrator deleted
		// it, or the realm was rebuilt): register a new one, once.
		if err := registerLoginClient(ctx, issuerHTTP, md, entry); err != nil {
			return err
		}
		conf.ClientID = entry.ClientID
		da, err = requestDeviceCode(oauthCtx, conf)
	}
	if errors.Is(err, errInvalidClient) {
		return authErrorf("the issuer does not recognise client %q; check --client-id or ENGRAM_CLIENT_ID", entry.ClientID)
	}
	if err != nil {
		return err
	}

	printDevicePrompt(cmd, server, da)
	tok, err := pollDeviceToken(oauthCtx, conf, da)
	if err != nil {
		return classifyDeviceTokenError(err)
	}

	entry.Token = tok
	cache.Servers[server] = entry
	cache.DefaultServer = server
	if err := cache.save(path); err != nil {
		return err
	}

	if tok.RefreshToken == "" {
		_, _ = fmt.Fprintln(stderr,
			"WARNING: the issuer returned no refresh token; run engram login again when the access token expires")
	}
	if who := loginIdentity(ctx, md, entry.ClientID, tok); who != "" {
		_, _ = fmt.Fprintf(stderr, "Logged in to %s as %s.\n", server, who)
	} else {
		_, _ = fmt.Fprintf(stderr, "Logged in to %s.\n", server)
	}
	return nil
}

// registerLoginClient registers a new client and records it on entry,
// replacing any registration entry already held.
func registerLoginClient(ctx context.Context, hc *http.Client, md *issuerMetadata, entry *cachedLogin) error {
	if md.registration == "" {
		return authErrorf("the issuer %s offers no dynamic client registration; pass a pre-registered client with --client-id or ENGRAM_CLIENT_ID", entry.Issuer)
	}
	reg, err := registerClient(ctx, hc, md.registration)
	if err != nil {
		return err
	}
	entry.ClientID = reg.ClientID
	entry.Registered = true
	entry.RegistrationAccessToken = reg.RegistrationAccessToken
	entry.RegistrationClientURI = reg.RegistrationClientURI
	return nil
}

// printDevicePrompt tells the user where to approve the login. It goes to
// stderr, so stdout stays empty for a script.
func printDevicePrompt(cmd *cobra.Command, server string, da *oauth2.DeviceAuthResponse) {
	w := cmd.ErrOrStderr()
	_, _ = fmt.Fprintf(w, "To log in to %s, open this URL in a browser on any device and approve the login:\n\n", server)
	if da.VerificationURIComplete != "" {
		_, _ = fmt.Fprintf(w, "  %s\n\n", da.VerificationURIComplete)
		_, _ = fmt.Fprintf(w, "If the page asks for a code, enter %s.\n", da.UserCode)
	} else {
		_, _ = fmt.Fprintf(w, "  %s\n\nand enter the code %s.\n", da.VerificationURI, da.UserCode)
	}
	if !da.Expiry.IsZero() {
		_, _ = fmt.Fprintf(w, "Waiting for approval (the code expires in %s)...\n", time.Until(da.Expiry).Round(time.Minute))
	}
}

func runLogout(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load(cmd.Flags())
	if err != nil {
		return usageErrorf("load client configuration: %w", err)
	}
	path, err := credentialsPath()
	if err != nil {
		return usageErrorf("%w", err)
	}
	cache, err := loadCredentialCache(path)
	if err != nil {
		return err
	}
	stderr := cmd.ErrOrStderr()

	server := cfg.Client.ServerURL
	if server == "" {
		server = cache.DefaultServer
	}
	if server == "" {
		_, _ = fmt.Fprintln(stderr, "Not logged in to any server; nothing to log out of.")
		return nil
	}
	server = cacheKey(server)
	entry := cache.Servers[server]
	if entry == nil {
		_, _ = fmt.Fprintf(stderr, "Not logged in to %s; nothing to log out of.\n", server)
		return nil
	}

	ctx := cmd.Context()
	issuerHTTP := newIssuerHTTPClient()
	if entry.RevocationURL != "" && entry.Token != nil && entry.Token.RefreshToken != "" {
		if err := revokeRefreshToken(ctx, issuerHTTP, entry.RevocationURL, entry.ClientID, entry.Token.RefreshToken); err != nil {
			_, _ = fmt.Fprintf(stderr, "WARNING: could not revoke the refresh token at %s: %v\n", entry.RevocationURL, err)
		}
	}
	if entry.Registered && entry.RegistrationClientURI != "" && entry.RegistrationAccessToken != "" {
		if err := deleteRegisteredClient(ctx, issuerHTTP, entry.RegistrationClientURI, entry.RegistrationAccessToken); err != nil {
			_, _ = fmt.Fprintf(stderr, "WARNING: could not delete the registered client at %s: %v\n", entry.RegistrationClientURI, err)
		}
	}

	delete(cache.Servers, server)
	if cache.DefaultServer == server {
		cache.DefaultServer = ""
	}
	if err := cache.save(path); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stderr, "Logged out of %s.\n", server)
	return nil
}
