## ADDED Requirements

### Requirement: Protected Historical Data Browser API
The backend application SHALL expose read-only authenticated data browser endpoints under the app-owned `/api/v1/data/*` surface for historical normalized candles and raw Hyperliquid payload evidence.

#### Scenario: Candle endpoint returns deterministic normalized candles
- **WHEN** an authenticated operator calls `GET /api/v1/data/candles` with venue `hyperliquid-perps`, symbol, asset class, timeframe, and a UTC-compatible half-open `[start, end)` range
- **THEN** the system MUST return matching persisted canonical candle rows as operator-facing normalized candles in deterministic ascending order with chart-ready OHLC values, volume, quality, stable identity, provenance source, and provenance identity
- **AND** the system MUST NOT synthesize, interpolate, or fill missing candle intervals

#### Scenario: Candle endpoint enforces exact server range cap
- **WHEN** an authenticated operator calls `GET /api/v1/data/candles` with an otherwise valid range where `end - start` is greater than `10,000 * duration(timeframe)` using `1m=60s`, `5m=300s`, `15m=900s`, `1h=3600s`, `4h=14400s`, or `1d=86400s`
- **THEN** the system MUST reject the request with `400 Bad Request` before replaying candle data
- **AND** a request exactly equal to `10,000 * duration(timeframe)` MUST NOT be rejected as oversized

#### Scenario: Candle endpoint validates scope
- **WHEN** a candle request has unsupported venue, empty or non-canonicalizable symbol, unsupported asset class, unsupported timeframe, non-UTC-compatible timestamps, `start >= end`, or a range exceeding the exact 10,000-interval server cap
- **THEN** the system MUST reject the request with a 4xx error before returning data

#### Scenario: Raw payload list endpoint returns metadata only
- **WHEN** an authenticated operator calls `GET /api/v1/data/raw-payloads` with venue and optional symbol, asset class, timeframe, range, ingestion run ID, entity hint, endpoint, request type, limit, and cursor filters
- **THEN** the system MUST return matching raw payload metadata with deterministic pagination
- **AND** the response MUST NOT include raw response body bytes or bounded body previews

#### Scenario: Raw payload detail endpoint returns bounded preview
- **WHEN** an authenticated operator calls `GET /api/v1/data/raw-payloads/{id}` for a persisted raw payload
- **THEN** the system MUST return metadata plus a deterministic bounded response-body preview, response body size in bytes, and a boolean truncation flag

#### Scenario: Raw payload detail missing ID returns not found
- **WHEN** an authenticated operator calls `GET /api/v1/data/raw-payloads/{id}` for an unknown raw payload identity
- **THEN** the system MUST return `404`

#### Scenario: Candle raw payload endpoint returns linked evidence
- **WHEN** an authenticated operator calls `GET /api/v1/data/candle-raw-payloads` with venue, symbol, asset class, timeframe, candle start/end, provenance source, and provenance identity from a selected normalized candle
- **THEN** the system MUST return deterministic raw payload metadata linked to that exact provenance-bearing selected candle
- **AND** an absence of links in older or development data MUST be represented as an empty `items` collection rather than as a mutation or repair action

#### Scenario: Candle raw payload endpoint rejects missing provenance
- **WHEN** an authenticated operator calls `GET /api/v1/data/candle-raw-payloads` without a non-empty provenance source or without a non-empty provenance identity
- **THEN** the system MUST reject the request with `400 Bad Request` rather than performing ambiguous time-bucket matching

#### Scenario: Data browser endpoints require authentication
- **WHEN** a caller without a valid authenticated identity calls any `/api/v1/data/*` endpoint
- **THEN** the system MUST reject the request as unauthorized

### Requirement: Protected Historical Data Browser UI
The operator UI SHALL provide a protected `#/data` route for read-only historical data browsing.

#### Scenario: Authenticated operator opens data route
- **WHEN** an authenticated operator opens `#/data`
- **THEN** the UI MUST render filters for venue, symbol, asset class, timeframe, UTC start, UTC end, and optional ingestion run ID
- **AND** the authenticated nav MUST include `Chat / Data / Providers`

#### Scenario: Unauthenticated operator is redirected
- **WHEN** an unauthenticated user opens `#/data`
- **THEN** the UI MUST redirect to login using the existing protected route behavior

#### Scenario: Initial route does not auto-query large ranges
- **WHEN** the data browser first renders
- **THEN** the UI MUST show the filter form and explanatory copy without automatically calling the data APIs

#### Scenario: Filter submission loads candles and raw payload metadata
- **WHEN** an operator enters valid filters and activates Load
- **THEN** the UI MUST call the candle and raw payload metadata APIs with matching query params, disable Load while loading, and render summary counts, a candlestick chart from normalized candle OHLC values, and a raw payload metadata table

#### Scenario: Invalid filter input is handled client-side
- **WHEN** required filters are missing, the UTC range is invalid, or the selected timeframe/range exceeds the documented 10,000-interval server cap
- **THEN** the UI MUST show inline validation and MUST NOT call the data APIs

#### Scenario: Empty and error states are visible
- **WHEN** candles or raw payload metadata responses are empty
- **THEN** the UI MUST show appropriate empty states without clearing unrelated successful data unnecessarily
- **AND** API errors MUST be shown using alert semantics

#### Scenario: Raw payload detail drawer shows bounded preview
- **WHEN** an operator selects a raw payload metadata row
- **THEN** the UI MUST fetch raw payload detail and open an accessible detail drawer with metadata, hashes, body reference, instrument/timeframe/range hints, bounded response body preview, and truncation indication

#### Scenario: Selected candle shows linked raw evidence
- **WHEN** an operator selects a candle from the reliable v0 candle selection path
- **THEN** the UI MUST call the candle-linked raw payload API with the selected candle's `provenanceSource` and `provenanceIdentity` and render linked raw payload evidence or an empty evidence state

### Requirement: Historical Data Browser Read-Only Safety
The historical data browser SHALL remain read-only across backend and UI behavior.

#### Scenario: Browser does not mutate historical data
- **WHEN** an operator uses the historical data browser API or UI
- **THEN** the system MUST NOT start backfills, schedule ingestion, fill gaps, edit, delete, repair, re-normalize, or mutate raw payload, lineage, candle, trading, strategy, analytics, backtest, paper trading, or execution state

#### Scenario: UI terminology maps normalized copy to canonical rows
- **WHEN** the UI labels persisted candle rows for operators
- **THEN** it MUST use the phrase "normalized candles" while mapping those rows to existing canonical persisted `domain.Candle` data
- **AND** formal normalization-run and data-batch browsing MUST remain a follow-up unless separately scoped
