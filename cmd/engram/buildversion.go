// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
)

// lastRelease is the release-please-managed patch-bump BASE for dev-build
// version derivation (D-05) — it is the value the fourth `extra-files`
// entry in release-please-config.json rewrites automatically on every
// release, matched via the trailing annotation below (which must stay on
// the same line as the value; that placement is load-bearing for the
// updater, not decoration). It is the BASE, never a prediction of the next
// version: with bump-minor-pre-major: true the real next release after
// 0.14.0 is usually 0.15.0, not 0.14.1, so the derived 0.14.1-dev.0 below
// is a correctly-ordering lower bound and nothing more.
const lastRelease = "0.14.1" // x-release-please-version

// patchCorePattern anchors nextPatch's input to a plain, unsigned SemVer
// core: exactly three dot-separated components, each either "0" or a
// non-zero digit followed by more digits. strconv.Atoi alone would accept a
// leading sign ("+14", "-0" both parse) and silently reformat leading zeros
// ("014" -> 14) — neither of which is this "plain non-negative integer"
// grammar — so this anchored pattern is what actually enforces it, not the
// numeric parse that follows: "^"/"$" reject "0.14.0-rc.1", "v0.14.0", and
// "0.14"; the (0|[1-9][0-9]*) branch rejects signs and leading zeros in one
// construct.
var patchCorePattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// moduleTagPattern distinguishes a real release tag (as embedded by
// `go install pkg@vX.Y.Z`) from Go's own pseudo-version shape
// (vX.Y.Z-0.<timestamp>-<12charhash>, optionally +dirty) — which
// RESEARCH.md confirmed empirically is what a local `go build`'s embedded
// module version actually looks like, not the "(devel)" sentinel
// originally assumed. That correction doesn't change versionFromModuleVersion's
// locked decision below: the pseudo-version carries a timestamp and a
// full-length hash rather than the git-describe-style short hash D-04
// specifies, so the strict anchor here is what tells the two apart.
var moduleTagPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

// nextPatch parses v as a bare SemVer core (no prerelease, no build
// metadata, no "v" prefix — D-08) and returns the patch-incremented
// version. It returns ("", false) on anything that does not match — never
// a partially-formed string.
func nextPatch(v string) (string, bool) {
	m := patchCorePattern.FindStringSubmatch(v)
	if m == nil {
		return "", false
	}
	// strconv.ParseUint(c, 10, 32), not Atoi: the 32-bit width bounds each
	// component at 4294967295 — far beyond anything release-please could
	// produce — so patch+1 cannot wrap, and it makes the bound an explicit,
	// tested rejection rather than an unstated assumption.
	major, err := strconv.ParseUint(m[1], 10, 32)
	if err != nil {
		return "", false
	}
	minor, err := strconv.ParseUint(m[2], 10, 32)
	if err != nil {
		return "", false
	}
	patch, err := strconv.ParseUint(m[3], 10, 32)
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch+1), true
}

// deriveDevVersion composes the dev-build version string from a release
// base, a VCS revision, and its dirty state. It takes revision and dirty as
// parameters rather than reading them internally — the same seam
// outputFormatFromConfig (client_common.go) uses for isTTY — so a table
// test can force every branch without a real build-info fixture.
func deriveDevVersion(lastRelease, revision string, dirty bool) string {
	next, ok := nextPatch(lastRelease)
	if !ok {
		return ""
	}
	if revision == "" {
		return ""
	}
	short := revision
	if len(short) > 8 {
		short = short[:8]
	}
	// The literal 0 in "-dev.0" is a recorded decision (see 01-02-PLAN.md's
	// objective): debug.BuildInfo.Settings carries no commit-distance field
	// at all — only vcs, vcs.revision, vcs.time, and vcs.modified
	// (RESEARCH.md Open Question 1) — so a real "commits since last tag"
	// count would need either a git rev-list --count shell-out at build
	// time via ldflags (reopening the no-Taskfile-change premise D-04
	// exists to preserve) or git present at runtime on the end user's
	// machine. D-04's only ordering requirement is that this string sits
	// strictly above lastRelease and strictly below nextPatch(lastRelease),
	// and a fixed 0 fully satisfies that. Per-build uniqueness comes from
	// the revision and the dirty marker below, which is where it belongs.
	v := fmt.Sprintf("%s-dev.0+g%s", next, short)
	if dirty {
		v += ".dirty"
	}
	return v
}

// versionFromModuleVersion maps a bare release-tag module version
// (vX.Y.Z, as embedded in debug.BuildInfo.Main.Version by
// `go install …/cmd/engram@vX.Y.Z`) to its bare SemVer form (D-08: no "v"
// prefix), returning ("", false) on anything else — including Go's own
// pseudo-version shape, "(devel)", the empty string, and an unprefixed bare
// SemVer (module versions are always v-prefixed).
func versionFromModuleVersion(mainVersion string) (string, bool) {
	if !moduleTagPattern.MatchString(mainVersion) {
		return "", false
	}
	return strings.TrimPrefix(mainVersion, "v"), true
}

// resolvedVersion is the single source every version-reporting surface in
// this binary calls (version.go's RunE and all three of serve.go's
// surfaces), resolved in this order:
//  1. A GoReleaser-stamped release build — `version` is anything other
//     than "dev" — returns it unchanged. This is the only path the cask's
//     install gate ever exercises and must stay byte-identical to today.
//  2. debug.ReadBuildInfo() reporting not-ok returns the unchanged "dev"
//     sentinel.
//  3. bi.Main.Version being a real release tag (the `go install …@vX.Y.Z`
//     path) returns its bare form. This closes D-04's named bug — that
//     install path applies no ldflags and reports "dev" today even though
//     the module version is correct — and it is checked before the VCS
//     branch below because that install path embeds no VCS settings at
//     all.
//  4. Otherwise, bi.Settings is walked once for vcs.revision/vcs.modified
//     and passed to deriveDevVersion.
//  5. If derivation yields nothing (e.g. no VCS info embedded, such as a
//     -buildvcs=false build), the unchanged "dev" sentinel is returned.
func resolvedVersion() string {
	if version != "dev" {
		return version
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	if v, ok := versionFromModuleVersion(bi.Main.Version); ok {
		return v
	}
	var revision string
	var dirty bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if dev := deriveDevVersion(lastRelease, revision, dirty); dev != "" {
		return dev
	}
	return version
}
