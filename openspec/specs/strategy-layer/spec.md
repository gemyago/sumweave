# strategy-layer Specification

## Purpose
TBD - created by archiving change strategy-layer. Update Purpose after archive.
## Requirements
### Requirement: Canonical Strategy Domain
The system SHALL define canonical cross-slice strategy identity and candidate action records that are reusable by downstream deterministic slices and independent from persistence, venue, AI, evaluation-request, and execution implementation details.

#### Scenario: Strategy outputs do not expose implementation metadata
- **WHEN** governor or other deterministic runtime code consumes strategy outputs
- **THEN** the shared domain records MUST expose strategy identity and candidate actions without GORM tags, table names, venue payloads, AI prompt content, evaluation-request fields, evaluation-only parameter wrappers, or execution-order-only fields

#### Scenario: Strategy evaluation identity is explicit
- **WHEN** the system returns a strategy evaluation result
- **THEN** the `runtime/strategy` result MUST identify the instrument, timeframe, strategy kind, request parameters, requested time range, and ordered candidate actions produced for deterministic downstream use

#### Scenario: Candidate actions expose decision time and input range
- **WHEN** the system emits a candidate action
- **THEN** the action MUST include a UTC decision time used for ordering and a separate half-open input range describing the analytics inputs that contributed to the action

### Requirement: Deterministic Strategy Evaluation
The system SHALL evaluate strategy logic from canonical analytics inputs with stable ordering and explicit range semantics.

#### Scenario: Repeated evaluation is stable
- **WHEN** the strategy service evaluates the same strategy request with the same analytics inputs and parameters
- **THEN** it MUST return the same ordered candidate actions with the same decision times, input ranges, action kinds, and quality states

#### Scenario: Strategy evaluation uses explicit half-open boundaries
- **WHEN** a caller requests strategy evaluation for a time range
- **THEN** the strategy layer MUST request analytics only for that same `[start, end)` range and MUST NOT read or infer pre-start state from hidden data outside the requested range

#### Scenario: Candidate actions use analytics point time
- **WHEN** the strategy layer emits a candidate action from aligned analytics points
- **THEN** the decision time MUST equal the current aligned analytics point time normalized to UTC

#### Scenario: Candidate actions stay ordered
- **WHEN** the strategy layer returns candidate actions for a single evaluation
- **THEN** those actions MUST be ordered by decision time ascending

### Requirement: Initial Moving-Average Crossover Strategy
The system SHALL support an initial deterministic moving-average crossover strategy derived from existing analytics-layer moving averages over candle close prices.

#### Scenario: Strategy requests fast and slow moving averages
- **WHEN** a caller requests the moving-average crossover strategy with valid parameters
- **THEN** the strategy layer MUST request one fast moving-average series and one slow moving-average series from the analytics layer using the same instrument, timeframe, and `[start, end)` range

#### Scenario: Bullish crossover emits long candidate action
- **WHEN** the fast moving average is less than or equal to the slow moving average at the previous aligned point and greater than the slow moving average at the current aligned point
- **THEN** the strategy layer MUST emit a `long` candidate action at the current aligned point time

#### Scenario: Bearish crossover emits short candidate action
- **WHEN** the fast moving average is greater than or equal to the slow moving average at the previous aligned point and less than the slow moving average at the current aligned point
- **THEN** the strategy layer MUST emit a `short` candidate action at the current aligned point time

#### Scenario: First aligned point establishes baseline only
- **WHEN** the strategy layer observes the first aligned fast and slow moving-average point inside the requested range
- **THEN** it MUST establish the in-range comparison baseline and MUST NOT emit a candidate action until a later aligned point produces a crossover

#### Scenario: No crossover emits no candidate action
- **WHEN** consecutive aligned moving-average points do not cross from one side of the slow average to the other
- **THEN** the strategy layer MUST NOT emit a candidate action for that aligned point

#### Scenario: Invalid crossover parameters are rejected
- **WHEN** a caller requests the moving-average crossover strategy with a non-positive window, equal windows, or a fast window greater than the slow window
- **THEN** the system MUST reject the request with a validation error and MUST NOT return a partial strategy evaluation

### Requirement: Strategy Quality Propagation
The system SHALL propagate analytics quality into strategy candidate actions deterministically across the full crossover decision window.

#### Scenario: Validated analytics inputs produce validated action
- **WHEN** every analytics point contributing to a crossover action has validated quality
- **THEN** the emitted candidate action MUST have validated quality

#### Scenario: Suspect analytics input produces suspect action
- **WHEN** any analytics point contributing to a crossover action has suspect quality
- **THEN** the emitted candidate action MUST have suspect quality

#### Scenario: Raw analytics input does not become validated action
- **WHEN** a crossover action is derived from one or more raw analytics points and no contributing analytics point is suspect
- **THEN** the emitted candidate action MUST have raw quality

### Requirement: Strategy Service Boundary
The system SHALL expose strategy evaluation through a runtime service boundary that depends on canonical analytics behavior rather than on venues, AI systems, or execution side effects.

#### Scenario: Strategy service consumes analytics contract
- **WHEN** the strategy layer needs moving-average inputs
- **THEN** it MUST use the analytics layer's canonical analytics calculation behavior or a consumer-defined interface with equivalent semantics

#### Scenario: Strategy stays outside venue mechanics
- **WHEN** strategy candidate actions are evaluated for venue-derived market data
- **THEN** the strategy layer MUST NOT depend on vendor payloads, symbol mapping rules, venue HTTP clients, pagination mechanics, or live venue network access

#### Scenario: Strategy stays outside AI-assisted research
- **WHEN** strategy candidate actions are evaluated for the deterministic path
- **THEN** the strategy layer MUST NOT depend on AI model calls, prompts, generated explanations, or agent session state

#### Scenario: Strategy emits no execution side effects
- **WHEN** the strategy layer evaluates candidate actions
- **THEN** it MUST NOT place orders, mutate venue state, or require execution-side dependencies in order to complete the evaluation

### Requirement: On-Demand Strategy V0
The system SHALL provide the initial strategy layer without requiring persisted strategy outputs or new external API routes.

#### Scenario: Strategy evaluation does not require strategy storage
- **WHEN** the initial strategy service evaluates a supported strategy kind
- **THEN** the evaluation MUST complete from analytics reads without requiring a strategy table, migration, backfill job, or materialized signal store

#### Scenario: External API remains unchanged
- **WHEN** the strategy layer is introduced
- **THEN** the system MUST NOT require a new public HTTP endpoint for strategy evaluation in order to satisfy the initial capability

#### Scenario: Backend wiring is deferred until needed
- **WHEN** the initial strategy runtime slice is introduced without a current backend consumer
- **THEN** the system MUST NOT require `apps/signal-foundry` dependency injection wiring in order to satisfy the initial capability

