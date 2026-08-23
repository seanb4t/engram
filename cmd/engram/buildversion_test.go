// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import "testing"

// TestNextPatch is the D-04/D-05 table test over nextPatch's grammar. Every
// row here is a pure-function assertion — no test in this file calls
// debug.ReadBuildInfo(), because a `go test` binary carries its own
// embedded build info rather than an injectable fixture.
//
// The "grammar" rows (M-1/L-6 from cycle-1 review) are the ones
// strconv.Atoi alone would have let through: Atoi accepts a leading sign
// ("+14", "-0" parse cleanly) and silently normalizes leading zeros
// ("014" -> 14), none of which is the "plain non-negative integer" grammar
// nextPatch claims. They exist to prove the anchored patchCorePattern is in
// place rather than the weaker Atoi-based parse.
func TestNextPatch(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"basic patch bump", "0.14.0", "0.14.1", true},
		{"double-digit patch bump", "1.2.9", "1.2.10", true},
		{"prerelease base rejected", "0.14.0-rc.1", "", false},
		{"v-prefixed input rejected (D-08)", "v0.14.0", "", false},
		{"two-component input rejected", "0.14", "", false},
		{"empty string rejected", "", "", false},
		{"leading plus on major rejected", "+0.14.0", "", false},
		{"leading plus on minor rejected", "0.+14.0", "", false},
		{"leading plus on patch rejected", "0.14.+0", "", false},
		{"leading minus on major rejected", "-0.14.0", "", false},
		{"leading minus on patch rejected", "0.14.-0", "", false},
		{"leading zero on major rejected", "01.14.0", "", false},
		{"leading zero on minor rejected", "0.014.0", "", false},
		{"leading zero on patch rejected", "0.14.00", "", false},
		{"component wider than 32-bit bound rejected", "1.2.4294967296", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := nextPatch(c.in)
			if got != c.want || ok != c.ok {
				t.Errorf("nextPatch(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
			}
		})
	}
}

// TestDeriveDevVersion covers deriveDevVersion's branches: the happy path,
// the dirty-marker suffix, a short revision used whole, and the two empty-
// string failure modes (no revision, unparseable base). It also carries the
// ordering assertion in its own subtest, deliberately scoped no wider than
// what it actually proves — see that subtest's comment.
func TestDeriveDevVersion(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		got := deriveDevVersion("0.14.0", "800a98f1a2b3c4d5e6f7", false)
		want := "0.14.1-dev.0+g800a98f1"
		if got != want {
			t.Errorf("deriveDevVersion(...) = %q, want %q", got, want)
		}
	})

	t.Run("dirty marker appended", func(t *testing.T) {
		got := deriveDevVersion("0.14.0", "800a98f1a2b3c4d5e6f7", true)
		want := "0.14.1-dev.0+g800a98f1.dirty"
		if got != want {
			t.Errorf("deriveDevVersion(..., dirty=true) = %q, want %q", got, want)
		}
	})

	t.Run("short revision used whole, not padded", func(t *testing.T) {
		got := deriveDevVersion("0.14.0", "abc123", false)
		want := "0.14.1-dev.0+gabc123"
		if got != want {
			t.Errorf("deriveDevVersion(..., short revision) = %q, want %q", got, want)
		}
	})

	t.Run("empty revision yields empty string", func(t *testing.T) {
		got := deriveDevVersion("0.14.0", "", false)
		if got != "" {
			t.Errorf("deriveDevVersion(lastRelease, \"\", false) = %q, want empty string", got)
		}
	})

	t.Run("unparseable base yields empty string", func(t *testing.T) {
		got := deriveDevVersion("garbage", "800a98f1a2b3c4d5e6f7", false)
		if got != "" {
			t.Errorf("deriveDevVersion(\"garbage\", ...) = %q, want empty string", got)
		}
	})

	// Ordering assertion, deliberately as narrow as it is (M-8 from cycle-1
	// review). golang.org/x/mod/semver-style precedence isn't available
	// without a new dependency, so the ordering fact is asserted
	// structurally: the derived string starts with the nextPatch result,
	// and the character immediately after it is "-", making it a
	// prerelease of that patch (hence ordered below it, per SemVer
	// precedence). This proves the derived string is a prerelease of the
	// patch-bumped lower bound nextPatch(lastRelease), which TestNextPatch
	// separately establishes is greater than lastRelease. It does NOT
	// establish lastRelease < dev-string < nextRelease: nextRelease is
	// whatever release-please computes from Conventional Commits (usually
	// a minor bump under bump-minor-pre-major: true), and this test never
	// observes it.
	t.Run("derived string is a prerelease of nextPatch(lastRelease)", func(t *testing.T) {
		base := "0.14.0"
		next, ok := nextPatch(base)
		if !ok {
			t.Fatalf("nextPatch(%q) returned not-ok, want ok", base)
		}
		got := deriveDevVersion(base, "800a98f1a2b3c4d5e6f7", false)
		if len(got) <= len(next) || got[:len(next)] != next || got[len(next)] != '-' {
			t.Errorf("deriveDevVersion(...) = %q, want a prerelease of %q (prefix %q followed by '-')", got, next, next)
		}
	})
}

// TestVersionFromModuleVersion covers the go install pkg@vX.Y.Z path (the
// live bug D-04 names) and its rejections: (devel), a Go pseudo-version
// (confirmed empirically — RESEARCH.md's Verified Finding — to be what a
// local `go build` actually embeds, not "(devel)"), a dirty pseudo-version,
// the empty string, and an unprefixed bare SemVer (module versions are
// always v-prefixed).
func TestVersionFromModuleVersion(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"release tag maps to bare SemVer", "v0.14.0", "0.14.0", true},
		{"devel sentinel rejected", "(devel)", "", false},
		{"go pseudo-version rejected", "v0.14.1-0.20260823183658-cc16ea664fb6", "", false},
		{"dirty go pseudo-version rejected", "v0.14.1-0.20260823183658-cc16ea664fb6+dirty", "", false},
		{"empty string rejected", "", "", false},
		{"unprefixed bare SemVer rejected", "0.14.0", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := versionFromModuleVersion(c.in)
			if got != c.want || ok != c.ok {
				t.Errorf("versionFromModuleVersion(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
			}
		})
	}
}
