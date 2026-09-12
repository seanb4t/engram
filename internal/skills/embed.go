// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package skills embeds the curation skills' canonical content — vendored
// from skill/engram/skills/ into internal/skills/data/ by `task
// skills:vendor` (D-01) — and installs it into a runtime's native skill
// destination or, where none exists, an AGENTS.md fallback block. It is
// the "install" half of D-05's declare/install split: internal/setup
// DECLARES a SkillTarget (destination + format) in each runtime's own
// Plan(); this package owns the embed, the structural inventory walk, and
// the actual write. internal/setup can never import this package
// (leafpurity_test.go); this package can never import internal/setup
// (importgate_test.go) — cmd/engram is the only composer.
package skills

import "embed"

// all: is required — a bare `//go:embed data` excludes files and
// directories whose names begin with "_" or "." with no build error,
// which would silently drop such content from every released binary
// (D-04). See internal/webauth/static.go for this repo's shipped
// precedent of the same directive, and
// internal/webauth/static_test.go:38 for the regression class this
// prefix exists to prevent — that class was hit once already in this
// exact codebase (GH #106).
//
//go:embed all:data
var skillsFS embed.FS
