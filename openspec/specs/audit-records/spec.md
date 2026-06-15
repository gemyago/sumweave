# audit-records Specification

## Purpose
TBD - created by archiving change add-durable-paper-backtest-audit. Update Purpose after archive.
## Requirements
### Requirement: Durable Decision Trace Records
The system SHALL persist compact decision trace records that explain deterministic paper/backtest strategy decisions without storing unbounded market snapshots.

#### Scenario: Trace records capture deterministic decision context
- **WHEN** a deterministic paper or backtest decision is traced
- **THEN** the trace MUST persist a stable trace identity, mode, UTC decision timestamp, strategy id, strategy version, strategy artifact hash, venue, symbol, asset class, timeframe, dataset or run reference when available, input time range, compact analytics facts or references, data quality status, evaluator/rule name and version, result, stable reason codes, and compact metadata
- **AND** the trace MUST NOT store large full market snapshots, AI prompts, private venue payloads, credentials, GORM tags, table names, or UI request fields in the shared domain record

#### Scenario: Trace result values are explicit
- **WHEN** a trace is persisted
- **THEN** its result MUST be one of `no_action`, `intent_created`, `blocked_before_intent`, or `error`, or a smaller documented subset
- **AND** unsupported result values MUST be rejected with a validation error

#### Scenario: No-action persistence is bounded
- **WHEN** a strategy tick produces no action
- **THEN** the system MUST either persist a compact summary trace or document no-action persistence as deferred
- **AND** it MUST NOT persist full per-candle market snapshots for every no-action tick

### Requirement: Durable Order Intent Records
The system SHALL persist order intent records that represent explicit paper/backtest execution intent derived from strategy output before governor evaluation.

#### Scenario: Every order intent references one trace
- **WHEN** an order intent is persisted
- **THEN** it MUST include a stable intent identity and exactly one persisted trace reference
- **AND** it MUST reject missing, empty, or unknown trace references

#### Scenario: Order intent captures requested execution semantics
- **WHEN** a paper/backtest intent is created
- **THEN** it MUST persist strategy id, strategy version, strategy artifact hash, mode, venue, instrument, action kind or side, order type, requested quantity or requested notional, requested limit price for limit orders, reduce-only flag when modeled, source rule or reason code, candidate action reference when available, UTC creation timestamp, status, and compact metadata
- **AND** initial order type support MUST be limited to `limit` unless a later change explicitly adds another order type

#### Scenario: Order intent status transitions are stable
- **WHEN** an order intent moves through the deterministic path
- **THEN** its status MUST use stable values from `created`, `sent_to_governor`, `approved`, `rejected`, `blocked`, and `execution_created`, or a smaller documented subset
- **AND** invalid transitions MUST be rejected without creating duplicate intent records

### Requirement: Audit Query And Linkage
The system SHALL support deterministic audit lookup and linkage across trace, intent, governor, and execution records.

#### Scenario: Trace queries are deterministic
- **WHEN** callers query traces by strategy, instrument, mode, and time range
- **THEN** the system MUST return matching trace records in deterministic UTC timestamp order with a stable trace id tie-breaker

#### Scenario: Governor and execution references are attached when available
- **WHEN** governor decisions or execution command/order/fill records are created from an order intent
- **THEN** the trace and intent records MUST retain references sufficient to navigate from strategy decision to governor outcome and execution result
- **AND** absent references MUST mean the downstream step has not happened or failed, not that the link was silently lost

#### Scenario: SQLite audit store migrates explicit schema
- **WHEN** the audit store is opened against SQLite and migrated
- **THEN** the schema MUST use explicit table names, explicit column names, uniqueness constraints for stable trace and intent identities, UTC timestamp columns, and bounded metadata columns suitable for local deterministic tests

