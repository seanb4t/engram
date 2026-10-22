// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import "testing"

// TestMCPResourceURL pins D-03 (GH-526, 260909-ofg-PLAN.md): the registry
// carries exactly one row for server.mcp_resource_url with an empty Legacy
// (brand-new key, nothing retired to guard against) and an empty Flag
// (env-only -- a deployment-topology value, never typed at a prompt); the
// env var reaches cfg.Server.MCPResourceURL when set; and it stays empty
// when unset (no registry Default for this key).
func TestMCPResourceURL(t *testing.T) {
	t.Run("registry entry", func(t *testing.T) {
		var matches int
		for _, f := range registry {
			if f.Key != "server.mcp_resource_url" {
				continue
			}
			matches++
			if f.Legacy != "" {
				t.Fatalf("server.mcp_resource_url has Legacy=%q, want empty (brand-new var)", f.Legacy)
			}
			if f.Flag != "" {
				t.Fatalf("server.mcp_resource_url has Flag=%q, want empty (env-only, D-03)", f.Flag)
			}
		}
		if matches != 1 {
			t.Fatalf("registry has %d entries for server.mcp_resource_url, want exactly 1", matches)
		}
	})

	t.Run("from env", func(t *testing.T) {
		t.Setenv("ENGRAM_QDRANT_ADDR", "localhost:6334")
		t.Setenv("ENGRAM_MCP_RESOURCE_URL", "https://gateway.example.test/mcp")
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Server.MCPResourceURL != "https://gateway.example.test/mcp" {
			t.Fatalf("Server.MCPResourceURL = %q, want %q", cfg.Server.MCPResourceURL, "https://gateway.example.test/mcp")
		}
	})

	t.Run("unset is empty", func(t *testing.T) {
		t.Setenv("ENGRAM_QDRANT_ADDR", "localhost:6334")
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Server.MCPResourceURL != "" {
			t.Fatalf("Server.MCPResourceURL = %q, want empty", cfg.Server.MCPResourceURL)
		}
	})
}

// TestMCPResourceOrigins checks that ENGRAM_MCP_RESOURCE_ORIGINS has one
// env-only registry row, reaches cfg.Server.MCPResourceOrigins verbatim, and
// is empty when unset.
func TestMCPResourceOrigins(t *testing.T) {
	t.Run("registry entry", func(t *testing.T) {
		var matches int
		for _, f := range registry {
			if f.Key != "server.mcp_resource_origins" {
				continue
			}
			matches++
			if f.Env != "ENGRAM_MCP_RESOURCE_ORIGINS" || f.Legacy != "" || f.Flag != "" || f.Default != "" {
				t.Fatalf("server.mcp_resource_origins row = %+v, want env-only with no legacy, flag or default", f)
			}
		}
		if matches != 1 {
			t.Fatalf("registry has %d entries for server.mcp_resource_origins, want exactly 1", matches)
		}
	})

	t.Run("from env", func(t *testing.T) {
		t.Setenv("ENGRAM_QDRANT_ADDR", "localhost:6334")
		t.Setenv("ENGRAM_MCP_RESOURCE_ORIGINS", "https://a.example, https://b.example")
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if want := "https://a.example, https://b.example"; cfg.Server.MCPResourceOrigins != want {
			t.Fatalf("Server.MCPResourceOrigins = %q, want %q", cfg.Server.MCPResourceOrigins, want)
		}
	})

	t.Run("unset is empty", func(t *testing.T) {
		t.Setenv("ENGRAM_QDRANT_ADDR", "localhost:6334")
		cfg, err := Load(nil)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Server.MCPResourceOrigins != "" {
			t.Fatalf("Server.MCPResourceOrigins = %q, want empty", cfg.Server.MCPResourceOrigins)
		}
	})
}
