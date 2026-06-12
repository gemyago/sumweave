## ADDED Requirements

### Requirement: Canonical Analytics Domain
The system SHALL define canonical analytics output records that are reusable by downstream deterministic slices and independent from persistence, venue, and AI implementation details.

#### Scenario: Analytics outputs do not expose implementation metadata
- **WHEN** strategy or other deterministic runtime code consumes analytics outputs
- **THEN** those outputs MUST be available as shared domain records without GORM tags, table names, vendor payloads, AI prompt content, or database-only fields

#### Scenario: Analytics identity is explicit
- **WHEN** the system returns an analytics series
- **THEN** the series MUST identify the instrument, timeframe, indicator kind, indicator parameters, requested time range, and output points needed for deterministic downstream use

#### Scenario: Analytics points expose point time and value range
- **WHEN** the system returns an analytics point
- **THEN** the point MUST include a UTC point time used for ordering and a separate half-open value range describing the candle input interval that contributed to the value

### Requirement: Deterministic Candle Analytics
The system SHALL compute candle-derived analytics from canonical replay candle data with stable ordering and explicit half-open range semantics.

#### Scenario: Repeated calculation is stable
- **WHEN** the analytics service calculates the same indicator with the same canonical candle replay inputs and parameters
- **THEN** it MUST return the same ordered points with the same point times, value ranges, values, quality states, and indicator identity

#### Scenario: Analytics request uses half-open boundaries
- **WHEN** a caller requests candle analytics for a time range
- **THEN** the service MUST read and calculate over candles whose start times are inside `[start, end)` and MUST exclude candles whose start time equals `end`

#### Scenario: Output points use candle close time
- **WHEN** an analytics point is emitted for a current candle
- **THEN** the point time MUST equal the current candle's `TimeRange.End` normalized to UTC

#### Scenario: Output points expose contributing range
- **WHEN** an analytics point is emitted from one or more contributing candles
- **THEN** the point value range MUST start at the first contributing candle's `TimeRange.Start` and end at the current candle's `TimeRange.End`

#### Scenario: Output points use a stable tie-breaker
- **WHEN** analytics are calculated from an ordered replay candle sequence
- **THEN** output points MUST be ordered by point time ascending and by the current source replay candle identity ascending when point times are equal

### Requirement: Initial Candle Indicators
The system SHALL support an initial deterministic indicator set derived from candle close prices.

#### Scenario: Moving average over close prices
- **WHEN** a caller requests a moving average with a positive window size
- **THEN** the system MUST emit each point as the arithmetic mean of the close prices from the window of consecutive candles ending at that point

#### Scenario: Moving average value range covers the window
- **WHEN** the system emits a moving-average point
- **THEN** the point value range MUST cover the first candle in the window through the current candle using half-open range semantics

#### Scenario: Period return over close prices
- **WHEN** a caller requests a period return with a positive lookback size and a positive lookback close price
- **THEN** the system MUST emit each point as `(current close - lookback close) / lookback close`

#### Scenario: Period return value range covers the lookback interval
- **WHEN** the system emits a period-return point
- **THEN** the point value range MUST start at the lookback candle's `TimeRange.Start` and end at the current candle's `TimeRange.End`

#### Scenario: Invalid period-return denominator fails the request
- **WHEN** an otherwise-computable period-return point would use a zero or negative lookback close price
- **THEN** the system MUST reject the request with an error and MUST NOT return a partial analytics series

#### Scenario: Warmup points are omitted
- **WHEN** the replayed candle range does not yet contain enough prior candles for a requested moving average or period return point
- **THEN** the system MUST omit that output point rather than inventing, padding, or reading hidden data outside the requested replay range

#### Scenario: Invalid indicator parameters are rejected
- **WHEN** a caller requests an indicator with a non-positive window size, non-positive lookback size, or unsupported indicator kind
- **THEN** the system MUST reject the request with a validation error and MUST NOT return a partial analytics series

### Requirement: Analytics Quality Propagation
The system SHALL propagate input data quality into analytics outputs deterministically.

#### Scenario: Validated inputs produce validated output
- **WHEN** every candle contributing to an analytics point has validated quality
- **THEN** the output point MUST have validated quality

#### Scenario: Suspect inputs produce suspect output
- **WHEN** any candle contributing to an analytics point has suspect quality
- **THEN** the output point MUST have suspect quality

#### Scenario: Raw inputs do not become validated output
- **WHEN** an analytics point is calculated from one or more raw candles and no contributing candle is suspect
- **THEN** the output point MUST have raw quality

### Requirement: Analytics Service Boundary
The system SHALL expose analytics calculation through a runtime service boundary that depends on canonical data reads rather than on venues, AI systems, or external network calls.

#### Scenario: Analytics service consumes data replay contract
- **WHEN** the analytics service needs candle input
- **THEN** it MUST use the data layer's canonical candle replay behavior or a consumer-defined interface with equivalent semantics

#### Scenario: Analytics stays outside venue mechanics
- **WHEN** analytics are calculated for venue-derived market data
- **THEN** the analytics layer MUST NOT depend on vendor payloads, symbol mapping rules, venue HTTP clients, pagination mechanics, or live venue network access

#### Scenario: Analytics stays outside AI-assisted research
- **WHEN** analytics are calculated for the deterministic path
- **THEN** the analytics layer MUST NOT depend on AI model calls, prompts, generated explanations, or agent session state

### Requirement: On-Demand Analytics V0
The system SHALL provide the initial analytics layer without requiring persisted analytics outputs or new external API routes.

#### Scenario: Analytics calculation does not require analytics storage
- **WHEN** the initial analytics service calculates a supported indicator
- **THEN** the calculation MUST complete from canonical data reads without requiring an analytics table, migration, backfill job, or materialized indicator store

#### Scenario: External API remains unchanged
- **WHEN** the analytics layer is introduced
- **THEN** the system MUST NOT require a new public HTTP endpoint for analytics in order to satisfy the initial capability

#### Scenario: Backend wiring is deferred until needed
- **WHEN** the initial analytics runtime slice is introduced without a current backend consumer
- **THEN** the system MUST NOT require `apps/signal-foundry` dependency injection wiring in order to satisfy the initial capability
