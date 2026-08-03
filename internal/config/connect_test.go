// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"strconv"
	"strings"
	"testing"
)

// TestConnectHeadlessDefault pins D-10: with no ENGRAM_CONNECT_HEADLESS set,
// Load resolves cfg.Connect.Headless to "false", and strconv.ParseBool of
// that value yields false — an unset flag defaults off.
func TestConnectHeadlessDefault(t *testing.T) {
	t.Setenv("ENGRAM_QDRANT_ADDR", "localhost:6334")
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Connect.Headless != "false" {
		t.Fatalf("Connect.Headless = %q, want \"false\"", cfg.Connect.Headless)
	}
	got, err := strconv.ParseBool(cfg.Connect.Headless)
	if err != nil {
		t.Fatalf("ParseBool(%q): %v", cfg.Connect.Headless, err)
	}
	if got {
		t.Fatalf("ParseBool(%q) = true, want false", cfg.Connect.Headless)
	}
}

// TestConnectHeadlessFromEnv proves ENGRAM_CONNECT_HEADLESS=true reaches
// cfg.Connect.Headless through the single ENGRAM_ registry.
func TestConnectHeadlessFromEnv(t *testing.T) {
	t.Setenv("ENGRAM_QDRANT_ADDR", "localhost:6334")
	t.Setenv("ENGRAM_CONNECT_HEADLESS", "true")
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Connect.Headless != "true" {
		t.Fatalf("Connect.Headless = %q, want \"true\"", cfg.Connect.Headless)
	}
}

// TestConnectHeadlessHasNoLegacyVar pins D-10's "no Legacy value" rule: the
// registry entry for connect.headless carries an empty Legacy field, so no
// MEM_* name can satisfy it and the retired-var fatal guard stays intact —
// this is a brand-new var, not a renamed one.
func TestConnectHeadlessHasNoLegacyVar(t *testing.T) {
	for _, f := range registry {
		if f.Key == "connect.headless" {
			if f.Legacy != "" {
				t.Fatalf("connect.headless has Legacy=%q, want empty (brand-new var)", f.Legacy)
			}
			return
		}
	}
	t.Fatal("registry has no connect.headless entry")
}

// TestConnectHeadlessHasFlagBinding proves --connect-headless is wired
// through the same flag registry every other Env+Flag field uses:
// config.FlagDefault reports "false", and the flag name maps to the
// connect.headless koanf key.
func TestConnectHeadlessHasFlagBinding(t *testing.T) {
	if got := FlagDefault("connect-headless"); got != "false" {
		t.Fatalf("FlagDefault(\"connect-headless\") = %q, want \"false\"", got)
	}
	if got := flagToKey["connect-headless"]; got != "connect.headless" {
		t.Fatalf("flagToKey[\"connect-headless\"] = %q, want \"connect.headless\"", got)
	}
}

// TestConnectHeadlessRejectsNonBoolean proves Validate rejects a non-boolean
// connect.headless value at load time, naming the env var, rather than
// letting a typo silently read as off.
func TestConnectHeadlessRejectsNonBoolean(t *testing.T) {
	cfg := validConfigForServiceAuthTests()
	cfg.Connect.Headless = "yes"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate(): want error for non-boolean connect.headless, got nil")
	}
	if !strings.Contains(err.Error(), "ENGRAM_CONNECT_HEADLESS") {
		t.Fatalf("Validate() error %v does not name ENGRAM_CONNECT_HEADLESS", err)
	}
}
