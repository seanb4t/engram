# API Coverage — v0.12.x Phase 1

No external API integration: the detector fired on the words "Connect lane" and "MCP lane", which are
engram's own two in-process transports, not a third-party service — this phase composes engram's
existing verifier chain, adds one `ENGRAM_` config key, and edits its own ConnectRPC interceptors.

Confirmed by re-reading the phase scope rather than by preference:

- `01-RESEARCH.md` § Standard Stack — "Zero new dependencies… no `go get` needed for this phase";
  every listed library (`connectrpc.com/connect`, `modelcontextprotocol/go-sdk`, `coreos/go-oidc`)
  is already in `go.mod` and none gains a new call surface here.
- `01-RESEARCH.md` § Package Legitimacy Audit — "Not applicable. This phase installs zero new
  packages."
- `01-RESEARCH.md` § Environment Availability — "no new external service, tool, or runtime
  dependency beyond what's already required to build and test this repository."
- The go-sdk's `RequireBearerToken`/`verify()` behavior is **reimplemented**, not newly integrated,
  precisely because `verify()` is unexported and offers no extraction point.

Every plan carries a `git diff --exit-code go.mod go.sum` acceptance criterion, so an accidental
dependency addition fails a gate instead of passing silently.
