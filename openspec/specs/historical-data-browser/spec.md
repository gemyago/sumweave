# historical-data-browser Specification

## Purpose
TBD - created by archiving change add-historical-data-browser-api-ui. Update Purpose after archive.
## Requirements
### Requirement: Protected Historical Data Browser API
The backend application SHALL expose read-only authenticated data browser endpoints under the app-owned `/api/v1/data/*` surface for historical normalized candle discovery, deterministic candle browsing, and raw Hyperliquid payload evidence.

#### Scenario: Candle availability endpoint returns browseable normalized entries
- **WHEN** an authenticated operator calls `GET /api/v1/data/candle-availability`
- **THEN** the system MUST return a paginated `items` collection where each item represents exactly one available venue, symbol, and asset class entry that has persisted normalized candle data
- **AND** each item MUST include a non-empty `timeframes` collection of timeframe availability summaries with timeframe, earliest persisted candle start, latest persisted candle end, and persisted candle count
- **AND** each item MUST include a deterministic per-entry default slice containing timeframe, UTC `start`, and UTC `end` values valid for `GET /api/v1/data/candles`
- **AND** the response MUST exclude symbols that exist only in raw payload metadata or live venue symbol/reference data

#### Scenario: Candle availability endpoint filters and paginates entries deterministically
- **WHEN** an authenticated operator calls `GET /api/v1/data/candle-availability` with query parameters
- **THEN** the endpoint MUST accept only optional exact `venue`, `symbol`, and `assetClass` filters plus `limit` and opaque `cursor` pagination controls
- **AND** omitted `limit` MUST default to 50 entries, accepted `limit` values MUST be within 1 through 200 entries, and pagination MUST apply to top-level venue + symbol + asset class items rather than individual timeframe summaries
- **AND** unsupported query parameters, unsupported filter values, invalid limits, or invalid cursors MUST be rejected with a 4xx error

#### Scenario: Candle availability defaults can seed candle browsing
- **WHEN** the first candle availability page includes a default selection
- **THEN** the default selection MUST mirror the first returned item’s venue, symbol, asset class, per-entry default timeframe, UTC `start`, and UTC `end` values
- **AND** the selected range MUST be bounded to at most 500 timeframe intervals and MUST NOT require a mutation or ingestion action
- **AND** candle availability responses requested with `cursor` MUST omit the top-level default selection

#### Scenario: Candle availability endpoint supports empty data
- **WHEN** an authenticated operator calls `GET /api/v1/data/candle-availability` and no persisted normalized candles exist for the requested scope
- **THEN** the system MUST return an empty availability collection without inventing symbols, ranges, or default candle filters

#### Scenario: Candle endpoint returns deterministic normalized candles
- **WHEN** an authenticated operator calls `GET /api/v1/data/candles` with venue `hyperliquid-perps`, symbol, asset class, timeframe, and a UTC-compatible half-open `[start, end)` range
- **THEN** the system MUST return matching persisted canonical candle rows as operator-facing normalized candles in deterministic ascending order with chart-ready OHLC values, volume, quality, stable identity, provenance source, and provenance identity
- **AND** the system MUST NOT synthesize, interpolate, or fill missing candle intervals

#### Scenario: Candle endpoint still requires an exact candle scope
- **WHEN** an authenticated operator calls `GET /api/v1/data/candles` without venue, symbol, asset class, timeframe, start, or end
- **THEN** the system MUST reject the request with a 4xx error instead of guessing defaults from raw payloads, live venue symbols, or partial filters

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
The operator UI SHALL provide a protected `#/data` route for browse-first, read-only historical normalized candle browsing.

#### Scenario: Authenticated operator opens data route
- **WHEN** an authenticated operator opens `#/data`
- **THEN** the UI MUST load normalized candle availability before requiring manual candle filter entry
- **AND** the UI MUST render available venue, symbol, and asset class entries with timeframe/range summaries when availability exists
- **AND** the authenticated nav MUST include Data, Chat, and Providers entries

#### Scenario: Authenticated default route opens data
- **WHEN** an authenticated operator opens the application without an explicit route
- **THEN** the UI MUST land on `#/data` rather than the chat route
- **AND** explicit `#/chat` navigation MUST remain available

#### Scenario: Post-login fallback route opens data unless a route was requested
- **WHEN** an unauthenticated user completes login after opening the app root, an empty hash, or any flow without a saved explicit destination
- **THEN** the UI MUST navigate to `#/data`
- **AND** when the unauthenticated user originally requested an explicit protected route such as `#/chat` or `#/data`, successful login MUST return to that requested route instead of overriding it with the data default

#### Scenario: Unauthenticated operator is redirected
- **WHEN** an unauthenticated user opens `#/data`
- **THEN** the UI MUST redirect to login using the existing protected route behavior

#### Scenario: Default availability selection loads candles
- **WHEN** the availability response includes a default selection
- **THEN** the UI MUST select that entry and call `GET /api/v1/data/candles` with the default venue, symbol, asset class, timeframe, start, and end from the availability response
- **AND** the UI MUST render the returned normalized candles without requiring the operator to press Load first
- **AND** the UI MUST NOT call the broad `GET /api/v1/data/raw-payloads` metadata list as part of this initial availability/default-candle auto-load

#### Scenario: Default candle selection is populated when data exists
- **WHEN** the default or selected candle slice returns one or more candles
- **THEN** the UI MUST select a deterministic default candle from the returned slice
- **AND** the selected candle state MUST include the candle's venue, symbol, asset class, timeframe, start/end, provenance source, and provenance identity for linked evidence lookup
- **AND** default candle selection MUST use the same linked raw evidence behavior as manual candle selection

#### Scenario: No availability avoids guessed candle queries
- **WHEN** the availability response has no browseable normalized candle entries
- **THEN** the UI MUST show an empty state explaining that no normalized candle data is available
- **AND** the UI MUST NOT call the candle endpoint with guessed venue, symbol, timeframe, or time range values

#### Scenario: Selecting an availability entry begins browsing
- **WHEN** an operator selects an available venue, symbol, and asset class entry
- **THEN** the UI MUST use that item’s per-entry default slice and load candles for that exact scope
- **AND** selecting an entry MUST NOT start ingestion, backfill, repair, or any other mutation

#### Scenario: UTC range picker replaces free-entry range text boxes
- **WHEN** an operator edits the candle query range after selecting an availability entry
- **THEN** the UI MUST provide the shared UTC-aware range picker instead of requiring free-entry UTC start and end text boxes
- **AND** the picker MUST show the selected UTC `start` and `end` values that will be sent to the data APIs
- **AND** changing the range in the picker MUST NOT call the data APIs until the operator activates Load

#### Scenario: Data browser presets use deterministic candle anchors
- **WHEN** an operator chooses Last 24h, Last 7d, Last 30d, Last 90d, or Last 180d for a selected availability entry
- **THEN** the UI MUST resolve the preset against that entry and timeframe’s latest persisted candle end when available
- **AND** the resolved UTC range MUST remain visible and stable until the operator changes it again
- **AND** the UI MUST respect the entry’s earliest available candle start and the selected timeframe’s range limits before enabling Load

#### Scenario: Filter edits remain explicit and validated
- **WHEN** an operator edits timeframe or UTC start/end after selecting an availability entry and activates Load
- **THEN** the UI MUST call the candle and raw payload metadata APIs with the selected scope and edited query params, disable Load while loading, and render summary counts, a candlestick chart from normalized candle OHLC values, and a raw payload metadata table
- **AND** invalid UTC ranges, ranges outside the selected availability bounds, or ranges exceeding the documented 10,000-interval server cap MUST show inline validation and MUST NOT call the data APIs

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
The historical data browser's browsing and inspection behavior SHALL remain read-only; only a clearly labeled historical backfill action may create a durable ingestion job.

#### Scenario: Browser does not implicitly mutate historical data
- **WHEN** an operator loads availability, selects availability entries, edits filters, loads candles, selects candles, views linked evidence, or browses raw payload metadata/detail
- **THEN** the system MUST NOT start backfills, schedule ingestion, fill gaps, edit, delete, repair, re-normalize, or mutate raw payload, lineage, candle, trading, strategy, analytics, backtest, paper trading, or execution state

#### Scenario: Explicit backfill action creates only a job
- **WHEN** an operator uses the Data page's clearly labeled `Start historical backfill` action with an explicit scope
- **THEN** the UI MUST call the durable jobs API to create a `data.historical_raw_candle_backfill` job and show a link to that job in the generic admin jobs workspace
- **AND** the Data page MUST NOT execute ingestion directly or pretend candles are available before the job succeeds and data is reloaded

#### Scenario: UI terminology maps normalized copy to canonical rows
- **WHEN** the UI labels persisted candle rows for operators
- **THEN** it MUST use the phrase "normalized candles" while mapping those rows to existing canonical persisted `domain.Candle` data
- **AND** formal normalization-run and data-batch browsing MUST remain a follow-up unless separately scoped

### Requirement: Historical Data Backfill Entry Point
The Data page SHALL provide an explicit operator entry point for starting historical raw candle backfill jobs from a selected or manually entered candle scope.

#### Scenario: Operator starts backfill from current data scope
- **WHEN** an authenticated operator has selected or entered venue, symbol, asset class, timeframe, start, and end on the Data page and activates `Start historical backfill`
- **THEN** the UI MUST submit those fields plus optional page size/idempotency key to `POST /api/v1/jobs/historical-data-backfills`
- **AND** successful creation MUST show the created job id/status and a route link to the generic admin job detail

#### Scenario: Backfill entry validates before submit
- **WHEN** required scope fields are missing, the UTC range is invalid, `start >= end`, or the range exceeds the documented job interval cap known to the client
- **THEN** the UI MUST show inline validation and MUST NOT call the jobs API

#### Scenario: Backfill entry preserves data browsing state
- **WHEN** a backfill job is created from the Data page
- **THEN** existing availability, candle, selected candle, and raw evidence UI state MUST remain honest and MUST NOT be replaced by optimistic synthetic candle data

