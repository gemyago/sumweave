# execution-layer Specification

## Purpose
TBD - created by archiving change governor-and-execution-layers. Update Purpose after archive.
## Requirements
### Requirement: Canonical Execution Domain
The system SHALL define canonical cross-slice execution command, order, fill, and reconciliation records that are reusable after governor approval and independent from persistence, venue payloads, AI, and HTTP API details.

#### Scenario: Execution records do not expose implementation metadata
- **WHEN** deterministic runtime code consumes execution records
- **THEN** the shared domain records MUST expose approved governor decision source, command identity, order identity, fill identity, status, quantity, price where applicable, and UTC event times without GORM tags, table names, vendor payloads, AI prompt content, or HTTP request fields

#### Scenario: Execution command identifies approved source
- **WHEN** an execution command is created
- **THEN** it MUST retain the approved governor decision and the original strategy candidate action that caused the command

#### Scenario: Execution quantities are explicit
- **WHEN** execution turns an approved decision into a command or order
- **THEN** it MUST use an explicit positive quantity supplied to the execution layer and MUST NOT infer order size from the strategy candidate action alone

### Requirement: Approval-Only Execution Admission
The system SHALL create execution commands only from governor decisions whose status is approved.

#### Scenario: Approved decision creates command
- **WHEN** execution receives an approved governor decision with a positive quantity and valid UTC request time
- **THEN** it MUST create a deterministic execution command linked to that approved decision

#### Scenario: Rejected decision is refused
- **WHEN** execution receives a rejected governor decision
- **THEN** it MUST reject command creation with a validation error and MUST NOT return a command

#### Scenario: Blocked decision is refused
- **WHEN** execution receives a blocked governor decision
- **THEN** it MUST reject command creation with a validation error and MUST NOT return a command

#### Scenario: Missing approved decision is refused
- **WHEN** execution receives no governor decision or a malformed decision that does not retain a candidate action
- **THEN** it MUST reject command creation with a validation error and MUST NOT return a command

### Requirement: Local Order And Fill Lifecycle
The system SHALL validate local execution order and fill records from execution commands without requiring live venue submission.

#### Scenario: Command creates local order record
- **WHEN** execution records an order for a valid execution command with a venue, client order identifier, positive quantity, and valid UTC event time
- **THEN** it MUST return a canonical order record linked to the command

#### Scenario: Fill creates local fill record
- **WHEN** execution records a fill for a known order with a fill identifier, positive quantity, positive price, and valid UTC event time
- **THEN** it MUST return a canonical fill record linked to the order and command

#### Scenario: Fill cannot exceed order quantity during reconciliation
- **WHEN** execution reconciles an order whose total fill quantity is greater than the order quantity
- **THEN** it MUST return a reconciliation result that marks the order as overfilled instead of silently treating it as filled

#### Scenario: Filled quantity determines reconciliation state
- **WHEN** execution reconciles an order and total fill quantity is zero, less than the order quantity, or equal to the order quantity
- **THEN** it MUST return `open`, `partially-filled`, or `filled` reconciliation state respectively

### Requirement: Deterministic Execution Behavior
The system SHALL produce stable execution records from the same approved decisions, command parameters, order inputs, and fill inputs.

#### Scenario: Repeated command creation is stable
- **WHEN** execution creates a command from the same approved decision, quantity, and request time
- **THEN** it MUST return the same command identity and command fields

#### Scenario: Reconciliation sorts fills deterministically
- **WHEN** execution reconciles an order with multiple fills
- **THEN** it MUST process fills ordered by event time ascending and fill identity ascending for ties

#### Scenario: Execution validates UTC event times
- **WHEN** execution creates commands, orders, fills, or reconciliation records
- **THEN** it MUST normalize event times to UTC and reject missing event times with a validation error

### Requirement: Execution Service Boundary
The system SHALL expose execution behavior through a runtime service boundary that depends on canonical governor approvals and local execution records rather than on upstream workflow orchestration, AI systems, or live venue mechanics.

#### Scenario: Execution consumes governor approvals
- **WHEN** execution receives work from the deterministic path
- **THEN** it MUST consume canonical approved governor decision records or a consumer-defined interface with equivalent semantics

#### Scenario: Execution does not own upstream workflow
- **WHEN** data, analytics, strategy, and governor steps must be run together
- **THEN** execution MUST NOT become the orchestrator for those upstream slices

#### Scenario: Execution stays outside AI-assisted research
- **WHEN** execution commands or records are created for the deterministic path
- **THEN** execution MUST NOT depend on AI model calls, prompts, generated explanations, or agent session state

#### Scenario: Execution v0 avoids live venue trading
- **WHEN** the initial execution layer is introduced
- **THEN** it MUST NOT require live venue credentials, wallet signing, private trading endpoints, order submission network calls, or venue-specific trading adapters

### Requirement: On-Demand Execution V0
The system SHALL provide the initial execution layer without requiring persisted execution ledgers, backend wiring, UI screens, or new external API routes.

#### Scenario: Execution does not require execution storage
- **WHEN** the initial execution service creates commands, records orders, records fills, or reconciles local records
- **THEN** it MUST complete from provided inputs without requiring an execution table, migration, event ledger, or materialized order store

#### Scenario: External API remains unchanged
- **WHEN** the execution layer is introduced
- **THEN** the system MUST NOT require a new public HTTP endpoint for execution commands or records in order to satisfy the initial capability

#### Scenario: Backend wiring is deferred until needed
- **WHEN** the initial execution runtime slice is introduced without a current backend consumer
- **THEN** the system MUST NOT require `apps/signal-foundry` dependency injection wiring in order to satisfy the initial capability

### Requirement: Persistent Paper Execution Ledger
The system SHALL persist paper/backtest execution commands, orders, and fills after governor approval without requiring live venue submission.

#### Scenario: Approved input persists command first
- **WHEN** execution receives an approved paper or backtest governor decision or approved order intent with explicit execution inputs
- **THEN** it MUST persist one execution command before creating order or fill side effects
- **AND** the command MUST retain mode, trace id when available, intent id when available, governor decision reference, venue, instrument, action kind or side, order type, approved quantity and/or notional, limit price for limit orders, status, and UTC timestamps

#### Scenario: Command creates deterministic paper order
- **WHEN** a persisted command creates a paper order
- **THEN** execution MUST persist one deterministic order with command id, mode, venue, instrument, order type `limit`, documented time-in-force, reduce-only flag when modeled, client order id, status, approved quantity and/or notional, limit price, and UTC timestamps
- **AND** the client order id MUST be deterministic for the same command

#### Scenario: Paper fills are persisted with provenance
- **WHEN** the simulator creates a fill for a persisted paper order
- **THEN** execution MUST persist a fill linked to the order and command, with deterministic fill id, quantity, price, fee and slippage metadata, source market data reference, and UTC event timestamp

#### Scenario: Ledger retries are idempotent
- **WHEN** the same command is retried after a crash or duplicate request
- **THEN** execution MUST NOT create duplicate commands, orders, fills, or client order ids
- **AND** a new client order id MUST require a new command or a future cancel/replace workflow outside this change

#### Scenario: Unsupported modes and order types are rejected
- **WHEN** execution receives live mode, market orders, or unsupported order types
- **THEN** it MUST reject the request with a validation error and MUST NOT place live orders or call private venue APIs

### Requirement: Deterministic Limit Fill Simulator
The system SHALL simulate paper limit fills deterministically from replay market data using documented closed-candle semantics.

#### Scenario: Buy or long limit fills when candle trades through limit
- **WHEN** a paper buy/long limit order is open and a later eligible closed candle has low price less than or equal to the limit price
- **THEN** the simulator MUST create one full fill at the limit price with deterministic zero fee/slippage assumptions unless configured otherwise by explicit inputs

#### Scenario: Sell or short limit fills when candle trades through limit
- **WHEN** a paper sell/short limit order is open and a later eligible closed candle has high price greater than or equal to the limit price
- **THEN** the simulator MUST create one full fill at the limit price with deterministic zero fee/slippage assumptions unless configured otherwise by explicit inputs

#### Scenario: No eligible candle leaves order open
- **WHEN** no later eligible closed candle reaches the limit price
- **THEN** the simulator MUST create no fill and MUST leave reconciliation able to classify the order as open

#### Scenario: Simulator is deterministic
- **WHEN** the same order, command, and ordered replay market data are simulated more than once
- **THEN** the simulator MUST return the same fill/no-fill result, fill identifiers, timestamps, prices, fee/slippage metadata, and ordering

### Requirement: Paper Position And Portfolio Snapshots
The system SHALL persist deterministic paper position and portfolio snapshots projected from execution fills.

#### Scenario: Position projection orders fills deterministically
- **WHEN** position snapshots are projected from fills
- **THEN** fills MUST be applied by event time ascending and stable fill id tie-breaker
- **AND** repeated projection from the same fills MUST produce the same snapshots

#### Scenario: Position math covers v0 supported paths
- **WHEN** fills open, increase, reduce, or flatten long and short positions
- **THEN** snapshots MUST update signed quantity, average entry price, realized PnL when calculable, exposure notional, source fill reference, UTC timestamp, and metadata according to documented deterministic rules
- **AND** reversal must either be explicitly supported with tests or rejected as deferred with a clear validation result

#### Scenario: Portfolio snapshots aggregate supported state
- **WHEN** portfolio snapshots are projected from position and fill state
- **THEN** snapshots MUST persist mode, UTC timestamp, gross exposure, net exposure when useful, realized PnL, optional unrealized PnL only when a deterministic mark/reference price is supplied, equity or cash/collateral values only when modeled, and metadata for deferred assumptions

#### Scenario: Snapshot queries are deterministic
- **WHEN** callers query position snapshots by strategy, instrument, mode, and time, or portfolio snapshots by mode and time
- **THEN** the system MUST return matching snapshots in deterministic timestamp order with stable snapshot id tie-breakers

#### Scenario: Perps v0 assumptions are explicit
- **WHEN** paper snapshots are produced for the first Hyperliquid perps scope
- **THEN** metadata or documentation MUST state that intentional leverage, funding payments, liquidation modeling, and margin optimization are deferred, and USDC collateral is assumed only when collateral is modeled

