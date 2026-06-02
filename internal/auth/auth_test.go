// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import "testing"

func TestIdentityPrefersHumanReadable(t *testing.T) {
	tests := []struct {
		name                     string
		subject, email, username string
		want                     string
	}{
		{"email wins", "sub-123", "a@b.com", "alice", "a@b.com"},
		{"username when no email", "sub-123", "", "alice", "alice"},
		{"subject as last resort", "sub-123", "", "", "sub-123"},
		{"email beats username", "sub-123", "a@b.com", "alice", "a@b.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := identity(tt.subject, tt.email, tt.username); got != tt.want {
				t.Errorf("identity(%q,%q,%q) = %q, want %q",
					tt.subject, tt.email, tt.username, got, tt.want)
			}
		})
	}
}
