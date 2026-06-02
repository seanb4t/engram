<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Contributing to engram

Thanks for your interest. engram is a small, focused Go service; the bar is
simple, well-tested, well-bounded code.

## Development

```sh
task           # lint + test (the default)
task build     # build ./cmd/engram → bin/engram
task test      # go test ./...
task lint      # golangci-lint + yamlfmt + actionlint + rumdl
task fmt       # gofmt + dprint + yamlfmt
task license:add   # apply Apache-2.0 SPDX headers
```

Requires Go 1.26+. Dev tools (golangci-lint, license-eye, dprint, yamlfmt,
rumdl, actionlint, goreleaser, helm) are listed in the Taskfile; install via
`task tools` or your package manager.

## Conventions

- **Commits:** [Conventional Commits](https://www.conventionalcommits.org/)
  (`type(scope): summary`). Validated in CI on PR titles via cocogitto.
- **License headers:** every Go and Markdown file carries the Apache-2.0 SPDX
  header (`task license:check` enforces it; `task license:add` applies it).
- **Formatting/linting:** `task fmt` then `task lint` must be clean before a PR.
- **CLI:** commands use [cobra](https://github.com/spf13/cobra); config is
  env-first with flag overrides (no viper).
- **Layout:** entrypoint in `cmd/engram`; logic in `internal/`; the Helm chart
  in `charts/engram`.

## Releases

Tagging `vX.Y.Z` triggers goreleaser (multi-arch image) and the OCI Helm chart
push. Versions are derived from conventional commits; there is no in-repo
changelog (GitHub Release notes are generated at release time).

## License

By contributing you agree your contributions are licensed under the
[Apache License 2.0](./LICENSE).
