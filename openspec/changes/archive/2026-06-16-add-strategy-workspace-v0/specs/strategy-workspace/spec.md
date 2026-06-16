## ADDED Requirements

### Requirement: Protected Strategy Workspace API
The backend application SHALL expose protected app-owned strategy workspace endpoints for constrained v0 strategy validation, saved version management, demo strategy discovery, and duplication.

#### Scenario: Strategy endpoints require authentication
- **WHEN** a caller without a valid authenticated identity calls any `/api/v1/strategies*` endpoint
- **THEN** the system MUST reject the request as unauthorized

#### Scenario: Strategy validation uses strict DSL path
- **WHEN** an authenticated operator validates a moving-average-crossover strategy definition
- **THEN** the backend MUST validate it through the existing strict Strategy DSL v0 and StrategyArtifact canonicalization path
- **AND** the validation response MUST include deterministic validation errors or canonical artifact preview data without saving a runnable version

#### Scenario: Strategy validation preview is canonical and complete
- **WHEN** validation succeeds for a strategy definition
- **THEN** the response MUST include the canonical schema version, strategy kind, normalized instrument, normalized timeframe, parameter summary, canonical artifact JSON/hash preview, and whether that hash already resolves to an existing artifact when knowable
- **AND** the response MUST NOT create a strategy version registry row or otherwise make the candidate runnable
- **WHEN** validation fails
- **THEN** the response MUST include deterministic field/path errors and MUST NOT include a misleading artifact hash preview

#### Scenario: Valid strategy save creates immutable version
- **WHEN** an authenticated operator saves a valid strategy definition with display metadata
- **THEN** the backend MUST create or reuse the immutable StrategyArtifact by canonical hash
- **AND** it MUST persist a product-facing strategy version containing strategy id, version, display name, status, source type, artifact hash, schema version, kind, instrument, timeframe, parameter summary, notes, and UTC creation metadata

#### Scenario: Strategy status model is explicit
- **WHEN** a human-created or demo strategy version is persisted in v0
- **THEN** its persisted status MUST be `ready` unless it was explicitly created by a migration/admin/future path as `archived`
- **AND** `draft` MUST remain a non-persisted client/API response state only
- **AND** `ready` MUST mean eligible for deterministic backtest evaluation only, not recommendation, promotion, live trading, or autonomous execution
- **AND** source type MUST remain separate from status and distinguish at least `human`, `demo`, and reserved `ai_draft`

#### Scenario: Saved strategy versions are not mutated
- **WHEN** an authenticated operator edits an existing saved strategy version
- **THEN** saving the edit MUST create a new strategy version linked to the parent version
- **AND** the existing saved version and its immutable artifact MUST remain unchanged

#### Scenario: Strategy listing exposes version rows
- **WHEN** an authenticated operator lists strategy workspace records
- **THEN** the response MUST return deterministic strategy version rows with status, source type, artifact hash, archetype, instrument, timeframe, and creation metadata
- **AND** the strategy listing MUST NOT require or synthesize a latest evaluation summary in v0; evaluation summaries MUST come from evaluation history/detail endpoints

### Requirement: Demo Strategy Versions
The system SHALL provide deterministic demo strategy versions that remain normal validated strategy records and are honest about local data availability.

#### Scenario: Demo strategies are seeded idempotently
- **WHEN** demo strategy seeding runs more than once
- **THEN** at least three demo moving-average-crossover strategy versions MUST exist without duplicate version rows
- **AND** each demo MUST be created through the same backend validation and StrategyArtifact persistence path as human-created strategies
- **AND** each demo MUST use fixed implementation-time strategy definitions selected before backend API/UI work proceeds
- **AND** each persisted demo row MUST have source type `demo` and status `ready`

#### Scenario: Demo strategies are labeled as examples
- **WHEN** an authenticated operator views demo strategy rows or details
- **THEN** the UI and API-visible metadata MUST mark them as demo/example strategies and not recommendations
- **AND** the copy MUST state when evaluation success depends on matching local historical data being present

#### Scenario: Demo strategy can be duplicated
- **WHEN** an authenticated operator duplicates a demo strategy version
- **THEN** the system MUST return a human-editable candidate/draft response linked to the demo parent
- **AND** it MUST NOT mutate the demo strategy version or its artifact

#### Scenario: Demo evaluation fails clearly when local data is absent
- **WHEN** an authenticated operator evaluates a demo-derived or demo strategy whose fixed market scope/range lacks matching local historical candles
- **THEN** the evaluation MUST be persisted as failed with a replay/data-unavailable stage reason/details
- **AND** the system MUST NOT fabricate candles, metrics, fills, or successful evidence

### Requirement: Protected Evaluation Runner API
The backend application SHALL expose protected evaluation/backtest endpoints that run deterministic durable backtests from saved strategy versions.

#### Scenario: Evaluation endpoints require authentication
- **WHEN** a caller without a valid authenticated identity calls any `/api/v1/evaluations/backtests*` endpoint
- **THEN** the system MUST reject the request as unauthorized

#### Scenario: Evaluation derives runtime inputs from artifact
- **WHEN** an authenticated operator starts an evaluation with strategy id, strategy version, time range, quantity, optional governor policy hash, and optional note
- **THEN** the backend MUST load the saved strategy version and immutable artifact
- **AND** it MUST derive instrument, timeframe, strategy kind, and parameters from the artifact rather than accepting independent UI-supplied runtime parameters

#### Scenario: Evaluation only runs ready saved versions
- **WHEN** an authenticated operator starts an evaluation for a missing, mismatched, draft, or archived strategy version
- **THEN** the backend MUST reject the request before running the durable backtest flow
- **AND** it MUST NOT create strategy decisions, order intents, governor decisions, or simulated executions for that rejected request

#### Scenario: Evaluation uses fixed safe default governor policy when omitted
- **WHEN** an evaluation request omits a governor policy hash
- **THEN** the backend MUST use the idempotently created default paper governor policy artifact with mode `paper`, allowed action kinds `long` and `short`, minimum quality `raw`, and maximum approved count `50`
- **AND** it MUST persist or return the policy hash/reference used for the run
- **AND** it MUST NOT create request-specific policy variants or bypass governor evaluation

#### Scenario: Evaluation uses deterministic durable backtest flow
- **WHEN** an evaluation request resolves to valid strategy artifact, time range, quantity, and safe governor policy
- **THEN** the backend MUST run the existing deterministic durable backtest flow in Data -> Analytics -> Strategy -> Governor -> Execution order
- **AND** completed runs MUST persist or link BacktestRun, EvaluationReport, decision traces, order intents, governor decisions/references, and simulated execution records where applicable

#### Scenario: Failed evaluation preserves debuggable status
- **WHEN** validation, replay, analytics, strategy, governor, execution, or report creation fails during evaluation
- **THEN** the run history/detail MUST expose failed status with stage-specific reason/details sufficient to debug
- **AND** downstream stages MUST NOT be silently marked successful

#### Scenario: Non-approved governor decisions do not execute
- **WHEN** governor decisions are rejected or blocked during an evaluation
- **THEN** those decisions MUST be visible in evaluation evidence
- **AND** the flow MUST NOT create simulated execution records for those non-approved decisions

### Requirement: Evaluation History And Detail
The system SHALL provide operator-facing evaluation history and detail views backed by persisted runtime evidence.

#### Scenario: Evaluation history is filterable
- **WHEN** an authenticated operator lists evaluation runs
- **THEN** the response and UI MUST show run id, strategy id/version, artifact hash, instrument, timeframe, tested range, status, evaluation decision, trade/fill count when available, blocked/rejected governor counts when available, and UTC lifecycle timestamps
- **AND** the operator MUST be able to filter by strategy and status at minimum

#### Scenario: Evaluation detail shows summary and metrics
- **WHEN** an authenticated operator opens an evaluation detail
- **THEN** the UI MUST show status, evaluation decision, tested range, dataset reference/checksum when available, strategy id/version/artifact hash, governor policy reference, and compact metrics such as fill count, max drawdown, rejected/blocked governor counts, realized PnL, or final portfolio summary only when those metrics are derivable

#### Scenario: Evaluation detail shows evidence tables
- **WHEN** an authenticated operator opens an evaluation detail with persisted evidence
- **THEN** the UI MUST render strategy decision trace rows, order intent rows, governor decision rows, and simulated execution rows with stable identifiers and reason/status fields
- **AND** missing optional evidence MUST be shown as an empty/unavailable state rather than fabricated data

### Requirement: Strategy Workspace UI
The operator UI SHALL provide protected strategy and evaluation workspace routes for the v0 human workflow.

#### Scenario: Strategy routes are protected and navigable
- **WHEN** an authenticated operator uses the app navigation
- **THEN** the nav MUST include strategy and evaluation workspace entries alongside existing protected operator routes
- **AND** unauthenticated access to those routes MUST redirect using the existing protected route behavior

#### Scenario: Strategy editor is constrained to supported v0 fields
- **WHEN** an operator creates or edits a strategy in the UI
- **THEN** the editor MUST expose only supported moving-average-crossover fields for identity, venue/symbol/asset class/active state, timeframe, fast window, slow window, and notes
- **AND** it MUST show validation status, deterministic errors, canonical schema/kind/instrument/timeframe/parameter preview, canonical artifact hash preview, and persisted artifact hash after save

#### Scenario: Evaluation UI starts from a saved strategy version
- **WHEN** an operator starts an evaluation from the strategy workspace
- **THEN** the UI MUST submit strategy id/version, time range, quantity, optional policy hash, and note only
- **AND** it MUST NOT submit independent strategy parameters that can mismatch the artifact

### Requirement: AI-Ready Safety Constraints
The strategy workspace SHALL preserve metadata and boundaries needed for later AI-assisted iteration without adding AI to the runtime path.

#### Scenario: Source metadata distinguishes future AI drafts
- **WHEN** strategy versions or evaluation runs are stored or returned
- **THEN** records MUST include source metadata sufficient to distinguish human, demo, and future AI-draft/evaluation origins

#### Scenario: Evaluation outputs are compact and retrievable
- **WHEN** a caller retrieves evaluation detail or report evidence
- **THEN** the response MUST be deterministic, bounded, and compact enough for future critique or comparison workflows
- **AND** it MUST preserve stable references to strategy artifact, dataset, governor policy, audit, intents, and simulated execution evidence

#### Scenario: No AI or live trading path is introduced
- **WHEN** an operator validates, saves, duplicates, or evaluates strategy versions in this slice
- **THEN** the system MUST NOT call AI providers, execute AI-generated code, place live orders, expose manual order controls, bypass StrategyArtifact validation, bypass governor policy, or promote a strategy autonomously
