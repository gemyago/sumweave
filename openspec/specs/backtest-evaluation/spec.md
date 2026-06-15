# backtest-evaluation Specification

## Purpose
TBD - created by archiving change add-durable-paper-backtest-audit. Update Purpose after archive.
## Requirements
### Requirement: Dataset Reference For Reproducible Backtests
The system SHALL persist a compact dataset reference record for backtest and evaluation records without storing large raw datasets in relational rows; this change does not introduce a separate data capture object.

#### Scenario: Dataset reference identifies replay inputs
- **WHEN** a backtest dataset reference record is created
- **THEN** it MUST include a stable dataset reference identity, entity types used, instrument or instruments, timeframe or timeframes, half-open time range, source data hashes or replay identity range/checksum, UTC creation time, and compact metadata for assumptions

#### Scenario: Dataset reference avoids mutable floating queries
- **WHEN** a backtest run references data
- **THEN** it MUST reference a persisted dataset reference identity or deterministic replay checksum rather than only an unqualified mutable query

### Requirement: Backtest Run Lifecycle
The system SHALL persist minimal backtest run records that capture reproducible strategy, dataset, policy, simulator, metric, and failure context.

#### Scenario: Backtest run is created with immutable references
- **WHEN** a backtest run is created
- **THEN** it MUST persist backtest id, strategy id and version, strategy artifact hash, dataset reference, governor policy id/version/hash when governor is included, mode `backtest`, tested market range, fee model id or explicit fee assumptions, slippage model id or explicit slippage assumptions, execution simulator version, status, UTC created/updated timestamps, and versioned metrics JSON when available
- **AND** completed run records MUST preserve exact strategy artifact and dataset references

#### Scenario: Backtest status transitions are explicit
- **WHEN** a backtest run moves through its lifecycle
- **THEN** status MUST transition through supported values from `pending`, `running`, `completed`, and `failed`, or a smaller documented subset
- **AND** failed runs MUST preserve nullable error/failure details sufficient to debug

#### Scenario: Backtest query behavior is deterministic
- **WHEN** callers query backtest runs by strategy, dataset, status, or time
- **THEN** the system MUST return matching runs in deterministic timestamp order with stable backtest id tie-breakers

### Requirement: Evaluation Report Scaffold
The system SHALL persist compact evaluation reports that reference backtest evidence and strategy artifacts.

#### Scenario: Evaluation report references backtest and strategy evidence
- **WHEN** an evaluation report is created
- **THEN** it MUST persist evaluation id, strategy id/version/hash, backtest id, dataset id/reference, decision, summary metrics, stable failure reasons, UTC creation time, and optional notes
- **AND** it MUST NOT store large report artifacts directly in relational rows

#### Scenario: Evaluation decision values are stable
- **WHEN** an evaluation report records a decision
- **THEN** the decision MUST be one of `promote_to_paper_candidate`, `reject`, `needs_review`, or a smaller documented subset
- **AND** unsupported decision values MUST be rejected with a validation error

#### Scenario: Metrics are compact and honest
- **WHEN** summary metrics are recorded
- **THEN** they MUST use a schema/version field and may include trade count, win/loss count when positions/fills are available, gross/net PnL when fees/slippage are modeled, max drawdown when portfolio snapshots are available, blocked/rejected governor decision count, and data quality warning count
- **AND** metrics that are not implemented or not derivable MUST be omitted rather than emitted as misleading zero values

#### Scenario: SQLite backtest store migrates explicit schema
- **WHEN** the backtest/evaluation store is opened against SQLite and migrated
- **THEN** the schema MUST support dataset references, backtest runs, evaluation reports, lifecycle updates, failure recording, and deterministic queries using explicit table names, explicit column names, uniqueness constraints, and UTC timestamp fields

