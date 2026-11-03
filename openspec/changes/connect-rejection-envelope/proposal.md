# Proposal

## Why

Originating issues: #942, a follow-up to #923.

Issue #942 asks for lane-parity cases that derive from the constraints in the proto and the surface rules.
The work found a defect that #942 did not name. The Connect validation interceptor rejects a request
before the shared core runs, and its error has no `field=`/`hint=` envelope. An empty `content`
returns `validation error: content: must be at least 1 characters` on Connect and
`field=content hint=required` on MCP. docs-site `reference/errors.md` promises the same envelope on
both lanes.

`TestModelLaneParity` does not see this gap, because it calls the Connect handlers directly and skips
the interceptor. So a test derived from the proto cannot pass until the interceptor gives the
envelope. The fix and the derived test ship together.

## What Changes

- **BREAKING (Connect error text and codes).** The Connect interceptor turns each validation failure
  into the shared envelope. The `validation error: ...` text goes away. A length or count failure
  moves from `CodeInvalidArgument` to `CodeOutOfRange`, and a window ordering failure moves to
  `CodeFailedPrecondition`, as the class table in `reference/errors.md` already says. The CLI exit code
  does not change, because all three codes map to the same exit.
- **Proto constraints change so that the core answers where it has the better hint.**
  - `category` on store and schedule, and `scope` on store discovery, skip their rule on an empty
    value. The core then answers `hint=required`.
  - The schedule `category` allowlist gains `discovery`. The core then answers
    `hint=not_applicable`, as the declared rule says.
  - The schedule window CEL rule splits in two, one for each declared surface rule. A CEL rule is a
    cross-field expression in the proto.
- **Derived lane-parity cases.** `TestModelLaneParity` gains cases that it builds when it runs, from
  the proto constraints and the surface rules. These cases use the real Connect handler chain. The
  chain includes the interceptor, so this path gets test coverage.

## Out of Scope

- The remaining CI waste items in #923. That issue says they ship as their own PRs.
- #924 (mutation testing) and the area gap issues #932 to #941.
- #944 (MCP `supersede_memory` with an empty array). It is a gap in the same contract, but on the MCP
  lane, and it needs its own decision about the input schema.
- Tightening `category` on MCP. MCP accepts any non-empty category, and live records use `handoff`.
  This difference stays and is now stated in the spec.
- The memory size bounds. They come from configuration, so `TestModelWriteBounds` keeps its
  hand-written generators.
- Generated code or a generated test tree. The test reads the descriptors when it runs.

## Capabilities

### New Capabilities

- `connect-rejection-envelope`: What a Connect caller receives when a request fails a declared
  constraint: the field, the hint and the code.

### Modified Capabilities

- `test-practice`: Adds the requirement that lane-parity cases derive from the declared constraints.

## Impact

- **MCP tool schemas:** deliberately unchanged. The rejections on this lane stay as they are.
- **Connect proto:** constraint options change on `StoreMemoryRequest`, `ScheduleMemoryRequest` and
  `StoreDiscoveryRequest`. No field changes. This is a compatibility decision about error text and
  codes, not about the message shapes. No client reads the old text: `rg 'validation error'` finds no
  consumer in `internal/`, `cmd/`, `ui/src/` or `skill/`.
- **Hint codes:** no new code. Connect now emits existing codes on more paths.
- **CLI:** affected only in text. Connect-backed verbs print the envelope. Exit codes do not change.
- **Console (`ui/`):** no source change. `ui/src/lib/errors/connect-error.ts` already parses the
  envelope, so these rejections now reach its hint handling. Regenerated proto clients need a
  `task ui:build`.
- **Helm chart:** deliberately unchanged. There are no new `ENGRAM_` keys and no stored payload keys.
- **docs-site:** `reference/errors.md` and `contributing/testing.md` change.
- **engram skill:** deliberately unchanged. Agents use the MCP lane.
