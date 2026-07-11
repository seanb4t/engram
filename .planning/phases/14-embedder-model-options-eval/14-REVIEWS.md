---
phase: 14
reviewers: [codex, antigravity]
reviewed_at: 2026-07-11T15:28:21Z
plans_reviewed: [14-01-PLAN.md, 14-02-PLAN.md, 14-03-PLAN.md]
---

# Cross-AI Plan Review — Phase 14

> Reviewers: **Codex** (codex-cli 0.144.1) and **Antigravity** (agy 1.1.1). Both are prompt-fed, source-grounding reviewers running in the git working tree (not diff-only), so both verdicts are weighted as grounded plan reviews.

---

## Codex Review

# Cross-AI Plan Review — Phase 14

## Overall assessment

The plans are well researched and mostly align with the production embedding path. In particular, they correctly replace Gemini `task_type` params with query/document instruction templates, reuse the existing embedder builder, preserve the #261 hard rank gate, and avoid introducing provider-specific code.

However, one central integration gap prevents the phase from meeting its stated success criteria as written: `task eval:retrieval` currently runs only `TestRetrievalEval`, so the proposed `TestEmbedAsymmetryDiffer` would never run through the documented task target. The plans also do not assert the configured embedding dimension in that test, and the evidence gate can accept text containing failure results. These should be corrected before execution.

Overall phase risk: **HIGH until the eval-target and evidence-gate issues are fixed; MEDIUM afterward.**

---

# Plan 14-01 — Gemini differ-case eval fixture

## Summary

The test design uses the right production seams and correctly targets the post-research Gemini mechanism: instruction-prefixed query and document strings rather than JSON `task_type` parameters. The default test suite remains cheap. The plan is incomplete, however, because it does not connect the new test to `task eval:retrieval`, does not validate vector dimensions, and understates the Qdrant startup behavior of the package-level `TestMain`.

## Strengths

- The proposed production-path reuse is sound. `StoreAndEmbedderFromEnvNoEnsure` loads validated environment configuration and returns the actual `*embed.Client` used by the application ([internal/server/tools.go:145](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:145), [internal/server/tools.go:154](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:154)). `embedderFromConfig` wires both instruction fields and both parameter maps into that client ([internal/server/tools.go:339](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:339), [internal/server/tools.go:353](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:353)).

- The Gemini instruction recipe will exercise the intended text path. A query instruction containing `{query}` is substituted literally ([internal/embed/embed.go:187](/Volumes/Code/github.com/seanb4t/engram/internal/embed/embed.go:187), [internal/embed/embed.go:196](/Volumes/Code/github.com/seanb4t/engram/internal/embed/embed.go:196)); the document equivalent substitutes `{document}` ([internal/embed/embed.go:147](/Volumes/Code/github.com/seanb4t/engram/internal/embed/embed.go:147), [internal/embed/embed.go:155](/Volumes/Code/github.com/seanb4t/engram/internal/embed/embed.go:155)). This avoids the `queryParams`/`documentParams` path entirely.

- The base URL requires no new implementation. `/v1beta/openai` is already recognized and joined directly to `/embeddings` ([internal/embed/embed.go:115](/Volumes/Code/github.com/seanb4t/engram/internal/embed/embed.go:115), [internal/embed/embed.go:124](/Volumes/Code/github.com/seanb4t/engram/internal/embed/embed.go:124)).

- The default-suite skip claim is correct. `TestMain` checks `ENGRAM_RETRIEVAL_EVAL` before starting testcontainers ([internal/retrievaleval/retrieval_eval_test.go:230](/Volumes/Code/github.com/seanb4t/engram/internal/retrievaleval/retrieval_eval_test.go:230)), and the existing test repeats the guard defensively ([internal/retrievaleval/retrieval_eval_test.go:45](/Volumes/Code/github.com/seanb4t/engram/internal/retrievaleval/retrieval_eval_test.go:45)).

- Keeping the probe synthetic follows the established fixture convention; the current #261 corpus explicitly contains only synthetic public tooling text ([internal/retrievaleval/fixtures.go:36](/Volumes/Code/github.com/seanb4t/engram/internal/retrievaleval/fixtures.go:36)).

## Concerns

- **HIGH — The new test is not added to the official eval target.** `task eval:retrieval` uses `-run TestRetrievalEval`, which cannot match `TestEmbedAsymmetryDiffer` ([Taskfile.yaml:56](/Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml:56)). Plan 14-01 modifies only the two Go files, while later plans document `task eval:retrieval` as the Gemini differ command. Consequently, the permanent gate would exist but would not run via the phase’s promised entrypoint.

- **MEDIUM — The assertion can pass with malformed or wrong-sized vectors.** The embed client verifies only that the response has a `data` entry; it does not reject an empty or incorrectly sized embedding ([internal/embed/embed.go:257](/Volumes/Code/github.com/seanb4t/engram/internal/embed/embed.go:257)). If one side returns a 3072-element vector and the other returns an empty or differently sized vector, a simple inequality assertion passes even though the configured 3072-dimension contract is broken. `StoreAndEmbedderFromEnvNoEnsure` already returns the configured `dim`, but the plan discards it ([internal/server/tools.go:154](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:154)).

- **MEDIUM — A live differ-only run still starts Qdrant.** When `ENGRAM_RETRIEVAL_EVAL=1`, package `TestMain` starts a Qdrant testcontainer before it knows which test is selected ([internal/retrievaleval/retrieval_eval_test.go:234](/Volumes/Code/github.com/seanb4t/engram/internal/retrievaleval/retrieval_eval_test.go:234), [internal/retrievaleval/retrieval_eval_test.go:242](/Volumes/Code/github.com/seanb4t/engram/internal/retrievaleval/retrieval_eval_test.go:242)). The proposed test itself does not use Qdrant, but the command still requires Docker or waits up to three minutes for startup failure. Plan 14-03 acknowledges Docker as a prerequisite, but Plan 14-01’s wording implies the differ probe avoids that dependency.

- **LOW — Exact inequality proves configured input transformation, not retrieval benefit.** The production methods turn the same raw probe into two different request strings ([internal/embed/embed.go:155](/Volumes/Code/github.com/seanb4t/engram/internal/embed/embed.go:155), [internal/embed/embed.go:196](/Volumes/Code/github.com/seanb4t/engram/internal/embed/embed.go:196)). Unequal vectors show that the prefix affected output, which matches the stated gate, but they do not show that Gemini understood the text as a retrieval role. The evidence should describe this narrower guarantee accurately.

## Suggestions

1. Add `Taskfile.yaml` to `files_modified` and change the target to run both tests, for example:

   ```yaml
   go test ./internal/retrievaleval/ -run 'Test(RetrievalEval|EmbedAsymmetryDiffer)$' -v
   ```

   Alternatively, add separate `eval:retrieval` and `eval:embed-asymmetry` targets and document them precisely.

2. Retain `dim` from `StoreAndEmbedderFromEnvNoEnsure` and assert:

   - both vectors are non-empty;
   - `len(queryVec) == int(dim)`;
   - `len(documentVec) == int(dim)`;
   - then assert vector inequality.

3. Either accept and document Docker as a package-level prerequisite, or refactor the testcontainer lifecycle so a differ-only run does not start Qdrant.

4. Use `t.Fatal` rather than `t.Errorf` for equality so the correctness gate stops immediately and cannot be obscured by later output.

## Risk Assessment

**MEDIUM.** The core implementation mechanism is correct, but the test would not run through the promised task target and does not fully validate the dimension contract.

---

# Plan 14-02 — Model and recipe documentation

## Summary

This plan has strong scope discipline: it adds one focused guide, updates the stale Gemini guidance, and keeps Helm recipes as comments without changing defaults. The provider matrix and bidirectional guide split are sensible. The primary risks are that the advertised “copy-paste” shell examples are not shell-safe as specified, local-provider examples may remain too abstract to satisfy the requirement, and the verification checks are too weak to validate recipe completeness.

## Strengths

- Correcting the existing Gemini row is necessary. The current guide explicitly groups Google with providers using request-body asymmetry and recommends `task_type` params ([docs-site/src/content/docs/guides/embedding-instructions.md:86](/Volumes/Code/github.com/seanb4t/engram/docs-site/src/content/docs/guides/embedding-instructions.md:86), [docs-site/src/content/docs/guides/embedding-instructions.md:97](/Volumes/Code/github.com/seanb4t/engram/docs-site/src/content/docs/guides/embedding-instructions.md:97)). The plan removes precisely the stale behavior that would recreate the phase’s target regression.

- The proposed Gemini environment fields map directly to supported configuration keys ([internal/config/registry.go:30](/Volumes/Code/github.com/seanb4t/engram/internal/config/registry.go:30), [internal/config/registry.go:32](/Volumes/Code/github.com/seanb4t/engram/internal/config/registry.go:32), [internal/config/registry.go:35](/Volumes/Code/github.com/seanb4t/engram/internal/config/registry.go:35), [internal/config/registry.go:44](/Volumes/Code/github.com/seanb4t/engram/internal/config/registry.go:44)).

- The Helm recipe fields are real chart surfaces. The Deployment renders query and document instructions separately ([charts/engram/templates/memory-mcp.yaml:37](/Volumes/Code/github.com/seanb4t/engram/charts/engram/templates/memory-mcp.yaml:37), [charts/engram/templates/memory-mcp.yaml:46](/Volumes/Code/github.com/seanb4t/engram/charts/engram/templates/memory-mcp.yaml:46)), and consumes API keys via `secretKeyRef` ([charts/engram/templates/memory-mcp.yaml:49](/Volumes/Code/github.com/seanb4t/engram/charts/engram/templates/memory-mcp.yaml:49)). The plan’s no-inline-secret policy matches the chart.

- Preserving the current neutral default is low-risk. It is presently `ollama/bge-m3` at 1024 dimensions with empty query/document instructions ([charts/engram/values.yaml:21](/Volumes/Code/github.com/seanb4t/engram/charts/engram/values.yaml:21), [charts/engram/values.yaml:29](/Volumes/Code/github.com/seanb4t/engram/charts/engram/values.yaml:29), [charts/engram/values.yaml:38](/Volumes/Code/github.com/seanb4t/engram/charts/engram/values.yaml:38)).

- The reindex warning is grounded in the shipped behavior: the existing reindex guide explains that both dimension changes and same-dimension model changes require migration to a new collection ([docs-site/src/content/docs/guides/reindex.md:6](/Volumes/Code/github.com/seanb4t/engram/docs-site/src/content/docs/guides/reindex.md:6)).

## Concerns

- **MEDIUM — The proposed API-key placeholder is not copy-paste-safe shell syntax.** Plan 14-02 prescribes `ENGRAM_OPENAI_API_KEY=<your-key>` in executable env blocks. In POSIX-like shells, unquoted angle brackets are redirection tokens, so this is likely to produce a syntax error rather than set the variable. This matters because the plan explicitly labels the blocks “copy-paste.”

- **MEDIUM — Local recipes may not satisfy the concrete-pairing requirement.** The plan specifies concrete cloud models and dimensions but describes TEI/Ollama/vLLM as “operator-chosen.” The requirement calls for each supported recipe to pair base URL, model, dimension, and query instruction. The current chart at least provides one concrete local pairing—`ollama/bge-m3`, 1024, no instruction ([charts/engram/values.yaml:21](/Volumes/Code/github.com/seanb4t/engram/charts/engram/values.yaml:21), [charts/engram/values.yaml:24](/Volumes/Code/github.com/seanb4t/engram/charts/engram/values.yaml:24))—but the plan does not explicitly require similarly complete TEI and vLLM examples.

- **MEDIUM — The documented Gemini command would not run the differ test.** The plan requires the new guide to tell users to run `task eval:retrieval`, but the current target selects only `TestRetrievalEval` ([Taskfile.yaml:56](/Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml:56)). Without the Plan 14-01/Taskfile correction, the guide would publish a non-working verification recipe.

- **LOW — Helm lint cannot validate commented recipes.** `helm lint` and `helm template` ignore comments. The plan’s grep checks establish only that a few strings occur somewhere; they do not verify that every recipe includes the correct dimension, both Gemini instructions, base URL, reindex note, or secret reference. The actual rendering fields are split between `memory.embed` and `memory.openai` ([charts/engram/values.yaml:21](/Volumes/Code/github.com/seanb4t/engram/charts/engram/values.yaml:21), [charts/engram/values.yaml:39](/Volumes/Code/github.com/seanb4t/engram/charts/engram/values.yaml:39)), so malformed comment indentation or incomplete snippets could still look plausible.

- **LOW — “Every recipe cross-links” is not directly enforceable in `values.yaml`.** Helm comments cannot create a docs-site hyperlink. They can include a stable route, but the acceptance criteria should say “references the reindex guide route” for Helm and “links” for Markdown.

## Suggestions

1. Use shell-safe placeholders, such as:

   ```sh
   export ENGRAM_OPENAI_API_KEY='replace-with-your-key'
   ```

   For local providers, explicitly set it to an empty quoted value only if the gateway tolerates that.

2. Give TEI, Ollama, and vLLM each a complete concrete example, even if all use a clearly labeled example model such as BGE-M3. Include the exact server-side model identifier expected by that runtime.

3. Do not document `task eval:retrieval` for Gemini until the Taskfile target runs the differ test. Otherwise document the exact `go test -run '^TestEmbedAsymmetryDiffer$'` command.

4. Strengthen verification with exact assertions for each recipe’s model, dimension, base URL, query instruction, document instruction where applicable, and reindex reference.

5. Present Helm examples as complete commented `memory:` snippets so the relationship between `memory.embed` and `memory.openai` is unambiguous.

## Risk Assessment

**MEDIUM.** Runtime risk is low because the plan changes docs and comments, but misleading copy-paste commands or incomplete local recipes would directly undermine the operator-facing success criterion.

---

# Plan 14-03 — Live eval and committed evidence

## Summary

The wave ordering and human checkpoint are appropriate: live credentials and model-ID ambiguity genuinely require external confirmation after the fixture and recipes exist. Reusing the existing #261 assertion is also correct. The plan’s central evidence claims are nevertheless too permissive: the named task does not run the differ test, dimension success is not an enforced condition, and grep-based artifact validation can accept evidence containing failures.

## Strengths

- Wave 2 correctly depends on both code and documentation. The differ test must exist before live execution, and the confirmed model ID must be reconciled into the recipes afterward.

- The #261 evaluation reuses the existing hard gate without inventing a new threshold. The fixture has Record T plus 15 distractors and two queries ([internal/retrievaleval/fixtures.go:61](/Volumes/Code/github.com/seanb4t/engram/internal/retrievaleval/fixtures.go:61), [internal/retrievaleval/fixtures.go:68](/Volumes/Code/github.com/seanb4t/engram/internal/retrievaleval/fixtures.go:68)). The test fails when Record T is absent from the default top eight and logs the rank when it passes ([internal/retrievaleval/retrieval_eval_test.go:152](/Volumes/Code/github.com/seanb4t/engram/internal/retrievaleval/retrieval_eval_test.go:152), [internal/retrievaleval/retrieval_eval_test.go:170](/Volumes/Code/github.com/seanb4t/engram/internal/retrievaleval/retrieval_eval_test.go:170)).

- The requested evidence lines already exist in stable form: per-query rank-bar output is logged at [internal/retrievaleval/retrieval_eval_test.go:173](/Volumes/Code/github.com/seanb4t/engram/internal/retrievaleval/retrieval_eval_test.go:173), and aggregate recall/MRR at [internal/retrievaleval/retrieval_eval_test.go:196](/Volumes/Code/github.com/seanb4t/engram/internal/retrievaleval/retrieval_eval_test.go:196).

- The evidence scope is appropriately limited to public model IDs, numeric metrics, and synthetic fixture output. The embedding client sends the API key only in the Authorization header ([internal/embed/embed.go:243](/Volumes/Code/github.com/seanb4t/engram/internal/embed/embed.go:243), [internal/embed/embed.go:248](/Volumes/Code/github.com/seanb4t/engram/internal/embed/embed.go:248)); the plan correctly excludes request headers and environment dumps from the committed artifact.

- The model-ID checkpoint addresses a genuine uncertainty before finalizing public operator instructions rather than baking an unverified alias into the guide.

## Concerns

- **HIGH — The plan’s `task eval:retrieval` truth is false against the current Taskfile.** The plan states that both runs go through the task target and that a Gemini invocation produces `TestEmbedAsymmetryDiffer PASS`. The actual target runs only `-run TestRetrievalEval` ([Taskfile.yaml:56](/Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml:56)). Plan 14-03 quietly uses a different direct `go test -run TestEmbedAsymmetryDiffer` command for Gemini, contradicting its own must-have and the roadmap criterion that calls for the task target.

- **HIGH — The evidence validator accepts failure text.** Its automated check only looks for the words `recall@8`, `differ`, and `261`. An artifact containing `differ FAIL`, failed rank bars, or `recall@8=0.00` would satisfy those greps. The actual hard criterion is both #261 queries ranking within the top eight ([internal/retrievaleval/retrieval_eval_test.go:170](/Volumes/Code/github.com/seanb4t/engram/internal/retrievaleval/retrieval_eval_test.go:170)); successful evidence should therefore show two rank-bar PASS lines and `recall@8=1.00`.

- **MEDIUM — Dimension verification is observational rather than gating.** The checkpoint asks the operator to report vector length and “expects” 3072, but it does not say that any other length blocks Task 2. Meanwhile, the proposed Go test discards the configured dimension. The client itself returns whatever vector length the provider supplied ([internal/embed/embed.go:257](/Volumes/Code/github.com/seanb4t/engram/internal/embed/embed.go:257)). Thus a differ PASS with the wrong dimension could still be recorded as satisfying success criterion #1.

- **MEDIUM — Docker is unnecessarily required for the Gemini-only probe.** Package `TestMain` starts Qdrant whenever the eval env flag is enabled ([internal/retrievaleval/retrieval_eval_test.go:234](/Volumes/Code/github.com/seanb4t/engram/internal/retrievaleval/retrieval_eval_test.go:234)), even when `-run` selects only the differ test. The plan documents this prerequisite, so it is not hidden, but it creates avoidable fragility for an API-only check.

- **MEDIUM — “Record output verbatim” conflicts with safe redaction.** Human-pasted terminal output may include shell commands containing secrets, exported environment values, provider error bodies, or unrelated terminal text. The plan says both “verbatim” and “redact API keys.” Redaction must take precedence, and the artifact should use a narrow structured template rather than copying arbitrary terminal output.

- **LOW — Stating that evidence “closes #261 and #334” does not itself close the GitHub issues.** No plan task performs issue closure or ensures a later PR uses closing keywords. This is an administrative gap rather than a technical one.

## Suggestions

1. Fix `task eval:retrieval` before the checkpoint so it runs both tests, or create explicit task targets and align the roadmap, guide, validation file, and evidence commands with them.

2. Make the checkpoint fail closed:

   - confirmed model ID must return HTTP 200;
   - both returned vector lengths must equal 3072;
   - `TestEmbedAsymmetryDiffer` must exit zero with its PASS line;
   - both #261 queries must report hard-rank PASS;
   - aggregate output must report `recall@8=1.00`.

3. Strengthen the evidence verification to reject `FAIL`, require exact successful output, and require a recorded zero exit status for each command.

4. Have the differ test assert configured dimensions directly, so the committed evidence does not depend solely on manually counting curl output.

5. Use a structured evidence template containing only:

   - date;
   - provider/model;
   - vector dimensions;
   - sanitized command with `$API_KEY` placeholder;
   - exit status;
   - exact PASS and metric lines.

6. Add an issue-handoff step stating how #261 and #334 are closed after merge—PR closing keywords or an explicit issue update.

## Risk Assessment

**HIGH.** This plan is the phase’s correctness and closure gate. As written, its named command omits one of the required tests, its artifact checks can pass on failed results, and dimension correctness is not enforced.

---

# Recommended pre-execution changes

The minimum changes needed to make the phase execution-ready are:

1. Add `Taskfile.yaml` to Plan 14-01 and make the official eval entrypoint run both the differ and retrieval tests.
2. Assert non-empty vectors and exact configured dimensions in `TestEmbedAsymmetryDiffer`.
3. Make Plan 14-03 fail closed on model ID, 3072 dimensions, test exit status, two hard-rank passes, and `recall@8=1.00`.
4. Replace shell-unsafe `<your-key>` assignments with quoted placeholders.
5. Make TEI, Ollama, and vLLM examples concrete enough to pair base URL, model, dimension, and instruction as required.

No files were edited.

---

## Antigravity Review

# Cross-AI Plan Review: Phase 14 (Embedder Model Options & Eval)

I have performed a thorough review of the implementation plans (`14-01-PLAN.md`, `14-02-PLAN.md`, and `14-03-PLAN.md`) against the current codebase at `/Volumes/Code/github.com/seanb4t/engram`.

Here is the structured feedback on plan quality, completeness, and risks.

---

## 1. Summary

The plans for Phase 14 are well-structured, logically partitioned, and show an excellent understanding of the asymmetric embedding requirements for Gemini and Qwen3. The plans correctly leverage the existing evaluation harness in `internal/retrievaleval/` and the Go test structure to enforce the correctness gates without introducing unnecessary abstractions or library dependencies. 

However, there are **three critical verification bottlenecks** that will block or invalidate execution if not corrected:
1. Naming the differ test `TestEmbedAsymmetryDiffer` prevents it from being run by the existing `task eval:retrieval` task due to a name filter.
2. The differ test will fail under symmetric model configurations because it unconditionally asserts vector inequality.
3. Pre-existing markdown formatting violations in `.planning/` will immediately fail the `task lint` acceptance gates.

---

## 2. Strengths

- **Direct Mitigation of Pitfall 12 (D-04):** The differ-case test `TestEmbedAsymmetryDiffer` in [14-01-PLAN.md](file:///Volumes/Code/github.com/seanb4t/engram/.planning/phases/14-embedder-model-options-eval/14-01-PLAN.md) is highly effective. By embedding the same string via `EmbedQuery` and `Embed` and verifying they differ, it creates a robust, automated check ensuring the Gemini compatibility endpoint actually honors the text-prefix instruction (rather than silently ignoring it, which a unit test inspecting the request payload shape would miss).
- **Zero-Cost Skip Gate Consistency (D-05):** Adding `os.Getenv("ENGRAM_RETRIEVAL_EVAL") != "1"` as the first statement in the test (matching [retrieval_eval_test.go:46-48](file:///Volumes/Code/github.com/seanb4t/engram/internal/retrievaleval/retrieval_eval_test.go#L46-L48)) prevents unnecessary Docker/testcontainer instantiation or API network calls during default `go test ./...` sweeps.
- **Production-Parity Construction (D-03):** The plans correctly construct the embedder client using the exported [server.StoreAndEmbedderFromEnvNoEnsure()](file:///Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go#L154) helper, ensuring the tests run with the identical configuration parser and HTTP client wrapper as the production server.
- **Secrecy and Safety (T-14-01):** The plans adhere to security boundaries by keeping the testing probe (`differProbe`) synthetic, avoiding private data leakage to cloud APIs, and explicitly using placeholders (`<your-key>`) in Helm and docs instead of committing real API credentials.

---

## 3. Concerns

### 🔴 Concern 1: Test Failure with Symmetric Models
* **Path/Line:** `internal/retrievaleval/retrieval_eval_test.go` (to be created in `14-01-PLAN.md` Task 2)
* **Severity:** **HIGH**
* **Mechanism:** The differ test calls `em.EmbedQuery(ctx, differProbe)` and `em.Embed(ctx, differProbe)` and asserts they are not equal (`!reflect.DeepEqual(qVec, dVec)`). However, if an operator runs the evaluation suite using a *symmetric* model configuration (such as OpenAI `text-embedding-3-small` or local `bge-m3` without instructions), both query and document embeddings will be generated raw (symmetric). 
* **Impact:** The vectors will be identical, causing the differ test to fail. This breaks the suite for valid, documented symmetric configurations.

### 🟡 Concern 2: Test Exclusion from `task eval:retrieval`
* **Path/Line:** [Taskfile.yaml:59](file:///Volumes/Code/github.com/seanb4t/engram/Taskfile.yaml#L59)
* **Severity:** **MEDIUM**
* **Mechanism:** The `eval:retrieval` task command is defined as:
  ```yaml
  ENGRAM_RETRIEVAL_EVAL=1 go test ./internal/retrievaleval/ -run TestRetrievalEval -v
  ```
  Because the `-run` flag uses a strict regular expression filter, naming the new test `TestEmbedAsymmetryDiffer` (as specified in `14-01-PLAN.md` Task 2) means it will **not** match the pattern `TestRetrievalEval` and will be skipped during the verification phase.
* **Impact:** Executing `task eval:retrieval` in `14-03-PLAN.md` will fail to run the asymmetry differ assertion, bypassing success criterion #1.

### 🔴 Concern 3: Pre-existing Markdown Lint Failures
* **Path/Line:** [.rumdl.toml:17-29](file:///Volumes/Code/github.com/seanb4t/engram/.rumdl.toml#L17-L29)
* **Severity:** **HIGH**
* **Mechanism:** The `exclude` list in `.rumdl.toml` does not currently omit `.planning/`. Running `task lint:markdown` (which is part of the default `task` and `task lint` targets) fails with 584 issues in `.planning/` files.
* **Impact:** Since plans 14-01, 14-02, and 14-03 demand `task lint` or `task` exits 0, execution will immediately block on pre-existing markdown violations.

---

## 4. Suggestions

### Suggestion 1: Guard the Differ Test with a Symmetric Check
In `TestEmbedAsymmetryDiffer`, inspect the active environment configuration to check if an asymmetric instruction or custom parameters are configured. If they are empty, skip the assertion:
```go
// Skip the differ check if the active config is symmetric
if os.Getenv("ENGRAM_EMBED_QUERY_INSTRUCTION") == "" &&
   os.Getenv("ENGRAM_EMBED_DOCUMENT_INSTRUCTION") == "" &&
   os.Getenv("ENGRAM_EMBED_QUERY_PARAMS") == "" &&
   os.Getenv("ENGRAM_EMBED_DOCUMENT_PARAMS") == "" {
    t.Skip("Symmetric configuration detected; skipping query/document vector differ check.")
}
```

### Suggestion 2: Rename Test to Match the Filter
Rename the differ test from `TestEmbedAsymmetryDiffer` to `TestRetrievalEval_AsymmetryDiffer`. Because Go's `-run` performs substring/regex matching, running `task eval:retrieval` (which filters for `TestRetrievalEval`) will match and execute both `TestRetrievalEval` and `TestRetrievalEval_AsymmetryDiffer` automatically.

### Suggestion 3: Pull `REQ-lint-planning-exclude` into Wave 1
To prevent markdown lint failures from breaking the verification steps, add `".planning"` to the `exclude` list in [.rumdl.toml:17-29](file:///Volumes/Code/github.com/seanb4t/engram/.rumdl.toml#L17-L29) as a quick first-wave task:
```diff
 exclude = [
   ".git",
   ".worktrees",
+  ".planning",
   ".beads", 
```

### Suggestion 4: Provide Copy-Pasteable Curl Commands in Verification
To simplify the manual checkpoint in `14-03-PLAN.md` Task 1, write out the exact curl commands with authorization header format:
```sh
# For gemini-embedding-2:
curl -X POST https://generativelanguage.googleapis.com/v1beta/openai/embeddings \
  -H "Authorization: Bearer $ENGRAM_OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gemini-embedding-2","input":"test"}'

# For gemini-embedding-2-preview:
curl -X POST https://generativelanguage.googleapis.com/v1beta/openai/embeddings \
  -H "Authorization: Bearer $ENGRAM_OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gemini-embedding-2-preview","input":"test"}'
```

---

## 5. Risk Assessment

* **Rating: MEDIUM**
* **Justification:** The core logic, recipe variables, and architectural boundaries are extremely solid and require no changes to the production Go codebase. However, the execution flow is fragile due to the naming of the test function (which prevents its execution during `task eval:retrieval`), the symmetric model failure edge case, and the blocking markdown linter. Implementing the four suggestions above resolves these gaps completely, reducing the implementation risk to **LOW**.

---

## Consensus Summary

Both reviewers independently rated the plans architecturally sound (correct Gemini instruction-prefix mechanism, production-parity eval path, no new Go abstraction, security discipline) but flagged **verification-plumbing gaps** that would let the phase "pass" without actually exercising its correctness gate. The single most important finding was raised by **both** reviewers and is **confirmed against source**.

### ✅ Agreed Strengths (2+ reviewers)

- **Correct Gemini mechanism (Pitfall-12 mitigation).** The differ-case uses the instruction-prefix path (`ENGRAM_EMBED_QUERY_INSTRUCTION`/`ENGRAM_EMBED_DOCUMENT_INSTRUCTION`, `embed.go` EmbedQuery/Embed), never `task_type`/`*_PARAMS` — directly kills the silent no-op the phase targets.
- **Production-parity construction** via `server.StoreAndEmbedderFromEnvNoEnsure()` (`internal/server/tools.go:154`) — tests run the identical config parser + HTTP client as prod.
- **Zero-cost skip gate** on `ENGRAM_RETRIEVAL_EVAL=1`, consistent with the existing `gh261Case`/`TestMain` guard.
- **Security**: synthetic probe content, placeholder secrets, `secretKeyRef` indirection in Helm — no credential/private-data leakage.
- **Scope discipline / no new abstraction**; reuse of the existing `gh261` hard gate without inventing a new threshold.

### 🔴 Agreed Concerns (highest priority)

1. **[VERIFIED — top consensus] `task eval:retrieval` will NOT run the differ test.** `Taskfile.yaml:59` runs `go test ./internal/retrievaleval/ -run TestRetrievalEval -v`; Go's `-run` regex does not match the plan's `TestEmbedAsymmetryDiffer`, so the differ assertion is silently skipped by the entrypoint 14-02/14-03 document. Success criterion #1's automated proof would exist but never run via the promised command. Codex rated **HIGH**, Antigravity **MEDIUM** — treat as **HIGH** (it defeats the phase's headline correctness gate). *Confirmed by direct Taskfile inspection during this review.*

### 🟠 Notable single-reviewer concerns (verified or high-value)

- **[VERIFIED — Antigravity, HIGH] Full-`task` gate blocks on pre-existing `.planning/` markdown lint.** `.rumdl.toml:17` excludes `.git`/`.beads` but not `.planning`; `task lint:markdown` runs `rumdl check .` over the whole tree (`Taskfile.yaml:76`). Any acceptance criterion demanding `task`/`task lint` exit 0 will fail on pre-existing `.planning/` noise (a known systemic issue, ref REQ-lint-planning-exclude / issue #335). Fix: add `.planning` to `.rumdl.toml` exclude as a Wave-1 task, OR scope the plans' gate to `task lint:go` + `task test`. *Structural gap confirmed; exit-code failure per Antigravity's baseline run.*
- **[Antigravity, HIGH] Differ test breaks under symmetric configs.** `!reflect.DeepEqual(qVec,dVec)` fails for valid symmetric operator configs (OpenAI `text-embedding-3-small`, bare `bge-m3`), where query==document. The differ assertion must be guarded to only run when an asymmetric instruction/param is configured (skip otherwise).
- **[Codex, HIGH] Evidence validator accepts failure text.** 14-03's grep for `recall@8`/`differ`/`261` passes on `differ FAIL` or `recall@8=0.00`. The gate must require the PASS lines + `recall@8=1.00` and a zero exit status.
- **[Codex, MEDIUM] Differ assertion passes with wrong-sized/empty vectors.** `embed.go:257` only checks a `data` entry exists. Assert both vectors are non-empty and `len == dim` (3072, the value `StoreAndEmbedderFromEnvNoEnsure` returns but the plan discards), then assert inequality — and use `t.Fatal` not `t.Errorf`.
- **[Codex, MEDIUM] `<your-key>` is not shell-safe** in "copy-paste" env blocks (angle brackets are redirection tokens) → use quoted `export ENGRAM_OPENAI_API_KEY='...'`.
- **[Codex, MEDIUM] Local recipes too abstract.** TEI/Ollama/vLLM described as "operator-chosen" vs REQ-embed-model-docs' requirement to pair base URL + model + dim + query instruction concretely.
- **[Codex, MEDIUM] Evidence "verbatim" vs "redact keys" conflict** → redaction must win; use a structured evidence template, not arbitrary terminal paste.
- **[Codex, LOW] "Closes #261/#334" performs no actual GitHub closure** — add an issue-handoff/PR-closing-keyword step.

### ⚖️ Divergent Views

- **Severity of the eval-target gap:** Codex HIGH vs Antigravity MEDIUM (resolved above → treat as HIGH).
- **Fix approach for the eval-target gap:** Antigravity — *rename* the test to `TestRetrievalEval_AsymmetryDiffer` so the existing `-run TestRetrievalEval` regex matches it via substring (no Taskfile change). Codex — *broaden the Taskfile target* (`-run 'Test(RetrievalEval|EmbedAsymmetryDiffer)$'`) or add a dedicated `eval:embed-asymmetry` target, and add `Taskfile.yaml` to 14-01's `files_modified`. Either resolves it; the rename is the smaller diff, the Taskfile change is the more explicit entrypoint.

### Recommended next step

Replan incorporating this feedback:

```
/gsd-plan-phase 14 --reviews
```

The planner must, at minimum: (1) make the differ test reachable via the documented eval command; (2) scope or fix the `task`/rumdl gate for `.planning/`; (3) guard the differ assertion for symmetric configs; (4) make 14-03's evidence gate fail-closed (PASS lines + `recall@8=1.00` + exit 0 + dimension check); (5) shell-safe placeholders + concrete local recipes.
