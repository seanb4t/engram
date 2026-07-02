// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import "testing"

func TestOwnerClaimGuard(t *testing.T) {
	cases := []struct {
		name         string
		bearerIssuer string
		uiEnabled    bool
		ownerClaim   string
		wantErr      bool
	}{
		{"no auth lane, empty claim ok", "", false, "", false},
		{"bearer lane, default claim ok", "https://issuer", false, "email", false},
		{"bearer lane, non-default claim ok (warns, no error)", "https://issuer", false, "preferred_username", false},
		{"bearer lane, empty claim rejected", "https://issuer", false, "", true},
		{"ui lane, empty claim rejected", "", true, "", true},
		{"ui lane, default claim ok", "", true, "email", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ownerClaimGuard(tc.bearerIssuer, tc.uiEnabled, tc.ownerClaim)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ownerClaimGuard(%q, %v, %q) err=%v, wantErr=%v",
					tc.bearerIssuer, tc.uiEnabled, tc.ownerClaim, err, tc.wantErr)
			}
		})
	}
}
