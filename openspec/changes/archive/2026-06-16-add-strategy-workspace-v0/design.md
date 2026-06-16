## Context

Signal Foundry's product architecture is the deterministic path `Data -> Analytics -> Strategy -> Governor -> Execution`, with AI limited to research, drafting, critique, explanation, and summaries outside that path. Current runtime foundations already include strict strategy DSL v0 validation/canonicalization for moving-average crossover, immutable strategy artifacts, governor policy artifacts, durable paper/backtest orchestration, backtest/evaluation report persistence, audit traces/intents, and paper execution records. The app already owns protected `/api/v1/*` operator routes and the UI has protected `#/data`, `#/chat`, and `#/providers` routes.

This change should productize the existing runtime rather than introduce a new strategy language, policy editor, job system, live execution control, or AI runtime loop.

## Goals / Non-Goals

**Goals:**

- Provide a protected human workflow to create, validate, save, duplicate, and list constrained v0 strategy versions.
- Keep saved strategy definitions immutable by creating or reusing canonical `StrategyArtifact` rows and adding only product registry metadata around them.
- Provide at least three deterministic demo strategy versions that validate through the same backend path and are useful for seed/list/detail/duplicate/editor correctness; successful evaluation remains conditional on matching local historical data being present.
- Let an operator run a deterministic backtest/evaluation from a saved strategy version and inspect history/detail evidence from persisted runtime records.
- Preserve enough source metadata and compact evaluation output for a follow-up AI critique/iteration workflow.

**Non-Goals:**

- No live trading controls, manual order placement, autonomous promotion, or paper/live lifecycle beyond backtest evaluation.
- No arbitrary executable strategy code, code-like DSL fields, AI-generated code execution, or AI calls from the deterministic runtime path.
- No full governor policy editor, complex portfolio optimizer, multi-strategy allocator, async job queue, or multi-user permission model in this slice.
- No chart overlay is required; table-first evaluation detail is sufficient.

## Decisions

1. Keep `StrategyArtifact` as the immutable source for runnable runtime inputs.

   The workspace registry stores product metadata (`strategyId`, `version`, `displayName`, `status`, `sourceType`, notes, parent linkage, timestamps, and `artifactHash`) but never stores independently runnable parameters that can diverge from the artifact. Evaluation create requests accept stable product inputs (`strategyId`, `strategyVersion`, time range, quantity, optional policy hash/note), then the backend loads the registry row and artifact, derives instrument/timeframe/kind/parameters from canonical artifact bytes, and builds the durable flow request.

2. Define the v0 strategy status and source model narrowly.

   Persisted strategy version `status` values are `ready` and `archived`. Client/editor `draft` is a non-persisted UI/API response state only; it must not be stored as a registry row. Human saves and demo seeding create `ready` rows. Editing or duplicating an existing saved row returns a draftable candidate payload, and saving that candidate creates a new immutable `ready` row linked to the parent. There is no v0 status mutation endpoint; if an `archived` row exists by migration/admin/future path, it remains visible in detail/list metadata but is not eligible for new evaluations. `ready` means eligible for deterministic backtest evaluation only, not recommended, promoted, live, or autonomous. `sourceType` is separate origin metadata with v0 values `human`, `demo`, and reserved `ai_draft`.

3. Keep persisted draft scope minimal for v0.

   The UI can present a "draft" form for new or duplicated strategy versions, but v0 persistence starts when a valid candidate version is saved. Editing an existing saved version populates a new client-side draft and saving creates a new immutable registry version. This avoids a second mutable draft table while satisfying the non-mutation rule for saved versions. A future AI draft flow can add persisted draft records if needed.

4. Put the version registry close to strategy runtime concepts, with thin app services over it.

   `runtime/strategy` should own registry validation/persistence because strategy identity/version/artifact linkage is a product runtime concept. `apps/signal-foundry` should own HTTP DTOs, auth, demo seed timing, and orchestration. Constructors should accept consumer-defined interfaces and return concrete structs, with explicit GORM column names and UTC timestamps.

5. Make demo strategies deterministic, idempotent, and honest about data availability.

   Demo seed should call the same strategy validation/artifact creation path as human saves, use fixed strategy ids/versions/source type `demo`, create `ready` rows, and be idempotent. Demo copy must say examples/not recommendations. The exact demo market scopes, windows, and suggested evaluation ranges must be locked as implementation constants before backend API/UI work starts. They should be selected from the current canonical instrument/timeframe support and, when the target environment has guaranteed historical candles, from that inventory. If matching local data is absent, demo rows still validate and support list/detail/duplicate/editor flows, but evaluation must fail with a clear replay/data-unavailable stage error rather than fabricating data or silently passing.

6. Lock the v0 safe default governor policy before evaluation work.

   When an evaluation request omits a policy hash, the app must use one fixed idempotently created paper-only default governor policy artifact: `mode=paper`, `allowedActionKinds=["long","short"]`, `minimumQuality="raw"`, and `maximumApprovedCount=50`. Service initialization should ensure this artifact exists and is active/retrievable for paper backtests; the evaluation service may call the same ensure-default path defensively, but must not invent request-specific policies. The policy is safe because it is paper/backtest-only, bounded, immutable by hash, and still blocks/rejects decisions through the existing governor path when its limits are reached.

7. Run evaluations synchronously for v0, but persist lifecycle and failures.

   The create evaluation API may block until the durable flow finishes or fails. It must still create a stable run id, persist `pending/running/completed/failed` lifecycle through existing backtest records, and return compact status/error details. If later ranges become slow, an async job state can be introduced without changing strategy version semantics.

8. Keep the app API product-shaped and protected.

   Add app-owned routes under `/api/v1/strategies*` and `/api/v1/evaluations/backtests*` in `apps/signal-foundry/internal/api/http/v1routes.yaml` using camelCase JSON. The UI can use focused handwritten typed API wrappers, consistent with the data browser approach, unless a broader app API generation workflow is introduced separately.

9. Build evaluation detail from persisted evidence, not UI-submitted parameters.

   Detail endpoints should assemble summary, metrics, strategy/version/artifact, dataset reference, governor policy reference, strategy decision traces, order intents, governor decisions, and simulated execution records from runtime persistence or flow result records. Missing optional metrics must be omitted or labeled unavailable rather than emitted as misleading zero values.

10. Keep strategy list independent from evaluation summary aggregation in v0.

   Strategy list rows should expose strategy/version metadata, status, source, artifact hash, kind, instrument, timeframe, and creation metadata only. Latest evaluation summaries are deferred from the strategy list because evaluation evidence and metrics are introduced later in the chunk order; operators should use evaluation history/detail endpoints for run summaries. A future change can add denormalized latest-run badges after evaluation storage/query semantics are proven.

11. Preserve AI-ready metadata without AI behavior.

   Strategy version records should include source type values such as `human`, `demo`, and reserved `ai_draft`. Evaluation run records or metadata should capture request source/note when available. Compact report/detail endpoints should be deterministic and auditable so a future AI feature can critique or propose next candidates through the same validation/evaluation APIs.

## Risks / Trade-offs

- Runtime support is narrow → UI exposes only moving-average crossover fields and rejects/omits unsupported archetypes.
- Demo symbols may lack local historical data → demos still validate/list/duplicate, while evaluations fail with stage-specific replay/data errors rather than hiding the issue.
- Existing governor policy artifact support is minimal → predefine and idempotently ensure the fixed safe paper backtest default; defer policy editing.
- Full-stack slice size is high → implement in ordered backend/runtime/UI chunks, keeping each chunk shippable and test-backed.
- Synchronous runs can become slow → bound v0 ranges through UI/API validation and defer async queues until measured need.

## Migration Plan

1. Add and migrate strategy version registry tables with explicit names/columns; keep artifact tables immutable and untouched.
2. Lock and test fixed demo definitions and the safe default governor policy artifact payload, then add idempotent demo version seeding through the registry service and wire it in app startup/service initialization.
3. Add backend strategy and evaluation services/controllers/routes behind existing auth.
4. Add UI strategy/evaluation routes and update `ui-wireframe.md` in the same UI behavior chunk.
5. Rollback can leave immutable artifact/backtest/audit records intact; removing the workspace routes/pages disables the product surface without deleting historical evidence.

## Open Questions

- None for planning. Implementation must choose fixed demo market scopes/ranges in the first chunk from currently supported canonical inputs; if the local environment cannot guarantee matching candle data, the demo copy and tests must assert the explicit replay/data-unavailable failure semantics instead of a happy-path demo evaluation.
