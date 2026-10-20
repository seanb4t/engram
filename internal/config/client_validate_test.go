// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"strings"
	"testing"

	flag "github.com/spf13/pflag"
)

// TestValidateOutputFormat pins the single exported validator both the
// client tier (via ValidateClient) and the operator tier
// (cmd/engram/operator_output.go's operatorOutputFormat) call: exactly
// "json", "text", and "" are legal; every other value is rejected naming
// both the flag and its legal values.
func TestValidateOutputFormat(t *testing.T) {
	for _, v := range []string{"json", "text", ""} {
		if err := ValidateOutputFormat(v); err != nil {
			t.Errorf("ValidateOutputFormat(%q) = %v, want nil", v, err)
		}
	}

	cases := []string{"yaml", "JSON", "Text", " json", "json "}
	for _, v := range cases {
		err := ValidateOutputFormat(v)
		if err == nil {
			t.Fatalf("ValidateOutputFormat(%q) = nil, want an error", v)
		}
		if !strings.Contains(err.Error(), "--output") || !strings.Contains(err.Error(), v) {
			t.Errorf("ValidateOutputFormat(%q) error = %q, want it to name --output and the illegal value", v, err)
		}
	}
}

// TestValidateSetupAuth pins `engram setup --auth`'s validator: exactly
// "oauth", "oauth-client", "bearer", "none", and "" (empty — the caller
// defaults it) are legal; every other value is rejected naming both the
// flag and all four accepted modes.
func TestValidateSetupAuth(t *testing.T) {
	for _, v := range []string{"oauth", "oauth-client", "bearer", "none", ""} {
		if err := ValidateSetupAuth(v); err != nil {
			t.Errorf("ValidateSetupAuth(%q) = %v, want nil", v, err)
		}
	}

	err := ValidateSetupAuth("basic")
	if err == nil {
		t.Fatal("ValidateSetupAuth(\"basic\") = nil, want an error")
	}
	for _, want := range []string{"--auth", "basic", "oauth", "oauth-client", "bearer", "none"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ValidateSetupAuth(\"basic\") error = %q, want it to contain %q", err, want)
		}
	}
}

// TestClientConfigLoadPrecedence locks in the client.* registry rows'
// wiring into cfg.Client: registry defaults, the ENGRAM_ env layer (including
// the empty-env-preserves-default guard), and the deliberate absence of an
// ENGRAM_TOKEN row (D-13 — the credential never routes through koanf).
func TestClientConfigLoadPrecedence(t *testing.T) {
	t.Run("defaults with no env set", func(t *testing.T) {
		t.Setenv("ENGRAM_SERVER_URL", "")
		t.Setenv("ENGRAM_TIMEOUT", "")
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Client.Timeout != "30s" {
			t.Errorf("Client.Timeout = %q, want 30s default", cfg.Client.Timeout)
		}
		if cfg.Client.Insecure != "false" {
			t.Errorf("Client.Insecure = %q, want false default", cfg.Client.Insecure)
		}
	})

	t.Run("ENGRAM_SERVER_URL sets ServerURL", func(t *testing.T) {
		t.Setenv("ENGRAM_SERVER_URL", "https://x")
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Client.ServerURL != "https://x" {
			t.Errorf("Client.ServerURL = %q, want https://x", cfg.Client.ServerURL)
		}
	})

	t.Run("ENGRAM_TIMEOUT sets Timeout", func(t *testing.T) {
		t.Setenv("ENGRAM_TIMEOUT", "45s")
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Client.Timeout != "45s" {
			t.Errorf("Client.Timeout = %q, want 45s", cfg.Client.Timeout)
		}
	})

	t.Run("empty ENGRAM_TIMEOUT preserves default", func(t *testing.T) {
		t.Setenv("ENGRAM_TIMEOUT", "")
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Client.Timeout != "30s" {
			t.Errorf("Client.Timeout = %q, want 30s (empty env preserves default)", cfg.Client.Timeout)
		}
	})

	t.Run("ENGRAM_CLIENT_ID sets ClientID", func(t *testing.T) {
		t.Setenv("ENGRAM_CLIENT_ID", "engram-cli")
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Client.ClientID != "engram-cli" {
			t.Errorf("Client.ClientID = %q, want engram-cli", cfg.Client.ClientID)
		}
	})

	t.Run("--client-id wins over ENGRAM_CLIENT_ID", func(t *testing.T) {
		t.Setenv("ENGRAM_CLIENT_ID", "from-env")
		f := flag.NewFlagSet("login", flag.ContinueOnError)
		f.String("client-id", "", "")
		if err := f.Parse([]string{"--client-id", "from-flag"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		cfg, err := Load(f)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Client.ClientID != "from-flag" {
			t.Errorf("Client.ClientID = %q, want from-flag", cfg.Client.ClientID)
		}
	})

	t.Run("no ENGRAM_TOKEN registry row", func(t *testing.T) {
		for _, f := range registry {
			if f.Env == "ENGRAM_TOKEN" {
				t.Fatalf("registry has an ENGRAM_TOKEN row (key %q) — the credential must never reach koanf (D-13)", f.Key)
			}
		}
	})
}

// TestLoadOverlayBindsNonStringFlag locks in that Load's changed-flag overlay
// reads a flag of any pflag type via Value.String(), not GetString — D-04
// puts a bool (--insecure) into the registry, so the overlay must not error
// on a non-string flag.
func TestLoadOverlayBindsNonStringFlag(t *testing.T) {
	t.Run("bool flag supplied", func(t *testing.T) {
		f := flag.NewFlagSet("client", flag.ContinueOnError)
		f.Bool("insecure", false, "")
		if err := f.Parse([]string{"--insecure"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		cfg, err := Load(f)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Client.Insecure != "true" {
			t.Errorf("Client.Insecure = %q, want true", cfg.Client.Insecure)
		}
	})

	t.Run("bool flag not supplied", func(t *testing.T) {
		f := flag.NewFlagSet("client", flag.ContinueOnError)
		f.Bool("insecure", false, "")
		if err := f.Parse([]string{}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		cfg, err := Load(f)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Client.Insecure != "false" {
			t.Errorf("Client.Insecure = %q, want false default", cfg.Client.Insecure)
		}
	})

	t.Run("string flag still overlays", func(t *testing.T) {
		f := flag.NewFlagSet("client", flag.ContinueOnError)
		f.String("server", "", "")
		if err := f.Parse([]string{"--server", "https://x"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		cfg, err := Load(f)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Client.ServerURL != "https://x" {
			t.Errorf("Client.ServerURL = %q, want https://x", cfg.Client.ServerURL)
		}
	})

	t.Run("existing string-flag row still overlays (regression)", func(t *testing.T) {
		t.Setenv("ENGRAM_LISTEN_ADDR", "")
		f := flag.NewFlagSet("serve", flag.ContinueOnError)
		f.String("listen-addr", "", "")
		if err := f.Parse([]string{"--listen-addr", ":9090"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		cfg, err := Load(f)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Server.ListenAddr != ":9090" {
			t.Errorf("Server.ListenAddr = %q, want :9090", cfg.Server.ListenAddr)
		}
	})
}

// validClientConfig returns a ClientConfig whose fields all pass
// ValidateClient. Tests mutate one field to exercise a single rule.
// Deliberately a fresh helper local to this file (zero pre-existing call
// sites), so the hand-built-literal tax this package's Config-shaped test
// helper already carries stays confined to Config, never spreading to
// ClientConfig.
func validClientConfig() ClientConfig {
	return ClientConfig{
		Timeout:  "30s",
		Insecure: "false",
	}
}

// TestClientConfigValidateFieldRules locks in ValidateClient's rules,
// including D-05's central claim: 0 (in either duration spelling) is a
// rejected usage error, never treated as unbounded.
func TestClientConfigValidateFieldRules(t *testing.T) {
	if err := ValidateClient(validClientConfig()); err != nil {
		t.Fatalf("ValidateClient(valid) = %v, want nil", err)
	}

	cases := []struct {
		name   string
		mutate func(*ClientConfig)
		want   string
	}{
		{"timeout zero", func(c *ClientConfig) { c.Timeout = "0" }, "ENGRAM_TIMEOUT"},
		{"timeout zero (0s spelling)", func(c *ClientConfig) { c.Timeout = "0s" }, "ENGRAM_TIMEOUT"},
		{"timeout negative", func(c *ClientConfig) { c.Timeout = "-1s" }, "ENGRAM_TIMEOUT"},
		{"timeout not a duration", func(c *ClientConfig) { c.Timeout = "abc" }, "ENGRAM_TIMEOUT"},
		{"timeout empty", func(c *ClientConfig) { c.Timeout = "" }, "ENGRAM_TIMEOUT"},
		{"insecure non-bool", func(c *ClientConfig) { c.Insecure = "maybe" }, "--insecure"},
		{"output bogus", func(c *ClientConfig) { c.Output = "bogus" }, "--output"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validClientConfig()
			tc.mutate(&c)
			err := ValidateClient(c)
			if err == nil {
				t.Fatalf("ValidateClient() = nil, want error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("ValidateClient() error = %q, want substring %q", err, tc.want)
			}
		})
	}

	t.Run("insecure true and false both valid", func(t *testing.T) {
		for _, v := range []string{"true", "false"} {
			c := validClientConfig()
			c.Insecure = v
			if err := ValidateClient(c); err != nil {
				t.Errorf("ValidateClient(Insecure=%q) = %v, want nil", v, err)
			}
		}
	})

	t.Run("output json, text, and empty all valid", func(t *testing.T) {
		for _, v := range []string{"json", "text", ""} {
			c := validClientConfig()
			c.Output = v
			if err := ValidateClient(c); err != nil {
				t.Errorf("ValidateClient(Output=%q) = %v, want nil", v, err)
			}
		}
	})

	t.Run("multiple bad fields aggregate into one error", func(t *testing.T) {
		c := ClientConfig{Timeout: "0", Insecure: "maybe", Output: "bogus"}
		err := ValidateClient(c)
		if err == nil {
			t.Fatal("ValidateClient() = nil, want aggregated error")
		}
		for _, want := range []string{"ENGRAM_TIMEOUT", "--insecure", "--output"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("ValidateClient() error = %q, want substring %q", err, want)
			}
		}
	})
}

// TestValidateConsole pins the console-only validator: empty values mean
// "not set"; an unknown theme names the setting and every valid theme; a
// non-boolean mouse value names the setting.
func TestValidateConsole(t *testing.T) {
	for _, c := range []ClientConfig{
		{},
		{TUITheme: "engram", TUIMouse: "true"},
		{TUITheme: "tokyo-night", TUIMouse: "false"},
	} {
		if err := ValidateConsole(c); err != nil {
			t.Errorf("ValidateConsole(%+v) = %v, want nil", c, err)
		}
	}

	err := ValidateConsole(ClientConfig{TUITheme: "nope"})
	if err == nil {
		t.Fatal(`ValidateConsole(theme "nope") = nil, want an error`)
	}
	for _, want := range append([]string{"--theme", "ENGRAM_TUI_THEME", "nope"}, TUIThemes...) {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("theme error = %q, want it to contain %q", err, want)
		}
	}

	err = ValidateConsole(ClientConfig{TUIMouse: "maybe"})
	if err == nil {
		t.Fatal(`ValidateConsole(mouse "maybe") = nil, want an error`)
	}
	for _, want := range []string{"--mouse", "ENGRAM_TUI_MOUSE", "maybe"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("mouse error = %q, want it to contain %q", err, want)
		}
	}
}

// TestValidateClientIgnoresConsoleSettings: a console setting never fails
// another client command.
func TestValidateClientIgnoresConsoleSettings(t *testing.T) {
	c := ClientConfig{Timeout: "30s", Insecure: "false", TUITheme: "nope", TUIMouse: "maybe"}
	if err := ValidateClient(c); err != nil {
		t.Errorf("ValidateClient with bad console settings = %v, want nil", err)
	}
}

// TestValidatePrimeScope: every rejection names the setting the value came
// from, and an empty value is an error only when --scope was passed.
func TestValidatePrimeScope(t *testing.T) {
	accepted := []struct {
		scope    string
		fromFlag bool
	}{
		{"", false},
		{"repo:github.com/org/repo", true},
		{"repo:github.com/org/repo:ws:feat", false},
		{"project:infra", true},
	}
	for _, c := range accepted {
		if err := ValidatePrimeScope(c.scope, c.fromFlag); err != nil {
			t.Errorf("ValidatePrimeScope(%q, %v) = %v, want nil", c.scope, c.fromFlag, err)
		}
	}

	rejected := []struct {
		name     string
		scope    string
		fromFlag bool
		want     []string
	}{
		{"empty flag", "", true, []string{"--scope", "blank"}},
		{"blank flag", "  ", true, []string{"--scope", "blank"}},
		{"blank env", " ", false, []string{"ENGRAM_SCOPE", "blank"}},
		{"rule flag", "rule:repo:github.com/org/repo", true, []string{"--scope", "rule scope is derived from the memory scope"}},
		{"rule env", "rule:repo:github.com/org/repo", false, []string{"ENGRAM_SCOPE", "rule"}},
		{"discovery flag", "discovery:repo:github.com/org/repo", true, []string{"--scope", "discovery"}},
		{"discovery env", "discovery:repo:github.com/org/repo", false, []string{"ENGRAM_SCOPE", "discovery"}},
	}
	for _, c := range rejected {
		t.Run(c.name, func(t *testing.T) {
			err := ValidatePrimeScope(c.scope, c.fromFlag)
			if err == nil {
				t.Fatalf("ValidatePrimeScope(%q, %v) = nil, want an error", c.scope, c.fromFlag)
			}
			for _, want := range c.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q, want it to contain %q", err, want)
				}
			}
		})
	}
}

// TestLoadPrimeSettings: --scope wins over ENGRAM_SCOPE, ENGRAM_SCOPE
// applies alone, and ENGRAM_PROJECT has no flag.
func TestLoadPrimeSettings(t *testing.T) {
	load := func(t *testing.T, args ...string) PrimeConfig {
		t.Helper()
		f := flag.NewFlagSet("prime", flag.ContinueOnError)
		f.String("scope", "", "")
		if err := f.Parse(args); err != nil {
			t.Fatalf("parse: %v", err)
		}
		cfg, err := Load(f)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		return cfg.Prime
	}

	t.Run("--scope wins over ENGRAM_SCOPE", func(t *testing.T) {
		t.Setenv("ENGRAM_SCOPE", "repo:a/b")
		if got := load(t, "--scope", "repo:c/d").Scope; got != "repo:c/d" {
			t.Errorf("Prime.Scope = %q, want repo:c/d", got)
		}
	})

	t.Run("ENGRAM_SCOPE alone", func(t *testing.T) {
		t.Setenv("ENGRAM_SCOPE", "repo:a/b")
		if got := load(t).Scope; got != "repo:a/b" {
			t.Errorf("Prime.Scope = %q, want repo:a/b", got)
		}
	})

	t.Run("ENGRAM_PROJECT", func(t *testing.T) {
		t.Setenv("ENGRAM_PROJECT", "selfhosted-cluster")
		if got := load(t).Project; got != "selfhosted-cluster" {
			t.Errorf("Prime.Project = %q, want selfhosted-cluster", got)
		}
	})
}

// TestValidateSetupCallbackPort pins --callback-port's bounds: empty is
// "not set" (0), 1..65535 parse, and 0, 70000, a negative or a non-numeric
// value are rejected with an error naming the range.
func TestValidateSetupCallbackPort(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
		ok   bool
	}{
		{"", 0, true}, {"1", 1, true}, {"8765", 8765, true}, {"65535", 65535, true},
		{"0", 0, false}, {"65536", 0, false}, {"70000", 0, false}, {"-1", 0, false}, {"abc", 0, false},
	} {
		got, err := ValidateSetupCallbackPort(tc.in)
		if (err == nil) != tc.ok {
			t.Errorf("ValidateSetupCallbackPort(%q) err = %v, want ok=%v", tc.in, err, tc.ok)
			continue
		}
		if err != nil && !strings.Contains(err.Error(), "1 to 65535") {
			t.Errorf("ValidateSetupCallbackPort(%q) err = %q, want it to name the range", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("ValidateSetupCallbackPort(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
