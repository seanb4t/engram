No external API integration: this phase only adds eight fields to engram's OWN `EngramService` protobuf schema and wires them through `memoryToProto`. No third-party service is consumed.

Why the detector fired anyway: the "Connect" and "MCP" terms it matched are this
project's own two read/write lanes, not a third-party service, SDK, or endpoint being
consumed. `go.mod`, `go.sum`, `ui/package.json` and `ui/package-lock.json` are all
untouched across this phase (`059807ab..HEAD`), and `05-RESEARCH.md` § Package
Legitimacy Audit records that no new external Go or JS dependency is added — all work
is on already-vendored `google.golang.org/protobuf`, `connectrpc.com/connect`, and
`buf`.
