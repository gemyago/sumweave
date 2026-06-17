# ai-strategy-assistant-tools Specification

## Purpose
TBD - created by archiving change add-ai-strategy-assistant-tools-v0. Update Purpose after archive.
## Requirements
### Requirement: Internal-Alpha Strategy Assistant Tool Pack
The backend application SHALL register a focused internal-alpha Signal Foundry strategy assistant tool pack with the existing runtime agent tool registry using product-domain service calls.

#### Scenario: Tool pack is registered through existing agent registry
- **WHEN** the backend app constructs the runtime agent for regular chat profiles
- **THEN** the app MUST register strategy assistant tools with the existing `agent.ToolsRegistry` before tool stubs are needed for agent runs
- **AND** the registered tool names MUST include `sf_data_list_candle_availability`, `sf_data_get_candles`, `sf_data_get_candle_evidence`, `sf_strategy_list_versions`, `sf_strategy_get_version`, `sf_strategy_validate_definition`, `sf_strategy_duplicate_version`, `sf_strategy_create_version`, `sf_evaluation_run_backtest`, `sf_evaluation_list_backtests`, `sf_evaluation_get_backtest_detail`, `sf_evaluation_get_backtest_report`, `sf_evaluation_get_backtest_evidence`, `sf_jobs_start_historical_data_backfill`, `sf_jobs_list`, and `sf_jobs_get`

#### Scenario: Tool pack reuses existing services
- **WHEN** a strategy assistant tool is invoked
- **THEN** it MUST call existing app/runtime data, strategy workspace, evaluation workspace, and jobs services directly rather than issuing HTTP loopback requests or raw SQL/database queries
- **AND** service validation, canonicalization, persistence, durable job orchestration, and deterministic evaluation behavior MUST remain the source of truth

#### Scenario: Alpha registration defers fine-grained permissions
- **WHEN** the tool pack is registered in v0
- **THEN** the tools MAY be globally available to regular internal-alpha chat profiles
- **AND** profile-level tool filtering enforcement and operator confirmation gates MUST remain deferred follow-ups rather than blockers for this change

#### Scenario: Tool results expose safe error and truncation metadata
- **WHEN** a strategy assistant tool returns a recoverable validation, not-found, conflict, data-unavailable, not-ready, unsaved-version, missing-artifact, job-failed, or truncation outcome
- **THEN** the result MUST include a machine-actionable error object with stable `code` and safe `message` fields, plus field/path errors or retryable/safe detail fields when useful
- **AND** the error result MUST NOT expose SQL, table names, GORM internals, stack traces, credentials, or raw service internals
- **WHEN** a tool returns a bounded partial list, report, candle, evidence view, or job list
- **THEN** the result MUST include deterministic truncation metadata such as `isTruncated`, `limit`, `returned`, optional total/cursor/range continuation data, and a concise next-step hint

### Requirement: Strategy Assistant Data Tools
The strategy assistant tool pack SHALL expose bounded read-only data browsing tools for normalized candles and candle-linked evidence.

#### Scenario: Assistant lists candle availability
- **WHEN** the assistant calls `sf_data_list_candle_availability` with optional exact venue, symbol, and asset class filters plus bounded pagination inputs
- **THEN** the tool MUST return compact normalized candle availability entries containing venue, symbol, asset class, timeframe summaries, counts, UTC start/end bounds, and default selection data when available
- **AND** empty availability MUST be represented as an empty collection without inventing symbols, ranges, or ingestion actions

#### Scenario: Assistant reads bounded normalized candles
- **WHEN** the assistant calls `sf_data_get_candles` with venue, symbol, asset class, timeframe, and an explicit half-open UTC range
- **THEN** the tool MUST return persisted normalized candles in deterministic ascending order with chart/summary-friendly OHLC, volume, quality, stable identity, provenance source, and provenance identity fields
- **AND** the tool MUST enforce a deterministic output cap or truncation indicator and MUST NOT synthesize, interpolate, backfill, or mutate candles

#### Scenario: Assistant reads candle-linked evidence metadata
- **WHEN** the assistant calls `sf_data_get_candle_evidence` with the selected candle scope and required provenance source/identity
- **THEN** the tool MUST return bounded raw payload evidence metadata linked to that exact candle provenance
- **AND** missing links MUST be returned as an empty collection, and raw response body bytes or raw SQL/database access MUST NOT be exposed

### Requirement: Strategy Assistant Strategy Tools
The strategy assistant tool pack SHALL expose constrained strategy version discovery, validation, duplication, and alpha save tools backed by the existing strategy workspace behavior.

#### Scenario: Assistant lists and reads strategy versions
- **WHEN** the assistant calls `sf_strategy_list_versions`
- **THEN** the tool MUST return compact saved strategy version metadata including strategy id, version, display name, status, source type/label, artifact hash, schema version, kind, instrument, timeframe, parameter summary, parent linkage, notes, and UTC creation/update metadata
- **AND** the list result MUST NOT require or synthesize latest evaluation aggregation
- **WHEN** the assistant calls `sf_strategy_get_version` for an existing saved version
- **THEN** the tool MUST return the saved version metadata plus its constrained Strategy DSL v0 definition

#### Scenario: Assistant validates strategy definitions without saving
- **WHEN** the assistant calls `sf_strategy_validate_definition` with a Strategy DSL v0 definition
- **THEN** the tool MUST use the same strict Strategy DSL v0 and StrategyArtifact canonicalization path as the protected strategy workspace API
- **AND** successful validation MUST return canonical preview/hash data while failed validation returns deterministic field/path errors
- **AND** validation MUST NOT create a saved strategy version or runnable artifact reference beyond any existing read-only artifact lookup behavior

#### Scenario: Assistant duplicates and saves versions through workspace path
- **WHEN** the assistant calls `sf_strategy_duplicate_version` for an existing saved version
- **THEN** the tool MUST return an editable candidate linked to the parent without mutating the source version or artifact
- **WHEN** the assistant calls `sf_strategy_create_version` with display metadata and a valid definition
- **THEN** the tool MUST save an immutable strategy version through the existing workspace save path and strict artifact validation/canonicalization
- **AND** the tool MUST NOT accept arbitrary executable strategy code, code-like DSL fields, or any validation bypass

### Requirement: Strategy Assistant Evaluation Tools
The strategy assistant tool pack SHALL expose deterministic backtest/evaluation tools that operate only on saved ready strategy versions and existing bounded evidence views.

#### Scenario: Assistant runs deterministic backtest evaluation
- **WHEN** the assistant calls `sf_evaluation_run_backtest` with strategy id, strategy version, half-open UTC start/end, positive quantity, optional governor policy hash, and optional note
- **THEN** the backend MUST load the saved ready strategy version and immutable artifact, derive instrument/timeframe/kind/parameters from the artifact, apply the requested or default governor policy, and run the existing deterministic backtest/evaluation service
- **AND** the tool MUST NOT accept independent runtime strategy parameters, bypass governor policy, place live orders, or promote a strategy to paper/live trading

#### Scenario: Evaluation rejects non-runnable strategy versions
- **WHEN** `sf_evaluation_run_backtest` references a draft candidate, unsaved definition, saved non-ready strategy version, or saved version whose immutable artifact is missing or cannot be loaded
- **THEN** the tool MUST reject the request with a stable structured error result
- **AND** it MUST NOT create, persist, or execute an evaluation run for that request

#### Scenario: Missing local data is reported honestly
- **WHEN** an evaluation run cannot replay matching local historical candles or otherwise fails during a deterministic stage
- **THEN** the tool MUST return or reference a persisted failed evaluation status with stage-specific reason/details
- **AND** it MUST NOT fabricate candles, metrics, fills, evidence, or successful report content

#### Scenario: Assistant reads evaluation history, reports, and evidence
- **WHEN** the assistant calls `sf_evaluation_list_backtests`, `sf_evaluation_get_backtest_detail`, `sf_evaluation_get_backtest_report`, or `sf_evaluation_get_backtest_evidence`
- **THEN** the tools MUST return deterministic bounded views backed by persisted run/report/evidence records, including stable strategy/artifact/dataset/policy references and compact metrics only when derivable
- **AND** missing optional evidence MUST be explicit as empty/unavailable rather than fabricated

### Requirement: Strategy Assistant Profile And Skills
The system SHALL provide reusable alpha profile guidance and workflow skills for operating the strategy assistant through chat.

#### Scenario: Strategy assistant profile guidance is available
- **WHEN** an operator wants to use the strategy assistant workflow
- **THEN** the system MUST provide either an idempotently seeded `strategy-assistant` regular profile or a documented profile payload with instructions for data discovery, historical ingestion job orchestration when data is missing, strict definition validation/save, deterministic evaluation, evidence critique, and safe next-step summarization
- **AND** the guidance MUST tell the assistant not to claim live-trading readiness, bypass the DSL/governor/evaluation path, place orders, repeatedly start duplicate jobs, or expose raw SQL/database access

#### Scenario: Strategy workflow skills are discoverable
- **WHEN** skills support is enabled for the alpha flow and the assistant calls `skills_list`
- **THEN** the available skills MUST include concise workflow skills for historical data jobs, strategy research loop, backtest critique, and strategy iteration
- **AND** `skills_read` for those skills MUST return operational instructions covering data discovery, missing-data backfill jobs, validation, evaluation, evidence summary, and safety boundaries

#### Scenario: Existing chat UI can drive the alpha flow
- **WHEN** an authenticated operator uses the existing chat route with the strategy assistant profile
- **THEN** the operator MUST be able to observe tool calls/results through the chat stream or existing debug view and receive stable job, strategy, evaluation, or route references for follow-up
- **AND** no new complex AI workflow UI is required for v0

#### Scenario: Default alpha smoke path is executable
- **WHEN** maintainers run the default strategy assistant acceptance/smoke coverage for this change in the local repository setup
- **THEN** an automated test or executable script MUST exercise data availability, bounded candle reads or explicit missing-data detection, historical backfill job start/list/get, polling to terminal status, post-job data inspection, evaluation run or explicit data-unavailable failure, report/evidence reads, and workflow skill discovery without requiring manual UI clicks as the primary gate
- **AND** any manual runbook MUST be a clearly labeled conditional fallback for coverage that cannot be automated in this slice

### Requirement: Strategy Assistant Safety Boundaries
The strategy assistant tool pack SHALL remain outside live trading and raw database access boundaries.

#### Scenario: No live trading or raw database tool is exposed
- **WHEN** the strategy assistant tool pack is registered
- **THEN** it MUST NOT include tools for live trading, manual order placement, wallet operations, exchange credential access, private venue trading endpoints, autonomous promotion, arbitrary executable strategy code, or raw SQL/database access

#### Scenario: Deterministic path remains authoritative
- **WHEN** the assistant drafts, saves, evaluates, reads, critiques, or summarizes strategy work
- **THEN** strategy saving MUST still pass strict StrategyArtifact validation/canonicalization, evaluation MUST still derive runtime inputs from saved artifacts and use governor policy, and AI output MUST remain advisory research/summarization outside the critical execution path

### Requirement: Strategy Assistant Historical Data Job Tools
The strategy assistant tool pack SHALL expose explicit durable job tools for historical raw candle backfill orchestration.

#### Scenario: Assistant starts historical backfill job explicitly
- **WHEN** the assistant calls `sf_jobs_start_historical_data_backfill` with valid historical raw candle backfill input
- **THEN** the tool MUST create a durable `historical_raw_candle_backfill` job through the app jobs service, set requested-by source `agent`, preserve agent session/run metadata when available, return job id/status/input summary, and tell the assistant to poll `sf_jobs_get` before running evaluation

#### Scenario: Assistant lists jobs before creating duplicates
- **WHEN** the assistant calls `sf_jobs_list` with status, job type, source, or pagination filters
- **THEN** the tool MUST return a bounded deterministic job list suitable for finding already queued/running matching historical backfill jobs
- **AND** the tool guidance MUST instruct the assistant not to repeatedly start duplicate jobs when a matching queued or running job already exists

#### Scenario: Assistant inspects job detail before evaluation
- **WHEN** the assistant calls `sf_jobs_get` for a job id
- **THEN** the tool MUST return bounded job detail including status, input, requester metadata, timestamps, worker/attempt metadata, result report, missing interval preview, raw payload count when present, and safe error fields when failed
- **AND** the assistant MUST treat `succeeded` as the only terminal status that permits re-checking availability and proceeding to synchronous evaluation

#### Scenario: Job tools preserve safety boundaries
- **WHEN** the strategy assistant job tools are registered or invoked
- **THEN** they MUST NOT expose real-money execution, continuous ingestion, scheduling, cancellation unless separately implemented safely, raw SQL/database access, arbitrary shell access, private venue behavior, or a convenience `sf_data_ensure_candles` tool

