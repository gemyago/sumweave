## ADDED Requirements

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
