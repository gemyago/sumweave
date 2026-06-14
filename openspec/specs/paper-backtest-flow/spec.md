# paper-backtest-flow Specification

## Purpose
TBD - created by archiving change add-thin-paper-backtest-flow. Update Purpose after archive.
## Requirements
### Requirement: Thin Runtime Paper Backtest Boundary
The system SHALL provide a thin runtime orchestration boundary for one deterministic paper backtest flow without moving slice-owned behavior into the flow.

#### Scenario: Flow stays in runtime orchestration area
- **WHEN** the paper backtest flow is implemented
- **THEN** it MUST live in a small runtime orchestration package such as `runtime/flows` or `runtime/runs`
- **AND** it MUST NOT be implemented inside `runtime/execution`, `apps/signal-foundry`, or `apps/signal-ui`

#### Scenario: Flow preserves deterministic slice order
- **WHEN** a paper backtest run is executed
- **THEN** the flow MUST orchestrate the deterministic path in the order Data -> Analytics -> Strategy -> Governor -> Execution
- **AND** it MUST NOT call execution before governor approval decisions are available

#### Scenario: Flow avoids unrelated product surfaces
- **WHEN** the paper backtest flow is introduced
- **THEN** it MUST NOT require new HTTP routes, UI screens, scheduled jobs, persistence migrations, AI model calls, live venue credentials, wallet signing, or private trading endpoint calls

### Requirement: Explicit Paper Backtest Run Inputs
The system SHALL require explicit deterministic inputs for the paper backtest flow rather than inferring runtime state from the environment.

#### Scenario: Run request identifies deterministic scope
- **WHEN** a caller starts a paper backtest run
- **THEN** the request MUST include a stable run identity, canonical instrument, timeframe, half-open time range, moving-average crossover strategy parameters, governor policy, and fixed positive execution quantity

#### Scenario: Invalid run request is rejected
- **WHEN** a caller starts a paper backtest run with missing run identity, invalid canonical market scope, invalid strategy parameters, invalid governor policy, or non-positive execution quantity
- **THEN** the flow MUST reject the request with a validation error
- **AND** it MUST NOT return a partial paper execution result

### Requirement: Deterministic Cross-Slice Orchestration
The system SHALL compose existing deterministic slice services for the paper backtest run without duplicating slice calculations or policy behavior.

#### Scenario: Strategy consumes analytics over replay data
- **WHEN** the paper backtest flow evaluates strategy output
- **THEN** analytics MUST consume canonical replay candle data for the requested instrument, timeframe, and half-open time range
- **AND** strategy MUST consume analytics for the same requested instrument, timeframe, and half-open time range

#### Scenario: Governor consumes strategy actions
- **WHEN** strategy evaluation produces ordered candidate actions
- **THEN** governor evaluation MUST receive those candidate actions and the request governor policy
- **AND** the paper backtest result MUST include the ordered governor decisions returned by the governor slice

#### Scenario: Downstream stages stop after upstream failure
- **WHEN** data replay, analytics, strategy, or governor evaluation fails
- **THEN** the flow MUST return an error identifying the failed stage
- **AND** it MUST NOT call later deterministic stages for that run

### Requirement: Local Paper Execution From Approved Decisions
The system SHALL create local paper execution records only for approved governor decisions using deterministic replay-derived fill prices.

#### Scenario: Approved decision creates paper execution record set
- **WHEN** governor returns an approved decision with a replay candle whose end time equals the decision time
- **THEN** the flow MUST create one local execution command, one local order, one local fill priced at that candle close, and one reconciliation for that approved decision
- **AND** the paper backtest result MUST retain those records linked to the approved decision

#### Scenario: Non-approved decisions do not execute
- **WHEN** governor returns rejected or blocked decisions
- **THEN** the flow MUST include those decisions in the governor result
- **AND** it MUST NOT create commands, orders, fills, or reconciliations for those non-approved decisions

#### Scenario: Missing fill price fails the run
- **WHEN** an approved decision has no replay candle close price at the decision time
- **THEN** the flow MUST fail the run with an error
- **AND** it MUST NOT invent, interpolate, or read hidden data outside the requested replay range

### Requirement: Stable Paper Backtest Results
The system SHALL return stable in-memory paper backtest results for repeated runs over the same inputs and replay data.

#### Scenario: Repeated run is stable
- **WHEN** a caller executes the same paper backtest request against the same replay candle data more than once
- **THEN** the flow MUST return the same strategy result, governor decisions, execution commands, orders, fills, reconciliations, command identifiers, order identifiers, local client order identifiers, fill identifiers, flow-local reconciliation identifiers, and record ordering

#### Scenario: Result preserves ordered execution records
- **WHEN** multiple approved decisions are executed in one paper backtest run
- **THEN** paper execution records MUST be ordered by the source approved decision order
- **AND** stable local client order identifiers, fill identifiers, and flow-local reconciliation identifiers MUST be derived from the run identity and approved decision order rather than randomness or wall-clock time
- **AND** command identifiers and order identifiers MUST be produced only from deterministic execution-slice inputs supplied by the flow

