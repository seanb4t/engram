// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package scope

import (
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestPropertyNormalizeRemote(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		host := rapid.StringMatching(`[a-z][a-z0-9-]{0,10}(\.[a-z]{2,6}){1,2}`).Draw(t, "host")
		// An owner of digits only reads as a port after an scp colon, by
		// design, so the owner always holds a letter.
		owner := rapid.StringMatching(`[A-Za-z][A-Za-z0-9_.-]{0,15}`).Draw(t, "owner")
		// A repository named *.git loses that suffix, which is why the
		// property claims no idempotence.
		repo := rapid.StringMatching(`[A-Za-z0-9_-][A-Za-z0-9_.-]{0,15}[A-Za-z0-9_-]`).
			Filter(func(s string) bool { return !strings.HasSuffix(s, ".git") }).Draw(t, "repo")
		suffix := rapid.SampledFrom([]string{"", ".git", "/", ".git/"}).Draw(t, "suffix")
		port := rapid.IntRange(1, 65535).Draw(t, "port")
		user := rapid.SampledFrom([]string{"", "git@", "deploy@"}).Draw(t, "user")
		forms := []string{
			fmt.Sprintf("https://%s%s/%s/%s%s", user, host, owner, repo, suffix),
			fmt.Sprintf("https://%s:%d/%s/%s%s", host, port, owner, repo, suffix),
			fmt.Sprintf("ssh://%s%s/%s/%s%s", user, host, owner, repo, suffix),
			fmt.Sprintf("ssh://git@%s:%d/%s/%s%s", host, port, owner, repo, suffix),
			fmt.Sprintf("git@%s:%s/%s%s", host, owner, repo, suffix),
			fmt.Sprintf("  git://%s/%s/%s%s  ", host, owner, repo, suffix),
		}
		remote := rapid.SampledFrom(forms).Draw(t, "remote")
		want := host + "/" + owner + "/" + repo
		if suffix == ".git/" {
			// Surrounding slashes are trimmed after the .git suffix, so a
			// .git before a trailing slash stays.
			want += ".git"
		}
		if got := NormalizeRemote(remote); got != want {
			t.Fatalf("NormalizeRemote(%q) = %q, want %q", remote, got, want)
		}
	})
}
