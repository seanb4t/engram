No external API integration: this phase only adds eight fields to engram's OWN `EngramService` protobuf schema and wires them through `memoryToProto`. No third-party service is consumed.

Why the detector fired anyway: the "Connect" and "MCP" terms it matched are this
project's own two read/write lanes, not a third-party service, SDK, or endpoint being
consumed. `ui/package.json` and `ui/package-lock.json` are untouched across this phase,
and no third-party *service* is called by any code path this phase adds.

Corrected 2026-08-16: this paragraph previously also claimed `go.mod` and `go.sum` were
untouched across `059807ab..HEAD`, citing `05-RESEARCH.md` § Package Legitimacy Audit.
That was written before the gap-closure plan 05-04, which adds `github.com/chromedp/chromedp`
`v0.16.0` plus six transitive modules as a TEST-ONLY dependency of `internal/e2e`
(absent from `go list -deps ./cmd/engram`, so it is not linked into the shipped binary).
The dependency is verified in `05-SECURITY.md` under `T-05-SC`, whose disposition moved
from `accept` to `mitigate` for the same reason. The remaining production work is still
on already-vendored `google.golang.org/protobuf`, `connectrpc.com/connect`, and `buf`.
