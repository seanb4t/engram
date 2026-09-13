# Red-Evidence Manifest — milestone 2026-08-23.01

Retired target-test mappings for the red-evidence patches archived under this
directory. Each patch is a reversible mutation that was proven to make its named
test FAIL; `internal/store`'s `TestRedEvidencePatchesAreLive` applied them on every
run while this milestone was open.

They are **no longer executed**. The harness is scoped to the open milestone only
(see the SCOPE note on `redEvidenceDirs`), mirroring `internal/keylinks`'
`TestActiveMilestoneKeyLinksSatisfiable` and its D-04 rationale: a patch asserts
that mutating *today's* source makes a test fail, so re-applying a shipped
milestone's patches at HEAD reds on any legitimate later refactor — a red that is
not a defect. The mapping is recorded here so the archive stays self-describing
and a historical patch can still be re-run by hand against the commit it was
authored at.

## 01-version-homebrew-distribution

1 patch.

| Patch | Target test proven RED |
|---|---|
| `version-json-key.patch` | `TestVersionJSONLane` |

The phase's contract is that `engram version --output json` emits a machine-readable
payload the cask's install gate parses by the `version` key; renaming that key is the
exact regression that would make a cask install pass its own gate against a payload no
consumer can read.

**Total:** 1 patch across 1 phase.
