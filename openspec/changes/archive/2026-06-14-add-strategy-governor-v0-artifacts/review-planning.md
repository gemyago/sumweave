# Planning Review

## Round 1

- Scope: `add-strategy-governor-v0-artifacts`
- Trigger: planning artifacts created from the fetched Notion requirements.
- Verdict: changes requested.
- Findings:
  1. `specs/strategy-dsl/spec.md` and `tasks.md` mix artifact-level canonical JSON/hash and versioned artifact metadata into chunk 1. Move those assertions into `specs/strategy-artifacts/spec.md` and task 2.1 so chunk 1 stays limited to typed DSL validation/canonicalization plus `strategy.EvaluateRequest` mapping.
  2. `specs/strategy-dsl/spec.md` is missing an explicit invalid instrument/timeframe rejection scenario. Add it so the validation cases already listed in task 1.1 are covered by the spec.
  3. `specs/governor-policy-artifacts/spec.md` and task 3.1 are missing an explicit set-normalization case for `allowedActionKinds`. Add a scenario/test that equivalent payloads with reordered action kinds produce identical canonical JSON bytes and hashes.
- Decision: not ready for implementation until the above revisions are made; after cleanup, keep the chunk order `1 -> 2 -> 3 -> 4`.

## Round 2

- Scope: `add-strategy-governor-v0-artifacts`
- Trigger: re-review of revised planning artifacts after Round 1 findings.
- Verdict: approved.
- Findings:
  1. Resolved: `specs/strategy-dsl/spec.md` and `tasks.md` now keep artifact metadata, canonical JSON bytes, and hash assertions in `specs/strategy-artifacts/spec.md` and task 2.x instead of the DSL chunk.
  2. Resolved: `specs/strategy-dsl/spec.md` now explicitly requires rejection of invalid instrument and timeframe values.
  3. Resolved: `specs/governor-policy-artifacts/spec.md` and task 3.1 now require `allowedActionKinds` reorder normalization to identical canonical bytes and hashes.
  4. Resolved: `tasks.md` now reflects strict sequential chunking with correct ownership split across DSL, strategy artifacts, governor policy validation, and governor policy storage.
- Decision: ready for implementation; execute in order `1 -> 2 -> 3 -> 4` and keep artifact-scope assertions in the artifact chunks.
